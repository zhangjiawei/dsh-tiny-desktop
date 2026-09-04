package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type installReceipt struct {
	DSH, PNPM        string
	Plugins          []Plugin
	Policy, Registry string
}

var exactVersion = regexp.MustCompile(`^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)

func (i *Installer) readReceipt() (installReceipt, error) {
	var receipt installReceipt
	b, err := os.ReadFile(filepath.Join(i.Paths.Runtime, "receipt.json"))
	if err != nil {
		return receipt, err
	}
	err = json.Unmarshal(b, &receipt)
	if err == nil {
		if len(receipt.Plugins) != len(Plugins) {
			return receipt, fmt.Errorf("插件安装记录不完整")
		}
		for n, p := range receipt.Plugins {
			if p.Name != Plugins[n].Name || !exactVersion.MatchString(p.Version) {
				return receipt, fmt.Errorf("插件安装记录无效")
			}
		}
	}
	// v0.1 receipts predate the explicit policy; migrate their meaning, not data.
	if receipt.Policy == "" {
		receipt.Policy = "pinned"
	}
	if receipt.Registry == "" {
		receipt.Registry = "https://registry.npmjs.org"
	}
	return receipt, err
}
func (i *Installer) resolvePlugins(ctx context.Context) ([]Plugin, error) {
	selected := append([]Plugin(nil), Plugins...)
	if i.Settings.PluginPolicy != "latest" {
		return selected, nil
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if i.Settings.Proxy != "" {
		u, _ := url.Parse(i.Settings.Proxy)
		transport.Proxy = http.ProxyURL(u)
	}
	client := &http.Client{Transport: transport, Timeout: 30 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" || len(via) > 5 {
			return fmt.Errorf("拒绝不安全仓库重定向")
		}
		return nil
	}}
	defer client.CloseIdleConnections()
	for n, p := range selected {
		endpoint := strings.TrimRight(i.Settings.Registry, "/") + "/" + url.PathEscape(p.Name) + "/latest"
		req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
		if err != nil {
			return nil, err
		}
		res, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		var meta struct {
			Version string `json:"version"`
		}
		err = json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(&meta)
		res.Body.Close()
		if res.StatusCode != 200 || err != nil || !exactVersion.MatchString(meta.Version) {
			return nil, fmt.Errorf("无法解析插件最新版: %s (HTTP %d)", p.Name, res.StatusCode)
		}
		selected[n].Version = meta.Version
		i.Log.Add("解析最新版 " + p.Name + "@" + meta.Version)
	}
	return selected, nil
}
