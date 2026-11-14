(() => {
'use strict';
const $ = id => document.getElementById(id);
const token = document.body.dataset.token;
const pending = new Map();
let sequence = 0;
// Each method is an explicit asynchronous Go binding, not an HTTP endpoint.
function call(method, ...params) {
  return new Promise((resolve,reject) => {
    const id = ++sequence;
    pending.set(id,{resolve,reject});
    window.chrome.webview.postMessage(JSON.stringify({id,method,params,token}));
  });
}
window.chrome.webview.addEventListener('message', event => {
  const msg = event.data;
  if (!msg || msg.kind !== 'native-response') return;
  const p = pending.get(msg.id);
  if (!p) return;
  pending.delete(msg.id);
  msg.error ? p.reject(new Error(msg.error)) : p.resolve(msg.result);
});
const rc = Object.freeze({
  refresh: () => call('reportRefresh'),
  setGame: path => call('setGame',path),
  selectAccount: id => call('selectAccount',id),
  chooseGame: () => call('chooseGame'),
  checkAddon: () => call('checkAddon'),
  applyAddon: version => call('applyAddon',version),
  checkApp: () => call('checkApp'),
  checkAppStartup: () => call('checkAppStartup'),
  applyApp: version => call('applyApp',version),
  appReady: () => call('appReady'),
  openExternal: url => call('openExternal',url),
  extraList: () => call('extraList'),
  checkExtra: id => call('checkExtra',id),
  applyExtra: (id,version) => call('applyExtra',id+'@'+version),
  appPlacement: () => call('appPlacement'),
  installAppCopy: () => call('installAppCopy'),
  repairAppLinks: list => call('repairAppLinks',list),
  openInstalledApp: () => call('openInstalledApp')
});
// One vocabulary for every addon (Relax Coach bundle and More addons):
// Install when the addon is not there yet, otherwise Update.
const verb = install => install ? 'Install' : 'Update';
// WoW can stay open (1.5.2); the result says whether /reload is enough.
const clickThen = install => ' Click '+verb(install)+'. WoW can stay open.';
const progressText = 'Downloading, verifying and installing… Do not force-close the application.';
const failText = (install,err) => verb(install)+' failed: '+err.message+' Check again before retrying.';
let offer = null, appOffer = null, busy = false, refreshing = false, restarting = false;
// Reads asked for while busy run once the UI is free again (see updateButtons).
let startupCheckPending = false, extrasPending = false;
let healthPending = false, healthSent = false;
// Two animation frames follow a successful Go report RPC and DOM rendering.
// This is startup health, not an idle heartbeat or a page-load acknowledgement.
function acknowledgeRenderedShell() {
  if (healthPending || healthSent) return;
  healthPending = true;
  requestAnimationFrame(()=>requestAnimationFrame(async()=>{
    try { await rc.appReady(); healthSent = true; }
    catch(err) { $('app-update-status').textContent = 'Startup verification failed: '+err.message; }
    finally { healthPending = false; }
  }));
}
let reportGeneration = 0, switchingFolder = false, switchingAccount = false;
let lastReport = null;
// Snapshot the Go-rendered report before interaction, using its escaped context.
let renderedReport = {
  HTML: $('report-body').innerHTML,
  Game: $('report-body').dataset.game,
  Account: $('report-body').dataset.account
};
function message(text) { $('global-status').textContent = text; }
let userNavigated = false, initialViewChosen = false, placement = null;
const autoChecked = new Set();
function show(view) {
  $('setup-view').hidden = view !== 'setup';
  $('report-view').hidden = view !== 'report';
  $('update-view').hidden = view !== 'update';
  $('farm-view').hidden = view !== 'farm';
  $('settings-view').hidden = view !== 'settings';
  for (const name of ['setup','report','farm','update','settings']) {
    $('nav-'+name).classList.toggle('active',name === view);
    if (name === view) $('nav-'+name).setAttribute('aria-current','page');
    else $('nav-'+name).removeAttribute('aria-current');
  }
}
function runDeferred() {
  if (busy || restarting) return;
  if (extrasPending) { extrasPending = false; loadExtras(); }
  if (startupCheckPending && !busy) { startupCheckPending = false; startupAppCheck(); }
}
function updateButtons() {
  // Every busy=false path calls this; a microtask lets its finally finish first.
  if (!busy && (startupCheckPending || extrasPending)) queueMicrotask(runDeferred);
  const disabled = document.body.dataset.updatesDisabled === 'true';
  $('refresh').disabled = refreshing || switchingFolder || switchingAccount;
  $('account-select').disabled = busy || !lastReport || !(lastReport.Accounts || []).length;
  $('check-app-update').disabled = busy || disabled;
  $('apply-app-update').disabled = busy || disabled || !appOffer || !appOffer.Available;
  $('check-update').disabled = busy || disabled;
  $('apply-update').disabled = busy || disabled || !offer || !offer.Available;
  $('setup-install').disabled = busy || disabled || !offer || !offer.Available;
  for (const id of ['browse-folder','use-folder','game-path','setup-browse','setup-app-install','setup-app-open','settings-replace','settings-keep']) $(id).disabled = busy;
  $('settings-repair').disabled = busy || !linksNeedRepair();
  renderBundleLabels();
  for (const b of document.querySelectorAll('#extra-list button')) {
    const o = extraOffers[b.dataset.id];
    b.disabled = busy || disabled || (b.dataset.role === 'apply' && !(o && o.Available));
  }
}
const extraOffers = {};
function extraRow(item) {
  let row = document.getElementById('extra-'+item.ID);
  if (!row) {
    row = document.createElement('div');
    row.id = 'extra-'+item.ID; row.className = 'extra';
    const h = document.createElement('h4'); h.className = 'extra-name';
    const p = document.createElement('p'); p.className = 'subtle extra-blurb';
    const dl = document.createElement('dl'); dl.className = 'versions';
    for (const [k,label] of [['installed','Installed'],['offered','Offered']]) {
      const d = document.createElement('div'); const dt = document.createElement('dt'); dt.textContent = label;
      const dd = document.createElement('dd'); dd.className = 'extra-'+k; d.append(dt,dd); dl.append(d);
    }
    const actions = document.createElement('div'); actions.className = 'actions';
    const check = document.createElement('button'); check.className = 'secondary'; check.textContent = 'Check';
    check.dataset.id = item.ID; check.dataset.role = 'check';
    const apply = document.createElement('button'); apply.className = 'primary'; apply.textContent = 'Install';
    apply.dataset.id = item.ID; apply.dataset.role = 'apply';
    actions.append(check, apply);
    const status = document.createElement('p'); status.className = 'status extra-status'; status.setAttribute('role','status');
    row.append(h,p,dl,actions,status);
    $('extra-list').append(row);
    check.addEventListener('click',()=>checkExtra(item.ID));
    apply.addEventListener('click',()=>applyExtra(item.ID));
  }
  row.querySelector('.extra-name').textContent = item.Name;
  row.querySelector('.extra-blurb').textContent = item.Blurb || '';
  row.dataset.problem = item.Problem ? '1' : '';
  row.querySelector('.extra-installed').textContent = item.Problem ? 'Unrecognized' : (item.InstalledVersion === '0.0.0' ? 'Not installed' : item.InstalledVersion);
  if (item.Problem && !extraOffers[item.ID]) row.querySelector('.extra-status').textContent = 'A '+item.Name+' folder exists but is not '+item.Name+' ('+item.Problem+'). It will not be replaced.';
  const o = extraOffers[item.ID];
  row.querySelector('.extra-offered').textContent = o ? (o.Available ? o.Version : 'None') : 'Not checked';
  row.querySelector('[data-role=apply]').textContent = item.InstalledVersion === '0.0.0' ? 'Install' : 'Update';
  return row;
}
async function loadExtras() {
  try { for (const item of await rc.extraList()) extraRow(item); }
  catch(err) { $('extra-list').textContent = 'Could not read installed addons: '+err.message; }
  updateButtons();
}
async function checkExtra(id) {
  if (busy) return;
  busy = true; delete extraOffers[id]; updateButtons();
  const row = document.getElementById('extra-'+id);
  row.querySelector('.extra-status').textContent = 'Checking the latest stable GitHub release…';
  try {
    const o = await rc.checkExtra(id);
    extraOffers[id] = o;
    extraRow(o);
    row.querySelector('.extra-status').textContent = (o.Status || '') + (o.Available ? clickThen(o.Install) : ' No files changed.');
  } catch(err) { row.querySelector('.extra-status').textContent = 'Check failed: '+err.message; }
  finally { busy = false; updateButtons(); }
}
async function applyExtra(id) {
  const o = extraOffers[id];
  if (busy || !o || !o.Available) return;
  busy = true; updateButtons();
  const row = document.getElementById('extra-'+id);
  row.querySelector('.extra-status').textContent = progressText;
  try {
    const result = await rc.applyExtra(id, o.Version);
    row.querySelector('.extra-status').textContent = result.message + (result.backupPath ? '\nBackup: '+result.backupPath : '');
  } catch(err) { row.querySelector('.extra-status').textContent = failText(o.Install, err); }
  finally { delete extraOffers[id]; busy = false; await loadExtras(); }
}
function displayAccount(result) {
  const select = $('account-select');
  const accounts = result.Accounts || [];
  const selected = result.Account || '';
  const available = accounts.some(account => account.ID === selected);
  select.replaceChildren();
  function option(value,text,disabled = false) {
    const el = document.createElement('option');
    el.value = value; el.textContent = text; el.disabled = disabled;
    select.append(el);
  }
  option('',accounts.length ? 'Choose an account…' : 'No account outputs yet',true);
  for (const account of accounts) option(account.ID,account.ID);
  if (selected && !available) option(selected,selected+' — unavailable',true);
  select.value = selected;
  $('account-source').textContent = selected
    ? 'Account '+selected+' · '+(result.Stale ? 'Stale — last good save: ' : 'Save source: ')+(result.SVPath || 'Unavailable')
    : (accounts.length ? 'Choose an account to view its saved characters.' : 'Install the addon, log in, then /reload to create account data.');
  const time = $('account-save-time');
  time.dateTime = result.SaveTime || '';
  time.textContent = result.SaveTime ? new Date(result.SaveTime).toLocaleString() : 'Not loaded';
  $('account-status').textContent = result.ConfigWarning || (selected && !available
    ? 'Selected account output is unavailable. Refresh will not switch accounts.' : '');
}
function setReportFilter(filters,button) {
  filters.querySelectorAll('button').forEach(b=>b.classList.toggle('on',b===button));
  filters.closest('.card').querySelectorAll('.more .fix').forEach(row=>{row.hidden=button.dataset.f!=='all'&&row.dataset.area!==button.dataset.f;});
}
// Farming page: shown only when the selected account has an AH Profit save.
// The HTML comes from html/template in Go, like the report body.
let renderedFarm = $('farm-body').innerHTML;
function filterFarmHouse() {
  const select = $('farm-house');
  const house = select ? select.value : null;
  const rows = $('farm-body').querySelectorAll('tr[data-house]');
  let shown = 0;
  for (const row of rows) { row.hidden = house !== null && row.dataset.house !== house; if (!row.hidden) shown++; }
  const empty = $('farm-body').querySelector('.farm-empty-house');
  if (empty) empty.hidden = !rows.length || shown > 0;
}
function displayFarm(result) {
  const has = result.HasFarm === true;
  $('nav-farm').hidden = !has;
  if (!has && !$('farm-view').hidden) show('report');
  const html = has ? result.FarmHTML : '';
  if (html !== renderedFarm) {
    const previous = $('farm-house') ? $('farm-house').value : null;
    $('farm-body').innerHTML = html;
    renderedFarm = html;
    const select = $('farm-house');
    if (select && previous !== null && [...select.options].some(o=>o.value===previous)) select.value = previous;
  }
  filterFarmHouse();
}
$('farm-body').addEventListener('change',event=>{ if (event.target && event.target.id === 'farm-house') filterFarmHouse(); });
function displayReport(result,folderChanged = false) {
  lastReport = result;
  if (result.AppVersion) $('installed-app-version').textContent = result.AppVersion;
  if (typeof result.AppUpdateStatus === 'string') $('app-last-result').textContent = result.AppUpdateStatus;
  displayAccount(result);
  // Compare server snapshots, not the DOM mutated by filters and expanded slots.
  const sameContext = renderedReport && renderedReport.Game === result.Game && renderedReport.Account === result.Account;
  if (!sameContext || renderedReport.HTML !== result.HTML) {
    const filtersByCharacter = new Map();
    if (sameContext) for (const filters of $('report-body').querySelectorAll('.filters')) {
      const character = filters.closest('section[id]');
      const selected = filters.querySelector('button.on');
      if (character && selected) filtersByCharacter.set(character.id,selected.dataset.f);
    }
    // HTML is rendered by html/template in Go. No release notes or JS input use this sink.
    $('report-body').innerHTML = result.HTML;
    for (const filters of $('report-body').querySelectorAll('.filters')) {
      const character = filters.closest('section[id]');
      const area = character && filtersByCharacter.get(character.id);
      const selected = [...filters.querySelectorAll('button')].find(b=>b.dataset.f===area);
      if (selected) setReportFilter(filters,selected);
    }
    renderedReport = {HTML:result.HTML,Game:result.Game,Account:result.Account};
  }
  displayFarm(result);
  $('installed-version').textContent = result.InstalledVersion === '0.0.0' ? 'Not installed' : result.InstalledVersion;
  $('setup-installed').textContent = $('installed-version').textContent;
  $('setup-folder').textContent = result.Game || 'Not chosen yet.';
  // Once a folder is known its button is no longer the step's main action.
  $('setup-folder-found').hidden = !result.Game;
  $('setup-browse').className = result.Game ? 'secondary' : 'primary';
  $('setup-browse').textContent = result.Game ? 'Change folder…' : 'Choose folder…';
  $('folder-label').textContent = result.Game;
  if (folderChanged || document.activeElement !== $('game-path')) $('game-path').value = result.Game;
  if (folderChanged) {
    offer = null;
    $('offered-version').textContent = 'Not checked';
    $('release-link').hidden = true;
    $('release-notes').textContent = 'Check again for this game folder.';
    $('setup-offered').textContent = 'Not checked';
    autoChecked.clear(); // a folder chosen again gets a fresh lookup
    bundleStatus('Game folder changed. Check again before installing or updating.', 'initial');
    for (const k of Object.keys(extraOffers)) delete extraOffers[k];
    if ($('extra-list').children.length) loadExtras();
    updateButtons();
  }
  // Say it before any click, from the folder state alone; recomputed on every
  // refresh, so the line goes away once the folder is Relax Journey again.
  if (!busy && !offer && result.Bundle && statusKind !== 'result') {
    const note = result.Bundle.state === 'replaced' ? foreignBeforeCheck()
      : result.Bundle.journeyUnreadable ? unreadableJourney+' Nothing is offered for it.' : '';
    if (note) bundleStatus(note, 'note');
    else if (statusKind === 'note') bundleStatus('', 'initial');
  }
  updateButtons();
}
async function refresh(manual = false) {
  if (refreshing || switchingFolder || switchingAccount) return;
  refreshing = true;
  updateButtons();
  const generation = reportGeneration;
  try {
    const result = await rc.refresh();
    if (generation !== reportGeneration) return;
    displayReport(result);
    chooseInitialView(result);
    acknowledgeRenderedShell();
    if (manual) message('Report refreshed. The game must save before new data appears.');
  }
  catch (err) { if (generation === reportGeneration) message('Could not refresh the report: '+err.message); }
  finally { refreshing = false; updateButtons(); }
}
$('nav-setup').addEventListener('click',()=>{ userNavigated = true; show('setup'); maybeAutoCheck(); });
$('nav-report').addEventListener('click',()=>{ userNavigated = true; show('report'); });
$('nav-update').addEventListener('click',()=>{ userNavigated = true; show('update'); if (!busy) loadExtras(); else extrasPending = true; });
$('nav-farm').addEventListener('click',()=>{ userNavigated = true; show('farm'); });
$('nav-settings').addEventListener('click',()=>{ userNavigated = true; show('settings'); if (!busy) loadPlacement(); });
$('setup-report').addEventListener('click',()=>{ userNavigated = true; show('report'); });
$('setup-more').addEventListener('click',()=>{ userNavigated = true; show('update'); if (!busy) loadExtras(); else extrasPending = true; });
$('refresh').addEventListener('click',()=>refresh(true));
async function changeFolder(select) {
  if (busy) return;
  let folder = $('folder-label').textContent;
  busy = switchingFolder = true;
  // Invalidate earlier replies before Go can change the selected/update target.
  ++reportGeneration;
  $('folder-label').textContent = 'Changing game folder…';
  $('account-source').textContent = 'Changing game folder…';
  $('report-body').hidden = true;
  $('farm-body').hidden = true;
  updateButtons();
  try {
    const result = await select();
    if (result) {
      folder = result.Game;
      displayReport(result,true);
      message('Game folder saved.');
    }
  }
  catch(err) { message(err.message); }
  finally {
    // Cancellation/errors leave the backend selection and checked offer unchanged.
    $('folder-label').textContent = folder;
    $('report-body').hidden = false;
    $('farm-body').hidden = false;
    if (lastReport) displayAccount(lastReport);
    busy = switchingFolder = false;
    updateButtons();
    maybeAutoCheck();
  }
}
$('game-form').addEventListener('submit',event => {
  event.preventDefault();
  changeFolder(()=>rc.setGame($('game-path').value));
});
$('browse-folder').addEventListener('click',()=>changeFolder(rc.chooseGame));
$('setup-browse').addEventListener('click',()=>changeFolder(rc.chooseGame));
$('account-select').addEventListener('change',async()=>{
  if (busy) return;
  const id = $('account-select').value;
  if (!id || (lastReport && id === lastReport.Account)) return;
  busy = switchingAccount = true;
  const generation = ++reportGeneration;
  // An account switch is a data boundary, including failed reads or writes.
  $('report-body').replaceChildren();
  renderedReport = null;
  $('farm-body').replaceChildren();
  renderedFarm = null;
  $('account-source').textContent = 'Changing account…';
  $('account-save-time').textContent = 'Not loaded';
  $('account-save-time').dateTime = '';
  $('account-status').textContent = '';
  updateButtons();
  try {
    const result = await rc.selectAccount(id);
    if (generation !== reportGeneration) return;
    displayReport(result);
    message(result.ConfigWarning || 'Account selection remembered for this game folder.');
  } catch(err) {
    if (generation !== reportGeneration) return;
    if (lastReport) displayAccount(lastReport);
    message(err.message);
  } finally {
    if (generation === reportGeneration) {
      busy = switchingAccount = false;
      updateButtons();
    }
  }
});
// The Relax Coach bundle (Coach, Journey, guide content, Relax Quest Planner).
// The Updates card and the Get started page share one offer and one flow.
function bundleInstall() {
  if (offer) return offer.Install === true;
  if (lastReport && lastReport.Bundle) return lastReport.Bundle.install === true;
  return !!lastReport && lastReport.InstalledVersion === '0.0.0';
}
// A Journey that is not Relax Journey: its own state, neutral text (the normal
// Journey next to Coach is a supported choice) and a Repair button. The text
// names the action the visible button really offers.
const foreignJourney = action => 'The Journey installed here is not Relax Journey (another updater such as CurseForge, Wago or Journey Guide Manager may have replaced it). '+action+'; if you use the normal Journey on purpose, you can ignore this.';
const foreignRepair = foreignJourney('Click Repair to install Relax Journey');
const foreignUpdate = foreignJourney('Click Update to install Relax Journey with it');
const foreignNoRelease = foreignJourney('Repair needs a published Relax Coach release, so check again later');
const unreadableJourney = "Relax Coach couldn't read Journey's TOC (Interface\\AddOns\\Journey\\Journey.toc), so it can't tell whether this is Relax Journey.";
// Before any Check the Repair button is disabled: point at Check instead.
const foreignBeforeCheck = () => document.body.dataset.updatesDisabled === 'true'
  ? foreignJourney('Updates are disabled by -no-update, so Repair is not available here')
  : foreignJourney('Click Check; when a release is found, Repair installs Relax Journey');
// What the shared bundle status line shows: the page's initial text, a
// folder-state note, or the result of an operation (check, install, error).
// A folder-state note may replace only the initial text or an earlier note,
// never an operation result or an error.
let statusKind = 'initial';
function bundleReplaced() {
  const b = offer ? offer.Bundle : (lastReport && lastReport.Bundle);
  return !!(b && b.state === 'replaced');
}
function bundleVerb() {
  return bundleReplaced() && (!offer || offer.Repair) ? 'Repair' : verb(bundleInstall());
}
function renderBundleLabels() {
  const v = bundleVerb();
  $('apply-update').textContent = v;
  $('setup-install').textContent = v;
  const b = lastReport && lastReport.Bundle;
  $('step-folder').classList.toggle('done', !!(lastReport && lastReport.Game));
  $('step-install').classList.toggle('done', !!(b && b.state === 'installed'));
}
// Partial installs: say what is missing and what Install/Update will do.
function bundleNote(b, available) {
  if (b && b.state === 'replaced') return ' '+(available ? foreignUpdate : foreignNoRelease);
  const unreadable = b && b.journeyUnreadable ? ' '+unreadableJourney+(available ? '' : ' Nothing is offered for it.') : '';
  if (!b || b.state !== 'partial') return unreadable;
  const missing = (b.missing || []).join(', '), present = (b.present || []).join(', ');
  let text = '';
  if (missing) text += ' Missing folders: '+missing+'.';
  if (b.install && present) text += ' Already there: '+present+'; '+verb(true)+' moves '+((b.present || []).length > 1 ? 'them' : 'it')+' into RelaxCoachBackups first.';
  if (missing) text += available ? ' '+verb(b.install)+' puts every folder in place.' : ' Check again after a moment; if no Install is offered, the bundle ZIP fallback in the README restores them.';
  return text+unreadable;
}
function bundleStatus(text, kind = 'result') { $('update-status').textContent = text; $('setup-status').textContent = text; statusKind = kind; }
async function checkBundle() {
  if (busy) return;
  busy = true; offer = null; updateButtons();
  $('release-link').hidden = true;
  $('offered-version').textContent = $('setup-offered').textContent = 'Checking…';
  bundleStatus('Checking the latest stable GitHub release…');
  try {
    offer = await rc.checkAddon();
    const install = offer.Install === true;
    $('installed-version').textContent = offer.InstalledVersion === '0.0.0' ? 'Not installed' : offer.InstalledVersion;
    $('setup-installed').textContent = $('installed-version').textContent;
    $('offered-version').textContent = $('setup-offered').textContent = offer.Available ? offer.Version : 'None';
    $('release-name').textContent = offer.Name || 'Release notes';
    $('release-notes').textContent = offer.Notes || 'No release notes provided.';
    $('release-link').href = offer.HTMLURL;
    $('release-link').hidden = !offer.HTMLURL;
    bundleStatus(offer.Available && offer.Repair && bundleReplaced()
      ? foreignRepair+' WoW can stay open.'
      : offer.Available
      ? 'Relax Coach '+offer.Version+' is ready to '+(install ? 'install' : 'update')+'.'+clickThen(install)
        +(offer.Repair ? ' This completes an interrupted or incomplete install: every Relax Coach folder is installed again, and the folders there now are kept in RelaxCoachBackups.' : bundleNote(offer.Bundle,true))
      : (offer.Notes || 'No compatible new addon update is available.')+' No files changed.'+bundleNote(offer.Bundle,false));
  } catch(err) {
    $('offered-version').textContent = $('setup-offered').textContent = 'Unavailable';
    $('release-notes').textContent = 'Release notes unavailable. Check again when online.';
    bundleStatus('Check failed: '+err.message);
  } finally { busy = false; updateButtons(); }
}
async function applyBundle() {
  if (busy || !offer || !offer.Available) return;
  const install = offer.Install === true, label = bundleVerb();
  busy = true; updateButtons();
  bundleStatus(progressText);
  try {
    // Only confirm the displayed version. URLs/checksums stay in Go memory.
    const result = await rc.applyAddon(offer.Version);
    bundleStatus(result.message + (result.backupPath ? '\nBackup: '+result.backupPath : ''));
    await refresh();
  } catch(err) { bundleStatus(label === 'Repair' ? 'Repair failed: '+err.message+' Check again before retrying.' : failText(install, err)); }
  finally { offer = null; busy = false; updateButtons(); }
}
$('check-update').addEventListener('click',checkBundle);
$('apply-update').addEventListener('click',applyBundle);
$('setup-install').addEventListener('click',applyBundle);
// A new player (no game folder, or Coach not installed) starts on Get started.
function chooseInitialView(result) {
  if (initialViewChosen) return;
  initialViewChosen = true;
  if (!userNavigated && (!result.Game || (result.Bundle ? result.Bundle.install === true : result.InstalledVersion === '0.0.0'))) show('setup');
  maybeAutoCheck(); // also when the player opened Get started before this first report
}
// On Get started only, look up the latest release once per folder so Install
// shows the version it will install. This reads release information; nothing
// is downloaded or installed until the player clicks Install.
function maybeAutoCheck() {
  if ($('setup-view').hidden || busy || offer || document.body.dataset.updatesDisabled === 'true') return;
  const game = lastReport && lastReport.Game;
  if (!game || autoChecked.has(game)) return;
  autoChecked.add(game);
  checkBundle();
}
// Step 1 of Get started and the Settings page: where the app runs from and its
// shortcuts. State text is redrawn from Go; the two status lines below it hold
// only results and errors of a click, so no refresh can wipe them.
const linkLabel = {desktop:'Desktop', startmenu:'Start menu'};
const linkText = l => l.State === 'ok' ? 'Opens the installed copy.'
  : l.State === 'missing' ? 'Missing.'
  : l.State === 'unavailable' ? 'Folder not found; no shortcut can be added there.'
  : l.Target ? 'Points somewhere else: '+l.Target : 'Exists, but it is not a shortcut to the installed copy.';
const canLink = () => !!placement && ['installed','other-copy'].includes(placement.State);
function linksNeedRepair() {
  return canLink() && (placement.Links || []).some(l => l.State === 'missing' || l.State === 'other');
}
function renderPlacement(p) {
  placement = p || null;
  const state = placement ? placement.State : 'unavailable';
  let text = (placement && placement.Message) || '';
  if (state === 'installed') text = 'Relax Coach is installed on this PC'+(placement.Target ? ' ('+placement.Target+')' : '')+'.';
  else if (state === 'other-copy' && !placement.SameCopy) text += ' The installed copy may be older than this one; Update everything there brings it up to date.';
  else if (state === 'unavailable') text = 'Relax Coach could not tell where it is installed. You can keep using it from here.';
  if (state === 'other-copy') text += ' Nothing is overwritten.';
  $('setup-app-text').textContent = text;
  $('setup-app-install').hidden = state !== 'offer';
  $('setup-app-open').hidden = state !== 'other-copy';
  $('step-app').classList.toggle('done', state === 'installed');
  if (state === 'offer' || state === 'other-copy') $('nav-setup').dataset.new = 'true';
  else delete $('nav-setup').dataset.new;
  $('settings-running').textContent = (placement && placement.Running) || 'Unknown';
  $('settings-target').textContent = (placement && placement.Target) || 'Unknown';
  $('settings-location').textContent = canLink() ? '' : state === 'offer' ? 'Not installed yet: use Get started, step 1.' : text;
  const list = $('settings-links');
  list.replaceChildren();
  for (const l of (placement && placement.Links) || []) {
    const li = document.createElement('li'); li.dataset.where = l.Where; li.dataset.state = l.State;
    const b = document.createElement('b'); b.textContent = linkLabel[l.Where] || l.Where;
    li.append(b, ' '+linkText(l)); list.append(li);
  }
  updateButtons();
}
async function loadPlacement() {
  try {
    const p = await rc.appPlacement();
    renderPlacement(p);
    // A copy that is not the installed one opens on step 1 (also with addons installed).
    if (!userNavigated && p && ['offer','other-copy'].includes(p.State)) { show('setup'); maybeAutoCheck(); }
  } catch(err) { renderPlacement(null); }
}
async function openInstalled(statusId, prefix) {
  try {
    await rc.openInstalledApp(); // Go closes this window after a successful start
    $(statusId).textContent = prefix+'Opening the installed Relax Coach…';
    restarting = true;
  } catch(err) {
    $(statusId).textContent = prefix+'Relax Coach could not reopen from its folder ('+err.message+'). Start it from '
      +((placement && placement.Target) || 'its folder')+' next time; you can delete the downloaded file.';
  }
}
$('setup-app-install').addEventListener('click',async()=>{
  if (busy) return;
  busy = true; updateButtons();
  $('setup-app-status').textContent = 'Installing Relax Coach and adding shortcuts…';
  let copied = null;
  try { copied = await rc.installAppCopy(); renderPlacement(copied); }
  catch(err) { $('setup-app-status').textContent = 'Install failed: '+err.message; }
  finally { busy = false; updateButtons(); }
  if (!copied) return;
  busy = true; updateButtons();
  try { await openInstalled('setup-app-status', (copied.Message || '')+' '); }
  finally { if (!restarting) { busy = false; updateButtons(); } }
});
$('setup-app-open').addEventListener('click',async()=>{
  if (busy) return;
  busy = true; updateButtons();
  $('setup-app-status').textContent = 'Opening the installed Relax Coach…';
  try { await openInstalled('setup-app-status', ''); }
  finally { if (!restarting) { busy = false; updateButtons(); } }
});
async function repairLinks(replace) {
  if (busy) return;
  busy = true; $('settings-confirm').hidden = true; updateButtons();
  $('settings-status').textContent = 'Repairing shortcuts…';
  try { const p = await rc.repairAppLinks(replace.join(',')); renderPlacement(p); $('settings-status').textContent = p.Message || 'Shortcuts checked.'; }
  catch(err) { $('settings-status').textContent = 'Repair failed: '+err.message; }
  finally { busy = false; updateButtons(); }
}
$('settings-repair').addEventListener('click',()=>{
  if (busy || !linksNeedRepair()) return;
  const others = placement.Links.filter(l => l.State === 'other');
  if (!others.length) { repairLinks([]); return; }
  // Never replace a shortcut the player did not confirm by name.
  $('settings-confirm-text').textContent = others.map(l => 'The '+linkLabel[l.Where]+' shortcut '+(l.Target ? 'points at '+l.Target : 'is not a shortcut to the installed copy')+'.').join(' ')+' Replace it?';
  $('settings-confirm').hidden = false;
});
$('settings-replace').addEventListener('click',()=>repairLinks(placement.Links.filter(l => l.State === 'other').map(l => l.Where)));
$('settings-keep').addEventListener('click',()=>{
  $('settings-confirm').hidden = true;
  if (placement.Links.some(l => l.State === 'missing')) repairLinks([]);
  else $('settings-status').textContent = 'Nothing was changed.';
});
// Read-only release check once per launch so players learn about a new app
// (and its new look) without visiting GitHub. Nothing downloads or installs
// until they click Update App; -no-update skips it entirely.
async function startupAppCheck() {
  if (appOffer || document.body.dataset.updatesDisabled === 'true') return;
  if (busy) { startupCheckPending = true; return; } // runs once the bundle check ends
  // Go holds appBusy for the whole check and rejects other operations meanwhile,
  // so the UI stays locked (busy) too; the finally gives a starved auto-check its retry.
  busy = true; updateButtons();
  try {
    const found = await rc.checkAppStartup();
    if (!found || !found.Available || appOffer) return;
    appOffer = found;
    showAppOffer(appOffer); updateButtons();
    $('nav-update').dataset.new = 'true';
    $('nav-update').title = 'Relax Coach '+appOffer.Version+' is available';
    message('Relax Coach '+appOffer.Version+' is available. Open Updates and click Update App. No re-download needed.');
  } catch(_) { /* offline: the manual check still works */ }
  finally { busy = false; updateButtons(); maybeAutoCheck(); }
}
function showAppOffer(o) {
  $('installed-app-version').textContent = o.InstalledVersion;
  $('offered-app-version').textContent = o.Version || 'Unavailable';
  $('app-release-name').textContent = o.Name || 'App release notes';
  $('app-release-notes').textContent = o.Notes || 'No release notes provided.';
  $('app-release-link').href = o.HTMLURL;
  $('app-release-link').hidden = !o.HTMLURL;
  $('app-update-status').textContent = o.Available
    ? 'Click Update App to download, verify, back up and restart Relax Coach. WoW can stay open.'
    : 'No compatible new app update is available. See release notes. No files changed.';
}
$('check-app-update').addEventListener('click',async()=>{
  if (busy || document.body.dataset.updatesDisabled === 'true') return;
  busy = true; appOffer = null; updateButtons();
  $('app-release-link').hidden = true;
  $('offered-app-version').textContent = 'Checking…';
  $('app-update-status').textContent = 'Checking the latest stable app release…';
  try {
    appOffer = await rc.checkApp();
    showAppOffer(appOffer);
    if (!appOffer.Available) delete $('nav-update').dataset.new;
  } catch(err) {
    $('offered-app-version').textContent = 'Unavailable';
    $('app-release-notes').textContent = 'Release notes unavailable. Check again when online.';
    $('app-update-status').textContent = 'App update check failed: '+err.message;
  } finally { busy = false; updateButtons(); }
});
$('apply-app-update').addEventListener('click',async()=>{
  if (busy || !appOffer || !appOffer.Available || document.body.dataset.updatesDisabled === 'true') return;
  busy = true; updateButtons();
  $('app-update-status').textContent = 'Downloading and verifying the app. Relax Coach will restart when the trusted worker is ready. Close normally to cancel before handoff; do not force-close.';
  try {
    const result = await rc.applyApp(appOffer.Version);
    restarting = result.restartReady === true;
    $('app-update-status').textContent = result.message || 'The app update was not handed off. Check again.';
  } catch(err) { $('app-update-status').textContent = 'App update failed: '+err.message+' Check again before retrying.'; }
  finally { appOffer = null; busy = restarting; updateButtons(); }
});
// Keep every link inside the document or route it through native allowlist validation.
function interceptLink(event) {
  const link = event.target.closest('a');
  if (!link) return;
  event.preventDefault(); event.stopPropagation();
  const url = link.getAttribute('href') || '';
  if (url.startsWith('#')) {
    const section = document.getElementById(url.slice(1));
    if (section) section.scrollIntoView({block:'start'});
  } else if (event.type === 'click' && event.button === 0) {
    rc.openExternal(url).catch(err=>message(err.message));
  }
}
document.addEventListener('click',interceptLink,true);
document.addEventListener('auxclick',interceptLink,true);
document.addEventListener('dragover',event=>event.preventDefault());
document.addEventListener('drop',event=>event.preventDefault());
document.addEventListener('click',event=>{
  const button = event.target.closest('.filters button');
  if (button) setReportFilter(button.closest('.filters'),button);
  const slot = event.target.closest('.slot');
  if (slot) slot.classList.toggle('open');
});
// A convenience refresh, not a heartbeat. No timer can close the native window.
setInterval(()=>{if (!document.hidden && !busy) refresh();},5000);
refresh().then(loadPlacement).then(startupAppCheck);
})();
