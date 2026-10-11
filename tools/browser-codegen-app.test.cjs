// UI state and download transport tests. The separate real-Wasm/CLI smoke
// validates compiler behavior; these simulated Worker replies do not do so.
'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { webcrypto } = require('node:crypto');

const source = fs.readFileSync(path.join(__dirname, '../site/codegen/probe.js'), 'utf8');
const html = fs.readFileSync(path.join(__dirname, '../site/codegen/index.html'), 'utf8');
const fixtureBook = JSON.parse(fs.readFileSync(path.join(__dirname, 'browser-codegen/fixtures.json'), 'utf8'));
const metadata = {
  schema: 'gooo/browser-body-codegen-metadata/v1',
  compiler_pin: fixtureBook.compiler_pin,
  bridge_source_sha256: 'b'.repeat(64),
  go_version: 'go1.27.2', native_execution: false, model_calls: false,
  resource_limits: { max_source_bytes: 65536, max_request_bytes: 409600, max_activity_bytes: 128 },
};
const manifest = {
  compiler_pin: metadata.compiler_pin, bridge_source_sha256: metadata.bridge_source_sha256,
  wasm: { bytes: 4, sha256: 'a'.repeat(64) },
};
const generatedGo = '// 한글 😀\r\npackage sample\r\n\r\nfunc Clamp(input int64) int64 {\r\n\treturn 7\r\n}\r\n';

async function until(predicate) {
  for (let attempt = 0; attempt < 40; attempt += 1) {
    if (predicate()) return;
    await new Promise(resolve => setImmediate(resolve));
  }
  assert.fail('UI did not reach the expected state');
}

class Element {
  constructor() {
    this._text = ''; this.value = ''; this.hidden = false; this.disabled = false;
    this.children = []; this.listeners = {}; this.className = ''; this.parent = null;
  }
  get textContent() { return this._text + this.children.map(child => child.textContent).join(''); }
  set textContent(value) { this._text = String(value); this.children = []; }
  get disabled() { return this._disabled; }
  set disabled(value) { this._disabled = Boolean(value); }
  append(...children) {
    for (const child of children) child.parent = this;
    this.children.push(...children);
  }
  appendChild(child) { this.append(child); return child; }
  remove() {
    if (this.parent) this.parent.children = this.parent.children.filter(child => child !== this);
    this.parent = null;
  }
  addEventListener(name, handler) { this.listeners[name] = handler; }
  // Deliberately allow forced clicks on disabled buttons to exercise the
  // handler's own state guard, rather than relying on native disabled behavior.
  emit(name, event = {}) { return this.listeners[name]?.({ preventDefault() {}, ...event }); }
}

