//go:build darwin

package main

/*
#cgo CFLAGS: -mmacosx-version-min=11.0
#cgo LDFLAGS: -framework Cocoa
void pgp_register_services(void);
void pgp_setup_tray(const void* iconData, int iconLen);
void pgp_tray_set_clipboard(int kind);
void pgp_window_move_to_active_space(void);
void pgp_set_dock_icon_visible(int visible);
*/
import "C"

import (
	"strings"
	"unsafe"

	"github.com/developaaah/pgp-manager/backend/tray/icons"
)

// serviceApp is the App instance receiving NSServices and tray callbacks.
var serviceApp *App

// pgpGoServiceFired is called from Objective-C when the user invokes a
// "PGP: …" text entry in the macOS Services menu.
//
//export pgpGoServiceFired
func pgpGoServiceFired(action *C.char, text *C.char) {
	if serviceApp == nil {
		return
	}
	act := C.GoString(action)
	txt := C.GoString(text)
	go serviceApp.runTextAction(act, txt)
}

// pgpGoFileServiceFired is called from Objective-C when the user invokes a
// "PGP: … File" entry on a Finder selection. Fired once per invocation with
// all selected paths newline-joined.
//
//export pgpGoFileServiceFired
func pgpGoFileServiceFired(action *C.char, paths *C.char) {
	if serviceApp == nil {
		return
	}
	act := C.GoString(action)
	list := strings.Split(C.GoString(paths), "\n")
	go serviceApp.runFileAction(act, list)
}

// pgpGoTrayAction is called from Objective-C when a tray menu item is clicked.
//
//export pgpGoTrayAction
func pgpGoTrayAction(action *C.char) {
	if serviceApp == nil {
		return
	}
	act := C.GoString(action)
	go serviceApp.handleTrayAction(act)
}

func registerServices(a *App) {
	serviceApp = a
	C.pgp_register_services()
	C.pgp_setup_tray(unsafe.Pointer(&icons.TemplatePNG[0]), C.int(len(icons.TemplatePNG)))
	C.pgp_window_move_to_active_space()
	// Wails forces NSApplicationActivationPolicyRegular at startup, which
	// shows a Dock icon even with LSUIElement=true in Info.plist. Reset to
	// Accessory so the app lives only in the menu bar when tray mode is on.
	if a.cfg.StartInTray && !a.needsSetup {
		C.pgp_set_dock_icon_visible(0)
	}
}

// trayUpdateClipboard shows only the tray clipboard actions applying to the
// current clipboard content.
func trayUpdateClipboard(kind int) {
	C.pgp_tray_set_clipboard(C.int(kind))
}

// prepareWindowForShow makes the window appear on the currently active
// Space (macOS) before it is shown. It also re-asserts the activation policy
// because Wails may reset it to Regular internally during WindowShow.
func prepareWindowForShow() {
	C.pgp_window_move_to_active_space()
	if serviceApp != nil && serviceApp.cfg.StartInTray {
		C.pgp_set_dock_icon_visible(0)
	}
}
