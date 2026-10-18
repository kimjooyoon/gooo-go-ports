"""Synthetic packaging tests; fake command bytes are never executed."""
import copy
import importlib.util
import gzip
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest import mock

SPEC = importlib.util.spec_from_file_location("bundle", Path(__file__).with_name("api_policy_cli_bundle.py"))
BUNDLE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(BUNDLE)


class BundleTests(unittest.TestCase):
    def fixture(self, directory):
        root = Path(directory)
        go = root / "toolchain/bin/go"
        go.parent.mkdir(parents=True)
        go.write_bytes(b"fake Go path; not executable")
        upstream = root / "examples/api-policy-revision/profiles/original/oracle/upstream/LICENSE"
        upstream.parent.mkdir(parents=True)
        upstream.write_text("synthetic upstream license\n")
        notice = root / "examples/api-policy-revision/profiles/original/NOTICE.md"
        notice.write_text("Synthetic original notice with source and modification attribution.\n")
        (root / "LICENSE").write_text("synthetic repository license\n")
        (go.parent.parent / "LICENSE").write_text("synthetic Go license\n")
        license_path = root / "tools/licenses/gooo-83928fa-MIT.txt"
        license_path.parent.mkdir(parents=True)
        license_path.write_bytes((Path(__file__).parent / "licenses/gooo-83928fa-MIT.txt").read_bytes())
        binary = root / "tested-command"
        binary.write_bytes(b"synthetic unit-test command bytes; never run\n")
        binary.chmod(0o700)
        out = root / "out"
        out.mkdir()
        main = {"Path": BUNDLE.PARENT, "Version": "v0.0.0-20261018000000-111111111111", "Sum": "h1:" + "A" * 43 + "="}
        dep = {"Path": BUNDLE.ORIGINAL, "Version": "v0.0.0-20261014100709-8cddcc4e65af", "Sum": "h1:" + "B" * 43 + "="}
        build = {"GoVersion": "go1.27.2", "Path": BUNDLE.PARENT + "/cmd/compare", "Main": main, "Deps": [dep],
                 "Settings": [{"Key": key, "Value": value} for key, value in
                              (("CGO_ENABLED", "0"), ("GOOS", "darwin"), ("GOARCH", "arm64"))]}
        build_path = out / "build-info.json"
        build_path.write_text(json.dumps(build))
        ref = lambda path: {"path": str(path), "bytes": path.stat().st_size, "sha256": BUNDLE.sha(path.read_bytes())}
        identities = {"compiler_source_sha": BUNDLE.PIN, "original_generation_sha256": "sha256:" + "a" * 64,
                      "revision_generation_sha256": "sha256:" + "b" * 64}
        report = {"status": "PUBLIC_API_POLICY_CONSUMER_PASS", "temporary_sources_binaries_build_cache_removed": True,
                  "requested_revision": "1" * 40, "resolved": {"parent": {"Origin": {"Hash": "1" * 40}, "Version": main["Version"]}},
                  "go_toolchain": {"version": "go version go1.27.2 darwin/arm64", "executable": ref(go)},
                  "source": {"current_original_generation": identities["original_generation_sha256"],
                             "current_parent_generation": identities["revision_generation_sha256"]},
                  "consumer_fixture_source_contract": {"identities": identities},
                  "acceptance": {"direct_install": True, "direct_install_cases": 2,
                                 "direct_install_tampered_report_refused": True, "runtime_empty_path": True},
                  "direct_install": {"binary": ref(binary), "build_info_source": ref(build_path), "runtime_empty_path": True,
                                     "target": build["Path"] + "@" + main["Version"],
                                     "build_info": {"go_version": build["GoVersion"], "package": build["Path"],
                                                    "main": main, "dependencies": [dep], "no_replacements": True},
                                     "cases": [{"action": action, "baseline_complete": True, "comparison_complete": True,
                                                "replay_bytes_exact": True, "report_sha256": "c" * 64}
                                               for action in ("read", "write")],
                                     "tampered_report": {"field": "differences.reason", "exit_code": 1, "stdout_bytes": 0,
                                                         "diagnostic": {"status": "REFUSED", "diagnostics": [
                                                             {"code": "REPORT_MISMATCH", "stage": "verify_comparison"}]}}},
                  "bundle_facts": {}}
        for action in ("read", "write"):
            report["bundle_facts"][action] = {"group_present": True, "group_value": "admin", "principal_present": True,
                                            "principal_value": "synthetic:user", "authenticated": True,
                                            "caller_tenant": "tenant-a", "resource_tenant": "tenant-a",
                                            "requested_action": action, "allowed_action": action}
        return binary, out, report, go, root

    def test_deterministic_repack_has_exact_content_modes_and_licenses(self):
        with tempfile.TemporaryDirectory() as directory:
            args = self.fixture(directory)
            first = BUNDLE.make_bundle(*args)
            first_bytes = Path(first["archive"]["path"]).read_bytes()
            Path(first["archive"]["path"]).unlink()
            Path(first["checksums"]["path"]).unlink()
            second = BUNDLE.make_bundle(*args)
            self.assertEqual(first["status"], "VALIDATED_NATIVE_CLI_BUNDLE")
            self.assertEqual(first["archive"]["sha256"], second["archive"]["sha256"])
            self.assertEqual(first_bytes, Path(second["archive"]["path"]).read_bytes())
            with tarfile.open(first["archive"]["path"], "r:gz") as archive:
                self.assertEqual(len(archive.getmembers()), 10)
                self.assertEqual(archive.getmember("gooo-policy-compare").mode, 0o755)
                self.assertEqual(archive.extractfile("gooo-policy-compare").read(), args[0].read_bytes())
                self.assertEqual(archive.extractfile("NOTICE-original.md").read(),
                                 (args[4] / "examples/api-policy-revision/profiles/original/NOTICE.md").read_bytes())
                manifest = json.load(archive.extractfile("BUNDLE.json"))
                self.assertEqual(manifest["build_info"]["main"]["Sum"], args[2]["direct_install"]["build_info"]["main"]["Sum"])
                self.assertEqual(json.load(archive.extractfile("write-facts.json")), args[2]["bundle_facts"]["write"])
                self.assertIn(b"No Gooo compiler, Go or Python", archive.extractfile("START-HERE.txt").read())
            self.assertEqual(Path(first["checksums"]["path"]).read_text(), first["archive"]["sha256"] + "  " + Path(first["archive"]["path"]).name + "\n")

    def test_failure_status_cleanup_or_finite_checks_cannot_package(self):
        for change in (lambda d: d.update(status="FAIL"),
                       lambda d: d.update(temporary_sources_binaries_build_cache_removed=False),
                       lambda d: d["acceptance"].update(direct_install_tampered_report_refused=False),
                       lambda d: d["direct_install"]["cases"][1].update(replay_bytes_exact=False)):
            with self.subTest(change=change), tempfile.TemporaryDirectory() as directory:
                args = self.fixture(directory)
                change(args[2])
                with self.assertRaises(ValueError):
                    BUNDLE.make_bundle(*args)
                self.assertEqual({path.name for path in args[1].iterdir()}, {"build-info.json"})

    def test_binary_identity_size_and_symlink_are_refused(self):
        for kind in ("bytes", "oversize", "symlink"):
            with self.subTest(kind=kind), tempfile.TemporaryDirectory() as directory:
                args = self.fixture(directory)
                if kind == "bytes":
                    args[0].write_bytes(b"different command")
                elif kind == "oversize":
                    with args[0].open("wb") as stream:
                        stream.truncate(BUNDLE.MAX_BINARY + 1)
                else:
                    saved = args[0].with_suffix(".saved")
                    args[0].rename(saved)
                    args[0].symlink_to(saved)
                with self.assertRaises(ValueError):
                    BUNDLE.make_bundle(*args)

    def test_raw_buildinfo_hash_platform_cgo_and_replacement_are_bound(self):
        for kind in ("stored-hash", "outside-output", "platform", "cross-build", "cgo", "replace", "summary"):
            with self.subTest(kind=kind), tempfile.TemporaryDirectory() as directory:
                args = self.fixture(directory)
                ref = args[2]["direct_install"]["build_info_source"]
                path = Path(ref["path"])
                build = json.loads(path.read_bytes())
                if kind == "platform": build["Settings"][1]["Value"] = "windows"
                if kind == "cross-build": args[2]["go_toolchain"]["version"] = "go version go1.27.2 linux/amd64"
                if kind == "cgo": build["Settings"][0]["Value"] = "1"
                if kind == "replace": build["Main"]["Replace"] = {"Path": "local"}
                if kind == "summary": args[2]["direct_install"]["build_info"]["main"]["Sum"] = "h1:" + "C" * 43 + "="
                path.write_text(json.dumps(build) + "\n")
                if kind == "outside-output":
                    path.rename(args[4] / "outside-build-info.json")
                    path = args[4] / "outside-build-info.json"
                    ref["path"] = str(path)
                if kind != "stored-hash":
                    ref.update(bytes=path.stat().st_size, sha256=BUNDLE.sha(path.read_bytes()))
                with self.assertRaises(ValueError):
                    BUNDLE.make_bundle(*args)

    def test_gzip_corruption_and_nonregular_tar_member_are_refused(self):
        with tempfile.TemporaryDirectory() as directory:
            args = self.fixture(directory)
            result = BUNDLE.make_bundle(*args)
            path = Path(result["archive"]["path"])
            raw = bytearray(path.read_bytes())
            raw[-8] ^= 1  # gzip CRC; tar headers/content remain otherwise untouched.
            path.write_bytes(raw)
            with self.assertRaisesRegex(gzip.BadGzipFile, "CRC check failed"):
                BUNDLE.verify_archive(path, {})
            with tarfile.open(path, "w:gz") as archive:
                member = tarfile.TarInfo("command")
                member.type, member.linkname, member.mode = tarfile.SYMTYPE, "elsewhere", 0o755
                archive.addfile(member)
            with self.assertRaises(ValueError):
                BUNDLE.verify_archive(path, {"command": (b"", 0o755)})

    def test_partial_output_removed_and_existing_files_never_overwritten(self):
        with tempfile.TemporaryDirectory() as directory:
            args = self.fixture(directory)
            with mock.patch.object(BUNDLE, "verify_archive", side_effect=ValueError("readback failed")):
                with self.assertRaisesRegex(ValueError, "readback failed"):
                    BUNDLE.make_bundle(*args)
            self.assertEqual({path.name for path in args[1].iterdir()}, {"build-info.json"})
            result = BUNDLE.make_bundle(*args)
            saved = Path(result["archive"]["path"]).read_bytes()
            with self.assertRaises(FileExistsError):
                BUNDLE.make_bundle(*args)
            self.assertEqual(Path(result["archive"]["path"]).read_bytes(), saved)

    def test_cleanup_error_preserves_primary_failure_and_attempts_other_outputs(self):
        with tempfile.TemporaryDirectory() as directory:
            args = self.fixture(directory)
            original_read, original_unlink = BUNDLE.read, Path.unlink
            attempted = []
            def fail_checksum_read(path, cap):
                if path.name == "SHA256SUMS":
                    raise ValueError("primary checksum readback failure")
                return original_read(path, cap)
            def fail_one_cleanup(path, **kwargs):
                attempted.append(path.name)
                if path.name == "SHA256SUMS":
                    raise OSError("synthetic cleanup refusal")
                return original_unlink(path, **kwargs)
            with mock.patch.object(BUNDLE, "read", side_effect=fail_checksum_read), \
                 mock.patch.object(Path, "unlink", fail_one_cleanup):
                with self.assertRaisesRegex(ValueError, "primary checksum readback failure") as caught:
                    BUNDLE.make_bundle(*args)
            self.assertEqual(attempted, ["SHA256SUMS", "gooo-policy-compare-darwin-arm64.tar.gz"])
            self.assertFalse((args[1] / attempted[1]).exists())
            if hasattr(caught.exception, "__notes__"):
                self.assertIn("synthetic cleanup refusal", caught.exception.__notes__[0])

    def test_exact_facts_types_unicode_and_bounds_are_preserved(self):
        with tempfile.TemporaryDirectory() as directory:
            args = self.fixture(directory)
            original = args[2]["bundle_facts"]["read"]
            for kind in ("unknown", "boolean", "surrogate", "oversize"):
                changed = copy.deepcopy(original)
                if kind == "unknown": changed["extra"] = True
                if kind == "boolean": changed["authenticated"] = 1
                if kind == "surrogate": changed["principal_value"] = "\ud800"
                if kind == "oversize": changed["group_value"] = "x" * 65537
                with self.subTest(kind=kind), self.assertRaises(ValueError):
                    BUNDLE.facts(changed, "read")


if __name__ == "__main__":
    unittest.main()
