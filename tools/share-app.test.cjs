// Exercise the shipped app's async wiring without a browser or DOM dependency.
// The tiny DOM and runtime doubles count calls; browser-smoke.cjs separately
// runs every shared reference input through the actual Go WebAssembly program.
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { webcrypto, createHash } = require('node:crypto');
const share = require('../site/share.js');
const codepoint = require('../site/codepoint.js');
const source = fs.readFileSync(path.join(__dirname, '../site/app.js'), 'utf8');
const bytes = Buffer.from('fixture program bytes for app scheduling tests');
const wasm = createHash('sha256').update(bytes).digest('hex');
const revision = '2'.repeat(40);
const metadata = { wasm_sha256: wasm, source_revision: revision, source_dirty: false,
  compiler_revision: '1'.repeat(40), compiler_version: '0.6.24-dev', go_version: 'go1.27.2' };
const link = (options = {}) => share.format({ operation: 'http.StatusText',
  inputs: ['9223372036854775807', '0'], wasm, revision, ...options });
const deferred = () => { let resolve, reject; const promise = new Promise((a, b) => { resolve = a; reject = b; }); return { promise, resolve, reject }; };
async function until(predicate) {
  for (let i = 0; i < 200; i++) {
    if (predicate()) return;
    await new Promise(resolve => setTimeout(resolve, 2));
  }
  assert.fail('app did not reach the expected asynchronous state');
}
class Element {
  constructor() {
    this.value = ''; this.hidden = false; this.disabled = false; this.textContent = '';
    this.dataset = {}; this.children = []; this.listeners = new Map();
    this.style = {}; this.attributes = {};
    this.firstChild = { textContent: '' }; this.classList = { toggle() {} };
  }
  addEventListener(name, fn) { this.listeners.set(name, fn); }
  emit(name, event = {}) { return this.listeners.get(name)?.({ preventDefault() {}, ...event }); }
  replaceChildren(...children) { this.children = children; }
  append(...children) { this.children.push(...children); }
  setAttribute(name, value) { this.attributes[name] = value; }
  focus() { this.focused = true; }
  select() { this.selected = true; }
}
const rgbOutput = { type: 'rgb', value: { red: '63', green: '128', blue: '192' } };
const cmykOutput = { type: 'cmyk', value: { cyan: '170', magenta: '85', yellow: '0', black: '63' } };
function app(fragment = '', clipboard = async () => {}, runtimeReply) {
  const elements = new Map();
  const get = id => { if (!elements.has(id)) elements.set(id, new Element()); return elements.get(id); };
  get('operation').value = 'http.StatusText'; get('first-input').value = '404'; get('second-input').value = '56832';
  const scenarioButtons = ['http', 'unicode', 'bytes', 'colors', 'duration'].map(scenario => { const e = new Element(); e.dataset.scenario = scenario; return e; });
  const events = new Map(), calls = [], copied = [];
  const metaGate = deferred(), wasmGate = deferred(), fixtureGate = deferred();
  let url = new URL('https://example.test/demo/' + fragment);
  const context = {
    URL, URLSearchParams, Uint8Array, Date, setTimeout, console, crypto: webcrypto, GoooShare: share, GoooCodePoint: codepoint,
    document: { getElementById: get, createElement: () => new Element(), createTextNode: text => ({ textContent: text }), querySelectorAll: () => scenarioButtons },
    location: { get href() { return url.href; }, get hash() { return url.hash; } },
    history: { replaceState(_state, _title, href) { url = new URL(href); } },
    addEventListener: (name, fn) => events.set(name, fn),
    navigator: { clipboard: { async writeText(value) { copied.push(value); return clipboard(value); } } },
    WebAssembly: { async instantiate() { return { instance: {} }; } },
    fetch: name => {
      if (name === './metadata.json') return metaGate.promise;
      if (name === './demo.wasm') return wasmGate.promise;
      if (name === './fixtures.json') return fixtureGate.promise;
      throw new Error('unexpected fetch: ' + name);
    },
    Go: class {
      constructor() { this.importObject = {}; }
      run() {
        context.goooReady = true;
        context.goooRun = (operation, ...inputs) => {
          calls.push([operation, ...inputs]);
          const output = operation === 'color.RGBToCMYK' ? cmykOutput : operation === 'color.CMYKToRGB' ? rgbOutput : { type: 'text', value: 'runtime test result' };
          return JSON.stringify(runtimeReply ? runtimeReply(operation, inputs) : { ok: true, operation, inputs, output });
        };
        return new Promise(() => {});
      }
    }
  };
  context.window = context;
  vm.runInNewContext(source, context, { filename: 'site/app.js' });
  const loadMeta = (value = metadata) => metaGate.resolve({ ok: true, json: async () => value });
  const loadWasm = () => wasmGate.resolve({ ok: true, arrayBuffer: async () => bytes });
  return { get, calls, copied, scenarioButtons, loadMeta, loadWasm,
    async ready(value = metadata) { loadMeta(value); loadWasm(); await until(() => get('runtime-message').textContent === 'WebAssembly ready'); await new Promise(r => setTimeout(r, 0)); },
    hash(value) { url.hash = value; events.get('hashchange')(); },
    metaFail() { metaGate.resolve({ ok: false }); },
    finishChecks(fixtures = { schema: 'gooo/browser-fixtures/v2', cases: [{ operation: 'bits.Reverse8', inputs: ['170', '0'], expected: { type: 'text', value: 'runtime test result' } }], rejections: [] }) { fixtureGate.resolve({ ok: true, json: async () => fixtures }); }
  };
}

