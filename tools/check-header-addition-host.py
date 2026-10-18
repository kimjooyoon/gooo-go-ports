#!/usr/bin/env python3
"""Fresh bound generated Gooo → local atomic host check. Stdlib only.

Read legacy full-run timing or an explicit experimental shard reference without
executing tools. Both routes share independent Go case-document ABI expectations.
"""
import argparse, hashlib, importlib.util, json, os, pathlib, re, shutil, signal, stat, subprocess, sys, tempfile, time

# Explicit sibling loading works under Python -I without cwd/sys.path imports.
_CONTRACT_SPEC=importlib.util.spec_from_file_location("gooo_native_verification_contract",pathlib.Path(__file__).with_name("native_verification_contract.py"))
CONTRACT=importlib.util.module_from_spec(_CONTRACT_SPEC)
sys.modules[_CONTRACT_SPEC.name]=CONTRACT
_CONTRACT_SPEC.loader.exec_module(CONTRACT)

REV='83928fa4538245abe33b08549da47f8cbdc50ed2'
ROOT='app/diagnostic-header-addition'
ENTRY=ROOT+':AssessAddition'
RID='gooo://diagnostic-header-addition/request/v1'
OID='gooo://diagnostic-header-addition/result/v1'
EXAMPLE='examples/header-addition-host'
TEMPLATES=['bindings.go.txt','bindings_test.go.txt','go.mod.txt','host.go.txt','host_test.go.txt','main.go.txt']
OWN_FILES=[EXAMPLE+'/'+x for x in ['LICENSE','NOTICE','README.md','provenance.json',*TEMPLATES]]+['tools/check-header-addition-host.py','tools/check-header-addition-host_test.py']
PACKAGES=[('go/golang.org/x/net/http/httpguts','ports/net/httpguts/headers.gooo'),('app/header-field-facts','examples/header-field-facts/main.gooo'),('go/strconv','ports/strconv/decimal.gooo'),('app/payload-admission','examples/payload-admission/main.gooo'),(ROOT,'examples/diagnostic-header-addition/main.gooo')]

# Preserve the independently stated owners from the original suites74..81
# binding checks. A coordinated catalog/event relabel must not redefine them.
APPENDED_SOURCE_OWNERS = {
    'ports/net/httpguts/headers.gooo': 'go/golang.org/x/net/http/httpguts',
    'examples/header-field-facts/main.gooo': 'app/header-field-facts',
    'ports/net/textproto/canonical.gooo': 'go/net/textproto',
    'examples/canonical-header-key/main.gooo': 'app/canonical-header-key',
    'ports/net/http/etag.gooo': 'go/net/http/etag',
    'examples/strong-validator-facts/main.gooo': 'app/strong-validator-facts',
    'ports/net/http/precondition.gooo': 'go/net/http/precondition',
    'ports/strings/search.gooo': 'go/strings',
    'ports/strings/last_index.gooo': 'go/strings',
    'ports/strings/literal.gooo': 'go/strings',
    'examples/literal-final-marker-report/main.gooo': 'app/literal-final-marker-report',
}

CANONICAL_SUMMARY = {
    "canonical_header_reference_cases": 1059,
    "canonical_header_report_reference_cases": 1059,
    "canonical_header_raw_input_occurrences": 1059,
    "canonical_header_unique_keys": 1054,
    "canonical_header_within_partition_duplicates": 0,
    "canonical_header_cross_partition_duplicates": 5,
    "canonical_header_report_fields": 8,
    "canonical_header_port_source_count": 2,
    "canonical_header_report_source_count": 4,
    "canonical_header_scope": "Decoded validUTF8 Text0..1024bytes, including empty and invalid header names. Actual Go1.27.2 net/textproto.CanonicalMIMEHeaderKey supplies canonical Text; the importing original8-field Report retains exact original/canonical Text, changed/validity, byte lengths and first invalid offset/octet. All1059 ordered occurrences,1054 unique keys,zero within-partition and five cross-partition duplicates are retained per entry. No arbitrary-byte/unbounded Go string, HTTP parsing, authorization, effect, production or performance claim.",
    "canonical_header_provenance_scope": "Fresh native execution and saved replay bind original Text, exact typed8 result fields and order, current2/4-source closures, lowered source/record/callee ABI, producer identities and raw outer saved parents. The scalar expectation is the actual pinned Go standard library; Report diagnostics reuse the independent existing header reference. Independent inner typed canonical prior_result_sha256 remains NOT_CHECKED; private pilot receipts are not fresh public acceptance."
}

ETAG_SUMMARY = {
    "etag_scan_reference_cases": 192,
    "etag_strong_reference_cases": 31,
    "etag_report_reference_cases": 24,
    "etag_scan_unique_inputs": 190,
    "etag_scan_duplicate_occurrences": 2,
    "etag_report_fields": 13,
    "etag_port_source_count": 1,
    "etag_report_source_count": 3,
    "etag_scope": "Bounded decoded Unicode-scalar validUTF8 Text0..1024bytes per operand, including empty/NUL/control/invalidETag grammar. Private pinned Go scanETag/etagStrongMatch are exposed as dedicated Gooo APIs; the original13field Report separates raw header validity from parsed tag facts. All192 scanner occurrences,31 raw pairs and24 facts inputs retained. No arbitrary-byte/unbounded strings, fullIfMatch, currentresource authority, effects or production-safety claim.",
    "etag_provenance_scope": "Fresh native and saved replay bind original inputs, exact2/13 ordered typed fields, current1/3source closures, lowered bodies, generated ABI, Report3callee ordinals0/1/2, exact explicit Go binary and raw saved parent. All247 independent Go expectations are separately compared with literal anchors. Inner canonical/typed-plan hashing remains NOT_CHECKED; a label or private firstcase is not public acceptance."
}

IF_MATCH_SUMMARY = {'if_match_reference_cases': 284, 'if_match_input_ports': 2, 'if_match_result_fields': 1, 'if_match_port_source_count': 1, 'if_match_conditions': {'condNone': 0, 'condTrue': 1, 'condFalse': 2}, 'if_match_scope': 'Selected If-Match and current ETag are two unchanged decoded Unicode-scalar validUTF8 Text0..1024bytes each, including empty/NUL/control/invalid grammar. Private Go checkIfMatch compatibility returns original condNone0/condTrue1/condFalse2 as one required int64 field. Initial empty differs from whitespace-only; star suffix, adjacent tags and early strong match are preserved. No Header.Get or multiple-field selection, HTTP adapter, authenticated current version, authorization, effects or CAS guarantee.', 'if_match_provenance_scope': 'All284 ordered input occurrences are retained; fresh expectations come only from the pinned original Go enum/checkIfMatch adapter with three explicit lookup substitutions and unchanged scanETag/etagStrongMatch/TrimString. Fresh native execution and saved replay bind original two inputs, one ordered integer result field, current one-source closure, lowered body/record/generated ABI, exact explicit Go binary and raw saved parent. Independent inner canonical/typed-plan hashing remains NOT_CHECKED; historical private receipts do not establish current-source acceptance.'}

LAST_INDEX_SUMMARY = {
    'last_index_reference_cases': 1694,
    'final_marker_report_reference_cases': 89,
    'last_index_scope': 'LastIndex finite corpus v1: 1694 unique decoded valid-UTF8 Text pairs, each argument <=1024 bytes. Independent pinned Go strings.LastIndex preserves the final literal byte offset, overlapping final matches, -1 on misses and byte length for an empty needle. Includes26 separately stated anchors, complete21x21 short pairs over [a,b,NUL,é] of0..2tokens, all1024 final byte offsets, length boundaries and64 fixed-PCG candidates, deduplicated in order. No arbitrary-byte or longer Go strings, normalization, whole-package, exhaustive-string, performance or adoption claim.',
    'final_marker_report_scope': 'FinalMarkerReport v1 finite corpus: 89 unique representative pairs from26 manual anchors and64 fixed-PCG candidates. Go strings.Index/LastIndex/Count are independently called for all8 fields; original Texts and UTF8 byte lengths are preserved. LastIndex includes overlaps while Count remains non-overlapping. This separate v1 result does not extend or rebind the existing7-field literal-text Report, its9313 pairs, compact policy or saved evidence. No production enforcement or safety claim.',
    'last_index_finite_provenance_scope': 'Fresh LastIndex and separate FinalMarkerReport v1 suites compare independent Go oracle values and preserve original Texts, exact scalar/8-field result identities and order, selected source images, entry and producer identities, generated bytes and raw outer saved-receipt bindings. Inner prior_result_sha256, full generation-report provenance and composition/runtime typed-plan equality are NOT_CHECKED by this finite path. Existing literal-text Report seven-field ABI, all9313 original pairs and compact policy remain unchanged.'
}

