// Simulated DOM scheduling and presentation checks with typed stub results.
// These are not Gooo execution evidence; service-policy-smoke.cjs runs Wasm.
'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { createHash } = require('node:crypto');
const protocol = require('../site/service-policy-protocol.js');
const { expectedBindings } = require('./service-policy-smoke.cjs');
const source = fs.readFileSync(path.join(__dirname, '../site/service-policy.js'), 'utf8');
const html = fs.readFileSync(path.join(__dirname, '../site/service-policy.html'), 'utf8');
const clone = value => JSON.parse(JSON.stringify(value));
function metadata() {
  const bindings = expectedBindings(), files = {};
  for (const name of protocol.FILE_NAMES) {
    const raw = name.startsWith('source/') ? fs.readFileSync(path.join(__dirname, '../examples/agentmesh-service-policy', name.slice(7))) : Buffer.from('synthetic schema fixture');
    files[name] = { sha256: createHash('sha256').update(raw).digest('hex'), bytes: raw.length };
  }
  return { schema: 'gooo/browser-service-policy/v1', compiler_revision: bindings.compiler_source_sha, go_version: 'go1.27.3',
    source_revision: 'a'.repeat(40), source_dirty: true, generation_sha256: bindings.generation_sha256, bindings,
    wasm_sha256: files['program.wasm'].sha256, wasm_bytes: files['program.wasm'].bytes,
    wasm_exec_sha256: files['wasm_exec.js'].sha256, wasm_exec_bytes: files['wasm_exec.js'].bytes, files };
}
class Element {
  constructor() { this._text = ''; this.value = ''; this.hidden = false; this.disabled = false; this.children = []; this.dataset = {}; this.listeners = {}; this.attributes = {}; }
  get textContent() { return this._text + this.children.map(child => child.textContent).join(''); }
  set textContent(value) { this._text = String(value); this.children = []; }
  append(...children) { children.forEach(child => { child.parent = this; }); this.children.push(...children); }
  replaceChildren(...children) { this._text = ''; this.children = []; this.append(...children); }
  remove() { if (this.parent) this.parent.children = this.parent.children.filter(child => child !== this); }
  setAttribute(name, value) { this.attributes[name] = value; }
  addEventListener(name, handler) { this.listeners[name] = handler; }
  emit(name, event = {}) { return this.listeners[name]?.({ preventDefault() {}, ...event }); }
}
function app(options = {}) {
  const elements = new Map(), workers = [], timers = new Map(), events = new Map(), urls = [], revoked = [], downloads = [], body = new Element();
  let nextTimer = 0;
  const get = id => { if (!elements.has(id)) elements.set(id, new Element()); return elements.get(id); };
  const defaults = { mode: 'match', pattern: 'mcp://weather', before: 'mcp://*.x', after: 'mcp://a.*', req_type: 'mcp', req_name: 'weather' };
  for (const [name, value] of Object.entries(defaults)) get('policy-' + name).value = value;
  const fields = ['pattern', 'before', 'after', 'req_type', 'req_name'].map(name => { const row = new Element(); row.dataset.policyField = name; return row; });
  const presets = ['match', 'denied', 'same-request', 'witness', 'subset', 'invalid'].map(name => { const button = new Element(); button.dataset.policyPreset = name; return button; });
  class Worker {
    constructor(url) { if (options.workerError) throw new Error('no Worker'); assert.equal(url, './service-policy-worker.js'); this.messages = []; this.terminated = false; workers.push(this); }
    terminate() { this.terminated = true; }
    postMessage(message) { if (options.postError) throw new Error('no transport'); this.messages.push(clone(message)); }
    deliver(message) { this.onmessage({ data: message }); }
  }
  const context = { Worker, GoooServicePolicyProtocol: protocol, TextDecoder,
    Blob: class extends Blob { constructor(...args) { if (options.blobError) throw new Error('no Blob'); super(...args); } },
    URL: { createObjectURL(blob) { if (options.urlError) throw new Error('no URL'); const url = 'blob:policy-' + (urls.length + 1); urls.push({ url, blob }); return url; }, revokeObjectURL(url) { revoked.push(url); } },
    setTimeout(handler, delay) { const id = ++nextTimer; timers.set(id, { handler, delay }); return id; }, clearTimeout(id) { timers.delete(id); },
    addEventListener(name, handler) { events.set(name, handler); },
    document: { body, getElementById: get, createElement(tag) { const value = new Element(); if (tag === 'a') value.click = () => { if (options.clickError) throw new Error('blocked'); downloads.push({ href: value.href, filename: value.download, attached: body.children.includes(value) }); }; return value; },
      querySelectorAll(selector) { if (selector === '[data-policy-field]') return fields; assert.equal(selector, '[data-policy-preset]'); return presets; } } };
  context.window = context;
  vm.runInNewContext(source, context, { filename: 'site/service-policy.js' });
  const current = () => workers.at(-1);
  const ready = (worker = current(), program = metadata()) => worker.deliver({ type: 'ready', program });
  const type = (name, value) => { get('policy-' + name).value = value; get('policy-' + name).emit('input'); };
  const mode = value => { get('policy-mode').value = value; get('policy-mode').emit('change'); };
  const run = () => { get('policy-form').emit('submit'); return current()?.messages.at(-1); };
  const reply = (decision = { domain_status: 'VALID_INPUT', invalid_field: '', matched: false }, worker = current(), changes = {}) => {
    const request = worker.messages.at(-1);
    worker.deliver({ type: 'result', id: request.id, mode: request.mode,
      proof: { facts: clone(request.facts), decision, bindings: metadata().bindings }, program: metadata(), ...changes });
  };
  const transfer = (name, text, kind = 'paste', extra = {}) => {
    let prevented = false;
    get('policy-' + name).emit(kind, { preventDefault() { prevented = true; },
      [kind === 'paste' ? 'clipboardData' : 'dataTransfer']: { types: ['text/plain'], getData() { return text; }, ...extra } });
    assert.equal(prevented, true);
  };
  return { get, fields, workers, current, ready, type, mode, run, reply, transfer, timers, urls, revoked, downloads, body,
    select: files => { get('policy-observation-file').files = Array.isArray(files) ? files : [files]; return get('policy-observation-file').emit('change'); },
    preset: name => presets.find(button => button.dataset.policyPreset === name).emit('click'), download: () => get('policy-download').emit('click'),
    pagehide: () => events.get('pagehide')(), pageshow: () => events.get('pageshow')({ persisted: true }) };
}
const invalid = { domain_status: 'INVALID_INPUT', invalid_field: 'pattern', matched: false };
const witness = { domain_status: 'VALID_INPUT', invalid_field: '', status: 'WIDENING_WITNESS', rule_id: 'CHECKED_CANDIDATE_REQUEST',
  supported_subset: false, has_witness: true, witness_type: 'mcp', witness_name: 'a.y', before_match: false, after_match: true };