test('same program executes exact shared inputs once with either readiness order', async () => {
  for (const first of ['metadata', 'runtime']) {
    const a = app(link());
    assert.equal(a.get('first-input').value, '9223372036854775807');
    if (first === 'metadata') { a.loadMeta(); await new Promise(r => setTimeout(r, 0)); }
    else { a.loadWasm(); await until(() => a.get('runtime-message').textContent === 'WebAssembly ready'); }
    assert.equal(a.calls.length, 0);
    await a.ready();
    assert.deepEqual(a.calls, [['http.StatusText', '9223372036854775807', '0']]);
    a.hash(link());
    assert.equal(a.calls.length, 1, 'duplicate hash event executed a second time');
  }
});

test('a changed source revision with identical Wasm still executes once', async () => {
  const a = app(link({ revision: '3'.repeat(40), inputs: ['9007199254740993', '0'] }));
  await a.ready();
  assert.deepEqual(a.calls, [['http.StatusText', '9007199254740993', '0']]);
});

test('different program waits for explicit Run and then updates its notice', async () => {
  const a = app(link({ wasm: 'f'.repeat(64), inputs: ['-9223372036854775808', '0'] }));
  await a.ready();
  assert.equal(a.calls.length, 0);
  assert.match(a.get('share-status').textContent, /different version/);
  assert.equal(a.get('run-button').disabled, false);
  a.get('run-form').emit('submit');
  assert.deepEqual(a.calls, [['http.StatusText', '-9223372036854775808', '0']]);
  assert.match(a.get('share-status').textContent, /ran with the current program/);
  assert.doesNotMatch(a.get('share-status').textContent, /press Run/);
});

test('metadata must match actual fetched program bytes before any auto-run or share', async () => {
  const a = app(link());
  await a.ready({ ...metadata, wasm_sha256: 'f'.repeat(64) });
  assert.equal(a.calls.length, 0);
  assert.equal(a.get('share-button').disabled, true);
  a.get('run-form').emit('submit');
  assert.equal(a.calls.length, 1);
  const b = app(link());
  b.metaFail(); b.loadWasm();
  await until(() => b.get('runtime-message').textContent === 'WebAssembly ready');
  assert.equal(b.calls.length, 0);
  b.get('run-form').emit('submit');
  assert.equal(b.calls.length, 1);
});

test('malformed shared links block even synthetic form submission until explicitly reset', async () => {
  for (const fragment of [link() + '&a=1', link().replace('v=1', 'v=2'), link() + '&result=trusted', link({ inputs: ['0', '0'] }).replace('a=0', 'a=2147483648').replace('http.StatusText', 'utf8.ValidRune')]) {
    const a = app(fragment); await a.ready();
    assert.equal(a.get('run-button').disabled, true);
    assert.equal(a.get('share-button').disabled, true);
    assert.equal(a.get('share-reset').hidden, false);
    a.get('run-form').emit('submit');
    assert.equal(a.calls.length, 0);
    a.get('share-reset').emit('click');
    a.get('run-form').emit('submit');
    assert.deepEqual(a.calls, [['http.StatusText', '404', '0']]);
  }
});

