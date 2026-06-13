//go:build windows

// Package clipboard exposes a cheap clipboard change counter where the OS
// provides one, so the monitor does not have to read the clipboard content
// on every poll tick.
package clipboard

import "syscall"

var procGetClipboardSequenceNumber = syscall.NewLazyDLL("user32.dll").NewProc("GetClipboardSequenceNumber")

// ChangeCount returns the Windows clipboard sequence number. The second
// return value reports that change counting is supported on this platform.
func ChangeCount() (int64, bool) {
	r, _, _ := procGetClipboardSequenceNumber.Call()
	return int64(r), true
}
