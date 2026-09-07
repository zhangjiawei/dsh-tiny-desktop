package core

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"
)

// ParseCommand is argv parsing, never shell evaluation. Quotes are supported;
// pipes, substitutions and redirects are rejected rather than silently run.
func ParseCommand(command string) ([]string, error) {
	if len(command) > 8192 {
		return nil, errors.New("启动命令过长")
	}
	var args []string
	var word strings.Builder
	var quote rune
	started := false
	chars := []rune(command)
	for n := 0; n < len(chars); n++ {
		c := chars[n]
		if strings.ContainsRune("\n\r`$|;&<>", c) {
			return nil, errors.New("启动命令仅支持程序与参数，不支持 Shell 管道、变量或重定向")
		}
		if c == '\\' && n+1 < len(chars) && (chars[n+1] == '"' || chars[n+1] == '\'' || unicode.IsSpace(chars[n+1])) {
			n++
			word.WriteRune(chars[n])
			started = true
			continue
		}
		if quote != 0 {
			if c == quote {
				quote = 0
			} else {
				word.WriteRune(c)
			}
			started = true
			continue
		}
		if c == '"' || c == '\'' {
			quote = c
			started = true
			continue
		}
		if unicode.IsSpace(c) {
			if started {
				args = append(args, word.String())
				word.Reset()
				started = false
			}
			continue
		}
		word.WriteRune(c)
		started = true
	}
	if quote != 0 {
		return nil, errors.New("启动命令引号未闭合")
	}
	if started {
		args = append(args, word.String())
	}
	trustedCount := 0
	for index := 0; index < len(args); index++ {
		a := args[index]
		key, value, inline := strings.Cut(a, "=")
		switch key {
		case "--port", "--host", "--hostname", "--token", "--no-auth", "--disable-auth":
			return nil, errors.New("端口、监听地址和认证参数由桌面外壳管理，请勿在启动命令中覆盖")
		case "--trusted-host":
			if !inline {
				if index+1 >= len(args) {
					return nil, errors.New("--trusted-host 必须填写精确的主机或 host:port")
				}
				index++
				value = args[index]
			}
			if _, err := normalizeTrustedAuthority(value); err != nil {
				return nil, err
			}
			trustedCount++
			if trustedCount > maxTrustedAuthorities {
				return nil, errors.New("--trusted-host 最多允许填写 16 个")
			}
		}
	}
	return args, nil
}

// ParseManagedArgs validates only the optional arguments appended to Tiny's
// managed `dsh web` command. The executable, subcommand, port, listener and
// authentication remain owned by the desktop shell.
func ParseManagedArgs(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	args, err := ParseCommand("dsh web " + value)
	if err != nil {
		return nil, err
	}
	return append([]string(nil), args[2:]...), nil
}

func legacyManagedCommand(command string) ([]string, bool) {
	args, err := ParseCommand(command)
	if err != nil {
		return nil, false
	}
	if len(args) == 0 {
		return nil, true
	}
	for _, base := range []string{ManagedCommand, DefaultCommand} {
		expected, _ := ParseCommand(base)
		stripped := make([]string, 0, len(args))
		extra := make([]string, 0, 2)
		for index := 0; index < len(args); index++ {
			key, _, inline := strings.Cut(args[index], "=")
			if key != "--trusted-host" {
				stripped = append(stripped, args[index])
				continue
			}
			extra = append(extra, args[index])
			if !inline && index+1 < len(args) {
				index++
				extra = append(extra, args[index])
			}
		}
		if len(stripped) != len(expected) {
			continue
		}
		match := true
		for index := range expected {
			match = match && stripped[index] == expected[index]
		}
		if match {
			return extra, true
		}
	}
	return nil, false
}

func managedWindowsDefaultDLX(args []string, osName string) ([]string, bool) {
	if osName != "windows" {
		return nil, false
	}
	expected, err := ParseCommand(DefaultCommand)
	if err != nil {
		return nil, false
	}
	stripped := make([]string, 0, len(args))
	managed := []string{"web"}
	for index := 0; index < len(args); index++ {
		key, _, inline := strings.Cut(args[index], "=")
		if key != "--trusted-host" {
			stripped = append(stripped, args[index])
			continue
		}
		managed = append(managed, args[index])
		if !inline {
			index++
			managed = append(managed, args[index])
		}
	}
	if len(stripped) != len(expected) {
		return nil, false
	}
	for index := range expected {
		if stripped[index] != expected[index] {
			return nil, false
		}
	}
	return managed, true
}

func (i *Installer) launchCommand(r Runtime) (string, []string, error) {
	if i.Settings.RuntimeMode == RuntimeModeManaged {
		extra, err := ParseManagedArgs(i.Settings.ExtraArgs)
		if err != nil {
			return "", nil, err
		}
		return r.Node, append([]string{r.CLI, "web"}, extra...), nil
	}
	args, err := ParseCommand(i.Settings.Command)
	if err != nil {
		return "", nil, err
	}
	if len(args) == 0 {
		return r.Node, []string{r.CLI, "web"}, nil
	}
	// pnpm 10.28.0 has a Windows no-console pipe race when dlx runs DSH's
	// very short postinstall script ("readStream must be readable"). The exact
	// default command is equivalent to the pinned DSH that Ensure already
	// installed and rebuilt in this private runtime, so execute that copy on
	// Windows. Any genuinely custom command still runs exactly as configured.
	if managed, ok := managedWindowsDefaultDLX(args, runtime.GOOS); ok {
		return r.Node, append([]string{r.CLI}, managed...), nil
	}
	name := args[0]
	args = args[1:]
	// Execute package-manager JS directly so pnpm works on Windows without cmd.exe.
	switch strings.ToLower(name) {
	case "dsh", "dsh.cmd":
		// Explicit spelling of the legacy managed launch; never use a global dsh.
		return r.Node, append([]string{r.CLI}, args...), nil
	case "node", "node.exe":
		return r.Node, args, nil
	case "pnpm", "pnpm.cmd":
		return r.Node, append([]string{filepath.Join(i.Paths.Runtime, "tools/node_modules/pnpm/bin/pnpm.cjs"), "--config.cache-dir=" + filepath.Join(i.Paths.Runtime, "pnpm-cache")}, args...), nil
	case "npm", "npm.cmd":
		return r.Node, append([]string{r.NPM}, args...), nil
	case "npx", "npx.cmd":
		return r.Node, append([]string{filepath.Join(filepath.Dir(r.NPM), "npx-cli.js")}, args...), nil
	}
	executable, err := exec.LookPath(name)
	return executable, args, err
}

// backgroundCommandContext is used for short-lived helper probes. In a GUI
// application every Windows child must inherit the no-console process policy,
// including commands that only read a version or locale.
func backgroundCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	prepareProcess(cmd)
	return cmd
}