def need(ok,message):
    if not ok: raise RuntimeError(message)
def sha(raw): return hashlib.sha256(raw).hexdigest()
def digest(raw): return 'sha256:'+sha(raw)
def load(raw):
    def pairs(items):
        result={}
        for key,value in items:
            need(key not in result,'duplicate JSON key: '+key);result[key]=value
        return result
    return json.loads(raw,object_pairs_hook=pairs,parse_constant=lambda _: (_ for _ in ()).throw(ValueError('nonfinite JSON')))
def equal(a,b):
    if type(a) is not type(b): return False
    if isinstance(a,dict): return a.keys()==b.keys() and all(equal(a[k],b[k]) for k in a)
    if isinstance(a,list): return len(a)==len(b) and all(equal(x,y) for x,y in zip(a,b))
    return a==b
def go_json(value):
    return json.dumps(value,ensure_ascii=False,sort_keys=True,separators=(',',':')).replace('<','\\u003c').replace('>','\\u003e').replace('&','\\u0026').replace('\u2028','\\u2028').replace('\u2029','\\u2029').encode('utf-8')
def canonical(path):
    p=pathlib.Path(path);need(p.is_absolute() and p.resolve()==p and not p.is_symlink(),'canonical absolute path required: '+str(p));return p
def safe_relative(name):
    need(type(name) is str and name and not name.startswith('/') and '\\' not in name and all(x not in ('','..','.') for x in name.split('/')),'unsafe relative source path');return name
def reference(raw): return {'bytes':len(raw),'sha256':sha(raw)}
def save(path,value):
    with path.open('x',encoding='utf-8') as f: json.dump(value,f,ensure_ascii=False,indent=2);f.write('\n')
def frozen_read(path,pins):
    p=canonical(path);need(p.is_file(),'regular input required: '+str(p));raw=p.read_bytes();ref=reference(raw)
    need(str(p) not in pins or equal(pins[str(p)],ref),'input changed during read: '+str(p));pins[str(p)]=ref;return raw
def unchanged(pins):
    for name,ref in pins.items():
        p=canonical(name);need(p.is_file() and equal(reference(p.read_bytes()),ref),'frozen source/tool drift: '+name)

class PinScanObservations:
    """Coarse diagnostics only; the existing validation callbacks stay intact."""
    def __init__(self):
        self.output=None;self.observations=[];self.write_error=None;self._write_attempted=False;self.cancellation=None

    def is_cancellation(self,failure):
        return not isinstance(failure,Exception) or failure is self.cancellation

    def observe(self,label,boundary,operation,pins=None):
        count,total=None,None
        if pins is not None:
            try:
                count=len(pins)
                sizes=[ref['bytes'] for ref in pins.values()]
                if all(type(size) is int and size>=0 for size in sizes):total=sum(sizes)
            except Exception as failure:
                if self.is_cancellation(failure):raise
                # Metadata collection must not replace the validation's refusal.
                count,total=None,None
        row={'label':label,'boundary':boundary,'declared_pin_count':count,
             'declared_logical_bytes':total,'state':'STARTED','elapsed_ns':None,
             'timing_available':False}
        self.observations.append(row)
        try:started=time.monotonic_ns()
        except BaseException as failure:
            if self.is_cancellation(failure):raise
            started=None
        refused=False
        try:
            result=operation()
        except BaseException:
            refused=True;row['state']='REFUSED';raise
        else:
            row['state']='COMPLETED';return result
        finally:
            try:ended=time.monotonic_ns()
            except BaseException as failure:
                if not refused and self.is_cancellation(failure):raise
                ended=None
            if type(started) is int and type(ended) is int and ended>=started:
                row['elapsed_ns']=ended-started;row['timing_available']=True

    def write_once(self,preserve_failure=False):
        if self.output is None or self._write_attempted:return
        self._write_attempted=True
        document={'schema':'gooo/pin-scan-observations/v1','diagnostic_only':True,
                  'observations':self.observations,
                  'scope':'Coarse callback wall intervals. Initial-verification is a parent span containing shared verification. Declared pin counts and logical bytes are not actual read bytes on refusal or physical I/O; durations do not isolate path, read or SHA work.'}
        try:save(self.output/'pin-scans.json',document)
        except BaseException as failure:
            if not preserve_failure and self.is_cancellation(failure):raise
            # A prior validation failure takes precedence over secondary diagnostics.
            self.write_error={'type':type(failure).__name__,'message':str(failure)}
def reviewed_contract(root,read_bytes=None):
    try:return CONTRACT.load_reviewed_contract(root,read_bytes=read_bytes)
    except (OSError,CONTRACT.ContractError) as error:raise RuntimeError('reviewed native contract: '+str(error)) from error

def source_roster(root,catalog,expected=None):
    expected=reviewed_contract(root) if expected is None else expected
    names={row['path'] for row in expected.document['sources']}
    qualified={row['path'] for row in expected.document['sources'] if 'catalog-source' in row['roles']}
    need(type(catalog) is dict and type(catalog.get('packages')) is list,'current catalog packages')
    current=[]
    for package in catalog['packages']:
        need(type(package) is dict and type(package.get('path')) is str and type(package.get('sources')) is list,'current catalog package/source ownership')
        current.extend(safe_relative(name) for name in package['sources'])
    need(len(current)==len(set(current)) and set(current)==qualified,'exact reviewed catalog-source membership/ownership')
    need(set(OWN_FILES).issubset(names),'all host templates/checker must be in current timing source identity')
    need(set(CONTRACT.CONTRACT_FILES).issubset(names),'all five contract inputs must be in current timing source identity')
    need('cmd/build-demo/api_policy_build.go' in names,'API policy builder continuation is a required timing input')
    return names

def validate_timing_sources(root,catalog,sources,read,expected=None):
    expected=reviewed_contract(root,read) if expected is None else expected
    roster=source_roster(root,catalog,expected)
    need(type(sources) is dict and len(roster)==expected.source_count and set(sources)==roster,'exact reviewed full native timing source roster')
    for name in sorted(roster):
        need(equal(reference(read(root/name)),sources[name]),'current timing source bytes: '+name)

def validate_command_resources(header,batches):
    """Check optional wait accounting, independently of semantic verification.

    This current-source acceptance reader requires the new schema and both CLI
    observations. Historical timing without accounting is not current evidence.
    NOT_AVAILABLE is valid accounting coverage; a value is never a resource limit,
    an effect authorization or a substitute for receipt/replay verification.
    """
    need(type(header) is dict and header.get('command_resources_schema')=='gooo/native-command-resources/v1','current command resources schema')
    goos=header.get('goos');need(type(goos) is str and goos.strip(),'command resources header GOOS')
    need(type(batches) is list,'command resources batch list')
    metrics=('user_cpu_ns','system_cpu_ns','maxrss_raw','maxrss_bytes')
    fields={'status','goos','scope','maxrss_unit',*metrics}
    for batch in batches:
        need(type(batch) is dict,'command resources batch object')
        resources=batch.get('command_resources')
        need(type(resources) is dict and set(resources)=={'execute','replay'},'both exact execute/replay command resources')
        for phase in ('execute','replay'):
            value=resources[phase];need(type(value) is dict,'command resources '+phase+' object')
            status=value.get('status')
            need(status in ('OBSERVED','NOT_AVAILABLE'),'current command resources '+phase+' wait coverage')
            need(set(value)==fields|({'reason'} if status=='NOT_AVAILABLE' else set()),'exact command resources '+phase+' fields')
            need(value['goos']==goos and value['scope']=='waited-command-accounting','command resources '+phase+' GOOS/scope binding')
            if status=='NOT_AVAILABLE':
                need(type(value['reason']) is str and value['reason'].strip(),'command resources '+phase+' unavailable reason')
                need(all(value[key] is None for key in (*metrics,'maxrss_unit')),'unavailable command resources '+phase+' are null')
                continue
            need(all(type(value[key]) is int and 0<=value[key]<=9223372036854775807 for key in metrics),'observed command resources '+phase+' bounded integer accounting')
            need(goos in ('linux','darwin'),'observed command resources '+phase+' supported wait units')
            unit,scale=('KiB',1024) if goos=='linux' else ('bytes',1)
            need(value['maxrss_unit']==unit and value['maxrss_bytes']==value['maxrss_raw']*scale,'observed command resources '+phase+' exact OS RSS conversion')

