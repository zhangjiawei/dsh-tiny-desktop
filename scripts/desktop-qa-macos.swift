// Native controls only. Never dump the DSH conversation or read the clipboard.
import AppKit
import ApplicationServices

let pid = pid_t(CommandLine.arguments[1])!
let action = CommandLine.arguments[2]
// A locked session can return AXApplication objects in place of windows and
// silently discard mouse events. Never report that as a successful GUI test.
if (CGSessionCopyCurrentDictionary() as? [String: Any])?["CGSSessionScreenIsLocked"] as? Bool
  == true
{
  print("BLOCKED: unlock the macOS desktop before native UI verification")
  exit(2)
}
func attr(_ e: AXUIElement, _ n: String) -> AnyObject? {
  var v: CFTypeRef?
  AXUIElementCopyAttributeValue(e, n as CFString, &v)
  return v
}
func nodes(_ e: AXUIElement) -> [AXUIElement] {
  var seen = Set<CFHashCode>()
  var result = [AXUIElement]()
  func walk(_ n: AXUIElement) {
    if !seen.insert(CFHash(n)).inserted { return }
    result.append(n)
    for c in attr(n, "AXChildren") as? [AXUIElement] ?? [] { walk(c) }
  }
  walk(e)
  return result
}
func title(_ e: AXUIElement) -> String { attr(e, "AXTitle") as? String ?? "" }
func clickElement(_ e: AXUIElement) -> Bool {
  guard let pos = attr(e, "AXPosition"), let size = attr(e, "AXSize") else { return false }
  var point = CGPoint.zero
  var dimensions = CGSize.zero
  guard AXValueGetValue(pos as! AXValue, .cgPoint, &point),
    AXValueGetValue(size as! AXValue, .cgSize, &dimensions), dimensions.width > 0,
    dimensions.height > 0
  else { return false }
  point.x += dimensions.width / 2
  point.y += dimensions.height / 2
  guard CGDisplayBounds(CGMainDisplayID()).contains(point) else { return false }
  CGEvent(
    mouseEventSource: nil, mouseType: .leftMouseDown, mouseCursorPosition: point, mouseButton: .left
  )?.post(tap: .cghidEventTap)
  CGEvent(
    mouseEventSource: nil, mouseType: .leftMouseUp, mouseCursorPosition: point, mouseButton: .left)?
    .post(tap: .cghidEventTap)
  RunLoop.current.run(until: Date().addingTimeInterval(1))
  return true
}
func pressTray(_ item: AXUIElement) -> Bool {
  // Menu bar managers may move a live status item off-screen. AXPress still
  // exercises its native click callback; an off-screen mouse click does not.
  if AXUIElementPerformAction(item, kAXPressAction as CFString) == .success {
    RunLoop.current.run(until: Date().addingTimeInterval(0.5))
    return true
  }
  return clickElement(item)
}
let app = AXUIElementCreateApplication(pid)
let windows = attr(app, "AXWindows") as? [AXUIElement] ?? []
let control = windows.first { title($0).contains("设置") || title($0).contains("Settings") }
if action == "tray-cycle" {
  func expect(_ condition: Bool, _ message: String) {
    if !condition {
      print("FAIL:", message)
      exit(1)
    }
  }
  func visible() -> Int {
    return
      (CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID)
      as? [[String: Any]] ?? []).filter {
        ($0[kCGWindowOwnerPID as String] as? Int) == Int(pid)
          && ($0[kCGWindowLayer as String] as? Int) == 0
      }.count
  }
  func waitForVisible() {
    let end = Date().addingTimeInterval(5)
    while Date() < end {
      let running = NSRunningApplication(processIdentifier: pid)
      expect(
        running != nil && running?.isTerminated == false,
        "application crashed during tray transition")
      if visible() > 0 && running?.activationPolicy == .regular { return }
      RunLoop.current.run(until: Date().addingTimeInterval(0.1))
    }
    expect(false, "window/Dock did not restore")
  }
  func waitForNativeMinimise(_ window: AXUIElement) {
    let end = Date().addingTimeInterval(5)
    while Date() < end {
      let running = NSRunningApplication(processIdentifier: pid)
      expect(
        running != nil && running?.isTerminated == false,
        "application crashed during native minimise")
      let minimised = attr(window, "AXMinimized") as? Bool ?? false
      if minimised && visible() == 0 && running?.activationPolicy == .regular { return }
      RunLoop.current.run(until: Date().addingTimeInterval(0.1))
    }
    expect(false, "minimise did not preserve the native Dock entry")
  }
  func waitForTrayHidden() {
    let end = Date().addingTimeInterval(5)
    while Date() < end {
      let running = NSRunningApplication(processIdentifier: pid)
      expect(
        running != nil && running?.isTerminated == false,
        "application crashed during close-to-tray")
      if visible() == 0 && running?.activationPolicy == .accessory { return }
      RunLoop.current.run(until: Date().addingTimeInterval(0.1))
    }
    expect(false, "close did not hide windows/Dock into the tray")
  }
  guard let control, let extras = attr(app, "AXExtrasMenuBar"),
    let item = nodes(extras as! AXUIElement).first(where: {
      (attr($0, "AXRole") as? String) == "AXMenuBarItem"
    })
  else {
    print("FAIL: native control/tray unavailable")
    exit(1)
  }
  expect(
    AXUIElementSetAttributeValue(control, "AXMinimized" as CFString, kCFBooleanTrue) == .success,
    "minimize rejected")
  waitForNativeMinimise(control)
  expect(pressTray(item), "tray click unavailable")
  waitForVisible()
  let restored = attr(app, "AXWindows") as? [AXUIElement] ?? []
  guard let restoredWindow = restored.first(where: {
    title($0) == "DSH Tiny" || title($0).contains("设置") || title($0).contains("Settings")
  }), let close = attr(restoredWindow, "AXCloseButton")
  else {
    print("FAIL: no window was restored")
    exit(1)
  }
  expect(
    AXUIElementPerformAction(close as! AXUIElement, kAXPressAction as CFString) == .success,
    "close rejected")
  waitForTrayHidden()
  expect(pressTray(item), "second tray click unavailable")
  waitForVisible()
  print("PASS: minimize keeps Dock → tray restore → close hides to tray → restore (same app process)")
  exit(0)
}
if action == "quit" {
  guard let bar = attr(app, "AXMenuBar") else { exit(1) }
  let items = nodes(bar as! AXUIElement)
  guard
    let quit = items.first(where: {
      (attr($0, "AXRole") as? String) == "AXMenuItem" && ["退出", "Quit"].contains(title($0))
    })
  else {
    print("FAIL: no quit menu")
    exit(1)
  }
  print("QUIT", AXUIElementPerformAction(quit, kAXPressAction as CFString).rawValue)
  exit(0)
}
if action == "state" {
  print("POLICY", NSRunningApplication(processIdentifier: pid)?.activationPolicy.rawValue ?? -1)
  let visible =
    (CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID)
    as? [[String: Any]] ?? []).filter {
      ($0[kCGWindowOwnerPID as String] as? Int) == Int(pid)
        && ($0[kCGWindowLayer as String] as? Int) == 0
    }
  print("VISIBLE", visible.count)
  for w in windows {
    print(
      "WINDOW",
      title(w).contains("设置") || title(w).contains("Settings") ? "control" : "workspace",
      "MINIMIZED", attr(w, "AXMinimized") ?? "unknown" as AnyObject)
  }
  exit(0)
}
if action == "close-workspace" {
  guard let workspace = windows.first(where: { title($0) == "DSH Tiny" }),
    let button = attr(workspace, "AXCloseButton")
  else { exit(1) }
  print(
    "CLOSE", AXUIElementPerformAction(button as! AXUIElement, kAXPressAction as CFString).rawValue)
  exit(0)
}
if action == "control" || action == "assert-menu" {
  guard let bar = attr(app, "AXMenuBar") else { exit(1) }
  guard
    let item = nodes(bar as! AXUIElement).first(where: {
      (attr($0, "AXRole") as? String) == "AXMenuItem"
        && ["Settings", "设置"].contains(title($0))
    })
  else {
    print("FAIL: control menu missing")
    exit(1)
  }
  if action == "control" { AXUIElementPerformAction(item, kAXPressAction as CFString) }
  print("MENU", title(item))
  exit(0)
}
if action == "tray" {
  guard let extras = attr(app, "AXExtrasMenuBar") else {
    print("FAIL: no extras menu bar")
    exit(1)
  }
  guard
    let item = nodes(extras as! AXUIElement).first(where: {
      (attr($0, "AXRole") as? String) == "AXMenuBarItem"
    })
  else { exit(1) }
  print("TRAY CLICK", pressTray(item))
  exit(0)
}
guard let control else {
  print("FAIL: control missing")
  exit(1)
}
AXUIElementPerformAction(control, kAXRaiseAction as CFString)
if action == "minimize" {
  print(
    "MINIMIZE",
    AXUIElementSetAttributeValue(control, "AXMinimized" as CFString, kCFBooleanTrue).rawValue)
  exit(0)
}
if action == "close" {
  if let b = attr(control, "AXCloseButton") {
    print("CLOSE", AXUIElementPerformAction(b as! AXUIElement, kAXPressAction as CFString).rawValue)
  }
  exit(0)
}
let all = nodes(control)
if action == "language" {
  guard
    let popup = all.first(where: {
      (attr($0, "AXRole") as? String) == "AXPopUpButton"
        && (title($0).hasPrefix("语言") || title($0).hasPrefix("Language"))
    })
  else {
    print("FAIL: language picker missing")
    exit(1)
  }
  guard let pos = attr(popup, "AXPosition"), let size = attr(popup, "AXSize") else { exit(1) }
  var point = CGPoint.zero
  var dimensions = CGSize.zero
  guard AXValueGetValue(pos as! AXValue, .cgPoint, &point),
    AXValueGetValue(size as! AXValue, .cgSize, &dimensions), dimensions.width > 0
  else { exit(1) }
  point.x += dimensions.width / 2
  point.y += dimensions.height / 2
  CGEvent(
    mouseEventSource: nil, mouseType: .leftMouseDown, mouseCursorPosition: point, mouseButton: .left
  )?.post(tap: .cghidEventTap)
  CGEvent(
    mouseEventSource: nil, mouseType: .leftMouseUp, mouseCursorPosition: point, mouseButton: .left)?
    .post(tap: .cghidEventTap)
  RunLoop.current.run(until: Date().addingTimeInterval(0.3))
  func key(_ code: CGKeyCode) {
    CGEvent(keyboardEventSource: nil, virtualKey: code, keyDown: true)?.post(tap: .cghidEventTap)
    CGEvent(keyboardEventSource: nil, virtualKey: code, keyDown: false)?.post(tap: .cghidEventTap)
    RunLoop.current.run(until: Date().addingTimeInterval(0.1))
  }
  key(126)
  key(126)
  key(126)
  let selected = CommandLine.arguments[3]
  if selected == "zh" || selected == "en" { key(125) }
  if selected == "en" { key(125) }
  key(36)
  RunLoop.current.run(until: Date().addingTimeInterval(1))
  print("SELECT", selected)
  exit(0)
}
if action == "press" {
  let wanted = CommandLine.arguments[3]
  guard
    let item = all.first(where: {
      ["AXButton", "AXLink", "AXCheckBox", "AXPopUpButton", "AXMenuItem"].contains(
        attr($0, "AXRole") as? String ?? "") && title($0) == wanted
    })
  else {
    print("FAIL: target not found", wanted)
    exit(1)
  }
  print("PRESS", AXUIElementPerformAction(item, kAXPressAction as CFString).rawValue)
  RunLoop.current.run(until: Date().addingTimeInterval(0.5))
  exit(0)
}
if action == "inspect" {
  for e in all {
    let role = attr(e, "AXRole") as? String ?? ""
    if ["AXButton", "AXLink", "AXCheckBox", "AXPopUpButton", "AXTextField", "AXMenuItem"].contains(
      role)
    {
      print(role, title(e), attr(e, "AXValue") ?? "" as AnyObject)
    }
  }
  exit(0)
}
if action == "assert" {
  let expected = CommandLine.arguments[3]
  let found = all.contains {
    [title($0), attr($0, "AXValue") as? String ?? ""].contains(where: { $0.contains(expected) })
  }
  print(found ? "PASS" : "FAIL", expected)
  exit(found ? 0 : 1)
}
