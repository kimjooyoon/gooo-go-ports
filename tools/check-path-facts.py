"""Run the two distinct fresh path verification layers; no import-time launch.

Actual manual24 CLI execute/replay is separate from full9841 generated-function
value comparisons. No historical receipt or checked-in generated candidate is an
execution input. Private owned work is removed only after all groups are gone.
"""
from pathlib import Path
from decimal import Decimal
import argparse
import hashlib
import json
import os
import shutil
import signal
import stat
import subprocess
import sys
import tempfile
import time

ENTRIES = [('ext','go/path:Ext','path-ext.workspace.json'),
           ('isabs','go/path:IsAbs','path-isabs.workspace.json'),
           ('report','app/path-facts:Report','path-facts.workspace.json')]
WORK_CAP, PRE_CAP, FREE_MIN = 384<<20, 320<<20, 5<<30
STDOUT_CAP, STDERR_CAP, ROWS_CAP, PROOF_CAP = 8<<20, 1<<20, 8<<20, 16<<20
SOURCE_NAMES = ['LICENSE','go.mod','upstream.json','internal/jsonkeys/keys.go',
 'ports/path/path.gooo','examples/path-facts/main.gooo','examples/path-facts/inputs.json',
 'examples/path-facts/README.md','third_party/GO-LICENSE','third_party/go/path.go.txt',
 'path-ext.workspace.json','path-isabs.workspace.json','path-facts.workspace.json',
 'cmd/check-path-facts/main.go','cmd/check-path-facts/inspect.go',
 'cmd/check-path-facts/result_digest.go','cmd/check-path-facts/platform.go',
 'cmd/check-path-facts/result-profile-83928fa.json',
 'cmd/check-path-facts/generated_binding.go','cmd/check-path-facts/exhaustive.go',
 'cmd/check-path-facts/exhaustive_test.go.txt','cmd/check-path-facts/generated_binding_test.go',
 'cmd/check-path-facts/testdata/manual-contract.json',
 'cmd/path-facts-oracle/main.go','cmd/path-facts-oracle/main_test.go',
 'tools/check-path-facts.py','tools/check-path-facts_test.py']


def ref(path):
    raw = path.read_bytes()
    return {'path':str(path),'bytes':len(raw),'sha256':hashlib.sha256(raw).hexdigest()}


def save_new(path,value):
    with path.open('x') as f:
        json.dump(value,f,indent=2)
        f.write('\n')


def logical(root,active=False):
    # Only live owned-work descendants may disappear while Go removes temps.
    # A missing root, permission failure, symlink, or stable scan still fails.
    root=Path(root)
    root_info=root.lstat()
    if not stat.S_ISDIR(root_info.st_mode) or stat.S_ISLNK(root_info.st_mode):
        raise RuntimeError('Unexpected owned root kind')
    def walk_error(exc):
        missing=Path(exc.filename) if exc.filename is not None else root
        if active and isinstance(exc,FileNotFoundError) and missing!=root and root in missing.parents:return
        raise exc
    total=0
    for parent,dirs,files in os.walk(root,followlinks=False,onerror=walk_error):
        for name in dirs+files:
            try:info=(Path(parent)/name).lstat()
            except FileNotFoundError:
                if active:continue
                raise
            if stat.S_ISLNK(info.st_mode):raise RuntimeError('Unexpected owned path symlink')
            if stat.S_ISREG(info.st_mode):total+=info.st_size
    final_root=root.lstat()
    if not stat.S_ISDIR(final_root.st_mode) or (final_root.st_dev,final_root.st_ino)!=(root_info.st_dev,root_info.st_ino):
        raise RuntimeError('Owned root changed during scan')
    return total


def strict_object(pairs):
    out={}
    for key,value in pairs:
        if key in out:raise ValueError('Duplicate JSON field')
        out[key]=value
    return out


def load(path):
    return json.loads(path.read_bytes().decode('utf-8'),object_pairs_hook=strict_object,
                      parse_float=lambda _: (_ for _ in ()).throw(ValueError('Noninteger data')),
                      parse_constant=lambda _: (_ for _ in ()).throw(ValueError('Nonfinite data')))


def exact(value,want):
    return type(value) is int and value==want


def load_process(path):
    value=json.loads(path.read_bytes().decode('utf-8'),object_pairs_hook=strict_object,parse_float=Decimal,
                     parse_constant=lambda _: (_ for _ in ()).throw(ValueError('Nonfinite process diagnostic')))
    if type(value) is not dict:raise ValueError('Process ledger must be an object')
    def integers_only(item):
        if type(item) is Decimal:raise ValueError('Decimal outside process timing diagnostics')
        if type(item) is dict:
            for child in item.values():integers_only(child)
        elif type(item) is list:
            for child in item:integers_only(child)
    for key,item in value.items():
        if key in ('elapsed_seconds','timeout_seconds','test_timeout_seconds'):
            if type(item) not in (int,Decimal) or (type(item) is Decimal and not item.is_finite()) or item<0:raise ValueError('Invalid process timing diagnostic')
        else:integers_only(item)
    return value


