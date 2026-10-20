"""Concrete record ABI for the optional Prefix and original rule-pair layer.

Policy expectations come only from the independently executed Go oracle. The
shared classifier module supplies strict JSON, source and scalar utilities;
this module never delegates a record check to the four-scalar classifier ABI.
"""
from pathlib import Path
import json
import re

PORT='go/net/netip'; PROBE='app/ipv4-prefix-probe'; APP='app/ipv4-rule-pair'
REPORT=APP+':Report'; CONTAINS=PROBE+':Contains'; OVERLAPS=PROBE+':Overlaps'
ADDR='gooo://go/net/netip/ipv4/address/v1'; PREFIX='gooo://go/net/netip/ipv4/prefix/v1'
RULE='gooo://ipv4-rule-pair/rule/v1'; RESULT='gooo://ipv4-rule-pair/result/v2'
INTEGER='gooo://std/integer'; BOOLEAN='gooo://std/boolean'
OCTETS=('a','b','c','d'); RULE_FIELDS=OCTETS+('bits','effect','scope','priority')
FACTS=('valid','comparable','overlap','opposing_effect','conflict','witness_valid','witness_a','witness_b','witness_c','witness_d','reason')
MANIFESTS={CONTAINS:'prefix-contains.workspace.json',OVERLAPS:'prefix-overlaps.workspace.json',REPORT:'ipv4-rule-pair.workspace.json'}
SOURCES={PORT:('ports/net/netip/ipv4.gooo','ports/net/netip/prefix.gooo'),
         PROBE:('examples/ipv4-prefix-probe/main.gooo',),APP:('examples/ipv4-rule-pair/main.gooo',)}
IDS={'IPv4AddressValid':'gooo://go-ports/net/netip/ipv4/address-valid/v1',
     'IPv4PrefixValid':'gooo://go-ports/net/netip/ipv4/prefix-valid/v1',
     'IPv4PrefixDivisor':'gooo://go-ports/net/netip/ipv4/divisor/v1',
     'PrefixContains':'gooo://go/net/netip/ipv4/prefix-contains',
     'PrefixOverlaps':'gooo://go/net/netip/ipv4/prefix-overlaps',
     'Contains':'gooo://ipv4-prefix-probe/contains/v1','Overlaps':'gooo://ipv4-prefix-probe/overlaps/v1',
     'RuleValid':'gooo://ipv4-rule-pair/rule-valid/v1','Report':'gooo://ipv4-rule-pair/report/v2'}
RECORDS={'IPv4Address':(ADDR,OCTETS), 'IPv4Prefix':(PREFIX,OCTETS+('bits',)),
         'Rule':(RULE,RULE_FIELDS),'RulePairReport':(RESULT,FACTS)}
SIGNATURES={'IPv4AddressValid':(['IPv4Address'],'Boolean'),'IPv4PrefixValid':(['IPv4Prefix'],'Boolean'),
 'IPv4PrefixDivisor':(['Integer'],'Integer'),'PrefixContains':(['IPv4Prefix','IPv4Address'],'Boolean'),
 'PrefixOverlaps':(['IPv4Prefix','IPv4Prefix'],'Boolean'),'Contains':(['IPv4Prefix','IPv4Address'],'Boolean'),
 'Overlaps':(['IPv4Prefix','IPv4Prefix'],'Boolean'),'RuleValid':(['Rule'],'Boolean'),'Report':(['Rule','Rule'],'RulePairReport')}
S=None


def configure(shared):
    global S
    S=shared


def need(ok,message):
    if not ok:raise S.ReceiptError(message)


def entity(name):
    return {'Integer':INTEGER,'Boolean':BOOLEAN}.get(name,RECORDS.get(name,(None,))[0])


def record_types(report):
    names=('IPv4Address','IPv4Prefix','RulePairReport','Rule') if report else ('IPv4Address','IPv4Prefix')
    result=[]
    for name in names:
        eid,fields=RECORDS[name]
        result.append({'name':name,'id':eid,'go_name':'GoooRecord'+S.digest(eid.encode())[7:],
            'fields':[{'name':f,'id':eid+'/'+f,'type_id':'urn:gooo:type:'+('boolean' if eid==RESULT and i<6 else 'integer'),
                       'go_name':'GoooField'+S.digest((eid+'/'+f).encode())[7:]} for i,f in enumerate(fields)]})
    return result


def port_types(entry):return SIGNATURES[entry.split(':')[-1]][0]


