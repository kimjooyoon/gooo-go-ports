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
                "examples/tool-definition-snapshot/reference/main.go": (),
                "examples/tool-definition-snapshot/example-input.json": (),
                "examples/tool-definition-snapshot/provenance_test.py": (),
                "tools/unrelated.py": ()}
        roster = b.source_roster(tree)
        self.assertTrue(b.SOURCE_EXTRAS <= roster)
        self.assertIn(b.PROFILE + "newkernel/new.go", roster)
        self.assertIn("examples/explicit-observation-availability/new.json", roster)
        self.assertNotIn(b.PROFILE + "README.md", roster)
        self.assertIn("examples/tool-definition-snapshot/reference/main.go", roster)
        self.assertIn("examples/tool-definition-snapshot/example-input.json", roster)
        self.assertIn("examples/tool-definition-snapshot/provenance_test.py", roster)
        self.assertIn("definition-snapshot.workspace.json", roster)
        self.assertIn("ports/tool-origin/definition_snapshot.gooo", roster)
        self.assertIn(b.REGRESSION_ROOT + "README.md", roster)
        self.assertIn(b.REGRESSION_ROOT + "LICENSE", roster)
        new_tree = dict(tree)
        new_tree[b.REGRESSION_ROOT + "output-schema-string-to-number/snapshot-input.json"] = ()
        new_tree["internal/runtimepackagesurface/projection_generated.go"] = ()
        new_tree["cmd/check-runtime-package-surface/main.go"] = ()
        expanded = b.source_roster(new_tree)
        self.assertIn(b.REGRESSION_ROOT + "output-schema-string-to-number/snapshot-input.json", expanded)
        self.assertNotIn("internal/runtimepackagesurface/projection_generated.go", expanded)
        self.assertNotIn("cmd/check-runtime-package-surface/main.go", expanded)

    def test_exact_generation_roster_refuses_equal_count_substitution(self):
        b.check_generation_roster(dict.fromkeys(b.GENERATION_MANIFESTS))
        self.assertEqual(len(b.GENERATION_MANIFESTS), 6)
        for missing in b.GENERATION_MANIFESTS:
            changed = set(b.GENERATION_MANIFESTS) - {missing}
            with self.assertRaisesRegex(ValueError, "exact runtime generation"):
                b.check_generation_roster(changed)
            changed.add(b.PROFILE + "unrelated-generation.json")
            with self.assertRaisesRegex(ValueError, "exact runtime generation"):
                b.check_generation_roster(changed)
        with self.assertRaisesRegex(ValueError, "exact runtime generation"):
            b.check_generation_roster(b.GENERATION_MANIFESTS | {b.PROFILE + "unrelated-generation.json"})

    def test_snapshot_materials_require_current_complete_source_bindings(self):
        groups = {
            "source": {"ports/tool-origin/definition_snapshot.gooo"},
            "generator_source": b.SNAPSHOT_GENERATOR_SOURCES,
            "reference_source": b.SNAPSHOT_REFERENCE_SOURCES,
            "generated_assets": b.SNAPSHOT_GENERATED_ASSETS,
        }
        sources = {}
        generation = {}
        for group, names in groups.items():
            prefix = b.SNAPSHOT_ROOT if group == "reference_source" else ""
            generation[group] = {}
            for name in names:
                digest = b.sha((prefix + name).encode())
                sources[prefix + name] = {"sha256": digest}
                generation[group][name] = "sha256:" + digest
        for key, path in (("workspace_sha256", "definition-snapshot.workspace.json"),
                          ("cases_sha256", b.SNAPSHOT_ROOT + "kernel.cases.json")):
            digest = b.sha(path.encode()); sources[path] = {"sha256": digest}
            generation[key] = "sha256:" + digest
        b.check_snapshot_materials(generation, sources)
        for group in groups:
            changed = copy.deepcopy(generation)
            changed[group].pop(next(iter(changed[group])))
            with self.assertRaisesRegex(ValueError, "snapshot material roster"):
                b.check_snapshot_materials(changed, sources)
        for name in sources:
            changed = copy.deepcopy(sources); changed[name]["sha256"] = "0" * 64
            with self.subTest(name=name), self.assertRaisesRegex(ValueError, "snapshot material differs"):
                b.check_snapshot_materials(generation, changed)

    def test_member_refuses_parent_symlink_and_traversal(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp); (root / "real").mkdir(); (root / "real/value").write_bytes(b"ok")
            (root / "alias").symlink_to(root / "real", target_is_directory=True)
            self.assertEqual(b.member(root, "real/value"), b"ok")
            for name in ("alias/value", "../value", "/value", "real//value"):
                with self.assertRaises(ValueError):
                    b.member(root, name)


