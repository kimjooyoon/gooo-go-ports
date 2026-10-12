// Validate transport and UTF-16-to-UTF-8 input binding. Index and Count are
// implemented exclusively by the compiled Gooo Report activity.
((root, factory) => {
  const api = factory();
  if (typeof module === 'object' && module.exports) module.exports = api;
  else root.GoooLiteralSearchProtocol = api;
})(globalThis, () => {
  'use strict';
  const MAX_BYTES = 1024;
  const MAX_JSON_SOURCE = 8192;
  const compilerRevision = '83928fa4538245abe33b08549da47f8cbdc50ed2';
  const fields = Object.freeze(['text', 'needle', 'first_byte_offset', 'found', 'non_overlapping_count', 'text_byte_length', 'needle_byte_length']);
  const sha = value => typeof value === 'string' && /^[a-f0-9]{64}$/.test(value);
  const revision = value => typeof value === 'string' && /^[a-f0-9]{40}$/.test(value);
  const own = (value, key) => Object.prototype.hasOwnProperty.call(value, key);

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
  function units(text) { return Object.freeze(Array.from({ length: text.length }, (_, index) => text.charCodeAt(index))); }
  function escaped(text) { return '"' + units(text).map(unit => '\\u' + unit.toString(16).toUpperCase().padStart(4, '0')).join('') + '"'; }
  function bindString(text) {
    if (typeof text !== 'string') throw new Error('Each input must be a string.');
    if (text.length > MAX_BYTES) throw new Error('Each decoded input must fit in 1,024 UTF-8 bytes. Nothing was searched.');
    const bytes = new TextEncoder().encode(text);
    if (bytes.length > MAX_BYTES) throw new Error('Each decoded input must fit in 1,024 UTF-8 bytes. Nothing was searched.');
    // ignoreBOM:true preserves a leading U+FEFF. TextEncoder replaces only
    // unpaired UTF-16 surrogates; no trimming or normalization is performed.
    const bound = new TextDecoder('utf-8', { fatal: true, ignoreBOM: true }).decode(bytes);
    return Object.freeze({ text: bound, bytes: bytes.length, changed: bound !== text });
  }
  function read(source, mode) {
    if (typeof source !== 'string') throw new Error('The input must be text.');
    let original = source;
    if (mode === 'json') {
      if (source.length > MAX_JSON_SOURCE) throw new Error('JSON notation exceeds 8,192 UTF-16 code units. Nothing was searched.');
      try { original = JSON.parse(source); } catch { throw new Error('Enter one valid JSON string, including its double quotes.'); }
      if (typeof original !== 'string') throw new Error('JSON input must be a string, not an object, array, number or other value.');
    } else if (mode !== 'text') throw new Error('Choose Text or JSON string input.');
    const bound = bindString(original);
    return Object.freeze({ mode, capturedSource: source, original, originalUtf16Units: units(original), originalEscaped: escaped(original),
      bound: bound.text, boundUtf16Units: units(bound.text), boundEscaped: escaped(bound.text), utf8Bytes: bound.bytes, changed: bound.changed });
  }
  function validateInputs(inputs) {
    if (!Array.isArray(inputs) || inputs.length !== 2) throw new Error('Provide exactly two bound string inputs.');
    const copy = Array.from(inputs, value => {
      const bound = bindString(value);
      if (bound.changed) throw new Error('Worker inputs must already be valid UTF-8 string bindings.');
      return value;
    });
    return Object.freeze(copy);
  }
  function bindInputs(inputs) {
    if (!Array.isArray(inputs) || inputs.length !== 2) throw new Error('Provide exactly two string inputs.');
    return Object.freeze(Array.from(inputs, value => bindString(value).text));
  }
  function requestId(id) { if (!Number.isSafeInteger(id) || id < 1) throw new Error('Invalid request ID.'); }
  function validateRequest(message) {
    object(message, ['type', 'id', 'inputs'], 'request');
    if (message.type !== 'search') throw new Error('Unsupported request type.');
    requestId(message.id);
    return Object.freeze({ id: message.id, inputs: validateInputs(message.inputs) });
  }
  function validateCancel(message) {
    object(message, ['type', 'id'], 'cancellation');
    if (message.type !== 'cancel') throw new Error('Unsupported cancellation type.');
    requestId(message.id); return message.id;
  }
  function decimal(value, min, max) {
    if (typeof value !== 'string' || !/^(?:0|[1-9][0-9]*|-[1-9][0-9]*)$/.test(value) || value.length > 5 ||
        Number(value) < min || Number(value) > max) throw new Error('Invalid compiled decimal field.');
    return value;
  }
  function validateOutput(output, suppliedInputs) {
    const inputs = validateInputs(suppliedInputs);
    object(output, fields, 'compiled output');
    if (output.text !== inputs[0] || output.needle !== inputs[1] || typeof output.found !== 'boolean') {
      throw new Error('The output does not match the two bound inputs or its Boolean type.');
    }
    const textBytes = bindString(inputs[0]).bytes, needleBytes = bindString(inputs[1]).bytes;
    decimal(output.first_byte_offset, -1, textBytes);
    decimal(output.non_overlapping_count, 0, MAX_BYTES + 1);
    decimal(output.text_byte_length, 0, MAX_BYTES);
    decimal(output.needle_byte_length, 0, MAX_BYTES);
    if (output.text_byte_length !== String(textBytes) || output.needle_byte_length !== String(needleBytes) ||
        output.found !== (output.first_byte_offset !== '-1')) throw new Error('The output contains inconsistent byte lengths or found status.');
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
    if (reply.ok !== true || !Array.isArray(reply.inputs) || reply.inputs.length !== 2 ||
        !reply.inputs.every((value, index) => value === inputs[index])) throw new Error('The compiled reply does not match the requested inputs.');
    return validateOutput(reply.output, inputs);
  }
  function parseBridgeReply(raw, originalInputs) {
    const inputs = bindInputs(originalInputs);
    return Object.freeze({ ok: true, inputs, output: parseCompiledReply(raw, inputs) });
  }
  function hashMap(value) {
    if (!value || typeof value !== 'object' || Array.isArray(value) || !Object.keys(value).length ||
        Object.keys(value).some(key => !key || key.startsWith('/') || key.split('/').includes('..') || !sha(value[key]))) {
      throw new Error('Invalid build source or file hash bindings.');
    }
  }
  function validateMetadata(metadata) {
    if (!metadata || metadata.schema !== 'gooo/browser-literal-search/v1' ||
        metadata.entry_id !== 'gooo://go-ports/examples/literal-text-report/report' ||
        metadata.result_entity_id !== 'gooo://go-ports/examples/literal-text-report/result' ||
        JSON.stringify(metadata.input_entity_ids) !== JSON.stringify(['gooo://std/text', 'gooo://std/text']) ||
        metadata.compiler_revision !== compilerRevision || metadata.compiler_version !== '0.6.32-dev' || metadata.go_version !== 'go1.27.2' ||
        !revision(metadata.source_revision) || typeof metadata.source_dirty !== 'boolean' ||
        !Number.isSafeInteger(metadata.wasm_bytes) || metadata.wasm_bytes < 1 || metadata.wasm_bytes > 32 * 1024 * 1024 ||
        ['native_cases', 'fixture_cases', 'fixture_rejections'].some(key => !Number.isSafeInteger(metadata[key]) || metadata[key] < 1) ||
        metadata.native_receipt_expectation_cases !== 0 ||
        ['wasm_sha256', 'generated_sha256', 'execution_sha256', 'replay_sha256', 'fixtures_sha256'].some(key => !sha(metadata[key]))) {
      throw new Error('Literal search build metadata is incomplete or has unsupported pins.');
    }
    hashMap(metadata.files_sha256); hashMap(metadata.source_sha256);
    if (metadata.native_cases !== metadata.fixture_cases ||
        [['program.wasm', 'wasm_sha256'], ['generated.go', 'generated_sha256'], ['execution.json', 'execution_sha256'],
          ['replay.json', 'replay_sha256'], ['fixtures.json', 'fixtures_sha256']].some(([file, field]) => metadata.files_sha256[file] !== metadata[field])) {
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
        !Array.isArray(message.inputs) || message.inputs.length !== 2 || !message.inputs.every((value, index) => value === inputs[index])) {
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
    if (!Array.isArray(snapshots) || snapshots.length !== 2 || !Array.isArray(origins) || origins.length !== 2) throw new Error('Invalid observation inputs.');
    const accepted = snapshots.map(snapshot => read(snapshot.capturedSource, snapshot.mode));
    const inputs = accepted.map(snapshot => snapshot.bound);
    output = validateOutput(output, inputs);
    return Object.freeze({ schema: 'gooo/literal-search-observation/v1', kind: 'browser-observation',
      input_binding: 'TextEncoder UTF-8 then TextDecoder with ignoreBOM:true; lone UTF-16 surrogates become U+FFFD; no trimming or Unicode normalization',
      inputs: Object.freeze(accepted.map((snapshot, index) => Object.freeze({ name: index ? 'needle' : 'text', mode: snapshot.mode,
        capture_origin: origins[index], captured_source: snapshot.capturedSource, captured_source_utf16_units: units(snapshot.capturedSource),
        original: snapshot.original, original_utf16_units: snapshot.originalUtf16Units, original_escaped: snapshot.originalEscaped,
        bound: snapshot.bound, bound_utf16_units: snapshot.boundUtf16Units, bound_escaped: snapshot.boundEscaped,
        bound_utf8_bytes: snapshot.utf8Bytes, replaced_unpaired_surrogates: snapshot.changed }))),
      output, program: validateMetadata(program),
      scope: 'Observation from this browser and compiled WebAssembly program. It is not a native execution receipt, saved native replay, signed attestation, or full-library compatibility proof.' });
  }
  return Object.freeze({ MAX_BYTES, MAX_JSON_SOURCE, fields, read, bindString, bindInputs, validateInputs, validateRequest, validateCancel,
    validateOutput, parseCompiledReply, parseBridgeReply, validateMetadata, validateReady, validateResult, validateError, observation });
});
