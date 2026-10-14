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

function savedObservation(mode = 'match', decision) {
  const program = metadata(), row = corpusRows().find(item => item.mode === mode && item.want.domain_status === 'VALID_INPUT');
  const result = proof(row, program);
  if (decision) result.decision = decision;
  return clone(protocol.observation(mode, row.facts, result, program));
}
function withSavedProgram(saved, change) {
  const result = clone(saved); change(result.program);
  if (result.program.bindings !== undefined) result.proof.bindings = clone(result.program.bindings);
  return result;
}

test('saved observations round-trip every mode and exact input representation as immutable claims', () => {
  for (const mode of Object.keys(FIELDS)) {
    const samples = corpusRows().filter(row => row.mode === mode && row.want.domain_status === 'INVALID_INPUT');
    const saved = savedObservation(mode);
    assert.deepEqual(clone(protocol.parseObservation(JSON.stringify(saved))), saved);
    for (const row of samples.slice(0, 1)) {
      const program = metadata(), exported = protocol.observation(mode, row.facts, proof(row, program), program);
      assert.deepEqual(clone(protocol.parseObservation(JSON.stringify(exported))), clone(exported));
    }
    const parsed = protocol.validateObservation(saved);
    for (const value of [parsed, parsed.inputs, parsed.inputs[0], parsed.inputs[0].utf16_units,
      parsed.proof, parsed.proof.facts, parsed.proof.decision, parsed.program, parsed.program.bindings.assets, parsed.program.files]) assert.equal(Object.isFrozen(value), true);
    const original = parsed.inputs[0].text; saved.inputs[0].text = 'mutated'; assert.equal(parsed.inputs[0].text, original);
    assert.throws(() => { parsed.proof.facts[FIELDS[mode][0]] = 'mutated'; }, TypeError);
  }
  const program = metadata();
  for (const text of ['', '\r\n\t\0', '\uFEFF', '<tag>&', 'e\u0301', 'é', '😀', 'x'.repeat(129), '\uffff'.repeat(4096), '😀'.repeat(2048)]) {
    const facts = { ...fact('match'), req_name: text };
    const result = { facts, decision: { domain_status: 'INVALID_INPUT', invalid_field: 'req_name', matched: false }, bindings: program.bindings };
    const parsed = protocol.parseObservation(JSON.stringify(protocol.observation('match', facts, result, program)));
    assert.equal(parsed.inputs[2].text, text); assert.equal(parsed.proof.facts.req_name, text);
    assert.deepEqual(parsed.inputs[2].utf16_units, Array.from({ length: text.length }, (_, index) => text.charCodeAt(index)));
  }
});

test('saved observation parser refuses duplicate keys, excessive depth and noncanonical numeric tokens before rounding', () => {
  const saved = savedObservation(), raw = JSON.stringify(saved);
  for (const token of ['100.0', '100.00000000000000001', '1e2', '1E+2', '-0', '-100', '9007199254740992', '9007199254740993', '1e999']) {
    assert.throws(() => protocol.parseObservation(raw.replace('"wasm_bytes":100', '"wasm_bytes":' + token)), /canonical nonnegative safe integers/, token);
  }
  for (const changed of [
    raw.replace('"schema":', '"schema":"ignored","schema":'),
    raw.replace('"kind":', '"k\\u0069nd":"ignored","kind":'),
    raw.replace('"matched":', '"matched":false,"matched":'),
    raw.replace('"sha256":', '"sha256":"ignored","sha256":'),
    raw.replace('"text":', '"text":"ignored","text":'),
    raw + '{}', '[' + raw + ']', '\uFEFF' + raw, raw.slice(0, -1),
    '{"a":NaN}', '{"a":Infinity}', '{"a":01}', '{"a":+1}', '{"a":.1}', '{"a":1.}',
  ]) assert.throws(() => protocol.parseObservation(changed));
  assert.throws(() => protocol.parseObservation('['.repeat(13) + '0' + ']'.repeat(13)), /nested too deeply/);
  assert.throws(() => protocol.parseObservation('['.repeat(12) + '0' + ']'.repeat(12)), error => !/nested too deeply/.test(error.message));
  for (const value of ['', null, false, {}, new String(raw)]) assert.throws(() => protocol.parseObservation(value));
});

