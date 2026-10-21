#!/usr/bin/env python3
"""Run one finite native/replay validation; never overwrite an evidence directory."""
import argparse
import copy
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import sys
import tempfile
import time


ROOT = Path(__file__).resolve().parent
MODULE = 'github.com/kimjooyoon/gooo-go-ports/examples/api-policy-revision'
PIN = '205896a413e5dea92366309c35ae85e542d055f6'
OLD_GENERATION = 'sha256:79b76bda704cb63b9390802766a66ac72ba02c04ae135600cb05de6797c97d46'
OLD_PROFILE = 'cel-api-original-v1'
NEW_PROFILE = 'cel-api-readonly-principal-v1'
FIELDS = (
    'allowed', 'reason', 'authenticated', 'tenant_present', 'tenant_matched',
    'action_present', 'action_matched', 'access_allowed', 'access_error', 'access_result',
    'group_matched', 'principal_matched', 'group_missing', 'principal_defaulted', 'effective_principal',
)
BINDING_TESTS = {
    'TestRevisionBindingsActualEmbeddedEvidence', 'TestRevisionBindingsOptionalErrorExtension',
    'TestRevisionBindingsPreserveEveryProgramField', 'TestRevisionBindingsRequireActualNativeCompletion',
    'TestRevisionBindingsRejectDifferentNativeProducer', 'TestRevisionBindingsRequireExactNativeProjection',
    'TestRevisionBindingsRequireClassifierInputRecordIdentity', 'TestRevisionBindingsKeepOriginalProfileFrozen',
}
MAX_CAPTURE_BYTES = 16 * 1024 * 1024


class VerificationError(Exception):
    pass


def need(condition, message):
    if not condition:
        raise VerificationError(message)


def digest(raw):
    return 'sha256:' + hashlib.sha256(raw).hexdigest()


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        need(key not in result, 'duplicate JSON output key')
        result[key] = value
    return result


def invalid_constant(value):
    raise VerificationError('non-finite JSON value: ' + value)


def decode(raw):
    try:
        return json.loads(raw.decode('utf-8'), object_pairs_hook=unique_object,
                          parse_constant=invalid_constant)
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise VerificationError('invalid UTF-8/JSON process output') from exc


def exact(actual, expected, where):
    need(type(actual) is type(expected), where + ': JSON type differs')
    if isinstance(expected, dict):
        need(actual.keys() == expected.keys(), where + ': keys differ')
        for key in expected:
            exact(actual[key], expected[key], where + '/' + key)
    elif isinstance(expected, list):
        need(len(actual) == len(expected), where + ': length differs')
        for index, (left, right) in enumerate(zip(actual, expected)):
            exact(left, right, where + '/' + str(index))
    else:
        need(actual == expected, where + ': value differs')


def private_write(directory, name, raw):
    target = directory / name
    with os.fdopen(os.open(target, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), 'wb') as output:
        output.write(raw)
    return {'path': str(target), 'bytes': len(raw), 'sha256': digest(raw)}


def check_bindings():
    old_root = ROOT / 'profiles/original'
    old_raw = (old_root / 'generation.json').read_bytes()
    old = decode(old_raw)
    need(old['schema'] == 'gooo/cel-public-generation/v1' and len(old['assets']) == 28,
         'original generation roster differs')
    need(digest(old_raw) == OLD_GENERATION, 'original generation bytes differ')
    profiles = decode((ROOT / 'profiles.json').read_bytes())
    original = profiles['original']
    need(original['source_commit'] == '8cddcc4e65afb42431f8f42a86f29859ae2f4d69',
         'original immutable source commit differs')
    need(original['module'] == 'github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy',
         'original module path differs')
    exact(original['assets'], old['assets'], 'original profile roster')
    need(original['generation_sha256'] == OLD_GENERATION, 'original profile generation differs')
    current_raw = (ROOT / 'generation.json').read_bytes()
    current = decode(current_raw)
    need(current['schema'] == 'gooo/api-policy-revision-generation/v1'
         and current['compiler_source_sha'] == PIN and len(current['assets']) == 38,
         'revision generation identity/roster differs')
    need(digest(current_raw) != OLD_GENERATION, 'candidate reused original generation')
    paths = {'generation.json': ROOT / 'generation.json',
             'profiles/original/generation.json': old_root / 'generation.json'}
    for prefix, directory, assets in (
        ('', ROOT, current['assets']),
        ('profiles/original/', old_root, {**old['assets'], **original['packaging_assets']}),
    ):
        for name, expected in assets.items():
            relative = Path(name)
            need(not relative.is_absolute() and '..' not in relative.parts, 'unsafe asset path')
            source = directory / relative
            need(source.is_file() and not source.is_symlink(), 'missing/nonregular source asset')
            need(digest(source.read_bytes()) == expected, 'asset changed: ' + prefix + name)
            paths[prefix + name] = source
    for name in ('verify.py', 'README.md', 'testdata/read-facts.json', 'testdata/write-facts.json'):
        paths[name] = ROOT / name
    for source in ROOT.rglob('*_test.go'):
        relative = source.relative_to(ROOT)
        if 'profiles' not in relative.parts:
            paths[str(relative)] = source
    for source in ROOT.rglob('*.json'):
        relative = source.relative_to(ROOT)
        if 'profiles' in relative.parts or 'testdata' not in relative.parts:
            continue
        need(source.is_file() and not source.is_symlink(), 'nonregular test fixture source')
        paths[relative.as_posix()] = source
    before = {name: digest(path.read_bytes()) for name, path in sorted(paths.items())}
    expected_old = {'compiler_source_sha': '83928fa4538245abe33b08549da47f8cbdc50ed2', 'assets': old['assets'], 'generation_sha256': OLD_GENERATION}
    expected_new = {'compiler_source_sha': PIN, 'assets': current['assets'], 'generation_sha256': digest(current_raw)}
    return paths, before, expected_old, expected_new


