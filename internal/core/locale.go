package core

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// ResolveLanguage uses the UI language, not geographic region. Unsupported
// languages fall back to English; explicit choices survive a system change.
func ResolveLanguage(choice, system string) string {
	if choice != "system" {
		return choice
	}
	if strings.HasPrefix(strings.ToLower(system), "zh") {
		return "zh"
	}
	return "en"
}
func SystemLanguage() string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var b []byte
	if runtime.GOOS == "darwin" {
		b, _ = exec.CommandContext(ctx, "defaults", "read", "-g", "AppleLanguages").Output()
		parts := strings.FieldsFunc(string(b), func(r rune) bool { return r == '(' || r == ')' || r == '"' || r == ',' || r == '\n' || r == ' ' })
		if len(parts) > 0 {
			return parts[0]
		}
	} else if runtime.GOOS == "windows" {
		cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "[cultureinfo]::CurrentUICulture.Name")
		prepareProcess(cmd)
		b, _ = cmd.Output()
		if len(b) > 0 {
			return strings.TrimSpace(string(b))
		}
	}
	for _, key := range []string{"LANGUAGE", "LC_ALL", "LC_MESSAGES", "LANG"} {
		if s := os.Getenv(key); s != "" {
			return strings.Split(s, ":")[0]
		}
	}
	return "en"
}
