//go:build !windows && !linux

// Package services installs file-manager context-menu entries that launch
// PGP Manager with "--context <action> <path>..." arguments. macOS needs no
// installation (NSServices from Info.plist); on Windows the entries live in
// the per-user registry, on Linux in the user's file-manager config dirs.
package services

import "fmt"

// Supported reports whether context-menu installation applies to this OS.
// macOS registers its Services automatically via Info.plist.
func Supported() bool { return false }

// Installed reports whether context-menu entries exist.
func Installed() bool { return false }

// Install is not needed on this platform.
func Install() error {
	return fmt.Errorf("services: context-menu install not needed on this platform")
}

// Uninstall is not needed on this platform.
func Uninstall() error {
	return fmt.Errorf("services: context-menu install not needed on this platform")
}
