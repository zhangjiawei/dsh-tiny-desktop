package core

import (
	"errors"
	"os/exec"
	"path/filepath"
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
	for _, a := range args {
		key, _, _ := strings.Cut(a, "=")
		switch key {
		case "--port", "--host", "--hostname", "--trusted-host", "--token", "--no-auth", "--disable-auth":
			return nil, errors.New("端口、监听地址和认证参数由桌面外壳管理，请勿在启动命令中覆盖")
		}
	}
	return args, nil
}

func (i *Installer) launchCommand(r Runtime) (string, []string, error) {
	args, err := ParseCommand(i.Settings.Command)
	if err != nil {
		return "", nil, err
	}
	if len(args) == 0 {
		return r.Node, []string{r.CLI, "web"}, nil
	}
	name := args[0]
	args = args[1:]
	// Execute package-manager JS directly so pnpm works on Windows without cmd.exe.
	switch strings.ToLower(name) {
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
