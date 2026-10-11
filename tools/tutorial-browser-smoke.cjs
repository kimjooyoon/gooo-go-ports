// Native tutorial receipts and the actual shipped Wasm, with exact integers.
// No clamp, label selection or imported Gooo helper is implemented here.
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { createHash, webcrypto } = require('node:crypto');
const entry = 'urn:learn-gooo:describe';
const names = ['score', 'remaining', 'in_range', 'label'];
const ids = ['urn:learn-gooo:score', 'urn:learn-gooo:remaining', 'urn:learn-gooo:in-range-field', 'urn:learn-gooo:label'];
function keys(value, expected) { assert.ok(value && typeof value === 'object' && !Array.isArray(value)); assert.deepEqual(Object.keys(value).sort(), [...expected].sort()); }
function integer(value) { assert.equal(typeof value, 'string'); assert.match(value, /^(?:0|[1-9][0-9]*|-[1-9][0-9]*)$/); assert.ok(value.length <= 20); assert.ok(BigInt(value) >= -9223372036854775808n && BigInt(value) <= 9223372036854775807n); }
function output(value) { keys(value, names); integer(value.score); integer(value.remaining); assert.equal(typeof value.in_range, 'boolean'); assert.equal(typeof value.label, 'string'); assert.ok(Buffer.byteLength(value.label, 'utf8') <= 1024); }
function exactJSON(raw) {
 return JSON.parse(raw, (_, value, context) => {
  if (typeof value !== 'number') return value;
  assert.ok(context && typeof context.source === 'string', 'Node with JSON.parse source context required');
  assert.match(context.source, /^-?(?:0|[1-9][0-9]*)$/);
  return context.source;
 });
}
function labelFromUnits(units) { assert.ok(Array.isArray(units)); for (const n of units) assert.ok(Number.isInteger(n) && n >= 0 && n <= 65535); return String.fromCharCode(...units); }
function boundLabel(raw) { return new TextDecoder('utf-8', { ignoreBOM: true }).decode(new TextEncoder().encode(raw)); }
async function runSmoke(directory) {
 const site = path.resolve(directory), dir = path.join(site, 'tutorial');
 const bytes = name => fs.readFileSync(path.join(dir, name));
 const read = name => JSON.parse(bytes(name).toString('utf8'));
 const exact = name => exactJSON(bytes(name).toString('utf8'));
 const hash = name => createHash('sha256').update(bytes(name)).digest('hex');
 const manifest = read('manifest.json'), lock = read('upstream.json');
 assert.equal(manifest.schema, 'gooo/browser-tutorial/v1'); assert.equal(manifest.entry_id, entry);
 assert.equal(manifest.result_entity_id, 'urn:learn-gooo:summary');
 assert.deepEqual(manifest.input_entity_ids, ['urn:gooo:type:integer', 'urn:gooo:type:string']);
 for (const key of ['compiler_revision', 'compiler_version', 'go_version']) assert.equal(manifest[key], lock[key]);
 assert.match(manifest.source_revision, /^[a-f0-9]{40}$/); assert.equal(typeof manifest.source_dirty, 'boolean');
 for (const [name, key] of [['program.wasm','wasm_sha256'], ['generated.go','generated_sha256'], ['execution.json','execution_sha256'], ['replay.json','replay_sha256'], ['fixtures.json','fixtures_sha256'], ['source.zip','source_zip_sha256']]) assert.equal(hash(name), manifest[key], name);
 assert.equal(bytes('program.wasm').length, manifest.wasm_bytes); assert.equal(bytes('source.zip').length, manifest.source_zip_bytes);
 for (const [name, digest] of Object.entries(manifest.files_sha256)) assert.equal(hash(name), digest, name);
 keys(manifest.sample_sha256, ['limits.gooo','main.gooo','inputs.json','gooo.workspace.json','NOTICE','README.md']);
 for (const [name, digest] of Object.entries(manifest.sample_sha256)) assert.equal(hash(`source/${name}`), digest, name);
 assert.equal(hash('source/inputs.json'), hash('inputs.json'));
 keys(manifest.source_sha256, ['examples/learn-gooo/limits.gooo','examples/learn-gooo/main.gooo']);
 const sourceBindings = {};
 for (const [name, digest] of Object.entries(manifest.source_sha256)) {
  assert.equal(hash(`bound-source/${name}`), digest, name); assert.equal(hash(`source/${path.basename(name)}`), digest, name);
  const encoded = JSON.stringify(bytes(`bound-source/${name}`).toString('utf8')).replace(/[<>&\u2028\u2029]/g, c => `\\u${c.charCodeAt(0).toString(16).padStart(4,'0')}`);
  sourceBindings[name] = `sha256:${createHash('sha256').update(encoded).digest('hex')}`;
 }
 const fixture = read('fixtures.json'); assert.equal(fixture.schema, 'gooo/tutorial-browser-fixtures/v1');
 for (const key of ['compiler_revision','go_version']) assert.equal(fixture[key], manifest[key]);
 assert.equal(fixture.cases.length, manifest.fixture_cases); assert.equal(fixture.rejections.length, manifest.fixture_rejections);
 assert.equal(fixture.cases.length, manifest.native_cases);
 const manual = [
  {score:'0',remaining:'100',in_range:false,label:'sensor: low'},
  {score:'42',remaining:'58',in_range:true,label:'sensor: low'},
  {score:'75',remaining:'25',in_range:true,label:'sensor: high'},
  {score:'100',remaining:'0',in_range:false,label:'sensor: high'}
 ];
 assert.deepEqual(fixture.cases.slice(0,4).map(row => row.expected), manual);
 let offset = 0;
 for (const stem of ['', 'boundary-']) {
  const inputDoc = exact(`${stem}inputs.json`); assert.equal(inputDoc.schema,'gooo/body-composition-inputs/v1');
  const rows = fixture.cases.slice(offset, offset + inputDoc.inputs.length);
  for (const suffix of ['execution.json','replay.json']) {
   const record = exact(`${stem}${suffix}`); assert.equal(record.schema,'gooo/workspace-body-execution-receipt/v1'); assert.equal(record.decision,'OBSERVED'); assert.equal(record.error,undefined); assert.equal(record.cases_digest,undefined);
   assert.equal(record.manifest_digest,`sha256:${hash('workspace.json')}`); assert.equal(record.inputs_digest,`sha256:${hash(`${stem}inputs.json`)}`);
   const {program,composition,runtime} = record.result;
   assert.equal(program.entry.package_path,'app/report'); assert.equal(program.entry.activity,'Describe');
   assert.equal(composition.source,bytes('generated.go').toString('utf8')); assert.equal(composition.generated_sha256,`sha256:${manifest.generated_sha256}`);
   assert.equal(program.lowered_gooo_source,composition.gooo_source);
   const bound = {};
   for (const pkg of program.workspace_image.packages) for (const source of pkg.sources) {assert.ok(!Object.hasOwn(bound,source.filename));bound[source.filename]=source.source_digest;}
   assert.deepEqual(bound,sourceBindings);
   assert.equal(runtime.stage,'COMPLETE');assert.equal(runtime.failure,undefined);assert.equal(runtime.producer_source_sha,manifest.compiler_revision);assert.equal(runtime.go_version.split(' ')[2],manifest.go_version);assert.equal(runtime.generated_sha256,`sha256:${manifest.generated_sha256}`);
   for (const key of ['model_calls','finite_total','finite_passed']) assert.equal(runtime[key],'0');
   assert.equal(runtime.projection_replayed,true);assert.equal(runtime.runtime_replayed,true);assert.equal(runtime.traces.length,rows.length);
   for (let i=0;i<rows.length;i++) {
    const row=rows[i], supplied=inputDoc.inputs[i], trace=runtime.traces[i];integer(row.inputs[0]);output(row.expected);
    assert.equal(supplied['app/report:Describe.input1'],labelFromUnits(row.label_utf16));
    assert.deepEqual([supplied['app/report:Describe.input0'],boundLabel(supplied['app/report:Describe.input1'])],row.inputs);
    assert.equal(trace.case_index,String(i));assert.equal(trace.deliveries.length,1);const d=trace.deliveries[0];keys(d,['activity_id','inputs','actual','actual_fields']);assert.equal(d.activity_id,entry);
    assert.deepEqual(d.inputs,[{port:'input0',entity_id:'urn:gooo:type:integer',value:row.inputs[0]},{port:'input1',entity_id:'urn:gooo:type:string',value:row.inputs[1]}]);
    assert.deepEqual(d.actual,row.expected);assert.deepEqual(d.actual_fields,names.map((name,index)=>({id:ids[index],name,value:row.expected[name]})));
   }
   if(suffix==='replay.json'){assert.equal(record.replayed_from_sha256,`sha256:${hash(`${stem}execution.json`)}`);assert.equal(record.result.replay.model_calls,'0');}else {assert.equal(record.replayed_from_sha256,undefined);assert.equal(record.result.replay,undefined);}
  }
  offset+=rows.length;
 }
 assert.equal(offset,fixture.cases.length);
 let nativeRejections=0;
 for(const row of fixture.rejections.filter(r=>r.native)){
  const stem=`rejected-${String(nativeRejections).padStart(2,'0')}`;const record=read(`${stem}-execution.json`);
  assert.equal(record.decision,'FAIL_CLOSED');assert.equal(record.manifest_digest,`sha256:${hash('workspace.json')}`);assert.equal(record.inputs_digest,`sha256:${hash(`${stem}-inputs.json`)}`);assert.match(record.error,/Text requires a string of at most 1024 UTF-8 bytes/);nativeRejections++;
 }
 assert.equal(nativeRejections,manifest.native_rejections);
 globalThis.crypto ??= webcrypto;require(path.join(site,'wasm_exec.js'));const go = new Go();
 const {instance}=await WebAssembly.instantiate(bytes('program.wasm'),go.importObject);let runtimeFailure;
 go.run(instance).then(()=>{runtimeFailure=new Error('Tutorial runtime stopped.');},e=>{runtimeFailure=e;});
 assert.equal(globalThis.goooTutorialReady,true);assert.equal(typeof globalThis.goooTutorial,'function');
 const invoke=args=>{assert.equal(runtimeFailure,undefined);const raw=globalThis.goooTutorial(...args);assert.equal(typeof raw,'string');return JSON.parse(raw);};
 const reject=args=>{const r=invoke(args);keys(r,['ok','error']);assert.equal(r.ok,false);assert.ok(typeof r.error==='string'&&r.error.length>0);};
 const seen=new Set();
 for(const row of fixture.cases){assert.ok(!seen.has(row.name));seen.add(row.name);const rawLabel=labelFromUnits(row.label_utf16);assert.equal(boundLabel(rawLabel),row.inputs[1]);const actual=invoke([row.inputs[0],rawLabel]);keys(actual,['ok','inputs','output']);assert.equal(actual.ok,true,row.name);assert.deepEqual(actual.inputs,row.inputs,row.name);output(actual.output);assert.deepEqual(actual.output,row.expected,row.name);}
 for(const row of fixture.rejections)reject([row.inputs[0],labelFromUnits(row.label_utf16)]);
 const wrongTypes=[0,9007199254740992,NaN,Infinity,true,null,undefined,{},[],new String('x'),0n,Symbol('x'),()=>0];let typeRejections=0;
 for(let slot=0;slot<2;slot++)for(const value of wrongTypes){const args=['42','sensor'];args[slot]=value;reject(args);typeRejections++;}
 for(const args of [[],['42'],['42','sensor','extra']])reject(args);
 assert.deepEqual(invoke(['42','sensor']),{ok:true,inputs:['42','sensor'],output:manual[1]});await Promise.resolve();assert.equal(runtimeFailure,undefined);
 return {schema:'gooo/tutorial-browser-smoke/v1',status:'PASS',cases:fixture.cases.length,manual_examples:4,native_cases:manifest.native_cases,native_rejections:nativeRejections,fixture_rejections:fixture.rejections.length,type_rejections:typeRejections,arity_rejections:3,wasm_sha256:manifest.wasm_sha256,compiler_revision:manifest.compiler_revision,source_revision:manifest.source_revision,scope:'Actual shipped Wasm matches saved native observations and four manual examples; int64 exactness, bound Text, full record fields, source/compiler/input/replay identities checked. No DOM test or all-input claim.'};
}
module.exports={runSmoke};
if(require.main===module)runSmoke(process.argv[2]||'out/browser/site').then(r=>{console.log(JSON.stringify(r));process.exit(0);}).catch(e=>{console.error(e);process.exit(1);});
