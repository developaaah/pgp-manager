package crypto

import (
	gocrypto "crypto"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/ProtonMail/go-crypto/openpgp/packet"
	pgpcrypto "github.com/ProtonMail/gopenpgp/v3/crypto"
	"github.com/ProtonMail/gopenpgp/v3/profile"
	"github.com/developaaah/pgp-manager/backend/keystore"
)

// GenerateKey creates a new PGP key pair and imports it into the store.
// keyType: "rsa2048", "rsa3072" (default), "rsa4096", "ed25519".
// expiryYears: "0" = no expiry, "1".."n" = expires in n years (0 secs = no expiry).
func GenerateKey(ks *keystore.Store, pgpHandle *pgpcrypto.PGPHandle, name, email, passphrase, keyType, expiryYears string) error {
	var key *pgpcrypto.Key
	var err error

	lifetimeSecs := expiryToSecs(expiryYears)

	switch keyType {
	case "rsa2048":
		h := pgpcrypto.PGPWithProfile(rsa2048Profile())
		key, err = h.KeyGeneration().AddUserId(name, email).Lifetime(lifetimeSecs).New().GenerateKey()
	case "rsa4096":
		key, err = pgpHandle.KeyGeneration().AddUserId(name, email).Lifetime(lifetimeSecs).New().GenerateKeyWithSecurity(1)
	case "ed25519":
		key, err = pgpHandle.KeyGeneration().
			OverrideProfileAlgorithm(pgpcrypto.KeyGenerationCurve25519Legacy).
			AddUserId(name, email).Lifetime(lifetimeSecs).New().GenerateKey()
	default: // "rsa3072" or ""
		key, err = pgpHandle.KeyGeneration().AddUserId(name, email).Lifetime(lifetimeSecs).New().GenerateKey()
	}

	if err != nil {
		return fmt.Errorf("keygen: generate: %w", err)
	}

	if passphrase != "" {
		key, err = pgpHandle.LockKey(key, []byte(passphrase))
		if err != nil {
			return fmt.Errorf("keygen: lock: %w", err)
		}
	}

	armored, err := key.Armor()
	if err != nil {
		return fmt.Errorf("keygen: armor: %w", err)
	}

	fp, err := ks.Import(armored)
	if err != nil {
		slog.Warn("keygen: import failed", "name", name, "email", email, "error", err)
		return fmt.Errorf("keygen: import: %w", err)
	}

	slog.Debug("keygen: key imported", "fingerprint", fp, "name", name, "email", email, "keyType", keyType)
	return nil
}

// expiryToSecs converts a year-count string ("0", "1", "2", …) to seconds.
// "0" or empty returns 0 (no expiry).
func expiryToSecs(years string) int32 {
	n, _ := strconv.Atoi(years)
	if n <= 0 {
		return 0
	}
	return int32(n) * 365 * 24 * 3600
}

func rsa2048Profile() *profile.Custom {
	return &profile.Custom{
		SetKeyAlgorithm: func(cfg *packet.Config, _ int8) {
			cfg.Algorithm = packet.PubKeyAlgoRSA
			cfg.RSABits = 2048
		},
		Hash:                 gocrypto.SHA256,
		CipherEncryption:     packet.CipherAES256,
		CompressionAlgorithm: packet.CompressionZLIB,
	}
}
