#!/usr/bin/env python3
"""Check strict four-input helper output through actual Gooo execute/saved replay.

Legacy full-run timing or an explicit experimental shard reference supplies the
same independent Go case/raw-receipt anchors. Import has no execution side effects.
"""
import argparse,hashlib,importlib.util,json,os,pathlib,re,signal,subprocess,sys,time
REV='205896a413e5dea92366309c35ae85e542d055f6'
ENTRY='app/diagnostic-header-addition:AssessAddition'
ROOT='app/diagnostic-header-addition'
RID='gooo://diagnostic-header-addition/request/v1'
OID='gooo://diagnostic-header-addition/result/v1'
SOURCES=[('go/golang.org/x/net/http/httpguts','ports/net/httpguts/headers.gooo'),('app/header-field-facts','examples/header-field-facts/main.gooo'),('go/strconv','ports/strconv/decimal.gooo'),('app/payload-admission','examples/payload-admission/main.gooo'),(ROOT,'examples/diagnostic-header-addition/main.gooo')]
LABELS=['request-id-eligible','value-crlf','header-snapshot-unbound','both-max-refused']
def need(ok, message):
    if not ok: raise RuntimeError(message)
def sha(raw): return hashlib.sha256(raw).hexdigest()
def load(raw):
    def pairs(items):
        d = {}
        for k,v in items: need(k not in d,'duplicate JSON key'); d[k] = v
        return d
    return json.loads(raw,object_pairs_hook=pairs,parse_constant=lambda _: (_ for _ in ()).throw(ValueError('nonfinite JSON')))
def equal(a,b):
    if type(a) is not type(b): return False
    if isinstance(a,dict): return a.keys() == b.keys() and all(equal(a[k],b[k]) for k in a)
    if isinstance(a,list): return len(a) == len(b) and all(equal(x,y) for x,y in zip(a,b))
    return a == b
def canonical(path):
    p = pathlib.Path(path); need(p.is_absolute() and p.resolve() == p and not p.is_symlink(),'canonical absolute path required'); return p
def save(path,doc):
    with path.open('x') as f: json.dump(doc,f,ensure_ascii=False,indent=2); f.write('\n')
def processes():
    raw = subprocess.check_output(['/bin/ps','-Aww','-o','pid=,ppid=,pgid=,lstart=,command='],text=True,timeout=2)
    rows = {}
    for line in raw.splitlines():
        v = line.strip().split(None,8)
        if len(v) == 9: rows[int(v[0])] = {'pid':int(v[0]),'ppid':int(v[1]),'pgid':int(v[2]),'birth':' '.join(v[3:8]),'command':v[8]}
    return rows
ACTIVITY = re.compile(r'activity\s+(\w+)\(([^)]*)\)\s*->\s*(\w+)\s+id\s+"([^"]+)"\s+computes\s*(?:`([^`]*)`|("(?:\\.|[^"\\])*"))',re.S)
RECORD = re.compile(r'entity\s+(\w+)\s+id\s+"([^"]+)"\s+fields\s*\{(.*?)\}',re.S)
FIELD = re.compile(r'field\s+(\w+)\s+id\s+"([^"]+)"\s+type\s+(string|integer|boolean)\s+required\s+one')
def activities(text):
    out=[]
    for m in ACTIVITY.finditer(text):out.append({'name':m[1],'inputs':[x.strip() for x in m[2].split(',')],'output':m[3],'id':m[4],'body':m[5] if m[5] is not None else load(m[6])})
    need(len(out)==len(re.findall(r'(?m)^activity\s+',text)),'unparsed source activity');return out
