"""Pure exact-ABI parser controls; no compiler, native program or external IO."""
from pathlib import Path
import copy
import importlib.util
import unittest


_path = Path(__file__).with_name('receipts.py')
_spec = importlib.util.spec_from_file_location('retry_after_receipts_under_test', _path)
R = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(R)


class CompleteStructBodyTests(unittest.TestCase):
    def fixture(self):
        records = [
            {'name': 'SourceA', 'id': 'gooo://synthetic/a', 'go_name': 'RecordA',
             'fields': [{'name': 'input', 'id': 'gooo://synthetic/a/input',
                         'type_id': 'urn:gooo:type:string', 'go_name': 'FieldA'}]},
            {'name': 'SourceB', 'id': 'gooo://synthetic/b', 'go_name': 'RecordB',
             'fields': [{'name': 'value', 'id': 'gooo://synthetic/b/value',
                         'type_id': 'urn:gooo:type:integer', 'go_name': 'FieldB'}]},
        ]
        source = '''package gooo_workspace

type RecordA struct {
\tFieldA string `json:"input"`
}

type RecordB struct {
\tFieldB int64 `json:"value"`
}

func Helper(input string) RecordA {
\treturn RecordA{FieldA: input}
}

func Entry(input0 string) RecordB {
\treturn RecordB{FieldB: 0}
}
'''
        return source, ['Helper', 'Entry'], records

    def assert_record_refusal(self, source, names, records):
        with self.assertRaises(R.ReceiptError) as raised:
            R.go_parts(source, names, records)
        self.assertEqual(raised.exception.boundary, 'record_abi')

    def test_valid_two_records_and_two_functions(self):
        source, names, records = self.fixture()
        before = copy.deepcopy(records)
        functions = R.go_parts(source, names, records)
        self.assertEqual(set(functions), {'Helper', 'Entry'})
        self.assertEqual(functions['Helper'][:2], ('input string', 'RecordA'))
        self.assertEqual(functions['Entry'][:2], ('input0 string', 'RecordB'))
        self.assertEqual(records, before)

    def test_abi_neutral_blank_lines_are_allowed(self):
        source, names, records = self.fixture()
        changed = source.replace('type RecordA struct {\n', 'type RecordA struct {\n\t \n\n', 1)
        self.assertEqual(R.go_parts(changed, names, records), R.go_parts(source, names, records))

    def test_every_nonempty_struct_line_is_consumed_and_exact(self):
        source, names, records = self.fixture()
        mutations = {
            'extra-any-json-ignored': '\tInjected any `json:"-"`',
            'extra-allowed-scalar': '\tInjected string `json:"injected"`',
            'tagless': '\tInjected int64',
            'blank-json-name': '\tInjected string `json:""`',
            'comment-line': '\t// this line must not disappear from ABI checking',
            'nonfield-statement': '\tpanic("not a field")',
            'duplicate-field': '\tFieldA string `json:"input"`',
            'embedded-field': '\tForeignRecord',
            'trailing-unconsumed-token': '\tInjected string `json:"injected"` unexpected',
            'two-declarations-on-one-line': '\tInjected string `json:"injected"`; Hidden any',
        }
        for name, injected in mutations.items():
            with self.subTest(mutation=name):
                changed = source.replace('type RecordA struct {\n', 'type RecordA struct {\n' + injected + '\n', 1)
                self.assert_record_refusal(changed, names, records)
        for name, changed in {
            'expected-field-replaced-by-blank': source.replace('\tFieldA string `json:"input"`', '\t '),
            'reviewed-field-kind-changed': source.replace('\tFieldA string `json:"input"`', '\tFieldA bool `json:"input"`'),
            'reviewed-json-name-changed': source.replace('`json:"input"`', '`json:"different"`'),
        }.items():
            with self.subTest(mutation=name):
                self.assert_record_refusal(changed, names, records)


class ReceiptTransportAndProducerTests(unittest.TestCase):
    def identity(self, version):
        return {'schema': 'gooo/build-identity/v1', 'compiler_source_sha': R.REV,
                'vcs_revision': R.REV, 'source_status': 'CLEAN_VCS', 'vcs_modified': 'false',
                'go_version': 'go1.27.3', 'native_go_required': 'go1.27.3',
                'build_info_observed': True, 'version': '0.6.37-dev',
                'module_version': version, 'module': {'path': R.PRODUCER_PATH, 'version': version},
                'decision_runtime': {'module_path': 'github.com/kimjooyoon/gooo-decision-runtime',
                                     'version': 'v0.2.51-experimental', 'replaced': False}}

    def test_clean_bootstrap_and_installed_module_keep_exact_observed_identity(self):
        for version in R.PRODUCER_VERSIONS:
            identity = self.identity(version)
            selected = R.producer_module(identity)
            self.assertEqual(selected, identity['module'])
            selected['version'] = 'foreign'
            self.assertEqual(identity['module']['version'], version)

    def test_old_dirty_replaced_or_cross_bound_producer_is_refused(self):
        original = self.identity(R.PRODUCER_VERSIONS[0])
        mutations = [
            lambda x: x.update(compiler_source_sha='0'*40),
            lambda x: x.update(vcs_revision='0'*40),
            lambda x: x.update(vcs_modified='true'),
            lambda x: x.update(source_status='DIRTY_VCS'),
            lambda x: x.update(go_version='go1.27.2'),
            lambda x: x.update(native_go_required='go1.27.2'),
            lambda x: x.update(build_info_observed=False),
            lambda x: x.update(version='0.6.32-dev'),
            lambda x: x['decision_runtime'].update(replaced=True),
            lambda x: x['decision_runtime'].update(version='v0.2.50-experimental'),
            lambda x: x['module'].update(path='github.com/foreign/compiler'),
            lambda x: x['module'].update(version='v0.0.0-20261020180505-000000000000'),
            lambda x: x.update(module_version=R.PRODUCER_VERSIONS[1]),
        ]
        for mutate in mutations:
            changed = copy.deepcopy(original)
            mutate(changed)
            with self.subTest(identity=changed), self.assertRaises(R.ReceiptError):
                R.producer_module(changed)

    def test_receipt_budget_accepts_full_bound_and_refuses_one_byte_over(self):
        self.assertEqual(R.load(b'{}' + b' ' * ((8 << 20) - 2)), {})
        for raw in (b'', b'{}' + b' ' * ((8 << 20) - 1)):
            with self.assertRaises(R.ReceiptError) as raised:
                R.load(raw)
            self.assertEqual(raised.exception.boundary, 'json')


if __name__ == '__main__':
    unittest.main()
