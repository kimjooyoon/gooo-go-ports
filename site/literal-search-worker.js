// Execute the generated Report module in an isolated Worker. No search logic
// or generated source is rewritten in JavaScript.
((root, factory) => {
  if (typeof module === 'object' && module.exports) module.exports = factory(require('./literal-search-protocol.js'));
  else { root.importScripts('./literal-search-protocol.js'); factory(root.GoooLiteralSearchProtocol).startWorker(root); }
})(globalThis, protocol => {
  'use strict';
  async function loadBrowserRuntime(scope, fatal) {
    const metadataResponse = await scope.fetch('./literal-search/manifest.json', { cache: 'no-store' });
    if (!metadataResponse.ok) throw new Error('The search build manifest could not be loaded.');
    const program = protocol.validateMetadata(await metadataResponse.json());
    const response = await scope.fetch('./literal-search/program.wasm', { cache: 'no-store' });
    if (!response.ok) throw new Error('The compiled search program could not be loaded.');
    const bytes = await response.arrayBuffer();
    if (bytes.byteLength !== program.wasm_bytes) throw new Error('The program size differs from the manifest.');
    const digest = await scope.crypto.subtle.digest('SHA-256', bytes);
    const observed = Array.from(new Uint8Array(digest), value => value.toString(16).padStart(2, '0')).join('');
    if (observed !== program.wasm_sha256) throw new Error('The program hash differs from the manifest.');
    if (!scope.WebAssembly.validate(bytes)) throw new Error('The search program is not a valid WebAssembly module.');
    scope.importScripts('./wasm_exec.js');
    const go = new scope.Go();
    const { instance } = await scope.WebAssembly.instantiate(bytes, go.importObject);
    go.run(instance).then(() => fatal('The compiled search program stopped.'), () => fatal('The compiled search program stopped unexpectedly.'));
    if (scope.goooLiteralSearchReady !== true || typeof scope.goooLiteralSearch !== 'function') throw new Error('The compiled Report API is unavailable.');
    return { program, run: (...args) => scope.goooLiteralSearch(...args) };
  }
  function startWorker(scope, load = loadBrowserRuntime) {
    let runtime = null, generation = 0, activeId = null, lastId = 0, stopped = false;
    function error(message, id) {
      const reply = { type: 'error', message: String(message).slice(0, 240) || 'Search failed.' };
      if (Number.isSafeInteger(id) && id > 0) reply.id = id;
      scope.postMessage(reply);
    }
    function fatal(message) {
      if (stopped) return;
      stopped = true; generation++; runtime = null;
      error(message, activeId); activeId = null;
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
        if (!runtime || stopped) throw new Error('The compiled search program is not ready.');
        if (request.id <= lastId) throw new Error('Request IDs must increase.');
        lastId = request.id; activeId = request.id;
        const token = ++generation, currentRuntime = runtime;
        // Recheck the token after this microtask boundary. Worker message tasks
        // cannot interrupt a synchronous Wasm call; the page terminates its
        // Worker on cancellation, changed input or the execution deadline.
        await Promise.resolve();
        if (stopped || token !== generation) return;
        const raw = currentRuntime.run(...request.inputs);
        const output = protocol.parseCompiledReply(raw, request.inputs);
        if (stopped || token !== generation) return;
        const result = { type: 'result', id: request.id, inputs: request.inputs, output, program: currentRuntime.program };
        protocol.validateResult(result, { ...request, program: currentRuntime.program });
        scope.postMessage(result);
        activeId = null;
      } catch (failure) {
        if (!stopped) error(failure.message, message && message.id);
        if (message && message.id === activeId) activeId = null;
      }
    });
    const ready = Promise.resolve().then(() => load(scope, fatal)).then(value => {
      if (stopped) return;
      const program = protocol.validateMetadata(value.program);
      if (typeof value.run !== 'function') throw new Error('The compiled search API is unavailable.');
      runtime = { program, run: value.run };
      scope.postMessage({ type: 'ready', program });
    }).catch(failure => fatal(failure.message));
    return { ready };
  }
  return Object.freeze({ loadBrowserRuntime, startWorker });
});
