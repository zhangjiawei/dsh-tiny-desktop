package core

import (
	"context"
	"github.com/coder/websocket"
	"io"
	"net"
	"net/http"
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
