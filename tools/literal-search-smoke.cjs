'use strict';

// Run the actual shipped Go Wasm. No JavaScript Index/Count implementation.
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const {createHash, webcrypto} = require('node:crypto');
const {Worker} = require('node:worker_threads');
const fields = ['text','needle','first_byte_offset','found','non_overlapping_count','text_byte_length','needle_byte_length'];
const ids = ['text','needle','first-byte-offset','found','non-overlapping-count','text-byte-length','needle-byte-length'];
const entry = 'gooo://go-ports/examples/literal-text-report/report';
const kind = ['string','string','integer','boolean','integer','integer','integer'];
const keys = (v, expected) => {assert.ok(v && typeof v==='object' && !Array.isArray(v));assert.deepEqual(Object.keys(v).sort(),[...expected].sort());};
const hashBytes = raw => createHash('sha256').update(raw).digest('hex');
const unitsText = units => {assert.ok(Array.isArray(units));for(const u of units)assert.ok(Number.isInteger(u)&&u>=0&&u<=65535);return String.fromCharCode(...units);};
const encodedString = text => JSON.stringify(text).replace(/[<>&\u2028\u2029]/g,c=>'\\u'+c.charCodeAt(0).toString(16).padStart(4,'0'));
function nativeOutput(actual) {
 keys(actual,fields);assert.equal(typeof actual.found,'boolean');assert.equal(typeof actual.text,'string');assert.equal(typeof actual.needle,'string');
 const out={...actual};for(const name of fields.filter((_,i)=>kind[i]==='integer')){assert.ok(Number.isSafeInteger(actual[name]));out[name]=String(actual[name]);}return out;
}
async function actualWorkerSmoke(site, fixture, manifest, protocol) {
 // A real thread executes the production loader, request handler, protocol and
 // Go Wasm. Only static-file transport replaces browser fetch/importScripts;
 // the compiled callback and all returned values are the actual shipped code.
 const worker=new Worker(`
  const {parentPort,workerData}=require('node:worker_threads');
  const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
  globalThis.crypto??=require('node:crypto').webcrypto;
  globalThis.fetch=async(url,options)=>{
   assert.equal(options.cache,'no-store');assert.ok(['./literal-search/manifest.json','./literal-search/program.wasm'].includes(url));
   const raw=fs.readFileSync(path.join(workerData.site,url));
   return{ok:true,json:async()=>JSON.parse(raw.toString('utf8')),arrayBuffer:async()=>raw.buffer.slice(raw.byteOffset,raw.byteOffset+raw.byteLength)};
  };
  globalThis.importScripts=name=>{assert.equal(name,'./wasm_exec.js');require(path.join(workerData.site,name));};
  globalThis.postMessage=data=>parentPort.postMessage(data);
  globalThis.addEventListener=(event,handler)=>{assert.equal(event,'message');parentPort.on('message',data=>handler({data}));};
  require(path.join(workerData.site,'literal-search-worker.js')).startWorker(globalThis);
 `,{eval:true,workerData:{site}});
 const messages=[];let waiting,failure;
 worker.on('message',message=>{if(waiting){const resolve=waiting;waiting=null;resolve(message);}else messages.push(message);});
 worker.on('error',error=>{failure=error;if(waiting){const resolve=waiting;waiting=null;resolve({workerFailure:error.message});}});
 const next=()=>new Promise((resolve,reject)=>{
  if(failure)return reject(failure);if(messages.length)return resolve(messages.shift());
  const timer=setTimeout(()=>{waiting=null;reject(new Error('Actual literal Worker timed out.'));},10000);
  waiting=message=>{clearTimeout(timer);if(message.workerFailure)reject(new Error(message.workerFailure));else resolve(message);};
 });
 try {
  const ready=await next();const program=protocol.validateReady(ready);
  assert.deepEqual(JSON.parse(JSON.stringify(program)),manifest);
  let id=0;
  for(const row of fixture.cases){const request={type:'search',id:++id,inputs:row.inputs};worker.postMessage(request);const message=await next();
   const result=protocol.validateResult(message,{...request,program});assert.deepEqual(result,row.expected,row.name);
  }
  for(const message of [
   {type:'search',id:1,inputs:['','']},
   {type:'search',id:++id,inputs:[0,'']},
   {type:'search',id:++id,inputs:['a'.repeat(1025),'']},
   {type:'search',id:++id,inputs:['\uD800','']},
   {type:'search',id:++id,inputs:['',''],unexpected:true},
   {type:'unknown',id:++id,inputs:['','']},
  ]) {worker.postMessage(message);const reply=await next();protocol.validateError(reply,message.id);}
  return{cases:fixture.cases.length,request_rejections:6};
 } finally {await worker.terminate();}
}
async function runSmoke(directory) {
 const site=path.resolve(directory),dir=path.join(site,'literal-search');
 const bytes=name=>fs.readFileSync(path.join(dir,name));
 const read=name=>JSON.parse(bytes(name).toString('utf8'));
 const hash=name=>hashBytes(bytes(name));
 const protocol=require(path.join(site,'literal-search-protocol.js'));
 const manifest=read('manifest.json'),lock=read('upstream.json');protocol.validateMetadata(manifest);
 for(const key of ['compiler_revision','compiler_version','go_version'])assert.equal(manifest[key],lock[key]);
 for(const [name,key] of [['program.wasm','wasm_sha256'],['generated.go','generated_sha256'],['execution.json','execution_sha256'],['replay.json','replay_sha256'],['fixtures.json','fixtures_sha256']])assert.equal(hash(name),manifest[key],name);
 assert.equal(bytes('program.wasm').length,manifest.wasm_bytes);
 for(const [name,digest] of Object.entries(manifest.files_sha256))assert.equal(hash(name),digest,name);
 const sourceFiles=['ports/strings/search.gooo','ports/strings/literal.gooo','examples/literal-text-report/main.gooo'];keys(manifest.source_sha256,sourceFiles);
 const sourceBindings={};
 for(const name of sourceFiles){assert.equal(hash('bound-source/'+name),manifest.source_sha256[name]);sourceBindings[name]='sha256:'+hashBytes(Buffer.from(encodedString(bytes('bound-source/'+name).toString('utf8'))));}
 const compiler=read('compiler.json');assert.equal(compiler.compiler_source_sha,manifest.compiler_revision);assert.equal(compiler.version,manifest.compiler_version);assert.equal(compiler.source_status,'CLEAN_VCS');assert.equal(compiler.vcs_modified,'false');assert.equal(compiler.go_version,manifest.go_version);
 const fixture=read('fixtures.json');assert.equal(fixture.schema,'gooo/literal-search-browser-fixtures/v1');assert.equal(fixture.compiler_revision,manifest.compiler_revision);assert.equal(fixture.go_version,manifest.go_version);
 assert.equal(fixture.cases.length,48);assert.equal(fixture.rejections.length,6);assert.equal(fixture.cases.length,manifest.fixture_cases);assert.equal(fixture.rejections.length,manifest.fixture_rejections);assert.equal(manifest.native_cases,48);assert.equal(manifest.native_receipt_expectation_cases,0);
 const workspace=read('workspace.json');assert.deepEqual(workspace.entry,{package_path:'app/literal-text-report',activity:'Report'});
 let offset=0;
 for(const stem of ['', 'boundary-']) {
  const inputs=read(stem+'inputs.json');keys(inputs,['schema','inputs']);assert.equal(inputs.schema,'gooo/body-composition-inputs/v1');
  const rows=fixture.cases.slice(offset,offset+inputs.inputs.length);assert.equal(rows.length,24);
  for(const suffix of ['execution.json','replay.json']) {
   const record=read(stem+suffix);assert.equal(record.schema,'gooo/workspace-body-execution-receipt/v1');assert.equal(record.decision,'OBSERVED');assert.equal(record.error,undefined);assert.equal(record.cases_digest,undefined);
   assert.equal(record.manifest_digest,'sha256:'+hash('workspace.json'));assert.equal(record.inputs_digest,'sha256:'+hash(stem+'inputs.json'));
   const {program,composition,runtime}=record.result;assert.equal(program.entry.package_path,'app/literal-text-report');assert.equal(program.entry.activity,'Report');
   assert.equal(composition.source,bytes('generated.go').toString('utf8'));assert.equal(composition.generated_sha256,'sha256:'+manifest.generated_sha256);
   assert.equal(program.lowered_gooo_source,composition.gooo_source);const bound={};
   for(const pkg of program.workspace_image.packages)for(const source of pkg.sources){assert.ok(!Object.hasOwn(bound,source.filename));bound[source.filename]=source.source_digest;}assert.deepEqual(bound,sourceBindings);
   assert.equal(runtime.stage,'COMPLETE');assert.equal(runtime.failure,undefined);assert.equal(runtime.producer_source_sha,manifest.compiler_revision);assert.equal(runtime.go_version.split(' ')[2],manifest.go_version);assert.equal(runtime.generated_sha256,'sha256:'+manifest.generated_sha256);
   for(const key of ['model_calls','finite_total','finite_passed'])assert.equal(runtime[key],0);assert.equal(runtime.projection_replayed,true);assert.equal(runtime.runtime_replayed,true);assert.equal(runtime.traces.length,rows.length);
   for(let i=0;i<rows.length;i++){
    const row=rows[i],supplied=inputs.inputs[i],trace=runtime.traces[i];keys(supplied,['app/literal-text-report:Report.input0','app/literal-text-report:Report.input1']);
    assert.deepEqual([supplied['app/literal-text-report:Report.input0'],supplied['app/literal-text-report:Report.input1']],row.inputs);
    assert.deepEqual(protocol.bindInputs([unitsText(row.text_utf16),unitsText(row.needle_utf16)]),row.inputs);
    assert.equal(trace.case_index,i);assert.equal(trace.deliveries.length,1);const d=trace.deliveries[0];keys(d,['activity_id','inputs','actual','actual_fields']);assert.equal(d.activity_id,entry);
    assert.deepEqual(d.inputs,row.inputs.map((value,index)=>({port:'input'+index,entity_id:'gooo://std/text',value})));assert.deepEqual(nativeOutput(d.actual),row.expected);
    assert.deepEqual(d.actual_fields,fields.map((name,index)=>({id:'gooo://go-ports/examples/literal-text-report/result/'+ids[index],name,value:d.actual[name]})));
   }
   if(suffix==='replay.json'){assert.equal(record.replayed_from_sha256,'sha256:'+hash(stem+'execution.json'));assert.equal(record.result.replay.model_calls,0);}else{assert.equal(record.replayed_from_sha256,undefined);assert.equal(record.result.replay,undefined);}
  }
  const checked=read(stem+'checked-report.json');assert.equal(checked.schema,'gooo/literal-text-report-checked/v1');assert.equal(checked.decision,'PASS_FINITE_GO_ORACLE_AND_SAVED_REPLAY');
  assert.equal(checked.execution_sha256,'sha256:'+hash(stem+'execution.json'));assert.equal(checked.replay_sha256,'sha256:'+hash(stem+'replay.json'));assert.equal(checked.generated_sha256,'sha256:'+manifest.generated_sha256);
  for(const key of ['native_observed_cases','independent_go_oracle_cases','saved_replay_observed_cases'])assert.equal(checked[key],rows.length);assert.equal(checked.native_receipt_expectation_cases,0);
  assert.match(checked.prior_result_sha256,/^sha256:[a-f0-9]{64}$/);offset+=rows.length;
 }
 assert.equal(offset,fixture.cases.length);
 globalThis.crypto??=webcrypto;require(path.join(site,'wasm_exec.js'));const go=new Go();
 const {instance}=await WebAssembly.instantiate(bytes('program.wasm'),go.importObject);let failed;
 go.run(instance).then(()=>{failed=new Error('Literal runtime stopped.');},error=>{failed=error;});assert.equal(globalThis.goooLiteralSearchReady,true);
 assert.equal(typeof globalThis.goooLiteralSearch,'function');const seen=new Set();let observations=0;
 const invoke=args=>{assert.equal(failed,undefined);const raw=globalThis.goooLiteralSearch(...args);assert.equal(typeof raw,'string');return raw;};
 for(const row of fixture.cases){assert.ok(!seen.has(row.name));seen.add(row.name);const original=[unitsText(row.text_utf16),unitsText(row.needle_utf16)];const raw=invoke(original);const reply=protocol.parseBridgeReply(raw,original);
  assert.deepEqual(reply.inputs,row.inputs);assert.deepEqual(reply.output,row.expected,row.name);
  const report=protocol.observation(original.map(text=>protocol.read(text,'text')),reply.output,manifest,['smoke text','smoke phrase']);
  assert.equal(report.kind,'browser-observation');assert.equal(report.schema,'gooo/literal-search-observation/v1');assert.deepEqual(report.output,row.expected);
  for(let i=0;i<2;i++){assert.equal(report.inputs[i].original,original[i]);assert.equal(report.inputs[i].bound,row.inputs[i]);assert.deepEqual(report.inputs[i].original_utf16_units,i?row.needle_utf16:row.text_utf16);}
  const roundTrip=JSON.parse(JSON.stringify(report));assert.deepEqual(roundTrip.output,row.expected);observations++;
 }
 const reject=args=>{const r=JSON.parse(invoke(args));keys(r,['ok','error']);assert.equal(r.ok,false);assert.ok(typeof r.error==='string'&&r.error.length);};
 for(const row of fixture.rejections)reject([unitsText(row.text_utf16),unitsText(row.needle_utf16)]);
 const wrong=[0,NaN,Infinity,true,null,undefined,{},[],new String('x'),0n,Symbol('x'),()=>0];let typeRejections=0;
 for(let slot=0;slot<2;slot++)for(const value of wrong){const args=['가x가','x'];args[slot]=value;reject(args);typeRejections++;}
 for(const args of [[],['x'],['x','x','x']])reject(args);
 const anchors=[{text:'가x가',needle:'x',first_byte_offset:'3',non_overlapping_count:'1'},{text:'aaaaa',needle:'aa',first_byte_offset:'0',non_overlapping_count:'2'},{text:'ée\u0301',needle:'\u0301',first_byte_offset:'3',non_overlapping_count:'1'},{text:'e\u0301',needle:'é',first_byte_offset:'-1',non_overlapping_count:'0'},{text:'가x가',needle:'',first_byte_offset:'0',non_overlapping_count:'4'}];
 for(const a of anchors){const r=JSON.parse(invoke([a.text,a.needle]));for(const [key,v]of Object.entries(a))assert.equal(r.output[key],v,key);}
 for(const name of ['literal-search.html','literal-search.css','literal-search.js','literal-search-protocol.js','literal-search-worker.js'])assert.ok(fs.statSync(path.join(site,name)).isFile(),name);
 const workerObservation=await actualWorkerSmoke(site,fixture,manifest,protocol);
 await Promise.resolve();assert.equal(failed,undefined);
 return{schema:'gooo/literal-search-browser-smoke/v1',status:'PASS',native_observed_cases:48,strong_checked_native_batches:2,native_receipt_expectation_cases:0,wasm_cases:48,actual_worker_cases:workerObservation.cases,actual_worker_request_rejections:workerObservation.request_rejections,browser_observation_exports:observations,domain_rejections:6,type_rejections:typeRejections,arity_rejections:3,manual_anchors:anchors.length,wasm_sha256:manifest.wasm_sha256,generated_sha256:manifest.generated_sha256,source_revision:manifest.source_revision,source_dirty:manifest.source_dirty,scope:'Actual Go Wasm callbacks and production Worker loader/protocol in Node worker_threads with static-file transport, independent Go fixtures, strong checked native receipts and saved replay. DOM input capture is covered separately by unit tests; this is not a full browser-DOM test, public adoption or a signed attestation.'};
}
if(require.main===module)runSmoke(process.argv[2]||'out/browser/site').then(result=>{console.log(JSON.stringify(result));process.exit(0);},error=>{console.error(error);process.exit(1);});
module.exports={runSmoke};
