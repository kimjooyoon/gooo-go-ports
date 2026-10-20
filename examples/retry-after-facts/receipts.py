"""Pure saved-receipt checks for the reviewed Retry-After facts profile.

No project module is imported, no process is launched and no file is opened on
import. ``prepare`` reads only fixed source/corpus paths via its caller's bounded,
frozen ``read(Path) -> bytes`` function. ``batches`` emits original input values
only; the independent oracle is never supplied to the compiler.

This module binds saved observations. It does not authenticate producers, execute
native code, grant retry authority, or establish current request/time facts.
"""
from pathlib import Path
import copy
import hashlib
import json
import re

__all__ = ['ReceiptError', 'prepare', 'batches', 'validate', 'negative_controls']
REV = '205896a413e5dea92366309c35ae85e542d055f6'
PACKAGE = 'app/retry-after-facts'
ENTRY = 'gooo://retry-after-facts/assess/v1'
RESULT = 'gooo://retry-after-facts/report/v1'
CALLEE = 'gooo://go/strconv/parse-int-decimal64/v1'
SOURCE_PATHS = ('ports/strconv/decimal.gooo', 'examples/retry-after-facts/main.gooo')
GO_TYPES = {'string': 'string', 'integer': 'int64', 'boolean': 'bool'}
INPUT_NAMES = ['header', 'header_present', 'elapsed_seconds', 'max_wait_seconds', 'request_binding_current', 'guidance_applicable']
INPUT_TYPES = ['Text', 'Boolean', 'Integer', 'Integer', 'Boolean', 'Boolean']
INPUT_IDS = ['gooo://std/' + t for t in ['text', 'boolean', 'integer', 'integer', 'boolean', 'boolean']]
PARAMETERS = [{'name': 'input' + str(i), 'type': t} for i, t in enumerate(['string', 'bool', 'int64', 'int64', 'bool', 'bool'])]
TOKEN = re.compile(r'//[^\n]*|/\*.*?\*/|"(?:\\.|[^"\\])*"|[A-Za-z_]\w*|\d+|&&|\|\||==|!=|<=|>=|[^\s]', re.S)
NOT_CHECKED = {
    'inner_canonical_prior_result_sha256': 'NOT_CHECKED: no independent Go canonical prior-result serializer',
    'typed_plan_sha256': 'NOT_CHECKED: cross-layer binding/shape only; canonical typed plan digest is not recomputed',
    'inner_program_semantic_and_route_equivalence_digests': 'NOT_CHECKED: no independent typed compiler or control-flow digest implementation',
    'instrumented_observed_projection_and_driver_sha256': 'NOT_CHECKED: exact GoAST/format instrumentation not regenerated',
    'runtime_composition_suite_executable_hashes': 'NOT_CHECKED: no reconstructed native build or retained executable read',
    'fresh_execution_or_authenticated_producer': 'NOT_CHECKED: this module checks supplied evidence only',
}
# Literal reviewed semantic profile, rather than a hash of a local artifact.
EXPECTED_CONTRACT = {'schema': 'gooo/retry-after-facts-contract/v2',
 'original_policy': True,
 'package_path': 'app/retry-after-facts',
 'activity': 'AssessDelay',
 'activity_id': 'gooo://retry-after-facts/assess/v1',
 'result_id': 'gooo://retry-after-facts/report/v1',
 'inputs': ['header:Text',
            'header_present:Boolean',
            'elapsed_seconds:Integer',
            'max_wait_seconds:Integer',
            'request_binding_current:Boolean',
            'guidance_applicable:Boolean'],
 'transport': {'header': 'Valid Unicode-scalar UTF8 Text0..64 bytes; exact echo; '
                         'missing/wrong-type/duplicates/unknown keys/non-scalar Unicode/over64B '
                         'refuse BEFORE compiler invocation. Direct raw native outside declared '
                         'domain unsupported.',
               'integers': 'Exact JSON signed-int64; Boolean/floating-point not integers.',
               'booleans': 'Exact JSON Boolean.',
               'positional_keys': ['app/retry-after-facts:AssessDelay.input0',
                                   'app/retry-after-facts:AssessDelay.input1',
                                   'app/retry-after-facts:AssessDelay.input2',
                                   'app/retry-after-facts:AssessDelay.input3',
                                   'app/retry-after-facts:AssessDelay.input4',
                                   'app/retry-after-facts:AssessDelay.input5']},
 'parser': 'Always call existing go/strconv:ParseIntDecimal64 on original header, including '
           'invalid metadata. Preserve parsed_value and error_kind exactly as its pinned port (0 '
           'success,1 syntax,2 range with signed-int64 saturation); this is diagnostics, not '
           'admission.',
 'output_fields': [{'name': 'header',
                    'type': 'string',
                    'id': 'gooo://retry-after-facts/report/v1/header'},
                   {'name': 'header_present',
                    'type': 'boolean',
                    'id': 'gooo://retry-after-facts/report/v1/header_present'},
                   {'name': 'parsed_value',
                    'type': 'integer',
                    'id': 'gooo://retry-after-facts/report/v1/parsed_value'},
                   {'name': 'parse_error_kind',
                    'type': 'integer',
                    'id': 'gooo://retry-after-facts/report/v1/parse_error_kind'},
                   {'name': 'delta_seconds_grammar',
                    'type': 'boolean',
                    'id': 'gooo://retry-after-facts/report/v1/delta_seconds_grammar'},
                   {'name': 'elapsed_seconds',
                    'type': 'integer',
                    'id': 'gooo://retry-after-facts/report/v1/elapsed_seconds'},
                   {'name': 'max_wait_seconds',
                    'type': 'integer',
                    'id': 'gooo://retry-after-facts/report/v1/max_wait_seconds'},
                   {'name': 'request_binding_current',
                    'type': 'boolean',
                    'id': 'gooo://retry-after-facts/report/v1/request_binding_current'},
                   {'name': 'guidance_applicable',
                    'type': 'boolean',
                    'id': 'gooo://retry-after-facts/report/v1/guidance_applicable'},
                   {'name': 'delay_in_policy',
                    'type': 'boolean',
                    'id': 'gooo://retry-after-facts/report/v1/delay_in_policy'},
                   {'name': 'cooldown_active',
                    'type': 'boolean',
                    'id': 'gooo://retry-after-facts/report/v1/cooldown_active'},
                   {'name': 'remaining_seconds',
                    'type': 'integer',
                    'id': 'gooo://retry-after-facts/report/v1/remaining_seconds'},
                   {'name': 'state',
                    'type': 'string',
                    'id': 'gooo://retry-after-facts/report/v1/state'},
                   {'name': 'reason',
                    'type': 'string',
                    'id': 'gooo://retry-after-facts/report/v1/reason'}],
 'derived_fields': {'delta_seconds_grammar': 'Nonempty ASCII digits0..9 only; no trim, sign, '
                                             'separator, HTTP-date or Unicode digits; leading '
                                             'zeros allowed.',
                    'delay_in_policy': 'Valid numeric metadata AND parser success/error0 AND delta '
                                       'grammar AND0<=parsed_value<=max_wait_seconds. Independent '
                                       'of the three confirmation booleans; membership alone is '
                                       'not trusted guidance.',
                    'cooldown_active': 'True only when selected state is WAIT.',
                    'remaining_seconds': '-1 unless admitted WAIT or READY_TO_REASSESS; WAIT uses '
                                         'delay-elapsed, READY uses0.',
                    'echo': 'header,header_present,elapsed_seconds,max_wait_seconds,request_binding_current,guidance_applicable '
                            'preserved exactly.'},
 'initial': {'state': 'UNKNOWN',
             'reason': 'GUIDANCE_UNCONFIRMED',
             'cooldown_active': False,
             'remaining_seconds': -1},
 'ordered_terminal_rules': [{'when': 'elapsed_seconds<0 OR max_wait_seconds<0 OR '
                                     'max_wait_seconds>86400',
                             'state': 'DOMAIN_REFUSED',
                             'reason': 'INPUT_DOMAIN_INVALID'},
                            {'when': 'request_binding_current is false',
                             'state': 'UNKNOWN',
                             'reason': 'REQUEST_BINDING_NOT_CURRENT'},
                            {'when': 'guidance_applicable is false',
                             'state': 'UNKNOWN',
                             'reason': 'GUIDANCE_NOT_APPLICABLE'},
                            {'when': 'header_present is false',
                             'state': 'UNKNOWN',
                             'reason': 'HEADER_NOT_CONFIRMED'},
                            {'when': 'parser.error_kind==1',
                             'state': 'UNKNOWN',
                             'reason': 'DELAY_SYNTAX_INVALID'},
                            {'when': 'parser.error_kind==2',
                             'state': 'UNKNOWN',
                             'reason': 'DELAY_RANGE_ERROR'},
                            {'when': 'parser.parsed is false OR parser.error_kind!=0',
                             'state': 'UNKNOWN',
                             'reason': 'PARSER_CONTRACT_UNSUPPORTED'},
                            {'when': 'delta_seconds_grammar is false',
                             'state': 'UNKNOWN',
                             'reason': 'DELTA_SECONDS_GRAMMAR'},
                            {'when': 'parsed_value<0 OR parsed_value>max_wait_seconds',
                             'state': 'UNKNOWN',
                             'reason': 'WAIT_POLICY_EXCEEDED'},
                            {'when': 'elapsed_seconds<parsed_value',
                             'state': 'WAIT',
                             'reason': 'COOLDOWN_ACTIVE',
                             'cooldown_active': True,
                             'remaining_seconds': 'parsed_value-elapsed_seconds (both bounded '
                                                  'nonnegative and delay<=86400)'},
                            {'when': 'otherwise',
                             'state': 'READY_TO_REASSESS',
                             'reason': 'COOLDOWN_ELAPSED',
                             'remaining_seconds': 0}],
 'arithmetic': 'No wall clock, absolute timestamp, duration scaling or timestamp arithmetic. Only '
               'subtract when0<=elapsed<delay<=max_wait<=86400; elapsed may otherwise span all '
               'nonnegative int64 without subtraction.',
 'host_contract': ['Host establishes selected response/request/whole-operation binding, confirmed '
                   'header presence, applicability and elapsed observation. A supplied true is not '
                   'authentication; absence of confirmation is UNKNOWN.',
                   'READY_TO_REASSESS schedules a fresh policy assessment. It never manufactures '
                   'provider ALLOW or ABSENT, dispatch eligibility, authority, a durable claim or '
                   'a retry-loop effect.',
                   'WAIT is a finite time fact, not a server repeat contract. Existing access, '
                   'whole-operation history, request/key binding, budget and atomic claim '
                   'obligations remain independent.',
                   'Saved replay reconstructs prior facts and cannot establish current guidance or '
                   'elapsed time.'],
 'compatibility': 'Original bounded composition over an existing Gooo port, not full Go '
                  'SDK/rate-limiter/HTTP-date/RFC compatibility. The86400 policy maximum and64B '
                  'header cap are explicit local profile choices; x-ratelimit-reset, '
                  'multiple-header selection and network effects are absent.'}

