package core

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallerEnvironmentPinsIndependentProfile(t *testing.T) {
	root := t.TempDir()
	paths, err := NewPaths(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DSH_PROFILE_DIR", filepath.Join(t.TempDir(), "wrong-profile"))
	t.Setenv("DSH_RUNTIME_DIR", filepath.Join(t.TempDir(), "wrong-runtime"))

	installer := Installer{Paths: paths, Settings: Defaults()}
	values := map[string]string{}
	for _, item := range installer.environment(Runtime{Node: "node", Bin: "bin"}) {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			values[strings.ToUpper(key)] = value
		}
	}

	if got, want := values["DSH_PROFILE_DIR"], filepath.Join(paths.Data, "profiles", "web"); got != want {
		t.Fatalf("plugin verification can read the wrong profile: got %q, want %q", got, want)
	}
	if got, want := values["DSH_RUNTIME_DIR"], filepath.Join(paths.Runtime, "dsh"); got != want {
		t.Fatalf("plugin verification can read the wrong runtime: got %q, want %q", got, want)
	}
}
