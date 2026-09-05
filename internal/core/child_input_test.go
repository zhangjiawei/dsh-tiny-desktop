package core

import (
	"io"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestManagedStdinStaysOpenUntilCleanup(t *testing.T) {
	if os.Getenv("DSH_TINY_STDIN_HELPER") == "1" {
		_, err := os.Stdin.Read(make([]byte, 1))
		if err != io.EOF {
			os.Exit(2)
		}
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestManagedStdinStaysOpenUntilCleanup")
	cmd.Env = append(os.Environ(), "DSH_TINY_STDIN_HELPER=1")
	started, closeInput, err := attachManagedStdin(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer closeInput()
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	started()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		t.Fatalf("managed stdin reached EOF before cleanup: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	closeInput()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("managed stdin did not close")
	}
}
