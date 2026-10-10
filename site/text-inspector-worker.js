// Browser application composition of existing Gooo ports, not a Gooo string API
// or the native whitespace-inspector activity. Generated code is never changed.
((root, factory) => {
  if (typeof module === 'object' && module.exports) {
    module.exports = factory(require('./text-inspector-protocol.js'));
  } else {
    root.importScripts('./text-inspector-protocol.js');
    factory(root.GoooTextInspectorProtocol).startWorker(root);
  }
})(globalThis, (protocol) => {
  'use strict';
  const chunkSize = 16;
  const nextTurn = () => new Promise(resolve => setTimeout(resolve, 0));
  function cancelled() { const error = new Error('Inspection cancelled.'); error.name = 'AbortError'; return error; }

  async function inspectPoints(points, run, options = {}) {
    points = protocol.validatePoints(points); // Validate the whole request before the first call.
    if (typeof run !== 'function') throw new Error('Compiled program is unavailable.');
    const current = options.isCurrent || (() => true);
    const yieldControl = options.yieldControl || nextTurn;
    const rows = [];
    for (let index = 0; index < points.length; index++) {
      if (!current()) throw cancelled();
      const input = points[index];
      const values = protocol.OPERATIONS.map(operation =>
        protocol.parseCompiledReply(run(operation, input, '0'), operation, input));
      if (!current()) throw cancelled();
      rows.push({ input, validScalar: values[0], goSpace: values[1], utf8Bytes: values[2], utf16Units: values[3], goLetter: values[4], goDigit: values[5] });
      const completed = index + 1;
      if (completed % chunkSize === 0 || completed === points.length) {
        if (options.onProgress) options.onProgress(completed, points.length);
        if (completed < points.length) await yieldControl();
      }
    }
    if (!current()) throw cancelled();
    return rows;
  }

  async function loadBrowserRuntime(scope, onFatal) {
    const metadataResponse = await scope.fetch('./metadata.json');
    if (!metadataResponse.ok) throw new Error('Program metadata could not be loaded.');
    const metadata = await metadataResponse.json();
    const identity = protocol.validateMetadata(metadata);
    const wasmResponse = await scope.fetch('./demo.wasm');
    if (!wasmResponse.ok) throw new Error('The Gooo program could not be loaded.');
    const bytes = await wasmResponse.arrayBuffer();
    if (bytes.byteLength !== metadata.wasm_bytes) throw new Error('The Gooo program size does not match its metadata.');
    const digest = await scope.crypto.subtle.digest('SHA-256', bytes);
    const observed = Array.from(new Uint8Array(digest), value => value.toString(16).padStart(2, '0')).join('');
    if (observed !== identity.wasmSha256) throw new Error('The Gooo program hash does not match its metadata.');
    scope.importScripts('./wasm_exec.js');
    const go = new scope.Go();
    const { instance } = await scope.WebAssembly.instantiate(bytes, go.importObject);
    go.run(instance).then(() => onFatal('The Gooo program stopped.'), () => onFatal('The Gooo program stopped unexpectedly.'));
    if (scope.goooReady !== true || typeof scope.goooRun !== 'function') throw new Error('The Gooo program did not expose its existing API.');
    return { identity, run: (...args) => scope.goooRun(...args) };
  }

  function startWorker(scope, load = loadBrowserRuntime) {
    let runtime = null;
    let generation = 0;
    let activeId = null;
    let lastId = 0;
    let stopped = false;
    function error(message, id) {
      const reply = { type: 'error', message: String(message).slice(0, 240) || 'Inspection failed.' };
      if (Number.isSafeInteger(id) && id > 0) reply.id = id;
      scope.postMessage(reply);
    }
    function fatal(message) {
      if (stopped) return;
      stopped = true;
      generation++;
      error(message, activeId);
      runtime = null;
      activeId = null;
    }
    scope.addEventListener('message', async event => {
      const message = event.data;
      try {
        if (message && message.type === 'cancel') {
          const id = protocol.validateCancel(message);
          if (id === activeId) { generation++; activeId = null; }
          return;
        }
        const request = protocol.validateRequest(message);
        if (!runtime || stopped) throw new Error('The Gooo program is not ready.');
        if (request.id <= lastId) throw new Error('Request IDs must increase.');
        lastId = request.id;
        activeId = request.id;
        const token = ++generation;
        const currentRuntime = runtime;
        const isCurrent = () => !stopped && token === generation;
        try {
          const rows = await inspectPoints(request.points, currentRuntime.run, { isCurrent,
            onProgress: (completed, total) => { if (isCurrent()) scope.postMessage({ type: 'progress', id: request.id, completed, total }); } });
          if (!isCurrent()) return;
          const reply = { type: 'result', id: request.id, rows, ...currentRuntime.identity };
          protocol.validateResult(reply, { ...request, ...currentRuntime.identity });
          scope.postMessage(reply);
        } catch (failure) {
          if (isCurrent() && failure.name !== 'AbortError') error(failure.message, request.id);
        } finally {
          if (isCurrent()) activeId = null;
        }
      } catch (failure) {
        error(failure.message, message && message.id);
      }
    });
    const ready = Promise.resolve().then(() => load(scope, fatal)).then(value => {
      if (stopped) return;
      protocol.validateReady({ type: 'ready', ...value.identity });
      if (typeof value.run !== 'function') throw new Error('Compiled program is unavailable.');
      runtime = value;
      scope.postMessage({ type: 'ready', ...value.identity });
    }).catch(failure => fatal(failure.message));
    return { ready };
  }
  return Object.freeze({ inspectPoints, loadBrowserRuntime, startWorker });
});