test('valid denial and invalid input have distinct readable summaries with complete fields', () => {
  const h = app(); assert.equal(h.get('policy-run').disabled, true); h.ready(); h.run(); h.reply();
  assert.match(h.get('policy-summary').textContent, /input is valid/); assert.equal(h.get('policy-fields').children.length, 3);
  assert.equal(h.get('policy-fields').children[0].children[1].textContent, '"VALID_INPUT"');
  assert.equal(h.get('policy-download').disabled, false);
  h.preset('invalid'); h.run(); h.reply(invalid);
  assert.match(h.get('policy-summary').textContent, /rejected the input in pattern/);
  assert.equal(h.get('policy-error').hidden, true); assert.equal(h.get('policy-download').disabled, false);
});
test('request comparison and grant witness keep their distinct scopes and all returned fields', () => {
  const h = app(); h.ready(); h.preset('same-request'); assert.equal(h.fields.find(row => row.dataset.policyField === 'pattern').hidden, true);
  const request = h.run(); assert.deepEqual(request.facts, { before: 'mcp://*.x', after: 'mcp://a.*', req_type: 'mcp', req_name: 'a.x' });
  h.reply({ domain_status: 'VALID_INPUT', invalid_field: '', before_match: true, after_match: true, change: 'UNCHANGED_ALLOW' });
  assert.match(h.get('policy-summary').textContent, /Other requests may behave differently/);
  h.preset('witness'); assert.equal(h.get('policy-result').hidden, true); assert.equal(h.get('policy-download').disabled, true);
  h.run(); h.reply(witness); assert.match(h.get('policy-summary').textContent, /mcp:\/\/a.y/); assert.equal(h.get('policy-fields').children.length, 10);
});
test('Unicode and oversized policy-domain input reach transport unchanged; bad UTF16 stays visibly refused', () => {
  const h = app(); h.ready();
  for (const value of ['é', '😀', '\0\r\n', 'x'.repeat(129), 'x'.repeat(4096)]) {
    h.type('req_name', value); assert.equal(h.get('policy-run').disabled, false);
    assert.equal(h.run().facts.req_name, value); h.reply({ ...invalid, invalid_field: 'req_name' });
  }
  for (const value of ['\ud800', '\udc00', 'x'.repeat(4097)]) {
    assert.doesNotThrow(() => h.type('req_name', value)); assert.equal(h.get('policy-run').disabled, true);
    assert.equal(h.get('policy-download').disabled, true); assert.equal(h.get('policy-input-error').hidden, false);
  }
});
test('paste/drop preserves pre-editor CRLF and HTML text; no result survives a replaced input', () => {
  const capture = html.match(/<section\b([^>]*\bclass="policy-captured"[^>]*)>([\s\S]*?)<\/section>/);
  assert.ok(capture, 'captured facts must be in an always-visible section');
  assert.doesNotMatch(capture[1], /\bhidden\b/);
  assert.doesNotMatch(capture[2], /<details\b/);
  assert.match(capture[2], /id="policy-input-preview"/); assert.match(capture[2], /id="policy-input-origin"/);
  for (const kind of ['paste', 'drop']) {
    const h = app(); h.ready(); h.run(); h.reply();
    const value = '\uFEFF\r\n<unsafe>&é'; h.transfer('req_name', value, kind);
    assert.equal(h.get('policy-req_name').value, ''); assert.match(h.get('policy-input-preview').textContent, /\\r\\n/);
    assert.ok(h.get('policy-input-preview').textContent.includes('req_name: ' + JSON.stringify(value)));
    assert.equal(h.get('policy-capture-req_name').hidden, false);
    assert.match(h.get('policy-origin-req_name').textContent, /text is captured below/);
    assert.match(h.get('policy-input-origin').textContent, /Request name: (pasted|dropped) plain text/);
    assert.equal(h.get('policy-download').disabled, true); assert.equal(h.get('policy-result').hidden, true);
    assert.equal(h.run().facts.req_name, value);
    h.reply({ ...invalid, invalid_field: 'req_name' });
    assert.ok(h.get('policy-input-preview').textContent.includes('req_name: ' + JSON.stringify(value)));
    assert.equal(h.get('policy-capture-req_name').hidden, false);
    h.transfer('req_name', '\ud800', kind);
    assert.match(h.get('policy-input-preview').textContent, /req_name: "\\ud800"/);
    assert.equal(h.get('policy-input-error').hidden, false); assert.equal(h.get('policy-run').disabled, true);
    assert.equal(h.get('policy-capture-req_name').hidden, false);
    h.transfer('req_name', '', kind);
    assert.match(h.get('policy-input-preview').textContent, /req_name: ""/);
    assert.equal(h.run().facts.req_name, '');
  }
  const h = app(); h.ready(); h.transfer('pattern', '', 'drop', { files: [{}] }); assert.equal(h.get('policy-run').disabled, true);
});
test('capture origins describe only current active fields after edits, mode changes and examples', () => {
  const h = app(); h.ready();
  h.transfer('pattern', 'mcp://*'); h.transfer('req_name', 'é', 'drop');
  assert.match(h.get('policy-input-origin').textContent, /Service pattern: pasted plain text/);
  assert.match(h.get('policy-input-origin').textContent, /Request name: dropped plain text/);
  h.type('req_name', 'weather');
  assert.equal(h.get('policy-capture-req_name').hidden, true);
  assert.equal(h.get('policy-origin-req_name').textContent, '');
  assert.doesNotMatch(h.get('policy-input-origin').textContent, /Request name: dropped/);
  assert.match(h.get('policy-input-origin').textContent, /Service pattern: pasted/);
  h.mode('refine');
  assert.doesNotMatch(h.get('policy-input-origin').textContent, /pasted|dropped/);
  assert.doesNotMatch(h.get('policy-input-preview').textContent, /pattern:|req_name:/);
  h.preset('witness'); h.mode('match');
  assert.match(h.get('policy-input-origin').textContent, /Service pattern: pasted/);
  assert.equal(h.run().facts.pattern, 'mcp://*'); h.reply();
  h.preset('denied');
  assert.doesNotMatch(h.get('policy-input-origin').textContent, /pasted|dropped/);
  assert.equal(h.get('policy-capture-pattern').hidden, true);
  assert.deepEqual(h.run().facts, { pattern: 'mcp://weather', req_type: 'mcp', req_name: 'stocks' }); h.reply();
  h.transfer('req_name', 'é'); h.transfer('req_name', '', 'drop', { files: [{}] });
  assert.equal(h.get('policy-capture-req_name').hidden, true);
  assert.doesNotMatch(h.get('policy-input-origin').textContent, /pasted|dropped/);
  assert.match(h.get('policy-input-preview').textContent, /req_name: ""/);
  assert.equal(h.get('policy-run').disabled, true); assert.equal(h.get('policy-input-error').hidden, false);
});
test('clearing a captured value explicitly replaces it with empty text and invalidates results and active jobs', () => {
  const h = app(); h.ready(); h.transfer('pattern', 'mcp://*'); h.transfer('req_name', 'weather');
  h.run(); h.reply(); assert.equal(h.get('policy-download').disabled, false);
  h.get('policy-clear-req_name').emit('click');
  assert.equal(h.get('policy-capture-req_name').hidden, true);
  assert.equal(h.get('policy-capture-pattern').hidden, false);
  assert.equal(h.get('policy-result').hidden, true); assert.equal(h.get('policy-download').disabled, true);
  assert.match(h.get('policy-input-preview').textContent, /req_name: ""/);
  const old = h.current(); assert.deepEqual(h.run().facts, { pattern: 'mcp://*', req_type: 'mcp', req_name: '' });
  h.get('policy-clear-pattern').emit('click');
  assert.equal(old.terminated, true); assert.notEqual(h.current(), old); h.reply(undefined, old);
  assert.equal(h.get('policy-result').hidden, true); assert.equal(h.get('policy-download').disabled, true);
  assert.doesNotMatch(h.get('policy-input-origin').textContent, /pasted|dropped/);
  h.ready(); assert.equal(h.run().facts.pattern, '');
});
test('input changes and cancellation terminate active Worker and suppress late results', () => {
  for (const change of [h => h.type('req_name', 'new'), h => h.transfer('req_name', 'pasted'), h => h.transfer('req_name', 'dropped', 'drop'), h => h.mode('refine'), h => h.preset('denied'), h => h.get('policy-cancel').emit('click')]) {
    const h = app(); h.ready(); const old = h.current(); h.run(); change(h);
    assert.equal(old.terminated, true); assert.notEqual(h.current(), old); h.reply(undefined, old);
    assert.equal(h.get('policy-result').hidden, true); assert.equal(h.get('policy-download').disabled, true);
    h.ready(); assert.equal(h.get('policy-run').disabled, false);
  }
});
test('stale request IDs do not complete the current job and changed facts or bindings fail closed', () => {
  const h = app(); h.ready(); const request = h.run(); h.reply(undefined, h.current(), { id: request.id - 1 });
  assert.equal(h.get('policy-result').hidden, true); assert.equal(h.get('policy-cancel').hidden, false); h.reply();
  for (const mutate of [result => { result.proof.facts.req_name = 'changed'; }, result => { result.proof.decision.extra = true; }, result => { result.program.source_dirty = false; }]) {
    const c = app(); c.ready(); const request = c.run();
    const result = { type: 'result', id: request.id, mode: request.mode, proof: { facts: clone(request.facts), decision: { domain_status: 'VALID_INPUT', invalid_field: '', matched: false }, bindings: metadata().bindings }, program: metadata() };
    mutate(result); c.current().deliver(result); assert.equal(c.current().terminated, true); assert.equal(c.get('policy-download').disabled, true); assert.equal(c.get('policy-error').hidden, false);
  }
});
test('download retains full proof, exact inputs and program; editing invalidates export', async () => {
  const h = app(); h.ready(); h.run(); h.reply(); h.download();
  assert.equal(h.downloads.length, 1); assert.equal(h.downloads[0].attached, true);
  const observation = JSON.parse(await h.urls[0].blob.text());
  assert.equal(observation.kind, 'browser-observation'); assert.equal(observation.inputs[2].text, 'weather');
  assert.deepEqual(observation.proof.decision, { domain_status: 'VALID_INPUT', invalid_field: '', matched: false });
  assert.deepEqual(observation.program, metadata()); assert.equal(h.body.children.length, 0);
  h.type('req_name', 'new'); h.download(); assert.equal(h.downloads.length, 1);
  h.pagehide(); assert.deepEqual(h.revoked, [h.urls[0].url]);
});
test('loading and execution timeouts, stopped Worker and download failures are visible and retryable', () => {
  const loading = app(); [...loading.timers.values()].find(timer => timer.delay === 60000).handler(); assert.equal(loading.get('policy-retry').hidden, false);
  const running = app(); running.ready(); running.run(); [...running.timers.values()].find(timer => timer.delay === 15000).handler(); assert.equal(running.current().terminated, true); assert.equal(running.get('policy-download').disabled, true);
  for (const options of [{ workerError: true }, { postError: true }]) { const h = app(options); if (!options.workerError) { h.ready(); h.run(); } assert.equal(h.get('policy-error').hidden, false); }
  for (const options of [{ blobError: true }, { urlError: true }, { clickError: true }]) {
    const h = app(options); h.ready(); h.run(); h.reply(); h.download(); assert.match(h.get('policy-download-status').textContent, /could not start/); assert.equal(h.get('policy-download').disabled, false);
  }
  const h = app(); h.ready(); h.pagehide(); assert.equal(h.current().terminated, true); h.pageshow(); assert.equal(h.workers.length, 2); h.ready(); assert.equal(h.get('policy-run').disabled, false);
});

