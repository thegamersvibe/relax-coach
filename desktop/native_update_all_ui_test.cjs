// Relax 1.6 B2: Update everything and the automatic run after an app update.
// Go-rendered shell with a fake native transport; not a Windows GUI pass.
const {test}=require('node:test');
const assert=require('node:assert/strict');
const {readFileSync}=require('node:fs');
const {JSDOM}=require('jsdom');
const html=readFileSync(process.env.NATIVE_HTML_OUT,'utf8');
const settle=()=>new Promise(r=>setTimeout(r,30));
const installed={state:'installed',install:false,present:['ForeverCoach','Journey','Journey-Content','RelaxQuestPlanner'],missing:[]};
const replaced={...installed,state:'replaced'};
const partial={...installed,state:'partial',present:['ForeverCoach','Journey','Journey-Content'],missing:['RelaxQuestPlanner']};
const report=(game,version,bundle=installed)=>({HTML:'<p>report</p>',Game:game,Account:'',Accounts:[],InstalledVersion:version,AppVersion:'1.6.0',AppUpdateStatus:'',Bundle:bundle});
const UPDATE=['checkApp','applyApp','checkAddon','applyAddon','extraList','checkExtra','applyExtra'];
const noApp={Version:'1.6.0',InstalledVersion:'1.6.0',Available:false};
const current={...noApp,Bundle:installed};
const bundle=(over={})=>({Version:'1.6.1',InstalledVersion:'1.6.0',Available:true,Install:false,Repair:false,Bundle:installed,...over});
const extras=[
  {ID:'rankup',Name:'RankUp',InstalledVersion:'1.0.0'},
  {ID:'maps',Name:'Forever Maps',InstalledVersion:'0.0.0'},
  {ID:'odd',Name:'Odd',InstalledVersion:'2.0.0',Problem:'not this addon'},
  {ID:'ahp',Name:'AH Profit',InstalledVersion:'0.6.0'},
];
const extraOffer=id=>({ID:id,Name:id,InstalledVersion:'1.0.0',Version:'9.9.9',Available:true,Install:false,Status:id+' 9.9.9 is ready to update.'});
async function setup(t,handlers={},opts={}){
  const calls=[];let listener;let tick=null;
  const page=opts.disabled?html.replace('data-updates-disabled="false"','data-updates-disabled="true"'):html;
  const dom=new JSDOM(page,{runScripts:'dangerously',pretendToBeVisual:true,beforeParse(w){
    w.setInterval=fn=>{tick=fn;return 1};w.requestAnimationFrame=()=>0;
    w.chrome={webview:{addEventListener(_,fn){listener=fn},postMessage(raw){
      const req=JSON.parse(raw);calls.push(req);
      Promise.resolve().then(async()=>{try{const fn=handlers[req.method];const result=fn?await fn(...req.params):null;listener({data:{kind:'native-response',id:req.id,result}})}catch(e){listener({data:{kind:'native-response',id:req.id,error:e.message}})}});
    }}};w.fetch=()=>assert.fail('network from native JS');
  }});
  t.after(()=>dom.window.close());await settle();await settle();await settle();
  const updates=()=>calls.filter(c=>UPDATE.includes(c.method)).map(c=>c.method+(c.params.length?'('+c.params.join(',')+')':''));
  return {calls,updates,tick:()=>tick(),$:id=>dom.window.document.getElementById(id),count:m=>calls.filter(c=>c.method===m).length};
}
const base=(over={})=>({reportRefresh:()=>report('C:\\WoW','1.6.0'),appPlacement:()=>({State:'installed'}),...over});

test('app update available: checkApp, applyApp and nothing else',async t=>{
  const {$,updates}=await setup(t,base({
    checkApp:()=>({Version:'1.6.1',InstalledVersion:'1.6.0',Available:true}),
    applyApp:()=>({restartReady:true,message:'Restarting…'})}));
  $('nav-update').click();await settle();
  const before=updates().length;
  $('update-all').click();await settle();await settle();
  assert.deepEqual(updates().slice(before),['checkApp','applyApp(1.6.1)']);
  assert.match($('update-all-status').textContent,/close and reopen by itself/);
  assert.match($('ua-app').textContent,/1\.6\.0 → 1\.6\.1: restarting/);
  assert.equal($('update-all').disabled,true,'stays busy while the app restarts');
});

