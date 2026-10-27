"""Finite original-row receipts and generation bindings, not producer authentication."""
import importlib.util
import json
import re
from pathlib import Path

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]


def module(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


host = module('sdk_refresh_owned', ROOT/'examples/service-grant-many/verify.py')
base = module('sdk_refresh_json', ROOT/'examples/recorded-catalog-comparison/generate.py')
read = base.read
sha = base.digest
same = host.exact
PROFILE = host.strict(read(HERE/'profile.json'))
MANIFEST = ROOT/'sdk-catalog-row-visibility.workspace.json'
SOURCE = ROOT/'ports/tool-origin/sdk_row_visibility.gooo'
ENTRY = 'gooo/sdk-catalog/row-visibility:CompareSDKRows'
PREFIX = 'gooo://sdk-catalog/row-visibility/v1/'
FIELDS = ['state', 'reason', 'wire_count', 'sdk_count', 'first_difference_index',
          'wire_name', 'sdk_name', 'mapping_equal', 'scope']
ASSETS = [HERE/'internal/rowvisibility/semantic.gooo.go', HERE/'generation.json']


def identity(value):
    for key, expected in {
        'schema': 'gooo/build-identity/v1', 'language': 'gooo',
        'compiler_source_sha': PROFILE['compiler_revision'], 'vcs_revision': PROFILE['compiler_revision'],
        'version': PROFILE['compiler_version'], 'source_status': 'CLEAN_VCS',
        'vcs_modified': 'false', 'build_info_observed': True,
        'go_version': PROFILE['go_version'], 'native_go_required': PROFILE['go_version'],
        'module_version': PROFILE['module_version'],
    }.items():
        same(value[key], expected)
    same(value['module'], {'path': 'github.com/kimjooyoon/meta-ontology-go', 'version': PROFILE['module_version']})
    same(value['decision_runtime'], {'module_path': 'github.com/kimjooyoon/gooo-decision-runtime',
                                   'version': 'v0.2.51-experimental', 'replaced': False})


def pair(execution, replay, go_identity):
    raw = read(execution)
    first, second = host.strict(raw), host.strict(read(replay))
    assert second['replayed_from_sha256'] == sha(raw)
    corpus = host.strict(read(HERE/'row-cases.json'))
    same(corpus['schema'], 'gooo/body-composition-cases/v1')
    assert len(corpus['cases']) == 8
    for document in [first, second]:
        same(document['schema'], 'gooo/workspace-body-execution-receipt/v1')
        same(document['decision'], 'PASS')
        same(document['manifest'], str(MANIFEST))
        same(document['manifest_digest'], sha(read(MANIFEST)))
        same(document['cases_digest'], sha(read(HERE/'row-cases.json')))
        result = document['result']; runtime = result['runtime']; composition = result['composition']
        same(composition['stage'], 'COMPLETE')
        assert composition['original_source_sha256'] == sha(composition['gooo_source'].encode())
        same(runtime['stage'], 'COMPLETE')
        for name, value in {'finite_total': 8, 'finite_passed': 8, 'model_calls': 0,
                            'producer_source_sha': PROFILE['compiler_revision'],
                            'go_tool_sha256': 'sha256:'+go_identity['sha256'],
                            'go_version': go_identity['version']}.items():
            same(runtime[name], value)
        same(runtime['producer_module'], {'path': 'github.com/kimjooyoon/meta-ontology-go', 'version': PROFILE['module_version']})
        assert runtime['original_source_sha256'] == composition['original_source_sha256']
        assert runtime['typed_plan_sha256'] == composition['plan']['typed_plan_sha256']
        for observation in [runtime['toolchain'], runtime['build'], *runtime['runs']]:
            for name in ['started', 'completed']:
                same(observation[name], True)
            for name in ['canceled', 'timed_out', 'output_truncated']:
                same(observation[name], False)
            same(observation['exit_code'], 0)
        plan = composition['plan']
        assert len(plan['activities']) == 1 and len(plan['record_types']) == 2
        activity = plan['activities'][0]
        same(activity['id'], PREFIX+'compare')
        same(activity['output_entity_id'], PREFIX+'result')
        same([(x['port'], x['entity_id']) for x in activity['inputs']],
             [('input0', PREFIX+'names'), ('input1', PREFIX+'names')])
        records = {x['id']: x for x in plan['record_types']}
        fields = records[PREFIX+'result']['fields']
        same([x['name'] for x in fields], FIELDS)
        for field in fields:
            same(field['id'], PREFIX+'result/'+field['name'])
            expected = 'integer' if field['name'] in FIELDS[2:5] else ('boolean' if field['name'] == 'mapping_equal' else 'string')
            same(field['type_id'], 'urn:gooo:type:'+expected)
            same(field.get('cardinality', 'one'), 'one')
        same([(x['name'], x['id'], x['type_id'], x.get('cardinality', 'one')) for x in records[PREFIX+'names']['fields']],
             [('names', PREFIX+'names/values', 'urn:gooo:type:string', 'many')])
        origins = result['program']['activity_origins']
        assert len(origins) == 1
        source = origins[0]['input_source']
        for key, value in {'package_path': 'gooo/sdk-catalog/row-visibility', 'filename': str(SOURCE.relative_to(ROOT)),
                           'activity_id': PREFIX+'compare', 'source_sha256': sha(read(SOURCE)), 'status': 'MAPPED'}.items():
            same(source[key], value)
        assert len(runtime['traces']) == 8
        for index, trace in enumerate(runtime['traces']):
            same(trace['case_index'], index)
            assert len(trace['deliveries']) == 1
            delivery = trace['deliveries'][0]; case = corpus['cases'][index]
            same(delivery['activity_id'], PREFIX+'compare')
            same(delivery['passed'], True)
            same(delivery['actual'], case['expected'][ENTRY])
            same(delivery['expected'], case['expected'][ENTRY])
            same([(x['port'], x['entity_id'], x['value']) for x in delivery['inputs']],
                 [(port, PREFIX+'names', case['inputs'][ENTRY+'.'+port]) for port in ['input0', 'input1']])
            for item in delivery['inputs']:
                same(item['fields'], [{'id': PREFIX+'names/values', 'name': 'names', 'value': item['value']['names']}])
            same(delivery['actual_fields'], [{'id': PREFIX+'result/'+field, 'name': field, 'value': delivery['actual'][field]} for field in FIELDS])
    same(first['result']['composition']['plan'], second['result']['composition']['plan'])
    return first


def generated(raw, native):
    found = re.findall(rb'^const GoooGenerationOrigin = (".*")$', raw, re.M)
    assert len(found) == 1
    origin = host.strict(json.loads(found[0]))
    same(origin['schema'], 'gooo/library-generation-origin/v1')
    same(origin['entry_activity_id'], PREFIX+'compare')
    same(origin['lowered_source_sha256'], native['result']['composition']['original_source_sha256'])
    same(origin['typed_plan_sha256'], native['result']['composition']['plan']['typed_plan_sha256'])
    same(origin['stage_exports'], None)
    context = origin['context']
    for key, value in {'compiler_version': PROFILE['compiler_version'], 'compiler_source_sha': PROFILE['compiler_revision'],
                       'compiler_source_status': 'CLEAN_VCS', 'go_version': PROFILE['go_version'],
                       'workspace_manifest_sha256': sha(read(MANIFEST)),
                       'workspace_image_sha256': native['result']['program']['workspace_image']['digest']}.items():
        same(context[key], value)
    same(context['sources'], [{'package_path': 'gooo/sdk-catalog/row-visibility', 'filename': str(SOURCE.relative_to(ROOT)),
                              'source_json_string_sha256': sha(base.go_json_string(read(SOURCE).decode()))}])
    return origin
