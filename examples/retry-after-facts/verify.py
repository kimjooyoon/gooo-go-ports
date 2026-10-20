#!/usr/bin/env python3
"""Fresh, bounded native/oracle/saved-replay check for Retry-After facts.

Run from a checkout with its pinned compiler and Go1.27.3. This is a finite
profile check, not producer authentication or permission to dispatch a retry.
"""
import argparse
import copy
import hashlib
import importlib.util
import json
import os
import pathlib
import re
import shutil
import signal
import stat
import sys
import tempfile
import time

EXAMPLE = 'examples/retry-after-facts'
REV = '205896a413e5dea92366309c35ae85e542d055f6'
PROFILE = [EXAMPLE + '/' + name for name in (
    'main.gooo', 'contract.json', 'reference/main.go', 'witnesses.json',
    'inputs.json', 'request_inputs.py', 'request_inputs_test.py', 'request.json',
    'README.md', 'receipts.py', 'receipts_test.py', 'verify.py', 'verify_test.py')]
PROFILE += ['retry-after-facts.workspace.json', 'ports/strconv/decimal.gooo',
            'ports/strconv/provenance.json', 'README.md', 'docs/ai-agents.md',
            'upstream.json', 'go.mod', 'go.sum', 'LICENSE',
            'tools/check-header-addition-host.py', 'tools/native_verification_contract.py',
            '.github/workflows/ci.yml', 'tools/check-ci-scope-workflow_test.py']


def need(condition, message):
    if not condition:
        raise RuntimeError(message)


def canonical(value):
    path = pathlib.Path(value)
    need(path.is_absolute() and path.resolve() == path and not path.is_symlink(),
         'canonical absolute path required: ' + str(path))
    return path


def ref(raw):
    return {'bytes': len(raw), 'sha256': hashlib.sha256(raw).hexdigest()}


class Frozen:
    """Read bounded regular files without following the final symlink."""
    def __init__(self):
        self.pins = {}
        self.metadata = {}

    def read(self, value, cap=1 << 20):
        path = canonical(value)
        with os.fdopen(os.open(path, os.O_RDONLY | os.O_NOFOLLOW), 'rb') as stream:
            before = os.fstat(stream.fileno())
            need(stat.S_ISREG(before.st_mode) and 0 <= before.st_size <= cap,
                 'bounded regular input required: ' + str(path))
            raw = stream.read(cap + 1)
            after = os.fstat(stream.fileno())
        final = path.lstat()
        signature = lambda info: (info.st_dev, info.st_ino, info.st_mode, info.st_size, info.st_mtime_ns)
        need(len(raw) <= cap and len(raw) == before.st_size and
             signature(before) == signature(after) == signature(final),
             'input changed during bounded read: ' + str(path))
        need(str(path) not in self.metadata or self.metadata[str(path)] == signature(before),
             'frozen file identity/metadata changed: ' + str(path))
        current = ref(raw)
        need(str(path) not in self.pins or self.pins[str(path)] == current,
             'frozen input changed: ' + str(path))
        self.pins[str(path)] = current
        self.metadata[str(path)] = signature(before)
        return raw

    def check(self):
        for name, expected in list(self.pins.items()):
            self.read(pathlib.Path(name), max(1 << 20, expected['bytes']))


