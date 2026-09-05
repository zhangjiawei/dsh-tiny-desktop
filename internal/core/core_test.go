package core

import (
	"archive/zip"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsAtomicRoundTrip(t *testing.T) {
	p, e := NewPaths(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	s := Defaults()
	s.Proxy = "http://127.0.0.1:7890"
	if e = p.SaveSettings(s); e != nil {
		t.Fatal(e)
	}
	got, e := p.LoadSettings()
	if e != nil || got != s {
		t.Fatalf("%+v %v", got, e)
	}
	bad := s
	bad.Port = 80
	if p.SaveSettings(bad) == nil {
		t.Fatal("invalid settings accepted")
	}
	got, _ = p.LoadSettings()
	if got != s {
		t.Fatal("valid settings overwritten")
	}
}
func TestSettingsNoProxyCredentials(t *testing.T) {
	s := Defaults()
	s.Proxy = "https://user:secret@proxy.example"
	if s.Validate() == nil {
		t.Fatal("embedded credentials accepted")
	}
}
func TestLaunchURLBoundary(t *testing.T) {
	token := strings.Repeat("a", 32)
	for _, tc := range []struct {
		url string
		ok  bool
	}{{"http://127.0.0.1:3080/?token=" + token, true}, {"http://evil.test:3080/?token=" + token, false}, {"http://127.0.0.1:3081/?token=" + token, false}, {"http://127.0.0.1:3080/?token=short", false}, {"http://127.0.0.1:3080/?token=" + token + "&token=" + token, false}} {
		_, ok := ParseLaunchURL("dsh web: "+tc.url, 3080)
		if ok != tc.ok {
			t.Errorf("%s = %v", Redact(tc.url), ok)
		}
	}
}
func TestAuthRequiresCookieAndHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("token") == "valid" {
			http.SetCookie(w, &http.Cookie{Name: "auth", Value: "yes", Path: "/", HttpOnly: true})
			http.Redirect(w, r, "/", 302)
			return
		}
		if _, e := r.Cookie("auth"); e != nil {
			w.WriteHeader(401)
			return
		}
		io.WriteString(w, "<!DOCTYPE html><html>ready</html>")
	}))
	defer srv.Close()
	if e := VerifyLaunchURL(context.Background(), srv.URL+"/?token=valid"); e != nil {
		t.Fatal(e)
	}
	if VerifyLaunchURL(context.Background(), srv.URL+"/") == nil {
		t.Fatal("unauthenticated page accepted")
	}
}
func TestPortConflictDoesNotKillOwner(t *testing.T) {
	ln, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	alternative, e := CandidatePort(port)
	if e != nil || alternative == port {
		t.Fatal(alternative, e)
	}
	c, e := net.Dial("tcp", ln.Addr().String())
	if e != nil {
		t.Fatal("unrelated listener lost", e)
	}
	c.Close()
}
func TestRedactionAndSplitLines(t *testing.T) {
	s := Redact("dsh web: http://127.0.0.1:3080/?token=VERY_SECRET api_key=OTHER_SECRET")
	if strings.Contains(s, "VERY_SECRET") || strings.Contains(s, "OTHER_SECRET") {
		t.Fatal(s)
	}
	var lines []string
	if e := ReadLines(strings.NewReader("a\nb\n"), func(s string) { lines = append(lines, s) }); e != nil || len(lines) != 2 {
		t.Fatal(e, lines)
	}
}
func TestArchiveRejectsTraversal(t *testing.T) {
	for _, name := range []string{"../escape", "/absolute", "x/../../escape", `x\..\escape`, "C:/escape"} {
		if _, e := safeArchivePath(t.TempDir(), name); e == nil {
			t.Fatal(name)
		}
	}
	if _, e := safeArchivePath(t.TempDir(), "node/bin/node"); e != nil {
		t.Fatal(e)
	}
}
func TestZipExtractionAndTraversal(t *testing.T) {
	for _, name := range []string{"node/bin/node", "../escape"} {
		root := t.TempDir()
		file := filepath.Join(root, "archive.zip")
		f, _ := os.Create(file)
		z := zip.NewWriter(f)
		w, _ := z.Create(name)
		io.WriteString(w, "hello")
		z.Close()
		f.Close()
		dest := filepath.Join(root, "stage")
		os.Mkdir(dest, 0700)
		e := extractArchive(file, dest, true)
		if name == "../escape" && e == nil {
			t.Fatal("zip traversal accepted")
		}
		if name != "../escape" && e != nil {
			t.Fatal(e)
		}
	}
}
func TestImportPreviewBackupRestore(t *testing.T) {
	source := t.TempDir()
	os.WriteFile(filepath.Join(source, "settings.yaml"), []byte("language: zh"), 0600)
	os.Mkdir(filepath.Join(source, "sessions"), 0700)
	os.WriteFile(filepath.Join(source, "sessions", "same.json"), []byte("source must not win"), 0600)
	os.WriteFile(filepath.Join(source, "sessions", "new.json"), []byte("source addition"), 0600)
	os.WriteFile(filepath.Join(source, ".credentials.yaml"), []byte("private"), 0600)
	os.Mkdir(filepath.Join(source, "profiles"), 0700)
	os.WriteFile(filepath.Join(source, "profiles", "code.js"), []byte("do not copy"), 0600)
	preview, e := PreviewImport(source, false)
	if e != nil || preview.Files != 3 {
		t.Fatal(preview, e)
	}
	p, _ := NewPaths(t.TempDir())
	os.WriteFile(filepath.Join(p.Data, "keep"), []byte("original"), 0600)
	os.Mkdir(filepath.Join(p.Data, "sessions"), 0700)
	os.WriteFile(filepath.Join(p.Data, "sessions", "same.json"), []byte("tiny wins"), 0600)
	m := NewManager(p, Defaults())
	mergedPreview, e := m.PreviewImport(source, false)
	if e != nil || mergedPreview.Conflicts != 1 || mergedPreview.Files != 2 {
		t.Fatalf("unexpected Tiny-wins preview: %+v, %v", mergedPreview, e)
	}
	backup, e := m.Import(source, false)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(p.Data, ".credentials.yaml")); !os.IsNotExist(e) {
		t.Fatal("credentials imported")
	}
	if _, e = os.Stat(filepath.Join(p.Data, "profiles")); !os.IsNotExist(e) {
		t.Fatal("executable profile imported")
	}
	if b, readErr := os.ReadFile(filepath.Join(p.Data, "sessions", "same.json")); readErr != nil || string(b) != "tiny wins" {
		t.Fatalf("source overwrote Tiny data: %q, %v", b, readErr)
	}
	if b, readErr := os.ReadFile(filepath.Join(p.Data, "sessions", "new.json")); readErr != nil || string(b) != "source addition" {
		t.Fatalf("source addition missing: %q, %v", b, readErr)
	}
	if e = m.RestoreBackup(backup); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(p.Data, "sessions", "new.json")); !os.IsNotExist(e) {
		t.Fatal("overlay restore retained imported addition")
	}
	b, e := os.ReadFile(filepath.Join(p.Data, "keep"))
	if e != nil || string(b) != "original" {
		t.Fatal("backup lost")
	}
}
func TestImportSkipsSymlinks(t *testing.T) {
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "settings.yaml"), []byte("language: zh"), 0600)
	if e := os.Symlink(t.TempDir(), filepath.Join(src, "sessions")); e != nil {
		t.Skip(e)
	}
	preview, e := PreviewImport(src, false)
	if e != nil {
		t.Fatal(e)
	}
	if preview.Skipped != 1 || len(preview.SkippedItems) != 1 || !strings.Contains(preview.SkippedItems[0], "sessions") {
		t.Fatalf("symlink skip not reported: %+v", preview)
	}
}

