"""Check saved API exports against independent expectations and original Gooo.
Exactly thirteen synthetic records; the separately supplied retained CE input is optional.
"""
import argparse
import importlib.util
import json
from pathlib import Path

_spec = importlib.util.spec_from_file_location('explicit_availability_verifier', Path(__file__).resolve().with_name('verify.py'))
v = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(v)

e,o,host,ROOT,HERE=v.e,v.o,v.host,v.ROOT,v.HERE
SYNTHETIC=('supplied-unknown','unobserved-unknown','unobserved-other-text','asymmetric-unobserved','known-page-change','unobserved-policy','optional-timestamps','backend-unobserved-same','backend-unobserved-different','backend-unobserved-asymmetric','backend-unobserved-known-host','backend-unobserved-known-connection','backend-unobserved-known-name')
CE='retained-ce-missing20'

def reference_request(record):
    label=record['label'];p=record['report']['projection'];inputs=p['inputs']
    origin,collection,binding,availability=[inputs[e.ENTRY+'.input'+str(n)] for n in range(4)]
    assert label in (*SYNTHETIC,CE)
    # These are authored finite collection anchors. No report decision is used
    # to manufacture its own expected value; the independent Go reference
    # computes all definition relations, raw result values and invalidity.
    complete=label in ('supplied-unknown','unobserved-policy','optional-timestamps')
    state='RECORDED_CONTEXT_COMPLETE' if complete else 'COLLECTION_CONTEXT_UNKNOWN'
    reason='RECORDED_SCOPE_PAGES_AND_CONTEXT_SATISFIED' if complete else 'EXPLICIT_CONTEXT_UNOBSERVED'
    action='FOLLOW_ALIAS_RESULT' if complete else 'RECORD_CAPTURE_TIME_CONTEXT'
    if label=='known-page-change':state,reason,action='UNSTABLE_CONFIGURATION','RECORDED_CONTEXT_CHANGED_DURING_CAPTURE','RECAPTURE_STABLE_CONTEXT'
    bi=origin['before_aliases'].index(origin['target_alias']);ai=origin['after_aliases'].index(origin['target_alias'])
    parent={'origin_context_id':origin['context_id'],'context_id':collection['context_id'],'origin_state':'UNCHANGED','origin_reason':'SAME_DECLARED_ORIGIN','origin_before_index':bi,'origin_after_index':ai,'state':state,'reason':reason,'next_action':action,'first_problem_code':'' if complete else reason,'authorization_relation':'SAME_RECORDED' if complete else 'UNKNOWN','contains_synthetic_fixture':label!=CE,'remote_atomicity':'UNKNOWN'}
    wanted='BINDINGS_MATCH' if label in ('supplied-unknown','optional-timestamps') else 'UNASSESSED'
    if label.startswith('backend-unobserved-'):
        parent['authorization_relation']='SAME_RECORDED'
    unavailable_origin=label in ('backend-unobserved-same','backend-unobserved-different','backend-unobserved-asymmetric',CE)
    known_origin=label.startswith('backend-unobserved-known-')
    if unavailable_origin:
        parent['origin_state']='ORIGIN_UNOBSERVED';parent['origin_reason']='BACKEND_BINDING_UNOBSERVED'
    if known_origin:
        parent['origin_state']='ORIGIN_CHANGED';parent['origin_reason']='DIFFERENT_SUPPLIED_ORIGIN_COMPONENT';wanted='ORIGIN_NOT_UNCHANGED'

    assert record['expected_state']==wanted and record['expected_collection_state']==state
    return {'label':label,'origin':origin,'collection':collection,'binding':binding,'availability':availability,'parent_literal':parent,'anchor_state':wanted,'anchor_definition_relation':'UNASSESSED' if unavailable_origin or known_origin else 'SAME_SUPPLIED'}


