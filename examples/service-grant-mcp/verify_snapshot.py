"""Independent stdlib MCP wire checks against the pinned public snapshot CLI.

Exercises the existing Gooo-generated library. No new compiler build or native
Gooo execute/replay receipt, model invocation, collection or tool effect occurs.
"""
import argparse
import datetime
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import signal
import subprocess
import time

MODULE = 'github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison'
VERSION = 'v0.0.0-20261027155614-e4d44fd856ee'
MODULE_SUM = 'h1:ATJzvyeAk2y4MazX9BXoZDYHb94ePod4Ufb2awKzSVE='
GENERATION = '9b54075941cbc495f4f4128999269c7f6d6d32bf2217dd73df46105e266c2e82'
TOOL = 'gooo_compare_saved_tool_definitions'
PROTOCOLS = ('2025-11-25', '2026-07-28')
HERE = Path(__file__).resolve().parent
PENDING_SIGNAL = None


class WireCancelled(BaseException):
    pass


def checkpoint():
    if PENDING_SIGNAL is not None:
        raise WireCancelled('wire client interrupted: '+str(PENDING_SIGNAL))


def load_module(path):
    spec = importlib.util.spec_from_file_location('snapshot_legacy_wire', path)
    result = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(result)
    return result


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def descriptor(path):
    raw = path.read_bytes()
    return {'bytes': len(raw), 'sha256': sha(raw)}


def strict(raw):
    def pairs(items):
        result = {}
        for key, value in items:
            assert key not in result, 'duplicate JSON member'
            result[key] = value
        return result
    return json.loads(raw, object_pairs_hook=pairs,
                      parse_constant=lambda value: (_ for _ in ()).throw(ValueError(value)))


