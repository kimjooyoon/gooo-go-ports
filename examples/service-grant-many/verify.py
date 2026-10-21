"""Portable fresh generation, independent reference and compiled transport verifier."""
from pathlib import Path
from contextlib import contextmanager
import argparse,datetime,hashlib,importlib.util,json,os,selectors,signal,stat,subprocess,time,traceback
HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[1]
CAP=32<<20;LIMIT=512<<20
# Active private Go1.27.3 compatibility profile; historical release JSON stays unchanged.
ACTIVE_COMPILER_PROFILE = {'name': 'current-private-037', 'compiler_revision': '205896a413e5dea92366309c35ae85e542d055f6', 'compiler_version': '0.6.37-dev', 'go_version': 'go1.27.3', 'module': {'path': 'github.com/kimjooyoon/meta-ontology-go', 'version': 'v0.6.37-dev.0.20261020180505-205896a413e5'}, 'decision_runtime': {'module_path': 'github.com/kimjooyoon/gooo-decision-runtime', 'version': 'v0.2.51-experimental', 'replaced': False}}
R=None;records=[];diagnostics=[]
_cancellation_state=None

def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()

def ref(x):
 h=hashlib.sha256();count=0
 with x.open('rb') as f:
  for b in iter(lambda:f.read(1<<20),b''):h.update(b);count+=len(b)
 return {'path':str(x),'bytes':count,'sha256':h.hexdigest()}

def save(p,x):p.write_text(json.dumps(x,ensure_ascii=False,indent=2)+'\n')

def strict(data):
 def pairs(items):
  d={}
  for k,v in items:
   if k in d:raise ValueError('DUPLICATE_KEY:'+k)
   d[k]=v
  return d
 return json.loads(data,object_pairs_hook=pairs,parse_constant=lambda s:(_ for _ in ()).throw(ValueError('NONFINITE:'+s)))

def exact(a,b):
 assert type(a) is type(b),(type(a).__name__,type(b).__name__)
 if isinstance(a,dict):
  assert set(a)==set(b),(set(a),set(b))
  for k in a:exact(a[k],b[k])
 elif isinstance(a,list):
  assert len(a)==len(b)
  for x,y in zip(a,b):exact(x,y)
 else:assert a==b,(a,b)

def absent(pg):
 try:os.killpg(pg,0)
 except ProcessLookupError:return True
 return False

def usage():
 total=0
 for name in ['cache','tmp','modcache','gopath']:
  for parent,dirs,files in os.walk(R/name,followlinks=False):
   for name in dirs:
    try:assert not (Path(parent)/name).is_symlink()
    except FileNotFoundError:pass
   for name in files:
    try:
     z=(Path(parent)/name).lstat();assert stat.S_ISREG(z.st_mode);total+=z.st_size
    except FileNotFoundError:pass
 return total

class VerifierCancelled(BaseException):
 """Cancellation must reach the same bounded cleanup as other first failures."""

def keep_first(first,error,where):
 if first is None:return error
 if error is first:return first
 diagnostics.append(where+': '+type(error).__name__+': '+str(error))
 if hasattr(first,'add_note'):first.add_note(where+': '+str(error))
 return first

@contextmanager
def blocked_interrupts():
 # Only the supported POSIX hosts are used. Restore the caller's exact mask.
 previous=signal.pthread_sigmask(signal.SIG_BLOCK,{signal.SIGTERM,signal.SIGINT});first=None
 try:yield previous
 except BaseException as error:first=error
 finally:
  try:signal.pthread_sigmask(signal.SIG_SETMASK,previous)
  except BaseException as error:first=keep_first(first,error,'signal mask restoration')
 if first is not None:raise first

def pending_cancellation():
 state=_cancellation_state
 if state is None or state['signal'] is None:return None
 if state['error'] is None:state['error']=VerifierCancelled('signal '+signal.Signals(state['signal']).name)
 if state['SIGTERM'] and state['SIGINT'] and not state['second_noted']:
  state['second_noted']=True
  other=signal.SIGINT if state['signal']==signal.SIGTERM else signal.SIGTERM
  diagnostics.append('additional cancellation: '+signal.Signals(other).name)
 return state['error']

def check_cancelled():
 error=pending_cancellation()
 if error is not None:raise error

