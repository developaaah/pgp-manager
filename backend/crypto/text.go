// Package crypto provides PGP text encryption and decryption using gopenpgp v3.
package crypto

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/ProtonMail/gopenpgp/v3/crypto"
	"github.com/ProtonMail/gopenpgp/v3/profile"
	"github.com/developaaah/pgp-manager/backend/keystore"
	"github.com/developaaah/pgp-manager/backend/model"
)

// EncryptText encrypts plaintext for the given recipient fingerprints.
// If signingFingerprint and signingPassphrase are non-empty, the message is
// also signed inline (single PGP MESSAGE block, standard sign+encrypt).
func EncryptText(ks *keystore.Store, plaintext string, recipientFingerprints []string, signingFingerprint, signingPassphrase string) model.EncryptResult {
	pgpHandle := crypto.PGPWithProfile(profile.RFC4880())

	recipients, err := resolveRecipients(ks, recipientFingerprints)
	if err != nil {
		return model.EncryptResult{Error: err.Error()}
	}

	// Build encryption handle — chain Recipient() for each key.
	encBuilder := pgpHandle.Encryption()
	for _, k := range recipients {
		encBuilder = encBuilder.Recipient(k)
	}

	// Integrate signing into the encryption step if requested.
	if signingFingerprint != "" {
		privArmored, err := ks.GetArmored(signingFingerprint)
		if err != nil {
			return model.EncryptResult{Error: fmt.Errorf("signing key not found: %w", err).Error()}
		}
		privKey, err := crypto.NewKeyFromArmored(privArmored)
		if err != nil {
			return model.EncryptResult{Error: fmt.Errorf("invalid signing key: %w", err).Error()}
		}
		locked, err := privKey.IsLocked()
		if err != nil {
			return model.EncryptResult{Error: fmt.Errorf("inspect signing key: %w", err).Error()}
		}
		if locked {
			privKey, err = privKey.Unlock([]byte(signingPassphrase))
			if err != nil {
				return model.EncryptResult{Error: fmt.Errorf("wrong passphrase for signing key: %w", err).Error()}
			}
		}
		encBuilder = encBuilder.SigningKey(privKey)
	}

	encHandle, err := encBuilder.New()
	if err != nil {
		return model.EncryptResult{Error: fmt.Errorf("create encryptor: %w", err).Error()}
	}

	msg, err := encHandle.Encrypt([]byte(plaintext))
	if err != nil {
		return model.EncryptResult{Error: fmt.Errorf("encrypt: %w", err).Error()}
	}

	armoredMsg, err := msg.Armor()
	if err != nil {
		return model.EncryptResult{Error: fmt.Errorf("armor: %w", err).Error()}
	}

	return model.EncryptResult{Armored: armoredMsg}
}

// DecryptText decrypts an armored message. The fingerprint identifies which
// private key to use for decryption. If signed is true, signature verification
// is attempted and the signer's fingerprint is returned.
func DecryptText(ks *keystore.Store, armored, fingerprint, passphrase string, signed bool) model.DecryptResult {
	// Load private key from keystore
	privArmored, err := ks.GetArmored(fingerprint)
	if err != nil {
		return model.DecryptResult{Error: fmt.Errorf("keystore: %w", err).Error()}
	}

	privKey, err := crypto.NewKeyFromArmored(privArmored)
	if err != nil {
		return model.DecryptResult{Error: fmt.Errorf("parse private key: %w", err).Error()}
	}

	// Unlock private key if needed (keys without passphrase are already unlocked).
	locked, err := privKey.IsLocked()
	if err != nil {
		return model.DecryptResult{Error: fmt.Errorf("inspect private key: %w", err).Error()}
	}
	if locked {
		privKey, err = privKey.Unlock([]byte(passphrase))
		if err != nil {
			return model.DecryptResult{Error: fmt.Errorf("unlock private key: %w", err).Error()}
		}
	}

	pgpHandle := crypto.PGPWithProfile(profile.RFC4880())

	// Decrypt the armored message directly (no PGPMessage parsing needed)
	decHandle, err := pgpHandle.Decryption().DecryptionKey(privKey).New()
	if err != nil {
		return model.DecryptResult{Error: fmt.Errorf("create decryptor: %w", err).Error()}
	}

	result, err := decHandle.Decrypt([]byte(armored), crypto.Armor)
	if err != nil {
		return model.DecryptResult{Error: fmt.Errorf("decrypt: %w", err).Error()}
	}

	plaintext := result.String()
	var signedBy string

	// Verify signature if requested
	if signed && len(result.Signatures) > 0 && result.Signatures[0].SignedBy != nil {
		signedBy = result.Signatures[0].SignedBy.GetFingerprint()
	}

	return model.DecryptResult{Plaintext: plaintext, SignedBy: signedBy}
}

// FindDecryptionKey inspects the PKESK headers of an armored PGP MESSAGE to find
// which stored private key is the intended recipient.
// Returns the fingerprint of the matching key, or "" if none found.
func FindDecryptionKey(ks *keystore.Store, armoredMsg string) string {
	block, err := armor.Decode(strings.NewReader(armoredMsg))
	if err != nil {
		return ""
	}
	return matchByKeyIDs(ks, extractKeyIDs(block.Body))
}

// FindDecryptionKeyFromFile reads a PGP file (armored or binary) from disk and
// returns the fingerprint of the stored private key that is the intended recipient.
// Returns "" if the file cannot be read or no matching key is found.
func FindDecryptionKeyFromFile(ks *keystore.Store, filePath string) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	// Try armored first.
	if block, err := armor.Decode(bytes.NewReader(data)); err == nil {
		if fp := matchByKeyIDs(ks, extractKeyIDs(block.Body)); fp != "" {
			return fp
		}
	}
	// Fall back to raw binary packet stream.
	return matchByKeyIDs(ks, extractKeyIDs(bytes.NewReader(data)))
}

// extractKeyIDs reads PGP packets from r and returns all non-zero PKESK key IDs.
func extractKeyIDs(r io.Reader) []uint64 {
	pr := packet.NewReader(r)
	var ids []uint64
	for {
		p, err := pr.Next()
		if err != nil {
			break
		}
		if ek, ok := p.(*packet.EncryptedKey); ok && ek.KeyId != 0 {
			ids = append(ids, ek.KeyId)
		}
	}
	return ids
}

// matchByKeyIDs returns the fingerprint of the first stored private key whose
// primary key or any subkey matches one of the given 64-bit key IDs.
func matchByKeyIDs(ks *keystore.Store, keyIDs []uint64) string {
	if len(keyIDs) == 0 {
		return ""
	}
	keys, err := ks.List()
	if err != nil {
		return ""
	}
	for _, ki := range keys {
		if !ki.IsPrivate {
			continue
		}
		arm, err := ks.GetArmored(ki.Fingerprint)
		if err != nil {
			continue
		}
		key, err := crypto.NewKeyFromArmored(arm)
		if err != nil {
			continue
		}
		entity := key.GetEntity()
		if entity == nil {
			continue
		}
		for _, keyID := range keyIDs {
			if entity.PrimaryKey.KeyId == keyID {
				return ki.Fingerprint
			}
			for _, sub := range entity.Subkeys {
				if sub.PublicKey.KeyId == keyID {
					return ki.Fingerprint
				}
			}
		}
	}
	return ""
}