def cases(encode):
    def page(identifier, changed=False):
        tool = {'name': 'search', 'description': 'Find saved documents',
                'inputSchema': {'type': 'object'},
                'outputSchema': {'type': 'number' if changed else 'string'}}
        return {'request_json': encode({'jsonrpc': '2.0', 'id': identifier,
                                       'method': 'tools/list', 'params': {}}).decode(),
                'response_json': encode({'jsonrpc': '2.0', 'id': identifier,
                                        'result': {'tools': [tool]}}).decode()}
    def input(after):
        return encode({'target_name': 'search', 'before': {'pages': [page(1)]},
                       'after': {'pages': after}})
    normal = [('same', input([page(2)]), 'OBSERVED_DEFINITION_SAME'),
              ('output-schema-change', input([page(2, True)]), 'OBSERVED_DEFINITION_CHANGED'),
              ('no-after-pages', input([]), 'UNASSESSED')]
    partial = page(2)
    response = strict(partial['response_json']); response['result']['nextCursor'] = ''
    partial['response_json'] = encode(response).decode()
    normal.append(('selected-row-with-incomplete-pages', input([partial]), 'OBSERVED_DEFINITION_SAME'))
    def change(label, edit, state):
        document = strict(input([page(2)])); edit(document)
        normal.append((label, encode(document), state))
    change('empty-target', lambda d: d.update(target_name=''), 'INVALID_INPUT')
    def rows(document, values, side='after'):
        p = document[side]['pages'][0]; response = strict(p['response_json'])
        response['result']['tools'] = values; p['response_json'] = encode(response).decode()
    original_tool = strict(page(2)['response_json'])['result']['tools'][0]
    change('duplicate-observed-name', lambda d: rows(d, [original_tool, original_tool]), 'AMBIGUOUS_NAME')
    change('complete-observed-removal', lambda d: rows(d, []), 'OBSERVED_NAME_REMOVED')
    change('complete-observed-addition', lambda d: rows(d, [], 'before'), 'OBSERVED_NAME_ADDED')
    change('absent-both', lambda d: d.update(target_name='missing'), 'OBSERVED_NAME_ABSENT')
    def annotation(document):
        p = document['after']['pages'][0]; response = strict(p['response_json'])
        response['result']['tools'][0]['annotations'] = {'readOnlyHint': True}
        p['response_json'] = encode(response).decode()
    change('annotation-claim-added', annotation, 'OBSERVED_ANNOTATIONS_CHANGED')
    def number_spelling(document):
        for side, number in [('before', '1e0'), ('after', '1.0')]:
            p = document[side]['pages'][0]
            p['response_json'] = p['response_json'].replace('"inputSchema":{"type":"object"}',
                '"inputSchema":{"type":"object","minimum":' + number + '}')
    change('number-spelling', number_spelling, 'OBSERVED_DEFINITION_CHANGED')
    base_raw=normal[0][1]
    exact_limit=b'{'+b' '*((1<<20)-len(base_raw))+base_raw[1:]
    assert len(exact_limit)==1<<20
    normal.append(('argument-byte-limit',exact_limit,'OBSERVED_DEFINITION_SAME'))
    malformed = [('duplicate-target', b'{"target_name":"x","target_name":"y","before":{"pages":[]},"after":{"pages":[]}}'),
                 ('null-pages', b'{"target_name":"x","before":{"pages":null},"after":{"pages":[]}}'),
                 ('unknown-field', b'{"target_name":"x","before":{"pages":[]},"after":{"pages":[]},"state":"SAME"}'),
                 ('unpaired-surrogate', b'{"target_name":"\\ud800","before":{"pages":[]},"after":{"pages":[]}}'),
                 ('bad-saved-response', encode({'target_name': 'x', 'before': {'pages': [
                     {'request_json': page(1)['request_json'], 'response_json': '{}'}]}, 'after': {'pages': []}}))]
    malformed.append(('argument-byte-limit-plus-one',b'{'+b' '*((1<<20)+1-len(base_raw))+base_raw[1:]))
    expanded=[]
    for index in range(16):
        current=page(index+10)
        request=strict(current['request_json']);response=strict(current['response_json'])
        if index:request['params']['cursor']=str(index)
        if index<15:response['result']['nextCursor']=str(index+1)
        response['result']['tools'][0]['name']='search' if index==0 else 'search-'+str(index)
        response['result']['tools'][0]['description']='<'*50000
        current['request_json']=encode(request).decode();current['response_json']=encode(response).decode();expanded.append(current)
    oversized_report=input(expanded);assert len(oversized_report)<1<<20
    malformed.append(('report-byte-limit',oversized_report))
    return normal, malformed


def run_reference(out, label, argv, raw=b''):
    checkpoint()
    folder = out / label; folder.mkdir()
    (folder / 'stdin').write_bytes(raw)
    env = dict(os.environ, PATH='', GOPROXY='off', GOSUMDB='off', GOENV='off',
               GOTOOLCHAIN='local', GOWORK='off', GOTELEMETRY='off')
    started = time.monotonic()
    record = {'argv': argv, 'first_failure': None, 'cwd': str(folder)}
    with (folder/'stdin').open('rb') as inp, (folder/'stdout').open('wb') as stdout, (folder/'stderr').open('wb') as stderr:
        child = subprocess.Popen(argv, cwd=folder, env=env, stdin=inp, stdout=stdout,
                                 stderr=stderr, start_new_session=True)
        record['pid'] = child.pid
        try:
            record['exit_code'] = child.wait(timeout=15)
        except BaseException as error:
            record['first_failure'] = repr(error)
            for number in (signal.SIGTERM, signal.SIGKILL):
                try: os.killpg(child.pid, number)
                except ProcessLookupError: break
                try: child.wait(timeout=1)
                except subprocess.TimeoutExpired: pass
            record['exit_code'] = child.wait()
    record.update(elapsed_seconds=time.monotonic()-started, waited=True, output_files_closed=True)
    try: os.killpg(child.pid, 0); record['process_group_ESRCH'] = False
    except ProcessLookupError: record['process_group_ESRCH'] = True
    for name in ('stdin', 'stdout', 'stderr'):
        assert (folder/name).stat().st_size <= (4 << 20) + (64 << 10)
        record[name] = descriptor(folder/name)
    (folder/'process.json').write_text(json.dumps(record, indent=2)+'\n')
    assert record['first_failure'] is None and record['process_group_ESRCH']
    return record, (folder/'stdout').read_bytes(), (folder/'stderr').read_bytes()