test('no app update: bundle, then only installed More addons, in list order',async t=>{
  const {$,updates}=await setup(t,base({
    checkApp:()=>noApp, checkAddon:()=>bundle(),
    applyAddon:v=>({version:v,message:'Relax Coach '+v+' installed.',backupPath:'C:\\WoW\\RelaxCoachBackups\\x'}),
    extraList:()=>extras, checkExtra:id=>extraOffer(id), applyExtra:arg=>({message:arg+' installed.'})}));
  $('nav-update').click();await settle();
  const before=updates().length;
  $('update-all').click();await settle();await settle();await settle();
  assert.deepEqual(updates().slice(before).filter(m=>m!=='extraList'),
    ['checkApp','checkAddon','applyAddon(1.6.1)','checkExtra(rankup)','applyExtra(rankup@9.9.9)','checkExtra(ahp)','applyExtra(ahp@9.9.9)']);
  assert.match($('update-all-status').textContent,/^Done: the app is up to date; Relax Coach addons 1\.6\.1; RankUp 9\.9\.9; AH Profit 9\.9\.9\./);
  assert.match($('ua-bundle').textContent,/1\.6\.1: updated · backup C:\\WoW\\RelaxCoachBackups\\x/);
  assert.ok($('ua-extra-rankup'));
  assert.equal($('ua-extra-maps'),null,'a More addon that is not installed is not touched');
  assert.match($('ua-extra-odd').textContent,/2\.0\.0: skipped \(not this addon\)/,'an unrecognised folder is named, not touched');
});

test('a failing applyAddon stops the chain: no More addons are checked',async t=>{
  const {$,updates}=await setup(t,base({
    checkApp:()=>noApp, checkAddon:()=>bundle(),
    applyAddon:()=>{throw Error('Close WoW and try again: Windows refused to move a file that is in use. The previous addon folders were restored; nothing was changed.')},
    extraList:()=>extras, checkExtra:id=>extraOffer(id)}));
  $('nav-update').click();await settle();
  $('update-all').click();await settle();await settle();
  assert.equal(updates().filter(m=>m.startsWith('checkExtra')).length,0);
  assert.match($('update-all-status').textContent,/^Stopped at the Relax Coach addons: Close WoW and try again.*Done: the app is up to date\. Not done: More addons\./);
  assert.equal($('update-all').disabled,false,'usable again');
});

test('a failing More addon stops the rest and lists them',async t=>{
  const {$,updates}=await setup(t,base({
    checkApp:()=>noApp, checkAddon:()=>({...noApp,InstalledVersion:'1.6.0'}),
    extraList:()=>extras, checkExtra:id=>extraOffer(id), applyExtra:()=>{throw Error('checksum mismatch')}}));
  $('nav-update').click();await settle();
  $('update-all').click();await settle();await settle();
  assert.deepEqual(updates().filter(m=>/Extra/.test(m)),['checkExtra(rankup)','applyExtra(rankup@9.9.9)']);
  assert.match($('update-all-status').textContent,/Stopped at RankUp: checksum mismatch.*Not done: AH Profit\./);
});

test('first install, repair and a replaced Journey are left to their own buttons',async t=>{
  for (const [o,why] of [[bundle({Install:true,InstalledVersion:'0.0.0'}),/they are not installed yet; use Install below/],[bundle({Repair:true,Install:true,Version:'1.6.0'}),/repair/],[bundle({Bundle:replaced}),/Journey is not Relax Journey/]]) {
    const {$,updates}=await setup(t,base({checkApp:()=>noApp, checkAddon:()=>o, extraList:()=>[]}));
    $('nav-update').click();await settle();
    $('update-all').click();await settle();await settle();
    assert.equal(updates().filter(m=>m.startsWith('applyAddon')).length,0);
    assert.match($('ua-bundle').textContent,why);
  }
});

