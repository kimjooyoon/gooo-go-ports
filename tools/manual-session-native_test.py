#!/usr/bin/env python3
"""Fault controls use Python stand-ins only; never Go/Gooo or native fixtures."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time
import unittest
from unittest import mock

HERE=Path(__file__).resolve().parent
SPEC=importlib.util.spec_from_file_location('manual_session_native',HERE/'manual-session-native.py')
M=importlib.util.module_from_spec(SPEC); SPEC.loader.exec_module(M)
FAST=dict(poll=.01,sample=.02,close=.15,term=.15,kill=.3,drain=.3,allocated=1<<30,free=0)
FAKE=r'''
import json,os,signal,subprocess,sys,threading,time
from pathlib import Path
work=Path(sys.argv[1]); mode=sys.argv[2]; owner=work/'records/owned'; transport=owner/'session-transport'
transport.mkdir(parents=True); (transport/'native-work').mkdir()
child=subprocess.Popen([sys.executable,'-c','import time; time.sleep(60)'],cwd=owner,preexec_fn=os.setpgrp)
threading.Thread(target=child.wait,daemon=True).start()
doc=dict(schema='gooo/native-session-process-start/v1',pid=child.pid,
         argv=[sys.executable,'package','session'],compiler=sys.executable,configured_go_tool=sys.executable,
         cwd=str(owner),native_work=str(transport/'native-work'))
(transport/'started.json').write_text(json.dumps(doc))
print('owner ready',flush=True)
if mode=='exit':
    time.sleep(.12); os._exit(7)
if mode=='normal':
    child.terminate(); child.wait(); sys.exit(0)
signal.signal(signal.SIGINT,signal.SIG_IGN); signal.signal(signal.SIGTERM,signal.SIG_IGN)
while True: time.sleep(.01)
'''


def posix_probe(pid, owner, compiler, verifier):
    # Tests own these Python stand-ins. Production always uses the stricter
    # Linux proc argv/exe/cwd/start-time proof; there is no CLI probe override.
    try:
        return dict(start=str(pid),pgid=os.getpgid(pid),sid=os.getsid(pid),state='S',binding=True)
    except ProcessLookupError:
        return None


@unittest.skipUnless(os.name=='posix','requires isolated Unix process groups')
class ControllerFaultTests(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory(prefix='manual-session-fault-')
        self.work=Path(self.temp.name).resolve(); self.fake=self.work/'fake-verifier.py'; self.fake.write_text(FAKE)
        self.children=[]
    def tearDown(self):
        # Only PIDs created by these controls or recorded by their Python stand-ins.
        ids=[]
        for p in self.work.glob('records/*/session-transport/started.json'):
            try: ids.append(json.loads(p.read_text())['pid'])
            except (OSError,ValueError,KeyError): pass
        for p in (self.work/'diagnostics').glob('*-process.json') if (self.work/'diagnostics').exists() else []:
            try: ids.append(json.loads(p.read_text()).get('pid'))
            except (OSError,ValueError): pass
        for pid in ids:
            if type(pid) is int and pid>1 and pid!=os.getpid():
                try: os.killpg(pid,signal.SIGKILL)
                except ProcessLookupError: pass
                try: os.waitpid(pid,os.WNOHANG)
                except ChildProcessError: pass
        for p in self.children:
            if p.poll() is None: p.kill()
            p.wait()
        self.temp.cleanup()
    def native(self,mode,**kwargs):
        return M.run_command(self.work,'native',[sys.executable,str(self.fake),str(self.work),mode],2,
                             native=dict(compiler=sys.executable,go_tool=sys.executable),limits=FAST,probe=posix_probe,**kwargs)
    def assert_clean(self,record):
        self.assertTrue(record['waited']); self.assertFalse(record['verifier_group_remaining'])
        self.assertTrue(record['streams_joined'])
        for group in record.get('owner_cleanup',{}).get('groups',[]):
            self.assertFalse(group['group_remaining'],group)
    def test_small_capture_has_complete_hash_and_wait(self):
        payload=b'actual stdout\n'
        code,r=M.run_command(self.work,'small',[sys.executable,'-c','print("actual stdout")'],2,limits=FAST)
        self.assertEqual(code,0); self.assert_clean(r)
        self.assertEqual(r['stdout']['observed_sha256'],hashlib.sha256(payload).hexdigest())
        self.assertTrue(r['stdout']['hash_complete']); self.assertFalse(r['stdout']['clipped'])
    def test_large_stream_is_prefix_bounded_before_collection(self):
        with mock.patch.object(M,'LOG_CAP',4096):
            code,r=M.run_command(self.work,'large',[sys.executable,'-c','import os; os.write(1,b"x"*1048576)'],2,limits=FAST)
        self.assertEqual(code,1); self.assertIn('prefix',r['first_error']); self.assert_clean(r)
        self.assertTrue(r['stdout']['clipped']); self.assertEqual(r['stdout']['retained_bytes'],4096)
        self.assertLessEqual(Path(r['stdout']['path']).stat().st_size,4096)
    def test_deadline_forces_ignoring_child_and_waits(self):
        code,r=M.run_command(self.work,'deadline',[sys.executable,'-c',
            'import signal,time; signal.signal(signal.SIGINT,signal.SIG_IGN); signal.signal(signal.SIGTERM,signal.SIG_IGN); time.sleep(60)'],.15,limits=FAST)
        self.assertEqual(code,1); self.assertEqual(r['first_error'],'command deadline'); self.assert_clean(r)
        self.assertTrue(r['forced_verifier_group_cleanup'])
    def test_sampling_exception_cleans_separately_grouped_owner(self):
        def sample():
            if list(self.work.glob('records/*/session-transport/started.json')):
                raise RuntimeError('injected sampling failure')
            return dict(allocated_bytes=0,free_bytes=1<<40)
        code,r=self.native('ignore',sample=sample)
        self.assertEqual(code,1); self.assertIn('sampling failure',r['first_error']); self.assert_clean(r)
        self.assertIn('owner_cleanup',r,r)
        groups=r['owner_cleanup']['groups']; self.assertEqual(len(groups),1)
        self.assertTrue(groups[0]['source'].endswith('started.json')); self.assertIn('SIGTERM',groups[0]['signals'])
    def test_record_write_exception_uses_cleanup_and_fallback(self):
        tripped=[False]
        def writer(path,record):
            if not tripped[0] and list(self.work.glob('records/*/session-transport/started.json')):
                tripped[0]=True; raise OSError('injected record write failure')
            M.write_json(path,record)
        code,r=self.native('ignore',writer=writer)
        self.assertEqual(code,1); self.assertTrue(tripped[0]); self.assertIn('record write failure',r['first_error']); self.assert_clean(r)
        self.assertTrue((self.work/'diagnostics/native-fallback.json').exists())
    def test_original_forced_exit_keeps_first_error_and_started_fallback(self):
        code,r=self.native('exit')
        self.assertEqual(code,1); self.assertEqual(r['exit_code'],7)
        self.assertEqual(r['first_error'],'original exit status 7'); self.assert_clean(r)
        self.assertIn('owner_cleanup',r,r)
        self.assertEqual(len(r['owner_cleanup']['groups']),1)
        self.assertFalse((self.work/'records/owned/session-owner.json').exists())
        code,inventory=M.collect(self.work)
        self.assertEqual(code,0)
        self.assertTrue((self.work/'upload/records/owned/session-transport/started.json').exists())
    def test_sigterm_to_controller_forwards_interrupt_and_cleans_owner(self):
        child_script=self.work/'controller.py'
        child_script.write_text('''import importlib.util,json,os,sys\nfrom pathlib import Path\ns=importlib.util.spec_from_file_location('m',sys.argv[1]);m=importlib.util.module_from_spec(s);s.loader.exec_module(m)\ndef probe(pid,owner,compiler,verifier):\n try:return dict(start=str(pid),pgid=os.getpgid(pid),sid=os.getsid(pid),state='S',binding=True)\n except ProcessLookupError:return None\nlimits=json.loads(sys.argv[4]);work=Path(sys.argv[2])\ncode,r=m.run_command(work,'native',[sys.executable,sys.argv[3],str(work),'ignore'],10,native=dict(compiler=sys.executable,go_tool=sys.executable),probe=probe,limits=limits)\nsys.exit(code)\n''')
        p=subprocess.Popen([sys.executable,str(child_script),str(HERE/'manual-session-native.py'),str(self.work),str(self.fake),json.dumps(FAST)],stdout=subprocess.PIPE,stderr=subprocess.PIPE,start_new_session=True)
        self.children.append(p)
        until=time.monotonic()+3
        while not (self.work/'records/owned/session-transport/started.json').exists() and time.monotonic()<until: time.sleep(.01)
        self.assertTrue((self.work/'records/owned/session-transport/started.json').exists())
        p.send_signal(signal.SIGTERM); out,err=p.communicate(timeout=4)
        self.assertEqual(p.returncode,1,(out,err)); r=json.loads((self.work/'diagnostics/native-process.json').read_text())
        self.assertEqual(r['received_signals'],['SIGTERM']); self.assertIn('SIGTERM',r['first_error']); self.assert_clean(r)
        self.assertTrue(r['interrupt_requested'])
    def test_unrelated_group_is_refused_and_never_signaled(self):
        unrelated=subprocess.Popen([sys.executable,'-c','import time;time.sleep(60)'],start_new_session=True)
        self.children.append(unrelated)
        owner=self.work/'records/wrong'; transport=owner/'session-transport'; transport.mkdir(parents=True)
        doc=dict(schema='gooo/native-session-process-start/v1',pid=unrelated.pid,argv=[sys.executable,'package','session'],compiler=sys.executable,configured_go_tool=sys.executable,cwd=str(owner),native_work=str(transport/'native-work'))
        (transport/'started.json').write_text(json.dumps(doc))
        owners=M.Owners(self.work/'records',sys.executable,sys.executable,os.getpid(),posix_probe)
        owners.discover(); result=owners.stop(lambda seconds:time.sleep(seconds),FAST)
        self.assertEqual(result['groups'],[]); self.assertTrue(result['unresolved']); self.assertIsNone(unrelated.poll())
    def test_pid_birth_change_refuses_signal(self):
        p=subprocess.Popen([sys.executable,'-c','import time;time.sleep(60)'],start_new_session=True); self.children.append(p)
        owners=M.Owners(self.work/'records',sys.executable,sys.executable,os.getpid(),lambda *a:dict(start='new',pgid=p.pid,sid=os.getpid(),state='S',binding=True))
        owners.groups[p.pid]=dict(pid=p.pid,owner_root=str(self.work),proof=dict(start='old'),signals=[],absent_seen=False)
        owners.signal(p.pid,signal.SIGTERM)
        self.assertIn('refused_signal',owners.groups[p.pid]); self.assertIsNone(p.poll())
    def test_oversized_collection_never_reads_whole_log_or_uploads_raw(self):
        diag=self.work/'diagnostics'; diag.mkdir(); raw=diag/'huge.log'
        with raw.open('wb') as stream: stream.truncate(20<<20)
        original=Path.read_bytes
        def reject_large(path):
            if path==raw: raise AssertionError('unbounded read_bytes')
            return original(path)
        with mock.patch.object(Path,'read_bytes',reject_large): code,inventory=M.collect(self.work)
        self.assertEqual(code,1); self.assertTrue(inventory['rejected'])
        target=self.work/'upload/diagnostics/huge.log'; self.assertEqual(target.stat().st_size,1<<20)
        self.assertFalse(inventory['kept'][0]['complete']); self.assertIsNone(inventory['kept'][0]['source_sha256'])
        self.assertLessEqual(sum(p.stat().st_size for p in (self.work/'upload').rglob('*') if p.is_file()),64<<20)
    def test_process_json_retained_when_started_and_final_are_absent(self):
        transport=self.work/'records/failed/session-transport'; transport.mkdir(parents=True); (self.work/'diagnostics').mkdir()
        raw=b'{"schema":"gooo/native-session-transport-summary/v1","waited":false}\n'; (transport/'process.json').write_bytes(raw)
        code,inventory=M.collect(self.work); self.assertEqual(code,0)
        self.assertEqual((self.work/'upload/records/failed/session-transport/process.json').read_bytes(),raw)
    def test_refused_force_kill_still_bounds_wait_and_reports_incomplete(self):
        original_popen=M.subprocess.Popen
        spawned=[]
        def launch(*args,**kwargs):
            child=original_popen(*args,**kwargs); spawned.append(child); return child
        with mock.patch.object(M.subprocess,'Popen',side_effect=launch), \
             mock.patch.object(original_popen,'kill',side_effect=PermissionError('injected kill refusal')), \
             mock.patch.object(M.os,'killpg',side_effect=PermissionError('injected group refusal')):
            start=time.monotonic()
            code,r=M.run_command(self.work,'refused-kill',[sys.executable,'-c',
                'import signal,time; signal.signal(signal.SIGINT,signal.SIG_IGN); signal.signal(signal.SIGTERM,signal.SIG_IGN); time.sleep(60)'],.15,limits=FAST)
        self.children.extend(spawned)
        self.assertEqual(code,1); self.assertLess(time.monotonic()-start,2)
        self.assertFalse(r['waited']); self.assertTrue(r['verifier_group_remaining'])
        self.assertFalse(r['streams_joined']); self.assertEqual(r['first_error'],'command deadline')
    def test_capture_cli_parses_options_before_command_separator(self):
        p=subprocess.run([sys.executable,'-I','-B',str(HERE/'manual-session-native.py'),'capture','--work',str(self.work),'--label','cli','--timeout','2','--',sys.executable,'-c','print("cli")'],capture_output=True,text=True,timeout=5)
        self.assertEqual(p.returncode,0,p.stderr); self.assertTrue((self.work/'diagnostics/cli-process.json').exists())


class LinuxIdentityFixtureTests(unittest.TestCase):
    def test_proc_identity_requires_exact_argv_exe_cwd_and_session(self):
        with tempfile.TemporaryDirectory(prefix='manual-proc-fixture-') as folder:
            root=Path(folder).resolve(); owner=root/'owner'; owner.mkdir(); compiler=root/'gooo'; compiler.write_bytes(b'not executable; source fixture only')
            proc=root/'proc/200'; proc.mkdir(parents=True)
            fields=['S','100','200','100']+['0']*15+['12345']
            (proc/'stat').write_text('200 (gooo) '+' '.join(fields))
            (proc/'cmdline').write_bytes(os.fsencode(compiler)+b'\0package\0session\0')
            (proc/'cwd').symlink_to(owner,target_is_directory=True); (proc/'exe').symlink_to(compiler)
            with mock.patch.object(M.os,'getpgid',return_value=200),mock.patch.object(M.os,'getsid',return_value=100):
                good=M.proc_identity(200,owner,str(compiler),100,proc_root=root/'proc')
                self.assertTrue(good['binding']); self.assertEqual(good['start'],'12345')
                (proc/'cmdline').write_bytes(b'unrelated\0package\0session\0')
                self.assertFalse(M.proc_identity(200,owner,str(compiler),100,proc_root=root/'proc')['binding'])


if __name__=='__main__': unittest.main()
