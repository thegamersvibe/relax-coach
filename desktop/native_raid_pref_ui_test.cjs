// Preference-save replies versus a newer report, through the Go-rendered shell.
const {test}=require('node:test');
const assert=require('node:assert/strict');
const {readFileSync}=require('node:fs');
const {JSDOM}=require('jsdom');
const file=process.env.NATIVE_HTML_OUT;
const html=readFileSync(file,'utf8');
const opened=readFileSync(file+'.raid-opened','utf8');
const settle=()=>new Promise(r=>setTimeout(r,10));
const report=(raids,names,body='<p>Coach</p>')=>({HTML:body,Game:'C:\\Training',Account:'A#1',Accounts:[{ID:'A#1'}],HasRaids:raids,RaidHTML:raids?opened:'',WowheadNames:names,InstalledVersion:'1.8.1'});
async function setup(t){
 let listener,tick,finish;const state={raids:true,names:false};
 const dom=new JSDOM(html,{runScripts:'dangerously',pretendToBeVisual:true,beforeParse(w){
  w.setInterval=fn=>{tick=fn;return 1};w.requestAnimationFrame=()=>0;
  w.fetch=()=>assert.fail('unexpected network');
  w.chrome={webview:{addEventListener(_,fn){listener=fn},postMessage(raw){
   const req=JSON.parse(raw);
   const send=result=>listener({data:{kind:'native-response',id:req.id,result}});
   // The save reply is built now (before any demotion) and delivered later.
   if(req.method==='setWowheadNames') {const built=report(state.raids,req.params[0]==='true','<p>Saved reply</p>');state.names=built.WowheadNames;finish=()=>send(built);return;}
   queueMicrotask(()=>send(req.method==='reportRefresh'?report(state.raids,state.names):null));
  }}};
 }});
 t.after(()=>dom.window.close());await settle();await settle();
 const $=id=>dom.window.document.getElementById(id);
 return {dom,state,$,tick:()=>tick(),finish:()=>finish(),async toggle(){const box=$('wowhead-names');box.checked=!box.checked;box.dispatchEvent(new dom.window.Event('change',{bubbles:true}));await settle();}};
}
test('preference reply built before a demotion cannot restore Raids',async t=>{
 const x=await setup(t);
 assert.ok(x.$('nav-raids'),'fixture starts admitted');
 await x.toggle();
 x.state.raids=false;x.tick();await settle();
 assert.equal(x.$('nav-raids'),null,'demotion refresh removes Raids');
 x.finish();await settle();
 assert.equal(x.$('nav-raids'),null,'late preference reply restored Raids navigation');
 assert.equal(x.$('raids-view'),null,'late preference reply restored the Raids page');
 assert.equal(x.$('wowhead-names').checked,true,'the saved preference still applies');
 assert.doesNotMatch(x.$('report-body').textContent,/Saved reply/,'older reply report replaced the newer one');
 x.tick();await settle();
 assert.equal(x.$('nav-raids'),null);assert.equal(x.$('wowhead-names').checked,true);
});
test('preference reply with no newer report still displays its report',async t=>{
 const x=await setup(t);
 await x.toggle();
 x.finish();await settle();
 assert.ok(x.$('nav-raids'));assert.equal(x.$('wowhead-names').checked,true);
 assert.match(x.$('report-body').textContent,/Saved reply/,'current reply report was not displayed');
 assert.match(x.$('wowhead-names-status').textContent,/Online lookup enabled/);
});
