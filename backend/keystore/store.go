package keystore

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	openpgp "github.com/ProtonMail/go-crypto/openpgp/v2"
	"github.com/ProtonMail/gopenpgp/v3/crypto"
	"github.com/developaaah/pgp-manager/backend/config"
	"github.com/developaaah/pgp-manager/backend/gnupg"
	"github.com/developaaah/pgp-manager/backend/model"
)

// ErrAlreadyExists is returned when all keys in an import block are already present.
var ErrAlreadyExists = errors.New("key already in keystore")

// Store manages PGP key files on disk.
type Store struct {
	dir      string
	useGnupg bool // true for the default store (cfg.KeysDir == "") — supplements keys from GnuPG's keyring
}

// New creates a new Store backed by the given config.
func New(cfg *config.Config) (*Store, error) {
	dir, err := StorageDir(cfg)
	if err != nil {
		return nil, fmt.Errorf("keystore: %w", err)
	}
	return &Store{
		dir:      dir,
		useGnupg: cfg.KeysDir == "",
	}, nil
}

// List returns metadata for all keys in the store.
// When the store uses the GnuPG home directory, it additionally reads keys from
// GnuPG's native keyring (via "gpg --export") if the gpg binary is available.
func (s *Store) List() ([]*model.KeyInfo, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("keystore: %w", err)
	}

	seen := make(map[string]bool)
	var keys []*model.KeyInfo

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".asc" {
			continue
		}

		path := filepath.Join(s.dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		info, err := armoredToInfo(string(data))
		if err != nil {
			slog.Warn("skipping invalid key file", "file", entry.Name())
			continue
		}
		seen[info.Fingerprint] = true
		keys = append(keys, info)
	}

	// Supplement with keys from GnuPG's native keyring when using gnupg home dir.
	if s.useGnupg && gnupg.IsAvailable() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		pubArmoreds, gpgErr := gnupg.ExportPublicKeys(ctx)
		if gpgErr != nil {
			slog.Debug("gpg export public keys failed (skipping)", "error", gpgErr)
		}
		for _, arm := range pubArmoreds {
			info, err := armoredToInfo(arm)
			if err != nil {
				continue
			}
			if seen[info.Fingerprint] {
				continue
			}
			seen[info.Fingerprint] = true
			keys = append(keys, info)
		}

		privArmoreds, gpgErr := gnupg.ExportSecretKeys(ctx)
		if gpgErr != nil {
			slog.Debug("gpg export secret keys failed (skipping)", "error", gpgErr)
		}
		for _, arm := range privArmoreds {
			info, err := armoredToInfo(arm)
			if err != nil {
				continue
			}
			if seen[info.Fingerprint] {
				continue
			}
			seen[info.Fingerprint] = true
			keys = append(keys, info)
		}
	}

	return keys, nil
}

// Import adds a single armored key to the store.
// Returns (fingerprint, ErrAlreadyExists, nil) when the key is already present.
func (s *Store) Import(armored string) (string, error) {
	armored = SanitizeArmored(armored)
	key, err := crypto.NewKeyFromArmored(armored)
	if err != nil {
		return "", fmt.Errorf("keystore: invalid armored key: %w", err)
	}

	fp := key.GetFingerprint()
	path := filepath.Join(s.dir, fp+".asc")

	if _, err := os.Stat(path); err == nil {
		return fp, ErrAlreadyExists
	}

	if err := os.WriteFile(path, []byte(armored), 0600); err != nil {
		return "", fmt.Errorf("keystore: %w", err)
	}

	return fp, nil
}

// keyRingPacketTags are the packet types that belong in a transferable key
// (RFC 9580 §10.1/§10.2): signature, secret/public (sub)keys, user ID and
// user attribute.
var keyRingPacketTags = map[uint8]bool{
	2:  true, // signature
	5:  true, // secret key
	6:  true, // public key
	7:  true, // secret subkey
	13: true, // user ID
	14: true, // public subkey
	17: true, // user attribute
}