test('NeedsApp stops the chain and the bundle card says app first',async t=>{
  const {$,updates}=await setup(t,base({checkApp:()=>noApp, checkAddon:()=>bundle({Available:false,NeedsApp:true,Version:'1.7.0'}), extraList:()=>extras}));
  $('nav-update').click();await settle();
  $('check-update').click();await settle();
  assert.match($('update-status').textContent,/Relax Coach 1\.7\.0 needs app 1\.7\.0 first: click Update App above, or Update everything/);
  assert.equal($('apply-update').disabled,true);
  $('update-all').click();await settle();await settle();
  assert.equal(updates().filter(m=>/apply|checkExtra/.test(m)).length,0);
  assert.match($('update-all-status').textContent,/needs app 1\.7\.0 first, and no app update could be installed here\. Download RelaxCoach\.exe from https:\/\/github\.com\/thegamersvibe\/relax-coach\/releases\/latest/);
});

test('every button is off for the whole chain and the 5-second refresh does not run',async t=>{
  let release;const gate=new Promise(r=>release=r);
  const {$,tick,count}=await setup(t,base({checkApp:()=>noApp, checkAddon:()=>bundle(), applyAddon:async v=>{await gate;return {version:v,message:'ok'}}, extraList:()=>[]}));
  $('nav-update').click();await settle();
  $('update-all').click();await settle();await settle();
  const refreshes=count('reportRefresh');
  tick();tick();await settle();
  assert.equal(count('reportRefresh'),refreshes,'no timer refresh during the chain');
  for (const id of ['update-all','check-update','apply-update','check-app-update','apply-app-update','setup-install','setup-browse']) assert.equal($(id).disabled,true,id);
  release();await settle();await settle();
  assert.equal($('update-all').disabled,false);
});

test('results and errors survive the timer and manual refreshes',async t=>{
  const {$,tick}=await setup(t,base({checkApp:()=>noApp, checkAddon:()=>bundle(), applyAddon:()=>{throw Error('disk full')}, extraList:()=>[]}));
  $('nav-update').click();await settle();
  $('update-all').click();await settle();await settle();
  const text=$('update-all-status').textContent;
  assert.match(text,/Stopped at the Relax Coach addons: disk full/);
  tick();await settle();$('refresh').click();await settle();
  assert.equal($('update-all-status').textContent,text);
});

test('U4: the bundle card asks for Check first; the launch notice says Update everything',async t=>{
  const {$}=await setup(t,base({checkAppStartup:()=>({Version:'1.6.1',InstalledVersion:'1.6.0',Available:true})}));
  assert.match($('update-status').textContent,/^Click Check first\./);
  assert.match($('update-notice').textContent,/Open Updates and click Update everything/);
});

// Automatic run after an app update.
const after={Updated:true,From:'1.5.6',Game:'C:\\WoW'};
test('automatic run: all conditions met runs the bundle and More addons once',async t=>{
  const {$,updates,count,tick}=await setup(t,base({afterAppUpdate:()=>after, gameState:()=>'closed',
    checkAddon:()=>bundle(), applyAddon:v=>({version:v,message:'ok'}), extraList:()=>extras, checkExtra:id=>extraOffer(id), applyExtra:a=>({message:a})}));
  await settle();await settle();
  assert.deepEqual(updates().filter(m=>m!=='extraList'),['checkAddon','applyAddon(1.6.1)','checkExtra(rankup)','applyExtra(rankup@9.9.9)','checkExtra(ahp)','applyExtra(ahp@9.9.9)']);
  assert.equal(count('checkApp'),0,'the app step is skipped: it was just updated');
  assert.equal(count('checkAppStartup'),0,'no launch-time release check on that launch');
  assert.equal($('update-view').hidden,false,'shows Updates');
  assert.match($('global-status').textContent,/Relax Coach was updated to 1\.6\.0\. Updating your addons…/);
  assert.match($('update-all-status').textContent,/^Done: Relax Coach addons 1\.6\.1; RankUp 9\.9\.9; AH Profit 9\.9\.9\./);
  tick();await settle();$('refresh').click();await settle();
  assert.equal(count('afterAppUpdate'),1);assert.equal(count('applyAddon'),1,'never runs twice');
});

