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

host = module('snapshot_owned', ROOT/'examples/service-grant-many/verify.py')
base = module('snapshot_imported_layouts', ROOT/'examples/recorded-catalog-comparison/generate.py')
PIN = host.ACTIVE_COMPILER_PROFILE['compiler_revision']
GO_VERSION = host.ACTIVE_COMPILER_PROFILE['go_version']
ENTRY = 'gooo/tool-origin/definition-snapshot:CompareDefinitionSnapshot'
PREFIX = 'gooo://tool-origin/definition-snapshot/v1/'
MANIFEST = ROOT/'definition-snapshot.workspace.json'
SOURCES = {'gooo/tool-origin/definition-snapshot':'ports/tool-origin/definition_snapshot.gooo'}
INPUT_ENTITIES = (PREFIX+'facts', PREFIX+'facts', PREFIX+'selection')
OUTPUT_ROOT = 'examples/recorded-catalog-comparison'
ASSETS = {OUTPUT_ROOT+'/internal/snapshotkernel/'+name for name in ('projection_generated.go','types_generated.go','bridge_generated.go')} | {OUTPUT_ROOT+'/snapshot-generation.json',OUTPUT_ROOT+'/snapshot-kernel-cases.json'}
REFERENCE = ('reference/main.go','reference/go.mod','reference/make_inputs.py','reference-inputs.json','case-labels.json','abi.json','input-boundaries.json')

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


def authored_contract():
    contract=host.strict(read(HERE/'abi.json',64<<10))
    assert contract['schema']=='gooo/definition-snapshot-authored-contract/v1'
    assert contract['status']=='SOURCE_WIP_NOT_A_GENERATED_PLAN_OR_EXECUTION_RECEIPT'
    assert contract['entry']==ENTRY and contract['activity_id']==PREFIX+'compare'
    assert contract['source']==SOURCES['gooo/tool-origin/definition-snapshot']
    assert contract['output_entity_id']==PREFIX+'result'
    same([(p['port'],p['entity_id']) for p in contract['inputs']], [('input'+str(i),entity) for i,entity in enumerate(INPUT_ENTITIES)])
    return contract


def semantic_records(plan):
    contract=authored_contract()
    wanted={record['id']:record for record in contract['records'].values()}
    assert len(wanted)==4
    records={r['id']:r for r in plan['record_types']}
    assert len(records)==len(plan['record_types'])==len(wanted) and set(records)==set(wanted)
    for identity,record in records.items():
        fields=record['fields'];expected=wanted[identity]['fields']
        assert 1<=len(fields)<=32 and len(fields)==len(expected)
        assert len({f['name'] for f in fields})==len(fields)
        assert len({f['go_name'] for f in fields})==len(fields)
        assert re.fullmatch(r'[A-Za-z_][A-Za-z0-9_]*',record['go_name'])
        assert {f['name'] for f in fields}=={f['name'] for f in expected}
        by_name={f['name']:f for f in expected}
        for field in fields:
            wanted_field=by_name[field['name']]
            for key in ('id','type_id','cardinality'):
                assert field.get(key,'one')==wanted_field[key]
            assert field['id']==identity+'/'+field['name']
            assert re.fullmatch(r'[A-Za-z_][A-Za-z0-9_]*',field['go_name'])
    assert len(plan['activities'])==1
    activity=plan['activities'][0]
    assert activity['id']==PREFIX+'compare' and activity['output_entity_id']==PREFIX+'result'
    same([(p['port'],p['entity_id']) for p in activity['inputs']], [('input'+str(i),entity) for i,entity in enumerate(INPUT_ENTITIES)])
    assert re.fullmatch(r'[A-Za-z_][A-Za-z0-9_]*',activity['go_function'])
    return records,activity


def corpus_count():
    inputs=host.strict(read(HERE/'reference-inputs.json',2<<20))
    labels=host.strict(read(HERE/'case-labels.json',64<<10))
    assert inputs['schema']=='gooo/definition-snapshot-reference-inputs/v1'
    assert labels['origin']=='SYNTHETIC_FIXTURE'
    names=[row['label'] for row in inputs['cases']]
    assert 1<=len(names)<=128 and len(names)==len(set(names))
    same(names,labels['cases'])
    return len(names)


