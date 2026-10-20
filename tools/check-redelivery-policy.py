"""Fresh public manual16/replay plus full12288 generated-native policy evidence.
No import-time launch. Public path lifecycle is loaded only by explicit main/tests.
"""
from pathlib import Path
from decimal import Decimal
import argparse
import hashlib
import itertools
import json
import os
import shutil
import signal
import stat
import sys
import tempfile
import time
import types
import zlib

ENTRY='app/automatic-redelivery:AssessRedelivery'
ACTIVITY_ID='gooo://automatic-redelivery/assess/v1'
WORK_CAP,PRE_CAP,FREE_MIN=384<<20,320<<20,5<<30
STDOUT_CAP,STDERR_CAP,PROOF_CAP=1<<20,256<<10,8<<20
RAW_CAP,GZIP_CAP,LINE_CAP,WIRE_CAP=16<<20,2<<20,16<<10,8192
GZIP_HEADER=bytes.fromhex('1f8b08000000000002ff')
MODULE_NAMES=('go.mod','main.go','candidate.go','corpus.go','rows.go','boundary.go','policy.go','harness_test.go','generated.go')
POSITIVE=('go-root','build-oracle','build-checker','compiler-build','original-oracle-manual16','batch-0-strict','batch-0-execute','batch-0-replay','batch-0-observe','batch-8-strict','batch-8-execute','batch-8-replay','batch-8-observe','harness-tests','build-harness','full12288-generated-native')
HELPER_SHA='95d565708d554fe1d37c0eee9030bc243b11c0755d6d1032197a0a346e030c28'
H=None

WIRE_NAMES=('missing','unknown','null-text','null-boolean','null-integer','bool-as-integer','number-as-boolean','fractional-integer','out-of-int64','duplicate-exact','duplicate-escaped','raw-invalid-utf8','unpaired-high','unpaired-low','text-1025','trailing','malformed','nonobject')
WIRE_CODES=('MISSING_FIELD','UNKNOWN_FIELD','TEXT_TYPE','BOOLEAN_TYPE','INTEGER_TOKEN','INTEGER_TOKEN','BOOLEAN_TYPE','INTEGER_TOKEN','INTEGER_RANGE','DUPLICATE_FIELD','DUPLICATE_FIELD','INVALID_RAW_UTF8','INVALID_UNICODE_SCALAR','INVALID_UNICODE_SCALAR','TEXT_BYTE_BOUND','TRAILING_JSON','MALFORMED_JSON','OBJECT_REQUIRED')

MANUAL_COUNTS={'count': 8, 'manual_total': 16, 'required_output_fields': 15, 'native_observed_cases': 8, 'saved_replay_observed_cases': 8, 'independent_go_oracle_cases': 8, 'typed_fields_compared_per_phase': 120, 'echo_values_compared_per_phase': 72, 'native_receipt_expectation_cases': 0, 'full_native_cases': 0, 'full_CLI_cases': 0, 'full_saved_replay_cases': 0}
TAMPERS=[('derivedReason', 'execution', 'independent Go output differs'), ('originalEcho', 'execution', 'record input echo method'), ('integerKind', 'execution', 'record field attempts_if_claimed kind/int64'), ('missingRequiredField', 'execution', 'record or echo roster'), ('fieldID', 'execution', 'field 0 identity/type/order/value'), ('inputTextPort', 'execution', 'trace 0 port 0 original typed input'), ('inputIntegerKind', 'execution', 'trace 0 port 7 original typed input'), ('sourceImage', 'execution', 'workspace original source image hash'), ('generationBody', 'execution', 'generation source, body, typecheck or replay provenance'), ('typedPlan', 'execution', 'native typed plan hash'), ('producerRevision', 'execution', 'native producer/toolchain'), ('inputFileDigest', 'execution', 'input-only receipt/file binding'), ('rawReplayParent', 'replay', 'raw receipt and inner pinned result replay binding'), ('innerPriorResult', 'replay', 'raw receipt and inner pinned result replay binding'), ('pureCallsFalse', 'execution', 'unexpected pure-call/entity-alias metadata'), ('pureCallsString', 'execution', 'unexpected pure-call/entity-alias metadata'), ('pureCallsArray', 'execution', 'unexpected pure-call/entity-alias metadata'), ('pureCallsObject', 'execution', 'unexpected pure-call/entity-alias metadata')]


def bind_helper(repository):
    global H
    path=repository/'tools/check-path-facts.py';raw=path.read_bytes()
    if hashlib.sha256(raw).hexdigest()!=HELPER_SHA:raise ValueError('Public lifecycle source differs')
    sys.dont_write_bytecode=True
    H=types.ModuleType('public_path_lifecycle');H.__file__=str(path)
    exec(compile(raw,str(path),'exec'),H.__dict__)
    for name in ('ref','save_new','logical','strict_object','load','exact','load_process','error_detail'):
        globals()[name]=getattr(H,name)
    return H


def raw_token(raw,path,token):
    text,(a,b)=H.value_span(raw,path)
    changed=(text[:a]+token+text[b:]).encode('utf-8')
    if changed==raw:raise ValueError('Tamper did not change token')
    before=json.loads(text[a:b],parse_float=Decimal);after=json.loads(token,parse_float=Decimal)
    if type(before) is type(after):raise ValueError('Kind-only tamper did not change type')
    if not changed.startswith(text[:a].encode()) or not changed.endswith(text[b:].encode()):raise ValueError('Tamper changed unrelated bytes')
    return changed


