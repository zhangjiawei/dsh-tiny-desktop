//go:build !darwin

package main

import "github.com/wailsapp/wails/v3/pkg/application"

const loginAutostartIdentifier = "com.zhangjiawei.dsh-tiny-desktop"

func enableLoginLaunch(app *application.App, hidden bool) error {
	opts := application.AutostartOptions{Identifier: loginAutostartIdentifier}
	if hidden {
		opts.Arguments = []string{"--hidden"}
	}
	return app.Autostart.EnableWithOptions(opts)
}

func disableLoginLaunch(app *application.App) error {
	return app.Autostart.Disable()
}
