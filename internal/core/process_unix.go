//go:build !windows

package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	// kill(pid, 0) also succeeds for an unreaped zombie. Treating that owner as
	// active leaves its already reparented DSH child behind during a fast relaunch.
	status, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "stat=").Output()
	if err != nil {
		return true // Fail closed: never clean up when ownership is uncertain.
	}
	return !strings.HasPrefix(strings.TrimSpace(string(status)), "Z")
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
	if !strings.Contains(line, "dsh/lib/bin.js") || !processEnvironmentEquals(pid, "DSH_HOME", filepath.Clean(dataDir)) {
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

// processEnvironmentEquals reads the process environment without logging it.
// ps renders variables as space-delimited KEY=VALUE entries; matching the full
// value plus boundaries still supports paths containing spaces.
func processEnvironmentEquals(pid int, key, value string) bool {
	if pid <= 0 || key == "" || value == "" {
		return false
	}
	environ, err := exec.Command("ps", "eww", "-p", strconv.Itoa(pid), "-o", "command=").Output()
	if err != nil {
		return false
	}
	haystack := " " + strings.TrimSpace(string(environ)) + " "
	return strings.Contains(haystack, " "+key+"="+value+" ")
}

// cleanupManagedOrphans removes only DSH processes whose command line proves
// that they belong to this Tiny data directory. Processes descended from a
// live Tiny owner are retained, which makes a second-instance launch harmless.
func cleanupManagedOrphans(paths Paths, owners map[int]bool) []int {
	out, err := exec.Command("ps", "-axo", "pid=,ppid=,command=").Output()
	if err != nil {
		return nil
	}
	parents := map[int]int{}
	var candidates []int
	for _, raw := range strings.Split(string(out), "\n") {
		fields := strings.Fields(raw)
		if len(fields) < 2 {
			continue
		}
		pid, e1 := strconv.Atoi(fields[0])
		ppid, e2 := strconv.Atoi(fields[1])
		if e1 != nil || e2 != nil || pid <= 0 {
			continue
		}
		parents[pid] = ppid
		if !owners[pid] && strings.Contains(raw, filepath.Clean(paths.Runtime)) && strings.Contains(raw, "dsh/lib/bin.js") {
			candidates = append(candidates, pid)
		}
	}
	ownedByLive := func(pid int) bool {
		seen := map[int]bool{}
		for pid > 1 && !seen[pid] {
			if owners[pid] {
				return true
			}
			seen[pid] = true
			parent, ok := parents[pid]
			if !ok {
				return false
			}
			pid = parent
		}
		return false
	}
	var removed []int
	for _, candidate := range candidates {
		if ownedByLive(candidate) || !processEnvironmentEquals(candidate, "DSH_HOME", filepath.Clean(paths.Data)) {
			continue
		}
		if terminateMarkedProcess(candidate, "", paths.Data) {
			removed = append(removed, candidate)
		}
	}
	return removed
}
