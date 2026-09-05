// First check the exact packaged exe using native UI Automation. Then exercise
// the same source in a QA-only instrumented WebView2 build (not a mock bridge).
// Wails deliberately clears SDK debugging environment variables; published
// binaries therefore never expose a CDP port. Temporary CI profiles only.
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { resolve, join } from "node:path";
import { createServer } from "node:net";

assert.equal(process.platform, "win32", "requires native Windows WebView2");
const [binaryArg, rootArg, qaBinaryArg] = process.argv.slice(2);
assert.ok(binaryArg && rootArg && qaBinaryArg, "requires published exe, disposable smoke root and QA exe");
const smokeRoot = resolve(rootArg);
assert.ok(process.env.RUNNER_TEMP && smokeRoot.startsWith(resolve(process.env.RUNNER_TEMP) + "\\"), "CI temporary roots only");
// The exact packaged GUI must complete a truly fresh install without console
// stdin or a system Node/dsh. This is the seam that reproduces user machines;
// the earlier CLI smoke intentionally cannot cover a windowsgui process.
const root = resolve(process.env.RUNNER_TEMP, "dsh-tiny-gui-install");
assert.ok(root.startsWith(resolve(process.env.RUNNER_TEMP) + "\\"), "fresh GUI root must remain under RUNNER_TEMP");
await rm(root, {recursive:true, force:true});
await mkdir(root, {recursive:true});
const settingsPath = join(root, "settings.json");
const defaultCommand = "pnpm --allow-build=@deepseek-ai/dsh-subprocess-local --allow-build=node-pty --allow-build=koffi dlx @deepseek-ai/dsh@0.1.2-rc.1 web";
const settings = {port:3080, proxy:"", lan:false, hideOnClose:true, trayOnly:true,
  autoStart:true, alwaysOnTop:false, language:"en", command:defaultCommand, registry:"https://registry.npmjs.org",
  startupMinutes:60, width:1280, height:840};
await writeFile(settingsPath, JSON.stringify(settings));
const listener = createServer();
const debugPort = 49223; // Must match the CI-only linker flag; fail on collision.
await new Promise((r,j) => {listener.once('error',j);listener.listen(debugPort, "127.0.0.1", r);});
await new Promise(r => listener.close(r));
const privateRuntimeEnv = {...process.env, DSH_TINY_HOME:root,
  PATH:process.env.PATH.split(';').filter(part=>!part.toLowerCase().includes('node')).join(';')};