// File ingestion uses real Promise scheduling, UTF-8 decoding and the protocol
// parser. Worker decisions below remain explicit presentation fixtures, not
// evidence that a Gooo policy or its domain validator executed.
function savedObservation(mode = 'match', facts = { pattern: 'mcp://weather', req_type: 'mcp', req_name: 'weather' },
    decision = { domain_status: 'VALID_INPUT', invalid_field: '', matched: false }) {
  const program = metadata();
  return protocol.observation(mode, facts, { facts, decision, bindings: program.bindings }, program);
}
function fileBytes(bytes, options = {}) {
  const buffer = Buffer.from(bytes);
  return { name: options.name ?? 'observation.json', size: options.size ?? buffer.length,
    arrayBuffer: options.read ?? (() => Promise.resolve(buffer.buffer.slice(buffer.byteOffset, buffer.byteOffset + buffer.byteLength))) };
}
function fileObservation(observation = savedObservation(), options = {}) {
  return fileBytes(JSON.stringify(observation, null, 2) + '\n', options);
}
function deferredFile(observation = savedObservation(), options = {}) {
  let resolve, reject;
  const file = fileObservation(observation, { ...options, read: () => new Promise((yes, no) => { resolve = yes; reject = no; }) });
  return { file, complete() { const bytes = Buffer.from(JSON.stringify(observation, null, 2) + '\n'); resolve(bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength)); },
    fail() { reject(new Error('selected file read failed')); } };
}
function assertNoFresh(h) {
  assert.equal(h.get('policy-result').hidden, true);
  assert.equal(h.get('policy-comparison').hidden, true);
  assert.equal(h.get('policy-download').disabled, true);
}

