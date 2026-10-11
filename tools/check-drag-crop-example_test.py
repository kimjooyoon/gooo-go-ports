"""Receipt checker regressions using synthetic records; no compiler execution."""
import copy
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest


ROOT = Path(__file__).resolve().parent.parent
SPEC = importlib.util.spec_from_file_location("check_drag_crop", ROOT / "tools/check-drag-crop-example.py")
CHECK = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(CHECK)


def encode(value):
    return json.dumps(value, ensure_ascii=False, separators=(",", ":")).encode()


class DragCropReceiptTest(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        for name in ["upstream.json", "drag-crop.workspace.json", "examples/crop-planner/drag_inputs.json",
                     *[name for names in CHECK.SOURCES.values() for name in names]]:
            path = self.root / name
            path.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / name, path)

    def receipts(self):
        manifest_raw = (self.root / "drag-crop.workspace.json").read_bytes()
        inputs_raw = (self.root / "examples/crop-planner/drag_inputs.json").read_bytes()
        manifest = json.loads(manifest_raw)
        inputs = json.loads(inputs_raw)["inputs"]
        lock = json.loads((self.root / "upstream.json").read_bytes())
        packages = []
        for p in manifest["packages"]:
            package = {"path": p["path"], "sources": [
                {"filename": name, "source_digest": CHECK.source_digest(self.root / name)}
                for name in p["sources"]]}
            if p["path"] == "example/crop-planner":
                package["exports"] = [{"name": "DragCropPlan", "id": CHECK.ACTIVITY, "kind": "activity",
                                       "input_types": [CHECK.RECTANGLE] * 2, "output_type": CHECK.RESULT}]
            packages.append(package)
        lo, hi = -(2**63), 2**63 - 1
        # Literal example answers; synthetic checker fixtures are not native evidence.
        outputs = [[100, 100, 200, 200, True], [100, 100, 200, 200, True],
                   [100, 100, 200, 200, True], [100, 100, 200, 200, True],
                   [0, 40, 200, 480, True], [0, 0, 0, 0, False],
                   [0, 0, 0, 0, False], [0, 0, 0, 0, False], [-8, -30, 5, 4, True],
                   [hi - 1, hi - 1, hi, hi, True], [lo, lo, lo + 1, lo + 1, True],
                   [9007199254740993, -9007199254740992, 9007199254740994, -9007199254740991, True]]
        traces = []
        for i, (supplied, row) in enumerate(zip(inputs, outputs)):
            actual = dict(zip(CHECK.RESULT_NAMES, row))
            ports = []
            for n in range(2):
                value = supplied[CHECK.ENTRY + ".input" + str(n)]
                ports.append({"port": "input" + str(n), "entity_id": CHECK.RECTANGLE, "value": value,
                              "fields": CHECK.fields(CHECK.RECTANGLE, CHECK.COORDINATES, value)})
            traces.append({"case_index": i, "deliveries": [{"activity_id": CHECK.ACTIVITY, "inputs": ports,
                           "actual": actual, "actual_fields": CHECK.fields(CHECK.RESULT, CHECK.RESULT_NAMES, actual)}]})
        source = "package main\n// Synthetic checker fixture, never executed.\n"
        generated = CHECK.digest(source.encode())
        receipt = {"schema": "gooo/workspace-body-execution-receipt/v1", "decision": "OBSERVED",
                   "manifest_digest": CHECK.digest(manifest_raw), "inputs_digest": CHECK.digest(inputs_raw),
                   "result": {"program": {"entry": manifest["entry"], "workspace_image": {"packages": packages}},
                              "composition": {"stage": "COMPLETE", "source": source, "generated_sha256": generated,
                                              "steps": [{"generation": {"report": {"activity_id": CHECK.ACTIVITY,
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

    def check_raw(self, execution, replay):
        a, b = self.root / "execution.json", self.root / "replay.json"
        a.write_bytes(execution)
        b.write_bytes(replay)
        try:
            CHECK.check(self.root, a, b)
        finally:
            self.assertEqual(a.read_bytes(), execution)
            self.assertEqual(b.read_bytes(), replay)

    def check_pair(self, execution, replay):
        raw = encode(execution)
        replay["replayed_from_sha256"] = CHECK.digest(raw)
        self.check_raw(raw, encode(replay))

    def test_valid_exact_int64_and_replay(self):
        self.check_pair(*self.receipts())

    def test_rejects_replay_detached_from_execution(self):
        a, b = self.receipts()
        b["replayed_from_sha256"] = "sha256:" + "0" * 64
        with self.assertRaisesRegex(ValueError, "replay binding"):
            self.check_raw(encode(a), encode(b))

    def test_rejects_duplicate_decoded_keys_on_each_side(self):
        for side in ("execution", "replay"):
            for members in (b'"decision":"PASS"', b'"\\u0064ecision":"OBSERVED"',
                            b'"unknown":[{"x":0,"x":1}]'):
                with self.subTest(side=side, members=members):
                    a, b = self.receipts()
                    raw = encode(a)
                    if side == "execution":
                        raw = b"{" + members + b"," + raw[1:]
                    b["replayed_from_sha256"] = CHECK.digest(raw)
                    saved = encode(b)
                    if side == "replay":
                        saved = b"{" + members + b"," + saved[1:]
                    with self.assertRaisesRegex(ValueError, "duplicate"):
                        self.check_raw(raw, saved)

    def test_rejects_altered_records_and_execution_identity(self):
        def delivery(receipt):
            return receipt["result"]["runtime"]["traces"][0]["deliveries"][0]

        mutations = {
            "float coordinate": lambda r: delivery(r)["actual"].update(min_x=100.0),
            "Boolean coordinate": lambda r: delivery(r)["actual"].update(min_x=True),
            "Boolean flag": lambda r: delivery(r)["actual"].update(has_pixels=1),
            "wrong output": lambda r: delivery(r)["actual"].update(max_x=201),
            "out of int64": lambda r: delivery(r)["actual"].update(max_x=2**63),
            "wrong input": lambda r: delivery(r)["inputs"][0]["value"].update(max_x=641),
            "input field ID": lambda r: delivery(r)["inputs"][0]["fields"][0].update(id="wrong"),
            "output field ID": lambda r: delivery(r)["actual_fields"][0].update(id="wrong"),
            "wrong activity": lambda r: delivery(r).update(activity_id="wrong"),
            "wrong entry": lambda r: r["result"]["program"]["entry"].update(activity="CropPlan"),
            "compiler": lambda r: r["result"]["runtime"].update(producer_source_sha="wrong"),
            "counter bool": lambda r: r["result"]["runtime"].update(finite_total=False),
            "case bool": lambda r: r["result"]["runtime"]["traces"][0].update(case_index=False),
            "generated source": lambda r: r["result"]["composition"].update(source="changed"),
            "32-bit Go": lambda r: r["result"]["runtime"].update(go_version="go version go1.27.2 linux/386"),
            "missing trace": lambda r: r["result"]["runtime"]["traces"].pop(),
            "export ID": lambda r: r["result"]["program"]["workspace_image"]["packages"][1]["exports"][0].update(id="wrong"),
        }
        for side in (0, 1):
            for name, mutate in mutations.items():
                with self.subTest(side=side, name=name):
                    pair = self.receipts()
                    mutate(pair[side])
                    with self.assertRaises(ValueError):
                        self.check_pair(*pair)

    def test_rejects_precision_loss_on_each_side(self):
        for side in (0, 1):
            with self.subTest(side=side):
                pair = self.receipts()
                value = pair[side]["result"]["runtime"]["traces"][-1]["deliveries"][0]["actual"]
                value["min_x"] = 9007199254740992
                with self.assertRaisesRegex(ValueError, "documented output"):
                    self.check_pair(*pair)

    def test_rejects_missing_wrong_kind_and_aliased_activity_id(self):
        for side in (0, 1):
            for mutation in ("missing kind", "wrong kind", "aliased ID"):
                with self.subTest(side=side, mutation=mutation):
                    pair = self.receipts()
                    exports = pair[side]["result"]["program"]["workspace_image"]["packages"][1]["exports"]
                    if mutation == "missing kind":
                        del exports[0]["kind"]
                    elif mutation == "wrong kind":
                        exports[0]["kind"] = "entity"
                    else:
                        alias = copy.deepcopy(exports[0])
                        alias["name"] = "DifferentActivity"
                        exports.append(alias)
                    with self.assertRaisesRegex(ValueError, "export activity identity"):
                        self.check_pair(*pair)

    def test_rejects_any_source_digest_or_roster_change(self):
        for side in (0, 1):
            for package in range(2):
                for source in range(2):
                    with self.subTest(side=side, package=package, source=source):
                        pair = self.receipts()
                        sources = pair[side]["result"]["program"]["workspace_image"]["packages"][package]["sources"]
                        sources[source]["source_digest"] = "sha256:" + "0" * 64
                        with self.assertRaisesRegex(ValueError, "source digest"):
                            self.check_pair(*pair)
                pair = self.receipts()
                sources = pair[side]["result"]["program"]["workspace_image"]["packages"][package]["sources"]
                sources[1] = copy.deepcopy(sources[0])
                with self.assertRaisesRegex(ValueError, "source roster"):
                    self.check_pair(*pair)

    def test_rejects_current_source_change_and_manifest_relabeling(self):
        a, b = self.receipts()
        path = self.root / "ports/image/canon.gooo"
        path.write_bytes(path.read_bytes() + b"\n// changed\n")
        with self.assertRaisesRegex(ValueError, "source digest"):
            self.check_pair(a, b)
        manifest_path = self.root / "drag-crop.workspace.json"
        manifest = json.loads(manifest_path.read_bytes())
        manifest["entry"]["activity"] = "CropPlan"
        manifest_path.write_bytes(encode(manifest))
        with self.assertRaisesRegex(ValueError, "manifest entry"):
            self.check_pair(*self.receipts())


if __name__ == "__main__":
    unittest.main()
