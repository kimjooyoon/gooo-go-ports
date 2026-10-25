"""Synthetic Python-only producer binding controls; no processes are spawned."""
import copy
import importlib.util
import tempfile
import unittest
from pathlib import Path

HERE=Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('snapshot_producer_evidence',HERE/'evidence.py')
e=importlib.util.module_from_spec(spec);spec.loader.exec_module(e)


class ProducerTests(unittest.TestCase):
    def setUp(self):
        temporary=tempfile.TemporaryDirectory(prefix='snapshot-synthetic-producer-')
        self.addCleanup(temporary.cleanup);self.root=Path(temporary.name)
        self.binary=self.root/'compiler';self.binary.write_bytes(b'SYNTHETIC TEST DATA, NOT AN EXECUTABLE')
        self.identity_path=self.root/'identity.json';self.process_path=self.root/'process.json'
        profile=e.host.ACTIVE_COMPILER_PROFILE
        self.identity={'schema':'gooo/build-identity/v1','language':'gooo','compiler_source_sha':e.PIN,'vcs_revision':e.PIN,'source_status':'CLEAN_VCS','vcs_modified':'false','go_version':e.GO_VERSION,'native_go_required':e.GO_VERSION,'version':profile['compiler_version'],'build_info_observed':True,'module':profile['module'],'module_version':profile['module']['version'],'decision_runtime':profile['decision_runtime']}
        e.host.save(self.identity_path,self.identity)
        self.process={'argv':[str(self.binary),'version','--build','--json'],'exit_code':0,'first_failure':None,'cwd':str(e.ROOT),'waited':True,'streams_EOF':True,'PG_ESRCH':True,'PID':123,'PGID':123,'stdout':e.host.ref(self.identity_path)}
        e.host.save(self.process_path,self.process)
        self.producer={'binary':e.host.ref(self.binary),'identity':self.identity_path.name,'process':self.process_path.name}

    def test_exact_identity_process_and_binary_bytes_are_bound(self):
        binary,identity,paths=e.verify_producer(self.root,self.producer)
        self.assertEqual(binary,self.producer['binary']);self.assertEqual(identity,self.identity)
        self.assertEqual(paths,[self.binary,self.identity_path,self.process_path])

    def test_changed_binary_bytes_are_refused_even_at_same_path(self):
        self.binary.write_bytes(b'DIFFERENT SYNTHETIC BINARY')
        with self.assertRaises(AssertionError):e.verify_producer(self.root,self.producer)

    def test_identity_invoked_by_different_binary_is_refused(self):
        changed=copy.deepcopy(self.process);changed['argv'][0]='/different/compiler'
        e.host.save(self.process_path,changed)
        with self.assertRaises(AssertionError):e.verify_producer(self.root,self.producer)

    def test_wrong_pin_module_runtime_or_go_identity_is_refused(self):
        mutations=[('compiler_source_sha','0'*40),('vcs_revision','0'*40),('vcs_modified','true'),('source_status','UNKNOWN'),('go_version','go1.0'),('native_go_required','go1.0'),('module',{'path':'other','version':'other'}),('decision_runtime',{}),('build_info_observed',1)]
        for key,value in mutations:
            with self.subTest(key=key):
                identity=copy.deepcopy(self.identity);identity[key]=value;e.host.save(self.identity_path,identity)
                process=copy.deepcopy(self.process);process['stdout']=e.host.ref(self.identity_path);e.host.save(self.process_path,process)
                with self.assertRaises(AssertionError):e.verify_producer(self.root,self.producer)

    def test_wrong_stdout_hash_or_incomplete_process_is_refused(self):
        for key,value in [('stdout',{'sha256':'0'*64}),('exit_code',1),('exit_code',False),('waited',False),('streams_EOF',False),('PG_ESRCH',False),('first_failure','timeout'),('cwd','/wrong'),('PGID',456)]:
            with self.subTest(key=key):
                process=copy.deepcopy(self.process);process[key]=value;e.host.save(self.process_path,process)
                with self.assertRaises(AssertionError):e.verify_producer(self.root,self.producer)

    def test_binary_symlink_is_refused(self):
        alias=self.root/'alias';alias.symlink_to(self.binary)
        self.producer['binary']['path']=str(alias)
        with self.assertRaises(AssertionError):e.verify_producer(self.root,self.producer)

    def test_fresh_lineage_cannot_suppress_its_execution_claim(self):
        self.assertEqual(e.lineage_inputs({'lineage':{'kind':'FRESH_VERIFIER_RUN','fresh_native_execution':True}},self.root,self.identity),[])
        with self.assertRaises(AssertionError):
            e.lineage_inputs({'lineage':{'kind':'FRESH_VERIFIER_RUN','fresh_native_execution':False}},self.root,self.identity)


if __name__=='__main__':unittest.main()
