//go:build !windows

package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestProcessAliveRejectsZombie(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exit 0")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Wait()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(cmd.Process.Pid) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("exited, unreaped owner remained classified as alive")
}

func TestProcessEnvironmentEqualsSupportsSpaces(t *testing.T) {
	home := filepath.Join(t.TempDir(), "Application Support", "dsh")
	cmd := exec.Command("sleep", "30")
	cmd.Env = append(os.Environ(), "DSH_HOME="+home)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()
	if !processEnvironmentEquals(cmd.Process.Pid, "DSH_HOME", home) {
		t.Fatal("exact DSH_HOME with spaces was not detected")
	}
	if processEnvironmentEquals(cmd.Process.Pid, "DSH_HOME", home+"-other") {
		t.Fatal("different DSH_HOME was accepted")
	}
}
