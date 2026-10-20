#!/usr/bin/env python3
"""Compare two editable Gooo policies with fresh execution and official replay.

Supplied scalar facts are not authenticated events or current authority.
The official replay command owns inner canonical receipt verification.
"""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import shutil
import signal
import sys
import tempfile
import time

REPO = Path(__file__).resolve().parents[2]
BASE = 'examples/editable-api-policy/'
SPEC = importlib.util.spec_from_file_location('editable_policy_processes', REPO / 'tools/check-header-addition-host.py')
H = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(H)
PACKAGE, ACTIVITY = 'app/editable-api-policy', 'EvaluateAPI'
ENTRY = PACKAGE + ':' + ACTIVITY
AID, RID = 'gooo://editable-api-policy/evaluate/v1', 'gooo://api-example/decision/v1'
FIELDS = ('allowed', 'reason', 'authenticated', 'tenant_present', 'tenant_matched',
          'action_present', 'action_matched', 'access_allowed', 'access_error',
          'access_result', 'group_matched', 'principal_matched', 'group_missing',
          'principal_defaulted', 'effective_principal')
TEXT_FIELDS = {'reason', 'access_result', 'effective_principal'}
INPUT_TYPES = ['Boolean' if i in (0, 2, 4) else 'Text' for i in range(9)]
FACT_FIELDS = ('group_present', 'group_value', 'principal_present', 'principal_value',
               'authenticated', 'caller_tenant', 'resource_tenant', 'requested_action', 'allowed_action')
DEPENDENCIES = [('go/cel.dev/cel-go/codelab', 'ports/cel/scalar_access.gooo'),
                ('app/cel-api-policy', 'examples/cel-api-policy/main.gooo')]
SOURCE_INSPECTION036 = {'compiler_revision': '31497af3beaef210b5a075867b0de76cf55ee3ca',
                        'compiler_version': '0.6.36-dev',
                        'decision_runtime_version': 'v0.2.51-experimental', 'go_version': 'go1.27.2'}


def compiler_profile(lock, name='default'):
    H.need(lock['compiler_revision'] == H.REV and lock['go_version'] == 'go1.27.2', 'repository tool pins')
    if name == 'default':
        return {'name': name, 'compiler_revision': H.REV, 'compiler_version': lock['compiler_version'],
                'go_version': lock['go_version']}
    H.need(name == 'source-inspection036' and
           H.equal(lock.get('consumer_compiler_profiles', {}).get('source_inspection036'), SOURCE_INSPECTION036),
           'explicit source-inspection036 compiler/SDK/Go contract')
    return {'name': name, **SOURCE_INSPECTION036}


def uses_inspection(profile):
    return profile is not None and profile['name'] == 'source-inspection036'


def compiler_identity(identity, profile):
    revision = profile['compiler_revision']
    for key, want in {'compiler_source_sha': revision, 'vcs_revision': revision, 'source_status': 'CLEAN_VCS',
                      'vcs_modified': 'false', 'build_info_observed': True, 'version': profile['compiler_version'],
                      'go_version': profile['go_version'], 'native_go_required': profile['go_version']}.items():
        H.need(H.equal(identity.get(key), want), 'pinned clean compiler identity: ' + key)
    if uses_inspection(profile):
        H.need(identity.get('schema') == 'gooo/build-identity/v1' and identity.get('language') == 'gooo' and
               H.equal(identity.get('decision_runtime'), {'module_path': 'github.com/kimjooyoon/gooo-decision-runtime',
                       'version': profile['decision_runtime_version'], 'replaced': False}),
               'source-inspection036 exact unreplaced decision runtime SDK')


def decoded(raw):
    return H.load(raw.decode('utf-8', 'strict'))


