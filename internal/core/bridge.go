package core

import "net/url"

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
		return topOrigin == origin && TrustedControlOrigin(window, topOrigin, true)
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
