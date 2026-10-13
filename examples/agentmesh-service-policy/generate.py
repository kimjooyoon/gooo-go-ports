#!/usr/bin/env python3
"""Produce projections only from pinned compiler-owned package execution/lowering.

All execute/replay outputs are retained privately under --out. Public program
snapshots contain compiler-owned source/entry data, not private manifest paths.
"""
import argparse,copy,hashlib,json,os,subprocess,tempfile
from pathlib import Path
if not __debug__: raise SystemExit("Python optimization disables integrity assertions; run without -O/PYTHONOPTIMIZE")
ROOT=Path(__file__).resolve().parent
REPO=ROOT.parents[1]
CANONICAL={"matcher.gooo":"ports/agentmesh/service_pattern.gooo","upstream/LICENSE":"ports/agentmesh/LICENSE-APACHE-2.0","matcher-workspace.json":"agentmesh-matcher.workspace.json","impact-workspace.json":"service-grant-impact.workspace.json","refinement-workspace.json":"service-grant-refinement.workspace.json"}
PUBLIC_SOURCES={"ports/agentmesh/service_pattern.gooo":"matcher.gooo","ports/agentmesh/LICENSE-APACHE-2.0":"upstream/LICENSE","examples/agentmesh-service-policy/impact.gooo":"impact.gooo","agentmesh-matcher.workspace.json":"matcher-workspace.json","service-grant-impact.workspace.json":"impact-workspace.json","examples/agentmesh-service-policy/refinement.gooo":"refinement.gooo","service-grant-refinement.workspace.json":"refinement-workspace.json"}
PIN='83928fa4538245abe33b08549da47f8cbdc50ed2'
MODULE='github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy'
FILES=['matcher.gooo','impact.gooo','refinement.gooo','matcher-workspace.json','impact-workspace.json','refinement-workspace.json','source-pin.json','compiler.json','upstream/LICENSE','upstream/api/tar.go.txt','upstream/api/network.go.txt','upstream/api/tar_test.go.txt','evidence/oracle-extracted.go.txt','evidence/extraction.json','evidence/oracle-go.mod.txt','evidence/oracle-command.go.txt','evidence/observations.json','evidence/repository-adaptation.json','matcher-cases.json','impact-cases.json','refinement-cases.json']
for prefix in ['matcher','impact','refinement']:
    FILES+=['receipts/'+prefix+'-'+suffix for suffix in ['program.json','lowered.gooo','body-codegen.json','seed-cases.json']]+['internal/'+prefix+'/generated.go']
FILES+=['model.go','domain.go','input.go','bridge.go','bindings.go','adapter_generated.go','cmd/evaluate/main.go','generate.py','verification_checks.py','verify.py','go.mod']
MIRRORS={'evidence/oracle-extracted.go.txt': 'oracle/extracted.go', 'evidence/extraction.json': 'oracle/extraction.json', 'evidence/oracle-go.mod.txt': 'oracle/go.mod', 'evidence/oracle-command.go.txt': 'oracle/cmd/oracle/main.go', 'evidence/observations.json': 'oracle/observations.json'}
CONFIGS=[('matcher','go/google/agentmesh/api','MatchServicePattern','gooo://agentmesh-prototype/match-service-pattern','(string,string,string)','bool'),('impact','app/service-grant-impact','CompareServiceGrant','gooo://service-impact/compare','(string,string,string,string)','GrantImpact'),('refinement','app/service-grant-refinement','AnalyzeServiceGrant','gooo://service-refinement/analyze','(string,string)','GrantRefinement')]
def digest(raw):return 'sha256:'+hashlib.sha256(raw).hexdigest()
def encoded(value):return (json.dumps(value,indent=2,sort_keys=True)+'\n').encode()
def source_digest(raw):
    value=json.dumps(raw.decode(),ensure_ascii=False,separators=(',',':')).replace('<','\\u003c').replace('>','\\u003e').replace('&','\\u0026').replace('\u2028','\\u2028').replace('\u2029','\\u2029')
    return digest(value.encode())
def stable_body(value):
    r=copy.deepcopy(value);latency=r['report'].pop('route_decision_latency_ms');assert isinstance(latency,(int,float)) and latency>=0
    env=r['report']['completeness_receipt']['scope'].pop('execution_environment');assert isinstance(env,str) and env.count('/')==1 and all(env.split('/'))
    return r

