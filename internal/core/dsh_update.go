package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const managedRuntimeManifest = "managed-runtime.json"

type runtimeSlot struct {
	Version string `json:"version"`
	Dir     string `json:"dir"`
}

type managedRuntimeState struct {
	Current        runtimeSlot  `json:"current"`
	Previous       *runtimeSlot `json:"previous,omitempty"`
	RollbackBackup string       `json:"rollbackBackup,omitempty"`
	UpdatedAt      string       `json:"updatedAt"`
}

// DSHUpdateInfo is safe to expose to the control window: it contains version
// metadata only, never the authenticated launch URL or registry credentials.
type DSHUpdateInfo struct {
	Mode            string `json:"mode"`
	Channel         string `json:"channel"`
	CurrentVersion  string `json:"currentVersion"`
	TargetVersion   string `json:"targetVersion"`
	PreviousVersion string `json:"previousVersion"`
	Available       bool   `json:"available"`
	CanRollback     bool   `json:"canRollback"`
	Busy            bool   `json:"busy"`
	Status          string `json:"status"`
	LastChecked     string `json:"lastChecked"`
}

type packageMetadata struct {
	DistTags map[string]string          `json:"dist-tags"`
	Versions map[string]json.RawMessage `json:"versions"`
}

func (i *Installer) activeRuntime() (string, string) {
	state, err := readManagedRuntimeState(i.Paths)
	if err == nil && state.Current.Version != "" {
		return state.Current.Version, state.Current.Dir
	}
	// Before managed-runtime.json existed, receipt.json was the only installed
	// version record. Preserve it until the user explicitly applies an update.
	var receipt installReceipt
	if contents, readErr := os.ReadFile(filepath.Join(i.Paths.Runtime, "receipt.json")); readErr == nil && json.Unmarshal(contents, &receipt) == nil && exactVersion.MatchString(receipt.DSH) {
		return receipt.DSH, filepath.Join(i.Paths.Runtime, "dsh")
	}
	return DSHVersion, filepath.Join(i.Paths.Runtime, "dsh")
}

func readManagedRuntimeState(paths Paths) (managedRuntimeState, error) {
	var state managedRuntimeState
	contents, err := os.ReadFile(filepath.Join(paths.Runtime, managedRuntimeManifest))
	if err != nil {
		return state, err
	}
	if err = json.Unmarshal(contents, &state); err != nil {
		return state, err
	}
	for _, slot := range []*runtimeSlot{&state.Current, state.Previous} {
		if slot == nil {
			continue
		}
		if !exactVersion.MatchString(slot.Version) || !validRuntimeSlot(paths, slot.Dir) {
			return managedRuntimeState{}, errors.New("DSH 运行时版本指针无效")
		}
	}
	if state.RollbackBackup != "" && !validRollbackBackup(paths, state.RollbackBackup) {
		return managedRuntimeState{}, errors.New("DSH 回退点路径无效")
	}
	return state, nil
}

func writeManagedRuntimeState(paths Paths, state managedRuntimeState) error {
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	contents, _ := json.MarshalIndent(state, "", "  ")
	return AtomicWrite(filepath.Join(paths.Runtime, managedRuntimeManifest), contents, 0600)
}

func pathInside(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel)
}

func validRuntimeSlot(paths Paths, path string) bool {
	clean := filepath.Clean(path)
	return clean == filepath.Join(paths.Runtime, "dsh") || filepath.Dir(clean) == filepath.Join(paths.Runtime, "dsh-versions")
}

func validRollbackBackup(paths Paths, path string) bool {
	clean := filepath.Clean(path)
	return filepath.Dir(clean) == filepath.Clean(paths.Runtime) && strings.HasPrefix(filepath.Base(clean), ".dsh-update-backup-")
}

func (m *Manager) localUpdateInfo() DSHUpdateInfo {
	m.mu.Lock()
	s := m.settings
	busy := m.updateBusy
	status := m.updateStatus
	target := m.updateTarget
	checked := m.updateChecked
	m.mu.Unlock()
	i := Installer{Paths: m.paths, Settings: s, Log: &m.log}
	current, _ := i.activeRuntime()
	info := DSHUpdateInfo{Mode: s.RuntimeMode, Channel: s.DSHChannel, CurrentVersion: current, TargetVersion: target, Busy: busy, Status: status, LastChecked: checked}
	info.Available = target != "" && compareVersions(target, current) != 0
	if state, err := readManagedRuntimeState(m.paths); err == nil && state.Previous != nil {
		info.PreviousVersion = state.Previous.Version
		info.CanRollback = true
	}
	if s.RuntimeMode == RuntimeModeCustom {
		info.Status = "自定义命令控制 DSH 版本"
	}
	return info
}

