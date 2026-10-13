"""Pure evidence assertions, shared by fresh acceptance and adversarial tests."""

if not __debug__: raise SystemExit("Python optimization disables integrity assertions; run without -O/PYTHONOPTIMIZE")

ENTRIES = {
    'matcher': ('go/google/agentmesh/api', 'MatchServicePattern', ['pattern', 'req_type', 'req_name']),
    'impact': ('app/service-grant-impact', 'CompareServiceGrant', ['before', 'after', 'req_type', 'req_name']),
    'refinement': ('app/service-grant-refinement', 'AnalyzeServiceGrant', ['before', 'after']),
}
FIELD_TYPES = {'before_match': ('gooo://service-impact/before', 'urn:gooo:type:boolean', bool),
               'after_match': ('gooo://service-impact/after', 'urn:gooo:type:boolean', bool),
               'change': ('gooo://service-impact/change', 'urn:gooo:type:string', str)}

REFINEMENT_FIELDS = {name: ('gooo://service-refinement/' + name,
                           'urn:gooo:type:' + ('boolean' if kind is bool else 'string'), kind)
                     for name, kind in [('status', str), ('rule_id', str), ('supported_subset', bool),
                                        ('has_witness', bool), ('witness_type', str), ('witness_name', str),
                                        ('before_match', bool), ('after_match', bool)]}
SUBSET_RULES = {'CANDIDATE_EMPTY', 'IDENTICAL_PATTERNS', 'OLD_GLOBAL', 'OLD_TYPED_ALL',
                'EXACT_CANDIDATE_ACCEPTED_BY_OLD', 'PREFIX_HIERARCHY', 'SUFFIX_HIERARCHY'}


def check_refinement_decision(actual, expected):
    import re
    assert type(actual) is dict and set(actual) == set(REFINEMENT_FIELDS)
    assert type(expected) is dict and set(expected) == set(REFINEMENT_FIELDS)
    for field, (_, _, kind) in REFINEMENT_FIELDS.items():
        assert type(actual[field]) is kind and type(expected[field]) is kind
        assert actual[field] == expected[field]
    if actual['status'] == 'WIDENING_WITNESS':
        assert actual['rule_id'] == 'CHECKED_CANDIDATE_REQUEST'
        assert actual['supported_subset'] is False and actual['has_witness'] is True
        assert actual['before_match'] is False and actual['after_match'] is True
        assert 0 < len(actual['witness_type']) <= 128 and re.fullmatch(r'[A-Za-z][A-Za-z0-9+.-]*', actual['witness_type'])
        assert 0 < len(actual['witness_name']) <= 128 and re.fullmatch(r'[A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+)*', actual['witness_name'])
    else:
        assert actual['status'] in {'SUPPORTED_SUBSET', 'UNPROVEN'}
        assert actual['has_witness'] is actual['before_match'] is actual['after_match'] is False
        assert actual['witness_type'] == actual['witness_name'] == ''
        assert actual['supported_subset'] is (actual['status'] == 'SUPPORTED_SUBSET')
        assert actual['rule_id'] in SUBSET_RULES if actual['supported_subset'] else actual['rule_id'] == ''


def check_native_refinement(actual, cases, expected_bindings):
    assert type(cases) is list and len(cases) == 101
    assert type(actual) is list and len(actual) == len(cases)
    for result, case in zip(actual, cases):
        assert type(result) is dict and set(result) == {'decision', 'facts', 'bindings'}
        _facts(result['facts'], case, ENTRIES['refinement'][2])
        check_refinement_decision(result['decision'], case['want'])
        assert result['bindings'] == expected_bindings


def check_witness_oracle(actual, requests, source_revision):
    assert type(actual) is dict and set(actual) == {'schema', 'go_version', 'source_revision', 'observations', 'impact_matches'}
    assert actual['schema'] == 'gooo/agentmesh-matcher-oracle/v1' and actual['go_version'] == 'go1.27.2'
    assert actual['source_revision'] == source_revision
    assert type(actual['observations']) is list and len(actual['observations']) == len(requests) == 126
    for result, request in zip(actual['observations'], requests):
        assert type(result) is dict and set(result) == {'name', 'pattern', 'req_type', 'req_name', 'matched'}
        for field in ['name', 'pattern', 'req_type', 'req_name']:
            assert type(result[field]) is str and result[field] == request[field]
        assert type(result['matched']) is bool and result['matched'] is request['want']


def _facts(actual, expected, fields):
    assert type(actual) is dict and set(actual) == set(fields)
    assert all(type(actual[name]) is str and actual[name] == expected[name] for name in fields)


