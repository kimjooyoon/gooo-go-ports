'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { createHash, webcrypto } = require('node:crypto');
const { performance } = require('node:perf_hooks');
const { Worker } = require('node:worker_threads');

const ROOT = path.resolve(__dirname, '..');
const MODULE = path.join(ROOT, 'examples/agentmesh-service-policy');
const FIELDS = { match: ['pattern', 'req_type', 'req_name'], impact: ['before', 'after', 'req_type', 'req_name'], refine: ['before', 'after'] };
const CORPORA = ['matcher-cases.json', 'evidence/observations.json', 'impact-cases.json', 'refinement-cases.json', 'checked-cases.json', 'domain-cases.json'];
const hash = bytes => createHash('sha256').update(bytes).digest('hex');
const read = filename => JSON.parse(fs.readFileSync(filename, 'utf8'));
const clone = value => JSON.parse(JSON.stringify(value));
const units = text => Array.from({ length: text.length }, (_, index) => text.charCodeAt(index));
const keys = (value, names) => {
  assert.ok(value !== null && typeof value === 'object' && !Array.isArray(value));
  assert.deepEqual(Object.keys(value).sort(), [...names].sort());
};

function invalid(mode, field) {
  const gate = { domain_status: 'INVALID_INPUT', invalid_field: field };
  if (mode === 'match') return { ...gate, matched: false };
  if (mode === 'impact') return { ...gate, before_match: false, after_match: false, change: 'INVALID_INPUT' };
  return { ...gate, status: 'INVALID_INPUT', rule_id: '', supported_subset: false, has_witness: false,
    witness_type: '', witness_name: '', before_match: false, after_match: false };
}

function matcherExpectations(cases, observations) {
  assert.equal(cases.length, 63); assert.equal(observations.length, 63);
  const expected = new Map();
  for (const row of observations) {
    keys(row, ['name', 'pattern', 'req_type', 'req_name', 'matched']);
    assert.ok(typeof row.name === 'string' && row.name.length > 0);
    assert.ok(!expected.has(row.name), 'duplicate oracle observation name');
    assert.equal(typeof row.matched, 'boolean');
    for (const field of FIELDS.match) assert.equal(typeof row[field], 'string');
    expected.set(row.name, row);
  }
  const names = new Set();
  for (const row of cases) {
    assert.ok(typeof row.name === 'string' && row.name.length > 0);
    assert.ok(!names.has(row.name), 'duplicate matcher case name'); names.add(row.name);
    assert.ok(expected.has(row.name), 'missing independent oracle observation: ' + row.name);
    const observed = expected.get(row.name);
    for (const field of FIELDS.match) assert.equal(row[field], observed[field], row.name + ': exact original oracle ' + field);
    if (Object.hasOwn(row, 'want')) {
      assert.equal(typeof row.want, 'boolean'); assert.equal(row.want, observed.matched, row.name + ': explicit anchor agrees with independent oracle');
    }
  }
  assert.deepEqual([...names].sort(), [...expected.keys()].sort(), 'exact matcher/oracle name roster');
  return expected;
}

