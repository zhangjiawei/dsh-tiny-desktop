package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProfileChangesStayProducerAgnosticAndContinuous(t *testing.T) {
	profile := t.TempDir()
	manifest := filepath.Join(profile, "package.json")
	pending := filepath.Join(profile, ".dsh-pending-updates.json")
	if err := os.WriteFile(manifest, []byte(`{"dependencies":{"plugin":"1.0.0"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	changes, err := watchProfileChanges(ctx, profile, nil, 10*time.Millisecond, 2)
	if err != nil {
		t.Fatal(err)
	}
	// A producer-specific marker is irrelevant to Tiny. Dependency changes
	// become pending activation but can never stop the managed DSH process.
	if err = os.WriteFile(pending, []byte(`{"plugin":"1.1.0"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(manifest, []byte(`{"dependencies":{"plugin":"1.1.0"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-changes:
	case <-time.After(time.Second):
		t.Fatal("dependency manifest change was not reported")
	}
	if err = os.Remove(pending); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(profile, "pnpm-lock.yaml"), []byte("lockfileVersion: '9.0'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-changes:
	case <-time.After(time.Second):
		t.Fatal("continuous watcher did not report the lockfile change")
	}
	select {
	case <-changes:
		t.Fatal("unchanged profile produced a duplicate notification")
	case <-time.After(80 * time.Millisecond):
	}
}

func TestProfileChangeOnlyMarksActivationPending(t *testing.T) {
	paths, err := NewPaths(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	manager := NewManager(paths, Defaults())
	cancelled := false
	manager.cancel = func() { cancelled = true }
	manager.phase = "running"
	manager.noteProfileChange()
	snapshot := manager.Snapshot()
	if cancelled || snapshot.Phase != "running" {
		t.Fatal("profile change interrupted the managed process")
	}
	if !snapshot.ActivationPending || snapshot.ActivationChangedAt == "" {
		t.Fatalf("profile change did not expose pending activation: %+v", snapshot)
	}
}

func TestProfileActivationRequiresMatchingBootGeneration(t *testing.T) {
	profile := t.TempDir()
	manifest := filepath.Join(profile, "package.json")
	if err := os.WriteFile(manifest, []byte(`{"dependencies":{"plugin":"1.0.0"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	boot, err := profileFingerprint(profile)
	if err != nil {
		t.Fatal(err)
	}
	paths, _ := NewPaths(t.TempDir())
	manager := NewManager(paths, Defaults())
	manager.noteProfileChange()
	if err = os.WriteFile(manifest, []byte(`{"dependencies":{"plugin":"2.0.0"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	manager.markProfileActivated(profile, boot)
	if !manager.Snapshot().ActivationPending {
		t.Fatal("concurrent profile change was incorrectly marked active")
	}
	current, _ := profileFingerprint(profile)
	manager.markProfileActivated(profile, current)
	if manager.Snapshot().ActivationPending {
		t.Fatal("matching successful boot generation did not clear activation")
	}
}
