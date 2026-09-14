package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/zhangjiawei/dsh-tiny-desktop/internal/core"
)

const maxWorkspaceDownloadBytes int64 = 4 << 30

var errDownloadCancelled = errors.New("已取消下载")

// downloadWorkspaceFile authenticates a short-lived HTTP client against the
// live workspace URL, then streams one same-origin GET into a user-selected
// destination. The token stays in memory and is never included in errors.
func downloadWorkspaceFile(ctx context.Context, manager *core.Manager, app *application.App, workspace *application.WebviewWindow, rawURL, hint string) (string, error) {
	launch, err := manager.LaunchURL()
	if err != nil {
		return "", err
	}
	target, ok := core.WorkspaceDownloadURL(rawURL, launch)
	if !ok {
		return "", errors.New("下载地址不是当前 DSH 工作区的同源地址")
	}
	filename := core.DownloadFilename(hint, target)
	directory := application.Path(application.PathDownload)
	if directory == "" {
		directory = filepath.Dir(filepath.Join(os.TempDir(), filename))
	}
	destination, err := app.Dialog.SaveFile().
		SetMessage("选择下载文件的保存位置").
		SetDirectory(directory).
		SetFilename(filename).
		AttachToWindow(workspace).
		PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(destination) == "" {
		return "", errDownloadCancelled
	}
	if err := streamAuthenticatedDownload(ctx, launch, target, destination); err != nil {
		return "", err
	}
	return destination, nil
}

func streamAuthenticatedDownload(ctx context.Context, launch, target, destination string) error {
	launchURL, err := url.Parse(launch)
	if err != nil {
		return errors.New("DSH 启动地址无效")
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return errors.New("无法初始化下载认证会话")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	client := &http.Client{Jar: jar, Timeout: 20 * time.Minute, Transport: transport, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 3 || req.URL.Scheme != launchURL.Scheme || !strings.EqualFold(req.URL.Host, launchURL.Host) {
			return errors.New("下载重定向超出当前 DSH 工作区")
		}
		return nil
	}}
	defer client.CloseIdleConnections()
	rootReq, err := http.NewRequestWithContext(ctx, http.MethodGet, launch, nil)
	if err != nil {
		return errors.New("无法创建 DSH 认证请求")
	}
	rootResp, err := client.Do(rootReq)
	if err != nil {
		return fmt.Errorf("DSH 认证失败: %w", redactDownloadError(err))
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(rootResp.Body, 2*1024*1024))
	rootResp.Body.Close()
	if rootResp.StatusCode != http.StatusOK || len(jar.Cookies(launchURL)) == 0 {
		return fmt.Errorf("DSH 认证失败: HTTP %d", rootResp.StatusCode)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return errors.New("无法创建下载请求")
	}
	req.Header.Set("Referer", launchURL.Scheme+"://"+launchURL.Host+"/")
	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("下载请求失败: %w", redactDownloadError(err))
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("下载请求失败: HTTP %d", response.StatusCode)
	}
	if response.ContentLength > maxWorkspaceDownloadBytes {
		return errors.New("下载文件超过 4 GiB 限制")
	}
	dir := filepath.Dir(destination)
	tmp, err := os.CreateTemp(dir, ".dsh-tiny-download-*")
	if err != nil {
		return fmt.Errorf("无法创建临时下载文件: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	defer tmp.Close()
	written, err := io.Copy(tmp, io.LimitReader(response.Body, maxWorkspaceDownloadBytes+1))
	if err != nil {
		return fmt.Errorf("写入下载文件失败: %w", err)
	}
	if written > maxWorkspaceDownloadBytes {
		return errors.New("下载文件超过 4 GiB 限制")
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭下载文件失败: %w", err)
	}
	// Replace only after the complete response is on disk, so a failed or
	// cancelled transfer never destroys an existing user file.
	if err := os.Remove(destination); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("无法替换目标文件: %w", err)
	}
	if err := os.Rename(tmpPath, destination); err != nil {
		return fmt.Errorf("无法完成下载: %w", err)
	}
	return nil
}

func redactDownloadError(err error) error {
	if err == nil {
		return nil
	}
	return errors.New(core.Redact(err.Error()))
}
