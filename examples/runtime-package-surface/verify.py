"""Fresh source-bound fourteen-case native/replay and declaration-mutation checks."""
import argparse
import copy
import gzip
import importlib.util
import json
import os
import shutil
import stat
import sys
import traceback
from pathlib import Path

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('surface_e', HERE/'evidence.py')
e = importlib.util.module_from_spec(spec)
spec.loader.exec_module(e)
spec = importlib.util.spec_from_file_location('surface_g', HERE/'generate.py')
g = importlib.util.module_from_spec(spec)
spec.loader.exec_module(g)
host, ROOT = e.host, e.ROOT


def snapshot(include_generated):
    paths = [ROOT/'upstream.json', ROOT/'go.mod', ROOT/'go.sum', e.MANIFEST,
             *(ROOT/name for name in e.SOURCES), *g.REFERENCES, *g.HELPERS,
             Path(__file__), ROOT/'internal/runtimepackagesurface/evaluate_test.go']
    if include_generated:
        paths += [ROOT/name for name in sorted(g.ASSETS)]
    result = {}
    for path in sorted(set(paths)):
        assert not path.is_symlink() and stat.S_ISREG(path.stat().st_mode)
        result[str(path.relative_to(ROOT))] = dict(host.ref(path), mode=stat.S_IMODE(path.stat().st_mode))
    return result


def provenance(generated, index, gofmt, used):
    document = host.strict(e.read(generated/'generation.provenance.jsonld', 2<<20))
    assert document['entity_identity_profile'] == g.PROFILE
    graph = document['@graph']
    by_id = {node['@id']: node for node in graph}
    assert len(by_id) == len(graph)
    activities = [node for node in graph if node['@type'] == 'prov:Activity']
    assert len(activities) == 1
    activity = activities[0]
    nodes = {(node['role'], node['path']): node for node in graph if node['@type'] == 'prov:Entity'}
    assert len(nodes) == len(graph)-1
    def check(role, path, digest):
        node = nodes[(role, path)]
        raw = json.dumps([g.PROFILE, activity['@id'], role, path, digest], ensure_ascii=True, separators=(',', ':')).encode()
        assert node['@id'] == 'urn:gooo:runtime-package-surface:entity:'+e.sha(raw).removeprefix('sha256:')
        assert node['sha256'] == digest
        return node
    paths = g.inputs(index, gofmt, used)
    index_value = host.strict(e.read(index))
    receipts = {e.shared.local_member(index.parent, index_value[key]) for key in ('execution', 'replay')}
    parents = []
    for path in paths:
        name = str(path.relative_to(ROOT)) if path.is_relative_to(ROOT) else str(path)
        node = check('input', name, e.sha(e.read(path, 64<<20)))
        assert not any(key in node for key in ('prov:wasDerivedFrom', 'prov:wasGeneratedBy', 'prov:used'))
        if path in receipts:
            assert node['original_JSON_sha256'] == e.sha(e.receipt(path)[0])
        parents.append({'@id': node['@id']})
    pin = check('selected-pins', 'upstream.json#consumed-runtime-package-surface-pins', e.sha(g.PINS))
    assert pin['canonicalJSON'].encode() == g.PINS
    e.same(pin['jsonPointers'], ['/compiler_revision', '/go_version'])
    e.same(pin['prov:wasDerivedFrom'], [parents[0]])
    selected = {'@id': pin['@id']}
    e.same(activity['prov:used'], parents+[selected])
    assert len(nodes) == len(paths)+1+len(g.ASSETS)
    for name in g.ASSETS:
        node = check('output', name, e.sha(e.read(generated/name)))
        e.same(node['prov:wasGeneratedBy'], {'@id': activity['@id']})
        e.same(node['prov:wasDerivedFrom'], [selected, *parents[1:]])


