"""Focused orchestration controls; no compiler, API or subprocess is launched."""
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch
import gzip
import hashlib
import importlib.util
import io
import itertools
import json
import os
import shutil
import sys
import tempfile
import time
import types
import unittest

sys.dont_write_bytecode=True
REPO=Path(__file__).resolve().parents[1]
spec=importlib.util.spec_from_file_location('policy_runner',REPO/'tools/check-redelivery-policy.py')
m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
m.bind_helper(REPO)


class TransportControls(unittest.TestCase):
    def test_kind_only_preserves_other_bytes(self):
        raw=b'{"prefix":"\\u003c\\u00e9","n":1,"tail":[false]}'
        changed=m.raw_token(raw,['n'],'1.0')
        self.assertEqual(changed,b'{"prefix":"\\u003c\\u00e9","n":1.0,"tail":[false]}')
        self.assertEqual(m.raw_token(raw,['n'],'true'),raw.replace(b':1,',b':true,'))
        with self.assertRaisesRegex(ValueError,'type'):m.raw_token(raw,['n'],'2')

    def test_remove_member_preserves_surroundings(self):
        for raw,key,want in [(b'{"a":1,"b":2}', 'a',b'{"b":2}'),(b'{"a":1,"b":2}', 'b',b'{"a":1}'),(b'{"a":1,"b":2,"c":3}','b',b'{"a":1,"c":3}')]:
            self.assertEqual(m.without_member(raw,[],key),want)

    def test_original16_mapping_bounds_and_kind(self):
        root=REPO/'examples/automatic-redelivery';contract=m.load(root/'contract.json');witness=m.load(root/'manual-witnesses.json');native=m.load(root/'manual-inputs.json')
        batches=m.batch_documents(native,witness,contract,(root/'main.gooo').stat().st_size)
        self.assertEqual([(b['offset'],b['count']) for b in batches],[(0,8),(8,8)])
        self.assertEqual((root/'manual-inputs.json').stat().st_size,8438)
        for b in batches:self.assertLessEqual(max(b['raw_bytes'],b['translated_upper_bytes']),8192)
        native['inputs'][0][m.ENTRY+'.input7']=True
        with self.assertRaisesRegex(ValueError,'mapping'):m.batch_documents(native,witness,contract,5748)

    def test_wire_manifest_raw_bindings_and_valid_bases(self):
        d=REPO/'cmd/check-redelivery-policy/testdata/wire';manifest=m.load(d/'manifest.json')
        self.assertEqual(len(manifest['cases']),36);self.assertEqual(len({r['file'] for r in manifest['cases']}),36)
        named=json.loads(manifest['base_named']);native=json.loads(manifest['base_native'])['inputs']
        names=[x['name'] for x in m.load(REPO/'examples/automatic-redelivery/contract.json')['input_order']]
        self.assertTrue(m.typed_equal(native,[{m.ENTRY+'.input'+str(i):named[n] for i,n in enumerate(names)}]))
        for r in manifest['cases']:
            raw=(d/r['file']).read_bytes();self.assertEqual(len(raw),r['bytes']);self.assertEqual(hashlib.sha256(raw).hexdigest(),r['sha256'])
        self.assertFalse(m.typed_equal(1,True));self.assertFalse(m.typed_equal(1,1.0))


