package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"strconv"
)

func desktopWindowsOptions() application.WindowsOptions {
	options := application.WindowsOptions{}
	if port, err := strconv.Atoi(webviewDebugPort); err == nil && port >= 1024 && port <= 65535 {
		// Wails' internal loader deliberately clears WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS.
		// Native QA must pass its compile-time-only instrumentation via Wails options.
		options.AdditionalBrowserArgs = []string{"--remote-debugging-port=" + strconv.Itoa(port), "--remote-debugging-address=127.0.0.1"}
	}
	return options
}