def without_member(raw,path,key):
    text,(a,b)=H.value_span(raw,path+[key]);_,(start,end)=H.value_span(raw,path)
    first=text.rfind(json.dumps(key),start,a)
    if first<0 or text[first+len(json.dumps(key)):a].strip()!=':':raise ValueError('Canonical member span required')
    last=b
    while last<end and text[last].isspace():last+=1
    if text[last]==',':last+=1
    else:
        previous=first-1
        while previous>start and text[previous].isspace():previous-=1
        if text[previous]==',':first=previous
    result=(text[:first]+text[last:]).encode()
    before=json.loads(raw);after=json.loads(result)
    if before==after:raise ValueError('Member removal did not change value')
    return result


def load_receipt(path):
    # Receipt metadata legitimately contains fractional latency diagnostics.
    # Preserve those as Decimal; typed actual/input/field values below still
    # require exact Python kinds, so an integer output encoded as 1.0 fails.
    return json.loads(path.read_bytes().decode('utf-8'),object_pairs_hook=strict_object,parse_float=Decimal,
                      parse_constant=lambda _: (_ for _ in ()).throw(ValueError('Nonfinite receipt diagnostic')))


def typed_equal(left,right):
    if type(left) is not type(right):return False
    if type(left) is dict:return set(left)==set(right) and all(typed_equal(left[k],right[k]) for k in left)
    if type(left) is list:return len(left)==len(right) and all(typed_equal(a,b) for a,b in zip(left,right))
    return left==right


def validated_oracle(value,witness,contract,contract_sha):
    if type(value) is not dict or value.get('schema')!='gooo/redelivery-independent-manual-oracle/v1' or value.get('contract_sha256')!=contract_sha:
        raise ValueError('Wrong fresh independent oracle identity')
    cases=value.get('cases')
    if type(cases) is not list or len(cases)!=16 or len(witness['cases'])!=16:raise ValueError('Require exact independent manual16')
    fields=contract['output_fields'];inputs=contract['input_order']
    if len(fields)!=15 or len(inputs)!=9:raise ValueError('Fixed9input15field contract required')
    for i,(row,original) in enumerate(zip(cases,witness['cases'])):
        if not exact(row.get('index'),i) or row.get('name')!=original['name'] or not typed_equal(row.get('inputs'),original['inputs']):raise ValueError('Independent oracle changed witness order or input')
        report=row.get('expected')
        if type(report) is not dict or set(report)!={f['name'] for f in fields}:raise ValueError('Expected fifteen present scalar fields')
        for field in fields:
            kind={'string':str,'boolean':bool,'integer':int}[field['type']]
            if type(report[field['name']]) is not kind:raise ValueError('Expected field scalar kind differs')
        for field in inputs:
            if not typed_equal(report[field['name']],original['inputs'][field['name']]):raise ValueError('Expected echo differs')
    return cases


def batch_documents(full_inputs,witness,contract,source_bytes):
    rows=full_inputs.get('inputs');names=[f['name'] for f in contract['input_order']]
    if full_inputs.get('schema')!='gooo/body-composition-inputs/v1' or type(rows) is not list or len(rows)!=16 or len(names)!=9:raise ValueError('Fixed input-only manual16 required')
    if type(source_bytes) is not int or not 0<source_bytes<=(10<<20):raise ValueError('Selected source bound differs')
    # Pinned839 ordinal starts0/increments once per ActivityDecl, so its decimal
    # digits are bounded by selected original source bytes. execute.go preserves
    # each .inputN suffix. This is a conservative name, not an observed lowering.
    largest='GoooPackage'+'9'*len(str(source_bytes))+'ActivityAssessRedelivery'
    for i,row in enumerate(rows):
        want={ENTRY+'.input'+str(j):witness['cases'][i]['inputs'][name] for j,name in enumerate(names)}
        if not typed_equal(row,want):raise ValueError('Native manual witness mapping differs')
    batches=[]
    for offset in (0,8):
        selected=rows[offset:offset+8]
        document={'schema':'gooo/body-composition-inputs/v1','inputs':selected}
        raw=(json.dumps(document,ensure_ascii=True,separators=(',',':'))+'\n').encode()
        translated={'schema':'gooo/body-composition-inputs/v1','cases':[{'inputs':{largest+'.input'+str(j):row[ENTRY+'.input'+str(j)] for j in range(9)},'expected':{}} for row in selected]}
        upper=len(json.dumps(translated,ensure_ascii=True,separators=(',',':')).encode())+1
        if len(raw)>WIRE_CAP or upper>WIRE_CAP:raise ValueError('Fixed8case source-derived wire bound exceeds8192')
        batches.append({'offset':offset,'count':8,'raw':raw,'raw_bytes':len(raw),'translated_upper_bytes':upper,'max_lowered_key_bytes':len(largest+'.input8'),'source_bytes':source_bytes})
    return batches


def compare_raw_receipt(path,cases,phase,contract):
    raw=load_receipt(path)
    if raw.get('decision')!='OBSERVED':raise ValueError('Wrong input-only observation decision')
    runtime=raw['result']['runtime']
    if not exact(runtime.get('finite_total'),0) or not exact(runtime.get('finite_passed'),0):raise ValueError('No expected values may enter native receipt')
    traces=runtime.get('traces')
    if type(traces) is not list or len(traces)!=8 or len(cases)!=8:raise ValueError('Eight ordered single-activity traces required')
    for index,(trace,row) in enumerate(zip(traces,cases)):
        if type(trace) is not dict or not exact(trace.get('case_index'),index):raise ValueError('Local trace order differs')
        deliveries=trace.get('deliveries')
        if type(deliveries) is not list or len(deliveries)!=1:raise ValueError('Exactly one actual delivery per trace required')
        d=deliveries[0]
        if type(d) is not dict or d.get('activity_id')!=ACTIVITY_ID:raise ValueError('Original activity identity differs')
        expected=row['expected']
        if not typed_equal(d.get('actual'),expected):raise ValueError(phase+' actual fifteen typed fields differ')
        values=[row['inputs'][f['name']] for f in contract['input_order']]
        expected_inputs=[{'port':'input'+str(j),'entity_id':'gooo://std/text' if j in (0,1,8) else 'gooo://std/integer' if j==7 else 'gooo://std/boolean','value':value} for j,value in enumerate(values)]
        if not typed_equal(d.get('inputs'),expected_inputs):raise ValueError(phase+' original nine typed inputs differ')
        expected_fields=[{'name':f['name'],'id':f['id'],'value':expected[f['name']]} for f in contract['output_fields']]
        if not typed_equal(d.get('actual_fields'),expected_fields):raise ValueError(phase+' fifteen field identities/order/values differ')
    return {'cases':8,'typed_fields_compared':120,'echo_values_compared':72}