test('saved file loads exact raw facts atomically without a check or a claimed pass', async () => {
  const h = app(); h.ready(); h.run(); h.reply();
  const value = '\r\n\t\0e\u0301😀é' + 'x'.repeat(129);
  const observation = savedObservation('match', { pattern: 'mcp://weather', req_type: 'mcp', req_name: value }, { ...invalid, invalid_field: 'req_name' });
  const file = fileObservation(observation, { name: '<script>claim</script>.json' });
  const before = h.current().messages.length;
  await h.select(file);
  assert.equal(h.current().messages.length, before, 'import never auto-runs');
  assertNoFresh(h);
  assert.equal(h.get('policy-run').textContent, 'Recheck with current Gooo');
  assert.equal(h.get('policy-saved-wrap').hidden, false);
  assert.match(h.get('policy-file-status').textContent, /<script>claim<\/script>\.json/);
  assert.equal(h.get('policy-file-status').children.length, 0, 'filename is text only');
  assert.match(h.get('policy-file-status').textContent, /unsigned, unverified/);
  for (const field of protocol.MODE_FIELDS.match) {
    assert.equal(h.get('policy-' + field).value, '');
    assert.match(h.get('policy-origin-' + field).textContent, /Saved file/);
  }
  assert.equal(h.get('policy-observation-file').value, '');
  const request = h.run();
  assert.equal(request.facts.req_name, value);
  h.reply({ ...invalid, invalid_field: 'req_name' });
  assert.match(h.get('policy-comparison-summary').textContent, /All 3 saved decision fields match/);
  assert.match(h.get('policy-program-comparison').textContent, /recorded identity remains unverified/);
  assert.equal(h.get('policy-download').disabled, false);
  h.download();
  const fresh = JSON.parse(await h.urls[0].blob.text());
  assert.equal(fresh.inputs[2].text, value);
  assert.equal(fresh.proof.facts.req_name, value);
  assert.equal(fresh.inputs[2].utf16_units.length, value.length);
  assert.deepEqual(fresh.program, metadata());
});

