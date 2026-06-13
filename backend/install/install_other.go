//go:build !darwin && !linux

// Package install manages the system-wide installation of PGP Manager
// (macOS: copy the app bundle to /Applications, Linux: desktop entry).
package install

import "fmt"

// Supported reports whether a system-wide install is possible.
func Supported() bool { return false }

// Installed reports whether the app is installed system-wide.
func Installed() bool { return false }

// Install is not available on this platform.
func Install(_ []byte) error {
	return fmt.Errorf("install: not supported on this platform")
}