def input_rows(raw):
    H.need(len(raw) <= 30000, 'inputs exceed the 30000-byte joint wire limit')
    doc = decoded(raw)
    H.need(type(doc) is dict and set(doc) == {'schema', 'inputs'} and
           doc['schema'] == 'gooo/body-composition-inputs/v1', 'input-only document schema')
    rows = doc['inputs']; keys = [ENTRY + '.input' + str(i) for i in range(9)]
    H.need(type(rows) is list and 1 <= len(rows) <= 3, 'supply one to three explicit scalar rows')
    for row in rows:
        H.need(type(row) is dict and set(row) == set(keys), 'nine exact supplied facts; missing facts are errors')
        for i, key in enumerate(keys):
            value = row[key]
            H.need(type(value) is (bool if INPUT_TYPES[i] == 'Boolean' else str), 'wrong scalar type: input' + str(i))
            if type(value) is str:
                H.need(len(value.encode('utf-8', 'strict')) <= 1024, 'Text must be valid scalar UTF8, at most1024 bytes')
    return rows


def named_facts_inputs(raw):
    H.need(len(raw) <= 30000, 'facts exceed the 30000-byte source limit')
    facts = decoded(raw)
    H.need(type(facts) is list and 1 <= len(facts) <= 3, 'supply one to three named facts objects')
    rows = []
    for row in facts:
        H.need(type(row) is dict and set(row) == set(FACT_FIELDS), 'nine exact named facts; missing facts are errors')
        for i, name in enumerate(FACT_FIELDS):
            value = row[name]
            H.need(type(value) is (bool if INPUT_TYPES[i] == 'Boolean' else str), 'wrong scalar type: ' + name)
            if type(value) is str:
                H.need(len(value.encode('utf-8', 'strict')) <= 1024, 'Text must be valid scalar UTF8, at most1024 bytes: ' + name)
        rows.append({ENTRY + '.input' + str(i): row[name] for i, name in enumerate(FACT_FIELDS)})
    inputs = H.go_json({'schema': 'gooo/body-composition-inputs/v1', 'inputs': rows}) + b'\n'
    input_rows(inputs)  # Retain the native byte, scalar type and Text-domain checks.
    return inputs


def decision(value):
    H.need(type(value) is dict and set(value) == set(FIELDS), 'exact fifteen-field decision')
    for name in FIELDS:
        H.need(type(value[name]) is (str if name in TEXT_FIELDS else bool), 'decision scalar type: ' + name)
        if name in TEXT_FIELDS: value[name].encode('utf-8', 'strict')
    return value


def producer(runtime, go_version, go_sha, profile=None):
    revision = H.REV if profile is None else profile['compiler_revision']
    H.need(runtime['producer_source_sha'] == revision and runtime['go_tool_selection'] == 'explicit' and
           runtime['go_tool_sha256'] == go_sha and runtime['go_version'] == go_version, 'pinned producer and selected Go')


def output_binding(runtime, decisions):
    # The native driver emits records in declared field order, not map-key order.
    matrix = [[{name: value[name] for name in FIELDS}] for value in decisions]
    text = json.dumps(matrix, ensure_ascii=False, separators=(',', ':'), allow_nan=False)
    for old, new in (('&', '\\u0026'), ('<', '\\u003c'), ('>', '\\u003e'), ('\u2028', '\\u2028'), ('\u2029', '\\u2029')):
        text = text.replace(old, new)
    digest = H.digest(text.encode('utf-8', 'strict') + b'\n')
    H.need(len(runtime['runs']) == 2 and all(r['stdout_sha256'] == digest for r in runtime['runs']),
           'ordered trace values match both reported native stdout hashes')


def compare(before, after):
    decision(before); decision(after)
    changed = [n for n in FIELDS if not H.equal(before[n], after[n])]
    status = 'UNCHANGED'
    if not before['allowed'] and after['allowed']: status = 'NEWLY_ALLOWED'
    elif before['allowed'] and not after['allowed']: status = 'NEWLY_DENIED'
    elif before['reason'] != after['reason']: status = 'REASON_CHANGED'
    elif changed: status = 'OBSERVATIONS_CHANGED'
    return {'status': status, 'changed_fields': changed, 'changed_count': len(changed),
            'before_allowed': before['allowed'], 'after_allowed': after['allowed'],
            'before_reason': before['reason'], 'after_reason': after['reason'],
            'before': before, 'after': after}


