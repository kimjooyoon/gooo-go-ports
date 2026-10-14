'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { createHash } = require('node:crypto');
const protocol = require('../site/service-policy-protocol.js');
const { corpusRows, boundaryRows, expectedBindings, matcherExpectations } = require('./service-policy-smoke.cjs');
const MODULE = path.resolve(__dirname, '../examples/agentmesh-service-policy');
const clone = value => JSON.parse(JSON.stringify(value));
const hash = value => createHash('sha256').update(value).digest('hex');
const FIELDS = { match: ['pattern', 'req_type', 'req_name'], impact: ['before', 'after', 'req_type', 'req_name'], refine: ['before', 'after'] };
const SOURCE_FILES = ['matcher.gooo', 'impact.gooo', 'refinement.gooo', 'domain.gooo', 'checked_matcher.gooo', 'checked_impact.gooo', 'checked_refinement.gooo',
  'model.go', 'domain.go', 'input.go', 'bridge.go', 'bindings.go', 'adapter_generated.go',
  'internal/checked-matcher/generated.go', 'internal/checked-impact/generated.go', 'internal/checked-refinement/generated.go',
  'matcher-workspace.json', 'impact-workspace.json', 'refinement-workspace.json', 'generation.json', 'compiler.json', 'source-pin.json', 'go.mod',
  'upstream/LICENSE', 'upstream/api/tar.go.txt', 'upstream/api/network.go.txt', 'upstream/api/tar_test.go.txt',
  'LICENSE-APACHE-2.0', 'NOTICE.md', 'cmd/browser/bridge.go', 'cmd/browser/main_js.go', 'cmd/browser/main_native.go'];

// Synthetic asset sizes/digests test schema validation only, not execution.
// The real Worker smoke separately verifies every shipped file and native binding.
function metadata() {
  const bindings = expectedBindings();
  const files = Object.fromEntries(SOURCE_FILES.map(name => {
    const filename = path.join(MODULE, name), raw = fs.readFileSync(filename);
    return ['source/' + name, { sha256: hash(raw), bytes: raw.length }];
  }));
  files['program.wasm'] = { sha256: 'a'.repeat(64), bytes: 100 };
  files['wasm_exec.js'] = { sha256: 'b'.repeat(64), bytes: 100 };
  return { schema: 'gooo/browser-service-policy/v1', compiler_revision: bindings.compiler_source_sha, go_version: 'go1.27.2',
    source_revision: 'c'.repeat(40), source_dirty: false, generation_sha256: bindings.generation_sha256,
    bindings, wasm_sha256: files['program.wasm'].sha256, wasm_bytes: 100,
    wasm_exec_sha256: files['wasm_exec.js'].sha256, wasm_exec_bytes: 100, files };
}
const fact = mode => Object.fromEntries(FIELDS[mode].map(name => [name, name === 'req_type' ? 'mcp' : name === 'req_name' ? 'x' : '*']));
const proof = (row, program) => ({ facts: clone(row.facts), decision: clone(row.want), bindings: clone(program.bindings) });

test('matcher expectations use all 63 fixed independent observations with exact roster and inputs', () => {
  const cases = JSON.parse(fs.readFileSync(path.join(MODULE, 'matcher-cases.json'), 'utf8'));
  const observations = JSON.parse(fs.readFileSync(path.join(MODULE, 'evidence/observations.json'), 'utf8'));
  const expected = matcherExpectations(cases, observations); assert.equal(expected.size, 63);
  for (const row of cases.filter(value => !Object.hasOwn(value, 'want'))) assert.equal(typeof expected.get(row.name).matched, 'boolean');
  for (const make of [
    rows => rows.slice(1),
    rows => [...rows, clone(rows[0])],
    rows => { rows[1] = clone(rows[0]); return rows; },
    rows => { rows[0].name = 'unrelated'; return rows; },
    rows => { rows[0].matched = 1; return rows; },
    rows => { rows[0].extra = true; return rows; },
    ...FIELDS.match.map(field => rows => { rows[0][field] += 'x'; return rows; }),
  ]) assert.throws(() => matcherExpectations(cases, make(clone(observations))));
  const wrongAnchor = clone(cases); wrongAnchor[0].want = !wrongAnchor[0].want;
  assert.throws(() => matcherExpectations(wrongAnchor, observations));
  const untypedAnchor = clone(cases); untypedAnchor[0].want = 1;
  assert.throws(() => matcherExpectations(untypedAnchor, observations));
  const duplicateCase = clone(cases); duplicateCase[1] = clone(duplicateCase[0]);
  assert.throws(() => matcherExpectations(duplicateCase, observations));
});

