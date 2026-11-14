// Relax 1.6 A3: install step, Settings and the Downloads pointer.
// Go-rendered shell with a fake native transport; not a Windows GUI pass.
const {test}=require('node:test');
const assert=require('node:assert/strict');
const {readFileSync}=require('node:fs');
const {JSDOM}=require('jsdom');
const html=readFileSync(process.env.NATIVE_HTML_OUT,'utf8');
const settle=()=>new Promise(r=>setTimeout(r,30));
const installed={state:'installed',install:false,present:['ForeverCoach','Journey','Journey-Content','RelaxQuestPlanner'],missing:[]};
const report=(game,version,bundle)=>({HTML:'<p>report</p>',Game:game,Account:'',Accounts:[],InstalledVersion:version,AppVersion:'1.6.0',AppUpdateStatus:'',Bundle:bundle});
const T='C:\\Users\\p\\RelaxCoach\\RelaxCoach.exe', D='C:\\Users\\p\\Downloads\\RelaxCoach.exe';
const link=(Where,State,Target='')=>({Where,Path:'x\\Relax Coach.lnk',State,Target});
const offer={State:'offer',Running:D,Target:T,Message:'Relax Coach is running from C:\\Users\\p\\Downloads. Install it in C:\\Users\\p\\RelaxCoach with Desktop and Start menu shortcuts.',Links:[link('desktop','missing'),link('startmenu','missing')]};
const other=(same,links=[link('desktop','ok',T),link('startmenu','ok',T)])=>({State:'other-copy',Running:D,Target:T,SameCopy:same,Message:'Relax Coach is already installed in C:\\Users\\p\\RelaxCoach. This copy runs from C:\\Users\\p\\Downloads. Open the installed copy and use its updates; this copy changes nothing there.',Links:links});
const inst=links=>({State:'installed',Running:T,Target:T,Links:links});
async function setup(t,handlers={}){
  const calls=[];let listener;let tick=null;
  const dom=new JSDOM(html,{runScripts:'dangerously',pretendToBeVisual:true,beforeParse(w){
    w.setInterval=fn=>{tick=fn;return 1};w.requestAnimationFrame=()=>0;
    w.chrome={webview:{addEventListener(_,fn){listener=fn},postMessage(raw){
      const req=JSON.parse(raw);calls.push(req);
      Promise.resolve().then(async()=>{try{const fn=handlers[req.method];const result=fn?await fn(...req.params):null;listener({data:{kind:'native-response',id:req.id,result}})}catch(e){listener({data:{kind:'native-response',id:req.id,error:e.message}})}});
    }}};w.fetch=()=>assert.fail('network from native JS');
  }});
  t.after(()=>dom.window.close());await settle();await settle();
  return {calls,tick:()=>tick(),$:id=>dom.window.document.getElementById(id),count:m=>calls.filter(c=>c.method===m).length,
    order:()=>calls.map(c=>c.method).filter(m=>['installAppCopy','openInstalledApp','repairAppLinks'].includes(m))};
}

test('offer: Get started first with the install step, also when Coach is installed',async t=>{
  const {$,count}=await setup(t,{reportRefresh:()=>report('C:\\WoW','1.5.6',installed),appPlacement:()=>offer});
  assert.equal($('setup-view').hidden,false,'Get started opens for a copy outside the install folder');
  assert.equal($('report-view').hidden,true);
  assert.equal($('step-app').querySelector('h3').textContent,'1. Install Relax Coach on this PC');
  assert.equal($('step-folder').querySelector('h3').textContent.slice(0,2),'2.');
  assert.equal($('step-install').querySelector('h3').textContent.slice(0,2),'3.');
  assert.equal($('step-done').querySelector('h3').textContent.slice(0,2),'4.');
  assert.equal($('setup-app-install').hidden,false);
  assert.equal($('setup-app-install').textContent,'Install and add shortcuts');
  assert.equal($('setup-app-install').className,'primary');
  assert.equal($('setup-app-open').hidden,true);
  assert.equal($('nav-setup').dataset.new,'true','Get started is marked');
  assert.equal($('step-app').classList.contains('done'),false,'no tick before install');
  assert.match($('setup-app-text').textContent,/Desktop and Start menu shortcuts/);
  assert.equal(count('installAppCopy'),0,'nothing copied without a click');
  assert.equal($('app-placement'),null,'the floating box is gone');
});

test('click: installAppCopy then openInstalledApp, in that order',async t=>{
  let opened=false;
  const {$,order}=await setup(t,{reportRefresh:()=>report('C:\\WoW','1.5.6',installed),appPlacement:()=>offer,
    installAppCopy:()=>({...other(true),State:'copied',Message:'Relax Coach was copied to '+T+'. A "Relax Coach" shortcut was added to the Desktop. A "Relax Coach" shortcut was added to the Start menu. Your game folder choice is kept.'}),
    openInstalledApp:()=>{opened=true;return {State:'opened',Target:T}}});
  $('setup-app-install').click();await settle();await settle();
  assert.deepEqual(order(),['installAppCopy','openInstalledApp']);
  assert.ok(opened);
  assert.match($('setup-app-status').textContent,/shortcut was added to the Start menu/,'says what was created');
  assert.match($('setup-app-status').textContent,/Opening the installed Relax Coach/);
  assert.equal($('setup-app-install').disabled,true,'stays busy while the window closes');
});

