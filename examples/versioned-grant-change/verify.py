#!/usr/bin/env python3
"""Run the finite Gooo versioned-grant example with fresh component references.

The independent literal corpus owns final-policy expectations. Go only writes
case documents and observes upstream components. Official replay owns inner
canonical validation; this reader checks current sources and typed boundaries.
"""
import argparse
import copy
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import signal
import stat
import sys
import tempfile
import time

REPO = Path(__file__).resolve().parents[2]
HELPER = REPO / 'tools/check-header-addition-host.py'
SPEC = importlib.util.spec_from_file_location('versioned_grant_public_helpers', HELPER)
H = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(H)
PACKAGE = 'gooo/service-policy/versioned-change'
ACTIVITY = 'AssessVersionedGrantChange'
ENTRY = PACKAGE + ':' + ACTIVITY
AID = 'gooo://service-policy/versioned-change/assess/v1'
RID = 'gooo://service-policy/versioned-change/decision/v1'
BASE = 'examples/versioned-grant-change/'
MANIFEST = 'versioned-grant-change.workspace.json'
FIELDS = [('status', 'string'), ('reason', 'string'),
          ('expected_validator_strict', 'boolean'), ('current_validator_strict', 'boolean'),
          ('if_match_condition', 'integer'), ('version_match', 'boolean'),
          ('grant_status', 'string'), ('grant_reason', 'string'), ('invalid_field', 'string'),
          ('required_before_match', 'boolean'), ('required_after_match', 'boolean'),
          ('subset_rule_id', 'string')]
KINDS = {'string': str, 'boolean': bool, 'integer': int}
CORPUS_SHA = 'cd8faa4f6493d1ca63ca3124473ec369f4979d5836ac1ae9c695aa1f34fafc09'


def decoded(raw):
    return H.load(raw.decode('utf-8', 'strict'))


def result_matrix(rows):
    # Native records serialize in their declared field order, unlike Go maps.
    value = [[{n: row['expected'][ENTRY][n] for n, _ in FIELDS}] for row in rows]
    text = json.dumps(value, ensure_ascii=False, separators=(',', ':'), allow_nan=False)
    for old, new in (('&', '\\u0026'), ('<', '\\u003c'), ('>', '\\u003e'), ('\u2028', '\\u2028'), ('\u2029', '\\u2029')):
        text = text.replace(old, new)
    return text.encode('utf-8', 'strict') + b'\n'


def case_values(row):
    H.need(type(row) is dict and set(row) == {'inputs', 'expected'}, 'case row shape')
    keys = [ENTRY + '.input' + str(i) for i in range(6)]
    H.need(type(row['inputs']) is dict and set(row['inputs']) == set(keys), 'six exact input bindings')
    values = [row['inputs'][k] for k in keys]
    for v in values:
        H.need(type(v) is str and len(v.encode('utf-8', 'strict')) <= 1024, 'scalar UTF8 Text0..1024')
    H.need(type(row['expected']) is dict and set(row['expected']) == {ENTRY}, 'one expected root')
    expected = row['expected'][ENTRY]
    H.need(type(expected) is dict and set(expected) == {n for n, _ in FIELDS}, 'twelve expected fields')
    for n, k in FIELDS:
        H.need(type(expected[n]) is KINDS[k], 'expected scalar type: ' + n)
        if k == 'integer':
            H.need(-(1 << 63) <= expected[n] < (1 << 63), 'expected int64')
    return values, expected


def wire_upper(document, source_bytes):
    H.need(type(source_bytes) is int and 0 < source_bytes <= 10 << 20, 'source-byte lowering bound')
    rows = document['cases']
    H.need(type(rows) is list and 0 < len(rows) <= 128, 'bounded finite batch rows')
    lowered_key = len('GoooPackage') + len(str(source_bytes)) + len('Activity') + len(ACTIVITY)
    return len(H.go_json(document)) + 7 * len(rows) * max(0, lowered_key - len(ENTRY))


