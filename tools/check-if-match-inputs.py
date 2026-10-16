#!/usr/bin/env python3
"""Observe strict two-Text handoff separately from the284-case Go differential suite.

Uses the existing bounded process/source helpers; it does not compute Go expected
values, provide a native fallback, or implement inner canonical receipt hashing.
"""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import shutil
import signal
import sys

_BASE = Path(__file__).resolve().with_name('check-header-addition-host.py')
_SPEC = importlib.util.spec_from_file_location('if_match_process_helpers', _BASE)
H = importlib.util.module_from_spec(_SPEC)
_SPEC.loader.exec_module(H)
ENTRY = 'go/net/http/precondition:IfMatchCondition'
PACKAGE, ACTIVITY = ENTRY.split(':')
ID = 'gooo://go/net/http/if-match-condition'
RESULT = 'gooo://go/net/http/if-match-result/v1'
SOURCE = 'ports/net/http/precondition.gooo'
SOURCE_SHA = '9924cde4a2e01222658929a87007d6e34db85f2d4f49c057d110bb8d1bf05aeb'
MANIFEST = 'if-match.workspace.json'
SCHEMA = 'gooo/body-composition-inputs/v1'
KEYS = ('if_match', 'current_etag')


def encode(value):
    return (json.dumps(value, ensure_ascii=False, separators=(',', ':'))+'\n').encode('utf-8')


def negative_requests():
    """35 explicit transport refusals. Invalid header grammar is not a refusal."""
    wrong = (None, False, True, 0, 1.5, [], {})
    for port, key in enumerate(KEYS):
        base = dict(zip(KEYS, ('"a"', '"a"')))
        for index, value in enumerate(wrong):
            yield f'wrong-{port}-{index}', encode(dict(base, **{key: value}))
        missing = dict(base); del missing[key]
        yield f'missing-{port}', encode(missing)
        raw = encode(base); token = encode(key).strip()+b':'+encode(base[key]).strip()
        for label, duplicate in [('plain', encode(key).strip()), ('escaped', encode(key).strip().replace(b'_', b'\\u005f', 1))]:
            yield f'duplicate-{port}-{label}', raw.replace(token, token+b','+duplicate+b':'+encode(base[key]).strip(), 1)
        raw = encode(dict(base, **{key: 'PLACEHOLDER'}))
        yield f'wire-utf8-{port}', raw.replace(b'PLACEHOLDER', b'\xff')
        for label, value in [('high', b'\\ud800'), ('low', b'\\udc00')]:
            yield f'surrogate-{port}-{label}', raw.replace(b'PLACEHOLDER', value)
        for label, value in [('ascii', 'a'*1025), ('multibyte', 'é'*512+'a')]:
            yield f'over1024-{port}-{label}', encode(dict(base, **{key: value}))
    yield 'both-over1024', encode(dict(zip(KEYS, ('a'*1025, 'a'*1025))))
    yield 'array', b'[]\n'
    yield 'unknown', encode({'if_match': '', 'current_etag': '', 'extra': ''})
    yield 'trailing', encode({'if_match': '', 'current_etag': ''})+b'{}'
    yield 'raw-over16KiB', b' '*16385


def positive_requests():
    # Literal transport anchors, separate from the fresh284 independent-Go cases.
    return [('', '"x"', 0), ('\r\n"x"\t', '"x"', 1),
            ('*garbage', '\x00bad', 1), ('\x00'*1024, '\x00'*1024, 2),
            ('"café 🧪"', '"café 🧪"', 2)]


