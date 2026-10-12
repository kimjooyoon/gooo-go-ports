// Transport, scheduling and simulated DOM capture checks. Stubbed return
// records here are not parser evidence; uuid-parse-smoke.cjs runs actual Wasm.
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { webcrypto, createHash } = require('node:crypto');
const protocol = require('../site/uuid-parse-protocol.js');
const workerAPI = require('../site/uuid-parse-worker.js');
const source = fs.readFileSync(path.join(__dirname, '../site/uuid-parse.js'), 'utf8');
const clone = value => JSON.parse(JSON.stringify(value));
const zero = '00000000-0000-0000-0000-000000000000';
const partial = '00112233-4455-6677-8899-aabbccddeeff';
const failure = { parsed: false, canonical_uuid: partial, error: 'invalid UUID format', invalid_length: false };
const success = { parsed: true, canonical_uuid: zero, error: '', invalid_length: false };
const compiled = (inputs, output = failure) => JSON.stringify({ ok: true, inputs, output });
function manifest() {
  return { schema: 'gooo/browser-uuid-parse/v1', entry_id: 'gooo://github.com/google/uuid/text/parse-canonical',
    result_entity_id: 'gooo://github.com/google/uuid/text/parse-canonical-result', input_entity_ids: ['gooo://std/text'],
    compiler_revision: '83928fa4538245abe33b08549da47f8cbdc50ed2', compiler_version: '0.6.32-dev', go_version: 'go1.27.2',
    source_revision: 'b'.repeat(40), source_dirty: false, wasm_bytes: 8, native_cases: 11, fixture_cases: 11, fixture_rejections: 4,
    native_receipt_expectation_cases: 0, wasm_sha256: 'a'.repeat(64), generated_sha256: 'c'.repeat(64),
    execution_sha256: 'd'.repeat(64), replay_sha256: 'e'.repeat(64), fixtures_sha256: 'f'.repeat(64),
    source_sha256: { 'ports/google/uuid/text/parse_canonical.gooo': '1'.repeat(64) }, files_sha256: { 'program.wasm': 'a'.repeat(64),
      'generated.go': 'c'.repeat(64), 'execution.json': 'd'.repeat(64), 'replay.json': 'e'.repeat(64), 'fixtures.json': 'f'.repeat(64) } };
}

test('shared exact Text binding preserves BOM, CRLF, NUL, non-ASCII and every original UTF-16 unit', () => {
  const original = '\uFEFF\r\n\0e\u0301👩‍💻\uD800A\uDFFF', snapshot = protocol.read(original, 'text');
  assert.equal(snapshot.original, original); assert.equal(snapshot.bound, '\uFEFF\r\n\0e\u0301👩‍💻\uFFFDA\uFFFD');
  assert.equal(snapshot.changed, true); assert.equal(snapshot.utf8Bytes, new TextEncoder().encode(original).length);
  assert.deepEqual(snapshot.originalUtf16Units, Array.from({ length: original.length }, (_, i) => original.charCodeAt(i)));
  assert.equal(JSON.parse(snapshot.originalEscaped), original); assert.equal(JSON.parse(snapshot.boundEscaped), snapshot.bound);
  for (const value of ['', ' \t\n ', 'e\u0301', 'é', '[00112233-4455-6677-8899-aabbccddeeff]']) assert.equal(protocol.read(value, 'text').bound, value);
  assert.ok(Object.isFrozen(snapshot)); assert.ok(Object.isFrozen(snapshot.originalUtf16Units));
});