ACTIVITY = re.compile(r'activity\s+(\w+)\(([^)]*)\)\s*->\s*(\w+)\s+id\s+"([^"]+)"\s+computes\s*(?:`([^`]*)`|("(?:\\.|[^"\\])*"))',re.S)
RECORD = re.compile(r'entity\s+(\w+)\s+id\s+"([^"]+)"\s+fields\s*\{(.*?)\}',re.S)
FIELD = re.compile(r'field\s+(\w+)\s+id\s+"([^"]+)"\s+type\s+(string|integer|boolean)\s+required\s+one')
def activities(text):
    out=[]
    for m in ACTIVITY.finditer(text):out.append({'name':m[1],'inputs':[x.strip() for x in m[2].split(',')],'output':m[3],'id':m[4],'body':m[5] if m[5] is not None else load(m[6])})
    need(len(out)==len(re.findall(r'(?m)^activity\s+',text)),'unparsed source activity');return out
def contracts(sources):
    definitions={};records={};ordinal=0
    for pkg,filename,text in sources:
        imports=dict(re.findall(r'^import\s+(\w+)\s+"([^"]+)"',text,re.M))
        for d in activities(text):
            d.update(package=pkg,source=filename,lowered='GoooPackage'+str(ordinal)+'Activity'+d['name'],imports=imports);ordinal+=1;definitions[pkg+':'+d['name']]=d
        for name,ident,body in RECORD.findall(text):
            fs=[{'name':n,'id':i,'type_id':'urn:gooo:type:'+t,'go_name':'GoooField'+sha(i.encode())} for n,i,t in FIELD.findall(body)]
            need(ident not in records and 0<len(fs)<=16,'record roster/limit');records[ident]={'name':name,'id':ident,'go_name':'GoooRecord'+sha(ident.encode()),'fields':fs}
    need(len(definitions)==6 and len(records)==5,'exact six activity/five record source closure')
    for key,d in definitions.items():
        body=d['body'];sites=[]
        for m in re.finditer(r'\b([a-z]\w*)\.([A-Z]\w*)\s*(?=\()',body):
            need(m[1] in d['imports'],'unknown source callee alias');callee=d['imports'][m[1]]+':'+m[2];need(callee in definitions,'unknown pure callee')
            sites.append({'caller':key,'callee':callee,'surface':'computes','start_byte':len(body[:m.start()].encode()),'end_byte':len(body[:m.end()].encode())})
        d['sites']=sites;d['lowered_body']=body
        for site in sites:
            target=definitions[site['callee']];alias=next(a for a,p in d['imports'].items() if p==target['package']);d['lowered_body']=d['lowered_body'].replace(alias+'.'+target['name'],target['lowered'])
    need(len(definitions[ENTRY]['sites'])==2,'root two actual composition calls');return definitions,records
