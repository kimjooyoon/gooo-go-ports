// UI scheduling and rendering tests. Worker/real-Wasm tests validate computation.
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const input = require('../site/text-input.js');
const protocol = require('../site/text-inspector-protocol.js');
const source = fs.readFileSync(path.join(__dirname, '../site/text-inspector.js'), 'utf8');
const identity = { wasmSha256: 'a'.repeat(64), sourceRevision: 'b'.repeat(40) };

class Element {
  constructor() {
    this._text = ''; this.children = []; this.value = ''; this.hidden = false; this.disabled = false;
    this.attributes = {}; this.dataset = {}; this.listeners = {}; this.firstChild = { textContent: '' };
  }
  get textContent() { return this._text + this.children.map(child => child.textContent).join(''); }
  set textContent(value) { this._text = String(value); this.children = []; }
  replaceChildren(...children) { this._text = ''; this.children = children; }
  append(...children) { this.children.push(...children); }
  setAttribute(name, value) { this.attributes[name] = value; }
  addEventListener(name, handler) { this.listeners[name] = handler; }
  emit(name, event = {}) { return this.listeners[name]?.({ preventDefault() {}, ...event }); }
}

function app() {
  const elements = new Map();
  const get = id => { if (!elements.has(id)) elements.set(id, new Element()); return elements.get(id); };
  get('text-mode').value = 'text';
  const presets = Object.keys(input.presets).map(key => { const button = new Element(); button.dataset.textPreset = key; return button; });
  const workers = [];
  const timers = new Map();
  let timerId = 0;
  class Worker {
    constructor(url) { assert.equal(url, './text-inspector-worker.js'); this.messages = []; this.terminated = false; workers.push(this); }
    postMessage(message) { this.messages.push(JSON.parse(JSON.stringify(message))); }
    terminate() { this.terminated = true; }
    deliver(data) { this.onmessage({ data }); }
  }
  const context = { Worker, GoooTextInput: input, GoooTextInspectorProtocol: protocol,
    setTimeout(handler, delay) { const id = ++timerId; timers.set(id, { handler, delay }); return id; },
    clearTimeout(id) { timers.delete(id); },
    document: { getElementById: get, createElement: () => new Element(), querySelectorAll(selector) { assert.equal(selector, '[data-text-preset]'); return presets; } } };
  context.window = context;
  vm.runInNewContext(source, context, { filename: 'site/text-inspector.js' });
  const current = () => workers.at(-1);
  const ready = (worker = current()) => worker.deliver({ type: 'ready', ...identity });
  const type = value => { get('text-capture').value = value; get('text-capture').emit('input'); };
  const transfer = (value, kind = 'paste') => {
    let prevented = false;
    get('text-capture').emit(kind, { preventDefault() { prevented = true; },
      [kind === 'paste' ? 'clipboardData' : 'dataTransfer']: { types: ['text/plain'], getData(format) { assert.equal(format, 'text/plain'); return value; } } });
    assert.equal(prevented, true);
  };
  const run = () => { get('text-form').emit('submit'); return current().messages.at(-1); };
  const reply = (mapper = point => ({ input: point, validScalar: true, goSpace: false, utf8Bytes: '1', utf16Units: '1' }), worker = current(), changes = {}) => {
    const request = worker.messages.at(-1);
    worker.deliver({ type: 'result', id: request.id, rows: request.points.map(mapper), ...identity, ...changes });
  };
  const mode = name => { get('text-mode').value = name; get('text-mode').emit('change'); };
  return { get, workers, current, ready, type, transfer, run, reply, mode, timers, preset: name => presets.find(button => button.dataset.textPreset === name).emit('click') };
}

test('raw paste/drop replaces the complete input and preserves CRLF before editor normalization', () => {
  for (const kind of ['paste', 'drop']) {
    const a = app(); a.ready(); a.type('old input');
    a.transfer('A\u00A0\r\n\u200B😀', kind);
    assert.equal(a.get('text-capture').value, '');
    assert.equal(a.get('text-preview').textContent, '"\\u0041\\u00A0\\u000D\\u000A\\u200B\\uD83D\\uDE00"');
    assert.match(a.get('text-source').textContent, /before insertion.*cleared.*CRLF/);
    assert.equal(a.current().messages.length, 0);
    assert.deepEqual(a.run().points, ['65', '160', '13', '10', '8203', '128512']);
  }
});

test('typing after paste starts a new authoritative input, and edits hide old results', () => {
  const a = app(); a.ready(); a.transfer('\r\n'); a.run(); a.reply();
  assert.equal(a.get('text-rows').children.length, 2);
  a.type('x');
  assert.equal(a.get('text-rows').children.length, 0);
  assert.equal(a.get('text-summary').hidden, true);
  assert.equal(a.get('text-preview').textContent, '"\\u0078"');
  assert.match(a.get('text-source').textContent, /editor behavior/);
  assert.deepEqual(a.run().points, ['120']);
});

