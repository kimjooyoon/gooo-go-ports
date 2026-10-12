"""Pure parser regressions and optional tampering of real, immutable receipts.

Set GOOO_FITS_CROP_EXECUTION and GOOO_FITS_CROP_REPLAY after actual native runs.
No receipt or generated program is synthesized as execution evidence here.
"""
import copy
import importlib.util
import json
import os
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parent.parent
SPEC = importlib.util.spec_from_file_location("fits_crop_check", ROOT / "tools/check-fits-crop.py")
CHECK = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(CHECK)


def encode(value):
    return json.dumps(value, ensure_ascii=False, separators=(",", ":")).encode()


class FitsCropParserTests(unittest.TestCase):
    def test_exact_boolean_type_and_literal_answers(self):
        CHECK.boolean_result(True, True)
        CHECK.boolean_result(False, False)
        for value in [0, 1, 0.0, 1.0, None, "true", [], {}]:
            with self.subTest(value=value), self.assertRaisesRegex(ValueError, "Boolean output type"):
                CHECK.boolean_result(value, True)
        with self.assertRaisesRegex(ValueError, "documented Boolean output"):
            CHECK.boolean_result(False, True)

    def test_scalar_fields_only_nil_or_empty_list(self):
        for fields in [None, []]:
            CHECK.scalar_fields(fields)
        for fields in [0, False, "", {}, [{"name": "value", "value": True}]]:
            with self.subTest(fields=fields), self.assertRaisesRegex(ValueError, "Boolean scalar fields"):
                CHECK.scalar_fields(fields)

    def test_exact_original_int64_and_noncanonical_records(self):
        CHECK.validate_rectangle({"min_x": CHECK.HI, "min_y": CHECK.LO,
                                  "max_x": CHECK.LO, "max_y": 9007199254740993})
        base = {"min_x": 1, "min_y": 2, "max_x": 5, "max_y": 6}
        for field in CHECK.COORDINATES:
            for bad in [None, False, True, "1", 1.0, 1.5, CHECK.HI + 1, CHECK.LO - 1]:
                with self.subTest(field=field, bad=bad), self.assertRaisesRegex(ValueError, "exact int64"):
                    CHECK.validate_rectangle(dict(base, **{field: bad}))
        for value in [None, [], "record", 0, dict(base, extra=1), {k: v for k, v in base.items() if k != "max_x"}]:
            with self.subTest(value=value), self.assertRaisesRegex(ValueError, "rectangle fields"):
                CHECK.validate_rectangle(value)

    def test_duplicate_decoded_keys_and_exact_number_comparison(self):
        for raw in [b'{"x":0,"x":1}', b'{"x":0,"\\u0078":1}', b'{"unknown":[{"x":0,"x":1}]}']:
            with self.subTest(raw=raw), self.assertRaisesRegex(ValueError, "duplicate"):
                CHECK.loads_receipt(raw)
        self.assertFalse(CHECK.exact(1, True))
        self.assertFalse(CHECK.exact(1, 1.0))
        self.assertFalse(CHECK.exact(9007199254740993, 9007199254740992))


@unittest.skipUnless(os.environ.get("GOOO_FITS_CROP_EXECUTION") and os.environ.get("GOOO_FITS_CROP_REPLAY"),
                     "real FitsCrop execute/replay receipts have not been supplied")
class FitsCropActualReceiptTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.execution_path = Path(os.environ["GOOO_FITS_CROP_EXECUTION"])
        cls.replay_path = Path(os.environ["GOOO_FITS_CROP_REPLAY"])
        cls.execution_raw = cls.execution_path.read_bytes()
        cls.replay_raw = cls.replay_path.read_bytes()
        CHECK.check_bytes(ROOT, cls.execution_raw, cls.replay_raw)

    def tearDown(self):
        self.assertEqual(self.execution_path.read_bytes(), self.execution_raw)
        self.assertEqual(self.replay_path.read_bytes(), self.replay_raw)

    def test_actual_original_pair(self):
        CHECK.check_bytes(ROOT, self.execution_raw, self.replay_raw)

    def test_tampering_each_side_with_matching_parent_to_reach_target_guard(self):
        def delivery(r, index=0):
            return r["result"]["runtime"]["traces"][index]["deliveries"][0]

        def consumer_export(r):
            package = next(p for p in r["result"]["program"]["workspace_image"]["packages"] if p["path"] == "example/fits-crop")
            return next(e for e in package["exports"] if e.get("name") == "FitsCrop")

        mutations = {
            "integer result": (lambda r: delivery(r).update(actual=1), "Boolean output type"),
            "float result": (lambda r: delivery(r).update(actual=1.0), "Boolean output type"),
            "wrong result": (lambda r: delivery(r).update(actual=False), "documented Boolean output"),
            "scalar fields": (lambda r: delivery(r).update(actual_fields=[{"id": "wrong", "name": "value", "value": True}]), "Boolean scalar fields"),
            "case Boolean index": (lambda r: r["result"]["runtime"]["traces"][0].update(case_index=False), "case order"),
            "input ports swapped": (lambda r: delivery(r)["inputs"].reverse(), "exact original input records"),
            "input field ID": (lambda r: delivery(r)["inputs"][0]["fields"][0].update(id="wrong"), "exact original input records"),
            "input fields reordered": (lambda r: delivery(r)["inputs"][0]["fields"].reverse(), "exact original input records"),
            "narrowed precision": (lambda r: delivery(r, 13)["inputs"][0]["value"].update(min_x=9007199254740992), "exact original input records"),
            "normalized receiver": (lambda r: delivery(r, 9)["inputs"][1]["value"].update(min_x=CHECK.LO, max_x=CHECK.HI), "exact original input records"),
            "source hash": (lambda r: r["result"]["program"]["workspace_image"]["packages"][0]["sources"][0].update(source_digest="wrong"), "source digest"),
            "manifest digest": (lambda r: r.update(manifest_digest="wrong"), "manifest digest"),
            "inputs digest": (lambda r: r.update(inputs_digest="wrong"), "input digest"),
            "producer": (lambda r: r["result"]["runtime"].update(producer_source_sha="wrong"), "compiler identity"),
            "entry": (lambda r: r["result"]["program"]["entry"].update(activity="Other"), "entry identity"),
            "entry package": (lambda r: r["result"]["program"]["entry"].update(package_path="go/image"), "entry identity"),
            "lowered entry name": (lambda r: r["result"]["program"]["entry"].update(lowered_name="WrongLoweredActivity"), "entry lowered activity binding"),
            "empty lowered entry": (lambda r: r["result"]["program"]["entry"].update(lowered_name=""), "entry lowered activity binding"),
            "export ID": (lambda r: consumer_export(r).update(id="wrong"), "export activity identity"),
            "output entity": (lambda r: consumer_export(r).update(output_type=CHECK.RECTANGLE), "export type identity"),
            "generated program": (lambda r: r["result"]["composition"].update(source="changed"), "generated source binding"),
            "Boolean counter": (lambda r: r["result"]["runtime"].update(finite_total=False), "input-only runtime counters"),
            "scored counter": (lambda r: r["result"]["runtime"].update(finite_passed=1), "input-only runtime counters"),
            "case score": (lambda r: delivery(r).update(passed=True), "input-only case score"),
            "case expectation": (lambda r: delivery(r).update(expected=True), "input-only case score"),
            "PASS receipt": (lambda r: r.update(decision="PASS"), "observed execution"),
            "32-bit Go": (lambda r: r["result"]["runtime"].update(go_version="go version go1.27.2 linux/386"), "64-bit Go"),
        }
        for side in (0, 1):
            for name, (mutate, reason) in mutations.items():
                with self.subTest(side=side, name=name):
                    pair = [copy.deepcopy(CHECK.loads_receipt(self.execution_raw)), copy.deepcopy(CHECK.loads_receipt(self.replay_raw))]
                    mutate(pair[side])
                    original = encode(pair[0])
                    pair[1]["replayed_from_sha256"] = CHECK.digest(original)
                    with self.assertRaisesRegex(ValueError, reason):
                        CHECK.check_bytes(ROOT, original, encode(pair[1]))

    def test_detached_replay_and_duplicate_wire_keys(self):
        replay = CHECK.loads_receipt(self.replay_raw)
        replay["replayed_from_sha256"] = "sha256:" + "0" * 64
        with self.assertRaisesRegex(ValueError, "saved replay binding"):
            CHECK.check_bytes(ROOT, self.execution_raw, encode(replay))
        for side in (0, 1):
            for key in [b'"decision":"PASS",', b'"\\u0064ecision":"OBSERVED",', b'"unknown":[{"x":0,"x":1}],']:
                pair = [self.execution_raw, self.replay_raw]
                pair[side] = b"{" + key + pair[side][1:]
                with self.subTest(side=side, key=key), self.assertRaisesRegex(ValueError, "duplicate"):
                    CHECK.check_bytes(ROOT, *pair)


if __name__ == "__main__":
    unittest.main()
