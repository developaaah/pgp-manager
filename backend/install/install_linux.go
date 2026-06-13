//go:build linux

// Package install manages the system-wide installation of PGP Manager
// (macOS: copy the app bundle to /Applications, Linux: desktop entry).
package install

import (
	"fmt"
	"os"
	"path/filepath"
)

func dataHome() (string, error) {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return xdg, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share"), nil
}

func desktopPath() (string, error) {
	data, err := dataHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(data, "applications", "pgp-manager.desktop"), nil
}

func iconPath() (string, error) {
	data, err := dataHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(data, "icons", "pgp-manager.png"), nil
}

// Supported reports whether a system-wide install is possible.
func Supported() bool {
	return true
}

// Installed reports whether the desktop entry exists.
func Installed() bool {
	path, err := desktopPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// Install writes the application icon and a desktop entry pointing at the
// currently running executable, making the app appear in application
// launchers and registering the pgp-manager:// URL scheme.
func Install(icon []byte) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("install: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	icoPath, err := iconPath()
	if err != nil {
		return fmt.Errorf("install: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(icoPath), 0755); err != nil {
		return fmt.Errorf("install: %w", err)
	}
	if err := os.WriteFile(icoPath, icon, 0644); err != nil {
		return fmt.Errorf("install: %w", err)
	}

	deskPath, err := desktopPath()
	if err != nil {
		return fmt.Errorf("install: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(deskPath), 0755); err != nil {
		return fmt.Errorf("install: %w", err)
	}
	entry := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=PGP Manager
Comment=PGP key management and encryption
Exec="%s" %%u
Icon=%s
Terminal=false
Categories=Utility;Security;
MimeType=x-scheme-handler/pgp-manager;
`, exe, icoPath)
	if err := os.WriteFile(deskPath, []byte(entry), 0644); err != nil {
		return fmt.Errorf("install: %w", err)
	}
	return nil
}
