//go:build windows || linux

// Package tray provides the Windows/Linux system tray via fyne.io/systray.
// (macOS uses a native NSStatusItem — see platform_bridge_darwin.m.)
package tray

import (
	"runtime"
	"sync"

	"fyne.io/systray"
	"github.com/developaaah/pgp-manager/backend/tray/icons"
)

// Clipboard kinds — mirror the values used by the clipboard monitor.
const (
	KindNone    = 0
	KindText    = 1
	KindMessage = 2
	KindSigned  = 3
	KindKey     = 4
)

var (
	mu        sync.Mutex
	itEncrypt *systray.MenuItem
	itSign    *systray.MenuItem
	itDecrypt *systray.MenuItem
	itVerify  *systray.MenuItem
	itImport  *systray.MenuItem
	lastKind  = KindNone
)

// Run starts the system tray in a background goroutine. onAction receives
// the tray menu actions ("open", "encrypt-clipboard", "decrypt-clipboard",
// "sign-clipboard", "verify-clipboard", "import-clipboard", "quit").
func Run(onAction func(action string)) {
	go systray.Run(func() {
		if runtime.GOOS == "windows" {
			systray.SetIcon(icons.WhiteICO)
		} else {
			systray.SetIcon(icons.WhitePNG)
		}
		systray.SetTooltip("PGP Manager")

		open := systray.AddMenuItem("Open PGP Manager", "")
		systray.AddSeparator()

		mu.Lock()
		itEncrypt = systray.AddMenuItem("Encrypt Clipboard", "")
		itSign = systray.AddMenuItem("Sign Clipboard", "")
		itDecrypt = systray.AddMenuItem("Decrypt Clipboard", "")
		itVerify = systray.AddMenuItem("Verify Clipboard", "")
		itImport = systray.AddMenuItem("Import Key from Clipboard", "")
		kind := lastKind
		mu.Unlock()

		systray.AddSeparator()
		quit := systray.AddMenuItem("Quit PGP Manager", "")

		SetClipboardKind(kind)

		handle := func(item *systray.MenuItem, action string) {
			go func() {
				for range item.ClickedCh {
					onAction(action)
				}
			}()
		}
		handle(open, "open")
		handle(itEncrypt, "encrypt-clipboard")
		handle(itSign, "sign-clipboard")
		handle(itDecrypt, "decrypt-clipboard")
		handle(itVerify, "verify-clipboard")
		handle(itImport, "import-clipboard")
		handle(quit, "quit")
	}, nil)
}

// SetClipboardKind shows only the clipboard actions that apply to the
// current clipboard content.
func SetClipboardKind(kind int) {
	mu.Lock()
	defer mu.Unlock()
	lastKind = kind
	if itEncrypt == nil { // tray not ready yet — applied in Run
		return
	}
	setVisible(itEncrypt, kind == KindText)
	setVisible(itSign, kind == KindText)
	setVisible(itDecrypt, kind == KindMessage)
	setVisible(itVerify, kind == KindSigned)
	setVisible(itImport, kind == KindKey)
}

func setVisible(item *systray.MenuItem, visible bool) {
	if visible {
		item.Show()
	} else {
		item.Hide()
	}
}
