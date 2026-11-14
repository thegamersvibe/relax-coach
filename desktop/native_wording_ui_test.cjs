// Relax 1.6 wording and accessibility (U6-U11, U13 text, U18-U20).
// Go-rendered shell with a fake native transport; not a Windows GUI pass.
const {test}=require('node:test');
const assert=require('node:assert/strict');
const {readFileSync}=require('node:fs');
const {JSDOM}=require('jsdom');
const html=readFileSync(process.env.NATIVE_HTML_OUT,'utf8');
const settle=()=>new Promise(r=>setTimeout(r,30));
const installed={state:'installed',install:false,present:['ForeverCoach','Journey','Journey-Content','RelaxQuestPlanner'],missing:[]};
const report=(game,version)=>({HTML:'',Game:game,Account:'',Accounts:[],InstalledVersion:version,AppVersion:'1.6.0',AppUpdateStatus:'',Bundle:installed});
async function setup(t,handlers={}){
  const calls=[];let listener;const timers=[];
  const dom=new JSDOM(html,{runScripts:'dangerously',pretendToBeVisual:true,beforeParse(w){
    w.setInterval=()=>1;w.requestAnimationFrame=()=>0;
    const realTimeout=w.setTimeout.bind(w);
    // Long page timers (the short-message clear) run only when the test says so.
    w.setTimeout=(fn,ms,...a)=>ms>=1000?(timers.push(fn),timers.length):realTimeout(fn,ms,...a);
    w.chrome={webview:{addEventListener(_,fn){listener=fn},postMessage(raw){
      const req=JSON.parse(raw);calls.push(req);
      Promise.resolve().then(async()=>{try{const fn=handlers[req.method];const result=fn?await fn(...req.params):null;listener({data:{kind:'native-response',id:req.id,result}})}catch(e){listener({data:{kind:'native-response',id:req.id,error:e.message}})}});
    }}};w.fetch=()=>assert.fail('network from native JS');
  }});
  t.after(()=>dom.window.close());await settle();await settle();
  const d=dom.window.document;
  return {calls,d,w:dom.window,runTimers:()=>timers.splice(0).forEach(f=>f()),$:id=>d.getElementById(id)};
}
const base=(over={})=>({reportRefresh:()=>report('C:\\WoW','1.6.0'),appPlacement:()=>({State:'installed'}),...over});

test('U6: release notes show as plain text, not Markdown',async t=>{
  const notes='## What\'s new\n- **Campfire:** two songs\n- See `README` and [the guide](https://example.com/x)\n__Done__';
  const {$}=await setup(t,base({checkAddon:()=>({Version:'1.6.1',InstalledVersion:'1.6.0',Available:true,Bundle:installed,Notes:notes}),
    checkApp:()=>({Version:'1.6.1',InstalledVersion:'1.6.0',Available:true,Notes:notes})}));
  $('nav-update').click();await settle();
  $('check-update').click();await settle();
  $('check-app-update').click();await settle();
  for (const id of ['release-notes','app-release-notes']) {
    const text=$(id).textContent;
    assert.equal(text,"What's new\n• Campfire: two songs\n• See README and the guide\nDone",id);
    assert.equal($(id).children.length,0,id+': still plain text, no markup');
  }
});

test('U7: short confirmations clear; errors stay; the update notice has its own line',async t=>{
  let fail=false;
  const {$,runTimers}=await setup(t,{reportRefresh:()=>{if(fail)throw Error('save unreadable');return report('C:\\WoW','1.6.0')},appPlacement:()=>({State:'installed'}),
    checkAppStartup:()=>({Version:'1.6.1',InstalledVersion:'1.6.0',Available:true})});
  await settle();
  assert.match($('update-notice').textContent,/1\.6\.1 is available/);
  $('refresh').click();await settle();
  assert.match($('global-status').textContent,/Report refreshed/);
  runTimers();
  assert.equal($('global-status').textContent,'','the confirmation clears');
  assert.match($('update-notice').textContent,/1\.6\.1 is available/,'the notice is untouched');
  fail=true;$('refresh').click();await settle();
  runTimers();
  assert.match($('global-status').textContent,/Could not refresh the report: save unreadable/,'errors stay');
});

