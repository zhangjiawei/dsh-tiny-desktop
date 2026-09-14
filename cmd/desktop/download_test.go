package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStreamAuthenticatedDownloadUsesDSHCookieAndAtomicReplace(t *testing.T) {
	const body = "session export\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			http.SetCookie(w, &http.Cookie{Name: "dsh-session", Value: "authenticated", Path: "/"})
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html>ready</html>"))
		case "/api/session/export":
			if r.Header.Get("Cookie") != "dsh-session=authenticated" {
				http.Error(w, "missing cookie", http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(body))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	destination := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(destination, []byte("old content"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := streamAuthenticatedDownload(context.Background(), server.URL+"/?token=long-lived-token-value", server.URL+"/api/session/export", destination); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Fatalf("downloaded body = %q", got)
	}
}

func TestStreamAuthenticatedDownloadKeepsExistingFileOnFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.SetCookie(w, &http.Cookie{Name: "dsh-session", Value: "authenticated", Path: "/"})
			_, _ = w.Write([]byte("<html>ready</html>"))
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	destination := filepath.Join(t.TempDir(), "session.jsonl")
	const original = "keep this file"
	if err := os.WriteFile(destination, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if err := streamAuthenticatedDownload(context.Background(), server.URL+"/?token=long-lived-token-value", server.URL+"/missing", destination); err == nil {
		t.Fatal("failed download unexpectedly succeeded")
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != original {
		t.Fatalf("failed download changed existing file: %q", got)
	}
}
