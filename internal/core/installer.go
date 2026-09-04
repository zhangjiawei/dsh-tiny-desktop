package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type Runtime struct{ Node, NPM, CLI, Bin string }
type Installer struct {
	Paths    Paths
	Settings Settings
	Log      *Log
}

func (i *Installer) environment(r Runtime) []string {
	// Explicitly replace inherited DSH_HOME and NODE_OPTIONS: a GUI launched from
	// a terminal must not leak instrumentation or configuration into this runtime.
	env := []string{}
	for _, v := range os.Environ() {
		k, _, _ := strings.Cut(v, "=")
		switch strings.ToUpper(k) {
		case "DSH_HOME", "NODE_OPTIONS", "PATH", "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY":
			continue
		}
		env = append(env, v)
	}
	env = append(env, "DSH_HOME="+i.Paths.Data, "PATH="+r.Bin+string(os.PathListSeparator)+filepath.Dir(r.Node)+string(os.PathListSeparator)+os.Getenv("PATH"), "CI=true")
	env = append(env, "npm_config_registry="+i.Settings.Registry)
	if i.Settings.Proxy != "" {
		env = append(env, "HTTP_PROXY="+i.Settings.Proxy, "HTTPS_PROXY="+i.Settings.Proxy)
	}
	return env
}
func (i *Installer) run(ctx context.Context, r Runtime, args ...string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	cmd := exec.Command(r.Node, args...)
	cmd.Env = i.environment(r)
	cmd.Dir = i.Paths.Runtime
	prepareProcess(cmd)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	if err = cmd.Start(); err != nil {
		return err
	}
	group, err := attachProcess(cmd)
	if err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		return err
	}
	defer group.close()
	// CommandContext alone only terminates the direct child, not npm lifecycle
	// children. Cancel owns the process group and never scans/kills by name.
	finished := make(chan struct{})
	defer close(finished)
	go func() {
		select {
		case <-ctx.Done():
			group.kill()
		case <-finished:
		}
	}()
	scanErr := ReadLines(out, i.Log.Add)
	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("安装命令失败: %w", err)
	}
	return scanErr
}
func (i *Installer) node(ctx context.Context) (Runtime, error) {
	if p, err := exec.LookPath("node"); err == nil {
		b, e := exec.CommandContext(ctx, p, "--version").Output()
		v := strings.Split(strings.TrimPrefix(strings.TrimSpace(string(b)), "v"), ".")
		major, _ := strconv.Atoi(v[0])
		if e == nil && major >= 24 {
			real, _ := filepath.EvalSymlinks(p)
			for _, npm := range []string{filepath.Join(filepath.Dir(real), "../lib/node_modules/npm/bin/npm-cli.js"), filepath.Join(filepath.Dir(real), "node_modules/npm/bin/npm-cli.js")} {
				if _, e = os.Stat(npm); e == nil {
					return Runtime{Node: p, NPM: npm}, nil
				}
			}
		}
	}
	asset, err := assetFor(runtime.GOOS + "/" + runtime.GOARCH)
	if err != nil {
		return Runtime{}, err
	}
	base := filepath.Base(asset.URL)
	folder := strings.TrimSuffix(strings.TrimSuffix(base, ".tar.gz"), ".zip")
	root := filepath.Join(i.Paths.Runtime, folder)
	node := filepath.Join(root, "bin", exeName("node"))
	npm := filepath.Join(root, "lib/node_modules/npm/bin/npm-cli.js")
	if runtime.GOOS == "windows" {
		node = filepath.Join(root, "node.exe")
		npm = filepath.Join(root, "node_modules/npm/bin/npm-cli.js")
	}
	if _, err = os.Stat(node); err == nil {
		return Runtime{Node: node, NPM: npm}, nil
	}
	i.Log.Add("正在下载独立 Node.js " + NodeVersion + "，并验证 SHA-256")
	stage, err := os.MkdirTemp(i.Paths.Runtime, ".node-*")
	if err != nil {
		return Runtime{}, err
	}
	defer os.RemoveAll(stage)
	archive := filepath.Join(stage, "download")
	if err = download(ctx, asset, archive, i.Settings.Proxy); err != nil {
		return Runtime{}, err
	}
	if err = extractArchive(archive, stage, runtime.GOOS == "windows"); err != nil {
		return Runtime{}, err
	}
	if err = os.Rename(filepath.Join(stage, folder), root); err != nil {
		return Runtime{}, err
	}
	return Runtime{Node: node, NPM: npm}, nil
}
func (i *Installer) Ensure(ctx context.Context) (Runtime, error) {
	r, err := i.node(ctx)
	if err != nil {
		return r, err
	}
	toolsDir := filepath.Join(i.Paths.Runtime, "tools")
	r.Bin = filepath.Join(toolsDir, "node_modules/.bin")
	r.CLI = filepath.Join(i.Paths.Runtime, "dsh/node_modules/@deepseek-ai/dsh/lib/bin.js")
	receipt := filepath.Join(i.Paths.Runtime, "receipt.json")
	previous, receiptErr := i.readReceipt()
	if receiptErr == nil && previous.DSH == DSHVersion && previous.PNPM == PnpmVersion && previous.Policy == i.Settings.PluginPolicy && previous.Registry == i.Settings.Registry {
		if _, e := os.Stat(r.CLI); e == nil {
			return r, nil
		}
	}
	selected, err := i.resolvePlugins(ctx)
	if err != nil {
		return r, err
	}
	expected, _ := json.Marshal(installReceipt{DSHVersion, PnpmVersion, selected, i.Settings.PluginPolicy, i.Settings.Registry})
	registryArg := "--registry=" + i.Settings.Registry
	if err = os.MkdirAll(toolsDir, 0700); err != nil {
		return r, err
	}
	i.Log.Add("安装独立 pnpm " + PnpmVersion)
	if err = i.run(ctx, r, r.NPM, "install", "--prefix", toolsDir, "--save-exact", "--ignore-scripts", "--no-audit", "--no-fund", registryArg, "pnpm@"+PnpmVersion); err != nil {
		return r, err
	}
	i.Log.Add("安装固定版本 DSH " + DSHVersion)
	dshDir := filepath.Join(i.Paths.Runtime, "dsh")
	if err = os.MkdirAll(dshDir, 0700); err != nil {
		return r, err
	}
	policy := []byte(`{"private":true,"allowScripts":{"@deepseek-ai/dsh-subprocess-local@0.1.2-rc.1":true,"node-pty@1.2.0-beta.15":true,"koffi@3.2.1":true}}`)
	if err = AtomicWrite(filepath.Join(dshDir, "package.json"), policy, 0600); err != nil {
		return r, err
	}
	if err = i.run(ctx, r, r.NPM, "install", "--prefix", dshDir, "--save-exact", "--ignore-scripts", "--no-audit", "--no-fund", registryArg, "@deepseek-ai/dsh@"+DSHVersion); err != nil {
		return r, err
	}
	if err = i.run(ctx, r, r.NPM, "rebuild", "--prefix", dshDir, "--ignore-scripts=false", "@deepseek-ai/dsh-subprocess-local@0.1.2-rc.1", "node-pty@1.2.0-beta.15", "koffi@3.2.1"); err != nil {
		return r, err
	}
	// Let the official CLI initialize and reconcile its profile. No private
	// cordis config format is invented by the desktop shell.
	i.Log.Add("初始化独立 Web profile")
	if err = i.run(ctx, r, r.CLI, "plugin", "--profile", "web", "install", registryArg); err != nil {
		return r, err
	}
	profile := filepath.Join(i.Paths.Data, "profiles/web")
	if _, err = os.Stat(profile); err != nil {
		return r, errors.New("DSH profile 未在预期独立目录生成")
	}
	// Only known native dependencies may execute build scripts. Never approve-all.
	if err = AtomicWrite(filepath.Join(profile, "pnpm-workspace.yaml"), []byte("autoInstallPeers: false\nnodeLinker: hoisted\nonlyBuiltDependencies:\n  - esbuild\n  - node-pty\n"), 0600); err != nil {
		return r, err
	}
	args := []string{r.CLI, "plugin", "--profile", "web", "add", "--save-exact", registryArg}
	for _, p := range selected {
		args = append(args, p.Name+"@"+p.Version)
	}
	i.Log.Add("安装 6 个默认插件")
	if err = i.run(ctx, r, args...); err != nil {
		return r, err
	}
	if err = AtomicWrite(receipt, expected, 0600); err != nil {
		return r, err
	}
	return r, nil
}
