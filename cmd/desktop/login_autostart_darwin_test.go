//go:build darwin

package main

import (
	"bytes"
	"testing"
)

func TestLoginLaunchAgentVisibility(t *testing.T) {
	visible, err := loginLaunchAgent("/Applications/DSH Tiny.app/Contents/MacOS/dsh-tiny", false)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(visible, []byte("--hidden")) {
		t.Fatal("visible login item contains hidden argument")
	}
	hidden, err := loginLaunchAgent("/Applications/DSH Tiny.app/Contents/MacOS/dsh-tiny", true)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(hidden, []byte("<string>--hidden</string>")) {
		t.Fatal("hidden login item does not contain hidden argument")
	}
}
