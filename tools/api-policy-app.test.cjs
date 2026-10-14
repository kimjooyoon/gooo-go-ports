// Actual app source with simulated DOM/transport scheduling. These tests do not
// establish Gooo, Wasm, Chrome, file-picker or completed download acceptance.
'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { createHash } = require('node:crypto');
const ROOT = path.resolve(__dirname, '..');
const source = fs.readFileSync(path.join(ROOT, 'site/api-policy.js'), 'utf8');
const html = fs.readFileSync(path.join(ROOT, 'site/api-policy.html'), 'utf8');
const sha = raw => createHash('sha256').update(raw).digest('hex');
const bytes = raw => new Uint8Array(raw);
const clone = value => JSON.parse(JSON.stringify(value));
const flush = async () => { for (let i = 0; i < 8; i++) await new Promise(resolve => setImmediate(resolve)); };
const facts = { group_present: true, group_value: 'admin', principal_present: true, principal_value: 'synthetic:user', authenticated: true,
  caller_tenant: 'tenant-a', resource_tenant: 'tenant-a', requested_action: 'read', allowed_action: 'read' };
const decision = { allowed: true, reason: 'ALLOW', authenticated: true, tenant_present: true, tenant_matched: true, action_present: true,
  action_matched: true, access_allowed: true, access_error: false, access_result: 'ALLOW', group_matched: true, principal_matched: false,
  group_missing: false, principal_defaulted: false, effective_principal: 'synthetic:user' };
