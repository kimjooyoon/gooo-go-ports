const test = require('node:test');
const assert = require('node:assert/strict');
const protocol = require('../site/crop-planner-protocol.js');
const normal = [...protocol.presets.partial];
const output = { min_x: '0', min_y: '40', max_x: '200', max_y: '480', has_pixels: true };
const reply = (inputs = normal, value = output) => JSON.stringify({ ok: true, inputs, output: value });
const metadata = { schema: 'gooo/browser-crop-planner/v1', wasm_sha256: 'a'.repeat(64), wasm_bytes: 100,
  source_revision: 'b'.repeat(40), source_dirty: false, compiler_revision: 'c'.repeat(40), go_version: 'go1.27.2',
  generated_sha256: 'd'.repeat(64), execution_sha256: 'e'.repeat(64), replay_sha256: 'f'.repeat(64), fixture_cases: 486 };

test('all presets preserve eight original int64 strings, including reversed and extreme rectangles', () => {
  assert.equal(Object.keys(protocol.presets).length, 9);
  for (const inputs of Object.values(protocol.presets)) {
    const accepted = protocol.validateInputs(inputs);
    assert.deepEqual(accepted, inputs);
    assert.ok(Object.isFrozen(accepted));
  }
  assert.deepEqual(protocol.validateInputs(protocol.presets.reversedCrop).slice(4), ['90', '80', '10', '20']);
  assert.equal(protocol.validateInputs(protocol.presets.precise)[4], '9007199254740993');
  assert.equal(protocol.validateInputs(protocol.presets.minimum)[4], '-9223372036854775808');
  assert.equal(protocol.validateInputs(protocol.presets.maximum)[6], '9223372036854775807');
});

test('every coordinate rejects wrong types, noncanonical notation and int64 overflow without coercion', () => {
  const malformed = [null, true, 0, {}, [], '', ' ', '+1', '-0', '01', '-01', '1.5', '1e2', ' 1', '1 ',
    '١', '１２', '9223372036854775808', '-9223372036854775809', '0'.repeat(21)];
  for (let index = 0; index < 8; index++) {
    for (const bad of malformed) {
      const inputs = [...normal]; inputs[index] = bad;
      assert.throws(() => protocol.validateInputs(inputs), error => error.inputIndex === index);
    }
    const sparse = [...normal]; delete sparse[index];
    assert.throws(() => protocol.validateInputs(sparse), error => error.inputIndex === index);
  }
  for (const inputs of [null, {}, [], normal.slice(1), [...normal, '0']]) assert.throws(() => protocol.validateInputs(inputs));
});

test('output coordinates retain exact integers beyond 2^53 and at both int64 endpoints', () => {
  const cases = [
    ['precise', { min_x: '9007199254740993', min_y: '-9007199254740992', max_x: '9007199254740994', max_y: '-9007199254740991', has_pixels: true }],
    ['maximum', { min_x: '9223372036854775806', min_y: '9223372036854775806', max_x: '9223372036854775807', max_y: '9223372036854775807', has_pixels: true }],
    ['minimum', { min_x: '-9223372036854775808', min_y: '-9223372036854775808', max_x: '-9223372036854775807', max_y: '-9223372036854775807', has_pixels: true }]
  ];
  for (const [preset, expected] of cases) {
    const inputs = protocol.presets[preset];
    assert.deepEqual(protocol.parseReply(reply(inputs, expected), inputs).output, expected);
  }
  const zero = { min_x: '0', min_y: '0', max_x: '0', max_y: '0', has_pixels: false };
  for (const preset of ['edge', 'empty', 'reversedCrop', 'reversedBounds']) {
    assert.deepEqual(protocol.parseReply(reply(protocol.presets[preset], zero), protocol.presets[preset]).output, zero);
  }
});

test('every returned input is bound to the exact original value and order', () => {
  for (let index = 0; index < 8; index++) {
    const changed = [...normal]; changed[index] = changed[index] === '0' ? '1' : '0';
    assert.throws(() => protocol.parseReply(reply(changed), normal), /original eight/);
  }
  assert.throws(() => protocol.parseReply(reply(normal.slice(1)), normal), /original eight/);
  assert.throws(() => protocol.parseReply(reply([...normal, '0']), normal), /original eight/);
  assert.throws(() => protocol.parseReply(reply([...normal.slice(4), ...normal.slice(0, 4)]), normal), /original eight/);
});

test('malformed, incomplete or internally inconsistent results never become a crop result', () => {
  for (const field of protocol.fields) {
    const missing = { ...output }; delete missing[field];
    assert.throws(() => protocol.parseReply(reply(normal, missing), normal), /invalid crop record/);
    for (const bad of [null, 0, true, '-0', '01', '9223372036854775808']) {
      assert.throws(() => protocol.parseReply(reply(normal, { ...output, [field]: bad }), normal), /invalid crop record/);
    }
  }
  for (const value of [null, [], { ...output, other: '0' }, { ...output, has_pixels: 'true' }]) {
    assert.throws(() => protocol.parseReply(reply(normal, value), normal), /invalid crop record/);
  }
  for (const value of [{ ...output, has_pixels: false }, { ...output, max_x: '0' }, { ...output, min_y: '500' }]) {
    assert.throws(() => protocol.parseReply(reply(normal, value), normal), /inconsistent crop record/);
  }
  for (const raw of [null, '{}', 'null', '[1]', 'oops', JSON.stringify({ ok: true, inputs: normal, output, extra: true })]) {
    assert.throws(() => protocol.parseReply(raw, normal));
  }
  assert.throws(() => protocol.parseReply(JSON.stringify({ ok: false, error: 'bad input' }), normal), /rejected this input: bad input/);
});

test('metadata requires the crop schema, complete immutable identities and finite build sizes', () => {
  assert.deepEqual(protocol.validateMetadata(metadata), metadata);
  for (const key of Object.keys(metadata)) {
    const missing = { ...metadata }; delete missing[key];
    assert.throws(() => protocol.validateMetadata(missing), /metadata/);
  }
  for (const patch of [{ schema: 'gooo/browser-demo/v2' }, { wasm_sha256: 'a'.repeat(63) },
    { source_revision: 'main' }, { source_dirty: 'false' }, { wasm_bytes: 0 }, { wasm_bytes: 33554433 },
    { wasm_bytes: 1.5 }, { fixture_cases: 0 }, { fixture_cases: 1.5 }]) {
    assert.throws(() => protocol.validateMetadata({ ...metadata, ...patch }), /metadata/);
  }
});