const releaseChild = spawn(resolve(binaryArg), [], {env:privateRuntimeEnv,stdio:"ignore"});
try {
  const code = await new Promise((r,j)=>{
    releaseChild.once('error',j);
    const probe=spawn('pwsh',['-NoProfile','-File','scripts/windows-control-ready.ps1','-AppProcessId',String(releaseChild.pid),'-ExpectedStatus','Running','-TimeoutSeconds','900'],{stdio:'inherit'});
    probe.once('error',j);probe.once('exit',r);
  });
  assert.equal(code,0,'packaged native control status failed');
} finally {
  if(releaseChild.pid) await new Promise(r=>spawn('taskkill',['/PID',String(releaseChild.pid),'/T','/F'],{stdio:'ignore'}).on('exit',r));
}
console.log('PASS: packaged windowsgui app completed a fresh private install without console stdin or system Node/dsh');
settings.autoStart = false;
await writeFile(settingsPath, JSON.stringify(settings));
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
  if (ws?.readyState===WebSocket.OPEN) {
    // Keep failure evidence narrow: never dump logs, launch URLs or QR content.
    try {console.error('QA timeout state',await evaluate(`({phase:document.querySelector('#phase')?.textContent,route:location.hash,notice:document.querySelector('#notice')?.textContent,invalid:[...document.querySelectorAll('#settings-form :invalid')].map(e=>e.id)})`));} catch {}
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
  warning:document.querySelector('#port-warning')?.textContent,
  warningHidden:document.querySelector('#port-warning')?.hidden,
  pending:document.querySelector('#settings-status')?.textContent})`);
// Deliberately occupy the requested port: the banner must describe the real
// listening service, not merely a guessed candidate or a pending preference.
const occupied = createServer(socket => socket.destroy());
try {
  await new Promise((r,j)=>{occupied.once('error',j);occupied.listen(3080,'127.0.0.1',r);});
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
  assert.equal(await evaluate(`document.title`), 'DSH Tiny · Settings');
  await evaluate(`location.hash='#settings';document.querySelector('#command-example').click();document.querySelector('#registry-default').click()`);
  const defaults = await evaluate(`({command:document.querySelector('#command').value,registry:document.querySelector('#registry').value})`);
  assert.match(defaults.command,/^pnpm .* dlx @deepseek-ai\/dsh@.+ web$/);
  assert.equal(defaults.registry,'https://registry.npmmirror.com');
  await evaluate(`document.querySelector('#settings-form').requestSubmit()`);
  await until(async()=>JSON.parse(await readFile(settingsPath,'utf8')).registry===defaults.registry,'default mirror saved');
  // Stress the actual asynchronous WebView2 message boundary. A request sent
  // immediately before another hash change must still reach the same document.
  for (let n=0;n<6;n++) {
    const port=n%2 ? 3080 : 3081;
    await evaluate(`location.hash='#settings';document.querySelector('#port-input').value='${port}';document.querySelector('#settings-form').requestSubmit();location.hash='#overview'`);
    await until(async()=>JSON.parse(await readFile(settingsPath,'utf8')).port===port,'save during hash navigation');
  }
  console.log('PASS: six consecutive saves across in-flight hash navigation');
  await evaluate(`location.hash='#overview'`);
  console.log("PASS: native WebView2 idle status, empty logs and independent data directory rendered");
  await evaluate(`document.querySelector('#start').click()`);
  const running=await until(async()=>{const s=await status();return s.phase==="Running" && s.logs ? s : null;}, "DSH ready through native control bridge",180000);
  assert.match(running.port,/127\.0\.0\.1\s*:\s*\d+/);
  const actualPort = Number(running.port.split(':').at(-1).trim());
  assert.notEqual(actualPort,3080);
  assert.equal(running.warningHidden,false);
  assert.ok(running.warning.includes(String(actualPort)) && running.warning.includes('3080'));
  console.log('PASS: default pnpm command and domestic mirror; real occupied-port banner uses actual random service port');
  console.log("PASS: native Start button, authenticated DSH readiness, port and logs");
  // Hash navigation must keep working. Save runtime settings without stopping.
  await evaluate(`location.hash='#settings';document.querySelector('#port-input').value='43081';document.querySelector('#settings-form').requestSubmit()`);
  await until(async()=>{const s=await status();return s.pending?.includes("pending") && s.phase==="Running" && s.port===running.port;}, "save without interrupting running service");
  assert.equal(JSON.parse(await readFile(settingsPath,"utf8")).port,43081);
  assert.equal((await status()).warning,running.warning,'pending preference must not change active conflict warning');
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
  assert.equal((await status()).warningHidden,true);
  console.log("PASS: open, QR, stop, next-start configuration over real Windows bridge");
  await evaluate(`document.querySelector('#quit').click()`);
  assert.equal(await evaluate(`document.querySelector('#quit-dialog').open`),true);
  await evaluate(`document.querySelector('#cancel-quit').click()`);
  assert.equal((await status()).phase,'Running');
  assert.equal(await evaluate(`document.querySelector('#quit-dialog').open`),false);
  // Do not await a CDP reply from a document that intentionally disappears.
  await evaluate(`document.querySelector('#quit').click();setTimeout(()=>document.querySelector('#confirm-quit').click(),100)`);
  const quitDeadline=Date.now()+30000;
  while(child.exitCode===null && Date.now()<quitDeadline) await pause(250);
  assert.equal(child.exitCode,0,'Settings Quit must exit the app cleanly');
  const freed=createServer();
  await new Promise((r,j)=>{freed.once('error',j);freed.listen(43081,'127.0.0.1',r);});
  await new Promise(r=>freed.close(r));
  console.log('PASS: cancel quit preserves DSH; Settings Quit exits application and releases DSH service port');
} finally {
  if (child.exitCode===null && ws?.readyState===WebSocket.OPEN) {
    try {await evaluate(`document.querySelector('#stop')?.click()`);await until(async()=>(await status()).phase==="Stopped","cleanup",30000);} catch {}
    ws.close();
  }
  // This process tree belongs exclusively to this test; never kill by image name.
  if (child.pid && child.exitCode===null) await new Promise(r=>spawn("taskkill",["/PID",String(child.pid),"/T","/F"],{stdio:"ignore"}).on("exit",r));
  if (occupied.listening) await new Promise(r=>occupied.close(r));
  for (const p of pending.values()) clearTimeout(p.timer);
}