function binding(root) {
  const raw = fs.readFileSync(path.join(root, 'generation.json')), generation = JSON.parse(raw);
  return { compiler_source_sha: '83928fa4538245abe33b08549da47f8cbdc50ed2', generation_sha256: 'sha256:' + sha(raw), assets: generation.assets };
}
function metadata() {
  const module = path.join(ROOT, 'examples/api-policy-revision');
  return { schema: 'gooo/api-policy-browser-bridge-info/v1', scope: 'unsigned_historical_hypothetical_comparison', modes: ['baseline', 'compare', 'replay'],
    profiles: { original: 'cel-api-original-v1', candidate: 'cel-api-readonly-v1' }, limits: { text_utf8_bytes: 65536, baseline_input_bytes: 4194304,
      compare_input_bytes: 4194304, replay_input_bytes: 16777216, original_proof_bytes: 4194304, native_facts_raw_bytes: 3539232, output_bytes: 67108864 },
    bindings: { original: binding(path.join(module, 'profiles/original')), revision: binding(module) } };
}
function report() {
  const info = metadata();
  return { schema: 'gooo/api-policy-revision-comparison/v1', status: 'COMPARED', scope: 'same_saved_facts_two_sealed_profiles',
    before_profile: info.profiles.original, candidate_profile: info.profiles.candidate,
    before: { facts: clone(facts), decision: clone(decision), bindings: info.bindings.original },
    after: { facts: clone(facts), decision: clone(decision), bindings: info.bindings.revision },
    differences: Object.fromEntries(Object.keys(decision).map(key => [key, false])), summary: { classification: 'UNCHANGED', changed_count: 0 },
    classifier_bindings: info.bindings.revision };
}
class Element {
  constructor() { this.value = ''; this.hidden = false; this.disabled = false; this.dataset = {}; this.children = []; this.listeners = {}; this._text = ''; }
  get textContent() { return this._text + this.children.map(child => child.textContent).join(''); }
  set textContent(text) { this._text = String(text); this.children = []; }
  get lastChild() { return this.children.at(-1); }
  append(...children) { for (const child of children) { child.parent = this; this.children.push(child); } }
  replaceChildren(...children) { this.children = []; this._text = ''; this.append(...children); }
  remove() { if (this.parent) this.parent.children = this.parent.children.filter(child => child !== this); }
  addEventListener(name, handler) { this.listeners[name] = handler; }
  emit(name, data = {}) { return this.listeners[name]?.({ preventDefault() {}, ...data }); }
}
function app(options = {}) {
  const elements = new Map(), workers = [], urls = new Map(), revoked = [], downloads = [], timers = new Map(), windowEvents = {}, fetches = [];
  let serial = 0, encoded = 0;
  const get = id => { if (!elements.has(id)) elements.set(id, new Element()); return elements.get(id); };
  get('save').disabled = true; get('result').hidden = true; get('replay').disabled = true;
  const workerRaw = Buffer.from('actual app transport fixture; not executable Wasm');
  const descriptor = (name, raw) => ({ path: name, bytes: raw.length, sha256: sha(raw) });
  const manifest = { schema: 'gooo/browser-api-policy-revision/v1', scope: 'unsigned_historical_hypothetical_comparison', source: {}, bridge: {}, build_verification: {},
    files: { 'worker.js': descriptor('../api-policy-worker.js', workerRaw), 'program.wasm': descriptor('./program.wasm', Buffer.from('wasm')),
      'wasm_exec.js': descriptor('./wasm_exec.js', Buffer.from('runtime')) }, expected_info: metadata(), limits: {} };
  const response = raw => ({ ok: true, body: { getReader() { let done = false; return { async read() { if (done) return { done: true }; done = true; return { done: false, value: bytes(raw) }; }, releaseLock() {} }; } } });
  class URLAdapter extends URL {}
  URLAdapter.createObjectURL = blob => { const url = 'blob:http://api-policy.test/' + (++serial); urls.set(url, blob); return url; };
  URLAdapter.revokeObjectURL = url => { revoked.push(url); urls.delete(url); };
  class OwnedWorker {
    constructor(url) { assert.ok(urls.has(url)); this.url = url; this.listeners = {}; this.messages = []; this.terminated = false; workers.push(this); }
    addEventListener(name, callback) { this.listeners[name] = callback; }
    terminate() { this.terminated = true; }
    postMessage(message, transfer) { this.messages.push(structuredClone(message, { transfer })); }
    deliver(message) { this.listeners.message({ data: message }); }
  }
  class CheckedEncoder extends TextEncoder { encode(text) { encoded++; return super.encode(text); } }
  const body = new Element();
  const context = { URL: URLAdapter, location: new URL('http://api-policy.test/api-policy.html'), Worker: OwnedWorker, Blob,
    ArrayBuffer, Uint8Array, TextEncoder: CheckedEncoder, TextDecoder, AbortController, WebAssembly,
    crypto: { subtle: { async digest(name, raw) { assert.equal(name, 'SHA-256'); return bytes(createHash('sha256').update(Buffer.from(raw)).digest()).buffer; } } },
    async fetch(url, config) { fetches.push({ url: String(url), config });
      if (options.deferFetch) await options.deferFetch;
      if (String(url).endsWith('/api-policy/manifest.json')) return response(Buffer.from(JSON.stringify(manifest)));
      assert.equal(String(url), 'http://api-policy.test/api-policy-worker.js');
      return response(options.badWorkerHash ? Buffer.from(workerRaw.map((value, i) => i ? value : value ^ 1)) : workerRaw);
    },
    setTimeout(fn, delay) { const id = ++serial; timers.set(id, { fn, delay }); return id; }, clearTimeout(id) { timers.delete(id); },
    document: { body, getElementById: get, createDocumentFragment: () => new Element(), createElement(tag) { const item = new Element();
      if (tag === 'a') item.click = () => downloads.push({ href: item.href, name: item.download, blob: urls.get(item.href) }); return item; } } };
  context.window = { addEventListener(name, handler) { windowEvents[name] = handler; } };
  vm.runInNewContext(source, context, { filename: 'site/api-policy.js' });
  const run = async (operation = 'compare') => { get(operation).emit('click'); await flush(); return workers.at(-1); };
  const result = (worker = workers.at(-1), raw = Buffer.from(JSON.stringify(report())), changes = {}) => {
    const request = worker.messages.at(-1);
    worker.deliver({ type: 'result', id: request.id, operation: request.operation, bytes: bytes(raw).buffer, ...changes });
  };
  const load = async (kind, raw, deferred) => { const input = get(kind === 'facts' ? 'facts-file' : 'report-file');
    input.files = [{ name: 'raw.json', size: raw.length, arrayBuffer: () => deferred || Promise.resolve(bytes(raw).buffer) }]; input.emit('change'); await flush(); };
  return { get, workers, run, result, load, timers, downloads, revoked, urls, manifest, fetches, encoded: () => encoded,
    edit(text) { get('facts').value = text; get('facts').emit('input'); }, cancel() { get('cancel').emit('click'); },
    save() { get('save').emit('click'); }, pagehide() { windowEvents.pagehide(); } };
}