// Expected policy decisions are immutable, independently authored corpus values.
// No matcher, subset algorithm, or new Go bridge is used as an expectation oracle.
function corpusRows() {
  const rows = [];
  const matcher = read(path.join(MODULE, 'matcher-cases.json'));
  const matcherOracle = matcherExpectations(matcher, read(path.join(MODULE, 'evidence/observations.json')));
  for (const [file, mode, count] of [['matcher-cases.json', 'match', 63], ['impact-cases.json', 'impact', 16], ['refinement-cases.json', 'refine', 101]]) {
    const cases = mode === 'match' ? matcher : read(path.join(MODULE, file)); assert.equal(cases.length, count);
    for (const row of cases) rows.push({ name: file + ':' + row.name, mode,
      facts: Object.fromEntries(FIELDS[mode].map(key => [key, row[key]])),
      want: { domain_status: 'VALID_INPUT', invalid_field: '', ...(mode === 'match' ? { matched: matcherOracle.get(row.name).matched } : row.want) } });
  }
  const checked = read(path.join(MODULE, 'checked-cases.json')); assert.equal(checked.length, 250);
  for (const row of checked) rows.push({ name: 'checked:' + row.name, mode: { matcher: 'match', impact: 'impact', refinement: 'refine' }[row.api], facts: row.facts, want: row.want });
  const domain = read(path.join(MODULE, 'domain-cases.json')); assert.equal(domain.length, 662);
  for (const row of domain) {
    const field = { ValidateServicePattern: 'pattern', ValidateServiceType: 'req_type', ValidateServiceName: 'req_name' }[row.activity];
    assert.ok(field); assert.equal(typeof row.want, 'boolean');
    // With another required request component empty, every VALID_INPUT match is
    // the documented empty-request denial. This observes the isolated grammar
    // predicate without implementing service matching or pattern refinement.
    const facts = { pattern: '*', req_type: '', req_name: '', [field]: row.input };
    rows.push({ name: 'domain:' + row.name, mode: 'match', facts,
      want: row.want ? { domain_status: 'VALID_INPUT', invalid_field: '', matched: false } : invalid('match', field) });
  }
  assert.equal(rows.length, 1092);
  assert.equal(new Set(rows.map(row => row.name)).size, rows.length);
  return rows;
}

function boundaryRows() {
  const rows = [];
  for (const mode of Object.keys(FIELDS)) {
    const base = mode === 'match' ? { pattern: '*', req_type: 'mcp', req_name: 'x' }
      : mode === 'impact' ? { before: '*', after: '*', req_type: 'mcp', req_name: 'x' } : { before: '*', after: '*' };
    for (const field of FIELDS[mode]) for (const [label, value] of [
      ['nul', '\u0000'], ['newline', '\n'], ['non-ascii', 'é'], ['astral', '😀'],
      ['noncharacter', '\uffff'], ['unicode-max-transport', '😀'.repeat(2048)], ['ascii-max-transport', 'a'.repeat(4096)],
    ]) rows.push({ name: mode + ':' + field + ':' + label, mode, facts: { ...base, [field]: value }, want: invalid(mode, field) });
  }
  assert.equal(rows.length, 63);
  return rows;
}

function expectedBindings() {
  const bytes = fs.readFileSync(path.join(MODULE, 'generation.json')), generation = JSON.parse(bytes);
  assert.equal(generation.schema, 'gooo/agentmesh-native-generation/v1');
  assert.equal(generation.origin, 'fresh_pinned_generation');
  assert.equal(Object.keys(generation.assets).length, 89);
  return { compiler_source_sha: read(path.join(ROOT, 'upstream.json')).compiler_revision,
    generation_sha256: 'sha256:' + hash(bytes), generation_origin: generation.origin, assets: generation.assets };
}

function checkProof(proof, row, bindings) {
  keys(proof, ['facts', 'decision', 'bindings']);
  assert.deepEqual(proof.facts, row.facts, row.name + ': complete original facts');
  assert.deepEqual(proof.decision, row.want, row.name + ': all exact typed decision fields');
  assert.deepEqual(proof.bindings, bindings, row.name + ': all 89 source bindings');
}

