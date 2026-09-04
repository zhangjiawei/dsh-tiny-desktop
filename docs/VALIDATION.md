# 验证记录

## v0.2.0 回归（2026-09-04）

- 本机 `go test -race ./internal/core`、`go vet ./internal/core`、`npm test --prefix frontend` 与前端构建通过。
- 来源检查回归先失败后通过：允许可信控制文档的 `#overview` / `#settings`，拒绝外部站点、伪造域名、其他窗口、子框架和非控制路径。原生分享按钮在旧版失败，修复后能够打开认证二维码弹窗。
- 前端语言轮询回归先失败后通过：重复状态轮询不再重写语言选项 DOM，避免打断原生下拉菜单。
- macOS 原生窗口验证英文即时切换、英文菜单、系统中文识别；切换外观不重启 DSH。隐藏到托盘时 activation policy 为 accessory，屏幕可见应用窗口数为零。
- 托盘恢复测试捕获 Wails beta.16 Dock API 主线程同步派发造成的 SIGILL。已将托盘和菜单动作移至 Go 工作协程；修复后的完整隐藏/恢复回归等待解锁本机桌面，不能据代码检查标为通过。
- 隔离目录 `/tmp/dsh-tiny-v020-latest-20260904` 完成真实 latest 安装：codex-ui 0.2.103、im-connect 0.1.34、automation 0.1.30、dshmarket 1.41.0、task-complete-notify 0.2.0、better-sidebar 0.18.0；普通重启复用精确版本回执。
- 同一隔离目录使用 UI 示例中的 `pnpm --allow-build=… dlx @deepseek-ai/dsh@0.1.2-rc.1 web` 完成真实端口避让、token→Cookie 认证和进程停止；CLI、profile、实际 dlx 缓存三处原生 PTY 创建/输出/退出通过。没有用受管 CLI 的 PTY 结果冒充 dlx 测试。
- 候选代码 `be2798c` 的 [v0.2.0 六平台 CI](https://github.com/zhangjiawei/dsh-tiny-desktop/actions/runs/33876884592) 已成功完成。macOS Intel/ARM64、Windows x64/ARM64、Linux x64/ARM64 全部通过核心测试、前端回归、真实 DSH/六插件安装、认证、进程退出、真实自定义 pnpm dlx 与其原生 PTY、原生打包及产物上传。三个 amd64 runner 额外通过 race；两个 Linux runner 额外通过真实 LAN 认证。
- CI 曾捕获并修复 Windows Dock 服务缺少 x/image/x/text 模块校验记录；新增 `go mod tidy -diff` 门禁。Windows 检出 CRLF 导致的纯换行误报也已通过 `.gitattributes` 固定 LF 解决，而非移除检查。
- 本地最终候选包重新打包并通过 `codesign --verify --deep --strict`，正常退出旧 DSH Tiny 后使用同一独立目录启动新版；2026-09-04 21:17:35（UTC+8）记录“已认证并就绪，端口 64913”。旧包备份保留在忽略目录 `work/DSH Tiny before-final-v020.app`；原 DshShell 和原数据目录未改变。
- **发布仍待完成**：Mac 当前锁屏，`scripts/desktop-qa-macos.swift <qa-pid> tray-cycle` 明确返回 BLOCKED，不把丢弃的鼠标事件算作成功。解锁后需在隔离 QA 实例验证完整最小化/关闭/托盘恢复循环，以及切换路由后的打开窗口、打开浏览器、检查更新和 QR 按钮。然后创建 v0.2.0 tag，等待发布工作流并校验公开资产；目前尚未创建 v0.2.0 Release。
- 下面的历史发布记录仅属于 v0.1.0，不是 v0.2.0 的 GUI 或公开资产验证结果。

## 本地验证（2026-09-04，macOS Intel）

- `go test -race ./internal/core`：通过。覆盖设置原子保存、代理约束、认证 URL 校验、Cookie/HTML 就绪、端口避让、日志脱敏、归档路径、导入备份恢复、符号链接拒绝和六平台 Node 清单。
- Windows amd64 / arm64 核心交叉编译：通过；这不是 Windows 原生运行验证。
- `npm run build --prefix frontend`：TypeScript 类型检查与 ES module 打包通过。
- `go run ./cmd/smoke --root /tmp/dsh-tiny-smoke-20260904`：实际下载 Node 24.20.0 并通过 SHA-256，安装 DSH 0.1.2-rc.1 和六个插件；3080 占用后切换 59406；完成官方 token→Cookie 认证，读取真实 HTML，并停止自己的进程。
- 官方 `--profile web --dump-config`：六个默认 bundle 均在最终配置中。
- Wails 原生窗口：控制中心显示运行中；主窗口显示真实 DSH 新用户配置页。`scripts/inspect-macos.swift <test-pid> --verify` 已从失败转为通过。
- 修复并回归：控制页必须打包为 ES module；macOS 主窗口应从 `about:blank` 启动，避免从 `wails://` 跳转时 SameSite=Strict Cookie 导致首次认证重定向失败。
- 原 DshShell、原 `~/.dsh` 未修改；测试使用专属临时目录。
- 第三轮全新隔离安装 `/tmp/dsh-tiny-release-check-terminal-20260904`：私有 Node 下载、六插件注册、CLI/profile 两套真实 PTY 创建/输出/退出、3080 冲突回退到 61220 全部通过。
- `go vet ./internal/core` 与提交内容敏感信息模式检查：通过。

## 发布门禁

候选代码 `71cd29a` 的 [原生 CI 记录](https://github.com/zhangjiawei/dsh-tiny-desktop/actions/runs/33872211021)：

| 平台 | 核心测试 | 实际 DSH / 六插件 / 原生 PTY | 原生打包 | 人工 GUI |
|---|---|---|---|---|
| macOS Intel | 通过（含 race） | 通过 | 通过 | 本地通过 |
| macOS ARM64 | 通过 | 通过 | 通过 | 未验证 |
| Windows x64 | 通过（含 race） | 通过 | 通过 | 未验证 |
| Windows ARM64 | 通过 | 通过 | 通过 | 未验证 |
| Linux x64 | 通过（含 race） | 通过 | 通过 | 未验证 |
| Linux ARM64 | 通过 | 通过 | 通过 | 未验证 |

两个 Linux runner 额外通过真实私有 IPv4 地址的 DSH token→Cookie 认证。修复并回归了 Windows `/absolute` 归档路径判定，以及一次性 ConPTY 验证器退出后事件循环被后台句柄保留的问题。测试不再仅根据原生依赖文件存在判定终端可用。

[v0.1.0 发布工作流](https://github.com/zhangjiawei/dsh-tiny-desktop/actions/runs/33873119411) 已在 tag `v0.1.0`（`5af9531`）上再次通过全部六平台测试及打包，`release` job 成功。公开 [Release](https://github.com/zhangjiawei/dsh-tiny-desktop/releases/tag/v0.1.0) 包含六个压缩包及 SHA256SUMS.txt，标记为预发布、非草稿。

发布后从公开下载地址重新下载全部六个安装包：`shasum -a 256 -c SHA256SUMS.txt` 全部 OK。解压后 `file` 确认 Mach-O x86_64/arm64、PE GUI x86-64/Aarch64、ELF x86-64/aarch64 与资产名称一致。两个 macOS `.app` 均通过 `codesign --verify --deep --strict`；这仅验证 ad-hoc 完整性，不代表 Apple 公证。

## 尚未覆盖 / 限制

- Windows/Linux 原生 GUI 操作未由本地 Mac 代替验证。
- 模型对话需要用户自己的 API Key；IM 平台绑定和通知权限未代替用户配置。
- 局域网 Host 保留、错误 Host 拒绝、WebSocket 双向转发的自动化测试通过。当前本地 Mac 访问最小私有 IPv4 HTTP 测试服务器也会超时，因此该机器的真实 LAN 认证尚未通过；未修改系统代理/防火墙。Linux CI 的真实 LAN 认证已通过，但不代表所有机器的网络权限配置。
- 全新隔离目录的第二轮验证：六个固定版本插件注册、CLI 和插件 profile 中真实原生 PTY 的创建/输出/退出通过，不仅检查包文件存在。
- macOS 使用 ad-hoc 签名，无 Apple Developer ID 公证；Windows 无商业签名。
- 更新入口由用户下载替换，不做静默二进制覆盖；不包含分屏或跨平台透明窗口效果。
- DSH 及 Wails v3 本身仍为预发布版本；第三方插件的业务功能不因“安装成功”而视为全部验证。