test('whole decoded UTF-8 byte boundary accepts 1024 and rejects overflow before parsing', () => {
  for (const value of ['x'.repeat(1024), 'é'.repeat(512), '😀'.repeat(256), '\uD800'.repeat(341) + 'a']) assert.equal(protocol.read(value, 'text').utf8Bytes, 1024);
  for (const value of ['x'.repeat(1025), 'é'.repeat(513), '😀'.repeat(257), '\uD800'.repeat(342)]) assert.throws(() => protocol.read(value, 'text'), /parser was not called/);
  for (const values of [null, 'x', [], ['x', 'y'], [true], [null], new Array(1)]) assert.throws(() => protocol.validateInputs(values));
  assert.deepEqual(protocol.validateInputs(['']), ['']); assert.throws(() => protocol.validateInputs(['\uD800']), /already/);
  assert.deepEqual(protocol.bindInputs(['\uD800']), ['\uFFFD']);
});

test('JSON mode retains notation separately, decodes only one string and bounds notation', () => {
  const captured = '  "\\uFEFF\\r\\n\\uD800\\uD83D\\uDE00"\t', snapshot = protocol.read(captured, 'json');
  assert.equal(snapshot.capturedSource, captured); assert.equal(snapshot.original, '\uFEFF\r\n\uD800😀'); assert.equal(snapshot.bound, '\uFEFF\r\n\uFFFD😀');
  for (const input of ['null', '{}', '[]', '1', 'true', '"unfinished', '"a" "b"', '"\\uGGGG"', '\uFEFF"x"']) assert.throws(() => protocol.read(input, 'json'));
  assert.throws(() => protocol.read(' '.repeat(8193), 'json'), /8,192/);
  assert.throws(() => protocol.read('"' + '\\u0061'.repeat(1025) + '"', 'json'), /1,024/);
  assert.throws(() => protocol.read('x', 'unknown'));
});

test('parser false with canonical-shaped partial return is a completed ok:true four-field record', () => {
  const inputs = ['raw invalid input'];
  assert.deepEqual(protocol.parseCompiledReply(compiled(inputs), inputs), failure);
  assert.deepEqual(protocol.parseCompiledReply(compiled(inputs, success), inputs), success);
  const length = { parsed: false, canonical_uuid: zero, error: 'invalid UUID length: 0', invalid_length: true };
  assert.deepEqual(protocol.parseCompiledReply(compiled([''], length), ['']), length);
  const bad = [r => { r.ok = false; }, r => { r.inputs = ['other']; }, r => { r.inputs.push(''); }, r => { r.extra = true; },
    r => { r.output.parsed = 'false'; }, r => { r.output.invalid_length = 0; }, r => { r.output.error = ''; },
    r => { r.output.parsed = true; }, r => { r.output.canonical_uuid = ''; }, r => { r.output.extra = true; },
    r => { r.output.error = '\uD800'; }, r => { r.output.error = 'é'.repeat(121); }];
  for (const field of protocol.fields) bad.push(r => { delete r.output[field]; });
  for (const change of bad) { const reply = { ok: true, inputs: [...inputs], output: clone(failure) }; change(reply); assert.throws(() => protocol.parseCompiledReply(JSON.stringify(reply), inputs)); }
  for (const raw of ['{', 'null', '[]', ' '.repeat(32769), JSON.stringify({ ok: false, error: '' }), JSON.stringify({ ok: false, error: 'x'.repeat(241) })]) assert.throws(() => protocol.parseCompiledReply(raw, inputs));
  assert.throws(() => protocol.parseCompiledReply(JSON.stringify({ ok: false, error: 'runtime rejection' }), inputs), /compiled program rejected/);
  assert.deepEqual(protocol.parseBridgeReply(compiled(['\uFFFD']), ['\uD800']), { ok: true, inputs: ['\uFFFD'], output: failure });
});

