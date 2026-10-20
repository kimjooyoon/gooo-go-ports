"""Finite conditional-apply evidence checks; no host policy implementation."""
import hashlib
import json
from collections import Counter
from verification_checks import check_trace_envelope

if not __debug__:
    raise SystemExit("Python optimization disables integrity assertions; run without -O/PYTHONOPTIMIZE")

PIN = '205896a413e5dea92366309c35ae85e542d055f6'
PACKAGE = 'gooo/service-policy/conditional-apply'
ACTIVITY = 'AssessServiceGrantUpdate'
ENTRY = PACKAGE + ':' + ACTIVITY
BASE = 'gooo://service-policy/conditional-apply/'
ACTIVITY_ID = BASE + 'assess/v1'
RECORD_ID = BASE + 'decision/v1'
FACTS = ('before', 'after', 'required_type', 'required_name')
FIELDS = {'status': str, 'reason': str, 'invalid_field': str,
          'required_observed': bool, 'required_before_match': bool,
          'required_after_match': bool, 'subset_rule_id': str}
CORPUS_SHA = '62f887a20cb23c525d0fdc2090886809c94bb873ef1429b2ecf96a31c6aa4e95'
DEMO = [
    {'case': 'safe-tightening', 'status': 'APPLIED', 'reason': 'APPLIED',
     'observed_revision': 1, 'committed_revision': 2},
    {'case': 'stale-proposal', 'status': 'CONFLICT', 'reason': 'STALE_REVISION',
     'observed_revision': 2, 'committed_revision': 0},
    {'case': 'widening-refused', 'status': 'REFUSED', 'reason': 'WIDENING_WITNESS',
     'observed_revision': 2, 'committed_revision': 0},
    {'case': 'recover-dropped-response', 'status': 'APPLIED', 'reason': 'APPLIED',
     'observed_revision': 2, 'committed_revision': 3},
]


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def digest(raw):
    return 'sha256:' + sha(raw)


def encoded(value):
    return (json.dumps(value, indent=2, sort_keys=True) + '\n').encode()


def ident(path):
    raw = path.read_bytes()
    return {'path': str(path), 'bytes': len(raw), 'sha256': sha(raw)}


def load_json(raw):
    def unique(items):
        result = {}
        for key, value in items:
            assert key not in result, 'duplicate JSON member: ' + key
            result[key] = value
        return result

    def reject_constant(value):
        raise ValueError('non-JSON numeric constant: ' + value)

    return json.loads(raw, object_pairs_hook=unique, parse_constant=reject_constant)


def strict_equal(actual, expected):
    """JSON equality without Python's bool/int and int/float equivalences."""
    assert type(actual) is type(expected), 'JSON value type differs'
    if type(expected) is dict:
        assert set(actual) == set(expected), 'JSON member set differs'
        for key in expected:
            strict_equal(actual[key], expected[key])
    elif type(expected) is list:
        assert len(actual) == len(expected), 'JSON array length differs'
        for left, right in zip(actual, expected):
            strict_equal(left, right)
    else:
        assert actual == expected, 'JSON value differs'


def case_facts(case):
    assert type(case) is dict and set(case) == {'inputs', 'expected'}
    assert type(case['inputs']) is dict and set(case['inputs']) == {
        ENTRY + '.input' + str(i) for i in range(4)}
    assert type(case['expected']) is dict and set(case['expected']) == {ENTRY}
    result = {name: case['inputs'][ENTRY + '.input' + str(i)] for i, name in enumerate(FACTS)}
    assert all(type(value) is str for value in result.values())
    check_decision(case['expected'][ENTRY], case['expected'][ENTRY])
    return result


def check_decision(actual, expected):
    assert type(actual) is dict and type(expected) is dict
    assert set(actual) == set(expected) == set(FIELDS)
    for field, kind in FIELDS.items():
        assert type(actual[field]) is kind and type(expected[field]) is kind
        assert actual[field] == expected[field]


