#!/usr/bin/env python3
"""Finite transport refusals; no Go/compiler/network or synthetic native PASS."""

import copy
import hashlib
import importlib.util
import io
from pathlib import Path
import stat
import tempfile
import unittest
import warnings
from unittest.mock import patch
import zipfile


ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location("manual_native_shard", ROOT / "tools/manual-native-shard.py")
M = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(M)


def context():
    return {"repository": M.REPOSITORY, "run_id": "17", "attempt": 2,
            "tested_commit": "a" * 40, "tested_tree": "b" * 40,
            "compiler_revision": M.COMPILER, "go_version": M.GO_VERSION, "workers": 3}


def run():
    return {"id": 17, "run_attempt": 2, "head_sha": "a" * 40,
            "event": "workflow_dispatch", "path": M.WORKFLOW,
            "status": "in_progress", "conclusion": None,
            "repository": {"full_name": M.REPOSITORY, "id": 99},
            "head_repository": {"full_name": M.REPOSITORY, "id": 99}}


def job(name, index=10, finished=True):
    return {"id": index, "name": name, "run_id": 17, "run_attempt": 2,
            "head_sha": "a" * 40, "status": "completed" if finished else "in_progress",
            "conclusion": "success" if finished else None,
            "started_at": "2026-10-18T12:00:00Z", "completed_at": "2026-10-18T12:01:00Z"}


def artifact(suffix="control", index=30):
    return {"id": index, "name": M.artifact_prefix(context()) + suffix,
            "size_in_bytes": 1024, "digest": "sha256:" + "c" * 64,
            "created_at": "2026-10-18T12:00:30Z", "expired": False,
            "workflow_run": {"id": 17, "head_sha": "a" * 40,
                             "repository_id": 99, "head_repository_id": 99}}


def fake_go(root):
    installation = root.resolve() / "selected-go"
    (installation / "bin").mkdir(parents=True)
    (installation / "pkg/tool").mkdir(parents=True)
    for name in ("src/math/bits/make_tables.go", "src/image/draw/draw.go"):
        target = installation / name
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(b"synthetic standard library source")
    executable = installation / "bin/go"
    executable.write_bytes(b"synthetic Go binary; never executed")
    return executable