test('a failed reopen keeps the window and says where the app is',async t=>{
  const {$,order}=await setup(t,{reportRefresh:()=>report('C:\\WoW','1.5.6',installed),appPlacement:()=>offer,
    installAppCopy:()=>({...other(true),State:'copied',Message:'Relax Coach was copied to '+T+'.'}),
    openInstalledApp:()=>{throw Error('blocked by antivirus')}});
  $('setup-app-install').click();await settle();await settle();
  assert.deepEqual(order(),['installAppCopy','openInstalledApp']);
  assert.match($('setup-app-status').textContent,/could not reopen from its folder \(blocked by antivirus\)\. Start it from C:\\Users\\p\\RelaxCoach\\RelaxCoach\.exe next time/);
  assert.equal($('setup-app-open').disabled,false,'usable again');
});

test('a failed copy never tries to open anything',async t=>{
  const {$,order}=await setup(t,{reportRefresh:()=>report('C:\\WoW','1.5.6',installed),appPlacement:()=>offer,
    installAppCopy:()=>{throw Error('could not copy')}});
  $('setup-app-install').click();await settle();await settle();
  assert.deepEqual(order(),['installAppCopy']);
  assert.equal($('setup-app-status').textContent,'Install failed: could not copy');
});

test('other-copy: only "Open the installed Relax Coach" is primary; no copy is possible',async t=>{
  for (const same of [true,false]) {
    const {$,order}=await setup(t,{reportRefresh:()=>report('C:\\WoW','1.5.6',installed),appPlacement:()=>other(same),
      openInstalledApp:()=>({State:'opened',Target:T})});
    assert.equal($('setup-view').hidden,false);
    assert.equal($('setup-app-open').hidden,false);
    assert.equal($('setup-app-open').textContent,'Open the installed Relax Coach');
    assert.equal($('setup-app-install').hidden,true,'no copy button');
    const primaries=[...$('step-app').querySelectorAll('button.primary')].filter(b=>!b.hidden);
    assert.equal(primaries.length,1);
    assert.equal(/may be older/.test($('setup-app-text').textContent),!same,'older note only when the files differ');
    assert.match($('setup-app-text').textContent,/Nothing is overwritten\./);
    assert.equal($('nav-setup').dataset.new,'true');
    assert.equal($('step-app').classList.contains('done'),false,'a second copy is not the installed one');
    $('setup-app-open').click();await settle();await settle();
    assert.deepEqual(order(),['openInstalledApp']);
  }
});

test('installed: step 1 ticked, no nav mark, Coach Report stays first',async t=>{
  const {$}=await setup(t,{reportRefresh:()=>report('C:\\WoW','1.5.6',installed),appPlacement:()=>inst([link('desktop','ok',T),link('startmenu','ok',T)])});
  assert.equal($('report-view').hidden,false);
  assert.ok($('step-app').classList.contains('done'));
  assert.equal($('nav-setup').dataset.new,undefined);
  assert.equal($('setup-app-install').hidden,true);
  assert.equal($('setup-app-open').hidden,true);
});

test('a player who already chose a page is not moved to Get started',async t=>{
  let release;const gate=new Promise(r=>release=r);
  const {$}=await setup(t,{reportRefresh:()=>report('C:\\WoW','1.5.6',installed),appPlacement:async()=>{await gate;return offer}});
  $('nav-update').click();release();await settle();await settle();
  assert.equal($('update-view').hidden,false);
  assert.equal($('nav-setup').dataset.new,'true','still marked');
});

test('Settings: missing enables Repair, which sends no replace names',async t=>{
  let p=inst([link('desktop','missing'),link('startmenu','ok',T)]);
  const {$,calls,order}=await setup(t,{reportRefresh:()=>report('C:\\WoW','1.5.6',installed),appPlacement:()=>p,
    repairAppLinks:list=>{p=inst([link('desktop','ok',T),link('startmenu','ok',T)]);p.Message='A "Relax Coach" shortcut was added to the Desktop. The Start menu shortcut was already there.';return p}});
  $('nav-settings').click();await settle();
  assert.equal($('settings-view').hidden,false);
  assert.equal($('settings-target').textContent,T);
  assert.match($('settings-links').textContent,/Desktop Missing\./);
  assert.match($('settings-links').textContent,/Start menu Opens the installed copy\./);
  assert.equal($('settings-repair').disabled,false);
  $('settings-repair').click();await settle();await settle();
  assert.deepEqual(order(),['repairAppLinks']);
  assert.deepEqual(calls.find(c=>c.method==='repairAppLinks').params,['']);
  assert.match($('settings-status').textContent,/added to the Desktop/);
  assert.equal($('settings-repair').disabled,true,'nothing left to repair');
});

