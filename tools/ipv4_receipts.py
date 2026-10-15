"""Public IPv4 input/receipt checks; no target execution or import-time work.

The Gooo source owns classification. Fresh independent Go output supplies the
expected values; this module checks transport, provenance and typed results.
"""
from decimal import Decimal
from pathlib import Path
import hashlib
import json
import re
import stat

REV = '83928fa4538245abe33b08549da47f8cbdc50ed2'
PORT = 'go/net/netip'
APP = 'app/ipv4-address-facts'
REPORT_ENTRY = APP + ':Report'
RESULT = 'gooo://ipv4-address-facts/result/v1'
INTEGER, BOOLEAN = 'gooo://std/integer', 'gooo://std/boolean'
METHODS = ('IsPrivate', 'IsLoopback', 'IsUnspecified', 'IsMulticast', 'IsLinkLocalUnicast', 'IsGlobalUnicast')
FLAGS = ('is_private', 'is_loopback', 'is_unspecified', 'is_multicast', 'is_link_local_unicast', 'is_global_unicast')
FIELDS = ('a', 'b', 'c', 'd', 'valid') + FLAGS
PORT_SOURCE = 'ports/net/netip/ipv4.gooo'
APP_SOURCE = 'examples/ipv4-address-facts/main.gooo'
IDS = dict(zip(METHODS, ('gooo://go/net/netip/ipv4/' + name for name in
                       ('is-private', 'is-loopback', 'is-unspecified', 'is-multicast', 'is-link-local-unicast', 'is-global-unicast'))))
IDS.update(IPv4Valid='gooo://go-ports/net/netip/ipv4/valid', Report='gooo://ipv4-address-facts/report/v1')
MANIFESTS = dict(zip((PORT + ':' + name for name in METHODS),
                    ('ipv4-is-private.workspace.json', 'ipv4-is-loopback.workspace.json',
                     'ipv4-is-unspecified.workspace.json', 'ipv4-is-multicast.workspace.json',
                     'ipv4-is-link-local-unicast.workspace.json', 'ipv4-is-global-unicast.workspace.json')))
MANIFESTS[REPORT_ENTRY] = 'ipv4-address-facts.workspace.json'


class InputError(ValueError):
    def __init__(self, code):
        self.code = code
        super().__init__(code)


class ReceiptError(ValueError):
    def __init__(self, code):
        self.code = code
        super().__init__(code)


def need(ok, message):
    if not ok:
        raise ReceiptError(message)


def exact(left, right):
    if type(left) is not type(right):
        return False
    if type(left) is dict:
        return set(left) == set(right) and all(exact(left[key], right[key]) for key in left)
    if type(left) is list:
        return len(left) == len(right) and all(exact(a, b) for a, b in zip(left, right))
    return left == right


def digest(raw):
    return 'sha256:' + hashlib.sha256(raw).hexdigest()


def is_digest(value):
    return type(value) is str and re.fullmatch(r'sha256:[0-9a-f]{64}', value) is not None


def integer(value, want=None):
    return type(value) is int and -(1 << 63) <= value < (1 << 63) and (want is None or value == want)


def strict_json(raw, error=ReceiptError):
    """Preserve fractional diagnostic kinds; reject duplicate decoded keys."""
    if type(raw) is not bytes:
        raise error('RAW_BYTES_REQUIRED')
    try:
        text = raw.decode('utf-8')
    except UnicodeDecodeError:
        raise error('INVALID_RAW_UTF8') from None

    def pairs(items):
        result = {}
        for key, value in items:
            if key in result:
                raise error('DUPLICATE_FIELD')
            result[key] = value
        return result

    def nonfinite(_):
        raise error('MALFORMED_JSON')

    decoder = json.JSONDecoder(object_pairs_hook=pairs, parse_float=Decimal, parse_constant=nonfinite)
    try:
        start = len(text) - len(text.lstrip())
        value, end = decoder.raw_decode(text, start)
    except (json.JSONDecodeError, RecursionError):
        raise error('MALFORMED_JSON') from None
    if text[end:].strip():
        raise error('TRAILING_JSON')
    return value


def load_receipt(raw):
    if len(raw) > 8 << 20:
        raise ReceiptError('RECEIPT_BYTE_BOUND')
    return strict_json(raw)


def validate_inputs(raw, entry):
    """Strict public boundary; semantic out-of-octet int64s remain valid inputs."""
    if entry not in MANIFESTS:
        raise InputError('ENTRY_REQUIRED')
    if type(raw) is not bytes:
        raise InputError('RAW_BYTES_REQUIRED')
    if len(raw) > 8192:
        raise InputError('BYTE_BOUND')
    document = strict_json(raw, InputError)
    if type(document) is not dict:
        raise InputError('DOCUMENT_OBJECT')
    if set(document) != {'schema', 'inputs'}:
        raise InputError('DOCUMENT_FIELDS')
    if document['schema'] != 'gooo/body-composition-inputs/v1':
        raise InputError('SCHEMA_REQUIRED')
    rows = document['inputs']
    if type(rows) is not list or not 1 <= len(rows) <= 24:
        raise InputError('ROW_COUNT')
    names = [entry + '.input' + str(i) for i in range(4)]
    for row in rows:
        if type(row) is not dict:
            raise InputError('ROW_OBJECT')
        for i, name in enumerate(names):
            if name not in row:
                raise InputError('MISSING_PORT:input' + str(i))
        if set(row) != set(names):
            raise InputError('UNKNOWN_PORT')
        for i, name in enumerate(names):
            if type(row[name]) is not int:
                raise InputError('INTEGER_TOKEN:input' + str(i))
            if not -(1 << 63) <= row[name] < (1 << 63):
                raise InputError('INTEGER_RANGE:input' + str(i))
    return rows