def module(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    loaded = importlib.util.module_from_spec(spec)
    sys.modules[name] = loaded
    spec.loader.exec_module(loaded)
    return loaded


def save(path, value):
    with path.open('x', encoding='utf-8') as stream:
        json.dump(value, stream, ensure_ascii=False, indent=2)
        stream.write('\n')


def run(args):
    root, compiler, go, out = map(canonical, (args.root, args.compiler, args.go, args.out))
    need(root.is_dir() and not out.exists() and out.parent.is_dir(),
         'existing checkout/output parent and fresh output directory required')
    need(out != root and (root not in out.parents or out.relative_to(root).parts[0] == 'out'),
         'in-checkout output must be below out/')
    out.mkdir()
    frozen = Frozen()
    runner = work = work_identity = ctx = None
    controls, results = [], []
    error, removed = None, False
    started = time.monotonic()
    def interrupted(number, _frame):
        raise RuntimeError('received signal ' + str(number))
    handlers = {number: signal.signal(number, interrupted) for number in (signal.SIGINT, signal.SIGTERM)}
    try:
        # The fixed import locations are source inputs, never receipt-selected code.
        for name in PROFILE:
            frozen.read(root / name)
        need(canonical(pathlib.Path(__file__).resolve()) == root / EXAMPLE / 'verify.py',
             'verifier belongs to the selected checkout')
        frozen.read(compiler, 64 << 20)
        frozen.read(go, 64 << 20)
        need(os.access(compiler, os.X_OK) and os.access(go, os.X_OK), 'executable compiler and Go required')
        python = canonical(pathlib.Path(sys.executable).resolve())
        frozen.read(python, 64 << 20)
        H = module('retry_after_owned_commands', root / 'tools/check-header-addition-host.py')
        R = module('retry_after_receipts', root / EXAMPLE / 'receipts.py')
        frozen.check()
        need(shutil.disk_usage(out).free >= 1 << 30, 'at least1GiB free required')
        work = pathlib.Path(tempfile.mkdtemp(prefix='.retry-after-owned-', dir=out))
        work_identity = (work.stat().st_dev, work.stat().st_ino)
        (work / 'tmp').mkdir()
        # Reuse the existing Go cache. Do not create another global cache or fetch.
        env = {key: os.environ[key] for key in ('PATH', 'HOME', 'SYSTEMROOT', 'GOCACHE') if key in os.environ}
        env.update(GOTOOLCHAIN='local', GOWORK='off', GOENV='off', GOPROXY='off',
                   GOSUMDB='off', GOFLAGS='-mod=readonly -p=1', CGO_ENABLED='0',
                   GOMAXPROCS='2', TMPDIR=str(work / 'tmp'))
        runner = H.OwnedCommands(out, work, root, frozen.pins, env)
        runner.started = started
        save(out / 'owned-work.json', {'path': str(work), 'whole_timeout_s': 300,
             'owned_byte_cap': 1 << 30, 'cache': 'existing Go cache; no downloads',
             'cleanup': 'exact owned inode only, after observed child closure'})
        version = runner.call('go-version', [go, 'version'], 15).decode('utf-8').strip()
        need(re.fullmatch(r'go version go1\.27\.3 (?:linux/amd64|darwin/arm64)', version),
             'pinned Go1.27.3 and supported platform required')
        identity = R.load(runner.call('compiler-identity', [compiler, 'version', '--build', '--json'], 15))
        producer = R.producer_module(identity)
        starter_raw = runner.call('request-input-gate',
            [python, '-I', '-B', root / EXAMPLE / 'request_inputs.py', root / EXAMPLE / 'request.json'], 15)
        starter = R.load(starter_raw)
        oracle_raw = runner.call('independent-go-reference',
            [go, 'run', './' + EXAMPLE + '/reference', root / EXAMPLE / 'witnesses.json'], 60)
        ctx = R.prepare(root, oracle_raw, version, 'sha256:' + frozen.pins[str(go)]['sha256'], frozen.read, producer)
        parts = R.batches(ctx)
        need(R.equal(starter, {'schema': 'gooo/body-composition-inputs/v1', 'inputs': [ctx['inputs'][4]]}),
             'actual transport gate is the original header-04 input')
        need(ctx['witnesses'][4]['name'] == 'header-04', 'starter witness identity')
        need(sum(R.equal(row, starter['inputs'][0]) for row in ctx['inputs']) == 1,
             'starter input occurs exactly once in the original corpus')
        starter_rows = copy.deepcopy(parts[0]['rows'])
        starter_rows[4] = starter['inputs'][0]
        actual_raw = R.go_json({'schema': 'gooo/body-composition-inputs/v1', 'inputs': starter_rows}, html=False) + b'\n'
        need(actual_raw == parts[0]['raw'], 'gate-bound batch retains exact original values and order')
        parts[0]['raw'] = actual_raw
        for part in parts:
            index = part['index']
            input_path = out / ('inputs-' + str(index) + '.json')
            with input_path.open('xb') as stream:
                stream.write(part['raw'])
            frozen.read(input_path)
            manifest = root / 'retry-after-facts.workspace.json'
            execute_label, replay_label = 'execute-' + str(index), 'saved-replay-' + str(index)
            execute = runner.call(execute_label,
                [compiler, 'package', 'execute', '--json', '--go', go, '--inputs', input_path, manifest], 90)
            program = R.validate(execute, ctx, part)
            execution_path = out / execute_label / 'stdout'
            frozen.read(execution_path, R.RECEIPT_CAP)
            replay = runner.call(replay_label,
                [compiler, 'package', 'replay', '--json', '--go', go, '--receipt', execution_path,
                 '--inputs', input_path, manifest], 90)
            R.validate(replay, ctx, part, parent_raw=execute, parent_program=program)
            frozen.read(out / replay_label / 'stdout', R.RECEIPT_CAP)
            if index == 0:
                controls = R.negative_controls(execute, replay, ctx, part)
            results.append({'index': index, 'start': part['start'], 'end': part['end'],
                'inputs': ref(part['raw']), 'execute': ref(execute), 'saved_replay': ref(replay),
                'independent_wire': part['wire_ref']})
        need(len(controls) == 11 and len({row['name'] for row in controls}) == 11 and
             all(row['status'] == 'REFUSED_AT_EXPECTED_BOUNDARY' for row in controls),
             'all11 distinct mutation controls refused at the intended boundaries')
        frozen.check()
        need(time.monotonic() - started <= 300, 'whole300s verification budget')
    except BaseException as problem:
        error = {'type': type(problem).__name__, 'message': str(problem)}
        if hasattr(problem, 'boundary'):
            error['boundary'] = problem.boundary
    finally:
        # A process-observation failure leaves the work directory for diagnosis.
        if work is not None and (runner is None or runner.closed):
            try:
                info = work.lstat()
                need(stat.S_ISDIR(info.st_mode) and not work.is_symlink() and
                     (info.st_dev, info.st_ino) == work_identity and work.parent == out,
                     'owned cleanup inode/parent mismatch')
                shutil.rmtree(work)
                removed = not work.exists()
            except BaseException as problem:
                error = error or {'type': type(problem).__name__, 'message': str(problem)}
        for number, previous in handlers.items():
            signal.signal(number, previous)
    closed = runner is not None and runner.closed
    accepted = error is None and closed and removed and len(results) == 2
    summary = {'schema': 'gooo/retry-after-facts-verification/v1',
        'status': 'NATIVE_ORACLE_AND_SAVED_REPLAY_MATCH' if accepted else 'NOT_ACCEPTED',
        'error': error, 'cases': 113 if accepted else 0, 'batches': len(results),
        'typed_fields_per_phase': 1582 if accepted else 0,
        'input_scalar_checks_per_phase': 678 if accepted else 0,
        'native_runs_per_phase': 4 if accepted else 0,
        'starter_gate_bound': accepted, 'starter_case_index': 4,
        'negative_controls': controls, 'processes_closed': closed, 'owned_work_removed': removed,
        'model_calls': 0, 'finite_total': 0, 'catalog_extended': False,
        'native_acceptance': False, 'producer_authenticated': False,
        'source_and_tool_pins': frozen.pins, 'batch_results': results,
        'commands': [] if runner is None else runner.observations,
        'not_checked': None if ctx is None else ctx['not_checked'],
        'wall_seconds': time.monotonic() - started,
        'scope': '113 source-authored inputs, independent pinned Go oracle, fresh native and saved replay. No current provider facts, retry authority, scheduler or producer authentication.'}
    save(out / 'summary.json', summary)
    print(json.dumps({'status': summary['status'], 'summary': str(out / 'summary.json'), 'error': error}))
    return 0 if accepted else 1


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ('root', 'compiler', 'go', 'out'):
        parser.add_argument('--' + name, required=True)
    args = parser.parse_args(argv)
    try:
        return run(args)
    except (RuntimeError, OSError, ValueError) as error:
        print('Retry-After verification refused: ' + str(error), file=sys.stderr)
        return 1


if __name__ == '__main__':
    raise SystemExit(main())
