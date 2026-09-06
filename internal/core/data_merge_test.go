package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportMergesSettingsCredentialsAndWorkspaceData(t *testing.T) {
	source := t.TempDir()
	writeTestFile(t, filepath.Join(source, "settings.yaml"), "ui-onboarding:\n  notice: source\nllm-pi-ai:\n  provider: source-provider\n")
	writeTestFile(t, filepath.Join(source, ".credentials.yaml"), "version: 1\nrefs:\n  DEEPSEEK_API_KEY: source-deepseek-key\n  SHARED_KEY: source-must-not-win\n")
	sourceWorkspace := `{"unit":{"name":"workspace","version":2},"global":{"owner":"source"},"tables":{"workspaces":{"source-project":{"path":"C:/source"},"shared":{"path":"C:/source-must-not-win"}}}}`
	writeTestFile(t, filepath.Join(source, "storages", "workspace.json"), sourceWorkspace)

	paths, err := NewPaths(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tinySettings := "ui-onboarding:\n  notice: tiny-wins\n"
	tinyCredentials := "version: 1\nrefs:\n  TINY_ONLY_KEY: tiny-only\n  SHARED_KEY: tiny-wins\n"
	writeTestFile(t, filepath.Join(paths.Data, "settings.yaml"), tinySettings)
	writeTestFile(t, filepath.Join(paths.Data, ".credentials.yaml"), tinyCredentials)
	tinyWorkspace := `{"unit":{"name":"workspace","version":2},"global":{"owner":"tiny"},"tables":{"workspaces":{"tiny-project":{"path":"C:/tiny"},"shared":{"path":"C:/tiny-wins"}}}}`
	writeTestFile(t, filepath.Join(paths.Data, "storages", "workspace.json"), tinyWorkspace)
	manager := NewManager(paths, Defaults())

	preview, err := manager.PreviewImport(source, true)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Files != 0 || preview.Merged != 3 || preview.Conflicts != 0 {
		t.Fatalf("project/config merge was classified incorrectly: %+v", preview)
	}

	backup, err := manager.Import(source, true)
	if err != nil {
		t.Fatal(err)
	}
	settings := readTestFile(t, filepath.Join(paths.Data, "settings.yaml"))
	for _, expected := range []string{"tiny-wins", "llm-pi-ai", "source-provider"} {
		if !strings.Contains(settings, expected) {
			t.Fatalf("merged settings missing %q", expected)
		}
	}
	credentials := readTestFile(t, filepath.Join(paths.Data, ".credentials.yaml"))
	for _, expected := range []string{"DEEPSEEK_API_KEY", "source-deepseek-key", "TINY_ONLY_KEY", "SHARED_KEY", "tiny-wins"} {
		if !strings.Contains(credentials, expected) {
			t.Fatalf("merged credentials missing %q", expected)
		}
	}
	if strings.Contains(credentials, "source-must-not-win") {
		t.Fatal("source credential overwrote the Tiny value")
	}
	workspace := readTestFile(t, filepath.Join(paths.Data, "storages", "workspace.json"))
	for _, expected := range []string{"source-project", "tiny-project", "C:/tiny-wins"} {
		if !strings.Contains(workspace, expected) {
			t.Fatalf("merged workspace missing %q: %s", expected, workspace)
		}
	}
	if strings.Contains(workspace, "source-must-not-win") {
		t.Fatal("source workspace record overwrote Tiny")
	}

	if err = manager.RestoreBackup(backup); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, filepath.Join(paths.Data, "settings.yaml")); got != tinySettings {
		t.Fatalf("settings restore changed Tiny data: %q", got)
	}
	if got := readTestFile(t, filepath.Join(paths.Data, ".credentials.yaml")); got != tinyCredentials {
		t.Fatalf("credentials restore changed Tiny data: %q", got)
	}
	if got := readTestFile(t, filepath.Join(paths.Data, "storages", "workspace.json")); got != tinyWorkspace {
		t.Fatalf("workspace restore changed Tiny data: %q", got)
	}
}

func TestWorkspaceStorageMergeAddsProjectsAndKeepsTinyRecords(t *testing.T) {
	tiny := []byte(`{"unit":{"name":"workspace","version":2},"global":{"owner":"tiny"},"tables":{"workspaces":{"shared":{"path":"tiny"},"tiny":{"path":"tiny-only"}}}}`)
	source := []byte(`{"unit":{"name":"workspace","version":2},"global":{"owner":"source"},"tables":{"workspaces":{"shared":{"path":"source"},"source":{"path":"source-only"}}}}`)
	merged, changed, err := mergeStorageDocuments(tiny, source)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("source-only workspace record was not merged")
	}
	var document storageDocument
	if err = json.Unmarshal(merged, &document); err != nil {
		t.Fatal(err)
	}
	var global map[string]string
	if json.Unmarshal(document.Global, &global) != nil || global["owner"] != "tiny" {
		t.Fatalf("source global replaced Tiny global: %s", document.Global)
	}
	workspaces := document.Tables["workspaces"]
	var shared, sourceOnly map[string]string
	_ = json.Unmarshal(workspaces["shared"], &shared)
	_ = json.Unmarshal(workspaces["source"], &sourceOnly)
	if len(workspaces) != 3 || shared["path"] != "tiny" || sourceOnly["path"] != "source-only" {
		t.Fatalf("unexpected workspace merge: %s", merged)
	}
}

func writeTestFile(t *testing.T, name, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, name string) string {
	t.Helper()
	contents, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}
