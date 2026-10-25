"""Source/typed-value checks shared by this example's generator and verifier.
Saved local receipts are observations; these checks do not authenticate producers.
"""
import importlib.util
import json
import re
import stat
import zlib
from pathlib import Path

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]

def module(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value

host = module('definition_owned', ROOT/'examples/service-grant-many/verify.py')
base = module('definition_imported_layouts', ROOT/'examples/recorded-catalog-comparison/generate.py')
PIN = host.ACTIVE_COMPILER_PROFILE['compiler_revision']
GO_VERSION = host.ACTIVE_COMPILER_PROFILE['go_version']
ENTRY = 'gooo/tool-origin/explicit-availability:AssessDefinitionAvailability'
AVAIL = 'gooo://tool-origin/explicit-availability/v1/'
PREFIX = 'gooo://tool-origin/definition-binding/v1/'
CORE = 'gooo://tool-origin/v1/'
RECORDED = 'gooo://tool-origin/recorded-catalog/v1/'
MANIFEST = ROOT/'explicit-availability.workspace.json'
SOURCES = dict(base.SOURCES, **{'gooo/tool-origin/definition-binding':'ports/tool-origin/definition_binding.gooo', 'gooo/tool-origin/explicit-availability':'ports/tool-origin/explicit_availability.gooo'})
DEF_INPUT = ('context_id','alias_context_id','collection_context_id','before_definition_ids','after_definition_ids','before_definition_profiles','after_definition_profiles','before_capability_ids','after_capability_ids','before_policy_revision','after_policy_revision')
DEF_OUTPUT = ('context_id','origin_context_id','collection_context_id','origin_state','origin_reason','collection_state','collection_reason','before_index','after_index','collection_first_problem_code','definition_relation','definition_profile_relation','capability_relation','authorization_relation','policy_relation','before_definition_id','after_definition_id','before_definition_profile','after_definition_profile','before_capability_id','after_capability_id','before_policy_revision','after_policy_revision','state','reason','next_action','declared_bindings_match','behavior_equivalence_proven','execution_authorized','live_effect_performed','contains_synthetic_fixture','remote_atomicity')
FIELDS = {CORE+'input':base.ORIGIN_INPUT, CORE+'result':base.ALIAS_OUTPUT,
          RECORDED+'input':base.COLLECTION_INPUT, RECORDED+'result':base.RAW_OUTPUT,
          PREFIX+'input':DEF_INPUT, PREFIX+'result':DEF_OUTPUT}
AVAIL_INPUT = ('context_id','alias_context_id','collection_context_id','definition_context_id','field_states','field_values','before_catalog_index','after_catalog_index','capture_begin_indexes','capture_end_indexes','page_context_indexes','before_policy_index','after_policy_index','before_row_page_indexes','after_row_page_indexes')
FIELDS[AVAIL+'input']=AVAIL_INPUT
INPUT_ENTITIES = (CORE+'input',RECORDED+'input',PREFIX+'input',AVAIL+'input')
OUTPUT_ROOT = 'examples/recorded-catalog-comparison'
ASSETS = {OUTPUT_ROOT+'/internal/availabilitykernel/'+name for name in ('projection_generated.go','types_generated.go','bridge_generated.go')} | {OUTPUT_ROOT+'/availability-generation.json',OUTPUT_ROOT+'/availability-kernel-cases.json'}
REFERENCE = ('reference/main.go','reference/go.mod','reference-inputs.json','case-labels.json')

sha = base.digest
same = host.exact

def read(path, limit=32<<20):
    assert not path.is_symlink() and stat.S_ISREG(path.stat().st_mode), 'regular file required'
    with path.open('rb') as source:
        raw = source.read(limit+1)
    assert len(raw)<=limit, 'file byte limit'
    return raw


def receipt(path):
    wire=read(path,2<<20)
    stream=zlib.decompressobj(16+zlib.MAX_WBITS)
    raw=stream.decompress(wire,(32<<20)+1)
    assert len(raw)<=32<<20 and stream.eof and not stream.unused_data and not stream.unconsumed_tail, 'one complete bounded gzip receipt required'
    return raw,host.strict(raw)


def semantic_records(plan):
    records={r['id']:r for r in plan['record_types']}
    assert len(records)==len(plan['record_types'])==7 and set(records)==set(FIELDS)
    integer_fields={'before_index','after_index','origin_before_index','origin_after_index','origin_conflict_first_index','origin_conflict_second_index','conflict_first_index','conflict_second_index','first_problem_capture_index','first_problem_page_index','first_problem_declared_index','page_capture_indexes','before_catalog_index','after_catalog_index','capture_begin_indexes','capture_end_indexes','page_context_indexes','before_policy_index','after_policy_index','before_row_page_indexes','after_row_page_indexes'}
    boolean_fields={'origin_execution_authorized','execution_authorized','live_removal_proven','behavior_equivalence_proven','live_effect_performed','declared_bindings_match','contains_synthetic_fixture','capture_relevant_change_observed'}
    many={CORE+'input':set(base.ORIGIN_INPUT[2:]), CORE+'result':set(), RECORDED+'input':set(base.COLLECTION_INPUT[4:]),
          RECORDED+'result':{'capture_pagination_states','capture_configuration_states','capture_recording_bases','capture_evidence_kinds'}, PREFIX+'input':set(DEF_INPUT[3:9]), PREFIX+'result':set(), AVAIL+'input':{'field_states','field_values','capture_begin_indexes','capture_end_indexes','page_context_indexes','before_row_page_indexes','after_row_page_indexes'}}
    for identity,record in records.items():
        fields=record['fields']
        assert len(fields)==len(FIELDS[identity]) and {f['name'] for f in fields}==set(FIELDS[identity])
        assert re.fullmatch(r'[A-Za-z_][A-Za-z0-9_]*',record['go_name'])
        for field in fields:
            name=field['name']
            assert field['id']==identity+'/'+name
            primitive='integer' if name in integer_fields else 'boolean' if name in boolean_fields else 'string'
            assert field['type_id']=='urn:gooo:type:'+primitive
            assert field.get('cardinality','one')==('many' if name in many[identity] else 'one')
            assert re.fullmatch(r'[A-Za-z_][A-Za-z0-9_]*',field['go_name'])
    assert len(plan['activities'])==1
    activity=plan['activities'][0]
    assert activity['id']==AVAIL+'assess' and activity['output_entity_id']==PREFIX+'result'
    same([(p['port'],p['entity_id']) for p in activity['inputs']], [('input'+str(i),e) for i,e in enumerate(INPUT_ENTITIES)])
    assert re.fullmatch(r'[A-Za-z_][A-Za-z0-9_]*',activity['go_function'])
    return records,activity


def observed_fields(actual,value,record):
    assert type(value) is dict and set(value)=={f['name'] for f in record['fields']}
    same(actual,[{'id':f['id'],'name':f['name'],'value':value[f['name']]} for f in record['fields']])
    types={'urn:gooo:type:string':str,'urn:gooo:type:boolean':bool,'urn:gooo:type:integer':int}
    for field in record['fields']:
        v=value[field['name']]
        if field.get('cardinality','one')=='many':
            assert type(v) is list
            assert all(type(x) is types[field['type_id']] for x in v)
        else:
            assert type(v) is types[field['type_id']]


def verify_pair(execution,replay,cases_path,go_identity):
    cases_raw=read(cases_path,2<<20)
    cases=host.strict(cases_raw)
    assert cases['schema']=='gooo/body-composition-cases/v1' and 1<=len(cases['cases'])<=8
    source={p:read(ROOT/p) for p in SOURCES.values()}
    parent=None;first=None
    for path in (execution,replay):
        raw,document=receipt(path)
        assert document['decision']=='PASS' and document['manifest_digest']==sha(read(MANIFEST)) and document['cases_digest']==sha(cases_raw)
        result=document['result'];runtime=result['runtime'];program=result['program'];composition=result['composition']
        assert runtime['stage']=='COMPLETE' and runtime['producer_source_sha']==PIN and runtime['go_version']==go_identity['version']
        assert runtime['go_tool_sha256']=='sha256:'+go_identity['sha256'] and runtime['go_tool_selection']=='explicit'
        same(runtime['producer_module'],host.ACTIVE_COMPILER_PROFILE['module'])
        assert runtime['model_calls']==0 and runtime['projection_replayed'] is True and runtime['runtime_replayed'] is True
        assert runtime['finite_total']==runtime['finite_passed']==len(cases['cases'])
        for process in [runtime['toolchain'],runtime['build'],*runtime['runs']]:
            assert process['started'] is True and process['completed'] is True and process['exit_code']==0
            assert process['canceled'] is False and process['timed_out'] is False and process['output_truncated'] is False
        assert len(runtime['runs'])==2
        same([p['path'] for p in program['workspace_image']['packages']],list(SOURCES))
        for package in program['workspace_image']['packages']:
            name=SOURCES[package['path']]
            assert len(package['sources'])==1 and package['sources'][0]['filename']==name
            assert package['sources'][0]['source_digest']==sha(base.go_json_string(source[name].decode()))
        wanted={CORE+'compare':SOURCES['gooo/tool-origin'],PREFIX+'is-sha256-text':SOURCES['gooo/tool-origin/definition-binding'], **{AVAIL+k:SOURCES['gooo/tool-origin/explicit-availability'] for k in ('valid','compare-origin','compare','assess')}}
        origins=program['activity_origins']
        assert len(origins)==len(wanted) and {o['lowered_activity_id'] for o in origins}==set(wanted)
        for origin in origins:
            binding=origin['input_source'];name=wanted[origin['lowered_activity_id']]
            assert binding['status']=='MAPPED' and binding['filename']==name and binding['source_sha256']==sha(source[name])
        assert program['entry']['package_path']=='gooo/tool-origin/explicit-availability' and program['entry']['activity']=='AssessDefinitionAvailability'
        assert sha(program['lowered_gooo_source'].encode())==result['execution_source_sha256']
        assert composition['stage']=='COMPLETE' and sha(composition['source'].encode())==composition['generated_sha256']==runtime['generated_sha256']
        assert sha(composition['driver_source'].encode())==composition['driver_sha256']==runtime['driver_sha256']
        assert runtime['typed_plan_sha256']==composition['plan']['typed_plan_sha256']
        records,activity=semantic_records(composition['plan'])
        assert len(runtime['traces'])==len(cases['cases'])
        for i,(trace,case) in enumerate(zip(runtime['traces'],cases['cases'])):
            assert trace['case_index']==i and len(trace['deliveries'])==1
            delivery=trace['deliveries'][0]
            assert delivery['activity_id']==AVAIL+'assess' and delivery['passed'] is True
            assert set(case['inputs'])=={ENTRY+'.input'+str(n) for n in range(4)} and len(delivery['inputs'])==4
            for n,(value,entity) in enumerate(zip(delivery['inputs'],INPUT_ENTITIES)):
                assert value['port']=='input'+str(n) and value['entity_id']==entity
                same(value['value'],case['inputs'][ENTRY+'.input'+str(n)])
                observed_fields(value['fields'],value['value'],records[entity])
            actual=delivery['actual'];same(actual,case['expected'][ENTRY]);same(delivery['expected'],actual)
            observed_fields(delivery['actual_fields'],actual,records[PREFIX+'result'])
            assert all(actual[k] is False for k in ('behavior_equivalence_proven','execution_authorized','live_effect_performed'))
            assert actual['remote_atomicity']=='UNKNOWN'
        if parent is None:
            parent=raw;first=composition
        else:
            assert document['replayed_from_sha256']==sha(parent) and result['replay']['model_calls']==0
            same(composition,first)
    return len(cases['cases']),first


def local_member(root,name):
    assert type(name) is str and name and '\\' not in name and ':' not in name
    p=Path(name)
    assert not p.is_absolute() and all(x not in ('','.','..') for x in name.split('/'))
    result=root/p
    assert result.resolve().is_relative_to(root.resolve())
    for ancestor in [result,*list(result.parents)[:len(p.parts)-1]]:
        assert not ancestor.is_symlink()
    return result


def verify_index(index_path,go_identity):
    index=host.strict(read(index_path,64<<10))
    assert set(index)=={'schema','batches'} and index['schema']=='gooo/explicit-availability-generation-input/v1'
    assert type(index['batches']) is list and len(index['batches'])==5
    corpus=host.strict(read(HERE/'kernel.cases.json',2<<20))['cases']
    assert len(corpus)==40
    offset=0;first=None;used=[]
    for batch in index['batches']:
        assert set(batch)=={'first_case','cases','execution','replay'} and type(batch['first_case']) is int and batch['first_case']==offset
        paths=[local_member(index_path.parent,batch[name]) for name in ('cases','execution','replay')]
        assert len(set(paths))==3
        rows=host.strict(read(paths[0],2<<20))['cases']
        assert len(rows)==min(8,40-offset)
        same(rows,corpus[offset:offset+len(rows)])
        count,composition=verify_pair(paths[1],paths[2],paths[0],go_identity)
        if first is None:first=composition
        else:
            for key in ('source','driver_source','plan','generated_sha256','driver_sha256'):
                same(composition[key],first[key])
        offset+=count;used+=paths
    assert offset==40 and len(set(used))==len(used)
    return first,used
