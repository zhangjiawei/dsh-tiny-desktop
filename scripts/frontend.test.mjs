import { test } from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { transform } from "../frontend/node_modules/esbuild/lib/main.js";

test("control page forbids nested frames required by Linux origin policy", async () => {
  const html = await readFile(new URL("../frontend/src/index.html", import.meta.url), "utf8");
  assert.match(html, /frame-src 'none'/);
  assert.doesNotMatch(html, /<iframe\b/i);
});

test("quit action uses the shared navigation and confirmation design system", async () => {
  const html = await readFile(new URL("../frontend/src/index.html", import.meta.url), "utf8");
  const css = await readFile(new URL("../frontend/src/style.css", import.meta.url), "utf8");
  assert.match(html, /id="quit" class="exit-button"/);
  assert.match(html, /id="quit-dialog" class="confirm-dialog" role="alertdialog"/);
  assert.match(html, /class="confirm-content"/);
  assert.match(html, /id="confirm-quit" class="danger-solid"/);
  assert.match(css, /\.dialog-actions button\s*\{[^}]*min-width:\s*88px/s);
});

test("tray setting distinguishes native minimize from close-to-tray", async () => {
  const html = await readFile(new URL("../frontend/src/index.html", import.meta.url), "utf8");
  const i18n = await readFile(new URL("../frontend/src/i18n.ts", import.meta.url), "utf8");
  assert.match(html, /点 × 关闭时隐藏 Dock \/ 任务栏图标；最小化仍保留。/);
  assert.match(i18n, /minimizing keeps the native entry/);
  assert.doesNotMatch(html, /最小化或关闭时隐藏 Dock/);
});

test("overview provides a real one-click service restart", async () => {
  const html = await readFile(new URL("../frontend/src/index.html", import.meta.url), "utf8");
  const client = await readFile(new URL("../frontend/src/main.ts", import.meta.url), "utf8");
  const backend = await readFile(new URL("../cmd/desktop/main.go", import.meta.url), "utf8");
  assert.match(html, /id="restart-service">↻ 一键重启/);
  assert.match(client, /action\("restart-service", async \(\) => \{[\s\S]*call\("restartService"\)/);
  assert.match(backend, /case "restartService":[\s\S]*manager\.Restart\(\)/);
});

test("reverse-proxy access stays separate from LAN binding and secrets", async () => {
  const html = await readFile(new URL("../frontend/src/index.html", import.meta.url), "utf8");
  const client = await readFile(new URL("../frontend/src/main.ts", import.meta.url), "utf8");
  const backend = await readFile(new URL("../cmd/desktop/main.go", import.meta.url), "utf8");
  for (const id of ["lan-address", "trusted-hosts", "public-url", "copy-public", "share-public"])
    assert.match(html, new RegExp(`id="${id}"`));
  assert.match(html, /禁止协议、路径、通配符和用户信息/);
  assert.match(html, /不同的本地端口/);
  assert.match(client, /dialogCopyAction = kind === "public" \? "copyPublic" : "copyShare"/);
  assert.match(backend, /case "sharePublic":[\s\S]*manager\.PublicShareURL/);
  assert.match(backend, /case "copyPublic":[\s\S]*manager\.PublicShareURL/);
  assert.doesNotMatch(html, /token=[A-Za-z0-9_-]{20,}/);
});

test("runtime settings keep managed and custom version ownership separate", async () => {
  const html = await readFile(new URL("../frontend/src/index.html", import.meta.url), "utf8");
  const client = await readFile(new URL("../frontend/src/main.ts", import.meta.url), "utf8");
  const backend = await readFile(new URL("../cmd/desktop/main.go", import.meta.url), "utf8");
  assert.match(html, /id="runtime-mode"[\s\S]*value="managed"[\s\S]*value="custom"/);
  assert.match(html, /id="dsh-channel"[\s\S]*value="recommended"[\s\S]*value="stable"[\s\S]*value="preview"[\s\S]*value="fixed"/);
  assert.match(html, /普通启动时静默升级/);
  assert.match(html, /id="extra-args"/);
  assert.match(html, /id="custom-runtime" hidden/);
  for (const action of ["checkDSHUpdate", "applyDSHUpdate", "rollbackDSH"])
    assert.match(client, new RegExp(`call\\("${action}"\\)`));
  assert.match(backend, /case "applyDSHUpdate":[\s\S]*manager\.ApplyDSHUpdate/);
  assert.match(backend, /case "rollbackDSH":[\s\S]*manager\.RollbackDSH/);
});

test("runtime updater has a visually consistent three-node track", async () => {
  const html = await readFile(new URL("../frontend/src/index.html", import.meta.url), "utf8");
  const css = await readFile(new URL("../frontend/src/style.css", import.meta.url), "utf8");
  assert.match(html, /runtime-node[\s\S]*dsh-current[\s\S]*runtime-node candidate[\s\S]*dsh-target[\s\S]*runtime-node rollback[\s\S]*dsh-previous/);
  assert.match(css, /\.runtime-track\s*\{[\s\S]*grid-template-columns/);
  assert.match(css, /\.runtime-node\.candidate\s*\{[\s\S]*var\(--green-soft\)/);
  assert.match(css, /@media \(max-width: 680px\)[\s\S]*\.runtime-track/);
});

test("import preview exposes Tiny-wins merge and skipped entries", async () => {
  const html = await readFile(new URL("../frontend/src/index.html", import.meta.url), "utf8");
  const client = await readFile(new URL("../frontend/src/main.ts", import.meta.url), "utf8");
  const backend = await readFile(new URL("../cmd/desktop/main.go", import.meta.url), "utf8");
  assert.match(html, /Tiny 已有内容始终优先/);
  assert.match(html, /不支持项会跳过并显示原因/);
  assert.match(client, /可新增.*p\.files/s);
  assert.match(client, /p\.merged.*配置\/数据文件按条目补充/s);
  assert.match(client, /p\.conflicts.*保留 Tiny 版本/s);
  assert.match(client, /p\.skipped.*不支持项已跳过/s);
  assert.match(backend, /manager\.PreviewImport\(source, d\.Credentials\)/);
  assert.match(backend, /manager\.ImportWithRestart\(d\.Source, d\.Credentials\)/);
});

test("status polling never rewrites an open language picker", async () => {
  let value = "语言",
    writes = 0;
  const node = {
    parentElement: { closest: () => null },
    get textContent() {
      return value;
    },
    set textContent(v) {
      value = v;
      writes++;
    },
  };
  globalThis.NodeFilter = { SHOW_TEXT: 4 };
  globalThis.document = {
    documentElement: { lang: "" },
    title: "",
    body: {},
    getElementById: () => null,
    createTreeWalker() {
      let done = false;
      return {
        currentNode: node,
        nextNode() {
          if (done) return false;
          done = true;
          return true;
        },
      };
    },
  };
  const source = await readFile(
    new URL("../frontend/src/i18n.ts", import.meta.url),
    "utf8",
  );
  const { code } = await transform(source, { loader: "ts", format: "esm" });
  const { setLanguage } = await import(
    "data:text/javascript;base64," + Buffer.from(code).toString("base64")
  );
  setLanguage("en", "zh-CN");
  assert.equal(value, "Language");
  const before = writes;
  setLanguage("en", "zh-CN");
  assert.equal(
    writes,
    before,
    "polling mutated picker text while the user was selecting",
  );
  setLanguage("system", "zh-CN");
  assert.equal(value, "语言");
  assert.equal(document.documentElement.lang, "zh-CN");
});