test('facts preserve all UTF16 code units without policy grammar filtering', () => {
  assert.equal(protocol.MAX_UNITS, 4096);
  for (const mode of Object.keys(FIELDS)) for (const name of FIELDS[mode]) {
    for (const text of ['', '\0', '\n', 'é', '😀', '\uffff', 'a'.repeat(128), 'a'.repeat(129), 'a'.repeat(4096), '😀'.repeat(2048)]) {
      const facts = { ...fact(mode), [name]: text };
      assert.deepEqual(clone(protocol.validateFacts(mode, facts)), facts);
      const expected = FIELDS[mode].map(field => Array.from({ length: facts[field].length }, (_, index) => facts[field].charCodeAt(index)));
      assert.deepEqual(protocol.packFacts(mode, facts), expected);
    }
    for (const text of ['\ud800', '\udc00', '\ud800x', 'x\udc00', '\udc00\ud800', 'a'.repeat(4097), '😀'.repeat(2048) + 'x']) {
      assert.throws(() => protocol.validateFacts(mode, { ...fact(mode), [name]: text }), mode + ':' + name);
    }
    for (const value of [null, undefined, 0, false, {}, [], new String('x')]) {
      assert.throws(() => protocol.validateFacts(mode, { ...fact(mode), [name]: value }));
    }
    const missing = fact(mode); delete missing[name]; assert.throws(() => protocol.validateFacts(mode, missing));
  }
  for (const mode of Object.keys(FIELDS)) {
    for (const malformed of [null, [], {}, { ...fact(mode), extra: '' }]) assert.throws(() => protocol.validateFacts(mode, malformed));
  }
  for (const mode of ['unknown', '__proto__', 'constructor', '', null, false]) assert.throws(() => protocol.validateFacts(mode, fact('match')));
});

test('JSON envelopes reject duplicate literal and escaped keys and oversized or malformed input', () => {
  assert.deepEqual(protocol.parseExactJSON('{"a":{"b":1},"c":[{"b":2}]}'), { a: { b: 1 }, c: [{ b: 2 }] });
  for (const raw of ['{"ok":true,"ok":false}', '{"a":1,"\\u0061":2}', '{"a":{"x":1,"x":2}}',
    '[{"x":1,"x":2}]', '{"a":1,}', '{', 'undefined', '{"a":NaN}', '"unterminated']) assert.throws(() => protocol.parseExactJSON(raw));
  assert.throws(() => protocol.parseExactJSON('{"a":1}', 3));
  assert.throws(() => protocol.parseExactJSON(null));
});

test('all independent corpus decisions pass closed structural validation without computing matching', () => {
  for (const row of [...corpusRows(), ...boundaryRows()]) assert.deepEqual(clone(protocol.validateDecision(row.mode, row.want)), row.want, row.name);
});

test('every decision field is mandatory, exact typed and rejects extra fields', () => {
  const examples = [...corpusRows().filter(row => row.name.startsWith('checked:')), ...corpusRows().filter(row => !row.name.startsWith('domain:')).slice(0, 180)];
  const seen = new Set();
  for (const row of examples) {
    const signature = row.mode + ':' + JSON.stringify(row.want); if (seen.has(signature)) continue; seen.add(signature);
    assert.throws(() => protocol.validateDecision(row.mode, { ...row.want, extra: false }));
    for (const [field, value] of Object.entries(row.want)) {
      const missing = { ...row.want }; delete missing[field]; assert.throws(() => protocol.validateDecision(row.mode, missing), row.name + ':' + field);
      for (const wrong of typeof value === 'boolean' ? [0, 1, 'false', null, []] : [false, 0, null, [], {}]) {
        assert.throws(() => protocol.validateDecision(row.mode, { ...row.want, [field]: wrong }), row.name + ':' + field);
      }
    }
  }
});

