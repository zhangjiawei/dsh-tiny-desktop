package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// TestWindowLifecycleKeepsMinimiseNative locks the desktop convention: the
// minimise button leaves a normal Dock/taskbar entry, while close-to-tray may
// still hide all windows. This inspects the real event wiring in main.go so a
// later handler cannot silently route minimise back through hideToTray.
func TestWindowLifecycleKeepsMinimiseNative(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	minimiseHooks := 0
	closeHooksWithTrayHide := 0
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		event, ok := call.Args[0].(*ast.SelectorExpr)
		if !ok {
			return true
		}
		handler, ok := call.Args[1].(*ast.FuncLit)
		if !ok {
			return true
		}
		switch {
		case selector.Sel.Name == "OnWindowEvent" && event.Sel.Name == "WindowMinimise":
			minimiseHooks++
			if callsIdentifier(handler.Body, "hideToTray") {
				t.Error("minimise is wired to hideToTray; it must remain in the Dock/taskbar")
			}
		case selector.Sel.Name == "RegisterHook" && event.Sel.Name == "WindowClosing":
			if callsIdentifier(handler.Body, "hideToTray") {
				closeHooksWithTrayHide++
			}
		}
		return true
	})

	if minimiseHooks == 0 {
		t.Log("no minimise override: native Dock/taskbar behaviour is preserved")
	}
	if closeHooksWithTrayHide != 2 {
		t.Fatalf("close-to-tray wiring changed: got %d hooks, want 2", closeHooksWithTrayHide)
	}
}

func TestWorkspaceKeepsEmbeddedDiagnosticsWithoutPrivilegingControlPage(t *testing.T) {
	contents, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(contents)
	if !strings.Contains(source, `Name: "workspace"`) || !strings.Contains(source, `DevToolsEnabled: true`) {
		t.Fatal("embedded workspace diagnostics are not enabled")
	}
	if !strings.Contains(source, `--default-contextmenu: show`) {
		t.Fatal("workspace right-click menu is not forced on")
	}
	if !strings.Contains(source, `CmdOrCtrl+Shift+I`) || !strings.Contains(source, `workspace.OpenDevTools()`) {
		t.Fatal("workspace developer tools lack an explicit shortcut/menu action")
	}
	if !strings.Contains(source, `appMenu.AddRole(application.EditMenu)`) {
		t.Fatal("standard clipboard menu is missing")
	}
	controlStart := strings.Index(source, `Name: "control"`)
	workspaceStart := strings.Index(source, `Name: "workspace"`)
	if controlStart < 0 || workspaceStart < 0 || strings.Contains(source[controlStart:workspaceStart], `DevToolsEnabled: true`) {
		t.Fatal("control window must not expose workspace diagnostics")
	}
}

func TestProductionPackagesCompileDeveloperToolsForExplicitWorkspaceDiagnostics(t *testing.T) {
	contents, err := os.ReadFile("../../scripts/package.mjs")
	if err != nil {
		t.Fatal(err)
	}
	source := string(contents)
	if !strings.Contains(source, `platform === "linux" ? "production" : "production,devtools"`) {
		t.Fatal("release tags do not preserve production mode and supported workspace diagnostics")
	}
	workflow, err := os.ReadFile("../../.github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(workflow), `go build -tags production,devtools`) {
		t.Fatal("Windows native smoke would compile OpenDevTools as a no-op")
	}
}

func callsIdentifier(node ast.Node, name string) bool {
	found := false
	ast.Inspect(node, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		identifier, ok := call.Fun.(*ast.Ident)
		if ok && identifier.Name == name {
			found = true
			return false
		}
		return true
	})
	return found
}