def _json_bytes(value):
    return (json.dumps(value, separators=(',', ':')) + '\n').encode('utf-8')


def input_baseline(entry=REPORT_ENTRY):
    return _json_bytes({'schema': 'gooo/body-composition-inputs/v1', 'inputs': [
        {entry + '.input' + str(i): v for i, v in enumerate((10, 0, 0, 1))}]})


def wire_controls():
    """Deterministic raw bytes, frozen by caller before preflight evaluation."""
    entry = REPORT_ENTRY
    baseline = input_baseline(entry)
    document = json.loads(baseline)
    controls = []
    families = [('missing', None, 'MISSING_PORT'), ('null', b'null', 'INTEGER_TOKEN'),
                ('boolean', b'true', 'INTEGER_TOKEN'), ('string', b'"1"', 'INTEGER_TOKEN'),
                ('fractional', b'1.0', 'INTEGER_TOKEN'), ('exponent', b'1e0', 'INTEGER_TOKEN'),
                ('above-int64', b'9223372036854775808', 'INTEGER_RANGE'),
                ('below-int64', b'-9223372036854775809', 'INTEGER_RANGE')]
    for port in range(4):
        key = entry + '.input' + str(port)
        for family, token, code in families:
            row = dict(document['inputs'][0])
            if token is None:
                del row[key]
                changed = _json_bytes({'schema': document['schema'], 'inputs': [row]})
            else:
                # Each canonical baseline value has one literal member; only its
                # token is replaced, preserving the other three valid ports.
                member = json.dumps(key).encode() + b':' + str(row[key]).encode()
                changed = baseline.replace(member, json.dumps(key).encode() + b':' + token)
            controls.append({'name': 'input' + str(port) + '-' + family, 'raw': changed,
                             'expected_diagnostic': code + ':input' + str(port)})
    row = dict(document['inputs'][0]); row[entry + '.input4'] = 0
    unknown = _json_bytes({'schema': document['schema'], 'inputs': [row]})
    key = json.dumps(entry + '.input0').encode()
    member = key + b':10'
    duplicate = baseline.replace(member, member + b',' + member)
    escaped_key = json.dumps(entry + '.input0').replace('input0', 'input\\u0030').encode()
    escaped_duplicate = baseline.replace(member, member + b',' + escaped_key + b':10')
    documents = [('unknown', unknown, 'UNKNOWN_PORT'),
                 ('duplicate-exact', duplicate, 'DUPLICATE_FIELD'),
                 ('duplicate-escaped', escaped_duplicate, 'DUPLICATE_FIELD'),
                 ('raw-invalid-UTF8', baseline[:-1] + b'\xff', 'INVALID_RAW_UTF8'),
                 ('trailing', baseline + b'{}', 'TRAILING_JSON'),
                 ('malformed', baseline[:-3], 'MALFORMED_JSON'),
                 ('nonobject', b'[]\n', 'DOCUMENT_OBJECT'),
                 ('empty-rows', _json_bytes({'schema': document['schema'], 'inputs': []}), 'ROW_COUNT'),
                 ('over24-rows', _json_bytes({'schema': document['schema'], 'inputs': document['inputs'] * 25}), 'ROW_COUNT'),
                 ('over8192-bytes', baseline + b' ' * (8193 - len(baseline)), 'BYTE_BOUND')]
    controls.extend({'name': name, 'raw': raw, 'expected_diagnostic': code} for name, raw, code in documents)
    if len(controls) != 42 or len({c['name'] for c in controls}) != 42:
        raise InputError('CONTROL_ROSTER')
    return controls


def _saved_ref(callback, name, raw):
    value = callback(name, raw)
    need(type(value) is dict and type(value.get('path')) is str and
         type(value.get('bytes')) is int and value['bytes'] == len(raw) and
         value.get('sha256') == hashlib.sha256(raw).hexdigest(), 'CONTROL_RAW_REF')
    return value


def run_wire_controls(save_callback=None):
    controls = wire_controls()
    # Retain all raw controls before evaluating any negative. The caller's
    # source freeze binds this deterministic generator, with no historic output.
    if save_callback is not None:
        for case in controls:
            case['rawref'] = _saved_ref(save_callback, 'wire-' + case['name'], case['raw'])
    baselines = []
    for entry in MANIFESTS:
        raw = input_baseline(entry)
        if save_callback is not None:
            _saved_ref(save_callback, 'wire-baseline-' + entry.split(':')[-1], raw)
        validate_inputs(raw, entry)
        baselines.append(entry)
    records = []
    for case in controls:
        try:
            validate_inputs(case['raw'], REPORT_ENTRY)
        except InputError as exc:
            if exc.code != case['expected_diagnostic']:
                raise InputError('UNINTENDED_DIAGNOSTIC:' + case['name']) from exc
            records.append({**case, 'code': exc.code, 'observed_diagnostic': exc.code,
                            'baseline_sha256': hashlib.sha256(input_baseline()).hexdigest(),
                            'baseline_entry_count': len(baselines)})
        else:
            raise InputError('CONTROL_ACCEPTED:' + case['name'])
    return records