class LifecycleControls(unittest.TestCase):
    def attempt(self,root):
        a=m.Attempt.__new__(m.Attempt);a.work=root/'work';a.out=root/'out';a.work.mkdir();a.out.mkdir()
        a.repo=root;a.started=time.monotonic();a.overall=300;a.stages=[];a.groups=set();a.children={};a.reaped=set();a.env={}
        a.pending_process=None;a.secondary_errors=[];a.frozen=lambda:None
        p=a.out/'freeze.json';p.write_text('{}');a.freeze=m.ref(p)
        a.invoke=types.MethodType(m.H.Attempt.invoke,a)
        return a

    def test_policy_caps_active_and_terminal(self):
        for suffix,cap in [('.stdout',m.STDOUT_CAP),('.stderr',m.STDERR_CAP)]:
            for active in (False,True):
                with tempfile.TemporaryDirectory() as d:
                    a=self.attempt(Path(d));(a.out/('sample'+suffix)).write_bytes(b'x'*(cap+1))
                    with self.assertRaisesRegex(RuntimeError,'Policy stage output cap'):a.budget(active)

    def test_actual_reused_invoke_catches_fast_exit_cap(self):
        class Child:
            pid=424242
            def poll(self):return 0
            def wait(self,timeout=None):return 0
        with tempfile.TemporaryDirectory() as d:
            a=self.attempt(Path(d))
            def launch(*args,**kwargs):kwargs['stdout'].write(b'x'*(m.STDOUT_CAP+1));kwargs['stdout'].flush();return Child()
            with patch.object(m.H.subprocess,'Popen',launch),patch.object(m.H,'group_state',return_value={'state':'absent','error':None}):
                with self.assertRaisesRegex(RuntimeError,'Policy stage output cap'):a.invoke('fast',['must-not-execute'])
            p=m.load_process(a.out/'fast.process.json')
            self.assertTrue(p['process_reaped']);self.assertEqual(p['owned_group_state']['state'],'absent')
            self.assertEqual(m.load_process(Path(p['started_ledger']['path']))['status'],'STARTED')

    def test_cleanup_error_does_not_mask_primary(self):
        with tempfile.TemporaryDirectory() as d:
            out=Path(d);fake=SimpleNamespace(owns_out=True,out=out,work=out,stages=[],secondary_errors=[])
            fake.prepare=lambda:None
            def fail():raise ValueError('original-stage')
            def cleanup():raise PermissionError('ownership-unknown')
            fake.run=fail;fake.cleanup=cleanup
            with patch.object(m,'Attempt',return_value=fake):
                with self.assertRaisesRegex(ValueError,'original-stage'):m.execute_attempt(None)
            failure=json.loads((out/'failure.json').read_bytes())
            self.assertEqual(failure['primary_error']['type'],'ValueError')
            self.assertEqual(failure['secondary_errors'][0]['operation'],'owned-work-cleanup')

    def test_failure_record_and_stderr_errors_preserve_primary(self):
        with tempfile.TemporaryDirectory() as d:
            fake=SimpleNamespace(owns_out=True,out=Path(d),work=None,stages=[],secondary_errors=[])
            fake.prepare=lambda:None;fake.cleanup=lambda:None
            def fail():raise ValueError('original-stage')
            def no_stderr(*args):raise BrokenPipeError('stderr unavailable')
            fake.run=fail
            with patch.object(m,'Attempt',return_value=fake),patch.object(m,'save_new',side_effect=OSError('cannot write failure')),patch.object(m.sys,'stderr',SimpleNamespace(write=no_stderr)):
                with self.assertRaisesRegex(ValueError,'original-stage'):m.execute_attempt(None)

    def test_retained_tamper_bytes_cannot_change(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);freeze=root/'freeze.json';freeze.write_text('{}')
            tamper=root/'tamper.json';tamper.write_text('{"changed":true}')
            a=m.Attempt.__new__(m.Attempt);a.sources=[];a.tools=[];a.freeze=m.ref(freeze)
            a.runtime_refs=[m.ref(tamper)];a.producers=[];a.module_refs=[]
            a.frozen(False)
            tamper.write_text('{"changed":false}')
            with self.assertRaisesRegex(ValueError,'Frozen public source/tool/evidence changed'):a.frozen(False)


    def source_repository(self,root):
        # Copy only the public preflight inputs. These bytes are never built or
        # executed; the two tool files below are deliberately inert placeholders.
        names=set(m.CORE)|{'cmd/check-redelivery-policy/'+n for n in m.CHECKER}|{
            'tools/redelivery-native-source/'+n+'.txt' for n in m.TEMPLATES}|{
            'tools/check-redelivery-policy.py','tools/check-redelivery-policy_test.py',
            'tools/check-path-facts.py','cmd/check-path-facts/result-profile-83928fa.json',
            'cmd/check-path-facts/result-profile-go1273.json',
            'cmd/check-path-facts/testdata/current-go1273-execution.json',
            'cmd/check-path-facts/testdata/current-go1273-execution.provenance.json',
            'cmd/check-literal-text-report/testdata/pinned-original-execution.json',
            'internal/jsonkeys/keys.go','internal/jsonkeys/keys_test.go',
            'internal/jsonkeys/keys_byte_walker_test.go','LICENSE','go.mod','upstream.json',
            'cmd/check-redelivery-policy/testdata/wire/manifest.json'}
        if (REPO/'go.sum').exists():names.add('go.sum')
        wire=m.load(REPO/'cmd/check-redelivery-policy/testdata/wire/manifest.json')
        names.update('cmd/check-redelivery-policy/testdata/wire/'+x['file'] for x in wire['cases'])
        for name in names:
            target=root/name;target.parent.mkdir(parents=True,exist_ok=True)
            shutil.copyfile(REPO/name,target)
        go,compiler=root/'inert-go',root/'inert-compiler'
        go.write_bytes(b'not an executable; preflight may only hash this file')
        compiler.write_bytes(b'not an executable; preflight may only hash this file')
        return SimpleNamespace(repository=root,out=root/'out',go=go,compiler=compiler,timeout=300)

    def prepared_source_attempt(self,root):
        args=self.source_repository(root)
        work=root/'owned-work';work.mkdir()
        with patch.object(m.H.subprocess,'Popen',side_effect=AssertionError('preflight must not launch')):
            attempt=m.Attempt(args)
            with patch.object(m.tempfile,'mkdtemp',return_value=str(work)):
                attempt.prepare()
        return attempt

    def test_source_preflight_freezes_all_three_jsonkey_files(self):
        try:
            with tempfile.TemporaryDirectory() as d:
                root=Path(d).resolve();attempt=self.prepared_source_attempt(root)
                freeze=m.load(attempt.out/'source-freeze.json')
                want={'keys.go','keys_test.go','keys_byte_walker_test.go'}
                files=[x for x in freeze['files'] if Path(x['path']).parent==root/'internal/jsonkeys']
                self.assertEqual({Path(x['path']).name for x in files},want)
                self.assertEqual(len(files),3)
                for item in files:self.assertEqual(item,m.ref(Path(item['path'])))
                attempt.frozen(False)
                self.assertEqual(attempt.stages,[]);self.assertEqual(attempt.children,{})
        finally:m.bind_helper(REPO)

    def test_source_preflight_missing_or_extra_jsonkey_file_refused(self):
        try:
            for mutation in ('missing','extra'):
                with self.subTest(mutation=mutation),tempfile.TemporaryDirectory() as d:
                    root=Path(d).resolve();args=self.source_repository(root)
                    if mutation=='missing':(root/'internal/jsonkeys/keys_byte_walker_test.go').unlink()
                    else:(root/'internal/jsonkeys/unlisted.go').write_text('package jsonkeys\n')
                    expected='Public source must be canonical regular file' if mutation=='missing' else 'Public code source roster differs'
                    with patch.object(m.H.subprocess,'Popen',side_effect=AssertionError('preflight must not launch')):
                        with self.assertRaisesRegex(ValueError,expected):m.Attempt(args)
                    self.assertFalse(args.out.exists())
        finally:m.bind_helper(REPO)

    def test_source_preflight_file_or_directory_symlink_refused(self):
        try:
            for mutation in ('file','directory'):
                with self.subTest(mutation=mutation),tempfile.TemporaryDirectory() as d:
                    root=Path(d).resolve();args=self.source_repository(root)
                    if mutation=='file':
                        source=root/'internal/jsonkeys/keys_byte_walker_test.go'
                        target=root/'original-walker';source.rename(target);source.symlink_to(target)
                    else:
                        source=root/'internal/jsonkeys';target=root/'original-jsonkeys'
                        source.rename(target);source.symlink_to(target,target_is_directory=True)
                    with patch.object(m.H.subprocess,'Popen',side_effect=AssertionError('preflight must not launch')):
                        with self.assertRaisesRegex(ValueError,'Public source must be canonical regular file'):m.Attempt(args)
                    self.assertFalse(args.out.exists())
        finally:m.bind_helper(REPO)

    def test_source_freeze_rejects_replaced_jsonkey_file(self):
        try:
            with tempfile.TemporaryDirectory() as d:
                root=Path(d).resolve();attempt=self.prepared_source_attempt(root)
                source=root/'internal/jsonkeys/keys_byte_walker_test.go'
                replacement=root/'replacement';replacement.write_bytes(source.read_bytes()+b'\n// changed after freeze\n')
                replacement.replace(source)
                self.assertEqual({p.name for p in source.parent.iterdir()},{'keys.go','keys_test.go','keys_byte_walker_test.go'})
                with self.assertRaisesRegex(ValueError,'Frozen public source/tool/evidence changed'):attempt.frozen(False)
                self.assertEqual(attempt.stages,[]);self.assertEqual(attempt.children,{})
        finally:m.bind_helper(REPO)


