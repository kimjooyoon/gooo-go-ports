"""Generate the standalone Gooo evaluator from all 60 native/replay cases."""
import argparse
import datetime
import importlib.util
import json
import subprocess
import uuid
from pathlib import Path

HERE=Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('definition_evidence',HERE/'evidence.py')
e=importlib.util.module_from_spec(spec);spec.loader.exec_module(e)
ROOT=e.ROOT
MODULE_ROOT=e.OUTPUT_ROOT
PIN_BYTES=json.dumps({'compiler_revision':e.PIN,'go_version':e.GO_VERSION},sort_keys=True,separators=(',',':')).encode()


def render(index,gofmt):
    assert gofmt.is_absolute() and gofmt.name=='gofmt' and gofmt.is_file()
    go=gofmt.parent/'go';assert go.is_file()
    version=subprocess.run([str(go),'version'],check=True,capture_output=True,timeout=15).stdout.decode().strip()
    assert version in ('go version '+e.GO_VERSION+' darwin/arm64','go version '+e.GO_VERSION+' linux/amd64')
    identity=dict(e.host.ref(go),version=version)
    composition,used=e.verify_index(index,identity)
    plan=composition['plan'];records,activity=e.semantic_records(plan)
    raw=composition['source']
    assert raw.startswith('package main\n') and 'import ' not in raw
    layouts=[('OriginInput',records[e.CORE+'input']),('CollectionInput',records[e.RECORDED+'input']),('DefinitionInput',records[e.PREFIX+'input']),('Decision',records[e.PREFIX+'result'])]
    primitive={'urn:gooo:type:string':'string','urn:gooo:type:integer':'int64','urn:gooo:type:boolean':'bool'}
    types='// Code generated from the source-bound Gooo typed plan; DO NOT EDIT.\npackage definitionkernel\n'
    for name,record in layouts:
        types+='\ntype '+name+' struct {\n'
        for field in record['fields']:
            kind=('[]' if field.get('cardinality','one')=='many' else '')+primitive[field['type_id']]
            types+='\t'+e.base.go_name(field['name'])+' '+kind+' `json:"'+field['name']+'"`\n'
        types+='}\n'
    bridge='// Code generated from compiler-owned record/field identities; DO NOT EDIT.\npackage definitionkernel\n\nfunc Evaluate(origin OriginInput, collection CollectionInput, binding DefinitionInput) Decision {\n'
    for local,public,record in [('nativeOrigin','origin',layouts[0][1]),('nativeCollection','collection',layouts[1][1]),('nativeBinding','binding',layouts[2][1])]:
        bridge+='\t'+local+' := '+record['go_name']+'{\n'
        for field in record['fields']:
            bridge+='\t\t'+field['go_name']+': '+public+'.'+e.base.go_name(field['name'])+',\n'
        bridge+='\t}\n'
    bridge+='\tactual := '+activity['go_function']+'(nativeOrigin, nativeCollection, nativeBinding)\n\treturn Decision{\n'
    for field in layouts[3][1]['fields']:
        bridge+='\t\t'+e.base.go_name(field['name'])+': actual.'+field['go_name']+',\n'
    bridge+='\t}\n}\n'
    generated={MODULE_ROOT+'/internal/definitionkernel/projection_generated.go':raw.replace('package main\n','package definitionkernel\n',1).encode()}
    for name,value in [('types_generated.go',types),('bridge_generated.go',bridge)]:
        p=subprocess.run([str(gofmt)],input=value.encode(),capture_output=True,check=True,timeout=15)
        assert not p.stderr
        generated[MODULE_ROOT+'/internal/definitionkernel/'+name]=p.stdout
    generated[MODULE_ROOT+'/definition-kernel-cases.json']=e.read(HERE/'kernel.cases.json',2<<20)
    metadata={'schema':'gooo/definition-binding-generation/v1','compiler_source_sha':e.PIN,'go_version':e.GO_VERSION,'entry':e.ENTRY,'activity_id':e.PREFIX+'assess',
              'source':{p:e.sha(e.read(ROOT/p)) for p in e.SOURCES.values()},'workspace_sha256':e.sha(e.read(e.MANIFEST)),
              'cases_sha256':e.sha(e.read(HERE/'kernel.cases.json')),'cases':60,'batches':8,
              'reference_source':{p:e.sha(e.read(HERE/p)) for p in e.REFERENCE},
              'generator_source':{str(p.relative_to(ROOT)):e.sha(e.read(p)) for p in [Path(__file__),HERE/'evidence.py',ROOT/'examples/recorded-catalog-comparison/generate.py',ROOT/'examples/service-grant-many/verify.py']},
              'selected_upstream_pins_sha256':e.sha(PIN_BYTES),'typed_plan_sha256':plan['typed_plan_sha256'],'raw_projection_sha256':e.sha(raw.encode()),
              'projection_transform':'Replace first package token main with definitionkernel; all other bytes unchanged.',
              'generated_assets':{p:e.sha(b) for p,b in generated.items()},
              'scope':'All60 finite synthetic cases and their own saved replay. Matching supplied metadata is not authenticated origin, behavior equivalence, capability truth or effect authorization.'}
    generated[MODULE_ROOT+'/definition-generation.json']=(json.dumps(metadata,indent=2,sort_keys=True)+'\n').encode()
    assert set(generated)==e.ASSETS
    return generated,used


