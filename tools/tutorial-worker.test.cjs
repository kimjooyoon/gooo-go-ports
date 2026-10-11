// Transport failures are tested with counted doubles. Actual Gooo equivalence
// belongs to tutorial-browser-smoke.cjs and the native build evidence.
const test = require('node:test');
const assert = require('node:assert/strict');
const { webcrypto, createHash } = require('node:crypto');
const protocol = require('../site/tutorial-protocol.js');
const worker = require('../site/tutorial-worker.js');
const bytes = Buffer.from('tutorial test program');
const hash = value => createHash('sha256').update(value).digest('hex');
const metadata = { schema: 'gooo/browser-tutorial/v1', compiler_revision: '83928fa4538245abe33b08549da47f8cbdc50ed2',
  compiler_version: '0.6.32-dev', go_version: 'go1.27.2', source_revision: 'b'.repeat(40), source_dirty: false,
  entry_id: 'urn:learn-gooo:describe', result_entity_id: 'urn:learn-gooo:summary',
  input_entity_ids: ['urn:gooo:type:integer', 'urn:gooo:type:string'], wasm_sha256: hash(bytes), wasm_bytes: bytes.length,
  generated_sha256: 'c'.repeat(64), execution_sha256: 'd'.repeat(64), replay_sha256: 'e'.repeat(64),
  fixture_cases: 20, native_cases: 4, source_zip_sha256: 'f'.repeat(64), source_zip_bytes: 10,
  sample_sha256: Object.fromEntries(protocol.files.map(file => [file, 'a'.repeat(64)])) };
const copy = value => JSON.parse(JSON.stringify(value));
const output = { score: '17', remaining: '83', in_range: false, label: 'returned by a counted test double' };
const reply = inputs => JSON.stringify({ ok: true, inputs: [inputs[0], protocol.boundText(inputs[1])], output });

test('whole int64 range and native Text transport preserve exact inputs without calculating the result', () => {
  const labels = ['', ' sensor ', '\0', '\r\n', '"\\\n', '\uFEFFsensor', '\u2028\u2029', 'e\u0301', '😀', '\uD800', '\uDC00', '\uD800A\uDC00'];
  const bound = ['', ' sensor ', '\0', '\r\n', '"\\\n', '\uFEFFsensor', '\u2028\u2029', 'e\u0301', '😀', '\uFFFD', '\uFFFD', '\uFFFDA\uFFFD'];
  for (const score of ['-9223372036854775808', '9223372036854775807', '9007199254740992', '9007199254740993']) {
    labels.forEach((label, index) => {
      assert.deepEqual(protocol.validateInputs([score, label]), [score, label]);
      assert.equal(protocol.boundText(label), bound[index]);
      const result = protocol.parseReply(reply([score, label]), [score, label]);
      assert.deepEqual(result.inputs, [score, bound[index]]);
      assert.deepEqual(result.output, output, 'UI transport must not replace the program output with its own clamp');
    });
  }
  assert.equal(protocol.labelFromEditor('"\\r\\n\\u0000\\ud800"', 'json'), '\r\n\0\uD800');
  assert.equal(protocol.labelFromEditor('\\n', 'text'), '\\n');
  for (const raw of ['null', '1', '{}', '[]', 'sensor', '"broken']) assert.throws(() => protocol.labelFromEditor(raw, 'json'));
});

test('input limit is 1024 UTF-8 bytes, preserving output-limit failures rather than narrowing input', () => {
  for (const label of ['a'.repeat(1024), 'é'.repeat(512), '😀'.repeat(256), '\uD800'.repeat(341) + 'a']) {
    assert.equal(protocol.validateInputs(['42', label])[1], label);
  }
  for (const label of ['a'.repeat(1025), 'é'.repeat(513), '😀'.repeat(257), '\uD800'.repeat(342)]) {
    assert.throws(() => protocol.validateInputs(['42', label]), /1,024/);
  }
  assert.throws(() => protocol.parseReply(JSON.stringify({ ok: false, error: 'returned Text exceeds 1024 bytes' }), ['42', 'a'.repeat(1024)]), /returned Text/);
  const badOutput = { ok: true, inputs: ['42', 'a'], output: { ...output, label: 'a'.repeat(1025) } };
  assert.throws(() => protocol.parseReply(JSON.stringify(badOutput), ['42', 'a']));
});

test('reply shape, bound inputs, exact integers and Boolean fields fail closed', () => {
  const valid = JSON.parse(reply(['9007199254740993', '\uFEFFx\uD800']));
  const changes = [r => { r.inputs[0] = '9007199254740992'; }, r => { r.inputs[1] = 'x�'; },
    r => { r.inputs[1] = '\uFEFFx\uD800'; }, r => { r.inputs.push(''); }, r => { r.extra = true; },
    r => { delete r.output.score; }, r => { r.output.score = 17; }, r => { r.output.score = '017'; },
    r => { r.output.score = '101'; }, r => { r.output.remaining = '-1'; }, r => { r.output.in_range = 'false'; },
    r => { r.output.label = null; }, r => { r.output.label = '\uD800'; }, r => { r.output.extra = true; }];
  for (const change of changes) { const value = copy(valid); change(value); assert.throws(() => protocol.parseReply(JSON.stringify(value), ['9007199254740993', '\uFEFFx\uD800'])); }
  for (const raw of [null, '{', 'null', '[]', 'x'.repeat(20001)]) assert.throws(() => protocol.parseReply(raw, ['0', '']));
});

