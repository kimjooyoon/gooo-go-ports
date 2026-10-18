"""Actual exited-child/buffered-pipe regression; no MCP or Go execution."""
import argparse
import importlib.util
import io
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import time
import unittest

RECORDS = []


def absent(pid):
    try:
        os.killpg(pid, 0)
        return False
    except ProcessLookupError:
        return True


class CloseRegression(unittest.TestCase):
    def session(self, label):
        session = MODULE.Session(label, '2025-11-25')
        # Deterministically close the pipe reader before buffering a new write.
        self.assertEqual(session.process.wait(timeout=2), 0)
        self.assertIsInstance(session.process.stdin, io.BufferedWriter)
        pending = b'buffered request after child exit\n'
        self.assertEqual(session.process.stdin.write(pending), len(pending))
        return session

    def finish(self, session):
        # Rescue is test-owned, not credited to the original broken close method.
        rescued = {'stdout': bytearray(), 'stderr': bytearray()}
        try:
            for name in rescued:
                stream = getattr(session.process, name)
                if not stream.closed:
                    os.set_blocking(stream.fileno(), True)
                    raw = stream.read()
                    if raw:
                        rescued[name].extend(raw)
                    self.assertEqual(stream.read(), b'')
        finally:
            try:
                session.process.stdin.close()
            except BrokenPipeError:
                pass
            for name in ('stdout', 'stderr'):
                getattr(session.process, name).close()
            session.selector.close()
            code = session.process.wait(timeout=1)
        row = {'label': session.label, 'PID': session.process.pid,
               'exit_code': code, 'waited': True, 'process_group_ESRCH': absent(session.process.pid),
               'stdout_closed': session.process.stdout.closed,
               'stderr_closed': session.process.stderr.closed,
               'test_rescued_stdout': rescued['stdout'].decode(),
               'test_rescued_stderr': rescued['stderr'].decode(),
               'method_process_json_retained': (session.work/'process.json').exists(),
               'child_was_already_waited_to_force_closed_pipe': True}
        RECORDS.append(row)
        self.assertTrue(row['process_group_ESRCH'])
        return row

    def primary_then_close(self, session, read_error=None):
        if read_error is not None:
            def failed_read():
                raise read_error
            session.read = failed_read
        first = caught = None
        try:
            try:
                session.process.stdin.flush()
            except BrokenPipeError as error:
                first = error
                try:
                    raise
                finally:
                    session.close(repr(error))
        except BaseException as error:
            caught = error
        self.assertIsNotNone(first, 'fixture did not cause a real buffered write failure')
        return first, caught

    def test_primary_failure_and_diagnostics_survive_buffered_close(self):
        s = self.session('primary')
        try:
            first, caught = self.primary_then_close(s)
            self.assertIs(caught, first, 'secondary close error replaced the actual request failure')
            record = json.loads((s.work/'process.json').read_text())
            self.assertEqual(record['failure'], repr(first))
            self.assertIn('BrokenPipeError', record['stdin_close_error'])
            self.assertTrue(record['streams_EOF'] and record['waited'] and record['process_group_ESRCH'])
            self.assertEqual((s.work/'stdout').read_bytes(), b'child stdout retained\n')
            self.assertEqual((s.work/'stderr').read_bytes(), b'child stderr retained\n')
        finally:
            self.finish(s)

    def test_first_close_failure_is_recorded_and_raised(self):
        s = self.session('close-first')
        try:
            with self.assertRaises(BrokenPipeError) as raised:
                s.close(None)
            record = json.loads((s.work/'process.json').read_text())
            self.assertEqual(record['failure'], repr(raised.exception))
            self.assertEqual(record['stdin_close_error'], repr(raised.exception))
            self.assertTrue(record['streams_EOF'] and record['waited'] and record['process_group_ESRCH'])
            self.assertEqual((s.work/'stdout').read_bytes(), b'child stdout retained\n')
            self.assertEqual((s.work/'stderr').read_bytes(), b'child stderr retained\n')
        finally:
            self.finish(s)

    def test_secondary_read_error_keeps_the_primary_failure(self):
        s = self.session('read-secondary')
        try:
            secondary = OSError('owned drain failure')
            first, caught = self.primary_then_close(s, secondary)
            self.assertIs(caught, first)
            record = json.loads((s.work/'process.json').read_text())
            self.assertEqual(record['failure'], repr(first))
            self.assertEqual(record['teardown_error'], repr(secondary))
            self.assertTrue(record['waited'] and record['process_group_ESRCH'])
            self.assertFalse(record['streams_EOF'])
        finally:
            self.finish(s)

    def cancellation(self, label, cancellation):
        s = self.session(label)
        try:
            first, caught = self.primary_then_close(s, cancellation)
            self.assertIs(caught, cancellation, 'cancellation was swallowed by an ordinary error')
            record = json.loads((s.work/'process.json').read_text())
            self.assertEqual(record['failure'], repr(first))
            self.assertEqual(record['teardown_error'], repr(cancellation))
            self.assertTrue(record['waited'] and record['process_group_ESRCH'])
        finally:
            self.finish(s)

    def test_keyboard_interrupt_propagates_after_cleanup(self):
        self.cancellation('keyboard-interrupt', KeyboardInterrupt('owned cancellation'))

    def test_system_exit_propagates_after_cleanup(self):
        self.cancellation('system-exit', SystemExit(17))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--verifier', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    args.out.mkdir(mode=0o700)
    child = args.out/'exited-child.py'
    child.write_text('#!'+sys.executable+'\nimport os\nos.write(1,b"child stdout retained\\n")\nos.write(2,b"child stderr retained\\n")\nos._exit(0)\n')
    child.chmod(0o700)
    spec = importlib.util.spec_from_file_location('owned_mcp_verifier', args.verifier)
    MODULE = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(MODULE)
    MODULE.BINARY = child.resolve()
    MODULE.OUTPUT = args.out
    started = time.monotonic()
    result = unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(CloseRegression))
    report = {'verifier': str(args.verifier), 'tests_run': result.testsRun,
              'failures': len(result.failures), 'errors': len(result.errors),
              'success': result.wasSuccessful(), 'actual_children': RECORDS,
              'all_children_waited_closed_group_absent': all(x['waited'] and x['process_group_ESRCH'] and x['stdout_closed'] and x['stderr_closed'] for x in RECORDS),
              'elapsed_seconds': time.monotonic()-started,
              'MCP_Go_network_or_policy_execution': 0}
    (args.out/'regression.json').write_text(json.dumps(report, indent=2)+'\n')
    sys.exit(0 if result.wasSuccessful() else 1)
