"""Pure evidence-checker regressions; these fixtures are not native observations.

The small fixture projects real retained research rows and equal execute/replay
traces. Repetition and mutation below test verifier invariants only. No compiler,
Go build, network request or workspace execution runs from this test module.
"""

import copy
import json
import os
from pathlib import Path
import subprocess
import sys
import unittest

from verification_checks import check_native_rows, check_workspace_trace


ROOT = Path(__file__).resolve().parent
FIXTURE = json.loads((ROOT / 'testdata/verifier-fixture.json').read_text())


def native_inputs():
    """Repeat one fixture row to isolate the exact 63/16 cardinality contract."""
    bindings = copy.deepcopy(FIXTURE['expected_bindings'])
    matcher = dict(copy.deepcopy(FIXTURE['native']['matcher']), bindings=bindings)
    impact = dict(copy.deepcopy(FIXTURE['native']['impact']), bindings=bindings)
    row = dict(matcher['facts'], matched=matcher['matched'])
    changed = dict(impact['facts'], want=impact['decision'])
    return [copy.deepcopy(matcher) for _ in range(63)], [copy.deepcopy(impact) for _ in range(16)], \
        [copy.deepcopy(row) for _ in range(63)], [copy.deepcopy(changed) for _ in range(16)], bindings


def workspace_check(prefix, context):
    check_workspace_trace(prefix, context['trace'], context['index'], context['case'],
                          context['program'], context['body_report'])


class NativeEvidenceChecks(unittest.TestCase):
    def test_real_projection_fixture_and_valid_cardinalities(self):
        self.assertIs(FIXTURE['fixture_only'], True)
        self.assertEqual(FIXTURE['schema'], 'gooo/agentmesh-verifier-test-fixture/v1')
        self.assertEqual(len(FIXTURE['origins']), 8)
        self.assertTrue(all(len(origin['sha256']) == 64 for origin in FIXTURE['origins']))
        check_native_rows(*native_inputs())

    def test_truncated_empty_and_extra_native_arrays_refused(self):
        for position, count in [(0, 63), (1, 16)]:
            for size in [0, count - 1, count + 1]:
                with self.subTest(position=position, size=size):
                    args = list(native_inputs())
                    args[position] = (args[position] * 2)[:size]
                    with self.assertRaises(AssertionError):
                        check_native_rows(*args)

    def test_native_array_shape_and_row_fields_refused(self):
        for position in [0, 1]:
            with self.subTest(position=position, mutation='tuple'):
                args = list(native_inputs())
                args[position] = tuple(args[position])
                with self.assertRaises(AssertionError):
                    check_native_rows(*args)
            with self.subTest(position=position, mutation='extra-row-field'):
                args = list(native_inputs())
                args[position][-1]['extra'] = 'unexpected'
                with self.assertRaises(AssertionError):
                    check_native_rows(*args)

    def test_each_impact_raw_fact_refused_when_altered(self):
        for field in ['before', 'after', 'req_type', 'req_name']:
            for mutation in ['value', 'missing', 'wrong-type']:
                with self.subTest(field=field, mutation=mutation):
                    args = list(native_inputs())
                    facts = args[1][-1]['facts']
                    if mutation == 'missing':
                        del facts[field]
                    else:
                        facts[field] = 'different' if mutation == 'value' else False
                    with self.assertRaises(AssertionError):
                        check_native_rows(*args)
        args = list(native_inputs())
        args[1][-1]['facts']['extra'] = 'unexpected'
        with self.assertRaises(AssertionError):
            check_native_rows(*args)

    def test_native_boolean_zero_one_refused_even_when_equal(self):
        for field in ['matched', 'before_match', 'after_match']:
            for number in [0, 1]:
                with self.subTest(field=field, number=number):
                    args = list(native_inputs())
                    if field == 'matched':
                        args[2][-1]['matched'] = bool(number)
                        args[0][-1]['matched'] = number
                    else:
                        args[3][-1]['want'][field] = bool(number)
                        args[1][-1]['decision'][field] = number
                    with self.assertRaises(AssertionError):
                        check_native_rows(*args)

    def test_native_decision_string_and_binding_changes_refused(self):
        args = list(native_inputs())
        args[1][-1]['decision']['change'] = 1
        with self.assertRaises(AssertionError):
            check_native_rows(*args)
        for position in [0, 1]:
            with self.subTest(position=position):
                args = list(native_inputs())
                args[position][-1]['bindings']['generation_sha256'] = 'sha256:changed'
                with self.assertRaises(AssertionError):
                    check_native_rows(*args)


