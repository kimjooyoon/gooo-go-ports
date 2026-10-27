"""Regenerate the public library from fresh pinned native and own-replay evidence."""
import argparse
import copy
import gzip
import importlib.util
import json
import os
import sys
import traceback
from pathlib import Path

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('sdk_refresh_evidence', HERE/'evidence.py')
e = importlib.util.module_from_spec(spec)
spec.loader.exec_module(e)
h = e.host


def snapshot():
    paths = [e.SOURCE, e.MANIFEST, e.ROOT/'upstream.json', e.ROOT/'examples/service-grant-many/verify.py',
             e.ROOT/'examples/recorded-catalog-comparison/generate.py']
    paths += [p for p in HERE.rglob('*') if p.is_file() and '__pycache__' not in p.parts and p not in e.ASSETS]
    return {str(p.relative_to(e.ROOT)): h.ref(p) for p in sorted(set(paths))}


def main():
    parser = argparse.ArgumentParser()
    for name in ['compiler', 'go', 'go-cache', 'module-cache', 'out']:
        parser.add_argument('--'+name, type=Path, required=True)
    parser.add_argument('--write-generated', action='store_true', help='Initial all-absent bootstrap only; never overwrite assets.')
    args = parser.parse_args()
    out = args.out.resolve()
    assert not out.is_relative_to(e.ROOT.resolve())
    out.mkdir(mode=0o700, exist_ok=False)
    h.R = out
    first = None
    summary = {'schema': 'gooo/sdk-catalog-refresh-verification/v1', 'status': 'FIRST_FAILURE_NO_RETRY',
               'compiler_revision': e.PROFILE['compiler_revision'], 'go_version': e.PROFILE['go_version'],
               'global_compiler_pin_changed': False, 'native_cases': 0, 'replay_cases': 0,
               'model_calls': 0, 'network_calls': 0, 'engine_builds': 0, 'external_adoption': False,
               'initial_bootstrap': args.write_generated, 'generated_write_records': [], 'native_acceptance': False}
    try:
        if args.write_generated:
            assert all(not p.exists() and not p.is_symlink() for p in e.ASSETS)
        before = snapshot(); h.save(out/'source-before.json', before)
        compiler = args.compiler.resolve(); go = args.go.resolve()
        binary_before = h.ref(compiler); go_before = h.ref(go)
        for name in ['home', 'tmp', 'gopath']:
            (out/name).mkdir(mode=0o700)
        env = {'PATH': str(go.parent)+':/usr/bin:/bin', 'GOROOT': str(go.parent.parent),
               'HOME': str(out/'home'), 'TMPDIR': str(out/'tmp'), 'GOPATH': str(out/'gopath'),
               'GOCACHE': str(args.go_cache.resolve()), 'GOMODCACHE': str(args.module_cache.resolve()),
               'GOTOOLCHAIN': 'local', 'GOWORK': 'off', 'GOENV': 'off', 'GOPROXY': 'off', 'GOSUMDB': 'off',
               'GOTELEMETRY': 'off', 'CGO_ENABLED': '0', 'GOMAXPROCS': '2', 'GOFLAGS': '-mod=readonly -p=1',
               'LANG': 'C', 'LC_ALL': 'C'}

        def run(name, argv, seconds=120, expected_exit=0):
            return h.owned(name, argv, env, seconds, e.ROOT, expected_exit=expected_exit)

        identity = h.strict(run('compiler-identity', [compiler, 'version', '--build', '--json'], 15))
        e.identity(identity)
        version = run('go-identity', [go, 'version'], 15).decode().strip()
        assert version in ['go version '+e.PROFILE['go_version']+' '+target for target in ['linux/amd64', 'darwin/arm64']]
        go_identity = dict(go_before, version=version)
        summary.update(compiler=binary_before, compiler_identity=identity, go_identity=go_identity)
        refusals = []
        for label, names, detail in [
            ('rows257', ['a']*257, 'repeated field has 257 values, limit 256'),
            ('name1025', ['a'*1025], 'element 0: Text requires a string of at most 1024 UTF-8 bytes'),
        ]:
            path = out/(label+'.cases.json')
            h.save(path, {'schema': 'gooo/body-composition-cases/v1', 'cases': [{
                'inputs': {e.ENTRY+'.input0': {'names': names}, e.ENTRY+'.input1': {'names': []}},
                'expected': {e.ENTRY: h.strict(e.read(HERE/'row-cases.json'))['cases'][0]['expected'][e.ENTRY]},
            }]})
            run(label, [compiler, 'package', 'execute', '--json', '--receipt-compression', 'gzip',
                        '--go', go, '--cases', path, e.MANIFEST], 90, expected_exit=1)
            receipt = h.strict(e.read(out/label/'stdout'))
            e.same(receipt['decision'], 'FAIL_CLOSED')
            e.same(receipt['manifest_digest'], e.sha(e.read(e.MANIFEST)))
            e.same(receipt['cases_digest'], e.sha(e.read(path)))
            message = 'case 0: input "'+e.ENTRY+'.input0": record "RowNames" field "names": '+detail
            e.same(receipt['input_diagnostic'], {'stage': 'INPUT_VALIDATION', 'code': 'INVALID_INPUT_VALUE',
                                               'case_index': 0, 'key': e.ENTRY+'.input0', 'message': message})
            refusals.append({'label': label, 'receipt': h.ref(out/label/'stdout'), 'native_execution': False})
        h.save(out/'typed-input-refusals.json', refusals)
        common = ['--json', '--receipt-compression', 'gzip', '--go', go, '--cases', HERE/'row-cases.json']
        run('native', [compiler, 'package', 'execute', *common, e.MANIFEST], 180)
        execution = out/'native/stdout'
        run('own-replay', [compiler, 'package', 'replay', '--receipt', execution, *common, e.MANIFEST], 180)
        replay = out/'own-replay/stdout'
        native = e.pair(execution, replay, go_identity)
        controls = []
        original = h.strict(e.read(replay))
        for label in ['parent', 'producer', 'manifest', 'source', 'ordered-inputs', 'field-id', 'actual', 'count']:
            bad = copy.deepcopy(original)
            runtime = bad['result']['runtime']; delivery = runtime['traces'][0]['deliveries'][0]
            if label == 'parent': bad['replayed_from_sha256'] = 'sha256:'+'0'*64
            elif label == 'producer': runtime['producer_source_sha'] = '0'*40
            elif label == 'manifest': bad['manifest_digest'] = 'sha256:'+'0'*64
            elif label == 'source': bad['result']['program']['activity_origins'][0]['input_source']['source_sha256'] = 'sha256:'+'0'*64
            elif label == 'ordered-inputs': delivery['inputs'].reverse()
            elif label == 'field-id': delivery['actual_fields'][0]['id'] = e.PREFIX+'wrong'
            elif label == 'actual': delivery['actual']['mapping_equal'] = False
            else: runtime['finite_passed'] = 7
            path = out/('negative-'+label+'.json.gz')
            path.write_bytes(gzip.compress(json.dumps(bad, separators=(',', ':')).encode(), mtime=0))
            try: e.pair(execution, path, go_identity)
            except (AssertionError, ValueError): pass
            else: raise AssertionError('receipt mutation accepted: '+label)
            controls.append({'label': label, 'refused': True, 'receipt': h.ref(path)})
        h.save(out/'negative-controls.json', controls)
        generated = out/'generated'
        run('generate', [compiler, 'package', 'generate', '--go-package', 'rowvisibility', '--embed-origin', '--out', generated, e.MANIFEST])
        assert {p.name for p in generated.iterdir()} == {'semantic.gooo.go'}
        raw = e.read(generated/'semantic.gooo.go')
        origin = e.generated(raw, native)
        metadata = {'schema': 'gooo/sdk-catalog-row-generation/v1', 'profile': h.strict(e.read(HERE/'profile.json')),
                    'source_sha256': e.sha(e.read(e.SOURCE)), 'manifest_sha256': e.sha(e.read(e.MANIFEST)),
                    'literal_cases_sha256': e.sha(e.read(HERE/'row-cases.json')), 'literal_native_cases': 8,
                    'library_sha256': e.sha(raw), 'origin': origin,
                    'scope': 'Actual compiler-owned library from this public manifest. Native8 and own replay8 verified separately; finite observations, not authorization or whole-engine compatibility.'}
        payloads = [raw, (json.dumps(metadata, indent=2, sort_keys=True)+'\n').encode()]
        e.same(before, snapshot())
        for path, payload in zip(e.ASSETS, payloads):
            if args.write_generated:
                h.check_cancelled()
                assert not path.exists() and not path.is_symlink()
                path.parent.mkdir(parents=True, exist_ok=True)
                record = {'path': str(path.relative_to(e.ROOT)), 'completed': False}
                summary['generated_write_records'].append(record)
                with path.open('xb') as dest:
                    dest.write(payload); dest.flush(); os.fsync(dest.fileno())
                record['completed'] = True
            assert e.read(path) == payload, str(path)
        after = snapshot(); e.same(before, after); h.save(out/'source-after.json', after)
        e.same(h.ref(compiler), binary_before); e.same(h.ref(go), go_before)
        summary.update(status='FINITE_SDK_ROW_NATIVE_OWN_REPLAY_AND_LIBRARY_PASS', native_cases=8, replay_cases=8,
                       typed_input_refusals=2,
                       ordered_inputs_verified=True, generated_origin_verified=True, generated_assets_exact=True,
                       receipt_negative_controls=8, source_unchanged=True, native_acceptance=True,
                       generated_assets={str(p.relative_to(e.ROOT)): h.ref(p) for p in e.ASSETS})
    except BaseException as error:
        first = {'type': type(error).__name__, 'message': str(error), 'traceback': traceback.format_exc()}
        h.save(out/'first-failure.json', first)
    finally:
        pending = h.pending_cancellation()
        if pending is not None and first is None:
            first = {'type': type(pending).__name__, 'message': str(pending)}
        closed = bool(h.records) and all(r['waited'] and r['streams_EOF'] and r['PG_ESRCH'] for r in h.records)
        if first is None and not closed:
            first = {'type': 'RuntimeError', 'message': 'owned process closure not established'}
        if first is not None:
            summary['status'] = 'FIRST_FAILURE_NO_RETRY'
            summary['native_acceptance'] = False
        summary.update(first_failure=first, all_direct_wait_EOF_PG_ESRCH=closed, processes=h.records)
        h.save(out/'summary.json', summary)
    if first is not None: raise SystemExit(1)
    print(json.dumps({'status': summary['status'], 'summary': h.ref(out/'summary.json')}))


if __name__ == '__main__':
    with h.cancellation_handlers():
        main()
