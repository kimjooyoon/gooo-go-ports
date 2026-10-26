"""Finite synthetic inputs and hand-authored anchors; no admission-rule execution."""
from copy import deepcopy
from pathlib import Path
import argparse
import json

HERE = Path(__file__).resolve().parent.parent
PROFILE = 'gooo/mcp-tool-definition-json/v1'
VALIDATOR = 'gooo/mcp-tool-shape/v1'
DIGEST = 'sha256:' + 'a' * 64
ANNOTATION = 'sha256:' + 'b' * 64
CHANGED = 'sha256:' + 'c' * 64


def facts(names=('lookup',), pages=1):
    return {
        'page_request_cursor_kinds': ['ABSENT'] + ['STRING'] * (pages - 1) if pages else [],
        'page_request_cursor_values': [''] + ['p' + str(i) for i in range(1, pages)] if pages else [],
        'page_next_cursor_kinds': ['STRING'] * (pages - 1) + ['ABSENT'] if pages else [],
        'page_next_cursor_values': ['p' + str(i) for i in range(1, pages)] + [''] if pages else [],
        'original_names': list(names), 'definition_ids': [DIGEST] * len(names),
        'definition_profiles': [PROFILE] * len(names), 'annotation_ids': [ANNOTATION] * len(names),
        'row_page_indexes': [0] * len(names),
    }


def evidence(f=None):
    if f is None:
        f = facts()
    return {'attempt_id': 'attempt:7', 'candidate_id': DIGEST, 'collector_state': 'SUCCESS',
            'binding_state': 'BOUND', 'validator_profile': VALIDATOR,
            'row_definition_ids': deepcopy(f['definition_ids']),
            'row_validation_states': ['ACCEPTED'] * len(f['original_names'])}


def state():
    return {'expected_registry_version': 'registry:3', 'current_registry_version': 'registry:3',
            'expected_scope_epoch': 'scope:2', 'current_scope_epoch': 'scope:2',
            'expected_invalidation_epoch': 'invalidation:4', 'current_invalidation_epoch': 'invalidation:4',
            'expected_latest_attempt': 'attempt:7', 'current_latest_attempt': 'attempt:7'}


CASES = []
BOUNDARIES = []


def add(label, reason, *, snapshot=None, ev=None, st=None, coverage='COMPLETE',
        coverage_reason='OBSERVED_PAGE_CHAIN_COMPLETE', problem_page_index=-1,
        problem_row_index=-1, boundary=None, **anchors):
    f = deepcopy(facts() if snapshot is None else snapshot)
    e = deepcopy(evidence(f) if ev is None else ev)
    s = deepcopy(state() if st is None else st)
    literal = {'state': 'CANDIDATE_ELIGIBLE' if reason == 'COMPLETE_CANDIDATE' else 'RETAIN_STORED_CATALOG',
               'reason': reason, 'coverage': coverage, 'coverage_reason': coverage_reason,
               'problem_page_index': problem_page_index, 'problem_row_index': problem_row_index,
               'candidate_page_count': len(f['page_request_cursor_kinds']),
               'candidate_row_count': len(f['original_names']),
               'execution_authorized': False, 'live_effect_performed': False, 'remote_atomicity': 'UNKNOWN'}
    literal.update(anchors)
    CASES.append({'label': label, 'snapshot': f, 'evidence': e, 'state': s, 'anchors': literal})
    if boundary:
        port, record, field, element, kind, actual, limit = boundary
        BOUNDARIES.append({'reference_case_index': len(CASES) - 1, 'label': label, 'input_port': port,
                           'record': record, 'field': field, 'element_index': element, 'boundary': kind,
                           'observed_size': actual, 'limit': limit, 'stage': 'INPUT_VALIDATION',
                           'code': 'INVALID_INPUT_VALUE'})


def invalid(label, f, detail='INVALID_ROW_FACT', row=0, page=-1, boundary=None):
    add(label, 'INVALID_SNAPSHOT', snapshot=f, coverage='INVALID', coverage_reason=detail,
        problem_row_index=row, problem_page_index=page, boundary=boundary)


