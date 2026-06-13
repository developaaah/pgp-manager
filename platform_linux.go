//go:build linux

package main

import (
	"github.com/developaaah/pgp-manager/backend/tray"
)

// registerServices starts the system tray (StatusNotifierItem via D-Bus).
func registerServices(a *App) {
	tray.Run(a.handleTrayAction)
}

// trayUpdateClipboard shows only the tray clipboard actions applying to the
// current clipboard content.
func trayUpdateClipboard(kind int) {
	tray.SetClipboardKind(kind)
}

// prepareWindowForShow is a no-op on Linux (workspace placement is up to
// the window manager).
func prepareWindowForShow() {}
