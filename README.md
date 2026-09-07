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

### v0.3 桌面设置

- **托盘模式**：最小化始终使用系统原生行为，窗口保留在 Dock/任务栏。开启“仅驻留托盘”后，只有点 × 关闭才隐藏本应用所有窗口，只保留菜单栏/托盘图标，DSH 继续运行。单击图标恢复并置前，右键打开菜单。关闭此开关则保留普通关闭行为。Linux 桌面需支持系统托盘；也可再次启动应用恢复设置。菜单栏管理工具可能隐藏图标，请在对应工具中显示它。
- **语言**：默认跟随操作系统的 UI 语言，支持简体中文、English；其他语言回退 English。选择后立即生效，原生菜单同步更新，不重启 DSH。这里只控制外壳，DSH 工作空间保持自己的语言设置。
- **运行配置**：语言、置顶、隐藏策略即时生效；运行中也可以保存启动命令、仓库、端口等，不停止当前服务。待应用的启动配置会明确提示，下次启动生效；需要立即应用时点击“应用并重启”。无效输入会先被校验，不会先停掉正常会话。
- **一键重启**：概览页运行中直接点击“↻ 一键重启”，应用只停止自己管理的 DSH 进程树并立即重新启动；认证就绪后自动恢复工作空间，无需手动停止再启动。
- **Web 插件更新重启**：普通 Web/CLI 进程没有官方桌面端的 Node IPC，所以 Web 页自己的“重连”不等同于重启 DSH 进程。Tiny 会监听独立 `profiles/web/package.json`；只有插件安装标记消失且清单连续稳定后，才停止并重新拉起自己管理的 DSH 进程树。它与概览“一键重启”达到相同的进程级结果，不修改 DSH 或插件源码；失败且仍保留 pending 标记时不会误重启。
- **新安装默认开关**：自动启动、关闭后继续运行、仅驻留托盘、局域网分享开启；工作空间置顶关闭。升级保留已有设置，不覆盖用户选择。凭据导入仍需单独主动勾选。
- **设置窗口**：采用齿轮标识，Windows 使用独立的窗口齿轮图标，Linux 使用窗口图标；macOS 保留系统惯例，共享应用 Dock 图标仍为 DSH Tiny。侧栏“退出应用”经确认后停止后台 DSH 并退出；取消不会中断服务。
- **DSH 版本与启动命令**：默认是“Tiny 托管”，版本、安装、切换和回退只有一个管理入口。用户只填写附加参数；端口、`web`、`--no-open`、监听地址和认证始终由外壳管理。局域网绑定 IPv4、反向代理可信主机和公网地址各自独立配置；普通启动只复用已安装版本，不联网、不解析浮动标签，也不静默升级。
- **更新通道**：“Tiny 推荐”跟随桌面版本发布并经过完整矩阵验证；“正式稳定版”选择仓库中最高的非预发布版本；“预览版”比较 `latest` 与 `next`；“固定版本”只接受完整版本号。检查更新只读仓库元数据，不停服务。显式升级会保留当前六插件的精确版本，避免 DSH 与插件同时漂移。
- **事务升级与回退**：新 DSH 先安装到同盘临时槽位，动态识别且只放行 subprocess-local、node-pty、koffi 三项原生构建依赖，完成 CLI 自检后才切换。运行中的服务还必须重新完成 token 认证就绪检查；任何阶段失败都会恢复旧运行时和升级前的 Profile 控制状态。成功后只保留一个上一版本及其恢复点，中断遗留会在下次启动清理。
- **高级自定义模式**：可切换到“自定义完整命令”；此时命令逐参数直接执行，不经 Shell，Tiny 不再决定 DSH 版本，也不会提供托管升级或回退。切回托管模式不会删除命令。v0.2 及更早版本的默认命令（含可选 trusted-host）会自动迁移为托管模式，真正修改过的命令逐字保留为自定义模式。

高级模式可点击“填入 pnpm dlx 示例”获得以下命令（原生依赖需要明确许可，不能全部放行）：

```sh
pnpm --allow-build=@deepseek-ai/dsh-subprocess-local --allow-build=node-pty --allow-build=koffi dlx @deepseek-ai/dsh@0.1.2-rc.1 web
```

默认使用 npmmirror 国内镜像（镜像同步可能延迟）；自定义命令继承设置中的 HTTPS 仓库，显式 `--config.registry=…` 优先。pnpm 由本应用私有 Node 启动，dlx 缓存放在 `runtime/pnpm-cache`；不要求全局安装 pnpm。启动等待可设 1–120 分钟，默认 60 分钟。六插件仍在首次安装时解析所选仓库的 latest，安装回执存在时保持已安装版本。

**首次安装即最新版**：全新数据目录首次启动时，自动从仓库解析六个预置插件的 `latest` 并安装，不需要设置开关。实际版本记录到 `runtime/receipt.json`，普通重启和升级外壳会复用已有安装，不会静默升级插件。首次安装需要联网；解析失败会显示错误，重试即可，不会悄悄回退到写死的旧版本。自定义命令中的 DSH 版本与这六个插件版本是两回事。