def literal_facts(action):
    return {
        'group_present': True, 'group_value': 'admin', 'principal_present': True,
        'principal_value': 'synthetic:user', 'authenticated': True,
        'caller_tenant': 'tenant-a', 'resource_tenant': 'tenant-a',
        'requested_action': action, 'allowed_action': action,
    }


def literal_decision():
    return {
        'allowed': True, 'reason': 'ALLOW', 'authenticated': True,
        'tenant_present': True, 'tenant_matched': True,
        'action_present': True, 'action_matched': True,
        'access_allowed': True, 'access_error': False, 'access_result': 'ALLOW',
        'group_matched': True, 'principal_matched': False, 'group_missing': False,
        'principal_defaulted': False, 'effective_principal': 'synthetic:user',
    }


def expected_report(action, original_bindings, revision_bindings):
    before, after = literal_decision(), literal_decision()
    flags = {field: False for field in FIELDS}
    summary = {'classification': 'UNCHANGED', 'changed_count': 0}
    if action == 'write':
        after['allowed'], after['reason'] = False, 'ACTION_NOT_READ_ONLY'
        flags['allowed'], flags['reason'] = True, True
        summary = {'classification': 'NEWLY_DENIED', 'changed_count': 2}
    return {
        'schema': 'gooo/api-policy-revision-comparison/v1', 'status': 'COMPARED',
        'scope': 'same_saved_facts_two_sealed_profiles',
        'before_profile': OLD_PROFILE, 'candidate_profile': NEW_PROFILE,
        'before': {'facts': literal_facts(action), 'decision': before, 'bindings': original_bindings},
        'after': {'facts': literal_facts(action), 'decision': after, 'bindings': revision_bindings},
        'differences': flags, 'summary': summary, 'classifier_bindings': revision_bindings,
    }


def required_tests():
    result = set()
    for source in ROOT.rglob('*_test.go'):
        relative = source.relative_to(ROOT)
        if 'profiles' in relative.parts:
            continue
        package = MODULE if relative.parent == Path('.') else MODULE + '/' + relative.parent.as_posix()
        for name in re.findall(r'^func (Test\w+)\(t \*testing\.T\)', source.read_text(), re.M):
            result.add((package, name))
    need({(MODULE, name) for name in BINDING_TESTS}.issubset(result), 'eight binding tests are not all present')
    need(any(package == MODULE + '/cmd/compare' for package, _ in result), 'CLI tests are absent')
    return result


def check_test_log(raw, declared):
    events = [decode(line) for line in raw.splitlines() if line.strip()]
    need(not any(event.get('Action') == 'fail' for event in events), 'Go test reported a failure')
    skipped = [event for event in events if event.get('Action') == 'skip' and event.get('Test')]
    need(not skipped, 'Go tests skipped a named test')
    passed = {(event.get('Package'), event.get('Test')) for event in events
              if event.get('Action') == 'pass' and event.get('Test')}
    need(declared.issubset(passed), 'a declared top-level Go test did not pass')
    packages = {event.get('Package') for event in events
                if event.get('Action') == 'pass' and not event.get('Test')}
    need({MODULE, MODULE + '/cmd/compare'}.issubset(packages), 'native library/CLI package did not pass')
    return {'declared_top_level_tests': len(declared), 'passed_top_level_tests': len(declared),
            'named_skips': 0, 'binding_tests_present_and_passed': 8,
            'native_and_cli_packages_passed': True}


