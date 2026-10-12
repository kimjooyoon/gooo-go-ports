// Transport, scheduling and DOM tests. Stubbed compiled outputs here are not
// computational evidence; literal-search-smoke.cjs runs the actual Wasm oracle.
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { webcrypto, createHash } = require('node:crypto');
const protocol = require('../site/literal-search-protocol.js');
const workerAPI = require('../site/literal-search-worker.js');
const source = fs.readFileSync(path.join(__dirname, '../site/literal-search.js'), 'utf8');
const clone = value => JSON.parse(JSON.stringify(value));
function manifest() {
  return { schema: 'gooo/browser-literal-search/v1', entry_id: 'gooo://go-ports/examples/literal-text-report/report',
    result_entity_id: 'gooo://go-ports/examples/literal-text-report/result', input_entity_ids: ['gooo://std/text', 'gooo://std/text'],
    compiler_revision: '83928fa4538245abe33b08549da47f8cbdc50ed2', compiler_version: '0.6.32-dev', go_version: 'go1.27.2',
    source_revision: 'b'.repeat(40), source_dirty: false, wasm_bytes: 8, native_cases: 11, fixture_cases: 11, fixture_rejections: 8,
    native_receipt_expectation_cases: 0, wasm_sha256: 'a'.repeat(64), generated_sha256: 'c'.repeat(64),
    execution_sha256: 'd'.repeat(64), replay_sha256: 'e'.repeat(64), fixtures_sha256: 'f'.repeat(64),
    source_sha256: { 'ports/strings/literal.gooo': '1'.repeat(64) }, files_sha256: { 'program.wasm': 'a'.repeat(64),
      'generated.go': 'c'.repeat(64), 'execution.json': 'd'.repeat(64), 'replay.json': 'e'.repeat(64), 'fixtures.json': 'f'.repeat(64) } };
}
// These intentionally arbitrary bounded outputs test transport, not Index/Count.
function output(inputs) {
  return { text: inputs[0], needle: inputs[1], first_byte_offset: '-1', found: false, non_overlapping_count: '0',
    text_byte_length: String(protocol.bindString(inputs[0]).bytes), needle_byte_length: String(protocol.bindString(inputs[1]).bytes) };
}
const compiled = inputs => JSON.stringify({ ok: true, inputs, output: output(inputs) });

test('UTF-8 binding preserves BOM, CRLF, NUL, combining text and paired surrogates, replacing only unpaired ones', () => {
  const original = '\uFEFF\r\n\0e\u0301👩‍💻\uD800A\uDFFF';
  const snapshot = protocol.read(original, 'text');
  assert.equal(snapshot.original, original);
  assert.equal(snapshot.bound, '\uFEFF\r\n\0e\u0301👩‍💻\uFFFDA\uFFFD');
  assert.equal(snapshot.changed, true);
  assert.equal(snapshot.utf8Bytes, new TextEncoder().encode(original).length);
  assert.deepEqual(snapshot.originalUtf16Units, Array.from({ length: original.length }, (_, index) => original.charCodeAt(index)));
  assert.equal(JSON.parse(snapshot.originalEscaped), original);
  assert.equal(JSON.parse(snapshot.boundEscaped), snapshot.bound);
  assert.equal(protocol.read('e\u0301', 'text').bound, 'e\u0301');
  assert.equal(protocol.read('é', 'text').bound, 'é');
  assert.notEqual(protocol.read('e\u0301', 'text').bound, protocol.read('é', 'text').bound);
  assert.equal(protocol.read(' \t\n ', 'text').bound, ' \t\n ');
  assert.equal(Object.isFrozen(snapshot), true);
  assert.equal(Object.isFrozen(snapshot.originalUtf16Units), true);
});

test('limits apply to both whole decoded inputs and reject overflow without truncation', () => {
  for (const text of ['x'.repeat(1024), 'é'.repeat(512), '😀'.repeat(256), '\uD800'.repeat(341) + 'a']) {
    assert.equal(protocol.read(text, 'text').utf8Bytes, 1024);
  }
  for (const text of ['x'.repeat(1025), 'é'.repeat(513), '😀'.repeat(257), '\uD800'.repeat(342)]) assert.throws(() => protocol.read(text, 'text'), /1,024 UTF-8 bytes/);
  for (const inputs of [null, 'x', [], ['x'], ['x', 'y', 'z'], [true, 'x'], ['x', null], new Array(2)]) assert.throws(() => protocol.validateInputs(inputs));
  assert.deepEqual(protocol.validateInputs(['', '']), ['', '']);
  assert.throws(() => protocol.validateInputs(['\uD800', '']), /already/);
  assert.deepEqual(protocol.bindInputs(['\uD800', '\uFEFF']), ['\uFFFD', '\uFEFF']);
  assert.throws(() => protocol.bindInputs(['a', '😀'.repeat(257)]));
});

