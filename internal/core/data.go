package core

import (
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
	Source      string `json:"source"`
	Files       int    `json:"files"`
	Bytes       int64  `json:"bytes"`
	Credentials bool   `json:"credentials"`
}

// The allowlist imports user content/configuration, never executable profiles,
// node_modules, lockfiles or running-process state from the original app.
var importRoots = map[string]bool{"sessions": true, "attachments": true, "skills": true, "settings.yaml": true, ".agent-presets": true, "task-board": true}

func importFiles(source string, credentials bool, visit func(string, fs.FileInfo) error) error {
	root, err := filepath.EvalSymlinks(source)
	if err != nil {
		return err
	}
	if _, err = os.Stat(filepath.Join(root, "settings.yaml")); err != nil {
		return errors.New("请选择包含 settings.yaml 的 DSH 数据目录")
	}
	return filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		top := strings.Split(rel, string(filepath.Separator))[0]
		if !importRoots[top] && !(credentials && top == ".credentials.yaml") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("拒绝导入符号链接: %s", rel)
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return errors.New("拒绝导入特殊文件")
		}
		return visit(rel, info)
	})
}
func PreviewImport(source string, credentials bool) (ImportPreview, error) {
	p := ImportPreview{Source: source, Credentials: credentials}
	err := importFiles(source, credentials, func(_ string, i fs.FileInfo) error {
		p.Files++
		p.Bytes += i.Size()
		if p.Files > 100000 || p.Bytes > 5<<30 {
			return errors.New("导入超过 100000 文件或 5 GiB 限制")
		}
		return nil
	})
	return p, err
}
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
	if _, err = PreviewImport(source, credentials); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(m.paths.Root, ".import-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	// The source is read-only. Each copied file is rechecked after copying; an
	// actively changing source aborts instead of reporting a consistent import.
	err = importFiles(source, credentials, func(rel string, before fs.FileInfo) error {
		src := filepath.Join(source, rel)
		dst := filepath.Join(stage, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
			return err
		}
		in, err := os.Open(src)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, io.LimitReader(in, before.Size()+1))
		closeErr := out.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		after, err := os.Lstat(src)
		if err != nil {
			return err
		}
		if !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
			return errors.New("导入源正在变化；请先退出原 DSH 再导入")
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	backup := filepath.Join(m.paths.Root, "backup-"+time.Now().Format("20060102-150405.000000000"))
	if err = os.Rename(m.paths.Data, backup); err != nil {
		return "", err
	}
	if err = os.Rename(stage, m.paths.Data); err != nil {
		os.Rename(backup, m.paths.Data)
		return "", err
	}
	// Invalidate the installation receipt: the next start recreates the pinned
	// web profile in the imported data, without importing foreign executable code.
	_ = os.Remove(filepath.Join(m.paths.Runtime, "receipt.json"))
	m.log.Add("数据已导入，原数据备份在 " + backup)
	return backup, nil
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
