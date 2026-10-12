"""Only strict input/parser checks; this file never invokes the compiler."""
import importlib.util
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parent.parent
SPEC = importlib.util.spec_from_file_location("fits_crop_inputs", ROOT / "tools/check-fits-crop-inputs.py")
PROBE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PROBE)


class FitsCropMalformedInputParserTests(unittest.TestCase):
    def test_every_generated_input_is_rejected_by_trusted_preflight(self):
        cases = list(PROBE.negative_inputs())
        self.assertEqual(len(cases), 90)
        self.assertEqual(len({c["name"] for c in cases}), len(cases))
        self.assertEqual(sum(c["duplicate"] for c in cases), 8)
        self.assertEqual({c["position"] for c in cases}, {0, 1})
        for case in cases:
            with self.subTest(case=case["name"]), self.assertRaises(ValueError):
                PROBE.CHECK.validate_inputs(case["raw"], 1)

    def test_documented_inputs_preserve_int64_and_noncanonical_shapes(self):
        raw = (ROOT / "examples/fits-crop/inputs.json").read_bytes()
        rows = PROBE.CHECK.validate_inputs(raw, 14)
        self.assertEqual(len(rows), 14)
        self.assertEqual(rows[9][PROBE.CHECK.ENTRY + ".input1"]["min_x"], PROBE.CHECK.HI)
        self.assertEqual(rows[9][PROBE.CHECK.ENTRY + ".input1"]["max_x"], PROBE.CHECK.LO)
        self.assertEqual(rows[13][PROBE.CHECK.ENTRY + ".input0"]["min_x"], 9007199254740993)

    def test_missing_unknown_ports_and_duplicate_decoded_input_keys(self):
        raw = (ROOT / "examples/fits-crop/inputs.json").read_bytes()
        rows = PROBE.CHECK.validate_inputs(raw)
        import json
        for key in [PROBE.CHECK.ENTRY + ".input0", PROBE.CHECK.ENTRY + ".input1"]:
            row = dict(rows[0])
            del row[key]
            with self.assertRaisesRegex(ValueError, "input ports"):
                PROBE.CHECK.validate_inputs(json.dumps({"schema": "gooo/body-composition-inputs/v1", "inputs": [row]}).encode())
        duplicate = b'{"schema":"gooo/body-composition-inputs/v1","inputs":[],"\\u0069nputs":[]}'
        with self.assertRaisesRegex(ValueError, "duplicate"):
            PROBE.CHECK.validate_inputs(duplicate)


if __name__ == "__main__":
    unittest.main()