def check_contract(contract, manifest, sources):
    H.need(contract['schema'] == 'gooo/versioned-grant-change-contract/v1' and H.equal(contract['entry'], {
        'package_path': PACKAGE, 'activity': ACTIVITY, 'activity_id': AID, 'result_entity_id': RID}), 'public entry contract')
    H.need(contract['workspace'] == MANIFEST and H.equal(contract['sources'], [
        {'package': pkg['path'], 'path': pkg['sources'][0], 'sha256': H.sha(sources[pkg['sources'][0]])}
        for pkg in manifest['packages']]), 'contract/current sources')
    names = ('selected_if_match', 'observed_policy_etag', 'before_grant', 'candidate_grant', 'required_type', 'required_name')
    H.need(H.equal(contract['input_order'], [
        {'port': 'input' + str(i), 'name': n, 'binding': ENTRY + '.input' + str(i), 'type': 'Text',
         'entity_id': 'gooo://std/text', 'required': True, 'minimum_utf8_bytes': 0, 'maximum_utf8_bytes': 1024}
        for i, n in enumerate(names)]), 'six public input contracts')
    H.need(H.equal(contract['output_fields'], [
        {'name': n, 'id': RID + '/' + n, 'type': k, 'required': True, 'cardinality': 'one'} for n, k in FIELDS]), 'ordered public output contract')
    record_contracts = []
    for pkg in manifest['packages']:
        source = pkg['sources'][0]
        for name, ident, body in H.RECORD.findall(sources[source].decode()):
            record_contracts.append({'name': name, 'id': ident, 'source': source, 'fields': [
                {'name': n, 'id': i, 'type': k, 'required': True, 'cardinality': 'one'} for n, i, k in H.FIELD.findall(body)]})
    H.need(H.equal(contract['records'], record_contracts), 'all nine documented source record layouts')
    H.need(contract['corpus']['sha256'] == CORPUS_SHA and contract['corpus']['occurrences'] == 48 and
           contract['pins']['compiler_revision'] == H.REV and contract['pins']['go_version'] == 'go1.27.2', 'public corpus/tool contract')
    bounds = contract['wire_bounds']
    H.need(H.equal(bounds['each_scalar_utf8_bytes'], [0, 1024]) and bounds['raw_go_case_document_max_bytes'] == 30000 and
           bounds['translated_compact_upper_max_bytes'] == 32768 and bounds['absolute_max_rows_per_batch'] == 128 and
           bounds['initial_max_rows_per_batch'] == 8, 'public joint wire bounds')


def agentmesh_sources(root, read):
    """Retain the existing public oracle's exact upstream extraction checks."""
    base = root / 'examples/agentmesh-service-policy'
    pin = decoded(read(base / 'source-pin.json'))
    H.need(pin['revision'] == '9fd1a41a15174df0aaef601c942410c2116bff91', 'AgentMesh revision')
    for f in pin['archive_files'] + pin['canonical_components']:
        raw = read(base / H.safe_relative(f['filename']))
        H.need(len(raw) == f['bytes'] and H.sha(raw) == f['sha256'], 'immutable AgentMesh source: ' + f['filename'])
    H.need(read(base / 'LICENSE-APACHE-2.0') == read(base / 'upstream/LICENSE'), 'AgentMesh license')
    extraction = decoded(read(base / 'oracle/extraction.json'))
    H.need(H.equal(pin['exact_function_bytes'], extraction['exact_function_bytes']), 'upstream extraction metadata')
    extracted = read(base / 'oracle/extracted.go')
    def function(raw, name):
        start = raw.index(('func ' + name + '(').encode())
        return raw[start:raw.index(b'\n}\n', start) + 3]
    for name, file in [('MatchServicePattern', 'tar.go.txt'), ('ParseServiceTarget', 'network.go.txt')]:
        actual = function(extracted, name)
        H.need(actual == function(read(base / 'upstream/api' / file), name) and H.sha(actual) == pin['exact_function_bytes'][name], 'unchanged upstream function: ' + name)
    regex = pin['exact_regexp_assignment']
    H.need(('var ' + regex) in extracted.decode() and regex in read(base / 'upstream/api/network.go.txt').decode(), 'upstream URI regexp assignment')


