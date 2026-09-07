package core

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

const maxTrustedAuthorities = 16

// normalizeLANAddress keeps network binding independent from Host trust. Tiny
// deliberately binds only one private IPv4 interface; public DNS names belong
// to the reverse-proxy trust list and never influence the listener address.
func normalizeLANAddress(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	ip := net.ParseIP(value)
	if ip == nil || ip.To4() == nil || !ip.IsPrivate() {
		return "", errors.New("局域网绑定地址必须是私有 IPv4；留空时自动选择默认路由网卡")
	}
	return ip.To4().String(), nil
}

// normalizeTrustedAuthority accepts the exact authority grammar implemented by
// DSH: one canonical host or host:port. Schemes, paths, wildcards and userinfo
// are rejected so a copied URL cannot silently broaden the browser trust fence.
func normalizeTrustedAuthority(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 260 || strings.ContainsAny(value, "/?#@* \\\t\r\n") {
		return "", fmt.Errorf("受信任主机 %q 必须是精确的 host 或 host:port", value)
	}
	host, port := value, ""
	hasPort := false
	if strings.HasPrefix(value, "[") {
		end := strings.IndexByte(value, ']')
		if end < 0 {
			return "", fmt.Errorf("受信任主机 %q 的 IPv6 必须使用方括号", value)
		}
		host = value[1:end]
		if rest := value[end+1:]; rest != "" {
			if !strings.HasPrefix(rest, ":") {
				return "", fmt.Errorf("受信任主机 %q 必须是精确的 host 或 host:port", value)
			}
			hasPort = true
			port = rest[1:]
		}
	} else if strings.Count(value, ":") == 1 {
		hasPort = true
		host, port, _ = strings.Cut(value, ":")
	} else if strings.Contains(value, ":") {
		return "", fmt.Errorf("受信任主机 %q 的 IPv6 必须使用方括号", value)
	}
	if hasPort {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 || strconv.Itoa(n) != port {
			return "", fmt.Errorf("受信任主机 %q 的端口必须在 1–65535 之间且不能有前导零", value)
		}
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.To4() != nil {
			host = ip.To4().String()
		} else {
			host = "[" + ip.String() + "]"
		}
	} else {
		host = strings.ToLower(host)
		if !validDNSName(host) {
			return "", fmt.Errorf("受信任主机 %q 不是有效的 ASCII 域名或 IP", value)
		}
	}
	if hasPort {
		return host + ":" + port, nil
	}
	return host, nil
}

func validDNSName(host string) bool {
	if host == "" || len(host) > 253 || strings.HasSuffix(host, ".") {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
				return false
			}
		}
	}
	return true
}

func splitTrustedAuthorities(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
}

func parseTrustedAuthorities(value string) ([]string, error) {
	items := splitTrustedAuthorities(value)
	if len(items) > maxTrustedAuthorities {
		return nil, fmt.Errorf("受信任主机最多允许 %d 个", maxTrustedAuthorities)
	}
	result := make([]string, 0, len(items))
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		normalized, err := normalizeTrustedAuthority(item)
		if err != nil {
			return nil, err
		}
		if !seen[normalized] {
			seen[normalized] = true
			result = append(result, normalized)
		}
	}
	return result, nil
}

// normalizePublicURL accepts one HTTPS origin only. Current DSH owns absolute
// root routes such as /api and does not implement a base-path flag, so accepting
// a non-root path would create an attractive but broken Tunnel configuration.
func normalizePublicURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", errors.New("公网访问地址必须是不含凭据、路径、查询或片段的 HTTPS 地址")
	}
	authority, err := normalizeTrustedAuthority(u.Host)
	if err != nil {
		return "", fmt.Errorf("公网访问地址无效: %w", err)
	}
	return "https://" + authority, nil
}

func (s Settings) effectiveTrustedAuthorities() ([]string, error) {
	items, err := parseTrustedAuthorities(s.TrustedHosts)
	if err != nil {
		return nil, err
	}
	if publicURL, err := normalizePublicURL(s.PublicURL); err != nil {
		return nil, err
	} else if publicURL != "" {
		u, _ := url.Parse(publicURL)
		items = append(items, u.Host)
	}
	result := make([]string, 0, len(items))
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		normalized, err := normalizeTrustedAuthority(item)
		if err != nil {
			return nil, err
		}
		if !seen[normalized] {
			seen[normalized] = true
			result = append(result, normalized)
		}
	}
	if len(result) > maxTrustedAuthorities {
		return nil, fmt.Errorf("公网地址与额外受信任主机合计最多允许 %d 个", maxTrustedAuthorities)
	}
	return result, nil
}

func appendTrustedAuthorities(args []string, values ...string) []string {
	existing := make(map[string]bool)
	for index := 0; index < len(args); index++ {
		key, value, inline := strings.Cut(args[index], "=")
		if key != "--trusted-host" {
			continue
		}
		if !inline && index+1 < len(args) {
			index++
			value = args[index]
		}
		if normalized, err := normalizeTrustedAuthority(value); err == nil {
			existing[normalized] = true
		}
	}
	for _, value := range values {
		if !existing[value] {
			args = append(args, "--trusted-host", value)
			existing[value] = true
		}
	}
	return args
}

func legacyLANAddress(extra string) (string, bool) {
	args, err := ParseManagedArgs(extra)
	if err != nil || len(args) != 2 || args[0] != "--trusted-host" {
		return "", false
	}
	address, err := normalizeLANAddress(args[1])
	return address, err == nil && address != ""
}
