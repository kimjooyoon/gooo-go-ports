"""Render an internal Go kernel only from pinned genuine execution/replay bytes.

This checks finite consistency and identities, not unsigned producer authenticity.
No handwritten Go policy or selection implementation is emitted.
"""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
PIN = '205896a413e5dea92366309c35ae85e542d055f6'
ENTRY = 'gooo/service-policy/candidate-selection:SelectCandidate'
PREFIX = 'gooo://service-policy/candidate-selection/'
INPUTS = {'phase': 'Phase', 'context_id': 'ContextID', 'candidate_ids': 'CandidateIDs',
          'assessment_ids': 'AssessmentIDs', 'context_ids': 'ContextIDs',
          'statuses': 'Statuses', 'priorities': 'Priorities', 'bindings_verified': 'BindingsVerified'}
OUTPUTS = {'context_id': 'ContextID', 'state': 'State', 'reason': 'Reason',
           'selected_candidate_id': 'SelectedCandidateID', 'selected_assessment_id': 'SelectedAssessmentID',
           'selected_index': 'SelectedIndex', 'selected_priority': 'SelectedPriority',
           'candidate_count': 'CandidateCount', 'eligible_count': 'EligibleCount',
           'invalid_index': 'InvalidIndex', 'unknown_index': 'UnknownIndex',
           'execution_authorized': 'ExecutionAuthorized'}

def digest(raw):
    return 'sha256:' + hashlib.sha256(raw).hexdigest()

def strict(raw):
    def unique(rows):
        result = {}
        for name, value in rows:
            if name in result:
                raise ValueError('duplicate JSON field: ' + name)
            result[name] = value
        return result
    return json.loads(raw, object_pairs_hook=unique,
                      parse_constant=lambda x: (_ for _ in ()).throw(ValueError(x)))

def read(path):
    raw = path.read_bytes()
    if len(raw) > 16 << 20:
        raise ValueError('receipt/input exceeds finite renderer budget')
    return raw

def go_json_string(text):
    return json.dumps(text, ensure_ascii=False).replace('<', '\\u003c').replace('>', '\\u003e').replace('&', '\\u0026').replace('\u2028', '\\u2028').replace('\u2029', '\\u2029').encode()

