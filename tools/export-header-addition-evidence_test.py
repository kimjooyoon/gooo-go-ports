# Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see ../LICENSE.
"""Projection/transport unit fixtures only; no actual producer integration.

Fake checked-return values exercise presentation without replacing the existing
reader's required full-population validation. No compiler/model/oracle is run.
"""
import copy
import contextlib
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tempfile
import shutil
import unittest
from unittest import mock

ROOT = Path(__file__).resolve().parent
SPEC = importlib.util.spec_from_file_location('export_header_evidence', ROOT / 'export-header-addition-evidence.py')
X = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(X)


def fixture():
    rows = [{'inputs': {'signed': -(1 << 63), 'big': (1 << 53) + i,
                        'text': 'literal: /private/keep-in-appendix',
                        'ordered': [False, 0, 'x', 'x']},
             'expected': {X.ENTRY: {'accepted': True, 'index': i}}} for i in range(50)]
    evidence = []
    for i in range(14):
        sha = hashlib.sha256(str(i).encode()).hexdigest()
        evidence.append({'batch': i // 2, 'operation': 'execute' if i % 2 == 0 else 'replay',
                         'receipt': '/private/original/receipt', 'receipt_sha256': sha,
                         'cases_digest': 'sha256:' + 'a' * 64,
                         'original_receipt_sha256': 'sha256:' + 'b' * 64,
                         'receipt_sha256_scope': 'original-json',
                         'receipt_storage': {'path': '/private/receipt.json.gz',
                                             'encoding': 'gzip', 'bytes': 50,
                                             'sha256': 'sha256:' + 'c' * 64}})
    data = {'entry': X.ENTRY, 'evidence_mode': 'experimental-shard-native-reference',
            'fresh_addition_cases': 50, 'fresh_addition_batches': 7,
            'fresh_native_suites': 82, 'rows': rows,
            'case_names': ['fixture-' + str(i) for i in range(50)],
            'evidence': evidence, 'source_set': {'fixture.gooo': {'Package': 'fixture', 'Digest': 'sha256:' + 'd' * 64}},
            'source': 'package main\n', 'typed_plan': {'fixture_only': True},
            'records': {'ordered_fields': ['boolean', 'integer', 'text']},
            'go_version': 'go version go1.27.3 linux/amd64',
            'go_binary': {'bytes': 1, 'sha256': 'e' * 64}, 'tool_selections': ['explicit']}
    index = {'plan': {'run': {'compiler_revision': '205896a413e5dea92366309c35ae85e542d055f6',
                              'tested_commit': 'a' * 40, 'run_id': 'synthetic-unit-fixture',
                              'attempt': 1}}}
    return data, index