class WorkspaceEvidenceChecks(unittest.TestCase):
    def reject_each_phase(self, prefix, mutation, change):
        for phase in FIXTURE['workspace'][prefix]['phases']:
            with self.subTest(prefix=prefix, phase=phase, mutation=mutation):
                context = copy.deepcopy(FIXTURE['workspace'][prefix])
                change(context)
                with self.assertRaises(AssertionError):
                    workspace_check(prefix, context)

    def test_real_execute_and_replay_projections_accepted(self):
        for prefix in ['matcher', 'impact']:
            context = FIXTURE['workspace'][prefix]
            self.assertEqual(context['phases'], ['execute', 'replay'])
            for phase in context['phases']:
                with self.subTest(prefix=prefix, phase=phase):
                    workspace_check(prefix, copy.deepcopy(context))

    def test_trace_shape_case_index_and_activity_identity_refused(self):
        for prefix in ['matcher', 'impact']:
            for mutation, change in [
                ('index', lambda c: c['trace'].__setitem__('case_index', 1)),
                ('bool-index', lambda c: c['trace'].__setitem__('case_index', False)),
                ('no-delivery', lambda c: c['trace'].__setitem__('deliveries', [])),
                ('extra-delivery', lambda c: c['trace']['deliveries'].append(copy.deepcopy(c['trace']['deliveries'][0]))),
                ('activity-id', lambda c: c['trace']['deliveries'][0].__setitem__('activity_id', 'gooo://wrong/activity')),
                ('passed-number', lambda c: c['trace']['deliveries'][0].__setitem__('passed', 1)),
            ]:
                self.reject_each_phase(prefix, mutation, change)

    def test_workspace_entry_and_bound_export_contract_refused(self):
        for prefix in ['matcher', 'impact']:
            for field in ['package_path', 'activity', 'lowered_name']:
                self.reject_each_phase(prefix, 'entry-' + field,
                                       lambda c, f=field: c['program']['entry'].__setitem__(f, 'wrong'))
            for field in ['activity', 'activity_id', 'output_type']:
                self.reject_each_phase(prefix, 'report-' + field,
                                       lambda c, f=field: c['body_report'].__setitem__(f, 'wrong'))
            for field in ['id', 'name', 'kind', 'output_type']:
                self.reject_each_phase(prefix, 'export-' + field,
                                       lambda c, f=field: c['program']['workspace_image']['packages'][0]['exports'][0].__setitem__(f, 'wrong'))
            self.reject_each_phase(prefix, 'input-type',
                                   lambda c: c['program']['workspace_image']['packages'][0]['exports'][0]['input_types'].__setitem__(-1, 'gooo://wrong/text'))

    def test_input_port_entity_value_type_and_roster_refused(self):
        for prefix in ['matcher', 'impact']:
            for field, value in [('port', 'input0'), ('entity_id', 'gooo://wrong/text'), ('value', False)]:
                self.reject_each_phase(prefix, 'last-input-' + field,
                                       lambda c, f=field, v=value: c['trace']['deliveries'][0]['inputs'][-1].__setitem__(f, v))
            for mutation, change in [
                ('missing-input', lambda c: c['trace']['deliveries'][0]['inputs'].pop()),
                ('extra-input', lambda c: c['trace']['deliveries'][0]['inputs'].append(copy.deepcopy(c['trace']['deliveries'][0]['inputs'][0]))),
                ('reordered-input', lambda c: c['trace']['deliveries'][0]['inputs'].reverse()),
                ('extra-input-key', lambda c: c['trace']['deliveries'][0]['inputs'][-1].__setitem__('type_id', 'unexpected')),
            ]:
                self.reject_each_phase(prefix, mutation, change)

    def test_workspace_boolean_zero_one_refused_even_when_equal(self):
        for prefix, fields in [('matcher', [None]), ('impact', ['before_match', 'after_match'])]:
            for field in fields:
                for number in [0, 1]:
                    def change(context, f=field, n=number):
                        delivery = context['trace']['deliveries'][0]
                        expected = next(iter(context['case']['expected']))
                        if f is None:
                            context['case']['expected'][expected] = bool(n)
                            delivery['expected'] = bool(n)
                            delivery['actual'] = n
                        else:
                            context['case']['expected'][expected][f] = bool(n)
                            delivery['expected'][f] = bool(n)
                            delivery['actual'][f] = n
                            for item in delivery['actual_fields']:
                                if item['name'] == f:
                                    item['value'] = bool(n)
                    self.reject_each_phase(prefix, str(field) + '-' + str(number), change)

    def test_impact_field_id_name_value_and_roster_refused(self):
        for index in range(3):
            for field, value in [('id', 'gooo://wrong/field'), ('name', 'wrong'), ('value', 1)]:
                self.reject_each_phase('impact', str(index) + '-' + field,
                                       lambda c, i=index, f=field, v=value: c['trace']['deliveries'][0]['actual_fields'][i].__setitem__(f, v))
        for mutation, change in [
            ('missing-field', lambda c: c['trace']['deliveries'][0]['actual_fields'].pop()),
            ('extra-field', lambda c: c['trace']['deliveries'][0]['actual_fields'].append(copy.deepcopy(c['trace']['deliveries'][0]['actual_fields'][0]))),
            ('reordered-fields', lambda c: c['trace']['deliveries'][0]['actual_fields'].reverse()),
        ]:
            self.reject_each_phase('impact', mutation, change)

    def test_impact_record_schema_identity_field_types_and_roster_refused(self):
        for field in ['id', 'name']:
            self.reject_each_phase('impact', 'record-' + field,
                                   lambda c, f=field: c['body_report']['record_types'][0].__setitem__(f, 'wrong'))
        for index in range(3):
            for field in ['id', 'name', 'type_id']:
                self.reject_each_phase('impact', 'schema-' + str(index) + '-' + field,
                                       lambda c, i=index, f=field: c['body_report']['record_types'][0]['fields'][i].__setitem__(f, 'wrong'))
        self.reject_each_phase('impact', 'missing-schema-field',
                               lambda c: c['body_report']['record_types'][0]['fields'].pop())
        self.reject_each_phase('impact', 'extra-schema-field',
                               lambda c: c['body_report']['record_types'][0]['fields'].append(copy.deepcopy(c['body_report']['record_types'][0]['fields'][0])))


class OptimizedPythonRefusal(unittest.TestCase):
    def test_startup_refuses_optimization_before_help_or_native_work(self):
        for script in ['generate.py', 'verify.py']:
            for mode in ['-O', 'PYTHONOPTIMIZE']:
                with self.subTest(script=script, mode=mode):
                    env = os.environ.copy()
                    env['PYTHONDONTWRITEBYTECODE'] = '1'
                    env.pop('PYTHONOPTIMIZE', None)
                    command = [sys.executable]
                    if mode == '-O':
                        command.append('-O')
                    else:
                        env['PYTHONOPTIMIZE'] = '1'
                    command += [str(ROOT / script), '--help']
                    result = subprocess.run(command, cwd=ROOT, env=env, capture_output=True,
                                            text=True, timeout=10)
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn('optimization', result.stderr.lower())
                    self.assertEqual(result.stdout, '')


if __name__ == '__main__':
    unittest.main()