浏览器必须完成 DSH 官方认证。用“在浏览器中打开”或“复制认证链接”完成首次登录；不需要从日志里找 token。**链接是完整访问凭证，请勿公开分享。** v0.2.1 新安装默认开启可信私有局域网分享，可在设置中关闭；二维码使用实际启用的私有 IPv4 地址，仍由 DSH 校验认证与 Host。部分 Android 浏览器的扫码入口会先在预览上下文处理重定向，导致认证 Cookie 没有进入最终页签；局域网二维码因此先打开 DSH Tiny 同源确认页，用户点击“确认并进入工作空间”后才在当前浏览器完成认证。“复制认证链接”继续使用原始直达链接。Windows 会优先选择默认路由网卡而不是 WSL/Hyper-V 等虚拟接口；特殊多网卡环境可在“局域网绑定 IPv4”填写准确的私有地址。未找到私有 IPv4 时继续仅本机访问，并写入日志，不阻止启动。HTTP 局域网流量未加密，不要用于公共 Wi-Fi、公网或不可信网络；操作系统防火墙/本地网络权限可能需要用户授权。已有配置的局域网开关保持原选择。

### 反向代理与公网访问

Tiny 支持精确的受信任反向代理主机：ASCII 域名、IPv4、方括号 IPv6，以及它们的可选端口。协议、路径、通配符、用户信息和模糊匹配会被拒绝；最多 16 个。可信主机只影响 DSH 的 Host/Origin 安全边界，不会改变局域网监听地址。填写一个不含路径的 HTTPS“公网访问地址”后，其 authority 会自动加入可信列表；运行中可在概览复制公网认证连接或生成二维码。Tiny 只在用户明确点击时把当前内存 token 与公网地址组合，token 不写入设置、状态快照或日志。

建议为每台主机使用独立 Cloudflare Tunnel。一个 Windows Tunnel 可以配置多个 Public Hostname，每个服务使用独立子域名并回源不同的本地端口，例如 DSH 使用 `https://zgpc.example.com` → `http://127.0.0.1:3080`。DSH 当前没有 base-path 配置，不能用 `/dsh` 这类路径前缀部署。公网入口建议先启用 Cloudflare Access；首次通过 Access 后再打开 Tiny 生成的带 token 链接，浏览器得到 DSH 的 authority-bound Cookie，之后可直接访问裸域名。两层认证均保留。

## 独立数据与导入

默认根目录：

| 系统 | 目录 |
|---|---|
| macOS | `~/Library/Application Support/dsh-tiny-desktop` |
| Windows | `%AppData%/dsh-tiny-desktop` |
| Linux | `$XDG_CONFIG_HOME/dsh-tiny-desktop` 或 `~/.config/dsh-tiny-desktop` |

根目录下 `dsh/` 是独立 DSH_HOME，`runtime/` 是托管运行环境，`logs/runtime.log` 是脱敏日志（单文件 5 MiB 轮换，保留上一份）。测试可通过 `DSH_TINY_HOME` 指定另一个根目录。

导入前只需退出源 DSH；Tiny 自己的服务若正在运行，会由数据页自动安全停止，并在导入成功或失败后自动恢复，无需前往概览。设置选择原 `~/.dsh`，预览会分别显示可新增文件、按条目合并的配置/数据文件、同名冲突和跳过项，按需勾选凭据。导入范围包括会话、项目存储、附件、技能、设置、agent presets 和 task-board；不导入执行代码、profiles、缓存、锁。

**合并始终以 Tiny 当前数据为准**：普通文件同路径已存在时保留 Tiny；`settings.yaml` 递归补充 Tiny 缺少的配置键；`.credentials.yaml` 只补充 Tiny 缺少的凭据引用/记录，因此可导入来源独有的 `DEEPSEEK_API_KEY`，同名密钥仍保留 Tiny；`storages/*.json` 按 DSH 官方单元格式补充缺少的表记录，使来源项目进入 Tiny，同名项目与全局值仍保留 Tiny。逐记录存储目录继续按文件补充。撤销记录保存在 `backup-*`，同时保存本次修改文件的导入前版本，恢复时会移除新增项并还原被合并的配置；源目录始终只读。凭据内容不会进入日志或预览。

Windows 可读取的 OneDrive/Cloud Files 数据占位对象按普通数据复制；符号链接、junction、socket、管道、设备、ProjFS、未知 reparse point，以及无法检查或读取的非必要项会直接跳过。预览显示跳过数量、相对路径和可用的 reparse tag，最多列出 10 个示例，完整数量仍会统计。它们不会被复制，也不会阻断其他正常文件；只有缺失、不可读或类型无效的必要配置 `settings.yaml` 会终止导入，避免生成无法使用的数据空间。

## 六个预置插件

| 插件 | 首次安装 |
|---|---|
| @michengai/dsh-codex-ui | 自动解析 latest |
| @michengai/dsh-im-connect | 自动解析 latest |
| @michengai/dsh-automation | 自动解析 latest |
| dshmarket | 自动解析 latest |
| task-complete-notify-for-dsh | 自动解析 latest |
| dsh-better-sidebar | 自动解析 latest |

DSH **0.1.2-rc.1**，pnpm **10.28.0**，Wails **v3.0.0-beta.16**。首次初始化通过官方 `dsh plugin --profile web` 安装并注册；关闭 peer 自动安装，由宿主提供 DSH API；原生构建仅允许列出的依赖。运行环境显式传递独立 `DSH_PROFILE_DIR` 和 `DSH_RUNTIME_DIR`，插件自助更新后的校验不会误读全局 `~/.dsh`。插件是独立第三方代码，有自己的许可证与网络/通知行为。

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

不提供分屏。采用原生菜单/托盘加独立设置，不承诺与 DshShell 的私有实现或所有外观效果完全一致。应用更新入口打开官方项目 Releases，由用户校验并替换程序；不会静默覆盖正在运行的二进制。

本项目非 DeepSeek 或 DshShell 官方产品，不复制其品牌图标或源代码。原始 shell 代码采用 MIT；运行时与插件保留各自许可。