test('JSON string mode retains captured notation separately from exact decoded UTF-16', () => {
  const source = '  "\\uFEFF\\r\\n\\uD800\\uD83D\\uDE00"\t';
  const value = protocol.read(source, 'json');
  assert.equal(value.capturedSource, source);
  assert.equal(value.original, '\uFEFF\r\n\uD800😀');
  assert.equal(value.bound, '\uFEFF\r\n\uFFFD😀');
  assert.equal(value.changed, true);
  for (const source of ['null', '[]', '{}', '1', 'true', '"unfinished', '"a" "b"', '"\\uGGGG"', '\uFEFF"x"']) assert.throws(() => protocol.read(source, 'json'));
  assert.throws(() => protocol.read(' '.repeat(8193), 'json'), /8,192/);
  assert.throws(() => protocol.read('"' + '\\u0061'.repeat(1025) + '"', 'json'), /1,024/);
  assert.throws(() => protocol.read('x', 'other'));
});

test('requests, cancellation and error messages have exact bounded shapes', () => {
  const original = ['x', 'y'];
  const accepted = protocol.validateRequest({ type: 'search', id: 7, inputs: original }); original[0] = 'changed';
  assert.deepEqual(accepted.inputs, ['x', 'y']);
  for (const request of [null, {}, { type: 'search', id: 0, inputs: ['', ''] }, { type: 'search', id: 1.2, inputs: ['', ''] },
    { type: 'search', id: Number.MAX_SAFE_INTEGER + 1, inputs: ['', ''] }, { type: 'other', id: 1, inputs: ['', ''] },
    { type: 'search', id: 1, inputs: ['', ''], extra: true }]) assert.throws(() => protocol.validateRequest(request));
  assert.equal(protocol.validateCancel({ type: 'cancel', id: 1 }), 1);
  assert.throws(() => protocol.validateCancel({ type: 'cancel', id: 1, extra: true }));
  assert.equal(protocol.validateError({ type: 'error', message: 'stopped' }), 'stopped');
  assert.equal(protocol.validateError({ type: 'error', id: 3, message: 'stopped' }, 3), 'stopped');
  for (const message of [{ type: 'error', id: 2, message: 'old' }, { type: 'error', id: 3, message: '' },
    { type: 'error', id: 3, message: 'x'.repeat(241) }, { type: 'error', id: 3, message: 'x', extra: true }]) assert.throws(() => protocol.validateError(message, 3));
});

test('bridge reply requires all seven exact fields, bound inputs and canonical decimal/Boolean types', () => {
  const inputs = ['가x가', 'x'], valid = { ok: true, inputs, output: { text: inputs[0], needle: inputs[1], first_byte_offset: '3',
    found: true, non_overlapping_count: '1', text_byte_length: '7', needle_byte_length: '1' } };
  assert.deepEqual(protocol.parseCompiledReply(JSON.stringify(valid), inputs), valid.output);
  const mutations = [r => { r.inputs.reverse(); }, r => { r.inputs.push(''); }, r => { r.inputs[0] = 'same bytes'; },
    r => { r.output.text = 'other'; }, r => { r.output.needle = 1; }, r => { r.ok = 1; }, r => { r.extra = true; },
    r => { r.output.found = 'true'; }, r => { r.output.found = false; }, r => { r.output.text_byte_length = '6'; },
    r => { r.output.needle_byte_length = '01'; }, r => { r.output.extra = true; }];
  for (const field of protocol.fields) mutations.push(r => { delete r.output[field]; });
  for (const value of ['-0', '+1', '01', ' 1', '1 ', '1e0', '1.0', '1026', '-2', 1, null]) {
    mutations.push(r => { r.output.non_overlapping_count = value; });
  }
  for (const value of ['8', '-2', '1e0', 3]) mutations.push(r => { r.output.first_byte_offset = value; });
  for (const change of mutations) { const value = clone(valid); change(value); assert.throws(() => protocol.parseCompiledReply(JSON.stringify(value), inputs)); }
  for (const raw of ['{', 'null', '[]', ' '.repeat(32769), JSON.stringify({ ok: false, error: '' }),
    JSON.stringify({ ok: false, error: 'x'.repeat(241) }), JSON.stringify({ ok: false, error: 'rejected', output: {} })]) assert.throws(() => protocol.parseCompiledReply(raw, inputs));
  assert.throws(() => protocol.parseCompiledReply(JSON.stringify({ ok: false, error: 'rejected' }), inputs), /compiled program rejected/);
  const bound = ['\uFFFD', ''];
  assert.deepEqual(protocol.parseBridgeReply(compiled(bound), ['\uD800', '']), { ok: true, inputs: bound, output: output(bound) });
});