def render(execution, replay, gofmt):
    cases_raw = read(HERE / 'kernel.cases.json')
    cases = strict(cases_raw)['cases']
    source_raw = read(ROOT / 'ports/service-policy/candidate_selection.gooo')
    manifest_raw = read(ROOT / 'grant-candidate-selection.workspace.json')
    parent = read(execution)
    first = None
    for path in (execution, replay):
        receipt = strict(read(path))
        runtime = receipt['result']['runtime']
        assert receipt['decision'] == 'PASS'
        assert receipt['manifest_digest'] == digest(manifest_raw) and receipt['cases_digest'] == digest(cases_raw)
        assert runtime['producer_source_sha'] == PIN and runtime['go_version'].startswith('go version go1.27.3 ')
        assert runtime['model_calls'] == 0 and runtime['finite_total'] == runtime['finite_passed'] == len(cases)
        packages = receipt['result']['program']['workspace_image']['packages']
        assert len(packages) == 1 and packages[0]['path'] == 'gooo/service-policy/candidate-selection'
        sources = packages[0]['sources']
        assert len(sources) == 1 and sources[0]['filename'] == 'ports/service-policy/candidate_selection.gooo'
        assert sources[0]['source_digest'] == digest(go_json_string(source_raw.decode()))
        assert len(runtime['traces']) == len(cases)
        for index, (trace, case) in enumerate(zip(runtime['traces'], cases)):
            assert trace['case_index'] == index and len(trace['deliveries']) == 1
            delivery = trace['deliveries'][0]
            assert delivery['activity_id'] == PREFIX + 'select/v1'
            assert delivery['input'] == case['inputs'][ENTRY]
            actual, wanted = delivery['actual'], case['expected'][ENTRY]
            # Canonical JSON distinguishes bool/int and preserves every field.
            assert json.dumps(actual, sort_keys=True) == json.dumps(wanted, sort_keys=True)
            assert set(actual) == set(OUTPUTS) and actual['execution_authorized'] is False
        composition = receipt['result']['composition']
        if first is None:
            first = composition
        else:
            assert receipt['replayed_from_sha256'] == digest(parent) and composition == first
    raw, plan = first['source'], first['plan']
    assert raw.startswith('package main\n') and 'import ' not in raw
    records = {r['id']: r for r in plan['record_types']}
    inp, out = records[PREFIX+'input/v1'], records[PREFIX+'result/v1']
    assert {f['name'] for f in inp['fields']} == set(INPUTS)
    assert {f['name'] for f in out['fields']} == set(OUTPUTS)
    activity = next(a for a in plan['activities'] if a['id'] == PREFIX+'select/v1')
    assert activity['go_function'] == 'GoooComposedActivity0'
    primitive = {'urn:gooo:type:string': 'string', 'urn:gooo:type:integer': 'int64', 'urn:gooo:type:boolean': 'bool'}
    types = '// Code generated from the Gooo typed plan; DO NOT EDIT.\npackage selector\n'
    for name, record, fields in [('Input', inp, INPUTS), ('Decision', out, OUTPUTS)]:
        types += '\ntype '+name+' struct {\n'
        for field in record['fields']:
            typ = primitive[field['type_id']]
            if field.get('cardinality', 'one') == 'many':
                typ = '[]'+typ
            types += '\t'+fields[field['name']]+' '+typ+' `json:"'+field['name']+'"`\n'
        types += '}\n'
    bridge = '// Code generated from compiler-owned record/field identities; DO NOT EDIT.\npackage selector\n\nfunc Evaluate(input Input) Decision {\n\tnative := '+inp['go_name']+'{\n'
    for field in inp['fields']:
        bridge += '\t\t'+field['go_name']+': input.'+INPUTS[field['name']]+',\n'
    bridge += '\t}\n\tactual := '+activity['go_function']+'(native)\n\treturn Decision{\n'
    for field in out['fields']:
        bridge += '\t\t'+OUTPUTS[field['name']]+': actual.'+field['go_name']+',\n'
    bridge += '\t}\n}\n'
    produced = {'internal/selector/projection_generated.go': raw.replace('package main\n', 'package selector\n', 1).encode()}
    for name, value in [('types_generated.go', types), ('bridge_generated.go', bridge)]:
        r = subprocess.run([str(gofmt)], input=value.encode(), capture_output=True, timeout=15, check=True)
        assert not r.stderr
        produced['internal/selector/'+name] = r.stdout
    manifest = {'schema': 'gooo/grant-selection-kernel-generation/v1', 'compiler_source_sha': PIN,
        'go_version': 'go1.27.3', 'source': {'ports/service-policy/candidate_selection.gooo': digest(source_raw)},
        'workspace_sha256': digest(manifest_raw), 'cases_sha256': digest(cases_raw),
        'typed_plan_sha256': plan['typed_plan_sha256'], 'raw_projection_sha256': digest(raw.encode()),
        'projection_transform': 'Replace first package token main with selector; all other bytes unchanged.',
        'generated_assets': {name: digest(value) for name, value in produced.items()},
        'scope': 'Finite native/replay source-bound generation; not authenticated authority or host integration proof.'}
    produced['kernel-generation.json'] = (json.dumps(manifest, indent=2, sort_keys=True)+'\n').encode()
    return produced

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--execution', type=Path, required=True)
    parser.add_argument('--replay', type=Path, required=True)
    parser.add_argument('--gofmt', type=Path, required=True)
    parser.add_argument('--out', type=Path)
    parser.add_argument('--check', action='store_true')
    args = parser.parse_args()
    assert args.gofmt.is_absolute() and (args.check != (args.out is not None))
    produced = render(args.execution, args.replay, args.gofmt)
    if args.check:
        for name, raw in produced.items():
            assert (HERE/name).read_bytes() == raw, name
    else:
        assert args.out.is_absolute() and not args.out.exists()
        assert not args.out.resolve().is_relative_to(ROOT.resolve())
        args.out.mkdir()
        for name, raw in produced.items():
            p = args.out/name
            p.parent.mkdir(parents=True, exist_ok=True)
            p.write_bytes(raw)
    print(json.dumps({'status': 'FINITE_GOOO_KERNEL_RENDERED_OR_CHECKED', 'files': len(produced), 'compiler': PIN}))
