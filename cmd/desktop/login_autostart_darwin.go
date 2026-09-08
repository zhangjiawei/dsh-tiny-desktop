//go:build darwin

package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/zhangjiawei/dsh-tiny-desktop/internal/core"
)

const loginAutostartIdentifier = "com.zhangjiawei.dsh-tiny-desktop"

func loginLaunchAgentPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", loginAutostartIdentifier+".plist"), nil
}

func enableLoginLaunch(_ *application.App, hidden bool) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("读取应用路径失败: %w", err)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(executable); resolveErr == nil {
		executable = resolved
	}
	path, err := loginLaunchAgentPath()
	if err != nil {
		return fmt.Errorf("读取登录启动目录失败: %w", err)
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("创建登录启动目录失败: %w", err)
	}
	body, err := loginLaunchAgent(executable, hidden)
	if err != nil {
		return err
	}
	// Do not bootstrap the agent in the current session: registration should
	// take effect at the next login and must never spawn a duplicate app now.
	if err = core.AtomicWrite(path, body, 0644); err != nil {
		return fmt.Errorf("保存登录启动项失败: %w", err)
	}
	return nil
}

func disableLoginLaunch(_ *application.App) error {
	path, err := loginLaunchAgentPath()
	if err != nil {
		return err
	}
	// Removing the file is sufficient for the next login and deliberately does
	// not bootout the current agent, which could terminate the running app.
	if err = os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("移除登录启动项失败: %w", err)
	}
	return nil
}

func loginLaunchAgent(executable string, hidden bool) ([]byte, error) {
	arguments := []string{executable}
	if hidden {
		arguments = append(arguments, "--hidden")
	}
	var body bytes.Buffer
	body.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	body.WriteString("<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n")
	body.WriteString("<plist version=\"1.0\"><dict>\n<key>Label</key><string>")
	if err := xml.EscapeText(&body, []byte(loginAutostartIdentifier)); err != nil {
		return nil, err
	}
	body.WriteString("</string>\n<key>ProgramArguments</key><array>\n")
	for _, argument := range arguments {
		body.WriteString("<string>")
		if err := xml.EscapeText(&body, []byte(argument)); err != nil {
			return nil, err
		}
		body.WriteString("</string>\n")
	}
	body.WriteString("</array>\n<key>RunAtLoad</key><true/>\n<key>KeepAlive</key><false/>\n</dict></plist>\n")
	return body.Bytes(), nil
}
