/* Experimental fixed-profile UI. No policy semantics.
 * Input authority is displayed editor text encoded as UTF-8, or untouched raw
 * file bytes. Parsed native outputs are only for display, never an alternative
 * decision or replay validator.
 */
'use strict';

(() => {
  const REQUEST_BYTES = 4 * 1024 * 1024, REPORT_BYTES = 16 * 1024 * 1024;
  const OUTPUT_BYTES = 64 * 1024 * 1024, DEADLINE_MS = 120000;
  const ENCODER = new TextEncoder(), UTF8 = new TextDecoder('utf-8', { fatal: true });
  const byID = id => document.getElementById(id);
  const ui = Object.fromEntries(['facts', 'facts-file', 'facts-origin', 'use-editor', 'sample-read', 'sample-write',
    'compare', 'report-file', 'report-origin', 'replay', 'cancel', 'status', 'result', 'result-origin', 'classification',
    'changed-count', 'decision-rows', 'result-facts', 'raw-report', 'runtime-info', 'save'].map(id => [id, byID(id)]));
  const FIELDS = ['allowed', 'reason', 'authenticated', 'tenant_present', 'tenant_matched', 'action_present',
    'action_matched', 'access_allowed', 'access_error', 'access_result', 'group_matched', 'principal_matched',
    'group_missing', 'principal_defaulted', 'effective_principal'];
  const TEXT_FIELDS = new Set(['reason', 'access_result', 'effective_principal']);
  const FACT_FIELDS = ['group_present', 'group_value', 'principal_present', 'principal_value', 'authenticated',
    'caller_tenant', 'resource_tenant', 'requested_action', 'allowed_action'];
  const FACT_BOOLEANS = new Set(['group_present', 'principal_present', 'authenticated']);
  const PROFILES = { original: 'cel-api-original-v1', candidate: 'cel-api-readonly-principal-v1' };
  const SAMPLE = action => '{\n  "group_present": true,\n  "group_value": "admin",\n  "principal_present": true,\n  "principal_value": "synthetic:user",\n  "authenticated": true,\n  "caller_tenant": "tenant-a",\n  "resource_tenant": "tenant-a",\n  "requested_action": "' + action + '",\n  "allowed_action": "' + action + '"\n}';

  let epoch = 0, nextID = 0, active = null;
  let factsFileBytes = null, factsFileLabel = '', savedReportBytes = null, savedReportLabel = '';
  let verifiedBytes = null, downloadURL = null;

  function exactObject(value, keys) {
    return value !== null && typeof value === 'object' && !Array.isArray(value) &&
      Object.keys(value).sort().join('|') === [...keys].sort().join('|');
  }
  function sameJSON(left, right) {
    if (left === right) return true;
    if (!left || !right || typeof left !== 'object' || typeof right !== 'object' || Array.isArray(left) !== Array.isArray(right)) return false;
    const keys = Object.keys(left).sort(), other = Object.keys(right).sort();
    return keys.length === other.length && keys.every((key, index) => key === other[index] && sameJSON(left[key], right[key]));
  }
  function status(message, state = '') { ui.status.textContent = message; ui.status.dataset.state = state; }
  function clearResult() {
    verifiedBytes = null;
    if (downloadURL) URL.revokeObjectURL(downloadURL);
    downloadURL = null; ui.save.disabled = true; ui.result.hidden = true;
    ui['decision-rows'].replaceChildren(); ui['raw-report'].textContent = ''; ui['result-facts'].textContent = '';
    ui.classification.textContent = ''; ui['changed-count'].textContent = ''; ui['result-origin'].textContent = '';
    if (factsFileBytes !== null) ui['facts-origin'].textContent = factsFileLabel + ' No current comparison result.';
    if (savedReportBytes !== null) ui['report-origin'].textContent = savedReportLabel + ' No current recheck result.';
  }
  function stopActive() {
    if (!active) return;
    active.controller.abort();
    if (active.worker) active.worker.terminate();
    if (active.workerURL) URL.revokeObjectURL(active.workerURL);
    clearTimeout(active.timer); active = null; ui.cancel.disabled = true;
  }
  function invalidate(message = 'Input changed. Run a fresh comparison or recheck.') {
    epoch++; stopActive(); clearResult(); status(message);
    ui.replay.disabled = savedReportBytes === null;
  }
  function failCurrent(token, message, incomplete = false) {
    if (!active || token !== epoch || active.token !== token) return;
    stopActive(); clearResult(); status(message, incomplete ? 'incomplete' : 'error');
  }
  function begin(message) {
    invalidate(message);
    const token = epoch, controller = new AbortController();
    active = { token, id: ++nextID, controller, worker: null, workerURL: null, timer: null };
    active.timer = setTimeout(() => failCurrent(token, 'Incomplete: the 120-second deadline expired. No result was retained.', true), DEADLINE_MS);
    ui.cancel.disabled = false;
    return active;
  }
  function current(job) { return active === job && job.token === epoch; }

  function clearFactsFile() {
    factsFileBytes = null; factsFileLabel = ''; ui['facts-file'].value = ''; ui.facts.disabled = false; ui['use-editor'].hidden = true;
    ui['facts-origin'].textContent = 'The displayed editor text is used. Browsers may fold raw line endings; use a file when exact input bytes matter.';
  }
  // The displayed textarea value is the sole editor authority. Browser editing
  // may fold raw CR/CRLF whitespace. Exact raw bytes use the separate file path;
  // do not reconstruct ambiguous edits or keep an invisible alternate string.
  ui.facts.addEventListener('input', () => { clearFactsFile(); invalidate(); });

  function encodeEditor() {
    const text = ui.facts.value;
    if (text.length < 1 || text.length > REQUEST_BYTES) throw new Error('The editor JSON must fit within the 4 MiB request limit.');
    // TextEncoder replaces isolated UTF-16 surrogates. Reject them first so the
    // native decoder never sees a silently changed value. Literal JSON escapes
    // such as backslash-uD800 are kept as ASCII and checked by Go instead.
    for (let i = 0; i < text.length; i++) {
      const unit = text.charCodeAt(i);
      if (unit >= 0xd800 && unit <= 0xdbff) {
        const next = text.charCodeAt(++i);
        if (!(next >= 0xdc00 && next <= 0xdfff)) throw new Error('The editor contains an unpaired UTF-16 surrogate. Nothing was encoded or run.');
      } else if (unit >= 0xdc00 && unit <= 0xdfff) throw new Error('The editor contains an unpaired UTF-16 surrogate. Nothing was encoded or run.');
    }
    const raw = ENCODER.encode(text);
    if (raw.byteLength > REQUEST_BYTES) throw new Error('The encoded JSON exceeds the 4 MiB request limit.');
    return raw;
  }

  async function loadFile(kind, input, limit) {
    invalidate('Reading raw file bytes. This does not run a policy or recheck.');
    if (kind === 'facts') {
      factsFileBytes = null; factsFileLabel = ''; ui.facts.disabled = false; ui['use-editor'].hidden = true;
      ui['facts-origin'].textContent = 'No raw facts file loaded. The displayed editor text will be used; browsers may fold raw line endings.';
    }
    else { savedReportBytes = null; savedReportLabel = ''; ui.replay.disabled = true; ui['report-origin'].textContent = 'No report loaded.'; }
    const file = input.files && input.files[0];
    if (!file) { status('No file selected.'); return; }
    if (!Number.isSafeInteger(file.size) || file.size < 1 || file.size > limit) {
      status('The selected file is empty or exceeds the ' + (kind === 'facts' ? '4' : '16') + ' MiB byte limit.', 'error'); return;
    }
    const job = begin('Reading raw file bytes. No computation has started.');
    try {
      const buffer = await file.arrayBuffer();
      if (!current(job)) return;
      if (buffer.byteLength !== file.size || buffer.byteLength < 1 || buffer.byteLength > limit) throw new Error('The file bytes do not match the declared size.');
      const raw = new Uint8Array(buffer);
      stopActive();
      // Do not decode, parse, pretty-print, or roundtrip file bytes via textarea.
      if (kind === 'facts') {
        factsFileBytes = raw; ui.facts.disabled = true; ui['use-editor'].hidden = false;
        factsFileLabel = 'Raw facts file loaded: ' + file.name + ' (' + raw.byteLength + ' bytes). The editor above is not used.';
        ui['facts-origin'].textContent = factsFileLabel + ' No current comparison result.';
      } else {
        savedReportBytes = raw; ui.replay.disabled = false;
        savedReportLabel = 'Raw report loaded: ' + file.name + ' (' + raw.byteLength + ' bytes).';
        ui['report-origin'].textContent = savedReportLabel + ' No current recheck result.';
      }
      status('File loaded, unverified. Choose an explicit comparison or recheck.');
    } catch (error) {
      if (current(job)) failCurrent(job.token, error.message || 'The selected file could not be read.');
    }
  }

  async function boundedFetch(url, limit, signal, declaredSize) {
    const target = new URL(url, location.href);
    if (target.origin !== location.origin || target.search || target.hash || !/^https?:$/.test(target.protocol)) throw new Error('A package path is not same-origin HTTP(S).');
    const response = await fetch(target.href, { cache: 'no-store', credentials: 'omit', redirect: 'error', referrerPolicy: 'no-referrer', signal });
    if (!response.ok || !response.body) throw new Error('A required local package file could not be loaded.');
    const reader = response.body.getReader(), chunks = [];
    let size = 0;
    try {
      for (;;) {
        const next = await reader.read(); if (next.done) break;
        size += next.value.byteLength;
        if (size > limit) throw new Error('A package file exceeds its declared byte bound.');
        chunks.push(next.value);
      }
    } finally { reader.releaseLock(); }
    if (declaredSize !== undefined && size !== declaredSize) throw new Error('A package file size differs from the manifest.');
    const bytes = new Uint8Array(size); let offset = 0;
    for (const chunk of chunks) { bytes.set(chunk, offset); offset += chunk.byteLength; }
    return bytes;
  }
  async function digest(bytes) {
    return [...new Uint8Array(await crypto.subtle.digest('SHA-256', bytes))].map(n => n.toString(16).padStart(2, '0')).join('');
  }
  function binding(value, count, generation) {
    return exactObject(value, ['compiler_source_sha', 'generation_sha256', 'assets']) &&
      value.compiler_source_sha === '83928fa4538245abe33b08549da47f8cbdc50ed2' && value.generation_sha256 === generation &&
      value.assets && typeof value.assets === 'object' && !Array.isArray(value.assets) && Object.keys(value.assets).length === count &&
      Object.entries(value.assets).every(([name, hash]) => name && !name.split('/').includes('..') && /^sha256:[0-9a-f]{64}$/.test(hash));
  }
  function validateManifest(manifest) {
    if (!exactObject(manifest, ['schema', 'scope', 'source', 'bridge', 'build_verification', 'files', 'expected_info', 'limits']) ||
        manifest.schema !== 'gooo/browser-api-policy-revision/v1' ||
        manifest.scope !== 'unsigned_historical_hypothetical_comparison' ||
        !exactObject(manifest.files, ['worker.js', 'program.wasm', 'wasm_exec.js'])) throw new Error('The UI manifest schema or runtime roster is invalid.');
    const paths = { 'worker.js': '../api-policy-worker.js', 'program.wasm': './program.wasm', 'wasm_exec.js': './wasm_exec.js' };
    for (const [name, file] of Object.entries(manifest.files)) {
      if (!exactObject(file, ['path', 'bytes', 'sha256']) || file.path !== paths[name] ||
          !Number.isSafeInteger(file.bytes) || file.bytes < 1 || file.bytes > (name === 'program.wasm' ? 32 * 1024 * 1024 : 128 * 1024) ||
          !/^[0-9a-f]{64}$/.test(file.sha256)) throw new Error('The UI manifest contains an invalid asset declaration.');
    }
    const info = manifest.expected_info;
    const limits = { text_utf8_bytes: 65536, baseline_input_bytes: REQUEST_BYTES, compare_input_bytes: REQUEST_BYTES,
      replay_input_bytes: REPORT_BYTES, original_proof_bytes: REQUEST_BYTES, native_facts_raw_bytes: 3539232, output_bytes: OUTPUT_BYTES };
    if (!exactObject(info, ['schema', 'scope', 'modes', 'profiles', 'limits', 'bindings']) ||
        info.schema !== 'gooo/api-policy-browser-bridge-info/v1' || info.scope !== 'unsigned_historical_hypothetical_comparison' ||
        !sameJSON(info.modes, ['baseline', 'compare', 'replay']) || !sameJSON(info.profiles, PROFILES) || !sameJSON(info.limits, limits) ||
        !exactObject(info.bindings, ['original', 'revision']) ||
        !binding(info.bindings.original, 28, 'sha256:79b76bda704cb63b9390802766a66ac72ba02c04ae135600cb05de6797c97d46') ||
        !binding(info.bindings.revision, 38, 'sha256:24f37267a7f4ad723b7228eeecd55fdc151c8534fa8a95bca19bb73d9b3973d0')) throw new Error('The UI manifest changes the fixed native profiles, complete bindings or limits.');
    return manifest;
  }

  function displayReport(raw, manifest, operation) {
    if (raw.byteLength < 1 || raw.byteLength > OUTPUT_BYTES) throw new Error('The complete native response exceeds its transport limit.');
    const text = UTF8.decode(raw), report = JSON.parse(text);
    if (!exactObject(report, ['schema', 'status', 'scope', 'before_profile', 'candidate_profile', 'before', 'after', 'differences', 'summary', 'classifier_bindings']) ||
        report.schema !== 'gooo/api-policy-revision-comparison/v1' || report.status !== 'COMPARED' || report.scope !== 'same_saved_facts_two_sealed_profiles' ||
        report.before_profile !== PROFILES.original || report.candidate_profile !== PROFILES.candidate ||
        !exactObject(report.differences, FIELDS) || !FIELDS.every(field => typeof report.differences[field] === 'boolean') ||
        !exactObject(report.summary, ['classification', 'changed_count']) || typeof report.summary.classification !== 'string' ||
        !Number.isSafeInteger(report.summary.changed_count) || report.summary.changed_count < 0 || report.summary.changed_count > 15 ||
        !sameJSON(report.classifier_bindings, manifest.expected_info.bindings.revision)) throw new Error('The native report shape or complete classifier bindings do not match this UI.');
    for (const proof of [report.before, report.after]) {
      if (!exactObject(proof, ['facts', 'decision', 'bindings']) || !exactObject(proof.facts, FACT_FIELDS) ||
          !FACT_FIELDS.every(field => typeof proof.facts[field] === (FACT_BOOLEANS.has(field) ? 'boolean' : 'string')) ||
          !exactObject(proof.decision, FIELDS) || !FIELDS.every(field => typeof proof.decision[field] === (TEXT_FIELDS.has(field) ? 'string' : 'boolean')))
        throw new Error('The complete native facts or fifteen-field decisions are unavailable.');
    }
    if (!sameJSON(report.before.bindings, manifest.expected_info.bindings.original) ||
        !sameJSON(report.after.bindings, manifest.expected_info.bindings.revision)) throw new Error('The native decision profile bindings do not match.');
    // Display native values verbatim. Do not derive flags, counts or decisions.
    const rows = document.createDocumentFragment();
    for (const field of FIELDS) {
      const row = document.createElement('tr');
      for (const value of [field, report.before.decision[field], report.after.decision[field], report.differences[field]]) {
        const cell = document.createElement('td'); cell.textContent = String(value); row.append(cell);
      }
      row.lastChild.dataset.changed = String(report.differences[field]); rows.append(row);
    }
    ui['decision-rows'].replaceChildren(rows);
    ui.classification.textContent = report.summary.classification; ui['changed-count'].textContent = String(report.summary.changed_count);
    ui['result-facts'].textContent = JSON.stringify(report.before.facts, null, 2); ui['raw-report'].textContent = text;
    ui['result-origin'].textContent = operation === 'replay' ? 'The selected saved report was fully recalculated by Go. Saving keeps the exact new native response bytes.' :
      'Go created a fresh original baseline and compared the read-only candidate using the same supplied facts.';
    verifiedBytes = raw; ui.result.hidden = false; ui.save.disabled = false;
    if (operation === 'compare' && factsFileBytes !== null) ui['facts-origin'].textContent = factsFileLabel + ' Compared by Go; complete result below.';
    if (operation === 'replay') ui['report-origin'].textContent = savedReportLabel + ' Rechecked by Go; complete result below.';
  }

  async function run(operation) {
    const job = begin('Preparing an isolated native run…');
    try {
      if (!globalThis.Worker || !globalThis.crypto || !crypto.subtle || !globalThis.WebAssembly) throw new Error('This browser lacks required Worker, WebCrypto or WebAssembly support.');
      // Clone a file snapshot before transfer. The retained raw input survives
      // cancellation and is never decoded/reserialized by JavaScript.
      const raw = operation === 'compare' ? (factsFileBytes ? new Uint8Array(factsFileBytes) : encodeEditor()) :
        (savedReportBytes ? new Uint8Array(savedReportBytes) : null);
      if (!raw) throw new Error('Load a saved report first.');
      const manifestURL = new URL('./api-policy/manifest.json', location.href);
      const manifest = validateManifest(JSON.parse(UTF8.decode(await boundedFetch(manifestURL, 512 * 1024, job.controller.signal))));
      if (!current(job)) return;
      ui['runtime-info'].textContent = JSON.stringify({ source: manifest.source, bridge: manifest.bridge,
        runtime_files: manifest.files, native: manifest.expected_info }, null, 2);
      const workerFile = manifest.files['worker.js'];
      const workerBytes = await boundedFetch(new URL(workerFile.path, manifestURL), 128 * 1024, job.controller.signal, workerFile.bytes);
      if (await digest(workerBytes) !== workerFile.sha256) throw new Error('Worker source hash mismatch.');
      if (!current(job)) return;
      // Execute exactly the bytes just hashed, not a new Worker URL fetch.
      job.workerURL = URL.createObjectURL(new Blob([workerBytes], { type: 'text/javascript' }));
      job.worker = new Worker(job.workerURL);
      const worker = job.worker;
      worker.addEventListener('error', event => { event.preventDefault(); failCurrent(job.token, 'The isolated runtime stopped. No result was retained.', true); });
      worker.addEventListener('messageerror', () => failCurrent(job.token, 'The isolated runtime returned an unreadable message. No result was retained.', true));
      worker.addEventListener('message', event => {
        if (!current(job) || worker !== job.worker) return;
        const message = event.data;
        if (!message || message.id !== job.id) return;
        if (message.type === 'phase' && typeof message.message === 'string') { status(message.message); return; }
        if (message.type === 'failure' && typeof message.message === 'string' && typeof message.incomplete === 'boolean') {
          failCurrent(job.token, (message.incomplete ? 'Incomplete: ' : 'Native refusal: ') + message.message, message.incomplete); return;
        }
        try {
          if (!exactObject(message, ['type', 'id', 'operation', 'bytes']) || message.type !== 'result' || message.operation !== operation ||
              Object.getPrototypeOf(message.bytes) !== ArrayBuffer.prototype || message.bytes.byteLength < 1 || message.bytes.byteLength > OUTPUT_BYTES)
            throw new Error('The isolated runtime returned an invalid result transport.');
          const bytes = new Uint8Array(message.bytes);
          displayReport(bytes, manifest, operation);
          stopActive(); status('Completed native recalculation. The full report is ready to inspect or save.', 'complete');
        } catch (error) { failCurrent(job.token, error.message || 'The native result could not be displayed safely.'); }
      });
      const asset = name => ({ ...manifest.files[name], url: new URL(manifest.files[name].path, manifestURL).href });
      worker.postMessage({ id: job.id, operation, bytes: raw.buffer, config: {
        program: asset('program.wasm'), runtime: asset('wasm_exec.js'), expected_info: manifest.expected_info
      } }, [raw.buffer]);
    } catch (error) {
      if (current(job)) failCurrent(job.token, error.message || 'The native run could not start.');
    }
  }

  ui['sample-read'].addEventListener('click', () => { clearFactsFile(); ui.facts.value = SAMPLE('read'); invalidate('Read sample loaded. Click Compare policies.'); });
  ui['sample-write'].addEventListener('click', () => { clearFactsFile(); ui.facts.value = SAMPLE('write'); invalidate('Write sample loaded. Click Compare policies.'); });
  ui['use-editor'].addEventListener('click', () => { clearFactsFile(); invalidate('Editor selected. No saved result is current.'); });
  ui['facts-file'].addEventListener('change', () => { void loadFile('facts', ui['facts-file'], REQUEST_BYTES); });
  ui['report-file'].addEventListener('change', () => { void loadFile('report', ui['report-file'], REPORT_BYTES); });
  ui.compare.addEventListener('click', () => { void run('compare'); });
  ui.replay.addEventListener('click', () => { void run('replay'); });
  ui.cancel.addEventListener('click', () => { invalidate('Cancelled. No result was retained.'); ui.status.dataset.state = 'incomplete'; });
  ui.save.addEventListener('click', () => {
    if (!verifiedBytes || active || ui.result.hidden) return;
    if (downloadURL) URL.revokeObjectURL(downloadURL);
    downloadURL = URL.createObjectURL(new Blob([verifiedBytes], { type: 'application/json' }));
    const anchor = document.createElement('a'); anchor.href = downloadURL; anchor.download = 'gooo-api-policy-comparison.json';
    document.body.append(anchor); anchor.click(); anchor.remove();
    status('Download requested for the exact recalculated native bytes.');
  });
  window.addEventListener('pagehide', () => { epoch++; stopActive(); clearResult(); });
  ui.facts.value = SAMPLE('read');
})();
