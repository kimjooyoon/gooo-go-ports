"""Reference location controls with mocked downloads; no executable is run."""

import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock

SPEC = importlib.util.spec_from_file_location(
    "reference_runtime_under_test", Path(__file__).with_name("service_grant_mcp_reference_runtime.py"))
P = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(P)


class RuntimeControls(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.refs = self.root / "out/service-grant-mcp"
        self.refs.mkdir(parents=True)
        self.context = self.root / "context"
        (self.context / "runtime").mkdir(parents=True)
        self.runner = self.root / "runner-temp"
        self.runner.mkdir()
        self.go = self.root / "fake-go"
        self.go.write_bytes(b"synthetic Go identity; never executed")
        self.server = b"synthetic server; never executed"
        (self.context / "runtime/gooo-service-grant-mcp").write_bytes(self.server)
        self.write(self.context / "context.json", {"archive_sha256": "a" * 64,
                   "source_commit": "1" * 40, "binary_sha256": P.C.digest(self.server)})
        self.prepared = {"schema": "gooo/prepared-saved-native-reference-packet/v1",
            "status": "PINNED_REFERENCE_BYTES_MATERIALIZED_NO_EXECUTION",
            "native_archive_sha256": "a" * 64, "native_source": "1" * 40,
            "original_observation_go_tool": {"bytes": self.go.stat().st_size,
                                             "sha256": P.C.digest(self.go.read_bytes())}, "references": {}}
        self.modules = {}
        for role, dependency in [("alias", P.C.ALIAS_DEPENDENCY), ("snapshot", P.C.SNAPSHOT.DEPENDENCY)]:
            raw = ("synthetic " + role).encode()
            (self.refs / (role + "-compare")).write_bytes(raw)
            self.prepared["references"][role] = {"file": role + "-compare", "module": dependency,
                                                  "bytes": len(raw), "sha256": P.C.digest(raw)}
            directory = self.root / (role + "-module")
            directory.mkdir()
            self.modules[role] = directory
        self.write(self.refs / "prepared-references.json", self.prepared)
        targets = P.destinations(self.runner)
        self.alias = {"binary": {"path": str(targets["server"])},
                      "alias_cli": {"path": str(targets["alias"])},
                      "go_identity_reader": {"path": str(self.go)}}
        self.write(self.refs / "alias-wire/acceptance.json", self.alias)
        records = []
        for label, target in [("server-identity", targets["server"]),
                              ("snapshot-cli-identity", targets["snapshot"])]:
            records.append({"label": label, "record": {"argv": [str(self.go), "version", "-m", "-json", str(target)],
                                                         "cwd": str(self.refs / "snapshot-wire" / label)}})
        records.append({"label": "reference-case", "record": {
            "argv": [str(targets["snapshot"]), "--definition-snapshot"], "cwd": str(self.refs / "snapshot-wire/reference-case")}})
        self.snapshot = {"reference_processes": records}
        self.write(self.refs / "snapshot-wire/acceptance.json", self.snapshot)
        self.download = mock.patch.object(P, "download", side_effect=self.mock_download)
        self.downloader = self.download.start()
        self.addCleanup(self.download.stop)

    def write(self, path, value):
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(value) + "\n")

    def mock_download(self, go, dependency, output, workdir):
        self.assertTrue(Path(workdir).is_dir())
        self.assertEqual(Path(workdir).parent, self.runner)
        role = "alias" if dependency == P.C.ALIAS_DEPENDENCY else "snapshot"
        return self.modules[role]

    def prepare(self):
        return P.prepare(self.refs, self.context, self.go, self.runner)

    def test_original_locations_arguments_and_owned_cleanup(self):
        original_alias = (self.refs / "alias-wire/acceptance.json").read_bytes()
        original_snapshot = (self.refs / "snapshot-wire/acceptance.json").read_bytes()
        state = self.prepare()
        self.assertEqual(state["status"], "ORIGINAL_LOCATIONS_AND_MODULES_READY_NO_REFERENCE_EXECUTION")
        self.assertEqual(self.downloader.call_count, 2)
        self.assertEqual(len(state["arguments"]), 16)
        self.assertFalse(state["reference_cases_or_MCP_executed"])
        for role, path in P.destinations(self.runner).items():
            expected = self.server if role == "server" else (self.refs / (role + "-compare")).read_bytes()
            self.assertEqual(path.read_bytes(), expected)
        self.assertEqual((self.refs / "alias-wire/acceptance.json").read_bytes(), original_alias)
        self.assertEqual((self.refs / "snapshot-wire/acceptance.json").read_bytes(), original_snapshot)
        clean = P.cleanup(self.refs, self.runner)
        self.assertEqual(clean["cleanup"], "OWNED_REFERENCE_FILES_REMOVED")
        self.assertTrue(all(not x.exists() for x in P.destinations(self.runner).values()))
        self.assertTrue(all(x.exists() for x in self.modules.values()))

    def test_wrong_go_identity_refuses_before_download_or_location_writes(self):
        self.go.write_bytes(b"different Go bytes")
        with self.assertRaisesRegex(P.C.Refusal, "Go identity"):
            self.prepare()
        self.downloader.assert_not_called()
        self.assertFalse((self.refs / "runtime-state.json").exists())

    def test_existing_target_or_directory_is_never_overwritten(self):
        path = P.destinations(self.runner)["server"]
        path.write_bytes(b"existing")
        with self.assertRaisesRegex(P.C.Refusal, "overwrite"):
            self.prepare()
        self.assertEqual(path.read_bytes(), b"existing")
        self.downloader.assert_not_called()
        path.unlink()
        P.destinations(self.runner)["alias"].parent.mkdir()
        with self.assertRaisesRegex(P.C.Refusal, "fresh"):
            self.prepare()

    def test_original_cwd_or_argv_mismatch_is_not_silently_relocated(self):
        self.snapshot["reference_processes"][0]["record"]["cwd"] = "/old/unrelated/location"
        self.write(self.refs / "snapshot-wire/acceptance.json", self.snapshot)
        with self.assertRaisesRegex(P.C.Refusal, "cwd"):
            self.prepare()
        self.downloader.assert_not_called()
        self.assertTrue(all(not x.exists() for x in P.destinations(self.runner).values()))

    def test_failed_module_download_records_original_error_and_no_binaries(self):
        error = RuntimeError("first module failure")
        self.downloader.side_effect = error
        with self.assertRaises(RuntimeError) as caught:
            self.prepare()
        self.assertIs(caught.exception, error)
        state = P.load(self.refs / "runtime-state.json")
        self.assertEqual(state["status"], "FIRST_FAILURE_NO_AUTOMATIC_RETRY")
        self.assertEqual(state["first_failure"]["message"], "first module failure")
        self.assertEqual(state["created"], [])
        self.assertTrue(all(not x.exists() for x in P.destinations(self.runner).values()))

    def test_cleanup_refuses_changed_bytes_and_wrong_owned_root(self):
        self.prepare()
        server = P.destinations(self.runner)["server"]
        server.write_bytes(b"changed after preparation")
        with self.assertRaisesRegex(P.C.Refusal, "changed"):
            P.cleanup(self.refs, self.runner)
        self.assertEqual(server.read_bytes(), b"changed after preparation")
        other = self.root / "other-runner"
        other.mkdir()
        with self.assertRaisesRegex(P.C.Refusal, "ownership"):
            P.cleanup(self.refs, other)


if __name__ == "__main__": unittest.main()