def record_fields(records,ident,value):return [{'id':f['id'],'name':f['name'],'value':value[f['name']]} for f in records[ident]['fields']]
def receipt(raw,rows,cases,manifest,images,go_version,go_sha,definitions,records,producer_manifest_path,prior=None):
    DEFS,RECORDS=definitions,records
    r=load(raw);need(r['schema']=='gooo/workspace-body-execution-receipt/v1' and r['decision']=='PASS' and not r.get('error'),'producer PASS envelope')
    need(r['manifest']==producer_manifest_path and r['manifest_digest']=='sha256:'+sha(manifest) and r['cases_digest']=='sha256:'+sha(cases) and 'inputs_digest' not in r,'exact manifest/case bytes')
    result=r['result'];p=result['program'];c=result['composition'];rt=result['runtime'];root=DEFS[ENTRY]
    ref={'package_path':ROOT,'activity':'AssessAddition','lowered_name':root['lowered']}
    need(result['schema']=='gooo/workspace-body-execution/v1' and p['schema']=='gooo/workspace-body-program/v1' and equal(p['entry'],ref) and equal(p['activities'],[ref]),'single exact original/lowered root')
    image=p['workspace_image'];need(image['schema']=='gooo/package-runtime-image/v1' and equal(image['init_order'],[x['package'] for x in images]),'exact five-package order')
    seen=[{'package':pkg['path'],'filename':s['filename'],'source_digest':s['source_digest']} for pkg in image['packages'] for s in pkg['sources']]
    need(equal(seen,images) and len(image['packages'])==5,'actual Go JSON-string source-image digests')
    need(equal(image['entry'],{'package_path':ROOT,'source':root['source'],'activity':'AssessAddition','inputs':['Text','Text','Text','RequestFacts'],'output':'HeaderAdditionDecision'}),'image typed entry')
    lowered=p['lowered_gooo_source'];actual=activities(lowered);need(len(actual)==6,'exact lowered callee source roster')
    for d in DEFS.values():
        a=[a for a in actual if a['name']==d['lowered']];need(len(a)==1 and all(equal(a[0][k],d[k]) for k in ['id','inputs','output']) and a[0]['body']==d['lowered_body'],'source-resolved lowered body/signature')
    pure=p['pure_calls'];need(pure['schema']=='gooo/workspace-pure-calls/v1','pure source schema')
    calls={x['activity']['package_path']+':'+x['activity']['activity']:x for x in pure['activities']};need(set(calls)==set(DEFS) and len(calls)==len(pure['activities']),'exact pure participant closure')
    for key,d in DEFS.items():
        x=calls[key];need(equal(x['activity'],{'package_path':d['package'],'activity':d['name'],'lowered_name':d['lowered']}) and x['source']==d['source'] and x['source_body_sha256']=='sha256:'+sha(d['body'].encode()),'source pure body identities')
    queue=[ENTRY];sites=[]
    for key in queue:
        for site in DEFS[key]['sites']:
            sites.append(site)
            if site['callee'] not in queue:queue.append(site['callee'])
    need(equal(pure['sites'],sites) and len(sites)==5,'exact source call sites and five transitive edges')
    source_sha='sha256:'+sha(lowered.encode());need(c['schema']=='gooo/body-composition/v1' and c['stage']=='COMPLETE' and c['gooo_source']==lowered and c['original_source_sha256']==c['selected_source_sha256']==result['execution_source_sha256']==source_sha,'lowered composition source binding')
    plan=c['plan'];inputs=[{'port':'input'+str(i),'type':'Text' if i<3 else 'RequestFacts','entity_id':'gooo://std/text' if i<3 else RID,'from':-1} for i in range(4)]
    need(plan['schema']=='gooo/body-composition-plan/v1' and plan['entry_activity']==root['lowered'] and equal(plan['edges'],[]) and len(plan['activities'])==1,'typed no-effect single-node plan')
    a=plan['activities'][0];need(equal(a['inputs'],inputs) and a['name']==root['lowered'] and a['id']==root['id'] and a['input_type']=='Text' and a['output_type']=='HeaderAdditionDecision' and a['input_entity_id']=='gooo://std/text' and a['output_entity_id']==OID and a['input_from']==-1 and a['assembling'] is False and a.get('prepared',False) is False and a['go_function']=='GoooComposedActivity0' and not plan.get('preparations') and not p.get('entity_aliases'),'typed caller/return ABI')
    records={x['id']:x for x in plan['record_types']};need(len(records)==len(plan['record_types']) and equal(records,RECORDS),'exact five typed record identities/ordered fields')
    need(len(c['steps'])==1,'one actual source generation');generation=c['steps'][0]['generation'];report=generation['report'];closure=report['call_closure']
    need(report['schema']=='gooo/body-codegen-report/v3' and report['decision']=='PASS' and report['activity']==root['lowered'] and report['activity_id']==root['id'] and report['compiler_source_sha']==REV and report['typecheck_passed'] is True and report['deterministic_replay'] is True,'native source generation producer')
    need(c['steps'][0]['input_source_sha256']==source_sha and report['source_digest']==source_sha and report['program_digest']=='sha256:'+sha(root['lowered_body'].encode()) and report['generated_digest']=='sha256:'+sha(generation['source'].encode()) and report['replay_digest']==report['generated_digest'] and equal(report['repository_writes'],0),'generation actual source/program/output digest binding')
    generated_records={x['id']:x for x in report['record_types']};parameters=[{'name':'input'+str(i),'type':'string' if i<3 else 'RequestFacts'} for i in range(4)]
    need(len(generated_records)==len(report['record_types']) and equal(generated_records,RECORDS) and equal(report['input_parameters'],parameters) and report['input_type']=='(string,string,string,RequestFacts)' and report['output_type']=='HeaderAdditionDecision','generated full record and four-parameter typed ABI')
    actual_callees={x['name']:x for x in closure['activities']};wanted={d['lowered']:{'name':d['lowered'],'activity_id':d['id'],'program_sha256':'sha256:'+sha(d['lowered_body'].encode())} for key,d in DEFS.items() if key!=ENTRY}
    need(closure['schema']=='gooo/pure-activity-call-closure/v1' and len(actual_callees)==len(closure['activities']) and equal(actual_callees,wanted) and equal(closure['max_calls_per_invocation'],5),'exact typed transitive callee program closure')
    edge_ordinals={ENTRY:[0,1],'app/header-field-facts:Report':[8,9],'app/payload-admission:Report':[0]};edges=[]
    for key,ordinals in edge_ordinals.items():
        for site,ordinal in zip(DEFS[key]['sites'],ordinals):edges.append({'caller_id':DEFS[key]['id'],'callee_id':DEFS[site['callee']]['id'],'ordinal':ordinal})
    order=lambda e:(e['caller_id'],e['ordinal'],e['callee_id'])
    need(equal(sorted(closure['edges'],key=order),sorted(edges,key=order)),'exact typed call-edge identities/ordinals')
    need(rt['schema']=='gooo/body-composition-runtime/v1' and rt['stage']=='COMPLETE' and not rt.get('failure') and equal([rt['finite_total'],rt['finite_passed'],rt['model_calls']],[len(rows),len(rows),0]) and rt['projection_replayed'] is True and rt['runtime_replayed'] is True,'finite fresh native runtime')
    need(rt['producer_source_sha']==REV and rt['go_version']==go_version and rt['go_tool_selection'] in ['explicit','path','compiler_goroot','toolchain_cache','path_fallback'] and rt['go_tool_sha256']=='sha256:'+go_sha,'exact producer and observed Go binary')
    need(rt['original_source_sha256']==source_sha and rt['selected_source_sha256']==source_sha and rt['typed_plan_sha256']==plan['typed_plan_sha256'],'runtime source/typed-plan crossbinding')
    need(c['generated_sha256']=='sha256:'+sha(c['source'].encode())==rt['generated_sha256'] and c['driver_sha256']=='sha256:'+sha(c['driver_source'].encode())==rt['driver_sha256'],'actual generated/driver source crossbinding')
    need(len(rt['traces'])==len(rows),'ordered finite trace count')
    output_fields=RECORDS[OID]['fields'];types={'string':str,'boolean':bool,'integer':int}
    for i,(trace,row) in enumerate(zip(rt['traces'],rows)):
        expected=row['expected'][ENTRY];need(set(expected)=={f['name'] for f in output_fields} and all(type(expected[f['name']]) is types[f['type_id'].split(':')[-1]] for f in output_fields) and all(-(1<<63)<=v<(1<<63) for v in expected.values() if type(v) is int),'all16 reference scalar types/ranges')
        need(equal(trace['case_index'],i) and len(trace['deliveries'])==1,'one ordered root delivery');d=trace['deliveries'][0]
        supplied=[{'port':'input'+str(j),'entity_id':'gooo://std/text' if j<3 else RID,'value':row['inputs'][ENTRY+'.input'+str(j)]} for j in range(4)];supplied[3]['fields']=record_fields(RECORDS,RID,supplied[3]['value'])
        need(d['activity_id']==root['id'] and equal(d['inputs'],supplied),'all typed scalar/12-field original record inputs')
        need(d['passed'] is True and equal(d['actual'],expected) and equal(d['expected'],expected) and equal(d['actual_fields'],record_fields(RECORDS,OID,expected)),'all16 typed actual/expected values and ordered field IDs')
    if prior is not None:need(r['replayed_from_sha256']=='sha256:'+sha(prior) and result['replay']['schema']=='gooo/workspace-body-replay/v1' and equal(result['replay']['model_calls'],0),'exact saved raw outer parent')
    return {'receipt_sha256':sha(raw),'cases':len(rows),'typed_result_fields':len(rows)*16,'typed_input_record_fields':len(rows)*12,'source':c['source'],'plan':plan,'tool_selection':rt['go_tool_selection']}



def current_population(summary,planned,ends,expected=None):
    """Compare every current suite to reviewed definitions, never actual totals.

    suite_end.source_set is a map, so this checks exact closure membership, not
    an observed manifest order. The reviewed row preserves that ordered source
    closure; the workspace/profile verifier independently checks actual order.
    """
    expected=reviewed_contract(pathlib.Path(__file__).resolve().parents[1]) if expected is None else expected
    runs=summary['runs'];rows=expected.document['suites'];count=expected.suite_count
    need(type(runs) is list and type(planned) is list and type(ends) is list,'current population lists')
    need(len(runs)==len(planned)==len(ends)==count and equal(summary['cases'],expected.total_cases) and equal(summary['passed'],expected.total_cases),'fresh complete reviewed population')
    prefix=rows[:expected.document['prefix']['count']]
    need(sum(r['cases'] for r in runs[:len(prefix)])==sum(r['cases'] for r in prefix) and sum(r['batches'] for r in runs[:len(prefix)])==sum(r['batches'] for r in prefix),'unchanged complete80 prefix population')
    # Independent historical anchors remain in addition to the reviewed row
    # equality. They cannot turn a partial old population into current evidence.
    need(sum(r['cases'] for r in runs[:79])==173582 and sum(r['batches'] for r in runs[:79])==2247,'unchanged complete79 prefix population')
    need(sum(r['cases'] for r in runs[:76])==173335 and sum(r['batches'] for r in runs[:76])==2240,'unchanged complete76 prefix population')
    need(sum(r['cases'] for r in runs[:74])==171217 and sum(r['batches'] for r in runs[:74])==2181,'unchanged complete74 prefix population')
    for contract in (CANONICAL_SUMMARY,ETAG_SUMMARY,IF_MATCH_SUMMARY,LAST_INDEX_SUMMARY):
        for key,value in contract.items():need(equal(summary.get(key),value),'preserved summary field/count/scope: '+key)
    indices=[]
    for end in ends:
        need(type(end) is dict and type(end.get('suite_index')) is int,'exact suite end index type')
        indices.append(end['suite_index'])
    need(len(set(indices))==count and set(indices)==set(range(count)),'every reviewed suite ends exactly once')
    by_index={end['suite_index']:end for end in ends}
    for row in rows:
        i=row['index'];r=runs[i]
        need(type(r) is dict,'suite observation object')
        need(equal({key:r.get(key) for key in ('package','activity','cases','passed','batches','replay_equal')},
             {'package':row['package'],'activity':row['activity'],'cases':row['cases'],'passed':row['cases'],'batches':row['batches'],'replay_equal':True}),'exact reviewed original suite '+str(i))
        need(equal(planned[i],{key:row[key] for key in ('index','package','activity','cases')}),'exact reviewed planned suite '+str(i))
        actual=by_index[i].get('source_set')
        need(type(actual) is dict and set(actual)==set(row['source_paths']),'exact reviewed source closure membership '+str(i))

