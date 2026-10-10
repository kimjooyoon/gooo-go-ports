const test = require('node:test');
const assert = require('node:assert/strict');
const text = require('../site/text-input.js');
const reports = require('../site/text-inspection-report.js');
const binding = { wasmSha256: 'a'.repeat(64), sourceRevision: 'b'.repeat(40) };
const clone = value => JSON.parse(JSON.stringify(value));
// Authored observations test report transport and aggregation, not Go semantics.
const observed = (input, validScalar = true, goSpace = false, utf8Bytes = '1', utf16Units = '1', goLetter = false, goDigit = false) =>
  ({ input: String(input), validScalar, goSpace, goLetter, goDigit, utf8Bytes, utf16Units });
const ascii = raw => reports.create(text.decode(raw), [...raw].map(char => observed(char.charCodeAt(0))), binding);

test('report preserves CRLF, combining marks, supplementary and lone-surrogate units through JSON pasteback', () => {
  const raw = 'A\r\n\u0301😀\uD800B\uDFFF';
  const snapshot = text.decode(raw);
  const rows = [observed(65), observed(13, true, true), observed(10, true, true), observed(769, true, false, '2'),
    observed(128512, true, false, '4', '2'), observed(55296, false, false, '-1', '-1'), observed(66),
    observed(57343, false, false, '-1', '-1')];
  const report = reports.create(snapshot, rows, binding);
  assert.deepEqual(report.input.utf16Units, [65, 13, 10, 769, 55357, 56832, 55296, 66, 57343]);
  assert.equal(report.input.escapedJson, '"\\u0041\\u000D\\u000A\\u0301\\uD83D\\uDE00\\uD800\\u0042\\uDFFF"');
  assert.deepEqual(text.read(report.input.escapedJson, 'json'), snapshot);
  assert.deepEqual(report.rows.map(row => [row.codePoint, row.offset, row.width]),
    [[65, 0, 1], [13, 1, 1], [10, 2, 1], [769, 3, 1], [128512, 4, 2], [55296, 6, 1], [66, 7, 1], [57343, 8, 1]]);
  assert.deepEqual(report.totals, { codePoints: 8, receivedUtf16Units: 9, goWhitespaceItems: 2, utf8Bytes: null, utf16Units: null });
  assert.deepEqual(JSON.parse(reports.serialize(report)), report);
  assert.ok(reports.serialize(report).endsWith('\n'));
  assert.equal(report.kind, 'browser-observation');
  assert.match(report.scope, /Not a native compiler receipt/);
  assert.equal(Object.hasOwn(report, 'status'), false);
});

test('valid and empty totals aggregate received Gooo values without host classification', () => {
  // Deliberately arbitrary semantic values prove the helper does not classify A.
  const report = reports.create(text.decode('AA'), [observed(65, true, true, '4', '2'), observed(65, true, false, '3', '1')], binding);
  assert.deepEqual(report.totals, { codePoints: 2, receivedUtf16Units: 2, goWhitespaceItems: 1, utf8Bytes: 7, utf16Units: 3 });
  assert.deepEqual(JSON.parse(reports.serialize(report)), report);
  const empty = reports.create(text.decode(''), [], binding);
  assert.deepEqual(empty.input, { utf16Units: [], escapedJson: '""' });
  assert.deepEqual(empty.rows, []);
  assert.deepEqual(empty.totals, { codePoints: 0, receivedUtf16Units: 0, goWhitespaceItems: 0, utf8Bytes: 0, utf16Units: 0 });
});

test('either invalid scalar or unencodable received length makes both encoded totals unavailable', () => {
  for (const row of [observed(65, false), observed(65, true, false, '-1', '1'), observed(65, true, false, '1', '-1')]) {
    const report = reports.create(text.decode('A'), [row], binding);
    assert.equal(report.totals.utf8Bytes, null);
    assert.equal(report.totals.utf16Units, null);
    assert.equal(report.totals.receivedUtf16Units, 1);
  }
});

test('report clones and deeply freezes input, output and program binding', () => {
  const snapshot = clone(text.decode('A'));
  const rows = [observed(65)];
  const program = { ...binding };
  const report = reports.create(snapshot, rows, program);
  snapshot.items[0].decimal = '66'; rows[0].goSpace = true; program.wasmSha256 = 'c'.repeat(64);
  assert.equal(report.rows[0].codePoint, 65);
  assert.equal(report.rows[0].goSpace, false);
  assert.equal(report.program.wasmSha256, binding.wasmSha256);
  function frozen(value) {
    if (value && typeof value === 'object') { assert.ok(Object.isFrozen(value)); Object.values(value).forEach(frozen); }
  }
  frozen(report);
});

