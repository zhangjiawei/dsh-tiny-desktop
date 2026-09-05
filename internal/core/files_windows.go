//go:build windows

package core

import (
	"time"

	"golang.org/x/sys/windows"
)

func replaceFile(from, to string) error {
	a, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return err
	}
	b, err := windows.UTF16PtrFromString(to)
	if err != nil {
		return err
	}
	// Defender and indexers can briefly open a newly synced settings file
	// without delete sharing. Keep the old file intact and retry only the final
	// atomic replacement; never truncate the destination in place.
	return replaceFileWithRetry(func() error {
		return windows.MoveFileEx(a, b, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
	}, time.Sleep)
}
