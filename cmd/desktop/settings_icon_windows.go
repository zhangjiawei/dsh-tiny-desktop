package main

import (
	"fmt"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"
)

// installWindowIcon decodes the source PNG at the real system small/large icon
// metrics. Loading a one-entry 256 px ICO with LoadImageW allowed Windows to
// retain or badly scale the placeholder resource on some display/DPI setups.
func installWindowIcon(w *application.WebviewWindow, data []byte) (func(), error) {
	small, err := w32.CreateSmallHIconFromImage(data)
	if err != nil {
		return nil, fmt.Errorf("create small window icon: %w", err)
	}
	large, err := w32.CreateLargeHIconFromImage(data)
	if err != nil {
		w32.DestroyIcon(small)
		return nil, fmt.Errorf("create large window icon: %w", err)
	}
	application.InvokeSync(func() {
		hwnd := w32.HWND(uintptr(w.NativeWindow()))
		w32.SendMessage(hwnd, w32.WM_SETICON, w32.ICON_SMALL, uintptr(small))
		w32.SendMessage(hwnd, w32.WM_SETICON, w32.ICON_BIG, uintptr(large))
	})
	// Handles remain owned by this process until all native windows close.
	return func() {
		w32.DestroyIcon(small)
		w32.DestroyIcon(large)
	}, nil
}