def main():
    p=argparse.ArgumentParser()
    for key in ('compiler','go','out','synthetic-export'):p.add_argument('--'+key,type=Path,required=True)
    p.add_argument('--retained-ce-export',type=Path)
    p.add_argument('--go-cache',type=Path)
    p.add_argument('--module-cache',type=Path)
    a=p.parse_args();out=a.out.resolve();assert not out.exists() and not out.is_relative_to(ROOT);out.mkdir(parents=True);host.R=out
    summary={'schema':'gooo/explicit-availability-consumer-verification/v1','status':'FIRST_FAILURE_NO_RETRY','native_acceptance':False,'external_adoption':False,'live_effect_performed':False,'model_calls':0}
    try:
        go=a.go.resolve();compiler=a.compiler.resolve()
        for name in ('home','tmp','gopath','cache','modcache'):(out/name).mkdir(mode=0o700)
        cache=a.go_cache.resolve() if a.go_cache else out/'cache';mods=a.module_cache.resolve() if a.module_cache else out/'modcache'
        env={'PATH':str(go.parent)+':/usr/bin:/bin','GOROOT':str(go.parent.parent),'HOME':str(out/'home'),'TMPDIR':str(out/'tmp'),'GOCACHE':str(cache),'GOMODCACHE':str(mods),'GOPATH':str(out/'gopath'),'GOTOOLCHAIN':'local','GOENV':'off','GOWORK':'off','GOPROXY':'off','GOSUMDB':'off','GOTELEMETRY':'off','CGO_ENABLED':'0','GOMAXPROCS':'2','GOFLAGS':'-mod=readonly -p=1','LANG':'C','LC_ALL':'C'}
        def run(label,argv,seconds=90,stdin=None,environment=None):return host.owned(label,argv,environment or env,seconds,ROOT,stdin)
        before=v.source_snapshot();host.save(out/'source-before.json',before)
        identity=host.strict(run('compiler-identity',[compiler,'version','--build','--json'],15))
        assert identity['compiler_source_sha']==identity['vcs_revision']==e.PIN and identity['source_status']=='CLEAN_VCS' and identity['vcs_modified']=='false' and identity['go_version']==e.GO_VERSION
        e.same(identity['module'],host.ACTIVE_COMPILER_PROFILE['module']);e.same(identity['decision_runtime'],host.ACTIVE_COMPILER_PROFILE['decision_runtime'])
        version=run('Go-identity',[go,'version'],15).decode().strip();assert version in ('go version '+e.GO_VERSION+' darwin/arm64','go version '+e.GO_VERSION+' linux/amd64');go_identity=dict(host.ref(go),version=version)
        synthetic=host.strict(e.read(a.synthetic_export,4<<20));assert [r['label'] for r in synthetic]==list(SYNTHETIC)
        groups=[('synthetic',synthetic)];sources=[a.synthetic_export]
        if a.retained_ce_export:
            retained=host.strict(e.read(a.retained_ce_export,4<<20));assert len(retained)==1 and retained[0]['label']==CE
            assert e.sha(retained[0]['raw_input_json'].encode())=='sha256:3c3594278dc721804b0b46f8fc3bd3f6ff37a094e5b0bf0802978362ece4f7fb'
            groups.append(('retained-ce',retained));sources.append(a.retained_ce_export)
        binary=out/'compare'
        run('CLI-build',[go,'-C',ROOT/'examples/recorded-catalog-comparison','build','-trimpath','-buildvcs=false','-o',binary,'./cmd/compare'])
        info=host.strict(run('CLI-build-info',[go,'version','-m','-json',binary],15));assert info['GoVersion']==e.GO_VERSION and not info.get('Deps')
        runtime=dict(env,PATH='');runtime.pop('GOROOT',None)
        results=[]
        for label,rows in groups:
            requests=[];diagnostics=[]
            for record in rows:
                report=record['report'];projection=report['projection'];raw=record['raw_input_json'].encode();sidecar=record['availability_json'].encode()
                assert len(raw)+len(sidecar)<=1<<20 and projection['input_sha256']==e.sha(raw) and projection['availability_sha256']==e.sha(sidecar)
                policy={'before_revision':record['before_policy_revision'],'after_revision':record['after_policy_revision']}
                assert projection['policy_sha256']==e.sha(json.dumps(policy,separators=(',',':'),ensure_ascii=False).encode())
                assert len(report['observations'])==len(projection['field_projection']['inputs'])==34
                raw_path=out/(record['label']+'-raw.json');raw_path.write_bytes(raw)
                sidecar_path=out/(record['label']+'-availability.json');sidecar_path.write_bytes(sidecar)
                wire=run(record['label']+'-CLI',[binary,'--definitions','--diagnostics','--availability',sidecar_path,'--before-policy',record['before_policy_revision'],'--after-policy',record['after_policy_revision']],15,raw_path,runtime)
                assert len(wire)<=4<<20;e.same(host.strict(wire),report)
                requests.append(reference_request(record))
                for n,(field,actual) in enumerate(zip(projection['field_projection']['inputs'],report['observations'])):
                    assert field['value_state'] in ('SUPPLIED','UNOBSERVED') and actual['value']==field['value']
                    diagnostics.append({'label':record['label']+'-%d'%n,'input':field})
                e.same(report['bindings'],host.strict(e.read(ROOT/'examples/recorded-catalog-comparison/availability-generation.json')))
                e.same(report['observation_bindings'],host.strict(e.read(ROOT/'examples/recorded-catalog-comparison/availability-observation-generation.json')))
            request=out/(label+'-reference-inputs.json');host.save(request,{'schema':'gooo/explicit-availability-reference-inputs/v1','cases':requests})
            expected=run(label+'-independent-Go-reference',[go,'-C',HERE/'reference','run','.','-input',request]);cases=out/(label+'.cases.json');cases.write_bytes(expected)
            expected_cases=host.strict(expected)['cases'];assert len(expected_cases)==len(rows)
            for c,r in zip(expected_cases,rows):e.same(c['inputs'],r['report']['projection']['inputs']);e.same(c['expected'][e.ENTRY],r['report']['decision'])
            request=out/(label+'-diagnostic-reference-inputs.json');host.save(request,{'schema':'gooo/explicit-observation-reference-inputs/v1','cases':diagnostics})
            diagnostic=host.strict(run(label+'-independent-Go-diagnostics',[go,'-C',HERE/'observation-reference','run','.','-input',request]))['cases']
            actuals=[d for r in rows for d in r['report']['observations']];assert len(actuals)==len(diagnostic)
            for c,d in zip(diagnostic,actuals):e.same(c['expected'][o.ENTRY],d)
            batches=[];total=0
            for start in range(0,len(expected_cases),8):
                stem=label+'-%02d'%(start//8);batch=out/(stem+'.cases.json')
                host.save(batch,{'schema':'gooo/body-composition-cases/v1','cases':expected_cases[start:start+8]})
                run(stem+'-execute',[compiler,'package','execute','--json','--receipt-compression','gzip','--go',go,'--cases',batch,e.MANIFEST],180)
                execution=out/(stem+'-execute')/'stdout'
                run(stem+'-replay',[compiler,'package','replay','--json','--receipt-compression','gzip','--go',go,'--receipt',execution,'--cases',batch,e.MANIFEST],180)
                replay=out/(stem+'-replay')/'stdout';count,_=e.verify_pair(execution,replay,batch,go_identity);total+=count
                batches.append({'cases':host.ref(batch),'execution':host.ref(execution),'own_replay':host.ref(replay)})
            assert total==len(rows)
            results.append({'kind':label,'records':len(rows),'diagnostics_independently_checked':len(diagnostic),'cases':host.ref(cases),'batches':batches})
        after=v.source_snapshot();e.same(before,after);host.save(out/'source-after.json',after)
        summary.update(source_unchanged=True,cli_report_equal=True,binary=host.ref(binary),status='PASS',finite_native_verified=True,exports=[host.ref(p) for p in sources],profiles=results,scope='Original Gooo and own replay agree with exported API decisions for thirteen synthetic records and an optional separately labeled retained CE record. Collection expectations are authored finite anchors; all 34 diagnostics per record are independently computed by Go. This is not authenticated capture or whole-project CI acceptance.')
    except BaseException as exc:
        summary['first_failure']={'type':type(exc).__name__,'message':str(exc)[:2048]};raise
    finally:
        summary['processes']=[host.strict(e.read(p)) for p in sorted(out.glob('*/process.json'))]
        summary['all_direct_wait_EOF_PG_ESRCH']=bool(summary['processes']) and all(p['waited'] and p['streams_EOF'] and p['PG_ESRCH'] for p in summary['processes'])
        host.save(out/'summary.json',summary);print(json.dumps({'status':summary['status'],'summary':host.ref(out/'summary.json')}))

if __name__=='__main__':main()
