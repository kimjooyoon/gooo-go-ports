#!/usr/bin/env python3
"""Finite original-Python/independent-Go/Gooo resolver check. Stdlib only."""
import argparse
import ast
import dataclasses
import enum
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import signal
import tempfile
import time
import typing

PACKAGE = 'gooo/autobot/ai-lines'
ENTRY = PACKAGE + ':ResolveLine'
ACTIVITY_ID = 'gooo://autobot/ai-lines/resolve-line/v1'
RID = 'gooo://autobot/ai-lines/line-decision/v1'
SOURCE = 'ports/autobot/ai_lines.gooo'
MANIFEST = 'autobot-resolver.workspace.json'
EXAMPLE = 'examples/autobot-resolver'
FIELDS = ['key', 'line', 'source', 'stored_present', 'stored_value', 'floor_present', 'floor_value', 'reason']
TYPES = ['Text' if i in (0, 2, 5) else 'Boolean' for i in range(9)]
REV = '205896a413e5dea92366309c35ae85e542d055f6'
FIXED = {
    'corpus.json': 'bf622cf3171cd9bab4baf25561fe84f9a464f2e1c9a5c24c54021df850e4ed50',
    'oracle.go': 'ee13bf8d87ef0cf724c8025443df0bf36fcb719ad8afbc230ff3aa51c67f8864',
    'upstream/ai_lines.py.txt': '359a2852ac3bfee256b83f93bc9f386e1af75ecaa3ab21818dbd48081a444c9c',
    'upstream/LICENSE': 'deb04126dcfbc5ff823935ac8b404fb86ea53dd4743e2d6620bffe83ec9ec0e8',
    'upstream/NOTICE': '6f76fa3e99640baf4a6fd1d0268eb2e44146d066e814e5827046dd294672508f',
}


def need(ok, message):
    if not ok:
        raise ValueError(message)


def digest(raw):
    return 'sha256:' + hashlib.sha256(raw).hexdigest()


def load(raw, label):
    def pairs(items):
        result = {}
        for key, value in items:
            need(key not in result, 'duplicate JSON key: ' + key)
            result[key] = value
        return result
    try:
        return json.loads(raw.decode('utf-8'), object_pairs_hook=pairs,
                          parse_constant=lambda _: (_ for _ in ()).throw(ValueError('nonfinite JSON')))
    except (UnicodeError, ValueError) as error:
        raise ValueError(label + ': invalid JSON: ' + str(error)) from error


def equal(a, b):
    if type(a) is not type(b):
        return False
    if isinstance(a, dict):
        return a.keys() == b.keys() and all(equal(a[k], b[k]) for k in a)
    if isinstance(a, list):
        return len(a) == len(b) and all(equal(x, y) for x, y in zip(a, b))
    return a == b


def json_bytes(value):
    text = json.dumps(value, ensure_ascii=False, separators=(',', ':'), allow_nan=False)
    for old, new in [('&', '\\u0026'), ('<', '\\u003c'), ('>', '\\u003e'), ('\u2028', '\\u2028'), ('\u2029', '\\u2029')]:
        text = text.replace(old, new)
    return text.encode('utf-8') + b'\n'


def validate_rows(corpus):
    need(type(corpus) is dict and set(corpus) == {'schema', 'cases'} and
         corpus['schema'] == 'gooo/autobot-resolver-corpus/v1', 'corpus schema/keys')
    cases = corpus['cases']
    need(type(cases) is list and len(cases) == 204, 'exact original204 rows')
    identifiers, rows = set(), []
    for case in cases:
        need(type(case) is dict and set(case) == {'id', 'inputs'} and
             type(case['id']) is str and case['id'] and case['id'] not in identifiers, 'unique original case identity')
        identifiers.add(case['id'])
        row = case['inputs']
        need(type(row) is list and len(row) == 9, 'nine scalar inputs')
        for kind, value in zip(TYPES, row):
            need(type(value) is (str if kind == 'Text' else bool), 'strict scalar input type')
            if kind == 'Text':
                try:
                    size = len(value.encode('utf-8'))
                except UnicodeError as error:
                    raise ValueError('Text must contain Unicode scalars') from error
                need(size <= 1024, 'Text1024UTF8-byte bound')
        need((row[1] and row[2] in ('autonomous', 'draft_then_send', 'never')) or
             (not row[1] and row[2] == ''), 'canonical stored presence/enum')
        need(row[4] or row[5] == '', 'canonical absent baseline')
        rows.append(row)
    return rows


