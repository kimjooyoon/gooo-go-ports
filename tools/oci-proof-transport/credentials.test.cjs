'use strict';
const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const vm = require('node:vm');
const { spawnSync } = require('node:child_process');
const helper = path.resolve(__dirname, '../registry_action_auth.py');
const dist = path.dirname(require.resolve('@sigstore/oci'));

// Execute the actual pinned reader with only os.homedir redirected into a
// disposable test directory. No real user Docker credentials are inspected.
function readerFor(home) {
  const module = { exports: {} };
  vm.runInNewContext(fs.readFileSync(path.join(dist, 'credentials.js'), 'utf8'), {
    module, exports: module.exports, Buffer, process,
    require(name) {
      if (name === 'node:os' || name === 'os') return { ...os, homedir: () => home };
      if (name === './name') return require(path.join(dist, 'name.js'));
      if (['node:fs', 'fs', 'node:path', 'path', 'node:child_process', 'child_process'].includes(name)) return require(name);
      throw Error('unexpected reader dependency: ' + name);
    },
  }, { filename: 'pinned-credentials.js' });
  return module.exports.getRegistryCredentials;
}

test('actual pinned reader fails custom-only, succeeds after bridge, and exact prior bytes/mode return', () => {
  const root = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'gooo-credential-contract-')));
  try {
    const home = path.join(root, 'fake-home');
    const privateDir = path.join(root, 'private');
    const stateDir = path.join(root, 'state');
    fs.mkdirSync(home); fs.mkdirSync(privateDir);
    fs.mkdirSync(path.join(home, '.docker'));
    const target = path.join(home, '.docker', 'config.json');
    const prior = Buffer.from('{"auths":{}}\n  ');
    fs.writeFileSync(target, prior, { mode: 0o640 });
    const config = path.join(privateDir, 'config.json');
    fs.writeFileSync(config, JSON.stringify({auths:{'ghcr.io':{auth:Buffer.from('dummy:synthetic-token').toString('base64')}}}), {mode:0o600});
    const env = { ...process.env, GITHUB_ACTIONS:'true', GITHUB_REPOSITORY:'kimjooyoon/gooo-go-ports',
      GITHUB_REF:'refs/heads/main', GITHUB_RUN_ATTEMPT:'1', GITHUB_RUN_ID:'123', GITHUB_ACTOR:'dummy',
      RUNNER_TEMP:fs.realpathSync(root), DOCKER_CONFIG:privateDir };
    const reader = readerFor(home);
    assert.throws(() => reader('ghcr.io/kimjooyoon/gooo-service-grant-mcp'), /No credentials found/);
    for (const mode of ['stage', 'restore']) {
      const result = spawnSync('python3', ['-I', '-B', helper, mode, '--home', home, '--state-dir', stateDir,
        ...(mode === 'stage' ? ['--private-config', config] : [])], {env, encoding:'utf8', timeout:10000});
      assert.equal(result.status, 0, result.stdout + result.stderr);
      assert.ok(!result.stdout.includes('synthetic-token'));
      if (mode === 'stage') {
        const credentials = reader('ghcr.io/kimjooyoon/gooo-service-grant-mcp');
        assert.equal(credentials.username, 'dummy');
        assert.equal(credentials.password, 'synthetic-token');
        assert.equal(fs.statSync(target).mode & 0o777, 0o600);
      }
    }
    assert.deepEqual(fs.readFileSync(target), prior);
    assert.equal(fs.statSync(target).mode & 0o777, 0o640);
    assert.throws(() => reader('ghcr.io/kimjooyoon/gooo-service-grant-mcp'), /No credentials found/);
  } finally { fs.rmSync(root, {recursive:true, force:true}); }
});
