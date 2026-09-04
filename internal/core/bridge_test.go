package core

import "testing"

func TestWindowsControlStatusMessage(t *testing.T) {
	// Wails beta.16 windowsWebviewWindow.processMessage supplies Origin and
	// TopOrigin, but leaves IsMainFrame at its zero value. Replay that message.
	if !TrustedControlMessage("windows", "control", "http://wails.localhost/#overview", "http://wails.localhost/#overview", false) {
		t.Fatal("Windows status message rejected: control stays connecting; data and logs never arrive")
	}
}

func TestWindowsHashNavigationInFlight(t *testing.T) {
	// WebMessageReceived.Source is a message-time URL; GetSource is queried
	// later by Wails. A same-document hash navigation can happen between them.
	for _, urls := range [][2]string{
		{"http://wails.localhost/", "http://wails.localhost/#settings"},
		{"http://wails.localhost/#settings", "http://wails.localhost/#overview"},
	} {
		if !TrustedControlMessage("windows", "control", urls[0], urls[1], false) {
			t.Fatal("same-document navigation dropped a privileged settings request")
		}
	}
	for _, top := range []string{"http://wails.localhost/index.html#settings", "wails://localhost/#settings", "http://wails.localhost/?x=1#settings", "https://evil.test/#settings"} {
		if TrustedControlMessage("windows", "control", "http://wails.localhost/#settings", top, false) {
			t.Fatal("hash normalisation admitted a different document", top)
		}
	}
}

func TestPlatformControlBoundary(t *testing.T) {
	for _, platform := range []string{"darwin", "windows", "linux"} {
		origin := "wails://localhost/#settings"
		if platform == "windows" {
			origin = "http://wails.localhost/#settings"
		}
		if !TrustedControlMessage(platform, "control", origin, origin, platform == "darwin") {
			t.Errorf("%s control rejected", platform)
		}
		for _, hostile := range []string{"https://evil.test", "http://127.0.0.1:3080/", "wails://localhost/evil.html", ""} {
			if TrustedControlMessage(platform, "control", hostile, origin, true) || TrustedControlMessage(platform, "workspace", origin, origin, true) {
				t.Errorf("%s accepted untrusted message", platform)
			}
		}
	}
	if TrustedControlMessage("windows", "control", "http://wails.localhost/", "https://evil.test", true) ||
		TrustedControlMessage("windows", "control", "http://wails.localhost/", "", true) ||
		TrustedControlMessage("darwin", "control", "wails://localhost/", "wails://localhost/", false) {
		t.Fatal("missing or mismatched frame information accepted")
	}
}

func TestControlNavigationKeepsBridge(t *testing.T) {
	for _, origin := range []string{"wails://localhost/", "wails://localhost/#overview", "http://wails.localhost/#settings"} {
		if !TrustedControlOrigin("control", origin, true) {
			t.Errorf("control navigation broke actions: %s", origin)
		}
	}
}
func TestControlBridgeRejectsUntrusted(t *testing.T) {
	for _, origin := range []string{"https://evil.test", "wails://localhost.evil/", "wails://user@localhost/", "http://wails.localhost:3080/", "http://127.0.0.1:3080/", "wails://localhost/evil.html"} {
		if TrustedControlOrigin("control", origin, true) {
			t.Errorf("untrusted origin accepted: %s", origin)
		}
	}
	if TrustedControlOrigin("workspace", "wails://localhost/", true) || TrustedControlOrigin("control", "wails://localhost/", false) {
		t.Fatal("wrong window/frame accepted")
	}
}
