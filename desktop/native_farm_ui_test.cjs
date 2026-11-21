// Farming page (AH Profit): Go-rendered shell and fragment, fake native transport.
// This is not a Windows GUI or WebView2 pass.
const {test}=require('node:test');
const assert=require('node:assert/strict');
const {readFileSync}=require('node:fs');
const {JSDOM}=require('jsdom');
const html=readFileSync(process.env.NATIVE_HTML_OUT,'utf8');
const farm=readFileSync(process.env.FARM_HTML_OUT,'utf8');
const nameCoverage=JSON.parse(readFileSync(process.env.FARM_HTML_OUT+'.names.json','utf8'));
const settle=()=>new Promise(r=>setTimeout(r,30));
const fire=el=>{const E=el.ownerDocument.defaultView.Event;el.dispatchEvent(new E('change',{bubbles:true}));};
const installed={state:'installed',install:false,present:['ForeverCoach','Journey','Journey-Content','RelaxQuestPlanner'],missing:[]};
const report=(account,hasFarm,farmHTML)=>({HTML:'<p>report '+account+'</p>',Game:'C:\\WoW\\_forever_',Account:account,Accounts:[{ID:'A#1'},{ID:'B#2'}],InstalledVersion:'1.5.2',AppVersion:'1.5.2',AppUpdateStatus:'',Bundle:installed,HasFarm:hasFarm,FarmHTML:farmHTML||''});
async function setup(t,handlers){
  let listener;
  const dom=new JSDOM(html,{runScripts:'dangerously',pretendToBeVisual:true,beforeParse(w){
    w.setInterval=()=>1;w.requestAnimationFrame=()=>0;
    w.chrome={webview:{addEventListener(_,fn){listener=fn},postMessage(raw){
      const req=JSON.parse(raw);
      Promise.resolve().then(async()=>{try{const fn=handlers[req.method];const result=fn?await fn(...req.params):null;listener({data:{kind:'native-response',id:req.id,result}})}catch(e){listener({data:{kind:'native-response',id:req.id,error:e.message}})}});
    }}};w.fetch=()=>assert.fail('network from native JS');
  }});
  t.after(()=>dom.window.close());await settle();await settle();
  const $=id=>dom.window.document.getElementById(id);
  return {$,dom,shown:()=>[...$('farm-sell').querySelectorAll('tr[data-house]')].filter(r=>!r.hidden).map(r=>r.dataset.house)};
}

test('item names link to Forever through the external browser bridge',async t=>{
  const opened=[];
  const {$,dom}=await setup(t,{reportRefresh:()=>report('B#2',true,farm),openExternal:url=>opened.push(url)});
  $('nav-farm').click();
  for (const [card,id] of [['farm-sell',2772],['farm-hot',2772],['farm-fallers',8001]]) {
    const link=$(card).querySelector('td:first-child a');
    assert.ok(link,card+' has an item-name link');
    const url='https://www.wowhead.com/forever/item='+id;
    assert.equal(link.getAttribute('href'),url);
    assert.ok(link.textContent.trim(),'the name itself is clickable');
    const click=new dom.window.MouseEvent('click',{bubbles:true,cancelable:true,button:0});
    assert.equal(link.dispatchEvent(click),false,'native navigation prevented');
    await settle();
    assert.equal(opened.at(-1),url,'uses openExternal');
  }
  assert.equal(dom.window.document.querySelectorAll('script[src],link[rel="stylesheet"]').length,0,'no remote JS or CSS');
});

test('Wowhead setting defaults on and saves the off choice through native settings',async t=>{
  const saved=[];
  const state={...report('B#2',true,farm),WowheadNames:true};
  const {$}=await setup(t,{reportRefresh:()=>state,setWowheadNames:enabled=>{saved.push(enabled);state.WowheadNames=enabled==='true';return state}});
  $('nav-settings').click();
  const toggle=$('wowhead-names');
  assert.ok(toggle,'online-name preference in Settings');
  assert.match(toggle.closest('label').textContent,/Look up missing item names online \(Wowhead\)/);
  assert.equal(toggle.checked,true);
  toggle.checked=false;fire(toggle);await settle();await settle();
  assert.deepEqual(saved,['false']);assert.equal(toggle.checked,false);assert.equal(toggle.disabled,false);
  $('refresh').click();await settle();await settle();
  assert.equal(toggle.checked,false,'report refresh preserves persisted opt-out');
});

