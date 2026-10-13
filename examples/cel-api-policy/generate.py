#!/usr/bin/env python3
"""Use the pinned compiler's actual workspace execution as the lowering producer.

The pinned release has no package-inspect command. No Gooo body is rewritten or
flattened by this script. Seed execution/replay is real; generation is separate.
"""
import argparse
import copy
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parent
REPO = ROOT.parent.parent
PIN = '83928fa4538245abe33b08549da47f8cbdc50ed2'
CANONICAL = {'canonical/scalar_access.gooo':'ports/cel/scalar_access.gooo',
             'canonical/main.gooo':'examples/cel-api-policy/main.gooo',
             'canonical/cel-scalar.workspace.json':'cel-scalar.workspace.json',
             'canonical/cel-api-policy.workspace.json':'cel-api-policy.workspace.json',
             'canonical/source-pin.json':'examples/cel-api-policy/oracle/source-pin.json'}
CONFIGS = [('scalar','cel-scalar.workspace.json','go/cel.dev/cel-go/codelab','ScalarAccess','gooo://cel-example/scalar-access','(bool,string,bool,string)','AccessDecision'),
           ('api','cel-api-policy.workspace.json','app/cel-api-policy','APIAccess','gooo://api-example/api-access','(bool,string,bool,string,bool,string,string,string,string)','ApiDecision')]
FILES = list(CANONICAL) + ['compiler.json','adapter_generated.go','model.go','bridge.go','bindings.go','input.go','cmd/evaluate/main.go','generate.py']
for prefix,*_ in CONFIGS:
    FILES += ['receipts/'+prefix+'-'+suffix for suffix in ['seed-inputs.json','execution.json','replay.json','lowered.gooo','body-codegen.json']]
    FILES += ['internal/'+prefix+'/generated.go']

def digest(raw): return 'sha256:'+hashlib.sha256(raw).hexdigest()
def source_digest(raw):
    encoded=json.dumps(raw.decode(),ensure_ascii=False,separators=(",",":")).replace("<","\\u003c").replace(">","\\u003e").replace("&","\\u0026").replace("\u2028","\\u2028").replace("\u2029","\\u2029").encode()
    return digest(encoded)
def canonical(value): return (json.dumps(value,indent=2,sort_keys=True)+'\n').encode()
def run(cmd, data=None):
    env=os.environ.copy();env.update(GOTOOLCHAIN='local',GOWORK='off',GOPROXY='off',GOSUMDB='off')
    return subprocess.run([str(x) for x in cmd],input=data,cwd=REPO,env=env,capture_output=True,check=True,timeout=60).stdout

def program(receipt, workspace, package, activity, prefix):
    assert receipt['schema']=='gooo/workspace-body-execution-receipt/v1'
    assert receipt['decision'] in ['OBSERVED','PASS']
    assert receipt['manifest_digest']==digest((REPO/workspace).read_bytes())
    assert receipt['inputs_digest']==digest((ROOT/'receipts'/(prefix+'-seed-inputs.json')).read_bytes())
    p=receipt['result']['program']; assert p['schema']=='gooo/workspace-body-program/v1'
    e=p['entry']; assert e['package_path']==package and e['activity']==activity
    assert p['lowered_gooo_source'] and e['lowered_name']
    image_entry=p['workspace_image']['entry']
    source='ports/cel/scalar_access.gooo' if prefix=='scalar' else 'examples/cel-api-policy/main.gooo'
    inputs=['Boolean','Text','Boolean','Text']+([] if prefix=='scalar' else ['Boolean','Text','Text','Text','Text'])
    output='AccessDecision' if prefix=='scalar' else 'ApiDecision'
    assert image_entry=={'package_path':package,'source':source,'activity':activity,'inputs':inputs,'output':output}
    for pkg in p['workspace_image']['packages']:
        expected_name='ScalarAccess' if pkg['path']=='go/cel.dev/cel-go/codelab' else 'APIAccess'
        expected_id='gooo://cel-example/scalar-access' if expected_name=='ScalarAccess' else 'gooo://api-example/api-access'
        expected_output='gooo://cel-example/access-decision/v1' if expected_name=='ScalarAccess' else 'gooo://api-example/decision/v1'
        expected_inputs=['gooo://std/boolean','gooo://std/text','gooo://std/boolean','gooo://std/text']+([] if expected_name=='ScalarAccess' else ['gooo://std/boolean','gooo://std/text','gooo://std/text','gooo://std/text','gooo://std/text'])
        actual=[e for e in pkg['exports'] if e['kind']=='activity']
        assert actual==[{'name':expected_name,'kind':'activity','id':expected_id,'input_types':expected_inputs,'output_type':expected_output}]

    expected={'ports/cel/scalar_access.gooo'} | ({'examples/cel-api-policy/main.gooo'} if prefix=='api' else set())
    sources={s['filename']:s['source_digest'] for pkg in p['workspace_image']['packages'] for s in pkg['sources']}
    assert set(sources)==expected
    for filename,sha in sources.items():assert sha==source_digest((REPO/filename).read_bytes())
    if prefix=='api':
        sites=p['pure_calls']['sites']
        assert len(sites)==1 and sites[0]['caller']=='app/cel-api-policy:APIAccess' and sites[0]['callee']=='go/cel.dev/cel-go/codelab:ScalarAccess'
        assert {a['source'] for a in p['pure_calls']['activities']}==expected
    runtime=receipt['result']['runtime']
    assert runtime['producer_source_sha']==PIN and runtime['go_version'].split()[:3]==['go','version','go1.27.2']
    assert runtime['projection_replayed'] and runtime['runtime_replayed']
    assert runtime['runs'] and all(r['started'] and r['completed'] and r['exit_code']==0 and not r['timed_out'] and not r['output_truncated'] for r in runtime['runs'])
    return p

