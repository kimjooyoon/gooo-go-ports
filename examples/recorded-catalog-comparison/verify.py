"""Finite source/native equivalence and seven synthetic saved-catalog consumers.

Independent state expectations come from the committed consumer corpus. Full
consumer reports are compared with fresh native execution of their projected
inputs; that equivalence check is not another independent policy oracle. No
live collection, producer authentication or execution permission is established.
"""
import argparse
import copy
import gzip
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import stat
import sys
import traceback
import zlib

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
RUNNER = ROOT / 'examples/service-grant-many/verify.py'
spec = importlib.util.spec_from_file_location('recorded_catalog_owned', RUNNER)
host = importlib.util.module_from_spec(spec)
spec.loader.exec_module(host)
PIN = '205896a413e5dea92366309c35ae85e542d055f6'
GO_VERSION = 'go1.27.3'
MODULE = 'github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison'
ENTRY = 'gooo/tool-origin/recorded-catalog:CompareRecordedCatalog'
PREFIX = 'gooo://tool-origin/recorded-catalog/v1/'
CORE = 'gooo://tool-origin/v1/'
MANIFEST = ROOT / 'recorded-catalog-comparison.workspace.json'
SOURCES = {'gooo/tool-origin': 'ports/tool-origin/alias_origin.gooo',
           'gooo/tool-origin/recorded-catalog': 'ports/tool-origin/recorded_catalog.gooo'}
GENERATED = {'internal/kernel/projection_generated.go',
             'internal/kernel/types_generated.go',
             'internal/kernel/bridge_generated.go', 'generation.json', 'go.mod'}
EXCLUDED = {'.git', '__pycache__', '.cache', 'cache', 'tmp', 'out', 'output',
            'bin', 'gopath', 'modcache', 'home', 'node_modules'}


def sha(raw):
    return 'sha256:' + hashlib.sha256(raw).hexdigest()


def go_json(value):
    # Matches encoding/json for the typed string/bool/integer fixtures here.
    # Embedded saved frames remain strings; this is not RFC 8785.
    return (json.dumps(value, ensure_ascii=False, separators=(',', ':'), allow_nan=False)
            .replace('<', '\\u003c').replace('>', '\\u003e').replace('&', '\\u0026')
            .replace('\u2028', '\\u2028').replace('\u2029', '\\u2029').encode())


def bounded(path, limit):
    assert not path.is_symlink() and path.is_file(), str(path)
    with path.open('rb') as source:
        data = source.read(limit + 1)
    assert len(data) <= limit, str(path)
    return data


def receipt(path):
    wire = bounded(path, 2 << 20)
    stream = zlib.decompressobj(16 + zlib.MAX_WBITS)
    raw = stream.decompress(wire, (32 << 20) + 1)
    assert len(raw) <= 32 << 20 and stream.eof
    assert not stream.unused_data and not stream.unconsumed_tail
    return raw, host.strict(raw)


def source_snapshot():
    paths = [ROOT / 'upstream.json', MANIFEST, RUNNER,
             *[ROOT / name for name in SOURCES.values()]]
    for directory, dirs, files in os.walk(HERE, followlinks=False):
        parent = Path(directory)
        for name in dirs:
            assert not (parent / name).is_symlink(), str(parent / name)
        dirs[:] = sorted(name for name in dirs if name not in EXCLUDED)
        paths.extend(parent / name for name in sorted(files) if not name.endswith('.pyc'))
    result = {}
    for path in sorted(set(paths)):
        assert not path.is_symlink() and stat.S_ISREG(path.stat().st_mode)
        result[str(path.relative_to(ROOT))] = dict(host.ref(path), mode=stat.S_IMODE(path.stat().st_mode))
    required = {'verify.py', 'generate.py', 'kernel.inputs.json', 'kernel.cases.json',
                'consumer-cases.json', 'compare.go', 'compare_test.go', 'transport.go',
                'transport_test.go', 'model.go', 'cmd/compare/main.go', *GENERATED}
    assert all(str(HERE.relative_to(ROOT) / name) in result for name in required)
    return result


