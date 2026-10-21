'use strict';
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const contract = require('../../registry/oci-recovery/contract.json');
const { originalBundle } = require('./attach.cjs');

function verifyMaterials(directory) {
  const documents = {};
  for (const [name, ref] of Object.entries(contract.original_materials)) {
    const file = path.join(directory, name);
    assert.ok(fs.lstatSync(file).isFile());
    const raw = fs.readFileSync(file);
    assert.equal(raw.length, ref.bytes);
    assert.equal(crypto.createHash('sha256').update(raw).digest('hex'), ref.sha256);
    documents[name] = JSON.parse(raw);
  }
  originalBundle(fs.readFileSync(path.join(directory, 'original-build.bundle.json')));
  const m = documents['build-manifest.json'];
  assert.equal(m.workflow_commit, contract.assembly_source);
  assert.equal(m.run_id, String(contract.assembly_run));
  assert.equal(m.run_attempt, '1');
  assert.equal(m.repository, 'kimjooyoon/gooo-go-ports');
  assert.equal(m.local_image_id, documents['public-manifest.json'].config.digest);
  assert.equal(contract.original_materials['public-manifest.json'].sha256, contract.digest.slice(7));
  assert.equal(m.native_source, 'fceb89f8b53eabd6687d16258d7cee89853c2cfe');
  assert.equal(m.native_version, 'v0.1.0-preview.16');
  assert.equal(m.archive_sha256, '0ae9bc25eef2ad73535642a66b8b7602e9f64a148b3e60696e36048a3b4a0524');
  assert.equal(m.binary_sha256, 'a848a8747669e46e9afef6c0f1ed8b2014df4838cc370416f60ab9c6acccfc0a');
  for (const name of ['sbom.cdx.json', 'scan.json', 'trivy-db-version.json']) {
    assert.deepEqual(m.files[name], contract.original_materials[name]);
  }
  assert.equal(documents['scan.json'].Metadata.ImageID, m.local_image_id);
  assert.equal(documents['sbom.cdx.json'].bomFormat, 'CycloneDX');
  assert.equal(documents['trivy-db-version.json'].Version, '0.76.0');
  const vulnerable = documents['scan.json'].Results.flatMap(r => r.Vulnerabilities || [])
    .filter(v => ['HIGH', 'CRITICAL'].includes(v.Severity));
  assert.equal(vulnerable.length, 0);
  return {status:'ORIGINAL_MATERIALS_BYTE_BINDINGS_VERIFIED', image:contract.image,
    subject_digest:contract.digest, original_assembly_source:contract.assembly_source,
    original_assembly_run:contract.assembly_run,
    current_vulnerability_scan:false, original_build_resigned:false, image_push_performed:false};
}
module.exports = { verifyMaterials };
if (require.main === module) {
  try { console.log(JSON.stringify(verifyMaterials(path.join(__dirname, '../../registry/oci-recovery')))); }
  catch { console.error('ORIGINAL_MATERIAL_BINDING_FAILED'); process.exitCode = 1; }
}
