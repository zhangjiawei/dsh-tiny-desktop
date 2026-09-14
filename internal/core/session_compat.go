package core

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const sessionSourceSetMarker = "const SOURCE_KINDS = new Set(["

var sessionSourceEntry = regexp.MustCompile(`^\s*"([^"]+)"\s*,?\s*$`)

// ensureSessionMigrationSourceAllowlist repairs a DSH packaging drift: the
// v2-to-v3 format package carries the audited source vocabulary, while an
// older persistence worker may have been published with a shorter list. The
// format package is the authority; unknown worker layouts are rejected.
func ensureSessionMigrationSourceAllowlist(dshDir string) error {
	workerPath := filepath.Join(dshDir, "node_modules", "@deepseek-ai", "dsh-session-persistence-jsonl", "lib", "worker.cjs")
	formatPath := filepath.Join(dshDir, "node_modules", "@deepseek-ai", "dsh-session-format-v2-to-v3", "lib", "index.js")
	worker, err := os.ReadFile(workerPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取 DSH 历史迁移 worker 失败: %w", err)
	}
	format, err := os.ReadFile(formatPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取 DSH 历史迁移规范失败: %w", err)
	}
	if !bytes.Contains(worker, []byte("cannot safely transform unclassified message source")) {
		return nil
	}
	canonical, _, err := sessionSourceKinds(string(format))
	if err != nil {
		return fmt.Errorf("读取 DSH 历史迁移规范白名单失败: %w", err)
	}
	installed, blockEnd, err := sessionSourceKinds(string(worker))
	if err != nil {
		return fmt.Errorf("检测到未知的 DSH 历史迁移 worker，未执行兼容修复: %w", err)
	}
	installedSet := make(map[string]bool, len(installed))
	for _, kind := range installed {
		installedSet[kind] = true
	}
	var missing []string
	for _, kind := range canonical {
		if !installedSet[kind] {
			missing = append(missing, kind)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	start := strings.Index(string(worker), sessionSourceSetMarker)
	if start < 0 || blockEnd <= start {
		return errors.New("DSH 历史迁移 worker 白名单定位失败")
	}
	block := string(worker[start+len(sessionSourceSetMarker) : blockEnd])
	trimmed := strings.TrimRight(block, "\n\r\t ")
	if trimmed == "" {
		return errors.New("DSH 历史迁移 worker 白名单为空")
	}
	if !strings.HasSuffix(strings.TrimSpace(trimmed), ",") {
		trimmed += ","
	}
	for _, kind := range missing {
		trimmed += "\n\t" + fmt.Sprintf("%q", kind) + ","
	}
	updatedBlock := trimmed + "\n"
	patched := append([]byte{}, worker[:start+len(sessionSourceSetMarker)]...)
	patched = append(patched, []byte(updatedBlock)...)
	patched = append(patched, worker[blockEnd:]...)
	if bytes.Equal(worker, patched) {
		return errors.New("DSH 历史迁移 worker 兼容修复没有产生变化")
	}
	info, err := os.Stat(workerPath)
	if err != nil {
		return fmt.Errorf("检查 DSH 历史迁移 worker 失败: %w", err)
	}
	// Reuse the shared atomic replacement path. In particular, Windows needs
	// MoveFileEx(REPLACE_EXISTING) when worker.cjs already exists; a direct
	// os.Rename is not portable for that case.
	if err = AtomicWrite(workerPath, patched, info.Mode().Perm()); err != nil {
		return fmt.Errorf("启用 DSH 历史迁移修复失败: %w", err)
	}
	return nil
}

// sessionSourceKinds extracts the first canonical SOURCE_KINDS declaration.
// It accepts only one quoted string per line, so arbitrary JavaScript is never
// interpreted as part of a compatibility patch.
func sessionSourceKinds(source string) ([]string, int, error) {
	start := strings.Index(source, sessionSourceSetMarker)
	if start < 0 {
		return nil, 0, errors.New("缺少 SOURCE_KINDS 声明")
	}
	rest := source[start+len(sessionSourceSetMarker):]
	endRel := strings.Index(rest, "]);")
	if endRel < 0 {
		return nil, 0, errors.New("SOURCE_KINDS 声明未闭合")
	}
	block := rest[:endRel]
	var kinds []string
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		match := sessionSourceEntry.FindStringSubmatch(line)
		if match == nil {
			return nil, 0, errors.New("SOURCE_KINDS 含有非字符串条目")
		}
		kinds = append(kinds, match[1])
	}
	if len(kinds) == 0 {
		return nil, 0, errors.New("SOURCE_KINDS 为空")
	}
	return kinds, start + len(sessionSourceSetMarker) + endRel, nil
}
