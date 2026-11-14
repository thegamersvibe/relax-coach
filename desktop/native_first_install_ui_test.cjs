// First install from the app: Go-rendered shell, fake native transport.
// This is not a Windows GUI or WebView2 pass.
const {test}=require('node:test');
const assert=require('node:assert/strict');
const {readFileSync}=require('node:fs');
const {JSDOM}=require('jsdom');
const html=readFileSync(process.env.NATIVE_HTML_OUT,'utf8');
const settle=()=>new Promise(r=>setTimeout(r,30));
const none={state:'none',install:true,present:[],missing:['ForeverCoach','Journey','Journey-Content','RelaxQuestPlanner']};
const installed={state:'installed',install:false,present:['ForeverCoach','Journey','Journey-Content','RelaxQuestPlanner'],missing:[]};
const report=(game,version,bundle)=>({HTML:'<p>report</p>',Game:game,Account:'',Accounts:[],InstalledVersion:version,AppVersion:'1.5.1',AppUpdateStatus:'',Bundle:bundle});
async function setup(t,handlers={},opts={}){
  const calls=[];let listener;
  const page=opts.disabled?html.replace('data-updates-disabled="false"','data-updates-disabled="true"'):html;
  const dom=new JSDOM(page,{runScripts:'dangerously',pretendToBeVisual:true,beforeParse(w){
    w.setInterval=()=>1;w.requestAnimationFrame=()=>0;
    w.chrome={webview:{addEventListener(_,fn){listener=fn},postMessage(raw){
      const req=JSON.parse(raw);calls.push(req);
      Promise.resolve().then(async()=>{try{const fn=handlers[req.method];const result=fn?await fn(...req.params):null;listener({data:{kind:'native-response',id:req.id,result}})}catch(e){listener({data:{kind:'native-response',id:req.id,error:e.message}})}});
    }}};w.fetch=()=>assert.fail('network from native JS');
  }});
  t.after(()=>dom.window.close());await settle();await settle();
  return {calls,$:id=>dom.window.document.getElementById(id),count:m=>calls.filter(c=>c.method===m).length};
}

test('new player: Get started first, then folder, Install, done',async t=>{
  let state=report('','0.0.0',none);
  const {$,calls,count}=await setup(t,{
    reportRefresh:()=>state,
    chooseGame:()=>(state=report('C:\\WoW\\_forever_','0.0.0',none)),
    checkAddon:()=>({Version:'1.5.1',InstalledVersion:'0.0.0',Available:true,Install:true,Bundle:none,Name:'Relax Coach 1.5.1',Notes:'notes'}),
    applyAddon:v=>{state=report('C:\\WoW\\_forever_',v,installed);return {version:v,message:'Coach, Journey, Relax Quest Planner and guide content '+v+' installed; every installed file verified.'}},
    appPlacement:()=>({State:'installed'}),
  });
  assert.equal($('setup-view').hidden,false,'Get started is the first page for a new player');
  assert.equal($('report-view').hidden,true);
  assert.equal($('setup-install').textContent,'Install');
  assert.equal($('apply-update').textContent,'Install');
  assert.equal($('setup-install').disabled,true,'nothing to install before a folder is chosen');
  assert.equal($('setup-browse').className,'primary','no folder yet: choosing it is the main action');
  assert.equal($('setup-folder-found').hidden,true);
  assert.equal(count('checkAddon'),0,'no check without a game folder');
  $('setup-browse').click();await settle();await settle();
  assert.equal($('setup-folder').textContent,'C:\\WoW\\_forever_');
  assert.equal($('setup-folder-found').hidden,false,'found folder is labelled');
  assert.equal($('setup-browse').className,'secondary','the folder button is no longer the main action (U3)');
  assert.equal($('setup-browse').textContent,'Change folder…');
  assert.ok($('step-folder').classList.contains('done'));
  assert.equal(count('checkAddon'),1,'one release lookup for the chosen folder');
  assert.equal(count('applyAddon'),0,'nothing installed without a click');
  assert.equal($('setup-offered').textContent,'1.5.1');
  assert.equal($('setup-install').disabled,false);
  assert.match($('setup-status').textContent,/Relax Coach 1\.5\.1 is ready to install\. Click Install\. WoW can stay open\./);
  $('setup-install').click();await settle();await settle();
  assert.deepEqual(calls.find(c=>c.method==='applyAddon').params,['1.5.1']);
  assert.match($('setup-status').textContent,/1\.5\.1 installed/);
  assert.match($('update-status').textContent,/1\.5\.1 installed/,'Updates card shows the same result');
  assert.ok($('step-install').classList.contains('done'));
  assert.equal($('setup-install').textContent,'Update');
  assert.equal($('setup-install').disabled,true,'offer consumed');
  assert.equal(count('applyExtra')+count('installAppCopy'),0);
  $('setup-report').click();assert.equal($('report-view').hidden,false);
});

