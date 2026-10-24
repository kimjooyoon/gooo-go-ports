"""Render the saved-catalog kernel from a fresh pinned execution/replay pair.
The imported alias decision and all collection policy remain compiler-owned Gooo.
"""
import argparse
import datetime
import hashlib
import json
import re
import subprocess
import uuid
import zlib
from pathlib import Path

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
FACTS = json.loads((ROOT / 'upstream.json').read_bytes())
PIN = FACTS['compiler_revision']
GO_VERSION = FACTS['go_version']
SELECTED_PIN_BYTES = json.dumps({'compiler_revision': PIN, 'go_version': GO_VERSION}, sort_keys=True, separators=(',', ':')).encode()
ENTRY = 'gooo/tool-origin/recorded-catalog:CompareRecordedCatalog'
PREFIX = 'gooo://tool-origin/recorded-catalog/v1/'
ORIGIN_PREFIX = 'gooo://tool-origin/v1/'
SOURCES = {'gooo/tool-origin': 'ports/tool-origin/alias_origin.gooo', 'gooo/tool-origin/recorded-catalog': 'ports/tool-origin/recorded_catalog.gooo'}
ORIGIN_INPUT = ["context_id","target_alias","before_servers","before_names","before_aliases","after_servers","after_names","after_aliases"]
COLLECTION_INPUT = ["context_id","alias_context_id","before_catalog_authorization_context_id","after_catalog_authorization_context_id","declared_sides","declared_host_ids","declared_connection_ids","capture_sides","capture_host_ids","capture_connection_ids","capture_recording_bases","capture_evidence_kinds","capture_watch_coverages","capture_remote_atomicities","capture_begin_bindings","capture_end_bindings","capture_begin_auth_contexts","capture_end_auth_contexts","capture_begin_config_revisions","capture_end_config_revisions","capture_begin_protocols","capture_end_protocols","capture_relevant_change_observed","page_capture_indexes","page_request_cursor_kinds","page_request_cursor_values","page_next_cursor_kinds","page_next_cursor_values","page_bindings","page_auth_contexts","page_config_revisions","page_protocols"]
ALIAS_OUTPUT = ["context_id","state","reason","before_index","after_index","conflict_catalog","conflict_first_index","conflict_second_index","execution_authorized"]
COLLECTION_OUTPUT = ["context_id","alias_context_id","state","reason","next_action","before_pagination_state","after_pagination_state","before_configuration_state","after_configuration_state","authorization_relation","recording_basis_state","remote_atomicity","first_problem_side","first_problem_code","first_problem_capture_index","first_problem_page_index","first_problem_declared_index","contains_synthetic_fixture","live_removal_proven","execution_authorized","capture_pagination_states","capture_configuration_states","capture_recording_bases","capture_evidence_kinds"]
# The raw result stays within the pinned 32-field record limit. Both public
# context references are mechanically read from the same compiler-owned field.
COLLECTION_RAW = {name: ('origin_context_id' if name == 'alias_context_id' else name) for name in COLLECTION_OUTPUT}
RAW_OUTPUT = {'origin_'+name for name in ALIAS_OUTPUT} | set(COLLECTION_RAW.values())
MODULE = 'github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison'

def digest(raw):
    return 'sha256:' + hashlib.sha256(raw).hexdigest()

def strict(raw):
    def unique(rows):
        out = {}
        for name, value in rows:
            if name in out:
                raise ValueError('duplicate JSON field: '+name)
            out[name] = value
        return out
    return json.loads(raw, object_pairs_hook=unique, parse_constant=lambda x: (_ for _ in ()).throw(ValueError(x)))

def read(path):
    with path.open('rb') as source:
        raw = source.read((32 << 20)+1)
    if len(raw) > 32 << 20:
        raise ValueError('finite renderer raw byte budget exceeded')
    if raw[:2] == bytes((31,139)):
        if len(raw) > 2 << 20:
            raise ValueError('finite renderer gzip byte budget exceeded')
        stream = zlib.decompressobj(16+zlib.MAX_WBITS)
        decoded = stream.decompress(raw, (32 << 20)+1)
        if len(decoded) > 32 << 20 or not stream.eof or stream.unused_data or stream.unconsumed_tail:
            raise ValueError('bounded complete single gzip receipt required')
        return decoded
    return raw

def go_json_string(value):
    return json.dumps(value, ensure_ascii=False).replace('<', '\\u003c').replace('>', '\\u003e').replace('&', '\\u0026').replace('\u2028', '\\u2028').replace('\u2029', '\\u2029').encode()

def same(left, right):
    assert json.dumps(left, sort_keys=True, separators=(',', ':')) == json.dumps(right, sort_keys=True, separators=(',', ':'))

def go_name(value):
    return ''.join({'id': 'ID', 'ids': 'IDs'}.get(part, part.title()) for part in value.split('_'))

