import base64
import json
import os
from pathlib import Path
import stat
import tempfile
import unittest
from unittest import mock

import registry_action_auth as auth


class RegistryActionAuthTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix='gooo-auth-test-')
        self.root = Path(self.tmp.name).resolve()
        self.home = self.root / 'home'
        self.home.mkdir()
        self.writer = self.root / 'writer'
        self.writer.mkdir(mode=0o700)
        self.config = self.writer / 'config.json'
        self.raw = json.dumps({'auths': {'ghcr.io': {'auth': base64.b64encode(b'fixture:dummy-only').decode()}}}).encode()
        self.config.write_bytes(self.raw)
        self.config.chmod(0o600)
        self.state = self.root / 'state'
        self.env = dict(GITHUB_ACTIONS='true', GITHUB_REPOSITORY='kimjooyoon/gooo-go-ports',
                        GITHUB_REF='refs/heads/main', GITHUB_RUN_ATTEMPT='1', GITHUB_RUN_ID='42',
                        GITHUB_ACTOR='fixture', RUNNER_TEMP=str(self.root))

    def tearDown(self):
        self.tmp.cleanup()

    def stage(self):
        return auth.stage(self.config, self.home, self.state, self.env)

    def restore(self):
        return auth.restore(self.home, self.state, self.env)

    def original(self, raw=b'{ "auths": {}, "keep": "exact spacing" }\n', mode=0o640):
        directory = self.home / '.docker'
        directory.mkdir()
        path = directory / 'config.json'
        path.write_bytes(raw)
        path.chmod(mode)
        return path

    def test_existing_default_bytes_and_mode_restored(self):
        path = self.original()
        before = path.read_bytes()
        self.assertEqual(self.stage(), 'ACTION_REGISTRY_AUTH_STAGED')
        self.assertEqual(path.read_bytes(), self.raw)
        self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o600)
        self.assertEqual(self.restore(), 'ACTION_REGISTRY_AUTH_RESTORED')
        self.assertEqual(path.read_bytes(), before)
        self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o640)
        self.assertFalse((self.state / 'original').exists())
        self.restore()
        self.assertEqual(path.read_bytes(), before)

    def test_previously_missing_directory_removed(self):
        self.stage()
        self.assertTrue((self.home / '.docker/config.json').exists())
        self.restore()
        self.assertFalse((self.home / '.docker').exists())
        self.restore()

    def test_existing_empty_directory_preserved(self):
        (self.home / '.docker').mkdir(mode=0o750)
        self.stage()
        self.restore()
        self.assertTrue((self.home / '.docker').is_dir())
        self.assertEqual(stat.S_IMODE((self.home / '.docker').stat().st_mode), 0o750)

    def test_identical_bytes_still_restore_original_mode(self):
        path = self.original(self.raw, 0o640)
        self.stage()
        self.restore()
        self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o640)

    def test_concurrent_change_is_not_overwritten(self):
        path = self.original()
        self.stage()
        path.write_bytes(b'newer actor data')
        with self.assertRaisesRegex(auth.AuthError, 'CONFIG_CHANGED_CONCURRENTLY'):
            self.restore()
        self.assertEqual(path.read_bytes(), b'newer actor data')
        self.assertTrue((self.state / 'original').is_file())

    def test_changed_backup_is_not_applied(self):
        self.original()
        self.stage()
        (self.state / 'original').write_bytes(b'tampered backup')
        with self.assertRaisesRegex(auth.AuthError, 'BACKUP_CHANGED'):
            self.restore()

    def test_symlink_target_refused(self):
        target = self.original()
        target.unlink()
        target.symlink_to(self.config)
        with self.assertRaisesRegex(auth.AuthError, 'SYMLINK_TARGET'):
            self.stage()

    def test_symlink_directory_refused(self):
        (self.home / '.docker').symlink_to(self.writer, target_is_directory=True)
        with self.assertRaisesRegex(auth.AuthError, 'UNSAFE_DIRECTORY'):
            self.stage()

    def test_source_permissions_refused(self):
        self.config.chmod(0o644)
        with self.assertRaisesRegex(auth.AuthError, 'PRIVATE_CONFIG_PERMISSIONS'):
            self.stage()

    def test_source_other_registry_and_actor_refused(self):
        for value, code in [({'auths': {'example.com': {'auth': 'eDp5'}}}, 'REGISTRY_SCOPE'),
                            ({'auths': {'ghcr.io': {'auth': 'eDp5'}}}, 'ACTOR_OR_PASSWORD')]:
            self.config.write_text(json.dumps(value))
            with self.assertRaisesRegex(auth.AuthError, code):
                self.stage()
            self.assertFalse(self.state.exists())

    def test_duplicate_config_key_refused(self):
        self.config.write_text('{"auths":{},"auths":{}}')
        with self.assertRaisesRegex(auth.AuthError, 'DUPLICATE_JSON_KEY'):
            self.stage()

    def test_context_and_state_scope_refused(self):
        for key, value in [('GITHUB_ACTIONS', 'false'), ('GITHUB_REPOSITORY', 'other/repo'),
                           ('GITHUB_REF', 'refs/pull/1/merge'), ('GITHUB_RUN_ATTEMPT', '2')]:
            env = dict(self.env, **{key: value})
            with self.assertRaises(auth.AuthError):
                auth.stage(self.config, self.home, self.state, env)
        with self.assertRaisesRegex(auth.AuthError, 'STATE_OUTSIDE_RUNNER_TEMP'):
            auth.stage(self.config, self.home, self.root / 'writer/nested', self.env)

    def test_state_cannot_be_reused_or_restored_by_other_run(self):
        self.stage()
        with self.assertRaisesRegex(auth.AuthError, 'STATE_ALREADY_EXISTS'):
            self.stage()
        with self.assertRaisesRegex(auth.AuthError, 'STATE_CONTEXT'):
            auth.restore(self.home, self.state, dict(self.env, GITHUB_RUN_ID='43'))

    def test_partial_stage_before_target_write_restores_original(self):
        target = self.original()
        before = target.read_bytes()
        original_atomic = auth.atomic
        def interrupted(path, raw, mode=0o600):
            if path == target:
                raise OSError('synthetic interruption before target write')
            return original_atomic(path, raw, mode)
        with mock.patch.object(auth, 'atomic', interrupted):
            with self.assertRaises(OSError):
                self.stage()
        self.assertEqual(json.loads((self.state / 'state.json').read_bytes())['status'], 'prepared')
        self.restore()
        self.assertEqual(target.read_bytes(), before)
        self.assertEqual(stat.S_IMODE(target.stat().st_mode), 0o640)

    def test_partial_stage_after_target_write_restores_original(self):
        target = self.original()
        before = target.read_bytes()
        original_atomic = auth.atomic
        def interrupted(path, raw, mode=0o600):
            if path == self.state / 'state.json' and json.loads(raw)['status'] == 'staged':
                raise OSError('synthetic interruption after target write')
            return original_atomic(path, raw, mode)
        with mock.patch.object(auth, 'atomic', interrupted):
            with self.assertRaises(OSError):
                self.stage()
        self.assertEqual(target.read_bytes(), self.raw)
        self.assertEqual(json.loads((self.state / 'state.json').read_bytes())['status'], 'prepared')
        self.restore()
        self.assertEqual(target.read_bytes(), before)
        self.assertEqual(stat.S_IMODE(target.stat().st_mode), 0o640)

    def test_new_directory_foreign_file_not_deleted(self):
        self.stage()
        foreign = self.home / '.docker/keep'
        foreign.write_text('keep')
        with self.assertRaisesRegex(auth.AuthError, 'NEW_DOCKER_DIRECTORY_NOT_EMPTY'):
            self.restore()
        self.assertEqual(foreign.read_text(), 'keep')


if __name__ == '__main__':
    unittest.main()