def validate_suite_sources(root,catalog,ends,read,expected):
    """Check every selected map against current catalog ownership and bytes.

    Maps do not observe manifest source order. That separate workspace/profile
    check is retained by the native verifier; reviewed source_paths keep order.
    """
    catalog_owners={name:pkg['path'] for pkg in catalog['packages'] for name in pkg['sources']}
    for row in expected.document['suites']:
        for name in row['source_paths']:
            if name in APPENDED_SOURCE_OWNERS:
                need(catalog_owners.get(name)==APPENDED_SOURCE_OWNERS[name],
                     'independent appended source owner: '+name)
        # Source-set maps have sorted wire keys; do not call that manifest order.
        texts=[(catalog_owners[name],name,read(root/name).decode('utf-8')) for name in row['source_paths']]
        selected={name:{'Package':pkg,'Digest':digest(go_json(text))} for pkg,name,text in sorted(texts,key=lambda x:x[1])}
        event=next(e for e in ends if equal(e['suite_index'],row['index']))
        need(equal(event['source_set'],selected) and event['source_set_sha256']==digest(json.dumps(selected,ensure_ascii=False,separators=(',',':')).encode()),'reviewed actual current source-map/digest '+str(row['index']))

def read_verification(root,verification_out,timing):
    """Read-only complete public run binding. No compiler, Go, subprocess or writes.

    rows are the original fresh native testCaseMap documents, never reconstructed
    from Gooo actuals. pins includes every byte this function relies upon.
    """
    root,verification_out,timing=map(canonical,(root,verification_out,timing));pins={}
    need(root.is_dir() and verification_out.is_dir() and timing.parent==verification_out,'current run paths')
    read=lambda p:frozen_read(p,pins)
    contract_expected=reviewed_contract(root,read)
    events=[load(line) for line in read(timing).splitlines()]
    need(len(events)>2 and all(equal(e.get('seq'),i) and e.get('schema')=='gooo/native-verification-timing/v1' for i,e in enumerate(events)),'complete ordered timing event stream')
    header,footer=events[0],events[-1]
    need(header['kind']=='header' and footer['kind']=='run_end','timing header/footer')
    need(header['compiler_revision']==REV and header['go_version']=='go1.27.2','timing compiler/Go pins')
    need(header['workspace_profile']['name']==contract_expected.document['profiles']['workspace'] and header['case_profile']['name']==contract_expected.document['profiles']['case'],'current workspace/case profiles')
    summary=load(read(verification_out/'summary.json'));catalog_raw=read(root/'verification-catalog.json');catalog=load(catalog_raw)
    lock=load(read(root/'upstream.json'))
    need(lock['compiler_revision']==REV and lock['go_version']=='go1.27.2','current upstream pins')
    need(summary['schema']=='gooo/go-ports-differential/v1' and summary['compiler_revision']==REV and summary['go_version']=='go1.27.2','native summary producer')
    need(equal(summary['cases'],contract_expected.total_cases) and equal(summary['passed'],contract_expected.total_cases) and len(summary['runs'])==contract_expected.suite_count,'fresh complete reviewed-suite original summary')
    need(summary['verification_catalog_sha256']==digest(catalog_raw) and header['catalog']==reference(catalog_raw),'current catalog bytes')
    need(equal(summary['source_sha256'],header['catalog_source_sha256']),'summary/header source map')
    validate_timing_sources(root,catalog,header['identity']['source_files'],read,contract_expected)
    need(len(catalog['packages'])==56 and len(summary['source_sha256'])==67,'exact current56 packages/67 Gooo sources')
    catalog_sources={name:sha(read(root/name)) for pkg in catalog['packages'] for name in pkg['sources']}
    need(equal(summary['source_sha256'],catalog_sources),'current catalog Gooo source bytes')
    need(equal(header['upstream_pins'].get('compiler_revision'),REV) and equal(header['upstream_pins'].get('go_version'),'go1.27.2'),'header upstream compiler pins')
    compiler_raw=read(root/'.gooo');need(equal(reference(compiler_raw),header['identity']['binaries']['compiler']),'current actual compiler binary')
    need(set(header['identity']['binaries'])=={'compiler','go','verifier'},'three observed native tool identities')
    for ref in header['identity']['binaries'].values():need(type(ref['bytes']) is int and ref['bytes']>0 and re.fullmatch('[0-9a-f]{64}',ref['sha256']) is not None,'tool identity shape')
    need(footer['status']=='COMPLETE' and footer['complete'] is True and footer['aborted'] is False and footer['source_bindings_unchanged'] is True and footer['successful_close_return_required'] is True,'successful complete timing close')
    need(equal(footer['identity_after'],header['identity']) and equal(footer['event_count'],len(events)),'whole run source identity unchanged')
    need(len(events)==contract_expected.timing_events and equal(footer['seq'],contract_expected.timing_events-1),'exact reviewed timing row/footer count')
    for key in ['failed_suites','canceled_suites','not_started_suites','missing_suite_finishes','failed_batches','incomplete_batches','logger_errors']:need(equal(footer[key],0),'timing close zero '+key)
    need(equal(footer['not_started_suite_indices'],[]) and all(equal(value,contract_expected.suite_count) for value in (footer['expected_suites'],footer['successful_suites'],header['expected_suites'])) and all(equal(value,contract_expected.total_cases) for value in (header['expected_cases'],footer['verified_cases'])),'complete suite/case counts')
    planned=header['planned_suites'];need(type(planned) is list and len(planned)==contract_expected.suite_count and all(equal(p['index'],i) for i,p in enumerate(planned)),'reviewed planned suite order')
    beginnings=[e for e in events if e['kind']=='suite_begin'];ends=[e for e in events if e['kind']=='suite_end'];batches=[e for e in events if e['kind']=='batch_end']
    validate_command_resources(header,batches)
    need(len(beginnings)==len(ends)==contract_expected.suite_count and len(batches)==contract_expected.total_batches and all(equal(value,contract_expected.total_batches) for value in (footer['observed_batches'],footer['verified_batches'])),'all successful suite/batch events')
    need(set(e['suite_index'] for e in beginnings)==set(range(contract_expected.suite_count)) and set(e['suite_index'] for e in ends)==set(range(contract_expected.suite_count)),'every suite begins/ends once')
    need(set(e['kind'] for e in events)=={'header','suite_begin','batch_end','suite_end','run_end'},'only recognized complete timing events')
    for end in ends:
        i=end['suite_index'];p=planned[i];s=summary['runs'][i]
        need(end['status']=='VERIFIED' and end['returned_error'] is False and end['context_error'] is False and end['package']==p['package']==s['package'] and end['activity']==p['activity']==s['activity'],'successful planned original suite')
        need(equal(end['original_observation'],s) and end['covered_cases']==s['cases']==s['passed']==p['cases'] and s['replay_equal'] is True and s['batches']==end['observed_batches'],'original suite observation equality')
        bs=sorted((e for e in batches if e['suite_index']==i),key=lambda e:e['batch_index']);at=0
        need(len(bs)==s['batches'],'suite actual batch count')
        for j,b in enumerate(bs):
            need(equal(b['batch_index'],j) and equal(b['start'],at) and type(b['end']) is int and b['end']>at and b['cases']==b['end']-at and b['status']=='VERIFIED' and b['verified'] is True and b['returned_error'] is False and b['context_error'] is False,'successful contiguous actual batches')
            need(b['package']==p['package'] and b['activity']==p['activity'] and b['compiler_revision']==REV and b['go_version']=='go1.27.2' and b['source_set_sha256']==end['source_set_sha256'],'batch source/producer binding');at=b['end']
        need(at==p['cases'],'suite actual complete coverage')
    current_population(summary,planned,ends,contract_expected)
    validate_suite_sources(root,catalog,ends,read,contract_expected)
    need(planned[73]=={'index':73,'package':ROOT,'activity':'AssessAddition','cases':50},'preserved addition suite is exact50-case entry at73')
    addition=next(e for e in ends if e['suite_index']==73);addition_batches=sorted((e for e in batches if e['suite_index']==73),key=lambda e:e['batch_index'])
    evidence_root=verification_out/(ROOT.replace('/','-')+'-AssessAddition')
    anchors=[]
    for batch in addition_batches:
        workspace=evidence_root/('%03d'%batch['batch_index'])/'gooo.workspace.json'
        anchors.append({**{key:batch[key] for key in ('batch_index','start','end','manifest_digest','cases_digest','original_receipt_sha256')},'workspace':workspace,'producer_manifest_path':str(workspace)})
    return read_addition_evidence(root,read,pins,addition['source_set'],addition['source_set_sha256'],anchors,'go version go1.27.2 '+summary['goos']+'/'+summary['goarch'],header['identity']['binaries']['go'],contract_expected.suite_count)

