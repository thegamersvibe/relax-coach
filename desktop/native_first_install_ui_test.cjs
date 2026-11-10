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
  assert.equal(count('checkAddon'),0,'no check without a game folder');
  $('setup-browse').click();await settle();await settle();
  assert.equal($('setup-folder').textContent,'C:\\WoW\\_forever_');
  assert.ok($('step-folder').classList.contains('done'));
  assert.equal(count('checkAddon'),1,'one release lookup for the chosen folder');
  assert.equal(count('applyAddon'),0,'nothing installed without a click');
  assert.equal($('setup-offered').textContent,'1.5.1');
  assert.equal($('setup-install').disabled,false);
  assert.match($('setup-status').textContent,/Relax Coach 1\.5\.1 is ready to install\. Close WoW, then click Install\./);
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
    applyAddon:()=>{throw Error('close World of Warcraft before installing or updating addons')},
    checkExtra:()=>({ID:'rankup',Name:'RankUp',InstalledVersion:'0.0.0',Version:'1.0.1',Available:true,Install:true,Status:'RankUp 1.0.1 is ready to install.'}),
    applyExtra:()=>{throw Error('close World of Warcraft before installing or updating addons')}});
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
  assert.equal($('extra-rankup').querySelector('.extra-status').textContent,$('update-status').textContent,'same WoW-closed failure wording');
  assert.equal($('update-status').textContent,'Install failed: close World of Warcraft before installing or updating addons Check again before retrying.');
});

test('first run: copying the app is offered, never automatic',async t=>{
  let placement={State:'offer',Message:'Relax Coach is running from C:\\Users\\p\\Downloads. Optional: copy it.'};
  const {$,count}=await setup(t,{reportRefresh:()=>report('C:\\WoW','1.5.0',installed),appPlacement:()=>placement,
    installAppCopy:()=>({State:'copied',Message:'Relax Coach was copied to C:\\Users\\p\\RelaxCoach\\RelaxCoach.exe. A "Relax Coach" shortcut was added to the desktop.'})});
  assert.equal($('app-placement').hidden,false);
  assert.equal($('app-copy').hidden,false);
  assert.equal(count('installAppCopy'),0,'not copied without a click');
  $('app-copy').click();await settle();
  assert.equal(count('installAppCopy'),1);
  assert.match($('app-placement-text').textContent,/shortcut was added/);
  assert.equal($('app-copy').hidden,true);
  $('app-copy-later').click();
  assert.equal($('app-placement').hidden,true);
  for (const state of ['installed','unavailable']) {
    placement={State:state};
    const r=await setup(t,{reportRefresh:()=>report('C:\\WoW','1.5.0',installed),appPlacement:()=>placement});
    assert.equal(r.$('app-placement').hidden,true,state);
  }
  placement={State:'offer',Message:'x'};
  const later=await setup(t,{reportRefresh:()=>report('C:\\WoW','1.5.0',installed),appPlacement:()=>placement});
  later.$('app-copy-later').click();
  assert.equal(later.$('app-placement').hidden,true);
  assert.equal(later.count('installAppCopy'),0);
});

test('updates disabled: Get started never looks up a release',async t=>{
  const {$,count}=await setup(t,{reportRefresh:()=>report('C:\\WoW','0.0.0',none)},{disabled:true});
  assert.equal($('setup-view').hidden,false);
  assert.equal(count('checkAddon'),0);
  assert.equal($('setup-install').disabled,true);
});