def provenance(index,gofmt,generated,used):
    paths=[ROOT/'upstream.json',e.MANIFEST,*[ROOT/p for p in e.SOURCES.values()],HERE/'kernel.cases.json',*[HERE/p for p in e.REFERENCE],
           Path(__file__),HERE/'evidence.py',ROOT/'examples/recorded-catalog-comparison/generate.py',ROOT/'examples/service-grant-many/verify.py',index,gofmt,gofmt.parent/'go',*used]
    assert len(set(paths))==len(paths)
    graph=[];parents=[]
    receipt_paths={e.local_member(index.parent,b[k]) for b in e.host.strict(e.read(index))['batches'] for k in ('execution','replay')}
    upstream=None
    for path in paths:
        raw=e.read(path,64<<20);identity='urn:'+e.sha(raw)
        node={'@id':identity,'@type':'prov:Entity','path':str(path.relative_to(ROOT)) if path.is_relative_to(ROOT) else str(path),'sha256':e.sha(raw)}
        if path in receipt_paths:node['original_JSON_sha256']=e.sha(e.receipt(path)[0])
        if path==ROOT/'upstream.json':upstream={'@id':identity}
        graph.append(node);parents.append({'@id':identity})
    selected={'@id':'urn:'+e.sha(PIN_BYTES)}
    graph.append({'@id':selected['@id'],'@type':'prov:Entity','path':'upstream.json#consumed-definition-pins','sha256':e.sha(PIN_BYTES),'canonicalJSON':PIN_BYTES.decode(),'jsonPointers':['/compiler_revision','/go_version'],'prov:wasDerivedFrom':[upstream]})
    activity='urn:uuid:'+str(uuid.uuid4())
    graph.append({'@id':activity,'@type':'prov:Activity','prov:used':parents+[selected],'prov:endedAtTime':{'@value':datetime.datetime.now(datetime.timezone.utc).isoformat(),'@type':'http://www.w3.org/2001/XMLSchema#dateTime'}})
    dependencies=[selected,*parents[1:]]
    for name,raw in generated.items():
        graph.append({'@id':'urn:'+e.sha(raw),'@type':'prov:Entity','path':name,'sha256':e.sha(raw),'prov:wasGeneratedBy':{'@id':activity},'prov:wasDerivedFrom':dependencies})
    return {'@context':{'prov':'http://www.w3.org/ns/prov#','gooo':'https://github.com/kimjooyoon/gooo-go-ports/provenance#','path':'gooo:path','sha256':'gooo:sha256','original_JSON_sha256':'gooo:originalJSONSHA256','canonicalJSON':'gooo:canonicalJSON','jsonPointers':'gooo:jsonPointers'},'@graph':graph,'scope':'Actual local generation dependencies only; no producer authentication, live capture or effect authorization.'}


def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--index',type=Path,required=True)
    parser.add_argument('--gofmt',type=Path,required=True)
    parser.add_argument('--out',type=Path)
    parser.add_argument('--check',action='store_true')
    args=parser.parse_args()
    assert args.index.is_absolute() and args.gofmt.is_absolute() and args.check!=(args.out is not None)
    generated,used=render(args.index,args.gofmt)
    if args.check:
        for name,raw in generated.items():assert e.read(ROOT/name,4<<20)==raw,name
    else:
        assert args.out.is_absolute() and not args.out.exists() and not args.out.resolve().is_relative_to(ROOT.resolve())
        args.out.mkdir()
        for name,raw in generated.items():
            p=args.out/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(raw)
        (args.out/'generation.provenance.jsonld').write_text(json.dumps(provenance(args.index,args.gofmt,generated,used),indent=2)+'\n')
    print(json.dumps({'status':'CHECKED_EXACT_GENERATED_BYTES' if args.check else 'GENERATED_FROM_ALL60_SOURCE_BOUND_NATIVE_REPLAY_CASES','files':sorted(generated)}))

if __name__=='__main__':main()
