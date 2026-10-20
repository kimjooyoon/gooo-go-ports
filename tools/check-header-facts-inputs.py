#!/usr/bin/env python3
"""Check the actual strict-helper -> input-only Gooo CLI route. Stdlib only."""
import argparse, hashlib, json, os, pathlib, re, signal, subprocess, sys, time

REV = '205896a413e5dea92366309c35ae85e542d055f6'
BASE, PACKAGE = 'app/header-field-facts:Report', 'go/golang.org/x/net/http/httpguts'
PREFIX = 'gooo://go-ports/examples/header-field-facts/'
FIELDS = ['name','value','name_valid','value_valid','name_empty','name_has_uppercase','value_has_obs_text','value_edge_whitespace','name_invalid_offset','name_invalid_octet','value_invalid_offset','value_invalid_octet']
IDS = ['name','value','name-valid','value-valid','name-empty','name-has-uppercase','value-has-obs-text','value-edge-whitespace','name-invalid-offset','name-invalid-octet','value-invalid-offset','value-invalid-octet']
FIXED = {'ports/net/httpguts/headers.gooo':'a033b96ab0adb394a9d79137715d7a4170e22061e4ae9aa282f8ab3671da9f76','examples/header-field-facts/main.gooo':'4aa66012d230252bc120fdacaac555172e3a1d6c5654181aa07c7347257a805e','header-field-facts.workspace.json':'6f49f7783a87d4829b81b18c19fed4308cb3009e4ce189973c83ccc8d4c179d9','examples/header-field-facts/request.json':'ae35739895a668eecffddf99cdede53afdf6ee86b78998005ed27c1c65a610f3','examples/header-field-facts/valid-request.json':'48c0290ff78343c3f26fb61b9f7b6c60de965e325374d67099ab13cbfa33732e','examples/header-field-facts/unicode-request.json':'0acfcc73613f46f87b18a892c56607175ef2e58932bd68940c1a6dce24bd2807'}
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
def main():
    parser = argparse.ArgumentParser()
    for name in ['compiler','input-command','go','root','out']: parser.add_argument('--'+name,required=True)
    a = parser.parse_args(); root,compiler,helper,go,out = [canonical(x) for x in [a.root,a.compiler,a.input_command,a.go,a.out]]
    need(root.is_dir() and not out.exists(),'root must exist and output must be fresh')
    out.mkdir(); (out/'tmp').mkdir(); started = time.monotonic(); observations = []; native_count = 0
    tracked = [compiler,helper,go,*[root/p for p in FIXED],*[root/p for p in ['upstream.json','go.mod','go.sum','cmd/header-facts-input/main.go','internal/headerinput/preflight.go']]]
    pins = {str(p):{'bytes':len(p.read_bytes()),'sha256':sha(p.read_bytes())} for p in tracked}
    def unchanged():
        for p,ref in pins.items():
            f = canonical(p); raw = f.read_bytes(); need(len(raw) == ref['bytes'] and sha(raw) == ref['sha256'],'frozen source/tool drift: '+p)
    def call(label,args,seconds,data=None):
        nonlocal native_count
        unchanged(); d = out/label; d.mkdir(); (d/'stdin').write_bytes(data or b''); begin = time.monotonic(); why = None; known = {}; child = None
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
                remaining = live()
                if remaining:
                    why = why or 'observed owned processes remained'
                    for group in {r['pgid'] for r in remaining}:
                        need(group > 1 and group != os.getpgrp(),'refuse unrelated process group')
                        try: os.killpg(group,signal.SIGKILL)
                        except ProcessLookupError: pass
                if child is not None:
                    try: child.wait(timeout=3)
                    except subprocess.TimeoutExpired: why = why or 'direct child not reaped'
                for _ in range(5):
                    remaining = live()
                    if not remaining: break
                    time.sleep(0.05)
                save(d/'process.json',{'exit_code':None if child is None else child.returncode,'failure':why,'wall_seconds':time.monotonic()-begin,'observed_owned_processes_remaining':[{k:r[k] for k in ['pid','pgid','birth']} for r in remaining],'scope':'Polling ancestry and unique tmp command paths; not a kernel fork trace.'})
        need(child is not None and why is None and not remaining,'command failure: '+label+' '+str(why))
        need((d/'stdout').stat().st_size <= 8<<20 and (d/'stderr').stat().st_size <= 2<<20,'final output cap')
        unchanged(); return child.returncode,(d/'stdout').read_bytes()
    env = os.environ.copy(); env.update(GOTOOLCHAIN='local',GOWORK='off',GOENV='off',GOPROXY='off',GOSUMDB='off',GOOO_LAYA_URL='',GOOO_LAYA_API_KEY='',TMPDIR=str(out/'tmp'))
    try:
        for p,h in FIXED.items(): need(pins[str(root/p)]['sha256'] == h,'current fixed source/request pin: '+p)
        lock = load((root/'upstream.json').read_bytes()); need(lock['compiler_revision'] == REV and lock['go_version'] == 'go1.27.3','upstream compiler/Go pins')
        save(out/'source-tool-pins.json',pins)
        rc,raw = call('go-version',[str(go),'version'],20); go_version = raw.decode().strip(); need(rc == 0 and re.fullmatch(r'go version go1\.27\.3 [a-z0-9_]+/[a-z0-9_]+',go_version),'Go identity')
        rc,raw = call('compiler-version',[str(compiler),'version','--build','--json'],20); identity = load(raw)
        need(rc == 0 and all(equal(identity[k],v) for k,v in {'schema':'gooo/build-identity/v1','version':'0.6.37-dev','compiler_source_sha':REV,'vcs_revision':REV,'source_status':'CLEAN_VCS','vcs_modified':'false','go_version':'go1.27.3','native_go_required':'go1.27.3','build_info_observed':True}.items()),'compiler build identity')
        manifest = root/'header-field-facts.workspace.json'; manifest_raw = manifest.read_bytes()
        # Both fixed sources are ASCII. Match pinned Go's JSON string digest, including HTML escaping.
        source_images = []
        for package,name in [(PACKAGE,'ports/net/httpguts/headers.gooo'),('app/header-field-facts','examples/header-field-facts/main.gooo')]:
            text = (root/name).read_bytes().decode('ascii'); encoded = json.dumps(text,ensure_ascii=False).replace('<','\\u003c').replace('>','\\u003e').replace('&','\\u0026').encode()
            source_images.append((package,name,'sha256:'+sha(encoded)))
        for i,name in enumerate(['request.json','valid-request.json','unicode-request.json']):
            request = (root/'examples/header-field-facts'/name).read_bytes(); supplied = load(request); label = 'positive-'+str(i)
            rc,trusted = call(label+'-preflight',[str(helper)],10,request)
            need(rc == 0 and equal(load(trusted),{'schema':'gooo/body-composition-inputs/v1','inputs':[{BASE+'.input0':supplied['name'],BASE+'.input1':supplied['value']}]}),'actual helper CLI schema')
            expected = dict(name=supplied['name'],value=supplied['value'],name_valid=i!=0,value_valid=i!=0,name_empty=False,name_has_uppercase=True,value_has_obs_text=i==2,value_edge_whitespace=False,name_invalid_offset=1 if i==0 else -1,name_invalid_octet=32 if i==0 else -1,value_invalid_offset=2 if i==0 else -1,value_invalid_octet=13 if i==0 else -1)
            for operation in ['execute','replay']:
                prior = out/(label+'-execute')/'stdout'; args = [str(compiler),'package',operation,'--json','--go',str(go),'--inputs',str(out/(label+'-preflight')/'stdout')]
                if operation == 'replay': args += ['--receipt',str(prior)]
                rc,raw = call(label+'-'+operation,args+[str(manifest)],90); r = load(raw); result = r['result']; rt = result['runtime']
                need(rc == 0 and r['schema'] == 'gooo/workspace-body-execution-receipt/v1' and r['decision'] == 'OBSERVED' and not r.get('error'),'OBSERVED receipt')
                need(r['manifest'] == str(manifest) and r['manifest_digest'] == 'sha256:'+sha(manifest_raw) and r['inputs_digest'] == 'sha256:'+sha(trusted) and 'cases_digest' not in r,'original manifest/input-only bytes')
                pe = result['program']['entry']; image = result['program']['workspace_image']
                need(result['schema'] == 'gooo/workspace-body-execution/v1' and pe['package_path']+':'+pe['activity'] == BASE,'original entry')
                need(image['schema'] == 'gooo/package-runtime-image/v1' and image['init_order'] == [PACKAGE,'app/header-field-facts'] and [(p['path'],f['filename'],f['source_digest']) for p in image['packages'] for f in p['sources']] == source_images,'current ordered source images')
                need(rt['schema'] == 'gooo/body-composition-runtime/v1' and rt['stage'] == 'COMPLETE' and not rt.get('failure') and equal([rt['finite_passed'],rt['finite_total'],rt['model_calls']],[0,0,0]) and rt['projection_replayed'] is True and rt['runtime_replayed'] is True,'input-only native runtime')
                need(rt['producer_source_sha'] == REV and rt['go_version'] == go_version and rt['go_tool_selection'] == 'explicit' and rt['go_tool_sha256'] == 'sha256:'+pins[str(go)]['sha256'],'current producer/tool pins')
                need(len(rt['traces']) == 1 and equal(rt['traces'][0]['case_index'],0) and len(rt['traces'][0]['deliveries']) == 1,'one ordered input-only delivery')
                d = rt['traces'][0]['deliveries'][0]; inputs = [{'port':'input0','entity_id':'gooo://std/text','value':supplied['name']},{'port':'input1','entity_id':'gooo://std/text','value':supplied['value']}]
                need(d['activity_id'] == PREFIX+'report/v1' and equal(d['inputs'],inputs) and equal(d['actual'],expected) and 'expected' not in d and 'passed' not in d,'exact input echoes and12 reference values/types')
                need(equal(d['actual_fields'],[{'id':PREFIX+ident,'name':field,'value':expected[field]} for field,ident in zip(FIELDS,IDS)]),'exact12 ordered field IDs/names/values/types')
                if operation == 'replay': need(r['replayed_from_sha256'] == 'sha256:'+sha(prior.read_bytes()) and result['replay']['schema'] == 'gooo/workspace-body-replay/v1' and equal(result['replay']['model_calls'],0),'saved raw outer parent')
                observations.append({'request':name,'operation':operation,'receipt_sha256':sha(raw),'typed_fields':12})
        negatives = [b'{}',b'{"name":"x"}',b'{"name":null,"value":""}',b'{"name":false,"value":""}',b'{"name":"x","value":[]}',b'{"name":"x","value":"","extra":1}',b'{"name":"x","name":"y","value":""}',b'{"name":"x","na\\u006de":"y","value":""}',b'{"name":"x","value":""}{}',b'{"name":"\\ud800","value":""}',b'{"name":"x","value":"\\udc00"}',b'{"name":"x","value":"\xff"}',json.dumps({'name':'x'*1025,'value':''}).encode(),json.dumps({'name':'x','value':'a'*1025}).encode(),b' '*16385,b'[]']
        for i,raw in enumerate(negatives):
            before = native_count; rc,trusted = call('negative-'+str(i),[str(helper)],10,raw)
            need(rc != 0 and trusted == b'' and native_count == before,'prelaunch refusal must emit no trusted inputs or launch Gooo')
        need(native_count == 6 and len(observations) == 6,'actual native route count'); unchanged()
        status,error = 'THREE_INPUT_ROUTES_AND16_PRELAUNCH_REFUSALS_PASS',None
    except BaseException as failure: status,error = 'FIRST_FAILURE_PRESERVED',str(failure)
    summary = {'schema':'gooo/header-facts-input-route/v1','status':status,'failure':error,'positive_requests':3 if error is None else None,'prelaunch_refusals':16 if error is None else None,'actual_Gooo_commands':native_count,'typed_fields_checked':sum(x['typed_fields'] for x in observations),'observations':observations,'elapsed_s':time.monotonic()-started,'scope':'Current helper output reaches real pinned Gooo execute/saved replay. Refusals return before that branch. Separate from940 differential cases. Independent inner typed canonical parent digest and protection against bypass/direct-CLI callers remain NOT_CHECKED.'}
    save(out/'summary.json',summary); print(json.dumps(summary)); return 0 if error is None else 1
if __name__ == '__main__': sys.exit(main())
