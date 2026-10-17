// Finite actual Go/Wasm package verification. Node-adapted browser transport
// is recorded separately and is not evidence of Chrome or deployment acceptance.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { createHash, webcrypto } from 'node:crypto';
import { performance } from 'node:perf_hooks';
import { fileURLToPath } from 'node:url';
import { Worker, isMainThread, parentPort, workerData } from 'node:worker_threads';
import { execFileSync } from 'node:child_process';

const ORIGINAL = 'sha256:79b76bda704cb63b9390802766a66ac72ba02c04ae135600cb05de6797c97d46';
const REVISION = 'sha256:af9452cce284a6b944adf50c62f6b9f5c1a0647d6f1794d3549dd5dae7ff43ee';
const COMPILER = '83928fa4538245abe33b08549da47f8cbdc50ed2';
const FACTS = ['group_present', 'group_value', 'principal_present', 'principal_value', 'authenticated', 'caller_tenant', 'resource_tenant', 'requested_action', 'allowed_action'];
const TEXTS = ['group_value', 'principal_value', 'caller_tenant', 'resource_tenant', 'requested_action', 'allowed_action'];
const DECISIONS = ['allowed', 'reason', 'authenticated', 'tenant_present', 'tenant_matched', 'action_present', 'action_matched', 'access_allowed', 'access_error', 'access_result', 'group_matched', 'principal_matched', 'group_missing', 'principal_defaulted', 'effective_principal'];
const REPORT = ['schema', 'status', 'scope', 'before_profile', 'candidate_profile', 'before', 'after', 'differences', 'summary', 'classifier_bindings'];
const LIMITS = { text_utf8_bytes: 65536, baseline_input_bytes: 4194304, compare_input_bytes: 4194304,
  replay_input_bytes: 16777216, original_proof_bytes: 4194304, native_facts_raw_bytes: 3539232,
  output_bytes: 67108864 };
// Literal source-derived observation for all-false presence/authentication and
// identical nonempty raw Text fields. An assertion, never a policy fallback.
const RAW_DENIAL = { allowed: false, reason: 'NOT_AUTHENTICATED', authenticated: false,
  tenant_present: true, tenant_matched: true, action_present: true, action_matched: true,
  access_allowed: false, access_error: true, access_result: 'ERROR_NO_SUCH_KEY_GROUP',
  group_matched: false, principal_matched: false, group_missing: true,
  principal_defaulted: true, effective_principal: '' };
const hash = bytes => createHash('sha256').update(bytes).digest('hex');
const clone = value => JSON.parse(JSON.stringify(value));
const encoder = new TextEncoder();
const decoder = new TextDecoder('utf-8', { fatal: true, ignoreBOM: true });
const ownBytes = bytes => new Uint8Array(bytes);
const utf8 = text => ownBytes(encoder.encode(text));
const bytesEqual = (left, right, message) => assert.equal(Buffer.compare(Buffer.from(left), Buffer.from(right)), 0, message);
const errorRecord = error => {
  const text = String(error.stack || error), bytes = encoder.encode(text);
  return { error: text.slice(0, 8192), error_utf8_bytes: bytes.length, error_sha256: hash(bytes),
    error_text_truncated: text.length > 8192 };
};

function keys(value, names, message) {
  assert.ok(value !== null && typeof value === 'object' && !Array.isArray(value), message);
  assert.deepEqual(Object.keys(value).sort(), [...names].sort(), message);
}

function jsonBytes(value) {
  // Go encoding/json's escaping, over the declared field order in the actual
  // native observation. This is serialization, never a policy/classifier oracle.
  return utf8(JSON.stringify(value).replace(/[<>&\u2028\u2029]/gu,
    character => '\\u' + character.charCodeAt(0).toString(16).padStart(4, '0')));
}

function decodeBytes(bytes) {
  assert.ok(bytes instanceof Uint8Array && Object.getPrototypeOf(bytes) === Uint8Array.prototype);
  assert.ok(bytes.length > 0 && bytes.length <= LIMITS.output_bytes);
  const text = decoder.decode(bytes);
  bytesEqual(encoder.encode(text), bytes, 'native output must roundtrip as exact UTF-8 bytes');
  assert.ok(!text.startsWith('\ufeff'), 'native JSON output contains BOM');
  return { bytes: ownBytes(bytes), value: JSON.parse(text) };
}

function bindings(value, generation, count) {
  keys(value, ['compiler_source_sha', 'assets', 'generation_sha256']);
  assert.equal(value.compiler_source_sha, COMPILER);
  assert.equal(value.generation_sha256, generation);
  assert.equal(Object.keys(value.assets).length, count);
  for (const [name, digest] of Object.entries(value.assets)) {
    assert.ok(name.length > 0 && !path.posix.isAbsolute(name) && !name.split('/').includes('..'));
    assert.match(digest, /^sha256:[0-9a-f]{64}$/u);
  }
}

function validateReport(report) {
  keys(report, REPORT);
  assert.equal(report.schema, 'gooo/api-policy-revision-comparison/v1');
  assert.equal(report.status, 'COMPARED');
  assert.equal(report.scope, 'same_saved_facts_two_sealed_profiles');
  assert.equal(report.before_profile, 'cel-api-original-v1');
  assert.equal(report.candidate_profile, 'cel-api-readonly-principal-v1');
  for (const side of ['before', 'after']) {
    keys(report[side], ['facts', 'decision', 'bindings']);
    keys(report[side].facts, FACTS);
    for (const name of FACTS) assert.equal(typeof report[side].facts[name], TEXTS.includes(name) ? 'string' : 'boolean');
    keys(report[side].decision, DECISIONS);
    for (const name of DECISIONS) assert.equal(typeof report[side].decision[name], ['reason', 'access_result', 'effective_principal'].includes(name) ? 'string' : 'boolean');
  }
  keys(report.differences, DECISIONS);
  for (const value of Object.values(report.differences)) assert.equal(typeof value, 'boolean');
  keys(report.summary, ['classification', 'changed_count']);
  assert.equal(typeof report.summary.classification, 'string');
  assert.ok(Number.isSafeInteger(report.summary.changed_count));
  bindings(report.before.bindings, ORIGINAL, 28);
  bindings(report.after.bindings, REVISION, 37);
  assert.deepEqual(report.classifier_bindings, report.after.bindings);
}