def validate_pin(pin, read):
    need(pin['schema'] == 'gooo/autobot-resolver-source-pin/v1' and
         pin['upstream_revision'] == 'a9c47e296907f1e5f681c38c05146cc8d3c2e7a7', 'pinned original revision')
    refs = pin['upstream_files'] + [pin['port'], pin['workspace'],
                                  pin['verification_inputs']['corpus'], pin['verification_inputs']['oracle']]
    paths = {EXAMPLE + '/' + name for name in FIXED} | {SOURCE, MANIFEST}
    need(len(refs) == len(paths) and {ref['path'] for ref in refs} == paths, 'exact source/provenance reference roster')
    for ref in refs:
        raw = read(ref['path'])
        need(type(ref['bytes']) is int and len(raw) == ref['bytes'] and
             hashlib.sha256(raw).hexdigest() == ref['sha256'], 'source pin bytes: ' + ref['path'])
    need(pin['entry'] == {'package_path': PACKAGE, 'activity': 'ResolveLine', 'qualified': ENTRY, 'id': ACTIVITY_ID} and
         [row['type'] for row in pin['inputs']] == TYPES and
         [row['port'] for row in pin['inputs']] == ['input' + str(i) for i in range(9)] and
         [row['entity_id'] for row in pin['inputs']] == ['gooo://std/' + ('text' if t == 'Text' else 'boolean') for t in TYPES],
         'source pin entry/input ABI')
    fields = [{'name': name, 'id': RID.rsplit('/', 1)[0] + '/' + name.replace('_', '-'),
               'type': 'boolean' if name in ('stored_present', 'floor_present') else 'string', 'required': True}
              for name in FIELDS]
    need(pin['output']['id'] == RID and equal(pin['output']['fields'], fields) and
         pin['verification_inputs']['cases'] == 204 and pin['verification_inputs']['group_rows'] == 8 and
         pin['verification_inputs']['groups'] == 26, 'source pin output/corpus contract')


def original_reference(source, rows):
    names = {'AiLine', '_RANKS', 'DEFAULT_LINE', 'LineResolution', 'resolve_line', '_unset',
             'SOURCE_STORED', 'SOURCE_DEFAULT', 'SOURCE_BASELINE', 'SOURCE_ROLE',
             'SOURCE_UNAVAILABLE', 'SOURCE_UNMANAGED', 'SOURCE_NO_COMPANY'}
    selected, found = [], set()
    for node in ast.parse(source).body:
        name = getattr(node, 'name', None)
        if isinstance(node, ast.Assign) and len(node.targets) == 1 and isinstance(node.targets[0], ast.Name):
            name = node.targets[0].id
        if name in names:
            need(name not in found, 'duplicate original pure definition')
            selected.append(node)
            found.add(name)
    need(found == names, 'exact pinned original pure-node roster')
    namespace = {'__name__': __name__, 'Enum': enum.Enum, 'dataclass': dataclasses.dataclass, 'Optional': typing.Optional}
    # Full source identity is checked before this function. Imports, tool/scope
    # classification and storage/dispatch code are not selected or executed.
    exec(compile(ast.Module(body=selected, type_ignores=[]), 'pinned-autobot-pure-definitions', 'exec', dont_inherit=True), namespace)
    values = []
    for row in rows:
        value = namespace['resolve_line'](row[0], stored=namespace['AiLine'](row[2]) if row[1] else None,
            mutating=row[3], baseline_category=row[5] if row[4] else None, role_requires_approval=row[6],
            store_unavailable=row[7], scope_missing=row[8])
        values.append(dict(key=value.key, line=value.line.value, source=value.source,
            stored_present=value.stored is not None, stored_value=value.stored.value if value.stored is not None else '',
            floor_present=value.floor_category is not None, floor_value=value.floor_category if value.floor_category is not None else '',
            reason=value.reason))
    return values


def input_document(raw, rows):
    need(0 < len(raw) <= 32768 and raw.endswith(b'\n') and not raw.endswith(b'\n\n'), 'converter final wire/LF bound')
    expected = {'schema': 'gooo/body-composition-inputs/v1',
                'inputs': [{ENTRY + '.input' + str(i): value for i, value in enumerate(row)} for row in rows]}
    need(equal(load(raw, 'input converter stdout'), expected), 'converter exact original nine inputs; no expected values')


