# 验证记录

## v0.2.11 Profile、Windows 导入与一键重启

- 插件更新根因：Tiny 的 DSH 子进程已有独立 `DSH_HOME`，但未显式设置 `DSH_PROFILE_DIR`。`@michengai/dsh-codex-ui` 的 CLI fallback 用 `DSH_HOME` 调用 `dsh plugin --profile web add`，命令实际更新 Tiny Profile；其安装后校验却在变量缺失时回退到用户 `~/.dsh/profiles/web`。本机两个 manifest 的插件版本确实不同，稳定解释“命令结束但没有进入当前 Profile”。
- `TestInstallerEnvironmentPinsIndependentProfile` 先注入错误的继承 Profile/Runtime：修复前返回继承路径并失败；修复后确认环境中只有 Tiny 的 `dsh/profiles/web` 与 `runtime/dsh`。不会读取、写入或更新全局 DSH。
- `TestImportSkipsSpecialFileAndKeepsRegularData` 使用白名单 `sessions` 下的命名管道回放旧错误：修复前整次预览失败；修复后正常文件完成导入，管道未复制，预览报告 `sessions/runtime.pipe`。`TestImportSkipsSymlinks` 同样要求符号链接只计入跳过项。
- `TestImportPreviewBackupRestore` 验证补充合并：源端同名文件不能覆盖 Tiny，新文件可加入，恢复只移除本次新增项并保留 Tiny 原数据。`TestWindowsCloudReparsePolicyIsNarrow` 只允许 Cloud_0…F、OneDrive、File Placeholder、Storage Sync，并把 junction、symlink、AF_UNIX、ProjFS 和 AppExecLink tag 分类为跳过项。
- Windows 专用 `TestWindowsImportSkipsJunctionWithPathAndTag` 创建真实目录 junction，要求预览成功、跳过数量为 1，并同时报告相对路径与 `0xA0000003`；此项只以 Windows CI 结果为准。
- 前端静态回归要求概览存在“一键重启”，受信消息桥按固定顺序调用 `Stop`/`Start`。正式 Windows WebView2 QA 会点击该按钮，观察离开 Running 后再次达到已认证 Running；不会以手动 Stop/Start 代替。

## v0.2.10 最小化与关闭窗口语义