test('editing restored input or selecting an example cancels pending automatic execution', async () => {
  for (const edit of [a => { a.get('first-input').value = '9007199254740992'; a.get('first-input').emit('input'); }, a => a.scenarioButtons[2].emit('click')]) {
    const a = app(link()); edit(a); await a.ready();
    assert.equal(a.calls.length, 0);
    a.get('run-form').emit('submit'); assert.equal(a.calls.length, 1);
  }
});

test('manual Run before metadata arrives consumes the pending shared request', async () => {
  const a = app(link()); a.loadWasm();
  await until(() => a.get('runtime-message').textContent === 'WebAssembly ready');
  a.get('run-form').emit('submit');
  a.loadMeta(); await new Promise(r => setTimeout(r, 5));
  assert.equal(a.calls.length, 1);
});

test('ordinary anchors keep the default input and never automatically execute', async () => {
  const a = app('#workbench'); await a.ready();
  assert.equal(a.get('first-input').value, '404'); assert.equal(a.calls.length, 0);
  a.hash('#checks-title'); assert.equal(a.calls.length, 0);
});

test('a new shared hash during reference checks waits, then executes once', async () => {
  const a = app(); await a.ready();
  const checking = a.get('checks-button').emit('click');
  a.hash(link({ operation: 'utf16.DecodeRune', inputs: ['65533', '65533'] }));
  assert.equal(a.calls.length, 0);
  a.finishChecks(); await checking;
  assert.deepEqual(a.calls, [['bits.Reverse8', '170', '0'], ['utf16.DecodeRune', '65533', '65533']]);
});

test('copy preserves exact values and program identity; rejected clipboard exposes a selectable URL', async () => {
  for (const unavailable of [false, true]) {
    const a = app(link(), async () => { if (unavailable) throw new Error('denied'); }); await a.ready();
    await a.get('share-button').emit('click');
    const value = share.parse(new URL(a.copied[0]).hash);
    assert.deepEqual(value, { operation: 'http.StatusText', inputs: ['9223372036854775807', '0'], wasm, revision });
    assert.equal(a.get('share-fallback').hidden, !unavailable);
    if (unavailable) { assert.equal(a.get('share-url').value, a.copied[0]); assert.equal(a.get('share-url').selected, true); }
  }
});

test('late clipboard failure cannot restore a stale URL after inputs change', async () => {
  const clipboard = deferred(); const a = app(link(), () => clipboard.promise); await a.ready();
  const copying = a.get('share-button').emit('click');
  a.get('first-input').value = '404'; a.get('first-input').emit('input');
  clipboard.reject(new Error('denied')); await copying;
  assert.equal(a.get('share-fallback').hidden, true);
  assert.equal(a.get('share-status').hidden, true);
});

const inputNames = ['first', 'second', 'third', 'fourth'];
const colorCases = [
  { operation: 'color.RGBToCMYK', inputs: ['64', '128', '192'], labels: ['Red', 'Green', 'Blue'], output: cmykOutput },
  { operation: 'color.CMYKToRGB', inputs: ['170', '85', '0', '63'], labels: ['Cyan', 'Magenta', 'Yellow', 'Black'], output: rgbOutput }
];
function select(a, operation) { a.get('operation').value = operation; a.get('operation').emit('change'); }
function enter(a, inputs) { inputs.forEach((value, i) => { a.get(inputNames[i] + '-input').value = value; }); }

test('v2 restores every labeled color channel, runs exact arity, and renders named string results', async () => {
  for (const item of colorCases) {
    const a = app(link({ operation: item.operation, inputs: item.inputs }));
    inputNames.forEach((name, i) => {
      assert.equal(a.get(name + '-input').value, item.inputs[i] || '0');
      assert.equal(a.get(name + '-field').hidden, i >= item.inputs.length);
      assert.equal(a.get(name + '-input').disabled, i >= item.inputs.length);
      assert.equal(a.get(name + '-label').textContent, item.labels[i] || '');
    });
    await a.ready();
    assert.deepEqual(a.calls, [[item.operation, ...item.inputs]]);
    assert.equal(a.get('result-badge').textContent, 'EXECUTED');
    assert.deepEqual(JSON.parse(a.get('raw-output').textContent).output, item.output);
    const parts = a.get('result-content').children.find(e => e.className === 'pair-result').children;
    assert.deepEqual(parts.map(e => e.children.map(c => c.textContent)), Object.entries(item.output.value));
    a.hash(link({ operation: item.operation, inputs: item.inputs })); assert.equal(a.calls.length, 1);
    select(a, 'http.StatusText'); a.get('run-form').emit('submit');
    assert.deepEqual(a.calls[1], ['http.StatusText', '404', '0']);
    assert.equal(a.get('third-field').hidden, true); assert.equal(a.get('fourth-field').hidden, true);
    a.scenarioButtons[3].emit('click');
    assert.equal(a.get('operation').value, 'color.RGBToCMYK');
  }
});

