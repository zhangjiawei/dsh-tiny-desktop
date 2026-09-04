import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {transform} from '../frontend/node_modules/esbuild/lib/main.js';

test('status polling never rewrites an open language picker',async()=>{
 let value='语言',writes=0;
 const node={parentElement:{closest:()=>null},get textContent(){return value},set textContent(v){value=v;writes++}};
 globalThis.NodeFilter={SHOW_TEXT:4};
 globalThis.document={documentElement:{lang:''},title:'',body:{},getElementById:()=>null,createTreeWalker(){let done=false;return {currentNode:node,nextNode(){if(done)return false;done=true;return true}}}};
 const source=await readFile(new URL('../frontend/src/i18n.ts',import.meta.url),'utf8');
 const {code}=await transform(source,{loader:'ts',format:'esm'});
 const {setLanguage}=await import('data:text/javascript;base64,'+Buffer.from(code).toString('base64'));
 setLanguage('en','zh-CN');assert.equal(value,'Language');const before=writes;
 setLanguage('en','zh-CN');assert.equal(writes,before,'polling mutated picker text while the user was selecting');
 setLanguage('system','zh-CN');assert.equal(value,'语言');assert.equal(document.documentElement.lang,'zh-CN');
});
