package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportMergesSettingsCredentialsAndWorkspaceData(t *testing.T) {
	source := t.TempDir()
	writeTestFile(t, filepath.Join(source, "settings.yaml"), "ui-onboarding:\n  notice: source\nllm-pi-ai:\n  provider: source-provider\n")
	writeTestFile(t, filepath.Join(source, ".credentials.yaml"), "version: 1\nrefs:\n  DEEPSEEK_API_KEY: source-deepseek-key\n  SHARED_KEY: source-must-not-win\n")
	sourceWorkspace := `{"unit":{"name":"workspace","version":2},"global":{"initialized":true,"workspaceIds":["source-project","shared"],"archivedSessionIds":[]},"tables":{"workspaces":{"source-project":{"path":"C:/source","sessionIds":[]},"shared":{"path":"C:/source-must-not-win","sessionIds":[]}}}}`
	writeTestFile(t, filepath.Join(source, "storages", "workspace.json"), sourceWorkspace)

	paths, err := NewPaths(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tinySettings := "ui-onboarding:\n  notice: tiny-wins\n"
	tinyCredentials := "version: 1\nrefs:\n  TINY_ONLY_KEY: tiny-only\n  SHARED_KEY: tiny-wins\n"
	writeTestFile(t, filepath.Join(paths.Data, "settings.yaml"), tinySettings)
	writeTestFile(t, filepath.Join(paths.Data, ".credentials.yaml"), tinyCredentials)
	tinyWorkspace := `{"unit":{"name":"workspace","version":2},"global":{"initialized":true,"workspaceIds":["tiny-project","shared"],"archivedSessionIds":[]},"tables":{"workspaces":{"tiny-project":{"path":"C:/tiny","sessionIds":[]},"shared":{"path":"C:/tiny-wins","sessionIds":[]}}}}`
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
	tiny := []byte(`{"unit":{"name":"workspace","version":2},"global":{"initialized":true,"workspaceIds":["shared","tiny"],"archivedSessionIds":["tiny-archived"]},"tables":{"workspaces":{"shared":{"path":"tiny","sessionIds":[]},"tiny":{"path":"tiny-only","sessionIds":[]}}}}`)
	source := []byte(`{"unit":{"name":"workspace","version":2},"global":{"initialized":true,"workspaceIds":["shared","source"],"archivedSessionIds":["source-archived"]},"tables":{"workspaces":{"shared":{"path":"source","sessionIds":[]},"source":{"path":"source-only","sessionIds":[]}}}}`)
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
	var global struct {
		WorkspaceIDs       []string `json:"workspaceIds"`
		ArchivedSessionIDs []string `json:"archivedSessionIds"`
	}
	if json.Unmarshal(document.Global, &global) != nil {
		t.Fatalf("merged workspace global is invalid: %s", document.Global)
	}
	if err = validateWorkspaceRegistryForTest(global.WorkspaceIDs, document.Tables["workspaces"]); err != nil {
		t.Fatal(err)
	}
	if strings.Join(global.WorkspaceIDs, ",") != "shared,tiny,source" {
		t.Fatalf("unexpected merged workspace order: %v", global.WorkspaceIDs)
	}
	if strings.Join(global.ArchivedSessionIDs, ",") != "tiny-archived,source-archived" {
		t.Fatalf("unexpected merged archive set: %v", global.ArchivedSessionIDs)
	}
	workspaces := document.Tables["workspaces"]
	var shared, sourceOnly map[string]string
	_ = json.Unmarshal(workspaces["shared"], &shared)
	_ = json.Unmarshal(workspaces["source"], &sourceOnly)
	if len(workspaces) != 3 || shared["path"] != "tiny" || sourceOnly["path"] != "source-only" {
		t.Fatalf("unexpected workspace merge: %s", merged)
	}
}

func TestWorkspaceStorageMergeCoalescesSamePathUnderTinyRecord(t *testing.T) {
	tiny := []byte(`{"unit":{"name":"workspace","version":2},"global":{"initialized":true,"workspaceIds":["tiny"],"archivedSessionIds":[]},"tables":{"workspaces":{"tiny":{"path":"C:/same","title":"Tiny","sessionIds":["tiny-session"],"createdAt":"tiny-created","updatedAt":"tiny-updated"}}}}`)
	source := []byte(`{"unit":{"name":"workspace","version":2},"global":{"initialized":true,"workspaceIds":["source"],"archivedSessionIds":[]},"tables":{"workspaces":{"source":{"path":"C:/same","title":"Source","sessionIds":["tiny-session","source-session"],"createdAt":"source-created","updatedAt":"source-updated"}}}}`)
	merged, changed, err := mergeStorageDocuments(tiny, source)
	if err != nil || !changed {
		t.Fatalf("merge same path = changed %v, %v", changed, err)
	}
	var document storageDocument
	if err = json.Unmarshal(merged, &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Tables["workspaces"]) != 1 {
		t.Fatalf("same path created a duplicate workspace: %s", merged)
	}
	var tinyRecord map[string]any
	if err = json.Unmarshal(document.Tables["workspaces"]["tiny"], &tinyRecord); err != nil {
		t.Fatal(err)
	}
	if tinyRecord["title"] != "Tiny" || tinyRecord["createdAt"] != "tiny-created" || tinyRecord["updatedAt"] != "tiny-updated" {
		t.Fatalf("Tiny-owned fields changed: %+v", tinyRecord)
	}
	sessions, _ := tinyRecord["sessionIds"].([]any)
	if len(sessions) != 2 || sessions[0] != "tiny-session" || sessions[1] != "source-session" {
		t.Fatalf("source-only session was not appended: %+v", sessions)
	}
}

func TestRepairWorkspaceRegistryAddsLegacyOrphanWithoutChangingRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace.json")
	orphanID := "853f3223-152c-42a4-a25c-9d2ad97f2814"
	corrupt := `{"unit":{"name":"workspace","version":2},"global":{"initialized":true,"workspaceIds":["tiny"],"archivedSessionIds":[]},"tables":{"workspaces":{"tiny":{"path":"C:/tiny","sessionIds":[]},"` + orphanID + `":{"path":"C:/imported","sessionIds":["session-imported"]}}}}`
	writeTestFile(t, path, corrupt)

	repaired, err := repairWorkspaceRegistryFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if repaired != 1 {
		t.Fatalf("repaired = %d, want 1", repaired)
	}
	var document storageDocument
	if err = json.Unmarshal([]byte(readTestFile(t, path)), &document); err != nil {
		t.Fatal(err)
	}
	var global workspaceGlobalState
	if err = json.Unmarshal(document.Global, &global); err != nil {
		t.Fatal(err)
	}
	if strings.Join(global.WorkspaceIDs, ",") != "tiny,"+orphanID {
		t.Fatalf("workspace order = %v", global.WorkspaceIDs)
	}
	var imported workspaceRecordState
	if err = json.Unmarshal(document.Tables["workspaces"][orphanID], &imported); err != nil {
		t.Fatal(err)
	}
	if imported.Path != "C:/imported" || strings.Join(imported.SessionIDs, ",") != "session-imported" {
		t.Fatalf("imported record changed: %+v", imported)
	}
	if got := readTestFile(t, path+".tiny-v0.2.12-recovery"); got != corrupt {
		t.Fatal("pre-repair workspace backup is not byte-exact")
	}
}