class SnapshotPackets(unittest.TestCase):
    def packet(self):
        root = Path(__file__).resolve().parents[1]
        names = [p for p in (root / b.REGRESSION_ROOT).rglob("*") if p.is_file()]
        sources = {str(p.relative_to(root)): b.identity(p.read_bytes()) for p in names}
        return root, sources, b.check_snapshot_regressions(root, sources, (root / "LICENSE").read_bytes())

    def test_complete_packet_source_and_byte_bindings(self):
        root, sources, packet = self.packet()
        self.assertEqual(len(packet["files"]), 27)
        self.assertEqual(len(packet["rows"]), 3)
        self.assertEqual(len(b.GENERATION_MANIFESTS), 6)
        for name in sources:
            changed = dict(sources)
            changed.pop(name)
            with self.subTest(name=name), self.assertRaisesRegex(ValueError, "source roster"):
                b.check_snapshot_regressions(root, changed, (root / "LICENSE").read_bytes())
        with self.assertRaisesRegex(ValueError, "license differs"):
            b.check_snapshot_regressions(root, sources, b"wrong license")

    def test_archive_retains_all_packet_bytes_and_labels_historical_results(self):
        root, sources, packet = self.packet()
        rows = [{"label": label, "raw_input_json": "{}", "availability_json": "{}",
                 "before_policy_revision": "p", "after_policy_revision": "p"}
                for label in ("supplied-unknown", "unobserved-unknown")]
        checked = {"rows": rows, "binary": b"synthetic-test-binary", "platform": "linux/amd64", "build": build(),
                   "licenses": {name: (root / path).read_bytes() for name, path in b.LICENSES.items()},
                   "sources": sources, "public_export": b.identity(b"synthetic-test-export"),
                   "compiler_source_sha": "b" * 40,
                   "generations": {name: b.identity(b"synthetic-generation") for name in b.GENERATION_MANIFESTS},
                   "packaging_sources": {"tools/recorded_catalog_cli_bundle.py": b.identity(b"synthetic-packager")},
                   "snapshots": packet}
        files = b.archive_files(checked, "a" * 40, None)
        self.assertEqual(len(files), 36)
        for name, raw in packet["files"].items():
            self.assertEqual(files[b.REGRESSION_ARCHIVE + name], (raw, 0o644))
        manifest = b.load(files["manifest.json"][0])
        self.assertEqual(len(manifest["generation_manifests"]), 6)
        self.assertEqual(manifest["snapshot_regressions"]["original_validation_source_commit"], packet["original_source_commit"])
        self.assertEqual(manifest["source_revision"], "a" * 40)
        self.assertNotIn("verification.json", files)
        guide = files["START-HERE.txt"][0].decode()
        self.assertIn("outside the archive", guide)
        for case in b.REGRESSION_CASES:
            self.assertIn(case, guide)

    def test_packet_corruption_not_masked_by_source_hash_update(self):
        root, _, packet = self.packet()
        for name in ("output-schema-string-to-number/before.request.json",
                     "same-definition-version-context-only/observed-decision.json"):
            with self.subTest(name=name), tempfile.TemporaryDirectory() as tmp:
                copied = Path(tmp)
                sources = {}
                for relative, original in packet["files"].items():
                    raw = original + b" " if relative == name else original
                    path = copied / b.REGRESSION_ROOT / relative
                    path.parent.mkdir(parents=True, exist_ok=True); path.write_bytes(raw)
                    sources[str(path.relative_to(copied))] = b.identity(raw)
                with self.assertRaises(ValueError):
                    b.check_snapshot_regressions(copied, sources, (root / "LICENSE").read_bytes())

    def test_full_decision_original_frames_and_current_generation_are_required(self):
        _, _, packet = self.packet()
        for row in packet["rows"]:
            value = b.load(row["input"])
            projection = {"input_json": row["input"].decode(), "input_sha256": "sha256:" + b.sha(row["input"]),
                          "context_id": row["expected"]["context_id"], "target_name": value["target_name"]}
            for side in ("before", "after"):
                projection[side] = {"pages": [{"page_index": 0, **{
                    kind + "_sha256": "sha256:" + b.sha(value[side]["pages"][0][kind + "_json"].encode())
                    for kind in ("request", "response")}}]}
            generation = {"source": "current-generated-material"}
            report = {"schema": "gooo/definition-snapshot-report/v1", "decision": copy.deepcopy(row["expected"]),
                      "projection": projection, "bindings": generation}
            b.check_snapshot_report(b.encoded(report), row["input"], row, generation)
            changes = [lambda r: r["decision"].pop("before_definition_id"),
                       lambda r: r["decision"].update(execution_authorized=True),
                       lambda r: r["decision"].update(before_index=False),
                       lambda r: r.update(bindings={"source": "historical-generated-material"}),
                       lambda r: r["projection"]["before"]["pages"][0].update(request_sha256="sha256:" + "0" * 64)]
            for mutate in changes:
                bad = copy.deepcopy(report); mutate(bad)
                with self.subTest(case=row["case"]), self.assertRaises((ValueError, KeyError)):
                    b.check_snapshot_report(b.encoded(bad), row["input"], row, generation)


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

    def test_nested_packet_paths_are_exact_and_traversal_is_refused(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            files = {"mcp-upgrade-regressions/case/snapshot-input.json": (b"{}\n", 0o644)}
            archive = root / "safe.tgz"
            b.write_archive(archive, files)
            b.verify_archive(archive, files)
            for i, name in enumerate(("../escape", "/absolute", "a//b", "a/./b", "a/../b", "a\\b")):
                with self.subTest(name=name), self.assertRaisesRegex(ValueError, "safe relative"):
                    b.write_archive(root / (str(i) + ".tgz"), {name: (b"x", 0o644)})

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
