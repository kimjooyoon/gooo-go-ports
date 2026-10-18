// Transport and result contracts only. Policy grammar and decisions belong to
// the unchanged checked Gooo activities in the compiled service-policy module.
((root, factory) => {
  const api = factory();
  if (typeof module === 'object' && module.exports) module.exports = api;
  else root.GoooServicePolicyProtocol = api;
})(globalThis, () => {
  'use strict';
  const MAX_UNITS = 4096;
  const MAX_OBSERVATION_BYTES = 1048576;
  const MODE_FIELDS = Object.freeze({ match: Object.freeze(['pattern', 'req_type', 'req_name']),
    impact: Object.freeze(['before', 'after', 'req_type', 'req_name']), refine: Object.freeze(['before', 'after']) });
  const DECISION_FIELDS = Object.freeze({ match: Object.freeze(['domain_status', 'invalid_field', 'matched']),
    impact: Object.freeze(['domain_status', 'invalid_field', 'before_match', 'after_match', 'change']),
    refine: Object.freeze(['domain_status', 'invalid_field', 'status', 'rule_id', 'supported_subset', 'has_witness',
      'witness_type', 'witness_name', 'before_match', 'after_match']) });
  const SUBSET_RULES = Object.freeze(['CANDIDATE_EMPTY', 'IDENTICAL_PATTERNS', 'OLD_GLOBAL', 'OLD_TYPED_ALL',
    'EXACT_CANDIDATE_ACCEPTED_BY_OLD', 'PREFIX_HIERARCHY', 'SUFFIX_HIERARCHY']);
  const COMPILER_REVISION = '83928fa4538245abe33b08549da47f8cbdc50ed2';
  const GENERATION_SHA256 = 'sha256:40ae704abbf2cae1974d2eec51cec2564c8d72dd44536b31c9c4f55f71fc0f85';
  const own = (value, key) => Object.prototype.hasOwnProperty.call(value, key);
  const sha = value => typeof value === 'string' && /^[a-f0-9]{64}$/.test(value);
  const nativeSHA = value => typeof value === 'string' && /^sha256:[a-f0-9]{64}$/.test(value);
  function object(value, fields, label) {
    if (!value || typeof value !== 'object' || Array.isArray(value) ||
        JSON.stringify(Object.keys(value).sort()) !== JSON.stringify([...fields].sort())) throw new Error(`Invalid ${label} fields.`);
  }
  function canonical(value) {
    if (value === null || typeof value === 'string' || typeof value === 'boolean' ||
        (typeof value === 'number' && Number.isFinite(value))) return value;
    if (Array.isArray(value)) return Object.freeze(value.map(canonical));
    if (value && typeof value === 'object') {
      const result = Object.create(null);
      for (const key of Object.keys(value).sort()) result[key] = canonical(value[key]);
      return Object.freeze(result);
    }
    throw new Error('The result must contain only JSON values.');
  }
  const same = (a, b) => JSON.stringify(canonical(a)) === JSON.stringify(canonical(b));
  function modeFields(mode) {
    if (typeof mode !== 'string' || !own(MODE_FIELDS, mode)) throw new Error('Unsupported policy mode.');
    return MODE_FIELDS[mode];
  }
  function units(text, limit = MAX_UNITS) {
    if (typeof text !== 'string' || text.length > limit) throw new Error(`Each input must be text of at most ${limit} UTF-16 code units.`);
    const values = [];
    for (let index = 0; index < text.length; index++) {
      const value = text.charCodeAt(index);
      if (value >= 0xd800 && value <= 0xdbff) {
        const next = text.charCodeAt(index + 1);
        if (!(next >= 0xdc00 && next <= 0xdfff)) throw new Error('Unpaired UTF-16 surrogate: the input was not changed or evaluated.');
        values.push(value, next); index++;
      } else {
        if (value >= 0xdc00 && value <= 0xdfff) throw new Error('Unpaired UTF-16 surrogate: the input was not changed or evaluated.');
        values.push(value);
      }
    }
    return Object.freeze(values);
  }
  function escaped(text) {
    return '"' + units(text).map(value => '\\u' + value.toString(16).toUpperCase().padStart(4, '0')).join('') + '"';
  }
  function validateFacts(mode, facts) {
    const fields = modeFields(mode); object(facts, fields, 'policy facts');
    for (const field of fields) units(facts[field]);
    return canonical(facts);
  }
  function packFacts(mode, facts) {
    const checked = validateFacts(mode, facts);
    return Object.freeze(modeFields(mode).map(field => units(checked[field])));
  }
  function requestId(id) {
    if (!Number.isSafeInteger(id) || id < 1) throw new Error('Invalid request ID.');
  }
  function validateRequest(message) {
    object(message, ['type', 'id', 'mode', 'facts'], 'request');
    if (message.type !== 'evaluate') throw new Error('Unsupported request type.');
    requestId(message.id);
    return Object.freeze({ id: message.id, mode: message.mode, facts: validateFacts(message.mode, message.facts) });
  }
  function validateCancel(message) {
    object(message, ['type', 'id'], 'cancellation');
    if (message.type !== 'cancel') throw new Error('Unsupported cancellation type.');
    requestId(message.id); return message.id;
  }
  function parseExactJSON(raw, maxLength = 262144) {
    if (typeof raw !== 'string' || raw.length > maxLength) throw new Error('The compiled reply is unreadable.');
    let value;
    try { value = JSON.parse(raw); } catch { throw new Error('The compiled reply is invalid JSON.'); }
    // JSON.parse has checked the syntax. Track only object keys to refuse
    // duplicate keys, including equivalent escaped spellings, before use.
    const stack = [];
    for (const match of raw.matchAll(/"(?:\\[\s\S]|[^"\\])*"|[{}\[\]:,]/g)) {
      const token = match[0], top = stack[stack.length - 1];
      if (token === '{') stack.push({ object: true, key: true, seen: new Set() });
      else if (token === '[') stack.push({ object: false });
      else if (token === '}' || token === ']') stack.pop();
      else if (token === ',' && top?.object) top.key = true;
      else if (token === ':' && top?.object) top.key = false;
      else if (token[0] === '"' && top?.object && top.key) {
        const key = JSON.parse(token);
        if (top.seen.has(key)) throw new Error('Duplicate JSON field.');
        top.seen.add(key);
      }
    }
    return value;
  }
  function validateDecision(mode, decision) {
    const fields = modeFields(mode); object(decision, DECISION_FIELDS[mode], 'compiled decision');
    if (!['VALID_INPUT', 'INVALID_INPUT'].includes(decision.domain_status) || typeof decision.invalid_field !== 'string' ||
        (decision.domain_status === 'VALID_INPUT' ? decision.invalid_field !== '' : !fields.includes(decision.invalid_field))) {
      throw new Error('Invalid input-domain decision.');
    }
    const invalid = decision.domain_status === 'INVALID_INPUT';
    if (mode === 'match') {
      if (typeof decision.matched !== 'boolean' || invalid && decision.matched) throw new Error('Invalid matcher decision or refusal sentinel.');
    } else {
      if (typeof decision.before_match !== 'boolean' || typeof decision.after_match !== 'boolean') throw new Error('Invalid match observations.');
      if (mode === 'impact') {
        const expected = invalid ? 'INVALID_INPUT' : decision.before_match ?
          (decision.after_match ? 'UNCHANGED_ALLOW' : 'REVOKED') : (decision.after_match ? 'WIDENED' : 'UNCHANGED_DENY');
        if (decision.change !== expected || invalid && (decision.before_match || decision.after_match)) throw new Error('Invalid impact decision or refusal sentinel.');
      } else {
        if (typeof decision.status !== 'string' || typeof decision.rule_id !== 'string' || typeof decision.supported_subset !== 'boolean' ||
            typeof decision.has_witness !== 'boolean' || typeof decision.witness_type !== 'string' || typeof decision.witness_name !== 'string') {
          throw new Error('Invalid refinement decision field type.');
        }
        const noWitness = !decision.has_witness && decision.witness_type === '' && decision.witness_name === '' && !decision.before_match && !decision.after_match;
        let coherent = false;
        if (invalid) coherent = decision.status === 'INVALID_INPUT' && decision.rule_id === '' && !decision.supported_subset && noWitness;
        else if (decision.status === 'SUPPORTED_SUBSET') coherent = decision.supported_subset && SUBSET_RULES.includes(decision.rule_id) && noWitness;
        else if (decision.status === 'UNPROVEN') coherent = !decision.supported_subset && decision.rule_id === '' && noWitness;
        else if (decision.status === 'WIDENING_WITNESS') {
          // Output-domain and sentinel checks only; Gooo already computed and
          // checked this witness. JavaScript does not rematch either pattern.
          const represented = value => value.length > 0 && value.length <= 128 && /^[\x20-\x7e]+$/.test(value);
          coherent = !decision.supported_subset && decision.has_witness && decision.rule_id === 'CHECKED_CANDIDATE_REQUEST' &&
            !decision.before_match && decision.after_match && represented(decision.witness_type) && represented(decision.witness_name);
        }
        if (!coherent) throw new Error('Invalid refinement status, witness or refusal sentinel.');
      }
    }
    return canonical(decision);
  }
  // These rosters describe the unchanged native module and the published build.
  const ASSET_NAMES = Object.freeze([
  "adapter_generated.go",
  "apply/evaluate.go",
  "apply/store.go",
  "apply/store_test.go",
  "apply/types.go",
  "bindings.go",
  "bridge.go",
  "checked-cases.json",
  "checked_impact.gooo",
  "checked_matcher.gooo",
  "checked_refinement.gooo",
  "cmd/apply-demo/main.go",
  "cmd/evaluate/main.go",
  "compiler.json",
  "conditional-apply-workspace.json",
  "conditional_apply.gooo",
  "domain-cases.json",
  "domain-reference.json",
  "domain.go",
  "domain.gooo",
  "evidence/extraction.json",
  "evidence/observations.json",
  "evidence/oracle-command.go.txt",
  "evidence/oracle-extracted.go.txt",
  "evidence/oracle-go.mod.txt",
  "evidence/repository-adaptation.json",
  "generate.py",
  "go.mod",
  "grant-update-cases.json",
  "grant_update.go",
  "grant_update_test.go",
  "impact-cases.json",
  "impact-raw-workspace.json",
  "impact-workspace.json",
  "impact.gooo",
  "input.go",
  "internal/checked-impact/generated.go",
  "internal/checked-matcher/generated.go",
  "internal/checked-refinement/generated.go",
  "internal/conditional-apply/generated.go",
  "internal/impact/generated.go",
  "internal/matcher/generated.go",
  "internal/refinement/generated.go",
  "matcher-cases.json",
  "matcher-raw-workspace.json",
  "matcher-workspace.json",
  "matcher.gooo",
  "model.go",
  "receipts/checked-impact-body-codegen.json",
  "receipts/checked-impact-lowered.gooo",
  "receipts/checked-impact-program.json",
  "receipts/checked-impact-seed-cases.json",
  "receipts/checked-matcher-body-codegen.json",
  "receipts/checked-matcher-lowered.gooo",
  "receipts/checked-matcher-program.json",
  "receipts/checked-matcher-seed-cases.json",
  "receipts/checked-refinement-body-codegen.json",
  "receipts/checked-refinement-lowered.gooo",
  "receipts/checked-refinement-program.json",
  "receipts/checked-refinement-seed-cases.json",
  "receipts/conditional-apply-body-codegen.json",
  "receipts/conditional-apply-lowered.gooo",
  "receipts/conditional-apply-program.json",
  "receipts/conditional-apply-seed-cases.json",
  "receipts/impact-body-codegen.json",
  "receipts/impact-lowered.gooo",
  "receipts/impact-program.json",
  "receipts/impact-seed-cases.json",
  "receipts/matcher-body-codegen.json",
  "receipts/matcher-lowered.gooo",
  "receipts/matcher-program.json",
  "receipts/matcher-seed-cases.json",
  "receipts/refinement-body-codegen.json",
  "receipts/refinement-lowered.gooo",
  "receipts/refinement-program.json",
  "receipts/refinement-seed-cases.json",
  "refinement-cases.json",
  "refinement-raw-workspace.json",
  "refinement-workspace.json",
  "refinement.gooo",
  "source-pin.json",
  "upstream/LICENSE",
  "upstream/api/network.go.txt",
  "upstream/api/tar.go.txt",
  "upstream/api/tar_test.go.txt",
  "verification_checks.py",
  "verify.py",
  "verify_grant_update.py",
  "verify_grant_update_test.py"
]);
  const FILE_NAMES = Object.freeze([
  "program.wasm",
  "wasm_exec.js",
  "source/matcher.gooo",
  "source/impact.gooo",
  "source/refinement.gooo",
  "source/domain.gooo",
  "source/checked_matcher.gooo",
  "source/checked_impact.gooo",
  "source/checked_refinement.gooo",
  "source/model.go",
  "source/domain.go",
  "source/input.go",
  "source/bridge.go",
  "source/bindings.go",
  "source/adapter_generated.go",
  "source/internal/checked-matcher/generated.go",
  "source/internal/checked-impact/generated.go",
  "source/internal/checked-refinement/generated.go",
  "source/matcher-workspace.json",
  "source/impact-workspace.json",
  "source/refinement-workspace.json",
  "source/generation.json",
  "source/compiler.json",
  "source/source-pin.json",
  "source/go.mod",
  "source/upstream/LICENSE",
  "source/upstream/api/tar.go.txt",
  "source/upstream/api/network.go.txt",
  "source/upstream/api/tar_test.go.txt",
  "source/LICENSE-APACHE-2.0",
  "source/NOTICE.md",
  "source/cmd/browser/bridge.go",
  "source/cmd/browser/main_js.go",
  "source/cmd/browser/main_native.go"
]);
  function validateBindings(bindings) {
    object(bindings, ['compiler_source_sha', 'generation_sha256', 'generation_origin', 'assets'], 'native source bindings');
    if (bindings.compiler_source_sha !== COMPILER_REVISION || bindings.generation_sha256 !== GENERATION_SHA256 ||
        bindings.generation_origin !== 'fresh_pinned_generation') throw new Error('Native source identity differs from the pinned module.');
    object(bindings.assets, ASSET_NAMES, 'native asset roster');
    if (Object.values(bindings.assets).some(value => !nativeSHA(value))) throw new Error('Invalid native asset hash.');
    return canonical(bindings);
  }
  function validateMetadata(metadata) {
    object(metadata, ['schema', 'compiler_revision', 'go_version', 'source_revision', 'source_dirty', 'generation_sha256', 'bindings',
      'wasm_sha256', 'wasm_bytes', 'wasm_exec_sha256', 'wasm_exec_bytes', 'files'], 'build manifest');
    if (metadata.schema !== 'gooo/browser-service-policy/v1' || metadata.compiler_revision !== COMPILER_REVISION ||
        metadata.go_version !== 'go1.27.2' || typeof metadata.source_revision !== 'string' || !/^[a-f0-9]{40}$/.test(metadata.source_revision) || typeof metadata.source_dirty !== 'boolean' ||
        metadata.generation_sha256 !== GENERATION_SHA256 || !sha(metadata.wasm_sha256) || !sha(metadata.wasm_exec_sha256)) throw new Error('Unsupported policy build identity.');
    validateBindings(metadata.bindings);
    object(metadata.files, FILE_NAMES, 'published file roster');
    let total = 0;
    for (const [name, file] of Object.entries(metadata.files)) {
      object(file, ['sha256', 'bytes'], 'published file');
      if (!sha(file.sha256) || !Number.isSafeInteger(file.bytes) || file.bytes < 1 || file.bytes > 32 * 1024 * 1024) throw new Error('Invalid published file hash or size.');
      total += file.bytes;
    }
    if (total > 64 * 1024 * 1024 || metadata.files['program.wasm'].sha256 !== metadata.wasm_sha256 ||
        metadata.files['program.wasm'].bytes !== metadata.wasm_bytes || metadata.files['wasm_exec.js'].sha256 !== metadata.wasm_exec_sha256 ||
        metadata.files['wasm_exec.js'].bytes !== metadata.wasm_exec_bytes) throw new Error('Inconsistent program or runtime bindings.');
    if ('sha256:' + metadata.files['source/generation.json'].sha256 !== GENERATION_SHA256) throw new Error('Published generation identity changed.');
    for (const name of ASSET_NAMES) {
      const file = metadata.files['source/' + name];
      if (file && 'sha256:' + file.sha256 !== metadata.bindings.assets[name]) throw new Error('Published source differs from the native asset binding.');
    }
    if ('sha256:' + metadata.files['source/LICENSE-APACHE-2.0'].sha256 !== metadata.bindings.assets['upstream/LICENSE']) throw new Error('Published upstream license differs from the native binding.');
    return canonical(metadata);
  }
  function parseMetadata(raw) { return validateMetadata(parseExactJSON(raw)); }
  function parseBindings(raw, program) {
    const value = parseExactJSON(raw);
    rejectFailure(value);
    const bindings = validateBindings(value), metadata = validateMetadata(program);
    if (!same(bindings, metadata.bindings)) throw new Error('The running module does not match the published native bindings.');
    return bindings;
  }
  function rejectFailure(value) {
    if (value && value.ok === false) {
      object(value, ['ok', 'error'], 'compiled failure');
      if (typeof value.error !== 'string' || !value.error || value.error.length > 240) throw new Error('Invalid compiled failure.');
      throw new Error('The compiled program refused evaluation: ' + value.error);
    }
  }
  function validateProof(mode, proof, suppliedFacts, program) {
    const facts = validateFacts(mode, suppliedFacts), metadata = validateMetadata(program);
    object(proof, ['facts', 'decision', 'bindings'], 'compiled proof');
    if (!same(validateFacts(mode, proof.facts), facts)) throw new Error('The compiled facts differ from the request.');
    validateDecision(mode, proof.decision);
    if (!same(validateBindings(proof.bindings), metadata.bindings)) throw new Error('The compiled proof has different source bindings.');
    return canonical(proof);
  }
  function parseCompiledReply(raw, mode, facts, program) {
    const reply = parseExactJSON(raw); rejectFailure(reply);
    object(reply, ['ok', 'proof'], 'compiled reply');
    if (reply.ok !== true) throw new Error('Invalid compiled success flag.');
    return validateProof(mode, reply.proof, facts, program);
  }
  function validateReady(message) {
    object(message, ['type', 'program'], 'ready message');
    if (message.type !== 'ready') throw new Error('Invalid ready message.');
    return validateMetadata(message.program);
  }
  function validateResult(message, expected) {
    object(message, ['type', 'id', 'mode', 'proof', 'program'], 'result');
    requestId(expected.id); modeFields(expected.mode);
    if (message.type !== 'result' || message.id !== expected.id || message.mode !== expected.mode ||
        !same(validateMetadata(message.program), validateMetadata(expected.program))) throw new Error('Stale or mismatched policy result.');
    return validateProof(expected.mode, message.proof, expected.facts, expected.program);
  }
  function validateError(message, expectedId) {
    object(message, own(message || {}, 'id') ? ['type', 'id', 'message'] : ['type', 'message'], 'error');
    if (message.type !== 'error' || typeof message.message !== 'string' || !message.message || message.message.length > 240) throw new Error('Invalid worker error.');
    if (own(message, 'id')) requestId(message.id);
    if (expectedId !== undefined && message.id !== expectedId) throw new Error('Stale worker error.');
    return message.message;
  }
  function observation(mode, facts, proof, program) {
    const checkedFacts = validateFacts(mode, facts), metadata = validateMetadata(program);
    const checkedProof = validateProof(mode, proof, checkedFacts, metadata);
    return canonical({ schema: 'gooo/service-policy-browser-observation/v1', kind: 'browser-observation', mode,
      input_binding: 'Exact UTF-16 code units; unpaired surrogates refused; no trimming, normalization or replacement.',
      inputs: modeFields(mode).map(name => ({ name, text: checkedFacts[name], utf16_units: units(checkedFacts[name]), escaped: escaped(checkedFacts[name]) })),
      proof: checkedProof, program: metadata,
      scope: 'Unsigned observation from this browser and compiled Gooo program. Not a native execution receipt, saved replay, authorization, policy write, attestation or complete Agent Mesh compatibility claim.' });
  }
  // Stored metadata is an unsigned claim. Check its known v1 structure and
  // internal relationships without relaxing the live runtime's pinned gates.
  function validateStoredBindings(bindings) {
    object(bindings, ['compiler_source_sha', 'generation_sha256', 'generation_origin', 'assets'], 'saved native source bindings');
    if (typeof bindings.compiler_source_sha !== 'string' || !/^[a-f0-9]{40}$/.test(bindings.compiler_source_sha) ||
        !nativeSHA(bindings.generation_sha256) || bindings.generation_origin !== 'fresh_pinned_generation') throw new Error('Invalid saved native source identity.');
    object(bindings.assets, ASSET_NAMES, 'saved native asset roster');
    if (Object.values(bindings.assets).some(value => !nativeSHA(value))) throw new Error('Invalid saved native asset hash.');
    return canonical(bindings);
  }
  function validateStoredMetadata(metadata) {
    object(metadata, ['schema', 'compiler_revision', 'go_version', 'source_revision', 'source_dirty', 'generation_sha256', 'bindings',
      'wasm_sha256', 'wasm_bytes', 'wasm_exec_sha256', 'wasm_exec_bytes', 'files'], 'saved build manifest');
    const bindings = validateStoredBindings(metadata.bindings);
    if (metadata.schema !== 'gooo/browser-service-policy/v1' || metadata.compiler_revision !== bindings.compiler_source_sha ||
        metadata.generation_sha256 !== bindings.generation_sha256 || typeof metadata.go_version !== 'string' ||
        !/^go[1-9][0-9]{0,2}\.(?:0|[1-9][0-9]{0,2})\.(?:0|[1-9][0-9]{0,2})$/.test(metadata.go_version) ||
        typeof metadata.source_revision !== 'string' || !/^[a-f0-9]{40}$/.test(metadata.source_revision) ||
        typeof metadata.source_dirty !== 'boolean' || !sha(metadata.wasm_sha256) || !sha(metadata.wasm_exec_sha256)) throw new Error('Unsupported saved policy build identity.');
    object(metadata.files, FILE_NAMES, 'saved file roster');
    let total = 0;
    for (const file of Object.values(metadata.files)) {
      object(file, ['sha256', 'bytes'], 'saved file');
      if (!sha(file.sha256) || !Number.isSafeInteger(file.bytes) || file.bytes < 1 || file.bytes > 32 * 1024 * 1024) throw new Error('Invalid saved file hash or size.');
      total += file.bytes;
    }
    if (total > 64 * 1024 * 1024 || metadata.files['program.wasm'].sha256 !== metadata.wasm_sha256 ||
        metadata.files['program.wasm'].bytes !== metadata.wasm_bytes || metadata.files['wasm_exec.js'].sha256 !== metadata.wasm_exec_sha256 ||
        metadata.files['wasm_exec.js'].bytes !== metadata.wasm_exec_bytes) throw new Error('Inconsistent saved program or runtime bindings.');
    if ('sha256:' + metadata.files['source/generation.json'].sha256 !== metadata.generation_sha256) throw new Error('Inconsistent saved generation identity.');
    for (const name of ASSET_NAMES) {
      const file = metadata.files['source/' + name];
      if (file && 'sha256:' + file.sha256 !== bindings.assets[name]) throw new Error('Saved source differs from its native asset binding.');
    }
    if ('sha256:' + metadata.files['source/LICENSE-APACHE-2.0'].sha256 !== bindings.assets['upstream/LICENSE']) throw new Error('Saved upstream license differs from its native binding.');
    return canonical(metadata);
  }
  function validateObservation(value) {
    object(value, ['schema', 'kind', 'mode', 'input_binding', 'inputs', 'proof', 'program', 'scope'], 'saved observation');
    if (value.schema !== 'gooo/service-policy-browser-observation/v1' || value.kind !== 'browser-observation' ||
        value.input_binding !== 'Exact UTF-16 code units; unpaired surrogates refused; no trimming, normalization or replacement.' ||
        value.scope !== 'Unsigned observation from this browser and compiled Gooo program. Not a native execution receipt, saved replay, authorization, policy write, attestation or complete Agent Mesh compatibility claim.') throw new Error('Unsupported saved observation format.');
    const fields = modeFields(value.mode);
    if (!Array.isArray(value.inputs) || value.inputs.length !== fields.length) throw new Error('Invalid saved input roster.');
    const facts = Object.create(null);
    fields.forEach((name, index) => {
      const input = value.inputs[index];
      object(input, ['name', 'text', 'utf16_units', 'escaped'], 'saved input');
      if (input.name !== name) throw new Error('Saved inputs must use the mode field order.');
      const encoded = units(input.text);
      if (!Array.isArray(input.utf16_units) || input.utf16_units.length !== encoded.length ||
          input.utf16_units.some(unit => !Number.isSafeInteger(unit) || unit < 0 || unit > 65535 || Object.is(unit, -0)) ||
          !same(input.utf16_units, encoded) || input.escaped !== escaped(input.text)) throw new Error('Saved input representations disagree.');
      facts[name] = input.text;
    });
    object(value.proof, ['facts', 'decision', 'bindings'], 'saved proof');
    if (!same(validateFacts(value.mode, value.proof.facts), facts)) throw new Error('Saved proof facts differ from its inputs.');
    validateDecision(value.mode, value.proof.decision);
    const program = validateStoredMetadata(value.program);
    if (!same(validateStoredBindings(value.proof.bindings), program.bindings)) throw new Error('Saved proof has different source bindings.');
    return canonical(value);
  }
  function parseObservation(raw) {
    if (typeof raw !== 'string' || raw.length === 0 || raw.length > MAX_OBSERVATION_BYTES) throw new Error('The saved observation exceeds its text limit or is unreadable.');
    // Check depth and numeric spellings before JSON.parse can round a token.
    // Strings are consumed whole, so their digits and braces are not tokens.
    let depth = 0;
    for (const match of raw.matchAll(/"(?:\\[\s\S]|[^"\\])*"|[{}\[\]]|-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?/g)) {
      const token = match[0];
      if (token === '{' || token === '[') {
        if (++depth > 12) throw new Error('The saved observation is nested too deeply.');
      } else if (token === '}' || token === ']') depth--;
      else if (token[0] !== '"' && (!/^(?:0|[1-9][0-9]*)$/.test(token) || token.length > 16 ||
          token.length === 16 && token > '9007199254740991')) throw new Error('Saved numbers must be canonical nonnegative safe integers.');
    }
    return validateObservation(parseExactJSON(raw, MAX_OBSERVATION_BYTES));
  }
  function compareObservation(saved, mode, facts, proof, currentProgram) {
    const claimed = validateObservation(saved), checkedFacts = validateFacts(mode, facts), program = validateMetadata(currentProgram);
    const current = validateProof(mode, proof, checkedFacts, program);
    if (claimed.mode !== mode || !same(claimed.proof.facts, checkedFacts)) throw new Error('The fresh check must use the saved mode and exact inputs.');
    const differences = DECISION_FIELDS[mode].filter(field => claimed.proof.decision[field] !== current.decision[field])
      .map(field => ({ field, saved: claimed.proof.decision[field], current: current.decision[field] }));
    return canonical({ decision_equal: differences.length === 0, decision_differences: differences,
      program_equal: same(claimed.program, program), bindings_equal: same(claimed.program.bindings, program.bindings),
      executable_equal: ['wasm_sha256', 'wasm_bytes', 'wasm_exec_sha256', 'wasm_exec_bytes'].every(field => claimed.program[field] === program[field]) });
  }
  return Object.freeze({ MAX_UNITS, MAX_OBSERVATION_BYTES, MODE_FIELDS, DECISION_FIELDS, SUBSET_RULES, ASSET_NAMES, FILE_NAMES,
    validateFacts, packFacts, escaped, validateRequest, validateCancel, parseExactJSON, validateDecision,
    validateBindings, validateMetadata, parseMetadata, parseBindings, validateProof, parseCompiledReply,
    validateReady, validateResult, validateError, observation, parseObservation, validateObservation, compareObservation });
});
