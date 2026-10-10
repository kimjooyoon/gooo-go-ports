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
  const sharing = require(path.join(dir, 'share.js'));
  assert.equal(hash('demo.wasm'), metadata.wasm_sha256);
  assert.equal(hash('generated.go'), metadata.generated_sha256);
  assert.equal(hash('execution.json'), metadata.execution_sha256);
  assert.equal(metadata.schema, 'gooo/browser-demo/v2');
  assert.equal(hash('evidence-index.json'), metadata.evidence_index_sha256);
  const evidence = read('evidence-index.json');
  assert.equal(evidence.schema, 'gooo/browser-build-evidence/v1');
  assert.equal(hash(evidence.source_cases), evidence.source_cases_sha256);
  const sourceCases = read(evidence.source_cases).cases;
  assert.equal(evidence.cases, sourceCases.length);
  assert.equal(metadata.native_cases, evidence.cases);
  assert.equal(metadata.native_batches, evidence.batches.length);
  assert.equal(evidence.generated_sha256, metadata.generated_sha256);
  let nativeCases = 0;
  for (const [index, batch] of evidence.batches.entries()) {
    const prefix = `evidence/${String(index).padStart(3, '0')}/`;
    assert.equal(batch.case_file, prefix + 'cases.json');
    assert.equal(batch.execution, prefix + 'execution.json');
    assert.equal(batch.replay, prefix + 'replay.json');
    assert.equal(hash(batch.case_file), batch.case_sha256);
    assert.equal(hash(batch.execution), batch.execution_sha256);
    assert.equal(hash(batch.replay), batch.replay_sha256);
    const cases = read(batch.case_file).cases;
    assert.ok(cases.length > 0 && cases.length <= 128);
    assert.ok(fs.statSync(path.join(dir, batch.case_file)).size <= 32768);
    assert.equal(cases.length, batch.cases);
    nativeCases += batch.cases;
    for (const receiptPath of [batch.execution, batch.replay]) {
      const record = read(receiptPath);
      assert.equal(record.cases_digest, 'sha256:' + batch.case_sha256);
      assert.equal(record.result.composition.source, fs.readFileSync(path.join(dir, 'generated.go'), 'utf8'));
      assert.equal(record.result.runtime.producer_source_sha, metadata.compiler_revision);
      assert.equal(record.result.runtime.finite_total, batch.cases);
      assert.equal(record.result.runtime.finite_passed, batch.cases);
    }
    assert.equal(read(batch.replay).replayed_from_sha256, 'sha256:' + batch.execution_sha256);
  }
  assert.equal(nativeCases, sourceCases.length);
  assert.equal(hash('execution.json'), evidence.batches[0].execution_sha256);
  assert.equal(hash('replay.json'), evidence.batches[0].replay_sha256);
  const execution = read('execution.json');
  assert.equal(execution.result.composition.source, fs.readFileSync(path.join(dir, 'generated.go'), 'utf8'));
  const go = new Go();
  const { instance } = await WebAssembly.instantiate(fs.readFileSync(path.join(dir, 'demo.wasm')), go.importObject);
  go.run(instance).catch(error => { console.error(error); process.exit(1); });
  assert.equal(globalThis.goooReady, true);
  const fixture = read('fixtures.json');
  assert.equal(fixture.schema, 'gooo/browser-fixtures/v2');
  assert.equal(fixture.cases.length, 6909);
  assert.equal(new Set(fixture.cases.map(row => row.operation)).size, 16);
  for (const row of fixture.cases) {
    const shared = sharing.parse(sharing.format({ operation: row.operation, inputs: row.inputs,
      wasm: metadata.wasm_sha256, revision: metadata.source_revision }));
    assert.equal(shared.operation, row.operation);
    assert.deepEqual(shared.inputs, row.inputs);
    const actual = JSON.parse(globalThis.goooRun(shared.operation, ...shared.inputs));
    assert.equal(actual.ok, true, JSON.stringify(row));
    assert.equal(actual.operation, row.operation);
    assert.deepEqual(actual.inputs, row.inputs);
    assert.deepEqual(actual.output, row.expected, JSON.stringify(row));
  }
  for (const row of fixture.rejections) {
    assert.equal(JSON.parse(globalThis.goooRun(row.operation, ...row.inputs)).ok, false, JSON.stringify(row));
  }
  const malformed = [[], ['bits.Len8', 255, '0'], ['bits.Len8', '255', 0], [null, '0', '0'],
    ['bits.Len8', '0'], ['bits.Len8', '0', '0', '0']];
  for (const [op, arity] of [['color.RGBToCMYK', 3], ['color.CMYKToRGB', 4], ['bits.RotateLeft8', 2]]) {
    for (let position = 0; position < arity; position++) {
      for (const wrongType of [0, true, null]) {
        const args = [op, ...Array(arity).fill('0')];
        args[position + 1] = wrongType;
        malformed.push(args);
      }
    }
  }
  for (const args of malformed) {
    assert.equal(JSON.parse(globalThis.goooRun(...args)).ok, false, JSON.stringify(args));
  }
  console.log(JSON.stringify({schema:'gooo/browser-smoke/v2', functions:16, native_cases:evidence.cases, native_batches:evidence.batches.length, cases:fixture.cases.length,
    shared_inputs:fixture.cases.length, rejected:fixture.rejections.length + malformed.length,
    wasm_sha256:metadata.wasm_sha256, status:'PASS'}));
  process.exit(0);
})().catch(error => { console.error(error); process.exit(1); });