def go_string(value):
    return json.dumps(value, ensure_ascii=False, separators=(',', ':')).replace('&', r'\u0026').replace('<', r'\u003c').replace('>', r'\u003e').replace('\u2028', r'\u2028').replace('\u2029', r'\u2029').encode('utf-8')


def activity_bodies(source):
    matches = re.findall(r'\bactivity\s+(\w+)\s*\([^)]*\)[^`]*?\bcomputes\s+("(?:\\.|[^"\\])*"|`[^`]*`)', source, re.S)
    result = {}
    for name, token in matches:
        need(name not in result, 'Repeated computes activity')
        result[name] = token[1:-1] if token.startswith('`') else json.loads(token)
        need(type(result[name]) is str, 'Computes body is not a string')
    return result


def _read_regular(path, limit):
    path = Path(path)
    need(path.is_absolute() and path.resolve() == path, 'CANONICAL_FILE_REQUIRED')
    before = path.lstat()
    need(stat.S_ISREG(before.st_mode) and before.st_size <= limit, 'REGULAR_FILE_BOUND')
    raw = path.read_bytes()
    after = path.lstat()
    need((before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns) ==
         (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns), 'FILE_CHANGED_DURING_READ')
    return raw


class Evidence:
    def __init__(self, repository, oracle, selected_go_version):
        self.repository = Path(repository).resolve()
        self.go_version = selected_go_version
        need(type(selected_go_version) is str and re.fullmatch(r'go version go1\.27\.2 [a-z0-9]+/[a-z0-9]+', selected_go_version) is not None,
             'SELECTED_GO_VERSION')
        corpus = strict_json(_read_regular(self.repository / 'examples/ipv4-address-facts/corpus-202.json', 128 << 10))
        original = strict_json(_read_regular(self.repository / 'examples/ipv4-address-facts/corpus.json', 32 << 10))
        need(type(oracle) is list and len(oracle) == 202 and len(corpus['cases']) == 202 and len(original['cases']) == 64,
             'ORACLE_CORPUS_COUNT')
        need(exact(corpus['cases'][:64], original['cases']), 'ORIGINAL64_PREFIX')
        for i, (row, frozen) in enumerate(zip(oracle, corpus['cases'])):
            need(integer(row.get('index'), i) and row.get('name') == frozen['name'] and row.get('cohort') == frozen['cohort'] and
                 exact(row.get('inputs'), frozen['inputs']), 'ORACLE_CORPUS_BINDING')
            need(type(row['inputs']) is dict and set(row['inputs']) == set(FIELDS[:4]) and
                 all(integer(v) for v in row['inputs'].values()), 'ORACLE_INPUT_KINDS')
            expected = row.get('expected')
            need(type(expected) is dict and set(expected) == set(FIELDS) and
                 all(integer(expected[f]) and exact(expected[f], row['inputs'][f]) for f in FIELDS[:4]) and
                 all(type(expected[f]) is bool for f in FIELDS[4:]), 'ORACLE_REPORT_KINDS_ECHOES')
            valid = all(0 <= row['inputs'][f] <= 255 for f in FIELDS[:4])
            need(expected['valid'] is valid and (valid or not any(expected[f] for f in FLAGS)), 'ORACLE_DOMAIN_EXTENSION')
        self.rows = oracle
        self.bodies = {}
        for package, filename, names in ((PORT, PORT_SOURCE, ('IPv4Valid',) + METHODS), (APP, APP_SOURCE, ('Report',))):
            raw = _read_regular(self.repository / filename, 32 << 10)
            bodies = activity_bodies(raw.decode('utf-8'))
            need(set(bodies) == set(names), 'Original computes-body roster differs')
            for name in names:
                key = package + ':' + name
                self.bodies[key] = {'body': bodies[name], 'source': filename, 'id': IDS[name]}

    def graph(self, entry, lowered=None):
        queue, sites, bodies = [entry], [], {}
        for key in queue:
            body = self.bodies[key]['body']
            edits = []
            for match in re.finditer(r'((?:netip\.)?[A-Z]\w*)\(', body):
                token = match.group(1)
                name = token.split('.')[-1]
                callee = PORT + ':' + name
                need(callee in self.bodies, 'Unexpected source call token')
                start, end = match.span(1)
                sites.append({'caller': key, 'callee': callee, 'surface': 'computes',
                              'start_byte': len(body[:start].encode()), 'end_byte': len(body[:end].encode())})
                if lowered is not None:
                    edits.append((start, end, lowered[callee]))
                if callee not in queue:
                    queue.append(callee)
            for start, end, name in reversed(edits):
                body = body[:start] + name + body[end:]
            bodies[key] = body
        return queue, sites, bodies

    def workspace(self, image, entry):
        report = entry.startswith(APP + ':')
        packages = {PORT: (PORT_SOURCE, 'netip', 'gooo_ports_net_netip')}
        if report:
            packages[APP] = (APP_SOURCE, 'ipv4_address_facts', 'gooo_original_ipv4_address_facts')
        need(image['schema'] == 'gooo/package-runtime-image/v1' and len(image['packages']) == len(packages), 'Workspace image roster')
        need(exact(image['init_order'], [PORT, APP] if report else [PORT]), 'Workspace init/import order')
        need(exact(image['entry'], {'package_path': entry.split(':')[0], 'source': APP_SOURCE if report else PORT_SOURCE,
             'activity': entry.split(':')[-1], 'inputs': ['Integer'] * 4,
             'output': 'IPv4AddressFacts' if report else 'Boolean'}), 'Workspace original entry/four input types')
        seen = set()
        for package in image['packages']:
            path = package['path']
            need(path in packages and path not in seen, 'Unexpected/repeated workspace package')
            seen.add(path)
            source, name, namespace = packages[path]
            need(package['name'] == name and package['namespace'] == namespace, 'Original package/namespace differs')
            need(exact(package['imports'], [PORT] if path == APP else None), 'Original imports differ')
            sources = package['sources']
            need(type(sources) is list and len(sources) == 1 and sources[0]['filename'] == source and
                 sources[0]['source_digest'] == digest(go_string((self.repository / source).read_text(encoding='utf-8'))), 'Original source image digest differs')
            expected = {'Integer': ('entity', INTEGER)}
            if path == PORT:
                expected['Boolean'] = ('entity', BOOLEAN)
                expected.update({method: ('activity', IDS[method]) for method in ('IPv4Valid',) + METHODS})
            else:
                expected.update({'IPv4AddressFacts': ('entity', RESULT), 'Report': ('activity', IDS['Report'])})
            exports = package['exports']
            need(type(exports) is list and len(exports) == len(expected), 'Workspace export count')
            exported = set()
            for value in exports:
                key = value['name']
                need(key in expected and key not in exported and (value['kind'], value['id']) == expected[key], 'Original export identity')
                exported.add(key)
                if value['kind'] == 'activity':
                    need(exact(value['input_types'], [INTEGER] * 4) and value['output_type'] == (RESULT if key == 'Report' else BOOLEAN), 'Four-Integer activity export ABI')
        return packages

    def process(self, value):
        need(type(value) is dict and value.get('started') is True and value.get('completed') is True and
             value.get('canceled') is False and value.get('timed_out') is False and value.get('output_truncated') is False and
             integer(value.get('diagnostics_bytes'), 0) and integer(value.get('exit_code'), 0) and
             is_digest(value.get('stdout_sha256')) and value.get('stderr_sha256') == digest(b''), 'Incomplete/truncated native process')

    def inspect(self, raw, entry, indices, inputs, manifest, go_tool_digest, original=None, prior_typed=None):
        need(entry in MANIFESTS and type(indices) is list and 1 <= len(indices) <= 24 and
             all(type(i) is int for i in indices) and indices == list(range(indices[0], indices[0] + len(indices))) and
             0 <= indices[0] and indices[-1] < (64 if entry == REPORT_ENTRY else 48), 'FIXED_CLI_COHORT')
        inputs, manifest = Path(inputs), Path(manifest)
        need(manifest == self.repository / MANIFESTS[entry], 'PUBLIC_MANIFEST_PATH')
        manifest_raw = _read_regular(manifest, 8192)
        manifest_value = strict_json(manifest_raw)
        packages = [{'path': PORT, 'sources': [PORT_SOURCE]}]
        if entry == REPORT_ENTRY:
            packages.append({'path': APP, 'sources': [APP_SOURCE]})
        need(exact(manifest_value, {'schema': 'gooo/package-workspace-manifest/v1',
             'entry': {'package_path': entry.split(':')[0], 'activity': entry.split(':')[-1]},
             'packages': packages}), 'PUBLIC_MANIFEST_CONTENT')
        input_raw = _read_regular(inputs, 8192)
        input_rows = validate_inputs(input_raw, entry)
        need(exact(input_rows, [{entry + '.input' + str(i): self.rows[index]['inputs'][field]
             for i, field in enumerate(FIELDS[:4])} for index in indices]), 'ORIGINAL_INPUT_COHORT')
        need(is_digest(go_tool_digest), 'SELECTED_GO_TOOL_DIGEST')
        e = load_receipt(raw)
        name = entry.split(':')[-1]
        report = name == 'Report'
        expected_top = {'schema', 'decision', 'manifest', 'manifest_digest', 'inputs_digest', 'result'}
        if original is not None:
            expected_top.add('replayed_from_sha256')
        need(type(e) is dict and set(e) == expected_top and e['schema'] == 'gooo/workspace-body-execution-receipt/v1' and
             e['decision'] == 'OBSERVED' and e['manifest'] == str(manifest) and
             e['manifest_digest'] == digest(manifest_raw) and e['inputs_digest'] == digest(input_raw), 'Input-only receipt/file binding')
        r, p, c, rt = e['result'], e['result']['program'], e['result']['composition'], e['result']['runtime']
        need(r['schema'] == 'gooo/workspace-body-execution/v1' and p['schema'] == 'gooo/workspace-body-program/v1', 'Workspace/program schema')
        self.workspace(p['workspace_image'], entry)
        needed, sites, _ = self.graph(entry)
        calls = p.get('pure_calls')
        need(type(calls) is dict and calls.get('schema') == 'gooo/workspace-pure-calls/v1' and
             type(calls.get('sites')) is list and
             sorted(json.dumps(site, sort_keys=True, separators=(',', ':')) for site in calls['sites']) ==
             sorted(json.dumps(site, sort_keys=True, separators=(',', ':')) for site in sites), 'Exact source-declared call-site multiset/byte spans')
        activities = calls['activities']
        need(type(activities) is list and len(activities) == len(needed), 'Pure-call activity closure count')
        lowered_names = {}
        source_bytes = sum((self.repository / filename).stat().st_size for filename in ({PORT_SOURCE, APP_SOURCE} if report else {PORT_SOURCE}))
        for value in activities:
            ref = value['activity']
            key = ref['package_path'] + ':' + ref['activity']
            need(key in needed and key not in lowered_names, 'Unexpected/repeated pure-call activity')
            package, activity = key.split(':')
            lowered_name = value['activity'].get('lowered_name')
            match = re.fullmatch(r'GoooPackage([0-9]+)Activity' + re.escape(activity), lowered_name) if type(lowered_name) is str else None
            need(match is not None and int(match.group(1)) < source_bytes and lowered_name not in lowered_names.values(), 'Source-bound lowered activity name')
            lowered_names[key] = lowered_name
            need(exact(value['activity'], {'package_path': package, 'activity': activity, 'lowered_name': lowered_name}) and
                 value['source'] == self.bodies[key]['source'] and value['source_body_sha256'] == digest(self.bodies[key]['body'].encode()), 'Original pure-call body/source binding')
        expected_ref = {'package_path': entry.split(':')[0], 'activity': name, 'lowered_name': lowered_names[entry]}
        need(exact(p['entry'], expected_ref) and exact(p['activities'], [expected_ref]), 'Selected original activity identity')
        _, _, rewritten = self.graph(entry, lowered_names)
        lowered = p['lowered_gooo_source']
        actual_bodies = activity_bodies(lowered)
        need(type(lowered) is str and lowered and set(actual_bodies) == {lowered_names[key] for key in needed} and
             all(actual_bodies[lowered_names[key]] == rewritten[key] for key in needed), 'Lowered bodies differ from exact source call-name rewriting')
        source_digest = digest(lowered.encode())
        need(r['execution_source_sha256'] == source_digest and c['schema'] == 'gooo/body-composition/v1' and c['stage'] == 'COMPLETE' and
             c['original_source_sha256'] == c['selected_source_sha256'] == source_digest and c['gooo_source'] == lowered, 'Execution/lowered source crosslink')
        need(not any(r.get(key) is not None for key in ('body_fills', 'assembly_policy', 'assembly_policy_history', 'continuation')), 'Unexpected body fill/policy/continuation')
        plan = c['plan']
        need(plan['schema'] == 'gooo/body-composition-plan/v1' and plan.get('entry_activity') == lowered_names[entry] and
             is_digest(plan['typed_plan_sha256']) and re.fullmatch('[0-9a-f]{64}', plan['semantic_fingerprint']) and
             exact(plan['edges'], []) and plan.get('preparations') is None and len(plan['activities']) == 1, 'Typed single-root composition plan')
        a = plan['activities'][0]
        need(exact(a, {'name': lowered_names[entry], 'id': IDS[name], 'input_type': 'Integer',
                       'output_type': 'IPv4AddressFacts' if report else 'Boolean', 'input_entity_id': INTEGER,
                       'output_entity_id': RESULT if report else BOOLEAN, 'go_function': 'GoooComposedActivity0',
                       'input_from': -1, 'assembling': False,
                       'inputs': [{'port': 'input' + str(i), 'type': 'Integer', 'entity_id': INTEGER, 'from': -1} for i in range(4)]}), 'Four-Integer/native result ABI')
        if report:
            records = plan['record_types']
            need(len(records) == 1 and exact(records[0], {'name': 'IPv4AddressFacts', 'id': RESULT,
                 'go_name': 'GoooRecord' + digest(RESULT.encode())[7:],
                 'fields': [{'name': field, 'id': RESULT + '/' + field,
                             'type_id': 'urn:gooo:type:' + ('integer' if index < 4 else 'boolean'),
                             'go_name': 'GoooField' + digest((RESULT + '/' + field).encode())[7:]} for index, field in enumerate(FIELDS)]}), 'Required11-field native record ABI')
        else:
            need(plan.get('record_types') is None, 'Scalar unexpectedly has record ABI')
        steps = c['steps']
        need(type(steps) is list and len(steps) == 1 and steps[0]['input_source_sha256'] == source_digest, 'One root body generation step')
        generation = steps[0]['generation']
        gr = generation['report']
        need(gr['program_digest'] == digest(rewritten[entry].encode()), 'ROOT_PROGRAM_DIGEST')
        need(gr['input_type'] == '(int64,int64,int64,int64)' and exact(gr['input_parameters'], [{'name': 'input' + str(i), 'type': 'int64'} for i in range(4)]), 'GENERATION_ABI')
        need(gr['schema'] == 'gooo/body-codegen-report/v3' and gr['decision'] == 'PASS' and gr['activity'] == lowered_names[entry] and
             gr['activity_id'] == IDS[name] and gr['compiler_source_sha'] == REV and gr['source_digest'] == source_digest and
             gr['program_digest'] == digest(rewritten[entry].encode()) and
             gr['generated_digest'] == gr['replay_digest'] == digest(generation['source'].encode()) and
             gr['input_type'] == '(int64,int64,int64,int64)' and exact(gr['input_parameters'], [{'name': 'input' + str(i), 'type': 'int64'} for i in range(4)]) and
             gr['output_type'] == ('IPv4AddressFacts' if report else 'bool') and exact(gr.get('record_types'), plan.get('record_types')) and gr['typecheck_passed'] is True and
             gr['deterministic_replay'] is True and integer(gr['repository_writes'], 0), 'Generation/root/typecheck/source/replay provenance')
        closure = gr['call_closure']
        need(closure['schema'] == 'gooo/pure-activity-call-closure/v1' and
             {(value['name'], value['activity_id']) for value in closure['activities']} ==
             {(lowered_names[key], self.bodies[key]['id']) for key in needed if key != entry} and
             len(closure['activities']) == len(needed) - 1, 'Generated callee identity closure')
        by_lowered = {lowered_names[key]: key for key in needed}
        need(all(value['program_sha256'] == digest(rewritten[by_lowered[value['name']]].encode()) for value in closure['activities']), 'Generated callee program/body digest')
        expected_edges = []
        for key in needed:
            outgoing = [site for site in sites if site['caller'] == key]
            expected_edges.extend((self.bodies[key]['id'], self.bodies[site['callee']]['id'], i) for i, site in enumerate(outgoing))
        need(all(integer(edge.get('ordinal')) for edge in closure['edges']) and
             sorted((edge['caller_id'], edge['callee_id'], edge['ordinal']) for edge in closure['edges']) == sorted(expected_edges), 'Generated nested call edges')
        def call_budget(key, stack=()):
            need(key not in stack and len(stack) < 16, 'Unexpected recursive source closure')
            return sum(1 + call_budget(site['callee'], stack + (key,)) for site in sites if site['caller'] == key)
        need(integer(closure['max_calls_per_invocation'], call_budget(entry)), 'Source-derived static call budget')
        generated, driver = c['source'], c['driver_source']
        need(type(generated) is str and generated and type(driver) is str and driver and
             c['generated_sha256'] == digest(generated.encode()) and c['driver_sha256'] == digest(driver.encode()), 'Generated/driver raw source digest')
        need(rt['schema'] == 'gooo/body-composition-runtime/v1' and rt['stage'] == 'COMPLETE' and rt.get('failure') is None and
             rt['producer_source_sha'] == REV and rt['go_version'] == self.go_version and
             rt['go_tool_sha256'] == go_tool_digest and rt['go_tool_selection'] == 'explicit' and
             rt['producer_module']['path'] == 'github.com/kimjooyoon/meta-ontology-go' and
             rt['producer_module']['version'] == 'v0.0.0-20261011201109-83928fa45382', 'Fresh pinned producer/tool identity')
        need(rt['typed_plan_sha256'] == plan['typed_plan_sha256'], 'TYPED_PLAN_BINDING')
        need(rt['generated_sha256'] == c['generated_sha256'] and rt['driver_sha256'] == c['driver_sha256'] and
             rt['original_source_sha256'] == rt['selected_source_sha256'] == source_digest and rt['typed_plan_sha256'] == plan['typed_plan_sha256'] and
             is_digest(rt['executable_sha256']) and is_digest(rt['runtime_suite_sha256']) and integer(rt['model_calls'], 0) and
             integer(rt['finite_total'], 0) and integer(rt['finite_passed'], 0) and rt['projection_replayed'] is True and rt['runtime_replayed'] is True, 'Native generated/typed-plan/input-only counters')
        self.process(rt['toolchain'])
        self.process(rt['build'])
        need(type(rt['runs']) is list and len(rt['runs']) == 2, 'Exactly two fresh native executions required')
        for process in rt['runs']:
            self.process(process)
        need(rt['runs'][0]['stdout_sha256'] == rt['runs'][1]['stdout_sha256'], 'Two fresh native stdout values differ')
        if original is None:
            need(r.get('replay') is None, 'Unexpected saved replay metadata')
        else:
            prior = load_receipt(original)
            replay = r['replay']
            need(e['replayed_from_sha256'] == digest(original), 'REPLAY_RAW_PARENT')
            need(is_digest(prior_typed) and replay['prior_result_sha256'] == prior_typed, 'REPLAY_TYPED_RESULT')
            need(replay['schema'] == 'gooo/workspace-body-replay/v1' and
                 integer(replay['model_calls'], 0) and integer(replay['body_fills_replayed'], 0) and
                 exact(p, prior['result']['program']) and exact(c, prior['result']['composition']), 'Outer raw-parent/typed-inner saved replay or original program/composition differs')
        traces = rt['traces']
        need(type(traces) is list and len(traces) == len(indices), 'Exact ordered trace roster')
        outputs = []
        for local, (trace, index) in enumerate(zip(traces, indices)):
            need(integer(trace['case_index'], local) and len(trace['deliveries']) == 1, 'Trace local index/one delivery')
            delivery = trace['deliveries'][0]
            row = self.rows[index]
            expected = row['expected'] if report else row['expected'][FLAGS[METHODS.index(name)]]
            need(delivery['activity_id'] == IDS[name] and exact(delivery.get('inputs'),
                 [{'port': 'input' + str(i), 'entity_id': INTEGER, 'value': row['inputs'][field]} for i, field in enumerate(FIELDS[:4])]) and
                 not any(delivery.get(key) is not None for key in ('input', 'input_fields', 'producer_id', 'expected', 'passed', 'fault', 'blocked_by')), 'Original4 typed inputs/activity/input-only delivery')
            actual = delivery.get('actual')
            if report:
                need(type(actual) is dict and set(actual) == set(FIELDS), 'REPORT_REQUIRED_FIELDS')
                need(all(integer(actual[field]) for field in FIELDS[:4]), 'REPORT_INTEGER_KIND')
                need(all(type(actual[field]) is bool for field in FIELDS[4:]), 'REPORT_BOOLEAN_KIND')
                need(all(exact(actual[field], row['inputs'][field]) for field in FIELDS[:4]), 'ORIGINAL_REPORT_ECHO')
            need(exact(actual, expected), 'Actual typed value differs from independent Go oracle')
            if report:
                need(exact(delivery['actual_fields'], [{'id': RESULT + '/' + field, 'name': field, 'value': expected[field]} for field in FIELDS]), 'REPORT_FIELD_IDENTITY')
            else:
                need(type(delivery['actual']) is bool and delivery.get('actual_fields') is None, 'Scalar must be Boolean only')
            outputs.append(delivery['actual'])
        return {'entry': entry, 'indices': list(indices), 'output_count': len(indices),
                'original_echoes': len(indices) * 4 if report else 0, 'generated_source': generated, 'values': outputs, 'generated_sha256': c['generated_sha256'], 'driver_sha256': c['driver_sha256'],
                'typed_plan_sha256': plan['typed_plan_sha256'], 'source_sha256': source_digest,
                'native_internal_runs': 2, 'outputs': len(indices), 'typed_scalar_fields': len(indices) * (11 if report else 1)}



