#!/usr/bin/env python3
"""Black-box checks for the policy report's deliberately limited presentation.

Run explicitly: python3 -I -B check_summary.py /absolute/path/to/summary.py
No formatter import, policy calculation, Gooo run, build, or network is used.
The frozen write fixture is an existing saved comparison. The read fixture is
an explicitly synthetic literal display case, NOT a new Gooo observation.
"""

import copy
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


FIXTURE_SHA256 = "49df4793058fcdf06906f0123ff1da3f2b538b3c78d4a8973f4c4c50a7b8db32"
REPORT_CAP = 16 << 20
OUTPUT_CAP = 4096
SECRET = "SUMMARY_PRIVACY_SENTINEL_b5d198af_"
ERROR_PREFIX = b"Gooo policy summary: "
DECISION_NAMES = (
    "allowed", "reason", "authenticated", "tenant_present", "tenant_matched",
    "action_present", "action_matched", "access_allowed", "access_error",
    "access_result", "group_matched", "principal_matched", "group_missing",
    "principal_defaulted", "effective_principal",
)
CLASSIFICATIONS = (
    "NEWLY_ALLOWED", "NEWLY_DENIED", "REASON_CHANGED",
    "OBSERVATIONS_CHANGED", "UNCHANGED",
)
REASONS = (
    "ALLOW", "NOT_AUTHENTICATED", "TENANT_UNSPECIFIED", "TENANT_MISMATCH",
    "ACTION_UNSPECIFIED", "ACTION_MISMATCH", "ACCESS_ERROR", "ACCESS_DENIED",
    "ACTION_NOT_READ_ONLY",
)


def raw_json(value):
    return json.dumps(value, ensure_ascii=False, separators=(",", ":")).encode("utf-8")


def sha256(raw):
    return hashlib.sha256(raw).hexdigest()


def replace_at(value, path, replacement):
    current = value
    for part in path[:-1]:
        current = current[part]
    current[path[-1]] = replacement