def check_rows(path,contract):
    # Independent transport/order/type/echo recheck, deliberately no policy
    # evaluator. Policy values are independently compared by unchanged Go
    # Evaluate in the fresh harness and again in its own gzip readback.
    if path.stat().st_size>GZIP_CAP:raise ValueError('Compressed rows exceed2MiB')
    names=[x['name'] for x in contract['input_order']];domains=[x['domain'] for x in contract['input_order']]
    if len(names)!=9 or len(contract['output_fields'])!=15:raise ValueError('Require9input15field contract')
    expected=iter(itertools.product(*domains));count=0;size=0;raw_hash=hashlib.sha256();pending=b''
    def row(raw):
        nonlocal count
        if not raw or len(raw)>LINE_CAP:raise ValueError('Invalid JSONL line bound')
        value=json.loads(raw.decode('utf-8'),object_pairs_hook=strict_object,parse_float=lambda _: (_ for _ in ()).throw(ValueError('JSONL exact integer required')),parse_constant=lambda _: (_ for _ in ()).throw(ValueError('JSONL nonfinite')))
        if type(value) is not dict or set(value)!={'index','inputs','actual'} or not exact(value.get('index'),count):raise ValueError('Full row index/shape/order differs')
        try:tuple_values=next(expected)
        except StopIteration:raise ValueError('Excess full-domain row')
        inputs=dict(zip(names,tuple_values))
        if not typed_equal(value.get('inputs'),inputs):raise ValueError('Full tuple input/order differs')
        actual=value.get('actual')
        if type(actual) is not dict or set(actual)!={x['name'] for x in contract['output_fields']}:raise ValueError('All fifteen actual fields required')
        for field in contract['output_fields']:
            item=actual[field['name']];kind={'string':str,'boolean':bool,'integer':int}[field['type']]
            if type(item) is not kind:raise ValueError('Actual field exact scalar kind differs')
            if kind is int and not -(1<<63)<=item<(1<<63):raise ValueError('Actual integer outsideint64')
            if kind is str and len(item.encode('utf-8'))>1024:raise ValueError('Actual text invalid or oversized')
        if any(not typed_equal(actual[n],inputs[n]) for n in names):raise ValueError('Actual nine original echoes differ')
        count+=1
    decoder=zlib.decompressobj(31)
    with path.open('rb') as stream:
        if stream.read(10)!=GZIP_HEADER:raise ValueError('Gzip deterministic header differs')
        stream.seek(0)
        while True:
            chunk=stream.read(64<<10)
            if not chunk:break
            # Limit each decoded output allocation; unconsumed bytes are fed
            # again until progress. Stream only one gzip member.
            data=chunk
            while data:
                piece=decoder.decompress(data,64<<10);data=decoder.unconsumed_tail
                size+=len(piece)
                if size>RAW_CAP:raise ValueError('Raw full JSONL exceeds16MiB')
                raw_hash.update(piece);pending+=piece
                while b'\n' in pending:
                    line,pending=pending.split(b'\n',1);row(line)
                if len(pending)>LINE_CAP:raise ValueError('Unterminated JSONL line exceeds bound')
                if decoder.unused_data:raise ValueError('Extra gzip member or trailing bytes')
                if not data:break
        if not decoder.eof or pending:raise ValueError('Incomplete gzip or missing final LF')
    try:next(expected)
    except StopIteration:pass
    else:raise ValueError('Missing full-domain tuple')
    if count!=12288:raise ValueError('Exact full-domain row count differs')
    compressed=ref(path)
    return {'path':str(path),'compressed_bytes':compressed['bytes'],'compressed_sha256':compressed['sha256'],'uncompressed_bytes':size,'uncompressed_sha256':raw_hash.hexdigest(),'row_count':count}


def check_summary(value,rows,boundary):
    if value.get('schema')!='gooo/redelivery-full-domain-generated-native/v1' or value.get('decision')!='FULL12288_GENERATED_NATIVE_VALUES_MATCHED':raise ValueError('Full generated layer identity differs')
    counts={'input_count':12288,'generated_function_calls':12288,'oracle_evaluations':12288,'typed_field_comparisons':184320,'echo_value_comparisons':110592,'mismatches':0,'readback_row_count':12288,'readback_oracle_evaluations':12288,'readback_typed_field_comparisons':184320,'readback_echo_value_comparisons':110592,'readback_mismatches':0,'full_CLI_cases':0,'full_saved_replay_cases':0}
    if any(not exact(value.get(k),v) for k,v in counts.items()):raise ValueError('Full generated counts/layers differ')
    if not typed_equal(value.get('rows'),rows) or not typed_equal(value.get('boundary'),boundary):raise ValueError('Full summary raw rows/boundary differ')


CORE=['automatic-redelivery.workspace.json']+['examples/automatic-redelivery/'+x for x in
 ('main.gooo','contract.json','inputs.json','manual-inputs.json','manual-witnesses.json','README.md')]+[
 'examples/automatic-redelivery/reference/'+x for x in ('go.mod','main.go','policy.go','policy_test.go','strict.go')]
