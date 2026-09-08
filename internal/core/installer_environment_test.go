package core

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestExecutablePathKeepsPrivateToolsFirstAndAddsCrossPlatformHostTools(t *testing.T) {
	tests := []struct {
		name, goos, home string
		env              map[string]string
		want             []string
	}{
		{
			name: "macOS GUI launch",
			goos: "darwin",
			home: "/Users/tester",
			env:  map[string]string{"PATH": "/usr/bin:/bin"},
			want: []string{"/managed/tools", "/managed/node/bin", "/custom/bin", "/usr/local/bin", "/opt/homebrew/bin", "/Users/tester/.local/bin"},
		},
		{
			name: "Windows GUI launch",
			goos: "windows",
			home: `C:\Users\tester`,
			env: map[string]string{
				"PATH":         `C:\Windows\System32`,
				"APPDATA":      `C:\Users\tester\AppData\Roaming`,
				"LOCALAPPDATA": `C:\Users\tester\AppData\Local`,
				"ProgramFiles": `C:\Program Files`,
				"SystemRoot":   `C:\Windows`,
			},
			want: []string{`C:\managed\tools`, `C:\managed\node`, `C:\custom\bin`, `C:\Users\tester\AppData\Roaming\npm`, `C:\Users\tester\scoop\shims`, `C:\Program Files\nodejs`},
		},
		{
			name: "Linux desktop launch",
			goos: "linux",
			home: "/home/tester",
			env:  map[string]string{"PATH": "/usr/bin:/bin"},
			want: []string{"/managed/tools", "/managed/node/bin", "/custom/bin", "/usr/local/bin", "/home/tester/.local/bin", "/home/tester/.nix-profile/bin"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			getenv := func(key string) string { return test.env[key] }
			managedTools, managedNode, custom := "/managed/tools", "/managed/node/bin", "/custom/bin"
			if test.goos == "windows" {
				managedTools, managedNode, custom = `C:\managed\tools`, `C:\managed\node`, `C:\custom\bin`
			}
			got := executablePathForPlatform(test.goos, managedTools, managedNode, custom, test.home, getenv)
			for _, path := range test.want {
				if !pathListContains(test.goos, got, path) {
					t.Errorf("PATH %q does not contain %q", got, path)
				}
			}
			if first := strings.Split(got, pathListSeparator(test.goos))[0]; first != managedTools {
				t.Errorf("private tools lost precedence: got first %q", first)
			}
		})
	}
}

func TestPrivateUnixNodeRestoresNPMAndNPXLaunchers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix launchers use symbolic links")
	}
	root := t.TempDir()
	for _, name := range []string{"npm-cli.js", "npx-cli.js"} {
		path := filepath.Join(root, "lib", "node_modules", "npm", "bin", name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("#!/usr/bin/env node\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := ensurePrivateNodeLaunchers(root, "linux"); err != nil {
		t.Fatal(err)
	}
	if err := ensurePrivateNodeLaunchers(root, "linux"); err != nil {
		t.Fatalf("launcher repair is not idempotent: %v", err)
	}
	for _, name := range []string{"npm", "npx"} {
		path := filepath.Join(root, "bin", name)
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("%s launcher was not restored: %v", name, err)
		}
	}
}

func TestPrivateUnixNodeRejectsUnexpectedLauncher(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix launchers use symbolic links")
	}
	root := t.TempDir()
	for _, name := range []string{"npm-cli.js", "npx-cli.js"} {
		path := filepath.Join(root, "lib", "node_modules", "npm", "bin", name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("#!/usr/bin/env node\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "npm"), []byte("unexpected"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := ensurePrivateNodeLaunchers(root, "darwin"); err == nil {
		t.Fatal("unexpected private launcher was trusted")
	}
}

func TestAdditionalCommandPathsRequireAbsoluteDirectories(t *testing.T) {
	s := Defaults()
	s.CommandPaths = "relative/bin"
	if err := s.Validate(); err == nil {
		t.Fatal("relative command path was accepted")
	}
	s.CommandPaths = filepath.Join(t.TempDir(), "bin")
	if err := s.Validate(); err != nil {
		t.Fatalf("absolute command path was rejected: %v", err)
	}
}

func pathListContains(goos, list, want string) bool {
	comparison := func(value string) string { return value }
	if goos == "windows" {
		comparison = strings.ToLower
	}
	for _, item := range strings.Split(list, pathListSeparator(goos)) {
		if comparison(item) == comparison(want) {
			return true
		}
	}
	return false
}
