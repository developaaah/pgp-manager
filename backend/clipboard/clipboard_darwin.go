//go:build darwin

// Package clipboard exposes a cheap clipboard change counter where the OS
// provides one, so the monitor does not have to read the clipboard content
// on every poll tick.
package clipboard

/*
#cgo LDFLAGS: -framework AppKit
long pgp_clipboard_change_count(void);
*/
import "C"

// ChangeCount returns NSPasteboard's changeCount. The second return value
// reports that change counting is supported on this platform.
func ChangeCount() (int64, bool) {
	return int64(C.pgp_clipboard_change_count()), true
}