def validate_inputs(raw,entry):
    if entry not in MANIFESTS:raise S.InputError('ENTRY')
    if len(raw)>8192:raise S.InputError('DOCUMENT_BOUND')
    value=S.strict_json(raw,S.InputError)
    def fail(ok,code):
        if not ok:raise S.InputError(code)
    fail(type(value) is dict,'OBJECT_REQUIRED')
    fail(set(value)=={'schema','inputs'} and value.get('schema')=='gooo/body-composition-inputs/v1','DOCUMENT_FIELDS')
    rows=value['inputs'];fail(type(rows) is list and 1<=len(rows)<=18,'ROW_COUNT')
    for row in rows:
        fail(type(row) is dict,'ROW_OBJECT')
        fail(set(row)=={entry+'.input0',entry+'.input1'},'PORT_ROSTER')
        for i,name in enumerate(port_types(entry)):
            record=row[entry+'.input'+str(i)];fields=RECORDS[name][1]
            fail(type(record) is dict,'RECORD_OBJECT')
            fail(set(record)==set(fields),'RECORD_FIELDS')
            fail(all(S.integer(record[f]) for f in fields),'RECORD_INTEGER')
    return rows


def input_document(rows,entry):
    sides=('left','right') if entry==REPORT else ('prefix','address' if entry==CONTAINS else 'other_prefix')
    value={'schema':'gooo/body-composition-inputs/v1','inputs':[
        {entry+'.input'+str(i):r['inputs'][side] for i,side in enumerate(sides)} for r in rows]}
    raw=(json.dumps(value,separators=(',',':'))+'\n').encode()
    validate_inputs(raw,entry);return raw


def expected_value(row,entry):return row['expected'] if entry==REPORT else row['expected']['contains' if entry==CONTAINS else 'overlaps']


def check_arithmetic(rt,composition,activity,callees):
    names=('observed_projection_sha256','observed_driver_sha256','fault_sites','outcomes')
    need(all(k in rt for k in names) and all(S.is_digest(rt[k]) for k in names[:2]),'PAIR_ARITHMETIC_BINDING')
    need(S.exact(rt['outcomes'],{k:0 for k in ('matched','mismatched','faulted','blocked','unobserved')}),'PAIR_ARITHMETIC_BINDING')
    sites=rt['fault_sites'];raw=composition['source'].encode()
    need(type(sites) is list and 1<=len(sites)<=64,'PAIR_ARITHMETIC_BINDING')
    keys=('expression_id','root_activity_id','activity_id','operator','expression','projection_sha256','start','end');positions=[]
    for site in sites:
        need(type(site) is dict and set(site)==set(keys) and all(type(site[k]) is str for k in keys[:6]) and S.integer(site['start']) and S.integer(site['end']) and 0<=site['start']<site['end']<=len(raw),'PAIR_ARITHMETIC_BINDING')
        need(site['root_activity_id']==activity and site['activity_id'] in {activity,*callees} and site['operator'] in ('/','%') and site['expression'].encode()==raw[site['start']:site['end']] and site['projection_sha256']==composition['generated_sha256'],'PAIR_ARITHMETIC_BINDING')
        original={k:('' if k=='expression_id' else site[k]) for k in keys}
        need(site['expression_id']==S.digest(S.go_string(original)),'PAIR_ARITHMETIC_BINDING')
        positions.append((site['start'],-site['end']))
    need(positions==sorted(set(positions)),'PAIR_ARITHMETIC_BINDING')
    return {k:rt[k] for k in names}


