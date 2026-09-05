//go:build !windows

package core

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestImportSkipsSpecialFileAndKeepsRegularData(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "settings.yaml"), []byte("language: zh"), 0600); err != nil {
		t.Fatal(err)
	}
	sessions := filepath.Join(source, "sessions")
	if err := os.Mkdir(sessions, 0700); err != nil {
		t.Fatal(err)
	}
	const relative = "sessions/runtime.pipe"
	if err := syscall.Mkfifo(filepath.Join(source, filepath.FromSlash(relative)), 0600); err != nil {
		t.Fatal(err)
	}

	regular := filepath.Join(sessions, "kept.json")
	if err := os.WriteFile(regular, []byte("kept"), 0600); err != nil {
		t.Fatal(err)
	}

	preview, err := PreviewImport(source, false)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Files != 2 || preview.Skipped != 1 || len(preview.SkippedItems) != 1 || !strings.Contains(filepath.ToSlash(preview.SkippedItems[0]), relative) {
		t.Fatalf("preview must keep regular files and report skipped %q: %+v", relative, preview)
	}

	paths, err := NewPaths(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	manager := NewManager(paths, Defaults())
	if _, err := manager.Import(source, false); err != nil {
		t.Fatal(err)
	}
	if contents, err := os.ReadFile(filepath.Join(paths.Data, "sessions", "kept.json")); err != nil || string(contents) != "kept" {
		t.Fatalf("regular import lost: %q, %v", contents, err)
	}
	if _, err := os.Lstat(filepath.Join(paths.Data, filepath.FromSlash(relative))); !os.IsNotExist(err) {
		t.Fatalf("special file was copied: %v", err)
	}
}
