"""Receipt parser regressions; synthetic subprocess output, no compiler runs."""
import contextlib
import copy
import importlib.util
import io
import json
import os
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
YCBCR = script("check-ycbcr-example")
RECTANGLE = script("check-rectangle-inputs")


def encode(value):
    return json.dumps(value, ensure_ascii=False, separators=(",", ":")).encode()


def prepend(raw, members):
    return b"{" + members + b"," + raw[1:]


class ReceiptConsumersTest(unittest.TestCase):
    def setUp(self):
        self.cwd = Path.cwd()
        # Production tools run at the repository root, including in CI.
        os.chdir(ROOT)
        self.addCleanup(os.chdir, self.cwd)

    def crop_receipts(self, source_json=None):
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
            if source_json is None:
                text = json.dumps(Path(filename).read_bytes().decode("utf-8"), ensure_ascii=False)
                for a, b in [("&", "\\u0026"), ("<", "\\u003c"), (">", "\\u003e"),
                             ("\u2028", "\\u2028"), ("\u2029", "\\u2029")]:
                    text = text.replace(a, b)
                encoded = text.encode("utf-8")
            else:
                encoded = source_json[filename]
            package = {"path": p["path"], "sources": [{"filename": filename,
                       "source_digest": CROP.digest(encoded)}]}
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

    def run_example(self, checker, execution, replay):
        with tempfile.TemporaryDirectory() as tmp:
            a, b = Path(tmp) / "execution.json", Path(tmp) / "replay.json"
            a.write_bytes(execution)
            b.write_bytes(replay)
            try:
                with mock.patch.object(sys, "argv", [checker.__file__, str(a), str(b)]), \
                        contextlib.redirect_stdout(io.StringIO()):
                    checker.main()
            finally:
                self.assertEqual(a.read_bytes(), execution)
                self.assertEqual(b.read_bytes(), replay)

    def run_crop(self, execution, replay):
        self.run_example(CROP, execution, replay)

    def ycbcr_receipts(self, source_json):
        manifest_raw = Path("ycbcr-inspector.workspace.json").read_bytes()
        inputs_raw = Path("examples/ycbcr-inspector/inputs.json").read_bytes()
        manifest, inputs = json.loads(manifest_raw), json.loads(inputs_raw)["inputs"]
        lock = json.loads(Path("upstream.json").read_bytes())
        activity = "gooo://go-ports/examples/ycbcr-inspector/inspect-rgb"
        entity = "gooo://go-ports/examples/ycbcr-inspector/inspection"
        entry = "app/ycbcr-inspector:InspectRGB"
        names = ["in_byte_domain", "y", "cb", "cr", "red", "green", "blue",
                 "red_delta", "green_delta", "blue_delta", "exact"]
        # The documented finite outputs make a complete synthetic receipt;
        # these fixtures never launch the compiler or evaluate color policy.
        outputs = [
            [True, 0, 128, 128, 0, 0, 0, 0, 0, 0, True],
            [True, 255, 128, 128, 255, 255, 255, 0, 0, 0, True],
            [True, 76, 85, 255, 254, 0, 0, -1, 0, 0, False],
            [True, 150, 44, 21, 0, 255, 1, 0, 0, 1, False],
            [True, 29, 255, 107, 0, 0, 254, 0, 0, -1, False],
            [True, 128, 128, 128, 128, 128, 128, 0, 0, 0, True],
            [True, 79, 101, 163, 128, 63, 31, 0, -1, -1, False],
            [True, 114, 178, 148, 142, 82, 203, 0, -1, 1, False],
            [True, 153, 88, 163, 202, 142, 82, 0, 0, -1, False],
            *[[False, -1, -1, -1, -1, -1, -1, -1, -1, -1, False] for _ in range(3)],
        ]
        packages = []
        for package in manifest["packages"]:
            value = {"path": package["path"], "sources": [
                {"filename": name, "source_digest": YCBCR.digest(source_json[name])}
                for name in package["sources"]]}
            if package["path"] == "app/ycbcr-inspector":
                value["exports"] = [{"name": "InspectRGB", "id": activity,
                                     "input_types": ["gooo://std/integer"] * 3, "output_type": entity}]
            packages.append(value)
        traces = []
        for i, (row, values) in enumerate(zip(inputs, outputs)):
            actual = dict(zip(names, values))
            traces.append({"case_index": i, "deliveries": [{"activity_id": activity,
                "inputs": [{"port": "input" + str(n), "entity_id": "gooo://std/integer",
                            "value": row[entry + ".input" + str(n)]} for n in range(3)],
                "actual": actual, "actual_fields": [{"id": entity + "/" + n.replace("_", "-"),
                                                     "name": n, "value": actual[n]} for n in names]}]})
        source = "package main\n// Synthetic parser fixture, never executed.\n"
        generated = YCBCR.digest(source.encode())
        receipt = {"schema": "gooo/workspace-body-execution-receipt/v1", "decision": "OBSERVED",
            "manifest_digest": YCBCR.digest(manifest_raw), "inputs_digest": YCBCR.digest(inputs_raw),
            "result": {"program": {"entry": manifest["entry"], "workspace_image": {"packages": packages}},
                "composition": {"stage": "COMPLETE", "source": source, "generated_sha256": generated,
                    "steps": [{"generation": {"report": {"activity_id": activity,
                               "compiler_source_sha": lock["compiler_revision"]}}}]},
                "runtime": {"schema": "gooo/body-composition-runtime/v1", "stage": "COMPLETE",
                    "producer_source_sha": lock["compiler_revision"],
                    "go_version": "go version " + lock["go_version"] + " linux/amd64",
                    "generated_sha256": generated, "finite_total": 0, "finite_passed": 0,
                    "model_calls": 0, "projection_replayed": True, "runtime_replayed": True,
                    "traces": traces}}}
        replay = copy.deepcopy(receipt)
        replay["result"]["replay"] = {"model_calls": 0}
        return receipt, replay

    @contextlib.contextmanager
    def source_workspace(self, example, source_raw, source_json):
        with tempfile.TemporaryDirectory() as tmp:
            directory = Path(tmp)
            manifest_name = {"crop-planner": "crop-planner.workspace.json",
                             "ycbcr-inspector": "ycbcr-inspector.workspace.json"}[example]
            manifest_raw = (ROOT / manifest_name).read_bytes()
            inputs_name = "examples/" + example + "/inputs.json"
            for name in [manifest_name, inputs_name, "upstream.json"]:
                path = directory / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes((ROOT / name).read_bytes())
            expected = {}
            for package in json.loads(manifest_raw)["packages"]:
                for name in package["sources"]:
                    path = directory / name
                    path.parent.mkdir(parents=True, exist_ok=True)
                    path.write_bytes(source_raw)
                    expected[name] = source_json
            before = Path.cwd()
            os.chdir(directory)
            try:
                yield expected
            finally:
                os.chdir(before)

    def test_example_source_digests_preserve_line_endings(self):
        # Literal Go JSON-string bytes are independent of either checker or
        # fixture encoder. In particular, CRLF and CR must not collapse to LF.
        samples = [
            (b"alpha\nbeta\n", b'"alpha\\nbeta\\n"'),
            (b"alpha\r\nbeta\r\n", b'"alpha\\r\\nbeta\\r\\n"'),
            (b"alpha\rbeta\r", b'"alpha\\rbeta\\r"'),
            ("<&>\u2028\u2029é\n".encode("utf-8"),
             b'"\\u003c\\u0026\\u003e\\u2028\\u2029\xc3\xa9\\n"'),
        ]
        self.assertEqual(len({CROP.digest(encoded) for _, encoded in samples}), len(samples))
        for example, checker, fixture in [("crop-planner", CROP, self.crop_receipts),
                                          ("ycbcr-inspector", YCBCR, self.ycbcr_receipts)]:
            for index, (source_raw, expected_json) in enumerate(samples):
                with self.subTest(example=example, source=source_raw), \
                        self.source_workspace(example, source_raw, expected_json) as expected:
                    execution, replay = fixture(expected)
                    replay["replayed_from_sha256"] = checker.digest(encode(execution))
                    self.run_example(checker, encode(execution), encode(replay))
                    for side in ["execution", "replay"]:
                        changed, repeated = copy.deepcopy(execution), copy.deepcopy(replay)
                        target = changed if side == "execution" else repeated
                        target["result"]["program"]["workspace_image"]["packages"][0]["sources"][0]["source_digest"] = checker.digest(samples[(index + 1) % len(samples)][1])
                        # Rebind the exact parent so the source digest check,
                        # rather than replay hash validation, rejects the edit.
                        repeated["replayed_from_sha256"] = checker.digest(encode(changed))
                        with self.subTest(side=side), self.assertRaises(AssertionError):
                            self.run_example(checker, encode(changed), encode(repeated))

    def test_example_source_digests_reject_invalid_utf8(self):
        for example, checker, fixture in [("crop-planner", CROP, self.crop_receipts),
                                          ("ycbcr-inspector", YCBCR, self.ycbcr_receipts)]:
            with self.subTest(example=example), \
                    self.source_workspace(example, b"invalid\xff\n", b'"placeholder"') as expected:
                execution, replay = fixture(expected)
                replay["replayed_from_sha256"] = checker.digest(encode(execution))
                with self.assertRaises(UnicodeDecodeError):
                    self.run_example(checker, encode(execution), encode(replay))

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
