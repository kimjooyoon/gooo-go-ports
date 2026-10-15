"""Strict public IPv4 evidence boundaries, with fresh synthetic valid baselines.

No stored runtime golden, Go compiler, oracle or Gooo process is used by these
unit tests. Actual independent values and generated code remain runner duties.
"""
from pathlib import Path
from decimal import Decimal
import copy
import hashlib
import importlib.util
import json
import tempfile
import unittest
from unittest import mock

HERE = Path(__file__).resolve().parent
SPEC = importlib.util.spec_from_file_location('ipv4_receipt_unit', HERE / 'ipv4_receipts.py')
R = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(R)
SPAN_SPEC = importlib.util.spec_from_file_location('public_path_span_unit', HERE / 'check-path-facts.py')
SPAN = importlib.util.module_from_spec(SPAN_SPEC)
SPAN_SPEC.loader.exec_module(SPAN)
REPO = HERE.parent
GO = 'sha256:' + 'a' * 64
PRIOR = 'sha256:' + 'b' * 64


def packed(value):
    return json.dumps(value, separators=(',', ':')).encode()


def synthetic_oracle():
    """Kind/echo-only stand-in, not a copied implementation of netip."""
    corpus = json.loads((REPO / 'examples/ipv4-address-facts/corpus-202.json').read_bytes())
    rows = []
    for source in corpus['cases']:
        row = copy.deepcopy(source)
        expected = {**source['inputs'], **{f: False for f in R.FIELDS[4:]}}
        expected['valid'] = all(0 <= value <= 255 for value in source['inputs'].values())
        row['expected'] = expected
        rows.append(row)
    # One independently obvious manual witness used by synthetic trace fixtures.
    assert rows[0]['inputs'] == {'a': 0, 'b': 0, 'c': 0, 'd': 0}
    rows[0]['expected']['is_unspecified'] = True
    return rows


