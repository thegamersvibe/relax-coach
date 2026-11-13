// Actual Go-rendered embedded JS; fake transport, not a Windows GUI pass.
const {test}=require('node:test');
const assert=require('node:assert/strict');
const {readFileSync}=require('node:fs');
const {JSDOM}=require('jsdom');
const html=readFileSync(process.env.NATIVE_HTML_OUT,'utf8');
const settle=()=>new Promise(r=>setTimeout(r,25));
const report={HTML:'<p>No game selected</p>',Game:'',Account:'',Accounts:[],InstalledVersion:'0.0.0',AppVersion:'1.4.1',AppUpdateStatus:'Previous update rolled back safely.'};
const offer={Version:'1.5.0',InstalledVersion:'1.4.1',Available:true,Name:'<img src=x>',Notes:'<script>unsafe</script>',HTMLURL:'https://github.com/thegamersvibe/relax-coach/releases/tag/v1.5.0'};
async function setup(t,handlers={},disabled=false){
 const calls=[],frames=[];let listener;
 const dom=new JSDOM(disabled?html.replace('data-updates-disabled="false"','data-updates-disabled="true"'):html,{runScripts:'dangerously',pretendToBeVisual:true,beforeParse(w){
  w.setInterval=()=>1;w.requestAnimationFrame=fn=>frames.push(fn);
  w.chrome={webview:{addEventListener(_,fn){listener=fn},postMessage(raw){
   const req=JSON.parse(raw);calls.push(req);assert.equal(req.token,'test-token');
   Promise.resolve().then(async()=>{try{const result=handlers[req.method]?await handlers[req.method](...req.params):(req.method==='reportRefresh'?report:null);listener({data:{kind:'native-response',id:req.id,result}})}catch(e){listener({data:{kind:'native-response',id:req.id,error:e.message}})}});
  }}};w.fetch=()=>assert.fail('network from native JS');
 }});
 t.after(()=>dom.window.close());await settle();
 return {dom,calls,$:id=>dom.window.document.getElementById(id),frame:()=>frames.splice(0).forEach(fn=>fn())};
}
test('Updates has separate app controls with backend installed version and recovery result',async t=>{
 const {$,calls}=await setup(t);
 assert.equal($('nav-update').textContent,'Updates');
 assert.equal($('check-app-update')?.textContent,'Check App Update');
 assert.equal($('apply-app-update')?.textContent,'Update App');
 assert.equal($('installed-app-version').textContent,'1.4.1');
 assert.match($('app-last-result').textContent,/rolled back/);
 assert.equal($('check-app-update').disabled,false,'app checks need no game/account');
 assert.ok($('apply-update'),'addon controls retained');
 assert.equal(calls.filter(c=>c.method==='checkApp').length,1,'one read-only startup check');
 assert.equal(calls.some(c=>c.method==='applyApp'||c.method==='applyAddon'),false,'no automatic installs');
});
test('app offer is inert, independent and confirms only version after explicit restart click',async t=>{
 const {$,calls}=await setup(t,{checkApp:()=>offer,checkAddon:()=>({...offer,Version:'2.0.0',InstalledVersion:'1.0.0'}),applyApp:()=>({restartReady:true,message:'Restarting Relax Coach'})});
 assert.ok($('check-app-update'),'app controls missing');
 $('check-app-update').click();await settle();
 assert.equal($('app-release-notes').querySelector('script'),null);
 assert.match($('app-release-notes').textContent,/<script>/);
 assert.equal($('app-release-name').querySelector('img'),null);
 assert.match($('app-update-status').textContent,/restart/i);
 $('check-update').click();await settle();
 assert.equal($('offered-app-version').textContent,'1.5.0');
 assert.equal($('offered-version').textContent,'2.0.0');
 $('apply-app-update').click();await settle();
 assert.deepEqual(calls.find(c=>c.method==='applyApp').params,['1.5.0']);
 assert.equal(calls.some(c=>c.method==='applyAddon'),false);
 assert.equal($('check-app-update').disabled,true,'handoff keeps controls locked');
});
test('app errors consume offers and allow explicit retry; no-update disables both',async t=>{
 let fail=false;
 const {$}=await setup(t,{checkApp:()=>{if(fail)throw Error('offline');return offer},applyApp:()=>{throw Error('worker failed')}});
 assert.ok($('check-app-update'),'app controls missing');
 $('check-app-update').click();await settle();$('apply-app-update').click();await settle();
 assert.match($('app-update-status').textContent,/worker failed/);assert.equal($('apply-app-update').disabled,true);assert.equal($('check-app-update').disabled,false);
 $('check-app-update').click();await settle();fail=true;$('check-app-update').click();await settle();
 assert.match($('app-update-status').textContent,/offline/);assert.equal($('apply-app-update').disabled,true);
 const off=await setup(t,{},true);
 for(const id of ['check-app-update','apply-app-update','check-update','apply-update'])assert.equal(off.$(id).disabled,true,id);
});
test('no-update result shows backend reason instead of up-to-date claim',async t=>{
 const {$}=await setup(t,{checkApp:()=>({...offer,Available:false,Notes:'Required executable is missing'})});
 assert.ok($('check-app-update'),'app controls missing');$('check-app-update').click();await settle();
 assert.equal($('apply-app-update').disabled,true);assert.match($('app-release-notes').textContent,/missing/);assert.doesNotMatch($('app-update-status').textContent,/up to date/i);
});
test('native health waits for real report response and rendered frames, then acknowledges once',async t=>{
 let resolve;const pending=new Promise(r=>resolve=r);
 const s=await setup(t,{reportRefresh:()=>pending});
 s.frame();s.frame();await settle();assert.equal(s.calls.some(c=>c.method==='appReady'),false);
 resolve(report);await settle();assert.equal(s.calls.some(c=>c.method==='appReady'),false);
 s.frame();s.frame();await settle();
 assert.deepEqual(s.calls.filter(c=>c.method==='appReady').map(c=>c.params),[[]]);
 s.$('refresh').click();await settle();s.frame();s.frame();await settle();assert.equal(s.calls.filter(c=>c.method==='appReady').length,1);
});
test('failed initial report does not acknowledge; successful manual retry does',async t=>{
 let fail=true;const s=await setup(t,{reportRefresh:()=>{if(fail)throw Error('render unavailable');return report}});
 s.frame();s.frame();await settle();assert.equal(s.calls.some(c=>c.method==='appReady'),false);
 fail=false;s.$('refresh').click();await settle();s.frame();s.frame();await settle();assert.equal(s.calls.filter(c=>c.method==='appReady').length,1);
});
test('startup check announces a new app on Updates without installing; -no-update skips it',async t=>{
 const {$,calls}=await setup(t,{checkApp:()=>offer});
 assert.equal(calls.filter(c=>c.method==='checkApp').length,1);
 assert.equal($('nav-update').dataset.new,'true');
 assert.equal($('nav-update').textContent,'Updates');
 assert.equal($('offered-app-version').textContent,'1.5.0');
 assert.equal($('apply-app-update').disabled,false,'one click to update');
 assert.match($('global-status').textContent,/1\.5\.0 is available/);
 assert.equal(calls.some(c=>c.method==='applyApp'),false,'never installs by itself');
 const quiet=await setup(t,{checkApp:()=>({...offer,Available:false})});
 assert.equal(quiet.$('nav-update').dataset.new,undefined);
 assert.equal(quiet.$('global-status').textContent,'');
 const off=await setup(t,{checkApp:()=>offer},true);
 assert.equal(off.calls.some(c=>c.method==='checkApp'),false);
 const down=await setup(t,{checkApp:()=>{throw Error('offline')}});
 assert.equal(down.$('offered-app-version').textContent,'Not checked');
 assert.equal(down.$('check-app-update').disabled,false);
});
