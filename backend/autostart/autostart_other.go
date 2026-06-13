//go:build !darwin && !windows && !linux

// Package autostart manages launching PGP Manager at login.
package autostart

import "fmt"

// Supported reports whether login items can be managed.
func Supported() bool {
	return false
}

// Enabled reports whether autostart is active.
func Enabled() (bool, error) {
	return false, nil
}

// SetEnabled is not available on this platform.
func SetEnabled(bool) error {
	return fmt.Errorf("autostart: not supported on this platform")
}
