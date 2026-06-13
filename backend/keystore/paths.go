package keystore

import (
	"fmt"
	"os"

	"github.com/developaaah/pgp-manager/backend/config"
)

// StorageDir returns the directory where keys are stored.
// If cfg.KeysDir is non-empty it is used directly.
// Otherwise keys live next to the config file (default ~/.pgp), so the
// whole directory is portable.
func StorageDir(cfg *config.Config) (string, error) {
	if cfg.KeysDir != "" {
		if err := os.MkdirAll(cfg.KeysDir, 0700); err != nil {
			return "", fmt.Errorf("keystore: %w", err)
		}
		return cfg.KeysDir, nil
	}

	base, err := cfg.Dir()
	if err != nil {
		return "", fmt.Errorf("keystore: %w", err)
	}
	if err := os.MkdirAll(base, 0700); err != nil {
		return "", fmt.Errorf("keystore: %w", err)
	}
	return base, nil
}
