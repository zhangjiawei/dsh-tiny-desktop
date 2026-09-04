package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var startupURLPattern = regexp.MustCompile(`dsh web:\s+(https?://[^\s)]+)`)

// ParseLaunchURL accepts only the process's expected loopback address and port.
// Arbitrary URLs emitted by third-party plugins must never become privileged UI.
func ParseLaunchURL(line string, port int) (string, bool) {
	m := startupURLPattern.FindStringSubmatch(ansiPattern.ReplaceAllString(line, ""))
	if len(m) != 2 {
		return "", false
	}
	u, err := url.Parse(m[1])
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() != strconv.Itoa(port) || u.Path != "/" || u.User != nil {
		return "", false
	}
	tokens := u.Query()["token"]
	if len(tokens) != 1 || len(tokens[0]) < 20 {
		return "", false
	}
	return u.String(), true
}
func VerifyLaunchURL(ctx context.Context, login string) error {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 5 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 3 || req.URL.Host != via[0].URL.Host || req.URL.Scheme != "http" {
			return errors.New("拒绝认证重定向到其他地址")
		}
		return nil
	}}
	defer client.CloseIdleConnections()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, login, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("认证请求失败: %s", Redact(err.Error()))
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024))
	if err != nil {
		return err
	}
	if response.StatusCode != 200 || !strings.Contains(strings.ToLower(string(body)), "<html") || strings.Contains(string(body), "authentication required") {
		return fmt.Errorf("页面未就绪 (HTTP %d)", response.StatusCode)
	}
	if len(jar.Cookies(req.URL)) == 0 {
		return errors.New("认证流程未签发会话 Cookie")
	}
	return nil
}

// CandidatePort is advisory, not a reservation. The supervisor must retry on
// EADDRINUSE because the port can be stolen between this check and Node's bind.
func CandidatePort(preferred int) (int, error) {
	ln, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(preferred)))
	if err != nil {
		ln, err = net.Listen("tcp4", "127.0.0.1:0")
	}
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}