def _raw_value(raw, path, token, span_callback):
    """Use the existing public lifecycle span parser, preserving other bytes."""
    text, (start, end) = span_callback(raw, path)
    need(type(text) is str and 0 <= start < end <= len(text), 'TAMPER_VALUE_SPAN')
    changed = (text[:start] + token + text[end:]).encode('utf-8')
    need(changed != raw and changed.startswith(text[:start].encode('utf-8')) and
         changed.endswith(text[end:].encode('utf-8')), 'TAMPER_UNRELATED_BYTES')
    return changed


def _other_digest(value):
    need(type(value) is str and value, 'TAMPER_STRING_REQUIRED')
    return value[:-1] + ('0' if value[-1] != '0' else '1')


def tamper_cases(execution_raw, replay_raw, span_callback):
    """18 named intended gates over fresh Report evidence, never a native run."""
    original, replay = load_receipt(execution_raw), load_receipt(replay_raw)
    need(original['result']['program']['workspace_image']['entry']['activity'] == 'Report' and
         replay['result']['program']['workspace_image']['entry']['activity'] == 'Report', 'TAMPER_REPORT_REQUIRED')
    delivery = ['result', 'runtime', 'traces', 0, 'deliveries', 0]
    generation = ['result', 'composition', 'steps', 0, 'generation', 'report']
    runtime = ['result', 'runtime']
    body = original['result']['runtime']['traces'][0]['deliveries'][0]['actual']
    scalar_without_d = {key: value for key, value in body.items() if key != 'd'}
    specs = [
        ('derivedBoolean', 'execution', delivery + ['actual', 'is_private'],
         not body['is_private'], 'Actual typed value differs from independent Go oracle'),
        ('originalEcho', 'execution', delivery + ['actual', 'a'],
         body['a'] + 1, 'ORIGINAL_REPORT_ECHO'),
        ('integerKind', 'execution', delivery + ['actual', 'a'],
         None, 'REPORT_INTEGER_KIND'),
        ('missingRequiredField', 'execution', delivery + ['actual'],
         scalar_without_d, 'REPORT_REQUIRED_FIELDS'),
        ('fieldID', 'execution', delivery + ['actual_fields', 0, 'id'],
         RESULT + '/wrong-field', 'REPORT_FIELD_IDENTITY'),
        ('originalInputPort', 'execution', delivery + ['inputs', 0, 'port'],
         'input1', 'Original4 typed inputs/activity/input-only delivery'),
        ('sourceImage', 'execution', ['result', 'program', 'workspace_image', 'packages', 0, 'sources', 0, 'source_digest'],
         _other_digest(original['result']['program']['workspace_image']['packages'][0]['sources'][0]['source_digest']),
         'Original source image digest differs'),
        ('rootBody', 'execution', generation + ['program_digest'],
         _other_digest(original['result']['composition']['steps'][0]['generation']['report']['program_digest']), 'ROOT_PROGRAM_DIGEST'),
        ('typedPlan', 'execution', runtime + ['typed_plan_sha256'],
         _other_digest(original['result']['runtime']['typed_plan_sha256']), 'TYPED_PLAN_BINDING'),
        ('producerRevision', 'execution', runtime + ['producer_source_sha'],
         _other_digest(original['result']['runtime']['producer_source_sha']), 'Fresh pinned producer/tool identity'),
        ('inputFileDigest', 'execution', ['inputs_digest'],
         _other_digest(original['inputs_digest']), 'Input-only receipt/file binding'),
        ('rawReplayParent', 'replay', ['replayed_from_sha256'],
         _other_digest(replay['replayed_from_sha256']), 'REPLAY_RAW_PARENT'),
        ('innerPriorResult', 'replay', ['result', 'replay', 'prior_result_sha256'],
         _other_digest(replay['result']['replay']['prior_result_sha256']), 'REPLAY_TYPED_RESULT'),
        ('calleeTarget', 'execution', ['result', 'program', 'pure_calls', 'sites', 0, 'callee'],
         PORT + ':IsPrivate', 'Exact source-declared call-site multiset/byte spans'),
        ('calleeBody', 'execution', generation + ['call_closure', 'activities', 0, 'program_sha256'],
         _other_digest(original['result']['composition']['steps'][0]['generation']['report']['call_closure']['activities'][0]['program_sha256']),
         'Generated callee program/body digest'),
        ('callBudget', 'execution', generation + ['call_closure', 'max_calls_per_invocation'],
         original['result']['composition']['steps'][0]['generation']['report']['call_closure']['max_calls_per_invocation'] + 1,
         'Source-derived static call budget'),
        ('parameterABI', 'execution', generation + ['input_parameters', 0, 'type'],
         'bool', 'GENERATION_ABI'),
        ('generatedSourceHash', 'execution', ['result', 'composition', 'generated_sha256'],
         _other_digest(original['result']['composition']['generated_sha256']), 'Generated/driver raw source digest'),
    ]
    result = []
    for name, target, path, value, diagnostic in specs:
        raw = execution_raw if target == 'execution' else replay_raw
        token = str(body['a']) + '.0' if name == 'integerKind' else json.dumps(value, separators=(',', ':'))
        changed = _raw_value(raw, path, token, span_callback)
        # Load the changed bytes now; mutation validity is separate from its
        # eventual intended checker refusal. All unrelated raw bytes are kept.
        load_receipt(changed)
        result.append({'name': name, 'target': target, 'raw': changed,
                       'expected_diagnostic': diagnostic, 'diagnostic': diagnostic,
                       'path': path, 'base_sha256': hashlib.sha256(raw).hexdigest()})
    need(len(result) == 18 and len({c['name'] for c in result}) == 18, 'TAMPER_ROSTER')
    return result