class RowsControls(unittest.TestCase):
    def test_row_kind_reaches_intended_gate_before_population(self):
        contract=m.load(REPO/'examples/automatic-redelivery/contract.json')
        inputs={f['name']:f['domain'][0] for f in contract['input_order']}
        actual={f['name']:inputs[f['name']] if f['name'] in inputs else {'string':'x','boolean':False,'integer':0}[f['type']] for f in contract['output_fields']}
        for change,diagnostic in [('kind','scalar kind'),('echo','echoes'),('missing','fifteen'),('order','index/shape/order')]:
            row={'index':0,'inputs':dict(inputs),'actual':dict(actual)}
            if change=='kind':row['actual']['attempts_if_claimed']=True
            elif change=='echo':row['actual']['method']='altered'
            elif change=='missing':del row['actual']['reason']
            else:row['index']=1
            buffer=io.BytesIO()
            with gzip.GzipFile(fileobj=buffer,mode='wb',mtime=0,compresslevel=9) as z:z.write((json.dumps(row)+'\n').encode())
            with tempfile.TemporaryDirectory() as d:
                path=Path(d)/'rows.gz';path.write_bytes(buffer.getvalue())
                with self.assertRaisesRegex(ValueError,diagnostic):m.check_rows(path,contract)


if __name__=='__main__':unittest.main()