add('complete-one-page', 'COMPLETE_CANDIDATE')
f = facts(('lookup', 'write', 'archive'), 3); f['row_page_indexes'] = [0, 1, 2]
f['definition_ids'] = [DIGEST, CHANGED, ANNOTATION]
add('complete-three-pages-all-rows', 'COMPLETE_CANDIDATE', snapshot=f)
add('terminal-empty-catalog', 'COMPLETE_CANDIDATE', snapshot=facts(()))
f = facts(()); f['page_next_cursor_kinds'][0] = 'NULL'
add('null-terminal-empty-catalog', 'COMPLETE_CANDIDATE', snapshot=f)
add('no-pages-is-not-empty-catalog', 'INCOMPLETE_CATALOG', snapshot=facts((), 0), coverage='INCOMPLETE', coverage_reason='NO_SAVED_PAGES')
f = facts(); f['page_next_cursor_kinds'][0] = 'STRING'; f['page_next_cursor_values'][0] = 'p1'
add('known-selected-row-partial-is-not-candidate', 'INCOMPLETE_CATALOG', snapshot=f, coverage='INCOMPLETE', coverage_reason='MISSING_TERMINAL_PAGE', problem_page_index=0)
for collector in ('ERROR', 'TIMEOUT', 'CANCELLED'):
    e = evidence(f); e['collector_state'] = collector
    add('page-two-' + collector.lower(), 'COLLECTOR_' + collector, snapshot=f, ev=e, coverage='INCOMPLETE', coverage_reason='MISSING_TERMINAL_PAGE', problem_page_index=0)
    e = evidence(); e['collector_state'] = collector
    add('complete-facts-collector-' + collector.lower(), 'COLLECTOR_' + collector, ev=e)
f = facts(); f['page_request_cursor_kinds'][0] = 'STRING'; f['page_request_cursor_values'][0] = 'middle'
add('missing-initial-page', 'INCOMPLETE_CATALOG', snapshot=f, coverage='INCOMPLETE', coverage_reason='MISSING_INITIAL_PAGE', problem_page_index=0)
f = facts(pages=2); f['page_request_cursor_values'][1] = 'wrong'
add('broken-page-link', 'INCOMPLETE_CATALOG', snapshot=f, coverage='INCOMPLETE', coverage_reason='CURSOR_LINK_MISMATCH', problem_page_index=1)
for terminal in ('ABSENT', 'NULL'):
    f = facts(pages=2); f['page_next_cursor_kinds'][0] = terminal; f['page_next_cursor_values'][0] = ''
    add('page-after-' + terminal.lower(), 'INCOMPLETE_CATALOG', snapshot=f, coverage='INCOMPLETE', coverage_reason='PAGE_AFTER_TERMINAL', problem_page_index=1)
f = facts(pages=2); f['page_next_cursor_values'][0] = ''; f['page_request_cursor_values'][1] = ''; f['row_page_indexes'][0] = 1
add('empty-string-cursor-links-page', 'COMPLETE_CANDIDATE', snapshot=f)
f = facts(); f['page_next_cursor_kinds'][0] = 'STRING'
add('empty-string-next-needs-another-page', 'INCOMPLETE_CATALOG', snapshot=f, coverage='INCOMPLETE', coverage_reason='MISSING_TERMINAL_PAGE', problem_page_index=0)
f = facts(pages=3); f['page_next_cursor_values'] = ['repeat', 'repeat', '']; f['page_request_cursor_values'] = ['', 'repeat', 'repeat']
add('opaque-repeated-cursor-links', 'COMPLETE_CANDIDATE', snapshot=f)
for label, names, pages, indexes, problem in [
    ('duplicate-name', ('lookup', 'lookup'), 1, [0, 0], 1),
    ('non-selected-name-duplicate', ('lookup', 'other', 'other'), 1, [0, 0, 0], 2),
    ('duplicate-across-pages', ('lookup', 'lookup'), 2, [0, 1], 1),
    ('earliest-second-duplicate', ('a', 'b', 'b', 'a'), 1, [0] * 4, 2),
]:
    f = facts(names, pages); f['row_page_indexes'] = indexes
    add(label, 'DUPLICATE_NAME', snapshot=f, problem_row_index=problem)