test('creation rejects inconsistent snapshot contents, offsets and original-unit renderings', () => {
  const original = text.decode('A😀');
  const rows = [observed(65), observed(128512, true, false, '4', '2')];
  const mutations = [s => { s.codeUnits--; }, s => { s.escaped = '"A😀"'; }, s => { s.items.pop(); },
    s => { s.items.reverse(); }, s => { s.items[1].offset++; }, s => { s.items[1].width = 1; },
    s => { s.items[0].decimal = '66'; }, s => { s.items[0].notation = 'U+0042'; },
    s => { s.items[1].units = '\\uFFFD'; }, s => { s.items[0].extra = true; }, s => { s.extra = true; }];
  for (const change of mutations) { const snapshot = clone(original); change(snapshot); assert.throws(() => reports.create(snapshot, rows, binding)); }
  assert.throws(() => reports.create({ items: [], codeUnits: 0, escaped: 'x'.repeat(3075) }, [], binding));
});

test('creation rejects incomplete, reordered, mistyped or misbound returned rows and identities', () => {
  const snapshot = text.decode('AB');
  const original = [observed(65), observed(66)];
  const mutations = [rows => { rows.pop(); }, rows => { rows.reverse(); }, rows => { rows[0].input = '065'; },
    rows => { rows[0].input = '66'; }, rows => { rows[0].goSpace = 'false'; }, rows => { rows[0].validScalar = null; },
    rows => { rows[0].utf8Bytes = 1; }, rows => { rows[0].utf16Units = '4'; }, rows => { delete rows[0].goSpace; },
    rows => { rows[0].extra = true; }];
  for (const change of mutations) { const rows = clone(original); change(rows); assert.throws(() => reports.create(snapshot, rows, binding)); }
  for (const value of [{}, { ...binding, wasmSha256: 'bad' }, { ...binding, sourceRevision: 'C'.repeat(40) }, { ...binding, extra: true }]) {
    assert.throws(() => reports.create(snapshot, original, value));
  }
});

test('serialization revalidates reconstruction, row roster, output types, totals and report identity', () => {
  const original = ascii('AB');
  const mutations = [r => { r.schema = 'other'; }, r => { r.kind = 'native-receipt'; }, r => { r.scope = 'PASS'; },
    r => { r.input.utf16Units[0] = 66; }, r => { r.input.escapedJson = '"\\u0041"'; },
    r => { r.input.utf16Units[0] = 65536; }, r => { r.input.utf16Units[0] = -0; }, r => { r.input.utf16Units[0] = '65'; },
    r => { r.rows.pop(); }, r => { r.rows.reverse(); }, r => { r.rows[0].codePoint = '65'; },
    r => { r.rows[0].offset = 1; }, r => { r.rows[0].width = 2; }, r => { r.rows[0].utf8Bytes = '01'; },
    r => { r.rows[0].utf16Units = '0'; }, r => { r.rows[0].goSpace = null; }, r => { r.rows[0].extra = true; },
    r => { r.totals.codePoints++; }, r => { r.totals.receivedUtf16Units++; }, r => { r.totals.goWhitespaceItems++; },
    r => { r.totals.utf8Bytes = null; }, r => { r.totals.utf16Units++; }, r => { r.program.wasmSha256 = 'x'; },
    r => { delete r.input; }, r => { r.status = 'PASS'; }];
  for (const change of mutations) { const report = clone(original); change(report); assert.throws(() => reports.serialize(report)); }
});

test('256 code points / 512 original UTF-16 units survive export and pasteback without truncation', () => {
  const raw = '😀'.repeat(256);
  const snapshot = text.decode(raw);
  const report = reports.create(snapshot, Array.from({ length: 256 }, () => observed(128512, true, false, '4', '2')), binding);
  assert.equal(report.rows.length, 256);
  assert.equal(report.input.utf16Units.length, 512);
  assert.equal(report.input.escapedJson.length, 3074);
  assert.deepEqual(report.totals, { codePoints: 256, receivedUtf16Units: 512, goWhitespaceItems: 0, utf8Bytes: 1024, utf16Units: 512 });
  assert.deepEqual(text.read(report.input.escapedJson, 'json'), snapshot);
  assert.deepEqual(JSON.parse(reports.serialize(report)), report);
  const tooManyUnits = clone(report); tooManyUnits.input.utf16Units.push(65);
  assert.throws(() => reports.serialize(tooManyUnits));
  const tooManyPoints = ascii('A'.repeat(256));
  const over = clone(tooManyPoints); over.input.utf16Units.push(65);
  assert.throws(() => reports.serialize(over));
});

test('v2 preserves both classification values and rejects absent or mistyped observations', () => {
  const original = reports.create(text.decode('A1'), [observed(65, true, false, '1', '1', true, false), observed(49, true, false, '1', '1', false, true)], binding);
  assert.equal(original.schema, 'gooo/text-inspection-report/v2');
  assert.deepEqual(JSON.parse(reports.serialize(original)).rows.map(r => [r.goLetter, r.goDigit]), [[true, false], [false, true]]);
  for (const key of ['goLetter', 'goDigit']) {
    for (const value of [undefined, null, 'true', 1]) {
      const report = clone(original); report.rows[0][key] = value;
      assert.throws(() => reports.serialize(report));
    }
  }
  const old = clone(original); old.schema = 'gooo/text-inspection-report/v1';
  assert.throws(() => reports.serialize(old));
});