// parsePacketHeader decodes an OpenPGP packet header at the start of b.
// Returns the packet tag, the header length, the body length and whether
// the header is well-formed with a definite length. Partial body lengths
// and indeterminate old-format lengths report ok=false — for sanitizing we
// only trust packets whose extent is fully known.
func parsePacketHeader(b []byte) (tag uint8, hdrLen, bodyLen int, ok bool) {
	if len(b) < 2 || b[0]&0x80 == 0 {
		return 0, 0, 0, false
	}
	if b[0]&0x40 != 0 {
		// New format: tag in the low 6 bits, variable-length length field.
		tag = b[0] & 0x3F
		switch l := int(b[1]); {
		case l < 192:
			return tag, 2, l, true
		case l <= 223:
			if len(b) < 3 {
				return 0, 0, 0, false
			}
			return tag, 3, (l-192)<<8 + int(b[2]) + 192, true
		case l == 255:
			if len(b) < 6 {
				return 0, 0, 0, false
			}
			return tag, 6, int(b[2])<<24 | int(b[3])<<16 | int(b[4])<<8 | int(b[5]), true
		default:
			return tag, 0, 0, false // partial body length
		}
	}
	// Old format: tag in bits 5-2, length type in bits 1-0.
	tag = (b[0] >> 2) & 0x0F
	switch b[0] & 0x03 {
	case 0:
		return tag, 2, int(b[1]), true
	case 1:
		if len(b) < 3 {
			return 0, 0, 0, false
		}
		return tag, 3, int(b[1])<<8 | int(b[2]), true
	case 2:
		if len(b) < 5 {
			return 0, 0, 0, false
		}
		return tag, 5, int(b[1])<<24 | int(b[2])<<16 | int(b[3])<<8 | int(b[4]), true
	default:
		return tag, 0, 0, false // indeterminate length
	}
}

// validPacketChain reports whether body[i:] is a sequence of well-formed
// definite-length packets reaching exactly EOF, starting with a key-ring
// packet. Used to confirm resync candidates.
func validPacketChain(body []byte, i int) bool {
	tag, h, l, ok := parsePacketHeader(body[i:])
	if !ok || !keyRingPacketTags[tag] {
		return false
	}
	for i < len(body) {
		_, h, l, ok = parsePacketHeader(body[i:])
		if !ok || i+h+l > len(body) {
			return false
		}
		i += h + l
	}
	return i == len(body)
}

// SanitizeArmored re-writes an armored key block keeping only packets that
// belong in a key ring. GnuPG exports can carry proprietary packets (comment
// packets, tag 16; ring-trust, tag 12) that go-crypto rejects as "unknown
// critical packet type" — GnuPG itself drops them on import, so do we.
// Foreign packets with broken or partial lengths are skipped by scanning
// forward to the next position from which a valid packet chain reaches EOF,
// so good packets after the junk survive.
// If nothing changed or nothing could be read, the input is returned
// unchanged so the regular parser produces its own error message.
func SanitizeArmored(armoredBlock string) string {
	block, err := armor.Decode(strings.NewReader(armoredBlock))
	if err != nil {
		return armoredBlock
	}
	data, err := io.ReadAll(block.Body)
	if err != nil {
		return armoredBlock
	}

	var body bytes.Buffer
	changed := false
	i := 0
	for i < len(data) {
		tag, hdrLen, bodyLen, ok := parsePacketHeader(data[i:])
		if ok && i+hdrLen+bodyLen <= len(data) {
			if keyRingPacketTags[tag] {
				body.Write(data[i : i+hdrLen+bodyLen])
			} else {
				changed = true
			}
			i += hdrLen + bodyLen
			continue
		}
		// Unreadable header, partial length, or length beyond EOF —
		// resync at the next position holding a valid packet chain.
		changed = true
		j := i + 1
		for ; j < len(data); j++ {
			if validPacketChain(data, j) {
				break
			}
		}
		slog.Debug("keystore: sanitize: skipping unreadable data", "offset", i, "resync", j)
		i = j
	}
	if !changed || body.Len() == 0 {
		return armoredBlock
	}

	var out bytes.Buffer
	w, err := armor.Encode(&out, block.Type, nil)
	if err != nil {
		return armoredBlock
	}
	if _, err := w.Write(body.Bytes()); err != nil {
		w.Close()
		return armoredBlock
	}
	if err := w.Close(); err != nil {
		return armoredBlock
	}
	return out.String()
}

// splitArmoredBlocks splits concatenated armored blocks ("-----BEGIN PGP …"
// to "-----END PGP …-----") into individual blocks. ReadArmoredKeyRing only
// parses the first block of its input, so multi-block input (several keys
// copied in sequence) must be split first.
func splitArmoredBlocks(armored string) []string {
	const beginMarker = "-----BEGIN PGP"
	const endMarker = "-----END PGP"
	var blocks []string
	rest := armored
	for {
		begin := strings.Index(rest, beginMarker)
		if begin == -1 {
			break
		}
		end := strings.Index(rest[begin:], endMarker)
		if end == -1 {
			break
		}
		tail := strings.Index(rest[begin+end+len(endMarker):], "-----")
		if tail == -1 {
			break
		}
		stop := begin + end + len(endMarker) + tail + len("-----")
		blocks = append(blocks, rest[begin:stop])
		rest = rest[stop:]
	}
	return blocks
}