def inspect_receipt(raw, trusted, supplied, condition, workspace, manifest, source_image, go_version, go_sha, prior=None):
    r = H.load(raw)
    H.need(r['schema']=='gooo/workspace-body-execution-receipt/v1' and r['decision']=='OBSERVED' and not r.get('error'), 'input-only OBSERVED receipt')
    H.need(r['manifest']==str(workspace) and r['manifest_digest']==H.digest(manifest) and r['inputs_digest']==H.digest(trusted) and 'cases_digest' not in r, 'exact manifest/input bytes')
    result = r['result']; program = result['program']; runtime = result['runtime']; image = program['workspace_image']
    H.need(result['schema']=='gooo/workspace-body-execution/v1' and program['entry']['package_path']+':'+program['entry']['activity']==ENTRY, 'exact original entry')
    H.need(image['schema']=='gooo/package-runtime-image/v1' and H.equal(image['init_order'], [PACKAGE]) and len(image['packages'])==1, 'one-source package closure')
    sources = [(p['path'], s['filename'], s['source_digest']) for p in image['packages'] for s in p['sources']]
    H.need(sources==[(PACKAGE, SOURCE, source_image)], 'current exact source-image bytes')
    H.need(H.equal(image['entry'], {'package_path':PACKAGE, 'source':SOURCE, 'activity':ACTIVITY, 'inputs':['Text','Text'], 'output':'IfMatchResult'}), 'image two-Text integer-record ABI')
    H.need(runtime['schema']=='gooo/body-composition-runtime/v1' and runtime['stage']=='COMPLETE' and not runtime.get('failure') and H.equal([runtime['finite_passed'],runtime['finite_total'],runtime['model_calls']], [0,0,0]) and runtime['projection_replayed'] is True and runtime['runtime_replayed'] is True, 'input-only native runtime; no finite expected values')
    H.need(runtime['producer_source_sha']==H.REV and runtime['go_version']==go_version and runtime['go_tool_selection']=='explicit' and runtime['go_tool_sha256']=='sha256:'+go_sha, 'current compiler/explicit Go identities')
    H.need(len(runtime['traces'])==1 and H.equal(runtime['traces'][0]['case_index'],0) and len(runtime['traces'][0]['deliveries'])==1, 'one original input-only delivery')
    d = runtime['traces'][0]['deliveries'][0]
    inputs = [{'port':'input'+str(i), 'entity_id':'gooo://std/text', 'value':s} for i,s in enumerate(supplied)]
    H.need(d['activity_id']==ID and H.equal(d['inputs'],inputs) and H.equal(d['actual'],{'condition':condition}) and 'expected' not in d and 'passed' not in d, 'unchanged typed original inputs/integer actual')
    H.need(H.equal(d['actual_fields'],[{'id':RESULT+'/condition','name':'condition','value':condition}]), 'one ordered integer field ID/name/value/type')
    if prior is not None:
        H.need(r['replayed_from_sha256']==H.digest(prior) and result['replay']['schema']=='gooo/workspace-body-replay/v1' and H.equal(result['replay']['model_calls'],0), 'official saved raw parent')
    return {'receipt_sha256':H.sha(raw), 'typed_fields':1, 'finite_total':0}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ['root','compiler','input-command','go','out']: parser.add_argument('--'+name, required=True)
    args = parser.parse_args()
    root, compiler, helper, go, out = map(H.canonical, [args.root,args.compiler,args.input_command,args.go,args.out])
    H.need(root.is_dir() and out.parent.is_dir() and not out.exists(), 'root/existing parent and fresh output required')
    out.mkdir(); work=out/'owned-work'; work.mkdir()
    pins={}; observations=[]; refusals=[]; native_count=0; refused=0; runner=None; error=None; cleanup=False
    def interrupted(signum, _frame): raise RuntimeError('received signal '+str(signum))
    for sig in (signal.SIGINT,signal.SIGTERM): signal.signal(sig,interrupted)
    # Match the existing host observer: caller Go hooks/flags and model keys
    # cannot introduce an unpinned cache executable or alter the selected build.
    env={key:os.environ[key] for key in ['PATH','HOME','SYSTEMROOT'] if key in os.environ}
    env.update(GOTOOLCHAIN='local',GOWORK='off',GOENV='off',GOPROXY='off',GOSUMDB='off',GOFLAGS='-mod=readonly',CGO_ENABLED='0',GOMAXPROCS='2')
    # Own only transient build/module caches. No shared cache deletion.
    for key,name in [('TMPDIR','tmp'),('GOTMPDIR','gotmp'),('GOCACHE','gocache'),('GOMODCACHE','modules')]:
        (work/name).mkdir();env[key]=str(work/name)
    try:
        names=[SOURCE,MANIFEST,'upstream.json','go.mod','go.sum','cmd/if-match-input/main.go','internal/headerinput/preflight.go','tools/check-if-match-inputs.py','tools/check-header-addition-host.py']
        for path in [compiler,helper,go,*[root/name for name in names]]: H.frozen_read(path,pins)
        H.need(pins[str(root/SOURCE)]['sha256']==SOURCE_SHA, 'current public Gooo source pin')
        manifest=(root/MANIFEST).read_bytes()
        H.need(H.equal(H.load(manifest),{'schema':'gooo/package-workspace-manifest/v1','entry':{'package_path':PACKAGE,'activity':ACTIVITY},'packages':[{'path':PACKAGE,'sources':[SOURCE]}]}), 'exact current workspace')
        lock=H.load((root/'upstream.json').read_bytes())
        H.need(lock['compiler_revision']==H.REV and lock['go_version']=='go1.27.2', 'current upstream pins')
        runner=H.OwnedCommands(out,work,root,pins,env)
        version=H.load(runner.call('compiler-identity',[compiler,'version','--build','--json'],20))
        wanted={'schema':'gooo/build-identity/v1','version':'0.6.32-dev','compiler_source_sha':H.REV,'vcs_revision':H.REV,'source_status':'CLEAN_VCS','vcs_modified':'false','go_version':'go1.27.2','native_go_required':'go1.27.2','build_info_observed':True}
        H.need(all(H.equal(version[k],v) for k,v in wanted.items()), 'clean compiler identity')
        go_version=runner.call('go-identity',[go,'version'],20).decode().strip()
        H.need(H.re.fullmatch(r'go version go1\.27\.2 [a-z0-9_]+/[a-z0-9_]+',go_version), 'actual Go1.27.2 identity')
        negatives=list(negative_requests());H.need(len(negatives)==len({n for n,_ in negatives})==35, 'complete35 strict refusal roster')
        for name,raw in negatives:
            request=out/(name+'.request.json');request.write_bytes(raw)
            before=native_count
            trusted=runner.call(name,[helper],10,stdin_path=request,allowed_exit=(1,))
            diagnostic=(out/name/'stderr').read_bytes()
            H.need(trusted==b'' and b'HEADER_TRANSPORT_' in diagnostic and native_count==before and request.read_bytes()==raw, 'actual prelaunch refusal/stdout-empty/diagnostic/original raw input')
            refusals.append({'name':name,'request_sha256':H.sha(raw),'stdout_bytes':0,'stderr_sha256':H.sha(diagnostic),'controller_Gooo_commands':0})
            refused+=1
        H.need(native_count==0, 'controller Gooo calls for strict negatives must remain zero')
        source_image=H.digest(H.go_json((root/SOURCE).read_text(encoding='utf-8')))
        for i,(selected,current,condition) in enumerate(positive_requests()):
            request=out/f'positive-{i}.request.json';request_raw=encode(dict(zip(KEYS,(selected,current))));request.write_bytes(request_raw)
            trusted=runner.call(f'positive-{i}-preflight',[helper],10,stdin_path=request)
            H.need(request.read_bytes()==request_raw and H.equal(H.load(trusted), {'schema':SCHEMA,'inputs':[{ENTRY+'.input0':selected,ENTRY+'.input1':current}]}), 'trusted decoded Texts must remain exact')
            inputs=out/f'positive-{i}-preflight'/'stdout';prior=None
            for operation in ['execute','replay']:
                label=f'positive-{i}-'+operation
                argv=[compiler,'package',operation,'--json','--go',go,'--inputs',inputs]
                if prior is not None: argv.extend(['--receipt',out/f'positive-{i}-execute'/'stdout'])
                native_count+=1;raw=runner.call(label,[*argv,root/MANIFEST],90)
                checked=inspect_receipt(raw,trusted,(selected,current),condition,root/MANIFEST,manifest,source_image,go_version,pins[str(go)]['sha256'],prior)
                observations.append(dict(request=i,operation=operation,**checked));prior=raw
        H.need(native_count==10 and len(observations)==10 and refused==35, 'exact actual route counts')
        H.unchanged(pins)
    except BaseException as failure: error=str(failure)
    finally:
        if runner is None or runner.closed:
            # Only this invocation-created work directory is removed; raw output remains.
            shutil.rmtree(work);cleanup=True
    status='FIVE_INPUT_ROUTES_AND35_PRELAUNCH_REFUSALS_PASS' if error is None else 'FIRST_FAILURE_PRESERVED'
    summary={'schema':'gooo/if-match-input-route/v1','status':status,'failure':error,'positive_requests':5 if error is None else None,'prelaunch_refusals':refused,'refusals':refusals,'controller_Gooo_commands_for_negatives':0,'actual_Gooo_commands':native_count,'typed_fields_checked':sum(x['typed_fields'] for x in observations),'observations':observations,'source_tool_unchanged':error is None,'processes_closed':runner.closed if runner else True,'owned_work_removed':cleanup,'source_tool_pins':pins,'commands':runner.observations if runner else [],'scope':'Five literal transport anchors with actual input-only OBSERVED execute/official saved replay and35 strict helper refusals before the controller invokes Gooo. Each receipt finite_total/passed0; not284 independent-Go finite cases. No machine-wide no-execution claim, inner canonical hashing, HTTP enforcement, current truth, authorization or effects.'}
    try: H.save(out/'summary.json',summary)
    except BaseException as failure:
        print('write summary.json: '+str(failure)+'; first_failure='+repr(error),file=sys.stderr);return 1
    print(json.dumps(summary));return 0 if error is None else 1


if __name__=='__main__':sys.exit(main())