def context(root, variant, read, profile=None):
    workspace = root / ('editable-api-policy-' + variant + '.workspace.json')
    raw = read(workspace); manifest = decoded(raw)
    packages = DEPENDENCIES + [(PACKAGE, BASE + variant + '.gooo')]
    H.need(H.equal(manifest, {'schema': 'gooo/package-workspace-manifest/v1',
        'entry': {'package_path': PACKAGE, 'activity': ACTIVITY},
        'packages': [{'path': p, 'sources': [s]} for p, s in packages]}), 'fixed three-source workspace contract')
    definitions = {}; images = []; records = {}
    for package, name in packages:
        text = read(root / name).decode('utf-8', 'strict')
        activities = H.activities(text)
        if not uses_inspection(profile): H.need(len(activities) == 1, 'one activity per selected source')
        for d in activities:
            d.update(package=package, source=name)
            key = package + ':' + d['name']
            H.need(key not in definitions, 'unique source activity name')
            definitions[key] = d
        images.append((package, name, H.digest(H.go_json(text))))
        for record, ident, body in H.RECORD.findall(text):
            fields = [{'name': n, 'id': i, 'type_id': 'urn:gooo:type:' + t} for n, i, t in H.FIELD.findall(body)]
            value = {'name': record, 'id': ident, 'fields': fields}
            H.need(ident not in records or H.equal(records[ident], value), 'consistent repeated record declaration')
            records[ident] = value
    original = {ENTRY, 'app/cel-api-policy:APIAccess', 'go/cel.dev/cel-go/codelab:ScalarAccess'}
    H.need(original <= set(definitions) if uses_inspection(profile) else original == set(definitions), 'three original activity names')
    if uses_inspection(profile):
        H.need(len({d['id'] for d in definitions.values()}) == len(definitions), 'unique authored source activity IDs')
    d = definitions[ENTRY]
    H.need(d['inputs'] == INPUT_TYPES and d['output'] == 'ApiDecision' and d['id'] == AID, 'stable editable entry ABI')
    wanted = [{'name': n, 'id': 'gooo://api-example/decision/' + n.replace('_', '-'),
               'type_id': 'urn:gooo:type:' + ('string' if n in TEXT_FIELDS else 'boolean')} for n in FIELDS]
    H.need(H.equal(records[RID]['fields'], wanted), 'stable ordered fifteen-field ABI')
    return {'workspace': workspace, 'raw': raw, 'packages': packages, 'images': images,
            'definitions': definitions, 'records': records}


