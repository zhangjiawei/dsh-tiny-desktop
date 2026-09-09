package core

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
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

// SyncDSHLocalePreference mirrors Tiny's language choice into DSH's supported
// Host-backed setting. DSH otherwise derives the initial locale from the
// embedded WebView's browser language, which can be English even on a Chinese
// desktop. Existing settings are decoded and rewritten only when the value
// changes; unrelated setting values remain in the same YAML document.
func SyncDSHLocalePreference(dataDir, choice, system string) error {
	path := filepath.Join(dataDir, "settings.yaml")
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var document map[string]any
	if err = yaml.Unmarshal(contents, &document); err != nil {
		return err
	}
	if document == nil {
		document = map[string]any{}
	}
	want := ResolveLanguage(choice, system)
	locale, _ := document["locale"].(map[string]any)
	if locale == nil {
		locale = map[string]any{}
		document["locale"] = locale
	}
	if got, _ := locale["preference"].(string); got == want {
		return nil
	}
	locale["preference"] = want
	updated, err := yaml.Marshal(document)
	if err != nil {
		return err
	}
	if len(updated) == 0 {
		return errors.New("DSH locale settings produced an empty document")
	}
	return AtomicWrite(path, updated, 0600)
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
		cmd := backgroundCommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "[cultureinfo]::CurrentUICulture.Name")
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