def inspect(raw, rows, expected, input_raw, source, manifest_raw, manifest_path, version, go_raw, H, parent=None):
    """Read only the observed input-only receipt; expectations stay external."""
    try:
        receipt = load(raw, 'Gooo receipt')
        need(receipt['schema'] == 'gooo/workspace-body-execution-receipt/v1' and
             receipt['decision'] == 'OBSERVED' and not receipt.get('error'), 'input-only OBSERVED envelope')
        need(receipt['manifest'] == str(manifest_path) and receipt['manifest_digest'] == digest(manifest_raw), 'exact current manifest')
        need(receipt['inputs_digest'] == digest(input_raw) and 'cases_digest' not in receipt, 'exact actual converter bytes; no finite case envelope')
        result = receipt['result']; program = result['program']; composition = result['composition']; runtime = result['runtime']
        need(result['schema'] == 'gooo/workspace-body-execution/v1' and program['schema'] == 'gooo/workspace-body-program/v1', 'program schemas')
        image = program['workspace_image']
        need(image['schema'] == 'gooo/package-runtime-image/v1' and image['init_order'] == [PACKAGE] and
             len(image['packages']) == 1 and image['packages'][0]['path'] == PACKAGE, 'sole original source package')
        images = image['packages'][0]['sources']
        need(len(images) == 1 and images[0]['filename'] == SOURCE and
             images[0]['source_digest'] == digest(H.go_json(source.decode('utf-8'))), 'exact current source image')
        definitions = H.activities(source.decode('utf-8')); lowered = H.activities(program['lowered_gooo_source'])
        need(len(definitions) == len(lowered) == 1, 'one source/lowered activity')
        definition, lowered = definitions[0], lowered[0]
        need(definition['name'] == 'ResolveLine' and definition['id'] == ACTIVITY_ID and definition['inputs'] == TYPES and
             all(equal(lowered[k], definition[k]) for k in ['id', 'inputs', 'output', 'body']), 'unchanged original body and nine-input ABI')
        entry = {'package_path': PACKAGE, 'activity': 'ResolveLine', 'lowered_name': lowered['name']}
        need(equal(program['entry'], entry) and equal(program['activities'], [entry]), 'exact sole program entry')
        need(equal(image['entry'], {'package_path': PACKAGE, 'source': SOURCE, 'activity': 'ResolveLine',
                                   'inputs': TYPES, 'output': 'LineDecision'}), 'source entry ABI')
        need(composition['schema'] == 'gooo/body-composition/v1' and composition['stage'] == 'COMPLETE' and
             len(composition['steps']) == 1, 'complete single generated composition')
        generation = composition['steps'][0]['generation']; report = generation['report']; plan = composition['plan']
        need(report['schema'] == 'gooo/body-codegen-report/v3' and report['decision'] == 'PASS' and
             report['compiler_source_sha'] == REV and report['activity_id'] == ACTIVITY_ID and
             report['output_type'] == 'LineDecision' and report['typecheck_passed'] is True and
             report['deterministic_replay'] is True and equal(report['repository_writes'], 0), 'typed codegen producer')
        need(equal(report['input_parameters'], [{'name': 'input' + str(i), 'type': 'string' if kind == 'Text' else 'bool'}
                                               for i, kind in enumerate(TYPES)]), 'nine generated input parameters')
        need(plan['schema'] == 'gooo/body-composition-plan/v1' and len(plan['activities']) == 1 and
             plan['edges'] == [], 'single no-edge typed plan')
        activity = plan['activities'][0]
        ports = [{'port': 'input' + str(i), 'type': kind, 'entity_id': 'gooo://std/' +
                  ('text' if kind == 'Text' else 'boolean'), 'from': -1} for i, kind in enumerate(TYPES)]
        need(activity['name'] == entry['lowered_name'] and activity['id'] == ACTIVITY_ID and
             activity['output_entity_id'] == RID and equal(activity['inputs'], ports), 'planned typed ports/result')
        fields = [{'name': name, 'id': RID.rsplit('/', 1)[0] + '/' + name.replace('_', '-'),
                   'type_id': 'urn:gooo:type:' + ('boolean' if name in ('stored_present', 'floor_present') else 'string')}
                  for name in FIELDS]
        for layouts in [report['record_types'], plan['record_types']]:
            need(len(layouts) == 1 and layouts[0]['id'] == RID and
                 equal([{k: field[k] for k in ('name', 'id', 'type_id')} for field in layouts[0]['fields']], fields),
                 'all eight ordered field identities/types')
        source_sha = digest(program['lowered_gooo_source'].encode('utf-8'))
        need(composition['gooo_source'] == program['lowered_gooo_source'] and source_sha == result['execution_source_sha256'] ==
             composition['original_source_sha256'] == composition['selected_source_sha256'] ==
             runtime['original_source_sha256'] == runtime['selected_source_sha256'], 'lowered source bindings')
        need(composition['steps'][0]['input_source_sha256'] == report['source_digest'] == source_sha and
             report['program_digest'] == digest(lowered['body'].encode()) and
             digest(generation['source'].encode()) == report['generated_digest'] == report['replay_digest'], 'codegen source/program/output hashes')
        need(digest(composition['source'].encode()) == composition['generated_sha256'] == runtime['generated_sha256'] and
             digest(composition['driver_source'].encode()) == composition['driver_sha256'] == runtime['driver_sha256'], 'generated source/driver hashes')
        need(runtime['schema'] == 'gooo/body-composition-runtime/v1' and runtime['producer_source_sha'] == REV and
             runtime['go_version'] == version and runtime['go_tool_selection'] == 'explicit' and
             runtime['go_tool_sha256'] == digest(go_raw), 'pinned compiler and actual explicit Go bytes')
        need(runtime['stage'] == 'COMPLETE' and not runtime.get('failure') and
             all(equal(runtime[key], 0) for key in ['finite_total', 'finite_passed', 'model_calls']) and
             runtime['projection_replayed'] is True and runtime['runtime_replayed'] is True, 'input-only complete zero-finite/zero-model runtime')
        need(len(runtime['runs']) == 2, 'two actual native executions')
        for process in [runtime['toolchain'], runtime['build']] + runtime['runs']:
            need(process['started'] is True and process['completed'] is True and equal(process['exit_code'], 0) and
                 all(process[key] is False for key in ('canceled', 'timed_out', 'output_truncated')), 'native process clean completion')
        need(len(runtime['traces']) == len(rows) == len(expected), 'complete original ordered rows')
        for i, (trace, row, wanted) in enumerate(zip(runtime['traces'], rows, expected)):
            need(equal(trace['case_index'], i) and len(trace['deliveries']) == 1, 'ordered sole delivery')
            delivery = trace['deliveries'][0]
            supplied = [{'port': 'input' + str(j), 'entity_id': port['entity_id'], 'value': value}
                        for j, (port, value) in enumerate(zip(ports, row))]
            need(delivery['activity_id'] == ACTIVITY_ID and 'passed' not in delivery and 'expected' not in delivery,
                 'input-only observation without producer expectations')
            need(equal(delivery['inputs'], supplied), 'all nine original typed input echoes')
            need(type(wanted) is dict and list(wanted) == FIELDS and equal(delivery['actual'], wanted), 'all eight external independent outputs')
            field_values = [{'id': field['id'], 'name': name, 'value': wanted[name]} for field, name in zip(fields, FIELDS)]
            need(equal(delivery['actual_fields'], field_values), 'all eight ordered typed fields')
        output_hash = digest(json_bytes([[{name: value[name] for name in FIELDS}] for value in expected]))
        need(all(process['stdout_sha256'] == output_hash for process in runtime['runs']), 'both actual native stdout hashes')
        if parent is not None:
            need(receipt['replayed_from_sha256'] == digest(parent) and result['replay']['schema'] == 'gooo/workspace-body-replay/v1' and
                 equal(result['replay']['model_calls'], 0), 'exact raw saved original replay parent')
        return {'rows': len(rows), 'receipt_sha256': digest(raw), 'source_sha256': source_sha}
    except (KeyError, IndexError, TypeError, UnicodeError) as error:
        raise ValueError('malformed Gooo receipt structure: ' + str(error)) from error


