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

// Plugin replacements are applied only when an existing Tiny profile still
// contains a package that this product has explicitly replaced. Keeping this
// map separate from Plugins makes the one-time migration auditable and avoids
// treating arbitrary user-added profile dependencies as disposable.
var pluginReplacements = map[string]string{
	"@michengai/dsh-im-connect": "@xmanrui/dsh-im",
}

var exactVersion = regexp.MustCompile(`^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)

func (i *Installer) readReceipt() (installReceipt, error) {
	_, dir := i.activeRuntime()
	return i.readReceiptAt(i.receiptPath(dir))
}

func (i *Installer) readReceiptAt(path string) (installReceipt, error) {
	var receipt installReceipt
	b, err := os.ReadFile(path)
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

func (i *Installer) receiptPath(dshDir string) string {
	if dshDir == "" || filepath.Clean(dshDir) == filepath.Join(i.Paths.Runtime, "dsh") {
		return filepath.Join(i.Paths.Runtime, "receipt.json")
	}
	return filepath.Join(dshDir, "receipt.json")
}
func (i *Installer) resolvePlugins(ctx context.Context) ([]Plugin, error) {
	selected := append([]Plugin(nil), Plugins...)
	// First installation resolves all six latest tags automatically. It is not
	// a preference users must discover and enable in Settings.
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

func profileNeedsPluginMigration(profile string) (bool, error) {
	contents, err := os.ReadFile(filepath.Join(profile, "package.json"))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var manifest struct {
		Dependencies         map[string]string `json:"dependencies"`
		DevDependencies      map[string]string `json:"devDependencies"`
		OptionalDependencies map[string]string `json:"optionalDependencies"`
	}
	if err = json.Unmarshal(contents, &manifest); err != nil {
		return false, fmt.Errorf("无法读取 Web profile 依赖清单: %w", err)
	}
	for oldName := range pluginReplacements {
		if _, ok := manifest.Dependencies[oldName]; ok {
			return true, nil
		}
		if _, ok := manifest.DevDependencies[oldName]; ok {
			return true, nil
		}
		if _, ok := manifest.OptionalDependencies[oldName]; ok {
			return true, nil
		}
	}
	return false, nil
}

func obsoletePluginPackages(profile string, desired []Plugin) ([]string, error) {
	contents, err := os.ReadFile(filepath.Join(profile, "package.json"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var manifest struct {
		Dependencies         map[string]string `json:"dependencies"`
		DevDependencies      map[string]string `json:"devDependencies"`
		OptionalDependencies map[string]string `json:"optionalDependencies"`
	}
	if err = json.Unmarshal(contents, &manifest); err != nil {
		return nil, fmt.Errorf("无法读取 Web profile 依赖清单: %w", err)
	}
	present := func(name string) bool {
		_, dependency := manifest.Dependencies[name]
		if dependency {
			return true
		}
		_, dependency = manifest.DevDependencies[name]
		if dependency {
			return true
		}
		_, dependency = manifest.OptionalDependencies[name]
		return dependency
	}
	wanted := make(map[string]bool, len(desired))
	for _, plugin := range desired {
		wanted[plugin.Name] = true
	}
	removals := make([]string, 0, len(pluginReplacements))
	for oldName, newName := range pluginReplacements {
		switch {
		case wanted[newName] && present(oldName):
			removals = append(removals, oldName)
		case wanted[oldName] && present(newName):
			removals = append(removals, newName)
		}
	}
	return removals, nil
}
