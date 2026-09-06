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
	"time"
)

func TestLegacyCommandsMigrateToOneVersionOwner(t *testing.T) {
	for _, test := range []struct {
		command string
		mode    string
		extra   string
	}{
		{"", RuntimeModeManaged, ""},
		{ManagedCommand, RuntimeModeManaged, ""},
		{DefaultCommand + " --trusted-host 172.16.8.136", RuntimeModeManaged, "--trusted-host 172.16.8.136"},
		{"pnpm dlx @deepseek-ai/dsh@9.9.9 web", RuntimeModeCustom, ""},
	} {
		paths, _ := NewPaths(t.TempDir())
		contents, _ := json.Marshal(map[string]any{"command": test.command, "registry": DefaultRegistry})
		if err := os.WriteFile(filepath.Join(paths.Root, "settings.json"), contents, 0600); err != nil {
			t.Fatal(err)
		}
		got, err := paths.LoadSettings()
		if err != nil || got.RuntimeMode != test.mode || got.ExtraArgs != test.extra || got.Command != mapEmptyCommand(test.command) {
			t.Fatalf("command %q: %+v, %v", test.command, got, err)
		}
	}
}

func mapEmptyCommand(command string) string {
	if command == "" {
		return ManagedCommand
	}
	return command
}

func TestManagedArgumentsCannotTakeOwnershipFromTiny(t *testing.T) {
	got, err := ParseManagedArgs(`--trusted-host 172.16.8.136 --verbose`)
	if err != nil || !reflect.DeepEqual(got, []string{"--trusted-host", "172.16.8.136", "--verbose"}) {
		t.Fatal(got, err)
	}
	for _, invalid := range []string{"--port 4000", "--host 0.0.0.0", "--token secret", "--no-auth", "x | y"} {
		if _, err = ParseManagedArgs(invalid); err == nil {
			t.Errorf("accepted managed override %q", invalid)
		}
	}
}

func TestVersionOrderingHandlesStableAndNumericPrerelease(t *testing.T) {
	for _, test := range []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0-rc.9", 1}, {"1.0.0-rc.10", "1.0.0-rc.2", 1},
		{"1.2.0", "1.10.0", -1}, {"1.0.0+one", "1.0.0+two", 0},
	} {
		if got := compareVersions(test.a, test.b); got != test.want {
			t.Errorf("%s ? %s = %d", test.a, test.b, got)
		}
	}
}

func TestUpdateChannelsAndRegistryFailures(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(packageMetadata{DistTags: map[string]string{"latest": "0.3.0-rc.2", "next": "0.3.0-rc.10"}, Versions: map[string]json.RawMessage{"0.2.0": json.RawMessage(`{}`), "0.3.0-rc.10": json.RawMessage(`{}`)}})
	}))
	defer server.Close()
	previous := http.DefaultTransport
	http.DefaultTransport = server.Client().Transport
	defer func() { http.DefaultTransport = previous }()
	s := Defaults()
	s.Registry = server.URL
	s.DSHChannel = DSHChannelPreview
	if got, err := resolveDSHTarget(context.Background(), s); err != nil || got != "0.3.0-rc.10" {
		t.Fatal(got, err)
	}
	s.DSHChannel = DSHChannelStable
	if got, err := resolveDSHTarget(context.Background(), s); err != nil || got != "0.2.0" {
		t.Fatal(got, err)
	}
	s.Registry = "https://127.0.0.1:1"
	if _, err := resolveDSHTarget(context.Background(), s); err == nil {
		t.Fatal("unreachable registry accepted")
	}
}