func (m *Manager) CheckDSHUpdate(ctx context.Context) (DSHUpdateInfo, error) {
	info := m.localUpdateInfo()
	m.mu.Lock()
	s := m.settings
	m.mu.Unlock()
	if s.RuntimeMode != RuntimeModeManaged {
		return info, errors.New("自定义命令模式由命令自身控制 DSH 版本；切换到 Tiny 托管模式后才能检查和升级")
	}
	target, err := resolveDSHTarget(ctx, s)
	if err != nil {
		return info, err
	}
	info.TargetVersion = target
	info.Available = compareVersions(target, info.CurrentVersion) != 0
	info.LastChecked = time.Now().Format(time.RFC3339)
	if info.Available {
		info.Status = "发现可用版本 " + target
	} else {
		info.Status = "当前已是所选通道版本"
	}
	m.mu.Lock()
	m.updateTarget, m.updateChecked, m.updateStatus = info.TargetVersion, info.LastChecked, info.Status
	m.mu.Unlock()
	return info, nil
}

func resolveDSHTarget(ctx context.Context, s Settings) (string, error) {
	switch s.DSHChannel {
	case DSHChannelRecommended:
		return DSHVersion, nil
	case DSHChannelFixed:
		if !exactVersion.MatchString(s.FixedDSHVersion) {
			return "", errors.New("固定 DSH 版本无效")
		}
	}
	meta, err := fetchPackageMetadata(ctx, s)
	if err != nil {
		return "", err
	}
	if s.DSHChannel == DSHChannelPreview {
		candidates := []string{meta.DistTags["latest"], meta.DistTags["next"]}
		best := ""
		for _, candidate := range candidates {
			if exactVersion.MatchString(candidate) && (best == "" || compareVersions(candidate, best) > 0) {
				best = candidate
			}
		}
		if best == "" {
			return "", errors.New("仓库没有可用的 latest / next 预览版本")
		}
		return best, nil
	}
	stable := make([]string, 0)
	for version := range meta.Versions {
		if exactVersion.MatchString(version) && !strings.Contains(version, "-") {
			stable = append(stable, version)
		}
	}
	if len(stable) == 0 {
		return "", errors.New("仓库目前没有 DSH 正式稳定版；现有运行时保持不变")
	}
	sort.Slice(stable, func(a, b int) bool { return compareVersions(stable[a], stable[b]) > 0 })
	return stable[0], nil
}

func fetchPackageMetadata(ctx context.Context, s Settings) (packageMetadata, error) {
	var meta packageMetadata
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if s.Proxy != "" {
		proxy, _ := url.Parse(s.Proxy)
		transport.Proxy = http.ProxyURL(proxy)
	}
	client := &http.Client{Transport: transport, Timeout: 30 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" || len(via) > 5 {
			return errors.New("拒绝不安全仓库重定向")
		}
		return nil
	}}
	defer client.CloseIdleConnections()
	endpoint := strings.TrimRight(s.Registry, "/") + "/" + url.PathEscape("@deepseek-ai/dsh")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return meta, err
	}
	res, err := client.Do(req)
	if err != nil {
		return meta, fmt.Errorf("检查 DSH 版本失败: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return meta, fmt.Errorf("检查 DSH 版本失败: HTTP %d", res.StatusCode)
	}
	if err = json.NewDecoder(io.LimitReader(res.Body, 8<<20)).Decode(&meta); err != nil {
		return meta, errors.New("仓库返回的 DSH 版本信息无效")
	}
	return meta, nil
}

