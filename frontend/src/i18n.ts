// Keep translation at the shell boundary. Never rewrite or inject the DSH DOM.
export let language = "en";
const en: Record<string, string> = {
  设置: "Settings",
  退出应用: "Quit app",
  确认退出: "Confirm quit",
  "退出 DSH Tiny？": "Quit DSH Tiny?",
  "退出将停止 DSH 服务，正在运行的任务可能中断。关闭窗口则可继续在托盘运行。": "Quitting stops DSH and may interrupt running tasks. Close the window to keep it running in the tray.",
  取消: "Cancel",
  "正在退出应用…": "Quitting app…",
  恢复默认命令: "Reset command",
  恢复默认仓库: "Reset registry",
  启动程序及参数: "Program and arguments",
  "支持带引号的路径和参数；不支持 Shell 管道、变量、重定向或多条命令。端口、监听地址和认证由应用管理；局域网分享可选填 --trusted-host 私有 IPv4，未填写则使用默认路由网卡地址。请勿填写 --port、--host、--token 或关闭认证的参数。pnpm 使用应用内置环境，仅允许示例中列出的必要安装脚本。": "Quoted paths and arguments are supported; shell pipes, variables, redirects and multiple commands are not. The app manages ports, listen addresses and authentication. For LAN sharing, --trusted-host may specify one private IPv4 address; otherwise the default-route adapter is used. Do not add --port, --host, --token or disable authentication. pnpm uses the app's private runtime and permits only the necessary build scripts listed in the default command.",
  "端口占用自动检测；被占用时自动选择随机可用端口。": "Automatically detect occupied ports and select a random available port when needed.",
  "默认使用 npmmirror 国内镜像。启动命令继承此仓库；命令中显式指定的 registry 优先。镜像同步可能延迟，可按需切换仓库。": "Defaults to the China-friendly npmmirror registry. Launch commands inherit this registry unless explicitly overridden. Mirrors may lag behind upstream; change the registry when needed.",
  "本地运行 · 数据独立": "Local · Isolated data",
  "管理你的本地 DSH 工作空间。": "Manage your local DSH workspace.",
  "首次运行自动安装独立环境与 6 个最新版插件，不影响原有 DshShell。":
    "First launch installs an isolated runtime and the latest six plugins. Your existing DshShell is untouched.",
  快捷操作: "Quick actions",
  预置插件: "Included plugins",
  "6 个插件": "6 plugins",
  编码工作台: "Coding workspace",
  即时通讯: "Messaging",
  自动化任务: "Scheduled tasks",
  插件市场: "Plugin marketplace",
  桌面提醒: "Desktop alerts",
  侧栏增强: "Sidebar enhancements",
  "首次安装自动获取最新版，之后重启保持已安装版本。":
    "Latest versions on first install. Subsequent launches keep installed versions.",
  "外观立即生效，启动配置应用并重启后生效。":
    "Appearance applies immediately. Apply and restart for launch settings.",
  外观与窗口: "Appearance & windows",
  "跟随系统或手动选择，仅影响桌面外壳。":
    "Follow your system or choose a language for the desktop shell.",
  仅驻留托盘: "Tray-only mode",
  "点 × 关闭时隐藏 Dock / 任务栏图标；最小化仍保留。":
    "Hide from the Dock / taskbar when closed with ×; minimizing keeps the native entry.",
  关闭窗口后继续运行: "Keep running after closing",
  "关闭工作空间窗口时不停止 DSH 服务。":
    "Closing the workspace window keeps DSH running.",
  工作空间置顶: "Always on top",
  "DSH 窗口始终显示在其他窗口上方。":
    "Keep the DSH workspace above other windows.",
  启动与网络: "Launch & network",
  "自动启动 DSH": "Start DSH automatically",
  "打开应用时自动准备并启动工作空间。":
    "Prepare and start the workspace when the app opens.",
  "支持带引号的参数，不执行 Shell 管道或变量。端口与认证由外壳管理。":
    "Quoted arguments are supported, not shell pipes or variables. The app manages the port and authentication.",
  "允许 1–120 分钟准备依赖。": "Allow 1–120 minutes to prepare dependencies.",
  "支持 HTTPS 镜像，用于运行环境和插件安装。":
    "HTTPS registry for runtime and plugin installation.",
  "仅影响本应用，不修改系统代理。":
    "Applies only to this app, not the system proxy.",
  局域网分享: "LAN sharing",
  "仅限可信私有网络。链接具有完整权限，HTTP 未加密。":
    "Trusted private networks only. Links grant full access; HTTP is unencrypted.",
  "已有工作空间的插件不会随重启自动升级。":
    "Existing workspace plugins never update silently on restart.",
  "从原 DSH 补充数据，Tiny 已有内容始终优先。":
    "Add data from an existing DSH installation. Existing Tiny data always wins.",
  当前数据空间: "Current data space",
  "请先退出原 DSH，确保导入时源数据不再变化。":
    "Exit the original DSH before importing so the source data stays unchanged.",
  "最近 500 行输出，认证链接和常见敏感字段自动脱敏。":
    "Last 500 log lines. Authenticated links and common secrets are redacted.",
  "填入 pnpm dlx 示例": "Use pnpm dlx example",
  "示例仅允许 DSH subprocess、node-pty、koffi 的必要安装脚本；不要使用全部放行选项。":
    "The example allows only the required DSH subprocess, node-pty and koffi scripts. Never approve all dependency scripts.",
  概览: "Overview",
  运行设置: "Settings",
  数据迁移: "Import data",
  运行日志: "Logs",
  "小巧外壳，完整能力。": "Tiny shell. Full capability.",
  "让工作，轻装上阵。": "Less shell. More work.",
  "DSH 的完整体验。一个安静、独立、随时可控的桌面入口。":
    "The full DSH experience. A quiet, isolated desktop home that stays in your control.",
  "连接设置…": "Connecting to settings…",
  "你的工作空间正在准备。": "Preparing your workspace.",
  "首次运行会安装独立环境和默认插件。已有的 DshShell 不会被修改。":
    "The first launch installs an isolated runtime and default plugins. Your existing DshShell is untouched.",
  "启动工作空间 ↗": "Start workspace ↗",
  打开窗口: "Open window",
  "↻ 一键重启": "↻ Restart",
  停止服务: "Stop service",
  "↗ 在浏览器中打开": "↗ Open in browser",
  "⧉ 复制认证链接": "⧉ Copy authenticated link",
  "▦ 分享二维码": "▦ Share QR code",
  "↓ 检查更新": "↓ Check for updates",
  开箱即用: "Ready to use",
  "6 个默认插件": "6 default plugins",
  任务完成通知: "Task notifications",
  "保持简单，也留有余地。": "Simple. With room to adapt.",
  "外观设置立即生效；运行设置可保存或应用并重启。":
    "Appearance changes apply immediately. Save runtime settings while stopped, or apply and restart.",
  语言: "Language",
  跟随系统: "Follow system",
  简体中文: "简体中文",
  "默认跟随系统语言。仅控制桌面外壳，DSH 工作空间语言由其自身设置管理。":
    "Follows the system by default. This controls the desktop shell; the DSH workspace has its own language settings.",
  "关闭时从 Dock / 任务栏隐藏":
    "Hide from Dock / taskbar when closed",
  "开启后仅保留菜单栏 / 系统托盘图标。单击恢复窗口，右键打开菜单；不会停止 DSH。":
    "Keep only the menu bar / system tray icon. Click to restore; right-click for the menu. DSH keeps running.",
  自定义启动命令: "Custom launch command",
  "留空使用内置托管启动。支持带引号的参数，不支持 Shell 管道或变量。端口、--no-open 和认证由外壳管理。":
    "Leave blank for the managed runtime. Quoted arguments are supported, not shell pipes or variables. The shell manages the port, --no-open and authentication.",
  "npm / pnpm 仓库": "npm / pnpm registry",
  "用于运行环境包和插件安装，可配置 HTTPS 镜像；自定义命令中的仓库参数优先。":
    "HTTPS registry for runtime packages and plugins. Registry arguments in a custom command take precedence.",
  "启动等待时间（分钟）": "Startup timeout (minutes)",
  "用于等待自定义命令下载依赖并启动服务，范围 1–120 分钟。":
    "Allow 1–120 minutes for a custom command to download dependencies and start the server.",
  首选端口: "Preferred port",
  "端口被占用时自动选择空闲端口，不会终止其他应用。":
    "Automatically selects a free port when occupied. Other applications are never terminated.",
  "HTTP(S) 代理": "HTTP(S) proxy",
  "仅用于本应用的安装和 DSH 子进程；不修改系统代理。":
    "Used only for this app's installation and DSH child process, not the system proxy.",
  "打开应用后自动启动 DSH": "Start DSH when the app opens",
  关闭窗口时隐藏到托盘: "Keep running when the workspace window closes",
  "DSH 窗口始终置顶": "Keep the DSH window on top",
  "启用局域网分享（链接持有者拥有完整访问权限）":
    "Enable LAN sharing (link holders have full access)",
  "默认关闭。仅在可信私有网络使用；HTTP 流量未加密，不适合公共 Wi-Fi 或公网。":
    "Off by default. Trusted private networks only: HTTP traffic is unencrypted. Not for public Wi-Fi or the internet.",
  保存设置: "Save settings",
  应用并重启: "Apply and restart",
  "迁入，而不是覆盖。": "Move in. Never overwrite.",
  "只读原目录，复制到本应用的独立数据空间。请先退出原 DSH，确保源数据不再变化。":
    "Copy the source read-only into an isolated data space. Exit the original DSH first so its data stops changing.",
  当前独立数据目录: "Current isolated data directory",
  "导入会补充会话、附件、技能和设置；同名路径保留 Tiny 版本，不复制插件代码、缓存或运行锁。不支持项会跳过并显示原因。":
    "Add sessions, attachments, skills and settings. Existing Tiny paths win; plugin code, caches and locks are excluded. Unsupported items are skipped with a reason.",
  "导入会补充会话、项目、附件、技能、设置和可选凭据；配置文件按条目合并，Tiny 已有值始终优先。不复制插件代码或运行锁，不支持项会跳过并显示原因。":
    "Import sessions, projects, attachments, skills, settings, and optional credentials. Config records are merged while existing Tiny values always win. Plugin code and runtime locks are excluded; unsupported items are skipped.",
  "请先退出原 DSH，确保来源数据不再变化。Tiny 服务若正在运行，会自动安全停止，并在导入完成或失败后自动恢复，无需前往概览操作。":
    "Exit the original DSH so source data stops changing. If Tiny is running, it stops safely and resumes automatically after success or failure; no Overview step is required.",
  "正在安全停止服务并合并数据，完成后会自动恢复…":
    "Safely stopping the service and merging data; it will resume automatically…",
  "合并完成，DSH 已自动重新启动。": "Merge complete. DSH restarted automatically.",
  "正在恢复导入前的数据，完成后会自动恢复服务…":
    "Restoring pre-import data; the service will resume automatically…",
  "同时复制凭据（API Key 等敏感配置）":
    "Also copy credentials (API keys and other secrets)",
  "选择原 DSH 数据目录…": "Choose original DSH data directory…",
  确认导入: "Confirm import",
  恢复此次导入前的备份: "Restore the backup from before this import",
  "状态，始终看得见。": "Always know what's happening.",
  "最近 500 行运行输出，认证链接和常见敏感字段自动脱敏。":
    "The latest 500 log lines. Authenticated links and common secret fields are redacted.",
  "同一台电脑，换个浏览器。": "Same computer. Another browser.",
  "在可信局域网内继续。": "Continue on a trusted private network.",
  "此链接包含完整访问凭证，请勿公开。当前为仅本机地址，手机无法访问。":
    "This link grants full access. Do not publish it. The local-only address cannot be opened on a phone.",
  "此链接包含完整访问权限。仅限可信私有网络；HTTP 未加密，请勿公开或转发给不可信的人。":
    "This link grants full access. Trusted private networks only; HTTP is unencrypted. Never publish or share with untrusted people.",
  "扫码后，在打开的页面点击“进入工作空间”完成认证。":
    "After scanning, tap Continue in the opened page to complete authentication.",
  复制认证链接: "Copy authenticated link",
  关闭: "Close",
  已停止: "Stopped",
  准备运行环境: "Preparing runtime",
  启动与认证中: "Starting and authenticating",
  运行中: "Running",
  需要处理: "Needs attention",
  "一切就绪，开始你的下一步。": "Ready for your next move.",
  "遇到了一点问题。": "Something needs your attention.",
  "你的工作空间，随时待命。": "Your workspace is on standby.",
  "首次准备，需要一点时间。": "First-time setup takes a moment.",
  "操作仍在处理中，请查看日志":
    "The operation is still running. Check the logs.",
  "认证链接已复制，请勿公开分享":
    "Authenticated link copied. Do not share publicly.",
  "已打开本项目的 GitHub Releases": "Opened this project's GitHub Releases.",
  设置已保存: "Settings saved.",
  "保存不会中断服务，启动参数在下次启动时生效。": "Saving keeps your service running. Launch changes take effect on the next start.",
  "已保存，启动参数有待应用。下次启动生效，或点击“应用并重启”。": "Saved changes are pending. They apply on the next start, or choose Apply & restart.",
  "设置已保存，下次启动生效；当前服务未中断。": "Settings saved for the next start. Your current service was not interrupted.",
  "设置连接失败，正在重试。请退出并重新打开应用，仍失败请更新版本。": "Settings connection failed. Retrying. Quit and reopen the app; update it if the problem persists.",
  连接失败: "Connection failed",
  "选项已改变，请重新选择目录以预览。":
    "Options changed. Choose the directory again to preview.",
  "合并完成。Tiny 同名数据保持不变。":
    "Merge complete. Existing Tiny data was kept.",
  已恢复备份: "Backup restored.",
  已打开工作空间: "Workspace opened.",
  已在默认浏览器中打开: "Opened in the default browser.",
  正在应用设置并重启: "Applying settings and restarting…",
  "正在重新启动 DSH…": "Restarting DSH…",
};
export function t(text: string) {
  return language === "en" ? en[text] || text : text;
}
const originals = new WeakMap<Node, string>();
let appliedLanguage = "";
export function setLanguage(choice: string, system: string) {
  language = (choice === "system" ? system || navigator.language : choice)
    .toLowerCase()
    .startsWith("zh")
    ? "zh"
    : "en";
  // Rewriting option text every status tick can dismiss native WebKit pickers
  // and invalidate accessibility nodes while the user is interacting with them.
  if (appliedLanguage === language) return;
  appliedLanguage = language;
  document.documentElement.lang = language === "zh" ? "zh-CN" : "en";
  document.title =
    language === "zh" ? "DSH Tiny · 设置" : "DSH Tiny · Settings";
  const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
  while (walker.nextNode()) {
    const node = walker.currentNode;
    if (
      node.parentElement?.closest(
        "#phase,#headline,#error,#log-output,#notice,#preview,#data-path,#port,#share-title,#share-warning",
      )
    )
      continue;
    if (!originals.has(node))
      originals.set(node, (node.textContent || "").replace(/\s+/g, " ").trim());
    const original = originals.get(node)!;
    if (en[original]) node.textContent = t(original);
  }
  document.getElementById("close-share")?.setAttribute("aria-label", t("关闭"));
  document
    .getElementById("command")
    ?.setAttribute("placeholder", t("启动程序及参数"));
}
