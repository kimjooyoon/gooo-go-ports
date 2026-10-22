"""Render an internal Go kernel only from pinned fresh execution/replay bytes.

This checks finite consistency and identities, not unsigned producer authenticity.
No handwritten Go policy or origin comparison implementation is emitted.
"""
import argparse
import hashlib
import json
import zlib
import re
import datetime
import uuid
from pathlib import Path
import subprocess

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
FACTS = json.loads((ROOT / 'upstream.json').read_bytes())
PIN = FACTS['compiler_revision']
GO_VERSION = FACTS['go_version']
SELECTED_PINS = {'compiler_revision': PIN, 'go_version': GO_VERSION}
SELECTED_PIN_BYTES = json.dumps(SELECTED_PINS, sort_keys=True, separators=(',', ':')).encode()
assert re.fullmatch(r'go1[.][0-9]+[.][0-9]+', GO_VERSION)
ENTRY = 'gooo/tool-origin:CompareAliasOrigin'
PREFIX = 'gooo://tool-origin/v1/'
INPUTS = {'context_id':'ContextID','target_alias':'TargetAlias','before_servers':'BeforeServers',
          'before_names':'BeforeNames','before_aliases':'BeforeAliases','after_servers':'AfterServers',
          'after_names':'AfterNames','after_aliases':'AfterAliases'}
OUTPUTS = {'context_id':'ContextID','state':'State','reason':'Reason','before_index':'BeforeIndex',
           'after_index':'AfterIndex','conflict_catalog':'ConflictCatalog','conflict_first_index':'ConflictFirstIndex',
           'conflict_second_index':'ConflictSecondIndex','execution_authorized':'ExecutionAuthorized'}

def digest(raw):
    return 'sha256:' + hashlib.sha256(raw).hexdigest()

def strict(raw):
    def unique(rows):
        result = {}
        for name, value in rows:
            if name in result:
                raise ValueError('duplicate JSON field: ' + name)
            result[name] = value
        return result
    return json.loads(raw, object_pairs_hook=unique,
                      parse_constant=lambda x: (_ for _ in ()).throw(ValueError(x)))

def read(path):
    with path.open('rb') as source:
        raw=source.read((32 << 20)+1)
    if len(raw)>32 << 20:
        raise ValueError('finite renderer raw byte budget exceeded')
    if raw[:2] == bytes((31,139)):
        if len(raw)>2 << 20:
            raise ValueError('finite renderer gzip byte budget exceeded')
        stream=zlib.decompressobj(16+zlib.MAX_WBITS)
        decoded=stream.decompress(raw,(32 << 20)+1)
        if len(decoded)>32 << 20 or not stream.eof or stream.unused_data or stream.unconsumed_tail:
            raise ValueError('bounded single complete gzip receipt required')
        return decoded
    return raw

def go_json_string(text):
    return json.dumps(text, ensure_ascii=False).replace('<', '\\u003c').replace('>', '\\u003e').replace('&', '\\u0026').replace('\u2028', '\\u2028').replace('\u2029', '\\u2029').encode()

