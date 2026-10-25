"""Source-bound finite package-surface evidence; references do not authenticate producers."""
import importlib.util
import re
from pathlib import Path

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
spec = importlib.util.spec_from_file_location('surface_shared', ROOT/'examples/tool-definition-binding/evidence.py')
shared = importlib.util.module_from_spec(spec)
spec.loader.exec_module(shared)
host = shared.host
base = shared.base
read, receipt, sha, same = shared.read, shared.receipt, shared.sha, shared.same
PIN, GO_VERSION = shared.PIN, shared.GO_VERSION
ENTRY = 'example/runtime-package-surface:Inspect'
PACKAGE = 'example/runtime-package-surface'
PREFIX = 'urn:gooo:runtime-package-surface:'
SOURCES = ('ports/runtime-package-surface/contract.gooo', 'ports/runtime-package-surface/inspect.gooo')
MANIFEST = ROOT/'runtime-package-surface.workspace.json'
CASES = HERE/'cases.json'


def validate_upstream():
    upstream = host.strict(read(HERE/'upstream.json'))
    assert upstream['schema'] == 'gooo/runtime-package-surface-upstream/v1'
    by_name = {item['name']: item for item in upstream['files']}
    for path in [*(ROOT/name for name in SOURCES), CASES]:
        raw = read(path)
        assert len(raw) == by_name[path.name]['bytes']
        assert sha(raw) == 'sha256:'+by_name[path.name]['sha256']
    assert sha(read(ROOT/upstream['license_file'])) == 'sha256:'+upstream['license_sha256']
    cases = host.strict(read(CASES))
    assert cases['schema'] == 'gooo/body-composition-cases/v1' and len(cases['cases']) == 14
    return upstream


def authored_contract():
    contract = host.strict(read(HERE/'abi.json', 64<<10))
    assert contract['schema'] == 'gooo/runtime-package-surface-authored-contract/v1'
    assert contract['status'] == 'AUTHORED_SOURCE_ABI_NOT_EXECUTION_EVIDENCE'
    assert contract['entry'] == ENTRY and contract['activity_id'] == PREFIX+'inspect'
    assert contract['input_entity_id'] == PREFIX+'observation'
    assert contract['output_entity_id'] == PREFIX+'assessment'
    # Check the explicit authored field identities, not an ID inferred from names.
    source = read(ROOT/SOURCES[0]).decode('utf-8')
    records = {}
    for name, identity, body in re.findall(r'entity (\w+) id "([^"]+)" fields \{\n(.*?)\n\}', source, re.S):
        fields = [{'name': n, 'id': i, 'type_id': 'urn:gooo:type:'+t, 'cardinality': c}
                  for n, i, t, c in re.findall(r'field (\w+) id "([^"]+)" type (\w+) required (\w+)', body)]
        assert name not in records and fields
        records[name] = {'id': identity, 'fields': fields}
    same(contract['records'], records)
    assert set(records) == {'Observation', 'Assessment', 'Paths'}
    return contract


def semantic_records(plan):
    contract = authored_contract()
    records = {r['id']: r for r in plan['record_types']}
    wanted = {r['id']: r for r in contract['records'].values()}
    assert len(records) == len(plan['record_types']) == 3 and set(records) == set(wanted)
    identifiers = []
    for identity, record in records.items():
        identifiers.append(record['go_name'])
        fields = record['fields']
        expected = wanted[identity]['fields']
        assert len(fields) == len(expected)
        same([f['name'] for f in fields], [f['name'] for f in expected])
        assert len({f['go_name'] for f in fields}) == len(fields)
        for field, original in zip(fields, expected):
            for key in ('name', 'id', 'type_id', 'cardinality'):
                same(field.get(key, 'one'), original[key])
            identifiers.append(field['go_name'])
    assert len(plan['activities']) == 1
    activity = plan['activities'][0]
    for key in ('id', 'input_entity_id', 'output_entity_id'):
        same(activity[key], contract['activity_id' if key == 'id' else key])
    assert activity['input_from'] == -1 and activity['assembling'] is False and 'inputs' not in activity
    identifiers.append(activity['go_function'])
    assert all(re.fullmatch(r'[A-Za-z_][A-Za-z0-9_]*', name) for name in identifiers)
    return records, activity


def observed_fields(fields, value, record):
    assert type(value) is dict and set(value) == {f['name'] for f in record['fields']}
    same(fields, [{'id': f['id'], 'name': f['name'], 'value': value[f['name']]} for f in record['fields']])
    types = {'urn:gooo:type:string': str, 'urn:gooo:type:boolean': bool, 'urn:gooo:type:integer': int}
    for field in record['fields']:
        item = value[field['name']]
        if field.get('cardinality', 'one') == 'many':
            assert type(item) is list and all(type(v) is types[field['type_id']] for v in item)
        else:
            assert type(item) is types[field['type_id']]


