"""Offline refusal regressions; no Go, MCP executable, or network is run."""
import copy
import gzip
import importlib.util
import io
import os
from pathlib import Path
import struct
import tarfile
import tempfile
import unittest
from unittest.mock import patch

SPEC = importlib.util.spec_from_file_location("mcp_bundle_under_test", Path(__file__).with_name("service_grant_mcp_bundle.py"))
M = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(M)
COMMIT = M.REFERENCE_SOURCE


def build():
    settings = {"-buildmode": "exe", "-compiler": "gc", "-trimpath": "true", "CGO_ENABLED": "0",
                "GOOS": "darwin", "GOARCH": "arm64", "GOARM64": "v8.0", "vcs": "git",
                "vcs.revision": COMMIT, "vcs.time": "2026-10-19T00:16:10Z", "vcs.modified": "false"}
    return {"GoVersion": "go1.27.2", "Path": M.MODULE,
            "Main": {"Path": M.MODULE, "Version": "v0.0.0-20261019001610-"+COMMIT[:12]},
            "Deps": copy.deepcopy([x["dependency"] for x in M.DEPENDENCIES]),
            "Settings": [{"Key": k, "Value": v} for k, v in settings.items()]}


class BundleRefusals(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(dir=os.environ.get("MCP_BUNDLE_TEST_TMP"))
        self.root = Path(self.temp.name)

    def tearDown(self):
        self.temp.cleanup()

    def test_real_build_shape_and_bad_identity_controls(self):
        M.build_identity(build(), "darwin/arm64", COMMIT)
        mutations = [
            ("wrong-Go", lambda x: x.update(GoVersion="go1.26.0")),
            ("wrong-package", lambda x: x.update(Path=M.MODULE+"/cmd/evaluate")),
            ("main-replace", lambda x: x["Main"].update(Replace={})),
            ("dependency-replace", lambda x: x["Deps"][0].update(Replace={})),
            ("dependency-sum", lambda x: x["Deps"][0].update(Sum="h1:wrong")),
            ("extra-dependency", lambda x: x["Deps"].append(copy.deepcopy(x["Deps"][0]))),
            ("missing-dependency", lambda x: x["Deps"].pop()),
            ("CGO", lambda x: next(r for r in x["Settings"] if r["Key"] == "CGO_ENABLED").update(Value="1")),
            ("dirty", lambda x: next(r for r in x["Settings"] if r["Key"] == "vcs.modified").update(Value="true")),
            ("other-source", lambda x: next(r for r in x["Settings"] if r["Key"] == "vcs.revision").update(Value="0"*40)),
            ("duplicate-setting", lambda x: x["Settings"].append(copy.deepcopy(x["Settings"][0]))),
        ]
        for label, mutate in mutations:
            with self.subTest(label=label):
                value = build()
                mutate(value)
                with self.assertRaises(ValueError):
                    M.build_identity(value, "darwin/arm64", COMMIT)

    def test_duplicate_and_nonfinite_json(self):
        for data in (b'{"a":1,"a":2}', b'{"a":NaN}'):
            with self.subTest(data=data), self.assertRaises(ValueError):
                M.load(data)

    def test_exact_types_refuse_boolean_as_zero(self):
        with self.assertRaises(ValueError):
            M.exact({"exit": False}, {"exit": 0})

    def test_missing_binary_buildinfo_and_wrong_report_binary_refused(self):
        with self.assertRaises(ValueError):
            M.embedded_info(b"not a compiled Go command")
        report = {"status": "INDEPENDENT_WIRE_TRIAL_PASS", "observations": [],
                  "binary_sha256": "1"*64, "build_identity": build(), "sdk_client_calls": 0,
                  "model_calls": 0, "external_adoption": False, "scope": "finite"}
        with self.assertRaisesRegex(ValueError, "wire binary"):
            M.checked_report(report, build(), "2"*64, self.root, {})

    def test_macho_architecture_and_nonbinary(self):
        # Header retained from the real clean darwin/arm64 build.
        header = bytes.fromhex("cffaedfe0c000001000000000200000012000000d80a00000400200000000000")
        self.assertEqual(M.binary_platform(header), "darwin/arm64")
        wrong = bytearray(header)
        wrong[4:8] = struct.pack("<I", 0x01000007)
        for value in (bytes(wrong), b"#!/bin/sh\nexit 0\n"):
            with self.subTest(value=value[:8]), self.assertRaises(ValueError):
                M.binary_platform(value)

    def test_elf_arch_and_dynamic_interpreter(self):
        header = bytearray(120)
        header[:6] = b"\x7fELF\x02\x01"
        header[16:20] = struct.pack("<HH", 2, 62)
        header[32:40] = struct.pack("<Q", 64)
        header[54:58] = struct.pack("<HH", 56, 1)
        self.assertEqual(M.binary_platform(header), "linux/amd64")
        for offset, data in [(18, struct.pack("<H", 183)), (64, struct.pack("<I", 3)), (64, struct.pack("<I", 2))]:
            value = bytearray(header)
            value[offset:offset+len(data)] = data
            with self.subTest(offset=offset, data=data), self.assertRaises(ValueError):
                M.binary_platform(value)

    def test_changed_notice_text_is_refused(self):
        with patch.object(M, "member", return_value=b"changed license bytes"):
            with self.assertRaisesRegex(ValueError, "dependency notice"):
                M.notice_files(self.root, self.root, self.root)

    def test_source_path_symlink_and_traversal_refused(self):
        (self.root/"real").mkdir()
        (self.root/"real/file").write_bytes(b"x")
        (self.root/"link").symlink_to(self.root/"real", target_is_directory=True)
        for path in ("link/file", "../file", "/file"):
            with self.subTest(path=path), self.assertRaises(ValueError):
                M.member(self.root, path)

    def archive(self, rows):
        path = self.root/"test.tar.gz"
        with path.open("wb") as f:
            with gzip.GzipFile(fileobj=f, mode="wb", mtime=0, filename="") as gz:
                with tarfile.open(fileobj=gz, mode="w", format=tarfile.USTAR_FORMAT) as archive:
                    for name, kind, raw in rows:
                        member = tarfile.TarInfo(name)
                        member.mode = 0o755
                        member.type = kind
                        member.size = len(raw) if kind == tarfile.REGTYPE else 0
                        member.linkname = "elsewhere" if kind == tarfile.SYMTYPE else ""
                        archive.addfile(member, io.BytesIO(raw) if member.isfile() else None)
        return path

    def test_valid_actual_archive_extraction(self):
        raw = b"actual archive bytes"
        path = self.archive([(M.COMMAND, tarfile.REGTYPE, raw)])
        expected = {M.COMMAND: (raw, 0o755)}
        M.extract_checked(path, expected, self.root/"extracted")
        self.assertEqual((self.root/"extracted"/M.COMMAND).read_bytes(), raw)

    def test_symlink_duplicate_traversal_and_changed_archive_members(self):
        cases = [
            [(M.COMMAND, tarfile.SYMTYPE, b"")],
            [(M.COMMAND, tarfile.REGTYPE, b"x"), (M.COMMAND, tarfile.REGTYPE, b"x")],
            [("../escape", tarfile.REGTYPE, b"x")],
            [(M.COMMAND, tarfile.REGTYPE, b"wrong")],
        ]
        for rows in cases:
            with self.subTest(rows=rows), self.assertRaises(ValueError):
                M.safe_archive(self.archive(rows), {M.COMMAND: (b"x", 0o755)})
        with self.assertRaises(ValueError):
            M.safe_archive(self.root/"test.tar.gz", {"../escape": (b"x", 0o755)})

    def test_truncated_or_concatenated_gzip_refused(self):
        p = self.archive([(M.COMMAND, tarfile.REGTYPE, b"x")])
        original = p.read_bytes()
        for raw in (original[:-4], original+original):
            p.write_bytes(raw)
            with self.subTest(bytes=len(raw)), self.assertRaises((ValueError, EOFError)):
                M.safe_archive(p, {M.COMMAND: (b"x", 0o755)})

    def test_child_cleanup_failure_is_not_success_and_keeps_primary(self):
        closed = io.BytesIO()
        closed.close()
        class Child:
            pid = 54321
            returncode = 0
            stdin = stdout = stderr = closed
            def poll(self): return 0
            def wait(self, timeout): return 0
        tracker = M.OwnedChildren(self.root)
        tracker.children.append(Child())
        with patch.object(M, "absent", return_value=False), patch.object(M.os, "killpg") as kill:
            with self.assertRaisesRegex(ValueError, "owned child group remains"):
                tracker.close()
            self.assertEqual({call.args[0] for call in kill.call_args_list}, {54321})
        primary = RuntimeError("original wire failure")
        with patch.object(M, "absent", return_value=False), patch.object(M.os, "killpg"):
            with self.assertRaises(RuntimeError) as caught:
                tracker.close(primary)
        self.assertIs(caught.exception, primary)

    def test_committed_candidate_cannot_skip_extracted_native_check(self):
        out = self.root/"out"
        out.mkdir()
        with self.assertRaisesRegex(ValueError, "requires fresh"):
            M.produce(self.root, self.root, self.root, self.root, self.root, self.root,
                      out, COMMIT, COMMIT, False)

    def test_closure_write_failure_keeps_existing_primary_or_becomes_first(self):
        tracker = M.OwnedChildren(self.root)
        primary = RuntimeError("original wire failure")
        diagnostic = OSError("closure record write failure")
        with patch.object(M.Path, "write_bytes", side_effect=diagnostic):
            with self.assertRaises(RuntimeError) as caught:
                tracker.close(primary)
            self.assertIs(caught.exception, primary)
            if hasattr(primary, "add_note"):
                self.assertIn("child closure diagnostic", " ".join(primary.__notes__))
            with self.assertRaises(OSError) as caught_without_primary:
                tracker.close()
            self.assertIs(caught_without_primary.exception, diagnostic)


if __name__ == "__main__":
    unittest.main(verbosity=2)