def definitions(manifest, sources):
    defs, records, imports = {}, {}, {}
    for pkg in manifest['packages']:
        H.need(len(pkg['sources']) == 1, 'one source per selected package')
        name = pkg['sources'][0]
        text = sources[name].decode('utf-8', 'strict')
        aliases = dict(re.findall(r'^import\s+(\w+)\s+"([^"]+)"', text, re.M))
        imports[pkg['path']] = list(aliases.values())
        for d in H.activities(text):
            d.update(package=pkg['path'], source=name, aliases=aliases)
            key = pkg['path'] + ':' + d['name']
            H.need(key not in defs, 'unique activity')
            defs[key] = d
        for name, ident, body in H.RECORD.findall(text):
            fields = [{'name': n, 'id': i, 'type_id': 'urn:gooo:type:' + k,
                       'go_name': 'GoooField' + H.sha(i.encode())} for n, i, k in H.FIELD.findall(body)]
            H.need(ident not in records and 0 < len(fields) <= 16, 'unique bounded record')
            records[ident] = {'name': name, 'id': ident, 'go_name': 'GoooRecord' + H.sha(ident.encode()), 'fields': fields}
    H.need(len(imports) == 12 and len(records) == 9 and sum(len(r['fields']) for r in records.values()) == 61, 'source12/record9/field61 closure')
    for d in defs.values():
        body = d['body']
        sites = []
        for m in re.finditer(r'\b([a-z]\w*)\.([A-Z]\w*)\s*(?=\()', body):
            H.need(m[1] in d['aliases'], 'declared call alias')
            callee = d['aliases'][m[1]] + ':' + m[2]
            H.need(callee in defs, 'selected pure callee')
            sites.append({'caller': d['package'] + ':' + d['name'], 'callee': callee, 'surface': 'computes',
                          'start_byte': len(body[:m.start()].encode()), 'end_byte': len(body[:m.end()].encode())})
        for m in re.finditer(r'(?<![\w.])([A-Z]\w*)\s*(?=\()', body):
            callee = d['package'] + ':' + m[1]
            if callee in defs:
                sites.append({'caller': d['package'] + ':' + d['name'], 'callee': callee, 'surface': 'computes',
                              'start_byte': len(body[:m.start()].encode()), 'end_byte': len(body[:m.end()].encode())})
        d['sites'] = sorted(sites, key=lambda x: x['start_byte'])
    H.need(len(defs[ENTRY]['sites']) == 4, 'four direct Gooo component calls')
    reachable = [ENTRY]
    for key in reachable:
        for site in defs[key]['sites']:
            if site['callee'] not in reachable: reachable.append(site['callee'])
    return {k: defs[k] for k in reachable}, records, imports


