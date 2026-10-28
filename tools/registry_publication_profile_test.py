#!/usr/bin/env python3
# Copyright (c) 2026 Gooo Go Ports contributors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""Version separation and refusal boundaries for fixed Registry publications."""
import copy
import importlib.util
import json
import os
from pathlib import Path
import tempfile
import types
import unittest
from unittest import mock

ROOT = Path(__file__).resolve().parent.parent
SPEC = importlib.util.spec_from_file_location('profile_publisher', ROOT / 'tools/publish-service-grant-registry.py')
M = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(M)


class ProfileTests(unittest.TestCase):
    def setUp(self):
        self.profile = M.publication_profile('v0.1.0-preview.17')
        self.metadata = M.load((ROOT / 'registry/service-grant-preview17.server.json').read_bytes())

    def test_default_preserves_frozen_legacy_identities(self):
        self.assertIsNone(M.publication_profile('v0.1.0-preview.16'))
        self.assertEqual(M.VERSION, '0.1.0-preview.16')
        self.assertEqual(M.OCI_DIGEST, 'sha256:a83d8fa57657bb877453cce7ed11f1ff1000a84b1e80c65a4e1c0b20196d3ee4')
        self.assertEqual(M.OCI_RECOVERY['run_id'], 39687579594)
        self.assertNotEqual(self.profile['SOURCE'], self.profile['PUBLICATION']['source'])

    def test_unknown_or_edited_profile_is_refused(self):
        with mock.patch.object(Path, 'read_bytes') as read, self.assertRaises(M.Refused):
            M.publication_profile('latest')
        read.assert_not_called()
        raw = (ROOT / 'tools/profiles/mcp-registry-preview17.json').read_bytes()
        with mock.patch.object(Path, 'read_bytes', return_value=raw[:-1] + b' '), self.assertRaises(M.Refused):
            M.publication_profile('v0.1.0-preview.17')

    def test_metadata_bytes_and_binary_argument_role_are_fixed(self):
        raw = (ROOT / 'registry/service-grant-preview17.server.json').read_bytes()
        self.assertEqual((len(raw), M.sha(raw)), (self.profile['METADATA_BYTES'], self.profile['METADATA_SHA']))
        M.check_metadata(self.metadata, self.profile)
        package = self.metadata['packages'][0]
        self.assertEqual(package['packageArguments'], [{'type': 'named', 'name': '--definition-snapshot'}])
        self.assertNotIn('--definition-snapshot', [arg['name'] for arg in package['runtimeArguments']])
        variants = []
        for alteration in ('no-task', 'docker-task', 'tag', 'environment', 'unbounded-memory'):
            value = copy.deepcopy(self.metadata)
            p = value['packages'][0]
            if alteration == 'no-task': p.pop('packageArguments')
            elif alteration == 'docker-task': p['runtimeArguments'] += p.pop('packageArguments')
            elif alteration == 'tag': p['identifier'] = self.profile['OCI_IMAGE'] + ':latest'
            elif alteration == 'environment': p['environmentVariables'] = [{'name': 'TOKEN'}]
            else: p['runtimeArguments'] = [a for a in p['runtimeArguments'] if a['name'] != '--memory']
            variants.append(value)
        for value in variants:
            with self.subTest(package=value['packages'][0]), self.assertRaises(M.Refused):
                M.check_metadata(value, self.profile)

    def test_native_and_saved_reference_assets_have_distinct_roles(self):
        p = self.profile
        self.assertEqual(len(p['ASSETS']), 6)
        self.assertEqual(len(p['BUNDLE_IDENTITIES']), 2)
        self.assertNotIn('gooo-mcp-saved-reference-linux-amd64.tar.gz', p['BUNDLE_IDENTITIES'])
        total = sum(row[0] for row in p['ASSETS'].values()) + M.PUBLISHER_BYTES
        self.assertLess(total + 4 * 65536 + 131072, M.HTTP_CAP)

    def test_six_asset_release_uses_selected_tag_source_and_download_urls(self):
        p = self.profile
        base = 'https://github.com/kimjooyoon/gooo-go-ports/releases/'
        release = {'id': p['RELEASE_ID'], 'draft': False, 'prerelease': True,
                   'published_at': '2026-10-28T01:51:15Z', 'tag_name': 'v0.1.0-preview.17',
                   'target_commitish': p['SOURCE'], 'html_url': base + 'tag/v0.1.0-preview.17',
                   'assets': [{'id': p['ASSET_IDS'][name], 'name': name, 'state': 'uploaded',
                               'size': size, 'digest': 'sha256:' + digest,
                               'browser_download_url': base + 'download/v0.1.0-preview.17/' + name}
                              for name, (size, digest) in p['ASSETS'].items()]}
        M.release_assets(release, p)
        for change in ('release-page', 'asset-page', 'boolean-id', 'missing-reference'):
            altered = copy.deepcopy(release)
            if change == 'release-page': altered['html_url'] = base + 'tag/v0.1.0-preview.16'
            elif change == 'asset-page': altered['assets'][0]['browser_download_url'] = base + 'download/v0.1.0-preview.16/x'
            elif change == 'boolean-id': altered['assets'][0]['id'] = True
            else: altered['assets'].pop()
            with self.subTest(change=change), self.assertRaises(M.Refused):
                M.release_assets(altered, p)

    def fixture(self):
        publication = self.profile['PUBLICATION']
        run = {'id': publication['run_id'], 'head_sha': publication['source'], 'run_attempt': 1,
               'event': 'workflow_dispatch', 'head_branch': 'main', 'status': 'completed',
               'conclusion': 'success', 'path': publication['workflow']}
        steps = {
            'verify': ['Verify runtime'],
            'publish': ['Push exactly once under a unique run tag and capture the manifest digest',
                        'Attest the actual pushed image provenance',
                        "Attest the same digest's retained CycloneDX SBOM"],
            'public-readback': ['Run the existing full-proof and refusal smoke on that pulled image',
                                'Record finite public readback acceptance only after cleanup succeeds'],
        }
        jobs = {'total_count': 4, 'jobs': [
            {'name': name, 'status': 'completed', 'conclusion': 'success',
             'steps': [{'name': step, 'status': 'completed', 'conclusion': 'success'} for step in values]}
            for name, values in steps.items()] + [
            {'name': 'registry-install-candidate', 'status': 'completed', 'conclusion': 'skipped', 'steps': []}]}
        temporary = tempfile.TemporaryDirectory(dir=ROOT / 'tools')
        self.addCleanup(temporary.cleanup)
        calls = []
        def public_json(label, url):
            calls.append(url)
            return copy.deepcopy(jobs if label.endswith('-jobs') else run)
        runtime = types.SimpleNamespace(proof=Path(temporary.name), public_json=public_json)
        return runtime, run, jobs, calls

    def test_exact_original_publication_is_not_legacy_recovery(self):
        runtime, _, _, calls = self.fixture()
        M.require_publication(runtime, self.profile)
        self.assertEqual(len(calls), 2)
        self.assertTrue(calls[0].endswith('/40632109714/attempts/1'))
        self.assertTrue(calls[1].endswith('/attempts/1/jobs?per_page=100'))
        proof = M.load((runtime.proof / 'reviewed-OCI-publication.json').read_bytes())
        self.assertEqual(proof['native_source'], self.profile['SOURCE'])
        self.assertEqual(proof['assembly_source'], self.profile['PUBLICATION']['source'])
        self.assertFalse((runtime.proof / 'reviewed-OCI-recovery.json').exists())

    def test_foreign_attempt_context_or_incomplete_publication_is_refused(self):
        for key, value in [('id', True), ('head_sha', self.profile['SOURCE']), ('run_attempt', 2),
                           ('event', 'pull_request'), ('conclusion', 'failure'),
                           ('status', 'in_progress'), ('path', '.github/workflows/service-grant-oci-proof-recovery.yml')]:
            runtime, run, _, _ = self.fixture()
            run[key] = value
            with self.subTest(key=key), self.assertRaises(M.Refused):
                M.require_publication(runtime, self.profile)

    def test_missing_attestation_or_cleanup_and_changed_job_coverage_are_refused(self):
        for change in ('missing-job', 'duplicate', 'attestation', 'cleanup', 'failed', 'skipped-verify', 'candidate-success'):
            runtime, _, jobs, _ = self.fixture()
            if change == 'missing-job': jobs['jobs'].pop()
            elif change == 'duplicate': jobs['jobs'][2] = jobs['jobs'][0]
            elif change == 'attestation': jobs['jobs'][1]['steps'].pop()
            elif change == 'cleanup': jobs['jobs'][2]['steps'][-1]['conclusion'] = 'failure'
            elif change == 'failed': jobs['jobs'][1]['conclusion'] = 'failure'
            elif change == 'skipped-verify': jobs['jobs'][0]['conclusion'] = 'skipped'
            else: jobs['jobs'][-1]['conclusion'] = 'success'
            with self.subTest(change=change), self.assertRaises(M.Refused):
                M.require_publication(runtime, self.profile)

    def test_timeout_budget_is_scoped_to_selected_registry_version(self):
        base = ('https://registry.modelcontextprotocol.io/v0.1/servers/'
                'io.github.kimjooyoon%2Fgooo-service-grant/versions/')
        self.assertEqual(M.read_timeout(base + '0.1.0-preview.17'), 20)
        self.assertEqual(M.read_timeout(base + '0.1.0-preview.17', self.profile['VERSION']), 45)
        self.assertEqual(M.read_timeout(base + '0.1.0-preview.16', self.profile['VERSION']), 20)
        self.assertEqual(M.read_timeout(base + '0.1.0-preview.17?other=1', self.profile['VERSION']), 20)

    def run_flow(self, validate_only, fail_publication=False):
        _, original_run, original_jobs, _ = self.fixture()
        temporary = tempfile.TemporaryDirectory(dir=ROOT / 'tools')
        self.addCleanup(temporary.cleanup)
        out = Path(temporary.name) / 'flow'
        instances, trace = [], []
        actual = M.Runtime
        metadata = self.metadata
        class Runtime(actual):
            def __init__(self, *args, **kwargs):
                super().__init__(*args, **kwargs)
                instances.append(self)
            def public_json(self, label, _url):
                trace.append(label)
                if fail_publication:
                    raise M.Refused('original publication unavailable')
                return copy.deepcopy(original_jobs if label.endswith('-jobs') else original_run)
            def registry_version(self, label, allow_missing=False):
                trace.append(label)
                return None if allow_missing else copy.deepcopy(metadata)
            def command(self, label, argv, authenticated=False, **_kwargs):
                trace.append(label)
                self.calls.append({'label': label, 'authenticated': authenticated, 'argv': argv,
                                   'exit_code': 0, 'direct_wait': True, 'EOF': True, 'PG_ESRCH': True})
                return b'mcp-publisher v1.8.1\n' if label == 'publisher-version' else b''
        environment = {'GITHUB_ACTIONS': 'true', 'GITHUB_REPOSITORY': M.REPOSITORY,
                       'GITHUB_EVENT_NAME': 'pull_request' if validate_only else 'workflow_dispatch',
                       'GITHUB_REF': 'refs/pull/9/merge' if validate_only else 'refs/heads/main',
                       'GITHUB_RUN_ATTEMPT': '1'}
        argv = ['registry-test', '--out', str(out), '--release-version', 'v0.1.0-preview.17',
                '--validate-only' if validate_only else '--allow-publish']
        def preview(runtime, profile):
            self.assertEqual(profile['SOURCE'], self.profile['SOURCE'])
            trace.append('native-release-byte-gate')
        error = None
        with mock.patch.dict(os.environ, environment, clear=True), \
                mock.patch.object(__import__('sys'), 'argv', argv), \
                mock.patch.object(M.platform, 'system', return_value='Linux'), \
                mock.patch.object(M.platform, 'machine', return_value='x86_64'), \
                mock.patch.object(M, 'Runtime', Runtime), \
                mock.patch.object(M, 'public_preview', side_effect=preview), \
                mock.patch.object(M, 'bootstrap', side_effect=lambda r: r.home / 'mcp-publisher'), \
                mock.patch.object(M.signal, 'signal', return_value=None):
            try:
                M.main()
            except BaseException as caught:
                error = caught
        return out, instances[0], trace, error

    def test_readonly_17_checks_original_publication_without_authentication_or_registry_absence(self):
        out, runtime, trace, error = self.run_flow(True)
        self.assertIsNone(error)
        self.assertEqual(trace, ['native-release-byte-gate', 'reviewed-OCI-publication-run',
                                'reviewed-OCI-publication-jobs', 'publisher-version', 'publisher-validate'])
        self.assertFalse(any(c['authenticated'] for c in runtime.calls))
        self.assertFalse(runtime.home.exists())
        proof = M.load((out / 'proof/validation-only.json').read_bytes())
        self.assertEqual(proof['version'], '0.1.0-preview.17')

    def test_17_publishes_once_only_after_original_proof_and_absence_then_reads_back(self):
        out, runtime, trace, error = self.run_flow(False)
        self.assertIsNone(error)
        self.assertLess(trace.index('reviewed-OCI-publication-jobs'), trace.index('publisher-login'))
        self.assertLess(trace.index('registry-before'), trace.index('publisher-login'))
        self.assertEqual(trace.count('publisher-publish'), 1)
        self.assertEqual(trace[-1], 'registry-after')
        self.assertFalse(runtime.home.exists())
        proof = M.load((out / 'proof/registration.json').read_bytes())
        self.assertIsNone(proof['OCI_recovery'])
        self.assertEqual(proof['OCI_publication']['source'], self.profile['PUBLICATION']['source'])

    def test_failed_17_publication_proof_stops_before_official_cli_and_removes_private_home(self):
        out, runtime, trace, error = self.run_flow(False, fail_publication=True)
        self.assertIsInstance(error, M.Refused)
        self.assertEqual(runtime.calls, [])
        self.assertNotIn('publisher-login', trace)
        self.assertFalse(runtime.home.exists())
        proof = M.load((out / 'proof/first-failure.json').read_bytes())
        self.assertEqual(proof['stage'], 'reviewed-OCI-publication')

    def test_repeated_17_publish_attempt_is_refused_before_runtime_creation(self):
        environment = {'GITHUB_ACTIONS': 'true', 'GITHUB_REPOSITORY': M.REPOSITORY,
                       'GITHUB_EVENT_NAME': 'workflow_dispatch', 'GITHUB_REF': 'refs/heads/main',
                       'GITHUB_RUN_ATTEMPT': '2'}
        argv = ['registry-test', '--out', '/private/tmp/never-created-profile-test',
                '--release-version', 'v0.1.0-preview.17', '--allow-publish']
        with mock.patch.dict(os.environ, environment, clear=True), \
                mock.patch.object(__import__('sys'), 'argv', argv), \
                mock.patch.object(M.platform, 'system', return_value='Linux'), \
                mock.patch.object(M.platform, 'machine', return_value='x86_64'), \
                mock.patch.object(M, 'Runtime') as runtime, self.assertRaisesRegex(M.Refused, 'attempt one'):
            M.main()
        runtime.assert_not_called()

    def test_boolean_publication_attempt_is_refused_without_external_read(self):
        runtime, _, _, calls = self.fixture()
        profile = copy.deepcopy(self.profile)
        profile['PUBLICATION']['attempt'] = True
        with self.assertRaises(M.Refused):
            M.require_publication(runtime, profile)
        self.assertEqual(calls, [])


if __name__ == '__main__':
    unittest.main()
