#!/usr/bin/env python3
"""CI control: refactor an editable policy using the explicit source-inspection profile.

The public consumer owns fresh source inspection, typed native observations and
official saved replay. This control compares two actual consumer runs; it does
not implement policy behavior or independently parse the Gooo call graph.
"""
import argparse
import importlib.util
import os
from pathlib import Path
import shutil
import signal
import sys
import tempfile
import time

REPO = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location('editable_helper_processes', REPO / 'tools/check-header-addition-host.py')
H = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(H)
BASE = 'examples/editable-api-policy/'
FILES = tuple(BASE + n for n in ('run.py', 'run_test.py', 'README.md', 'NOTICE.md',
                               'before.gooo', 'after.gooo', 'demo-inputs.json', 'demo-expected.json')) + (
    'editable-api-policy-before.workspace.json', 'editable-api-policy-after.workspace.json',
    'ports/cel/scalar_access.gooo', 'examples/cel-api-policy/main.gooo',
    'tools/check-header-addition-host.py', 'tools/native_verification_contract.py', 'upstream.json')
REV = '31497af3beaef210b5a075867b0de76cf55ee3ca'
ENTRY = 'app/editable-api-policy:EvaluateAPI'
USED = 'app/editable-api-policy:IsDeployBot'
UNUSED = 'app/editable-api-policy:UnusedIdentity'
PREDICATE = 'base.effective_principal != "synthetic:deploy-bot"'
HELPERS = '''
activity IsDeployBot(Text) -> Boolean id "gooo://editable-api-policy/is-deploy-bot/v1" computes `
return input == "synthetic:deploy-bot"
`

activity UnusedIdentity(Text) -> Text id "gooo://editable-api-policy/unused-identity/v1" computes `
return input
`
'''


def load(path):
    return H.load(path.read_text(encoding='utf-8'))


def complete(summary):
    for key, wanted in {
        'schema': 'gooo/editable-api-policy-comparison/v1',
        'status': 'EDITABLE_POLICY_COMPARISON_AND_OFFICIAL_REPLAY_PASS', 'failure': None,
        'rows': 3, 'positive_receipts': 4, 'package_commands': 7,
        'compiler_profile': 'source-inspection036', 'prepared_inspections': 2,
        'input_echoes_checked': 108, 'typed_output_fields_checked': 180,
        'compiler_revision': REV, 'finite_total': 0, 'model_calls': 0, 'demo_checked': True,
        'cross_workspace_refused': True, 'cross_workspace_refusal_stage': 'manifest',
        'source_tool_unchanged': True, 'processes_closed': True, 'owned_work_removed': True,
    }.items():
        H.need(H.equal(summary.get(key), wanted), 'fresh consumer summary: ' + key)
    commands = summary['commands']
    H.need([c['label'] for c in commands] == ['compiler-identity', 'go-version',
           'before-inspect', 'before-execute', 'before-replay', 'after-inspect',
           'after-execute', 'after-replay', 'cross-workspace-refusal'] and
           all(c['failure'] is None and c['observed_owned_processes_remaining'] == []
           and c['exit_code'] == (1 if c['label'] == 'cross-workspace-refusal' else 0) for c in commands),
           'all current inspection/native/replay/refusal commands closed')


