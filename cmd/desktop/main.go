package main

import (
	"context"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/dock"
	"github.com/zhangjiawei/dsh-tiny-desktop/frontend"
	"github.com/zhangjiawei/dsh-tiny-desktop/internal/core"
)

var version = "0.3.6"

// QA builds may override this via -ldflags to test in an isolated app instance.
var instanceID = "com.zhangjiawei.dsh-tiny-desktop"

// Set only by the CI link step for an instrumented QA executable. Published
// binaries leave this empty; neither environment variables nor app settings
// can turn on a debugging port in a normal release.
var webviewDebugPort = ""

// The DSH page remains the source of truth for navigation. This capture-phase
// listener only hands external HTTP(S) links to Tiny; same-origin DSH routes
// continue to work inside the workspace as before.
const workspaceExternalLinkBridge = `(function(){
  if (window.__dshTinyExternalLinkBridge) return;
  window.__dshTinyExternalLinkBridge = true;
  document.addEventListener("click", function(event) {
    if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    var target = event.target;
    var element = target && target.nodeType === 1 ? target : target && target.parentElement;
    var anchor = element && element.closest ? element.closest("a[href]") : null;
    if (!anchor) return;
    try {
      var link = new URL(anchor.href, window.location.href);
      if ((link.protocol !== "http:" && link.protocol !== "https:") || link.origin === window.location.origin) return;
      event.preventDefault();
      event.stopPropagation();
      window._wails && window._wails.invoke && window._wails.invoke(JSON.stringify({id:0, action:"externalLink", data:{url:link.href}}));
    } catch (_) {}
  }, true);
})();`

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
	var startHidden atomic.Bool
	startHidden.Store(hiddenLoginLaunch(os.Args[1:]))
	manager := core.NewManager(p, settings)
	if settingsError != nil {
		manager.ReportError(settingsError)
	}
	var app *application.App
	var control, workspace *application.WebviewWindow
	// Assigned after application.New so the trusted UI bridge can update the
	// native login registration without exposing it to the DSH workspace.
	var syncLaunchAtLogin func(bool, bool) error
	var iconMu sync.Mutex
	iconCleanup := func() {}
	dockIcon := dock.New() // Used from Go only; never exposed to the DSH window.
	var applyAppearance = func() {}
	restore := func(w *application.WebviewWindow) {
		if w == nil {
			return
		}
		startHidden.Store(false)
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
	appIcon, _ := fs.ReadFile(assets, "icon.png")
	settingsIcon, _ := fs.ReadFile(assets, "settings.png")
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
	macPolicy := application.ActivationPolicyRegular
	if startHidden.Load() {
		macPolicy = application.ActivationPolicyAccessory
	}
	app = application.New(application.Options{Name: "DSH Tiny", Description: "An independent desktop home for DeepSeek Harness", Icon: appIcon, Assets: application.AssetOptions{Handler: http.FileServer(http.FS(assets))},
		Windows:        desktopWindowsOptions(),
		SingleInstance: &application.SingleInstanceOptions{UniqueID: instanceID, OnSecondInstanceLaunch: func(application.SecondInstanceData) { showControl() }},
		Mac:            application.MacOptions{ActivationPolicy: macPolicy, ApplicationShouldTerminateAfterLastWindowClosed: false},
		RawMessageHandler: func(w application.Window, message string, origin *application.OriginInfo) {
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
			if request.Action == "externalLink" {
				// The workspace gets one narrowly scoped capability: open a validated
				// external link in the system browser. It cannot call Tiny controls.
				expected, e := manager.LaunchURL()
				if origin == nil || e != nil || !core.TrustedWorkspaceMessage(runtime.GOOS, origin.Origin, origin.TopOrigin, expected, origin.IsMainFrame) {
					return
				}
				var data struct {
					URL string `json:"url"`
				}
				if json.Unmarshal(request.Data, &data) != nil {
					return
				}
				if u, ok := core.ExternalLinkURL(data.URL); ok {
					go func() { _ = app.Browser.OpenURL(u) }()
				}
				return
			}
			// Window identity alone is insufficient: a trusted window could navigate to
			// a hostile document. Require both the local origin and the top-level frame.
			if origin == nil || !core.TrustedControlMessage(runtime.GOOS, w.Name(), origin.Origin, origin.TopOrigin, origin.IsMainFrame) {
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
				case "restartService":
					// A runtime restart is deliberately separate from settings save:
					// one click stops only this app's owned process tree, starts it
					// again, and lets the existing phase watcher restore the window.
					e = manager.Restart()
					if e == nil {
						result = manager.Snapshot()
					}
				case "checkDSHUpdate":
					ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
					result, e = manager.CheckDSHUpdate(ctx)
					cancel()
				case "applyDSHUpdate":
					ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
					result, e = manager.ApplyDSHUpdate(ctx)
					cancel()
				case "rollbackDSH":
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
					result, e = manager.RollbackDSH(ctx)
					cancel()
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
					result, e = manager.QRShareURL()
				case "copyShare":
					var u string
					u, e = manager.ShareURL()
					if e == nil {
						app.Clipboard.SetText(u)
					}
				case "sharePublic":
					result, e = manager.PublicShareURL()
				case "copyPublic":
					var u string
					u, e = manager.PublicShareURL()
					if e == nil {
						app.Clipboard.SetText(u)
					}
				case "configure", "restart", "appearance":
					var s core.Settings
					e = json.Unmarshal(request.Data, &s)
					if e == nil {
						if request.Action == "appearance" {
							previous := manager.Snapshot().Settings
							loginChanged := previous.LaunchAtLogin != s.LaunchAtLogin || previous.LaunchHidden != s.LaunchHidden
							if loginChanged {
								e = syncLaunchAtLogin(s.LaunchAtLogin, s.LaunchHidden)
							}
							if e == nil {
								e = manager.ConfigureAppearance(s)
							}
							if e != nil && loginChanged {
								// Keep the OS registration and persisted setting aligned when
								// an atomic settings write fails after registration succeeds.
								_ = syncLaunchAtLogin(previous.LaunchAtLogin, previous.LaunchHidden)
							}
						} else {
							// Validate before restart: a typo must not interrupt work.
							e = s.Validate()
							if e == nil {
								e = manager.Configure(s)
							}
							if e == nil && request.Action == "restart" {
								// All user-requested process restarts cross the same
								// supervisor boundary after the control page confirms risk.
								e = manager.Restart()
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
							result, e = manager.PreviewImport(source, d.Credentials)
						}
					}
				case "import":
					var d struct {
						Source      string `json:"source"`
						Credentials bool   `json:"credentials"`
					}
					e = json.Unmarshal(request.Data, &d)
					if e == nil {
						result, e = manager.ImportWithRestart(d.Source, d.Credentials)
					}
				case "restore":
					var d struct {
						Backup string `json:"backup"`
					}
					e = json.Unmarshal(request.Data, &d)
					if e == nil {
						e = manager.RestoreBackupWithRestart(d.Backup)
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
	syncLaunchAtLogin = func(enabled, hidden bool) error {
		if enabled {
			return enableLoginLaunch(app, hidden)
		}
		return disableLoginLaunch(app)
	}
	if err := syncLaunchAtLogin(settings.LaunchAtLogin, settings.LaunchHidden); err != nil {
		// A login-registration failure must not prevent DSH from starting. The
		// error is kept in the process log for diagnosis and retry from Settings.
		log.Printf("登录启动设置失败: %s", core.Redact(err.Error()))
	}
	control = app.Window.NewWithOptions(application.WebviewWindowOptions{Name: "control", Title: "DSH Tiny · 设置", Width: 1000, Height: 800, MinWidth: 820, MinHeight: 680, Hidden: startHidden.Load(), URL: "/", Linux: application.LinuxWindow{Icon: settingsIcon}, BackgroundColour: application.NewRGB(245, 246, 245)})
	// Start at a neutral document, not the wails:// control origin. WKWebView
	// otherwise withholds DSH's SameSite=Strict cookie on the first redirect.
	workspace = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "workspace", Title: "DSH Tiny", Width: settings.Width, Height: settings.Height,
		MinWidth: 760, MinHeight: 540, Hidden: true, AlwaysOnTop: settings.AlwaysOnTop, URL: "about:blank",
		// The workspace has no privileged Tiny bindings. Hide the native context
		// menu while retaining an explicit inspector entry, so diagnostics do not
		// depend on WebView-localised menu text or upstream DSH instrumentation.
		DevToolsEnabled: true, DefaultContextMenuDisabled: true,
		// Wails evaluates JS after each completed navigation on all desktop
		// backends. Keeping the bridge in the window options avoids a race where
		// an event hook fires before a remote DSH document is runtime-ready.
		JS:  workspaceExternalLinkBridge,
		CSS: `html { --default-contextmenu: hide; }`,
		KeyBindings: map[string]func(application.Window){
			"CmdOrCtrl+,":       func(application.Window) { showControl() },
			"CmdOrCtrl+R":       func(w application.Window) { w.Reload() },
			"CmdOrCtrl+Shift+I": func(w application.Window) { w.OpenDevTools() },
		},
	})
	// Reinstall after every DSH navigation so a reload or SPA-level document
	// replacement cannot silently lose the default-browser link behavior.
	workspace.RegisterHook(events.Mac.WebViewDidFinishNavigation, func(*application.WindowEvent) { workspace.ExecJS(workspaceExternalLinkBridge) })
	workspace.RegisterHook(events.Windows.WebViewNavigationCompleted, func(*application.WindowEvent) { workspace.ExecJS(workspaceExternalLinkBridge) })
	workspace.RegisterHook(events.Linux.WindowLoadFinished, func(*application.WindowEvent) { workspace.ExecJS(workspaceExternalLinkBridge) })
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
	// Leave minimise entirely to the native window manager. A minimised window
	// must keep its Dock/taskbar entry; only an explicit close enters tray-only
	// mode. Registering a minimise callback here previously collapsed both
	// actions into the same behaviour.
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
	if runtime.GOOS != "linux" {
		addMenu(menu, "开发者工具", "Developer tools", func(*application.Context) { workspace.OpenDevTools() })
	}
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
	if runtime.GOOS != "linux" {
		addMenu(appSubmenu, "开发者工具", "Developer tools", func(*application.Context) { workspace.OpenDevTools() })
	}
	addMenu(appSubmenu, "退出", "Quit", func(*application.Context) { app.Quit() })
	// Replacing Wails' default application menu must preserve native edit roles;
	// otherwise copy/paste shortcuts fail in both control inputs and web content.
	appMenu.AddRole(application.EditMenu)
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
		if !s.Settings.TrayOnly && runtime.GOOS == "darwin" && !startHidden.Load() {
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
					settingsCleanup, settingsErr := installWindowIcon(control, settingsIcon)
					workspaceCleanup, workspaceErr := installWindowIcon(workspace, appIcon)
					if settingsErr == nil && workspaceErr == nil {
						iconMu.Lock()
						iconCleanup = func() { settingsCleanup(); workspaceCleanup() }
						iconMu.Unlock()
					} else {
						if settingsCleanup != nil {
							settingsCleanup()
						}
						if workspaceCleanup != nil {
							workspaceCleanup()
						}
						if settingsErr != nil {
							log.Print(core.Redact(settingsErr.Error()))
						}
						if workspaceErr != nil {
							log.Print(core.Redact(workspaceErr.Error()))
						}
					}
					appearanceReady = true
				}
				s := manager.Snapshot()
				if s.Phase == "running" && last != "running" && !startHidden.Load() {
					showWorkspace()
				}
				if s.Phase == "running" && last != "running" {
					startHidden.Store(false)
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

func hiddenLoginLaunch(args []string) bool {
	for _, arg := range args {
		if arg == "--hidden" {
			return true
		}
	}
	return false
}
