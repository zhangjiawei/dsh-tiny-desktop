//go:build windows

package core

import (
	"golang.org/x/sys/windows"
	"os/exec"
	"path/filepath"
	"strings"
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

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	windows.CloseHandle(h)
	return true
}

func terminateMarkedProcess(pid int, executable, dataDir string) bool {
	if pid <= 0 {
		return false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	var buffer [windows.MAX_PATH]uint16
	size := uint32(len(buffer))
	if err = windows.QueryFullProcessImageName(h, 0, &buffer[0], &size); err != nil {
		return false
	}
	image := filepath.Clean(string(syscall.UTF16ToString(buffer[:size])))
	// QueryFullProcessImageName returns only the executable path, not the
	// command line. The managed Node path is the stable ownership boundary on
	// Windows; checking the app data directory against the image would reject
	// every valid marker because node.exe lives in runtime/, not dsh/.
	_ = dataDir
	if executable == "" || !strings.EqualFold(image, filepath.Clean(executable)) {
		return false
	}
	return windows.TerminateProcess(h, 1) == nil
}