def source_program(program, ctx, profile=None):
    H.need(program['schema'] == 'gooo/workspace-body-program/v1' and
           program['entry']['package_path'] == PACKAGE and program['entry']['activity'] == ACTIVITY, 'current entry')
    H.need(H.equal(program['activities'], [program['entry']]), 'one prepared entry activity')
    image = program['workspace_image']
    actual_images = [(p['path'], s['filename'], s['source_digest']) for p in image['packages'] for s in p['sources']]
    H.need(image['schema'] == 'gooo/package-runtime-image/v1' and len(image['packages']) == 3 and
           image['init_order'] == [p for p, _ in ctx['packages']] and actual_images == ctx['images'], 'three current source images')
    H.need(H.equal(image['entry'], {'package_path': PACKAGE, 'source': ctx['packages'][-1][1],
           'activity': ACTIVITY, 'inputs': INPUT_TYPES, 'output': 'ApiDecision'}), 'image entry names and signature')
    pure = program.get('pure_calls')
    if pure is None:
        H.need(uses_inspection(profile), 'retain the original three-activity call closure')
        return
    H.need(pure['schema'] == 'gooo/workspace-pure-calls/v1', 'source pure-call schema')
    participants = pure['activities']
    if not uses_inspection(profile):
        H.need(len(participants) == 3 and len(pure['sites']) == 2, 'retain the original three-activity call closure')
    seen = set(); lowered_names = set()
    for item in participants:
        activity = item['activity']; key = activity['package_path'] + ':' + activity['activity']
        H.need(key in ctx['definitions'] and key not in seen, 'unique current source participant'); seen.add(key)
        d = ctx['definitions'][key]
        H.need(item['source'] == d['source'] and item['source_body_sha256'] == H.digest(d['body'].encode()), 'current source computes body')
        if uses_inspection(profile):
            H.need(item['activity_id'] == d['id'] == item['lowered_activity_id'] and
                   type(activity['lowered_name']) is str and activity['lowered_name'] and
                   activity['lowered_name'] not in lowered_names, 'authored current source participant ID and unique lowered name')
            lowered_names.add(activity['lowered_name'])
            if key == ENTRY: H.need(H.equal(activity, program['entry']), 'prepared source entry reference')
    if not uses_inspection(profile): H.need(seen == set(ctx['definitions']), 'complete source participants')
    else:
        H.need(ENTRY in seen, 'prepared call closure contains current entry')
        for site in pure['sites']:
            H.need(site['caller'] in seen and site['callee'] in seen and site['surface'] == 'computes' and
                   type(site['start_byte']) is int and type(site['end_byte']) is int and
                   0 <= site['start_byte'] < site['end_byte'] <= len(ctx['definitions'][site['caller']]['body'].encode()),
                   'prepared call site binds current participant body bytes')


def inspect_source(raw, ctx, profile):
    H.need(uses_inspection(profile), 'source inspection requires its explicit compiler profile')
    receipt = decoded(raw)
    H.need(set(receipt) == {'schema', 'decision', 'manifest', 'manifest_digest', 'program', 'program_executions',
                           'model_predictions', 'transport_capacity', 'scope'} and
           receipt['schema'] == 'gooo/workspace-source-inspection/v1' and receipt['decision'] == 'PREPARED' and
           H.equal([receipt['program_executions'], receipt['model_predictions']], [0, 0]),
           'fresh source-only PREPARED inspection, no execution or model prediction')
    H.need(receipt['manifest'] == str(ctx['workspace']) and receipt['manifest_digest'] == H.digest(ctx['raw']),
           'prepared current manifest byte binding')
    source_program(receipt['program'], ctx, profile)
    return receipt['program']


def prepared_binding(program, ctx, profile=None):
    if uses_inspection(profile):
        # This is a fresh observation by the selected compiler, not an independent parser or semantic oracle.
        H.need('prepared_program' in ctx and H.equal(program, ctx['prepared_program']),
               'native program equals its fresh same-compiler source inspection in full')