def validate_native_process(process):
    if not exact(process.get('exit_code'),0) or process.get('process_reaped') is not True or process.get('context_error') is not None or process.get('storage_or_output_error') is not None:
        raise ValueError('Retained native process was not successful/reaped')


def error_detail(exc):
    value={'type':type(exc).__name__,'repr':repr(exc)}
    if type(getattr(exc,'errno',None)) is int:value['errno']=exc.errno
    return value


def group_exists(pgid):
    try:os.killpg(pgid,0);return True
    except ProcessLookupError:return False


def group_state(pgid):
    try:return {'state':'present' if group_exists(pgid) else 'absent','error':None}
    except BaseException as exc:return {'state':'unknown','error':error_detail(exc)}


def terminate(child,wait_seconds):
    # Permission failure is unknown ownership, never proof of absence. Reaping
    # the direct child is attempted separately even if signalling is denied.
    errors=[];deadline=time.monotonic()+max(0,wait_seconds);before=group_state(child.pid)
    if before['state']=='unknown':errors.append({'operation':'group-probe-before','error':before['error']})
    try:running=child.poll() is None
    except BaseException as exc:
        running=True;errors.append({'operation':'poll','error':error_detail(exc)})
    if running or before['state']!='absent':
        for sig,delay in ((signal.SIGTERM,0.2),(signal.SIGKILL,0)):
            try:os.killpg(child.pid,sig)
            except ProcessLookupError:break
            except BaseException as exc:errors.append({'operation':'signal','signal':int(sig),'error':error_detail(exc)})
            if delay:
                try:time.sleep(min(delay,max(0,deadline-time.monotonic())))
                except BaseException as exc:errors.append({'operation':'signal-delay','error':error_detail(exc)})
    code=None;reaped=False
    try:code=child.wait(timeout=max(0,deadline-time.monotonic()));reaped=True
    except BaseException as exc:errors.append({'operation':'wait','error':error_detail(exc)})
    after=group_state(child.pid)
    if after['state']=='unknown':errors.append({'operation':'group-probe-after','error':after['error']})
    return {'exit_code':code,'process_reaped':reaped,'owned_group_before':before,
            'owned_group_state':after,'owned_group_remaining':False if after['state']=='absent' else True if after['state']=='present' else None,
            'termination_errors':errors}


def checked_entry(value,entry):
    if value.get('schema')!='gooo/path-facts-checked/v1' or value.get('decision')!='MANUAL24_CLI_REPLAY_AND_FULL9841_GENERATED_NATIVE_PASS' or value.get('entry')!=entry:
        raise ValueError('Wrong path verification identity')
    for key in ('native_observed_cases','saved_replay_observed_cases','independent_go_oracle_cases'):
        if not exact(value.get(key),24):raise ValueError('Wrong manual CLI count')
    for key,want in [('full_generated_native_outputs',9841),('full_CLI_outputs',0),('full_saved_replay_outputs',0),('native_receipt_expectation_cases',0)]:
        if not exact(value.get(key),want):raise ValueError('Wrong evidence-layer count')
    full=value['full_generated_corpus']
    for key,want in [('input_count',9841),('comparison_count',9841),('scalar_comparison_count',59046 if entry=='report' else 9841),('mismatches',0)]:
        if not exact(full.get(key),want):raise ValueError('Wrong generated-native count')
    if full.get('generated_sha256')!=value.get('generated_sha256'):raise ValueError('Generated source link differs')
    return full


def value_span(raw, path):
    text = raw.decode('utf-8')
    decoder = json.JSONDecoder()

    def whitespace(position):
        while position < len(text) and text[position].isspace():
            position += 1
        return position

    def locate(position, remaining):
        position = whitespace(position)
        if not remaining:
            _, end = decoder.raw_decode(text, position)
            return position, end
        wanted, rest = remaining[0], remaining[1:]
        opening = text[position]
        position, index = whitespace(position + 1), 0
        while text[position] not in '}]':
            if opening == '{':
                key, end = decoder.raw_decode(text, position)
                position = whitespace(end)
                assert text[position] == ':'
                position = whitespace(position + 1)
            else:
                assert opening == '['
                key = index
            if key == wanted:
                return locate(position, rest)
            _, position = decoder.raw_decode(text, position)
            position = whitespace(position)
            if text[position] == ',':
                position = whitespace(position + 1)
            index += 1
        raise ValueError('Missing mutation path')

    return text, locate(0, path)