def negative_receipts(out, execution, replay, cases, go_identity):
    _, original = e.receipt(replay)
    controls = []
    for label in ('wrong-parent', 'wrong-source', 'wrong-input', 'wrong-field-ID', 'wrong-value-type', 'input-only'):
        bad = copy.deepcopy(original)
        delivery = bad['result']['runtime']['traces'][0]['deliveries'][0]
        if label == 'wrong-parent':
            bad['replayed_from_sha256'] = 'sha256:'+'0'*64
        elif label == 'wrong-source':
            bad['result']['program']['activity_origins'][0]['input_source']['source_sha256'] = 'sha256:'+'0'*64
        elif label == 'wrong-input':
            delivery['input']['origin_ref'] = 'tampered'
        elif label == 'wrong-field-ID':
            delivery['actual_fields'][0]['id'] = 'urn:wrong-field'
        elif label == 'wrong-value-type':
            delivery['actual']['missing_count'] = False
            for field in delivery['actual_fields']:
                if field['name'] == 'missing_count':
                    field['value'] = False
        else:
            bad['decision'] = 'OBSERVED'
            bad['result']['runtime']['finite_total'] = 0
        path = out/(label+'.json.gz')
        path.write_bytes(gzip.compress(json.dumps(bad, separators=(',', ':')).encode(), mtime=0))
        try:
            e.verify_receipt(path, cases, go_identity, parent=execution)
        except (AssertionError, ValueError):
            controls.append({'label': label, 'receipt': host.ref(path), 'reader_refused': True})
        else:
            raise AssertionError('modified receipt accepted: '+label)
    host.save(out/'receipt-negative-controls.json', controls)
    return len(controls)