def read_addition_evidence(root,read,pins,source_set,source_set_sha256,batches,go_version,go_binary,suite_count):
    """Read the same seven local blobs and original producer-path bindings.

    Independent Go case rows remain the ABI oracle. A relocated blob path never
    replaces the original manifest string inside any preserved raw receipt.
    """
    need(type(batches) is list and len(batches)==7,'seven bounded addition batches')
    need(type(suite_count) is int and suite_count>0,'positive complete suite population')
    need(type(go_binary) is dict and set(go_binary)=={'bytes','sha256'} and type(go_binary['bytes']) is int and go_binary['bytes']>0 and type(go_binary['sha256']) is str and re.fullmatch('[0-9a-f]{64}',go_binary['sha256']) is not None,'addition Go identity shape')
    need(type(go_version) is str and re.fullmatch(r'go version go1\.27\.2 [a-z0-9_]+/[a-z0-9_]+',go_version) is not None,'addition pinned Go version/platform')
    manifest_doc={'schema':'gooo/package-workspace-manifest/v1','entry':{'package_path':ROOT,'activity':'AssessAddition'},'packages':[{'path':pkg,'sources':[name]} for pkg,name in PACKAGES]}
    public_manifest=load(read(root/'diagnostic-header-addition.workspace.json'));need(equal(public_manifest,manifest_doc),'public exact five-package manifest')
    source_texts=[(pkg,name,read(root/name).decode('utf-8')) for pkg,name in PACKAGES];definitions,records=contracts(source_texts)
    current_source_set={name:{'Package':pkg,'Digest':digest(go_json(text))} for pkg,name,text in sorted(source_texts,key=lambda x:x[1])}
    # Outer map sorted, nested struct field order exactly Package,Digest (Go json.Marshal).
    source_set_raw=json.dumps(current_source_set,ensure_ascii=False,separators=(',',':')).encode()
    need(equal(source_set,current_source_set) and source_set_sha256==digest(source_set_raw),'actual source set exact5 and Go-map digest')
    images=[{'package':pkg,'filename':name,'source_digest':digest(go_json(text))} for pkg,name,text in source_texts]
    contract=load(read(root/'examples/diagnostic-header-addition/contract.json'));corpus=load(read(root/'examples/diagnostic-header-addition/corpus.json'))
    need(contract['entry']==ENTRY and contract['activity_id']==definitions[ENTRY]['id'] and contract['input_entity_id']==RID and contract['result_entity_id']==OID,'current contract IDs')
    for key,ident in [('input_fields',RID),('fields',OID)]:
        fields=contract[key];actual=records[ident]['fields']
        need(len(fields)==len(actual) and all(f['name']==a['name'] and f['id']==a['id'] and 'urn:gooo:type:'+f['type']==a['type_id'] for f,a in zip(fields,actual)),'contract/source ordered fields '+key)
    need(corpus['case_count']==50 and len(corpus['cases'])==50 and len({c['label'] for c in corpus['cases']})==50,'current 50 named source cases')
    rows=[];evidence=[];source=None;plan=None;selections=set()
    go_sha=go_binary['sha256']
    anchors={'batch_index','start','end','manifest_digest','cases_digest','original_receipt_sha256','workspace','producer_manifest_path'}
    for index,batch in enumerate(batches):
        need(type(batch) is dict and set(batch)==anchors,'exact addition batch reference fields')
        start,end=index*8,min(index*8+8,50);need(equal(batch['batch_index'],index) and equal(batch['start'],start) and equal(batch['end'],end) and 1<=end-start<=8,'exact bounded addition range')
        need(isinstance(batch['workspace'],pathlib.Path),'local workspace must be a Path')
        workspace=canonical(batch['workspace']);batchdir=workspace.parent
        need(type(batch['producer_manifest_path']) is str and batch['producer_manifest_path'] and '\x00' not in batch['producer_manifest_path'],'original producer manifest path string')
        manifest=read(workspace)
        need(equal(load(manifest),manifest_doc) and digest(manifest)==batch['manifest_digest'],'fresh batch exact manifest bytes')
        for _,name,text in source_texts:need(read(batchdir/name)==text.encode('utf-8'),'copied source equals current source: '+name)
        case_raw=read(batchdir/'cases.json');cases=load(case_raw)
        need(cases['schema']=='gooo/body-composition-cases/v1' and len(cases['cases'])==end-start and digest(case_raw)==batch['cases_digest'],'fresh actual Go expected case document bytes')
        for offset,row in enumerate(cases['cases']):
            spec=corpus['cases'][start+offset];inp=spec['input'];expected_inputs={ENTRY+'.input'+str(j):v for j,v in enumerate([inp['name'],inp['value'],inp['declared_length'],inp['facts']])}
            need(set(row)=={'inputs','expected'} and equal(row['inputs'],expected_inputs) and set(row['expected'])=={ENTRY},'fresh case inputs agree with source corpus')
            expected=row['expected'][ENTRY];need(all(equal(expected.get(k),v) for k,v in spec['expected'].items()),'fresh full Go expected agrees with partial source specification')
        original=read(batchdir/'execution.json');replay=read(batchdir/'replay.json')
        need(digest(original)==batch['original_receipt_sha256'],'fresh timing original raw receipt parent')
        for operation,raw,prior in [('execute',original,None),('replay',replay,original)]:
            observed=receipt(raw,cases['cases'],case_raw,manifest,images,go_version,go_sha,definitions,records,batch['producer_manifest_path'],prior)
            need(observed['source'].startswith('package main\n') and (source is None or source==observed['source']),'same exact generated Go text across seven original receipts and seven saved replays')
            need(plan is None or equal(plan,observed['plan']),'same current typed plan across all14 receipts')
            source,plan=observed['source'],observed['plan'];selections.add(observed['tool_selection'])
            evidence.append({'batch':index,'operation':operation,'receipt':str(batchdir/('execution.json' if operation=='execute' else 'replay.json')),'receipt_sha256':sha(raw),'cases_digest':batch['cases_digest'],'original_receipt_sha256':batch['original_receipt_sha256']})
        rows.extend(cases['cases'])
    need(len(rows)==50 and source is not None,'exact50 fresh case rows and generated source');unchanged(pins)
    return {'rows':rows,'source':source,'pins':pins,'case_names':[c['label'] for c in corpus['cases']],'source_set':source_set,'typed_plan':plan,'records':records,'entry':ENTRY,'evidence':evidence,'go_binary':go_binary,'go_version':go_version,'tool_selections':sorted(selections),'fresh_native_suites':suite_count,'fresh_addition_batches':7,'fresh_addition_cases':50}

def read_shard_verification(root,index_path):
    """Read only the explicit experimental provider API; never try legacy input."""
    root,index_path=map(canonical,(root,index_path))
    provider=canonical(root/'tools/native-shard-reference.py');bootstrap_pins={}
    provider_raw=frozen_read(provider,bootstrap_pins)
    spec=importlib.util.spec_from_file_location('gooo_native_shard_reference',provider)
    need(spec is not None and spec.loader is not None,'explicit shard provider module')
    module=importlib.util.module_from_spec(spec);sys.modules[spec.name]=module
    # Execute only these frozen sibling bytes, never a stale or writable pyc.
    exec(compile(provider_raw,str(provider),'exec'),module.__dict__)
    unchanged(bootstrap_pins)
    reference=module.read_reference(root,index_path)
    need(type(reference) is dict and set(reference)=={'pins','source_set','source_set_sha256','batches','go_version','go_binary','suite_count'},'exact shard reference provider API')
    pins=reference['pins'];need(type(pins) is dict,'shard reference frozen pins')
    for name,value in bootstrap_pins.items():need(equal(pins.get(name),value),'shard provider bytes are frozen with its evidence')
    read=lambda p:frozen_read(p,pins)
    read(index_path)
    result=read_addition_evidence(root,read,pins,reference['source_set'],reference['source_set_sha256'],reference['batches'],reference['go_version'],reference['go_binary'],reference['suite_count'])
    result['evidence_mode']='experimental-shard-native-reference'
    return result

