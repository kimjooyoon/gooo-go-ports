/* Fixed-profile API policy package. One fresh Worker handles one user request.
 * Policy, difference flags, classification and saved replay come only from Go.
 */
'use strict';

const INPUT_LIMIT = 4 * 1024 * 1024;
const REPORT_LIMIT = 16 * 1024 * 1024;
const OUTPUT_LIMIT = 64 * 1024 * 1024;
const UTF8 = new TextDecoder('utf-8', { fatal: true });
let started = false;
let finished = false;
let activeID = null;

class NativeRefusal extends Error {}

function fail(message, incomplete = true) {
  if (finished) return;
  finished = true;
  self.postMessage({ type: 'failure', id: activeID, incomplete, message: String(message).slice(0, 1200) });
}

function phase(message) {
  if (!finished) self.postMessage({ type: 'phase', id: activeID, message });
}

function exactObject(value, keys) {
  return value !== null && typeof value === 'object' && !Array.isArray(value) &&
    Object.keys(value).sort().join('|') === [...keys].sort().join('|');
}

function sameJSON(left, right) {
  if (left === right) return true;
  if (!left || !right || typeof left !== 'object' || typeof right !== 'object') return false;
  if (Array.isArray(left) !== Array.isArray(right)) return false;
  const keys = Object.keys(left).sort(), other = Object.keys(right).sort();
  return keys.length === other.length && keys.every((key, index) => key === other[index] && sameJSON(left[key], right[key]));
}

async function checkedBytes(url, descriptor) {
  const target = new URL(url);
  if (target.origin !== self.location.origin || target.search || target.hash ||
      !/^https?:$/.test(target.protocol)) throw new Error('A runtime path is outside this static package.');
  if (!Number.isSafeInteger(descriptor.bytes) || descriptor.bytes < 1 || descriptor.bytes > 32 * 1024 * 1024 ||
      !/^[0-9a-f]{64}$/.test(descriptor.sha256)) throw new Error('Invalid runtime asset declaration.');
  const response = await fetch(target.href, { cache: 'no-store', credentials: 'omit', redirect: 'error', referrerPolicy: 'no-referrer' });
  if (!response.ok || !response.body) throw new Error('A checked runtime asset could not be loaded.');
  const reader = response.body.getReader(), bytes = new Uint8Array(descriptor.bytes);
  let offset = 0;
  try {
    for (;;) {
      const next = await reader.read();
      if (next.done) break;
      if (offset + next.value.byteLength > bytes.byteLength) throw new Error('Runtime asset size mismatch.');
      bytes.set(next.value, offset); offset += next.value.byteLength;
    }
  } finally { reader.releaseLock(); }
  if (offset !== bytes.byteLength) throw new Error('Runtime asset size mismatch.');
  const observed = [...new Uint8Array(await crypto.subtle.digest('SHA-256', bytes))].map(n => n.toString(16).padStart(2, '0')).join('');
  if (observed !== descriptor.sha256) throw new Error('Runtime asset hash mismatch.');
  return bytes;
}

function fullBytes(value, limit) {
  if (!(value instanceof Uint8Array) || Object.getPrototypeOf(value) !== Uint8Array.prototype ||
      Object.getPrototypeOf(value.buffer) !== ArrayBuffer.prototype || value.byteOffset !== 0 ||
      value.byteLength < 1 || value.byteLength !== value.buffer.byteLength || value.byteLength > limit ||
      (typeof value.buffer.resizable === 'boolean' && value.buffer.resizable)) throw new Error('Native byte transport is invalid.');
  return value;
}

async function loadRuntime(config) {
  if (!exactObject(config, ['program', 'runtime', 'expected_info'])) throw new Error('Invalid runtime configuration.');
  for (const [key, name] of [['program', 'program.wasm'], ['runtime', 'wasm_exec.js']]) {
    const descriptor = config[key];
    if (!exactObject(descriptor, ['path', 'bytes', 'sha256', 'url']) || descriptor.path !== './' + name ||
        !new URL(descriptor.url).pathname.endsWith('/api-policy/' + name)) throw new Error('Unexpected runtime asset path.');
  }
  const [wasm, runtime] = await Promise.all([
    checkedBytes(config.program.url, config.program), checkedBytes(config.runtime.url, config.runtime)
  ]);
  if (!WebAssembly.validate(wasm)) throw new Error('The checked program is not valid WebAssembly.');
  // importScripts executes the exact verified byte snapshot, not a second GET.
  const runtimeURL = URL.createObjectURL(new Blob([runtime], { type: 'text/javascript' }));
  try { importScripts(runtimeURL); } finally { URL.revokeObjectURL(runtimeURL); }
  if (typeof self.Go !== 'function') throw new Error('The checked Go runtime is unavailable.');
  const go = new self.Go();
  const { instance } = await WebAssembly.instantiate(wasm, go.importObject);
  const lifecycle = go.run(instance);
  Promise.resolve(lifecycle).then(() => fail('Native runtime exited; no completed result is available.'),
    () => fail('Native runtime stopped; no completed result is available.'));
  if (typeof self.goooAPIRevision !== 'function') throw new Error('The native API was not registered.');
  const info = JSON.parse(UTF8.decode(fullBytes(self.goooAPIRevisionInfo, OUTPUT_LIMIT)));
  if (!sameJSON(info, config.expected_info)) throw new Error('Complete native profile bindings or limits differ from the manifest.');
  await Promise.resolve();
  if (finished) throw new Error('The native runtime did not remain available.');
  return info;
}

function callNative(mode, raw) {
  const response = self.goooAPIRevision(mode, fullBytes(raw, mode === 'replay' ? REPORT_LIMIT : INPUT_LIMIT));
  if (!response || response.ok !== true) {
    if (!exactObject(response, ['ok', 'error']) || response.ok !== false || typeof response.error !== 'string')
      throw new Error('The native API returned an invalid refusal.');
    throw new NativeRefusal(response.error);
  }
  if (!exactObject(response, ['ok', 'data'])) throw new Error('The native API returned an invalid success envelope.');
  return fullBytes(response.data, OUTPUT_LIMIT);
}

self.addEventListener('message', async event => {
  if (started) return;
  started = true;
  try {
    const message = event.data;
    if (!exactObject(message, ['id', 'operation', 'bytes', 'config']) ||
        !Number.isSafeInteger(message.id) || message.id < 1 || !['compare', 'replay'].includes(message.operation) ||
        Object.getPrototypeOf(message.bytes) !== ArrayBuffer.prototype ||
        (typeof message.bytes.resizable === 'boolean' && message.bytes.resizable)) throw new Error('Invalid owned request transport.');
    activeID = message.id;
    const raw = fullBytes(new Uint8Array(message.bytes), message.operation === 'replay' ? REPORT_LIMIT : INPUT_LIMIT);
    phase('Loading and checking the native runtime…');
    await loadRuntime(message.config);
    if (finished) return;
    let output;
    if (message.operation === 'compare') {
      phase('Computing the original baseline…');
      const baseline = callNative('baseline', raw);
      phase('Comparing both fixed profiles…');
      // Baseline already returns the complete native CompareRequest, including
      // both profile IDs and its original proof. Pass those exact bytes along.
      output = callNative('compare', baseline);
    } else {
      phase('Recalculating the complete saved report…');
      output = callNative('replay', raw);
    }
    if (finished) return;
    finished = true;
    self.postMessage({ type: 'result', id: activeID, operation: message.operation, bytes: output.buffer }, [output.buffer]);
  } catch (error) { fail(error && error.message ? error.message : 'The native operation could not complete.', !(error instanceof NativeRefusal)); }
});