test('all metadata pins, hashes, counters and input/entry/result identities are mandatory', () => {
  const valid = manifest();
  assert.deepEqual(clone(protocol.validateMetadata(valid)), valid);
  const mutations = [m => { m.schema = 'v2'; }, m => { m.entry_id = 'other'; }, m => { m.result_entity_id = 'other'; },
    m => { m.input_entity_ids[0] = 'urn:gooo:type:string'; }, m => { m.input_entity_ids.push('gooo://std/text'); },
    m => { m.compiler_revision = 'c'.repeat(40); }, m => { m.compiler_version = 'v0.6.32-dev'; }, m => { m.go_version = 'go1.27.1'; },
    m => { m.source_revision = 'x'; }, m => { m.source_dirty = 'false'; }, m => { m.wasm_bytes = 0; },
    m => { m.wasm_bytes = 32 * 1024 * 1024 + 1; }, m => { m.native_receipt_expectation_cases = 1; },
    m => { m.files_sha256 = {}; }, m => { m.source_sha256 = { '../source.gooo': 'a'.repeat(64) }; },
    m => { m.native_cases = 12; }, m => { m.files_sha256['generated.go'] = '2'.repeat(64); },
    m => { delete m.files_sha256['replay.json']; }];
  for (const field of ['native_cases', 'fixture_cases', 'fixture_rejections']) mutations.push(m => { m[field] = 0; });
  for (const field of ['wasm_sha256', 'generated_sha256', 'execution_sha256', 'replay_sha256', 'fixtures_sha256']) mutations.push(m => { m[field] = 'z'.repeat(64); });
  for (const change of mutations) { const value = clone(valid); change(value); assert.throws(() => protocol.validateMetadata(value)); }
  const frozen = protocol.validateMetadata(valid); valid.files_sha256['program.wasm'] = 'f'.repeat(64);
  assert.equal(frozen.files_sha256['program.wasm'], 'a'.repeat(64));
  assert.equal(Object.isFrozen(frozen.files_sha256), true);
});

test('result binds the exact entire program manifest, request and all output fields', () => {
  const inputs = ['a', 'b'], program = manifest(), expected = { id: 3, inputs, program };
  const result = { type: 'result', id: 3, inputs, output: output(inputs), program };
  assert.deepEqual(protocol.validateResult(result, expected), result.output);
  const changes = [r => { r.id = 2; }, r => { r.inputs.reverse(); }, r => { r.program.source_dirty = true; },
    r => { r.program.generated_sha256 = '3'.repeat(64); }, r => { r.program.files_sha256['program.wasm'] = '2'.repeat(64); },
    r => { r.output.text = 'z'; }, r => { r.extra = true; }];
  for (const change of changes) { const value = clone(result); change(value); assert.throws(() => protocol.validateResult(value, expected)); }
  assert.throws(() => protocol.validateReady({ type: 'ready', program, extra: true }));
  // Object key ordering alone is not a different program identity.
  assert.doesNotThrow(() => protocol.validateResult({ ...result, program: Object.fromEntries(Object.entries(program).reverse()) }, expected));
});

test('observation preserves captured JSON, original/bound UTF-16 and honest scope', () => {
  const snapshots = [protocol.read('"\\uFEFF\\uD800\\r\\n"', 'json'), protocol.read('\uFFFD', 'text')];
  const inputs = snapshots.map(value => value.bound);
  const report = protocol.observation(snapshots, output(inputs), manifest(), ['paste', 'typed']);
  assert.equal(report.kind, 'browser-observation');
  assert.deepEqual(report.inputs[0].original_utf16_units, [65279, 55296, 13, 10]);
  assert.deepEqual(report.inputs[0].bound_utf16_units, [65279, 65533, 13, 10]);
  assert.equal(report.inputs[0].captured_source, '"\\uFEFF\\uD800\\r\\n"');
  assert.equal(JSON.parse(report.inputs[0].original_escaped), '\uFEFF\uD800\r\n');
  assert.equal(report.inputs[0].replaced_unpaired_surrogates, true);
  assert.equal(report.inputs[1].replaced_unpaired_surrogates, false);
  assert.deepEqual(clone(report.program), manifest());
  assert.match(report.scope, /not a native execution receipt.*signed attestation/);
  assert.equal(Object.hasOwn(report, 'receipt'), false);
});

