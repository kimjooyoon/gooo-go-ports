"""Author finite synthetic inputs and literal anchors; do not evaluate domain rules."""
from copy import deepcopy
from pathlib import Path
import argparse
import json

HERE = Path(__file__).resolve().parent.parent
PROFILE = 'gooo/mcp-tool-definition-json/v1'
DIGEST = 'sha256:' + 'a' * 64
ANNOTATION = 'sha256:' + 'b' * 64
CHANGED = 'sha256:' + 'c' * 64


def facts(names=('lookup',), pages=1):
    return {
        'page_request_cursor_kinds': ['ABSENT'] + ['STRING'] * (pages - 1) if pages else [],
        'page_request_cursor_values': [''] + ['p' + str(i) for i in range(1, pages)] if pages else [],
        'page_next_cursor_kinds': ['STRING'] * (pages - 1) + ['ABSENT'] if pages else [],
        'page_next_cursor_values': ['p' + str(i) for i in range(1, pages)] + [''] if pages else [],
        'original_names': list(names),
        'definition_ids': [DIGEST] * len(names),
        'definition_profiles': [PROFILE] * len(names),
        'annotation_ids': [ANNOTATION] * len(names),
        'row_page_indexes': [0] * len(names),
    }


CASES = []


def add(label, state, *, before=None, after=None, selection=None,
        before_coverage='COMPLETE', after_coverage='COMPLETE',
        definition='UNASSESSED', **anchors):
    literal = {
        'state': state,
        'before_page_coverage': before_coverage,
        'after_page_coverage': after_coverage,
        'definition_relation': definition,
    }
    literal.update(anchors)
    CASES.append({
        'label': label,
        'before': deepcopy(facts() if before is None else before),
        'after': deepcopy(facts() if after is None else after),
        'selection': deepcopy({'context_id': 'synthetic:snapshot-fixture', 'target_name': 'lookup'} if selection is None else selection),
        'anchors': literal,
    })


add('same-complete', 'OBSERVED_DEFINITION_SAME', definition='SAME_OBSERVED', annotation_relation='SAME_OBSERVED', profile_relation='SAME_OBSERVED', observed_presence_relation='OBSERVED_BOTH', before_index=0, after_index=0)
f = facts(); f['definition_ids'][0] = CHANGED
add('descriptor-changed', 'OBSERVED_DEFINITION_CHANGED', after=f, definition='DIFFERENT_OBSERVED', annotation_relation='SAME_OBSERVED')
f = facts(); f['annotation_ids'][0] = CHANGED
add('annotation-claims-changed', 'OBSERVED_ANNOTATIONS_CHANGED', after=f, definition='SAME_OBSERVED', annotation_relation='DIFFERENT_OBSERVED')
f = facts(); f['definition_ids'][0] = f['annotation_ids'][0] = CHANGED
add('descriptor-and-annotations-changed', 'OBSERVED_DEFINITION_CHANGED', after=f, definition='DIFFERENT_OBSERVED', annotation_relation='DIFFERENT_OBSERVED')
add('observed-added', 'OBSERVED_NAME_ADDED', before=facts(()), observed_presence_relation='OBSERVED_ADDED', before_index=-1, after_index=0)
add('observed-removed', 'OBSERVED_NAME_REMOVED', after=facts(()), observed_presence_relation='OBSERVED_REMOVED', before_index=0, after_index=-1)
add('absent-from-two-complete-empty-pages', 'OBSERVED_NAME_ABSENT', before=facts(()), after=facts(()), observed_presence_relation='ABSENT_BOTH_COMPLETE')
add('target-absent-unrelated-rows', 'OBSERVED_NAME_ABSENT', before=facts(('unrelated-before',)), after=facts(('unrelated-after',)), observed_presence_relation='ABSENT_BOTH_COMPLETE')
add('target-at-second-row', 'OBSERVED_DEFINITION_SAME', before=facts(('other', 'lookup')), after=facts(('lookup', 'other')), definition='SAME_OBSERVED', before_index=1, after_index=0)

