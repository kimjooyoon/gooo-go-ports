'use strict';
const fs=require('node:fs');
const path=require('node:path');
const vm=require('node:vm');
const assert=require('node:assert/strict');
const {spawnSync}=require('node:child_process');
const os=require('node:os');
const {webcrypto,createHash}=require('node:crypto');
const {performance}=require('node:perf_hooks');

// context.source is available in the audited Node 22 runtime. Never convert a
// compiler number to Number for receipt comparison: int64 tokens can round.
class JSONNumberToken {
  constructor(token) { this.token = token; }
}
function parseExactJSON(raw) {
  return JSON.parse(raw, (_key, value, context) => {
    if (typeof value !== 'number') return value;
    assert.equal(typeof context?.source, 'string', 'Node must expose JSON number source tokens');
    return new JSONNumberToken(context.source);
  });
}

// The exclusions name only compiler receipt fields observed in this pin. No
// suffix-based filtering: a user's record field such as elapsed_ms is semantic.
const excludedPaths = new Map([
  ['$.report.route_decision_latency_ms', 'measured route timing'],
  ...['behavioral_probe_ms', 'final_emission_ms', 'ir_plan_build_ms', 'provider_decision_ms', 'total_ms']
    .map(key => ['$.report.body_fill.timing.' + key, 'measured IR fill timing']),
  ...['decision_latency_ms', 'total_ms']
    .map(key => ['$.report.body_search.' + key, 'measured IR search timing']),
  ...['decision_latency_ms', 'evaluation_ms']
    .map(key => ['$.report.body_search.attempts[*].' + key, 'measured IR search attempt timing']),
  ['$.report.record_assembly.generation_ns', 'measured record assembly timing'],
  ...['bounded_search_ms', 'final_emission_ms', 'plan_prepare_ms', 'source_binding_ms', 'total_ms']
    .map(key => ['$.report.body_paths.timing.' + key, 'measured typed path timing']),
  ['$.report.completeness_receipt.scope.execution_environment', 'asserted separately against manifest native environment versus js/wasm'],
  ['$.report.completeness_receipt.scope.record_assembly.observation_sha256', 'each raw timing-bearing receipt digest independently rebound with Go encoding/json'],
  ['$.report.completeness_receipt.scope.typed_path.observation_sha256', 'each raw timing-bearing receipt digest independently rebound with Go encoding/json'],
]);
function comparable(value, path = '$') {
  if (value instanceof JSONNumberToken) return value;
  if (Array.isArray(value)) return value.map(item => comparable(item, path + '[*]'));
  if (!value || typeof value !== 'object') return value;
  return Object.fromEntries(Object.entries(value)
    .filter(([key]) => !excludedPaths.has(path + '.' + key))
    .map(([key, child]) => [key, comparable(child, path + '.' + key)]));
}
function selfCheck() {
  const max = '9223372036854775807', adjacent = '9223372036854775806';
  assert.equal(JSON.parse(max), JSON.parse(adjacent), 'regression must demonstrate the former rounded comparison');
  assert.notDeepEqual(parseExactJSON(max), parseExactJSON(adjacent));
  assert.equal(parseExactJSON('-9223372036854775808').token, '-9223372036854775808');
  assert.equal(parseExactJSON('1.234567890123456789').token, '1.234567890123456789');
  const first = parseExactJSON('{"report":{"record_assembly":{"case_results":[{"actual":{"elapsed_ms":1,"value_ns":2}}]},"route_decision_latency_ms":1}}');
  const second = parseExactJSON('{"report":{"record_assembly":{"case_results":[{"actual":{"elapsed_ms":2,"value_ns":2}}]},"route_decision_latency_ms":2}}');
  assert.notDeepEqual(comparable(first), comparable(second), 'semantic timing-suffixed record values must not be excluded');
  for (const key of ['elapsed_ms', 'value_us', 'value_ns']) {
    const record = value => parseExactJSON('{"report":{"record_assembly":{"case_results":[{"actual":{' + JSON.stringify(key) + ':' + value + '}}]}}}');
    assert.notDeepEqual(comparable(record(1)), comparable(record(2)), key + ': semantic record value must remain compared');
  }
  assert.deepEqual(comparable(parseExactJSON('{"report":{"route_decision_latency_ms":1}}')), comparable(parseExactJSON('{"report":{"route_decision_latency_ms":2}}')));
  assert.notDeepEqual(comparable(parseExactJSON('{"report":{"not_a_timing_ms":1}}')), comparable(parseExactJSON('{"report":{"not_a_timing_ms":2}}')));
  assert.notDeepEqual(comparable(parseExactJSON('{"report":{"observation_sha256":"one"}}')), comparable(parseExactJSON('{"report":{"observation_sha256":"two"}}')));
  return {
    status: 'PASS', original_number_rounding_reproduced: true,
    adjacent_int64_tokens_distinguished: true, int64_minimum_preserved: true,
    long_decimal_token_preserved: true, semantic_ms_us_ns_record_fields_preserved: true,
    unknown_timing_suffix_field_preserved: true, unrelated_observation_hash_preserved: true,
    only_allowlisted_timing_paths_excluded: true,
  };
}