function workerHarness(run = (...inputs) => compiled(inputs), loadFailure = null) {
  const messages = []; let listener, fatal;
  const scope = { postMessage(message) { messages.push(clone(message)); }, addEventListener(type, callback) { assert.equal(type, 'message'); listener = callback; } };
  const controller = workerAPI.startWorker(scope, async (_scope, onFatal) => {
    fatal = onFatal; if (loadFailure) throw new Error(loadFailure); return { program: manifest(), run };
  });
  return { messages, ready: controller.ready, send: value => listener({ data: value }), fatal: message => fatal(message) };
}

test('Worker rejects malformed/early/duplicate requests and performs one compiled call with exact input binding', async () => {
  const calls = [], h = workerHarness((...inputs) => { calls.push(inputs); return compiled(inputs); });
  await h.send({ type: 'search', id: 1, inputs: ['a', 'b'] }); assert.equal(calls.length, 0);
  await h.ready;
  for (const inputs of [['a', '\uD800'], ['x'.repeat(1025), ''], ['a'], [null, '']]) await h.send({ type: 'search', id: 2, inputs });
  assert.equal(calls.length, 0);
  await h.send({ type: 'search', id: 2, inputs: ['\uFEFF\r\n', ''] });
  assert.deepEqual(calls, [['\uFEFF\r\n', '']]);
  assert.equal(h.messages.at(-1).type, 'result');
  await h.send({ type: 'search', id: 2, inputs: ['a', 'b'] });
  assert.equal(h.messages.at(-1).type, 'error'); assert.equal(calls.length, 1);
});

test('Worker suppresses cancelled, superseded and fatal results without any partial result', async () => {
  const calls = [], h = workerHarness((...inputs) => { calls.push(inputs); return compiled(inputs); }); await h.ready;
  const old = h.send({ type: 'search', id: 1, inputs: ['old', ''] });
  const current = h.send({ type: 'search', id: 2, inputs: ['new', ''] });
  await Promise.all([old, current]);
  assert.deepEqual(calls, [['new', '']]);
  assert.deepEqual(h.messages.filter(value => value.type === 'result').map(value => value.id), [2]);
  const cancelled = h.send({ type: 'search', id: 3, inputs: ['cancelled', ''] });
  await h.send({ type: 'cancel', id: 3 }); await cancelled;
  assert.equal(calls.length, 1);
  const fatalPending = h.send({ type: 'search', id: 4, inputs: ['fatal', ''] }); h.fatal('runtime stopped'); await fatalPending;
  assert.equal(calls.length, 1);
  assert.equal(h.messages.at(-1).type, 'error'); assert.equal(h.messages.at(-1).id, 4);
  await h.send({ type: 'search', id: 5, inputs: ['blocked', ''] }); assert.equal(calls.length, 1);
  const broken = workerHarness(() => '{}'); await broken.ready; await broken.send({ type: 'search', id: 1, inputs: ['', ''] });
  assert.equal(broken.messages.filter(value => value.type === 'result').length, 0); assert.equal(broken.messages.at(-1).type, 'error');
  const failed = workerHarness(undefined, 'hash mismatch'); await failed.ready;
  assert.deepEqual(failed.messages, [{ type: 'error', message: 'hash mismatch' }]);
});

function runtimeScope() {
  const bytes = new Uint8Array([0, 97, 115, 109, 1, 0, 0, 0]).buffer;
  const metadata = manifest(); metadata.wasm_sha256 = createHash('sha256').update(Buffer.from(bytes)).digest('hex');
  metadata.files_sha256['program.wasm'] = metadata.wasm_sha256;
  const requests = [], imported = [], fatals = []; let instantiated = 0, executions = 0;
  const scope = { crypto: webcrypto, fetch: async (url, options) => {
    requests.push({ url, options }); return { ok: true, json: async () => metadata, arrayBuffer: async () => bytes };
  }, importScripts: name => imported.push(name), WebAssembly: { validate: WebAssembly.validate, instantiate: async () => {
    instantiated++; return { instance: {} };
  } } };
  scope.Go = class { constructor() { this.importObject = {}; } run() { executions++; scope.goooLiteralSearchReady = true;
    scope.goooLiteralSearch = (...inputs) => compiled(inputs); return new Promise(() => {});
  } };
  return { scope, bytes, metadata, requests, imported, fatals, instances: () => instantiated, executions: () => executions };
}

