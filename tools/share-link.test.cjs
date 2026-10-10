'use strict';

const assert = require('node:assert/strict');
const test = require('node:test');
const share = require('../site/share.js');

// Keep these expectations independent of the implementation's operation table.
const runeOperations = [
  'utf8.ValidRune', 'utf8.RuneLen', 'utf16.IsSurrogate',
  'utf16.RuneLen', 'utf16.EncodeRune'
];
const byteOperations = [
  'utf8.RuneStart', 'bits.Len8', 'bits.LeadingZeros8',
  'bits.TrailingZeros8', 'bits.OnesCount8', 'bits.Reverse8'
];
const operations = ['http.StatusText', ...runeOperations, 'utf16.DecodeRune', ...byteOperations];
const wasm = 'ab'.repeat(32);
const revision = 'cd'.repeat(20);
const fields = { v: '1', op: 'http.StatusText', a: '404', b: '0', wasm, rev: revision };
const state = (overrides = {}) => ({ operation: 'http.StatusText', inputs: ['404', '0'], wasm, revision, ...overrides });
const fragment = (overrides = {}) => '#gooo-share?' + new URLSearchParams({ ...fields, ...overrides }).toString();

function accepts(operation, inputs) {
  const original = [...inputs];
  const expected = state({ operation, inputs: [...inputs] });
  assert.doesNotThrow(() => share.validateInputs(operation, inputs));
  assert.deepEqual(inputs, original, 'validation must not rewrite caller inputs');
  const encoded = share.format(expected);
  const decoded = share.parse(encoded);
  assert.deepEqual(decoded, expected);
  assert.equal(share.format(decoded), encoded, 'format must have a stable canonical form');
  assert.ok(decoded.inputs.every(value => typeof value === 'string'));
}

function rejects(operation, inputs) {
  assert.throws(() => share.validateInputs(operation, inputs));
  assert.throws(() => share.format(state({ operation, inputs })));
  if (Array.isArray(inputs) && inputs.length === 2 && inputs.every(value => typeof value === 'string')) {
    assert.throws(() => share.parse(fragment({ op: operation, a: inputs[0], b: inputs[1] })));
  }
}

test('ordinary page anchors are not shared function inputs', () => {
  for (const anchor of ['', '#', '#workbench', '#evidence', '#source', '#another-section?x=1']) {
    assert.equal(share.parse(anchor), null, anchor);
  }
});

test('format uses the complete canonical versioned fragment', () => {
  assert.equal(share.format(state()), `#gooo-share?v=1&op=http.StatusText&a=404&b=0&wasm=${wasm}&rev=${revision}`);
  assert.deepEqual(share.parse(fragment()), state());
  const reordered = `#gooo-share?rev=${revision}&b=0&wasm=${wasm}&a=404&op=http.StatusText&v=1`;
  assert.deepEqual(share.parse(reordered), state());
  assert.equal(share.format(share.parse(reordered)), fragment());
});

test('all 13 allowed operations round trip without computing a result', () => {
  assert.equal(operations.length, 13);
  for (const operation of operations) {
    accepts(operation, operation === 'utf16.DecodeRune' ? ['55357', '56832'] : ['0', '0']);
  }
});

test('HTTP inputs preserve int64 endpoints and adjacent values beyond Number precision', () => {
  for (const value of [
    '-9223372036854775808', '-9223372036854775807',
    '-9007199254740993', '-9007199254740992', '-9007199254740991',
    '-1', '0', '1', '9007199254740991', '9007199254740992',
    '9007199254740993', '9223372036854775806', '9223372036854775807'
  ]) {
    accepts('http.StatusText', [value, '0']);
  }
  for (const value of ['-9223372036854775809', '9223372036854775808', '999999999999999999999999999999999999']) {
    rejects('http.StatusText', [value, '0']);
  }
  assert.notEqual(
    share.format(state({ inputs: ['9007199254740992', '0'] })),
    share.format(state({ inputs: ['9007199254740993', '0'] }))
  );
});

test('rune operations accept the int32 input domain, including invalid Unicode scalars', () => {
  for (const operation of runeOperations) {
    for (const value of ['-2147483648', '-1', '0', '55296', '57343', '65533', '128512', '1114112', '2147483647']) {
      accepts(operation, [value, '0']);
    }
    for (const value of ['-2147483649', '2147483648', '-9223372036854775808', '9223372036854775807']) {
      rejects(operation, [value, '0']);
    }
  }
});

test('DecodeRune validates both int32 inputs without treating invalid pairs as malformed input', () => {
  for (const inputs of [
    ['-2147483648', '2147483647'], ['2147483647', '-2147483648'],
    ['55296', '56320'], ['56319', '57343'], ['55357', '56832'],
    ['56832', '55357'], ['65533', '65533'], ['0', '0'], ['-1', '-1']
  ]) {
    accepts('utf16.DecodeRune', inputs);
  }
  for (const value of ['-2147483649', '2147483648', '9007199254740993']) {
    rejects('utf16.DecodeRune', [value, '0']);
    rejects('utf16.DecodeRune', ['0', value]);
  }
});