@contextmanager
def cancellation_handlers():
 global _cancellation_state
 previous={};first=None;previous_state=_cancellation_state
 state={'signal':None,'SIGTERM':False,'SIGINT':False,'error':None,'second_noted':False}
 def interrupted(number,_frame):
  # No asynchronous raise: a signal must never jump over cleanup entry/body.
  if state['signal'] is None:state['signal']=number
  if number==signal.SIGTERM:state['SIGTERM']=True
  if number==signal.SIGINT:state['SIGINT']=True
 try:
  with blocked_interrupts():
   _cancellation_state=state
   for number in (signal.SIGTERM,signal.SIGINT):previous[number]=signal.signal(number,interrupted)
  check_cancelled()
  yield
  check_cancelled()
 except BaseException as error:first=error
 finally:
  error=pending_cancellation()
  if error is not None:first=keep_first(first,error,'deferred cancellation')
  for number,handler in previous.items():
   try:signal.signal(number,handler)
   except BaseException as error:first=keep_first(first,error,'signal handler restoration')
  error=pending_cancellation()
  if error is not None:first=keep_first(first,error,'deferred cancellation')
  _cancellation_state=previous_state
 if first is not None:raise first

def owned(label,argv,env,seconds,cwd,stdin=None,expected_exit=0):
 check_cancelled()
 d=R/label;d.mkdir();start=time.monotonic();at=now();child=None;sel=selectors.DefaultSelector();handles={};counts={'stdout':0,'stderr':0};first=None;eof=False;waited=False;pgabs=False;peak=0;last_sample=0
 try:
  for name in ('stdout','stderr'):handles[name]=(d/name).open('xb')
  with stdin.open('rb') if stdin is not None else open(os.devnull,'rb') as inp:
   # Keep the child object, pipe capture and PID journal owned before delivery.
   with blocked_interrupts() as previous_mask:
    check_cancelled()
    child=subprocess.Popen([str(a) for a in argv],cwd=cwd,env=env,stdin=inp,stdout=subprocess.PIPE,stderr=subprocess.PIPE,start_new_session=True,preexec_fn=lambda:signal.pthread_sigmask(signal.SIG_SETMASK,previous_mask))
    for name,pipe in [('stdout',child.stdout),('stderr',child.stderr)]:
     os.set_blocking(pipe.fileno(),False);sel.register(pipe,selectors.EVENT_READ,name)
    save(d/'started-process.json',{'PID':child.pid,'PGID':child.pid,'argv':[str(a) for a in argv],'cwd':str(cwd),'at_utc':at})
  check_cancelled()
  while sel.get_map() or child.poll() is None:
   check_cancelled()
   if time.monotonic()-start>seconds:raise TimeoutError('OWNED_TIME_LIMIT_'+str(seconds))
   if time.monotonic()-last_sample>.1:
    used=usage();peak=max(peak,used);last_sample=time.monotonic()
    if used>LIMIT:raise RuntimeError('OWNED_CACHE_512M_LIMIT')
    if sum((d/name).stat().st_size for name in handles)>CAP:raise RuntimeError('STREAM_32M_LIMIT')
   for key,_ in sel.select(.02):
    b=os.read(key.fileobj.fileno(),65536)
    if not b:sel.unregister(key.fileobj);key.fileobj.close();continue
    name=key.data;room=max(0,CAP-sum(counts.values()));counts[name]+=len(b);handles[name].write(b[:room]);handles[name].flush()
    if sum(counts.values())>CAP:raise RuntimeError('STREAM_32M_LIMIT')
  check_cancelled()
  child.wait(timeout=max(.001,seconds-(time.monotonic()-start)));waited=True
  if child.returncode!=expected_exit:raise RuntimeError('EXIT_'+str(child.returncode))
 except BaseException as error:first=error
 finally:
  try:
   # A second cancellation cannot cut off wait/EOF/group cleanup.
   with blocked_interrupts():
    if child is not None:
     if child.poll() is None:
      try:os.killpg(child.pid,signal.SIGTERM)
      except ProcessLookupError:pass
      try:child.wait(timeout=2)
      except subprocess.TimeoutExpired:
       try:os.killpg(child.pid,signal.SIGKILL)
       except ProcessLookupError:pass
       try:child.wait(timeout=2)
       except BaseException as error:first=keep_first(first,error,'direct child wait')
     if child.returncode is not None:child.wait();waited=True
     if not absent(child.pid):
      try:os.killpg(child.pid,signal.SIGTERM)
      except ProcessLookupError:pass
     until=time.monotonic()+2
     while sel.get_map() and time.monotonic()<until:
      for key,_ in sel.select(.02):
       b=os.read(key.fileobj.fileno(),65536)
       if not b:sel.unregister(key.fileobj);key.fileobj.close();continue
       name=key.data;room=max(0,CAP-sum(counts.values()));counts[name]+=len(b);handles[name].write(b[:room]);handles[name].flush()
     eof=not sel.get_map();pgabs=absent(child.pid)
     if not pgabs:
      try:os.killpg(child.pid,signal.SIGKILL)
      except ProcessLookupError:pass
      until=time.monotonic()+2
      while not absent(child.pid) and time.monotonic()<until:time.sleep(.02)
      pgabs=absent(child.pid)
     if not eof:first=keep_first(first,RuntimeError('EOF_NOT_OBSERVED'),'capture cleanup')
     if not pgabs:first=keep_first(first,RuntimeError('OWNED_PG_REMAINS'),'group cleanup')
     if sum(counts.values())>CAP:first=keep_first(first,RuntimeError('STREAM_32M_LIMIT'),'capture cleanup')
  except BaseException as error:first=keep_first(first,error,'cleanup or signal restoration')
  for key in list(sel.get_map().values()):
   try:sel.unregister(key.fileobj);key.fileobj.close()
   except BaseException as error:first=keep_first(first,error,'pipe closure')
  try:sel.close()
  except BaseException as error:first=keep_first(first,error,'selector closure')
  for h in handles.values():
   try:h.close()
   except BaseException as error:first=keep_first(first,error,'capture file closure')
  cancellation=pending_cancellation()
  if cancellation is not None:first=keep_first(first,cancellation,'deferred cancellation')
  error=None if first is None else type(first).__name__+': '+str(first)
  try:
   rec={'label':label,'argv':[str(a) for a in argv],'cwd':str(cwd),'PID':child.pid if child else None,'PGID':child.pid if child else None,'started_utc':at,'ended_utc':now(),'elapsed_seconds':time.monotonic()-start,'exit_code':child.returncode if child else None,'first_failure':error,'streams_EOF':eof,'waited':waited,'PG_ESRCH':pgabs,'stdout':ref(d/'stdout') if (d/'stdout').exists() else None,'stderr':ref(d/'stderr') if (d/'stderr').exists() else None,'stdin':ref(stdin) if stdin else None,'received_pipe_bytes':counts,'sampled_peak_combined_owned_cache_bytes':peak,'sampled_not_continuous_quota':True}
   records.append(rec)
   save(d/'process.json',rec)
  except BaseException as error:first=keep_first(first,error,'process record')
 if first is not None:raise first
 check_cancelled()
 return (d/'stdout').read_bytes()

