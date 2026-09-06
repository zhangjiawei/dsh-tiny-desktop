// Recreate the user's interrupted-install failure using a disposable, already
// boot-tested runtime. This fixture never targets global DSH or real user data.
import assert from 'node:assert/strict';
import {readFile, writeFile, rename, mkdir, rm} from 'node:fs/promises';
import {spawnSync} from 'node:child_process';
import {resolve, join, sep, delimiter, dirname} from 'node:path';

const root=resolve(process.argv[2] || '.');
const base=process.env.DSH_TINY_TEST_BASE || process.env.RUNNER_TEMP;
assert.ok(base && root.startsWith(resolve(base)+sep), 'requires a disposable test root under RUNNER_TEMP or DSH_TINY_TEST_BASE');
const data=join(root,'dsh'), profile=join(data,'profiles','web');
const manifestPath=join(profile,'package.json'), lockPath=join(profile,'pnpm-lock.yaml');
const runtimeRoot=join(root,'runtime'), legacyRuntime=join(runtimeRoot,'dsh');
let activeRuntime=legacyRuntime;
try {
  const managed=JSON.parse(await readFile(join(runtimeRoot,'managed-runtime.json'),'utf8'));
  activeRuntime=resolve(managed.current.dir);
  assert.ok(activeRuntime.startsWith(runtimeRoot+sep),'managed runtime escaped disposable root');
} catch (error) {
  if (error.code!=='ENOENT') throw error;
}
const receiptPath=activeRuntime===legacyRuntime ? join(runtimeRoot,'receipt.json') : join(activeRuntime,'receipt.json');
const receiptBackup=receiptPath+'.recovery-fixture';
const [originalManifest,originalLock,originalReceipt]=await Promise.all([manifestPath,lockPath,receiptPath].map(p=>readFile(p)));
const receipt=JSON.parse(originalReceipt), manifest=JSON.parse(originalManifest);
assert.equal(receipt.Plugins.length,6,'requires the production six-plugin smoke fixture');
for (const plugin of receipt.Plugins) {
  assert.ok(manifest.dependencies[plugin.name]);
  delete manifest.dependencies[plugin.name];
}
// A sentinel outside the executable profile proves the recovery retains data.
const sentinel=join(data,'sessions','recovery-test-sentinel.txt');
await mkdir(dirname(sentinel),{recursive:true});
await writeFile(sentinel,'keep user data\n',{flag:'wx'});
let succeeded=false;
try {
  await writeFile(manifestPath,JSON.stringify(manifest,null,2));
  await rename(receiptPath,receiptBackup);
  const cli=join(activeRuntime,'node_modules','@deepseek-ai','dsh','lib','bin.js');
  const privateBin=join(root,'runtime','tools','node_modules','.bin');
  const broken=spawnSync(process.execPath,[cli,'plugin','--profile','web','install','--registry=https://registry.npmmirror.com'],{
    env:{...process.env,CI:'true',DSH_HOME:data,DSH_PROFILE_DIR:profile,DSH_RUNTIME_DIR:activeRuntime,PATH:[privateBin,dirname(process.execPath),process.env.PATH].join(delimiter)},encoding:'utf8',timeout:30000,
  });
  assert.notEqual(broken.status,0,'stale lock fixture did not reproduce');
  assert.match((broken.stdout || '')+(broken.stderr || ''),/ERR_PNPM_OUTDATED_LOCKFILE/);
  console.log('PASS: reproduced the real six-dependency frozen-lockfile failure');
  // Exercise production Manager.Start -> Installer.Ensure, not a patched pnpm
  // invocation. The normal smoke also verifies six plugins, PTY and shutdown.
  const repaired=spawnSync('go',['run','./cmd/smoke','--root',root],{stdio:'inherit',timeout:20*60*1000});
  assert.equal(repaired.status,0,'production installer did not recover');
  assert.equal(await readFile(sentinel,'utf8'),'keep user data\n');
  assert.equal(JSON.parse(await readFile(receiptPath,'utf8')).Plugins.length,6);
  succeeded=true;
  console.log('PASS: interrupted installation recovered; all six plugins and session data retained');
} finally {
  if (!succeeded) {
    // Preserve the pre-test fixture on failure; the assertion still fails CI.
    await writeFile(manifestPath,originalManifest);
    await writeFile(lockPath,originalLock);
    await writeFile(receiptPath,originalReceipt);
  }
  await rm(receiptBackup,{force:true});
  await rm(sentinel,{force:true});
}