test('downloaded inputs preserve int64 JSON tokens and exact string escapes', () => {
  const label = '\uFEFF\r\n\0\uD800😀';
  const raw = protocol.inputDocument(['9223372036854775807', label]);
  assert.match(raw, /"app\/report:Describe.input0": 9223372036854775807,/);
  assert.equal(JSON.parse(raw).inputs[0]['app/report:Describe.input1'], label);
  assert.match(raw, /\\r\\n\\u0000\\ud800/);
});

function harness(run = (...inputs) => reply(inputs), loadError) {
  const messages = [], calls = [];
  let listener, kill;
  const scope = { postMessage: message => messages.push(message), addEventListener(type, fn) { listener = fn; } };
  const control = worker.startWorker(scope, async (_, fatal) => {
    kill = fatal;
    if (loadError) throw new Error(loadError);
    return { metadata, run: (...inputs) => { calls.push(inputs); return run(...inputs); } };
  });
  return { messages, calls, ready: () => control.ready, send: message => listener({ data: message }), kill: () => kill('Program stopped.') };
}

test('invalid arity/type/canonical values never reach the compiled callback; full valid inputs do', async () => {
  const h = harness(); await h.ready();
  const invalid = [[], ['0'], ['0', '', 'extra'], ['0', null], ['0', 1], ['0', 'é'.repeat(513)],
    ...['', ' 1', '1 ', '+1', '01', '-0', '1e2', '1.0', '9223372036854775808', '-9223372036854775809'].map(score => [score, 'sensor'])];
  let id = 0;
  for (const inputs of invalid) h.send({ type: 'run', id: ++id, inputs });
  assert.equal(h.calls.length, 0);
  h.send({ type: 'run', id: ++id, inputs: ['-9223372036854775808', '\uFEFF\uD800\r\n\0'] });
  assert.deepEqual(h.calls, [['-9223372036854775808', '\uFEFF\uD800\r\n\0']]);
  assert.equal(h.messages.at(-1).result.inputs[1], '\uFEFF�\r\n\0');
  h.send({ type: 'run', id, inputs: ['0', ''] });
  assert.equal(h.calls.length, 1);
});

test('early requests, output failures and stopped runtimes never publish success', async () => {
  const h = harness(() => '{}');
  h.send({ type: 'run', id: 1, inputs: ['42', 'sensor'] });
  assert.equal(h.calls.length, 0);
  await h.ready();
  h.send({ type: 'run', id: 2, inputs: ['42', 'sensor'] });
  assert.equal(h.calls.length, 1);
  assert.equal(h.messages.filter(m => m.type === 'result').length, 0);
  h.kill(); h.send({ type: 'run', id: 3, inputs: ['42', 'sensor'] });
  assert.equal(h.calls.length, 1);
  assert.equal(h.messages.some(m => m.fatal === true), true);
  const failed = harness(undefined, 'Bad hash'); await failed.ready();
  assert.equal(failed.messages.some(m => m.type === 'ready'), false);
});

function runtimeScope(changes = {}) {
  const manifest = { ...copy(metadata), ...changes };
  let calls = 0, imported = 0;
  const fetches = [];
  const scope = { crypto: webcrypto, fetch: async (name, options) => {
    fetches.push({ name, options });
    return { ok: true, json: async () => manifest, arrayBuffer: async () => bytes };
  }, importScripts() { imported++; }, WebAssembly: { instantiate: async () => { calls++; return { instance: {} }; } } };
  scope.Go = class { constructor() { this.importObject = {}; } run() {
    scope.goooTutorialReady = true; scope.goooTutorial = (...inputs) => reply(inputs); return new Promise(() => {});
  } };
  return { scope, fetches, get instantiated() { return calls; }, get imported() { return imported; } };
}

test('hash, byte size and independent entry/type contract are checked before any Wasm instantiation', async () => {
  for (const change of [{ wasm_sha256: '0'.repeat(64) }, { wasm_bytes: 3 }, { schema: 'gooo/browser-demo/v2' },
    { entry_id: 'other' }, { result_entity_id: 'other' }, { input_entity_ids: ['urn:gooo:type:string', 'urn:gooo:type:integer'] },
    { sample_sha256: {} }, { compiler_revision: 'a'.repeat(40) },
    { compiler_revision: '366ee6ef8f9d2b9b57331dfd26b6f908c0289851' }, { compiler_version: '0.6.24-dev' },
    { compiler_revision: '366ee6ef8f9d2b9b57331dfd26b6f908c0289851', compiler_version: '0.6.24-dev' },
    { compiler_revision: '90ff2bb5aeffc2ac553496b646d14db1e8d33fdb' }, { compiler_version: '0.6.29-dev' },
    { compiler_revision: '90ff2bb5aeffc2ac553496b646d14db1e8d33fdb', compiler_version: '0.6.29-dev' }, { source_zip_sha256: null }]) {
    const h = runtimeScope(change);
    await assert.rejects(worker.loadRuntime(h.scope, () => {}));
    assert.equal(h.instantiated, 0); assert.equal(h.imported, 0);
  }
  const h = runtimeScope(); const runtime = await worker.loadRuntime(h.scope, () => {});
  assert.equal(h.instantiated, 1); assert.equal(runtime.metadata.wasm_sha256, hash(bytes));
  assert.deepEqual(h.fetches.map(x => x.name), ['./tutorial/manifest.json', './tutorial/program.wasm']);
  assert.ok(h.fetches.every(x => x.options.cache === 'no-store'));
});
