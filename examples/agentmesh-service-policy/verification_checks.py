"""Pure evidence assertions, shared by fresh acceptance and adversarial tests."""

if not __debug__: raise SystemExit("Python optimization disables integrity assertions; run without -O/PYTHONOPTIMIZE")

ENTRIES = {
    'matcher': ('go/google/agentmesh/api', 'MatchServicePattern', ['pattern', 'req_type', 'req_name']),
    'impact': ('app/service-grant-impact', 'CompareServiceGrant', ['before', 'after', 'req_type', 'req_name']),
}
FIELD_TYPES = {'before_match': ('gooo://service-impact/before', 'urn:gooo:type:boolean', bool),
               'after_match': ('gooo://service-impact/after', 'urn:gooo:type:boolean', bool),
               'change': ('gooo://service-impact/change', 'urn:gooo:type:string', str)}


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
    assert type(delivery) is dict and set(delivery) == names | ({'actual_fields'} if prefix == 'impact' else set())
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
    assert exported['output_type'] == 'gooo://service-impact/decision/v1'
    assert body_report['output_type'] == 'GrantImpact'
    _decision(delivery['actual'], expected)
    _decision(delivery['expected'], expected)
    records = [r for r in body_report['record_types'] if r['id'] == exported['output_type']]
    assert len(records) == 1 and records[0]['name'] == 'GrantImpact'
    schema_fields = records[0]['fields']
    assert type(schema_fields) is list and len(schema_fields) == len(FIELD_TYPES)
    assert [f['name'] for f in schema_fields] == list(FIELD_TYPES)
    actual_fields = delivery['actual_fields']
    assert type(actual_fields) is list and len(actual_fields) == len(schema_fields)
    for schema, actual in zip(schema_fields, actual_fields):
        name = schema['name']; field_id, type_id, kind = FIELD_TYPES[name]
        assert schema['id'] == field_id and schema['type_id'] == type_id
        assert type(actual) is dict and set(actual) == {'id', 'name', 'value'}
        assert actual['id'] == schema['id'] and actual['name'] == name
        assert type(actual['value']) is kind and actual['value'] == expected[name]