f = facts(('lookup', 'other')); f['definition_profiles'][1] = 'unknown/profile/v1'
add('non-selected-row-unsupported-profile', 'UNSUPPORTED_DEFINITION_PROFILE', snapshot=f, problem_row_index=1)
e = evidence(); e['validator_profile'] = 'unknown/validator/v1'
add('unsupported-validator', 'UNSUPPORTED_VALIDATOR_PROFILE', ev=e)
e = evidence(facts(())); e['validator_profile'] = 'unknown/validator/v1'
add('empty-catalog-still-requires-validator', 'UNSUPPORTED_VALIDATOR_PROFILE', snapshot=facts(()), ev=e)
for field in ('row_definition_ids', 'row_validation_states'):
    e = evidence(); e[field] = []
    add('missing-' + field, 'ROW_VALIDATION_LENGTH', ev=e)
    e = evidence(); e[field].append(e[field][0])
    add('extra-' + field, 'ROW_VALIDATION_LENGTH', ev=e)
e = evidence(facts(())); e['row_validation_states'] = ['ACCEPTED']
add('empty-catalog-rejects-extra-outcome', 'ROW_VALIDATION_LENGTH', snapshot=facts(()), ev=e)
for value, label in [(CHANGED, 'different'), ('', 'empty'), ('sha256:' + 'A' * 64, 'uppercase')]:
    e = evidence(); e['row_definition_ids'][0] = value
    add('row-binding-' + label, 'ROW_VALIDATION_BINDING_MISMATCH', ev=e, problem_row_index=0)
f = facts(('lookup', 'write')); f['definition_ids'] = [DIGEST, CHANGED]
e = evidence(f); e['row_definition_ids'].reverse()
add('row-bindings-reordered', 'ROW_VALIDATION_BINDING_MISMATCH', snapshot=f, ev=e, problem_row_index=0)
for outcome, reason in [('REJECTED', 'ROW_REJECTED'), ('UNKNOWN', 'ROW_UNASSESSED'), ('accepted', 'INVALID_ROW_VALIDATION_STATE'), ('', 'INVALID_ROW_VALIDATION_STATE'), ('bad\0state', 'INVALID_ROW_VALIDATION_STATE')]:
    f = facts(('lookup', 'other')); e = evidence(f); e['row_validation_states'][1] = outcome
    add('non-selected-row-outcome-' + (outcome.lower().replace('\0', '-') or 'empty'), reason, snapshot=f, ev=e, problem_row_index=1)
for field, reason in [('current_registry_version', 'REGISTRY_CHANGED'), ('current_scope_epoch', 'SCOPE_CHANGED'), ('current_invalidation_epoch', 'INVALIDATED'), ('current_latest_attempt', 'SUPERSEDED_ATTEMPT')]:
    s = state(); s[field] = 'new:token'
    add('stale-' + field, reason, st=s)
s = state(); s['current_latest_attempt'] = 'newer:failed-attempt'
add('newer-attempt-failed-older-still-superseded', 'SUPERSEDED_ATTEMPT', st=s)
s = state(); s['expected_latest_attempt'] = s['current_latest_attempt'] = 'other:attempt'
add('attempt-evidence-binding-mismatch', 'ATTEMPT_BINDING_MISMATCH', st=s)
e = evidence(); e['binding_state'] = 'UNBOUND'
add('unbound-candidate', 'UNBOUND_CANDIDATE', ev=e)
e = evidence(facts(())); e['binding_state'] = 'UNBOUND'
add('empty-catalog-still-requires-binding', 'UNBOUND_CANDIDATE', snapshot=facts(()), ev=e)
for field, values in [('attempt_id', [('', 'empty'), ('bad\0attempt', 'nul')]), ('candidate_id', [('', 'empty'), ('sha256:ab', 'short'), ('sha256:' + 'A' * 64, 'uppercase'), ('sha256:' + 'g' * 64, 'nonhex')]), ('collector_state', [('success', 'unknown'), ('', 'empty')]), ('binding_state', [('bound', 'unknown')]), ('validator_profile', [('', 'empty'), ('bad\0profile', 'nul')])]:
    for value, label in values:
        e = evidence(); e[field] = value
        add('evidence-' + field + '-' + label, 'INVALID_REFRESH_EVIDENCE', ev=e)
for field in state():
    s = state(); s[field] = ''
    add('state-' + field + '-empty', 'INVALID_REFRESH_STATE', st=s)
