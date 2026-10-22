"""Fresh source-bound Gooo generation, independent oracle and row API verifier.

Uses the existing portable owned-process cancellation runner. Supplied caches are
reused and never removed; no cold-cache or global resource-bound claim is made.
"""
import argparse,copy,hashlib,importlib.util,json,os,shutil,traceback,zlib
from pathlib import Path
HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[1]
RUNNER=ROOT/'examples/service-grant-many/verify.py'
spec=importlib.util.spec_from_file_location('alias_origin_owned',RUNNER)
host=importlib.util.module_from_spec(spec);spec.loader.exec_module(host)
FACTS=host.strict((ROOT/'upstream.json').read_bytes())
PIN=FACTS['compiler_revision'];GO_VERSION=FACTS['go_version']
ENTRY='gooo/tool-origin:CompareAliasOrigin'

def sha(raw):return 'sha256:'+hashlib.sha256(raw).hexdigest()

def receipt(path):
    with path.open('rb') as source:wire=source.read((2 << 20)+1)
    assert len(wire)<=2 << 20
    stream=zlib.decompressobj(16+zlib.MAX_WBITS);raw=stream.decompress(wire,(32 << 20)+1)
    assert len(raw)<=32 << 20 and stream.eof and not stream.unused_data and not stream.unconsumed_tail
    return raw,host.strict(raw)

