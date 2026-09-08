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

// profileFingerprint covers only dependency ownership files. Runtime caches and
// Cordis patch files are intentionally excluded: they are either transient or
// handled by Harness hot composition and must not imply a process restart.
func profileFingerprint(profile string) ([]byte, error) {
	digest := sha256.New()
	for index, name := range []string{"package.json", "pnpm-lock.yaml"} {
		contents, err := os.ReadFile(filepath.Join(profile, name))
		if err != nil {
			// A lockfile is optional for custom profiles; package.json is the
			// required ownership boundary that proves this is a usable profile.
			if index != 0 && os.IsNotExist(err) {
				contents = nil
			} else {
				return nil, err
			}
		}
		digest.Write([]byte(name))
		digest.Write([]byte{0})
		if contents == nil {
			digest.Write([]byte{0})
		} else {
			digest.Write([]byte{1})
			digest.Write(contents)
		}
		digest.Write([]byte{0})
	}
	return digest.Sum(nil), nil
}

// watchProfileChanges reports stable dependency changes for the lifetime of
// the DSH child. It deliberately knows nothing about the writer or its batch
// protocol. A change updates UI activation state only; the supervisor never
// turns this notification into an implicit process restart.
func watchProfileChanges(ctx context.Context, profile string, baseline []byte, interval time.Duration, stableTicks int) (<-chan struct{}, error) {
	if interval <= 0 || stableTicks < 1 {
		return nil, errors.New("无效的 Profile 监听参数")
	}
	if baseline == nil {
		var err error
		baseline, err = profileFingerprint(profile)
		if err != nil {
			return nil, err
		}
	}
	changes := make(chan struct{}, 1)
	go func() {
		defer close(changes)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		var candidate []byte
		stable := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				current, readErr := profileFingerprint(profile)
				if readErr != nil || bytes.Equal(current, baseline) {
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
					select {
					case changes <- struct{}{}:
						baseline = current
						candidate = nil
						stable = 0
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()
	return changes, nil
}
