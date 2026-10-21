#!/usr/bin/env python3
# Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause.
"""Check finite original-policy observations against a separate Go reference."""
import argparse
import collections
import copy
import datetime
import hashlib
import json
from pathlib import Path
import uuid

D = Path(__file__).resolve().parent
ROOT = D.parents[1]
PINS = json.loads((ROOT / "upstream.json").read_bytes())
ENTRY = dict(package_path='app/deny-rule-scope', source='examples/deny-rule-scope/main.gooo',
             activity='DiagnoseDenyRule', inputs=['DenyRuleFacts'], output='DenyRuleDiagnosis')
PIN = PINS['compiler_revision']


def ref(p):
    raw = p.read_bytes()
    return dict(path=str(p), bytes=len(raw), sha256=hashlib.sha256(raw).hexdigest())


def digest(p):
    return 'sha256:' + ref(p)['sha256']


def load(p):
    def unique(rows):
        result = {}
        for key, value in rows:
            assert key not in result, 'duplicate JSON member'
            result[key] = value
        return result
    assert p.stat().st_size <= 8 << 20
    def bad_constant(value):
        raise ValueError('nonfinite JSON constant: ' + value)
    return json.loads(p.read_bytes(), object_pairs_hook=unique, parse_constant=bad_constant)


def same(a, b):
    assert type(a) is type(b), 'type differs'
    if type(a) is dict:
        assert a.keys() == b.keys(), 'field roster differs'
        for key in a:
            same(a[key], b[key])
    elif type(a) is list:
        assert len(a) == len(b), 'length differs'
        for left, right in zip(a, b):
            same(left, right)
    else:
        assert a == b, 'value differs'


def source_digest(p):
    text = json.dumps(p.read_text(), ensure_ascii=False, separators=(',', ':'))
    for char, escaped in [('<', '\\u003c'), ('>', '\\u003e'), ('&', '\\u0026'),
                          ('\u2028', '\\u2028'), ('\u2029', '\\u2029')]:
        text = text.replace(char, escaped)
    return 'sha256:' + hashlib.sha256(text.encode()).hexdigest()


FACTS = EXPECTED = EXEC = REPLAY = None
EXEC_PATH = REPLAY_PATH = INPUT_PATH = EXPECTED_PATH = None


def check(receipt, replay=False):
    assert receipt['decision'] == 'OBSERVED'
    assert receipt['manifest_digest'] == digest(ROOT / 'deny-rule-scope.workspace.json')
    assert receipt['inputs_digest'] == digest(INPUT_PATH)
    if replay:
        assert receipt['replayed_from_sha256'] == digest(EXEC_PATH)
    image = receipt['result']['program']['workspace_image']
    same(image['entry'], ENTRY)
    assert len(image['packages']) == 1
    package = image['packages'][0]
    assert package['path'] == ENTRY['package_path']
    assert len(package['sources']) == 1
    source = package['sources'][0]
    assert source['filename'] == 'examples/deny-rule-scope/main.gooo'
    assert source['source_digest'] == source_digest(D / 'main.gooo')
    runtime = receipt['result']['runtime']
    assert runtime['stage'] == 'COMPLETE'
    same(runtime['model_calls'], 0)
    assert runtime['producer_source_sha'] == PIN
    version = runtime['go_version'].split()
    assert len(version) == 4 and version[:3] == ['go', 'version', PINS['go_version']]
    same(runtime['projection_replayed'], True)
    same(runtime['runtime_replayed'], True)
    assert len(runtime['runs']) == 2
    for process in [runtime['toolchain'], runtime['build'], *runtime['runs']]:
        for name, expected in [('started', True), ('completed', True), ('canceled', False),
                               ('timed_out', False), ('output_truncated', False), ('exit_code', 0)]:
            same(process[name], expected)
    assert len(runtime['traces']) == len(FACTS) == len(EXPECTED) == 87
    for index, trace in enumerate(runtime['traces']):
        same(trace['case_index'], index)
        assert len(trace['deliveries']) == 1
        delivery = trace['deliveries'][0]
        assert delivery['activity_id'] == 'gooo://deny-rule-scope/diagnose/v1'
        same(delivery['input'], FACTS[index])
        same(delivery['actual'], EXPECTED[index])
        for field_name, entity, values in [('input_fields', 'DenyRuleFacts', FACTS[index]),
                                            ('actual_fields', 'DenyRuleDiagnosis', EXPECTED[index])]:
            fields = delivery[field_name]
            assert len(fields) == len(values)
            assert len({f['name'] for f in fields}) == len(values)
            same({f['name']: f['value'] for f in fields}, values)
            for f in fields:
                assert f['id'] == 'gooo://deny-rule-scope/' + entity + '/v1/' + f['name']


