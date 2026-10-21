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
        build = {"GoVersion": "go1.27.3", "Path": BUNDLE.PARENT + "/cmd/compare", "Main": main, "Deps": [dep],
                 "Settings": [{"Key": key, "Value": value} for key, value in
                              (("CGO_ENABLED", "0"), ("GOOS", "darwin"), ("GOARCH", "arm64"))]}
        build_path = out / "build-info.json"
        build_path.write_text(json.dumps(build))
        ref = lambda path: {"path": str(path), "bytes": path.stat().st_size, "sha256": BUNDLE.sha(path.read_bytes())}
        identities = {"compiler_source_sha": BUNDLE.PIN, "original_generation_sha256": "sha256:" + "a" * 64,
                      "revision_generation_sha256": "sha256:" + "b" * 64}
        report = {"status": "PUBLIC_API_POLICY_CONSUMER_PASS", "temporary_sources_binaries_build_cache_removed": True,
                  "requested_revision": "1" * 40, "resolved": {"parent": {"Origin": {"Hash": "1" * 40}, "Version": main["Version"]}},
                  "go_toolchain": {"version": "go version go1.27.3 darwin/arm64", "executable": ref(go)},
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

    def batch_report(self, report):
        report["acceptance"].update(direct_install_batch_rows=3, direct_install_batch_tampered_report_refused=True)
        report["direct_install"]["batch"] = {
            "rows": 3, "order": ["write", "read", "write"], "complete_records_exact": True,
            "replay_bytes_exact": True, "comparison_sha256": "d" * 64,
            "late_tampered_report": {"exit_code": 1, "stdout_bytes": 0, "diagnostic": {
                "schema": "gooo/api-policy-revision-diagnostic/v1", "command": "compare", "mode": "replay",
                "status": "REFUSED", "diagnostics": [{"severity": "error", "code": "REPORT_MISMATCH",
                    "stage": "verify_comparison", "message": "synthetic changed report", "item_index": 2}]}}}

    def test_batch_guide_and_manifest_require_observed_batch_checks(self):
        for batch in (False, True):
            with self.subTest(batch=batch), tempfile.TemporaryDirectory() as directory:
                args = self.fixture(directory)
                if batch:
                    self.batch_report(args[2])
                result = BUNDLE.make_bundle(*args)
                with tarfile.open(result["archive"]["path"], "r:gz") as archive:
                    guide = archive.extractfile("START-HERE.txt").read()
                    manifest = json.load(archive.extractfile("BUNDLE.json"))
                self.assertEqual(b"--batch" in guide, batch)
                self.assertEqual("batch" in manifest["finite_checks"], batch)
                if batch:
                    self.assertEqual(manifest["finite_checks"]["batch"], args[2]["direct_install"]["batch"])
                    self.assertIn(b"cat read-facts.json", guide)
                    self.assertIn(b"cat write-facts.json", guide)
                    self.assertIn(b"item_index is zero-based", guide)

    def test_partial_or_failed_batch_proof_cannot_advertise_batch(self):
        for kind in ("missing-proof", "missing-acceptance", "order", "replay", "tamper-exit", "partial-output", "item-index"):
            with self.subTest(kind=kind), tempfile.TemporaryDirectory() as directory:
                args = self.fixture(directory)
                self.batch_report(args[2])
                direct = args[2]["direct_install"]
                if kind == "missing-proof": del direct["batch"]
                elif kind == "missing-acceptance": del args[2]["acceptance"]["direct_install_batch_rows"]
                elif kind == "order": direct["batch"]["order"] = ["read", "write", "read"]
                elif kind == "replay": direct["batch"]["replay_bytes_exact"] = False
                elif kind == "tamper-exit": direct["batch"]["late_tampered_report"]["exit_code"] = 0
                elif kind == "partial-output": direct["batch"]["late_tampered_report"]["stdout_bytes"] = 1
                else: direct["batch"]["late_tampered_report"]["diagnostic"]["diagnostics"][0]["item_index"] = 0
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
                if kind == "cross-build": args[2]["go_toolchain"]["version"] = "go version go1.27.3 linux/amd64"
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


SERVICE_SPEC = importlib.util.spec_from_file_location("service_bundle", Path(__file__).with_name("service_policy_cli_bundle.py"))
SERVICE = importlib.util.module_from_spec(SERVICE_SPEC)
SERVICE_SPEC.loader.exec_module(SERVICE)


class ServiceBundleTests(unittest.TestCase):
    def build(self):
        return {"GoVersion": "go1.27.3", "Path": SERVICE.MODULE + "/cmd/evaluate",
                "Main": {"Path": SERVICE.MODULE, "Version": SERVICE.VERSION, "Sum": SERVICE.MODULE_SUM},
                "Settings": [{"Key": key, "Value": value} for key, value in
                             (("CGO_ENABLED", "0"), ("GOOS", "darwin"), ("GOARCH", "arm64"))]}

    def fixture(self, directory):
        root = Path(directory)
        repository = Path(__file__).resolve().parent.parent
        for relative, _ in SERVICE.LICENSES.values():
            target = root / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes((repository / relative).read_bytes())
        go = root / "go/bin/go"
        go.parent.mkdir(parents=True)
        go.write_bytes(b"fake Go, never executed")
        (go.parent.parent / "LICENSE").write_bytes(b"synthetic Go license for file-only tests")
        generation = {"origin": "fresh_pinned_generation", "assets": {"entry": "sha256:" + "a" * 64}}
        source = {"LICENSE": (root / "LICENSE").read_bytes(),
                  "NOTICE.md": (root / "examples/agentmesh-service-policy/NOTICE.md").read_bytes(),
                  "LICENSE-APACHE-2.0": (root / "examples/agentmesh-service-policy/LICENSE-APACHE-2.0").read_bytes()}
        source["upstream/LICENSE"] = source["LICENSE-APACHE-2.0"]
        build = SERVICE.build_identity(self.build(), "darwin/arm64")
        return root, go, generation, source, build

    def test_native_public_buildinfo_refuses_wrong_identity_not_just_metadata_label(self):
        for mutate in (lambda b: b.update(GoVersion="go1.26.0"),
                       lambda b: b.update(Path=SERVICE.MODULE + "/cmd/browser"),
                       lambda b: b["Main"].update(Version="(devel)"),
                       lambda b: b["Main"].update(Sum="h1:" + "B" * 43 + "="),
                       lambda b: b["Main"].update(Replace={"Path": "local"}),
                       lambda b: b.update(Deps=[{"Path": "unexpected"}]),
                       lambda b: b["Settings"][0].update(Value="1"),
                       lambda b: b["Settings"][1].update(Value="linux")):
            changed = self.build()
            mutate(changed)
            with self.subTest(mutate=mutate), self.assertRaises(ValueError):
                SERVICE.build_identity(changed, "darwin/arm64")
        self.assertEqual(SERVICE.build_identity(self.build(), "darwin/arm64")["dependencies"], [])
        changed = self.build()
        changed["Deps"] = []
        self.assertEqual(SERVICE.build_identity(changed, "darwin/arm64")["main"]["Sum"], SERVICE.MODULE_SUM)

    def test_four_control_bytes_and_typed_full_bindings(self):
        expected_hashes = ["eaa120445bed9038ab6d9a4e71949344f6d92c44bad73fc7d713b8fc17c0dcb4",
                           "947f10f40652f17eb97c9f7d7a372a493a9eca5e24976ab7dbbe9c1863bac34e",
                           "a9f4c61b9e64f46c6c81b94610ed3d5181b735d3532eb10417b6e52da01c2bbc",
                           "d744fc59eceac73a8cc68430047424c5b065137f4807daea9a411318a220cc0a"]
        generation = {"assets": {"entry": "sha256:" + "a" * 64}}
        for index, (_, raw, facts, decision) in enumerate(SERVICE.controls()):
            self.assertEqual(SERVICE.sha(raw), expected_hashes[index])
            if facts is None:
                SERVICE.checked_result(b"", b"case0: expected exact JSON Text service facts\n", 1, facts, decision, generation)
                with self.assertRaises(ValueError):
                    SERVICE.checked_result(b"[]", b"", 0, facts, decision, generation)
                continue
            row = {"facts": facts, "decision": decision, "bindings": {
                "compiler_source_sha": SERVICE.PIN, "generation_sha256": "sha256:" + SERVICE.GENERATION,
                "generation_origin": "fresh_pinned_generation", "assets": generation["assets"]}}
            SERVICE.checked_result(SERVICE.encoded([row]), b"", 0, facts, decision, generation)
            for kind in ("bool-as-int", "missing-field", "asset", "fact", "generation", "stderr"):
                changed = copy.deepcopy(row)
                stderr = b""
                if kind == "bool-as-int": changed["decision"]["required_observed"] = int(decision["required_observed"])
                elif kind == "missing-field": del changed["decision"]["subset_rule_id"]
                elif kind == "asset": changed["bindings"]["assets"]["entry"] = "sha256:" + "b" * 64
                elif kind == "fact": changed["facts"]["before"] = 1
                elif kind == "generation": changed["bindings"]["generation_sha256"] = "sha256:" + "b" * 64
                else: stderr = b"unexpected"
                with self.subTest(index=index, kind=kind), self.assertRaises(ValueError):
                    SERVICE.checked_result(SERVICE.encoded([changed]), stderr, 0, facts, decision, generation)

    def test_duplicate_json_nonfinite_and_output_structural_errors_refused(self):
        for raw in (b'{"x":1,"x":2}', b'{"x":NaN}', b'{"x":Infinity}'):
            with self.assertRaises(ValueError):
                SERVICE.load(raw)

    def test_exact_twelve_regular_members_preserve_real_licenses_and_generator_scope(self):
        with tempfile.TemporaryDirectory() as directory:
            root, go, generation, source, build = self.fixture(directory)
            files = SERVICE.archive_files(b"fake native executable; never run", build, generation, source, root, go, "1" * 40)
            path = root / "candidate.tar.gz"
            first = SERVICE.write_archive(path, files)
            self.assertEqual(len(files), 12)
            self.assertEqual(files["LICENSE"][0], source["LICENSE"])
            self.assertEqual(SERVICE.sha(files["LICENSE"][0]), SERVICE.LICENSES["LICENSE"][1])
            self.assertEqual(files["LICENSE-AgentMesh-Apache-2.0"][0], source["upstream/LICENSE"])
            self.assertIn(b"no engine binary", files["START-HERE.txt"][0])
            manifest = SERVICE.load(files["BUNDLE.json"][0])
            self.assertEqual(manifest["source_commit"], SERVICE.REVISION)
            self.assertEqual(manifest["packaging_checkout_commit"], "1" * 40)
            self.assertFalse(manifest["finite_checks"]["grant_applied"])
            self.assertEqual(set(manifest["members"]), set(files) - {"BUNDLE.json"})
            saved = path.read_bytes()
            with self.assertRaises(FileExistsError):
                SERVICE.write_archive(path, files)
            self.assertEqual(path.read_bytes(), saved)
            path.unlink()
            self.assertEqual(SERVICE.write_archive(path, files)["sha256"], first["sha256"])

    def test_extra_links_corruption_and_gzip_tail_cannot_pass_readback(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "candidate.tar.gz"
            files = {"command": (b"command", 0o755)}
            SERVICE.write_archive(path, files)
            path.write_bytes(path.read_bytes() + gzip.compress(b""))
            with self.assertRaises(ValueError): SERVICE.verify_archive(path, files)
            path.unlink()
            SERVICE.write_archive(path, files)
            raw = bytearray(path.read_bytes()); raw[-8] ^= 1; path.write_bytes(raw)
            with self.assertRaises(Exception): SERVICE.verify_archive(path, files)
            with tarfile.open(path, "w:gz") as archive:
                item = tarfile.TarInfo("command"); item.type = tarfile.SYMTYPE; item.linkname = "outside"
                archive.addfile(item)
            with self.assertRaises(ValueError): SERVICE.verify_archive(path, files)

    def test_full_zip_h1_crc_roster_and_injected_source_are_checked(self):
        import zipfile
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory); module = root / "module"; module.mkdir()
            content = {**{"a" + str(n): b"asset" for n in range(89)},
                       **{"p" + str(n): b"public" for n in range(16)},
                       "go.mod": b"module synthetic\n", "LICENSE": b"license", "NOTICE.md": b"notice",
                       "LICENSE-APACHE-2.0": b"apache", "upstream/LICENSE": b"apache", "extra": b"extra"}
            generation = {"schema": "gooo/agentmesh-native-generation/v1", "origin": "fresh_pinned_generation", "assets": {
                name: "sha256:" + SERVICE.sha(raw) for name, raw in content.items() if name.startswith("a")},
                "public_sources": {name: "sha256:" + SERVICE.sha(raw) for name, raw in content.items() if name.startswith("p")}}
            content["generation.json"] = SERVICE.encoded(generation)
            prefix = SERVICE.MODULE + "@" + SERVICE.VERSION + "/"
            path = root / "module.zip"
            with zipfile.ZipFile(path, "w") as archive:
                for name, raw in content.items():
                    target = module / name; target.parent.mkdir(parents=True, exist_ok=True); target.write_bytes(raw)
                    if name.startswith("p"):
                        (root / name).write_bytes(raw)
                    archive.writestr(prefix + name, raw)
            current = root / "examples/agentmesh-service-policy"; current.mkdir(parents=True)
            (current / "generation.json").write_bytes(content["generation.json"])
            self.assertEqual(len(content), 112)
            with mock.patch.object(SERVICE, "MODULE_SUM", SERVICE.dirhash({prefix + name: raw for name, raw in content.items()})), \
                 mock.patch.object(SERVICE, "MOD_SUM", SERVICE.dirhash({"go.mod": content["go.mod"]})), \
                 mock.patch.object(SERVICE, "GENERATION", SERVICE.sha(content["generation.json"])):
                self.assertEqual(SERVICE.module_source(module, path, root)[0], generation)
                injected = module / "unsealed.go"; injected.write_bytes(b"package injected")
                with self.assertRaisesRegex(ValueError, "roster"):
                    SERVICE.module_source(module, path, root)
                injected.unlink()
                (module / "a0").write_bytes(b"changed")
                with self.assertRaisesRegex(ValueError, "differs from ZIP"):
                    SERVICE.module_source(module, path, root)

    def test_failed_control_never_publishes_and_cleans_work_preserving_first_failure(self):
        with tempfile.TemporaryDirectory() as directory:
            root, go, generation, source, build = self.fixture(directory)
            binary = root / "binary"; binary.write_bytes(b"fake, never run"); binary.chmod(0o700)
            out = root / "out"; out.mkdir()
            native_build = self.build()
            calls = []
            def fake_run(argv, raw, cwd, env, output, label, deadline):
                calls.append((label, env))
                if label == "build-info": return SERVICE.encoded(native_build), b"", 0, {}
                raise ValueError("original control failure")
            with mock.patch.object(SERVICE, "module_source", return_value=(generation, source)), \
                 mock.patch.object(SERVICE, "native_platform", return_value="darwin/arm64"), \
                 mock.patch.object(SERVICE, "run_owned", side_effect=fake_run):
                with self.assertRaisesRegex(ValueError, "original control failure"):
                    SERVICE.produce(binary, root, root / "zip", go, root, out, "1" * 40)
            self.assertEqual([row[0] for row in calls], ["build-info", "control-1"])
            self.assertTrue(all(row[1]["PATH"] == "" and "GOROOT" not in row[1] for row in calls))
            self.assertFalse((out / "work").exists())
            self.assertFalse(any(path.name.endswith("tar.gz") for path in out.iterdir()))
            self.assertEqual(SERVICE.load((out / "first-failure.json").read_bytes())["error"], "original control failure")

    def test_diagnostic_write_failure_preserves_first_command_failure(self):
        import sys
        import time
        for overflow in (False, True):
            with self.subTest(overflow=overflow), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                original_write = Path.write_bytes
                def fail_stdout(path, raw):
                    if path.name == "tiny.stdout":
                        raise OSError("synthetic diagnostic write failure")
                    return original_write(path, raw)
                script = "import sys; sys.stdout.buffer.write(b'x' * " + str(SERVICE.MAX_STREAM + 1 if overflow else 1) + ")"
                with mock.patch.object(Path, "write_bytes", fail_stdout):
                    with self.assertRaisesRegex(ValueError if overflow else OSError,
                                                "stream exceeds bound" if overflow else "diagnostic write failure") as caught:
                        SERVICE.run_owned([sys.executable, "-I", "-c", script], b"", root,
                                          {"PATH": ""}, root, "tiny", time.monotonic() + 10)
                self.assertTrue((root / "tiny.stderr").exists())
                self.assertTrue((root / "tiny.json").exists())
                if overflow and hasattr(caught.exception, "__notes__"):
                    self.assertIn("synthetic diagnostic write failure", caught.exception.__notes__[0])

    def test_empty_path_native_command_closes_pipes_and_group(self):
        import sys
        import time
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            stdout, stderr, code, record = SERVICE.run_owned(
                [sys.executable, "-I", "-c", "import sys; sys.stdout.buffer.write(sys.stdin.buffer.read())"],
                b"synthetic", root, {"PATH": ""}, root, "tiny", time.monotonic() + 10)
            self.assertEqual((stdout, stderr, code), (b"synthetic", b"", 0))
            self.assertTrue(record["waited"] and record["streams_eof"] and record["process_group_absent"])


if __name__ == "__main__":
    unittest.main()