class Metadata(unittest.TestCase):
    def test_artifact_payload_does_not_consume_metadata_response_cap(self):
        class Response(io.BytesIO):
            status = 200

        with tempfile.TemporaryDirectory() as temporary, patch.dict(M.os.environ, {"GH_TOKEN": "synthetic"}):
            api = M.API(context(), Path(temporary))
            api.calls.append({"artifact_id": 30, "bytes": 32 << 20})
            with patch.object(api.opener, "open", return_value=Response(b'{"artifacts":[],"total_count":0}')):
                self.assertEqual(api.get("actions/runs/17/artifacts", "metadata.json"),
                                 {"artifacts": [], "total_count": 0})
            api.calls.append({"path": "earlier-metadata", "bytes": 8 << 20})
            with patch.object(api.opener, "open", return_value=Response(b'{"total_count":0}')):
                with self.assertRaises(M.Refused):
                    api.get("actions/runs/17/artifacts", "overflow.json")

    def test_current_run_and_each_foreign_identity_refuse(self):
        M.validate_run(run(), context())
        for key, value in (("id", 18), ("run_attempt", 1), ("head_sha", "d" * 40),
                           ("event", "pull_request"), ("path", ".github/workflows/ci.yml"),
                           ("conclusion", "cancelled"), ("id", True)):
            with self.subTest(key=key, value=value):
                value_run = run()
                value_run[key] = value
                with self.assertRaises(M.Refused):
                    M.validate_run(value_run, context())
        value_run = run()
        value_run["head_repository"]["id"] = 100
        with self.assertRaises(M.Refused):
            M.validate_run(value_run, context())

    def test_run_status_conclusion_pairs_are_closed(self):
        for status, conclusion in (("queued", None), ("in_progress", None), ("completed", "success")):
            value = run()
            value.update(status=status, conclusion=conclusion)
            M.validate_run(value, context())
        for status in ("queued", "in_progress", "completed", "waiting", "requested", "failure", "cancelled", None):
            for conclusion in (None, "success", "failure", "cancelled", "skipped", "timed_out", "unknown"):
                if (status, conclusion) in (("queued", None), ("in_progress", None), ("completed", "success")):
                    continue
                value = run()
                value.update(status=status, conclusion=conclusion)
                with self.subTest(status=status, conclusion=conclusion), self.assertRaises(M.Refused):
                    M.validate_run(value, context())
        value = run()
        del value["conclusion"]
        with self.assertRaises(M.Refused):
            M.validate_run(value, context())

    def test_queued_run_requires_current_running_job_and_exact_identity(self):
        queued = run()
        queued["status"] = "queued"
        own = "native-shard-0"
        live = job(own, 11, finished=False)
        jobs = {"total_count": 2, "jobs": [job(M.PLAN_JOB), live]}
        with tempfile.TemporaryDirectory() as temporary, patch.dict(M.os.environ, {"GH_TOKEN": "synthetic"}):
            api = M.API(context(), Path(temporary))
            def check(run_value, jobs_value):
                with patch.object(api, "get", side_effect=[run_value, jobs_value]):
                    return api.run_jobs([M.PLAN_JOB, own], {M.PLAN_JOB})
            self.assertEqual(check(queued, jobs)[1][own], live)
            for status, conclusion in (("queued", None), ("completed", "success"),
                                       ("completed", "failure"), ("completed", "cancelled"),
                                       ("in_progress", "failure"), ("in_progress", "cancelled")):
                bad = copy.deepcopy(jobs)
                bad["jobs"][1].update(status=status, conclusion=conclusion)
                with self.subTest(status=status, conclusion=conclusion), self.assertRaises(M.Refused):
                    check(queued, bad)
            for key, value in (("run_id", 18), ("run_attempt", 1), ("head_sha", "d" * 40)):
                bad = copy.deepcopy(jobs)
                bad["jobs"][1][key] = value
                with self.subTest(job_key=key), self.assertRaises(M.Refused):
                    check(queued, bad)
            with self.assertRaises(M.Refused):
                check(queued, {"total_count": 1, "jobs": [job(M.PLAN_JOB)]})
            for key, value in (("id", 18), ("run_attempt", 1), ("head_sha", "d" * 40),
                               ("event", "pull_request"), ("path", ".github/workflows/ci.yml")):
                foreign = copy.deepcopy(queued)
                foreign[key] = value
                with self.subTest(run_key=key), self.assertRaises(M.Refused):
                    check(foreign, jobs)

    def test_complete_unique_successful_workers_required(self):
        names = [M.PLAN_JOB] + ["native-shard-" + str(i) for i in range(3)]
        items = [job(name, i + 1) for i, name in enumerate(names)]
        M.validate_jobs({"total_count": 4, "jobs": items}, context(), names, set(names))
        mutations = [items[:-1], items + [items[1]], items + [job("native-shard-3", 6)]]
        for outcome in ("failure", "cancelled", "skipped", None):
            value = copy.deepcopy(items)
            value[2]["conclusion"] = outcome
            mutations.append(value)
        for key, value in (("run_id", 18), ("run_attempt", 1), ("head_sha", "d" * 40),
                           ("status", "in_progress"), ("run_attempt", True)):
            item = copy.deepcopy(items)
            item[1][key] = value
            mutations.append(item)
        for value in mutations:
            with self.subTest(value=value):
                with self.assertRaises(M.Refused):
                    M.validate_jobs({"total_count": len(value), "jobs": value}, context(), names, set(names))

    def test_artifact_closed_set_and_concurrent_control_download(self):
        a, worker = artifact(), artifact("shard-0", 31)
        expected = {a["name"]: job(M.PLAN_JOB)}
        M.validate_artifacts({"total_count": 1, "artifacts": [a]}, context(), expected, run())
        M.validate_artifacts({"total_count": 2, "artifacts": [a, worker]}, context(), expected, run(), complete=False)
        with self.assertRaises(M.Refused):
            M.validate_artifacts({"total_count": 2, "artifacts": [a, worker]}, context(), expected, run())
        for values in ([], [a, a], [a, artifact("shard-3", 32)]):
            with self.subTest(values=values):
                with self.assertRaises(M.Refused):
                    M.validate_artifacts({"total_count": len(values), "artifacts": values}, context(), expected, run())

    def test_artifact_ownership_digest_size_and_creation_time(self):
        expected = {artifact()["name"]: job(M.PLAN_JOB)}
        values = []
        for key, value in (("expired", True), ("size_in_bytes", M.MAX_ZIP + 1),
                           ("size_in_bytes", True), ("digest", "PASS"),
                           ("created_at", "2026-10-18T11:59:59Z"),
                           ("created_at", "2026-10-18T12:02:00Z")):
            item = artifact()
            item[key] = value
            values.append(item)
        for key, value in (("id", 18), ("head_sha", "d" * 40), ("repository_id", 100),
                           ("head_repository_id", 100)):
            item = artifact()
            item["workflow_run"][key] = value
            values.append(item)
        for item in values:
            with self.subTest(item=item):
                with self.assertRaises(M.Refused):
                    M.validate_artifacts({"total_count": 1, "artifacts": [item]}, context(), expected, run())

    def test_previous_attempt_artifact_does_not_supply_current_missing_one(self):
        item = artifact()
        expected = {item["name"]: job(M.PLAN_JOB)}
        item["name"] = item["name"].replace("-a2-", "-a1-")
        with self.assertRaises(M.Refused):
            M.validate_artifacts({"total_count": 1, "artifacts": [item]}, context(), expected, run())

    def test_full82_plan_not_smoke_and_exact_typed_context(self):
        value = {"schema": "gooo/native-ci-shard-plan/v1", "run": context(),
                 "cases": 175649, "batches": 2305, "suites": [{"index": i} for i in range(82)],
                 "workspaces": [{} for _ in range(82)], "plan_sha256": "sha256:" + "c" * 64,
                 "shards": [{"index": i, "original_indices": list(range(i, 82, 3))} for i in range(3)]}
        M.validate_plan(value, context())
        for key, replacement in (("cases", 6), ("batches", 6), ("suites", value["suites"][:6]),
                                 ("shards", value["shards"][:2])):
            bad = copy.deepcopy(value)
            bad[key] = replacement
            with self.assertRaises(M.Refused):
                M.validate_plan(bad, context())
        bad = copy.deepcopy(value)
        bad["shards"][0]["original_indices"][0] = 1
        with self.assertRaises(M.Refused):
            M.validate_plan(bad, context())
        bad = copy.deepcopy(value)
        bad["run"]["workers"] = True
        with self.assertRaises(M.Refused):
            M.validate_plan(bad, context())
        bad = copy.deepcopy(value)
        bad["shards"][1]["index"] = True
        with self.assertRaises(M.Refused):
            M.validate_plan(bad, context())

    def test_handoff_binds_job_plan_artifact_compiler_go_and_verifier(self):
        with tempfile.TemporaryDirectory() as temporary:
            plan = Path(temporary) / "plan.json"
            plan.write_bytes(b"original-plan")
            ref = {"bytes": 1, "sha256": "d" * 64}
            control = {"files": {"compiler": ref, "verify": ref}, "go_binary": ref}
            a = artifact()
            j = job("native-shard-0")
            value = {"schema": "gooo/manual-native-shard-handoff/v1", "context": context(),
                     "index": 0, "job_id": j["id"], "job_name": j["name"],
                     "status": "COMMAND_SUCCEEDED", "plan_file": M.file_ref(plan),
                     "control_artifact": {k: a[k] for k in ("id", "digest", "size_in_bytes")},
                     "compiler_binary": ref, "go_binary": ref, "verifier_binary": ref}
            M.handoff_matches(value, context(), plan, control, 0, j, a)
            for key, replacement in (("index", True), ("job_id", 11), ("job_name", "native-shard-1"),
                                     ("status", "FAILED"), ("plan_file", {"bytes": 1, "sha256": "e" * 64}),
                                     ("compiler_binary", {}), ("go_binary", {}),
                                     ("verifier_binary", {}), ("control_artifact", {})):
                bad = copy.deepcopy(value)
                bad[key] = replacement
                with self.subTest(key=key), self.assertRaises(M.Refused):
                    M.handoff_matches(bad, context(), plan, control, 0, j, a)
            bad = copy.deepcopy(value)
            bad["context"]["attempt"] = True
            with self.assertRaises(M.Refused):
                M.handoff_matches(bad, context(), plan, control, 0, j, a)

    def test_all_seven_exhaustive_reports_required_in_fresh_reference(self):
        plan = {"run": context()}
        suites = [{"original_index": i, "exhaustive": []} for i in range(82)]
        for index, kind in M.EXHAUSTIVE:
            suites[index]["exhaustive"] = [{"kind": kind}]
        reference = {"schema": "gooo/native-shard-reference-index/v2",
                     "state": "RAW_REFERENCES_CHECKED_PRODUCER_UNAUTHENTICATED",
                     "raw_receipts_inspected": True, "producer_authenticated": False,
                     "native_acceptance": False, "plan": plan, "suites": suites,
                     "shards": [{}, {}, {}]}
        M.validate_reference(reference, plan)
        for index, _ in M.EXHAUSTIVE:
            bad = copy.deepcopy(reference)
            bad["suites"][index]["exhaustive"] = []
            with self.subTest(index=index), self.assertRaises(M.Refused):
                M.validate_reference(bad, plan)
        bad = copy.deepcopy(reference)
        bad["suites"][35]["exhaustive"].append({"kind": "simple-fold"})
        with self.assertRaises(M.Refused):
            M.validate_reference(bad, plan)


