// Exact decimal transport for two compiled consumers. No crop arithmetic.
((root, factory) => {
  const api = factory();
  if (typeof module === 'object' && module.exports) module.exports = api;
  else root.GoooCropPlannerProtocol = api;
})(globalThis, () => {
  'use strict';
  const fields = Object.freeze(['min_x', 'min_y', 'max_x', 'max_y']);
  const inputIds = Object.freeze(['bounds-min-x', 'bounds-min-y', 'bounds-max-x', 'bounds-max-y',
    'crop-min-x', 'crop-min-y', 'crop-max-x', 'crop-max-y']);
  const labels = Object.freeze(['Bounds min_x', 'Bounds min_y', 'Bounds max_x', 'Bounds max_y',
    'Requested crop min_x', 'Requested crop min_y', 'Requested crop max_x', 'Requested crop max_y']);
  const cornerLabels = Object.freeze([...labels.slice(0, 4),
    'First corner x', 'First corner y', 'Second corner x', 'Second corner y']);
  const modes = Object.freeze({
    ordered: Object.freeze({ label: 'Keep rectangle order', entryId: 'gooo://example/crop-planner/crop-plan',
      assetBase: './crop/', runtimeName: 'goooCrop', readyName: 'goooCropReady', activity: 'CropPlan',
      source: 'examples/crop-planner/crop_planner.gooo', portSource: 'ports/image/geometry.gooo' }),
    corners: Object.freeze({ label: 'Two-corner selection', entryId: 'gooo://example/crop-planner/drag-crop-plan',
      assetBase: './drag-crop/', runtimeName: 'goooDragCrop', readyName: 'goooDragCropReady', activity: 'DragCropPlan',
      source: 'examples/crop-planner/drag_crop.gooo', portSource: 'ports/image/canon.gooo' })
  });
  function modeFor(mode) {
    if (!Object.hasOwn(modes, mode)) throw new Error('Choose a supported crop selection mode.');
    return modes[mode];
  }
  const decimal = /^(?:0|[1-9][0-9]*|-[1-9][0-9]*)$/;
  const minimum = -9223372036854775808n;
  const maximum = 9223372036854775807n;
  const presets = Object.freeze({
    partial: Object.freeze(['0', '0', '640', '480', '-20', '40', '200', '550']),
    edge: Object.freeze(['0', '0', '640', '480', '640', '0', '800', '100']),
    reversedCrop: Object.freeze(['0', '0', '640', '480', '90', '80', '10', '20']),
    reversedBounds: Object.freeze(['640', '0', '0', '480', '10', '10', '30', '30']),
    empty: Object.freeze(['7', '9', '7', '20', '7', '9', '8', '21']),
    negative: Object.freeze(['-20', '-30', '5', '10', '-8', '-40', '12', '4']),
    precise: Object.freeze(['9007199254740992', '-9007199254740993', '9007199254740994', '-9007199254740991',
      '9007199254740993', '-9007199254740992', '9007199254740995', '-9007199254740991']),
    maximum: Object.freeze(['-9223372036854775808', '-9223372036854775808', '9223372036854775807', '9223372036854775807',
      '9223372036854775806', '9223372036854775806', '9223372036854775807', '9223372036854775807']),
    minimum: Object.freeze(['-9223372036854775808', '-9223372036854775808', '9223372036854775807', '9223372036854775807',
      '-9223372036854775808', '-9223372036854775808', '-9223372036854775807', '-9223372036854775807'])
  });

  function integer(value) {
    if (typeof value !== 'string' || value.length > 20 || !decimal.test(value)) return false;
    const parsed = BigInt(value);
    return parsed >= minimum && parsed <= maximum;
  }

  function validateInputs(inputs, mode = 'ordered') {
    modeFor(mode);
    const inputLabels = mode === 'corners' ? cornerLabels : labels;
    if (!Array.isArray(inputs) || inputs.length !== 8) throw new Error('Enter all eight coordinates.');
    for (let index = 0; index < inputs.length; index++) {
      const value = inputs[index];
      if (!integer(value)) {
        const error = new Error(`${inputLabels[index]} must be a decimal integer from -9223372036854775808 to 9223372036854775807. Use no spaces, plus sign, leading zeros or -0.`);
        error.inputIndex = index;
        throw error;
      }
    }
    return Object.freeze([...inputs]);
  }

  function exactKeys(value, keys) {
    return value !== null && typeof value === 'object' && !Array.isArray(value) &&
      Object.keys(value).sort().join(',') === [...keys].sort().join(',');
  }

  function parseReply(raw, originalInputs, mode = 'ordered') {
    const inputs = validateInputs(originalInputs, mode);
    if (typeof raw !== 'string' || raw.length > 4096) throw new Error('The program returned an unreadable result.');
    let reply;
    try { reply = JSON.parse(raw); } catch { throw new Error('The program returned invalid JSON.'); }
    if (exactKeys(reply, ['ok', 'error']) && reply.ok === false && typeof reply.error === 'string' && reply.error.length > 0) {
      throw new Error(`The program rejected this input: ${reply.error}`);
    }
    if (!exactKeys(reply, ['ok', 'inputs', 'output']) || reply.ok !== true ||
        !Array.isArray(reply.inputs) || reply.inputs.length !== inputs.length ||
        !reply.inputs.every((value, index) => value === inputs[index])) {
      throw new Error('The program result did not match the original eight input coordinates.');
    }
    const output = reply.output;
    if (!exactKeys(output, [...fields, 'has_pixels']) || !fields.every(field => integer(output[field])) ||
        typeof output.has_pixels !== 'boolean') throw new Error('The program returned an invalid crop record.');
    // Validate the result contract without reimplementing intersection in JavaScript.
    if (output.has_pixels
      ? BigInt(output.min_x) >= BigInt(output.max_x) || BigInt(output.min_y) >= BigInt(output.max_y)
      : fields.some(field => output[field] !== '0')) throw new Error('The program returned an inconsistent crop record.');
    return Object.freeze({ ok: true, inputs, output: Object.freeze({ ...output }) });
  }

  function validateMetadata(metadata, mode = 'ordered') {
    const expected = modeFor(mode);
    const sha = value => typeof value === 'string' && /^[a-f0-9]{64}$/.test(value);
    const revision = value => typeof value === 'string' && /^[a-f0-9]{40}$/.test(value);
    if (!metadata || metadata.schema !== 'gooo/browser-crop-planner/v1' || metadata.entry_id !== expected.entryId ||
        !sha(metadata.wasm_sha256) || !sha(metadata.generated_sha256) ||
        !sha(metadata.execution_sha256) || !sha(metadata.replay_sha256) ||
        !revision(metadata.source_revision) || !revision(metadata.compiler_revision) ||
        typeof metadata.source_dirty !== 'boolean' || typeof metadata.go_version !== 'string' || !metadata.go_version ||
        !Number.isSafeInteger(metadata.wasm_bytes) || metadata.wasm_bytes < 1 || metadata.wasm_bytes > 32 * 1024 * 1024 ||
        !Number.isSafeInteger(metadata.fixture_cases) || metadata.fixture_cases < 1) {
      throw new Error('Crop planner build metadata is incomplete or has an unsupported format.');
    }
    return Object.freeze({ ...metadata });
  }

  return Object.freeze({ fields, inputIds, presets, modes, modeFor, validateInputs, parseReply, validateMetadata });
});
