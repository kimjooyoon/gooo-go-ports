#!/usr/bin/env python3
"""Generate exact native profiles from actual imported Gooo execution/replay.

The archived original generation is never rewritten or regenerated here.
Consuming the generated comparison needs neither this compiler nor a network.
"""
import argparse
import copy
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parent
REPO = ROOT.parent.parent
PIN = '205896a413e5dea92366309c35ae85e542d055f6'
HISTORICAL_COMPILER = '83928fa4538245abe33b08549da47f8cbdc50ed2'
MODULE = 'github.com/kimjooyoon/gooo-go-ports/examples/api-policy-revision'
CONFIGS = [
    ('candidate', 'candidate.workspace.json', 'app/api-policy-readonly', 'ReadOnlyAPI', 'gooo://api-revision/readonly/v1', '(bool,string,bool,string,bool,string,string,string,string)', 'ApiDecision', 'readonly.gooo'),
    ('flags', 'differences.workspace.json', 'gooo/api-policy-revision/classifier', 'DifferenceFlags', 'gooo://api-policy-revision/difference-flags', '(ApiDecision,ApiDecision)', 'ApiDifferenceFlags', 'compare.gooo'),
    ('summary', 'summary.workspace.json', 'gooo/api-policy-revision/classifier', 'ChangeSummary', 'gooo://api-policy-revision/change-summary', '(ApiDecision,ApiDecision)', 'ApiChangeSummary', 'compare.gooo'),
]
FILES = ['readonly.gooo', 'compare.gooo', 'candidate.workspace.json', 'differences.workspace.json', 'summary.workspace.json', 'profiles.json', 'compiler.json', 'adapter_generated.go', 'model.go', 'compare.go', 'input.go', 'bindings.go', 'go.mod', 'go.sum', 'cmd/compare/main.go', 'cmd/compare/diagnostics.go', 'generate.py', 'canonical/main.gooo', 'canonical/scalar_access.gooo', 'testdata/classifier-expected.json']
for prefix, *_ in CONFIGS:
    FILES += ['receipts/' + prefix + '-' + suffix for suffix in ['seed-inputs.json', 'execution.json', 'replay.json', 'lowered.gooo', 'body-codegen.json']]
    FILES += ['internal/' + prefix + '/generated.go']


def digest(raw):
    return 'sha256:' + hashlib.sha256(raw).hexdigest()


def source_digest(raw):
    quoted = json.dumps(raw.decode(), ensure_ascii=False, separators=(',', ':')).replace('<', '\\u003c').replace('>', '\\u003e').replace('&', '\\u0026').replace('\u2028', '\\u2028').replace('\u2029', '\\u2029').encode()
    return digest(quoted)


def canonical(value):
    return (json.dumps(value, indent=2, sort_keys=True) + '\n').encode()


def run(cmd, data=None):
    env = os.environ.copy()
    env.update(GOTOOLCHAIN='local', GOWORK='off', GOPROXY='off', GOSUMDB='off')
    return subprocess.run([str(x) for x in cmd], input=data, cwd=ROOT, env=env, capture_output=True, check=True, timeout=90).stdout


