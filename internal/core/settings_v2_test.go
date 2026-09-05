package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
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
	got, err = ParseCommand(`dsh web --trusted-host 172.16.8.136`)
	if err != nil || trustedHost(got) != "172.16.8.136" {
		t.Fatalf("private trusted host rejected: %v %v", got, err)
	}
	for _, s := range []string{`pnpm dlx dsh web --port=80`, `dsh web --host 0.0.0.0`, `dsh web --no-auth`, `echo hi; dsh web`, `$(whoami)`, `dsh "broken`} {
		if _, e := ParseCommand(s); e == nil {
			t.Errorf("accepted unsafe/invalid command: %s", s)
		}
	}
	for _, s := range []string{`dsh web --trusted-host`, `dsh web --trusted-host example.com`, `dsh web --trusted-host=8.8.8.8`} {
		if _, e := ParseCommand(s); e == nil {
			t.Errorf("accepted invalid trusted host: %s", s)
		}
	}
}

func TestWindowsDefaultDLXUsesInstalledPrivateDSH(t *testing.T) {
	args, err := ParseCommand(DefaultCommand + ` --trusted-host 172.16.8.136`)
	if err != nil {
		t.Fatal(err)
	}
	managed, ok := managedWindowsDefaultDLX(args, "windows")
	if !ok || strings.Join(managed, " ") != "web --trusted-host 172.16.8.136" {
		t.Fatalf("managed Windows command = %v, %v", managed, ok)
	}
	if _, ok = managedWindowsDefaultDLX(args, "darwin"); ok {
		t.Fatal("non-Windows command was rewritten")
	}
	custom, err := ParseCommand(strings.Replace(DefaultCommand, DSHVersion, "9.9.9", 1))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok = managedWindowsDefaultDLX(custom, "windows"); ok {
		t.Fatal("custom DSH version was rewritten")
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
	if e != nil || s.StartupMinutes != 60 {
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

func TestSaveWhileRunningNeverStopsService(t *testing.T) {
	p, _ := NewPaths(t.TempDir())
	s := Defaults()
	m := NewManager(p, s)
	cancelled := false
	m.cancel = func() { cancelled = true }
	m.phase, m.port, m.launch = "running", 3080, "http://127.0.0.1:3080/"
	s.Port = 4000
	if err := m.Configure(s); err != nil {
		t.Fatal(err)
	}
	if cancelled || m.Snapshot().Phase != "running" || m.Snapshot().Port != 3080 {
		t.Fatal("saving settings interrupted the active service")
	}
	stored, err := p.LoadSettings()
	if err != nil || stored.Port != 4000 {
		t.Fatal("new settings not persisted", err)
	}
	if !m.Snapshot().RestartRequired {
		t.Fatal("pending restart was not reported")
	}
	s.Port = 3080
	s.AutoStart = false
	if err = m.Configure(s); err != nil {
		t.Fatal(err)
	}
	if m.Snapshot().RestartRequired {
		t.Fatal("appearance/auto-start changes require no service restart")
	}
	s.Port = 0
	if m.Configure(s) == nil || m.Snapshot().Settings.Port != 3080 {
		t.Fatal("invalid save modified settings")
	}
}

func TestFreshToggleDefaultsPreserveExistingChoices(t *testing.T) {
	p, _ := NewPaths(t.TempDir())
	s, err := p.LoadSettings()
	if err != nil || !s.TrayOnly || !s.LAN || !s.AutoStart || !s.HideOnClose || s.AlwaysOnTop {
		t.Fatalf("unexpected fresh defaults: %+v, %v", s, err)
	}
	s.LAN, s.TrayOnly, s.AutoStart, s.HideOnClose = false, false, false, false
	if err = p.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	got, err := p.LoadSettings()
	if err != nil || got != s {
		t.Fatal("upgrade overwrote explicit user choices")
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

func TestExistingInstallReusesReceiptWithoutRegistryAccess(t *testing.T) {
	// With no system Node and an unreachable registry, this can only pass if a
	// legacy install is reused without attempting a latest-version lookup.
	t.Setenv("PATH", "")
	p, _ := NewPaths(t.TempDir())
	s := Defaults()
	s.Registry = "https://127.0.0.1:1"
	asset, err := assetFor(runtime.GOOS + "/" + runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	folder := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(asset.URL), ".tar.gz"), ".zip")
	node := filepath.Join(p.Runtime, folder, "bin", exeName("node"))
	npm := filepath.Join(p.Runtime, folder, "lib", "node_modules", "npm", "bin", "npm-cli.js")
	if runtime.GOOS == "windows" {
		node = filepath.Join(p.Runtime, folder, "node.exe")
		npm = filepath.Join(p.Runtime, folder, "node_modules", "npm", "bin", "npm-cli.js")
	}
	cli := filepath.Join(p.Runtime, "dsh/node_modules/@deepseek-ai/dsh/lib/bin.js")
	for _, path := range []string{node, npm, cli} {
		if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	plugins := append([]Plugin(nil), Plugins...)
	for n := range plugins {
		plugins[n].Version = "1.2.3"
	}
	// A changed download mirror is not an instruction to reinstall or upgrade
	// a completed profile. Reuse it even when the new registry is unreachable.
	b, _ := json.Marshal(installReceipt{DSHVersion, PnpmVersion, plugins, "pinned", "https://registry.npmjs.org"})
	if err = os.WriteFile(filepath.Join(p.Runtime, "receipt.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	i := Installer{p, s, &Log{}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err = i.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestSystemNodeMustMatchPinnedRuntime(t *testing.T) {
	for _, tc := range []struct {
		version string
		want    bool
	}{
		{"24.20.0", true},
		{"v24.20.0", true},
		{"24.19.0", false},
		{"24.21.0", false},
		{"25.0.0", false},
	} {
		if got := systemNodeMatches(tc.version); got != tc.want {
			t.Errorf("systemNodeMatches(%q) = %v, want %v", tc.version, got, tc.want)
		}
	}
}

func TestNodePublishRetriesTransientWindowsAccessDenied(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "stage", "node-v24")
	target := filepath.Join(root, "node-v24")
	executable := filepath.Join(target, "node.exe")
	npm := filepath.Join(target, "node_modules", "npm", "bin", "npm-cli.js")
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	for path, contents := range map[string]string{
		filepath.Join(source, "node.exe"):                                 "node",
		filepath.Join(source, "node_modules", "npm", "bin", "npm-cli.js"): "npm",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	attempts := 0
	rename := func(oldPath, newPath string) error {
		attempts++
		if attempts < 3 {
			return &os.PathError{Op: "rename", Path: oldPath + " -> " + newPath, Err: os.ErrPermission}
		}
		return os.Rename(oldPath, newPath)
	}
	if err := publishNodeRuntime(source, target, []string{executable, npm}, rename, func(time.Duration) {}); err != nil {
		t.Fatal(err)
	}
	if attempts != 3 {
		t.Fatalf("rename attempts = %d, want 3", attempts)
	}
}

func TestNodePublishReplacesIncompleteTarget(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "stage", "node-v24")
	target := filepath.Join(root, "node-v24")
	executable := filepath.Join(target, "node.exe")
	npm := filepath.Join(target, "node_modules", "npm", "bin", "npm-cli.js")
	for _, dir := range []string{source, target} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for path, contents := range map[string]string{
		filepath.Join(source, "node.exe"):                                 "node",
		filepath.Join(source, "node_modules", "npm", "bin", "npm-cli.js"): "npm",
		filepath.Join(target, "node.exe"):                                 "partial-node",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(target, "interrupted"), []byte("partial"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := publishNodeRuntime(source, target, []string{executable, npm}, os.Rename, func(time.Duration) {}); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(executable); err != nil || string(got) != "node" {
		t.Fatalf("published executable = %q, %v", got, err)
	}
	if matches, err := filepath.Glob(target + ".incomplete-*"); err != nil || len(matches) != 0 {
		t.Fatalf("incomplete runtime backups = %v, %v", matches, err)
	}
}

func TestNodePublishRestoresIncompleteTargetWhenReplacementFails(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "stage", "node-v24")
	target := filepath.Join(root, "node-v24")
	executable := filepath.Join(target, "node.exe")
	npm := filepath.Join(target, "node_modules", "npm", "bin", "npm-cli.js")
	for _, dir := range []string{source, target} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	marker := filepath.Join(target, "interrupted")
	if err := os.WriteFile(marker, []byte("partial"), 0600); err != nil {
		t.Fatal(err)
	}
	rename := func(oldPath, newPath string) error {
		if oldPath == source {
			return &os.PathError{Op: "rename", Path: oldPath + " -> " + newPath, Err: os.ErrPermission}
		}
		return os.Rename(oldPath, newPath)
	}
	if err := publishNodeRuntime(source, target, []string{executable, npm}, rename, func(time.Duration) {}); err == nil {
		t.Fatal("expected permanent publish failure")
	}
	if got, err := os.ReadFile(marker); err != nil || string(got) != "partial" {
		t.Fatalf("restored marker = %q, %v", got, err)
	}
}

func TestNodePublishRejectsIncompleteExtractionBeforeMovingTarget(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "stage", "node-v24")
	target := filepath.Join(root, "node-v24")
	executable := filepath.Join(target, "node.exe")
	npm := filepath.Join(target, "node_modules", "npm", "bin", "npm-cli.js")
	for _, dir := range []string{source, target} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	marker := filepath.Join(target, "interrupted")
	if err := os.WriteFile(marker, []byte("partial"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "node.exe"), []byte("node"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := publishNodeRuntime(source, target, []string{executable, npm}, os.Rename, func(time.Duration) {}); err == nil {
		t.Fatal("expected incomplete extraction failure")
	}
	if got, err := os.ReadFile(marker); err != nil || string(got) != "partial" {
		t.Fatalf("target changed before source validation: %q, %v", got, err)
	}
}

func TestAtomicReplacementRetriesTransientAccessDenied(t *testing.T) {
	attempts := 0
	err := replaceFileWithRetry(func() error {
		attempts++
		if attempts < 3 {
			return os.ErrPermission
		}
		return nil
	}, func(time.Duration) {})
	if err != nil || attempts != 3 {
		t.Fatalf("atomic replacement = %v after %d attempts", err, attempts)
	}
}
