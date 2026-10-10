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

const colorOperations = [['color.RGBToCMYK', 3], ['color.CMYKToRGB', 4]];

test('color links use v2 and preserve every channel in order', () => {
  for (const [operation, count] of colorOperations) {
    const inputs = ['1', '17', '128', '255'].slice(0, count);
    accepts(operation, inputs);
    const encoded = share.format(state({ operation, inputs }));
    assert.equal(encoded, `#gooo-share?v=2&op=${operation}&a=1&b=17&c=128${count === 4 ? '&d=255' : ''}&wasm=${wasm}&rev=${revision}`);
    const params = new URLSearchParams(encoded.split('?')[1]);
    for (const [key, value] of params) {
      const missing = new URLSearchParams(params); missing.delete(key);
      assert.throws(() => share.parse('#gooo-share?' + missing));
      assert.throws(() => share.parse(encoded + `&${key}=${value}`));
    }
    assert.throws(() => share.parse(encoded + (count === 3 ? '&d=0' : '&e=0')));
    for (const suffix of ['&result=trusted', '&%63=128', '&extra=0']) assert.throws(() => share.parse(encoded + suffix));
    for (const version of ['1', '0', '3', '02', '2.0', '']) assert.throws(() => share.parse(encoded.replace('v=2', 'v=' + version)));
    for (let n = 0; n <= 5; n++) if (n !== count) rejects(operation, Array(n).fill('0'));
    rejects(operation, new Array(count));
  }
});

test('every color channel accepts the full byte range and rejects invalid spellings or types', () => {
  for (const [operation, count] of colorOperations) {
    for (let index = 0; index < count; index++) {
      for (let channel = 0; channel <= 255; channel++) {
        const inputs = Array(count).fill('0'); inputs[index] = String(channel);
        accepts(operation, inputs);
      }
      for (const value of ['-1', '256', '9007199254740993', '9223372036854775807', '-9223372036854775808', '', '-0', '+0', '01', ' 1', '1 ', '1.0', '1e2', '0xff', '1\n', 0, 255n, null, undefined, new String('1')]) {
        const inputs = Array(count).fill('0'); inputs[index] = value;
        rejects(operation, inputs);
        if (typeof value === 'string') {
          const params = new URLSearchParams(share.format(state({ operation, inputs: Array(count).fill('0') })).split('?')[1]);
          params.set(['a', 'b', 'c', 'd'][index], value);
          assert.throws(() => share.parse('#gooo-share?' + params));
        }
      }
    }
  }
});