def run(record, out, label, command, cwd, environment, data=b'', expected_exit=0, timeout=900):
    started = time.monotonic()
    timed_out = False
    process = subprocess.Popen([str(part) for part in command], cwd=cwd, env=environment,
                               stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                               start_new_session=(os.name == 'posix'))
    try:
        stdout, stderr = process.communicate(data, timeout=timeout)
    except subprocess.TimeoutExpired:
        timed_out = True
        if os.name == 'posix':
            os.killpg(process.pid, signal.SIGTERM)
        else:
            process.terminate()
        try:
            stdout, stderr = process.communicate(timeout=5)
        except subprocess.TimeoutExpired:
            if os.name == 'posix':
                os.killpg(process.pid, signal.SIGKILL)
            else:
                process.kill()
            stdout, stderr = process.communicate()
    need(len(stdout) <= MAX_CAPTURE_BYTES and len(stderr) <= MAX_CAPTURE_BYTES, 'process output exceeded evidence cap')
    row = {'label': label, 'command': [str(part) for part in command], 'cwd': str(cwd),
           'exit_code': process.returncode, 'expected_exit': expected_exit,
           'timed_out': timed_out, 'elapsed_seconds': time.monotonic() - started,
           'input_sha256': digest(data),
           'stdout': private_write(out, label + '.stdout', stdout),
           'stderr': private_write(out, label + '.stderr', stderr)}
    record['operations'].append(row)
    need(not timed_out and process.returncode == expected_exit, 'unexpected process outcome: ' + label)
    return stdout


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--compiler', type=Path, required=True)
    parser.add_argument('--go', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    out = args.out.expanduser().absolute()
    try:
        out.mkdir(mode=0o700, parents=True, exist_ok=False)
    except OSError as exc:
        print('Evidence directory must be new: ' + str(exc), file=sys.stderr)
        return 1
    started = time.monotonic()
    record = {'schema': 'gooo/api-policy-revision-verification/v1', 'status': 'FAILED',
              'scope': 'finite_two_profile_native_and_saved_replay', 'operations': [],
              'decision_fields': 15, 'fresh_process_pairs': 0, 'classifier_cases': 0,
              'altered_report_refused': False, 'original_generation_preserved': False,
              'first_user_time_measured': False, 'independent_adoption_observed': False,
              'setup_prerequisites': ['Python 3 standard library', 'installed Go 1.27.3 and race C toolchain',
                                      'existing exact Gooo compiler source ' + PIN,
                                      'previously downloaded exact original module with sealed Go checksums'],
              'limits': ['Synthetic readonly policy observations are distinct from the 23 synthetic classifier pairs.',
                         'No system-call/compiler/network/dispatcher tracer is installed; PATH-empty execution is a process configuration observation.',
                         'Unsigned internally consistent facts are not factual attestation or current authorization.',
                         'Validation failures must produce no CLI JSON; stdout/file I/O is not claimed atomic.',
                         'Recorded elapsed time uses this machine and its existing toolchain/cache, not an independent first-user timing.']}
    try:
        compiler, go = args.compiler.expanduser().resolve(strict=True), args.go.expanduser().resolve(strict=True)
        environment = os.environ.copy()
        environment.update(GOTOOLCHAIN='local', GOWORK='off', GOENV='off', GOPROXY='off', GOSUMDB='off', GOFLAGS='')
        for name in ('GOOO_LAYA_URL', 'GOOO_LAYA_API_KEY', 'GOOS', 'GOARCH'):
            environment.pop(name, None)
        record['tools'] = {name: {'path': str(path), 'sha256': digest(path.read_bytes())}
                           for name, path in (('compiler', compiler), ('go', go))}
        paths, before, old_bindings, new_bindings = check_bindings()
        record['source_snapshot_before'] = before
        declared = required_tests()
        version = run(record, out, 'go-version', [go, 'version'], ROOT, environment, timeout=30)
        need(version.decode().split()[:3] == ['go', 'version', 'go1.27.3'], 'Go 1.27.3 is required')
        generation_started = time.monotonic()
        run(record, out, 'generation-check', [sys.executable, ROOT / 'generate.py', '--check', '--compiler', compiler, '--go', go], ROOT, environment)
        record['generation_check_seconds'] = time.monotonic() - generation_started
        classifier = decode((ROOT / 'testdata/classifier-expected.json').read_bytes())
        need(len(classifier['cases']) == 23, 'classifier corpus changed')
        record['classifier_cases'] = 23
        run(record, out, 'go-vet', [go, 'vet', './...'], ROOT, environment)
        race_environment = {**environment, 'CGO_ENABLED': '1'}
        test_output = run(record, out, 'go-race-tests', [go, 'test', '-race', '-json', './...'], ROOT, race_environment)
        record['go_tests'] = check_test_log(test_output, declared)
        with tempfile.TemporaryDirectory(prefix='gooo-api-revision-verify-') as temporary:
            temporary = Path(temporary)
            executable = temporary / ('compare.exe' if os.name == 'nt' else 'compare')
            build_environment = {**environment, 'CGO_ENABLED': '0'}
            build_started = time.monotonic()
            run(record, out, 'build-consumer', [go, 'build', '-trimpath', '-o', executable, './cmd/compare'], ROOT, build_environment)
            record['consumer_build_seconds'] = time.monotonic() - build_started
            record['consumer_executable'] = {'bytes': executable.stat().st_size, 'sha256': digest(executable.read_bytes()),
                                             'temporary_executable_removed_on_completion': True}
            consumer_environment = {**environment, 'PATH': ''}
            record['consumer_process_configuration'] = {'PATH': '', 'GOTOOLCHAIN': 'local', 'GOPROXY': 'off',
                                                         'GOSUMDB': 'off', 'absolute_executable': True,
                                                         'working_directory': 'fresh private temporary directory per fact case'}
            record['cases'] = []
            for action in ('read', 'write'):
                work = temporary / ('consumer-' + action)
                work.mkdir(mode=0o700)
                facts_raw = (ROOT / 'testdata' / (action + '-facts.json')).read_bytes()
                exact(decode(facts_raw), literal_facts(action), action + '/nine facts fixture')
                baseline_raw = run(record, out, action + '-baseline', [executable, '-mode', 'baseline'], work, consumer_environment, facts_raw, timeout=30)
                baseline = decode(baseline_raw)
                expected = expected_report(action, old_bindings, new_bindings)
                exact(baseline, {'before_profile': OLD_PROFILE, 'candidate_profile': NEW_PROFILE, 'before': expected['before']}, action + '/baseline')
                compared_raw = run(record, out, action + '-comparison', [executable, '-mode', 'compare'], work, consumer_environment, baseline_raw, timeout=30)
                compared = decode(compared_raw)
                exact(compared, expected, action + '/all comparison fields')
                replay_raw = run(record, out, action + '-replay', [executable, '-mode', 'replay'], work, consumer_environment, compared_raw, timeout=30)
                exact(decode(replay_raw), expected, action + '/all replay fields')
                need(replay_raw == compared_raw, action + ': complete replay bytes differ')
                need(not list(work.iterdir()), 'consumer created a working-directory file')
                record['cases'].append({'name': action, 'facts_compared': 9, 'before_decision_fields_compared': 15,
                                        'after_decision_fields_compared': 15, 'difference_flags_compared': 15,
                                        'summary': expected['summary'], 'complete_bindings_compared': True,
                                        'comparison_and_replay_bytes_equal': True, 'consumer_working_directory_unchanged': True})
                record['fresh_process_pairs'] += 1
                if action == 'write':
                    altered = copy.deepcopy(compared)
                    altered['differences']['reason'] = False
                    altered_raw = (json.dumps(altered, indent=2) + '\n').encode()
                    private_write(out, 'write-altered-report.json', altered_raw)
                    refused = run(record, out, 'write-altered-refusal', [executable, '-mode', 'replay'], work, consumer_environment, altered_raw, expected_exit=1, timeout=30)
                    need(refused == b'', 'altered comparison produced partial CLI JSON')
                    record['altered_report_refused'] = True
        after = {name: digest(path.read_bytes()) for name, path in sorted(paths.items())}
        exact(after, before, 'source snapshot after all validation')
        record['source_snapshot_after'] = after
        record['original_generation_preserved'] = True
        record['original_generation_sha256'] = OLD_GENERATION
        record['original_asset_count'] = 28
        record['revision_generation_sha256'] = new_bindings['generation_sha256']
        record['revision_asset_count'] = 38
        record['status'] = 'FINITE_TWO_PROFILE_NATIVE_AND_SAVED_REPLAY_PASS'
    except (VerificationError, OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError) as exc:
        record['failure'] = str(exc)
    record['elapsed_seconds'] = time.monotonic() - started
    receipt = private_write(out, 'verification.json', (json.dumps(record, indent=2) + '\n').encode())
    print(json.dumps({'status': record['status'], 'verification': receipt}))
    return 0 if record['status'] == 'FINITE_TWO_PROFILE_NATIVE_AND_SAVED_REPLAY_PASS' else 1


if __name__ == '__main__':
    sys.exit(main())