test('v2 mismatched program restores all channels but requires manual Run', async () => {
  for (const item of colorCases) {
    const a = app(link({ operation: item.operation, inputs: item.inputs, wasm: 'f'.repeat(64) })); await a.ready();
    assert.equal(a.calls.length, 0);
    assert.match(a.get('share-status').textContent, /different version/);
    a.get('run-form').emit('submit');
    assert.deepEqual(a.calls, [[item.operation, ...item.inputs]]);
  }
});

test('every color field rejects invalid input before execution and clears earlier results', async () => {
  for (const item of colorCases) {
    const a = app(); await a.ready(); select(a, item.operation);
    a.get('run-form').emit('submit'); assert.equal(a.calls.length, 1);
    for (let i = 0; i < item.inputs.length; i++) {
      for (const invalid of ['-1', '256', '9007199254740993', '-0', '+1', '01', '1.0', '', ' 1']) {
        enter(a, item.inputs); a.get(inputNames[i] + '-input').value = invalid;
        a.get(inputNames[i] + '-input').emit('input'); a.get('run-form').emit('submit');
        assert.equal(a.calls.length, 1, `${item.operation} channel ${i} accepted ${invalid}`);
        assert.equal(a.get('result-badge').textContent, 'ERROR');
        assert.equal(a.get('raw-result').hidden, true);
        assert.equal(a.get('result-content').children.some(e => e.className === 'color-swatch'), false);
      }
    }
    for (const bad of [link({ operation: item.operation, inputs: item.inputs }).replace('v=2', 'v=1'), link({ operation: item.operation, inputs: item.inputs }) + '&c=1', link({ operation: item.operation, inputs: item.inputs }).replace('c=' + item.inputs[2], 'c=256')]) {
      a.hash(bad); a.get('run-form').emit('submit'); assert.equal(a.calls.length, 1);
      assert.equal(a.get('run-button').disabled, true);
    }
  }
});

test('color result records require every named canonical byte string and the expected output kind', async () => {
  for (const item of colorCases) {
    let output = item.output;
    const a = app('', undefined, (operation, inputs) => ({ ok: true, operation, inputs, output }));
    await a.ready(); select(a, item.operation);
    const invalidOutputs = [{ type: 'text', value: 'wrong kind' }, { ...item.output, value: null }, { ...item.output, value: [] }];
    for (const key of Object.keys(item.output.value)) {
      const missing = { ...item.output.value }; delete missing[key];
      invalidOutputs.push({ ...item.output, value: missing });
      for (const value of ['-1', '256', '00', '-0', '1.0', '1e2', '1\n', 0, null]) invalidOutputs.push({ ...item.output, value: { ...item.output.value, [key]: value } });
    }
    invalidOutputs.push({ ...item.output, value: { ...item.output.value, extra: '0' } });
    for (const invalid of invalidOutputs) {
      output = invalid; a.get('run-form').emit('submit');
      assert.equal(a.get('result-badge').textContent, 'ERROR', JSON.stringify(invalid));
      assert.equal(a.get('raw-result').hidden, true);
    }
    output = item.output; a.get('run-form').emit('submit'); assert.equal(a.get('result-badge').textContent, 'EXECUTED');
  }
});

test('RGB previews use actual input or returned RGB, including black and white', async () => {
  for (const value of ['0', '255']) {
    for (const item of colorCases) {
      const rgb = { red: value, green: value, blue: value };
      const a = app('', undefined, (operation, inputs) => ({ ok: true, operation, inputs, output: operation === 'color.CMYKToRGB' ? { type: 'rgb', value: rgb } : cmykOutput }));
      await a.ready(); select(a, item.operation);
      // CMYK inputs deliberately differ from the mock's RGB result: the UI must not recalculate it.
      enter(a, item.operation === 'color.RGBToCMYK' ? [value, value, value] : ['123', '45', '67', '89']);
      a.get('run-form').emit('submit');
      const swatch = a.get('result-content').children.find(e => e.className === 'color-swatch');
      assert.equal(swatch.style.backgroundColor, `rgb(${value}, ${value}, ${value})`);
      assert.equal(swatch.attributes.role, 'img');
      assert.equal(swatch.attributes['aria-label'], `${item.operation === 'color.RGBToCMYK' ? 'Input' : 'Returned'} RGB: ${value}, ${value}, ${value}`);
    }
  }
});