def check_tampers(checker, execution_raw, replay_raw, entry, indices, inputs, manifest,
                  go_tool_digest, prior_typed, save_callback, span_callback):
    """Require accepted fresh baselines, then exact-gate refusals of18changes."""
    need(entry == REPORT_ENTRY, 'TAMPER_REPORT_REQUIRED')
    original = checker.inspect(execution_raw, entry, indices, inputs, manifest, go_tool_digest)
    repeated = checker.inspect(replay_raw, entry, indices, inputs, manifest, go_tool_digest,
                               original=execution_raw, prior_typed=prior_typed)
    need(exact(original, repeated), 'TAMPER_BASELINES_DIFFER')
    controls = tamper_cases(execution_raw, replay_raw, span_callback)
    for case in controls:
        case['rawref'] = _saved_ref(save_callback, 'tamper-' + case['name'], case['raw'])
    records = []
    for case in controls:
        try:
            checker.inspect(case['raw'], entry, indices, inputs, manifest, go_tool_digest,
                            original=execution_raw if case['target'] == 'replay' else None,
                            prior_typed=prior_typed if case['target'] == 'replay' else None)
        except ReceiptError as exc:
            need(exc.code == case['expected_diagnostic'], 'UNINTENDED_TAMPER_DIAGNOSTIC:' + case['name'])
            records.append({'name': case['name'], 'target': case['target'],
                            'expected_diagnostic': case['expected_diagnostic'],
                            'observed_diagnostic': exc.code, 'rawref': case['rawref'],
                            'base_sha256': case['base_sha256'], 'path': case['path']})
        else:
            raise ReceiptError('TAMPER_ACCEPTED:' + case['name'])
    return records
