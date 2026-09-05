package core

import (
	"context"
	"github.com/coder/websocket"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestActualPrivateAddress(t *testing.T) {
	if os.Getenv("DSH_TINY_TEST_LAN") != "1" {
		t.Skip("explicit local network probe only")
	}
	ip, e := privateAddress()
	if e != nil {
		t.Fatal(e)
	}
	ln, e := net.Listen("tcp4", net.JoinHostPort(ip, "0"))
	if e != nil {
		t.Fatal(e)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ready") })}
	go server.Serve(ln)
	defer server.Close()
	client := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: nil}}
	r, e := client.Get("http://" + ln.Addr().String())
	if e != nil {
		t.Fatal(e)
	}
	r.Body.Close()
}

func TestLANAddressPrefersDefaultRouteOverVirtualAdapters(t *testing.T) {
	candidates := []interfaceAddress{
		{Name: "vEthernet (WSL)", IP: net.ParseIP("172.29.112.1")},
		{Name: "Wi-Fi", IP: net.ParseIP("172.16.8.136")},
	}
	if got := selectPrivateAddress(net.ParseIP("172.16.8.136"), candidates); got != "172.16.8.136" {
		t.Fatalf("selected %q, want physical default-route address", got)
	}
	if got := selectPrivateAddress(nil, candidates); got != "172.16.8.136" {
		t.Fatalf("fallback selected virtual adapter: %q", got)
	}
}

func TestLANPreservesAuthorityAndWebsocket(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") != "websocket" {
			io.WriteString(w, r.Host)
			return
		}
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer conn.CloseNow()
		kind, data, err := conn.Read(r.Context())
		if err == nil {
			_ = conn.Write(r.Context(), kind, data)
		}
	}))
	defer upstream.Close()
	target, _ := url.Parse(upstream.URL)
	edge := httptest.NewUnstartedServer(nil)
	authority := edge.Listener.Addr().String()
	edge.Config.Handler = newLANHandler(target, authority)
	edge.Start()
	defer edge.Close()
	res, err := http.Get(edge.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if string(body) != authority {
		t.Fatalf("host rewritten: %s", body)
	}
	req, _ := http.NewRequest("GET", edge.URL, nil)
	req.Host = "evil.example"
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatal("foreign Host accepted")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(edge.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	if err = conn.Write(ctx, websocket.MessageText, []byte("echo")); err != nil {
		t.Fatal(err)
	}
	_, data, err := conn.Read(ctx)
	if err != nil || string(data) != "echo" {
		t.Fatal(string(data), err)
	}
}

func TestLANQRHandoffDoesNotSpendAuthenticationInScanner(t *testing.T) {
	const token = "test-only-qr-token-1234567890"
	upstreamHits := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits++
		if r.Method == http.MethodGet && r.URL.Path == "/" && r.URL.Query().Get("token") == token {
			http.SetCookie(w, &http.Cookie{Name: "dsh-test", Value: "authenticated", Path: "/", HttpOnly: true})
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		if cookie, err := r.Cookie("dsh-test"); err == nil && cookie.Value == "authenticated" {
			io.WriteString(w, "ready")
			return
		}
		http.Error(w, "authentication required", http.StatusUnauthorized)
	}))
	defer upstream.Close()
	target, _ := url.Parse(upstream.URL)
	edge := httptest.NewUnstartedServer(nil)
	authority := edge.Listener.Addr().String()
	edge.Config.Handler = newLANHandler(target, authority)
	edge.Start()
	defer edge.Close()

	// QR scanners commonly preview a URL before handing it to the user's real
	// browser. A preview must receive a confirmation page without exchanging the
	// DSH launch token or redirecting to the clean, unauthenticated root URL.
	qrURL := edge.URL + "/.dsh-tiny/share?token=" + token
	preview, err := (&http.Client{Transport: &http.Transport{Proxy: nil}}).Get(qrURL)
	if err != nil {
		t.Fatal(err)
	}
	previewBody, _ := io.ReadAll(preview.Body)
	preview.Body.Close()
	if preview.StatusCode != http.StatusOK || !strings.Contains(string(previewBody), "<form") {
		t.Fatalf("scanner preview consumed or rejected QR handoff: HTTP %d", preview.StatusCode)
	}
	if strings.Contains(string(previewBody), token) {
		t.Fatal("bearer token was copied into the confirmation page body")
	}
	if preview.Header.Get("Cache-Control") != "no-store" || preview.Header.Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("QR confirmation page may be cached or leak its bearer URL")
	}
	if upstreamHits != 0 {
		t.Fatalf("scanner preview reached DSH authentication: %d requests", upstreamHits)
	}

	// The user's explicit tap posts the same bearer URL. Only then may the edge
	// redirect to DSH, letting this browser retain the authority-bound cookie.
	jar, _ := cookiejar.New(nil)
	browser := &http.Client{Jar: jar, Transport: &http.Transport{Proxy: nil}}
	response, err := browser.Post(qrURL, "application/x-www-form-urlencoded", nil)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || string(body) != "ready" {
		t.Fatalf("explicit QR confirmation did not authenticate: HTTP %d", response.StatusCode)
	}
}

func TestQRShareURLKeepsBearerButUsesLANHandoff(t *testing.T) {
	const token = "test-only-qr-token-1234567890"
	m := &Manager{
		phase:  "running",
		launch: "http://127.0.0.1:3080/?token=" + token,
		lanIP:  "172.16.8.136",
		port:   3080,
	}
	direct, err := m.ShareURL()
	if err != nil {
		t.Fatal(err)
	}
	qr, err := m.QRShareURL()
	if err != nil {
		t.Fatal(err)
	}
	directURL, _ := url.Parse(direct)
	qrURL, _ := url.Parse(qr)
	if directURL.Path != "/" || qrURL.Path != qrSharePath {
		t.Fatalf("unexpected direct/QR paths: %q %q", directURL.Path, qrURL.Path)
	}
	if directURL.Host != qrURL.Host || qrURL.Host != "172.16.8.136:3080" {
		t.Fatalf("unexpected share authorities: %q %q", directURL.Host, qrURL.Host)
	}
	if directURL.Query().Get("token") != token || qrURL.Query().Get("token") != token {
		t.Fatal("direct or QR share URL lost its bearer token")
	}
}
