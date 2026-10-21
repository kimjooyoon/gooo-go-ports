"""Bridge a job-private Docker login to the pinned attestation reader.

actions/attest@1e69f48 uses @sigstore/oci0.7.2, whose credential reader
uses os.homedir()/.docker/config.json and does not read DOCKER_CONFIG.
Keep this temporary copy and its restoration separate from publishing.
Never include the state directory or either credential file in artifacts.
"""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import tempfile


class AuthError(ValueError):
    pass


def require(value, code):
    if not value:
        raise AuthError(code)


def pairs(items):
    result = {}
    for key, value in items:
        require(key not in result, 'DUPLICATE_JSON_KEY')
        result[key] = value
    return result


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def directory(path):
    path = Path(path).absolute()
    require(path == path.resolve() and path.is_dir() and not path.is_symlink(), 'UNSAFE_DIRECTORY')
    require(path.stat().st_uid == os.getuid(), 'DIRECTORY_OWNER')
    return path


def regular(path, limit):
    require(not path.is_symlink(), 'SYMLINK_FILE')
    info = path.stat()
    require(stat.S_ISREG(info.st_mode) and info.st_uid == os.getuid(), 'FILE_OWNER_OR_TYPE')
    require(info.st_size <= limit, 'FILE_SIZE_LIMIT')
    return path.read_bytes(), stat.S_IMODE(info.st_mode)


def atomic(path, raw, mode=0o600):
    fd, name = tempfile.mkstemp(prefix='.gooo-auth-', dir=path.parent)
    try:
        with os.fdopen(fd, 'wb') as stream:
            os.fchmod(stream.fileno(), mode)
            stream.write(raw)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(name, path)
    finally:
        if os.path.exists(name):
            os.unlink(name)


def context(environ, state_dir):
    require(environ.get('GITHUB_ACTIONS') == 'true', 'GITHUB_ACTIONS_REQUIRED')
    require(environ.get('GITHUB_REPOSITORY') == 'kimjooyoon/gooo-go-ports', 'REPOSITORY_SCOPE')
    require(environ.get('GITHUB_REF') == 'refs/heads/main', 'MAIN_REF_REQUIRED')
    require(environ.get('GITHUB_RUN_ATTEMPT') == '1', 'FIRST_ATTEMPT_REQUIRED')
    run = environ.get('GITHUB_RUN_ID', '')
    require(run.isascii() and run.isdecimal() and int(run) > 0, 'RUN_ID_REQUIRED')
    root = directory(environ['RUNNER_TEMP'])
    state_dir = Path(state_dir).absolute()
    require(state_dir.parent == root and state_dir == state_dir.resolve(), 'STATE_OUTSIDE_RUNNER_TEMP')
    return state_dir, run


def stage(private_config, home, state_dir, environ):
    state_dir, run = context(environ, state_dir)
    home = directory(home)
    private_config = Path(private_config).absolute()
    directory(private_config.parent)
    require(private_config == private_config.resolve(), 'UNSAFE_PRIVATE_CONFIG')
    raw, mode = regular(private_config, 65536)
    require(mode & 0o077 == 0, 'PRIVATE_CONFIG_PERMISSIONS')
    doc = json.loads(raw, object_pairs_hook=pairs)
    require(isinstance(doc, dict) and set(doc) <= {'auths', 'HttpHeaders'}, 'PRIVATE_CONFIG_SHAPE')
    require(isinstance(doc.get('auths'), dict) and set(doc['auths']) == {'ghcr.io'}, 'REGISTRY_SCOPE')
    entry = doc['auths']['ghcr.io']
    require(isinstance(entry, dict) and set(entry) == {'auth'} and isinstance(entry['auth'], str), 'AUTH_SHAPE')
    try:
        username, password = base64.b64decode(entry['auth'], validate=True).decode().split(':', 1)
    except (ValueError, UnicodeError):
        raise AuthError('AUTH_ENCODING') from None
    require(username == environ.get('GITHUB_ACTOR') and bool(password), 'ACTOR_OR_PASSWORD')
    require('HttpHeaders' not in doc or isinstance(doc['HttpHeaders'], dict), 'HTTP_HEADERS_SHAPE')
    target_dir = home / '.docker'
    had_dir = target_dir.exists()
    if had_dir:
        directory(target_dir)
    else:
        require(not target_dir.is_symlink(), 'SYMLINK_DIRECTORY')
    target = target_dir / 'config.json'
    require(not target.is_symlink(), 'SYMLINK_TARGET')
    original, original_mode = regular(target, 1 << 20) if target.exists() else (None, None)
    require(not state_dir.exists(), 'STATE_ALREADY_EXISTS')
    state_dir.mkdir(mode=0o700)
    if original is not None:
        atomic(state_dir / 'original', original)
    state = dict(schema='gooo/action-registry-auth/v1', run=run, home=str(home),
                 original_exists=original is not None, original_mode=original_mode,
                 original_sha256=digest(original) if original is not None else None,
                 staged_sha256=digest(raw), docker_dir_existed=had_dir, status='prepared')
    atomic(state_dir / 'state.json', (json.dumps(state) + '\n').encode())
    if not had_dir:
        target_dir.mkdir(mode=0o700)
    directory(target_dir)
    atomic(target, raw)
    state['status'] = 'staged'
    atomic(state_dir / 'state.json', (json.dumps(state) + '\n').encode())
    return 'ACTION_REGISTRY_AUTH_STAGED'