func TestRepairWorkspaceRegistryCoalescesDuplicatePathAndSessionOwnership(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace.json")
	writeTestFile(t, path, `{"unit":{"name":"workspace","version":2},"global":{"initialized":true,"workspaceIds":["tiny"],"archivedSessionIds":[]},"tables":{"workspaces":{"tiny":{"path":"C:/same","title":"Tiny","sessionIds":["tiny-session"],"createdAt":"tiny-created","updatedAt":"tiny-updated"},"imported":{"path":"C:/same","title":"Source","sessionIds":["tiny-session","source-session"],"createdAt":"source-created","updatedAt":"source-updated"}}}}`)
	repaired, err := repairWorkspaceRegistryFile(path)
	if err != nil || repaired != 1 {
		t.Fatalf("repair duplicate workspace = %d, %v", repaired, err)
	}
	var document storageDocument
	if err = json.Unmarshal([]byte(readTestFile(t, path)), &document); err != nil {
		t.Fatal(err)
	}
	if _, exists := document.Tables["workspaces"]["imported"]; exists {
		t.Fatal("redundant imported workspace with the same canonical path remains")
	}
	var tiny map[string]any
	if err = json.Unmarshal(document.Tables["workspaces"]["tiny"], &tiny); err != nil {
		t.Fatal(err)
	}
	if tiny["title"] != "Tiny" || tiny["createdAt"] != "tiny-created" || tiny["updatedAt"] != "tiny-updated" {
		t.Fatalf("Tiny-owned workspace fields changed: %+v", tiny)
	}
	sessions, _ := tiny["sessionIds"].([]any)
	if len(sessions) != 2 || sessions[0] != "tiny-session" || sessions[1] != "source-session" {
		t.Fatalf("sessions were not safely combined: %+v", sessions)
	}
}

func TestRepairWorkspaceRegistryLeavesValidFileByteExact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace.json")
	valid := `{"unit":{"name":"workspace","version":2},"global":{"initialized":true,"workspaceIds":["tiny"],"archivedSessionIds":[]},"tables":{"workspaces":{"tiny":{"path":"C:/tiny","sessionIds":[]}}}}`
	writeTestFile(t, path, valid)
	repaired, err := repairWorkspaceRegistryFile(path)
	if err != nil || repaired != 0 {
		t.Fatalf("repair valid workspace = %d, %v", repaired, err)
	}
	if got := readTestFile(t, path); got != valid {
		t.Fatal("valid workspace was rewritten")
	}
}

// validateWorkspaceRegistryForTest mirrors the DSH workspace service's
// persisted-state invariant. Keeping this check at the merge boundary catches
// corrupt imports before a real DSH process has to load the profile.
func validateWorkspaceRegistryForTest(order []string, workspaces map[string]json.RawMessage) error {
	seen := make(map[string]bool, len(order))
	for _, id := range order {
		if seen[id] {
			return fmt.Errorf("workspace domain is inconsistent: registry order repeats workspace %q", id)
		}
		if _, exists := workspaces[id]; !exists {
			return fmt.Errorf("workspace domain is inconsistent: registry order references missing workspace %q", id)
		}
		seen[id] = true
	}
	for id := range workspaces {
		if !seen[id] {
			return fmt.Errorf("workspace domain is inconsistent: workspace %q is absent from registry order", id)
		}
	}
	return nil
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
