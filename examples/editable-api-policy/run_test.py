#!/usr/bin/env python3
"""Transport/comparison/refusal controls; no compiler, network or subprocess."""
import copy
import contextlib
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock

PATH = Path(__file__).resolve().with_name('run.py')
SPEC = importlib.util.spec_from_file_location('editable_policy', PATH)
R = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(R)


class ConsumerControls(unittest.TestCase):
    def setUp(self):
        self.raw = PATH.with_name('demo-inputs.json').read_bytes()
        self.doc = R.decoded(self.raw)
        self.expected = R.decoded(PATH.with_name('demo-expected.json').read_bytes())['rows']

    def test_three_literals_and_actual_comparison(self):
        self.assertEqual(len(R.input_rows(self.raw)), 3)
        rows = [R.compare(r['before'], r['after']) for r in self.expected]
        self.assertEqual([r['changed_count'] for r in rows], [0, 2, 0])
        for actual, literal in zip(rows, self.expected):
            for name in ('status', 'changed_fields'): self.assertEqual(actual[name], literal[name])
        edited = copy.deepcopy(self.expected[2]['after']); edited['reason'] = 'MY_EDITED_REASON'
        self.assertEqual(R.compare(self.expected[2]['before'], edited)['status'], 'REASON_CHANGED')

    def test_missing_unknown_wrong_scalar_and_surrogate(self):
        key = R.ENTRY + '.input0'
        for mode in ('missing', 'extra', 'integer', 'null', 'surrogate', 'over1024'):
            doc = copy.deepcopy(self.doc)
            if mode == 'missing': del doc['inputs'][0][key]
            elif mode == 'extra': doc['inputs'][0]['unknown'] = False
            elif mode == 'integer': doc['inputs'][0][key] = 1
            elif mode == 'null': doc['inputs'][0][key] = None
            else: doc['inputs'][0][R.ENTRY + '.input1'] = '\ud800' if mode == 'surrogate' else 'é' * 513
            # ASCII escaping lets the actual UTF8/surrogate decoder see the input.
            raw = json.dumps(doc).encode()
            with self.subTest(mode=mode), self.assertRaises((RuntimeError, UnicodeError)):
                R.input_rows(raw)

    def test_json_refusals_and_lf_inclusive_limit(self):
        for raw in (b'{"x":1,"\\u0078":2}', b'{"x":NaN}', b'{"x":"\xff"}', self.raw + b'{}'):
            with self.subTest(raw=raw[:30]), self.assertRaises((ValueError, RuntimeError, UnicodeError)):
                R.input_rows(raw)
        exact = self.raw + b' ' * (30000 - len(self.raw))
        self.assertEqual(len(R.input_rows(exact)), 3)
        with self.assertRaises(RuntimeError): R.input_rows(exact + b'\n')

    def test_output_types_and_classifications(self):
        before = self.expected[0]['before']
        for key, bad in [('allowed', 1), ('reason', None), ('access_result', '\udc00')]:
            value = dict(before, **{key: bad})
            with self.subTest(key=key), self.assertRaises((RuntimeError, UnicodeError)): R.compare(before, value)
        value = dict(before); del value['reason']
        with self.assertRaises(RuntimeError): R.decision(value)
        denied = dict(before, allowed=False)
        self.assertEqual(R.compare(denied, before)['status'], 'NEWLY_ALLOWED')
        changed = dict(before, group_matched=False)
        self.assertEqual(R.compare(before, changed)['status'], 'OBSERVATIONS_CHANGED')

    def test_tampered_producer_identity(self):
        rt = {'producer_source_sha': R.H.REV, 'go_tool_selection': 'explicit',
              'go_tool_sha256': 'sha256:tool', 'go_version': 'go version go1.27.3 test/arch'}
        R.producer(rt, rt['go_version'], rt['go_tool_sha256'])
        for key, bad in [('producer_source_sha', 'other'), ('go_tool_selection', 'ambient'),
                         ('go_tool_sha256', 'sha256:other'), ('go_version', 'go version go1.27.1 test/arch')]:
            with self.subTest(key=key), self.assertRaises(RuntimeError):
                R.producer(dict(rt, **{key: bad}), rt['go_version'], rt['go_tool_sha256'])

    def inspection_profile(self):
        lock = {'compiler_revision': R.H.REV, 'compiler_version': '0.6.37-dev', 'go_version': 'go1.27.3',
                'consumer_compiler_profiles': {'source_inspection037': dict(R.SOURCE_INSPECTION037)}}
        return lock, R.compiler_profile(lock, 'source-inspection037')

    def test_explicit_profile_contract_and_producer_are_separate(self):
        lock, profile = self.inspection_profile()
        self.assertEqual(R.compiler_profile(lock)['compiler_revision'], R.H.REV)
        for key in R.SOURCE_INSPECTION037:
            changed = copy.deepcopy(lock); changed['consumer_compiler_profiles']['source_inspection037'][key] = 'other'
            with self.subTest(key=key), self.assertRaises(RuntimeError):
                R.compiler_profile(changed, 'source-inspection037')
        rt = {'producer_source_sha': profile['compiler_revision'], 'go_tool_selection': 'explicit',
              'go_tool_sha256': 'sha256:tool', 'go_version': 'go version go1.27.3 test/arch'}
        R.producer(rt, rt['go_version'], rt['go_tool_sha256'], profile)
        # Both current profiles now select the same205 source. A foreign
        # historical source must still be refused with either profile selection.
        self.assertEqual(profile['compiler_revision'], R.H.REV)
        R.producer(rt, rt['go_version'], rt['go_tool_sha256'])
        for selected in (None, profile):
            with self.subTest(profile=selected), self.assertRaises(RuntimeError):
                R.producer(dict(rt, producer_source_sha=R.SOURCE_INSPECTION036['compiler_revision']),
                           rt['go_version'], rt['go_tool_sha256'], selected)
        self.assertEqual(R.H.REV, lock['compiler_revision'])

    def test_inspection_identity_requires_clean_unreplaced_sdk(self):
        _, profile = self.inspection_profile()
        identity = {'schema': 'gooo/build-identity/v1', 'language': 'gooo',
                    'compiler_source_sha': profile['compiler_revision'], 'vcs_revision': profile['compiler_revision'],
                    'source_status': 'CLEAN_VCS', 'vcs_modified': 'false', 'build_info_observed': True,
                    'version': profile['compiler_version'], 'go_version': profile['go_version'],
                    'native_go_required': profile['go_version'],
                    'decision_runtime': {'module_path': 'github.com/kimjooyoon/gooo-decision-runtime',
                                         'version': profile['decision_runtime_version'], 'replaced': False}}
        R.compiler_identity(identity, profile)
        for key, bad in [('compiler_source_sha', R.SOURCE_INSPECTION036['compiler_revision']), ('vcs_modified', 'true'),
                         ('build_info_observed', False), ('go_version', 'go1.27.1')]:
            with self.subTest(key=key), self.assertRaises(RuntimeError): R.compiler_identity(dict(identity, **{key: bad}), profile)
        for key, bad in [('module_path', 'other'), ('version', 'v0.2.41-experimental'), ('replaced', True)]:
            changed = copy.deepcopy(identity); changed['decision_runtime'][key] = bad
            with self.subTest(sdk_key=key), self.assertRaises(RuntimeError): R.compiler_identity(changed, profile)

    def test_default_refuses_helpers_and_inspection_allows_unused_declaration(self):
        _, profile = self.inspection_profile()
        helper = b'\nactivity UnusedPolicy(Boolean) -> Boolean id "gooo://editable-api-policy/unused/v1" computes `return input0`\n'
        def read(path):
            raw = path.read_bytes()
            return raw + helper if path == PATH.with_name('after.gooo') else raw
        with self.assertRaisesRegex(RuntimeError, 'one activity per selected source'):
            R.context(R.REPO, 'after', read)
        ctx = R.context(R.REPO, 'after', read, profile)
        self.assertIn(R.PACKAGE + ':UnusedPolicy', ctx['definitions'])
        # A small source-contract object, not a fabricated native/PASS receipt.
        entry = {'package_path': R.PACKAGE, 'activity': R.ACTIVITY, 'lowered_name': 'entry'}
        participants = []
        for key, d in ctx['definitions'].items():
            if key.endswith(':UnusedPolicy'): continue
            ref = entry if key == R.ENTRY else {'package_path': d['package'], 'activity': d['name'], 'lowered_name': d['name']}
            participants.append({'activity': ref, 'activity_id': d['id'], 'lowered_activity_id': d['id'],
                                 'source': d['source'], 'source_body_sha256': R.H.digest(d['body'].encode())})
        program = {'schema': 'gooo/workspace-body-program/v1', 'entry': entry, 'activities': [entry],
                   'workspace_image': {'schema': 'gooo/package-runtime-image/v1', 'init_order': [p for p, _ in ctx['packages']],
                       'packages': [{'path': p, 'sources': [{'filename': n, 'source_digest': sha}]} for p, n, sha in ctx['images']],
                       'entry': {'package_path': R.PACKAGE, 'source': ctx['packages'][-1][1], 'activity': R.ACTIVITY,
                                 'inputs': R.INPUT_TYPES, 'output': 'ApiDecision'}},
                   'pure_calls': {'schema': 'gooo/workspace-pure-calls/v1', 'activities': participants, 'sites': [
                       {'caller': R.ENTRY, 'callee': 'app/cel-api-policy:APIAccess', 'surface': 'computes', 'start_byte': 0, 'end_byte': 1}]}}
        R.source_program(program, ctx, profile)
        changed = copy.deepcopy(program); changed['pure_calls']['activities'][0]['source_body_sha256'] = 'sha256:other'
        with self.assertRaises(RuntimeError): R.source_program(changed, ctx, profile)
        changed = copy.deepcopy(program); changed['pure_calls']['activities'][0]['activity_id'] = 'gooo://other'
        with self.assertRaises(RuntimeError): R.source_program(changed, ctx, profile)

    def test_prepared_program_mismatch_is_refused_without_using_output_values(self):
        _, profile = self.inspection_profile()
        program = {'lowered_gooo_source': 'current body', 'pure_calls': {'sites': [{'callee': 'original:Policy'}]}}
        ctx = {'prepared_program': copy.deepcopy(program)}
        R.prepared_binding(program, ctx, profile)
        for changed in ({**program, 'lowered_gooo_source': 'different body'},
                        {**program, 'pure_calls': {'sites': [{'callee': 'other:Policy'}]}},
                        {**program, 'uninspected_metadata': True}):
            with self.subTest(changed=changed), self.assertRaises(RuntimeError): R.prepared_binding(changed, ctx, profile)
        with self.assertRaises(RuntimeError): R.prepared_binding(program, {}, profile)

    def test_refusal_requires_exact_parent_and_failure_kind(self):
        parent = b'original receipt'
        ctx = {'workspace': Path('/synthetic/after.workspace.json'), 'raw': b'current after manifest'}
        refusal = {'schema': 'gooo/workspace-body-execution-receipt/v1', 'decision': 'FAIL_CLOSED',
                   'error': 'saved execution envelope or workspace manifest differs',
                   'replayed_from_sha256': R.H.digest(parent), 'manifest_digest': R.H.digest(ctx['raw']),
                   'manifest': str(ctx['workspace'])}
        R.check_refusal(R.H.go_json(refusal), parent, ctx)
        for key, bad in [('decision', 'OBSERVED'), ('error', 'Go build failed'),
                         ('error', 'saved workspace differs from current sources, dependencies, entry or lowered graph'),
                         ('replayed_from_sha256', R.H.digest(b'other receipt')), ('manifest', '/other'),
                         ('manifest_digest', R.H.digest(b'other manifest'))]:
            with self.subTest(key=key), self.assertRaises(RuntimeError):
                R.check_refusal(R.H.go_json(dict(refusal, **{key: bad})), parent, ctx)
        with self.assertRaises(RuntimeError):
            R.check_refusal(R.H.go_json(dict(refusal, inputs_digest='unexpected early-stage field')), parent, ctx)

    def test_inspection_refusal_requires_exact_structured_manifest_failure(self):
        _, profile = self.inspection_profile()
        parent = b'original receipt'
        ctx = {'workspace': Path('/synthetic/after.workspace.json'), 'raw': b'current after manifest'}
        refusal = {'schema': 'gooo/workspace-body-execution-receipt/v1', 'decision': 'FAIL_CLOSED',
                   'error': 'saved execution envelope or workspace manifest differs',
                   'replayed_from_sha256': R.H.digest(parent), 'manifest_digest': R.H.digest(ctx['raw']),
                   'manifest': str(ctx['workspace']), 'failure': {'code': 'REPLAY_MANIFEST_MISMATCH',
                       'detail': 'saved execution envelope or workspace manifest differs'}}
        R.check_refusal(R.H.go_json(refusal), parent, ctx, profile)
        with self.assertRaises(RuntimeError): R.check_refusal(R.H.go_json(refusal), parent, ctx)
        for failure in ({'code': 'REPLAY_SOURCE_MISMATCH', 'detail': refusal['error']},
                        {'code': 'REPLAY_MANIFEST_MISMATCH', 'detail': 'other stage'},
                        {**refusal['failure'], 'unknown': True}):
            with self.subTest(failure=failure), self.assertRaises(RuntimeError):
                R.check_refusal(R.H.go_json(dict(refusal, failure=failure)), parent, ctx, profile)
        for changed in ({k: v for k, v in refusal.items() if k != 'failure'}, {**refusal, 'unknown': True}):
            with self.assertRaises(RuntimeError): R.check_refusal(R.H.go_json(changed), parent, ctx, profile)

    def test_trace_to_reported_native_stdout_binding(self):
        values = [r['before'] for r in self.expected]
        values[0] = dict(values[0], reason='<&>\u2028\u2029\n"')
        # Independent encoding of the native record field order and Go escapes.
        matrix = [[dict((name, row[name]) for name in R.FIELDS)] for row in values]
        raw = json.dumps(matrix, ensure_ascii=False, separators=(',', ':'))
        for old, new in (('<', '\\u003c'), ('>', '\\u003e'), ('&', '\\u0026'), ('\u2028', '\\u2028'), ('\u2029', '\\u2029')):
            raw = raw.replace(old, new)
        sha = R.H.digest(raw.encode() + b'\n')
        runtime = {'runs': [{'stdout_sha256': sha}, {'stdout_sha256': sha}]}
        R.output_binding(runtime, values)
        broken = copy.deepcopy(runtime); broken['runs'][1]['stdout_sha256'] = R.H.digest(b'other stdout')
        with self.assertRaises(RuntimeError): R.output_binding(broken, values)
        changed = copy.deepcopy(values); changed[0]['allowed'] = False
        with self.assertRaises(RuntimeError): R.output_binding(runtime, changed)


