"""Finite original Gooo, independent Go, generated API/CLI and own-replay checks."""
import argparse,copy,gzip,importlib.util,json,os,shutil,stat,sys,traceback
from pathlib import Path
HERE=Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('observation_e',HERE/'evidence.py');e=importlib.util.module_from_spec(spec);spec.loader.exec_module(e)
spec=importlib.util.spec_from_file_location('observation_g',HERE/'generate.py');g=importlib.util.module_from_spec(spec);spec.loader.exec_module(g)
spec=importlib.util.spec_from_file_location('observation_graph_check',e.ROOT/'examples/tool-definition-binding/verify.py');old=importlib.util.module_from_spec(spec);spec.loader.exec_module(old)
host=e.host;ROOT=e.ROOT;MODULE=ROOT/g.OUTPUT
surface_spec=importlib.util.spec_from_file_location('runtime_package_surface_adapter', ROOT/'tools/runtime_package_surface.py')
runtime_surface=importlib.util.module_from_spec(surface_spec);surface_spec.loader.exec_module(runtime_surface)
EXCLUDED={'.git','__pycache__','.cache','cache','tmp','out','output','bin','gopath','modcache','home','node_modules'}

def snapshot():
 paths=[ROOT/'upstream.json',e.MANIFEST,ROOT/e.SOURCE,*g.HELPERS,ROOT/'examples/tool-definition-binding/verify.py',ROOT/'examples/tool-definition-binding/provenance_test.py']
 paths.extend(runtime_surface.source_roster(ROOT))
 for directory in (HERE,MODULE):
  for parent,dirs,files in os.walk(directory,followlinks=False):
   p=Path(parent);assert all(not (p/n).is_symlink() for n in dirs);dirs[:]=sorted(n for n in dirs if n not in EXCLUDED)
   paths += [p/n for n in sorted(files) if not n.endswith('.pyc')]
 result={}
 for p in sorted(set(paths)):
  assert not p.is_symlink() and stat.S_ISREG(p.stat().st_mode)
  result[str(p.relative_to(ROOT))]=dict(host.ref(p),mode=stat.S_IMODE(p.stat().st_mode))
 assert g.ASSETS<=set(result);return result

def provenance(generated,index,gofmt,used):
 document=host.strict(e.read(generated/'generation.provenance.jsonld',2<<20));profile='gooo/observation-diagnostics-file-occurrence/v1';assert document['entity_identity_profile']==profile
 graph=document['@graph'];old.provenance_graph(graph);activities=[n for n in graph if n['@type']=='prov:Activity'];assert len(activities)==1
 nodes={n['path']:n for n in graph if n['@type']=='prov:Entity'};assert len(nodes)==len(graph)-1
 def check(node,role,path,digest):
  raw=json.dumps([profile,activities[0]['@id'],role,path,digest],ensure_ascii=True,separators=(',',':')).encode()
  assert node['@id']=='urn:gooo:observation-diagnostics:entity:'+e.sha(raw).removeprefix('sha256:')
  assert node['role']==role and node['path']==path and node['sha256']==digest
 paths=[ROOT/'upstream.json',e.MANIFEST,ROOT/e.SOURCE,HERE/'kernel.cases.json',*g.REFERENCES,*g.HELPERS,index,gofmt,gofmt.parent/'go',*used]
 receipts={e.shared.local_member(index.parent,b[k]) for b in host.strict(e.read(index))['batches'] for k in ('execution','replay')};parents=[]
 for p in paths:
  name=str(p.relative_to(ROOT)) if p.is_relative_to(ROOT) else str(p);node=nodes[name];check(node,'input',name,'sha256:'+host.ref(p)['sha256']);parents.append({'@id':node['@id']})
  if p in receipts:assert node['original_JSON_sha256']==e.sha(e.receipt(p)[0])
 name='upstream.json#consumed-observation-pins';pin=nodes[name];check(pin,'selected-pins',name,e.sha(g.PINS));assert pin['canonicalJSON'].encode()==g.PINS
 e.same(pin['jsonPointers'],['/compiler_revision','/go_version']);e.same(pin['prov:wasDerivedFrom'],[parents[0]]);selected={'@id':pin['@id']};e.same(activities[0]['prov:used'],parents+[selected])
 assert len(nodes)==len(paths)+1+len(g.ASSETS)
 for name in g.ASSETS:
  n=nodes[name];check(n,'output',name,'sha256:'+host.ref(generated/name)['sha256']);e.same(n['prov:wasGeneratedBy'],{'@id':activities[0]['@id']});e.same(n['prov:wasDerivedFrom'],[selected,*parents[1:]])

