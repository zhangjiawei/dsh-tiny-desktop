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
