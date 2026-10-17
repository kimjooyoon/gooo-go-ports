#!/usr/bin/env python3
"""Pure parser/reference/receipt negative controls; never runs Go or Gooo."""
import copy
import importlib.util
import json
from pathlib import Path
import sys
import unittest

ROOT = Path(__file__).resolve().parents[2]
EXAMPLE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('autobot_resolver_verifier', EXAMPLE / 'verify.py')
V = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = V
spec.loader.exec_module(V)
H = V.helpers(ROOT)


def fixture():
    """Unsigned synthetic structural fixture, not a native PASS observation."""
    source = (ROOT / V.SOURCE).read_bytes()
    manifest = (ROOT / V.MANIFEST).read_bytes()
    rows = V.validate_rows(V.load((EXAMPLE / 'corpus.json').read_bytes(), 'unit corpus'))[:2]
    values = V.original_reference((EXAMPLE / 'upstream/ai_lines.py.txt').read_text(), rows)
    wire = V.json_bytes({'schema': 'gooo/body-composition-inputs/v1',
                        'inputs': [{V.ENTRY + '.input' + str(i): x for i, x in enumerate(row)} for row in rows]})
    lowered = source.decode().replace('activity ResolveLine(', 'activity GoooPackage0ActivityResolveLine(')
    definition = H.activities(source.decode())[0]
    source_sha = V.digest(lowered.encode())
    generated, driver, go = b'package main\n// synthetic generated\n', b'package main\n// synthetic driver\n', b'synthetic-Go-identity'
    fields = [{'name': name, 'id': V.RID.rsplit('/', 1)[0] + '/' + name.replace('_', '-'),
               'type_id': 'urn:gooo:type:' + ('boolean' if name in ('stored_present', 'floor_present') else 'string')}
              for name in V.FIELDS]
    ports = [{'port': 'input' + str(i), 'type': kind, 'entity_id': 'gooo://std/' +
              ('text' if kind == 'Text' else 'boolean'), 'from': -1} for i, kind in enumerate(V.TYPES)]
    record = {'id': V.RID, 'fields': fields}
    report = {'schema': 'gooo/body-codegen-report/v3', 'decision': 'PASS', 'compiler_source_sha': V.REV,
              'activity_id': V.ACTIVITY_ID, 'output_type': 'LineDecision', 'typecheck_passed': True,
              'deterministic_replay': True, 'repository_writes': 0, 'record_types': [record],
              'input_parameters': [{'name': 'input' + str(i), 'type': 'string' if kind == 'Text' else 'bool'}
                                   for i, kind in enumerate(V.TYPES)], 'source_digest': source_sha,
              'program_digest': V.digest(definition['body'].encode()), 'generated_digest': V.digest(generated),
              'replay_digest': V.digest(generated)}
    entry = {'package_path': V.PACKAGE, 'activity': 'ResolveLine', 'lowered_name': 'GoooPackage0ActivityResolveLine'}
    program = {'schema': 'gooo/workspace-body-program/v1', 'entry': entry, 'activities': [entry],
               'lowered_gooo_source': lowered, 'workspace_image': {'schema': 'gooo/package-runtime-image/v1',
               'init_order': [V.PACKAGE], 'packages': [{'path': V.PACKAGE, 'sources': [{'filename': V.SOURCE,
               'source_digest': V.digest(H.go_json(source.decode()))}]}], 'entry': {'package_path': V.PACKAGE,
               'source': V.SOURCE, 'activity': 'ResolveLine', 'inputs': V.TYPES, 'output': 'LineDecision'}}}
    composition = {'schema': 'gooo/body-composition/v1', 'stage': 'COMPLETE', 'gooo_source': lowered,
                   'original_source_sha256': source_sha, 'selected_source_sha256': source_sha,
                   'source': generated.decode(), 'generated_sha256': V.digest(generated),
                   'driver_source': driver.decode(), 'driver_sha256': V.digest(driver),
                   'steps': [{'input_source_sha256': source_sha, 'generation': {'source': generated.decode(), 'report': report}}],
                   'plan': {'schema': 'gooo/body-composition-plan/v1', 'edges': [], 'record_types': [copy.deepcopy(record)],
                   'activities': [{'name': entry['lowered_name'], 'id': V.ACTIVITY_ID,
                                   'output_entity_id': V.RID, 'inputs': ports}]}}
    process = {'started': True, 'completed': True, 'exit_code': 0, 'canceled': False,
               'timed_out': False, 'output_truncated': False}
    runtime = {'schema': 'gooo/body-composition-runtime/v1', 'stage': 'COMPLETE', 'producer_source_sha': V.REV,
               'go_version': 'go version go1.27.2 synthetic/fixture', 'go_tool_selection': 'explicit',
               'go_tool_sha256': V.digest(go), 'original_source_sha256': source_sha, 'selected_source_sha256': source_sha,
               'generated_sha256': V.digest(generated), 'driver_sha256': V.digest(driver),
               'finite_total': 0, 'finite_passed': 0, 'model_calls': 0, 'projection_replayed': True,
               'runtime_replayed': True, 'toolchain': copy.deepcopy(process), 'build': copy.deepcopy(process),
               'runs': [dict(process, stdout_sha256=V.digest(V.json_bytes([[x] for x in values]))) for _ in range(2)],
               'traces': [{'case_index': i, 'deliveries': [{'activity_id': V.ACTIVITY_ID,
               'inputs': [{'port': p['port'], 'entity_id': p['entity_id'], 'value': x} for p, x in zip(ports, row)],
               'actual': value, 'actual_fields': [{'id': f['id'], 'name': f['name'], 'value': value[f['name']]} for f in fields]}]}
               for i, (row, value) in enumerate(zip(rows, values))]}
    receipt = {'schema': 'gooo/workspace-body-execution-receipt/v1', 'decision': 'OBSERVED',
               'manifest': str(ROOT / V.MANIFEST), 'manifest_digest': V.digest(manifest), 'inputs_digest': V.digest(wire),
               'result': {'schema': 'gooo/workspace-body-execution/v1', 'program': program, 'composition': composition,
                          'runtime': runtime, 'execution_source_sha256': source_sha}}
    args = (rows, values, wire, source, manifest, ROOT / V.MANIFEST, runtime['go_version'], go, H)
    return receipt, args


