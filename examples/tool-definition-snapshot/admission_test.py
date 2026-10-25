"""Python-only decoder controls, not execution receipts or native acceptance.

All synthetic receipt/process documents below live in temporary directories.
The actual verifier must obtain its six refusal receipts from the pinned compiler.
"""
import copy
import gzip
import importlib.util
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

HERE=Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('snapshot_admission_evidence',HERE/'evidence.py')
e=importlib.util.module_from_spec(spec);spec.loader.exec_module(e)


class AdmissionTests(unittest.TestCase):
    compiler={'path':'/synthetic/no-execution/compiler'}
    def test_every_original_reference_case_has_one_boundary(self):
        native,probes=e.partition_corpus()
        self.assertEqual(len(native),83)
        self.assertEqual([p['reference_case_index'] for p,_ in probes],[51,54,65,75,80,83])
        self.assertEqual(sorted([n for n,_ in native]+[p['reference_case_index'] for p,_ in probes]),list(range(89)))
        self.assertEqual(e.corpus_count(),89)
        # Domain-invalid, but typed-admissible cases must still reach the body.
        self.assertTrue({73,85}.issubset(n for n,_ in native))

    def test_missing_wrong_or_duplicate_boundary_cannot_hide_a_reference_case(self):
        document=e.host.strict(e.read(HERE/'input-boundaries.json'))
        original=e.read
        variants=[]
        missing=copy.deepcopy(document);missing['rejections'].pop();variants.append(missing)
        duplicate=copy.deepcopy(document);duplicate['rejections'][-1]=duplicate['rejections'][0];variants.append(duplicate)
        for key,value in [('reference_case_index',50),('field','wrong'),('element_index',1),('limit',1025),('observed_size',1024),('boundary','collection_elements'),('stage','COMPLETE')]:
            value_doc=copy.deepcopy(document);value_doc['rejections'][0][key]=value;variants.append(value_doc)
        for value in variants:
            with self.subTest(value=value['rejections'][0]):
                def read(path,*args):
                    return json.dumps(value).encode() if path==HERE/'input-boundaries.json' else original(path,*args)
                with patch.object(e,'read',side_effect=read):
                    with self.assertRaises(AssertionError):e.partition_corpus()

    def fixture(self,root,probe,case):
        cases=root/'case.json';receipt=root/'stdout';process=root/'process.json'
        e.host.save(cases,{'schema':'gooo/body-composition-cases/v1','cases':[case]})
        key=e.ENTRY+'.'+probe['input_port']
        prefix='case 0: input "'+key+'": record "'+probe['record']+'" field "'+probe['field']+'": '
        if probe['boundary']=='collection_elements':
            detail='repeated field has 257 values, limit 256'
        elif probe['element_index'] is None:
            detail='Text requires a string of at most 1024 UTF-8 bytes'
        else:
            detail='element 0: Text requires a string of at most 1024 UTF-8 bytes'
        message=prefix+detail
        doc={'schema':'gooo/workspace-body-execution-receipt/v1','decision':'FAIL_CLOSED','manifest':str(e.MANIFEST),'manifest_digest':e.sha(e.read(e.MANIFEST)),'cases_digest':e.sha(e.read(cases)),
             'error':'generate workspace activity bodies: '+message,'input_diagnostic':{'stage':'INPUT_VALIDATION','code':'INVALID_INPUT_VALUE','case_index':0,'key':key,'message':message}}
        go={'path':'/synthetic/no-execution/bin/go'}
        rec={'exit_code':1,'first_failure':None,'waited':True,'streams_EOF':True,'PG_ESRCH':True,'cwd':str(e.ROOT),'PID':123,'PGID':123,
             'argv':['/synthetic/no-execution/compiler','package','execute','--json','--receipt-compression','gzip','--go',go['path'],'--cases',str(cases),str(e.MANIFEST)]}
        self.write_receipt(receipt,doc,process,rec)
        return cases,receipt,process,go,doc,rec

    def write_receipt(self,path,document,process,record):
        path.write_bytes(gzip.compress(json.dumps(document,separators=(',',':')).encode(),mtime=0))
        record['stdout']=e.host.ref(path)
        e.host.save(process,record)

    def test_all_six_exact_decoder_shapes_are_accepted_without_execution_claim(self):
        _,probes=e.partition_corpus()
        with tempfile.TemporaryDirectory(prefix='snapshot-synthetic-admission-') as directory:
            for probe,case in probes:
                with self.subTest(index=probe['reference_case_index']):
                    cases,receipt,process,go,_,_=self.fixture(Path(directory),probe,case)
                    observed=e.verify_input_rejection(receipt,cases,probe,process,go,self.compiler)
                    self.assertFalse(observed['native_execution'])
                    self.assertEqual(observed['reference_case_index'],probe['reference_case_index'])

    def test_pass_label_or_wrong_boundary_evidence_is_refused(self):
        _,probes=e.partition_corpus();probe,case=probes[0]
        changes=[('decision','PASS'),('schema','other'),('manifest','/wrong/path'),('manifest_digest','sha256:'+'0'*64),('cases_digest','sha256:'+'0'*64),('error','unrelated refusal'),('result',{})]
        diagnostic=[('stage','COMPLETE'),('code','OTHER'),('case_index',3),('case_index',False),('key',e.ENTRY+'.input1'),('message','wrong record, field, element or limit')]
        with tempfile.TemporaryDirectory(prefix='snapshot-synthetic-admission-') as directory:
            cases,receipt,process,go,doc,rec=self.fixture(Path(directory),probe,case)
            for nested,key,value in [(False,k,v) for k,v in changes]+[(True,k,v) for k,v in diagnostic]:
                with self.subTest(key=key,value=value):
                    altered=copy.deepcopy(doc)
                    (altered['input_diagnostic'] if nested else altered)[key]=value
                    self.write_receipt(receipt,altered,process,copy.deepcopy(rec))
                    with self.assertRaises(AssertionError):e.verify_input_rejection(receipt,cases,probe,process,go,self.compiler)

    def test_wrong_or_unclosed_process_cannot_substitute_for_executable_probe(self):
        _,probes=e.partition_corpus();probe,case=probes[0]
        with tempfile.TemporaryDirectory(prefix='snapshot-synthetic-admission-') as directory:
            cases,receipt,process,go,doc,rec=self.fixture(Path(directory),probe,case)
            changes=[('exit_code',0),('exit_code',True),('first_failure','timeout'),('waited',False),('streams_EOF',False),('PG_ESRCH',False),('cwd','/wrong'),('PID',0),('PGID',456),('argv',['compiler','check']),('argv',['/different/compiler',*rec['argv'][1:]]),('stdout',{'sha256':'0'*64})]
            for key,value in changes:
                with self.subTest(key=key):
                    altered=copy.deepcopy(rec);altered[key]=value;e.host.save(process,altered)
                    with self.assertRaises(AssertionError):e.verify_input_rejection(receipt,cases,probe,process,go,self.compiler)

    def test_wrong_case_even_with_matching_receipt_digest_is_refused(self):
        _,probes=e.partition_corpus();probe,case=probes[0]
        with tempfile.TemporaryDirectory(prefix='snapshot-synthetic-admission-') as directory:
            cases,receipt,process,go,doc,rec=self.fixture(Path(directory),probe,case)
            altered=copy.deepcopy(case)
            altered['inputs'][e.ENTRY+'.input0']['page_next_cursor_values'][0]='short'
            e.host.save(cases,{'schema':'gooo/body-composition-cases/v1','cases':[altered]})
            doc['cases_digest']=e.sha(e.read(cases));self.write_receipt(receipt,doc,process,rec)
            with self.assertRaises(AssertionError):e.verify_input_rejection(receipt,cases,probe,process,go,self.compiler)


if __name__=='__main__':unittest.main()
