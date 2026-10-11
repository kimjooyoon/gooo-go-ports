// UI state and transport tests; real computation is exercised by the Wasm smoke.
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { webcrypto, createHash } = require('node:crypto');
const protocol = require('../site/tutorial-protocol.js');
const source = fs.readFileSync(path.join(__dirname, '../site/tutorial.js'), 'utf8');
const html = fs.readFileSync(path.join(__dirname, '../site/tutorial.html'), 'utf8');
const hash = data => createHash('sha256').update(data).digest('hex');
const samples = Object.fromEntries(protocol.files.map(file => [file, Buffer.from(`original ${file}\n`)]));
const archive = Buffer.from('synthetic archive for trust-boundary testing');
const metadata = { schema: 'gooo/browser-tutorial/v1', compiler_revision: '83928fa4538245abe33b08549da47f8cbdc50ed2',
  compiler_version: '0.6.32-dev', go_version: 'go1.27.2', source_revision: 'b'.repeat(40), source_dirty: false,
  entry_id: 'urn:learn-gooo:describe', result_entity_id: 'urn:learn-gooo:summary',
  input_entity_ids: ['urn:gooo:type:integer', 'urn:gooo:type:string'], wasm_sha256: 'a'.repeat(64), wasm_bytes: 4,
  generated_sha256: 'c'.repeat(64), execution_sha256: 'd'.repeat(64), replay_sha256: 'e'.repeat(64),
  fixture_cases: 20, native_cases: 4, source_zip_sha256: hash(archive), source_zip_bytes: archive.length,
  sample_sha256: Object.fromEntries(protocol.files.map(file => [file, hash(samples[file])])) };