def check_native(actual, cases, bindings):
    assert type(actual) is list and len(actual) == len(cases) > 0
    for proof, case in zip(actual, cases):
        assert type(proof) is dict and set(proof) == {'facts', 'decision', 'bindings'}
        strict_equal(proof['facts'], case_facts(case))
        check_decision(proof['decision'], case['expected'][ENTRY])
        strict_equal(proof['bindings'], bindings)


def check_trace(trace, index, case, program, report):
    entry = program['entry']
    assert entry['package_path'] == PACKAGE and entry['activity'] == ACTIVITY
    packages = [p for p in program['workspace_image']['packages'] if p['path'] == PACKAGE]
    assert len(packages) == 1
    exported = [e for e in packages[0]['exports'] if e['kind'] == 'activity' and e['name'] == ACTIVITY]
    assert len(exported) == 1
    strict_equal(exported[0], {'name': ACTIVITY, 'kind': 'activity', 'id': ACTIVITY_ID,
                              'input_types': ['gooo://std/text'] * 4, 'output_type': RECORD_ID})
    assert report['activity'] == entry['lowered_name'] and report['activity_id'] == ACTIVITY_ID
    assert report['output_type'] == 'GrantUpdateDecision'
    strict_equal(report['input_parameters'], [{'name': 'input' + str(i), 'type': 'string'} for i in range(4)])
    records = [r for r in report['record_types'] if r['id'] == RECORD_ID]
    assert len(records) == 1 and records[0]['name'] == 'GrantUpdateDecision'
    schemas = records[0]['fields']
    assert type(schemas) is list and [s['name'] for s in schemas] == list(FIELDS)
    check_trace_envelope(trace, index)
    delivery = trace['deliveries'][0]
    assert type(delivery) is dict and set(delivery) == {
        'activity_id', 'inputs', 'actual', 'expected', 'passed', 'actual_fields'}
    assert delivery['activity_id'] == ACTIVITY_ID and delivery['passed'] is True
    facts = case_facts(case)
    strict_equal(delivery['inputs'], [
        {'port': 'input' + str(i), 'entity_id': 'gooo://std/text', 'value': facts[name]}
        for i, name in enumerate(FACTS)])
    expected = case['expected'][ENTRY]
    check_decision(delivery['actual'], expected)
    check_decision(delivery['expected'], expected)
    assert type(delivery['actual_fields']) is list and len(delivery['actual_fields']) == 7
    for schema, actual, (name, kind) in zip(schemas, delivery['actual_fields'], FIELDS.items()):
        assert schema['id'] == BASE + name
        assert schema['type_id'] == 'urn:gooo:type:' + ('boolean' if kind is bool else 'string')
        strict_equal(actual, {'id': BASE + name, 'name': name, 'value': expected[name]})


def source_digest(raw):
    text = json.dumps(raw.decode(), ensure_ascii=False, separators=(',', ':'))
    for before, after in [('<', '\\u003c'), ('>', '\\u003e'), ('&', '\\u0026'),
                          ('\u2028', '\\u2028'), ('\u2029', '\\u2029')]:
        text = text.replace(before, after)
    return digest(text.encode())


def check_program_sources(program, manifest, repo):
    assert program['schema'] == 'gooo/workspace-body-program/v1'
    strict_equal(manifest['entry'], {'package_path': PACKAGE, 'activity': ACTIVITY})
    strict_equal(program['activities'], [program['entry']])
    expected = manifest['packages']
    packages = program['workspace_image']['packages']
    assert type(expected) is list and type(packages) is list
    names = [p['path'] for p in expected]
    assert len(names) == len(set(names)) == len(packages)
    assert [p['path'] for p in packages] == program['workspace_image']['init_order']
    assert set(p['path'] for p in packages) == set(names)
    specs = {p['path']: p for p in expected}
    for package in packages:
        sources = package['sources']
        paths = specs[package['path']]['sources']
        assert type(sources) is list and len(sources) == len(paths) > 0
        assert len(set(paths)) == len(paths)
        for source, filename in zip(sources, paths):
            assert source['filename'] == filename
            assert source['source_digest'] == source_digest((repo / filename).read_bytes())
    image_entry = program['workspace_image']['entry']
    assert image_entry['package_path'] == PACKAGE and image_entry['activity'] == ACTIVITY
    assert image_entry['source'] in specs[PACKAGE]['sources']
    strict_equal(image_entry['inputs'], ['Text'] * 4)
    assert image_entry['output'] == 'GrantUpdateDecision'


