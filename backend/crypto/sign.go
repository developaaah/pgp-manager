package crypto

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/ProtonMail/gopenpgp/v3/crypto"
	"github.com/ProtonMail/gopenpgp/v3/profile"
	"github.com/developaaah/pgp-manager/backend/keystore"
	"github.com/developaaah/pgp-manager/backend/model"
)

// SignText signs plaintext and returns a PGP SIGNED MESSAGE (cleartext signature).
// The original text remains readable; only the detached signature block is added.
func SignText(store *keystore.Store, plaintext, fingerprint, passphrase string) model.SignResult {
	armored, err := store.GetArmored(fingerprint)
	if err != nil {
		return model.SignResult{Error: fmt.Errorf("keystore: %w", err).Error()}
	}

	key, err := crypto.NewKeyFromArmored(armored)
	if err != nil {
		return model.SignResult{Error: fmt.Errorf("parse key: %w", err).Error()}
	}

	locked, err := key.IsLocked()
	if err != nil {
		return model.SignResult{Error: fmt.Errorf("inspect key: %w", err).Error()}
	}
	var unlockedKey *crypto.Key
	if locked {
		unlockedKey, err = key.Unlock([]byte(passphrase))
		if err != nil {
			return model.SignResult{Error: fmt.Errorf("unlock key: %w", err).Error()}
		}
	} else {
		unlockedKey = key
	}

	signingKeyRing, err := crypto.NewKeyRing(unlockedKey)
	if err != nil {
		return model.SignResult{Error: fmt.Errorf("create key ring: %w", err).Error()}
	}

	pgpHandle := crypto.PGPWithProfile(profile.RFC4880())
	signHandle, err := pgpHandle.Sign().SigningKeys(signingKeyRing).New()
	if err != nil {
		return model.SignResult{Error: fmt.Errorf("create sign handle: %w", err).Error()}
	}

	// SignCleartext produces -----BEGIN PGP SIGNED MESSAGE----- with the
	// plaintext readable inline. Sign(..., Armor) produces a PGP MESSAGE
	// (opaque binary), which looks identical to encrypted output.
	signedBytes, err := signHandle.SignCleartext([]byte(plaintext))
	if err != nil {
		return model.SignResult{Error: fmt.Errorf("sign: %w", err).Error()}
	}

	return model.SignResult{Armored: string(signedBytes)}
}

// VerifyText verifies a PGP-signed message against all keys in the keystore.
// Handles both cleartext (-----BEGIN PGP SIGNED MESSAGE-----) and
// inline (-----BEGIN PGP MESSAGE-----) signatures.
func VerifyText(store *keystore.Store, signed string) model.VerifyResult {
	keys, err := store.List()
	if err != nil {
		return model.VerifyResult{Error: fmt.Errorf("list keys: %w", err).Error()}
	}

	isCleartext := strings.HasPrefix(strings.TrimSpace(signed), "-----BEGIN PGP SIGNED MESSAGE")

	for _, k := range keys {
		armored, err := store.GetArmored(k.Fingerprint)
		if err != nil {
			continue
		}

		key, err := crypto.NewKeyFromArmored(armored)
		if err != nil {
			continue
		}
		if key.IsPrivate() {
			key, err = key.ToPublic()
			if err != nil {
				continue
			}
		}

		keyRing, err := crypto.NewKeyRing(key)
		if err != nil {
			continue
		}

		pgpHandle := crypto.PGPWithProfile(profile.RFC4880())
		verifyHandle, err := pgpHandle.Verify().VerificationKeys(keyRing).New()
		if err != nil {
			continue
		}

		var signedBy string
		var verified bool

		if isCleartext {
			result, err := verifyHandle.VerifyCleartext([]byte(signed))
			if err != nil {
				slog.Debug("cleartext verify error", "fingerprint", k.Fingerprint, "error", err)
				continue
			}
			if result.SignatureError() != nil || len(result.Signatures) == 0 {
				continue
			}
			verified = true
			if result.Signatures[0].SignedBy != nil {
				signedBy = result.Signatures[0].SignedBy.GetFingerprint()
			} else {
				signedBy = k.Fingerprint
			}
		} else {
			result, err := verifyHandle.VerifyInline([]byte(signed), crypto.Armor)
			if err != nil {
				slog.Debug("inline verify error", "fingerprint", k.Fingerprint, "error", err)
				continue
			}
			if result == nil {
				continue
			}
			sigErr := result.VerifyResult.SignatureError()
			if sigErr != nil || len(result.VerifyResult.Signatures) == 0 {
				continue
			}
			verified = true
			if result.VerifyResult.Signatures[0].SignedBy != nil {
				signedBy = result.VerifyResult.Signatures[0].SignedBy.GetFingerprint()
			} else {
				signedBy = k.Fingerprint
			}
		}

		if verified {
			return model.VerifyResult{
				Valid:    true,
				SignedBy: signedBy,
				UID:      k.PrimaryUID,
				Email:    k.Email,
			}
		}
	}

	return model.VerifyResult{
		Valid: false,
		Error: "signature could not be verified with any known key",
	}
}

// SignFile reads the file at inputPath, signs it using the private key
// identified by fingerprint and passphrase, and writes the cleartext-signed
// output to outputPth (or inputPath + ".asc" if outputPth is empty).
func SignFile(store *keystore.Store, inputPath, outputPth, fingerprint, passphrase string) model.SignResult {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return model.SignResult{Error: fmt.Errorf("read input: %w", err).Error()}
	}
	result := SignText(store, string(data), fingerprint, passphrase)
	if result.Error != "" {
		return result
	}
	if outputPth == "" {
		outputPth = inputPath + ".asc"
	}
	if err := os.WriteFile(outputPth, []byte(result.Armored), 0644); err != nil {
		return model.SignResult{Error: fmt.Errorf("write output: %w", err).Error()}
	}
	return model.SignResult{Armored: result.Armored, OutputPath: outputPth}
}

// VerifyFile reads the file at inputPath and verifies its PGP signature
// against all keys in the keystore.
func VerifyFile(store *keystore.Store, inputPath string) model.VerifyResult {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return model.VerifyResult{Error: fmt.Errorf("read input: %w", err).Error()}
	}
	return VerifyText(store, string(data))
}
