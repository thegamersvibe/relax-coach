'use strict';
const {test} = require('node:test');
const assert = require('node:assert/strict');
const {readFileSync} = require('node:fs');
const {JSDOM} = require('jsdom');
const html = readFileSync(process.env.NATIVE_HTML_OUT + '.retirement', 'utf8');
const settle = () => new Promise(resolve => setTimeout(resolve, 10));
function boot() {
  const calls = [], frames = [];
  let receive;
  const dom = new JSDOM(html, {runScripts: 'dangerously', beforeParse(window) {
    window.chrome = {webview: {
      addEventListener(kind, fn) { assert.equal(kind, 'message'); receive = fn; },
      postMessage(raw) { calls.push(JSON.parse(raw)); }
    }};
    window.requestAnimationFrame = fn => frames.push(fn);
    window.fetch = () => { throw new Error('network forbidden'); };
  }});
  return {dom, calls, frames, reply(index, result, error) { receive({data: {kind: 'native-response', id: calls[index].id, result, error}}); }};
}
test('retirement renders before health, never checks updates, and escapes removal paths', async () => {
  const x = boot();
  try {
    assert.deepEqual(x.calls.map(c => c.method), ['reportRefresh']);
    x.reply(0, 'Relax Coach is retired. Its addons have been removed from this PC.');
    await settle();
    assert.equal(x.calls.length, 1);
    assert.equal(x.frames.length, 1);
    x.frames.shift()();
    assert.equal(x.calls.length, 1);
    x.frames.shift()();
    await settle();
    assert.deepEqual(x.calls.map(c => c.method), ['reportRefresh', 'appReady']);
    x.reply(1, {Removed: ['ForeverCoach', '<img src=x onerror=evil()>'], Skipped: ['skipped linked Journey'], SavedSettingsDeferred: true});
    await settle();
    const doc = x.dom.window.document;
    assert.deepEqual([...doc.querySelectorAll('#removed li')].map(e => e.textContent), ['ForeverCoach', '<img src=x onerror=evil()>']);
    assert.equal(doc.querySelectorAll('img,button,a,nav,input').length, 0);
    assert.equal(doc.querySelector('#status').textContent, 'close WoW and start Relax Coach again to remove saved settings');
    assert.equal(x.calls.length, 2);
  } finally { x.dom.window.close(); }
});
test('failed initial bridge never asks for health or deletion', async () => {
  const x = boot();
  try {
    x.reply(0, null, 'bridge failed');
    await settle();
    assert.equal(x.calls.length, 1);
    assert.equal(x.frames.length, 0);
    assert.match(x.dom.window.document.querySelector('#status').textContent, /bridge failed/);
  } finally { x.dom.window.close(); }
});
test('empty run says nothing left and unresolved folder is explicit', async () => {
  const x = boot();
  try {
    x.reply(0, 'Relax Coach is retired. Its addons have been removed from this PC.');
    await settle(); x.frames.shift()(); x.frames.shift()(); await settle();
    x.reply(1, {NoGame: true}); await settle();
    assert.equal(x.dom.window.document.querySelector('#status').textContent, 'nothing left to remove. no game folder resolved; nothing was deleted');
  } finally { x.dom.window.close(); }
});
