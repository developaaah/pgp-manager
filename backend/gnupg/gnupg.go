// Package gnupg provides integration with the system GnuPG installation.
package gnupg

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// HomeDir returns the GnuPG home directory.
// Respects $GNUPGHOME; falls back to ~/.gnupg (Unix) or %APPDATA%\gnupg (Windows).
func HomeDir() string {
	if h := os.Getenv("GNUPGHOME"); h != "" {
		return h
	}
	switch runtime.GOOS {
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "gnupg")
		}
		return filepath.Join(os.Getenv("USERPROFILE"), ".gnupg")
	default:
		return filepath.Join(os.Getenv("HOME"), ".gnupg")
	}
}

// IsAvailable returns true if the gpg binary is found in PATH.
func IsAvailable() bool {
	_, err := exec.LookPath("gpg")
	return err == nil
}

// ExportPublicKeys runs "gpg --export --armor" and returns one armored block per key.
// Returns nil, nil when the keyring is empty. Returns an error only on gpg failure.
func ExportPublicKeys(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "gpg", "--export", "--armor")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return splitArmoredBlocks(string(out), "-----BEGIN PGP PUBLIC KEY BLOCK-----", "-----END PGP PUBLIC KEY BLOCK-----"), nil
}

// ExportSecretKeys runs "gpg --export-secret-keys --armor" and returns one armored
// block per private key. The exported blocks are still passphrase-encrypted
// (the passphrase is not removed), so they can be parsed and stored safely.
// Returns nil, nil when no private keys exist. Returns an error only on gpg failure.
func ExportSecretKeys(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "gpg", "--export-secret-keys", "--armor")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return splitArmoredBlocks(string(out), "-----BEGIN PGP PRIVATE KEY BLOCK-----", "-----END PGP PRIVATE KEY BLOCK-----"), nil
}

// splitArmoredBlocks splits concatenated PGP armored blocks into individual strings.
func splitArmoredBlocks(data, beginMarker, endMarker string) []string {
	var blocks []string
	remaining := data
	for {
		start := strings.Index(remaining, beginMarker)
		if start < 0 {
			break
		}
		end := strings.Index(remaining[start:], endMarker)
		if end < 0 {
			break
		}
		end += start + len(endMarker)
		blocks = append(blocks, strings.TrimSpace(remaining[start:end]))
		remaining = remaining[end:]
	}
	return blocks
}