async function workerSuite() {
  let phase = 'initialization', runtimeFailure;
  const send = value => parentPort.postMessage(value);
  const measured = (name, kind, fn) => {
    send({ type: 'step-start', name, kind });
    const started = performance.now();
    try {
      const { value, metrics = {} } = fn();
      send({ type: 'step', record: { name, kind, status: 'PASS', elapsed_ms: performance.now() - started, ...metrics } });
      return value;
    } catch (error) {
      send({ type: 'step', record: { name, kind, status: 'FAIL', elapsed_ms: performance.now() - started, ...errorRecord(error) } });
      throw error;
    }
  };
  try {
    globalThis.crypto ??= webcrypto;
    const { fixture, expected } = workerData.fixtures;
    send({ type: 'step-start', name: 'actual-Go-Wasm-startup', kind: 'initialization' });
    const startup = performance.now();
    // The actual supplied official wasm_exec.js executes in this Worker global
    // realm. API Uint8Arrays are constructed here, never in a separate vm realm.
    vm.runInThisContext(fs.readFileSync(workerData.options['wasm-exec'], 'utf8'), { filename: workerData.options['wasm-exec'] });
    assert.equal(typeof globalThis.Go, 'function');
    const go = new globalThis.Go();
    const { instance } = await WebAssembly.instantiate(fs.readFileSync(workerData.options.wasm), go.importObject);
    go.run(instance).then(() => { runtimeFailure = new Error('Go.run terminated unexpectedly'); }, error => { runtimeFailure = error; });
    await Promise.resolve();
    assert.equal(runtimeFailure, undefined);
    assert.equal(typeof globalThis.goooAPIRevision, 'function');
    const infoResult = decodeBytes(globalThis.goooAPIRevisionInfo);
    const info = infoResult.value;
    keys(info, ['schema', 'scope', 'modes', 'profiles', 'limits', 'bindings']);
    assert.equal(info.schema, 'gooo/api-policy-browser-bridge-info/v1');
    assert.equal(info.scope, 'unsigned_historical_hypothetical_comparison');
    assert.deepEqual(info.modes, ['baseline', 'compare', 'replay']);
    assert.deepEqual(info.profiles, { original: 'cel-api-original-v1', candidate: 'cel-api-readonly-principal-v1' });
    assert.deepEqual(info.limits, LIMITS);
    assert.deepEqual(info.bindings, expected, 'all actual native source bindings');
    phase = 'cases';
    send({ type: 'ready', info, info_utf8_bytes: infoResult.bytes.length,
      info_sha256: hash(infoResult.bytes), info_utf8_byte_roundtrip_exact: true,
      startup_elapsed_ms: performance.now() - startup });

    const invoke = args => {
      assert.equal(runtimeFailure, undefined);
      return globalThis.goooAPIRevision(...args);
    };
    const accept = (name, mode, input, want, expectationOrigin = 'actual-native-fixture') => measured(name, 'native-success', () => {
      const before = ownBytes(input);
      const response = invoke([mode, input]);
      keys(response, ['ok', 'data']); assert.equal(response.ok, true);
      bytesEqual(input, before, 'input bytes were mutated');
      const result = decodeBytes(response.data);
      if (want !== undefined) {
        assert.deepEqual(result.value, want, 'all typed native fixture fields');
        bytesEqual(result.bytes, jsonBytes(want), 'canonical bytes from actual native fixture');
      }
      return { value: result, metrics: { mode, input_bytes: input.length, input_sha256: hash(input), output_bytes: result.bytes.length,
        output_sha256: hash(result.bytes), exact_utf8_roundtrip: true,
        independent_fixture_exact: want !== undefined && expectationOrigin === 'actual-native-fixture',
        expected_record_origin: want !== undefined ? expectationOrigin : 'native-transport-roundtrip' } };
    });
    const reject = (name, args, options = {}) => measured(name, options.kind || 'explicit-no-data-refusal', () => {
      const response = invoke(args);
      keys(response, ['ok', 'error']); assert.equal(response.ok, false);
      assert.equal(typeof response.error, 'string'); assert.ok(response.error.length > 0);
      assert.equal(Object.hasOwn(response, 'data'), false);
      if (options.expectedError !== undefined) assert.equal(response.error, options.expectedError, 'actual native refusal stage');
      const metrics = { no_data: true, returned_error: response.error,
        ...(options.stage ? { expected_native_stage: options.stage } : {}),
        ...(options.validity ? { saved_mutation_validity: options.validity } : {}) };
      const input = args[1];
      if (ArrayBuffer.isView(input) && Object.getPrototypeOf(input) === Uint8Array.prototype
          && input.buffer instanceof ArrayBuffer && !input.buffer.resizable) {
        metrics.input_bytes = input.byteLength;
        if (input.byteLength > 0) metrics.input_sha256 = hash(input);
        else metrics.input_hash_observed = false;
      }
      return { value: undefined, metrics };
    });
    const requestFor = report => ({ before_profile: report.before_profile, candidate_profile: report.candidate_profile, before: report.before });
    const pipeline = (name, facts, fixtureReport, literalDenial = false) => {
      const baseline = accept(name + ':baseline', 'baseline', jsonBytes(facts), fixtureReport ? requestFor(fixtureReport) : undefined);
      assert.deepEqual(baseline.value.before.facts, facts);
      const comparison = accept(name + ':compare', 'compare', ownBytes(baseline.bytes), fixtureReport);
      validateReport(comparison.value);
      assert.deepEqual(comparison.value.before, baseline.value.before);
      assert.deepEqual(comparison.value.before.facts, facts); assert.deepEqual(comparison.value.after.facts, facts);
      assert.deepEqual(comparison.value.before.bindings, expected.original);
      assert.deepEqual(comparison.value.after.bindings, expected.revision);
      if (literalDenial) measured(name + ':all-literal-boundary-fields', 'complete-literal-boundary', () => {
        assert.deepEqual(comparison.value.before.decision, RAW_DENIAL);
        assert.deepEqual(comparison.value.after.decision, RAW_DENIAL);
        assert.deepEqual(comparison.value.differences, fixture.cases[0].report.differences);
        assert.deepEqual(comparison.value.summary, { classification: 'UNCHANGED', changed_count: 0 });
        return { value: undefined, metrics: { facts: 9, decision_fields_per_side: 15, difference_flags: 15,
          summary_fields: 2, full_binding_assets: 64,
          expectation_origin: 'source-derived literal denial, not another independent native run' } };
      });
      const replay = accept(name + ':replay', 'replay', ownBytes(comparison.bytes), fixtureReport);
      assert.deepEqual(replay.value, comparison.value); bytesEqual(replay.bytes, comparison.bytes, 'native saved replay bytes');
      return { baseline, comparison };
    };

    for (const row of fixture.cases) pipeline('native-fixture:' + row.action, row.report.before.facts, row.report);
    const readReport = fixture.cases[0].report;
    const readFacts = readReport.before.facts;
    // A full ordinary caller view may still own methods/constructor properties.
    // These regressions never alter global intrinsics. The validated buffer's
    // original bytes must remain authoritative through the actual Go copy.
    const writeReport = fixture.cases[1].report;
    const transportPayloads = report => ({ baseline: report.before.facts,
      compare: requestFor(report), replay: report });
    const readPayloads = transportPayloads(readReport), writePayloads = transportPayloads(writeReport);
    const transportOverrides = [
      ['own-subarray-substitution', (input, counts, substitute) => {
        Object.defineProperty(input, 'subarray', { value: function () {
          counts.subarray_calls++; return ownBytes(substitute);
        } });
      }],
      ['own-subarray-throw', (input, counts) => {
        Object.defineProperty(input, 'subarray', { value: function () {
          counts.subarray_calls++; throw new Error('caller subarray must not execute');
        } });
      }],
      ['own-constructor-getter-throw', (input, counts) => {
        Object.defineProperty(input, 'constructor', { get() {
          counts.constructor_reads++; throw new Error('caller constructor must not execute');
        } });
      }],
      ['own-constructor-species-getter-throw', (input, counts) => {
        const constructor = {};
        Object.defineProperty(constructor, Symbol.species, { get() {
          counts.species_reads++; throw new Error('caller species getter must not execute');
        } });
        Object.defineProperty(input, 'constructor', { value: constructor });
      }],
      ['own-constructor-species-substitution', (input, counts, substitute) => {
        const constructor = { [Symbol.species]: function AlternateSpecies() {
          counts.species_constructor_calls++; return ownBytes(substitute);
        } };
        Object.defineProperty(input, 'constructor', { value: constructor });
      }],
    ];
    for (const mode of ['baseline', 'compare', 'replay']) {
      const originalCanonical = jsonBytes(readPayloads[mode]);
      const alternateCanonical = jsonBytes(writePayloads[mode]);
      const size = Math.max(originalCanonical.length, alternateCanonical.length);
      const padTransportInput = bytes => {
        const value = new Uint8Array(size); value.fill(0x20); value.set(bytes); return value;
      };
      const originalBytes = padTransportInput(originalCanonical);
      const substitutedBytes = padTransportInput(alternateCanonical);
      assert.equal(originalBytes.length, substitutedBytes.length);
      assert.notEqual(hash(originalBytes), hash(substitutedBytes));
      // Both same-length byte payloads are valid native inputs. A successful
      // ordinary write call also prevents a bridge from simply replaying the
      // earlier read fixture instead of respecting genuine caller bytes.
      accept('caller-owned-byte-authority:' + mode + ':write', mode, ownBytes(substitutedBytes),
        mode === 'baseline' ? requestFor(writeReport) : writeReport);
      for (const [label, decorate] of transportOverrides) measured('caller-own-override:' + mode + ':' + label,
        'caller-own-transport-regression', () => {
          const input = ownBytes(originalBytes);
          const counts = { subarray_calls: 0, constructor_reads: 0, species_reads: 0, species_constructor_calls: 0 };
          decorate(input, counts, substitutedBytes);
          assert.equal(Object.getPrototypeOf(input), Uint8Array.prototype);
          assert.equal(input.byteOffset, 0); assert.equal(input.byteLength, input.buffer.byteLength);
          const response = invoke([mode, input]);
          assert.deepEqual(Object.values(counts), [0, 0, 0, 0], 'no caller method/constructor/species may execute');
          keys(response, ['ok', 'data']); assert.equal(response.ok, true);
          const result = decodeBytes(response.data);
          const wanted = mode === 'baseline' ? requestFor(readReport) : readReport;
          assert.deepEqual(result.value, wanted, 'all original read-byte facts/decisions/flags/bindings');
          bytesEqual(result.bytes, jsonBytes(wanted), 'original native canonical result bytes');
          bytesEqual(new Uint8Array(input.buffer), originalBytes, 'genuine caller buffer bytes remain unchanged');
          assert.deepEqual(Object.values(counts), [0, 0, 0, 0], 'harness must not invoke caller overrides either');
          return { value: undefined, metrics: { mode, input_bytes: originalBytes.length,
            input_sha256: hash(originalBytes), substitute_bytes: substitutedBytes.length,
            substitute_sha256: hash(substitutedBytes), output_bytes: result.bytes.length,
            output_sha256: hash(result.bytes), override_calls: counts, original_bytes_authoritative: true,
            caller_buffer_unchanged: true, complete_native_fixture_exact: true, exact_utf8_roundtrip: true } };
        });
    }
    const comparisonMismatch = 'saved comparison differs from complete native re-evaluation';
    const originalDecisionMismatch = 'original proof: saved API decision does not match re-evaluated facts';
    const originalBindingMismatch = 'original proof: saved API proof does not match this bound source generation';
    const changeScalar = value => typeof value === 'boolean' ? !value : value + '!';
    const savedTamper = (name, change, expectedError, stage = 'complete-VerifyComparison', validity = 'transport-valid-typed-single-saved-report-tamper') => {
      const mutant = clone(readReport); change(mutant);
      reject('saved-tamper:' + name, ['replay', jsonBytes(mutant)],
        { kind: 'saved-tamper-no-data-refusal', expectedError, stage, validity });
    };
    for (const side of ['before', 'after']) {
      for (const field of DECISIONS) savedTamper(side + ':decision:' + field,
        mutant => { mutant[side].decision[field] = changeScalar(mutant[side].decision[field]); },
        side === 'before' ? originalDecisionMismatch : comparisonMismatch,
        side === 'before' ? 'original-VerifyAPIProof-decision' : 'complete-VerifyComparison');
      for (const field of FACTS) savedTamper(side + ':one-side-fact:' + field, mutant => {
        mutant[side].facts[field] = changeScalar(mutant[side].facts[field]);
        const other = side === 'before' ? 'after' : 'before';
        assert.deepEqual(mutant[other].facts, readReport[other].facts, 'only one saved side may change');
      }, side === 'before' ? originalDecisionMismatch : comparisonMismatch,
      side === 'before' ? 'original-VerifyAPIProof-facts' : 'complete-VerifyComparison');
    }
    for (const field of DECISIONS) savedTamper('difference:' + field,
      mutant => { mutant.differences[field] = !mutant.differences[field]; }, comparisonMismatch);
    savedTamper('summary:classification', mutant => { mutant.summary.classification = 'REASON_CHANGED'; }, comparisonMismatch);
    savedTamper('summary:changed-count', mutant => { mutant.summary.changed_count = 1; }, comparisonMismatch);
    for (const field of ['schema', 'status', 'scope']) savedTamper('context:' + field,
      mutant => { mutant[field] += '!'; }, 'expected exact typed bounded API revision object',
      'fixed-context-decoder', 'scalar-shape-valid-fixed-identifier-invalid');
    for (const [field, value] of [['before_profile', 'cel-api-readonly-principal-v1'], ['candidate_profile', 'cel-api-original-v1']]) {
      savedTamper('context:' + field, mutant => { mutant[field] = value; },
        'comparison requires the exact original and readonly profiles', 'fixed-profile-decoder',
        'scalar-shape-valid-known-profile-in-wrong-position');
    }
    for (const target of ['before', 'after', 'classifier_bindings']) {
      const get = mutant => target === 'classifier_bindings' ? mutant.classifier_bindings : mutant[target].bindings;
      const error = target === 'before' ? originalBindingMismatch : comparisonMismatch;
      const stage = target === 'before' ? 'original-VerifyAPIProof-bindings' : 'complete-VerifyComparison';
      savedTamper(target + ':compiler', mutant => { get(mutant).compiler_source_sha = '0'.repeat(40); }, error, stage);
      savedTamper(target + ':generation', mutant => { get(mutant).generation_sha256 = 'sha256:' + '0'.repeat(64); }, error, stage);
      for (const asset of Object.keys(get(readReport).assets)) {
        savedTamper(target + ':asset-digest:' + asset, mutant => { get(mutant).assets[asset] = 'sha256:' + '0'.repeat(64); }, error, stage);
        savedTamper(target + ':asset-removal:' + asset, mutant => { delete get(mutant).assets[asset]; }, error, stage);
      }
      savedTamper(target + ':extra-asset', mutant => { get(mutant).assets['consumer-synthetic-extra.txt'] = 'sha256:' + '0'.repeat(64); }, error, stage);
    }
    // Unsigned consistency is not factual attestation. This coherent synthetic
    // change preserves all recorded decisions and is expected to be accepted;
    // it must not be counted among the single-side tamper refusals.
    const coherent = clone(readReport);
    for (const side of ['before', 'after']) {
      coherent[side].facts.caller_tenant = 'consistent-synthetic-tenant';
      coherent[side].facts.resource_tenant = 'consistent-synthetic-tenant';
    }
    accept('consistent-unsigned-synthetic-facts:accepted', 'replay', jsonBytes(coherent), coherent,
      'coherent unsigned synthetic counterexample, not an independent native fixture');
    const validByMode = { baseline: jsonBytes(readFacts), compare: jsonBytes(requestFor(readReport)), replay: jsonBytes(readReport) };
    const duplicateFields = { baseline: ['group_present', 'true'], compare: ['before_profile', '"cel-api-original-v1"'], replay: ['schema', '"gooo/api-policy-revision-comparison/v1"'] };
    for (const mode of ['baseline', 'compare', 'replay']) {
      const valid = validByMode[mode], text = decoder.decode(valid), [field, value] = duplicateFields[mode];
      const needle = JSON.stringify(field) + ':' + value;
      assert.ok(text.includes(needle));
      const escapedKey = '\\u' + field.charCodeAt(0).toString(16).padStart(4, '0') + field.slice(1);
      const malformed = [
        ['empty', new Uint8Array()], ['BOM', ownBytes(Buffer.concat([Buffer.from([0xef, 0xbb, 0xbf]), Buffer.from(valid)]))],
        ['duplicate', utf8(text.replace(needle, needle + ',' + needle))],
        ['escaped-duplicate', utf8(text.replace(needle, needle + ',"' + escapedKey + '":' + value))],
        ['null-root', utf8('null')], ['null-field', utf8(text.replace(needle, JSON.stringify(field) + ':null'))],
        ['wrong-type', utf8(text.replace(needle, JSON.stringify(field) + ':[]'))],
        ['trailing', utf8(text + ' {}')], ['unknown-field', utf8(text.replace('{', '{"unexpected":true,'))],
      ];
      for (const [label, bytes] of malformed) reject(mode + ':malformed:' + label, [mode, bytes]);
      for (const [label, sequence] of [['overlong', [0xc0, 0xaf]], ['continuation', [0x80]], ['truncated', [0xf0, 0x9f, 0x98]], ['surrogate', [0xed, 0xa0, 0x80]]]) {
        const start = text.indexOf('synthetic:user'); assert.ok(start >= 0);
        const bytes = ownBytes(Buffer.concat([Buffer.from(text.slice(0, start), 'utf8'), Buffer.from(sequence), Buffer.from(text.slice(start + 'synthetic:user'.length), 'utf8')]));
        reject(mode + ':malformed-utf8:' + label, [mode, bytes]);
      }
      for (const escape of ['\\ud800', '\\udc00', '\\ud800x', '\\udc00\\ud800']) {
        reject(mode + ':escaped-isolated-surrogate:' + escape, [mode, utf8(text.replace('synthetic:user', escape))]);
      }
    }

    const raw = validByMode.baseline;
    for (const [label, args] of [
      ['missing-all', []], ['missing-bytes', ['baseline']], ['extra-argument', ['baseline', ownBytes(raw), true]],
      ['unknown-mode', ['unknown', ownBytes(raw)]], ['null-mode', [null, ownBytes(raw)]],
      ['number-mode', [1, ownBytes(raw)]], ['coerced-mode', [{ toString: () => 'baseline' }, ownBytes(raw)]],
      ['boxed-mode', [new String('baseline'), ownBytes(raw)]],
    ]) reject('signature:' + label, args);
    for (const [label, value] of [
      ['undefined', undefined], ['null', null], ['boolean', true], ['number', 1], ['JSON-string', decoder.decode(raw)],
      ['lone-high-UTF16-string', '\ud800'], ['lone-low-UTF16-string', '\udc00'], ['emoji-string', '😀'],
      ['array', [...raw]], ['object', {}], ['ArrayBuffer', ownBytes(raw).buffer], ['Uint16Array', new Uint16Array([1])],
      ['clamped', new Uint8ClampedArray(raw)], ['DataView', new DataView(ownBytes(raw).buffer)], ['Buffer', Buffer.from(raw)],
      ['subclass', new (class extends Uint8Array {})(raw)], ['proxy', new Proxy(ownBytes(raw), {})],
    ]) reject('wrong-input-type:' + label, ['baseline', value]);
    const shared = new Uint8Array(new SharedArrayBuffer(raw.length)); shared.set(raw);
    reject('buffer:shared', ['baseline', shared]);
    const resizable = measured('buffer:resizable-capability', 'capability', () => {
      const value = new ArrayBuffer(raw.length, { maxByteLength: raw.length + 32 });
      assert.equal(value.resizable, true);
      return { value, metrics: { resizable_buffer_supported: true } };
    });
    new Uint8Array(resizable).set(raw);
    reject('buffer:resizable', ['baseline', new Uint8Array(resizable)]);
    const detached = ownBytes(raw); structuredClone(detached.buffer, { transfer: [detached.buffer] });
    assert.equal(detached.byteLength, 0); reject('buffer:detached', ['baseline', detached]);
    const partial = new Uint8Array(raw.length + 2); partial.set(raw, 1);
    reject('buffer:subarray', ['baseline', partial.subarray(1, raw.length + 1)]);

    for (const [label, text] of [['replacement', '\ufffd'], ['emoji', '😀'], ['combining', 'e\u0301'], ['NUL', '\u0000'], ['CRLF', '\r\n']]) {
      const facts = Object.fromEntries(FACTS.map(name => [name, TEXTS.includes(name) ? text : false]));
      pipeline('raw-preservation:' + label, facts, undefined, true);
    }
    const maximum = Object.fromEntries(FACTS.map(name => [name, TEXTS.includes(name) ? '<'.repeat(65536) : false]));
    const largest = pipeline('maximum-six-escaped-texts', maximum, undefined, true);
    assert.ok(largest.comparison.bytes.length > LIMITS.compare_input_bytes);
    assert.ok(largest.comparison.bytes.length <= LIMITS.replay_input_bytes);
    assert.ok(decoder.decode(largest.comparison.bytes).includes('\\u003c'));
    for (const field of TEXTS) reject('text-plus-one:' + field, ['baseline', jsonBytes({ ...readFacts, [field]: '<'.repeat(65537) })]);
    const padded = (bytes, size) => {
      assert.ok(bytes.length <= size); const result = new Uint8Array(size); result.fill(0x20); result.set(bytes); return result;
    };
    accept('native-facts-raw-guard:at', 'baseline', padded(validByMode.baseline, LIMITS.native_facts_raw_bytes), requestFor(readReport));
    reject('native-facts-raw-guard:plus-one', ['baseline', padded(validByMode.baseline, LIMITS.native_facts_raw_bytes + 1)]);
    accept('compare-raw-4MiB:at', 'compare', padded(validByMode.compare, LIMITS.compare_input_bytes), readReport);
    for (const mode of ['baseline', 'compare']) reject(mode + ':raw-4MiB-plus-one', [mode, padded(validByMode[mode], LIMITS.compare_input_bytes + 1)]);
    accept('replay-raw-16MiB:at', 'replay', padded(validByMode.replay, LIMITS.replay_input_bytes), readReport);
    reject('replay-raw-16MiB-plus-one', ['replay', padded(validByMode.replay, LIMITS.replay_input_bytes + 1)]);
    await Promise.resolve(); assert.equal(runtimeFailure, undefined);
    send({ type: 'done', maximum_report_bytes: largest.comparison.bytes.length,
      native_fixture_cases: 2, decision_fields: 15, difference_flags: 15, summary_fields: 2,
      full_bindings_exact: true, no_semantic_refusal_inferred_from_timeout: true });
  } catch (error) {
    send({ type: 'failure', phase, ...errorRecord(error) });
  }
}