empty = facts((), 0)
add('no-before-pages-not-added', 'UNASSESSED', before=empty, before_coverage='INCOMPLETE', before_coverage_reason='NO_SAVED_PAGES')
add('no-after-pages-not-removed', 'UNASSESSED', after=empty, after_coverage='INCOMPLETE', after_coverage_reason='NO_SAVED_PAGES')
add('no-pages-both-not-absent', 'UNASSESSED', before=empty, after=empty, before_coverage='INCOMPLETE', after_coverage='INCOMPLETE')
partial = facts(); partial['page_next_cursor_kinds'][0] = 'STRING'; partial['page_next_cursor_values'][0] = 'next'
add('known-before-row-with-missing-terminal', 'OBSERVED_DEFINITION_SAME', before=partial, before_coverage='INCOMPLETE', definition='SAME_OBSERVED', before_coverage_reason='MISSING_TERMINAL_PAGE', before_problem_page_index=0)
add('known-after-row-with-missing-terminal', 'OBSERVED_DEFINITION_SAME', after=partial, after_coverage='INCOMPLETE', definition='SAME_OBSERVED')
add('known-both-rows-partial', 'OBSERVED_DEFINITION_SAME', before=partial, after=partial, before_coverage='INCOMPLETE', after_coverage='INCOMPLETE', definition='SAME_OBSERVED', comparison_scope='SAVED_ROWS_ONLY')
empty_partial = facts(()); empty_partial['page_next_cursor_kinds'][0] = 'STRING'; empty_partial['page_next_cursor_values'][0] = 'next'
add('missing-before-target-partial-not-added', 'UNASSESSED', before=empty_partial, before_coverage='INCOMPLETE')
add('missing-after-target-partial-not-removed', 'UNASSESSED', after=empty_partial, after_coverage='INCOMPLETE')
add('both-targets-missing-one-partial-not-absent', 'UNASSESSED', before=facts(()), after=empty_partial, after_coverage='INCOMPLETE')
add('added-needs-only-absence-side-complete', 'OBSERVED_NAME_ADDED', before=facts(()), after=partial, after_coverage='INCOMPLETE', observed_presence_relation='OBSERVED_ADDED')
add('removed-needs-only-absence-side-complete', 'OBSERVED_NAME_REMOVED', before=partial, after=facts(()), before_coverage='INCOMPLETE', observed_presence_relation='OBSERVED_REMOVED')
f = deepcopy(partial); f['definition_ids'][0] = CHANGED
add('known-change-survives-incomplete-pages', 'OBSERVED_DEFINITION_CHANGED', before=partial, after=f, before_coverage='INCOMPLETE', after_coverage='INCOMPLETE', definition='DIFFERENT_OBSERVED')

