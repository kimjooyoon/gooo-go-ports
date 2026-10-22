"""Reproduce source-owned selection and verify the actual public JSON command.

Shares the existing cancellation/owned-process runner; it does not build an
engine or interpret a saved PASS as a substitute for fresh execution.
"""
import argparse
import copy
import importlib.util
import json
import os
from pathlib import Path
import shutil
import sys
import traceback

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
ENTRY = 'gooo/service-policy/candidate-selection:SelectCandidate'
PIN = '205896a413e5dea92366309c35ae85e542d055f6'
RUNNER = ROOT / 'examples/service-grant-many/verify.py'


def load(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


host = load('grant_selection_owned_runner', RUNNER)


def check_receipts(execution, replay, cases_path, go_identity):
    cases = host.strict(cases_path.read_bytes())['cases']
    expected_inputs = [case['inputs'][ENTRY] for case in cases]
    source = ROOT / 'ports/service-policy/candidate_selection.gooo'
    parent = None
    for path in (execution, replay):
        receipt = host.strict(path.read_bytes())
        assert receipt['decision'] == 'PASS'
        assert receipt['manifest_digest'] == 'sha256:' + host.ref(ROOT / 'grant-candidate-selection.workspace.json')['sha256']
        assert receipt['cases_digest'] == 'sha256:' + host.ref(cases_path)['sha256']
        result = receipt['result']
        runtime = result['runtime']
        assert runtime['producer_source_sha'] == PIN
        assert runtime['go_tool_sha256'] == 'sha256:' + go_identity['sha256']
        assert runtime['go_version'] == go_identity['version']
        assert runtime['model_calls'] == 0
        assert runtime['finite_passed'] == runtime['finite_total'] == len(cases)
        assert len(runtime['runs']) == 2 and len(runtime['traces']) == len(cases)
        origins = result['program']['activity_origins']
        assert len(origins) == 1
        assert origins[0]['input_source']['source_sha256'] == 'sha256:' + host.ref(source)['sha256']
        for index, (trace, case) in enumerate(zip(runtime['traces'], cases)):
            assert trace['case_index'] == index and len(trace['deliveries']) == 1
            delivery = trace['deliveries'][0]
            assert delivery['activity_id'] == 'gooo://service-policy/candidate-selection/select/v1'
            host.exact(delivery['input'], expected_inputs[index])
            host.exact(delivery['actual'], case['expected'][ENTRY])
            assert len(delivery['actual']) == 12 and delivery['actual']['execution_authorized'] is False
        if parent is None:
            parent = receipt
        else:
            assert receipt['replayed_from_sha256'] == 'sha256:' + host.ref(execution)['sha256']
            host.exact(receipt['result']['composition'], parent['result']['composition'])
    return len(cases)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--compiler', type=Path, required=True)
    parser.add_argument('--go', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--go-cache', type=Path, help='Reuse a reviewed build cache; never removed by this verifier.')
    parser.add_argument('--module-cache', type=Path, help='Reuse a pinned-module cache; never removed by this verifier.')
    args = parser.parse_args()
    out = args.out.resolve()
    assert not out.is_relative_to(HERE), 'evidence must not modify the consumer module'
    out.mkdir(parents=True, exist_ok=False)
    host.R = out
    first = None
    summary = {'schema': 'gooo/grant-candidate-selection-verification/v1',
               'status': 'FIRST_FAILURE_NO_RETRY', 'compiler_revision': PIN,
               'model_calls': 0, 'engine_builds': 0, 'external_adoption': False,
               'local_lineage_complete_claim': False, 'execution_authorized': False}
    source_paths = [ROOT / 'grant-candidate-selection.workspace.json',
                    ROOT / 'ports/service-policy/candidate_selection.gooo', RUNNER,
                    *[p for p in HERE.rglob('*') if p.is_file() and '__pycache__' not in p.parts]]
    before = {str(p.relative_to(ROOT)): host.ref(p) for p in source_paths}
    host.save(out / 'source-before.json', before)
    try:
        compiler, go = args.compiler.resolve(), args.go.resolve()
        for path in (compiler, go):
            assert path.is_file()
        for name in ('cache', 'tmp', 'modcache', 'gopath', 'home', 'bin'):
            (out / name).mkdir(mode=0o700)
        cache = args.go_cache.resolve() if args.go_cache else out / 'cache'
        modules = args.module_cache.resolve() if args.module_cache else out / 'modcache'
        env = {'PATH': str(go.parent) + ':/usr/bin:/bin', 'GOROOT': str(go.parent.parent),
               'HOME': str(out / 'home'), 'TMPDIR': str(out / 'tmp'),
               'GOCACHE': str(cache), 'GOMODCACHE': str(modules), 'GOPATH': str(out / 'gopath'),
               'GOTOOLCHAIN': 'local', 'GOENV': 'off', 'GOWORK': 'off',
               'GOPROXY': 'https://proxy.golang.org', 'GOSUMDB': 'sum.golang.org',
               'GOTELEMETRY': 'off', 'CGO_ENABLED': '0', 'GOMAXPROCS': '2',
               'GOFLAGS': '-mod=readonly -p=1', 'LANG': 'C', 'LC_ALL': 'C'}
        def run(label, argv, seconds=90, stdin=None, expected_exit=0, environment=None):
            return host.owned(label, argv, environment or env, seconds, ROOT, stdin, expected_exit)
        version = run('go-identity', [go, 'version'], 15).decode().strip()
        assert version in ('go version go1.27.3 darwin/arm64', 'go version go1.27.3 linux/amd64')
        go_identity = dict(host.ref(go), version=version)
        identity = host.strict(run('compiler-identity', [compiler, 'version', '--build', '--json'], 15))
        for key, value in {'compiler_source_sha': PIN, 'vcs_revision': PIN,
                           'source_status': 'CLEAN_VCS', 'vcs_modified': 'false',
                           'go_version': 'go1.27.3', 'native_go_required': 'go1.27.3',
                           'build_info_observed': True}.items():
            host.exact(identity[key], value)
        host.exact(identity['module'], host.ACTIVE_COMPILER_PROFILE['module'])
        host.exact(identity['decision_runtime'], host.ACTIVE_COMPILER_PROFILE['decision_runtime'])
        oracle = run('independent-kernel-reference', [go, '-C', HERE / 'reference', 'run', '.', HERE / 'kernel-inputs.json'])
        host.exact(host.strict(oracle), host.strict((HERE / 'kernel.cases.json').read_bytes()))
        def execute_and_replay(stem, cases):
            manifest = ROOT / 'grant-candidate-selection.workspace.json'
            run(stem + '-execute', [compiler, 'package', 'execute', '--json', '--go', go, '--cases', cases, manifest], 180)
            execution = out / (stem + '-execute') / 'stdout'
            run(stem + '-replay', [compiler, 'package', 'replay', '--json', '--go', go, '--receipt', execution, '--cases', cases, manifest], 180)
            replay = out / (stem + '-replay') / 'stdout'
            return execution, replay, check_receipts(execution, replay, cases, go_identity)
        execution, replay, count = execute_and_replay('kernel', HERE / 'kernel.cases.json')
        assert count == 82
        run('check-generated-kernel', [sys.executable, '-I', '-B', HERE / 'generate.py', '--execution', execution,
                                      '--replay', replay, '--gofmt', go.parent / 'gofmt', '--check'])
        run('verify-pinned-modules', [go, '-C', HERE, 'mod', 'verify'])
        run('module-vet', [go, '-C', HERE, 'vet', './...'])
        run('module-race', [go, '-C', HERE, 'test', '-race', '-count=1', './...'], 180,
            environment=dict(env, CGO_ENABLED='1'))
        binary = out / 'bin/gooo-grant-select'
        run('consumer-build', [go, '-C', HERE, 'build', '-trimpath', '-buildvcs=false', '-o', binary, './cmd/select'])
        build = host.strict(run('consumer-build-info', [go, 'version', '-m', '-json', binary], 15))
        assert build['GoVersion'] == 'go1.27.3'
        assert build['Main']['Path'] == 'github.com/kimjooyoon/gooo-go-ports/examples/grant-candidate-selection'
        assert 'Replace' not in build['Main']
        assert len(build['Deps']) == 1
        dep = build['Deps'][0]
        assert dep['Path'] == 'github.com/kimjooyoon/gooo-go-ports/examples/service-grant-many'
        assert dep['Version'] == 'v0.0.0-20261020182808-7180a4a0f36a' and 'Replace' not in dep
        assert (dep['Path'] + ' ' + dep['Version'] + ' ' + dep['Sum']) in (HERE / 'go.sum').read_text().splitlines()
        settings = {x['Key']: x['Value'] for x in build['Settings']}
        for key, value in {'-trimpath': 'true', 'CGO_ENABLED': '0', 'GOOS': version.split()[-1].split('/')[0],
                           'GOARCH': version.split()[-1].split('/')[1]}.items():
            assert settings[key] == value
        summary['consumer_binary'] = host.ref(binary)
        host.save(out / 'consumer-metadata.json', build)
        runtime = {'PATH': '', 'HOME': str(out / 'home'), 'TMPDIR': str(out / 'tmp'),
                   'LANG': 'C', 'LC_ALL': 'C', 'GOMAXPROCS': '2'}
        sample = (HERE / 'example-input.json').read_bytes()
        columns, selections = [], []
        for name, data in [('proposed', sample), ('refused', sample.replace(b'mcp://*', b'http://*'))]:
            path = out / (name + '.json'); path.write_bytes(data)
            report = host.strict(run(name, [binary], 15, path, environment=runtime))
            assert report['input_sha256'] == 'sha256:' + host.ref(path)['sha256']
            result = report['result']; facts = result['facts']; assessments = result['assessments']
            host.exact(facts, host.strict(data))
            assert len(assessments) == 2
            policy = host.strict((HERE / 'policy-generation.json').read_bytes())
            expected_binding = {'compiler_source_sha': policy['compiler_source_sha'],
                                'generation_sha256': 'sha256:' + host.ref(HERE / 'policy-generation.json')['sha256'],
                                'sources': policy['public_sources'], 'assets': policy['assets']}
            host.exact(result['bindings']['policy'], expected_binding)
            for i, assessment in enumerate(assessments):
                proof = assessment['proof']
                host.exact(proof['facts'], {'before': facts['before'], 'after': facts['candidates'][i]['after'],
                           'required_types': [r['type'] for r in facts['required_requests']],
                           'required_names': [r['name'] for r in facts['required_requests']]})
                host.exact(proof['bindings'], expected_binding)
                assert assessment['candidate_id'] == facts['candidates'][i]['id']
                assert assessment['supplied_preference'] == facts['candidates'][i]['preference']
                # This fixed ASCII corpus needs none of Go JSON's HTML escaping.
                raw = json.dumps(proof, separators=(',', ':'), ensure_ascii=False).encode()
                import hashlib
                assert assessment['proof_sha256'] == 'sha256:' + hashlib.sha256(raw).hexdigest()
            choice = result['selection']; selections.append(choice)
            assert choice['state'] == ('PROPOSED' if name == 'proposed' else 'REFUSED')
            assert choice['execution_authorized'] is False
            if name == 'proposed':
                assert choice['selected_candidate_id'] == 'all-mcp' and choice['selected_index'] == 1 and choice['selected_priority'] == 10
            else:
                assert choice['selected_candidate_id'] == '' and choice['selected_index'] == -1
            columns.append({'phase': 'SELECT', 'context_id': result['context_id'],
                            'candidate_ids': [a['candidate_id'] for a in assessments],
                            'assessment_ids': [a['proof_sha256'] for a in assessments],
                            'context_ids': [result['context_id']] * len(assessments),
                            'statuses': [a['proof']['decision']['status'] for a in assessments],
                            'priorities': [a['supplied_preference'] for a in assessments],
                            'bindings_verified': [True] * len(assessments)})
        wire = out / 'actual-consumer-columns.json'; host.save(wire, columns)
        raw = run('actual-consumer-reference', [go, '-C', HERE / 'reference', 'run', '.', wire])
        cases = out / 'actual-consumer.cases.json'; cases.write_bytes(raw)
        for case, choice in zip(host.strict(raw)['cases'], selections):
            host.exact(case['expected'][ENTRY], choice)
        actual_execution, actual_replay, actual_count = execute_and_replay('actual-consumer', cases)
        assert actual_count == 2
        original_replay = host.strict(actual_replay.read_bytes())
        mutations = []
        changed = copy.deepcopy(original_replay); changed['replayed_from_sha256'] = 'sha256:' + '0' * 64
        mutations.append(('wrong-replay-parent', changed))
        changed = copy.deepcopy(original_replay); changed['result']['runtime']['producer_source_sha'] = '0' * 40
        mutations.append(('wrong-producer-source', changed))
        changed = copy.deepcopy(original_replay); changed['manifest_digest'] = 'sha256:' + '0' * 64
        mutations.append(('wrong-workspace', changed))
        changed = copy.deepcopy(original_replay)
        changed['result']['runtime']['traces'][0]['deliveries'][0]['actual']['selected_assessment_id'] = 'sha256:' + '0' * 64
        mutations.append(('changed-assessment-id', changed))
        for label, changed in mutations:
            path = out / (label + '.json'); host.save(path, changed)
            try:
                check_receipts(actual_execution, path, cases, go_identity)
            except AssertionError:
                pass
            else:
                raise RuntimeError('mutated receipt accepted: ' + label)
        host.save(out / 'receipt-negative-controls.json', {'status': 'ALL_MUTATIONS_REJECTED',
                  'controls': [label for label, _ in mutations],
                  'scope': 'Mutations of fresh genuine replay; not external producer authentication.'})
        bad = [('external-binding-flag', sample.replace(b'"preference": 10', b'"preference": 10, "bindings_verified": true')),
               ('late-invalid-preference', sample.replace(b'"preference": 10', b'"preference": 1.5')),
               ('duplicate-root-key', sample.replace(b'"before": "*"', b'"before": "*", "before": "*"'))]
        for name, data in bad:
            path = out / (name + '.json'); path.write_bytes(data)
            assert run(name, [binary], 15, path, 1, runtime) == b''
            diagnostic = host.strict((out / name / 'stderr').read_bytes())
            assert diagnostic['state'] == 'INVALID_TRANSPORT'
        after = {str(p.relative_to(ROOT)): host.ref(p) for p in source_paths}
        host.exact(after, before); host.save(out / 'source-after.json', after)
        host.check_cancelled()
        summary.update(status='SOURCE_KERNEL82_AND_NATIVE_CONSUMER_REPLAY_PASS', kernel_cases=count,
                       actual_consumer_cases=2, typed_selection_fields=12, transport_refusals=len(bad),
                       receipt_negative_controls=len(mutations),
                       runtime_path_empty=True, runtime_goroot_absent=True,
                       reused_external_cache=bool(args.go_cache or args.module_cache), no_cold_cache_claim=True)
    except BaseException as error:
        first = {'type': type(error).__name__, 'message': str(error), 'traceback': traceback.format_exc()}
        host.save(out / 'first-failure.json', first)
    finally:
        pending = host.pending_cancellation()
        if pending is not None and first is None:
            first = {'type': type(pending).__name__, 'message': str(pending)}
            summary['status'] = 'FIRST_FAILURE_NO_RETRY'
        closed = bool(host.records) and all(r['waited'] and r['streams_EOF'] and r['PG_ESRCH'] for r in host.records)
        summary.update(first_failure=first, records=host.records, all_direct_wait_EOF_PG_ESRCH=closed)
        if first is None and not closed:
            summary['status'] = 'FIRST_FAILURE_NO_RETRY'
            first = {'type': 'RuntimeError', 'message': 'process closure not established'}
            summary['first_failure'] = first
        # Remove only completed owned executables. Shared caches and compilers stay.
        if closed:
            try:
                shutil.rmtree(out / 'bin', ignore_errors=False)
            except BaseException as error:
                diagnostic = {'type': type(error).__name__, 'message': str(error)}
                summary['owned_cleanup_failure'] = diagnostic
                if first is None:
                    first = diagnostic
        pending = host.pending_cancellation()
        if pending is not None and first is None:
            first = {'type': type(pending).__name__, 'message': str(pending)}
        if first is not None:
            summary['status'] = 'FIRST_FAILURE_NO_RETRY'
            summary['first_failure'] = first
        host.save(out / 'summary.json', summary)
    if first is not None:
        raise SystemExit(1)
    host.check_cancelled()
    print(json.dumps({'status': summary['status'], 'summary': host.ref(out / 'summary.json')}))


if __name__ == '__main__':
    with host.cancellation_handlers():
        main()
