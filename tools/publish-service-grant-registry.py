#!/usr/bin/env python3
# Copyright (c) 2026 Gooo Go Ports contributors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""Validate or publish one fixed custom-install metadata version after public checks.

No Go/compiler build, package-registry account, PAT, or automatic retry. Only
the official publisher handles the job's OIDC token and its private token file.
"""
import argparse
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import platform
import re
import selectors
import shutil
import signal
import stat
import struct
import subprocess
import tarfile
import threading
import time
import urllib.parse
import urllib.error
import urllib.request

REPOSITORY = 'kimjooyoon/gooo-go-ports'
SOURCE = 'c87d924747b894f1fdb0f74b513fc504129e4de0'
TREE = 'a2f0f9323c4888c1760ff7caf635ea6408f8a2a8'
TAG = 'v0.1.0-preview.13'
TAG_OBJECT = 'eb21972a361dfd419e2664fb3f313f78365f40f4'
RELEASE_ID = 416848648
NAME = 'io.github.kimjooyoon/gooo-service-grant'
VERSION = '0.1.0-preview.13'
METADATA_SHA = '891529d92ac784ed5068f33da8297402a77b5389a86b710d81e6192e02c97f6f'
REGISTRY = 'https://registry.modelcontextprotocol.io'
PUBLISHER_VERSION = 'v1.8.1'
PUBLISHER_RELEASE_ID = 366432047
PUBLISHER_ASSET_ID = 504323513
PUBLISHER_ARCHIVE = 'mcp-publisher_linux_amd64.tar.gz'
PUBLISHER_BYTES = 7339841
PUBLISHER_SHA = 'a06c9096dcb9727c13555b6be26c7effa707b01f06a4c561ba7a3635443cf2cc'
PUBLISHER_TAR_BYTES = 19786752
PUBLISHER_MEMBERS = {
    'LICENSE': (12227, 0o644, '0382b0057770ca05e9c350a50aa3b1c1fea84da0bc81d723bf00b9aa841be58a'),
    'README.md': (7804, 0o644, '26fb79e25dca63e70689f49968c090bd2a88e788b603f9bffe8b56309f64e98e'),
    'mcp-publisher': (19763362, 0o755, '5e39fe8b6fc3c8b01bed6f1de364b88e9dd10ccbef6d3d3b1a0caa46115bf869'),
}
ASSETS = {
    'gooo-service-grant-mcp-darwin-arm64.tar.gz':
        (6843318, 'f866d79df3876aad85cc6942325f38b9ba422937f00db500241a690e282f712f'),
    'gooo-service-grant-mcp-darwin-arm64.tar.gz.sha256':
        (109, 'a772d84f543a6f9da16df0b8aecef01654034dda7c8b6458282860c79e1f1fe4'),
    'gooo-service-grant-mcp-linux-amd64.tar.gz':
        (7093748, '982ae6910ee3a28c9ab7f1944a44d27d956bf7b9a09101bfaeebd290ff4ab32d'),
    'gooo-service-grant-mcp-linux-amd64.tar.gz.sha256':
        (108, 'e6722caebc2356e78d0a176805351f2834353b8ddef1feb4974eb42bce00fb23'),
}
HTTP_CAP = 32 * 1024 * 1024
STREAM_CAP = 65536
BINARY_CAP = 64 * 1024 * 1024


class Refused(RuntimeError):
    pass


class PublisherCancelled(BaseException):
    pass


def require(condition, message):
    if not condition:
        raise Refused(message)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def unique(pairs):
    value = {}
    for key, item in pairs:
        require(key not in value, 'duplicate JSON key')
        value[key] = item
    return value


def load(raw):
    def bad_constant(_):
        raise Refused('nonfinite JSON')
    return json.loads(raw, object_pairs_hook=unique, parse_constant=bad_constant)


def dump(path, value):
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False) + '\n')


def typed_subset(actual, expected):
    require(type(actual) is type(expected), 'public metadata type changed')
    if isinstance(expected, dict):
        for key, value in expected.items():
            require(key in actual, 'public metadata field missing')
            typed_subset(actual[key], value)
    elif isinstance(expected, list):
        require(len(actual) == len(expected), 'public metadata list changed')
        for left, right in zip(actual, expected):
            typed_subset(left, right)
    else:
        require(actual == expected, 'public metadata value changed')


def release_assets(release):
    require(type(release['id']) is int and release['id'] == RELEASE_ID,
            'wrong public preview release ID')
    require(release['draft'] is False and release['prerelease'] is True
            and bool(release['published_at']), 'preview is not a public prerelease')
    require(release['tag_name'] == TAG and release['target_commitish'] == SOURCE,
            'preview source or tag changed')
    require(release['html_url'] == f'https://github.com/{REPOSITORY}/releases/tag/{TAG}',
            'wrong public release page')
    rows = release['assets']
    require(type(rows) is list and len(rows) == len(ASSETS), 'wrong preview asset count')
    require(len({r['name'] for r in rows}) == len(rows), 'duplicate preview asset name')
    require({r['name'] for r in rows} == set(ASSETS), 'preview asset names changed')
    for row in rows:
        size, digest = ASSETS[row['name']]
        require(row['state'] == 'uploaded' and type(row['size']) is int
                and row['size'] == size and row['digest'] == 'sha256:' + digest,
                'preview asset size, state or digest changed')
        require(row['browser_download_url'] ==
                f'https://github.com/{REPOSITORY}/releases/download/{TAG}/{row["name"]}',
                'preview asset download URL changed')
    return rows


def publisher_tar_member(unpacked):
    """Validate all three pinned files; return only the one executable's bytes."""
    with tarfile.open(fileobj=io.BytesIO(unpacked), mode='r:') as archive:
        rows = archive.getmembers()
        require(len(rows) == len(PUBLISHER_MEMBERS), 'wrong publisher member count')
        require(len({m.name for m in rows}) == len(rows)
                and {m.name for m in rows} == set(PUBLISHER_MEMBERS),
                'publisher archive names or duplicate paths refused')
        require(all(m.type in (tarfile.REGTYPE, tarfile.AREGTYPE)
                    and not m.pax_headers and not m.sparse for m in rows),
                'publisher archive file type refused')
        binary = None
        for member in rows:
            size, mode, digest = PUBLISHER_MEMBERS[member.name]
            require(member.size == size and member.mode == mode,
                    'publisher member size or mode changed')
            with archive.extractfile(member) as stream:
                data = stream.read(size + 1)
            require(len(data) == size and sha(data) == digest,
                    'publisher member bytes changed')
            if member.name == 'mcp-publisher':
                binary = data
    require(binary is not None and binary[:6] == b'\x7fELF\x02\x01'
            and len(binary) >= 64 and struct.unpack_from('<H', binary, 18)[0] == 62,
            'publisher is not a Linux amd64 ELF')
    return binary