f = facts(); f['page_request_cursor_kinds'][0] = 'STRING'; f['page_request_cursor_values'][0] = 'middle'
add('known-rows-with-no-initial-page', 'OBSERVED_DEFINITION_SAME', before=f, before_coverage='INCOMPLETE', definition='SAME_OBSERVED', before_coverage_reason='MISSING_INITIAL_PAGE', before_problem_page_index=0)
f = facts(pages=2); f['page_request_cursor_values'][1] = 'wrong'
add('broken-link-known-rows', 'OBSERVED_DEFINITION_SAME', before=f, before_coverage='INCOMPLETE', definition='SAME_OBSERVED', before_coverage_reason='CURSOR_LINK_MISMATCH', before_problem_page_index=1)
f = facts(pages=2); f['page_request_cursor_kinds'][1] = 'ABSENT'; f['page_request_cursor_values'][1] = ''
add('missing-request-cursor-known-rows', 'OBSERVED_DEFINITION_SAME', after=f, after_coverage='INCOMPLETE', definition='SAME_OBSERVED', after_coverage_reason='CURSOR_LINK_MISMATCH')
f = facts(pages=2); f['page_next_cursor_kinds'][0] = 'ABSENT'; f['page_next_cursor_values'][0] = ''
add('page-after-absent-terminal', 'OBSERVED_DEFINITION_SAME', before=f, before_coverage='INCOMPLETE', definition='SAME_OBSERVED', before_coverage_reason='PAGE_AFTER_TERMINAL', before_problem_page_index=1)
f = facts(pages=2); f['page_next_cursor_kinds'][0] = 'NULL'; f['page_next_cursor_values'][0] = ''
add('page-after-null-terminal', 'OBSERVED_DEFINITION_SAME', before=f, before_coverage='INCOMPLETE', definition='SAME_OBSERVED', before_coverage_reason='PAGE_AFTER_TERMINAL')
f = facts(); f['page_next_cursor_kinds'][0] = 'NULL'
add('null-next-is-terminal-not-absent-normalization', 'OBSERVED_DEFINITION_SAME', before=f, definition='SAME_OBSERVED', before_coverage_reason='OBSERVED_PAGE_CHAIN_COMPLETE')
f = facts(pages=2); f['page_next_cursor_values'][0] = ''; f['page_request_cursor_values'][1] = ''; f['row_page_indexes'][0] = 1
add('empty-string-cursor-links-next-page', 'OBSERVED_DEFINITION_SAME', before=f, definition='SAME_OBSERVED')
f = facts(); f['page_next_cursor_kinds'][0] = 'STRING'
add('empty-string-next-is-not-terminal', 'OBSERVED_DEFINITION_SAME', before=f, before_coverage='INCOMPLETE', definition='SAME_OBSERVED', before_coverage_reason='MISSING_TERMINAL_PAGE')
f = facts(pages=3); f['page_next_cursor_values'] = ['repeat', 'repeat', '']; f['page_request_cursor_values'] = ['', 'repeat', 'repeat']; f['row_page_indexes'][0] = 2
add('repeated-opaque-cursor-valid-links', 'OBSERVED_DEFINITION_SAME', before=f, definition='SAME_OBSERVED')
f = facts(pages=2); f['page_request_cursor_kinds'][0] = 'STRING'; f['page_request_cursor_values'][0] = 'not-first'; f['page_next_cursor_kinds'][1] = 'STRING'; f['page_next_cursor_values'][1] = 'missing'
add('first-coverage-problem-retained', 'OBSERVED_DEFINITION_SAME', before=f, before_coverage='INCOMPLETE', definition='SAME_OBSERVED', before_coverage_reason='MISSING_INITIAL_PAGE', before_problem_page_index=0)

f = facts(('lookup', 'lookup'))
add('duplicate-selected-name-before', 'AMBIGUOUS_NAME', before=f, before_problem_row_index=1, before_index=-1, after_index=-1, before_definition_id='', after_definition_id='')
add('duplicate-selected-name-after', 'AMBIGUOUS_NAME', after=f, after_problem_row_index=1, before_index=-1, after_index=-1)
f = facts(('lookup', 'other', 'other'))
add('unrelated-duplicate-still-ambiguous', 'AMBIGUOUS_NAME', before=f, before_problem_row_index=2, before_definition_id='', after_definition_id='')
f = facts(('a', 'b', 'b', 'a', 'lookup'))
add('earliest-second-duplicate-occurrence', 'AMBIGUOUS_NAME', after=f, after_problem_row_index=2)
f = facts(('lookup', 'lookup'), pages=2); f['row_page_indexes'] = [0, 1]
add('duplicate-across-pages', 'AMBIGUOUS_NAME', before=f, before_problem_row_index=1)
f = facts(('lookup', 'lookup')); f['page_next_cursor_kinds'][0] = 'STRING'; f['page_next_cursor_values'][0] = 'next'
add('ambiguity-precedes-partial-comparison', 'AMBIGUOUS_NAME', before=f, before_coverage='INCOMPLETE', before_problem_row_index=1, before_index=-1, after_index=-1)

for field in ('page_request_cursor_values', 'page_next_cursor_kinds', 'page_next_cursor_values'):
    f = facts(); f[field] = []
    add('page-column-' + field, 'INVALID_INPUT', before=f, before_coverage='INVALID', before_coverage_reason='PAGE_COLUMN_LENGTH')
for field in ('definition_ids', 'definition_profiles', 'annotation_ids', 'row_page_indexes'):
    f = facts(); f[field] = []
    add('row-column-' + field, 'INVALID_INPUT', before=f, before_coverage='INVALID', before_coverage_reason='ROW_COLUMN_LENGTH')
for field, value in (('page_request_cursor_kinds', 'NULL'), ('page_request_cursor_kinds', 'UNKNOWN'), ('page_next_cursor_kinds', 'UNKNOWN')):
    f = facts(); f[field][0] = value
    add('invalid-' + field + '-' + value.lower(), 'INVALID_INPUT', before=f, before_coverage='INVALID', before_coverage_reason='INVALID_CURSOR_TAG_OR_VALUE', before_problem_page_index=0)