test('Wowhead opt-out is loaded on startup and a failed write restores it',async t=>{
  const {$}=await setup(t,{reportRefresh:()=>({...report('B#2',true,farm),WowheadNames:false}),setWowheadNames:()=>{throw new Error('Test settings failure')}});
  const toggle=$('wowhead-names');assert.ok(toggle);
  assert.equal(toggle.checked,false);
  toggle.checked=true;fire(toggle);await settle();await settle();
  assert.equal(toggle.checked,false);assert.equal(toggle.disabled,false);
  assert.match($('wowhead-names-status').textContent,/Test settings failure/);
});

// Regression from the held native-reply probe: a setting save must not cross
// the same account boundary already guarded by ordinary report refreshes.
for (const ordering of ['during-switch','after-switch']) for (const outcome of ['success','failure']) test('preference reply cannot restore account A '+ordering+' '+outcome,async t=>{
  let finishPreference,finishSelection;
  // A failed write leaves the stored preference on in the next account report.
  const state=account=>({...report(account,true,'<p>Synthetic farm '+account+'</p>'),WowheadNames:outcome==='failure'});
  const {$}=await setup(t,{
    reportRefresh:()=>({...state('A#1'),WowheadNames:true}),
    setWowheadNames:()=>new Promise((resolve,reject)=>{finishPreference=result=>outcome==='success'?resolve(result):reject(new Error('Old request failed'))}),
    selectAccount:()=>new Promise(r=>{finishSelection=r})
  });
  $('wowhead-names').checked=false;fire($('wowhead-names'));await settle();
  assert.equal(typeof finishPreference,'function','preference request pending');
  assert.equal($('account-select').disabled,false,'account selector stays usable during save');
  $('account-select').value='B#2';fire($('account-select'));await settle();
  assert.equal(typeof finishSelection,'function','account switch pending');
  assert.equal($('farm-body').textContent,'','old farm initially cleared');
  assert.equal($('report-body').textContent,'','old report initially cleared');
  if (ordering==='after-switch') { finishSelection(state('B#2'));await settle(); }
  finishPreference(state('A#1'));await settle();await settle();
  assert.equal($('account-select').value,'B#2','late preference reply must not restore A');
  assert.equal($('farm-body').textContent,ordering==='during-switch'?'':'Synthetic farm B#2');
  assert.equal($('report-body').textContent,ordering==='during-switch'?'':'report B#2');
  assert.equal($('wowhead-names').checked,outcome==='failure','successful replies own the app-wide setting; failed replies use the latest report');
  assert.equal($('wowhead-names-status').textContent,'','stale replies leave no success, error or saving message');
  if (ordering==='during-switch') { finishSelection(state('B#2'));await settle(); }
  assert.equal($('account-select').value,'B#2');
  assert.equal($('farm-body').textContent,'Synthetic farm B#2');
  assert.equal($('report-body').textContent,'report B#2');
  assert.equal($('account-select').disabled,false);
  assert.equal($('wowhead-names').disabled,false);
  assert.equal($('wowhead-names').checked,outcome==='failure','settled checkbox matches the stored preference');
});

