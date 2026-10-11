((root, factory) => {
  if (typeof module === 'object' && module.exports) module.exports = factory(require('./tutorial-protocol.js'));
  else { root.importScripts('./tutorial-protocol.js'); factory(root.GoooTutorialProtocol).startWorker(root); }
})(globalThis, protocol => {
  'use strict';
  async function loadRuntime(scope, fatal) {
    if (!scope.crypto?.subtle || !scope.WebAssembly) throw new Error('Use HTTPS or localhost in a browser with WebAssembly support.');
    const options = { cache: 'no-store' };
    const manifest = await scope.fetch('./tutorial/manifest.json', options);
    if (!manifest.ok) throw new Error('The tutorial build information could not be loaded.');
    const metadata = protocol.validateMetadata(await manifest.json());
    const response = await scope.fetch('./tutorial/program.wasm', options);
    if (!response.ok) throw new Error('The tutorial program could not be loaded.');
    const bytes = await response.arrayBuffer();
    if (bytes.byteLength !== metadata.wasm_bytes) throw new Error('The tutorial program size does not match this build.');
    const digest = await scope.crypto.subtle.digest('SHA-256', bytes);
    const hash = Array.from(new Uint8Array(digest), value => value.toString(16).padStart(2, '0')).join('');
    if (hash !== metadata.wasm_sha256) throw new Error('The tutorial program hash does not match this build. Retry loading.');
    scope.importScripts('./wasm_exec.js');
    const go = new scope.Go();
    const { instance } = await scope.WebAssembly.instantiate(bytes, go.importObject);
    Promise.resolve(go.run(instance)).then(() => fatal('The program stopped. Retry loading.'), () => fatal('The program stopped unexpectedly. Retry loading.'));
    if (scope.goooTutorialReady !== true || typeof scope.goooTutorial !== 'function') throw new Error('The program did not expose its tutorial API.');
    return { metadata, run: (...inputs) => scope.goooTutorial(...inputs) };
  }
  function startWorker(scope, load = loadRuntime) {
    let runtime, stopped = false, lastId = 0;
    function fatal(message) {
      if (stopped) return;
      stopped = true; runtime = null;
      scope.postMessage({ type: 'error', fatal: true, message: String(message).slice(0, 500) || 'The program is unavailable.' });
    }
    scope.addEventListener('message', event => {
      const message = event.data;
      try {
        const request = protocol.validateRequest(message);
        if (stopped || !runtime) throw new Error('The program is not ready.');
        if (request.id <= lastId) throw new Error('Request IDs must increase.');
        lastId = request.id;
        const result = protocol.parseReply(runtime.run(...request.inputs), request.inputs);
        if (!stopped) scope.postMessage({ type: 'result', id: request.id, wasmSha256: runtime.metadata.wasm_sha256, result });
      } catch (error) {
        const reply = { type: 'error', fatal: false, message: String(error.message).slice(0, 500) || 'Execution failed.' };
        if (Number.isSafeInteger(message?.id) && message.id > 0) reply.id = message.id;
        scope.postMessage(reply);
      }
    });
    const ready = Promise.resolve().then(() => load(scope, fatal)).then(value => {
      if (stopped) return;
      protocol.validateMetadata(value.metadata);
      if (typeof value.run !== 'function') throw new Error('The tutorial API is unavailable.');
      runtime = value;
      scope.postMessage({ type: 'ready', metadata: value.metadata });
    }).catch(error => fatal(error.message));
    return { ready };
  }
  return Object.freeze({ loadRuntime, startWorker });
});
