#!/usr/bin/env python3
# Copyright (c) 2026 Gooo Go Ports contributors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""Offline safety checks; one stdlib fake child, no publisher/HTTP/authentication."""
import copy
import gzip
import importlib.util
import io
from pathlib import Path
import struct
import tarfile
import types
import unittest
from unittest import mock

SOURCE = Path(__file__).with_name('publish-service-grant-registry.py')
SPEC = importlib.util.spec_from_file_location('registry_publisher', SOURCE)
M = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(M)


def release():
    return {
        'id': M.RELEASE_ID, 'draft': False, 'prerelease': True,
        'published_at': '2026-10-19T00:00:00Z', 'tag_name': M.TAG,
        'target_commitish': M.SOURCE,
        'html_url': f'https://github.com/{M.REPOSITORY}/releases/tag/{M.TAG}',
        'assets': [dict(id=M.ASSET_IDS[name], name=name, size=size, digest='sha256:' + digest, state='uploaded',
                        browser_download_url=f'https://github.com/{M.REPOSITORY}/releases/download/{M.TAG}/{name}')
                   for name, (size, digest) in M.ASSETS.items()],
    }


def elf():
    value = bytearray(64)
    value[:6] = b'\x7fELF\x02\x01'
    struct.pack_into('<H', value, 18, 62)
    return bytes(value)


def archive(rows):
    raw = io.BytesIO()
    with tarfile.open(fileobj=raw, mode='w', format=tarfile.USTAR_FORMAT) as target:
        for name, kind, body in rows:
            member = tarfile.TarInfo(name)
            member.type = kind
            if kind == tarfile.REGTYPE:
                member.size = len(body)
                target.addfile(member, io.BytesIO(body))
            else:
                member.linkname = '/tmp/unapproved'
                target.addfile(member)
    return gzip.compress(raw.getvalue())


class PublicReleaseGate(unittest.TestCase):
    def test_complete_fixed_public_release(self):
        self.assertEqual(len(M.release_assets(release())), 4)

    def test_draft_stops_before_source_download_or_authentication(self):
        value = release()
        value['draft'] = True

        class FakeRuntime:
            calls = []

            def public_json(self, label, url):
                self.calls.append(label)
                return value

            def fetch(self, *args):
                raise AssertionError('download must not begin')

            def command(self, *args, **kwargs):
                raise AssertionError('publisher must not start')

        runtime = FakeRuntime()
        with self.assertRaisesRegex(M.Refused, 'public prerelease'):
            M.public_preview(runtime)
        self.assertEqual(runtime.calls, ['public-release14'])

    def test_unpublished_release_refused(self):
        value = release()
        value['published_at'] = None
        with self.assertRaises(M.Refused):
            M.release_assets(value)

    def test_wrong_release_id_or_bool_refused(self):
        for identity in (M.RELEASE_ID + 1, True):
            value = release()
            value['id'] = identity
            with self.assertRaises(M.Refused):
                M.release_assets(value)

    def test_source_change_refused(self):
        value = release()
        value['target_commitish'] = 'main'
        with self.assertRaises(M.Refused):
            M.release_assets(value)

    def test_missing_asset_refused(self):
        value = release()
        value['assets'].pop()
        with self.assertRaises(M.Refused):
            M.release_assets(value)

    def test_duplicate_asset_refused(self):
        value = release()
        value['assets'][-1] = copy.deepcopy(value['assets'][0])
        with self.assertRaises(M.Refused):
            M.release_assets(value)

    def test_digest_or_foreign_download_url_refused(self):
        for field, replacement in (('digest', 'sha256:' + '0' * 64),
                                   ('browser_download_url', 'https://example.com/archive')):
            value = release()
            value['assets'][0][field] = replacement
            with self.assertRaises(M.Refused):
                M.release_assets(value)


