"""Portable finite Gooo/reference/API/CLI/native-replay verification.

Uses the existing owned-process runner; adds no engine build, service or full port
corpus. Synthetic saved-format cases are not external adoption or live capture.
"""
import argparse
import copy
import gzip
import importlib.util
import json
import os
import shutil
import stat
import sys
import traceback
from pathlib import Path

HERE=Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('definition_verifier_evidence',HERE/'evidence.py')
e=importlib.util.module_from_spec(spec);spec.loader.exec_module(e)
host=e.host;ROOT=e.ROOT;MODULE=ROOT/e.OUTPUT_ROOT
EXCLUDED={'.git','__pycache__','.cache','cache','tmp','out','output','bin','gopath','modcache','home','node_modules'}


def snapshot():
    paths=[ROOT/'upstream.json',e.MANIFEST,*[ROOT/p for p in e.SOURCES.values()],ROOT/'examples/service-grant-many/verify.py']
    for directory in (HERE,MODULE):
        for parent,dirs,files in os.walk(directory,followlinks=False):
            p=Path(parent)
            assert all(not (p/d).is_symlink() for d in dirs)
            dirs[:]=sorted(d for d in dirs if d not in EXCLUDED)
            paths += [p/f for f in sorted(files) if not f.endswith('.pyc')]
    result={}
    for p in sorted(set(paths)):
        assert not p.is_symlink() and stat.S_ISREG(p.stat().st_mode)
        result[str(p.relative_to(ROOT))]=dict(host.ref(p),mode=stat.S_IMODE(p.stat().st_mode))
    assert e.ASSETS <= set(result)
    return result


def provenance_graph(graph):
    """Check this generator's finite graph, not general PROV-O conformance."""
    assert type(graph) is list and graph
    by_id={}
    for node in graph:
        assert type(node) is dict and type(node.get('@id')) is str and node['@id']
        assert node['@id'] not in by_id,'duplicate provenance ID'
        assert node['@type'] in ('prov:Entity','prov:Activity')
        by_id[node['@id']]=node
    edges={identity:set() for identity in by_id}
    for identity,node in by_id.items():
        for key,target_type,many in [('prov:wasDerivedFrom','prov:Entity',True),('prov:wasGeneratedBy','prov:Activity',False),('prov:used','prov:Entity',True)]:
            if key not in node:continue
            assert node['@type']==('prov:Activity' if key=='prov:used' else 'prov:Entity')
            targets=node[key] if many else [node[key]]
            assert type(targets) is list
            for target in targets:
                assert type(target) is dict and set(target)=={'@id'}
                reference=target['@id']
                assert reference in by_id,'unresolved provenance reference'
                assert reference!=identity,'self-referential provenance edge'
                assert by_id[reference]['@type']==target_type,'wrong provenance reference type'
                edges[identity].add(reference)
    # Dependencies point from outputs to activities/inputs. Remove leaves first.
    remaining=set(edges)
    while remaining:
        leaves={identity for identity in remaining if not edges[identity]&remaining}
        assert leaves,'cyclic provenance dependencies'
        remaining-=leaves


