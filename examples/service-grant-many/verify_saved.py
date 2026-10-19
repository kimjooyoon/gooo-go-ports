"""Read only saved producer outputs; invokes no compiler, Go, or network."""
import hashlib,json,re
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
OUT=None
PIN='de99b770b8b5adea608233c3c0664902336e3310'
ENTRY='gooo/service-policy/multi-required:AssessRequiredRequests'
ACTIVITY='gooo://service-policy/multi-required/assess/v1'
OUT_FIELDS=[('status','status',str),('reason','reason',str),('invalid_field','invalid_field',str),('invalid_index','invalid_index',int),('first_counterexample','first_counterexample',int),('request_count','request_count',int),('checked_count','checked_count',int),('all_preserved','all_preserved',bool),('supported_subset','supported_subset',bool),('subset_rule_id','subset_rule_id',str),('counterexample_before','counterexample_before_match',bool),('counterexample_after','counterexample_after_match',bool)]
IN_FIELDS=[('before','before',str),('after','after',str),('types','required_types',list),('names','required_names',list)]
def sha(b):return 'sha256:'+hashlib.sha256(b).hexdigest()
def exact(a,b):
 assert type(a) is type(b),(type(a),type(b))
 if isinstance(a,dict):
  assert a.keys()==b.keys()
  for k in a:exact(a[k],b[k])
 elif isinstance(a,list):
  assert len(a)==len(b)
  for av,bv in zip(a,b):exact(av,bv)
 else:assert a==b,(a,b)
def file(path):
 b=path.read_bytes();return {'path':str(path),'bytes':len(b),'sha256':hashlib.sha256(b).hexdigest()}
def validate(x,casefile,expected,parent=None,go_identity=None):
 assert x['decision']=='PASS' and x['schema']=='gooo/workspace-body-execution-receipt/v1'
 assert x['manifest']==str(ROOT/'service-grant-many.workspace.json')
 assert x['manifest_digest']==sha((ROOT/'service-grant-many.workspace.json').read_bytes())
 assert x['cases_digest']==sha(casefile.read_bytes())
 if parent:assert x['replayed_from_sha256']==sha(parent.read_bytes())
 r=x['result'];p=r['program'];rt=r['runtime'];composition=r['composition']
 assert set(p['entry'])=={'package_path','activity','lowered_name'}
 assert p['entry']['package_path']=='gooo/service-policy/multi-required' and p['entry']['activity']=='AssessRequiredRequests'
 assert re.fullmatch(r'GoooPackage[0-9]+ActivityAssessRequiredRequests',p['entry']['lowered_name'])
 assert re.search(r'(?m)^activity '+re.escape(p['entry']['lowered_name'])+r'\(',p['lowered_gooo_source'])
 assert p['workspace_image']['entry']=={'package_path':'gooo/service-policy/multi-required','source':'ports/service-policy/multi_required.gooo','activity':'AssessRequiredRequests','inputs':['RequestColumns'],'output':'Decision'}
 assert r['execution_source_sha256']==sha(p['lowered_gooo_source'].encode())
 assert rt['original_source_sha256']==rt['selected_source_sha256']==r['execution_source_sha256']
 assert rt['generated_sha256']==composition['generated_sha256']==sha(composition['source'].encode())
 assert rt['driver_sha256']==composition['driver_sha256']==sha(composition['driver_source'].encode())
 assert rt['typed_plan_sha256']==composition['plan']['typed_plan_sha256']
 assert rt['producer_source_sha']==PIN
 assert re.fullmatch(r'go version go1\.27\.2 (darwin/arm64|linux/amd64)',rt['go_version'])
 assert rt['go_tool_selection']=='explicit' and re.fullmatch(r'sha256:[0-9a-f]{64}',rt['go_tool_sha256'])
 if go_identity is not None:
  exact({'version':rt['go_version'],'sha256':rt['go_tool_sha256']},go_identity)
 assert rt['stage']=='COMPLETE' and rt['model_calls']==0 and type(rt['model_calls']) is int
 assert rt['projection_replayed'] is True and rt['runtime_replayed'] is True
 assert rt['finite_total']==rt['finite_passed']==len(expected)
 sources={s['filename'] for pkg in p['workspace_image']['packages'] for s in pkg['sources']}
 assert sources=={'ports/service-policy/multi_required.gooo','ports/agentmesh/service_pattern.gooo','examples/agentmesh-service-policy/domain.gooo','examples/agentmesh-service-policy/refinement.gooo','examples/agentmesh-service-policy/checked_refinement.gooo'}
 seen=set()
 for a in p['activity_origins']:
  origin=a['input_source'];path=origin['filename'];assert path in sources
  assert origin['status']=='MAPPED' and origin['source_sha256']==sha((ROOT/path).read_bytes())
  assert origin['activity_id']==a['lowered_activity_id'];seen.add(path)
 assert seen==sources
 for result in [rt['toolchain'],rt['build'],*rt['runs']]:
  assert result['started'] is True and result['completed'] is True
  assert result['exit_code']==0 and result['canceled'] is False and result['timed_out'] is False and result['output_truncated'] is False
 assert len(rt['runs'])==2 and rt['runs'][0]['stdout_sha256']==rt['runs'][1]['stdout_sha256']
 assert len(rt['traces'])==len(expected)
 outputs=[]
 for i,(trace,row) in enumerate(zip(rt['traces'],expected)):
  assert trace['case_index']==i and len(trace['deliveries'])==1
  d=trace['deliveries'][0];assert d['activity_id']==ACTIVITY and d['passed'] is True
  exact(d['input'],row['facts']);exact(d['expected'],row['decision']);exact(d['actual'],row['decision'])
  assert len(d['input_fields'])==4 and len(d['actual_fields'])==12
  for f,(suffix,name,t) in zip(d['input_fields'],IN_FIELDS):
   assert f['id']=='gooo://service-policy/multi-required/input/'+suffix and f['name']==name and type(f['value']) is t
   exact(f['value'],row['facts'][name])
   if t is list:assert all(type(v) is str for v in f['value'])
  for f,(suffix,name,t) in zip(d['actual_fields'],OUT_FIELDS):
   assert f['id']=='gooo://service-policy/multi-required/'+suffix and f['name']==name and type(f['value']) is t
   exact(f['value'],row['decision'][name])
  outputs.append({'case':row['name'],'decision':d['actual']})
 return {'outputs':outputs,'program':p,'native_run_observations':len(rt['runs']),'build_seconds':rt['build']['wall_ns']/1e9,'native_run_seconds':[r['wall_ns']/1e9 for r in rt['runs']],'read_observation':rt['read_observation'],'local_observation':rt['local_observation'],'field_observation':rt['field_observation']}