class Preview14IdentityGate(unittest.TestCase):
    def test_published_preview13_metadata_bytes_remain_unchanged(self):
        raw = (SOURCE.parent.parent / 'registry/service-grant-preview13.server.json').read_bytes()
        self.assertEqual(len(raw), 581)
        self.assertEqual(M.sha(raw),
                         '891529d92ac784ed5068f33da8297402a77b5389a86b710d81e6192e02c97f6f')
        self.assertEqual(M.load(raw)['version'], '0.1.0-preview.13')
        self.assertEqual(M.VERSION, '0.1.0-preview.14')

    def test_native_asset_id_mismatch_and_bool_refused(self):
        for identity in (M.ASSET_IDS[next(iter(M.ASSETS))] + 1, True):
            value = release()
            value['assets'][0]['id'] = identity
            with self.subTest(identity=identity), self.assertRaisesRegex(M.Refused, 'asset ID'):
                M.release_assets(value)

    def test_BUNDLE_raw_hash_and_source_are_fixed(self):
        name = next(iter(M.BUNDLE_IDENTITIES))
        body = M.json.dumps({'source_commit': M.SOURCE}).encode()
        data = archive([('BUNDLE.json', tarfile.REGTYPE, body)])
        with mock.patch.dict(M.BUNDLE_IDENTITIES, {name: (len(body), M.sha(body))}, clear=True):
            self.assertEqual(M.preview_bundle_identity(data, name),
                             {'bytes': len(body), 'sha256': M.sha(body),
                              'source_commit': M.SOURCE})
            changed = archive([('BUNDLE.json', tarfile.REGTYPE, body.replace(b'694', b'693', 1))])
            with self.assertRaisesRegex(M.Refused, 'BUNDLE bytes'):
                M.preview_bundle_identity(changed, name)
        wrong = M.json.dumps({'source_commit': 'c87d924747b894f1fdb0f74b513fc504129e4de0'}).encode()
        with mock.patch.dict(M.BUNDLE_IDENTITIES, {name: (len(wrong), M.sha(wrong))}, clear=True):
            with self.assertRaisesRegex(M.Refused, 'BUNDLE source'):
                M.preview_bundle_identity(
                    archive([('BUNDLE.json', tarfile.REGTYPE, wrong)]), name)


class PublisherArchiveGate(unittest.TestCase):
    def test_unpinned_whole_archive_refused_before_unpacking(self):
        with self.assertRaisesRegex(M.Refused, 'archive hash changed'):
            M.publisher_member(b'not-the-official-archive')

    def test_unexpected_fourth_member_refused_by_inner_tar_gate(self):
        # Inner stage only: the public bootstrap always checks whole SHA first.
        raw = archive([('LICENSE', tarfile.REGTYPE, b'a'),
                       ('README.md', tarfile.REGTYPE, b'b'),
                       ('mcp-publisher', tarfile.REGTYPE, elf()),
                       ('extra-binary', tarfile.REGTYPE, elf())])
        with self.assertRaisesRegex(M.Refused, 'member count'):
            M.publisher_tar_member(gzip.decompress(raw))


class MetadataGate(unittest.TestCase):
    def test_fixed_metadata_hash_and_shape(self):
        path = SOURCE.parent.parent / 'registry/service-grant-preview14.server.json'
        raw = path.read_bytes()
        self.assertEqual(len(raw), M.METADATA_BYTES)
        self.assertEqual(M.sha(raw), M.METADATA_SHA)
        value = M.load(raw)
        self.assertEqual(value['name'], M.NAME)
        self.assertEqual(value['version'], M.VERSION)
        self.assertNotIn('packages', value)
        self.assertNotIn('remotes', value)
        self.assertLessEqual(len(value['description']), 100)
        self.assertEqual(value['websiteUrl'],
                         f'https://github.com/{M.REPOSITORY}/releases/tag/{M.TAG}')

    def test_duplicate_and_nonfinite_json_refused(self):
        for raw in (b'{"name":"a","name":"b"}', b'{"n":NaN}'):
            with self.assertRaises(M.Refused):
                M.load(raw)

    def test_public_readback_must_preserve_types_and_values(self):
        M.typed_subset({'name': M.NAME, 'repository': {'source': 'github', 'id': 'extra'}},
                       {'name': M.NAME, 'repository': {'source': 'github'}})
        for actual in ({'n': True}, {'n': 2}):
            with self.assertRaises(M.Refused):
                M.typed_subset(actual, {'n': 1})