func compareVersions(a, b string) int {
	parse := func(value string) ([3]int, []string) {
		var numbers [3]int
		value, _, _ = strings.Cut(value, "+")
		base, pre, _ := strings.Cut(value, "-")
		parts := strings.Split(base, ".")
		for index := 0; index < len(parts) && index < 3; index++ {
			numbers[index], _ = strconv.Atoi(parts[index])
		}
		if pre == "" {
			return numbers, nil
		}
		return numbers, strings.Split(pre, ".")
	}
	an, ap := parse(a)
	bn, bp := parse(b)
	for index := range an {
		if an[index] < bn[index] {
			return -1
		}
		if an[index] > bn[index] {
			return 1
		}
	}
	if len(ap) == 0 && len(bp) == 0 {
		return 0
	}
	if len(ap) == 0 {
		return 1
	}
	if len(bp) == 0 {
		return -1
	}
	for index := 0; index < len(ap) && index < len(bp); index++ {
		if ap[index] == bp[index] {
			continue
		}
		ai, ae := strconv.Atoi(ap[index])
		bi, be := strconv.Atoi(bp[index])
		if ae == nil && be == nil {
			if ai < bi {
				return -1
			}
			return 1
		}
		if ae == nil {
			return -1
		}
		if be == nil {
			return 1
		}
		if ap[index] < bp[index] {
			return -1
		}
		return 1
	}
	if len(ap) < len(bp) {
		return -1
	}
	if len(ap) > len(bp) {
		return 1
	}
	return 0
}

func (m *Manager) ApplyDSHUpdate(ctx context.Context) (DSHUpdateInfo, error) {
	if !m.updateMu.TryLock() {
		return m.localUpdateInfo(), errors.New("另一项 DSH 版本操作正在进行")
	}
	defer m.updateMu.Unlock()
	info, err := m.CheckDSHUpdate(ctx)
	if err != nil || !info.Available {
		return info, err
	}
	opCtx, cancel := context.WithCancel(ctx)
	operationDone := make(chan struct{})
	m.mu.Lock()
	s := m.settings
	wasRunning := m.cancel != nil
	m.updateBusy, m.updateStatus = true, "正在准备升级 "+info.TargetVersion
	m.updateCancel, m.updateDone = cancel, operationDone
	m.mu.Unlock()
	defer func() {
		cancel()
		m.mu.Lock()
		m.updateBusy, m.updateCancel, m.updateDone = false, nil, nil
		close(operationDone)
		m.mu.Unlock()
	}()
	m.stopService()
	backup, err := snapshotUpdateState(m.paths)
	if err != nil {
		return info, fmt.Errorf("创建升级回退点失败: %w", err)
	}
	keepBackup := false
	defer func() {
		if !keepBackup {
			_ = os.RemoveAll(backup)
		}
	}()
	i := Installer{Paths: m.paths, Settings: s, Log: &m.log}
	oldVersion, oldDir := i.activeRuntime()
	oldState, stateErr := readManagedRuntimeState(m.paths)
	if stateErr != nil {
		oldState = managedRuntimeState{Current: runtimeSlot{Version: oldVersion, Dir: oldDir}}
	}
	oldReceipt, _ := i.readReceiptAt(i.receiptPath(oldDir))
	stage, err := os.MkdirTemp(m.paths.Runtime, ".dsh-update-")
	if err != nil {
		return info, err
	}
	defer os.RemoveAll(stage)
	stageRuntime := filepath.Join(stage, "runtime")
	targetDir := filepath.Join(m.paths.Runtime, "dsh-versions", safeVersionDir(info.TargetVersion))
	if err = os.MkdirAll(filepath.Dir(targetDir), 0700); err != nil {
		return info, err
	}
	if _, statErr := os.Stat(targetDir); statErr == nil {
		_ = os.RemoveAll(targetDir)
	}
	m.setUpdateStatus("正在安装并验证 DSH " + info.TargetVersion)
	updater := Installer{Paths: m.paths, Settings: s, Log: &m.log, TargetVersion: info.TargetVersion, TargetDir: stageRuntime, PinnedPlugins: oldReceipt.Plugins}
	if _, err = updater.Ensure(opCtx); err != nil {
		_ = restoreUpdateState(m.paths, backup)
		return info, fmt.Errorf("DSH 升级未完成，已保留原版本: %w", err)
	}
	if err = publishRuntimeSlot(stageRuntime, targetDir); err != nil {
		_ = restoreUpdateState(m.paths, backup)
		return info, err
	}
	newState := managedRuntimeState{Current: runtimeSlot{Version: info.TargetVersion, Dir: targetDir}, Previous: &runtimeSlot{Version: oldVersion, Dir: oldDir}, RollbackBackup: backup}
	if err = writeManagedRuntimeState(m.paths, newState); err != nil {
		_ = os.RemoveAll(targetDir)
		_ = restoreUpdateState(m.paths, backup)
		return info, err
	}
	if wasRunning {
		m.setUpdateStatus("正在用新版本启动并进行认证就绪检查")
		if err = m.startService(); err == nil {
			err = m.waitForReady(opCtx, 10*time.Minute)
		}
		if err != nil {
			m.stopService()
			_ = writeManagedRuntimeState(m.paths, oldState)
			restoreErr := restoreUpdateState(m.paths, backup)
			startErr := m.startService()
			if startErr == nil {
				startErr = m.waitForReady(context.Background(), 3*time.Minute)
			}
			_ = os.RemoveAll(targetDir)
			if restoreErr != nil || startErr != nil {
				return info, fmt.Errorf("新版本启动失败；自动回退也需要处理（数据恢复: %v，旧版启动: %v）: %w", restoreErr, startErr, err)
			}
			return m.localUpdateInfo(), fmt.Errorf("新版本启动失败，已自动恢复 %s: %w", oldVersion, err)
		}
	}
	if oldState.Previous != nil && oldState.Previous.Dir != oldDir && oldState.Previous.Dir != targetDir && pathInside(filepath.Join(m.paths.Runtime, "dsh-versions"), oldState.Previous.Dir) {
		_ = os.RemoveAll(oldState.Previous.Dir)
	}
	if oldState.RollbackBackup != "" && oldState.RollbackBackup != backup && validRollbackBackup(m.paths, oldState.RollbackBackup) {
		_ = os.RemoveAll(oldState.RollbackBackup)
	}
	keepBackup = true
	m.setUpdateStatus("DSH 已升级到 " + info.TargetVersion)
	return m.localUpdateInfo(), nil
}

