package crypto

import (
	"github.com/ProtonMail/gopenpgp/v3/crypto"
	"github.com/developaaah/pgp-manager/backend/keystore"
)

// KeyLocked reports whether the stored private key is passphrase-protected.
// Unreadable or invalid keys report false — the subsequent operation surfaces
// the real error.
func KeyLocked(ks *keystore.Store, fingerprint string) bool {
	armored, err := ks.GetArmored(fingerprint)
	if err != nil {
		return false
	}
	key, err := crypto.NewKeyFromArmored(armored)
	if err != nil {
		return false
	}
	locked, err := key.IsLocked()
	return err == nil && locked
}