CHECKER=('main.go','inspect.go','binding.go','origin.go','wire.go','wire_test.go','observer_test.go','result_digest.go','result_digest_test.go','current_result_profile_test.go')
TEMPLATES=tuple(x for x in MODULE_NAMES if x not in ('policy.go','generated.go'))


class Attempt:
    def __init__(self,args):
        self.repository=args.repository.resolve();self.repo=self.repository;self.out=args.out.resolve()
        self.go=args.go.resolve();self.compiler=args.compiler.resolve();self.overall=args.timeout
        if type(self.overall) is not int or not 1<=self.overall<=300:raise ValueError('Timeout must be1..300seconds')
        if self.out.exists():raise ValueError('Refuse existing policy proof directory')
        self.started=time.monotonic();self.stages=[];self.specs=[];self.groups=set();self.children={};self.reaped=set()
        self.work=None;self.owner=None;self.owns_out=False;self.secondary_errors=[];self.pending_process=None
        self.last_group_observations=[];self.work_cleanup_complete=False
        self.producers=[];self.module_refs=[];self.runtime_refs=[];self.records=[];self.oracle_ref=None
        helper=bind_helper(self.repository)
        for name in ('invoke','require_groups_gone','cleanup'):
            setattr(self,name,types.MethodType(getattr(helper.Attempt,name),self))
        self.contract_path=self.repository/'examples/automatic-redelivery/contract.json';self.contract=load(self.contract_path)
        self.witness_path=self.repository/'examples/automatic-redelivery/manual-witnesses.json';self.witness=load(self.witness_path)
        self.native_path=self.repository/'examples/automatic-redelivery/manual-inputs.json'
        self.profile=self.repository/'cmd/check-path-facts/result-profile-go1273.json'
        self.reference=self.repository/'examples/automatic-redelivery/reference'
        self.wire_dir=self.repository/'cmd/check-redelivery-policy/testdata/wire';self.wire=load(self.wire_dir/'manifest.json')
        cases=self.wire.get('cases')
        if not exact(self.wire.get('count'),36) or type(cases) is not list or len(cases)!=36:raise ValueError('Exact36 wire fixtures required')
        wanted=[(mode+'-'+name,mode,'WIRE_REFUSED: '+code) for mode in ('named','native') for name,code in zip(WIRE_NAMES,WIRE_CODES)]
        if [(x.get('name'),x.get('format'),x.get('stderr')) for x in cases]!=wanted:raise ValueError('Require ordered18named+18native intended wire controls')
        names=CORE+['cmd/check-redelivery-policy/'+n for n in CHECKER]+[
         'tools/redelivery-native-source/'+n+'.txt' for n in TEMPLATES]+[
         'tools/check-redelivery-policy.py','tools/check-redelivery-policy_test.py',
         'tools/check-path-facts.py','cmd/check-path-facts/result-profile-83928fa.json','cmd/check-path-facts/result-profile-go1273.json',
         'cmd/check-path-facts/testdata/current-go1273-execution.json',
         'cmd/check-path-facts/testdata/current-go1273-execution.provenance.json',
         'cmd/check-literal-text-report/testdata/pinned-original-execution.json',
         'internal/jsonkeys/keys.go','internal/jsonkeys/keys_test.go','LICENSE','go.mod','upstream.json',
         'cmd/check-redelivery-policy/testdata/wire/manifest.json']
        if (self.repository/'go.sum').exists():names.append('go.sum')
        for item in cases:
            if type(item) is not dict or item.get('format') not in ('named','native') or Path(item.get('file','')).name!=item.get('file'):raise ValueError('Bad wire source path')
            names.append('cmd/check-redelivery-policy/testdata/wire/'+item['file'])
        if len(names)!=len(set(names)):raise ValueError('Duplicate public source')
        self.sources=[]
        for name in names:
            path=self.repository/name
            if path.resolve()!=path or path.is_symlink() or not path.is_file():raise ValueError('Public source must be canonical regular file')
            self.sources.append(ref(path))
        for directory,want in [(self.repository/'cmd/check-redelivery-policy',set(CHECKER)|{'testdata'}),
          (self.repository/'tools/redelivery-native-source',{n+'.txt' for n in TEMPLATES}),
          (self.reference,{'go.mod','main.go','policy.go','policy_test.go','strict.go'}),
          (self.repository/'internal/jsonkeys',{'keys.go','keys_test.go'})]:
            actual={p.name for p in directory.iterdir()}
            if actual!=want:raise ValueError('Public code source roster differs')
            if any(p.is_symlink() for p in directory.iterdir()):raise ValueError('Source roster symlink')
        if {p.name for p in (self.wire_dir.parent).iterdir()}!={'wire'}:raise ValueError('Checker testdata roster differs')
        if {p.name for p in self.wire_dir.iterdir()}!={'manifest.json'}|{x['file'] for x in cases}:raise ValueError('Wire fixture roster differs')
        for item in cases:
            got=ref(self.wire_dir/item['file'])
            if got['bytes']!=item['bytes'] or got['sha256']!=item['sha256'] or not exact(item.get('exit_code'),1):raise ValueError('Wire source binding differs')
        self.tools=[ref(self.go),ref(self.compiler),ref(Path(sys.executable).resolve())]
        self.contract_ref=ref(self.contract_path);self.policy_ref=ref(self.reference/'policy.go')
        self.stage_names=list(POSITIVE)+['wire-'+x['name'] for x in cases]+['tamper-'+x[0] for x in TAMPERS]
        if len(self.stage_names)!=70 or len(set(self.stage_names))!=70:raise ValueError('Exact70 stage roster required')

    def prepare(self):
        self.out.mkdir(parents=True);self.owns_out=True
        self.work=Path(tempfile.mkdtemp(prefix='gooo-public-redelivery-')).resolve();self.owner=self.work.stat()
        for name in ('cache','gopath','module-cache','tmp','bin','module'):(self.work/name).mkdir()
        self.module=self.work/'module'
        save_new(self.out/'source-freeze.json',{'schema':'gooo/redelivery-public-runtime-source-freeze/v1',
          'repository_root':str(self.repository),'files':self.sources,'tools':self.tools,'overall_seconds':self.overall})
        self.freeze=ref(self.out/'source-freeze.json')
        self.env=dict(os.environ)
        for key in list(self.env):
            if key.startswith(('GOOO_','LAYA_')) or key.endswith(('_API_KEY','_TOKEN')) or key in ('GOCACHEPROG','LD_PRELOAD','DYLD_INSERT_LIBRARIES'):self.env.pop(key,None)
        self.env.update(GOCACHE=str(self.work/'cache'),GOPATH=str(self.work/'gopath'),GOMODCACHE=str(self.work/'module-cache'),
          GOTMPDIR=str(self.work/'tmp'),TMPDIR=str(self.work/'tmp'),GOROOT=str(self.go.parent.parent),GOTOOLCHAIN='local',GOWORK='off',
          GOENV='off',GOFLAGS='',GOPROXY='off',GOSUMDB='off',CGO_ENABLED='0',GOTELEMETRY='off',
          PATH=str(self.go.parent)+':/usr/bin:/bin:/usr/sbin:/sbin',PYTHONDONTWRITEBYTECODE='1')
        self.env.pop('GOOS',None);self.env.pop('GOARCH',None)

    def frozen(self,producers=True):
        for item in self.sources+self.tools+[self.freeze]+self.runtime_refs+(self.producers+self.module_refs if producers else []):
            if ref(Path(item['path']))!=item:raise ValueError('Frozen public source/tool/evidence changed')
        if producers and self.module_refs and sorted(p.name for p in self.module.iterdir())!=sorted(MODULE_NAMES):raise ValueError('Owned module roster differs')

    def budget(self,active=False):
        if time.monotonic()-self.started>=self.overall:raise RuntimeError('Whole policy timeout')
        if logical(self.work,active=active)>WORK_CAP:raise RuntimeError('Owned work exceeds384MiB')
        if logical(self.out)>PROOF_CAP:raise RuntimeError('Proof exceeds8MiB')
        for suffix,cap in (('.stdout',STDOUT_CAP),('.stderr',STDERR_CAP)):
            for p in self.out.glob('*'+suffix):
                if p.is_symlink() or not p.is_file() or p.stat().st_size>cap:raise RuntimeError('Policy stage output cap')
        if shutil.disk_usage(self.work).free<FREE_MIN:raise RuntimeError('Require5GiB free')

    def call(self,name,argv,seconds=120,cwd=None,expected=0,diagnostic=None,substring=None):
        if name!=self.stage_names[len(self.specs)]:raise ValueError('Stage order differs')
        previous=self.repo;self.repo=Path(cwd) if cwd else self.repository
        self.specs.append({'name':name,'argv':list(map(str,argv)),'cwd':str(self.repo),'expected':expected,'diagnostic':diagnostic,'substring':substring})
        try:
            output=self.invoke(name,argv,seconds=seconds,expected=expected,diagnostic=diagnostic)
            if substring is not None and (output.stat().st_size!=0 or substring not in (self.out/(name+'.stderr')).read_text()):raise ValueError('Wrong intended tamper refusal')
            return output
        finally:self.repo=previous

    def observe_args(self,record,out):
        return [str(self.work/'bin/checker'),'--snapshot',str(self.repository),'--contract',str(self.contract_path),
         '--oracle',self.oracle_ref['path'],'--oracle-origin',str(self.out/'oracle-origin.json'),'--inputs',record['inputs']['path'],
         '--offset',str(record['offset']),'--count','8','--execution',record['execution']['path'],'--replay',record['replay']['path'],
         '--compiler-build',str(self.out/'compiler-build.stdout'),'--go-tool',str(self.go),'--result-profile',str(self.profile),'--out',str(out)]

    def run(self):
        goroot=self.call('go-root',[str(self.go),'env','GOROOT'],30).read_text().strip()
        if not goroot or not self.go.samefile(Path(goroot)/'bin/go'):raise ValueError('Explicit Go root differs')
        self.call('build-oracle',[str(self.go),'build','-o',str(self.work/'bin/oracle'),'.'],cwd=self.reference)
        self.producers.append(ref(self.work/'bin/oracle'))
        self.call('build-checker',[str(self.go),'build','-o',str(self.work/'bin/checker'),'./cmd/check-redelivery-policy'])
        self.producers.append(ref(self.work/'bin/checker'))
        build=self.call('compiler-build',[str(self.compiler),'version','--build','--json'],30)
        identity=load(build)
        for key,want in [('version','0.6.37-dev'),('compiler_source_sha','205896a413e5dea92366309c35ae85e542d055f6'),('source_status','CLEAN_VCS'),('vcs_modified','false'),('go_version','go1.27.3'),('native_go_required','go1.27.3')]:
            if identity.get(key)!=want:raise ValueError('Fresh compiler identity differs')
        fresh=self.call('original-oracle-manual16',[str(self.work/'bin/oracle'),'--contract',str(self.contract_path),'--manual'],30,self.reference)
        self.oracle_ref=ref(fresh);self.runtime_refs.append(self.oracle_ref)
        self.oracle_cases=validated_oracle(load(fresh),self.witness,self.contract,self.contract_ref['sha256'])
        if len({r['expected']['reason'] for r in self.oracle_cases})!=13:raise ValueError('All13 manual reason witnesses required')
        closure=[ref(self.reference/n) for n in ('go.mod','main.go','policy.go','policy_test.go','strict.go')]
        origin={'schema':'gooo/redelivery-public-oracle-origin/v1','oracle':self.oracle_ref,'process':ref(self.out/'original-oracle-manual16.process.json'),
         'contract':self.contract_ref,'witness':ref(self.witness_path),'native_inputs':ref(self.native_path),'oracle_policy':self.policy_ref,
         'source_freeze':self.freeze,'oracle_binary':ref(self.work/'bin/oracle'),'go_tool':self.tools[0],'source_closure':closure}
        save_new(self.out/'oracle-origin.json',origin);self.runtime_refs.append(ref(self.out/'oracle-origin.json'))
        batches=batch_documents(load(self.native_path),self.witness,self.contract,(self.repository/'examples/automatic-redelivery/main.gooo').stat().st_size)
        for b in batches:
            name='batch-'+str(b['offset']);inputs=self.out/(name+'-inputs.json')
            with inputs.open('xb') as f:f.write(b['raw'])
            record={'offset':b['offset'],'count':8,'inputs':ref(inputs),'wire_bound':{'raw_bytes':b['raw_bytes'],'translated_upper_bytes':b['translated_upper_bytes']}}
            self.call(name+'-strict',[str(self.work/'bin/checker'),'--check-input',str(inputs),'--format','native','--result-profile',str(self.profile)],30)
            record['execution']=ref(self.call(name+'-execute',[str(self.compiler),'package','execute','--json','--go',str(self.go),'--inputs',str(inputs),str(self.repository/'automatic-redelivery.workspace.json')]))
            record['replay']=ref(self.call(name+'-replay',[str(self.compiler),'package','replay','--json','--go',str(self.go),'--receipt',record['execution']['path'],'--inputs',str(inputs),str(self.repository/'automatic-redelivery.workspace.json')]))
            checked=self.out/(name+'-checked.json');self.call(name+'-observe',self.observe_args(record,checked),30)
            record['checked']=ref(checked)
            for slot,stage in [('execute_process','-execute'),('replay_process','-replay'),('observe_process','-observe')]:record[slot]=ref(self.out/(name+stage+'.process.json'))
            value=load(checked)
            if value.get('schema')!='gooo/redelivery-manual-batch-checked/v1' or value.get('decision')!='FIXED_MANUAL16_SLICE_NATIVE_AND_SAVED_REPLAY_PASS' or not exact(value.get('offset'),b['offset']):raise ValueError('Wrong manual checker identity')
            for key,want in MANUAL_COUNTS.items():
                if not exact(value.get(key),want):raise ValueError('Wrong manual checker typed counts')
            cases=self.oracle_cases[b['offset']:b['offset']+8]
            compare_raw_receipt(Path(record['execution']['path']),cases,'execute',self.contract)
            compare_raw_receipt(Path(record['replay']['path']),cases,'replay',self.contract)
            self.records.append(record)
        self.full_layer(build)
        for item in self.wire['cases']:
            path=self.wire_dir/item['file']
            argv=[str(self.work/'bin/oracle'),'--contract',str(self.contract_path),'--input',str(path)] if item['format']=='named' else [str(self.work/'bin/checker'),'--check-input',str(path),'--format','native','--result-profile',str(self.profile)]
            self.call('wire-'+item['name'],argv,30,self.reference if item['format']=='named' else None,1,item['stderr'])
        self.tampers()
        self.audit_retained(True);self.budget()
        return {'schema':'gooo/redelivery-public-two-layer-CI/v1','status':'VERIFIED_VALUES_PENDING_OWNED_CLEANUP','source_freeze':self.freeze,
         'compiler_revision':'205896a413e5dea92366309c35ae85e542d055f6','go_version':'go1.27.3','contract_sha256':self.contract_ref['sha256'],
         'top_level_stages':70,'manual_execute_outputs':16,'manual_saved_replay_outputs':16,'manual_reason_count':13,'typed_fields_compared_per_phase':240,'echo_values_compared_per_phase':144,
         'generated_native_outputs':12288,'generated_native_scalar_comparisons':184320,'generated_native_echo_comparisons':110592,
         'readback_outputs':12288,'readback_scalar_comparisons':184320,'readback_echo_comparisons':110592,
         'full_CLI_outputs':0,'full_saved_replay_outputs':0,'native_receipt_expectation_cases':0,'strict_transport_refusals':36,'receipt_tamper_refusals':18,
         'entries':self.records,'full':self.full_record,'stages':self.stages,'oracle':self.oracle_ref,'oracle_origin':ref(self.out/'oracle-origin.json'),
         'retained_runtime_refs':self.runtime_refs,
         'producer_binaries_observed_while_present':self.producers,'automatic_retries':0,
         'scope':'Manual16 actualCLI+savedreplay and full12288 fresh-generated values are distinct. Original66 corpus unchanged. Trusted-fact policy only; no host adapter/effect/current authority/safety/speedup claim.'}

    def full_layer(self,build):
        receipts=[load_receipt(Path(r[k]['path'])) for r in self.records for k in ('execution','replay')]
        first=receipts[0];c=first['result']['composition'];generated=c['source'].encode()
        if c['generated_sha256']!='sha256:'+hashlib.sha256(generated).hexdigest():raise ValueError('Fresh generated digest differs')
        for other in receipts[1:]:
            d=other['result']['composition']
            if any(d[k]!=c[k] for k in ('source','generated_sha256','driver_sha256')) or d['plan']['typed_plan_sha256']!=c['plan']['typed_plan_sha256']:raise ValueError('Manual slices generated identity differs')
        generated_path=self.out/'generated.go.txt'
        with generated_path.open('xb') as f:f.write(generated)
        self.generated_ref=ref(generated_path)
        for name in MODULE_NAMES:
            raw=generated if name=='generated.go' else (self.reference/'policy.go').read_bytes() if name=='policy.go' else (self.repository/'tools/redelivery-native-source'/(name+'.txt')).read_bytes()
            with (self.module/name).open('xb') as f:f.write(raw)
            self.module_refs.append(ref(self.module/name))
        origin={'schema':'gooo/redelivery-public-generated-source-origin/v1','repository_root':str(self.repository),
         'source_freeze':self.freeze,'oracle_origin':ref(self.out/'oracle-origin.json'),'contract':self.contract_ref,
         'candidate':ref(self.repository/'examples/automatic-redelivery/main.gooo'),'manifest':ref(self.repository/'automatic-redelivery.workspace.json'),
         'native_inputs':ref(self.native_path),'compiler_build':ref(build),'result_profile':ref(self.profile),'generated_source':self.generated_ref,
         'oracle_policy':self.policy_ref,'entry':first['result']['program']['entry'],'activity_id':ACTIVITY_ID,'record_id':'gooo://automatic-redelivery/decision/v1',
         'generated_function':'GoooComposedActivity0','contract_sha256':self.contract_ref['sha256'],
         'compiler_revision':'205896a413e5dea92366309c35ae85e542d055f6','Go_version':'go1.27.3',
         'generated_sha256':c['generated_sha256'],'driver_sha256':c['driver_sha256'],
         'lowered_source_sha256':first['result']['execution_source_sha256'],'typed_plan_sha256':c['plan']['typed_plan_sha256'],
         'original_body_sha256':c['steps'][0]['generation']['report']['program_digest'],'slices':self.records}
        save_new(self.out/'generated-origin.json',origin);self.origin_ref=ref(self.out/'generated-origin.json')
        self.runtime_refs.extend([self.generated_ref,self.origin_ref])
        self.call('harness-tests',[str(self.go),'test','-count=1','./...'],cwd=self.module)
        self.call('build-harness',[str(self.go),'build','-o',str(self.work/'bin/harness'),'.'],cwd=self.module)
        binary=ref(self.work/'bin/harness');self.producers.append(binary)
        bound={'schema':'gooo/redelivery-full-domain-runtime-boundary/v1','source_origin':self.origin_ref,
         'generated_source':ref(self.module/'generated.go'),'oracle_policy':ref(self.module/'policy.go'),
         'module_sources':self.module_refs,'go_tool':self.tools[0],'binary':binary}
        save_new(self.out/'generated-boundary.json',bound);self.boundary_ref=ref(self.out/'generated-boundary.json');self.runtime_refs.append(self.boundary_ref)
        self.rows_path=self.out/'full12288-actual-rows.jsonl.gz';checked=self.out/'full12288-checked.json'
        self.call('full12288-generated-native',[binary['path'],'--generated-boundary',self.boundary_ref['path'],'--rows',str(self.rows_path),'--summary',str(checked)],cwd=self.module)
        self.row_info=check_rows(self.rows_path,self.contract);self.checked_ref=ref(checked)
        check_summary(load(checked),self.row_info,self.boundary_ref)
        self.full_record={'checked':self.checked_ref,'rows':self.row_info,'boundary':self.boundary_ref,'generated':self.generated_ref,'origin':self.origin_ref}

    def tampers(self):
        record=self.records[0];base=['result','runtime','traces',0,'deliveries',0]
        for name,target,diagnostic in TAMPERS:
            raw=Path(record['execution' if target=='execution' else 'replay']['path']).read_bytes()
            if name in ('derivedReason','originalEcho'):
                field='reason' if name=='derivedReason' else 'method';index=11 if name=='derivedReason' else 0
                raw=H.changed_value(raw,base+['actual',field],'CONTROL_MUTATION')
                raw=H.changed_value(raw,base+['actual_fields',index,'value'],'CONTROL_MUTATION')
            elif name=='integerKind':
                raw=raw_token(raw,base+['actual','attempts_if_claimed'],'1.0');raw=raw_token(raw,base+['actual_fields',13,'value'],'1.0')
            elif name=='missingRequiredField':
                raw=without_member(raw,base+['actual'],'reason')
            elif name=='inputIntegerKind':raw=raw_token(raw,base+['inputs',7,'value'],'1.0')
            elif name.startswith('pureCalls'):raw=H.wrong_pure_calls(raw,{'pureCallsFalse':False,'pureCallsString':'','pureCallsArray':[],'pureCallsObject':{}}[name])
            else:
                paths={'fieldID':(base+['actual_fields',0,'id'],'gooo://mutation'),
                 'inputTextPort':(base+['inputs',0,'value'],'CONTROL_MUTATION'),
                 'sourceImage':(['result','program','workspace_image','packages',0,'sources',0,'source_digest'],'sha256:'+'0'*64),
                 'generationBody':(['result','composition','steps',0,'generation','report','program_digest'],'sha256:'+'0'*64),
                 'typedPlan':(['result','composition','plan','typed_plan_sha256'],'sha256:'+'0'*64),
                 'producerRevision':(['result','runtime','producer_source_sha'],'0'*40),
                 'inputFileDigest':(['inputs_digest'],'sha256:'+'0'*64),
                 'rawReplayParent':(['replayed_from_sha256'],'sha256:'+'0'*64),
                 'innerPriorResult':(['result','replay','prior_result_sha256'],'sha256:'+'0'*64)}
                path,value=paths[name];raw=H.changed_value(raw,path,value)
            path=self.out/('tamper-'+name+'.json')
            with path.open('xb') as f:f.write(raw)
            tamper_ref=ref(path);self.runtime_refs.append(tamper_ref)
            changed=dict(record);changed['execution' if target=='execution' else 'replay']=tamper_ref
            self.call('tamper-'+name,self.observe_args(changed,self.out/('tamper-'+name+'-unused.json')),30,expected=1,substring=diagnostic)
            if (self.out/('tamper-'+name+'-unused.json')).exists():raise ValueError('Tamper produced fabricated checked report')

    def audit_retained(self,producers):
        self.frozen(producers)
        if len(self.stages)!=70 or len(self.specs)!=70 or [s['name'] for s in self.specs]!=self.stage_names:raise ValueError('Incomplete ordered70stage proof')
        for item,spec in zip(self.stages,self.specs):
            if ref(Path(item['path']))!=item:raise ValueError('Process ledger changed')
            p=load_process(Path(item['path']))
            for key,want in [('name',spec['name']),('argv',spec['argv']),('cwd',spec['cwd']),('exit_code',spec['expected']),('expected_exit_code',spec['expected']),('automatic_retries',0),('status','COMPLETED'),('process_reaped',True),('owned_group_remaining',False),('owned_group_state',{'state':'absent','error':None}),('primary_error',None),('abort',None),('secondary_errors',[]),('termination_errors',[]),('source_tools_unchanged_after',True),('source_freeze_before',self.freeze),('source_freeze_after',self.freeze)]:
                if not typed_equal(p.get(key),want):raise ValueError('Retained process identity/ownership differs: '+key)
            started=p['started_ledger']
            if ref(Path(started['path']))!=started:raise ValueError('STARTED bytes changed')
            s=load_process(Path(started['path']))
            if s.get('status')!='STARTED' or any(not typed_equal(s.get(k),p.get(k)) for k in ('name','pid','owned_pgid','argv','cwd','source_freeze_before','timeout_seconds','expected_exit_code','automatic_retries')):raise ValueError('STARTED identity differs')
            if type(p.get('pid')) is not int or p['pid']<=0 or type(p.get('owned_pgid')) is not int or p['pid']!=p['owned_pgid']:raise ValueError('Owned PID differs')
            for key,cap in [('stdout',STDOUT_CAP),('stderr',STDERR_CAP)]:
                r=p[key]
                if ref(Path(r['path']))!=r or r['bytes']>cap:raise ValueError('Retained log changed/exceeds cap')
            if spec['diagnostic'] is not None or spec['substring'] is not None:
                error=Path(p['stderr']['path']).read_text().strip()
                if p['stdout']['bytes']!=0 or (error!=spec['diagnostic'] if spec['diagnostic'] is not None else spec['substring'] not in error):raise ValueError('Intended negative diagnostic differs')
        validated_oracle(load(Path(self.oracle_ref['path'])),self.witness,self.contract,self.contract_ref['sha256'])
        for record in self.records:
            for key in ('inputs','execution','replay','checked','execute_process','replay_process','observe_process'):
                if ref(Path(record[key]['path']))!=record[key]:raise ValueError('Manual retained binding changed')
            cases=self.oracle_cases[record['offset']:record['offset']+8]
            for key in ('execution','replay'):compare_raw_receipt(Path(record[key]['path']),cases,key,self.contract)
        if ref(Path(self.checked_ref['path']))!=self.checked_ref:raise ValueError('Full checked report changed')
        rows=check_rows(self.rows_path,self.contract)
        if not typed_equal(rows,self.row_info):raise ValueError('Full retained rows changed')
        check_summary(load(Path(self.checked_ref['path'])),rows,self.boundary_ref)