async function directBridge(site, manifest, bindings) {
  const logs = [];
  const context = vm.createContext({ console: { log: (...args) => logs.push(args.join(' ')), error: (...args) => logs.push(args.join(' ')) },
    WebAssembly, TextEncoder, TextDecoder, crypto: webcrypto, performance, setTimeout, clearTimeout });
  vm.runInContext(fs.readFileSync(path.join(site, 'service-policy/wasm_exec.js'), 'utf8'), context);
  const go = new context.Go();
  const { instance } = await WebAssembly.instantiate(fs.readFileSync(path.join(site, 'service-policy/program.wasm')), go.importObject);
  let failed;
  go.run(instance).then(() => { failed = new Error('Service-policy runtime terminated.'); }, error => { failed = error; });
  assert.equal(context.goooServicePolicyReady, true);
  assert.deepEqual(JSON.parse(context.goooServicePolicyBindings()), bindings);
  const invoke = args => { assert.equal(failed, undefined); return JSON.parse(context.goooServicePolicyEvaluate(...args)); };
  for (const mode of Object.keys(FIELDS)) {
    const row = corpusRows().find(value => value.mode === mode);
    const response = invoke([mode, FIELDS[mode].map(field => units(row.facts[field]))]);
    keys(response, ['ok', 'proof']); assert.equal(response.ok, true); checkProof(response.proof, row, bindings);
  }
  let refused = 0;
  const reject = args => { const result = invoke(args); keys(result, ['ok', 'error']); assert.equal(result.ok, false); assert.ok(typeof result.error === 'string' && result.error.length > 0); refused++; };
  for (const args of [[], ['match'], ['unknown', []], [null, []], ['match', []], ['match', [[], []]],
    ['match', [[], [], [], []]], ['match', null], ['match', {}], ['match', '*'], ['match', [[], [], []], 'extra']]) reject(args);
  const extraProperty = [65]; extraProperty.extra = 1;
  const badFields = [null, true, 1, '*', {}, [undefined], [null], [true], ['65'], [-1], [65536], [1.5], [NaN], [Infinity],
    [0xd800], [0xdc00], [0xd800, 65], [65, 0xdc00], [0xdc00, 0xd800], new Array(1), new Array(4097).fill(65),
    new Uint16Array([65]), extraProperty];
  for (let index = 0; index < 3; index++) for (const bad of badFields) {
    const input = [[42], [109, 99, 112], [120]]; input[index] = bad; reject(['match', input]);
  }
  await Promise.resolve(); assert.equal(failed, undefined);
  return { modes: 3, malformed_utf16_or_shape_rejections: refused, runtime_log_lines: logs.length };
}

