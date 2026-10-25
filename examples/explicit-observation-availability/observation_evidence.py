"""Source-bound finite single-field observation diagnostics evidence.
Hashes identify supplied records; they do not authenticate a producer.
"""
import importlib.util,json,re
from pathlib import Path
HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[1]
spec=importlib.util.spec_from_file_location('observation_shared_evidence',ROOT/'examples/tool-definition-binding/evidence.py')
shared=importlib.util.module_from_spec(spec);spec.loader.exec_module(shared)
host=shared.host;read=shared.read;receipt=shared.receipt;sha=shared.sha;same=shared.same
PIN=shared.PIN;GO_VERSION=shared.GO_VERSION
ENTRY='gooo/tool-origin/explicit-observation:AssessObservationField'
PREFIX='gooo://tool-origin/explicit-observation/v1/'
SOURCE='ports/tool-origin/explicit_observation.gooo'
MANIFEST=ROOT/'explicit-observation.workspace.json'
INPUT=('context_id','field_path','field_kind','recording_basis','value_state','value','comparison_scope','expected_value_state','expected_value','expected_field_path','source_document','expected_source_document')
OUTPUT=INPUT+('state','reason','next_action','required_for_context','known_change','producer_authenticated','behavior_equivalence_proven','execution_authorized','live_effect_performed')
FALSE_FIELDS=('producer_authenticated','behavior_equivalence_proven','execution_authorized','live_effect_performed')
BOOL_FIELDS={'required_for_context','known_change',*FALSE_FIELDS}

def semantic_records(plan):
 records={r['id']:r for r in plan['record_types']}
 assert len(records)==len(plan['record_types'])==2 and set(records)=={PREFIX+'input',PREFIX+'result'}
 for role,names in [('input',INPUT),('result',OUTPUT)]:
  record=records[PREFIX+role];assert [f['name'] for f in record['fields']]==list(names)
  assert re.fullmatch(r'[A-Za-z_][A-Za-z0-9_]*',record['go_name'])
  for field in record['fields']:
   assert field['id']==PREFIX+role+'/'+field['name'] and field.get('cardinality','one')=='one'
   assert field['type_id']=='urn:gooo:type:'+('boolean' if field['name'] in BOOL_FIELDS else 'string')
   assert re.fullmatch(r'[A-Za-z_][A-Za-z0-9_]*',field['go_name'])
 assert len(plan['activities'])==1
 activity=plan['activities'][0];assert activity['id']==PREFIX+'assess' and activity['output_entity_id']==PREFIX+'result'
 assert activity['input_entity_id']==PREFIX+'input' and activity['input_from']==-1 and activity['assembling'] is False and 'inputs' not in activity
 assert re.fullmatch(r'[A-Za-z_][A-Za-z0-9_]*',activity['go_function'])
 return records,activity

