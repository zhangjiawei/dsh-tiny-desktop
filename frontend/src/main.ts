// All data from the process is rendered as text, never HTML. The desktop bridge
// is request/response only; the DSH window has no privileged Go bindings.
// @ts-ignore provided by the embedded Wails asset server
import { System } from "/wails/runtime.js";
// @ts-ignore QRCode is a vetted, pinned browser dependency
import QRCode from "qrcode";
type Settings = {
  port: number;
  proxy: string;
  lan: boolean;
  hideOnClose: boolean;
  autoStart: boolean;
  alwaysOnTop: boolean;
  language: string;
  width: number;
  height: number;
};
type State = {
  phase: string;
  error: string;
  port: number;
  data: string;
  logs: { time: string; text: string }[];
  settings: Settings;
};
const $ = <T extends HTMLElement = HTMLElement>(id: string) =>
  document.getElementById(id) as T;
const pending = new Map<
  number,
  { resolve: (v: any) => void; reject: (e: Error) => void }
>();
let seq = 0;
let state: State | undefined;
let importedBackup = "";
let source = "";
let first = true;
(window as any).tinyReply = (id: number, value: any, error: string) => {
  const p = pending.get(id);
  if (!p) return;
  pending.delete(id);
  error ? p.reject(new Error(error)) : p.resolve(value);
};
function call(action: string, data: unknown = {}): Promise<any> {
  return new Promise((resolve, reject) => {
    const id = ++seq;
    pending.set(id, { resolve, reject });
    System.invoke(JSON.stringify({ id, action, data }));
    setTimeout(() => {
      if (pending.delete(id)) reject(new Error("操作仍在处理中，请查看日志"));
    }, 120000);
  });
}
let noticeTimer: ReturnType<typeof setTimeout>;
function notice(message: string) {
  $("notice").textContent = message;
  $("notice").style.display = "block";
  clearTimeout(noticeTimer);
  noticeTimer = setTimeout(() => ($("notice").style.display = "none"), 6000);
}
function action(id: string, fn: () => Promise<unknown>) {
  $(id).onclick = () => {
    fn().catch((e) => notice(e.message));
  };
}
const labels: Record<string, string> = {
  stopped: "已停止",
  installing: "准备运行环境",
  starting: "启动与认证中",
  running: "运行中",
  error: "需要处理",
};
function render(s: State) {
  state = s;
  $("phase").textContent = labels[s.phase] || s.phase;
  $("phase").className = "badge " + s.phase;
  $("port").textContent = `127.0.0.1 : ${s.port || "—"}`;
  $("headline").textContent =
    s.phase === "running"
      ? "一切就绪，开始你的下一步。"
      : s.phase === "error"
        ? "遇到了一点问题。"
        : s.phase === "stopped"
          ? "你的工作空间，随时待命。"
          : "首次准备，需要一点时间。";
  $("error").textContent = s.error;
  $("data-path").textContent = s.data;
  $("log-output").textContent = s.logs
    .map((l) => `${l.time}  ${l.text}`)
    .join("\n");
  const active = ["installing", "starting", "running"].includes(s.phase);
  ($("start") as HTMLButtonElement).disabled = active;
  ($("stop") as HTMLButtonElement).disabled = !active;
  for (const id of ["open", "browser", "copy", "share"])
    $<HTMLButtonElement>(id).disabled = s.phase !== "running";
  if (first) {
    first = false;
    $<HTMLInputElement>("port-input").value = String(s.settings.port);
    $<HTMLInputElement>("proxy").value = s.settings.proxy;
    $<HTMLInputElement>("autostart").checked = s.settings.autoStart;
    $<HTMLInputElement>("hide").checked = s.settings.hideOnClose;
    $<HTMLInputElement>("ontop").checked = s.settings.alwaysOnTop;
    $<HTMLInputElement>("lan").checked = s.settings.lan;
  }
}
function route() {
  const id = location.hash.slice(1) || "overview";
  for (const section of document.querySelectorAll<HTMLElement>(
    "main > section",
  ))
    section.hidden = section.id !== id;
  for (const a of document.querySelectorAll("nav a"))
    a.classList.toggle("active", a.getAttribute("href") === "#" + id);
}
window.addEventListener("hashchange", route);
route();
action("start", () => call("start"));
action("stop", () => call("stop"));
action("open", () => call("open"));
action("browser", () => call("browser"));
action("copy", async () => {
  await call("copy");
  notice("认证链接已复制，请勿公开分享");
});
action("dialog-copy", () => call("copyShare"));
action("updates", async () => {
  await call("updates");
  notice("已打开本项目的 GitHub Releases");
});
action("share", async () => {
  const url = await call("share");
  const lan=state?.settings.lan;
  $("share-title").textContent=lan?"在可信局域网内继续。":"同一台电脑，换个浏览器。";
  $("share-warning").textContent=lan?"此链接包含完整访问权限。仅限可信私有网络；HTTP 未加密，请勿公开或转发给不可信的人。":"此链接包含完整访问凭证，请勿公开。当前为仅本机地址，手机无法访问。";
  await QRCode.toCanvas($("qr"), url, {
    width: 250,
    margin: 2,
    color: { dark: "#172b2a", light: "#fffef8" },
  });
  $<HTMLDialogElement>("share-dialog").showModal();
});
$("close-share").onclick = () => {
  $<HTMLDialogElement>("share-dialog").close();
  const c = $<HTMLCanvasElement>("qr");
  c.getContext("2d")?.clearRect(0, 0, c.width, c.height);
};
$("settings-form").onsubmit = (e) => {
  e.preventDefault();
  if (!state) return;
  const settings = {
    ...state.settings,
    port: Number($<HTMLInputElement>("port-input").value),
    proxy: $<HTMLInputElement>("proxy").value,
    autoStart: $<HTMLInputElement>("autostart").checked,
    hideOnClose: $<HTMLInputElement>("hide").checked,
    alwaysOnTop: $<HTMLInputElement>("ontop").checked,
    lan: $<HTMLInputElement>("lan").checked,
  };
  call("configure", settings)
    .then(() => notice("设置已保存"))
    .catch((e) => notice(e.message));
};
action("choose", async () => {
  const p = await call("preview", {
    credentials: $<HTMLInputElement>("credentials").checked,
  });
  if (!p) return;
  source = p.source;
  $("preview").textContent =
    `${source}\n${p.files} 个文件 · ${(p.bytes / 1024 / 1024).toFixed(1)} MiB\n${p.credentials ? "包含敏感凭据" : "不包含凭据"}`;
  $<HTMLButtonElement>("import").disabled = false;
});
$<HTMLInputElement>("credentials").onchange = () => {
  source = "";
  $<HTMLButtonElement>("import").disabled = true;
  $("preview").textContent = "选项已改变，请重新选择目录以预览。";
};
action("import", async () => {
  if (!source) return;
  importedBackup = await call("import", {
    source,
    credentials: $<HTMLInputElement>("credentials").checked,
  });
  $("restore").hidden = false;
  $<HTMLButtonElement>("import").disabled = true;
  notice("导入完成。启动后将重建独立插件环境。");
});
action("restore", async () => {
  await call("restore", { backup: importedBackup });
  $("restore").hidden = true;
  notice("已恢复备份");
});
async function poll() {
  try {
    render(await call("status"));
  } catch (e) {
    notice((e as Error).message);
  } finally {
    setTimeout(poll, 1000);
  }
}
poll();