test('automatic run: each unmet condition installs nothing and marks Update everything',async t=>{
  const cases=[
    ['WoW open',{gameState:()=>'open',checkAddon:()=>bundle()},/WoW is open/],
    ['WoW state unknown',{gameState:()=>'unknown',checkAddon:()=>bundle()},/could not tell whether WoW is running/],
    ['gameState error',{gameState:()=>{throw Error('x')},checkAddon:()=>bundle()},/could not tell whether WoW is running/],
    ['no game folder',{reportRefresh:()=>report('','0.0.0'),gameState:()=>'closed',checkAddon:()=>bundle()},/Choose your game folder/],
    ['Coach not installed',{reportRefresh:()=>report('C:\\WoW','0.0.0'),gameState:()=>'closed',checkAddon:()=>bundle({Install:true})},/not installed/],
    ['NeedsApp',{gameState:()=>'closed',checkAddon:()=>bundle({NeedsApp:true,Version:'1.7.0'})},/were not updated: this release needs app 1\.7\.0 first.*releases\/latest/],
    ['check fails',{gameState:()=>'closed',checkAddon:()=>{throw Error('offline')}},/addon check failed \(offline\)/],
  ];
  for (const [name,over,why] of cases) {
    const {$,count}=await setup(t,base({afterAppUpdate:()=>after,extraList:()=>extras,checkExtra:id=>extraOffer(id),...over}));
    await settle();await settle();
    assert.equal(count('applyAddon')+count('applyExtra')+count('checkExtra'),0,name);
    assert.equal($('update-all').dataset.new,'true',name+': button marked');
    assert.equal($('nav-update').dataset.new,'true',name+': nav marked');
    assert.match($('update-all-status').textContent,why,name);
    assert.match($('global-status').textContent,/Relax Coach was updated to 1\.6\.0\./,name);
  }
});

test('R4: a held bundle is marked and named, never "up to date"; the More addons still update',async t=>{
  const cases=[
    ['Repair offer',bundle({Repair:true,Install:true,Version:'1.6.0'}),/they need a repair; use Repair below/],
    ['replaced Journey, update offered',bundle({Bundle:replaced}),/Journey is not Relax Journey; use Repair below/],
    ['partial, update offered',bundle({Bundle:partial}),/an addon folder is missing: RelaxQuestPlanner; use Repair below/],
    ['partial, nothing offered',{...noApp,Bundle:partial},/an addon folder is missing: RelaxQuestPlanner; use Repair below/],
    ['replaced, nothing offered',{...noApp,Bundle:replaced},/Journey is not Relax Journey; use Repair below/],
  ];
  for (const [name,offer,why] of cases) {
    const {$,count,updates}=await setup(t,base({afterAppUpdate:()=>after,gameState:()=>'closed',checkAddon:()=>offer,
      extraList:()=>extras,checkExtra:id=>extraOffer(id),applyExtra:a=>({message:a})}));
    await settle();await settle();await settle();
    assert.equal(count('applyAddon'),0,name);
    assert.deepEqual(updates().filter(m=>m.startsWith('applyExtra')),['applyExtra(rankup@9.9.9)','applyExtra(ahp@9.9.9)'],name+': More addons still update');
    assert.match($('global-status').textContent,new RegExp('Your Relax Coach addons were not updated: '+why.source+'\\. Checking your More addons…'),name);
    assert.match($('ua-bundle').textContent,why,name);
    assert.doesNotMatch($('ua-bundle').textContent+$('global-status').textContent,/up to date/,name+': no up-to-date claim');
    assert.equal($('update-all').dataset.new,'true',name+': button marked');
    assert.equal($('nav-update').dataset.new,'true',name+': nav marked');
  }
});

test('R3: a new run and a folder change clear every old line; a Problem addon is named',async t=>{
  let list=extras;
  const {$}=await setup(t,base({checkApp:()=>noApp,checkAddon:()=>bundle(),applyAddon:v=>({message:'ok'}),
    extraList:()=>list,checkExtra:id=>extraOffer(id),applyExtra:a=>({message:a}),chooseGame:()=>report('D:\\Other','1.6.0')}));
  $('nav-update').click();await settle();
  $('update-all').click();await settle();await settle();await settle();
  assert.match($('ua-extra-odd').textContent,/skipped \(not this addon\)/);
  assert.match($('update-all-status').textContent,/Skipped: Odd \(see above\)/);
  assert.doesNotMatch($('update-all-status').textContent,/Everything is up to date/);
  // a new run: RankUp is gone from the list, so its old line must go too
  list=[extras[3]];
  $('update-all').click();await settle();await settle();await settle();
  assert.equal($('ua-extra-rankup'),null,'old line cleared at the start of a run');
  assert.equal($('ua-extra-odd'),null);
  assert.ok($('ua-extra-ahp'));
  // a folder change clears the bundle and app lines too
  $('browse-folder').click();await settle();await settle();
  assert.equal($('ua-extra-ahp'),null);
  assert.match($('ua-bundle').textContent,/not checked yet/);
  assert.match($('ua-app').textContent,/not checked yet/);
  // only a Problem addon installed: never "Everything is up to date"
  const p=await setup(t,base({checkApp:()=>noApp,checkAddon:()=>({...current}),extraList:()=>[extras[2]]}));
  p.$('nav-update').click();await settle();
  p.$('update-all').click();await settle();await settle();
  assert.equal(p.$('update-all-status').textContent,'Done: the app is up to date. Skipped: Odd (see above).');
});

