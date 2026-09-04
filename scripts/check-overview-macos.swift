// A focused native regression probe. Never walks the user's DSH conversation.
import AppKit
import ApplicationServices
let pid = pid_t(CommandLine.arguments[1])!
func attr(_ e: AXUIElement, _ n: String) -> AnyObject? { var v:CFTypeRef?; AXUIElementCopyAttributeValue(e,n as CFString,&v);return v }
func nodes(_ e:AXUIElement)->[AXUIElement]{var seen=Set<CFHashCode>();var result=[AXUIElement]();func walk(_ n:AXUIElement){if !seen.insert(CFHash(n)).inserted{return};result.append(n);for c in attr(n,"AXChildren") as? [AXUIElement] ?? []{walk(c)}};walk(e);return result}
let root=AXUIElementCreateApplication(pid)
guard let w=(attr(root,"AXWindows") as? [AXUIElement] ?? []).first(where:{(attr($0,"AXTitle") as? String ?? "").contains("控制中心")}) else {print("FAIL: no control window");exit(1)}
let button=nodes(w).first{(attr($0,"AXRole") as? String)=="AXButton" && (attr($0,"AXTitle") as? String ?? "").contains("分享二维码")}
guard let button else {print("FAIL: no share button");exit(1)}
AXUIElementPerformAction(button,kAXPressAction as CFString)
RunLoop.current.run(until:Date().addingTimeInterval(2))
let shown=nodes(w).contains{(attr($0,"AXRole") as? String)=="AXButton" && (attr($0,"AXTitle") as? String)=="复制认证链接"}
print(shown ? "PASS: share button opened QR dialog" : "FAIL: share button did not open QR dialog")
exit(shown ? 0 : 1)