def helpers(root):
    spec = importlib.util.spec_from_file_location('autobot_owned_commands', root / 'tools/check-header-addition-host.py')
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', default=str(Path(__file__).resolve().parents[2]))
    for name in ['compiler', 'go', 'input-command', 'out']:
        parser.add_argument('--' + name, required=True)
    args = parser.parse_args(argv)
    root = Path(args.root).absolute()
    need(root.resolve() == root and root.is_dir(), 'canonical repository root required')
    H = helpers(root)
    compiler, go, command = [H.canonical(Path(value).absolute()) for value in [args.compiler, args.go, args.input_command]]
    out = H.canonical((root / args.out).absolute())
    need(not out.exists() and out.parent.is_dir() and out != root and out not in root.parents, 'fresh output directory with existing parent required')
    out.mkdir()
    pins = {}; runner = None; work = None; failure = None; stage = 'source'; verified = []; cleaned = False
    started = time.monotonic()
    original_handlers = {number: signal.signal(number, lambda signum, _: (_ for _ in ()).throw(RuntimeError('signal ' + str(signum))))
                         for number in [signal.SIGTERM, signal.SIGINT]}
    try:
        read = lambda path: H.frozen_read(path, pins)
        example = root / EXAMPLE
        raw_files = {name: read(example / name) for name in FIXED}
        need(all(hashlib.sha256(raw_files[name]).hexdigest() == sha for name, sha in FIXED.items()), 'original source/license/corpus/oracle identities')
        for name in ['verify.py', 'verify_test.py', 'source-pin.json', 'README.md', 'NOTICE.md']:
            read(example / name)
        validate_pin(load(read(example / 'source-pin.json'), 'source pin'), lambda name: read(root / name))
        for name in ['tools/check-header-addition-host.py', 'cmd/autobot-line-input/main.go', 'cmd/autobot-line-input/main_test.go', 'upstream.json']:
            read(root / name)
        source, manifest_raw = read(root / SOURCE), read(root / MANIFEST)
        need(equal(load(manifest_raw, 'workspace'), {'schema': 'gooo/package-workspace-manifest/v1',
             'entry': {'package_path': PACKAGE, 'activity': 'ResolveLine'},
             'packages': [{'path': PACKAGE, 'sources': [SOURCE]}]}), 'one current source manifest')
        upstream = load(read(root / 'upstream.json'), 'compiler lock')
        need(upstream['compiler_revision'] == REV and upstream['go_version'] == 'go1.27.3', 'repository compiler/Go pins')
        rows = validate_rows(load(raw_files['corpus.json'], 'original corpus'))
        stage = 'original_python_ast'
        original = original_reference(raw_files['upstream/ai_lines.py.txt'].decode('utf-8'), rows)
        read(compiler); go_raw = read(go); read(command)
        need(shutil.disk_usage(out).free >= 2 << 30, 'at least2GiB free for bounded owned work')
        work = Path(tempfile.mkdtemp(prefix='.autobot-owned-', dir=out)); owner = (work.stat().st_dev, work.stat().st_ino)
        for name in ['cache', 'modules', 'tmp', 'gopath']:
            (work / name).mkdir()
        env = {key: os.environ[key] for key in ['PATH', 'HOME', 'SYSTEMROOT'] if key in os.environ}
        env.update(GOTOOLCHAIN='local', GOWORK='off', GOENV='off', GOPROXY='off', GOSUMDB='off',
                   GOVCS='*:off', GOTELEMETRY='off', GOFLAGS='-mod=readonly', GOCACHE=str(work / 'cache'),
                   GOMODCACHE=str(work / 'modules'), GOPATH=str(work / 'gopath'), TMPDIR=str(work / 'tmp'),
                   CGO_ENABLED='0', GOMAXPROCS='2', GOOO_LAYA_URL='', GOOO_LAYA_API_KEY='')
        runner = H.OwnedCommands(out, work, root, pins, env); runner.started = started
        H.save(out / 'owned-work.json', {'work': str(work), 'whole_timeout_s': 300, 'owned_byte_cap': 1 << 30,
                                       'retained_byte_cap': 64 << 20, 'environment': env})
        stage = 'tool_identity'
        identity = load(runner.call('compiler-identity', [compiler, 'version', '--build', '--json'], 10), 'compiler identity')
        need(identity['compiler_source_sha'] == identity['vcs_revision'] == REV and identity['source_status'] == 'CLEAN_VCS' and
             identity['vcs_modified'] == 'false' and identity['go_version'] == 'go1.27.3', 'existing clean pinned compiler')
        version = runner.call('go-version', [go, 'version'], 10).decode().strip()
        need(len(version.split()) == 4 and version.split()[:3] == ['go', 'version', 'go1.27.3'], 'actual explicit Go1.27.3')
        stage = 'independent_go_oracle'
        H.save(out / 'oracle-input.json', rows)
        expected = load(runner.call('go-oracle', [go, 'run', example / 'oracle.go'], 90,
                        stdin_path=out / 'oracle-input.json', extra_env={'GO111MODULE': 'off', 'GOFLAGS': ''}), 'independent Go oracle stdout')
        need(equal(expected, original), 'all204 independent Go values equal exact original Python definitions')
        H.save(out / 'original-python-expected.json', original)
        for offset in range(0, 204, 8):
            batch, wanted = rows[offset:offset + 8], expected[offset:offset + 8]
            stage = 'input_converter:' + str(offset)
            request = out / ('rows-%03d.json' % offset); H.save(request, batch); read(request)
            label = '%03d-input' % offset
            input_raw = runner.call(label, [command], 10, stdin_path=request)
            input_document(input_raw, batch)
            need((out / label / 'stderr').stat().st_size == 0, 'input converter stderr must be empty')
            input_path = out / label / 'stdout'; read(input_path)
            parent = None
            for operation in ['execute', 'replay']:
                stage = operation + ':' + str(offset)
                args = [compiler, 'package', operation, '--json', '--inputs', input_path, '--go', go]
                if parent is not None:
                    args += ['--receipt', out / ('%03d-execute' % offset) / 'stdout']
                args += [root / MANIFEST]
                need(shutil.disk_usage(out).free >= 2 << 30 and H.owned_bytes(out) - H.owned_bytes(work) <= 64 << 20, 'free/retained evidence bounds')
                raw = runner.call('%03d-%s' % (offset, operation), args, 90)
                stage = 'receipt_' + stage
                verified.append({'offset': offset, 'operation': operation, **inspect(raw, batch, wanted, input_raw, source,
                    manifest_raw, root / MANIFEST, version, go_raw, H, parent)})
                need(H.owned_bytes(out) - H.owned_bytes(work) <= 64 << 20, 'retained evidence64MiB bound')
                if parent is None:
                    parent = raw
        H.unchanged(pins)
        need(len(verified) == 52 and all(sum(x['rows'] for x in verified if x['operation'] == op) == 204
                                       for op in ['execute', 'replay']), 'whole original204 coverage in both phases')
        stage = 'complete'
    except BaseException as error:
        failure = str(error)
    finally:
        if work is not None and runner is not None and runner.closed:
            try:
                need((work.stat().st_dev, work.stat().st_ino) == owner, 'owned work identity before cleanup')
                shutil.rmtree(work); cleaned = True
            except BaseException as error:
                failure = failure or 'owned cleanup: ' + str(error)
        for number, handler in original_handlers.items():
            signal.signal(number, handler)
        successful = failure is None and len(verified) == 52 and cleaned and runner is not None and runner.closed
        H.save(out / 'summary.json', {'schema': 'gooo/autobot-resolver-verification/v1',
            'status': 'ORIGINAL_PYTHON_GO_AND_GOOO_REPLAY_204_PASS' if successful else 'FAIL', 'failure': failure,
            'stage': stage, 'rows': 204 if successful else None, 'batches': 26 if successful else None,
            'Gooo_commands': sum(x['label'].endswith(('-execute', '-replay')) for x in runner.observations) if runner else 0,
            'helper_commands': sum(x['label'].endswith('-input') for x in runner.observations) if runner else 0,
            'input_echoes_each_phase': 1836 if successful else None, 'output_fields_each_phase': 1632 if successful else None,
            'finite_total': 0 if successful else None, 'model_calls': 0 if successful else None,
            'source_tool_unchanged': successful, 'processes_closed': runner.closed if runner else True,
            'owned_work_removed': cleaned, 'compiler_revision': REV, 'verified': verified,
            'source_tool_pins': pins, 'commands': runner.observations if runner else [], 'wall_seconds': time.monotonic() - started,
            'scope': '204 finite original pure-rule rows through the strict CLI, external original-Python and independent-Go expectations; OBSERVED input-only native/saved replay, not finite producer expectations. No AutoBot integration, current authority, artifact persistence, dispatcher/effects, all-string, production or speed claim. Inner typed canonical parent digest is not independently checked.'})
    return 0 if successful else 1


if __name__ == '__main__':
    raise SystemExit(main())