def verify_pair(execution,replay,cases_path,go_identity):
    cases=host.strict(cases_path.read_bytes())['cases'];original=None
    for path in (execution,replay):
        raw,doc=receipt(path);runtime=doc['result']['runtime']
        assert doc['decision']=='PASS' and doc['manifest_digest']==sha((ROOT/'alias-origin-change.workspace.json').read_bytes()) and doc['cases_digest']==sha(cases_path.read_bytes())
        assert runtime['producer_source_sha']==PIN and runtime['go_version']==go_identity['version'] and runtime['go_tool_sha256']=='sha256:'+go_identity['sha256'] and runtime['model_calls']==0
        assert runtime['finite_total']==runtime['finite_passed']==len(cases) and len(runtime['traces'])==len(cases)
        origins=doc['result']['program']['activity_origins'];assert len(origins)==1 and origins[0]['input_source']['source_sha256']==sha((ROOT/'ports/tool-origin/alias_origin.gooo').read_bytes())
        for i,(trace,case) in enumerate(zip(runtime['traces'],cases)):
            assert trace['case_index']==i and len(trace['deliveries'])==1
            delivery=trace['deliveries'][0];assert delivery['activity_id']=='gooo://tool-origin/v1/compare'
            host.exact(delivery['input'],case['inputs'][ENTRY]);host.exact(delivery['actual'],case['expected'][ENTRY]);assert len(delivery['actual'])==9 and delivery['actual']['execution_authorized'] is False
        if original is None:original=(raw,doc)
        else:
            assert doc['replayed_from_sha256']==sha(original[0]);host.exact(doc['result']['composition'],original[1]['result']['composition'])
    return len(cases),original[1]['result']['composition']['source']

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--compiler',type=Path,required=True);parser.add_argument('--go',type=Path,required=True);parser.add_argument('--out',type=Path,required=True);parser.add_argument('--go-cache',type=Path);parser.add_argument('--module-cache',type=Path);args=parser.parse_args()
    out=args.out.resolve();assert not out.is_relative_to(ROOT.resolve());out.mkdir(parents=True,exist_ok=False);host.R=out
    source_paths=[ROOT/'upstream.json',ROOT/'alias-origin-change.workspace.json',ROOT/'ports/tool-origin/alias_origin.gooo',RUNNER,*[p for p in HERE.rglob('*') if p.is_file() and '__pycache__' not in p.parts]]
    before={str(p.relative_to(ROOT)):host.ref(p) for p in source_paths};host.save(out/'source-before.json',before)
    first=None;summary={'schema':'gooo/alias-origin-verification/v1','compiler_revision':PIN,'go_version':GO_VERSION,'status':'FIRST_FAILURE_NO_RETRY','model_calls':0,'external_adoption':False,'ADK_runtime_executed':False,'native_acceptance':False,'execution_authorized':False}
    try:
        compiler,go=args.compiler.resolve(),args.go.resolve();assert compiler.is_file() and go.is_file()
        for name in ('cache','tmp','modcache','gopath','home','bin'):(out/name).mkdir(mode=0o700)
        cache=args.go_cache.resolve() if args.go_cache else out/'cache';modules=args.module_cache.resolve() if args.module_cache else out/'modcache'
        env={'PATH':str(go.parent)+':/usr/bin:/bin','GOROOT':str(go.parent.parent),'HOME':str(out/'home'),'TMPDIR':str(out/'tmp'),'GOCACHE':str(cache),'GOMODCACHE':str(modules),'GOPATH':str(out/'gopath'),'GOTOOLCHAIN':'local','GOENV':'off','GOWORK':'off','GOPROXY':'https://proxy.golang.org','GOSUMDB':'sum.golang.org','GOTELEMETRY':'off','CGO_ENABLED':'0','GOMAXPROCS':'2','GOFLAGS':'-mod=readonly -p=1','LANG':'C','LC_ALL':'C'}
        def run(label,argv,seconds=90,stdin=None,expected_exit=0,environment=None):return host.owned(label,argv,environment or env,seconds,ROOT,stdin,expected_exit)
        version=run('go-identity',[go,'version'],15).decode().strip();assert version in ('go version '+GO_VERSION+' darwin/arm64','go version '+GO_VERSION+' linux/amd64')
        go_identity=dict(host.ref(go),version=version)
        identity=host.strict(run('compiler-identity',[compiler,'version','--build','--json'],15));profile=host.ACTIVE_COMPILER_PROFILE
        assert profile['compiler_revision']==PIN and profile['go_version']==GO_VERSION,'shared validated runner/compiler profile differs from root facts'
        for key,value in {'compiler_source_sha':PIN,'version':FACTS['compiler_version'],'vcs_revision':PIN,'source_status':'CLEAN_VCS','vcs_modified':'false','go_version':GO_VERSION,'native_go_required':GO_VERSION,'build_info_observed':True}.items():host.exact(identity[key],value)
        host.exact(identity['module'],profile['module']);host.exact(identity['decision_runtime'],profile['decision_runtime'])
        reference=run('independent-kernel-reference',[go,'-C',HERE/'reference','run','.'],90,HERE/'kernel.inputs.json');assert reference==(HERE/'kernel.cases.json').read_bytes()
        def native_pair(stem,cases):
            manifest=ROOT/'alias-origin-change.workspace.json'
            run(stem+'-execute',[compiler,'package','execute','--json','--receipt-compression','gzip','--go',go,'--cases',cases,manifest],180)
            execution=out/(stem+'-execute')/'stdout'
            run(stem+'-replay',[compiler,'package','replay','--json','--receipt-compression','gzip','--go',go,'--receipt',execution,'--cases',cases,manifest],180)
            replay=out/(stem+'-replay')/'stdout';count,raw_projection=verify_pair(execution,replay,cases,go_identity)
            return execution,replay,count,raw_projection
        execution,replay,count,raw_projection=native_pair('kernel',HERE/'kernel.cases.json');assert count==111
        generated=out/'generated';run('source-bound-generation',['python3','-I','-B',HERE/'generate.py','--execution',execution,'--replay',replay,'--gofmt',go.parent/'gofmt','--out',generated],60)
        generated_names=[]
        for p in generated.rglob('*'):
            if p.is_file() and p.name!='generation.provenance.jsonld':
                name=str(p.relative_to(generated));assert p.read_bytes()==(HERE/name).read_bytes(),name;generated_names.append(name)
        host.save(out/'generated-asset-check.json',{'files':generated_names,'all_exact':True,'actual_PROV_O':host.ref(generated/'generation.provenance.jsonld')})
        run('check-consumed-pin-dependencies',['python3','-I','-B',HERE/'generator_dependencies_test.py','--execution',execution,'--replay',replay,'--gofmt',go.parent/'gofmt','--out',out/'consumed-pin-dependencies'],180)
        dependency=host.strict((out/'consumed-pin-dependencies/summary.json').read_bytes())
        assert dependency['status']=='CONSUMED_PINS_STABLE_UNRELATED_AND_REJECT_RELEVANT_CHANGES' and dependency['all_direct_wait_EOF_PG_ESRCH'] and dependency['public_source_unchanged'] and dependency['stable_assets']==7
        summary['consumed_pin_dependency_regression']=True
        run('module-integrity',[go,'-C',HERE,'mod','verify']);run('go-vet',[go,'-C',HERE,'vet','./...'])
        race=dict(env,CGO_ENABLED='1');run('API-race-and-schema-tests',[go,'-C',HERE,'test','-race','./...','-count=1'],180,environment=race)
        deps=[x for x in run('native-runtime-dependencies',[go,'-C',HERE,'list','-deps','-f','{{if not .Standard}}{{.ImportPath}}{{end}}','./cmd/compare']).decode().splitlines() if x];assert len(deps)==3 and all(x.startswith('github.com/kimjooyoon/gooo-go-ports/examples/alias-origin-change') for x in deps)
        binary=out/'bin/compare';run('native-CLI-build',[go,'-C',HERE,'build','-o',binary,'./cmd/compare']);run('native-CLI-build-info',[go,'version','-m',binary],20)
        runtime=dict(env,PATH='');runtime.pop('GOROOT',None)
        sample=host.strict((HERE/'example-input.json').read_bytes());duplicate=copy.deepcopy(sample);duplicate['after'].append({'server':'another','original_name':'other','exposed_name':'docs_search'})
        reports=[];actual_cases=[]
        unicode_alias='docs_<>&'+chr(0x2028)+chr(0x2029)
        unicode_case={'target_alias':unicode_alias,'before':[{'server':'서버','original_name':'é','exposed_name':unicode_alias}],'after':[{'server':'서버','original_name':'é','exposed_name':unicode_alias}]}
        for name,facts in [('origin-changed',sample),('ambiguous',duplicate),('opaque-unicode-origin',unicode_case)]:
            path=out/(name+'.json');path.write_text(json.dumps(facts,separators=(',',':'))+'\n');report=host.strict(run(name,[binary],15,path,environment=runtime));host.exact(report['facts'],facts)
            canonical=json.dumps(facts,ensure_ascii=False,separators=(',',':')).replace('<','\\u003c').replace('>','\\u003e').replace('&','\\u0026').replace('\u2028','\\u2028').replace('\u2029','\\u2029').encode();facts_digest=sha(canonical)
            assert report['facts_sha256']==facts_digest and report['input_sha256']==sha(path.read_bytes()) and report['decision']['context_id']=='gooo/alias-origin/v1/'+facts_digest and report['decision']['execution_authorized'] is False
            host.exact(report['bindings'],host.strict((HERE/'generation.json').read_bytes()));reports.append(report)
            flat={'context_id':'gooo/alias-origin/v1/'+facts_digest,'target_alias':facts['target_alias']}
            for side in ('before','after'):
                for col,key in [('servers','server'),('names','original_name'),('aliases','exposed_name')]:flat[side+'_'+col]=[row[key] for row in facts[side]]
            actual_cases.append({'inputs':{ENTRY:flat}})
        actual_input=out/'actual-kernel-inputs.json';actual_input.write_text(json.dumps({'schema':'gooo/body-composition-cases/v1','cases':actual_cases})+'\n')
        expected=run('independent-actual-reference',[go,'-C',HERE/'reference','run','.'],90,actual_input);actual_case_path=out/'actual-kernel.cases.json';actual_case_path.write_bytes(expected)
        for report,case in zip(reports,host.strict(expected)['cases']):host.exact(report['decision'],case['expected'][ENTRY])
        _,_,actual_count,actual_projection=native_pair('actual-CLI-kernel',actual_case_path);assert actual_count==3 and actual_projection==raw_projection
        _,original_replay=receipt(replay);negative=[]
        for label in ('wrong-parent','wrong-workspace-source','wrong-input','wrong-decision'):
            altered=copy.deepcopy(original_replay)
            if label=='wrong-parent':altered['replayed_from_sha256']='sha256:'+'0'*64
            elif label=='wrong-workspace-source':altered['result']['program']['workspace_image']['packages'][0]['sources'][0]['source_digest']='sha256:'+'0'*64
            elif label=='wrong-input':altered['result']['runtime']['traces'][0]['deliveries'][0]['input']['target_alias']='wrong'
            else:altered['result']['runtime']['traces'][0]['deliveries'][0]['actual']['state']='ORIGIN_CHANGED'
            import gzip
            path=out/(label+'.json.gz');path.write_bytes(gzip.compress(json.dumps(altered,separators=(',',':')).encode(),mtime=0))
            assert run(label,['python3','-I','-B',HERE/'generate.py','--execution',execution,'--replay',path,'--gofmt',go.parent/'gofmt','--check'],60,expected_exit=1)==b''
            negative.append({'control':label,'wire':host.ref(path)})
        host.save(out/'receipt-negative-controls.json',negative)
        bad=[('caller-authority',b'{"target_alias":"x","before":[],"after":[],"execution_authorized":true}'),('duplicate-key',b'{"target_alias":"x","target_alias":"y","before":[],"after":[]}'),('null-array',b'{"target_alias":"x","before":null,"after":[]}')]
        for name,data in bad:
            path=out/(name+'.json');path.write_bytes(data);assert run(name,[binary],15,path,1,runtime)==b'';assert host.strict((out/name/'stderr').read_bytes())['state']=='INVALID_TRANSPORT'
        after={str(p.relative_to(ROOT)):host.ref(p) for p in source_paths};host.exact(after,before);host.save(out/'source-after.json',after);host.check_cancelled()
        summary.update(status='SOURCE_KERNEL111_AND_ROW_API_NATIVE_CONSUMER_REPLAY_PASS',kernel_cases=count,actual_consumer_cases=actual_count,typed_decision_fields=9,public_row_API_cases=104,transport_refusals=len(bad),receipt_negative_controls=len(negative),runtime_path_empty=True,runtime_goroot_absent=True,runtime_nonstandard_packages=deps,reused_external_cache=bool(args.go_cache or args.module_cache),no_cold_cache_claim=True)
    except BaseException as error:
        first = {'type': type(error).__name__, 'message': str(error), 'traceback': traceback.format_exc()}
        host.save(out / 'first-failure.json', first)
    finally:
        pending = host.pending_cancellation()
        if pending is not None and first is None:
            first = {'type': type(pending).__name__, 'message': str(pending)}
            summary['status'] = 'FIRST_FAILURE_NO_RETRY'
        closed = bool(host.records) and all(r['waited'] and r['streams_EOF'] and r['PG_ESRCH'] for r in host.records)
        summary.update(first_failure=first, records=host.records, all_direct_wait_EOF_PG_ESRCH=closed)
        if first is None and not closed:
            summary['status'] = 'FIRST_FAILURE_NO_RETRY'
            first = {'type': 'RuntimeError', 'message': 'process closure not established'}
            summary['first_failure'] = first
        # Remove only completed owned executables. Shared caches and compilers stay.
        if closed:
            try:
                shutil.rmtree(out / 'bin', ignore_errors=False)
            except BaseException as error:
                diagnostic = {'type': type(error).__name__, 'message': str(error)}
                summary['owned_cleanup_failure'] = diagnostic
                if first is None:
                    first = diagnostic
        pending = host.pending_cancellation()
        if pending is not None and first is None:
            first = {'type': type(pending).__name__, 'message': str(pending)}
        if first is not None:
            summary['status'] = 'FIRST_FAILURE_NO_RETRY'
            summary['first_failure'] = first
        host.save(out / 'summary.json', summary)
    if first is not None:
        raise SystemExit(1)
    host.check_cancelled()
    print(json.dumps({'status': summary['status'], 'summary': host.ref(out / 'summary.json')}))


if __name__ == '__main__':
    with host.cancellation_handlers():
        main()