func TestRuntimeManifestRejectsEscapesAndUpdateSnapshotRestores(t *testing.T) {
	paths, _ := NewPaths(t.TempDir())
	bad := managedRuntimeState{Current: runtimeSlot{Version: "1.0.0", Dir: filepath.Join(paths.Runtime, "..", "outside")}}
	contents, _ := json.Marshal(bad)
	if err := os.WriteFile(filepath.Join(paths.Runtime, managedRuntimeManifest), contents, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readManagedRuntimeState(paths); err == nil {
		t.Fatal("manifest path escape accepted")
	}
	bad = managedRuntimeState{Current: runtimeSlot{Version: "1.0.0", Dir: paths.Runtime}, RollbackBackup: filepath.Join(paths.Runtime, "dsh")}
	contents, _ = json.Marshal(bad)
	if err := os.WriteFile(filepath.Join(paths.Runtime, managedRuntimeManifest), contents, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readManagedRuntimeState(paths); err == nil {
		t.Fatal("broad internal runtime path accepted")
	}
	_ = os.Remove(filepath.Join(paths.Runtime, managedRuntimeManifest))
	storage := filepath.Join(paths.Data, "storages", "workspace.json")
	if err := os.MkdirAll(filepath.Dir(storage), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(storage, []byte("before"), 0600); err != nil {
		t.Fatal(err)
	}
	backup, err := snapshotUpdateState(paths)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(backup)
	if err = os.WriteFile(storage, []byte("after"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(paths.Data, "settings.yaml"), []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = restoreUpdateState(paths, backup); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(storage)
	if string(got) != "before" {
		t.Fatalf("snapshot restored %q", got)
	}
	if _, err = os.Stat(filepath.Join(paths.Data, "settings.yaml")); !os.IsNotExist(err) {
		t.Fatal("new control file survived rollback")
	}
}

func TestLegacyReceiptRemainsCurrentUntilExplicitUpdate(t *testing.T) {
	paths, _ := NewPaths(t.TempDir())
	receipt, _ := json.Marshal(installReceipt{DSH: "0.1.1", PNPM: PnpmVersion})
	if err := os.WriteFile(filepath.Join(paths.Runtime, "receipt.json"), receipt, 0600); err != nil {
		t.Fatal(err)
	}
	version, dir := (&Installer{Paths: paths}).activeRuntime()
	if version != "0.1.1" || dir != filepath.Join(paths.Runtime, "dsh") {
		t.Fatalf("legacy runtime changed silently: %s %s", version, dir)
	}
}

func TestCleanupKeepsOnlyActivePreviousAndRollbackData(t *testing.T) {
	paths, _ := NewPaths(t.TempDir())
	versions := filepath.Join(paths.Runtime, "dsh-versions")
	current, previous, orphan := filepath.Join(versions, "2.0.0"), filepath.Join(versions, "1.0.0"), filepath.Join(versions, "0.9.0")
	backup, stale := filepath.Join(paths.Runtime, ".dsh-update-backup-keep"), filepath.Join(paths.Runtime, ".dsh-update-stale")
	for _, dir := range []string{current, previous, orphan, backup, stale} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	state := managedRuntimeState{Current: runtimeSlot{Version: "2.0.0", Dir: current}, Previous: &runtimeSlot{Version: "1.0.0", Dir: previous}, RollbackBackup: backup}
	if err := writeManagedRuntimeState(paths, state); err != nil {
		t.Fatal(err)
	}
	cleanupUpdateArtifacts(paths)
	for _, dir := range []string{current, previous, backup} {
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("live artifact removed: %s", dir)
		}
	}
	for _, dir := range []string{orphan, stale} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Errorf("orphan survived: %s", dir)
		}
	}
}

func TestManualRollbackSwapsRuntimeAndRestoresControlState(t *testing.T) {
	paths, _ := NewPaths(t.TempDir())
	storage := filepath.Join(paths.Data, "storages", "workspace.json")
	if err := os.MkdirAll(filepath.Dir(storage), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(storage, []byte("old-state"), 0600); err != nil {
		t.Fatal(err)
	}
	oldData, err := snapshotUpdateState(paths)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(storage, []byte("new-state"), 0600); err != nil {
		t.Fatal(err)
	}
	current := filepath.Join(paths.Runtime, "dsh-versions", "2.0.0")
	previous := filepath.Join(paths.Runtime, "dsh-versions", "1.0.0")
	for _, dir := range []string{current, previous} {
		if err = os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	state := managedRuntimeState{Current: runtimeSlot{Version: "2.0.0", Dir: current}, Previous: &runtimeSlot{Version: "1.0.0", Dir: previous}, RollbackBackup: oldData}
	if err = writeManagedRuntimeState(paths, state); err != nil {
		t.Fatal(err)
	}
	m := NewManager(paths, Defaults())
	if _, err = m.RollbackDSH(context.Background()); err != nil {
		t.Fatal(err)
	}
	contents, _ := os.ReadFile(storage)
	if string(contents) != "old-state" {
		t.Fatalf("rollback data = %q", contents)
	}
	reversed, err := readManagedRuntimeState(paths)
	if err != nil || reversed.Current.Version != "1.0.0" || reversed.Previous == nil || reversed.Previous.Version != "2.0.0" || reversed.RollbackBackup == "" {
		t.Fatalf("rollback state = %+v, %v", reversed, err)
	}
}

func TestCustomModeDisablesManagedUpdater(t *testing.T) {
	paths, _ := NewPaths(t.TempDir())
	s := Defaults()
	s.RuntimeMode = RuntimeModeCustom
	s.Command = "dsh web"
	m := NewManager(paths, s)
	if _, err := m.CheckDSHUpdate(context.Background()); err == nil {
		t.Fatal("custom command unexpectedly gained a second version owner")
	}
}

func TestLifecycleConflictsAreBlockedAndStopCancelsUpdate(t *testing.T) {
	paths, _ := NewPaths(t.TempDir())
	m := NewManager(paths, Defaults())
	cancelled := make(chan struct{})
	done := make(chan struct{})
	m.mu.Lock()
	m.updateBusy = true
	m.updateCancel = func() { close(cancelled) }
	m.updateDone = done
	m.mu.Unlock()
	if err := m.Start(); err == nil {
		t.Fatal("start entered an update transaction")
	}
	if err := m.Configure(Defaults()); err == nil {
		t.Fatal("settings changed during an update transaction")
	}
	stopped := make(chan struct{})
	go func() { m.Stop(); close(stopped) }()
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("stop did not cancel update")
	}
	select {
	case <-stopped:
		t.Fatal("stop returned before rollback cleanup")
	default:
	}
	m.mu.Lock()
	m.updateBusy = false
	m.updateCancel = nil
	m.updateDone = nil
	close(done)
	m.mu.Unlock()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("stop did not finish after cleanup")
	}
}