def fixture(checker, directory, entry=R.REPORT_ENTRY):
    """Assemble schema-valid metadata; no generated function is claimed run."""
    report = entry == R.REPORT_ENTRY
    name = entry.split(':')[-1]
    manifest = REPO / R.MANIFESTS[entry]
    input_path = directory / (name + '.inputs.json')
    inputs = {entry + '.input' + str(i): 0 for i in range(4)}
    input_path.write_bytes(packed({'schema': 'gooo/body-composition-inputs/v1', 'inputs': [inputs]}))
    needed, sites, _ = checker.graph(entry)
    all_names = ('IPv4Valid',) + R.METHODS + ('Report',)
    numbers = dict(zip(all_names, range(8)))
    lowering = {key: 'GoooPackage' + str(numbers[key.split(':')[-1]]) + 'Activity' + key.split(':')[-1] for key in needed}
    _, _, rewritten = checker.graph(entry, lowering)
    lowered = '\n'.join('activity ' + lowering[key] + '(Integer, Integer, Integer, Integer) -> ' +
                        ('IPv4AddressFacts' if key.endswith(':Report') else 'Boolean') +
                        ' computes ' + json.dumps(rewritten[key]) for key in needed)
    source_digest = R.digest(lowered.encode())
    packages = []
    for package in ([R.PORT, R.APP] if report else [R.PORT]):
        app = package == R.APP
        source = R.APP_SOURCE if app else R.PORT_SOURCE
        exports = [{'name': 'Integer', 'kind': 'entity', 'id': R.INTEGER}]
        if app:
            exports.append({'name': 'IPv4AddressFacts', 'kind': 'entity', 'id': R.RESULT})
            activities = ['Report']
        else:
            exports.append({'name': 'Boolean', 'kind': 'entity', 'id': R.BOOLEAN})
            activities = ['IPv4Valid'] + list(R.METHODS)
        exports += [{'name': n, 'kind': 'activity', 'id': R.IDS[n],
                     'input_types': [R.INTEGER] * 4, 'output_type': R.RESULT if n == 'Report' else R.BOOLEAN} for n in activities]
        packages.append({'path': package, 'name': 'ipv4_address_facts' if app else 'netip',
                         'namespace': 'gooo_original_ipv4_address_facts' if app else 'gooo_ports_net_netip',
                         'imports': [R.PORT] if app else None, 'exports': exports,
                         'sources': [{'filename': source, 'source_digest': R.digest(R.go_string((REPO / source).read_text()))}]})
    activity_ref = {'package_path': entry.split(':')[0], 'activity': name, 'lowered_name': lowering[entry]}
    pure_activities = [{'activity': {'package_path': key.split(':')[0], 'activity': key.split(':')[-1], 'lowered_name': lowering[key]},
                        'source': checker.bodies[key]['source'], 'source_body_sha256': R.digest(checker.bodies[key]['body'].encode())} for key in needed]
    plan = {'schema': 'gooo/body-composition-plan/v1', 'entry_activity': lowering[entry],
            'typed_plan_sha256': R.digest(b'synthetic typed plan'), 'semantic_fingerprint': 'c' * 64,
            'edges': [], 'activities': [{'name': lowering[entry], 'id': R.IDS[name], 'input_type': 'Integer',
                'output_type': 'IPv4AddressFacts' if report else 'Boolean', 'input_entity_id': R.INTEGER,
                'output_entity_id': R.RESULT if report else R.BOOLEAN, 'go_function': 'GoooComposedActivity0',
                'input_from': -1, 'assembling': False, 'inputs': [
                    {'port': 'input' + str(i), 'type': 'Integer', 'entity_id': R.INTEGER, 'from': -1} for i in range(4)]}]}
    if report:
        plan['record_types'] = [{'name': 'IPv4AddressFacts', 'id': R.RESULT,
            'go_name': 'GoooRecord' + R.digest(R.RESULT.encode())[7:], 'fields': [
                {'name': f, 'id': R.RESULT + '/' + f, 'type_id': 'urn:gooo:type:' + ('integer' if i < 4 else 'boolean'),
                 'go_name': 'GoooField' + R.digest((R.RESULT + '/' + f).encode())[7:]} for i, f in enumerate(R.FIELDS)]}]
    edges = []
    for key in needed:
        outgoing = [site for site in sites if site['caller'] == key]
        edges += [{'caller_id': checker.bodies[key]['id'], 'callee_id': checker.bodies[site['callee']]['id'], 'ordinal': i} for i, site in enumerate(outgoing)]
    def cost(key):
        return sum(1 + cost(site['callee']) for site in sites if site['caller'] == key)
    emitted = 'package main\n// synthetic metadata fixture, never built\n'
    step_source = 'package main\n// synthetic step, never built\n'
    driver = 'package main\n// synthetic driver, never built\n'
    generation = {'schema': 'gooo/body-codegen-report/v3', 'decision': 'PASS', 'activity': lowering[entry],
        'activity_id': R.IDS[name], 'compiler_source_sha': R.REV, 'source_digest': source_digest,
        'program_digest': R.digest(rewritten[entry].encode()), 'generated_digest': R.digest(step_source.encode()),
        'replay_digest': R.digest(step_source.encode()), 'input_type': '(int64,int64,int64,int64)',
        'input_parameters': [{'name': 'input' + str(i), 'type': 'int64'} for i in range(4)],
        'output_type': 'IPv4AddressFacts' if report else 'bool', 'typecheck_passed': True,
        'deterministic_replay': True, 'repository_writes': 0,
        'call_closure': {'schema': 'gooo/pure-activity-call-closure/v1', 'activities': [
            {'name': lowering[key], 'activity_id': checker.bodies[key]['id'], 'program_sha256': R.digest(rewritten[key].encode())} for key in needed if key != entry],
            'edges': edges, 'max_calls_per_invocation': cost(entry)}}
    if report:
        generation['record_types'] = copy.deepcopy(plan['record_types'])
    composition = {'schema': 'gooo/body-composition/v1', 'stage': 'COMPLETE',
        'original_source_sha256': source_digest, 'selected_source_sha256': source_digest, 'gooo_source': lowered,
        'plan': plan, 'steps': [{'input_source_sha256': source_digest, 'generation': {'source': step_source, 'report': generation}}],
        'source': emitted, 'driver_source': driver, 'generated_sha256': R.digest(emitted.encode()), 'driver_sha256': R.digest(driver.encode())}
    process = {'started': True, 'completed': True, 'canceled': False, 'timed_out': False,
        'output_truncated': False, 'diagnostics_bytes': 0, 'exit_code': 0,
        'stdout_sha256': R.digest(b'synthetic process output'), 'stderr_sha256': R.digest(b'')}
    expected = checker.rows[0]['expected'] if report else checker.rows[0]['expected'][R.FLAGS[R.METHODS.index(name)]]
    delivery = {'activity_id': R.IDS[name], 'inputs': [{'port': 'input' + str(i), 'entity_id': R.INTEGER, 'value': 0} for i in range(4)], 'actual': expected}
    if report:
        delivery['actual_fields'] = [{'id': R.RESULT + '/' + f, 'name': f, 'value': expected[f]} for f in R.FIELDS]
    runtime = {'schema': 'gooo/body-composition-runtime/v1', 'stage': 'COMPLETE',
        'producer_source_sha': R.REV, 'go_version': checker.go_version, 'go_tool_sha256': GO, 'go_tool_selection': 'explicit',
        'producer_module': {'path': 'github.com/kimjooyoon/meta-ontology-go', 'version': 'v0.0.0-20261011201109-83928fa45382'},
        'generated_sha256': composition['generated_sha256'], 'driver_sha256': composition['driver_sha256'],
        'original_source_sha256': source_digest, 'selected_source_sha256': source_digest,
        'typed_plan_sha256': plan['typed_plan_sha256'], 'executable_sha256': R.digest(b'not retained synthetic binary'),
        'runtime_suite_sha256': R.digest(b'synthetic suite'), 'model_calls': 0, 'finite_total': 0, 'finite_passed': 0,
        'projection_replayed': True, 'runtime_replayed': True, 'toolchain': copy.deepcopy(process),
        'build': copy.deepcopy(process), 'runs': [copy.deepcopy(process), copy.deepcopy(process)],
        'traces': [{'case_index': 0, 'deliveries': [delivery]}]}
    original = {'schema': 'gooo/workspace-body-execution-receipt/v1', 'decision': 'OBSERVED',
        'manifest': str(manifest), 'manifest_digest': R.digest(manifest.read_bytes()), 'inputs_digest': R.digest(input_path.read_bytes()),
        'result': {'schema': 'gooo/workspace-body-execution/v1', 'program': {'schema': 'gooo/workspace-body-program/v1',
            'workspace_image': {'schema': 'gooo/package-runtime-image/v1', 'packages': packages,
                'init_order': [R.PORT, R.APP] if report else [R.PORT], 'entry': {'package_path': entry.split(':')[0],
                'source': R.APP_SOURCE if report else R.PORT_SOURCE, 'activity': name, 'inputs': ['Integer'] * 4,
                'output': 'IPv4AddressFacts' if report else 'Boolean'}}, 'pure_calls': {'schema': 'gooo/workspace-pure-calls/v1',
                'activities': pure_activities, 'sites': sites}, 'entry': activity_ref, 'activities': [activity_ref], 'lowered_gooo_source': lowered},
            'execution_source_sha256': source_digest, 'composition': composition, 'runtime': runtime}}
    original_raw = packed(original)
    repeated = copy.deepcopy(original)
    repeated['replayed_from_sha256'] = R.digest(original_raw)
    repeated['result']['replay'] = {'schema': 'gooo/workspace-body-replay/v1', 'prior_result_sha256': PRIOR,
        'model_calls': 0, 'body_fills_replayed': 0}
    return original_raw, packed(repeated), input_path, manifest


