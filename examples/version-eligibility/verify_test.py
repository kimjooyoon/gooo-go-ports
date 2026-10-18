"""Boundary tests; no Go/Gooo command or module download is performed."""
import copy
import importlib.util
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest import mock

spec = importlib.util.spec_from_file_location("verify", Path(__file__).with_name("verify.py"))
verify = importlib.util.module_from_spec(spec)
spec.loader.exec_module(verify)


class Boundaries(unittest.TestCase):
    def inputs(self):
        base = Path(__file__).parent
        return (json.loads((base / "reference-inputs.json").read_text()),
                json.loads((base / "inputs.json").read_text()))

    def test_reference_sync_preserves_original_order_and_types(self):
        reference, inputs = self.inputs()
        self.assertEqual(len(verify.synchronized(reference, inputs)), 8)
        for mutate in (lambda x: x["inputs"].reverse(),
                       lambda x: x["inputs"][0].update({verify.ENTRY + ".input0": True}),
                       lambda x: x.update({"expected": []})):
            changed = copy.deepcopy(inputs)
            mutate(changed)
            with self.assertRaises(ValueError):
                verify.synchronized(reference, changed)

    def test_domain_overflow_and_duplicate_json_refused(self):
        reference, inputs = self.inputs()
        reference[0]["current"] = "1" * 1025
        with self.assertRaises(ValueError):
            verify.synchronized(reference, inputs)
        for raw in (b'{"a":1,"\\u0061":2}', b'{"x":NaN}', b'"\xff"'):
            with self.assertRaises((ValueError, UnicodeError)):
                verify.load(raw)

    def test_minimum_requires_ordered_third_input(self):
        base = Path(__file__).parent
        reference = json.loads((base / "minimum-reference-inputs.json").read_text())
        inputs = json.loads((base / "minimum-inputs.json").read_text())
        selected = verify.policy("minimum")
        rows = verify.synchronized(reference, inputs, selected)
        self.assertEqual(len(rows), 8)
        self.assertEqual(len(rows[0]), 3)
        self.assertEqual(len(selected["fields"]), 13)
        changed = copy.deepcopy(inputs)
        del changed["inputs"][0][selected["entry"] + ".input2"]
        with self.assertRaises(ValueError):
            verify.synchronized(reference, changed, selected)
        with self.assertRaises(ValueError):
            verify.synchronized(reference, inputs)  # Upgrade cannot consume minimum inputs.

    def test_minimum_does_not_change_upgrade_profile(self):
        self.assertEqual(verify.policy("upgrade")["fields"], verify.FIELDS)
        self.assertEqual(len(verify.policy("upgrade")["parameters"]), 2)
        minimum = verify.policy("minimum")
        self.assertEqual(minimum["entry"], "app/version-eligibility:AssessUpdate")
        self.assertEqual(minimum["rid"], "gooo://go-ports/examples/version-eligibility/v1/update-result")
        with self.assertRaises(ValueError):
            verify.policy("unknown")

    def test_boundary_keeps_go_compare_argument_direction(self):
        reference = json.loads((Path(__file__).parent / "boundary-reference-inputs.json").read_text())
        selected = verify.policy("compare-boundary")
        rows = verify.synchronized(reference, None, selected)
        self.assertEqual(len(rows), 9)
        self.assertEqual(selected["sources"], verify.SOURCES[:1])
        for actual, original in zip(rows, reference):
            self.assertEqual(actual[selected["entry"] + ".input0"], original["candidate"])
            self.assertEqual(actual[selected["entry"] + ".input1"], original["current"])
        changed = copy.deepcopy(reference)
        changed[0]["candidate"] = True
        with self.assertRaises(ValueError):
            verify.synchronized(changed, None, selected)
        with self.assertRaises(ValueError):
            verify.synchronized(reference[:-1], None, selected)

    def test_stable_sync_preserves_minimum_and_upgrade_profiles(self):
        base = Path(__file__).parent
        reference = json.loads((base / "stable-reference-inputs.json").read_text())
        inputs = json.loads((base / "stable-inputs.json").read_text())
        minimum = copy.deepcopy(verify.policy("minimum"))
        upgrade = copy.deepcopy(verify.policy("upgrade"))
        selected = verify.policy("stable")
        self.assertEqual(selected["parameters"], ("current", "candidate", "minimum"))
        self.assertEqual(selected["entry"], "app/version-eligibility:AssessStableSameMajorUpdate")
        self.assertEqual(selected["rid"], "gooo://go-ports/examples/version-eligibility/v1/stable-same-major-result")
        self.assertEqual(len(selected["fields"]), 16)
        self.assertEqual(selected["fields"], minimum["fields"][:-1] +
                         ("same_major", "candidate_stable", "policy_eligible", "policy_reason"))
        self.assertEqual(selected["types"], minimum["types"][:-1] + (bool, bool, bool, str))
        rows = verify.synchronized(reference, inputs, selected)
        self.assertEqual(len(rows), 12)
        for row, original in zip(rows, reference):
            self.assertEqual(row, {selected["entry"] + ".input" + str(i): original[key]
                                   for i, key in enumerate(("current", "candidate", "minimum"))})
        for mutate in (lambda x: x["inputs"][0].pop(selected["entry"] + ".input2"),
                       lambda x: x["inputs"][0].update({selected["entry"] + ".input2": True}),
                       lambda x: x["inputs"].reverse()):
            changed = copy.deepcopy(inputs)
            mutate(changed)
            with self.assertRaises(ValueError):
                verify.synchronized(reference, changed, selected)
        self.assertEqual(verify.policy("minimum"), minimum)
        self.assertEqual(verify.policy("upgrade"), upgrade)
        self.assertEqual(len(minimum["fields"]), 13)
        self.assertEqual(minimum["fields"][-1], "raw_changed")
        self.assertEqual(upgrade["fields"], verify.FIELDS)

    def test_major_and_prerelease_share_single_text_anchors(self):
        reference = json.loads((Path(__file__).parent / "projection-reference-inputs.json").read_text())
        versions = [row["version"] for row in reference]
        for name, activity in (("major", "Major"), ("prerelease", "Prerelease")):
            with self.subTest(policy=name):
                selected = verify.policy(name)
                self.assertEqual(selected["reference"], "projection-reference-inputs.json")
                self.assertEqual(selected["parameters"], ("version",))
                self.assertEqual(selected["sources"], verify.SOURCES[:1])
                self.assertEqual(selected["entry"], "go/golang.org/x/mod/semver:" + activity)
                self.assertEqual(selected["aid"], "gooo://golang.org/x/mod/semver/" + activity)
                self.assertTrue(selected["scalar"])
                self.assertIs(selected["scalar_type"], str)
                self.assertIsNone(selected["inputs"])
                rows = verify.synchronized(reference, None, selected)
                self.assertEqual(len(rows), 9)
                self.assertEqual(rows, [{selected["entry"]: value} for value in versions])
                for mutate in (lambda x: x[0].update(version=True),
                               lambda x: x[0].update(candidate="v1.0.0"),
                               lambda x: x.pop()):
                    changed = copy.deepcopy(reference)
                    mutate(changed)
                    with self.assertRaises(ValueError):
                        verify.synchronized(changed, None, selected)

    def test_buildinfo_replacement_and_wrong_checksum_refused(self):
        good = {"GoVersion": "go1.27.2", "Path": verify.ORACLE,
                "Main": {"Path": verify.ORACLE},
                "Deps": [{"Path": verify.MODULE, "Version": verify.VERSION, "Sum": verify.SUM}]}
        verify.build_info(good)
        for mutate in (lambda x: x["Deps"][0].update({"Replace": {"Path": "local"}}),
                       lambda x: x["Deps"][0].update({"Sum": "h1:wrong"}),
                       lambda x: x.update({"GoVersion": "go1.27.1"})):
            changed = copy.deepcopy(good)
            mutate(changed)
            with self.assertRaises(ValueError):
                verify.build_info(changed)

    def test_existing_output_refused_before_any_child(self):
        with tempfile.TemporaryDirectory() as directory:
            with mock.patch.object(sys, "argv", ["verify", "--out", directory, "--module-cache", "/missing"]):
                with mock.patch.object(verify.subprocess, "Popen") as child:
                    with self.assertRaisesRegex(ValueError, "output exists"):
                        verify.main()
                    child.assert_not_called()

    def test_regular_input_rejects_directory_and_symlink(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            with self.assertRaises(ValueError):
                verify.read(root)
            (root / "original").write_text("{}")
            (root / "link").symlink_to(root / "original")
            with self.assertRaises(ValueError):
                verify.read(root / "link")

    def test_process_stdin_is_empty_and_overflow_is_closed(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            work = root / "work"
            work.mkdir()
            runner = verify.Runner(root, work, {})
            raw = runner.run("stdin", [sys.executable, "-c", "import sys; print(len(sys.stdin.buffer.read()))"], work)
            self.assertEqual(raw, b"0\n")
            with mock.patch.object(verify, "MAX_OUTPUT", 64):
                with self.assertRaisesRegex(ValueError, "bounded command output"):
                    runner.run("overflow", [sys.executable, "-c", "print('x'*128)"], work)
            self.assertTrue(all(p.poll() is not None for p in runner.processes))
            self.assertTrue(all(r["waited"] for r in runner.commands))

    def test_only_fixed_oracle_cache_diagnostic_is_supported(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            work = root / "work"
            work.mkdir()
            runner = verify.Runner(root, work, {})
            command = [sys.executable, "-c", "import sys; sys.stderr.write('go: downloading golang.org/x/mod v0.29.0\\n')"]
            runner.run("oracle-build", command, work)
            with self.assertRaisesRegex(ValueError, "unexpected command diagnostics"):
                runner.run("execute", command, work)

    def test_expired_deadline_still_waits_for_owned_child(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            work = root / "work"
            work.mkdir()
            runner = verify.Runner(root, work, {})
            runner.deadline = 0
            with self.assertRaisesRegex(ValueError, "deadline"):
                runner.run("deadline", [sys.executable, "-c", "import time; time.sleep(30)"], work)
            self.assertTrue(runner.commands[0]["waited"])
            self.assertIsNotNone(runner.processes[0].poll())


if __name__ == "__main__":
    unittest.main()
