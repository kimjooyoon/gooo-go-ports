"""Synthetic record protocol controls; no oracle/compiler/native launch.

The fixture is deliberately not a policy evaluator or a native receipt. Its
all-false expected values test transport/provenance gates only. Actual policy
values are tested separately against the fresh independent Go oracle.
"""
from pathlib import Path
import copy
import hashlib
import importlib.util
import json
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

sys.dont_write_bytecode=True
HERE=Path(__file__).resolve().parent

def module(name,path):
    spec=importlib.util.spec_from_file_location(name,path);value=importlib.util.module_from_spec(spec);spec.loader.exec_module(value);return value

R=module('shared_ipv4_protocol',HERE/'ipv4_receipts.py')
Q=module('record_ipv4_protocol',HERE/'prefix_rule_pair_receipts.py');Q.configure(R)
H=module('owned_lifecycle_spans',HERE/'check-path-facts.py')
VERSION='go version go1.27.2 linux/amd64';GO='sha256:'+'8'*64

def wire(value):return (json.dumps(value,separators=(',',':'))+'\n').encode()

def oracle_fixture():
    result={'schema':'gooo/ipv4-prefix-rule-pair-independent-oracle/v1','go_version':'go1.27.2'}
    for key,path in (('pair_cases','examples/ipv4-rule-pair/corpus-112.json'),('prefix_cases','examples/ipv4-prefix-probe/corpus-18.json')):
        rows=R.strict_json((HERE.parent/path).read_bytes())['cases'];result[key]=[]
        for i,row in enumerate(rows):
            fields=('left','right') if key=='pair_cases' else ('prefix','address','other_prefix')
            expected={f:False if j<6 else 0 for j,f in enumerate(Q.FACTS)} if key=='pair_cases' else {'contains':False,'overlaps':False}
            result[key].append({'index':i,'name':row['name'],'inputs':{f:row[f] for f in fields},'expected':expected})
    return result