class ReceiptError(RuntimeError):
    def __init__(self, boundary, detail):
        self.boundary = boundary
        super().__init__(boundary + ': ' + detail)

def need(value, boundary, detail):
    if not value:
        raise ReceiptError(boundary, detail)

def sha(raw):
    return hashlib.sha256(raw).hexdigest()

def digest(raw):
    return 'sha256:' + sha(raw)

def reference(raw):
    return {'bytes': len(raw), 'sha256': sha(raw)}

def equal(a, b):
    if type(a) is not type(b):
        return False
    if type(a) is dict:
        return set(a) == set(b) and all(equal(a[k], b[k]) for k in a)
    if type(a) is list:
        return len(a) == len(b) and all(equal(x, y) for x, y in zip(a, b))
    return a == b

def go_json(value, html=True):
    text = json.dumps(value, ensure_ascii=False, separators=(',', ':'), allow_nan=False)
    text = text.replace('\u2028', '\\u2028').replace('\u2029', '\\u2029')
    if html:
        text = text.replace('<', '\\u003c').replace('>', '\\u003e').replace('&', '\\u0026')
    return text.encode('utf-8')

def lexical_mask(source):
    chars = list(source)
    i = 0
    while i < len(source):
        start = i
        if source.startswith('//', i):
            end = source.find('\n', i)
            i = len(source) if end < 0 else end
        elif source.startswith('/*', i):
            end = source.find('*/', i + 2)
            need(end >= 0, 'body', 'closed comment')
            i = end + 2
        elif source[i] in ('"', "'", '`'):
            quote = source[i]
            i += 1
            while i < len(source):
                if source[i] == quote:
                    i += 1
                    break
                i += 2 if quote != '`' and source[i] == '\\' else 1
        else:
            i += 1
            continue
        for j in range(start, min(i, len(source))):
            if source[j] != '\n':
                chars[j] = ' '
    return ''.join(chars)

