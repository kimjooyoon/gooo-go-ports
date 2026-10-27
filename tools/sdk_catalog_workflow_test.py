"""Guard actual SDK tests and independent native generation without weakening CI."""
import hashlib
import importlib.util
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('sdk_catalog_workflow', HERE/'sdk_catalog_workflow.py')
s = importlib.util.module_from_spec(spec)
spec.loader.exec_module(s)
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


if __name__ == '__main__':
    unittest.main()
