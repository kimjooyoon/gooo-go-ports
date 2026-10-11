import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {cp, mkdtemp, readFile, rm, writeFile} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import path from 'node:path';
import test from 'node:test';

const toolDirectory = new URL('.', import.meta.url);
const preloader = `
import fs from 'node:fs';
const read = name => fs.readFileSync(new URL(name, import.meta.url), 'utf8');
const clamp = read('clamp.stdout');
const changed = read('changed.stdout');
const double = read('selected-double.stdout');
const expected = [clamp, changed, null, clamp, null, null, null, null, null, double, clamp.replaceAll('Clamp', '계산_２')];
const diagnostics = {
  5: 'requires only --activity NAME', 6: 'requires only --activity NAME',
  7: 'does not support binary output or execution', 8: 'does not support backend option produceGccDump',
};
let index = 0;
globalThis.fetch = async (url, options) => {
  if (!options?.body) return {status: 200, json: async () => [{id: 'gooo', defaultOptions: '--activity Clamp'}]};
  const current = index++;
  const output = expected[current];
  const failed = output === null;
  const mode = process.env.PROBE_FAILURE_MODE;
  const fragment = 'package sample\\n//gooo:generated:start id="sample://clamp" kind="activity"';
  const generated = failed ? (mode === 'partial-asm' ? fragment : mode === 'sentinel' ? '<Compilation failed>' : '') : output;
  return {status: 200, json: async () => ({
    code: failed ? 1 : 0, languageId: 'go',
    asm: generated ? generated.split('\\n').map(text => ({text})) : [],
    stdout: failed && mode === 'partial-stdout' ? [{text: fragment}] : [],
    stderr: failed ? [{text: diagnostics[current] ?? 'unsupported input'}] : [],
  })};
};
`;

for (const mode of ['empty', 'sentinel', 'partial-asm', 'partial-stdout']) {
    test(`probe ${mode.startsWith('partial-') ? 'rejects' : 'accepts'} failed output: ${mode}`, async () => {
        const scratch = await mkdtemp(path.join(tmpdir(), 'gooo-ce-probe-safety-'));
        try {
            await cp(new URL('fixtures/', toolDirectory), scratch, {recursive: true});
            await writeFile(path.join(scratch, 'api-probe.mjs'), await readFile(new URL('api-probe.mjs', toolDirectory)));
            const preloadPath = path.join(scratch, 'fake-fetch.mjs');
            await writeFile(preloadPath, preloader);
            const result = spawnSync(process.execPath, ['--import', preloadPath, path.join(scratch, 'api-probe.mjs')], {
                encoding: 'utf8', env: {...process.env, PROBE_FAILURE_MODE: mode}, timeout: 5000,
            });
            assert.equal(result.error, undefined);
            if (mode.startsWith('partial-')) {
                assert.notEqual(result.status, 0, 'partial generated Go must never be reported as PASS');
                assert.match(result.stderr, /unexpected (generated source|stdout) on failure/);
                await assert.rejects(readFile(path.join(scratch, 'api-observation.json')), {code: 'ENOENT'});
            } else {
                assert.equal(result.status, 0, result.stderr);
                const observation = JSON.parse(await readFile(path.join(scratch, 'api-observation.json'), 'utf8'));
                assert.equal(observation.cases.length, 11);
                assert(observation.cases.every(item => item.pass));
            }
        } finally {
            await rm(scratch, {recursive: true, force: true});
        }
    });
}
