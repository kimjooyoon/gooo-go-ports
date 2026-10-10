// Run the actual Worker computation/loader and UI decoder against independent
// Go fixtures, using the same existing demo.wasm. No reference math is bundled.
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { webcrypto, createHash } = require('node:crypto');

async function runSmoke(directory) {
  const dir = path.resolve(directory);
  const read = name => JSON.parse(fs.readFileSync(path.join(dir, name), 'utf8'));
  const hash = name => createHash('sha256').update(fs.readFileSync(path.join(dir, name))).digest('hex');
  const protocol = require(path.join(dir, 'text-inspector-protocol.js'));
  const worker = require(path.join(dir, 'text-inspector-worker.js'));
  const text = require(path.join(dir, 'text-input.js'));
  const reports = require(path.join(dir, 'text-inspection-report.js'));
  const fixture = read('text-inspector-fixtures.json');
  const metadata = read('metadata.json');
  assert.equal(fixture.schema, 'gooo/text-inspector-fixtures/v2');
  assert.equal(fixture.go_version, metadata.go_version);
  assert.equal(fixture.compiler_revision, metadata.compiler_revision);
  assert.deepEqual(fixture.limits, { max_code_points: 256, max_utf16_units: 512 });
  assert.ok(Array.isArray(fixture.cases) && fixture.cases.length > 0);
  assert.equal(hash('generated.go'), metadata.generated_sha256);
  globalThis.crypto ??= webcrypto;
  const allowed = new Set(['./metadata.json', './demo.wasm']);
  const scope = {
    crypto: globalThis.crypto,
    WebAssembly,
    async fetch(name) {
      assert.ok(allowed.has(name), `unexpected Worker fetch: ${name}`);
      const bytes = fs.readFileSync(path.join(dir, name));
      return { ok: true, json: async () => JSON.parse(bytes.toString('utf8')),
        arrayBuffer: async () => bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength) };
    },
    importScripts(name) {
      assert.equal(name, './wasm_exec.js');
      require(path.join(dir, name));
      this.Go = globalThis.Go;
    }
  };
  Object.defineProperties(scope, {
    goooReady: { get: () => globalThis.goooReady },
    goooRun: { get: () => globalThis.goooRun }
  });
  let runtimeFailure;
  const runtime = await worker.loadBrowserRuntime(scope, message => { runtimeFailure = message; });
  let observations = 0, calls = 0, requestId = 0, reportsCreated = 0, reportsRoundtripped = 0;
  const ids = new Set();
  for (const item of fixture.cases) {
    assert.equal(typeof item.id, 'string');
    assert.ok(!ids.has(item.id), `duplicate fixture ${item.id}`);
    ids.add(item.id);
    assert.ok(Array.isArray(item.units) && item.units.length <= 512);
    assert.ok(item.units.every(unit => Number.isInteger(unit) && unit >= 0 && unit <= 65535));
    const original = String.fromCharCode(...item.units);
    const decoded = text.decode(original);
    assert.equal(decoded.codeUnits, item.units.length, item.id);
    assert.equal(decoded.items.length, item.expected.length, item.id);
    assert.deepEqual(decoded.items.map(point => ({ codePoint: Number(point.decimal), offset: point.offset, units: point.width })),
      item.expected.map(point => ({ codePoint: point.codePoint, offset: point.offset, units: point.units })), item.id);
    const points = decoded.items.map(point => point.decimal);
    const id = ++requestId;
    const rows = await worker.inspectPoints(points, (...args) => { calls++; return runtime.run(...args); });
    const checked = protocol.validateResult({ type: 'result', id, rows, ...runtime.identity }, { id, points, ...runtime.identity });
    assert.deepEqual(checked, item.expected.map(point => ({ input: String(point.codePoint), validScalar: point.validScalar,
      goSpace: point.goSpace, goLetter: point.goLetter, goDigit: point.goDigit, utf8Bytes: String(point.utf8Bytes), utf16Units: String(point.utf16Units) })), item.id);
    const report = reports.create(decoded, checked, runtime.identity);
    const serialized = reports.serialize(report);
    const restored = JSON.parse(serialized);
    assert.equal(restored.schema, 'gooo/text-inspection-report/v2', item.id);
    assert.equal(restored.kind, 'browser-observation', item.id);
    assert.match(restored.scope, /Not a native compiler receipt or an independent reference-test result\./, item.id);
    assert.ok(!Object.hasOwn(restored, 'status'), item.id);
    assert.deepEqual(restored.program, runtime.identity, item.id);
    assert.deepEqual(restored.input.utf16Units, item.units, item.id);
    assert.equal(restored.input.escapedJson, decoded.escaped, item.id);
    assert.equal(JSON.parse(restored.input.escapedJson), original, item.id);
    assert.deepEqual(text.read(restored.input.escapedJson, 'json'), decoded, item.id);
    assert.deepEqual(restored.rows, item.expected.map(point => ({ codePoint: point.codePoint,
      offset: point.offset, width: point.units, validScalar: point.validScalar, goSpace: point.goSpace, goLetter: point.goLetter, goDigit: point.goDigit,
      utf8Bytes: String(point.utf8Bytes), utf16Units: String(point.utf16Units) })), item.id);
    // Expected totals come from the independent Go fixture fields, never from
    // JavaScript character classification or the report's own aggregate fields.
    const encodable = item.expected.every(point => point.validScalar && point.utf8Bytes !== -1 && point.utf16Units !== -1);
    assert.deepEqual(restored.totals, { codePoints: item.expected.length, receivedUtf16Units: item.units.length,
      goWhitespaceItems: item.expected.filter(point => point.goSpace).length,
      utf8Bytes: encodable ? item.expected.reduce((sum, point) => sum + point.utf8Bytes, 0) : null,
      utf16Units: encodable ? item.expected.reduce((sum, point) => sum + point.utf16Units, 0) : null }, item.id);
    reportsCreated++;
    assert.equal(reports.serialize(restored), serialized, item.id);
    reportsRoundtripped++;
    assert.equal(runtimeFailure, undefined);
    observations += points.length;
  }
  assert.equal(calls, observations * 6);
  return { schema: 'gooo/text-inspector-smoke/v1', status: 'PASS', text_cases: fixture.cases.length,
    code_point_observations: observations, compiled_operation_calls: calls,
    reports_created: reportsCreated, reports_roundtripped: reportsRoundtripped,
    operations: protocol.OPERATIONS, new_port_functions: 0,
    go_version: fixture.go_version, unicode_version: fixture.unicode_version,
    compiler_revision: fixture.compiler_revision, source_revision: runtime.identity.sourceRevision,
    wasm_sha256: runtime.identity.wasmSha256,
    source_sha256: Object.fromEntries(['text-inspector-protocol.js', 'text-inspector-worker.js', 'text-input.js', 'text-inspection-report.js',
      'text-inspector-fixtures.json', 'metadata.json', 'generated.go'].map(name => [name, hash(name)])),
    scope: 'Existing generated Gooo operations called through the actual Worker loader/computation with a Node file-fetch adapter; exact UI UTF-16 decoder and local observation report roundtrips checked against authored positions and independent pinned-Go semantic fixtures. This is not a native Gooo string API or a DOM/clipboard/download interaction test.' };
}

module.exports = { runSmoke };
if (require.main === module) {
  runSmoke(process.argv[2] || 'out/browser/site').then(result => {
    console.log(JSON.stringify(result));
    process.exit(0);
  }).catch(error => { console.error(error); process.exit(1); });
}