def inspect_receipt(raw, rows, inputs, ctx, go_version, go_sha, parent=None, profile=None):
    receipt = decoded(raw)
    H.need(receipt['schema'] == 'gooo/workspace-body-execution-receipt/v1' and
           receipt['decision'] == 'OBSERVED' and not receipt.get('error'), 'input-only OBSERVED receipt')
    H.need(receipt['manifest'] == str(ctx['workspace']) and receipt['manifest_digest'] == H.digest(ctx['raw']) and
           receipt['inputs_digest'] == H.digest(inputs) and 'cases_digest' not in receipt, 'current manifest/input byte binding')
    result = receipt['result']; program = result['program']; composition = result['composition']; rt = result['runtime']
    H.need(result['schema'] == 'gooo/workspace-body-execution/v1' and
           composition['schema'] == 'gooo/body-composition/v1' and composition['stage'] == 'COMPLETE', 'complete native composition')
    prepared_binding(program, ctx, profile)
    source_program(program, ctx, profile)
    lowered = H.digest(program['lowered_gooo_source'].encode())
    H.need(program['lowered_gooo_source'] == composition['gooo_source'] and lowered == result['execution_source_sha256'] ==
           composition['original_source_sha256'] == composition['selected_source_sha256'] == rt['original_source_sha256'] == rt['selected_source_sha256'], 'raw lowered source bindings')
    H.need(H.digest(composition['source'].encode()) == composition['generated_sha256'] == rt['generated_sha256'] and
           H.digest(composition['driver_source'].encode()) == composition['driver_sha256'] == rt['driver_sha256'], 'generated Go/driver raw hashes')
    H.need(len(composition['steps']) == 1, 'one generated entry')
    report = composition['steps'][0]['generation']['report']
    revision = H.REV if profile is None else profile['compiler_revision']
    H.need(report['schema'] == 'gooo/body-codegen-report/v3' and report['compiler_source_sha'] == revision and
           report['activity_id'] == AID and report['output_type'] == 'ApiDecision' and
           report['typecheck_passed'] is True and report['deterministic_replay'] is True, 'code generation producer and ABI')
    H.need(composition['steps'][0]['input_source_sha256'] == lowered and
           H.digest(composition['steps'][0]['generation']['source'].encode()) == report['generated_digest'] == report['replay_digest'],
           'raw generated entry source binding')
    H.need(H.equal(report['input_parameters'], [{'name': 'input' + str(i), 'type': 'bool' if t == 'Boolean' else 'string'}
                                               for i, t in enumerate(INPUT_TYPES)]), 'nine typed generation parameters')
    plan = composition['plan']; H.need(len(plan['activities']) == 1 and plan['entry_activity'] == program['entry']['lowered_name'], 'one current planned entry')
    activity = plan['activities'][0]
    H.need(activity['id'] == AID and activity['output_entity_id'] == RID and
           H.equal(activity['inputs'], [{'port': 'input' + str(i), 'type': t, 'entity_id': 'gooo://std/' + ('boolean' if t == 'Boolean' else 'text'), 'from': -1}
                                       for i, t in enumerate(INPUT_TYPES)]), 'planned scalar ports and result ID')
    for layouts in [report['record_types'], plan['record_types']]:
        record = [r for r in layouts if r['id'] == RID]
        H.need(len(record) == 1 and H.equal([{k: f[k] for k in ('name', 'id', 'type_id')} for f in record[0]['fields']],
                                         ctx['records'][RID]['fields']), 'ordered output field IDs/types')
    H.need(rt['schema'] == 'gooo/body-composition-runtime/v1' and rt['stage'] == 'COMPLETE' and
           H.equal([rt['finite_passed'], rt['finite_total'], rt['model_calls']], [0, 0, 0]) and
           rt['projection_replayed'] is True and rt['runtime_replayed'] is True, 'input-only runtime, no embedded expectations')
    producer(rt, go_version, go_sha, profile)
    H.need(len(rt['runs']) == 2, 'two fresh native runs per command')
    for p in [rt['toolchain'], rt['build']] + rt['runs']:
        H.need(p['started'] and p['completed'] and p['exit_code'] == 0 and
               not any(p[k] for k in ('canceled', 'timed_out', 'output_truncated')), 'completed native process observations')
    H.need(len(rt['traces']) == len(rows), 'one trace per supplied row')
    decisions = []
    for i, trace in enumerate(rt['traces']):
        H.need(H.equal(trace['case_index'], i) and len(trace['deliveries']) == 1, 'original row order')
        delivery = trace['deliveries'][0]
        echoes = [{'port': 'input' + str(j), 'entity_id': 'gooo://std/' + ('boolean' if t == 'Boolean' else 'text'),
                   'value': rows[i][ENTRY + '.input' + str(j)]} for j, t in enumerate(INPUT_TYPES)]
        H.need(delivery['activity_id'] == AID and H.equal(delivery['inputs'], echoes) and
               'expected' not in delivery and 'passed' not in delivery, 'typed exact input echoes, observations only')
        value = decision(delivery['actual'])
        H.need(H.equal(delivery['actual_fields'], [{'id': f['id'], 'name': f['name'], 'value': value[f['name']]}
                                                  for f in ctx['records'][RID]['fields']]), 'ordered typed actual fields')
        decisions.append(value)
    output_binding(rt, decisions)
    if parent is None:
        H.need('replayed_from_sha256' not in receipt and 'replay' not in result, 'fresh initial observation')
    else:
        H.need(receipt['replayed_from_sha256'] == H.digest(parent) and result['replay']['schema'] == 'gooo/workspace-body-replay/v1' and
               H.equal(result['replay']['model_calls'], 0), 'official replay with its exact original raw parent')
    return decisions