function app(options = {}) {
  const tags = new Map([...html.matchAll(/<[^>]+\bid="([^"]+)"[^>]*>/g)].map(match => [match[1], match[0]]));
  const elements = new Map(), workers = [], timers = new Map(), events = new Map();
  const objectUrls = [], revokedUrls = [], downloads = [], anchors = [], fetches = [], pendingDigests = [];
  const body = new Element();
  let nextTimer = 0;
  const get = id => {
    assert.ok(tags.has(id), 'missing HTML id: ' + id);
    if (!elements.has(id)) {
      const element = new Element();
      element.hidden = /\shidden(?:\s|>)/.test(tags.get(id));
      element.disabled = /\sdisabled(?:\s|>)/.test(tags.get(id));
      elements.set(id, element);
    }
    return elements.get(id);
  };
  const appendBody = body.append.bind(body);
  body.append = (...children) => {
    if (options.appendError) throw new Error('link attachment failed');
    appendBody(...children);
  };
  class Worker {
    constructor(url) {
      assert.equal(url, './probe-worker.js');
      this.messages = []; this.terminated = false; workers.push(this);
    }
    postMessage(message) {
      if (options.postError) throw new Error('worker dispatch failed');
      this.messages.push(JSON.parse(JSON.stringify(message)));
    }
    terminate() { this.terminated = true; }
    deliver(data) { this.onmessage({ data }); }
  }
  const context = {
    Worker, TextEncoder, Uint8Array, console,
    crypto: {
      randomUUID: () => webcrypto.randomUUID(),
      subtle: { digest(algorithm, bytes) {
        if (!options.holdDigest) return webcrypto.subtle.digest(algorithm, bytes);
        return new Promise((resolve, reject) => {
          pendingDigests.push({ release() { webcrypto.subtle.digest(algorithm, bytes).then(resolve, reject); } });
        });
      } },
    },
    Blob: class extends Blob {
      constructor(...args) {
        if (options.blobError) throw new Error('Blob unavailable');
        super(...args);
      }
    },
    URL: {
      createObjectURL(blob) {
        if (options.urlError) throw new Error('object URL unavailable');
        const url = 'blob:local-codegen-' + (objectUrls.length + 1);
        objectUrls.push({ url, blob }); return url;
      },
      revokeObjectURL(url) { revokedUrls.push(url); },
    },
    setTimeout(fn, delay) { const id = ++nextTimer; timers.set(id, { fn, delay }); return id; },
    clearTimeout(id) { timers.delete(id); },
    addEventListener(name, fn) { events.set(name, fn); },
    fetch: async (url, fetchOptions) => {
      assert.equal(url, './fixtures.json');
      fetches.push({ url, options: fetchOptions });
      return { ok: true, json: async () => JSON.parse(JSON.stringify(fixtureBook)) };
    },
    document: {
      body, getElementById: get,
      createElement(tag) {
        const element = new Element();
        if (tag === 'a') {
          anchors.push(element);
          element.click = () => {
            if (options.clickError) throw new Error('download click failed');
            downloads.push({ href: element.href, filename: element.download,
              hidden: element.hidden, attached: body.children.includes(element) });
          };
        } else assert.equal(tag, 'option');
        return element;
      },
    },
  };
  context.window = context;
  vm.runInNewContext(source, context, { filename: 'site/codegen/probe.js' });
  const current = () => workers.at(-1);
  const generate = async () => {
    const worker = current();
    const count = worker.messages.length;
    await get('probe-generate').emit('click');
    return worker.messages.length > count ? worker.messages.at(-1) : null;
  };
  const reply = (request = current().messages.at(-1), changes = {}, worker = current()) => {
    assert.ok(request, 'a generation request is required');
    const response = {
      schema: 'gooo/browser-body-codegen-response/v1', id: request.request.id,
      source_sha256: request.sourceSha, activity: request.request.activity,
      compiler_pin: metadata.compiler_pin, bridge_source_sha256: metadata.bridge_source_sha256,
      native_execution: false, model_calls: false, ok: true, source: generatedGo,
      gooo_source: request.request.source, report: null, ...changes.response,
    };
    // These are raw JSON numeric tokens, never pre-rounded JavaScript Numbers.
    const reportJSON = '{"compiler_source_sha":"UNBOUND_LOCAL_SOURCE",' +
      '"maximum":9223372036854775807,"adjacent":9223372036854775806,' +
      '"minimum":-9223372036854775808,"decimal":1.234567890123456789}';
    const raw = JSON.stringify(response).replace('"report":null', '"report":' + reportJSON);
    const message = {
      type: 'result', id: request.request.id, sourceSha: request.sourceSha,
      activity: request.request.activity, response: JSON.parse(raw), responseJSON: raw,
      ...changes.message,
    };
    worker.deliver(message);
    return { raw, response: message.response };
  };
  const fireTimers = delay => {
    for (const [id, timer] of [...timers]) if (timer.delay === delay) {
      timers.delete(id); timer.fn();
    }
  };
  return {
    get, current, workers, timers, body, anchors, downloads, objectUrls, revokedUrls, fetches, pendingDigests,
    async ready(worker = current()) {
      worker.deliver({ type: 'ready', metadata, manifest });
      await until(() => !get('probe-generate').disabled);
    },
    generate, reply, fireTimers,
    edit(id, value) { get(id).value = value; get(id).emit('input'); },
    sample(id) { get('probe-cases').value = id; get('probe-cases').emit('change'); },
    downloadGo: () => get('probe-download-go').emit('click'),
    downloadResponse: () => get('probe-download-response').emit('click'),
    retry: () => get('probe-retry').emit('click'),
    pagehide: () => events.get('pagehide')({ persisted: true }),
    pageshow: () => events.get('pageshow')({ persisted: true }),
  };
}

function assertUnavailable(a) {
  assert.equal(a.get('probe-download-go').disabled, true);
  assert.equal(a.get('probe-download-response').disabled, true);
  assert.equal(a.get('probe-output').textContent, '');
  assert.equal(a.get('probe-report').textContent, '');
  const count = a.objectUrls.length;
  a.downloadGo(); a.downloadResponse();
  assert.equal(a.objectUrls.length, count, 'forced clicks must not download an unavailable result');
}

async function successful(a) {
  await a.ready();
  const request = await a.generate();
  const result = a.reply(request);
  assert.equal(a.get('probe-download-go').disabled, false);
  assert.equal(a.get('probe-download-response').disabled, false);
  return { request, ...result };
}

test('downloads begin disabled and compiler loading never triggers generation or saving', async () => {
  assert.match(html, /id="probe-download-go"[^>]*\sdisabled(?:\s|>)/);
  assert.match(html, /id="probe-download-response"[^>]*\sdisabled(?:\s|>)/);
  assert.match(html, /id="probe-download-status"[^>]*role="status"[^>]*\shidden(?:\s|>)/);
  const a = app();
  assertUnavailable(a);
  assert.equal(a.get('probe-download-status').hidden, true);
  await a.ready(); assertUnavailable(a);
  assert.equal(a.current().messages.length, 0);
  assert.equal(a.fetches.length, 1);
  assert.equal(a.fetches[0].options.credentials, 'omit');
});

test('current successful downloads preserve Go UTF-8 bytes and original int64 JSON tokens exactly', async () => {
  const a = app();
  const { raw, response } = await successful(a);
  assert.equal(response.report.maximum, response.report.adjacent, 'fixture must reproduce Number rounding');
  assert.notEqual(JSON.stringify(response), raw, 'a parse/stringify round trip must change this response');
  a.downloadGo(); a.downloadResponse();
  assert.deepEqual(a.downloads.map(row => row.filename), ['gooo-generated.go', 'gooo-compiler-response.json']);
  assert.ok(a.downloads.every(row => row.hidden && row.attached));
  assert.equal(a.body.children.length, 0, 'temporary anchors are removed after each click');
  assert.equal(a.objectUrls[0].blob.type, 'text/plain;charset=utf-8');
  assert.equal(a.objectUrls[1].blob.type, 'application/json;charset=utf-8');
  assert.deepEqual(Buffer.from(await a.objectUrls[0].blob.arrayBuffer()), Buffer.from(generatedGo, 'utf8'));
  assert.deepEqual(Buffer.from(await a.objectUrls[1].blob.arrayBuffer()), Buffer.from(raw, 'utf8'));
  assert.equal(await a.objectUrls[1].blob.text(), raw, 'do not append a newline or reformat the JSON');
  assert.match(raw, /"maximum":9223372036854775807,"adjacent":9223372036854775806/);
  assert.match(raw, /"minimum":-9223372036854775808/);
  assert.match(raw, /"compiler_source_sha":"UNBOUND_LOCAL_SOURCE"/);
  assert.match(a.get('probe-download-status').textContent, /Download requested/);
  assert.equal(a.get('probe-download-status').hidden, false);
  assert.deepEqual(a.revokedUrls, [], 'successful download URLs must remain readable initially');
  a.fireTimers(1000);
  assert.deepEqual(a.revokedUrls, a.objectUrls.map(row => row.url));
  a.fireTimers(1000);
  assert.equal(a.revokedUrls.length, 2, 'URLs are revoked once only');
});

test('source, activity, and sample changes immediately invalidate previous downloads', async () => {
  const changes = [
    a => a.edit('probe-source', a.get('probe-source').value.replace('return 0', 'return 9')),
    a => a.edit('probe-activity', 'Missing'),
    a => a.sample('changed'),
    a => a.sample('clamp'),
  ];
  for (const change of changes) {
    const a = app(); await successful(a); a.downloadGo();
    change(a); assertUnavailable(a);
    assert.equal(a.get('probe-download-status').textContent, '');
    assert.equal(a.get('probe-download-status').hidden, true);
    assert.deepEqual(a.revokedUrls, [], 'input edits must not interrupt an already requested immutable Blob');
    a.fireTimers(1000); assert.equal(a.revokedUrls.length, 1);
  }
});

test('edit-and-restore needs fresh generation and download handlers independently compare current inputs', async () => {
  for (const id of ['probe-source', 'probe-activity']) {
    const a = app(); await successful(a);
    const original = a.get(id).value;
    a.edit(id, original + 'x'); a.edit(id, original);
    assertUnavailable(a);
    const request = await a.generate(); a.reply(request);
    assert.equal(a.get('probe-download-go').disabled, false);
    a.get(id).value = original + 'unannounced change';
    a.downloadGo(); a.downloadResponse();
    assert.equal(a.objectUrls.length, 0, 'guard must compare values even if no input event was dispatched');
  }
});

test('new generation clears downloads during source hashing; edits cancel preparation before dispatch', async () => {
  const options = {};
  const a = app(options); await successful(a);
  options.holdDigest = true;
  const pending = a.generate();
  await until(() => a.pendingDigests.length === 1);
  assertUnavailable(a);
  assert.equal(a.get('probe-generate').disabled, true);
  assert.equal(await a.generate(), null, 'cannot dispatch another generation while hashing');
  const original = a.get('probe-source').value;
  a.edit('probe-source', original + '\n'); a.edit('probe-source', original);
  a.pendingDigests[0].release();
  assert.equal(await pending, null);
  assert.equal(a.current().messages.length, 1, 'edited-and-restored preparation must not dispatch');
  assertUnavailable(a);
  options.holdDigest = false;
  a.reply(await a.generate());
  assert.equal(a.get('probe-download-response').disabled, false);
});

test('in-flight responses after source/activity/sample edits cannot restore old downloads', async () => {
  for (const change of [
    a => { const original = a.get('probe-source').value; a.edit('probe-source', original + '\n'); a.edit('probe-source', original); },
    a => { const original = a.get('probe-activity').value; a.edit('probe-activity', 'Missing'); a.edit('probe-activity', original); },
    a => { a.sample('changed'); a.sample('clamp'); },
  ]) {
    const a = app(); await successful(a);
    const stale = await a.generate(); assertUnavailable(a);
    change(a); a.reply(stale); assertUnavailable(a);
    const fresh = await a.generate();
    a.reply(stale); assertUnavailable(a);
    a.reply(fresh);
    assert.equal(a.get('probe-download-go').disabled, false);
    const before = a.get('probe-report').textContent;
    a.reply(stale);
    assert.equal(a.get('probe-report').textContent, before, 'late duplicate response cannot replace a fresh success');
  }
});

test('compiler rejection, generation error, and invalid input retain no downloadable success', async () => {
  for (const failure of [
    (a, request) => a.reply(request, { response: { ok: false, source: '', error: 'type mismatch' } }),
    (a, request) => a.reply(request, { message: { type: 'generation-error', message: 'generation failed' } }),
  ]) {
    const a = app(); await successful(a);
    const request = await a.generate(); failure(a, request);
    assertUnavailable(a); assert.ok(a.get('probe-error').textContent);
    a.reply(await a.generate());
    assert.equal(a.get('probe-download-response').disabled, false);
  }
  for (const [id, value] of [['probe-source', ''], ['probe-source', '\ud800'], ['probe-source', ' '.repeat(65537)], ['probe-activity', '']]) {
    const a = app(); await successful(a); a.edit(id, value);
    assert.equal(await a.generate(), null); assertUnavailable(a);
    assert.ok(a.get('probe-error').textContent);
  }
});

test('successful response requires string Go source and existing request/provenance bindings', async () => {
  for (const changes of [
    { response: { source: undefined } }, { response: { source: 7 } },
    { response: { compiler_pin: 'c'.repeat(40) } },
    { response: { bridge_source_sha256: 'c'.repeat(64) } },
    { response: { source_sha256: 'sha256:' + 'c'.repeat(64) } },
    { message: { sourceSha: 'sha256:' + 'c'.repeat(64) } },
    { message: { responseJSON: null } },
  ]) {
    const a = app(); await successful(a);
    a.reply(await a.generate(), changes);
    assertUnavailable(a);
    assert.equal(a.current().terminated, true);
    assert.equal(a.get('probe-retry').hidden, false);
  }
});

test('retry, worker failure, and generation timeout erase downloads until a fresh success', async () => {
  for (const stop of [
    a => a.retry(),
    a => a.current().deliver({ type: 'fatal', message: 'worker failed' }),
    a => a.current().onerror({ message: 'worker failed', preventDefault() {} }),
    a => a.current().onmessageerror(),
    async a => { await a.generate(); a.fireTimers(15000); },
  ]) {
    const a = app(); await successful(a);
    const oldWorker = a.current(); const stale = oldWorker.messages.at(-1);
    await stop(a); assertUnavailable(a);
    if (a.current() === oldWorker) a.retry();
    assert.equal(oldWorker.terminated, true);
    a.reply(stale, {}, oldWorker); assertUnavailable(a);
    await a.ready(); assertUnavailable(a);
    a.reply(await a.generate());
    assert.equal(a.get('probe-download-go').disabled, false);
  }
});

test('failure/retry and pagehide during hashing cannot dispatch or restore old preparation', async () => {
  for (const replaceWorker of [
    a => {
      a.current().deliver({ type: 'fatal', message: 'worker failed during source hashing' });
      assert.equal(a.get('probe-retry').hidden, false);
      a.retry();
    },
    a => { a.pagehide(); a.pageshow(); },
  ]) {
    const options = {};
    const a = app(options); await successful(a);
    options.holdDigest = true;
    const pending = a.generate(); await until(() => a.pendingDigests.length === 1);
    const oldWorker = a.current(); replaceWorker(a); await a.ready();
    assertUnavailable(a);
    options.holdDigest = false;
    a.reply(await a.generate());
    const fresh = a.get('probe-report').textContent;
    a.pendingDigests[0].release(); assert.equal(await pending, null);
    assert.equal(oldWorker.messages.length, 1);
    assert.equal(a.current().messages.length, 1, 'old preparation must not dispatch to the replacement worker');
    assert.equal(a.get('probe-report').textContent, fresh);
    assert.equal(a.get('probe-download-response').disabled, false);
  }
});

test('initialization timeout cannot enable download and retry recovers', async () => {
  const a = app(); a.fireTimers(60000); assertUnavailable(a);
  assert.equal(a.current().terminated, true);
  a.retry(); await a.ready(); a.reply(await a.generate());
  assert.equal(a.get('probe-download-go').disabled, false);
});

test('download start failures clean up URLs and anchors while retaining a valid result for retry', async () => {
  for (const failure of ['blobError', 'urlError', 'appendError', 'clickError']) {
    const options = { [failure]: true };
    const a = app(options); const { raw } = await successful(a);
    a.downloadResponse();
    assert.equal(a.downloads.length, 0);
    assert.equal(a.body.children.length, 0);
    assert.deepEqual(a.revokedUrls, a.objectUrls.map(row => row.url), failure + ': any created URL must be revoked immediately');
    assert.equal([...a.timers.values()].filter(timer => timer.delay === 1000).length, 0);
    assert.match(a.get('probe-download-status').textContent, /could not be started/);
    assert.equal(a.get('probe-output').textContent, generatedGo);
    assert.equal(a.get('probe-report').textContent, raw);
    assert.equal(a.get('probe-download-response').disabled, false);
    options[failure] = false; a.downloadResponse();
    assert.equal(a.downloads.length, 1);
    assert.equal(await a.objectUrls.at(-1).blob.text(), raw);
    assert.match(a.get('probe-download-status').textContent, /Download requested/);
    a.fireTimers(1000);
    assert.equal(new Set(a.revokedUrls).size, a.objectUrls.length);
  }
});

test('pagehide clears outputs and all outstanding URLs; BFCache return needs fresh generation', async () => {
  const a = app(); await successful(a);
  a.downloadGo(); a.downloadResponse(); a.downloadResponse();
  const oldWorker = a.current(); const stale = oldWorker.messages.at(-1);
  a.pagehide(); assertUnavailable(a);
  assert.equal(oldWorker.terminated, true);
  assert.deepEqual(a.revokedUrls, a.objectUrls.map(row => row.url));
  assert.equal(a.timers.size, 0);
  assert.equal(a.get('probe-download-status').hidden, true);
  a.fireTimers(1000); assert.equal(a.revokedUrls.length, 3);
  a.pageshow(); assert.notEqual(a.current(), oldWorker);
  a.reply(stale, {}, oldWorker); assertUnavailable(a);
  await a.ready(); assertUnavailable(a);
  a.reply(await a.generate());
  assert.equal(a.get('probe-download-go').disabled, false);
});