def check_report(result, reference, raw, exact):
    assert result.get('isError', False) is False
    exact(result['structuredContent'], reference)
    assert len(result['content']) == 1 and result['content'][0]['type'] == 'text'
    exact(strict(result['content'][0]['text']), reference)
    projection = reference['projection']; decision = reference['decision']
    assert projection['input_json'] == raw.decode() and projection['input_sha256'] == 'sha256:'+sha(raw)
    assert decision['origin_relation'] == decision['authorization_relation'] == 'NOT_ASSESSED'
    assert all(decision[k] is False for k in ['behavior_equivalence_proven', 'execution_authorized', 'live_effect_performed'])


def validate_saved(out, binary, cli, module, go, legacy_path):
    """Read retained output only; never rerun a producer to repair a reader."""
    out, binary, cli, module, go, legacy_path = map(Path, (out, binary, cli, module, go, legacy_path))
    r = strict((out/'acceptance.json').read_bytes()); assert r['first_failure'] is None
    assert r['schema'] == 'gooo/snapshot-MCP-wire-verification/v1' and r['accepted'] is True
    for name, path in [('binary', binary), ('snapshot_cli', cli), ('go', go), ('legacy_verifier', legacy_path)]:
        assert r[name] == descriptor(path)
    assert r['verifier'] == descriptor(Path(__file__))
    assert sha((module/'snapshot-generation.json').read_bytes()) == GENERATION
    normal, malformed=cases(load_module(legacy_path).encoded)
    expected_labels={'server-identity','snapshot-cli-identity'}|{'reference-'+x[0] for x in normal+malformed}
    assert {x['label'] for x in r['reference_processes']}==expected_labels and len(r['reference_processes'])==len(expected_labels)
    for item in r['reference_processes']:
        folder=out/item['label']; actual=strict((folder/'process.json').read_bytes())
        assert actual == item['record'] and actual['waited'] and actual['output_files_closed'] and actual['process_group_ESRCH'] and actual['first_failure'] is None
        label=item['label']
        if label in ['server-identity','snapshot-cli-identity']:
            target=binary if label=='server-identity' else cli
            argv=[str(go),'version','-m','-json',str(target)];raw=b'';exit_code=0
        else:
            name=label.removeprefix('reference-')
            raw=next(x[1] for x in normal+malformed if x[0]==name)
            argv=[str(cli),'--definition-snapshot'];exit_code=1 if any(x[0]==name for x in malformed) else 0
        assert actual['argv']==argv and actual['cwd']==str(folder) and actual['exit_code']==exit_code
        assert (folder/'stdin').read_bytes()==raw
        for stream in ('stdin','stdout','stderr'): assert descriptor(folder/stream) == actual[stream]
    legacy=load_module(legacy_path); normal, malformed=cases(legacy.encoded)
    expected={name: strict((out/('reference-'+name)/'stdout').read_bytes()) for name,_,_ in normal}
    diagnostics={name: strict((out/('reference-'+name)/'stderr').read_bytes()) for name,_ in malformed}
    for name,_,state in normal:
        assert expected[name]['decision']['state']==state and not (out/('reference-'+name)/'stderr').read_bytes()
    for name,_ in malformed:assert not (out/('reference-'+name)/'stdout').read_bytes()
    for label in ['server-identity','snapshot-cli-identity']:
        assert not (out/label/'stderr').read_bytes()
        build=strict((out/label/'stdout').read_bytes());assert build['GoVersion']=='go1.27.3'
        dependency=build['Main'] if label=='snapshot-cli-identity' else next(d for d in build['Deps'] if d['Path']==MODULE)
        assert dependency=={'Path':MODULE,'Version':VERSION,'Sum':MODULE_SUM}
        assert 'Replace' not in build['Main'] and all('Replace' not in d for d in build.get('Deps') or [])
    assert expected['selected-row-with-incomplete-pages']['decision']['after_page_coverage']=='INCOMPLETE'
    assert diagnostics['report-byte-limit']['code']=='OUTPUT_LIMIT_EXCEEDED'
    legacy.exact(r['schemas']['output'],strict((legacy_path.parent/'schemas/snapshot-output.schema.json').read_bytes()))
    expected_sessions={'wire-'+protocol+'-'+str(bits) for protocol in PROTOCOLS for bits in range(8)}
    assert {x['label'] for x in r['sessions']}==expected_sessions and len(r['sessions'])==len(expected_sessions)
    for session in r['sessions']:
        folder=out/session['label']; process=strict((folder/'process.json').read_bytes())
        assert process['failure'] is None and process['exit_code']==0 and process['waited'] and process['streams_EOF'] and process['process_group_ESRCH']
        assert r['executed_cases']=={'normal':[x[0] for x in normal],'malformed':[x[0] for x in malformed]}
        assert process['runtime_PATH_empty'] and process['runtime_GOROOT_absent'] and process['server_created_work_files']==[]
        for stream in ('stdin','stdout','stderr'): assert descriptor(folder/stream)==session['streams'][stream]
        response_lines=(folder/'stdout').read_bytes().splitlines(); assert all(len(x)+1<=(16<<20)+(64<<10) for x in response_lines)
        responses={row['id']:row for row in map(strict,response_lines) if 'id' in row}
        bits=int(session['label'].rsplit('-',1)[1]); assert session['protocol'] in PROTOCOLS
        flags=[flag for bit,flag in [(1,'--many-required'),(2,'--alias-origin-change'),(4,'--definition-snapshot')] if bits&bit]
        assert session['argv']==[str(binary),*flags] and session['snapshot_enabled']==bool(bits&4)
        requests={json.loads(line)['id']:line for line in (folder/'stdin').read_bytes().splitlines() if 'id' in json.loads(line)}
        def bound_request(identifier, raw):
            request=requests[identifier]; obj=json.loads(request)
            assert obj['method']=='tools/call' and obj['params']['name']==TOOL
            assert b'"arguments":'+raw+b'}' in request
        lists=[responses[k]['result']['tools'] for k,v in requests.items() if json.loads(v)['method']=='tools/list']; assert len(lists)==1
        tools={x['name']:x for x in lists[0]}; assert len(tools)==len(lists[0])
        want={'assess_service_grant'}
        if bits&1:want.add('assess_service_grant_change')
        if bits&2:want.add('gooo_compare_alias_origin')
        if bits&4:want.add(TOOL)
        assert set(tools)==want
        if bits&4:
            legacy.exact({'input':tools[TOOL]['inputSchema'],'output':tools[TOOL]['outputSchema']},r['schemas'])
        else:
            unavailable=[k for k,v in requests.items() if json.loads(v)['method']=='tools/call' and json.loads(v)['params']['name']==TOOL];assert len(unavailable)==1
            assert responses[unavailable[0]]['error']['code']==-32602
        assert [x['case'] for x in session['normal_calls']]==([x[0] for x in normal] if session['snapshot_enabled'] else [])
        assert [x['case'] for x in session['refused_calls']]==([x[0] for x in malformed] if session['snapshot_enabled'] else [])
        for observed in session['normal_calls']:
            name=observed['case']; raw=next(x[1] for x in normal if x[0]==name)
            bound_request(observed['id'],raw)
            check_report(responses[observed['id']]['result'],expected[name],raw,legacy.exact)
        for observed in session['refused_calls']:
            bound_request(observed['id'],next(x[1] for x in malformed if x[0]==observed['case']))
            response=responses[observed['id']]
            if 'error' in response:
                assert response['error']['code']==-32602 and 'result' not in response
            else:
                result=response['result']; assert result.get('isError') is True and 'structuredContent' not in result
                legacy.exact(strict(result['content'][0]['text']),diagnostics[observed['case']])
    assert len(r['sessions'])==16 and sum(x['snapshot_enabled'] for x in r['sessions'])==8
    return r


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    for name in ['binary','snapshot-cli','snapshot-module','go','out']:
        parser.add_argument('--'+name,type=Path,required=True)
    parser.add_argument('--legacy-verifier',type=Path,default=HERE/'verify.py')
    args=parser.parse_args()
    args.binary=args.binary.resolve(); args.snapshot_cli=args.snapshot_cli.resolve(); args.go=args.go.resolve()
    args.snapshot_module=args.snapshot_module.resolve(); args.legacy_verifier=args.legacy_verifier.resolve()
    args.out.mkdir(parents=True,exist_ok=True); assert not any(args.out.iterdir())
    report={'schema':'gooo/snapshot-MCP-wire-verification/v1','at_utc':datetime.datetime.now(datetime.timezone.utc).isoformat(),
            'accepted':False,'first_failure':None,'reference_processes':[],'sessions':[],
            'new_native_Gooo_receipts':False,'new_compiler_builds':0,'model_or_remote_tool_calls':0}
    for name,path in [('binary',args.binary),('snapshot_cli',args.snapshot_cli),('go',args.go),('legacy_verifier',args.legacy_verifier),('verifier',Path(__file__))]:report[name]=descriptor(path)
    old_handlers={}
    def interrupted(number,_frame):
        global PENDING_SIGNAL
        if PENDING_SIGNAL is None:PENDING_SIGNAL=number
    for number in [signal.SIGTERM,signal.SIGINT]:
        old_handlers[number]=signal.getsignal(number);signal.signal(number,interrupted)
    legacy=load_module(args.legacy_verifier); legacy.BINARY=args.binary; legacy.OUTPUT=args.out
    try:
        assert sha((args.snapshot_module/'snapshot-generation.json').read_bytes())==GENERATION
        for name,binary in [('server',args.binary),('snapshot-cli',args.snapshot_cli)]:
            rec,stdout,stderr=run_reference(args.out,name+'-identity',[str(args.go),'version','-m','-json',str(binary)])
            report['reference_processes'].append({'label':name+'-identity','record':rec})
            identity=strict(stdout); assert rec['exit_code']==0 and not stderr and identity['GoVersion']=='go1.27.3'
            module=identity['Main'] if name=='snapshot-cli' else next(x for x in identity['Deps'] if x['Path']==MODULE)
            assert module['Path']==MODULE and module['Version']==VERSION and module['Sum']==MODULE_SUM
            assert 'Replace' not in identity['Main'] and all('Replace' not in x for x in identity.get('Deps',[]))
        normal,malformed=cases(legacy.encoded); expected={}; diagnostics={}
        report['executed_cases']={'normal':[x[0] for x in normal],'malformed':[x[0] for x in malformed]}
        for name,raw,state in normal:
            rec,stdout,stderr=run_reference(args.out,'reference-'+name,[str(args.snapshot_cli),'--definition-snapshot'],raw)
            report['reference_processes'].append({'label':'reference-'+name,'record':rec}); assert rec['exit_code']==0 and not stderr
            value=strict(stdout); assert value['decision']['state']==state; expected[name]=value
        assert expected['selected-row-with-incomplete-pages']['decision']['after_page_coverage']=='INCOMPLETE'
        for name,raw in malformed:
            rec,stdout,stderr=run_reference(args.out,'reference-'+name,[str(args.snapshot_cli),'--definition-snapshot'],raw)
            report['reference_processes'].append({'label':'reference-'+name,'record':rec}); assert rec['exit_code']==1 and not stdout
            diagnostics[name]=strict(stderr)
            if name=='report-byte-limit':assert diagnostics[name]['code']=='OUTPUT_LIMIT_EXCEEDED'
        schemas=None
        for protocol in PROTOCOLS:
            for bits in range(8):
                checkpoint()
                flags=[flag for bit,flag in [(1,'--many-required'),(2,'--alias-origin-change'),(4,'--definition-snapshot')] if bits&bit]
                label='wire-'+protocol+'-'+str(bits); session=legacy.Session(label,protocol,flags,stream_limit=32<<20); failure=None
                item={'label':label,'protocol':protocol,'argv':[str(args.binary),*flags],'snapshot_enabled':bool(bits&4),'normal_calls':[],'refused_calls':[]}
                try:
                    if protocol=='2025-11-25':
                        session.request('initialize',{'protocolVersion':protocol,'clientInfo':{'name':'snapshot-wire','version':'0'},'capabilities':{}});session.notify_initialized()
                    else: session.request('server/discover')
                    listing=session.request('tools/list')['result']['tools']; tools={x['name']:x for x in listing}
                    want={'assess_service_grant'}
                    if bits&1:want.add('assess_service_grant_change')
                    if bits&2:want.add('gooo_compare_alias_origin')
                    if bits&4:want.add(TOOL)
                    assert set(tools)==want
                    if not bits&4:
                        denied=session.request('tools/call',{'name':TOOL},normal[0][1]); assert denied['error']['code']==-32602
                    else:
                        tool=tools[TOOL]; assert tool['annotations']['readOnlyHint'] is True and tool['annotations']['idempotentHint'] is True and tool['annotations']['openWorldHint'] is False
                        assert tool['inputSchema']['additionalProperties'] is False and set(tool['inputSchema']['required'])=={'target_name','before','after'}
                        assert tool['outputSchema']['additionalProperties'] is False and set(tool['outputSchema']['required'])=={'schema','projection','inputs','decision','bindings'}
                        current={'input':tool['inputSchema'],'output':tool['outputSchema']}
                        if schemas is None:schemas=current
                        else:legacy.exact(current,schemas)
                        for name,raw,_ in normal:
                            checkpoint()
                            response=session.request('tools/call',{'name':TOOL},raw); check_report(response['result'],expected[name],raw,legacy.exact)
                            item['normal_calls'].append({'case':name,'id':response['id']})
                        for name,raw in malformed:
                            checkpoint()
                            response=session.request('tools/call',{'name':TOOL},raw)
                            if 'error' in response: assert response['error']['code']==-32602 and 'result' not in response
                            else:
                                result=response['result']; assert result.get('isError') is True and 'structuredContent' not in result
                                legacy.exact(strict(result['content'][0]['text']),diagnostics[name])
                            item['refused_calls'].append({'case':name,'id':response['id']})
                except BaseException as error:failure=repr(error);raise
                finally:
                    session.close(failure)
                    item['streams']={name:descriptor(args.out/label/name) for name in ['stdin','stdout','stderr']};report['sessions'].append(item)
        checkpoint()
        report.update(accepted=True,status='INTERNAL_SNAPSHOT_MCP_TWO_PROTOCOLS_ALL_EIGHT_PROFILES_PASS',schemas=schemas,module={'path':MODULE,'version':VERSION,'sum':MODULE_SUM,'generation_sha256':GENERATION})
    except BaseException as error:report.update(first_failure={'type':type(error).__name__,'message':str(error)},status='NOT_ACCEPTED')
    (args.out/'acceptance.json').write_text(json.dumps(report,indent=2)+'\n')
    if report['accepted']:
        try: validate_saved(args.out,args.binary,args.snapshot_cli,args.snapshot_module,args.go,args.legacy_verifier)
        except BaseException as error:
            (args.out/'initial-producer-summary.json').write_text(json.dumps(report,indent=2)+'\n')
            report.update(accepted=False,status='SAVED_REVIEW_REFUSED',first_failure={'type':type(error).__name__,'message':str(error),'stage':'saved_review'})
            (args.out/'acceptance.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps({'accepted':report['accepted'],'status':report['status'],'sessions':len(report['sessions']),'first_failure':report['first_failure']}))
    for number,handler in old_handlers.items():signal.signal(number,handler)
    return 0 if report['accepted'] else 2

if __name__=='__main__':raise SystemExit(main())
