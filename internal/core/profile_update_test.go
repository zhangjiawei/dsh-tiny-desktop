package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProfileUpdateWaitsUntilPendingInstallCompletes(t *testing.T) {
	profile := t.TempDir()
	manifest := filepath.Join(profile, "package.json")
	pending := filepath.Join(profile, ".dsh-pending-updates.json")
	if err := os.WriteFile(manifest, []byte(`{"dependencies":{"plugin":"1.0.0"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updates, err := watchProfileUpdates(ctx, profile, 10*time.Millisecond, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(pending, []byte(`{"plugin":"1.1.0"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(manifest, []byte(`{"dependencies":{"plugin":"1.1.0"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-updates:
		t.Fatal("profile restarted while plugin installation was still pending")
	case <-time.After(80 * time.Millisecond):
	}
	if err = os.Remove(pending); err != nil {
		t.Fatal(err)
	}
	select {
	case <-updates:
	case <-time.After(time.Second):
		t.Fatal("completed profile update did not request a restart")
	}
}
