package main

import goruntime "runtime"

// GetPlatform returns the OS identifier: "darwin", "linux", or "windows".
// Used by the frontend to conditionally show the custom macOS titlebar.
//
//bind
func (a *App) GetPlatform() string {
	return goruntime.GOOS
}
