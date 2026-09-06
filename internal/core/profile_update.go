package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"time"
)

var errProfileUpdated = errors.New("DSH Web Profile 已更新")

// watchProfileUpdates reports one completed package-manifest update. DSH writes
// .dsh-pending-updates.json while a plugin operation is in progress, so Tiny
// never stops the process until that marker is gone and the manifest remains
// stable for multiple polls.
func watchProfileUpdates(ctx context.Context, profile string, interval time.Duration, stableTicks int) (<-chan struct{}, error) {
	if interval <= 0 || stableTicks < 1 {
		return nil, errors.New("无效的 Profile 监听参数")
	}
	manifest := filepath.Join(profile, "package.json")
	baseline, err := fileDigest(manifest)
	if err != nil {
		return nil, err
	}
	updates := make(chan struct{}, 1)
	go func() {
		defer close(updates)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		var candidate []byte
		stable := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				current, readErr := fileDigest(manifest)
				if readErr != nil || bytes.Equal(current, baseline) {
					candidate = nil
					stable = 0
					continue
				}
				if _, pendingErr := os.Lstat(filepath.Join(profile, ".dsh-pending-updates.json")); pendingErr == nil || !os.IsNotExist(pendingErr) {
					candidate = nil
					stable = 0
					continue
				}
				if !bytes.Equal(candidate, current) {
					candidate = current
					stable = 1
				} else {
					stable++
				}
				if stable >= stableTicks {
					updates <- struct{}{}
					return
				}
			}
		}
	}()
	return updates, nil
}

func fileDigest(name string) ([]byte, error) {
	contents, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(contents)
	return digest[:], nil
}
