// Test UI trust boundaries with a counted runtime double. The crop Wasm smoke
// test separately exercises actual compilation and the independent Go oracle.
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { webcrypto, createHash } = require('node:crypto');
const protocol = require('../site/crop-planner-protocol.js');
const source = fs.readFileSync(path.join(__dirname, '../site/crop-planner.js'), 'utf8');
const html = fs.readFileSync(path.join(__dirname, '../site/crop-planner.html'), 'utf8');
const bytes = Buffer.from('synthetic Wasm bytes for UI loading tests');
const metadata = { schema: 'gooo/browser-crop-planner/v1', wasm_sha256: createHash('sha256').update(bytes).digest('hex'),
  wasm_bytes: bytes.length, source_revision: 'b'.repeat(40), source_dirty: false, compiler_revision: 'c'.repeat(40),
  go_version: 'go1.27.2', generated_sha256: 'd'.repeat(64), execution_sha256: 'e'.repeat(64), replay_sha256: 'f'.repeat(64), fixture_cases: 486 };
const expected = { min_x: '0', min_y: '40', max_x: '200', max_y: '480', has_pixels: true };
const deferred = () => { let resolve, reject; const promise = new Promise((yes, no) => { resolve = yes; reject = no; }); return { promise, resolve, reject }; };
async function until(predicate) {
  for (let attempt = 0; attempt < 200; attempt++) {
    if (predicate()) return;
    await new Promise(resolve => setTimeout(resolve, 2));
  }
  assert.fail('UI did not reach the expected state');
}
class Element {
  constructor() { this.value = ''; this.hidden = false; this.disabled = false; this.textContent = ''; this.attributes = {}; this.listeners = {}; }
  addEventListener(name, fn) { this.listeners[name] = fn; }
  emit(name, event = {}) { return this.listeners[name]?.({ preventDefault() {}, ...event }); }
  setAttribute(name, value) { this.attributes[name] = value; }
  focus() { this.focused = true; }
}
function app(options = {}) {
  const elements = new Map();
  const ids = new Set([...html.matchAll(/\bid="([^"]+)"/g)].map(match => match[1]));
  const get = id => {
    assert.ok(ids.has(id), `app references a missing HTML id: ${id}`);
    if (!elements.has(id)) elements.set(id, new Element());
    return elements.get(id);
  };
  protocol.inputIds.forEach((id, index) => get(id).value = protocol.presets.partial[index]);
  get('crop-run').disabled = true;
  const requests = [], calls = [], instantiated = [];
  const runtime = deferred();
  let reloaded = false;
  const context = {
    GoooCropPlannerProtocol: protocol, Uint8Array, Date, AbortController, console,
    crypto: options.noCrypto ? undefined : webcrypto, setTimeout, clearTimeout,
    document: { getElementById: get }, location: { reload() { reloaded = true; } },
    fetch: async (name, fetchOptions) => {
      requests.push({ name, options: fetchOptions });
      if (options.fetch) return options.fetch(name, fetchOptions);
      if (name === './crop/metadata.json') return { ok: true, json: async () => ({ ...metadata, ...options.metadata }) };
      assert.equal(name, './crop/crop.wasm');
      return { ok: true, arrayBuffer: async () => options.bytes || bytes };
    },
    WebAssembly: { async instantiate(program) { instantiated.push(Buffer.from(program)); return { instance: {} }; } },
    Go: class {
      constructor() { this.importObject = {}; }
      run() {
        context.goooCropReady = true;
        context.goooCrop = (...inputs) => {
          calls.push(inputs);
          return options.reply ? options.reply(inputs) : JSON.stringify({ ok: true, inputs, output: expected });
        };
        return runtime.promise;
      }
    }
  };
  context.window = context;
  vm.runInNewContext(source, context, { filename: 'site/crop-planner.js' });
  return { get, calls, requests, instantiated, runtime,
    ready: () => until(() => !get('crop-run').disabled),
    failed: () => until(() => get('crop-runtime').className === 'crop-runtime error'),
    run: () => get('crop-form').emit('submit'),
    edit(index, value) { get(protocol.inputIds[index]).value = value; get(protocol.inputIds[index]).emit('input'); },
    preset(key) { get('crop-preset').value = key; get('crop-preset').emit('change'); },
    get reloaded() { return reloaded; }
  };
}

test('loading checks exact bytes before enabling execution and never submits coordinates during loading', async () => {
  const a = app(); await a.ready();
  assert.equal(a.calls.length, 0);
  assert.deepEqual(a.instantiated, [bytes]);
  assert.deepEqual(a.requests.map(request => request.name), ['./crop/metadata.json', './crop/crop.wasm']);
  assert.ok(a.requests.every(request => request.options.cache === 'no-store'));
  assert.equal(a.get('crop-source-link').href, `https://github.com/kimjooyoon/gooo-go-ports/blob/${metadata.source_revision}/examples/crop-planner/crop_planner.gooo`);
  a.run();
  assert.deepEqual(a.calls, [[...protocol.presets.partial]]);
  assert.equal(a.get('result-max-y').textContent, '480');
  assert.equal(a.get('result-has-pixels').textContent, 'true');
  assert.equal(a.get('crop-result-badge').textContent, 'EXECUTED');
});

test('all eight edited coordinates invalidate visible and raw results; malformed input causes no execution', async () => {
  const a = app(); await a.ready();
  for (let index = 0; index < 8; index++) {
    a.preset('partial'); a.run();
    assert.equal(a.get('crop-result-values').hidden, false);
    a.edit(index, '9223372036854775808');
    assert.equal(a.get('crop-result-values').hidden, true);
    assert.equal(a.get('crop-raw-result').hidden, true);
    assert.equal(a.get('crop-raw-output').textContent, '');
    assert.equal(a.get('result-max-y').textContent, '');
    const calls = a.calls.length;
    a.run();
    assert.equal(a.calls.length, calls);
    assert.equal(a.get(protocol.inputIds[index]).attributes['aria-invalid'], 'true');
    assert.equal(a.get('crop-result-badge').textContent, 'NOT RUN');
  }
});

test('exact large coordinates survive preset, runtime call, rendering and raw JSON', async () => {
  const value = { min_x: '9007199254740993', min_y: '-9007199254740992', max_x: '9007199254740994', max_y: '-9007199254740991', has_pixels: true };
  const a = app({ reply: inputs => JSON.stringify({ ok: true, inputs, output: value }) });
  await a.ready(); a.preset('precise');
  assert.equal(a.calls.length, 0);
  a.run();
  assert.deepEqual(a.calls, [[...protocol.presets.precise]]);
  assert.equal(a.get('result-min-x').textContent, '9007199254740993');
  assert.deepEqual(JSON.parse(a.get('crop-raw-output').textContent).output, value);
  a.preset('edge');
  assert.equal(a.get('crop-result-values').hidden, true);
  assert.equal(a.calls.length, 1, 'choosing a preset must not execute it');
});

test('mismatched or malformed runtime replies clear the last accepted result', async () => {
  let bad = false;
  const a = app({ reply: inputs => JSON.stringify({ ok: true, inputs: bad ? [...inputs].reverse() : inputs, output: expected }) });
  await a.ready(); a.run();
  assert.equal(a.get('crop-result-values').hidden, false);
  bad = true; a.run();
  assert.equal(a.get('crop-result-values').hidden, true);
  assert.equal(a.get('crop-raw-output').textContent, '');
  assert.match(a.get('crop-error').textContent, /original eight/);
});

test('wrong hashes, sizes, schemas or unavailable integrity checks never instantiate or execute', async () => {
  for (const options of [
    { metadata: { wasm_sha256: '0'.repeat(64) } }, { bytes: Buffer.from('different size') },
    { metadata: { schema: 'gooo/browser-demo/v2' } }, { noCrypto: true },
    { fetch: async () => ({ ok: false, status: 404 }) }
  ]) {
    const a = app(options); await a.failed();
    assert.equal(a.get('crop-run').disabled, true);
    assert.equal(a.get('crop-retry').hidden, false);
    assert.equal(a.instantiated.length, 0);
    a.run(); assert.equal(a.calls.length, 0);
    a.get('crop-retry').emit('click'); assert.equal(a.reloaded, true);
  }
});

test('runtime termination clears an accepted result and disables future calls', async () => {
  const a = app(); await a.ready(); a.run();
  a.runtime.resolve(); await a.failed();
  assert.equal(a.get('crop-result-values').hidden, true);
  assert.equal(a.get('crop-raw-output').textContent, '');
  assert.equal(a.get('crop-program-hash').textContent, '');
  assert.equal(a.get('crop-run').disabled, true);
  const count = a.calls.length; a.run(); assert.equal(a.calls.length, count);
});

test('dirty builds link to the bundled exact source instead of a clean commit', async () => {
  const a = app({ metadata: { source_dirty: true } }); await a.ready();
  assert.equal(a.get('crop-source-link').href, './crop/source/examples/crop-planner/crop_planner.gooo');
  assert.equal(a.get('crop-port-link').href, './crop/source/ports/image/geometry.gooo');
  assert.match(a.get('crop-program').textContent, /local changes/);
});