class Files(unittest.TestCase):
    def make_zip(self, members):
        data = io.BytesIO()
        with warnings.catch_warnings(), zipfile.ZipFile(data, "w", zipfile.ZIP_DEFLATED) as archive:
            warnings.simplefilter("ignore", UserWarning)
            for name, value in members:
                archive.writestr(name, value)
        return data.getvalue()

    def test_original_bytes_preserved_and_crc_failure_refused(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            archive = root / "input.zip"
            archive.write_bytes(self.make_zip([("evidence/execution.json", b'{"same":true}\n'),
                                               ("evidence/replay.json", b'{"parent":"original"}\n')]))
            M.extract(archive, root / "out")
            self.assertEqual((root / "out/evidence/execution.json").read_bytes(), b'{"same":true}\n')
            damaged = bytearray(archive.read_bytes())
            damaged[53] ^= 127
            archive.write_bytes(damaged)
            with self.assertRaises((zipfile.BadZipFile, M.Refused, OSError)):
                M.extract(archive, root / "bad")
            stored = io.BytesIO()
            with zipfile.ZipFile(stored, "w", zipfile.ZIP_STORED) as source:
                source.writestr("crc", b"original")
            corrupted = stored.getvalue().replace(b"original", b"modified", 1)
            archive.write_bytes(corrupted)
            with self.assertRaises(M.Refused):
                M.extract(archive, root / "bad-crc")

    def test_traversal_duplicate_symlink_and_size_caps(self):
        link = zipfile.ZipInfo("link")
        link.external_attr = (stat.S_IFLNK | 0o777) << 16
        for members in (([("../escape", b"x")]), ([("same", b"a"), ("same", b"b")]),
                        ([(link, b"outside")]), ([("/absolute", b"x")])):
            with self.subTest(members=members), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                archive = root / "input.zip"
                archive.write_bytes(self.make_zip(members))
                with self.assertRaises(M.Refused):
                    M.extract(archive, root / "out")
        with tempfile.TemporaryDirectory() as temporary, patch.object(M, "MAX_FILE", 1):
            root = Path(temporary)
            archive = root / "input.zip"
            archive.write_bytes(self.make_zip([("large", b"xx")]))
            with self.assertRaises(M.Refused):
                M.extract(archive, root / "out")

    def test_digest_is_content_and_symlink_refuses(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            path = root / "file"
            path.write_bytes(b"original")
            expected = {"bytes": 8, "sha256": hashlib.sha256(b"original").hexdigest()}
            self.assertEqual(M.file_ref(path), expected)
            self.assertEqual(M.file_ref(path), expected)
            link = root / "link"
            link.symlink_to(path)
            with self.assertRaises(M.Refused):
                M.file_ref(link)

    def test_duplicate_JSON_and_boolean_integer_confusion_refuse(self):
        with self.assertRaises(M.Refused):
            M.decode(b'{"attempt":1,"attempt":2}')
        self.assertFalse(M.exact({"workers": True}, {"workers": 1}))

    def test_child_environment_has_no_workflow_credentials(self):
        with tempfile.TemporaryDirectory() as temporary:
            go = fake_go(Path(temporary))
            with patch.dict(M.os.environ, {"GH_TOKEN": "synthetic", "GITHUB_TOKEN": "synthetic",
                                          "ACTIONS_RUNTIME_TOKEN": "synthetic", "GOOO_LAYA_API_KEY": "synthetic",
                                          "GOROOT": "/wrong/ambient/root", "PATH": "/ambient/bin"}):
                child = M.child_environment(go)
        self.assertTrue(all(k not in child for k in ("GH_TOKEN", "GITHUB_TOKEN", "ACTIONS_RUNTIME_TOKEN", "GOOO_LAYA_API_KEY")))
        self.assertEqual(child["GOMAXPROCS"], "2")
        self.assertEqual(child["GOROOT"], str(go.parent.parent))
        self.assertEqual(child["PATH"].split(M.os.pathsep)[0], str(go.parent))

    def test_selected_go_layout_rejects_missing_or_foreign_root(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            go = fake_go(root)
            self.assertEqual(M.go_installation_root(go), go.parent.parent)
            with self.assertRaises(M.Refused):
                M.go_installation_root(Path("relative/bin/go"))
            link = root / "linked-go"
            link.symlink_to(go)
            with self.assertRaises(M.Refused):
                M.go_installation_root(link)
            (go.parent.parent / "src/math/bits/make_tables.go").unlink()
            with self.assertRaises(M.Refused):
                M.go_installation_root(go)

    def test_owned_child_uses_selected_root_without_ambient_credentials(self):
        class Child:
            returncode = 0
            def wait(self, timeout=None):
                return 0
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            go = fake_go(root)
            with patch.dict(M.os.environ, {"GOROOT": "/wrong/ambient/root", "GH_TOKEN": "synthetic"}):
                runner = M.Commands(root, root, go)
                with patch.object(M.subprocess, "Popen", return_value=Child()) as start:
                    runner.run("environment", ["synthetic-never-executed"], 10)
            environment = start.call_args.kwargs["env"]
            self.assertEqual(environment["GOROOT"], str(go.parent.parent))
            self.assertNotIn("GH_TOKEN", environment)
            self.assertEqual(runner.records[0]["goroot"], str(go.parent.parent))
            self.assertTrue(runner.records[0]["direct_child_waited"])

    def test_bad_sdk_main_path_preserves_first_failure_and_observations(self):
        for missing in ("src/math/bits/make_tables.go", "pkg/tool"):
            with self.subTest(missing=missing), tempfile.TemporaryDirectory() as temporary:
                base = Path(temporary).resolve()
                checkout = base / "checkout"
                checkout.mkdir()
                out = base / "new-output"
                go = fake_go(base)
                path = go.parent.parent / missing
                path.rmdir() if path.is_dir() else path.unlink()
                argv = ["manual-native-shard.py", "plan", "--root", str(checkout),
                        "--out", str(out), "--go", str(go)]
                before_handlers = {n: M.signal.getsignal(n) for n in (M.signal.SIGTERM, M.signal.SIGINT)}
                with patch("sys.argv", argv), patch.dict(M.os.environ, {"RUNNER_TEMP": str(base)}), \
                     patch.object(M, "current_context", return_value=context()), \
                     patch.object(M, "DEADLINE", None), patch.object(M, "API") as api, \
                     patch.object(M.subprocess, "Popen") as producer:
                    with self.assertRaises(M.Refused):
                        M.main()
                api.assert_not_called()
                producer.assert_not_called()
                failure = M.read_json(out / "first-failure.json")
                observed = M.read_json(out / "observations.json")
                self.assertEqual(failure["type"], "Refused")
                self.assertEqual(failure["context"], context())
                self.assertFalse(failure["native_acceptance"])
                self.assertEqual(observed["commands"], [])
                self.assertEqual(observed["transfers"], [])
                self.assertEqual(observed["selected_go"], str(go))
                self.assertIsNone(observed["selected_goroot"])
                self.assertFalse(observed["selected_goroot_validated"])
                self.assertEqual(observed["sampled_owned_byte_limit"], 3 << 30)
                self.assertFalse(observed["native_acceptance"])
                self.assertEqual(before_handlers, {n: M.signal.getsignal(n) for n in before_handlers})

    def test_signal_interrupt_waits_for_owned_group_and_retains_command(self):
        class Child:
            pid = 123456
            returncode = -9

            def __init__(self):
                self.calls = 0

            def wait(self, timeout=None):
                self.calls += 1
                if self.calls == 1:
                    M.interrupted(M.signal.SIGTERM, None)
                return -9

        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            child = Child()
            runner = M.Commands(root, root, fake_go(root))
            with patch.object(M.subprocess, "Popen", return_value=child), patch.object(M.os, "killpg") as kill:
                with self.assertRaises(InterruptedError):
                    runner.run("cancelled", ["synthetic-never-executed"], 10)
            kill.assert_called_once_with(child.pid, M.signal.SIGKILL)
            self.assertEqual(child.calls, 2)
            self.assertTrue(runner.records[0]["direct_child_waited"])
            self.assertEqual(runner.records[0]["state"], "INTERRUPTED_OR_BOUNDED_FAILURE")

    def test_phase_budget_and_sampled_owned_budget_are_fail_closed(self):
        with patch.object(M, "DEADLINE", 0):
            with self.assertRaises(M.Refused):
                M.available(1)
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / "original").write_bytes(b"123")
            self.assertEqual(M.owned_size(root), 3)
            (root / "link").symlink_to(root / "original")
            with self.assertRaises(M.Refused):
                M.owned_size(root)


class Workflow(unittest.TestCase):
    def test_manual_only_parallel_three_failure_join_and_pins(self):
        source = (ROOT / M.WORKFLOW).read_text()
        self.assertIn("  workflow_dispatch:\n", source)
        self.assertNotIn("  pull_request:", source)
        self.assertNotIn("  push:", source)
        self.assertIn("      fail-fast: false\n      max-parallel: 3\n", source)
        self.assertIn("        index: [0, 1, 2]", source)
        self.assertIn("    if: always()\n    needs: [plan, native-shards]", source)
        self.assertNotIn("continue-on-error", source)
        for action in ("actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1",
                       "actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e",
                       "actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9"):
            self.assertEqual(source.count(action), 3)
        self.assertEqual(source.count("actions: read"), 3)
        self.assertNotIn("actions: write", source)
        self.assertEqual(source.count("retention-days: 1"), 3)
        self.assertEqual(source.count("include-hidden-files: true"), 3)


if __name__ == "__main__":
    unittest.main()
