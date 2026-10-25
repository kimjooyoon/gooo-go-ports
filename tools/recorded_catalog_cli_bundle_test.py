"""Refusal and publication controls for the saved-catalog distribution boundary."""
import copy
import gzip
import importlib.util
import io
import json
import os
from pathlib import Path
import struct
import sys
import tarfile
import tempfile
import unittest
from unittest import mock

SPEC = importlib.util.spec_from_file_location("catalog_bundle", Path(__file__).with_name("recorded_catalog_cli_bundle.py"))
b = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(b)


def build():
    return {"GoVersion": "go1.27.3", "Path": b.MODULE + "/cmd/compare",
            "Main": {"Path": b.MODULE, "Version": "(devel)"},
            "Settings": [{"Key": k, "Value": v} for k, v in {
                "-buildmode": "exe", "-compiler": "gc", "-trimpath": "true", "CGO_ENABLED": "0",
                "GOOS": "linux", "GOARCH": "amd64", "GOAMD64": "v1"}.items()]}


def uv(value):
    result = bytearray()
    while value >= 128:
        result.append((value & 127) | 128); value >>= 7
    result.append(value)
    return bytes(result)


def embedded(lines):
    version = b"go1.27.3"
    payload = bytes.fromhex("3077af0c9274080241e1c107e6d618e6") + lines.encode() + bytes.fromhex("f932433186182072008242104116d8f2")
    header = b"\xff Go buildinf:" + bytes([8, 2]) + bytes(16)
    return header + uv(len(version)) + version + uv(len(payload)) + payload


class Contracts(unittest.TestCase):
    def test_exact_json_rejects_bool_integer_substitution(self):
        for left, right in [(True, 1), ({"v": False}, {"v": 0}), ([True], [1])]:
            with self.assertRaises(ValueError):
                b.exact(left, right)

    def test_strict_json_refuses_duplicate_and_nonfinite(self):
        for raw in (b'{"x":1,"x":2}', b'{"x":NaN}', b'{"x":Infinity}'):
            with self.assertRaises(ValueError):
                b.load(raw)

    def test_retained_size_is_not_a_boolean(self):
        with self.assertRaises(ValueError):
            b.matches(b"x", {"bytes": True, "sha256": b.sha(b"x")})

    def test_no_missing_vcs_identity_is_invented(self):
        value = build()
        b.build_identity(value, "linux/amd64", "go1.27.3")
        for mutation in (
            lambda x: x["Main"].update(Version="v0.0.0-20261025120000-abcdef012345"),
            lambda x: x["Main"].update(Replace={"Path": "local"}),
            lambda x: x.update(Deps=[{"Path": "unexpected"}]),
            lambda x: x["Settings"].append({"Key": "vcs.revision", "Value": "a" * 40}),
            lambda x: x["Settings"].append({"Key": "CGO_ENABLED", "Value": "1"}),
        ):
            changed = copy.deepcopy(value); mutation(changed)
            with self.assertRaises(ValueError):
                b.build_identity(changed, "linux/amd64", "go1.27.3")

    def test_embedded_info_preserves_devel_and_refuses_dependencies(self):
        text = "path\t" + b.MODULE + "/cmd/compare\nmod\t" + b.MODULE + "\t(devel)\t\nbuild\tCGO_ENABLED=0\n"
        actual = b.embedded_info(embedded(text))
        self.assertEqual(actual["Main"], {"Path": b.MODULE, "Version": "(devel)"})
        for changed in (text + "dep\texample.com/x\tv1.0.0\th1:abc\n", text + "path\tduplicate\n"):
            with self.assertRaises(ValueError):
                b.embedded_info(embedded(changed))
        with self.assertRaises(ValueError):
            b.embedded_info(embedded(text) + embedded(text))
        with self.assertRaises(ValueError):
            b.embedded_info(embedded(text)[:-1])

    def test_platform_rejects_dynamic_elf(self):
        raw = bytearray(120); raw[:6] = b"\x7fELF\x02\x01"
        raw[16:20] = struct.pack("<HH", 2, 62); raw[32:40] = struct.pack("<Q", 64)
        raw[54:58] = struct.pack("<HH", 56, 1)
        self.assertEqual(b.binary_platform(bytes(raw)), "linux/amd64")
        raw[64:68] = struct.pack("<I", 3)
        with self.assertRaises(ValueError):
            b.binary_platform(bytes(raw))

    def test_roster_includes_added_runtime_source_not_just_declared_subset(self):
        tree = {b.PROFILE + "main.go": (), b.PROFILE + "newkernel/new.go": (),
                b.PROFILE + "README.md": (), "examples/explicit-observation-availability/new.json": (),
                "tools/unrelated.py": ()}
        roster = b.source_roster(tree)
        self.assertTrue(b.SOURCE_EXTRAS <= roster)
        self.assertIn(b.PROFILE + "newkernel/new.go", roster)
        self.assertIn("examples/explicit-observation-availability/new.json", roster)
        self.assertNotIn(b.PROFILE + "README.md", roster)

    def test_member_refuses_parent_symlink_and_traversal(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp); (root / "real").mkdir(); (root / "real/value").write_bytes(b"ok")
            (root / "alias").symlink_to(root / "real", target_is_directory=True)
            self.assertEqual(b.member(root, "real/value"), b"ok")
            for name in ("alias/value", "../value", "/value", "real//value"):
                with self.assertRaises(ValueError):
                    b.member(root, name)


