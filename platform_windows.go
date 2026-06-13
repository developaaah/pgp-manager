//go:build windows

package main

import (
	"github.com/developaaah/pgp-manager/backend/tray"
)

// registerServices starts the system tray. (The pgp-manager:// URL scheme is
// no longer registered — toast action buttons were removed together with
// the detection notifications.)
func registerServices(a *App) {
	tray.Run(a.handleTrayAction)
}

// trayUpdateClipboard shows only the tray clipboard actions applying to the
// current clipboard content.
func trayUpdateClipboard(kind int) {
	tray.SetClipboardKind(kind)
}

// prepareWindowForShow is a no-op on Windows.
func prepareWindowForShow() {}
