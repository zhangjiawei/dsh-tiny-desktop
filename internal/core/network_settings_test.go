package core

import (
	"reflect"
	"strings"
	"testing"
)

func TestTrustedAuthoritiesAcceptExactDNSIPv4IPv6AndPorts(t *testing.T) {
	cases := map[string]string{
		"Example.COM":      "example.com",
		"example.com:8443": "example.com:8443",
		"192.168.1.2":      "192.168.1.2",
		"[fd00::1]":        "[fd00::1]",
		"[FD00::1]:8443":   "[fd00::1]:8443",
		"127.0.0.1:3080":   "127.0.0.1:3080",
	}
	for input, want := range cases {
		if got, err := normalizeTrustedAuthority(input); err != nil || got != want {
			t.Errorf("normalizeTrustedAuthority(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
}

func TestTrustedAuthoritiesRejectBroadOrURLLikeValues(t *testing.T) {
	invalid := []string{
		"", "https://example.com", "example.com/path", "*.example.com",
		"user@example.com", "例子.com", "example.com:0", "example.com:65536",
		"example.com:0443", "fd00::1", "[fd00::1]tail", "example.com.",
		"example.com:", "[fd00::1]:",
	}
	for _, input := range invalid {
		if _, err := normalizeTrustedAuthority(input); err == nil {
			t.Errorf("accepted invalid trusted authority %q", input)
		}
	}
}

func TestPublicURLIsHTTPSOriginAndAutomaticallyTrusted(t *testing.T) {
	for input, want := range map[string]string{
		"https://zgpc.zjwcf.us.ci":  "https://zgpc.zjwcf.us.ci",
		"https://example.com:8443/": "https://example.com:8443",
		"https://[fd00::1]:8443":    "https://[fd00::1]:8443",
	} {
		if got, err := normalizePublicURL(input); err != nil || got != want {
			t.Errorf("normalizePublicURL(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	for _, input := range []string{
		"http://example.com", "https://example.com/dsh", "https://example.com?q=1",
		"https://example.com/#x", "https://user@example.com", "//example.com",
	} {
		if _, err := normalizePublicURL(input); err == nil {
			t.Errorf("accepted invalid public URL %q", input)
		}
	}
	s := Defaults()
	s.TrustedHosts = "proxy.internal\nZGPC.ZJWCF.US.CI"
	s.PublicURL = "https://zgpc.zjwcf.us.ci"
	got, err := s.effectiveTrustedAuthorities()
	want := []string{"proxy.internal", "zgpc.zjwcf.us.ci"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("effective trusted authorities = %v, %v; want %v", got, err, want)
	}
}

func TestLANBindingIsIndependentAndLegacySettingMigrates(t *testing.T) {
	p, _ := NewPaths(t.TempDir())
	legacy := `{"port":3080,"proxy":"","lan":true,"hideOnClose":true,"trayOnly":true,"autoStart":true,"alwaysOnTop":false,"language":"system","runtimeMode":"managed","dshChannel":"recommended","fixedDshVersion":"0.1.2-rc.1","extraArgs":"--trusted-host 172.16.8.136","command":"pnpm dlx @deepseek-ai/dsh@0.1.2-rc.1 web","registry":"https://registry.npmmirror.com","startupMinutes":60,"width":1280,"height":840}`
	if err := AtomicWrite(p.Root+"/settings.json", []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := p.LoadSettings()
	if err != nil || loaded.LANAddress != "172.16.8.136" || loaded.ExtraArgs != "" {
		t.Fatalf("legacy LAN migration = %+v, %v", loaded, err)
	}
	loaded.TrustedHosts = "zgpc.zjwcf.us.ci"
	loaded.PublicURL = "https://zgpc.zjwcf.us.ci"
	if err = loaded.Validate(); err != nil {
		t.Fatal(err)
	}
	args := appendTrustedAuthorities([]string{"web", "--trusted-host", "zgpc.zjwcf.us.ci"}, "zgpc.zjwcf.us.ci", loaded.LANAddress)
	if got := strings.Join(args, " "); got != "web --trusted-host zgpc.zjwcf.us.ci --trusted-host 172.16.8.136" {
		t.Fatalf("trusted arguments = %q", got)
	}
}

func TestPublicShareURLUsesLiveTokenWithoutPersistingIt(t *testing.T) {
	p, _ := NewPaths(t.TempDir())
	s := Defaults()
	s.PublicURL = "https://zgpc.zjwcf.us.ci"
	m := NewManager(p, s)
	m.phase = "running"
	m.port = 3080
	m.launch = "http://127.0.0.1:3080/?token=secret-for-explicit-share-only"
	got, err := m.PublicShareURL()
	if err != nil || got != "https://zgpc.zjwcf.us.ci?token=secret-for-explicit-share-only" {
		t.Fatalf("public share URL = %q, %v", got, err)
	}
	snapshot := m.Snapshot()
	if strings.Contains(snapshot.Error+strings.Join(logTexts(snapshot.Logs), ""), "secret-for-explicit") {
		t.Fatal("snapshot leaked public share token")
	}
	m.activeSettings.PublicURL = ""
	if _, err = m.PublicShareURL(); err == nil {
		t.Fatal("public share URL succeeded without a configured origin")
	}
}

func logTexts(lines []LogLine) []string {
	result := make([]string, len(lines))
	for index := range lines {
		result[index] = lines[index].Text
	}
	return result
}
