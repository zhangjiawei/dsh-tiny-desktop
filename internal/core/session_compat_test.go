package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSessionCompatFixture(t *testing.T, root, worker, format string) string {
	t.Helper()
	workerPath := filepath.Join(root, "node_modules", "@deepseek-ai", "dsh-session-persistence-jsonl", "lib", "worker.cjs")
	formatPath := filepath.Join(root, "node_modules", "@deepseek-ai", "dsh-session-format-v2-to-v3", "lib", "index.js")
	for _, path := range []string{workerPath, formatPath} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(workerPath, []byte(worker), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(formatPath, []byte(format), 0600); err != nil {
		t.Fatal(err)
	}
	return workerPath
}

func TestEnsureSessionMigrationSourceAllowlistPatchesCanonicalSet(t *testing.T) {
	root := t.TempDir()
	worker := writeSessionCompatFixture(t, root,
		"const SOURCE_KINDS = new Set([\n\t\"user\",\n\t\"agent-message\"\n]);\nthrow new Error(\"cannot safely transform unclassified message source\");\n",
		"const SOURCE_KINDS = new Set([\n\t\"user\",\n\t\"agent-message\",\n\t\"automation\",\n\t\"webhook\"\n]);\n")
	if err := ensureSessionMigrationSourceAllowlist(root); err != nil {
		t.Fatal(err)
	}
	patched, err := os.ReadFile(worker)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(patched), "\"automation\"") || !strings.Contains(string(patched), "\"webhook\"") || strings.Count(string(patched), "\"automation\"") != 1 || strings.Count(string(patched), "\"webhook\"") != 1 {
		t.Fatalf("canonical source kinds were not merged exactly once: %s", patched)
	}
	if err := ensureSessionMigrationSourceAllowlist(root); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(worker)
	if string(again) != string(patched) {
		t.Fatal("compatibility repair is not idempotent")
	}
}

func TestEnsureSessionMigrationSourceAllowlistRejectsUnknownWorker(t *testing.T) {
	root := t.TempDir()
	worker := writeSessionCompatFixture(t, root,
		"const SOURCE_KINDS = new Set([\n\tSOURCE_KIND\n]);\nthrow new Error(\"cannot safely transform unclassified message source\");\n",
		"const SOURCE_KINDS = new Set([\n\t\"user\",\n\t\"automation\"\n]);\n")
	original, err := os.ReadFile(worker)
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureSessionMigrationSourceAllowlist(root); err == nil {
		t.Fatal("unknown worker was patched instead of rejected")
	}
	unchanged, _ := os.ReadFile(worker)
	if string(unchanged) != string(original) {
		t.Fatal("unknown worker was modified")
	}
}
