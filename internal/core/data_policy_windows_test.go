//go:build windows

package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsImportSkipsJunctionWithPathAndTag(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "settings.yaml"), []byte("language: zh"), 0600); err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	junction := filepath.Join(source, "sessions")
	if output, err := exec.Command("cmd.exe", "/d", "/c", "mklink", "/J", junction, target).CombinedOutput(); err != nil {
		t.Fatalf("create test junction: %v: %s", err, output)
	}

	preview, err := PreviewImport(source, false)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Skipped != 1 || len(preview.SkippedItems) != 1 || !strings.Contains(preview.SkippedItems[0], "sessions") || !strings.Contains(preview.SkippedItems[0], "0xA0000003") {
		t.Fatalf("junction skip must name its path and tag, got %+v", preview)
	}
}
