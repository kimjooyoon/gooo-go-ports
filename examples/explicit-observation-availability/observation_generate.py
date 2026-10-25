"""Compile the source-bound finite field evaluator into the existing Go module."""
import argparse,datetime,importlib.util,json,subprocess,uuid
from pathlib import Path
HERE=Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('observation_evidence',HERE/'observation_evidence.py');e=importlib.util.module_from_spec(spec);spec.loader.exec_module(e)
ROOT=e.ROOT;OUTPUT='examples/recorded-catalog-comparison'
ASSETS={OUTPUT+'/internal/availabilityobservationkernel/'+n for n in ('projection_generated.go','types_generated.go','bridge_generated.go')}|{OUTPUT+'/availability-observation-generation.json',OUTPUT+'/availability-observation-kernel-cases.json'}
REFERENCES=[HERE/'observation-reference/main.go',HERE/'observation-reference/go.mod',HERE/'observation-reference-inputs.json']
HELPERS=[Path(__file__),HERE/'observation_evidence.py',ROOT/'examples/tool-definition-binding/evidence.py',ROOT/'examples/recorded-catalog-comparison/generate.py',ROOT/'examples/service-grant-many/verify.py']
PINS=json.dumps({'compiler_revision':e.PIN,'go_version':e.GO_VERSION},sort_keys=True,separators=(',',':')).encode()

def render(index,gofmt):
 assert gofmt.is_absolute() and gofmt.name=='gofmt' and gofmt.is_file()
 go=gofmt.parent/'go';assert go.is_file()
 version=subprocess.run([str(go),'version'],check=True,capture_output=True,timeout=15).stdout.decode().strip()
 assert version in ('go version '+e.GO_VERSION+' darwin/arm64','go version '+e.GO_VERSION+' linux/amd64')
 composition,used=e.verify_index(index,dict(e.host.ref(go),version=version))
 raw=composition['source'];assert raw.startswith('package main\n') and 'import ' not in raw
 records,activity=e.semantic_records(composition['plan'])
 layouts=[('FieldInput',records[e.PREFIX+'input']),('Result',records[e.PREFIX+'result'])]
 types='// Code generated from the source-bound Gooo typed plan; DO NOT EDIT.\npackage availabilityobservationkernel\n'
 for name,record in layouts:
  types+='\ntype '+name+' struct {\n'
  for field in record['fields']:
   primitive={'urn:gooo:type:string':'string','urn:gooo:type:boolean':'bool'}[field['type_id']]
   types+='\t'+e.shared.base.go_name(field['name'])+' '+primitive+' `json:"'+field['name']+'"`\n'
  types+='}\n'
 source_record=layouts[0][1]
 bridge='// Code generated from compiler-owned record/field identities; DO NOT EDIT.\npackage availabilityobservationkernel\n\nfunc Evaluate(input FieldInput) Result {\n\tnative := '+source_record['go_name']+'{\n'
 for field in source_record['fields']:bridge+='\t\t'+field['go_name']+': input.'+e.shared.base.go_name(field['name'])+',\n'
 bridge+='\t}\n\tactual := '+activity['go_function']+'(native)\n\treturn Result{\n'
 for field in layouts[1][1]['fields']:bridge+='\t\t'+e.shared.base.go_name(field['name'])+': actual.'+field['go_name']+',\n'
 bridge+='\t}\n}\n'
 generated={OUTPUT+'/internal/availabilityobservationkernel/projection_generated.go':raw.replace('package main\n','package availabilityobservationkernel\n',1).encode()}
 for name,value in [('types_generated.go',types),('bridge_generated.go',bridge)]:
  result=subprocess.run([str(gofmt)],input=value.encode(),capture_output=True,check=True,timeout=15);assert not result.stderr
  generated[OUTPUT+'/internal/availabilityobservationkernel/'+name]=result.stdout
 generated[OUTPUT+'/availability-observation-kernel-cases.json']=e.read(HERE/'observation-kernel.cases.json',2<<20)
 metadata={'schema':'gooo/explicit-observation-generation/v1','compiler_source_sha':e.PIN,'go_version':e.GO_VERSION,'entry':e.ENTRY,'activity_id':e.PREFIX+'assess','source':{e.SOURCE:e.sha(e.read(ROOT/e.SOURCE))},'workspace_sha256':e.sha(e.read(e.MANIFEST)),'cases_sha256':e.sha(e.read(HERE/'observation-kernel.cases.json')),'cases':12,'batches':2,'reference_source':{str(p.relative_to(ROOT)):e.sha(e.read(p)) for p in REFERENCES},'generator_source':{str(p.relative_to(ROOT)):e.sha(e.read(p)) for p in HELPERS},'selected_upstream_pins_sha256':e.sha(PINS),'typed_plan_sha256':composition['plan']['typed_plan_sha256'],'raw_projection_sha256':e.sha(raw.encode()),'projection_transform':'Replace first package token main with availabilityobservationkernel; all other bytes unchanged.','generated_assets':{p:e.sha(b) for p,b in generated.items()},'scope':'Finite synthetic field diagnostics and own saved replay. Supplied text and matching observations do not authenticate producer, prove behavior equivalence or authorize effects.'}
 generated[OUTPUT+'/availability-observation-generation.json']=(json.dumps(metadata,sort_keys=True,indent=2)+'\n').encode();assert set(generated)==ASSETS
 return generated,used