def verify_pair(execution, replay, cases_path, go_identity):
    cases_raw = bounded(cases_path, 2 << 20)
    corpus = host.strict(cases_raw)
    assert corpus['schema'] == 'gooo/body-composition-cases/v1'
    cases = corpus['cases']
    original = None
    for path in (execution, replay):
        raw, document = receipt(path)
        runtime = document['result']['runtime']
        assert document['decision'] == 'PASS'
        assert document['manifest_digest'] == sha(MANIFEST.read_bytes())
        assert document['cases_digest'] == sha(cases_raw)
        assert runtime['stage'] == 'COMPLETE' and runtime['producer_source_sha'] == PIN
        assert runtime['go_version'] == go_identity['version']
        assert runtime['go_tool_sha256'] == 'sha256:' + go_identity['sha256']
        host.exact(runtime['producer_module'], host.ACTIVE_COMPILER_PROFILE['module'])
        assert runtime['model_calls'] == 0
        assert runtime['finite_total'] == runtime['finite_passed'] == len(cases)
        assert runtime['projection_replayed'] is True and runtime['runtime_replayed'] is True
        assert len(runtime['traces']) == len(cases)
        program = document['result']['program']
        packages = program['workspace_image']['packages']
        assert [p['path'] for p in packages] == list(SOURCES)
        for package in packages:
            name = SOURCES[package['path']]
            assert len(package['sources']) == 1
            source = package['sources'][0]
            assert source['filename'] == name
            assert source['source_digest'] == sha(go_json((ROOT / name).read_text()))
        expected_origins = {CORE + 'compare': SOURCES['gooo/tool-origin'],
                            PREFIX + 'compare': SOURCES['gooo/tool-origin/recorded-catalog']}
        origins = program['activity_origins']
        assert len(origins) == len(expected_origins)
        assert {o['lowered_activity_id'] for o in origins} == set(expected_origins)
        for origin in origins:
            source = origin['input_source']
            name = expected_origins[origin['lowered_activity_id']]
            assert source['status'] == 'MAPPED' and source['filename'] == name
            assert source['source_sha256'] == sha((ROOT / name).read_bytes())
        plan = document['result']['composition']['plan']
        assert len(plan['activities']) == 1 and plan['activities'][0]['id'] == PREFIX + 'compare'
        records = {r['id']: r for r in plan['record_types']}
        assert len(records) == len(plan['record_types']) == 4
        assert set(records) == {CORE + 'input', CORE + 'result', PREFIX + 'input', PREFIX + 'result'}
        for index, (trace, case) in enumerate(zip(runtime['traces'], cases)):
            assert trace['case_index'] == index and len(trace['deliveries']) == 1
            delivery = trace['deliveries'][0]
            assert delivery['activity_id'] == PREFIX + 'compare' and delivery['passed'] is True
            assert set(case['inputs']) == {ENTRY + '.input0', ENTRY + '.input1'}
            assert len(delivery['inputs']) == 2
            for port, entity, item in zip(('input0', 'input1'), (CORE + 'input', PREFIX + 'input'), delivery['inputs']):
                assert item['port'] == port and item['entity_id'] == entity
                host.exact(item['value'], case['inputs'][ENTRY + '.' + port])
                fields = records[entity]['fields']
                host.exact(item['fields'], [{'id': f['id'], 'name': f['name'], 'value': item['value'][f['name']]} for f in fields])
            actual = delivery['actual']
            host.exact(actual, case['expected'][ENTRY])
            host.exact(delivery['expected'], actual)
            fields = records[PREFIX + 'result']['fields']
            assert len(fields) == len(actual) == 32 and set(actual) == {f['name'] for f in fields}
            host.exact(delivery['actual_fields'], [{'id': f['id'], 'name': f['name'], 'value': actual[f['name']]} for f in fields])
            assert actual['execution_authorized'] is actual['origin_execution_authorized'] is actual['live_removal_proven'] is False
            assert actual['remote_atomicity'] == 'UNKNOWN'
        if original is None:
            original = (raw, document)
        else:
            assert document['replayed_from_sha256'] == sha(original[0])
            host.exact(document['result']['composition'], original[1]['result']['composition'])
    return len(cases), original[1]['result']['composition']['source']