const output = { score: '42', remaining: '58', in_range: true, label: 'sensor: low' };
async function until(predicate) {
  for (let i = 0; i < 100; i++) { if (predicate()) return; await new Promise(resolve => setTimeout(resolve, 2)); }
  assert.fail('UI did not reach the expected state');
}
class Element {
  constructor() { this.value = ''; this.hidden = false; this.disabled = false; this.textContent = ''; this.listeners = {}; this.attributes = {}; }
  addEventListener(name, handler) { this.listeners[name] = handler; }
  emit(name, event = {}) { return this.listeners[name]?.({ preventDefault() {}, ...event }); }
  setAttribute(name, value) { this.attributes[name] = value; }
  remove() { this.removed = true; }
}
function app(options = {}) {
  const elements = new Map(), workers = [], requests = [], timers = new Map(), urls = [], revoked = [], downloads = [], events = new Map();
  const tags = new Map([...html.matchAll(/<[^>]+\bid="([^"]+)"[^>]*>/g)].map(match => [match[1], match[0]]));
  function get(id) {
    assert.ok(tags.has(id), `missing HTML id: ${id}`);
    if (!elements.has(id)) { const el = new Element(); el.hidden = /\shidden(?:\s|>)/.test(tags.get(id)); elements.set(id, el); }
    return elements.get(id);
  }
  get('tutorial-score').value = '-5'; get('tutorial-label').value = 'sensor'; get('tutorial-label-mode').value = 'text';
  get('tutorial-preset').value = '-5'; get('tutorial-source-file').value = 'main.gooo';
  let timerId = 0, reloaded = false;
  class Worker {
    constructor(url) { assert.equal(url, './tutorial-worker.js'); this.messages = []; this.terminated = false; workers.push(this); }
    postMessage(message) { this.messages.push(JSON.parse(JSON.stringify(message))); }
    terminate() { this.terminated = true; }
    deliver(data) { this.onmessage({ data }); }
  }
  const context = { Worker, TextEncoder, TextDecoder, Uint8Array, AbortController, Blob, console,
    GoooTutorialProtocol: protocol, crypto: options.noCrypto ? undefined : webcrypto,
    location: { reload() { reloaded = true; } },
    setTimeout(fn, delay) { const id = ++timerId; timers.set(id, { fn, delay }); return id; },
    clearTimeout(id) { timers.delete(id); }, addEventListener(name, fn) { events.set(name, fn); },
    fetch: async (url, fetchOptions) => {
      requests.push({ url, options: fetchOptions });
      if (options.fetch) return options.fetch(url, fetchOptions);
      const bytes = url.endsWith('source.zip') ? archive : samples[url.split('/').at(-1)];
      assert.ok(bytes, `unexpected fetch ${url}`);
      return { ok: true, arrayBuffer: async () => bytes };
    },
    URL: { createObjectURL(blob) { const url = 'blob:local-' + urls.length; urls.push({ url, blob }); return url; }, revokeObjectURL(url) { revoked.push(url); } },
    document: { getElementById: get, body: { append(el) { el.attached = true; } }, createElement(tag) {
      assert.equal(tag, 'a'); const el = new Element();
      el.click = () => { if (options.downloadError) throw new Error('download blocked'); downloads.push({ url: el.href, name: el.download, attached: el.attached }); };
      return el;
    } }
  };
  context.window = context;
  vm.runInNewContext(source, context, { filename: 'site/tutorial.js' });
  const current = () => workers.at(-1);
  const send = data => current().deliver(data);
  return { get, current, requests, timers, urls, revoked, downloads,
    ready: async () => { send({ type: 'ready', metadata: { ...metadata, ...options.metadata } }); await until(() => get('tutorial-runtime').textContent.startsWith('Ready')); },
    deliverReady: () => send({ type: 'ready', metadata: { ...metadata, ...options.metadata } }),
    failed: () => until(() => get('tutorial-runtime').className === 'tutorial-runtime error'),
    run: () => { get('tutorial-form').emit('submit'); return current()?.messages.at(-1); },
    reply: (request = current().messages.at(-1), changes = {}) => send({ type: 'result', id: request.id,
      wasmSha256: metadata.wasm_sha256, result: { ok: true, inputs: [request.inputs[0], protocol.boundText(request.inputs[1])], output }, ...changes }),
    send,
    edit(id, value) { get(id).value = value; get(id).emit('input'); },
    mode(mode) { get('tutorial-label-mode').value = mode; get('tutorial-label-mode').emit('change'); },
    preset(score) { get('tutorial-preset').value = score; get('tutorial-preset').emit('change'); },
    download: () => get('tutorial-download-inputs').emit('click'),
    pagehide: (persisted = false) => events.get('pagehide')({ persisted }),
    pageshow: () => events.get('pageshow')?.({ persisted: true }), get reloaded() { return reloaded; } };
}

test('no automatic execution; six displayed files and portable ZIP are verified before ready', async () => {
  const a = app(); a.run(); assert.equal(a.current().messages.length, 0);
  await a.ready();
  assert.equal(a.current().messages.length, 0);
  assert.equal(a.requests.length, 7);
  assert.ok(a.requests.every(r => r.options.cache === 'no-store'));
  assert.equal(a.get('tutorial-source').textContent, samples['main.gooo'].toString());
  assert.equal(a.get('tutorial-download').hidden, false);
  assert.equal(await a.urls[0].blob.text(), archive.toString());
  for (const score of ['-5', '42', '75', '105']) {
    a.preset(score); const request = a.run();
    assert.deepEqual(request.inputs, [score, 'sensor']); a.reply(request);
    assert.equal(a.get('result-score').textContent, '42', 'UI displays the returned value instead of implementing clamp');
    assert.equal(a.get('result-label').textContent, '"sensor: low"');
    assert.equal(a.get('tutorial-result-badge').textContent, 'EXECUTED');
  }
});

test('both input edits and presets erase old fields/raw JSON immediately; stale work cannot restore it', async () => {
  const a = app(); await a.ready(); const first = a.run(); a.reply(first);
  for (const [id, value] of [['tutorial-score', '9007199254740993'], ['tutorial-label', 'new label']]) {
    a.edit(id, value);
    assert.equal(a.get('tutorial-values').hidden, true);
    assert.equal(a.get('tutorial-raw-output').textContent, '');
    assert.equal(a.get('result-score').textContent, '');
    assert.equal(a.get('tutorial-result-badge').textContent, 'NOT RUN');
    const running = a.run(); a.edit(id, value + (id.endsWith('score') ? '0' : '!'));
    a.reply(running);
    assert.equal(a.get('tutorial-values').hidden, true);
    a.preset('42'); a.reply(a.run());
  }
  const count = a.current().messages.length;
  a.preset('105'); assert.equal(a.current().messages.length, count);
  assert.equal(a.get('tutorial-values').hidden, true);
});

test('full int64 and JSON-string labels retain BOM, controls and native surrogate conversion visibly', async () => {
  const a = app(); await a.ready(); a.mode('json');
  const raw = '"\\ufeff\\r\\n\\u0000\\ud800😀"';
  a.edit('tutorial-label', raw); a.edit('tutorial-score', '9223372036854775807');
  assert.equal(a.get('tutorial-label-warning').hidden, false);
  assert.equal(a.get('tutorial-label-preview').textContent, JSON.stringify('\uFEFF\r\n\0�😀'));
  const request = a.run(); assert.deepEqual(request.inputs, ['9223372036854775807', '\uFEFF\r\n\0\uD800😀']);
  a.reply(request, { result: { ok: true, inputs: ['9223372036854775807', '\uFEFF\r\n\0�😀'],
    output: { ...output, label: '\uFEFF\r\n\0�😀: high' } } });
  assert.equal(a.get('result-label').textContent, JSON.stringify('\uFEFF\r\n\0�😀: high'));
  a.download();
  const download = a.downloads.at(-1); assert.equal(download.name, 'inputs.json'); assert.equal(download.attached, true);
  const text = await a.urls.find(item => item.url === download.url).blob.text();
  assert.match(text, /: 9223372036854775807,/);
  assert.equal(JSON.parse(text).inputs[0]['app/report:Describe.input1'], request.inputs[1]);
});

test('invalid decimal, oversized label or invalid JSON submits nothing and does not keep a successful result', async () => {
  const a = app(); await a.ready(); a.reply(a.run());
  for (const score of ['9223372036854775808', '+1', '-0', ' 42', '1e2']) {
    a.edit('tutorial-score', score); const count = a.current().messages.length; a.run();
    assert.equal(a.current().messages.length, count); assert.equal(a.get('tutorial-values').hidden, true);
    assert.equal(a.get('tutorial-score').attributes['aria-invalid'], 'true');
  }
  a.preset('42'); a.edit('tutorial-label', '😀'.repeat(257)); const count = a.current().messages.length; a.run();
  assert.equal(a.current().messages.length, count);
  a.mode('json');
  for (const value of ['null', '{}', 'false', '"unterminated']) { a.edit('tutorial-label', value); a.run(); assert.equal(a.current().messages.length, count); }
});

test('accepted 1024-byte inputs reach the worker; actual output-limit errors clear any earlier success', async () => {
  const a = app(); await a.ready(); a.reply(a.run()); a.edit('tutorial-label', 'x'.repeat(1024));
  const request = a.run(); assert.equal(request.inputs[1].length, 1024);
  a.send({ type: 'error', fatal: false, id: request.id, message: 'Returned Text exceeds 1024 bytes.' });
  assert.equal(a.get('tutorial-values').hidden, true); assert.equal(a.get('tutorial-raw-output').textContent, '');
  assert.match(a.get('tutorial-error').textContent, /Returned Text/);
  assert.equal(a.get('tutorial-run').disabled, false, 'input limit must not be reduced to make outputs fit');
});

test('mismatched output, malformed ready and runtime failure leave no visible success', async () => {
  const a = app(); await a.ready(); a.reply(a.run());
  const request = a.run();
  a.reply(request, { result: { ok: true, inputs: ['0', 'wrong'], output } }); await a.failed();
  assert.equal(a.get('tutorial-values').hidden, true); assert.equal(a.get('tutorial-run').disabled, true);
  assert.equal(a.current().terminated, true); assert.equal(a.get('tutorial-download').hidden, true);
  const b = app(); await b.ready(); b.reply(b.run()); b.current().onerror(); await b.failed();
  assert.equal(b.get('result-label').textContent, '');
  const c = app(); c.send({ type: 'ready', metadata, extra: true }); await c.failed();
  assert.equal(c.requests.length, 0);
});

test('wrong source/ZIP hashes and failed HTTP loads prevent ready and project download', async () => {
  for (const bad of ['main.gooo', 'source.zip', 'http']) {
    const a = app({ fetch: async url => {
      if (bad === 'http') return { ok: false, status: 404 };
      const data = url.endsWith('source.zip') ? archive : samples[url.split('/').at(-1)];
      return { ok: true, arrayBuffer: async () => url.endsWith(bad) ? Buffer.from('tampered') : data };
    } });
    a.deliverReady(); await a.failed(); a.run();
    assert.equal(a.current().messages.length, 0); assert.equal(a.get('tutorial-download').hidden, true);
    assert.equal(a.get('tutorial-values').hidden, true); assert.equal(a.get('tutorial-retry').hidden, false);
    a.get('tutorial-retry').emit('click'); assert.equal(a.reloaded, true);
  }
});

test('loading/runtime timeouts and page exit terminate workers and release downloads', async () => {
  const a = app(); const load = [...a.timers.values()].find(t => t.delay === 30000); load.fn(); await a.failed();
  assert.equal(a.current().terminated, true);
  const b = app(); await b.ready(); b.run(); [...b.timers.values()].find(t => t.delay === 10000).fn(); await b.failed();
  assert.equal(b.get('tutorial-values').hidden, true);
  const c = app(); await c.ready(); c.pagehide();
  assert.equal(c.current().terminated, true); assert.deepEqual(c.revoked, [c.urls[0].url]);
});

test('cached tutorial preserves its worker and loading timeout before the ready message', async () => {
  const a = app(); const worker = a.current();
  const loadTimer = [...a.timers.entries()];
  a.pagehide(true); a.pageshow();
  assert.equal(worker.terminated, false);
  assert.equal(a.current(), worker);
  assert.deepEqual([...a.timers.entries()], loadTimer);
  assert.equal(a.get('tutorial-run').disabled, true);
  await a.ready();
  assert.equal(a.get('tutorial-run').disabled, false);
  assert.equal(a.timers.size, 0);
  assert.equal(worker.messages.length, 0, 'restoration must not automatically run the user input');
});

test('cached tutorial preserves in-flight source verification and the verified project download', async () => {
  let release;
  const gate = new Promise(resolve => { release = resolve; });
  const a = app({ fetch: async url => {
    await gate;
    const bytes = url.endsWith('source.zip') ? archive : samples[url.split('/').at(-1)];
    return { ok: true, arrayBuffer: async () => bytes };
  } });
  a.deliverReady();
  await until(() => a.requests.length === protocol.files.length);
  a.pagehide(true); a.pageshow();
  assert.equal(a.current().terminated, false);
  assert.ok(a.requests.every(request => !request.options.signal.aborted));
  release();
  await until(() => a.get('tutorial-runtime').textContent.startsWith('Ready'));
  assert.equal(a.requests.length, 7);
  assert.equal(a.get('tutorial-download').hidden, false);
  assert.equal(await a.urls[0].blob.text(), archive.toString());
  assert.deepEqual(a.revoked, []);
  assert.equal(a.timers.size, 0);
});

test('cached tutorial retains edited inputs and a live ZIP while allowing a new run on return', async () => {
  const a = app(); await a.ready(); a.preset('42'); a.reply(a.run());
  a.edit('tutorial-label', 'kept input');
  const worker = a.current(), zipURL = a.get('tutorial-download').href;
  a.pagehide(true); a.pageshow();
  assert.equal(worker.terminated, false);
  assert.equal(a.get('tutorial-score').value, '42');
  assert.equal(a.get('tutorial-label').value, 'kept input');
  assert.equal(a.get('tutorial-download').href, zipURL);
  assert.equal(a.get('tutorial-download').hidden, false);
  assert.deepEqual(a.revoked, []);
  assert.ok(a.requests.every(request => !request.options.signal.aborted));
  assert.equal(worker.messages.length, 1);
  a.edit('tutorial-score', '75');
  const request = a.run(); assert.deepEqual(request.inputs, ['75', 'kept input']);
  a.reply(request, { result: { ok: true, inputs: request.inputs,
    output: { score: '75', remaining: '25', in_range: true, label: 'kept input: high' } } });
  assert.equal(a.get('result-score').textContent, '75');
  assert.equal(a.get('tutorial-result-badge').textContent, 'EXECUTED');
  assert.equal(a.current(), worker);
  assert.equal(a.urls.length, 1);
});

test('cached tutorial retains a pending run and its timeout until its result resumes', async () => {
  const a = app(); await a.ready(); a.preset('42');
  const request = a.run(), worker = a.current(), runTimer = [...a.timers.entries()];
  a.pagehide(true); a.pageshow();
  assert.equal(worker.terminated, false);
  assert.equal(a.get('tutorial-result-badge').textContent, 'RUNNING');
  assert.equal(a.get('tutorial-run').disabled, true);
  assert.deepEqual([...a.timers.entries()], runTimer);
  assert.equal(worker.messages.length, 1);
  a.reply(request);
  assert.equal(a.get('result-score').textContent, '42');
  assert.equal(a.get('tutorial-result-badge').textContent, 'EXECUTED');
  assert.equal(a.get('tutorial-run').disabled, false);
  assert.equal(a.timers.size, 0);
});

test('terminal pagehide after a cached return still releases the worker and verified download', async () => {
  const a = app(); await a.ready();
  a.pagehide(true); a.pageshow();
  assert.equal(a.current().terminated, false);
  assert.deepEqual(a.revoked, []);
  a.pagehide(false);
  assert.equal(a.current().terminated, true);
  assert.ok(a.requests.every(request => request.options.signal.aborted));
  assert.deepEqual(a.revoked, [a.urls[0].url]);
});