def check_archive():
    profile = json.loads((ROOT / 'profiles.json').read_bytes())['original']
    frozen = ROOT / 'profiles/original'
    assert not frozen.is_symlink() and not frozen.parent.is_symlink(), 'frozen module root symlink is unsupported'
    manifest_raw = (frozen / 'generation.json').read_bytes()
    assert digest(manifest_raw) == profile['generation_sha256'] == 'sha256:79b76bda704cb63b9390802766a66ac72ba02c04ae135600cb05de6797c97d46'
    manifest = json.loads(manifest_raw)
    assert manifest['schema'] == 'gooo/cel-public-generation/v1' and manifest['assets'] == profile['assets'] and len(profile['assets']) == 28
    expected_files = set(profile['assets']) | set(profile['packaging_assets']) | {'generation.json'}
    actual_files = {p.relative_to(frozen).as_posix() for p in frozen.rglob('*') if p.is_file()}
    assert actual_files == expected_files, 'frozen module file roster changed'
    assert not any(p.is_symlink() for p in frozen.rglob('*')), 'frozen module symlink is unsupported'
    for name, sha in {**profile['assets'], **profile['packaging_assets']}.items():
        assert digest((frozen / name).read_bytes()) == sha, 'frozen original byte changed: ' + name
    assert (frozen / 'go.mod').read_text().splitlines()[0] == 'module ' + profile['module']
    assert profile['source_commit'] == '8cddcc4e65afb42431f8f42a86f29859ae2f4d69'
    for name in ['main.gooo', 'scalar_access.gooo']:
        assert (ROOT / 'canonical' / name).read_bytes() == (frozen / 'canonical' / name).read_bytes()


def check_runtime_sources():
    # Tests are separate evidence, while every compiled non-test Go source must
    # be sealed. The nested module has its own exact immutable roster above.
    actual = {p.relative_to(ROOT).as_posix() for p in ROOT.rglob('*.go')
              if not p.name.endswith('_test.go') and not p.is_relative_to(ROOT / 'profiles/original')}
    assert actual == {name for name in FILES if name.endswith('.go')}, 'unsealed compiled Go source'


def program(receipt, cfg, expected_go="go1.27.3", expected_compiler=PIN):
    prefix, workspace, package, activity, _, _, _, source = cfg
    assert receipt['schema'] == 'gooo/workspace-body-execution-receipt/v1' and receipt['decision'] in ['OBSERVED', 'PASS']
    assert receipt['manifest_digest'] == digest((ROOT / workspace).read_bytes())
    assert receipt['inputs_digest'] == digest((ROOT / 'receipts' / (prefix + '-seed-inputs.json')).read_bytes())
    p = receipt['result']['program']
    assert p['schema'] == 'gooo/workspace-body-program/v1' and p['entry']['package_path'] == package and p['entry']['activity'] == activity and p['entry']['lowered_name']
    assert p['workspace_image']['entry']['source'] == source
    sources = {s['filename']: s['source_digest'] for pkg in p['workspace_image']['packages'] for s in pkg['sources']}
    assert set(sources) == {'canonical/main.gooo', 'canonical/scalar_access.gooo', source}
    for name, sha in sources.items():
        assert sha == source_digest((ROOT / name).read_bytes())
    r = receipt['result']['runtime']
    assert r['stage'] == 'COMPLETE' and r['producer_source_sha'] == expected_compiler and r['go_version'].split()[:3] == ['go', 'version', expected_go]
    assert r['projection_replayed'] and r['runtime_replayed'] and len(r['runs']) == 2
    for process in [r['toolchain'], r['build'], *r['runs']]:
        assert process['started'] and process['completed'] and process['exit_code'] == 0 and not process['canceled'] and not process['timed_out'] and not process['output_truncated']
    traces = r['traces']
    if prefix != 'candidate':
        expected = json.loads((ROOT / 'testdata/classifier-expected.json').read_bytes())['cases']
        assert len(traces) == len(expected) == 23
        for trace, case in zip(traces, expected):
            assert trace['deliveries'][0]['actual'] == case[prefix if prefix == 'flags' else 'summary']
    return p


def stable_body(body):
    value = copy.deepcopy(body)
    latency = value['report'].pop('route_decision_latency_ms')
    assert isinstance(latency, (int, float)) and latency >= 0
    environment = value['report']['completeness_receipt']['scope'].pop('execution_environment')
    assert environment.count('/') == 1
    return value


def check_saved_replay(receipt, raw_execution):
    assert receipt['replayed_from_sha256'] == digest(raw_execution)
    replay = receipt['result']['replay']
    assert replay['schema'] == 'gooo/workspace-body-replay/v1' and type(replay['model_calls']) is int and replay['model_calls'] == 0
    # prior_result_sha256 uses producer-specific typed normalization. This check
    # binds the raw saved execution; it does not infer that separate inner hash.