// The backend serializes writes/report snapshots, but native reply delivery can
// interleave. Adapt the held-service probe with a single app-wide stored value,
// separating backend completion from transport delivery (24 cases per action).
const preferenceOrders = [
  ['save','saveReply','select','selectReply'],
  ['save','select','saveReply','selectReply'],
  ['save','select','selectReply','saveReply'],
  ['select','selectReply','save','saveReply'],
  ['select','save','selectReply','saveReply'],
  ['select','save','saveReply','selectReply']
];
for (const action of ['account','folder','folder-cancel']) for (const initial of [false,true]) for (const outcome of ['success','failure']) for (const order of preferenceOrders) test('preference backend order '+[action,initial,outcome,order.join('/')].join(' '),async t=>{
  let stored=initial,account='A#1',game='C:\\WoW\\_forever_',saveResult,selectionResult;
  const steps={};
  const snapshot=()=>({...report(account,true,'<p>Synthetic farm '+account+' '+game+'</p>'),Game:game,WowheadNames:stored});
  const select=(nextAccount,nextGame)=>new Promise(resolve=>{
    steps.select=()=>{
      account=nextAccount;game=nextGame;
      selectionResult=action==='folder-cancel'?null:snapshot();
    };
    steps.selectReply=()=>resolve(selectionResult);
  });
  const {$,dom}=await setup(t,{
    reportRefresh:snapshot,
    setWowheadNames:value=>new Promise((resolve,reject)=>{
      steps.save=()=>{if(outcome==='success')stored=value==='true';saveResult=snapshot();};
      steps.saveReply=()=>outcome==='success'?resolve(saveResult):reject(new Error('Synthetic settings failure'));
    }),
    selectAccount:next=>select(next,game),
    setGame:next=>select(account,next),
    chooseGame:()=>select(account,game)
  });
  const toggle=$('wowhead-names');
  assert.equal(toggle.checked,initial);
  toggle.checked=!initial;fire(toggle);await settle();
  if(action==='account'){$('account-select').value='B#2';fire($('account-select'));}
  else if(action==='folder'){$('game-path').value='D:\\WoW\\_forever_';$('game-form').dispatchEvent(new dom.window.Event('submit',{bubbles:true,cancelable:true}));}
  else $('browse-folder').click();
  await settle();
  for(const step of order){
    assert.equal(typeof steps[step],'function',step+' admitted');
    const body=$('report-body').innerHTML,farmBody=$('farm-body').innerHTML;
    steps[step]();await settle();
    if(step==='saveReply'){
      assert.equal(toggle.checked,outcome==='success'?saveResult.WowheadNames:initial,'reply reconciles the setting');
      assert.equal($('report-body').innerHTML,body,'save cannot restore an earlier report');
      assert.equal($('farm-body').innerHTML,farmBody,'save cannot restore an earlier farm');
    }
  }
  assert.equal(toggle.checked,stored,'settled checkbox matches the app-wide backend');
  assert.equal(toggle.disabled,false);
  assert.equal($('account-select').value,account);
  assert.equal($('folder-label').textContent,game);
  assert.equal($('report-body').textContent,'report '+account);
  assert.equal($('farm-body').textContent,'Synthetic farm '+account+' '+game);
  assert.equal($('report-body').hidden,false);
  assert.equal($('farm-body').hidden,false);
  if(action==='folder-cancel'&&outcome==='failure')assert.match($('wowhead-names-status').textContent,/Could not save preference: Synthetic settings failure/,'cancelled chooser must retain the current save error');
  else assert.equal($('wowhead-names-status').textContent,'','dropped reply leaves no stale status');
});

test('failed preference for the current account is reported after switching away and back',async t=>{
  let failPreference;
  const state=account=>({...report(account,true,'<p>Synthetic farm '+account+'</p>'),WowheadNames:true});
  const {$}=await setup(t,{
    reportRefresh:()=>state('A#1'),
    setWowheadNames:()=>new Promise((_,reject)=>{failPreference=()=>reject(new Error('Current settings failure'))}),
    selectAccount:account=>state(account)
  });
  const toggle=$('wowhead-names');
  toggle.checked=false;fire(toggle);await settle();
  assert.equal(typeof failPreference,'function');
  for (const account of ['B#2','A#1']) {
    $('account-select').value=account;fire($('account-select'));await settle();
    assert.equal($('report-body').textContent,'report '+account);
  }
  failPreference();await settle();await settle();
  assert.equal(toggle.checked,true,'failed opt-out cannot claim lookups are off');
  assert.equal(toggle.disabled,false);
  assert.match($('wowhead-names-status').textContent,/Could not save preference: Current settings failure/);
  assert.equal($('account-select').value,'A#1');
  assert.equal($('farm-body').textContent,'Synthetic farm A#1');
});

test('no AH Profit save: no Farming entry' ,async t=>{
  const {$}=await setup(t,{reportRefresh:()=>report('A#1',false)});
  assert.equal($('nav-farm').hidden,true);
  assert.equal($('farm-view').hidden,true);
});

test('AH Profit save: Farming entry, page, and the current house first',async t=>{
  const {$,shown}=await setup(t,{reportRefresh:()=>report('B#2',true,farm)});
  assert.equal($('nav-farm').hidden,false);
  $('nav-farm').click();
  assert.equal($('farm-view').hidden,false);
  assert.equal($('report-view').hidden,true);
  assert.equal($('nav-farm').getAttribute('aria-current'),'page');
  for (const id of ['farm-gold','farm-sessions','farm-sell','farm-gathering','farm-auctions']) assert.ok($(id),id);
  assert.match($('farm-gold').textContent,/Earned/);
  assert.match($('farm-sessions').textContent,/Elwynn Forest/);
  assert.ok(!$('farm-body').innerHTML.includes('<b>Tester</b>'),'session names stay escaped');
  assert.equal($('farm-house').value,'Stormrage|Alliance');
  assert.ok(shown().length>0 && shown().every(h=>h==='Stormrage|Alliance'),'only the current house');
  $('farm-house').value='Stormrage|Horde';
  fire($('farm-house'));
  assert.deepEqual(shown(),['Stormrage|Horde'],'another house is one choice away');
  $('farm-house').value='Other Realm|Alliance';
  fire($('farm-house'));
  assert.deepEqual(shown(),[]);
  const goldRows=[...$('farm-gold').querySelectorAll('tr'),...$('farm-sessions').querySelectorAll('tr')];
  assert.ok(goldRows.length>4 && goldRows.every(r=>!r.hidden),'Gold and Sessions are for the whole account, whatever the house');
  assert.equal($('farm-body').querySelector('.farm-empty-house').hidden,false,'says so when a house has no bank items');
  assert.ok(!$('farm-body').innerHTML.includes('<b>Linen</b>'),'save text stays escaped');
});

