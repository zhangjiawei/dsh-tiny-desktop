// Independently verify public downloads after publishing; never execute them.
// Downloads live under ignored work/ and can be removed after verification.
import {mkdir,writeFile} from 'node:fs/promises';
import {createHash} from 'node:crypto';
const tag=process.argv[2], repo='zhangjiawei/dsh-tiny-desktop';
if(!/^v\d+\.\d+\.\d+$/.test(tag || ''))throw Error('Provide a release tag, for example v0.2.4');
const dir=new URL('../work/release-'+tag+'-verify/',import.meta.url);
const targets=['macos-amd64.zip','macos-arm64.zip','windows-amd64.zip','windows-arm64.zip','linux-amd64.tar.gz','linux-arm64.tar.gz'].map(s=>'dsh-tiny-desktop-'+tag.slice(1)+'-'+s);
async function download(url){
  const r=await fetch(url);
  if(!r.ok)throw Error('Download HTTP '+r.status);
  return Buffer.from(await r.arrayBuffer());
}
const r=await fetch('https://api.github.com/repos/'+repo+'/releases/tags/'+tag);
if(!r.ok)throw Error('Release HTTP '+r.status);
const release=await r.json();
if(release.draft || !release.prerelease || release.tag_name!==tag)throw Error('Unexpected release state');
const names=release.assets.map(a=>a.name).sort();
if(JSON.stringify(names)!==JSON.stringify([...targets,'SHA256SUMS.txt'].sort()))throw Error('Unexpected asset set');
await mkdir(dir,{recursive:true});
const sums=await download(release.assets.find(a=>a.name==='SHA256SUMS.txt').browser_download_url);
await writeFile(new URL('SHA256SUMS.txt',dir),sums);
const expected=new Map(sums.toString('utf8').trim().split('\n').map(line=>{
  const m=/^([0-9a-f]{64})\s+([^\s]+)$/.exec(line);
  if(!m)throw Error('Malformed checksum');
  return [m[2],m[1]];
}));
if(expected.size!==6)throw Error('Expected six checksums');
for(const name of targets){
  const asset=release.assets.find(a=>a.name===name);
  const body=await download(asset.browser_download_url);
  const hash=createHash('sha256').update(body).digest('hex');
  if(hash!==expected.get(name))throw Error('Checksum mismatch: '+name);
  if(body.length!==asset.size)throw Error('Size mismatch: '+name);
  await writeFile(new URL(name,dir),body);
  console.log('PASS',name,body.length,'bytes',hash);
}
console.log('PASS public prerelease, six archives and SHA256SUMS:',release.html_url);
