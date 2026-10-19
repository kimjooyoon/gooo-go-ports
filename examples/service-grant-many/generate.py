"""Render only from genuine four-call source-bound receipts; no semantic Go fallback."""
from pathlib import Path
import argparse,hashlib,importlib.util,json,re
HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[1]
PIN='de99b770b8b5adea608233c3c0664902336e3310'
ENTRY='gooo/service-policy/multi-required:AssessRequiredRequests'
AID='gooo://service-policy/multi-required/assess/v1'

def digest(b):return 'sha256:'+hashlib.sha256(b).hexdigest()
def encoded(x):return (json.dumps(x,ensure_ascii=False,indent=2,sort_keys=True)+'\n').encode()
def strict(b):
 def pairs(rows):
  d={}
  for k,v in rows:
   if k in d:raise ValueError('duplicate key: '+k)
   d[k]=v
  return d
 return json.loads(b,object_pairs_hook=pairs,parse_constant=lambda x:(_ for _ in ()).throw(ValueError(x)))
def saved_reader():
 spec=importlib.util.spec_from_file_location('public_many_saved',HERE/'verify_saved.py');m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);return m

def validate(receipts,go_identity=None):
 v=saved_reader();expected=strict((HERE/'reference-expected.json').read_bytes());assert len(expected)==16
 provenance=strict((HERE/'source-provenance.json').read_bytes())
 for row in provenance['byte_exact_copies']:
  b=(ROOT/row['path']).read_bytes();assert len(b)==row['bytes'] and digest(b)=='sha256:'+row['sha256']
 projections=[];programs=[];observations=[];tool_identities=[]
 for stem,rows in [('normal',expected[:1]),('remaining',expected[1:])]:
  cases=HERE/(stem+'-cases.json');parent=receipts/(stem+'-execute')/'stdout'
  for replay in [False,True]:
   label=stem+('-saved-replay' if replay else '-execute');folder=receipts/label
   x=strict((folder/'stdout').read_bytes());process=strict((folder/'process.json').read_bytes())
   assert process['exit_code']==0 and type(process['exit_code']) is int and process['waited'] is True and process['streams_EOF'] is True and process['PG_ESRCH'] is True
   assert process.get('first_failure',process.get('failure')) is None
   result=v.validate(x,cases,rows,parent if replay else None,go_identity)
   tool_identities.append((x['result']['runtime']['go_version'],x['result']['runtime']['go_tool_sha256']))
   co=x['result']['composition'];projections.append((co['source'],co['driver_source'],co['plan']))
   programs.append(x['result']['program'])
   observations.append({'label':label,'read_observation':result['read_observation'],'local_observation':result['local_observation'],'field_observation':result['field_observation']})
 assert all(p==projections[0] for p in projections) and all(p==programs[0] for p in programs)
 assert all(t==tool_identities[0] for t in tool_identities)
 return projections[0],programs[0],observations

def bridge(plan):
 inputs={'before':'Before','after':'After','required_types':'RequiredTypes','required_names':'RequiredNames'}
 outputs={'status':'Status','reason':'Reason','invalid_field':'InvalidField','invalid_index':'InvalidIndex','first_counterexample':'FirstCounterexample','request_count':'RequestCount','checked_count':'CheckedCount','all_preserved':'AllPreserved','supported_subset':'SupportedSubset','subset_rule_id':'SubsetRuleID','counterexample_before_match':'CounterexampleBeforeMatch','counterexample_after_match':'CounterexampleAfterMatch'}
 records={r['id']:r for r in plan['record_types']};inp=records['gooo://service-policy/multi-required/input/v1'];dec=records['gooo://service-policy/multi-required/decision/v1']
 assert {f['name'] for f in inp['fields']}==set(inputs) and {f['name'] for f in dec['fields']}==set(outputs)
 for f in inp['fields']:assert f['type_id']=='urn:gooo:type:string' and f.get('cardinality','one')==('many' if f['name'].startswith('required_') else 'one')
 activity=next(a for a in plan['activities'] if a['id']==AID);assert activity['go_function']=='GoooComposedActivity0'
 text='// Code generated from compiler-owned typed-plan identities; DO NOT EDIT.\npackage servicegrantmany\n\n// EvaluateColumns returns the Gooo decision on ordered caller-supplied facts.\nfunc EvaluateColumns(input RequestColumns) Decision {\n\tnative := '+inp['go_name']+'{\n'
 for f in inp['fields']:text+='\t\t'+f['go_name']+': input.'+inputs[f['name']]+',\n'
 text+='\t}\n\tactual := '+activity['go_function']+'(native)\n\treturn Decision{\n'
 # Tabs are formatter-normalized by --gofmt before bindings are captured.
 for f in dec['fields']:text+='\t\t'+outputs[f['name']]+': actual.'+f['go_name']+',\n'
 return (text+'\t}\n}\n').encode()

