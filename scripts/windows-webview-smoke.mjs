// First check the exact packaged exe using native UI Automation. Then exercise
// the same source in a QA-only instrumented WebView2 build (not a mock bridge).
// Wails deliberately clears SDK debugging environment variables; published
// binaries therefore never expose a CDP port. Temporary CI profiles only.
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { readFile, writeFile } from "node:fs/promises";
import { resolve, join } from "node:path";
import { createServer } from "node:net";

assert.equal(process.platform, "win32", "requires native Windows WebView2");
const [binaryArg, rootArg, qaBinaryArg] = process.argv.slice(2);
assert.ok(binaryArg && rootArg && qaBinaryArg, "requires published exe, disposable smoke root and QA exe");
const root = resolve(rootArg);
assert.ok(process.env.RUNNER_TEMP && root.startsWith(resolve(process.env.RUNNER_TEMP) + "\\"), "CI temporary roots only");
const settingsPath = join(root, "settings.json");
// The preceding real DSH smoke populated this isolated runtime. Only this QA
// profile uses deterministic language and disables auto-start for an idle test.
const settings = {port:3080, proxy:"", lan:false, hideOnClose:true, trayOnly:true,
  autoStart:false, alwaysOnTop:false, language:"en", command:"", registry:"https://registry.npmjs.org",
  startupMinutes:60, width:1280, height:840};
await writeFile(settingsPath, JSON.stringify(settings));
const listener = createServer();
const debugPort = 49223; // Must match the CI-only linker flag; fail on collision.
await new Promise((r,j) => {listener.once('error',j);listener.listen(debugPort, "127.0.0.1", r);});
await new Promise(r => listener.close(r));
const releaseChild = spawn(resolve(binaryArg), [], {env:{...process.env,DSH_TINY_HOME:root},stdio:"ignore"});
try {
  const code = await new Promise((r,j)=>{
    releaseChild.once('error',j);
    const probe=spawn('pwsh',['-NoProfile','-File','scripts/windows-control-ready.ps1','-AppProcessId',String(releaseChild.pid)],{stdio:'inherit'});
    probe.once('error',j);probe.once('exit',r);
  });
  assert.equal(code,0,'packaged native control status failed');
} finally {
  if(releaseChild.pid) await new Promise(r=>spawn('taskkill',['/PID',String(releaseChild.pid),'/T','/F'],{stdio:'ignore'}).on('exit',r));
}
const child = spawn(resolve(qaBinaryArg), [], {env:{...process.env, DSH_TINY_HOME:root}, stdio:"ignore"});
let spawnError;
child.on("error", e => {spawnError = e;});
const pause = ms => new Promise(r => setTimeout(r, ms));
async function until(fn, label, timeout=30000) {
  const end = Date.now()+timeout;
  while (Date.now()<end) {
    if (spawnError) throw spawnError;
    if (child.exitCode !== null) throw Error(`native app exited: ${child.exitCode}`);
    const result = await fn();
    if (result) return result;
    await pause(250);
  }
  throw Error(`timeout: ${label}`);
}
let ws, seq=0;
const pending = new Map();
async function evaluate(expression) {
  const id=++seq;
  const result = await new Promise((resolve,reject)=>{
    const timer=setTimeout(()=>{pending.delete(id);reject(Error("CDP evaluation timed out"));},10000);
    pending.set(id,{resolve,reject,timer});
    ws.send(JSON.stringify({id,method:"Runtime.evaluate",params:{expression,returnByValue:true,awaitPromise:true}}));
  });
  assert.ok(!result.exceptionDetails, "control JavaScript evaluation failed");
  return result.result?.value;
}
const status = () => evaluate(`({phase:document.querySelector('#phase')?.textContent,
  data:document.querySelector('#data-path')?.textContent, port:document.querySelector('#port')?.textContent,
  logs:!!document.querySelector('#log-output')?.textContent,
  pending:document.querySelector('#settings-status')?.textContent})`);
try {
  const target=await until(async()=>{
    try {
      const targets=await (await fetch(`http://127.0.0.1:${debugPort}/json/list`)).json();
      return targets.find(t=>t.type==="page" && t.url.startsWith("http://wails.localhost/"));
    } catch { return null; }
  }, "WebView2 control document");
  ws=new WebSocket(target.webSocketDebuggerUrl);
  await new Promise((r,j)=>{ws.addEventListener("open",r,{once:true});ws.addEventListener("error",j,{once:true});});
  ws.addEventListener("message",({data})=>{
    const msg=JSON.parse(data), p=pending.get(msg.id);
    if (!p) return;
    pending.delete(msg.id);clearTimeout(p.timer);
    msg.error ? p.reject(Error("CDP command failed")) : p.resolve(msg.result);
  });
  const idle=await until(async()=>{const s=await status();return s.phase==="Stopped" && s.data ? s : null;}, "real status bridge (data directory)");
  assert.ok(idle.data.toLowerCase().startsWith(root.toLowerCase()));
  console.log("PASS: native WebView2 idle status, empty logs and independent data directory rendered");
  await evaluate(`document.querySelector('#start').click()`);
  const running=await until(async()=>{const s=await status();return s.phase==="Running" && s.logs ? s : null;}, "DSH ready through native control bridge",180000);
  assert.match(running.port,/127\.0\.0\.1\s*:\s*\d+/);
  console.log("PASS: native Start button, authenticated DSH readiness, port and logs");
  // Hash navigation must keep working. Save runtime settings without stopping.
  await evaluate(`location.hash='#settings';document.querySelector('#port-input').value='43081';document.querySelector('#settings-form').requestSubmit()`);
  await until(async()=>{const s=await status();return s.pending?.includes("pending") && s.phase==="Running" && s.port===running.port;}, "save without interrupting running service");
  assert.equal(JSON.parse(await readFile(settingsPath,"utf8")).port,43081);
  console.log("PASS: save persists pending launch settings while current service stays running");
  await evaluate(`location.hash='#overview';document.querySelector('#open').click()`);
  await until(()=>evaluate(`document.querySelector('#notice')?.textContent==='Workspace opened.'`),"open workspace response");
  await evaluate(`document.querySelector('#share').click()`);
  await until(()=>evaluate(`document.querySelector('#share-dialog')?.open===true`),"authenticated QR response");
  await evaluate(`document.querySelector('#close-share').click();document.querySelector('#stop').click()`);
  await until(async()=>(await status()).phase==="Stopped","native Stop button");
  // Verify the next start actually applies the saved port, then stop cleanly.
  await evaluate(`document.querySelector('#start').click()`);
  await until(async()=>{const s=await status();return s.phase==="Running" && s.port.includes('43081');},"saved launch port applied",180000);
  console.log("PASS: open, QR, stop, next-start configuration over real Windows bridge");
} finally {
  if (ws?.readyState===WebSocket.OPEN) {
    try {await evaluate(`document.querySelector('#stop')?.click()`);await until(async()=>(await status()).phase==="Stopped","cleanup",30000);} catch {}
    ws.close();
  }
  // This process tree belongs exclusively to this test; never kill by image name.
  if (child.pid) await new Promise(r=>spawn("taskkill",["/PID",String(child.pid),"/T","/F"],{stdio:"ignore"}).on("exit",r));
  for (const p of pending.values()) clearTimeout(p.timer);
}