test('saved observation roots, inputs and proof require exact fields, order and matching representations', () => {
  const saved = savedObservation();
  const refuse = change => { const altered = clone(saved); change(altered); assert.throws(() => protocol.validateObservation(altered)); };
  for (const key of Object.keys(saved)) refuse(value => { delete value[key]; });
  for (const key of ['schema', 'kind', 'input_binding', 'scope']) refuse(value => { value[key] += 'changed'; });
  for (const mode of ['__proto__', 'unknown', null, false]) refuse(value => { value.mode = mode; });
  refuse(value => { value.extra = true; });
  for (const inputs of [null, {}, [], [saved.inputs[0]], [...saved.inputs, saved.inputs[0]], [...saved.inputs].reverse(),
    [saved.inputs[0], saved.inputs[0], saved.inputs[2]]]) refuse(value => { value.inputs = inputs; });
  for (const field of ['name', 'text', 'utf16_units', 'escaped']) refuse(value => { delete value.inputs[0][field]; });
  for (const patch of [{ extra: 1 }, { name: 'other' }, { text: null }, { text: '\ud800' }, { text: 'x'.repeat(4097) },
    { text: 'different' }, { utf16_units: [] }, { utf16_units: '42' }, { utf16_units: [0] },
    { utf16_units: [42.5] }, { utf16_units: [-0] }, { utf16_units: [65536] }, { escaped: '*' }, { escaped: null }]) {
    refuse(value => Object.assign(value.inputs[0], patch));
  }
  for (const field of ['facts', 'decision', 'bindings']) refuse(value => { delete value.proof[field]; });
  refuse(value => { value.proof.extra = true; });
  refuse(value => { value.proof.facts.pattern += 'x'; });
  refuse(value => { value.proof.facts.extra = ''; });
  refuse(value => { value.proof.decision.matched = 1; });
  refuse(value => { value.proof.decision.domain_status = 'UNKNOWN'; });
  refuse(value => { value.proof.decision.extra = true; });
  refuse(value => { value.proof.bindings.assets['matcher.gooo'] = 'sha256:' + '0'.repeat(64); });
});

test('known-v1 historical metadata is admitted internally while live runtime pins remain strict', () => {
  const saved = savedObservation();
  const changed = withSavedProgram(saved, program => {
    program.compiler_revision = program.bindings.compiler_source_sha = 'd'.repeat(40);
    program.generation_sha256 = program.bindings.generation_sha256 = 'sha256:' + 'e'.repeat(64);
    program.files['source/generation.json'].sha256 = 'e'.repeat(64);
    program.go_version = 'go1.26.5'; program.source_revision = 'f'.repeat(40); program.source_dirty = true;
    program.wasm_sha256 = program.files['program.wasm'].sha256 = 'd'.repeat(64);
    program.wasm_bytes = program.files['program.wasm'].bytes = 101;
    program.wasm_exec_sha256 = program.files['wasm_exec.js'].sha256 = 'e'.repeat(64);
    program.wasm_exec_bytes = program.files['wasm_exec.js'].bytes = 102;
    program.bindings.assets['matcher.gooo'] = 'sha256:' + 'f'.repeat(64);
    program.files['source/matcher.gooo'].sha256 = 'f'.repeat(64);
  });
  assert.deepEqual(clone(protocol.parseObservation(JSON.stringify(changed))), changed);
  assert.throws(() => protocol.validateMetadata(changed.program));
  assert.throws(() => protocol.validateBindings(changed.program.bindings));
  assert.throws(() => protocol.validateReady({ type: 'ready', program: changed.program }));
  const comparison = protocol.compareObservation(changed, saved.mode, saved.proof.facts, saved.proof, saved.program);
  assert.deepEqual(clone(comparison), { decision_equal: true, decision_differences: [], program_equal: false, bindings_equal: false, executable_equal: false });
  for (const isolated of [
    withSavedProgram(saved, p => { p.source_revision = 'f'.repeat(40); }),
    withSavedProgram(saved, p => { p.go_version = 'go2.0.0'; }),
    withSavedProgram(saved, p => { p.source_dirty = !p.source_dirty; }),
    withSavedProgram(saved, p => { p.files['source/NOTICE.md'].bytes++; }),
  ]) {
    const result = protocol.compareObservation(isolated, saved.mode, saved.proof.facts, saved.proof, saved.program);
    assert.equal(result.program_equal, false); assert.equal(result.bindings_equal, true); assert.equal(result.executable_equal, true);
  }
});

