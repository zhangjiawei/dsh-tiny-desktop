//go:build !windows

package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type processGroup struct{ pid int }

func prepareProcess(cmd *exec.Cmd)                       { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }
func attachProcess(cmd *exec.Cmd) (*processGroup, error) { return &processGroup{cmd.Process.Pid}, nil }
func (g *processGroup) terminate() {
	if g != nil {
		_ = syscall.Kill(-g.pid, syscall.SIGTERM)
	}
}
func (g *processGroup) kill() {
	if g != nil {
		_ = syscall.Kill(-g.pid, syscall.SIGKILL)
	}
}
func (g *processGroup) close() {}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}

func terminateMarkedProcess(pid int, executable, dataDir string) bool {
	if pid <= 0 || pid == os.Getpid() {
		return false
	}
	command, err := exec.Command("ps", "-p", fmt.Sprint(pid), "-o", "command=").Output()
	if err != nil {
		return false
	}
	line := strings.TrimSpace(string(command))
	if executable != "" && !strings.Contains(line, filepath.Clean(executable)) {
		return false
	}
	if !strings.Contains(line, filepath.Clean(dataDir)) || !strings.Contains(line, "dsh/lib/bin.js") {
		return false
	}
	if err = syscall.Kill(pid, syscall.SIGTERM); err != nil {
		return false
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
	return true
}