test('Settings: an "other" link needs the confirmation before its name is sent',async t=>{
  let p=inst([link('desktop','other','C:\\Old\\RelaxCoach.exe'),link('startmenu','missing')]);
  const {$,calls}=await setup(t,{reportRefresh:()=>report('C:\\WoW','1.5.6',installed),appPlacement:()=>p,
    repairAppLinks:list=>({...inst([link('desktop',list.includes('desktop')?'ok':'other',T),link('startmenu','ok',T)]),Message:'done '+list})});
  $('nav-settings').click();await settle();
  assert.match($('settings-links').textContent,/Points somewhere else: C:\\Old\\RelaxCoach\.exe/);
  $('settings-repair').click();await settle();
  assert.equal(calls.filter(c=>c.method==='repairAppLinks').length,0,'asks first');
  assert.equal($('settings-confirm').hidden,false);
  assert.match($('settings-confirm-text').textContent,/The Desktop shortcut points at C:\\Old\\RelaxCoach\.exe\. Replace it\?/);
  $('settings-keep').click();await settle();await settle();
  assert.deepEqual(calls.filter(c=>c.method==='repairAppLinks').map(c=>c.params),[['']],'keep: only the missing one is created');
  p=inst([link('desktop','other','C:\\Old\\RelaxCoach.exe'),link('startmenu','ok',T)]);
  $('nav-settings').click();await settle();
  $('settings-repair').click();await settle();
  $('settings-replace').click();await settle();await settle();
  assert.deepEqual(calls.filter(c=>c.method==='repairAppLinks').map(c=>c.params),[[''],['desktop']]);
  assert.equal($('settings-confirm').hidden,true);
});

test('Settings before install: Repair is off and points at step 1',async t=>{
  const {$}=await setup(t,{reportRefresh:()=>report('C:\\WoW','1.5.6',installed),appPlacement:()=>offer});
  $('nav-settings').click();await settle();
  assert.equal($('settings-repair').disabled,true);
  assert.match($('settings-location').textContent,/Get started, step 1/);
});

test('refreshes never wipe a placement result or error',async t=>{
  const p=inst([link('desktop','missing'),link('startmenu','ok',T)]);
  const {$,tick}=await setup(t,{reportRefresh:()=>report('C:\\WoW','1.5.6',installed),appPlacement:()=>p,
    repairAppLinks:()=>{throw Error('Windows blocked writing to the Desktop')}});
  $('nav-settings').click();await settle();
  $('settings-repair').click();await settle();await settle();
  const err=$('settings-status').textContent;
  assert.match(err,/Repair failed: Windows blocked writing to the Desktop/);
  tick();await settle();                       // the 5-second refresh
  $('refresh').click();await settle();         // manual refresh
  $('nav-settings').click();await settle();    // re-reads placement
  assert.equal($('settings-status').textContent,err);
  const r=await setup(t,{reportRefresh:()=>report('C:\\WoW','1.5.6',installed),appPlacement:()=>other(true),openInstalledApp:()=>{throw Error('gone')}});
  r.$('setup-app-open').click();await settle();await settle();
  const msg=r.$('setup-app-status').textContent;
  assert.match(msg,/could not reopen/);
  r.tick();await settle();r.$('refresh').click();await settle();r.$('nav-setup').click();await settle();
  assert.equal(r.$('setup-app-status').textContent,msg);
});

test('every placement button is off while busy',async t=>{
  let release;const gate=new Promise(r=>release=r);
  const {$}=await setup(t,{reportRefresh:()=>report('C:\\WoW','1.5.6',installed),appPlacement:()=>offer,installAppCopy:async()=>{await gate;throw Error('x')}});
  $('setup-app-install').click();await settle();
  for (const id of ['setup-app-install','setup-app-open','settings-repair','settings-replace','settings-keep','setup-browse']) assert.equal($(id).disabled,true,id);
  release();await settle();
});

test('Repair stays off while a repair runs',async t=>{
  let release;const gate=new Promise(r=>release=r);
  const p=inst([link('desktop','missing'),link('startmenu','ok',T)]);
  const {$,count}=await setup(t,{reportRefresh:()=>report('C:\\WoW','1.5.6',installed),appPlacement:()=>p,repairAppLinks:async()=>{await gate;return p}});
  $('nav-settings').click();await settle();
  assert.equal($('settings-repair').disabled,false);
  $('settings-repair').click();await settle();
  assert.equal($('settings-repair').disabled,true,'busy');
  $('settings-repair').click();await settle();
  assert.equal(count('repairAppLinks'),1,'one repair at a time');
  release();await settle();await settle();
  assert.equal($('settings-repair').disabled,false,'free again, the link is still missing');
});
