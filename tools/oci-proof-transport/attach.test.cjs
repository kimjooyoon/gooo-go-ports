'use strict';
const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { originalBundle, attachOriginal } = require('./attach.cjs');
const contract = require('../../registry/oci-recovery/contract.json');
const raw = fs.readFileSync(path.join(__dirname, '../../registry/oci-recovery/original-build.bundle.json'));

test('the actual original bundle passes semantic subject checks independent of JSON key order', () => {
  const bundle = originalBundle(raw);
  const statement = JSON.parse(Buffer.from(bundle.dsseEnvelope.payload, 'base64'));
  assert.equal(statement.subject[0].digest.sha256, contract.digest.slice(7));
  assert.equal(statement.predicate.buildDefinition.resolvedDependencies[0].digest.gitCommit, contract.assembly_source);
});

test('changed bundle bytes are refused before credential access or network', async () => {
  let accessed = false;
  const forbidden = () => { accessed = true; throw Error('must not run'); };
  await assert.rejects(attachOriginal(Buffer.concat([raw, Buffer.from('\n')]), forbidden, forbidden), /ORIGINAL_BUNDLE_HASH/);
  assert.equal(accessed, false);
});

test('only the exact existing signed bytes and fixed subject are transported once', async () => {
  const calls = [];
  const credential = { username: 'dummy', password: 'not-a-real-token' };
  const result = await attachOriginal(raw, async (options) => {
    calls.push(options);
    return { digest: 'sha256:' + '1'.repeat(64) };
  }, image => { assert.equal(image, contract.image); return credential; });
  assert.equal(calls.length, 1);
  assert.equal(calls[0].artifact, raw);
  assert.equal(calls[0].credentials, credential);
  assert.equal(calls[0].imageDigest, contract.digest);
  assert.equal(calls[0].annotations['dev.sigstore.bundle.predicateType'], 'https://slsa.dev/provenance/v1');
  assert.equal(result.original_build_resigned, false);
  assert.equal(result.image_push_performed, false);
});

test('registry failure is propagated with no application-level retry', async () => {
  let calls = 0;
  await assert.rejects(attachOriginal(raw, async () => { calls++; throw Error('transport failed'); }, () => ({})), /transport failed/);
  assert.equal(calls, 1);
});

test('a response without a concrete attachment digest cannot become a successful receipt', async () => {
  await assert.rejects(attachOriginal(raw, async () => ({ digest: 'latest' }), () => ({})), /ATTACHMENT_DIGEST/);
});

test('dependency lock and actual credential reader match the recorded immutable upstream implementation', () => {
  const lock = require('./package-lock.json');
  const dependency = lock.packages['node_modules/@sigstore/oci'];
  assert.equal(dependency.version, contract.credential_package.version);
  assert.equal(dependency.integrity, contract.credential_package.integrity);
  const reader = path.join(path.dirname(require.resolve('@sigstore/oci')), 'credentials.js');
  assert.equal(crypto.createHash('sha256').update(fs.readFileSync(reader)).digest('hex'), contract.credential_package.reader_sha256);
});

test('original SBOM and historical scanner bytes bind to the exact original image config', () => {
  const { verifyMaterials } = require('./materials.cjs');
  const result = verifyMaterials(path.join(__dirname, '../../registry/oci-recovery'));
  assert.equal(result.subject_digest, contract.digest);
  assert.equal(result.current_vulnerability_scan, false);
});

test('altered retained SBOM cannot be passed to a new signing operation', () => {
  const os = require('node:os');
  const { verifyMaterials } = require('./materials.cjs');
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'gooo-material-tamper-'));
  try {
    const source = path.join(__dirname, '../../registry/oci-recovery');
    for (const name of fs.readdirSync(source)) fs.copyFileSync(path.join(source,name),path.join(temp,name));
    fs.appendFileSync(path.join(temp,'sbom.cdx.json'), '\n');
    assert.throws(() => verifyMaterials(temp));
  } finally { fs.rmSync(temp,{recursive:true,force:true}); }
});

test('verified registry result comparison requires complete original statements and exact SBOM facts', () => {
  const { verifyResults } = require('./verify-results.cjs');
  const build = {verificationResult:{statement:JSON.parse(Buffer.from(originalBundle(raw).dsseEnvelope.payload,'base64'))}};
  const sbomDocument = require('../../registry/oci-recovery/sbom.cdx.json');
  const sbom = {verificationResult:{statement:{predicateType:'https://cyclonedx.org/bom',
    subject:build.verificationResult.statement.subject,predicate:sbomDocument}}};
  assert.equal(verifyResults([build],[sbom],raw,sbomDocument).sbom_statement_is_new_recovery_attestation,true);
  for (const changed of [[],[{verificationResult:{statement:{...sbom.verificationResult.statement,predicate:{}}}}]]) {
    assert.throws(() => verifyResults([build],changed,raw,sbomDocument));
  }
  const alteredBuild = structuredClone(build);
  alteredBuild.verificationResult.statement.predicate.runDetails.metadata.invocationId='other';
  assert.throws(() => verifyResults([alteredBuild],[sbom],raw,sbomDocument));
});
