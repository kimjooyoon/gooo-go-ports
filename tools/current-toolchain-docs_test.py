#!/usr/bin/env python3
# Copyright (c) 2026 Gooo Go Ports contributors. All rights reserved.
# Use of this source code is governed by a BSD-style license in LICENSE.
"""Exercise source/profile drift, detached output and historical-byte boundaries."""
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location('toolchain_docs', ROOT / 'tools/current-toolchain-docs.py')
DOCS = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(DOCS)


class ToolchainDocs(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='gooo doc facts ')
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name)
        self.root = self.base / 'source'
        for module in DOCS.MODULES:
            for name in ('compiler.json', 'generation.json', 'go.mod', 'generate.py', 'verify.py', 'README.md'):
                relative = Path('examples') / module / name
                target = self.root / relative
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(ROOT / relative, target)

    def path(self, module, name):
        return self.root / 'examples' / module / name

    def rebind_fixture_asset(self, module, name):
        """Keep the fixture manifest coherent to test a deeper profile conflict."""
        manifest = self.path(module, 'generation.json')
        value = json.loads(manifest.read_text())
        value['assets'][name] = 'sha256:' + hashlib.sha256(self.path(module, name).read_bytes()).hexdigest()
        manifest.write_text(json.dumps(value))

    def identity(self, module):
        return json.loads(self.path(module, 'compiler.json').read_text())

    def mutate_text(self, path, before, after):
        original = path.read_text()
        self.assertIn(before, original)
        changed = original.replace(before, after)
        self.assertNotEqual(changed, original)
        path.write_text(changed)

    def refused_before_output(self):
        target = self.base / 'candidate'
        with self.assertRaises(DOCS.Refused):
            DOCS.generate(self.root, target)
        self.assertFalse(target.exists())

    def test_generation_is_detached_and_preserves_unmarked_bytes(self):
        before = {str(p.relative_to(self.root)): p.read_bytes() for p in self.root.rglob('*') if p.is_file()}
        out = self.base / 'candidate'
        outputs = DOCS.generate(self.root, out)
        self.assertEqual(len(outputs), 2)
        for relative, raw in outputs.items():
            original = before[relative]
            self.assertEqual(original.split(DOCS.START)[0], raw.split(DOCS.START)[0])
            self.assertEqual(original.split(DOCS.END)[1], raw.split(DOCS.END)[1])
            self.assertEqual((out / relative).read_bytes(), raw)
        self.assertEqual(before, {str(p.relative_to(self.root)): p.read_bytes() for p in self.root.rglob('*') if p.is_file()})
        proof = json.loads((out / 'generation.provenance.jsonld').read_text())
        self.assertFalse(proof['source_modified'])
        activities = [r for r in proof['@graph'] if r['@type'] == 'prov:Activity']
        self.assertEqual(len(activities), 1)
        generated = [r for r in proof['@graph'] if 'prov:wasGeneratedBy' in r]
        self.assertEqual(len(generated), 2)
        for row in generated:
            self.assertEqual(row['prov:wasGeneratedBy']['@id'], activities[0]['@id'])
            self.assertNotIn(row['@id'], [r['@id'] for r in row['prov:wasDerivedFrom']])

    def test_repeated_generation_has_same_bytes_and_distinct_activities(self):
        first, second = self.base / 'one', self.base / 'two'
        self.assertEqual(DOCS.generate(self.root, first), DOCS.generate(self.root, second))
        activities = []
        for folder in (first, second):
            proof = json.loads((folder / 'generation.provenance.jsonld').read_text())
            activities.append(next(r['@id'] for r in proof['@graph'] if r['@type'] == 'prov:Activity'))
        self.assertNotEqual(*activities)

    def test_bound_asset_drift_is_refused(self):
        path = self.path('agentmesh-service-policy', 'generate.py')
        path.write_bytes(path.read_bytes() + b'\n# unrecorded source change\n')
        self.refused_before_output()

    def test_rehashed_generator_pin_conflict_is_refused(self):
        module = 'agentmesh-service-policy'
        path = self.path(module, 'generate.py')
        revision = self.identity(module)['compiler_source_sha']
        wrong = 'a' * 40 if revision != 'a' * 40 else 'b' * 40
        self.mutate_text(path, revision, wrong)
        self.rebind_fixture_asset(module, 'generate.py')
        self.refused_before_output()

    def test_rehashed_verifier_go_conflict_is_refused(self):
        module = 'service-grant-many'
        path = self.path(module, 'verify.py')
        go = self.identity(module)['go_version']
        prefix, patch = go.rsplit('.', 1)
        self.mutate_text(path, go, prefix + '.' + str(int(patch) + 1))
        self.rebind_fixture_asset(module, 'verify.py')
        self.refused_before_output()

    def test_many_generation_identity_conflict_is_refused(self):
        path = self.path('service-grant-many', 'generation.json')
        value = json.loads(path.read_text())
        revision = value['compiler_source_sha']
        value['compiler_source_sha'] = 'a' * 40 if revision != 'a' * 40 else 'b' * 40
        path.write_text(json.dumps(value))
        self.refused_before_output()

    def test_nonobject_metadata_is_refused(self):
        self.path('agentmesh-service-policy', 'compiler.json').write_text('[]')
        self.refused_before_output()

    def test_nonliteral_pin_is_refused_without_execution(self):
        module = 'agentmesh-service-policy'
        path = self.path(module, 'generate.py')
        revision = self.identity(module)['compiler_source_sha']
        self.mutate_text(path, "PIN='" + revision + "'", 'PIN=unknown_call()')
        self.rebind_fixture_asset(module, 'generate.py')
        self.refused_before_output()

    def test_unknown_generation_schema_is_refused(self):
        path = self.path('agentmesh-service-policy', 'generation.json')
        value = json.loads(path.read_text())
        value['schema'] = 'unknown/v2'
        path.write_text(json.dumps(value))
        self.refused_before_output()

    def test_duplicate_fact_key_is_refused(self):
        path = self.path('service-grant-many', 'compiler.json')
        go = self.identity('service-grant-many')['go_version']
        path.write_text('{"go_version":' + json.dumps(go) + ',' + path.read_text().lstrip()[1:])
        self.refused_before_output()

    def test_missing_or_duplicate_markers_in_second_consumer_leave_no_output(self):
        path = self.path('service-grant-many', 'README.md')
        raw = path.read_bytes()
        for modified in (raw.replace(DOCS.END, b''), raw + DOCS.START + DOCS.END):
            with self.subTest(modified=len(modified)):
                path.write_bytes(modified)
                self.refused_before_output()

    def test_stale_doc_check_refuses_without_altering_source(self):
        self.assertEqual(DOCS.check(self.root), 2)
        path = self.path('service-grant-many', 'README.md')
        raw = path.read_bytes()
        prefix, rest = raw.split(DOCS.START)
        block, suffix = rest.split(DOCS.END)
        stale = prefix + DOCS.START + b'stale toolchain instructions\n' + DOCS.END + suffix
        path.write_bytes(stale)
        with self.assertRaises(DOCS.Refused):
            DOCS.check(self.root)
        self.assertEqual(path.read_bytes(), stale)

    def test_existing_output_is_not_replaced(self):
        out = self.base / 'candidate'
        out.mkdir()
        sentinel = out / 'keep'
        sentinel.write_bytes(b'existing user file')
        with self.assertRaises(DOCS.Refused):
            DOCS.generate(self.root, out)
        self.assertEqual(sentinel.read_bytes(), b'existing user file')
        self.assertEqual(list(out.iterdir()), [sentinel])

    def test_output_inside_source_tree_is_refused(self):
        out = self.root / 'candidate'
        with self.assertRaises(DOCS.Refused):
            DOCS.generate(self.root, out)
        self.assertFalse(out.exists())


if __name__ == '__main__':
    unittest.main()
