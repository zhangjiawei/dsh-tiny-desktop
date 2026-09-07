package core

import (
	_ "embed"
	"errors"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const qrSharePath = "/.dsh-tiny/share"

//go:embed qr_share_page.html
var qrSharePage []byte

type interfaceAddress struct {
	Name string
	IP   net.IP
}

func privateAddress() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	var candidates []interfaceAddress
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, _ := iface.Addrs()
		for _, address := range addresses {
			ip, _, e := net.ParseCIDR(address.String())
			if e == nil && ip.To4() != nil && ip.IsPrivate() {
				candidates = append(candidates, interfaceAddress{Name: iface.Name, IP: ip})
			}
		}
	}
	if selected := selectPrivateAddress(defaultRouteAddress(), candidates); selected != "" {
		return selected, nil
	}
	return "", errors.New("未找到可用的私有 IPv4 局域网地址")
}

func defaultRouteAddress() net.IP {
	// UDP connect selects a route without sending a packet. Its local endpoint
	// identifies the interface used for ordinary LAN/internet traffic, avoiding
	// WSL/Hyper-V host-only adapters that often enumerate first on Windows.
	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 9})
	if err != nil {
		return nil
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP
}

func selectPrivateAddress(route net.IP, candidates []interfaceAddress) string {
	if route != nil && route.To4() != nil && route.IsPrivate() {
		for _, candidate := range candidates {
			if candidate.IP.Equal(route) {
				return candidate.IP.String()
			}
		}
	}
	best, bestScore := "", -10000
	for _, candidate := range candidates {
		score := 100
		name := strings.ToLower(candidate.Name)
		for _, marker := range []string{"vethernet", "wsl", "hyper-v", "vmware", "virtualbox", "docker", "tailscale", "zerotier", "utun", "veth", "virbr", "vmnet", "wireguard"} {
			if strings.Contains(name, marker) {
				score -= 1000
				break
			}
		}
		if score > bestScore {
			best, bestScore = candidate.IP.String(), score
		}
	}
	return best
}

// LAN access never disables DSH authentication. Keep the incoming Host and
// Origin so DSH issues a cookie bound to the LAN authority, not to loopback.
func lanProxy(ip string, port int) (*http.Server, error) {
	authority := net.JoinHostPort(ip, strconv.Itoa(port))
	listener, err := net.Listen("tcp4", authority)
	if err != nil {
		return nil, err
	}
	target := &url.URL{Scheme: "http", Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(port))}
	server := &http.Server{ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20, Handler: newLANHandler(target, authority)}
	go server.Serve(listener)
	return server, nil
}
func newLANHandler(target *url.URL, authority string) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &http.Transport{Proxy: nil}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		http.Error(w, "DSH 暂时不可用", http.StatusBadGateway)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != authority {
			http.Error(w, "Invalid Host", http.StatusForbidden)
			return
		}
		if serveQRShare(w, r) {
			return
		}
		proxy.ServeHTTP(w, r)
	})
}

// serveQRShare prevents QR-scanner previews from spending the token-to-cookie
// exchange in a browser container that is discarded during handoff. GET/HEAD
// never reach DSH; only the user's explicit POST redirects this same browser to
// the authenticated root. The bearer stays in the URL and is never logged.
func serveQRShare(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path != qrSharePath {
		return false
	}
	query := r.URL.Query()
	tokens := query["token"]
	if len(query) != 1 || len(tokens) != 1 || len(tokens[0]) < 20 {
		http.Error(w, "Invalid share link", http.StatusBadRequest)
		return true
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'")
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodGet {
			_, _ = w.Write(qrSharePage)
		}
	case http.MethodPost:
		destination := &url.URL{Path: "/", RawQuery: url.Values{"token": {tokens[0]}}.Encode()}
		http.Redirect(w, r, destination.String(), http.StatusSeeOther)
	default:
		w.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
	return true
}
func (m *Manager) ShareURL() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.phase != "running" {
		return "", errors.New("服务尚未就绪")
	}
	u, err := url.Parse(m.launch)
	if err != nil {
		return "", err
	}
	if m.lanIP != "" {
		u.Host = net.JoinHostPort(m.lanIP, strconv.Itoa(m.port))
	}
	return u.String(), nil
}

// QRShareURL keeps the ordinary copied URL direct, but routes LAN QR scans
// through a confirmation page so scanner previews cannot lose DSH's
// authority-bound cookie when they hand the URL to another browser.
func (m *Manager) QRShareURL() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.phase != "running" {
		return "", errors.New("服务尚未就绪")
	}
	u, err := url.Parse(m.launch)
	if err != nil {
		return "", err
	}
	if m.lanIP != "" {
		u.Host = net.JoinHostPort(m.lanIP, strconv.Itoa(m.port))
		u.Path = qrSharePath
	}
	return u.String(), nil
}

// PublicShareURL combines the configured HTTPS origin with the live in-memory
// DSH bearer token. It is computed only for an explicit user copy/share action:
// the resulting secret is never persisted, logged or included in snapshots.
func (m *Manager) PublicShareURL() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.phase != "running" {
		return "", errors.New("服务尚未就绪")
	}
	publicURL, err := normalizePublicURL(m.activeSettings.PublicURL)
	if err != nil {
		return "", err
	}
	if publicURL == "" {
		return "", errors.New("请先配置公网访问地址并应用重启")
	}
	launch, err := url.Parse(m.launch)
	if err != nil {
		return "", err
	}
	public, err := url.Parse(publicURL)
	if err != nil {
		return "", err
	}
	public.RawQuery = launch.RawQuery
	return public.String(), nil
}