s = state(); s['current_scope_epoch'] = 'bad\0scope'
add('state-token-nul', 'INVALID_REFRESH_STATE', st=s)
for field in ('page_request_cursor_values', 'page_next_cursor_kinds', 'page_next_cursor_values'):
    f = facts(); f[field] = []
    invalid('page-length-' + field, f, 'PAGE_COLUMN_LENGTH', -1)
for field in ('definition_ids', 'definition_profiles', 'annotation_ids', 'row_page_indexes'):
    f = facts(); f[field] = []
    invalid('row-length-' + field, f, 'ROW_COLUMN_LENGTH', -1)
for field, value, label in [('page_request_cursor_kinds', 'NULL', 'null-request'), ('page_next_cursor_kinds', 'UNKNOWN', 'unknown-next'), ('page_request_cursor_values', 'extra', 'absent-request-value'), ('page_next_cursor_values', 'extra', 'absent-next-value')]:
    f = facts(); f[field][0] = value
    invalid(label, f, 'INVALID_CURSOR_TAG_OR_VALUE', -1, 0)
for field, value, label in [('original_names', '', 'empty-name'), ('original_names', 'bad\0name', 'nul-name'), ('definition_ids', 'sha256:' + 'A' * 64, 'uppercase-definition'), ('definition_ids', 'sha256:ab', 'short-definition'), ('annotation_ids', 'sha256:' + 'g' * 64, 'nonhex-annotation'), ('definition_profiles', '', 'empty-definition-profile'), ('row_page_indexes', -1, 'negative-page'), ('row_page_indexes', 1, 'out-of-range-page')]:
    f = facts(('lookup', 'other')); f[field][1] = value
    invalid('non-selected-row-' + label, f, row=1)
f = facts(('lookup', 'other'), 2); f['row_page_indexes'] = [1, 0]
invalid('row-page-order', f, row=1)
f = facts(pages=0)
invalid('rows-without-pages', f)
f = facts(pages=16); f['row_page_indexes'][0] = 15
add('sixteen-pages-boundary', 'COMPLETE_CANDIDATE', snapshot=f)
invalid('seventeen-pages-refused', facts(pages=17), 'SNAPSHOT_LIMIT', -1)
f = facts(tuple('tool-' + str(i) for i in range(256)))
add('two-hundred-fifty-six-rows', 'COMPLETE_CANDIDATE', snapshot=f)
f = facts(('x' * 1024,))
add('name-1024-byte-boundary', 'COMPLETE_CANDIDATE', snapshot=f)
f = facts(pages=2); f['page_next_cursor_values'][0] = f['page_request_cursor_values'][1] = 'x' * 1024
add('cursor-1024-byte-boundary', 'COMPLETE_CANDIDATE', snapshot=f)
e = evidence(); e['attempt_id'] = 'x' * 1024
s = state(); s['expected_latest_attempt'] = s['current_latest_attempt'] = e['attempt_id']
add('attempt-1024-byte-boundary', 'COMPLETE_CANDIDATE', ev=e, st=s)
# Typed-input admission failures remain in full domain-reference cases. Only the
# separate native subset excludes them; each omission names its exact boundary.
for field, detail, row, page in [('original_names', 'INVALID_ROW_FACT', 0, -1), ('definition_profiles', 'INVALID_ROW_FACT', 0, -1), ('page_next_cursor_values', 'INVALID_CURSOR_TAG_OR_VALUE', -1, 0)]:
    f = facts(); f[field][0] = 'x' * 1025
    if field == 'page_next_cursor_values': f['page_next_cursor_kinds'][0] = 'STRING'
    invalid('typed-' + field + '-1025', f, detail, row, page, ('input0', 'SnapshotFacts', field, 0, 'text_utf8_bytes', 1025, 1024))
f = facts(tuple('tool-' + str(i) for i in range(257)))
invalid('typed-257-rows', f, 'SNAPSHOT_LIMIT', -1, boundary=('input0', 'SnapshotFacts', 'original_names', None, 'collection_elements', 257, 256))
for field, reason in [('attempt_id', 'INVALID_REFRESH_EVIDENCE'), ('validator_profile', 'INVALID_REFRESH_EVIDENCE')]:
    e = evidence(); e[field] = 'x' * 1025
    add('typed-evidence-' + field + '-1025', reason, ev=e, boundary=('input1', 'RefreshEvidence', field, None, 'text_utf8_bytes', 1025, 1024))
