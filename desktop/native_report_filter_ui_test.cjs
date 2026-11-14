// Actual Go-rendered report and embedded JS, with a synthetic native transport.
// Run after generating NATIVE_HTML_OUT with TestNativeShellIsSelfContainedAndLockedDown.
const {test}=require('node:test');
const assert=require('node:assert/strict');
const {readFileSync}=require('node:fs');
const {JSDOM}=require('jsdom');
const html=readFileSync(process.env.NATIVE_HTML_OUT,'utf8');
const fixture=new JSDOM(html);
const characterTemplate=fixture.window.document.querySelector('#report-body section[id]').outerHTML;
assert.ok(fixture.window.document.querySelector('#report-body .filters'),'Go fixture must render Everything else');
// Account TEST comes from nativeTestApp; use the untouched Go-rendered input value,
// not a parsed display label or the still-loading account dropdown.
const initialReport={
  HTML:fixture.window.document.getElementById('report-body').innerHTML,
  Game:fixture.window.document.getElementById('game-path').value,
  Account:'TEST',Accounts:[{ID:'TEST'}],InstalledVersion:'1.4.0'
};
const initialCharacter=fixture.window.document.querySelector('#report-body section[id]').id;
assert.equal(initialCharacter,'Testwarrior-Test');
assert.ok(initialReport.Game,'Go fixture must select a game folder');
fixture.window.close();
const gameA='C:\\FixtureA\\_forever_',gameB='D:\\FixtureB\\_forever_';
const characterA='Realm-CharacterA',characterB='Realm-CharacterB';
const settle=()=>new Promise(resolve=>setTimeout(resolve,20));
function report({account='A#1',game=gameA,characters=[characterA],revision=0}={}) {
  const fragment=JSDOM.fragment('<div></div>');
  const root=fragment.firstElementChild;
  for(const id of characters){
    const section=JSDOM.fragment(characterTemplate).firstElementChild;
    section.id=id;
    section.querySelector('.who .name').textContent=id;
    root.append(section);
  }
  const note=root.ownerDocument.createElement('p');note.textContent='Fixture revision '+revision;root.append(note);
  return {HTML:root.innerHTML,Game:game,Account:account,Accounts:[{ID:'A#1'},{ID:'B#2'}],InstalledVersion:'1.4.0',SVPath:game+'\\WTF\\Account\\'+account+'\\SavedVariables\\ForeverCoach.lua'};
}
async function setup(t,handlers={}) {
  const calls=[],timers=[];let listener;
  const dom=new JSDOM(html,{runScripts:'dangerously',pretendToBeVisual:true,beforeParse(window){
    window.setInterval=(fn,delay)=>{timers.push({fn,delay});return timers.length;};
    window.chrome={webview:{addEventListener(name,fn){assert.equal(name,'message');listener=fn;},postMessage(raw){
      const req=JSON.parse(raw);calls.push(req);
      Promise.resolve().then(async()=>{
        assert.equal(req.token,'test-token');
        try {const fn=handlers[req.method];const result=fn?await fn(...req.params):report();listener({data:{kind:'native-response',id:req.id,result}});}
        catch(error){listener({data:{kind:'native-response',id:req.id,error:error.message}});}
      });
    }}};
    window.fetch=()=>assert.fail('native report UI must not use HTTP');
  }});
  t.after(()=>dom.window.close());await settle();
  return {dom,calls,$:id=>dom.window.document.getElementById(id),tick:()=>{assert.ok(timers.some(t=>t.delay===5000));timers.forEach(t=>t.fn());}};
}
function chooseFilter($,id,area){
  const button=[...$(id).querySelectorAll('.filters button')].find(b=>b.dataset.f===area);
  assert.ok(button,'filter button exists: '+area);button.click();return button;
}
function assertFilter($,id,area){
  const section=$(id);assert.ok(section,'character section exists');
  assert.deepEqual([...section.querySelectorAll('.filters button.on')].map(b=>b.dataset.f),[area]);
  for(const row of section.querySelectorAll('.more .fix'))assert.equal(row.hidden,area!=='all'&&row.dataset.area!==area,'matching finding visibility');
}

