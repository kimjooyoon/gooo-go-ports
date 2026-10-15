"""Pure runner/row/ledger controls; no compiler, Go or network is launched."""
from pathlib import Path
from decimal import Decimal
import contextlib
import importlib.util
import io
import json
import subprocess
import sys
import tempfile
import types
import unittest
from unittest import mock

sys.dont_write_bytecode = True
HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('ipv4_runner_under_test', HERE/'check-ipv4.py')
M = importlib.util.module_from_spec(spec)
spec.loader.exec_module(M)


def write(path, value):
    path.write_text(json.dumps(value)+'\n',encoding='utf-8')
    return M.ref(path)


class RunnerTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        M.bind_modules(HERE.parent,HERE/'check-path-facts.py',True)

    def setUp(self):
        self.launch = mock.patch.object(subprocess,'Popen',side_effect=AssertionError('Producer launch forbidden in pure controls'))
        self.launch.start();self.addCleanup(self.launch.stop)

    def test_metadata_rejects_coercion_duplicates_and_trailing(self):
        self.assertEqual(M.read_json(b'{"n":1}'),{'n':1})
        self.assertIsInstance(M.read_json(b'{"elapsed":0.125}',diagnostics=True)['elapsed'],Decimal)
        for raw in (b'{"n":1.0}',b'{"n":1e0}',b'{"n":1,"\\u006e":2}',b'{} {}',b'\xff',b'{"n":NaN}'):
            with self.subTest(raw=raw),self.assertRaises((ValueError,UnicodeError)):
                M.read_json(raw)
        summary={'schema':'gooo/ipv4-public-sidecar/v1','status':'IPV4_CURRENT_NATIVE_REPLAY_GENERATED_PASS',
                 'compiler_revision':M.REV,'go_version':M.GO_VERSION,**M.COUNTS}
        M.check_counts(summary)
        for key in M.COUNTS:
            for bad in (True,float(summary[key]),summary[key]+1):
                with self.subTest(key=key,bad=bad),self.assertRaisesRegex(ValueError,'Summary count'):
                    M.check_counts({**summary,key:bad})

    def test_batch_order_original_inputs_and_finite_counts(self):
        corpus=M.load((HERE.parent/'examples/ipv4-address-facts/corpus-202.json').resolve())['cases']
        specs=M.batch_specs();self.assertEqual(len(specs),15)
        self.assertEqual(len(M.stage_names()),54);self.assertEqual(len(set(M.stage_names())),54)
        self.assertEqual(sum(len(b['indices']) for b in specs),352)
        for method in M.METHODS:
            self.assertEqual([i for b in specs if b['entry'].endswith(':'+method) for i in b['indices']],list(range(48)))
        self.assertEqual([i for b in specs[12:] for i in b['indices']],list(range(64)))
        for b in specs:
            raw=M.input_document(corpus,b);self.assertLessEqual(len(raw),8192)
            rows=M.R.validate_inputs(raw,b['entry'])
            for row,index in zip(rows,b['indices']):
                self.assertEqual(list(row.values()),[corpus[index]['inputs'][k] for k in ('a','b','c','d')])

    def protocol_fixture(self):
        oracle=[];rows=[]
        for i in range(202):
            inputs={'a':i,'b':0,'c':-1 if i%3 else 0,'d':9223372036854775807}
            expected={**inputs,**{k:bool((i+j)%2) for j,k in enumerate(M.R.FIELDS[4:])}}
            item={'index':i,'name':'synthetic-'+str(i),'cohort':'synthetic','inputs':inputs,'expected':expected}
            oracle.append(item);rows.append({**{k:item[k] for k in ('index','name','cohort','inputs')},'actual':expected})
        return oracle,rows

    def test_generated_rows_all11fields_kinds_echo_order_and_count(self):
        oracle,rows=self.protocol_fixture()
        with tempfile.TemporaryDirectory() as tmp:
            path=Path(tmp).resolve()/'rows.jsonl'
            def put(values,lf=True):
                path.write_bytes(('\n'.join(json.dumps(row,separators=(',',':')) for row in values)+ ('\n' if lf else '')).encode())
            put(rows);checked=M.check_generated_rows(path,oracle)
            self.assertEqual((checked['generated_Report_calls'],checked['typed_fields'],checked['original_echoes']),(202,2222,808))
            for field in M.R.FIELDS:
                changed=json.loads(json.dumps(rows));changed[0]['actual'][field]=None
                put(changed)
                with self.subTest(field=field),self.assertRaisesRegex(ValueError,'exact kinds'):
                    M.check_generated_rows(path,oracle)
            changed=json.loads(json.dumps(rows));changed[0]['actual']['valid']=1;put(changed)
            with self.assertRaisesRegex(ValueError,'exact kinds'):M.check_generated_rows(path,oracle)
            changed=json.loads(json.dumps(rows));changed[0]['actual']['a']=0.0;put(changed)
            with self.assertRaisesRegex(ValueError,'Noninteger metadata'):M.check_generated_rows(path,oracle)
            changed=json.loads(json.dumps(rows));changed[0]['actual']['valid']=not changed[0]['actual']['valid'];put(changed)
            with self.assertRaisesRegex(ValueError,'independent Go'):M.check_generated_rows(path,oracle)
            for changed in (rows[:-1],rows+[rows[-1]]):
                put(changed)
                with self.assertRaisesRegex(ValueError,'row count'):M.check_generated_rows(path,oracle)
            changed=list(rows);changed[0],changed[1]=changed[1],changed[0];put(changed)
            with self.assertRaisesRegex(ValueError,'input order'):M.check_generated_rows(path,oracle)
            put(rows,False)
            with self.assertRaisesRegex(ValueError,'final LF'):M.check_generated_rows(path,oracle)

    def test_optional_pair_counts_stage_suffix_and_typed_rows(self):
        self.assertEqual(M.stage_names(True)[:54],M.stage_names())
        self.assertEqual(len(M.stage_names(True)),68)
        self.assertEqual(len(set(M.stage_names(True))),68)
        summary={'schema':'gooo/ipv4-public-sidecar/v2','status':'IPV4_PREFIX_RULE_PAIR_CURRENT_NATIVE_REPLAY_GENERATED_PASS',
                 'compiler_revision':M.REV,'go_version':M.GO_VERSION,**M.COUNTS,'top_level_stages':68,
                 'include_prefix_rule_pair':True,'prefix_rule_pair':dict(M.PAIR_COUNTS)}
        M.check_counts(summary)
        for k in M.PAIR_COUNTS:
            for bad in (True,float(summary['prefix_rule_pair'][k]),summary['prefix_rule_pair'][k]+1):
                changed={**summary,'prefix_rule_pair':{**summary['prefix_rule_pair'],k:bad}}
                with self.assertRaisesRegex(ValueError,'Pair count'):M.check_counts(changed)
        rows=[];oracle=[]
        for i in range(112):
            inputs={s:{f:(i if f=='priority' else 0) for f in M.Q.RULE_FIELDS} for s in ('left','right')}
            actual={f:(False if j<6 else 0) for j,f in enumerate(M.Q.FACTS)}
            oracle.append({'index':i,'name':str(i),'inputs':inputs,'expected':actual})
            rows.append({'index':i,'name':str(i),'inputs':inputs,'actual':actual})
        with tempfile.TemporaryDirectory() as tmp:
            path=Path(tmp).resolve()/'rows.jsonl'
            def put(v):path.write_bytes(('\n'.join(json.dumps(r) for r in v)+'\n').encode())
            put(rows);M.check_pair_rows(path,oracle)
            for f in M.Q.FACTS:
                changed=json.loads(json.dumps(rows));changed[0]['actual'][f]=None;put(changed)
                with self.assertRaisesRegex(ValueError,'fact kinds'):M.check_pair_rows(path,oracle)
            put(rows[:-1])
            with self.assertRaisesRegex(ValueError,'count/LF'):M.check_pair_rows(path,oracle)
            changed=list(rows);changed[0],changed[1]=changed[1],changed[0];put(changed)
            with self.assertRaisesRegex(ValueError,'input/order'):M.check_pair_rows(path,oracle)

    def test_regular_refs_refuse_byte_changes_symlinks_and_coercion(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp).resolve();path=root/'file';path.write_bytes(b'original')
            reference=M.ref(path);M.check_ref(reference)
            path.write_bytes(b'changed')
            with self.assertRaisesRegex(ValueError,'referenced bytes'):M.check_ref(reference)
            link=root/'link';link.symlink_to(path)
            with self.assertRaisesRegex(ValueError,'Canonical'):M.ref(link)
            reference=M.ref(path);reference['bytes']=True
            with self.assertRaisesRegex(ValueError,'Malformed'):M.check_ref(reference)

    def ledger_fixture(self,root):
        freeze=write(root/'freeze.json',{'fixture':True});stages=[];specs=[]
        for i,name in enumerate(M.stage_names()):
            spec={'name':name,'argv':['synthetic-owned-driver',name],'cwd':str(root),'expected_exit_code':0};specs.append(spec)
            start={**spec,'status':'STARTED','pid':7000+i,'owned_pgid':7000+i,'source_freeze_before':freeze,
                   'timeout_seconds':1,'automatic_retries':0}
            started=write(root/(str(i)+'.started.json'),start)
            stdout=root/(str(i)+'.stdout');stdout.write_bytes(b'output\n');stderr=root/(str(i)+'.stderr');stderr.write_bytes(b'')
            terminal={**start,'status':'COMPLETED','exit_code':0,'process_reaped':True,'owned_group_remaining':False,
                      'owned_group_state':{'state':'absent','error':None},'abort':None,'primary_error':None,
                      'secondary_errors':[],'termination_errors':[],'source_tools_unchanged_after':True,'source_freeze_after':freeze,
                      'stdout':M.ref(stdout),'stderr':M.ref(stderr),'started_ledger':started,'elapsed_seconds':0.125}
            stages.append(write(root/(str(i)+'.process.json'),terminal))
        return stages,specs,freeze

    def test_ordered54_stages_bind_started_terminal_and_stream_bytes(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp).resolve();stages,specs,freeze=self.ledger_fixture(root)
            M.audit_stages(stages,specs,freeze,root)
            with self.assertRaisesRegex(ValueError,'Unique ordered'):M.audit_stages([stages[0]]*54,specs,freeze,root)
            with self.assertRaisesRegex(ValueError,'Process ledger'):M.audit_stages(stages[::-1],specs,freeze,root)
            changed=M.load(Path(stages[0]['path']),diagnostics=True);changed['exit_code']=True
            changed['elapsed_seconds']=0.125
            write(Path(stages[0]['path']),changed);modified=[M.ref(Path(stages[0]['path']))]+stages[1:]
            with self.assertRaisesRegex(ValueError,'exit_code'):M.audit_stages(modified,specs,freeze,root)
            changed['exit_code']=0;write(Path(stages[0]['path']),changed);modified[0]=M.ref(Path(stages[0]['path']))
            Path(changed['stdout']['path']).write_bytes(b'forged\n')
            with self.assertRaisesRegex(ValueError,'referenced bytes'):M.audit_stages(modified,specs,freeze,root)

    def test_module_roster_refuses_extra_compile_sources(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp).resolve();module=root/'module';module.mkdir();(module/'main.go').write_text('package main\n')
            a=object.__new__(M.Attempt);a.sources=[];a.tools=[];a.freeze=write(root/'freeze.json',{})
            a.runtime_refs=[];a.producers=[];a.module_copies=[];a.modules=[{'root':str(module),'names':['main.go']}]
            a.frozen()
            for name in ('extra.go','extra.s','extra.syso'):
                p=module/name;p.write_bytes(b'extra')
                with self.subTest(name=name),self.assertRaisesRegex(ValueError,'exact file roster'):a.frozen()
                p.unlink()

    def test_cleanup_requires_reaped_absent_and_exact_inode(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp).resolve();work=root/'owned';work.mkdir();out=root/'out';out.mkdir()
            a=object.__new__(M.Attempt);a.work=work;a.out=out;a.owner=work.stat();a.groups={741};a.reaped=set()
            a.last_group_observations=[];a.work_cleanup_complete=False
            with mock.patch.object(M.H,'group_state',return_value={'state':'absent','error':None}):
                with self.assertRaisesRegex(RuntimeError,'child unreaped'):a.cleanup()
                self.assertTrue(work.exists());a.reaped.add(741);a.cleanup()
            self.assertFalse(work.exists());self.assertTrue(a.work_cleanup_complete)
            self.assertEqual(M.load(out/'work-cleanup.json')['owned_inode'],a.owner.st_ino)

    def test_readback_cli_never_starts_producer(self):
        with mock.patch.object(sys,'argv',['check-ipv4.py','--verify-summary','/tmp/nonexistent-proof.json']),\
             mock.patch.object(M,'verify_summary',return_value={'schema':'synthetic-readback'}) as verify,\
             contextlib.redirect_stdout(io.StringIO()) as stdout:
            M.main();verify.assert_called_once_with(Path('/tmp/nonexistent-proof.json'))
            self.assertEqual(json.loads(stdout.getvalue()),{'schema':'synthetic-readback'})
        with mock.patch.object(sys,'argv',['check-ipv4.py','--verify-summary','/tmp/x','--go','/tmp/go']):
            with self.assertRaisesRegex(ValueError,'producer arguments'):M.main()


if __name__=='__main__':
    unittest.main()
