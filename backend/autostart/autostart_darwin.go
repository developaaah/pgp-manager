//go:build darwin

// Package autostart manages launching PGP Manager at login.
package autostart

/*
#cgo CFLAGS: -mmacosx-version-min=11.0
#cgo LDFLAGS: -framework ServiceManagement -framework Foundation

// Implemented in autostart_bridge_darwin.m
int pgp_autostart_supported(void);
int pgp_autostart_status(void);
int pgp_autostart_set(int enable);
*/
import "C"

import "fmt"

// Supported reports whether login items can be managed (SMAppService needs
// macOS 13+ and a proper app bundle).
func Supported() bool {
	return C.pgp_autostart_supported() == 1
}

// Enabled reports whether the app is registered as a login item.
func Enabled() (bool, error) {
	switch C.pgp_autostart_status() {
	case 1:
		return true, nil
	case 0:
		return false, nil
	default:
		return false, fmt.Errorf("autostart: requires macOS 13 or later")
	}
}

// SetEnabled registers or unregisters the app as a login item.
func SetEnabled(enable bool) error {
	v := 0
	if enable {
		v = 1
	}
	switch C.pgp_autostart_set(C.int(v)) {
	case 0:
		return nil
	case -1:
		return fmt.Errorf("autostart: requires macOS 13 or later")
	default:
		return fmt.Errorf("autostart: registration failed (app must run from a proper .app bundle)")
	}
}