def main():
 p=argparse.ArgumentParser()
 for name in ('compiler','go','out'):p.add_argument('--'+name,type=Path,required=True)
 for name in ('go-cache','module-cache'):p.add_argument('--'+name,type=Path)
 a=p.parse_args();out=a.out.resolve();assert not out.is_relative_to(ROOT.resolve());out.mkdir(parents=True,exist_ok=False);host.R=out
 summary={'schema':'gooo/observation-diagnostics-verification/v1','status':'FIRST_FAILURE_NO_RETRY','compiler_revision':e.PIN,'go_version':e.GO_VERSION,'model_calls':0,'engine_builds':0,'external_adoption':False,'live_capture_executed':False,'native_acceptance':False,'execution_authorized':False,'synthetic_consumer_observations_only':True};first=None
 try:
  before=snapshot();host.save(out/'source-before.json',before)
  facts=host.strict(e.read(ROOT/'upstream.json'));assert facts['compiler_revision']==e.PIN and facts['go_version']==e.GO_VERSION
  compiler=a.compiler.resolve();go=a.go.resolve();gofmt=go.parent/'gofmt';assert compiler.is_file() and go.is_file() and gofmt.is_file()
  for name in ('cache','tmp','modcache','gopath','home','bin'):(out/name).mkdir(mode=0o700)
  cache=a.go_cache.resolve() if a.go_cache else out/'cache';modules=a.module_cache.resolve() if a.module_cache else out/'modcache';assert not cache.is_relative_to(ROOT) and not modules.is_relative_to(ROOT)
  env={'PATH':str(go.parent)+':/usr/bin:/bin','GOROOT':str(go.parent.parent),'HOME':str(out/'home'),'TMPDIR':str(out/'tmp'),'GOCACHE':str(cache),'GOMODCACHE':str(modules),'GOPATH':str(out/'gopath'),'GOTOOLCHAIN':'local','GOENV':'off','GOWORK':'off','GOPROXY':'off','GOSUMDB':'off','GOTELEMETRY':'off','CGO_ENABLED':'0','GOMAXPROCS':'2','GOFLAGS':'-mod=readonly -p=1','LANG':'C','LC_ALL':'C'}
  def run(label,argv,seconds=90,stdin=None,expected_exit=0,environment=None):return host.owned(label,argv,environment or env,seconds,ROOT,stdin,expected_exit)
  identity=host.strict(run('compiler-identity',[compiler,'version','--build','--json'],15))
  for key,value in {'compiler_source_sha':e.PIN,'vcs_revision':e.PIN,'source_status':'CLEAN_VCS','vcs_modified':'false','go_version':e.GO_VERSION,'native_go_required':e.GO_VERSION,'build_info_observed':True,'version':facts['compiler_version']}.items():e.same(identity[key],value)
  e.same(identity['module'],host.ACTIVE_COMPILER_PROFILE['module']);e.same(identity['decision_runtime'],host.ACTIVE_COMPILER_PROFILE['decision_runtime'])
  version=run('Go-identity',[go,'version'],15).decode().strip();assert version in ('go version '+e.GO_VERSION+' darwin/arm64','go version '+e.GO_VERSION+' linux/amd64');go_identity=dict(host.ref(go),version=version)
  summary.update(compiler=host.ref(compiler),compiler_identity=identity,go_identity=go_identity)
  run('graph-regression-controls',[sys.executable,'-I','-B',ROOT/'examples/tool-definition-binding/provenance_test.py'],30)
  expected=run('independent-Go-reference',[go,'-C',HERE/'reference','run','.','-input',HERE/'reference-inputs.json']);assert expected==e.read(HERE/'kernel.cases.json',2<<20)
  kernel=host.strict(expected);assert len(kernel['cases'])==51;projection=None
  def pairs(stem,corpus):
   nonlocal projection
   directory=out/stem;directory.mkdir();index={'schema':'gooo/observation-diagnostics-generation-input/v1','batches':[]};total=0
   for start in range(0,len(corpus['cases']),8):
    n=start//8;path=directory/('batch-%02d.cases.json'%n);rows=corpus['cases'][start:start+8];host.save(path,{'schema':'gooo/body-composition-cases/v1','cases':rows});label=stem+'-%02d'%n
    run(label+'-execute',[compiler,'package','execute','--json','--receipt-compression','gzip','--go',go,'--cases',path,e.MANIFEST],180);execution=out/(label+'-execute')/'stdout'
    run(label+'-replay',[compiler,'package','replay','--json','--receipt-compression','gzip','--go',go,'--receipt',execution,'--cases',path,e.MANIFEST],180);replay=out/(label+'-replay')/'stdout'
    count,composition=e.verify_pair(execution,replay,path,go_identity);total+=count
    if projection is None:projection=composition['source']
    else:assert projection==composition['source']
    index['batches'].append({'first_case':start,'cases':str(path.relative_to(out)),'execution':str(execution.relative_to(out)),'replay':str(replay.relative_to(out))})
   return total,index
  count,index=pairs('kernel',kernel);assert count==51;index_path=out/'generation-input.json';host.save(index_path,index);composition,used=e.verify_index(index_path,go_identity)
  generated=out/'generated';run('source-bound-generation',[sys.executable,'-I','-B',HERE/'generate.py','--index',index_path,'--gofmt',gofmt,'--out',generated],120)
  assert {str(p.relative_to(generated)) for p in generated.rglob('*') if p.is_file()}==g.ASSETS|{'generation.provenance.jsonld'}
  for name in g.ASSETS:assert e.read(generated/name,4<<20)==e.read(ROOT/name,4<<20),name
  provenance(generated,index_path,gofmt,used);run('exact-generated-check',[sys.executable,'-I','-B',HERE/'generate.py','--index',index_path,'--gofmt',gofmt,'--check'],120)
  run('module-integrity',[go,'-C',MODULE,'mod','verify']);run('go-vet',[go,'-C',MODULE,'vet','./...']);run('reference-vet',[go,'-C',HERE/'reference','vet','./...'])
  export=out/'consumer-export.json';run('API-CLI-race-export',[go,'-C',MODULE,'test','-race','./...','-count=1'],240,environment=dict(env,CGO_ENABLED='1',GOOO_OBSERVATION_EXPORT=str(export)))
  records=host.strict(e.read(export,2<<20));assert len(records)==4
  requests=[]
  for record in records:
   report=record['report'];raw=record['raw_input_json'].encode();assert host.strict(raw) and report['observation_projection']['input_sha256']==report['comparison']['projection']['input_sha256']==e.sha(raw)
   inputs=report['observation_projection']['inputs'];results=report['observations'];assert len(inputs)==len(results)==34
   for n,(value,expected) in enumerate(zip(inputs,results)):
    requests.append({'label':record['label']+'-%02d'%n,'input':value,'anchor_state':expected['state'],'anchor_action':expected['next_action']})
  assert len(requests)==136;request_path=out/'consumer-reference-inputs.json';host.save(request_path,{'schema':'gooo/observation-diagnostics-reference-inputs/v1','cases':requests})
  consumers=host.strict(run('consumer-independent-Go-reference',[go,'-C',HERE/'reference','run','.','-input',request_path]));assert len(consumers['cases'])==136
  position=0
  for record in records:
   for value,expected in zip(record['report']['observation_projection']['inputs'],record['report']['observations']):
    case=consumers['cases'][position];e.same(case['inputs'][e.ENTRY],value);e.same(case['expected'][e.ENTRY],expected);position+=1
  module='github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison'
  surface=runtime_surface.verify(ROOT,MODULE,go,go_identity,env,run,host,out,'runtime-dependencies')
  deps=surface['assessment']['observed_paths'];summary['runtime_package_surface']=surface
  binary=out/'bin/compare';run('CLI-build',[go,'-C',MODULE,'build','-trimpath','-buildvcs=false','-o',binary,'./cmd/compare'])
  info=host.strict(run('CLI-build-info',[go,'version','-m','-json',binary],20));assert info['GoVersion']==e.GO_VERSION and info['Main']['Path']==module and not info.get('Deps');settings={s['Key']:s['Value'] for s in info['Settings']};assert settings['CGO_ENABLED']=='0' and settings['-trimpath']=='true'
  runtime=dict(env,PATH='');runtime.pop('GOROOT',None)
  for n,record in enumerate(records):
   path=out/('consumer-%02d.json'%n);path.write_text(record['raw_input_json']);wire=run('CLI-consumer-%02d'%n,[binary,'--definitions','--diagnostics','--before-policy',record['before_policy_revision'],'--after-policy',record['after_policy_revision']],15,path,environment=runtime);report=host.strict(wire);e.same(report,record['report'])
   baseline=host.strict(run('CLI-original-%02d'%n,[binary,'--definitions','--before-policy',record['before_policy_revision'],'--after-policy',record['after_policy_revision']],15,path,environment=runtime));e.same(report['comparison'],baseline)
  assert run('repeat-identical-input',[binary,'--definitions','--diagnostics','--before-policy',record['before_policy_revision'],'--after-policy',record['after_policy_revision']],15,path,environment=runtime)==wire
  actual,_=pairs('consumer',consumers);assert actual==136
  batch=index['batches'][0];execution=e.shared.local_member(out,batch['execution']);replay=e.shared.local_member(out,batch['replay']);cases=e.shared.local_member(out,batch['cases']);_,original=e.receipt(replay);controls=[]
  for label in ('wrong-parent','wrong-source','wrong-input','wrong-field-ID','wrong-authority','input-only'):
   bad=copy.deepcopy(original);delivery=bad['result']['runtime']['traces'][0]['deliveries'][0]
   if label=='wrong-parent':bad['replayed_from_sha256']='sha256:'+'0'*64
   elif label=='wrong-source':bad['result']['program']['activity_origins'][0]['input_source']['source_sha256']='sha256:'+'0'*64
   elif label=='wrong-input':delivery['input']['field_path']='/tampered'
   elif label=='wrong-field-ID':delivery['actual_fields'][0]['id']='gooo://wrong-field'
   elif label=='wrong-authority':delivery['actual']['execution_authorized']=True
   else:bad['decision']='OBSERVED';bad['result']['runtime']['finite_total']=0
   path=out/(label+'.json.gz');path.write_bytes(gzip.compress(json.dumps(bad,separators=(',',':')).encode(),mtime=0))
   try:e.verify_pair(execution,path,cases,go_identity)
   except (AssertionError,ValueError):pass
   else:raise AssertionError('receipt mutation accepted: '+label)
   bad_index=copy.deepcopy(index);bad_index['batches'][0]['replay']=path.name;p=out/(label+'-index.json');host.save(p,bad_index);assert run(label,[sys.executable,'-I','-B',HERE/'generate.py','--index',p,'--gofmt',gofmt,'--check'],120,expected_exit=1)==b'';controls.append({'label':label,'receipt':host.ref(path),'reader_refused':True,'generator_refused':True})
  for label,field,value in [('coverage-gap','first_case',1),('metadata-path-escape','cases','../outside.cases.json')]:
   bad=copy.deepcopy(index);bad['batches'][0][field]=value;p=out/(label+'-index.json');host.save(p,bad);assert run(label,[sys.executable,'-I','-B',HERE/'generate.py','--index',p,'--gofmt',gofmt,'--check'],120,expected_exit=1)==b'';controls.append({'label':label,'generator_refused':True})
  host.save(out/'receipt-negative-controls.json',controls)
  after=snapshot();e.same(before,after);host.save(out/'source-after.json',after)
  summary.update(status='SOURCE_FIELD51_AND_SAVED_FRAME136_NATIVE_REPLAY_PASS',kernel_cases=51,consumer_fields=136,consumer_records=4,typed_input_fields=12,typed_output_fields=21,receipt_negative_controls=8,graph_regression_controls=12,generated_assets=5,generated_assets_exact=True,PROV_O_bindings_verified=True,source_unchanged=True,source_files=len(before),runtime_path_empty=True,runtime_goroot_absent=True,original_comparison_equal=True,repeated_identical_input_equal=True,reused_external_cache=bool(a.go_cache or a.module_cache),no_cold_cache_claim=True,runtime_nonstandard_packages=deps)
 except BaseException as error:first={'type':type(error).__name__,'message':str(error),'traceback':traceback.format_exc()};host.save(out/'first-failure.json',first)
 finally:
  pending=host.pending_cancellation()
  if pending is not None and first is None:first={'type':type(pending).__name__,'message':str(pending)}
  closed=bool(host.records) and all(r['waited'] and r['streams_EOF'] and r['PG_ESRCH'] for r in host.records)
  if first is None and not closed:first={'type':'RuntimeError','message':'process closure not established'}
  if closed:
   try:shutil.rmtree(out/'bin')
   except BaseException as error:
    summary['owned_cleanup_failure']={'type':type(error).__name__,'message':str(error)}
    if first is None:first=summary['owned_cleanup_failure']
  if first is not None:summary['status']='FIRST_FAILURE_NO_RETRY'
  summary.update(first_failure=first,records=host.records,all_direct_wait_EOF_PG_ESRCH=closed);host.save(out/'summary.json',summary)
 if first is not None:raise SystemExit(1)
 host.check_cancelled();print(json.dumps({'status':summary['status'],'summary':host.ref(out/'summary.json')}))
if __name__=='__main__':
 with host.cancellation_handlers():main()