def changed_value(raw, path, replacement):
    text, (first, last) = value_span(raw, path)
    result = (text[:first] + json.dumps(replacement) + text[last:]).encode('utf-8')
    assert json.loads(result) != json.loads(raw)
    return result


def wrong_pure_calls(raw, replacement):
    prior = json.loads(raw)
    program = prior['result']['program']
    if 'pure_calls' in program:
        return changed_value(raw, ['result', 'program', 'pure_calls'], replacement)
    text, (first, last) = value_span(raw, ['result', 'program'])
    assert text[first] == '{' and text[last - 1] == '}'
    comma = ',' if program else ''
    inserted = comma + '"pure_calls":' + json.dumps(replacement)
    result = (text[:last - 1] + inserted + text[last - 1:]).encode('utf-8')
    changed = json.loads(result)
    assert changed['result']['program']['pure_calls'] == replacement
    del changed['result']['program']['pure_calls']
    assert changed == prior
    return result




class Attempt:
    def __init__(self,args):
        self.repo=args.repository.resolve();self.out=args.out.resolve()
        self.go=args.go.resolve();self.compiler=args.compiler.resolve()
        self.overall=args.timeout;self.started=time.monotonic();self.stages=[]
        self.work=None;self.owner=None;self.owns_out=False;self.groups=set();self.children={};self.reaped=set();self.oracle_ref=None
        self.secondary_errors=[];self.pending_process=None;self.last_group_observations=[];self.work_cleanup_complete=False
        if not 1<=self.overall<=900:raise ValueError('Timeout must be1..900seconds')
        if self.out.exists():raise RuntimeError('Refuse existing proof output directory')
        names=SOURCE_NAMES+[str(p.relative_to(self.repo)) for p in sorted((self.repo/'cmd/path-facts-oracle/testdata/negative').glob('*.json'))]
        if len(names)-len(SOURCE_NAMES)!=15:raise ValueError('Require exactly15 frozen transport fixtures')
        self.sources=[ref(self.repo/name) for name in names]
        self.tools=[ref(self.go),ref(self.compiler),ref(Path(sys.executable).resolve())]
        for name,want in [('src/path/path.go','0a5024c364387eeea9ab5fe0b285f0d62355e8cd181cdbddfdfa227f6916d94c'),
                          ('src/unicode/utf8/utf8.go','3c145ee25fd7631ebbc670fd1236e8c163ca30ae8913c796489efbf3f267ad54')]:
            observed=ref(self.go.parent.parent/name)
            if observed['sha256']!=want:raise ValueError('Pinned original Go source differs')
            self.tools.append(observed)

    def prepare(self):
        # Assignment of Attempt precedes all owned resource creation, so every
        # failure after this point can be cleaned by main's finally block.
        self.out.mkdir(parents=True)
        self.owns_out=True
        self.work=Path(tempfile.mkdtemp(prefix='gooo-path-native-check-')).resolve()
        self.owner=self.work.stat()
        for name in ('cache','gopath','module-cache','tmp','bin'):(self.work/name).mkdir()
        save_new(self.out/'source-freeze.json',{'schema':'gooo/path-CI-source-freeze/v1','files':self.sources,'tools':self.tools,'overall_seconds':self.overall})
        self.freeze=ref(self.out/'source-freeze.json')
        self.env=dict(os.environ)
        for key in list(self.env):
            if key.startswith(('GOOO_','LAYA_')) or key.endswith(('_API_KEY','_TOKEN')) or key in ('GOCACHEPROG','LD_PRELOAD','DYLD_INSERT_LIBRARIES'):
                self.env.pop(key,None)
        self.env.update(GOCACHE=str(self.work/'cache'),GOPATH=str(self.work/'gopath'),GOMODCACHE=str(self.work/'module-cache'),
             GOTMPDIR=str(self.work/'tmp'),TMPDIR=str(self.work/'tmp'),GOROOT=str(self.go.parent.parent),GOTOOLCHAIN='local',GOWORK='off',
             GOENV='off',GOFLAGS='',GOPROXY='off',GOSUMDB='off',CGO_ENABLED='0',GOTELEMETRY='off',
             PATH=str(self.go.parent)+':/usr/bin:/bin:/usr/sbin:/sbin',PYTHONDONTWRITEBYTECODE='1')
        self.env.pop('GOOS',None);self.env.pop('GOARCH',None)

    def frozen(self):
        extra=[self.oracle_ref] if self.oracle_ref is not None else []
        for expected in self.sources+self.tools+[self.freeze]+extra:
            if ref(Path(expected['path']))!=expected:raise RuntimeError('Frozen source or tool changed')

    def budget(self,active=False):
        if time.monotonic()-self.started>=self.overall:raise RuntimeError('Overall path verification timeout')
        if logical(self.work,active=active)>WORK_CAP:raise RuntimeError('Owned work exceeds384MiB')
        rows=sum(p.stat().st_size for p in self.out.rglob('rows.jsonl'))
        if rows>ROWS_CAP or logical(self.out)-rows>PROOF_CAP:raise RuntimeError('Retained path proof cap')
        if shutil.disk_usage(self.work).free<FREE_MIN:raise RuntimeError('Require5GiB free')

    def invoke(self,name,argv,seconds=120,expected=0,diagnostic=None):
        self.frozen();self.budget()
        if logical(self.work)>=PRE_CAP:raise RuntimeError('Pre-stage320MiB hold')
        stdout=self.out/(name+'.stdout');stderr=self.out/(name+'.stderr')
        row={'name':name,'argv':list(map(str,argv)),'cwd':str(self.repo),'source_freeze_before':self.freeze,
             'timeout_seconds':min(seconds,self.overall-(time.monotonic()-self.started)),'automatic_retries':0,
             'expected_exit_code':expected,'expected_refusal_diagnostic':diagnostic}
        child=None;begin=time.monotonic();primary=None;secondary=[]
        try:
            with stdout.open('xb') as so,stderr.open('xb') as se:
                child=subprocess.Popen(argv,cwd=self.repo,env=self.env,stdout=so,stderr=se,start_new_session=True)
                self.groups.add(child.pid);self.children[child.pid]=child;self.reaped.discard(child.pid)
                row.update(pid=child.pid,owned_pgid=child.pid,status='STARTED')
                self.pending_process=row
                started=self.out/(name+'.started.json')
                save_new(started,row)
                row['started_ledger']=ref(started)
                while child.poll() is None:
                    self.budget(active=True)
                    if stdout.stat().st_size>STDOUT_CAP or stderr.stat().st_size>STDERR_CAP:raise RuntimeError('Child output cap')
                    if time.monotonic()-begin>=row['timeout_seconds']:raise RuntimeError('Child timeout')
                    time.sleep(0.1)
        except BaseException as exc:primary=exc
        finally:
            if child is not None:
                outcome=terminate(child,min(2,max(0,self.overall-(time.monotonic()-self.started))))
                row.update(outcome)
                if outcome['process_reaped']:self.reaped.add(child.pid)
                secondary.extend(outcome['termination_errors'])
            else:row.update(exit_code=None,process_reaped=False,owned_group_remaining=None,owned_group_state={'state':'unknown','error':None})
            row.update(elapsed_seconds=time.monotonic()-begin,source_freeze_after=self.freeze)
            for key,path in (('stdout',stdout),('stderr',stderr)):
                try:row[key]=ref(path) if path.exists() else None
                except BaseException as exc:
                    row[key]=None;secondary.append({'operation':key+'-capture','error':error_detail(exc)})
            try:self.frozen();row['source_tools_unchanged_after']=True
            except BaseException as exc:
                row['source_tools_unchanged_after']=False;secondary.append({'operation':'source-postcheck','error':error_detail(exc)})
            if primary is None:
                if secondary or row.get('owned_group_remaining') is not False or row.get('process_reaped') is not True:
                    primary=RuntimeError('Stage completion or ownership unconfirmed: '+name)
                elif type(row.get('exit_code')) is not int or row['exit_code']!=expected:
                    primary=RuntimeError('Unexpected process result: '+name)
                elif row['stdout']['bytes']>STDOUT_CAP or row['stderr']['bytes']>STDERR_CAP:
                    primary=RuntimeError('Terminal output cap')
                elif diagnostic is not None:
                    try:
                        if row['stdout']['bytes']!=0 or stderr.read_text().strip()!=diagnostic:raise RuntimeError('Wrong refusal diagnostic: '+name)
                    except BaseException as exc:primary=exc
            row.update(status='FAILED' if primary is not None else 'COMPLETED',abort=repr(primary) if primary is not None else None,
                       primary_error=error_detail(primary) if primary is not None else None,secondary_errors=secondary)
            self.pending_process=row
            try:
                ledger=self.out/(name+'.process.json');save_new(ledger,row);self.stages.append(ref(ledger));self.pending_process=None
            except BaseException as exc:
                secondary.append({'operation':'terminal-ledger-write','error':error_detail(exc)})
                self.secondary_errors.extend(secondary)
                if primary is None:primary=exc
                row.update(status='FAILED',abort=repr(primary),primary_error=error_detail(primary),secondary_errors=secondary)
        if primary is not None:raise primary.with_traceback(primary.__traceback__)
        self.frozen();self.budget()
        return stdout

    def require_groups_gone(self):
        observations=[{'pgid':pgid,'direct_child_reaped':pgid in getattr(self,'reaped',set()),**group_state(pgid)} for pgid in sorted(self.groups)]
        self.last_group_observations=observations
        if any(row['state']!='absent' or row['direct_child_reaped'] is not True for row in observations):
            raise RuntimeError('Owned descendants remain, ownership unknown, or child unreaped; refuse work cleanup')

    def arguments(self,entry,inputs,execution,replay,fullout):
        return [str(self.work/'bin/checker'),'--entry',entry,'--snapshot',str(self.repo),'--inputs',str(inputs),
            '--execution',str(execution),'--replay',str(replay),'--compiler-build',str(self.out/'compiler-build.stdout'),
            '--oracle',str(self.out/'manual-original-go.json'),'--manual',str(self.repo/'cmd/check-path-facts/testdata/manual-contract.json'),
            '--full-out',str(fullout),'--out',str(self.out/(fullout.name+'-checked.json'))]

    def cleanup_native_work(self,entry,full,checked):
        owned=full['owned_native_work'];path=Path(owned['path'])
        base=self.work/'tmp'
        if not path.is_absolute() or path.resolve()!=path or path.parent!=base or not path.name.startswith('gooo-path-native-'):
            raise ValueError('Native work must be a direct owned TMPDIR child')
        if full.get('owned_native_work_removed') is not False:raise ValueError('Child must leave native work to outer cleanup')
        info=path.lstat()
        if not stat.S_ISDIR(info.st_mode) or path.is_symlink() or type(owned.get('device')) is not int or type(owned.get('inode')) is not int:
            raise ValueError('Native work ownership kind differs')
        if (info.st_dev,info.st_ino)!=(owned['device'],owned['inode']):raise ValueError('Native work identity changed')
        self.require_groups_gone()
        size=logical(path);shutil.rmtree(path)
        ledger=self.out/(entry+'-native-work-cleanup.json')
        save_new(ledger,{'owned_path':str(path),'device':info.st_dev,'inode':info.st_ino,'logical_bytes_removed':size,
             'checker_checked_report':ref(checked),'all_observed_owned_groups_gone':True,'removed':True})
        return ref(ledger)

    def audit_retained(self,records):
        seen_rows=set()
        def bound(expected):
            observed=ref(Path(expected['path']))
            want=expected['sha256']
            if type(expected.get('bytes')) is not int or observed['bytes']!=expected['bytes'] or observed['sha256']!=want.removeprefix('sha256:'):
                raise ValueError('Retained proof changed')
        bound(self.oracle_ref)
        for ledger in self.stages:
            bound(ledger);process=load_process(Path(ledger['path']))
            expected=process.get('expected_exit_code')
            if type(expected) is not int or expected not in (0,1) or not exact(process.get('exit_code'),expected) or process.get('process_reaped') is not True or process.get('owned_group_remaining') is not False or process.get('abort') is not None or process.get('source_tools_unchanged_after') is not True:
                raise ValueError('Retained stage result/ownership differs')
            if process.get('source_freeze_before')!=self.freeze or process.get('source_freeze_after')!=self.freeze:raise ValueError('Stage source freeze differs')
            if process.get('status')!='COMPLETED' or process.get('primary_error') is not None or process.get('secondary_errors')!=[] or process.get('termination_errors')!=[] or process.get('owned_group_state')!={'state':'absent','error':None}:raise ValueError('Stage completion diagnostics differ')
            started_ref=process['started_ledger'];bound(started_ref);started=load_process(Path(started_ref['path']))
            if started.get('status')!='STARTED' or any(started.get(k)!=process.get(k) for k in ('name','pid','owned_pgid','argv','cwd','source_freeze_before','timeout_seconds','expected_exit_code')):raise ValueError('Started process identity differs')
            if started.get('source_freeze_before')!=self.freeze or type(started.get('pid')) is not int or started['pid']<=0 or started.get('owned_pgid')!=started['pid']:raise ValueError('Started process ownership differs')
            for key,cap in (('stdout',STDOUT_CAP),('stderr',STDERR_CAP)):
                bound(process[key])
                if process[key]['bytes']>cap:raise ValueError('Retained stage output exceeds cap')
            diagnostic=process.get('expected_refusal_diagnostic')
            if diagnostic is not None and (process['stdout']['bytes']!=0 or Path(process['stderr']['path']).read_text().strip()!=diagnostic):raise ValueError('Retained refusal diagnostic differs')
        for record in records:
            for key in ('checked','native_work_cleanup','execute','saved_replay','inputs'):bound(record[key])
            full=record['full_generated']
            for key in ('rows_file','generated_file','harness_file','process_file','native_report_file','go_path_source_file','go_tool_file'):bound(full[key])
            if full['generated_file']['sha256']!=full['generated_sha256'] or full['harness_file']['sha256']!=full['harness_sha256'] or full['rows_file']['sha256']!=full['rows_sha256']:
                raise ValueError('Retained source/row semantic link differs')
            rows_path=Path(full['rows_path'])
            if rows_path!=self.out/(record['entry']+'-full')/'rows.jsonl' or rows_path in seen_rows:raise ValueError('Wrong retained row roster')
            seen_rows.add(rows_path)
            process=load_process(Path(full['process_file']['path']))
            validate_native_process(process)
            for key,cap in (('stdout',STDOUT_CAP),('stderr',STDERR_CAP)):
                bound(process[key])
                if process[key]['bytes']>cap:raise ValueError('Retained native output exceeds cap')
            cleanup=load(Path(record['native_work_cleanup']['path']))
            if cleanup.get('removed') is not True or Path(cleanup['owned_path']).exists():raise ValueError('Native work cleanup not retained')
        if seen_rows!=set(self.out.rglob('rows.jsonl')):raise ValueError('Extra or missing retained full rows')

    def run(self):
        goroot=self.invoke('go-root',[str(self.go),'env','GOROOT'],seconds=30).read_text().strip()
        if not goroot or not self.go.samefile(Path(goroot)/'bin/go'):raise ValueError('Explicit Go is not reported GOROOT/bin/go')
        self.env['GOROOT']=goroot
        self.invoke('unit-tests',[str(self.go),'test','-count=1','./cmd/path-facts-oracle','./cmd/check-path-facts'])
        self.invoke('orchestrator-unit-tests',[str(Path(sys.executable).resolve()),'-I','-B',str(self.repo/'tools/check-path-facts_test.py')],seconds=30)
        self.invoke('build-oracle',[str(self.go),'build','-o',str(self.work/'bin/oracle'),'./cmd/path-facts-oracle'])
        self.invoke('build-checker',[str(self.go),'build','-o',str(self.work/'bin/checker'),'./cmd/check-path-facts'])
        self.invoke('compiler-build',[str(self.compiler),'version','--build','--json'],seconds=30)
        self.invoke('original-go-manual24',[str(self.work/'bin/oracle'),'--manual',str(self.repo/'cmd/check-path-facts/testdata/manual-contract.json'),'--corpus','manual','--out',str(self.out/'manual-original-go.json')],seconds=30)
        self.oracle_ref=ref(self.out/'manual-original-go.json')
        manual=load(self.repo/'cmd/check-path-facts/testdata/manual-contract.json')
        receipts={};records=[]
        for entry,key,manifest in ENTRIES:
            inputs=self.out/(entry+'-inputs.json')
            raw=(json.dumps({'schema':'gooo/body-composition-inputs/v1','inputs':[{key:r['input']} for r in manual['cases']]},ensure_ascii=True,separators=(',',':'))+'\n').encode()
            if len(raw)>8192 or len(manual['cases'])!=24:raise ValueError('Fixed manual wire/row bound')
            source_bytes=(self.repo/'ports/path/path.gooo').stat().st_size
            if entry=='report':source_bytes+=(self.repo/'examples/path-facts/main.gooo').stat().st_size
            if not 0<source_bytes<=(10<<20):raise ValueError('Bounded selected source closure required')
            lowered='GoooPackage'+'9'*len(str(source_bytes))+'Activity'+key.split(':')[1]
            translated={'schema':'gooo/body-composition-inputs/v1','cases':[{'inputs':{lowered:r['input']},'expected':{}} for r in manual['cases']]}
            upper=len(json.dumps(translated,ensure_ascii=True,separators=(',',':')).encode())
            if upper>8192:raise ValueError('Source-derived translated wire bound exceeds8192')
            save_new(self.out/(entry+'-wire-bound.json'),{'raw_bytes_including_LF':len(raw),'source_closure_bytes':source_bytes,
                 'maximum_lowered_key_bytes':len(lowered),'translated_compact_upper_bound_bytes':upper,
                 'scope':'Pinned839 source-derived bound, not observed translated bytes or actual LoweredName; raw file bytes measured exactly.'})
            with inputs.open('xb') as f:f.write(raw)
            self.invoke(entry+'-strict-inputs',[str(self.work/'bin/oracle'),'--check-input',str(inputs),'--entry',key],seconds=30)
            execution=self.invoke(entry+'-execute',[str(self.compiler),'package','execute','--json','--go',str(self.go),'--inputs',str(inputs),str(self.repo/manifest)])
            replay=self.invoke(entry+'-replay',[str(self.compiler),'package','replay','--json','--go',str(self.go),'--receipt',str(execution),'--inputs',str(inputs),str(self.repo/manifest)])
            full=self.out/(entry+'-full')
            self.invoke(entry+'-observe',self.arguments(entry,inputs,execution,replay,full))
            checked=self.out/(full.name+'-checked.json');result=load(checked);generated=checked_entry(result,entry)
            cleanup=self.cleanup_native_work(entry,generated,checked)
            records.append({'entry':entry,'checked':ref(checked),'full_generated':generated,'native_work_cleanup':cleanup,'execute':ref(execution),'saved_replay':ref(replay),'inputs':ref(inputs)})
            receipts[entry]=(inputs,execution,replay)
        # Source-bound strict transport controls do not claim bareCLI rejection.
        negative_names=['text-1025','raw-invalid-utf8','unpaired-high-surrogate','unpaired-low-surrogate','null','boolean','integer','array','object','missing','duplicate-input','escaped-duplicate-input','unknown-input','malformed','trailing-json']
        reasons=['TEXT_BYTE_BOUND','INVALID_RAW_UTF8','INVALID_UNICODE_SCALAR','INVALID_UNICODE_SCALAR']+['TEXT_TYPE']*5+['REQUIRED_INPUT','DUPLICATE_KEY','DUPLICATE_KEY','UNEXPECTED_INPUT','MALFORMED_JSON','TRAILING_JSON']
        for name,reason in zip(negative_names,reasons):
            self.invoke('negative-'+name,[str(self.work/'bin/oracle'),'--check-input',str(self.repo/'cmd/path-facts-oracle/testdata/negative'/(name+'.json')),'--entry','app/path-facts:Report'],seconds=30,expected=1,diagnostic=reason)
        tampers=[]
        for entry,(inputs,execution,replay) in receipts.items():
            raw=execution.read_bytes();delivery=['result','runtime','traces',0,'deliveries',0]
            if entry=='report':
                altered=changed_value(raw,delivery+['actual','ext'],'.wrong')
                altered=changed_value(altered,delivery+['actual_fields',1,'value'],'.wrong')
            else:altered=changed_value(raw,delivery+['actual'],'.wrong' if entry=='ext' else True)
            controls=[('actual',altered,True,'execution: invalid native evidence: independent Go oracle differs at row 0'),
             ('input',changed_value(raw,delivery+['input'],'changed'),True,'execution: invalid native evidence: original single input, activity identity or input-only metadata at row 0'),
             ('file',changed_value(raw,['inputs_digest'],'sha256:'+'0'*64),True,'execution: invalid native evidence: input-only receipt or file binding'),
             ('replay-link',changed_value(replay.read_bytes(),['replayed_from_sha256'],'sha256:'+'0'*64),False,'replay: invalid native evidence: saved receipt replay binding')]
            if entry!='report':
                for kind,value in [('boolean',False),('text','wrong'),('array',[])]:controls.append(('pure-calls-'+kind,wrong_pure_calls(replay.read_bytes(),value),False,'replay: invalid native evidence: unexpected pure-call metadata for a direct scalar entry'))
            for name,altered,is_execution,diagnostic in controls:
                path=self.out/(entry+'-tamper-'+name+'.json')
                with path.open('xb') as f:f.write(altered)
                self.invoke(entry+'-tamper-'+name,self.arguments(entry,inputs,path if is_execution else execution,replay if is_execution else path,self.out/(entry+'-tamper-'+name+'-unused-full')),seconds=30,expected=1,diagnostic=diagnostic)
                tampers.append(ref(path))
        if len(tampers)!=18:raise ValueError('Tamper roster changed')
        self.frozen();self.budget();self.audit_retained(records)
        summary={'schema':'gooo/path-facts-two-layer-CI/v1','status':'VERIFIED_VALUES_PENDING_OWNED_WORK_CLEANUP',
             'compiler_revision':'83928fa4538245abe33b08549da47f8cbdc50ed2','go_version':'go1.27.2',
             'manual_input_count_per_entry':24,'manual_entry_count':3,'manual_execute_outputs':72,'manual_saved_replay_outputs':72,
             'generated_native_input_count_per_entry':9841,'generated_native_entry_outputs':29523,'generated_native_scalar_comparisons':78728,
             'full_CLI_outputs':0,'full_saved_replay_outputs':0,'strict_transport_refusals':15,'receipt_tamper_refusals':18,
             'native_receipt_expectation_cases':0,'source_freeze':self.freeze,'original_Go_manual_oracle':self.oracle_ref,'entries':records,'stages':self.stages,
             'scope':'All9841x3 values use freshly generated native Gooo functions; only manual24x3 have individual CLI and saved-replay observations. Existing native66/159909/2109 and7 exhaustive checks are separate and unchanged. No fullText-domain/library/safety/performance claim.'}
        save_new(self.out/'pending-verification.json',summary)
        self.budget()
        return summary

    def cleanup(self):
        if self.work is None:return
        self.require_groups_gone()
        current=self.work.stat()
        if self.work.is_symlink() or not stat.S_ISDIR(current.st_mode) or (current.st_dev,current.st_ino)!=(self.owner.st_dev,self.owner.st_ino):raise RuntimeError('Owned work identity changed')
        size=logical(self.work);shutil.rmtree(self.work)
        save_new(self.out/'work-cleanup.json',{'owned_path':str(self.work),'logical_bytes_removed':size,'owned_inode':self.owner.st_ino,'all_top_level_groups_reaped':True})
        self.work_cleanup_complete=True