def main():
 expected=json.loads((ROOT/'examples/service-grant-many/reference-expected.json').read_bytes());assert len(expected)==16
 copied=json.loads((ROOT/'examples/service-grant-many/source-provenance.json').read_bytes())['byte_exact_copies']
 for row in copied:
  b=(ROOT/row['path']).read_bytes();assert len(b)==row['bytes'] and hashlib.sha256(b).hexdigest()==row['sha256']
 results=[];first_program=None
 for stem,casefile,rows in [('normal',ROOT/'examples/service-grant-many/normal-cases.json',expected[:1]),('remaining',ROOT/'examples/service-grant-many/remaining-cases.json',expected[1:])]:
  parent=OUT/(stem+'-execute')/'stdout'
  for label,replay in [(stem+'-execute',False),(stem+'-saved-replay',True)]:
   x=json.loads((OUT/label/'stdout').read_bytes());v=validate(x,casefile,rows,parent if replay else None)
   if first_program is None:first_program=v.pop('program')
   else:exact(v.pop('program'),first_program)
   process=json.loads((OUT/label/'process.json').read_bytes())
   assert type(process['exit_code']) is int and process['exit_code']==0 and process.get('first_failure') is None and process['streams_EOF'] is True and process['waited'] is True and process['PG_ESRCH'] is True
   results.append({'label':label,'receipt':file(OUT/label/'stdout'),'process':file(OUT/label/'process.json'),**v})
 result={'status':'PUBLIC_ID_CANDIDATE_16_CASES_AND_SAVED_REPLAY_MATCH','compiler_revision':PIN,'cases':16,'execute_commands':2,'saved_replay_commands':2,'native_builds':4,'native_run_process_observations':8,'native_case_observations':64,'typed_output_fields_per_case':12,'retained_case_trace_field_checks':384,'fresh_independent_Go_oracle_compile_and_run':0,'accepted_reference_expected_reused':True,'model_calls':0,'producer_authenticated':False,'native_acceptance':False,'external_adoption_observed':False,'full_CEL_compatibility':False,'live_authorization_or_effect':False,'reused_source_IDs_and_license_bytes_unchanged':True,'results':results,'source':file(ROOT/'ports/service-policy/multi_required.gooo'),'workspace':file(ROOT/'service-grant-many.workspace.json'),'case_facts':file(ROOT/'examples/service-grant-many/case-facts.json'),'independent_oracle':file(ROOT/'examples/service-grant-many/reference-expected.json'),'scope':'Fresh public-ID candidate finite maintainer-owned source/native comparison and saved replay; not publicly integrated. No external adopter or current trusted authority. Collection-read/local trace completeness is reported separately, never inferred from functional PASS.'}
 (OUT/'functional-acceptance.json').write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n')
 print(json.dumps({'status':result['status'],'cases':16,'native_runs':8,'native_case_observations':64,'receipt':str(OUT/'functional-acceptance.json')}))
if __name__=='__main__':
 import argparse
 ap=argparse.ArgumentParser();ap.add_argument('--out',type=Path,required=True);args=ap.parse_args();OUT=args.out.resolve();main()