test('request, error and metadata validation binds exact pins, hashes, counters and full request identity', () => {
  assert.deepEqual(protocol.validateRequest({ type: 'parse', id: 7, inputs: [''] }), { id: 7, inputs: [''] });
  for (const message of [{ type: 'parse', id: 0, inputs: [''] }, { type: 'parse', id: 1.2, inputs: [''] },
    { type: 'parse', id: 1, inputs: [''], extra: true }, { type: 'search', id: 1, inputs: [''] }]) assert.throws(() => protocol.validateRequest(message));
  assert.equal(protocol.validateCancel({ type: 'cancel', id: 1 }), 1); assert.throws(() => protocol.validateCancel({ type: 'cancel', id: 1, extra: true }));
  assert.equal(protocol.validateError({ type: 'error', id: 1, message: 'failed' }, 1), 'failed');
  for (const message of [{ type: 'error', id: 2, message: 'stale' }, { type: 'error', id: 1, message: '' },
    { type: 'error', id: 1, message: 'x'.repeat(241) }, { type: 'error', id: 1, message: 'failed', extra: true }]) assert.throws(() => protocol.validateError(message, 1));
  const mutations = [m => { m.schema = 'other'; }, m => { m.entry_id = 'other'; }, m => { m.result_entity_id = 'other'; },
    m => { m.input_entity_ids.push('gooo://std/text'); }, m => { m.compiler_revision = 'a'.repeat(40); }, m => { m.compiler_version = 'v0.6.32-dev'; },
    m => { m.go_version = 'go1.27.1'; }, m => { m.source_revision = ''; }, m => { m.source_dirty = 0; }, m => { m.wasm_bytes = 0; },
    m => { m.wasm_bytes = 32 * 1024 * 1024 + 1; }, m => { m.native_cases++; }, m => { m.native_receipt_expectation_cases = 1; },
    m => { m.source_sha256 = { '../port.gooo': 'a'.repeat(64) }; }, m => { m.files_sha256 = {}; }, m => { m.files_sha256['generated.go'] = 'a'.repeat(64); }];
  for (const key of ['native_cases', 'fixture_cases', 'fixture_rejections']) mutations.push(m => { m[key] = 0; });
  for (const key of ['wasm_sha256', 'generated_sha256', 'execution_sha256', 'replay_sha256', 'fixtures_sha256']) mutations.push(m => { m[key] = 'z'.repeat(64); });
  for (const change of mutations) { const value = manifest(); change(value); assert.throws(() => protocol.validateMetadata(value)); }
  const program = manifest(), frozen = protocol.validateMetadata(program); program.files_sha256['program.wasm'] = 'f'.repeat(64); assert.equal(frozen.files_sha256['program.wasm'], 'a'.repeat(64));
  const expected = { id: 1, inputs: [''], program: manifest() }, result = { type: 'result', ...expected, output: failure };
  assert.deepEqual(protocol.validateResult(result, expected), failure);
  for (const change of [r => { r.id++; }, r => { r.inputs = ['changed']; }, r => { r.program.source_dirty = true; }, r => { r.output = {}; }, r => { r.extra = true; }]) {
    const value = clone(result); change(value); assert.throws(() => protocol.validateResult(value, expected));
  }
  assert.throws(() => protocol.validateReady({ type: 'ready', program: manifest(), extra: true }));
  assert.doesNotThrow(() => protocol.validateResult({ ...result, program: Object.fromEntries(Object.entries(expected.program).reverse()) }, expected));
});

