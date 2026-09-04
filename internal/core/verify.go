package core

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed verify-runtime.cjs
var verifyRuntimeScript string

// VerifyInstallation is used by the release smoke test. Check the final bundle
// receipt and launch a real native terminal in both the CLI and plugin profiles.
func (m *Manager) VerifyInstallation(ctx context.Context) error {
	m.mu.Lock()
	s := m.settings
	m.mu.Unlock()
	i := Installer{m.paths, s, &m.log}
	r, err := i.Ensure(ctx)
	if err != nil {
		return err
	}
	var pkg struct {
		Dependencies map[string]string `json:"dependencies"`
		DSH          struct {
			Profile struct {
				Bundles []string `json:"bundles"`
			} `json:"profile"`
		} `json:"dsh"`
	}
	b, err := os.ReadFile(filepath.Join(m.paths.Data, "profiles/web/package.json"))
	if err != nil {
		return err
	}
	if err = json.Unmarshal(b, &pkg); err != nil {
		return err
	}
	receipt, err := i.readReceipt()
	if err != nil {
		return err
	}
	for _, p := range receipt.Plugins {
		found := false
		for _, bundle := range pkg.DSH.Profile.Bundles {
			if bundle == p.Name {
				found = true
			}
		}
		if !found || pkg.Dependencies[p.Name] != p.Version {
			return fmt.Errorf("插件未按固定版本注册: %s", p.Name)
		}
	}
	dirs := []string{filepath.Join(m.paths.Runtime, "dsh"), filepath.Join(m.paths.Data, "profiles/web")}
	if strings.Contains(s.Command, "dlx") {
		// Also exercise the actual dlx cache, not only the managed CLI. A web
		// server can boot even when pnpm skipped a PTY helper's build script.
		matches, _ := filepath.Glob(filepath.Join(m.paths.Runtime, "pnpm-cache/dlx/*/pkg/node_modules/.pnpm/@deepseek-ai+dsh@*/node_modules/@deepseek-ai/dsh"))
		if len(matches) == 0 {
			return fmt.Errorf("未找到自定义 dlx 启动的独立缓存，无法验证其原生终端")
		}
		dirs = append(dirs, matches...)
	}
	for _, dir := range dirs {
		if err = i.run(ctx, r, "-e", verifyRuntimeScript, dir); err != nil {
			return fmt.Errorf("原生终端验证失败: %w", err)
		}
	}
	return nil
}
