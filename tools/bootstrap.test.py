#!/usr/bin/env python3
"""Exercise installer failures with real local Git and a controlled build tool.

Only this temporary copy's repository URL and commit pin are changed. The build
stub injects failures; the shipped Go identity validator runs with real Go.
No public network, engine checkout, or existing user installation is modified.
"""
import hashlib
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import tempfile
import time
import unittest

ROOT = Path(__file__).resolve().parents[1]
REAL_GIT = shutil.which("git")
REAL_GO = str(Path(subprocess.check_output(["go", "env", "GOROOT"], cwd=ROOT, text=True).strip()) / "bin" / "go")
PIN = json.loads((ROOT / "upstream.json").read_text())


def git(*args, cwd=None):
    return subprocess.check_output([REAL_GIT, *args], cwd=cwd, text=True, stderr=subprocess.DEVNULL).strip()


def snapshot(path):
    """All names, kinds, modes and bytes, including Git metadata and symlinks."""
    if not path.exists() and not path.is_symlink():
        return None
    paths = [path] + sorted(path.rglob("*")) if path.is_dir() and not path.is_symlink() else [path]
    result = []
    for item in paths:
        relative = str(item.relative_to(path))
        if item.is_symlink():
            result.append((relative, "link", os.readlink(item)))
        elif item.is_dir():
            result.append((relative, "directory"))
        else:
            result.append((relative, "file", item.stat().st_mode, hashlib.sha256(item.read_bytes()).hexdigest()))
    return result