test('all three imported modes compare complete decisions only after fresh checked replies', async () => {
  const cases = [
    ['match', { pattern: 'mcp://weather', req_type: 'mcp', req_name: 'stocks' }, { domain_status: 'VALID_INPUT', invalid_field: '', matched: false }],
    ['impact', { before: 'mcp://*.x', after: 'mcp://a.*', req_type: 'mcp', req_name: 'a.x' }, { domain_status: 'VALID_INPUT', invalid_field: '', before_match: true, after_match: true, change: 'UNCHANGED_ALLOW' }],
    ['refine', { before: 'mcp://*.x', after: 'mcp://a.*' }, witness],
    ['refine', { before: 'mcp://bad*.', after: 'mcp://weather' }, { domain_status: 'INVALID_INPUT', invalid_field: 'before', status: 'INVALID_INPUT', rule_id: '', supported_subset: false, has_witness: false, witness_type: '', witness_name: '', before_match: false, after_match: false }]
  ];
  for (const [mode, facts, decision] of cases) {
    const h = app(); h.ready(); await h.select(fileObservation(savedObservation(mode, facts, decision)));
    assertNoFresh(h); assert.equal(h.current().messages.length, 0);
    assert.equal(h.get('policy-mode').value, mode);
    const request = h.run(); assert.deepEqual(request.facts, facts);
    h.reply(decision);
    assert.equal(h.get('policy-comparison').hidden, false);
    assert.equal(h.get('policy-decision-differences').children.length, 0);
    assert.match(h.get('policy-comparison-summary').textContent, new RegExp(`All ${protocol.DECISION_FIELDS[mode].length} saved decision fields match`));
    assert.match(h.get('policy-program-comparison').textContent, /Native bindings match/);
  }
});

