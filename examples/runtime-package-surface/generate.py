"""Reuse the single-record projection bridge for a root CI tool, outside product imports."""
import argparse
import datetime
import importlib.util
import json
import subprocess
import uuid
from pathlib import Path

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('surface_evidence', HERE/'evidence.py')
e = importlib.util.module_from_spec(spec)
spec.loader.exec_module(e)
ROOT = e.ROOT
OUTPUT = 'internal/runtimepackagesurface'
ASSETS = {OUTPUT+'/'+name for name in ('projection_generated.go', 'types_generated.go', 'bridge_generated.go', 'generation.json')}
HELPERS = [Path(__file__), HERE/'evidence.py', ROOT/'examples/tool-definition-binding/evidence.py',
           ROOT/'examples/recorded-catalog-comparison/generate.py', ROOT/'examples/service-grant-many/verify.py']
REFERENCES = [HERE/'abi.json', HERE/'cases.json', HERE/'upstream.json', ROOT/'ports/runtime-package-surface/LICENSE']
PINS = json.dumps({'compiler_revision': e.PIN, 'go_version': e.GO_VERSION}, sort_keys=True, separators=(',', ':')).encode()
PROFILE = 'gooo/runtime-package-surface-file-occurrence/v1'


def inputs(index, gofmt, used):
    paths = [ROOT/'upstream.json', e.MANIFEST, *(ROOT/name for name in e.SOURCES),
             *REFERENCES, *HELPERS, index, gofmt, gofmt.parent/'go', *used]
    assert len(set(paths)) == len(paths)
    return paths


def render(index, gofmt):
    facts = e.host.strict(e.read(ROOT/'upstream.json'))
    selected = {key: facts[key] for key in ('compiler_revision', 'go_version')}
    assert json.dumps(selected, sort_keys=True, separators=(',', ':')).encode() == PINS
    assert gofmt.is_absolute() and gofmt.name == 'gofmt' and gofmt.is_file()
    go = gofmt.parent/'go'
    version = subprocess.run([str(go), 'version'], check=True, capture_output=True, timeout=15).stdout.decode().strip()
    assert version in ('go version '+e.GO_VERSION+' darwin/arm64', 'go version '+e.GO_VERSION+' linux/amd64')
    composition, used = e.verify_index(index, dict(e.host.ref(go), version=version))
    records, activity = e.semantic_records(composition['plan'])
    raw = composition['source']
    assert raw.startswith('package main\n') and 'import ' not in raw
    layouts = [('Observation', records[e.PREFIX+'observation']), ('Assessment', records[e.PREFIX+'assessment'])]
    primitive = {'urn:gooo:type:string': 'string', 'urn:gooo:type:integer': 'int64', 'urn:gooo:type:boolean': 'bool'}
    types = '// Code generated from the source-bound Gooo typed plan; DO NOT EDIT.\npackage runtimepackagesurface\n'
    for name, record in layouts:
        types += '\ntype '+name+' struct {\n'
        for field in record['fields']:
            kind = ('[]' if field.get('cardinality', 'one') == 'many' else '')+primitive[field['type_id']]
            types += '\t'+e.base.go_name(field['name'])+' '+kind+' `json:"'+field['name']+'"`\n'
        types += '}\n'
    source = layouts[0][1]
    bridge = '// Code generated from compiler-owned record/field identities; DO NOT EDIT.\npackage runtimepackagesurface\n\nfunc Evaluate(input Observation) Assessment {\n\tnative := '+source['go_name']+'{\n'
    for field in source['fields']:
        bridge += '\t\t'+field['go_name']+': input.'+e.base.go_name(field['name'])+',\n'
    bridge += '\t}\n\tactual := '+activity['go_function']+'(native)\n\treturn Assessment{\n'
    for field in layouts[1][1]['fields']:
        bridge += '\t\t'+e.base.go_name(field['name'])+': actual.'+field['go_name']+',\n'
    bridge += '\t}\n}\n'
    generated = {OUTPUT+'/projection_generated.go': raw.replace('package main\n', 'package runtimepackagesurface\n', 1).encode()}
    for name, value in [('types_generated.go', types), ('bridge_generated.go', bridge)]:
        result = subprocess.run([str(gofmt)], input=value.encode(), capture_output=True, check=True, timeout=15)
        assert not result.stderr
        generated[OUTPUT+'/'+name] = result.stdout
    metadata = {
        'schema': 'gooo/runtime-package-surface-generation/v1',
        'compiler_source_sha': e.PIN, 'go_version': e.GO_VERSION, 'entry': e.ENTRY,
        'activity_id': e.PREFIX+'inspect', 'cases': 14,
        'source': {name: e.sha(e.read(ROOT/name)) for name in e.SOURCES},
        'workspace_sha256': e.sha(e.read(e.MANIFEST)),
        'reference_source': {str(p.relative_to(ROOT)): e.sha(e.read(p)) for p in REFERENCES},
        'generator_source': {str(p.relative_to(ROOT)): e.sha(e.read(p)) for p in HELPERS},
        'selected_upstream_pins_sha256': e.sha(PINS),
        'typed_plan_sha256': composition['plan']['typed_plan_sha256'],
        'raw_projection_sha256': e.sha(raw.encode()),
        'projection_transform': 'Replace first package token main with runtimepackagesurface; all other bytes unchanged.',
        'generated_assets': {name: e.sha(value) for name, value in generated.items()},
        'scope': 'Frozen fourteen synthetic package observations and own replay. Root CI checker only; matching paths do not authenticate a producer, prove behavior or authorize updates.'}
    generated[OUTPUT+'/generation.json'] = (json.dumps(metadata, sort_keys=True, indent=2)+'\n').encode()
    assert set(generated) == ASSETS
    return generated, used


