package core

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed runtime-manifest.json
var runtimeManifest []byte

type runtimeAsset struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

func assetFor(platform string) (runtimeAsset, error) {
	var m struct {
		Assets map[string]runtimeAsset `json:"assets"`
	}
	if err := json.Unmarshal(runtimeManifest, &m); err != nil {
		return runtimeAsset{}, err
	}
	a, ok := m.Assets[platform]
	if !ok {
		return a, fmt.Errorf("不支持的平台: %s", platform)
	}
	return a, nil
}

// Downloads are HTTPS-only and checked against the hash shipped with this app,
// not against a checksum fetched from the same untrusted response as the binary.
func download(ctx context.Context, a runtimeAsset, dest, proxy string) error {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxy != "" {
		u, err := url.Parse(proxy)
		if err != nil {
			return err
		}
		transport.Proxy = http.ProxyURL(u)
	}
	client := &http.Client{Transport: transport, Timeout: 15 * time.Minute, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if r.URL.Scheme != "https" || len(via) > 5 {
			return errors.New("拒绝不安全下载重定向")
		}
		return nil
	}}
	defer client.CloseIdleConnections()
	req, err := http.NewRequestWithContext(ctx, "GET", a.URL, nil)
	if err != nil {
		return err
	}
	if req.URL.Scheme != "https" {
		return errors.New("下载地址必须使用 HTTPS")
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("下载失败 HTTP %d", res.StatusCode)
	}
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(res.Body, 256<<20+1))
	if err != nil {
		return err
	}
	if n > 256<<20 {
		return errors.New("下载超过 256 MiB 限制")
	}
	if hex.EncodeToString(h.Sum(nil)) != a.SHA256 {
		return errors.New("Node.js SHA-256 校验失败")
	}
	return f.Sync()
}

func safeArchivePath(root, name string) (string, error) {
	// Backslashes are rejected on every platform so an archive has one meaning.
	if strings.Contains(name, "\\") || strings.Contains(name, ":") || filepath.IsAbs(name) {
		return "", errors.New("归档路径不安全")
	}
	p := filepath.Join(root, filepath.FromSlash(name))
	rel, err := filepath.Rel(root, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("归档路径越界")
	}
	return p, nil
}
func extractArchive(src, dst string, zipped bool) error {
	var total int64
	write := func(name string, mode os.FileMode, size int64, r io.Reader) error {
		p, err := safeArchivePath(dst, name)
		if err != nil {
			return err
		}
		if mode.IsDir() {
			return os.MkdirAll(p, 0700)
		}
		// Official Node npm symlinks are deliberately omitted; npm-cli.js is invoked
		// directly. No extracted link can redirect a later write outside staging.
		if mode&os.ModeSymlink != 0 {
			return nil
		}
		if !mode.IsRegular() {
			return errors.New("归档包含特殊文件")
		}
		total += size
		if size < 0 || total > 1<<30 {
			return errors.New("解压大小超过限制")
		}
		if err = os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			return err
		}
		f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode.Perm()&0755)
		if err != nil {
			return err
		}
		_, err = io.CopyN(f, r, size)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		return closeErr
	}
	if zipped {
		z, err := zip.OpenReader(src)
		if err != nil {
			return err
		}
		defer z.Close()
		for _, f := range z.File {
			r, err := f.Open()
			if err != nil {
				return err
			}
			err = write(f.Name, f.Mode(), int64(f.UncompressedSize64), r)
			r.Close()
			if err != nil {
				return err
			}
		}
		return nil
	}
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if h.Typeflag == tar.TypeSymlink {
			continue
		}
		if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeDir {
			return errors.New("归档包含不支持的条目")
		}
		if err = write(h.Name, h.FileInfo().Mode(), h.Size, tr); err != nil {
			return err
		}
	}
}