test('actual editor authority is displayed text and UTF16 refuses before TextEncoder or Worker', async () => {
  assert.match(html, /Browsers may fold raw line endings; use a file for exact input bytes/);
  for (const text of ['\ud800', '\udc00', '{"x":"\ud800x"}']) {
    const h = app(); h.edit(text); await h.run();
    assert.equal(h.encoded(), 0); assert.equal(h.workers.length, 0); assert.match(h.get('status').textContent, /unpaired UTF-16/);
    assert.equal(h.get('save').disabled, true);
  }
  const h = app(), text = ' \r\n' + JSON.stringify({ ...facts, principal_value: '😀e\u0301\ufffd\0\r\n' }) + '\n';
  h.edit(text); const worker = await h.run();
  assert.deepEqual(Buffer.from(worker.messages[0].bytes), Buffer.from(text)); assert.equal(h.encoded(), 1);
});
test('raw facts and report loading neither calculates nor normalizes bytes', async () => {
  for (const kind of ['facts', 'report']) {
    const h = app(), raw = Buffer.from([0xef, 0xbb, 0xbf, 0xc0, 0xaf, 0, 13, 10]);
    await h.load(kind, raw); assert.equal(h.workers.length, 0); assert.equal(h.fetches.length, 0); assert.equal(h.encoded(), 0);
    assert.equal(h.get('save').disabled, true); assert.match(h.get('status').textContent, /unverified/);
    const worker = await h.run(kind === 'facts' ? 'compare' : 'replay');
    assert.deepEqual(Buffer.from(worker.messages[0].bytes), raw);
    assert.equal(worker.messages[0].operation, kind === 'facts' ? 'compare' : 'replay');
  }
});
test('file routes preserve the entire declared byte bounds and reject plus one before transport', async () => {
  for (const [kind, limit, operation] of [['facts', 4194304, 'compare'], ['report', 16777216, 'replay']]) {
    const h = app(), raw = Buffer.alloc(limit, 32); raw[0] = 123;
    await h.load(kind, raw); const worker = await h.run(operation);
    assert.deepEqual(Buffer.from(worker.messages[0].bytes), raw);
    const oldCount = h.workers.length; await h.load(kind, Buffer.alloc(limit + 1, 32));
    assert.equal(h.workers.length, oldCount); assert.equal(h.get('save').disabled, true); assert.match(h.get('status').textContent, /exceeds/);
  }
});
test('new edits, samples, cancellation, deadline and pagehide cannot revive stale results or downloads', async () => {
  const controls = [h => h.edit(JSON.stringify(facts)), h => h.get('sample-write').emit('click'), h => h.cancel(),
    h => [...h.timers.values()][0].fn(), h => h.pagehide()];
  for (const invalidate of controls) {
    const h = app(), old = await h.run(); invalidate(h); assert.equal(old.terminated, true);
    h.result(old); assert.equal(h.get('save').disabled, true); assert.equal(h.get('result').hidden, true);
    h.save(); assert.equal(h.downloads.length, 0);
  }
});
test('file-read and initialization epochs suppress late continuations before Worker creation', async () => {
  let release; const pending = new Promise(resolve => { release = resolve; });
  const h = app(); await h.load('facts', Buffer.from('{}'), pending); h.cancel(); release(bytes(Buffer.from('{}')).buffer); await flush();
  assert.equal(h.get('facts').disabled, false); assert.equal(h.get('save').disabled, true); assert.equal(h.workers.length, 0);
  let resume; const delayed = new Promise(resolve => { resume = resolve; });
  const j = app({ deferFetch: delayed }); j.get('compare').emit('click'); await flush(); j.cancel(); resume(); await flush();
  assert.equal(j.workers.length, 0); assert.equal(j.get('save').disabled, true);
});
test('a newer run owns readiness and an older complete result cannot replace it', async () => {
  const h = app(), old = await h.run(), current = await h.run();
  assert.notEqual(current, old); assert.equal(old.terminated, true);
  h.result(old); assert.equal(h.get('save').disabled, true); assert.equal(h.get('result').hidden, true);
  h.result(current); assert.equal(h.get('save').disabled, false); assert.equal(current.terminated, true);
});
test('only complete current bound results allow exact returned bytes to be downloaded', async () => {
  const h = app(); h.save(); assert.equal(h.downloads.length, 0);
  const worker = await h.run(), raw = Buffer.from(JSON.stringify(report()) + '\n'); h.result(worker, raw);
  assert.equal(h.get('decision-rows').children.length, 1); // one fragment, with all fifteen rows
  assert.equal(h.get('decision-rows').children[0].children.length, 15);
  assert.equal(h.get('save').disabled, false); h.save(); assert.equal(h.downloads.length, 1);
  assert.deepEqual(Buffer.from(await h.downloads[0].blob.arrayBuffer()), raw);
  const firstURL = h.downloads[0].href; h.edit(JSON.stringify(facts)); assert.ok(h.revoked.includes(firstURL));
  assert.equal(h.get('save').disabled, true); h.save(); assert.equal(h.downloads.length, 1);
});
test('wrong request/result identity, missing scalar, native refusal and changed binding leave no downloadable result', async () => {
  for (const mutate of [value => { delete value.after.decision.allowed; }, value => { value.classifier_bindings.assets.extra = 'sha256:' + '0'.repeat(64); }]) {
    const h = app(), worker = await h.run(), value = report(); mutate(value); h.result(worker, Buffer.from(JSON.stringify(value)));
    assert.equal(h.get('save').disabled, true); assert.equal(h.get('result').hidden, true);
  }
  const h = app(), worker = await h.run(); h.result(worker, undefined, { id: 999 });
  assert.equal(h.get('save').disabled, true); h.result(worker, undefined, { operation: 'replay' }); assert.equal(h.get('save').disabled, true);
  const j = app(), native = await j.run(); native.deliver({ type: 'failure', id: native.messages[0].id, message: 'typed saved report mismatch', incomplete: false });
  assert.match(j.get('status').textContent, /Native refusal/); assert.equal(j.get('save').disabled, true);
});
test('actual app verifies Worker bytes before creating a Worker', async () => {
  const h = app({ badWorkerHash: true }); await h.run(); assert.equal(h.workers.length, 0);
  assert.match(h.get('status').textContent, /Worker source hash mismatch/); assert.equal(h.get('save').disabled, true);
});