class CancellationPriority(unittest.TestCase):
    def command_cancelled_during_cleanup(self, exit_code):
        runtime = object.__new__(M.Runtime)
        runtime.home = Path('/synthetic-never-created')
        runtime.calls = []
        runtime.first_failure = None
        runtime.cleaning = False
        runtime.checkpoint = lambda: None
        runtime.journal = lambda: None
        pipe = types.SimpleNamespace(fileno=lambda: 123, close=lambda: None)

        class FakeProcess:
            pid = 123456789
            returncode = exit_code
            stdout = stderr = pipe
            waits = 0

            def poll(self):
                return self.returncode

            def wait(self, timeout):
                self.waits += 1
                if self.waits == 2:
                    self.assert_cleanup_state()
                    runtime.interrupted(M.signal.SIGTERM, None)
                return self.returncode

            @staticmethod
            def assert_cleanup_state():
                if not runtime.cleaning:
                    raise AssertionError('signal must be injected in protected cleanup')

        selector = types.SimpleNamespace(register=lambda *a: None,
                                         get_map=lambda: {}, close=lambda: None)
        with mock.patch.object(M.subprocess, 'Popen', return_value=FakeProcess()), \
                mock.patch.object(M.selectors, 'DefaultSelector', return_value=selector), \
                mock.patch.object(M.signal, 'pthread_sigmask', return_value=set()), \
                mock.patch.object(M.os, 'set_blocking'), \
                mock.patch.object(M.os, 'killpg', side_effect=ProcessLookupError):
            try:
                runtime.command('offline-mocked-command', ['never-executed'])
            except BaseException as error:
                return runtime, error
        self.fail('a cancellation or prior failure must be raised')

    def test_cleanup_signal_is_not_a_success(self):
        runtime, error = self.command_cancelled_during_cleanup(0)
        self.assertIsInstance(error, M.PublisherCancelled)
        self.assertIs(error, runtime.first_failure)
        self.assertEqual(runtime.calls[0]['failure_class'], 'PublisherCancelled')

    def test_prior_command_refusal_survives_cleanup_signal(self):
        runtime, error = self.command_cancelled_during_cleanup(2)
        self.assertIsInstance(error, M.Refused)
        self.assertIs(error, runtime.first_failure)
        self.assertEqual(runtime.calls[0]['failure_class'], 'Refused')

