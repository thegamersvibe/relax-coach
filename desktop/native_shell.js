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
  applyApp: version => call('applyApp',version),
  appReady: () => call('appReady'),
  openExternal: url => call('openExternal',url),
  extraList: () => call('extraList'),
  checkExtra: id => call('checkExtra',id),
  applyExtra: (id,version) => call('applyExtra',id+'@'+version)
});
let offer = null, appOffer = null, busy = false, refreshing = false, restarting = false;
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
function show(view) {
  $('report-view').hidden = view !== 'report';
  $('update-view').hidden = view !== 'update';
  for (const name of ['report','update']) {
    $('nav-'+name).classList.toggle('active',name === view);
    if (name === view) $('nav-'+name).setAttribute('aria-current','page');
    else $('nav-'+name).removeAttribute('aria-current');
  }
}
function updateButtons() {
  const disabled = document.body.dataset.updatesDisabled === 'true';
  $('refresh').disabled = refreshing || switchingFolder || switchingAccount;
  $('account-select').disabled = busy || !lastReport || !(lastReport.Accounts || []).length;
  $('check-app-update').disabled = busy || disabled;
  $('apply-app-update').disabled = busy || disabled || !appOffer || !appOffer.Available;
  $('check-update').disabled = busy || disabled;
  $('apply-update').disabled = busy || disabled || !offer || !offer.Available;
  for (const id of ['browse-folder','use-folder','game-path']) $(id).disabled = busy;
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
    row.querySelector('.extra-status').textContent = (o.Status || '') + (o.Available ? ' Close WoW, then click '+(o.Install?'Install':'Update')+'.' : ' No files changed.');
  } catch(err) { row.querySelector('.extra-status').textContent = 'Check failed: '+err.message; }
  finally { busy = false; updateButtons(); }
}
async function applyExtra(id) {
  const o = extraOffers[id];
  if (busy || !o || !o.Available) return;
  busy = true; updateButtons();
  const row = document.getElementById('extra-'+id);
  row.querySelector('.extra-status').textContent = 'Downloading, verifying and installing…';
  try {
    const result = await rc.applyExtra(id, o.Version);
    row.querySelector('.extra-status').textContent = result.message + (result.backupPath ? '\nBackup: '+result.backupPath : '');
  } catch(err) { row.querySelector('.extra-status').textContent = 'Install failed: '+err.message+' Check again before retrying.'; }
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
  $('installed-version').textContent = result.InstalledVersion === '0.0.0' ? 'Not installed' : result.InstalledVersion;
  $('folder-label').textContent = result.Game;
  if (folderChanged || document.activeElement !== $('game-path')) $('game-path').value = result.Game;
  if (folderChanged) {
    offer = null;
    $('offered-version').textContent = 'Not checked';
    $('release-link').hidden = true;
    $('release-notes').textContent = 'Check for updates for this game folder.';
    $('update-status').textContent = 'Game folder changed. Check again before updating.';
    for (const k of Object.keys(extraOffers)) delete extraOffers[k];
    if ($('extra-list').children.length) loadExtras();
    updateButtons();
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
    acknowledgeRenderedShell();
    if (manual) message('Report refreshed. The game must save before new data appears.');
  }
  catch (err) { if (generation === reportGeneration) message('Could not refresh the report: '+err.message); }
  finally { refreshing = false; updateButtons(); }
}
$('nav-report').addEventListener('click',()=>show('report'));
$('nav-update').addEventListener('click',()=>{ show('update'); if (!busy) loadExtras(); });
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
    if (lastReport) displayAccount(lastReport);
    busy = switchingFolder = false;
    updateButtons();
  }
}
$('game-form').addEventListener('submit',event => {
  event.preventDefault();
  changeFolder(()=>rc.setGame($('game-path').value));
});
$('browse-folder').addEventListener('click',()=>changeFolder(rc.chooseGame));
$('account-select').addEventListener('change',async()=>{
  if (busy) return;
  const id = $('account-select').value;
  if (!id || (lastReport && id === lastReport.Account)) return;
  busy = switchingAccount = true;
  const generation = ++reportGeneration;
  // An account switch is a data boundary, including failed reads or writes.
  $('report-body').replaceChildren();
  renderedReport = null;
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
$('check-update').addEventListener('click',async()=>{
  if (busy) return;
  busy = true; offer = null; updateButtons();
  $('release-link').hidden = true;
  $('offered-version').textContent = 'Checking…';
  $('update-status').textContent = 'Checking the latest stable GitHub release…';
  try {
    offer = await rc.checkAddon();
    $('installed-version').textContent = offer.InstalledVersion === '0.0.0' ? 'Not installed' : offer.InstalledVersion;
    $('offered-version').textContent = offer.Available ? offer.Version : 'None';
    $('release-name').textContent = offer.Name || 'Release notes';
    $('release-notes').textContent = offer.Notes || 'No release notes provided.';
    $('release-link').href = offer.HTMLURL;
    $('release-link').hidden = !offer.HTMLURL;
    $('update-status').textContent = offer.Available ? 'Update available. Close WoW, then click Update Addon to verify, back up and install.' : (offer.Notes || 'No compatible new addon update is available.') + ' No files changed.';
  } catch(err) {
    $('offered-version').textContent = 'Unavailable';
    $('release-notes').textContent = 'Release notes unavailable. Check again when online.';
    $('update-status').textContent = 'Update check failed: '+err.message;
  } finally { busy = false; updateButtons(); }
});
$('apply-update').addEventListener('click',async()=>{
  if (busy || !offer || !offer.Available) return;
  busy = true; updateButtons();
  $('update-status').textContent = 'Downloading, verifying and installing… Do not force-close the application.';
  try {
    // Only confirm the displayed version. URLs/checksums stay in Go memory.
    const result = await rc.applyAddon(offer.Version);
    $('update-status').textContent = result.message + (result.backupPath ? '\nBackup: '+result.backupPath : '');
    await refresh();
  } catch(err) { $('update-status').textContent = 'Update failed: '+err.message+' Check for updates before trying again.'; }
  finally { offer = null; busy = false; updateButtons(); }
});
$('check-app-update').addEventListener('click',async()=>{
  if (busy || document.body.dataset.updatesDisabled === 'true') return;
  busy = true; appOffer = null; updateButtons();
  $('app-release-link').hidden = true;
  $('offered-app-version').textContent = 'Checking…';
  $('app-update-status').textContent = 'Checking the latest stable app release…';
  try {
    appOffer = await rc.checkApp();
    $('installed-app-version').textContent = appOffer.InstalledVersion;
    $('offered-app-version').textContent = appOffer.Version || 'Unavailable';
    $('app-release-name').textContent = appOffer.Name || 'App release notes';
    $('app-release-notes').textContent = appOffer.Notes || 'No release notes provided.';
    $('app-release-link').href = appOffer.HTMLURL;
    $('app-release-link').hidden = !appOffer.HTMLURL;
    $('app-update-status').textContent = appOffer.Available
      ? 'Click Update App to download, verify, back up and restart Relax Coach. WoW can stay open.'
      : 'No compatible new app update is available. See release notes. No files changed.';
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
refresh();
})();
