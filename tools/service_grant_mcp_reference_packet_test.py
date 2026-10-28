"""Synthetic archive controls only; the native envelope is explicitly mocked.

These tests establish materialization/refusal behavior, not native execution or
the validity of the fake native bytes. Real packets need the unmocked validator.
"""

import copy
import gzip
import importlib.util
import io
import json
from pathlib import Path
import stat
import tarfile
import tempfile
import unittest
from unittest import mock
import zipfile

SPEC = importlib.util.spec_from_file_location(
    "reference_packet_under_test", Path(__file__).with_name("service_grant_mcp_reference_packet.py"))
P = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(P)


def encoded(value):
    return (json.dumps(value, sort_keys=True) + "\n").encode()


def wire_zip(files):
    output = io.BytesIO()
    with zipfile.ZipFile(output, "w", zipfile.ZIP_DEFLATED) as archive:
        for name, raw in files:
            archive.writestr(name, raw)
    return output.getvalue()


def packet_tar(files, change=None):
    output = io.BytesIO()
    with tarfile.open(fileobj=output, mode="w:gz", format=tarfile.USTAR_FORMAT) as archive:
        for name, raw in sorted(files.items()):
            info = tarfile.TarInfo(name)
            info.mode = 0o755 if name in ("alias-compare", "snapshot-compare") else 0o644
            info.size = len(raw)
            if change:
                change(info)
            archive.addfile(info, io.BytesIO(raw) if info.isfile() else None)
    return output.getvalue()