test('UI displays compiled Boolean/length fields rather than reclassifying input', () => {
  const a = app(); a.ready(); a.transfer('\uFEFF\uD800😀'); a.run();
  a.reply(point => ({ input: point, validScalar: true, goSpace: true, utf8Bytes: '4', utf16Units: '2' }));
  const rows = a.get('text-rows').children;
  assert.equal(rows.length, 3);
  assert.equal(rows[1].children[3].textContent, 'Yes', 'surrogate result must be the worker Boolean');
  assert.equal(rows[0].children[4].textContent, 'Yes', 'BOM result must be the worker Boolean');
  assert.match(rows[1].children[1].textContent, /\\uD800.*1 received unit/);
  assert.match(rows[2].children[0].textContent, /offset 2/);
  assert.match(a.get('text-summary').textContent, /4 received UTF-16.*3 Go whitespace.*12 UTF-8 bytes and 6 UTF-16/);
});

test('invalid scalars or returned -1 encoding values suppress totals but retain received-unit count', () => {
  for (const row of [
    { validScalar: false, goSpace: false, utf8Bytes: '-1', utf16Units: '-1' },
    { validScalar: true, goSpace: false, utf8Bytes: '-1', utf16Units: '1' },
    { validScalar: true, goSpace: false, utf8Bytes: '1', utf16Units: '-1' },
    { validScalar: false, goSpace: true, utf8Bytes: '1', utf16Units: '1' }
  ]) {
    const a = app(); a.ready(); a.mode('json'); a.type('"\\uD800"'); a.run();
    a.reply(point => ({ input: point, ...row }));
    assert.match(a.get('text-summary').textContent, /1 received UTF-16.*totals are unavailable/);
    assert.doesNotMatch(a.get('text-summary').textContent, /Encoded totals:/);
  }
});

test('all-or-nothing bounds disable running and clear old results without posting truncated input', () => {
  const a = app(); a.ready(); a.type('x'); a.run(); a.reply();
  for (const source of ['x'.repeat(257), '😀'.repeat(256) + 'x']) {
    a.transfer(source);
    assert.equal(a.get('text-run').disabled, true);
    assert.equal(a.get('text-rows').children.length, 0);
    assert.equal(a.get('text-capture').attributes['aria-invalid'], 'true');
    a.run(); assert.equal(a.current().messages.length, 1);
  }
  a.transfer('😀'.repeat(256));
  assert.equal(a.get('text-run').disabled, false);
  assert.equal(a.run().points.length, 256);
});

test('JSON mode and presets preserve explicit lone surrogates; invalid JSON cannot reuse prior input', () => {
  const a = app(); a.ready(); a.preset('surrogates');
  assert.equal(a.get('text-mode').value, 'json');
  assert.equal(a.get('text-json-limit').hidden, false);
  assert.match(a.get('text-source').textContent, /Built-in example.*not a claim about clipboard/);
  assert.deepEqual(a.run().points, ['55296', '65', '57343', '65533']); a.reply();
  a.type('"\\u');
  assert.equal(a.get('text-run').disabled, true);
  assert.equal(a.get('text-summary').hidden, true);
  a.run(); assert.equal(a.current().messages.length, 1);
  a.transfer('"\\r\\n"');
  assert.deepEqual(a.run().points, ['13', '10']); a.reply();
  a.mode('text');
  assert.equal(a.get('text-rows').children.length, 0);
  assert.equal(a.get('text-preview').textContent, 'No input yet.');
});

test('empty text and empty JSON send zero points and render no fabricated per-item calls', () => {
  for (const json of [false, true]) {
    const a = app(); a.ready(); if (json) a.mode('json');
    a.type(json ? '""' : '');
    assert.equal(a.get('text-capture').attributes['aria-invalid'], 'false');
    assert.deepEqual(a.run().points, []); a.reply();
    assert.equal(a.get('text-rows').children.length, 0);
    assert.match(a.get('text-progress').textContent, /empty input.*no Gooo calls/);
    assert.match(a.get('text-summary').textContent, /0 decoded items.*0 UTF-8 bytes and 0 UTF-16/);
  }
});

test('in-flight edits terminate the worker and ignore every late event from its old instance', () => {
  const a = app(); a.ready(); a.type('old'); const request = a.run(); const old = a.current();
  a.transfer('new');
  assert.equal(old.terminated, true);
  assert.equal(a.workers.length, 2);
  assert.equal(a.get('text-run').disabled, true);
  a.ready(old);
  old.deliver({ type: 'progress', id: request.id, completed: 3, total: 3 });
  a.reply(undefined, old);
  old.deliver({ type: 'error', id: request.id, message: 'late failure' });
  old.onerror({ preventDefault() {} });
  assert.equal(a.get('text-rows').children.length, 0);
  assert.equal(a.get('text-result-error').hidden, true);
  a.ready(); assert.deepEqual(a.run().points, ['110', '101', '119']); a.reply();
  assert.match(a.get('text-progress').textContent, /Complete/);
});