def check_refusal(raw, parent, ctx, profile=None):
    r = decoded(raw)
    keys = {'schema', 'decision', 'manifest', 'manifest_digest', 'error', 'replayed_from_sha256'}
    if uses_inspection(profile): keys.add('failure')
    H.need(set(r) == keys and
           r['schema'] == 'gooo/workspace-body-execution-receipt/v1' and r['decision'] == 'FAIL_CLOSED' and
           r['error'] == 'saved execution envelope or workspace manifest differs' and
           r['replayed_from_sha256'] == H.digest(parent) and r['manifest'] == str(ctx['workspace']) and
           r['manifest_digest'] == H.digest(ctx['raw']), 'exact current manifest-stage cross-workspace refusal')
    if uses_inspection(profile):
        H.need(H.equal(r['failure'], {'code': 'REPLAY_MANIFEST_MISMATCH',
                                     'detail': 'saved execution envelope or workspace manifest differs'}),
               'exact source-inspection036 manifest-stage failure code and detail')


def check_demo(inputs, comparisons, root, read):
    H.need(H.equal(decoded(inputs), decoded(read(root / (BASE + 'demo-inputs.json')))), '--check-demo requires the supplied demo facts')
    expected = decoded(read(root / (BASE + 'demo-expected.json')))
    H.need(expected['schema'] == 'gooo/editable-api-policy-demo-expectations/v1' and len(expected['rows']) == len(comparisons), 'demo literal rows')
    for row, want in zip(comparisons, expected['rows']):
        for key in ('before', 'after', 'status', 'changed_fields'): H.need(H.equal(row[key], want[key]), 'demo literal mismatch: ' + key)


