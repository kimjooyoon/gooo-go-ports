# Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
"""Finite assembly tests only; no native runtime, SDK, network or Go execution."""

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

sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parent
SPEC = importlib.util.spec_from_file_location("saved_frame_assembler", ROOT / "assemble_saved_frames.py")
ASSEMBLER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(ASSEMBLER)


class SavedFrameAssemblyTests(unittest.TestCase):
    def setUp(self):
        parent = ROOT / "unit-tmp"
        parent.mkdir(exist_ok=True)
        self.temporary = tempfile.TemporaryDirectory(prefix="owned-", dir=parent)
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        (self.root / "plans").mkdir()
        (self.root / "request").write_bytes(b'{"jsonrpc":"2.0","id":1,"method":"tools/list"}\n')
        (self.root / "response").write_bytes(b'{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}\n')
        self.page = {
            "context": {"capture_extension": ["preserved", None, 2]},
            "request_file": "../request",
            "response_file": str(self.root / "response"),
            "declared_exposed_names": ["host__first", "host__second"],
        }
        self.plan = {
            "target_alias": "host__first",
            "before": {
                "catalog_authorization_context_id": "opaque-local-id",
                "declared_connections": [{"host_id": "host", "connection_id": "c"}],
                "captures": [{"begin": {"unknown_future_field": True}, "pages": [copy.deepcopy(self.page)]}],
            },
            "after": {"captures": [], "metadata_extension": {"unchanged": False}},
        }

    def execute(self, plan=None, raw=None):
        path = self.root / "plans" / "plan.json"
        path.write_bytes(raw if raw is not None else json.dumps(self.plan if plan is None else plan).encode())
        output, diagnostics = io.BytesIO(), io.BytesIO()
        status = ASSEMBLER.run([str(path)], output, diagnostics)
        return status, output.getvalue(), diagnostics.getvalue()

    def assert_refused(self, **arguments):
        status, output, diagnostic = self.execute(**arguments)
        self.assertEqual(status, 1)
        self.assertEqual(output, b"")
        self.assertTrue(diagnostic)
        return diagnostic

    def test_exact_frame_text_and_metadata_order(self):
        request = b' \r\n{"method":"tools/list", "s":"\\\\u0041", "q":"\\\\\\""} \r\n'
        response = ' \t{"雪":"😀","decomposed":"e\u0301","text":"\\\\n"} \r\n'.encode()
        (self.root / "request").write_bytes(request)
        (self.root / "response").write_bytes(response)
        second = copy.deepcopy(self.page)
        second["declared_exposed_names"] = ["second_alias", "first_alias"]
        self.plan["before"]["captures"][0]["pages"].append(second)
        self.plan["after"]["captures"] = [copy.deepcopy(self.plan["before"]["captures"][0])]
        original = copy.deepcopy(self.plan)
        status, output, diagnostic = self.execute()
        self.assertEqual((status, diagnostic), (0, b""))
        decoded = json.loads(output)
        expected = copy.deepcopy(original)
        for side in ("before", "after"):
            for capture in expected[side]["captures"]:
                for page in capture["pages"]:
                    page["request_json"] = request.decode("utf-8")
                    page["response_json"] = response.decode("utf-8")
                    del page["request_file"], page["response_file"]
        self.assertEqual(decoded, expected)
        pages = decoded["before"]["captures"][0]["pages"]
        self.assertEqual(pages[0]["request_json"].encode(), request)
        self.assertEqual(pages[0]["response_json"].encode(), response)
        self.assertEqual(pages[1]["declared_exposed_names"], ["second_alias", "first_alias"])
        self.assertEqual(self.plan, original)
        self.assertEqual(list(pages[0]), ["context", "request_json", "response_json", "declared_exposed_names"])

    def test_later_missing_frame_keeps_stdout_empty(self):
        later = copy.deepcopy(self.page)
        later["response_file"] = "../missing-later"
        self.plan["before"]["captures"][0]["pages"].append(later)
        self.assertIn(b"pages[1].response_file", self.assert_refused())

    def test_future_metadata_numbers_preserve_exact_json_tokens(self):
        numbers = b'[-0,1E+04,1e-400,0.123456789012345678901234567890123456789,123456789012345678901234567890]'
        raw = json.dumps(self.plan).encode()
        raw = raw[:-1] + b',"future_metadata":{"numbers":' + numbers + b',"nested":[{"ratio":-0.0000000000000000000000000000000001}]}}'
        status, output, diagnostic = self.execute(raw=raw)
        self.assertEqual((status, diagnostic), (0, b""))
        self.assertIn(b'"numbers":' + numbers, output)
        self.assertIn(b'"ratio":-0.0000000000000000000000000000000001', output)
        # Independent parse callbacks compare the preserved token strings, not
        # float values (which would hide underflow, rounding and negative zero).
        original = json.loads(raw, parse_int=str, parse_float=str)
        assembled = json.loads(output, parse_int=str, parse_float=str)
        self.assertEqual(assembled["future_metadata"], original["future_metadata"])
        path = self.root / "plans" / "plan.json"
        parsed = ASSEMBLER.load_plan(path)
        number = parsed["future_metadata"]["numbers"][0]
        self.assertNotIsInstance(number, str)
        self.assertEqual(number.token, "-0")

    def test_invalid_utf8_is_refused_without_replacement(self):
        (self.root / "response").write_bytes(b'{"x":"\xc3("}')
        self.assertIn(b"INVALID_FRAME_UTF8", self.assert_refused())

    def test_frame_exact_64k_boundary_and_overflow(self):
        # Frame JSON semantics deliberately belong to the native importer.
        raw = b" " * ASSEMBLER.MAX_FRAME_BYTES
        (self.root / "request").write_bytes(raw)
        status, output, _ = self.execute()
        self.assertEqual(status, 0)
        self.assertEqual(json.loads(output)["before"]["captures"][0]["pages"][0]["request_json"].encode(), raw)
        (self.root / "request").write_bytes(raw + b" ")
        self.assertIn(b"BYTE_LIMIT", self.assert_refused())

    def test_inner_duplicate_nonfinite_and_invalid_json_are_preserved(self):
        # These are NOT rejected here. Existing native strictJSON owns that decision.
        request = b'{"same":1,"same":2,"n":NaN}\r\n'
        response = b"not JSON at all \n"
        (self.root / "request").write_bytes(request)
        (self.root / "response").write_bytes(response)
        status, output, diagnostic = self.execute()
        self.assertEqual((status, diagnostic), (0, b""))
        page = json.loads(output)["before"]["captures"][0]["pages"][0]
        self.assertEqual(page["request_json"].encode(), request)
        self.assertEqual(page["response_json"].encode(), response)

    def test_duplicate_plan_keys_and_nonfinite_plan_are_refused(self):
        for raw in [
            b'{"before":{"captures":[]},"before":{"captures":[]},"after":{"captures":[]}}',
            b'{"before":{"captures":[],"x":1,"\\u0078":2},"after":{"captures":[]}}',
            b'{"before":{"captures":[]},"after":{"captures":[]},"number":NaN}',
        ]:
            with self.subTest(raw=raw):
                self.assert_refused(raw=raw)

    def test_inline_file_pair_mixing_is_refused(self):
        for extra, remove in [
            ({"request_json": "{}"}, []),
            ({"response_json": "{}"}, ["response_file"]),
            ({"request_json": "{}", "response_json": "{}"}, ["request_file", "response_file"]),
        ]:
            plan = copy.deepcopy(self.plan)
            page = plan["before"]["captures"][0]["pages"][0]
            page.update(extra)
            for key in remove:
                del page[key]
            with self.subTest(extra=extra):
                self.assert_refused(plan=plan)

    def test_capture_page_and_output_bounds(self):
        plan = copy.deepcopy(self.plan)
        plan["before"]["captures"] *= 9
        self.assertIn(b"CAPTURE_LIMIT", self.assert_refused(plan=plan))
        plan = copy.deepcopy(self.plan)
        plan["before"]["captures"][0]["pages"] *= 17
        self.assertIn(b"PAGE_LIMIT", self.assert_refused(plan=plan))
        (self.root / "request").write_bytes(b"\x00" * ASSEMBLER.MAX_FRAME_BYTES)
        (self.root / "response").write_bytes(b"\x00" * ASSEMBLER.MAX_FRAME_BYTES)
        plan = copy.deepcopy(self.plan)
        plan["before"]["captures"][0]["pages"] *= 2
        self.assertIn(b"output: BYTE_LIMIT", self.assert_refused(plan=plan))
        self.assertIn(b"BYTE_LIMIT", self.assert_refused(raw=b" " * (ASSEMBLER.MAX_PLAN_BYTES + 1)))

    def test_directory_refused_and_symlink_to_regular_allowed(self):
        plan = copy.deepcopy(self.plan)
        plan["before"]["captures"][0]["pages"][0]["response_file"] = str(self.root)
        self.assert_refused(plan=plan)
        link = self.root / "response-link"
        link.symlink_to(self.root / "response")
        plan["before"]["captures"][0]["pages"][0]["response_file"] = str(link)
        self.assertEqual(self.execute(plan=plan)[0], 0)

    @unittest.skipUnless(hasattr(os, "mkfifo"), "FIFO unavailable on this platform")
    def test_fifo_refused_without_waiting_for_a_writer(self):
        fifo = self.root / "fifo"
        os.mkfifo(fifo)
        self.plan["before"]["captures"][0]["pages"][0]["response_file"] = str(fifo)
        path = self.root / "plans" / "fifo-plan.json"
        path.write_text(json.dumps(self.plan))
        result = subprocess.run(
            [sys.executable, "-I", "-B", str(ROOT / "assemble_saved_frames.py"), str(path)],
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=2,
        )
        self.assertEqual(result.returncode, 1)
        self.assertEqual(result.stdout, b"")
        self.assertIn(b"REGULAR_FILE_REQUIRED", result.stderr)

    def test_actual_stdout_failure_can_leave_a_prefix_but_returns_one(self):
        class FailedOutput:
            def __init__(self):
                self.prefix = b""

            def write(self, raw):
                if self.prefix:
                    raise OSError("simulated destination failure")
                self.prefix = raw[:7]
                return 7

            def flush(self):
                pass

        path = self.root / "plans" / "plan.json"
        path.write_text(json.dumps(self.plan))
        output, diagnostics = FailedOutput(), io.BytesIO()
        self.assertEqual(ASSEMBLER.run([str(path)], output, diagnostics), 1)
        self.assertEqual(len(output.prefix), 7)
        self.assertTrue(diagnostics.getvalue())


if __name__ == "__main__":
    unittest.main(verbosity=2)