test('existing player: Coach Report first, Update label, no automatic check',async t=>{
  const {$,count}=await setup(t,{reportRefresh:()=>report('C:\\WoW\\_forever_','1.5.0',installed)});
  assert.equal($('report-view').hidden,false);
  assert.equal($('setup-view').hidden,true);
  assert.equal($('apply-update').textContent,'Update');
  assert.equal(count('checkAddon'),0);
  $('nav-setup').click();await settle();
  assert.equal($('setup-install').textContent,'Update');
});

test('partial installs say what is missing and what happens',async t=>{
  const partialCoach={state:'partial',install:false,present:['ForeverCoach','Journey'],missing:['Journey-Content','RelaxQuestPlanner']};
  let r=await setup(t,{reportRefresh:()=>report('C:\\WoW','1.5.1',partialCoach),
    checkAddon:()=>({Available:false,InstalledVersion:'1.5.1',Install:false,Bundle:partialCoach,Notes:'No compatible new addon update is available. Your installed version is current or newer and will not be downgraded.'})});
  assert.equal(r.$('report-view').hidden,false,'Coach is installed: not a first install');
  r.$('check-update').click();await settle();
  assert.match(r.$('update-status').textContent,/Missing folders: Journey-Content, RelaxQuestPlanner\./);
  assert.match(r.$('update-status').textContent,/bundle ZIP fallback/);
  assert.equal(r.$('apply-update').disabled,true,'no reinstall is offered');
  const normalJourney={state:'partial',install:true,present:['Journey'],missing:['ForeverCoach','Journey-Content','RelaxQuestPlanner']};
  r=await setup(t,{reportRefresh:()=>report('C:\\WoW','0.0.0',normalJourney),
    checkAddon:()=>({Version:'1.5.1',Available:true,InstalledVersion:'0.0.0',Install:true,Bundle:normalJourney})});
  assert.equal(r.$('setup-view').hidden,false);
  assert.match(r.$('setup-status').textContent,/Already there: Journey; Install moves it into RelaxCoachBackups first\./);
  assert.match(r.$('setup-status').textContent,/Install puts every folder in place\./);
});

test('bundle and More addons use the same verbs and messages',async t=>{
  const list=[{ID:'rankup',Name:'RankUp',Blurb:'',InstalledVersion:'0.0.0'}];
  const {$}=await setup(t,{reportRefresh:()=>report('C:\\WoW','0.0.0',none),extraList:()=>list,
    checkAddon:()=>({Version:'1.5.1',Available:true,InstalledVersion:'0.0.0',Install:true,Bundle:none}),
    applyAddon:()=>{throw Error('Close WoW and try again: Windows refused to move a file that is in use. The previous addon folders were restored; nothing was changed.')},
    checkExtra:()=>({ID:'rankup',Name:'RankUp',InstalledVersion:'0.0.0',Version:'1.0.1',Available:true,Install:true,Status:'RankUp 1.0.1 is ready to install.'}),
    applyExtra:()=>{throw Error('Close WoW and try again: Windows refused to move a file that is in use. The previous addon folders were restored; nothing was changed.')}});
  await settle();
  $('nav-update').click();await settle();
  const row=$('extra-rankup');
  assert.equal(row.querySelector('[data-role=apply]').textContent,$('apply-update').textContent,'same verb');
  assert.equal(row.querySelector('[data-role=check]').textContent,$('check-update').textContent,'same check label');
  row.querySelector('[data-role=check]').click();await settle();
  const extraReady=row.querySelector('.extra-status').textContent, bundleReady=$('update-status').textContent;
  assert.equal(extraReady.replace('RankUp 1.0.1',''),bundleReady.replace('Relax Coach 1.5.1',''),'same ready wording');
  row.querySelector('[data-role=apply]').click();await settle();await settle();
  $('apply-update').click();await settle();await settle();
  assert.equal($('extra-rankup').querySelector('.extra-status').textContent,$('update-status').textContent,'same file-in-use failure wording');
  assert.equal($('update-status').textContent,'Install failed: Close WoW and try again: Windows refused to move a file that is in use. The previous addon folders were restored; nothing was changed. Check again before retrying.');
});

