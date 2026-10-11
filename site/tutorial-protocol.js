// Transport and shape checks only. The tutorial's calculation stays in Gooo.
((root, factory) => {
  const api = factory();
  if (typeof module === 'object' && module.exports) module.exports = api;
  else root.GoooTutorialProtocol = api;
})(globalThis, () => {
  'use strict';
  const files = Object.freeze(['limits.gooo', 'main.gooo', 'inputs.json', 'gooo.workspace.json', 'NOTICE', 'README.md']);
  const fields = Object.freeze(['score', 'remaining', 'in_range', 'label']);
  const integer = value => typeof value === 'string' && value.length <= 20 && /^(?:0|[1-9][0-9]*|-[1-9][0-9]*)$/.test(value) &&
    BigInt(value) >= -9223372036854775808n && BigInt(value) <= 9223372036854775807n;
  const sha = value => typeof value === 'string' && /^[a-f0-9]{64}$/.test(value);
  const revision = value => typeof value === 'string' && /^[a-f0-9]{40}$/.test(value);
  const exact = (value, keys) => value !== null && typeof value === 'object' && !Array.isArray(value) &&
    Object.keys(value).sort().join(',') === [...keys].sort().join(',');

  function boundText(text) {
    // Match Go's JSON string decoder / JS UTF-8 transport for unpaired surrogates.
    return new TextDecoder('utf-8', { ignoreBOM: true }).decode(new TextEncoder().encode(text));
  }
  function labelFromEditor(value, mode) {
    if (typeof value !== 'string') throw new Error('Enter a text label.');
    if (mode === 'text') return value;
    if (mode !== 'json') throw new Error('Choose a supported label input mode.');
    let label;
    try { label = JSON.parse(value); } catch { throw new Error('Enter one JSON string, including its double quotes.'); }
    if (typeof label !== 'string') throw new Error('The label must be a JSON string, not a number, object or null.');
    return label;
  }
  function validateInputs(inputs) {
    if (!Array.isArray(inputs) || inputs.length !== 2 || !integer(inputs[0])) {
      const error = new Error('Score must be a decimal integer from -9223372036854775808 to 9223372036854775807. Use no spaces, plus sign, leading zeros or -0.');
      error.inputIndex = 0; throw error;
    }
    if (typeof inputs[1] !== 'string' || new TextEncoder().encode(inputs[1]).length > 1024) {
      const error = new Error('The label must fit in 1,024 UTF-8 bytes. Nothing was shortened or run.');
      error.inputIndex = 1; throw error;
    }
    return Object.freeze([...inputs]);
  }
  function parseReply(raw, original) {
    const inputs = validateInputs(original);
    if (typeof raw !== 'string' || raw.length > 20000) throw new Error('The program returned an unreadable result.');
    let reply;
    try { reply = JSON.parse(raw); } catch { throw new Error('The program returned invalid JSON.'); }
    if (exact(reply, ['ok', 'error']) && reply.ok === false && typeof reply.error === 'string' && reply.error) {
      throw new Error(`The program rejected this input: ${reply.error}`);
    }
    if (!exact(reply, ['ok', 'inputs', 'output']) || reply.ok !== true || !Array.isArray(reply.inputs) ||
        reply.inputs.length !== 2 || reply.inputs[0] !== inputs[0] || reply.inputs[1] !== boundText(inputs[1])) {
      throw new Error('The program result did not match the submitted score and bound label.');
    }
    const output = reply.output;
    if (!exact(output, fields) || !integer(output.score) || !integer(output.remaining) ||
        BigInt(output.score) < 0n || BigInt(output.score) > 100n || BigInt(output.remaining) < 0n || BigInt(output.remaining) > 100n ||
        typeof output.in_range !== 'boolean' || typeof output.label !== 'string' ||
        output.label !== boundText(output.label) || new TextEncoder().encode(output.label).length > 1024) {
      throw new Error('The program returned an invalid Summary record.');
    }
    return Object.freeze({ ok: true, inputs: Object.freeze([...reply.inputs]), output: Object.freeze({ ...output }) });
  }
  function validateMetadata(value) {
    if (!value || value.schema !== 'gooo/browser-tutorial/v1' ||
        !['wasm_sha256', 'generated_sha256', 'execution_sha256', 'replay_sha256'].every(key => sha(value[key])) ||
        !revision(value.source_revision) || value.compiler_revision !== '366ee6ef8f9d2b9b57331dfd26b6f908c0289851' ||
        value.go_version !== 'go1.27.2' || value.compiler_version !== '0.6.24-dev' || typeof value.source_dirty !== 'boolean' ||
        value.entry_id !== 'urn:learn-gooo:describe' || value.result_entity_id !== 'urn:learn-gooo:summary' ||
        !Array.isArray(value.input_entity_ids) || value.input_entity_ids.length !== 2 ||
        value.input_entity_ids[0] !== 'urn:gooo:type:integer' || value.input_entity_ids[1] !== 'urn:gooo:type:string' ||
        !Number.isSafeInteger(value.wasm_bytes) ||
        value.wasm_bytes < 1 || value.wasm_bytes > 32 * 1024 * 1024 ||
        !Number.isSafeInteger(value.fixture_cases) || value.fixture_cases < 1 ||
        !Number.isSafeInteger(value.native_cases) || value.native_cases < 4 ||
        !sha(value.source_zip_sha256) || !Number.isSafeInteger(value.source_zip_bytes) ||
        value.source_zip_bytes < 1 || value.source_zip_bytes > 1024 * 1024 ||
        !exact(value.sample_sha256, files) || !files.every(name => sha(value.sample_sha256[name]))) {
      throw new Error('Tutorial build metadata is incomplete or has an unsupported format.');
    }
    return Object.freeze({ ...value, sample_sha256: Object.freeze({ ...value.sample_sha256 }) });
  }
  function validateRequest(value) {
    if (!exact(value, ['type', 'id', 'inputs']) || value.type !== 'run' || !Number.isSafeInteger(value.id) || value.id < 1) {
      throw new Error('The tutorial request is malformed.');
    }
    return { type: 'run', id: value.id, inputs: validateInputs(value.inputs) };
  }
  function validateResult(value, request, metadata) {
    if (!exact(value, ['type', 'id', 'wasmSha256', 'result']) || value.type !== 'result' ||
        value.id !== request.id || value.wasmSha256 !== metadata.wasm_sha256) throw new Error('The result belongs to a different request or program.');
    return parseReply(JSON.stringify(value.result), request.inputs);
  }
  function inputDocument(inputs) {
    inputs = validateInputs(inputs);
    // Interpolate the validated integer token, never a JavaScript Number.
    return '{\n  "schema": "gooo/body-composition-inputs/v1",\n  "inputs": [\n    {"app/report:Describe.input0": ' + inputs[0] +
      ',\n     "app/report:Describe.input1": ' + JSON.stringify(inputs[1]) + '}\n  ]\n}\n';
  }
  return Object.freeze({ files, fields, integer, exact, boundText, labelFromEditor, validateInputs, parseReply,
    validateMetadata, validateRequest, validateResult, inputDocument });
});