test('v2 clipboard fallback preserves all channels and extra-field edits cancel stale work', async () => {
  for (const item of colorCases) {
    const gate = deferred(); const a = app(link({ operation: item.operation, inputs: item.inputs }), () => gate.promise);
    const last = inputNames[item.inputs.length - 1] + '-input';
    a.get(last).emit('input'); await a.ready(); assert.equal(a.calls.length, 0);
    const copying = a.get('share-button').emit('click');
    assert.deepEqual(share.parse(new URL(a.copied[0]).hash).inputs, item.inputs);
    a.get(last).value = '1'; a.get(last).emit('input');
    gate.reject(new Error('denied')); await copying;
    assert.equal(a.get('share-fallback').hidden, true);
    const b = app(link({ operation: item.operation, inputs: item.inputs }), async () => { throw new Error('denied'); }); await b.ready();
    await b.get('share-button').emit('click');
    assert.equal(b.get('share-fallback').hidden, false);
    assert.deepEqual(share.parse(new URL(b.get('share-url').value).hash).inputs, item.inputs);
    assert.equal(b.get('share-url').selected, true);
  }
});

test('reference checks compare every color channel and pass intentional bad arities to the runtime', async () => {
  for (const item of colorCases) {
    const a = app('', undefined, (operation, inputs) => inputs.length === item.inputs.length
      ? { ok: true, operation, inputs, output: item.output } : { ok: false, error: 'Invalid input count' });
    await a.ready();
    const mismatches = Object.keys(item.output.value).map(key => ({ ...item.output, value: { ...item.output.value, [key]: '1' } }));
    const cases = [item.output, ...mismatches].map(expected => ({ operation: item.operation, inputs: item.inputs, expected }));
    const checking = a.get('checks-button').emit('click');
    const rejected = [[], ['0'], ['0', '0', '0', '0', '0']];
    a.finishChecks({ schema: 'gooo/browser-fixtures/v2', cases, rejections: rejected.map(inputs => ({ operation: item.operation, inputs })) });
    await checking;
    assert.deepEqual(a.calls, [...cases.map(() => [item.operation, ...item.inputs]), ...rejected.map(inputs => [item.operation, ...inputs])]);
    assert.ok(a.get('checks-status').textContent.startsWith(`1 / ${cases.length} reference cases matched. 3 / 3 invalid-input checks rejected`));
    assert.match(a.get('checks-status').textContent, /First failure/);
  }
});

test('positive color fixture cases cannot silently use the wrong arity', async () => {
  const a = app(); await a.ready();
  const checking = a.get('checks-button').emit('click');
  a.finishChecks({ schema: 'gooo/browser-fixtures/v2', cases: [{ operation: 'color.RGBToCMYK', inputs: ['0', '0'], expected: cmykOutput }], rejections: [] });
  await checking;
  assert.equal(a.calls.length, 0); assert.match(a.get('checks-status').textContent, /incomplete/);
});

test('rotation UI preserves an extreme signed count and shows only the returned byte', async () => {
  const operation = 'bits.RotateLeft8';
  for (const count of ['-9223372036854775808', '-1', '9007199254740992', '9007199254740993', '9223372036854775807']) {
    const a = app(link({ operation, inputs: ['129', count] }), undefined,
      (operation, inputs) => ({ ok: true, operation, inputs, output: { type: 'integer', value: '217' } }));
    await a.ready();
    assert.deepEqual(a.calls, [[operation, '129', count]]);
    assert.equal(a.get('first-label').textContent, 'Byte value');
    assert.equal(a.get('second-label').textContent, 'Rotation count');
    assert.equal(a.get('second-field').hidden, false);
    assert.equal(a.get('second-input').disabled, false);
    assert.equal(a.get('second-input').value, count);
    assert.match(a.get('input-range').textContent, /Byte: 0 to 255/);
    assert.match(a.get('input-range').textContent, /-9223372036854775808 to 9223372036854775807/);
    const preview = a.get('result-content').children.find(child => child.className === 'bit-preview');
    assert.ok(preview);
    assert.equal(preview.children[0].children[1].textContent, '10000001');
    assert.equal(preview.children[1].children[1].textContent, '11011001');
    await a.get('share-button').emit('click');
    assert.deepEqual(share.parse(new URL(a.copied[0]).hash).inputs, ['129', count]);
  }
});