def render(execution, replay, gofmt):
    cases_raw = read(HERE/'kernel.cases.json')
    cases = strict(cases_raw)['cases']
    manifest_raw = read(ROOT/'recorded-catalog-comparison.workspace.json')
    source_raw = {path: read(ROOT/path) for path in SOURCES.values()}
    assert gofmt.is_absolute() and gofmt.name == 'gofmt' and gofmt.is_file()
    selected_go = gofmt.parent/'go'
    assert selected_go.is_file()
    go_digest = digest(selected_go.read_bytes())
    first = None
    parent = read(execution)
    for receipt_path in (execution, replay):
        receipt = strict(read(receipt_path))
        assert receipt['decision'] == 'PASS'
        assert receipt['manifest_digest'] == digest(manifest_raw) and receipt['cases_digest'] == digest(cases_raw)
        result = receipt['result']
        runtime = result['runtime']
        assert runtime['producer_source_sha'] == PIN and runtime['go_version'].startswith('go version '+GO_VERSION+' ')
        assert runtime['go_tool_sha256'] == go_digest
        assert runtime['model_calls'] == 0 and runtime['finite_total'] == runtime['finite_passed'] == len(cases)
        assert runtime['projection_replayed'] is True and runtime['runtime_replayed'] is True
        packages = result['program']['workspace_image']['packages']
        assert len(packages) == len(SOURCES) and {package['path'] for package in packages} == set(SOURCES)
        for package in packages:
            path = SOURCES[package['path']]
            assert len(package['sources']) == 1
            source = package['sources'][0]
            assert source['filename'] == path and source['source_digest'] == digest(go_json_string(source_raw[path].decode()))
        origins = result['program']['activity_origins']
        wanted_origins = {
            ORIGIN_PREFIX+'compare': SOURCES['gooo/tool-origin'],
            PREFIX+'compare': SOURCES['gooo/tool-origin/recorded-catalog'],
        }
        assert len(origins) == len(wanted_origins) and {origin['lowered_activity_id'] for origin in origins} == set(wanted_origins)
        for origin in origins:
            source = origin['input_source']
            path = wanted_origins[origin['lowered_activity_id']]
            assert source['status'] == 'MAPPED' and source['filename'] == path and source['source_sha256'] == digest(source_raw[path])
        record_list = result['composition']['plan']['record_types']
        typed_records = {record['id']: record for record in record_list}
        assert len(typed_records) == len(record_list)
        semantic_fields = {
            ORIGIN_PREFIX+'input': {name: ORIGIN_PREFIX+'input/'+name for name in ORIGIN_INPUT},
            ORIGIN_PREFIX+'result': {name: ORIGIN_PREFIX+'result/'+name for name in ALIAS_OUTPUT},
            PREFIX+'input': {name: PREFIX+'input/'+name for name in COLLECTION_INPUT},
            PREFIX+'result': {name: PREFIX+'result/'+name for name in RAW_OUTPUT},
        }
        assert set(typed_records) == set(semantic_fields)
        for record_id, wanted in semantic_fields.items():
            fields = typed_records[record_id]['fields']
            assert len(fields) == len(wanted)
            same({field['name']: field['id'] for field in fields}, wanted)
        def check_observed_fields(fields, value, record_id):
            names = [field['name'] for field in fields]
            assert len(fields) == len(value) == len(set(names))
            same({field['name']: field['value'] for field in fields}, value)
            same({field['name']: field['id'] for field in fields}, semantic_fields[record_id])
        traces = runtime['traces']
        assert len(traces) == len(cases)
        for index, (trace, case) in enumerate(zip(traces, cases)):
            assert trace['case_index'] == index and len(trace['deliveries']) == 1
            delivery = trace['deliveries'][0]
            assert delivery['activity_id'] == PREFIX+'compare'
            assert len(delivery['inputs']) == 2
            for port_index, entity in enumerate((ORIGIN_PREFIX+'input', PREFIX+'input')):
                supplied = delivery['inputs'][port_index]
                assert supplied['port'] == 'input'+str(port_index) and supplied['entity_id'] == entity
                same(supplied['value'], case['inputs'][ENTRY+'.input'+str(port_index)])
                check_observed_fields(supplied['fields'], supplied['value'], entity)
            actual = delivery['actual']
            assert set(actual) == RAW_OUTPUT and actual['origin_execution_authorized'] is False and actual['execution_authorized'] is False and actual['live_removal_proven'] is False
            same(actual, case['expected'][ENTRY])
            check_observed_fields(delivery['actual_fields'], actual, PREFIX+'result')
        composition = result['composition']
        if first is None:
            first = composition
        else:
            assert receipt['replayed_from_sha256'] == digest(parent)
            same(composition, first)
    raw, plan = first['source'], first['plan']
    assert raw.startswith('package main\n') and 'import ' not in raw
    records = {record['id']: record for record in plan['record_types']}
    assert len(records) == len(plan['record_types'])
    expected_records = {ORIGIN_PREFIX+'input', ORIGIN_PREFIX+'result', PREFIX+'input', PREFIX+'result'}
    assert set(records) == expected_records
    origin_in, collection_in, origin_out, out = (records[name] for name in (ORIGIN_PREFIX+'input', PREFIX+'input', ORIGIN_PREFIX+'result', PREFIX+'result'))
    for record, names in ((origin_in, ORIGIN_INPUT), (collection_in, COLLECTION_INPUT), (origin_out, ALIAS_OUTPUT), (out, RAW_OUTPUT)):
        assert len(record['fields']) == len(names) and {field['name'] for field in record['fields']} == set(names)
    assert len(plan['activities']) == 1
    activity = plan['activities'][0]
    assert activity['id'] == PREFIX+'compare' and activity['output_entity_id'] == PREFIX+'result'
    same([(item['port'], item['entity_id']) for item in activity['inputs']], [('input0', ORIGIN_PREFIX+'input'), ('input1', PREFIX+'input')])
    for token in [activity['go_function'], *[record['go_name'] for record in records.values()], *[field['go_name'] for record in records.values() for field in record['fields']]]:
        assert re.fullmatch(r'[A-Za-z_][A-Za-z0-9_]*', token)
    primitive = {'urn:gooo:type:string': 'string', 'urn:gooo:type:integer': 'int64', 'urn:gooo:type:boolean': 'bool'}
    def field_map(record):
        return {field['name']: field for field in record['fields']}
    def type_of(field):
        return ('[]' if field.get('cardinality', 'one') == 'many' else '') + primitive[field['type_id']]
    types = '// Code generated from the Gooo typed plan; DO NOT EDIT.\npackage kernel\n'
    layouts = [('OriginInput', origin_in, {name:name for name in ORIGIN_INPUT}),
               ('CollectionInput', collection_in, {name:name for name in COLLECTION_INPUT}),
               ('AliasDecision', origin_out, {name:name for name in ALIAS_OUTPUT}),
               ('CollectionDecision', out, COLLECTION_RAW)]
    for name, record, mapping in layouts:
        columns = field_map(record)
        types += '\ntype '+name+' struct {\n'
        for public, original in mapping.items():
            types += '\t'+go_name(public)+' '+type_of(columns[original])+' `json:"'+public+'"`\n'
        types += '}\n'
    types += '\ntype Decision struct {\n\tAlias AliasDecision `json:"alias"`\n\tCollection CollectionDecision `json:"collection"`\n}\n'
    bridge = '// Code generated from compiler-owned record/field identities; DO NOT EDIT.\npackage kernel\n\nfunc Evaluate(origin OriginInput, context CollectionInput) Decision {\n'
    for local, public, record in [('nativeOrigin','origin',origin_in),('nativeContext','context',collection_in)]:
        bridge += '\t'+local+' := '+record['go_name']+'{\n'
        for field in record['fields']:
            bridge += '\t\t'+field['go_name']+': '+public+'.'+go_name(field['name'])+',\n'
        bridge += '\t}\n'
    bridge += '\tactual := '+activity['go_function']+'(nativeOrigin, nativeContext)\n\treturn Decision{\n\t\tAlias: AliasDecision{\n'
    output_fields = field_map(out)
    for field in ALIAS_OUTPUT:
        bridge += '\t\t\t'+go_name(field)+': actual.'+output_fields['origin_'+field]['go_name']+',\n'
    bridge += '\t\t},\n\t\tCollection: CollectionDecision{\n'
    for field, original in COLLECTION_RAW.items():
        bridge += '\t\t\t'+go_name(field)+': actual.'+output_fields[original]['go_name']+',\n'
    bridge += '\t\t},\n\t}\n}\n'
    produced = {'internal/kernel/projection_generated.go': raw.replace('package main\n', 'package kernel\n', 1).encode()}
    for name, value in [('types_generated.go', types), ('bridge_generated.go', bridge)]:
        completed = subprocess.run([str(gofmt)], input=value.encode(), capture_output=True, timeout=15, check=True)
        assert not completed.stderr
        produced['internal/kernel/'+name] = completed.stdout
    produced['go.mod'] = ('module '+MODULE+'\n\ngo '+GO_VERSION[2:]+'\n').encode()
    manifest = {
        'schema': 'gooo/recorded-catalog-generation/v1', 'compiler_source_sha': PIN, 'go_version': GO_VERSION,
        'entry': ENTRY, 'activity_id': PREFIX+'compare',
        'source': {path: digest(value) for path, value in source_raw.items()},
        'workspace_sha256': digest(manifest_raw), 'cases_sha256': digest(cases_raw),
        'generator_sha256': digest(Path(__file__).read_bytes()), 'selected_upstream_pins_sha256': digest(SELECTED_PIN_BYTES),
        'typed_plan_sha256': plan['typed_plan_sha256'], 'raw_projection_sha256': digest(raw.encode()),
        'projection_transform': 'Replace first package token main with kernel; all other bytes unchanged.',
        'collection_field_mapping': COLLECTION_RAW,
        'alias_field_mapping': {name:'origin_'+name for name in ALIAS_OUTPUT},
        'generated_assets': {name: digest(value) for name, value in produced.items()},
        'scope': 'Finite source-bound native execution/replay; saved declared context, not authenticated collection, live removal or effect authority.'
    }
    produced['generation.json'] = (json.dumps(manifest, indent=2, sort_keys=True)+'\n').encode()
    return produced

