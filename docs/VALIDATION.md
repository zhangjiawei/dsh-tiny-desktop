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

## 发布门禁

GitHub Actions 原生六平台测试、打包及 Release 尚在执行前准备阶段；不可将配置好的工作流描述为已通过。每个平台须通过核心测试、真实 DSH 安装/认证/关闭后才提供发布资产。

## 尚未覆盖 / 限制

- Windows/Linux 原生 GUI 操作未由本地 Mac 代替验证。
- 模型对话需要用户自己的 API Key；IM 平台绑定和通知权限未代替用户配置。
- 局域网代理已实现，实际认证/WebSocket 与关闭回归待完成。
- macOS 使用 ad-hoc 签名，无 Apple Developer ID 公证；Windows 无商业签名。
- 更新入口由用户下载替换，不做静默二进制覆盖；不包含分屏或跨平台透明窗口效果。
- DSH 及 Wails v3 本身仍为预发布版本；第三方插件的业务功能不因“安装成功”而视为全部验证。
