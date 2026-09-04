//go:build !windows

package core

import (
	"os/exec"
	"syscall"
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
