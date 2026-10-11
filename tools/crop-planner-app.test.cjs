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
const dragBytes = Buffer.from('different synthetic DragCropPlan Wasm bytes');
const metadata = { schema: 'gooo/browser-crop-planner/v1', entry_id: protocol.modes.ordered.entryId, wasm_sha256: createHash('sha256').update(bytes).digest('hex'),
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
  const requests = [], calls = [], modeCalls = [], instantiated = [], started = [];
  const runtimes = { ordered: deferred(), corners: deferred() };
  let reloaded = false;
  function publish(mode) {
    const config = protocol.modeFor(mode);
    context[config.readyName] = true;
    context[config.runtimeName] = (...inputs) => {
      calls.push(inputs);
      modeCalls.push({ mode, inputs });
      return options.reply ? options.reply(inputs, mode) : JSON.stringify({ ok: true, inputs, output: expected });
    };
  }
  async function defaultFetch(name) {
    const mode = name.startsWith('./drag-crop/') ? 'corners' : 'ordered';
    const config = protocol.modeFor(mode);
    const program = mode === 'ordered' ? bytes : dragBytes;
    if (name === config.assetBase + 'metadata.json') return { ok: true, json: async () => ({ ...metadata,
      entry_id: config.entryId, wasm_sha256: createHash('sha256').update(program).digest('hex'),
      wasm_bytes: program.length, ...options.metadata, ...options.modeMetadata?.[mode] }) };
    assert.equal(name, config.assetBase + 'crop.wasm');
    return { ok: true, arrayBuffer: async () => options.bytes || program };
  }
  const context = {
    GoooCropPlannerProtocol: protocol, Uint8Array, Date, AbortController, console,
    crypto: options.noCrypto ? undefined : webcrypto,
    setTimeout: (fn, delay) => setTimeout(fn, delay === 30000 && options.timeoutMs ? options.timeoutMs : delay), clearTimeout,
    document: { getElementById: get }, location: { reload() { reloaded = true; } },
    fetch: async (name, fetchOptions) => {
      requests.push({ name, options: fetchOptions });
      return options.fetch ? options.fetch(name, fetchOptions, defaultFetch) : defaultFetch(name);
    },
    WebAssembly: { async instantiate(program) {
      const mode = Buffer.from(program).equals(dragBytes) ? 'corners' : 'ordered';
      instantiated.push(Buffer.from(program));
      if (options.instantiate) await options.instantiate(mode);
      return { instance: { mode } };
    } },
    Go: class {
      constructor() { this.importObject = {}; }
      run(instance) {
        started.push(instance.mode);
        if (!options.deferReady?.includes(instance.mode)) publish(instance.mode);
        return runtimes[instance.mode].promise;
      }
    }
  };
  context.window = context;
  vm.runInNewContext(source, context, { filename: 'site/crop-planner.js' });
  return { get, calls, modeCalls, requests, instantiated, started, runtimes, runtime: runtimes.ordered, publish,
    ready: () => until(() => !get('crop-run').disabled),
    failed: () => until(() => get('crop-runtime').className === 'crop-runtime error'),
    run: () => get('crop-form').emit('submit'),
    edit(index, value) { get(protocol.inputIds[index]).value = value; get(protocol.inputIds[index]).emit('input'); },
    preset(key) { get('crop-preset').value = key; get('crop-preset').emit('change'); },
    mode(key) { get('crop-mode').value = key; get('crop-mode').emit('change'); },
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

test('mode switch clears stale results, relabels only selection and calls the actual selected runtime', async () => {
  const zero = { min_x: '0', min_y: '0', max_x: '0', max_y: '0', has_pixels: false };
  const selection = { min_x: '10', min_y: '20', max_x: '90', max_y: '80', has_pixels: true };
  const a = app({ reply: (inputs, mode) => JSON.stringify({ ok: true, inputs, output: mode === 'corners' ? selection : zero }) });
  await a.ready();
  assert.equal(a.get('crop-mode').value, 'ordered');
  a.preset('reversedCrop'); a.run();
  assert.equal(a.get('result-has-pixels').textContent, 'false');
  a.mode('corners');
  assert.equal(a.get('crop-result-values').hidden, true);
  assert.equal(a.get('crop-raw-output').textContent, '');
  assert.equal(a.get('crop-run').disabled, true);
  assert.equal(a.get('crop-label-min-x').textContent, 'First corner x');
  assert.equal(a.get('crop-label-max-y').textContent, 'Second corner y');
  assert.match(a.get('crop-mode-help').textContent, /Bounds remain unchanged/);
  assert.match(a.get('crop-mode-help').textContent, /\(200,200\) to \(100,100\)/);
  assert.equal(a.calls.length, 1, 'switching modes must not execute input');
  await a.ready(); a.run();
  assert.equal(a.get('result-has-pixels').textContent, 'true');
  assert.deepEqual(a.modeCalls, [
    { mode: 'ordered', inputs: [...protocol.presets.reversedCrop] },
    { mode: 'corners', inputs: [...protocol.presets.reversedCrop] }
  ]);
  const raw = JSON.parse(a.get('crop-raw-output').textContent);
  assert.equal(raw.mode, 'corners');
  assert.equal(raw.entry_id, protocol.modes.corners.entryId);
  assert.deepEqual(raw.inputs, protocol.presets.reversedCrop);
  assert.equal(a.get('crop-generated-link').href, './drag-crop/generated.go');
  assert.equal(a.get('crop-metadata-link').href, './drag-crop/metadata.json');
  assert.equal(a.get('crop-replay-link').href, './drag-crop/replay.json');
  assert.equal(a.get('crop-source-link').href, `https://github.com/kimjooyoon/gooo-go-ports/blob/${metadata.source_revision}/examples/crop-planner/drag_crop.gooo`);
  a.mode('ordered');
  assert.equal(a.get('crop-result-values').hidden, true);
  assert.equal(a.get('crop-run').disabled, false);
  assert.equal(a.get('crop-label-min-x').textContent, 'min_x');
  assert.equal(a.get('crop-label-max-y').textContent, 'max_y');
  assert.equal(a.get('crop-generated-link').href, './crop/generated.go');
  assert.equal(a.instantiated.length, 2, 'loaded runtimes are reused');
  assert.ok(a.requests.every(request => request.options.cache === 'no-store'));
  a.run();
  assert.equal(JSON.parse(a.get('crop-raw-output').textContent).entry_id, protocol.modes.ordered.entryId);
});

test('two-corner mode preserves precision and reversed bounds exactly; editing clears results', async () => {
  const value = { min_x: '9007199254740993', min_y: '-9007199254740992', max_x: '9007199254740994', max_y: '-9007199254740991', has_pixels: true };
  const a = app({ reply: inputs => JSON.stringify({ ok: true, inputs, output: value }) });
  await a.ready(); a.mode('corners'); await a.ready(); a.preset('precise'); a.run();
  assert.equal(a.get('result-min-x').textContent, '9007199254740993');
  assert.deepEqual(a.modeCalls[0], { mode: 'corners', inputs: [...protocol.presets.precise] });
  for (let i = 0; i < 8; i++) {
    a.run(); a.edit(i, protocol.presets.precise[i]);
    assert.equal(a.get('crop-result-values').hidden, true);
    assert.equal(a.get('crop-raw-output').textContent, '');
  }
  a.preset('reversedBounds'); a.run();
  assert.deepEqual(a.calls.at(-1), [...protocol.presets.reversedBounds]);
  a.edit(4, '1e2'); const count = a.calls.length; a.run();
  assert.equal(a.calls.length, count);
  assert.match(a.get('crop-error').textContent, /^First corner x/);
});

test('metadata from the other mode fails before its Wasm is fetched or instantiated', async () => {
  const a = app({ modeMetadata: { corners: { entry_id: protocol.modes.ordered.entryId } } });
  await a.ready(); a.run(); a.mode('corners'); await a.failed();
  assert.equal(a.get('crop-result-values').hidden, true);
  assert.equal(a.get('crop-run').disabled, true);
  assert.equal(a.instantiated.length, 1);
  assert.ok(!a.requests.some(request => request.name === './drag-crop/crop.wasm'));
  a.run(); assert.equal(a.calls.length, 1);
  a.mode('ordered'); await a.ready(); a.run();
  assert.equal(a.calls.length, 2, 'failure in one mode does not poison the other');
});

test('dirty source and guide links stay inside the selected artifact snapshot', async () => {
  const a = app({ metadata: { source_dirty: true } });
  await a.ready();
  assert.equal(a.get('crop-native-guide').href, './crop/source/examples/crop-planner/README.md');
  a.mode('corners'); await a.ready();
  assert.equal(a.get('crop-source-link').href, './drag-crop/source/examples/crop-planner/drag_crop.gooo');
  assert.equal(a.get('crop-port-link').href, './drag-crop/source/ports/image/canon.gooo');
  assert.equal(a.get('crop-native-guide').href, './drag-crop/source/docs/drag-crop.md');
  assert.match(a.get('crop-program').textContent, /DragCropPlan.*local changes/);
});

test('switching during a metadata fetch aborts it and ignores late cancelled completion', async () => {
  const pending = deferred(); let originalFetch;
  const a = app({ fetch: (name, options, fallback) => {
    if (name === './crop/metadata.json' && !originalFetch) {
      originalFetch = { options, fallback };
      return pending.promise;
    }
    return fallback(name);
  } });
  a.mode('corners');
  assert.equal(originalFetch.options.signal.aborted, true);
  await a.ready(); a.run();
  pending.resolve(await originalFetch.fallback('./crop/metadata.json'));
  await new Promise(resolve => setTimeout(resolve, 10));
  assert.deepEqual(a.started, ['corners']);
  assert.equal(a.get('crop-mode').value, 'corners');
  assert.equal(a.get('crop-result-values').hidden, false);
  assert.equal(a.get('crop-source-link').textContent, 'DragCropPlan source');
  a.mode('ordered'); await a.ready();
  assert.deepEqual(a.started, ['corners', 'ordered']);
  assert.equal(a.requests.filter(request => request.name === './crop/metadata.json').length, 2);
});

test('cancelled instantiation cannot start a runtime or overwrite the current mode', async () => {
  const pending = deferred();
  const a = app({ instantiate: mode => mode === 'ordered' ? pending.promise : undefined });
  await until(() => a.instantiated.length === 1);
  a.mode('corners'); await a.ready(); a.run();
  pending.resolve(); await new Promise(resolve => setTimeout(resolve, 10));
  assert.deepEqual(a.started, ['corners']);
  assert.equal(a.get('crop-result-values').hidden, false);
  assert.equal(JSON.parse(a.get('crop-raw-output').textContent).mode, 'corners');
});

test('started but not ready runtimes stay separate during rapid mode switches', async () => {
  const a = app({ deferReady: ['ordered'] });
  await until(() => a.started.includes('ordered'));
  a.mode('corners'); a.run(); assert.equal(a.calls.length, 0);
  await a.ready(); a.run();
  a.publish('ordered'); await new Promise(resolve => setTimeout(resolve, 35));
  assert.equal(a.get('crop-source-link').textContent, 'DragCropPlan source');
  assert.equal(a.get('crop-result-values').hidden, false);
  a.mode('ordered'); await a.ready(); a.run();
  assert.deepEqual(a.modeCalls.map(call => call.mode), ['corners', 'ordered']);
  assert.equal(a.instantiated.length, 2);
});

test('inactive runtime termination cannot clear the current mode result or enable a failed mode', async () => {
  const a = app(); await a.ready(); a.mode('corners'); await a.ready(); a.run();
  a.runtime.reject(new Error('ordered stopped'));
  await new Promise(resolve => setTimeout(resolve, 10));
  assert.equal(a.get('crop-result-values').hidden, false);
  assert.equal(a.get('crop-run').disabled, false);
  assert.equal(JSON.parse(a.get('crop-raw-output').textContent).mode, 'corners');
  a.mode('ordered'); await a.failed();
  assert.equal(a.get('crop-run').disabled, true);
  assert.equal(a.get('crop-result-values').hidden, true);
  a.mode('corners'); await a.ready(); a.run();
  assert.equal(a.get('crop-result-values').hidden, false);
  a.runtimes.corners.resolve(); await a.failed();
  assert.equal(a.get('crop-result-values').hidden, true);
});

test('timeout aborts loading and late bytes never start a program', async () => {
  const pending = deferred(); let fetchOptions; let fallbackFetch;
  const a = app({ timeoutMs: 20, fetch: (name, options, fallback) => {
    fetchOptions = options; fallbackFetch = fallback; return pending.promise;
  } });
  await a.failed();
  assert.equal(fetchOptions.signal.aborted, true);
  pending.resolve(await fallbackFetch('./crop/metadata.json'));
  await new Promise(resolve => setTimeout(resolve, 10));
  assert.equal(a.instantiated.length, 0);
  assert.equal(a.started.length, 0);
  assert.equal(a.get('crop-run').disabled, true);
  a.run(); assert.equal(a.calls.length, 0);
});