test('U8: the app update progress text is plain',async t=>{
  let release;const gate=new Promise(r=>release=r);
  const {$}=await setup(t,base({checkApp:()=>({Version:'1.6.1',InstalledVersion:'1.6.0',Available:true}),applyApp:async()=>{await gate;return {restartReady:false,message:'x'}}}));
  $('nav-update').click();await settle();
  $('check-app-update').click();await settle();
  $('apply-app-update').click();await settle();
  const text=$('app-update-status').textContent;
  assert.match(text,/Relax Coach will close and reopen by itself\. Don't close it\./);
  assert.doesNotMatch(text,/trusted worker|handoff/);
  release();await settle();
});

test('U9-U11: short fine print, current footer, "Latest" labels',async t=>{
  const {d}=await setup(t,base({extraList:()=>[{ID:'rankup',Name:'RankUp',InstalledVersion:'1.0.0'}]}));
  d.getElementById('nav-update').click();await settle();
  const text=d.body.textContent;
  for (const old of ['Offered','Update App remains separate','private bundle','From 1.4.2','Legacy packages','trusted worker'])
    assert.equal(text.includes(old),false,old);
  for (const p of d.querySelectorAll('#update-view .scope')) assert.ok((p.textContent.match(/\. /g)||[]).length===0,'one sentence: '+p.textContent);
  assert.match(text,/Latest app/);assert.match(text,/Latest addons/);
  assert.equal(d.querySelector('#extra-rankup .versions div:nth-child(2) dt').textContent,'Latest');
  const go=await setup(t,{appPlacement:()=>({State:'installed'})}); // the Go-rendered report keeps its footer
  assert.match(go.d.querySelector('.foot').textContent,/Updates installs the app, the Relax Coach addons/);
  assert.equal(go.d.body.textContent.includes('private bundle'),false);
});

test('U18: report filters expose which one is on',async t=>{
  const {d}=await setup(t,{appPlacement:()=>({State:'installed'})}); // keeps the Go-rendered report
  const buttons=[...d.querySelectorAll('.filters button')];
  assert.deepEqual(buttons.map(b=>b.getAttribute('aria-pressed')),['true','false','false','false','false']);
  buttons[2].click();
  assert.deepEqual(buttons.map(b=>b.getAttribute('aria-pressed')),['false','false','true','false','false']);
});

test('U19: gear slots open with Enter and Space and expose their state',async t=>{
  const {d,w}=await setup(t,{appPlacement:()=>({State:'installed'})});
  const slot=d.querySelector('.slot');
  assert.equal(slot.getAttribute('role'),'button');
  assert.equal(slot.getAttribute('aria-expanded'),'false');
  slot.dispatchEvent(new w.KeyboardEvent('keydown',{key:'Enter',bubbles:true}));
  assert.ok(slot.classList.contains('open'));assert.equal(slot.getAttribute('aria-expanded'),'true');
  slot.dispatchEvent(new w.KeyboardEvent('keydown',{key:' ',bubbles:true}));
  assert.equal(slot.classList.contains('open'),false);assert.equal(slot.getAttribute('aria-expanded'),'false');
  slot.dispatchEvent(new w.KeyboardEvent('keydown',{key:'a',bubbles:true}));
  assert.equal(slot.getAttribute('aria-expanded'),'false','other keys do nothing');
  slot.click();assert.equal(slot.getAttribute('aria-expanded'),'true','a click keeps the state in step');
});

test('U20: disabled main buttons say why',async t=>{
  const {$}=await setup(t,base({checkAddon:()=>({Version:'1.7.0',InstalledVersion:'1.6.0',Available:false,NeedsApp:true,Bundle:installed})}));
  assert.equal($('apply-update').title,'Click Check first.');
  assert.equal($('apply-app-update').title,'Click Check App Update first.');
  $('nav-update').click();await settle();
  $('check-update').click();await settle();
  assert.equal($('apply-update').title,'Update the app first.');
  const none=await setup(t,{reportRefresh:()=>report('','0.0.0'),appPlacement:()=>({State:'offer'})});
  assert.equal(none.$('setup-install').title,'Choose your game folder first (step 2).');
  assert.equal(none.$('settings-repair').title,'Install Relax Coach first (Get started, step 1).');
  assert.equal(none.$('check-update').title,'','enabled buttons carry no reason');
  const ready=await setup(t,base({checkAddon:()=>({Version:'1.6.1',InstalledVersion:'1.6.0',Available:true,Bundle:installed})}));
  ready.$('nav-update').click();await settle();ready.$('check-update').click();await settle();
  assert.equal(ready.$('apply-update').disabled,false);
  assert.equal(ready.$('apply-update').title,'','an enabled main button carries no reason');
});
