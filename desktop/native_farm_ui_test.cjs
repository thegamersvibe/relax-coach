// Farming page (AH Profit): Go-rendered shell and fragment, fake native transport.
// This is not a Windows GUI or WebView2 pass.
const {test}=require('node:test');
const assert=require('node:assert/strict');
const {readFileSync}=require('node:fs');
const {JSDOM}=require('jsdom');
const html=readFileSync(process.env.NATIVE_HTML_OUT,'utf8');
const farm=readFileSync(process.env.FARM_HTML_OUT,'utf8');
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
  return {$,dom,shown:()=>[...$('farm-body').querySelectorAll('tr[data-house]')].filter(r=>!r.hidden).map(r=>r.dataset.house)};
}

test('no AH Profit save: no Farming entry',async t=>{
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
  assert.match($('farm-gold').textContent,/Update AH Profit to get gold history/);
  assert.equal($('farm-house').value,'Stormrage|Alliance');
  assert.ok(shown().length>0 && shown().every(h=>h==='Stormrage|Alliance'),'only the current house');
  $('farm-house').value='Stormrage|Horde';
  fire($('farm-house'));
  assert.deepEqual(shown(),['Stormrage|Horde'],'another house is one choice away');
  $('farm-house').value='Other Realm|Alliance';
  fire($('farm-house'));
  assert.deepEqual(shown(),[]);
  assert.equal($('farm-body').querySelector('.farm-empty-house').hidden,false,'says so when a house has no bank items');
  assert.ok(!$('farm-body').innerHTML.includes('<b>Linen</b>'),'save text stays escaped');
});

test('every house that has bank rows can be chosen, scanned or not',async t=>{
  const {$,shown}=await setup(t,{reportRefresh:()=>report('B#2',true,farm)});
  $('nav-farm').click();
  const options=[...$('farm-house').options].map(o=>o.value);
  const houses=[...new Set([...$('farm-body').querySelectorAll('tr[data-house]')].map(r=>r.dataset.house))];
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