test('R2: the app card no longer says addons are unchanged without the follow-up',async t=>{
  const {$}=await setup(t,base());
  const scope=[...$('app-update-status').parentElement.querySelectorAll('.scope')].map(e=>e.textContent).join(' ');
  assert.match(scope,/This step does not touch addons, settings or saves; your addons follow when WoW is closed\./);
});

test('automatic run: bundle already current still updates the installed More addons',async t=>{
  const {$,updates,count}=await setup(t,base({afterAppUpdate:()=>after,gameState:()=>'closed',checkAddon:()=>({...current}),
    extraList:()=>extras, checkExtra:id=>extraOffer(id), applyExtra:a=>({message:a})}));
  await settle();await settle();
  assert.equal(count('applyAddon'),0,'the bundle is current');
  assert.deepEqual(updates().filter(m=>/Extra/.test(m)),['checkExtra(rankup)','applyExtra(rankup@9.9.9)','checkExtra(ahp)','applyExtra(ahp@9.9.9)'],'installed, recognised More addons only');
  assert.match($('global-status').textContent,/Your Relax Coach addons are up to date; checking your More addons…/);
  assert.match($('ua-bundle').textContent,/up to date/);
  assert.match($('update-all-status').textContent,/^Done: RankUp 9\.9\.9; AH Profit 9\.9\.9\./);
  assert.equal($('update-all').dataset.new,undefined,'nothing to mark');
});

test('automatic run: bundle and More addons all current',async t=>{
  const {$,count}=await setup(t,base({afterAppUpdate:()=>after,gameState:()=>'closed',checkAddon:()=>({...current}),
    extraList:()=>extras.filter(e=>e.ID!=='odd'), checkExtra:id=>({...extraOffer(id),Available:false,Version:'1.0.0'})}));
  await settle();await settle();
  assert.equal(count('applyAddon')+count('applyExtra'),0);
  assert.equal($('update-all-status').textContent,'Everything is up to date.');
  assert.equal($('update-all').dataset.new,undefined);
});

test('automatic run: bundle current but WoW open updates nothing',async t=>{
  const {$,count}=await setup(t,base({afterAppUpdate:()=>after,gameState:()=>'open',checkAddon:()=>({...current}),
    extraList:()=>extras, checkExtra:id=>extraOffer(id), applyExtra:a=>({message:a})}));
  await settle();await settle();
  assert.equal(count('checkExtra')+count('applyExtra'),0);
  assert.match($('update-all-status').textContent,/WoW is open/);
});

test('updates disabled (-no-update): the automatic run stays silent',async t=>{
  const {$,count,calls}=await setup(t,base({afterAppUpdate:()=>after,gameState:()=>'closed',checkAddon:()=>bundle(),extraList:()=>extras,checkExtra:id=>extraOffer(id)}),{disabled:true});
  await settle();await settle();
  assert.equal(count('afterAppUpdate'),0);
  assert.equal(count('gameState')+count('checkApp')+count('applyAddon')+count('checkAddon')+count('applyExtra')+count('checkExtra'),0);
  assert.equal($('update-all').disabled,true);
  assert.equal($('update-all').dataset.new,undefined,'no mark');
  assert.equal($('nav-update').dataset.new,undefined,'no nav mark');
  assert.equal($('update-all-status').textContent,'','no status');
  assert.doesNotMatch($('global-status').textContent,/updated to|Update everything/);
});