e = evidence(); e['row_validation_states'][0] = 'x' * 1025
add('typed-row-outcome-1025', 'INVALID_ROW_VALIDATION_STATE', ev=e, problem_row_index=0, boundary=('input1', 'RefreshEvidence', 'row_validation_states', 0, 'text_utf8_bytes', 1025, 1024))
e = evidence(); e['row_validation_states'] = ['ACCEPTED'] * 257
add('typed-257-row-outcomes', 'ROW_VALIDATION_LENGTH', ev=e, boundary=('input1', 'RefreshEvidence', 'row_validation_states', None, 'collection_elements', 257, 256))
s = state(); s['current_scope_epoch'] = 'x' * 1025
add('typed-state-token-1025', 'INVALID_REFRESH_STATE', st=s, boundary=('input2', 'RefreshState', 'current_scope_epoch', None, 'text_utf8_bytes', 1025, 1024))
# Precedence controls combine failures so a passing implementation cannot merely
# have a branch for each isolated reason and still report the wrong cause.
e = evidence(); e['binding_state'] = 'UNBOUND'; e['attempt_id'] = ''
s = state(); s['current_registry_version'] = ''
add('evidence-before-unbound-before-state', 'INVALID_REFRESH_EVIDENCE', ev=e, st=s)
e = evidence(); e['binding_state'] = 'UNBOUND'
add('unbound-before-invalid-state', 'UNBOUND_CANDIDATE', ev=e, st=s)
e = evidence(); e['collector_state'] = 'ERROR'; e['validator_profile'] = 'unknown/v1'
add('collector-before-unsupported-validator', 'COLLECTOR_ERROR', ev=e)
f = facts(); f['annotation_ids'][0] = ''
e = evidence(f); e['validator_profile'] = 'unknown/v1'
add('unsupported-validator-before-invalid-snapshot', 'UNSUPPORTED_VALIDATOR_PROFILE', snapshot=f, ev=e, coverage='INVALID', coverage_reason='INVALID_ROW_FACT', problem_row_index=0)
f = facts(('lookup', 'lookup')); f['page_next_cursor_kinds'][0] = 'STRING'; f['page_next_cursor_values'][0] = 'more'
add('incomplete-before-duplicate', 'INCOMPLETE_CATALOG', snapshot=f, coverage='INCOMPLETE', coverage_reason='MISSING_TERMINAL_PAGE', problem_page_index=0, problem_row_index=1)
f = facts(('lookup', 'other')); f['definition_profiles'][1] = 'unknown/v1'
e = evidence(f); e['row_validation_states'] = []
add('unsupported-row-profile-before-outcome-length', 'UNSUPPORTED_DEFINITION_PROFILE', snapshot=f, ev=e, problem_row_index=1)
f = facts(('lookup', 'other')); e = evidence(f); e['row_definition_ids'][1] = CHANGED; e['row_validation_states'][0] = 'REJECTED'
add('later-binding-before-earlier-rejection', 'ROW_VALIDATION_BINDING_MISMATCH', snapshot=f, ev=e, problem_row_index=1)
e = evidence(f); e['row_validation_states'] = ['UNKNOWN', 'REJECTED']
add('first-outcome-problem-retained', 'ROW_UNASSESSED', snapshot=f, ev=e, problem_row_index=0)
s = state(); s['current_registry_version'] = 'changed'; s['current_scope_epoch'] = 'changed'; s['current_invalidation_epoch'] = 'changed'; s['current_latest_attempt'] = 'changed'
add('registry-before-other-stale-tokens', 'REGISTRY_CHANGED', st=s)
e = evidence(); e['row_validation_states'][0] = 'REJECTED'
add('row-rejection-before-stale-host-state', 'ROW_REJECTED', ev=e, st=s, problem_row_index=0)
f = facts(('lookup', 'other')); f['definition_profiles'][1] = 'unknown/v1'
e = evidence(f); e['binding_state'] = 'UNBOUND'
add('metadata-refusal-preserves-candidate-diagnostic', 'UNBOUND_CANDIDATE', snapshot=f, ev=e, problem_row_index=1)


