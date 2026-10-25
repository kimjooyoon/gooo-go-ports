"""Finite independent Go expectations, original Gooo execution and own replay.
This checks a bounded synthetic corpus, not authenticated collection or effects.
"""
import argparse
import copy
import gzip
import importlib.util
import json
import os
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]

def module(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    result = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(result)
    return result

e = module('explicit_availability_evidence', HERE/'evidence.py')
o = module('explicit_observation_evidence', HERE/'observation_evidence.py')
g = module('explicit_availability_generation', HERE/'generate.py')
og = module('explicit_observation_generation', HERE/'observation_generate.py')
host = e.host


def source_snapshot():
    paths = [ROOT/'upstream.json',e.MANIFEST,o.MANIFEST,*[ROOT/p for p in e.SOURCES.values()],ROOT/o.SOURCE,
             ROOT/'examples/tool-definition-binding/evidence.py',ROOT/'examples/recorded-catalog-comparison/generate.py',ROOT/'examples/service-grant-many/verify.py',
             ROOT/'definition-snapshot.workspace.json',ROOT/'ports/tool-origin/definition_snapshot.gooo']
    for directory in (HERE,ROOT/'examples/recorded-catalog-comparison',ROOT/'examples/tool-definition-snapshot'):
        paths.extend(p for p in directory.rglob('*') if p.is_file() and p.suffix in ('.py','.go','.mod','.json') and '__pycache__' not in p.parts)
    return {str(p.relative_to(ROOT)):host.ref(p) for p in sorted(set(paths))}


def main():
    parser = argparse.ArgumentParser()
    for key in ('compiler', 'go', 'out'):
        parser.add_argument('--'+key, type=Path, required=True)
    parser.add_argument('--go-cache', type=Path)
    parser.add_argument('--module-cache', type=Path)
    parser.add_argument('--prepare-generated', action='store_true', help='write fresh output assets without changing or comparing repository assets')
    args = parser.parse_args()
    out = args.out.resolve()
    assert not out.is_relative_to(ROOT) and not out.exists()
    out.mkdir(parents=True)
    host.R = out
    summary = {'schema':'gooo/explicit-availability-verification/v1', 'status':'FIRST_FAILURE_NO_RETRY', 'model_calls':0, 'external_adoption':False, 'live_effect_performed':False, 'native_acceptance':False}
    first = None
    try:
        compiler, go = args.compiler.resolve(), args.go.resolve()
        gofmt = go.parent/'gofmt'
        for name in ('home','tmp','gopath','cache','modcache'):
            (out/name).mkdir(mode=0o700)
        cache = args.go_cache.resolve() if args.go_cache else out/'cache'
        mods = args.module_cache.resolve() if args.module_cache else out/'modcache'
        assert not cache.is_relative_to(ROOT) and not mods.is_relative_to(ROOT)
        env = {'PATH':str(go.parent)+':/usr/bin:/bin', 'GOROOT':str(go.parent.parent), 'HOME':str(out/'home'), 'TMPDIR':str(out/'tmp'), 'GOCACHE':str(cache), 'GOMODCACHE':str(mods), 'GOPATH':str(out/'gopath'), 'GOTOOLCHAIN':'local', 'GOENV':'off', 'GOWORK':'off', 'GOPROXY':'off', 'GOSUMDB':'off', 'GOTELEMETRY':'off', 'CGO_ENABLED':'0', 'GOMAXPROCS':'2', 'GOFLAGS':'-mod=readonly -p=1', 'LANG':'C', 'LC_ALL':'C'}
        def run(label, argv, seconds=90):
            return host.owned(label, argv, env, seconds, ROOT)
        identity = host.strict(run('compiler-identity',[compiler,'version','--build','--json'],15))
        facts = host.strict(e.read(ROOT/'upstream.json'))
        for key, want in {'compiler_source_sha':e.PIN,'vcs_revision':e.PIN,'source_status':'CLEAN_VCS','vcs_modified':'false','go_version':e.GO_VERSION,'native_go_required':e.GO_VERSION,'build_info_observed':True,'version':facts['compiler_version']}.items():
            e.same(identity[key],want)
        e.same(identity['module'],host.ACTIVE_COMPILER_PROFILE['module'])
        e.same(identity['decision_runtime'],host.ACTIVE_COMPILER_PROFILE['decision_runtime'])
        version = run('Go-identity',[go,'version'],15).decode().strip()
        assert version in ('go version '+e.GO_VERSION+' darwin/arm64','go version '+e.GO_VERSION+' linux/amd64')
        go_identity = dict(host.ref(go),version=version)
        summary.update(compiler=host.ref(compiler),compiler_identity=identity,go_identity=go_identity)
        before = source_snapshot()
        host.save(out/'source-before.json',before)
        results = []
        for label, evidence, generator, refdir, request, corpus in (
            ('availability',e,g,'reference','reference-inputs.json','kernel.cases.json'),
            ('observation',o,og,'observation-reference','observation-reference-inputs.json','observation-kernel.cases.json')):
            expected = run(label+'-independent-Go-reference',[go,'-C',HERE/refdir,'run','.','-input',HERE/request])
            assert expected == e.read(HERE/corpus,2<<20)
            run(label+'-reference-vet',[go,'-C',HERE/refdir,'vet','./...'])
            assert run(label+'-reference-gofmt',[gofmt,'-l',HERE/refdir/'main.go'],15)==b''
            rows = host.strict(expected)['cases']
            batches = []
            for start in range(0,len(rows),8):
                number = start//8
                cases = out/(label+'-%02d.cases.json'%number)
                host.save(cases,{'schema':'gooo/body-composition-cases/v1','cases':rows[start:start+8]})
                stem = label+'-%02d'%number
                run(stem+'-execute',[compiler,'package','execute','--json','--receipt-compression','gzip','--go',go,'--cases',cases,evidence.MANIFEST],180)
                execution = out/(stem+'-execute')/'stdout'
                run(stem+'-replay',[compiler,'package','replay','--json','--receipt-compression','gzip','--go',go,'--receipt',execution,'--cases',cases,evidence.MANIFEST],180)
                replay = out/(stem+'-replay')/'stdout'
                evidence.verify_pair(execution,replay,cases,go_identity)
                batches.append({'first_case':start,'cases':cases.name,'execution':str(execution.relative_to(out)),'replay':str(replay.relative_to(out))})
            index = out/(label+'-generation-input.json')
            schema = 'gooo/explicit-'+('availability' if label=='availability' else 'observation')+'-generation-input/v1'
            host.save(index,{'schema':schema,'batches':batches})
            evidence.verify_index(index,go_identity)
            generated, used = generator.render(index,gofmt)
            directory = out/(label+'-generated'); directory.mkdir()
            for path, raw in generated.items():
                destination = directory/path;destination.parent.mkdir(parents=True,exist_ok=True);destination.write_bytes(raw)
                if not args.prepare_generated:
                    assert e.read(ROOT/path,4<<20)==raw,path
            host.save(directory/'generation.provenance.jsonld',generator.provenance(index,gofmt,generated,used))
            # Refuse corrupt source/value/identity/parent records without running a new native process.
            b=batches[0];execution=out/b['execution'];replay=out/b['replay'];cases=out/b['cases'];_,original=e.receipt(replay)
            controls=[]
            for kind in ('wrong-parent','wrong-source','wrong-input','wrong-field-id','authority-claim'):
                bad=copy.deepcopy(original);delivery=bad['result']['runtime']['traces'][0]['deliveries'][0]
                if kind=='wrong-parent':bad['replayed_from_sha256']='sha256:'+'0'*64
                elif kind=='wrong-source':bad['result']['program']['activity_origins'][0]['input_source']['source_sha256']='sha256:'+'0'*64
                elif kind=='wrong-input':
                    if label=='availability':delivery['inputs'][3]['value']['field_states'][0]='UNRECOGNIZED'
                    else:delivery['input']['value']='tampered'
                elif kind=='wrong-field-id':delivery['actual_fields'][0]['id']='gooo://wrong-field'
                else:delivery['actual']['execution_authorized']=True
                path=out/(label+'-'+kind+'.json.gz');path.write_bytes(gzip.compress(json.dumps(bad,separators=(',',':')).encode(),mtime=0))
                try:evidence.verify_pair(execution,path,cases,go_identity)
                except (AssertionError,ValueError):pass
                else:raise AssertionError('mutated receipt accepted: '+kind)
                controls.append({'mutation':kind,'receipt':host.ref(path),'refused':True})
            results.append({'profile':label,'cases':len(rows),'batches':len(batches),'generation_index':host.ref(index),'generated_assets':{p:e.sha(raw) for p,raw in generated.items()},'negative_controls':controls})
        after = source_snapshot()
        e.same(before,after);host.save(out/'source-after.json',after)
        summary.update(status='PASS',native_acceptance=False,finite_native_verified=True,profiles=results,source_unchanged=True,generated_comparison=not args.prepare_generated,scope='40 finite collection/definition cases with authored collection anchors and independent Go origin/definition oracle; 12 independently computed diagnostic cases; fresh source-bound execution and each own saved replay. No producer authentication, live capture, behavior equivalence or effect authority.')
    except BaseException as exc:
        first=exc;summary['first_failure']={'type':type(exc).__name__,'message':str(exc)[:2048]}
        raise
    finally:
        summary['processes']=[host.strict(e.read(p)) for p in sorted(out.glob('*/process.json'))]
        summary['all_direct_wait_EOF_PG_ESRCH']=bool(summary['processes']) and all(p['waited'] and p['streams_EOF'] and p['PG_ESRCH'] for p in summary['processes'])
        summary['remaining_owned_groups']=0 if summary['all_direct_wait_EOF_PG_ESRCH'] else 'NOT_ESTABLISHED'
        host.save(out/'summary.json',summary)
        print(json.dumps({'status':summary['status'],'summary':host.ref(out/'summary.json')}))

if __name__=='__main__':main()
