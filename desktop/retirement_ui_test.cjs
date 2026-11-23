'use strict';
const {test} = require('node:test');
const assert = require('node:assert/strict');
const {readFileSync} = require('node:fs');
const {JSDOM} = require('jsdom');
const html = readFileSync(process.env.NATIVE_HTML_OUT + '.retirement', 'utf8');
const settle = () => new Promise(resolve => setTimeout(resolve, 10));
function boot(disabled = false) {
  const calls = [], frames = [];
  let receive;
  const dom = new JSDOM(disabled ? html.replace('data-updates-disabled="false"', 'data-updates-disabled="true"') : html, {runScripts: 'dangerously', beforeParse(window) {
    window.chrome = {webview: {
      addEventListener(kind, fn) { assert.equal(kind, 'message'); receive = fn; },
      postMessage(raw) { calls.push(JSON.parse(raw)); }
    }};
    window.requestAnimationFrame = fn => frames.push(fn);
    window.fetch = () => { throw new Error('network forbidden'); };
  }});
  return {dom, calls, frames, reply(index, result, error) { receive({data: {kind: 'native-response', id: calls[index].id, result, error}}); }};
}
test('retirement renders before health, only checks app updates, and escapes moved paths', async () => {
  const x = boot();
  try {
    assert.deepEqual(x.calls.map(c => c.method), ['reportRefresh']);
    x.reply(0, 'Relax Coach is retired. Its addons have been moved out of the game (kept in RelaxCoachRetired, not deleted).');
    await settle();
    assert.equal(x.calls.length, 1);
    assert.equal(x.frames.length, 1);
    x.frames.shift()();
    assert.equal(x.calls.length, 1);
    x.frames.shift()();
    await settle();
    assert.deepEqual(x.calls.map(c => c.method), ['reportRefresh', 'appReady']);
    x.reply(1, {Moved: ['ForeverCoach', '<img src=x onerror=evil()>'], Skipped: ['skipped linked Journey'], SavedSettingsDeferred: true});
    await settle();
    const doc = x.dom.window.document;
    assert.deepEqual([...doc.querySelectorAll('#moved li')].map(e => e.textContent), ['ForeverCoach', '<img src=x onerror=evil()>']);
    assert.equal(doc.querySelectorAll('img,a,nav,input').length, 0);
    assert.equal(doc.querySelector('#status').textContent, 'close WoW and start Relax Coach again to move saved settings (WoW is open or its process state is unreadable)');
    assert.deepEqual(x.calls.map(c => c.method), ['reportRefresh', 'appReady', 'checkAppStartup']);
  } finally { x.dom.window.close(); }
});
test('newer app shows Update App and applies only on a click', async () => {
 const x = boot();
 try {
  x.reply(0, 'retired'); await settle(); x.frames.shift()(); x.frames.shift()(); await settle();
  x.reply(1, {Moved: []}); await settle();
  assert.equal(x.calls[2]?.method, 'checkAppStartup');
  const button = x.dom.window.document.querySelector('#update-app');
  assert.equal(button.hidden, true);
  x.reply(2, {Available: true, Version: '1.9.1'}); await settle();
  assert.equal(button.hidden, false); assert.equal(button.textContent, 'Update App');
  assert.equal(x.calls.length, 3);
  button.click(); await settle();
  assert.equal(x.calls[3].method, 'applyApp'); assert.deepEqual(x.calls[3].params, ['1.9.1']);
  x.reply(3, {RestartReady: true}); await settle();
  assert.equal(button.disabled, true);
  assert.equal(x.calls.length, 4);
 } finally { x.dom.window.close(); }
});
test('no-update does not check or show an update action', async () => {
 const x = boot(true);
 try {
  x.reply(0, 'retired'); await settle(); x.frames.shift()(); x.frames.shift()(); await settle();
  x.reply(1, {}); await settle();
  assert.equal(x.calls.length, 2);
  assert.equal(x.dom.window.document.querySelector('#update-app').hidden, true);
 } finally { x.dom.window.close(); }
});
test('failed initial bridge never asks for health or moves' , async () => {
  const x = boot();
  try {
    x.reply(0, null, 'bridge failed');
    await settle();
    assert.equal(x.calls.length, 1);
    assert.equal(x.frames.length, 0);
    assert.match(x.dom.window.document.querySelector('#status').textContent, /bridge failed/);
  } finally { x.dom.window.close(); }
});
test('invalid pending journal reports a blocked recovery, never an empty or retryable run', async () => {
  const x = boot();
  try {
    x.reply(0, 'retired'); await settle(); x.frames.shift()(); x.frames.shift()(); await settle();
    x.reply(1, {Moved: [], RecoveryBlocked: true, Skipped: ['invalid retirement journal']}); await settle();
    assert.equal(x.dom.window.document.querySelector('#status').textContent, 'a previous move could not be verified; nothing more was moved');
    assert.doesNotMatch([...x.dom.window.document.querySelectorAll('#status, #skipped')].map(e => e.textContent).join(' '), /nothing left to move|will retry/);
    assert.deepEqual(x.calls.map(c => c.method), ['reportRefresh', 'appReady', 'checkAppStartup']);
  } finally { x.dom.window.close(); }
});
test('empty run says nothing left and unresolved folder is explicit', async () => {
  const x = boot();
  try {
    x.reply(0, 'Relax Coach is retired. Its addons have been moved out of the game (kept in RelaxCoachRetired, not deleted).');
    await settle(); x.frames.shift()(); x.frames.shift()(); await settle();
    x.reply(1, {NoGame: true}); await settle();
    assert.equal(x.dom.window.document.querySelector('#status').textContent, 'nothing left to move. no game folder resolved; nothing was moved');
  } finally { x.dom.window.close(); }
});