def tokens(body, rename):
    result = []
    for match in TOKEN.finditer(body):
        token = match[0]
        if token.startswith(('//', '/*')):
            continue
        result.append(rename.get(token, 'var' if token == 'let' else token))
    # The compiler makes mutable integer literals explicitly int64. No other
    # token, branch, operator, call, string value or field access is discarded.
    result = [v for i, v in enumerate(result)
              if not (v == 'int64' and i >= 2 and i + 1 < len(result)
                      and result[i - 2] == 'var' and result[i + 1] == '=')]
    return [v for i, v in enumerate(result)
            if not (v == ',' and i + 1 < len(result) and result[i + 1] == '}')]

def go_parts(source, names, records):
    mask = lexical_mask(source)
    all_functions = re.findall(r'^func (\w+)\(', mask, re.M)
    need(set(all_functions) == set(names) and len(all_functions) == len(names), 'body', 'exact two generated pure functions')
    functions = {}
    consumed = list(mask)
    for m in re.finditer(r'^func (\w+)\(([^)]*)\) (\w+) \{', mask, re.M):
        depth, end = 1, m.end()
        while depth:
            need(end < len(mask), 'body', 'balanced generated function')
            depth += (mask[end] == '{') - (mask[end] == '}')
            end += 1
        functions[m[1]] = (m[2], m[3], source[m.end():end - 1])
        consumed[m.start():end] = ' ' * (end - m.start())
    need(set(functions) == set(names), 'body', 'typed generated function declarations')
    observed_records = []
    for m in re.finditer(r'^type (\w+) struct \{\n([\s\S]*?)^\}', source, re.M):
        fields = []
        for line in m[2].splitlines():
            if not line.strip():
                continue
            field = re.fullmatch(r'\s+(\w+)\s+(string|int64|bool)\s+`json:"([^"]+)"`\s*', line)
            need(field is not None, 'record_abi', 'every nonempty generated struct line must be one reviewed scalar field')
            fields.append(field.groups())
        observed_records.append((m[1], fields))
        consumed[m.start():m.end()] = ' ' * (m.end() - m.start())
    expected = [(r['go_name'], [(f['go_name'], GO_TYPES[f['type_id'].split(':')[-1]], f['name']) for f in r['fields']]) for r in records]
    need(equal(observed_records, expected), 'record_abi', 'generated Go structs, field names/types/order/JSON tags')
    rest = ''.join(consumed)
    rest = re.sub(r'^package (?:gooo_workspace|main)\s*', '', rest, count=1)
    need(not rest.strip(), 'body', 'no imports, globals, init or additional generated code')
    return functions


# Current57-case receipt includes typed local/field provenance. Its retained
# original is7,565,743bytes; keep this finite parser within the existing8MiB
# owned-command stdout cap rather than removing bounds or reducing the corpus.
RECEIPT_CAP = 8 << 20
PRODUCER_PATH = 'github.com/kimjooyoon/meta-ontology-go'
PRODUCER_VERSIONS = ('v0.0.0-20261020180505-205896a413e5',
                     'v0.6.37-dev.0.20261020180505-205896a413e5')


def producer_module(identity):
    """Bind independently queried clean compiler identity, never receipt input."""
    need(type(identity) is dict and identity.get('schema') == 'gooo/build-identity/v1' and
         identity.get('compiler_source_sha') == identity.get('vcs_revision') == REV and
         identity.get('source_status') == 'CLEAN_VCS' and identity.get('vcs_modified') == 'false' and
         identity.get('go_version') == identity.get('native_go_required') == 'go1.27.3' and
         identity.get('build_info_observed') is True and identity.get('version') == '0.6.37-dev' and
         equal(identity.get('decision_runtime'), {'module_path': 'github.com/kimjooyoon/gooo-decision-runtime',
               'version': 'v0.2.51-experimental', 'replaced': False}),
         'go_identity', 'clean pinned compiler and unreplaced runtime build identity')
    module = identity.get('module')
    need(type(module) is dict and set(module) == {'path', 'version'} and
         module['path'] == PRODUCER_PATH and module['version'] in PRODUCER_VERSIONS and
         identity.get('module_version') == module['version'],
         'go_identity', 'actual clean-bootstrap or installed-source module identity')
    return copy.deepcopy(module)


