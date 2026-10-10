const test = require('node:test');
const assert = require('node:assert/strict');
const { webcrypto, createHash } = require('node:crypto');
const protocol = require('../site/text-inspector-protocol.js');
const worker = require('../site/text-inspector-worker.js');
const identity = { wasmSha256: 'a'.repeat(64), sourceRevision: 'b'.repeat(40) };
const clone = value => JSON.parse(JSON.stringify(value));
const reply = (operation, input, value) => JSON.stringify({ ok: true, operation, inputs: [input, '0'],
  output: { type: operation.includes('RuneLen') ? 'integer' : 'boolean', value } });
const fixedRun = (operation, input) => reply(operation, input, operation.includes('RuneLen') ? '1' : true);
const row = input => ({ input, validScalar: true, goSpace: true, goLetter: true, goDigit: true, utf8Bytes: '1', utf16Units: '1' });

test('request keeps ordered duplicate points and lone-surrogate integers without classification', () => {
  const original = ['0', '55296', '57343', '1114111', '0'];
  const request = protocol.validateRequest({ type: 'inspect', id: 1, points: original });
  original[0] = '99';
  assert.deepEqual(request.points, ['0', '55296', '57343', '1114111', '0']);
  assert.equal(protocol.validatePoints(Array(256).fill('1114111')).length, 256);
  assert.deepEqual(protocol.validatePoints([]), []);
});

test('malformed and oversized inputs are all rejected before any compiled call', async () => {
  for (const value of [null, {}, '65', Array(257).fill('65'), ['-1'], ['-0'], ['+1'], ['01'], [' 1'],
    ['1 '], ['1e1'], ['1.0'], ['0x41'], ['1114112'], ['9007199254740993'], [65], [true], [null], new Array(1)]) {
    let calls = 0;
    await assert.rejects(worker.inspectPoints(value, () => { calls++; return ''; }));
    assert.equal(calls, 0);
  }
  for (const value of [null, {}, { type: 'inspect', id: 0, points: [] }, { type: 'inspect', id: 1.5, points: [] },
    { type: 'inspect', id: Number.MAX_SAFE_INTEGER + 1, points: [] }, { type: 'inspect', id: 1, points: [], extra: true },
    { type: 'other', id: 1, points: [] }]) assert.throws(() => protocol.validateRequest(value));
});

test('compiled replies bind exact operation, both input strings, output type and scalar range', () => {
  const valid = JSON.parse(reply('utf8.RuneLen', '160', '2'));
  assert.equal(protocol.parseCompiledReply(JSON.stringify(valid), 'utf8.RuneLen', '160'), '2');
  const mutate = change => { const value = clone(valid); change(value); return JSON.stringify(value); };
  const bad = ['null', '[]', '{', ' '.repeat(513), mutate(r => { r.ok = false; }), mutate(r => { r.operation = 'utf16.RuneLen'; }),
    mutate(r => { r.inputs[0] = '161'; }), mutate(r => { r.inputs[1] = '1'; }), mutate(r => { r.inputs[0] = 160; }),
    mutate(r => { r.inputs.push('0'); }), mutate(r => { r.output.type = 'boolean'; }), mutate(r => { r.extra = true; }),
    mutate(r => { delete r.operation; }), mutate(r => { r.output.extra = true; })];
  for (const value of [2, '02', '+2', '-0', '0', '5', '-2', null, true]) bad.push(mutate(r => { r.output.value = value; }));
  for (const raw of bad) assert.throws(() => protocol.parseCompiledReply(raw, 'utf8.RuneLen', '160'));
  for (const value of ['3', '4', '0', '-2']) assert.throws(() => protocol.parseCompiledReply(reply('utf16.RuneLen', '65', value), 'utf16.RuneLen', '65'));
  for (const value of [0, 1, 'true', null]) assert.throws(() => protocol.parseCompiledReply(reply('unicode.IsSpace', '65', value), 'unicode.IsSpace', '65'));
  assert.throws(() => protocol.parseCompiledReply(reply('bits.Len8', '65', '1'), 'bits.Len8', '65'));
});

test('six existing unary calls supply displayed values; host does not reclassify results', async () => {
  const calls = [];
  const points = ['65', '55296', '65'];
  const rows = await worker.inspectPoints(points, (...args) => { calls.push(args); return fixedRun(...args); });
  // Deliberately semantically arbitrary protocol values remain untouched. This
  // test checks transport only; the real-Wasm smoke compares independent Go.
  assert.deepEqual(rows, points.map(row));
  assert.deepEqual(calls, points.flatMap(point => protocol.OPERATIONS.map(operation => [operation, point, '0'])));
  let emptyCalls = 0;
  assert.deepEqual(await worker.inspectPoints([], () => { emptyCalls++; }), []);
  assert.equal(emptyCalls, 0);
});

test('bad later compiled reply fails the whole request and exposes no result rows', async () => {
  let calls = 0;
  await assert.rejects(worker.inspectPoints(['65', '66'], (operation, input) => {
    calls++;
    return input === '66' ? reply(operation, '65', true) : fixedRun(operation, input);
  }));
  assert.equal(calls, 7);
});