def arguments(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ('root', 'compiler', 'go', 'out'): parser.add_argument('--' + name, required=True)
    source = parser.add_mutually_exclusive_group()
    source.add_argument('--inputs', help='one to three complete native scalar rows; defaults to demo-inputs.json')
    source.add_argument('--facts', help='JSON array of one to three exact nine-field named API facts objects')
    parser.add_argument('--compiler-profile', choices=('default', 'source-inspection036'), default='default',
                        help='explicit separate compiler contract; default retains the repository 839 pin')
    parser.add_argument('--check-demo', action='store_true', help='also check the original three-row literals; omit after editing policy')
    args = parser.parse_args(argv)
    if args.facts == '': parser.error('--facts requires a nonempty path')
    return args


def main():
    args = arguments()
    out = H.canonical(Path(args.out)); H.need(out.parent.is_dir() and not out.exists(), 'fresh output with an existing parent required')
    out.mkdir(); started = time.monotonic(); pins = {}; work = runner = owner = None
    failure = None; comparisons = []; source_ok = cleaned = refused = False; positives = prepared = 0
    handlers = {n: signal.signal(n, lambda number, frame: (_ for _ in ()).throw(RuntimeError('signal ' + str(number)))) for n in (signal.SIGINT, signal.SIGTERM)}
    try:
        root, compiler, go = [H.canonical(Path(x)) for x in (args.root, args.compiler, args.go)]
        H.need(root == REPO and root.is_dir(), 'run from this source checkout')
        read = lambda path: H.frozen_read(path, pins)
        source_path = args.facts or args.inputs
        path = H.canonical(Path(source_path)) if source_path else root / (BASE + 'demo-inputs.json')
        H.need(path.stat().st_size <= 30000, 'input byte limit before reading'); inputs = read(path)
        if args.facts:
            inputs = named_facts_inputs(inputs)
            path = out / 'facts-inputs.json'
            with path.open('xb') as handle: handle.write(inputs)
            H.need(read(path) == inputs, 'saved named-facts native input bytes')
        rows = input_rows(inputs)
        lock = decoded(read(root / 'upstream.json')); profile = compiler_profile(lock, args.compiler_profile)
        contexts = {v: context(root, v, read, profile) for v in ('before', 'after')}
        for ctx in contexts.values():
            H.need(sum(pins[str(root / n)]['bytes'] for _, n in ctx['packages']) <= 10 << 20, 'bounded source closure')
        for name in ('run.py', 'run_test.py', 'README.md', 'NOTICE.md'): read(root / (BASE + name))
        if args.check_demo:
            for name in ('demo-inputs.json', 'demo-expected.json'): read(root / (BASE + name))
        for name in ('check-header-addition-host.py', 'native_verification_contract.py'): read(root / 'tools' / name)
        read(compiler); go_sha = H.digest(read(go))
        H.need(shutil.disk_usage(out).free >= 2 << 30, 'minimum2GiB free')
        work = Path(tempfile.mkdtemp(prefix='.editable-policy-owned-', dir=out)); owner = (work.stat().st_dev, work.stat().st_ino)
        for name in ('cache', 'modules', 'tmp', 'gopath'): (work / name).mkdir()
        env = {k: os.environ[k] for k in ('PATH', 'HOME', 'SYSTEMROOT') if k in os.environ}
        env.update(GOTOOLCHAIN='local', GOWORK='off', GOENV='off', GOPROXY='off', GOSUMDB='off', GOFLAGS='-mod=readonly',
                   GOCACHE=str(work / 'cache'), GOMODCACHE=str(work / 'modules'), GOPATH=str(work / 'gopath'), TMPDIR=str(work / 'tmp'),
                   CGO_ENABLED='0', GOMAXPROCS='2', GOOO_LAYA_URL='', GOOO_LAYA_API_KEY='')
        runner = H.OwnedCommands(out, work, root, pins, env); runner.started = started
        def call(label, argv, allowed_exit=(0,)):
            H.need(shutil.disk_usage(out).free >= 2 << 30 and H.owned_bytes(out) - H.owned_bytes(work) <= 64 << 20, 'free/evidence budget')
            raw = runner.call(label, argv, 90, allowed_exit=allowed_exit)
            H.need(H.owned_bytes(out) - H.owned_bytes(work) <= 64 << 20, 'retained64MiB bound'); return raw
        identity = decoded(call('compiler-identity', [compiler, 'version', '--build', '--json']))
        compiler_identity(identity, profile)
        go_version = call('go-version', [go, 'version']).decode('utf-8').strip()
        H.need(go_version.startswith('go version go1.27.2 '), 'Go1.27.2 required')
        observed = {}; initial = {}
        for variant in ('before', 'after'):
            ctx = contexts[variant]; parent = None
            if uses_inspection(profile):
                raw = call(variant + '-inspect', [compiler, 'package', 'inspect', '--json', ctx['workspace']])
                ctx['prepared_program'] = inspect_source(raw, ctx, profile); prepared += 1
            for operation in ('execute', 'replay'):
                label = variant + '-' + operation
                argv = [compiler, 'package', operation, '--json', '--inputs', path, '--go', go]
                if parent is not None: argv += ['--receipt', out / (variant + '-execute') / 'stdout']
                raw = call(label, [*argv, ctx['workspace']])
                values = inspect_receipt(raw, rows, inputs, ctx, go_version, go_sha, parent, profile); positives += 1
                if parent is None: observed[variant] = values; initial[variant] = raw
                else: H.need(H.equal(values, observed[variant]), 'fresh official replay values equal its initial observation')
                parent = raw
        argv = [compiler, 'package', 'replay', '--json', '--receipt', out / 'before-execute/stdout',
                '--inputs', path, '--go', go, contexts['after']['workspace']]
        raw = call('cross-workspace-refusal', argv, allowed_exit=(1,))
        check_refusal(raw, initial['before'], contexts['after'], profile); refused = True
        comparisons = [dict(index=i, **compare(a, b)) for i, (a, b) in enumerate(zip(observed['before'], observed['after']))]
        if args.check_demo: check_demo(inputs, comparisons, root, read)
        H.unchanged(pins); source_ok = True
    except BaseException as error: failure = str(error)
    finally:
        try:
            H.unchanged(pins); source_ok = runner is not None
        except BaseException as error: failure = failure or str(error); source_ok = False
        if work is not None and (runner is None or runner.closed):
            try:
                H.need(not work.is_symlink() and (work.stat().st_dev, work.stat().st_ino) == owner, 'owned inode before cleanup')
                shutil.rmtree(work); cleaned = True
            except BaseException as error: failure = failure or 'owned cleanup: ' + str(error)
        for number, handler in handlers.items(): signal.signal(number, handler)
    summary = {'schema': 'gooo/editable-api-policy-comparison/v1',
        'status': 'EDITABLE_POLICY_COMPARISON_AND_OFFICIAL_REPLAY_PASS' if failure is None else 'FIRST_FAILURE_PRESERVED',
        'failure': failure, 'rows': len(comparisons), 'positive_receipts': positives,
        'compiler_profile': args.compiler_profile, 'prepared_inspections': prepared,
        'package_commands': sum(x['label'] not in ('compiler-identity', 'go-version') for x in runner.observations) if runner else 0,
        'input_echoes_checked': positives * (len(rows) if 'rows' in locals() else 0) * 9,
        'typed_output_fields_checked': positives * (len(rows) if 'rows' in locals() else 0) * 15,
        'compiler_revision': profile['compiler_revision'] if positives == 4 else None,
        'finite_total': 0 if positives == 4 else None, 'model_calls': 0 if positives == 4 else None,
        'demo_checked': args.check_demo and failure is None,
        'cross_workspace_refused': refused, 'cross_workspace_refusal_stage': 'manifest' if refused else None,
        'comparisons': comparisons, 'source_tool_unchanged': source_ok,
        'processes_closed': runner.closed if runner else True, 'owned_work_removed': cleaned,
        'commands': runner.observations if runner else [], 'source_tool_pins': pins,
        'scope': 'One to three supplied synthetic scalar rows, input-only OBSERVED/finite0. Each successful execute/replay runs two fresh native executions. Official replay owns canonical verification; raw outer parents/current sources/typed observations and reported native stdout hashes are checked here. Cross-workspace refusal exercises the manifest stage, not a same-manifest changed-source guard. No live authorization, trace truth, missing-state synthesis, stateful replay, policy Go fallback or external adoption.'}
    try: H.save(out / 'summary.json', summary)
    except BaseException as error:
        print('summary write failed: ' + str(error) + '; first_failure=' + repr(failure), file=sys.stderr); return 1
    print(H.go_json({'status': summary['status'], 'summary': str(out / 'summary.json'), 'failure': failure}).decode())
    if failure is None:
        for row in comparisons:
            brief = lambda text: H.go_json(text if len(text) <= 120 else text[:117] + '…').decode()
            print(str(row['index']) + ' ' + row['status'] + ': ' + brief(row['before_reason']) + ' -> ' +
                  brief(row['after_reason']) + '; changed=' + (','.join(row['changed_fields']) or '-'))
    return 0 if failure is None else 1


if __name__ == '__main__': sys.exit(main())
