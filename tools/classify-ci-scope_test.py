#!/usr/bin/env python3
"""Exercise committed Git trees and the real CLI, without a compiler or network."""

import importlib.util
import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock


SCRIPT = Path(__file__).with_name("classify-ci-scope.py")
spec = importlib.util.spec_from_file_location("ci_scope", SCRIPT)
scope = importlib.util.module_from_spec(spec)
spec.loader.exec_module(scope)


AGENT_DOCUMENTS = (
    ".agents/skills/gooo-first-program/SKILL.md",
    ".agents/skills/gooo-first-program/agents/openai.yaml",
    "docs/ai-agents.md", "context7.json",
)
OTHER_DOCUMENTS = (
    ".agents/skills/another/SKILL.md",
    ".agents/skills/gooo-first-program/agents/openai.yml",
    ".agents/skills/gooo-first-program/references/README.md",
    "docs/ai-agents.md.bak", "context7.local.json",
)


class GitScopeTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.repo = self.root / "repo"
        self.repo.mkdir()
        self.env = dict(os.environ, GIT_CONFIG_GLOBAL=os.devnull,
                        GIT_CONFIG_NOSYSTEM="1", GIT_AUTHOR_NAME="Scope fixture",
                        GIT_AUTHOR_EMAIL="fixture@example.invalid",
                        GIT_COMMITTER_NAME="Scope fixture",
                        GIT_COMMITTER_EMAIL="fixture@example.invalid")
        self.git("init", "-q", "--object-format=sha1")
        for name in ("README.md", "CONTRIBUTING.md", "ports/example.gooo",
                     "examples/sample/README.md", "third_party/google-uuid/README.md",
                     "AGENTS.md", "tools/check.py", ".github/workflows/ci.yml",
                     *AGENT_DOCUMENTS, *OTHER_DOCUMENTS):
            path = self.repo / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text("original\n")
        self.git("add", ".")
        self.base = self.commit("base")

    def git(self, *args):
        return subprocess.check_output(["git", "-C", str(self.repo), *args], env=self.env).decode().strip()

    def commit(self, message):
        self.git("commit", "-q", "-m", message)
        return self.git("rev-parse", "HEAD")

    def merge(self, changed=None):
        for name in changed or []:
            with (self.repo / name).open("a") as stream:
                stream.write("changed\n")
        self.git("add", "-A")
        self.git("commit", "-q", "--allow-empty", "-m", "PR head")
        self.pr_head = self.git("rev-parse", "HEAD")
        tree = self.git("rev-parse", "HEAD^{tree}")
        self.tested = self.git("commit-tree", tree, "-p", self.base, "-p", self.pr_head, "-m", "tested merge")
        self.git("checkout", "-q", "--detach", self.tested)
        self.event = {"pull_request": {"base": {"sha": self.base}, "head": {"sha": self.pr_head}}}

    def result(self, event_name="pull_request", expected=None):
        return scope.classify(self.repo, event_name, self.event, expected or self.tested)

    def test_each_allowed_document_and_mixed_delta(self):
        cases = (["README.md"], ["CONTRIBUTING.md"],
                 *([name] for name in AGENT_DOCUMENTS),
                 ["README.md", "CONTRIBUTING.md"], list(AGENT_DOCUMENTS),
                 ["README.md", "CONTRIBUTING.md", *AGENT_DOCUMENTS])
        for paths in cases:
            with self.subTest(paths=paths):
                self.git("checkout", "-q", "--detach", self.base)
                self.merge(paths)
                result = self.result()
                self.assertTrue(result["docs_only"])
                self.assertEqual(result["status"], "DOCS_ONLY_CONTENT_CHANGE")
                self.assertEqual(result["reason"], "exact_static_document_content_delta")
                self.assertEqual({c["path"] for c in result["changes"]}, set(paths))
                self.assertEqual(result["checkout_parents"], [self.base, self.pr_head])

    def test_mixed_code_and_policy_changes_require_full(self):
        for name in ("ports/example.gooo", "AGENTS.md", "tools/check.py", ".github/workflows/ci.yml",
                     "examples/sample/README.md", "third_party/google-uuid/README.md", *OTHER_DOCUMENTS):
            with self.subTest(path=name):
                self.git("checkout", "-q", "--detach", self.base)
                self.merge(["README.md", *AGENT_DOCUMENTS, name])
                self.assertFalse(self.result()["docs_only"])

    def test_static_agent_paths_still_require_existing_regular_content_only(self):
        baseline = self.base
        for name in (AGENT_DOCUMENTS[0], "context7.json"):
            for mutation in ("add", "delete", "executable", "symlink"):
                with self.subTest(path=name, mutation=mutation):
                    self.git("checkout", "-q", "--detach", baseline)
                    self.base = baseline
                    path = self.repo / name
                    if mutation == "add":
                        self.git("rm", "-q", name)
                        self.base = self.commit("base without agent document")
                        path.parent.mkdir(parents=True, exist_ok=True)
                        path.write_text("new document\n")
                    elif mutation == "delete":
                        path.unlink()
                    elif mutation == "symlink":
                        path.unlink()
                        path.symlink_to("ports/example.gooo")
                    else:
                        path.chmod(0o755)
                        with path.open("a") as stream:
                            stream.write("changed\n")
                    self.merge()
                    result = self.result()
                    self.assertFalse(result["docs_only"])
                    self.assertEqual(result["status"], "FULL_REQUIRED")
        self.base = baseline

    def test_rename_between_allowed_agent_paths_requires_full(self):
        self.git("rm", "-q", "context7.json")
        self.base = self.commit("base without allowed rename target")
        self.git("mv", AGENT_DOCUMENTS[0], "context7.json")
        self.merge()
        result = self.result()
        self.assertFalse(result["docs_only"])
        self.assertEqual(result["status"], "FULL_REQUIRED")
        self.assertEqual(result["reason"], "not_existing_regular_document_content_only")
        self.assertEqual({c["status"] for c in result["changes"]}, {"A", "D"})
        self.assertTrue(all(c["allowed_path"] for c in result["changes"]))

    def test_added_document_requires_full(self):
        (self.repo / "new.md").write_text("new\n")
        self.merge(["README.md"])
        self.assertFalse(self.result()["docs_only"])

    def test_previously_absent_allowed_document_requires_full(self):
        self.git("rm", "-q", "CONTRIBUTING.md")
        self.base = self.commit("base without contributing")
        (self.repo / "CONTRIBUTING.md").write_text("added\n")
        self.merge()
        self.assertFalse(self.result()["docs_only"])

    def test_deleted_document_requires_full(self):
        (self.repo / "README.md").unlink()
        self.merge()
        self.assertFalse(self.result()["docs_only"])

    def test_rename_is_not_hidden_by_heuristics(self):
        self.git("mv", "README.md", "renamed.md")
        self.merge()
        result = self.result()
        self.assertFalse(result["docs_only"])
        self.assertEqual({c["status"] for c in result["changes"]}, {"A", "D"})

    def test_executable_mode_change_requires_full(self):
        (self.repo / "README.md").chmod(0o755)
        self.merge(["README.md"])
        self.assertFalse(self.result()["docs_only"])

    def test_symlink_change_requires_full(self):
        path = self.repo / "README.md"
        path.unlink()
        path.symlink_to("ports/example.gooo")
        self.merge()
        self.assertFalse(self.result()["docs_only"])

    def test_existing_executable_document_requires_full(self):
        (self.repo / "README.md").chmod(0o755)
        self.git("add", "README.md")
        self.base = self.commit("executable base document")
        self.merge(["README.md"])
        self.assertFalse(self.result()["docs_only"])

    def test_empty_delta_requires_full(self):
        self.merge()
        self.assertEqual(self.result()["reason"], "empty_tree_delta")

    def test_push_tag_and_other_events_always_full(self):
        self.merge(["README.md"])
        for ref in ("refs/heads/main", "refs/tags/v0.1.0"):
            with self.subTest(ref=ref):
                event = dict(self.event, ref=ref)
                self.assertFalse(scope.classify(self.repo, "push", event, self.tested)["docs_only"])
        for event in ("push", "workflow_dispatch", "pull_request_target", "schedule"):
            with self.subTest(event=event):
                self.assertFalse(scope.classify(self.repo, event, {}, self.tested)["docs_only"])

    def test_head_and_event_identity_errors_fail(self):
        self.merge(["README.md"])
        with self.assertRaises(scope.ScopeError):
            self.result(expected=self.pr_head)
        for value in ("main", "0" * 40, "A" * 40, None):
            with self.subTest(base=value):
                self.event["pull_request"]["base"]["sha"] = value
                with self.assertRaises(scope.ScopeError):
                    self.result()

    def test_missing_event_and_invalid_expected_sha_fail(self):
        self.merge(["README.md"])
        with self.assertRaises(scope.ScopeError):
            scope.classify(self.repo, "pull_request", {}, self.tested)
        with self.assertRaises(scope.ScopeError):
            self.result(expected="HEAD")
        self.event["pull_request"]["head"]["sha"] = "not-a-sha"
        with self.assertRaises(scope.ScopeError):
            self.result()

    def test_wrong_merge_parent_order_requires_full(self):
        self.merge(["README.md"])
        tree = self.git("rev-parse", self.tested + "^{tree}")
        self.tested = self.git("commit-tree", tree, "-p", self.pr_head, "-p", self.base, "-m", "wrong parent order")
        self.git("checkout", "-q", "--detach", self.tested)
        self.assertFalse(self.result()["docs_only"])

    def test_direct_head_or_wrong_parent_identity_requires_full(self):
        self.merge(["README.md"])
        self.git("checkout", "-q", "--detach", self.pr_head)
        self.assertFalse(scope.classify(self.repo, "pull_request", self.event, self.pr_head)["docs_only"])
        self.git("checkout", "-q", "--detach", self.tested)
        self.event["pull_request"]["head"]["sha"] = self.base
        self.assertFalse(self.result()["docs_only"])

    def test_actual_merge_tree_not_just_pr_head_is_checked(self):
        self.merge(["README.md"])
        (self.repo / "ports/example.gooo").write_text("merge-only code change\n")
        self.git("add", ".")
        tree = self.git("write-tree")
        self.tested = self.git("commit-tree", tree, "-p", self.base, "-p", self.pr_head, "-m", "merge resolution")
        self.git("checkout", "-q", "--detach", self.tested)
        self.assertFalse(self.result()["docs_only"])

    def test_dirty_tracked_or_untracked_checkout_fails(self):
        self.merge(["README.md"])
        (self.repo / "ports/example.gooo").write_text("uncommitted\n")
        with self.assertRaises(scope.ScopeError):
            self.result()
        self.git("checkout", "--", "ports/example.gooo")
        (self.repo / "unexpected.go").write_text("untracked\n")
        with self.assertRaises(scope.ScopeError):
            self.result()

    def test_unusual_path_is_not_split_into_allowed_names(self):
        (self.repo / "README.md\nCONTRIBUTING.md").write_text("not allowed\n")
        self.merge(["README.md"])
        self.assertFalse(self.result()["docs_only"])

    def test_cli_writes_boolean_output_and_failure_summary(self):
        self.merge(["README.md"])
        event = self.root / "event.json"
        event.write_text(json.dumps(self.event))
        summary, output = self.root / "scope.json", self.root / "outputs"
        command = [sys.executable, "-B", str(SCRIPT), "--repo", str(self.repo),
                   "--event-name", "pull_request", "--event-path", str(event),
                   "--expected-head", self.tested, "--summary", str(summary),
                   "--github-output", str(output)]
        completed = subprocess.run(command, capture_output=True)
        self.assertEqual(completed.returncode, 0, completed.stderr)
        self.assertEqual(output.read_text(), "docs_only=true\n")
        self.assertTrue(json.loads(summary.read_text())["docs_only"])
        event.write_text("not JSON")
        completed = subprocess.run(command, capture_output=True)
        self.assertNotEqual(completed.returncode, 0)
        self.assertEqual(output.read_text(), "docs_only=true\ndocs_only=false\n")
        self.assertEqual(json.loads(summary.read_text())["status"], "CLASSIFICATION_FAILED")


class RawDiffTests(unittest.TestCase):
    def test_malformed_raw_diff_and_command_failure_fail(self):
        for raw in (b"bad", b"bad\0README.md\0", b"\0README.md\0", b"bad\0README.md\0tail"):
            with self.subTest(raw=raw), self.assertRaises(scope.ScopeError):
                scope.parse_raw_diff(raw)
        with mock.patch.object(scope.subprocess, "run", side_effect=OSError("unavailable")):
            with self.assertRaises(scope.ScopeError):
                scope.git(Path("."), "status")
        with mock.patch.object(scope.subprocess, "run", return_value=mock.Mock(returncode=1)):
            with self.assertRaises(scope.ScopeError):
                scope.git(Path("."), "status")

    def test_duplicate_raw_path_is_rejected(self):
        entry = b":100644 100644 " + b"1" * 40 + b" " + b"2" * 40 + b" M\0README.md\0"
        with self.assertRaises(scope.ScopeError):
            scope.parse_raw_diff(entry + entry)


if __name__ == "__main__":
    unittest.main()