def restore(home, state_dir, environ):
    state_dir, run = context(environ, state_dir)
    home = directory(home)
    directory(state_dir)
    state_raw, _ = regular(state_dir / 'state.json', 16384)
    state = json.loads(state_raw, object_pairs_hook=pairs)
    require(isinstance(state, dict) and set(state) == {'schema', 'run', 'home', 'original_exists', 'original_mode', 'original_sha256', 'staged_sha256', 'docker_dir_existed', 'status'}, 'STATE_SHAPE')
    require(state['schema'] == 'gooo/action-registry-auth/v1' and state['run'] == run and state['home'] == str(home), 'STATE_CONTEXT')
    require(type(state['original_exists']) is bool and type(state['docker_dir_existed']) is bool, 'STATE_TYPES')
    require(isinstance(state['staged_sha256'], str) and re.fullmatch('[0-9a-f]{64}', state['staged_sha256']), 'STATE_DIGEST')
    if state['original_exists']:
        require(type(state['original_mode']) is int and 0 <= state['original_mode'] <= 0o777, 'STATE_MODE')
        require(isinstance(state['original_sha256'], str) and re.fullmatch('[0-9a-f]{64}', state['original_sha256']), 'STATE_DIGEST')
    else:
        require(state['original_mode'] is None and state['original_sha256'] is None, 'STATE_ABSENT_FILE')
    require(state['status'] in {'prepared', 'staged', 'restored'}, 'STATE_PHASE')
    target_dir = home / '.docker'
    target = target_dir / 'config.json'
    require(not target.is_symlink() and not target_dir.is_symlink(), 'SYMLINK_TARGET')
    current, current_mode = regular(target, 1 << 20) if target.exists() else (None, None)
    already_original = ((current is None and not state['original_exists']) or
                        (current is not None and state['original_exists'] and digest(current) == state['original_sha256']))
    if not already_original:
        require(current is not None and digest(current) == state['staged_sha256'], 'CONFIG_CHANGED_CONCURRENTLY')
        if state['original_exists']:
            original, _ = regular(state_dir / 'original', 1 << 20)
            require(digest(original) == state['original_sha256'], 'BACKUP_CHANGED')
            atomic(target, original, state['original_mode'])
        else:
            target.unlink()
    elif state['original_exists'] and current_mode != state['original_mode']:
        require(state['status'] != 'restored' and current_mode == 0o600, 'CONFIG_MODE_CHANGED_CONCURRENTLY')
        atomic(target, current, state['original_mode'])
    if not state['docker_dir_existed'] and target_dir.exists():
        directory(target_dir)
        require(not list(target_dir.iterdir()), 'NEW_DOCKER_DIRECTORY_NOT_EMPTY')
        target_dir.rmdir()
    state['status'] = 'restored'
    atomic(state_dir / 'state.json', (json.dumps(state) + '\n').encode())
    backup = state_dir / 'original'
    if backup.exists():
        regular(backup, 1 << 20)
        backup.unlink()
    return 'ACTION_REGISTRY_AUTH_RESTORED'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('mode', choices=['stage', 'restore'])
    parser.add_argument('--home', required=True)
    parser.add_argument('--state-dir', required=True)
    parser.add_argument('--private-config')
    args = parser.parse_args()
    try:
        if args.mode == 'stage':
            require(bool(args.private_config), 'PRIVATE_CONFIG_REQUIRED')
            result = stage(args.private_config, args.home, args.state_dir, os.environ)
        else:
            result = restore(args.home, args.state_dir, os.environ)
        print(json.dumps({'status': result}))
    except AuthError as error:
        print(json.dumps({'status': 'ACTION_REGISTRY_AUTH_FAILED', 'code': str(error)}))
        raise SystemExit(1)
    except (OSError, ValueError, KeyError, TypeError):
        # Neither parsed credentials nor raw exception objects belong in logs.
        print(json.dumps({'status': 'ACTION_REGISTRY_AUTH_FAILED'}))
        raise SystemExit(1)


if __name__ == '__main__':
    main()