def verify_pair(execution,replay,cases_path,go_identity):
 cases_raw=read(cases_path,2<<20);cases=host.strict(cases_raw)
 assert cases['schema']=='gooo/body-composition-cases/v1' and 1<=len(cases['cases'])<=8
 source=read(ROOT/SOURCE);parent=None;first=None
 for path in (execution,replay):
  raw,document=receipt(path)
  assert document['schema']=='gooo/workspace-body-execution-receipt/v1' and document['decision']=='PASS'
  assert document['manifest_digest']==sha(read(MANIFEST)) and document['cases_digest']==sha(cases_raw)
  result=document['result'];runtime=result['runtime'];program=result['program'];composition=result['composition']
  assert runtime['stage']=='COMPLETE' and runtime['producer_source_sha']==PIN and runtime['go_version']==go_identity['version']
  assert runtime['go_tool_sha256']=='sha256:'+go_identity['sha256'] and runtime['go_tool_selection']=='explicit'
  same(runtime['producer_module'],host.ACTIVE_COMPILER_PROFILE['module'])
  assert runtime['model_calls']==0 and runtime['projection_replayed'] is True and runtime['runtime_replayed'] is True
  assert runtime['finite_total']==runtime['finite_passed']==len(cases['cases']) and len(runtime['runs'])==2
  for process in [runtime['toolchain'],runtime['build'],*runtime['runs']]:
   assert process['started'] is True and process['completed'] is True and process['exit_code']==0
   assert process['canceled'] is False and process['timed_out'] is False and process['output_truncated'] is False
  packages=program['workspace_image']['packages'];assert len(packages)==1 and packages[0]['path']=='gooo/tool-origin/explicit-observation'
  sources=packages[0]['sources'];assert len(sources)==1 and sources[0]['filename']==SOURCE and sources[0]['source_digest']==sha(shared.base.go_json_string(source.decode()))
  expected_origins={PREFIX+'is-pointer',PREFIX+'resolve-availability',PREFIX+'assess'}
  origins=program['activity_origins'];assert len(origins)==len(expected_origins) and {o['lowered_activity_id'] for o in origins}==expected_origins
  for origin in origins:
   binding=origin['input_source'];assert binding['status']=='MAPPED' and binding['filename']==SOURCE and binding['source_sha256']==sha(source)
  assert program['entry']['package_path']=='gooo/tool-origin/explicit-observation' and program['entry']['activity']=='AssessObservationField'
  assert sha(program['lowered_gooo_source'].encode())==result['execution_source_sha256']
  assert composition['stage']=='COMPLETE' and sha(composition['source'].encode())==composition['generated_sha256']==runtime['generated_sha256']
  assert sha(composition['driver_source'].encode())==composition['driver_sha256']==runtime['driver_sha256'] and runtime['typed_plan_sha256']==composition['plan']['typed_plan_sha256']
  records,activity=semantic_records(composition['plan']);assert len(runtime['traces'])==len(cases['cases'])
  for i,(trace,case) in enumerate(zip(runtime['traces'],cases['cases'])):
   assert trace['case_index']==i and len(trace['deliveries'])==1 and set(case['inputs'])=={ENTRY}
   delivery=trace['deliveries'][0];assert delivery['activity_id']==PREFIX+'assess' and delivery['passed'] is True
   same(delivery['input'],case['inputs'][ENTRY]);shared.observed_fields(delivery['input_fields'],delivery['input'],records[PREFIX+'input'])
   actual=delivery['actual'];same(actual,case['expected'][ENTRY]);same(delivery['expected'],actual)
   shared.observed_fields(delivery['actual_fields'],actual,records[PREFIX+'result'])
   assert all(actual[k] is False for k in FALSE_FIELDS)
  if parent is None:parent=raw;first=composition
  else:
   assert document['replayed_from_sha256']==sha(parent) and result['replay']['model_calls']==0
   same(composition,first)
 return len(cases['cases']),first

def verify_index(path,go_identity):
 value=host.strict(read(path,64<<10));assert set(value)=={'schema','batches'} and value['schema']=='gooo/explicit-observation-generation-input/v1'
 full=host.strict(read(HERE/'observation-kernel.cases.json',2<<20));assert full['schema']=='gooo/body-composition-cases/v1'
 assert type(value['batches']) is list and len(value['batches'])==(len(full['cases'])+7)//8
 covered=0;first=None;used=[]
 for batch in value['batches']:
  assert set(batch)=={'first_case','cases','execution','replay'} and type(batch['first_case']) is int and batch['first_case']==covered
  cases,execution,replay=[shared.local_member(path.parent,batch[k]) for k in ('cases','execution','replay')]
  expected=full['cases'][covered:covered+8];same(host.strict(read(cases))['cases'],expected)
  count,composition=verify_pair(execution,replay,cases,go_identity);assert count==len(expected)
  if first is None:first=composition
  else:
   stable={'schema','stage','original_source_sha256','selected_source_sha256','generated_sha256','driver_sha256','plan','steps','gooo_source','source','driver_source','scope'}
   assert set(first)==set(composition)==stable|{'elapsed_ns'}
   assert type(first['elapsed_ns']) is type(composition['elapsed_ns']) is int and first['elapsed_ns']>=0 and composition['elapsed_ns']>=0
   for key in stable:same(first[key],composition[key])
  covered+=count;used.extend([cases,execution,replay])
 assert covered==len(full['cases']) and len(set(used))==len(used)
 return first,used