function parseArguments(argv) {
  const options = { 'timeout-ms': 120000, 'total-timeout-ms': 900000 };
  const allowed = new Set(['site', 'module', 'out']);
  for (let i = 0; i < argv.length; i += 2) {
    const name = argv[i]?.slice(2), value = argv[i + 1];
    assert.ok(argv[i]?.startsWith('--') && allowed.delete(name) && typeof value === 'string', 'unknown, duplicate or incomplete CLI option');
    assert.ok(path.isAbsolute(value), name + ' must be absolute'); options[name] = value;
  }
  assert.equal(allowed.size, 0, '--site, --module and --out are required');
  return options;
}
function descriptor(value, withPath = false) {
  keys(value, withPath ? ['path', 'bytes', 'sha256'] : ['bytes', 'sha256']);
  assert.ok(Number.isSafeInteger(value.bytes) && value.bytes >= 0); assert.match(value.sha256, /^[0-9a-f]{64}$/u);
}
function packageFile(root, relative, maximum = 32 * 1024 * 1024) {
  assert.ok(typeof relative === 'string' && relative.length && !path.posix.isAbsolute(relative) && !relative.includes('\\') && !relative.split('/').includes('..'));
  const realRoot = fs.realpathSync(root), filename = path.resolve(root, relative);
  assert.ok(filename.startsWith(realRoot + path.sep));
  assert.ok(fs.statSync(filename).isFile() && !fs.lstatSync(filename).isSymbolicLink());
  assert.equal(fs.realpathSync(filename), filename, 'declared package/source symlink');
  assert.ok(fs.statSync(filename).size <= maximum, 'bounded package source read');
  return fs.readFileSync(filename);
}
function checkDescriptor(raw, ref) { descriptor(ref, Object.hasOwn(ref, 'path')); assert.equal(raw.length, ref.bytes); assert.equal(hash(raw), ref.sha256); }
function currentCheckout(root) {
  const git = args => execFileSync('git', ['-C', root, ...args], { encoding: 'utf8', env: { ...process.env, GIT_OPTIONAL_LOCKS: '0' }, timeout: 15000, maxBuffer: 4 * 1024 * 1024 });
  return { head: git(['rev-parse', 'HEAD']).trim(), tree: git(['rev-parse', 'HEAD^{tree}']).trim(),
    dirty: git(['status', '--porcelain=v1', '--untracked-files=normal']).length !== 0 };
}
function literalFixture(row) {
  const action = row.action, expectedFacts = { group_present: true, group_value: 'admin', principal_present: true,
    principal_value: 'synthetic:user', authenticated: true, caller_tenant: 'tenant-a', resource_tenant: 'tenant-a', requested_action: action, allowed_action: action };
  const before = { allowed: true, reason: 'ALLOW', authenticated: true, tenant_present: true, tenant_matched: true,
    action_present: true, action_matched: true, access_allowed: true, access_error: false, access_result: 'ALLOW',
    group_matched: true, principal_matched: false, group_missing: false, principal_defaulted: false, effective_principal: 'synthetic:user' };
  const after = { ...before, ...(action === 'write' ? { allowed: false, reason: 'ACTION_NOT_READ_ONLY' } : {}) };
  const flags = { allowed: action === 'write', reason: action === 'write', authenticated: false, tenant_present: false, tenant_matched: false,
    action_present: false, action_matched: false, access_allowed: false, access_error: false, access_result: false, group_matched: false,
    principal_matched: false, group_missing: false, principal_defaulted: false, effective_principal: false };
  assert.deepEqual(row.report.before.facts, expectedFacts); assert.deepEqual(row.report.after.facts, expectedFacts);
  assert.deepEqual(row.report.before.decision, before); assert.deepEqual(row.report.after.decision, after);
  assert.deepEqual(row.report.differences, flags);
  assert.deepEqual(row.report.summary, action === 'write' ? { classification: 'NEWLY_DENIED', changed_count: 2 } : { classification: 'UNCHANGED', changed_count: 0 });
}
function readPackage(options) {
  options.site = fs.realpathSync(options.site); options.module = fs.realpathSync(options.module);
  const packageRoot = path.join(options.site, 'api-policy'), repoRoot = path.resolve(options.module, '../..');
  const manifestRaw = packageFile(packageRoot, 'manifest.json', 524288), manifest = JSON.parse(decoder.decode(manifestRaw));
  keys(manifest, ['schema', 'scope', 'source', 'bridge', 'build_verification', 'files', 'expected_info', 'limits']);
  assert.equal(manifest.schema, 'gooo/browser-api-policy-revision/v1'); assert.equal(manifest.scope, 'unsigned_historical_hypothetical_comparison');
  keys(manifest.files, ['worker.js', 'program.wasm', 'wasm_exec.js']);
  const source = manifest.source;
  keys(source, ['head', 'tree', 'dirty', 'tracked_file_count', 'native_sources', 'ui_files', 'native_fixtures', 'native_fixture_receipt', 'published_sources', 'historical_git_objects_checked']);
  assert.match(source.head, /^[0-9a-f]{40}$/u); assert.match(source.tree, /^[0-9a-f]{40}$/u); assert.equal(typeof source.dirty, 'boolean');
  assert.equal(source.tracked_file_count, 82); assert.equal(source.historical_git_objects_checked, false);
  assert.equal(Object.keys(source.native_sources).length, 82);
  for (const [name, record] of Object.entries(source.native_sources)) {
    keys(record, ['bytes', 'sha256', 'mode']); assert.ok(name.startsWith('examples/api-policy-revision/'));
    assert.ok(['100644', '100755'].includes(record.mode));
    checkDescriptor(packageFile(repoRoot, name), { bytes: record.bytes, sha256: record.sha256 });
    assert.equal(fs.statSync(path.join(repoRoot, name)).mode & 0o111 ? '100755' : '100644', record.mode);
  }
  const uiNames = ['api-policy.html', 'api-policy.js', 'api-policy.css', 'api-policy-worker.js'];
  keys(source.ui_files, uiNames);
  for (const name of uiNames) checkDescriptor(packageFile(options.site, name, 262144), source.ui_files[name]);
  const runtimePaths = { 'worker.js': '../api-policy-worker.js', 'program.wasm': './program.wasm', 'wasm_exec.js': './wasm_exec.js' };
  for (const [name, expectedPath] of Object.entries(runtimePaths)) {
    const declared = manifest.files[name]; descriptor(declared, true); assert.equal(declared.path, expectedPath);
    checkDescriptor(packageFile(name === 'worker.js' ? options.site : packageRoot, name === 'worker.js' ? 'api-policy-worker.js' : name), declared);
  }
  assert.equal(manifest.bridge.path, './source/api-policy-browser-bridge.go.txt');
  assert.equal(manifest.build_verification.path, './build.json');
  for (const ref of [manifest.bridge, manifest.build_verification, source.native_fixtures, source.native_fixture_receipt]) {
    assert.ok(ref.path.startsWith('./')); checkDescriptor(packageFile(packageRoot, ref.path.slice(2), 4194304), ref);
  }
  assert.equal(source.native_fixtures.path, './native-fixtures.json'); assert.equal(source.native_fixture_receipt.path, './native-fixtures-receipt.json');
  // Independent closed roster: a coherently rehashed package must not silently
  // omit a source file or replace a checkout source with another valid payload.
  const revisionPublished = ['readonly.gooo', 'compare.gooo', 'candidate.workspace.json', 'differences.workspace.json',
    'summary.workspace.json', 'profiles.json', 'model.go', 'compare.go', 'input.go', 'bindings.go', 'adapter_generated.go',
    'go.mod', 'generation.json', 'compiler.json', 'canonical/main.gooo', 'canonical/scalar_access.gooo',
    'internal/candidate/generated.go', 'internal/flags/generated.go', 'internal/summary/generated.go', 'cmd/compare/main.go', 'cmd/compare/diagnostics.go'];
  const originalPublished = ['adapter_generated.go', 'model.go', 'bridge.go', 'bindings.go', 'input.go', 'proof_input.go',
    'replay.go', 'internal/api/generated.go', 'internal/scalar/generated.go', 'canonical/main.gooo', 'canonical/scalar_access.gooo',
    'canonical/cel-api-policy.workspace.json', 'canonical/cel-scalar.workspace.json', 'canonical/source-pin.json',
    'compiler.json', 'generation.json', 'go.mod', 'LICENSE-APACHE-2.0', 'NOTICE.md', 'oracle/upstream/LICENSE'];
  const publishedOrigins = Object.fromEntries([
    ...revisionPublished.map(name => ['source/revision/' + name, 'examples/api-policy-revision/' + name]),
    ...originalPublished.map(name => ['source/original/' + name, 'examples/api-policy-revision/profiles/original/' + name]),
    ['source/api-policy-browser-bridge.go.txt', 'tools/api-policy-browser-bridge.go.txt'],
    ['source/LICENSE', 'LICENSE'], ['source/NOTICE', 'NOTICE'], ['source/GO-LICENSE', 'third_party/GO-LICENSE'],
  ]);
  keys(source.published_sources, Object.keys(publishedOrigins));
  for (const [name, record] of Object.entries(source.published_sources)) {
    assert.ok(name.startsWith('source/')); checkDescriptor(packageFile(packageRoot, name, 4194304), record);
  }
  const bridgeRaw = packageFile(repoRoot, 'tools/api-policy-browser-bridge.go.txt', 131072);
  checkDescriptor(bridgeRaw, manifest.bridge);
  bytesEqual(bridgeRaw, packageFile(packageRoot, 'source/api-policy-browser-bridge.go.txt', 131072), 'published exact bridge template');
  const raw = packageFile(packageRoot, 'native-fixtures.json', 1048576), fixture = JSON.parse(decoder.decode(raw));
  keys(fixture, ['schema', 'scope', 'cases']); assert.equal(fixture.schema, 'gooo/api-policy-consumer-observation/v1');
  assert.equal(fixture.scope, 'synthetic_unsigned_historical_hypothetical_readonly');
  assert.deepEqual(fixture.cases.map(row => row.action), ['read', 'write']);
  for (const row of fixture.cases) { keys(row, ['action', 'report']); validateReport(row.report); literalFixture(row); }
  const expected = { original: fixture.cases[0].report.before.bindings, revision: fixture.cases[0].report.after.bindings };
  for (const row of fixture.cases) {
    assert.deepEqual(row.report.before.bindings, expected.original); assert.deepEqual(row.report.after.bindings, expected.revision); assert.deepEqual(row.report.classifier_bindings, expected.revision);
  }
  for (const [root, wanted] of [[options.module, expected.revision], [path.join(options.module, 'profiles/original'), expected.original]]) {
    const generationRaw = packageFile(root, 'generation.json'), generation = JSON.parse(decoder.decode(generationRaw));
    assert.equal('sha256:' + hash(generationRaw), wanted.generation_sha256); assert.deepEqual(generation.assets, wanted.assets);
    for (const [name, digest] of Object.entries(wanted.assets)) assert.equal('sha256:' + hash(packageFile(root, name)), digest);
  }
  const info = manifest.expected_info;
  keys(info, ['schema', 'scope', 'modes', 'profiles', 'limits', 'bindings']);
  assert.equal(info.schema, 'gooo/api-policy-browser-bridge-info/v1'); assert.equal(info.scope, 'unsigned_historical_hypothetical_comparison');
  assert.deepEqual(info.modes, ['baseline', 'compare', 'replay']);
  assert.deepEqual(info.profiles, { original: 'cel-api-original-v1', candidate: 'cel-api-readonly-principal-v1' });
  assert.deepEqual(info.limits, LIMITS); assert.deepEqual(info.bindings, expected);
  const receipt = JSON.parse(decoder.decode(packageFile(packageRoot, 'native-fixtures-receipt.json', 1048576)));
  keys(receipt, ['schema', 'status', 'scope', 'source', 'go', 'fixture', 'processes', 'acceptance', 'limits']);
  assert.equal(receipt.schema, 'gooo/api-policy-browser-native-fixtures-receipt/v1'); assert.equal(receipt.status, 'FRESH_NATIVE_CLI_FIXTURES_PASS');
  assert.equal(receipt.scope, 'source_local_two_explicit_replacements_not_public_noReplace');
  keys(receipt.source, ['head', 'tree', 'dirty', 'native_sources_sha256', 'original_generation', 'revision_generation']);
  assert.deepEqual(receipt.source, { head: source.head, tree: source.tree, dirty: source.dirty,
    native_sources_sha256: hash(jsonBytes(source.native_sources)), original_generation: ORIGINAL, revision_generation: REVISION });
  keys(receipt.go, ['version', 'executable', 'runtime']); assert.equal(receipt.go.version, 'go1.27.2');
  descriptor(receipt.go.executable); assert.ok(receipt.go.executable.bytes > 0);
  assert.deepEqual(receipt.go.runtime, { bytes: manifest.files['wasm_exec.js'].bytes, sha256: manifest.files['wasm_exec.js'].sha256 });
  assert.deepEqual(receipt.fixture, source.native_fixtures);
  assert.deepEqual(receipt.acceptance, { cases: 2, facts: 9, decision_fields_per_side: 15, difference_flags: 15, summary_fields: 2,
    full_bindings_exact: true, raw_stdout_one_LF: true, replay_raw_bytes_exact: true });
  assert.ok(Array.isArray(receipt.limits) && receipt.limits.every(value => typeof value === 'string'));
  const processes = [];
  for (const row of fixture.cases) {
    const request = { before_profile: row.report.before_profile, candidate_profile: row.report.candidate_profile, before: row.report.before };
    const baseline = Buffer.from(jsonBytes(request)), compared = Buffer.from(jsonBytes(row.report));
    const baselineRaw = Buffer.concat([baseline, Buffer.from('\n')]), reportRaw = Buffer.concat([compared, Buffer.from('\n')]);
    for (const [operation, input, stdout, canonical] of [['baseline', jsonBytes(row.report.before.facts), baselineRaw, baseline], ['compare', baselineRaw, reportRaw, compared], ['replay', reportRaw, reportRaw, compared]]) {
      const metric = bytes => ({ bytes: bytes.length, sha256: hash(bytes) });
      processes.push({ name: row.action + '-' + operation, operation, action: row.action, exit_code: 0,
        stdin: metric(input), stdout: metric(stdout), stderr: metric(Buffer.alloc(0)), canonical_stdout: metric(canonical) });
    }
  }
  assert.deepEqual(receipt.processes, processes, 'all actual CLI input/raw stdout/specified canonical bytes and exits');
  const build = JSON.parse(decoder.decode(packageFile(packageRoot, 'build.json', 4194304)));
  keys(build, ['schema', 'status', 'scope', 'source', 'go', 'native_executable', 'temporary_main_module_sha256', 'explicit_local_replacements',
    'temporary_sources_native_binary_module_cache_removed', 'normal_build_cache_retained_not_copied', 'wasm', 'runtime', 'native_fixtures',
    'native_fixture_receipt', 'acceptance', 'published_source_subset', 'verbose_process_logs_published', 'limits']);
  assert.equal(build.schema, 'gooo/browser-api-policy-build/v1'); assert.equal(build.status, 'NATIVE_FIXTURES_AND_WASM_BUILD_PASS');
  assert.equal(build.scope, 'source_local_generated_Gooo_native_to_Wasm_build_not_browser_execution');
  keys(build.source, ['head', 'tree', 'dirty', 'native_sources', 'control_sources', 'source_before_after_equal', 'historical_git_objects_checked']);
  assert.deepEqual({ head: build.source.head, tree: build.source.tree, dirty: build.source.dirty }, { head: source.head, tree: source.tree, dirty: source.dirty });
  assert.deepEqual(build.source.native_sources, source.native_sources); assert.equal(build.source.source_before_after_equal, true);
  assert.equal(build.source.historical_git_objects_checked, false);
  const controlNames = ['upstream.json', 'cmd/build-demo/main.go', 'cmd/build-demo/api_policy.go', 'cmd/build-demo/api_policy_build.go', 'tools/api-policy-browser-bridge.go.txt',
    'site/api-policy.html', 'site/api-policy.js', 'site/api-policy.css', 'site/api-policy-worker.js', 'LICENSE', 'NOTICE', 'third_party/GO-LICENSE'];
  keys(build.source.control_sources, controlNames);
  for (const name of controlNames) checkDescriptor(packageFile(repoRoot, name, 4194304), build.source.control_sources[name]);
  for (const name of uiNames) assert.deepEqual(source.ui_files[name], build.source.control_sources['site/' + name],
    'packaged UI must match recorded checkout control: ' + name);
  for (const [publishedName, sourceName] of Object.entries(publishedOrigins)) {
    const original = sourceName.startsWith('examples/') ? source.native_sources[sourceName] : build.source.control_sources[sourceName];
    assert.ok(original, 'missing checkout source for public payload: ' + publishedName);
    assert.deepEqual(source.published_sources[publishedName], { bytes: original.bytes, sha256: original.sha256 },
      'published payload must match checkout source: ' + publishedName);
  }
  assert.deepEqual(build.go, receipt.go); descriptor(build.native_executable); assert.ok(build.native_executable.bytes > 0);
  assert.match(build.temporary_main_module_sha256, /^[0-9a-f]{64}$/u); assert.equal(build.explicit_local_replacements, 2);
  assert.equal(build.temporary_sources_native_binary_module_cache_removed, true); assert.equal(build.normal_build_cache_retained_not_copied, true);
  assert.deepEqual(build.wasm, manifest.files['program.wasm']); assert.deepEqual(build.runtime, manifest.files['wasm_exec.js']);
  assert.deepEqual(build.native_fixtures, source.native_fixtures); assert.deepEqual(build.native_fixture_receipt, source.native_fixture_receipt);
  assert.deepEqual(build.acceptance, receipt.acceptance); assert.deepEqual(build.published_source_subset, source.published_sources);
  assert.equal(build.verbose_process_logs_published, false); assert.ok(Array.isArray(build.limits) && build.limits.every(value => typeof value === 'string'));
  const checkout = currentCheckout(repoRoot);
  assert.deepEqual(checkout, { head: source.head, tree: source.tree, dirty: source.dirty }, 'actual shallow checkout identity/dirty observation');
  options['wasm-exec'] = path.join(packageRoot, 'wasm_exec.js'); options.wasm = path.join(packageRoot, 'program.wasm'); options.observations = path.join(packageRoot, 'native-fixtures.json');
  const watch = [...new Set([fileURLToPath(import.meta.url), path.join(packageRoot, 'manifest.json'), path.join(packageRoot, 'build.json'), path.join(packageRoot, 'native-fixtures.json'), path.join(packageRoot, 'native-fixtures-receipt.json'), options['wasm-exec'], options.wasm,
    ...uiNames.map(name => path.join(options.site, name)), ...controlNames.map(name => path.join(repoRoot, name)), ...Object.keys(source.native_sources).map(name => path.join(repoRoot, name)), ...Object.keys(source.published_sources).map(name => path.join(packageRoot, name))])];
  return { fixture, expected, fixture_sha256: hash(raw), fixture_bytes: raw.length, manifest, watch,
    checkout, repoRoot,
    binding_scope: 'Complete65 local bound source bytes and declared public payloads checked by Node; UI only fetches Worker/runtime/Wasm and compares full metadata. Native executable identity is recorded in the builder receipt; this reader does not fetch or re-execute it.' };
}
async function shippedLoaderWorker() {
  const { options, manifest, fault, input, operation } = workerData;
  const metrics = { fetches: {}, runtime_blob_imports: 0, wasm_instantiations: 0, native_calls: 0, runtime_snapshot_exact: false, wasm_snapshot_exact: false };
  try {
    globalThis.crypto ??= webcrypto;
    globalThis.self = globalThis;
    globalThis.location = new URL('blob:https://api-policy-package.test/owned-worker');
    let requestHandler;
    globalThis.addEventListener = (name, handler) => { assert.equal(name, 'message'); requestHandler = handler; };
    globalThis.postMessage = (message, transfer) => parentPort.postMessage({ type: 'loader-message', message, metrics: clone(metrics) }, transfer);
    globalThis.fetch = async (url, fetchOptions) => {
      assert.deepEqual(fetchOptions, { cache: 'no-store', credentials: 'omit', redirect: 'error', referrerPolicy: 'no-referrer' });
      const parsed = new URL(url); assert.equal(parsed.origin, 'https://api-policy-package.test');
      assert.ok(['/api-policy/program.wasm', '/api-policy/wasm_exec.js'].includes(parsed.pathname));
      assert.equal(parsed.search + parsed.hash, '');
      metrics.fetches[parsed.pathname] = (metrics.fetches[parsed.pathname] || 0) + 1;
      assert.equal(metrics.fetches[parsed.pathname], 1, 'runtime/Wasm must not be refetched for execution');
      let raw = fs.readFileSync(path.join(options.site, parsed.pathname.slice(1)));
      if ((fault === 'runtime-hash' && parsed.pathname.endsWith('wasm_exec.js')) || (fault === 'wasm-hash' && parsed.pathname.endsWith('program.wasm'))) {
        raw = Buffer.from(raw); raw[raw.length - 1] ^= 1;
      }
      let read = false;
      return { ok: true, body: { getReader() { return { async read() {
        if (read) return { done: true }; read = true; return { done: false, value: new Uint8Array(raw) };
      }, releaseLock() {} }; } } };
    };
    const blobs = new Map(); let serial = 0;
    globalThis.Blob = class { constructor(parts, config) { assert.equal(config.type, 'text/javascript'); this.raw = Buffer.concat(parts.map(value => Buffer.from(value))); } };
    URL.createObjectURL = value => { const url = 'blob:https://api-policy-package.test/' + (++serial); blobs.set(url, value.raw); return url; };
    URL.revokeObjectURL = url => assert.ok(blobs.delete(url));
    globalThis.importScripts = url => {
      assert.ok(blobs.has(url), 'execute exact verified runtime Blob, not a network refetch');
      const raw = blobs.get(url); assert.equal(hash(raw), manifest.files['wasm_exec.js'].sha256);
      metrics.runtime_blob_imports++; metrics.runtime_snapshot_exact = true;
      vm.runInThisContext(decoder.decode(raw), { filename: 'checked-api-policy-wasm_exec.js' });
      const originalRun = globalThis.Go.prototype.run;
      globalThis.Go.prototype.run = function(instance) {
        const running = originalRun.call(this, instance), callback = globalThis.goooAPIRevision;
        assert.equal(typeof callback, 'function');
        globalThis.goooAPIRevision = (...args) => { metrics.native_calls++; return callback(...args); };
        if (fault === 'native-binding') {
          const info = JSON.parse(decoder.decode(globalThis.goooAPIRevisionInfo));
          const key = Object.keys(info.bindings.original.assets)[0]; info.bindings.original.assets[key] = 'sha256:' + '0'.repeat(64);
          globalThis.goooAPIRevisionInfo = new Uint8Array(jsonBytes(info));
        }
        return running;
      };
    };
    const instantiate = WebAssembly.instantiate;
    WebAssembly.instantiate = async (raw, imports) => {
      assert.equal(hash(raw), manifest.files['program.wasm'].sha256); metrics.wasm_instantiations++; metrics.wasm_snapshot_exact = true;
      return instantiate(raw, imports);
    };
    vm.runInThisContext(fs.readFileSync(path.join(options.site, 'api-policy-worker.js'), 'utf8'), { filename: 'shipped-api-policy-worker.js' });
    assert.equal(typeof requestHandler, 'function');
    const config = { program: { ...manifest.files['program.wasm'], url: 'https://api-policy-package.test/api-policy/program.wasm' },
      runtime: { ...manifest.files['wasm_exec.js'], url: 'https://api-policy-package.test/api-policy/wasm_exec.js' }, expected_info: clone(manifest.expected_info) };
    if (fault === 'path') config.program.path = './unexpected.wasm';
    await requestHandler({ data: { id: 1, operation, bytes: new Uint8Array(input).buffer, config } });
  } catch (error) { parentPort.postMessage({ type: 'adapter-failure', ...errorRecord(error), metrics }); }
}
async function loaderCase(options, manifest, fixtures, fault, operation) {
  const started = performance.now(), report = fixtures.fixture.cases[operation === 'compare' ? 1 : 0].report;
  const input = jsonBytes(operation === 'compare' ? report.before.facts : report);
  const worker = new Worker(new URL(import.meta.url), { workerData: { kind: 'shipped-loader', options, manifest, fault, operation, input }, stdout: true, stderr: true });
  let timer, logs = { stdout: 0, stderr: 0 }, terminal;
  try {
    terminal = await new Promise((resolve, reject) => {
      timer = setTimeout(() => reject(new Error('INCOMPLETE: shipped Worker operational timeout, not an input refusal')), options['timeout-ms']);
      for (const stream of ['stdout', 'stderr']) worker[stream].on('data', data => { logs[stream] += data.length; if (logs[stream] > 65536) reject(new Error('INCOMPLETE: bounded adapter diagnostics exceeded')); });
      worker.on('error', reject); worker.on('exit', code => reject(new Error('INCOMPLETE: shipped Worker exited before terminal observation: ' + code)));
      worker.on('message', message => {
        try {
          if (message.type === 'adapter-failure') { reject(new Error('Adapter failure: ' + message.error)); return; }
          assert.equal(message.type, 'loader-message');
          if (message.message.type === 'phase') return;
          resolve(message);
        } catch (error) { reject(error); }
      });
    });
    const message = terminal.message, metrics = terminal.metrics;
    assert.equal(message.id, 1);
    if (fault === 'none') {
      keys(message, ['type', 'id', 'operation', 'bytes']); assert.equal(message.type, 'result'); assert.equal(message.operation, operation);
      const actual = decodeBytes(new Uint8Array(message.bytes)); validateReport(actual.value);
      assert.deepEqual(actual.value, report); bytesEqual(actual.bytes, jsonBytes(report), 'shipped Worker exact native fixture output');
      assert.equal(metrics.runtime_blob_imports, 1); assert.equal(metrics.wasm_instantiations, 1);
      assert.equal(metrics.runtime_snapshot_exact, true); assert.equal(metrics.wasm_snapshot_exact, true);
      assert.equal(metrics.native_calls, operation === 'compare' ? 2 : 1);
      assert.deepEqual(metrics.fetches, { '/api-policy/program.wasm': 1, '/api-policy/wasm_exec.js': 1 });
      return { name: 'shipped-worker:' + operation, status: 'PASS', observation: 'Node-adapted transport running shipped Worker and actual Go/Wasm', elapsed_ms: performance.now() - started,
        ...metrics, output_bytes: actual.bytes.length, output_sha256: hash(actual.bytes), complete_native_fixture_exact: true, diagnostic_bytes: logs };
    }
    keys(message, ['type', 'id', 'incomplete', 'message']); assert.equal(message.type, 'failure');
    assert.equal(message.incomplete, true); assert.equal(Object.hasOwn(message, 'bytes'), false); assert.equal(Object.hasOwn(message, 'data'), false);
    assert.equal(metrics.native_calls, 0, 'no policy call after failed readiness');
    if (fault === 'native-binding') {
      assert.match(message.message, /Complete native profile bindings/); assert.equal(metrics.wasm_instantiations, 1); assert.equal(metrics.runtime_blob_imports, 1);
    } else if (fault === 'path') {
      assert.match(message.message, /Unexpected runtime asset path/); assert.deepEqual(metrics.fetches, {}); assert.equal(metrics.wasm_instantiations, 0);
    } else {
      assert.match(message.message, /Runtime asset hash mismatch/); assert.equal(metrics.runtime_blob_imports, 0); assert.equal(metrics.wasm_instantiations, 0);
    }
    return { name: 'shipped-worker:' + fault, status: 'PASS', observation: 'Node-adapted shipped loader refusal, not semantic input denial or Chrome', no_data: true,
      returned_error: message.message, elapsed_ms: performance.now() - started, ...metrics, diagnostic_bytes: logs };
  } finally { clearTimeout(timer); await worker.terminate(); }
}
async function actualAppWorkerHashControl(options, manifest) {
  const started = performance.now(), appRaw = fs.readFileSync(path.join(options.site, 'api-policy.js'));
  const elements = new Map(), timers = new Set(), fetches = {}; let createdWorkers = 0, resolveStatus;
  const observed = new Promise(resolve => { resolveStatus = resolve; });
  const element = id => {
    if (!elements.has(id)) elements.set(id, { value: '', hidden: false, disabled: id === 'save', dataset: {}, listeners: {},
      addEventListener(name, handler) { this.listeners[name] = handler; }, replaceChildren() {},
      set textContent(value) { this._text = String(value); if (/Worker source hash mismatch/.test(this._text)) resolveStatus(this._text); }, get textContent() { return this._text || ''; } });
    return elements.get(id);
  };
  class URLAdapter extends URL {}
  URLAdapter.createObjectURL = () => { throw new Error('Worker bytes must be refused before executable Blob creation'); };
  URLAdapter.revokeObjectURL = () => {};
  const context = { URL: URLAdapter, location: new URL('https://api-policy-package.test/api-policy.html'), TextEncoder, TextDecoder, ArrayBuffer, Uint8Array, AbortController, Blob, WebAssembly, crypto: webcrypto,
    Worker: class { constructor() { createdWorkers++; throw new Error('corrupted Worker must never be constructed'); } },
    window: { addEventListener() {} }, document: { getElementById: element },
    setTimeout(fn, delay) { const value = setTimeout(fn, delay); timers.add(value); return value; }, clearTimeout(value) { clearTimeout(value); timers.delete(value); },
    async fetch(url, config) {
      assert.equal(config.cache, 'no-store'); assert.equal(config.credentials, 'omit'); assert.equal(config.redirect, 'error');
      const parsed = new URL(url); fetches[parsed.pathname] = (fetches[parsed.pathname] || 0) + 1;
      let raw;
      if (parsed.pathname === '/api-policy/manifest.json') raw = Buffer.from(JSON.stringify(manifest));
      else { assert.equal(parsed.pathname, '/api-policy-worker.js'); raw = Buffer.from(fs.readFileSync(path.join(options.site, 'api-policy-worker.js'))); raw[0] ^= 1; }
      let done = false; return { ok: true, body: { getReader() { return { async read() { if (done) return { done: true }; done = true; return { done: false, value: new Uint8Array(raw) }; }, releaseLock() {} }; } } };
    } };
  let deadline;
  try {
    vm.runInNewContext(decoder.decode(appRaw), context, { filename: 'shipped-api-policy.js' });
    element('compare').listeners.click();
    const text = await Promise.race([observed, new Promise((resolve, reject) => { deadline = setTimeout(() => reject(new Error('INCOMPLETE: app hash-control observation timed out')), options['timeout-ms']); })]);
    assert.match(text, /Worker source hash mismatch/); assert.equal(createdWorkers, 0); assert.equal(element('save').disabled, true);
    assert.equal(element('result').hidden, true); assert.equal(element('raw-report').textContent, '');
    assert.deepEqual(fetches, { '/api-policy/manifest.json': 1, '/api-policy-worker.js': 1 });
    return { name: 'shipped-app:worker-hash', status: 'PASS', observation: 'Actual shipped app source with DOM/fetch adapter; corrupted Worker refused before execution',
      elapsed_ms: performance.now() - started, created_workers: 0, no_result_or_download: true, fetches, app_sha256: hash(appRaw), returned_error: text };
  } finally { clearTimeout(deadline); for (const timer of timers) clearTimeout(timer); }
}
async function directRun(options, fixtures, result) {
  let worker, stepTimer, totalTimer, settled = false;
  await new Promise(resolve => {
    const finish = async (status, detail) => {
      if (settled) return; settled = true; clearTimeout(stepTimer); clearTimeout(totalTimer);
      result.status = status; Object.assign(result, detail);
      if (worker) {
        result.parent_termination_requested = true;
        try { result.worker_termination_exit = await worker.terminate(); }
        catch (error) { result.worker_termination_error = String(error); result.status = 'INCOMPLETE_RUNTIME'; }
      }
      resolve();
    };
    const arm = name => {
      clearTimeout(stepTimer); result.active_step = name;
      stepTimer = setTimeout(() => finish('INCOMPLETE_TIMEOUT', { timeout_kind: 'step', timed_out_step: name, semantic_input_refusal: false }), options['timeout-ms']);
    };
    try { worker = new Worker(new URL(import.meta.url), { workerData: { kind: 'direct', options, fixtures: { fixture: fixtures.fixture, expected: fixtures.expected } }, stdout: true, stderr: true }); }
    catch (error) { finish('INCOMPLETE_INITIALIZATION', { ...errorRecord(error), semantic_input_refusal: false }); return; }
    const logs = { stdout: 0, stderr: 0 };
    for (const stream of ['stdout', 'stderr']) worker[stream].on('data', raw => {
      logs[stream] += raw.length; result.worker_log_bytes = { ...logs };
      if (logs[stream] > 65536) finish('INCOMPLETE_RUNTIME', { error: 'bounded worker diagnostic stream exceeded', semantic_input_refusal: false });
    });
    worker.on('message', message => {
      if (settled) return;
      if (message.type === 'step-start') arm(message.name);
      else if (message.type === 'step') result.steps.push(message.record);
      else if (message.type === 'ready') {
        result.actual_info = message.info; result.startup_elapsed_ms = message.startup_elapsed_ms;
        result.info_utf8_bytes = message.info_utf8_bytes; result.info_sha256 = message.info_sha256; result.info_utf8_byte_roundtrip_exact = message.info_utf8_byte_roundtrip_exact;
      } else if (message.type === 'done') finish('DIRECT_FINITE_ACTUAL_WASM_PASS', { finite_results: message });
      else if (message.type === 'failure') finish(message.phase === 'initialization' ? 'INCOMPLETE_INITIALIZATION' : 'FAIL_DIRECT', { ...message, semantic_input_refusal: false });
      else finish('INCOMPLETE_RUNTIME', { error: 'unknown direct worker message', semantic_input_refusal: false });
    });
    worker.on('error', error => finish('INCOMPLETE_RUNTIME', { ...errorRecord(error), semantic_input_refusal: false }));
    worker.on('exit', code => { if (!settled) finish('INCOMPLETE_RUNTIME', { worker_unexpected_exit: code, semantic_input_refusal: false }); });
    arm('worker-initialization');
    totalTimer = setTimeout(() => finish('INCOMPLETE_TIMEOUT', { timeout_kind: 'total', semantic_input_refusal: false }), options['total-timeout-ms']);
  });
}
async function main() {
  assert.equal(Number(process.versions.node.split('.')[0]), 22, 'Node22 required');
  const options = parseArguments(process.argv.slice(2)), fixtures = readPackage(options);
  assert.ok(!path.resolve(options.out).startsWith(options.site + path.sep), 'smoke receipt must remain outside the tested site');
  const sourceRefs = fixtures.watch.map(filename => { const raw = fs.readFileSync(filename); return { path: filename, bytes: raw.length, sha256: hash(raw) }; });
  const fd = fs.openSync(options.out, 'wx', 0o600), started = performance.now();
  const result = { schema: 'gooo/api-policy-browser-smoke/v1', status: 'INCOMPLETE', started_at_utc: new Date().toISOString(), node: process.version,
    source_refs: sourceRefs, checkout_before: fixtures.checkout, expected_original_generation: ORIGINAL, expected_revision_generation: REVISION,
    fixture_sha256: fixtures.fixture_sha256, fixture_bytes: fixtures.fixture_bytes, timeout_ms: options['timeout-ms'], total_timeout_ms: options['total-timeout-ms'],
    public_manifest_bound: true, fresh_native_fixture_receipt_verified: true, steps: [], shipped_worker_observations: [], shipped_worker_loader_checks: {},
    limits: [fixtures.binding_scope,
      'Finite generated-native/Wasm parity and literal controls. No independent CEL evaluator, publisher authentication, production-safety or adoption claim.',
      'Shipped Worker/app execute under explicitly adapted Node DOM/fetch/Blob/importScripts transport; these observations are separate from actual Chrome or file-picker acceptance.',
      'Timeout/runtime exit/OOM is incomplete validation and never a semantic input refusal. Large payloads are asserted in memory and recorded only by length/hash/completeness.',
      'Native CLI raw stdout includes one final LF. Its six receipt identities and separately specified canonical-minus-one-LF bytes were checked; no arbitrary trimming.'] };
  try {
    await directRun(options, fixtures, result);
    result.completed_steps = result.steps.length; result.direct_actual_Wasm_steps = result.steps.length;
    result.explicit_no_data_refusals = result.steps.filter(row => row.no_data === true && row.status === 'PASS').length;
    result.saved_tamper_no_data_refusals = result.steps.filter(row => row.kind === 'saved-tamper-no-data-refusal' && row.status === 'PASS').length;
    result.caller_own_override_controls = result.steps.filter(row => row.kind === 'caller-own-transport-regression' && row.status === 'PASS').length;
    result.genuine_write_byte_authority_controls = result.steps.filter(row => row.name.startsWith('caller-owned-byte-authority:') && row.status === 'PASS').length;
    if (result.status === 'DIRECT_FINITE_ACTUAL_WASM_PASS') {
      assert.equal(result.completed_steps, 426); assert.equal(result.explicit_no_data_refusals, 373); assert.equal(result.saved_tamper_no_data_refusals, 283);
      assert.equal(result.caller_own_override_controls, 15); assert.equal(result.genuine_write_byte_authority_controls, 3);
      assert.ok(result.steps.every(row => row.status === 'PASS')); assert.deepEqual(result.actual_info, fixtures.manifest.expected_info);
      const bounded = () => {
        const remaining = Math.floor(options['total-timeout-ms'] - (performance.now() - started));
        assert.ok(remaining > 0, 'INCOMPLETE: package operational total timeout, not semantic refusal');
        return { ...options, 'timeout-ms': Math.min(options['timeout-ms'], remaining) };
      };
      for (const operation of ['compare', 'replay']) {
        result.shipped_worker_observations.push(await loaderCase(bounded(), fixtures.manifest, fixtures, 'none', operation));
        result.shipped_worker_loader_checks[operation] = true;
      }
      for (const [fault, field] of [['runtime-hash', 'runtime_hash_mismatch'], ['wasm-hash', 'wasm_hash_mismatch'], ['native-binding', 'native_binding_mismatch'], ['path', 'closed_path_refusal']]) {
        result.shipped_worker_observations.push(await loaderCase(bounded(), fixtures.manifest, fixtures, fault, 'compare'));
        result.shipped_worker_loader_checks[field] = true;
      }
      result.shipped_worker_observations.push(await actualAppWorkerHashControl(bounded(), fixtures.manifest));
      result.shipped_worker_loader_checks.worker_hash_mismatch = true;
      result.status = 'FINITE_ACTUAL_API_POLICY_BROWSER_PACKAGE_PASS';
    }
  } catch (error) {
    result.status = /INCOMPLETE/.test(String(error.message)) ? 'INCOMPLETE_RUNTIME_OR_TIMEOUT' : 'FAIL'; Object.assign(result, errorRecord(error));
  } finally {
    try {
      result.source_after = sourceRefs.map(record => { const raw = fs.readFileSync(record.path); return { path: record.path, bytes: raw.length, sha256: hash(raw) }; });
      result.source_before_after_equal = JSON.stringify(result.source_after) === JSON.stringify(sourceRefs);
      if (!result.source_before_after_equal) result.status = 'FAIL_SOURCE_CHANGED';
      result.checkout_after = currentCheckout(fixtures.repoRoot);
      result.checkout_before_after_equal = JSON.stringify(result.checkout_after) === JSON.stringify(fixtures.checkout);
      if (!result.checkout_before_after_equal) result.status = 'FAIL_CHECKOUT_CHANGED';
    } catch (error) { result.source_before_after_equal = false; result.status = 'FAIL_SOURCE_CHANGED'; Object.assign(result, errorRecord(error)); }
    result.completed_at_utc = new Date().toISOString(); result.elapsed_ms = performance.now() - started;
    try { fs.writeFileSync(fd, JSON.stringify(result, null, 2) + '\n'); } finally { fs.closeSync(fd); }
    console.log(JSON.stringify({ schema: result.schema, status: result.status, out: options.out, direct_actual_Wasm_steps: result.direct_actual_Wasm_steps,
      saved_tamper_no_data_refusals: result.saved_tamper_no_data_refusals, explicit_no_data_refusals: result.explicit_no_data_refusals,
      caller_own_override_controls: result.caller_own_override_controls, genuine_write_byte_authority_controls: result.genuine_write_byte_authority_controls,
      public_manifest_bound: result.public_manifest_bound, fresh_native_fixture_receipt_verified: result.fresh_native_fixture_receipt_verified,
      shipped_worker_loader_checks: result.shipped_worker_loader_checks, source_before_after_equal: result.source_before_after_equal }));
    process.exitCode = result.status === 'FINITE_ACTUAL_API_POLICY_BROWSER_PACKAGE_PASS' ? 0 : 1;
  }
}
if (isMainThread) main().catch(error => { console.error(String(error.stack || error)); process.exitCode = 1; });
else if (workerData.kind === 'shipped-loader') await shippedLoaderWorker();
else await workerSuite();