test('coherent false saved claim displays every changed field and exports only current proof', async () => {
  const h = app(); h.ready();
  const facts = { before: 'mcp://weather', after: 'mcp://*', req_type: 'mcp', req_name: 'weather' };
  const claim = { domain_status: 'VALID_INPUT', invalid_field: '', before_match: false, after_match: false, change: 'UNCHANGED_DENY' };
  await h.select(fileObservation(savedObservation('impact', facts, claim)));
  h.run(); const current = { domain_status: 'VALID_INPUT', invalid_field: '', before_match: true, after_match: true, change: 'UNCHANGED_ALLOW' }; h.reply(current);
  assert.match(h.get('policy-comparison-summary').textContent, /3 of 5/);
  assert.deepEqual(h.get('policy-decision-differences').children.map(row => row.children[0].textContent), ['before_match', 'after_match', 'change']);
  assert.match(h.get('policy-decision-differences').textContent, /Saved: false · Current: true/);
  assert.match(h.get('policy-decision-differences').textContent, /Saved: "UNCHANGED_DENY" · Current: "UNCHANGED_ALLOW"/);
  assert.deepEqual(JSON.parse(h.get('policy-saved-decision').textContent), claim);
  h.download(); const fresh = JSON.parse(await h.urls[0].blob.text());
  assert.deepEqual(fresh.proof.decision, current);
  assert.deepEqual(fresh.program, metadata());
});

test('recorded program metadata, native bindings and executable axes remain distinct and unverified', async () => {
  const mutations = [
    [saved => { saved.program.source_revision = 'b'.repeat(40); }, /Native bindings match/, /Executable and runtime identities match/, 'source_revision'],
    [saved => { saved.program.compiler_revision = 'b'.repeat(40); saved.program.bindings.compiler_source_sha = 'b'.repeat(40); saved.proof.bindings.compiler_source_sha = 'b'.repeat(40); }, /Native bindings differ/, /Executable and runtime identities match/, 'compiler_revision'],
    [saved => { saved.program.wasm_sha256 = 'b'.repeat(64); saved.program.files['program.wasm'].sha256 = 'b'.repeat(64); }, /Native bindings match/, /Executable and runtime identities differ/, 'wasm_sha256']
  ];
  for (const [mutate, bindings, executable, changed] of mutations) {
    const saved = clone(savedObservation()); mutate(saved);
    const h = app(); h.ready(); await h.select(fileObservation(saved));
    assert.equal(h.get('policy-saved-wrap').hidden, false);
    h.run(); h.reply();
    assert.match(h.get('policy-comparison-summary').textContent, /All 3/);
    const text = h.get('policy-program-comparison').textContent;
    assert.match(text, /Recorded program metadata differs/); assert.match(text, bindings); assert.match(text, executable);
    assert.match(text, new RegExp(changed)); assert.match(text, /unverified/);
    const identities = JSON.parse(h.get('policy-comparison-programs').textContent);
    assert.deepEqual(identities.recorded_unverified, saved.program);
    assert.deepEqual(identities.current_verified, metadata());
  }
});

test('latest initiated file selection wins despite reordered resolution or old rejection', async () => {
  for (const finishOld of ['complete', 'fail']) {
    const h = app(); h.ready();
    const a = deferredFile(savedObservation(), { name: 'A.json' });
    const bFacts = { before: 'mcp://*.x', after: 'mcp://a.*' };
    const b = deferredFile(savedObservation('refine', bFacts, witness), { name: 'B.json' });
    const waitA = h.select(a.file), waitB = h.select(b.file);
    assert.equal(h.get('policy-run').disabled, true); assert.equal(h.get('policy-cancel').hidden, false);
    b.complete(); await waitB;
    assert.match(h.get('policy-file-status').textContent, /B\.json/);
    assert.equal(h.get('policy-mode').value, 'refine');
    a[finishOld](); await waitA;
    assert.match(h.get('policy-file-status').textContent, /B\.json/);
    assert.equal(h.get('policy-file-error').hidden, true);
    assert.equal(h.get('policy-mode').value, 'refine');
    assert.deepEqual(h.run().facts, bFacts);
    h.reply(witness); assert.equal(h.get('policy-comparison').hidden, false);
  }
});

test('edit, paste, drop, clear, mode, preset, cancel, retry and pagehide supersede pending file reads', async () => {
  const actions = [h => h.type('req_name', 'edited'), h => h.transfer('req_name', 'pasted'), h => h.transfer('req_name', 'dropped', 'drop'),
    h => h.get('policy-clear-pattern').emit('click'), h => h.mode('refine'), h => h.preset('denied'),
    h => h.get('policy-cancel').emit('click'), h => h.get('policy-retry').emit('click'), h => h.pagehide()];
  for (const action of actions) {
    const h = app(); h.ready(); h.transfer('pattern', 'mcp://*');
    const pending = deferredFile(); const wait = h.select(pending.file); action(h);
    const mode = h.get('policy-mode').value, preview = h.get('policy-input-preview').textContent;
    pending.complete(); await wait;
    assert.equal(h.get('policy-mode').value, mode); assert.equal(h.get('policy-input-preview').textContent, preview);
    assertNoFresh(h); assert.equal(h.get('policy-saved-wrap').hidden, true);
    assert.notEqual(h.get('policy-run').textContent, 'Recheck with current Gooo');
  }
});

