// Report replies versus newer reports, through the Go-rendered shell. Each
// mocked reply is built (and stamped with Seq, as Go does) when requested.
const {test}=require('node:test');
const assert=require('node:assert/strict');
const {readFileSync}=require('node:fs');
const {JSDOM}=require('jsdom');
const file=process.env.NATIVE_HTML_OUT;
const html=readFileSync(file,'utf8');
const opened=readFileSync(file+'.raid-opened','utf8');
const settle=()=>new Promise(r=>setTimeout(r,10));
async function setup(t){
 let listener,tick,seq=0;const state={raids:true,names:false,body:'initial',hold:false};const held=[];
 const report=()=>({HTML:'<p>'+state.body+'</p>',Game:'C:\\Training',Account:'A#1',Accounts:[{ID:'A#1'}],HasRaids:state.raids,RaidHTML:state.raids?opened:'',WowheadNames:state.names,InstalledVersion:'1.8.1',Seq:++seq});
 const dom=new JSDOM(html,{runScripts:'dangerously',pretendToBeVisual:true,beforeParse(w){
  w.setInterval=fn=>{tick=fn;return 1};w.requestAnimationFrame=()=>0;
  w.fetch=()=>assert.fail('unexpected network');
  w.chrome={webview:{addEventListener(_,fn){listener=fn},postMessage(raw){
   const req=JSON.parse(raw);
   if(req.method==='setWowheadNames') state.names=req.params[0]==='true';
   const result=['reportRefresh','setWowheadNames'].includes(req.method)?report():null;
   const send=()=>listener({data:{kind:'native-response',id:req.id,result}});
   if(state.hold && result) held.push({method:req.method,send}); else queueMicrotask(send);
  }}};
 }});
 t.after(()=>dom.window.close());await settle();await settle();
 const $=id=>dom.window.document.getElementById(id);
 return {state,held,$,tick:()=>tick(),
  async toggle(){const box=$('wowhead-names');box.checked=!box.checked;box.dispatchEvent(new dom.window.Event('change',{bubbles:true}));await settle();},
  async deliver(method){const i=held.findIndex(h=>h.method===method);assert.ok(i>=0,method+' not held');held.splice(i,1)[0].send();await settle();}};
}
const gone=(x,why)=>{assert.equal(x.$('nav-raids'),null,why+': Raids navigation');assert.equal(x.$('raids-view'),null,why+': Raids page');};
test('preference reply built before a demotion cannot restore Raids',async t=>{
 const x=await setup(t);
 assert.ok(x.$('nav-raids'),'fixture starts admitted');
 x.state.hold=true;x.state.body='save reply built while admitted';await x.toggle();
 x.state.raids=false;x.state.body='later refresh after demotion';x.tick();await settle();
 await x.deliver('reportRefresh');gone(x,'demotion refresh');
 await x.deliver('setWowheadNames');
 gone(x,'late older preference reply');
 assert.equal(x.$('wowhead-names').checked,true,'the saved preference still applies');
 assert.match(x.$('report-body').textContent,/later refresh after demotion/,'older reply report replaced the newer one');
});
test('preference reply with no newer report still displays its report',async t=>{
 const x=await setup(t);
 x.state.body='saved reply';await x.toggle();
 assert.ok(x.$('nav-raids'));assert.equal(x.$('wowhead-names').checked,true);
 assert.match(x.$('report-body').textContent,/saved reply/,'current reply report was not displayed');
 assert.match(x.$('wowhead-names-status').textContent,/Online lookup enabled/);
});
test('preference reply built after a demotion removes Raids even after an older refresh',async t=>{
 const x=await setup(t);
 x.state.hold=true;x.state.body='older refresh built while admitted';x.tick();await settle();
 x.state.raids=false;x.state.body='newer save built after demotion';await x.toggle();
 await x.deliver('reportRefresh');
 await x.deliver('setWowheadNames');
 gone(x,'newer preference reply');
 assert.match(x.$('report-body').textContent,/newer save built after demotion/);
});
test('preference reply old refresh reply arriving after a newer save is dropped',async t=>{
 const x=await setup(t);
 x.state.hold=true;x.state.body='older refresh built while admitted';x.tick();await settle();
 x.state.raids=false;x.state.body='newer save built after demotion';await x.toggle();
 await x.deliver('setWowheadNames');gone(x,'newer save');
 await x.deliver('reportRefresh');
 gone(x,'old refresh delivered late');
 assert.match(x.$('report-body').textContent,/newer save built after demotion/,'old refresh replaced the newer report');
 assert.equal(x.$('wowhead-names').checked,true);
});