class StrictInputs(unittest.TestCase):
    def test_all_seven_entry_mappings_and_exact_int64_boundaries(self):
        for entry, manifest in R.MANIFESTS.items():
            with self.subTest(entry=entry):
                self.assertEqual(len(R.validate_inputs(R.input_baseline(entry), entry)), 1)
                ports = {entry + '.input' + str(i): value for i, value in enumerate((-(1 << 63), (1 << 63) - 1, -1, 256))}
                self.assertEqual(R.validate_inputs(packed({'schema': 'gooo/body-composition-inputs/v1', 'inputs': [ports]}), entry), [ports])
                self.assertTrue((REPO / manifest).is_file())
                wrong = R.input_baseline(entry).replace(b'.input0', b'.input8')
                with self.assertRaisesRegex(R.InputError, '^MISSING_PORT:input0$'):
                    R.validate_inputs(wrong, entry)

    def test_all42_exact_intended_refusals_after_seven_positive_baselines(self):
        saved = {}
        def save(name, raw):
            saved[name] = raw
            return {'path': '/unit/' + name, 'bytes': len(raw), 'sha256': hashlib.sha256(raw).hexdigest()}
        records = R.run_wire_controls(save)
        self.assertEqual(len(records), 42)
        self.assertEqual(len(saved), 49)
        self.assertEqual(len({x['name'] for x in records}), 42)
        for row in records:
            self.assertEqual(row['expected_diagnostic'], row['observed_diagnostic'])
            self.assertEqual(row['baseline_entry_count'], 7)
            self.assertEqual(saved['wire-' + row['name']], row['raw'])
        for i in range(4):
            self.assertEqual(sum(x['name'].startswith('input' + str(i) + '-') for x in records), 8)

    def test_strict_json_kinds_duplicate_decoding_and_diagnostic_precedence(self):
        self.assertIsInstance(R.strict_json(b'{"n":0.125}')['n'], Decimal)
        for raw, code in [(b'{"a":1,"\\u0061":2}', 'DUPLICATE_FIELD'),
                          (b'{"x":"\xff"}', 'INVALID_RAW_UTF8'), (b'{} {}', 'TRAILING_JSON'),
                          (b'{"x":NaN}', 'MALFORMED_JSON')]:
            with self.subTest(raw=raw), self.assertRaisesRegex(R.ReceiptError, '^' + code + '$'):
                R.strict_json(raw)
        with self.assertRaisesRegex(R.InputError, '^BYTE_BOUND$'):
            R.validate_inputs(b'\xff' * 8193, R.REPORT_ENTRY)
        with self.assertRaisesRegex(R.InputError, '^ENTRY_REQUIRED$'):
            R.validate_inputs(R.input_baseline(), 'app/wrong:Report')

    def test_nested_exact_kind_comparison_has_no_boolean_numeric_coercion(self):
        for changed in (True, False, Decimal('1.0'), 1.0, '1'):
            self.assertFalse(R.exact({'nested': [{'n': 1}]}, {'nested': [{'n': changed}]}))
        self.assertTrue(R.exact({'n': [-1, True]}, {'n': [-1, True]}))