test('loader verifies byte size, SHA, Wasm validity and exact pins before importing or executing runtime', async () => {
  for (const change of [h => { h.metadata.wasm_bytes = 9; }, h => { h.metadata.wasm_sha256 = 'a'.repeat(64); },
    h => { h.metadata.compiler_revision = 'b'.repeat(40); }, h => { h.scope.WebAssembly.validate = () => false; },
    h => { h.scope.fetch = async () => ({ ok: false }); }]) {
    const h = runtimeScope(); change(h); await assert.rejects(workerAPI.loadBrowserRuntime(h.scope, () => {}));
    assert.deepEqual(h.imported, []); assert.equal(h.executions(), 0); assert.equal(h.instances(), 0);
  }
  const h = runtimeScope(), runtime = await workerAPI.loadBrowserRuntime(h.scope, message => h.fatals.push(message));
  assert.deepEqual(h.requests.map(value => value.url), ['./literal-search/manifest.json', './literal-search/program.wasm']);
  assert.ok(h.requests.every(value => value.options.cache === 'no-store'));
  assert.deepEqual(h.imported, ['./wasm_exec.js']); assert.equal(h.instances(), 1); assert.equal(h.executions(), 1);
  assert.equal(runtime.program.wasm_sha256, h.metadata.wasm_sha256);
  assert.deepEqual(protocol.parseBridgeReply(runtime.run('a', 'b'), ['a', 'b']).output, output(['a', 'b']));
});

test('loader rejects missing compiled API, instantiate failures and sends runtime-stop notifications', async () => {
  const absent = runtimeScope(); absent.scope.Go = class { constructor() { this.importObject = {}; } run() { return new Promise(() => {}); } };
  await assert.rejects(workerAPI.loadBrowserRuntime(absent.scope, () => {}), /API/);
  const bad = runtimeScope(); bad.scope.WebAssembly.instantiate = async () => { throw new Error('bad imports'); };
  await assert.rejects(workerAPI.loadBrowserRuntime(bad.scope, () => {}), /bad imports/);
  for (const reject of [false, true]) {
    const h = runtimeScope(); h.scope.Go = class { constructor() { this.importObject = {}; } run() {
      h.scope.goooLiteralSearchReady = true; h.scope.goooLiteralSearch = () => '{}';
      return reject ? Promise.reject(new Error('stopped')) : Promise.resolve();
    } };
    await workerAPI.loadBrowserRuntime(h.scope, message => h.fatals.push(message)); await Promise.resolve();
    assert.equal(h.fatals.length, 1); assert.match(h.fatals[0], /stopped/);
  }
});

class Element {
  constructor() { this._text = ''; this.children = []; this.value = ''; this.hidden = false; this.disabled = false; this.attributes = {}; this.dataset = {}; this.listeners = {}; }
  get textContent() { return this._text + this.children.map(value => value.textContent).join(''); }
  set textContent(value) { this._text = String(value); this.children = []; }
  replaceChildren(...values) { this._text = ''; this.children = values; }
  append(...values) { for (const value of values) value.parent = this; this.children.push(...values); }
  remove() { if (this.parent) this.parent.children = this.parent.children.filter(value => value !== this); this.parent = null; }
  setAttribute(name, value) { this.attributes[name] = value; }
  addEventListener(name, handler) { this.listeners[name] = handler; }
  emit(name, event = {}) { return this.listeners[name]?.({ preventDefault() {}, ...event }); }
}
function app(options = {}) {
  const elements = new Map(), workers = [], timers = new Map(), events = new Map(), body = new Element();
  const urls = [], revoked = [], downloads = []; let timerId = 0;
  const get = id => { if (!elements.has(id)) elements.set(id, new Element()); return elements.get(id); };
  get('literal-text-mode').value = 'text'; get('literal-needle-mode').value = 'text';
  const presets = ['bytes', 'overlap', 'empty', 'surrogates'].map(name => { const value = new Element(); value.dataset.literalPreset = name; return value; });
  class Worker {
    constructor(url) { if (options.workerError) throw new Error('no workers'); assert.equal(url, './literal-search-worker.js'); this.messages = []; this.terminated = false; workers.push(this); }
    terminate() { this.terminated = true; }
    postMessage(message) { if (options.postError) throw new Error('post failed'); this.messages.push(clone(message)); }
    deliver(data) { this.onmessage({ data }); }
  }
  const context = { Worker, GoooLiteralSearchProtocol: protocol,
    Blob: class extends Blob { constructor(...args) { if (options.blobError) throw new Error('no blob'); super(...args); } },
    URL: { createObjectURL(blob) { if (options.urlError) throw new Error('no URL'); const url = 'blob:literal-' + (urls.length + 1); urls.push({ url, blob }); return url; },
      revokeObjectURL(url) { revoked.push(url); } },
    setTimeout(handler, delay) { const id = ++timerId; timers.set(id, { handler, delay }); return id; }, clearTimeout(id) { timers.delete(id); },
    addEventListener(name, handler) { events.set(name, handler); }, document: { body, getElementById: get,
      createElement(tag) { const value = new Element(); if (tag === 'a') value.click = () => { if (options.clickError) throw new Error('download refused'); downloads.push({ href: value.href, name: value.download, attached: body.children.includes(value) }); }; return value; },
      querySelectorAll(selector) { assert.equal(selector, '[data-literal-preset]'); return presets; } } };
  context.window = context; vm.runInNewContext(source, context, { filename: 'site/literal-search.js' });
  const current = () => workers.at(-1);
  const ready = (instance = current(), program = manifest()) => instance.deliver({ type: 'ready', program });
  const type = (name, value) => { get('literal-' + name).value = value; get('literal-' + name).emit('input'); };
  const mode = (name, value) => { get('literal-' + name + '-mode').value = value; get('literal-' + name + '-mode').emit('change'); };
  const transfer = (name, value, kind = 'paste', extra = {}) => {
    let prevented = false;
    get('literal-' + name).emit(kind, { preventDefault() { prevented = true; }, [kind === 'paste' ? 'clipboardData' : 'dataTransfer']:
      { types: ['text/plain'], getData(format) { assert.equal(format, 'text/plain'); return value; }, ...extra } });
    assert.equal(prevented, true);
  };
  const run = () => { get('literal-form').emit('submit'); return current()?.messages.at(-1); };
  const reply = (instance = current(), changed = {}) => { const request = instance.messages.at(-1);
    instance.deliver({ type: 'result', id: request.id, inputs: request.inputs, output: output(request.inputs), program: manifest(), ...changed }); };
  return { get, workers, current, ready, type, mode, transfer, run, reply, timers, body, urls, revoked, downloads,
    download: () => get('literal-download').emit('click'), pagehide: () => events.get('pagehide')(), pageshow: () => events.get('pageshow')({ persisted: true }),
    preset: name => presets.find(value => value.dataset.literalPreset === name).emit('click') };
}