function workerHarness(site, fault) {
  // A real thread runs the shipped loader, protocol and Wasm. Only browser
  // fetch/Blob/importScripts transport is mapped to this static output tree.
  const fetchPaths = ['./service-policy/manifest.json', ...Object.keys(read(path.join(site, 'service-policy/manifest.json')).files)
    .map(name => './service-policy/' + name)].sort();
  const fetchCounts = new Int32Array(new SharedArrayBuffer(Int32Array.BYTES_PER_ELEMENT * fetchPaths.length));
  const worker = new Worker(`
    const { parentPort, workerData } = require('node:worker_threads');
    const fs = require('node:fs'), path = require('node:path'), vm = require('node:vm');
    const assert = require('node:assert/strict');
    globalThis.crypto ??= require('node:crypto').webcrypto;
    const manifestPath = './service-policy/manifest.json';
    const realManifest = JSON.parse(fs.readFileSync(path.join(workerData.site, manifestPath), 'utf8'));
    const allowed = new Set([manifestPath, ...Object.keys(realManifest.files).map(name => './service-policy/' + name)]);
    const fetchCounts = new Int32Array(workerData.fetchCounts);
    assert.deepEqual([...allowed].sort(), workerData.fetchPaths);
    globalThis.fetch = async (name, options) => {
      assert.deepEqual(options, { cache: 'no-store', credentials: 'omit', redirect: 'error', referrerPolicy: 'no-referrer' });
      assert.ok(allowed.has(name), 'unexpected fetch: ' + name);
      Atomics.add(fetchCounts, workerData.fetchPaths.indexOf(name), 1);
      if (workerData.fault === 'missing-source' && name === './service-policy/source/domain.gooo') return { ok: false };
      let raw = fs.readFileSync(path.join(workerData.site, name));
      if (workerData.fault === 'manifest' && name === manifestPath) {
        const value = JSON.parse(raw); value.wasm_sha256 = '0'.repeat(64); raw = Buffer.from(JSON.stringify(value));
      }
      const target = { wasm: './service-policy/program.wasm', runtime: './service-policy/wasm_exec.js',
        source: './service-policy/source/domain.gooo' }[workerData.fault];
      if (name === target) { raw = Buffer.from(raw); raw[raw.length - 1] ^= 1; }
      return { ok: true, text: async () => raw.toString('utf8'),
        arrayBuffer: async () => raw.buffer.slice(raw.byteOffset, raw.byteOffset + raw.byteLength) };
    };
    const blobs = new Map(); let serial = 0;
    globalThis.Blob = class { constructor(parts, options) {
      assert.equal(options.type, 'text/javascript'); this.bytes = Buffer.concat(parts.map(value => Buffer.from(value)));
    } };
    URL.createObjectURL = blob => { const name = 'blob:service-policy-smoke-' + (++serial); blobs.set(name, blob.bytes); return name; };
    URL.revokeObjectURL = name => { assert.ok(blobs.delete(name)); };
    globalThis.importScripts = name => {
      assert.ok(blobs.has(name), 'runtime must execute already-verified Blob bytes');
      vm.runInThisContext(blobs.get(name).toString('utf8'), { filename: 'verified-service-policy-wasm_exec.js' });
      if (workerData.fault === 'native-bindings') {
        const original = Go.prototype.run;
        Go.prototype.run = function(instance) {
          const running = original.call(this, instance);
          const callback = globalThis.goooServicePolicyBindings;
          globalThis.goooServicePolicyBindings = () => {
            const value = JSON.parse(callback()); value.assets['matcher.gooo'] = 'sha256:' + '0'.repeat(64); return JSON.stringify(value);
          };
          return running;
        };
      }
    };
    if (workerData.fault === 'instantiate') {
      globalThis.WebAssembly = { ...WebAssembly, validate: WebAssembly.validate,
        instantiate: async () => { throw new Error('Injected WebAssembly initialization failure.'); } };
    }
    globalThis.postMessage = message => parentPort.postMessage(message);
    globalThis.addEventListener = (event, handler) => { assert.equal(event, 'message'); parentPort.on('message', data => handler({ data })); };
    require(path.join(workerData.site, 'service-policy-worker.js')).startWorker(globalThis);
  `, { eval: true, workerData: { site, fault, fetchPaths, fetchCounts: fetchCounts.buffer } });
  const queue = []; let waiting = null, failed = null;
  worker.on('message', value => { if (waiting) { const receive = waiting; waiting = null; receive(null, value); } else queue.push(value); });
  worker.on('error', error => { failed = error; if (waiting) { const receive = waiting; waiting = null; receive(error); } });
  return { worker, fetches: () => Object.fromEntries(fetchPaths.map((name, index) => [name, Atomics.load(fetchCounts, index)])),
    next: () => new Promise((resolve, reject) => {
    if (failed) return reject(failed);
    if (queue.length) return resolve(queue.shift());
    const timer = setTimeout(() => { waiting = null; reject(new Error('Service-policy Worker timed out: ' + (fault || 'normal'))); }, 30000);
    waiting = (error, value) => { clearTimeout(timer); error ? reject(error) : resolve(value); };
  }) };
}

