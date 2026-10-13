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
    if prefix.startswith('checked-'):
        return check_typed_workspace_trace(CHECKED[prefix.removeprefix('checked-')],trace,index,case,program,body_report)
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

CHECKED = {}
for _kind, _activity, _record, _core in [
    ('matcher', 'MatchServicePatternChecked', 'CheckedMatcherDecision', [('matched', bool)]),
    ('impact', 'CompareServiceGrantChecked', 'CheckedImpactDecision', [(n, t[2]) for n,t in FIELD_TYPES.items()]),
    ('refinement', 'AnalyzeServiceGrantChecked', 'CheckedRefinementDecision', [(n, t[2]) for n,t in REFINEMENT_FIELDS.items()]),
]:
    _base='gooo://service-policy/checked-'+_kind
    CHECKED[_kind]={'package':'gooo/service-policy/checked-'+_kind,'activity':_activity,'activity_id':_base+'/evaluate/v1',
        'record':_record,'record_id':_base+'/decision/v1','inputs':ENTRIES[_kind][2],
        'fields':{n:(_base+'/'+n,'urn:gooo:type:'+('boolean' if t is bool else 'string'),t)
                  for n,t in [('domain_status',str),('invalid_field',str)]+_core}}


def check_checked_decision(kind, actual, expected):
    config=CHECKED[kind];fields=config['fields']
    assert type(actual)is dict and type(expected)is dict and set(actual)==set(expected)==set(fields)
    for name,(_,_,typ) in fields.items():
        assert type(actual[name])is typ and type(expected[name])is typ and actual[name]==expected[name]
    assert actual['domain_status'] in {'VALID_INPUT','INVALID_INPUT'}
    core={k:v for k,v in actual.items() if k not in ['domain_status','invalid_field']}
    if actual['domain_status']=='INVALID_INPUT':
        assert actual['invalid_field'] in config['inputs']
        for name,(_,_,typ) in fields.items():
            if name in ['domain_status','invalid_field']:continue
            assert actual[name] == (False if typ is bool else 'INVALID_INPUT' if name in ['status','change'] else '')
    else:
        assert actual['invalid_field']==''
        if kind=='refinement':check_refinement_decision(core,core)
        elif kind=='impact':
            state={ (False,False):'UNCHANGED_DENY',(False,True):'WIDENED',(True,False):'REVOKED',(True,True):'UNCHANGED_ALLOW'}
            assert actual['change']==state[(actual['before_match'],actual['after_match'])]


def check_native_checked(actual, cases, expected_bindings):
    assert type(actual)is list and len(actual)==len(cases) and len(cases)>0
    for result,case in zip(actual,cases):
        assert type(result)is dict and set(result)=={'decision','facts','bindings'}
        _facts(result['facts'],case['facts'],CHECKED[case['api']]['inputs'])
        check_checked_decision(case['api'],result['decision'],case['want'])
        assert result['bindings']==expected_bindings


def check_typed_workspace_trace(config,trace,index,case,program,body_report):
    package=config['package'];activity=config['activity'];entry=package+':'+activity
    assert program['entry']['package_path']==package and program['entry']['activity']==activity
    packages=[p for p in program['workspace_image']['packages']if p['path']==package];assert len(packages)==1
    exports=[e for e in packages[0]['exports']if e['kind']=='activity' and e['name']==activity];assert len(exports)==1
    exported=exports[0];assert exported['id']==config['activity_id'] and exported['input_types']==['gooo://std/text']*len(config['inputs'])
    assert exported['output_type']==config['record_id'] and body_report['output_type']==config['record']
    assert body_report['activity']==program['entry']['lowered_name'] and body_report['activity_id']==config['activity_id']
    assert type(trace)is dict and set(trace)=={'case_index','deliveries'} and type(trace['case_index'])is int and trace['case_index']==index
    assert type(trace['deliveries'])is list and len(trace['deliveries'])==1
    delivery=trace['deliveries'][0];scalar=config['record']=='bool'
    assert type(delivery)is dict and set(delivery)=={'activity_id','inputs','actual','expected','passed'}| (set()if scalar else{'actual_fields'})
    assert delivery['activity_id']==config['activity_id'] and delivery['passed']is True
    assert type(delivery['inputs'])is list and len(delivery['inputs'])==len(config['inputs'])
    assert set(case['inputs'])=={entry+'.input'+str(i)for i in range(len(config['inputs']))} and set(case['expected'])=={entry}
    for i,item in enumerate(delivery['inputs']):
        assert type(item)is dict and set(item)=={'port','entity_id','value'}
        assert item['port']=='input'+str(i) and item['entity_id']=='gooo://std/text' and type(item['value'])is str
        assert item['value']==case['inputs'][entry+'.input'+str(i)]
    expected=case['expected'][entry]
    if scalar:
        assert type(expected)is bool and type(delivery['actual'])is bool and type(delivery['expected'])is bool
        assert delivery['actual']==delivery['expected']==expected
        return
    kind=package.removeprefix('gooo/service-policy/checked-')
    check_checked_decision(kind,delivery['actual'],expected);check_checked_decision(kind,delivery['expected'],expected)
    records=[r for r in body_report['record_types']if r['id']==config['record_id']];assert len(records)==1 and records[0]['name']==config['record']
    fields=records[0]['fields'];assert type(fields)is list and [f['name']for f in fields]==list(config['fields'])
    assert type(delivery['actual_fields'])is list and len(delivery['actual_fields'])==len(fields)
    for schema,actual in zip(fields,delivery['actual_fields']):
        name=schema['name'];fid,tid,typ=config['fields'][name]
        assert schema['id']==fid and schema['type_id']==tid
        assert type(actual)is dict and set(actual)=={'id','name','value'} and actual['id']==fid and actual['name']==name
        assert type(actual['value'])is typ and actual['value']==expected[name]