class SummaryChecks(unittest.TestCase):
    formatter = None
    saved_raw = None
    saved_write = None

    @classmethod
    def setUpClass(cls):
        fixture = Path(__file__).resolve().parent / "fixtures" / "synthetic-write.comparison.json"
        cls.saved_raw = fixture.read_bytes()
        if len(cls.saved_raw) != 14466 or sha256(cls.saved_raw) != FIXTURE_SHA256:
            raise AssertionError("frozen saved write fixture changed")
        cls.saved_write = json.loads(cls.saved_raw)
        # These are existing literal facts, not values obtained from formatter.
        write = cls.saved_write
        if (write["summary"] != {"classification": "NEWLY_DENIED", "changed_count": 2}
                or write["before"]["decision"]["reason"] != "ALLOW"
                or write["after"]["decision"]["reason"] != "ACTION_NOT_READ_ONLY"
                or tuple(k for k, v in write["differences"].items() if v)
                != ("allowed", "reason")):
            raise AssertionError("frozen saved write anchors differ")

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="gooo-policy-summary-check-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.counter = 0

    def path(self, name="report.json"):
        self.counter += 1
        return self.root / (str(self.counter) + "-" + name)

    def argv(self, report, digest):
        return [sys.executable, "-I", "-B", str(self.formatter),
                "--report", str(report), "--report-sha256", digest]

    def invoke(self, raw, *, digest=None, name="report.json", args=None):
        report = self.path(name)
        report.write_bytes(raw)
        before = report.read_bytes()
        argv = self.argv(report, sha256(raw) if digest is None else digest)
        if args is not None:
            argv = [sys.executable, "-I", "-B", str(self.formatter)] + args
        result = subprocess.run(argv, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                stdin=subprocess.DEVNULL, timeout=10, check=False)
        self.assertEqual(report.read_bytes(), before, "formatter changed its input")
        return result

    def assert_closed(self, result, label):
        self.assertEqual(result.returncode, 2)
        self.assertEqual(result.stdout, b"", "failure exposed stdout")
        self.assertEqual(result.stderr, ERROR_PREFIX + label.encode("ascii") + b"\n")
        self.assertNotIn(SECRET.encode(), result.stderr)
        self.assertNotIn(b"Traceback", result.stderr)

    def assert_safe_success(self, result):
        self.assertEqual(result.returncode, 0)
        self.assertEqual(result.stderr, b"")
        self.assertGreater(len(result.stdout), 0)
        self.assertLessEqual(len(result.stdout), OUTPUT_CAP)
        text = result.stdout.decode("utf-8", "strict")
        self.assertTrue(text.endswith("\n"))
        self.assertNotIn(SECRET, text)
        self.assertNotIn("\x1b", text)
        self.assertTrue(all(ord(c) >= 32 or c == "\n" for c in text))
        self.assertIn("does not authorize", text)
        return text

    def changed(self, path, value):
        report = copy.deepcopy(self.saved_write)
        replace_at(report, path, value)
        return report

    def assert_shape_refused(self, report):
        self.assert_closed(self.invoke(raw_json(report)), "REPORT_SHAPE_REFUSED")

    def test_saved_write_exact_anchors_and_only_changed_field_names(self):
        text = self.assert_safe_success(self.invoke(self.saved_raw))
        for literal in ("`NEWLY_DENIED`", "| Changed fields | 2 |",
                        "| Before reason | `ALLOW` |",
                        "| After reason | `ACTION_NOT_READ_ONLY` |",
                        "Changed field names: `allowed`, `reason`."):
            self.assertIn(literal, text)
        self.assertNotIn("effective_principal", text)
        self.assertNotIn("synthetic:user", text)
        self.assertNotIn("tenant-a", text)
        self.assertNotIn("compiler_source_sha", text)
        self.assertNotIn("generation_sha256", text)
        self.assertNotIn("classifier_bindings", text)

    def test_synthetic_literal_read_remains_unchanged(self):
        report = copy.deepcopy(self.saved_write)
        # A literal display test specified by the existing example/check_cli.
        # No policy result is inferred by this harness from arbitrary facts.
        for side in ("before", "after"):
            report[side]["facts"]["requested_action"] = "read"
            report[side]["facts"]["allowed_action"] = "read"
        report["after"]["decision"] = copy.deepcopy(report["before"]["decision"])
        report["differences"] = {name: False for name in DECISION_NAMES}
        report["summary"] = {"classification": "UNCHANGED", "changed_count": 0}
        text = self.assert_safe_success(self.invoke(raw_json(report)))
        self.assertIn("`UNCHANGED`", text)
        self.assertIn("| Changed fields | 0 |", text)
        self.assertIn("| Before reason | `ALLOW` |", text)
        self.assertIn("| After reason | `ALLOW` |", text)
        self.assertIn("Changed field names: (none).", text)

    def test_private_fact_and_other_decision_values_are_never_printed(self):
        report = copy.deepcopy(self.saved_write)
        for side in ("before", "after"):
            for name, value in report[side]["facts"].items():
                if type(value) is str:
                    report[side]["facts"][name] = SECRET + name + "<script>&|`\n::error::"
            report[side]["decision"]["effective_principal"] = SECRET + "principal"
            report[side]["decision"]["access_result"] = SECRET + "access-result"
        text = self.assert_safe_success(self.invoke(raw_json(report)))
        self.assertNotIn("<script>", text)
        self.assertNotIn("::error::", text)
        self.assertNotIn("principal_value", text)
        self.assertNotIn("caller_tenant", text)
        self.assertNotIn("resource_tenant", text)

    def test_safe_label_registry_and_supplied_summary_are_displayed(self):
        # These are transport/presentation checks. They deliberately do not
        # recompute a classifier, replay bindings, or attest semantic truth.
        for classification in CLASSIFICATIONS:
            with self.subTest(classification=classification):
                report = self.changed(("summary", "classification"), classification)
                text = self.assert_safe_success(self.invoke(raw_json(report)))
                self.assertIn("`" + classification + "`", text)
        for reason in REASONS:
            with self.subTest(reason=reason):
                report = self.changed(("after", "decision", "reason"), reason)
                text = self.assert_safe_success(self.invoke(raw_json(report)))
                self.assertIn("| After reason | `" + reason + "` |", text)
        report = self.changed(("summary", "changed_count"), 15)
        text = self.assert_safe_success(self.invoke(raw_json(report)))
        self.assertIn("| Changed fields | 15 |", text)

    def test_maximum_name_list_is_bounded_and_uses_fixed_field_order(self):
        report = copy.deepcopy(self.saved_write)
        report["differences"] = {name: True for name in reversed(DECISION_NAMES)}
        report["summary"] = {"classification": "OBSERVATIONS_CHANGED", "changed_count": 15}
        text = self.assert_safe_success(self.invoke(raw_json(report)))
        expected = "Changed field names: " + ", ".join("`" + name + "`" for name in DECISION_NAMES) + "."
        self.assertIn(expected, text)
        self.assertNotIn("synthetic:user", text)

    def test_digest_tampering_and_ordinary_whitespace_change(self):
        wrong = ("0" if FIXTURE_SHA256[0] != "0" else "1") + FIXTURE_SHA256[1:]
        self.assert_closed(self.invoke(self.saved_raw, digest=wrong), "REPORT_DIGEST_MISMATCH")
        self.assert_closed(self.invoke(self.saved_raw + b"\n", digest=FIXTURE_SHA256),
                           "REPORT_DIGEST_MISMATCH")

    def test_argument_errors_are_fixed_and_hide_caller_text(self):
        marker_path = str(self.root / (SECRET + "missing.json"))
        malformed = ([], ["--help"], [SECRET], ["--report", marker_path],
                     ["--report", marker_path, "--report-sha256", "A" * 64],
                     ["--report", marker_path, "--report-sha256", "0" * 63],
                     ["--report", marker_path, "--report-sha256", "g" * 64],
                     ["--report", marker_path, "--report-sha256", "sha256:" + "0" * 64],
                     ["--report", marker_path, "--report", marker_path])
        for args in malformed:
            with self.subTest(arguments=len(args)):
                self.assert_closed(self.invoke(self.saved_raw, args=args), "ARGUMENTS_INVALID")

    def test_missing_directory_symlink_and_fifo_reports_are_refused(self):
        targets = [self.root / (SECRET + "missing.json"), self.root]
        original = self.path()
        original.write_bytes(self.saved_raw)
        linked = self.path("symlink.json")
        linked.symlink_to(original)
        targets.append(linked)
        fifo = self.path("fifo.json")
        os.mkfifo(fifo, 0o600)
        targets.append(fifo)
        for report in targets:
            with self.subTest(kind=report.name):
                result = subprocess.run(self.argv(report, FIXTURE_SHA256),
                                        stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                        stdin=subprocess.DEVNULL, timeout=3, check=False)
                self.assert_closed(result, "REPORT_READ_REFUSED")
        self.assertEqual(original.read_bytes(), self.saved_raw)

    def test_stdout_append_alias_and_hardlink_alias_refuse_before_writing(self):
        for hardlink in (False, True):
            with self.subTest(hardlink=hardlink):
                report = self.path()
                report.write_bytes(self.saved_raw)
                output = report
                if hardlink:
                    output = self.path("same-inode.txt")
                    os.link(report, output)
                with output.open("ab", buffering=0) as sink:
                    result = subprocess.run(self.argv(report, FIXTURE_SHA256), stdout=sink,
                                            stderr=subprocess.PIPE, stdin=subprocess.DEVNULL,
                                            timeout=10, check=False)
                self.assertEqual(result.returncode, 2)
                self.assertEqual(result.stderr, ERROR_PREFIX + b"OUTPUT_REFUSED\n")
                self.assertEqual(report.read_bytes(), self.saved_raw)
                self.assertEqual(output.read_bytes(), self.saved_raw)

    def test_exact_top_nested_shapes_reject_missing_unknown_and_wrong_containers(self):
        object_paths = ((), ("before",), ("after",), ("before", "facts"),
                        ("after", "decision"), ("before", "bindings"),
                        ("classifier_bindings",), ("summary",), ("differences",))
        for path in object_paths:
            for mode in ("missing", "extra", "array", "null"):
                with self.subTest(path=path, mode=mode):
                    report = copy.deepcopy(self.saved_write)
                    current = report
                    for name in path:
                        current = current[name]
                    if mode == "missing":
                        del current[next(iter(current))]
                    elif mode == "extra":
                        current[SECRET + "unknown"] = True
                    elif not path:
                        report = [] if mode == "array" else None
                    else:
                        replace_at(report, path, [] if mode == "array" else None)
                    self.assert_shape_refused(report)

    def test_wrong_types_including_bool_as_int_are_refused(self):
        cases = [(("summary", "changed_count"), value)
                 for value in (True, False, None, 2.0, "2", -1, 16)]
        cases += [(("before", "facts", "authenticated"), 1),
                  (("before", "facts", "principal_value"), False),
                  (("after", "decision", "allowed"), 0),
                  (("after", "decision", "reason"), None),
                  (("after", "decision", "effective_principal"), {}),
                  (("differences", "allowed"), 1),
                  (("before", "bindings", "assets"), []),
                  (("before", "bindings", "compiler_source_sha"), 83928),
                  (("classifier_bindings", "generation_sha256"), True)]
        for path, value in cases:
            with self.subTest(path=path, value_type=type(value).__name__):
                self.assert_shape_refused(self.changed(path, value))

    def test_required_literals_unknown_labels_and_invisible_markup_refused(self):
        for path in (("schema",), ("status",), ("scope",),
                     ("before_profile",), ("candidate_profile",)):
            self.assert_shape_refused(self.changed(path, SECRET))
        unsafe = (SECRET, "ALLOW\u200b", "ALLOW\u202e", "ALLOW\n::error::",
                  "ALLOW\x1b[2J", "<script>" + SECRET, "![x](https://example.invalid)",
                  "ALLOW|injected", "`ALLOW`", "ALLOW ")
        for value in unsafe:
            for path in (("summary", "classification"),
                         ("before", "decision", "reason"),
                         ("after", "decision", "reason")):
                with self.subTest(path=path, label=repr(value)):
                    self.assert_shape_refused(self.changed(path, value))
        report = copy.deepcopy(self.saved_write)
        report["differences"][SECRET + "<script>"] = report["differences"].pop("allowed")
        self.assert_shape_refused(report)

    def test_duplicate_keys_at_multiple_depths_and_escape_equivalence_refused(self):
        raw = raw_json(self.saved_write)
        duplicate_documents = [
            b'{"schema":"gooo/api-policy-revision-comparison/v1",' + raw[1:],
            b'{"\\u0073chema":"gooo/api-policy-revision-comparison/v1",' + raw[1:],
            raw.replace(b'"facts":{', b'"facts":{"principal_value":"' +
                        SECRET.encode() + b'",', 1),
            raw.replace(b'"summary":{', b'"summary":{"changed_count":2,', 1),
            raw.replace(b'"differences":{', b'"differences":{"\\u0061llowed":true,', 1),
        ]
        report = copy.deepcopy(self.saved_write)
        report["classifier_bindings"]["assets"] = {"safe": "opaque"}
        nested_raw = raw_json(report).replace(b'"assets":{"safe":"opaque"}',
                      b'"assets":{"safe":"opaque","s\\u0061fe":"' + SECRET.encode() + b'"}', 1)
        duplicate_documents.append(nested_raw)
        for index, document in enumerate(duplicate_documents):
            with self.subTest(index=index):
                self.assert_closed(self.invoke(document), "REPORT_JSON_REFUSED")

    def test_malformed_unicode_json_nonfinite_and_trailing_data_refused(self):
        raw = raw_json(self.saved_write)
        malformed = (b"", b"\xef\xbb\xbf" + raw, raw + b"{}", raw + b"\xff",
                     raw.replace(b"synthetic:user", b"bad\xff" + SECRET.encode(), 1),
                     raw.replace(b"synthetic:user", b"\\ud800", 1),
                     raw.replace(b"synthetic:user", b"\\udfff", 1),
                     raw.replace(b'"schema":', b'"\\ud800":', 1),
                     raw.replace(b'"changed_count":2', b'"changed_count":NaN', 1),
                     raw.replace(b'"changed_count":2', b'"changed_count":Infinity', 1),
                     raw.replace(b'"changed_count":2', b'"changed_count":-Infinity', 1),
                     b"[" * 1100 + b"0" + b"]" * 1100)
        for index, document in enumerate(malformed):
            with self.subTest(index=index):
                result = self.invoke(document)
                # The unchanged 1,100-array input is valid JSON but not a report.
                # Parser recursion bounds vary by interpreter: either a closed
                # parse refusal or a post-parse shape refusal is correct here.
                if index == 11 and result.stderr == ERROR_PREFIX + b"REPORT_SHAPE_REFUSED\n":
                    self.assert_closed(result, "REPORT_SHAPE_REFUSED")
                else:
                    self.assert_closed(result, "REPORT_JSON_REFUSED")

    def test_valid_unicode_private_values_are_not_lossily_rejected(self):
        # Both direct UTF-8 and JSON's valid surrogate pair are legal transport.
        report = self.changed(("before", "facts", "principal_value"), SECRET + "한😀")
        self.assert_safe_success(self.invoke(raw_json(report)))
        escaped = json.dumps(report, ensure_ascii=True, separators=(",", ":")).encode("ascii")
        self.assert_safe_success(self.invoke(escaped))

    def test_text_asset_and_whole_report_boundaries(self):
        for size in (65536, 65537):
            report = self.changed(("before", "facts", "principal_value"), "x" * size)
            result = self.invoke(raw_json(report))
            if size == 65536:
                self.assert_safe_success(result)
            else:
                self.assert_closed(result, "REPORT_SHAPE_REFUSED")
        for suffix in ("x", "xx"):
            text = "한" * 21845 + suffix
            report = self.changed(("after", "decision", "effective_principal"), text)
            result = self.invoke(raw_json(report))
            if len(suffix) == 1:
                self.assert_safe_success(result)  # 65,536 UTF-8 bytes.
            else:
                self.assert_closed(result, "REPORT_SHAPE_REFUSED")
        for size in (65536, 65537):
            report = self.changed(("classifier_bindings", "assets"), {"safe": "v" * size})
            result = self.invoke(raw_json(report))
            if size == 65536:
                self.assert_safe_success(result)
            else:
                self.assert_closed(result, "REPORT_SHAPE_REFUSED")
        for size in (512, 513):
            report = self.changed(("classifier_bindings", "assets"), {"n" * size: "opaque"})
            result = self.invoke(raw_json(report))
            if size == 512:
                self.assert_safe_success(result)
            else:
                self.assert_closed(result, "REPORT_SHAPE_REFUSED")
        for count in (0, 64, 65):
            assets = {"asset-%02d" % i: "opaque" for i in range(count)}
            result = self.invoke(raw_json(self.changed(("classifier_bindings", "assets"), assets)))
            if count == 64:
                self.assert_safe_success(result)
            else:
                self.assert_closed(result, "REPORT_SHAPE_REFUSED")
        empty_name = self.changed(("classifier_bindings", "assets"), {"": "opaque"})
        self.assert_shape_refused(empty_name)
        cap_exact = self.saved_raw + b" " * (REPORT_CAP - len(self.saved_raw))
        self.assert_safe_success(self.invoke(cap_exact))
        self.assert_closed(self.invoke(cap_exact + b" "), "REPORT_READ_REFUSED")


if __name__ == "__main__":
    if len(sys.argv) != 2:
        raise SystemExit("usage: check_summary.py /absolute/path/to/summary.py")
    SummaryChecks.formatter = Path(sys.argv[1]).resolve(strict=True)
    if not SummaryChecks.formatter.is_file():
        raise SystemExit("formatter must be a regular source file")
    unittest.main(argv=[sys.argv[0]], verbosity=2)