test('updates disabled: Get started never looks up a release',async t=>{
  const {$,count}=await setup(t,{reportRefresh:()=>report('C:\\WoW','0.0.0',none)},{disabled:true});
  assert.equal($('setup-view').hidden,false);
  assert.equal(count('checkAddon'),0);
  assert.equal($('setup-install').disabled,true);
});

test('returning to an earlier folder looks the release up again',async t=>{
  let state=report('C:\\A','0.0.0',none);
  const {$,count}=await setup(t,{reportRefresh:()=>state,setGame:g=>(state=report(g,'0.0.0',none)),
    checkAddon:()=>({Version:'1.5.1',Available:true,Install:true,InstalledVersion:'0.0.0',Bundle:none})});
  const change=async g=>{$('game-path').value=g;const W=$('game-form').ownerDocument.defaultView;$('game-form').dispatchEvent(new W.Event('submit',{bubbles:true,cancelable:true}));await settle();await settle();};
  assert.equal($('setup-install').disabled,false);
  await change('C:\\B');assert.equal($('setup-install').disabled,false);
  await change('C:\\A');
  assert.equal(count('checkAddon'),3);
  assert.equal($('setup-install').disabled,false,'A has an offer again');
});

test('opening Get started before the first report still looks the release up',async t=>{
  let release;const first=new Promise(r=>{release=r;});
  const {$,count}=await setup(t,{reportRefresh:()=>first,checkAddon:()=>({Version:'1.5.1',Available:true,Install:true,InstalledVersion:'0.0.0',Bundle:none})});
  $('nav-setup').click();
  release(report('C:\\A','0.0.0',none));await settle();await settle();
  assert.equal(count('checkAddon'),1);
  assert.equal($('setup-install').disabled,false);
});

test('an incomplete install at the latest version is offered as Install (repair)',async t=>{
  const partial={state:'partial',install:false,present:['ForeverCoach'],missing:['Journey','Journey-Content','RelaxQuestPlanner']};
  const {$,calls}=await setup(t,{reportRefresh:()=>report('C:\\WoW','1.5.0',partial),
    checkAddon:()=>({Version:'1.5.0',Available:true,Install:true,Repair:true,InstalledVersion:'1.5.0',Bundle:partial}),
    applyAddon:v=>({version:v,message:'Relax Coach 1.5.0 installed again to complete an interrupted or incomplete install'})});
  $('check-update').click();await settle();
  assert.equal($('apply-update').textContent,'Install');
  assert.match($('update-status').textContent,/ready to install\. Click Install\. WoW can stay open\. This completes an interrupted or incomplete install/);
  $('apply-update').click();await settle();await settle();
  assert.deepEqual(calls.find(c=>c.method==='applyAddon').params,['1.5.0']);
  assert.match($('update-status').textContent,/installed again/);
});

test('WoW can stay open: ready text, then the /reload or restart advice from the install',async t=>{
  for (const advice of ['Done. Type /reload in game.','Done. Exit WoW completely and start it again so the new files load.']) {
    const list=[{ID:'rankup',Name:'RankUp',Blurb:'',InstalledVersion:'1.0.0'}];
    const {$}=await setup(t,{reportRefresh:()=>report('C:\\WoW','1.5.1',installed),extraList:()=>list,
      checkAddon:()=>({Version:'1.5.2',Available:true,InstalledVersion:'1.5.1',Install:false,Bundle:installed}),
      applyAddon:v=>({version:v,message:'Coach, Journey, Relax Quest Planner and guide content updated together; every installed file verified. '+advice}),
      checkExtra:()=>({ID:'rankup',Name:'RankUp',InstalledVersion:'1.0.0',Version:'1.0.1',Available:true,Install:false,Status:'RankUp 1.0.1 is ready to update.'}),
      applyExtra:()=>({version:'1.0.1',message:'RankUp updated to 1.0.1. The previous version is backed up. '+advice})});
    await settle();
    $('nav-update').click();await settle();
    $('check-update').click();await settle();
    assert.equal($('update-status').textContent.includes('Close WoW'),false,'no close-WoW request before installing');
    assert.match($('update-status').textContent,/ready to update\. Click Update\. WoW can stay open\./);
    const row=$('extra-rankup');
    row.querySelector('[data-role=check]').click();await settle();
    assert.match(row.querySelector('.extra-status').textContent,/ready to update\. Click Update\. WoW can stay open\./);
    $('apply-update').click();await settle();await settle();
    assert.ok($('update-status').textContent.includes(advice),'bundle shows the advice');
    row.querySelector('[data-role=apply]').click();await settle();await settle();
    assert.ok($('extra-rankup').querySelector('.extra-status').textContent.includes(advice),'More addons shows the advice');
  }
  assert.equal(/Close WoW first/.test(html),false,'no static close-WoW text left in the shell');
});

