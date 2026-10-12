"""Strict request/evidence checks using synthetic receipts; never run a compiler."""
import copy
import importlib.util
import json
from pathlib import Path
import unittest
from unittest import mock

ROOT = Path(__file__).resolve().parent.parent
SPEC = importlib.util.spec_from_file_location("literal_text_inputs", ROOT / "tools/check-literal-text-inputs.py")
PROBE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PROBE)


def receipt_fixture(case):
    path = Path("/frozen/" + case["entry"].rsplit(":", 1)[1] + ".workspace.json")
    manifest_raw = b"synthetic manifest bytes"
    error = case["expected_diagnostic"]
    if case["expected_rejection_stage"] == "INPUT_DOMAIN":
        activity = case["entry"].rsplit(":", 1)[1]
        key = "GoooPackage1Activity" + activity + ".input" + str(case["position"])
        error = "generate workspace activity bodies: case 0: " + ('input "' + key + '": ' if case["position"] is not None else "") + error
    receipt = {"schema": PROBE.RECEIPT_SCHEMA, "decision": "FAIL_CLOSED", "manifest": path.as_posix(),
               "manifest_digest": PROBE.digest(manifest_raw), "inputs_digest": PROBE.digest(case["raw"]), "error": error}
    return receipt, path, manifest_raw


class LiteralMalformedRequestTests(unittest.TestCase):
    def setUp(self):
        self.cases = list(PROBE.negative_inputs())

    def test_all_three_entries_and_both_arguments_have_independent_rejection_rosters(self):
        self.assertEqual(len(self.cases), 84)
        self.assertEqual(len({case["name"] for case in self.cases}), 84)
        for entry in PROBE.ENTRIES:
            selected = [case for case in self.cases if case["entry"] == entry]
            self.assertEqual(len(selected), 28)
            for position in range(2):
                selected_port = [case for case in selected if case["position"] == position]
                self.assertEqual(len(selected_port), 12)
                self.assertEqual(sum("duplicate" in case["name"] for case in selected_port), 2)
                self.assertEqual(sum("1025" in case["name"] for case in selected_port), 2)
            self.assertEqual(sum(case["expected_rejection_stage"] == "INPUT_JSON_DECODE" for case in selected), 7)
        for case in self.cases:
            with self.subTest(case=case["name"]), self.assertRaises(ValueError):
                PROBE.validate_inputs(case["raw"], case["entry"])

    def test_oversize_roster_counts_utf8_bytes_and_preserves_the_other_argument(self):
        for case in self.cases:
            if "1025" not in case["name"]:
                continue
            row = json.loads(case["raw"])["inputs"][0]
            key = case["entry"] + ".input" + str(case["position"])
            other = case["entry"] + ".input" + str(1 - case["position"])
            with self.subTest(case=case["name"]):
                self.assertEqual(len(row[key].encode("utf-8")), 1025)
                self.assertEqual(row[other], "tic" if case["position"] == 0 else "diagnostic")
                if "multibyte" in case["name"]:
                    self.assertEqual(len(row[key]), 513)

    def test_every_wrong_type_is_present_in_each_argument_without_coercion(self):
        for entry in PROBE.ENTRIES:
            for position in range(2):
                names = {case["name"].split("-", 1)[1].rsplit("-input", 1)[0]
                         for case in self.cases if case["entry"] == entry and case["position"] == position}
                self.assertTrue({"null", "false", "true", "integer", "fraction", "array", "object"} <= names)

    def test_supplied_example_and_exact_multibyte_boundary_are_in_domain(self):
        rows = PROBE.validate_inputs((ROOT / "examples/literal-text-report/inputs.json").read_bytes(), PROBE.ENTRIES[-1])
        self.assertEqual(len(rows), 24)
        for entry in PROBE.ENTRIES:
            raw = PROBE.encode({"schema": PROBE.SCHEMA, "inputs": [{entry + ".input0": "é" * 512,
                                                                      entry + ".input1": "🙂" * 256}]})
            accepted = PROBE.validate_inputs(raw, entry)[0]
            self.assertEqual(len(accepted[entry + ".input0"].encode()), 1024)
            self.assertEqual(len(accepted[entry + ".input1"].encode()), 1024)

    def test_wire_utf8_and_surrogates_are_preflight_issues_not_actual_cli_claims(self):
        entry = PROBE.ENTRIES[0]
        raw = PROBE.encode({"schema": PROBE.SCHEMA, "inputs": [{entry + ".input0": "PLACEHOLDER", entry + ".input1": ""}]})
        for replacement in (b"\xff", b"\\ud800"):
            with self.subTest(replacement=replacement), self.assertRaises(ValueError):
                PROBE.validate_inputs(raw.replace(b"PLACEHOLDER", replacement), entry)
        self.assertFalse(any("surrogate" in case["name"] or "wire-utf8" in case["name"] for case in self.cases))