// readAllEntities parses every armored block in the input and returns all
// contained entities.
func readAllEntities(armored string) (openpgp.EntityList, error) {
	var all openpgp.EntityList
	var lastErr error
	for _, block := range splitArmoredBlocks(armored) {
		entities, err := openpgp.ReadArmoredKeyRing(strings.NewReader(SanitizeArmored(block)))
		if err != nil {
			lastErr = err
			continue
		}
		all = append(all, entities...)
	}
	if len(all) == 0 {
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, fmt.Errorf("no armored PGP blocks found")
	}
	return all, nil
}

// ImportAll imports all keys from an armored block (may contain multiple keys).
// Returns the fingerprints of successfully imported keys.
func (s *Store) ImportAll(armored string) ([]string, error) {
	entities, err := readAllEntities(armored)
	if err != nil {
		return nil, fmt.Errorf("keystore: %w", err)
	}

	var fps []string
	var lastErr error
	dupCount := 0
	for _, entity := range entities {
		key, err := crypto.NewKeyFromEntity(entity)
		if err != nil {
			lastErr = err
			continue
		}
		arm, err := key.Armor()
		if err != nil {
			lastErr = err
			continue
		}
		fp := key.GetFingerprint()
		path := filepath.Join(s.dir, fp+".asc")
		if _, statErr := os.Stat(path); statErr == nil {
			dupCount++
			fps = append(fps, fp)
			continue
		}
		if err := os.WriteFile(path, []byte(arm), 0600); err != nil {
			lastErr = err
			continue
		}
		fps = append(fps, fp)
	}

	newCount := len(fps) - dupCount
	if newCount == 0 && lastErr != nil {
		return nil, fmt.Errorf("keystore: all imports failed: %w", lastErr)
	}
	if newCount == 0 && dupCount > 0 {
		return fps, ErrAlreadyExists
	}
	return fps, nil
}

// Preview parses an armored block (one or many keys) and returns metadata for each key.
// AlreadyExists is set to true when the key's fingerprint is already in the store.
func (s *Store) Preview(armored string) []model.KeyInfo {
	entities, err := readAllEntities(armored)
	if err != nil {
		return nil
	}
	var result []model.KeyInfo
	for _, entity := range entities {
		key, err := crypto.NewKeyFromEntity(entity)
		if err != nil {
			continue
		}
		arm, err := key.Armor()
		if err != nil {
			continue
		}
		info, err := armoredToInfo(arm)
		if err != nil {
			continue
		}
		path := filepath.Join(s.dir, info.Fingerprint+".asc")
		_, statErr := os.Stat(path)
		info.AlreadyExists = statErr == nil
		result = append(result, *info)
	}
	return result
}

// ImportMultiple imports all keys from an armored block that may contain multiple keys.
// Returns per-key results (success or failure) to support partial success.
func (s *Store) ImportMultiple(armored string) []model.KeyImportEntry {
	entities, err := readAllEntities(armored)
	if err != nil {
		return []model.KeyImportEntry{{Error: fmt.Sprintf("keystore: invalid armored data: %v", err)}}
	}

	var results []model.KeyImportEntry
	for _, entity := range entities {
		key, err := crypto.NewKeyFromEntity(entity)
		if err != nil {
			results = append(results, model.KeyImportEntry{Error: err.Error()})
			continue
		}

		arm, err := key.Armor()
		if err != nil {
			results = append(results, model.KeyImportEntry{Error: err.Error()})
			continue
		}

		fp := key.GetFingerprint()
		path := filepath.Join(s.dir, fp+".asc")
		if _, statErr := os.Stat(path); statErr == nil {
			results = append(results, model.KeyImportEntry{
				Fingerprint: fp,
				UID:         getUID(entity),
				Error:       "already exists",
			})
			continue
		}

		if err := os.WriteFile(path, []byte(arm), 0600); err != nil {
			results = append(results, model.KeyImportEntry{Error: err.Error()})
			continue
		}

		results = append(results, model.KeyImportEntry{
			Fingerprint: fp,
			UID:         getUID(entity),
		})
	}
	return results
}

// Delete removes a key from the store by fingerprint.
func (s *Store) Delete(fingerprint string) error {
	path := filepath.Join(s.dir, fingerprint+".asc")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("keystore: %w", err)
	}
	return nil
}

// GetArmored returns the armored key data for the given fingerprint.
func (s *Store) GetArmored(fingerprint string) (string, error) {
	path := filepath.Join(s.dir, fingerprint+".asc")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("keystore: %w", err)
	}
	return string(data), nil
}

