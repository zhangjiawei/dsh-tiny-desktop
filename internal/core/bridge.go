package core

import "net/url"

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