test('UI captures both pasted/dropped sources before editor CRLF normalization and sends only bound strings', () => {
  for (const kind of ['paste', 'drop']) {
    const h = app(); h.ready(); h.transfer('text', '\uFEFF\uD800\r\n<unsafe>', kind); h.transfer('needle', '\uDFFF', kind);
    assert.equal(h.get('literal-text').value, '');
    assert.equal(JSON.parse(h.get('literal-text-preview').textContent), '\uFEFF\uD800\r\n<unsafe>');
    assert.equal(JSON.parse(h.get('literal-text-bound').textContent), '\uFEFF\uFFFD\r\n<unsafe>');
    assert.equal(h.get('literal-text-bound-wrap').hidden, false);
    assert.match(h.get('literal-text-source').textContent, /before insertion.*CRLF/);
    assert.deepEqual(h.run().inputs, ['\uFEFF\uFFFD\r\n<unsafe>', '\uFFFD']);
    h.reply(); assert.equal(h.get('literal-output').children.length, 7);
    assert.equal(h.get('literal-output').children[0].children[1].children.length, 0, 'input is rendered with textContent');
  }
});

test('UI decodes JSON independently, clears stale results on edits and never runs partial/oversized inputs', () => {
  const h = app(); h.ready(); h.mode('text', 'json'); h.type('text', '"\\r\\n\\uD800"'); h.type('needle', '\uFFFD');
  assert.deepEqual(h.run().inputs, ['\r\n\uFFFD', '\uFFFD']); h.reply();
  assert.equal(h.get('literal-download').disabled, false);
  for (const value of ['"unfinished', 'null', '"' + 'x'.repeat(1025) + '"']) {
    h.type('text', value); assert.equal(h.get('literal-run').disabled, true); assert.equal(h.get('literal-download').disabled, true);
    assert.equal(h.get('literal-output').children.length, 0); h.run(); assert.equal(h.current().messages.length, 1);
  }
  h.type('text', '"ok"'); assert.equal(h.get('literal-run').disabled, false);
  h.transfer('needle', 'x'.repeat(1025)); assert.equal(h.get('literal-run').disabled, true);
  h.transfer('needle', '', 'paste', { files: [{}] }); assert.equal(h.get('literal-run').disabled, true);
  assert.match(h.get('literal-needle-status').textContent, /Files/);
  h.transfer('needle', '', 'drop', { getData() { throw new Error('denied'); } }); assert.equal(h.get('literal-run').disabled, true);
});

