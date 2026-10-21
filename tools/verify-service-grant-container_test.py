"""Offline lifecycle regressions; no Docker daemon or native command is run."""
import importlib.util
import io
import json
from pathlib import Path
import tempfile
import unittest
import types
from unittest import mock

SUBJECT = Path(__file__).with_name("verify-service-grant-container.py")

class CreateReplyLost(RuntimeError):
    pass

class CleanupUnavailable(RuntimeError):
    pass

class FakeTrial:
    def __init__(self, out, primary, cleanup_error=None):
        self.out, self.image, self.context = out, "sha256:" + "a" * 64, out
        self.docker, self.label = "/not-a-real-docker", "io.gooo.test=fake"
        self.primary, self.cleanup_error = primary, cleanup_error
        self.removal_names, self.absence_names = [], []
        self.create_name = None

    def command(self, argv, accept_failure=False):
        if argv[:2] == ["container", "create"]:
            self.create_name = argv[argv.index("--name") + 1]
            intent = json.loads((self.out / "default/intent.json").read_text())
            assert intent["container_name"] == self.create_name
            # The daemon may have created it even though no CID was returned.
            raise self.primary
        if argv[:3] == ["container", "rm", "--force"]:
            self.removal_names.append(argv[3])
            assert argv[3] == self.create_name
            if self.cleanup_error is not None:
                raise self.cleanup_error
            return 0, (self.create_name + "\n").encode(), b""
        raise AssertionError("Unexpected fake command: " + repr(argv))

    def removed(self, name):
        self.absence_names.append(name)
        assert name == self.create_name
        if self.cleanup_error is not None:
            raise self.cleanup_error

    def inspect(self, name):
        raise AssertionError("No CID returned; only recovery-name removal is expected")

class ConstructorCleanupTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="fake-container-", dir=Path(__file__).parent)
        self.addCleanup(self.temp.cleanup)
        self.out = Path(self.temp.name)
        # Defense against accidentally launching any real Docker/native child.
        self.no_children = mock.patch("subprocess.Popen", side_effect=AssertionError("real subprocess forbidden"))
        self.no_children.start()
        self.addCleanup(self.no_children.stop)
        spec = importlib.util.spec_from_file_location("reviewed_container_smoke", SUBJECT)
        self.subject = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.subject)

    def test_lost_create_reply_removes_by_durable_name_and_preserves_primary(self):
        primary = CreateReplyLost("injected create reply lost")
        trial = FakeTrial(self.out, primary)
        with self.assertRaises(CreateReplyLost) as caught:
            self.subject.ContainerSession(trial, "default")
        self.assertIs(caught.exception, primary)
        self.assertEqual(trial.removal_names, [trial.create_name])
        self.assertEqual(trial.absence_names, [trial.create_name])
        result = json.loads((self.out / "default/result.json").read_text())
        self.assertIs(result["container_absence_verified"], True)
        self.assertIn("create", json.dumps(result["first_failure"]).lower())

    def test_cleanup_failure_cannot_replace_create_primary_or_claim_absence(self):
        primary = CreateReplyLost("injected create reply lost")
        cleanup = CleanupUnavailable("injected cleanup daemon unavailable")
        trial = FakeTrial(self.out, primary, cleanup)
        with self.assertRaises(CreateReplyLost) as caught:
            self.subject.ContainerSession(trial, "default")
        self.assertIs(caught.exception, primary)
        self.assertEqual(trial.removal_names, [trial.create_name])
        result = json.loads((self.out / "default/result.json").read_text())
        self.assertIsNot(result.get("container_absence_verified"), True)
        self.assertIn("injected create reply lost", json.dumps(result["first_failure"]))
        self.assertIn("injected cleanup daemon unavailable", json.dumps(result["cleanup_failure"]))

    def test_signal_during_cleanup_is_latched_until_after_removal(self):
        primary = CreateReplyLost("injected create reply lost")
        trial = FakeTrial(self.out, primary)
        original = trial.removed
        def removal_with_signal(name):
            self.subject.PENDING_SIGNAL = 15
            original(name)
        trial.removed = removal_with_signal
        with self.assertRaises(CreateReplyLost):
            self.subject.ContainerSession(trial, "default")
        result = json.loads((self.out / "default/result.json").read_text())
        self.assertTrue(result["container_absence_verified"])
        with self.assertRaises(KeyboardInterrupt):
            self.subject.checkpoint()

    def test_diagnostic_write_failure_keeps_first_setup_error_and_closes_pipes(self):
        stdout, stderr = io.BytesIO(), io.BytesIO()
        child = types.SimpleNamespace(pid=99999999, stdout=stdout, stderr=stderr,
                                      returncode=0, wait=lambda **kwargs: 0)
        trial = self.subject.Trial("/not-docker", "sha256:" + "a" * 64,
                                   self.out, self.out, "io.gooo.test=fake")
        with mock.patch.object(self.subject.subprocess, "Popen", return_value=child), \
             mock.patch.object(self.subject.os, "killpg", side_effect=ProcessLookupError), \
             mock.patch.object(Path, "write_bytes", side_effect=OSError("diagnostic disk failure")):
            with self.assertRaises(io.UnsupportedOperation) as caught:
                trial.command(["fake"])
        self.assertTrue(stdout.closed and stderr.closed)
        record = json.loads((self.out / "commands/1/process.json").read_text())
        self.assertIn("UnsupportedOperation", record["failure"])
        self.assertTrue(any("diagnostic disk failure" in note for note in record["secondary_diagnostic_errors"]))

class RegistryLaunchInputTests(unittest.TestCase):
    def subject(self):
        spec = importlib.util.spec_from_file_location('registry_launch_test', SUBJECT)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        return module

    def test_actual_descriptor_drives_fixed_reference_and_runtime_argv(self):
        module = self.subject()
        package = module.registry_package(SUBJECT.parents[1] / 'registry/service-grant-preview16.server.json')
        self.assertEqual(package['runtime_argv'], ['--network', 'none', '--read-only', '--platform', 'linux/amd64'])
        trial = types.SimpleNamespace(image='sha256:' + 'a' * 64, label='io.gooo.test=registry', registry_package=package)
        argv = module.create_arguments(trial, 'owned-name')
        self.assertEqual(argv[-1], package['identifier'])
        self.assertIn('@sha256:a83d8fa57657bb877453cce7ed11f1ff1000a84b1e80c65a4e1c0b20196d3ee4', argv[-1])
        self.assertEqual(argv[argv.index('--network') + 1], 'none')
        self.assertEqual(argv[argv.index('--platform') + 1], 'linux/amd64')
        self.assertIn('--read-only', argv)
        self.assertNotIn(trial.image, argv)

    def test_changed_descriptor_is_refused_without_a_subprocess(self):
        module = self.subject()
        with tempfile.TemporaryDirectory(dir=SUBJECT.parent) as temporary:
            path = Path(temporary) / 'changed.json'
            raw = (SUBJECT.parents[1] / 'registry/service-grant-preview16.server.json').read_bytes()
            path.write_bytes(raw.replace(b'"none"', b'"host"'))
            with mock.patch('subprocess.Popen', side_effect=AssertionError('must not spawn')):
                with self.assertRaisesRegex(RuntimeError, 'descriptor bytes changed'):
                    module.registry_package(path)


if __name__ == "__main__":
    unittest.main()
