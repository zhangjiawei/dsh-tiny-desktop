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
	"strings"
	"time"
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
		case "DSH_HOME", "DSH_PROFILE_DIR", "DSH_RUNTIME_DIR", "NODE_OPTIONS", "PATH", "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY":
			continue
		}
		env = append(env, v)
	}
	// Companion plugins launch `dsh plugin` themselves and then verify the
	// result through DSH_PROFILE_DIR. Pin both lookup roots explicitly so their
	// post-install check cannot fall back to the user's unrelated ~/.dsh profile.
	env = append(env,
		"DSH_HOME="+i.Paths.Data,
		"DSH_PROFILE_DIR="+filepath.Join(i.Paths.Data, "profiles", "web"),
		"DSH_RUNTIME_DIR="+filepath.Join(i.Paths.Runtime, "dsh"),
		"PATH="+r.Bin+string(os.PathListSeparator)+filepath.Dir(r.Node)+string(os.PathListSeparator)+os.Getenv("PATH"),
		"CI=true",
	)
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
	stdinStarted, closeStdin, err := attachManagedStdin(cmd)
	if err != nil {
		return err
	}
	defer closeStdin()
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	if err = cmd.Start(); err != nil {
		return err
	}
	stdinStarted()
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
		// Even a read-only version probe creates a console host when it is
		// launched by a windowsgui process unless the child is explicitly hidden.
		b, e := backgroundCommandContext(ctx, p, "--version").Output()
		if e == nil && systemNodeMatches(string(b)) {
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
	if runtimeFilesReady(node, npm) {
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
	if err = publishNodeRuntime(filepath.Join(stage, folder), root, []string{node, npm}, os.Rename, time.Sleep); err != nil {
		return Runtime{}, err
	}
	return Runtime{Node: node, NPM: npm}, nil
}

func publishNodeRuntime(source, target string, required []string, rename func(string, string) error, sleep func(time.Duration)) error {
	// A reboot, antivirus scan or indexer may leave the destination incomplete
	// or hold a freshly extracted directory for a short time on Windows. Keep an
	// incomplete destination recoverable while retrying the same-volume rename.
	if runtimeFilesReady(required...) {
		return nil
	}
	sourceRequired := make([]string, 0, len(required))
	for _, path := range required {
		relative, err := filepath.Rel(target, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("运行时校验路径超出目标目录: %s", path)
		}
		sourceRequired = append(sourceRequired, filepath.Join(source, relative))
	}
	if !runtimeFilesReady(sourceRequired...) {
		return errors.New("解压后的 Node.js 运行环境不完整，未发布到正式目录")
	}
	backup := target + ".incomplete-" + filepath.Base(filepath.Dir(source))
	quarantined := false
	var lastErr error
	for attempt := 0; attempt < 20; attempt++ {
		if runtimeFilesReady(required...) {
			// A concurrent launcher may have completed the same private runtime.
			if quarantined {
				_ = os.RemoveAll(backup)
			}
			return nil
		}
		if !quarantined {
			if _, err := os.Stat(target); err == nil {
				if err = rename(target, backup); err != nil {
					lastErr = err
					sleep(nodePublishDelay(attempt))
					continue
				}
				quarantined = true
				continue
			}
		}
		if err := rename(source, target); err == nil {
			if quarantined {
				_ = os.RemoveAll(backup)
			}
			return nil
		} else {
			lastErr = err
		}
		sleep(nodePublishDelay(attempt))
	}
	if quarantined {
		if _, err := os.Stat(target); os.IsNotExist(err) {
			if restoreErr := rename(backup, target); restoreErr != nil {
				return fmt.Errorf("发布独立 Node.js 运行环境失败，且无法恢复原目录: %v（原错误: %w）", restoreErr, lastErr)
			}
		}
	}
	return fmt.Errorf("发布独立 Node.js 运行环境失败；Windows 可能仍在扫描或占用该目录，已自动重试: %w", lastErr)
}

func runtimeFilesReady(paths ...string) bool {
	for _, path := range paths {
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			return false
		}
	}
	return true
}

func nodePublishDelay(attempt int) time.Duration {
	delay := time.Duration(attempt+1) * 100 * time.Millisecond
	if delay > time.Second {
		return time.Second
	}
	return delay
}

func systemNodeMatches(version string) bool {
	// Node is part of the reproducible private runtime contract. Accepting an
	// arbitrary 24.x system binary made Windows GUI failures depend on whatever
	// patch release happened to be globally installed.
	return strings.TrimPrefix(strings.TrimSpace(version), "v") == NodeVersion
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
	// Preserve legacy pinned receipts too: ordinary launches must not silently
	// upgrade plugins that are already installed in an existing user's profile.
	// Registry is provenance, not installation identity: changing a mirror must
	// not reinitialise a completed profile or silently upgrade its plugins.
	if receiptErr == nil && previous.DSH == DSHVersion && previous.PNPM == PnpmVersion {
		if _, e := os.Stat(r.CLI); e == nil {
			return r, nil
		}
	}
	selected, err := i.resolvePlugins(ctx)
	if err != nil {
		return r, err
	}
	expected, _ := json.Marshal(installReceipt{DSHVersion, PnpmVersion, selected, "latest", i.Settings.Registry})
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
	// CI=true keeps all child package managers non-interactive. A desktop
	// profile, unlike the source repository, is mutable and may have an old lock
	// after an interrupted install. Reconcile ONLY this private profile's lock;
	// never remove it, set a global pnpm option, or disable frozen source builds.
	if err = i.run(ctx, r, r.CLI, "plugin", "--profile", "web", "install", "--no-frozen-lockfile", registryArg); err != nil {
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