def cursor(container, key):
    if key not in container:
        return {'tag': 'ABSENT', 'value': ''}
    if container[key] is None:
        return {'tag': 'NULL', 'value': ''}
    assert type(container[key]) is str
    return {'tag': 'STRING', 'value': container[key]}


def verify_consumer(report, exported, fixture, raw):
    facts = exported['input']
    host.exact(facts, fixture['input'])
    host.exact(report['facts'], facts)
    assert exported['label'] == fixture['label']
    for key in ('expected_alias_state', 'expected_collection_state'):
        host.exact(exported[key], fixture[key])
    wanted = copy.deepcopy(exported['report'])
    wanted['input_sha256'] = sha(raw)
    host.exact(report, wanted)
    facts_hash = sha(go_json(facts))
    assert report['input_sha256'] == sha(raw) and report['facts_sha256'] == facts_hash
    projection = report['projection']
    expected_facts = {'target_alias': facts['target_alias'], 'before': [], 'after': []}
    for side in ('before', 'after'):
        pages, rows = [], []
        for ci, capture in enumerate(facts[side]['captures']):
            assert capture['evidence_kind'] == 'SYNTHETIC_FIXTURE'
            for pi, page in enumerate(capture['pages']):
                request = host.strict(page['request_json'])
                response = host.strict(page['response_json'])
                host.exact(request['id'], response['id'])
                response_hash = sha(page['response_json'].encode())
                pages.append({'capture_index': ci, 'page_index': pi,
                              'request_sha256': sha(page['request_json'].encode()),
                              'response_sha256': response_hash,
                              'request_id': request['id'], 'response_id': response['id'],
                              'request_cursor': cursor(request.get('params', {}), 'cursor'),
                              'next_cursor': cursor(response['result'], 'nextCursor')})
                tools = response['result']['tools']
                assert len(tools) == len(page['declared_exposed_names'])
                origin_tuple = [capture['host_id'], capture['connection_id'], page['context']['backend_binding_revision']]
                origin_id = 'gooo/host-origin/v1/' + sha(go_json(origin_tuple))
                for ti, (tool, exposed) in enumerate(zip(tools, page['declared_exposed_names'])):
                    expected_facts[side].append({'server': origin_id, 'original_name': tool['name'], 'exposed_name': exposed})
                    rows.append({'capture_index': ci, 'page_index': pi, 'tool_index': ti,
                                 'response_sha256': response_hash, 'origin_tuple': origin_tuple, 'origin_id': origin_id})
        host.exact(projection[side + '_pages'], pages)
        host.exact(projection[side + '_rows'], rows)
    host.exact(projection['facts'], expected_facts)
    alias_hash = sha(go_json(expected_facts))
    assert projection['facts_sha256'] == alias_hash
    alias, collection = report['alias_decision'], report['collection_context']
    assert alias['state'] == fixture['expected_alias_state']
    assert collection['state'] == fixture['expected_collection_state']
    assert alias['context_id'] == collection['alias_context_id'] == 'gooo/alias-origin/v1/' + alias_hash
    assert collection['context_id'] == 'gooo/recorded-catalog/v1/' + sha(go_json([facts_hash, alias_hash]))
    assert alias['execution_authorized'] is collection['execution_authorized'] is collection['live_removal_proven'] is False
    assert collection['remote_atomicity'] == 'UNKNOWN' and collection['contains_synthetic_fixture'] is True
    host.exact(report['bindings'], host.strict((HERE / 'generation.json').read_bytes()))
    # The bridge restores this duplicate link from origin_context_id. The raw
    # compiler record has exactly 32 fields and omits collection.alias_context_id.
    flat = {'origin_' + key: value for key, value in alias.items()}
    flat.update({key: value for key, value in collection.items() if key != 'alias_context_id'})
    assert len(flat) == 32
    return {'inputs': {ENTRY + '.input0': exported['origin'], ENTRY + '.input1': exported['collection']},
            'expected': {ENTRY: flat}}