test('rotation UI rejects invalid bytes and counts without invoking the runtime', async () => {
  const a = app();
  await a.ready();
  a.get('operation').value = 'bits.RotateLeft8'; a.get('operation').emit('change');
  for (const inputs of [['-1', '0'], ['256', '0'], ['129', '9223372036854775808'], ['129', '-9223372036854775809'], ['129', '01'], ['129', '-0']]) {
    a.get('first-input').value = inputs[0]; a.get('second-input').value = inputs[1];
    a.get('run-form').emit('submit');
    assert.equal(a.get('result-badge').textContent, 'ERROR');
    assert.equal(a.get('raw-result').hidden, true);
  }
  assert.equal(a.calls.length, 0);
});

test('rotation binary preview rejects malformed and out-of-byte runtime results', async () => {
  for (const value of ['-1', '256', '01', '1.0', '9007199254740993']) {
    const a = app(link({ operation: 'bits.RotateLeft8', inputs: ['129', '1'] }), undefined,
      (operation, inputs) => ({ ok: true, operation, inputs, output: { type: 'integer', value } }));
    await a.ready();
    assert.equal(a.get('result-badge').textContent, 'ERROR');
    assert.equal(a.get('raw-result').hidden, true);
    assert.ok(!a.get('result-content').children.some(child => child.className === 'bit-preview'));
  }
});


test('duration UI preserves exact inputs, reports nanoseconds and only shows the returned integer', async () => {
  for (const operation of ['time.Duration.Round', 'time.Duration.Truncate']) {
    for (const inputs of [['-9223372036854775808', '9223372036854775807'], ['9007199254740993', '2'], ['-9007199254740993', '-1']]) {
      const a = app(link({ operation, inputs }), undefined,
        (operation, inputs) => ({ ok: true, operation, inputs, output: { type: 'integer', value: '217' } }));
      await a.ready();
      assert.deepEqual(a.calls, [[operation, ...inputs]]);
      assert.equal(a.get('first-label').textContent, 'Duration (ns)');
      assert.equal(a.get('first-input').inputMode, 'text');
      assert.equal(a.get('second-input').inputMode, 'text');
      assert.equal(a.get('second-label').textContent, 'Multiple (ns)');
      assert.equal(a.get('second-field').hidden, false);
      assert.equal(a.get('second-input').disabled, false);
      assert.equal(a.get('duration-presets-field').hidden, false);
      assert.equal(a.get('result-content').children[0].textContent, 'NANOSECONDS');
      assert.equal(a.get('result-content').children[1].textContent, '217');
      await a.get('share-button').emit('click');
      assert.deepEqual(share.parse(new URL(a.copied[0]).hash).inputs, inputs);
    }
  }
});

test('duration presets replace both input strings and cancel pending shared auto-run', async () => {
  const presets = { positive: ['1500000000', '1000000000'], negative: ['-1500000000', '1000000000'],
    upper: ['9223372036854775807', '10'], lower: ['-9223372036854775808', '10'],
    precise: ['9007199254740993', '2'], unchanged: ['-9223372036854775808', '0'] };
  for (const [preset, inputs] of Object.entries(presets)) {
    const a = app(link({ operation: 'time.Duration.Truncate', inputs: ['1', '1'] }), undefined,
      (operation, inputs) => ({ ok: true, operation, inputs, output: { type: 'integer', value: '217' } }));
    a.get('duration-presets').value = preset; a.get('duration-presets').emit('change');
    await a.ready();
    assert.equal(a.calls.length, 0);
    assert.equal(a.get('first-input').value, inputs[0]);
    assert.equal(a.get('second-input').value, inputs[1]);
    a.get('run-form').emit('submit');
    assert.deepEqual(a.calls, [['time.Duration.Truncate', ...inputs]]);
  }
});

test('duration inputs reject unsafe or noncanonical values before runtime execution', async () => {
  const a = app(); await a.ready();
  a.scenarioButtons[4].emit('click');
  assert.equal(a.get('operation').value, 'time.Duration.Round');
  for (const position of [0, 1]) {
    for (const value of ['9223372036854775808', '-9223372036854775809', '01', '-0', '1e2']) {
      const inputs = ['9007199254740993', '2']; inputs[position] = value;
      a.get('first-input').value = inputs[0]; a.get('second-input').value = inputs[1];
      a.get('run-form').emit('submit');
      assert.equal(a.get('result-badge').textContent, 'ERROR');
    }
  }
  assert.equal(a.calls.length, 0);
});