def publisher_member(raw):
    """Whole official hash first; no filesystem extraction or archive paths."""
    require(len(raw) == PUBLISHER_BYTES and sha(raw) == PUBLISHER_SHA,
            'official publisher archive hash changed')
    with gzip.GzipFile(fileobj=io.BytesIO(raw)) as stream:
        unpacked = stream.read(PUBLISHER_TAR_BYTES + 1)
        require(len(unpacked) == PUBLISHER_TAR_BYTES and stream.read(1) == b'',
                'publisher expanded archive length changed')
    return publisher_tar_member(unpacked)


def execution_mode(validate_only, allow_publish, environ):
    require(type(validate_only) is bool and type(allow_publish) is bool
            and validate_only != allow_publish, 'select exactly one execution mode')
    require(environ.get('GITHUB_ACTIONS') == 'true'
            and environ.get('GITHUB_REPOSITORY') == REPOSITORY,
            'only the owned Actions repository may execute')
    if validate_only:
        require(environ.get('GITHUB_EVENT_NAME') == 'pull_request'
                and re.fullmatch(r'refs/pull/[1-9][0-9]*/merge',
                                 environ.get('GITHUB_REF', '')) is not None,
                'validation-only requires the owned pull-request merge context')
        require(not any(environ.get(key) for key in
                        ('ACTIONS_ID_TOKEN_REQUEST_TOKEN', 'ACTIONS_ID_TOKEN_REQUEST_URL')),
                'validation-only must not have an OIDC request capability')
        return 'validate-only'
    require(environ.get('GITHUB_EVENT_NAME') == 'workflow_dispatch'
            and environ.get('GITHUB_REF') == 'refs/heads/main',
            'only an owned main-branch manual Actions job may publish')
    return 'publish'