test('Journey replaced by another updater: own text and a Repair button on Updates and Get started',async t=>{
  const replaced={state:'replaced',install:false,present:['ForeverCoach','Journey','Journey-Content','RelaxQuestPlanner'],missing:[]};
  const text='The Journey installed here is not Relax Journey (another updater such as CurseForge, Wago or Journey Guide Manager may have replaced it). Click Repair to install Relax Journey; if you use the normal Journey on purpose, you can ignore this.';
  let state=report('C:\\WoW','1.5.2',replaced);
  const {$,calls}=await setup(t,{reportRefresh:()=>state,
    checkAddon:()=>({Version:'1.5.2',Available:true,Install:true,Repair:true,InstalledVersion:'1.5.2',Bundle:replaced}),
    applyAddon:v=>{state=report('C:\\WoW',v,installed);return {version:v,message:'Relax Journey installed: … kept in the backup, not deleted.',backupPath:'C:\\WoW\\RelaxCoachBackups\\x'}}});
  await settle();
  assert.equal($('update-status').textContent,'The Journey installed here is not Relax Journey (another updater such as CurseForge, Wago or Journey Guide Manager may have replaced it). Click Check; when a release is found, Repair installs Relax Journey; if you use the normal Journey on purpose, you can ignore this.','shown from the folder state before any click, without pointing at the disabled Repair');
  assert.equal($('setup-status').textContent,$('update-status').textContent,'Get started shows it too');
  assert.equal($('apply-update').textContent,'Repair');
  assert.equal($('setup-install').textContent,'Repair');
  $('check-update').click();await settle();
  assert.equal($('update-status').textContent,text+' WoW can stay open.');
  assert.equal($('apply-update').textContent,'Repair');
  assert.equal($('setup-install').textContent,'Repair');
  $('setup-install').click();await settle();await settle();
  assert.deepEqual(calls.find(c=>c.method==='applyAddon').params,['1.5.2']);
  assert.match($('update-status').textContent,/Relax Journey installed/);
  assert.equal($('apply-update').textContent,'Update','back to Update once Relax Journey is restored');
});

test('a newer release over a replaced Journey keeps Update and says why',async t=>{
  const replaced={state:'replaced',install:false,present:['ForeverCoach','Journey','Journey-Content','RelaxQuestPlanner'],missing:[]};
  const {$}=await setup(t,{reportRefresh:()=>report('C:\\WoW','1.5.1',replaced),
    checkAddon:()=>({Version:'1.5.2',Available:true,Install:false,InstalledVersion:'1.5.1',Bundle:replaced})});
  await settle();
  $('check-update').click();await settle();
  assert.equal($('apply-update').textContent,'Update');
  assert.match($('update-status').textContent,/ready to update\. Click Update\. WoW can stay open\. The Journey installed here is not Relax Journey .*Click Update to install Relax Journey with it; if you use the normal Journey on purpose, you can ignore this\./);
  assert.equal(/Click Repair/.test($('update-status').textContent),false,'the button says Update');
});

test('no claim that /reload cannot load new files',()=>{
  assert.equal(/reload is not enough/i.test(html),false);
  assert.equal(/only load at game start/i.test(html),false);
});

