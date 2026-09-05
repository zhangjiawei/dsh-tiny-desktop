//go:build windows

package core

import (
	"context"
	"testing"

	"golang.org/x/sys/windows"
)

func TestBackgroundCommandNeverCreatesVisibleConsole(t *testing.T) {
	cmd := backgroundCommandContext(context.Background(), "cmd.exe", "/c", "exit", "0")
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.HideWindow {
		t.Fatal("background helper command must hide its Windows console")
	}
	if cmd.SysProcAttr.CreationFlags&windows.CREATE_NEW_PROCESS_GROUP == 0 {
		t.Fatal("background helper command must retain the managed process policy")
	}
}