def _decision(actual, expected):
    assert type(actual) is dict and set(actual) == set(FIELD_TYPES)
    for name, (_, _, kind) in FIELD_TYPES.items():
        assert type(actual[name]) is kind and type(expected[name]) is kind
        assert actual[name] == expected[name]


def check_native_rows(matcher, impact, rows, impacts, expected_bindings):
    assert type(rows) is list and len(rows) == 63
    assert type(impacts) is list and len(impacts) == 16
    assert type(matcher) is list and len(matcher) == len(rows)
    assert type(impact) is list and len(impact) == len(impacts)
    for actual, expected in zip(matcher, rows):
        assert type(actual) is dict and set(actual) == {'matched', 'facts', 'bindings'}
        assert type(actual['matched']) is bool and type(expected['matched']) is bool
        assert actual['matched'] == expected['matched']
        _facts(actual['facts'], expected, ENTRIES['matcher'][2])
        assert actual['bindings'] == expected_bindings
    for actual, expected in zip(impact, impacts):
        assert type(actual) is dict and set(actual) == {'decision', 'facts', 'bindings'}
        _facts(actual['facts'], expected, ENTRIES['impact'][2])
        _decision(actual['decision'], expected['want'])
        assert actual['bindings'] == expected_bindings


def check_workspace_trace(prefix, trace, index, case, program, body_report):
    package, activity, fields = ENTRIES[prefix]
    entry = package + ':' + activity
    assert program['entry']['package_path'] == package and program['entry']['activity'] == activity
    packages = [p for p in program['workspace_image']['packages'] if p['path'] == package]
    assert len(packages) == 1
    exports = [e for e in packages[0]['exports'] if e['kind'] == 'activity' and e['name'] == activity]
    assert len(exports) == 1
    exported = exports[0]
    assert exported['input_types'] == ['gooo://std/text'] * len(fields)
    assert body_report['activity_id'] == exported['id']
    assert body_report['activity'] == program['entry']['lowered_name']
    assert type(trace) is dict and set(trace) == {'case_index', 'deliveries'}
    assert type(trace['case_index']) is int and trace['case_index'] == index
    assert type(trace['deliveries']) is list and len(trace['deliveries']) == 1
    delivery = trace['deliveries'][0]
    names = {'activity_id', 'inputs', 'actual', 'expected', 'passed'}
    assert type(delivery) is dict and set(delivery) == names | ({'actual_fields'} if prefix != 'matcher' else set())
    assert delivery['activity_id'] == exported['id'] and delivery['passed'] is True
    assert type(delivery['inputs']) is list and len(delivery['inputs']) == len(fields)
    assert set(case['inputs']) == {entry + '.input' + str(i) for i in range(len(fields))}
    assert set(case['expected']) == {entry}
    for i, item in enumerate(delivery['inputs']):
        assert type(item) is dict and set(item) == {'port', 'entity_id', 'value'}
        assert item['port'] == 'input' + str(i) and item['entity_id'] == exported['input_types'][i]
        value = case['inputs'][entry + '.input' + str(i)]
        assert type(item['value']) is str and type(value) is str and item['value'] == value
    expected = case['expected'][entry]
    if prefix == 'matcher':
        assert exported['output_type'] == 'gooo://std/boolean' and body_report['output_type'] == 'bool'
        assert type(delivery['actual']) is bool and type(delivery['expected']) is bool and type(expected) is bool
        assert delivery['actual'] == delivery['expected'] == expected
        return
    entity_id, record_name, field_types, decision_check = ('gooo://service-impact/decision/v1', 'GrantImpact', FIELD_TYPES, _decision) if prefix == 'impact' else ('gooo://service-refinement/decision/v1', 'GrantRefinement', REFINEMENT_FIELDS, check_refinement_decision)
    assert exported['output_type'] == entity_id
    assert body_report['output_type'] == record_name
    decision_check(delivery['actual'], expected)
    decision_check(delivery['expected'], expected)
    records = [r for r in body_report['record_types'] if r['id'] == exported['output_type']]
    assert len(records) == 1 and records[0]['name'] == record_name
    schema_fields = records[0]['fields']
    assert type(schema_fields) is list and len(schema_fields) == len(field_types)
    assert [f['name'] for f in schema_fields] == list(field_types)
    actual_fields = delivery['actual_fields']
    assert type(actual_fields) is list and len(actual_fields) == len(schema_fields)
    for schema, actual in zip(schema_fields, actual_fields):
        name = schema['name']; field_id, type_id, kind = field_types[name]
        assert schema['id'] == field_id and schema['type_id'] == type_id
        assert type(actual) is dict and set(actual) == {'id', 'name', 'value'}
        assert actual['id'] == schema['id'] and actual['name'] == name
        assert type(actual['value']) is kind and actual['value'] == expected[name]