test('historical metadata still refuses inconsistent identities, malformed rosters and declared-size bounds', () => {
  const saved = savedObservation();
  const refuse = change => { const altered = withSavedProgram(saved, change); assert.throws(() => protocol.validateObservation(altered)); };
  for (const field of Object.keys(saved.program)) refuse(program => { delete program[field]; });
  for (const patch of [{ schema: 'v2' }, { compiler_revision: 'x'.repeat(40) }, { generation_sha256: 'sha256:' + '0'.repeat(64) },
    { source_revision: 'main' }, { source_dirty: 1 }, { go_version: 'go1.27' }, { go_version: 'go01.27.2' },
    { go_version: 'go1.27.2extra' }, { go_version: 'go1000.0.0' }, { wasm_bytes: 101 }, { wasm_exec_sha256: '0'.repeat(64) }, { extra: true }]) refuse(p => Object.assign(p, patch));
  for (const field of Object.keys(saved.program.bindings)) refuse(p => { delete p.bindings[field]; });
  for (const patch of [{ compiler_source_sha: 'bad' }, { generation_sha256: 'bad' }, { generation_origin: 'unverified' }, { extra: true }]) refuse(p => Object.assign(p.bindings, patch));
  for (const name of protocol.ASSET_NAMES) {
    refuse(p => { delete p.bindings.assets[name]; });
    refuse(p => { p.bindings.assets[name] = '0'.repeat(64); });
  }
  refuse(p => { p.bindings.assets['unknown'] = 'sha256:' + '0'.repeat(64); });
  for (const name of protocol.FILE_NAMES) {
    refuse(p => { delete p.files[name]; });
    for (const patch of [{ bytes: 0 }, { bytes: 1.5 }, { bytes: '1' }, { bytes: 33554433 }, { sha256: 'bad' }, { extra: true }]) refuse(p => Object.assign(p.files[name], patch));
    if (name.startsWith('source/') && (protocol.ASSET_NAMES.includes(name.slice(7)) || name === 'source/LICENSE-APACHE-2.0' || name === 'source/generation.json')) {
      refuse(p => { p.files[name].sha256 = '0'.repeat(64); });
    }
  }
  refuse(p => { p.files['../outside'] = { sha256: '0'.repeat(64), bytes: 1 }; });
  refuse(p => { for (const file of Object.values(p.files)) file.bytes = 33554432; p.wasm_bytes = p.wasm_exec_bytes = 33554432; });
});

