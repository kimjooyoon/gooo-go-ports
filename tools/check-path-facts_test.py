"""Prepared regressions for exact public evidence boundaries; no Gooo execution."""
from pathlib import Path
import copy
import errno
import io
import os
import subprocess
import time
import importlib.util
import json
import tempfile
import unittest
from unittest import mock

SPEC=importlib.util.spec_from_file_location('path_checker',Path(__file__).with_name('check-path-facts.py'))
CHECK=importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(CHECK)


def fixture(entry='report'):
    return {'schema':'gooo/path-facts-checked/v1','decision':'MANUAL24_CLI_REPLAY_AND_FULL9841_GENERATED_NATIVE_PASS','entry':entry,
        'native_observed_cases':24,'saved_replay_observed_cases':24,'independent_go_oracle_cases':24,
        'full_generated_native_outputs':9841,'full_CLI_outputs':0,'full_saved_replay_outputs':0,'native_receipt_expectation_cases':0,
        'generated_sha256':'sha256:'+'1'*64,'full_generated_corpus':{'input_count':9841,'comparison_count':9841,
        'scalar_comparison_count':59046 if entry=='report' else 9841,'mismatches':0,'generated_sha256':'sha256:'+'1'*64}}


class EvidenceLayerTests(unittest.TestCase):
    def test_positive_count_contract(self):
        for entry in ('ext','isabs','report'):
            self.assertEqual(CHECK.checked_entry(fixture(entry),entry)['input_count'],9841)

    def test_typed_count_and_layer_mutations(self):
        for key in ('native_observed_cases','saved_replay_observed_cases','independent_go_oracle_cases',
                    'full_generated_native_outputs','full_CLI_outputs','full_saved_replay_outputs','native_receipt_expectation_cases'):
            for value in (True,False,None,'24',24.0,-1):
                changed=fixture();changed[key]=value
                with self.subTest(key=key,value=value),self.assertRaises(ValueError):CHECK.checked_entry(changed,'report')
        for key in ('input_count','comparison_count','scalar_comparison_count','mismatches'):
            for value in (False,None,'0',0.0,-1):
                changed=fixture();changed['full_generated_corpus'][key]=value
                with self.subTest(key=key,value=value),self.assertRaises(ValueError):CHECK.checked_entry(changed,'report')
        for key,value in (('entry','ext'),('schema','wrong'),('decision','PASS'),('generated_sha256','wrong')):
            changed=fixture();changed[key]=value
            with self.subTest(key=key),self.assertRaises(ValueError):CHECK.checked_entry(changed,'report')

    def test_raw_span_change_preserves_unaffected_bytes(self):
        raw=b' {"unchanged":"\\uac00", "result":{"program":{},"runtime":{"traces":[{"deliveries":[{"actual":"old"}]}]}}} \n'
        path=['result','runtime','traces',0,'deliveries',0,'actual']
        text,(first,last)=CHECK.value_span(raw,path)
        changed=CHECK.changed_value(raw,path,'new')
        self.assertEqual(changed,text[:first].encode()+b'"new"'+text[last:].encode())
        self.assertEqual(json.loads(changed)['unchanged'],json.loads(raw)['unchanged'])
        for value in (False,'wrong',[]):
            changed=CHECK.wrong_pure_calls(raw,value)
            decoded=json.loads(changed);self.assertEqual(decoded['result']['program']['pure_calls'],value)
            del decoded['result']['program']['pure_calls'];self.assertEqual(decoded,json.loads(raw))

    def test_strict_data_json_refuses_wrong_numbers_duplicates_and_trailing(self):
        with tempfile.TemporaryDirectory() as d:
            path=Path(d)/'data.json'
            for raw in (b'{"n":0.125}',b'{"n":NaN}',b'{"n":1,"n":2}',b'{} {}',b'{"x":"\xff"}'):
                path.write_bytes(raw)
                with self.subTest(raw=raw),self.assertRaises(ValueError):CHECK.load(path)
            path.write_bytes(b'{"count":9841}')
            self.assertEqual(CHECK.load(path),{'count':9841})

    def test_native_cleanup_requires_exact_owned_subtree_and_identity(self):
        with tempfile.TemporaryDirectory() as d:
            base=Path(d).resolve()
            a=CHECK.Attempt.__new__(CHECK.Attempt);a.work=base/'owned';a.work.mkdir();(a.work/'tmp').mkdir();a.out=base/'proof';a.out.mkdir();a.groups=set()
            checked=a.out/'checked.json';checked.write_bytes(b'{}')
            native=a.work/'tmp/gooo-path-native-positive';native.mkdir();(native/'cache').write_bytes(b'cached')
            info=native.stat();full={'owned_native_work':{'path':str(native),'device':info.st_dev,'inode':info.st_ino},'owned_native_work_removed':False}
            wrong=copy.deepcopy(full);wrong['owned_native_work']['inode']+=1
            with self.assertRaises(ValueError):a.cleanup_native_work('wrong',wrong,checked)
            outside=base/'outside';outside.mkdir();i=outside.stat();wrong=copy.deepcopy(full);wrong['owned_native_work']={'path':str(outside),'device':i.st_dev,'inode':i.st_ino}
            with self.assertRaises(ValueError):a.cleanup_native_work('outside',wrong,checked)
            alias=base/'tmp-alias';alias.symlink_to(a.work/'tmp',target_is_directory=True)
            wrong=copy.deepcopy(full);wrong['owned_native_work']['path']=str(alias/native.name)
            with self.assertRaises(ValueError):a.cleanup_native_work('alias',wrong,checked)
            self.assertTrue(native.exists())
            a.groups={999}
            with mock.patch.object(CHECK,'group_exists',return_value=True),self.assertRaises(RuntimeError):a.cleanup_native_work('live',full,checked)
            a.groups=set();ledger=a.cleanup_native_work('accepted',full,checked)
            self.assertFalse(native.exists());self.assertTrue(outside.exists());self.assertEqual(CHECK.load(Path(ledger['path']))['logical_bytes_removed'],6)

    def test_actual_process_helper_limits_decimals_and_exact_exit(self):
        with tempfile.TemporaryDirectory() as d:
            path=Path(d)/'process.json'
            path.write_bytes(b'{"exit_code":0,"process_reaped":true,"context_error":null,"elapsed_seconds":0.125}')
            process=CHECK.load_process(path);CHECK.validate_native_process(process)
            self.assertEqual(str(process['elapsed_seconds']),'0.125')
            with self.assertRaises(ValueError):CHECK.load(path)
            for raw in (b'{"exit_code":0.0}',b'{"bytes":0.125}',b'{"elapsed_seconds":NaN}',b'{"x":1,"x":2}',b'{} {}'):
                path.write_bytes(raw)
                with self.subTest(raw=raw),self.assertRaises(ValueError):CHECK.load_process(path)
            for value in (False,True,0.0,None,'0',1):
                process={'exit_code':value,'process_reaped':True,'context_error':None}
                with self.subTest(exit_code=value),self.assertRaises(ValueError):CHECK.validate_native_process(process)

    def test_final_audit_rehashes_earlier_stage_stdout(self):
        with tempfile.TemporaryDirectory() as d:
            a=CHECK.Attempt.__new__(CHECK.Attempt);a.out=Path(d)
            oracle=a.out/'oracle.json';oracle.write_bytes(b'{}');a.oracle_ref=CHECK.ref(oracle)
            freeze=a.out/'freeze.json';freeze.write_bytes(b'{}');a.freeze=CHECK.ref(freeze)
            stdout=a.out/'earlier.stdout';stderr=a.out/'earlier.stderr';stdout.write_bytes(b'old');stderr.write_bytes(b'')
            started=a.out/'earlier.started.json';started_value={'status':'STARTED','name':'earlier','pid':123,'owned_pgid':123,'argv':['fake'],'cwd':d,'source_freeze_before':a.freeze,'timeout_seconds':30,'expected_exit_code':0};started.write_text(json.dumps(started_value))
            process={**started_value,'status':'COMPLETED','started_ledger':CHECK.ref(started),'primary_error':None,'secondary_errors':[],'termination_errors':[],'owned_group_state':{'state':'absent','error':None},'expected_exit_code':0,'exit_code':0,'process_reaped':True,'owned_group_remaining':False,'abort':None,
                'source_tools_unchanged_after':True,'source_freeze_before':a.freeze,'source_freeze_after':a.freeze,
                'stdout':CHECK.ref(stdout),'stderr':CHECK.ref(stderr),'elapsed_seconds':0.125}
            ledger=a.out/'earlier.process.json';ledger.write_text(json.dumps(process));a.stages=[CHECK.ref(ledger)]
            a.audit_retained([])
            stdout.write_bytes(b'changed')
            with self.assertRaises(ValueError):a.audit_retained([])


    def test_live_scan_only_ignores_disappearing_owned_descendants(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);item=root/'go_asm.h'
            def vanished_walk(*args,**kwargs):
                item.unlink()
                yield str(root),[],[item.name]
            item.write_bytes(b'temporary')
            with mock.patch.object(CHECK.os,'walk',side_effect=vanished_walk):
                self.assertEqual(CHECK.logical(root,active=True),0)
            item.write_bytes(b'temporary')
            with mock.patch.object(CHECK.os,'walk',side_effect=vanished_walk),self.assertRaises(FileNotFoundError):CHECK.logical(root)
            with self.assertRaises(FileNotFoundError):CHECK.logical(root/'missing-root',active=True)
            owned=root/'vanishing-owned-root';owned.mkdir()
            def vanished_root(*args,**kwargs):
                owned.rmdir()
                return iter(())
            with mock.patch.object(CHECK.os,'walk',side_effect=vanished_root),self.assertRaises(FileNotFoundError):CHECK.logical(owned,active=True)
            item.write_bytes(b'temporary');real_lstat=Path.lstat
            def denied_lstat(path,*args,**kwargs):
                if path==item:raise PermissionError(errno.EPERM,'not absence',str(item))
                return real_lstat(path,*args,**kwargs)
            with mock.patch.object(Path,'lstat',denied_lstat),self.assertRaises(PermissionError):CHECK.logical(root,active=True)
            for error in (PermissionError(errno.EPERM,'walk denied',str(root/'child')),OSError(errno.EIO,'I/O',str(root/'child')),FileNotFoundError(errno.ENOENT,'root lost',str(root))):
                def bad_walk(*args,**kwargs):
                    kwargs['onerror'](error)
                    return iter(())
                with self.subTest(error=error),mock.patch.object(CHECK.os,'walk',side_effect=bad_walk),self.assertRaises(type(error)):CHECK.logical(root,active=True)
            item.unlink();item.symlink_to('target')
            with self.assertRaisesRegex(RuntimeError,'symlink'):CHECK.logical(root,active=True)

    def fake_attempt(self,root):
        a=CHECK.Attempt.__new__(CHECK.Attempt);a.repo=root;a.out=root/'proof';a.out.mkdir();a.work=root/'work';a.work.mkdir()
        a.owner=a.work.stat();a.env={};a.started=time.monotonic();a.overall=300;a.groups=set();a.children={};a.reaped=set();a.stages=[];a.secondary_errors=[]
        a.pending_process=None;a.last_group_observations=[];a.work_cleanup_complete=False
        freeze=a.out/'freeze.json';freeze.write_bytes(b'{}');a.freeze=CHECK.ref(freeze);a.frozen=mock.Mock();a.budget=mock.Mock()
        return a

    def test_started_ledger_precedes_poll_failure_and_eperm_never_masks_primary(self):
        with tempfile.TemporaryDirectory() as d:
            a=self.fake_attempt(Path(d));primary=RuntimeError('primary scan failure');child=mock.Mock(pid=321)
            child.poll.return_value=None;child.wait.return_value=-15
            def budget(active=False):
                if active:
                    started=CHECK.load_process(a.out/'probe.started.json')
                    self.assertEqual((started['status'],started['pid'],started['owned_pgid']),('STARTED',321,321))
                    raise primary
            a.budget=budget
            denied=PermissionError(errno.EPERM,'unknown group ownership')
            with mock.patch.object(CHECK.subprocess,'Popen',return_value=child),mock.patch.object(CHECK.os,'killpg',side_effect=denied),mock.patch.object(CHECK.time,'sleep'):
                with self.assertRaises(RuntimeError) as caught:a.invoke('probe',['fake'])
                self.assertIs(caught.exception,primary)
                child.wait.assert_called_once();self.assertIn('timeout',child.wait.call_args.kwargs)
                process=CHECK.load_process(a.out/'probe.process.json')
                self.assertEqual(process['primary_error']['repr'],repr(primary));self.assertEqual(process['owned_group_state']['state'],'unknown')
                self.assertIsNone(process['owned_group_remaining']);self.assertTrue(process['process_reaped']);self.assertEqual(process['exit_code'],-15)
                self.assertEqual([x['operation'] for x in process['termination_errors']],['group-probe-before','signal','signal','group-probe-after'])
                self.assertTrue(process['secondary_errors']);self.assertNotEqual(process['status'],'COMPLETED')
                with mock.patch.object(CHECK.shutil,'rmtree') as remove,self.assertRaisesRegex(RuntimeError,'ownership unknown'):a.cleanup()
                remove.assert_not_called();self.assertTrue(a.work.exists())

    def test_started_ledger_write_failure_still_bounded_reaps_started_child(self):
        with tempfile.TemporaryDirectory() as d:
            a=self.fake_attempt(Path(d));child=mock.Mock(pid=654);child.poll.return_value=None;child.wait.return_value=0
            primary=OSError(errno.ENOSPC,'started ledger full');real_save=CHECK.save_new
            def save(path,value):
                if path.name.endswith('.started.json'):raise primary
                return real_save(path,value)
            with mock.patch.object(CHECK.subprocess,'Popen',return_value=child),mock.patch.object(CHECK.os,'killpg',side_effect=ProcessLookupError()),mock.patch.object(CHECK,'save_new',side_effect=save):
                with self.assertRaises(OSError) as caught:a.invoke('ledger',['fake'])
            self.assertIs(caught.exception,primary);child.wait.assert_called_once()
            self.assertEqual(CHECK.load_process(a.out/'ledger.process.json')['pid'],654)
            self.assertTrue(CHECK.load_process(a.out/'ledger.process.json')['process_reaped'])

    def test_primary_survives_cleanup_and_failure_ledger_errors(self):
        with tempfile.TemporaryDirectory() as d:
            a=self.fake_attempt(Path(d));a.owns_out=True;a.prepare=mock.Mock();primary=RuntimeError('original body failure')
            a.run=mock.Mock(side_effect=primary);a.cleanup=mock.Mock(side_effect=PermissionError(errno.EPERM,'unknown ownership'))
            with mock.patch.object(CHECK,'Attempt',return_value=a):
                with self.assertRaises(RuntimeError) as caught:CHECK.execute_attempt(object())
            self.assertIs(caught.exception,primary)
            failure=CHECK.load_process(a.out/'failure.json');self.assertEqual(failure['primary_error']['repr'],repr(primary))
            self.assertEqual(failure['secondary_errors'][0]['operation'],'owned-work-cleanup');self.assertFalse(failure['owned_work_cleanup_confirmed'])
            self.assertTrue(a.work.exists());self.assertFalse((a.out/'summary.json').exists())
            (a.out/'failure.json').unlink();stderr=io.StringIO()
            with mock.patch.object(CHECK,'Attempt',return_value=a),mock.patch.object(CHECK,'save_new',side_effect=PermissionError(errno.EPERM,'ledger denied')),mock.patch.object(CHECK.sys,'stderr',stderr):
                with self.assertRaises(RuntimeError) as caught:CHECK.execute_attempt(object())
            self.assertIs(caught.exception,primary)
            fallback=json.loads(stderr.getvalue());self.assertEqual(fallback['primary_error']['repr'],repr(primary))
            self.assertEqual(fallback['secondary_errors'][-1]['operation'],'failure-ledger-write')


    def test_terminal_ledger_failure_preserves_failed_pending_row_and_primary(self):
        for operation in ('save','ref'):
            with self.subTest(operation=operation),tempfile.TemporaryDirectory() as d:
                a=self.fake_attempt(Path(d));child=mock.Mock(pid=987);child.poll.return_value=0;child.wait.return_value=0
                primary=OSError(errno.ENOSPC,'terminal '+operation+' failure');real_save=CHECK.save_new;real_ref=CHECK.ref
                def save(path,value):
                    if operation=='save' and path.name.endswith('.process.json'):raise primary
                    return real_save(path,value)
                def reference(path):
                    if operation=='ref' and path.name.endswith('.process.json'):raise primary
                    return real_ref(path)
                with mock.patch.object(CHECK.subprocess,'Popen',return_value=child),mock.patch.object(CHECK.os,'killpg',side_effect=ProcessLookupError()),mock.patch.object(CHECK,'save_new',side_effect=save),mock.patch.object(CHECK,'ref',side_effect=reference):
                    with self.assertRaises(OSError) as caught:a.invoke('terminal',['fake'])
                self.assertIs(caught.exception,primary);child.wait.assert_called_once()
                self.assertEqual(a.pending_process['status'],'FAILED');self.assertEqual(a.pending_process['primary_error']['repr'],repr(primary))
                self.assertEqual(a.pending_process['secondary_errors'][-1]['operation'],'terminal-ledger-write')
                self.assertIn(987,a.reaped);self.assertEqual(a.stages,[])

    def test_absent_group_with_unreaped_child_never_allows_cleanup(self):
        with tempfile.TemporaryDirectory() as d:
            a=self.fake_attempt(Path(d));a.reaped.add(741);child=mock.Mock(pid=741);child.poll.return_value=0
            child.wait.side_effect=subprocess.TimeoutExpired('fake',0)
            with mock.patch.object(CHECK.subprocess,'Popen',return_value=child),mock.patch.object(CHECK.os,'killpg',side_effect=ProcessLookupError()):
                with self.assertRaisesRegex(RuntimeError,'completion or ownership'):a.invoke('unreaped',['fake'])
                process=CHECK.load_process(a.out/'unreaped.process.json')
                self.assertFalse(process['process_reaped']);self.assertFalse(process['owned_group_remaining']);self.assertNotIn(741,a.reaped)
                with mock.patch.object(CHECK.shutil,'rmtree') as remove,self.assertRaisesRegex(RuntimeError,'child unreaped'):a.cleanup()
                remove.assert_not_called()
            self.assertTrue(a.work.exists());self.assertFalse((a.out/'work-cleanup.json').exists())


if __name__=='__main__':unittest.main(verbosity=2)
