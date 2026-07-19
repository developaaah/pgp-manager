package crypto

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ProtonMail/gopenpgp/v3/crypto"
	"github.com/ProtonMail/gopenpgp/v3/profile"
	"github.com/developaaah/pgp-manager/backend/keystore"
	"github.com/developaaah/pgp-manager/backend/model"
)

// EncryptFile encrypts the file at inputPath for the given recipient
// fingerprints and writes the armored output to inputPath + ".pgp".
// If signingFingerprint is provided, the input is signed with that private key
// before encryption (sign-then-encrypt).
func EncryptFile(ks *keystore.Store, inputPath string, recipientFingerprints []string, signingFingerprint string) model.FileResult {
	if inputPath == "" {
		return model.FileResult{Error: "input path required"}
	}

	// 1. Input-Datei lesen
	inputData, err := os.ReadFile(inputPath)
	if err != nil {
		return model.FileResult{Error: fmt.Errorf("crypto: read input: %w", err).Error()}
	}

	var signedData []byte
	if signingFingerprint != "" {
		// Sign input data before encryption
		signResult := SignText(ks, string(inputData), signingFingerprint, "")
		if signResult.Error != "" {
			return model.FileResult{Error: fmt.Errorf("crypto: sign: %s", signResult.Error).Error()}
		}
		signedData = []byte(signResult.Armored)
	} else {
		signedData = inputData
	}

	// 2. Recipient-Keys auflösen
	pgpHandle := crypto.PGPWithProfile(profile.RFC4880())
	recipients, err := resolveRecipients(ks, recipientFingerprints)
	if err != nil {
		return model.FileResult{Error: err.Error()}
	}

	// 3. Encrypt (sign-then-encrypt)
	encBuilder := pgpHandle.Encryption()
	for _, k := range recipients {
		encBuilder = encBuilder.Recipient(k)
	}
	encHandle, err := encBuilder.New()
	if err != nil {
		return model.FileResult{Error: fmt.Errorf("crypto: create encryptor: %w", err).Error()}
	}

	pgpMsg, err := encHandle.Encrypt(signedData)
	if err != nil {
		return model.FileResult{Error: fmt.Errorf("crypto: encrypt: %w", err).Error()}
	}

	armored, err := pgpMsg.Armor()
	if err != nil {
		return model.FileResult{Error: fmt.Errorf("crypto: armor: %w", err).Error()}
	}

	// 4. Output schreiben (input.pgp)
	outputPath := inputPath + ".pgp"
	if err := os.WriteFile(outputPath, []byte(armored), 0644); err != nil {
		return model.FileResult{Error: fmt.Errorf("crypto: write output: %w", err).Error()}
	}

	return model.FileResult{OutputPath: outputPath}
}

// DecryptFile decrypts an armored PGP file at inputPath using the private key
// identified by fingerprint and passphrase. The output is written to a path
// derived from inputPath (strip .pgp/.asc/.gpg extension, or append .decrypted).
func DecryptFile(ks *keystore.Store, inputPath, fingerprint, passphrase string) model.FileResult {
	if inputPath == "" {
		return model.FileResult{Error: "input path required"}
	}
	if fingerprint == "" {
		return model.FileResult{Error: "fingerprint required"}
	}

	// 1. Input-Datei lesen
	armoredData, err := os.ReadFile(inputPath)
	if err != nil {
		return model.FileResult{Error: fmt.Errorf("crypto: read input: %w", err).Error()}
	}

	// 2. Private Key laden und entsperren
	privArmored, err := ks.GetArmored(fingerprint)
	if err != nil {
		return model.FileResult{Error: fmt.Errorf("crypto: keystore: %w", err).Error()}
	}

	privKey, err := crypto.NewKeyFromArmored(privArmored)
	if err != nil {
		return model.FileResult{Error: fmt.Errorf("crypto: parse private key: %w", err).Error()}
	}

	locked, err := privKey.IsLocked()
	if err != nil {
		return model.FileResult{Error: fmt.Errorf("crypto: inspect private key: %w", err).Error()}
	}
	if locked {
		privKey, err = privKey.Unlock([]byte(passphrase))
		if err != nil {
			return model.FileResult{Error: fmt.Errorf("crypto: unlock private key: %w", err).Error()}
		}
	}

	// 4. Decrypt — auto-detect armored vs binary
	pgpHandle := crypto.PGPWithProfile(profile.RFC4880())
	decHandle, err := pgpHandle.Decryption().DecryptionKey(privKey).New()
	if err != nil {
		return model.FileResult{Error: fmt.Errorf("crypto: create decryptor: %w", err).Error()}
	}

	encoding := int8(crypto.Armor)
	if !bytes.HasPrefix(armoredData, []byte("-----BEGIN PGP")) {
		encoding = 0 // binary
	}
	result, err := decHandle.Decrypt(armoredData, encoding)
	if err != nil {
		return model.FileResult{Error: fmt.Errorf("crypto: decrypt: %w", err).Error()}
	}

	if result == nil {
		return model.FileResult{Error: "decryption produced no output"}
	}

	// 5. Output-Pfad bestimmen: Extension entfernen oder '.decrypted' anhängen
	ext := filepath.Ext(inputPath)
	var outputPath string
	switch ext {
	case ".pgp", ".asc", ".gpg":
		outputPath = inputPath[:len(inputPath)-len(ext)]
	default:
		outputPath = inputPath + ".decrypted"
	}

	if err := os.WriteFile(outputPath, result.Bytes(), 0600); err != nil {
		return model.FileResult{Error: fmt.Errorf("crypto: write output: %w", err).Error()}
	}

	return model.FileResult{OutputPath: outputPath}
}

// EncryptFileWithOutput encrypts the file at inputPath for the given recipient
// fingerprints and writes the armored output to outputPth (or inputPath + ".pgp"
// if outputPth is empty).
// If signingFingerprint is provided, the input is signed with that private key
// before encryption (sign-then-encrypt).
func EncryptFileWithOutput(ks *keystore.Store, inputPath string, recipientFingerprints []string, outputPth, signingFingerprint string) model.FileResult {
	result := EncryptFile(ks, inputPath, recipientFingerprints, signingFingerprint)
	if result.Error != "" {
		return result
	}
	if outputPth == "" {
		return result
	}
	if result.OutputPath != outputPth {
		if err := os.Rename(result.OutputPath, outputPth); err != nil {
			return model.FileResult{Error: fmt.Errorf("rename output: %w", err).Error()}
		}
		result.OutputPath = outputPth
	}
	return result
}

// DecryptFileWithOutput decrypts an armored PGP file at inputPath using the
// private key identified by fingerprint and passphrase, writing to outputPth
// instead of the auto-derived path.
func DecryptFileWithOutput(ks *keystore.Store, inputPath, outputPth, fingerprint, passphrase string) model.FileResult {
	if inputPath == "" {
		return model.FileResult{Error: "input path required"}
	}
	if fingerprint == "" {
		return model.FileResult{Error: "fingerprint required"}
	}

	// Delegate to DecryptFile for the actual crypto work, then rename output.
	result := DecryptFile(ks, inputPath, fingerprint, passphrase)
	if result.Error != "" {
		return result
	}
	if outputPth == "" {
		return result
	}
	if result.OutputPath != outputPth {
		if err := os.Rename(result.OutputPath, outputPth); err != nil {
			return model.FileResult{Error: fmt.Errorf("rename output: %w", err).Error()}
		}
		result.OutputPath = outputPth
	}
	return result
}