def stable(receipt):
    value=copy.deepcopy(receipt)
    latency=value['report'].pop('route_decision_latency_ms')
    assert isinstance(latency,(int,float)) and latency>=0
    scope=value['report']['completeness_receipt']['scope']
    environment=scope.pop('execution_environment')
    assert isinstance(environment,str) and environment.count('/')==1 and all(environment.split('/'))
    # Environment is an observation of the generating process, not code identity.
    # Every other source/projection/semantic/producer field remains byte-exact.
    return value

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--compiler',type=Path,default=REPO/'.gooo');parser.add_argument('--go',type=Path,required=True);parser.add_argument('--check',action='store_true');args=parser.parse_args()
    version=run([args.go,'version']).decode().split();assert version[:3]==['go','version','go1.27.2'] and len(version)==4
    compiler_raw=run([args.compiler,'version','--build','--json']);compiler=json.loads(compiler_raw)
    assert compiler['compiler_source_sha']==compiler['vcs_revision']==PIN
    assert compiler['source_status']=='CLEAN_VCS' and compiler['vcs_modified']=='false' and compiler['go_version']==compiler['native_go_required']=='go1.27.2'
    assets={'compiler.json':compiler_raw}
    for mirror,source in CANONICAL.items():assets[mirror]=(REPO/source).read_bytes()
    entries={}
    with tempfile.TemporaryDirectory(prefix='gooo-cel-lowering-') as tmp:
        for prefix,workspace,package,activity,activity_id,input_type,output_type in CONFIGS:
            seed='examples/cel-api-policy/receipts/'+prefix+'-seed-inputs.json'
            fresh=run([args.compiler,'package','execute','--json','--inputs',seed,'--go',args.go,workspace]);receipt=json.loads(fresh)
            p=program(receipt,workspace,package,activity,prefix)
            if args.check:
                saved=json.loads((ROOT/'receipts'/(prefix+'-execution.json')).read_bytes())
                assert program(saved,workspace,package,activity,prefix)==p,'fresh compiler-owned lowering changed'
                replay=json.loads((ROOT/'receipts'/(prefix+'-replay.json')).read_bytes());assert program(replay,workspace,package,activity,prefix)==p
            else:
                assets['receipts/'+prefix+'-execution.json']=fresh
                replaypath=Path(tmp)/(prefix+'-execution.json');replaypath.write_bytes(fresh)
                replay=run([args.compiler,'package','replay','--json','--receipt',replaypath,'--inputs',seed,'--go',args.go,workspace])
                assert program(json.loads(replay),workspace,package,activity,prefix)==p
                assets['receipts/'+prefix+'-replay.json']=replay
            lowered=p['lowered_gooo_source'].encode();assets['receipts/'+prefix+'-lowered.gooo']=lowered
            path=Path(tmp)/(prefix+'-lowered.gooo');path.write_bytes(lowered)
            generated=run([args.compiler,'body-codegen','--json','--activity',p['entry']['lowered_name'],path]);body=json.loads(generated);r=body['report']
            assert r['schema']=='gooo/body-codegen-report/v3' and r['decision']=='PASS' and r['activity']==p['entry']['lowered_name']
            assert r['activity_id']==activity_id and r['input_type']==input_type and r['output_type']==output_type
            assert r['compiler_source_sha']==PIN and r['source_digest']==digest(lowered)
            assert r['completeness_receipt']['scope']['execution_environment']==version[3]
            assert r['generated_digest']==r['replay_digest']==digest(body['source'].encode()) and r['typecheck_passed'] and r['deterministic_replay'] and r['repository_writes']==0
            projected=run([args.go.parent/'gofmt'],('// Code generated from compiler-owned workspace lowering; DO NOT EDIT.\n'+body['source']).encode())
            assets['internal/'+prefix+'/generated.go']=projected;assets['receipts/'+prefix+'-body-codegen.json']=generated
            entries[prefix]=p['entry']['lowered_name']
            if args.check:assert stable(json.loads((ROOT/'receipts'/(prefix+'-body-codegen.json')).read_bytes()))==stable(body)
    adapter='''// Code generated from actual lowered entry identities; DO NOT EDIT.
package nativepolicy
import (
 scalar "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy/internal/scalar"
 api "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy/internal/api"
)
func nativeScalar(f Facts) any { return scalar.%s(f.GroupPresent, f.GroupValue, f.PrincipalPresent, f.PrincipalValue) }
func nativeAPI(f APIFacts) any { return api.%s(f.GroupPresent, f.GroupValue, f.PrincipalPresent, f.PrincipalValue, f.Authenticated, f.CallerTenant, f.ResourceTenant, f.RequestedAction, f.AllowedAction) }
'''%(entries['scalar'],entries['api'])
    assets['adapter_generated.go']=run([args.go.parent/'gofmt'],adapter.encode())
    if args.check:
        for name,raw in assets.items():
            if name.endswith('-body-codegen.json'):continue
            assert (ROOT/name).read_bytes()==raw,name+' changed'
        manifest=json.loads((ROOT/'generation.json').read_bytes());assert manifest['schema']=='gooo/cel-public-generation/v1'
        assert manifest['assets']=={name:digest((ROOT/name).read_bytes()) for name in FILES}
        print('PASS: fresh imported seed execution, saved lowering/replay, exact native projections and all bindings')
    else:
        for name,raw in assets.items(): (ROOT/name).write_bytes(raw)
        (ROOT/'generation.json').write_bytes(canonical({'schema':'gooo/cel-public-generation/v1','assets':{name:digest((ROOT/name).read_bytes()) for name in FILES}}))
        print('PASS: actual imported seed execution/replay and native projections generated; full finite comparisons separate')
if __name__=='__main__':main()