for field, value in (('page_request_cursor_values', 'extra'), ('page_next_cursor_values', 'extra')):
    f = facts(); f[field][0] = value
    add('value-without-string-' + field, 'INVALID_INPUT', before=f, before_coverage='INVALID', before_coverage_reason='INVALID_CURSOR_TAG_OR_VALUE')
f = facts(); f['page_next_cursor_kinds'][0] = 'NULL'; f['page_next_cursor_values'][0] = 'extra'
add('null-cursor-with-value-invalid', 'INVALID_INPUT', before=f, before_coverage='INVALID', before_coverage_reason='INVALID_CURSOR_TAG_OR_VALUE')
for label, value in [('nul', 'bad\0cursor'), ('too-long', 'x' * 1025)]:
    f = facts(); f['page_next_cursor_kinds'][0] = 'STRING'; f['page_next_cursor_values'][0] = value
    add('cursor-value-' + label, 'INVALID_INPUT', before=f, before_coverage='INVALID', before_coverage_reason='INVALID_CURSOR_TAG_OR_VALUE')
for label, value in [('empty', ''), ('nul', 'look\0up'), ('too-long', 'x' * 1025)]:
    f = facts(); f['original_names'][0] = value
    add('name-' + label, 'INVALID_INPUT', before=f, before_coverage='INVALID', before_coverage_reason='INVALID_ROW_FACT', before_problem_row_index=0)
for field in ('definition_ids', 'annotation_ids'):
    for label, value in [('empty', ''), ('short', 'sha256:ab'), ('uppercase', 'sha256:' + 'A' * 64), ('nonhex', 'sha256:' + 'g' * 64)]:
        f = facts(); f[field][0] = value
        add(field + '-' + label, 'INVALID_INPUT', after=f, after_coverage='INVALID', after_coverage_reason='INVALID_ROW_FACT', after_problem_row_index=0)
for label, value in [('empty', ''), ('nul', 'profile\0x'), ('too-long', 'x' * 1025)]:
    f = facts(); f['definition_profiles'][0] = value
    add('profile-' + label, 'INVALID_INPUT', before=f, before_coverage='INVALID', before_coverage_reason='INVALID_ROW_FACT')
f = facts(); f['definition_profiles'][0] = 'different-profile/v1'
add('different-profile-not-comparable', 'UNASSESSED', before=f, profile_relation='DIFFERENT_OBSERVED', reason='DEFINITION_PROFILE_NOT_COMPARABLE', observed_presence_relation='OBSERVED_BOTH')
add('same-unsupported-profile-not-comparable', 'UNASSESSED', before=f, after=f, profile_relation='SAME_OBSERVED', reason='DEFINITION_PROFILE_NOT_COMPARABLE')
for label, page in [('negative', -1), ('out-of-range', 1)]:
    f = facts(); f['row_page_indexes'][0] = page
    add('row-page-' + label, 'INVALID_INPUT', before=f, before_coverage='INVALID', before_coverage_reason='INVALID_ROW_FACT', before_problem_row_index=0)
