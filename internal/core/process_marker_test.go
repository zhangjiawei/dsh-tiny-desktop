package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProcessMarkerRoundTripAndOwnerGuard(t *testing.T) {
	root := t.TempDir()
	paths, err := NewPaths(root)
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager(paths, Defaults())
	if err = m.writeProcessMarker(43210, filepath.Join(paths.Runtime, "node")); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(processMarkerPath(paths))
	if err != nil || len(contents) == 0 {
		t.Fatalf("marker was not written: %v", err)
	}
	m.clearProcessMarker(43210)
	if _, err = os.Stat(processMarkerPath(paths)); !os.IsNotExist(err) {
		t.Fatalf("marker was not cleared: %v", err)
	}
}

func TestProcessMarkerDoesNotClearAnotherOwner(t *testing.T) {
	root := t.TempDir()
	paths, err := NewPaths(root)
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager(paths, Defaults())
	if err = AtomicWrite(processMarkerPath(paths), []byte(`{"pid":43210,"ownerPid":99999,"executable":"node"}`), 0600); err != nil {
		t.Fatal(err)
	}
	m.clearProcessMarker(43210)
	if _, err = os.Stat(processMarkerPath(paths)); err != nil {
		t.Fatalf("marker for another owner was cleared: %v", err)
	}
}