def receipt(raw, rows, case_raw, manifest_raw, workspace, sources, images, defs, records, imports,
            go_version, go_sha, parent=None):
    e = decoded(raw)
    H.need(e['schema'] == 'gooo/workspace-body-execution-receipt/v1' and e['decision'] == 'PASS' and not e.get('error'), 'producer envelope')
    H.need(e['manifest'] == str(workspace) and e['manifest_digest'] == H.digest(manifest_raw) and
           e['cases_digest'] == H.digest(case_raw) and 'inputs_digest' not in e, 'raw current manifest/case binding')
    r = e['result']; p = r['program']; c = r['composition']; rt = r['runtime']
    # Bind the producer's reported lowering names to authored source bodies.
    # Do not reimplement the engine's ordinal or canonical-ID algorithms.
    defs = copy.deepcopy(defs)
    pure = p['pure_calls']; participants = pure['activities']
    H.need(pure['schema'] == 'gooo/workspace-pure-calls/v1' and len(participants) == len(defs), 'pure source participants')
    seen_participants = set(); lowered_names = set()
    for x in participants:
        a = x['activity']; key = a['package_path'] + ':' + a['activity']
        H.need(key in defs and key not in seen_participants, 'unique authored pure participant')
        d = defs[key]; lowered = a['lowered_name']
        H.need(type(lowered) is str and re.fullmatch(r'GoooPackage\d+Activity' + re.escape(d['name']), lowered) is not None and
               lowered not in lowered_names and x['source'] == d['source'] and
               x['source_body_sha256'] == H.digest(d['body'].encode()), 'lowering name/current authored body binding')
        seen_participants.add(key); lowered_names.add(lowered); d['lowered'] = lowered
    for d in defs.values():
        lowered_body = d['body'].encode()
        for s in reversed(d['sites']):
            lowered_body = lowered_body[:s['start_byte']] + defs[s['callee']]['lowered'].encode() + lowered_body[s['end_byte']:]
        d['lowered_body'] = lowered_body.decode()
    root = defs[ENTRY]
    selected = {'package_path': PACKAGE, 'activity': ACTIVITY, 'lowered_name': root['lowered']}
    H.need(r['schema'] == 'gooo/workspace-body-execution/v1' and p['schema'] == 'gooo/workspace-body-program/v1' and
           H.equal(p['entry'], selected) and H.equal(p['activities'], [selected]), 'single selected root')
    image = p['workspace_image']; packages = image['packages']; order = image['init_order']
    H.need(image['schema'] == 'gooo/package-runtime-image/v1' and len(packages) == len(set(order)) == len(order) == 12 and
           set(order) == set(imports), 'exact package/image roster')
    seen = [{'package': pkg['path'], 'filename': s['filename'], 'source_digest': s['source_digest']}
            for pkg in packages for s in pkg['sources']]
    image_order = lambda row: (row['package'], row['filename'])
    H.need(len(seen) == len(images) == 12 and H.equal(sorted(seen, key=image_order), sorted(images, key=image_order)), 'current Go-written JSON-string source digests')
    for pkg in packages:
        dependencies = pkg.get('imports') or []
        H.need(len(dependencies) == len(set(dependencies)) and set(dependencies) == set(imports[pkg['path']]) and
               all(order.index(dep) < order.index(pkg['path']) for dep in dependencies), 'source imports/init order')
    H.need(H.equal(image['entry'], {'package_path': PACKAGE, 'source': BASE + 'main.gooo', 'activity': ACTIVITY,
                                 'inputs': ['Text'] * 6, 'output': 'VersionedGrantChangeDecision'}), 'entry image ABI')
    exports = [x for pkg in packages if pkg['path'] == PACKAGE for x in pkg['exports']
               if x['kind'] == 'activity' and x['name'] == ACTIVITY]
    H.need(len(exports) == 1 and H.equal(exports[0], {'name': ACTIVITY, 'kind': 'activity', 'id': AID,
           'input_types': ['gooo://std/text'] * 6, 'output_type': RID}), 'exported root ABI')
    lowered = p['lowered_gooo_source']; actual = H.activities(lowered)
    H.need(len(actual) == len(defs), 'source activity closure')
    for d in defs.values():
        aa = [a for a in actual if a['name'] == d['lowered']]
        H.need(len(aa) == 1 and all(H.equal(aa[0][k], d[k]) for k in ('id', 'inputs', 'output')) and
               aa[0]['body'] == d['lowered_body'], 'current lowered activity body/signature')
    queue, sites = [ENTRY], []
    for key in queue:
        for s in defs[key]['sites']:
            sites.append(s)
            if s['callee'] not in queue: queue.append(s['callee'])
    H.need(H.equal(pure['sites'], sites), 'actual source-resolved pure call sites')
    source_sha = H.digest(lowered.encode())
    H.need(c['schema'] == 'gooo/body-composition/v1' and c['stage'] == 'COMPLETE' and c['gooo_source'] == lowered and
           c['original_source_sha256'] == c['selected_source_sha256'] == r['execution_source_sha256'] == source_sha, 'composition current source')
    plan = c['plan']; roots = plan['activities']
    H.need(plan['schema'] == 'gooo/body-composition-plan/v1' and plan['entry_activity'] == root['lowered'] and
           H.equal(plan['edges'], []) and len(roots) == 1 and not plan.get('preparations'), 'one no-effect root plan')
    a = roots[0]
    H.need(a['id'] == AID and a['name'] == root['lowered'] and a['output_entity_id'] == RID and
           a['output_type'] == 'VersionedGrantChangeDecision' and a['assembling'] is False and not a.get('prepared') and
           H.equal(a['inputs'], [{'port': 'input' + str(i), 'type': 'Text', 'entity_id': 'gooo://std/text', 'from': -1} for i in range(6)]), 'six input/record plan ABI')
    H.need(len(plan['record_types']) == 9 and H.equal({x['id']: x for x in plan['record_types']}, records), 'nine full record layouts')
    H.need(len(c['steps']) == 1, 'one root generation')
    generation = c['steps'][0]['generation']; report = generation['report']
    H.need(report['schema'] == 'gooo/body-codegen-report/v3' and report['decision'] == 'PASS' and
           report['activity_id'] == AID and report['compiler_source_sha'] == H.REV and report['typecheck_passed'] is True and
           report['deterministic_replay'] is True and report['source_digest'] == source_sha and
           report['program_digest'] == H.digest(root['lowered_body'].encode()) and
           report['generated_digest'] == report['replay_digest'] == H.digest(generation['source'].encode()) and
           H.equal(report['repository_writes'], 0), 'source-bound native generation')
    H.need(H.equal(report['input_parameters'], [{'name': 'input' + str(i), 'type': 'string'} for i in range(6)]) and
           report['output_type'] == 'VersionedGrantChangeDecision' and len(report['record_types']) == 9 and
           H.equal({x['id']: x for x in report['record_types']}, records), 'generated record/parameter ABI')
    H.need(c['generated_sha256'] == rt['generated_sha256'] == H.digest(c['source'].encode()) and
           c['driver_sha256'] == rt['driver_sha256'] == H.digest(c['driver_source'].encode()), 'generated and driver byte links')
    H.need(rt['schema'] == 'gooo/body-composition-runtime/v1' and rt['stage'] == 'COMPLETE' and not rt.get('failure') and
           H.equal([rt['finite_total'], rt['finite_passed'], rt['model_calls']], [len(rows), len(rows), 0]) and
           rt['producer_source_sha'] == H.REV and rt['go_tool_selection'] == 'explicit' and rt['go_tool_sha256'] == go_sha and
           rt['go_version'] == go_version and rt['projection_replayed'] is True and rt['runtime_replayed'] is True and
           rt['original_source_sha256'] == rt['selected_source_sha256'] == source_sha and
           rt['typed_plan_sha256'] == plan['typed_plan_sha256'], 'finite runtime and exact tool/source crosslinks')
    H.need(len(rt['traces']) == len(rows), 'all ordered finite occurrences')
    for i, (trace, row) in enumerate(zip(rt['traces'], rows)):
        values, expected = case_values(row)
        H.need(type(trace['case_index']) is int and trace['case_index'] == i and len(trace['deliveries']) == 1, 'ordered trace')
        d = trace['deliveries'][0]
        H.need(d['activity_id'] == AID and H.equal(d['inputs'], [{'port': 'input' + str(j), 'entity_id': 'gooo://std/text', 'value': v} for j, v in enumerate(values)]), 'all six unchanged Text echoes')
        H.need(d['passed'] is True and H.equal(d['actual'], expected) and H.equal(d['expected'], expected) and
               H.equal(d['actual_fields'], [{'name': n, 'id': RID + '/' + n, 'value': expected[n]} for n, _ in FIELDS]), 'twelve typed literal results and ordered field IDs')
    H.need(len(rt['runs']) == 2, 'two reported internal passes')
    for cmd in [rt['toolchain'], rt['build']] + rt['runs']:
        H.need(cmd['started'] is True and cmd['completed'] is True and cmd['canceled'] is False and
               cmd['timed_out'] is False and cmd['output_truncated'] is False and type(cmd['exit_code']) is int and cmd['exit_code'] == 0, 'internal completion')
    matrix = result_matrix(rows)
    H.need(all(x['stdout_sha256'] == H.digest(matrix) for x in rt['runs']), 'reported output matrices equal literal values')
    if parent is None:
        H.need('replayed_from_sha256' not in e and 'replay' not in r, 'fresh execute has no saved parent')
    else:
        H.need(e['replayed_from_sha256'] == H.digest(parent) and r['replay']['schema'] == 'gooo/workspace-body-replay/v1' and
               H.equal(r['replay']['model_calls'], 0) and c['generated_sha256'] == decoded(parent)['result']['composition']['generated_sha256'], 'official exact raw-parent replay')
    return {'sha256': H.sha(raw), 'cases': len(rows), 'input_echoes': 6 * len(rows), 'typed_fields': 12 * len(rows)}


