//go:build !windows

package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"io/fs"
)

func installSettingsIcon(_ *application.WebviewWindow, _ fs.FS, _ string) (func(), error) {
	// Linux uses LinuxWindow.Icon. macOS has no per-window Dock icon: keep the
	// app's shared Dock identity and use a gear in the settings page/favicon.
	return func() {}, nil
}
