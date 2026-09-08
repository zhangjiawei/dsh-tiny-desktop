package main

import "testing"

func TestHiddenLoginLaunchArgument(t *testing.T) {
	if hiddenLoginLaunch(nil) || hiddenLoginLaunch([]string{"--other"}) {
		t.Fatal("ordinary app launch was treated as hidden login launch")
	}
	if !hiddenLoginLaunch([]string{"--other", "--hidden"}) {
		t.Fatal("hidden login launch was not detected")
	}
}