test('result identity and exact row roster reject stale, truncated, reordered and malformed replies', () => {
  const expected = { id: 3, points: ['65', '66'], ...identity };
  const valid = { type: 'result', id: 3, rows: expected.points.map(row), ...identity };
  assert.deepEqual(protocol.validateResult(valid, expected), valid.rows);
  const mutations = [r => { r.id = 2; }, r => { r.wasmSha256 = 'c'.repeat(64); }, r => { r.sourceRevision = 'c'.repeat(40); },
    r => { r.rows.pop(); }, r => { r.rows.reverse(); }, r => { r.rows[0].input = 65; }, r => { r.rows[0].goSpace = 'true'; },
    r => { r.rows[0].validScalar = 1; }, r => { r.rows[0].utf8Bytes = 1; }, r => { r.rows[0].utf16Units = '4'; },
    r => { r.rows[0].extra = true; }, r => { delete r.rows[0].input; }, r => { r.extra = true; }];
  for (const change of mutations) { const value = clone(valid); change(value); assert.throws(() => protocol.validateResult(value, expected)); }
});

test('ready, progress, cancellation and error messages have bounded exact shapes', () => {
  assert.deepEqual(protocol.validateReady({ type: 'ready', ...identity }), identity);
  assert.throws(() => protocol.validateReady({ type: 'ready', ...identity, wasmSha256: 'x' }));
  assert.equal(protocol.validateProgress({ type: 'progress', id: 3, completed: 16, total: 20 }, { id: 3, total: 20 }), 16);
  for (const completed of [-1, 21, 1.5, '1']) assert.throws(() => protocol.validateProgress({ type: 'progress', id: 3, completed, total: 20 }, { id: 3, total: 20 }));
  assert.throws(() => protocol.validateProgress({ type: 'progress', id: 2, completed: 1, total: 20 }, { id: 3, total: 20 }));
  assert.equal(protocol.validateCancel({ type: 'cancel', id: 3 }), 3);
  assert.throws(() => protocol.validateCancel({ type: 'cancel', id: 3, points: [] }));
  assert.equal(protocol.validateError({ type: 'error', id: 3, message: 'Failed.' }, 3), 'Failed.');
  assert.equal(protocol.validateError({ type: 'error', message: 'Load failed.' }, 3), 'Load failed.');
  assert.throws(() => protocol.validateError({ type: 'error', id: 2, message: 'Failed.' }, 3));
  assert.throws(() => protocol.validateError({ type: 'error', message: 'x'.repeat(241) }));
});

test('chunk yield permits cancellation without computing the next point', async () => {
  let current = true, calls = 0, yields = 0;
  await assert.rejects(worker.inspectPoints(Array(17).fill('65'), (...args) => { calls++; return fixedRun(...args); }, {
    isCurrent: () => current, yieldControl: async () => { yields++; current = false; }
  }), { name: 'AbortError' });
  assert.equal(calls, 16 * 6);
  assert.equal(yields, 1);
});

function harness(run = fixedRun) {
  const messages = [];
  let listener;
  const scope = { postMessage(message) { messages.push(message); }, addEventListener(type, callback) {
    assert.equal(type, 'message'); listener = callback;
  } };
  const controller = worker.startWorker(scope, async () => ({ identity, run }));
  return { messages, scope, ready: controller.ready, send: message => listener({ data: message }) };
}

test('Worker rejects early/duplicate IDs and suppresses a superseded result', async () => {
  const h = harness();
  await h.send({ type: 'inspect', id: 1, points: ['65'] });
  assert.equal(h.messages[0].type, 'error');
  await h.ready;
  const first = h.send({ type: 'inspect', id: 2, points: Array(17).fill('65') });
  await h.send({ type: 'inspect', id: 3, points: ['66'] });
  await first;
  assert.deepEqual(h.messages.filter(m => m.type === 'result').map(m => m.id), [3]);
  await h.send({ type: 'inspect', id: 3, points: ['66'] });
  assert.equal(h.messages.at(-1).type, 'error');
});

test('Worker cancellation and later output failure never publish partial rows', async () => {
  let calls = 0;
  const h = harness((...args) => { calls++; return fixedRun(...args); });
  await h.ready;
  const running = h.send({ type: 'inspect', id: 1, points: Array(17).fill('65') });
  await h.send({ type: 'cancel', id: 1 });
  await running;
  assert.equal(calls, 96);
  assert.equal(h.messages.filter(m => m.type === 'result').length, 0);
  const failed = harness((operation, input) => input === '66' ? '{}' : fixedRun(operation, input));
  await failed.ready;
  await failed.send({ type: 'inspect', id: 1, points: ['65', '66'] });
  assert.equal(failed.messages.filter(m => m.type === 'result').length, 0);
  assert.equal(failed.messages.at(-1).type, 'error');
});

