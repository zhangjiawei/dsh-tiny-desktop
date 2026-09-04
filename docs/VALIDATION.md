# 验证记录

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