test('rotation v1 links keep byte and signed int64 count domains separate', () => {
  const operation = 'bits.RotateLeft8';
  const counts = ['-9223372036854775808', '-9223372036854775807', '-9007199254740993', '-9007199254740992', '-9', '-8', '-1', '0', '1', '7', '8', '9', '9007199254740992', '9007199254740993', '9223372036854775806', '9223372036854775807'];
  for (let x = 0; x <= 255; x++) for (const k of counts) accepts(operation, [String(x), k]);
  for (const value of ['-1', '256', '-256', '257', '9007199254740993', '-9223372036854775808', '9223372036854775807']) rejects(operation, [value, '1']);
  for (const position of [0, 1]) {
    for (const value of ['', '-0', '+1', '01', ' 1', '1 ', '1.0', '1e2', '0xff', 'NaN', '１２', '9223372036854775808', '-9223372036854775809', 0, 255n, null, undefined]) {
      const inputs = ['129', '1']; inputs[position] = value;
      rejects(operation, inputs);
    }
  }
  for (let n = 0; n <= 5; n++) if (n !== 2) rejects(operation, Array(n).fill('0'));
  rejects(operation, new Array(2));
  const a = share.format(state({ operation, inputs: ['129', '9007199254740992'] }));
  const b = share.format(state({ operation, inputs: ['129', '9007199254740993'] }));
  assert.notEqual(a, b);
  assert.match(a, /^#gooo-share\?v=1&/);
  assert.throws(() => share.parse(a.replace('v=1', 'v=2')));
});


test('duration v1 links preserve both signed int64 values including nonpositive multiples', () => {
  const values = ['-9223372036854775808', '-9223372036854775807', '-9007199254740993', '-1', '0', '1', '9007199254740992', '9007199254740993', '9223372036854775807'];
  for (const operation of ['time.Duration.Round', 'time.Duration.Truncate']) {
    for (const duration of values) for (const multiple of values) accepts(operation, [duration, multiple]);
    for (const position of [0, 1]) {
      for (const value of ['', '-0', '+1', '01', ' 1', '1 ', '1.0', '1e2', '0xff', 'NaN', '１２', '9223372036854775808', '-9223372036854775809', 0, 1n, null, undefined]) {
        const inputs = ['9007199254740993', '2']; inputs[position] = value;
        rejects(operation, inputs);
      }
    }
    for (let n = 0; n <= 5; n++) if (n !== 2) rejects(operation, Array(n).fill('0'));
    rejects(operation, new Array(2));
    const encoded = share.format(state({ operation, inputs: ['-9223372036854775808', '9223372036854775807'] }));
    assert.match(encoded, /^#gooo-share\?v=1&/);
    assert.throws(() => share.parse(encoded.replace('v=1', 'v=2')));
    assert.throws(() => share.parse(encoded + '&c=0'));
    assert.throws(() => share.parse(encoded + '&result=0'));
  }
});

test('IsSpace v1 links preserve the signed rune domain, including surrogate and non-scalar values', () => {
  for (const input of ['-2147483648', '-1', '0', '9', '133', '160', '8203', '55296', '57343', '65279', '128512', '1114112', '2147483647']) {
    const value = { operation: 'unicode.IsSpace', inputs: [input, '0'], wasm: 'a'.repeat(64), revision: 'b'.repeat(40) };
    const fragment = share.format(value);
    assert.ok(fragment.startsWith('#gooo-share?v=1&'));
    assert.deepEqual(share.parse(fragment), value);
  }
  for (const input of ['-2147483649', '2147483648', '4294967328', '9007199254740993', ' 32', '+32', '032', '-0', '1e2']) {
    assert.throws(() => share.validateInputs('unicode.IsSpace', [input, '0']));
  }
  for (const inputs of [[], ['32'], ['32', '1'], ['32', '00'], ['32', '0', '0']]) assert.throws(() => share.validateInputs('unicode.IsSpace', inputs));
});

test('single-codepoint decoding is exact UTF-16 decoding, not trimming or whitespace classification', () => {
  const helper = require('../site/codepoint.js');
  for (const [text, decimal, notation, scalar] of [[' ', '32', 'U+0020', true], ['\t', '9', 'U+0009', true], ['\r', '13', 'U+000D', true], ['\u0085', '133', 'U+0085', true], ['\u00a0', '160', 'U+00A0', true], ['\u200b', '8203', 'U+200B', true], ['\ufeff', '65279', 'U+FEFF', true], ['😀', '128512', 'U+1F600', true], ['\ud800', '55296', 'U+D800', false], ['\udfff', '57343', 'U+DFFF', false]]) {
    assert.deepEqual(helper.fromText(text), { decimal, notation, scalar, carriageReturn: text === '\r' });
  }
  for (const text of ['', '  ', '\r\n', 'e\u0301', '👩‍💻', '🇰🇷', '\ud800x', '😀a', null, 32]) assert.throws(() => helper.fromText(text), /exactly one code point/);
});

test('classification share links retain signed rune boundaries and reject narrowing aliases', () => {
  for (const operation of ['unicode.IsLetter', 'unicode.IsDigit']) {
    for (const input of ['-2147483648', '-1', '0', '65', '55296', '1114112', '2147483647']) {
      const value = { operation, inputs: [input, '0'], wasm: 'a'.repeat(64), revision: 'b'.repeat(40) };
      assert.deepEqual(share.parse(share.format(value)), value);
    }
    for (const input of ['-2147483649', '2147483648', '4294967361', '4294967345', '065', '+65', '-0']) assert.throws(() => share.validateInputs(operation, [input, '0']));
    for (const inputs of [[], ['65'], ['65', '1'], ['65', '0', '0']]) assert.throws(() => share.validateInputs(operation, inputs));
  }
});