def stored_compiler_profile():
    """Select the exact stored producer role; never relabel a saved receipt."""
    value = json.loads((ROOT / 'compiler.json').read_bytes())
    historical = value.get('compiler_source_sha') == HISTORICAL_COMPILER
    revision = HISTORICAL_COMPILER if historical else PIN
    version = '0.6.32-dev' if historical else '0.6.37-dev'
    go = 'go1.27.2' if historical else 'go1.27.3'
    assert value['compiler_source_sha'] == value['vcs_revision'] == revision
    assert value['version'] == version and value['go_version'] == value['native_go_required'] == go
    assert value['source_status'] == 'CLEAN_VCS' and value['vcs_modified'] == 'false'
    return go, revision

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--compiler', type=Path, default=REPO / '.gooo')
    parser.add_argument('--go', type=Path, required=True)
    parser.add_argument('--check', action='store_true')
    args = parser.parse_args()
    args.compiler = args.compiler.resolve()
    args.go = args.go.resolve()
    version = run([args.go, 'version']).decode().split()
    assert version[:3] == ['go', 'version', 'go1.27.3']
    compiler_raw = run([args.compiler, 'version', '--build', '--json'])
    compiler = json.loads(compiler_raw)
    assert compiler['compiler_source_sha'] == compiler['vcs_revision'] == PIN and compiler['vcs_modified'] == 'false' and compiler['source_status'] == 'CLEAN_VCS'
    assert compiler['go_version'] == compiler['native_go_required'] == 'go1.27.3' and compiler['decision_runtime']['version'] == 'v0.2.51-experimental'
    check_archive()
    if args.check:
        check_runtime_sources()
    saved_go, saved_compiler = stored_compiler_profile() if args.check else (None, None)
    assets = {'compiler.json': compiler_raw}
    entries = {}
    with tempfile.TemporaryDirectory(prefix='gooo-api-revision-') as tmp:
        for cfg in CONFIGS:
            prefix, workspace, _, _, activity_id, input_type, output_type, _ = cfg
            seed = 'receipts/' + prefix + '-seed-inputs.json'
            fresh = run([args.compiler, 'package', 'execute', '--json', '--inputs', seed, '--go', args.go, workspace])
            receipt = json.loads(fresh)
            p = program(receipt, cfg)
            if args.check:
                assert program(json.loads((ROOT / 'receipts' / (prefix + '-execution.json')).read_bytes()), cfg, expected_go=saved_go, expected_compiler=saved_compiler) == p
                saved_replay = json.loads((ROOT / 'receipts' / (prefix + '-replay.json')).read_bytes())
                check_saved_replay(saved_replay, (ROOT / 'receipts' / (prefix + '-execution.json')).read_bytes())
                assert program(saved_replay, cfg, expected_go=saved_go, expected_compiler=saved_compiler) == p
            else:
                assets['receipts/' + prefix + '-execution.json'] = fresh
                execution_file = Path(tmp) / (prefix + '-execution.json')
                execution_file.write_bytes(fresh)
                replay = run([args.compiler, 'package', 'replay', '--json', '--receipt', execution_file, '--inputs', seed, '--go', args.go, workspace])
                check_saved_replay(json.loads(replay), fresh)
                assert program(json.loads(replay), cfg) == p
                assets['receipts/' + prefix + '-replay.json'] = replay
            lowered = p['lowered_gooo_source'].encode()
            assets['receipts/' + prefix + '-lowered.gooo'] = lowered
            lowered_file = Path(tmp) / (prefix + '-lowered.gooo')
            lowered_file.write_bytes(lowered)
            generated = run([args.compiler, 'body-codegen', '--json', '--activity', p['entry']['lowered_name'], lowered_file])
            body = json.loads(generated)
            report = body['report']
            assert report['schema'] == 'gooo/body-codegen-report/v3' and report['decision'] == 'PASS' and report['activity'] == p['entry']['lowered_name']
            assert report['activity_id'] == activity_id and report['input_type'] == input_type and report['output_type'] == output_type and report['compiler_source_sha'] == PIN
            assert report['source_digest'] == digest(lowered) and report['generated_digest'] == report['replay_digest'] == digest(body['source'].encode())
            assert report['typecheck_passed'] and report['deterministic_replay'] and report['repository_writes'] == 0
            assets['receipts/' + prefix + '-body-codegen.json'] = generated
            assets['internal/' + prefix + '/generated.go'] = run([args.go.parent / 'gofmt'], ('// Code generated from compiler-owned workspace lowering; DO NOT EDIT.\n' + body['source']).encode())
            entries[prefix] = p['entry']['lowered_name']
            if args.check:
                assert stable_body(json.loads((ROOT / 'receipts' / (prefix + '-body-codegen.json')).read_bytes())) == stable_body(body)
    record_type = 'GoooRecord' + hashlib.sha256(b'gooo://api-example/decision/v1').hexdigest()
    adapter = '''// Code generated from actual lowered entry identities; DO NOT EDIT.
package revisionpolicy
import (
 "encoding/json"
 original "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy"
 candidate "%s/internal/candidate"
 flags "%s/internal/flags"
 summary "%s/internal/summary"
)
func nativeCandidate(f original.APIFacts) any { return candidate.%s(f.GroupPresent, f.GroupValue, f.PrincipalPresent, f.PrincipalValue, f.Authenticated, f.CallerTenant, f.ResourceTenant, f.RequestedAction, f.AllowedAction) }
func nativeDifferences(before, after original.APIDecision) (any, error) {
 var a, b flags.%s
 first, err := json.Marshal(before); if err != nil { return nil, err }; if err := json.Unmarshal(first, &a); err != nil { return nil, err }
 second, err := json.Marshal(after); if err != nil { return nil, err }; if err := json.Unmarshal(second, &b); err != nil { return nil, err }
 return flags.%s(a,b), nil
}
func nativeSummary(before, after original.APIDecision) (any, error) {
 var a, b summary.%s
 first, err := json.Marshal(before); if err != nil { return nil, err }; if err := json.Unmarshal(first, &a); err != nil { return nil, err }
 second, err := json.Marshal(after); if err != nil { return nil, err }; if err := json.Unmarshal(second, &b); err != nil { return nil, err }
 return summary.%s(a,b), nil
}
''' % (MODULE, MODULE, MODULE, entries['candidate'], record_type, entries['flags'], record_type, entries['summary'])
    assets['adapter_generated.go'] = run([args.go.parent / 'gofmt'], adapter.encode())
    if args.check:
        for name, raw in assets.items():
            if name.endswith('-body-codegen.json'):
                continue
            assert (ROOT / name).read_bytes() == raw, 'generated bytes changed: ' + name
        manifest = json.loads((ROOT / 'generation.json').read_bytes())
        assert manifest['schema'] == 'gooo/api-policy-revision-generation/v1' and manifest['compiler_source_sha'] == PIN
        assert manifest['assets'] == {name: digest((ROOT / name).read_bytes()) for name in FILES}
        print('PASS: frozen original bytes, three actual Gooo native seed programs, saved lowering/replay, source bindings and exact projections')
    else:
        for name, raw in assets.items():
            destination = ROOT / name
            destination.parent.mkdir(parents=True, exist_ok=True)
            destination.write_bytes(raw)
        check_runtime_sources()
        (ROOT / 'generation.json').write_bytes(canonical({'schema': 'gooo/api-policy-revision-generation/v1', 'compiler_source_sha': PIN, 'assets': {name: digest((ROOT / name).read_bytes()) for name in FILES}}))
        print('PASS: three actual Gooo native programs and saved replay generated; native two-profile comparison validation remains separate')


if __name__ == '__main__':
    main()
