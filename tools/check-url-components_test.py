#!/usr/bin/env python3
"""Transport admission tests; fresh native receipt controls run in the verifier."""
import importlib.util
import json
from pathlib import Path
import unittest


SPEC = importlib.util.spec_from_file_location("url_component_check", Path(__file__).with_name("check-url-components.py"))
M = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(M)


def corpus(texts):
    return json.dumps({"schema": "gooo/body-composition-inputs/v1",
                       "inputs": [{M.ENTRY: text} for text in texts]}).encode()


class InputAdmission(unittest.TestCase):
    def test_byte_boundary_and_original_spelling(self):
        for text in ("", " " * 341, "\0" * 341, "한" * 113 + "ab", "é" * 170 + "a", "%2f+ ", "\\ud800", "\U00010000", "\U0010ffff"):
            with self.subTest(text=repr(text[:16])):
                self.assertEqual(M.inputs(corpus([text])), [{M.ENTRY: text}])
        self.assertEqual(len(M.inputs(corpus(["a"] * 128))), 128)

    def test_refuse_non_scalar_oversize_and_ambiguous_documents(self):
        bad = [corpus(["a" * 342]), corpus(["한" * 114]), corpus(["\ud800"]), corpus(["\udfff"]),
               corpus([]), corpus(["a"] * 129), corpus([False]), b'{"schema":null,"inputs":[]}',
               corpus(["a"]) + b'{}', b'\xff', b'[]', b'null', b' ' * ((1 << 20) + 1)]
        document = json.loads(corpus(["a"]))
        for change in ({"cases": []}, {"schema": "other"}, {"inputs": [{M.ENTRY + ".input0": "a"}]},
                       {"inputs": [{M.ENTRY: "a", "other": "b"}]}, {"inputs": "a"}):
            bad.append(json.dumps(dict(document, **change)).encode())
        bad.append(('{"schema":"gooo/body-composition-inputs/v1","inputs":[{"' + M.ENTRY + '":"a","' + M.ENTRY + '":"b"}]}').encode())
        for raw in bad:
            with self.subTest(raw=raw[:90]):
                with self.assertRaises((RuntimeError, ValueError, UnicodeError, TypeError)):
                    M.inputs(raw)

    def test_checked_in_corpus_has_distinct_boundaries(self):
        rows = M.inputs((Path(__file__).resolve().parent.parent / "examples/url-context-facts/inputs.json").read_bytes())
        texts = [row[M.ENTRY] for row in rows]
        self.assertEqual(len(texts), 128)
        self.assertEqual(len(set(texts)), 128)
        self.assertTrue(all(any(chr(octet) in text for text in texts) for octet in range(128)))
        self.assertIn("\U00010000", texts)
        self.assertIn("\U0010ffff", texts)
        self.assertIn("\0" * 341, texts)
        self.assertIn("/" * 341, texts)
        self.assertIn("%" * 341, texts)
        self.assertEqual(max(len(text.encode()) for text in texts), 341)


if __name__ == "__main__":
    unittest.main()
