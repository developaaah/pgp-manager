package crypto

import (
	"fmt"

	"github.com/ProtonMail/gopenpgp/v3/crypto"
	"github.com/developaaah/pgp-manager/backend/keystore"
)

// resolveRecipients loads the given fingerprints from the store as public
// encryption keys (private keys are converted via ToPublic — gopenpgp rejects
// locked keys in an encryption keyring). Any fingerprint that cannot be
// resolved fails the whole resolution: silently encrypting to fewer
// recipients than requested must never happen.
func resolveRecipients(ks *keystore.Store, fingerprints []string) ([]*crypto.Key, error) {
	if len(fingerprints) == 0 {
		return nil, fmt.Errorf("crypto: no recipients")
	}
	recipients := make([]*crypto.Key, 0, len(fingerprints))
	for _, fp := range fingerprints {
		armored, err := ks.GetArmored(fp)
		if err != nil {
			return nil, fmt.Errorf("crypto: recipient key %s not found: %w", fp, err)
		}
		key, err := crypto.NewKeyFromArmored(armored)
		if err != nil {
			return nil, fmt.Errorf("crypto: invalid recipient key %s: %w", fp, err)
		}
		if key.IsPrivate() {
			key, err = key.ToPublic()
			if err != nil {
				return nil, fmt.Errorf("crypto: extract public key for %s: %w", fp, err)
			}
		}
		recipients = append(recipients, key)
	}
	return recipients, nil
}
