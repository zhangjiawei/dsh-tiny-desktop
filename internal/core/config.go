// Package core owns the desktop runtime without depending on Wails. Keeping this
// seam free of UI types lets CI exercise the real installer and child processes.
package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
)

const Product = "dsh-tiny-desktop"
const DSHVersion = "0.1.2-rc.1"
const NodeVersion = "24.20.0"
const PnpmVersion = "10.28.0"

type Plugin struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

var Plugins = []Plugin{
	{"@michengai/dsh-codex-ui", "0.2.103"},
	{"@michengai/dsh-im-connect", "0.1.34"},
	{"@michengai/dsh-automation", "0.1.29"},
	{"dshmarket", "1.41.0"},
	{"task-complete-notify-for-dsh", "0.2.0"},
	{"dsh-better-sidebar", "0.18.0"},
}

type Settings struct {
	Port        int    `json:"port"`
	Proxy       string `json:"proxy"`
	LAN         bool   `json:"lan"`
	HideOnClose bool   `json:"hideOnClose"`
	AutoStart   bool   `json:"autoStart"`
	AlwaysOnTop bool   `json:"alwaysOnTop"`
	Language    string `json:"language"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

func Defaults() Settings {
	return Settings{Port: 3080, HideOnClose: true, AutoStart: true, Language: "zh", Width: 1280, Height: 840}
}
func (s Settings) Validate() error {
	if s.Port < 1024 || s.Port > 65535 {
		return errors.New("端口必须在 1024–65535 之间")
	}
	if s.Width < 760 || s.Height < 540 || s.Width > 10000 || s.Height > 10000 {
		return errors.New("窗口尺寸超出允许范围")
	}
	if s.Language != "zh" && s.Language != "en" {
		return errors.New("不支持的语言")
	}
	if s.Proxy != "" {
		u, err := url.Parse(s.Proxy)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
			return errors.New("代理必须为不含账号密码的 HTTP(S) 地址")
		}
	}
	return nil
}

type Paths struct{ Root, Data, Runtime, Logs string }

func NewPaths(root string) (Paths, error) {
	if root == "" {
		var err error
		root, err = os.UserConfigDir()
		if err != nil {
			return Paths{}, err
		}
		root = filepath.Join(root, Product)
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return Paths{}, err
	}
	p := Paths{Root: absolute, Data: filepath.Join(absolute, "dsh"), Runtime: filepath.Join(absolute, "runtime"), Logs: filepath.Join(absolute, "logs")}
	for _, dir := range []string{p.Root, p.Data, p.Runtime, p.Logs} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return p, err
		}
	}
	return p, nil
}
func (p Paths) LoadSettings() (Settings, error) {
	s := Defaults()
	b, err := os.ReadFile(filepath.Join(p.Root, "settings.json"))
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err = json.Unmarshal(b, &s); err != nil {
		return s, fmt.Errorf("设置文件损坏，未覆盖原文件: %w", err)
	}
	return s, s.Validate()
}
func (p Paths) SaveSettings(s Settings) error {
	if err := s.Validate(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWrite(filepath.Join(p.Root, "settings.json"), b, 0600)
}

// AtomicWrite stages alongside the destination so a crash cannot truncate the
// last good configuration. Windows needs an explicit replacement operation.
func AtomicWrite(path string, data []byte, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".pending-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return replaceFile(name, path)
}
func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}