class Runtime:
    pending_signal = None

    def __init__(self, out, validate_only=False):
        require(type(validate_only) is bool, 'execution mode must be explicit')
        self.validate_only = validate_only
        require(out.is_absolute() and out.resolve() == out and not out.exists(),
                'output must be a new canonical directory')
        self.out = out
        out.mkdir(mode=0o700)
        self.proof = out / 'proof'
        self.proof.mkdir(mode=0o700)
        self.home = out / 'private-home'
        self.home.mkdir(mode=0o700)
        self.started = time.monotonic()
        self.http = {'cap_bytes': HTTP_CAP, 'known_bytes': 0,
                     'unknown_reserved_bytes': 0, 'requests': []}
        self.calls = []
        self.stage = 'initialization'
        self.first_failure = None
        self.pending_signal = None
        self.cleaning = False
        self.handlers = {}

    def checkpoint(self):
        # Signals only latch a request. Raise at a protected safe point, never
        # asynchronously while entering command/private-HOME cleanup.
        if self.first_failure is not None:
            raise self.first_failure
        pending = self.pending_error()
        if pending is not None:
            raise pending
        limit = 480 if getattr(self, 'validate_only', False) else 600
        require(time.monotonic() - self.started <= limit, 'registry task deadline')

    def interrupted(self, number, _frame):
        # Do not overwrite an error raised just before its except handler.
        # No calls or asynchronous exceptions are allowed in this handler.
        if self.pending_signal is None:
            self.pending_signal = number

    def pending_error(self):
        if self.pending_signal is not None:
            return PublisherCancelled('publisher signal ' + str(self.pending_signal))
        return None

    def journal(self):
        dump(self.proof / 'http-ledger.json', self.http)
        dump(self.proof / 'processes.json', self.calls)

    def fetch(self, label, url, cap, accepted_statuses=(200,)):
        self.checkpoint()
        require(self.http['known_bytes'] + self.http['unknown_reserved_bytes'] + cap + 1
                <= HTTP_CAP, 'HTTP body budget')
        self.http['unknown_reserved_bytes'] += cap + 1
        row = {'label': label, 'url': url, 'cap_bytes': cap, 'EOF': False}
        self.http['requests'].append(row)
        self.journal()

        class Redirect(urllib.request.HTTPRedirectHandler):
            def redirect_request(self, req, fp, code, msg, headers, newurl):
                parsed = urllib.parse.urlsplit(newurl)
                require(parsed.scheme == 'https' and parsed.hostname in {
                    'github.com', 'release-assets.githubusercontent.com',
                    'objects.githubusercontent.com'}, 'unexpected public redirect')
                return super().redirect_request(req, fp, code, msg, headers, newurl)

        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), Redirect())
        request = urllib.request.Request(url, headers={
            'User-Agent': 'Gooo-fixed-preview13-registry-publisher',
            'Accept': 'application/json' if '/api.' in url or
                      'registry.modelcontextprotocol.io' in url else 'application/octet-stream'})
        raw = bytearray()
        try:
            try:
                response = opener.open(request, timeout=20)
            except urllib.error.HTTPError as error:
                if error.code not in accepted_statuses:
                    raise
                response = error
            with response:
                row['status'] = response.status
                require(response.status in accepted_statuses, 'unexpected public endpoint status')
                stated = response.headers.get('Content-Length')
                if stated is not None:
                    require(stated.isdigit() and int(stated) <= cap, 'public content length cap')
                while True:
                    self.checkpoint()
                    chunk = response.read(min(65536, cap + 1 - len(raw)))
                    if not chunk:
                        row['EOF'] = True
                        break
                    raw.extend(chunk)
                    require(len(raw) <= cap, 'public response body cap')
                if stated is not None:
                    require(len(raw) == int(stated), 'public response truncated')
        except BaseException as error:
            if self.first_failure is None:
                self.first_failure = error
            if isinstance(error, urllib.error.HTTPError):
                row['status'] = error.code
                error.close()
            row.update({'received_prefix_bytes': len(raw), 'received_prefix_sha256': sha(raw),
                        'failure_class': type(error).__name__})
            # Keep the full reservation charged. Do not retain error bodies,
            # response headers or signed redirects in the public artifact.
            try:
                self.journal()
            except BaseException:
                pass
            raise
        self.http['unknown_reserved_bytes'] -= cap + 1
        self.http['known_bytes'] += len(raw)
        row.update({'bytes': len(raw), 'sha256': sha(raw)})
        self.journal()
        return bytes(raw)

    def public_json(self, label, url, cap=65536):
        raw = self.fetch(label, url, cap)
        value = load(raw)
        (self.proof / (label + '.json')).write_bytes(raw)
        return value

    def command(self, label, argv, authenticated=False, timeout=30):
        if getattr(self, 'validate_only', False):
            expected = [
                ('publisher-version', [str(self.home / 'mcp-publisher'), '--version']),
                ('publisher-validate', [str(self.home / 'mcp-publisher'), 'validate',
                                        str(self.home / 'server.json')]),
            ]
            require(not authenticated and len(self.calls) < len(expected),
                    'validation-only cannot authenticate or run additional commands')
            wanted_label, wanted_argv = expected[len(self.calls)]
            require(label == wanted_label and list(argv) == wanted_argv,
                    'validation-only command or order refused before spawn')
        require(threading.current_thread() is threading.main_thread()
                and threading.active_count() == 1, 'publisher requires one main thread')
        self.checkpoint()
        require(len(self.calls) < 4, 'publisher command count')
        # No ambient credential/config/proxy environment reaches the publisher.
        env = {'HOME': str(self.home), 'XDG_CONFIG_HOME': str(self.home / '.config'),
               'TMPDIR': str(self.home), 'PATH': '/usr/bin:/bin', 'LANG': 'C', 'LC_ALL': 'C',
               'GOTOOLCHAIN': 'local', 'GOENV': 'off', 'GOWORK': 'off',
               'GOPROXY': 'off', 'GOSUMDB': 'off', 'GOTELEMETRY': 'off'}
        if authenticated:
            for key in ('ACTIONS_ID_TOKEN_REQUEST_URL', 'ACTIONS_ID_TOKEN_REQUEST_TOKEN'):
                require(bool(os.environ.get(key)), 'GitHub OIDC request capability missing')
                env[key] = os.environ[key]
        public_argv = {
            'publisher-version': [str(self.home / 'mcp-publisher'), '--version'],
            'publisher-validate': [str(self.home / 'mcp-publisher'), 'validate',
                                   str(self.home / 'server.json')],
        }
        retain_public_output = (not authenticated and label in public_argv
                                and list(argv) == public_argv[label])
        process = None
        failure = None
        out, err = bytearray(), bytearray()
        selector = selectors.DefaultSelector()
        started = time.monotonic()
        eof = absent = False
        try:
            mask = signal.pthread_sigmask(signal.SIG_BLOCK, {signal.SIGINT, signal.SIGTERM})
            try:
                def restore_child_mask():
                    signal.pthread_sigmask(signal.SIG_SETMASK, mask)
                process = subprocess.Popen(argv, stdin=subprocess.DEVNULL,
                    stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=env,
                    cwd=self.home, start_new_session=True, preexec_fn=restore_child_mask)
                for stream, target in ((process.stdout, out), (process.stderr, err)):
                    os.set_blocking(stream.fileno(), False)
                    selector.register(stream, selectors.EVENT_READ, target)
            finally:
                signal.pthread_sigmask(signal.SIG_SETMASK, mask)
            self.checkpoint()
            while selector.get_map() or process.poll() is None:
                self.checkpoint()
                require(time.monotonic() - started <= timeout, 'publisher command deadline')
                for key, _ in selector.select(.1):
                    chunk = os.read(key.fd, min(65536, STREAM_CAP + 1 - len(key.data)))
                    if not chunk:
                        selector.unregister(key.fileobj)
                    else:
                        key.data.extend(chunk)
                        require(len(key.data) <= STREAM_CAP, 'publisher command stream cap')
            eof = True
            process.wait(timeout=1)
            require(process.returncode == 0, 'publisher command failed: ' + label)
            self.checkpoint()
        except BaseException as error:
            failure = self.first_failure or error
            if self.first_failure is None:
                self.first_failure = failure
        finally:
            self.cleaning = True
            try:
                if process is not None:
                    for number in (signal.SIGTERM, signal.SIGKILL):
                        try:
                            os.killpg(process.pid, number)
                        except ProcessLookupError:
                            break
                        try:
                            process.wait(timeout=1)
                        except subprocess.TimeoutExpired:
                            pass
                    process.wait(timeout=1)
                    until = time.monotonic() + 1
                    while selector.get_map() and time.monotonic() < until:
                        for key, _ in selector.select(.02):
                            chunk = os.read(key.fd, 65536)
                            if not chunk:
                                selector.unregister(key.fileobj)
                            else:
                                key.data.extend(chunk[:max(0, STREAM_CAP + 1 - len(key.data))])
                    eof = not selector.get_map()
                    for stream in (process.stdout, process.stderr):
                        stream.close()
                    try:
                        os.killpg(process.pid, 0)
                    except ProcessLookupError:
                        absent = True
            except BaseException as error:
                failure = failure or self.first_failure or error
            finally:
                selector.close()
                self.cleaning = False
            failure = failure or self.first_failure
            public_output = None
            if retain_public_output:
                try:
                    folder = self.proof / 'public-cli'
                    folder.mkdir(mode=0o700, exist_ok=True)
                    public_output = {}
                    for name, data in (('stdout', out), ('stderr', err)):
                        path = folder / (label + '.' + name)
                        with path.open('xb') as output:
                            output.write(data)
                        public_output[name] = {'path': str(path.relative_to(self.proof)),
                                               'bytes': len(data), 'sha256': sha(data)}
                except BaseException as error:
                    failure = failure or self.first_failure or error
            failure = failure or self.first_failure or self.pending_error()
            self.calls.append({'label': label, 'authenticated': authenticated,
                'public_output': public_output,
                'PID': None if process is None else process.pid,
                'exit_code': None if process is None else process.returncode,
                'direct_wait': process is not None and process.returncode is not None,
                'EOF': eof, 'PG_ESRCH': absent, 'stdout_bytes': len(out),
                'stdout_sha256': sha(out), 'stderr_bytes': len(err), 'stderr_sha256': sha(err),
                'elapsed_seconds': time.monotonic() - started,
                'failure_class': None if failure is None else type(failure).__name__})
            try:
                self.journal()
                require(eof and absent, 'publisher process closure incomplete')
            except BaseException as error:
                failure = failure or self.first_failure or error
        failure = failure or self.first_failure or self.pending_error()
        if failure is not None:
            if self.first_failure is None:
                self.first_failure = failure
            raise failure
        # Authentication/publication raw is never retained. Only the two exact
        # anonymous version/validate argv may write bounded public-cli logs.
        return bytes(out + err)

    def registry_version(self, label, allow_missing=False):
        url = (REGISTRY + '/v0.1/servers/' + urllib.parse.quote(NAME, safe='')
               + '/versions/' + urllib.parse.quote(VERSION, safe=''))
        # Deleted versions also block republication of the same immutable version.
        if allow_missing:
            url += '?include_deleted=true'
        raw = self.fetch(label, url, 262144, (200, 404) if allow_missing else (200,))
        (self.proof / (label + '.json')).write_bytes(raw)
        if self.http['requests'][-1]['status'] == 404:
            return None
        response = load(raw)
        require(type(response.get('server')) is dict, 'registry response format')
        require(response['server']['name'] == NAME and response['server']['version'] == VERSION,
                'registry returned a different server version')
        return response['server']


