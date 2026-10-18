#!/usr/bin/env python3
"""Transport/comparison/refusal controls; no compiler, network or subprocess."""
import copy
import importlib.util
import json
from pathlib import Path
import unittest

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
              'go_tool_sha256': 'sha256:tool', 'go_version': 'go version go1.27.2 test/arch'}
        R.producer(rt, rt['go_version'], rt['go_tool_sha256'])
        for key, bad in [('producer_source_sha', 'other'), ('go_tool_selection', 'ambient'),
                         ('go_tool_sha256', 'sha256:other'), ('go_version', 'go version go1.27.1 test/arch')]:
            with self.subTest(key=key), self.assertRaises(RuntimeError):
                R.producer(dict(rt, **{key: bad}), rt['go_version'], rt['go_tool_sha256'])

    def inspection_profile(self):
        lock = {'compiler_revision': R.H.REV, 'compiler_version': '0.6.32-dev', 'go_version': 'go1.27.2',
                'consumer_compiler_profiles': {'source_inspection036': dict(R.SOURCE_INSPECTION036)}}
        return lock, R.compiler_profile(lock, 'source-inspection036')

    def test_explicit_profile_contract_and_producer_are_separate(self):
        lock, profile = self.inspection_profile()
        self.assertEqual(R.compiler_profile(lock)['compiler_revision'], R.H.REV)
        for key in R.SOURCE_INSPECTION036:
            changed = copy.deepcopy(lock); changed['consumer_compiler_profiles']['source_inspection036'][key] = 'other'
            with self.subTest(key=key), self.assertRaises(RuntimeError):
                R.compiler_profile(changed, 'source-inspection036')
        rt = {'producer_source_sha': profile['compiler_revision'], 'go_tool_selection': 'explicit',
              'go_tool_sha256': 'sha256:tool', 'go_version': 'go version go1.27.2 test/arch'}
        R.producer(rt, rt['go_version'], rt['go_tool_sha256'], profile)
        with self.assertRaises(RuntimeError): R.producer(rt, rt['go_version'], rt['go_tool_sha256'])
        with self.assertRaises(RuntimeError):
            R.producer(dict(rt, producer_source_sha=R.H.REV), rt['go_version'], rt['go_tool_sha256'], profile)
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
        for key, bad in [('compiler_source_sha', R.H.REV), ('vcs_modified', 'true'),
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


if __name__ == '__main__': unittest.main()