def main():
 global R
 ap=argparse.ArgumentParser();ap.add_argument('--compiler',type=Path,required=True);ap.add_argument('--go',type=Path,required=True);ap.add_argument('--out',type=Path,required=True);ap.add_argument('--write-generated',action='store_true');args=ap.parse_args()
 R=args.out.resolve();R.mkdir(mode=0o700,parents=True,exist_ok=False)
 first=None;observations=None;metadata=None;fresh_oracle=None;api_result=None;friendly_result=None;binary_metadata=None;source_before=None
 try:
  compiler=args.compiler.resolve();go=args.go.resolve()
  for p in [compiler,go]:assert p.is_file() and not p.is_symlink()
  for n in ['cache','tmp','modcache','gopath','home','bin']:(R/n).mkdir(mode=0o700)
  env={'PATH':str(go.parent)+':/usr/bin:/bin','GOROOT':str(go.parent.parent),'HOME':str(R/'home'),'TMPDIR':str(R/'tmp'),'GOCACHE':str(R/'cache'),'GOMODCACHE':str(R/'modcache'),'GOPATH':str(R/'gopath'),'GOTOOLCHAIN':'local','GOENV':'off','GOWORK':'off','GOPROXY':'off','GOSUMDB':'off','GOTELEMETRY':'off','CGO_ENABLED':'0','GOMAXPROCS':'2','GOFLAGS':'-mod=readonly -p=1','LANG':'C','LC_ALL':'C'}
  source_paths=[ROOT/'service-grant-many.workspace.json',*[p for p in HERE.rglob('*') if p.is_file() and '__pycache__' not in p.parts],*[ROOT/n for n in strict((HERE/'generation.json').read_bytes())['public_sources']]]
  source_before={str(p.relative_to(ROOT)):ref(p) for p in source_paths}
  save(R/'source-before.json',source_before)
  goversion=owned('go-identity',[go,'version'],env,15,ROOT).decode().strip()
  assert goversion in ['go version go1.27.3 darwin/arm64','go version go1.27.3 linux/amd64']
  go_identity={'version':goversion,'sha256':'sha256:'+ref(go)['sha256']}
  identity=strict(owned('compiler-identity',[compiler,'version','--build','--json'],env,15,ROOT))
  historical_pin=strict((HERE/'compiler-pin.json').read_bytes())
  assert historical_pin['compiler_revision']=='de99b770b8b5adea608233c3c0664902336e3310'
  pin=ACTIVE_COMPILER_PROFILE
  for key,want in {'compiler_source_sha':pin['compiler_revision'],'vcs_revision':pin['compiler_revision'],'version':pin['compiler_version'],'source_status':'CLEAN_VCS','vcs_modified':'false','go_version':'go1.27.3','native_go_required':'go1.27.3'}.items():assert identity[key]==want,key
  exact(identity['decision_runtime'],pin['decision_runtime']);exact(identity['module'],pin['module']);assert identity['build_info_observed'] is True
  for stem in ['normal','remaining']:
   cases=HERE/(stem+'-cases.json')
   owned(stem+'-execute',[compiler,'package','execute','--json','--cases',cases,'--go',go,ROOT/'service-grant-many.workspace.json'],env,180,ROOT)
   owned(stem+'-saved-replay',[compiler,'package','replay','--json','--receipt',R/(stem+'-execute')/'stdout','--cases',cases,'--go',go,ROOT/'service-grant-many.workspace.json'],env,180,ROOT)
  spec=importlib.util.spec_from_file_location('source_generator',HERE/'generate.py');gen=importlib.util.module_from_spec(spec);spec.loader.exec_module(gen)
  metadata,observations=gen.render(R,check=not args.write_generated,gofmt=go.parent/'gofmt' if args.write_generated else None,go_identity=go_identity)
  fresh_oracle=strict(owned('independent-reference',[go,'-C',HERE/'reference','run','.'],env,90,HERE))
  expected=strict((HERE/'reference-expected.json').read_bytes());exact(fresh_oracle[:16],expected);assert len(fresh_oracle)==17
  boundary={'status':'INVALID_INPUT','reason':'REQUIRED_REQUEST_LIMIT_EXCEEDED','invalid_field':'required_types','invalid_index':-1,'first_counterexample':-1,'request_count':257,'checked_count':0,'all_preserved':False,'supported_subset':False,'subset_rule_id':'','counterexample_before_match':False,'counterexample_after_match':False}
  exact(fresh_oracle[16]['decision'],boundary)
  owned('module-vet',[go,'-C',HERE,'vet','./...'],env,90,HERE)
  race=dict(env,CGO_ENABLED='1');owned('module-race',[go,'-C',HERE,'test','-race','-count=1','./...'],race,90,HERE)
  binary=R/'bin/evaluate';owned('consumer-build',[go,'-C',HERE,'build','-trimpath','-buildvcs=false','-o',binary,'./cmd/evaluate'],env,90,HERE)
  buildraw=owned('consumer-BuildInfo',[go,'version','-m',binary],env,15,HERE).decode()
  assert buildraw.splitlines()[0].endswith(': go1.27.3')
  lines=[line.strip().split('\t') for line in buildraw.splitlines()[1:]]
  assert ['path','github.com/kimjooyoon/gooo-go-ports/examples/service-grant-many/cmd/evaluate'] in lines
  assert ['mod','github.com/kimjooyoon/gooo-go-ports/examples/service-grant-many','(devel)'] in lines
  assert not any(row[0]=='dep' for row in lines)
  settings={row[1].split('=',1)[0]:row[1].split('=',1)[1] for row in lines if len(row)==2 and row[0]=='build' and '=' in row[1]}
  goos,goarch=goversion.split()[-1].split('/')
  for k,v in {'-buildmode':'exe','-compiler':'gc','-trimpath':'true','CGO_ENABLED':'0','GOARCH':goarch,'GOOS':goos}.items():assert settings[k]==v,(k,settings)
  binary_metadata={'version':'go1.27.3','module':'github.com/kimjooyoon/gooo-go-ports/examples/service-grant-many','dependencies':0,'settings':settings,'binary':ref(binary)};save(R/'consumer-metadata.json',binary_metadata)
  facts=strict((HERE/'reference-cases.json').read_bytes());wire=R/'columns17.json';wire.write_text(json.dumps([c['facts'] for c in facts],ensure_ascii=False)+'\n')
  runtime={'PATH':'','HOME':str(R/'home'),'TMPDIR':str(R/'tmp'),'LANG':'C','LC_ALL':'C','GOMAXPROCS':'2'}
  api_result=strict(owned('columns17',[binary,'-columns'],runtime,15,R,wire))
  generation_sha='sha256:'+ref(HERE/'generation.json')['sha256']
  for row,case in zip(api_result,fresh_oracle):
   assert set(row)=={'facts','decision','bindings'};exact(row['facts'],case['facts']);exact(row['decision'],case['decision'])
   exact(row['bindings'],{'compiler_source_sha':metadata['compiler_source_sha'],'generation_sha256':generation_sha,'sources':metadata['public_sources'],'assets':metadata['assets']})
  assert len(api_result)==17
  representable=[(i,c) for i,c in enumerate(facts) if len(c['facts']['required_types'])==len(c['facts']['required_names'])]
  f=R/'friendly16.json';f.write_text(json.dumps([{'before':c['facts']['before'],'after':c['facts']['after'],'required_requests':[{'type':t,'name':n} for t,n in zip(c['facts']['required_types'],c['facts']['required_names'])]} for i,c in representable],ensure_ascii=False)+'\n')
  friendly_result=strict(owned('friendly-default',[binary],runtime,15,R,f));assert len(friendly_result)==len(representable)==16
  for row,(i,c) in zip(friendly_result,representable):exact(row,api_result[i])
  save(R/'friendly-case-mapping.json',{'indices':[i for i,c in representable],'unequal_columns_kept_in_columns17':[i for i,c in enumerate(facts) if len(c['facts']['required_types'])!=len(c['facts']['required_names'])]})
  for label,bad in [('wrong-type',{'before':True}),('extra-field',dict(facts[0]['facts'],extra=1))]:
   f=R/(label+'.json');f.write_text(json.dumps([facts[0]['facts'],bad]));raw=owned(label,[binary,'-columns'],runtime,5,R,f,expected_exit=1);assert raw==b''
  f=R/'lossy-surrogate.json';f.write_bytes(b'['+json.dumps(facts[0]['facts']).encode()+b',{"before":"*","after":"mcp://*","required_types":["mcp"],"required_names":["\\ud800"]}]');assert owned('lossy-surrogate',[binary,'-columns'],runtime,5,R,f,expected_exit=1)==b''
  after={str(p.relative_to(ROOT)):ref(p) for p in source_paths}
  if args.write_generated:
   assert (HERE/'compiler.json').read_bytes()==(R/'compiler-identity'/'stdout').read_bytes()
   for key in source_before:
    if key not in ['examples/service-grant-many/compiler.json','examples/service-grant-many/generation.json','examples/service-grant-many/projection_generated.go','examples/service-grant-many/bridge_generated.go']:exact(after[key],source_before[key])
  else:exact(after,source_before)
  save(R/'source-after.json',after)
  check_cancelled()
 except BaseException as exc:
  first={'type':type(exc).__name__,'message':str(exc),'traceback':traceback.format_exc()}
  try:save(R/'first-failure.json',first)
  except BaseException as diagnostic:diagnostics.append('first failure record: '+str(diagnostic))
 finally:
  if first is None:
   try:check_cancelled()
   except BaseException as error:first={'type':type(error).__name__,'message':str(error),'traceback':traceback.format_exc()}
  closed=bool(records) and all(x['waited'] and x['streams_EOF'] and x['PG_ESRCH'] for x in records)
  summary={'schema':'gooo/service-grant-many-verification/v1','status':'PUBLIC_SOURCE_REFERENCE17_AND_NATIVE_TRANSPORT_PASS' if first is None else 'FIRST_FAILURE_NO_RETRY','first_failure':first,'compiler_revision':ACTIVE_COMPILER_PROFILE['compiler_revision'],'fresh_genuine_commands':4 if observations else None,'genuine_cases':16,'genuine_split':[1,15],'typed_decision_fields':12,'fresh_reference_cases':len(fresh_oracle) if fresh_oracle else 0,'actual_compiled_columns_cases':len(api_result) if api_result else 0,'actual_compiled_friendly_cases':len(friendly_result) if friendly_result else 0,'binary_metadata':binary_metadata,'sampled_combined_resource_limit_bytes':LIMIT,'no_cold_cache_claim':True,'observations':observations,'records':records,'all_direct_wait_EOF_PG_ESRCH':closed,'remaining_owned_groups':0 if closed else 'NOT_ESTABLISHED','model_calls':0,'engine_builds':0,'external_adoption':False,'local_lineage_complete_claim':False,'native257_semantic_decision':'Gooo-owned INVALID_INPUT, not a transport count refusal','scope':'Finite current-source function/transport comparison; no authority or grant application.'}
  try:save(R/'summary.json',summary)
  except BaseException as diagnostic:
   if first is None:first={'type':type(diagnostic).__name__,'message':str(diagnostic),'traceback':traceback.format_exc()}
   else:diagnostics.append('summary record: '+str(diagnostic))
 if first is not None:raise SystemExit(1)
 check_cancelled()
 print(json.dumps({'status':summary['status'],'summary':ref(R/'summary.json')}))
if __name__=='__main__':
 with cancellation_handlers():main()