- `TestWindowLifecycleKeepsMinimiseNative` 直接检查 `main.go` 的真实事件绑定：修复前稳定报错 `minimise is wired to hideToTray`；移除该绑定后通过，并额外要求设置窗口与工作空间的两个关闭钩子仍保留 `hideToTray`。
- 根因是“仅驻留托盘”将 `WindowMinimise` 和 `WindowClosing` 都路由到 `hideToTray`，把两种桌面动作错误合并。现在最小化不注册额外处理，完全保留系统原生 Dock/任务栏行为；点 × 才根据原开关进入托盘。
- Windows 原生发布探针新增稳定状态检查：最小化后原生窗口必须持续可见且为 iconic，点关闭后必须隐藏但进程存活，再次显示必须成功。macOS `tray-cycle` QA 同步改为最小化保留 regular activation policy、关闭切换 accessory policy。
- 本机 macOS 原生 QA 在执行时检测到系统锁屏，按验证规则返回 `BLOCKED`，不将锁屏下的辅助功能结果记为通过。Swift 脚本已通过类型检查；跨平台正式结果以后续 CI 记录为准。
- 本地 `go test -count=1 ./...`、核心 race、`go vet ./...`、模块 diff、前端 4 项测试、TypeScript/前端构建和 Swift 类型检查全部通过。锁屏 QA 专用进程已终止，其独立设置、候选二进制及临时目录已删除，未启动 DSH 安装、未修改用户数据。
- `main` 工作流 [33954635098](https://github.com/zhangjiawei/dsh-tiny-desktop/actions/runs/33954635098) 的六个平台构建全部成功；其中 Windows x64 原生门禁任务 `101275593539`、Windows ARM64 任务 `101275593726` 均通过正式 EXE 的 WebView2 状态、图标、最小化、关闭隐藏、进程存活与恢复检查。
- 标签工作流 [33955235644](https://github.com/zhangjiawei/dsh-tiny-desktop/actions/runs/33955235644) 再次完成六个平台构建及发布任务；Windows x64 任务 `101277264966`、Windows ARM64 任务 `101277264855` 的同一原生门禁均成功。
- 从未登录的公开发布页重新下载 v0.2.10 的六个归档和 `SHA256SUMS.txt`：全部哈希通过；Windows 为 x86-64/AArch64 GUI PE，Linux 为 x86-64/AArch64 ELF，macOS 为 x86_64/arm64 Mach-O，两个 App 的 `CFBundleShortVersionString` 均为 `0.2.10` 且 `codesign --verify --deep --strict` 通过。下载、解压和本机 QA 临时目录均已删除。

## v0.2.9 Android Opera 扫码认证上下文

- 同一局域网、同一二维码在 Android Maxthon 正常，Opera 内置扫码入口返回 DSH authentication required；把“复制认证链接”直接粘贴到同一 Opera 地址栏则正常。这排除了二维码缺 token 及 HTTP 本身被浏览器拒绝，差异集中在扫码入口的重定向/Cookie 上下文。
- DSH 的根 token 请求会签发按 authority 绑定、`HttpOnly; SameSite=Strict` 的会话 Cookie，并以 303 跳转到无 token 的 `/`。最小回归模拟扫码预览先访问二维码、再由新浏览器上下文接手最终地址，旧实现稳定得到 HTTP 401。
- 局域网二维码改为 `/.dsh-tiny/share?token=…`：GET/HEAD 仅返回不可缓存的静态确认页且不访问 DSH；用户明确 POST 后才 303 到 DSH token 根地址。回归验证预览阶段 DSH 请求数为 0、页面正文不包含 token，并在当前浏览器 Cookie 容器内认证成功。
- 正常启动隔离检查：`Start`、Installer、DSH 子进程、`VerifyLaunchURL`、工作空间窗口、浏览器打开及复制认证链接仍走原路径；只有运行后主动点击“分享二维码”才调用 `QRShareURL`。LAN 代理原有普通 HTTP、Host 边界和 WebSocket 回归继续通过；未启用 LAN 时二维码仍返回原本机地址。
- v0.2.8 标签工作流 [`33950795124`](https://github.com/zhangjiawei/dsh-tiny-desktop/actions/runs/33950795124) 的 Windows x64 已完成正式 EXE 全新安装并进入 Running，32×32 大图标返回绿色 25、浅色 90 像素；16×16 `ICON_SMALL2` 返回绿色 5、浅色 0 像素。旧探针错误要求 16×16 也保留浅色微细节，因此拦截发布；v0.2.8 不重写标签、不生成 Release。修正后小图标仍必须包含至少 2% 的自定义绿色齿轮像素，大图标仍同时要求绿色和浅色细节，不会把中性系统占位图标放行。
- 本地 `go test -count=1 ./...`、核心 race/vet、模块 diff、前端测试、TypeScript 与打包通过。main 工作流 [`33951389953`](https://github.com/zhangjiawei/dsh-tiny-desktop/actions/runs/33951389953) 六平台全部成功；Windows x64 job `101266761199` 与 ARM64 job `101266761051` 均完成正式 EXE 全新安装、状态/图标像素检查及 WebView2 打开、QR、停止、退出端口释放全流程。
- 正式标签工作流 [`33952114222`](https://github.com/zhangjiawei/dsh-tiny-desktop/actions/runs/33952114222) 的六个 build 及 release job `101270407577` 全部成功，创建公开预览版 [`v0.2.9`](https://github.com/zhangjiawei/dsh-tiny-desktop/releases/tag/v0.2.9)。Windows x64 job `101268740735` 与 ARM64 job `101268740682` 再次通过；x64 图标统计为 small/small2 `49/195/256`、large `25/90/1024`，ARM64 为 small/small2 `49/195/256`、large `229/724/1024`（绿色/浅色/总像素）。
- 发布后通过未登录的公开 Release 页与直链重新下载六个归档和 `SHA256SUMS.txt`，六项 SHA-256 全部匹配。解包确认 Mach-O x86_64/arm64、PE GUI x86-64/Aarch64、ELF x86-64/aarch64；两个 Mac 包版本均为 0.2.9，且通过 `codesign --verify --deep --strict`。macOS 仍是 ad-hoc 签名，Windows 仍未商业签名。
- 复核后删除本任务创建的 v0.2.8 失败日志响应、v0.2.9 公开页临时副本、六包下载及解包目录，合计约 95 MiB；逐项确认不存在。未清理工程源码、正式 Release、用户 DSH 数据或共享缓存。

## v0.2.7 Windows Node 目录发布恢复与启动闪屏

- 用户日志在 Node 24.20.0 解压完成后的同卷目录重命名阶段返回 Windows `Access is denied`。原实现仅执行一次 `os.Rename`，既不重试短暂文件占用，也不能处理系统重启留下的同名半成品目标目录。
- `TestNodePublishRetriesTransientWindowsAccessDenied` 注入前两次拒绝访问，确认第三次重试发布成功；`TestNodePublishReplacesIncompleteTarget` 验证同名半成品先隔离、新运行时就位后再清理；`TestNodePublishRestoresIncompleteTargetWhenReplacementFails` 验证连续失败时恢复旧目录，而不是丢失现场。
- 正式 EXE 已由打包脚本使用 `-H windowsgui`，长期 DSH 及安装子进程也已有 `HideWindow`，因此用户在已安装全局 Node 的 Windows 上双击时出现的短暂控制台，定位为未套用隐藏策略的 `node --version` 系统运行时探测。该探测与 PowerShell 语言探测现统一经过 `backgroundCommandContext`；Windows 专属回归直接检查 `HideWindow` 与进程组标志。
- v0.2.6 候选标签的 Windows x64 已成功完成全新安装、恢复、自定义 pnpm/PTY、正式打包并认证就绪，随后图标探针因 `ICON_SMALL=0`、`ICON_SMALL2` 与 large 均为有效自定义图标而误报。微软文档允许 `WM_GETICON` 返回 0 并要求使用后续回退；探针改为要求至少一个小图标槽存在、验证所有实际返回的槽和大图标。v0.2.6 未生成 Release，且不重写旧标签。
- 修正图标探针后的 Windows x64 已通过正式 EXE 的状态与图标检查，随后 QA 设置保存真实返回 `Access is denied`。该路径同样是临时文件到正式设置文件的 Windows 原子重命名；`replaceFileWithRetry` 注入两次拒绝访问后成功的回归，保证旧设置不被原地截断，并覆盖 Defender/索引器的短暂 delete-sharing 占用。
- 本地使用官方 Go 1.25.0 临时工具链通过 `go test -count=1 ./...`、核心 race/vet、模块 diff、前端测试/构建以及 Windows x64 GUI 交叉编译；编译结果由 `file` 确认为 PE32+ GUI 子系统。
- 最终 main 工作流 [`33947477853`](https://github.com/zhangjiawei/dsh-tiny-desktop/actions/runs/33947477853) 六平台全部成功。Windows x64 与 Windows ARM64 都完成真实首次安装、中断安装恢复、局域网认证、自定义 pnpm dlx/PTY、正式打包和原生 WebView2 控制桥；这证明修复同时覆盖用户报告的平台和另一 Windows 架构。
- 正式标签工作流 [`33948082437`](https://github.com/zhangjiawei/dsh-tiny-desktop/actions/runs/33948082437) 的六个平台 build 与 release job `101259120892` 全部成功，创建公开预览版 [`v0.2.7`](https://github.com/zhangjiawei/dsh-tiny-desktop/releases/tag/v0.2.7)。其中 Windows x64 job `101257676912`、Windows ARM64 job `101257676787` 均通过；v0.2.6 仍只保留失败候选标签，没有 Release，也未被重写。
- 发布后通过公开 Release API 重新下载六个归档和 `SHA256SUMS.txt`；六个文件大小与发布元数据一致，SHA-256 全部匹配。解包确认 Mach-O x86_64/arm64、PE GUI x86-64/Aarch64、ELF x86-64/aarch64；两个 Mac 包的版本元数据均为 0.2.7，并通过 `codesign --verify --deep --strict`。macOS 仍是 ad-hoc 签名，Windows 仍未商业签名。
- 发布验证结束后，仅清理本次任务明确创建的临时 Go 工具链、Windows 交叉编译文件、失败日志响应头、公开发布包与解压目录；工程源码、正式发布、用户 DSH 数据和共享缓存均保留。

## v0.2.5 Windows GUI stdin、图标与退出界面

- 用户提供的第二台 Windows 日志显示六插件已安装完成，随后默认 `pnpm dlx` 在 `@deepseek-ai/dsh-subprocess-local` 生命周期阶段由 pnpm 10.28.0 抛出 `Error: readStream must be readable`；进程为应用私有 pnpm/缓存，末行 Node 为系统 24.19.0，全局 dsh 没有出现在调用链中。
- `TestSystemNodeMustMatchPinnedRuntime` 在修复前因缺少严格版本边界失败；修复后只接受 24.20.0。其他版本使用下载、SHA-256 校验后的私有 Node。`TestManagedStdinStaysOpenUntilCleanup` 验证匿名 stdin 在子进程运行期保持可读且仅在清理时 EOF；安装器与长期 DSH 进程均使用该边界，不继承用户输入。后续正式 GUI 脱敏日志进一步证明异常来自 pnpm 的生命周期子进程 stdout 竞态，并非顶层 stdin：默认命令在 Windows 复用 Ensure 已按同版本安装和重建的私有 DSH；`TestWindowsDefaultDLXUsesInstalledPrivateDSH` 锁定只改写精确默认命令和可选私有 trusted host，其他系统及自定义版本不改写。
- Windows 窗口图标不再从单一 256 px ICO 经 `LoadImageW` 缩放，而是由 Wails/w32 按系统小/大图标指标从源 PNG 创建 HICON。原生 UIA 脚本新增像素检查，要求设置窗口的 16/32 图标同时包含浅底与绿色齿轮特征，不再把任意非零默认句柄记为通过。
- 退出区采用和导航一致的按钮结构；确认弹窗使用 alertdialog 语义、42 px 警示图标、分隔操作栏及等宽按钮。本地 1000×800、820×680 隔离浏览器检查无横向溢出，后者弹窗 440×189 px，两按钮均 88×36 px；该浏览器预览只证明布局，不替代 Windows 原生 GUI。
- 本地 Go 全量测试、核心 race/vet、模块 diff、前端测试/构建和 Windows amd64 GUI 交叉编译通过。最终六平台与 Windows 原生首次安装结果以下方发布工作流为准。
- Mac 全新隔离目录在系统 Node 26.7.0 下明确下载并校验私有 Node 24.20.0，完成当次最新六插件、默认 pnpm dlx、认证就绪、三处真实 PTY 和自有进程停止；测试目录约 1.3 GiB、交叉编译 exe 及浏览器截图均在记录结果后删除。
- 用户第二份日志还显示 Windows 误选 `172.29.112.1`（WSL/Hyper-V 虚拟网卡），实际物理局域网地址为 `172.16.8.136`。选择器回归先以虚拟网卡排在首位稳定复现旧行为，再改为优先默认路由源地址、回退时降低虚拟/隧道接口优先级。`--trusted-host` 现仅允许单个私有 IPv4；显式配置时用于绑定和 DSH 信任，未配置时由外壳注入自动选择的地址。
- 首轮 main 工作流 `33938094155` 的 Mac/Linux 四平台通过；Windows x64 普通首装、旧锁恢复、自定义 pnpm dlx 和打包通过，但新增正式 GUI 用例在 15 分钟后只报告“Running 或齿轮未满足”的合并超时，证据不足以区分安装与图标。后续探针分离阶段状态和三种图标句柄/像素统计，并在进入 Needs attention 或 Running 后图标异常时快速失败；该轮不发布。
- 诊断工作流 `33939650227` 的 Windows x64 脱敏日志复现到私有 Node 24.20.0 与 pnpm 10.28.0：六插件成功后，默认 dlx 安装的 `@deepseek-ai/dsh-subprocess-local` postinstall 已启动并立刻在 pnpm `byline(proc.stdout)` 抛出 `readStream must be readable`。这排除了全局 dsh、旧 Node、图标探针和六插件安装；该轮仅用于证据，不发布。
- 修复后工作流 `33940496161` 的正式 Windows EXE 已完成全新私有安装并进入 Running；原生窗口返回的 small/small2 均为 49 个绿色、195 个浅色像素（256 总像素），证明标题栏及高 DPI fallback 已使用齿轮图。large 为 25 个绿色、90 个浅色像素（1024 总像素），旧探针因要求 25% 不透明浅色背景而误判透明留白设计；门槛改为同时检查至少 2% 绿色齿轮与 8% 浅色细节，仍能排除系统占位图和纯色色块。
- 最终 main 工作流 [`33940800573`](https://github.com/zhangjiawei/dsh-tiny-desktop/actions/runs/33940800573) 六平台全部通过：Windows x64 job `101237785582` 与 Windows ARM64 job `101237785598` 均通过真实首次安装、旧锁恢复、局域网认证、自定义 dlx/PTY、正式打包及原生 WebView2 控制桥；其余 macOS Intel/ARM64、Linux x64/ARM64 四个平台也全部成功。main 事件的 release job 按设计跳过，正式发布只由版本标签触发。
- 正式标签工作流 [`33941668026`](https://github.com/zhangjiawei/dsh-tiny-desktop/actions/runs/33941668026) 的六平台 build 与 release job `101241925814` 全部成功，创建公开预览版 [`v0.2.5`](https://github.com/zhangjiawei/dsh-tiny-desktop/releases/tag/v0.2.5)。
- 发布后从公开 Release API 重新下载 macOS Intel/ARM64、Windows x64/ARM64、Linux x64/ARM64 六个包；每个下载尺寸均与 API 一致，SHA-256 均与 `SHA256SUMS.txt` 一致。校验目录在记录结果后删除。

## v0.2.4 安装恢复验证

- 用户 Windows 日志在独立 profile 初始化阶段报 `ERR_PNPM_OUTDATED_LOCKFILE`，缺少六个依赖 specifier。使用独立临时目录保留旧 lock、删除 manifest 的六依赖，并保留相同 pnpm workspace 设置，官方 CLI 稳定复现完全相同的错误。仅添加 `--no-frozen-lockfile` 后同一夹具安装成功。没有改动全局 DSH、全局 pnpm 或原用户数据。
- 生产安装器保留 `CI=true` 非交互行为，仅在私有 profile 初始化命令上显式允许锁文件协调。新 `install-recovery-smoke.mjs` 先用官方 CLI 验证旧命令确实报同一错误，再走真实 Manager/Installer 完整恢复。Mac 本地恢复后在 3080 认证就绪，六插件、真实 PTY、进程停止和会话哨兵内容保持均通过。
- 安装回执中的 Registry 改为来源记录，不再作为重装条件。更换到不可访问仓库时仍复用已安装版本的回归，修改前失败、修改后通过。
- v0.2.3 标签工作流 `33895811979` 已取消，未发布；不重写旧标签。新增恢复测试进入六平台发布关卡，最终结果见下节。

### 最终发布验证

[v0.2.4 工作流](https://github.com/zhangjiawei/dsh-tiny-desktop/actions/runs/33896340520) 的六平台和 release job 均已成功。Windows x64 job `101099752366`、ARM64 job `101099752315` 均通过：真实旧锁错误复现及完整恢复、六插件/PTY、会话哨兵保留、正式 exe UIA 齿轮窗口、连续六次 hash 导航保存、真实 3080 占用提示、待应用设置、打开/QR，以及取消退出和确认退出后的端口释放。两个 macOS、两个 Linux 也完成同一安装恢复关卡；Mac 界面操作证据沿用下方同功能源码的原生 QA 记录，不把 Linux 编译/启动测试写成原生 GUI 实测。

公开 [v0.2.4 Release](https://github.com/zhangjiawei/dsh-tiny-desktop/releases/tag/v0.2.4) 的六个归档及校验文件由 `node scripts/verify-release.mjs v0.2.4` 重新下载，大小和全部 SHA-256 匹配。解压确认 Mach-O x86_64/arm64、PE GUI x86-64/Aarch64、ELF x86-64/aarch64；两个 Mac 包的版本元数据均为 0.2.4，均通过 `codesign --verify --deep --strict`。仍是 ad-hoc 签名，不代表 Apple 公证；Windows 未商业签名。

### 本机善后清理

- 按用户要求永久删除 15 处已核实归属本任务的临时资源，包括隔离安装目录、候选应用、下载/解压的发布包、测试输出和专属 QA WebKit 缓存/Cookie。目录清单合计约 6.104 GiB；清理前后文件系统可用空间实测增加约 3.499 GiB，两者因共享块等文件系统计量差异不等同。
- 清理前发现旧 smoke 遗留 Node 进程 PID 36450，核对启动时间、专属临时目录与监听端口 59800 后，仅向该进程发送 SIGTERM。随后确认进程退出、端口释放，再删除对应目录；没有批量终止 Node 或 DSH。
- 15 处目标均确认不存在，未发现指向测试资源的启动项。工程源码、用户交付输出、正式应用、原 DSH 数据和共享 Node/npm/Go 缓存保留；测试缓存与构建产物可由已提交脚本重新生成，未清理用户正式数据。

## v0.2.3 导航时序修正

- v0.2.2 标签构建的 Windows x64 job `101094974237` 在 `default mirror saved` 等待处超时；此前真实 DSH/六插件/PTY 和正式 exe UIA 全部通过。发布被关卡阻止，未发布 v0.2.2 安装包；不移动或覆盖该标签。
- 定位到 Windows 校验使用 `topOrigin == origin`，而 Wails beta.16 分别从消息事件和 WebView 当前状态读取两个 URL。`TestWindowsHashNavigationInFlight` 构造同一页面导航前后不同 hash，在旧实现稳定失败；改为只去除 fragment 比较后通过。两侧仍独立验证可信来源，不同协议/路径/查询/站点保持拒绝。原始 CI 日志没有保存两侧来源 URL，无法仅凭该日志证明本次超时必然由此造成；因此增加真实桥接压力回归继续验证。
- Windows 原生测试新增连续六次“提交保存后立即切换 hash”，每次等待不同端口确实写入设置文件；超时仅记录路由、界面状态、通知和无效字段 ID，不记录认证链接、二维码或用户日志。
- 本地回归、race/vet、前端测试与构建通过。此修正最终在 v0.2.4 的两个 Windows 原生压力回归中通过；v0.2.3 已取消，未单独发布。

## v0.2.2 设置候选验证（未发布）

- 新增回归覆盖默认仓库/启动命令、旧版空命令兼容、私有 CLI 路由、真实端口占用及活动配置与待应用配置的区分。核心测试、race/vet、前端测试和 TypeScript 构建通过；Windows 桌面包本地交叉编译通过，不等同于 Windows 原生执行。
- 视觉预览在 1000×800、820×680 检查齿轮标识、侧栏退出按钮、长命令/限制、仓库默认值、端口提示和英文确认退出弹窗；无横向溢出。预览使用隔离模拟 bridge，仅用于布局，不计作原生功能通过。
- Mac 原生 QA 使用独立临时目录，将 3080 用测试监听器占住，以默认 pnpm 命令及 npmmirror 实际安装/启动：首次在 61674 认证成功，原生界面冲突提示及取消退出回归通过。最终候选构建再次在 61790 就绪，AX 验证提示包含真实端口；点击“退出应用”→“取消”仍为运行中，再点击“确认退出”后 App 正常以 0 退出，重新绑定 61790 成功，确认后台已释放端口。测试监听器已关闭，未影响其他服务。
- Windows CI 新增真实占用 3080 后验证随机服务端口/提示、恢复并使用默认命令及镜像、待应用设置不篡改活动端口提示、取消退出保持服务、确认退出并释放服务端口；正式 exe 另由 UIA 验证 Settings 标题与原生大小图标句柄。
- 六平台首装 smoke 改为实际生产默认 pnpm 命令及国内镜像。[候选工作流](https://github.com/zhangjiawei/dsh-tiny-desktop/actions/runs/33893779512) 的六平台构建、真实 DSH/六插件/PTY 全部通过。Windows x64 job `101091492938`、ARM64 job `101091493116` 的正式 exe UIA 和全部原生 WebView2 用例通过；ARM64 首次安装耗时约 7 分钟，日志持续推进，后续缓存启动正常。v0.2.2 标签轮次的保存超时另见上方记录，该标签未发布。

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