f = facts(('lookup', 'other'), pages=2); f['row_page_indexes'] = [1, 0]
add('row-page-order-invalid', 'INVALID_INPUT', before=f, before_coverage='INVALID', before_coverage_reason='INVALID_ROW_FACT', before_problem_row_index=1)
f = facts(pages=0)
add('rows-with-no-page-invalid', 'INVALID_INPUT', before=f, before_coverage='INVALID', before_coverage_reason='INVALID_ROW_FACT')
f = facts(pages=16); f['row_page_indexes'][0] = 15
add('sixteen-pages-boundary', 'OBSERVED_DEFINITION_SAME', before=f, definition='SAME_OBSERVED')
f = facts(pages=17)
add('seventeen-pages-rejected', 'INVALID_INPUT', before=f, before_coverage='INVALID', before_coverage_reason='SNAPSHOT_LIMIT')
f = facts(('lookup',) + tuple('tool-' + str(i) for i in range(1, 256)))
add('two-hundred-fifty-six-rows-boundary', 'OBSERVED_DEFINITION_SAME', before=f, definition='SAME_OBSERVED')
f = facts(('lookup',) + tuple('tool-' + str(i) for i in range(1, 257)))
add('two-hundred-fifty-seven-rows-rejected', 'INVALID_INPUT', after=f, after_coverage='INVALID', after_coverage_reason='SNAPSHOT_LIMIT')
f = facts(('x' * 1024,))
add('one-thousand-twenty-four-byte-name-boundary', 'OBSERVED_DEFINITION_SAME', before=f, after=f, selection={'context_id': 'synthetic:long-name', 'target_name': 'x' * 1024}, definition='SAME_OBSERVED')
f = facts(pages=2); f['page_next_cursor_values'][0] = 'x' * 1024; f['page_request_cursor_values'][1] = 'x' * 1024
add('one-thousand-twenty-four-byte-cursor-boundary', 'OBSERVED_DEFINITION_SAME', before=f, definition='SAME_OBSERVED')
for field in ('context_id', 'target_name'):
    for label, value in [('empty', ''), ('nul', 'bad\0value'), ('too-long', 'x' * 1025)]:
        selection = {'context_id': 'synthetic:selection', 'target_name': 'lookup'}; selection[field] = value
        add('selection-' + field + '-' + label, 'INVALID_INPUT', selection=selection, reason='INVALID_SELECTION_TEXT')
f = facts(); f['page_next_cursor_values'] = []; f['annotation_ids'] = []
add('page-column-priority-over-row-column', 'INVALID_INPUT', before=f, before_coverage='INVALID', before_coverage_reason='PAGE_COLUMN_LENGTH')
f = facts(pages=17); f['page_request_cursor_values'] = []
add('limit-priority-over-column-shape', 'INVALID_INPUT', before=f, before_coverage='INVALID', before_coverage_reason='SNAPSHOT_LIMIT')
f = facts(); f['page_request_cursor_kinds'][0] = 'NULL'; f['definition_ids'][0] = ''
add('cursor-priority-over-row-fact', 'INVALID_INPUT', before=f, before_coverage='INVALID', before_coverage_reason='INVALID_CURSOR_TAG_OR_VALUE')
f = facts(('lookup', 'lookup')); f['annotation_ids'][0] = ''
add('invalid-row-priority-over-duplicate', 'INVALID_INPUT', before=f, before_coverage='INVALID', before_coverage_reason='INVALID_ROW_FACT', before_problem_row_index=0)
f = facts(); f['definition_ids'][0] = ''
add('selection-priority-over-invalid-facts', 'INVALID_INPUT', before=f, before_coverage='INVALID', selection={'context_id': '', 'target_name': 'lookup'}, reason='INVALID_SELECTION_TEXT')


# Pinned compiler input admission is separate from the Gooo activity decision.
# These six original cases remain in reference-inputs/kernel.cases unchanged.
ENGINE_VALIDATION_SOURCES = [{'path': 'internal/bodyexecution/composition_cases.go',
  'sha256': 'sha256:aef6187afcd541db68891c52f0795c3cd8cde3e50e0a5887f9195c9486c4533a',
  'url': 'https://github.com/kimjooyoon/meta-ontology-go/blob/205896a413e5dea92366309c35ae85e542d055f6/internal/bodyexecution/composition_cases.go'},
 {'path': 'internal/bodyexecution/composition_records.go',
  'sha256': 'sha256:575768130c0ec3b103a8c861a6ade2f7bea8487d2a62de67a24463f20cda10e2',
  'url': 'https://github.com/kimjooyoon/meta-ontology-go/blob/205896a413e5dea92366309c35ae85e542d055f6/internal/bodyexecution/composition_records.go'},
 {'path': 'internal/bodyexecution/composition_collections.go',
  'sha256': 'sha256:90d72a6592e420de286d10100edf4002cefdd4da96b62d15af7b3d0fb63e05f1',
  'url': 'https://github.com/kimjooyoon/meta-ontology-go/blob/205896a413e5dea92366309c35ae85e542d055f6/internal/bodyexecution/composition_collections.go'},
 {'path': 'internal/bodyexecution/composition_capacity.go',
  'sha256': 'sha256:8146e93f42d733a888f6c12a0bf4ac7b902cb25563815ad791b0a5d4d86b0b46',
  'url': 'https://github.com/kimjooyoon/meta-ontology-go/blob/205896a413e5dea92366309c35ae85e542d055f6/internal/bodyexecution/composition_capacity.go'},
 {'path': 'internal/bodyexecution/text_json.go',
  'sha256': 'sha256:ec2ae0f7d97fc402e8fd646fb4d526bd81f415b3f95e79646ba2d8721d39749b',
  'url': 'https://github.com/kimjooyoon/meta-ontology-go/blob/205896a413e5dea92366309c35ae85e542d055f6/internal/bodyexecution/text_json.go'},
 {'path': 'internal/valuejson/text.go',
  'sha256': 'sha256:2b366a79dc6266f50de6a49b4de552cae52729bff7e4a1dc8e09868cc0535b74',
  'url': 'https://github.com/kimjooyoon/meta-ontology-go/blob/205896a413e5dea92366309c35ae85e542d055f6/internal/valuejson/text.go'},
 {'path': 'internal/syntax/entity_fields_support.go',
  'sha256': 'sha256:6e1e90d20587d0ccd884670704c6ad8ce9a5198af3bf1200925accd1f52512fe',
  'url': 'https://github.com/kimjooyoon/meta-ontology-go/blob/205896a413e5dea92366309c35ae85e542d055f6/internal/syntax/entity_fields_support.go'}]