def provenance(generated,index,gofmt):
    document=host.strict(e.read(generated/'generation.provenance.jsonld',2<<20))
    profile='gooo/definition-binding-file-occurrence/v1'
    assert document['entity_identity_profile']==profile
    graph=document['@graph'];provenance_graph(graph)
    activities=[n for n in graph if n['@type']=='prov:Activity']
    assert len(activities)==1
    def check_identity(node,role,path,digest):
        value=json.dumps([profile,activities[0]['@id'],role,path,digest],ensure_ascii=True,separators=(',',':')).encode()
        assert node['@id']=='urn:gooo:definition-binding:entity:'+e.sha(value).removeprefix('sha256:')
        assert node['role']==role and node['path']==path and node['sha256']==digest
    nodes={n['path']:n for n in graph if n['@type']=='prov:Entity'}
    assert len(nodes)==len(graph)-1
    used=[ROOT/'upstream.json',e.MANIFEST,*[ROOT/p for p in e.SOURCES.values()],HERE/'kernel.cases.json',*[HERE/p for p in e.REFERENCE],HERE/'generate.py',HERE/'evidence.py',ROOT/'examples/recorded-catalog-comparison/generate.py',ROOT/'examples/service-grant-many/verify.py',index,gofmt,gofmt.parent/'go']
    receipt_paths=set()
    for batch in host.strict(e.read(index))['batches']:
        for name in ('cases','execution','replay'):
            p=e.local_member(index.parent,batch[name]);used.append(p)
            if name!='cases':receipt_paths.add(p)
    dependencies=[]
    for p in used:
        key=str(p.relative_to(ROOT)) if p.is_relative_to(ROOT) else str(p)
        n=nodes[key];digest='sha256:'+host.ref(p)['sha256']
        check_identity(n,'input',key,digest)
        if p in receipt_paths:assert n['original_JSON_sha256']==e.sha(e.receipt(p)[0])
        dependencies.append({'@id':n['@id']})
    pins=json.dumps({'compiler_revision':e.PIN,'go_version':e.GO_VERSION},sort_keys=True,separators=(',',':')).encode()
    pin=nodes['upstream.json#consumed-definition-pins']
    check_identity(pin,'selected-pins','upstream.json#consumed-definition-pins',e.sha(pins))
    assert pin['canonicalJSON'].encode()==pins
    same=e.same;same(pin['jsonPointers'],['/compiler_revision','/go_version']);same(pin['prov:wasDerivedFrom'],[dependencies[0]])
    selected={'@id':pin['@id']};same(activities[0]['prov:used'],dependencies+[selected])
    assert len(nodes)==len(used)+1+len(e.ASSETS)
    for name in e.ASSETS:
        n=nodes[name];digest='sha256:'+host.ref(generated/name)['sha256']
        check_identity(n,'output',name,digest)
        same(n['prov:wasGeneratedBy'],{'@id':activities[0]['@id']})
        same(n['prov:wasDerivedFrom'],[selected,*dependencies[1:]])


def consumer_requests(records,fixtures):
    assert len(records)==len(fixtures['cases'])==12 and fixtures['evidence_kind']=='SYNTHETIC_FIXTURE'
    baseline=host.strict(e.read(HERE/'reference-inputs.json',2<<20))['cases'][5]['parent_literal']
    assert baseline['origin_state']=='UNCHANGED' and baseline['state']=='RECORDED_CONTEXT_COMPLETE'
    requests=[]
    for record,fixture in zip(records,fixtures['cases']):
        for key in ('label','input','before_policy_revision','after_policy_revision','expected_state','expected_definition_relation'):
            e.same(record[key],fixture[key])
        raw=record['raw_input_json'].encode();e.same(host.strict(raw),fixture['input'])
        p=record['report']['projection'];assert p['input_sha256']==e.sha(raw)
        values=p['inputs'];parent=copy.deepcopy(baseline)
        parent['origin_context_id']=values[e.ENTRY+'.input0']['context_id'];parent['context_id']=values[e.ENTRY+'.input1']['context_id']
        assert record['report']['decision']['state']==fixture['expected_state'] and record['report']['decision']['definition_relation']==fixture['expected_definition_relation']
        requests.append({'label':'raw-'+record['label'],'origin':values[e.ENTRY+'.input0'],'collection':values[e.ENTRY+'.input1'],'binding':values[e.ENTRY+'.input2'],'parent_literal':parent,'anchor_state':fixture['expected_state'],'anchor_definition_relation':fixture['expected_definition_relation']})
    return {'schema':'gooo/definition-binding-reference-inputs/v1','cases':requests}