def provenance(args, produced):
    used = [ROOT/'upstream.json', ROOT/'recorded-catalog-comparison.workspace.json',
            *[ROOT/path for path in SOURCES.values()], HERE/'kernel.cases.json',
            Path(__file__), args.execution, args.replay, args.gofmt, args.gofmt.parent/'go']
    activity = 'urn:uuid:'+str(uuid.uuid4())
    graph, parents, entities = [], [], {}
    for path in used:
        raw = path.read_bytes()
        identity = 'urn:sha256:'+digest(raw)[7:]
        node = {'@id':identity, '@type':'prov:Entity', 'path':str(path.relative_to(ROOT)) if path.is_relative_to(ROOT) else str(path), 'sha256':digest(raw)}
        if path in (args.execution,args.replay):
            node['original_JSON_sha256'] = digest(read(path))
        graph.append(node); entities[path] = {'@id':identity}; parents.append(entities[path])
    projection = {'@id':'urn:sha256:'+digest(SELECTED_PIN_BYTES)[7:]}
    graph.append({'@id':projection['@id'],'@type':'prov:Entity','path':'upstream.json#consumed-generator-pins',
                  'sha256':digest(SELECTED_PIN_BYTES),'jsonPointers':['/compiler_revision','/go_version'],
                  'canonicalJSON':SELECTED_PIN_BYTES.decode(),'prov:wasDerivedFrom':[entities[ROOT/'upstream.json']]})
    parents.append(projection)
    graph.append({'@id':activity,'@type':'prov:Activity','prov:used':parents,
                  'prov:endedAtTime':{'@value':datetime.datetime.now(datetime.timezone.utc).isoformat(),'@type':'http://www.w3.org/2001/XMLSchema#dateTime'}})
    for name, raw in produced.items():
        dependencies = [projection,entities[Path(__file__)]] if name == 'go.mod' else [projection if path==ROOT/'upstream.json' else entities[path] for path in used]
        graph.append({'@id':'urn:sha256:'+digest(raw)[7:],'@type':'prov:Entity','path':name,'sha256':digest(raw),
                      'prov:wasGeneratedBy':{'@id':activity},'prov:wasDerivedFrom':dependencies})
    return {'@context':{'prov':'http://www.w3.org/ns/prov#','gooo':'https://github.com/kimjooyoon/gooo-go-ports/provenance#','path':'gooo:path','sha256':'gooo:sha256','original_JSON_sha256':'gooo:originalJSONSHA256','scope':'gooo:scope','jsonPointers':'gooo:jsonPointers','canonicalJSON':'gooo:canonicalJSON'},'@graph':graph,
            'scope':'Actual local rendering derivation only. This activity did not collect the saved external observations.'}

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--execution', type=Path, required=True)
    parser.add_argument('--replay', type=Path, required=True)
    parser.add_argument('--gofmt', type=Path, required=True)
    parser.add_argument('--out', type=Path)
    parser.add_argument('--check', action='store_true')
    args = parser.parse_args()
    assert args.gofmt.is_absolute() and (args.check != (args.out is not None))
    produced = render(args.execution,args.replay,args.gofmt)
    if args.check:
        for name, raw in produced.items():
            assert (HERE/name).read_bytes() == raw, name
    else:
        assert args.out.is_absolute() and not args.out.exists() and not args.out.resolve().is_relative_to(ROOT.resolve())
        args.out.mkdir()
        for name, raw in produced.items():
            target = args.out/name
            target.parent.mkdir(parents=True,exist_ok=True)
            target.write_bytes(raw)
        (args.out/'generation.provenance.jsonld').write_text(json.dumps(provenance(args,produced),indent=2)+'\n')
    print(json.dumps({'files':list(produced),'status':'CHECKED_EXACT_GENERATED_BYTES' if args.check else 'GENERATED_FROM_FRESH_SOURCE_BOUND_NATIVE_PAIR'}))