def load(raw):
    need(type(raw) is bytes, 'json', 'raw JSON must be bytes')
    need(0 < len(raw) <= RECEIPT_CAP, 'json', 'bounded nonempty JSON evidence')
    def pairs(rows):
        out = {}
        for key, value in rows:
            need(key not in out, 'json', 'duplicate decoded key ' + key)
            out[key] = value
        return out
    try:
        return json.loads(raw.decode('utf-8', errors='strict'), object_pairs_hook=pairs,
                          parse_constant=lambda value: (_ for _ in ()).throw(ReceiptError('json', value)))
    except ReceiptError:
        raise
    except (UnicodeError, ValueError, TypeError) as error:
        raise ReceiptError('json', 'invalid UTF8/JSON: ' + str(error)) from error


def _source(path, raw):
    source = raw.decode('utf-8', errors='strict')
    activity = re.search(r'^activity (\w+)\(([^)]*)\) -> (\w+) id "([^"]+)" computes `([\s\S]*)`\s*\Z', source, re.M)
    need(activity is not None and len(re.findall(r'^activity ', source, re.M)) == 1,
         'source', 'one fixed source activity: ' + path)
    entities = []
    for match in re.finditer(r'^entity (\w+) id "([^"]+)"(?: fields \{\n([\s\S]*?)^\})?\n', source, re.M):
        fields = []
        if match[3] is not None:
            fields = re.findall(r'^    field (\w+) id "([^"]+)" type (string|integer|boolean) required one$', match[3], re.M)
        entities.append({'name': match[1], 'id': match[2], 'text': match[0].rstrip('\n'), 'fields': fields})
    need(len(entities) == 4, 'record_abi', 'three scalars and one record per current source')
    return source, activity, entities


def _input(value):
    need(type(value) is dict and set(value) == set(INPUT_NAMES), 'typedfield', 'exact six witness input names')
    need(type(value['header']) is str, 'typedfield', 'header Text')
    try:
        header = value['header'].encode('utf-8', errors='strict')
    except UnicodeError as error:
        raise ReceiptError('typedfield', 'header must contain Unicode scalars') from error
    need(len(header) <= 64, 'typedfield', 'header profile0..64 UTF8 bytes')
    for name in ['elapsed_seconds', 'max_wait_seconds']:
        need(type(value[name]) is int and -(1 << 63) <= value[name] < 1 << 63,
             'typedfield', 'exact signed-int64 input: ' + name)
    for name in ['header_present', 'request_binding_current', 'guidance_applicable']:
        need(type(value[name]) is bool, 'typedfield', 'exact Boolean input: ' + name)