class Bootstrap(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="gooo bootstrap test ")
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name)
        self.repo, self.app, self.state = [self.base / name for name in ("fixture engine", "ports project", "state")]
        for path in (self.repo, self.app / "tools", self.state, self.base / "bin"):
            path.mkdir(parents=True)
        git("init", str(self.repo))
        git("config", "user.name", "Bootstrap fixture", cwd=self.repo)
        git("config", "user.email", "fixture@example.invalid", cwd=self.repo)
        (self.repo / "source.txt").write_text("immutable fixture source\n")
        (self.repo / ".gitignore").write_text("ignored.txt\n")
        git("add", ".", cwd=self.repo)
        git("commit", "-m", "fixture", cwd=self.repo)
        self.revision = git("rev-parse", "HEAD", cwd=self.repo)
        self.origin = self.repo.as_uri()
        script = (ROOT / "tools/bootstrap.sh").read_text().replace(PIN["compiler_revision"], self.revision).replace("https://github.com/kimjooyoon/meta-ontology-go.git", self.origin)
        (self.app / "tools/bootstrap.sh").write_text(script)
        shutil.copyfile(ROOT / "tools/bootstrap-check.go", self.app / "tools/bootstrap-check.go")
        pin = dict(PIN, compiler_revision=self.revision)
        (self.app / "upstream.json").write_text(json.dumps(pin))
        self.env = dict(os.environ, PATH=str(self.base / "bin") + os.pathsep + os.environ["PATH"],
                        REAL_GIT=REAL_GIT, REAL_GO=REAL_GO, STATE_DIR=str(self.state),
                        TEST_REVISION=self.revision, TEST_COMPILER_VERSION=PIN["compiler_version"], GOTOOLCHAIN="local", GOWORK="off")
        self.executable("git", '''import os,sys
from pathlib import Path
args=sys.argv[1:]
if os.environ.get('FAIL_READ') in args: sys.exit(73)
for operation in ('fetch','checkout'):
 if operation in args:
  with open(Path(os.environ['STATE_DIR'])/'git.log','a') as log: log.write(operation+'\\n')
  if os.environ.get('FAIL_GIT')==operation: sys.exit(71)
os.execv(os.environ['REAL_GIT'],[os.environ['REAL_GIT'],*args])
''')
        self.executable("go", '''import json,os,sys,time
from pathlib import Path
args=sys.argv[1:]; state=Path(os.environ['STATE_DIR'])
if args==['env','GOVERSION']:
 print(os.environ.get('TEST_GOVERSION','go1.27.3')); sys.exit(0)
if args and args[0]=='build':
 with open(state/'build.log','a') as log: log.write('build\\n')
 if os.environ.get('WAIT_BUILD'):
  (state/'entered').write_text('ready')
  while not (state/'release').exists(): time.sleep(.01)
 output=Path(args[args.index('-o')+1])
 if os.environ.get('FAIL_BUILD'):
  output.write_text('partial failed build'); sys.exit(72)
 identity={'compiler_source_sha':os.environ['TEST_REVISION'],'version':os.environ['TEST_COMPILER_VERSION'],'go_version':'go1.27.3','native_go_required':'go1.27.3','source_status':'CLEAN_VCS','vcs_modified':'false'}
 if os.environ.get('BAD_IDENTITY'): identity[os.environ['BAD_IDENTITY']]='wrong'
 output.write_text("#!/bin/sh\\ncat <<'IDENTITY'\\n"+json.dumps(identity)+"\\nIDENTITY\\n")
 output.chmod(0o755); sys.exit(0)
os.execv(os.environ['REAL_GO'],[os.environ['REAL_GO'],*args])
''')

    def executable(self, name, text):
        path = self.base / "bin" / name
        path.write_text("#!" + sys.executable + "\n" + text)
        path.chmod(0o755)

    def run_bootstrap(self, **env):
        return subprocess.run(["sh", "tools/bootstrap.sh"], cwd=self.app, env=dict(self.env, **env), text=True, capture_output=True, timeout=40)

    def assert_success(self, result):
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(git("rev-parse", "HEAD", cwd=self.app / ".compiler"), self.revision)
        identity = json.loads(subprocess.check_output([str(self.app / ".gooo")], text=True))
        self.assertEqual(identity["compiler_source_sha"], self.revision)
        self.assertEqual(identity["go_version"], "go1.27.3")
        self.assertFalse((self.app / ".bootstrap.lock").exists())

    def incomplete(self, origin=None):
        git("init", str(self.app / ".compiler"))
        git("remote", "add", "origin", origin or self.origin, cwd=self.app / ".compiler")

    def old_binary(self):
        path = self.app / ".gooo"
        path.write_bytes(b"prior working executable\n")
        return path.read_bytes()

    def assert_rejected_preserving(self, result, path, before, reason):
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(reason, result.stderr)
        self.assertEqual(snapshot(path), before)
        self.assertFalse((self.app / ".bootstrap.lock").exists())

    def test_fetch_and_checkout_failure_leave_no_checkout_and_retry(self):
        for operation in ("fetch", "checkout"):
            with self.subTest(operation=operation):
                result = self.run_bootstrap(FAIL_GIT=operation)
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse((self.app / ".compiler").exists())
                self.assertFalse((self.app / ".bootstrap.lock").exists())
        self.assert_success(self.run_bootstrap())

    def test_failed_first_build_does_not_publish_checkout_or_binary(self):
        before = self.old_binary()
        result = self.run_bootstrap(FAIL_BUILD="1")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual((self.app / ".gooo").read_bytes(), before)
        self.assertFalse((self.app / ".compiler").exists())
        self.assert_success(self.run_bootstrap())

    def test_valid_existing_checkout_reused_after_build_failure(self):
        self.assert_success(self.run_bootstrap())
        before = (self.app / ".gooo").read_bytes()
        fetches = (self.state / "git.log").read_text()
        self.assertNotEqual(self.run_bootstrap(FAIL_BUILD="1").returncode, 0)
        self.assertEqual((self.app / ".gooo").read_bytes(), before)
        self.assert_success(self.run_bootstrap())
        self.assertEqual((self.state / "git.log").read_text(), fetches)

    def test_build_identity_failure_does_not_replace_old_binary(self):
        before = self.old_binary()
        for field in ("compiler_source_sha", "go_version", "native_go_required", "source_status", "vcs_modified", "version"):
            with self.subTest(field=field):
                result = self.run_bootstrap(BAD_IDENTITY=field)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("identity differs", result.stderr)
                self.assertEqual((self.app / ".gooo").read_bytes(), before)
                self.assertFalse((self.app / ".compiler").exists())
        self.assert_success(self.run_bootstrap())

    def test_legacy_failed_fetch_is_preserved_and_recovered(self):
        self.incomplete()
        before = snapshot(self.app / ".compiler")
        self.assertNotEqual(self.run_bootstrap(FAIL_GIT="fetch").returncode, 0)
        self.assertEqual(snapshot(self.app / ".compiler"), before)
        self.assert_success(self.run_bootstrap())
        backups = list(self.app.glob(".compiler-interrupted.*/compiler"))
        self.assertEqual(len(backups), 1)
        self.assertEqual(snapshot(backups[0]), before)

    def test_wrong_origin_incomplete_checkout_is_preserved(self):
        self.incomplete("https://example.invalid/unrelated.git")
        path = self.app / ".compiler"
        before = snapshot(path)
        self.assert_rejected_preserving(self.run_bootstrap(), path, before, "unrecognized origin")

    def test_working_files_in_incomplete_checkout_are_preserved(self):
        self.incomplete()
        (self.app / ".compiler" / "keep.txt").write_text("user data")
        path = self.app / ".compiler"
        before = snapshot(path)
        self.assert_rejected_preserving(self.run_bootstrap(), path, before, "working files")

    def test_nested_compiler_directory_is_not_treated_as_empty(self):
        self.incomplete()
        (self.app / ".compiler" / ".compiler").mkdir()
        compiler = self.app / ".compiler"
        before = snapshot(compiler)
        self.assert_rejected_preserving(self.run_bootstrap(), compiler, before, "working files")

    def test_failed_git_inspection_is_not_treated_as_empty_or_clean(self):
        self.incomplete()
        compiler = self.app / ".compiler"
        for command in ("for-each-ref", "ls-files"):
            before = snapshot(compiler)
            self.assert_rejected_preserving(self.run_bootstrap(FAIL_READ=command), compiler, before, "cannot inspect")
        self.assert_success(self.run_bootstrap())
        before = snapshot(compiler)
        self.assert_rejected_preserving(self.run_bootstrap(FAIL_READ="status"), compiler, before, "cannot inspect")

    def test_wrong_revision_is_preserved(self):
        git("clone", self.origin, str(self.app / ".compiler"))
        compiler = self.app / ".compiler"
        git("config", "user.name", "Fixture", cwd=compiler)
        git("config", "user.email", "fixture@example.invalid", cwd=compiler)
        (compiler / "source.txt").write_text("user commit")
        git("commit", "-am", "user", cwd=compiler)
        before = snapshot(compiler)
        self.assert_rejected_preserving(self.run_bootstrap(), compiler, before, "existing checkout preserved")

    def test_dirty_untracked_ignored_source_is_preserved(self):
        self.assert_success(self.run_bootstrap())
        compiler = self.app / ".compiler"
        for name in ("source.txt", "untracked.txt", "ignored.txt"):
            with self.subTest(name=name):
                path = compiler / name
                original = path.read_bytes() if path.exists() else None
                path.write_text("user data")
                before = snapshot(path)
                self.assert_rejected_preserving(self.run_bootstrap(), path, before, "existing checkout preserved")
                if original is None: path.unlink()
                else: path.write_bytes(original)

    def test_unknown_compiler_path_and_broken_symlink_are_preserved(self):
        path = self.app / ".compiler"
        path.write_text("unknown user file")
        before = snapshot(path)
        self.assert_rejected_preserving(self.run_bootstrap(), path, before, "regular compiler checkout")
        path.unlink()
        path.symlink_to(self.base / "missing")
        before = snapshot(path)
        self.assert_rejected_preserving(self.run_bootstrap(), path, before, "symlink")

    def test_executable_directory_and_symlink_are_preserved(self):
        path = self.app / ".gooo"
        path.mkdir(); (path / "keep").write_text("unknown")
        before = snapshot(path)
        self.assert_rejected_preserving(self.run_bootstrap(), path, before, "regular file")
        shutil.rmtree(path)
        target = self.base / "binary"; target.write_text("original")
        path.symlink_to(target)
        before = snapshot(path)
        self.assert_rejected_preserving(self.run_bootstrap(), path, before, "symlink")
        self.assertEqual(target.read_text(), "original")

    def test_wrong_go_fails_before_creating_install_state(self):
        result = self.run_bootstrap(TEST_GOVERSION="go1.28.0")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("GOTOOLCHAIN=go1.27.3", result.stderr)
        self.assertFalse((self.app / ".compiler").exists())
        self.assertFalse((self.app / ".bootstrap.lock").exists())
        self.assertFalse((self.state / "git.log").exists())

    def start_waiting(self):
        process = subprocess.Popen(["sh", "tools/bootstrap.sh"], cwd=self.app, env=dict(self.env, WAIT_BUILD="1"), stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, start_new_session=True)
        self.addCleanup(lambda: self.stop_process(process))
        for _ in range(400):
            if (self.state / "entered").exists(): return process
            if process.poll() is not None: self.fail(str(process.communicate()))
            time.sleep(.01)
        self.fail("controlled build did not start")

    @staticmethod
    def stop_process(process):
        if process.poll() is None:
            os.killpg(process.pid, signal.SIGKILL)
        process.communicate()

    def test_concurrent_invocation_does_not_enter_build_or_remove_lock(self):
        process = self.start_waiting()
        result = self.run_bootstrap()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("another bootstrap may be running", result.stderr)
        self.assertEqual((self.state / "build.log").read_text().splitlines(), ["build"])
        self.assertTrue((self.app / ".bootstrap.lock").exists())
        (self.state / "release").touch()
        stdout, stderr = process.communicate(timeout=40)
        self.assert_success(subprocess.CompletedProcess([], process.returncode, stdout, stderr))

    def assert_signal_recovery(self, signum):
        before = self.old_binary()
        process = self.start_waiting()
        os.killpg(process.pid, signum)
        process.communicate(timeout=10)
        self.assertNotEqual(process.returncode, 0)
        self.assertFalse((self.app / ".bootstrap.lock").exists())
        self.assertFalse((self.app / ".compiler").exists())
        self.assertEqual((self.app / ".gooo").read_bytes(), before)
        self.assert_success(self.run_bootstrap())

    def test_termination_cleans_owned_stage_and_preserves_old_binary(self):
        self.assert_signal_recovery(signal.SIGTERM)

    def test_interrupt_cleans_owned_stage_and_preserves_old_binary(self):
        self.assert_signal_recovery(signal.SIGINT)

    def test_hangup_cleans_owned_stage_and_preserves_old_binary(self):
        self.assert_signal_recovery(signal.SIGHUP)

    def test_user_edit_during_existing_checkout_build_is_preserved(self):
        self.assert_success(self.run_bootstrap())
        binary = (self.app / ".gooo").read_bytes()
        process = self.start_waiting()
        source = self.app / ".compiler/source.txt"
        source.write_text("edit during build")
        (self.state / "release").touch()
        stdout, stderr = process.communicate(timeout=40)
        self.assertNotEqual(process.returncode, 0, stdout + stderr)
        self.assertIn("changed while building", stderr)
        self.assertEqual(source.read_text(), "edit during build")
        self.assertEqual((self.app / ".gooo").read_bytes(), binary)
        self.assertFalse((self.app / ".bootstrap.lock").exists())

    def test_user_file_added_during_legacy_recovery_stays_in_place(self):
        self.incomplete()
        process = self.start_waiting()
        path = self.app / ".compiler/keep.txt"; path.write_text("new user data")
        before = snapshot(self.app / ".compiler")
        (self.state / "release").touch()
        stdout, stderr = process.communicate(timeout=40)
        self.assertNotEqual(process.returncode, 0, stdout + stderr)
        self.assertIn("working files", stderr)
        self.assertEqual(snapshot(self.app / ".compiler"), before)
        self.assertFalse(list(self.app.glob(".compiler-interrupted.*")))
        self.assertFalse((self.app / ".bootstrap.lock").exists())

    def test_sigkill_lock_is_preserved_with_explicit_recovery(self):
        process = self.start_waiting()
        os.killpg(process.pid, signal.SIGKILL); process.communicate(timeout=10)
        lock = self.app / ".bootstrap.lock"; before = snapshot(lock)
        result = self.run_bootstrap()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("inspect and move the lock aside", result.stderr)
        self.assertEqual(snapshot(lock), before)
        lock.rename(self.app / "preserved-killed-bootstrap")
        self.assert_success(self.run_bootstrap())

    def test_unknown_lock_is_never_deleted(self):
        lock = self.app / ".bootstrap.lock"; lock.mkdir(); (lock / "user-data").write_text("preserve")
        before = snapshot(lock)
        result = self.run_bootstrap()
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(snapshot(lock), before)


if __name__ == "__main__":
    unittest.main(verbosity=2)