ASSETS=['LICENSE','LICENSE-APACHE-2.0','NOTICE.md','go.mod','model.go','input.go','bindings.go','policy_test.go','cmd/evaluate/main.go','cmd/evaluate/main_test.go','projection_generated.go','bridge_generated.go','compiler-pin.json','compiler.json','source-provenance.json','normal-cases.json','remaining-cases.json','all-cases.json','case-facts.json','reference-expected.json','reference-cases.json','reference/go.mod','reference/main.go','reference/source-pin.json','reference/LICENSE','reference/upstream/extracted.go','reference/upstream/LICENSE','generate.py','verify.py','verify_saved.py']
def render(receipts,check=False,gofmt=None,go_identity=None):
 (raw,driver,plan),program,observations=validate(receipts,go_identity)
 assert raw.startswith('package main\n') and 'import ' not in raw
 projection=raw.replace('package main\n','package servicegrantmany\n',1).encode()
 adapter=bridge(plan)
 if gofmt is not None:
  import subprocess
  formatted=subprocess.run([str(gofmt)],input=adapter,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=10,check=True)
  assert not formatted.stderr;adapter=formatted.stdout
 elif (HERE/'bridge_generated.go').exists():
  # Saved rendering may use the already formatted identical compiler-owned map.
  current=(HERE/'bridge_generated.go').read_bytes()
  assert re.sub(rb'\s+',b'',current)==re.sub(rb'\s+',b'',adapter);adapter=current
 produced={'projection_generated.go':projection,'bridge_generated.go':adapter}
 for name,b in produced.items():
  if check:assert (HERE/name).read_bytes()==b,name
  else:(HERE/name).write_bytes(b)
 public_sources={s['filename']:digest((ROOT/s['filename']).read_bytes()) for p in program['workspace_image']['packages'] for s in p['sources']}
 assets={name:digest(produced.get(name,(HERE/name).read_bytes())) for name in ASSETS}
 metadata={'schema':'gooo/service-grant-many-generation/v1','origin':'fresh_pinned_generation','compiler_source_sha':PIN,'go_version':'go1.27.2','workspace_digest':digest((ROOT/'service-grant-many.workspace.json').read_bytes()),'public_sources':public_sources,'assets':assets,'raw_projection_sha256':digest(raw.encode()),'raw_driver_sha256':digest(driver.encode()),'typed_plan_sha256':plan['typed_plan_sha256'],'projection_sha256':digest(projection),'projection_transform':'Replace the first package token main with servicegrantmany; every other raw projection byte unchanged.','bridge_origin':'Compiler plan record/field/go_function identities; transport mapping only.','lowered_source_sha256':digest(program['lowered_gooo_source'].encode()),'source_and_proof_scope':'Finite four-command execute/replay16 cases. Generated artifacts are source-bound, not authenticated authority. Local observation completeness is reported separately.'}
 target=encoded(metadata)
 if check:assert (HERE/'generation.json').read_bytes()==target,'generation bindings'
 else:(HERE/'generation.json').write_bytes(target)
 return metadata,observations
if __name__=='__main__':
 ap=argparse.ArgumentParser();ap.add_argument('--receipts',type=Path,required=True);ap.add_argument('--gofmt',type=Path);ap.add_argument('--check',action='store_true');a=ap.parse_args();m,_=render(a.receipts.resolve(),a.check,a.gofmt);print(json.dumps({'status':'SOURCE_BOUND_GENUINE_RECEIPTS_CHECKED','compiler':m['compiler_source_sha'],'fresh_compiler_calls_in_renderer':0,'assets':len(m['assets'])}))
