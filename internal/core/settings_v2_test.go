package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCommandArgumentsAndManagedBoundary(t *testing.T) {
	got, err := ParseCommand(`pnpm dlx --config.registry=https://registry.npmjs.org "@deepseek-ai/dsh@0.1.2-rc.1" web`)
	want := []string{"pnpm", "dlx", "--config.registry=https://registry.npmjs.org", "@deepseek-ai/dsh@0.1.2-rc.1", "web"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("%v %v", got, err)
	}
	got, err = ParseCommand(`"C:\Program Files\nodejs\node.exe" "C:\runtime\cli.js" web`)
	if err != nil || got[0] != `C:\Program Files\nodejs\node.exe` {
		t.Fatalf("Windows quoting: %v %v", got, err)
	}
	for _, s := range []string{`pnpm dlx dsh web --port=80`, `dsh web --host 0.0.0.0`, `dsh web --no-auth`, `echo hi; dsh web`, `$(whoami)`, `dsh "broken`} {
		if _, e := ParseCommand(s); e == nil {
			t.Errorf("accepted unsafe/invalid command: %s", s)
		}
	}
}
func TestLanguageResolution(t *testing.T) {
	for _, x := range []struct{ choice, system, want string }{{"system", "zh-Hans-CN", "zh"}, {"system", "zh_TW", "zh"}, {"system", "en-US", "en"}, {"system", "ja-JP", "en"}, {"en", "zh-CN", "en"}, {"zh", "en-US", "zh"}} {
		if got := ResolveLanguage(x.choice, x.system); got != x.want {
			t.Errorf("%+v got %s", x, got)
		}
	}
}
func TestSettingsV1UpgradeAndLiveAppearance(t *testing.T) {
	p, _ := NewPaths(t.TempDir())
	AtomicWrite(filepath.Join(p.Root, "settings.json"), []byte(`{"port":3080,"language":"en","width":1280,"height":840}`), 0600)
	s, e := p.LoadSettings()
	if e != nil || s.PluginPolicy != "pinned" || s.StartupMinutes != 60 {
		t.Fatalf("migration: %+v %v", s, e)
	}
	m := NewManager(p, s)
	m.cancel = func() {}
	s.TrayOnly = true
	s.Language = "zh"
	s.Port = 4000
	if e = m.ConfigureAppearance(s); e != nil {
		t.Fatal(e)
	}
	got := m.Snapshot().Settings
	if got.Port != 3080 || !got.TrayOnly || got.Language != "zh" {
		t.Fatalf("appearance altered runtime: %+v", got)
	}
}
func TestLatestPluginsResolveAndReceiptFreezesVersions(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"version": "1.2.3"})
	}))
	defer server.Close()
	// The production client clones DefaultTransport; temporarily trust only this
	// local TLS fixture, without weakening certificate checks in production.
	previous := http.DefaultTransport
	http.DefaultTransport = server.Client().Transport
	defer func() { http.DefaultTransport = previous }()
	p, _ := NewPaths(t.TempDir())
	s := Defaults()
	s.PluginPolicy = "latest"
	s.Registry = server.URL
	i := Installer{p, s, &Log{}}
	selected, e := i.resolvePlugins(context.Background())
	if e != nil || len(selected) != 6 {
		t.Fatalf("%v %v", selected, e)
	}
	for _, x := range selected {
		if x.Version != "1.2.3" {
			t.Fatal(x)
		}
	}
	receipt := installReceipt{DSHVersion, PnpmVersion, selected, "latest", s.Registry}
	b, _ := json.Marshal(receipt)
	os.WriteFile(filepath.Join(p.Runtime, "receipt.json"), b, 0600)
	stored, e := i.readReceipt()
	if e != nil || !reflect.DeepEqual(stored.Plugins, selected) {
		t.Fatalf("receipt: %+v %v", stored, e)
	}
}