def main():
    ap=argparse.ArgumentParser();ap.add_argument('--compiler',type=Path,required=True);ap.add_argument('--go',type=Path,required=True);ap.add_argument('--out',type=Path,default=ROOT/'out/generation');ap.add_argument('--check',action='store_true');args=ap.parse_args()
    args.compiler=args.compiler.resolve();args.go=args.go.resolve();out=args.out.resolve();out.mkdir(parents=True,exist_ok=True);out.chmod(0o700)
    env=os.environ.copy();env.update(GOTOOLCHAIN='local',GOWORK='off',GOPROXY='off',GOSUMDB='off')
    def run(label,cmd,data=None,cwd=ROOT):
        r=subprocess.run([str(x) for x in cmd],input=data,cwd=cwd,env=env,capture_output=True,timeout=60)
        for stream,raw in [('stdout',r.stdout),('stderr',r.stderr)]:p=out/(label+'.'+stream);p.write_bytes(raw);p.chmod(0o600)
        if r.returncode!=0:raise RuntimeError(label+' failed; inspect retained private output')
        return r.stdout
    version=run('go-version',[args.go,'version']).decode().split();assert version[:3]==['go','version','go1.27.2'] and len(version)==4
    compiler_raw=run('compiler-identity',[args.compiler,'version','--build','--json']);identity=json.loads(compiler_raw)
    assert identity['compiler_source_sha']==identity['vcs_revision']==PIN and identity['source_status']=='CLEAN_VCS' and identity['vcs_modified']=='false' and identity['go_version']==identity['native_go_required']=='go1.27.2'
    for mirror,canonical in CANONICAL.items():
        assert (ROOT/mirror).read_bytes()==(REPO/canonical).read_bytes(), 'canonical source/workspace mirror diverged: '+mirror
    adaptation=json.loads((REPO/'upstream.json').read_bytes())['bounded_agentmesh_adaptations']
    assert len(adaptation)==1 and encoded(adaptation[0])==(ROOT/'evidence/repository-adaptation.json').read_bytes()
    produced={'compiler.json':compiler_raw,**{mirror:(ROOT/source).read_bytes() for mirror,source in MIRRORS.items()}};entries={}
    with tempfile.TemporaryDirectory(prefix='gooo-agentmesh-lowering-') as tmp:
        for prefix,pkg,activity,activity_id,input_type,output_type in CONFIGS:
            workspace=CANONICAL[prefix+'-workspace.json'];casepath=ROOT/'receipts'/(prefix+'-seed-cases.json');rows=json.loads(casepath.read_bytes())['cases']
            raw=run(prefix+'-execute',[args.compiler,'package','execute','--json','--cases',casepath,'--go',args.go,workspace],cwd=REPO);receipt=json.loads(raw)
            assert receipt['schema']=='gooo/workspace-body-execution-receipt/v1' and receipt['decision']=='PASS' and receipt['manifest_digest']==digest((REPO/workspace).read_bytes()) and receipt['cases_digest']==digest(casepath.read_bytes())
            p=receipt['result']['program'];assert p['schema']=='gooo/workspace-body-program/v1' and p['entry']['package_path']==pkg and p['entry']['activity']==activity and p['entry']['lowered_name']
            expected={'ports/agentmesh/service_pattern.gooo'}|({'examples/agentmesh-service-policy/'+prefix+'.gooo'} if prefix!='matcher' else set());sources={s['filename']:s['source_digest'] for package in p['workspace_image']['packages'] for s in package['sources']}
            assert set(sources)==expected and all(sha==source_digest((REPO/name).read_bytes()) for name,sha in sources.items())
            if prefix!='matcher':assert len(p['pure_calls']['sites'])==({'impact':2,'refinement':5}[prefix]) and all(site['caller']==pkg+':'+activity and site['callee']=='go/google/agentmesh/api:MatchServicePattern' for site in p['pure_calls']['sites'])
            rt=receipt['result']['runtime'];assert rt['producer_source_sha']==PIN and rt['go_version'].split()[:3]==['go','version','go1.27.2'] and rt['projection_replayed'] and rt['runtime_replayed'] and rt['finite_passed']==rt['finite_total']==len(rows)
            receipt_path=out/(prefix+'-execute.stdout')
            replay=json.loads(run(prefix+'-replay',[args.compiler,'package','replay','--json','--receipt',receipt_path,'--cases',casepath,'--go',args.go,workspace],cwd=REPO))
            assert replay['decision']=='PASS' and replay['replayed_from_sha256']==digest(raw) and replay['result']['program']==p and replay['result']['runtime']['finite_passed']==len(rows)
            lowered=p['lowered_gooo_source'].encode();lowerfile=Path(tmp)/(prefix+'.gooo');lowerfile.write_bytes(lowered)
            bodyraw=run(prefix+'-body-codegen',[args.compiler,'body-codegen','--json','--activity',p['entry']['lowered_name'],lowerfile]);body=json.loads(bodyraw);r=body['report']
            assert r['schema']=='gooo/body-codegen-report/v3' and r['decision']=='PASS' and r['activity']==p['entry']['lowered_name'] and r['activity_id']==activity_id and r['input_type']==input_type and r['output_type']==output_type
            assert r['compiler_source_sha']==PIN and r['source_digest']==digest(lowered) and r['generated_digest']==r['replay_digest']==digest(body['source'].encode()) and r['typecheck_passed'] and r['deterministic_replay'] and r['repository_writes']==0
            if prefix=='refinement':
                names=['status','rule_id','supported_subset','has_witness','witness_type','witness_name','before_match','after_match']
                assert len(r['record_types'])==1 and r['record_types'][0]['name']=='GrantRefinement' and r['record_types'][0]['id']=='gooo://service-refinement/decision/v1'
                fields=r['record_types'][0]['fields'];assert len(fields)==len(names)
                for i,f in enumerate(fields):
                    assert f['name']==names[i] and f['id']=='gooo://service-refinement/'+names[i] and f['type_id']=='urn:gooo:type:'+('boolean' if i in [2,3,6,7] else 'string')
            public_body=ROOT/'receipts'/(prefix+'-body-codegen.json')
            # Preserve existing receipt bytes if only excluded timing/environment
            # metadata changed; all semantic generation data was freshly checked.
            if public_body.exists() and stable_body(json.loads(public_body.read_bytes()))==stable_body(body):bodyraw=public_body.read_bytes()
            produced['receipts/'+prefix+'-program.json']=encoded(p);produced['receipts/'+prefix+'-lowered.gooo']=lowered;produced['receipts/'+prefix+'-body-codegen.json']=bodyraw
            produced['internal/'+prefix+'/generated.go']=run(prefix+'-gofmt',[args.go.parent/'gofmt'],body['source'].encode());entries[prefix]=p['entry']['lowered_name']
            if args.check:assert stable_body(json.loads((ROOT/'receipts'/(prefix+'-body-codegen.json')).read_bytes()))==stable_body(body)
    adapter='''// Code generated from pinned compiler-owned entry identities; DO NOT EDIT.
package servicepolicy
import (
 matcher "%s/internal/matcher"
 impact "%s/internal/impact"
 refinement "%s/internal/refinement"
)
func nativeMatcher(f MatcherFacts) bool { return matcher.%s(f.Pattern,f.ReqType,f.ReqName) }
func nativeImpact(f ImpactFacts) any { return impact.%s(f.Before,f.After,f.ReqType,f.ReqName) }
func nativeRefinement(f RefinementFacts) any { return refinement.%s(f.Before,f.After) }
'''%(MODULE,MODULE,MODULE,entries['matcher'],entries['impact'],entries['refinement'])
    produced['adapter_generated.go']=run('adapter-gofmt',[args.go.parent/'gofmt'],adapter.encode())
    if args.check:
        for name,raw in produced.items():
            if name.endswith('-body-codegen.json'):continue
            assert (ROOT/name).read_bytes()==raw,name+' differs from actual compiler-owned route'
        g=json.loads((ROOT/'generation.json').read_bytes());assert g['schema']=='gooo/agentmesh-native-generation/v1' and g['assets']=={name:digest((ROOT/name).read_bytes()) for name in FILES} and g['public_sources']=={canonical:digest((REPO/canonical).read_bytes()) for canonical in PUBLIC_SOURCES}
        print('PASS: fresh compiler-owned package execution/replay, source snapshots, exact projections and source bindings')
    else:
        for name,raw in produced.items():(ROOT/name).write_bytes(raw)
        (ROOT/'generation.json').write_bytes(encoded({'schema':'gooo/agentmesh-native-generation/v1','origin':'fresh_pinned_generation','public_sources':{canonical:digest((REPO/canonical).read_bytes()) for canonical in PUBLIC_SOURCES},'assets':{name:digest((ROOT/name).read_bytes()) for name in FILES}}))
        print('PASS: fresh pinned compiler-owned projections generated; finite comparisons remain separate')
if __name__=='__main__':main()
