package crypto

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	openpgp2 "github.com/ProtonMail/go-crypto/openpgp/v2"
	pgpcrypto "github.com/ProtonMail/gopenpgp/v3/crypto"
)

// AddSubkey generates a new signing or encryption subkey for an existing private key
// and returns the complete updated armored key.
//
// subkeyType: "sign" | "encrypt"
// expiryYears: 0 = no expiry
func AddSubkey(armoredPrivKey, subkeyType, passphrase string, expiryYears int) (string, error) {
	entities, err := openpgp2.ReadArmoredKeyRing(strings.NewReader(armoredPrivKey))
	if err != nil {
		return "", fmt.Errorf("subkeys: parse key: %w", err)
	}
	if len(entities) == 0 {
		return "", fmt.Errorf("subkeys: no keys found")
	}
	entity := entities[0]

	// Decrypt private keys (no-op if key has no passphrase).
	if err := entity.DecryptPrivateKeys([]byte(passphrase)); err != nil {
		return "", fmt.Errorf("subkeys: unlock: %w", err)
	}

	cfg := buildSubkeyConfig(expiryYears)

	switch subkeyType {
	case "sign":
		if err := entity.AddSigningSubkey(cfg); err != nil {
			return "", fmt.Errorf("subkeys: add signing subkey: %w", err)
		}
	case "encrypt":
		if err := entity.AddEncryptionSubkey(cfg); err != nil {
			return "", fmt.Errorf("subkeys: add encryption subkey: %w", err)
		}
	default:
		return "", fmt.Errorf("subkeys: unknown subkey type %q", subkeyType)
	}

	// Serialize while keys are still unlocked, then re-lock via gopenpgp.
	return serializeEntityArmored(entity, passphrase)
}

// RevokeSubkey revokes a subkey of an existing private key and returns the updated armored key.
func RevokeSubkey(armoredPrivKey, subkeyFingerprint, passphrase string) (string, error) {
	entities, err := openpgp2.ReadArmoredKeyRing(strings.NewReader(armoredPrivKey))
	if err != nil {
		return "", fmt.Errorf("subkeys: parse key: %w", err)
	}
	if len(entities) == 0 {
		return "", fmt.Errorf("subkeys: no keys found")
	}
	entity := entities[0]

	if err := entity.DecryptPrivateKeys([]byte(passphrase)); err != nil {
		return "", fmt.Errorf("subkeys: unlock: %w", err)
	}

	fpLower := strings.ToLower(subkeyFingerprint)
	var found bool
	for i := range entity.Subkeys {
		if strings.ToLower(hex.EncodeToString(entity.Subkeys[i].PublicKey.Fingerprint)) == fpLower {
			if err := entity.Subkeys[i].Revoke(packet.NoReason, "", &packet.Config{}); err != nil {
				return "", fmt.Errorf("subkeys: revoke: %w", err)
			}
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("subkeys: subkey %s not found", subkeyFingerprint)
	}

	return serializeEntityArmored(entity, passphrase)
}

// serializeEntityArmored writes the entity (with unlocked keys) to an ASCII-armored
// PGP key block. SerializePrivate re-signs all identity/subkey bindings, so the
// entity's private keys must be unlocked before calling.
// After serialization the result is re-parsed and optionally re-locked via gopenpgp.
func serializeEntityArmored(entity *openpgp2.Entity, passphrase string) (string, error) {
	// Serialize binary PGP packets while keys are still unlocked.
	var binBuf bytes.Buffer
	if err := entity.SerializePrivate(&binBuf, nil); err != nil {
		return "", fmt.Errorf("subkeys: serialize: %w", err)
	}

	// Wrap in ASCII armor.
	var armoredBuf bytes.Buffer
	aw, err := armor.Encode(&armoredBuf, "PGP PRIVATE KEY BLOCK", nil)
	if err != nil {
		return "", fmt.Errorf("subkeys: armor encode: %w", err)
	}
	if _, err := aw.Write(binBuf.Bytes()); err != nil {
		aw.Close()
		return "", fmt.Errorf("subkeys: armor write: %w", err)
	}
	if err := aw.Close(); err != nil {
		return "", fmt.Errorf("subkeys: armor close: %w", err)
	}

	// Re-parse and re-lock with gopenpgp if a passphrase was supplied.
	key, err := pgpcrypto.NewKeyFromArmored(armoredBuf.String())
	if err != nil {
		return "", fmt.Errorf("subkeys: re-parse: %w", err)
	}
	if passphrase != "" {
		key, err = pgpcrypto.PGP().LockKey(key, []byte(passphrase))
		if err != nil {
			return "", fmt.Errorf("subkeys: re-lock: %w", err)
		}
	}
	return key.Armor()
}

// buildSubkeyConfig returns a packet.Config for subkey generation with the given expiry.
func buildSubkeyConfig(expiryYears int) *packet.Config {
	cfg := &packet.Config{
		Algorithm: packet.PubKeyAlgoRSA,
		RSABits:   3072,
	}
	if expiryYears > 0 {
		cfg.KeyLifetimeSecs = uint32(expiryYears) * 365 * 24 * 3600
	}
	return cfg
}
