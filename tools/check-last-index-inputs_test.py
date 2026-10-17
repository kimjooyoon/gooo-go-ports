#!/usr/bin/env python3
"""Contract tests for the new observer; no project or native execution."""
import copy
import importlib.util
from pathlib import Path
import unittest

PATH = Path(__file__).with_name("check-last-index-inputs.py")
SPEC = importlib.util.spec_from_file_location("last_index_rejections", PATH)
PROBE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PROBE)


class LastIndexInputContracts(unittest.TestCase):
    def test_fresh_two_entry_roster_and_all_negative_preflights(self):
        cases = list(PROBE.negative_inputs())
        self.assertEqual(len(cases), 56)
        self.assertEqual(len({case["name"] for case in cases}), 56)
        for entry in PROBE.ENTRIES:
            self.assertEqual(sum(case["entry"] == entry for case in cases), 28)
        for case in cases:
            with self.subTest(case=case["name"]):
                self.assertNotIn(b"app/literal-text-report:", case["raw"])
                self.assertNotIn(b"go/strings:Index.", case["raw"])
                with self.assertRaises((ValueError, KeyError, TypeError)) as caught:
                    PROBE.validate_inputs(case["raw"], case["entry"])
                self.assertNotIn("untrusted entry", str(caught.exception))

    def test_valid_empty_unicode_limit_and_over_limit(self):
        for entry in PROBE.ENTRIES:
            for text, needle in (("", ""), ("aaaaa", "aa"), ("é" * 512, "é"), ("x", "🙂" * 256)):
                row = {entry + ".input0": text, entry + ".input1": needle}
                raw = PROBE.LEGACY.encode({"schema": PROBE.LEGACY.SCHEMA, "inputs": [row]})
                self.assertEqual(PROBE.validate_inputs(raw, entry), [row])
            for position in (0, 1):
                row = {entry + ".input0": "x", entry + ".input1": "x"}
                row[entry + ".input" + str(position)] = "é" * 512 + "a"
                raw = PROBE.LEGACY.encode({"schema": PROBE.LEGACY.SCHEMA, "inputs": [row]})
                with self.assertRaisesRegex(ValueError, "1024"):
                    PROBE.validate_inputs(raw, entry)

    def test_exact_new_closures_and_old_entry_refusal(self):
        raw = (PATH.parent.parent / PROBE.MANIFEST).read_bytes()
        manifests = PROBE.trusted_manifest_bytes(raw)
        self.assertEqual(set(manifests), set(PROBE.ENTRIES))
        port = PROBE.loads_receipt(manifests[PROBE.ENTRIES[0]][1])
        self.assertEqual(port["packages"][0]["sources"], [PROBE.SOURCES[0], PROBE.SOURCES[2]])
        self.assertEqual(manifests[PROBE.ENTRIES[1]][1], raw)
        original = PROBE.loads_receipt(raw)
        for mutate in (lambda d: d["entry"].update(package_path="app/literal-text-report"),
                       lambda d: d["packages"][0]["sources"].remove(PROBE.SOURCES[2]),
                       lambda d: d["packages"][0]["sources"].append("other.gooo"),
                       lambda d: d.update(unreviewed=True)):
            altered = copy.deepcopy(original)
            mutate(altered)
            with self.assertRaisesRegex(ValueError, "exact v1"):
                PROBE.trusted_manifest_bytes(PROBE.LEGACY.encode(altered))

    def test_receipt_cannot_forge_success_or_drop_input_manifest_binding(self):
        case = next(PROBE.negative_inputs())
        path = Path("/review/last-index.workspace.json")
        manifest = b"reviewed-manifest\n"
        receipt = {"schema": PROBE.LEGACY.RECEIPT_SCHEMA, "decision": "FAIL_CLOSED",
                   "manifest": path.as_posix(), "manifest_digest": PROBE.LEGACY.digest(manifest),
                   "inputs_digest": PROBE.LEGACY.digest(case["raw"]),
                   "error": 'generate workspace activity bodies: case 0: "go/strings:LastIndex.input0": ' + case["expected_diagnostic"]}

        def inspect(value, exit_code=1, timeout=False):
            return PROBE.LEGACY.inspect_rejection(case, PROBE.LEGACY.encode(value), exit_code, timeout, path, manifest)

        result = inspect(receipt)
        self.assertTrue(result["rejection_verified"])
        self.assertEqual(result["actual_runtime_stage"], "NOT_REPORTED")
        self.assertEqual(result["observed_counters"], "NOT_REPORTED")
        for key, value in (("decision", "OBSERVED"), ("manifest", "/other.json"),
                           ("manifest_digest", "sha256:wrong"), ("inputs_digest", "sha256:wrong"),
                           ("error", "unrelated rejection"), ("result", {})):
            with self.subTest(key=key):
                altered = dict(receipt, **{key: value})
                with self.assertRaises(ValueError):
                    inspect(altered)
        with self.assertRaises(ValueError):
            inspect(receipt, exit_code=0)
        with self.assertRaises(ValueError):
            inspect(receipt, timeout=True)


if __name__ == "__main__":
    unittest.main()
