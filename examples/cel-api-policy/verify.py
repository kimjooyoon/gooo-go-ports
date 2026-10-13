#!/usr/bin/env python3
"""Fresh real-CEL, importing-workspace, native-CLI and saved-replay evidence.

Uses cached dependencies and an already-installed compiler/Go toolchain offline.
Full framework suites are partitioned to preserve the observed 32 KiB JSON limit;
the native adapter retains all 52 scalar cases including its 65536-byte boundary.
"""
import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import subprocess

ROOT=Path(__file__).resolve().parent
REPO=ROOT.parent.parent
PIN='83928fa4538245abe33b08549da47f8cbdc50ed2'

def digest(raw):return hashlib.sha256(raw).hexdigest()
def encoded(value):return (json.dumps(value,indent=2,sort_keys=True)+'\n').encode()
def ident(path):
    raw=path.read_bytes()
    return {'bytes':len(raw),'sha256':digest(raw),'path':str(path)}

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--go',type=Path,required=True);parser.add_argument('--compiler',type=Path,default=REPO/'.gooo');parser.add_argument('--out',type=Path,default=ROOT/'out');args=parser.parse_args()
    out=args.out.resolve();out.mkdir(parents=True,exist_ok=True);os.chmod(out,0o700)
    env=os.environ.copy();env.update(GOTOOLCHAIN='local',GOWORK='off',GOPROXY='off',GOSUMDB='off')
    commands=[]
    def run(label,cmd,data=None,cwd=REPO,expect=0):
        started=datetime.datetime.now(datetime.timezone.utc).isoformat()
        result=subprocess.run([str(x) for x in cmd],input=data,cwd=cwd,env=env,capture_output=True,timeout=60)
        for stream,raw in [('stdout',result.stdout),('stderr',result.stderr)]:
            path=out/(label+'.'+stream);path.write_bytes(raw);os.chmod(path,0o600)
        commands.append({'label':label,'command':[str(x) for x in cmd],'started_at_utc':started,'finished_at_utc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'exit_code':result.returncode,'stdout':ident(out/(label+'.stdout')),'stderr':ident(out/(label+'.stderr'))})
        if result.returncode!=expect:raise RuntimeError(label+' unexpected process exit; inspect retained diagnostics')
        return result.stdout
    go_version=run('go-version',[args.go,'version']).decode().strip();assert go_version.split()[:3]==['go','version','go1.27.2']
    pin=json.loads((ROOT/'oracle/source-pin.json').read_bytes())
    upstream=json.loads((REPO/'upstream.json').read_bytes())
    records=upstream['bounded_cel_adaptations'];assert len(records)==1
    adaptation=records[0];assert adaptation['id']=='cel-codelab-auth-or/v1'
    assert adaptation['revision']==pin['origin_revision']=='6d32ef54e4f124929730b15d5eb9b9205da58992'
    assert adaptation['target']=='ports/cel/scalar_access.gooo' and adaptation['consumer_source']=='examples/cel-api-policy/main.gooo'
    for name,sha,size in [(adaptation['source_copy'],adaptation['source_sha256'],adaptation['source_bytes']),(adaptation['license'],adaptation['license_sha256'],11358)]:
        raw=(REPO/name).read_bytes();assert len(raw)==size and digest(raw)==sha
    for f in pin['files']:
        raw=(ROOT/'oracle'/f['filename']).read_bytes();assert len(raw)==f['bytes'] and digest(raw)==f['sha256']
    assert adaptation['runtime_module']==pin['runtime_dependency']['module']=='cel.dev/cel-go'
    assert adaptation['runtime_version']==pin['runtime_dependency']['version']=='v0.32.0'
    assert adaptation['runtime_revision']==pin['runtime_dependency']['origin']['Hash']=='f2039bc647bca407d882d90436fc8b91bab1ae62'
    selected=json.loads(run('actual-oracle-module',[args.go,'list','-m','-json','cel.dev/cel-go'],cwd=ROOT/'oracle'))
    assert selected['Path']=='cel.dev/cel-go' and selected['Version']=='v0.32.0'
    assert selected['Sum']==pin['runtime_dependency']['module_sum'] and selected['GoModSum']==pin['runtime_dependency']['go_mod_sum'] and not selected.get('Replace')
    baseline_raw=(ROOT/'oracle/observations.json').read_bytes();baseline=json.loads(baseline_raw)
    spec=json.loads((ROOT/'oracle/api-spec-receipt.json').read_bytes());assert digest(baseline_raw)==spec['baseline_observations_sha256']
    fresh_raw=run('fresh-real-cel-oracle',[args.go,'run','./cmd/oracle','--cases','cases.json'],cwd=ROOT/'oracle');fresh=json.loads(fresh_raw)
    assert fresh['schema']=='gooo-cel-typed-proto-oracle/v1' and fresh['status']=='EXECUTED_REAL_CEL_EVALUATOR' and fresh['go_version']=='go1.27.2' and fresh['dependency']=='cel.dev/cel-go@v0.32.0'
    assert fresh['expression']==pin['expression'] and fresh['case_count']==len(fresh['observations'])==52
    assert fresh['observations']==baseline['observations'],'fresh real CEL observations differ from archived baseline'
    run('generator-comparison-tests',['python3','-B',ROOT/'generate_test.py'])
    run('fresh-generation-check',['python3',ROOT/'generate.py','--compiler',args.compiler,'--go',args.go,'--check'])
    tests=run('native-tests',[args.go,'test','-json','-race','./...'],cwd=ROOT)
    run('native-vet',[args.go,'vet','./...'],cwd=ROOT)
    run('native-build',[args.go,'build','-o',out/'evaluate','./cmd/evaluate'],cwd=ROOT)
    scalar_rows=fresh['observations'];api_rows=json.loads((ROOT/'oracle/api-composition-cases.json').read_bytes());assert len(api_rows)==32
    scalar=json.loads(run('native-scalar',[out/'evaluate','-mode','scalar'],encoded([r['facts'] for r in scalar_rows])))
    api=json.loads(run('native-api',[out/'evaluate','-mode','original-api'],encoded([r['facts'] for r in api_rows])))
    expected_fields={'scalar':8,'api':15}
    for name,actual,expected,key in [('scalar',scalar,scalar_rows,'observed'),('api',api,api_rows,'expected')]:
        assert len(actual)==len(expected)
        for row,ref in zip(actual,expected):
            assert row['facts']==ref['facts'] and row['decision']==ref[key],ref['id']
            assert len(row['decision'])==expected_fields[name] and row['bindings']['assets']==json.loads((ROOT/'generation.json').read_bytes())['assets'] and row['bindings']['compiler_source_sha']==PIN
            assert row['bindings']['generation_sha256']=='sha256:'+digest((ROOT/'generation.json').read_bytes())
    def check_framework_record(record,casefile,selected,entry,prefix):
        assert record['schema']=='gooo/workspace-body-execution-receipt/v1' and record['decision']=='PASS'
        workspace='cel-scalar.workspace.json' if prefix=='scalar' else 'cel-api-policy.workspace.json'
        assert record['manifest_digest']=='sha256:'+digest((REPO/workspace).read_bytes())
        assert record['cases_digest']=='sha256:'+digest(casefile.read_bytes())
        runtime=record['result']['runtime']
        assert runtime['producer_source_sha']==PIN and runtime['go_version']==go_version
        assert runtime['projection_replayed'] and runtime['runtime_replayed']
        assert runtime['runs'] and all(r['started'] and r['completed'] and r['exit_code']==0 and not r['timed_out'] and not r['output_truncated'] for r in runtime['runs'])
        assert runtime['finite_passed']==runtime['finite_total']==len(selected) and len(runtime['traces'])==len(selected)
        activity_id='gooo://cel-example/scalar-access' if prefix=='scalar' else 'gooo://api-example/api-access'
        field_base='gooo://cel-example/access-decision/' if prefix=='scalar' else 'gooo://api-example/decision/'
        for index,(trace,case) in enumerate(zip(runtime['traces'],selected)):
            assert trace['case_index']==index and len(trace['deliveries'])==1
            delivery=trace['deliveries'][0];expected=case['expected'][entry]
            assert delivery['activity_id']==activity_id and delivery['actual']==delivery['expected']==expected and delivery['passed'] is True
            values=[case['inputs'][entry+'.input'+str(i)] for i in range(len(case['inputs']))]
            assert delivery['inputs']==[{'port':'input'+str(i),'entity_id':'gooo://std/boolean' if isinstance(value,bool) else 'gooo://std/text','value':value} for i,value in enumerate(values)]
            assert len(delivery['actual_fields'])==len(expected)
            assert {f['name']:(f['id'],f['value']) for f in delivery['actual_fields']}=={name:(field_base+name.replace('_','-'),value) for name,value in expected.items()}
    suites=[]
    for prefix,workspace,rows,expected_key,entry in [('scalar','cel-scalar.workspace.json',scalar_rows,'observed','go/cel.dev/cel-go/codelab:ScalarAccess'),('api','cel-api-policy.workspace.json',api_rows,'expected','app/cel-api-policy:APIAccess')]:
        order=['group_present','group_value','principal_present','principal_value']+([] if prefix=='scalar' else ['authenticated','caller_tenant','resource_tenant','requested_action','allowed_action'])
        accepted=[];outside=[]
        for row in rows:
            vals=[row['facts'][k] for k in order]
            if any(isinstance(v,str) and len(v.encode())>1024 for v in vals):outside.append(row);continue
            accepted.append({'inputs':{entry+'.input'+str(i):v for i,v in enumerate(vals)},'expected':{entry:row[expected_key]}})
        saved=json.loads((ROOT/(prefix+'-workspace-cases.json')).read_bytes());assert saved=={'schema':'gooo/body-composition-cases/v1','cases':accepted}
        count=48 if prefix=='scalar' else 32;assert len(accepted)==count
        batch_records=[]
        for idx in range(0,len(accepted),16):
            label=prefix+'-workspace-'+str(idx//16);casefile=out/(label+'-cases.json');casefile.write_bytes(encoded({'schema':'gooo/body-composition-cases/v1','cases':accepted[idx:idx+16]}));os.chmod(casefile,0o600)
            assert casefile.stat().st_size<=32768
            executed=run(label,[args.compiler,'package','execute','--json','--cases',casefile,'--go',args.go,workspace]);receipt=out/(label+'-execution.json');receipt.write_bytes(executed);os.chmod(receipt,0o600)
            current=json.loads(executed);check_framework_record(current,casefile,accepted[idx:idx+16],entry,prefix)
            runtime=current['result']['runtime'];assert runtime['finite_passed']==runtime['finite_total']==min(16,count-idx) and runtime['producer_source_sha']==PIN and runtime['projection_replayed'] and runtime['runtime_replayed']
            seed=json.loads((ROOT/'receipts'/(prefix+'-execution.json')).read_bytes());assert current['result']['program']==seed['result']['program']
            replay=json.loads(run(label+'-replay',[args.compiler,'package','replay','--json','--receipt',receipt,'--cases',casefile,'--go',args.go,workspace]));check_framework_record(replay,casefile,accepted[idx:idx+16],entry,prefix);assert replay['result']['program']==current['result']['program']
            assert replay['replayed_from_sha256']=='sha256:'+digest(executed)
            assert replay['result']['runtime']['finite_passed']==replay['result']['runtime']['finite_total']==min(16,count-idx)
            batch_records.append({'case_count':min(16,count-idx),'cases':ident(casefile),'execution':ident(receipt),'replay':ident(out/(label+'-replay.stdout'))})
        probes=[]
        for row in outside:
            vals=[row['facts'][k] for k in order];casefile=out/(prefix+'-limit-'+row['id']+'.json');casefile.write_bytes(encoded({'schema':'gooo/body-composition-inputs/v1','inputs':[{entry+'.input'+str(i):v for i,v in enumerate(vals)}]}));os.chmod(casefile,0o600)
            result=json.loads(run(prefix+'-limit-'+row['id'],[args.compiler,'package','execute','--json','--inputs',casefile,'--go',args.go,workspace],expect=1))
            assert result['decision']=='FAIL_CLOSED' and 'result' not in result and result.get('error'), 'framework boundary unexpectedly supported'
            # Large raw JSON is rejected by the 32KiB gate before its1024textgate.
            probes.append({'id':row['id'],'input':ident(casefile),'diagnostic':result['error'],'native_case_still_compared':True})
        suites.append({'workspace':workspace,'case_count':count,'batches':batch_records,'outside_framework_domain':probes})
    full_suite_probes=[]
    for prefix,workspace in [('scalar','cel-scalar.workspace.json'),('api','cel-api-policy.workspace.json')]:
        source=ROOT/(prefix+'-workspace-cases.json')
        assert source.stat().st_size>32768
        rejected_size=json.loads(run(prefix+'-full-suite-size-probe',[args.compiler,'package','execute','--json','--cases',source,'--go',args.go,workspace],expect=1))
        assert rejected_size['decision']=='FAIL_CLOSED' and rejected_size['error']=='invalid JSON size or UTF-8'
        full_suite_probes.append({'source':ident(source),'observed':rejected_size,'handled_by_deterministic_16_case_batches':True})
    # Isolate text bound itself with1025bytes within the32KiB JSON transport limit.
    entry='go/cel.dev/cel-go/codelab:ScalarAccess';f=scalar_rows[0]['facts'];vals=[f['group_present'],'x'*1025,f['principal_present'],f['principal_value']]
    path=out/'scalar-text1025-inputs.json';path.write_bytes(encoded({'schema':'gooo/body-composition-inputs/v1','inputs':[{entry+'.input'+str(i):v for i,v in enumerate(vals)}]}));os.chmod(path,0o600)
    rejected=json.loads(run('scalar-text1025-probe',[args.compiler,'package','execute','--json','--inputs',path,'--go',args.go,'cel-scalar.workspace.json'],expect=1))
    assert rejected['decision']=='FAIL_CLOSED' and 'at most 1024 UTF-8 bytes' in rejected['error']
    events=[json.loads(line) for line in tests.splitlines()];tamper=[e['Test'] for e in events if e.get('Action')=='pass' and e.get('Test','').startswith('TestEveryBoundAssetTamperRejected/')];assert len(tamper)==25
    assert any(e.get('Action')=='pass' and e.get('Test')=='TestManifestTamperRejected' for e in events)
    evidence={'schema':'gooo/cel-public-native-verification/v1','status':'FINITE_REAL_CEL_IMPORTED_NATIVE_AND_REPLAY_PASS','finished_at_utc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'go_version':go_version,'compiler_source_sha':PIN,'fresh_real_cel':{'case_count':52,'output_fields_compared':8,'source':ident(ROOT/'oracle/source-pin.json'),'baseline_unchanged':ident(ROOT/'oracle/observations.json'),'current_execution':ident(out/'fresh-real-cel-oracle.stdout')},'original_api':{'case_count':32,'output_fields_compared':15,'reference':ident(ROOT/'oracle/api-composition-cases.json'),'native_output':ident(out/'native-api.stdout')},'workspace_execution':suites,'framework_full_suite_size_probes':full_suite_probes,'framework_text1025_probe':rejected,'bindings':{'bound_assets':25,'all_asset_tamper_refusals':25,'manifest_tamper_refused':True,'generation':ident(ROOT/'generation.json')},'checks':{'fresh_compiler_owned_import_lowering':'PASS','fresh_projection_check':'PASS','go_test_race':'PASS','go_vet':'PASS','strict_raw_json_unicode_domain':'PASS'},'commands':commands,'limits':['One bounded codelab expression; not a generic CEL interpreter or exported upstream API','Native strictJSON adapter has65536byte validUTF8 limit; framework1024text and32KiB JSON transport gates are separate','32 original API tuples are authored expectations, not CEL authentication/API-effect evidence','Authentication/trusted context/current-state enforcement and atomic external effects remain host responsibilities','Hash consistency is not issuer authentication or executable signature','Finite84native and80overlappingworkspace tuples do not prove whole-domain or production safety']}
    proof=out/'verification.json';proof.write_bytes(encoded(evidence));os.chmod(proof,0o600)
    print('PASS: fresh real-CEL52x8; importing workspaces48+32 and saved replay; native52x8+32x15; 25 bound-asset tamper refusals')

if __name__=='__main__':main()
