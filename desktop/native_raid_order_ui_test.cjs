// Reviewer ordering matrix, through the Go-rendered shell and native transport.
const {test}=require('node:test');
const assert=require('node:assert/strict');
const {readFileSync}=require('node:fs');
const {JSDOM}=require('jsdom');
const file=process.env.NATIVE_HTML_OUT;
const html=readFileSync(file,'utf8');
const opened=readFileSync(file+'.raid-opened','utf8');
const settle=()=>new Promise(r=>setTimeout(r,10));
const report=(account='A#1')=>({HTML:'<p>Coach</p>',Game:'C:\\Training',Account:account,Accounts:[{ID:'A#1'},{ID:'B#2'}],HasRaids:true,RaidHTML:opened,InstalledVersion:'1.8.1'});
async function setup(t){
 let listener,tick,account='A#1';const pending=[];
 const dom=new JSDOM(html,{runScripts:'dangerously',pretendToBeVisual:true,beforeParse(w){
  w.setInterval=fn=>{tick=fn;return 1};w.requestAnimationFrame=()=>0;
  w.fetch=()=>assert.fail('unexpected network');
  w.chrome={webview:{addEventListener(_,fn){listener=fn},postMessage(raw){
   const req=JSON.parse(raw);
   if(req.method.startsWith('raid')) {pending.push(req);return;}
   if(req.method==='selectAccount') account=req.params[0];
   queueMicrotask(()=>listener({data:{kind:'native-response',id:req.id,result:['reportRefresh','selectAccount'].includes(req.method)?report(account):null}}));
  }}};
 }});
 t.after(()=>dom.window.close());await settle();await settle();
 return {dom,pending,$:id=>dom.window.document.getElementById(id),tick:()=>tick(),reply(req,result,error){listener({data:{kind:'native-response',id:req.id,result,error}})}};
}
const actions={
 raidPage:({$})=>$('nav-raids').click(),
 raidOpen:({$})=>$('raid-open').click(),
 raidNight:({$,dom})=>{const el=$('raid-night-select');el.value='1';el.dispatchEvent(new dom.window.Event('change',{bubbles:true}));},
 raidOfficerFile:({dom})=>dom.window.document.querySelector('[data-raid-officer-open]').click(),
 raidChooseLog:({dom})=>dom.window.document.querySelector('[data-raid-log]').click(),
 raidChooseOfficer:({dom})=>dom.window.document.querySelector('[data-raid-officer]').click()
};
function locked(x) {
 assert.equal(x.$('nav-raids'),null,'lock reply was discarded; Raids navigation remains');
 assert.equal(x.$('raids-view'),null,'locked page remains in DOM');
 assert.match(x.$('global-status').textContent,/Raids locked: selected account demoted/);
}
// The reviewer's original 12 cases: six routes, both reply orders, older refusal.
// Add the opposite request order as 12 controls: the newer request is refused.
for(const method of Object.keys(actions)) for(const lockFirst of [false,true]) for(const refusedIndex of [0,1]) {
 test('lock ordering latest='+method+' lockFirst='+lockFirst+' refusedIndex='+refusedIndex,async t=>{
  const x=await setup(t);actions.raidPage(x);await settle();actions[method](x);await settle();
  assert.deepEqual(x.pending.map(p=>p.method),['raidPage',method]);
  const deny=()=>x.reply(x.pending[refusedIndex],null,'Raids locked: selected account demoted');
  const allow=()=>x.reply(x.pending[1-refusedIndex],opened);
  if(lockFirst){deny();await settle();locked(x);allow();}else{allow();await settle();deny();}
  await settle();locked(x);
  x.tick();await settle();locked(x); // automatic report is not a new verdict
  x.$('refresh').click();await settle();
  assert.equal(x.pending.length,3,'manual refresh must obtain a later guarded verdict');
  x.reply(x.pending[2],opened);await settle();
  assert.ok(x.$('nav-raids'));assert.ok(x.$('raids-view'));
 });
}
for(const lockFirst of [false,true]) test('lock ordering new refusal beats held re-admission '+lockFirst,async t=>{
 const x=await setup(t);actions.raidPage(x);await settle();actions.raidOpen(x);await settle();
 x.reply(x.pending[0],null,'Raids locked: selected account demoted');await settle();locked(x);
 x.$('refresh').click();await settle();assert.equal(x.pending[2].method,'raidPage');
 const deny=()=>x.reply(x.pending[1],null,'Raids locked: selected account demoted');
 const allow=()=>x.reply(x.pending[2],opened);
 if(lockFirst){deny();await settle();allow();}else{allow();await settle();deny();}
 await settle();locked(x);
});
test('lock ordering refusal stays with its account across a switch',async t=>{
 const x=await setup(t);actions.raidPage(x);await settle();
 x.$('account-select').value='B#2';x.$('account-select').dispatchEvent(new x.dom.window.Event('change',{bubbles:true}));await settle();
 x.reply(x.pending[0],null,'Raids locked: selected account demoted');await settle();
 assert.ok(x.$('nav-raids'),'another account was locked');
 x.$('account-select').value='A#1';x.$('account-select').dispatchEvent(new x.dom.window.Event('change',{bubbles:true}));await settle();locked(x);
});
for(const error of ['Raids locked: selected account demoted','picker failed']) test('lock ordering held readmission cannot restore another account '+error,async t=>{
 const x=await setup(t);actions.raidPage(x);await settle();
 x.reply(x.pending[0],null,'Raids locked: selected account demoted');await settle();
 x.$('refresh').click();await settle();assert.equal(x.pending[1].method,'raidPage');
 x.$('account-select').value='B#2';x.$('account-select').dispatchEvent(new x.dom.window.Event('change',{bubbles:true}));await settle();
 x.reply(x.pending[1],null,error);await settle();
 assert.equal(x.$('account-select').value,'B#2','stale re-admission restored the previous account');
 assert.ok(x.$('nav-raids'));assert.ok(x.$('raids-view'));
});
for(const method of Object.keys(actions)) for(const error of [
 'path picker could not read this path; paste it instead',
 'raid selection changed while choosing a file',
 'invalid argument: expected string',
 'file named Raids locked: unavailable',
 'Raids lockedness is not a lock verdict'
]) test('route error keeps page and paste controls '+method+' '+error,async t=>{
 const x=await setup(t);actions.raidPage(x);await settle();x.reply(x.pending[0],opened);await settle();
 const before=x.$('raids-view');
 actions[method](x);await settle();x.reply(x.pending[1],null,error);await settle();
 assert.equal(x.$('raids-view'),before,'non-lock error removed the current page');
 assert.equal(before.hidden,false);assert.ok(x.$('nav-raids'));
 assert.ok(x.$('raid-log-path'));assert.ok(x.$('raid-officer-path'));
 assert.equal(x.$('global-status').textContent,error);
 x.tick();await settle();
 assert.ok(x.$('nav-raids'));assert.equal(x.$('raids-view'),before);
 assert.equal(x.pending.length,2,'automatic refresh tried to clear a fabricated lock');
 actions.raidPage(x);await settle();x.reply(x.pending[2],opened);await settle();
 assert.equal(x.$('raids-view').hidden,false);
});
test('route error during readmission retains the explicit lock only',async t=>{
 const x=await setup(t);actions.raidPage(x);await settle();
 x.reply(x.pending[0],null,'Raids locked: selected account demoted');await settle();locked(x);
 x.$('refresh').click();await settle();
 x.reply(x.pending[1],null,'path picker could not read this path; paste it instead');await settle();
 assert.equal(x.$('nav-raids'),null);assert.equal(x.$('raids-view'),null);
 assert.match(x.$('global-status').textContent,/path picker could not read/);
 x.tick();await settle();locked(x);
 x.$('refresh').click();await settle();x.reply(x.pending[2],opened);await settle();
 assert.ok(x.$('nav-raids'));
});
