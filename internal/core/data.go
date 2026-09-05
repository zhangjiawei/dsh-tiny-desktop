package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ImportPreview struct {
	Source        string   `json:"source"`
	Files         int      `json:"files"`
	Bytes         int64    `json:"bytes"`
	Credentials   bool     `json:"credentials"`
	Skipped       int      `json:"skipped"`
	SkippedItems  []string `json:"skippedItems,omitempty"`
	Conflicts     int      `json:"conflicts"`
	ConflictItems []string `json:"conflictItems,omitempty"`
}

// The allowlist imports user content/configuration, never executable profiles,
// node_modules, lockfiles or running-process state from the original app.
var importRoots = map[string]bool{"sessions": true, "attachments": true, "skills": true, "settings.yaml": true, ".agent-presets": true, "task-board": true}

// Keep the preview useful without sending an unbounded list of source paths to
// the WebView. The count still includes every skipped entry.
const importSkipSampleLimit = 10

func importFiles(source string, credentials bool, visit func(string, fs.FileInfo) error, skip func(string, string)) error {
	root, err := filepath.EvalSymlinks(source)
	if err != nil {
		return err
	}
	settingsPath := filepath.Join(root, "settings.yaml")
	settingsInfo, err := os.Lstat(settingsPath)
	if err != nil {
		return errors.New("请选择包含 settings.yaml 的 DSH 数据目录")
	}
	if settingsInfo.IsDir() || settingsInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("settings.yaml 必须是可读取的普通配置文件")
	}
	settingsAllowed, settingsDetail, settingsInspectErr := inspectImportEntry(settingsPath, settingsInfo)
	if settingsInspectErr != nil {
		return fmt.Errorf("无法检查必要配置 settings.yaml: %w", settingsInspectErr)
	}
	if !settingsAllowed {
		return fmt.Errorf("必要配置 settings.yaml 不受支持 (%s)", settingsDetail)
	}
	settingsFile, err := os.Open(settingsPath)
	if err != nil {
		return fmt.Errorf("无法读取必要配置 settings.yaml: %w", err)
	}
	if err = settingsFile.Close(); err != nil {
		return fmt.Errorf("无法读取必要配置 settings.yaml: %w", err)
	}
	return filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if path == root {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		top := strings.Split(rel, string(filepath.Separator))[0]
		if !importRoots[top] && !(credentials && top == ".credentials.yaml") {
			if info != nil && info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if err != nil {
			if rel == "settings.yaml" {
				return fmt.Errorf("无法读取必要配置 settings.yaml: %w", err)
			}
			skip(rel, "无法读取（权限不足或云文件不可用）")
			if info != nil && info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			skip(rel, "符号链接")
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		allowed, detail, inspectErr := inspectImportEntry(path, info)
		if inspectErr != nil {
			if rel == "settings.yaml" {
				return fmt.Errorf("无法检查必要配置 settings.yaml: %w", inspectErr)
			}
			skip(rel, "无法检查文件类型")
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !allowed {
			if rel == "settings.yaml" {
				return fmt.Errorf("必要配置 settings.yaml 不受支持 (%s)", detail)
			}
			skip(rel, detail)
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}
		return visit(rel, info)
	})
}
func PreviewImport(source string, credentials bool) (ImportPreview, error) {
	return previewImport(source, credentials, "")
}

// PreviewImport evaluates conflicts against this Tiny data space. Existing
// Tiny paths always win; the source is only allowed to add missing files.
func (m *Manager) PreviewImport(source string, credentials bool) (ImportPreview, error) {
	return previewImport(source, credentials, m.paths.Data)
}

func previewImport(source string, credentials bool, destination string) (ImportPreview, error) {
	p := ImportPreview{Source: source, Credentials: credentials}
	seenFiles := 0
	var seenBytes int64
	recordSkipped := func(rel, reason string) {
		p.Skipped++
		if len(p.SkippedItems) < importSkipSampleLimit {
			p.SkippedItems = append(p.SkippedItems, filepath.ToSlash(rel)+" — "+reason)
		}
	}
	err := importFiles(source, credentials, func(rel string, i fs.FileInfo) error {
		seenFiles++
		seenBytes += i.Size()
		if seenFiles > 100000 || seenBytes > 5<<30 {
			return errors.New("导入超过 100000 文件或 5 GiB 限制")
		}
		if destination != "" {
			conflict, reason, conflictErr := importDestinationConflict(destination, rel)
			if conflictErr != nil {
				return conflictErr
			}
			if conflict {
				p.Conflicts++
				if len(p.ConflictItems) < importSkipSampleLimit {
					p.ConflictItems = append(p.ConflictItems, filepath.ToSlash(rel)+" — "+reason)
				}
				return nil
			}
		}
		p.Files++
		p.Bytes += i.Size()
		return nil
	}, recordSkipped)
	return p, err
}

// importDestinationConflict checks every existing path component with Lstat.
// This both implements Tiny-wins merge semantics and prevents an existing
// destination symlink from redirecting imported data outside the data space.
func importDestinationConflict(destination, rel string) (bool, string, error) {
	parts := strings.Split(filepath.Clean(rel), string(filepath.Separator))
	current := destination
	for index, part := range parts {
		if part == "" || part == "." || part == ".." {
			return false, "", fmt.Errorf("无效导入路径: %s", rel)
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return false, "", nil
		}
		if err != nil {
			return false, "", fmt.Errorf("检查 Tiny 目标 %s: %w", rel, err)
		}
		if index == len(parts)-1 {
			return true, "Tiny 中已存在，保留 Tiny 版本", nil
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return true, "Tiny 中的上级路径不是安全目录，保留 Tiny 版本", nil
		}
	}
	return false, "", nil
}

type importOverlayBackup struct {
	Version int      `json:"version"`
	Files   []string `json:"files"`
	Dirs    []string `json:"dirs"`
}

const importOverlayMarker = ".tiny-import-overlay.json"

func (m *Manager) Import(source string, credentials bool) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		return "", errors.New("导入前必须停止当前服务")
	}
	source, err := filepath.EvalSymlinks(source)
	if err != nil {
		return "", err
	}
	source, err = filepath.Abs(source)
	if err != nil {
		return "", err
	}
	if source == m.paths.Data || strings.HasPrefix(source, m.paths.Root+string(filepath.Separator)) {
		return "", errors.New("不能从当前应用目录导入")
	}
	if _, err = m.PreviewImport(source, credentials); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(m.paths.Root, ".import-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	// The source is read-only. Each copied file is rechecked after copying; an
	// actively changing source aborts instead of reporting a consistent import.
	skipped := 0
	conflicts := 0
	seenFiles := 0
	var seenBytes int64
	recordSkipped := func(_ string, _ string) { skipped++ }
	err = importFiles(source, credentials, func(rel string, before fs.FileInfo) error {
		seenFiles++
		seenBytes += before.Size()
		if seenFiles > 100000 || seenBytes > 5<<30 {
			return errors.New("导入超过 100000 文件或 5 GiB 限制")
		}
		conflict, _, conflictErr := importDestinationConflict(m.paths.Data, rel)
		if conflictErr != nil {
			return conflictErr
		}
		if conflict {
			conflicts++
			return nil
		}
		src := filepath.Join(source, rel)
		dst := filepath.Join(stage, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
			return err
		}
		in, err := os.Open(src)
		if err != nil {
			if rel == "settings.yaml" {
				return fmt.Errorf("无法读取必要配置 settings.yaml: %w", err)
			}
			// A file can become unavailable between preview and copy (for
			// example, a dehydrated cloud placeholder). Treat it like the
			// unsupported entries found during traversal instead of blocking
			// all other user data.
			recordSkipped(rel, "无法读取（权限不足或云文件不可用）")
			return nil
		}
		defer in.Close()
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, io.LimitReader(in, before.Size()+1))
		closeErr := out.Close()
		if err != nil {
			return fmt.Errorf("复制导入文件 %s: %w", rel, err)
		}
		if closeErr != nil {
			return closeErr
		}
		after, err := os.Lstat(src)
		if err != nil {
			return fmt.Errorf("复查导入文件 %s: %w", rel, err)
		}
		if !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
			return errors.New("导入源正在变化；请先退出原 DSH 再导入")
		}
		return nil
	}, recordSkipped)
	if err != nil {
		return "", err
	}
	backup := filepath.Join(m.paths.Root, "backup-"+time.Now().Format("20060102-150405.000000000"))
	if err = os.Mkdir(backup, 0700); err != nil {
		return "", err
	}
	overlay := importOverlayBackup{Version: 1}
	rollback := func() {
		for index := len(overlay.Files) - 1; index >= 0; index-- {
			_ = os.Remove(filepath.Join(m.paths.Data, filepath.FromSlash(overlay.Files[index])))
		}
		for index := len(overlay.Dirs) - 1; index >= 0; index-- {
			_ = os.Remove(filepath.Join(m.paths.Data, filepath.FromSlash(overlay.Dirs[index])))
		}
	}
	err = filepath.Walk(stage, func(path string, info fs.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == stage || info.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(stage, path)
		if relErr != nil {
			return relErr
		}
		conflict, _, conflictErr := importDestinationConflict(m.paths.Data, rel)
		if conflictErr != nil {
			return conflictErr
		}
		if conflict {
			conflicts++
			return nil
		}
		createdDirs, prepareErr := prepareImportDestination(m.paths.Data, filepath.Dir(rel))
		for _, dir := range createdDirs {
			overlay.Dirs = append(overlay.Dirs, filepath.ToSlash(dir))
		}
		if prepareErr != nil {
			return prepareErr
		}
		destination := filepath.Join(m.paths.Data, rel)
		if renameErr := os.Rename(path, destination); renameErr != nil {
			return renameErr
		}
		overlay.Files = append(overlay.Files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		rollback()
		_ = os.RemoveAll(backup)
		return "", err
	}
	marker, err := json.MarshalIndent(overlay, "", "  ")
	if err == nil {
		err = os.WriteFile(filepath.Join(backup, importOverlayMarker), marker, 0600)
	}
	if err != nil {
		rollback()
		_ = os.RemoveAll(backup)
		return "", err
	}
	m.log.Add(fmt.Sprintf("数据合并完成：新增 %d 个文件，保留 %d 个 Tiny 同名项；撤销记录在 %s", len(overlay.Files), conflicts, backup))
	if skipped > 0 {
		m.log.Add(fmt.Sprintf("导入时已安全跳过 %d 个不支持或无法读取的项目", skipped))
	}
	return backup, nil
}

func prepareImportDestination(root, relDir string) ([]string, error) {
	if relDir == "." || relDir == "" {
		return nil, nil
	}
	var created []string
	current := root
	for _, part := range strings.Split(filepath.Clean(relDir), string(filepath.Separator)) {
		if part == "" || part == "." || part == ".." {
			return created, fmt.Errorf("无效导入目录: %s", relDir)
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			if err = os.Mkdir(current, 0700); err != nil {
				return created, err
			}
			rel, _ := filepath.Rel(root, current)
			created = append(created, rel)
			continue
		}
		if err != nil {
			return created, err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return created, fmt.Errorf("Tiny 目标上级路径不是安全目录: %s", relDir)
		}
	}
	return created, nil
}
func (m *Manager) RestoreBackup(backup string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		return errors.New("恢复前必须停止服务")
	}
	if filepath.Dir(backup) != m.paths.Root || !strings.HasPrefix(filepath.Base(backup), "backup-") {
		return errors.New("只能恢复本应用创建的备份")
	}
	info, err := os.Lstat(backup)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("备份目录无效")
	}
	markerPath := filepath.Join(backup, importOverlayMarker)
	if marker, markerErr := os.ReadFile(markerPath); markerErr == nil {
		var overlay importOverlayBackup
		if json.Unmarshal(marker, &overlay) != nil || overlay.Version != 1 {
			return errors.New("导入撤销记录损坏，未更改当前数据")
		}
		// Validate the complete manifest before removing the first item. A
		// malformed later entry must never turn restoration into a partial
		// path traversal or partial rollback.
		for _, item := range append(append([]string{}, overlay.Files...), overlay.Dirs...) {
			if invalidImportRelativePath(filepath.FromSlash(item)) {
				return errors.New("导入撤销记录包含无效路径，未更改当前数据")
			}
		}
		for index := len(overlay.Files) - 1; index >= 0; index-- {
			rel := filepath.FromSlash(overlay.Files[index])
			if err := os.Remove(filepath.Join(m.paths.Data, rel)); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("撤销导入文件 %s: %w", rel, err)
			}
		}
		for index := len(overlay.Dirs) - 1; index >= 0; index-- {
			rel := filepath.FromSlash(overlay.Dirs[index])
			// A directory may now contain data created by Tiny after import.
			// Remove it only when empty; otherwise deliberately keep it.
			_ = os.Remove(filepath.Join(m.paths.Data, rel))
		}
		return os.RemoveAll(backup)
	} else if !os.IsNotExist(markerErr) {
		return markerErr
	}
	current := filepath.Join(m.paths.Root, "backup-"+time.Now().Format("20060102-150405.000000000"))
	if err = os.Rename(m.paths.Data, current); err != nil {
		return err
	}
	if err = os.Rename(backup, m.paths.Data); err != nil {
		os.Rename(current, m.paths.Data)
		return err
	}
	_ = os.Remove(filepath.Join(m.paths.Runtime, "receipt.json"))
	return nil
}

func invalidImportRelativePath(rel string) bool {
	return !filepath.IsLocal(rel) || filepath.Clean(rel) != rel
}