def fixture(checker,entry,inputs):
    report=entry==Q.REPORT;app=entry.split(':')[0];names,ret=Q.SIGNATURES[entry.split(':')[-1]]
    closure,sites,_=checker.graph(entry);lowered={k:'GoooPackage'+str(i)+'Activity'+k.split(':')[-1] for i,k in enumerate(closure)}
    _,_,bodies=checker.graph(entry,lowered)
    lowered_source='\n'.join('activity '+lowered[k]+'('+','.join(Q.SIGNATURES[k.split(':')[-1]][0])+') -> '+Q.SIGNATURES[k.split(':')[-1]][1]+' computes '+json.dumps(bodies[k]) for k in closure)
    source=R.digest(lowered_source.encode());root={'package_path':app,'activity':entry.split(':')[-1],'lowered_name':lowered[entry]}
    packages=[]
    for path in (Q.PORT,app):
        pname,namespace={Q.PORT:('netip','gooo_ports_net_netip'),Q.PROBE:('ipv4_prefix_probe','gooo_original_ipv4_prefix_probe'),Q.APP:('ipv4_rule_pair','gooo_original_ipv4_rule_pair')}[path]
        exports=[]
        if path==Q.PORT:
            entity_names=('Integer','Boolean','IPv4Address','IPv4Prefix');activity_names=('IPv4Valid',)+R.METHODS+('IPv4AddressValid','IPv4PrefixValid','IPv4PrefixDivisor','PrefixContains','PrefixOverlaps')
        else:entity_names=('Rule','RulePairReport') if report else ();activity_names=('RuleValid','Report') if report else ('Contains','Overlaps')
        for name in entity_names:exports.append({'name':name,'kind':'entity','id':Q.entity(name)})
        for name in activity_names:
            ins,out=Q.SIGNATURES[name] if name in Q.SIGNATURES else (['Integer']*4,'Boolean')
            exports.append({'name':name,'kind':'activity','id':Q.IDS[name] if name in Q.IDS else R.IDS[name],'input_types':[Q.entity(t) for t in ins],'output_type':Q.entity(out)})
        packages.append({'path':path,'name':pname,'namespace':namespace,'imports':None if path==Q.PORT else [Q.PORT],'sources':[{'filename':f,'source_digest':R.digest(R.go_string((HERE.parent/f).read_text()))} for f in Q.SOURCES[path]],'exports':exports})
    image={'schema':'gooo/package-runtime-image/v1','init_order':[Q.PORT,app],'packages':packages,'entry':{'package_path':app,'source':Q.SOURCES[app][0],'activity':root['activity'],'inputs':names,'output':ret}}
    calls={'schema':'gooo/workspace-pure-calls/v1','sites':sites,'activities':[{'activity':{'package_path':k.split(':')[0],'activity':k.split(':')[-1],'lowered_name':lowered[k]},'source':checker.bodies[k]['source'],'source_body_sha256':R.digest(checker.bodies[k]['body'].encode())} for k in closure]}
    activity={'name':lowered[entry],'id':Q.IDS[root['activity']],'input_type':names[0],'output_type':ret,'input_entity_id':Q.entity(names[0]),'output_entity_id':Q.entity(ret),'go_function':'GoooComposedActivity0','input_from':-1,'assembling':False,'inputs':[{'port':'input'+str(i),'type':t,'entity_id':Q.entity(t),'from':-1} for i,t in enumerate(names)]}
    plan={'schema':'gooo/body-composition-plan/v1','entry_activity':lowered[entry],'typed_plan_sha256':'sha256:'+'7'*64,'semantic_fingerprint':'a'*64,'activities':[activity],'edges':[],'record_types':Q.record_types(report)}
    edges=[]
    for key in closure:edges.extend({'caller_id':checker.bodies[key]['id'],'callee_id':checker.bodies[s['callee']]['id'],'ordinal':i} for i,s in enumerate(x for x in sites if x['caller']==key))
    def budget(k):return sum(1+budget(s['callee']) for s in sites if s['caller']==k)
    call={'schema':'gooo/pure-activity-call-closure/v1','activities':[{'name':lowered[k],'activity_id':checker.bodies[k]['id'],'program_sha256':R.digest(bodies[k].encode())} for k in closure if k!=entry],'edges':edges,'max_calls_per_invocation':budget(entry)}
    generated='package main\nfunc synthetic(input int64) int64 { return input / input }\n';driver='synthetic protocol driver';gs=R.digest(generated.encode());ds=R.digest(driver.encode())
    generation={'report':{'schema':'gooo/body-codegen-report/v3','decision':'PASS','activity':lowered[entry],'activity_id':Q.IDS[root['activity']],'compiler_source_sha':R.REV,'source_digest':source,'program_digest':R.digest(bodies[entry].encode()),'generated_digest':gs,'replay_digest':gs,'input_type':'('+','.join(names)+')','input_parameters':[{'name':'input'+str(i),'type':t} for i,t in enumerate(names)],'output_type':ret if report else 'bool','record_types':Q.record_types(report),'typecheck_passed':True,'deterministic_replay':True,'repository_writes':0,'call_closure':call},'source':generated}
    composition={'schema':'gooo/body-composition/v1','stage':'COMPLETE','original_source_sha256':source,'selected_source_sha256':source,'gooo_source':lowered_source,'plan':plan,'steps':[{'input_source_sha256':source,'generation':generation}],'source':generated,'driver_source':driver,'generated_sha256':gs,'driver_sha256':ds}
    process={'started':True,'completed':True,'canceled':False,'timed_out':False,'output_truncated':False,'diagnostics_bytes':0,'exit_code':0,'stdout_sha256':R.digest(b''),'stderr_sha256':R.digest(b'')}
    traces=[]
    supplied=R.strict_json(inputs.read_bytes())['inputs']
    for i,(row,oracle) in enumerate(zip(supplied,checker.rows[entry][:18])):
        ports=[]
        for j,t in enumerate(names):
            eid,fields=Q.RECORDS[t];v=row[entry+'.input'+str(j)];ports.append({'port':'input'+str(j),'entity_id':eid,'value':v,'fields':[{'id':eid+'/'+f,'name':f,'value':v[f]} for f in fields]})
        actual=Q.expected_value(oracle,entry);d={'activity_id':Q.IDS[root['activity']],'inputs':ports,'actual':actual}
        if report:d['actual_fields']=[{'id':Q.RESULT+'/'+f,'name':f,'value':actual[f]} for f in Q.FACTS]
        traces.append({'case_index':i,'deliveries':[d]})
    runtime={'schema':'gooo/body-composition-runtime/v1','stage':'COMPLETE','producer_source_sha':R.REV,'go_version':VERSION,'go_tool_sha256':GO,'go_tool_selection':'explicit','producer_module':{'path':'github.com/kimjooyoon/meta-ontology-go','version':'v0.0.0-20261011201109-83928fa45382'},'generated_sha256':gs,'driver_sha256':ds,'original_source_sha256':source,'selected_source_sha256':source,'typed_plan_sha256':plan['typed_plan_sha256'],'executable_sha256':'sha256:'+'4'*64,'runtime_suite_sha256':'sha256:'+'5'*64,'model_calls':0,'finite_total':0,'finite_passed':0,'projection_replayed':True,'runtime_replayed':True,'toolchain':process,'build':process,'runs':[process,process],'traces':traces}
    expression='input / input';start=generated.index(expression)
    site={'expression_id':'','root_activity_id':Q.IDS[root['activity']],'activity_id':Q.IDS[root['activity']],'operator':'/','expression':expression,'projection_sha256':gs,'start':start,'end':start+len(expression)}
    site['expression_id']=R.digest(R.go_string(site))
    runtime.update(observed_projection_sha256='sha256:'+'6'*64,observed_driver_sha256='sha256:'+'3'*64,fault_sites=[site],outcomes={k:0 for k in ('matched','mismatched','faulted','blocked','unobserved')})
    manifest=HERE.parent/Q.MANIFESTS[entry]
    return {'schema':'gooo/workspace-body-execution-receipt/v1','decision':'OBSERVED','manifest':str(manifest),'manifest_digest':R.digest(manifest.read_bytes()),'inputs_digest':R.digest(inputs.read_bytes()),'result':{'schema':'gooo/workspace-body-execution/v1','program':{'schema':'gooo/workspace-body-program/v1','entry':root,'activities':[root],'workspace_image':image,'pure_calls':calls,'lowered_gooo_source':lowered_source},'execution_source_sha256':source,'composition':composition,'runtime':runtime}}