class ValidationOnlySafety(unittest.TestCase):
    def context(self, **changes):
        value = {'GITHUB_ACTIONS': 'true', 'GITHUB_REPOSITORY': M.REPOSITORY,
                 'GITHUB_EVENT_NAME': 'pull_request', 'GITHUB_REF': 'refs/pull/123/merge'}
        value.update(changes)
        return value

    def test_validation_context_is_owned_PR_without_OIDC_capability(self):
        self.assertEqual(M.execution_mode(True, False, self.context()), 'validate-only')
        for key, value in (('GITHUB_ACTIONS', 'false'), ('GITHUB_REPOSITORY', 'other/fork'),
                           ('GITHUB_EVENT_NAME', 'pull_request_target'),
                           ('GITHUB_EVENT_NAME', 'workflow_dispatch'),
                           ('GITHUB_REF', 'refs/heads/main'), ('GITHUB_REF', 'refs/pull/0/merge'),
                           ('GITHUB_REF', 'refs/pull/123/head'),
                           ('ACTIONS_ID_TOKEN_REQUEST_TOKEN', 'synthetic-not-a-real-token'),
                           ('ACTIONS_ID_TOKEN_REQUEST_URL', 'https://synthetic.invalid/oidc')):
            with self.subTest(context=(key, value)), self.assertRaises(M.Refused):
                M.execution_mode(True, False, self.context(**{key: value}))
        for flags in ((False, False), (True, True), (1, False)):
            with self.subTest(flags=flags), self.assertRaises(M.Refused):
                M.execution_mode(*flags, self.context())

    def test_publish_context_still_requires_owned_manual_main(self):
        env = self.context(GITHUB_EVENT_NAME='workflow_dispatch', GITHUB_REF='refs/heads/main')
        self.assertEqual(M.execution_mode(False, True, env), 'publish')
        for key, value in (('GITHUB_REPOSITORY', 'other/fork'),
                           ('GITHUB_REF', 'refs/heads/feature'),
                           ('GITHUB_EVENT_NAME', 'pull_request')):
            with self.subTest(context=(key, value)), self.assertRaises(M.Refused):
                M.execution_mode(False, True, dict(env, **{key: value}))

    def test_auth_publish_foreign_arguments_and_order_are_refused_before_spawn(self):
        runtime = object.__new__(M.Runtime)
        runtime.validate_only = True
        runtime.home = Path('/synthetic-never-created')
        runtime.calls = []
        executable = str(runtime.home / 'mcp-publisher')
        server_file = str(runtime.home / 'server.json')
        cases = [
            ('publisher-login', [executable, 'login', 'github-oidc'], False, []),
            ('publisher-publish', [executable, 'publish', server_file], True, []),
            ('publisher-version', [executable, '--version'], True, []),
            ('publisher-version', ['/untrusted/publisher', '--version'], False, []),
            ('publisher-validate', [executable, 'validate', server_file], False, []),
            ('publisher-validate', [executable, 'validate', '/unreviewed/server.json'], False,
             [{'label': 'publisher-version'}]),
            ('publisher-version', [executable, '--version'], False, [{}, {}]),
        ]
        with mock.patch.object(M.subprocess, 'Popen') as popen:
            for label, argv, authenticated, prior in cases:
                runtime.calls = prior
                with self.subTest(label=label, argv=argv, prior=len(prior)), self.assertRaises(M.Refused):
                    runtime.command(label, argv, authenticated=authenticated)
            popen.assert_not_called()

    def test_validation_child_environment_strips_synthetic_GitHub_and_OIDC_tokens(self):
        import tempfile
        temporary = tempfile.TemporaryDirectory(prefix='registry-public-output-mock-')
        self.addCleanup(temporary.cleanup)
        runtime = M.Runtime(Path(temporary.name) / 'out', validate_only=True)
        runtime.checkpoint = lambda: None
        runtime.journal = lambda: None
        pipe = types.SimpleNamespace(fileno=lambda: 123, close=lambda: None)
        process = types.SimpleNamespace(pid=123456789, returncode=0, stdout=pipe, stderr=pipe,
                                        poll=lambda: 0, wait=lambda timeout: 0)
        selector = types.SimpleNamespace(register=lambda *a: None, get_map=lambda: {}, close=lambda: None)
        tokens = {key: 'synthetic-not-a-real-token' for key in
                  ('GH_TOKEN', 'GITHUB_TOKEN', 'GH_ENTERPRISE_TOKEN', 'GITHUB_ENTERPRISE_TOKEN',
                   'ACTIONS_ID_TOKEN_REQUEST_TOKEN', 'ACTIONS_ID_TOKEN_REQUEST_URL',
                   'OTHER_AMBIENT_SECRET', 'OPENAI_API_KEY', 'HTTP_PROXY')}
        with mock.patch.dict(M.os.environ, tokens, clear=True), \
                mock.patch.object(M.subprocess, 'Popen', return_value=process) as popen, \
                mock.patch.object(M.selectors, 'DefaultSelector', return_value=selector), \
                mock.patch.object(M.signal, 'pthread_sigmask', return_value=set()), \
                mock.patch.object(M.os, 'set_blocking'), \
                mock.patch.object(M.os, 'killpg', side_effect=ProcessLookupError):
            self.assertEqual(runtime.command('publisher-version',
                             [str(runtime.home / 'mcp-publisher'), '--version']), b'')
        child_env = popen.call_args.kwargs['env']
        self.assertTrue(all(key not in child_env for key in tokens))
        self.assertEqual(child_env['HOME'], str(runtime.home))
        self.assertFalse(runtime.calls[0]['authenticated'])
        self.assertTrue(runtime.calls[0]['EOF'] and runtime.calls[0]['PG_ESRCH'])
        self.assertEqual(set(child_env), {'HOME', 'XDG_CONFIG_HOME', 'TMPDIR', 'PATH',
                         'LANG', 'LC_ALL', 'GOTOOLCHAIN', 'GOENV', 'GOWORK',
                         'GOPROXY', 'GOSUMDB', 'GOTELEMETRY'})
        self.assertIsNotNone(runtime.calls[0]['public_output'])
        self.assertEqual((runtime.proof / 'public-cli/publisher-version.stdout').read_bytes(), b'')
        self.assertEqual((runtime.proof / 'public-cli/publisher-version.stderr').read_bytes(), b'')

    def run_flow(self, validate_only, fail_at=None):
        import tempfile
        temp = tempfile.TemporaryDirectory(prefix='registry-no-auth-offline-')
        self.addCleanup(temp.cleanup)
        out = Path(temp.name) / 'out'
        instances, trace = [], []
        metadata = M.load((SOURCE.parent.parent / 'registry/service-grant-preview14.server.json').read_bytes())
        actual_runtime = M.Runtime

        class FakeRuntime(actual_runtime):
            def __init__(self, path, validate_only=False):
                super().__init__(path, validate_only=validate_only)
                instances.append(self)

            def registry_version(self, label, allow_missing=False):
                if self.validate_only:
                    raise AssertionError('validation must not require version absence or registry readback')
                trace.append((label, allow_missing))
                return None if label == 'registry-before' else metadata

            def command(self, label, argv, authenticated=False, timeout=30):
                trace.append((label, authenticated))
                if fail_at == label:
                    self.first_failure = M.Refused('synthetic validation refusal')
                    raise self.first_failure
                self.calls.append({'label': label, 'authenticated': authenticated, 'exit_code': 0,
                                   'direct_wait': True, 'EOF': True, 'PG_ESRCH': True})
                return b'mcp-publisher 1.8.1\n' if label == 'publisher-version' else b'validated\n'

        def preview(runtime):
            trace.append(('public-preview-all-four-whole-byte-gate', False))

        def bootstrap(runtime):
            trace.append(('official-pinned-bootstrap', False))
            return runtime.home / 'mcp-publisher'

        flag = '--validate-only' if validate_only else '--allow-publish'
        environment = self.context() if validate_only else self.context(
            GITHUB_EVENT_NAME='workflow_dispatch', GITHUB_REF='refs/heads/main')
        error = None
        with mock.patch.dict(M.os.environ, environment, clear=True), \
                mock.patch.object(__import__('sys'), 'argv',
                                  ['registry-test', '--out', str(out), flag]), \
                mock.patch.object(M.platform, 'system', return_value='Linux'), \
                mock.patch.object(M.platform, 'machine', return_value='x86_64'), \
                mock.patch.object(M, 'Runtime', FakeRuntime), \
                mock.patch.object(M, 'public_preview', side_effect=preview), \
                mock.patch.object(M, 'bootstrap', side_effect=bootstrap), \
                mock.patch.object(M.signal, 'signal', return_value=None):
            try:
                M.main()
            except BaseException as caught:
                error = caught
        return out, instances[0], trace, error

    def test_validate_only_stops_after_version_and_validate_even_if_version_exists(self):
        out, runtime, trace, error = self.run_flow(True)
        self.assertIsNone(error)
        self.assertEqual(trace, [('public-preview-all-four-whole-byte-gate', False),
                                 ('official-pinned-bootstrap', False),
                                 ('publisher-version', False), ('publisher-validate', False)])
        self.assertFalse(runtime.home.exists())
        proof = M.load((out / 'proof/validation-only.json').read_bytes())
        self.assertEqual(proof['status'], 'OFFICIAL_CLI_VALIDATED_FIXED_PUBLIC_METADATA_WITHOUT_AUTH_OR_PUBLISH')
        self.assertEqual(proof['authentication_commands'], 0)
        self.assertEqual(proof['publish_commands'], 0)
        self.assertFalse(proof['registry_version_absence_required'])
        self.assertFalse(proof['actual_registry_registration'])
        self.assertFalse(proof['automatic_client_installation_proven'])
        self.assertFalse((out / 'proof/registration.json').exists())

    def test_validate_only_first_failure_removes_home_and_never_enters_auth(self):
        out, runtime, trace, error = self.run_flow(True, 'publisher-validate')
        self.assertIs(error, runtime.first_failure)
        self.assertIsInstance(error, M.Refused)
        self.assertFalse(runtime.home.exists())
        self.assertEqual(trace[-1], ('publisher-validate', False))
        self.assertNotIn(('publisher-login', True), trace)
        self.assertNotIn(('publisher-publish', True), trace)
        self.assertFalse((out / 'proof/validation-only.json').exists())
        first = M.load((out / 'proof/first-failure.json').read_bytes())
        self.assertEqual(first['class'], 'Refused')
        self.assertEqual(first['stage'], 'official-registry-validation')

    def test_publish_keeps_absence_login_one_publish_and_readback(self):
        out, runtime, trace, error = self.run_flow(False)
        self.assertIsNone(error)
        self.assertEqual(trace, [('public-preview-all-four-whole-byte-gate', False),
                                 ('registry-before', True), ('official-pinned-bootstrap', False),
                                 ('publisher-version', False), ('publisher-validate', False),
                                 ('publisher-login', True), ('publisher-publish', True),
                                 ('registry-after', False)])
        self.assertEqual(sum(label == 'publisher-publish' for label, _ in trace), 1)
        self.assertTrue((out / 'proof/registration.json').exists())
        self.assertFalse((out / 'proof/validation-only.json').exists())
        self.assertFalse(runtime.home.exists())

    def test_namespace_still_refused_after_hash_gate_before_any_runtime(self):
        path = SOURCE.parent.parent / 'registry/service-grant-preview14.server.json'
        raw = path.read_bytes().replace(b'io.github.kimjooyoon/', b'io.github.wrongowner/')
        self.assertEqual(len(raw), M.METADATA_BYTES)
        with mock.patch.dict(M.os.environ, self.context(), clear=True), \
                mock.patch.object(__import__('sys'), 'argv',
                                  ['registry-test', '--out', '/synthetic-no-create', '--validate-only']), \
                mock.patch.object(M.platform, 'system', return_value='Linux'), \
                mock.patch.object(M.platform, 'machine', return_value='x86_64'), \
                mock.patch.object(M, 'METADATA_SHA', M.sha(raw)), \
                mock.patch.object(Path, 'read_bytes', return_value=raw), \
                mock.patch.object(M, 'Runtime') as runtime:
            with self.assertRaisesRegex(M.Refused, 'fixed custom-install metadata changed'):
                M.main()
            runtime.assert_not_called()

    def test_PR_workflow_has_no_oidc_secrets_or_publish_trigger(self):
        path = SOURCE.parent.parent / '.github/workflows/check-service-grant-registry.yml'
        raw = path.read_text()
        self.assertIn('  pull_request:\n', raw)
        self.assertNotIn('pull_request_target', raw)
        self.assertNotIn('workflow_dispatch', raw)
        self.assertNotIn('id-token:', raw)
        self.assertNotIn('secrets.', raw)
        self.assertNotIn('--allow-publish', raw)
        self.assertEqual(raw.count('      contents: read\n'), 1)
        self.assertEqual(raw.count('      - \'', raw.index('    paths:\n'), raw.index('\npermissions:')), 6)
        self.assertIn("      - 'registry/service-grant-preview13.server.json'\n", raw)
        self.assertIn("      - 'registry/service-grant-preview14.server.json'\n", raw)
        self.assertIn('    timeout-minutes: 10\n', raw)
        self.assertIn('actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1', raw)
        self.assertIn('actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9', raw)
        self.assertLess(raw.index('tools/publish-service-grant-registry_test.py'), raw.index('--validate-only'))

    def test_authenticated_raw_is_never_persisted_and_environment_is_allowlisted(self):
        import tempfile
        temporary = tempfile.TemporaryDirectory(prefix='registry-auth-mock-offline-')
        self.addCleanup(temporary.cleanup)
        runtime = M.Runtime(Path(temporary.name) / 'out')
        runtime.checkpoint = lambda: None
        stdout = types.SimpleNamespace(fileno=lambda: 123, close=lambda: None)
        stderr = types.SimpleNamespace(fileno=lambda: 124, close=lambda: None)
        process = types.SimpleNamespace(pid=123456789, returncode=0, stdout=stdout, stderr=stderr,
                                        poll=lambda: 0, wait=lambda timeout: 0)

        class Selector:
            def __init__(self):
                self.rows = {}

            def register(self, stream, _events, target):
                self.rows[stream.fileno()] = types.SimpleNamespace(fd=stream.fileno(), fileobj=stream, data=target)

            def unregister(self, stream):
                self.rows.pop(stream.fileno())

            def get_map(self):
                return self.rows

            def select(self, _timeout):
                return [(row, None) for row in list(self.rows.values())]

            def close(self):
                pass

        sentinel = b'synthetic-auth-response-must-not-be-retained'
        remaining = {123: [sentinel, b''], 124: [sentinel, b'']}
        environ = {'ACTIONS_ID_TOKEN_REQUEST_URL': 'https://synthetic.invalid/oidc',
                   'ACTIONS_ID_TOKEN_REQUEST_TOKEN': 'synthetic-not-a-real-token',
                   'GITHUB_TOKEN': 'synthetic-not-a-real-token',
                   'OTHER_AMBIENT_SECRET': 'synthetic-not-a-real-secret'}
        with mock.patch.dict(M.os.environ, environ, clear=True), \
                mock.patch.object(M.subprocess, 'Popen', return_value=process) as popen, \
                mock.patch.object(M.selectors, 'DefaultSelector', Selector), \
                mock.patch.object(M.signal, 'pthread_sigmask', return_value=set()), \
                mock.patch.object(M.os, 'read', side_effect=lambda fd, _n: remaining[fd].pop(0)), \
                mock.patch.object(M.os, 'set_blocking'), \
                mock.patch.object(M.os, 'killpg', side_effect=ProcessLookupError):
            result = runtime.command('publisher-login',
                     [str(runtime.home / 'mcp-publisher'), 'login', 'github-oidc', '--registry', M.REGISTRY],
                     authenticated=True)
        self.assertEqual(result, sentinel * 2)
        env = popen.call_args.kwargs['env']
        self.assertIn('ACTIONS_ID_TOKEN_REQUEST_TOKEN', env)
        self.assertIn('ACTIONS_ID_TOKEN_REQUEST_URL', env)
        self.assertNotIn('GITHUB_TOKEN', env)
        self.assertNotIn('OTHER_AMBIENT_SECRET', env)
        self.assertIsNone(runtime.calls[0]['public_output'])
        self.assertFalse((runtime.proof / 'public-cli').exists())
        for path in runtime.proof.iterdir():
            if path.is_file():
                self.assertNotIn(sentinel, path.read_bytes())

    def test_nonpublic_unauthenticated_argv_does_not_get_public_raw_capture(self):
        import tempfile
        temporary = tempfile.TemporaryDirectory(prefix='registry-private-argv-mock-')
        self.addCleanup(temporary.cleanup)
        runtime = M.Runtime(Path(temporary.name) / 'out')
        runtime.checkpoint = lambda: None
        pipe = types.SimpleNamespace(fileno=lambda: 123, close=lambda: None)
        process = types.SimpleNamespace(pid=123456789, returncode=0, stdout=pipe, stderr=pipe,
                                        poll=lambda: 0, wait=lambda timeout: 0)
        selector = types.SimpleNamespace(register=lambda *a: None, get_map=lambda: {}, close=lambda: None)
        with mock.patch.object(M.subprocess, 'Popen', return_value=process), \
                mock.patch.object(M.selectors, 'DefaultSelector', return_value=selector), \
                mock.patch.object(M.signal, 'pthread_sigmask', return_value=set()), \
                mock.patch.object(M.os, 'set_blocking'), \
                mock.patch.object(M.os, 'killpg', side_effect=ProcessLookupError):
            runtime.command('publisher-version', ['/unapproved/publisher', '--version'])
        self.assertIsNone(runtime.calls[0]['public_output'])
        self.assertFalse((runtime.proof / 'public-cli').exists())

    def test_first_TERM_at_cleanup_entry_preserves_primary_and_closes_actual_child(self):
        import inspect
        import sys
        import tempfile
        temporary = tempfile.TemporaryDirectory(prefix='registry-signal-offline-')
        self.addCleanup(temporary.cleanup)
        runtime = M.Runtime(Path(temporary.name) / 'out')
        primary = RuntimeError('SYNTHETIC_PRIMARY_BEFORE_CLEANUP')
        checkpoints = []

        def checkpoint():
            checkpoints.append(True)
            if len(checkpoints) == 2:
                raise primary

        runtime.checkpoint = checkpoint
        actual_popen = M.subprocess.Popen
        children = []

        def spawn(*args, **kwargs):
            child = actual_popen(*args, **kwargs)
            children.append(child)
            return child

        lines, start = inspect.getsourcelines(M.Runtime.command)
        target = start + next(i for i, line in enumerate(lines) if line.strip() == 'self.cleaning = True')
        injected = []

        def trace(frame, event, _arg):
            if (event == 'line' and frame.f_code is M.Runtime.command.__code__
                    and frame.f_lineno == target and not injected):
                self.assertIs(runtime.first_failure, primary)
                self.assertFalse(runtime.cleaning)
                self.assertEqual(len(children), 1)
                self.assertIsNone(children[0].poll())
                injected.append(children[0].pid)
                M.os.kill(M.os.getpid(), M.signal.SIGTERM)
            return trace

        old_handler = M.signal.signal(M.signal.SIGTERM, runtime.interrupted)
        old_trace = sys.gettrace()
        try:
            with mock.patch.object(M.subprocess, 'Popen', side_effect=spawn):
                sys.settrace(trace)
                with self.assertRaises(RuntimeError) as caught:
                    runtime.command('offline-actual-child-only',
                                    [sys.executable, '-I', '-B', '-c',
                                     'import time; print("synthetic-child",flush=True); time.sleep(2)'],
                                    timeout=5)
                self.assertIs(caught.exception, primary)
            self.assertEqual(len(injected), 1)
            self.assertEqual(len(runtime.calls), 1)
            call = runtime.calls[0]
            self.assertEqual(call['PID'], injected[0])
            self.assertTrue(call['direct_wait'] and call['EOF'] and call['PG_ESRCH'])
            self.assertEqual(call['failure_class'], 'RuntimeError')
            with self.assertRaises(ProcessLookupError):
                M.os.killpg(injected[0], 0)
            print(__import__('json').dumps({'registry_actual_fakechild_regression': call,
                  'actual_signal': 'SIGTERM at first command-cleanup entry',
                  'same_primary_exception_preserved': True,
                  'secondary_harness_needed': False}))
        finally:
            sys.settrace(old_trace)
            M.signal.signal(M.signal.SIGTERM, old_handler)
            for child in children:
                if child.poll() is None:
                    try:
                        M.os.killpg(child.pid, M.signal.SIGKILL)
                    except ProcessLookupError:
                        pass
                child.wait(timeout=1)
                for stream in (child.stdout, child.stderr):
                    if stream is not None and not stream.closed:
                        stream.close()

    def test_signal_between_primary_raise_and_command_or_main_capture_keeps_primary(self):
        import inspect
        import sys
        import tempfile
        temporary = tempfile.TemporaryDirectory(prefix='registry-error-capture-mock-')
        self.addCleanup(temporary.cleanup)
        observations = []

        def capture_line(function, text):
            lines, start = inspect.getsourcelines(function)
            return start + next(i for i, line in enumerate(lines) if line.strip() == text)

        def tracer(function, target, runtime, primary, phase):
            def trace(frame, event, _arg):
                if (event == 'line' and frame.f_code is function.__code__
                        and frame.f_lineno == target
                        and not any(row['phase'] == phase for row in observations)):
                    current = runtime()
                    self.assertIsNone(current.first_failure)
                    current.interrupted(M.signal.SIGTERM, None)
                    self.assertIsNone(current.first_failure)
                    self.assertEqual(current.pending_signal, M.signal.SIGTERM)
                    observations.append({'phase': phase, 'injected_before_primary_capture': True})
                return trace
            return trace

        runtime = M.Runtime(Path(temporary.name) / 'command-out')
        primary = RuntimeError('SYNTHETIC_COMMAND_PRIMARY')
        checks = []

        def checkpoint():
            checks.append(True)
            if len(checks) == 2:
                raise primary

        runtime.checkpoint = checkpoint
        pipe = types.SimpleNamespace(fileno=lambda: 123, close=lambda: None)
        process = types.SimpleNamespace(pid=123456789, returncode=0, stdout=pipe, stderr=pipe,
                                        poll=lambda: 0, wait=lambda timeout: 0)
        selector = types.SimpleNamespace(register=lambda *a: None, get_map=lambda: {}, close=lambda: None)
        target = capture_line(M.Runtime.command, 'failure = self.first_failure or error')
        old_trace = sys.gettrace()
        try:
            with mock.patch.object(M.subprocess, 'Popen', return_value=process), \
                    mock.patch.object(M.selectors, 'DefaultSelector', return_value=selector), \
                    mock.patch.object(M.signal, 'pthread_sigmask', return_value=set()), \
                    mock.patch.object(M.os, 'set_blocking'), \
                    mock.patch.object(M.os, 'killpg', side_effect=ProcessLookupError):
                sys.settrace(tracer(M.Runtime.command, target, lambda: runtime, primary, 'command'))
                with self.assertRaises(RuntimeError) as caught:
                    runtime.command('offline-mocked-command', ['never-executed'])
                self.assertIs(caught.exception, primary)
        finally:
            sys.settrace(old_trace)
        self.assertIs(runtime.first_failure, primary)
        self.assertEqual(runtime.calls[0]['failure_class'], 'RuntimeError')
        self.assertTrue(runtime.calls[0]['direct_wait'] and runtime.calls[0]['EOF']
                        and runtime.calls[0]['PG_ESRCH'])

        main_primary = RuntimeError('SYNTHETIC_MAIN_PRIMARY')
        instances = []
        actual_runtime = M.Runtime

        def make_runtime(*args, **kwargs):
            current = actual_runtime(*args, **kwargs)
            instances.append(current)
            return current

        out = Path(temporary.name) / 'main-out'
        target = capture_line(M.main, 'failure = runtime.first_failure or error')
        try:
            with mock.patch.dict(M.os.environ, self.context(), clear=True), \
                    mock.patch.object(sys, 'argv',
                                      ['registry-test', '--out', str(out), '--validate-only']), \
                    mock.patch.object(M.platform, 'system', return_value='Linux'), \
                    mock.patch.object(M.platform, 'machine', return_value='x86_64'), \
                    mock.patch.object(M, 'Runtime', side_effect=make_runtime), \
                    mock.patch.object(M, 'public_preview', side_effect=main_primary), \
                    mock.patch.object(M, 'bootstrap') as bootstrap, \
                    mock.patch.object(M.signal, 'signal', return_value=None):
                sys.settrace(tracer(M.main, target, lambda: instances[0], main_primary, 'main'))
                with self.assertRaises(RuntimeError) as caught:
                    M.main()
                self.assertIs(caught.exception, main_primary)
                bootstrap.assert_not_called()
        finally:
            sys.settrace(old_trace)
        self.assertIs(instances[0].first_failure, main_primary)
        self.assertFalse(instances[0].home.exists())
        self.assertEqual(instances[0].calls, [])
        first = M.load((out / 'proof/first-failure.json').read_bytes())
        self.assertEqual(first['class'], 'RuntimeError')
        self.assertEqual([row['phase'] for row in observations], ['command', 'main'])
        print(__import__('json').dumps({'registry_error_capture_regression': observations,
              'same_primary_exception_preserved_both': True,
              'main_private_HOME_removed': True, 'actual_new_children': 0}))



if __name__ == '__main__':
    unittest.main()