def _prepare(root, oracle_raw, go_version, go_sha, read, producer):
    root = Path(root)
    need(root.is_absolute() and callable(read), 'source', 'caller selects one absolute root and bounded frozen read')
    need(type(go_version) is str and re.fullmatch(r'go version go1\.27\.3 (?:linux/amd64|darwin/arm64)', go_version) is not None,
         'go_identity', 'pinned Go1.27.3 on Linux/amd64 or Darwin/arm64')
    need(type(go_sha) is str and re.fullmatch(r'sha256:[0-9a-f]{64}', go_sha) is not None,
         'go_identity', 'caller-provided actual Go binary digest')
    need(type(producer) is dict and set(producer) == {'path', 'version'} and
         producer['path'] == PRODUCER_PATH and producer['version'] in PRODUCER_VERSIONS,
         'go_identity', 'caller-supplied independently observed compiler module')
    refs = {}
    def current(name):
        # Names originate in this module, never a receipt, source import, oracle,
        # fixture row, manifest or external output-selected path.
        value = read(root / name)
        need(type(value) is bytes and len(value) <= 1 << 20, 'source', 'bounded exact current source bytes')
        observed = reference(value)
        need(name not in refs or equal(refs[name], observed), 'source', 'source changed during preparation')
        refs[name] = observed
        return value
    contract = load(current('examples/retry-after-facts/contract.json'))
    need(type(contract) is dict and set(contract) <= set(EXPECTED_CONTRACT) | {'status', 'supersedes_unimplemented_proposal', 'validation', 'created_at_utc'},
         'source', 'reviewed v2 contract keys')
    need(all(key in contract and equal(contract[key], value) for key, value in EXPECTED_CONTRACT.items()),
         'source', 'literal113-case/14-field/six-input semantic profile and ordered rules')
    witnesses = load(current('examples/retry-after-facts/witnesses.json'))
    input_document = load(current('examples/retry-after-facts/inputs.json'))
    need(type(input_document) is dict and set(input_document) == {'schema', 'inputs'} and input_document['schema'] == 'gooo/body-composition-inputs/v1',
         'typedfield', 'original input-only corpus document')
    inputs = input_document['inputs']
    oracle = load(oracle_raw)
    need(type(oracle) is dict and set(oracle) == {'schema', 'rows'} and oracle['schema'] == 'gooo/retry-after-facts-independent-reference/v1',
         'typedfield', 'fresh independent Go reference output')
    need(type(witnesses) is list and type(inputs) is list and type(oracle['rows']) is list and
         len(witnesses) == len(inputs) == len(oracle['rows']) == 113, 'typedfield', 'all original113 rows')
    fields = copy.deepcopy(EXPECTED_CONTRACT['output_fields'])
    witness_names = set()
    for index, (witness, positional, observed) in enumerate(zip(witnesses, inputs, oracle['rows'])):
        need(type(witness) is dict and set(witness) == {'name', 'input'} and type(witness['name']) is str and witness['name'] and witness['name'] not in witness_names,
             'typedfield', 'unique source-authored witness name')
        witness_names.add(witness['name']); _input(witness['input'])
        need(type(observed) is dict and set(observed) == {'index', 'name', 'input', 'result'} and
             equal([observed['index'], observed['name'], observed['input']], [index, witness['name'], witness['input']]),
             'typedfield', 'fresh Go result for the same original global index/name/input')
        need(equal(positional, {PACKAGE + ':AssessDelay.input' + str(i): witness['input'][name] for i, name in enumerate(INPUT_NAMES)}),
             'typedfield', 'original exact six positional values')
        need(type(observed['result']) is dict and set(observed['result']) == {field['name'] for field in fields}, 'typedfield', 'exact14 oracle fields')
        for field in fields:
            value = observed['result'][field['name']]
            need(type(value) is {'string': str, 'integer': int, 'boolean': bool}[field['type']], 'typedfield', 'independent scalar kind: ' + field['name'])
            if field['type'] == 'integer':
                need(-(1 << 63) <= value < 1 << 63, 'typedfield', 'independent int64 result')
        need(all(equal(observed['result'][name], witness['input'][name]) for name in INPUT_NAMES), 'typedfield', 'all six original observation echoes')
    # Freeze the current independent reference through the same caller-owned read.
    # Its authorship chronology and independence remain the caller's provenance.
    current('examples/retry-after-facts/reference/main.go')
    sources = [_source(name, current(name)) for name in SOURCE_PATHS]
    for index, (text, activity, entities) in enumerate(sources):
        need(re.findall(r'^package (\w+)$', text, re.M) == [('strconv' if index == 0 else 'retry_after_facts')], 'source', 'current source package')
        need(re.findall(r'^namespace (\w+)$', text, re.M) == [('gooo_ports_strconv' if index == 0 else 'gooo_ports_examples_retry_after_facts')], 'source', 'current source namespace')
        need(re.findall(r'^import (\w+) "([^"]+)"$', text, re.M) == ([] if index == 0 else [('decimal', 'go/strconv')]), 'callee', 'only fixed direct decimal import')
        expected_activity = ('ParseIntDecimal64', 'Text', 'DecimalInt64Result', CALLEE) if index == 0 else ('AssessDelay', ', '.join(INPUT_TYPES), 'RetryAfterFacts', ENTRY)
        need(tuple(activity.groups()[:4]) == expected_activity, 'six_abi', 'source stable activity identity and ordered ABI')
        scalars = {entity['name']: entity['id'] for entity in entities if not entity['fields']}
        need(scalars == {'Text': 'gooo://std/text', 'Integer': 'gooo://std/integer', 'Boolean': 'gooo://std/boolean'}, 'record_abi', 'three current source scalar identities')
    entities, entity_map = [], {}
    for _, _, declarations in sources:
        for entity in declarations:
            if entity['name'] in entity_map:
                need(equal(entity_map[entity['name']], entity), 'source', 'identical repeated scalar declaration')
            else:
                entity_map[entity['name']] = entity; entities.append(entity)
    records = []
    for entity in entities:
        if entity['fields']:
            records.append({'name': entity['name'], 'id': entity['id'], 'go_name': 'GoooRecord' + sha(entity['id'].encode()),
                            'fields': [{'name': name, 'id': ident, 'type_id': 'urn:gooo:type:' + typ, 'go_name': 'GoooField' + sha(ident.encode())} for name, ident, typ in entity['fields']]})
    need([(record['name'], record['id'], len(record['fields'])) for record in records] ==
         [('DecimalInt64Result', 'gooo://go-ports/strconv/decimal-int64-result/v1', 4), ('RetryAfterFacts', RESULT, 14)], 'record_abi', 'exact two current record identities')
    need([(field['name'], field['id'], field['type_id'].split(':')[-1]) for field in records[1]['fields']] ==
         [(field['name'], field['id'], field['type']) for field in fields], 'record_abi', 'source result record matches literal14-field contract')
    need([(field['name'], field['id'], field['type_id']) for field in records[0]['fields']] ==
         [(name, 'gooo://go-ports/strconv/decimal-int64-result/v1/' + name, 'urn:gooo:type:' + typ)
          for name, typ in [('input', 'string'), ('value', 'integer'), ('parsed', 'boolean'), ('error_kind', 'integer')]], 'record_abi', 'exact four-field parser result ABI')
    lowered_names = ['GoooPackage0ActivityParseIntDecimal64', 'GoooPackage1ActivityAssessDelay']
    lines = ['package gooo_workspace\nnamespace gooo_workspace\n', *[entity['text'] for entity in entities]]
    bodies = []
    for index, (_, activity, _) in enumerate(sources):
        body = activity[5]
        if index == 1:
            need(body.count('decimal.ParseIntDecimal64') == 1, 'callee', 'one explicit original parser call')
            body = body.replace('decimal.ParseIntDecimal64', lowered_names[0])
        bodies.append(body)
        lines.append('activity ' + lowered_names[index] + '(' + activity[2] + ') -> ' + activity[3] + ' id "' + activity[4] + '" computes ' + go_json(body, html=False).decode())
    lowered = '\n'.join(lines) + '\n'
    decimal_body, root_body = sources[0][1][5], sources[1][1][5]
    closure = {'schema': 'gooo/pure-activity-call-closure/v1', 'activities': [{'name': lowered_names[0], 'activity_id': CALLEE, 'program_sha256': digest(decimal_body.encode())}], 'edges': [{'caller_id': ENTRY, 'callee_id': CALLEE, 'ordinal': 0}], 'max_calls_per_invocation': 1}
    call_start = root_body.encode().index(b'decimal.ParseIntDecimal64')
    pure = {'schema': 'gooo/workspace-pure-calls/v1', 'activities': [
        {'activity': {'package_path': PACKAGE, 'activity': 'AssessDelay', 'lowered_name': lowered_names[1]}, 'activity_id': ENTRY, 'lowered_activity_id': ENTRY, 'source': SOURCE_PATHS[1], 'source_body_sha256': digest(root_body.encode())},
        {'activity': {'package_path': 'go/strconv', 'activity': 'ParseIntDecimal64', 'lowered_name': lowered_names[0]}, 'activity_id': CALLEE, 'lowered_activity_id': CALLEE, 'source': SOURCE_PATHS[0], 'source_body_sha256': digest(decimal_body.encode())}],
        'sites': [{'caller': PACKAGE + ':AssessDelay', 'callee': 'go/strconv:ParseIntDecimal64', 'surface': 'computes', 'start_byte': call_start, 'end_byte': call_start + len(b'decimal.ParseIntDecimal64')}]}
    manifest_raw = current('retry-after-facts.workspace.json')
    need(equal(load(manifest_raw), {'schema': 'gooo/package-workspace-manifest/v1', 'entry': {'package_path': PACKAGE, 'activity': 'AssessDelay'}, 'packages': [{'path': 'go/strconv', 'sources': [SOURCE_PATHS[0]]}, {'path': PACKAGE, 'sources': [SOURCE_PATHS[1]]}]}), 'source', 'exact fixed two-package manifest closure')
    return {'root': root, 'contract': contract, 'fields': fields, 'inputs': inputs, 'witnesses': witnesses,
            'oracle': oracle['rows'], 'oracle_ref': reference(oracle_raw), 'sources': sources, 'records': records,
            'lowered': lowered, 'bodies': bodies, 'names': lowered_names, 'pure': pure, 'closure': closure,
            'go_sha': go_sha, 'go_version': go_version, 'producer_module': copy.deepcopy(producer),
            'version_raw': (go_version + '\n').encode(),
            'manifest': manifest_raw, 'source_refs': refs, 'not_checked': copy.deepcopy(NOT_CHECKED)}