test('domain refusal and semantic state sentinels cannot contradict each other', () => {
  const rows = corpusRows();
  for (const mode of Object.keys(FIELDS)) {
    const refused = rows.find(row => row.mode === mode && row.want.domain_status === 'INVALID_INPUT');
    for (const patch of [{ invalid_field: '' }, { invalid_field: 'unknown' }, { domain_status: 'VALID_INPUT' }]) {
      assert.throws(() => protocol.validateDecision(mode, { ...refused.want, ...patch }));
    }
    const valid = rows.find(row => row.mode === mode && row.want.domain_status === 'VALID_INPUT');
    assert.throws(() => protocol.validateDecision(mode, { ...valid.want, invalid_field: FIELDS[mode][0] }));
    assert.throws(() => protocol.validateDecision(mode, { ...valid.want, domain_status: 'UNKNOWN' }));
  }
  const badMatch = rows.find(row => row.mode === 'match' && row.want.domain_status === 'INVALID_INPUT');
  assert.throws(() => protocol.validateDecision('match', { ...badMatch.want, matched: true }));
  for (const row of rows.filter(row => row.mode === 'impact')) {
    for (const field of ['before_match', 'after_match']) assert.throws(() => protocol.validateDecision('impact', { ...row.want, [field]: !row.want[field] }));
    assert.throws(() => protocol.validateDecision('impact', { ...row.want, change: 'UNKNOWN' }));
  }
  const subset = rows.find(row => row.mode === 'refine' && row.want.status === 'SUPPORTED_SUBSET').want;
  const witness = rows.find(row => row.mode === 'refine' && row.want.status === 'WIDENING_WITNESS').want;
  for (const patch of [{ supported_subset: false }, { has_witness: true }, { witness_type: 'a' }, { witness_name: 'x' },
    { before_match: true }, { after_match: true }, { rule_id: '' }, { rule_id: 'CHECKED_CANDIDATE_REQUEST' }]) assert.throws(() => protocol.validateDecision('refine', { ...subset, ...patch }));
  for (const patch of [{ supported_subset: true }, { has_witness: false }, { witness_type: '' }, { witness_name: '' },
    { before_match: true }, { after_match: false }, { rule_id: 'OLD_GLOBAL' }]) assert.throws(() => protocol.validateDecision('refine', { ...witness, ...patch }));
  for (const field of ['witness_type', 'witness_name']) for (const value of ['é', '\0', 'a'.repeat(129)]) {
    assert.throws(() => protocol.validateDecision('refine', { ...witness, [field]: value }));
  }
  // UNPROVEN is a defensive protocol contract, not a claimed reachable corpus case.
  const unproven = { ...subset, status: 'UNPROVEN', supported_subset: false, rule_id: '' };
  assert.deepEqual(clone(protocol.validateDecision('refine', unproven)), unproven);
  for (const patch of [{ rule_id: 'OLD_GLOBAL' }, { has_witness: true }, { after_match: true }, { status: 'APPLIED' }]) {
    assert.throws(() => protocol.validateDecision('refine', { ...unproven, ...patch }));
  }
});

test('published metadata binds the exact file roster, source identities and bounded byte sizes', () => {
  const program = metadata();
  assert.equal(SOURCE_FILES.length, 32);
  assert.equal(protocol.FILE_NAMES.length, 34);
  assert.equal(protocol.ASSET_NAMES.length, 89);
  assert.deepEqual(clone(protocol.validateMetadata(program)), program);
  assert.deepEqual(clone(protocol.validateReady({ type: 'ready', program })), program);
  for (const key of Object.keys(program)) {
    const changed = clone(program); delete changed[key]; assert.throws(() => protocol.validateMetadata(changed), key);
  }
  for (const patch of [{ schema: 'unknown' }, { compiler_revision: '0'.repeat(40) }, { source_revision: 'main' },
    { source_dirty: 0 }, { go_version: 'go1.27.1' }, { generation_sha256: 'sha256:' + '0'.repeat(64) },
    { wasm_sha256: '0'.repeat(64) }, { wasm_bytes: 0 }, { wasm_exec_bytes: 101 }, { extra: true }]) {
    assert.throws(() => protocol.validateMetadata({ ...program, ...patch }));
  }
  for (const filename of protocol.FILE_NAMES) {
    const missing = clone(program); delete missing.files[filename]; assert.throws(() => protocol.validateMetadata(missing), filename);
    for (const patch of [{ bytes: 0 }, { bytes: 1.5 }, { bytes: '100' }, { bytes: 33554433 }, { sha256: 'x'.repeat(64) }, { extra: 1 }]) {
      const changed = clone(program); Object.assign(changed.files[filename], patch); assert.throws(() => protocol.validateMetadata(changed), filename);
    }
  }
  const extra = clone(program); extra.files['source/../outside.go'] = { sha256: 'a'.repeat(64), bytes: 1 };
  assert.throws(() => protocol.validateMetadata(extra));
  for (const name of protocol.ASSET_NAMES) {
    const missing = clone(program); delete missing.bindings.assets[name]; assert.throws(() => protocol.validateMetadata(missing), name);
  }
  const duplicate = JSON.stringify(program).replace('"schema":', '"schema":"wrong","schema":');
  assert.throws(() => protocol.parseMetadata(duplicate));
  assert.throws(() => protocol.validateReady({ type: 'result', program }));
});

