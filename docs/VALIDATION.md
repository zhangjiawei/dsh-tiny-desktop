# 验证记录

## v0.2.2 设置功能验证

- 新增回归覆盖默认仓库/启动命令、旧版空命令兼容、私有 CLI 路由、真实端口占用及活动配置与待应用配置的区分。核心测试、race/vet、前端测试和 TypeScript 构建通过；Windows 桌面包本地交叉编译通过，不等同于 Windows 原生执行。
- 视觉预览在 1000×800、820×680 检查齿轮标识、侧栏退出按钮、长命令/限制、仓库默认值、端口提示和英文确认退出弹窗；无横向溢出。预览使用隔离模拟 bridge，仅用于布局，不计作原生功能通过。
- Mac 原生 QA 使用独立临时目录，将 3080 用测试监听器占住，以默认 pnpm 命令及 npmmirror 实际安装/启动：首次在 61674 认证成功，原生界面冲突提示及取消退出回归通过。最终候选构建再次在 61790 就绪，AX 验证提示包含真实端口；点击“退出应用”→“取消”仍为运行中，再点击“确认退出”后 App 正常以 0 退出，重新绑定 61790 成功，确认后台已释放端口。测试监听器已关闭，未影响其他服务。
- Windows CI 新增真实占用 3080 后验证随机服务端口/提示、恢复并使用默认命令及镜像、待应用设置不篡改活动端口提示、取消退出保持服务、确认退出并释放服务端口；正式 exe 另由 UIA 验证 Settings 标题与原生大小图标句柄。
- 六平台首装 smoke 改为实际生产默认 pnpm 命令及国内镜像。[候选工作流](https://github.com/zhangjiawei/dsh-tiny-desktop/actions/runs/33893779512) 的六平台构建、真实 DSH/六插件/PTY 全部通过。Windows x64 job `101091492938`、ARM64 job `101091493116` 的正式 exe UIA 和全部原生 WebView2 用例通过；ARM64 首次安装耗时约 7 分钟，日志持续推进，后续缓存启动正常。最终标签发布和公开下载结果待下方补充。

## v0.2.1 修复验证

- 根因回放：使用 Wails beta.16 Windows `processMessage` 实際字段（Origin/TopOrigin，IsMainFrame=false）运行 `go test ./internal/core -run TestWindowsControlStatusMessage -count=1`，修复前失败，修复后通过。旧版六平台 DSH smoke 没有覆盖原生窗口桥，这是 v0.2.0 漏检原因。
- 来源检查按平台适配；macOS 保留主 frame 校验，Windows 同时检查发送方与顶层控制 URL，Linux 配合 `frame-src 'none'` 的控制页 CSP。外部站点、非控制窗口、错误路径和缺失 Windows 顶层来源均拒绝。
- `TestSaveWhileRunningNeverStopsService` 修复前得到“请先停止服务再修改设置”，修复后确认服务状态/端口不变、配置持久化、待重启标记、恢复原配置清除标记、无效保存不生效。
- 新安装开关默认值及保留已有 false 选择的回归通过。`Log.Lines()` 空列表可序列化为 []，不是 Windows 空白页的根因。
- 本地 `go test ./...`、核心 race/vet、模块完整性、前端测试、TypeScript 构建通过。Mac QA 实测：服务 PID 55052 / 端口 57968 上保存首选端口 43082 后原 PID/端口不变、设置文件持久化、界面显示待应用；点击“应用并重启”后原进程退出，新 PID 55315 在 43082 认证就绪。托盘最小化/关闭/恢复回归通过。所有 QA 进程已正常退出，测试目录设置恢复 3080。
- Windows 验证新增两层：正式打包 exe 由 `windows-control-ready.ps1` 的 UIA 验证真实后台 Stopped 状态；同源码仅通过链接期端口参数构建的 QA exe 由 `windows-webview-smoke.mjs` 操作实际 WebView2/DSH，不模拟 bridge。正式包默认不提供调试端口，环境变量不能启用该编译期选项。
- 第一轮 Windows x64 的真实 DSH/六插件/PTY 均通过，但原生探针超时。原因是 Wails 内部 `preventEnvAndRegistryOverrides` 清空了 SDK 调试环境变量；已改为上述 UIA + 编译期 QA 探针，不将这次探针失败算作原生验证成功。最终标签结果见下方。
- 第二轮 [Windows x64 原生验证](https://github.com/zhangjiawei/dsh-tiny-desktop/actions/runs/33890667333/job/101081634854) 于 2026-09-04 23:43:36（UTC+8）通过：正式 exe UIA 状态；QA WebView2 初始空日志/独立目录；真实启动按钮、认证就绪、端口/日志；保存新端口时当前服务不变；打开窗口、QR、停止、下次启动应用新端口。全部步骤通过后才上传构建产物。
- 同轮 [Windows ARM64 原生验证](https://github.com/zhangjiawei/dsh-tiny-desktop/actions/runs/33890667333/job/101081635148) 的原生控制桥与产物上传步骤也通过。首次安装耗时约 4 分 53 秒，实时日志确认下载安装在推进，随后真实 pnpm dlx/PTY 通过；并非界面连接死锁。

### v0.2.1 发布闭环（2026-09-05，UTC+8）

最终标签 `v0.2.1`（`d9ce446`）的 [发布工作流](https://github.com/zhangjiawei/dsh-tiny-desktop/actions/runs/33891692956) 六个平台及 release job 全部成功。该轮包括 `go test ./...` 的正式包调试边界回归，Windows QA 构建显式固定对应 GOARCH，两个 Windows runner 再次完成正式 exe UIA 与真实 WebView2 全流程验证。

[v0.2.1 Release](https://github.com/zhangjiawei/dsh-tiny-desktop/releases/tag/v0.2.1) 已公开且为预发布，含六个安装包和 SHA256SUMS.txt。发布后从公开链接重新下载六包，全部文件大小及 SHA-256 匹配；解压确认 PE GUI x86-64/Aarch64、Mach-O x86_64/arm64、ELF x86-64/aarch64 与名称匹配。两个 macOS 包通过 `codesign --verify --deep --strict`，仅代表 ad-hoc 完整性，不代表 Apple 公证。Windows 仍未商业签名。

用户升级应从托盘菜单真正退出旧版，再解压新版运行；不删除独立数据目录，不重装已完成安装的六插件。已有开关选择保留，新安装采用新默认值。本轮没有替换原 DshShell，也没有改动原 `~/.dsh`。

## v0.2.0 本地验证（2026-09-04，macOS Intel）

- `go test -race ./internal/core`、`go vet ./internal/core`、`go mod tidy -diff`、`npm test --prefix frontend`、TypeScript 检查和前端打包通过。
- 默认安装行为：不传任何插件策略参数，以全新空目录 `/tmp/dsh-tiny-first-install-6BqjzT` 执行 `go run ./cmd/smoke --root …`。自动下载并校验 Node 24.20.0，解析并安装六插件 latest；21:50:01（UTC+8）完成官方 token→Cookie 认证、端口避让到 50314、CLI/profile 两处真实 PTY 创建/输出/退出及自己的进程停止。
- 本轮解析版本：codex-ui 0.2.103、im-connect 0.1.34、automation 0.1.30、dshmarket 1.41.0、task-complete-notify 0.2.0、better-sidebar 0.18.0。这是当时仓库返回的版本，不是写死的默认版本。
- 设置界面、Settings 模型和 smoke 命令均不再有插件策略开关。回归测试使用默认设置验证六插件 latest 解析；另以不可访问的 registry 验证旧安装回执可以离线复用，不静默升级。
- 控制桥来源回归先失败后通过：允许本地控制文档的 hash 路由，仍拒绝外部站点、伪造域名、其他窗口、子框架和非控制路径。
- 语言轮询回归先失败后通过：重复状态轮询不再重写语言选项，避免打断原生菜单。
- 原生托盘回归：`scripts/desktop-qa-macos.swift <qa-pid> tray-cycle` 通过“最小化 → accessory/零可见窗口 → 托盘恢复 → 关闭 → 再次恢复”，同一 App 进程未退出。修复了 Wails beta.16 Dock API 在 Cocoa 主线程同步派发造成的 SIGILL。
- 本机菜单栏管理工具将测试图标移动到屏幕外（负坐标）。回归使用该状态栏项目的原生 AXPress 触发同一个点击回调，不把屏幕外坐标点击算作成功；没有修改菜单栏管理工具。脚本也会明确拒绝锁屏下的 GUI 验证。
- 原生控制页切换 Settings → Overview 后，打开窗口、浏览器打开、检查更新均执行成功；实际窗口截图确认对应成功提示。QR 弹窗由原生回归确认打开。检查更新是打开本项目 Releases，不是静默下载安装。
- 原生英语 → 跟随系统中文切换通过，菜单同步变更；测试 DSH 子进程 PID 48154 和端口 49859 保持不变。
- 界面重设计已在 1000×740 和 820×680 下检查，中英文均无横向溢出；设置顶部、底部操作区、长命令输入、独立内容滚动、路由返回顶部均检查。浏览器预览使用明确隔离的模拟 bridge，仅用于视觉/排版；不把它当作原生功能验证。
- 真实自定义 pnpm dlx 启动已在此前隔离测试通过，包括实际 dlx 缓存中的原生 PTY；发布工作流会在六个平台再次验证。
- 原 DshShell、原 `~/.dsh` 未修改；测试仅使用专属临时目录。QR/认证链接截图不提交到仓库或公开资产。

## v0.2.0 跨平台发布门禁

最终 tag `v0.2.0`（`3a1140b`）的 [发布工作流](https://github.com/zhangjiawei/dsh-tiny-desktop/actions/runs/33880676139) 已成功完成全部六个 build job 和 release job，覆盖本轮“首次安装自动 latest”和视觉重设计，而非引用前一候选的结果。

macOS Intel/ARM64、Windows x64/ARM64、Linux x64/ARM64 全部通过核心/前端测试、模块完整性、真实 DSH/六个 latest 插件安装、认证/退出、pnpm dlx 原生 PTY 和打包。三个 amd64 runner 运行 race；两个 Linux runner 另做真实 LAN 认证。

[v0.2.0 Release](https://github.com/zhangjiawei/dsh-tiny-desktop/releases/tag/v0.2.0) 已公开（非草稿、预发布），含六个安装包与 SHA256SUMS.txt。发布后重新下载全部六包，SHA-256 与文件大小均通过复核；解压后 Mach-O x86_64/arm64、PE GUI x86-64/Aarch64、ELF x86-64/aarch64 与资产命名一致。两个 macOS 包均通过 `codesign --verify --deep --strict`，这不代表 Apple 公证。

本机应用已替换为公开下载的 macOS Intel 包（可执行文件与下载包逐字节一致），保留原独立数据目录；2026-09-04 22:04:50（UTC+8）再次记录“已认证并就绪，端口 51534”。本地候选包备份保留在忽略目录，未覆盖原 DshShell。

CI 修复记录：Windows Dock 服务补齐 x/image/x/text 模块记录；`.gitattributes` 固定模块文件 LF，防止 Windows CRLF 被 `go mod tidy -diff` 误报。Windows ConPTY 验证器在输出成功后显式退出，避免后台句柄保留事件循环。

## 历史 v0.1.0

[v0.1.0 发布工作流](https://github.com/zhangjiawei/dsh-tiny-desktop/actions/runs/33873119411) 在 tag `v0.1.0`（`5af9531`）通过全部六平台测试及打包。[Release](https://github.com/zhangjiawei/dsh-tiny-desktop/releases/tag/v0.1.0) 的六个压缩包从公开地址重新下载并通过 SHA256SUMS；Mach-O、PE、ELF 的架构与资产名称一致，两个 macOS 包通过 ad-hoc 完整性检查。

## 限制

- Windows x64/ARM64 原生控制页由对应 CI runner 的 UIA 与 WebView2 验证；没有将本机 Mac 结果代替 Windows。Linux 原生 GUI 和 macOS ARM64 GUI 仍未验证。
- 模型对话、IM 平台绑定与系统通知仍需用户自己的凭据和权限；安装成功不代表所有第三方业务功能已经验证。
- 本机私有 IPv4 HTTP 访问曾因系统网络环境超时；没有改动系统代理/防火墙。Linux CI 的真实 LAN 认证不代表每台用户机器的网络权限配置。
- macOS 仅 ad-hoc 签名，无 Apple Developer ID 公证；Windows 未商业签名。Linux 需要 GTK4/WebKitGTK 6.0。
- 更新入口由用户下载替换，不做静默二进制覆盖；没有分屏。
- DSH 和 Wails v3 本身仍为预发布版本，首次安装时的未来第三方 latest 版本可能引入兼容性变化，安装日志和精确版本回执用于排查。