for (const [fixture,want] of [
  ['absent','2 of 4 item names known (update Relax Coach and visit the auction house to name more)'],
  ['empty','2 of 4 item names known'],
  ['partial','3 of 4 item names known'],
  ['quality only','2 of 4 item names known'],
  ['capped','2 of 4 item names known'],
  ['all sources',null],
  ['complete Coach',null],
  ['absent but covered',null],
  ['no history',null]
]) test('item-name coverage: '+fixture,async t=>{
  assert.equal(typeof nameCoverage[fixture],'string','Go-rendered fixture exists');
  const {$}=await setup(t,{reportRefresh:()=>report('B#2',true,nameCoverage[fixture])});
  $('nav-farm').click();
  assert.equal($('farm-view').hidden,false);
  const notes=$('farm-body').querySelectorAll('#farm-names-filling');
  assert.equal(notes.length,want===null?0:1,'one note only while coverage is incomplete');
  if (want!==null) assert.equal(notes[0].textContent,want);
});

test('coverage is replaced on account switch, not borrowed from the old account',async t=>{
  let release;
  const {$}=await setup(t,{
    reportRefresh:()=>report('B#2',true,nameCoverage.capped),
    selectAccount:()=>new Promise(r=>{release=()=>r(report('A#1',true,nameCoverage.absent))})
  });
  $('nav-farm').click();
  assert.equal($('farm-names-filling').textContent,'2 of 4 item names known');
  $('account-select').value='A#1';fire($('account-select'));await settle();
  assert.equal($('farm-names-filling'),null,'old coverage is cleared while switching');
  release();await settle();await settle();
  assert.equal($('farm-names-filling').textContent,'2 of 4 item names known (update Relax Coach and visit the auction house to name more)');
});

test('hot items follow the chosen house; the bank-empty line counts only its own card',async t=>{
  const {$}=await setup(t,{reportRefresh:()=>report('B#2',true,farm)});
  $('nav-farm').click();
  const hot=()=>[...$('farm-hot').querySelectorAll('tr[data-house]')].filter(r=>!r.hidden);
  assert.ok(hot().length>1 && hot().every(r=>r.dataset.house==='Stormrage|Alliance'),'only the current house');
  assert.match($('farm-hot').textContent,/\+250%/);
  assert.ok($('farm-hot').querySelector('svg path').getAttribute('d').startsWith('M'),'a chart per row');
  $('farm-house').value='Other Realm|Alliance';fire($('farm-house'));
  assert.match(hot().map(r=>r.textContent).join(' '),/No scan today/);
  assert.equal($('farm-body').querySelector('.farm-empty-house').hidden,false,'no bank rows here, although hot rows show');
  $('farm-house').value='';fire($('farm-house'));
  assert.match(hot().map(r=>r.textContent).join(' '),/Pick an auction house/);
});

test('save-named bank rows use known ID quality and unknown rows stay default',async t=>{
  const {$,dom}=await setup(t,{reportRefresh:()=>report('B#2',true,farm)});
  $('nav-farm').click();
  const spans=[...$('farm-sell').querySelectorAll('td:first-child span')];
  const known=spans.filter(s=>s.textContent==='Sample Up');
  assert.equal(known.length,2,'the save name wins on both characters');
  for (const span of known) {
    assert.equal(span.className,'q2','Coach quality wins over GearQuest for the same ID');
    assert.equal(dom.window.getComputedStyle(span).color,'rgb(30, 255, 0)');
  }
  const unknown=spans.find(s=>s.textContent==='item 999999');
  assert.ok(unknown);
  assert.equal(unknown.className,'','unknown quality has no quality class');
});

