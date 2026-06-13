//go:build windows

// Package autostart manages launching PGP Manager at login.
package autostart

import (
	"errors"
	"os"

	"golang.org/x/sys/windows/registry"
)

const (
	runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	valueName  = "PGP Manager"
)

// Supported reports whether login items can be managed.
func Supported() bool {
	return true
}

// Enabled reports whether the Run-key entry exists.
func Enabled() (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false, nil
	}
	defer k.Close()
	_, _, err = k.GetStringValue(valueName)
	return err == nil, nil
}

// SetEnabled creates or removes the HKCU Run-key entry.
func SetEnabled(enable bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if enable {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		return k.SetStringValue(valueName, `"`+exe+`"`)
	}
	if err := k.DeleteValue(valueName); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil
}