class Evidence:
    def __init__(self,repository,oracle,selected_go_version,shared):
        configure(shared);self.repo=Path(repository).resolve();self.version=selected_go_version
        need(re.fullmatch(r'go version go1\.27\.2 [a-z0-9]+/[a-z0-9]+',selected_go_version) is not None,'SELECTED_GO_VERSION')
        need(oracle.get('schema')=='gooo/ipv4-prefix-rule-pair-independent-oracle/v1' and oracle.get('go_version')=='go1.27.2','PAIR_ORACLE_SCHEMA')
        self.rows={REPORT:oracle['pair_cases'],CONTAINS:oracle['prefix_cases'],OVERLAPS:oracle['prefix_cases']}
        for entry,filename,n in ((REPORT,'examples/ipv4-rule-pair/corpus-112.json',112),(CONTAINS,'examples/ipv4-prefix-probe/corpus-18.json',18)):
            corpus=S.strict_json(S._read_regular(self.repo/filename,128<<10));rows=self.rows[entry]
            need(type(rows) is list and len(rows)==len(corpus['cases'])==n,'PAIR_ORACLE_COUNT')
            for i,(actual,original) in enumerate(zip(rows,corpus['cases'])):
                inputs={k:original[k] for k in (('left','right') if entry==REPORT else ('prefix','address','other_prefix'))}
                need(S.integer(actual.get('index'),i) and actual.get('name')==original['name'] and S.exact(actual.get('inputs'),inputs),'PAIR_ORACLE_INPUT')
                for name,value in inputs.items():
                    fields=RULE_FIELDS if entry==REPORT else OCTETS+(('bits',) if name!='address' else ())
                    need(type(value) is dict and set(value)==set(fields) and all(S.integer(v) for v in value.values()),'PAIR_ORACLE_INPUT_KIND')
                expected=actual['expected']
                if entry==REPORT:
                    need(type(expected) is dict and set(expected)==set(FACTS) and all(type(expected[f]) is bool for f in FACTS[:6]) and all(S.integer(expected[f]) for f in FACTS[6:]),'PAIR_ORACLE_FACT_KIND')
                else:need(type(expected) is dict and set(expected)=={'contains','overlaps'} and all(type(v) is bool for v in expected.values()),'PREFIX_ORACLE_KIND')
        original=S.strict_json(S._read_regular(self.repo/'examples/ipv4-rule-pair/corpus-18.json',32<<10))
        expanded=S.strict_json(S._read_regular(self.repo/'examples/ipv4-rule-pair/corpus-112.json',128<<10))
        need(S.exact(expanded['cases'][:18],original['cases']),'PAIR_ORIGINAL18_PREFIX')
        self.bodies={}
        expected={PORT:('IPv4AddressValid','IPv4PrefixValid','IPv4PrefixDivisor','PrefixContains','PrefixOverlaps'),PROBE:('Contains','Overlaps'),APP:('RuleValid','Report')}
        for package,names in expected.items():
            source=SOURCES[package][-1];bodies=S.activity_bodies((self.repo/source).read_text())
            need(set(bodies)==set(names),'PAIR_SOURCE_BODY_ROSTER')
            for name in names:self.bodies[package+':'+name]={'body':bodies[name],'source':source,'id':IDS[name]}

    def graph(self,entry,lowered=None):
        queue=[entry];sites=[];rewritten={}
        for key in queue:
            body=self.bodies[key]['body'];edits=[]
            for match in re.finditer(r'((?:netip\.)?[A-Z]\w*)\(',body):
                token=match.group(1);name=token.split('.')[-1]
                callee=(PORT if token.startswith('netip.') else key.split(':')[0])+':'+name
                need(callee in self.bodies,'PAIR_UNEXPECTED_CALL')
                start,end=match.span(1);sites.append({'caller':key,'callee':callee,'surface':'computes','start_byte':len(body[:start].encode()),'end_byte':len(body[:end].encode())})
                if lowered is not None:edits.append((start,end,lowered[callee]))
                if callee not in queue:queue.append(callee)
            for start,end,name in reversed(edits):body=body[:start]+name+body[end:]
            rewritten[key]=body
        return queue,sites,rewritten

    def workspace(self,image,entry):
        app=entry.split(':')[0];packages={PORT:SOURCES[PORT],app:SOURCES[app]}
        need(image['schema']=='gooo/package-runtime-image/v1' and len(image['packages'])==2 and S.exact(image['init_order'],[PORT,app]),'PAIR_WORKSPACE_ROSTER')
        names,out=SIGNATURES[entry.split(':')[-1]]
        need(S.exact(image['entry'],{'package_path':app,'source':SOURCES[app][0],'activity':entry.split(':')[-1],'inputs':names,'output':out}),'PAIR_WORKSPACE_ENTRY')
        seen=set()
        for p in image['packages']:
            path=p['path'];need(path in packages and path not in seen,'PAIR_PACKAGE_ROSTER');seen.add(path)
            pname,namespace={PORT:('netip','gooo_ports_net_netip'),PROBE:('ipv4_prefix_probe','gooo_original_ipv4_prefix_probe'),APP:('ipv4_rule_pair','gooo_original_ipv4_rule_pair')}[path]
            need(p['name']==pname and p['namespace']==namespace and S.exact(p['imports'],None if path==PORT else [PORT]),'PAIR_PACKAGE_IDENTITY')
            need(type(p['sources']) is list and len(p['sources'])==len(packages[path]) and
                 S.exact([{k:v[k] for k in ('filename','source_digest')} for v in p['sources']],
                         [{'filename':f,'source_digest':S.digest(S.go_string((self.repo/f).read_bytes().decode('utf-8','strict')))} for f in packages[path]]),'PAIR_SOURCE_IMAGE')
            expected={}
            if path==PORT:
                expected.update({'Integer':('entity',INTEGER),'Boolean':('entity',BOOLEAN)})
                expected.update({n:('entity',RECORDS[n][0]) for n in ('IPv4Address','IPv4Prefix')})
                expected.update({n:('activity',IDS[n]) for n in SIGNATURES if n.startswith('IPv4') or n.startswith('Prefix')})
                expected.update({n:('activity',S.IDS[n]) for n in ('IPv4Valid',)+S.METHODS})
            elif path==APP:expected.update({'Rule':('entity',RULE),'RulePairReport':('entity',RESULT),**{n:('activity',IDS[n]) for n in ('RuleValid','Report')}})
            else:expected.update({n:('activity',IDS[n]) for n in ('Contains','Overlaps')})
            need(type(p['exports']) is list and len(p['exports'])==len(expected),'PAIR_EXPORT_COUNT')
            exported=set()
            for x in p['exports']:
                name=x['name'];need(name in expected and name not in exported and (x['kind'],x['id'])==expected[name],'PAIR_EXPORT_ID');exported.add(name)
                if x['kind']=='activity':
                    ins,ret=SIGNATURES[name] if name in SIGNATURES else (['Integer']*4,'Boolean')
                    need(S.exact(x['input_types'],[entity(t) for t in ins]) and x['output_type']==entity(ret),'PAIR_EXPORT_ABI')

    def inspect(self,raw,entry,inputs,manifest,go_digest,original=None,prior_typed=None):
        need(entry in MANIFESTS,'PAIR_ENTRY');report=entry==REPORT;rows=self.rows[entry][:18]
        inputs,manifest=Path(inputs),Path(manifest)
        need(manifest==self.repo/MANIFESTS[entry],'PAIR_MANIFEST_PATH')
        mr=S._read_regular(manifest,8192);ir=S._read_regular(inputs,8192)
        app=entry.split(':')[0]
        need(S.exact(S.strict_json(mr),{'schema':'gooo/package-workspace-manifest/v1','entry':{'package_path':app,'activity':entry.split(':')[-1]},'packages':[{'path':p,'sources':list(SOURCES[p])} for p in (PORT,app)]}),'PAIR_MANIFEST')
        supplied=validate_inputs(ir,entry);need(S.exact(supplied,S.strict_json(input_document(rows,entry))['inputs']),'PAIR_ORIGINAL_INPUTS')
        e=S.load_receipt(raw);top={'schema','decision','manifest','manifest_digest','inputs_digest','result'}
        if original is not None:top.add('replayed_from_sha256')
        need(type(e) is dict and set(e)==top and e['schema']=='gooo/workspace-body-execution-receipt/v1' and e['decision']=='OBSERVED' and e['manifest']==str(manifest) and e['manifest_digest']==S.digest(mr) and e['inputs_digest']==S.digest(ir),'PAIR_RECEIPT_BINDING')
        r=e['result'];p=r['program'];c=r['composition'];rt=r['runtime'];self.workspace(p['workspace_image'],entry)
        need(r['schema']=='gooo/workspace-body-execution/v1' and p['schema']=='gooo/workspace-body-program/v1','PAIR_PROGRAM_SCHEMA')
        closure,sites,_=self.graph(entry);calls=p['pure_calls']
        need(type(calls) is dict and calls['schema']=='gooo/workspace-pure-calls/v1' and S.exact(sorted(calls['sites'],key=lambda x:json.dumps(x,sort_keys=True)),sorted(sites,key=lambda x:json.dumps(x,sort_keys=True))),'PAIR_CALL_SITES')
        need(len(calls['activities'])==len(closure),'PAIR_CALL_COUNT');lowered={}
        for v in calls['activities']:
            a=v['activity'];key=a['package_path']+':'+a['activity'];name=a.get('lowered_name')
            need(key in closure and key not in lowered and type(name) is str and re.fullmatch(r'GoooPackage[0-9]{1,6}Activity'+re.escape(a['activity']),name) is not None and int(name.split('Package',1)[1].split('Activity',1)[0])<sum((self.repo/f).stat().st_size for path in (PORT,app) for f in SOURCES[path]) and name not in lowered.values(),'PAIR_LOWERED_ID')
            need(v['source']==self.bodies[key]['source'] and v['source_body_sha256']==S.digest(self.bodies[key]['body'].encode()),'PAIR_BODY_BINDING');lowered[key]=name
        root={'package_path':app,'activity':entry.split(':')[-1],'lowered_name':lowered[entry]}
        need(S.exact(p['entry'],root) and S.exact(p['activities'],[root]),'PAIR_SELECTED_ENTRY')
        _,_,bodies=self.graph(entry,lowered);actual_bodies=S.activity_bodies(p['lowered_gooo_source'])
        need(S.exact(actual_bodies,{lowered[k]:bodies[k] for k in closure}),'PAIR_LOWERED_BODY')
        source=S.digest(p['lowered_gooo_source'].encode())
        need(r['execution_source_sha256']==source and c['schema']=='gooo/body-composition/v1' and c['stage']=='COMPLETE' and c['original_source_sha256']==c['selected_source_sha256']==source and c['gooo_source']==p['lowered_gooo_source'],'PAIR_SOURCE_BINDING')
        need(all(r.get(k) is None for k in ('body_fills','assembly_policy','assembly_policy_history','continuation')),'PAIR_UNEXPECTED_FILL')
        plan=c['plan'];ins,ret=SIGNATURES[root['activity']];records=record_types(report)
        expected_activity={'name':lowered[entry],'id':IDS[root['activity']],'input_type':ins[0],'output_type':ret,'input_entity_id':entity(ins[0]),'output_entity_id':entity(ret),'go_function':'GoooComposedActivity0','input_from':-1,'assembling':False,'inputs':[{'port':'input'+str(i),'type':t,'entity_id':entity(t),'from':-1} for i,t in enumerate(ins)]}
        need(plan['schema']=='gooo/body-composition-plan/v1' and plan['entry_activity']==lowered[entry] and S.is_digest(plan['typed_plan_sha256']) and re.fullmatch('[0-9a-f]{64}',plan['semantic_fingerprint']) and S.exact(plan['activities'],[expected_activity]) and S.exact(plan['record_types'],records) and S.exact(plan['edges'],[]) and plan.get('preparations') is None,'PAIR_TYPED_PLAN_ABI')
        need(type(c['steps']) is list and len(c['steps'])==1 and c['steps'][0]['input_source_sha256']==source,'PAIR_GENERATION_STEP')
        generation=c['steps'][0]['generation'];g=generation['report']
        need(g['schema']=='gooo/body-codegen-report/v3' and g['decision']=='PASS' and g['activity']==lowered[entry] and g['activity_id']==IDS[root['activity']] and g['compiler_source_sha']==S.REV and g['source_digest']==source and g['program_digest']==S.digest(bodies[entry].encode()) and g['generated_digest']==g['replay_digest']==S.digest(generation['source'].encode()) and g['input_type']=='('+','.join(ins)+')' and S.exact(g['input_parameters'],[{'name':'input'+str(i),'type':t} for i,t in enumerate(ins)]) and g['output_type']==(ret if report else 'bool') and S.exact(g['record_types'],records) and g['typecheck_passed'] is True and g['deterministic_replay'] is True and S.integer(g['repository_writes'],0),'PAIR_GENERATION_ABI')
        call=g['call_closure'];need(call['schema']=='gooo/pure-activity-call-closure/v1' and len(call['activities'])==len(closure)-1,'PAIR_CALLEE_COUNT')
        expected_callees={(lowered[k],self.bodies[k]['id'],S.digest(bodies[k].encode())) for k in closure if k!=entry}
        need({(v['name'],v['activity_id'],v['program_sha256']) for v in call['activities']}==expected_callees,'PAIR_CALLEE_BINDING')
        edges=[]
        for k in closure:
            edges.extend((self.bodies[k]['id'],self.bodies[s['callee']]['id'],i) for i,s in enumerate(x for x in sites if x['caller']==k))
        need(all(S.integer(v['ordinal']) for v in call['edges']) and sorted((v['caller_id'],v['callee_id'],v['ordinal']) for v in call['edges'])==sorted(edges),'PAIR_CALL_EDGES')
        def budget(k,stack=()):
            need(k not in stack and len(stack)<16,'PAIR_RECURSION')
            return sum(1+budget(s['callee'],stack+(k,)) for s in sites if s['caller']==k)
        need(S.integer(call['max_calls_per_invocation'],budget(entry)),'PAIR_CALL_BUDGET')
        need(c['generated_sha256']==S.digest(c['source'].encode()) and c['driver_sha256']==S.digest(c['driver_source'].encode()),'PAIR_GENERATED_SOURCE')
        need(rt['schema']=='gooo/body-composition-runtime/v1' and rt['stage']=='COMPLETE' and rt.get('failure') is None and rt['producer_source_sha']==S.REV and rt['go_version']==self.version and rt['go_tool_sha256']==go_digest and rt['go_tool_selection']=='explicit' and S.exact(rt['producer_module'],{'path':'github.com/kimjooyoon/meta-ontology-go','version':'v0.0.0-20261011201109-83928fa45382'}),'PAIR_PRODUCER')
        need(rt['generated_sha256']==c['generated_sha256'] and rt['driver_sha256']==c['driver_sha256'] and rt['original_source_sha256']==rt['selected_source_sha256']==source and rt['typed_plan_sha256']==plan['typed_plan_sha256'] and S.is_digest(rt['executable_sha256']) and S.is_digest(rt['runtime_suite_sha256']) and all(S.integer(rt[k],0) for k in ('model_calls','finite_total','finite_passed')) and rt['projection_replayed'] is True and rt['runtime_replayed'] is True,'PAIR_RUNTIME_BINDING')
        arithmetic=check_arithmetic(rt,c,IDS[root['activity']],[self.bodies[k]['id'] for k in closure if k!=entry])
        S.Evidence.process(self,rt['toolchain']);S.Evidence.process(self,rt['build']);need(len(rt['runs'])==2,'PAIR_NATIVE_RUNS')
        for run in rt['runs']:S.Evidence.process(self,run)
        need(rt['runs'][0]['stdout_sha256']==rt['runs'][1]['stdout_sha256'],'PAIR_NATIVE_REPEAT')
        if original is None:need(r.get('replay') is None,'PAIR_UNEXPECTED_REPLAY')
        else:
            prior=S.load_receipt(original);rep=r['replay']
            need(S.exact(arithmetic,{k:prior['result']['runtime'].get(k) for k in arithmetic}),'PAIR_REPLAY_ARITHMETIC')
            need(e['replayed_from_sha256']==S.digest(original),'PAIR_RAW_PARENT')
            need(S.is_digest(prior_typed) and rep['prior_result_sha256']==prior_typed,'PAIR_TYPED_PARENT')
            need(rep['schema']=='gooo/workspace-body-replay/v1' and S.integer(rep['model_calls'],0) and S.integer(rep['body_fills_replayed'],0) and S.exact(p,prior['result']['program']) and S.exact(c,prior['result']['composition']),'PAIR_REPLAY_SOURCE')
        need(type(rt['traces']) is list and len(rt['traces'])==18,'PAIR_TRACE_COUNT');values=[]
        for i,(trace,row,supplied_row) in enumerate(zip(rt['traces'],rows,supplied)):
            need(S.integer(trace['case_index'],i) and len(trace['deliveries'])==1,'PAIR_TRACE_ORDER');d=trace['deliveries'][0];ports=[]
            for j,t in enumerate(ins):
                eid,fields=RECORDS[t];v=supplied_row[entry+'.input'+str(j)]
                ports.append({'port':'input'+str(j),'entity_id':eid,'value':v,'fields':[{'id':eid+'/'+f,'name':f,'value':v[f]} for f in fields]})
            need(d['activity_id']==IDS[root['activity']] and S.exact(d['inputs'],ports) and all(d.get(k) is None for k in ('input','input_fields','producer_id','expected','passed','fault','blocked_by')),'PAIR_TYPED_INPUTS')
            actual=d['actual'];want=expected_value(row,entry)
            if report:
                need(type(actual) is dict and set(actual)==set(FACTS) and all(type(actual[f]) is bool for f in FACTS[:6]) and all(S.integer(actual[f]) for f in FACTS[6:]),'PAIR_FACT_KINDS')
                need(S.exact(d['actual_fields'],[{'id':RESULT+'/'+f,'name':f,'value':actual[f]} for f in FACTS]),'PAIR_FIELD_IDS')
            else:need(type(actual) is bool and d.get('actual_fields') is None,'PREFIX_BOOLEAN')
            need(S.exact(actual,want),'PAIR_ORACLE_VALUE');values.append(actual)
        return {'entry':entry,'output_count':18,'typed_scalar_fields':198 if report else 18,'original_input_values':288 if report else (162 if entry==CONTAINS else 180),'output_echoes':0,'native_internal_runs':2,'generated_source':c['source'],'generated_sha256':c['generated_sha256'],'driver_sha256':c['driver_sha256'],'typed_plan_sha256':plan['typed_plan_sha256'],'source_sha256':source,'values':values}


