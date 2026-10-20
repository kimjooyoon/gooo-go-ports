"""Adversarial checks of proof identity and types, not a second policy engine."""
import copy
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

import verify_grant_update as V
from verify import audit_runtime


def fixtures():
    decision = {'status': 'INVALID_INPUT', 'reason': 'INVALID_CANONICAL_INPUT',
                'invalid_field': 'after', 'required_observed': False,
                'required_before_match': False, 'required_after_match': False,
                'subset_rule_id': ''}
    case = {'inputs': {V.ENTRY + '.input' + str(i): value for i, value in enumerate(
        ['*', 'bad://a.*.b', 'mcp', 'weather'])}, 'expected': {V.ENTRY: decision}}
    exported = {'name': V.ACTIVITY, 'kind': 'activity', 'id': V.ACTIVITY_ID,
                'input_types': ['gooo://std/text'] * 4, 'output_type': V.RECORD_ID}
    fields = [{'name': name, 'id': V.BASE + name,
               'type_id': 'urn:gooo:type:' + ('boolean' if typ is bool else 'string')}
              for name, typ in V.FIELDS.items()]
    report = {'activity': 'GoooPackage0ActivityAssessServiceGrantUpdate',
              'activity_id': V.ACTIVITY_ID, 'output_type': 'GrantUpdateDecision',
              'input_parameters': [{'name': 'input' + str(i), 'type': 'string'} for i in range(4)],
              'record_types': [{'name': 'GrantUpdateDecision', 'id': V.RECORD_ID, 'fields': fields}],
              'compiler_source_sha': V.PIN, 'source_digest': V.digest(b'lowered fixture'),
              'typecheck_passed': True, 'deterministic_replay': True,
              'generated_digest': V.digest(b'generated fixture'),
              'replay_digest': V.digest(b'generated fixture')}
    entry = {'package_path': V.PACKAGE, 'activity': V.ACTIVITY,
             'lowered_name': report['activity']}
    program = {'schema': 'gooo/workspace-body-program/v1', 'entry': entry, 'activities': [entry],
               'lowered_gooo_source': 'lowered fixture', 'workspace_image': {
                   'packages': [{'path': V.PACKAGE, 'exports': [exported], 'sources': []}],
                   'init_order': [V.PACKAGE], 'entry': {
                       'package_path': V.PACKAGE, 'activity': V.ACTIVITY, 'source': 'source.gooo',
                       'inputs': ['Text'] * 4, 'output': 'GrantUpdateDecision'}}}
    delivery = {'activity_id': V.ACTIVITY_ID, 'passed': True,
                'inputs': [{'port': 'input' + str(i), 'entity_id': 'gooo://std/text', 'value': value}
                           for i, value in enumerate(V.case_facts(case).values())],
                'actual': copy.deepcopy(decision), 'expected': copy.deepcopy(decision),
                'actual_fields': [{'id': V.BASE + name, 'name': name, 'value': value}
                                  for name, value in decision.items()]}
    return case, program, report, {'case_index': 0, 'deliveries': [delivery]}