def verify_provenance(generated, execution, replay, go):
    proof = host.strict(bounded(generated / 'generation.provenance.jsonld', 256 << 10))
    graph = proof['@graph']
    activities = [node for node in graph if node['@type'] == 'prov:Activity']
    entities = [node for node in graph if node['@type'] == 'prov:Entity']
    assert len(activities) == 1 and len(entities) + 1 == len(graph)
    by_path = {node['path']: node for node in entities}
    assert len(by_path) == len(entities)
    used = [ROOT / 'upstream.json', MANIFEST, *[ROOT / name for name in SOURCES.values()],
            HERE / 'kernel.cases.json', HERE / 'generate.py', execution, replay,
            go.parent / 'gofmt', go]
    dependencies = []
    for path in used:
        name = str(path.relative_to(ROOT)) if path.is_relative_to(ROOT) else str(path)
        node = by_path[name]
        digest = 'sha256:' + host.ref(path)['sha256']
        assert node['sha256'] == digest and node['@id'] == 'urn:' + digest
        if path in (execution, replay):
            assert node['original_JSON_sha256'] == sha(receipt(path)[0])
        dependencies.append({'@id': node['@id']})
    pin_bytes = json.dumps({'compiler_revision': PIN, 'go_version': GO_VERSION}, sort_keys=True, separators=(',', ':')).encode()
    pin = by_path['upstream.json#consumed-generator-pins']
    assert pin['sha256'] == sha(pin_bytes) and pin['@id'] == 'urn:' + sha(pin_bytes)
    assert pin['canonicalJSON'].encode() == pin_bytes
    host.exact(pin['jsonPointers'], ['/compiler_revision', '/go_version'])
    host.exact(pin['prov:wasDerivedFrom'], [dependencies[0]])
    pin_ref = {'@id': pin['@id']}
    host.exact(activities[0]['prov:used'], dependencies + [pin_ref])
    assert len(entities) == len(used) + 1 + len(GENERATED)
    for name in GENERATED:
        node = by_path[name]
        digest = 'sha256:' + host.ref(generated / name)['sha256']
        assert node['sha256'] == digest and node['@id'] == 'urn:' + digest
        host.exact(node['prov:wasGeneratedBy'], {'@id': activities[0]['@id']})
        expected = [pin_ref, dependencies[5]] if name == 'go.mod' else [pin_ref, *dependencies[1:]]
        host.exact(node['prov:wasDerivedFrom'], expected)