test('automatic run: gameState is asked again right before each install',async t=>{
  // closed at the first look, then WoW is started before the bundle install.
  let n=0;
  const r=await setup(t,base({afterAppUpdate:()=>after,gameState:()=>(n++===0?'closed':'open'),checkAddon:()=>bundle(),extraList:()=>extras,checkExtra:id=>extraOffer(id)}));
  await settle();await settle();
  assert.equal(r.count('applyAddon')+r.count('applyExtra'),0,'nothing installed');
  assert.match(r.$('update-all-status').textContent,/Stopped at the Relax Coach addons: WoW is open \(it was started during the update\)/);
  assert.equal(r.$('update-all').dataset.new,'true');
  // unknown right before the install also stops; the order is check, ask, install.
  let m=0;
  const u=await setup(t,base({afterAppUpdate:()=>after,gameState:()=>(m++===0?'closed':'unknown'),checkAddon:()=>bundle()}));
  await settle();await settle();
  assert.equal(u.count('applyAddon'),0);
  assert.match(u.$('update-all-status').textContent,/could not tell whether WoW is running/);
  // closed throughout: one ask before the check, one before each install.
  const ok=await setup(t,base({afterAppUpdate:()=>after,gameState:()=>'closed',checkAddon:()=>bundle(),applyAddon:v=>({message:'ok'}),extraList:()=>extras,checkExtra:id=>extraOffer(id),applyExtra:a=>({message:a})}));
  await settle();await settle();
  const seq=ok.calls.map(c=>c.method).filter(x=>/^(gameState|checkAddon|applyAddon|checkExtra|applyExtra)$/.test(x));
  assert.deepEqual(seq,['gameState','checkAddon','gameState','applyAddon','checkExtra','gameState','applyExtra','checkExtra','gameState','applyExtra']);
  // WoW started between two More addons: the first is done, the rest are listed.
  let k=0;
  const mid=await setup(t,base({afterAppUpdate:()=>after,gameState:()=>(k++<3?'closed':'open'),checkAddon:()=>bundle(),applyAddon:v=>({message:'ok'}),extraList:()=>extras,checkExtra:id=>extraOffer(id),applyExtra:a=>({message:a})}));
  await settle();await settle();
  assert.deepEqual(mid.calls.filter(c=>c.method==='applyExtra').map(c=>c.params[0]),['rankup@9.9.9']);
  assert.match(mid.$('update-all-status').textContent,/Stopped at AH Profit: WoW is open.*Not done: AH Profit\./);
});

test('automatic run: a fresh check, the normal apply, no retry, stops at the first failure',async t=>{
  const {$,count,calls}=await setup(t,base({afterAppUpdate:()=>after,gameState:()=>'closed',checkAddon:()=>bundle(),
    applyAddon:()=>{throw Error('disk full')},extraList:()=>extras,checkExtra:id=>extraOffer(id)}));
  await settle();await settle();await settle();
  assert.equal(count('checkAddon'),1,'one fresh check');
  assert.equal(count('applyAddon'),1,'one apply, never retried');
  assert.equal(count('checkExtra')+count('applyExtra'),0,'stopped at the first failure');
  assert.match($('update-all-status').textContent,/Stopped at the Relax Coach addons: disk full/);
  assert.equal(calls.find(c=>c.method==='applyAddon').params[0],'1.6.1','the checked version');
  $('nav-update').click();await settle();await settle();
  assert.equal(count('applyAddon'),1,'no automatic retry on later reloads');
});


test('not started by an app update: the normal launch check runs, nothing installs',async t=>{
  const {count}=await setup(t,base({afterAppUpdate:()=>({Updated:false}),checkAppStartup:()=>({Available:false})}));
  await settle();
  assert.equal(count('checkAppStartup'),1);
  assert.equal(count('applyAddon')+count('checkAddon')+count('gameState'),0);
});

test('an installed More addon offered as Install (a repair) is left to its own button',async t=>{
  const {$,updates}=await setup(t,base({checkApp:()=>noApp, checkAddon:()=>({...current}), extraList:()=>[extras[0]],
    checkExtra:id=>({...extraOffer(id),Install:true})}));
  $('nav-update').click();await settle();
  $('update-all').click();await settle();await settle();
  assert.deepEqual(updates().filter(m=>/Extra/.test(m)),['checkExtra(rankup)']);
  assert.equal(updates().filter(m=>m.startsWith('applyExtra')).length,0);
  assert.match($('ua-extra-rankup').textContent,/skipped \(needs Install from its own button\)/);
  assert.match($('update-all-status').textContent,/^Done: the app is up to date\. Skipped: RankUp/);
});