def check_domain_trace(activity,trace,index,case,program,body_report):
    # The pinned scalar case schema keys input by the entry itself, and traces
    # use `input`; input0/inputs belongs only to multi-argument activities.
    package='gooo/service-domain';entry=package+':'+activity
    aid={'ValidateServiceType':'gooo://service-domain/type','ValidateServiceName':'gooo://service-domain/name','ValidateServicePattern':'gooo://service-domain/pattern'}[activity]
    assert program['entry']['package_path']==package and program['entry']['activity']==activity
    packages=[p for p in program['workspace_image']['packages']if p['path']==package];assert len(packages)==1
    exports=[e for e in packages[0]['exports']if e['kind']=='activity' and e['name']==activity];assert len(exports)==1
    exported=exports[0];assert exported['id']==aid and exported['input_types']==['gooo://std/text'] and exported['output_type']=='gooo://std/boolean'
    assert body_report['activity']==program['entry']['lowered_name'] and body_report['activity_id']==aid
    assert body_report['input_type']=='string' and body_report['output_type']=='bool'
    assert set(case['inputs'])=={entry} and set(case['expected'])=={entry}
    assert type(trace)is dict and set(trace)=={'case_index','deliveries'} and type(trace['case_index'])is int and trace['case_index']==index
    assert type(trace['deliveries'])is list and len(trace['deliveries'])==1
    delivery=trace['deliveries'][0]
    assert type(delivery)is dict and set(delivery)=={'activity_id','input','actual','expected','passed'}
    assert delivery['activity_id']==aid and delivery['passed']is True
    assert type(delivery['input'])is str and delivery['input']==case['inputs'][entry]
    expected=case['expected'][entry]
    assert type(expected)is bool and type(delivery['actual'])is bool and type(delivery['expected'])is bool
    assert delivery['actual']==delivery['expected']==expected


def check_domain_reference(reference):
    import hashlib
    digest=lambda text:hashlib.sha256(text.encode()).hexdigest()
    assert reference['schema']=='gooo/frozen-service-domain-reference/v1'
    assert reference['baseline_commit']=='0c6a55f7021128358be05615f033c4f8c7678bef'
    assert reference['source_path']=='examples/agentmesh-service-policy/domain.go'
    assert reference['source_bytes']==len(reference['source'].encode())==2847
    assert reference['source_sha256']==digest(reference['source'])=='62345dae60badeb5bf69bc5e5995322c09402ab9eb3f0141903aac3041f91528'
    assert set(reference['functions'])==set(reference['function_sha256'])=={'validType','validName','validPattern'}
    for name,body in reference['functions'].items():
        start=reference['source'].index('func '+name+'(');end=reference['source'].index('\n}',start)+2
        assert reference['source'][start:end]==body and digest(body)==reference['function_sha256'][name]
    declarations=reference['source'][reference['source'].index('const MaxComponentBytes'):reference['source'].index('// This validates')]
    assert reference['declarations']==declarations and reference['declarations_sha256']==digest(declarations)