def main():
    parser = argparse.ArgumentParser()
    for name in ('compiler', 'go', 'out'):
        parser.add_argument('--' + name, type=Path, required=True)
    parser.add_argument('--go-cache', type=Path)
    parser.add_argument('--module-cache', type=Path)
    args = parser.parse_args()
    out = args.out.resolve()
    assert not out.is_relative_to(ROOT.resolve()), 'output must be fresh and outside the checkout'
    out.mkdir(parents=True, exist_ok=False)
    host.R = out
    first = None
    summary = {'schema': 'gooo/recorded-catalog-verification/v1',
               'status': 'FIRST_FAILURE_NO_RETRY', 'compiler_revision': PIN, 'go_version': GO_VERSION,
               'model_calls': 0, 'external_adoption': False, 'live_capture_executed': False,
               'synthetic_consumer_observations_only': True, 'native_acceptance': False,
               'execution_authorized': False,
               'consumer_equivalence_scope': 'Full reports against fresh native execution; independent public state literals are separate.'}
    try:
        before = source_snapshot()
        host.save(out / 'source-before.json', before)
        facts = host.strict((ROOT / 'upstream.json').read_bytes())
        profile = host.ACTIVE_COMPILER_PROFILE
        assert facts['compiler_revision'] == profile['compiler_revision'] == PIN
        assert facts['go_version'] == profile['go_version'] == GO_VERSION
        compiler, go = args.compiler.resolve(), args.go.resolve()
        assert compiler.is_file() and go.is_file()
        for name in ('cache', 'tmp', 'modcache', 'gopath', 'home', 'bin'):
            (out / name).mkdir(mode=0o700)
        cache = args.go_cache.resolve() if args.go_cache else out / 'cache'
        modules = args.module_cache.resolve() if args.module_cache else out / 'modcache'
        assert not cache.is_relative_to(ROOT) and not modules.is_relative_to(ROOT)
        env = {'PATH': str(go.parent) + ':/usr/bin:/bin', 'GOROOT': str(go.parent.parent),
               'HOME': str(out / 'home'), 'TMPDIR': str(out / 'tmp'), 'GOCACHE': str(cache),
               'GOMODCACHE': str(modules), 'GOPATH': str(out / 'gopath'), 'GOTOOLCHAIN': 'local',
               'GOENV': 'off', 'GOWORK': 'off', 'GOPROXY': 'off', 'GOSUMDB': 'off',
               'GOTELEMETRY': 'off', 'CGO_ENABLED': '0', 'GOMAXPROCS': '2',
               'GOFLAGS': '-mod=readonly -p=1', 'LANG': 'C', 'LC_ALL': 'C'}

        def run(label, argv, seconds=90, stdin=None, expected_exit=0, environment=None):
            return host.owned(label, argv, environment or env, seconds, ROOT, stdin, expected_exit)

        version = run('go-identity', [go, 'version'], 15).decode().strip()
        assert version in ('go version ' + GO_VERSION + ' darwin/arm64', 'go version ' + GO_VERSION + ' linux/amd64')
        go_identity = dict(host.ref(go), version=version)
        identity = host.strict(run('compiler-identity', [compiler, 'version', '--build', '--json'], 15))
        for key, value in {'compiler_source_sha': PIN, 'version': facts['compiler_version'],
                           'vcs_revision': PIN, 'source_status': 'CLEAN_VCS', 'vcs_modified': 'false',
                           'go_version': GO_VERSION, 'native_go_required': GO_VERSION, 'build_info_observed': True}.items():
            host.exact(identity[key], value)
        host.exact(identity['module'], profile['module'])
        host.exact(identity['decision_runtime'], profile['decision_runtime'])
        summary.update(compiler=host.ref(compiler), compiler_identity=identity, go_identity=go_identity)
        inputs = host.strict((HERE / 'kernel.inputs.json').read_bytes())
        cases = host.strict((HERE / 'kernel.cases.json').read_bytes())
        assert inputs['schema'] == cases['schema'] == 'gooo/body-composition-cases/v1'
        assert len(inputs['cases']) == len(cases['cases']) == 29
        for item, case in zip(inputs['cases'], cases['cases']):
            host.exact(item, {'inputs': case['inputs']})

        def native_pair(stem, path):
            run(stem + '-execute', [compiler, 'package', 'execute', '--json', '--receipt-compression',
                                   'gzip', '--go', go, '--cases', path, MANIFEST], 180)
            execution = out / (stem + '-execute') / 'stdout'
            run(stem + '-replay', [compiler, 'package', 'replay', '--json', '--receipt-compression',
                                  'gzip', '--go', go, '--receipt', execution, '--cases', path, MANIFEST], 180)
            replay = out / (stem + '-replay') / 'stdout'
            count, projection = verify_pair(execution, replay, path, go_identity)
            return execution, replay, count, projection

        execution, replay, count, raw_projection = native_pair('kernel', HERE / 'kernel.cases.json')
        assert count == 29
        generated = out / 'generated'
        run('source-bound-generation', [sys.executable, '-I', '-B', HERE / 'generate.py',
                                       '--execution', execution, '--replay', replay,
                                       '--gofmt', go.parent / 'gofmt', '--out', generated], 60)
        names = {str(p.relative_to(generated)) for p in generated.rglob('*') if p.is_file()}
        assert names == GENERATED | {'generation.provenance.jsonld'}
        for name in GENERATED:
            assert bounded(generated / name, 2 << 20) == bounded(HERE / name, 2 << 20), name
        generation = host.strict((generated / 'generation.json').read_bytes())
        assert generation['schema'] == 'gooo/recorded-catalog-generation/v1'
        assert generation['entry'] == ENTRY and generation['activity_id'] == PREFIX + 'compare'
        assert generation['compiler_source_sha'] == PIN and generation['go_version'] == GO_VERSION
        host.exact(generation['source'], {name: sha((ROOT / name).read_bytes()) for name in SOURCES.values()})
        assert generation['workspace_sha256'] == sha(MANIFEST.read_bytes())
        assert generation['cases_sha256'] == sha((HERE / 'kernel.cases.json').read_bytes())
        assert generation['generator_sha256'] == sha((HERE / 'generate.py').read_bytes())
        assert generation['raw_projection_sha256'] == sha(raw_projection.encode())
        pin_bytes = json.dumps({'compiler_revision': PIN, 'go_version': GO_VERSION}, sort_keys=True, separators=(',', ':')).encode()
        assert generation['selected_upstream_pins_sha256'] == sha(pin_bytes)
        host.exact(generation['generated_assets'], {name: sha((generated / name).read_bytes()) for name in GENERATED - {'generation.json'}})
        verify_provenance(generated, execution, replay, go)
        host.save(out / 'generated-asset-check.json', {'files': sorted(GENERATED), 'all_exact': True,
                                                     'actual_PROV_O': host.ref(generated / 'generation.provenance.jsonld'),
                                                     'provenance_bindings_verified': True})
        run('module-integrity', [go, '-C', HERE, 'mod', 'verify'])
        run('go-vet', [go, '-C', HERE, 'vet', './...'])
        export = out / 'consumer-export.json'
        race = dict(env, CGO_ENABLED='1', GOOO_RECORDED_EXPORT=str(export))
        run('API-race-and-fixture-export', [go, '-C', HERE, 'test', '-race', './...', '-count=1'], 180, environment=race)
        records = host.strict(bounded(export, 2 << 20))
        fixtures = host.strict((HERE / 'consumer-cases.json').read_bytes())
        assert fixtures['schema'] == 'gooo/recorded-catalog-consumer-cases/v1'
        assert fixtures['evidence_kind'] == 'SYNTHETIC_FIXTURE'
        assert len(records) == len(fixtures['cases']) == 7
        assert len({f['label'] for f in fixtures['cases']}) == 7
        deps = [line for line in run('native-runtime-dependencies', [go, '-C', HERE, 'list', '-deps', '-f',
                '{{if not .Standard}}{{.ImportPath}}{{end}}', './cmd/compare']).decode().splitlines() if line]
        assert set(deps) == {MODULE, MODULE + '/internal/kernel', MODULE + '/internal/definitionkernel', MODULE + '/internal/observationkernel', MODULE + '/internal/availabilitykernel', MODULE + '/internal/availabilityobservationkernel', MODULE + '/internal/snapshotkernel', MODULE + '/cmd/compare'} and len(deps) == 8
        binary = out / 'bin/compare'
        run('native-CLI-build', [go, '-C', HERE, 'build', '-trimpath', '-buildvcs=false', '-o', binary, './cmd/compare'])
        info = host.strict(run('native-CLI-build-info', [go, 'version', '-m', '-json', binary], 20))
        assert info['GoVersion'] == GO_VERSION and info['Main']['Path'] == MODULE and not info.get('Deps')
        settings = {s['Key']: s['Value'] for s in info['Settings']}
        assert settings['CGO_ENABLED'] == '0' and settings['-trimpath'] == 'true'
        summary['CLI_binary'] = host.ref(binary)
        runtime = dict(env, PATH='')
        runtime.pop('GOROOT', None)
        actual_cases, observations = [], []
        for index, (record, fixture) in enumerate(zip(records, fixtures['cases'])):
            path = out / ('consumer-' + str(index) + '.json')
            raw = b'\n' + go_json(record['input']) + b'\n'
            assert len(raw) <= 1 << 20
            path.write_bytes(raw)
            wire = run('consumer-' + str(index), [binary], 15, path, environment=runtime)
            assert len(wire) <= (4 << 20) + 1
            report = host.strict(wire)
            actual_cases.append(verify_consumer(report, record, fixture, raw))
            observations.append({'label': fixture['label'], 'evidence_kind': 'SYNTHETIC_FIXTURE',
                                 'input': host.ref(path), 'report': host.ref(out / ('consumer-' + str(index)) / 'stdout'),
                                 'alias_state': report['alias_decision']['state'],
                                 'collection_state': report['collection_context']['state']})
        assert run('repeat-identical-input', [binary], 15, path, environment=runtime) == wire
        actual_path = out / 'actual-consumer-kernel.cases.json'
        host.save(actual_path, {'schema': 'gooo/body-composition-cases/v1', 'cases': actual_cases})
        _, _, actual_count, actual_projection = native_pair('actual-consumer', actual_path)
        assert actual_count == 7 and actual_projection == raw_projection
        host.save(out / 'consumer-observations.json', observations)
        _, original_replay = receipt(replay)
        controls = []
        for label in ('wrong-parent', 'wrong-imported-source', 'wrong-origin-input', 'wrong-collection-input', 'wrong-authority'):
            altered = copy.deepcopy(original_replay)
            if label == 'wrong-parent':
                altered['replayed_from_sha256'] = 'sha256:' + '0' * 64
            elif label == 'wrong-imported-source':
                altered['result']['program']['workspace_image']['packages'][0]['sources'][0]['source_digest'] = 'sha256:' + '0' * 64
            else:
                delivery = altered['result']['runtime']['traces'][0]['deliveries'][0]
                if label == 'wrong-origin-input':
                    delivery['inputs'][0]['value']['target_alias'] = 'tampered-target'
                elif label == 'wrong-collection-input':
                    delivery['inputs'][1]['value']['before_catalog_authorization_context_id'] = 'tampered-context'
                else:
                    delivery['actual']['execution_authorized'] = True
            path = out / (label + '.json.gz')
            path.write_bytes(gzip.compress(json.dumps(altered, separators=(',', ':')).encode(), mtime=0))
            try:
                verify_pair(execution, path, HERE / 'kernel.cases.json', go_identity)
            except (AssertionError, ValueError):
                pass
            else:
                raise AssertionError('receipt mutation accepted: ' + label)
            assert run(label, [sys.executable, '-I', '-B', HERE / 'generate.py', '--execution', execution,
                               '--replay', path, '--gofmt', go.parent / 'gofmt', '--check'], 60, expected_exit=1) == b''
            controls.append({'control': label, 'wire': host.ref(path), 'verifier_refused': True, 'generator_refused': True})
        host.save(out / 'receipt-negative-controls.json', controls)
        after = source_snapshot()
        host.exact(after, before)
        host.save(out / 'source-after.json', after)
        host.check_cancelled()
        summary.update(status='SOURCE_RECORDED_CATALOG_AND_SYNTHETIC_CONSUMER_REPLAY_PASS',
                       kernel_cases=count, actual_consumer_cases=actual_count,
                       receipt_negative_controls=len(controls), source_unchanged=True,
                       source_files=len(before), generated_assets_exact=True,
                       generated_assets=len(GENERATED), runtime_path_empty=True,
                       runtime_goroot_absent=True, runtime_nonstandard_packages=deps,
                       repeated_identical_input_equal=True,
                       reused_external_cache=bool(args.go_cache or args.module_cache), no_cold_cache_claim=True)
    except BaseException as error:
        first = {'type': type(error).__name__, 'message': str(error), 'traceback': traceback.format_exc()}
        host.save(out / 'first-failure.json', first)
    finally:
        pending = host.pending_cancellation()
        if pending is not None and first is None:
            first = {'type': type(pending).__name__, 'message': str(pending)}
        closed = bool(host.records) and all(r['waited'] and r['streams_EOF'] and r['PG_ESRCH'] for r in host.records)
        if first is None and not closed:
            first = {'type': 'RuntimeError', 'message': 'process closure not established'}
        if closed:
            try:
                shutil.rmtree(out / 'bin')
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
        summary.update(first_failure=first, records=host.records, all_direct_wait_EOF_PG_ESRCH=closed)
        host.save(out / 'summary.json', summary)
    if first is not None:
        raise SystemExit(1)
    host.check_cancelled()
    print(json.dumps({'status': summary['status'], 'summary': host.ref(out / 'summary.json')}))


if __name__ == '__main__':
    with host.cancellation_handlers():
        main()