test('observation comparison checks every typed decision field without treating saved claims as authority', () => {
  const cases = [
    ['match', { domain_status: 'VALID_INPUT', invalid_field: '', matched: true }, { domain_status: 'INVALID_INPUT', invalid_field: 'pattern', matched: false }],
    ['impact', { domain_status: 'VALID_INPUT', invalid_field: '', before_match: true, after_match: true, change: 'UNCHANGED_ALLOW' },
      { domain_status: 'INVALID_INPUT', invalid_field: 'before', before_match: false, after_match: false, change: 'INVALID_INPUT' }],
    ['refine', { domain_status: 'VALID_INPUT', invalid_field: '', status: 'SUPPORTED_SUBSET', rule_id: 'OLD_GLOBAL', supported_subset: true,
      has_witness: false, witness_type: '', witness_name: '', before_match: false, after_match: false },
      { domain_status: 'VALID_INPUT', invalid_field: '', status: 'WIDENING_WITNESS', rule_id: 'CHECKED_CANDIDATE_REQUEST', supported_subset: false,
        has_witness: true, witness_type: 'mcp', witness_name: 'a.y', before_match: false, after_match: true }],
  ];
  for (const [mode, currentDecision, claimedDecision] of cases) {
    const current = savedObservation(mode, currentDecision), claimed = clone(current); claimed.proof.decision = claimedDecision;
    const result = protocol.compareObservation(claimed, mode, current.proof.facts, current.proof, current.program);
    const expected = protocol.DECISION_FIELDS[mode].filter(field => claimedDecision[field] !== currentDecision[field])
      .map(field => ({ field, saved: claimedDecision[field], current: currentDecision[field] }));
    assert.deepEqual(clone(result.decision_differences), expected);
    assert.equal(result.decision_equal, false); assert.equal(result.program_equal, true); assert.equal(result.bindings_equal, true); assert.equal(result.executable_equal, true);
    assert.equal(Object.isFrozen(result.decision_differences[0]), true);
    for (const field of protocol.DECISION_FIELDS[mode]) {
      const wrong = clone(claimed); wrong.proof.decision[field] = null; assert.throws(() => protocol.compareObservation(wrong, mode, current.proof.facts, current.proof, current.program), field);
    }
    assert.equal(protocol.compareObservation(current, mode, current.proof.facts, current.proof, current.program).decision_equal, true);
    const wrongFacts = { ...current.proof.facts, [FIELDS[mode][0]]: 'other' };
    assert.throws(() => protocol.compareObservation(current, mode, wrongFacts, { ...current.proof, facts: wrongFacts }, current.program));
    const otherMode = mode === 'match' ? 'impact' : 'match', other = savedObservation(otherMode);
    assert.throws(() => protocol.compareObservation(current, otherMode, other.proof.facts, other.proof, other.program));
    const badProgram = clone(current.program); badProgram.compiler_revision = 'd'.repeat(40);
    assert.throws(() => protocol.compareObservation(current, mode, current.proof.facts, current.proof, badProgram));
  }
  // A witness's request text may differ while all Boolean/status fields agree.
  const witness = cases[2][2], current = savedObservation('refine', witness), claimed = clone(current);
  claimed.proof.decision.witness_type = 'other'; claimed.proof.decision.witness_name = 'b.z';
  assert.deepEqual(clone(protocol.compareObservation(claimed, 'refine', current.proof.facts, current.proof, current.program).decision_differences),
    [{ field: 'witness_type', saved: 'other', current: 'mcp' }, { field: 'witness_name', saved: 'b.z', current: 'a.y' }]);
});

test('four maximum-length input exports fit the byte cap and the decoded-text cap has an exact boundary', t => {
  assert.equal(protocol.MAX_OBSERVATION_BYTES, 1048576);
  const program = metadata(), facts = Object.fromEntries(FIELDS.impact.map(name => [name, '\u001f'.repeat(4096)]));
  const result = { facts, decision: { domain_status: 'INVALID_INPUT', invalid_field: 'before', before_match: false, after_match: false, change: 'INVALID_INPUT' }, bindings: program.bindings };
  const exported = protocol.observation('impact', facts, result, program), raw = JSON.stringify(exported, null, 2) + '\n';
  const bytes = Buffer.byteLength(raw, 'utf8');
  assert.ok(bytes > 262144, 'exercise the old parser default limit');
  assert.ok(bytes <= protocol.MAX_OBSERVATION_BYTES, 'the supported four-field pretty export must fit');
  assert.deepEqual(clone(protocol.parseObservation(raw)), clone(exported));
  const exact = raw + ' '.repeat(protocol.MAX_OBSERVATION_BYTES - raw.length);
  assert.equal(exact.length, protocol.MAX_OBSERVATION_BYTES); assert.equal(Buffer.byteLength(exact), protocol.MAX_OBSERVATION_BYTES);
  assert.deepEqual(clone(protocol.parseObservation(exact)), clone(exported));
  assert.throws(() => protocol.parseObservation(exact + ' '), /text limit/);
  t.diagnostic(JSON.stringify({ maximum_input_pretty_export_bytes: bytes, fields: 4, units_each: 4096, exact_cap_accepted: protocol.MAX_OBSERVATION_BYTES, one_over_refused: true }));
});