def public_preview(runtime):
    api = 'https://api.github.com/repos/' + REPOSITORY
    release = runtime.public_json('public-release13', api + '/releases/' + str(RELEASE_ID))
    rows = release_assets(release)  # A draft/404 stops before download or OIDC.
    tag = runtime.public_json('tag-ref', api + '/git/ref/tags/' + TAG)
    require(tag['object']['type'] == 'tag' and tag['object']['sha'] == TAG_OBJECT,
            'annotated tag object changed')
    tag = runtime.public_json('tag-object', api + '/git/tags/' + TAG_OBJECT)
    require(tag['tag'] == TAG and tag['object']['type'] == 'commit'
            and tag['object']['sha'] == SOURCE, 'tag no longer points to fixed source')
    commit = runtime.public_json('release-source', api + '/git/commits/' + SOURCE)
    require(commit['sha'] == SOURCE and commit['tree']['sha'] == TREE, 'source tree changed')
    checksum_lines = {}
    for row in rows:
        size, digest = ASSETS[row['name']]
        body = runtime.fetch('asset-' + row['name'], row['browser_download_url'], size)
        require(len(body) == size and sha(body) == digest, 'public preview asset bytes changed')
        if row['name'].endswith('.sha256'):
            checksum_lines[row['name']] = body
    for name, (_, digest) in ASSETS.items():
        if not name.endswith('.sha256'):
            require(checksum_lines[name + '.sha256'] == (digest + '  ' + name + '\n').encode(),
                    'public checksum file disagrees')
    dump(runtime.proof / 'public-release-byte-gate.json', {
        'status': 'FOUR_PUBLIC_ASSET_BYTES_MATCH', 'release_id': RELEASE_ID,
        'source': SOURCE, 'tree': TREE, 'tag': TAG, 'assets': ASSETS,
        'new_MCP_or_Gooo_binary_execution': False})