def main():
    parser=argparse.ArgumentParser()
    for key in ('compiler','go','out'):parser.add_argument('--'+key,type=Path,required=True)
    for key in ('go-cache','module-cache'):parser.add_argument('--'+key,type=Path)
    args=parser.parse_args();out=args.out.resolve()
    assert not out.is_relative_to(ROOT.resolve())
    out.mkdir(parents=True,exist_ok=False);host.R=out;first=None
    summary={'schema':'gooo/definition-binding-verification/v1','status':'FIRST_FAILURE_NO_RETRY','compiler_revision':e.PIN,'go_version':e.GO_VERSION,'model_calls':0,'engine_builds':0,'external_adoption':False,'live_capture_executed':False,'synthetic_consumer_observations_only':True,'execution_authorized':False,'native_acceptance':False}
    try:
        before=snapshot();host.save(out/'source-before.json',before)
        facts=host.strict(e.read(ROOT/'upstream.json'));assert facts['compiler_revision']==e.PIN and facts['go_version']==e.GO_VERSION
        compiler=args.compiler.resolve();go=args.go.resolve();gofmt=go.parent/'gofmt'
        assert compiler.is_file() and go.is_file() and gofmt.is_file()
        for name in ('cache','tmp','modcache','gopath','home','bin'):(out/name).mkdir(mode=0o700)
        cache=args.go_cache.resolve() if args.go_cache else out/'cache';modules=args.module_cache.resolve() if args.module_cache else out/'modcache'
        assert not cache.is_relative_to(ROOT) and not modules.is_relative_to(ROOT)
        env={'PATH':str(go.parent)+':/usr/bin:/bin','GOROOT':str(go.parent.parent),'HOME':str(out/'home'),'TMPDIR':str(out/'tmp'),'GOCACHE':str(cache),'GOMODCACHE':str(modules),'GOPATH':str(out/'gopath'),'GOTOOLCHAIN':'local','GOENV':'off','GOWORK':'off','GOPROXY':'off','GOSUMDB':'off','GOTELEMETRY':'off','CGO_ENABLED':'0','GOMAXPROCS':'2','GOFLAGS':'-mod=readonly -p=1','LANG':'C','LC_ALL':'C'}
        def run(label,argv,seconds=90,stdin=None,expected_exit=0,environment=None):return host.owned(label,argv,environment or env,seconds,ROOT,stdin,expected_exit)
        version=run('go-identity',[go,'version'],15).decode().strip();assert version in ('go version '+e.GO_VERSION+' darwin/arm64','go version '+e.GO_VERSION+' linux/amd64')
        go_identity=dict(host.ref(go),version=version)
        identity=host.strict(run('compiler-identity',[compiler,'version','--build','--json'],15));profile=host.ACTIVE_COMPILER_PROFILE
        for key,value in {'compiler_source_sha':e.PIN,'vcs_revision':e.PIN,'source_status':'CLEAN_VCS','vcs_modified':'false','go_version':e.GO_VERSION,'native_go_required':e.GO_VERSION,'version':facts['compiler_version'],'build_info_observed':True}.items():e.same(identity[key],value)
        e.same(identity['module'],profile['module']);e.same(identity['decision_runtime'],profile['decision_runtime'])
        summary.update(compiler=host.ref(compiler),compiler_identity=identity,go_identity=go_identity)
        run('provenance-regression-controls',[sys.executable,'-I','-B',HERE/'provenance_test.py'],30)
        expected=run('independent-Go-reference',[go,'-C',HERE/'reference','run','.', '-input',HERE/'reference-inputs.json'])
        assert expected==e.read(HERE/'kernel.cases.json',2<<20)
        kernel=host.strict(expected);assert len(kernel['cases'])==60
        raw_projection=None
        def pairs(stem,corpus):
            nonlocal raw_projection
            root=out/stem;root.mkdir();index={'schema':'gooo/definition-binding-generation-input/v1','batches':[]};total=0
            for start in range(0,len(corpus['cases']),8):
                number=start//8;path=root/('batch-%02d.cases.json'%number)
                rows=corpus['cases'][start:start+8];host.save(path,{'schema':'gooo/body-composition-cases/v1','cases':rows})
                label=stem+'-%02d'%number
                run(label+'-execute',[compiler,'package','execute','--json','--receipt-compression','gzip','--go',go,'--cases',path,e.MANIFEST],180)
                execution=out/(label+'-execute')/'stdout'
                run(label+'-replay',[compiler,'package','replay','--json','--receipt-compression','gzip','--go',go,'--receipt',execution,'--cases',path,e.MANIFEST],180)
                replay=out/(label+'-replay')/'stdout'
                count,composition=e.verify_pair(execution,replay,path,go_identity);total+=count
                if raw_projection is None:raw_projection=composition['source']
                else:assert composition['source']==raw_projection
                index['batches'].append({'first_case':start,'cases':str(path.relative_to(out)),'execution':str(execution.relative_to(out)),'replay':str(replay.relative_to(out))})
            return total,index
        count,index=pairs('kernel',kernel);assert count==60
        index_path=out/'generation-input.json';host.save(index_path,index)
        generated=out/'generated'
        run('source-bound-generation',[sys.executable,'-I','-B',HERE/'generate.py','--index',index_path,'--gofmt',gofmt,'--out',generated],120)
        assert {str(p.relative_to(generated)) for p in generated.rglob('*') if p.is_file()}==e.ASSETS|{'generation.provenance.jsonld'}
        for name in e.ASSETS:assert e.read(generated/name,4<<20)==e.read(ROOT/name,4<<20),name
        provenance(generated,index_path,gofmt)
        run('exact-generated-check',[sys.executable,'-I','-B',HERE/'generate.py','--index',index_path,'--gofmt',gofmt,'--check'],120)
        run('module-integrity',[go,'-C',MODULE,'mod','verify'])
        run('go-vet',[go,'-C',MODULE,'vet','./...'])
        run('reference-vet',[go,'-C',HERE/'reference','vet','./...'])
        export=out/'consumer-export.json';race=dict(env,CGO_ENABLED='1',GOOO_DEFINITION_EXPORT=str(export))
        run('API-CLI-race-export',[go,'-C',MODULE,'test','-race','./...','-count=1'],180,environment=race)
        records=host.strict(e.read(export,2<<20));fixtures=host.strict(e.read(MODULE/'definition-consumer-cases.json',2<<20))
        requests=consumer_requests(records,fixtures);request_path=out/'consumer-reference-inputs.json';host.save(request_path,requests)
        expected=run('consumer-independent-Go-reference',[go,'-C',HERE/'reference','run','.', '-input',request_path])
        consumers=host.strict(expected)
        assert len(consumers['cases'])==12
        deps=[x for x in run('runtime-dependencies',[go,'-C',MODULE,'list','-deps','-f','{{if not .Standard}}{{.ImportPath}}{{end}}','./cmd/compare']).decode().splitlines() if x]
        module='github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison'
        assert set(deps)=={module,module+'/internal/kernel',module+'/internal/definitionkernel',module+'/internal/observationkernel',module+'/internal/availabilitykernel',module+'/internal/availabilityobservationkernel',module+'/cmd/compare'} and len(deps)==7
        binary=out/'bin/compare';run('CLI-build',[go,'-C',MODULE,'build','-trimpath','-buildvcs=false','-o',binary,'./cmd/compare'])
        info=host.strict(run('CLI-build-info',[go,'version','-m','-json',binary],20));assert info['GoVersion']==e.GO_VERSION and info['Main']['Path']==module and not info.get('Deps')
        settings={x['Key']:x['Value'] for x in info['Settings']};assert settings['CGO_ENABLED']=='0' and settings['-trimpath']=='true'
        runtime=dict(env,PATH='');runtime.pop('GOROOT',None)
        observations=[]
        for n,(record,case) in enumerate(zip(records,consumers['cases'])):
            raw=record['raw_input_json'].encode();path=out/('consumer-%02d.json'%n);path.write_bytes(raw)
            wire=run('CLI-consumer-%02d'%n,[binary,'--definitions','--before-policy',record['before_policy_revision'],'--after-policy',record['after_policy_revision']],15,path,environment=runtime)
            report=host.strict(wire);e.same(report,record['report'])
            e.same(report['decision'],case['expected'][e.ENTRY]);e.same(report['projection']['inputs'],case['inputs'])
            observations.append({'label':record['label'],'input':host.ref(path),'report':host.ref(out/('CLI-consumer-%02d'%n)/'stdout'),'state':report['decision']['state'],'definition_relation':report['decision']['definition_relation']})
        assert run('repeat-identical-input',[binary,'--definitions','--before-policy',record['before_policy_revision'],'--after-policy',record['after_policy_revision']],15,path,environment=runtime)==wire
        actual,_=pairs('consumer',consumers);assert actual==12
        host.save(out/'consumer-observations.json',observations)
        # Mutations must fail both the common evidence reader and the generator.
        first_batch=index['batches'][0];execution=e.local_member(out,first_batch['execution']);replay=e.local_member(out,first_batch['replay']);cases=e.local_member(out,first_batch['cases'])
        _,original=e.receipt(replay);controls=[]
        for label in ('wrong-parent','wrong-source','wrong-definition-input','wrong-field-ID','wrong-authority','input-only'):
            altered=copy.deepcopy(original);delivery=altered['result']['runtime']['traces'][0]['deliveries'][0]
            if label=='wrong-parent':altered['replayed_from_sha256']='sha256:'+'0'*64
            elif label=='wrong-source':altered['result']['program']['activity_origins'][0]['input_source']['source_sha256']='sha256:'+'0'*64
            elif label=='wrong-definition-input':delivery['inputs'][2]['value']['context_id']='tampered'
            elif label=='wrong-field-ID':delivery['actual_fields'][0]['id']='gooo://wrong-field'
            elif label=='wrong-authority':delivery['actual']['execution_authorized']=True
            else:altered['decision']='OBSERVED';altered['result']['runtime']['finite_total']=0
            bad=out/(label+'.json.gz');bad.write_bytes(gzip.compress(json.dumps(altered,separators=(',',':')).encode(),mtime=0))
            try:e.verify_pair(execution,bad,cases,go_identity)
            except (AssertionError,ValueError):pass
            else:raise AssertionError('receipt mutation accepted: '+label)
            altered_index=copy.deepcopy(index);altered_index['batches'][0]['replay']=bad.name
            bad_index=out/(label+'-index.json');host.save(bad_index,altered_index)
            assert run(label,[sys.executable,'-I','-B',HERE/'generate.py','--index',bad_index,'--gofmt',gofmt,'--check'],120,expected_exit=1)==b''
            controls.append({'label':label,'receipt':host.ref(bad),'verifier_refused':True,'generator_refused':True})
        for label,field,value in [('coverage-gap','first_case',1),('metadata-path-escape','cases','../outside.cases.json')]:
            bad_index=copy.deepcopy(index);bad_index['batches'][0][field]=value;p=out/(label+'-index.json');host.save(p,bad_index)
            assert run(label,[sys.executable,'-I','-B',HERE/'generate.py','--index',p,'--gofmt',gofmt,'--check'],120,expected_exit=1)==b''
            controls.append({'label':label,'generator_refused':True})
        probe=HERE/'probes/nonzero-counted-loop.gooo'
        run('probe-declaration',[compiler,'check',probe],30)
        refusal=host.strict(run('probe-generation',[compiler,'body-codegen','--json','--activity','Probe',probe],30,expected_exit=1))
        assert refusal['decision']=='FAIL_CLOSED' and refusal['source']=='' and refusal['repository_writes']==0 and refusal['compiler_source_sha']==e.PIN and refusal['error']=='for must use the same named counter from zero'
        host.save(out/'receipt-negative-controls.json',controls)
        after=snapshot();e.same(before,after);host.save(out/'source-after.json',after)
        summary.update(status='SOURCE_DEFINITION60_AND_SAVED_FRAME12_NATIVE_REPLAY_PASS',kernel_cases=count,actual_consumer_cases=actual,typed_decision_fields=32,receipt_negative_controls=8,generated_assets=len(e.ASSETS),generated_assets_exact=True,PROV_O_bindings_verified=True,source_unchanged=True,source_files=len(before),runtime_path_empty=True,runtime_goroot_absent=True,runtime_nonstandard_packages=deps,repeated_identical_input_equal=True,reused_external_cache=bool(args.go_cache or args.module_cache),no_cold_cache_claim=True,syntax_probe_refused=True)
    except BaseException as error:
        first={'type':type(error).__name__,'message':str(error),'traceback':traceback.format_exc()};host.save(out/'first-failure.json',first)
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
        summary.update(first_failure=first,records=host.records,all_direct_wait_EOF_PG_ESRCH=closed)
        host.save(out/'summary.json',summary)
    if first is not None:raise SystemExit(1)
    host.check_cancelled();print(json.dumps({'status':summary['status'],'summary':host.ref(out/'summary.json')}))

if __name__=='__main__':
    with host.cancellation_handlers():main()