class Archives(unittest.TestCase):
    def test_deterministic_exact_members_and_trailing_refusal(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp); files = {"run": (b"binary", 0o755), "START-HERE.txt": (b"guide\n", 0o644)}
            a, c = root / "a.tgz", root / "c.tgz"
            b.write_archive(a, files); b.write_archive(c, files)
            self.assertEqual(a.read_bytes(), c.read_bytes())
            original = a.read_bytes()
            for suffix in (b"junk", gzip.compress(b"", mtime=0)):
                a.write_bytes(original + suffix)
                with self.assertRaises(ValueError):
                    b.verify_archive(a, files)
            a.write_bytes(original)
            changed = dict(files, run=(b"binary", 0o644))
            with self.assertRaises(ValueError):
                b.verify_archive(a, changed)

    def test_archive_symlink_member_is_refused(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "bad.tgz"
            with tarfile.open(path, "w:gz") as archive:
                item = tarfile.TarInfo("run"); item.type = tarfile.SYMTYPE; item.linkname = "/tmp/value"
                archive.addfile(item)
            with self.assertRaises(ValueError):
                b.verify_archive(path, {"run": (b"", 0o755)})

    def test_failure_removes_candidate_and_never_publishes_archive(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp); out = root / "out"
            with mock.patch.object(b, "check_evidence", return_value={}), \
                 mock.patch.object(b, "archive_files", return_value={"run": (b"test", 0o755)}), \
                 mock.patch.object(b, "smoke", side_effect=ValueError("actual control mismatch")):
                with self.assertRaisesRegex(ValueError, "actual control mismatch"):
                    b.produce(root, root, root / "export", "a" * 40, out)
            self.assertEqual([p.name for p in out.iterdir()], ["first-failure.json"])
            self.assertEqual(json.loads((out / "first-failure.json").read_text())["status"], "FIRST_FAILURE_NO_PUBLISHABLE_ARCHIVE")


class OwnedCommands(unittest.TestCase):
    def run_python(self, code, *, seconds=2, cap=1024, input_raw=b""):
        records = []
        with tempfile.TemporaryDirectory() as tmp:
            output = b.run_owned([sys.executable, "-I", "-c", code], input_raw, Path(tmp),
                                 {"PATH": ""}, "control", records, seconds=seconds, cap=cap)
        return output, records

    def test_concurrent_stdin_stdout_and_clean_closure(self):
        (out, err, code), records = self.run_python("import sys; x=sys.stdin.buffer.read(); sys.stdout.buffer.write(x)", input_raw=b"x" * 100000, cap=100000)
        self.assertEqual((len(out), err, code), (100000, b"", 0))
        self.assertTrue(records[0]["process_group_closed"])

    def test_large_output_is_refused(self):
        with self.assertRaisesRegex(ValueError, "output exceeds"):
            self.run_python("import sys; sys.stdout.buffer.write(b'x'*100000)", cap=256)

    def test_deadline_is_enforced(self):
        with self.assertRaisesRegex(ValueError, "timed out"):
            self.run_python("import time; time.sleep(10)", seconds=0.1)


if __name__ == "__main__":
    unittest.main()