class ResolverVerificationTests(unittest.TestCase):
    def test_strict_json_errors_are_labelled(self):
        for raw in [b'{"a":1,"a":2}', b'{"a":1,"\\u0061":2}', b'NaN', b'{}[]', b'"\xff"']:
            with self.subTest(raw=raw), self.assertRaisesRegex(ValueError, 'unit-wire: invalid JSON'):
                V.load(raw, 'unit-wire')

    def test_corpus_preserves_all204_and_false_is_not_zero(self):
        corpus = V.load((EXAMPLE / 'corpus.json').read_bytes(), 'corpus')
        self.assertEqual(len(V.validate_rows(corpus)), 204)
        for index, value in [(0, None), (1, 0), (2, False), (3, 1), (5, '\ud800')]:
            broken = copy.deepcopy(corpus); broken['cases'][0]['inputs'][index] = value
            with self.subTest(index=index), self.assertRaises(ValueError):
                V.validate_rows(broken)
        broken = copy.deepcopy(corpus); broken['cases'][1]['id'] = broken['cases'][0]['id']
        with self.assertRaises(ValueError):
            V.validate_rows(broken)

    def test_none_and_empty_baseline_remain_distinct_and_untrimmed(self):
        text = (EXAMPLE / 'upstream/ai_lines.py.txt').read_text()
        rows = [['key', False, '', False, False, '', False, False, False],
                ['key', False, '', False, True, '', False, False, False],
                ['key', False, '', False, True, ' raw\x00\r\n\U0001f600 ', False, False, False]]
        a, b, c = V.original_reference(text, rows)
        self.assertFalse(a['floor_present']); self.assertTrue(b['floor_present']); self.assertEqual(b['floor_value'], '')
        self.assertEqual(c['floor_value'], rows[2][5]); self.assertEqual(c['line'], 'draft_then_send')
        self.assertEqual(c['reason'], "approval baseline category ' raw\x00\r\n\U0001f600 ' can never be autonomous")

    def test_original_priority_is_scope_then_store_then_stored_never(self):
        text = (EXAMPLE / 'upstream/ai_lines.py.txt').read_text()
        rows = [['key', True, 'never', True, True, 'baseline', True, True, True],
                ['key', True, 'never', True, True, 'baseline', True, True, False],
                ['key', True, 'never', True, True, 'baseline', True, False, False]]
        values = V.original_reference(text, rows)
        self.assertEqual([v['source'] for v in values], ['no_verified_company', 'store_unavailable', 'stored'])
        self.assertEqual([v['stored_present'] for v in values], [False, False, True])
        self.assertEqual([v['floor_present'] for v in values], [False, False, False])

    def test_original_imports_and_unselected_code_are_not_executed(self):
        source = 'raise RuntimeError("unselected")\n' + (EXAMPLE / 'upstream/ai_lines.py.txt').read_text()
        value = V.original_reference(source, [['x', False, '', True, False, '', False, False, False]])[0]
        self.assertEqual(value['line'], 'draft_then_send')
        with self.assertRaisesRegex(ValueError, 'pure-node roster'):
            V.original_reference(source.replace('def _unset(', 'def missing_unset('), [])

    def test_current_pin_and_all_original_hashes_are_bound(self):
        pin = V.load((EXAMPLE / 'source-pin.json').read_bytes(), 'pin')
        V.validate_pin(pin, lambda name: (ROOT / name).read_bytes())
        for mutate in [lambda x: x['port'].update(sha256='0' * 64),
                       lambda x: x['verification_inputs']['corpus'].update(path='unrelated'),
                       lambda x: x['output']['fields'][0].update(id='wrong'),
                       lambda x: x['output']['fields'][0].update(required=1)]:
            broken = copy.deepcopy(pin); mutate(broken)
            with self.assertRaises(ValueError):
                V.validate_pin(broken, lambda name: (ROOT / name).read_bytes())

    def test_converter_stdout_is_exact_and_not_a_finite_envelope(self):
        receipt, args = fixture(); rows, _, wire = args[:3]
        V.input_document(wire, rows)
        for raw in [wire[:-1], wire + b'\n', b' ' * 32769, wire.replace(b'inputs', b'cases', 1)]:
            with self.subTest(raw_length=len(raw)), self.assertRaises(ValueError):
                V.input_document(raw, rows)
        parsed = V.load(wire, 'wire'); parsed['inputs'][0][V.ENTRY + '.input1'] = 0
        with self.assertRaises(ValueError):
            V.input_document(V.json_bytes(parsed), rows)

    def test_synthetic_receipt_checks_all_ordered_fields_and_raw_parent(self):
        receipt, args = fixture()
        raw = V.json_bytes(receipt)
        self.assertEqual(V.inspect(raw, *args)['rows'], 2)
        replay = copy.deepcopy(receipt); replay['replayed_from_sha256'] = V.digest(raw)
        replay['result']['replay'] = {'schema': 'gooo/workspace-body-replay/v1', 'model_calls': 0}
        V.inspect(V.json_bytes(replay), *args, parent=raw)
        with self.assertRaisesRegex(ValueError, 'raw saved'):
            V.inspect(V.json_bytes(replay), *args, parent=raw + b'\n')

    def test_receipt_rejects_finite_expectations_and_missing_native_completion(self):
        receipt, args = fixture()
        mutations = [lambda x: x.update(decision='PASS'), lambda x: x.update(cases_digest='old'),
                     lambda x: x['result']['runtime'].update(finite_total=2),
                     lambda x: x['result']['runtime'].update(finite_passed=False),
                     lambda x: x['result']['runtime']['traces'][0]['deliveries'][0].update(passed=True),
                     lambda x: x['result']['runtime']['runs'][0].update(completed=False),
                     lambda x: x['result']['runtime']['runs'][1].update(output_truncated=True),
                     lambda x: x['result']['runtime']['build'].update(exit_code=False)]
        for mutate in mutations:
            broken = copy.deepcopy(receipt); mutate(broken)
            with self.subTest(mutation=mutate), self.assertRaises(ValueError):
                V.inspect(V.json_bytes(broken), *args)

    def test_receipt_rejects_source_tool_abi_inputs_and_output_substitution(self):
        receipt, args = fixture()
        mutations = [lambda x: x.update(inputs_digest='sha256:' + '0' * 64),
                     lambda x: x.update(manifest='wrong-workspace'),
                     lambda x: x['result']['program']['workspace_image']['packages'][0]['sources'][0].update(source_digest='wrong'),
                     lambda x: x['result']['runtime'].update(go_tool_selection='fallback'),
                     lambda x: x['result']['runtime'].update(go_tool_sha256='wrong'),
                     lambda x: x['result']['runtime'].update(producer_source_sha='wrong'),
                     lambda x: x['result']['composition']['plan']['activities'][0]['inputs'].reverse(),
                     lambda x: x['result']['composition']['steps'][0]['generation']['report']['record_types'][0]['fields'][0].update(id='wrong'),
                     lambda x: x['result']['runtime']['traces'][0]['deliveries'][0]['inputs'][8].update(value=True),
                     lambda x: x['result']['runtime']['traces'][0]['deliveries'][0]['actual'].update(stored_present=0),
                     lambda x: x['result']['runtime']['traces'][0]['deliveries'][0]['actual_fields'].reverse(),
                     lambda x: x['result']['runtime']['runs'][0].update(stdout_sha256='wrong')]
        for mutate in mutations:
            broken = copy.deepcopy(receipt); mutate(broken)
            with self.subTest(mutation=mutate), self.assertRaises(ValueError):
                V.inspect(V.json_bytes(broken), *args)

    def test_receipt_json_and_structure_failures_are_distinct(self):
        _, args = fixture()
        with self.assertRaisesRegex(ValueError, 'Gooo receipt: invalid JSON'):
            V.inspect(b'{', *args)
        with self.assertRaisesRegex(ValueError, 'malformed Gooo receipt structure'):
            V.inspect(b'{}', *args)


if __name__ == '__main__':
    unittest.main()