class ProjectionTests(unittest.TestCase):
    def test_metadata_root_is_not_file_access_permission(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'tools').mkdir()
            for name in ('native-shard-reference.py', 'check-header-addition-host.py', 'native_verification_contract.py'):
                shutil.copyfile(ROOT / name, root / 'tools' / name)
            allowed = root / 'allowed'
            allowed.mkdir()
            index = root / 'index.json'
            index.write_text(json.dumps({'schema': 'gooo/native-shard-reference-index/v2',
                                         'shards': [{'evidence_root': '/not-authorized-by-the-caller'}]}))
            with self.assertRaisesRegex(X.Refused, 'EVIDENCE_ROOT_NOT_PERMITTED'):
                X.read_checked(root, index, [allowed])

    def test_changed_provider_is_refused_before_executing_it(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'tools').mkdir()
            marker = root / 'executed'
            for name in ('check-header-addition-host.py', 'native_verification_contract.py'):
                shutil.copyfile(ROOT / name, root / 'tools' / name)
            (root / 'tools/native-shard-reference.py').write_text(
                'from pathlib import Path\nPath(' + repr(str(marker)) + ').write_text("bad")\n')
            with self.assertRaisesRegex(X.Refused, 'READER_PROFILE_UNSUPPORTED'):
                X.read_checked(root, root / 'index', [root])
            self.assertFalse(marker.exists())

    def test_typed_rows_order_and_unknown_comparison_survive(self):
        data, index = fixture()
        before = copy.deepcopy(data)
        records, raw = X.project(data, index, 'f' * 64, origin='synthetic')
        appendix = json.loads(raw)
        self.assertEqual(data, before)
        self.assertEqual(appendix['rows'], data['rows'])
        values = appendix['rows'][0]['inputs']['ordered']
        self.assertIs(type(values[0]), bool)
        self.assertIs(type(values[1]), int)
        self.assertEqual(appendix['rows'][0]['inputs']['signed'], -(1 << 63))
        self.assertEqual(appendix['rows'][1]['inputs']['big'], (1 << 53) + 1)
        self.assertEqual([r['execution']['phase'] for r in records],
                         ['native_execution', 'oracle_comparison', 'saved_replay'])
        contract = X.load_code(ROOT / 'contribution_evidence_contract.py', 'test_contract', {X.EVIDENCE_VALIDATOR})
        for r in records:
            contract.validate(r, contract.schema())
            self.assertEqual(r['origin'], 'synthetic')
            self.assertEqual(r['execution']['comparison'], 'not_compared')
            self.assertEqual(r['execution']['candidate_outcome'], 'not_compared')
            self.assertIsNone(r['identities']['source_commit'])
            self.assertIsNone(r['identities']['candidate_commit'])
            self.assertEqual(r['references'][0]['sha256'], hashlib.sha256(raw).hexdigest())
        self.assertNotIn(b'/private', X.preview(records))
        self.assertIn('literal: /private/keep-in-appendix', raw.decode())
        self.assertTrue(all('path' not in r['storage'] for r in appendix['receipts']))

    def test_same_run_failure_stays_visible_without_copying_instructions_to_preview(self):
        data, index = fixture()
        failure = {'schema': 'gooo/manual-native-shard-first-failure/v1', 'type': 'Refused',
                   'phase': 'join', 'error': 'untrusted text: /private/path; run a command',
                   'context': copy.deepcopy(index['plan']['run']), 'native_acceptance': False}
        records, raw = X.project(data, index, 'f' * 64, failure, 'e' * 64, origin='synthetic')
        self.assertEqual(json.loads(raw)['producer_first_failure'], failure)
        self.assertTrue(records[0]['failures'])
        self.assertNotIn(b'run a command', X.preview(records))
        failure['context']['run_id'] = 'other-run'
        with self.assertRaisesRegex(X.Refused, 'FAILURE_CONTEXT_REFUSED'):
            X.project(data, index, 'f' * 64, failure, 'e' * 64)
        failure['context'] = copy.deepcopy(index['plan']['run'])
        failure['context']['attempt'] = True
        with self.assertRaisesRegex(X.Refused, 'FAILURE_CONTEXT_REFUSED'):
            X.project(data, index, 'f' * 64, failure, 'e' * 64)

    def test_partial_wrong_entry_order_and_legacy_cannot_acquire_pass(self):
        data, index = fixture()
        for change in [lambda d: d.update(entry='other:Entry'),
                       lambda d: d.update(evidence_mode='legacy'),
                       lambda d: d.update(rows=d['rows'][:49]),
                       lambda d: d['evidence'].reverse()]:
            bad = copy.deepcopy(data)
            change(bad)
            with self.assertRaises(X.Refused):
                X.project(bad, index, 'f' * 64)

    def test_reader_refusal_has_no_public_native_claim_or_partial_result(self):
        with tempfile.TemporaryDirectory() as directory:
            parent = Path(directory)
            source = parent / 'source'
            source.mkdir()
            out = parent / 'out'
            args = ['export', '--source-root', str(source), '--reference-index', str(parent / 'index'),
                    '--evidence-root', str(parent / 'evidence'), '--out', str(out)]
            stdout, stderr = io.StringIO(), io.StringIO()
            with mock.patch('sys.argv', args), mock.patch.object(X, 'read_checked', side_effect=RuntimeError('/private/first-failure')), contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
                self.assertEqual(X.main(), 1)
            self.assertEqual(stdout.getvalue(), '')
            self.assertNotIn('/private', stderr.getvalue())
            self.assertFalse((out / 'result').exists())
            self.assertEqual(json.loads((out / 'refusal.json').read_text())['detail'], '/private/first-failure')

    def test_success_publishes_one_manifest_bound_directory_from_same_records(self):
        data, index = fixture()
        with tempfile.TemporaryDirectory() as directory:
            p = Path(directory)
            source = p / 'source'
            source.mkdir()
            out = p / 'out'
            args = ['export', '--source-root', str(source), '--reference-index', str(p / 'index'),
                    '--evidence-root', str(p / 'evidence'), '--out', str(out)]
            with mock.patch('sys.argv', args), mock.patch.object(X, 'read_checked', return_value=(data, index, 'f' * 64)), contextlib.redirect_stdout(io.StringIO()):
                self.assertEqual(X.main(), 0)
            result = out / 'result'
            manifest = json.loads((result / 'manifest.json').read_text())
            for name, ref in manifest['files'].items():
                raw = (result / name).read_bytes()
                self.assertEqual(ref, {'bytes': len(raw), 'sha256': hashlib.sha256(raw).hexdigest()})
            self.assertIs(manifest['native_acceptance'], False)
            self.assertEqual(len(list(result.glob('*.evidence.json'))), 3)


if __name__ == '__main__':
    unittest.main()