def bootstrap(runtime):
    api = 'https://api.github.com/repos/modelcontextprotocol/registry/releases/'
    release = runtime.public_json('publisher-release', api + str(PUBLISHER_RELEASE_ID), 131072)
    require(release['id'] == PUBLISHER_RELEASE_ID and release['tag_name'] == PUBLISHER_VERSION
            and release['draft'] is False and release['prerelease'] is False,
            'official publisher release changed')
    rows = [r for r in release['assets'] if r['id'] == PUBLISHER_ASSET_ID]
    require(len(rows) == 1, 'official publisher asset missing')
    row = rows[0]
    url = ('https://github.com/modelcontextprotocol/registry/releases/download/'
           + PUBLISHER_VERSION + '/' + PUBLISHER_ARCHIVE)
    require(row['name'] == PUBLISHER_ARCHIVE and row['state'] == 'uploaded'
            and row['size'] == PUBLISHER_BYTES and row['digest'] == 'sha256:' + PUBLISHER_SHA
            and row['browser_download_url'] == url, 'official publisher asset changed')
    body = runtime.fetch('publisher-archive', url, PUBLISHER_BYTES)
    require(len(body) == PUBLISHER_BYTES and sha(body) == PUBLISHER_SHA,
            'official publisher archive hash changed')
    binary = publisher_member(body)
    executable = runtime.home / 'mcp-publisher'
    with executable.open('xb') as output:
        output.write(binary)
    executable.chmod(0o700)
    dump(runtime.proof / 'publisher-bootstrap.json', {
        'version': PUBLISHER_VERSION, 'release_id': PUBLISHER_RELEASE_ID,
        'asset_id': PUBLISHER_ASSET_ID, 'archive_bytes': len(body),
        'archive_sha256': sha(body), 'regular_members': 3,
        'pinned_members': PUBLISHER_MEMBERS, 'extracted_executables': 1,
        'binary_bytes': len(binary), 'binary_sha256': sha(binary),
        'binary_identity_bound_by_exact_official_archive': True})
    return executable


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', required=True, type=Path)
    modes = parser.add_mutually_exclusive_group(required=True)
    modes.add_argument('--allow-publish', action='store_true')
    modes.add_argument('--validate-only', action='store_true')
    args = parser.parse_args()
    mode = execution_mode(args.validate_only, args.allow_publish, os.environ)
    require(platform.system() == 'Linux' and platform.machine() == 'x86_64',
            'publisher requires Linux amd64')
    require(threading.current_thread() is threading.main_thread()
            and threading.active_count() == 1, 'single main thread required')
    metadata_path = Path(__file__).resolve().parent.parent / 'registry/service-grant-preview13.server.json'
    require(stat.S_ISREG(metadata_path.lstat().st_mode), 'metadata must be a regular file')
    metadata_raw = metadata_path.read_bytes()
    require(len(metadata_raw) == 581 and sha(metadata_raw) == METADATA_SHA, 'metadata bytes changed')
    metadata = load(metadata_raw)
    require(metadata['name'] == NAME and metadata['version'] == VERSION
            and 'packages' not in metadata and 'remotes' not in metadata,
            'fixed custom-install metadata changed')
    runtime = Runtime(args.out, validate_only=args.validate_only)
    failure = None
    try:
        for number in (signal.SIGTERM, signal.SIGINT):
            runtime.handlers[number] = signal.signal(number, runtime.interrupted)
        with (runtime.out / 'EXECUTION-STARTED.json').open('x') as output:
            json.dump({'fixed_name': NAME, 'version': VERSION, 'source': SOURCE,
                       'mode': mode}, output)
        runtime.stage = 'public-preview-byte-gate'
        public_preview(runtime)
        if not args.validate_only:
            runtime.stage = 'registry-version-absence'
            require(runtime.registry_version('registry-before', allow_missing=True) is None,
                    'this version is already registered')
        runtime.stage = 'official-publisher-bootstrap'
        executable = bootstrap(runtime)
        server_file = runtime.home / 'server.json'
        server_file.write_bytes(metadata_raw)
        version = runtime.command('publisher-version', [str(executable), '--version'])
        require(re.search(rb'mcp-publisher v?1\.8\.1(?:\s|$)', version) is not None,
                'publisher version output changed')
        runtime.stage = 'official-registry-validation'
        runtime.command('publisher-validate', [str(executable), 'validate', str(server_file)])
        if not args.validate_only:
            runtime.stage = 'GitHub-OIDC-login'
            runtime.command('publisher-login', [str(executable), 'login', 'github-oidc',
                            '--registry', REGISTRY], authenticated=True)
            runtime.stage = 'one-publish-call'
            runtime.command('publisher-publish', [str(executable), 'publish', str(server_file)],
                            authenticated=True, timeout=60)
            runtime.stage = 'anonymous-registry-readback'
            server = runtime.registry_version('registry-after')
            typed_subset(server, metadata)
            require(not server.get('packages') and not server.get('remotes'),
                    'registry returned unrequested installation endpoints')
            dump(runtime.proof / 'registration.json', {
                'status': 'OFFICIAL_REGISTRY_EXACT_VERSION_ANONYMOUSLY_READ_BACK',
                'name': NAME, 'version': VERSION, 'metadata_sha256': METADATA_SHA,
                'source': SOURCE, 'release_id': RELEASE_ID, 'custom_install_only': True,
                'automatic_client_installation_proven': False, 'external_adoption': False})
    except BaseException as error:
        failure = runtime.first_failure or error
        if runtime.first_failure is None:
            runtime.first_failure = failure
    finally:
        runtime.cleaning = True
        try:
            # Only this newly created private home; no user credential directory.
            require(runtime.home.parent == runtime.out and runtime.home.resolve() == runtime.home
                    and not runtime.home.is_symlink(), 'private home cleanup path changed')
            shutil.rmtree(runtime.home)
            dump(runtime.proof / 'private-home-cleanup.json', {'removed': not runtime.home.exists(),
                 'contains_publisher_token_files': True, 'token_contents_read_by_helper': False})
            runtime.journal()
        except BaseException as error:
            failure = failure or runtime.first_failure or error
        finally:
            for number, handler in runtime.handlers.items():
                signal.signal(number, handler)
        failure = failure or runtime.first_failure or runtime.pending_error()
        if failure is not None:
            try:
                dump(runtime.proof / 'first-failure.json', {
                    'class': type(failure).__name__, 'stage': runtime.stage,
                    'automatic_retry': False, 'no_credential_or_external_error_text_retained': True})
            except BaseException:
                pass
    if failure is not None:
        raise failure
    if args.validate_only:
        require([call['label'] for call in runtime.calls] ==
                ['publisher-version', 'publisher-validate'],
                'validation-only must complete exactly its two permitted commands')
        require(all(call['authenticated'] is False and call['exit_code'] == 0
                    and call['direct_wait'] and call['EOF'] and call['PG_ESRCH']
                    for call in runtime.calls) and not runtime.home.exists(),
                'validation-only closure or private-home cleanup incomplete')
        dump(runtime.proof / 'validation-only.json', {
            'status': 'OFFICIAL_CLI_VALIDATED_FIXED_PUBLIC_METADATA_WITHOUT_AUTH_OR_PUBLISH',
            'mode': mode, 'name': NAME, 'version': VERSION,
            'metadata_sha256': METADATA_SHA, 'source': SOURCE, 'release_id': RELEASE_ID,
            'registry_version_absence_required': False, 'publisher_commands': 2,
            'authentication_commands': 0, 'publish_commands': 0,
            'actual_registry_registration': False, 'automatic_client_installation_proven': False,
            'external_adoption': False, 'private_home_removed': True,
            'all_direct_wait_EOF_PG_ESRCH': True})


if __name__ == '__main__':
    try:
        main()
    except BaseException as error:
        # Do not render arbitrary authentication response bodies or signed URLs.
        print(type(error).__name__ + ': registry publication stopped; no retry', file=__import__('sys').stderr)
        raise SystemExit(1)