test('duration UI rejects malformed runtime results instead of displaying an imprecise duration', async () => {
  for (const value of ['9223372036854775808', '-9223372036854775809', '01', '-0', '1.0', '1e2']) {
    const a = app(link({ operation: 'time.Duration.Round', inputs: ['1', '2'] }), undefined,
      (operation, inputs) => ({ ok: true, operation, inputs, output: { type: 'integer', value } }));
    await a.ready();
    assert.equal(a.get('result-badge').textContent, 'ERROR');
    assert.equal(a.get('raw-result').hidden, true);
  }
});

const spaceReply = (operation, inputs) => ({ ok: true, operation, inputs, output: { type: 'boolean', value: true } });
function pastePoint(a, raw, kind = 'paste') {
  let prevented = false;
  const data = { getData(type) { assert.equal(type, 'text/plain'); return raw; } };
  a.get('codepoint-input').emit(kind, { preventDefault() { prevented = true; }, [kind === 'paste' ? 'clipboardData' : 'dataTransfer']: data });
  assert.equal(prevented, true, 'raw text reached textarea normalization');
}

test('code-point paste keeps spaces, NEL, BOM and supplementary pairs exact without classifying them in JS', async () => {
  const points = [[' ', '32'], ['\t', '9'], ['\n', '10'], ['\u0085', '133'], ['\u00a0', '160'], ['\u200b', '8203'], ['\ufeff', '65279'], ['\u3000', '12288'], ['😀', '128512'], ['\ud800', '55296'], ['\udfff', '57343']];
  for (const [raw, decimal] of points) {
    // Deliberately return true even for BOM/ZWSP/surrogates. The UI must show
    // only the compiled reply and never replace it with a JS whitespace test.
    const a = app('', undefined, spaceReply); await a.ready(); select(a, 'unicode.IsSpace');
    assert.equal(a.get('codepoint-helper').hidden, false);
    assert.equal(a.get('codepoint-input').disabled, false);
    pastePoint(a, raw);
    assert.equal(a.get('first-input').value, decimal);
    assert.equal(a.get('codepoint-input').value, raw);
    assert.equal(a.get('run-button').disabled, false);
    assert.match(a.get('codepoint-status').textContent, new RegExp('decimal ' + decimal));
    assert.equal(a.calls.length, 0, 'input helper executed the function');
    a.get('run-form').emit('submit');
    assert.deepEqual(a.calls, [['unicode.IsSpace', decimal, '0']]);
    assert.equal(JSON.parse(a.get('raw-output').textContent).output.value, true);
    await a.get('share-button').emit('click');
    assert.deepEqual(share.parse(new URL(a.copied[0]).hash).inputs, [decimal, '0']);
  }
});

test('raw CRLF is rejected before textarea normalization; standalone CR remains decimal 13', async () => {
  const a = app('', undefined, spaceReply); await a.ready(); select(a, 'unicode.IsSpace');
  pastePoint(a, '\r');
  assert.equal(a.get('first-input').value, '13');
  assert.equal(a.get('codepoint-input').value, '');
  assert.match(a.get('codepoint-status').textContent, /Carriage return.*decimal 13/);
  a.get('run-form').emit('submit');
  assert.deepEqual(a.calls, [['unicode.IsSpace', '13', '0']]);
  for (const kind of ['paste', 'drop']) {
    pastePoint(a, '\r\n', kind);
    assert.equal(a.get('first-input').value, '13');
    assert.equal(a.get('run-button').disabled, true);
    assert.equal(a.get('share-button').disabled, true);
    assert.match(a.get('codepoint-status').textContent, /exactly one.*CRLF/);
    a.get('run-form').emit('submit');
    assert.equal(a.calls.length, 1, 'invalid helper reused the prior canonical value');
    assert.equal(a.get('raw-result').hidden, true);
  }
  a.get('first-input').value = '13'; a.get('first-input').emit('input');
  a.get('run-form').emit('submit');
  assert.deepEqual(a.calls[1], ['unicode.IsSpace', '13', '0']);
  assert.equal(a.get('codepoint-status').hidden, true);
});

