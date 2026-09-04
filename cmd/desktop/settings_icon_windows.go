package main

import (
	"fmt"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/zhangjiawei/dsh-tiny-desktop/internal/core"
	"golang.org/x/sys/windows"
	"io/fs"
	"path/filepath"
	"unsafe"
)

// Only the settings HWND gets a gear; the workspace and application's resource
// icon retain their identity. Called once after the native event loop is ready.
func installSettingsIcon(w *application.WebviewWindow, assets fs.FS, root string) (func(), error) {
	data, err := fs.ReadFile(assets, "settings.ico")
	if err != nil {
		return nil, err
	}
	path := filepath.Join(root, "settings.ico")
	if err = core.AtomicWrite(path, data, 0600); err != nil {
		return nil, err
	}
	wide, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	user32 := windows.NewLazySystemDLL("user32.dll")
	load := user32.NewProc("LoadImageW")
	send := user32.NewProc("SendMessageW")
	destroy := user32.NewProc("DestroyIcon")
	var icons []uintptr
	cleanup := func() {
		for _, icon := range icons {
			destroy.Call(icon)
		}
	}
	for _, size := range []uintptr{16, 32} {
		icon, _, e := load.Call(0, uintptr(unsafe.Pointer(wide)), 1, size, size, 0x10)
		if icon == 0 {
			cleanup()
			return nil, fmt.Errorf("load settings icon: %w", e)
		}
		icons = append(icons, icon)
	}
	application.InvokeSync(func() {
		hwnd := uintptr(w.NativeWindow())
		send.Call(hwnd, 0x80, 0, icons[0]) // WM_SETICON / ICON_SMALL
		send.Call(hwnd, 0x80, 1, icons[1]) // WM_SETICON / ICON_BIG
	})
	// Release our handles only after app.Run returns and native windows are gone.
	return cleanup, nil
}
