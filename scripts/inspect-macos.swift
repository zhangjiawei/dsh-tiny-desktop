// Local QA helper: inspect only the explicitly supplied test app PID. Never
// captures another app or the entire desktop; redact bearer tokens in AX text.
import AppKit
import ApplicationServices
let pid = pid_t(CommandLine.arguments[1])!
var failed=false
var webAreas=0
var running=false
var dshLoaded=false
var visited=Set<CFHashCode>()
func attr(_ el: AXUIElement, _ name: String) -> AnyObject? { var v: CFTypeRef?; AXUIElementCopyAttributeValue(el,name as CFString,&v);return v }
func walk(_ el: AXUIElement,_ depth:Int){
 if !visited.insert(CFHash(el)).inserted{return}
 if depth > 22 {return};let role=attr(el,"AXRole") as? String ?? "?"
 if role=="AXWebArea"{webAreas += 1}
 let fields=["AXTitle","AXDescription","AXValue"].compactMap{attr(el,$0) as? String}.map{$0.replacingOccurrences(of:"token=[^\\s&]+",with:"token=<REDACTED>",options:.regularExpression)}
 if role=="AXWebArea" && fields.contains("DeepSeek Harness"){dshLoaded=true}
 if fields.contains(where:{$0.contains("authentication required") || $0.contains("连接控制中心…")}) {failed=true}
 if fields.contains("运行中"){running=true}
 if CommandLine.arguments.count>2 && (role=="AXButton" || role=="AXLink") && fields.contains(CommandLine.arguments[2]) {print("PRESS",AXUIElementPerformAction(el,kAXPressAction as CFString).rawValue);exit(0)}
 print(String(repeating:" ",count:depth)+role+" "+fields.joined(separator:" | ").prefix(160))
 for child in attr(el,"AXChildren") as? [AXUIElement] ?? [] {walk(child,depth+1)}
}
for w in attr(AXUIElementCreateApplication(pid),"AXWindows") as? [AXUIElement] ?? [] {walk(w,0)}
for w in CGWindowListCopyWindowInfo([.optionOnScreenOnly,.excludeDesktopElements],kCGNullWindowID) as? [[String:Any]] ?? [] {if w[kCGWindowOwnerPID as String] as? Int == Int(pid) {print("WINDOW",w[kCGWindowNumber as String] ?? "",w[kCGWindowName as String] ?? "")}}
if CommandLine.arguments.contains("--verify") {failed = failed || webAreas < 2 || !dshLoaded;print(failed ? "FAIL: native readiness/authentication" : "PASS: native readiness/authentication");exit(failed ? 1 : 0)}
