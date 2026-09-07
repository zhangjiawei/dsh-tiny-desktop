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
	"strings"
	"time"
)

const Product = "dsh-tiny-desktop"
const DSHVersion = "0.1.2-rc.1"
const NodeVersion = "24.20.0"
const PnpmVersion = "10.28.0"
const DefaultRegistry = "https://registry.npmmirror.com"
const DefaultCommand = "pnpm --allow-build=@deepseek-ai/dsh-subprocess-local --allow-build=node-pty --allow-build=koffi dlx @deepseek-ai/dsh@" + DSHVersion + " web"
const ManagedCommand = "dsh web"
const RuntimeModeManaged = "managed"
const RuntimeModeCustom = "custom"
const DSHChannelRecommended = "recommended"
const DSHChannelStable = "stable"
const DSHChannelPreview = "preview"
const DSHChannelFixed = "fixed"

type Plugin struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

var Plugins = []Plugin{
	{Name: "@michengai/dsh-codex-ui"},
	{Name: "@michengai/dsh-im-connect"},
	{Name: "@michengai/dsh-automation"},
	{Name: "dshmarket"},
	{Name: "task-complete-notify-for-dsh"},
	{Name: "dsh-better-sidebar"},
}

type Settings struct {
	Port            int    `json:"port"`
	Proxy           string `json:"proxy"`
	LAN             bool   `json:"lan"`
	LANAddress      string `json:"lanAddress"`
	TrustedHosts    string `json:"trustedHosts"`
	PublicURL       string `json:"publicURL"`
	HideOnClose     bool   `json:"hideOnClose"`
	TrayOnly        bool   `json:"trayOnly"`
	AutoStart       bool   `json:"autoStart"`
	AlwaysOnTop     bool   `json:"alwaysOnTop"`
	Language        string `json:"language"`
	RuntimeMode     string `json:"runtimeMode"`
	DSHChannel      string `json:"dshChannel"`
	FixedDSHVersion string `json:"fixedDshVersion"`
	ExtraArgs       string `json:"extraArgs"`
	Command         string `json:"command"`
	Registry        string `json:"registry"`
	StartupMinutes  int    `json:"startupMinutes"`
	Width           int    `json:"width"`
	Height          int    `json:"height"`
}

func Defaults() Settings {
	return Settings{Port: 3080, LAN: true, TrayOnly: true, HideOnClose: true, AutoStart: true, Language: "system", RuntimeMode: RuntimeModeManaged, DSHChannel: DSHChannelRecommended, FixedDSHVersion: DSHVersion, Command: DefaultCommand, Registry: DefaultRegistry, StartupMinutes: 60, Width: 1280, Height: 840}
}
func (s Settings) Validate() error {
	if s.Port < 1024 || s.Port > 65535 {
		return errors.New("端口必须在 1024–65535 之间")
	}
	if s.Width < 760 || s.Height < 540 || s.Width > 10000 || s.Height > 10000 {
		return errors.New("窗口尺寸超出允许范围")
	}
	if s.Language != "system" && s.Language != "zh" && s.Language != "en" {
		return errors.New("不支持的语言")
	}
	u, err := url.Parse(s.Registry)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("包仓库必须为不含凭据的 HTTPS 地址")
	}
	if s.StartupMinutes < 1 || s.StartupMinutes > 120 {
		return errors.New("启动等待时间必须在 1–120 分钟之间")
	}
	if _, err := normalizeLANAddress(s.LANAddress); err != nil {
		return err
	}
	if _, err := parseTrustedAuthorities(s.TrustedHosts); err != nil {
		return err
	}
	if _, err := normalizePublicURL(s.PublicURL); err != nil {
		return err
	}
	if s.RuntimeMode != RuntimeModeManaged && s.RuntimeMode != RuntimeModeCustom {
		return errors.New("请选择有效的 DSH 运行模式")
	}
	if s.RuntimeMode == RuntimeModeManaged {
		switch s.DSHChannel {
		case DSHChannelRecommended, DSHChannelStable, DSHChannelPreview:
		case DSHChannelFixed:
			if !exactVersion.MatchString(s.FixedDSHVersion) {
				return errors.New("固定 DSH 版本必须是完整版本号，例如 0.1.2-rc.1")
			}
		default:
			return errors.New("请选择有效的 DSH 更新通道")
		}
		if _, err := ParseManagedArgs(s.ExtraArgs); err != nil {
			return err
		}
	} else if args, err := ParseCommand(s.Command); err != nil {
		return err
	} else if len(args) == 0 {
		return errors.New("自定义命令模式必须填写完整启动命令")
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
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(b, &fields)
	// Older releases represented the private managed CLI with an empty command.
	// Display it explicitly without silently switching an existing installation
	// to dlx (which may need a first download). New profiles use DefaultCommand.
	if command, exists := fields["command"]; !exists || string(command) == `""` {
		s.Command = ManagedCommand
	}
	// v0.2 and earlier exposed one full command. Migrate only commands that are
	// byte-for-byte equivalent to Tiny's historical managed launch; every truly
	// custom command is preserved and opts into advanced mode.
	if _, exists := fields["runtimeMode"]; !exists {
		if extra, managed := legacyManagedCommand(s.Command); managed {
			s.RuntimeMode = RuntimeModeManaged
			s.DSHChannel = DSHChannelRecommended
			s.FixedDSHVersion = DSHVersion
			s.ExtraArgs = strings.Join(extra, " ")
		} else {
			s.RuntimeMode = RuntimeModeCustom
		}
	}
	if s.DSHChannel == "" {
		s.DSHChannel = DSHChannelRecommended
	}
	if s.FixedDSHVersion == "" {
		s.FixedDSHVersion = DSHVersion
	}
	// v0.3.0 and earlier overloaded one --trusted-host IPv4 as both the
	// requested LAN listener and DSH's Host fence. Migrate only the exact legacy
	// shape; commands with any other custom argument keep their original text.
	if _, exists := fields["lanAddress"]; !exists && s.RuntimeMode == RuntimeModeManaged {
		if address, ok := legacyLANAddress(s.ExtraArgs); ok {
			s.LANAddress = address
			s.ExtraArgs = ""
		}
	}
	// v0.1 had a hardcoded Chinese default but no language chooser. Upgrade that
	// implicit default to system; retain an explicitly configured English value.
	if _, v2 := fields["startupMinutes"]; !v2 && s.Language == "zh" {
		s.Language = "system"
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

func replaceFileWithRetry(replace func() error, sleep func(time.Duration)) error {
	var err error
	for attempt := 0; attempt < 10; attempt++ {
		if err = replace(); err == nil {
			return nil
		}
		delay := time.Duration(attempt+1) * 50 * time.Millisecond
		if delay > 500*time.Millisecond {
			delay = 500 * time.Millisecond
		}
		sleep(delay)
	}
	return err
}
func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}
