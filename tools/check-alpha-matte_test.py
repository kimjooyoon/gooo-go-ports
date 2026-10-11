"""Focused observer regressions; synthetic records are not execution evidence."""
import copy
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent.parent
SPEC = importlib.util.spec_from_file_location("alpha_matte_checker", ROOT / "tools/check-alpha-matte.py")
CHECK = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(CHECK)


def encode(value):
    return json.dumps(value, ensure_ascii=False, separators=(",", ":")).encode()


class AlphaMatteReceiptTest(unittest.TestCase):
    def setUp(self):
        folder = tempfile.TemporaryDirectory()
        self.addCleanup(folder.cleanup)
        self.root = Path(folder.name)
        for name in ["upstream.json", "alpha-matte.workspace.json", "examples/alpha-matte/inputs.json",
                     *[n for names in CHECK.SOURCES.values() for n in names]]:
            path = self.root / name
            path.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / name, path)

    def pair(self):
        manifest_raw = (self.root / "alpha-matte.workspace.json").read_bytes()
        input_raw = (self.root / "examples/alpha-matte/inputs.json").read_bytes()
        manifest, fixture = json.loads(manifest_raw), json.loads(input_raw)["inputs"]
        lock = json.loads((self.root / "upstream.json").read_bytes())
        packages = []
        for p in manifest["packages"]:
            package = {"path": p["path"], "sources": [{"filename": n, "source_digest": CHECK.source_digest(self.root / n)} for n in p["sources"]]}
            if p["path"] == "app/alpha-matte":
                package["exports"] = [{"name": "MattePixel", "id": CHECK.ACTIVITY, "kind": "activity", "input_types": list(CHECK.INPUT_ENTITIES), "output_type": CHECK.RESULT}]
            packages.append(package)
        traces = []
        for i, (inputs, row) in enumerate(zip(fixture, CHECK.EXPECTED_ROWS)):
            actual = dict(zip(CHECK.RESULT_NAMES, row))
            ports = [{"port": "input" + str(n), "entity_id": CHECK.INPUT_ENTITIES[n], "value": inputs[CHECK.ENTRY + ".input" + str(n)], "fields": CHECK.fields(CHECK.INPUT_ENTITIES[n], CHECK.INPUT_NAMES[n], inputs[CHECK.ENTRY + ".input" + str(n)])} for n in (0, 1)]
            traces.append({"case_index": i, "deliveries": [{"activity_id": CHECK.ACTIVITY, "inputs": ports, "actual": actual, "actual_fields": CHECK.fields(CHECK.RESULT, CHECK.RESULT_NAMES, actual)}]})
        source = "package main\n// Synthetic observer fixture only.\n"
        digest = CHECK.digest(source.encode())
        a = {"schema": "gooo/workspace-body-execution-receipt/v1", "decision": "OBSERVED", "manifest_digest": CHECK.digest(manifest_raw), "inputs_digest": CHECK.digest(input_raw), "result": {
            "program": {"entry": manifest["entry"], "workspace_image": {"packages": packages}},
            "composition": {"stage": "COMPLETE", "source": source, "generated_sha256": digest, "steps": [{"generation": {"report": {"activity_id": CHECK.ACTIVITY, "compiler_source_sha": lock["compiler_revision"]}}}]},
            "runtime": {"schema": "gooo/body-composition-runtime/v1", "stage": "COMPLETE", "producer_source_sha": lock["compiler_revision"], "go_version": "go version go1.27.2 linux/amd64", "generated_sha256": digest, "finite_total": 0, "finite_passed": 0, "model_calls": 0, "projection_replayed": True, "runtime_replayed": True, "traces": traces}}}
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

    def test_mutations_reject_even_when_both_receipts_agree(self):
        def delivery(r, i=0):
            return r["result"]["runtime"]["traces"][i]["deliveries"][0]
        def exported(r):
            return r["result"]["program"]["workspace_image"]["packages"][1]["exports"]
        mutations = {
            "acceptance lie": lambda r: delivery(r)["actual"].update(in_byte_domain=False),
            "integer acceptance": lambda r: delivery(r)["actual"].update(in_byte_domain=1),
            "float output": lambda r: delivery(r)["actual"].update(red=10.0),
            "boolean output": lambda r: delivery(r)["actual"].update(premultiplied_red=False),
            "rounded alpha": lambda r: delivery(r, 3)["actual"].update(premultiplied_alpha=32768),
            "naive 8-bit blend": lambda r: delivery(r, 7)["actual"].update(red=1),
            "rejected is accepted": lambda r: delivery(r, 18)["actual"].update(in_byte_domain=True),
            "rejected channel": lambda r: delivery(r, 18)["actual"].update(blue=0),
            "missing output": lambda r: delivery(r)["actual"].pop("blue"),
            "extra output": lambda r: delivery(r)["actual"].update(extra=0),
            "narrowed original": lambda r: delivery(r, 22)["inputs"][0]["value"].update(red=255),
            "rounded original": lambda r: delivery(r, 22)["inputs"][0]["value"].update(red=9223372036854775806),
            "boolean input": lambda r: delivery(r)["inputs"][0]["value"].update(alpha=False),
            "input field order": lambda r: delivery(r)["inputs"][0]["fields"].reverse(),
            "input field identity": lambda r: delivery(r)["inputs"][0]["fields"][0].update(id="wrong"),
            "input field value": lambda r: delivery(r)["inputs"][0]["fields"][0].update(value=0),
            "input entity": lambda r: delivery(r)["inputs"][0].update(entity_id=CHECK.RGB),
            "input port order": lambda r: delivery(r)["inputs"].reverse(),
            "extra single input": lambda r: delivery(r).update(input=0),
            "output field ID": lambda r: delivery(r)["actual_fields"][0].update(id="wrong"),
            "output field order": lambda r: delivery(r)["actual_fields"].reverse(),
            "output field Boolean": lambda r: delivery(r)["actual_fields"][0].update(value=1),
            "source hash": lambda r: r["result"]["program"]["workspace_image"]["packages"][0]["sources"][0].update(source_digest="wrong"),
            "compiler": lambda r: r["result"]["runtime"].update(producer_source_sha="wrong"),
            "wrong Go version": lambda r: r["result"]["runtime"].update(go_version="go version go1.26.5 linux/amd64"),
            "32-bit Go": lambda r: r["result"]["runtime"].update(go_version="go version go1.27.2 linux/386"),
            "generation compiler": lambda r: r["result"]["composition"]["steps"][0]["generation"]["report"].update(compiler_source_sha="wrong"),
            "generation activity": lambda r: r["result"]["composition"]["steps"][0]["generation"]["report"].update(activity_id="wrong"),
            "entry": lambda r: r["result"]["program"]["entry"].update(activity="Other"),
            "delivery ID": lambda r: delivery(r).update(activity_id="wrong"),
            "export input order": lambda r: exported(r)[0]["input_types"].reverse(),
            "export identity": lambda r: exported(r)[0].update(id="wrong"),
            "missing export": lambda r: exported(r).clear(),
            "duplicate export": lambda r: exported(r).append(copy.deepcopy(exported(r)[0])),
            "source": lambda r: r["result"]["composition"].update(source="changed"),
            "manifest": lambda r: r.update(manifest_digest="wrong"),
            "inputs": lambda r: r.update(inputs_digest="wrong"),
            "case document": lambda r: r.update(cases_digest="wrong"),
            "PASS not OBSERVED": lambda r: r.update(decision="PASS"),
            "Boolean count": lambda r: r["result"]["runtime"].update(finite_total=False),
            "model calls": lambda r: r["result"]["runtime"].update(model_calls=1),
            "case order": lambda r: r["result"]["runtime"]["traces"].reverse(),
            "trace omission": lambda r: r["result"]["runtime"]["traces"].pop(),
        }
        def empty_generated(r):
            r["result"]["composition"].update(source="", generated_sha256=CHECK.digest(b""))
            r["result"]["runtime"]["generated_sha256"] = CHECK.digest(b"")
        mutations["coordinated empty program"] = empty_generated
        # Source order itself is not meaningful; move a source across packages.
        mutations["wrong source owner"] = lambda r: r["result"]["program"]["workspace_image"]["packages"][1]["sources"].append(
            r["result"]["program"]["workspace_image"]["packages"][0]["sources"].pop())
        for name, mutate in mutations.items():
            with self.subTest(name=name):
                a, b = self.pair()
                mutate(a)
                mutate(b)
                with self.assertRaises(ValueError):
                    self.check(a, b)

    def test_local_fixture_and_source_are_independently_bound(self):
        for name, mutate in {
            "rounded int64 fixture": lambda d: d["inputs"][22][CHECK.ENTRY+".input0"].update(red=9223372036854775806),
            "Boolean fixture": lambda d: d["inputs"][0][CHECK.ENTRY+".input0"].update(alpha=False),
            "float fixture": lambda d: d["inputs"][0][CHECK.ENTRY+".input1"].update(red=10.0),
            "reordered fixture": lambda d: d["inputs"].reverse(),
        }.items():
            with self.subTest(name=name):
                path = self.root / "examples/alpha-matte/inputs.json"
                before = path.read_bytes()
                fixture = json.loads(before)
                mutate(fixture)
                path.write_bytes(encode(fixture))
                try:
                    # Both receipts now faithfully match the changed input file.
                    with self.assertRaises(ValueError):
                        self.check(*self.pair())
                finally:
                    path.write_bytes(before)
        a, b = self.pair()
        source = self.root / "ports/image/nrgba.gooo"
        source.write_bytes(source.read_bytes()+b"\n// changed\n")
        with self.assertRaisesRegex(ValueError, "source digest"):
            self.check(a, b)

    def test_execution_and_saved_replay_roles_are_distinct(self):
        a, b = self.pair()
        a["replayed_from_sha256"] = "sha256:" + "0"*64
        with self.assertRaisesRegex(ValueError, "unexpected replay metadata"):
            self.check(a, b)
        a, b = self.pair()
        b["result"]["replay"]["model_calls"] = False
        with self.assertRaisesRegex(ValueError, "replay model calls"):
            self.check(a, b)
        a, b = self.pair()
        b["result"]["runtime"]["traces"][0]["deliveries"][0]["actual"]["red"] = 0
        with self.assertRaises(ValueError):
            self.check(a, b)

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
