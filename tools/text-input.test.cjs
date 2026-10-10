const test = require('node:test');
const assert = require('node:assert/strict');
const input = require('../site/text-input.js');

test('UTF-16 decoding preserves exact units, offsets, CRLF and unpaired values', () => {
  const result = input.decode('A\r\n😀\uD800x\uDFFF');
  assert.deepEqual(result.items.map(({ decimal, offset, width, units }) => ({ decimal, offset, width, units })), [
    { decimal: '65', offset: 0, width: 1, units: '\\u0041' },
    { decimal: '13', offset: 1, width: 1, units: '\\u000D' },
    { decimal: '10', offset: 2, width: 1, units: '\\u000A' },
    { decimal: '128512', offset: 3, width: 2, units: '\\uD83D \\uDE00' },
    { decimal: '55296', offset: 5, width: 1, units: '\\uD800' },
    { decimal: '120', offset: 6, width: 1, units: '\\u0078' },
    { decimal: '57343', offset: 7, width: 1, units: '\\uDFFF' }
  ]);
  assert.equal(result.codeUnits, 8);
  assert.equal(result.escaped, '"\\u0041\\u000D\\u000A\\uD83D\\uDE00\\uD800\\u0078\\uDFFF"');
  assert.equal(result.items[3].notation, 'U+1F600');
  assert.equal(Object.hasOwn(result.items[0], 'validScalar'), false);
  assert.equal(Object.hasOwn(result.items[0], 'goSpace'), false);
  assert.equal(Object.hasOwn(result.items[0], 'utf8Bytes'), false);
});

test('spaces, controls and BOM are decoded without trimming or classification', () => {
  assert.deepEqual(input.decode('\0\t\n\v\f\r \u0085\u00A0\u200B\uFEFF\u3000').items.map(point => point.decimal),
    ['0', '9', '10', '11', '12', '13', '32', '133', '160', '8203', '65279', '12288']);
});

test('combining marks, joined emoji and flags remain separate code points', () => {
  assert.deepEqual(input.decode('e\u0301👩‍💻🇰🇷').items.map(point => point.decimal), ['101', '769', '128105', '8205', '128187', '127472', '127479']);
  assert.deepEqual(input.decode('é').items.map(point => point.decimal), ['233']);
  assert.equal(input.decode('e\u0301').items.length, 2);
});

test('surrogate decoding pairs only an adjacent high then low unit', () => {
  for (const [text, points] of [
    ['\uD800\uDC00', ['65536']], ['\uDBFF\uDFFF', ['1114111']],
    ['\uDC00\uD800', ['56320', '55296']], ['\uD800\uD800\uDC00', ['55296', '65536']],
    ['\uDFFF\uDBFF\uDFFF', ['57343', '1114111']], ['\uFFFD', ['65533']]
  ]) assert.deepEqual(input.decode(text).items.map(point => point.decimal), points);
});

test('complete-input limits accept their boundaries and reject without truncation', () => {
  assert.equal(input.decode('x'.repeat(256)).items.length, 256);
  const pairs = input.decode('😀'.repeat(256));
  assert.equal(pairs.items.length, 256);
  assert.equal(pairs.codeUnits, 512);
  assert.equal(pairs.items[255].offset, 510);
  assert.throws(() => input.decode('x'.repeat(257)), /256 decoded items/);
  assert.throws(() => input.decode('😀'.repeat(256) + 'x'), /512 UTF-16/);
  assert.throws(() => input.decode('x'.repeat(1_000_000)), /512 UTF-16/);
  assert.throws(() => input.decode('\uD800'.repeat(512)), /256 decoded items/);
});

test('JSON input supports explicit CRLF and lone surrogates without replacement', () => {
  const decoded = input.read('"\\r\\n\\uD800A\\uDFFF\\uFFFD"', 'json');
  assert.deepEqual(decoded.items.map(point => point.decimal), ['13', '10', '55296', '65', '57343', '65533']);
  assert.equal(decoded.escaped, '"\\u000D\\u000A\\uD800\\u0041\\uDFFF\\uFFFD"');
  assert.deepEqual(input.read('"\\uD83D\\uDE00"', 'json'), input.read('😀', 'text'));
  assert.equal(input.read('"\\\\uD800"', 'json').items.length, 6, 'escaped backslash must remain literal text');
});

test('JSON source is bounded before parsing while all 512 escaped units still fit', () => {
  const maximumPairs = '"' + '\\uD83D\\uDE00'.repeat(256) + '"';
  assert.equal(maximumPairs.length, 3074);
  assert.equal(input.read(maximumPairs, 'json').items.length, 256);
  assert.equal(input.read(' '.repeat(4094) + '""', 'json').items.length, 0);
  assert.throws(() => input.read(' '.repeat(4095) + '""', 'json'), /4,096/);
  assert.throws(() => input.read('"' + 'x'.repeat(257) + '"', 'json'), /256 decoded items/);
  assert.throws(() => input.read('"' + 'x'.repeat(513) + '"', 'json'), /512 UTF-16/);
});

test('empty text is valid; invalid JSON and nonstring values are rejected', () => {
  for (const result of [input.read('', 'text'), input.read('""', 'json')]) {
    assert.equal(result.items.length, 0); assert.equal(result.codeUnits, 0); assert.equal(result.escaped, '""');
  }
  for (const source of ['', '"', '"\\uZZZZ"', '"x" "y"', '"\n"']) assert.throws(() => input.read(source, 'json'), /valid JSON string/);
  for (const source of ['null', '1', 'true', '[]', '{}']) assert.throws(() => input.read(source, 'json'), /must be a string/);
  assert.throws(() => input.read('x', 'unknown'), /Choose Text/);
  assert.throws(() => input.read(null, 'text'), /must be text/);
});

test('all presets contain their advertised unmodified sequences', () => {
  assert.deepEqual(input.read(input.presets.mixed.source, 'text').items.map(point => point.decimal), ['65', '160', '66', '13', '10', '8203', '128512']);
  assert.deepEqual(input.read(input.presets.boundaries.source, 'text').items.map(point => point.decimal), ['133', '65279']);
  assert.equal(input.read(input.presets.combined.source, 'text').items.length, 5);
  assert.equal(input.presets.surrogates.mode, 'json');
  assert.deepEqual(input.read(input.presets.surrogates.source, 'json').items.map(point => point.decimal), ['55296', '65', '57343', '65533']);
  assert.ok(Object.isFrozen(input.decode('x').items[0]));
});
