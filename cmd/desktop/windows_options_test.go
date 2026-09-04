package main

import "testing"

func TestReleaseDoesNotExposeWebviewDebugger(t *testing.T) {
	previous := webviewDebugPort
	defer func() { webviewDebugPort = previous }()
	webviewDebugPort = ""
	t.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", "--remote-debugging-port=49223")
	if len(desktopWindowsOptions().AdditionalBrowserArgs) != 0 {
		t.Fatal("release unexpectedly enabled browser debugging")
	}
	webviewDebugPort = "49223" // QA link-time override only.
	if got := desktopWindowsOptions().AdditionalBrowserArgs; len(got) != 2 || got[0] != "--remote-debugging-port=49223" {
		t.Fatal("QA instrumentation missing", got)
	}
}