class GrantUpdateEvidenceTests(unittest.TestCase):
    def setUp(self):
        self.case, self.program, self.report, self.trace = fixtures()
        self.bindings = {'compiler_source_sha': V.PIN, 'assets': [{'name': 'source', 'bytes': 0}],
                         'complete': False}
        self.proof = {'facts': V.case_facts(self.case), 'decision': copy.deepcopy(self.case['expected'][V.ENTRY]),
                      'bindings': copy.deepcopy(self.bindings)}

    def rejects(self, callback):
        with self.assertRaises((AssertionError, ValueError)):
            callback()

    def check_trace(self, trace=None, program=None, report=None):
        V.check_trace(trace or self.trace, 0, self.case, program or self.program, report or self.report)

    def test_valid_typed_proof_and_trace(self):
        V.check_native([self.proof], [self.case], self.bindings)
        self.check_trace()

    def test_json_duplicate_members_and_non_json_constants(self):
        for raw in ['{"facts":{},"facts":{}}', '{"decision":{"status":"x","status":"y"}}',
                    '{"binding":0,"binding":false}', '[NaN]', '[Infinity]', '[-Infinity]']:
            with self.subTest(raw=raw):
                self.rejects(lambda: V.load_json(raw))

    def test_native_cardinality_missing_duplicate_and_extra(self):
        for actual in [[], [self.proof, self.proof], {'0': self.proof}]:
            self.rejects(lambda: V.check_native(actual, [self.case], self.bindings))
        for key in self.proof:
            changed = copy.deepcopy(self.proof)
            del changed[key]
            self.rejects(lambda: V.check_native([changed], [self.case], self.bindings))

    def test_all_four_raw_facts_are_bound(self):
        for name in V.FACTS:
            for mode in ['changed', 'missing', 'wrong_type']:
                with self.subTest(name=name, mode=mode):
                    changed = copy.deepcopy(self.proof)
                    if mode == 'missing':
                        del changed['facts'][name]
                    else:
                        changed['facts'][name] = 'other' if mode == 'changed' else False
                    self.rejects(lambda: V.check_native([changed], [self.case], self.bindings))

    def test_all_seven_decision_fields_have_exact_types(self):
        for name, typ in V.FIELDS.items():
            with self.subTest(name=name):
                changed = copy.deepcopy(self.proof)
                changed['decision'][name] = 0 if typ is bool else None
                self.rejects(lambda: V.check_native([changed], [self.case], self.bindings))
                del changed['decision'][name]
                self.rejects(lambda: V.check_native([changed], [self.case], self.bindings))

    def test_bindings_use_strict_recursive_types_and_complete_shape(self):
        for mutate in [lambda b: b.update(complete=0), lambda b: b['assets'][0].update(bytes=False),
                       lambda b: b['assets'].append(b['assets'][0]), lambda b: b.pop('compiler_source_sha')]:
            changed = copy.deepcopy(self.proof)
            mutate(changed['bindings'])
            self.rejects(lambda: V.check_native([changed], [self.case], self.bindings))

    def test_native_order_not_only_multiset(self):
        second = copy.deepcopy(self.case)
        second['inputs'][V.ENTRY + '.input0'] = 'mcp://*'
        second_proof = copy.deepcopy(self.proof)
        second_proof['facts'] = V.case_facts(second)
        self.rejects(lambda: V.check_native([second_proof, self.proof], [self.case, second], self.bindings))

    def test_trace_index_bool_and_duplicate_delivery_refused(self):
        changed = copy.deepcopy(self.trace)
        changed['case_index'] = False
        self.rejects(lambda: self.check_trace(trace=changed))
        changed = copy.deepcopy(self.trace)
        changed['deliveries'].append(changed['deliveries'][0])
        self.rejects(lambda: self.check_trace(trace=changed))

    def test_trace_requires_all_delivery_fields(self):
        for key in self.trace['deliveries'][0]:
            changed = copy.deepcopy(self.trace)
            del changed['deliveries'][0][key]
            self.rejects(lambda: self.check_trace(trace=changed))

    def test_trace_raw_input_port_entity_value_and_order(self):
        for index in range(4):
            for key, value in [('port', 'input0' if index else 'input3'),
                               ('entity_id', 'urn:wrong'), ('value', 0), ('value', 'changed')]:
                with self.subTest(index=index, key=key, value=value):
                    changed = copy.deepcopy(self.trace)
                    changed['deliveries'][0]['inputs'][index][key] = value
                    self.rejects(lambda: self.check_trace(trace=changed))
        changed = copy.deepcopy(self.trace)
        changed['deliveries'][0]['inputs'][1] = changed['deliveries'][0]['inputs'][0]
        self.rejects(lambda: self.check_trace(trace=changed))

    def test_trace_actual_expected_and_actual_fields_boolean_zero(self):
        for container in ['actual', 'expected', 'actual_fields']:
            changed = copy.deepcopy(self.trace)
            target = changed['deliveries'][0][container]
            if container == 'actual_fields':
                target[3]['value'] = 0
            else:
                target['required_observed'] = 0
            self.rejects(lambda: self.check_trace(trace=changed))

    def test_field_ids_types_duplicates_and_missing_fields(self):
        for key, value in [('id', 'wrong'), ('name', 'required_observed')]:
            changed = copy.deepcopy(self.trace)
            changed['deliveries'][0]['actual_fields'][0][key] = value
            self.rejects(lambda: self.check_trace(trace=changed))
        for key, value in [('id', 'wrong'), ('type_id', 'urn:gooo:type:boolean')]:
            changed = copy.deepcopy(self.report)
            changed['record_types'][0]['fields'][0][key] = value
            self.rejects(lambda: self.check_trace(report=changed))
        changed = copy.deepcopy(self.report)
        changed['record_types'][0]['fields'].append(changed['record_types'][0]['fields'][0])
        self.rejects(lambda: self.check_trace(report=changed))
        changed = copy.deepcopy(self.trace)
        changed['deliveries'][0]['actual_fields'].pop()
        self.rejects(lambda: self.check_trace(trace=changed))

    def test_entry_activity_export_and_record_identity(self):
        for mutate in [lambda p: p['entry'].update(activity='Other'),
                       lambda p: p['workspace_image']['packages'].append(p['workspace_image']['packages'][0]),
                       lambda p: p['workspace_image']['packages'][0]['exports'].append(p['workspace_image']['packages'][0]['exports'][0]),
                       lambda p: p['workspace_image']['packages'][0]['exports'][0].update(id='wrong')]:
            changed = copy.deepcopy(self.program)
            mutate(changed)
            self.rejects(lambda: self.check_trace(program=changed))
        for mutate in [lambda r: r.update(activity_id='wrong'),
                       lambda r: r['record_types'].append(r['record_types'][0]),
                       lambda r: r.update(output_type='Other')]:
            changed = copy.deepcopy(self.report)
            mutate(changed)
            self.rejects(lambda: self.check_trace(report=changed))

    def test_program_sources_must_match_actual_manifest_files(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'source.gooo').write_bytes(b'original source')
            manifest = {'entry': {'package_path': V.PACKAGE, 'activity': V.ACTIVITY},
                        'packages': [{'path': V.PACKAGE, 'sources': ['source.gooo']}]}
            p = copy.deepcopy(self.program)
            p['workspace_image']['packages'][0]['sources'] = [
                {'filename': 'source.gooo', 'source_digest': V.source_digest(b'original source')}]
            V.check_program_sources(p, manifest, root)
            (root / 'source.gooo').write_bytes(b'changed source')
            self.rejects(lambda: V.check_program_sources(p, manifest, root))
            (root / 'source.gooo').write_bytes(b'original source')
            for mutate in [lambda p: p['workspace_image']['init_order'].append(V.PACKAGE),
                           lambda p: p['workspace_image']['packages'].append(p['workspace_image']['packages'][0]),
                           lambda p: p['workspace_image']['packages'][0]['sources'][0].update(filename='other')]:
                changed = copy.deepcopy(p)
                mutate(changed)
                self.rejects(lambda: V.check_program_sources(changed, manifest, root))

    def receipt(self, casefile, workspace):
        process = {'started': True, 'completed': True, 'canceled': False, 'timed_out': False,
                   'output_truncated': False, 'exit_code': 0}
        source_hash = V.digest(b'lowered fixture')
        runtime = {'schema': 'gooo/body-composition-runtime/v1', 'stage': 'COMPLETE',
                   'producer_source_sha': V.PIN, 'producer_module': {'path': 'github.com/kimjooyoon/meta-ontology-go'},
                   'go_version': 'go version go1.27.3 test/fixture', 'projection_replayed': True,
                   'runtime_replayed': True, 'finite_passed': 1, 'finite_total': 1, 'model_calls': 0,
                   'runs': [process], 'toolchain': process, 'build': process, 'traces': [self.trace],
                   'original_source_sha256': source_hash, 'selected_source_sha256': source_hash,
                   'generated_sha256': V.digest(b'generated composition'), 'driver_sha256': V.digest(b'driver'),
                   'typed_plan_sha256': V.digest(b'plan')}
        composition = {'schema': 'gooo/body-composition/v1', 'stage': 'COMPLETE',
                       'gooo_source': 'lowered fixture', 'original_source_sha256': source_hash,
                       'selected_source_sha256': source_hash, 'source': 'generated composition',
                       'generated_sha256': V.digest(b'generated composition'), 'driver_source': 'driver',
                       'driver_sha256': V.digest(b'driver'), 'plan': {'typed_plan_sha256': V.digest(b'plan')},
                       'steps': [{'input_source_sha256': source_hash,
                                  'generation': {'report': self.report, 'source': 'generated fixture'}}]}
        return {'schema': 'gooo/workspace-body-execution-receipt/v1', 'decision': 'PASS',
                'manifest_digest': V.digest(workspace.read_bytes()), 'cases_digest': V.digest(casefile.read_bytes()),
                'result': {'program': self.program, 'runtime': runtime, 'composition': composition,
                           'execution_source_sha256': source_hash}}

    def test_receipt_source_producer_program_manifest_and_process_types(self):
        corpus = {'schema': 'gooo/body-composition-cases/v1', 'cases': [self.case]}
        with tempfile.TemporaryDirectory() as directory:
            casefile = Path(directory) / 'cases.json'
            workspace = Path(directory) / 'workspace.json'
            casefile.write_bytes(V.encoded(corpus))
            workspace.write_bytes(b'fixture manifest')
            original = self.receipt(casefile, workspace)
            def check(value):
                V.check_receipt(value, corpus, casefile, workspace, self.program, self.report, audit_runtime)
            check(original)
            for mutate in [lambda d: d.update(cases_digest='wrong'),
                           lambda d: d.update(manifest_digest='wrong'),
                           lambda d: d['result']['runtime'].update(producer_source_sha='wrong'),
                           lambda d: d['result']['runtime'].update(model_calls=False),
                           lambda d: d['result']['runtime'].update(projection_replayed=1),
                           lambda d: d['result']['runtime']['runs'][0].update(exit_code=False),
                           lambda d: d['result']['runtime']['runs'][0].update(canceled=True),
                           lambda d: d['result']['runtime'].update(traces=[]),
                           lambda d: d['result']['program']['entry'].update(lowered_name='wrong'),
                           lambda d: d['result'].update(execution_source_sha256='wrong'),
                           lambda d: d['result']['composition']['steps'][0]['generation']['report'].update(source_digest='wrong'),
                           lambda d: d['result']['composition'].update(source='changed generated program')]:
                changed = copy.deepcopy(original)
                mutate(changed)
                self.rejects(lambda: check(changed))

    def test_generated_and_replay_digests_bound_to_saved_report(self):
        corpus = {'schema': 'gooo/body-composition-cases/v1', 'cases': [self.case]}
        with tempfile.TemporaryDirectory() as directory:
            casefile = Path(directory) / 'cases.json'
            workspace = Path(directory) / 'workspace.json'
            casefile.write_bytes(V.encoded(corpus))
            workspace.write_bytes(b'fixture manifest')
            original = self.receipt(casefile, workspace)
            def check(value):
                V.check_receipt(value, corpus, casefile, workspace, self.program, self.report, audit_runtime)
            check(original)
            changed = copy.deepcopy(original)
            generation = changed['result']['composition']['steps'][0]['generation']
            generation['source'] = 'coherently changed generated source'
            changed_digest = V.digest(generation['source'].encode())
            generation['report'].update(generated_digest=changed_digest, replay_digest=changed_digest)
            self.assertEqual(generation['report']['generated_digest'], V.digest(generation['source'].encode()))
            self.assertNotEqual(changed_digest, self.report['generated_digest'])
            self.rejects(lambda: check(changed))
            changed = copy.deepcopy(original)
            changed['result']['composition']['steps'][0]['generation']['report']['replay_digest'] = changed_digest
            self.rejects(lambda: check(changed))

    def test_saved_replay_must_identify_exact_execution_bytes(self):
        raw = b'{"receipt":"first execution"}\n'
        replay = {'replayed_from_sha256': V.digest(raw)}
        V.check_replay_identity(raw, replay)
        self.rejects(lambda: V.check_replay_identity(raw + b' ', replay))
        self.rejects(lambda: V.check_replay_identity(raw, {'replayed_from_sha256': False}))

    def test_demo_requires_exact_four_results_and_revision_types(self):
        encode = lambda rows: ('\n'.join(json.dumps(r) for r in rows) + '\n').encode()
        V.check_demo(encode(V.DEMO))
        for rows in [V.DEMO[:-1], V.DEMO + [V.DEMO[-1]], [V.DEMO[0]] * 4]:
            self.rejects(lambda: V.check_demo(encode(rows)))
        rows = copy.deepcopy(V.DEMO)
        rows[1]['committed_revision'] = False
        self.rejects(lambda: V.check_demo(encode(rows)))
        self.rejects(lambda: V.check_demo(encode(V.DEMO).replace(b'"committed_revision": 0', b'"committed_revision": 0, "committed_revision": 0', 1)))

    def test_optimized_python_cannot_disable_checks(self):
        result = subprocess.run([sys.executable, '-O', '-B', '-c', 'import verify_grant_update'],
                                cwd=Path(__file__).resolve().parent, capture_output=True, timeout=10)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(b'optimization disables integrity assertions', result.stderr)


if __name__ == '__main__':
    unittest.main()