for (const [card,direction,colour,change,supply,quality] of [
  ['farm-hot','hot-up','rgb(63, 185, 80)','+250%','20 / 20','q2'],
  ['farm-fallers','hot-down','rgb(248, 81, 73)','-50%','40 / 20','q3']
]) test(direction+' uses shared direction colour, signed changes and unit supply',async t=>{
  const {$,dom}=await setup(t,{reportRefresh:()=>report('B#2',true,farm)});
  $('nav-farm').click();
  const row=$(card).querySelector('svg').closest('tr');
  const cells=row.querySelectorAll('td');
  assert.equal(cells[2].textContent.trim(),change,'daily change keeps its sign');
  assert.equal(cells[4].textContent.trim(),change,'against usual keeps its sign');
  const headers=[...$(card).querySelectorAll('tr')].find(r=>r.querySelectorAll('th').length===8).querySelectorAll('th');
  assert.equal(headers[5].textContent,'listed (scan day) / usual','history peak, not a live count');
  assert.equal(cells[5].textContent.trim(),supply,'scan-day / usual units');
  assert.ok(cells[0].querySelector('.'+quality),'item-ID quality is independent of direction');
  for (const el of [cells[2],cells[4],row.querySelector('svg')]) {
    assert.ok(el.classList.contains(direction),'one direction class shared by changes and chart');
    assert.equal(dom.window.getComputedStyle(el).color,colour,'colour comes from page CSS');
    assert.equal(el.hasAttribute('style'),false,'no inline style');
  }
  const svg=row.querySelector('svg');
  assert.equal(svg.querySelector('path').getAttribute('stroke'),'currentColor');
  assert.equal(svg.querySelector('circle').getAttribute('fill'),'currentColor');
  if (card==='farm-fallers') {
    assert.equal(row.querySelector('.hot-dump').textContent,'dump');
    assert.equal(row.querySelector('.hot-spike'),null);
  }
});

test('fallers follow the chosen house and show their empty and unknown-house messages',async t=>{
  const {$}=await setup(t,{reportRefresh:()=>report('B#2',true,farm)});
  $('nav-farm').click();
  const visible=()=>[...$('farm-fallers').querySelectorAll('tr[data-house]')].filter(r=>!r.hidden);
  assert.ok(visible().length>1 && visible().every(r=>r.dataset.house==='Stormrage|Alliance'));
  assert.match(visible().map(r=>r.textContent).join(' '),/Down since yesterday/);
  $('farm-house').value='Other Realm|Alliance';fire($('farm-house'));
  assert.match(visible().map(r=>r.textContent).join(' '),/No scan today/);
  $('farm-house').value='';fire($('farm-house'));
  assert.match(visible().map(r=>r.textContent).join(' '),/Pick an auction house/);
});

test('every house that has bank rows can be chosen, scanned or not',async t=>{
  const {$,shown}=await setup(t,{reportRefresh:()=>report('B#2',true,farm)});
  $('nav-farm').click();
  const options=[...$('farm-house').options].map(o=>o.value);
  const houses=[...new Set([...$('farm-sell').querySelectorAll('tr[data-house]')].map(r=>r.dataset.house))];
  assert.ok(houses.includes('Nowhere|Horde'),'fixture has a row on an unscanned house');
  for (const h of houses) {
    assert.ok(options.includes(h),'house '+JSON.stringify(h)+' is in the list');
    $('farm-house').value=h;fire($('farm-house'));
    assert.ok(shown().length>0 && shown().every(x=>x===h),'its rows show: '+h);
  }
});

test('refresh keeps the chosen house; the page goes away when the save does',async t=>{
  let state=report('B#2',true,farm);
  const {$,shown}=await setup(t,{reportRefresh:()=>state});
  $('nav-farm').click();
  $('farm-house').value='Stormrage|Horde';
  fire($('farm-house'));
  state=report('B#2',true,farm.replace('Asking prices','Asking prices '));
  $('refresh').click();await settle();await settle();
  assert.equal($('farm-house').value,'Stormrage|Horde','a new render keeps the chosen house');
  assert.deepEqual(shown(),['Stormrage|Horde']);
  state=report('B#2',false);
  $('refresh').click();await settle();await settle();
  assert.equal($('nav-farm').hidden,true);
  assert.equal($('farm-view').hidden,true,'leaves a page that no longer exists');
  assert.equal($('report-view').hidden,false);
  assert.equal($('farm-body').innerHTML,'');
});

test('account switch clears the Farming page before the new account answers',async t=>{
  let release;
  const {$}=await setup(t,{reportRefresh:()=>report('B#2',true,farm),selectAccount:()=>new Promise(r=>{release=()=>r(report('A#1',false))})});
  $('nav-farm').click();
  assert.ok($('farm-body').innerHTML.length>100);
  $('account-select').value='A#1';
  fire($('account-select'));
  await settle();
  assert.equal($('farm-body').innerHTML,'','no farm data of the previous account while switching');
  release();await settle();await settle();
  assert.equal($('nav-farm').hidden,true);
});
