//go:build !darwin && !windows

// Package clipboard exposes a cheap clipboard change counter where the OS
// provides one, so the monitor does not have to read the clipboard content
// on every poll tick.
package clipboard

// ChangeCount reports that no change counter is available — the monitor
// falls back to polling the clipboard text.
func ChangeCount() (int64, bool) {
	return 0, false
}