test('proofs bind every original fact, decision and all native asset hashes', () => {
  const program = metadata();
  for (const mode of Object.keys(FIELDS)) {
    const row = corpusRows().find(item => item.mode === mode), correct = proof(row, program);
    assert.deepEqual(clone(protocol.validateProof(mode, correct, row.facts, program)), correct);
    assert.deepEqual(clone(protocol.parseCompiledReply(JSON.stringify({ ok: true, proof: correct }), mode, row.facts, program)), correct);
    for (const field of FIELDS[mode]) {
      const changed = clone(correct); changed.facts[field] += 'x'; assert.throws(() => protocol.validateProof(mode, changed, row.facts, program));
    }
    for (const field of ['facts', 'decision', 'bindings']) {
      const missing = clone(correct); delete missing[field]; assert.throws(() => protocol.validateProof(mode, missing, row.facts, program));
    }
    for (const name of protocol.ASSET_NAMES) {
      const changed = clone(correct); changed.bindings.assets[name] = 'sha256:' + '0'.repeat(64);
      assert.throws(() => protocol.validateProof(mode, changed, row.facts, program), name);
    }
    for (const raw of ['null', '{}', JSON.stringify({ ok: 1, proof: correct }), JSON.stringify({ ok: true, proof: correct, extra: 1 }),
      JSON.stringify({ ok: false, error: 'transport refused' }), '{"ok":false,"error":""}', '{"ok":false,"error":0}',
      '{"ok":false,"error":"' + 'x'.repeat(241) + '"}', JSON.stringify({ ok: true, proof: correct }).replace('"ok":true', '"ok":false,"ok":true')]) {
      assert.throws(() => protocol.parseCompiledReply(raw, mode, row.facts, program));
    }
  }
  assert.deepEqual(clone(protocol.parseBindings(JSON.stringify(program.bindings), program)), program.bindings);
  const changed = clone(program.bindings); changed.assets['matcher.gooo'] = 'sha256:' + '0'.repeat(64);
  assert.throws(() => protocol.parseBindings(JSON.stringify(changed), program));
});

test('request and response identities reject stale modes, IDs, facts and program metadata', () => {
  const program = metadata(), row = corpusRows()[0];
  const request = { type: 'evaluate', id: 17, mode: row.mode, facts: row.facts };
  assert.deepEqual(clone(protocol.validateRequest(request)), { id: 17, mode: row.mode, facts: row.facts });
  const result = { type: 'result', id: request.id, mode: row.mode, proof: proof(row, program), program };
  const expected = { ...request, program };
  assert.deepEqual(clone(protocol.validateResult(result, expected)), result.proof);
  for (const id of [0, -1, 1.5, Number.MAX_SAFE_INTEGER + 1, '17', null, false]) {
    assert.throws(() => protocol.validateRequest({ ...request, id }));
    assert.throws(() => protocol.validateResult({ ...result, id }, expected));
  }
  for (const patch of [{ id: 16 }, { id: 18 }, { mode: 'impact' }, { type: 'ready' }, { extra: true }]) {
    assert.throws(() => protocol.validateResult({ ...result, ...patch }, expected));
  }
  const altered = clone(program); altered.source_revision = 'd'.repeat(40);
  assert.throws(() => protocol.validateResult({ ...result, program: altered }, expected));
  for (const field of Object.keys(result)) {
    const missing = { ...result }; delete missing[field]; assert.throws(() => protocol.validateResult(missing, expected));
  }
  for (const malformed of [{ ...request, extra: 1 }, { ...request, type: 'apply' }, { ...request, mode: '__proto__' }]) assert.throws(() => protocol.validateRequest(malformed));
  assert.equal(protocol.validateCancel({ type: 'cancel', id: 17 }), 17);
  assert.throws(() => protocol.validateCancel({ type: 'cancel', id: 17, extra: true }));
  assert.equal(protocol.validateError({ type: 'error', id: 17, message: 'refused' }, 17), 'refused');
  assert.equal(protocol.validateError({ type: 'error', message: 'initialization failed' }), 'initialization failed');
  for (const message of [{ type: 'error', id: 16, message: 'refused' }, { type: 'error', message: 'missing request ID' },
    { type: 'error', id: 17, message: '' }, { type: 'error', id: 17, message: false }, { type: 'error', id: 17, message: 'refused', extra: true }]) {
    assert.throws(() => protocol.validateError(message, 17));
  }
});

test('unsigned observation exports retain literal and escaped input, never claim an applied grant', () => {
  const program = metadata(), row = boundaryRows().find(item => item.name === 'match:req_name:astral');
  const result = proof(row, program), observation = clone(protocol.observation(row.mode, row.facts, result, program));
  assert.equal(observation.kind, 'browser-observation');
  assert.equal(observation.schema, 'gooo/service-policy-browser-observation/v1');
  assert.deepEqual(observation.proof, result); assert.deepEqual(observation.program, program);
  const name = observation.inputs.find(input => input.name === 'req_name');
  assert.equal(name.text, '😀'); assert.deepEqual(name.utf16_units, [0xd83d, 0xde00]);
  assert.equal(name.escaped, '"\\uD83D\\uDE00"');
  assert.match(observation.scope, /Unsigned/); assert.match(observation.scope, /Not a native execution receipt/);
  assert.match(observation.scope, /policy write/);
});
