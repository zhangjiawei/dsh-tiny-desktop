package core

import "testing"

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
