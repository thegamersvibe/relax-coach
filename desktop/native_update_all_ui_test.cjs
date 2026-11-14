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
const report=(game,version,bundle=installed)=>({HTML:'<p>report</p>',Game:game,Account:'',Accounts:[],InstalledVersion:version,AppVersion:'1.6.0',AppUpdateStatus:'',Bundle:bundle});
const UPDATE=['checkApp','applyApp','checkAddon','applyAddon','extraList','checkExtra','applyExtra'];
const noApp={Version:'1.6.0',InstalledVersion:'1.6.0',Available:false};
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
  assert.equal($('ua-extra-odd'),null,'an unrecognised folder is not touched');
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
  for (const [o,why] of [[bundle({Install:true,InstalledVersion:'0.0.0'}),/first install/],[bundle({Repair:true,Install:true,Version:'1.6.0'}),/repair/],[bundle({Bundle:replaced}),/Journey is not Relax Journey/]]) {
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
  assert.match($('update-all-status').textContent,/needs the new app first/);
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
  assert.match($('global-status').textContent,/Open Updates and click Update everything/);
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
    ['Repair offer',{gameState:()=>'closed',checkAddon:()=>bundle({Repair:true,Install:true,Version:'1.6.0'})},/need attention/],
    ['replaced Journey',{gameState:()=>'closed',checkAddon:()=>bundle({Bundle:replaced})},/need attention/],
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

test('automatic run: bundle already current still updates the installed More addons',async t=>{
  const {$,updates,count}=await setup(t,base({afterAppUpdate:()=>after,gameState:()=>'closed',checkAddon:()=>({...noApp}),
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
  const {$,count}=await setup(t,base({afterAppUpdate:()=>after,gameState:()=>'closed',checkAddon:()=>({...noApp}),
    extraList:()=>extras, checkExtra:id=>({...extraOffer(id),Available:false})}));
  await settle();await settle();
  assert.equal(count('applyAddon')+count('applyExtra'),0);
  assert.equal($('update-all-status').textContent,'Everything is up to date.');
  assert.equal($('update-all').dataset.new,undefined);
});

test('automatic run: bundle current but WoW open updates nothing',async t=>{
  const {$,count}=await setup(t,base({afterAppUpdate:()=>after,gameState:()=>'open',checkAddon:()=>({...noApp}),
    extraList:()=>extras, checkExtra:id=>extraOffer(id), applyExtra:a=>({message:a})}));
  await settle();await settle();
  assert.equal(count('checkExtra')+count('applyExtra'),0);
  assert.match($('update-all-status').textContent,/WoW is open/);
});

test('updates disabled: no afterAppUpdate call and nothing runs',async t=>{
  const {$,count}=await setup(t,base({afterAppUpdate:()=>after,gameState:()=>'closed',checkAddon:()=>bundle()}),{disabled:true});
  await settle();
  assert.equal(count('afterAppUpdate'),0);
  assert.equal(count('applyAddon')+count('checkAddon'),0);
  assert.equal($('update-all').disabled,true);
});

test('not started by an app update: the normal launch check runs, nothing installs',async t=>{
  const {count}=await setup(t,base({afterAppUpdate:()=>({Updated:false}),checkAppStartup:()=>({Available:false})}));
  await settle();
  assert.equal(count('checkAppStartup'),1);
  assert.equal(count('applyAddon')+count('checkAddon')+count('gameState'),0);
});

test('an installed More addon offered as Install (a repair) is left to its own button',async t=>{
  const {$,updates}=await setup(t,base({checkApp:()=>noApp, checkAddon:()=>({...noApp}), extraList:()=>[extras[0]],
    checkExtra:id=>({...extraOffer(id),Install:true})}));
  $('nav-update').click();await settle();
  $('update-all').click();await settle();await settle();
  assert.deepEqual(updates().filter(m=>/Extra/.test(m)),['checkExtra(rankup)']);
  assert.equal(updates().filter(m=>m.startsWith('applyExtra')).length,0);
});