for(const changed of [false,true]){
  test(`startup keeps Gear selected through a delayed ${changed?'changed':'unchanged'} same-context response`,async t=>{
    let resolveFirst;const first=new Promise(resolve=>{resolveFirst=resolve;});
    const current=changed?report({game:initialReport.Game,account:initialReport.Account,characters:[initialCharacter],revision:1}):initialReport;
    const {$,calls,tick}=await setup(t,{reportRefresh:()=>first});
    assert.equal(calls.filter(c=>c.method==='reportRefresh').length,1);
    assert.equal($('account-select').value,'','account list is still loading');
    const button=chooseFilter($,initialCharacter,'Gear');assertFilter($,initialCharacter,'Gear');
    resolveFirst(current);await settle();
    assertFilter($,initialCharacter,'Gear');
    assert.equal($('account-select').value,initialReport.Account);
    if(changed){
      assert.match($('report-body').textContent,/Fixture revision 1/,'first new data must render');
      assert.notEqual($(initialCharacter).querySelector('.filters button.on'),button,'changed data must replace the report');
    }else{
      assert.equal($(initialCharacter).querySelector('.filters button.on'),button,'unchanged startup snapshot must not replace interactive DOM');
    }
    tick();await settle();assertFilter($,initialCharacter,'Gear');
    assert.equal(calls.filter(c=>c.method==='reportRefresh').length,2);
  });
}

for(const retry of ['manual','automatic']){
  test(`startup keeps the current filter after a first-response error and ${retry} retry`,async t=>{
    let rejectFirst,refreshes=0;const first=new Promise((_,reject)=>{rejectFirst=reject;});
    const {$,calls,tick}=await setup(t,{reportRefresh:()=>++refreshes===1?first:initialReport});
    const button=chooseFilter($,initialCharacter,'Gear');assertFilter($,initialCharacter,'Gear');
    rejectFirst(Error('fixture initial refresh failure'));await settle();
    assert.match($('global-status').textContent,/Could not refresh the report: fixture initial refresh failure/);
    assertFilter($,initialCharacter,'Gear');
    assert.equal($(initialCharacter).querySelector('.filters button.on'),button,'error must leave the pre-rendered report intact');
    chooseFilter($,initialCharacter,'Play');assertFilter($,initialCharacter,'Play');
    if(retry==='manual')$('refresh').click();else tick();
    await settle();
    assertFilter($,initialCharacter,'Play');
    assert.equal($('account-select').value,initialReport.Account);
    assert.equal(calls.filter(c=>c.method==='reportRefresh').length,2);
  });
}

for(const field of ['Account','Game'])for(const retry of [false,true]){
  test(`startup ${retry?'retry':'first response'} for a different ${field.toLowerCase()} does not inherit filters`,async t=>{
    let resolveFirst,rejectFirst,refreshes=0;
    const first=new Promise((resolve,reject)=>{resolveFirst=resolve;rejectFirst=reject;});
    const current={...initialReport,[field]:field==='Account'?'OTHER':gameB};
    current.Accounts=[{ID:current.Account}];
    const {$,calls,tick}=await setup(t,{reportRefresh:()=>++refreshes===1?first:current});
    const button=chooseFilter($,initialCharacter,'Gear');assertFilter($,initialCharacter,'Gear');
    if(retry){
      rejectFirst(Error('fixture initial refresh failure'));await settle();
      assert.match($('global-status').textContent,/fixture initial refresh failure/);
      assertFilter($,initialCharacter,'Gear');
      $('refresh').click();
    }else resolveFirst(current);
    await settle();
    assertFilter($,initialCharacter,'all');
    assert.notEqual($(initialCharacter).querySelector('.filters button.on'),button,'context change must replace even identical report HTML');
    assert.equal($('account-select').value,current.Account);
    assert.equal($('folder-label').textContent,current.Game);
    assert.equal(calls.filter(c=>c.method==='reportRefresh').length,retry?2:1);
    chooseFilter($,initialCharacter,'Next');tick();await settle();
    assertFilter($,initialCharacter,'Next');
  });
}

test('Everything else keeps Gear selected through an unchanged automatic refresh',async t=>{
  const unchanged=report();
  const {$,tick,calls}=await setup(t,{reportRefresh:()=>unchanged});
  const button=chooseFilter($,characterA,'Gear');assertFilter($,characterA,'Gear');
  tick();await settle();
  assertFilter($,characterA,'Gear');
  assert.equal($(characterA).querySelector('.filters button.on'),button,'unchanged data must not replace interactive DOM');
  assert.equal(calls.filter(c=>c.method==='reportRefresh').length,2);
});

test('Everything else keeps Gear selected when a manual refresh brings new report data',async t=>{
  let current=report();
  const {$}=await setup(t,{reportRefresh:()=>current});
  chooseFilter($,characterA,'Gear');
  current=report({revision:1});
  $('refresh').click();await settle();
  assert.match($('report-body').textContent,/Fixture revision 1/,'new data must still render');
  assertFilter($,characterA,'Gear');
});