def prepare(root, oracle_raw, go_version, go_sha, read, producer):
    """Bind fresh independent Go output to current fixed source/corpus inputs."""
    try:
        return _prepare(root, oracle_raw, go_version, go_sha, read, producer)
    except ReceiptError:
        raise
    except (KeyError, TypeError, ValueError, IndexError, AttributeError, UnicodeError) as error:
        raise ReceiptError('source', 'malformed current profile: ' + str(error)) from error


def batches(ctx):
    """Return all113 original inputs as57+56; never include oracle expectations."""
    result = []
    for index, start, end in [(0, 0, 57), (1, 57, 113)]:
        rows = copy.deepcopy(ctx['inputs'][start:end])
        raw = go_json({'schema': 'gooo/body-composition-inputs/v1', 'inputs': rows}, html=False) + b'\n'
        need(len(rows) == end - start and len(rows) <= 128 and 0 < len(raw) <= 32768,
             'batch', 'original bounded57+56 documents')
        values = [{field['name']: row['result'][field['name']] for field in ctx['fields']}
                  for row in ctx['oracle'][start:end]]
        wire = {'schema': 'gooo/arithmetic-outcome-observation/v1', 'outputs': [[value] for value in values],
                'faults': [[None] for _ in values], 'blocked': [[None] for _ in values], 'calls': []}
        wire_raw = go_json(wire) + b'\n'
        result.append({'index': index, 'start': start, 'end': end, 'raw': raw, 'rows': rows,
                       'wire_digest': digest(wire_raw), 'wire_ref': reference(wire_raw)})
    need(equal([row for part in result for row in part['rows']], ctx['inputs']), 'batch', 'all113 original input rows exactly once in order')
    return result


def _batch(ctx, batch):
    need(type(batch) is dict and type(batch.get('index')) is int and 0 <= batch['index'] < 2,
         'batch', 'one of the two fixed original batches')
    expected = batches(ctx)[batch['index']]
    need(equal(batch, expected), 'batch', 'batch metadata/raw/rows/wire must be independently reconstructed from current ctx')
    return expected