class ReceiptBoundaries(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.oracle = synthetic_oracle()

    def checker(self, version='go version go1.27.2 linux/amd64'):
        return R.Evidence(REPO, copy.deepcopy(self.oracle), version)

    def test_accepted_execute_replay_all_seven_entries_and_selected_host(self):
        with tempfile.TemporaryDirectory() as name:
            directory = Path(name).resolve()
            for version in ('go version go1.27.2 linux/amd64', 'go version go1.27.2 darwin/arm64'):
                checker = self.checker(version)
                for entry in R.MANIFESTS:
                    with self.subTest(version=version, entry=entry):
                        original, replay, inputs, manifest = fixture(checker, directory, entry)
                        first = checker.inspect(original, entry, [0], inputs, manifest, GO)
                        again = checker.inspect(replay, entry, [0], inputs, manifest, GO, original, PRIOR)
                        self.assertEqual(first, again)
                        self.assertEqual(first['output_count'], 1)
                        self.assertEqual(first['typed_scalar_fields'], 11 if entry == R.REPORT_ENTRY else 1)
                        self.assertEqual(first['original_echoes'], 4 if entry == R.REPORT_ENTRY else 0)
                        self.assertEqual(first['generated_sha256'], R.digest(first['generated_source'].encode()))

    def test_eighteen_fresh_targeted_tampers_fail_the_intended_gates(self):
        with tempfile.TemporaryDirectory() as name:
            checker = self.checker()
            original, replay, inputs, manifest = fixture(checker, Path(name).resolve())
            saved = {}
            def save(name, raw):
                saved[name] = raw
                return {'path': '/unit/' + name, 'bytes': len(raw), 'sha256': hashlib.sha256(raw).hexdigest()}
            records = R.check_tampers(checker, original, replay, R.REPORT_ENTRY, [0], inputs, manifest, GO, PRIOR, save, SPAN.value_span)
            self.assertEqual(len(records), 18)
            self.assertEqual(len(saved), 18)
            for row in records:
                self.assertEqual(row['expected_diagnostic'], row['observed_diagnostic'])
                base = original if row['target'] == 'execution' else replay
                changed = saved['tamper-' + row['name']]
                text, (a, b) = SPAN.value_span(base, row['path'])
                prefix, suffix = text[:a].encode(), text[b:].encode()
                self.assertTrue(changed.startswith(prefix) and changed.endswith(suffix))
                self.assertNotEqual(changed, base)
            changed = R.load_receipt(saved['tamper-integerKind'])
            self.assertIsInstance(changed['result']['runtime']['traces'][0]['deliveries'][0]['actual']['a'], Decimal)

    def test_tamper_controls_require_both_accepted_baselines_before_saving(self):
        with tempfile.TemporaryDirectory() as name:
            checker = self.checker()
            original, replay, inputs, manifest = fixture(checker, Path(name).resolve())
            save = mock.Mock()
            bad = R.load_receipt(replay);bad['replayed_from_sha256'] = 'sha256:' + '0' * 64
            with self.assertRaisesRegex(R.ReceiptError, '^REPLAY_RAW_PARENT$'):
                R.check_tampers(checker, original, packed(bad), R.REPORT_ENTRY, [0], inputs, manifest, GO, PRIOR, save, SPAN.value_span)
            save.assert_not_called()

    def test_unrelated_diagnostic_cannot_count_as_a_negative_pass(self):
        with tempfile.TemporaryDirectory() as name:
            checker = self.checker()
            original, replay, inputs, manifest = fixture(checker, Path(name).resolve())
            real_inspect = checker.inspect
            calls = 0
            def inspect(*args, **kwargs):
                nonlocal calls
                calls += 1
                if calls > 2:
                    raise R.ReceiptError('Input-only receipt/file binding')
                return real_inspect(*args, **kwargs)
            def save(name, raw):
                return {'path': '/unit/' + name, 'bytes': len(raw), 'sha256': hashlib.sha256(raw).hexdigest()}
            with mock.patch.object(checker, 'inspect', side_effect=inspect), self.assertRaisesRegex(R.ReceiptError, '^UNINTENDED_TAMPER_DIAGNOSTIC:derivedBoolean$'):
                R.check_tampers(checker, original, replay, R.REPORT_ENTRY, [0], inputs, manifest, GO, PRIOR, save, SPAN.value_span)

    def test_each_required_report_field_kind_value_and_identity_is_checked(self):
        with tempfile.TemporaryDirectory() as name:
            checker = self.checker();original, _, inputs, manifest = fixture(checker, Path(name).resolve())
            checker.inspect(original, R.REPORT_ENTRY, [0], inputs, manifest, GO)
            for field in R.FIELDS:
                changed = json.loads(original)
                actual = changed['result']['runtime']['traces'][0]['deliveries'][0]['actual']
                actual[field] = (not actual[field]) if type(actual[field]) is bool else actual[field] + 1
                with self.subTest(field=field), self.assertRaises(R.ReceiptError):
                    checker.inspect(packed(changed), R.REPORT_ENTRY, [0], inputs, manifest, GO)
            for field_index in range(11):
                changed = json.loads(original)
                changed['result']['runtime']['traces'][0]['deliveries'][0]['actual_fields'][field_index]['id'] += '/wrong'
                with self.subTest(identity=field_index), self.assertRaisesRegex(R.ReceiptError, '^REPORT_FIELD_IDENTITY$'):
                    checker.inspect(packed(changed), R.REPORT_ENTRY, [0], inputs, manifest, GO)

    def test_graph_source_abi_and_runtime_cohort_guards(self):
        with tempfile.TemporaryDirectory() as name:
            checker = self.checker();original, _, inputs, manifest = fixture(checker, Path(name).resolve())
            checker.inspect(original, R.REPORT_ENTRY, [0], inputs, manifest, GO)
            mutations = [
                (['result', 'program', 'pure_calls'], False),
                (['result', 'program', 'pure_calls', 'sites'], []),
                (['result', 'program', 'activities'], []),
                (['result', 'runtime', 'runs'], []),
                (['result', 'runtime', 'finite_passed'], True),
                (['result', 'runtime', 'traces', 0, 'case_index'], False),
                (['result', 'runtime', 'go_version'], 'go version go1.27.2 darwin/arm64'),
                (['result', 'composition', 'plan', 'activities', 0, 'inputs', 0, 'from'], False),
            ]
            for path, value in mutations:
                changed = R._raw_value(original, path, json.dumps(value), SPAN.value_span)
                with self.subTest(path=path), self.assertRaises(R.ReceiptError):
                    checker.inspect(changed, R.REPORT_ENTRY, [0], inputs, manifest, GO)
            for indices in ([True], [64], [1, 0], list(range(25))):
                with self.subTest(indices=indices), self.assertRaisesRegex(R.ReceiptError, '^FIXED_CLI_COHORT$'):
                    checker.inspect(original, R.REPORT_ENTRY, indices, inputs, manifest, GO)

    def test_constructor_binds_current_oracle_order_kind_and_original_prefix(self):
        self.checker()
        for mutate in ('index', 'input-kind', 'echo-kind', 'row-order'):
            rows = copy.deepcopy(self.oracle)
            if mutate == 'index':rows[0]['index'] = False
            elif mutate == 'input-kind':rows[0]['inputs']['a'] = False
            elif mutate == 'echo-kind':rows[0]['expected']['a'] = False
            else:rows[0], rows[1] = rows[1], rows[0]
            with self.subTest(mutate=mutate), self.assertRaises(R.ReceiptError):
                R.Evidence(REPO, rows, 'go version go1.27.2 linux/amd64')
        with self.assertRaisesRegex(R.ReceiptError, '^SELECTED_GO_VERSION$'):
            self.checker('go version go1.27.3 linux/amd64')

    def test_source_decoding_and_canonical_regular_file_boundary(self):
        body = 'return input0 >= 0 && input0 <= 255\n'
        raw = 'activity A(Integer) -> Boolean computes `' + body + '`'
        quoted = 'activity A(Integer) -> Boolean computes ' + json.dumps(body)
        self.assertEqual(R.activity_bodies(raw), R.activity_bodies(quoted))
        with self.assertRaisesRegex(R.ReceiptError, '^Repeated computes activity$'):
            R.activity_bodies(raw + '\n' + quoted)
        self.assertIn(b'\\u0026', R.go_string('a&b'))
        with tempfile.TemporaryDirectory() as name:
            directory = Path(name).resolve();target = directory / 'target';target.write_bytes(b'{}')
            alias = directory / 'alias';alias.symlink_to(target)
            with self.assertRaisesRegex(R.ReceiptError, '^CANONICAL_FILE_REQUIRED$'):
                R._read_regular(alias, 100)


if __name__ == '__main__':
    unittest.main()