INPUT_REJECTIONS = [
    (51, 'cursor-value-too-long', 'input0', 'SnapshotFacts', 'page_next_cursor_values', 0, 'text_utf8_bytes', 1025, 1024),
    (54, 'name-too-long', 'input0', 'SnapshotFacts', 'original_names', 0, 'text_utf8_bytes', 1025, 1024),
    (65, 'profile-too-long', 'input0', 'SnapshotFacts', 'definition_profiles', 0, 'text_utf8_bytes', 1025, 1024),
    (75, 'two-hundred-fifty-seven-rows-rejected', 'input1', 'SnapshotFacts', 'original_names', None, 'collection_elements', 257, 256),
    (80, 'selection-context_id-too-long', 'input2', 'Selection', 'context_id', None, 'text_utf8_bytes', 1025, 1024),
    (83, 'selection-target_name-too-long', 'input2', 'Selection', 'target_name', None, 'text_utf8_bytes', 1025, 1024),
]


def input_boundaries():
    rows=[]
    for index,label,port,record,field,element,kind,actual,limit in INPUT_REJECTIONS:
        assert CASES[index]['label']==label
        rows.append({'reference_case_index':index,'label':label,'input_port':port,'record':record,'field':field,'element_index':element,'boundary':kind,'observed_size':actual,'limit':limit,'stage':'INPUT_VALIDATION','code':'INVALID_INPUT_VALUE'})
    return {'schema':'gooo/definition-snapshot-input-boundaries/v1','compiler_revision':'205896a413e5dea92366309c35ae85e542d055f6','pinned_validation_sources':ENGINE_VALIDATION_SOURCES,'reference_cases':len(CASES),'native_cases':len(CASES)-len(rows),'rejections':rows,'scope':'Exact typed-input admission probes. These cases have independent domain-reference expectations but do not reach the Gooo body through package execute. Direct generated-function API tests are separate.'}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--check', action='store_true')
    args = parser.parse_args()
    assert 35 <= len(CASES) <= 128
    assert len({c['label'] for c in CASES}) == len(CASES)
    documents = {
        'input-boundaries.json': input_boundaries(),
        'reference-inputs.json': {'schema': 'gooo/definition-snapshot-reference-inputs/v1', 'cases': CASES},
        'case-labels.json': {'origin': 'SYNTHETIC_FIXTURE', 'cases': [c['label'] for c in CASES], 'scope': 'Finite authored fact-column and observation coverage controls; no server/model calls or external adoption.'},
    }
    for name, value in documents.items():
        raw = (json.dumps(value, ensure_ascii=False, indent=2) + '\n').encode()
        assert len(raw) <= 4 << 20
        if args.check:
            assert (HERE / name).read_bytes() == raw, name
        else:
            (HERE / name).write_bytes(raw)
    print(json.dumps({'cases': len(CASES), 'literal_anchor_fixture_files': sorted(documents), 'domain_execution': False, 'check': args.check}))


if __name__ == '__main__':
    main()