class LiteralRejectionReceiptTests(unittest.TestCase):
    def test_each_expected_route_keeps_absent_runtime_not_reported(self):
        for case in PROBE.negative_inputs():
            receipt, path, raw = receipt_fixture(case)
            with self.subTest(case=case["name"]):
                observed = PROBE.inspect_rejection(case, PROBE.encode(receipt), 1, False, path, raw)
                self.assertTrue(observed["rejection_verified"])
                self.assertFalse(observed["result_present"])
                self.assertFalse(observed["runtime_present"])
                for key in ("actual_runtime_stage", "observed_counters", "observed_build", "observed_runs"):
                    self.assertEqual(observed[key], "NOT_REPORTED")

    def test_valid_rejection_label_cannot_hide_wrong_request_route_or_runtime(self):
        case = next(iter(PROBE.negative_inputs()))
        receipt, path, raw = receipt_fixture(case)
        mutations = {
            "schema": lambda r: r.update(schema="wrong"),
            "decision": lambda r: r.update(decision="OBSERVED"),
            "manifest path": lambda r: r.update(manifest="/other.workspace.json"),
            "manifest digest": lambda r: r.update(manifest_digest=PROBE.digest(b"other")),
            "request digest": lambda r: r.update(inputs_digest=PROBE.digest(b"other")),
            "diagnostic": lambda r: r.update(error="unrelated failure"),
            "route": lambda r: r.update(error=r["error"].replace("generate workspace activity bodies: case 0:", "decode inputs:")),
            "case index": lambda r: r.update(error=r["error"].replace("case 0:", "case 1:")),
            "argument": lambda r: r.update(error=r["error"].replace(".input0", ".input1")),
            "empty result": lambda r: r.update(result={}),
            "zero counters": lambda r: r.update(result={"runtime": {"stage": "COMPLETE", "finite_total": 0,
                                                                     "finite_passed": 0, "model_calls": 0}}),
        }
        for name, mutate in mutations.items():
            altered = copy.deepcopy(receipt)
            mutate(altered)
            with self.subTest(name=name), self.assertRaises(ValueError):
                PROBE.inspect_rejection(case, PROBE.encode(altered), 1, False, path, raw)
        for exit_code, timed_out in ((0, False), (2, False), (None, True), (1, True)):
            with self.subTest(exit_code=exit_code, timed_out=timed_out), self.assertRaises(ValueError):
                PROBE.inspect_rejection(case, PROBE.encode(receipt), exit_code, timed_out, path, raw)

    def test_decode_failure_cannot_be_relabelled_as_a_later_body_failure(self):
        case = next(c for c in PROBE.negative_inputs() if c["expected_rejection_stage"] == "INPUT_JSON_DECODE")
        receipt, path, raw = receipt_fixture(case)
        receipt["error"] = "generate workspace activity bodies: " + receipt["error"]
        with self.assertRaisesRegex(ValueError, "precede body generation"):
            PROBE.inspect_rejection(case, PROBE.encode(receipt), 1, False, path, raw)

    def test_duplicate_receipt_keys_are_not_collapsed(self):
        case = next(iter(PROBE.negative_inputs()))
        receipt, path, raw = receipt_fixture(case)
        data = PROBE.encode(receipt).replace(b'"decision":"FAIL_CLOSED"',
                                           b'"decision":"OBSERVED","\\u0064ecision":"FAIL_CLOSED"')
        with self.assertRaisesRegex(ValueError, "duplicate"):
            PROBE.inspect_rejection(case, data, 1, False, path, raw)

    def test_clean_compiler_pin_is_independent_of_request_labels(self):
        lock = {"compiler_revision": PROBE.COMPILER, "compiler_version": PROBE.VERSION, "go_version": PROBE.GO}
        build = {"schema": "gooo/build-identity/v1", "version": PROBE.VERSION, "compiler_source_sha": PROBE.COMPILER,
                 "vcs_revision": PROBE.COMPILER, "source_status": "CLEAN_VCS", "vcs_modified": "false",
                 "go_version": PROBE.GO, "native_go_required": PROBE.GO}
        PROBE.validate_identity(build, lock)
        for field in build:
            altered = dict(build, **{field: "different"})
            with self.subTest(field=field), self.assertRaises(ValueError):
                PROBE.validate_identity(altered, lock)
        for field in lock:
            with self.subTest(lock_field=field), self.assertRaises(ValueError):
                PROBE.validate_identity(build, dict(lock, **{field: "different"}))

    def test_runner_environment_does_not_reuse_model_or_toolchain_configuration(self):
        with mock.patch.dict(PROBE.os.environ, {"GOOO_LAYA_URL": "external", "OPENAI_API_KEY": "secret",
                                                "GOTOOLCHAIN": "auto", "GOWORK": "/other/go.work"}):
            env = PROBE.clean_environment()
        self.assertNotIn("GOOO_LAYA_URL", env)
        self.assertNotIn("OPENAI_API_KEY", env)
        self.assertEqual({key: env[key] for key in ("GOTOOLCHAIN", "GOWORK", "GOENV", "GOPROXY", "GOSUMDB")},
                         {"GOTOOLCHAIN": "local", "GOWORK": "off", "GOENV": "off", "GOPROXY": "off", "GOSUMDB": "off"})


if __name__ == "__main__":
    unittest.main()
