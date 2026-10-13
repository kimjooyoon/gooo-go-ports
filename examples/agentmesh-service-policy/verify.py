#!/usr/bin/env python3
"""Fresh portable bounded Agent Mesh oracle/native/workspace checks, offline.

Run generate.py without --check first; a retained prototype origin is never
accepted as a newly verified public integration by this script.
"""
import argparse,datetime,hashlib,json,os,re,subprocess
from pathlib import Path
if not __debug__: raise SystemExit("Python optimization disables integrity assertions; run without -O/PYTHONOPTIMIZE")
from verification_checks import check_native_rows, check_workspace_trace, check_native_refinement, check_witness_oracle, CHECKED, check_native_checked, check_domain_trace, check_domain_reference
ROOT=Path(__file__).resolve().parent
REPO=ROOT.parents[1]
PIN='83928fa4538245abe33b08549da47f8cbdc50ed2'
def sha(raw):return hashlib.sha256(raw).hexdigest()
def encoded(x):return (json.dumps(x,indent=2,sort_keys=True)+'\n').encode()
def ident(p):r=p.read_bytes();return {'path':str(p),'bytes':len(r),'sha256':sha(r)}
def function(raw,name):
    prefix=('func '+name+'(').encode();start=raw.index(prefix);end=raw.index(b'\n}\n',start)+3;return raw[start:end]

def checked_case(kind,row):
    fields=CHECKED[kind]['inputs'];semantic={'matched':row.get('matched',row.get('want'))} if kind=='matcher' else row['want']
    return {'name':row['name'],'api':kind,'facts':{k:row[k]for k in fields},'want':dict(domain_status='VALID_INPUT',invalid_field='',**semantic),'coverage':['preserved-valid']}


