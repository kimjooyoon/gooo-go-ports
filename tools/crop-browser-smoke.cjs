// Exercise the shipped crop Wasm and exact string protocol against Go fixtures.
// Geometry stays in compiled Gooo; this script only checks evidence and values.
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { webcrypto, createHash } = require('node:crypto');

const coordinates = ['min_x', 'min_y', 'max_x', 'max_y'];
const decimal = /^(?:0|[1-9][0-9]*|-[1-9][0-9]*)$/;

function exactInteger(value, context) {
  assert.equal(typeof value, 'string', context);
  assert.ok(value.length <= 20 && decimal.test(value), context);
  const integer = BigInt(value);
  assert.ok(integer >= -9223372036854775808n && integer <= 9223372036854775807n, context);
}

function objectKeys(value, keys, context) {
  assert.ok(value && typeof value === 'object' && !Array.isArray(value), context);
  assert.deepEqual(Object.keys(value).sort(), [...keys].sort(), context);
}

function cropOutput(value, context) {
  objectKeys(value, [...coordinates, 'has_pixels'], context);
  for (const key of coordinates) exactInteger(value[key], `${context}.${key}`);
  assert.equal(typeof value.has_pixels, 'boolean', context);
}

async function runSmoke(directory) {
  const site = path.resolve(directory);
  const dir = path.join(site, 'crop');
  const bytes = name => fs.readFileSync(path.join(dir, name));
  const read = name => JSON.parse(bytes(name).toString('utf8'));
  const hash = name => createHash('sha256').update(bytes(name)).digest('hex');
  const metadata = read('metadata.json');
  assert.equal(metadata.schema, 'gooo/browser-crop-planner/v1');
  assert.match(metadata.source_revision, /^[a-f0-9]{40}$/);
  assert.equal(typeof metadata.source_dirty, 'boolean');
  const lock = read('upstream.json');
  assert.equal(metadata.compiler_revision, lock.compiler_revision);
  assert.equal(metadata.go_version, lock.go_version);
  for (const [name, key] of [
    ['crop.wasm', 'wasm_sha256'], ['generated.go', 'generated_sha256'],
    ['execution.json', 'execution_sha256'], ['replay.json', 'replay_sha256'],
    ['fixtures.json', 'fixtures_sha256'], ['crop-planner.workspace.json', 'manifest_sha256'],
    ['inputs.json', 'inputs_sha256']
  ]) {
    assert.match(metadata[key], /^[a-f0-9]{64}$/, key);
    assert.equal(hash(name), metadata[key], name);
  }
  assert.equal(bytes('crop.wasm').length, metadata.wasm_bytes);
  objectKeys(metadata.source_sha256, [
    'ports/image/geometry.gooo', 'examples/crop-planner/crop_planner.gooo'
  ], 'source roster');
  const expectedEncodedSources = {};
  for (const [name, expected] of Object.entries(metadata.source_sha256)) {
    assert.equal(hash(`source/${name}`), expected, name);
    // Source receipts hash Go's JSON-encoded string, including HTML escaping.
    const encoded = JSON.stringify(bytes(`source/${name}`).toString('utf8'))
      .replace(/[<>&\u2028\u2029]/g, character => `\\u${character.charCodeAt(0).toString(16).padStart(4, '0')}`);
    expectedEncodedSources[name] = createHash('sha256').update(encoded).digest('hex');
  }

  // Receipt JSON is used only for identities/counts and generated source here.
  // Never parse its numeric coordinates through JavaScript Number for an oracle.
  const inputs = read('inputs.json');
  assert.equal(inputs.schema, 'gooo/body-composition-inputs/v1');
  assert.ok(Array.isArray(inputs.inputs) && inputs.inputs.length > 0);
  const generated = bytes('generated.go').toString('utf8');
  for (const name of ['execution.json', 'replay.json']) {
    const record = read(name);
    assert.equal(record.schema, 'gooo/workspace-body-execution-receipt/v1', name);
    assert.equal(record.decision, 'OBSERVED', name);
    assert.equal(record.manifest_digest, `sha256:${metadata.manifest_sha256}`, name);
    assert.equal(record.inputs_digest, `sha256:${metadata.inputs_sha256}`, name);
    assert.equal(record.result.composition.source, generated, name);
    const program = record.result.program;
    assert.equal(program.entry.package_path, 'example/crop-planner', name);
    assert.equal(program.entry.activity, 'CropPlan', name);
    const boundSources = {};
    for (const pkg of program.workspace_image.packages) {
      for (const source of pkg.sources) {
        assert.ok(!Object.hasOwn(boundSources, source.filename), `duplicate source: ${source.filename}`);
        assert.match(source.source_digest, /^sha256:[a-f0-9]{64}$/);
        boundSources[source.filename] = source.source_digest.slice(7);
      }
    }
    assert.deepEqual(boundSources, expectedEncodedSources, name);
    const runtime = record.result.runtime;
    assert.equal(runtime.stage, 'COMPLETE', name);
    assert.equal(runtime.generated_sha256, `sha256:${metadata.generated_sha256}`, name);
    assert.equal(runtime.producer_source_sha, metadata.compiler_revision, name);
    assert.equal(runtime.go_version.split(' ')[2], metadata.go_version, name);
    assert.equal(runtime.model_calls, 0, name);
    assert.equal(runtime.projection_replayed, true, name);
    assert.equal(runtime.runtime_replayed, true, name);
    assert.equal(runtime.traces.length, inputs.inputs.length, name);
  }
  assert.equal(read('replay.json').replayed_from_sha256, `sha256:${metadata.execution_sha256}`);

  const fixture = read('fixtures.json');
  assert.equal(fixture.schema, 'gooo/crop-browser-fixtures/v1');
  assert.equal(fixture.go_version, metadata.go_version);
  assert.equal(fixture.compiler_revision, metadata.compiler_revision);
  assert.equal(fixture.go_source_sha256['src/image/geom.go'], lock.source_sha256['src/image/geom.go']);
  assert.ok(Array.isArray(fixture.cases) && fixture.cases.length > 0);
  assert.ok(Array.isArray(fixture.rejections) && fixture.rejections.length > 0);
  assert.equal(fixture.cases.length, metadata.fixture_cases);
  assert.equal(fixture.rejections.length, metadata.fixture_rejections);
  const names = new Set();
  const named = (row, group) => {
    assert.equal(typeof row.name, 'string');
    assert.ok(row.name.length > 0 && !names.has(`${group}:${row.name}`), `duplicate ${group} name: ${row.name}`);
    names.add(`${group}:${row.name}`);
  };

  globalThis.crypto ??= webcrypto;
  require(path.join(site, 'wasm_exec.js'));
  const go = new Go();
  const { instance } = await WebAssembly.instantiate(bytes('crop.wasm'), go.importObject);
  let runtimeFailure;
  go.run(instance).then(() => { runtimeFailure = new Error('Crop runtime stopped.'); }, error => { runtimeFailure = error; });
  assert.equal(globalThis.goooCropReady, true);
  assert.equal(typeof globalThis.goooCrop, 'function');
  const invoke = (args, context) => {
    assert.equal(runtimeFailure, undefined, context);
    const raw = globalThis.goooCrop(...args);
    assert.equal(typeof raw, 'string', context);
    return JSON.parse(raw);
  };
  const reject = (args, context) => {
    const actual = invoke(args, context);
    objectKeys(actual, ['ok', 'error'], context);
    assert.equal(actual.ok, false, context);
    assert.equal(typeof actual.error, 'string', context);
    assert.ok(actual.error.length > 0, context);
  };
  for (const row of fixture.cases) {
    named(row, 'case');
    assert.ok(Array.isArray(row.inputs) && row.inputs.length === 8, row.name);
    row.inputs.forEach((value, index) => exactInteger(value, `${row.name}.input${index}`));
    cropOutput(row.expected, `${row.name}.expected`);
    const actual = invoke(row.inputs, row.name);
    objectKeys(actual, ['ok', 'inputs', 'output'], row.name);
    assert.equal(actual.ok, true, row.name);
    assert.deepEqual(actual.inputs, row.inputs, row.name);
    cropOutput(actual.output, `${row.name}.output`);
    assert.deepEqual(actual.output, row.expected, row.name);
  }
  for (const row of fixture.rejections) {
    named(row, 'rejection');
    assert.ok(Array.isArray(row.inputs) && row.inputs.every(value => typeof value === 'string'), row.name);
    reject(row.inputs, row.name);
  }

  let typeRejections = 0;
  const wrongTypes = [
    ['number', 0], ['unsafe number', 9007199254740992], ['NaN', NaN], ['Infinity', Infinity],
    ['boolean', true], ['null', null], ['undefined', undefined], ['object', {}], ['array', []],
    ['boxed string', new String('0')], ['bigint', 0n], ['symbol', Symbol('coordinate')], ['function', () => '0']
  ];
  for (let position = 0; position < 8; position++) {
    for (const [name, value] of wrongTypes) {
      const args = Array(8).fill('0');
      args[position] = value;
      reject(args, `wrong ${name} at input ${position}`);
      typeRejections++;
    }
  }
  let arityRejections = 0;
  for (const count of [...Array(8).keys(), 9, 16]) {
    reject(Array(count).fill('0'), `wrong arity ${count}`);
    arityRejections++;
  }
  const recoveryCase = fixture.cases[0];
  assert.deepEqual(invoke(recoveryCase.inputs, 'valid call after rejections'), {
    ok: true, inputs: recoveryCase.inputs, output: recoveryCase.expected
  });
  await Promise.resolve();
  assert.equal(runtimeFailure, undefined);
  return {
    schema: 'gooo/crop-browser-smoke/v1', status: 'PASS', cases: fixture.cases.length,
    fixture_rejections: fixture.rejections.length, type_rejections: typeRejections,
    arity_rejections: arityRejections, rejected: fixture.rejections.length + typeRejections + arityRejections,
    native_observations: inputs.inputs.length, compiler_revision: metadata.compiler_revision,
    source_revision: metadata.source_revision, wasm_sha256: metadata.wasm_sha256,
    scope: 'Actual shipped crop Wasm compared with independent Go fixtures; exact decimal-string inputs and outputs, invalid JavaScript types, arities and saved native evidence identities checked. No DOM or image-file processing test.'
  };
}

module.exports = { runSmoke };
if (require.main === module) {
  runSmoke(process.argv[2] || 'out/browser/site').then(result => {
    console.log(JSON.stringify(result));
    process.exit(0);
  }).catch(error => { console.error(error); process.exit(1); });
}
