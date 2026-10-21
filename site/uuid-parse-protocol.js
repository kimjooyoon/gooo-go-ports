// Reuse the established exact Text binding; UUID parsing belongs exclusively
// to the unchanged compiled Gooo ParseCanonical activity.
((root, factory) => {
  if (typeof module === 'object' && module.exports) module.exports = factory(require('./literal-search-protocol.js'));
  else root.GoooUUIDParseProtocol = factory(root.GoooLiteralSearchProtocol);
})(globalThis, text => {
  'use strict';
  const fields = Object.freeze(['parsed', 'canonical_uuid', 'error', 'invalid_length']);
  const entry = 'gooo://github.com/google/uuid/text/parse-canonical';
  const result = 'gooo://github.com/google/uuid/text/parse-canonical-result';
  const compilerRevision = '205896a413e5dea92366309c35ae85e542d055f6';
  const sha = value => typeof value === 'string' && /^[a-f0-9]{64}$/.test(value);
  const revision = value => typeof value === 'string' && /^[a-f0-9]{40}$/.test(value);
  const own = (value, key) => Object.prototype.hasOwnProperty.call(value, key);
  const units = value => Object.freeze(Array.from({ length: value.length }, (_, index) => value.charCodeAt(index)));
  function object(value, keys, label) {
    if (!value || typeof value !== 'object' || Array.isArray(value) ||
        Object.keys(value).sort().join(',') !== [...keys].sort().join(',')) throw new Error(`Invalid ${label} fields.`);
  }
  function canonical(value) {
    if (value === null || typeof value === 'string' || typeof value === 'boolean' ||
        (typeof value === 'number' && Number.isFinite(value))) return value;
    if (Array.isArray(value)) return Object.freeze(value.map(canonical));
    if (value && typeof value === 'object') {
      const copy = Object.create(null);
      for (const key of Object.keys(value).sort()) copy[key] = canonical(value[key]);
      return Object.freeze(copy);
    }
    throw new Error('Program metadata must contain JSON values.');
  }
  function inputCall(fn, ...args) {
    try { return fn(...args); }
    catch (error) { throw new Error(error.message.replace('Nothing was searched.', 'The parser was not called.')); }
  }
  const read = (source, mode) => inputCall(text.read, source, mode);
  const bindString = value => inputCall(text.bindString, value);
  function validateInputs(inputs, allowBinding = false) {
    if (!Array.isArray(inputs) || inputs.length !== 1) throw new Error('Provide exactly one string input.');
    const bound = bindString(inputs[0]);
    if (!allowBinding && bound.changed) throw new Error('Worker inputs must already be valid UTF-8 string bindings.');
    return Object.freeze([bound.text]);
  }
  const bindInputs = inputs => validateInputs(inputs, true);
  function requestId(id) { if (!Number.isSafeInteger(id) || id < 1) throw new Error('Invalid request ID.'); }
  function validateRequest(message) {
    object(message, ['type', 'id', 'inputs'], 'request');
    if (message.type !== 'parse') throw new Error('Unsupported request type.');
    requestId(message.id);
    return Object.freeze({ id: message.id, inputs: validateInputs(message.inputs) });
  }
  function validateCancel(message) {
    object(message, ['type', 'id'], 'cancellation');
    if (message.type !== 'cancel') throw new Error('Unsupported cancellation type.');
    requestId(message.id); return message.id;
  }
  function validateOutput(output, inputs) {
    validateInputs(inputs);
    object(output, fields, 'compiled output');
    if (typeof output.parsed !== 'boolean' || typeof output.invalid_length !== 'boolean' ||
        typeof output.error !== 'string' || output.error.length > 240 ||
        typeof output.canonical_uuid !== 'string' || !/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(output.canonical_uuid) ||
        (output.parsed ? output.error !== '' || output.invalid_length : output.error === '')) {
      throw new Error('The compiled program returned an invalid four-field record.');
    }
    const boundError = bindString(output.error);
    if (boundError.changed || boundError.bytes > 240) throw new Error('The compiled error text is outside its UTF-8 byte domain.');
    // Canonical shape is a serialization constraint on success AND error.
    // It never establishes parser success; that observation is output.parsed.
    return Object.freeze({ ...output });
  }
  function parseCompiledReply(raw, suppliedInputs) {
    const inputs = validateInputs(suppliedInputs);
    if (typeof raw !== 'string' || raw.length > 32768) throw new Error('The compiled reply is unreadable.');
    let reply;
    try { reply = JSON.parse(raw); } catch { throw new Error('The compiled reply is invalid JSON.'); }
    if (reply && reply.ok === false) {
      object(reply, ['ok', 'error'], 'compiled rejection');
      if (typeof reply.error !== 'string' || !reply.error || reply.error.length > 240) throw new Error('Invalid compiled rejection.');
      throw new Error(`The compiled program rejected the input: ${reply.error}`);
    }
    object(reply, ['ok', 'inputs', 'output'], 'compiled reply');
    if (reply.ok !== true || !Array.isArray(reply.inputs) || reply.inputs.length !== 1 || reply.inputs[0] !== inputs[0]) {
      throw new Error('The compiled reply does not match the requested input.');
    }
    return validateOutput(reply.output, inputs);
  }
  function parseBridgeReply(raw, originals) {
    const inputs = bindInputs(originals);
    return Object.freeze({ ok: true, inputs, output: parseCompiledReply(raw, inputs) });
  }
  function hashMap(value) {
    if (!value || typeof value !== 'object' || Array.isArray(value) || !Object.keys(value).length ||
        Object.keys(value).some(key => !key || key.startsWith('/') || key.split('/').includes('..') || !sha(value[key]))) {
      throw new Error('Invalid build source or file hash bindings.');
    }
  }
  function validateMetadata(metadata) {
    if (!metadata || metadata.schema !== 'gooo/browser-uuid-parse/v1' || metadata.entry_id !== entry || metadata.result_entity_id !== result ||
        JSON.stringify(metadata.input_entity_ids) !== JSON.stringify(['gooo://std/text']) ||
        metadata.compiler_revision !== compilerRevision || metadata.compiler_version !== '0.6.37-dev' || metadata.go_version !== 'go1.27.3' ||
        !revision(metadata.source_revision) || typeof metadata.source_dirty !== 'boolean' ||
        !Number.isSafeInteger(metadata.wasm_bytes) || metadata.wasm_bytes < 1 || metadata.wasm_bytes > 32 * 1024 * 1024 ||
        ['native_cases', 'fixture_cases', 'fixture_rejections'].some(key => !Number.isSafeInteger(metadata[key]) || metadata[key] < 1) ||
        metadata.native_receipt_expectation_cases !== 0 ||
        ['wasm_sha256', 'generated_sha256', 'execution_sha256', 'replay_sha256', 'fixtures_sha256'].some(key => !sha(metadata[key]))) {
      throw new Error('UUID parse build metadata is incomplete or has unsupported pins.');
    }
    hashMap(metadata.files_sha256); hashMap(metadata.source_sha256);
    if (metadata.native_cases !== metadata.fixture_cases ||
        [['program.wasm', 'wasm_sha256'], ['generated.go', 'generated_sha256'], ['execution.json', 'execution_sha256'],
          ['replay.json', 'replay_sha256'], ['fixtures.json', 'fixtures_sha256']].some(([file, key]) => metadata.files_sha256[file] !== metadata[key])) {
      throw new Error('Build counters or primary artifact hash bindings are inconsistent.');
    }
    return canonical(metadata);
  }
  function validateReady(message) {
    object(message, ['type', 'program'], 'ready message');
    if (message.type !== 'ready') throw new Error('Invalid ready message.');
    return validateMetadata(message.program);
  }
  function validateResult(message, expected) {
    object(message, ['type', 'id', 'inputs', 'output', 'program'], 'result');
    requestId(expected.id);
    const inputs = validateInputs(expected.inputs);
    if (message.type !== 'result' || message.id !== expected.id ||
        JSON.stringify(validateMetadata(message.program)) !== JSON.stringify(validateMetadata(expected.program)) ||
        !Array.isArray(message.inputs) || message.inputs.length !== 1 || message.inputs[0] !== inputs[0]) {
      throw new Error('The result does not match the request and verified program.');
    }
    return validateOutput(message.output, inputs);
  }
  function validateError(message, expectedId) {
    object(message, own(message || {}, 'id') ? ['type', 'id', 'message'] : ['type', 'message'], 'error');
    if (message.type !== 'error' || typeof message.message !== 'string' || !message.message || message.message.length > 240) throw new Error('Invalid worker error.');
    if (own(message, 'id')) requestId(message.id);
    if (expectedId !== undefined && message.id !== undefined && message.id !== expectedId) throw new Error('Stale worker error.');
    return message.message;
  }
  function observation(snapshots, output, program, origins) {
    if (!Array.isArray(snapshots) || snapshots.length !== 1 || !Array.isArray(origins) || origins.length !== 1) throw new Error('Invalid observation input.');
    const snapshot = read(snapshots[0].capturedSource, snapshots[0].mode);
    output = validateOutput(output, [snapshot.bound]);
    return Object.freeze({ schema: 'gooo/uuid-parse-observation/v1', kind: 'browser-observation',
      input_binding: 'TextEncoder UTF-8 then TextDecoder with ignoreBOM:true; lone UTF-16 surrogates become U+FFFD; no trimming or Unicode normalization',
      inputs: Object.freeze([Object.freeze({ name: 'input', mode: snapshot.mode, capture_origin: origins[0],
        captured_source: snapshot.capturedSource, captured_source_utf16_units: units(snapshot.capturedSource), original: snapshot.original,
        original_utf16_units: snapshot.originalUtf16Units, original_escaped: snapshot.originalEscaped, bound: snapshot.bound,
        bound_utf16_units: snapshot.boundUtf16Units, bound_escaped: snapshot.boundEscaped, bound_utf8_bytes: snapshot.utf8Bytes,
        replaced_unpaired_surrogates: snapshot.changed })]), output, program: validateMetadata(program),
      scope: 'Observation from this browser and compiled WebAssembly program. It is not a native execution receipt, saved native replay, signed attestation, or full-library compatibility proof.' });
  }
  return Object.freeze({ MAX_BYTES: text.MAX_BYTES, MAX_JSON_SOURCE: text.MAX_JSON_SOURCE, fields, read, bindString, bindInputs, validateInputs,
    validateRequest, validateCancel, validateOutput, parseCompiledReply, parseBridgeReply, validateMetadata, validateReady, validateResult, validateError, observation });
});
