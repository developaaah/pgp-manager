//go:build darwin

// Package install manages the system-wide installation of PGP Manager
// (macOS: copy the app bundle to /Applications, Linux: desktop entry).
package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// bundlePath returns the path of the .app bundle the running executable
// lives in, or an error when not running from a bundle (wails dev).
func bundlePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	// <bundle>.app/Contents/MacOS/<binary>
	dir := filepath.Dir(filepath.Dir(filepath.Dir(exe)))
	if filepath.Ext(dir) != ".app" {
		return "", fmt.Errorf("install: not running from an app bundle")
	}
	return dir, nil
}

// Supported reports whether a system-wide install is possible.
func Supported() bool {
	_, err := bundlePath()
	return err == nil
}

// Installed reports whether the app already runs from /Applications.
func Installed() bool {
	b, err := bundlePath()
	if err != nil {
		return false
	}
	return strings.HasPrefix(b, "/Applications/")
}

// Install copies the app bundle to /Applications. The icon parameter is
// unused on macOS (the bundle carries its own icon).
func Install(_ []byte) error {
	b, err := bundlePath()
	if err != nil {
		return err
	}
	if strings.HasPrefix(b, "/Applications/") {
		return nil
	}
	target := filepath.Join("/Applications", filepath.Base(b))
	// ditto preserves bundle structure, signatures, and extended attributes.
	if out, err := exec.Command("ditto", b, target).CombinedOutput(); err != nil {
		return fmt.Errorf("install: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
