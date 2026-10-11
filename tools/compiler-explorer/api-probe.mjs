import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {readFile, writeFile} from 'node:fs/promises';

const read = name => readFile(new URL(name, import.meta.url), 'utf8');
const hash = text => createHash('sha256').update(text).digest('hex');
const languagesResponse = await fetch('http://127.0.0.1:10249/api/languages?fields=id,name,extensions,defaultOptions,defaultCompiler', {headers: {Accept: 'application/json'}});
assert.equal(languagesResponse.status, 200);
const languages = await languagesResponse.json();
const goooLanguage = languages.find(language => language.id === 'gooo');
assert.equal(goooLanguage?.defaultOptions, '--activity Clamp', 'language API must expose the configured selector');
const source = await read('clamp.gooo');
const idle = {
    producePp: null, produceGccDump: {libraryFunctions: false},
    produceCfg: false, produceOptInfo: false, produceIr: null,
    produceClangir: null, produceOptPipeline: null, produceLeanC: null,
    produceYul: null, overrides: [],
};
const cases = [
    {name: 'clamp', source, expected: await read('clamp.stdout')},
    {name: 'changed', source: await read('changed.gooo'), expected: await read('changed.stdout')},
    {name: 'invalid-body', source: await read('invalid-body.gooo'), failed: true},
    {name: 'corrected', source, expected: await read('clamp.stdout')},
    {name: 'malformed', source: await read('malformed.gooo'), failed: true},
    {name: 'missing-selector', source, args: '', failed: true, diagnostic: 'requires only --activity NAME'},
    {name: 'blocked-json', source, args: '--activity Clamp --json', failed: true, diagnostic: 'requires only --activity NAME'},
    {name: 'blocked-execution', source, filters: {execute: true}, failed: true, diagnostic: 'does not support binary output or execution'},
    {name: 'blocked-active-dump', source, backend: {produceGccDump: {opened: true}}, failed: true, diagnostic: 'does not support backend option produceGccDump'},
    {name: 'selected-double', source: await read('two-activities.gooo'), args: '--activity Double', expected: await read('selected-double.stdout')},
    {name: 'unicode-selector', source: source.replaceAll('Clamp', '계산_２'), args: '--activity 계산_２', expected: (await read('clamp.stdout')).replaceAll('Clamp', '계산_２')},
];
const observations = [];
for (const test of cases) {
    const request = {
        source: test.source, lang: 'gooo',
        options: {
            userArguments: test.args ?? '--activity Clamp',
            compilerOptions: {...idle, ...test.backend},
            filters: {labels: true, directives: true, commentOnly: true, trim: true, ...test.filters},
            tools: [], libraries: [], executeParameters: {args: [], stdin: ''},
        },
    };
    const response = await fetch('http://127.0.0.1:10249/api/compiler/gooolocal/compile', {
        method: 'POST', headers: {'Content-Type': 'application/json', Accept: 'application/json'},
        body: JSON.stringify(request),
    });
    assert.equal(response.status, 200, test.name);
    const result = await response.json();
    const generated = (result.asm ?? []).map(line => line.text).join('\n');
    const stderr = (result.stderr ?? []).map(line => line.text).join('\n');
    if (test.failed) {
        assert.notEqual(result.code, 0, test.name);
        assert(generated === '' || generated === '<Compilation failed>', `${test.name}: unexpected generated source on failure`);
        assert.equal((result.stdout ?? []).length, 0, `${test.name}: unexpected stdout on failure`);
        assert(!stderr.includes('Internal Compiler Explorer error'), `${test.name}: internal error`);
        assert(stderr.length > 0, `${test.name}: missing diagnostic`);
        if (test.diagnostic) assert(stderr.includes(test.diagnostic), `${test.name}: ${stderr}`);
    } else {
        assert.equal(result.code, 0, `${test.name}: ${stderr}`);
        assert.equal(result.languageId, 'go', test.name);
        assert.equal(generated, test.expected, `${test.name}: CE must preserve exact CLI source`);
    }
    observations.push({name: test.name, request, result, generated_sha256: hash(generated), pass: true});
}
await writeFile(new URL('api-observation.json', import.meta.url), JSON.stringify({
    schema: 'gooo/compiler-explorer-local-api-proof/v1',
    observed_at: new Date().toISOString(), status: 'LOCAL_ACTUAL_COMPILER_API_PASS',
    external_hosted_support: false, customer_adoption: false,
    language_metadata: goooLanguage,
    cases: observations,
}, null, 2) + '\n');
console.log(`PASS: ${observations.length} actual-compiler API cases`);