function recheckRows(rows) {
  const named = name => {
    const matches = rows.filter(row => row.name === name); assert.equal(matches.length, 1);
    return clone(matches[0]);
  };
  const cases = [];
  for (const [mode, validName, invalidName, falseDecision, changedFields] of [
    ['match', 'matcher-cases.json:upstream-5', 'checked:contract-global-invalid-type',
      { domain_status: 'VALID_INPUT', invalid_field: '', matched: true }, ['matched']],
    ['impact', 'impact-cases.json:impact-2', 'checked:contract-both-global-invalid-type',
      { domain_status: 'VALID_INPUT', invalid_field: '', before_match: false, after_match: false, change: 'UNCHANGED_DENY' },
      ['after_match', 'change']],
    ['refine', 'refinement-cases.json:matrix-suffix-prefix', 'checked:contract-candidate-empty-invalid-before',
      { domain_status: 'VALID_INPUT', invalid_field: '', status: 'SUPPORTED_SUBSET', rule_id: 'OLD_GLOBAL',
        supported_subset: true, has_witness: false, witness_type: '', witness_name: '', before_match: false, after_match: false },
      ['status', 'rule_id', 'supported_subset', 'has_witness', 'witness_type', 'witness_name', 'after_match']],
  ]) {
    const valid = named(validName), badInput = named(invalidName);
    assert.equal(valid.mode, mode); assert.equal(badInput.mode, mode);
    cases.push({ ...valid, name: mode + ':valid-roundtrip', kind: 'roundtrip' },
      { ...badInput, name: mode + ':invalid-roundtrip', kind: 'roundtrip' },
      { ...valid, name: mode + ':coherent-false-claim', kind: 'false-claim', falseDecision, changedFields },
      { ...valid, name: mode + ':cross-program-claim', kind: 'cross-program' });
  }
  for (const [label, text] of [['crlf', '\r\n'], ['tab', '\t'], ['nul', '\u0000'], ['combining', 'e\u0301'],
    ['supplementary', '😀'], ['non-ascii', 'é'], ['ascii-129', 'a'.repeat(129)], ['max-utf16', '😀'.repeat(2048)]]) {
    cases.push({ name: 'raw-roundtrip:' + label, kind: 'raw-boundary', mode: 'match',
      facts: { pattern: '*', req_type: 'mcp', req_name: text }, want: invalid('match', 'req_name') });
  }
  cases.push({ name: 'max-four-field-pretty-export', kind: 'max-export', mode: 'impact',
    facts: Object.fromEntries(FIELDS.impact.map(name => [name, '\uffff'.repeat(4096)])), want: invalid('impact', 'before') });
  assert.equal(cases.length, 21); assert.equal(new Set(cases.map(row => row.name)).size, cases.length);
  return cases;
}

// Deliberately unverified, structurally coherent claims for another known-v1
// program. These hashes are not fetched, treated as evidence, or used to run code.
function claimDifferentProgram(saved) {
  const program = saved.program;
  program.compiler_revision = 'c'.repeat(40); program.bindings.compiler_source_sha = program.compiler_revision;
  program.generation_sha256 = 'sha256:' + 'd'.repeat(64); program.bindings.generation_sha256 = program.generation_sha256;
  program.files['source/generation.json'].sha256 = 'd'.repeat(64);
  program.go_version = 'go1.27.3'; program.source_revision = 'e'.repeat(40); program.source_dirty = true;
  for (const [file, prefix, character] of [['program.wasm', 'wasm', 'a'], ['wasm_exec.js', 'wasm_exec', 'b']]) {
    program[prefix + '_sha256'] = character.repeat(64); program[prefix + '_bytes']++;
    program.files[file] = { sha256: program[prefix + '_sha256'], bytes: program[prefix + '_bytes'] };
  }
  saved.proof.bindings = clone(program.bindings);
}

