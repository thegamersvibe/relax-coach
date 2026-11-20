// Real Go HTML, fake native transport. Not a Windows/WebView2 acceptance test.
const {test}=require('node:test');
const assert=require('node:assert/strict');
const {readFileSync}=require('node:fs');
const {JSDOM}=require('jsdom');
const html=readFileSync(process.env.NATIVE_HTML_OUT,'utf8');
const raid=readFileSync(process.env.NATIVE_HTML_OUT+'.raid-shell','utf8');
const body=readFileSync(process.env.NATIVE_HTML_OUT+'.raid-body','utf8');
const deaths=readFileSync(process.env.NATIVE_HTML_OUT+'.raid-deaths','utf8');
const withBody=content=>'<section id="raids-view" hidden><header class="pagehead"><h2>Raids</h2></header><div id="raids-body" class="pane">'+content+'</div></section>';
const settle=()=>new Promise(r=>setTimeout(r,30));
const report=(allowed,account='A#1')=>({HTML:'<p>Coach</p>',Game:'C:\\WoW',Account:account,Accounts:[{ID:'A#1'},{ID:'B#2'}],HasRaids:allowed,RaidHTML:allowed?raid:'',InstalledVersion:'1.8.1'});
async function setup(t,handlers){
 let listener;const calls=[];
 const dom=new JSDOM(html,{runScripts:'dangerously',pretendToBeVisual:true,beforeParse(w){
  w.setInterval=()=>1;w.requestAnimationFrame=()=>0;
  w.chrome={webview:{addEventListener(_,f){listener=f},postMessage(raw){
   const req=JSON.parse(raw);calls.push(req);
   Promise.resolve().then(async()=>{try{const fn=handlers[req.method];const result=fn?await fn(...req.params):null;listener({data:{kind:'native-response',id:req.id,result}})}catch(e){listener({data:{kind:'native-response',id:req.id,error:e.message}})}});
  }}};w.fetch=()=>assert.fail('page sent network request');
 }});t.after(()=>dom.window.close());await settle();await settle();
 return {dom,calls,$:id=>dom.window.document.getElementById(id)};
}
test('row detail opens exact part evidence and closes without losing lists',async t=>{
 const {$,dom}=await setup(t,{reportRefresh:()=>report(true),raidPage:()=>withBody(body)});
 $('nav-raids').click();await settle();
 const row=dom.window.document.querySelector('[data-raid-player="1"]');
 row.querySelector('button').click();
 assert.equal($('raid-player-1').hidden,false);
 assert.match($('raid-player-1').textContent,/boss_damage/);
 const detail=$('raid-player-1').querySelector('details');detail.querySelector('summary').click();
 assert.equal(detail.open,true);
 $('raid-player-1').querySelector('[data-raid-close]').click();assert.equal($('raid-player-1').hidden,true);
 assert.ok($('raid-ranked'));
});
test('all player and officer names stay text at the real HTML insertion sink',async t=>{
 const escaped=readFileSync(process.env.NATIVE_HTML_OUT+'.raid-escape','utf8');
 const {$,dom}=await setup(t,{reportRefresh:()=>report(true),raidPage:()=>withBody(escaped)});
 $('nav-raids').click();await settle();
 assert.equal($('raids-view').querySelector('img,script,[onerror]'),null);
 const special=`<img src="x" onerror="alert(1)">&'`;
 for(const prefix of ['Logger','Ranked','Provisional','Unscored','Unmatched','Death','Owner','First','Following','Officer','Top','Bottom','ScoreTop','ScoreBottom']) {
  assert.ok($('raids-view').textContent.includes(prefix+special),prefix+' lost literal name text');
 }
});
test('every name is escaped, control-free and bidi-isolated at insertion',async t=>{
 const content=readFileSync(process.env.NATIVE_HTML_OUT+'.raid-bidi','utf8');
 const expected=JSON.parse(readFileSync(process.env.NATIVE_HTML_OUT+'.raid-bidi-names.json','utf8'));
 const {$,dom}=await setup(t,{reportRefresh:()=>report(true),raidPage:()=>withBody(content)});
 $('nav-raids').click();await settle();
 const view=$('raids-view');
 assert.equal(view.querySelector('img,script,[onerror]'),null);
 assert.doesNotMatch(view.textContent,/[\u061c\ufeff\u2060-\u2064\u00ad\u180e\u200b-\u200f\u202a-\u202e\u2066-\u2069]/u);
 for(const name of expected) assert.ok([...view.querySelectorAll('bdi')].some(n=>n.textContent===name),'missing isolated name');
 // Check every occurrence, including names repeated in closed player details.
 const walker=dom.window.document.createTreeWalker(view,dom.window.NodeFilter.SHOW_TEXT);
 let checked=0;
 while(walker.nextNode()) {
  const node=walker.currentNode;
  if(!node.textContent.includes('<img src=')) continue;
  assert.equal(node.parentElement.tagName,'BDI','name escaped isolation');
  assert.ok(expected.includes(node.textContent),'name text was changed');
  checked++;
 }
 assert.ok(checked>=expected.length);
});
test('death filters keep unknown separate and restore all rows',async t=>{
 const {$,dom}=await setup(t,{reportRefresh:()=>report(true),raidPage:()=>withBody(deaths)});
 $('nav-raids').click();await settle();
 const doc=dom.window.document,rows=()=>[...doc.querySelectorAll('[data-death-kind]')];
 assert.equal(rows().length,3);
 for(const filter of ['unknown','avoidable','all']) {
  const button=doc.querySelector('[data-death-filter="'+filter+'"]');button.click();
  assert.equal(button.getAttribute('aria-pressed'),'true');
  for(const row of rows()) assert.equal(row.hidden,filter!=='all'&&row.dataset.deathKind!==filter);
 }
});
for(const state of [false,null,'true']) test('locked state '+state+' has no Raids DOM',async t=>{
 const {$}=await setup(t,{reportRefresh:()=>report(state)});
 assert.equal($('nav-raids'),null);assert.equal($('raids-view'),null);
});
test('officer renders navigation and enters the guarded route',async t=>{
 const {$,calls}=await setup(t,{reportRefresh:()=>report(true),raidPage:()=>raid});
 assert.ok($('nav-raids'));$('nav-raids').click();await settle();
 assert.equal(calls.filter(r=>r.method==='raidPage').length,1);
 assert.equal($('raids-view').hidden,false);assert.equal($('report-view').hidden,true);
 $('nav-report').click();assert.equal($('raids-view').hidden,true);
});
test('account switch removes DOM immediately and drops late raid reply',async t=>{
 let releasePage,releaseAccount;
 const {$,dom}=await setup(t,{reportRefresh:()=>report(true),raidPage:()=>new Promise(r=>releasePage=r),selectAccount:()=>new Promise(r=>releaseAccount=r)});
 $('nav-raids').click();await settle();
 $('account-select').value='B#2';$('account-select').dispatchEvent(new dom.window.Event('change',{bubbles:true}));await settle();
 assert.equal($('nav-raids'),null);assert.equal($('raids-view'),null);
 releasePage(raid);await settle();assert.equal($('raids-view'),null);
 releaseAccount(report(false,'B#2'));await settle();assert.equal($('raids-view'),null);
});
test('refresh and a refused direct route both revoke existing page',async t=>{
 let allowed=true,refused=false;
 const {$}=await setup(t,{reportRefresh:()=>report(allowed),raidPage:()=>{if(refused)throw Error('Raids locked');return raid}});
 $('nav-raids').click();await settle();allowed=false;$('refresh').click();await settle();
 assert.equal($('nav-raids'),null);assert.equal($('raids-view'),null);assert.equal($('report-view').hidden,false);
 allowed=true;$('refresh').click();await settle();refused=true;$('nav-raids').click();await settle();
 assert.equal($('nav-raids'),null);assert.equal($('raids-view'),null);
 assert.match($('global-status').textContent,/locked/);
});
for (const refreshFirst of [false,true]) test('route lock wins over refresh reply order '+refreshFirst,async t=>{
 let releaseRefresh,rejectPage,hold=false;
 const {$,calls}=await setup(t,{reportRefresh:()=>hold?new Promise(r=>releaseRefresh=r):report(true),raidPage:()=>new Promise((_,reject)=>rejectPage=reject)});
 hold=true;$('refresh').click();await settle();$('nav-raids').click();await settle();
 if(refreshFirst){releaseRefresh(report(true));await settle();}
 rejectPage(Error('Raids locked: current reading invalid'));await settle();
 if(!refreshFirst){releaseRefresh(report(true));await settle();}
 assert.equal($('nav-raids'),null);assert.equal($('raids-view'),null);
 assert.match($('global-status').textContent,/Raids locked: current reading invalid/);
 assert.equal(calls.filter(r=>r.method==='raidPage').length,1);
});
test('a later refresh cannot unlock without a new guarded route verdict',async t=>{
 let locked=true;
 const {$,calls}=await setup(t,{reportRefresh:()=>report(true),raidPage:()=>{if(locked)throw Error('Raids locked: stale');return raid}});
 $('nav-raids').click();await settle();$('refresh').click();await settle();
 assert.equal($('nav-raids'),null);assert.match($('global-status').textContent,/locked/);
 locked=false;$('refresh').click();await settle();
 assert.ok($('nav-raids'));
 assert.ok(calls.filter(r=>r.method==='raidPage').length>=3,'refresh alone fabricated admission');
});
