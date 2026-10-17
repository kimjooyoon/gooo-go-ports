#!/usr/bin/env python3
"""Bounded local failure diagnostics; never invoke Go or a network endpoint."""
import contextlib
import importlib.util
import io
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest import mock

SPEC = importlib.util.spec_from_file_location(
    "public_consumer", Path(__file__).with_name("check-api-policy-public-consumer.py"))
HELPER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(HELPER)


class PublicConsumerDiagnosticsTests(unittest.TestCase):
    def fixture(self, root, raw):
        stderr = root / "02-parent-query.stderr"
        stderr.write_bytes(raw)
        (root / "steps.json").write_text(json.dumps([
            {"name": "installed-go", "exit_code": 0},
            {"name": "parent-query", "exit_code": 1,
             "stderr": {"path": str(stderr)}}]))

    def test_failed_proxy_query_keeps_last_step_exit_and_reason(self):
        raw = (b"go: invalid version: reading https://proxy.golang.org/example/@v/revision.info: 404 Not Found\n"
               b"\tserver response: not found: fetch timed out\n")
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self.fixture(root, raw)
            detail = HELPER.last_step_diagnostic(root)
            self.assertEqual(detail["name"], "parent-query")
            self.assertEqual(detail["exit_code"], 1)
            self.assertEqual(detail["stderr_prefix"].encode(), raw)
            self.assertFalse(detail["stderr_truncated"])
            report = {"status": "FAIL", "error": "Refused: parent-query returned nonzero",
                      "last_step": detail}
            encoded = HELPER.console_summary(report, root / "verification.json")
            observed = json.loads(encoded)
            self.assertEqual(observed["status"], "FAIL")
            self.assertIn("fetch timed out", observed["last_step"]["stderr_prefix"])
            self.assertNotIn("acceptance", observed)

    def test_stderr_is_bounded_one_json_line_and_cannot_emit_commands(self):
        raw = (b"::error::not-a-workflow-command\n\x1b[31m" + "🙂".encode()) * 300
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self.fixture(root, raw)
            detail = HELPER.last_step_diagnostic(root)
            self.assertTrue(detail["stderr_truncated"])
            self.assertEqual(detail["stderr_prefix_limit_bytes"], 2048)
            self.assertEqual(detail["stderr_prefix"], raw[:2048].decode("utf-8", errors="replace"))
            encoded = HELPER.console_summary(
                {"status": "FAIL", "error": "x" * 2000, "last_step": detail},
                root / "verification.json")
            self.assertLessEqual(len(encoded.encode()), HELPER.MAX_FAILURE_CONSOLE)
            self.assertNotIn("\n", encoded)
            self.assertNotIn("\x1b", encoded)
            self.assertNotIn("::error::", encoded)
            decoded = json.loads(encoded)
            self.assertEqual(len(decoded["error"]), 512)
            self.assertEqual(decoded["last_step"]["stderr_prefix"], detail["stderr_prefix"])

    def test_missing_or_unsafe_diagnostic_cannot_replace_primary_failure(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self.assertIsNone(HELPER.last_step_diagnostic(root))
            (root / "steps.json").write_text("not json")
            self.assertIn("unavailable", HELPER.last_step_diagnostic(root))
            outside = root / "outside"
            outside.mkdir()
            secret = outside / "secret"
            secret.write_text("must-not-read")
            (root / "steps.json").write_text(json.dumps([
                {"name": "parent-query", "exit_code": 1, "stderr": {"path": str(secret)}}]))
            self.assertNotIn("must-not-read", json.dumps(HELPER.last_step_diagnostic(root)))
            actual = root / "actual.stderr"
            actual.write_text("also-not-read")
            link = root / "linked.stderr"
            link.symlink_to(actual)
            (root / "steps.json").write_text(json.dumps([
                {"name": "parent-query", "exit_code": 1, "stderr": {"path": str(link)}}]))
            self.assertNotIn("also-not-read", json.dumps(HELPER.last_step_diagnostic(root)))

    def test_success_summary_remains_exact_even_for_expected_negative_exit(self):
        report = {"status": "PUBLIC_API_POLICY_CONSUMER_PASS", "last_step": {"exit_code": 1}}
        verification = Path("out::literal/verification.json")
        expected = json.dumps({"status": report["status"], "verification": str(verification)})
        self.assertEqual(HELPER.console_summary(report, verification), expected)

    def test_main_preserves_fail_exit_and_has_no_acceptance_after_diagnostic(self):
        with tempfile.TemporaryDirectory() as directory:
            target = Path(directory) / "fresh"
            args = ["check", "--go", "/not-invoked/go", "--revision", "a" * 40, "--out", str(target)]
            output = io.StringIO()
            with mock.patch.object(sys, "argv", args), \
                 mock.patch.object(HELPER, "verify", side_effect=HELPER.Refused("original failure")), \
                 mock.patch.object(HELPER, "last_step_diagnostic", return_value={"unavailable": "OSError"}), \
                 contextlib.redirect_stdout(output):
                code = HELPER.main()
            self.assertEqual(code, 1)
            report = json.loads((target / "verification.json").read_text())
            self.assertEqual(report["status"], "FAIL")
            self.assertEqual(report["error"], "Refused: original failure")
            self.assertNotIn("acceptance", report)
            self.assertEqual(json.loads(output.getvalue())["status"], "FAIL")

    def test_subprocess_failure_is_not_retried(self):
        with tempfile.TemporaryDirectory() as directory:
            recorder = HELPER.Recorder(Path(directory))
            with mock.patch.object(recorder, "run", return_value=(1, b"", b"fetch timed out")) as run:
                with self.assertRaises(HELPER.Refused):
                    recorder.success("parent-query", ["not-invoked"], Path(directory), {})
            run.assert_called_once()


class PublicConsumerIdentityTests(unittest.TestCase):
    def source(self):
        root = Path(__file__).resolve().parents[1]
        return (root / "testdata/api-policy-consumer/main.go.txt").read_bytes()

    def test_current_declared_consumer_identities_match_before_process_work(self):
        with mock.patch.object(HELPER.subprocess, "run", side_effect=AssertionError("no process")):
            self.assertIsNone(HELPER.verify_fixture_identities(self.source()))

    def test_prior_readonly_generation_is_refused(self):
        raw = self.source()
        old = b"sha256:a820e1c4cc249086472e792d31d12a9628bdfb56899bfc705efeca960893b64e"
        mutant = raw.replace(HELPER.PARENT_GENERATION.encode(), old, 1)
        self.assertNotEqual(mutant, raw)
        with self.assertRaisesRegex(HELPER.Refused, "expectedRevisionGeneration"):
            HELPER.verify_fixture_identities(mutant)

    def test_missing_duplicate_or_changed_identity_declarations_are_refused(self):
        raw = self.source()
        for name in ("expectedCompiler", "expectedOriginalGeneration", "expectedRevisionGeneration"):
            line = next(line for line in raw.splitlines(keepends=True)
                        if line.lstrip().startswith(name.encode() + b" "))
            for mutation in ("missing", "duplicate", "changed"):
                replacement = b"" if mutation == "missing" else line + line
                if mutation == "changed":
                    replacement = line[:line.index(b'"') + 1] + b"changed\"\n"
                mutant = raw.replace(line, replacement, 1)
                self.assertNotEqual(mutant, raw)
                with self.subTest(name=name, mutation=mutation), self.assertRaises(HELPER.Refused):
                    HELPER.verify_fixture_identities(mutant)

    def test_commented_identity_does_not_replace_declaration(self):
        raw = self.source()
        line = next(line for line in raw.splitlines(keepends=True)
                    if line.lstrip().startswith(b"expectedRevisionGeneration "))
        mutant = raw.replace(line, b"// " + line, 1)
        self.assertNotEqual(mutant, raw)
        with self.assertRaises(HELPER.Refused):
            HELPER.verify_fixture_identities(mutant)

    def test_fixture_preflight_is_unconditional_before_recorder_or_module_queries(self):
        import ast
        source = Path(HELPER.__file__).read_text()
        tree = ast.parse(source)
        verify = next(node for node in tree.body if isinstance(node, ast.FunctionDef) and node.name == "verify")
        calls = [node.value for node in verify.body if isinstance(node, ast.Expr)
                 and isinstance(node.value, ast.Call) and isinstance(node.value.func, ast.Name)
                 and node.value.func.id == "verify_fixture_identities"]
        self.assertEqual(len(calls), 1)
        self.assertEqual([node.id for node in calls[0].args], ["fixture_raw"])
        self.assertFalse(calls[0].keywords)
        recorder = next(node for node in verify.body if isinstance(node, ast.Assign)
                        and isinstance(node.value, ast.Call) and isinstance(node.value.func, ast.Name)
                        and node.value.func.id == "Recorder")
        self.assertLess(calls[0].lineno, recorder.lineno)


if __name__ == "__main__":
    unittest.main()
