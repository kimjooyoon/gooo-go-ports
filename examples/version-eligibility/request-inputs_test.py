"""Transport boundaries only; no Gooo, Go oracle or child process is invoked."""
import importlib.util
import io
import json
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("version_request", Path(__file__).with_name("request-inputs.py"))
request = importlib.util.module_from_spec(spec)
spec.loader.exec_module(request)


class RequestInputs(unittest.TestCase):
    def encoded(self, **changes):
        values = {"current": "v1.0.0", "candidate": "v2.0.0", "minimum": "v3.0.0"}
        values.update(changes)
        return json.dumps(values).encode()

    def call(self, raw, args=()):
        output, errors = io.BytesIO(), io.StringIO()
        code = request.run(args, io.BytesIO(raw), output, errors)
        return code, output.getvalue(), errors.getvalue()

    def test_one_request_maps_three_values_without_expected_results(self):
        code, raw, error = self.call(self.encoded())
        self.assertEqual((code, error), (0, ""))
        self.assertEqual(json.loads(raw), {"schema": "gooo/body-composition-inputs/v1", "inputs": [{
            request.ENTRY + ".input0": "v1.0.0", request.ENTRY + ".input1": "v2.0.0",
            request.ENTRY + ".input2": "v3.0.0"}]})
        code, raw, _ = self.call(self.encoded(current="", candidate=" bad ", minimum="\x00"))
        self.assertEqual(code, 0)
        self.assertEqual(list(json.loads(raw)["inputs"][0].values()), ["", " bad ", "\x00"])

    def test_1024_utf8_byte_boundary_and_scalar_pair(self):
        for value in ("a" * 1024, "é" * 512, "😀"):
            with self.subTest(value_bytes=len(value.encode())):
                code, raw, _ = self.call(self.encoded(candidate=value))
                self.assertEqual(code, 0)
                self.assertEqual(json.loads(raw)["inputs"][0][request.ENTRY + ".input1"], value)
        for value in ("a" * 1025, "é" * 513):
            with self.subTest(value_bytes=len(value.encode())):
                code, raw, error = self.call(self.encoded(candidate=value))
                self.assertEqual((code, raw), (1, b""))
                self.assertIn("1024 UTF-8 bytes", error)

    def test_invalid_transport_refuses_before_any_output(self):
        base = self.encoded()
        bad = [b"", b"null", b"[]", b"true", base + b"{}", b"{", b"\xff",
               b'{"current":"a","current":"b","candidate":"c","minimum":"d"}',
               b'{"current":"a","\\u0063urrent":"b","candidate":"c","minimum":"d"}',
               b'{"current":"\\ud800","candidate":"c","minimum":"d"}',
               b'{"current":"\\udc00","candidate":"c","minimum":"d"}',
               b'{"current":"a","candidate":"c","minimum":"d","extra":true}',
               b'{"current":"a","candidate":"c"}']
        for value in (None, True, 1, [], {}, float("nan")):
            bad.append(self.encoded(candidate=value))
        for raw in bad:
            with self.subTest(raw=raw):
                code, output, errors = self.call(raw)
                self.assertEqual((code, output), (1, b""))
                self.assertTrue(errors.startswith("version request refused: "))

    def test_raw_cap_includes_whitespace_and_preserves_request(self):
        raw = self.encoded()
        self.assertEqual(self.call(raw + b" " * (request.MAX_RAW_BYTES - len(raw)))[0], 0)
        code, output, errors = self.call(raw + b" " * (request.MAX_RAW_BYTES + 1 - len(raw)))
        self.assertEqual((code, output), (1, b""))
        self.assertIn("32 KiB", errors)

    def test_usage_does_not_read_input_and_io_failure_is_nonzero(self):
        class Unreadable:
            def read(self, _size):
                raise OSError("unreadable input")
        output, errors = io.BytesIO(), io.StringIO()
        self.assertEqual(request.run(["--unknown"], Unreadable(), output, errors), 2)
        self.assertEqual(output.getvalue(), b"")
        output, errors = io.BytesIO(), io.StringIO()
        self.assertEqual(request.run([], Unreadable(), output, errors), 1)
        self.assertEqual(output.getvalue(), b"")
        self.assertIn("unreadable input", errors.getvalue())


if __name__ == "__main__":
    unittest.main()