def check_receipt(document, corpus, casefile, workspace, program, saved_report, audit_runtime):
    audit_runtime(document, corpus, casefile, workspace)
    result = document['result']
    strict_equal(result['program'], program)
    runtime = result['runtime']
    assert runtime['schema'] == 'gooo/body-composition-runtime/v1' and runtime['stage'] == 'COMPLETE'
    assert runtime['producer_source_sha'] == PIN
    assert runtime['producer_module']['path'] == 'github.com/kimjooyoon/meta-ontology-go'
    for flag in ['projection_replayed', 'runtime_replayed']:
        assert runtime[flag] is True
    for name, expected in [('finite_passed', len(corpus['cases'])), ('finite_total', len(corpus['cases'])), ('model_calls', 0)]:
        assert type(runtime[name]) is int and runtime[name] == expected
    assert type(runtime['runs']) is list and runtime['runs']
    for process in [runtime['toolchain'], runtime['build'], *runtime['runs']]:
        for flag in ['started', 'completed']:
            assert process[flag] is True
        for flag in ['canceled', 'timed_out', 'output_truncated']:
            assert process[flag] is False
        assert type(process['exit_code']) is int and process['exit_code'] == 0
    composition = result['composition']
    assert composition['schema'] == 'gooo/body-composition/v1' and composition['stage'] == 'COMPLETE'
    source = program['lowered_gooo_source']
    source_hash = digest(source.encode())
    assert result['execution_source_sha256'] == source_hash and composition['gooo_source'] == source
    for key in ['original_source_sha256', 'selected_source_sha256']:
        assert composition[key] == runtime[key] == source_hash
    for key, source_key in [('generated_sha256', 'source'), ('driver_sha256', 'driver_source')]:
        assert composition[key] == runtime[key] == digest(composition[source_key].encode())
    assert composition['plan']['typed_plan_sha256'] == runtime['typed_plan_sha256']
    assert type(composition['steps']) is list and len(composition['steps']) == 1
    step = composition['steps'][0]
    assert step['input_source_sha256'] == source_hash
    report = step['generation']['report']
    for key in ['activity', 'activity_id', 'input_parameters', 'record_types', 'output_type',
                'compiler_source_sha', 'source_digest', 'generated_digest', 'replay_digest',
                'typecheck_passed', 'deterministic_replay']:
        strict_equal(report[key], saved_report[key])
    assert report['compiler_source_sha'] == PIN and report['source_digest'] == source_hash
    assert report['typecheck_passed'] is True and report['deterministic_replay'] is True
    assert report['generated_digest'] == digest(step['generation']['source'].encode())
    assert type(runtime['traces']) is list and len(runtime['traces']) == len(corpus['cases'])
    for index, (trace, case) in enumerate(zip(runtime['traces'], corpus['cases'])):
        check_trace(trace, index, case, program, report)


def check_replay_identity(raw, replay):
    assert replay['replayed_from_sha256'] == digest(raw)


def check_demo(raw):
    rows = raw.decode('utf-8', errors='strict').splitlines()
    assert len(rows) == len(DEMO)
    strict_equal([load_json(row) for row in rows], DEMO)