func (m *Manager) RollbackDSH(ctx context.Context) (DSHUpdateInfo, error) {
	if !m.updateMu.TryLock() {
		return m.localUpdateInfo(), errors.New("另一项 DSH 版本操作正在进行")
	}
	defer m.updateMu.Unlock()
	state, err := readManagedRuntimeState(m.paths)
	if err != nil || state.Previous == nil {
		return m.localUpdateInfo(), errors.New("没有可回退的 DSH 版本")
	}
	opCtx, cancel := context.WithCancel(ctx)
	operationDone := make(chan struct{})
	m.mu.Lock()
	wasRunning := m.cancel != nil
	m.updateBusy = true
	m.updateStatus = "正在回退到 " + state.Previous.Version
	m.updateCancel, m.updateDone = cancel, operationDone
	m.mu.Unlock()
	defer func() {
		cancel()
		m.mu.Lock()
		m.updateBusy, m.updateCancel, m.updateDone = false, nil, nil
		close(operationDone)
		m.mu.Unlock()
	}()
	m.stopService()
	backup, err := snapshotUpdateState(m.paths)
	if err != nil {
		return m.localUpdateInfo(), err
	}
	keepBackup := false
	defer func() {
		if !keepBackup {
			_ = os.RemoveAll(backup)
		}
	}()
	if state.RollbackBackup != "" {
		if err = restoreUpdateState(m.paths, state.RollbackBackup); err != nil {
			_ = restoreUpdateState(m.paths, backup)
			return m.localUpdateInfo(), fmt.Errorf("无法恢复上一版本的数据状态: %w", err)
		}
	}
	reversed := managedRuntimeState{Current: *state.Previous, Previous: &state.Current, RollbackBackup: backup}
	if err = writeManagedRuntimeState(m.paths, reversed); err != nil {
		_ = restoreUpdateState(m.paths, backup)
		return m.localUpdateInfo(), err
	}
	if wasRunning {
		if err = m.startService(); err == nil {
			err = m.waitForReady(opCtx, 5*time.Minute)
		}
		if err != nil {
			m.stopService()
			_ = writeManagedRuntimeState(m.paths, state)
			_ = restoreUpdateState(m.paths, backup)
			_ = m.startService()
			return m.localUpdateInfo(), fmt.Errorf("回退版本无法启动，已恢复当前版本: %w", err)
		}
	}
	if state.RollbackBackup != "" && state.RollbackBackup != backup && validRollbackBackup(m.paths, state.RollbackBackup) {
		_ = os.RemoveAll(state.RollbackBackup)
	}
	keepBackup = true
	m.setUpdateStatus("已回退到 " + reversed.Current.Version)
	return m.localUpdateInfo(), nil
}

