package main

import (
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/zhangjiawei/dsh-tiny-desktop/frontend"
	"github.com/zhangjiawei/dsh-tiny-desktop/internal/core"
)

var version = "0.1.0"

func main() {
	p, err := core.NewPaths(os.Getenv("DSH_TINY_HOME"))
	if err != nil {
		log.Fatal(err)
	}
	settings, err := p.LoadSettings()
	if err != nil {
		log.Fatal(err)
	}
	manager := core.NewManager(p, settings)
	var app *application.App
	var control, workspace *application.WebviewWindow
	assets, _ := fs.Sub(frontend.Assets, "dist")
	showWorkspace := func() {
		if u, e := manager.LaunchURL(); e == nil {
			workspace.SetURL(u)
			workspace.Show()
		} else {
			control.Show()
		}
	}
	app = application.New(application.Options{Name: "DSH Tiny", Description: "An independent desktop home for DeepSeek Harness", Assets: application.AssetOptions{Handler: http.FileServer(http.FS(assets))},
		SingleInstance: &application.SingleInstanceOptions{UniqueID: "com.zhangjiawei.dsh-tiny-desktop", OnSecondInstanceLaunch: func(application.SecondInstanceData) { control.Show() }},
		Mac:            application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: false},
		RawMessageHandler: func(w application.Window, message string, origin *application.OriginInfo) {
			// Window identity alone is insufficient: a trusted window could navigate to
			// a hostile document. Require both the local origin and the top-level frame.
			if w.Name() != "control" || origin == nil || !origin.IsMainFrame {
				return
			}
			o := strings.TrimSuffix(origin.Origin, "/")
			if o != "wails://localhost" && o != "http://wails.localhost" {
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
				case "configure":
					var s core.Settings
					e = json.Unmarshal(request.Data, &s)
					if e == nil {
						e = manager.Configure(s)
						if e == nil {
							workspace.SetAlwaysOnTop(s.AlwaysOnTop)
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
			}()
		}})
	control = app.Window.NewWithOptions(application.WebviewWindowOptions{Name: "control", Title: "DSH Tiny · 控制中心", Width: 1000, Height: 800, MinWidth: 820, MinHeight: 680, URL: "/", BackgroundColour: application.NewRGB(243, 245, 239)})
	// Start at a neutral document, not the wails:// control origin. WKWebView
	// otherwise withholds DSH's SameSite=Strict cookie on the first redirect.
	workspace = app.Window.NewWithOptions(application.WebviewWindowOptions{Name: "workspace", Title: "DSH Tiny", Width: settings.Width, Height: settings.Height, MinWidth: 760, MinHeight: 540, Hidden: true, AlwaysOnTop: settings.AlwaysOnTop, URL: "about:blank", KeyBindings: map[string]func(application.Window){"CmdOrCtrl+,": func(application.Window) { control.Show() }, "CmdOrCtrl+R": func(w application.Window) { w.Reload() }}})
	control.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) { control.Hide(); e.Cancel() })
	workspace.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		s := manager.Snapshot().Settings
		if s.HideOnClose {
			workspace.Hide()
			e.Cancel()
		} else {
			app.Quit()
		}
	})
	menu := app.NewMenu()
	menu.Add("打开工作空间").OnClick(func(*application.Context) { showWorkspace() })
	menu.Add("控制中心 / 设置").OnClick(func(*application.Context) { control.Show() })
	menu.AddSeparator()
	menu.Add("刷新").OnClick(func(*application.Context) { workspace.Reload() })
	menu.Add("放大").OnClick(func(*application.Context) { workspace.ZoomIn() })
	menu.Add("缩小").OnClick(func(*application.Context) { workspace.ZoomOut() })
	menu.Add("恢复缩放").OnClick(func(*application.Context) { workspace.ZoomReset() })
	menu.AddSeparator()
	menu.Add("退出 DSH Tiny").OnClick(func(*application.Context) { app.Quit() })
	tray := app.SystemTray.New()
	icon, _ := fs.ReadFile(assets, "tray.png")
	if runtime.GOOS == "darwin" {
		tray.SetTemplateIcon(icon)
	} else {
		icon, _ = fs.ReadFile(assets, "icon.png")
		tray.SetIcon(icon)
	}
	tray.SetMenu(menu)
	appMenu := app.NewMenu()
	appMenu.AddSubmenu("DSH Tiny").Add("控制中心").OnClick(func(*application.Context) { control.Show() })
	appMenu.AddSubmenu("文件").Add("退出").OnClick(func(*application.Context) { app.Quit() })
	app.Menu.Set(appMenu)
	app.OnShutdown(func() {
		manager.Stop()
		s := manager.Snapshot().Settings
		b := workspace.Bounds()
		s.Width = b.Width
		s.Height = b.Height
		if s.Validate() == nil {
			_ = p.SaveSettings(s)
		}
	})
	go func() {
		last := ""
		for {
			select {
			case <-app.Context().Done():
				return
			case <-time.After(time.Second):
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
}