def contracts():
    definitions={};records={};ordinal=0
    for source in PLAN['sources']:
        text=RAW[source['path']].decode('ascii');pkg=source['package'];imports=dict(re.findall(r'^import\s+(\w+)\s+"([^"]+)"',text,re.M))
        for d in activities(text):
            d.update(package=pkg,source=source['target'],lowered='GoooPackage'+str(ordinal)+'Activity'+d['name'],imports=imports);ordinal+=1;definitions[pkg+':'+d['name']]=d
        for name,ident,body in RECORD.findall(text):
            fs=[{'name':n,'id':i,'type_id':'urn:gooo:type:'+t,'go_name':'GoooField'+sha(i.encode())} for n,i,t in FIELD.findall(body)]
            need(ident not in records and 0<len(fs)<=16,'record roster/limit');records[ident]={'name':name,'id':ident,'go_name':'GoooRecord'+sha(ident.encode()),'fields':fs}
    need(len(definitions)==6 and len(records)==5,'exact six activity/five record source closure')
    for key,d in definitions.items():
        body=d['body'];sites=[]
        for m in re.finditer(r'\b([a-z]\w*)\.([A-Z]\w*)\s*(?=\()',body):
            need(m[1] in d['imports'],'unknown source callee alias');callee=d['imports'][m[1]]+':'+m[2];need(callee in definitions,'unknown pure callee')
            sites.append({'caller':key,'callee':callee,'surface':'computes','start_byte':len(body[:m.start()].encode()),'end_byte':len(body[:m.end()].encode())})
        d['sites']=sites;d['lowered_body']=body
        for site in sites:
            target=definitions[site['callee']];alias=next(a for a,p in d['imports'].items() if p==target['package']);d['lowered_body']=d['lowered_body'].replace(alias+'.'+target['name'],target['lowered'])
    need(len(definitions[ENTRY]['sites'])==2,'root two actual composition calls');return definitions,records