test('F1: a partial bundle goes to its own button, never Update everything',async t=>{
  const {$,count}=await setup(t,base({checkApp:()=>noApp, checkAddon:()=>bundle({Bundle:partial}), extraList:()=>[]}));
  $('nav-update').click();await settle();
  $('update-all').click();await settle();await settle();
  assert.equal(count('applyAddon'),0);
  assert.match($('ua-bundle').textContent,/left to its own button below \(an addon folder is missing: RelaxQuestPlanner; use Repair below\)/);
  // automatic: the bundle is not installed, the button is marked (More addons: see R4)
  const a=await setup(t,base({afterAppUpdate:()=>after,gameState:()=>'closed',checkAddon:()=>bundle({Bundle:partial}),extraList:()=>extras,checkExtra:id=>extraOffer(id),applyExtra:x=>({message:x})}));
  await settle();await settle();await settle();
  assert.equal(a.count('applyAddon'),0);
  assert.equal(a.$('update-all').dataset.new,'true');
  // a bundle offer without a Bundle state is not trusted either
  const n=await setup(t,base({checkApp:()=>noApp, checkAddon:()=>bundle({Bundle:undefined}), extraList:()=>[]}));
  n.$('nav-update').click();await settle();
  n.$('update-all').click();await settle();await settle();
  assert.equal(n.count('applyAddon'),0);
});

test('F2: WoW started during a slow check stops the automatic install',async t=>{
  let wow='closed';
  const {$,count}=await setup(t,base({afterAppUpdate:()=>after,gameState:()=>wow,
    checkAddon:()=>new Promise(r=>setTimeout(()=>{wow='open';r(bundle());},60)),extraList:()=>extras,checkExtra:id=>extraOffer(id)}));
  for (let i=0;i<8;i++) await settle();
  assert.equal(count('applyAddon')+count('applyExtra'),0);
  assert.match($('update-all-status').textContent,/WoW is open/);
  assert.equal($('update-all').dataset.new,'true');
  // a slow More addon check during which WoW starts
  let w2='closed';
  const m=await setup(t,base({afterAppUpdate:()=>after,gameState:()=>w2,checkAddon:()=>({...current}),extraList:()=>extras,
    checkExtra:id=>new Promise(r=>setTimeout(()=>{w2='open';r(extraOffer(id));},60))}));
  for (let i=0;i<8;i++) await settle();
  assert.equal(m.count('applyExtra'),0);
  assert.match(m.$('update-all-status').textContent,/Stopped at RankUp: WoW is open/);
});

test('F4: unavailable checks read skipped, applyExtra messages are kept, a folder change clears the lines',async t=>{
  const offers={rankup:{...extraOffer('rankup'),Available:false,Version:'',Status:'No release is published yet.'},ahp:extraOffer('ahp')};
  const {$}=await setup(t,base({checkApp:()=>noApp, checkAddon:()=>({...current}), extraList:()=>extras, checkExtra:id=>offers[id],
    applyExtra:()=>({message:'AH Profit 9.9.9 is installed. Restart WoW to load it.'}),
    chooseGame:()=>report('D:\\Other','1.6.0')}));
  $('nav-update').click();await settle();
  $('update-all').click();await settle();await settle();await settle();
  assert.match($('ua-extra-rankup').textContent,/1\.0\.0: skipped \(No release is published yet\.\)/);
  assert.match($('ua-extra-ahp').textContent,/9\.9\.9: updated — AH Profit 9\.9\.9 is installed\. Restart WoW to load it\./);
  assert.match($('update-all-status').textContent,/^Done: the app is up to date; AH Profit 9\.9\.9\. Skipped: Odd, RankUp \(see above\)\./);
  $('browse-folder').click();await settle();await settle();
  assert.equal($('ua-extra-rankup'),null,'old lines cleared');
  assert.equal($('ua-extra-ahp'),null);
  assert.equal($('update-all-status').textContent,'');
});

test('F3: wording on the cards',async t=>{
  const {$}=await setup(t,base());
  assert.match($('app-update-status').textContent,/your addons follow when WoW is closed\./);
  assert.match($('update-status').textContent,/Nothing is downloaded until you click (Install|Update) or Update everything, or after an app update, when WoW is closed\./);
});