class RecordProtocolTests(unittest.TestCase):
    def setUp(self):
        self.block=mock.patch.object(subprocess,'Popen',side_effect=AssertionError('No producer in source controls'));self.block.start();self.addCleanup(self.block.stop)
        self.checker=Q.Evidence(HERE.parent,oracle_fixture(),VERSION,R)

    def test_all310_transport_controls_and_three_accepted_mappings(self):
        saved=[]
        result=Q.run_wire(lambda name,raw:saved.append((name,raw)) or {'synthetic':name})
        self.assertEqual(len(result),310);self.assertEqual(len(saved),310)
        for entry in Q.MANIFESTS:self.assertEqual(len(Q.validate_inputs(Q.input_document(self.checker.rows[entry][:18],entry),entry)),18)
        self.assertEqual(len({v['name'] for v in result}),310)

    def test_record_entry_plan_callee_and_typed_replay_gates(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp).resolve()
            for entry in Q.MANIFESTS:
                ip=root/(entry.split(':')[-1]+'.json');ip.write_bytes(Q.input_document(self.checker.rows[entry][:18],entry));manifest=HERE.parent/Q.MANIFESTS[entry]
                value=fixture(self.checker,entry,ip);raw=wire(value);checked=self.checker.inspect(raw,entry,ip,manifest,GO)
                self.assertEqual(checked['output_count'],18)
                replay=copy.deepcopy(value);replay['replayed_from_sha256']=R.digest(raw);replay['result']['replay']={'schema':'gooo/workspace-body-replay/v1','model_calls':0,'body_fills_replayed':0,'prior_result_sha256':'sha256:'+'9'*64};rraw=wire(replay)
                self.assertEqual(checked,self.checker.inspect(rraw,entry,ip,manifest,GO,raw,'sha256:'+'9'*64))
                if entry==Q.REPORT:
                    records=Q.check_tampers(self.checker,raw,rraw,ip,manifest,GO,'sha256:'+'9'*64,lambda name,raw:{'synthetic':name},H.value_span)
                    self.assertEqual(len(records),18)
                    for f in Q.FACTS:
                        changed=copy.deepcopy(value);changed['result']['runtime']['traces'][0]['deliveries'][0]['actual'][f]=None
                        with self.subTest(f=f),self.assertRaisesRegex(R.ReceiptError,'PAIR_FACT_KINDS'):self.checker.inspect(wire(changed),entry,ip,manifest,GO)
                    changed=copy.deepcopy(value);changed['result']['composition']['plan']['record_types'][0]['fields'][0]['go_name']='wrong'
                    with self.assertRaisesRegex(R.ReceiptError,'PAIR_TYPED_PLAN_ABI'):self.checker.inspect(wire(changed),entry,ip,manifest,GO)


    def test_arithmetic_metadata_kinds_source_spans_and_replay_values(self):
        with tempfile.TemporaryDirectory() as tmp:
            ip=Path(tmp).resolve()/'record.json';entry=Q.REPORT;manifest=HERE.parent/Q.MANIFESTS[entry]
            ip.write_bytes(Q.input_document(self.checker.rows[entry][:18],entry));good=fixture(self.checker,entry,ip)
            self.checker.inspect(wire(good),entry,ip,manifest,GO)
            def alter(value,path,replacement):
                node=value
                for key in path[:-1]:node=node[key]
                node[path[-1]]=replacement
            changes=[(['fault_sites',0,'expression_id'],'sha256:'+'0'*64),(['fault_sites',0,'start'],True),(['fault_sites',0,'expression'],'wrong'),(['outcomes','matched'],True),(['outcomes','faulted'],1),(['observed_projection_sha256'],'wrong')]
            for path,replacement in changes:
                changed=copy.deepcopy(good);alter(changed['result']['runtime'],path,replacement)
                with self.subTest(path=path),self.assertRaisesRegex(R.ReceiptError,'PAIR_ARITHMETIC_BINDING'):self.checker.inspect(wire(changed),entry,ip,manifest,GO)
            raw=wire(good);replay=copy.deepcopy(good);replay['replayed_from_sha256']=R.digest(raw);replay['result']['replay']={'schema':'gooo/workspace-body-replay/v1','model_calls':0,'body_fills_replayed':0,'prior_result_sha256':'sha256:'+'9'*64}
            replay['result']['runtime']['observed_driver_sha256']='sha256:'+'2'*64
            with self.assertRaisesRegex(R.ReceiptError,'PAIR_REPLAY_ARITHMETIC'):self.checker.inspect(wire(replay),entry,ip,manifest,GO,raw,'sha256:'+'9'*64)

    def test_workspace_source_digest_preserves_crlf_and_bare_cr(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp).resolve();entry=Q.REPORT;ip=root/'inputs.json'
            ip.write_bytes(Q.input_document(self.checker.rows[entry][:18],entry))
            image=fixture(self.checker,entry,ip)['result']['program']['workspace_image']
            self.checker.repo=root
            # Literal Go JSON-string bytes; no fixture normalization is reused.
            for raw,encoded in ((b'// line\n',b'"// line\\n"'),
                                (b'// line\r\n',b'"// line\\r\\n"'),
                                (b'// line\r',b'"// line\\r"')):
                with self.subTest(raw=raw):
                    for package in image['packages']:
                        for source in package['sources']:
                            target=root/source['filename'];target.parent.mkdir(parents=True,exist_ok=True)
                            target.write_bytes(raw)
                            source['source_digest']='sha256:'+hashlib.sha256(encoded).hexdigest()
                    self.checker.workspace(image,entry)

    def test_workspace_source_digest_rejects_line_ending_changes(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp).resolve();entry=Q.REPORT;ip=root/'inputs.json'
            ip.write_bytes(Q.input_document(self.checker.rows[entry][:18],entry))
            image=fixture(self.checker,entry,ip)['result']['program']['workspace_image']
            self.checker.repo=root;targets=[]
            for package in image['packages']:
                for source in package['sources']:
                    target=root/source['filename'];target.parent.mkdir(parents=True,exist_ok=True)
                    target.write_bytes(b'// line\n');targets.append(target)
                    source['source_digest']='sha256:'+hashlib.sha256(b'"// line\\n"').hexdigest()
            self.checker.workspace(image,entry)
            for changed in (b'// line\r\n',b'// line\r'):
                with self.subTest(changed=changed):
                    targets[0].write_bytes(changed)
                    with self.assertRaisesRegex(R.ReceiptError,'^PAIR_SOURCE_IMAGE$'):
                        self.checker.workspace(image,entry)
            targets[0].write_bytes(b'// line\xff\n')
            with self.assertRaises(UnicodeDecodeError):self.checker.workspace(image,entry)


if __name__=='__main__':unittest.main()
