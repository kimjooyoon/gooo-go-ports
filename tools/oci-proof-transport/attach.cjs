'use strict';
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const contract = require('../../registry/oci-recovery/contract.json');

function originalBundle(raw) {
  if (raw.length > 1048576 || crypto.createHash('sha256').update(raw).digest('hex') !== contract.original_bundle_sha256) {
    throw new Error('ORIGINAL_BUNDLE_HASH');
  }
  const bundle = JSON.parse(raw);
  const statement = JSON.parse(Buffer.from(bundle.dsseEnvelope.payload, 'base64'));
  if (statement.predicateType !== 'https://slsa.dev/provenance/v1' ||
      !Array.isArray(statement.subject) || statement.subject.length !== 1 ||
      statement.subject[0].name !== contract.image ||
      Object.keys(statement.subject[0]).sort().join(',') !== 'digest,name' ||
      Object.keys(statement.subject[0].digest).join(',') !== 'sha256' ||
      statement.subject[0].digest.sha256 !== contract.digest.slice(7) ||
      statement.predicate.runDetails.metadata.invocationId !== `https://github.com/kimjooyoon/gooo-go-ports/actions/runs/${contract.assembly_run}/attempts/1`) {
    throw new Error('ORIGINAL_BUNDLE_SUBJECT');
  }
  return bundle;
}

async function attachOriginal(raw, attachArtifactToImage, getRegistryCredentials) {
  const bundle = originalBundle(raw);
  // Cryptographic certificate/source/hosted-runner verification is a separate
  // required workflow step before this byte-preserving transport operation.
  const result = await attachArtifactToImage({
    credentials: getRegistryCredentials(contract.image),
    imageName: contract.image,
    imageDigest: contract.digest,
    artifact: raw,
    mediaType: bundle.mediaType,
    annotations: {
      'dev.sigstore.bundle.content': 'dsse-envelope',
      'dev.sigstore.bundle.predicateType': 'https://slsa.dev/provenance/v1',
    },
    fetchOpts: { timeout: 30000, retry: 3 },
  });
  if (!/^sha256:[0-9a-f]{64}$/.test(result.digest)) throw new Error('ATTACHMENT_DIGEST');
  return { status: 'ORIGINAL_BUILD_BUNDLE_TRANSPORTED', image: contract.image,
    subject_digest: contract.digest, attachment_digest: result.digest,
    original_bundle_sha256: contract.original_bundle_sha256,
    image_push_performed: false, original_build_resigned: false };
}

async function main() {
  if (process.env.GITHUB_ACTIONS !== 'true' || process.env.GITHUB_REPOSITORY !== 'kimjooyoon/gooo-go-ports' ||
      process.env.GITHUB_REF !== 'refs/heads/main' || process.env.GITHUB_RUN_ATTEMPT !== '1') {
    throw new Error('GITHUB_CONTEXT');
  }
  const [bundlePath, outputPath] = process.argv.slice(2);
  if (!bundlePath || !outputPath) throw new Error('ARGUMENTS');
  const raw = fs.readFileSync(bundlePath);
  const { attachArtifactToImage, getRegistryCredentials } = require('@sigstore/oci');
  const result = await attachOriginal(raw, attachArtifactToImage, getRegistryCredentials);
  fs.writeFileSync(outputPath, JSON.stringify(result, null, 2) + '\n', { flag: 'wx', mode: 0o600 });
  console.log(JSON.stringify(result));
}

module.exports = { originalBundle, attachOriginal };
if (require.main === module) {
  main().catch(() => {
    // Registry clients may include request details in exceptions. Never log
    // those objects or a basic-auth value; retain the phase and first failure.
    console.error('ORIGINAL_BUILD_BUNDLE_TRANSPORT_FAILED');
    process.exitCode = 1;
  });
}