function runtimeScope(run = fixedRun) {
  const bytes = new Uint8Array([0, 1, 2, 3]).buffer;
  const metadata = { schema: 'gooo/browser-demo/v2', wasm_sha256: createHash('sha256').update(Buffer.from(bytes)).digest('hex'),
    source_revision: identity.sourceRevision, wasm_bytes: 4,
    entry_id: 'gooo://go-ports/examples/browser-demo/dispatch/v2', reply_entity_id: 'gooo://go-ports/examples/browser-demo/reply/v2',
    input_entity_ids: ['gooo://std/text', ...Array(4).fill('gooo://std/integer')] };
  const loaded = [];
  const scope = { crypto: webcrypto, fetch: async name => ({ ok: true, json: async () => metadata, arrayBuffer: async () => bytes }),
    importScripts: name => loaded.push(name), WebAssembly: { instantiate: async () => ({ instance: {} }) } };
  scope.Go = class { constructor() { this.importObject = {}; } run() {
    scope.goooReady = true; scope.goooRun = run; return new Promise(() => {});
  } };
  return { scope, metadata, loaded, bytes };
}

test('Worker loader verifies the current v2 metadata, byte length and hash before executing Wasm', async () => {
  for (const change of [m => { m.schema = 'v3'; }, m => { m.entry_id = 'other'; }, m => { m.input_entity_ids.pop(); },
    m => { m.wasm_bytes = 5; }, m => { m.wasm_sha256 = 'c'.repeat(64); }, m => { m.source_revision = 'x'; }]) {
    const { scope, metadata, loaded } = runtimeScope(); change(metadata);
    await assert.rejects(worker.loadBrowserRuntime(scope, () => {}));
    assert.deepEqual(loaded, []);
  }
  const { scope, metadata, loaded } = runtimeScope();
  const runtime = await worker.loadBrowserRuntime(scope, () => {});
  assert.deepEqual(runtime.identity, { wasmSha256: metadata.wasm_sha256, sourceRevision: metadata.source_revision });
  assert.deepEqual(loaded, ['./wasm_exec.js']);
  assert.equal(typeof runtime.run, 'function');
});

for (const staleResource of ['./metadata.json', './demo.wasm']) {
  test(`Worker loader executes the current program when HTTP cache holds stale ${staleResource}`, async () => {
    const calls = [];
    const { scope, metadata, loaded, bytes } = runtimeScope((...args) => {
      calls.push(args); return fixedRun(...args);
    });
    const staleBytes = new Uint8Array([4, 5, 6, 7, 8, 9, 10, 11]).buffer;
    const staleMetadata = { ...clone(metadata), source_revision: 'c'.repeat(40),
      wasm_bytes: staleBytes.byteLength,
      wasm_sha256: createHash('sha256').update(Buffer.from(staleBytes)).digest('hex') };
    // Only one resource is stale in each case. Default caching and no-cache
    // revalidation can reuse old bytes when a static server's validators collide.
    scope.fetch = async (name, options = {}) => {
      const serveStale = name === staleResource && options.cache !== 'no-store';
      if (name === './metadata.json') return { ok: true, json: async () => clone(serveStale ? staleMetadata : metadata) };
      assert.equal(name, './demo.wasm');
      return { ok: true, arrayBuffer: async () => serveStale ? staleBytes : bytes };
    };
    let instantiated = 0;
    scope.WebAssembly.instantiate = async received => {
      assert.deepEqual(new Uint8Array(received), new Uint8Array(bytes));
      instantiated++;
      return { instance: {} };
    };

    const runtime = await worker.loadBrowserRuntime(scope, () => {});
    assert.deepEqual(runtime.identity, { wasmSha256: metadata.wasm_sha256, sourceRevision: metadata.source_revision });
    assert.equal(instantiated, 1);
    assert.deepEqual(loaded, ['./wasm_exec.js']);
    const points = ['65', '160'];
    assert.deepEqual(await worker.inspectPoints(points, runtime.run), points.map(row));
    assert.deepEqual(calls, points.flatMap(point => protocol.OPERATIONS.map(operation => [operation, point, '0'])));
  });
}

test('failed Worker load reports an error without becoming ready', async () => {
  const messages = [];
  const scope = { addEventListener() {}, postMessage: message => messages.push(message) };
  await worker.startWorker(scope, async () => { throw new Error('Hash mismatch.'); }).ready;
  assert.deepEqual(messages, [{ type: 'error', message: 'Hash mismatch.' }]);
});

test('letter and digit outputs are mandatory Booleans and a final classification failure discards the request', async () => {
  for (const key of ['goLetter', 'goDigit']) {
    for (const value of [undefined, null, 0, 'true']) {
      const changed = { ...row('65'), [key]: value };
      const request = { id: 1, points: ['65'], ...identity };
      assert.throws(() => protocol.validateResult({ type: 'result', ...request, rows: [changed] }, request));
    }
  }
  for (const operation of ['unicode.IsLetter', 'unicode.IsDigit']) {
    for (const value of [null, 1, 'false']) assert.throws(() => protocol.parseCompiledReply(reply(operation, '65', value), operation, '65'));
  }
  let calls = 0;
  await assert.rejects(worker.inspectPoints(['65', '66'], (operation, input) => {
    calls++;
    return operation === 'unicode.IsDigit' && input === '66' ? reply(operation, input, 'true') : fixedRun(operation, input);
  }));
  assert.equal(calls, 12);
});
