// Actual Go-rendered embedded JS; fake transport, not a Windows GUI pass.
const {test}=require('node:test');
const assert=require('node:assert/strict');
const {readFileSync}=require('node:fs');
const {JSDOM}=require('jsdom');
const html=readFileSync(process.env.NATIVE_HTML_OUT,'utf8');
const settle=()=>new Promise(r=>setTimeout(r,25));
const report={HTML:'<p>ok</p>',Game:'C:\\WoW',Account:'',Accounts:[],InstalledVersion:'1.4.5',AppVersion:'1.4.5',AppUpdateStatus:''};
let installed={rankup:'0.0.0',forevermaps:'0.1.2'};
const list=()=>[{ID:'rankup',Name:'RankUp',Blurb:'<b>x</b>',InstalledVersion:installed.rankup},{ID:'forevermaps',Name:'Forever Maps',Blurb:'maps',InstalledVersion:installed.forevermaps}];
async function setup(t,handlers={}){
 const calls=[];let listener;
 const dom=new JSDOM(html,{runScripts:'dangerously',pretendToBeVisual:true,beforeParse(w){
  w.setInterval=()=>1;w.requestAnimationFrame=()=>0;
  w.chrome={webview:{addEventListener(_,fn){listener=fn},postMessage(raw){
   const req=JSON.parse(raw);calls.push(req);
   Promise.resolve().then(async()=>{try{const result=handlers[req.method]?await handlers[req.method](...req.params):(req.method==='reportRefresh'?report:null);listener({data:{kind:'native-response',id:req.id,result}})}catch(e){listener({data:{kind:'native-response',id:req.id,error:e.message}})}});
  }}};w.fetch=()=>assert.fail('network from native JS');
 }});
 t.after(()=>dom.window.close());await settle();
 return {dom,calls,$:id=>dom.window.document.getElementById(id)};
}
test('More addons: listed on Updates, nothing checked automatically, install then update',async t=>{
 installed={rankup:'0.0.0',forevermaps:'0.1.2'};
 const {$,calls}=await setup(t,{
  extraList:list,
  checkExtra:id=>({ID:id,Name:id==='rankup'?'RankUp':'Forever Maps',InstalledVersion:installed[id],Version:'1.0.1',Available:true,Install:installed[id]==='0.0.0',Status:'ready',Notes:'<script>x</script>'}),
  applyExtra:arg=>{const [id,v]=arg.split('@');installed[id]=v;return {version:v,message:'done '+id}},
 });
 $('nav-update').click();await settle();
 const rank=$('extra-rankup'),maps=$('extra-forevermaps');
 assert.ok(rank&&maps,'both addons listed');
 assert.equal(rank.querySelector('.extra-blurb').querySelector('b'),null,'blurb is text, not HTML');
 assert.equal(rank.querySelector('.extra-installed').textContent,'Not installed');
 assert.equal(maps.querySelector('.extra-installed').textContent,'0.1.2');
 assert.equal(rank.querySelector('[data-role=apply]').textContent,'Install');
 assert.equal(maps.querySelector('[data-role=apply]').textContent,'Update');
 assert.equal(rank.querySelector('[data-role=apply]').disabled,true,'install needs a check first');
 assert.equal(calls.some(c=>c.method==='checkExtra'||c.method==='applyExtra'),false,'no automatic network or install');
 rank.querySelector('[data-role=check]').click();await settle();
 assert.equal(rank.querySelector('.extra-offered').textContent,'1.0.1');
 assert.equal(rank.querySelector('[data-role=apply]').disabled,false);
 assert.equal(maps.querySelector('[data-role=apply]').disabled,true,'other addon still needs its own check');
 rank.querySelector('[data-role=apply]').click();await settle();await settle();
 assert.deepEqual(calls.find(c=>c.method==='applyExtra').params,['rankup@1.0.1']);
 assert.equal(calls.some(c=>c.method==='applyAddon'),false,'Coach bundle untouched');
 assert.match($('extra-rankup').querySelector('.extra-status').textContent,/done rankup/);
 assert.equal($('extra-rankup').querySelector('.extra-installed').textContent,'1.0.1');
 assert.equal($('extra-rankup').querySelector('[data-role=apply]').disabled,true,'offer consumed after install');
});
test('More addons: failed install consumes the offer and shows the reason',async t=>{
 installed={rankup:'1.0.0',forevermaps:'0.0.0'};
 const {$}=await setup(t,{extraList:list,
  checkExtra:id=>({ID:id,Name:'Forever Maps',InstalledVersion:'0.0.0',Version:'0.1.3',Available:true,Install:true,Status:'ready'}),
  applyExtra:()=>{throw Error('Close WoW and try again: Windows refused to move a file that is in use. Nothing was changed.')}});
 $('nav-update').click();await settle();
 const maps=$('extra-forevermaps');
 maps.querySelector('[data-role=check]').click();await settle();
 maps.querySelector('[data-role=apply]').click();await settle();await settle();
 assert.match($('extra-forevermaps').querySelector('.extra-status').textContent,/Close WoW and try again: Windows refused to move a file that is in use\. Nothing was changed\./);
 assert.equal($('extra-forevermaps').querySelector('[data-role=apply]').disabled,true);
 assert.equal($('extra-forevermaps').querySelector('[data-role=check]').disabled,false);
});
test('More addons: a folder that is not the addon shows Unrecognized, not Install',async t=>{
 const {$}=await setup(t,{extraList:()=>[{ID:'rankup',Name:'RankUp',Blurb:'',InstalledVersion:'0.0.0',Problem:'the RankUp folder does not contain RankUp'},{ID:'forevermaps',Name:'Forever Maps',Blurb:'',InstalledVersion:'0.0.0'}]});
 $('nav-update').click();await settle();
 const rank=$('extra-rankup');
 assert.equal(rank.querySelector('.extra-installed').textContent,'Unrecognized');
 assert.match(rank.querySelector('.extra-status').textContent,/will not be replaced/);
 assert.ok(rank.querySelector('[data-role=apply]').disabled,'no install offered');
 assert.equal($('extra-forevermaps').querySelector('.extra-installed').textContent,'Not installed');
});