func TestImportStillRequiresRealSettingsFile(t *testing.T) {
	src := t.TempDir()
	target := filepath.Join(t.TempDir(), "settings.yaml")
	if e := os.WriteFile(target, []byte("language: zh"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(target, filepath.Join(src, "settings.yaml")); e != nil {
		t.Skip(e)
	}
	if _, e := PreviewImport(src, false); e == nil || !strings.Contains(e.Error(), "settings.yaml") {
		t.Fatalf("required settings symlink must fail explicitly: %v", e)
	}
}

func TestOverlayRestoreRejectsTraversalBeforeRemovingData(t *testing.T) {
	paths, e := NewPaths(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	kept := filepath.Join(paths.Data, "sessions", "kept.json")
	if e = os.MkdirAll(filepath.Dir(kept), 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(kept, []byte("keep"), 0600); e != nil {
		t.Fatal(e)
	}
	backup := filepath.Join(paths.Root, "backup-malformed")
	if e = os.Mkdir(backup, 0700); e != nil {
		t.Fatal(e)
	}
	marker, _ := json.Marshal(importOverlayBackup{Version: 1, Files: []string{"sessions/kept.json", "nested/../../outside"}})
	if e = os.WriteFile(filepath.Join(backup, importOverlayMarker), marker, 0600); e != nil {
		t.Fatal(e)
	}
	manager := NewManager(paths, Defaults())
	if e = manager.RestoreBackup(backup); e == nil {
		t.Fatal("malformed overlay backup was accepted")
	}
	if contents, readErr := os.ReadFile(kept); readErr != nil || string(contents) != "keep" {
		t.Fatalf("restore partially removed data before validation: %q, %v", contents, readErr)
	}
}
func TestSnapshotNeverContainsToken(t *testing.T) {
	p, _ := NewPaths(t.TempDir())
	m := NewManager(p, Defaults())
	m.launch = "secret"
	m.phase = "running"
	b, _ := json.Marshal(m.Snapshot())
	if strings.Contains(string(b), "secret") {
		t.Fatal("token leaked")
	}
	u, e := m.LaunchURL()
	if e != nil || u != "secret" {
		t.Fatal(e)
	}
}

func TestPersistentLogRedactsQuotedCredentials(t *testing.T) {
	p, _ := NewPaths(t.TempDir())
	m := NewManager(p, Defaults())
	m.log.Add(`{"apiKey":"NEVER_PERSIST","authorization":"Bearer PRIVATE_BEARER"}`)
	b, e := os.ReadFile(filepath.Join(p.Logs, "runtime.log"))
	if e != nil {
		t.Fatal(e)
	}
	for _, secret := range []string{"NEVER_PERSIST", "PRIVATE_BEARER"} {
		if strings.Contains(string(b), secret) {
			t.Fatal("credential persisted")
		}
	}
}
func TestManifestSupportsAllReleaseTargets(t *testing.T) {
	for _, p := range []string{"darwin/amd64", "darwin/arm64", "windows/amd64", "windows/arm64", "linux/amd64", "linux/arm64"} {
		a, e := assetFor(p)
		if e != nil || len(a.SHA256) != 64 || !strings.HasPrefix(a.URL, "https://nodejs.org/dist/v24.") {
			t.Fatal(p, a, e)
		}
	}
}