class NamedFactsControls(unittest.TestCase):
    def fact(self):
        return {'group_present': False, 'group_value': 'group-raw',
                'principal_present': True, 'principal_value': 'principal-raw',
                'authenticated': False, 'caller_tenant': 'caller-raw',
                'resource_tenant': 'resource-raw', 'requested_action': 'request-raw',
                'allowed_action': 'allowed-raw'}

    def raw(self, rows):
        return json.dumps(rows, ensure_ascii=True, separators=(',', ':')).encode() + b'\n'

    def test_semantic_names_map_to_each_original_position_without_reordering_rows(self):
        fact = self.fact()
        raw = self.raw([fact, dict(reversed(list(fact.items()))), fact])
        saved = bytes(raw)
        converted = R.named_facts_inputs(raw)
        self.assertEqual(raw, saved)
        self.assertTrue(converted.endswith(b'\n'))
        self.assertEqual(converted, R.named_facts_inputs(raw))
        doc = R.decoded(converted)
        self.assertEqual(set(doc), {'schema', 'inputs'})
        self.assertEqual(doc['schema'], 'gooo/body-composition-inputs/v1')
        expected = {R.ENTRY + '.input0': False, R.ENTRY + '.input1': 'group-raw',
                    R.ENTRY + '.input2': True, R.ENTRY + '.input3': 'principal-raw',
                    R.ENTRY + '.input4': False, R.ENTRY + '.input5': 'caller-raw',
                    R.ENTRY + '.input6': 'resource-raw', R.ENTRY + '.input7': 'request-raw',
                    R.ENTRY + '.input8': 'allowed-raw'}
        self.assertEqual(doc['inputs'], [expected, expected, expected])
        self.assertEqual(R.input_rows(converted), doc['inputs'])

    def test_inactive_and_unicode_text_is_not_filled_trimmed_or_normalized(self):
        fact = self.fact()
        fact.update(principal_present=False, principal_value='  한e\u0301😀\x00<&>\u2028\u2029  ',
                    group_value='', caller_tenant=' A ', resource_tenant='A')
        value = R.input_rows(R.named_facts_inputs(self.raw([fact])))[0]
        self.assertIs(value[R.ENTRY + '.input2'], False)
        self.assertEqual(value[R.ENTRY + '.input3'], fact['principal_value'])
        self.assertEqual(value[R.ENTRY + '.input1'], '')
        self.assertEqual(value[R.ENTRY + '.input5'], ' A ')
        self.assertEqual(value[R.ENTRY + '.input6'], 'A')

    def test_exact_array_row_and_key_shape_including_a_bad_late_row(self):
        fact = self.fact()
        for value in ({'facts': [fact]}, fact, [], [fact] * 4, [None], ['facts']):
            with self.subTest(shape=type(value).__name__), self.assertRaises(RuntimeError):
                R.named_facts_inputs(self.raw(value))
        for mode in ('missing', 'extra'):
            late = dict(fact)
            if mode == 'missing': del late['principal_value']
            else: late['principal'] = 'not-an-alias'
            with self.subTest(mode=mode), self.assertRaises(RuntimeError):
                R.named_facts_inputs(self.raw([fact, fact, late]))

    def test_no_boolean_or_text_type_coercion(self):
        fact = self.fact()
        for name in ('group_present', 'principal_present', 'authenticated'):
            for bad in (0, 1, None, 'false'):
                with self.subTest(name=name, bad=bad), self.assertRaises(RuntimeError):
                    R.named_facts_inputs(self.raw([dict(fact, **{name: bad})]))
        for name in ('group_value', 'principal_value', 'caller_tenant', 'resource_tenant',
                     'requested_action', 'allowed_action'):
            for bad in (0, None, False, [], {}):
                with self.subTest(name=name, bad=bad), self.assertRaises(RuntimeError):
                    R.named_facts_inputs(self.raw([dict(fact, **{name: bad})]))

    def test_text_limit_counts_utf8_bytes_and_refuses_invalid_scalars(self):
        fact = dict(self.fact(), principal_value='é' * 512)
        self.assertEqual(R.input_rows(R.named_facts_inputs(self.raw([fact])))[0][R.ENTRY + '.input3'], fact['principal_value'])
        for text in ('é' * 513, '\ud800', '\udc00'):
            with self.subTest(text_length=len(text)), self.assertRaises((RuntimeError, UnicodeError)):
                R.named_facts_inputs(self.raw([dict(fact, principal_value=text)]))

    def test_strict_raw_json_duplicates_trailing_and_nonfinite_refuse(self):
        raw = self.raw([self.fact()])
        duplicate = raw.replace(b'"group_present":false', b'"group_present":false,"group_\\u0070resent":true')
        for bad in (duplicate, raw + b'{}', raw.replace(b'"group-raw"', b'NaN'),
                    raw.replace(b'"group-raw"', b'"\xff"')):
            with self.subTest(prefix=bad[:40]), self.assertRaises((ValueError, RuntimeError, UnicodeError)):
                R.named_facts_inputs(bad)

    def test_source_and_encoded_native_byte_limits_are_both_enforced(self):
        raw = self.raw([self.fact()])
        exact = raw + b' ' * (30000 - len(raw))
        self.assertEqual(len(R.input_rows(R.named_facts_inputs(exact))), 1)
        with self.assertRaises(RuntimeError): R.named_facts_inputs(exact + b'\n')
        # These six1024-byte facts fit the source budget, but Go-compatible HTML
        # escaping expands the actual native JSON past30000. No partial input is returned.
        fact = self.fact()
        for name in ('group_value', 'principal_value', 'caller_tenant', 'resource_tenant',
                     'requested_action', 'allowed_action'): fact[name] = '&' * 1024
        raw = self.raw([fact])
        self.assertLess(len(raw), 30000)
        with self.assertRaisesRegex(RuntimeError, '30000-byte joint wire limit'):
            R.named_facts_inputs(raw)

    def test_flags_are_mutually_exclusive_and_default_native_path_remains(self):
        base = ['--root', '/root', '--compiler', '/compiler', '--go', '/go', '--out', '/out']
        default = R.arguments(base)
        self.assertIsNone(default.facts)
        self.assertIsNone(default.inputs)
        self.assertEqual(default.compiler_profile, 'default')
        named = R.arguments(base + ['--facts', '/facts'])
        self.assertEqual(named.facts, '/facts')
        self.assertIsNone(named.inputs)
        native = R.arguments(base + ['--inputs', '/inputs'])
        self.assertEqual(native.inputs, '/inputs')
        for flags in (['--facts', '/facts', '--inputs', '/inputs'], ['--facts', ''], ['--facts']):
            with self.subTest(flags=flags), contextlib.redirect_stderr(io.StringIO()), self.assertRaises(SystemExit) as error:
                R.arguments(base + flags)
            self.assertEqual(error.exception.code, 2)


