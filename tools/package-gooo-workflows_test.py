#!/usr/bin/env python3
# Copyright 2026 Gooo Go Ports contributors. BSD-style terms: ../LICENSE.
"""Exercise the packaging CLI with immutable source commits in temporary Git repos."""
import json
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]
TEMPLATE = ROOT / "plugins/gooo-workflows"
HELPER = ROOT / "tools/package-gooo-workflows.py"


class PackageWorkflowsTest(unittest.TestCase):
    def setUp(self):
        workspace = tempfile.TemporaryDirectory(prefix="gooo-package-test-")
        self.addCleanup(workspace.cleanup)
        self.workspace = Path(workspace.name)
        self.repo = self.workspace / "repo"
        self.repo.mkdir()
        self.package = self.repo / "plugins/gooo-workflows"
        shutil.copytree(TEMPLATE, self.package)
        self.original = {
            path.relative_to(self.package).as_posix(): path.read_bytes()
            for path in self.package.rglob("*") if path.is_file()
        }
        provenance = json.loads(self.original["SOURCE.json"])
        self.base_order = [record["source_path"] for record in
                           provenance["skills_copied_without_content_changes"]]
        for record in provenance["skills_copied_without_content_changes"]:
            target = self.repo / record["source_path"]
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(self.original[record["package_path"]])
        (self.repo / "LICENSE").write_bytes(self.original["LICENSE"])
        (self.repo / "tools").mkdir()
        shutil.copyfile(HELPER, self.repo / "tools/package-gooo-workflows.py")
        self.git("init", "--quiet", "--template=")
        for key, value in (("user.name", "Fixture"),
                           ("user.email", "fixture@example.invalid"),
                           ("commit.gpgsign", "false"),
                           ("core.autocrlf", "false"),
                           ("core.hooksPath", str(self.workspace / "no-hooks"))):
            self.git("config", "--local", key, value)
        self.git("add", "--", "LICENSE", ".agents")
        self.git("commit", "--quiet", "-m", "Fixture source")
        provenance["source_commit"] = self.git("rev-parse", "HEAD").strip()
        (self.package / "SOURCE.json").write_text(
            json.dumps(provenance, indent=2) + "\n", encoding="utf-8")
        self.check()

    def git(self, *args):
        return subprocess.check_output(
            ["git", "-C", str(self.repo), *args], stderr=subprocess.PIPE,
            text=True)

    def invoke(self, *args):
        result = subprocess.run(
            [sys.executable, "-I", "-B", str(self.repo / "tools/package-gooo-workflows.py"), *args],
            cwd=self.repo, capture_output=True, text=True, timeout=30)
        self.assertEqual(result.returncode, 0, result.stderr)
        return json.loads(result.stdout)

    def check(self):
        return self.invoke("--check")

    def export(self, name, ref=None):
        output = self.workspace / name
        args = ["--output", str(output)]
        if ref is not None:
            args += ["--source-ref", ref, "--version", "0.1.1" if name == "with-refs" else "0.1.2"]
        self.invoke(*args)
        return output

    def promote_and_check(self, output, count):
        shutil.rmtree(self.package)
        shutil.copytree(output, self.package)
        summary = self.check()
        self.assertEqual(summary["files"], count)
        records = json.loads((self.package / "SOURCE.json").read_text())["skills_copied_without_content_changes"]
        self.assertEqual([r["source_path"] for r in records
                          if r["source_path"] in self.base_order], self.base_order)

    def commit_sources(self):
        self.git("add", "--all", "--", ".agents")
        self.git("commit", "--quiet", "-m", "Change fixture references")
        return self.git("rev-parse", "HEAD").strip()

    def add_references(self):
        self.references = {
            "references/alpha.md": b"# Alpha\nRetain this exact source.\n",
            "references/nested/beta.md": b"# Beta\nA nested Markdown resource.\n",
        }
        self.skill = self.repo / ".agents/skills/gooo-first-program"
        for name, raw in self.references.items():
            target = self.skill / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(raw)
        output = self.export("with-refs", self.commit_sources())
        for name, raw in self.references.items():
            self.assertEqual((output / "skills/gooo-first-program" / name).read_bytes(), raw)
        self.promote_and_check(output, 12)

    def test_legacy_export_preserves_ten_files(self):
        before = {path.relative_to(self.package).as_posix(): path.read_bytes()
                  for path in self.package.rglob("*") if path.is_file()}
        output = self.export("legacy")
        after = {path.relative_to(output).as_posix(): path.read_bytes()
                 for path in output.rglob("*") if path.is_file()}
        self.assertEqual(len(after), 10)
        self.assertEqual(after, before)
        self.promote_and_check(output, 10)

    def test_removed_references_leave_no_stale_files(self):
        self.add_references()
        for name in self.references:
            (self.skill / name).unlink()
        output = self.export("removed", self.commit_sources())
        self.promote_and_check(output, 10)
        for name in self.references:
            self.assertFalse((output / "skills/gooo-first-program" / name).exists())

    def test_renamed_reference_replaces_old_path(self):
        self.add_references()
        old = "references/alpha.md"
        new = "references/renamed.md"
        (self.skill / old).rename(self.skill / new)
        output = self.export("renamed", self.commit_sources())
        self.promote_and_check(output, 12)
        self.assertFalse((output / "skills/gooo-first-program" / old).exists())
        self.assertEqual((output / "skills/gooo-first-program" / new).read_bytes(), self.references[old])
        self.assertEqual((output / "skills/gooo-first-program/references/nested/beta.md").read_bytes(),
                         self.references["references/nested/beta.md"])


if __name__ == "__main__":
    unittest.main()