def verify_domain_checked(args,out,run,bindings,old_rows,impacts,refinements):
    checked=json.loads((ROOT/'checked-cases.json').read_bytes());domain=json.loads((ROOT/'domain-cases.json').read_bytes())
    assert len(checked)==250 and len(domain)==662 and sum('ascii-512'in r['coverage']for r in domain)==512
    assert {k:sum(r['api']==k for r in checked)for k in CHECKED}=={'matcher':32,'impact':113,'refinement':105}
    reference=json.loads((ROOT/'domain-reference.json').read_bytes())
    check_domain_reference(reference)
    oracle=out/'domain-oracle';oracle.mkdir();(oracle/'go.mod').write_text('module private.invalid/gooo-domain-oracle\n\ngo 1.27.2\n')
    (oracle/'domain.go').write_text('package main\nimport("regexp";"strings")\n'+reference['declarations']+'\n'+'\n\n'.join(reference['functions'].values())+'\n')
    (oracle/'main.go').write_text('''package main
import("encoding/json";"os";"runtime")
type Domain struct{Name string `json:"name"`;Activity string `json:"activity"`;Input string `json:"input"`;Want bool `json:"want"`}
type Checked struct{Name string `json:"name"`;API string `json:"api"`;Facts map[string]string `json:"facts"`}
func main(){var in struct{Domain []Domain `json:"domain"`;Checked []Checked `json:"checked"`};if err:=json.NewDecoder(os.Stdin).Decode(&in);err!=nil{panic(err)};d:=make([]map[string]any,0,len(in.Domain));for _,r:=range in.Domain{var v bool;switch r.Activity{case "ValidateServiceType":v=validType(r.Input);case "ValidateServiceName":v=validName(r.Input);case "ValidateServicePattern":v=validPattern(r.Input);default:panic("unknown activity")};d=append(d,map[string]any{"name":r.Name,"activity":r.Activity,"input":r.Input,"valid":v})};c:=make([]map[string]any,0,len(in.Checked));for _,r:=range in.Checked{keys:=map[string][]string{"matcher":{"pattern","req_type","req_name"},"impact":{"before","after","req_type","req_name"},"refinement":{"before","after"}}[r.API];if len(keys)==0{panic("unknown API")};invalid:="";for _,k:=range keys{v:=validPattern(r.Facts[k]);if k=="req_type"{v=validType(r.Facts[k])};if k=="req_name"{v=validName(r.Facts[k])};if !v&&invalid==""{invalid=k}};c=append(c,map[string]any{"name":r.Name,"api":r.API,"facts":r.Facts,"invalid_field":invalid})};json.NewEncoder(os.Stdout).Encode(map[string]any{"go_version":runtime.Version(),"domain":d,"checked":c})}
''')
    classified=json.loads(run('frozen-domain-oracle',[args.go,'run','.'],encoded({'domain':domain,'checked':checked}),cwd=oracle))
    assert classified['go_version']=='go1.27.2' and len(classified['domain'])==662 and len(classified['checked'])==250
    for actual,expected in zip(classified['domain'],domain):
        assert set(actual)=={'name','activity','input','valid'} and all(actual[k]==expected[k]for k in ['name','activity','input'])
        assert type(actual['valid'])is bool and actual['valid']==expected['want']
    for actual,expected in zip(classified['checked'],checked):
        assert set(actual)=={'name','api','facts','invalid_field'} and all(actual[k]==expected[k]for k in ['name','api','facts'])
        assert actual['invalid_field']==expected['want']['invalid_field']
        assert (actual['invalid_field']=='')==(expected['want']['domain_status']=='VALID_INPUT')
    # These APIs accept raw typed facts. No Decode*/Validate* grammar prefilter
    # stands between the malformed input and the generated checked activity.
    native_cases=[checked_case(k,r)for k,rs in [('matcher',old_rows),('impact',impacts),('refinement',refinements)]for r in rs]+checked
    host=out/'checked-native-host';host.mkdir();module='github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy'
    (host/'go.mod').write_text('module private.invalid/gooo-checked-host\n\ngo 1.27.2\n\nrequire '+module+' v0.0.0\nreplace '+module+' => '+json.dumps(str(ROOT))+'\n')
    (host/'main.go').write_text('''package main
import("encoding/json";"os";p "github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy")
func main(){var in struct{Checked []struct{API string `json:"api"`;Facts json.RawMessage `json:"facts"`} `json:"checked"`;Domain []struct{Name string `json:"name"`;Activity string `json:"activity"`;Input string `json:"input"`} `json:"domain"`};if err:=json.NewDecoder(os.Stdin).Decode(&in);err!=nil{panic(err)};results:=make([]any,0,len(in.Checked));for _,r:=range in.Checked{switch r.API{case "matcher":var f p.MatcherFacts;if json.Unmarshal(r.Facts,&f)!=nil{panic("facts")};v,e:=p.EvaluateMatcherChecked(f);if e!=nil{panic(e)};results=append(results,v);case "impact":var f p.ImpactFacts;if json.Unmarshal(r.Facts,&f)!=nil{panic("facts")};v,e:=p.EvaluateImpactChecked(f);if e!=nil{panic(e)};results=append(results,v);case "refinement":var f p.RefinementFacts;if json.Unmarshal(r.Facts,&f)!=nil{panic("facts")};v,e:=p.EvaluateRefinementChecked(f);if e!=nil{panic(e)};results=append(results,v);default:panic("API")}};ds:=make([]map[string]any,0,len(in.Domain));for _,r:=range in.Domain{f:=p.MatcherFacts{Pattern:"*",ReqType:"mcp",ReqName:"x"};switch r.Activity{case "ValidateServiceType":f.ReqType=r.Input;case "ValidateServiceName":f.ReqName=r.Input;case "ValidateServicePattern":f.Pattern=r.Input;default:panic("activity")};ds=append(ds,map[string]any{"name":r.Name,"activity":r.Activity,"input":r.Input,"valid":p.ValidateMatcher(f)==nil})};json.NewEncoder(os.Stdout).Encode(map[string]any{"checked":results,"domain":ds})}
''')
    native=json.loads(run('native-checked-domain',[args.go,'run','.'],encoded({'checked':native_cases,'domain':domain}),cwd=host))
    check_native_checked(native['checked'],native_cases,bindings)
    assert native['domain']==classified['domain']
    checked_batches=[];domain_batches=[]
    for kind in CHECKED:
        selected=[r for r in checked if r['api']==kind];config=CHECKED[kind];entry=config['package']+':'+config['activity'];prefix='checked-'+kind
        workspace={'matcher':'agentmesh-matcher.workspace.json','impact':'service-grant-impact.workspace.json','refinement':'service-grant-refinement.workspace.json'}[kind]
        for offset in range(0,len(selected),16):
            group=selected[offset:offset+16];corpus={'schema':'gooo/body-composition-cases/v1','cases':[{'inputs':{entry+'.input'+str(i):r['facts'][k]for i,k in enumerate(config['inputs'])},'expected':{entry:r['want']}}for r in group]}
            label='checked-direct-'+kind+'-'+str(offset//16);casefile=out/(label+'-cases.json');casefile.write_bytes(encoded(corpus));assert casefile.stat().st_size<=32768
            raw=run(label+'-execute',[args.compiler,'package','execute','--json','--cases',casefile,'--go',args.go,workspace],cwd=REPO)
            replay=run(label+'-replay',[args.compiler,'package','replay','--json','--receipt',out/(label+'-execute.stdout'),'--cases',casefile,'--go',args.go,workspace],cwd=REPO)
            audit_checked_receipts(raw,replay,corpus,casefile,REPO/workspace,prefix,ROOT/'receipts'/(prefix+'-program.json'),ROOT/'receipts'/(prefix+'-body-codegen.json'))
            checked_batches.append({'kind':kind,'case_count':len(group),'cases':ident(casefile),'execution':ident(out/(label+'-execute.stdout')),'replay':ident(out/(label+'-replay.stdout'))})
    probe=out/'domain-probe';probe.mkdir();(probe/'domain.gooo').write_bytes((ROOT/'domain.gooo').read_bytes())
    for activity in ['ValidateServiceType','ValidateServiceName','ValidateServicePattern']:
        selected=[r for r in domain if r['activity']==activity];entry='gooo/service-domain:'+activity
        workspace=probe/(activity+'.workspace.json');workspace.write_bytes(encoded({'schema':'gooo/package-workspace-manifest/v1','entry':{'package_path':'gooo/service-domain','activity':activity},'packages':[{'path':'gooo/service-domain','sources':['domain.gooo']}]}))
        for offset in range(0,len(selected),64):
            group=selected[offset:offset+64];corpus={'schema':'gooo/body-composition-cases/v1','cases':[{'inputs':{entry:r['input']},'expected':{entry:r['want']}}for r in group]}
            label='domain-'+activity+'-'+str(offset//64);casefile=out/(label+'-cases.json');casefile.write_bytes(encoded(corpus));assert casefile.stat().st_size<=32768
            raw=run(label+'-execute',[args.compiler,'package','execute','--json','--cases',casefile,'--go',args.go,workspace.name],cwd=probe)
            replay=run(label+'-replay',[args.compiler,'package','replay','--json','--receipt',out/(label+'-execute.stdout'),'--cases',casefile,'--go',args.go,workspace.name],cwd=probe)
            a=json.loads(raw);b=json.loads(replay);assert a['result']['program']==b['result']['program']
            p=a['result']['program'];packages=p['workspace_image']['packages'];assert len(packages)==1 and packages[0]['path']=='gooo/service-domain'
            sources=packages[0]['sources'];assert len(sources)==1 and sources[0]['filename']=='domain.gooo' and sources[0]['source_digest']==source_digest((ROOT/'domain.gooo').read_bytes())
            assert (probe/'domain.gooo').read_bytes()==(ROOT/'domain.gooo').read_bytes()
            for result in [a,b]:
                audit_runtime(result,corpus,casefile,workspace)
                report=result['result']['composition']['steps'][0]['generation']['report'];assert report['compiler_source_sha']==PIN
                assert report['source_digest']=='sha256:'+sha(p['lowered_gooo_source'].encode()) and report['typecheck_passed'] and report['deterministic_replay']
                for i,(trace,case) in enumerate(zip(result['result']['runtime']['traces'],corpus['cases'])):check_domain_trace(activity,trace,i,case,p,report)
            assert b['replayed_from_sha256']=='sha256:'+sha(raw)
            domain_batches.append({'activity':activity,'case_count':len(group),'cases':ident(casefile),'execution':ident(out/(label+'-execute.stdout')),'replay':ident(out/(label+'-replay.stdout'))})
    raw_compat=[]
    for prefix,workspace in [('matcher','agentmesh-matcher-raw.workspace.json'),('impact','service-grant-impact-raw.workspace.json'),('refinement','service-grant-refinement-raw.workspace.json')]:
        corpus=json.loads((ROOT/'receipts'/(prefix+'-seed-cases.json')).read_bytes());a=out/'fresh-generation'/(prefix+'-execute.stdout');b=out/'fresh-generation'/(prefix+'-replay.stdout')
        audit_checked_receipts(a.read_bytes(),b.read_bytes(),corpus,ROOT/'receipts'/(prefix+'-seed-cases.json'),REPO/workspace,prefix,ROOT/'receipts'/(prefix+'-program.json'),ROOT/'receipts'/(prefix+'-body-codegen.json'))
        raw_compat.append({'kind':prefix,'case_count':len(corpus['cases']),'workspace':ident(REPO/workspace),'execution':ident(a),'replay':ident(b),'scope':'Explicit raw compatibility; canonical-domain precondition remains'})
    return {'domain':{'cases':662,'ascii_predicate_observations':512,'native_matches_frozen_oracle':True,'workspace_batches':domain_batches,'frozen_reference':ident(ROOT/'domain-reference.json'),'fresh_oracle':ident(out/'frozen-domain-oracle.stdout')},'checked':{'additional_cases':250,'invalid_cases':243,'valid_cases':7,'preserved_valid_cases':180,'native_cases':430,'fields':{'matcher':3,'impact':5,'refinement':10},'native':ident(out/'native-checked-domain.stdout'),'workspace_batches':checked_batches,'direct_raw_text_without_go_grammar_prefilter':True,'first_invalid_argument_verified':True,'raw_compatibility':raw_compat}}


def source_digest(raw):
    value=json.dumps(raw.decode(),ensure_ascii=False,separators=(',',':')).replace('<','\\u003c').replace('>','\\u003e').replace('&','\\u0026').replace('\u2028','\\u2028').replace('\u2029','\\u2029')
    return 'sha256:'+sha(value.encode())


def audit_runtime(result,corpus,casefile,workspace):
    assert result['schema']=='gooo/workspace-body-execution-receipt/v1' and result['decision']=='PASS'
    assert result['manifest_digest']=='sha256:'+sha(workspace.read_bytes()) and result['cases_digest']=='sha256:'+sha(casefile.read_bytes())
    rt=result['result']['runtime'];assert rt['producer_source_sha']==PIN and rt['go_version'].split()[:3]==['go','version','go1.27.2'] and rt['projection_replayed'] and rt['runtime_replayed'] and rt['model_calls']==0
    assert rt['finite_passed']==rt['finite_total']==len(corpus['cases']) and len(rt['traces'])==len(corpus['cases'])
    assert rt['runs'] and all(r['started']and r['completed']and r['exit_code']==0 and not r['timed_out']and not r['output_truncated']for r in rt['runs'])


def audit_checked_receipts(raw,replayed,corpus,casefile,workspace,prefix,program_path,report_path):
    a=json.loads(raw);b=json.loads(replayed);snapshot=json.loads(program_path.read_bytes());report=json.loads(report_path.read_bytes())['report']
    for result in [a,b]:
        audit_runtime(result,corpus,casefile,workspace);assert result['result']['program']==snapshot
        for i,(trace,case) in enumerate(zip(result['result']['runtime']['traces'],corpus['cases'])):check_workspace_trace(prefix,trace,i,case,snapshot,report)
    assert b['replayed_from_sha256']=='sha256:'+sha(raw)

def main():
    ap=argparse.ArgumentParser();ap.add_argument('--compiler',type=Path,required=True);ap.add_argument('--go',type=Path,required=True);ap.add_argument('--out',type=Path,default=ROOT/'out/verification');args=ap.parse_args()
    args.compiler=args.compiler.resolve();args.go=args.go.resolve();out=args.out.resolve();out.mkdir(parents=True,exist_ok=True);out.chmod(0o700)
    env=os.environ.copy();env.update(GOTOOLCHAIN='local',GOWORK='off',GOPROXY='off',GOSUMDB='off');commands=[]
    def run(label,cmd,data=None,cwd=ROOT,expect=0,timeout=60):
        before=datetime.datetime.now(datetime.timezone.utc).isoformat();failure=None
        try:
            r=subprocess.run([str(x) for x in cmd],input=data,cwd=cwd,env=env,capture_output=True,timeout=timeout)
            stdout,stderr,exit_code=r.stdout,r.stderr,r.returncode
        except subprocess.TimeoutExpired as exc:
            stdout,stderr,exit_code=exc.stdout or b'',exc.stderr or b'',None;failure=exc
        for name,raw in [('stdout',stdout),('stderr',stderr)]:p=out/(label+'.'+name);p.write_bytes(raw);p.chmod(0o600)
        commands.append({'label':label,'command':[str(x) for x in cmd],'started_at_utc':before,'finished_at_utc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'exit_code':exit_code,'timed_out':failure is not None,'timeout_seconds':timeout,'stdout':ident(out/(label+'.stdout')),'stderr':ident(out/(label+'.stderr'))})
        (out/'commands.json').write_bytes(encoded(commands));(out/'commands.json').chmod(0o600)
        if failure is not None:raise RuntimeError(label+' timed out; inspect retained private outputs') from failure
        if exit_code!=expect:raise RuntimeError(label+' failed; inspect retained private outputs')
        return stdout
    run('verifier-helper-tests',['python3','-B','-m','unittest','-v','verify_test'])
    generation=json.loads((ROOT/'generation.json').read_bytes());assert generation['schema']=='gooo/agentmesh-native-generation/v1' and generation['origin']=='fresh_pinned_generation','fresh generator execution is required before public verification'
    source=json.loads((ROOT/'source-pin.json').read_bytes());assert source['revision']=='9fd1a41a15174df0aaef601c942410c2116bff91'
    for f in source['archive_files']+source['canonical_components']:
        raw=(ROOT/f['filename']).read_bytes();assert len(raw)==f['bytes'] and sha(raw)==f['sha256']
    assert (ROOT/'LICENSE-APACHE-2.0').read_bytes()==(ROOT/'upstream/LICENSE').read_bytes()
    extraction=json.loads((ROOT/'oracle/extraction.json').read_bytes());assert source['exact_function_bytes']==extraction['exact_function_bytes']
    for name,path in [('MatchServicePattern','upstream/api/tar.go.txt'),('ParseServiceTarget','upstream/api/network.go.txt')]:
        original=function((ROOT/path).read_bytes(),name);actual=function((ROOT/'oracle/extracted.go').read_bytes(),name)
        assert original==actual and sha(actual)==source['exact_function_bytes'][name]
    regex=source['exact_regexp_assignment'];assert ('var '+regex) in (ROOT/'oracle/extracted.go').read_text() and regex in (ROOT/'upstream/api/network.go.txt').read_text()
    cases=json.loads((ROOT/'matcher-cases.json').read_bytes());impacts=json.loads((ROOT/'impact-cases.json').read_bytes());assert len(cases)==63 and len(impacts)==16
    refinements=json.loads((ROOT/'refinement-cases.json').read_bytes());assert len(refinements)==101
    family_cells={(r['before_family'],r['after_family']) for r in refinements if 'matrix' in r['coverage']};assert len(family_cells)==36
    tests=(ROOT/'upstream/api/tar_test.go.txt').read_text();start=tests.index('func TestMatchServicePattern(');end=tests.index('\nfunc ',start+1);body=tests[start:end]
    official=[(a,b,c,d=='true') for a,b,c,d in re.findall(r'\{"([^"\n]*)", "([^"\n]*)", "([^"\n]*)", (true|false)\}',body)]
    assert len(official)==12 and [(r['pattern'],r['req_type'],r['req_name'],r['want']) for r in cases[:12]]==official and all(r['upstream'] for r in cases[:12])
    go_version=run('go-version',[args.go,'version']).decode().strip();assert go_version.split()[:3]==['go','version','go1.27.2']
    fresh=json.loads(run('fresh-extracted-upstream-oracle',[args.go,'run','./cmd/oracle','-cases',ROOT/'matcher-cases.json','-impact-cases',ROOT/'impact-cases.json'],cwd=ROOT/'oracle'))
    assert fresh['schema']=='gooo/agentmesh-matcher-oracle/v1' and fresh['go_version']=='go1.27.2' and fresh['source_revision']==source['revision']
    rows=fresh['observations'];assert len(rows)==63 and rows==json.loads((ROOT/'oracle/observations.json').read_bytes())
    assert len(fresh['impact_matches'])==16
    for r,c in zip(fresh['impact_matches'],impacts):assert r['name']==c['name'] and r['before_match']==c['want']['before_match'] and r['after_match']==c['want']['after_match']
    run('fresh-generation-check',['python3',ROOT/'generate.py','--compiler',args.compiler,'--go',args.go,'--out',out/'fresh-generation','--check'])
    # A clean CI runner may spend more than one minute compiling race instrumentation.
    # Keep the full test suite and a finite deadline; other commands retain 60 seconds.
    tests=run('go-tests',[args.go,'test','-json','-race','./...'],timeout=300);run('go-vet',[args.go,'vet','./...']);run('oracle-vet',[args.go,'vet','./...'],cwd=ROOT/'oracle')
    run('cli-build',[args.go,'build','-trimpath','-o',out/'evaluate','./cmd/evaluate'])
    matcher=json.loads(run('native-matcher',[out/'evaluate','-mode','matcher'],encoded([{k:r[k] for k in ['pattern','req_type','req_name']} for r in rows])))
    impact=json.loads(run('native-impact',[out/'evaluate','-mode','impact'],encoded([{k:r[k] for k in ['before','after','req_type','req_name']} for r in impacts])))
    refinement=json.loads(run('native-refinement',[out/'evaluate','-mode','refinement'],encoded([{k:r[k] for k in ['before','after']} for r in refinements])))
    expected_bindings={'compiler_source_sha':PIN,'generation_sha256':'sha256:'+sha((ROOT/'generation.json').read_bytes()),'generation_origin':'fresh_pinned_generation','assets':generation['assets']}
    check_native_rows(matcher,impact,rows,impacts,expected_bindings)
    check_native_refinement(refinement,refinements,expected_bindings)
    witness_requests=[]
    for actual,case in zip(refinement,refinements):
        d=actual['decision']
        if d['has_witness']:
            for side,want in [('before',False),('after',True)]:
                witness_requests.append({'name':case['name']+'/'+side,'pattern':actual['facts'][side],'req_type':d['witness_type'],'req_name':d['witness_name'],'want':want})
    assert len(witness_requests)==126
    witness_casefile=out/'refinement-witness-oracle-cases.json';witness_casefile.write_bytes(encoded(witness_requests));witness_casefile.chmod(0o600)
    witness_oracle=json.loads(run('fresh-extracted-witness-oracle',[args.go,'run','./cmd/oracle','-cases',witness_casefile,'-impact-cases',ROOT/'impact-cases.json'],cwd=ROOT/'oracle'))
    check_witness_oracle(witness_oracle,witness_requests,source['revision'])
    assert witness_oracle['impact_matches']==fresh['impact_matches']
    refinement_states={key:sum(r['decision']['status']==key for r in refinement) for key in ['SUPPORTED_SUBSET','WIDENING_WITNESS','UNPROVEN']}
    assert refinement_states=={'SUPPORTED_SUBSET':38,'WIDENING_WITNESS':63,'UNPROVEN':0}

    states=sorted(set(row['decision']['change'] for row in impact));assert states==['REVOKED','UNCHANGED_ALLOW','UNCHANGED_DENY','WIDENED']
    batches=[];refinement_batches=[]
    for prefix,values,entry,fields in [('matcher',rows,'go/google/agentmesh/api:MatchServicePattern',['pattern','req_type','req_name']),('impact',impacts,'app/service-grant-impact:CompareServiceGrant',['before','after','req_type','req_name']),('refinement',refinements,'app/service-grant-refinement:AnalyzeServiceGrant',['before','after'])]:
        config=CHECKED[prefix];entry=config['package']+':'+config['activity']
        for offset in range(0,len(values),16):
            selected=values[offset:offset+16];corpus={'schema':'gooo/body-composition-cases/v1','cases':[{'inputs':{entry+'.input'+str(i):r[k] for i,k in enumerate(fields)},'expected':{entry:checked_case(prefix,r)['want']}} for r in selected]}
            label=prefix+'-'+str(offset//16);casefile=out/(label+'-cases.json');casefile.write_bytes(encoded(corpus));casefile.chmod(0o600);assert casefile.stat().st_size<=32768
            workspace={'matcher':'agentmesh-matcher.workspace.json','impact':'service-grant-impact.workspace.json','refinement':'service-grant-refinement.workspace.json'}[prefix];raw=run(label+'-execute',[args.compiler,'package','execute','--json','--cases',casefile,'--go',args.go,workspace],cwd=REPO);execute=json.loads(raw)
            replay=json.loads(run(label+'-replay',[args.compiler,'package','replay','--json','--receipt',out/(label+'-execute.stdout'),'--cases',casefile,'--go',args.go,workspace],cwd=REPO))
            snapshot=json.loads((ROOT/'receipts'/('checked-'+prefix+'-program.json')).read_bytes())
            for result in [execute,replay]:
                assert result['schema']=='gooo/workspace-body-execution-receipt/v1' and result['decision']=='PASS' and result['manifest_digest']=='sha256:'+sha((REPO/workspace).read_bytes()) and result['cases_digest']=='sha256:'+sha(casefile.read_bytes())
                assert result['result']['program']==snapshot
                rt=result['result']['runtime'];assert rt['producer_source_sha']==PIN and rt['go_version']==go_version and rt['projection_replayed'] and rt['runtime_replayed'] and rt['finite_passed']==rt['finite_total']==len(selected) and rt['model_calls']==0
                assert rt['runs'] and all(r['started'] and r['completed'] and r['exit_code']==0 and not r['timed_out'] and not r['output_truncated'] for r in rt['runs'])
                assert len(rt['traces'])==len(selected)
                for index,(trace,case) in enumerate(zip(rt['traces'],corpus['cases'])):
                    check_workspace_trace('checked-'+prefix,trace,index,case,snapshot,json.loads((ROOT/'receipts'/('checked-'+prefix+'-body-codegen.json')).read_bytes())['report'])
            assert replay['replayed_from_sha256']=='sha256:'+sha(raw)
            (refinement_batches if prefix=='refinement' else batches).append({'case_count':len(selected),'cases':ident(casefile),'execution':ident(out/(label+'-execute.stdout')),'replay':ident(out/(label+'-replay.stdout'))})
    domain_checked=verify_domain_checked(args,out,run,expected_bindings,rows,impacts,refinements)
    events=[json.loads(line) for line in tests.splitlines()];passed=[r['Test'] for r in events if r.get('Action')=='pass' and 'Test' in r]
    assert sum(name.startswith('TestEveryBoundAssetTamperRejected/') for name in passed)==len(generation['assets'])
    for name in ['TestEveryExactMatcherAndImpactFieldMalformed','TestUnsupportedDomainRefusalsAndBoundaries','TestCLIUsesGeneratedGoooAndWholeBatchRefusal','TestRehashedSourceProjectionAndGenerationTamperRejected','TestCLICountAndByteBoundaries','TestSinglePatternRefinementMatrixAndWitnesses','TestRefinementValidatesBothPatternsBeforeShortcuts','TestRefinementOutputContradictionsRefused','TestCLIRefinementAndWholeBatchRefusal','TestRefinementRehashedProgramIdentityTamperRejected']:assert name in passed
    proof={'schema':'gooo/agentmesh-native-verification/v1','status':'FINITE_FRESH_ORACLE_NATIVE_WORKSPACE_REPLAY_PASS','finished_at_utc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'source_revision':source['revision'],'compiler_source_sha':PIN,'go_version':go_version,'matcher':{'upstream_cases':12,'total_cases':63,'fresh_oracle':ident(out/'fresh-extracted-upstream-oracle.stdout'),'native':ident(out/'native-matcher.stdout')},'original_impact':{'cases':16,'fields_compared':3,'states':states,'native':ident(out/'native-impact.stdout')},'refinement':{'cases':101,'fields_compared':8,'family_cells':36,'states':refinement_states,'fresh_oracle_witnesses':63,'fresh_oracle_matcher_observations':126,'fresh_oracle':ident(out/'fresh-extracted-witness-oracle.stdout'),'oracle_cases':ident(witness_casefile),'native':ident(out/'native-refinement.stdout'),'workspace_batches':refinement_batches},'strict_json':{'every_field_mutations':sum(name.startswith('TestEveryExactMatcherAndImpactFieldMalformed/') for name in passed),'transactional_later_failure':True,'max_count':1024,'max_batch_bytes':1048576,'max_component_bytes':128},'verifier_helper_contracts':{'native_array_cardinality':True,'all_raw_impact_facts_and_strict_types':True,'complete_typed_workspace_delivery_identity':True,'all_raw_refinement_facts_and_strict_types':True,'checked_witness_domain':True,'optimized_python_refused':True},'public_sources':generation['public_sources'],'bound_assets':len(generation['assets']),'asset_tamper_refusals':len(generation['assets']),'workspace_batches':batches,'commands':commands,'limits':['Canonical printable ASCII scheme/name domain only; no full URI parser','Threefield per-request impact plus single-pattern refinement; not role unions, live authorization, availability preservation or automatic application','No CEL eligibility/role checks/token trust/current-state provider/effect dispatcher/private APIProof replay','Source hashes identify consistency under trusted executable, not issuer/build/binary attestation','Finite79 old and101 refinement native observations overlap their180workspace cases;126fresh witness matcher observations validate63witnesses. This is not proof of compiler correctness or production safety']}
    proof.update(domain_checked)
    proof['verifier_helper_contracts'].update(checked_typed_records=True, checked_default_workspace_routing=True, raw_compatibility_explicit=True, direct_domain_execution=True)
    (out/'verification.json').write_bytes(encoded(proof));(out/'verification.json').chmod(0o600)
    print('PASS: fresh upstream63matcher;16originalthreefield impacts; 101single-pattern refinement decisions and63fresh-oracle checked witnesses; actual imported workspace execute/replay; strict native wrapper and complete bindings; checked default Gooo domain gates: 250directcases/662predicateobservations')
if __name__=='__main__':main()
