# DSH Tiny Desktop

An independent, lightweight desktop home for [DeepSeek Harness](https://github.com/deepseek-ai/deepseek-harness), built with **Wails v3, Go and TypeScript**. 中文界面，独立数据，原生 WebView，不修改 DSH 核心或已有的 DshShell。

## 使用

从 [GitHub Releases](https://github.com/zhangjiawei/dsh-tiny-desktop/releases) 下载对应平台/架构的压缩包，校验 `SHA256SUMS.txt` 后解压运行。

- macOS 13+：将 `DSH Tiny.app` 拖到 Applications。当前为 ad-hoc 签名，**没有 Apple Developer ID 公证**；系统可能要求在隐私与安全中确认打开。不要关闭系统 Gatekeeper。
- Windows：运行 `dsh-tiny.exe`，需要 Microsoft WebView2 Runtime。首版未做商业代码签名，可能显示 SmartScreen 提示。
- Linux：运行 `./dsh-tiny`；需要 GTK4 / WebKitGTK 6.0（Ubuntu 24.04: `libgtk-4-1 libwebkitgtk-6.0-4`）。通知还需要 `notify-send`。

首次运行会在应用专属目录下载运行环境和插件，需要联网与可用磁盘空间。仅复用精确匹配的 Node **24.20.0**；其他版本或缺少 Node/npm 时下载该固定版本并验证内置 SHA-256。不会执行 sudo、不会全局安装或修改 PATH，系统中是否已全局安装 dsh 不影响独立运行环境。

默认端口 **3080**；自动检测占用，不终止占用者。被占用时选择随机可用端口，并在概览醒目显示“端口已占用，已使用随机端口 xxxx”，其中 xxxx 是实际启动成功的服务端口。安装、启动和错误状态均在“设置”窗口查看；通过系统托盘或 `Cmd/Ctrl+,` 打开。

已全局安装 DSH 也可以使用：外壳管理自己的 DSH 运行环境和数据目录，不修改全局 DSH。Windows 启动时对系统 Node/语言的短命令探测也会隐藏控制台，不会因全局 Node 存在而闪出命令窗口。安装中断或插件清单与旧锁文件不匹配时，重试会在私有 profile 内协调锁文件；Node 解压目录被杀毒软件短暂占用会自动重试，重启留下的半成品运行时会在可恢复备份后替换，无需删除整个数据目录。已安装完成后更换仓库也不会强制重装/升级插件。

### v0.2 桌面设置

- **托盘模式**：开启“仅驻留托盘”后，最小化或点 × 会隐藏本应用所有窗口，只保留菜单栏/托盘图标，DSH 继续运行。单击图标恢复并置前，右键打开菜单。关闭此开关则保留普通 Dock/任务栏行为。Linux 桌面需支持系统托盘；也可再次启动应用恢复设置。菜单栏管理工具可能隐藏图标，请在对应工具中显示它。
- **语言**：默认跟随操作系统的 UI 语言，支持简体中文、English；其他语言回退 English。选择后立即生效，原生菜单同步更新，不重启 DSH。这里只控制外壳，DSH 工作空间保持自己的语言设置。
- **运行配置**：语言、置顶、隐藏策略即时生效；运行中也可以保存启动命令、仓库、端口等，不停止当前服务。待应用的启动配置会明确提示，下次启动生效；需要立即应用时点击“应用并重启”。无效输入会先被校验，不会先停掉正常会话。
- **新安装默认开关**：自动启动、关闭后继续运行、仅驻留托盘、局域网分享开启；工作空间置顶关闭。升级保留已有设置，不覆盖用户选择。凭据导入仍需单独主动勾选。
- **设置窗口**：采用齿轮标识，Windows 使用独立的窗口齿轮图标，Linux 使用窗口图标；macOS 保留系统惯例，共享应用 Dock 图标仍为 DSH Tiny。侧栏“退出应用”经确认后停止后台 DSH 并退出；取消不会中断服务。
- **启动命令**：新安装默认直接填入下方私有 `pnpm dlx` 命令，仓库默认填入 `https://registry.npmmirror.com`。两项均可编辑和单独恢复默认。已有自定义值保留；旧版空命令显示为 `dsh web`，仍使用原私有托管 CLI，不强制改成 dlx 重新下载。支持引号，不执行 Shell 管道、变量替换或重定向。端口与 `--no-open` 由外壳追加，不能自行覆盖监听地址、认证或关闭认证。局域网分享可选用正确参数名 `--trusted-host <私有 IPv4>` 指定网卡地址；未填写时优先采用默认路由对应的局域网地址。这些限制也在输入框下说明。自定义版本的兼容性由所选 DSH/插件决定。

  Windows 的无控制台程序中，pnpm 10.28.0 的 `dlx` 在 DSH 极短的 postinstall 结束时存在输出管道竞态。仅当命令精确匹配应用给出的默认示例（可附加 `--trusted-host`）时，外壳会执行首次准备阶段已经安装、重建且版本相同的私有 DSH，避免重复 dlx 安装；修改版本、程序或其他参数后仍按自定义命令原样执行。不会调用全局 dsh。

可点击“填入 pnpm dlx 示例”获得以下命令（原生依赖需要明确许可，不能全部放行）：

```sh
pnpm --allow-build=@deepseek-ai/dsh-subprocess-local --allow-build=node-pty --allow-build=koffi dlx @deepseek-ai/dsh@0.1.2-rc.1 web
```

默认使用 npmmirror 国内镜像（镜像同步可能延迟）；命令继承设置中的 HTTPS 仓库，显式 `--config.registry=…` 优先。pnpm 由本应用私有 Node 启动，dlx 缓存放在 `runtime/pnpm-cache`；不要求全局安装 pnpm。启动等待可设 1–120 分钟，默认 60 分钟，适应 dlx 首次下载。六插件仍在首次安装时解析所选仓库的 latest，安装回执存在时保持已安装版本。

**首次安装即最新版**：全新数据目录首次启动时，自动从仓库解析六个预置插件的 `latest` 并安装，不需要设置开关。实际版本记录到 `runtime/receipt.json`，普通重启和升级外壳会复用已有安装，不会静默升级插件。首次安装需要联网；解析失败会显示错误，重试即可，不会悄悄回退到写死的旧版本。自定义命令中的 DSH 版本与这六个插件版本是两回事。

浏览器必须完成 DSH 官方认证。用“在浏览器中打开”或“复制认证链接”完成首次登录；不需要从日志里找 token。**链接是完整访问凭证，请勿公开分享。** v0.2.1 新安装默认开启可信私有局域网分享，可在设置中关闭；二维码使用实际启用的私有 IPv4 地址，仍由 DSH 校验认证与 Host。Windows 会优先选择默认路由网卡而不是 WSL/Hyper-V 等虚拟接口；特殊多网卡环境可在启动命令指定 `--trusted-host`。未找到私有 IPv4 时继续仅本机访问，并写入日志，不阻止启动。HTTP 局域网流量未加密，不要用于公共 Wi-Fi、公网或不可信网络；操作系统防火墙/本地网络权限可能需要用户授权。已有配置的局域网开关保持原选择。

## 独立数据与导入

默认根目录：

| 系统 | 目录 |
|---|---|
| macOS | `~/Library/Application Support/dsh-tiny-desktop` |
| Windows | `%AppData%/dsh-tiny-desktop` |
| Linux | `$XDG_CONFIG_HOME/dsh-tiny-desktop` 或 `~/.config/dsh-tiny-desktop` |

根目录下 `dsh/` 是独立 DSH_HOME，`runtime/` 是托管运行环境，`logs/runtime.log` 是脱敏日志（单文件 5 MiB 轮换，保留上一份）。测试可通过 `DSH_TINY_HOME` 指定另一个根目录。

导入前先退出源 DSH，再停止本应用服务。设置选择原 `~/.dsh`，预览文件数量/大小，按需勾选凭据。仅导入会话、附件、技能、设置、agent presets 和 task-board；不导入执行代码、profiles、缓存、锁。目标数据会保留为 `backup-*`，导入后可恢复。原目录只读。

## 六个预置插件

| 插件 | 首次安装 |
|---|---|
| @michengai/dsh-codex-ui | 自动解析 latest |
| @michengai/dsh-im-connect | 自动解析 latest |
| @michengai/dsh-automation | 自动解析 latest |
| dshmarket | 自动解析 latest |
| task-complete-notify-for-dsh | 自动解析 latest |
| dsh-better-sidebar | 自动解析 latest |

DSH **0.1.2-rc.1**，pnpm **10.28.0**，Wails **v3.0.0-beta.16**。首次初始化通过官方 `dsh plugin --profile web` 安装并注册；关闭 peer 自动安装，由宿主提供 DSH API；原生构建仅允许列出的依赖。插件是独立第三方代码，有自己的许可证与网络/通知行为。

## 开发与验证

```sh
cd frontend
npm ci
npm run build
cd ..
go test -race ./internal/core
go run ./cmd/smoke --root /tmp/dsh-tiny-disposable-test
go run ./cmd/desktop
node scripts/package.mjs
```

需要 Go 版本以 `go.mod` 为准，以及对应平台的 Wails 原生开发依赖。主窗口加载真实 loopback DSH 页面；可信本地控制窗口通过严格校验窗口身份/来源的消息桥控制 Go 核心。没有向 DSH 页面暴露 Go 服务、没有 iframe、没有注入 DSH DOM。

测试矩阵与已知限制见 [VALIDATION](docs/VALIDATION.md)。GitHub Actions 在原生六平台 runner 上执行核心测试、真实隔离安装/认证/关闭，然后构建发布包；全部成功后才创建 Release。代码有安全边界、平台差异与恢复流程注释。

## 范围

不提供分屏。首版采用原生菜单/托盘加独立设置，不承诺与 DshShell 的私有实现或所有外观效果完全一致。应用更新入口打开官方项目 Releases，由用户校验并替换程序；不会静默覆盖正在运行的二进制。

本项目非 DeepSeek 或 DshShell 官方产品，不复制其品牌图标或源代码。原始 shell 代码采用 MIT；运行时与插件保留各自许可。
