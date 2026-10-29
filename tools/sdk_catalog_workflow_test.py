"""Guard actual SDK tests and independent native generation without weakening CI."""
import hashlib
import importlib.util
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('sdk_catalog_workflow', HERE/'sdk_catalog_workflow.py')
s = importlib.util.module_from_spec(spec)
spec.loader.exec_module(s)
portable_spec = importlib.util.spec_from_file_location('portable_workflow_snapshot_workflow', HERE/'portable_workflow_snapshot_workflow.py')
p = importlib.util.module_from_spec(portable_spec)
portable_spec.loader.exec_module(p)
WORKFLOW = HERE.parent/'.github/workflows/ci.yml'


class SDKWorkflowTests(unittest.TestCase):
    def test_only_exact_additions_recover_previous_workflow(self):
        previous = s.project(WORKFLOW.read_text())
        self.assertEqual(hashlib.sha256(previous.encode()).hexdigest(), s.PRIOR_SHA256)
        self.assertNotIn('sdk-catalog-refresh', previous)

    def test_missing_duplicate_moved_or_weakened_checks_fail(self):
        raw = WORKFLOW.read_text()
        replacements = [
            (s.COMMON, ''), (s.COMMON, s.COMMON*2),
            (s.DECLARATION, ''), (s.BUILD_VERIFY, ''), (s.UPLOAD, ''),
            (s.COMMON, s.COMMON.replace('-race ', '')),
            (s.BUILD_VERIFY, s.BUILD_VERIFY.replace("!= 'true'", "== 'true'", 1)),
            (s.BUILD_VERIFY, s.BUILD_VERIFY.replace('        timeout-minutes: 5', '        continue-on-error: true\n        timeout-minutes: 5')),
            (s.BUILD_VERIFY, s.BUILD_VERIFY.replace('--out "$RUNNER_TEMP/sdk-catalog-refresh"', '--write-generated --out "$RUNNER_TEMP/sdk-catalog-refresh"')),
            (s.BUILD_VERIFY, s.BUILD_VERIFY.replace('.replay_cases == 8 and ', '')),
            (s.UPLOAD, s.UPLOAD.replace('always() && ', '')),
            (s.UPLOAD, s.UPLOAD.replace('sdk-catalog-refresh-evidence', 'old-evidence')),
        ]
        for before, after in replacements:
            with self.subTest(after=after[:100]), self.assertRaises(ValueError):
                s.project(raw.replace(before, after, 1))
        moved = raw.replace(s.BUILD_VERIFY+s.UPLOAD, '', 1)+s.BUILD_VERIFY+s.UPLOAD
        with self.assertRaises(ValueError):
            s.project(moved)
        changed_base = raw.replace('go test -race -timeout=15m', 'go test -timeout=15m', 1)
        with self.assertRaises(ValueError):
            s.project(changed_base)

    def test_current_portable_layer_is_required_and_recovers_historical_sdk_source(self):
        raw = WORKFLOW.read_text()
        historical = p.project(raw)
        self.assertEqual(hashlib.sha256(historical.encode()).hexdigest(), p.PRIOR_SHA256)
        self.assertNotIn(p.STEP, historical)
        self.assertEqual(s.project(historical), s.project(raw))
        self.assertEqual(hashlib.sha256(s.project(historical).encode()).hexdigest(), s.PRIOR_SHA256)

    def test_portable_layer_cannot_be_deleted_moved_or_weakened(self):
        raw = WORKFLOW.read_text()
        mutants = {
            'deleted': raw.replace(p.STEP, '', 1),
            'duplicated': raw.replace(p.STEP, p.STEP * 2, 1),
            'moved': raw.replace(p.STEP, '', 1) + p.STEP,
            'conditional': raw.replace(p.STEP, p.STEP.replace('        run:', '        if: false\n        run:'), 1),
            'continue-on-error': raw.replace(p.STEP, p.STEP.replace('        run:', '        continue-on-error: true\n        run:'), 1),
            'command-weakened': raw.replace(p.STEP, p.STEP.replace('-I -B ', ''), 1),
            'command-replaced': raw.replace(p.STEP, p.STEP.replace('python3 -I -B tools/package-gooo-workflows_test.py -v', 'true'), 1),
            'baseline-weakened': raw.replace('go test -race -timeout=15m', 'go test -timeout=15m', 1),
        }
        for name, mutant in mutants.items():
            with self.subTest(name=name), self.assertRaises(ValueError):
                p.project(mutant)
            if name != 'deleted':
                with self.subTest(sdk_chain=name), self.assertRaises(ValueError):
                    s.project(mutant)


if __name__ == '__main__':
    unittest.main()
