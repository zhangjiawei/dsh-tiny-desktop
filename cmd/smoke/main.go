// smoke exercises the production installer and supervisor without a GUI. It
// requires an explicit disposable root so tests never touch a user's DSH data.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/zhangjiawei/dsh-tiny-desktop/internal/core"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	root := flag.String("root", "", "isolated test directory (required)")
	lan := flag.Bool("lan", false, "also verify opt-in LAN authentication")
	workspaceRecovery := flag.Bool("workspace-recovery", false, "seed and verify the v0.2.12 workspace registry recovery")
	command := flag.String("command", core.DefaultCommand, "launch command to exercise (defaults to production pnpm command)")
	flag.Parse()
	if *root == "" {
		fmt.Fprintln(os.Stderr, "--root is required")
		os.Exit(2)
	}
	p, e := core.NewPaths(*root)
	if e != nil {
		panic(e)
	}
	var corruptWorkspace []byte
	if *workspaceRecovery {
		corruptWorkspace, e = seedCorruptWorkspace(p)
		if e != nil {
			panic(e)
		}
	}
	settings := core.Defaults()
	settings.LAN = *lan
	settings.Command = *command
	m := core.NewManager(p, settings)
	m.Start()
	last := 0
	recoveryLogged := false
	deadline := time.After(20 * time.Minute)
	for {
		select {
		case <-deadline:
			m.Stop()
			fmt.Fprintln(os.Stderr, "timeout")
			os.Exit(1)
		case <-time.After(time.Second):
			s := m.Snapshot()
			for _, l := range s.Logs[last:] {
				fmt.Println(l.Time, l.Text)
				if strings.Contains(l.Text, "已恢复 1 个导入工作空间的注册关系") {
					recoveryLogged = true
				}
			}
			last = len(s.Logs)
			if s.Phase == "error" {
				fmt.Fprintln(os.Stderr, s.Error)
				os.Exit(1)
			}
			if s.Phase == "running" {
				fmt.Println("PASS: authenticated DSH startup on", s.Port)
				if *workspaceRecovery {
					backup, readErr := os.ReadFile(filepath.Join(p.Data, "storages", "workspace.json.tiny-v0.2.12-recovery"))
					if readErr != nil || !bytes.Equal(backup, corruptWorkspace) || !recoveryLogged {
						m.Stop()
						fmt.Fprintln(os.Stderr, "workspace registry recovery was not proven")
						os.Exit(1)
					}
					fmt.Println("PASS: v0.2.12 workspace registry repaired before DSH boot")
				}
				ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
				err := m.VerifyInstallation(ctx)
				if err == nil && *lan {
					var u string
					u, err = m.ShareURL()
					if err == nil {
						err = core.VerifyLaunchURL(ctx, u)
					}
				}
				cancel()
				for _, line := range m.Snapshot().Logs[last:] {
					fmt.Println(line.Time, line.Text)
				}
				m.Stop()
				if err != nil {
					fmt.Fprintln(os.Stderr, core.Redact(err.Error()))
					os.Exit(1)
				}
				fmt.Println("PASS: six registered plugins and real native PTY")
				if *lan {
					fmt.Println("PASS: LAN authority-bound authentication")
				}
				fmt.Println("PASS: owned process stopped")
				return
			}
		}
	}
}

func seedCorruptWorkspace(paths core.Paths) ([]byte, error) {
	tinyPath := filepath.Join(paths.Data, "smoke-tiny-workspace")
	importedPath := filepath.Join(paths.Data, "smoke-imported-workspace")
	for _, path := range []string{tinyPath, importedPath, filepath.Join(paths.Data, "storages")} {
		if err := os.MkdirAll(path, 0700); err != nil {
			return nil, err
		}
	}
	document := map[string]any{
		"unit": map[string]any{"name": "workspace", "version": 2},
		"global": map[string]any{
			"initialized":        true,
			"workspaceIds":       []string{"smoke-tiny"},
			"archivedSessionIds": []string{},
		},
		"tables": map[string]any{"workspaces": map[string]any{
			"smoke-tiny": map[string]any{
				"path": tinyPath, "title": "Tiny", "sessionIds": []string{},
				"createdAt": "2026-01-01T00:00:00.000Z", "updatedAt": "2026-01-01T00:00:00.000Z",
			},
			"853f3223-152c-42a4-a25c-9d2ad97f2814": map[string]any{
				"path": importedPath, "title": "Imported", "sessionIds": []string{},
				"createdAt": "2026-01-02T00:00:00.000Z", "updatedAt": "2026-01-02T00:00:00.000Z",
			},
		}},
	}
	contents, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, err
	}
	contents = append(contents, '\n')
	if err = os.WriteFile(filepath.Join(paths.Data, "storages", "workspace.json"), contents, 0600); err != nil {
		return nil, err
	}
	return contents, nil
}
