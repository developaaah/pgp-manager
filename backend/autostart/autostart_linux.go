//go:build linux

// Package autostart manages launching PGP Manager at login.
package autostart

import (
	"fmt"
	"os"
	"path/filepath"
)

// desktopFile returns the XDG autostart entry path.
func desktopFile() (string, error) {
	cfg := os.Getenv("XDG_CONFIG_HOME")
	if cfg == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		cfg = filepath.Join(home, ".config")
	}
	return filepath.Join(cfg, "autostart", "pgp-manager.desktop"), nil
}

// Supported reports whether login items can be managed.
func Supported() bool {
	return true
}

// Enabled reports whether the XDG autostart entry exists.
func Enabled() (bool, error) {
	path, err := desktopFile()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	return err == nil, nil
}

// SetEnabled creates or removes the XDG autostart .desktop entry.
func SetEnabled(enable bool) error {
	path, err := desktopFile()
	if err != nil {
		return err
	}
	if !enable {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=PGP Manager
Exec="%s"
X-GNOME-Autostart-enabled=true
`, exe)
	return os.WriteFile(path, []byte(content), 0644)
}