test('mode change, clear, invalid input and cancellation invalidate in-flight results', () => {
  for (const action of [a => a.mode('json'), a => a.get('text-clear').emit('click'), a => a.transfer('x'.repeat(257)), a => a.get('text-cancel').emit('click')]) {
    const a = app(); a.ready(); a.type('before'); a.run(); const old = a.current();
    action(a); a.reply(undefined, old);
    assert.equal(old.terminated, true);
    assert.equal(a.get('text-rows').children.length, 0);
    assert.equal(a.get('text-summary').hidden, true);
    assert.equal(a.get('text-cancel').hidden, true);
  }
});

test('progress never exposes partial rows; duplicate submits and stale IDs cannot replace results', () => {
  const a = app(); a.ready(); a.type('x'.repeat(32)); const request = a.run();
  a.run(); assert.equal(a.current().messages.length, 1);
  a.current().deliver({ type: 'progress', id: request.id, completed: 16, total: 32 });
  assert.equal(a.get('text-progress-bar').value, 16);
  assert.equal(a.get('text-rows').children.length, 0);
  a.reply(undefined, a.current(), { id: request.id + 1 });
  assert.equal(a.get('text-rows').children.length, 0);
  a.reply(); assert.equal(a.get('text-rows').children.length, 32);
  a.type('a');
  a.reply(); assert.equal(a.get('text-rows').children.length, 0);
});

test('malformed rows, program mismatch, worker errors and backwards progress clear results', () => {
  const corruptions = [
    (a, request) => a.reply(undefined, a.current(), { wasmSha256: 'c'.repeat(64) }),
    a => a.reply(() => ({ input: '66', validScalar: true, goSpace: false, utf8Bytes: '1', utf16Units: '1' })),
    a => a.reply(undefined, a.current(), { rows: [] }),
    a => a.reply(() => ({ input: '65', validScalar: true, goSpace: false, utf8Bytes: '0', utf16Units: '1' })),
    (a, request) => a.current().deliver({ type: 'error', id: request.id, message: 'compiled call failed' }),
    (a, request) => { a.current().deliver({ type: 'progress', id: request.id, completed: 1, total: 1 }); a.current().deliver({ type: 'progress', id: request.id, completed: 0, total: 1 }); }
  ];
  for (const corrupt of corruptions) {
    const a = app(); a.ready(); a.type('A'); const request = a.run(); corrupt(a, request);
    assert.equal(a.get('text-result-error').hidden, false);
    assert.equal(a.get('text-run').disabled, true);
    assert.equal(a.get('text-rows').children.length, 0);
    assert.equal(a.get('text-summary').hidden, true);
    assert.equal(a.get('text-retry').hidden, false);
    a.get('text-retry').emit('click'); a.ready(); assert.deepEqual(a.run().points, ['65']); a.reply();
    assert.equal(a.get('text-result-error').hidden, true);
  }
});

test('unsupported clipboard data invalidates previous input rather than silently reusing it', () => {
  const a = app(); a.ready(); a.type('A'); a.run(); a.reply();
  a.get('text-capture').emit('paste', { clipboardData: { types: ['image/png'], getData() { throw new Error('must not read unsupported format'); } } });
  assert.equal(a.get('text-run').disabled, true);
  assert.equal(a.get('text-rows').children.length, 0);
  assert.match(a.get('text-input-status').textContent, /plain text/);
  a.type('new'); a.run(); const old = a.current();
  a.get('text-capture').emit('drop', { dataTransfer: { getData() { throw new Error('clipboard access failed'); } } });
  assert.equal(old.terminated, true);
  assert.match(a.get('text-input-status').textContent, /could not be read/);
  assert.equal(a.get('text-run').disabled, true);
});

test('loading timeout is bounded and cannot stop a newer or already ready worker', () => {
  const a = app(); a.type('A');
  const firstTimer = [...a.timers.values()][0];
  assert.equal(firstTimer.delay, 30000);
  firstTimer.handler();
  assert.equal(a.current().terminated, true);
  assert.equal(a.timers.size, 0);
  assert.match(a.get('text-runtime').textContent, /timed out/);
  a.get('text-retry').emit('click');
  const current = a.current();
  const secondTimer = [...a.timers.values()][0];
  firstTimer.handler();
  assert.equal(current.terminated, false);
  a.ready();
  assert.equal(a.timers.size, 0);
  secondTimer.handler();
  assert.equal(current.terminated, false);
  assert.deepEqual(a.run().points, ['65']);
});

test('page exposes matching input controls, scripts and responsive semantic table', () => {
  const html = fs.readFileSync(path.join(__dirname, '../site/text-inspector.html'), 'utf8');
  const css = fs.readFileSync(path.join(__dirname, '../site/text-inspector.css'), 'utf8');
  assert.ok(html.indexOf('text-inspector-protocol.js') < html.indexOf('text-inspector.js'));
  for (const name of ['text-mode', 'text-capture', 'text-preview', 'text-form', 'text-run', 'text-rows', 'text-progress']) assert.match(html, new RegExp('id="' + name + '"'));
  assert.match(html, /<caption>One row per UTF-16-decoded code point/);
  assert.equal((html.match(/scope="col"/g) || []).length, 7);
  assert.match(html, /4,096 characters before decoding/);
  assert.match(css, /@media \(max-width: 620px\)/);
});
