# 验证记录

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

前一候选 `be2798c` 的 [六平台 CI](https://github.com/zhangjiawei/dsh-tiny-desktop/actions/runs/33876884592) 已通过。它早于本轮“首次安装自动 latest”和视觉重设计；不能用该结果代替最终 tag 验证。

最终 v0.2.0 tag 工作流须再次通过：macOS Intel/ARM64、Windows x64/ARM64、Linux x64/ARM64 的核心/前端测试、模块完整性、真实 DSH/六插件安装、认证/退出、pnpm dlx 原生 PTY 和打包。三个 amd64 runner 运行 race；两个 Linux runner 另做真实 LAN 认证。全部通过后才能创建公开预发布 Release，之后复核六个资产及 SHA256SUMS.txt。

CI 修复记录：Windows Dock 服务补齐 x/image/x/text 模块记录；`.gitattributes` 固定模块文件 LF，防止 Windows CRLF 被 `go mod tidy -diff` 误报。Windows ConPTY 验证器在输出成功后显式退出，避免后台句柄保留事件循环。

## 历史 v0.1.0

[v0.1.0 发布工作流](https://github.com/zhangjiawei/dsh-tiny-desktop/actions/runs/33873119411) 在 tag `v0.1.0`（`5af9531`）通过全部六平台测试及打包。[Release](https://github.com/zhangjiawei/dsh-tiny-desktop/releases/tag/v0.1.0) 的六个压缩包从公开地址重新下载并通过 SHA256SUMS；Mach-O、PE、ELF 的架构与资产名称一致，两个 macOS 包通过 ad-hoc 完整性检查。

## 限制

- Windows/Linux 原生 GUI 和 macOS ARM64 GUI 未由本机 Intel Mac 代替验证。
- 模型对话、IM 平台绑定与系统通知仍需用户自己的凭据和权限；安装成功不代表所有第三方业务功能已经验证。
- 本机私有 IPv4 HTTP 访问曾因系统网络环境超时；没有改动系统代理/防火墙。Linux CI 的真实 LAN 认证不代表每台用户机器的网络权限配置。
- macOS 仅 ad-hoc 签名，无 Apple Developer ID 公证；Windows 未商业签名。Linux 需要 GTK4/WebKitGTK 6.0。
- 更新入口由用户下载替换，不做静默二进制覆盖；没有分屏。
- DSH 和 Wails v3 本身仍为预发布版本，首次安装时的未来第三方 latest 版本可能引入兼容性变化，安装日志和精确版本回执用于排查。