class SourceComparisonControls(unittest.TestCase):
    def index_fixture(self, out):
        contexts = {name: R.context(R.REPO, name, lambda path: path.read_bytes()) for name in ('before', 'after')}
        receipts = {}
        # Linkage-only JSON fixtures, not synthetic successful native receipts.
        # Production calls the index builder only after full inspect_receipt validation.
        for variant in ('before', 'after'):
            initial = json.dumps({'unit_fixture': variant}).encode() + b'\n'
            receipts[variant + '-execute'] = initial
            receipts[variant + '-replay'] = json.dumps({'unit_fixture': variant,
                'replayed_from_sha256': 'sha256:' + hashlib.sha256(initial).hexdigest()}).encode() + b'\n'
        for label, raw in receipts.items():
            (out / label).mkdir(); (out / label / 'stdout').write_bytes(raw)
        expected = R.decoded(PATH.with_name('demo-expected.json').read_bytes())['rows']
        comparisons = [R.compare(row['before'], row['after']) for row in expected]
        return contexts, receipts, comparisons

    def test_raw_file_hash_is_not_the_receipt_json_string_digest(self):
        raw = b'activity Example() // <&>\r\n'
        encoded = json.dumps(raw.decode(), ensure_ascii=False).replace('<', '\\u003c').replace('&', '\\u0026').replace('>', '\\u003e').encode()
        digest = 'sha256:' + hashlib.sha256(encoded).hexdigest()
        ctx = {'source_raw': {'policy.gooo': raw}, 'images': [('example', 'policy.gooo', digest)]}
        result = R.source_metadata(ctx, 'example', 'policy.gooo')
        self.assertEqual(result['raw_sha256'], hashlib.sha256(raw).hexdigest())
        self.assertEqual(result['receipt_json_string_digest'], digest)
        self.assertNotEqual('sha256:' + result['raw_sha256'], digest)
        self.assertEqual(result['bytes'], len(raw))
        ctx['source_raw']['policy.gooo'] = raw + b'\n'
        with self.assertRaisesRegex(RuntimeError, 'validated receipt source image'):
            R.source_metadata(ctx, 'example', 'policy.gooo')

    def test_four_actual_file_refs_parents_manifests_and_complete_comparisons(self):
        with tempfile.TemporaryDirectory() as directory:
            out = Path(directory)
            contexts, receipts, comparisons = self.index_fixture(out)
            artifact = R.source_comparison(R.REPO, out, contexts, receipts, b'native input\n', comparisons, Path.read_bytes)
            self.assertNotIn('status', artifact)  # The display index is not a new PASS authority.
            self.assertEqual(len(artifact['raw_records']), 4)
            for label, raw in receipts.items():
                ref = artifact['raw_records'][label]
                self.assertEqual(ref['relative_path'], label + '/stdout')
                self.assertEqual(ref['bytes'], len(raw))
                self.assertEqual(ref['raw_sha256'], hashlib.sha256(raw).hexdigest())
                parent = hashlib.sha256(receipts[label.replace('-replay', '-execute')]).hexdigest() if label.endswith('-replay') else None
                self.assertEqual(ref['replay_parent_raw_sha256'], parent)
            self.assertEqual(artifact['native_input']['raw_sha256'], hashlib.sha256(b'native input\n').hexdigest())
            self.assertEqual(artifact['comparisons'], comparisons)
            for comparison in artifact['comparisons']:
                self.assertEqual(set(comparison['before']), set(R.FIELDS))
                self.assertEqual(set(comparison['after']), set(R.FIELDS))
            self.assertEqual(len(artifact['common_sources']), 2)
            for name, ctx in contexts.items():
                ref = artifact['variants'][name]['manifest']
                self.assertEqual(ref['relative_path'], 'editable-api-policy-' + name + '.workspace.json')
                self.assertEqual(ref['raw_sha256'], hashlib.sha256(ctx['raw']).hexdigest())
                source = artifact['variants'][name]['policy_source']
                self.assertEqual(source['relative_path'], R.BASE + name + '.gooo')
                self.assertEqual(source['raw_sha256'], hashlib.sha256(ctx['source_raw'][source['relative_path']]).hexdigest())
            saved = R.save_source_comparison(out, artifact, Path.read_bytes)
            raw = (out / saved['relative_path']).read_bytes()
            self.assertEqual(saved, {'relative_path': 'source-comparison.json', 'bytes': len(raw),
                                     'raw_sha256': hashlib.sha256(raw).hexdigest()})
            self.assertEqual(json.loads(raw), artifact)

    def test_saved_receipt_mutation_and_wrong_replay_parent_are_refused(self):
        with tempfile.TemporaryDirectory() as directory:
            out = Path(directory); contexts, receipts, comparisons = self.index_fixture(out)
            path = out / 'after-execute/stdout'; path.write_bytes(receipts['after-execute'] + b' ')
            with self.assertRaisesRegex(RuntimeError, 'actual command return'):
                R.source_comparison(R.REPO, out, contexts, receipts, b'input', comparisons, Path.read_bytes)
            path.write_bytes(receipts['after-execute'])
            bad = dict(receipts); bad['after-replay'] = json.dumps({'replayed_from_sha256': 'sha256:' + '0' * 64}).encode()
            (out / 'after-replay/stdout').write_bytes(bad['after-replay'])
            with self.assertRaisesRegex(RuntimeError, 'replay raw parent'):
                R.source_comparison(R.REPO, out, contexts, bad, b'input', comparisons, Path.read_bytes)
            self.assertFalse((out / 'source-comparison.json').exists())

    def test_diff_preserves_crlf_and_missing_final_lf(self):
        result = R.source_diff(b'value\r\n', b'value\n', 'before.gooo', 'after.gooo')
        self.assertEqual(result['status'], 'RENDERED')
        self.assertIn('-value\r\n+value\n', result['unified_diff'])
        self.assertEqual(result['before_line_endings']['crlf_count'], 1)
        self.assertEqual(result['after_line_endings']['crlf_count'], 0)
        result = R.source_diff(b'value', b'value\n', 'before.gooo', 'after.gooo')
        self.assertIn('-value\n\\ No newline at end of file\n+value\n', result['unified_diff'])
        self.assertFalse(result['before_line_endings']['ends_with_lf'])
        self.assertTrue(result['after_line_endings']['ends_with_lf'])

    def test_identical_source_skips_diff_even_when_large(self):
        raw = b'x' * (R.DIFF_MAX_BYTES + 1)
        with mock.patch.object(R.difflib, 'unified_diff', side_effect=AssertionError('must not compute')):
            result = R.source_diff(raw, raw, 'before.gooo', 'after.gooo')
        self.assertEqual(result['status'], 'IDENTICAL')
        self.assertEqual(result['unified_diff'], '')

    def test_display_limits_are_checked_before_diff_and_are_inclusive(self):
        for before, after, reason in [(b'a' * R.DIFF_MAX_BYTES, b'b', 'COMBINED_RAW_BYTES_EXCEED_131072'),
                                      (b'a\n' * 1001, b'b\n' * 1000, 'COMBINED_LF_LINES_EXCEED_2000')]:
            with self.subTest(reason=reason), mock.patch.object(R.difflib, 'unified_diff', side_effect=AssertionError('must not compute')):
                result = R.source_diff(before, after, 'before.gooo', 'after.gooo')
            self.assertEqual(result['status'], 'NOT_RENDERED_LIMIT')
            self.assertEqual(result['reason'], reason)
            self.assertEqual(result['unified_diff'], '')
        self.assertEqual(R.source_diff(b'a' * 65536, b'b' * 65536, 'before', 'after')['status'], 'RENDERED')
        self.assertEqual(R.source_diff(b'a\n' * 1000, b'b\n' * 1000, 'before', 'after')['status'], 'RENDERED')

    def test_artifact_write_failure_does_not_return_a_reference(self):
        with tempfile.TemporaryDirectory() as directory:
            out = Path(directory); (out / 'source-comparison.json').mkdir()
            with self.assertRaises(FileExistsError):
                R.save_source_comparison(out, {'unit_fixture': 'display only'}, Path.read_bytes)


if __name__ == '__main__': unittest.main()
