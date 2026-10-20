"""Focused observer regressions; synthetic records are not execution evidence."""
import copy
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent.parent
SPEC = importlib.util.spec_from_file_location("fixed_checker", ROOT / "tools/check-fixed-pixel-bounds.py")
CHECK = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(CHECK)


def encode(value):
    return json.dumps(value, ensure_ascii=False, separators=(",", ":")).encode()


class FixedReceiptTest(unittest.TestCase):
    def setUp(self):
        folder = tempfile.TemporaryDirectory()
        self.addCleanup(folder.cleanup)
        self.root = Path(folder.name)
        for name in ["upstream.json", "fixed-pixel-bounds.workspace.json", "examples/fixed-pixel-bounds/inputs.json",
                     *[n for names in CHECK.SOURCES.values() for n in names]]:
            path = self.root / name
            path.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / name, path)

    def pair(self):
        manifest_raw = (self.root / "fixed-pixel-bounds.workspace.json").read_bytes()
        input_raw = (self.root / "examples/fixed-pixel-bounds/inputs.json").read_bytes()
        manifest, fixture = json.loads(manifest_raw), json.loads(input_raw)["inputs"]
        lock = json.loads((self.root / "upstream.json").read_bytes())
        packages = []
        for p in manifest["packages"]:
            package = {"path": p["path"], "sources": [{"filename": n, "source_digest": CHECK.source_digest(self.root / n)} for n in p["sources"]]}
            if p["path"] == "example/fixed-pixel-bounds":
                package["exports"] = [{"name": "PixelBounds", "id": CHECK.ACTIVITY, "kind": "activity", "input_types": [CHECK.RECTANGLE] * 2, "output_type": CHECK.RESULT}]
            packages.append(package)
        traces = []
        for i, (inputs, row) in enumerate(zip(fixture, CHECK.EXPECTED_ROWS)):
            actual = dict(zip(CHECK.RESULT_NAMES, row))
            ports = [{"port": "input" + str(n), "entity_id": CHECK.RECTANGLE, "value": inputs[CHECK.ENTRY + ".input" + str(n)], "fields": CHECK.fields(CHECK.RECTANGLE, CHECK.COORDINATES, inputs[CHECK.ENTRY + ".input" + str(n)])} for n in (0, 1)]
            traces.append({"case_index": i, "deliveries": [{"activity_id": CHECK.ACTIVITY, "inputs": ports, "actual": actual, "actual_fields": CHECK.fields(CHECK.RESULT, CHECK.RESULT_NAMES, actual)}]})
        source = "package main\n// Synthetic observer fixture only.\n"
        digest = CHECK.digest(source.encode())
        a = {"schema": "gooo/workspace-body-execution-receipt/v1", "decision": "OBSERVED", "manifest_digest": CHECK.digest(manifest_raw), "inputs_digest": CHECK.digest(input_raw), "result": {
            "program": {"entry": manifest["entry"], "workspace_image": {"packages": packages}},
            "composition": {"stage": "COMPLETE", "source": source, "generated_sha256": digest, "steps": [{"generation": {"report": {"activity_id": CHECK.ACTIVITY, "compiler_source_sha": lock["compiler_revision"]}}}]},
            "runtime": {"schema": "gooo/body-composition-runtime/v1", "stage": "COMPLETE", "producer_source_sha": lock["compiler_revision"], "go_version": "go version go1.27.3 linux/amd64", "generated_sha256": digest, "finite_total": 0, "finite_passed": 0, "model_calls": 0, "projection_replayed": True, "runtime_replayed": True, "traces": traces}}}
        b = copy.deepcopy(a)
        b["result"]["replay"] = {"model_calls": 0}
        return a, b

    def check(self, a, b):
        a = encode(a) if isinstance(a, dict) else a
        if isinstance(b, dict):
            b["replayed_from_sha256"] = CHECK.digest(a)
            b = encode(b)
        first, second = self.root / "execute.json", self.root / "replay.json"
        first.write_bytes(a)
        second.write_bytes(b)
        try:
            CHECK.check(self.root, first, second)
        finally:
            self.assertEqual(first.read_bytes(), a)
            self.assertEqual(second.read_bytes(), b)

    def test_valid(self):
        self.check(*self.pair())

    def test_mutations(self):
        def delivery(r, i=0):
            return r["result"]["runtime"]["traces"][i]["deliveries"][0]
        mutations = {
            "accepted lie": lambda r: delivery(r)["actual"].update(accepted=False),
            "integer acceptance": lambda r: delivery(r)["actual"].update(accepted=1),
            "rounded empty gains area": lambda r: delivery(r, 2)["actual"].update(max_x=1, max_y=1, has_pixels=True),
            "rejection becomes empty acceptance": lambda r: delivery(r, 10)["actual"].update(accepted=True),
            "float": lambda r: delivery(r)["actual"].update(min_x=0.0),
            "narrowed original": lambda r: delivery(r, 15)["inputs"][0]["value"].update(min_x=9007199254740992),
            "field ID": lambda r: delivery(r)["actual_fields"][0].update(id="wrong"),
            "source hash": lambda r: r["result"]["program"]["workspace_image"]["packages"][0]["sources"][0].update(source_digest="wrong"),
            "compiler": lambda r: r["result"]["runtime"].update(producer_source_sha="wrong"),
            "entry": lambda r: r["result"]["program"]["entry"].update(activity="Other"),
            "source": lambda r: r["result"]["composition"].update(source="changed"),
        }
        for side in (0, 1):
            for name, mutate in mutations.items():
                with self.subTest(side=side, name=name):
                    pair = self.pair()
                    mutate(pair[side])
                    with self.assertRaises(ValueError):
                        self.check(*pair)

    def test_duplicate_decoded_keys_and_detached_replay(self):
        for side in (0, 1):
            for prefix in (b'"decision":"PASS",', b'"\\u0064ecision":"OBSERVED",', b'"extra":[{"x":0,"x":1}],'):
                with self.subTest(side=side, prefix=prefix):
                    a, b = self.pair()
                    a = encode(a)
                    if side == 0:
                        a = b"{" + prefix + a[1:]
                    b["replayed_from_sha256"] = CHECK.digest(a)
                    b = encode(b)
                    if side == 1:
                        b = b"{" + prefix + b[1:]
                    with self.assertRaisesRegex(ValueError, "duplicate"):
                        self.check(a, b)
        a, b = self.pair()
        b["replayed_from_sha256"] = "sha256:" + "0" * 64
        with self.assertRaisesRegex(ValueError, "replay binding"):
            self.check(encode(a), encode(b))


if __name__ == "__main__":
    unittest.main()
