package core

import (
	"errors"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"time"
)

func privateAddress() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, _ := iface.Addrs()
		for _, address := range addresses {
			ip, _, e := net.ParseCIDR(address.String())
			if e == nil && ip.To4() != nil && ip.IsPrivate() {
				return ip.String(), nil
			}
		}
	}
	return "", errors.New("未找到可用的私有 IPv4 局域网地址")
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
		proxy.ServeHTTP(w, r)
	})
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