def render(root,verification,rendered):
    """Only bridge IDs and fresh hashes are substituted. Policy source stays exact."""
    need(not rendered.exists(),'fresh rendered source directory required');rendered.mkdir();(rendered/'testdata').mkdir()
    records=verification['records'];request,result=records[RID],records[OID]
    substitutions={'@@REQUEST_RECORD@@':request['go_name'],'@@ASSESSOR_FUNCTION@@':verification['typed_plan']['activities'][0]['go_function'],'@@GENERATED_SOURCE_SHA256@@':sha(verification['source'].encode())}
    for tag,record in [('REQUEST',request),('RESULT',result)]:
        for field in record['fields']:substitutions['@@'+tag+':'+field['name']+'@@']=field['go_name']
    fixture={'schema':'gooo/fresh-header-assessor-abi-fixture/v1','rows':[]}
    for label,row in zip(verification['case_names'],verification['rows']):
        values=[row['inputs'][ENTRY+'.input'+str(i)] for i in range(4)]
        fixture['rows'].append({'label':label,'name':values[0],'value':values[1],'declared_length':values[2],'facts':values[3],'expected':row['expected'][ENTRY]})
    fixture_raw=(json.dumps(fixture,ensure_ascii=False,indent=2)+'\n').encode('utf-8');substitutions['@@ABI_FIXTURE_SHA256@@']=sha(fixture_raw)
    (rendered/'assessor_generated.go').write_bytes(verification['source'].encode('utf-8'));(rendered/'testdata/fresh-50.json').write_bytes(fixture_raw)
    transformations=[]
    for name in TEMPLATES:
        template=(root/EXAMPLE/name).read_text(encoding='utf-8');text=template
        for key,value in substitutions.items():text=text.replace(key,value)
        need('@@' not in text,'unexpanded bridge template: '+name);target=rendered/name[:-4];target.write_text(text,encoding='utf-8')
        transformations.append({'template':EXAMPLE+'/'+name,'rendered':target.name,'substitutions':[k for k in substitutions if k in template]})
    for name in ['LICENSE','NOTICE']:(rendered/name).write_bytes((root/EXAMPLE/name).read_bytes())
    notices=[]
    for _,name in PACKAGES:
        text=(root/name).read_text(encoding='utf-8');prefix=text.split('namespace ',1)[0]
        notices.append('Source: '+name+'\n'+prefix)
    (rendered/'SOURCE-NOTICES.txt').write_text('\n'.join(notices),encoding='utf-8')
    need(len(re.findall(r'^func Test\w+\(',''.join((rendered/name).read_text() for name in ['host_test.go','bindings_test.go']),re.M))==18,'exact18 top-level meaningful host tests')
    frozen={str(p):reference(p.read_bytes()) for p in sorted(rendered.rglob('*')) if p.is_file()}
    return {'rendered_pins':frozen,'transformations':transformations,'generated_source':{'transformation':'none; exact result.composition.source bytes','sha256':substitutions['@@GENERATED_SOURCE_SHA256@@']},'fixture':{'sha256':sha(fixture_raw),'origin':'fresh independent Go expected case documents; no Gooo actual or private fixture inheritance'}}

def processes():
    raw=subprocess.check_output(['/bin/ps','-Aww','-o','pid=,ppid=,pgid=,lstart=,command='],text=True,timeout=2)
    rows={}
    for line in raw.splitlines():
        fields=line.strip().split(None,8)
        if len(fields)==9:
            pid,ppid,pgid=map(int,fields[:3]);rows[pid]={'pid':pid,'ppid':ppid,'pgid':pgid,'birth':' '.join(fields[3:8]),'command':fields[8]}
    return rows

def owned_bytes(work):
    """Count regular owned bytes while Go removes temporary files/directories."""
    def walk_error(error):
        if not isinstance(error,FileNotFoundError):raise error
    size=0
    for directory,_,files in os.walk(work,onerror=walk_error,followlinks=False):
        for name in files:
            try:info=(pathlib.Path(directory)/name).lstat()
            except FileNotFoundError:continue
            if stat.S_ISREG(info.st_mode):size+=info.st_size
    return size

class OwnedCommands:
    """Bounded polling guard for this unique owned work path, not global cleanup."""
    def __init__(self,out,work,cwd,pins,env,pin_scans=None):
        self.out,self.work,self.cwd,self.pins,self.env=out,work,cwd,pins,env
        self.pin_scans=pin_scans if pin_scans is not None else PinScanObservations()
        self.started=time.monotonic();self.observations=[];self.closed=True
    def call(self,label,args,seconds,extra_env=None,stdin_path=None,allowed_exit=(0,)):
        self.pin_scans.observe(label,'pre',lambda:unchanged(self.pins),self.pins);directory=self.out/label;directory.mkdir();begin=time.monotonic();why=None;known={};child=None;remaining=[]
        env=dict(self.env);env.update(extra_env or {})
        def live():
            rows=processes();owned={pid for pid,row in rows.items() if pid in known and row['birth']==known[pid]['birth']}
            if child is not None and child.poll() is None and child.pid in rows:owned.add(child.pid)
            owned.update(pid for pid,row in rows.items() if re.search(re.escape(str(self.work))+r'(?=$|[/\s])',row['command']))
            while True:
                more={pid for pid,row in rows.items() if row['ppid'] in owned};before=len(owned);owned.update(more)
                if len(owned)==before:break
            for pid in owned:known[pid]=rows[pid]
            return [rows[pid] for pid in owned]
        # Optional stdin is an already retained request file; no shell or live pipe.
        input_stream=open(stdin_path,'rb') if stdin_path is not None else open(os.devnull,'rb')
        with input_stream,(directory/'stdout').open('xb') as stdout,(directory/'stderr').open('xb') as stderr:
            try:
                need(begin-self.started<300,'whole300s budget')
                child=subprocess.Popen(list(map(str,args)),cwd=self.cwd,env=env,stdin=input_stream,stdout=stdout,stderr=stderr,start_new_session=True)
                save(directory/'started.json',{'argv':list(map(str,args)),'pid':child.pid,'pgid':child.pid,'timeout_s':seconds,'environment_overrides':extra_env or {}})
                while child.poll() is None:
                    live();need(time.monotonic()<min(self.started+300,begin+seconds),'command/whole timeout')
                    need(stdout.tell()<=8<<20 and stderr.tell()<=2<<20,'command output cap')
                    need(owned_bytes(self.work)<=1<<30,'owned work1GiB bound');time.sleep(0.05)
            except BaseException as error:why=str(error)
            finally:
                try:remaining=live()
                except BaseException as error:why=why or 'process observation failed: '+str(error);self.closed=False
                if remaining:
                    why=why or 'observed owned processes remained'
                    for group in {row['pgid'] for row in remaining}:
                        need(group>1 and group!=os.getpgrp(),'refuse unrelated process group')
                        try:os.killpg(group,signal.SIGKILL)
                        except ProcessLookupError:pass
                # The direct child is a known new session even if ps failed.
                # Do not rely on a successful snapshot to stop that live group.
                if child is not None and child.poll() is None:
                    need(child.pid>1 and child.pid!=os.getpgrp(),'refuse unrelated direct process group')
                    try:os.killpg(child.pid,signal.SIGKILL)
                    except ProcessLookupError:pass
                if child is not None:
                    try:child.wait(timeout=3)
                    except subprocess.TimeoutExpired:why=why or 'direct child not reaped';self.closed=False
                for _ in range(5):
                    try:remaining=live()
                    except BaseException as error:why=why or str(error);self.closed=False;break
                    if not remaining:break
                    time.sleep(0.05)
                self.closed=self.closed and not remaining
                observation={'label':label,'exit_code':None if child is None else child.returncode,'failure':why,'wall_seconds':time.monotonic()-begin,'observed_owned_processes_remaining':[{k:row[k] for k in ['pid','pgid','birth']} for row in remaining],'scope':'Polling ancestry and unique owned work command paths; not a kernel fork trace.'}
                save(directory/'process.json',observation);self.observations.append(observation)
        need(child is not None and child.returncode in allowed_exit and why is None and not remaining,'command failure: '+label+' '+str(why))
        need((directory/'stdout').stat().st_size<=8<<20 and (directory/'stderr').stat().st_size<=2<<20,'final output cap')
        self.pin_scans.observe(label,'post',lambda:unchanged(self.pins),self.pins);return (directory/'stdout').read_bytes()