def refusal_controls(execute, replay, arguments):
    """Mutate actual retained evidence; no compiler or additional producer run."""
    checks = []
    def refuses(name, original, mutate, parent=None):
        value = decoded(original); mutate(value)
        raw = json.dumps(value, ensure_ascii=False, separators=(',', ':')).encode()
        try:
            receipt(raw, *arguments, parent=parent)
        except (RuntimeError, ValueError, KeyError, TypeError):
            checks.append(name)
        else:
            raise RuntimeError('evidence mutation accepted: ' + name)
    refuses('stale-case-bytes', execute, lambda x: x.update(cases_digest='sha256:' + '0' * 64))
    refuses('stale-source-image', execute, lambda x: x['result']['program']['workspace_image']['packages'][0]['sources'][0].update(source_digest='sha256:' + '0' * 64))
    refuses('changed-input-echo', execute, lambda x: x['result']['runtime']['traces'][0]['deliveries'][0]['inputs'][0].update(value='changed'))
    refuses('boolean-as-integer', execute, lambda x: x['result']['runtime']['traces'][0]['deliveries'][0]['actual'].update(version_match=1))
    refuses('wrong-field-id', execute, lambda x: x['result']['runtime']['traces'][0]['deliveries'][0]['actual_fields'][0].update(id='wrong'))
    refuses('wrong-result-entity', execute, lambda x: x['result']['composition']['plan']['activities'][0].update(output_entity_id='wrong'))
    refuses('wrong-go-producer', execute, lambda x: x['result']['runtime'].update(go_tool_sha256='sha256:' + '0' * 64))
    refuses('wrong-saved-parent', replay, lambda x: x.update(replayed_from_sha256='sha256:' + '0' * 64), execute)
    return checks


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ('root', 'compiler', 'go', 'out'): parser.add_argument('--' + name, required=True)
    parser.add_argument('--module-cache', default=os.environ.get('GOMODCACHE', str(Path.home() / 'go/pkg/mod')))
    args = parser.parse_args()
    root, compiler, go, out = [H.canonical(Path(x)) for x in (args.root, args.compiler, args.go, args.out)]
    module_cache = H.canonical(Path(args.module_cache))
    H.need(root == REPO and root.is_dir() and not out.exists(), 'current repository and fresh output')
    out.mkdir(); started = time.monotonic(); pins = {}; runner = None; work = None; owner = None
    error = None; cleaned = False; observations = []; source_ok = False; negatives = []; removed_bytes = 0
    previous = {n: signal.signal(n, lambda number, frame: (_ for _ in ()).throw(RuntimeError('signal ' + str(number)))) for n in (signal.SIGINT, signal.SIGTERM)}
    try:
        read = lambda p: H.frozen_read(p, pins)
        manifest_raw = read(root / MANIFEST); manifest = decoded(manifest_raw)
        H.need(manifest['schema'] == 'gooo/package-workspace-manifest/v1' and H.equal(manifest['entry'], {'package_path': PACKAGE, 'activity': ACTIVITY}), 'current manifest entry')
        sources = {H.safe_relative(n): read(root / n) for pkg in manifest['packages'] for n in pkg['sources']}
        H.need(len(sources) == 12, 'twelve authored sources')
        defs, records, imports = definitions(manifest, sources)
        master_raw = read(root / (BASE + 'cases.json')); master = decoded(master_raw)
        H.need(H.sha(master_raw) == CORPUS_SHA and master['schema'] == 'gooo/body-composition-cases/v1' and len(master['cases']) == 48, 'independent frozen48 literal corpus')
        for row in master['cases']: case_values(row)
        contract = decoded(read(root / (BASE + 'contract.json')))
        check_contract(contract, manifest, sources)
        for name in ('verify.py', 'verify_test.py', 'README.md', 'reference/main.go', 'reference/main_test.go'): read(root / (BASE + name))
        for name in ('tools/check-header-addition-host.py', 'go.mod', 'go.sum', 'upstream.json', 'third_party/GO-LICENSE',
                     'third_party/go/net/http/fs.go.txt', 'third_party/go/net/textproto/textproto.go.txt',
                     'cmd/verify/etag_reference.go', 'cmd/verify/if_match_reference.go',
                     'examples/agentmesh-service-policy/source-pin.json', 'examples/agentmesh-service-policy/NOTICE.md',
                     'examples/agentmesh-service-policy/upstream/LICENSE', 'examples/agentmesh-service-policy/upstream/api/tar.go.txt',
                     'examples/agentmesh-service-policy/oracle/go.mod', 'examples/agentmesh-service-policy/oracle/extracted.go',
                     'examples/agentmesh-service-policy/oracle/cmd/oracle/main.go'): read(root / name)
        lock = decoded(read(root / 'upstream.json'))
        H.need(lock['compiler_revision'] == H.REV and lock['go_version'] == 'go1.27.2', 'repository compiler/Go pins')
        agentmesh_sources(root, read)
        go_sha = H.digest(read(go)); read(compiler)
        H.need(shutil.disk_usage(out).free >= 2 << 30, 'minimum2GiB free')
        work = Path(tempfile.mkdtemp(prefix='.versioned-grant-owned-', dir=out)); owner = (work.stat().st_dev, work.stat().st_ino)
        for name in ('cache', 'modules', 'tmp', 'home', 'gopath'): (work / name).mkdir()
        env = {k: os.environ[k] for k in ('PATH', 'SYSTEMROOT') if k in os.environ}
        env.update(HOME=str(work / 'home'), GOPATH=str(work / 'gopath'), GOTOOLCHAIN='local', GOWORK='off', GOENV='off',
                   GOPROXY='off', GOSUMDB='off', GOFLAGS='-mod=readonly', CGO_ENABLED='0', GOMAXPROCS='2',
                   GOCACHE=str(work / 'cache'), GOMODCACHE=str(work / 'modules'), TMPDIR=str(work / 'tmp'))
        H.save(out / 'owned-work.json', {'work': str(work), 'device': owner[0], 'inode': owner[1],
            'cwd': str(root), 'argv': sys.argv, 'environment': env,
            'limits': {'whole_seconds': 300, 'command_seconds': 90, 'owned_bytes': 1 << 30,
                       'retained_bytes': 64 << 20, 'minimum_free_bytes': 2 << 30, 'stdout_bytes': 8 << 20, 'stderr_bytes': 2 << 20},
            'cleanup': 'Only this newly owned inode after all observed owned processes close; retain evidence and unknown-closure work.',
            'network_scope': 'Offline Go settings and copied modules, not an OS network sandbox.'})
        # Prepare only existing dependency bytes in private owned space; never write shared caches.
        for module in ('golang.org/x/net@v0.59.0', 'golang.org/x/text@v0.42.0'):
            src = H.canonical(module_cache / module)
            H.need(src.is_dir(), 'prepare pinned modules first: ' + module)
            for p in sorted(src.rglob('*')):
                H.need(not p.is_symlink(), 'no symlink module inputs')
                dst = work / 'modules' / module / p.relative_to(src)
                if p.is_dir(): dst.mkdir(parents=True, exist_ok=True)
                else:
                    H.need(p.is_file(), 'regular module file')
                    dst.parent.mkdir(parents=True, exist_ok=True); dst.write_bytes(read(p))
            path, version = module.split('@')
            for suffix in ('mod', 'info', 'ziphash'):
                src_meta = module_cache / 'cache/download' / path / '@v' / (version + '.' + suffix)
                if src_meta.is_file():
                    dst = work / 'modules/cache/download' / path / '@v' / src_meta.name
                    dst.parent.mkdir(parents=True, exist_ok=True); dst.write_bytes(read(src_meta))
            H.need(H.owned_bytes(work) <= 1 << 30, 'owned1GiB preparation cap')
        runner = H.OwnedCommands(out, work, root, pins, env); runner.started = started
        def call(label, argv, cwd=None):
            H.need(shutil.disk_usage(out).free >= 2 << 30 and H.owned_bytes(out) - H.owned_bytes(work) <= 64 << 20, 'free/evidence bounds')
            runner.cwd = cwd or root
            raw = runner.call(label, argv, 90)
            H.need(H.owned_bytes(out) - H.owned_bytes(work) <= 64 << 20, 'retained64MiB bound')
            return raw
        identity = decoded(call('compiler-identity', [compiler, 'version', '--build', '--json']))
        for k, v in {'compiler_source_sha': H.REV, 'vcs_revision': H.REV, 'source_status': 'CLEAN_VCS',
                     'vcs_modified': 'false', 'build_info_observed': True, 'version': '0.6.32-dev',
                     'go_version': 'go1.27.2', 'native_go_required': 'go1.27.2'}.items(): H.need(H.equal(identity.get(k), v), 'clean compiler identity: ' + k)
        go_version = call('go-version', [go, 'version']).decode().strip()
        H.need(go_version.startswith('go version go1.27.2 '), 'exact pinned Go version')
        call('reference-vet', [go, 'vet', './' + BASE + 'reference'])
        call('reference-tests', [go, 'test', '-count=1', './' + BASE + 'reference'])
        prepared = out / 'prepared'
        call('fresh-reference', [go, 'run', './' + BASE + 'reference', '--root', root, '--cases', root / (BASE + 'cases.json'), '--manifest', root / MANIFEST, '--out', prepared])
        plan = decoded(H.frozen_read(prepared / 'plan.json', pins)); images = plan['images']
        H.need(plan['schema'] == 'gooo/versioned-grant-reference-plan/v1' and plan['rows'] == 48 and
               plan['master_sha256'] == H.digest(master_raw) and plan['manifest_sha256'] == H.digest(manifest_raw) and
               plan['source_bytes'] == sum(map(len, sources.values())) and plan['go_version'] == 'go1.27.2', 'fresh reference plan/source binding')
        expected_images = [{'package': pkg['path'], 'filename': pkg['sources'][0], 'source_digest': H.digest(H.go_json(sources[pkg['sources'][0]].decode()))} for pkg in manifest['packages']]
        H.need(H.equal(images, expected_images), 'fresh Go actual source-image serialization')
        components = plan['components']
        H.need(len(components) == 48, 'all48 fresh component observations')
        for row, component in zip(master['cases'], components):
            values, expected = case_values(row)
            H.need(type(component['if_match_condition']) is int and component['if_match_condition'] == expected['if_match_condition'], 'fresh original IfMatch condition')
            for index, name, field in [(0, 'selected', 'expected_validator_strict'), (1, 'current', 'current_validator_strict')]:
                facts = component[name]
                H.need(type(facts['tag']) is str and type(facts['remainder']) is str and
                       type(facts['header_valid']) is bool and type(facts['strong']) is bool, 'fresh component scalar types')
                strict = facts['header_valid'] and facts['strong'] and facts['remainder'] == '' and facts['tag'] == values[index]
                H.need(H.equal(strict, expected[field]), 'fresh upstream validator facts agree with independent strict literal')
        impacts = []
        for i, row in enumerate(master['cases']):
            v, _ = case_values(row)
            impacts.append({'name': str(i), 'before': v[2], 'after': v[3], 'req_type': v[4], 'req_name': v[5]})
        H.save(prepared / 'matcher.json', []); H.save(prepared / 'impact.json', impacts)
        H.frozen_read(prepared / 'matcher.json', pins); H.frozen_read(prepared / 'impact.json', pins)
        oracle = decoded(call('agentmesh-components', [go, 'run', './cmd/oracle', '--cases', prepared / 'matcher.json', '--impact-cases', prepared / 'impact.json'], root / 'examples/agentmesh-service-policy/oracle'))
        H.need(oracle['schema'] == 'gooo/agentmesh-matcher-oracle/v1' and oracle['source_revision'] == '9fd1a41a15174df0aaef601c942410c2116bff91' and oracle['go_version'] == 'go1.27.2' and len(oracle['impact_matches']) == 48, 'fresh original AgentMesh reference')
        for i, (row, observed) in enumerate(zip(master['cases'], oracle['impact_matches'])):
            _, expected = case_values(row)
            H.need(observed['name'] == str(i), 'ordered upstream component identity')
            if expected['status'] != 'INVALID_INPUT':
                H.need(H.equal(observed['before_match'], expected['required_before_match']) and H.equal(observed['after_match'], expected['required_after_match']), 'fresh required-request component observations')
        position = 0
        for i, batch in enumerate(plan['batches']):
            start, end = batch['start'], batch['end']
            H.need(type(start) is int and type(end) is int and start == position and 0 < end - start <= 8 and end <= 48, 'contiguous complete greedy partition')
            case_path = H.canonical(prepared / H.safe_relative(batch['file'])); case_raw = H.frozen_read(case_path, pins)
            rows = master['cases'][start:end]
            document = decoded(case_raw)
            upper = wire_upper(document, sum(map(len, sources.values())))
            H.need(len(case_raw) <= 30000 and H.equal(document, {'schema': master['schema'], 'cases': rows}) and
                   type(batch['raw_bytes']) is int and batch['raw_bytes'] == len(case_raw) and
                   type(batch['lowered_upper_bytes']) is int and batch['lowered_upper_bytes'] == upper <= 32768, 'actual written case budget and unchanged occurrences')
            execute = call('execute-' + str(i), [compiler, 'package', 'execute', '--json', '--go', go, '--cases', case_path, root / MANIFEST])
            a = receipt(execute, rows, case_raw, manifest_raw, root / MANIFEST, sources, images, defs, records, imports, go_version, go_sha)
            replay = call('replay-' + str(i), [compiler, 'package', 'replay', '--json', '--go', go, '--receipt', out / ('execute-' + str(i)) / 'stdout', '--cases', case_path, root / MANIFEST])
            b = receipt(replay, rows, case_raw, manifest_raw, root / MANIFEST, sources, images, defs, records, imports, go_version, go_sha, execute)
            if i == 0:
                negatives = refusal_controls(execute, replay, (rows, case_raw, manifest_raw, root / MANIFEST, sources,
                                                              images, defs, records, imports, go_version, go_sha))
                H.save(out / 'negative-controls.json', negatives)
            observations.append({'start': start, 'end': end, 'execute': a, 'official_replay': b}); position = end
        H.need(position == 48 and runner.closed, 'all48 occurrences and observed processes closed')
        H.unchanged(pins); source_ok = True
    except BaseException as exc:
        error = str(exc)
    finally:
        try:
            H.unchanged(pins)
            source_ok = str(go) in pins and str(compiler) in pins
        except BaseException as exc:
            error = error or str(exc); source_ok = False
        if work is not None and (runner is None or runner.closed):
            try:
                H.need(not work.is_symlink() and (work.stat().st_dev, work.stat().st_ino) == owner, 'owned directory identity before cleanup')
                removed_bytes = H.owned_bytes(work)
                shutil.rmtree(work); cleaned = True
            except BaseException as exc: error = error or 'owned cleanup: ' + str(exc)
        for n, handler in previous.items(): signal.signal(n, handler)
    H.save(out / 'source-tool-pins.json', pins)
    summary = {'schema': 'gooo/versioned-grant-change-verification/v1', 'status': 'FINITE48_FRESH_REFERENCE_NATIVE_REPLAY_PASS' if error is None else 'FIRST_FAILURE_PRESERVED',
               'failure': error, 'compiler_revision': H.REV, 'cases': 48 if error is None else None,
               'input_echoes_each_phase': sum(x['execute']['input_echoes'] for x in observations),
               'typed_fields_each_phase': sum(x['execute']['typed_fields'] for x in observations),
               'source_images': 12, 'model_calls': 0, 'batches': observations, 'negative_controls': negatives,
               'source_tool_unchanged': source_ok, 'processes_closed': runner.closed if runner else True,
               'owned_work_removed': cleaned, 'owned_logical_bytes_removed': removed_bytes if cleaned else 0,
               'commands': runner.observations if runner else [], 'elapsed_s': time.monotonic() - started,
               'scope': 'Finite48 literals plus fresh upstream component observations only. No full domain, strict public network ingress, current authority, atomic effects, production safety or independent inner canonical verifier. No engine build/download or shared module-cache writes.'}
    H.save(out / 'summary.json', summary); print(json.dumps(summary)); return 0 if error is None else 1


if __name__ == '__main__':
    sys.exit(main())