def execute_attempt(args):
    attempt=None;summary=None;primary=None;secondary=[]
    try:attempt=Attempt(args);attempt.prepare();summary=attempt.run()
    except BaseException as exc:primary=exc
    if attempt is not None:
        try:attempt.cleanup()
        except BaseException as exc:
            secondary.append({'operation':'owned-work-cleanup','error':error_detail(exc)})
            if primary is None:primary=exc
    if primary is None:
        try:
            attempt.audit_retained(False)
            if not attempt.work_cleanup_complete or attempt.work.exists() or time.monotonic()-attempt.started>=attempt.overall:raise RuntimeError('Final cleanup/deadline unconfirmed')
            summary['status']='FRESH_MANUAL16_REPLAY_AND_FULL12288_GENERATED_NATIVE_PASS'
            summary['work_cleanup']=ref(attempt.out/'work-cleanup.json')
            raw=(json.dumps(summary,indent=2)+'\n').encode()
            if logical(attempt.out)+len(raw)>PROOF_CAP or shutil.disk_usage(attempt.out).free<FREE_MIN:raise RuntimeError('Final proof/free-space cap')
            with (attempt.out/'summary.json').open('xb') as f:f.write(raw)
        except BaseException as exc:primary=exc
    if primary is not None:
        failure={'status':'FAILED_OR_INTERRUPTED','primary_error':{'type':type(primary).__name__,'repr':repr(primary)},'secondary_errors':secondary+getattr(attempt,'secondary_errors',[]),
          'stages':getattr(attempt,'stages',[]),'automatic_retries':0,'unpersisted_process':getattr(attempt,'pending_process',None),
          'last_group_observations':getattr(attempt,'last_group_observations',[]),'owned_work_path':str(attempt.work) if attempt and attempt.work else None,
          'owned_cleanup_confirmed':getattr(attempt,'work_cleanup_complete',False)}
        if attempt is not None and attempt.owns_out:
            try:save_new(attempt.out/'failure.json',failure)
            except BaseException as exc:
                failure['secondary_errors'].append({'operation':'failure-write','error':repr(exc)})
                try:sys.stderr.write(json.dumps(failure)+'\n')
                except BaseException:pass
        raise primary.with_traceback(primary.__traceback__)
    return summary


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    for key in ('repository','go','compiler','out'):parser.add_argument('--'+key,required=True,type=Path)
    parser.add_argument('--timeout',type=int,default=300);args=parser.parse_args()
    for sig in (signal.SIGTERM,signal.SIGINT,signal.SIGHUP):signal.signal(sig,lambda n,_:(_ for _ in ()).throw(SystemExit(128+n)))
    execute_attempt(args)


if __name__=='__main__':main()