test('each byte operation accepts exactly the unsigned byte range', () => {
  for (const operation of byteOperations) {
    for (let value = 0; value <= 255; value++) accepts(operation, [String(value), '0']);
    for (const value of ['-1', '256', '257', '-2147483648', '2147483647', '9007199254740993']) {
      rejects(operation, [value, '0']);
    }
  }
});

test('integer spellings must be canonical decimal strings in either active input', () => {
  for (const value of [
    '', '-0', '+0', '+1', '00', '01', '-01', ' 1', '1 ', '\t1', '1\n',
    '1.0', '.1', '1e2', '1E2', '0x10', '0b10', '1_000', '--1',
    'NaN', 'Infinity', '-Infinity', '\u22121', '\uff11', '1\u0000', '%31'
  ]) {
    rejects('http.StatusText', [value, '0']);
    rejects('utf16.DecodeRune', ['0', value]);
  }
});

test('unused second input must be the literal string zero for every unary operation', () => {
  for (const operation of operations.filter(value => value !== 'utf16.DecodeRune')) {
    for (const value of ['1', '-1', '-0', '00', '', '+0', '0 ']) rejects(operation, ['0', value]);
  }
});

test('unknown and inherited-object operation names are rejected', () => {
  for (const operation of ['', 'http.statustext', 'http.StatusText ', 'bits.Len16', 'constructor', '__proto__', 'toString']) {
    rejects(operation, ['0', '0']);
  }
  for (const operation of [null, undefined, 1, true, ['http.StatusText'], new String('http.StatusText')]) {
    assert.throws(() => share.validateInputs(operation, ['0', '0']));
    assert.throws(() => share.format(state({ operation })));
  }
});

test('input containers and values are never coerced into the required two strings', () => {
  for (const inputs of [
    null, undefined, '404,0', {}, { 0: '404', 1: '0', length: 2 },
    [], ['404'], ['404', '0', '0'], new Array(2), [undefined, '0'],
    [404, '0'], ['404', 0], [404n, '0'], [true, '0'], [null, '0'],
    [new String('404'), '0'], ['404', new String('0')]
  ]) {
    assert.throws(() => share.validateInputs('http.StatusText', inputs));
    assert.throws(() => share.format(state({ inputs })));
  }
});

test('fragment requires every field exactly once and rejects unknown fields', () => {
  for (const key of Object.keys(fields)) {
    const params = new URLSearchParams(fields);
    params.delete(key);
    assert.throws(() => share.parse('#gooo-share?' + params.toString()), `missing ${key}`);
    assert.throws(() => share.parse(fragment() + '&' + key + '=' + encodeURIComponent(fields[key])), `duplicate ${key}`);
  }
  for (const suffix of ['&extra=1', '&__proto__=x', '&operation=http.StatusText', '&%76=1', '&a=405']) {
    assert.throws(() => share.parse(fragment() + suffix));
  }
  for (const value of ['', '0', '2', '01', '1.0', '+1']) assert.throws(() => share.parse(fragment({ v: value })));
});

test('reserved prefix cannot silently fall back to an ordinary page anchor', () => {
  for (const value of [
    '#gooo-share', '#gooo-share?', '#gooo-share&v=1', '#gooo-share/v=1',
    '#gooo-share-other', '#gooo-share#workbench', '#gooo-share?v=1#workbench',
    fragment() + '#workbench', '#gooo-share?' + 'a'.repeat(1024)
  ]) {
    assert.throws(() => share.parse(value), value.slice(0, 100));
  }
});

test('hash and revision identities require exact lowercase hexadecimal strings', () => {
  for (const key of ['wasm', 'revision']) {
    const size = key === 'wasm' ? 64 : 40;
    for (const value of [
      '', 'a'.repeat(size - 1), 'a'.repeat(size + 1), 'A'.repeat(size),
      'g'.repeat(size), ' ' + 'a'.repeat(size - 1), 'a'.repeat(size - 1) + '\n',
      null, undefined, 0, true, new String('a'.repeat(size)), ['a'.repeat(size)]
    ]) {
      assert.throws(() => share.format(state({ [key]: value })));
      if (typeof value === 'string') {
        assert.throws(() => share.parse(fragment({ [key === 'wasm' ? 'wasm' : 'rev']: value })));
      }
    }
  }
});

test('state and fragment argument shapes are validated without coercion', () => {
  for (const value of [null, undefined, 1, true, 'state', [], {}]) assert.throws(() => share.format(value));
  for (const key of ['operation', 'inputs', 'wasm', 'revision']) {
    const incomplete = state();
    delete incomplete[key];
    assert.throws(() => share.format(incomplete), `missing state ${key}`);
  }
  for (const value of [null, undefined, 1, true, [], {}, new String(fragment())]) assert.throws(() => share.parse(value));
});