def provenance(index, gofmt, generated, used):
    activity = 'urn:uuid:'+str(uuid.uuid4())
    def identity(role, path, digest):
        raw = json.dumps([PROFILE, activity, role, path, digest], ensure_ascii=True, separators=(',', ':')).encode()
        return 'urn:gooo:runtime-package-surface:entity:'+e.sha(raw).removeprefix('sha256:')
    value = e.host.strict(e.read(index))
    receipts = {e.shared.local_member(index.parent, value[key]) for key in ('execution', 'replay')}
    graph, parents = [], []
    for path in inputs(index, gofmt, used):
        raw = e.read(path, 64<<20)
        name = str(path.relative_to(ROOT)) if path.is_relative_to(ROOT) else str(path)
        digest = e.sha(raw)
        node = {'@id': identity('input', name, digest), '@type': 'prov:Entity', 'path': name, 'role': 'input', 'sha256': digest}
        if path in receipts:
            node['original_JSON_sha256'] = e.sha(e.receipt(path)[0])
        graph.append(node)
        parents.append({'@id': node['@id']})
    name = 'upstream.json#consumed-runtime-package-surface-pins'
    selected = {'@id': identity('selected-pins', name, e.sha(PINS))}
    graph.append({'@id': selected['@id'], '@type': 'prov:Entity', 'path': name, 'role': 'selected-pins', 'sha256': e.sha(PINS), 'canonicalJSON': PINS.decode(), 'jsonPointers': ['/compiler_revision', '/go_version'], 'prov:wasDerivedFrom': [parents[0]]})
    graph.append({'@id': activity, '@type': 'prov:Activity', 'prov:used': parents+[selected], 'prov:endedAtTime': {'@value': datetime.datetime.now(datetime.timezone.utc).isoformat(), '@type': 'http://www.w3.org/2001/XMLSchema#dateTime'}})
    for name, raw in generated.items():
        graph.append({'@id': identity('output', name, e.sha(raw)), '@type': 'prov:Entity', 'path': name, 'role': 'output', 'sha256': e.sha(raw), 'prov:wasGeneratedBy': {'@id': activity}, 'prov:wasDerivedFrom': [selected, *parents[1:]]})
    return {'@context': {'prov': 'http://www.w3.org/ns/prov#', 'gooo': 'https://github.com/kimjooyoon/gooo-go-ports/provenance#', 'path': 'gooo:path', 'role': 'gooo:role', 'sha256': 'gooo:sha256', 'original_JSON_sha256': 'gooo:originalJSONSHA256', 'canonicalJSON': 'gooo:canonicalJSON', 'jsonPointers': 'gooo:jsonPointers'}, 'entity_identity_profile': PROFILE, '@graph': graph, 'scope': 'Actual local generation inputs and generation-scoped file occurrences. No producer authentication or effect authority.'}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--index', type=Path, required=True)
    parser.add_argument('--gofmt', type=Path, required=True)
    parser.add_argument('--out', type=Path)
    parser.add_argument('--check', action='store_true')
    args = parser.parse_args()
    assert args.index.is_absolute() and args.gofmt.is_absolute() and args.check != (args.out is not None)
    generated, used = render(args.index, args.gofmt)
    if args.check:
        for name, raw in generated.items():
            assert e.read(ROOT/name, 4<<20) == raw, name
    else:
        assert args.out.is_absolute() and not args.out.exists() and not args.out.resolve().is_relative_to(ROOT.resolve())
        args.out.mkdir()
        for name, raw in generated.items():
            path = args.out/name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(raw)
        (args.out/'generation.provenance.jsonld').write_text(json.dumps(provenance(args.index, args.gofmt, generated, used), indent=2)+'\n')
    print(json.dumps({'status': 'EXACT_GENERATED_BYTES_CHECKED' if args.check else 'GENERATED_FROM14_NATIVE_AND_OWN_REPLAY_CASES', 'files': sorted(generated)}))


if __name__ == '__main__':
    main()
