// Exercise a real PTY, not just require(). Prebuild files can exist while their
// helper executable is missing or not executable after package installation.
const {createRequire}=require('node:module');
const {join}=require('node:path');
const req=createRequire(join(process.argv[1],'node_modules','node-pty','package.json'));
const pty=req('./lib/index.js');
const child=pty.spawn(process.execPath,['-e','process.stdout.write("TINY_PTY_OK")'],{name:'xterm',cols:80,rows:24,cwd:process.cwd(),env:process.env});
let output='';const timer=setTimeout(()=>{child.kill();process.exit(1)},15000);
child.onData(data=>output+=data);
child.onExit(({exitCode})=>{
  clearTimeout(timer);
  if(exitCode!==0||!output.includes('TINY_PTY_OK')){console.error('PTY check failed',{exitCode,receivedMarker:output.includes('TINY_PTY_OK')});process.exit(1)}
  // ConPTY's worker handles can keep Node's event loop alive even after the
  // terminal exited. This disposable verifier has finished; flush then exit.
  process.stdout.write('PASS: native PTY spawn and exit\n',()=>process.exit(0));
});
