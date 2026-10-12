'use strict';

// Actual shipped Go Wasm plus production Worker/protocol. This file never
// computes UUID parse results in JavaScript. Fixtures come from original Go.
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { createHash, webcrypto } = require('node:crypto');
const { Worker } = require('node:worker_threads');
const fields = ['parsed', 'canonical_uuid', 'error', 'invalid_length'];
const fieldIDs = ['parsed', 'canonical-uuid', 'error', 'invalid-length'];
const entry = 'gooo://github.com/google/uuid/text/parse-canonical';
const resultEntity = 'gooo://github.com/google/uuid/text/parse-canonical-result';
const inputKey = 'go/github.com/google/uuid/text:ParseCanonical';
const sources = { 'ports/google/uuid/quote.gooo': 'go/github.com/google/uuid/internal/quote',
  'ports/google/uuid/validate.gooo': 'go/github.com/google/uuid', 'ports/google/uuid/text/parse_canonical.gooo': 'go/github.com/google/uuid/text' };
const originalHashes = { 'uuid.go': '0edec8e34c6b6fe0db31b71a29069a09ed832e3fd04ee0175916b58f2b60e5c1',
  'util.go': '755f50afafb13dd89ae6177da7eb6dc31ae650ece2f9ba52821021e9f8de044e', 'LICENSE': '0a8d61ed3cbfd5312326e8126c31ce9c627a283adc99131b56896d29ada04b2d' };
const keys = (value, expected) => { assert.ok(value && typeof value === 'object' && !Array.isArray(value)); assert.deepEqual(Object.keys(value).sort(), [...expected].sort()); };
const hashBytes = raw => createHash('sha256').update(raw).digest('hex');
const unitsText = values => { assert.ok(Array.isArray(values)); for (const value of values) assert.ok(Number.isInteger(value) && value >= 0 && value <= 65535); return String.fromCharCode(...values); };
const encodedString = value => JSON.stringify(value).replace(/[<>&\u2028\u2029]/g, c => '\\u' + c.charCodeAt(0).toString(16).padStart(4, '0'));

async function actualWorkerSmoke(site, fixture, manifest, protocol) {
  // A real thread executes the shipped loader, protocol, handler and Wasm.
  // Only fetch/importScripts transport maps URLs onto the static output tree.
  const worker = new Worker(`
    const { parentPort, workerData } = require('node:worker_threads');
    const fs = require('node:fs'), path = require('node:path'), assert = require('node:assert/strict');
    globalThis.crypto ??= require('node:crypto').webcrypto;
    globalThis.fetch = async (url, options) => {
      assert.equal(options.cache, 'no-store'); assert.ok(['./uuid-parse/manifest.json', './uuid-parse/program.wasm'].includes(url));
      const raw = fs.readFileSync(path.join(workerData.site, url));
      return { ok: true, json: async () => JSON.parse(raw.toString('utf8')), arrayBuffer: async () => raw.buffer.slice(raw.byteOffset, raw.byteOffset + raw.byteLength) };
    };
    globalThis.importScripts = (...names) => { for (const name of names) { assert.equal(name, './wasm_exec.js'); require(path.join(workerData.site, name)); } };
    globalThis.postMessage = data => parentPort.postMessage(data);
    globalThis.addEventListener = (event, handler) => { assert.equal(event, 'message'); parentPort.on('message', data => handler({ data })); };
    require(path.join(workerData.site, 'uuid-parse-worker.js')).startWorker(globalThis);
  `, { eval: true, workerData: { site } });
  const messages = []; let waiting, failure;
  worker.on('message', message => { if (waiting) { const resolve = waiting; waiting = null; resolve(message); } else messages.push(message); });
  worker.on('error', error => { failure = error; if (waiting) { const resolve = waiting; waiting = null; resolve({ workerFailure: error.message }); } });
  const next = () => new Promise((resolve, reject) => {
    if (failure) return reject(failure); if (messages.length) return resolve(messages.shift());
    const timer = setTimeout(() => { waiting = null; reject(new Error('Actual UUID Worker timed out.')); }, 10000);
    waiting = message => { clearTimeout(timer); if (message.workerFailure) reject(new Error(message.workerFailure)); else resolve(message); };
  });
  try {
    const program = protocol.validateReady(await next()); assert.deepEqual(JSON.parse(JSON.stringify(program)), manifest);
    let id = 0;
    for (const row of fixture.cases) {
      const request = { type: 'parse', id: ++id, inputs: [row.input] }; worker.postMessage(request);
      assert.deepEqual(protocol.validateResult(await next(), { ...request, program }), row.expected, row.name);
    }
    for (const request of [ { type: 'parse', id: 1, inputs: [''] }, { type: 'parse', id: ++id, inputs: [0] },
      { type: 'parse', id: ++id, inputs: ['a'.repeat(1025)] }, { type: 'parse', id: ++id, inputs: ['\uD800'] },
      { type: 'parse', id: ++id, inputs: [''], unexpected: true }, { type: 'unknown', id: ++id, inputs: [''] } ]) {
      worker.postMessage(request); protocol.validateError(await next(), request.id);
    }
    return { cases: fixture.cases.length, request_rejections: 6 };
  } finally { await worker.terminate(); }
}

