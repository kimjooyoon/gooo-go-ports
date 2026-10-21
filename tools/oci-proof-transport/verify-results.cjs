'use strict';
const fs = require('node:fs');
const assert = require('node:assert/strict');
const { originalBundle } = require('./attach.cjs');
const contract = require('../../registry/oci-recovery/contract.json');

// These inputs must be successful gh attestation verify --format=json outputs.
// This comparison adds exact content binding; it is not signature verification.
function verifyResults(buildRows, sbomRows, originalRaw, originalSBOM) {
  const original = originalBundle(originalRaw);
  const originalStatement = JSON.parse(Buffer.from(original.dsseEnvelope.payload, 'base64'));
  const expectedSubject = [{name:contract.image,digest:{sha256:contract.digest.slice(7)}}];
  assert.ok(Array.isArray(buildRows) && buildRows.length > 0);
  assert.ok(Array.isArray(sbomRows) && sbomRows.length > 0);
  assert.ok(buildRows.some(row => {
    try { assert.deepEqual(row.verificationResult.statement, originalStatement); return true; }
    catch { return false; }
  }), 'no exact original verified build statement');
  assert.ok(sbomRows.some(row => {
    try {
      const statement = row.verificationResult.statement;
      assert.equal(statement.predicateType,'https://cyclonedx.org/bom');
      assert.deepEqual(statement.subject,expectedSubject);
      assert.deepEqual(statement.predicate,originalSBOM);
      return true;
    } catch { return false; }
  }), 'no verified exact retained SBOM');
  return {status:'REGISTRY_VERIFIED_STATEMENTS_MATCH_ORIGINAL_CONTENT',
    subject_digest:contract.digest, original_build_resigned:false,
    sbom_statement_is_new_recovery_attestation:true};
}
module.exports = { verifyResults };
if (require.main === module) {
  try {
    const base = require('node:path').join(__dirname,'../../registry/oci-recovery');
    const [build,sbom] = process.argv.slice(2);
    console.log(JSON.stringify(verifyResults(JSON.parse(fs.readFileSync(build)),JSON.parse(fs.readFileSync(sbom)),
      fs.readFileSync(base+'/original-build.bundle.json'),JSON.parse(fs.readFileSync(base+'/sbom.cdx.json')))));
  } catch { console.error('REGISTRY_VERIFIED_CONTENT_BINDING_FAILED'); process.exitCode=1; }
}