async function savedObservationRechecks(protocol, manifest, bindings, rows, evaluate, fetches) {
  const cases = recheckRows(rows), beforeFetches = fetches();
  let originalCalls = 0, freshCalls = 0, largestBytes = 0;
  for (const row of cases) {
    const original = await evaluate(row.mode, row.facts); originalCalls++;
    checkProof(original, row, bindings);
    const claimed = clone(protocol.observation(row.mode, row.facts, original, manifest));
    if (row.kind === 'false-claim') claimed.proof.decision = clone(row.falseDecision);
    if (row.kind === 'cross-program') {
      claimDifferentProgram(claimed);
      assert.throws(() => protocol.validateMetadata(claimed.program), row.name + ': live pins remain strict');
      assert.throws(() => protocol.validateProof(row.mode, claimed.proof, row.facts, manifest));
    }
    // Exercise actual saved JSON bytes, preserving BOMs for parser rejection and
    // refusing malformed UTF-8 instead of silently replacing or normalizing it.
    const bytes = Buffer.from(JSON.stringify(claimed, null, 2), 'utf8');
    assert.ok(bytes.length > 0 && bytes.length <= protocol.MAX_OBSERVATION_BYTES, row.name + ': bounded pretty export');
    largestBytes = Math.max(largestBytes, bytes.length);
    const raw = new TextDecoder('utf-8', { fatal: true, ignoreBOM: true }).decode(bytes);
    assert.deepEqual(Buffer.from(raw, 'utf8'), bytes, row.name + ': exact UTF-8 roundtrip');
    const saved = clone(protocol.parseObservation(raw));
    assert.deepEqual(saved, claimed, row.name + ': parser retains the complete unsigned claim');
    assert.equal(saved.mode, row.mode); assert.deepEqual(saved.proof.facts, row.facts);
    assert.deepEqual(saved.inputs, FIELDS[row.mode].map(name => ({ name, text: row.facts[name],
      utf16_units: units(row.facts[name]), escaped: '"' + units(row.facts[name])
        .map(value => '\\u' + value.toString(16).toUpperCase().padStart(4, '0')).join('') + '"' })),
    row.name + ': every raw text representation');
    // This is a second, newly identified request to the shipped real Worker.
    // Only the parsed mode/facts are sent; saved decisions and metadata cannot
    // select an executable or substitute for fresh generated Gooo execution.
    const fresh = await evaluate(saved.mode, saved.proof.facts); freshCalls++;
    checkProof(fresh, row, bindings);
    const comparison = clone(protocol.compareObservation(saved, saved.mode, saved.proof.facts, fresh, manifest));
    const differences = (row.changedFields || []).map(field => ({ field,
      saved: row.falseDecision[field], current: row.want[field] }));
    assert.deepEqual(comparison, { decision_equal: row.kind !== 'false-claim', decision_differences: differences,
      program_equal: row.kind !== 'cross-program', bindings_equal: row.kind !== 'cross-program',
      executable_equal: row.kind !== 'cross-program' }, row.name + ': exact independent comparison expectations');
    const currentExport = clone(protocol.observation(saved.mode, saved.proof.facts, fresh, manifest));
    assert.deepEqual(currentExport.proof, fresh); assert.deepEqual(currentExport.program, manifest);
    assert.deepEqual(currentExport.inputs, saved.inputs, row.name + ': current export keeps original input units');
    assert.deepEqual(fetches(), beforeFetches, row.name + ': importing claims does not fetch saved assets');
  }
  assert.equal(originalCalls, 21); assert.equal(freshCalls, 21);
  assert.equal(cases.filter(row => row.want.domain_status === 'INVALID_INPUT').length, 12);
  return { cases: cases.length, modes: 3, original_worker_evaluations: originalCalls, fresh_worker_evaluations: freshCalls,
    coherent_false_claims: 3, cross_program_claims: 3, invalid_domain_roundtrips: 12, raw_text_boundary_roundtrips: 8,
    max_four_field_export_roundtrips: 1, max_pretty_export_bytes: largestBytes,
    all_raw_facts_and_utf16_representations_preserved: true, all_typed_decision_fields_compared: true,
    all_89_current_bindings_compared: true, current_program_and_executable_bound: true,
    live_validators_reject_cross_program_claims: true,
    fetches_before_recheck: Object.values(beforeFetches).reduce((sum, count) => sum + count, 0),
    fetches_after_recheck: Object.values(fetches()).reduce((sum, count) => sum + count, 0), additional_fetches: 0,
    scope: 'Fresh generated Gooo evaluations compare unsigned saved claims; not native receipt replay, authenticity, authorization or policy application. Fetch observations cover only this Node Worker harness, not browser UI networking.' };
}