// armoredToInfo parses an armored key string and returns KeyInfo.
func armoredToInfo(armored string) (*model.KeyInfo, error) {
	key, err := crypto.NewKeyFromArmored(SanitizeArmored(armored))
	if err != nil {
		return nil, fmt.Errorf("armoredToInfo: %w", err)
	}

	info := &model.KeyInfo{
		Fingerprint: key.GetFingerprint(),
		IsPrivate:   key.IsPrivate(),
	}

	if entity := key.GetEntity(); entity != nil {
		for _, id := range entity.Identities {
			if id.UserId != nil {
				info.PrimaryUID = id.UserId.Name
				info.Email = id.UserId.Email
				break
			}
		}
		if entity.PrimaryKey != nil {
			info.CreatedAt = new(entity.PrimaryKey.CreationTime)
		}

		// Parse subkeys.
		info.Subkeys = parseSubkeys(entity)
	}

	now := time.Now().Unix()
	if key.IsRevoked(now) {
		info.Status = "revoked"
	} else if key.IsExpired(now) {
		info.Status = "expired"
	} else {
		info.Status = "valid"
	}

	return info, nil
}

// parseSubkeys extracts subkey metadata from an openpgp Entity.
func parseSubkeys(entity *openpgp.Entity) []model.SubkeyInfo {
	var subkeys []model.SubkeyInfo
	for _, sub := range entity.Subkeys {
		// Determine revocation status from revocation signatures.
		isRevoked := len(sub.Revocations) > 0

		// Determine expiry from binding signature lifetime.
		var expiresAt *time.Time
		if len(sub.Bindings) > 0 {
			sig := sub.Bindings[0].Packet
			if sig.KeyLifetimeSecs != nil {
				expiresAt = new(sub.PublicKey.CreationTime.Add(time.Duration(*sig.KeyLifetimeSecs) * time.Second))
			}
		}

		// Determine usage from binding signature flags.
		var usage []string
		if len(sub.Bindings) > 0 {
			sig := sub.Bindings[0].Packet
			if sig.FlagSign || sig.FlagEncryptCommunications || sig.FlagEncryptStorage {
				if sig.FlagSign {
					usage = append(usage, "sign")
				}
				if sig.FlagEncryptCommunications || sig.FlagEncryptStorage {
					usage = append(usage, "encrypt")
				}
			}
		}

		sk := model.SubkeyInfo{
			Fingerprint: hex.EncodeToString(sub.PublicKey.Fingerprint),
			Algorithm:   algorithmLabel(sub.PublicKey),
			CreatedAt:   sub.PublicKey.CreationTime,
			ExpiresAt:   expiresAt,
			IsRevoked:   isRevoked,
			Usage:       usage,
		}

		subkeys = append(subkeys, sk)
	}
	return subkeys
}

// algorithmLabel returns a human-readable algorithm string for a public key.
func algorithmLabel(pub *packet.PublicKey) string {
	bitLen, _ := pub.BitLength()
	switch pub.PubKeyAlgo {
	case packet.PubKeyAlgoRSA, packet.PubKeyAlgoRSAEncryptOnly, packet.PubKeyAlgoRSASignOnly:
		return fmt.Sprintf("RSA-%d", bitLen)
	case packet.PubKeyAlgoElGamal:
		return fmt.Sprintf("ElGamal-%d", bitLen)
	case packet.PubKeyAlgoECDSA:
		return fmt.Sprintf("ECDSA-%d", bitLen)
	case packet.PubKeyAlgoECDH:
		// 255-bit ECDH is Curve25519 in practice.
		if bitLen == 255 {
			return "Curve25519"
		}
		return fmt.Sprintf("ECDH-%d", bitLen)
	case packet.PubKeyAlgoEdDSA, packet.PubKeyAlgoEd25519:
		return "Ed25519"
	case packet.PubKeyAlgoEd448:
		return "Ed448"
	case packet.PubKeyAlgoX25519:
		return "X25519"
	case packet.PubKeyAlgoX448:
		return "X448"
	case packet.PubKeyAlgoDSA:
		return fmt.Sprintf("DSA-%d", bitLen)
	default:
		return fmt.Sprintf("unknown-%d", pub.PubKeyAlgo)
	}
}

// Overwrite replaces the key file identified by fingerprint with new armored data.
func (s *Store) Overwrite(armored string) (string, error) {
	key, err := crypto.NewKeyFromArmored(armored)
	if err != nil {
		return "", fmt.Errorf("keystore: invalid armored key: %w", err)
	}

	fp := key.GetFingerprint()
	path := filepath.Join(s.dir, fp+".asc")

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("keystore: %w", err)
	}
	if err := os.WriteFile(path, []byte(armored), 0600); err != nil {
		return "", fmt.Errorf("keystore: %w", err)
	}

	return fp, nil
}

// getUID returns the primary user ID name from the given entity.
func getUID(entity *openpgp.Entity) string {
	if entity == nil {
		return ""
	}
	for _, id := range entity.Identities {
		if id.UserId != nil {
			return id.UserId.Name
		}
	}
	return ""
}