def prepared(directory, variant):
    inspect_raw = (directory / (variant + '-inspect/stdout')).read_bytes()
    inspection = H.load(inspect_raw.decode('utf-8'))
    program = inspection['program']
    H.need(inspection['decision'] == 'PREPARED' and
           H.equal([inspection['program_executions'], inspection['model_predictions']], [0, 0]),
           'current source-only inspection')
    pure = program['pure_calls']
    names = {p['activity']['package_path'] + ':' + p['activity']['activity'] for p in pure['activities']}
    for operation in ('execute', 'replay'):
        receipt = load(directory / (variant + '-' + operation + '/stdout'))
        H.need(H.equal(receipt['result']['program'], program), 'own inspection matches full executed/replayed program')
    return names, len(pure['sites']), H.digest(inspect_raw)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ('root', 'compiler', 'go', 'out'): parser.add_argument('--' + name, required=True)
    args = parser.parse_args()
    out = H.canonical(Path(args.out))
    H.need(out.parent.is_dir() and not out.exists(), 'fresh output with existing parent')
    out.mkdir(); started = time.monotonic(); pins = {}; runner = work = owner = None
    failure = None; summaries = []; source_ok = cleaned = refused = False; inspected = []
    handlers = {n: signal.signal(n, lambda number, frame: (_ for _ in ()).throw(RuntimeError('signal ' + str(number))))
                for n in (signal.SIGINT, signal.SIGTERM)}
    try:
        root, compiler, go = [H.canonical(Path(v)) for v in (args.root, args.compiler, args.go)]
        H.need(root == REPO and root.is_dir(), 'run this checkout control')
        read = lambda path: H.frozen_read(path, pins)
        sources = {name: read(root / name) for name in FILES}
        read(Path(__file__).resolve()); read(compiler); read(go)
        H.need(shutil.disk_usage(out).free >= 2 << 30, 'minimum2GiB free')
        work = Path(tempfile.mkdtemp(prefix='.editable-helper-owned-', dir=out))
        owner = (work.stat().st_dev, work.stat().st_ino)
        env = {k: os.environ[k] for k in ('PATH', 'HOME', 'SYSTEMROOT') if k in os.environ}
        env.update(PYTHONDONTWRITEBYTECODE='1', TMPDIR=str(work))
        runner = H.OwnedCommands(out, work, root, pins, env); runner.started = started
        cases = {}
        for name in ('original', 'helper'):
            case = out / name; case.mkdir(); checkout = case / 'source'; checkout.mkdir()
            for path, raw in sources.items():
                target = checkout / path; target.parent.mkdir(parents=True, exist_ok=True); target.write_bytes(raw)
            if name == 'helper':
                original = sources[BASE + 'after.gooo'].decode('utf-8')
                H.need(original.count(PREDICATE) == 1, 'one original predicate to refactor')
                (checkout / (BASE + 'after.gooo')).write_text(
                    original.replace(PREDICATE, '!IsDeployBot(base.effective_principal)') + HELPERS, encoding='utf-8')
            for path in FILES: read(checkout / path)
            evidence = case / 'evidence'
            argv = [sys.executable, '-I', '-B', checkout / (BASE + 'run.py'), '--root', checkout,
                    '--compiler', compiler, '--go', go, '--compiler-profile', 'source-inspection036',
                    '--out', evidence, '--check-demo']
            try: runner.call(name + '-consumer', argv, 140)
            except BaseException as error:
                # Keep the bounded process observation and the child's first
                # diagnostic together, without disguising a child failure as
                # an accepted observation.
                child_summary = evidence / 'summary.json'
                detail = load(child_summary).get('failure') if child_summary.is_file() else None
                raise RuntimeError(str(error) + '; consumer first failure: ' + str(detail)) from error
            summary = load(evidence / 'summary.json'); complete(summary); summaries.append(summary)
            cases[name] = (checkout, evidence, summary)
            inspected.append({variant: prepared(evidence, variant)[2] for variant in ('before', 'after')})
            H.need(H.owned_bytes(out) - H.owned_bytes(work) <= 64 << 20, 'retained64MiB bound')
        original, helper = cases['original'], cases['helper']
        H.need(H.equal(original[2]['comparisons'], helper[2]['comparisons']),
               'same original three inputs and fifteen-field decisions after helper extraction')
        for variant in ('before', 'after'):
            old_names, old_sites, _ = prepared(original[1], variant)
            names, sites, _ = prepared(helper[1], variant)
            H.need(len(old_names) == 3 and old_sites == 2 and ENTRY in old_names, 'original three-activity closure')
            wanted = old_names | {USED} if variant == 'after' else old_names
            H.need(names == wanted and sites == (3 if variant == 'after' else 2) and UNUSED not in names,
                   'reachable helper selected; unused declaration excluded from prepared call closure')
            for operation in ('execute', 'replay'):
                before = load(original[1] / (variant + '-' + operation + '/stdout'))
                after = load(helper[1] / (variant + '-' + operation + '/stdout'))
                # Source-dependent local/read/frame observations may change in a
                # refactor. Compare only ordered typed facts/results across the
                # two programs; each full source program is bound to its own
                # preparation and official replay by the consumer above.
                values = lambda receipt: [{k: row[k] for k in ('case_index', 'deliveries')}
                                          for row in receipt['result']['runtime']['traces']]
                H.need(before['inputs_digest'] == after['inputs_digest'] and
                       H.equal(values(before), values(after)),
                       'actual native typed input echoes and output fields equal across refactor')
        default_out = out / 'default-helper-refusal'
        runner.call('default-helper-consumer', [sys.executable, '-I', '-B', helper[0] / (BASE + 'run.py'),
                    '--root', helper[0], '--compiler', compiler, '--go', go, '--out', default_out], 10, allowed_exit=(1,))
        denied = load(default_out / 'summary.json')
        H.need(denied['status'] == 'FIRST_FAILURE_PRESERVED' and denied['failure'] == 'one activity per selected source' and
               denied['compiler_profile'] == 'default' and H.equal(denied['package_commands'], 0) and
               H.equal(denied['positive_receipts'], 0) and denied['commands'] == [] and denied['processes_closed'] is True,
               'default profile refuses helper before launching any compiler or native command')
        refused = True; H.unchanged(pins); source_ok = True
    except BaseException as error: failure = str(error)
    finally:
        try: H.unchanged(pins)
        except BaseException as error: failure = failure or str(error); source_ok = False
        if work is not None and (runner is None or runner.closed):
            try:
                H.need(not work.is_symlink() and (work.stat().st_dev, work.stat().st_ino) == owner, 'owned inode before cleanup')
                shutil.rmtree(work); cleaned = True
            except BaseException as error: failure = failure or 'owned cleanup: ' + str(error)
        for number, handler in handlers.items(): signal.signal(number, handler)
    summary = {'schema': 'gooo/editable-api-policy-helpers-CI/v1',
        'status': 'HELPER_EXTRACTION_AND_OFFICIAL_REPLAY_PASS' if failure is None else 'FIRST_FAILURE_PRESERVED',
        'failure': failure, 'variant_runs': len(summaries), 'rows': 3 if failure is None else None,
        'typed_fields_compared': 90 if failure is None else None,
        'default_helper_refused': refused, 'default_package_commands': 0 if refused else None,
        'prepared_inspections': sum(s['prepared_inspections'] for s in summaries),
        'positive_receipts': sum(s['positive_receipts'] for s in summaries),
        'compiler_revision': REV if len(summaries) == 2 else None, 'model_calls': 0 if len(summaries) == 2 else None,
        'source_tool_unchanged': source_ok, 'processes_closed': runner.closed if runner else True,
        'owned_work_removed': cleaned, 'commands': runner.observations if runner else [],
        'inspection_sha256': inspected, 'source_tool_pins': pins, 'elapsed_s': time.monotonic() - started,
        'scope': 'Two synthetic three-row consumer runs; same-file helper and unreachable declaration control. '
                 'Fifteen typed decision fields compared for before/after, plus actual execute/replay traces. '
                 'Compiler inspection is trusted preparation, not an independent semantic oracle. '
                 'No full provenance coverage, live authorization, external adoption or performance claim.'}
    H.save(out / 'summary.json', summary)
    print(H.go_json({'status': summary['status'], 'failure': failure, 'summary': str(out / 'summary.json')}).decode())
    return 0 if failure is None else 1


if __name__ == '__main__': sys.exit(main())