def validate_identity(path):
    identity = load(path)
    assert identity['compiler_source_sha'] == PIN and identity['source_status'] == 'CLEAN_VCS'
    assert identity['vcs_modified'] == 'false' and identity['go_version'] == PINS['go_version']
    assert identity['native_go_required'] == PINS['go_version']
    assert identity['version'] == PINS['compiler_version']
    return identity


def main():
    global FACTS, EXPECTED, EXEC_PATH, REPLAY_PATH, EXEC, REPLAY, INPUT_PATH, EXPECTED_PATH
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--identity', type=Path, required=True)
    parser.add_argument('--identity-only', action='store_true')
    for name in ['execution', 'replay', 'expected', 'generated-inputs', 'out', 'go']:
        parser.add_argument('--' + name, type=Path)
    args = parser.parse_args()
    identity = validate_identity(args.identity)
    if args.identity_only:
        print(json.dumps({'status': 'PINNED_COMPILER_PROFILE_MATCHED', 'compiler': PIN,
                          'go': PINS['go_version']}))
        return
    assert all(getattr(args, name) is not None for name in ['execution', 'replay', 'expected', 'generated_inputs', 'out', 'go'])
    FACTS = load(D / 'case-facts.json')
    EXPECTED_PATH = args.expected
    EXPECTED = load(EXPECTED_PATH)
    EXEC_PATH, REPLAY_PATH = args.execution, args.replay
    EXEC, REPLAY = load(EXEC_PATH), load(REPLAY_PATH)
    INPUT_PATH = args.generated_inputs / 'inputs.json'
    generated = load(INPUT_PATH)
    assert generated['schema'] == 'gooo/body-composition-inputs/v1'
    same(generated['inputs'], [{'app/deny-rule-scope:DiagnoseDenyRule': row} for row in FACTS])
    same(load(args.generated_inputs / 'oracle-inputs.json'), FACTS)
    origin = load(args.generated_inputs / 'generation.provenance.jsonld')
    assert origin['family'] == 'gooo/deny-rule-input-projection/v1'
    graph = origin['@graph']
    assert len(graph) == 6 and len({node['@id'] for node in graph}) == 6
    for node in graph:
        assert type(node['@id']) is str and node['@id'].startswith('urn:uuid:')
        uuid.UUID(node['@id'][9:])
    activities = [n for n in graph if n['@type'] == 'prov:Activity']
    assert len(activities) == 1
    entities = [n for n in graph if n['@type'] == 'prov:Entity']
    sources = [n for n in entities if 'prov:wasGeneratedBy' not in n]
    outputs = [n for n in entities if 'prov:wasGeneratedBy' in n]
    assert {n['path'] for n in sources} == {'examples/deny-rule-scope/case-facts.json', 'deny-rule-scope.workspace.json', 'examples/deny-rule-scope/make-inputs.py'}
    assert {n['path'] for n in outputs} == {'inputs.json', 'oracle-inputs.json'}
    assert activities[0]['prov:used'] == [{'@id': n['@id']} for n in sources]
    for node in entities:
        path = (ROOT if node in sources else args.generated_inputs) / node['path']
        assert node['bytes'] == path.stat().st_size and node['sha256'] == ref(path)['sha256']
        if node in outputs:
            assert node['prov:wasGeneratedBy'] == {'@id': activities[0]['@id']}
    assert args.out.is_absolute() and not args.out.exists() and not args.out.resolve().is_relative_to(ROOT.resolve())
    check(EXEC)
    check(REPLAY, True)
    for receipt in [EXEC, REPLAY]:
        assert receipt['result']['runtime']['go_tool_sha256'] == digest(args.go)
    labels = load(D / 'case-labels.json')
    for label in ['negative', 'over-bound', 'missing-partition', 'overlap-partition', 'empty-surface', 'empty-rule']:
        assert EXPECTED[labels.index(label)]['status'] == 'INVALID_FACTS'
    assert EXPECTED[labels.index('at-bound')]['status'] == 'MATCHES_PRIOR_DENIALS'
    statuses = collections.Counter(row['status'] for row in EXPECTED)
    assert set(statuses) == {'INVALID_FACTS', 'UNRESOLVED_BACKEND', 'OUTSIDE_ENFORCEABLE_SURFACE',
                             'UNRESOLVED_INVENTORY', 'UNMATCHED_DENY', 'MATCHES_PRIOR_DENIALS',
                             'MATCHES_CURRENT_SURFACE'}
    # Assert the product invariant separately from the Go oracle implementation.
    for facts, result in zip(FACTS, EXPECTED):
        if result['warn_unmatched']:
            assert facts['inventory_complete'] and facts['backend_known'] and facts['backend_can_enforce']
            assert facts['prepolicy_matches'] == 0
        if not facts['inventory_complete'] or not facts['backend_known'] or not facts['backend_can_enforce']:
            assert result['warn_unmatched'] is False
    mutations = []
    for label in ['source', 'input', 'decision', 'boolean-type', 'missing-case', 'unfinished-run',
                  'metadata-boolean-type', 'model-count-type', 'replay-parent']:
        replay = label == 'replay-parent'
        altered = copy.deepcopy(REPLAY if replay else EXEC)
        if label == 'source':
            altered['result']['program']['workspace_image']['packages'][0]['sources'][0]['source_digest'] = 'sha256:' + '0' * 64
        elif label == 'input':
            altered['result']['runtime']['traces'][0]['deliveries'][0]['input']['rule_id'] += ':altered'
        elif label == 'decision':
            altered['result']['runtime']['traces'][0]['deliveries'][0]['actual']['status'] = 'UNMATCHED_DENY'
        elif label == 'boolean-type':
            altered['result']['runtime']['traces'][0]['deliveries'][0]['actual']['warn_unmatched'] = 0
        elif label == 'missing-case':
            altered['result']['runtime']['traces'].pop()
        elif label == 'unfinished-run':
            altered['result']['runtime']['runs'][0]['completed'] = False
        elif label == 'metadata-boolean-type':
            altered['result']['runtime']['runs'][0]['completed'] = 1
        elif label == 'model-count-type':
            altered['result']['runtime']['model_calls'] = False
        else:
            altered['replayed_from_sha256'] = 'sha256:' + '0' * 64
        # Python equates False and 0; compare the actual serialized mutation so
        # a type-changing control is recognized before strict typed validation.
        assert json.dumps(altered, sort_keys=True) != json.dumps(REPLAY if replay else EXEC, sort_keys=True), label
        try:
            check(altered, replay)
        except AssertionError:
            mutations.append(dict(label=label, effective_mutation=True, refused=True))
        else:
            raise AssertionError('mutation accepted: ' + label)
    result = dict(status='FINITE_ORIGINAL_GOOO_SCOPE_POLICY_NATIVE_AND_REPLAY_MATCHED_GO_REFERENCE',
                  at_utc=datetime.datetime.now(datetime.timezone.utc).isoformat(),
                  native_cases=87, saved_replay_cases=87, fields_per_result=12,
                  status_counts=dict(statuses), receipt_mutation_refusals=mutations,
                  compiler=PIN, go=PINS['go_version'], source=ref(D / 'main.gooo'),
                  execution=ref(EXEC_PATH), replay=ref(REPLAY_PATH),
                  independent_reference=ref(EXPECTED_PATH),
                  independent_reference_source=ref(D / 'reference/main.go'),
                  selected_go_tool=ref(args.go),
                  input_generation=ref(args.generated_inputs / 'generation.provenance.jsonld'),
                  source_and_all_typed_facts_and_decisions_bound=True,
                  model_calls=0, external_user_adoption=False,
                  source_modified_by_verifier=False,
                  scope='Original policy over supplied synthetic scope/count facts. Does not discover inventories, match raw rules, establish partition truth or enforce permissions. Not an OpenClaw fix or runtime integration.')
    with args.out.open('x') as f:
        json.dump(result, f, indent=2)
        f.write('\n')
    print(json.dumps(dict(status=result['status'], cases=87, mutation_refusals=len(mutations),
                         classification_counts=dict(statuses))))


if __name__ == '__main__':
    main()