def provenance(index,gofmt,generated,used):
 profile='gooo/observation-diagnostics-file-occurrence/v1';activity='urn:uuid:'+str(uuid.uuid4())
 def identity(role,path,digest):
  raw=json.dumps([profile,activity,role,path,digest],ensure_ascii=True,separators=(',',':')).encode()
  return 'urn:gooo:observation-diagnostics:entity:'+e.sha(raw).removeprefix('sha256:')
 paths=[ROOT/'upstream.json',e.MANIFEST,ROOT/e.SOURCE,HERE/'observation-kernel.cases.json',*REFERENCES,*HELPERS,index,gofmt,gofmt.parent/'go',*used]
 assert len(set(paths))==len(paths)
 receipt_paths={e.shared.local_member(index.parent,b[k]) for b in e.host.strict(e.read(index))['batches'] for k in ('execution','replay')}
 graph=[];parents=[]
 for path in paths:
  raw=e.read(path,64<<20);name=str(path.relative_to(ROOT)) if path.is_relative_to(ROOT) else str(path);digest=e.sha(raw)
  node={'@id':identity('input',name,digest),'@type':'prov:Entity','path':name,'role':'input','sha256':digest}
  if path in receipt_paths:node['original_JSON_sha256']=e.sha(e.receipt(path)[0])
  graph.append(node);parents.append({'@id':node['@id']})
 name='upstream.json#consumed-observation-pins';selected={'@id':identity('selected-pins',name,e.sha(PINS))}
 graph.append({'@id':selected['@id'],'@type':'prov:Entity','path':name,'role':'selected-pins','sha256':e.sha(PINS),'canonicalJSON':PINS.decode(),'jsonPointers':['/compiler_revision','/go_version'],'prov:wasDerivedFrom':[parents[0]]})
 graph.append({'@id':activity,'@type':'prov:Activity','prov:used':parents+[selected],'prov:endedAtTime':{'@value':datetime.datetime.now(datetime.timezone.utc).isoformat(),'@type':'http://www.w3.org/2001/XMLSchema#dateTime'}})
 for name,raw in generated.items():graph.append({'@id':identity('output',name,e.sha(raw)),'@type':'prov:Entity','path':name,'role':'output','sha256':e.sha(raw),'prov:wasGeneratedBy':{'@id':activity},'prov:wasDerivedFrom':[selected,*parents[1:]]})
 return {'@context':{'prov':'http://www.w3.org/ns/prov#','gooo':'https://github.com/kimjooyoon/gooo-go-ports/provenance#','path':'gooo:path','role':'gooo:role','sha256':'gooo:sha256','original_JSON_sha256':'gooo:originalJSONSHA256','canonicalJSON':'gooo:canonicalJSON','jsonPointers':'gooo:jsonPointers'},'entity_identity_profile':profile,'@graph':graph,'scope':'Actual local generation dependencies; generation-scoped file occurrences. No producer authentication, model, live capture or effect authority.'}

def main():
 p=argparse.ArgumentParser();p.add_argument('--index',type=Path,required=True);p.add_argument('--gofmt',type=Path,required=True);p.add_argument('--out',type=Path);p.add_argument('--check',action='store_true');a=p.parse_args()
 assert a.index.is_absolute() and a.gofmt.is_absolute() and a.check!=(a.out is not None)
 generated,used=render(a.index,a.gofmt)
 if a.check:
  for name,raw in generated.items():assert e.read(ROOT/name,4<<20)==raw,name
 else:
  assert a.out.is_absolute() and not a.out.exists() and not a.out.resolve().is_relative_to(ROOT.resolve());a.out.mkdir()
  for name,raw in generated.items():
   dest=a.out/name;dest.parent.mkdir(parents=True,exist_ok=True);dest.write_bytes(raw)
  (a.out/'generation.provenance.jsonld').write_text(json.dumps(provenance(a.index,a.gofmt,generated,used),indent=2)+'\n')
 print(json.dumps({'status':'EXACT_GENERATED_BYTES_CHECKED' if a.check else 'GENERATED_FROM12_NATIVE_AND_OWN_REPLAY_CASES','files':sorted(generated)}))
if __name__=='__main__':main()