class PacketControls(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.out = self.root / "prepared"
        self.native_raw = b"synthetic native archive, never executed"
        self.native = self.root / "native.tar.gz"
        self.native.write_bytes(self.native_raw)
        self.native_sha = P.C.digest(self.native_raw)
        self.source = "1" * 40
        self.native_binary = b"synthetic native executable, never executed"
        self.go = {"bytes": 1, "sha256": "f" * 64}
        self.shape = {"type": "object"}
        self.inventory = []
        self.native_files = {P.C.BINARY: self.native_binary}
        for i in range(62):
            name = "NOTICE-synthetic-%02d" % i
            raw = ("synthetic notice %d\n" % i).encode()
            self.native_files[name] = raw
            self.inventory.append({"file": name, "sha256": P.C.digest(raw)})
        self.native_files["NOTICE-INVENTORY.json"] = encoded(self.inventory)
        self.native_manifest = {"schema": P.C.SNAPSHOT.SCHEMA, "source_commit": self.source, "finite_checks": {},
                                "optional_profiles": {"definition_snapshot": {"output_schema": self.shape}}}
        self.files = {name: self.native_files[name] for name in
                      ["NOTICE-INVENTORY.json", *[r["file"] for r in self.inventory]]}
        self.files.update({"alias-compare": b"synthetic alias", "snapshot-compare": b"synthetic snapshot",
                           "START-HERE.txt": b"Synthetic format fixture; no execution.\n"})
        refs, reports = {}, []
        for role, module in [("alias", P.C.ALIAS_DEPENDENCY), ("snapshot", P.C.SNAPSHOT.DEPENDENCY)]:
            name = role + "-compare"
            identity = {"bytes": len(self.files[name]), "sha256": P.C.digest(self.files[name])}
            refs[role] = dict(identity, file=name, module=module, go_version=P.C.GO_VERSION,
                              dependencies=[], recorded_report=role + "-wire/acceptance.json",
                              original_reference_bytes_equal=True)
            report = {"accepted": True, "first_failure": None,
                      "binary": {"sha256": P.C.digest(self.native_binary)}, role + "_cli": identity,
                      "go": self.go, "schemas": {"output": self.shape},
                      "argv": ["/original/observed/server", "--flag"]}
            if role == "alias":
                del report["accepted"]
                report.update(schema="gooo/alias-origin-mcp-wire-verification/v1",
                              status="INTERNAL_ALIAS_MCP_TWO_PROTOCOL_COMPARISON_PASS", scope="full",
                              saved_evidence_validated=True, actual_protocol_sessions=8,
                              all_direct_wait_EOF_PG_ESRCH=True)
            else:
                report.update(schema="gooo/snapshot-MCP-wire-verification/v1",
                              status="INTERNAL_SNAPSHOT_MCP_TWO_PROTOCOLS_ALL_EIGHT_PROFILES_PASS")
            self.native_manifest["finite_checks"].update({role + "_full_report_sha256": P.C.digest(encoded(report)),
                                                        role + "_reference_cli_sha256": identity["sha256"]})
            reports.append((role + "-wire/acceptance.json", encoded(report)))
        self.original_capture = b'{"duplicate":1,"duplicate":2}\n'
        reports.append(("snapshot-wire/case/stdin", self.original_capture))
        self.files["source-wire.zip"] = wire_zip(reports)
        self.doc = {"schema": P.SCHEMA, "platform": "linux/amd64", "native_source": self.source,
                    "original_CI": {"repository": "kimjooyoon/gooo-go-ports", "run": 1,
                                    "attempt": 1, "job": 2, "url": P.REPOSITORY + "/actions/runs/1"},
                    "associated_native_archive": {"name": "gooo-service-grant-mcp-linux-amd64.tar.gz",
                        "bytes": len(self.native_raw), "sha256": self.native_sha,
                        "binary_sha256": P.C.digest(self.native_binary)},
                    "source_wire": {"file": "source-wire.zip", "bytes": len(self.files["source-wire.zip"]),
                        "sha256": P.C.digest(self.files["source-wire.zip"]), "original_provider_artifact": 3,
                        "alias_report": "alias-wire/acceptance.json", "snapshot_report": "snapshot-wire/acceptance.json",
                        "max_expanded_bytes": P.WIRE_EXPANDED_LIMIT},
                    "references": refs, "original_observation_go_tool": self.go, "notice_texts": 62,
                    "@context": {"prov": "http://www.w3.org/ns/prov#"},
                    "prov:wasDerivedFrom": [{"@id": "urn:sha256:" + P.C.digest(self.files["source-wire.zip"])},
                        {"@id": "urn:sha256:" + self.native_sha}, {"@id": P.REPOSITORY + "/tree/" + self.source}]}
        self.doc["members"] = {name: {"bytes": len(raw), "sha256": P.C.digest(raw),
                                       "mode": 0o755 if name.endswith("-compare") else 0o644}
                               for name, raw in self.files.items()}
        self.files["REFERENCE.json"] = encoded(self.doc)
        for name, value in [("unpack_checked", (self.native_files, {}, 0)),
                            ("validate_manifest", self.native_manifest)]:
            patch = mock.patch.object(P.C, name, return_value=value)
            patch.start()
            self.addCleanup(patch.stop)

    def prepare(self, raw=None, **changes):
        raw = raw if raw is not None else packet_tar(self.files)
        path = self.root / "packet.tar.gz"
        path.write_bytes(raw)
        args = dict(packet=path, expected_sha=P.C.digest(raw), expected_bytes=len(raw),
                    native_bundle=self.native, native_sha=self.native_sha, source_commit=self.source, out=self.out)
        args.update(changes)
        return P.prepare(**args)

    def test_preserves_raw_capture_argv_modes_and_records_no_execution(self):
        result = self.prepare()
        self.assertEqual((self.out / "snapshot-wire/case/stdin").read_bytes(), self.original_capture)
        original = json.loads((self.out / "alias-wire/acceptance.json").read_bytes())
        self.assertEqual(original["argv"], ["/original/observed/server", "--flag"])
        self.assertEqual(stat.S_IMODE((self.out / "alias-compare").stat().st_mode), 0o755)
        self.assertEqual(stat.S_IMODE((self.out / "source-wire.zip").stat().st_mode), 0o644)
        self.assertEqual(len(result["captures"]), 3)
        for field in ["source_report_argv_or_outputs_rewritten", "reference_or_native_executed",
                      "producer_authenticated", "image_verified", "new_native_Gooo_receipts"]:
            self.assertIs(result[field], False)
        with self.assertRaises(FileExistsError):
            self.prepare()

    def test_packet_hash_size_type_and_native_hash_refuse_before_output(self):
        for kwargs in [{"expected_sha": "0" * 64}, {"expected_bytes": 1},
                       {"expected_bytes": True}, {"native_sha": "0" * 64}]:
            with self.subTest(kwargs=kwargs), self.assertRaises(P.C.Refusal):
                self.prepare(**kwargs)
            self.assertFalse(self.out.exists())

    def test_source_roster_notice_reference_and_provenance_edits_refused(self):
        original = copy.deepcopy(self.files)
        for kind in ["source", "extra", "missing", "notice", "cli", "module", "provenance", "ci-bool"]:
            self.files = copy.deepcopy(original)
            doc = json.loads(self.files["REFERENCE.json"])
            if kind == "source": doc["native_source"] = "0" * 40
            elif kind == "extra": self.files["unexpected"] = b"extra"
            elif kind == "missing": del self.files["NOTICE-synthetic-00"]
            elif kind == "notice": self.files["NOTICE-synthetic-00"] = b"different"
            elif kind == "cli": doc["references"]["alias"]["sha256"] = "0" * 64
            elif kind == "module": doc["references"]["snapshot"]["module"] = {}
            elif kind == "provenance": doc["prov:wasDerivedFrom"] = []
            else: doc["original_CI"]["run"] = True
            self.files["REFERENCE.json"] = encoded(doc)
            with self.subTest(kind=kind), self.assertRaises(P.C.Refusal):
                self.prepare()
            self.assertFalse(self.out.exists())

    def test_tar_symlink_path_and_executable_notice_refused(self):
        for kind in ["symlink", "path", "mode"]:
            def change(info):
                if info.name == "START-HERE.txt":
                    if kind == "symlink": info.type, info.linkname, info.size = tarfile.SYMTYPE, "outside", 0
                    elif kind == "path": info.name = "../outside"
                    else: info.mode = 0o755
            with self.subTest(kind=kind), self.assertRaises(P.C.Refusal):
                P.unpack_packet(packet_tar(self.files, change))

    def test_tar_oversized_member_rejected_without_allocating_payload(self):
        info = tarfile.TarInfo("oversized")
        info.mode, info.size = 0o644, P.EXPANDED_LIMIT + 1
        raw = gzip.compress(info.tobuf(format=tarfile.USTAR_FORMAT) + b"\0" * 1024)
        with self.assertRaisesRegex(P.C.Refusal, "byte bound"):
            P.unpack_packet(raw)

    def test_zip_paths_duplicates_case_aliases_and_directory_conflicts_refused(self):
        required = [("alias-wire/acceptance.json", b"{}"), ("snapshot-wire/acceptance.json", b"{}")]
        for extras in [[("../outside", b"x")], [("/absolute", b"x")], [("a\\b", b"x")],
                       [("a//b", b"x")], [("a/../b", b"x")],
                       [("A", b"x"), ("a", b"y")], [("A", b"x"), ("a/b", b"y")]]:
            with self.subTest(extras=extras), self.assertRaises(P.C.Refusal):
                P.wire_members(wire_zip(required + extras))

    def test_zip_expansion_and_member_limits_refuse_before_writes(self):
        raw = self.files["source-wire.zip"]
        with mock.patch.object(P, "WIRE_EXPANDED_LIMIT", 1), self.assertRaises(P.C.Refusal):
            P.wire_members(raw)
        with mock.patch.object(P, "WIRE_MEMBER_LIMIT", 1), self.assertRaises(P.C.Refusal):
            P.wire_members(raw)
        self.assertFalse(self.out.exists())


if __name__ == "__main__":
    unittest.main()