function workerHarness(run = (...inputs) => compiled(inputs), loadFailure) {
  const messages = []; let listener, fatal;
  const scope = { postMessage(message) { messages.push(clone(message)); }, addEventListener(type, callback) { assert.equal(type, 'message'); listener = callback; } };
  const controller = workerAPI.startWorker(scope, async (_scope, onFatal) => { fatal = onFatal; if (loadFailure) throw new Error(loadFailure); return { program: manifest(), run }; });
  return { messages, ready: controller.ready, send: message => listener({ data: message }), fatal: message => fatal(message) };
}
test('Worker rejects early/malformed/duplicate requests; parser failures remain result messages', async () => {
  const calls = [], h = workerHarness((...inputs) => { calls.push(inputs); return compiled(inputs); });
  await h.send({ type: 'parse', id: 1, inputs: [''] }); assert.equal(calls.length, 0); await h.ready;
  for (const inputs of [['\uD800'], ['x'.repeat(1025)], [], [null]]) await h.send({ type: 'parse', id: 2, inputs });
  assert.equal(calls.length, 0); await h.send({ type: 'parse', id: 2, inputs: ['\uFEFF\r\n'] }); assert.deepEqual(calls, [['\uFEFF\r\n']]);
  assert.equal(h.messages.at(-1).type, 'result'); assert.deepEqual(h.messages.at(-1).output, failure);
  await h.send({ type: 'parse', id: 2, inputs: [''] }); assert.equal(h.messages.at(-1).type, 'error'); assert.equal(calls.length, 1);
});
test('Worker suppresses superseded/cancelled/fatal calls and does not return partial runtime results', async () => {
  const calls = [], h = workerHarness((...inputs) => { calls.push(inputs); return compiled(inputs); }); await h.ready;
  await Promise.all([h.send({ type: 'parse', id: 1, inputs: ['old'] }), h.send({ type: 'parse', id: 2, inputs: ['new'] })]); assert.deepEqual(calls, [['new']]);
  const cancelled = h.send({ type: 'parse', id: 3, inputs: ['cancel'] }); await h.send({ type: 'cancel', id: 3 }); await cancelled; assert.equal(calls.length, 1);
  const pending = h.send({ type: 'parse', id: 4, inputs: ['fatal'] }); h.fatal('stopped'); await pending; assert.equal(calls.length, 1); assert.equal(h.messages.at(-1).id, 4);
  const broken = workerHarness(() => '{}'); await broken.ready; await broken.send({ type: 'parse', id: 1, inputs: [''] }); assert.equal(broken.messages.at(-1).type, 'error');
  const failed = workerHarness(undefined, 'hash mismatch'); await failed.ready; assert.deepEqual(failed.messages, [{ type: 'error', message: 'hash mismatch' }]);
});
function runtimeScope() {
  const bytes = new Uint8Array([0, 97, 115, 109, 1, 0, 0, 0]).buffer, metadata = manifest();
  metadata.wasm_sha256 = createHash('sha256').update(Buffer.from(bytes)).digest('hex'); metadata.files_sha256['program.wasm'] = metadata.wasm_sha256;
  const requests = [], imports = [], fatals = []; let executions = 0;
  const scope = { crypto: webcrypto, fetch: async (url, options) => { requests.push({ url, options }); return { ok: true, json: async () => metadata, arrayBuffer: async () => bytes }; },
    importScripts: value => imports.push(value), WebAssembly: { validate: WebAssembly.validate, instantiate: async () => ({ instance: {} }) } };
  scope.Go = class { constructor() { this.importObject = {}; } run() { executions++; scope.goooUUIDParseReady = true; scope.goooUUIDParse = (...inputs) => compiled(inputs); return new Promise(() => {}); } };
  return { scope, metadata, requests, imports, fatals, executions: () => executions };
}
test('loader checks size, hash, pins and Wasm before runtime execution, with visible missing API/stop failures', async () => {
  for (const mutate of [h => { h.metadata.wasm_bytes++; }, h => { h.metadata.wasm_sha256 = 'a'.repeat(64); },
    h => { h.metadata.compiler_revision = 'a'.repeat(40); }, h => { h.scope.WebAssembly.validate = () => false; }, h => { h.scope.fetch = async () => ({ ok: false }); }]) {
    const h = runtimeScope(); mutate(h); await assert.rejects(workerAPI.loadBrowserRuntime(h.scope, () => {})); assert.deepEqual(h.imports, []); assert.equal(h.executions(), 0);
  }
  const h = runtimeScope(); await workerAPI.loadBrowserRuntime(h.scope, () => {});
  assert.deepEqual(h.requests.map(v => v.url), ['./uuid-parse/manifest.json', './uuid-parse/program.wasm']); assert.ok(h.requests.every(v => v.options.cache === 'no-store')); assert.deepEqual(h.imports, ['./wasm_exec.js']);
  const absent = runtimeScope(); absent.scope.Go = class { constructor() { this.importObject = {}; } run() { return new Promise(() => {}); } };
  await assert.rejects(workerAPI.loadBrowserRuntime(absent.scope, () => {}), /API/);
  const bad = runtimeScope(); bad.scope.WebAssembly.instantiate = async () => { throw new Error('bad imports'); }; await assert.rejects(workerAPI.loadBrowserRuntime(bad.scope, () => {}), /bad imports/);
  for (const reject of [false, true]) { const stopped = runtimeScope(); stopped.scope.Go = class { constructor() { this.importObject = {}; } run() {
    stopped.scope.goooUUIDParseReady = true; stopped.scope.goooUUIDParse = () => '{}'; return reject ? Promise.reject(new Error('stopped')) : Promise.resolve();
  } }; await workerAPI.loadBrowserRuntime(stopped.scope, message => stopped.fatals.push(message)); await Promise.resolve(); assert.match(stopped.fatals[0], /stopped/); }
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
  const elements = new Map(), workers = [], timers = new Map(), events = new Map(), body = new Element(), urls = [], revoked = [], downloads = []; let timerId = 0;
  const get = id => { if (!elements.has(id)) elements.set(id, new Element()); return elements.get(id); }; get('uuid-input-mode').value = 'text';
  const presets = ['canonical', 'raw', 'wrapped', 'raw-error', 'standard-error', 'zero', 'surrogates'].map(name => { const value = new Element(); value.dataset.uuidPreset = name; return value; });
  class Worker {
    constructor(url) { if (options.workerError) throw new Error('no workers'); assert.equal(url, './uuid-parse-worker.js'); this.messages = []; this.terminated = false; workers.push(this); }
    terminate() { this.terminated = true; } postMessage(message) { if (options.postError) throw new Error('post failed'); this.messages.push(clone(message)); }
    deliver(data) { this.onmessage({ data }); }
  }
  const context = { Worker, GoooUUIDParseProtocol: protocol,
    Blob: class extends Blob { constructor(...args) { if (options.blobError) throw new Error('no blob'); super(...args); } },
    URL: { createObjectURL(blob) { if (options.urlError) throw new Error('no URL'); const url = 'blob:uuid-' + (urls.length + 1); urls.push({ url, blob }); return url; }, revokeObjectURL(url) { revoked.push(url); } },
    setTimeout(handler, delay) { const id = ++timerId; timers.set(id, { handler, delay }); return id; }, clearTimeout(id) { timers.delete(id); }, addEventListener(name, handler) { events.set(name, handler); },
    document: { body, getElementById: get, createElement(tag) { const value = new Element(); if (tag === 'a') value.click = () => { if (options.clickError) throw new Error('download refused'); downloads.push({ href: value.href, name: value.download, attached: body.children.includes(value) }); }; return value; },
      querySelectorAll(selector) { assert.equal(selector, '[data-uuid-preset]'); return presets; } } };
  context.window = context; vm.runInNewContext(source, context, { filename: 'site/uuid-parse.js' });
  const current = () => workers.at(-1), ready = (instance = current()) => instance.deliver({ type: 'ready', program: manifest() });
  const type = value => { get('uuid-input').value = value; get('uuid-input').emit('input'); };
  const mode = value => { get('uuid-input-mode').value = value; get('uuid-input-mode').emit('change'); };
  const transfer = (value, kind = 'paste', extra = {}) => { let prevented = false; get('uuid-input').emit(kind, { preventDefault() { prevented = true; },
    [kind === 'paste' ? 'clipboardData' : 'dataTransfer']: { types: ['text/plain'], getData(format) { assert.equal(format, 'text/plain'); return value; }, ...extra } }); assert.equal(prevented, true); };
  const run = () => { get('uuid-form').emit('submit'); return current()?.messages.at(-1); };
  const reply = (instance = current(), changed = {}) => { const request = instance.messages.at(-1); instance.deliver({ type: 'result', id: request.id, inputs: request.inputs, output: failure, program: manifest(), ...changed }); };
  return { get, workers, current, ready, type, mode, transfer, run, reply, timers, body, urls, revoked, downloads,
    download: () => get('uuid-download').emit('click'), pagehide: () => events.get('pagehide')(), pageshow: () => events.get('pageshow')({ persisted: true }),
    preset: name => presets.find(value => value.dataset.uuidPreset === name).emit('click') };
}
test('paste/drop capture pre-editor CRLF and exact units; JSON mode binds once and rejects invalid/full overflow sources', () => {
  for (const kind of ['paste', 'drop']) { const h = app(); h.ready(); h.transfer('\uFEFF\uD800\r\n<unsafe>', kind);
    assert.equal(h.get('uuid-input').value, ''); assert.equal(JSON.parse(h.get('uuid-input-preview').textContent), '\uFEFF\uD800\r\n<unsafe>');
    assert.equal(JSON.parse(h.get('uuid-input-bound').textContent), '\uFEFF\uFFFD\r\n<unsafe>'); assert.equal(h.get('uuid-input-bound-wrap').hidden, false);
    assert.deepEqual(h.run().inputs, ['\uFEFF\uFFFD\r\n<unsafe>']); h.reply(); assert.equal(h.get('uuid-output').children.length, 4);
  }
  const h = app(); h.ready(); h.mode('json'); h.type('"\\r\\n\\uD800"'); assert.deepEqual(h.run().inputs, ['\r\n\uFFFD']); h.reply();
  for (const value of ['null', '"unfinished', '"' + 'x'.repeat(1025) + '"']) { h.type(value); assert.equal(h.get('uuid-run').disabled, true); assert.equal(h.get('uuid-download').disabled, true); assert.equal(h.get('uuid-output').children.length, 0); }
  h.mode('text'); h.transfer('', 'paste', { files: [{}] }); assert.equal(h.get('uuid-run').disabled, true);
  h.transfer('', 'drop', { getData() { throw new Error('denied'); } }); assert.equal(h.get('uuid-run').disabled, true);
});
test('built-in examples show their editable source and send it unchanged; typing starts a new input', () => {
  const h = app(); h.ready(); h.preset('raw-error');
  const original = '00112233445566778899aabbccddee0g';
  assert.equal(h.get('uuid-input').value, original);
  assert.equal(JSON.parse(h.get('uuid-input-preview').textContent), original);
  assert.deepEqual(h.run().inputs, [original]); h.reply();
  assert.equal(h.get('uuid-output').children[1].children[1].textContent, partial);
  assert.equal(h.get('uuid-output').children[0].children[1].textContent, 'false');
  h.type(h.get('uuid-input').value + 'x');
  assert.equal(h.get('uuid-download').disabled, true);
  assert.equal(JSON.parse(h.get('uuid-input-preview').textContent), original + 'x');
  assert.deepEqual(h.run().inputs, [original + 'x']); h.reply();
  h.preset('surrogates');
  assert.equal(h.get('uuid-input-mode').value, 'json');
  assert.equal(h.get('uuid-input').value, '"\\uFEFF\\uD800A\\r\\n"');
  assert.deepEqual(h.run().inputs, ['\uFEFF\uFFFDA\r\n']);
});
test('all four fields display/export on parser error; success comes only from returned parsed, including zero', async () => {
  const h = app(); h.ready(); h.preset('raw-error'); h.run(); h.reply();
  const rows = h.get('uuid-output').children; assert.equal(rows[0].children[1].textContent, 'false'); assert.equal(rows[1].children[1].textContent, partial);
  assert.match(rows[1].children[0].textContent, /Returned UUID on error/); assert.equal(rows[2].children[1].textContent, failure.error); assert.equal(rows[3].children[1].textContent, 'false');
  assert.match(h.get('uuid-result-note').textContent, /parsed: false/); assert.equal(h.get('uuid-result-error').hidden, true); assert.equal(h.get('uuid-download').disabled, false);
  h.download(); assert.deepEqual(JSON.parse(await h.urls[0].blob.text()).output, failure);
  h.preset('zero'); h.run(); h.reply(h.current(), { output: success }); assert.match(h.get('uuid-result-note').textContent, /parsed: true/);
  assert.equal(h.get('uuid-output').children[1].children[1].textContent, zero); assert.equal(h.get('uuid-output').children[2].children[1].textContent, '"" (empty; no parser error)');
  h.type('arbitrary'); h.run(); const unsafe = { ...failure, error: 'invalid urn prefix: "<unsafe>&"' }; h.reply(h.current(), { output: unsafe });
  assert.equal(h.get('uuid-output').children[2].children[1].textContent, unsafe.error); assert.equal(h.get('uuid-output').children[2].children[1].children.length, 0);
});
test('edits, presets and cancellation terminate busy Worker and discard every stale result/export', () => {
  for (const change of [h => h.type('current'), h => h.transfer('current'), h => h.get('uuid-cancel').emit('click'), h => h.preset('wrapped')]) {
    const h = app(); h.ready(); h.type('old'); h.run(); const old = h.current(); change(h); assert.equal(old.terminated, true); h.reply(old); old.onerror({ preventDefault() {} }); old.onmessageerror();
    assert.equal(h.get('uuid-output-wrap').hidden, true); assert.equal(h.get('uuid-download').disabled, true); h.ready(); h.run(); h.reply(); assert.equal(h.get('uuid-download').disabled, false); h.reply(old); assert.equal(h.get('uuid-download').disabled, false);
  }
});
test('corrupt replies, fatal errors and unsupported messages clear results and expose retry; stale IDs are ignored', () => {
  for (const send of [h => h.reply(h.current(), { program: { ...manifest(), source_dirty: true } }), h => h.reply(h.current(), { inputs: ['wrong'] }),
    h => h.reply(h.current(), { output: {} }), h => h.current().deliver({ type: 'unknown' }), h => h.current().deliver({ type: 'error', message: 'fatal' }),
    h => h.current().onmessageerror(), h => h.current().onerror({ preventDefault() {} })]) {
    const h = app(); h.ready(); h.run(); send(h); assert.equal(h.current().terminated, true); assert.equal(h.get('uuid-download').disabled, true); assert.equal(h.get('uuid-retry').hidden, false); assert.equal(h.timers.size, 0);
  }
  const h = app(); h.ready(); const request = h.run(); h.current().deliver({ type: 'error', id: request.id + 1, message: 'stale' }); assert.equal(h.current().terminated, false);
  h.reply(); h.current().deliver({ type: 'error', message: 'stopped after completion' }); assert.equal(h.get('uuid-download').disabled, true);
});
test('load/run deadlines stop their own Worker; retry, construction/post failure and restored-page lifecycle are explicit', () => {
  const h = app(), loading = [...h.timers.values()].find(v => v.delay === 30000); loading.handler(); assert.equal(h.current().terminated, true);
  h.get('uuid-retry').emit('click'); h.ready(); loading.handler(); assert.equal(h.current().terminated, false); h.run(); const running = [...h.timers.values()].find(v => v.delay === 10000);
  running.handler(); assert.equal(h.current().terminated, true); h.get('uuid-retry').emit('click'); h.ready(); h.run(); h.reply(); running.handler(); assert.equal(h.get('uuid-download').disabled, false);
  h.transfer('\r\n\uD800'); h.run(); h.reply(); const old = h.current(); h.pagehide(); assert.equal(old.terminated, true); h.pageshow(); h.ready(); assert.deepEqual(h.run().inputs, ['\r\n\uFFFD']); h.reply(old); assert.equal(h.get('uuid-download').disabled, true); h.reply();
  const absent = app({ workerError: true }); assert.equal(absent.get('uuid-retry').hidden, false);
  const posting = app({ postError: true }); posting.ready(); posting.run(); assert.equal(posting.current().terminated, true);
  const duplicate = app(); duplicate.ready(); duplicate.ready(); assert.equal(duplicate.current().terminated, true);
});
test('download captures all original/bound units and program hashes; requested downloads clean URLs and survive download refusal', async () => {
  const h = app(); h.ready(); h.preset('surrogates'); h.run(); h.reply(); h.download();
  const serialized = await h.urls[0].blob.text(), report = JSON.parse(serialized); assert.ok(serialized.endsWith('\n')); assert.equal(report.schema, 'gooo/uuid-parse-observation/v1');
  assert.equal(report.inputs[0].captured_source, '"\\uFEFF\\uD800A\\r\\n"'); assert.deepEqual(report.inputs[0].original_utf16_units, [65279, 55296, 65, 13, 10]);
  assert.deepEqual(report.inputs[0].bound_utf16_units, [65279, 65533, 65, 13, 10]); assert.equal(report.inputs[0].replaced_unpaired_surrogates, true);
  assert.deepEqual(report.output, failure); assert.deepEqual(report.program, manifest()); assert.match(report.scope, /not a native execution receipt.*signed attestation/);
  assert.equal(h.urls[0].blob.type, 'application/json;charset=utf-8'); assert.deepEqual(h.downloads, [{ href: 'blob:uuid-1', name: 'gooo-uuid-parse-observation.json', attached: true }]);
  assert.equal(h.body.children.length, 0); assert.match(h.get('uuid-download-status').textContent, /Download requested/); assert.doesNotMatch(h.get('uuid-download-status').textContent, /saved|successfully/i);
  [...h.timers.values()].find(v => v.delay === 1000).handler(); assert.deepEqual(h.revoked, ['blob:uuid-1']); assert.equal(h.timers.size, 0);
  for (const phase of ['blobError', 'urlError', 'clickError']) { const options = { [phase]: true }, failed = app(options); failed.ready(); failed.run(); failed.reply(); failed.download();
    assert.match(failed.get('uuid-download-status').textContent, /could not be started/); assert.equal(failed.get('uuid-download').disabled, false); assert.deepEqual(failed.revoked, failed.urls.map(v => v.url));
    options[phase] = false; failed.download(); failed.type('changed'); const count = failed.downloads.length; failed.download(); assert.equal(failed.downloads.length, count); failed.pagehide(); assert.deepEqual(failed.revoked, failed.urls.map(v => v.url));
  }
});
test('HTML controls have unique accessible names, dependencies, explicit error-return rules and honest observation scope', () => {
  const html = fs.readFileSync(path.join(__dirname, '../site/uuid-parse.html'), 'utf8'), ids = [...html.matchAll(/\bid="([^"]+)"/g)].map(v => v[1]);
  assert.equal(new Set(ids).size, ids.length); for (const id of [...source.matchAll(/\$\('([^']+)'\)/g)].map(v => v[1])) assert.ok(ids.includes(id), id);
  for (const suffix of ['', '-mode', '-label', '-source', '-status', '-preview', '-bound', '-bound-wrap']) assert.ok(ids.includes('uuid-input' + suffix));
  assert.match(html, /<label[^>]*for="uuid-input"/); assert.match(html, /correctly shaped returned UUID does not establish parser success/);
  assert.match(html, /Unpaired UTF-16 surrogates become U\+FFFD/); assert.match(html, /not a native execution receipt or signed attestation/);
  for (const name of ['literal-search-protocol.js', 'uuid-parse-protocol.js', 'uuid-parse.js']) assert.ok(html.includes('<script defer src="./' + name + '"></script>'));
  assert.match(fs.readFileSync(path.join(__dirname, '../site/uuid-parse.css'), 'utf8'), /@import url\('\.\/literal-search\.css'\)/);
});