def verify_receipt(path, cases_path, go_identity, *, source_root=ROOT, manifest=MANIFEST,
                   decision='PASS', passed=14, parent=None):
    raw, document = receipt(path)
    case_raw = read(cases_path, 2<<20)
    cases = host.strict(case_raw)
    assert cases['schema'] == 'gooo/body-composition-cases/v1' and len(cases['cases']) == 14
    same(cases, host.strict(read(CASES)))
    assert document['schema'] == 'gooo/workspace-body-execution-receipt/v1'
    assert document['decision'] == decision and document['manifest_digest'] == sha(read(manifest))
    assert document['cases_digest'] == sha(case_raw)
    result = document['result']
    runtime, program, composition = result['runtime'], result['program'], result['composition']
    assert runtime['stage'] == 'COMPLETE' and runtime['producer_source_sha'] == PIN
    same(runtime['producer_module'], host.ACTIVE_COMPILER_PROFILE['module'])
    assert runtime['go_version'] == go_identity['version'] and runtime['go_tool_sha256'] == 'sha256:'+go_identity['sha256']
    assert runtime['go_tool_selection'] == 'explicit' and runtime['model_calls'] == 0
    assert runtime['projection_replayed'] is True and runtime['runtime_replayed'] is True
    assert runtime['finite_total'] == 14 and runtime['finite_passed'] == passed and len(runtime['runs']) == 2
    for process in [runtime['toolchain'], runtime['build'], *runtime['runs']]:
        assert process['started'] is True and process['completed'] is True and process['exit_code'] == 0
        assert all(process[key] is False for key in ('canceled', 'timed_out', 'output_truncated'))
    assert program['entry']['package_path'] == PACKAGE and program['entry']['activity'] == 'Inspect'
    packages = program['workspace_image']['packages']
    assert len(packages) == 1 and packages[0]['path'] == PACKAGE
    sources = {s['filename']: s for s in packages[0]['sources']}
    assert len(packages[0]['sources']) == 2 and set(sources) == set(SOURCES)
    source_bytes = {name: read(source_root/name) for name in SOURCES}
    for name, source in source_bytes.items():
        assert sources[name]['source_digest'] == sha(base.go_json_string(source.decode('utf-8')))
    wanted = {PREFIX+'duplicates': SOURCES[0], PREFIX+'inspect': SOURCES[1]}
    origins = program['activity_origins']
    assert len(origins) == 2 and {o['lowered_activity_id'] for o in origins} == set(wanted)
    for origin in origins:
        binding = origin['input_source']; name = wanted[origin['lowered_activity_id']]
        assert binding['status'] == 'MAPPED' and binding['filename'] == name
        assert binding['package_path'] == PACKAGE and binding['source_sha256'] == sha(source_bytes[name])
        assert 0 <= binding['start_byte'] < binding['end_byte'] <= len(source_bytes[name])
    assert sha(program['lowered_gooo_source'].encode()) == result['execution_source_sha256']
    assert composition['stage'] == 'COMPLETE'
    assert sha(composition['source'].encode()) == composition['generated_sha256'] == runtime['generated_sha256']
    assert sha(composition['driver_source'].encode()) == composition['driver_sha256'] == runtime['driver_sha256']
    assert runtime['typed_plan_sha256'] == composition['plan']['typed_plan_sha256']
    records, _ = semantic_records(composition['plan'])
    assert len(runtime['traces']) == 14 and passed in (0, 14)
    for index, (trace, case) in enumerate(zip(runtime['traces'], cases['cases'])):
        assert trace['case_index'] == index and len(trace['deliveries']) == 1
        assert set(case['inputs']) == set(case['expected']) == {ENTRY}
        delivery = trace['deliveries'][0]
        assert delivery['activity_id'] == PREFIX+'inspect' and delivery['passed'] is (passed == 14)
        same(delivery['input'], case['inputs'][ENTRY])
        observed_fields(delivery['input_fields'], delivery['input'], records[PREFIX+'observation'])
        same(delivery['expected'], case['expected'][ENTRY])
        observed_fields(delivery['actual_fields'], delivery['actual'], records[PREFIX+'assessment'])
        if passed == 14:
            same(delivery['actual'], case['expected'][ENTRY])
        else:
            assert delivery['actual'] != case['expected'][ENTRY]
    if parent is not None:
        parent_raw, parent_document = receipt(parent)
        assert document['replayed_from_sha256'] == sha(parent_raw) and result['replay']['model_calls'] == 0
        same(composition, parent_document['result']['composition'])
    return composition


def verify_index(path, go_identity):
    validate_upstream()
    value = host.strict(read(path, 64<<10))
    assert set(value) == {'schema', 'cases', 'execution', 'replay'}
    assert value['schema'] == 'gooo/runtime-package-surface-generation-input/v1'
    cases, execution, replay = [shared.local_member(path.parent, value[k]) for k in ('cases', 'execution', 'replay')]
    assert len({cases, execution, replay}) == 3 and read(cases) == read(CASES)
    first = verify_receipt(execution, cases, go_identity)
    second = verify_receipt(replay, cases, go_identity, parent=execution)
    same(first, second)
    return first, [cases, execution, replay]
