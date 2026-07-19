//go:build windows

// Package services installs file-manager context-menu entries that launch
// PGP Manager with "--context <action> <path>..." arguments. macOS needs no
// installation (NSServices from Info.plist); on Windows the entries live in
// the per-user registry, on Linux in the user's file-manager config dirs.
package services

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// baseKey is the per-user Explorer context-menu entry for all file types.
// HKCU requires no elevation. On Windows 11 the entries appear under
// "Show more options" (the classic menu).
const baseKey = `HKCU\Software\Classes\*\shell\PGPManager`

var menuEntries = []struct{ key, title, action string }{
	{"01encrypt", "Encrypt", "encrypt-file"},
	{"02decrypt", "Decrypt", "decrypt-file"},
	{"03sign", "Sign", "sign-file"},
	{"04verify", "Verify Signature", "verify-file"},
}

func regCmd(args ...string) error {
	cmd := exec.Command("reg", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("services: reg %s: %s: %w", args[0], strings.TrimSpace(string(out)), err)
	}
	return nil
}

// Supported reports whether context-menu installation applies to this OS.
func Supported() bool { return true }

// Installed reports whether the Explorer context-menu entries exist.
func Installed() bool {
	cmd := exec.Command("reg", "query", baseKey)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run() == nil
}

// Install writes the cascading "PGP Manager" Explorer context menu for the
// currently running executable.
func Install() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("services: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	if err := regCmd("add", baseKey, "/v", "MUIVerb", "/d", "PGP Manager", "/f"); err != nil {
		return err
	}
	// An empty SubCommands value makes Explorer read the sub-entries from
	// the shell subkey (cascading menu without a COM handler).
	if err := regCmd("add", baseKey, "/v", "SubCommands", "/d", "", "/f"); err != nil {
		return err
	}
	if err := regCmd("add", baseKey, "/v", "Icon", "/d", exe, "/f"); err != nil {
		return err
	}
	for _, e := range menuEntries {
		sub := baseKey + `\shell\` + e.key
		if err := regCmd("add", sub, "/v", "MUIVerb", "/d", e.title, "/f"); err != nil {
			return err
		}
		// Explorer starts one process per selected file; the app batches
		// encrypt requests arriving in quick succession.
		command := fmt.Sprintf(`"%s" --context %s "%%1"`, exe, e.action)
		if err := regCmd("add", sub+`\command`, "/ve", "/d", command, "/f"); err != nil {
			return err
		}
	}
	return nil
}

// Uninstall removes the Explorer context-menu entries.
func Uninstall() error {
	return regCmd("delete", baseKey, "/f")
}