def wire_cases():
    """Eight wire mutations of every35 input fields, plus30 document controls."""
    cases=[]
    for entry in MANIFESTS:
        row={entry+'.input'+str(i):{f:(1 if f=='scope' else 0) for f in RECORDS[t][1]} for i,t in enumerate(port_types(entry))}
        base={'schema':'gooo/body-composition-inputs/v1','inputs':[row]}
        raw=(json.dumps(base,separators=(',',':'))+'\n').encode();validate_inputs(raw,entry)
        for port,t in enumerate(port_types(entry)):
            key=entry+'.input'+str(port)
            for field in RECORDS[t][1]:
                for family,token,code in (('null','null','RECORD_INTEGER'),('bool','false','RECORD_INTEGER'),('text','"0"','RECORD_INTEGER'),('float','0.0','RECORD_INTEGER'),('exponent','0e0','RECORD_INTEGER'),('overflow','9223372036854775808','RECORD_INTEGER'),('duplicate',None,'DUPLICATE_FIELD'),('missing',None,'RECORD_FIELDS')):
                    obj=json.loads(raw);record=obj['inputs'][0][key]
                    if family=='missing':del record[field];bad=json.dumps(obj,separators=(',',':')).encode()
                    else:
                        record[field]='WIRE_MUTATION_TOKEN';bad=json.dumps(obj,separators=(',',':')).encode()
                        replacement=(str(row[key][field])+',"'+field+'":'+str(row[key][field])) if family=='duplicate' else token
                        bad=bad.replace(b'"WIRE_MUTATION_TOKEN"',replacement.encode(),1)
                    cases.append({'name':entry.split(':')[-1]+'-input'+str(port)+'-'+field+'-'+family,'entry':entry,'raw':bad,'code':code})
        changed=json.loads(raw);changed['inputs'][0][entry+'.input0']['unknown']=0
        no_rows=json.loads(raw);no_rows['inputs']=[]
        no_record=json.loads(raw);no_record['inputs'][0][entry+'.input0']=None
        docs=[('object',b'null','OBJECT_REQUIRED'),('unknown',raw.replace(b'"schema":',b'"extra":0,"schema":',1),'DOCUMENT_FIELDS'),
              ('schema',raw.replace(b'body-composition-inputs/v1',b'body-composition-inputs/v0'),'DOCUMENT_FIELDS'),('rows',json.dumps(no_rows,separators=(',',':')).encode(),'ROW_COUNT'),
              ('port',raw.replace(b'.input0',b'.input2',1),'PORT_ROSTER'),('record',json.dumps(no_record,separators=(',',':')).encode(),'RECORD_OBJECT'),
              ('record-unknown',json.dumps(changed,separators=(',',':')).encode(),'RECORD_FIELDS'),('utf8',b'\xff','INVALID_RAW_UTF8'),('trailing',raw+b'{}','TRAILING_JSON'),('malformed',b'{','MALFORMED_JSON')]
        cases.extend({'name':entry.split(':')[-1]+'-document-'+name,'entry':entry,'raw':bad,'code':code} for name,bad,code in docs)
    need(len(cases)==310 and len({v['name'] for v in cases})==310,'PAIR_WIRE_ROSTER')
    return cases


