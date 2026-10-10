// Exercise the same unmodified Go Wasm runtime, module and bridge used by the UI.
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { webcrypto, createHash } = require('node:crypto');
globalThis.crypto ??= webcrypto;
const dir = path.resolve(process.argv[2] || 'out/browser/site');
const read = name => JSON.parse(fs.readFileSync(path.join(dir, name), 'utf8'));
const hash = name => createHash('sha256').update(fs.readFileSync(path.join(dir, name))).digest('hex');
require(path.join(dir, 'wasm_exec.js'));
(async () => {
  const metadata = read('metadata.json');
  assert.equal(hash('demo.wasm'), metadata.wasm_sha256);
  assert.equal(hash('generated.go'), metadata.generated_sha256);
  assert.equal(hash('execution.json'), metadata.execution_sha256);
  const execution = read('execution.json');
  assert.equal(execution.result.composition.source, fs.readFileSync(path.join(dir, 'generated.go'), 'utf8'));
  const go = new Go();
  const { instance } = await WebAssembly.instantiate(fs.readFileSync(path.join(dir, 'demo.wasm')), go.importObject);
  go.run(instance).catch(error => { console.error(error); process.exit(1); });
  assert.equal(globalThis.goooReady, true);
  const fixture = read('fixtures.json');
  for (const row of fixture.cases) {
    const actual = JSON.parse(globalThis.goooRun(row.operation, ...row.inputs));
    assert.equal(actual.ok, true, JSON.stringify(row));
    assert.equal(actual.operation, row.operation);
    assert.deepEqual(actual.inputs, row.inputs);
    assert.deepEqual(actual.output, row.expected, JSON.stringify(row));
  }
  for (const row of fixture.rejections) {
    assert.equal(JSON.parse(globalThis.goooRun(row.operation, ...row.inputs)).ok, false, JSON.stringify(row));
  }
  for (const args of [[], ['bits.Len8', 255, '0'], ['bits.Len8', '255', 0], [null, '0', '0']]) {
    assert.equal(JSON.parse(globalThis.goooRun(...args)).ok, false);
  }
  console.log(JSON.stringify({schema:'gooo/browser-smoke/v1', cases:fixture.cases.length,
    rejected:fixture.rejections.length + 4, wasm_sha256:metadata.wasm_sha256, status:'PASS'}));
  process.exit(0);
})().catch(error => { console.error(error); process.exit(1); });