def render(execution, replay, gofmt):
    cases_raw = read(HERE / 'kernel.cases.json')
    cases = strict(cases_raw)['cases']
    source_raw = read(ROOT / 'ports/tool-origin/alias_origin.gooo')
    manifest_raw = read(ROOT / 'alias-origin-change.workspace.json')
    assert gofmt.is_absolute() and gofmt.name=='gofmt' and gofmt.is_file()
    selected_go = gofmt.parent/'go'
    assert selected_go.is_file()
    go_digest = digest(selected_go.read_bytes())
    parent = read(execution)
    first = None
    for path in (execution, replay):
        receipt = strict(read(path))
        runtime = receipt['result']['runtime']
        assert receipt['decision'] == 'PASS'
        assert receipt['manifest_digest'] == digest(manifest_raw) and receipt['cases_digest'] == digest(cases_raw)
        assert runtime['producer_source_sha'] == PIN and runtime['go_version'].startswith('go version '+GO_VERSION+' ')
        assert runtime['go_tool_sha256'] == go_digest
        assert runtime['model_calls'] == 0 and runtime['finite_total'] == runtime['finite_passed'] == len(cases)
        packages = receipt['result']['program']['workspace_image']['packages']
        assert len(packages) == 1 and packages[0]['path'] == 'gooo/tool-origin'
        sources = packages[0]['sources']
        assert len(sources) == 1 and sources[0]['filename'] == 'ports/tool-origin/alias_origin.gooo'
        assert sources[0]['source_digest'] == digest(go_json_string(source_raw.decode()))
        assert len(runtime['traces']) == len(cases)
        for index, (trace, case) in enumerate(zip(runtime['traces'], cases)):
            assert trace['case_index'] == index and len(trace['deliveries']) == 1
            delivery = trace['deliveries'][0]
            assert delivery['activity_id'] == PREFIX + 'compare'
            assert delivery['input'] == case['inputs'][ENTRY]
            actual, wanted = delivery['actual'], case['expected'][ENTRY]
            # Canonical JSON distinguishes bool/int and preserves every field.
            assert json.dumps(actual, sort_keys=True) == json.dumps(wanted, sort_keys=True)
            assert set(actual) == set(OUTPUTS) and actual['execution_authorized'] is False
        composition = receipt['result']['composition']
        if first is None:
            first = composition
        else:
            assert receipt['replayed_from_sha256'] == digest(parent) and composition == first
    raw, plan = first['source'], first['plan']
    assert raw.startswith('package main\n') and 'import ' not in raw
    records = {r['id']: r for r in plan['record_types']}
    inp, out = records[PREFIX+'input'], records[PREFIX+'result']
    assert {f['name'] for f in inp['fields']} == set(INPUTS)
    assert {f['name'] for f in out['fields']} == set(OUTPUTS)
    activity = next(a for a in plan['activities'] if a['id'] == PREFIX+'compare')
    assert len(plan['activities']) == 1 and set(records) == {PREFIX+'input', PREFIX+'result'}
    for token in [inp['go_name'],out['go_name'],activity['go_function'],*[f['go_name'] for r in records.values() for f in r['fields']]]:
        assert re.fullmatch(r'[A-Za-z_][A-Za-z0-9_]*',token)
    primitive = {'urn:gooo:type:string': 'string', 'urn:gooo:type:integer': 'int64', 'urn:gooo:type:boolean': 'bool'}
    types = '// Code generated from the Gooo typed plan; DO NOT EDIT.\npackage kernel\n'
    for name, record, fields in [('Input', inp, INPUTS), ('Decision', out, OUTPUTS)]:
        types += '\ntype '+name+' struct {\n'
        for field in record['fields']:
            typ = primitive[field['type_id']]
            if field.get('cardinality', 'one') == 'many':
                typ = '[]'+typ
            types += '\t'+fields[field['name']]+' '+typ+' `json:"'+field['name']+'"`\n'
        types += '}\n'
    bridge = '// Code generated from compiler-owned record/field identities; DO NOT EDIT.\npackage kernel\n\nfunc Evaluate(input Input) Decision {\n\tnative := '+inp['go_name']+'{\n'
    for field in inp['fields']:
        bridge += '\t\t'+field['go_name']+': input.'+INPUTS[field['name']]+',\n'
    bridge += '\t}\n\tactual := '+activity['go_function']+'(native)\n\treturn Decision{\n'
    for field in out['fields']:
        bridge += '\t\t'+OUTPUTS[field['name']]+': actual.'+field['go_name']+',\n'
    bridge += '\t}\n}\n'
    produced = {'internal/kernel/projection_generated.go': raw.replace('package main\n', 'package kernel\n', 1).encode()}
    for name, value in [('types_generated.go', types), ('bridge_generated.go', bridge)]:
        r = subprocess.run([str(gofmt)], input=value.encode(), capture_output=True, timeout=15, check=True)
        assert not r.stderr
        produced['internal/kernel/'+name] = r.stdout
    validator = strict(read(HERE/'schema-validation.json'))
    assert validator['module']=='github.com/google/jsonschema-go' and validator['scope']=='tests_only' and re.fullmatch(r'v[0-9]+[.][0-9]+[.][0-9]+',validator['version'])
    assert all(re.fullmatch(r'github[.]com/google/(jsonschema-go|go-cmp) v[0-9]+[.][0-9]+[.][0-9]+(/go[.]mod)? h1:[A-Za-z0-9+/=]+',line) for line in validator['checksums'])
    produced['go.mod'] = ('module github.com/kimjooyoon/gooo-go-ports/examples/alias-origin-change\n\ngo '+GO_VERSION[2:]+'\n\nrequire '+validator['module']+' '+validator['version']+'\n').encode()
    produced['go.sum'] = ('\n'.join(validator['checksums'])+'\n').encode()
    produced['reference/go.mod'] = ('module gooo-alias-origin-reference\n\ngo '+GO_VERSION[2:]+'\n').encode()
    manifest = {'schema': 'gooo/alias-origin-generation/v1', 'compiler_source_sha': PIN,
        'go_version': GO_VERSION, 'source': {'ports/tool-origin/alias_origin.gooo': digest(source_raw)},
        'workspace_sha256': digest(manifest_raw), 'cases_sha256': digest(cases_raw),
        'generator_sha256': digest(Path(__file__).read_bytes()), 'selected_upstream_pins_sha256': digest(SELECTED_PIN_BYTES),
        'schema_validation_spec_sha256': digest((HERE/'schema-validation.json').read_bytes()),
        'typed_plan_sha256': plan['typed_plan_sha256'], 'raw_projection_sha256': digest(raw.encode()),
        'projection_transform': 'Replace first package token main with kernel; all other bytes unchanged.',
        'generated_assets': {name: digest(value) for name, value in produced.items()},
        'scope': 'Finite native/replay source-bound generation; not authenticated authority or host integration proof.'}
    produced['generation.json'] = (json.dumps(manifest, indent=2, sort_keys=True)+'\n').encode()
    return produced

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--execution',type=Path,required=True)
    parser.add_argument('--replay',type=Path,required=True)
    parser.add_argument('--gofmt',type=Path,required=True)
    parser.add_argument('--out',type=Path)
    parser.add_argument('--check',action='store_true')
    args=parser.parse_args()
    assert args.gofmt.is_absolute() and (args.check != (args.out is not None))
    produced=render(args.execution,args.replay,args.gofmt)
    if args.check:
        for name,raw in produced.items():assert (HERE/name).read_bytes()==raw,name
    else:
        assert args.out.is_absolute() and not args.out.exists() and not args.out.resolve().is_relative_to(ROOT.resolve())
        args.out.mkdir()
        for name,raw in produced.items():
            target=args.out/name;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(raw)
        used=[ROOT/'upstream.json',ROOT/'ports/tool-origin/alias_origin.gooo',ROOT/'alias-origin-change.workspace.json',HERE/'kernel.cases.json',HERE/'schema-validation.json',Path(__file__),args.execution,args.replay,args.gofmt,args.gofmt.parent/'go']
        activity='urn:uuid:'+str(uuid.uuid4());graph=[];parents=[];entities={}
        for path in used:
            raw=path.read_bytes();identity='urn:sha256:'+digest(raw)[7:];node={'@id':identity,'@type':'prov:Entity','path':str(path.relative_to(ROOT)) if path.is_relative_to(ROOT) else str(path),'sha256':digest(raw)}
            if path in (args.execution,args.replay):node['original_JSON_sha256']=digest(read(path))
            graph.append(node);reference={'@id':identity};parents.append(reference);entities[path]=reference
        projection_id='urn:sha256:'+digest(SELECTED_PIN_BYTES)[7:]
        projection={'@id':projection_id}
        graph.append({'@id':projection_id,'@type':'prov:Entity','path':'upstream.json#consumed-generator-pins','sha256':digest(SELECTED_PIN_BYTES),'jsonPointers':['/compiler_revision','/go_version'],'canonicalJSON':SELECTED_PIN_BYTES.decode(),'prov:wasDerivedFrom':[entities[ROOT/'upstream.json']]})
        parents.append(projection)
        graph.append({'@id':activity,'@type':'prov:Activity','prov:used':parents,'prov:endedAtTime':{'@value':datetime.datetime.now(datetime.timezone.utc).isoformat(),'@type':'http://www.w3.org/2001/XMLSchema#dateTime'}})
        for name,raw in produced.items():
            if name in ('go.mod','reference/go.mod'):
                dependencies=[ROOT/'upstream.json',Path(__file__)]
                if name=='go.mod':dependencies.append(HERE/'schema-validation.json')
            elif name=='go.sum':dependencies=[HERE/'schema-validation.json',Path(__file__)]
            else:dependencies=used
            graph.append({'@id':'urn:sha256:'+digest(raw)[7:],'@type':'prov:Entity','path':name,'sha256':digest(raw),'prov:wasGeneratedBy':{'@id':activity},'prov:wasDerivedFrom':[projection if path==ROOT/'upstream.json' else entities[path] for path in dependencies]})
        proof={'@context':{'prov':'http://www.w3.org/ns/prov#','gooo':'https://github.com/kimjooyoon/gooo-go-ports/provenance#','path':'gooo:path','sha256':'gooo:sha256','original_JSON_sha256':'gooo:originalJSONSHA256','scope':'gooo:scope','jsonPointers':'gooo:jsonPointers','canonicalJSON':'gooo:canonicalJSON'},'@graph':graph,'scope':'Actual local rendering derivation; not signed producer/current-state/effect authority.'}
        (args.out/'generation.provenance.jsonld').write_text(json.dumps(proof,indent=2)+'\n')
    print(json.dumps({'files':list(produced),'status':'GENERATED_FROM_FRESH_SOURCE_BOUND_NATIVE_PAIR' if args.out else 'CHECKED_EXACT_GENERATED_BYTES'}))