async function runSmoke(directory) {
  const site = path.resolve(directory), dir = path.join(site, 'uuid-parse');
  const bytes = name => fs.readFileSync(path.join(dir, name)), read = name => JSON.parse(bytes(name).toString('utf8')), hash = name => hashBytes(bytes(name));
  const protocol = require(path.join(site, 'uuid-parse-protocol.js')), manifest = read('manifest.json'), lock = read('upstream.json');
  protocol.validateMetadata(manifest);
  for (const key of ['compiler_revision', 'compiler_version', 'go_version']) assert.equal(manifest[key], lock[key]);
  const files = ['workspace.json', 'compiler.json', 'inputs.json', 'execution.json', 'replay.json', 'checked-report.json',
    'boundary-inputs.json', 'boundary-execution.json', 'boundary-replay.json', 'boundary-checked-report.json',
    'generated.go', 'bridge.go', 'fixtures.json', 'LICENSE', 'NOTICE', 'upstream.json', 'third_party/GO-LICENSE',
    'third_party/google-uuid/LICENSE', 'third_party/google-uuid/uuid.go', 'third_party/google-uuid/util.go', 'program.wasm'];
  keys(manifest.files_sha256, files);
  for (const [name, digest] of Object.entries(manifest.files_sha256)) assert.equal(hash(name), digest, name);
  for (const [name, key] of [['program.wasm', 'wasm_sha256'], ['generated.go', 'generated_sha256'], ['execution.json', 'execution_sha256'],
    ['replay.json', 'replay_sha256'], ['fixtures.json', 'fixtures_sha256']]) assert.equal(hash(name), manifest[key], name);
  assert.equal(bytes('program.wasm').length, manifest.wasm_bytes);
  keys(manifest.source_sha256, Object.keys(sources)); const sourceBindings = {}, checkedSourceBindings = {};
  for (const [name, packageName] of Object.entries(sources)) {
    assert.equal(hash('bound-source/' + name), manifest.source_sha256[name]);
    const digest = 'sha256:' + hashBytes(Buffer.from(encodedString(bytes('bound-source/' + name).toString('utf8'))));
    sourceBindings[name] = digest; checkedSourceBindings[name] = { package: packageName, source_digest: digest };
  }
  for (const [name, digest] of Object.entries(originalHashes)) assert.equal(hash('third_party/google-uuid/' + name), digest);
  const upstream = lock.external_sources.filter(value => value.target === 'ports/google/uuid/text/parse_canonical.gooo'); assert.equal(upstream.length, 1);
  const original = upstream[0]; assert.equal(original.module, 'github.com/google/uuid'); assert.equal(original.version, 'v1.6.0');
  assert.equal(original.revision, '0f11ee6918f41a04c201eceeadf612a377bc7fbc'); assert.equal(original.module_h1, 'h1:NIvaJDMOsjHA8n1jAhLSgzrAzy1Hgr+hNrb57e+94F0=');
  assert.equal(original.source_sha256, originalHashes['uuid.go']); assert.deepEqual(original.supporting_source_sha256, { 'util.go': originalHashes['util.go'] }); assert.equal(original.license_sha256, originalHashes.LICENSE);
  const compiler = read('compiler.json'); assert.equal(compiler.compiler_source_sha, manifest.compiler_revision); assert.equal(compiler.vcs_revision, manifest.compiler_revision);
  assert.equal(compiler.version, manifest.compiler_version); assert.equal(compiler.source_status, 'CLEAN_VCS'); assert.equal(compiler.vcs_modified, 'false'); assert.equal(compiler.go_version, manifest.go_version);
  const fixture = read('fixtures.json'); assert.equal(fixture.schema, 'gooo/uuid-parse-browser-fixtures/v1'); assert.equal(fixture.compiler_revision, manifest.compiler_revision); assert.equal(fixture.go_version, manifest.go_version);
  assert.equal(fixture.cases.length, 36); assert.equal(fixture.rejections.length, 3); assert.equal(fixture.cases.length, manifest.fixture_cases); assert.equal(fixture.rejections.length, manifest.fixture_rejections);
  assert.equal(manifest.native_cases, fixture.cases.length); assert.equal(manifest.native_receipt_expectation_cases, 0);
  const workspace = read('workspace.json'); assert.deepEqual(workspace.entry, { package_path: 'go/github.com/google/uuid/text', activity: 'ParseCanonical' });
  assert.deepEqual(workspace.packages, Object.entries(sources).map(([name, packageName]) => ({ path: packageName, sources: [name] })));
  let offset = 0; const scopes = [];
  for (const [stem, size] of [['', 12], ['boundary-', 24]]) {
    const inputs = read(stem + 'inputs.json'); keys(inputs, ['schema', 'inputs']); assert.equal(inputs.schema, 'gooo/body-composition-inputs/v1'); assert.equal(inputs.inputs.length, size);
    const rows = fixture.cases.slice(offset, offset + size);
    for (const suffix of ['execution.json', 'replay.json']) {
      const receipt = read(stem + suffix); assert.equal(receipt.schema, 'gooo/workspace-body-execution-receipt/v1'); assert.equal(receipt.decision, 'OBSERVED');
      assert.equal(receipt.error, undefined); assert.equal(receipt.cases_digest, undefined); assert.equal(receipt.manifest_digest, 'sha256:' + hash('workspace.json')); assert.equal(receipt.inputs_digest, 'sha256:' + hash(stem + 'inputs.json'));
      const { program, composition, runtime } = receipt.result; keys(program.entry, ['package_path', 'activity', 'lowered_name']);
      assert.equal(program.entry.package_path, workspace.entry.package_path); assert.equal(program.entry.activity, workspace.entry.activity);
      assert.match(program.entry.lowered_name, /^GoooPackage[0-9]+ActivityParseCanonical$/);
      assert.equal(composition.source, bytes('generated.go').toString('utf8')); assert.equal(composition.generated_sha256, 'sha256:' + manifest.generated_sha256);
      assert.equal(program.lowered_gooo_source, composition.gooo_source); const bound = {};
      for (const pkg of program.workspace_image.packages) for (const source of pkg.sources) { assert.ok(!Object.hasOwn(bound, source.filename)); bound[source.filename] = source.source_digest; }
      assert.deepEqual(bound, sourceBindings); assert.equal(runtime.stage, 'COMPLETE'); assert.equal(runtime.failure, undefined); assert.equal(runtime.producer_source_sha, manifest.compiler_revision);
      assert.equal(runtime.go_version.split(' ')[2], manifest.go_version); assert.equal(runtime.generated_sha256, 'sha256:' + manifest.generated_sha256);
      for (const key of ['model_calls', 'finite_total', 'finite_passed']) assert.equal(runtime[key], 0);
      assert.equal(runtime.projection_replayed, true); assert.equal(runtime.runtime_replayed, true); assert.equal(runtime.traces.length, rows.length);
      for (let i = 0; i < rows.length; i++) {
        const row = rows[i], supplied = inputs.inputs[i], trace = runtime.traces[i]; keys(supplied, [inputKey]);
        assert.equal(supplied[inputKey], unitsText(row.input_utf16)); assert.deepEqual(protocol.bindInputs([supplied[inputKey]]), [row.input]);
        assert.equal(trace.case_index, i); assert.equal(trace.deliveries.length, 1); const delivery = trace.deliveries[0]; keys(delivery, ['activity_id', 'input', 'actual', 'actual_fields']);
        assert.equal(delivery.activity_id, entry); assert.equal(delivery.input, row.input);
        keys(delivery.actual, fields); assert.deepEqual(protocol.validateOutput(delivery.actual, [row.input]), row.expected);
        assert.deepEqual(delivery.actual_fields, fields.map((name, index) => ({ id: resultEntity + '/' + fieldIDs[index], name, value: delivery.actual[name] })));
      }
      if (suffix === 'replay.json') { assert.equal(receipt.replayed_from_sha256, 'sha256:' + hash(stem + 'execution.json')); assert.equal(receipt.result.replay.model_calls, 0); }
      else { assert.equal(receipt.replayed_from_sha256, undefined); assert.equal(receipt.result.replay, undefined); }
    }
    const checked = read(stem + 'checked-report.json'); assert.equal(checked.schema, 'gooo/uuid-parse-canonical-checked/v1'); assert.equal(checked.decision, 'PASS_FINITE_GO_ORACLE_AND_SAVED_REPLAY');
    assert.equal(checked.execution_sha256, 'sha256:' + hash(stem + 'execution.json')); assert.equal(checked.replay_sha256, 'sha256:' + hash(stem + 'replay.json')); assert.equal(checked.generated_sha256, 'sha256:' + manifest.generated_sha256);
    assert.equal(checked.manifest_digest, 'sha256:' + hash('workspace.json')); assert.equal(checked.inputs_digest, 'sha256:' + hash(stem + 'inputs.json')); assert.deepEqual(checked.source_bindings, checkedSourceBindings);
    assert.equal(checked.oracle_module, 'github.com/google/uuid'); assert.equal(checked.oracle_version, 'v1.6.0'); assert.equal(checked.oracle_revision, original.revision); assert.deepEqual(checked.oracle_source_sha256, originalHashes);
    for (const key of ['compiler_revision', 'compiler_version', 'go_version']) assert.equal(checked[key], manifest[key]);
    for (const key of ['native_observed_cases', 'independent_go_oracle_cases', 'saved_replay_observed_cases']) assert.equal(checked[key], rows.length);
    assert.equal(checked.native_receipt_expectation_cases, 0); assert.deepEqual(checked.results, rows.map(row => row.expected));
    assert.equal(checked.verification_scope.input_source_entry_and_compiler_bindings, 'CHECKED'); assert.equal(checked.verification_scope.independent_original_google_uuid_oracle, 'CHECKED');
    assert.equal(checked.verification_scope.prior_typed_result_sha256, 'NOT_CHECKED'); assert.equal(checked.verification_scope.typed_plan_digest_recomputation, 'NOT_CHECKED');
    assert.equal(checked.verification_scope.compiler_generation_or_native_execution_reperformed_by_checker, 'NOT_CHECKED'); scopes.push(checked.verification_scope); offset += size;
  }
  assert.equal(offset, fixture.cases.length);
  globalThis.crypto ??= webcrypto; require(path.join(site, 'wasm_exec.js')); const go = new Go();
  const { instance } = await WebAssembly.instantiate(bytes('program.wasm'), go.importObject); let failed;
  go.run(instance).then(() => { failed = new Error('UUID runtime stopped.'); }, error => { failed = error; });
  assert.equal(globalThis.goooUUIDParseReady, true); assert.equal(typeof globalThis.goooUUIDParse, 'function');
  const invoke = args => { assert.equal(failed, undefined); const raw = globalThis.goooUUIDParse(...args); assert.equal(typeof raw, 'string'); return raw; };
  const seen = new Set(); let observations = 0;
  for (const row of fixture.cases) {
    assert.ok(!seen.has(row.name)); seen.add(row.name); keys(row, ['name', 'input', 'input_utf16', 'expected']);
    const original = unitsText(row.input_utf16), reply = protocol.parseBridgeReply(invoke([original]), [original]); assert.deepEqual(reply.inputs, [row.input]); assert.deepEqual(reply.output, row.expected, row.name);
    const report = protocol.observation([protocol.read(original, 'text')], reply.output, manifest, ['smoke input']);
    assert.equal(report.kind, 'browser-observation'); assert.equal(report.schema, 'gooo/uuid-parse-observation/v1'); assert.equal(report.inputs[0].original, original);
    assert.equal(report.inputs[0].bound, row.input); assert.deepEqual(report.inputs[0].original_utf16_units, row.input_utf16); assert.deepEqual(JSON.parse(JSON.stringify(report)).output, row.expected); observations++;
  }
  const reject = args => { const reply = JSON.parse(invoke(args)); keys(reply, ['ok', 'error']); assert.equal(reply.ok, false); assert.ok(typeof reply.error === 'string' && reply.error.length); };
  for (const row of fixture.rejections) reject([unitsText(row.input_utf16)]);
  const wrongTypes = [0, NaN, Infinity, true, null, undefined, {}, [], new String('x'), 0n, Symbol('x'), () => 0]; for (const value of wrongTypes) reject([value]);
  for (const args of [[], ['x', 'y']]) reject(args);
  const zero = '00000000-0000-0000-0000-000000000000', canonical = '00112233-4455-6677-8899-aabbccddeeff';
  const anchors = [
    { input: '[' + canonical + ']', expected: { parsed: true, canonical_uuid: canonical, error: '', invalid_length: false } },
    { input: '!' + canonical + '?', expected: { parsed: true, canonical_uuid: canonical, error: '', invalid_length: false } },
    { input: '00112233445566778899aabbccddee0g', expected: { parsed: false, canonical_uuid: canonical, error: 'invalid UUID format', invalid_length: false } },
    { input: '00112233-4455-6677-8899-aabbccddee0g', expected: { parsed: false, canonical_uuid: '00112233-4455-6677-8899-aabbccddee00', error: 'invalid UUID format', invalid_length: false } },
    { input: zero, expected: { parsed: true, canonical_uuid: zero, error: '', invalid_length: false } },
    { input: '', expected: { parsed: false, canonical_uuid: zero, error: 'invalid UUID length: 0', invalid_length: true } },
    { input: 'a'.repeat(1024), expected: { parsed: false, canonical_uuid: zero, error: 'invalid UUID length: 1024', invalid_length: true } }
  ];
  for (const row of anchors) assert.deepEqual(protocol.parseCompiledReply(invoke([row.input]), [row.input]), row.expected);
  for (const name of ['uuid-parse.html', 'uuid-parse.css', 'uuid-parse.js', 'uuid-parse-protocol.js', 'uuid-parse-worker.js', 'literal-search-protocol.js', 'literal-search.css', 'style.css']) assert.ok(fs.statSync(path.join(site, name)).isFile(), name);
  const workerObservation = await actualWorkerSmoke(site, fixture, manifest, protocol); await Promise.resolve(); assert.equal(failed, undefined);
  return { schema: 'gooo/uuid-parse-browser-smoke/v1', status: 'PASS', native_observed_cases: fixture.cases.length, checked_native_batches: 2,
    native_receipt_expectation_cases: 0, wasm_cases: fixture.cases.length, actual_worker_cases: workerObservation.cases, actual_worker_request_rejections: workerObservation.request_rejections,
    browser_observation_exports: observations, domain_rejections: fixture.rejections.length, type_rejections: wrongTypes.length, arity_rejections: 2, manual_anchors: anchors.length,
    wasm_sha256: manifest.wasm_sha256, generated_sha256: manifest.generated_sha256, source_revision: manifest.source_revision, source_dirty: manifest.source_dirty,
    native_checker_verification_scope: scopes,
    scope: 'Actual Go Wasm callbacks and production Worker loader/protocol in Node worker_threads with static-file transport, independent original Go fixtures, checked native input-only receipts and saved replay within the recorded checker scope. DOM capture is covered separately by simulated unit tests. Inner typed Result hash and typed-plan digest recomputation remain NOT_CHECKED. This is not a full browser-DOM test, public adoption, signed attestation or full-library compatibility proof.' };
}
if (require.main === module) runSmoke(process.argv[2] || 'out/browser/site').then(result => { console.log(JSON.stringify(result)); process.exit(0); }, error => { console.error(error); process.exit(1); });
module.exports = { runSmoke };
