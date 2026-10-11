'use strict';

// Check the shipped saved text against the independent Compiler Explorer pins.
// This does not compile or execute Gooo or the generated Go.
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');

const site = path.resolve(process.argv[2] || 'out/browser/site');
const fixtures = path.join(__dirname, 'compiler-explorer', 'fixtures');
const pins = JSON.parse(fs.readFileSync(path.join(__dirname, 'compiler-explorer', 'pins.json'), 'utf8'));
const book = JSON.parse(fs.readFileSync(path.join(site, 'source-view.json'), 'utf8'));
const sha = value => crypto.createHash('sha256').update(value).digest('hex');
const revision = '09861502085012fe41ab5993a220e3fde7f0a87d';
const sourcePrefix = `https://github.com/kimjooyoon/gooo-go-ports/blob/${revision}/tools/compiler-explorer/fixtures/`;
const expected = [
  ['clamp', 'Clamp', 'clamp.gooo', 'clamp.stdout'],
  ['changed', 'Clamp', 'changed.gooo', 'changed.stdout'],
  ['double', 'Double', 'two-activities.gooo', 'selected-double.stdout'],
  ['invalid-body', 'Clamp', 'invalid-body.gooo', null],
];

assert.equal(book.schema, 'gooo/source-view-book/v1');
assert.equal(book.metadata.compiler_revision, pins.compiler_revision);
assert.equal(book.metadata.compiler_version, pins.compiler_version);
assert.equal(book.metadata.go_version, pins.compiler_go_version);
assert.equal(book.metadata.source_status, 'CLEAN_VCS');
assert.equal(book.metadata.evidence_source_revision, revision);
assert.equal(book.metadata.evidence_url, 'https://github.com/kimjooyoon/gooo-go-ports/actions/runs/38152688106');
assert.equal(book.metadata.compiler_executes_in_browser, false);
assert.equal(book.metadata.generated_go_executed_by_this_page, false);
assert.ok(Number.isFinite(Date.parse(book.metadata.observed_at)));
assert.deepEqual(book.cases.map(item => item.id), expected.map(item => item[0]));

function checkPinned(text, recordedHash, filename) {
  const original = fs.readFileSync(path.join(fixtures, filename));
  assert.equal(sha(original), pins.fixtures_sha256[filename], `${filename}: independent fixture pin`);
  assert.deepEqual(Buffer.from(text, 'utf8'), original, `${filename}: shipped bytes`);
  assert.equal(recordedHash, pins.fixtures_sha256[filename], `${filename}: recorded binding`);
}

for (const [id, activity, sourceName, stdoutName] of expected) {
  const item = book.cases.find(saved => saved.id === id);
  assert.equal(item.activity, activity);
  assert.equal(item.source_url, sourcePrefix + sourceName);
  assert.ok(typeof item.title === 'string' && item.title.length > 0);
  assert.ok(typeof item.description === 'string' && item.description.length > 0);
  checkPinned(item.source, item.source_sha256, sourceName);
  if (stdoutName) {
    checkPinned(item.stdout, item.stdout_sha256, stdoutName);
    assert.equal(item.exit_code, 0);
    assert.equal(item.stderr, '');
  } else {
    assert.equal(item.exit_code, 1);
    assert.equal(item.stdout, '');
    assert.equal(item.stdout_sha256, sha(''));
    // Exact stderr from the linked actual Linux CLI observation, not an API renderer.
    assert.equal(item.stderr, 'gooo: invalid-body.gooo: body-codegen: unsupported activity statement *ast.ForStmt\n');
    assert.equal(item.stderr_sha256, '0d0862ec07e13fe6810a13304758552258e95abe11c0a6865a1e345a074124e5');
  }
  assert.equal(sha(item.stderr), item.stderr_sha256, `${id}: stderr bytes`);
}
for (const name of ['source-view.html', 'source-view.css', 'source-view.js']) {
  assert.ok(fs.statSync(path.join(site, name)).isFile(), `shipped asset: ${name}`);
}
process.stdout.write(JSON.stringify({
  schema: 'gooo/source-view-shipped-check/v1',
  cases: expected.length,
  accepted_saved_cases: 3,
  rejected_saved_cases: 1,
  compiler_revision: pins.compiler_revision,
  independent_fixture_bindings_match: true,
  saved_source_only: true,
  status: 'PASS',
}, null, 2) + '\n');
