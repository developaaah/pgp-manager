// Package icons embeds the tray icon assets (carbon "ibm--cloud--key-protect").
// Sources live in build/tray/.
package icons

import _ "embed"

// TemplatePNG is the black/alpha 32px icon used as a macOS template image.
//
//go:embed tray-black-32.png
var TemplatePNG []byte

// WhitePNG is the white/alpha 32px icon used for the Linux tray.
//
//go:embed tray-white-32.png
var WhitePNG []byte

// WhiteICO is the white icon wrapped as ICO for the Windows tray.
//
//go:embed tray-white.ico
var WhiteICO []byte