def verify_grant_update(args, out, run, bindings, ROOT, REPO, audit_runtime):
    raw_cases = (ROOT / 'grant-update-cases.json').read_bytes()
    assert sha(raw_cases) == CORPUS_SHA, 'independent 21-case corpus changed'
    corpus = load_json(raw_cases)
    assert set(corpus) == {'schema', 'cases'} and corpus['schema'] == 'gooo/body-composition-cases/v1'
    cases = corpus['cases']
    assert type(cases) is list and len(cases) == 21
    facts = [case_facts(case) for case in cases]
    states = dict(Counter(case['expected'][ENTRY]['status'] for case in cases))
    assert states == {'ELIGIBLE': 6, 'REFUSED': 6, 'INVALID_INPUT': 9}
    host = out / 'grant-update-native-host'
    host.mkdir()
    module = 'github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy'
    (host / 'go.mod').write_text('module private.invalid/gooo-grant-update-host\n\ngo 1.27.3\n\nrequire ' + module + ' v0.0.0\nreplace ' + module + ' => ' + json.dumps(str(ROOT)) + '\n')
    (host / 'main.go').write_text('''package main
import("encoding/json";"os";p "github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy")
func main(){var facts []p.GrantUpdateFacts;if err:=json.NewDecoder(os.Stdin).Decode(&facts);err!=nil{panic(err)};proofs:=make([]p.GrantUpdateProof,0,len(facts));for _,f:=range facts{v,err:=p.EvaluateGrantUpdate(f);if err!=nil{panic(err)};proofs=append(proofs,v)};if err:=json.NewEncoder(os.Stdout).Encode(proofs);err!=nil{panic(err)}}
''')
    native = run('native-grant-update', [args.go, 'run', '.'], encoded(facts), cwd=host)
    check_native(load_json(native), cases, bindings)
    workspace = REPO / 'service-grant-apply.workspace.json'
    program_path = ROOT / 'receipts/conditional-apply-program.json'
    report_path = ROOT / 'receipts/conditional-apply-body-codegen.json'
    program = load_json(program_path.read_bytes())
    report = load_json(report_path.read_bytes())['report']
    check_program_sources(program, load_json(workspace.read_bytes()), REPO)
    batches = []
    for offset in range(0, len(cases), 16):
        group = {'schema': corpus['schema'], 'cases': cases[offset:offset + 16]}
        label = 'grant-update-' + str(offset // 16)
        casefile = out / (label + '-cases.json')
        casefile.write_bytes(encoded(group))
        assert casefile.stat().st_size <= 32768
        raw = run(label + '-execute', [args.compiler, 'package', 'execute', '--json', '--cases', casefile,
                  '--go', args.go, workspace.name], cwd=REPO)
        replay = run(label + '-replay', [args.compiler, 'package', 'replay', '--json', '--receipt',
                     out / (label + '-execute.stdout'), '--cases', casefile, '--go', args.go, workspace.name], cwd=REPO)
        for document in [load_json(raw), load_json(replay)]:
            check_receipt(document, group, casefile, workspace, program, report, audit_runtime)
        check_replay_identity(raw, load_json(replay))
        batches.append({'case_count': len(group['cases']), 'cases': ident(casefile),
                        'execution': ident(out / (label + '-execute.stdout')),
                        'replay': ident(out / (label + '-replay.stdout'))})
    check_demo(run('grant-update-host-demo', [args.go, 'run', './cmd/apply-demo'], cwd=ROOT))
    return {'cases': 21, 'native_cases': 21, 'fields_compared': 7, 'states': states,
            'native': ident(out / 'native-grant-update.stdout'), 'workspace_batches': batches,
            'host_demo_cases': 4, 'host_demo': ident(out / 'grant-update-host-demo.stdout'),
            'all_raw_facts_and_strict_types': True, 'complete_typed_workspace_delivery_identity': True,
            'source_and_producer_bound': True, 'exact_replay_receipt_identity': True,
            'scope': 'Finite pure Gooo eligibility and in-memory host demo; no live authorization, durable store, or general safety claim.'}