def execute_attempt(args):
    attempt=None;summary=None;primary=None;secondary=[]
    try:
        attempt=Attempt(args);attempt.prepare();summary=attempt.run()
    except BaseException as exc:primary=exc
    if attempt is not None:
        try:attempt.cleanup()
        except BaseException as exc:
            secondary.append({'operation':'owned-work-cleanup','error':error_detail(exc)})
            if primary is None:primary=exc
    if primary is None:
        try:
            attempt.frozen();attempt.audit_retained(summary['entries'])
            if time.monotonic()-attempt.started>=attempt.overall:raise RuntimeError('Overall budget expired before final completion')
            summary['status']='MANUAL24_CLI_REPLAY_AND_FULL9841_GENERATED_NATIVE_PASS'
            summary['work_cleanup']=ref(attempt.out/'work-cleanup.json')
            cleanup=load(Path(summary['work_cleanup']['path']))
            if cleanup.get('all_top_level_groups_reaped') is not True or Path(cleanup['owned_path']).exists():raise RuntimeError('Final owned cleanup is incomplete')
            encoded=(json.dumps(summary,indent=2)+'\n').encode()
            rows=sum(p.stat().st_size for p in attempt.out.rglob('rows.jsonl'))
            if rows>ROWS_CAP or logical(attempt.out)-rows+len(encoded)>PROOF_CAP:raise RuntimeError('Final retained proof cap')
            if shutil.disk_usage(attempt.out).free<FREE_MIN:raise RuntimeError('Require5GiB free at completion')
            with (attempt.out/'summary.json').open('xb') as f:f.write(encoded)
        except BaseException as exc:primary=exc
    if primary is not None:
        failure={'status':'FAILED_OR_INTERRUPTED','error':repr(primary),'primary_error':error_detail(primary),
                 'secondary_errors':secondary+(getattr(attempt,'secondary_errors',[]) if attempt is not None else []),
                 'stages':getattr(attempt,'stages',[]),'automatic_retries':0,
                 'unpersisted_process':getattr(attempt,'pending_process',None),
                 'last_group_observations':getattr(attempt,'last_group_observations',[]),
                 'unconfirmed_reaped_pids':sorted(getattr(attempt,'groups',set())-getattr(attempt,'reaped',set())),
                 'owned_work_path':str(attempt.work) if attempt is not None and attempt.work is not None else None,
                 'owned_work_cleanup_confirmed':getattr(attempt,'work_cleanup_complete',False)}
        if attempt is not None and attempt.owns_out:
            try:save_new(attempt.out/'failure.json',failure)
            except BaseException as exc:
                failure['secondary_errors'].append({'operation':'failure-ledger-write','error':error_detail(exc)})
                try:sys.stderr.write(json.dumps(failure)+'\n')
                except BaseException:pass
        raise primary.with_traceback(primary.__traceback__)
    return summary


def main():
    p=argparse.ArgumentParser()
    for key in ('repository','out','go','compiler'):p.add_argument('--'+key,required=True,type=Path)
    p.add_argument('--timeout',type=int,default=300)
    args=p.parse_args()
    for sig in (signal.SIGTERM,signal.SIGINT,signal.SIGHUP):
        signal.signal(sig,lambda signum,_: (_ for _ in ()).throw(SystemExit(128+signum)))
    execute_attempt(args)


if __name__=='__main__':main()