def parse_args(argv=None):
    parser=argparse.ArgumentParser(description=__doc__)
    for name in ['root','go','out']:parser.add_argument('--'+name,required=True)
    mode=parser.add_mutually_exclusive_group(required=True)
    mode.add_argument('--verification-out')
    mode.add_argument('--shard-native-index')
    parser.add_argument('--timing')
    args=parser.parse_args(argv)
    if args.shard_native_index is not None:
        if not args.shard_native_index or args.timing is not None:parser.error('--shard-native-index requires a nonempty path and forbids --timing')
    elif not args.verification_out or not args.timing:parser.error('--verification-out and --timing require nonempty paths together')
    return args

def _main(pin_scans):
    args=parse_args();root=canonical(pathlib.Path(args.root).absolute());go=canonical(pathlib.Path(args.go).absolute());out=canonical((root/args.out).absolute())
    shard_index=canonical((root/args.shard_native_index).absolute()) if args.shard_native_index is not None else None
    verification_out=canonical((root/args.verification_out).absolute()) if args.verification_out is not None else None
    timing=canonical((root/args.timing).absolute()) if args.timing is not None else None
    need(root.is_dir() and not out.exists(),'root exists and output is fresh');out.mkdir();pin_scans.output=out;started=time.monotonic();runner=None;work=None;error=None;verification=None;rendered=None;pins={};cleanup=False
    def interrupted(signum,_frame):
        failure=RuntimeError('received signal '+str(signum))
        if pin_scans.cancellation is None:pin_scans.cancellation=failure
        raise failure
    original_handlers={number:signal.signal(number,interrupted) for number in [signal.SIGTERM,signal.SIGINT]}
    try:
        verification=pin_scans.observe('initial-verification','parent',lambda:read_shard_verification(root,shard_index) if shard_index is not None else read_verification(root,verification_out,timing));pins=dict(verification['pins']);go_raw=frozen_read(go,pins)
        need(equal(reference(go_raw),verification['go_binary']),'explicit host Go binary matches fresh differential Go identity')
        rendered=out/'rendered';transformation=render(root,verification,rendered);pins.update(transformation['rendered_pins'])
        save(out/'rendered-provenance.json',{'schema':'gooo/fresh-header-addition-host-provenance/v1','entry':ENTRY,'evidence':verification['evidence'],'input_pins':verification['pins'],**transformation})
        save(out/'frozen-inputs.json',pins);pin_scans.observe('initial','initial',lambda:unchanged(pins),pins)
        need(shutil.disk_usage(out).free>=1<<30,'at least1GiB free for bounded owned check')
        work=pathlib.Path(tempfile.mkdtemp(prefix='.header-addition-host-owned-',dir=out));(work/'tmp').mkdir();(work/'cache').mkdir();(work/'modules').mkdir();(work/'bin').mkdir()
        env={key:os.environ[key] for key in ['PATH','HOME','SYSTEMROOT'] if key in os.environ}
        env.update(GOTOOLCHAIN='local',GOWORK='off',GOENV='off',GOPROXY='off',GOSUMDB='off',GOFLAGS='-mod=readonly',CGO_ENABLED='0',GOMAXPROCS='2',GOCACHE=str(work/'cache'),GOMODCACHE=str(work/'modules'),TMPDIR=str(work/'tmp'))
        save(out/'owned-work.json',{'work':str(work),'whole_timeout_s':300,'owned_byte_cap':1<<30,'retained':'rendered source and fixture, input/evidence refs, logs and summaries','cleanup':'Only this exact owned work directory after all observed owned processes close.','environment':env})
        runner=OwnedCommands(out,work,rendered,pins,env,pin_scans);runner.started=started
        version=runner.call('go-version',[go,'version'],20).decode().strip();need(version==verification['go_version'],'host exact Go version/platform')
        runner.call('abi50',[go,'test','-count=1','-run','^TestFreshGeneratedABI50$','-v','.'],60)
        runner.call('vet',[go,'vet','./...'],45)
        cc=runner.call('cc-selection',[go,'env','CC'],10).decode().strip();need(cc and not any(x.isspace() for x in cc),'single observed CC executable')
        compiler=shutil.which(cc,path=env.get('PATH'));need(compiler is not None,'race C compiler available');compiler=pathlib.Path(compiler).resolve();frozen_read(compiler,pins)
        cc_version=runner.call('cc-version',[compiler,'--version'],10).decode(errors='replace');save(out/'race-c-toolchain.json',{'selection':cc,'resolved':str(compiler),'reference':pins[str(compiler)],'version':cc_version,'scope':'Observed driver identity; not a complete C toolchain/sysroot closure.'})
        runner.call('race',[go,'test','-race','-count=1','-v','./...'],120,{'CGO_ENABLED':'1','CC':str(compiler)})
        runner.call('build',[go,'build','-trimpath','-o',work/'bin/demo','.'],60)
        demo=load(runner.call('demo',[work/'bin/demo'],5));need(demo['Status']=='APPLIED' and demo['HasDecision'] is True and equal([demo['BeforeRevision'],demo['AfterRevision'],demo['PolicyRevision']],[1,2,1]) and demo['Decision']['eligible'] is True and demo['Decision']['candidate_name']=='x-request-id' and demo['Decision']['candidate_value']=='req-123','actual exact demo one application')
        pin_scans.observe('final','final',lambda:unchanged(pins),pins);need(runner.closed,'all observed processes closed');save(out/'final-source-tool-pins.json',pins)
    except BaseException as failure:error=str(failure)
    finally:
        if work is not None and (runner is None or runner.closed):
            try:shutil.rmtree(work);cleanup=True
            except BaseException as failure:error=error or 'owned cleanup failure: '+str(failure)
        for number,handler in original_handlers.items():signal.signal(number,handler)
    status=('EXPERIMENTAL_SHARD_REFERENCE_ABI50_AND_ATOMIC_HOST_PASS' if shard_index is not None else 'FRESH82_BOUND_ABI50_AND_ATOMIC_HOST_PASS') if error is None else 'FIRST_FAILURE_PRESERVED'
    summary={'schema':'gooo/header-addition-host-CI/v1','status':status,'failure':error,'fresh_native_suites':verification['fresh_native_suites'] if verification else None,'fresh_addition_cases':50 if verification else None,'fresh_addition_batches':7 if verification else None,'ABI_generated_calls':50 if error is None else None,'ABI_typed_fields':800 if error is None else None,'ABI_count_scope':'Focused abi50 command only. Race repeats these50 and host scenarios call the generated assessor again; this is not the total generated-call count.','host_test_functions':18 if error is None else None,'demo_status':'APPLIED' if error is None else None,'processes_closed':runner.closed if runner else True,'source_bindings_unchanged':error is None,'owned_work_removed':cleanup,'elapsed_s':time.monotonic()-started,'commands':runner.observations if runner else [],'scope':('Experimental shard reference with unauthenticated producer identity; no native acceptance or authenticated GitHub-run claim. ' if shard_index is not None else '')+'Fresh public native receipt source,50 independent-Go ABI expectations and local in-memory atomic host tests. No network effects, real authentication, durability, distributed transaction or production-safety claim. Independent inner typed canonical prior_result_sha256 remains NOT_CHECKED.'}
    save(out/'summary.json',summary);print(json.dumps(summary));return 0 if error is None else 1
def main():
    pin_scans=PinScanObservations();failed=True
    try:
        result=_main(pin_scans);failed=result!=0;return result
    finally:pin_scans.write_once(preserve_failure=failed)

if __name__=='__main__':sys.exit(main())