async function realWorker(site, manifest, bindings, protocol, rows) {
  const harness = workerHarness(site); let id = 0, observed = 0;
  try {
    assert.deepEqual(clone(protocol.validateReady(await harness.next())), manifest);
    const expectedFetches = Object.fromEntries(['./service-policy/manifest.json',
      ...Object.keys(manifest.files).map(name => './service-policy/' + name)].map(name => [name, 1]));
    assert.deepEqual(harness.fetches(), expectedFetches, 'exact current manifest and 34 files fetched once before readiness');
    const fetchesAtReady = Object.values(harness.fetches()).reduce((sum, count) => sum + count, 0);
    for (const row of rows) {
      const request = { type: 'evaluate', id: ++id, mode: row.mode, facts: row.facts };
      harness.worker.postMessage(request);
      const result = await harness.next();
      checkProof(result.proof, row, bindings);
      const accepted = clone(protocol.validateResult(result, { ...request, program: manifest }));
      checkProof(accepted, row, bindings);
      const report = clone(protocol.observation(row.mode, row.facts, accepted, manifest));
      assert.equal(report.kind, 'browser-observation');
      assert.equal(report.schema, 'gooo/service-policy-browser-observation/v1');
      assert.deepEqual(report.proof, accepted); assert.deepEqual(report.program, manifest);
      assert.deepEqual(report.inputs.map(input => [input.name, input.text, input.utf16_units]),
        FIELDS[row.mode].map(name => [name, row.facts[name], units(row.facts[name])]));
      observed++;
    }
    const baseline = { type: 'evaluate', mode: 'match', facts: { pattern: '*', req_type: 'mcp', req_name: 'x' } };
    const malformed = [ { ...baseline, id }, { ...baseline, id: ++id, mode: 'unknown' },
      { ...baseline, id: ++id, facts: { ...baseline.facts, req_type: '\ud800' } },
      { ...baseline, id: ++id, facts: { ...baseline.facts, req_name: 'a'.repeat(4097) } },
      { ...baseline, id: ++id, facts: { ...baseline.facts, req_name: 0 } },
      { ...baseline, id: ++id, facts: { ...baseline.facts, extra: '' } },
      { ...baseline, id: ++id, extra: true }, { ...baseline, id: ++id, type: 'apply' } ];
    for (const request of malformed) {
      harness.worker.postMessage(request); const response = await harness.next();
      assert.equal(response.type, 'error'); assert.equal(response.id, request.id);
      protocol.validateError(response, request.id);
    }
    // Refusal leaves the Worker usable for the next exact request.
    const row = rows[0], request = { type: 'evaluate', id: ++id, mode: row.mode, facts: row.facts };
    harness.worker.postMessage(request);
    checkProof(clone(protocol.validateResult(await harness.next(), { ...request, program: manifest })), row, bindings);
    const rechecks = await savedObservationRechecks(protocol, manifest, bindings, rows, async (mode, facts) => {
      const request = { type: 'evaluate', id: ++id, mode, facts };
      harness.worker.postMessage(request);
      return clone(protocol.validateResult(await harness.next(), { ...request, program: manifest }));
    }, harness.fetches);
    assert.deepEqual(harness.fetches(), expectedFetches, 'all evaluation paths preserve the exact current fetch roster');
    return { cases: rows.length, observation_exports: observed, request_rejections: malformed.length, recovery_after_refusal: true,
      saved_observation_recheck: { ...rechecks, fetches_at_ready: fetchesAtReady, exact_current_fetch_roster: true } };
  } finally { await harness.worker.terminate(); }
}

async function loaderFaults(site, protocol) {
  const modes = ['manifest', 'wasm', 'runtime', 'source', 'missing-source', 'instantiate', 'native-bindings'];
  for (const fault of modes) {
    const harness = workerHarness(site, fault);
    try {
      const message = await harness.next();
      assert.equal(message.type, 'error', fault + ': must refuse before readiness');
      assert.equal(Object.hasOwn(message, 'id'), false, fault + ': initialization error is not a request result');
      protocol.validateError(message);
    } finally { await harness.worker.terminate(); }
  }
  return modes;
}