test('UI displays the returned seven fields without computing its own search result', () => {
  const h = app(); h.ready(); h.type('text', 'abc'); h.type('needle', 'b'); const request = h.run();
  const arbitrary = { ...output(request.inputs), first_byte_offset: '2', found: true, non_overlapping_count: '7' };
  h.reply(h.current(), { output: arbitrary });
  assert.equal(h.get('literal-output').children[2].children[1].textContent, '2');
  assert.equal(h.get('literal-output').children[3].children[1].textContent, 'Yes');
  assert.equal(h.get('literal-output').children[4].children[1].textContent, '7');
  assert.match(h.get('literal-progress').textContent, /one compiled Gooo Report call/);
});

test('UI supersession/cancellation terminates a busy worker and ignores old callbacks/results', () => {
  for (const change of [h => h.type('text', 'current'), h => h.transfer('needle', 'current'), h => h.get('literal-cancel').emit('click'), h => h.preset('bytes')]) {
    const h = app(); h.ready(); h.type('text', 'old'); h.run(); const old = h.current();
    change(h); assert.equal(old.terminated, true); assert.notEqual(h.current(), old);
    h.reply(old); old.onerror({ preventDefault() {} }); old.onmessageerror();
    assert.equal(h.get('literal-output-wrap').hidden, true); assert.equal(h.get('literal-download').disabled, true);
    assert.equal(h.get('literal-retry').hidden, true);
    h.ready(); h.run(); h.reply(); assert.equal(h.get('literal-download').disabled, false);
    h.reply(old); assert.equal(h.get('literal-download').disabled, false);
  }
});

test('UI rejects corrupted result identities and fatal/unknown messages without retaining an export', () => {
  for (const send of [h => h.reply(h.current(), { program: { ...manifest(), source_dirty: true } }),
    h => h.reply(h.current(), { inputs: ['wrong', ''] }), h => h.reply(h.current(), { output: {} }),
    h => h.current().deliver({ type: 'unknown' }), h => h.current().deliver({ type: 'error', message: 'fatal' }),
    h => h.current().onmessageerror(), h => h.current().onerror({ preventDefault() {} })]) {
    const h = app(); h.ready(); h.run(); send(h);
    assert.equal(h.current().terminated, true); assert.equal(h.get('literal-run').disabled, true);
    assert.equal(h.get('literal-download').disabled, true); assert.equal(h.get('literal-output').children.length, 0);
    assert.equal(h.get('literal-retry').hidden, false); assert.equal(h.timers.size, 0);
  }
  const h = app(); h.ready(); h.run(); const id = h.current().messages.at(-1).id;
  h.current().deliver({ type: 'error', id: id + 1, message: 'stale' }); assert.equal(h.current().terminated, false);
  h.reply(); h.current().deliver({ type: 'error', message: 'stopped after completion' });
  assert.equal(h.get('literal-download').disabled, true);
});

test('load and actual run deadlines terminate their own worker and late timer callbacks cannot affect a replacement', () => {
  const h = app(), loading = [...h.timers.values()].find(value => value.delay === 30000); assert.ok(loading);
  loading.handler(); assert.equal(h.current().terminated, true); assert.match(h.get('literal-runtime').textContent, /30 seconds/);
  h.get('literal-retry').emit('click'); h.ready(); loading.handler(); assert.equal(h.current().terminated, false);
  h.run(); const running = [...h.timers.values()].find(value => value.delay === 10000); assert.ok(running);
  running.handler(); assert.equal(h.current().terminated, true); assert.match(h.get('literal-runtime').textContent, /10 seconds/);
  assert.equal(h.timers.size, 0); assert.equal(h.get('literal-download').disabled, true);
  h.get('literal-retry').emit('click'); h.ready(); h.run(); h.reply(); running.handler();
  assert.equal(h.current().terminated, false); assert.equal(h.get('literal-download').disabled, false);
  assert.equal(h.timers.size, 0);
});

test('UI handles Worker construction, posting and duplicate-ready failures visibly', () => {
  const absent = app({ workerError: true }); assert.equal(absent.get('literal-run').disabled, true); assert.equal(absent.get('literal-retry').hidden, false);
  const posting = app({ postError: true }); posting.ready(); posting.run(); assert.equal(posting.current().terminated, true); assert.equal(posting.timers.size, 0);
  const duplicated = app(); duplicated.ready(); duplicated.ready(); assert.equal(duplicated.current().terminated, true); assert.equal(duplicated.get('literal-download').disabled, true);
});

