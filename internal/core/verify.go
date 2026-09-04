package core

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	for _, p := range Plugins {
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
	for _, dir := range []string{filepath.Join(m.paths.Runtime, "dsh"), filepath.Join(m.paths.Data, "profiles/web")} {
		if err = i.run(ctx, r, "-e", verifyRuntimeScript, dir); err != nil {
			return fmt.Errorf("原生终端验证失败: %w", err)
		}
	}
	return nil
}
