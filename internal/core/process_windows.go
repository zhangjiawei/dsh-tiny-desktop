//go:build windows

package core

import (
	"golang.org/x/sys/windows"
	"os/exec"
	"syscall"
	"unsafe"
)

// A Job Object is owned by this desktop process. Closing its handle kills only
// our child tree, even if the original Node launcher exits before its children.
type processGroup struct{ job windows.Handle }

func prepareProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
}
func attachProcess(cmd *exec.Cmd) (*processGroup, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits)))
	if err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	p, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	defer windows.CloseHandle(p)
	if err = windows.AssignProcessToJobObject(job, p); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	return &processGroup{job}, nil
}
func (g *processGroup) terminate() { g.kill() }
func (g *processGroup) kill() {
	if g != nil && g.job != 0 {
		_ = windows.TerminateJobObject(g.job, 1)
	}
}
func (g *processGroup) close() {
	if g != nil && g.job != 0 {
		windows.CloseHandle(g.job)
		g.job = 0
	}
}
