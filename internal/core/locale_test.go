package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncDSHLocalePreferencePreservesSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.yaml")
	original := "ui-theme:\n  preference: system\nllm-pi-ai:\n  providers:\n    demo:\n      apiKey: secret\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if err := SyncDSHLocalePreference(dir, "system", "zh-Hans-CN"); err != nil {
		t.Fatal(err)
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(updated)
	if ((!strings.Contains(text, "locale:\n    preference: zh")) && !strings.Contains(text, "locale:\n  preference: zh")) || !strings.Contains(text, "apiKey: secret") {
		t.Fatalf("locale or existing settings missing: %s", text)
	}
	if err := SyncDSHLocalePreference(dir, "system", "zh-CN"); err != nil {
		t.Fatal(err)
	}
}

func TestSyncDSHLocalePreferenceMissingFileIsNoop(t *testing.T) {
	if err := SyncDSHLocalePreference(t.TempDir(), "en", "zh-CN"); err != nil {
		t.Fatal(err)
	}
}
