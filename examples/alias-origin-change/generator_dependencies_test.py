"""Exercise actual renderer dependency changes in an isolated source fixture.

No new native result is claimed: exact unchanged source/cases use the supplied
fresh native pair, and relevant changes must refuse that same old pair.
"""
import argparse
import ast
import hashlib
import importlib.util
import json
import os
import traceback
from pathlib import Path

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
RUNNER = ROOT / 'examples/service-grant-many/verify.py'
spec = importlib.util.spec_from_file_location('alias_dependency_owned', RUNNER)
host = importlib.util.module_from_spec(spec)
spec.loader.exec_module(host)


def digest(raw):
    return 'sha256:' + hashlib.sha256(raw).hexdigest()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--execution', type=Path, required=True)
    parser.add_argument('--replay', type=Path, required=True)
    parser.add_argument('--gofmt', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    out = args.out.resolve()
    assert not out.is_relative_to(ROOT.resolve())
    out.mkdir(parents=True, exist_ok=False)
    host.R = out / 'commands'
    host.R.mkdir()
    fixture = out / 'fixture'
    names = ['upstream.json', 'alias-origin-change.workspace.json',
             'ports/tool-origin/alias_origin.gooo',
             'examples/alias-origin-change/kernel.cases.json',
             'examples/alias-origin-change/schema-validation.json',
             'examples/alias-origin-change/generate.py']
    observed = [ROOT / name for name in names] + [Path(__file__), RUNNER]
    before = {str(p.relative_to(ROOT)): host.ref(p) for p in observed}
    for name in names:
        target = fixture / name
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes((ROOT / name).read_bytes())
    generator = fixture / 'examples/alias-origin-change/generate.py'
    facts_path = fixture / 'upstream.json'
    source_path = fixture / 'ports/tool-origin/alias_origin.gooo'
    cases_path = fixture / 'examples/alias-origin-change/kernel.cases.json'
    original_facts, original_source, original_cases = [p.read_bytes() for p in (facts_path, source_path, cases_path)]
    report = {'schema': 'gooo/alias-origin-consumed-pin-regression/v1',
              'status': 'FIRST_FAILURE_NO_RETRY', 'new_native_execution': False,
              'execution_authorized': False, 'model_calls': 0}
    first = None

    def render(label, expected=0):
        target = out / label
        stdout = host.owned(label, ['python3', '-I', '-B', generator,
                            '--execution', args.execution.resolve(),
                            '--replay', args.replay.resolve(),
                            '--gofmt', args.gofmt.resolve(), '--out', target],
                            dict(os.environ), 60, ROOT, expected_exit=expected)
        if expected:
            assert stdout == b'' and not target.exists()
            return None
        return target

    def assets(directory):
        return {str(p.relative_to(directory)): digest(p.read_bytes())
                for p in directory.rglob('*')
                if p.is_file() and p.name != 'generation.provenance.jsonld'}

    try:
        # Restrict every use of root FACTS to explicit literal fields. Reject a
        # future .get(), alias or dynamic read until its dependency is modeled.
        tree = ast.parse(generator.read_text())
        parents = {child: node for node in ast.walk(tree) for child in ast.iter_child_nodes(node)}
        reads = []
        for node in ast.walk(tree):
            if isinstance(node, ast.Name) and node.id == 'FACTS' and isinstance(node.ctx, ast.Load):
                parent = parents[node]
                assert isinstance(parent, ast.Subscript) and parent.value is node
                assert isinstance(parent.slice, ast.Constant) and isinstance(parent.slice.value, str)
                reads.append(parent.slice.value)
        assert sorted(reads) == ['compiler_revision', 'go_version']
        baseline = render('original-facts')
        expected_assets = assets(baseline)
        assert len(expected_assets) == 7
        changed = json.loads(original_facts)
        changed['maintenance_note'] = 'unrelated synthetic metadata; not an assertion about other ports'
        changed_raw = (json.dumps(changed, sort_keys=True, indent=4) + '\n').encode()
        facts_path.write_bytes(changed_raw)
        unrelated = render('unrelated-facts')
        assert assets(unrelated) == expected_assets
        facts_path.write_bytes(json.dumps(json.loads(original_facts), sort_keys=True, separators=(',', ':')).encode())
        formatted = render('format-only-facts')
        assert assets(formatted) == expected_assets
        proof = host.strict((unrelated / 'generation.provenance.jsonld').read_bytes())
        nodes = proof['@graph']
        projection = next(n for n in nodes if n.get('path') == 'upstream.json#consumed-generator-pins')
        whole = next(n for n in nodes if n.get('path') == 'upstream.json')
        assert whole['sha256'] == digest(changed_raw)
        assert set(host.strict(projection['canonicalJSON'])) == set(reads)
        assert projection['sha256'] == digest(projection['canonicalJSON'].encode())
        assert projection['jsonPointers'] == ['/compiler_revision', '/go_version']
        assert projection['prov:wasDerivedFrom'] == [{'@id': whole['@id']}]
        assert next(n for n in nodes if n.get('path') == str(args.gofmt.resolve().parent / 'go'))['sha256'] == digest((args.gofmt.resolve().parent / 'go').read_bytes())
        activity = next(n for n in nodes if n.get('@type') == 'prov:Activity')
        assert {'@id': whole['@id']} in activity['prov:used'] and {'@id': projection['@id']} in activity['prov:used']
        for name in expected_assets:
            entity = next(n for n in nodes if n.get('path') == name)
            assert {'@id': whole['@id']} not in entity['prov:wasDerivedFrom']
            if name != 'go.sum':
                assert {'@id': projection['@id']} in entity['prov:wasDerivedFrom']
        negative = []
        original_pins = json.loads(original_facts)
        go_parts = original_pins['go_version'].split('.')
        changed_go = '.'.join([*go_parts[:-1], str(int(go_parts[-1]) + 1)])
        for field, value in [('compiler_revision', '0' * len(original_pins['compiler_revision'])), ('go_version', changed_go)]:
            changed = json.loads(original_facts)
            changed[field] = value
            facts_path.write_text(json.dumps(changed))
            render('reject-' + field, 1)
            negative.append(field)
        facts_path.write_bytes(original_facts)
        source_path.write_bytes(original_source + b'\n// changed source needs new receipts\n')
        render('reject-source', 1)
        source_path.write_bytes(original_source)
        negative.append('source')
        cases_path.write_bytes(original_cases + b'\n')
        render('reject-cases', 1)
        cases_path.write_bytes(original_cases)
        negative.append('cases')
        after = {str(p.relative_to(ROOT)): host.ref(p) for p in observed}
        host.exact(after, before)
        report.update(status='CONSUMED_PINS_STABLE_UNRELATED_AND_REJECT_RELEVANT_CHANGES',
                      stable_assets=7, irrelevant_change_variants=2,
                      relevant_change_refusals=negative, public_source_unchanged=True,
                      static_literal_root_field_reads=reads, actual_root_PROV_retained=True)
    except BaseException as error:
        first = error
        report['first_failure'] = {'type': type(error).__name__, 'message': str(error), 'traceback': traceback.format_exc()}
    finally:
        closed = bool(host.records) and all(r['waited'] and r['streams_EOF'] and r['PG_ESRCH'] for r in host.records)
        report.update(processes=host.records, all_direct_wait_EOF_PG_ESRCH=closed)
        if not closed:
            report['status'] = 'FIRST_FAILURE_NO_RETRY'
            first = first or RuntimeError('incomplete process closure')
        host.save(out / 'summary.json', report)
    if first is not None:
        raise SystemExit(1)
    host.check_cancelled()
    print(json.dumps({'status': report['status'], 'summary': host.ref(out / 'summary.json')}))


if __name__ == '__main__':
    with host.cancellation_handlers():
        main()