test('file selection stops active check and late Worker replies cannot revive comparison or export', async () => {
  const h = app(); h.ready(); await h.select(fileObservation());
  const old = h.current(); h.run();
  const pending = deferredFile(); const wait = h.select(pending.file);
  assert.equal(old.terminated, true); h.reply(undefined, old);
  old.deliver({ type: 'error', message: 'stale load error' });
  old.deliver({ type: 'ready', program: metadata() });
  assertNoFresh(h); assert.equal(h.get('policy-file-error').hidden, true);
  pending.complete(); await wait; assertNoFresh(h);
  h.ready(); h.run(); h.reply(); assert.equal(h.get('policy-comparison').hidden, false);
});

test('file failures preserve inputs and clear all saved and fresh state; same file may be reselected', async () => {
  const raw = fileObservation();
  const failures = [fileBytes(''), fileBytes('{}'), fileBytes('[]'), fileBytes('{"schema":'),
    fileBytes(Buffer.from([0xC3, 0x28])), fileBytes(Buffer.concat([Buffer.from([0xEF, 0xBB, 0xBF]), Buffer.from(JSON.stringify(savedObservation()))])),
    fileBytes('{}', { size: protocol.MAX_OBSERVATION_BYTES + 1, read() { throw new Error('must not read oversized file'); } }),
    fileBytes('{}', { read() { return Promise.reject(new Error('read error')); } }),
    fileBytes('{}', { read() { return Promise.resolve(new ArrayBuffer(protocol.MAX_OBSERVATION_BYTES + 1)); } }),
    fileBytes('{}', { read() { return Promise.resolve('not an ArrayBuffer'); } })];
  for (const file of failures) {
    const h = app(); h.ready(); await h.select(raw); h.run(); h.reply();
    const preview = h.get('policy-input-preview').textContent, mode = h.get('policy-mode').value;
    await h.select(file);
    assert.equal(h.get('policy-input-preview').textContent, preview); assert.equal(h.get('policy-mode').value, mode);
    assertNoFresh(h); assert.equal(h.get('policy-saved-wrap').hidden, true);
    assert.equal(h.get('policy-file-error').hidden, false); assert.match(h.get('policy-file-status').textContent, /Prior input values are unchanged/);
    assert.equal(h.get('policy-run').textContent, 'Run policy check');
    await h.select(raw); assert.equal(h.get('policy-file-error').hidden, true); assert.equal(h.get('policy-saved-wrap').hidden, false);
  }
  for (const files of [[], [raw, raw]]) {
    const h = app(); h.ready(); await h.select(files); assertNoFresh(h); assert.equal(h.get('policy-file-error').hidden, false);
  }
});

test('file byte cap is enforced before reading and after completion; bounded timeout is retryable', async () => {
  const h = app(); h.ready(); let reads = 0;
  await h.select(fileBytes('{}', { size: protocol.MAX_OBSERVATION_BYTES + 1, read() { reads++; return Promise.resolve(new ArrayBuffer(2)); } }));
  assert.equal(reads, 0);
  const pending = deferredFile(); const wait = h.select(pending.file);
  const timer = [...h.timers.values()].find(value => value.delay === 15000); assert.ok(timer); timer.handler();
  assert.equal(h.get('policy-file-error').hidden, false); assert.match(h.get('policy-file-error').textContent, /timed out/);
  assertNoFresh(h); pending.complete(); await wait;
  assert.equal(h.get('policy-saved-wrap').hidden, true);
  await h.select(fileObservation()); assert.equal(h.get('policy-saved-wrap').hidden, false);
  h.run(); h.reply();
  const preview = h.get('policy-input-preview').textContent;
  const mismatch = deferredFile(savedObservation(), { size: 2 }); const badWait = h.select(mismatch.file);
  mismatch.complete(); await badWait;
  assert.match(h.get('policy-file-error').textContent, /differ from the selected file size/);
  assert.equal(h.get('policy-input-preview').textContent, preview); assertNoFresh(h);
  assert.equal(h.get('policy-saved-wrap').hidden, true);
});

