package core

import "testing"

func TestWorkspaceDownloadURLRequiresCurrentDSHAuthority(t *testing.T) {
	expected := "http://127.0.0.1:3080/?token=hidden-token-value"
	valid, ok := WorkspaceDownloadURL("http://127.0.0.1:3080/api/session/export?format=json", expected)
	if !ok || valid == "" {
		t.Fatal("same-authority download was rejected")
	}
	if _, ok := WorkspaceDownloadURL("http://127.0.0.1:3080/files/blob.bin", expected); !ok {
		t.Fatal("generic same-authority file download was rejected")
	}
	for _, raw := range []string{
		"https://127.0.0.1:3080/api/session/export",
		"http://127.0.0.2:3080/api/session/export",
		"http://127.0.0.1:3080/api/session/export#fragment",
		"http://user@127.0.0.1:3080/api/session/export",
		"file:///tmp/export.json",
	} {
		if _, ok := WorkspaceDownloadURL(raw, expected); ok {
			t.Fatalf("unsafe download URL accepted: %s", raw)
		}
	}
}

func TestDownloadFilenameRemovesPathControlAndLengthHazards(t *testing.T) {
	if got := DownloadFilename("../session.jsonl", "http://127.0.0.1:3080/"); got != "..-session.jsonl" {
		t.Fatalf("filename sanitisation = %q", got)
	}
	if got := DownloadFilename("", "http://127.0.0.1:3080/api/export/session.jsonl"); got != "session.jsonl" {
		t.Fatalf("URL filename = %q", got)
	}
	if got := DownloadFilename("\x00\n", "http://127.0.0.1:3080/"); got != "dsh-download" {
		t.Fatalf("empty sanitised filename = %q", got)
	}
}
