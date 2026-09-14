package core

import (
	"net/url"
	"path"
	"strings"
	"unicode"
)

// WorkspaceDownloadURL limits the download bridge to the authenticated
// authority currently loaded in the workspace. The desktop layer can then
// safely replay the request with its in-memory DSH session without exposing a
// bearer URL to another browser or to the log.
func WorkspaceDownloadURL(raw, expected string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Host == "" || u.Fragment != "" {
		return "", false
	}
	e, err := url.Parse(expected)
	if err != nil || e.User != nil || e.Host == "" || e.Fragment != "" {
		return "", false
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Scheme != e.Scheme {
		return "", false
	}
	if !strings.EqualFold(u.Host, e.Host) || u.Path == "" {
		return "", false
	}
	return u.String(), true
}

// DownloadFilename turns an HTML download hint or URL path into a harmless
// local filename. The selected save path is still controlled by the user.
func DownloadFilename(hint, rawURL string) string {
	name := strings.TrimSpace(hint)
	if name == "" {
		if u, err := url.Parse(rawURL); err == nil {
			name, _ = url.PathUnescape(path.Base(u.Path))
		}
	}
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		name = "dsh-download"
	}
	name = strings.NewReplacer("/", "-", "\\", "-", ":", "-", "\x00", "").Replace(name)
	var clean strings.Builder
	for _, r := range name {
		if unicode.IsControl(r) {
			continue
		}
		clean.WriteRune(r)
	}
	name = strings.TrimSpace(clean.String())
	if name == "" || name == "." || name == ".." {
		return "dsh-download"
	}
	// Keep the native save dialog compact and avoid platform-specific path
	// length surprises while preserving normal Unicode filenames.
	for len([]byte(name)) > 180 {
		name = string([]rune(name)[:len([]rune(name))-1])
	}
	return name
}