def kernel_corpus():
    """Tie saved native expectations to the current literal inputs and anchors.

    This check does not execute the independent Go oracle; verify.py does so and
    compares its complete output bytes before starting any native corpus batch.
    """
    literal=host.strict(read(HERE/'reference-inputs.json',2<<20))['cases']
    document=host.strict(read(HERE/'kernel.cases.json',2<<20))
    assert set(document)=={'schema','cases'} and document['schema']=='gooo/body-composition-cases/v1'
    corpus=document['cases'];assert len(corpus)==corpus_count()
    result=authored_contract()['records']['SnapshotDecision']
    for case,source in zip(corpus,literal):
        assert set(case)=={'inputs','expected'} and set(case['expected'])=={ENTRY}
        same(case['inputs'],{ENTRY+'.input0':source['before'],ENTRY+'.input1':source['after'],ENTRY+'.input2':source['selection']})
        actual=case['expected'][ENTRY]
        observed_fields([{'id':f['id'],'name':f['name'],'value':actual[f['name']]} for f in result['fields']],actual,result)
        assert len(source['anchors'])>=4
        for key,value in source['anchors'].items():same(actual[key],value)
        assert all(actual[key] is False for key in ('behavior_equivalence_proven','execution_authorized','live_effect_performed'))
        assert actual['origin_relation']==actual['authorization_relation']=='NOT_ASSESSED'
        assert actual['remote_atomicity']=='UNKNOWN' and actual['comparison_scope']=='SAVED_ROWS_ONLY'
    return corpus


def partition_corpus():
    """Account for every original reference case at its actual engine boundary."""
    corpus=kernel_corpus();literal=host.strict(read(HERE/'reference-inputs.json',2<<20))['cases']
    document=host.strict(read(HERE/'input-boundaries.json',64<<10))
    assert document['schema']=='gooo/definition-snapshot-input-boundaries/v1' and document['compiler_revision']==PIN
    assert document['reference_cases']==len(corpus)==89 and document['native_cases']==83
    probes=document['rejections'];assert len(probes)==6
    declared={p['reference_case_index']:p for p in probes}
    assert len(declared)==len(probes)
    contract=authored_contract()['records']
    native=[]
    for index,(case,original) in enumerate(zip(corpus,literal)):
        violation=None
        for port,record_name in [('input0','SnapshotFacts'),('input1','SnapshotFacts'),('input2','Selection')]:
            value=case['inputs'][ENTRY+'.'+port]
            record=contract[record_name]
            assert set(value)=={f['name'] for f in record['fields']}
            for field in record['fields']:
                item=value[field['name']];many=field['cardinality']=='many'
                if many:
                    assert type(item) is list
                    if len(item)>256 and violation is None:
                        violation=(port,record_name,field['name'],None,'collection_elements',len(item),256)
                sequence=item if many else [item]
                for element,scalar in enumerate(sequence):
                    kind=field['type_id']
                    if kind=='urn:gooo:type:string':
                        assert type(scalar) is str
                        size=len(scalar.encode('utf-8'))
                        if size>1024 and violation is None:
                            violation=(port,record_name,field['name'],element if many else None,'text_utf8_bytes',size,1024)
                    elif kind=='urn:gooo:type:integer':assert type(scalar) is int and -(1<<63)<=scalar<(1<<63)
                    elif kind=='urn:gooo:type:boolean':assert type(scalar) is bool
                    else:raise AssertionError('unsupported fixture scalar')
        if violation is None:
            assert index not in declared,'declared rejection is inside typed input limits'
            native.append((index,case))
        else:
            assert index in declared,'unaccounted typed input rejection'
            probe=declared[index]
            same(tuple(probe[k] for k in ('input_port','record','field','element_index','boundary','observed_size','limit')),violation)
            assert probe['label']==original['label'] and probe['stage']=='INPUT_VALIDATION' and probe['code']=='INVALID_INPUT_VALUE'
    assert len(native)==83 and len(native)+len(probes)==89
    return native,[(probe,corpus[probe['reference_case_index']]) for probe in probes]


def verify_compiler_identity(identity):
    facts=host.strict(read(ROOT/'upstream.json'));profile=host.ACTIVE_COMPILER_PROFILE
    for key,value in {'schema':'gooo/build-identity/v1','language':'gooo','compiler_source_sha':PIN,'vcs_revision':PIN,'source_status':'CLEAN_VCS','vcs_modified':'false','go_version':GO_VERSION,'native_go_required':GO_VERSION,'version':facts['compiler_version'],'build_info_observed':True}.items():
        same(identity[key],value)
    same(identity['module'],profile['module']);same(identity['decision_runtime'],profile['decision_runtime'])
    same(identity['module_version'],profile['module']['version'])


def verify_producer(root,producer):
    """Bind a saved identity observation and each launched binary occurrence.

    This checks retained local evidence; it does not authenticate its author.
    """
    assert set(producer)=={'binary','identity','process'}
    binary=producer['binary'];assert set(binary)=={'path','bytes','sha256'}
    executable=Path(binary['path'])
    assert executable.is_absolute() and not executable.is_symlink() and stat.S_ISREG(executable.stat().st_mode)
    same(host.ref(executable),binary)
    identity_path=local_member(root,producer['identity']);process_path=local_member(root,producer['process'])
    identity=host.strict(read(identity_path,64<<10));verify_compiler_identity(identity)
    process=host.strict(read(process_path,64<<10))
    same(process['argv'],[str(executable),'version','--build','--json'])
    same(process['exit_code'],0)
    assert process['first_failure'] is None and process['cwd']==str(ROOT)
    assert all(process[k] is True for k in ('waited','streams_EOF','PG_ESRCH'))
    assert type(process['PID']) is int and process['PID']>0
    same(process['PID'],process['PGID']);same(process['stdout'],host.ref(identity_path))
    return binary,identity,[executable,identity_path,process_path]


