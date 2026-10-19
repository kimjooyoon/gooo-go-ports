"""Finite stdlib cancellation regressions; no Go, compiler, SDK or network."""
import argparse
import datetime
import hashlib
import importlib.util
import json
import os
from pathlib import Path
from contextlib import contextmanager
import selectors
import signal
import subprocess
import sys
import tempfile
import time
import traceback
import unittest
from unittest import mock

CAP = 64 * 1024
TOTAL_SECONDS = 30
SCENARIOS = ('normal', 'term_live', 'int_live', 'term_spawn', 'int_spawn', 'term_cleanup_diagnostic', 'primary_error_cleanup_signal')
TARGET = None
OUT = None
START = None
CASE_RECORDS = []


def encoded(value):
    return (json.dumps(value, indent=2) + '\n').encode()


def write(path, value):
    path.write_bytes(encoded(value))


def reference(path):
    raw = path.read_bytes()
    return {'path': str(path), 'bytes': len(raw), 'sha256': hashlib.sha256(raw).hexdigest()}


def load_target(path):
    spec = importlib.util.spec_from_file_location('service_grant_many_verifier', path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def absent(pgid):
    try:
        os.killpg(pgid, 0)
        return False
    except ProcessLookupError:
        return True


def worker(target, root, scenario):
    module = load_target(target)
    module.R = root / 'evidence'
    module.R.mkdir()
    child_source = root / 'fake_child.py'
    child_source.write_text('''import json,os,signal,sys,time
from pathlib import Path
root=Path(sys.argv[1])
(root/'child-ready.json').write_text(json.dumps({'PID':os.getpid(),'PGID':os.getpgrp(),'mask':[int(s) for s in signal.pthread_sigmask(signal.SIG_BLOCK,set())]}))
print('fake-child-ready',flush=True)
print('fake-child-stderr',file=sys.stderr,flush=True)
(root/'child-output-ready').write_text('both output streams flushed')
if sys.argv[2]=='normal':
 time.sleep(.03)
else:
 while True:time.sleep(.02)
''')
    real_popen = subprocess.Popen
    real_killpg = os.killpg
    real_save = module.save
    real_usage = module.usage
    real_blocked = module.blocked_interrupts
    primary_error = RuntimeError('INJECTED_PRIMARY_USAGE_ERROR')
    primary_raised = False
    cleanup_signal_injected = False
    child = None
    second_signal = False
    original_handlers = {s: signal.getsignal(s) for s in (signal.SIGTERM, signal.SIGINT)}
    original_mask = signal.pthread_sigmask(signal.SIG_BLOCK, set())

    def observed_spawn(*args, **kwargs):
        nonlocal child
        child = real_popen(*args, **kwargs)
        write(root / 'spawn-observation.json', {'PID': child.pid, 'PGID': child.pid,
              'worker_PID': os.getpid(), 'worker_PGID': os.getpgrp(),
              'mask': [int(s) for s in signal.pthread_sigmask(signal.SIG_BLOCK, set())]})
        if scenario.endswith('_spawn'):
            until = time.monotonic() + 4
            while not (root / 'release-spawn').exists() and time.monotonic() < until:
                time.sleep(.005)
            # Always return the already started child to owned(), even on gate failure.
            if not (root / 'release-spawn').exists():
                write(root / 'spawn-gate-timeout.json', {'timed_out': True})
        return child

    def observed_killpg(pgid, number):
        nonlocal second_signal
        if scenario == 'term_cleanup_diagnostic' and child is not None and pgid == child.pid and number == signal.SIGTERM and not second_signal:
            second_signal = True
            write(root / 'second-signal.json', {'signal': 'SIGINT', 'during_cleanup': True,
                  'mask': [int(s) for s in signal.pthread_sigmask(signal.SIG_BLOCK, set())]})
            os.kill(os.getpid(), signal.SIGINT)
        return real_killpg(pgid, number)

    def observed_save(path, value):
        if scenario == 'term_cleanup_diagnostic' and path.name == 'process.json':
            raise OSError('injected process-record diagnostic failure')
        return real_save(path, value)

    def primary_usage():
        nonlocal primary_raised
        until = time.monotonic() + 2
        while not (root / 'child-output-ready').exists() and time.monotonic() < until:
            time.sleep(.005)
        assert (root / 'child-output-ready').is_file()
        primary_raised = True
        raise primary_error

    @contextmanager
    def observed_blocked():
        nonlocal cleanup_signal_injected
        if scenario == 'primary_error_cleanup_signal' and primary_raised and not cleanup_signal_injected:
            cleanup_signal_injected = True
            write(root / 'primary-cleanup-signal.json', {'signal': 'SIGTERM',
                  'worker_PID': os.getpid(), 'child_PID': child.pid,
                  'primary_error_already_raised': True, 'before_cleanup_mask': True,
                  'mask': [int(s) for s in signal.pthread_sigmask(signal.SIG_BLOCK, set())]})
            os.kill(os.getpid(), signal.SIGTERM)
        with real_blocked() as previous:
            yield previous

    module.subprocess.Popen = observed_spawn
    module.os.killpg = observed_killpg
    module.save = observed_save
    module.blocked_interrupts = observed_blocked
    if scenario == 'primary_error_cleanup_signal':
        module.usage = primary_usage
    first = None
    output = None
    try:
        with module.cancellation_handlers():
            output = module.owned('fake', [sys.executable, '-I', '-B', child_source, root, scenario],
                                  {'PATH': '', 'HOME': str(root), 'TMPDIR': str(root)}, 5, root)
    except BaseException as error:
        first = error
    finally:
        module.subprocess.Popen = real_popen
        module.os.killpg = real_killpg
        module.save = real_save
        module.usage = real_usage
        module.blocked_interrupts = real_blocked
    result = {'scenario': scenario, 'first_type': None if first is None else type(first).__name__,
              'first_message': None if first is None else str(first),
              'records': module.records, 'diagnostics': module.diagnostics,
              'output': None if output is None else output.decode(),
              'handlers_restored': all(signal.getsignal(s) is h or signal.getsignal(s) == h for s, h in original_handlers.items()),
              'mask_restored': signal.pthread_sigmask(signal.SIG_BLOCK, set()) == original_mask,
              'second_signal_injected': second_signal,
              'primary_error_identity_preserved': first is primary_error,
              'cleanup_signal_injected_after_primary_error': cleanup_signal_injected,
              'PID_journal_exists': (module.R / 'fake/started-process.json').is_file()}
    write(root / 'worker-result.json', result)
    print(json.dumps({'scenario': scenario, 'first_type': result['first_type']}), flush=True)
    return 0 if first is None else 1


def run_case(scenario):
    root = OUT / scenario
    root.mkdir()
    proc = None
    selector = selectors.DefaultSelector()
    handles = {}
    counts = {'stdout': 0, 'stderr': 0}
    child_pgid = None
    sent = False
    eof = False
    first = None
    started = time.monotonic()
    deadline = min(started + 8, START + TOTAL_SECONDS)
    try:
        for name in counts:
            handles[name] = (root / ('worker-' + name)).open('xb')
        previous = signal.pthread_sigmask(signal.SIG_BLOCK, {signal.SIGTERM, signal.SIGINT})
        try:
            proc = subprocess.Popen([sys.executable, '-I', '-B', str(Path(__file__).resolve()), '--worker',
                    '--target', str(TARGET), '--out', str(root), '--scenario', scenario],
                    cwd=root, env={'PATH': '', 'HOME': str(root), 'TMPDIR': str(root)},
                    stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                    start_new_session=True,
                    preexec_fn=lambda: signal.pthread_sigmask(signal.SIG_SETMASK, previous))
            write(root / 'worker-started.json', {'PID': proc.pid, 'PGID': proc.pid})
            for name, pipe in [('stdout', proc.stdout), ('stderr', proc.stderr)]:
                os.set_blocking(pipe.fileno(), False)
                selector.register(pipe, selectors.EVENT_READ, name)
        finally:
            signal.pthread_sigmask(signal.SIG_SETMASK, previous)
        while selector.get_map() or proc.poll() is None:
            if time.monotonic() > deadline:
                raise TimeoutError('bounded regression worker deadline')
            observation = root / 'spawn-observation.json'
            child_ready = root / 'child-ready.json'
            journal = root / 'evidence/fake/started-process.json'
            if scenario not in ('normal', 'primary_error_cleanup_signal') and not sent and observation.exists() and child_ready.exists() and (root / 'child-output-ready').exists() and (scenario.endswith('_spawn') or journal.exists()):
                info = json.loads(observation.read_bytes())
                child_pgid = info['PGID']
                assert child_pgid == info['PID'] and child_pgid > 1 and child_pgid != proc.pid
                assert os.getpgid(child_pgid) == child_pgid
                assert info['worker_PID'] == proc.pid and info['worker_PGID'] == proc.pid
                ready = json.loads(child_ready.read_bytes())
                assert ready['PID'] == child_pgid and ready['PGID'] == child_pgid
                assert signal.SIGTERM not in ready['mask'] and signal.SIGINT not in ready['mask']
                if scenario.endswith('_spawn'):
                    assert signal.SIGTERM in info['mask'] and signal.SIGINT in info['mask']
                    assert not journal.exists(), 'signal must precede started-PID journal'
                number = signal.SIGINT if scenario.startswith('int_') else signal.SIGTERM
                write(root / 'signal-sent.json', {'worker_PID': proc.pid, 'child_PID': child_pgid,
                      'signal': signal.Signals(number).name, 'boundary': 'Popen returned, before assignment/journal' if scenario.endswith('_spawn') else 'registered live child'})
                os.kill(proc.pid, number)
                sent = True
                if scenario.endswith('_spawn'):
                    (root / 'release-spawn').write_text('released after signal\n')
            for key, _ in selector.select(.005):
                raw = os.read(key.fileobj.fileno(), 4096)
                if not raw:
                    selector.unregister(key.fileobj)
                    key.fileobj.close()
                    continue
                counts[key.data] += len(raw)
                room = max(0, CAP - sum(p.stat().st_size for p in [root / 'worker-stdout', root / 'worker-stderr']))
                handles[key.data].write(raw[:room])
                handles[key.data].flush()
                if sum(counts.values()) > CAP:
                    raise ValueError('bounded worker output exceeded')
        eof = True
        proc.wait(timeout=1)
        expected_exit = 0 if scenario == 'normal' else 1
        assert proc.returncode == expected_exit, (scenario, proc.returncode)
        result = json.loads((root / 'worker-result.json').read_bytes())
        assert result['handlers_restored'] is True and result['mask_restored'] is True
        assert result['PID_journal_exists'] is True
        assert len(result['records']) == 1
        record = result['records'][0]
        child_pgid = record['PGID']
        assert record['PID'] == child_pgid
        assert record['waited'] is True and record['streams_EOF'] is True and record['PG_ESRCH'] is True
        assert absent(child_pgid)
        assert record['stdout']['bytes'] > 0 and record['stderr']['bytes'] > 0
        if scenario == 'normal':
            assert result['first_type'] is None and record['first_failure'] is None and record['exit_code'] == 0
        elif scenario == 'primary_error_cleanup_signal':
            assert not sent and result['cleanup_signal_injected_after_primary_error'] is True
            assert result['first_type'] == 'RuntimeError' and result['first_message'] == 'INJECTED_PRIMARY_USAGE_ERROR'
            assert result['primary_error_identity_preserved'] is True
            assert record['first_failure'] == 'RuntimeError: INJECTED_PRIMARY_USAGE_ERROR'
            assert record['exit_code'] == -signal.SIGTERM
            marker = json.loads((root / 'primary-cleanup-signal.json').read_bytes())
            assert marker['child_PID'] == child_pgid and marker['worker_PID'] == proc.pid
            assert marker['primary_error_already_raised'] is True and marker['before_cleanup_mask'] is True
            assert signal.SIGTERM not in marker['mask'] and signal.SIGINT not in marker['mask']
            assert any('deferred cancellation: VerifierCancelled: signal SIGTERM' in d for d in result['diagnostics'])
        else:
            wanted = 'SIGINT' if scenario.startswith('int_') else 'SIGTERM'
            assert sent and result['first_type'] == 'VerifierCancelled' and result['first_message'] == 'signal ' + wanted
            assert record['first_failure'] == 'VerifierCancelled: signal ' + wanted
            assert record['exit_code'] == -signal.SIGTERM, 'child must inherit original unblocked mask'
        if scenario != 'normal':
            assert result['output'] is None, 'cancelled/failed command must not return as success'
        if scenario == 'term_cleanup_diagnostic':
            assert result['second_signal_injected'] is True
            assert any('additional cancellation: SIGINT' in d for d in result['diagnostics'])
            assert any('process record: OSError: injected process-record' in d for d in result['diagnostics'])
            assert not (root / 'evidence/fake/process.json').exists()
        assert not (root / 'spawn-gate-timeout.json').exists()
    except BaseException as error:
        first = error
    finally:
        if proc is not None:
            if proc.poll() is None:
                try:
                    os.killpg(proc.pid, signal.SIGTERM)
                except ProcessLookupError:
                    pass
                try:
                    proc.wait(timeout=3)
                except subprocess.TimeoutExpired:
                    if child_pgid is None and (root / 'spawn-observation.json').exists():
                        child_pgid = json.loads((root / 'spawn-observation.json').read_bytes())['PGID']
                    if child_pgid is not None and not absent(child_pgid):
                        os.killpg(child_pgid, signal.SIGKILL)
                    try:
                        os.killpg(proc.pid, signal.SIGKILL)
                    except ProcessLookupError:
                        pass
                    proc.wait(timeout=1)
            proc.wait()
        if child_pgid is None and (root / 'spawn-observation.json').exists():
            info = json.loads((root / 'spawn-observation.json').read_bytes())
            assert info['PID'] == info['PGID'] and info['PGID'] > 1 and info['PGID'] != os.getpgrp()
            child_pgid = info['PGID']
        if child_pgid is not None and not absent(child_pgid):
            if first is None:
                first = RuntimeError('subject left its recorded child group alive')
            try:
                os.killpg(child_pgid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            until = time.monotonic() + 1
            while not absent(child_pgid) and time.monotonic() < until:
                time.sleep(.01)
        for key in list(selector.get_map().values()):
            selector.unregister(key.fileobj)
            key.fileobj.close()
        selector.close()
        for handle in handles.values():
            handle.close()
        closure = {'scenario': scenario, 'worker_PID': None if proc is None else proc.pid,
                   'worker_exit': None if proc is None else proc.returncode,
                   'worker_waited': proc is not None and proc.returncode is not None,
                   'worker_streams_EOF': eof,
                   'worker_PG_ESRCH': proc is not None and absent(proc.pid),
                   'child_PGID': child_pgid, 'child_PG_ESRCH': child_pgid is not None and absent(child_pgid),
                   'child_wait_scope': 'direct wait recorded inside worker; outer test is not this child parent',
                   'elapsed_seconds': time.monotonic() - started,
                   'pipe_bytes': counts,
                   'first_failure': None if first is None else type(first).__name__ + ': ' + str(first)}
        write(root / 'closure.json', closure)
        CASE_RECORDS.append(closure)
    if first is not None:
        raise first
    assert closure['worker_PG_ESRCH'] and closure['child_PG_ESRCH']


class CancellationRegressions(unittest.TestCase):
    def test_actual_normal_and_cancellation_boundaries(self):
        for scenario in SCENARIOS:
            with self.subTest(scenario=scenario):
                run_case(scenario)

    def test_mask_restoration_keeps_first_exception(self):
        module = load_target(TARGET)
        original = ValueError('earlier validation failure')
        with mock.patch.object(module.signal, 'pthread_sigmask', side_effect=[set(), OSError('restore mask')]):
            with self.assertRaises(ValueError) as caught:
                with module.blocked_interrupts():
                    raise original
        self.assertIs(caught.exception, original)
        self.assertTrue(any('signal mask restoration' in item for item in module.diagnostics))

    def test_handler_restoration_keeps_first_exception(self):
        module = load_target(TARGET)
        original = ValueError('earlier validation failure')
        with mock.patch.object(module.signal, 'pthread_sigmask', return_value=set()), \
                mock.patch.object(module.signal, 'signal', side_effect=[signal.SIG_DFL, signal.SIG_DFL, OSError('restore handler'), OSError('restore handler')]):
            with self.assertRaises(ValueError) as caught:
                with module.cancellation_handlers():
                    raise original
        self.assertIs(caught.exception, original)
        self.assertEqual(len(module.diagnostics), 2)


def main():
    global TARGET, OUT, START
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--target', type=Path, default=Path(__file__).resolve().parents[1] / 'examples/service-grant-many/verify.py')
    parser.add_argument('--out', type=Path)
    parser.add_argument('--worker', action='store_true', help=argparse.SUPPRESS)
    parser.add_argument('--scenario', choices=SCENARIOS, help=argparse.SUPPRESS)
    args, rest = parser.parse_known_args()
    TARGET = args.target.resolve()
    if args.worker:
        return worker(TARGET, args.out.resolve(), args.scenario)
    OUT = args.out.resolve() if args.out else Path(tempfile.mkdtemp(prefix='service-grant-many-verifier-'))
    if args.out:
        OUT.mkdir(parents=True, exist_ok=False)
    START = time.monotonic()
    first = None
    result = None
    previous = {}
    interrupted = False

    def cancel(number, _frame):
        nonlocal interrupted
        if not interrupted:
            interrupted = True
            raise KeyboardInterrupt('regression supervisor signal ' + signal.Signals(number).name)

    try:
        for number in (signal.SIGTERM, signal.SIGINT):
            previous[number] = signal.signal(number, cancel)
        result = unittest.main(argv=[sys.argv[0], *rest], exit=False, failfast=True)
    except BaseException as error:
        first = {'type': type(error).__name__, 'message': str(error), 'traceback': traceback.format_exc()}
    finally:
        for number, handler in previous.items():
            signal.signal(number, handler)
        outcome = result.result if result else None
        passed = first is None and outcome is not None and outcome.wasSuccessful()
        receipt = {'status': 'PASS' if passed else 'FIRST_FAILURE_PRESERVED',
                   'target': reference(TARGET), 'test_source': reference(Path(__file__).resolve()),
                   'first_failure': first, 'unittest_tests_run': outcome.testsRun if outcome else None,
                   'failures': outcome.failures if outcome else [], 'errors': outcome.errors if outcome else [],
                   'actual_cases': CASE_RECORDS, 'actual_fake_children': len(CASE_RECORDS),
                   'recorded_worker_and_child_groups_absent': all(c['worker_PG_ESRCH'] and c['child_PG_ESRCH'] for c in CASE_RECORDS),
                   'elapsed_seconds': time.monotonic() - START, 'whole_seconds_limit': TOTAL_SECONDS,
                   'worker_seconds_limit': 8, 'worker_combined_stream_cap': CAP,
                   'Go_compiler_native_model_network_SDK_calls': 0,
                   'scope': 'Real stdlib Python fake child only; no actual compiler or policy validation.'}
        write(OUT / 'receipt.json', receipt)
        print(json.dumps({'status': receipt['status'], 'receipt': reference(OUT / 'receipt.json')}), flush=True)
    return 0 if passed else 1


if __name__ == '__main__':
    raise SystemExit(main())
