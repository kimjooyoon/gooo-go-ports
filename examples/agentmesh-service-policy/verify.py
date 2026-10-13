#!/usr/bin/env python3
"""Fresh portable bounded Agent Mesh oracle/native/workspace checks, offline.

Run generate.py without --check first; a retained prototype origin is never
accepted as a newly verified public integration by this script.
"""
import argparse,datetime,hashlib,json,os,re,subprocess
from pathlib import Path
if not __debug__: raise SystemExit("Python optimization disables integrity assertions; run without -O/PYTHONOPTIMIZE")
from verification_checks import check_native_rows, check_workspace_trace
ROOT=Path(__file__).resolve().parent
REPO=ROOT.parents[1]
PIN='83928fa4538245abe33b08549da47f8cbdc50ed2'
def sha(raw):return hashlib.sha256(raw).hexdigest()
def encoded(x):return (json.dumps(x,indent=2,sort_keys=True)+'\n').encode()
def ident(p):r=p.read_bytes();return {'path':str(p),'bytes':len(r),'sha256':sha(r)}
def function(raw,name):
    prefix=('func '+name+'(').encode();start=raw.index(prefix);end=raw.index(b'\n}\n',start)+3;return raw[start:end]
def main():
    ap=argparse.ArgumentParser();ap.add_argument('--compiler',type=Path,required=True);ap.add_argument('--go',type=Path,required=True);ap.add_argument('--out',type=Path,default=ROOT/'out/verification');args=ap.parse_args()
    args.compiler=args.compiler.resolve();args.go=args.go.resolve();out=args.out.resolve();out.mkdir(parents=True,exist_ok=True);out.chmod(0o700)
    env=os.environ.copy();env.update(GOTOOLCHAIN='local',GOWORK='off',GOPROXY='off',GOSUMDB='off');commands=[]
    def run(label,cmd,data=None,cwd=ROOT,expect=0):
        before=datetime.datetime.now(datetime.timezone.utc).isoformat();r=subprocess.run([str(x) for x in cmd],input=data,cwd=cwd,env=env,capture_output=True,timeout=60)
        for name,raw in [('stdout',r.stdout),('stderr',r.stderr)]:p=out/(label+'.'+name);p.write_bytes(raw);p.chmod(0o600)
        commands.append({'label':label,'command':[str(x) for x in cmd],'started_at_utc':before,'finished_at_utc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'exit_code':r.returncode,'stdout':ident(out/(label+'.stdout')),'stderr':ident(out/(label+'.stderr'))})
        (out/'commands.json').write_bytes(encoded(commands));(out/'commands.json').chmod(0o600)
        if r.returncode!=expect:raise RuntimeError(label+' failed; inspect retained private outputs')
        return r.stdout
    run('verifier-helper-tests',['python3','-B','-m','unittest','-v','verify_test'])
    generation=json.loads((ROOT/'generation.json').read_bytes());assert generation['schema']=='gooo/agentmesh-native-generation/v1' and generation['origin']=='fresh_pinned_generation','fresh generator execution is required before public verification'
    source=json.loads((ROOT/'source-pin.json').read_bytes());assert source['revision']=='9fd1a41a15174df0aaef601c942410c2116bff91'
    for f in source['archive_files']+source['canonical_components']:
        raw=(ROOT/f['filename']).read_bytes();assert len(raw)==f['bytes'] and sha(raw)==f['sha256']
    assert (ROOT/'LICENSE-APACHE-2.0').read_bytes()==(ROOT/'upstream/LICENSE').read_bytes()
    extraction=json.loads((ROOT/'oracle/extraction.json').read_bytes());assert source['exact_function_bytes']==extraction['exact_function_bytes']
    for name,path in [('MatchServicePattern','upstream/api/tar.go.txt'),('ParseServiceTarget','upstream/api/network.go.txt')]:
        original=function((ROOT/path).read_bytes(),name);actual=function((ROOT/'oracle/extracted.go').read_bytes(),name)
        assert original==actual and sha(actual)==source['exact_function_bytes'][name]
    regex=source['exact_regexp_assignment'];assert ('var '+regex) in (ROOT/'oracle/extracted.go').read_text() and regex in (ROOT/'upstream/api/network.go.txt').read_text()
    cases=json.loads((ROOT/'matcher-cases.json').read_bytes());impacts=json.loads((ROOT/'impact-cases.json').read_bytes());assert len(cases)==63 and len(impacts)==16
    tests=(ROOT/'upstream/api/tar_test.go.txt').read_text();start=tests.index('func TestMatchServicePattern(');end=tests.index('\nfunc ',start+1);body=tests[start:end]
    official=[(a,b,c,d=='true') for a,b,c,d in re.findall(r'\{"([^"\n]*)", "([^"\n]*)", "([^"\n]*)", (true|false)\}',body)]
    assert len(official)==12 and [(r['pattern'],r['req_type'],r['req_name'],r['want']) for r in cases[:12]]==official and all(r['upstream'] for r in cases[:12])
    go_version=run('go-version',[args.go,'version']).decode().strip();assert go_version.split()[:3]==['go','version','go1.27.2']
    fresh=json.loads(run('fresh-extracted-upstream-oracle',[args.go,'run','./cmd/oracle','-cases',ROOT/'matcher-cases.json','-impact-cases',ROOT/'impact-cases.json'],cwd=ROOT/'oracle'))
    assert fresh['schema']=='gooo/agentmesh-matcher-oracle/v1' and fresh['go_version']=='go1.27.2' and fresh['source_revision']==source['revision']
    rows=fresh['observations'];assert len(rows)==63 and rows==json.loads((ROOT/'oracle/observations.json').read_bytes())
    assert len(fresh['impact_matches'])==16
    for r,c in zip(fresh['impact_matches'],impacts):assert r['name']==c['name'] and r['before_match']==c['want']['before_match'] and r['after_match']==c['want']['after_match']
    run('fresh-generation-check',['python3',ROOT/'generate.py','--compiler',args.compiler,'--go',args.go,'--out',out/'fresh-generation','--check'])
    tests=run('go-tests',[args.go,'test','-json','-race','./...']);run('go-vet',[args.go,'vet','./...']);run('oracle-vet',[args.go,'vet','./...'],cwd=ROOT/'oracle')
    run('cli-build',[args.go,'build','-trimpath','-o',out/'evaluate','./cmd/evaluate'])
    matcher=json.loads(run('native-matcher',[out/'evaluate','-mode','matcher'],encoded([{k:r[k] for k in ['pattern','req_type','req_name']} for r in rows])))
    impact=json.loads(run('native-impact',[out/'evaluate','-mode','impact'],encoded([{k:r[k] for k in ['before','after','req_type','req_name']} for r in impacts])))
    expected_bindings={'compiler_source_sha':PIN,'generation_sha256':'sha256:'+sha((ROOT/'generation.json').read_bytes()),'generation_origin':'fresh_pinned_generation','assets':generation['assets']}
    check_native_rows(matcher,impact,rows,impacts,expected_bindings)
    states=sorted(set(row['decision']['change'] for row in impact));assert states==['REVOKED','UNCHANGED_ALLOW','UNCHANGED_DENY','WIDENED']
    batches=[]
    for prefix,values,entry,fields in [('matcher',rows,'go/google/agentmesh/api:MatchServicePattern',['pattern','req_type','req_name']),('impact',impacts,'app/service-grant-impact:CompareServiceGrant',['before','after','req_type','req_name'])]:
        for offset in range(0,len(values),16):
            selected=values[offset:offset+16];corpus={'schema':'gooo/body-composition-cases/v1','cases':[{'inputs':{entry+'.input'+str(i):r[k] for i,k in enumerate(fields)},'expected':{entry:r['matched'] if prefix=='matcher' else r['want']}} for r in selected]}
            label=prefix+'-'+str(offset//16);casefile=out/(label+'-cases.json');casefile.write_bytes(encoded(corpus));casefile.chmod(0o600);assert casefile.stat().st_size<=32768
            workspace='agentmesh-matcher.workspace.json' if prefix=='matcher' else 'service-grant-impact.workspace.json';raw=run(label+'-execute',[args.compiler,'package','execute','--json','--cases',casefile,'--go',args.go,workspace],cwd=REPO);execute=json.loads(raw)
            replay=json.loads(run(label+'-replay',[args.compiler,'package','replay','--json','--receipt',out/(label+'-execute.stdout'),'--cases',casefile,'--go',args.go,workspace],cwd=REPO))
            snapshot=json.loads((ROOT/'receipts'/(prefix+'-program.json')).read_bytes())
            for result in [execute,replay]:
                assert result['schema']=='gooo/workspace-body-execution-receipt/v1' and result['decision']=='PASS' and result['manifest_digest']=='sha256:'+sha((REPO/workspace).read_bytes()) and result['cases_digest']=='sha256:'+sha(casefile.read_bytes())
                assert result['result']['program']==snapshot
                rt=result['result']['runtime'];assert rt['producer_source_sha']==PIN and rt['go_version']==go_version and rt['projection_replayed'] and rt['runtime_replayed'] and rt['finite_passed']==rt['finite_total']==len(selected) and rt['model_calls']==0
                assert rt['runs'] and all(r['started'] and r['completed'] and r['exit_code']==0 and not r['timed_out'] and not r['output_truncated'] for r in rt['runs'])
                assert len(rt['traces'])==len(selected)
                for index,(trace,case) in enumerate(zip(rt['traces'],corpus['cases'])):
                    check_workspace_trace(prefix,trace,index,case,snapshot,json.loads((ROOT/'receipts'/(prefix+'-body-codegen.json')).read_bytes())['report'])
            assert replay['replayed_from_sha256']=='sha256:'+sha(raw)
            batches.append({'case_count':len(selected),'cases':ident(casefile),'execution':ident(out/(label+'-execute.stdout')),'replay':ident(out/(label+'-replay.stdout'))})
    events=[json.loads(line) for line in tests.splitlines()];passed=[r['Test'] for r in events if r.get('Action')=='pass' and 'Test' in r]
    assert sum(name.startswith('TestEveryBoundAssetTamperRejected/') for name in passed)==len(generation['assets'])
    for name in ['TestEveryExactMatcherAndImpactFieldMalformed','TestUnsupportedDomainRefusalsAndBoundaries','TestCLIUsesGeneratedGoooAndWholeBatchRefusal','TestRehashedSourceProjectionAndGenerationTamperRejected','TestCLICountAndByteBoundaries']:assert name in passed
    proof={'schema':'gooo/agentmesh-native-verification/v1','status':'FINITE_FRESH_ORACLE_NATIVE_WORKSPACE_REPLAY_PASS','finished_at_utc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'source_revision':source['revision'],'compiler_source_sha':PIN,'go_version':go_version,'matcher':{'upstream_cases':12,'total_cases':63,'fresh_oracle':ident(out/'fresh-extracted-upstream-oracle.stdout'),'native':ident(out/'native-matcher.stdout')},'original_impact':{'cases':16,'fields_compared':3,'states':states,'native':ident(out/'native-impact.stdout')},'strict_json':{'every_field_mutations':sum(name.startswith('TestEveryExactMatcherAndImpactFieldMalformed/') for name in passed),'transactional_later_failure':True,'max_count':1024,'max_batch_bytes':1048576,'max_component_bytes':128},'verifier_helper_contracts':{'native_array_cardinality':True,'all_raw_impact_facts_and_strict_types':True,'complete_typed_workspace_delivery_identity':True,'optimized_python_refused':True},'public_sources':generation['public_sources'],'bound_assets':len(generation['assets']),'asset_tamper_refusals':len(generation['assets']),'workspace_batches':batches,'commands':commands,'limits':['Canonical printable ASCII scheme/name domain only; no full URI parser','Threefield per-request impact; not whole-policy inclusion or live authorization','No CEL eligibility/role checks/token trust/current-state provider/effect dispatcher/private APIProof replay','Source hashes identify consistency under trusted executable, not issuer/build/binary attestation','Finite79 native observations overlap79workspace cases and do not prove whole-domain or production safety']}
    (out/'verification.json').write_bytes(encoded(proof));(out/'verification.json').chmod(0o600)
    print('PASS: fresh upstream63matcher;16originalthreefield impacts; actual imported workspace execute/replay; strict native wrapper and complete bindings')
if __name__=='__main__':main()