def run_wire(save):
    result=[]
    cases=wire_cases()
    refs=[save('pair-wire-'+v['name'],v['raw']) for v in cases]
    for case,ref in zip(cases,refs):
        try:validate_inputs(case['raw'],case['entry'])
        except S.InputError as exc:need(exc.code==case['code'],'PAIR_WIRE_INTENDED_GATE')
        else:raise S.ReceiptError('PAIR_WIRE_ACCEPTED')
        result.append({k:case[k] for k in ('name','entry','code')}|{'rawref':ref,'observed_diagnostic':case['code']})
    return result


def tamper_cases(exraw,rraw,span):
    ex=S.load_receipt(exraw);rep=S.load_receipt(rraw);root=['result'];runtime=root+['runtime'];generation=root+['composition','steps',0,'generation','report'];delivery=runtime+['traces',0,'deliveries',0]
    other=lambda value:'sha256:'+('1' if value[7]!='1' else '2')+value[8:]
    descriptors=[('entry','execution',delivery+['activity_id'],'gooo://wrong','PAIR_TYPED_INPUTS'),
        ('input','execution',delivery+['inputs',0,'value','a'],123,'PAIR_TYPED_INPUTS'),
        ('input-field','execution',delivery+['inputs',0,'fields',0,'id'],'gooo://wrong','PAIR_TYPED_INPUTS'),
        ('value','execution',delivery+['actual','valid'],not ex['result']['runtime']['traces'][0]['deliveries'][0]['actual']['valid'],'PAIR_ORACLE_VALUE'),
        ('kind','execution',delivery+['actual','valid'],None,'PAIR_FACT_KINDS'),
        ('field','execution',delivery+['actual_fields',0,'id'],'gooo://wrong','PAIR_FIELD_IDS'),
        ('field-order','execution',delivery+['actual_fields'],list(reversed(ex['result']['runtime']['traces'][0]['deliveries'][0]['actual_fields'])),'PAIR_FIELD_IDS'),
        ('producer','execution',runtime+['producer_source_sha'],'0'*40,'PAIR_PRODUCER'),
        ('go','execution',runtime+['go_tool_sha256'],other(ex['result']['runtime']['go_tool_sha256']),'PAIR_PRODUCER'),
        ('generated','execution',root+['composition','generated_sha256'],other(ex['result']['composition']['generated_sha256']),'PAIR_GENERATED_SOURCE'),
        ('driver','execution',root+['composition','driver_sha256'],other(ex['result']['composition']['driver_sha256']),'PAIR_GENERATED_SOURCE'),
        ('plan','execution',runtime+['typed_plan_sha256'],other(ex['result']['runtime']['typed_plan_sha256']),'PAIR_RUNTIME_BINDING'),
        ('parameter','execution',generation+['input_parameters',0,'type'],'Boolean','PAIR_GENERATION_ABI'),
        ('callee','execution',generation+['call_closure','activities',0,'activity_id'],'gooo://wrong','PAIR_CALLEE_BINDING'),
        ('callee-body','execution',generation+['call_closure','activities',0,'program_sha256'],'sha256:'+'0'*64,'PAIR_CALLEE_BINDING'),
        ('raw-parent','replay',['replayed_from_sha256'],other(rep['replayed_from_sha256']),'PAIR_RAW_PARENT'),
        ('typed-parent','replay',root+['replay','prior_result_sha256'],other(rep['result']['replay']['prior_result_sha256']),'PAIR_TYPED_PARENT'),
        ('pure-calls','execution',root+['program','pure_calls'],False,'PAIR_CALL_SITES')]
    result=[]
    for name,target,path,value,code in descriptors:
        raw=exraw if target=='execution' else rraw;text,(start,end)=span(raw,path)
        need(type(text) is str and text.encode('utf-8')==raw and type(start) is int and type(end) is int and 0<=start<end<=len(text),'PAIR_TAMPER_SPAN')
        edits=[(start,end,json.dumps(value,separators=(',',':')))]
        if name=='value':
            same,(start,end)=span(raw,delivery+['actual_fields',0,'value'])
            need(same==text and type(start) is int and type(end) is int and 0<=start<end<=len(text),'PAIR_TAMPER_SPAN')
            edits.append((start,end,json.dumps(value)))
        ordered=sorted(edits)
        need(all(ordered[i][1]<=ordered[i+1][0] for i in range(len(ordered)-1)),'PAIR_TAMPER_OVERLAP')
        changed=text
        for start,end,token in reversed(ordered):changed=changed[:start]+token+changed[end:]
        changed=changed.encode('utf-8')
        need(changed!=raw,'PAIR_TAMPER_NO_CHANGE');result.append({'name':name,'target':target,'raw':changed,'code':code})
    return result


def check_tampers(checker,exraw,rraw,inputs,manifest,go_digest,prior,save,span):
    checker.inspect(exraw,REPORT,inputs,manifest,go_digest)
    checker.inspect(rraw,REPORT,inputs,manifest,go_digest,original=exraw,prior_typed=prior)
    cases=tamper_cases(exraw,rraw,span);refs=[save('pair-tamper-'+v['name'],v['raw']) for v in cases];result=[]
    for v,ref in zip(cases,refs):
        try:checker.inspect(v['raw'],REPORT,inputs,manifest,go_digest,original=exraw if v['target']=='replay' else None,prior_typed=prior if v['target']=='replay' else None)
        except S.ReceiptError as exc:need(exc.code==v['code'],'PAIR_TAMPER_INTENDED_GATE: '+v['name']+' '+exc.code)
        else:raise S.ReceiptError('PAIR_TAMPER_ACCEPTED')
        result.append({k:v[k] for k in ('name','target','code')}|{'rawref':ref,'observed_diagnostic':v['code']})
    return result
