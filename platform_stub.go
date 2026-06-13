//go:build !darwin && !windows && !linux

package main

func registerServices(_ *App) {}

func trayUpdateClipboard(_ int) {}

func prepareWindowForShow() {}