def record_fields(ident,value):return [{'id':f['id'],'name':f['name'],'value':value[f['name']]} for f in RECORDS[ident]['fields']]
def receipt(raw,row,trusted,manifest,images,go_version,prior=None):
    rows=[row]
    r=load(raw);need(r['schema']=='gooo/workspace-body-execution-receipt/v1' and r['decision']=='OBSERVED' and not r.get('error'),'actual input-only OBSERVED envelope')
    need(r['manifest']==str(MANIFEST) and r['manifest_digest']=='sha256:'+sha(manifest) and r['inputs_digest']=='sha256:'+sha(trusted) and 'cases_digest' not in r,'exact current manifest and actual helper output bytes')
    result=r['result'];p=result['program'];c=result['composition'];rt=result['runtime'];root=DEFS[ENTRY]
    ref={'package_path':ROOT,'activity':'AssessAddition','lowered_name':root['lowered']}
    need(result['schema']=='gooo/workspace-body-execution/v1' and p['schema']=='gooo/workspace-body-program/v1' and equal(p['entry'],ref) and equal(p['activities'],[ref]),'single exact original/lowered root')
    image=p['workspace_image'];need(image['schema']=='gooo/package-runtime-image/v1' and equal(image['init_order'],[s['package'] for s in PLAN['sources']]),'exact five-package order')
    seen=[{'package':pkg['path'],'filename':s['filename'],'source_digest':s['source_digest']} for pkg in image['packages'] for s in pkg['sources']]
    need(equal(seen,images) and len(image['packages'])==5,'actual Go JSON-string source-image digests')
    need(equal(image['entry'],{'package_path':ROOT,'source':root['source'],'activity':'AssessAddition','inputs':['Text','Text','Text','RequestFacts'],'output':'HeaderAdditionDecision'}),'image typed entry')
    lowered=p['lowered_gooo_source'];actual=activities(lowered);need(len(actual)==6,'exact lowered callee source roster')
    for d in DEFS.values():
        a=[a for a in actual if a['name']==d['lowered']];need(len(a)==1 and all(equal(a[0][k],d[k]) for k in ['id','inputs','output']) and a[0]['body']==d['lowered_body'],'source-resolved lowered body/signature')
    pure=p['pure_calls'];need(pure['schema']=='gooo/workspace-pure-calls/v1','pure source schema')
    calls={x['activity']['package_path']+':'+x['activity']['activity']:x for x in pure['activities']};need(set(calls)==set(DEFS) and len(calls)==len(pure['activities']),'exact pure participant closure')
    for key,d in DEFS.items():
        x=calls[key];need(equal(x['activity'],{'package_path':d['package'],'activity':d['name'],'lowered_name':d['lowered']}) and x['source']==d['source'] and x['source_body_sha256']=='sha256:'+sha(d['body'].encode()),'source pure body identities')
    queue=[ENTRY];sites=[]
    for key in queue:
        for site in DEFS[key]['sites']:
            sites.append(site)
            if site['callee'] not in queue:queue.append(site['callee'])
    need(equal(pure['sites'],sites) and len(sites)==5,'exact source call sites and five transitive edges')
    source_sha='sha256:'+sha(lowered.encode());need(c['schema']=='gooo/body-composition/v1' and c['stage']=='COMPLETE' and c['gooo_source']==lowered and c['original_source_sha256']==c['selected_source_sha256']==result['execution_source_sha256']==source_sha,'lowered composition source binding')
    plan=c['plan'];inputs=[{'port':'input'+str(i),'type':'Text' if i<3 else 'RequestFacts','entity_id':'gooo://std/text' if i<3 else RID,'from':-1} for i in range(4)]
    need(plan['schema']=='gooo/body-composition-plan/v1' and plan['entry_activity']==root['lowered'] and equal(plan['edges'],[]) and len(plan['activities'])==1,'typed no-effect single-node plan')
    a=plan['activities'][0];need(equal(a['inputs'],inputs) and a['name']==root['lowered'] and a['id']==root['id'] and a['input_type']=='Text' and a['output_type']=='HeaderAdditionDecision' and a['input_entity_id']=='gooo://std/text' and a['output_entity_id']==OID and a['input_from']==-1 and a['assembling'] is False and a.get('prepared',False) is False and a['go_function']=='GoooComposedActivity0' and not plan.get('preparations') and not p.get('entity_aliases'),'typed caller/return ABI')
    records={x['id']:x for x in plan['record_types']};need(len(records)==len(plan['record_types']) and equal(records,RECORDS),'exact five typed record identities/ordered fields')
    need(len(c['steps'])==1,'one actual source generation');generation=c['steps'][0]['generation'];report=generation['report'];closure=report['call_closure']
    need(report['schema']=='gooo/body-codegen-report/v3' and report['decision']=='PASS' and report['activity']==root['lowered'] and report['activity_id']==root['id'] and report['compiler_source_sha']==REV and report['typecheck_passed'] is True and report['deterministic_replay'] is True,'native source generation producer')
    need(c['steps'][0]['input_source_sha256']==source_sha and report['source_digest']==source_sha and report['program_digest']=='sha256:'+sha(root['lowered_body'].encode()) and report['generated_digest']=='sha256:'+sha(generation['source'].encode()) and report['replay_digest']==report['generated_digest'] and equal(report['repository_writes'],0),'generation actual source/program/output digest binding')
    generated_records={x['id']:x for x in report['record_types']};parameters=[{'name':'input'+str(i),'type':'string' if i<3 else 'RequestFacts'} for i in range(4)]
    need(len(generated_records)==len(report['record_types']) and equal(generated_records,RECORDS) and equal(report['input_parameters'],parameters) and report['input_type']=='(string,string,string,RequestFacts)' and report['output_type']=='HeaderAdditionDecision','generated full record and four-parameter typed ABI')
    actual_callees={x['name']:x for x in closure['activities']};wanted={d['lowered']:{'name':d['lowered'],'activity_id':d['id'],'program_sha256':'sha256:'+sha(d['lowered_body'].encode())} for key,d in DEFS.items() if key!=ENTRY}
    need(closure['schema']=='gooo/pure-activity-call-closure/v1' and len(actual_callees)==len(closure['activities']) and equal(actual_callees,wanted) and equal(closure['max_calls_per_invocation'],5),'exact typed transitive callee program closure')
    edge_ordinals={ENTRY:[0,1],'app/header-field-facts:Report':[8,9],'app/payload-admission:Report':[0]};edges=[]
    for key,ordinals in edge_ordinals.items():
        for site,ordinal in zip(DEFS[key]['sites'],ordinals):edges.append({'caller_id':DEFS[key]['id'],'callee_id':DEFS[site['callee']]['id'],'ordinal':ordinal})
    order=lambda e:(e['caller_id'],e['ordinal'],e['callee_id'])
    need(equal(sorted(closure['edges'],key=order),sorted(edges,key=order)),'exact typed call-edge identities/ordinals')
    need(rt['schema']=='gooo/body-composition-runtime/v1' and rt['stage']=='COMPLETE' and not rt.get('failure') and equal([rt['finite_total'],rt['finite_passed'],rt['model_calls']],[0,0,0]) and rt['projection_replayed'] is True and rt['runtime_replayed'] is True,'input-only zero-finite zero-model native runtime')
    need(rt['producer_source_sha']==REV and rt['go_version']==go_version and rt['go_tool_selection']=='explicit' and rt['go_tool_sha256']=='sha256:'+TOOLS[str(GO)]['sha256'],'exact producer and explicit Go binary')
    need(rt['original_source_sha256']==source_sha and rt['selected_source_sha256']==source_sha and rt['typed_plan_sha256']==plan['typed_plan_sha256'],'runtime source/typed-plan crossbinding')
    need(c['generated_sha256']=='sha256:'+sha(c['source'].encode())==rt['generated_sha256'] and c['driver_sha256']=='sha256:'+sha(c['driver_source'].encode())==rt['driver_sha256'],'actual generated/driver source crossbinding')
    need(len(rt['traces'])==len(rows),'ordered finite trace count')
    output_fields=RECORDS[OID]['fields'];types={'string':str,'boolean':bool,'integer':int}
    for i,(trace,row) in enumerate(zip(rt['traces'],rows)):
        expected=row['expected'][ENTRY];need(set(expected)=={f['name'] for f in output_fields} and all(type(expected[f['name']]) is types[f['type_id'].split(':')[-1]] for f in output_fields),'all16 reference scalar types')
        need(equal(trace['case_index'],i) and len(trace['deliveries'])==1,'one ordered root delivery');d=trace['deliveries'][0]
        supplied=[{'port':'input'+str(j),'entity_id':'gooo://std/text' if j<3 else RID,'value':row['inputs'][ENTRY+'.input'+str(j)]} for j in range(4)];supplied[3]['fields']=record_fields(RID,supplied[3]['value'])
        need(d['activity_id']==root['id'] and equal(d['inputs'],supplied),'all typed scalar/12-field original record inputs')
        need('expected' not in d and 'passed' not in d and equal(d['actual'],expected) and equal(d['actual_fields'],record_fields(OID,expected)),'all16 observed typed values match independent reference; no producer expectations')
    if prior is not None:need(r['replayed_from_sha256']=='sha256:'+sha(prior) and result['replay']['schema']=='gooo/workspace-body-replay/v1' and equal(result['replay']['model_calls'],0),'exact saved raw outer parent')
    return {'receipt_sha256':sha(raw),'observations':len(rows),'typed_result_fields':len(rows)*16,'typed_input_record_fields':len(rows)*12}