func (m *Manager) setUpdateStatus(status string) {
	m.mu.Lock()
	m.updateStatus = status
	m.mu.Unlock()
	m.log.Add(status)
}

func (m *Manager) waitForReady(ctx context.Context, maximum time.Duration) error {
	timer := time.NewTimer(maximum)
	defer timer.Stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return errors.New("新版本启动就绪检查超时")
		case <-ticker.C:
			m.mu.Lock()
			phase, last := m.phase, m.lastError
			m.mu.Unlock()
			if phase == "running" {
				return nil
			}
			if phase == "error" {
				return errors.New(last)
			}
		}
	}
}

func safeVersionDir(version string) string {
	return strings.NewReplacer("/", "-", "\\", "-", ":", "-").Replace(version)
}

func publishRuntimeSlot(source, target string) error {
	var last error
	for attempt := 0; attempt < 12; attempt++ {
		if last = os.Rename(source, target); last == nil {
			return nil
		}
		time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
	}
	return fmt.Errorf("发布 DSH 版本槽位失败，可能被安全软件占用: %w", last)
}

func cleanupUpdateArtifacts(paths Paths) {
	state, err := readManagedRuntimeState(paths)
	if err != nil && !os.IsNotExist(err) {
		return // A damaged pointer needs diagnosis; never guess which files are live.
	}
	keep := map[string]bool{}
	if err == nil {
		keep[filepath.Clean(state.Current.Dir)] = true
		if state.Previous != nil {
			keep[filepath.Clean(state.Previous.Dir)] = true
		}
		if state.RollbackBackup != "" {
			keep[filepath.Clean(state.RollbackBackup)] = true
		}
	}
	patterns := []string{filepath.Join(paths.Runtime, ".dsh-update-*"), filepath.Join(paths.Runtime, "dsh-versions", "*")}
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		for _, match := range matches {
			if !keep[filepath.Clean(match)] && pathInside(paths.Runtime, match) {
				_ = os.RemoveAll(match)
			}
		}
	}
}

var updateControlPaths = []string{
	"storages", "profiles/web/package.json", "profiles/web/pnpm-lock.yaml",
	"profiles/web/pnpm-workspace.yaml", ".credentials.yaml", "settings.yaml", "config.yaml",
}

func snapshotUpdateState(paths Paths) (string, error) {
	root, err := os.MkdirTemp(paths.Runtime, ".dsh-update-backup-")
	if err != nil {
		return "", err
	}
	manifest := map[string]bool{}
	for _, rel := range updateControlPaths {
		source := filepath.Join(paths.Data, filepath.FromSlash(rel))
		if _, statErr := os.Lstat(source); os.IsNotExist(statErr) {
			manifest[rel] = false
			continue
		}
		manifest[rel] = true
		if err = copyUpdatePath(source, filepath.Join(root, "data", filepath.FromSlash(rel))); err != nil {
			os.RemoveAll(root)
			return "", err
		}
	}
	contents, _ := json.Marshal(manifest)
	if err = os.WriteFile(filepath.Join(root, "manifest.json"), contents, 0600); err != nil {
		os.RemoveAll(root)
		return "", err
	}
	return root, nil
}

func restoreUpdateState(paths Paths, backup string) error {
	var manifest map[string]bool
	contents, err := os.ReadFile(filepath.Join(backup, "manifest.json"))
	if err != nil || json.Unmarshal(contents, &manifest) != nil {
		return errors.New("升级回退点损坏")
	}
	for _, rel := range updateControlPaths {
		target := filepath.Join(paths.Data, filepath.FromSlash(rel))
		if err = os.RemoveAll(target); err != nil {
			return err
		}
		if manifest[rel] {
			if err = copyUpdatePath(filepath.Join(backup, "data", filepath.FromSlash(rel)), target); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyUpdatePath(source, target string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("回退点不接受符号链接: %s", source)
	}
	if info.IsDir() {
		if err = os.MkdirAll(target, 0700); err != nil {
			return err
		}
		entries, err := os.ReadDir(source)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err = copyUpdatePath(filepath.Join(source, entry.Name()), filepath.Join(target, entry.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("回退点不接受特殊文件: %s", source)
	}
	if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