// Actual pinned compiler library in Wasm; generated Go is never executed.
const site=path.resolve(process.argv[2]||'out/browser/site/codegen');
const evidence=path.resolve(process.argv[3]||'out/browser/codegen-evidence');
const upstream=JSON.parse(fs.readFileSync(path.join(__dirname,'../upstream.json'),'utf8'));
const goBinary=process.env.GOOO_CODEGEN_GO||'go';
const compilerBinary=process.env.GOOO_CODEGEN_COMPILER||path.resolve(__dirname,'../.gooo');
const manifest=JSON.parse(fs.readFileSync(path.join(site,'build-manifest.json'),'utf8'));
const fixtureDocument=JSON.parse(fs.readFileSync(path.join(site,'fixtures.json'),'utf8'));
assert.equal(fixtureDocument.schema,'gooo/browser-codegen-probe-fixtures/v1');
assert.equal(fixtureDocument.compiler_pin,upstream.compiler_revision);
const fixtures=fixtureDocument.cases;
const hostRaw=fs.readFileSync(path.join(evidence,'host.stdout'),'utf8');
const host=hostRaw.trimEnd().split('\n').map(parseExactJSON);
assert.equal(fs.readFileSync(path.join(evidence,'host.stderr'),'utf8'),'');
const native=JSON.parse(fs.readFileSync(path.join(evidence,'native-cli-results.json'),'utf8')).map(row=>{
 const raw=fs.readFileSync(path.join(evidence,'cli-'+row.id+'.stdout'),'utf8').trim();
 return {...row,result:raw?parseExactJSON(raw):null,raw};
});
function hash(value){return createHash('sha256').update(value).digest('hex');}
const wasm=fs.readFileSync(path.join(site,'codegen.wasm'));
assert.equal(manifest.compiler_pin,upstream.compiler_revision);
assert.equal(manifest.compiler_tracked_source_edits,0);
assert.equal(manifest.core_checkout_still_clean,true);
assert.equal(manifest.vcs_embedding,false);
assert.equal(manifest.new_dependency_downloads_allowed,false);
assert.equal(manifest.go_version,upstream.go_version);
assert.equal(hash(wasm),manifest.wasm.sha256);
assert.equal(wasm.length,manifest.wasm.bytes);
const bridgeNames=['adapter.go','entry_host.go','entry_js.go'];
assert.deepEqual(Object.keys(manifest.bridge_files_sha256).sort(),bridgeNames);
const bridgeHashes=Object.fromEntries(bridgeNames.map(name=>{
 const digest=hash(fs.readFileSync(path.join(site,'bridge',name)));
 assert.equal(digest,manifest.bridge_files_sha256[name],name+': shipped bridge source binding');
 return [name,digest];
}));
assert.equal(hash(JSON.stringify(bridgeHashes)),manifest.bridge_source_sha256,'canonical bridge source set binding');
assert.equal(fixtures.length,16);
assert.equal(new Set(fixtures.map(row=>row.id)).size,16);
assert.equal(host.length,fixtures.length);
assert.deepEqual(native.map(row=>row.id),fixtures.filter(row=>row.native_comparison).map(row=>row.id));
const hostEnvironment=manifest.execution_environment;
assert.match(hostEnvironment,/^[a-z0-9]+\/[a-z0-9]+$/);
assert.notEqual(hostEnvironment,'js/wasm','Manifest must identify the actual native host toolchain');
const logs=[];
const sandbox={console:{log:(...v)=>logs.push(v.join(' ')),error:(...v)=>logs.push(v.join(' '))},WebAssembly,TextEncoder,TextDecoder,crypto:webcrypto,performance,setTimeout,clearTimeout};
const scope=vm.createContext(sandbox);
vm.runInContext(fs.readFileSync(path.join(site,'wasm_exec.js'),'utf8'),scope);
function largeIntegerTokens(value,path='$',found=[]){
 if(value instanceof JSONNumberToken){
  if(/^-?\d+$/.test(value.token)){
   const integer=BigInt(value.token);
   if(integer>9007199254740991n||integer< -9007199254740991n)found.push({path,token:value.token});
  }
  return found;
 }
 if(Array.isArray(value))value.forEach((child,i)=>largeIntegerTokens(child,path+'['+i+']',found));
 else if(value&&typeof value==='object')for(const [key,child] of Object.entries(value))largeIntegerTokens(child,path+'.'+key,found);
 return found;
}
(async()=>{
 const regressionChecks=selfCheck();
 fs.writeFileSync(path.join(evidence,'wasm-raw.ndjson'),'');
 const go=new scope.Go();
 go.argv=['gooo-browser-probe'];go.env={};
 const {instance}=await WebAssembly.instantiate(wasm,go.importObject);
 const running=go.run(instance);running.catch(error=>logs.push(String(error)));
 assert.equal(scope.goooBrowserReady,true);
 const metadata=JSON.parse(scope.goooBrowserMetadata());
 assert.equal(metadata.compiler_pin,manifest.compiler_pin);assert.equal(metadata.bridge_source_sha256,manifest.bridge_source_sha256);
 assert.equal(metadata.go_version,upstream.go_version);assert.equal(metadata.native_execution,false);assert.equal(metadata.model_calls,false);
 const rows=[],observed=[];
 const sourceKeys=['schema','decision','activity','activity_id','input_type','input_parameters','record_types','output_type','plan_sha256','source_digest','program_digest','generated_digest','replay_digest','route','candidate_routes','equivalence_rule','source_constructs','lowered_constructs','source_semantic_units','lowered_semantic_units','completeness_percent','route_equivalence','typecheck_passed','deterministic_replay','repository_writes','unsupported_constructs','body_fill','body_search','body_paths','record_assembly','call_closure'];
 for(let i=0;i<fixtures.length;i++){
  const f=fixtures[i];scope.requestJSON=JSON.stringify(f.request);
  assert.equal(hash(Buffer.from(f.request.source,'utf8')),f.origin_sha256,f.id+': fixture source binding');
  const raw=vm.runInContext('goooBrowserGenerate(requestJSON)',scope,{timeout:15000});
  const result=parseExactJSON(raw);observed.push(raw); fs.appendFileSync(path.join(evidence,'wasm-raw.ndjson'),raw+'\n');
  assert.equal(result.schema,'gooo/browser-body-codegen-response/v1');assert.equal(result.id,f.id);assert.equal(result.activity,f.request.activity);assert.equal(result.source_sha256,'sha256:'+hash(Buffer.from(f.request.source,'utf8')));
  assert.equal(result.compiler_pin,manifest.compiler_pin);assert.equal(result.bridge_source_sha256,manifest.bridge_source_sha256);
  assert.equal(result.native_execution,false);assert.equal(result.model_calls,false);
  assert.equal(host[i].id,f.id);
  assert.deepEqual(comparable(result),comparable(host[i]),f.id+': exact-number host/Wasm receipt');
  if(result.ok){
   assert.equal(result.report.completeness_receipt.scope.execution_environment,'js/wasm');
   assert.equal(host[i].report.completeness_receipt.scope.execution_environment,hostEnvironment);
  }
  let nativeMatch=null;
  const original=native.find(n=>n.id===f.id);
  if(original){
   assert.equal(result.ok,original.exit_code===0,f.id+': original CLI acceptance');
   if(result.ok){
    assert.equal(result.source,original.result.source,f.id+': original CLI generated bytes');
    assert.equal(result.gooo_source,original.result.gooo_source||'',f.id+': original selected Gooo');
    for(const key of sourceKeys)assert.deepEqual(comparable(result.report[key],'$.report.'+key),comparable(original.result.report[key],'$.report.'+key),f.id+': exact-number native structural report '+key);
    assert.equal(result.report.compiler_source_sha,'UNBOUND_LOCAL_SOURCE');
    assert.equal(original.result.report.compiler_source_sha,manifest.compiler_pin);
    assert.equal(original.result.report.completeness_receipt.scope.execution_environment,hostEnvironment);
   }else{
    assert.equal(original.exit_code,1);assert.equal(result.source,'');assert.equal(result.gooo_source,'');assert.equal(result.report,null);
    // --json legitimately returns a failure report on stdout. Preserve it and
    // assert its actual native failure contract rather than expecting silence.
    const failure=original.result;
    assert.equal(failure.schema,'gooo/body-codegen-report/v3');assert.equal(failure.decision,'FAIL_CLOSED');
    assert.equal(failure.source,'');assert.equal(failure.compiler_source_sha,manifest.compiler_pin);
    assert.equal(failure.repository_writes.token,'0');assert.ok(typeof failure.error==='string'&&failure.error.length>0);
    assert.equal(failure.completeness_receipt.scope.execution_environment,hostEnvironment);
    assert.equal(failure.completeness_receipt.scope.boundary.non_executing,true);
   }
   nativeMatch=true;
  } else {assert.equal(f.id,'invalid-import');assert.equal(result.ok,false);assert.equal(result.diagnostic.scope,'bridge_scope');}
  rows.push({id:f.id,ok:result.ok,host_wasm_structural_match:true,native_cli_match:nativeMatch,generated_go_sha256:result.ok?hash(result.source):null,scope:result.diagnostic?.scope||'compiler_body_codegen',unsafe_integer_tokens_compared_exactly:largeIntegerTokens(result)});
 }
 const int64Receipt=observed.find(raw=>parseExactJSON(raw).id==='ir-search');
 const changedInt64=int64Receipt.replace('"actual":9223372036854775807','"actual":9223372036854775806');
 assert.notEqual(changedInt64,int64Receipt,'fixture must exercise a raw int64 maximum result');
 assert.deepEqual(JSON.parse(changedInt64),JSON.parse(int64Receipt),'former rounded parser misses the actual one-unit mutation');
 assert.notDeepEqual(comparable(parseExactJSON(changedInt64)),comparable(parseExactJSON(int64Receipt)));
 regressionChecks.actual_int64_receipt_one_unit_mutation_rejected=true;
 const sample=fixtures.find(row=>row.id==='clamp').request;
 const wrapperRejections=[
  {id:'duplicate-key',code:'request_json',requestJSON:JSON.stringify(sample).replace('{','{"id":"duplicate",')},
  {id:'unknown-field',code:'unknown_field',requestJSON:JSON.stringify({...sample,provider:'outside-wrapper-profile'})},
  {id:'source-byte-bound',code:'source_size_or_utf8',requestJSON:JSON.stringify({...sample,source:' '.repeat(65537)})},
 ];
 for(const probe of wrapperRejections){
  scope.requestJSON=probe.requestJSON;
  const result=parseExactJSON(vm.runInContext('goooBrowserGenerate(requestJSON)',scope,{timeout:15000}));
  assert.equal(result.ok,false);assert.equal(result.source,'');assert.equal(result.gooo_source,'');assert.equal(result.report,null);
  assert.equal(result.diagnostic.scope,'bridge_request');assert.equal(result.diagnostic.code,probe.code);
  assert.equal(result.native_execution,false);assert.equal(result.model_calls,false);
 }
 const fresh={...sample,id:'fresh-edit',filename:'fresh-edit.gooo',source:sample.source.replace('return 0','return 42').replace('return input','return input + 3')};
 assert.ok(!fixtures.some(row=>row.request.source===fresh.source),'editable-source probe must differ from every saved sample');
 scope.requestJSON=JSON.stringify(fresh);
 const freshRaw=vm.runInContext('goooBrowserGenerate(requestJSON)',scope,{timeout:15000});
 const freshWasm=parseExactJSON(freshRaw);
 assert.equal(freshWasm.ok,true);assert.equal(freshWasm.id,fresh.id);assert.equal(freshWasm.activity,fresh.activity);
 assert.equal(freshWasm.source_sha256,'sha256:'+hash(Buffer.from(fresh.source,'utf8')));
 assert.equal(freshWasm.compiler_pin,manifest.compiler_pin);assert.equal(freshWasm.bridge_source_sha256,manifest.bridge_source_sha256);
 const freshDir=fs.mkdtempSync(path.join(os.tmpdir(),'gooo-codegen-edit-'));
 let freshNative;
 try{
  fs.writeFileSync(path.join(freshDir,fresh.filename),fresh.source);
  freshNative=spawnSync(compilerBinary,['body-codegen','--json','--activity',fresh.activity,fresh.filename],{cwd:freshDir,encoding:'utf8',maxBuffer:4*1024*1024,timeout:15000,env:{...process.env,GOOO_LAYA_URL:'',GOOO_LAYA_API_KEY:''}});
 }finally{fs.rmSync(freshDir,{recursive:true,force:true});}
 assert.equal(freshNative.status,0,freshNative.stderr||String(freshNative.error||'fresh native CLI failed'));
 assert.equal(freshNative.stderr,'');
 const freshOriginal=parseExactJSON(freshNative.stdout);
 assert.equal(freshOriginal.report.compiler_source_sha,manifest.compiler_pin);
 assert.equal(freshWasm.source,freshOriginal.source,'fresh edited source: actual CLI/Wasm generated bytes');
 assert.equal(freshWasm.gooo_source,freshOriginal.gooo_source||'');
 for(const key of sourceKeys)assert.deepEqual(comparable(freshWasm.report[key],'$.report.'+key),comparable(freshOriginal.report[key],'$.report.'+key),'fresh edited source: native structural report '+key);
 const freshProof={id:fresh.id,source_sha256:freshWasm.source_sha256,generated_go_sha256:hash(freshWasm.source),differs_from_all_saved_sources:true,actual_original_native_cli_match:true,generated_go_executed:false};
 fs.writeFileSync(path.join(evidence,'fresh-edit-request.json'),JSON.stringify(fresh,null,2)+'\n');
 fs.writeFileSync(path.join(evidence,'fresh-edit-native.stdout'),freshNative.stdout);
 fs.writeFileSync(path.join(evidence,'fresh-edit-native.stderr'),freshNative.stderr);
 fs.writeFileSync(path.join(evidence,'fresh-edit-wasm.stdout'),freshRaw+'\n');
 // Store original JSON, not a parse/stringify round-trip that changes int64s.
 fs.writeFileSync(path.join(evidence,'wasm-results.json'),'[\n'+observed.join(',\n')+'\n]\n');
 fs.writeFileSync(path.join(evidence,'wasm-console.json'),JSON.stringify(logs,null,2)+'\n');
 const nativeRaw=native.filter(row=>row.exit_code===0).map(row=>row.raw.replace(/^\{/,'{"id":'+JSON.stringify(row.id)+',')).join('\n')+'\n';
 const digestInput=hostRaw+observed.join('\n')+'\n'+nativeRaw;
 // The checked-in .go.txt helper is copied to an owned temporary file so it
 // does not become part of the observer's Go package. It uses only stdlib.
 const digestDir=fs.mkdtempSync(path.join(os.tmpdir(),'gooo-codegen-digests-'));
 let digestCheck;
 try{
  const helper=path.join(digestDir,'receipt-digests.go');
  fs.copyFileSync(path.join(__dirname,'browser-codegen-receipt-digests.go.txt'),helper);
  digestCheck=spawnSync(goBinary,['run',helper],{input:digestInput,encoding:'utf8',maxBuffer:4*1024*1024,env:{...process.env,GOTOOLCHAIN:'local',GOWORK:'off',GOPROXY:'off',GOFLAGS:'',CGO_ENABLED:'0'}});
 }finally{fs.rmSync(digestDir,{recursive:true,force:true});}
 assert.equal(digestCheck.status,0,digestCheck.stderr||'independent raw receipt hash check failed');
 const digestProof=JSON.parse(digestCheck.stdout);assert.equal(digestProof.count,6);
 fs.writeFileSync(path.join(evidence,'receipt-digest-bindings.json'),JSON.stringify({...digestProof,sources:['host original NDJSON','Wasm original NDJSON','successful native CLI original JSON'],checks_per_source:2},null,2)+'\n');
 const summary={schema:'gooo/browser-codegen-smoke/v1',status:'PASS',node_version:process.version,actual_wasm_bytes_verified:true,metadata,cases:rows,fresh_edit:freshProof,wrapper_rejections:wrapperRejections.map(({id,code})=>({id,code,scope:'bridge_request',rejected_without_generated_output:true})),receipt_comparison:'Original raw host/Wasm JSON is compared with exact numeric tokens, including int64 extremes, except explicitly allowlisted compiler timing/environment/digest paths. Native CLI original generated source/selected Gooo and listed structural fields match with exact numeric tokens. Engine compiler provenance is asserted separately. Every omitted timing-bearing observation hash is independently rebound against its own original raw receipt with Go encoding/json.',native_structural_report_keys:sourceKeys,excluded_paths:Object.fromEntries(excludedPaths),numeric_regression_checks:regressionChecks,independent_receipt_hash_bindings:digestProof.count,generated_go_executed:false,model_calls_performed:false,filesystem_provider:'Stock Go wasm_exec.js filesystem stubs; no Node filesystem/network supplied to VM.'};
 fs.writeFileSync(path.join(evidence,'browser-codegen-smoke.json'),JSON.stringify(summary,null,2)+'\n');
 console.log(JSON.stringify(summary));process.exit(0);
})().catch(error=>{console.error(error.stack);process.exit(1);});
