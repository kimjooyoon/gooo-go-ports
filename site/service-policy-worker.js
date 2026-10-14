// A single isolated runtime dispatches only the three checked, pure APIs.
// No policy matching, source compilation, authorization or writes happen here.
((root, factory) => {
  if (typeof module === 'object' && module.exports) module.exports = factory(require('./service-policy-protocol.js'));
  else { root.importScripts('./service-policy-protocol.js'); factory(root.GoooServicePolicyProtocol).startWorker(root); }
})(globalThis, protocol => {
  'use strict';
  async function loadBrowserRuntime(scope, fatal) {
    const options = { cache: 'no-store', credentials: 'omit', redirect: 'error', referrerPolicy: 'no-referrer' };
    const response = await scope.fetch('./service-policy/manifest.json', options);
    if (!response.ok) throw new Error('The policy build manifest could not be loaded.');
    const program = protocol.parseMetadata(await response.text());
    const runtimeBytes = new Map();
    // Only paths in the protocol's exact published roster can reach fetch.
    // Hash every source as well as the executable before advertising readiness.
    const names = protocol.FILE_NAMES;
    for (let offset = 0; offset < names.length; offset += 6) {
      await Promise.all(names.slice(offset, offset + 6).map(async name => {
        const file = program.files[name];
        const fetched = await scope.fetch('./service-policy/' + name, options);
        if (!fetched.ok) throw new Error('A published policy file could not be loaded: ' + name);
        const bytes = await fetched.arrayBuffer();
        if (bytes.byteLength !== file.bytes) throw new Error('Published policy file size mismatch: ' + name);
        const digest = await scope.crypto.subtle.digest('SHA-256', bytes);
        const observed = Array.from(new Uint8Array(digest), value => value.toString(16).padStart(2, '0')).join('');
        if (observed !== file.sha256) throw new Error('Published policy file hash mismatch: ' + name);
        if (name === 'program.wasm' || name === 'wasm_exec.js') runtimeBytes.set(name, bytes);
      }));
    }
    const wasm = runtimeBytes.get('program.wasm');
    if (!scope.WebAssembly.validate(wasm)) throw new Error('The policy executable is not valid WebAssembly.');
    // Execute the exact runtime bytes just checked, never a second network read.
    const runtimeURL = scope.URL.createObjectURL(new scope.Blob([runtimeBytes.get('wasm_exec.js')], { type: 'text/javascript' }));
    try { scope.importScripts(runtimeURL); } finally { scope.URL.revokeObjectURL(runtimeURL); }
    if (typeof scope.Go !== 'function') throw new Error('The verified Go runtime is unavailable.');
    const go = new scope.Go();
    const { instance } = await scope.WebAssembly.instantiate(wasm, go.importObject);
    let stopped = false;
    const stoppedRuntime = message => { stopped = true; fatal(message); };
    const execution = go.run(instance);
    Promise.resolve(execution).then(() => stoppedRuntime('The compiled policy program stopped.'),
      () => stoppedRuntime('The compiled policy program stopped unexpectedly.'));
    // Go registers the callbacks synchronously before blocking in main.
    if (scope.goooServicePolicyReady !== true || typeof scope.goooServicePolicyEvaluate !== 'function' ||
        typeof scope.goooServicePolicyBindings !== 'function') throw new Error('The checked policy API is unavailable.');
    protocol.parseBindings(scope.goooServicePolicyBindings(), program);
    await Promise.resolve();
    if (stopped) throw new Error('The compiled policy program did not remain available.');
    return { program, run: (mode, arrays) => scope.goooServicePolicyEvaluate(mode, arrays) };
  }
  function startWorker(scope, load = loadBrowserRuntime) {
    let runtime = null, generation = 0, activeId = null, lastId = 0, stopped = false;
    function error(message, id) {
      const reply = { type: 'error', message: String(message || 'Policy evaluation failed.').slice(0, 240) };
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
        if (!runtime || stopped) throw new Error('The compiled policy program is not ready.');
        if (request.id <= lastId) throw new Error('Request IDs must increase.');
        if (activeId !== null) throw new Error('A policy evaluation is already active.');
        lastId = request.id; activeId = request.id;
        const token = ++generation, current = runtime;
        // Messages cannot interrupt synchronous Wasm. The page terminates the
        // Worker on input edits, cancellation and its execution deadline.
        await Promise.resolve();
        if (stopped || token !== generation) return;
        const raw = current.run(request.mode, protocol.packFacts(request.mode, request.facts));
        const proof = protocol.parseCompiledReply(raw, request.mode, request.facts, current.program);
        if (stopped || token !== generation) return;
        const result = { type: 'result', id: request.id, mode: request.mode, proof, program: current.program };
        protocol.validateResult(result, { ...request, program: current.program });
        scope.postMessage(result); activeId = null;
      } catch (failure) {
        if (!stopped) error(failure.message, message && message.id);
        if (message && message.id === activeId) activeId = null;
      }
    });
    const ready = Promise.resolve().then(() => load(scope, fatal)).then(value => {
      if (stopped) return;
      const program = protocol.validateMetadata(value.program);
      if (typeof value.run !== 'function') throw new Error('The compiled policy API is unavailable.');
      runtime = { program, run: value.run };
      scope.postMessage({ type: 'ready', program });
    }).catch(failure => fatal(failure.message));
    return { ready };
  }
  return Object.freeze({ loadBrowserRuntime, startWorker });
});
