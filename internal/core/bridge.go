package core

import (
	"net/url"
	"strings"
)

// TrustedControlMessage adapts the fields actually supplied by Wails beta.16.
// Windows WebView2 supplies both document URLs, not IsMainFrame. Linux supplies
// the top document URL only; the control CSP must prohibit all nested frames.
// Do not reuse this policy for a control page that embeds external content.
func TrustedControlMessage(platform, window, origin, topOrigin string, mainFrame bool) bool {
	if !TrustedControlOrigin(window, origin, true) {
		return false
	}
	switch platform {
	case "darwin":
		return mainFrame
	case "windows":
		// WebView2 captures the message URL and current top URL at different
		// instants. SPA hash changes do not change the document or its authority.
		// Ignore ONLY fragments; retain exact scheme/host/path/query matching and
		// independently validate both URLs. The page CSP still forbids frames.
		sourceDocument, _, _ := strings.Cut(origin, "#")
		topDocument, _, _ := strings.Cut(topOrigin, "#")
		return sourceDocument == topDocument && TrustedControlOrigin(window, topOrigin, true)
	case "linux":
		return true // Wails reads the top WebView URI; frame-src 'none' is required.
	default:
		return false
	}
}

// TrustedControlOrigin checks the privileged local control document only.
func TrustedControlOrigin(window, origin string, mainFrame bool) bool {
	if window != "control" || !mainFrame {
		return false
	}
	// Wails sends a full document URL on macOS/Windows, including SPA hashes.
	// Compare the parsed authority, not the entire URL; still constrain the
	// document path so a navigated untrusted page cannot use the control bridge.
	u, err := url.Parse(origin)
	if err != nil || u.User != nil || u.RawQuery != "" || (u.Path != "" && u.Path != "/" && u.Path != "/index.html") {
		return false
	}
	return (u.Scheme == "wails" && u.Host == "localhost") || (u.Scheme == "http" && u.Host == "wails.localhost")
}

// TrustedWorkspaceMessage authenticates the intentionally tiny bridge exposed
// to the DSH WebView. It accepts only messages from the currently running DSH
// origin, never from an arbitrary page loaded into the workspace window.
func TrustedWorkspaceMessage(platform, origin, topOrigin, expected string, mainFrame bool) bool {
	if !mainFrame && platform != "windows" {
		return false
	}
	expectedURL, err := url.Parse(expected)
	if err != nil || expectedURL.User != nil || (expectedURL.Scheme != "http" && expectedURL.Scheme != "https") || expectedURL.Host == "" {
		return false
	}
	expectedAuthority := strings.ToLower(expectedURL.Scheme + "://" + expectedURL.Host)
	parseAuthority := func(raw string) string {
		u, err := url.Parse(raw)
		if err != nil || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return ""
		}
		return strings.ToLower(u.Scheme + "://" + u.Host)
	}
	if parseAuthority(origin) != expectedAuthority {
		return false
	}
	if platform == "windows" {
		originDocument, _, _ := strings.Cut(origin, "#")
		topDocument, _, _ := strings.Cut(topOrigin, "#")
		if originDocument != topDocument {
			return false
		}
	}
	if topOrigin != "" && parseAuthority(topOrigin) != expectedAuthority {
		return false
	}
	return true
}

// ExternalLinkURL keeps browser hand-off deliberately narrow. Credentials,
// local files, script URLs and other schemes must never leave the DSH window.
func ExternalLinkURL(raw string) (string, bool) {
	if len(raw) == 0 || len(raw) > 8192 {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Host == "" {
		return "", false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", false
	}
	return u.String(), true
}