for(const area of ['Build','Gear','Play','Next','all']){
  test(`${area} survives repeated polling and manual refreshes with changed data`,async t=>{
    let current=report();
    const {$,tick}=await setup(t,{reportRefresh:()=>current});
    chooseFilter($,characterA,'Gear');chooseFilter($,characterA,area);
    for(let revision=1;revision<=3;revision++){
      current=report({revision});tick();await settle();assertFilter($,characterA,area);
      $('refresh').click();await settle();assertFilter($,characterA,area);
      assert.match($('report-body').textContent,new RegExp('Fixture revision '+revision));
    }
  });
}

test('filter choices follow literal character IDs, not report ordering',async t=>{
  const special='Realm-Character "[x] É';
  let current=report({characters:[characterA,special]});
  const {$,tick}=await setup(t,{reportRefresh:()=>current});
  chooseFilter($,characterA,'Gear');chooseFilter($,special,'Play');
  current=report({characters:[special,characterB,characterA],revision:1});tick();await settle();
  assertFilter($,characterA,'Gear');assertFilter($,special,'Play');assertFilter($,characterB,'all');
});

test('a different character does not inherit a removed character filter',async t=>{
  let current=report();const {$,tick}=await setup(t,{reportRefresh:()=>current});
  chooseFilter($,characterA,'Next');
  current=report({characters:[characterB],revision:1});tick();await settle();
  assert.equal($(characterA),null);assertFilter($,characterB,'all');
});

function chooseAccount(dom,$,id){
  $('account-select').value=id;
  $('account-select').dispatchEvent(new dom.window.Event('change',{bubbles:true}));
}

test('an account change resets filters even when its rendered HTML is identical',async t=>{
  let current=report();
  const {dom,$,tick}=await setup(t,{reportRefresh:()=>current,selectAccount:account=>(current=report({account}))});
  chooseFilter($,characterA,'Gear');
  chooseAccount(dom,$,'B#2');await settle();
  assert.equal($('account-select').value,'B#2');assertFilter($,characterA,'all');
  chooseFilter($,characterA,'Play');tick();await settle();assertFilter($,characterA,'Play');
});

test('a game-folder change resets filters even with the same account and report HTML',async t=>{
  let current=report();
  const {dom,$}=await setup(t,{reportRefresh:()=>current,setGame:game=>(current=report({game}))});
  chooseFilter($,characterA,'Next');
  $('game-path').value=gameB;
  $('game-form').dispatchEvent(new dom.window.Event('submit',{bubbles:true,cancelable:true}));await settle();
  assert.equal($('folder-label').textContent,gameB);assertFilter($,characterA,'all');
});

test('failed account selection clears the view and identical old data can render on retry',async t=>{
  const current=report();
  const {dom,$}=await setup(t,{reportRefresh:()=>current,selectAccount:()=>{throw Error('fixture write failure');}});
  chooseFilter($,characterA,'Gear');chooseAccount(dom,$,'B#2');await settle();
  assert.equal($('report-body').textContent,'');assert.equal($('account-select').value,'A#1');
  $('refresh').click();await settle();assertFilter($,characterA,'all');
});

for(const outcome of ['cancel','error']){
  test(`folder picker ${outcome} preserves the existing filter`,async t=>{
    const {$,tick}=await setup(t,{chooseGame:()=>{if(outcome==='error')throw Error('fixture cancel');return null;}});
    chooseFilter($,characterA,'Play');$('browse-folder').click();await settle();tick();await settle();
    assertFilter($,characterA,'Play');assert.equal($('report-body').hidden,false);
  });
}

test('an unavailable filter falls back to All when the report is replaced',async t=>{
  let current=report();const {$,tick}=await setup(t,{reportRefresh:()=>current});
  chooseFilter($,characterA,'Gear');
  const fragment=JSDOM.fragment(report({revision:1}).HTML);fragment.querySelector('[data-f="Gear"]').remove();
  const container=fragment.ownerDocument.createElement('div');container.append(fragment);
  current={...report({revision:1}),HTML:container.innerHTML};tick();await settle();
  assertFilter($,characterA,'all');
});

test('a delayed old-account refresh cannot replace the newly selected account filter',async t=>{
  let resolveOld,calls=0,current=report();const old=new Promise(resolve=>{resolveOld=resolve;});
  const {dom,$}=await setup(t,{reportRefresh:()=>++calls===2?old:current,selectAccount:account=>(current=report({account}))});
  chooseFilter($,characterA,'Gear');$('refresh').click();await settle();
  chooseAccount(dom,$,'B#2');await settle();chooseFilter($,characterA,'Next');
  resolveOld(report({revision:99}));await settle();
  assert.equal($('account-select').value,'B#2');assertFilter($,characterA,'Next');
  assert.doesNotMatch($('report-body').textContent,/Fixture revision 99/);
});