async function runSmoke(directory) {
  const site = path.resolve(directory), dir = path.join(site, 'service-policy');
  const protocol = require(path.join(site, 'service-policy-protocol.js'));
  const manifest = read(path.join(dir, 'manifest.json')), bindings = expectedBindings();
  protocol.validateMetadata(manifest);
  assert.deepEqual(manifest.bindings, bindings);
  assert.equal(manifest.generation_sha256, bindings.generation_sha256);
  assert.equal(manifest.compiler_revision, bindings.compiler_source_sha);
  assert.equal(manifest.go_version, 'go1.27.2');
  assert.equal(Object.keys(manifest.files).length, 34);
  keys(manifest.files, protocol.FILE_NAMES);
  for (const [name, expected] of Object.entries(manifest.files)) {
    const bytes = fs.readFileSync(path.join(dir, name));
    assert.equal(bytes.length, expected.bytes, name + ': byte length');
    assert.equal(hash(bytes), expected.sha256, name + ': byte hash');
    if (name.startsWith('source/')) {
      const source = name.slice(7);
      assert.deepEqual(bytes, fs.readFileSync(path.join(MODULE, source)), name + ': exact checked-out source');
      if (Object.hasOwn(bindings.assets, source)) assert.equal('sha256:' + hash(bytes), bindings.assets[source]);
    }
  }
  for (const filename of ['service-policy-protocol.js', 'service-policy-worker.js']) {
    assert.deepEqual(fs.readFileSync(path.join(site, filename)), fs.readFileSync(path.join(ROOT, 'site', filename)));
  }
  const corpusHashes = {};
  for (const filename of CORPORA) {
    corpusHashes[filename] = hash(fs.readFileSync(path.join(MODULE, filename)));
    assert.equal('sha256:' + corpusHashes[filename], bindings.assets[filename]);
  }
  const direct = await directBridge(site, manifest, bindings);
  const corpus = corpusRows(), boundaries = boundaryRows();
  const worker = await realWorker(site, manifest, bindings, protocol, [...corpus, ...boundaries]);
  const failures = await loaderFaults(site, protocol);
  return { schema: 'gooo/service-policy-browser-smoke/v1', status: 'PASS', independent_existing_cases: 430,
    isolated_domain_cases: 662, utf16_policy_boundary_cases: boundaries.length, actual_worker_cases: worker.cases,
    all_facts_decision_fields_and_bindings_compared: true, bound_assets: Object.keys(bindings.assets).length,
    published_files_checked: Object.keys(manifest.files).length, direct_bridge: direct,
    actual_worker_request_rejections: worker.request_rejections, recovery_after_refusal: worker.recovery_after_refusal,
    browser_observation_exports: worker.observation_exports, real_loader_fault_controls: failures,
    saved_observation_recheck: worker.saved_observation_recheck,
    corpus_sha256: corpusHashes, generation_sha256: manifest.generation_sha256, wasm_sha256: manifest.wasm_sha256,
    scope: 'Actual generated checked Gooo in Wasm and shipped Worker code in Node worker_threads with static-file transport. Expectations come from existing independent corpora and explicit UTF16 boundaries; no new semantic oracle or policy reimplementation. Loader fault controls are injected failures, not real corrupt published assets. No full browser DOM, native receipt, saved replay, authorization, policy application, signature, or production safety claim.' };
}

if (require.main === module) runSmoke(process.argv[2] || 'out/browser/site').then(result => {
  console.log(JSON.stringify(result)); process.exit(0);
}, error => { console.error(error); process.exit(1); });
module.exports = { corpusRows, boundaryRows, expectedBindings, checkProof, matcherExpectations, runSmoke };