def _verify(r, ctx, batch, parent_raw=None, parent_program=None):
    need(r['schema'] == 'gooo/workspace-body-execution-receipt/v1' and r['decision'] == 'OBSERVED' and not r.get('error'), 'receipt', 'observed input-only receipt')
    need(r['manifest'] == str(ctx['root'] / 'retry-after-facts.workspace.json') and r['manifest_digest'] == digest(ctx['manifest']) and r['inputs_digest'] == digest(batch['raw']) and 'cases_digest' not in r, 'source', 'exact manifest and batch raw bytes')
    if parent_raw is not None:
        need(r['replayed_from_sha256'] == digest(parent_raw), 'raw_parent', 'saved replay exact raw parent including whitespace')
        need(equal(r['result']['program'], parent_program), 'raw_parent', 'saved program unchanged')
    p, c, rt = (r['result'][k] for k in ['program', 'composition', 'runtime'])
    need(rt['producer_source_sha'] == REV and rt['go_version'] == ctx['go_version'] and rt['go_tool_selection'] == 'explicit' and rt['go_tool_sha256'] == ctx['go_sha'] and equal(rt['producer_module'], ctx['producer_module']), 'go_identity', 'actual pinned native producer/tool digest')
    image = p['workspace_image']
    need(image['init_order'] == ['go/strconv', PACKAGE] and [pkg['path'] for pkg in image['packages']] == ['go/strconv', PACKAGE], 'source', 'exact two-package source closure')
    observed_sources = [(pkg['path'], source['filename'], source['source_digest']) for pkg in image['packages'] for source in pkg['sources']]
    need(equal(observed_sources, [('go/strconv', 'ports/strconv/decimal.gooo', digest(go_json(ctx['sources'][0][0]))), (PACKAGE, 'examples/retry-after-facts/main.gooo', digest(go_json(ctx['sources'][1][0])))]), 'source', 'source digest independently marshaled original text')
    need(equal(image['entry'], {'package_path': PACKAGE, 'source': 'examples/retry-after-facts/main.gooo', 'activity': 'AssessDelay', 'inputs': INPUT_TYPES, 'output': 'RetryAfterFacts'}), 'six_abi', 'source entry six scalar ABI')
    need(equal(p['pure_calls'], ctx['pure']), 'body', 'source body hashes and exact one resolved source call site')
    need(p['lowered_gooo_source'] == ctx['lowered'] == c['gooo_source'], 'source_lowering', 'independently reconstructed two-source lowered Gooo')
    source_digest = digest(ctx['lowered'].encode())
    need(r['result']['execution_source_sha256'] == c['original_source_sha256'] == c['selected_source_sha256'] == rt['original_source_sha256'] == rt['selected_source_sha256'] == source_digest, 'generated_hash', 'all lowered source byte hashes')
    need(c['stage'] == rt['stage'] == 'COMPLETE' and not rt.get('failure') and equal([rt['finite_total'], rt['finite_passed'], rt['model_calls']], [0, 0, 0]) and rt['projection_replayed'] is True and rt['runtime_replayed'] is True, 'process', 'complete input-only zero-model native observation')
    need(len(c['steps']) == 1 and c['steps'][0]['input_source_sha256'] == source_digest, 'generated_hash', 'one source-bound generation')
    generation = c['steps'][0]['generation']; report = generation['report']; plan = c['plan']
    need(report['schema'] == 'gooo/body-codegen-report/v3' and report['decision'] == 'PASS' and report['activity'] == ctx['names'][1] and report['activity_id'] == ENTRY and report['compiler_source_sha'] == REV and report['source_digest'] == source_digest, 'go_identity', 'typed generation identity/source')
    need(equal(report['input_parameters'], PARAMETERS) and report['input_type'] == '(string,bool,int64,int64,bool,bool)' and report['output_type'] == 'RetryAfterFacts', 'six_abi', 'exact six generated parameter kinds')
    need(equal(plan['record_types'], ctx['records']) and equal(report['record_types'], ctx['records']), 'record_abi', 'both record types/IDs/field types and generated names')
    need(equal(report['call_closure'], ctx['closure']), 'callee', 'only one ParseInt pure call and exact source body digest')
    scope = report['completeness_receipt']['scope']
    need(equal(scope['record_types'], ctx['records']) and equal(scope['call_closure'], ctx['closure']), 'record_abi', 'typed scope records/call closure consistency')
    activity = {'name': ctx['names'][1], 'id': ENTRY, 'input_type': 'Text', 'output_type': 'RetryAfterFacts', 'input_entity_id': INPUT_IDS[0], 'output_entity_id': RESULT, 'go_function': 'GoooComposedActivity0', 'input_from': -1, 'assembling': False, 'inputs': [{'port': 'input'+str(i), 'type': t, 'entity_id': INPUT_IDS[i], 'from': -1} for i, t in enumerate(INPUT_TYPES)]}
    need(equal(plan['activities'], [activity]) and plan['edges'] == [] and plan['entry_activity'] == ctx['names'][1], 'six_abi', 'one source-bound six-root graph')
    need(rt['typed_plan_sha256'] == plan['typed_plan_sha256'] and re.fullmatch(r'sha256:[0-9a-f]{64}', plan['typed_plan_sha256']) is not None, 'six_abi', 'reported typed-plan binding only; canonical recomputation NOT_CHECKED')
    need(report['generated_digest'] == report['replay_digest'] == digest(generation['source'].encode()) and c['generated_sha256'] == rt['generated_sha256'] == digest(c['source'].encode()) and c['driver_sha256'] == rt['driver_sha256'] == digest(c['driver_source'].encode()), 'generated_hash', 'actual generated/final/driver source byte digests')
    rename = {r['go_name']: r['name'] for r in ctx['records']}
    rename.update({f['go_name']: f['name'] for r in ctx['records'] for f in r['fields']})
    rename.update({ctx['names'][0]: 'CALL_PARSE', 'GoooComposedActivity0Call0': 'CALL_PARSE'})
    params = ', '.join(p['name'] + ' ' + p['type'] for p in PARAMETERS)
    for text, names in [(generation['source'], ctx['names']), (c['source'], ['GoooComposedActivity0Call0', 'GoooComposedActivity0'])]:
        funcs = go_parts(text, names, ctx['records'])
        for index, name in enumerate(names):
            args, output, body = funcs[name]
            need(args == ('input string' if index == 0 else params) and output == ctx['records'][index]['go_name'], 'six_abi', 'generated helper/entry typed function ABI')
            need(tokens(body, rename) == tokens(ctx['bodies'][index], rename), 'body', 'fixed-profile body tokens preserved through generation/composition; comments/types-only normalization')
    driver = 'package main\n\nimport (\n\t"encoding/json"\n\t"os"\n)\n\nfunc main() {\n\tvar in [][6]json.RawMessage\n\tif json.NewDecoder(os.Stdin).Decode(&in) != nil || len(in) > 128 {\n\t\tos.Exit(2)\n\t}\n\tout := make([][1]json.RawMessage, len(in))\n\tfor c, row := range in {\n'
    for i, param in enumerate(PARAMETERS):
        driver += '\t\tvar input%d %s\n\t\tif json.Unmarshal(row[%d], &input%d) != nil {\n\t\t\tos.Exit(2)\n\t\t}\n' % (i, param['type'], i, i)
    driver += '\t\tv0 := GoooComposedActivity0(input0, input1, input2, input3, input4, input5)\n\t\tout[c][0], _ = json.Marshal(v0)\n\t}\n\tif json.NewEncoder(os.Stdout).Encode(out) != nil {\n\t\tos.Exit(3)\n\t}\n}\n'
    need(c['driver_source'] == driver, 'six_abi', 'exact six typed input decoder/one result driver')
    need(len(rt['runs']) == 2 and rt['toolchain']['stdout_sha256'] == digest(ctx['version_raw']), 'go_identity', 'two native runs and observed Go version stdout hash')
    for process in [rt['toolchain'], rt['build'], *rt['runs']]:
        need(process['started'] is True and process['completed'] is True and equal(process['exit_code'], 0) and all(process[k] is False for k in ['canceled', 'timed_out', 'output_truncated']) and process['diagnostics_bytes'] == 0 and process['stderr_sha256'] == digest(b''), 'process', 'closed successful native child process')
    for process in rt['runs']:
        need(process['stdout_sha256'] == batch['wire_digest'], 'native_stdout', 'independent typed14-field Go stream / actual recorded native stdout hash')
    need(len(rt['traces']) == batch['end'] - batch['start'], 'typedfield', 'all ordered trace rows')
    for index, trace in enumerate(rt['traces']):
        row = ctx['oracle'][batch['start'] + index]
        need(equal(trace['case_index'], index) and len(trace['deliveries']) == 1, 'typedfield', 'one ordered root delivery')
        delivery = trace['deliveries'][0]
        need(delivery['activity_id'] == ENTRY and equal(delivery['actual'], row['result']), 'typedfield', 'all14 independent Go values and exact scalar types')
        need(equal(delivery['inputs'], [{'port': 'input'+str(i), 'entity_id': INPUT_IDS[i], 'value': row['input'][n]} for i, n in enumerate(INPUT_NAMES)]), 'six_abi', 'all six original typed trace inputs')
        need(equal(delivery['actual_fields'], [{'name': f['name'], 'id': f['id'], 'value': row['result'][f['name']]} for f in ctx['fields']]), 'typedfield', '14 ordered stable field IDs/values')
    return {'batch': batch['index'], 'cases': len(rt['traces']), 'typed_field_checks': len(rt['traces']) * 14, 'input_scalar_checks': len(rt['traces']) * 6, 'native_runs': 2, 'native_stdout_expected': batch['wire_ref'], 'generated_source': reference(generation['source'].encode()), 'composed_source': reference(c['source'].encode()), 'lowered_source': reference(ctx['lowered'].encode()), 'record_types': 2, 'pure_call_sites': 1}


