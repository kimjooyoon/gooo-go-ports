#!/usr/bin/env python3
"""Fixture/ABI/envelope controls, without a compiler, network or subprocess."""
import copy
import importlib.util
from pathlib import Path
import unittest

PATH = Path(__file__).resolve().with_name('verify.py')
SPEC = importlib.util.spec_from_file_location('versioned_grant_verifier', PATH)
V = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(V)


class FixtureControls(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.root = PATH.parents[2]
        cls.master = V.decoded(PATH.with_name('cases.json').read_bytes())
        cls.manifest = V.decoded((cls.root / V.MANIFEST).read_bytes())
        cls.sources = {n: (cls.root / n).read_bytes() for p in cls.manifest['packages'] for n in p['sources']}
        cls.contract = V.decoded(PATH.with_name('contract.json').read_bytes())

    def test_original48_types_and_order(self):
        self.assertEqual(V.H.sha(PATH.with_name('cases.json').read_bytes()), V.CORPUS_SHA)
        self.assertEqual(len(self.master['cases']), 48)
        for row in self.master['cases']:
            values, expected = V.case_values(row)
            self.assertEqual(len(values), 6)
            self.assertEqual(len(expected), 12)
        self.assertEqual(self.master['cases'][0]['expected'][V.ENTRY]['status'], 'ELIGIBLE')
        self.assertEqual(self.master['cases'][1]['expected'][V.ENTRY]['reason'], 'POLICY_VERSION_CHANGED')
        matrix = V.decoded(V.result_matrix(self.master['cases'][:2]))
        self.assertEqual(len(matrix), 2)
        self.assertEqual(list(matrix[0][0]), [n for n, _ in V.FIELDS])

    def test_duplicate_decoded_names(self):
        for raw in (b'{"x":1,"x":2}', b'{"x":1,"\\u0078":2}', b'{"nested":{"a":1,"a":2}}'):
            with self.subTest(raw=raw), self.assertRaises((RuntimeError, ValueError)):
                V.decoded(raw)

    def test_only_utf8_json(self):
        for raw in (b'{"x":"\xff"}', '{"x":1}'.encode('utf-16'), b'{"x":NaN}'):
            with self.subTest(raw=raw), self.assertRaises((UnicodeError, RuntimeError, ValueError)):
                V.decoded(raw)

    def test_missing_extra_and_wrong_text(self):
        key = V.ENTRY + '.input0'
        for change in ('missing', 'extra', 'bool', 'surrogate', 'too_long'):
            row = copy.deepcopy(self.master['cases'][0])
            if change == 'missing': del row['inputs'][key]
            if change == 'extra': row['inputs']['extra'] = ''
            if change == 'bool': row['inputs'][key] = True
            if change == 'surrogate': row['inputs'][key] = '\ud800'
            if change == 'too_long': row['inputs'][key] = 'é' * 513
            with self.subTest(change=change), self.assertRaises((RuntimeError, UnicodeError)):
                V.case_values(row)

    def test_bool_integer_and_result_contract_refusals(self):
        for field, bad in [('version_match', 1), ('if_match_condition', True), ('reason', None), ('status', []), ('if_match_condition', 1 << 63)]:
            row = copy.deepcopy(self.master['cases'][0]); row['expected'][V.ENTRY][field] = bad
            with self.subTest(field=field, bad=bad), self.assertRaises(RuntimeError): V.case_values(row)

    def test_semantic128_is_not_raw_ingress_limit(self):
        row = copy.deepcopy(self.master['cases'][0]); row['inputs'][V.ENTRY + '.input2'] = 'a' * 129
        values, _ = V.case_values(row)
        self.assertEqual(len(values[2]), 129)

    def test_joint_nul_envelope_is_not_full_cartesian_domain(self):
        row = copy.deepcopy(self.master['cases'][0])
        for k in row['inputs']: row['inputs'][k] = '\x00' * 1024
        V.case_values(row)
        document = {'schema': self.master['schema'], 'cases': [row]}
        self.assertGreater(V.wire_upper(document, sum(map(len, self.sources.values()))), 32768)
        with self.assertRaises(RuntimeError): V.wire_upper({'cases': []}, 1)
        with self.assertRaises(RuntimeError): V.wire_upper(document, 0)

    def test_source_closure_and_contract(self):
        defs, records, imports = V.definitions(self.manifest, self.sources)
        self.assertEqual(len(imports), 12); self.assertEqual(len(records), 9)
        self.assertEqual(sum(len(r['fields']) for r in records.values()), 61)
        self.assertEqual(len(defs[V.ENTRY]['sites']), 4)
        local = defs['gooo/service-domain:ValidateServicePattern']['sites']
        self.assertEqual([x['callee'] for x in local], ['gooo/service-domain:ValidateServiceType', 'gooo/service-domain:ValidateServiceName'])
        V.check_contract(self.contract, self.manifest, self.sources)
        for field in ('sources', 'output_fields', 'input_order', 'records'):
            c = copy.deepcopy(self.contract); c[field].reverse()
            with self.subTest(field=field), self.assertRaises(RuntimeError): V.check_contract(c, self.manifest, self.sources)
        c = copy.deepcopy(self.contract); c['wire_bounds']['initial_max_rows_per_batch'] = 128
        with self.assertRaises(RuntimeError): V.check_contract(c, self.manifest, self.sources)


if __name__ == '__main__': unittest.main()