# Immutable source identities of the accepted compiler's typed-input admission.
ENGINE_VALIDATION_SOURCES = [{'path': 'internal/bodyexecution/composition_cases.go', 'sha256': 'sha256:aef6187afcd541db68891c52f0795c3cd8cde3e50e0a5887f9195c9486c4533a', 'url': 'https://github.com/kimjooyoon/meta-ontology-go/blob/205896a413e5dea92366309c35ae85e542d055f6/internal/bodyexecution/composition_cases.go'}, {'path': 'internal/bodyexecution/composition_records.go', 'sha256': 'sha256:575768130c0ec3b103a8c861a6ade2f7bea8487d2a62de67a24463f20cda10e2', 'url': 'https://github.com/kimjooyoon/meta-ontology-go/blob/205896a413e5dea92366309c35ae85e542d055f6/internal/bodyexecution/composition_records.go'}, {'path': 'internal/bodyexecution/composition_collections.go', 'sha256': 'sha256:90d72a6592e420de286d10100edf4002cefdd4da96b62d15af7b3d0fb63e05f1', 'url': 'https://github.com/kimjooyoon/meta-ontology-go/blob/205896a413e5dea92366309c35ae85e542d055f6/internal/bodyexecution/composition_collections.go'}, {'path': 'internal/bodyexecution/composition_capacity.go', 'sha256': 'sha256:8146e93f42d733a888f6c12a0bf4ac7b902cb25563815ad791b0a5d4d86b0b46', 'url': 'https://github.com/kimjooyoon/meta-ontology-go/blob/205896a413e5dea92366309c35ae85e542d055f6/internal/bodyexecution/composition_capacity.go'}, {'path': 'internal/bodyexecution/text_json.go', 'sha256': 'sha256:ec2ae0f7d97fc402e8fd646fb4d526bd81f415b3f95e79646ba2d8721d39749b', 'url': 'https://github.com/kimjooyoon/meta-ontology-go/blob/205896a413e5dea92366309c35ae85e542d055f6/internal/bodyexecution/text_json.go'}, {'path': 'internal/valuejson/text.go', 'sha256': 'sha256:2b366a79dc6266f50de6a49b4de552cae52729bff7e4a1dc8e09868cc0535b74', 'url': 'https://github.com/kimjooyoon/meta-ontology-go/blob/205896a413e5dea92366309c35ae85e542d055f6/internal/valuejson/text.go'}, {'path': 'internal/syntax/entity_fields_support.go', 'sha256': 'sha256:6e1e90d20587d0ccd884670704c6ad8ce9a5198af3bf1200925accd1f52512fe', 'url': 'https://github.com/kimjooyoon/meta-ontology-go/blob/205896a413e5dea92366309c35ae85e542d055f6/internal/syntax/entity_fields_support.go'}]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--check', action='store_true')
    args = parser.parse_args()
    assert 80 <= len(CASES) <= 256
    assert len({c['label'] for c in CASES}) == len(CASES)
    boundaries = {'schema': 'gooo/catalog-refresh-input-boundaries/v1',
                  'compiler_revision': '205896a413e5dea92366309c35ae85e542d055f6',
                  'pinned_validation_sources': ENGINE_VALIDATION_SOURCES,
                  'reference_cases': len(CASES), 'native_cases': len(CASES) - len(BOUNDARIES),
                  'rejections': BOUNDARIES,
                  'scope': 'Exact typed-input admission probes. All remain in reference-inputs and kernel.cases with independent domain expectations. Exclusions from native cases are explicit; no body-level decision or native execution is claimed for them.'}
    documents = {
        'reference-inputs.json': {'schema': 'gooo/catalog-refresh-reference-inputs/v1', 'cases': CASES},
        'case-labels.json': {'origin': 'SYNTHETIC_FIXTURE', 'cases': [c['label'] for c in CASES],
                             'scope': 'Finite authored whole-candidate refresh obligations; no server calls, actual commits, or external adoption.'},
        'input-boundaries.json': boundaries,
    }
    for name, value in documents.items():
        raw = (json.dumps(value, ensure_ascii=False, indent=2) + '\n').encode()
        assert len(raw) <= 4 << 20
        if args.check:
            assert (HERE / name).read_bytes() == raw, name
        else:
            (HERE / name).write_bytes(raw)
    print(json.dumps({'cases': len(CASES), 'native_cases': len(CASES) - len(BOUNDARIES),
                      'typed_input_boundaries': len(BOUNDARIES), 'files': sorted(documents),
                      'domain_execution': False, 'check': args.check}))


if __name__ == '__main__':
    main()
