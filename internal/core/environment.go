package core

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// parseCommandPaths accepts one explicit absolute directory per line. Tiny
// never invokes a shell to expand variables or aliases, which keeps the same
// settings semantics on macOS, Windows and Linux.
func parseCommandPaths(value string) ([]string, error) {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		path := strings.TrimSpace(line)
		if path == "" {
			continue
		}
		if len(result) >= 32 {
			return nil, errors.New("额外命令目录最多填写 32 个")
		}
		if len(path) > 1024 || strings.ContainsRune(path, 0) || strings.ContainsRune(path, os.PathListSeparator) || !filepath.IsAbs(path) {
			return nil, errors.New("额外命令目录必须每行填写一个绝对目录，且不能包含 PATH 分隔符")
		}
		result = append(result, filepath.Clean(path))
	}
	return result, nil
}

func pathListSeparator(goos string) string {
	if goos == "windows" {
		return ";"
	}
	return ":"
}

func platformJoin(goos, base string, elements ...string) string {
	separator := "/"
	if goos == "windows" {
		separator = `\`
		base = strings.ReplaceAll(base, "/", separator)
	}
	result := strings.TrimRight(base, `/\`)
	for _, element := range elements {
		result += separator + strings.Trim(element, `/\`)
	}
	return result
}

func executablePathForPlatform(goos, managedTools, managedNode, commandPaths, home string, getenv func(string) string) string {
	separator := pathListSeparator(goos)
	paths := make([]string, 0, 32)
	seen := map[string]bool{}
	add := func(values ...string) {
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			key := value
			if goos == "windows" {
				key = strings.ToLower(key)
			}
			if !seen[key] {
				seen[key] = true
				paths = append(paths, value)
			}
		}
	}

	// Managed package tools and the private Node runtime always win. Explicit
	// user directories come next, followed by the inherited desktop-session PATH
	// and well-known locations that GUI launchers commonly omit.
	add(managedTools, managedNode)
	for _, line := range strings.Split(strings.ReplaceAll(commandPaths, "\r\n", "\n"), "\n") {
		add(strings.TrimSpace(line))
	}
	add(strings.Split(getenv("PATH"), separator)...)
	switch goos {
	case "darwin":
		add("/usr/local/bin", "/opt/homebrew/bin", "/opt/local/bin")
		if home != "" {
			add(platformJoin(goos, home, ".local", "bin"), platformJoin(goos, home, "bin"), platformJoin(goos, home, ".volta", "bin"), platformJoin(goos, home, ".asdf", "shims"), platformJoin(goos, home, ".local", "share", "mise", "shims"))
		}
		add("/usr/bin", "/bin", "/usr/sbin", "/sbin")
	case "linux":
		add("/usr/local/bin")
		if home != "" {
			add(platformJoin(goos, home, ".local", "bin"), platformJoin(goos, home, "bin"), platformJoin(goos, home, ".volta", "bin"), platformJoin(goos, home, ".asdf", "shims"), platformJoin(goos, home, ".local", "share", "mise", "shims"), platformJoin(goos, home, ".nix-profile", "bin"))
		}
		add("/usr/bin", "/bin", "/snap/bin", "/run/current-system/sw/bin")
	case "windows":
		if appData := getenv("APPDATA"); appData != "" {
			add(platformJoin(goos, appData, "npm"))
		}
		if home != "" {
			add(platformJoin(goos, home, "scoop", "shims"), platformJoin(goos, home, ".local", "bin"))
		}
		if chocolatey := getenv("ChocolateyInstall"); chocolatey != "" {
			add(platformJoin(goos, chocolatey, "bin"))
		}
		if programFiles := getenv("ProgramFiles"); programFiles != "" {
			add(platformJoin(goos, programFiles, "nodejs"), platformJoin(goos, programFiles, "Git", "cmd"))
		}
		if local := getenv("LOCALAPPDATA"); local != "" {
			add(platformJoin(goos, local, "Microsoft", "WindowsApps"))
		}
		if root := getenv("SystemRoot"); root != "" {
			add(platformJoin(goos, root, "System32"), root)
		}
	}
	return strings.Join(paths, separator)
}

func executablePath(r Runtime, commandPaths string) string {
	home, _ := os.UserHomeDir()
	return executablePathForPlatform(runtime.GOOS, r.Bin, filepath.Dir(r.Node), commandPaths, home, os.Getenv)
}

// ensurePrivateNodeLaunchers restores only the two official npm entry-point
// symlinks omitted by Tiny's hardened archive extractor. It never modifies a
// system Node installation or an unrelated executable.
func ensurePrivateNodeLaunchers(root, goos string) error {
	if goos == "windows" {
		return nil
	}
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0700); err != nil {
		return err
	}
	for _, name := range []string{"npm", "npx"} {
		relativeTarget := filepath.Join("..", "lib", "node_modules", "npm", "bin", name+"-cli.js")
		target := filepath.Join(bin, relativeTarget)
		if info, err := os.Stat(target); err != nil || info.IsDir() {
			return errors.New("独立 Node.js 缺少 " + name + " 命令实现")
		}
		launcher := filepath.Join(bin, name)
		info, err := os.Lstat(launcher)
		if err == nil {
			if info.IsDir() {
				return errors.New("独立 Node.js 的 " + name + " 命令入口异常")
			}
			if info.Mode()&os.ModeSymlink == 0 {
				return errors.New("独立 Node.js 的 " + name + " 命令入口不是受管链接")
			}
			current, readErr := os.Readlink(launcher)
			if readErr != nil || current != relativeTarget {
				return errors.New("独立 Node.js 的 " + name + " 命令入口指向异常")
			}
			continue
		}
		if !os.IsNotExist(err) {
			return err
		}
		if err = os.Symlink(relativeTarget, launcher); err != nil {
			return err
		}
	}
	return nil
}
