"""Receipt parser regressions; synthetic subprocess output, no compiler runs."""
import contextlib
import copy
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest import mock


ROOT = Path(__file__).resolve().parent.parent


def script(name):
    spec = importlib.util.spec_from_file_location(name, ROOT / "tools" / (name + ".py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


CROP = script("check-crop-example")
RECTANGLE = script("check-rectangle-inputs")


def encode(value):
    return json.dumps(value, ensure_ascii=False, separators=(",", ":")).encode()


def prepend(raw, members):
    return b"{" + members + b"," + raw[1:]


class ReceiptConsumersTest(unittest.TestCase):
    def setUp(self):
        self.cwd = Path.cwd()
        # Production tools run at the repository root, including in CI.
        import os
        os.chdir(ROOT)
        self.addCleanup(os.chdir, self.cwd)

    def crop_receipts(self):
        manifest_raw = Path("crop-planner.workspace.json").read_bytes()
        inputs_raw = Path("examples/crop-planner/inputs.json").read_bytes()
        manifest, inputs = json.loads(manifest_raw), json.loads(inputs_raw)["inputs"]
        lock = json.loads(Path("upstream.json").read_bytes())
        packages = []
        entry = "example/crop-planner:CropPlan"
        activity = "gooo://example/crop-planner/crop-plan"
        rectangle, result = "gooo://go/image/rectangle", "gooo://example/crop-planner/result"
        coordinates = ["min_x", "min_y", "max_x", "max_y"]
        names = coordinates + ["has_pixels"]
        for p in manifest["packages"]:
            filename = p["sources"][0]
            text = json.dumps(Path(filename).read_text(), ensure_ascii=False)
            for a, b in [("&", "\\u0026"), ("<", "\\u003c"), (">", "\\u003e"),
                         ("\u2028", "\\u2028"), ("\u2029", "\\u2029")]:
                text = text.replace(a, b)
            package = {"path": p["path"], "sources": [{"filename": filename,
                       "source_digest": CROP.digest(text.encode())}]}
            if p["path"] == "example/crop-planner":
                package["exports"] = [{"name": "CropPlan", "id": activity,
                                       "input_types": [rectangle, rectangle], "output_type": result}]
            packages.append(package)
        lo, hi = -(2**63), 2**63 - 1
        # The published twelve example results, solely a parser fixture here.
        outputs = [[0, 40, 200, 480, True], [0, 0, 640, 480, True], [100, 100, 200, 200, True],
                   *[[0, 0, 0, 0, False] for _ in range(5)], [-8, -30, 5, 4, True],
                   [hi - 1, hi - 1, hi, hi, True], [lo, lo, lo + 1, lo + 1, True],
                   [9007199254740993, -9007199254740992, 9007199254740994, -9007199254740991, True]]
        traces = []
        for i, (row, values) in enumerate(zip(inputs, outputs)):
            actual = dict(zip(names, values))
            ports = []
            for n in range(2):
                value = row[entry + ".input" + str(n)]
                ports.append({"port": "input" + str(n), "entity_id": rectangle,
                              "value": value, "fields": CROP.fields(rectangle, coordinates, value)})
            traces.append({"case_index": i, "deliveries": [{"activity_id": activity, "inputs": ports,
                           "actual": actual, "actual_fields": CROP.fields(result, names, actual)}]})
        source = "package main\n// Synthetic parser fixture, never executed.\n"
        generated = CROP.digest(source.encode())
        receipt = {"schema": "gooo/workspace-body-execution-receipt/v1", "decision": "OBSERVED",
                   "manifest_digest": CROP.digest(manifest_raw), "inputs_digest": CROP.digest(inputs_raw),
                   "result": {"program": {"entry": manifest["entry"], "workspace_image": {"packages": packages}},
                              "composition": {"stage": "COMPLETE", "source": source, "generated_sha256": generated,
                                              "steps": [{"generation": {"report": {"activity_id": activity,
                                                        "compiler_source_sha": lock["compiler_revision"]}}}]},
                              "runtime": {"schema": "gooo/body-composition-runtime/v1", "stage": "COMPLETE",
                                          "producer_source_sha": lock["compiler_revision"],
                                          "go_version": "go version " + lock["go_version"] + " linux/amd64",
                                          "generated_sha256": generated, "finite_total": 0, "finite_passed": 0,
                                          "model_calls": 0, "projection_replayed": True,
                                          "runtime_replayed": True, "traces": traces}}}
        replay = copy.deepcopy(receipt)
        replay["result"]["replay"] = {"model_calls": 0}
        return receipt, replay

    def run_crop(self, execution, replay):
        with tempfile.TemporaryDirectory() as tmp:
            a, b = Path(tmp) / "execution.json", Path(tmp) / "replay.json"
            a.write_bytes(execution)
            b.write_bytes(replay)
            try:
                with mock.patch.object(sys, "argv", ["check-crop-example.py", str(a), str(b)]), \
                        contextlib.redirect_stdout(io.StringIO()):
                    CROP.main()
            finally:
                self.assertEqual(a.read_bytes(), execution)
                self.assertEqual(b.read_bytes(), replay)

    def test_crop_normal_receipts_keep_precision_and_binding(self):
        a, b = self.crop_receipts()
        execution = encode(a)
        b["replayed_from_sha256"] = CROP.digest(execution)
        self.run_crop(execution, encode(b))
        b["replayed_from_sha256"] = "sha256:" + "0" * 64
        with self.assertRaises(AssertionError):
            self.run_crop(execution, encode(b))

    def test_crop_rejects_duplicate_receipt_keys(self):
        mutations = [b'"decision":"PASS"', b'"\\u0064ecision":"PASS"',
                     b'"unknown":[{"x":0,"x":1}]']
        for side in ["execution", "replay"]:
            for members in mutations:
                with self.subTest(side=side, members=members):
                    a, b = self.crop_receipts()
                    execution = prepend(encode(a), members) if side == "execution" else encode(a)
                    # Bind to the exact mutated bytes so a replay-hash mismatch cannot hide the parser gap.
                    b["replayed_from_sha256"] = CROP.digest(execution)
                    replay = prepend(encode(b), members) if side == "replay" else encode(b)
                    with self.assertRaisesRegex(ValueError, "duplicate"):
                        self.run_crop(execution, replay)

    def run_rectangle(self, members=None):
        lock = json.loads(Path("upstream.json").read_bytes())
        diagnostics = {"missing": 'requires field "max_y"', "text": "Integer requires an exact int64",
                       "high": "Integer requires an exact int64", "low": "Integer requires an exact int64",
                       "fraction": "Integer requires an exact int64", "null-field": "explicit Integer scalar required",
                       "extra": "contains an undeclared field", "null-record": "explicit JSON object required",
                       "duplicate": "duplicate or invalid JSON key"}
        emitted = []

        def compiler(argv, **kwargs):
            if argv[1] == "version":
                return subprocess.CompletedProcess(argv, 0, encode({"compiler_source_sha": lock["compiler_revision"],
                    "go_version": lock["go_version"], "vcs_modified": "false"}), b"")
            fixture, manifest = Path(argv[argv.index("--inputs") + 1]), Path(argv[-1])
            name, position = fixture.stem.rsplit("-", 1)
            raw = encode({"decision": "FAIL_CLOSED", "error": diagnostics[name] + ".input" + position,
                          "inputs_digest": CROP.digest(fixture.read_bytes()),
                          "manifest_digest": CROP.digest(manifest.read_bytes())})
            if members is not None:
                raw = prepend(raw, members)
            emitted.append((fixture.with_suffix(".receipt.json"), raw))
            return subprocess.CompletedProcess(argv, 1, raw, b"")

        with tempfile.TemporaryDirectory() as tmp:
            target = Path(tmp) / "probe"
            try:
                with mock.patch.object(sys, "argv", ["check-rectangle-inputs.py", str(target)]), \
                        mock.patch.object(RECTANGLE.subprocess, "run", side_effect=compiler), \
                        contextlib.redirect_stdout(io.StringIO()):
                    RECTANGLE.main()
                self.assertEqual(json.loads((target / "summary.json").read_bytes())["framework_rejections"], 18)
            finally:
                for path, raw in emitted:
                    self.assertEqual(path.read_bytes(), raw)

    def test_rectangle_normal_failure_receipts(self):
        self.run_rectangle()

    def test_rectangle_rejects_duplicate_failure_receipt_keys(self):
        for members in [b'"decision":"PASS"', b'"decision":"FAIL_CLOSED"',
                        b'"\\u0064ecision":"PASS"', b'"unknown":{"array":[{"x":0,"x":1}]}']:
            with self.subTest(members=members), self.assertRaisesRegex(ValueError, "duplicate"):
                self.run_rectangle(members)


class ReceiptJSONTest(unittest.TestCase):
    def test_preserves_values_and_unique_unknown_fields(self):
        from receipt_json import loads_receipt
        raw = (b'{"n":9223372036854775807,"low":-9223372036854775808,'
               b'"precise":9007199254740993,"a":[{"x":1},{"x":1},1,1],'
               b'"s":"{\\"x\\":1,\\"x\\":2}","unknown":{"flag":true},'
               b'"value":null,"fraction":1.5,"exponent":1e2,"X":1,"x":2} \n')
        before = bytes(raw)
        value = loads_receipt(raw)
        self.assertEqual(value, json.loads(raw))
        self.assertEqual(value["precise"], 9007199254740993)
        self.assertIs(type(value["precise"]), int)
        self.assertEqual(raw, before)
        # Canonically equivalent Unicode strings remain distinct decoded keys.
        self.assertEqual(len(loads_receipt('{"é":1,"e\\u0301":2}'.encode())), 2)

    def test_duplicate_decoded_keys_and_malformed_documents(self):
        from receipt_json import loads_receipt
        duplicates = [b'{"x":1,"x":1}', b'{"x":0,"\\u0078":1}',
                      '{"한":0,"\\ud55c":1}'.encode(),
                      '{"😀":0,"\\ud83d\\ude00":1}'.encode(),
                      b'{"unknown":[{"x":0,"x":1}]}']
        for raw in duplicates:
            with self.subTest(raw=raw), self.assertRaisesRegex(ValueError, "duplicate"):
                loads_receipt(raw)
        for raw in [b'', b'{} {}', b'{"x":1,}', b'[{"x":1}', b'{"x":}']:
            with self.subTest(raw=raw), self.assertRaises(ValueError):
                loads_receipt(raw)


if __name__ == "__main__":
    unittest.main()