def verify_input_rejection(path,cases_path,probe,process_path,go_identity,compiler_ref):
    cases_raw=read(cases_path,2<<20);cases=host.strict(cases_raw)
    assert set(cases)=={'schema','cases'} and cases['schema']=='gooo/body-composition-cases/v1' and len(cases['cases'])==1
    same(cases['cases'][0],kernel_corpus()[probe['reference_case_index']])
    _,document=receipt(path)
    assert set(document)=={'schema','decision','manifest','manifest_digest','cases_digest','error','input_diagnostic'}
    assert document['schema']=='gooo/workspace-body-execution-receipt/v1' and document['decision']=='FAIL_CLOSED'
    assert document['manifest']==str(MANIFEST) and document['manifest_digest']==sha(read(MANIFEST)) and document['cases_digest']==sha(cases_raw)
    key=ENTRY+'.'+probe['input_port']
    detail='record "'+probe['record']+'" field "'+probe['field']+'": '
    if probe['boundary']=='text_utf8_bytes':
        if probe['element_index'] is not None:detail+='element '+str(probe['element_index'])+': '
        detail+='Text requires a string of at most '+str(probe['limit'])+' UTF-8 bytes'
    else:
        assert probe['boundary']=='collection_elements'
        detail+='repeated field has '+str(probe['observed_size'])+' values, limit '+str(probe['limit'])
    message='case 0: input "'+key+'": '+detail
    same(document['input_diagnostic'],{'stage':'INPUT_VALIDATION','code':'INVALID_INPUT_VALUE','case_index':0,'key':key,'message':message})
    assert document['error']=='generate workspace activity bodies: '+message
    process=host.strict(read(process_path,64<<10))
    same(process['exit_code'],1)
    assert process['first_failure'] is None
    assert all(process[k] is True for k in ('waited','streams_EOF','PG_ESRCH'))
    same(process['stdout'],host.ref(path))
    same(process['argv'],[compiler_ref['path'],'package','execute','--json','--receipt-compression','gzip','--go',go_identity['path'],'--cases',str(cases_path),str(MANIFEST)])
    assert process['cwd']==str(ROOT) and type(process['PID']) is int and process['PID']>0
    same(process['PID'],process['PGID'])
    return {'reference_case_index':probe['reference_case_index'],'label':probe['label'],'stage':'INPUT_VALIDATION','code':'INVALID_INPUT_VALUE','native_execution':False,'case_index':0,'key':key,
            'boundary':{k:probe[k] for k in ('record','field','element_index','boundary','observed_size','limit')},'compiler':compiler_ref,'receipt':host.ref(path),'process':host.ref(process_path)}


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
        wanted={PREFIX+name:SOURCES['gooo/tool-origin/definition-snapshot'] for name in ('bounded-text','digest-text','inspect','compare')}
        origins=program['activity_origins']
        assert len(origins)==len(wanted) and {o['lowered_activity_id'] for o in origins}==set(wanted)
        for origin in origins:
            binding=origin['input_source'];name=wanted[origin['lowered_activity_id']]
            assert binding['status']=='MAPPED' and binding['filename']==name and binding['source_sha256']==sha(source[name])
        assert program['entry']['package_path']=='gooo/tool-origin/definition-snapshot' and program['entry']['activity']=='CompareDefinitionSnapshot'
        assert sha(program['lowered_gooo_source'].encode())==result['execution_source_sha256']
        assert composition['stage']=='COMPLETE' and sha(composition['source'].encode())==composition['generated_sha256']==runtime['generated_sha256']
        assert sha(composition['driver_source'].encode())==composition['driver_sha256']==runtime['driver_sha256']
        assert runtime['typed_plan_sha256']==composition['plan']['typed_plan_sha256']
        records,activity=semantic_records(composition['plan'])
        assert len(runtime['traces'])==len(cases['cases'])
        for i,(trace,case) in enumerate(zip(runtime['traces'],cases['cases'])):
            assert trace['case_index']==i and len(trace['deliveries'])==1
            delivery=trace['deliveries'][0]
            assert delivery['activity_id']==PREFIX+'compare' and delivery['passed'] is True
            assert set(case['inputs'])=={ENTRY+'.input'+str(n) for n in range(3)} and len(delivery['inputs'])==3
            for n,(value,entity) in enumerate(zip(delivery['inputs'],INPUT_ENTITIES)):
                assert value['port']=='input'+str(n) and value['entity_id']==entity
                same(value['value'],case['inputs'][ENTRY+'.input'+str(n)])
                observed_fields(value['fields'],value['value'],records[entity])
            actual=delivery['actual'];same(actual,case['expected'][ENTRY]);same(delivery['expected'],actual)
            observed_fields(delivery['actual_fields'],actual,records[PREFIX+'result'])
            assert all(actual[k] is False for k in ('behavior_equivalence_proven','execution_authorized','live_effect_performed'))
            assert actual['remote_atomicity']=='UNKNOWN'
            assert actual['origin_relation']==actual['authorization_relation']=='NOT_ASSESSED'
            assert actual['comparison_scope']=='SAVED_ROWS_ONLY'
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