def main():
    parser = argparse.ArgumentParser()
    for name in ('compiler', 'go', 'out'):
        parser.add_argument('--'+name, type=Path, required=True)
    for name in ('go-cache', 'module-cache'):
        parser.add_argument('--'+name, type=Path)
    parser.add_argument('--bootstrap', action='store_true')
    args = parser.parse_args()
    out = args.out.resolve()
    assert not out.is_relative_to(ROOT.resolve())
    out.mkdir(parents=True, exist_ok=False)
    host.R = out
    first = None
    summary = {'schema': 'gooo/runtime-package-surface-verification/v1', 'status': 'FIRST_FAILURE_NO_RETRY',
               'compiler_revision': e.PIN, 'go_version': e.GO_VERSION, 'model_calls': 0,
               'engine_builds': 0, 'network_calls': 0, 'bootstrap': args.bootstrap,
               'synthetic_observations_only': True, 'external_adoption': False}
    floor = 5*(1<<30)
    minimum = os.statvfs(out).f_bavail*os.statvfs(out).f_frsize
    original_usage = host.usage
    def usage():
        nonlocal minimum
        space = os.statvfs(out)
        minimum = min(minimum, space.f_bavail*space.f_frsize)
        if minimum < floor:
            raise RuntimeError('FREE_BELOW_5GIB')
        return original_usage()
    host.usage = usage
    try:
        usage()
        before = snapshot(not args.bootstrap)
        host.save(out/'source-before.json', before)
        e.validate_upstream()
        e.authored_contract()
        if args.bootstrap:
            assert all(not (ROOT/name).exists() for name in g.ASSETS), 'bootstrap never overwrites generated assets'
        facts = host.strict(e.read(ROOT/'upstream.json'))
        assert facts['compiler_revision'] == e.PIN and facts['go_version'] == e.GO_VERSION
        compiler, go = args.compiler.resolve(), args.go.resolve()
        gofmt = go.parent/'gofmt'
        assert compiler.is_file() and go.is_file() and gofmt.is_file()
        for name in ('cache', 'tmp', 'modcache', 'gopath', 'home', 'bin'):
            (out/name).mkdir(mode=0o700)
        cache = args.go_cache.resolve() if args.go_cache else out/'cache'
        modules = args.module_cache.resolve() if args.module_cache else out/'modcache'
        assert not cache.is_relative_to(ROOT) and not modules.is_relative_to(ROOT)
        env = {'PATH': str(go.parent)+':/usr/bin:/bin', 'GOROOT': str(go.parent.parent), 'HOME': str(out/'home'),
               'TMPDIR': str(out/'tmp'), 'GOCACHE': str(cache), 'GOMODCACHE': str(modules), 'GOPATH': str(out/'gopath'),
               'GOTOOLCHAIN': 'local', 'GOENV': 'off', 'GOWORK': 'off', 'GOPROXY': 'off', 'GOSUMDB': 'off',
               'GOTELEMETRY': 'off', 'CGO_ENABLED': '0', 'GOMAXPROCS': '1', 'GOFLAGS': '-mod=readonly -p=1', 'LANG': 'C', 'LC_ALL': 'C'}
        host.save(out/'environment.json', env)
        def run(label, argv, seconds=120, environment=None):
            usage()
            return host.owned(label, argv, environment or env, seconds, ROOT)
        version = run('go-identity', [go, 'version'], 15).decode().strip()
        assert version in ('go version '+e.GO_VERSION+' darwin/arm64', 'go version '+e.GO_VERSION+' linux/amd64')
        go_identity = dict(host.ref(go), version=version)
        identity = host.strict(run('compiler-identity', [compiler, 'version', '--build', '--json'], 15))
        for key, value in {'compiler_source_sha': e.PIN, 'vcs_revision': e.PIN, 'source_status': 'CLEAN_VCS',
                           'vcs_modified': 'false', 'go_version': e.GO_VERSION, 'native_go_required': e.GO_VERSION,
                           'build_info_observed': True, 'version': facts['compiler_version']}.items():
            e.same(identity[key], value)
        e.same(identity['module'], host.ACTIVE_COMPILER_PROFILE['module'])
        e.same(identity['decision_runtime'], host.ACTIVE_COMPILER_PROFILE['decision_runtime'])
        summary.update(compiler=host.ref(compiler), compiler_identity=identity, go_identity=go_identity)
        cases = out/'cases.json'
        cases.write_bytes(e.read(e.CASES))
        run('native-execute', [compiler, 'package', 'execute', '--json', '--receipt-compression', 'gzip', '--go', go, '--cases', cases, e.MANIFEST])
        execution = out/'native-execute/stdout'
        composition = e.verify_receipt(execution, cases, go_identity)
        run('own-replay', [compiler, 'package', 'replay', '--json', '--receipt-compression', 'gzip', '--go', go, '--receipt', execution, '--cases', cases, e.MANIFEST])
        replay = out/'own-replay/stdout'
        e.verify_receipt(replay, cases, go_identity, parent=execution)
        mutations = []
        needle = b'    module + "/internal/snapshotkernel",\n'
        source = e.read(ROOT/e.SOURCES[1])
        assert source.count(needle) == 1
        for label, replacement in [('unknown-declaration', b'    module + "/internal/unreviewed",\n'),
                                   ('duplicate-declaration', b'    module + "/internal/kernel",\n')]:
            directory = out/label
            for name in e.SOURCES:
                target = directory/name
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_bytes(e.read(ROOT/name))
            (directory/e.SOURCES[1]).write_bytes(source.replace(needle, replacement))
            manifest = directory/e.MANIFEST.name
            manifest.write_bytes(e.read(e.MANIFEST))
            run(label+'-execute', [compiler, 'package', 'execute', '--json', '--receipt-compression', 'gzip', '--go', go, '--cases', cases, manifest])
            path = out/(label+'-execute')/'stdout'
            mutant = e.verify_receipt(path, cases, go_identity, source_root=directory, manifest=manifest, decision='PROGRESS', passed=0)
            assert mutant['generated_sha256'] != composition['generated_sha256']
            _, value = e.receipt(path)
            actuals = [t['deliveries'][0]['actual'] for t in value['result']['runtime']['traces']]
            if label == 'duplicate-declaration':
                assert all(a['status'] == 'INVALID_DECLARED_SURFACE' for a in actuals)
            else:
                assert actuals[0]['status'] == 'MISSING_AND_UNDECLARED'
            mutations.append({'label': label, 'source': host.ref(directory/e.SOURCES[1]), 'receipt': host.ref(path), 'decision': 'PROGRESS', 'finite_passed': 0, 'finite_total': 14})
        host.save(out/'source-mutations.json', mutations)
        controls = negative_receipts(out, execution, replay, cases, go_identity)
        index = out/'generation-input.json'
        host.save(index, {'schema': 'gooo/runtime-package-surface-generation-input/v1', 'cases': 'cases.json', 'execution': 'native-execute/stdout', 'replay': 'own-replay/stdout'})
        _, used = e.verify_index(index, go_identity)
        generated = out/'generated'
        run('source-bound-generation', [sys.executable, '-I', '-B', HERE/'generate.py', '--index', index, '--gofmt', gofmt, '--out', generated])
        assert {str(p.relative_to(generated)) for p in generated.rglob('*') if p.is_file()} == g.ASSETS|{'generation.provenance.jsonld'}
        provenance(generated, index, gofmt, used)
        for name in sorted(g.ASSETS):
            raw = e.read(generated/name, 4<<20)
            if args.bootstrap:
                target = ROOT/name
                target.parent.mkdir(parents=True, exist_ok=True)
                with target.open('xb') as stream:
                    stream.write(raw)
            assert e.read(ROOT/name, 4<<20) == raw, name
        run('exact-generated-check', [sys.executable, '-I', '-B', HERE/'generate.py', '--index', index, '--gofmt', gofmt, '--check'])
        formatted = run('gofmt-check', [gofmt, '-l', ROOT/'internal/runtimepackagesurface'], 15)
        assert formatted == b'', formatted
        run('generated-api-vet', [go, 'vet', './internal/runtimepackagesurface'])
        run('independent-frozen14-api-race', [go, 'test', '-race', './internal/runtimepackagesurface', '-count=1'], environment=dict(env, CGO_ENABLED='1'))
        e.same(before, snapshot(not args.bootstrap))
        host.save(out/'source-after.json', snapshot(not args.bootstrap))
        e.same(host.ref(compiler), summary['compiler'])
        e.same(host.ref(go), {k: go_identity[k] for k in ('path', 'bytes', 'sha256')})
        summary.update(status='RUNTIME_SURFACE14_NATIVE_REPLAY_AND_TWO_MUTATIONS_PASS', native_cases=14, replay_cases=14,
                       source_mutations=2, rejected_mutation_cases=28, receipt_negative_controls=controls,
                       generated_assets=4, generated_assets_exact=True, PROV_O_bindings_verified=True,
                       source_unchanged=True, source_files=len(before), reused_external_cache=bool(args.go_cache or args.module_cache),
                       no_cold_cache_claim=True, independent_frozen_fixture_api_race=True)
    except BaseException as error:
        first = {'type': type(error).__name__, 'message': str(error), 'traceback': traceback.format_exc()}
        host.save(out/'first-failure.json', first)
    finally:
        pending = host.pending_cancellation()
        if pending is not None and first is None:
            first = {'type': type(pending).__name__, 'message': str(pending)}
        closed = bool(host.records) and all(r['waited'] and r['streams_EOF'] and r['PG_ESRCH'] for r in host.records)
        if first is None and not closed:
            first = {'type': 'RuntimeError', 'message': 'process closure not established'}
        if closed:
            shutil.rmtree(out/'bin', ignore_errors=True)
        if first is not None:
            summary['status'] = 'FIRST_FAILURE_NO_RETRY'
        summary.update(first_failure=first, records=host.records, all_direct_wait_EOF_PG_ESRCH=closed,
                       minimum_sampled_free_bytes=minimum, free_floor_bytes=floor,
                       owned_cache_limit_bytes=host.LIMIT, process_stream_limit_bytes=host.CAP,
                       sampled_not_hard_quota=True)
        host.save(out/'summary.json', summary)
    if first is not None:
        raise SystemExit(1)
    host.check_cancelled()
    print(json.dumps({'status': summary['status'], 'summary': host.ref(out/'summary.json')}))


if __name__ == '__main__':
    with host.cancellation_handlers():
        main()
