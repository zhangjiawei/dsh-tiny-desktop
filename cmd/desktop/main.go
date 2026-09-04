package main

import (
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/dock"
	"github.com/zhangjiawei/dsh-tiny-desktop/frontend"
	"github.com/zhangjiawei/dsh-tiny-desktop/internal/core"
)

var version = "0.2.4"

// QA builds may override this via -ldflags to test in an isolated app instance.
var instanceID = "com.zhangjiawei.dsh-tiny-desktop"

// Set only by the CI link step for an instrumented QA executable. Published
// binaries leave this empty; neither environment variables nor app settings
// can turn on a debugging port in a normal release.
var webviewDebugPort = ""

func main() {
	p, err := core.NewPaths(os.Getenv("DSH_TINY_HOME"))
	if err != nil {
		log.Fatal(err)
	}
	settings, settingsError := p.LoadSettings()
	// A truncated settings file must lead to a visible repairable control page,
	// not an invisible GUI exit. Preserve the bad file until an explicit save.
	if settingsError != nil {
		settings = core.Defaults()
		settings.AutoStart = false
	}
	manager := core.NewManager(p, settings)
	if settingsError != nil {
		manager.ReportError(settingsError)
	}
	var app *application.App
	var control, workspace *application.WebviewWindow
	var iconMu sync.Mutex
	iconCleanup := func() {}
	dockIcon := dock.New() // Used from Go only; never exposed to the DSH window.
	var applyAppearance = func() {}
	restore := func(w *application.WebviewWindow) {
		if runtime.GOOS == "darwin" {
			dockIcon.ShowAppIcon()
		}
		w.UnMinimise()
		w.Show()
		w.Focus()
	}
	showControl := func() { restore(control) }
	loadedURL := ""
	var loadMu sync.Mutex
	assets, _ := fs.Sub(frontend.Assets, "dist")
	showWorkspace := func() {
		loadMu.Lock()
		defer loadMu.Unlock()
		if u, e := manager.LaunchURL(); e == nil {
			// Restoring must not reload an active conversation or replay auth.
			if loadedURL != u {
				workspace.SetURL(u)
				loadedURL = u
			}
			restore(workspace)
		} else {
			showControl()
		}
	}
	app = application.New(application.Options{Name: "DSH Tiny", Description: "An independent desktop home for DeepSeek Harness", Assets: application.AssetOptions{Handler: http.FileServer(http.FS(assets))},
		Windows:        desktopWindowsOptions(),
		SingleInstance: &application.SingleInstanceOptions{UniqueID: instanceID, OnSecondInstanceLaunch: func(application.SecondInstanceData) { showControl() }},
		Mac:            application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: false},
		RawMessageHandler: func(w application.Window, message string, origin *application.OriginInfo) {
			// Window identity alone is insufficient: a trusted window could navigate to
			// a hostile document. Require both the local origin and the top-level frame.
			if origin == nil || !core.TrustedControlMessage(runtime.GOOS, w.Name(), origin.Origin, origin.TopOrigin, origin.IsMainFrame) {
				return
			}
			if len(message) > 16384 {
				return
			}
			var request struct {
				ID     int             `json:"id"`
				Action string          `json:"action"`
				Data   json.RawMessage `json:"data"`
			}
			if json.Unmarshal([]byte(message), &request) != nil {
				return
			}
			go func() {
				var result any
				var e error
				switch request.Action {
				case "status":
					result = manager.Snapshot()
				case "start":
					e = manager.Start()
				case "stop":
					manager.Stop()
				case "quit":
					result = true // Normal app shutdown stops the owned DSH process tree.
				case "open":
					showWorkspace()
				case "browser":
					var u string
					u, e = manager.LaunchURL()
					if e == nil {
						e = app.Browser.OpenURL(u)
					}
				case "copy":
					var u string
					u, e = manager.LaunchURL()
					if e == nil {
						app.Clipboard.SetText(u)
					}
				case "share":
					result, e = manager.ShareURL()
				case "copyShare":
					var u string
					u, e = manager.ShareURL()
					if e == nil {
						app.Clipboard.SetText(u)
					}
				case "configure", "restart", "appearance":
					var s core.Settings
					e = json.Unmarshal(request.Data, &s)
					if e == nil {
						if request.Action == "appearance" {
							e = manager.ConfigureAppearance(s)
						} else {
							// Validate before stopping: a typo must not interrupt work.
							e = s.Validate()
							if e == nil {
								if request.Action == "restart" {
									manager.Stop()
								}
								e = manager.Configure(s)
							}
							if e == nil && request.Action == "restart" {
								e = manager.Start()
							}
						}
						if e == nil {
							applyAppearance()
							result = manager.Snapshot()
						}
					}
				case "preview":
					var d struct {
						Credentials bool `json:"credentials"`
					}
					e = json.Unmarshal(request.Data, &d)
					if e == nil {
						var source string
						source, e = app.Dialog.OpenFile().CanChooseDirectories(true).CanChooseFiles(false).ShowHiddenFiles(true).SetTitle("选择原 DSH 数据目录").PromptForSingleSelection()
						if e == nil && source != "" {
							result, e = core.PreviewImport(source, d.Credentials)
						}
					}
				case "import":
					var d struct {
						Source      string `json:"source"`
						Credentials bool   `json:"credentials"`
					}
					e = json.Unmarshal(request.Data, &d)
					if e == nil {
						result, e = manager.Import(d.Source, d.Credentials)
					}
				case "restore":
					var d struct {
						Backup string `json:"backup"`
					}
					e = json.Unmarshal(request.Data, &d)
					if e == nil {
						e = manager.RestoreBackup(d.Backup)
					}
				case "updates":
					e = app.Browser.OpenURL("https://github.com/zhangjiawei/dsh-tiny-desktop/releases")
				default:
					return
				}
				errorText := ""
				if e != nil {
					errorText = core.Redact(e.Error())
				}
				payload, _ := json.Marshal([]any{request.ID, result, errorText})
				control.ExecJS("window.tinyReply?.(..." + string(payload) + ")")
				if request.Action == "quit" {
					app.Quit()
				}
			}()
		}})
	gear, _ := fs.ReadFile(assets, "settings.png")
	control = app.Window.NewWithOptions(application.WebviewWindowOptions{Name: "control", Title: "DSH Tiny · 设置", Width: 1000, Height: 800, MinWidth: 820, MinHeight: 680, URL: "/", Linux: application.LinuxWindow{Icon: gear}, BackgroundColour: application.NewRGB(245, 246, 245)})
	// Start at a neutral document, not the wails:// control origin. WKWebView
	// otherwise withholds DSH's SameSite=Strict cookie on the first redirect.
	workspace = app.Window.NewWithOptions(application.WebviewWindowOptions{Name: "workspace", Title: "DSH Tiny", Width: settings.Width, Height: settings.Height, MinWidth: 760, MinHeight: 540, Hidden: true, AlwaysOnTop: settings.AlwaysOnTop, URL: "about:blank", KeyBindings: map[string]func(application.Window){"CmdOrCtrl+,": func(application.Window) { showControl() }, "CmdOrCtrl+R": func(w application.Window) { w.Reload() }}})
	hideToTray := func() {
		// Hide every native window, so Windows/Linux remove their taskbar entries.
		// macOS additionally needs an accessory activation policy to remove Dock.
		control.Hide()
		workspace.Hide()
		if runtime.GOOS == "darwin" {
			dockIcon.HideAppIcon()
		}
	}
	control.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if manager.Snapshot().Settings.TrayOnly {
			hideToTray()
		} else {
			control.Hide()
		}
		e.Cancel()
	})
	for _, w := range []*application.WebviewWindow{control, workspace} {
		w.OnWindowEvent(events.Common.WindowMinimise, func(*application.WindowEvent) {
			if manager.Snapshot().Settings.TrayOnly {
				hideToTray()
			}
		})
	}
	workspace.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		s := manager.Snapshot().Settings
		if s.TrayOnly {
			hideToTray()
			e.Cancel()
		} else if s.HideOnClose {
			workspace.Hide()
			e.Cancel()
		} else {
			app.Quit()
		}
	})
	menu := app.NewMenu()
	var translated []struct {
		item   *application.MenuItem
		zh, en string
	}
	addMenu := func(m *application.Menu, zh, en string, action func(*application.Context)) {
		// Native callbacks may arrive on Cocoa's main thread. Wails beta.16's
		// Dock service synchronously dispatches to that thread, so run actions
		// off-thread (as the control bridge already does) to avoid SIGILL.
		item := m.Add(zh).OnClick(func(ctx *application.Context) { go action(ctx) })
		translated = append(translated, struct {
			item   *application.MenuItem
			zh, en string
		}{item, zh, en})
	}
	addMenu(menu, "打开工作空间", "Open workspace", func(*application.Context) { showWorkspace() })
	addMenu(menu, "设置", "Settings", func(*application.Context) { showControl() })
	menu.AddSeparator()
	addMenu(menu, "刷新", "Reload", func(*application.Context) { workspace.Reload() })
	addMenu(menu, "放大", "Zoom in", func(*application.Context) { workspace.ZoomIn() })
	addMenu(menu, "缩小", "Zoom out", func(*application.Context) { workspace.ZoomOut() })
	addMenu(menu, "恢复缩放", "Reset zoom", func(*application.Context) { workspace.ZoomReset() })
	menu.AddSeparator()
	addMenu(menu, "退出 DSH Tiny", "Quit DSH Tiny", func(*application.Context) { app.Quit() })
	tray := app.SystemTray.New()
	icon, _ := fs.ReadFile(assets, "tray.png")
	if runtime.GOOS == "darwin" {
		tray.SetTemplateIcon(icon)
	} else {
		icon, _ = fs.ReadFile(assets, "icon.png")
		tray.SetIcon(icon)
	}
	tray.SetMenu(menu)
	tray.OnClick(func() { go showWorkspace() }).OnDoubleClick(func() { go showWorkspace() }).OnRightClick(tray.ShowMenu)
	appMenu := app.NewMenu()
	appSubmenu := appMenu.AddSubmenu("DSH Tiny")
	addMenu(appSubmenu, "设置", "Settings", func(*application.Context) { showControl() })
	addMenu(appSubmenu, "退出", "Quit", func(*application.Context) { app.Quit() })
	app.Menu.Set(appMenu)
	applyAppearance = func() {
		s := manager.Snapshot()
		lang := core.ResolveLanguage(s.Settings.Language, s.SystemLanguage)
		for _, entry := range translated {
			label := entry.zh
			if lang == "en" {
				label = entry.en
			}
			entry.item.SetLabel(label)
		}
		title := "DSH Tiny · 设置"
		if lang == "en" {
			title = "DSH Tiny · Settings"
		}
		control.SetTitle(title)
		workspace.SetAlwaysOnTop(s.Settings.AlwaysOnTop)
		if !s.Settings.TrayOnly && runtime.GOOS == "darwin" {
			dockIcon.ShowAppIcon()
		}
	}
	app.OnShutdown(func() {
		manager.Stop()
		s := manager.Snapshot().Settings
		b := workspace.Bounds()
		s.Width = b.Width
		s.Height = b.Height
		if s.Validate() == nil && settingsError == nil {
			_ = p.SaveSettings(s)
		}
	})
	go func() {
		// Wait until Cocoa's event loop exists before changing Dock policy.
		last := ""
		appearanceReady := false
		for {
			select {
			case <-app.Context().Done():
				return
			case <-time.After(time.Second):
				if !appearanceReady {
					applyAppearance()
					cleanup, e := installSettingsIcon(control, assets, p.Root)
					if e == nil {
						iconMu.Lock()
						iconCleanup = cleanup
						iconMu.Unlock()
					} else {
						log.Print(core.Redact(e.Error()))
					}
					appearanceReady = true
				}
				s := manager.Snapshot()
				if s.Phase == "running" && last != "running" {
					showWorkspace()
				}
				last = s.Phase
			}
		}
	}()
	if settings.AutoStart {
		manager.Start()
	}
	if err = app.Run(); err != nil {
		log.Fatal(err)
	}
	iconMu.Lock()
	iconCleanup()
	iconMu.Unlock()
}
