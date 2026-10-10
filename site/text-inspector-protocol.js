// Transport validation only. All Unicode classification and scalar lengths
// come from the four existing generated Gooo operations.
((root, factory) => {
  const api = factory();
  if (typeof module === 'object' && module.exports) module.exports = api;
  else root.GoooTextInspectorProtocol = api;
})(globalThis, () => {
  'use strict';
  const MAX_POINTS = 256;
  const OPERATIONS = Object.freeze(['utf8.ValidRune', 'unicode.IsSpace', 'utf8.RuneLen', 'utf16.RuneLen']);
  const hash = /^[a-f0-9]{64}$/;
  const revision = /^[a-f0-9]{40}$/;

  function object(value, keys, label) {
    if (!value || typeof value !== 'object' || Array.isArray(value) ||
        Object.keys(value).sort().join(',') !== [...keys].sort().join(',')) {
      throw new Error(`Invalid ${label} fields.`);
    }
  }
  function requestId(id) {
    if (!Number.isSafeInteger(id) || id < 1) throw new Error('Invalid request ID.');
  }
  function validatePoints(points) {
    if (!Array.isArray(points) || points.length > MAX_POINTS) throw new Error('Use at most 256 code points.');
    const copy = [];
    for (const point of points) {
      if (typeof point !== 'string' || !/^(?:0|[1-9][0-9]{0,6})$/.test(point) || Number(point) > 0x10FFFF) {
        throw new Error('Each code point must be a canonical decimal value from 0 to 1114111.');
      }
      copy.push(point);
    }
    return Object.freeze(copy);
  }
  function validateRequest(message) {
    object(message, ['type', 'id', 'points'], 'request');
    if (message.type !== 'inspect') throw new Error('Unsupported request type.');
    requestId(message.id);
    return Object.freeze({ id: message.id, points: validatePoints(message.points) });
  }
  function validateCancel(message) {
    object(message, ['type', 'id'], 'cancellation');
    if (message.type !== 'cancel') throw new Error('Unsupported cancellation type.');
    requestId(message.id);
    return message.id;
  }
  function identity(value) {
    if (typeof value.wasmSha256 !== 'string' || !hash.test(value.wasmSha256) ||
        typeof value.sourceRevision !== 'string' || !revision.test(value.sourceRevision)) {
      throw new Error('Invalid program identity.');
    }
    return Object.freeze({ wasmSha256: value.wasmSha256, sourceRevision: value.sourceRevision });
  }
  function validateReady(message) {
    object(message, ['type', 'wasmSha256', 'sourceRevision'], 'ready message');
    if (message.type !== 'ready') throw new Error('Invalid ready message.');
    return identity(message);
  }
  function lengthValue(value, utf8) {
    if (typeof value !== 'string' || !(utf8 ? ['-1', '1', '2', '3', '4'] : ['-1', '1', '2']).includes(value)) {
      throw new Error('Invalid compiled scalar length.');
    }
    return value;
  }
  function parseCompiledReply(raw, operation, input) {
    if (!OPERATIONS.includes(operation)) throw new Error('Unsupported compiled operation.');
    if (typeof raw !== 'string' || raw.length > 512) throw new Error('Invalid compiled reply.');
    let reply;
    try { reply = JSON.parse(raw); } catch { throw new Error('Invalid compiled JSON.'); }
    object(reply, ['ok', 'operation', 'inputs', 'output'], 'compiled reply');
    if (reply.ok !== true || reply.operation !== operation || !Array.isArray(reply.inputs) ||
        reply.inputs.length !== 2 || reply.inputs[0] !== input || reply.inputs[1] !== '0') {
      throw new Error('Compiled reply does not match the requested operation and input.');
    }
    object(reply.output, ['type', 'value'], 'compiled output');
    const boolean = operation === 'utf8.ValidRune' || operation === 'unicode.IsSpace';
    if (reply.output.type !== (boolean ? 'boolean' : 'integer')) throw new Error('Invalid compiled output type.');
    if (boolean) {
      if (typeof reply.output.value !== 'boolean') throw new Error('Invalid compiled Boolean.');
      return reply.output.value;
    }
    return lengthValue(reply.output.value, operation === 'utf8.RuneLen');
  }
  function validateRow(row, point) {
    object(row, ['input', 'validScalar', 'goSpace', 'utf8Bytes', 'utf16Units'], 'row');
    if (row.input !== point || typeof row.validScalar !== 'boolean' || typeof row.goSpace !== 'boolean') {
      throw new Error('Invalid result input or Boolean fields.');
    }
    return Object.freeze({ input: point, validScalar: row.validScalar, goSpace: row.goSpace,
      utf8Bytes: lengthValue(row.utf8Bytes, true), utf16Units: lengthValue(row.utf16Units, false) });
  }
  function validateResult(message, expected) {
    object(message, ['type', 'id', 'rows', 'wasmSha256', 'sourceRevision'], 'result');
    requestId(expected.id);
    const points = validatePoints(expected.points);
    const actualIdentity = identity(message);
    identity(expected);
    if (message.type !== 'result' || message.id !== expected.id ||
        actualIdentity.wasmSha256 !== expected.wasmSha256 || actualIdentity.sourceRevision !== expected.sourceRevision ||
        !Array.isArray(message.rows) || message.rows.length !== points.length) {
      throw new Error('Result does not match the request and program.');
    }
    return Object.freeze(points.map((point, index) => validateRow(message.rows[index], point)));
  }
  function validateProgress(message, expected) {
    object(message, ['type', 'id', 'completed', 'total'], 'progress');
    requestId(expected.id);
    if (message.type !== 'progress' || message.id !== expected.id || message.total !== expected.total ||
        !Number.isInteger(message.total) || message.total < 0 || message.total > MAX_POINTS ||
        !Number.isInteger(message.completed) || message.completed < 0 || message.completed > message.total) {
      throw new Error('Invalid progress message.');
    }
    return message.completed;
  }
  function validateError(message, expectedId) {
    object(message, Object.hasOwn(message || {}, 'id') ? ['type', 'id', 'message'] : ['type', 'message'], 'error');
    if (message.type !== 'error' || typeof message.message !== 'string' || message.message.length < 1 || message.message.length > 240) {
      throw new Error('Invalid error message.');
    }
    if (Object.hasOwn(message, 'id')) requestId(message.id);
    if (expectedId !== undefined && message.id !== undefined && message.id !== expectedId) throw new Error('Stale error message.');
    return message.message;
  }
  function validateMetadata(metadata) {
    const expectedInputs = ['gooo://std/text', ...Array(4).fill('gooo://std/integer')];
    if (!metadata || metadata.schema !== 'gooo/browser-demo/v2' ||
        metadata.entry_id !== 'gooo://go-ports/examples/browser-demo/dispatch/v2' ||
        metadata.reply_entity_id !== 'gooo://go-ports/examples/browser-demo/reply/v2' ||
        JSON.stringify(metadata.input_entity_ids) !== JSON.stringify(expectedInputs) ||
        !Number.isSafeInteger(metadata.wasm_bytes) || metadata.wasm_bytes < 1 || metadata.wasm_bytes > 32 * 1024 * 1024) {
      throw new Error('Invalid browser program metadata.');
    }
    return identity({ wasmSha256: metadata.wasm_sha256, sourceRevision: metadata.source_revision });
  }
  return Object.freeze({ MAX_POINTS, OPERATIONS, validatePoints, validateRequest, validateCancel, validateReady,
    validateResult, validateProgress, validateError, validateMetadata, parseCompiledReply });
});