test('editing invalidates imported baseline; worker cancel/error/retry keep only unchanged unverified claim', async () => {
  for (const action of [h => h.type('req_name', 'weather'), h => h.transfer('req_name', 'weather'), h => h.transfer('req_name', 'weather', 'drop'),
    h => h.mode('match'), h => h.preset('match'), h => h.get('policy-clear-pattern').emit('click')]) {
    const h = app(); h.ready(); await h.select(fileObservation()); h.run(); h.reply(); action(h);
    assertNoFresh(h); assert.equal(h.get('policy-saved-wrap').hidden, true); assert.equal(h.get('policy-run').textContent, 'Run policy check');
  }
  for (const action of [h => h.get('policy-cancel').emit('click'), h => h.current().deliver({ type: 'error', id: h.current().messages.at(-1).id, message: 'check failed' }),
    h => [...h.timers.values()].find(timer => timer.delay === 15000).handler()]) {
    const h = app(); h.ready(); await h.select(fileObservation()); const old = h.current(); h.run(); action(h);
    assertNoFresh(h); assert.equal(h.get('policy-saved-wrap').hidden, false); assert.equal(h.get('policy-run').textContent, 'Recheck with current Gooo');
    h.reply(undefined, old); assertNoFresh(h);
    if (h.current() === old) h.get('policy-retry').emit('click');
    h.ready(); h.run(); h.reply(); assert.equal(h.get('policy-comparison').hidden, false);
  }
});

test('pagehide clears saved claim and comparison and BFCache restore never silently rechecks', async () => {
  const h = app(); h.ready(); await h.select(fileObservation()); h.run(); h.reply();
  h.pagehide(); assertNoFresh(h); assert.equal(h.get('policy-saved-wrap').hidden, true);
  h.pageshow(); h.ready(); assertNoFresh(h); assert.equal(h.current().messages.length, 0);
  assert.equal(h.get('policy-run').textContent, 'Run policy check');
});

test('imported baseline never relaxes current proof, binding or ready-program validation', async () => {
  const saved = clone(savedObservation()); saved.program.source_revision = 'b'.repeat(40);
  for (const mutate of [message => { message.proof.facts.req_name = 'different'; },
    message => { message.proof.bindings.assets[protocol.ASSET_NAMES[0]] = 'sha256:' + 'f'.repeat(64); },
    message => { message.program = clone(saved.program); }]) {
    const h = app(); h.ready(); await h.select(fileObservation(saved)); const request = h.run();
    const message = { type: 'result', id: request.id, mode: request.mode, proof: { facts: clone(request.facts),
      decision: { domain_status: 'VALID_INPUT', invalid_field: '', matched: false }, bindings: clone(metadata().bindings) }, program: metadata() };
    mutate(message); h.current().deliver(message);
    assertNoFresh(h); assert.equal(h.current().terminated, true); assert.equal(h.get('policy-error').hidden, false);
    assert.equal(h.get('policy-saved-wrap').hidden, false, 'only unchanged unverified baseline remains for retry');
  }
  const h = app(); await h.select(fileObservation(saved));
  const changed = metadata(); changed.compiler_revision = 'b'.repeat(40); changed.bindings.compiler_source_sha = 'b'.repeat(40);
  h.ready(h.current(), changed); assertNoFresh(h); assert.equal(h.current().terminated, true);
  assert.equal(h.get('policy-run').disabled, true);
});

test('maximum four-field export imports without truncating 4096-unit captured values', async () => {
  const value = '\0'.repeat(4096), facts = { before: value, after: value, req_type: value, req_name: value };
  const decision = { domain_status: 'INVALID_INPUT', invalid_field: 'before', before_match: false, after_match: false, change: 'INVALID_INPUT' };
  const file = fileObservation(savedObservation('impact', facts, decision));
  assert.ok(file.size < protocol.MAX_OBSERVATION_BYTES);
  const h = app(); h.ready(); await h.select(file); assert.equal(h.get('policy-file-error').hidden, true);
  const request = h.run(); assert.deepEqual(request.facts, facts);
  for (const field of protocol.MODE_FIELDS.impact) assert.equal(h.get('policy-' + field).value, '');
  h.reply(decision); assert.equal(h.get('policy-comparison').hidden, false);
  assert.match(h.get('policy-comparison-summary').textContent, /All 5/);
});

test('file controls and comparison text have accessible labels and no imported HTML or action URLs', () => {
  assert.match(html, /<label for="policy-observation-file">/);
  assert.match(html, /id="policy-observation-file"[^>]*type="file"[^>]*aria-describedby="policy-file-help policy-file-status"/);
  assert.doesNotMatch(html.match(/<input id="policy-observation-file"[^>]*>/)[0], /\bmultiple\b/);
  assert.match(html, /id="policy-file-status"[^>]*role="status"/);
  assert.match(html, /id="policy-file-error"[^>]*role="alert"/);
  assert.match(html, /id="policy-comparison-summary"[^>]*role="status"/);
  assert.doesNotMatch(source, /\.innerHTML\b|localStorage|sessionStorage|\bfetch\s*\(/);
  assert.match(source, /new TextDecoder\('utf-8', \{ fatal: true, ignoreBOM: true \}\)/);
});