// مانع round 2, S4 (his jr_ui_probe.cjs, inverted into the wanted behaviour).
const foreignB={state:'replaced',install:false,present:['ForeverCoach','Journey','Journey-Content','RelaxQuestPlanner'],missing:[]};
test('no compatible release over a foreign Journey: no "Click Repair" next to a disabled Update',async t=>{
  const {$}=await setup(t,{reportRefresh:()=>report('C:\\WoW','1.5.2',foreignB),
    checkAddon:()=>({InstalledVersion:'1.5.2',Available:false,Install:false,Repair:false,Bundle:foreignB,Notes:'No compatible release.'})});
  await settle();
  $('check-update').click();await settle();
  assert.equal($('apply-update').disabled,true);
  assert.equal(/Click Repair/.test($('update-status').textContent),false,$('update-status').textContent);
  assert.match($('update-status').textContent,/not Relax Journey/);
});
test('the foreign-Journey line goes away once a refresh sees Relax Journey again',async t=>{
  let b=foreignB;
  const {$}=await setup(t,{reportRefresh:()=>report('C:\\WoW','1.5.2',b)});
  await settle();
  assert.match($('update-status').textContent,/not Relax Journey/);
  b=installed;
  $('refresh').click();await settle();await settle();
  assert.equal(/not Relax Journey/.test($('update-status').textContent),false,$('update-status').textContent);
  assert.equal($('setup-status').textContent,$('update-status').textContent);
  assert.equal($('apply-update').textContent,'Update');
});
test('an unreadable Journey TOC: neutral note, no Repair',async t=>{
  const unreadable={state:'installed',install:false,present:['ForeverCoach','Journey','Journey-Content','RelaxQuestPlanner'],missing:[],journeyUnreadable:true};
  const {$}=await setup(t,{reportRefresh:()=>report('C:\\WoW','1.5.2',unreadable),
    checkAddon:()=>({InstalledVersion:'1.5.2',Available:false,Install:false,Repair:false,Bundle:unreadable,Notes:'Up to date.'})});
  await settle();
  $('check-update').click();await settle();
  assert.match($('update-status').textContent,/couldn't read Journey's TOC/);
  assert.equal(/Repair/.test($('update-status').textContent),false);
  assert.equal($('apply-update').textContent,'Update');
});

// مانع final round S5 (Astra's tt_ui_probe.cjs): the folder-state note never
// overwrites an operation result or error; N13/N15 wording.
for (const kind of ['check','repair']) for (const unreadableToc of [false,true]) {
  test('S5: a '+kind+' error survives refreshes ('+(unreadableToc?'unreadable TOC':'foreign Journey')+')',async t=>{
    const b=unreadableToc?{...installed,journeyUnreadable:true}:foreignB;
    const err=kind==='check'?'network offline':'Close WoW and try again: Windows refused to move a file that is in use.';
    const {$}=await setup(t,{reportRefresh:()=>report('C:\\WoW','1.5.2',b),
      checkAddon:()=>{if(kind==='check')throw Error(err);return {Version:'1.5.2',Available:true,Install:true,Repair:!unreadableToc,InstalledVersion:'1.5.2',Bundle:b}},
      applyAddon:()=>{throw Error(err)}});
    await settle();
    $('check-update').click();await settle();
    if(kind!=='check'){$('apply-update').click();await settle();await settle();}
    const text=$('update-status').textContent;
    assert.ok(text.includes(err),text);
    for(let i=0;i<3;i++){$('refresh').click();await settle();await settle();}
    assert.equal($('update-status').textContent,text,'a refresh erased the error');
    assert.equal($('setup-status').textContent,text);
  });
}
test('N13: with updates disabled the foreign-Journey note does not point at Repair',async t=>{
  const page=html.replace('data-updates-disabled="false"','data-updates-disabled="true"');
  assert.notEqual(page,html);
  const {JSDOM:D}=require('jsdom');let listener;
  const dom=new D(page,{runScripts:'dangerously',pretendToBeVisual:true,beforeParse(w){
    w.setInterval=()=>1;w.requestAnimationFrame=()=>0;w.fetch=()=>assert.fail('network');
    w.chrome={webview:{addEventListener(_,fn){listener=fn},postMessage(raw){const req=JSON.parse(raw);Promise.resolve().then(()=>listener({data:{kind:'native-response',id:req.id,result:req.method==='reportRefresh'?report('C:\\WoW','1.5.2',foreignB):null}}))}}};
  }});
  t.after(()=>dom.window.close());await settle();await settle();
  const s=dom.window.document.getElementById('update-status').textContent;
  assert.match(s,/Updates are disabled by -no-update, so Repair is not available here/);
  assert.equal(/Click Repair/.test(s),false);
});
test('N15: unreadable TOC with a newer release: no "Nothing is offered" next to Click Update',async t=>{
  const unreadable={...installed,journeyUnreadable:true};
  const {$}=await setup(t,{reportRefresh:()=>report('C:\\WoW','1.5.2',unreadable),
    checkAddon:()=>({Version:'1.5.3',Available:true,Install:false,Repair:false,InstalledVersion:'1.5.2',Bundle:unreadable})});
  await settle();
  $('check-update').click();await settle();
  assert.match($('update-status').textContent,/Click Update.*couldn't read Journey's TOC/);
  assert.equal(/Nothing is offered/.test($('update-status').textContent),false);
});
