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
    this.firstChild = { textContent: '' }; this.classList = { toggle() {} };
  }
  addEventListener(name, fn) { this.listeners.set(name, fn); }
  emit(name) { return this.listeners.get(name)?.({ preventDefault() {} }); }
  replaceChildren(...children) { this.children = children; }
  append(...children) { this.children.push(...children); }
  setAttribute() {}
  focus() { this.focused = true; }
  select() { this.selected = true; }
}
function app(fragment = '', clipboard = async () => {}) {
  const elements = new Map();
  const get = id => { if (!elements.has(id)) elements.set(id, new Element()); return elements.get(id); };
  get('operation').value = 'http.StatusText'; get('first-input').value = '404'; get('second-input').value = '56832';
  const scenarioButtons = ['http', 'unicode', 'bytes'].map(scenario => { const e = new Element(); e.dataset.scenario = scenario; return e; });
  const events = new Map(), calls = [], copied = [];
  const metaGate = deferred(), wasmGate = deferred(), fixtureGate = deferred();
  let url = new URL('https://example.test/demo/' + fragment);
  const context = {
    URL, URLSearchParams, Uint8Array, Date, setTimeout, console, crypto: webcrypto, GoooShare: share,
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
        context.goooRun = (operation, a, b) => {
          calls.push([operation, a, b]);
          return JSON.stringify({ ok: true, operation, inputs: [a, b], output: { type: 'text', value: 'runtime test result' } });
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
    finishChecks() { fixtureGate.resolve({ ok: true, json: async () => ({ cases: [{ operation: 'bits.Reverse8', inputs: ['170', '0'], expected: { type: 'text', value: 'runtime test result' } }], rejections: [] }) }); }
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