def lineage_inputs(index,root,identity):
    lineage=index['lineage']
    if lineage['kind']=='FRESH_VERIFIER_RUN':
        same(lineage,{'kind':'FRESH_VERIFIER_RUN','fresh_native_execution':True})
        return []
    assert set(lineage)=={'kind','fresh_native_execution','source_index','source_summary','derivation_source'}
    assert lineage['kind']=='RETAINED_OBSERVATIONS_NEW_INDEX' and lineage['fresh_native_execution'] is False
    paths=[]
    for key in ('source_index','source_summary'):
        reference=lineage[key];assert set(reference)=={'file','sha256'}
        path=local_member(root,reference['file']);assert sha(read(path,4<<20))==reference['sha256'];paths.append(path)
    source=host.strict(read(paths[0],64<<10))
    assert set(source)=={'schema','batches','rejections'} and source['schema']=='gooo/definition-snapshot-generation-input/v2'
    for key in ('batches','rejections'):same(source[key],index[key])
    summary=host.strict(read(paths[1],4<<20))
    assert summary['status']=='FINITE_SNAPSHOT_NATIVE_OWN_REPLAY_PASS' and summary['first_failure'] is None
    assert summary['all_direct_wait_EOF_PG_ESRCH'] is True
    same(summary['compiler'],index['producer']['binary']);same(summary['compiler_identity'],identity)
    for key,value in {'reference_cases':89,'kernel_cases':83,'input_rejection_cases':6,'input_rejections_verified':True,'reference_cases_accounted':89}.items():same(summary[key],value)
    derivation=lineage['derivation_source'];assert set(derivation)=={'file','sha256'}
    assert derivation['file']=='examples/tool-definition-snapshot/bind_producer.py'
    helper=ROOT/derivation['file'];assert sha(read(helper))==derivation['sha256'];paths.append(helper)
    return paths


def index_producer_inputs(index,root):
    _,identity,paths=verify_producer(root,index['producer'])
    return paths+lineage_inputs(index,root,identity)


def verify_index(index_path,go_identity):
    return verify_index_document(host.strict(read(index_path,64<<10)),index_path.parent,go_identity)


def verify_index_document(index,root,go_identity):
    assert set(index)=={'schema','producer','lineage','batches','rejections'} and index['schema']=='gooo/definition-snapshot-generation-input/v3'
    used=index_producer_inputs(index,root)
    compiler_ref=index['producer']['binary']
    native,probes=partition_corpus();total=len(native)
    assert type(index['batches']) is list and len(index['batches'])==(total+7)//8
    offset=0;first=None
    for batch in index['batches']:
        assert set(batch)=={'first_case','reference_case_indexes','cases','execution','replay'} and type(batch['first_case']) is int and batch['first_case']==offset
        paths=[local_member(root,batch[name]) for name in ('cases','execution','replay')]
        assert len(set(paths))==3
        rows=host.strict(read(paths[0],2<<20))['cases']
        assert len(rows)==min(8,total-offset)
        selected=native[offset:offset+len(rows)]
        same(batch['reference_case_indexes'],[n for n,_ in selected])
        same(rows,[case for _,case in selected])
        count,composition=verify_pair(paths[1],paths[2],paths[0],go_identity)
        if first is None:first=composition
        else:
            for key in ('source','driver_source','plan','generated_sha256','driver_sha256'):
                same(composition[key],first[key])
        offset+=count;used+=paths
    assert offset==total
    assert type(index['rejections']) is list and len(index['rejections'])==len(probes)
    for item,(probe,_) in zip(index['rejections'],probes):
        assert set(item)=={'reference_case_index','cases','execution','process'}
        same(item['reference_case_index'],probe['reference_case_index'])
        paths=[local_member(root,item[name]) for name in ('cases','execution','process')]
        verify_input_rejection(paths[1],paths[0],probe,paths[2],go_identity,compiler_ref)
        used+=paths
    assert len(set(used))==len(used)
    return first,used