test('a restored page reloads its terminated Worker and preserves exact captured inputs without old results', () => {
  const h = app(); h.ready(); h.transfer('text', '\r\n\uD800'); h.type('needle', 'a'); h.run(); h.reply();
  const old = h.current(); h.pagehide();
  assert.equal(old.terminated, true); assert.equal(h.get('literal-run').disabled, true); assert.equal(h.get('literal-download').disabled, true);
  h.pageshow(); assert.notEqual(h.current(), old); h.ready();
  assert.deepEqual(h.run().inputs, ['\r\n\uFFFD', 'a']);
  h.reply(old); assert.equal(h.get('literal-download').disabled, true);
  h.reply(); assert.equal(h.get('literal-download').disabled, false);
});

test('download exports exact captured/bound units, output and full program; object URLs are cleaned', async () => {
  const h = app(); h.ready(); h.preset('surrogates'); h.run(); h.reply(); h.download();
  assert.equal(h.urls.length, 1); const serialized = await h.urls[0].blob.text(), report = JSON.parse(serialized);
  assert.equal(serialized.endsWith('\n'), true); assert.equal(report.kind, 'browser-observation');
  assert.equal(report.inputs[0].captured_source, '"\\uFEFF\\uD800A\\r\\n"');
  assert.deepEqual(report.inputs[0].original_utf16_units, [65279, 55296, 65, 13, 10]);
  assert.deepEqual(report.inputs[0].bound_utf16_units, [65279, 65533, 65, 13, 10]);
  assert.equal(report.inputs[0].replaced_unpaired_surrogates, true); assert.deepEqual(report.program, manifest());
  assert.equal(report.output.text, '\uFEFF\uFFFDA\r\n'); assert.equal(h.urls[0].blob.type, 'application/json;charset=utf-8');
  assert.deepEqual(h.downloads, [{ href: 'blob:literal-1', name: 'gooo-literal-search-observation.json', attached: true }]);
  assert.equal(h.body.children.length, 0); assert.deepEqual(h.revoked, []);
  assert.match(h.get('literal-download-status').textContent, /Download requested/);
  assert.doesNotMatch(h.get('literal-download-status').textContent, /saved|successfully/i);
  const cleanup = [...h.timers.values()].find(value => value.delay === 1000); assert.ok(cleanup); cleanup.handler();
  assert.deepEqual(h.revoked, ['blob:literal-1']); assert.equal(h.timers.size, 0);
  h.pagehide(); cleanup.handler(); assert.equal(h.revoked.length, 1);
});

test('failed downloads release resources and preserve current results for retry; edits remove access to old reports', () => {
  for (const phase of ['blobError', 'urlError', 'clickError']) {
    const options = { [phase]: true }, h = app(options); h.ready(); h.run(); h.reply(); h.download();
    assert.match(h.get('literal-download-status').textContent, /could not be started/);
    assert.equal(h.get('literal-download').disabled, false); assert.equal(h.get('literal-output').children.length, 7);
    assert.equal(h.body.children.length, 0); assert.deepEqual(h.revoked, h.urls.map(value => value.url)); assert.equal(h.timers.size, 0);
    options[phase] = false; h.download(); assert.match(h.get('literal-download-status').textContent, /Download requested/);
    h.type('needle', 'changed'); assert.equal(h.get('literal-download').disabled, true); const count = h.downloads.length; h.download(); assert.equal(h.downloads.length, count);
    h.pagehide(); assert.deepEqual(h.revoked, h.urls.map(value => value.url)); assert.equal(h.current().terminated, true);
  }
});

test('HTML has unique referenced controls, accessible names and explicit string/offset/observation rules', () => {
  const html = fs.readFileSync(path.join(__dirname, '../site/literal-search.html'), 'utf8');
  const ids = [...html.matchAll(/\bid="([^"]+)"/g)].map(value => value[1]);
  assert.equal(new Set(ids).size, ids.length);
  for (const id of [...source.matchAll(/\$\('([^']+)'\)/g)].map(value => value[1])) assert.ok(ids.includes(id), id);
  for (const name of ['text', 'needle']) {
    for (const suffix of ['', '-mode', '-label', '-source', '-status', '-preview', '-bound', '-bound-wrap']) assert.ok(ids.includes('literal-' + name + suffix));
    assert.match(html, new RegExp('<label[^>]*for="literal-' + name + '"'));
  }
  assert.match(html, /byte offset is not a JavaScript character index/);
  assert.match(html, /Unpaired UTF-16 surrogates become U\+FFFD/);
  assert.match(html, /not a native execution receipt or signed attestation/);
  assert.match(html, /<script defer src="\.\/literal-search-protocol\.js"><\/script>/);
  assert.match(html, /<script defer src="\.\/literal-search\.js"><\/script>/);
});