test('multiple code points never truncate or normalize to one; editing numeric input clears helper state', async () => {
  const a = app('', undefined, spaceReply); await a.ready(); select(a, 'unicode.IsSpace');
  for (const text of ['ab', '  ', 'e\u0301', '👩‍💻', '🇰🇷', ' \n', '\ud800x', '😀a']) {
    pastePoint(a, text);
    assert.equal(a.get('first-input').value, '160');
    assert.equal(a.get('codepoint-input').attributes['aria-invalid'], 'true');
    a.get('run-form').emit('submit');
    await a.get('share-button').emit('click');
    assert.equal(a.calls.length, 0); assert.equal(a.copied.length, 0);
    assert.equal(a.get('result-badge').textContent, 'ERROR');
  }
  a.get('first-input').value = '65279'; a.get('first-input').emit('input');
  assert.equal(a.get('codepoint-input').value, '');
  assert.equal(a.get('run-button').disabled, false);
  a.get('run-form').emit('submit');
  assert.deepEqual(a.calls, [['unicode.IsSpace', '65279', '0']]);
  a.get('codepoint-input').value = 'e\u0301'; a.get('codepoint-input').emit('input');
  assert.equal(a.get('run-button').disabled, true);
  a.get('codepoint-input').value = ''; a.get('codepoint-input').emit('input');
  assert.equal(a.get('run-button').disabled, false);
  assert.equal(a.get('first-input').value, '65279');
});

test('whitespace presets and helper edits replace canonical input and cancel pending shared auto-run', async () => {
  for (const decimal of ['160', '133', '8203', '65279', '9', '12288']) {
    const a = app(link({ operation: 'unicode.IsSpace', inputs: ['32', '0'] }), undefined, spaceReply);
    a.get('whitespace-presets').value = decimal; a.get('whitespace-presets').emit('change');
    await a.ready();
    assert.equal(a.calls.length, 0);
    assert.equal(a.get('first-input').value, decimal);
    assert.equal(a.get('codepoint-input').value, '');
    a.get('run-form').emit('submit');
    assert.deepEqual(a.calls, [['unicode.IsSpace', decimal, '0']]);
  }
  const a = app(link({ operation: 'unicode.IsSpace', inputs: ['32', '0'] }), undefined, spaceReply);
  pastePoint(a, '\u0085'); await a.ready();
  assert.equal(a.calls.length, 0);
  a.get('run-form').emit('submit'); assert.deepEqual(a.calls, [['unicode.IsSpace', '133', '0']]);
});

test('function changes and shared input replace stale helper content or errors', async () => {
  const a = app('', undefined, spaceReply); await a.ready(); select(a, 'unicode.IsSpace');
  pastePoint(a, '\u00a0'); select(a, 'utf8.ValidRune');
  assert.equal(a.get('codepoint-helper').hidden, true); assert.equal(a.get('codepoint-input').disabled, true);
  assert.equal(a.get('codepoint-input').value, '');
  select(a, 'unicode.IsSpace'); pastePoint(a, 'ab');
  a.hash(link({ operation: 'unicode.IsSpace', inputs: ['-2147483648', '0'] }));
  assert.equal(a.get('codepoint-input').value, '');
  assert.equal(a.get('codepoint-status').hidden, true);
  assert.deepEqual(a.calls, [['unicode.IsSpace', '-2147483648', '0']]);
  a.hash(link({ operation: 'unicode.IsSpace', inputs: ['2147483647', '0'], wasm: 'f'.repeat(64) }));
  assert.equal(a.calls.length, 1); assert.equal(a.get('first-input').value, '2147483647');
  a.get('run-form').emit('submit'); assert.deepEqual(a.calls[1], ['unicode.IsSpace', '2147483647', '0']);
});

test('IsSpace rejects out-of-int32, noncanonical numeric values and a wrong output kind', async () => {
  const a = app('', undefined, spaceReply); await a.ready(); select(a, 'unicode.IsSpace');
  for (const value of ['-2147483649', '2147483648', '4294967328', '9007199254740993', ' 32', '+32', '032', '-0', '1e2']) {
    a.get('first-input').value = value; a.get('first-input').emit('input'); a.get('run-form').emit('submit');
    assert.equal(a.get('result-badge').textContent, 'ERROR');
  }
  assert.equal(a.calls.length, 0);
  const b = app(link({ operation: 'unicode.IsSpace', inputs: ['160', '0'] }), undefined,
    (operation, inputs) => ({ ok: true, operation, inputs, output: { type: 'text', value: 'true' } }));
  await b.ready(); assert.equal(b.get('result-badge').textContent, 'ERROR');
});
