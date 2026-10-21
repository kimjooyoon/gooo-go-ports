#!/usr/bin/env python3
"""Exercise the actual composite's append wrapper without Go or a GitHub runner.

The controlled late-mutation child is a transport fault fixture, not a policy
implementation or evidence that a GitHub job ran. Hosted composite checks remain
required. Never import either presentation program into this checker.
"""
import ast
import hashlib
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import textwrap
import unittest


WRITE_FIXTURE_SHA = '49df4793058fcdf06906f0123ff1da3f2b538b3c78d4a8973f4c4c50a7b8db32'
PREFIX = b'An existing synthetic job section.\n'
MUTATION = b'A controlled late transport mutation.\n'
EXPECTED = (
    '### Gooo readonly policy review\n\n'
    '| Result | Value |\n| --- | --- |\n'
    '| Classification | `NEWLY_DENIED` |\n'
    '| Changed fields | 2 |\n'
    '| Before reason | `ALLOW` |\n'
    '| After reason | `ACTION_NOT_READ_ONLY` |\n\n'
    'Changed field names: `allowed`, `reason`.\n\n'
    'Two supplied sealed profiles; this summary does not authorize a live action.\n'
).encode()


class AppendChecks(unittest.TestCase):
    action = formatter = fixture = None

    @classmethod
    def setUpClass(cls):
        # Extract the one real composite Python body, retaining its executable
        # bytes after YAML block indentation. No alternative wrapper is tested.
        action = cls.action.read_text()
        start = "        python3 -I -B - <<'PY'\n"
        end = '        PY\n'
        if action.count(start) != 1 or action.count(end) != 1:
            raise AssertionError('expected one composite append wrapper')
        body = action.split(start, 1)[1].split(end, 1)[0]
        cls.wrapper = textwrap.dedent(body).encode()
        ast.parse(cls.wrapper)
        cls.formatter_raw = cls.formatter.read_bytes()
        cls.fixture_raw = cls.fixture.read_bytes()
        if hashlib.sha256(cls.fixture_raw).hexdigest() != WRITE_FIXTURE_SHA:
            raise AssertionError('known synthetic report changed')

    def exercise(self, late_mutation):
        with tempfile.TemporaryDirectory(prefix='gooo-summary-append-check-') as work:
            root = Path(work).resolve()
            os.chmod(root, 0o700)
            result = root / 'work' / 'result'
            result.mkdir(parents=True, mode=0o700)
            report = result / 'comparison.json'
            report.write_bytes(self.fixture_raw)
            report.chmod(0o600)
            job_summary = root / 'runner-summary.md'
            job_summary.write_bytes(PREFIX)
            job_summary.chmod(0o600)
            action = root / 'action'
            action.mkdir(mode=0o700)
            if late_mutation:
                # This deterministic child runs after the parent preflight. It
                # emits bounded safe text but invalidates the append checkpoint.
                # A stale early summary.md must not imply successful append.
                child = (
                    'import os, sys\n'
                    'with open(os.environ["GITHUB_STEP_SUMMARY"], "ab") as stream:\n'
                    '    stream.write(' + repr(MUTATION) + ')\n'
                    'sys.stdout.buffer.write(b"Safe synthetic formatter bytes.\\n")\n'
                ).encode()
            else:
                child = self.formatter_raw
            (action / 'summary.py').write_bytes(child)
            wrapper = root / 'append.py'
            wrapper.write_bytes(self.wrapper)
            env = {name: value for name, value in os.environ.items()
                   if name in ('PATH', 'LANG', 'LC_ALL', 'SYSTEMROOT')}
            env.update(RUNNER_TEMP=str(root), TMPDIR=str(root),
                       GITHUB_STEP_SUMMARY=str(job_summary), GITHUB_ACTION_PATH=str(action),
                       GOOO_SUMMARY_REPORT_PATH=str(report), GOOO_SUMMARY_REPORT_SHA256=WRITE_FIXTURE_SHA)
            process = subprocess.run([sys.executable, '-I', '-B', str(wrapper)],
                                     stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                                     stderr=subprocess.PIPE, env=env, cwd=root,
                                     timeout=20, check=False)
            self.assertEqual(report.read_bytes(), self.fixture_raw)
            self.assertEqual(wrapper.read_bytes(), self.wrapper)
            self.assertEqual((action / 'summary.py').read_bytes(), child)
            self.assertEqual(process.stdout, b'')
            saved = result / 'summary.md'
            if late_mutation:
                self.assertEqual(process.returncode, 2)
                self.assertEqual(process.stderr, b'Gooo policy summary: SUMMARY_RENDER_OR_APPEND_REFUSED\n')
                self.assertEqual(job_summary.read_bytes(), PREFIX + MUTATION)
                self.assertFalse(saved.exists(), 'failed append left a misleading completion file')
            else:
                self.assertEqual(process.returncode, 0)
                self.assertEqual(process.stderr, b'')
                self.assertEqual(job_summary.read_bytes(), PREFIX + EXPECTED)
                self.assertEqual(saved.read_bytes(), EXPECTED)
                self.assertEqual(saved.stat().st_mode & 0o777, 0o600)

    def test_actual_wrapper_preserves_prefix_and_records_completed_safe_append(self):
        self.exercise(False)

    def test_late_append_refusal_does_not_leave_completion_for_expected_failure(self):
        self.exercise(True)


if __name__ == '__main__':
    if len(sys.argv) != 4:
        raise SystemExit('usage: check_summary_action.py ACTION_YAML FORMATTER_PY SYNTHETIC_REPORT_JSON')
    AppendChecks.action, AppendChecks.formatter, AppendChecks.fixture = [Path(name).resolve(strict=True) for name in sys.argv[1:]]
    unittest.main(argv=[sys.argv[0]], verbosity=2)