def parse_args(argv=None):
    parser=argparse.ArgumentParser(description=__doc__)
    for name in ['compiler','input-command','go','root','out']:parser.add_argument('--'+name,required=True)
    mode=parser.add_mutually_exclusive_group(required=True)
    mode.add_argument('--verification-out')
    mode.add_argument('--shard-native-index')
    parser.add_argument('--timing')
    args=parser.parse_args(argv)
    if args.shard_native_index is not None:
        if not args.shard_native_index or args.timing is not None:parser.error('--shard-native-index requires a nonempty path and forbids --timing')
    elif not args.verification_out or not args.timing:parser.error('--verification-out and --timing require nonempty paths together')
    return args

def _main(scan_state):
    global PLAN,RAW,DEFS,RECORDS,MANIFEST,GO,TOOLS
    a=parse_args();root,compiler,helper,go,out=[canonical(x) for x in [a.root,a.compiler,a.input_command,a.go,a.out]]
    shard_index=canonical(a.shard_native_index) if a.shard_native_index is not None else None
    verification_out=canonical(a.verification_out) if a.verification_out is not None else None
    timing=canonical(a.timing) if a.timing is not None else None
    need(root.is_dir() and not out.exists(),'root must exist and output must be fresh')
    out.mkdir();(out/'tmp').mkdir();started=time.monotonic();observations=[];refusals=[];native_count=0
    def interrupted(signum,_frame):
        failure=RuntimeError('received signal '+str(signum));observer=scan_state.get('observer')
        if observer is not None and observer.cancellation is None:observer.cancellation=failure
        raise failure
    for number in [signal.SIGTERM,signal.SIGINT]:signal.signal(number,interrupted)
    checker=root/'tools/check-header-addition-host.py'
    module_spec=importlib.util.spec_from_file_location('gooo_header_addition_host',checker)
    host=importlib.util.module_from_spec(module_spec);sys.modules[module_spec.name]=host;module_spec.loader.exec_module(host)
    pin_scans=host.PinScanObservations();pin_scans.output=out;scan_state['observer']=pin_scans
    try:
        verification=pin_scans.observe('initial-verification','parent',lambda:host.read_shard_verification(root,shard_index) if shard_index is not None else host.read_verification(root,verification_out,timing))
    except BaseException as failure:
        summary={'schema':'gooo/header-addition-input-route/v1','status':'FIRST_FAILURE_PRESERVED','failure':str(failure),'positive_requests':None,'prelaunch_refusals':0,'actual_Gooo_commands':0,'typed_fields_checked':0,'observations':[],'elapsed_s':time.monotonic()-started,'scope':('Experimental shard reference, producer unauthenticated; no native acceptance or authenticated GitHub-run claim.' if shard_index is not None else 'Legacy full-verifier evidence was refused before native input commands.')}
        save(out/'summary.json',summary);print(json.dumps(summary));return 1
    pins=dict(verification['pins'])
    tracked=[compiler,helper,go,pathlib.Path(__file__).resolve(),checker,*[root/name for _,name in SOURCES],root/'diagnostic-header-addition.workspace.json',root/'examples/diagnostic-header-addition/corpus.json',root/'upstream.json',root/'go.mod',root/'go.sum',root/'cmd/header-addition-input/main.go',root/'internal/headeradditioninput/preflight.go']
    for path in tracked:
        raw=canonical(path).read_bytes();value={'bytes':len(raw),'sha256':sha(raw)}
        need(str(path) not in pins or equal(pins[str(path)],value),'evidence/tool pin conflict');pins[str(path)]=value
    def unchanged():
        for p,ref in pins.items():
            raw=canonical(p).read_bytes();need(len(raw)==ref['bytes'] and sha(raw)==ref['sha256'],'frozen evidence/source/tool drift: '+p)
    def call(label,args,seconds,data=None):
        nonlocal native_count
        pin_scans.observe(label,'pre',unchanged,pins); d = out/label; d.mkdir(); (d/'stdin').write_bytes(data or b''); begin = time.monotonic(); why = None; known = {}; child = None
        def live():
            rows = processes(); owned = {pid for pid,r in rows.items() if pid in known and r['birth'] == known[pid]['birth']}
            if child is not None and child.poll() is None and child.pid in rows: owned.add(child.pid)
            owned.update(pid for pid,r in rows.items() if re.search(re.escape(str(out/'tmp'))+r'(?=$|[/\s])',r['command']))
            while True:
                more = {pid for pid,r in rows.items() if r['ppid'] in owned}; before = len(owned); owned.update(more)
                if len(owned) == before: break
            for pid in owned: known[pid] = rows[pid]
            return [rows[pid] for pid in owned]
        with (d/'stdin').open('rb') as si,(d/'stdout').open('xb') as so,(d/'stderr').open('xb') as se:
            try:
                need(begin-started < 300,'whole300s budget')
                child = subprocess.Popen(args,cwd=root,env=env,stdin=si,stdout=so,stderr=se,start_new_session=True)
                if args[:2] == [str(compiler),'package'] and args[2] in ['execute','replay']: native_count += 1
                save(d/'started.json',{'argv':list(map(str,args)),'pid':child.pid,'pgid':child.pid,'timeout_s':seconds})
                while child.poll() is None:
                    live(); need(time.monotonic() < min(started+300,begin+seconds),'command/whole timeout')
                    need(so.tell() <= 8<<20 and se.tell() <= 2<<20,'command output cap'); time.sleep(0.05)
            except BaseException as error: why = str(error)
            finally:
                observation_complete=True
                try:remaining=live()
                except BaseException as error:
                    remaining=[];why=why or 'process observation failed: '+str(error);observation_complete=False
                if remaining:
                    why = why or 'observed owned processes remained'
                    for group in {r['pgid'] for r in remaining}:
                        need(group > 1 and group != os.getpgrp(),'refuse unrelated process group')
                        try: os.killpg(group,signal.SIGKILL)
                        except ProcessLookupError: pass
                # This live direct child is our own new session even if ps failed.
                if child is not None and child.poll() is None:
                    need(child.pid>1 and child.pid!=os.getpgrp(),'refuse unrelated direct process group')
                    try:os.killpg(child.pid,signal.SIGKILL)
                    except ProcessLookupError:pass
                if child is not None:
                    try: child.wait(timeout=3)
                    except subprocess.TimeoutExpired: why = why or 'direct child not reaped'
                for _ in range(5):
                    try:remaining=live()
                    except BaseException as error:
                        why=why or 'process closure observation failed: '+str(error);observation_complete=False;break
                    if not remaining: break
                    time.sleep(0.05)
                save(d/'process.json',{'exit_code':None if child is None else child.returncode,'failure':why,'wall_seconds':time.monotonic()-begin,'observed_owned_processes_remaining':[{k:r[k] for k in ['pid','pgid','birth']} for r in remaining],'process_observation_complete':observation_complete,'scope':'Polling ancestry and unique tmp command paths; not a kernel fork trace.'})
        need(child is not None and why is None and not remaining and observation_complete,'command failure: '+label+' '+str(why))
        need((d/'stdout').stat().st_size <= 8<<20 and (d/'stderr').stat().st_size <= 2<<20,'final output cap')
        pin_scans.observe(label,'post',unchanged,pins); return child.returncode,(d/'stdout').read_bytes()
    env=os.environ.copy();env.update(GOTOOLCHAIN='local',GOWORK='off',GOENV='off',GOPROXY='off',GOSUMDB='off',GOOO_LAYA_URL='',GOOO_LAYA_API_KEY='',TMPDIR=str(out/'tmp'))
    try:
        pin_scans.observe('initial','initial',unchanged,pins);need(equal(pins[str(compiler)],pins[str(root/'.gooo')]) and equal(pins[str(go)],verification['go_binary']),'input route must use exact current evidence compiler/Go binaries');save(out/'source-tool-pins.json',pins)
        lock=load((root/'upstream.json').read_bytes());need(lock['compiler_revision']==REV and lock['go_version']=='go1.27.3','current upstream compiler/Go pins')
        MANIFEST=root/'diagnostic-header-addition.workspace.json';GO=go;TOOLS=pins
        PLAN={'sources':[{'package':package,'path':str(root/name),'target':name} for package,name in SOURCES]}
        RAW={str(root/name):(root/name).read_bytes() for _,name in SOURCES};DEFS,RECORDS=contracts()
        manifest=MANIFEST.read_bytes();wanted_manifest={'schema':'gooo/package-workspace-manifest/v1','entry':{'package_path':ROOT,'activity':'AssessAddition'},'packages':[{'path':package,'sources':[name]} for package,name in SOURCES]}
        need(equal(load(manifest),wanted_manifest),'exact ordered five-package public manifest')
        images=[]
        for package,name in SOURCES:
            text=RAW[str(root/name)].decode('ascii');encoded=json.dumps(text,ensure_ascii=False).replace('<','\\u003c').replace('>','\\u003e').replace('&','\\u0026').encode()
            images.append({'package':package,'filename':name,'source_digest':'sha256:'+sha(encoded)})
        spec=load((root/'examples/diagnostic-header-addition/corpus.json').read_bytes());need(len(spec['cases'])==len(verification['rows'])==50,'current exact50 input/oracle roster')
        labels=[x['label'] for x in spec['cases']];selection=[]
        for label in LABELS:
            need(labels.count(label)==1,'unique selected witness label');i=labels.index(label);request=spec['cases'][i]['input'];row=verification['rows'][i]
            need(equal(row['inputs'],{ENTRY+'.input0':request['name'],ENTRY+'.input1':request['value'],ENTRY+'.input2':request['declared_length'],ENTRY+'.input3':request['facts']}),'selected current Go case inputs')
            selection.append((label,request,row))
        rc,raw=call('go-version',[str(go),'version'],20);go_version=raw.decode().strip();need(rc==0 and re.fullmatch(r'go version go1\.27\.3 [a-z0-9_]+/[a-z0-9_]+',go_version),'actual explicit Go identity');need(go_version==verification['go_version'],'same observed Go version as current evidence')
        rc,raw=call('compiler-version',[str(compiler),'version','--build','--json'],20);identity=load(raw)
        need(rc==0 and all(equal(identity[k],v) for k,v in {'schema':'gooo/build-identity/v1','version':'0.6.37-dev','compiler_source_sha':REV,'vcs_revision':REV,'source_status':'CLEAN_VCS','vcs_modified':'false','go_version':'go1.27.3','native_go_required':'go1.27.3','build_info_observed':True}.items()),'actual pinned compiler identity')
        for index,(label,request,row) in enumerate(selection):
            prefix='positive-'+str(index);request_raw=json.dumps(request,ensure_ascii=False,separators=(',',':')).encode()
            rc,trusted=call(prefix+'-preflight',[str(helper)],10,request_raw)
            need(rc==0 and equal(load(trusted),{'schema':'gooo/body-composition-inputs/v1','inputs':[row['inputs']]}),'actual helper exact four-input wire document')
            trusted_path=out/(prefix+'-preflight')/'stdout'
            for operation in ['execute','replay']:
                prior_path=out/(prefix+'-execute')/'stdout';prior=prior_path.read_bytes() if operation=='replay' else None
                args=[str(compiler),'package',operation,'--json','--go',str(go),'--inputs',str(trusted_path)]
                if prior is not None:args+=['--receipt',str(prior_path)]
                rc,raw=call(prefix+'-'+operation,args+[str(MANIFEST)],90);need(rc==0,'actual native command refused; first response retained')
                checked=receipt(raw,row,trusted,manifest,images,go_version,prior)
                need(load(raw)['result']['composition']['source']==verification['source'],'input-only and current differential generated assessor bytes')
                observations.append({'label':label,'operation':operation,'typed_fields':16,**checked});save(out/(prefix+'-'+operation)/'checked.json',observations[-1])
        base=json.dumps(selection[0][1],separators=(',',':')).encode()
        def changed(key,value,fact=False):
            x=load(base)
            if fact:x['facts'][key]=value
            else:x[key]=value
            return json.dumps(x,separators=(',',':')).encode()
        missing=load(base);del missing['facts']['current_access']
        negatives=[('missing',b'{}'),('top-extra',changed('extra',True)),('top-decoded-duplicate',base.replace(b'"name":',b'"name":"first","na\\u006de":',1)),('facts-decoded-duplicate',base.replace(b'"current_access":true',b'"current_access":false,"current_\\u0061ccess":true',1)),('facts-null',changed('facts',None)),('missing-fact',json.dumps(missing).encode()),('name-null',changed('name',None)),('invalid-raw-utf8',base.replace(b'"req-123"',b'"\xff"',1)),('lone-surrogate',base.replace(b'"declared_length":"2"',b'"declared_length":"\\ud800"',1)),('allowed-name-too-large',changed('allowed_header_name','x'*1025,True)),('fraction-int',base.replace(b'"candidate_name_occurrences":0',b'"candidate_name_occurrences":0.5',1)),('exponent-int',base.replace(b'"observed_body_bytes":2',b'"observed_body_bytes":2e0',1)),('overflow-int',changed('observed_request_revision',9223372036854775808,True)),('boolean-number',changed('current_access',1,True)),('trailing',base+b'{}'),('raw-size',b' '*32769)]
        need(len(negatives)==16 and len({raw for _,raw in negatives})==16 and all(raw!=base for _,raw in negatives),'sixteen distinct actual malformed requests')
        for i,(label,request) in enumerate(negatives):
            before=native_count;rc,trusted=call('negative-'+str(i),[str(helper)],10,request)
            need(rc!=0 and trusted==b'' and native_count==before,'malformed request must return before trusted output/native branch');refusals.append(label)
        need(native_count==len(observations)==8 and len(refusals)==16,'complete actual native/refusal roster');pin_scans.observe('final','final',unchanged,pins);status,error='FOUR_INPUT_ROUTES_AND16_PRELAUNCH_REFUSALS_PASS',None
    except BaseException as failure:status,error='FIRST_FAILURE_PRESERVED',str(failure)
    summary={'schema':'gooo/header-addition-input-route/v1','status':status,'failure':error,'positive_requests':4 if error is None else None,'prelaunch_refusals':len(refusals),'refusal_labels':refusals,'actual_Gooo_commands':native_count,'typed_fields_checked':sum(x['typed_fields'] for x in observations),'typed_input_record_fields':sum(x['typed_input_record_fields'] for x in observations),'observations':observations,'elapsed_s':time.monotonic()-started,'scope':'Current helper output reaches actual pinned input-only execute/saved replay; all16 typed results use current independently computed Go case rows anchored to '+('experimental unauthenticated shard reference/case/raw receipts; no native acceptance or authenticated GitHub-run claim. ' if shard_index is not None else 'full-verifier timing/case/raw receipts. ')+'Raw replay parent checked. Inner typed canonical parent, direct CLI bypass, observation authenticity, network effects and durable/distributed safety remain NOT_CHECKED.'}
    save(out/'summary.json',summary);print(json.dumps(summary));return 0 if error is None else 1
def main():
    scan_state={};failed=True
    try:
        result=_main(scan_state);failed=result!=0;return result
    finally:
        if 'observer' in scan_state:scan_state['observer'].write_once(preserve_failure=failed)

if __name__=='__main__':sys.exit(main())