def validate(raw, ctx, batch, parent_raw=None, parent_program=None):
    """Check one actual receipt and return its exact source-bound program.

    Roots, corpus rows and expected values are selected before reading the
    receipt. All receipt path strings are compared as data, never opened.
    Canonical inner plan/prior-result hashes remain explicit NOT_CHECKED.
    """
    try:
        selected = _batch(ctx, batch)
        receipt = load(raw)
        need((parent_raw is None) == (parent_program is None), 'raw_parent', 'both raw parent and program required together')
        _verify(receipt, ctx, selected, parent_raw, parent_program)
        return copy.deepcopy(receipt['result']['program'])
    except ReceiptError:
        raise
    except (KeyError, TypeError, ValueError, IndexError, AttributeError, UnicodeError) as error:
        raise ReceiptError('receipt', 'malformed receipt/typed graph: ' + str(error)) from error


def negative_controls(execraw, replayraw, ctx, batch):
    """Refuse eleven in-memory corruptions; original/source/expected bytes stay intact."""
    selected = _batch(ctx, batch)
    original_program = validate(execraw, ctx, selected)
    validate(replayraw, ctx, selected, execraw, original_program)
    execution = load(execraw)
    controls = []
    mutations = [
        ('native-stdout-hash', 'native_stdout', lambda r: r['result']['runtime']['runs'][0].update(stdout_sha256='sha256:'+'0'*64)),
        ('typed-false-is-not-int-zero', 'typedfield', lambda r: r['result']['runtime']['traces'][0]['deliveries'][0]['actual'].update(parsed_value=False)),
        ('native-go-identity', 'go_identity', lambda r: r['result']['runtime'].update(go_tool_sha256='sha256:'+'0'*64)),
        ('original-source-image-hash', 'source', lambda r: r['result']['program']['workspace_image']['packages'][0]['sources'][0].update(source_digest='sha256:'+'0'*64)),
        ('source-body-hash', 'body', lambda r: r['result']['program']['pure_calls']['activities'][0].update(source_body_sha256='sha256:'+'0'*64)),
        ('lowered-body-injection', 'source_lowering', lambda r: r['result']['program'].update(lowered_gooo_source=r['result']['program']['lowered_gooo_source']+'\nactivity Injected() -> Text')),
        ('pure-callee-id', 'callee', lambda r: r['result']['composition']['steps'][0]['generation']['report']['call_closure']['edges'][0].update(callee_id='gooo://foreign/callee')),
        ('two-record-field-type', 'record_abi', lambda r: r['result']['composition']['steps'][0]['generation']['report']['record_types'][1]['fields'][0].update(type_id='urn:gooo:type:boolean')),
        ('six-parameter-kind', 'six_abi', lambda r: r['result']['composition']['steps'][0]['generation']['report']['input_parameters'][0].update(type='bool')),
        ('generated-source-byte-drift', 'generated_hash', lambda r: r['result']['composition']['steps'][0]['generation'].update(source=r['result']['composition']['steps'][0]['generation']['source']+'\n')),
    ]
    for name, boundary, mutate in mutations:
        changed = copy.deepcopy(execution)
        mutate(changed)
        try:
            _verify(changed, ctx, selected)
        except ReceiptError as error:
            need(error.boundary == boundary, 'control', name + ' refused at unexpected boundary: ' + str(error))
            controls.append({'name': name, 'status': 'REFUSED_AT_EXPECTED_BOUNDARY', 'boundary': boundary, 'diagnostic': str(error)})
        else:
            raise ReceiptError('control', name + ' was incorrectly accepted')
    try:
        _verify(load(replayraw), ctx, selected, execraw + b'\n', original_program)
    except ReceiptError as error:
        need(error.boundary == 'raw_parent', 'control', 'whitespace parent refused at wrong boundary')
        controls.append({'name': 'raw-parent-whitespace-is-not-same-parent', 'status': 'REFUSED_AT_EXPECTED_BOUNDARY', 'boundary': 'raw_parent', 'diagnostic': str(error)})
    else:
        raise ReceiptError('control', 'raw-parent whitespace mutation accepted')
    need(len(controls) == 11 and len({row['name'] for row in controls}) == 11, 'control', 'exact eleven refusal controls')
    return controls
