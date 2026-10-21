#!/usr/bin/env python3
# Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
"""Read-only check of saved source-catalog comparison records; no execution."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import sys
HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
ENTRY = 'app/tool-catalog-change:CompareCatalogEntry'
SOURCES = {'go/openclaw/gogcli/mcp': 'ports/gogcli/tool_selection.gooo', 'app/tool-catalog-change': 'examples/tool-catalog-change/main.gooo'}
RESULT_TYPES = {'name': str, 'before_present': bool, 'after_present': bool, 'before_selected': bool, 'after_selected': bool, 'added': bool, 'removed': bool, 'classification': str, 'before_service': str, 'before_risk': str, 'before_capability': str, 'after_service': str, 'after_risk': str, 'after_capability': str}

class Refused(ValueError):
    pass

def need(ok, message):
    if not ok:
        raise Refused(message)

def sha(path):
    return 'sha256:' + hashlib.sha256(path.read_bytes()).hexdigest()

def load(path):
    need(path.is_file() and (not path.is_symlink()), 'not a regular saved file: ' + str(path))
    need(path.stat().st_size <= 33554432, 'saved file exceeds receipt cap')

    def unique(rows):
        result = {}
        for (key, value) in rows:
            need(key not in result, 'duplicate JSON key: ' + key)
            result[key] = value
        return result
    return json.loads(path.read_bytes(), object_pairs_hook=unique, parse_constant=lambda _: (_ for _ in ()).throw(Refused('nonfinite JSON')))

def same(actual, expected, label):
    need(type(actual) is type(expected), label + ': wrong type')
    if type(expected) is dict:
        need(set(actual) == set(expected), label + ': missing or extra fields')
        for (key, value) in expected.items():
            same(actual[key], value, label + '.' + key)
    elif type(expected) is list:
        need(len(actual) == len(expected), label + ': wrong length')
        for (index, value) in enumerate(expected):
            same(actual[index], value, label + '[' + str(index) + ']')
    else:
        need(actual == expected, label + ': value mismatch')

def source_digest(path):
    encoded = json.dumps(path.read_bytes().decode('utf-8'), ensure_ascii=False, separators=(',', ':'))
    for (char, escaped) in [('<', '\\u003c'), ('>', '\\u003e'), ('&', '\\u0026'), ('\u2028', '\\u2028'), ('\u2029', '\\u2029')]:
        encoded = encoded.replace(char, escaped)
    return 'sha256:' + hashlib.sha256(encoded.encode()).hexdigest()

def validate_identity(identity_path):
    profile = load(ROOT / 'upstream.json')
    need(profile.get('schema') == 'gooo/go-ports-upstream/v1', 'unsupported root pin schema')
    pin = profile.get('compiler_revision')
    go = profile.get('go_version')
    need(isinstance(pin, str) and re.fullmatch('[0-9a-f]{40}', pin), 'invalid root compiler revision')
    need(isinstance(go, str) and re.fullmatch('go1\\.[0-9]+\\.[0-9]+', go), 'invalid root Go version')
    need(re.findall(b'^go ([^\\r\\n]+)$', (ROOT / 'go.mod').read_bytes(), re.M) == [go[2:].encode()], 'root module and toolchain pins differ')
    identity = load(identity_path)
    need(identity.get('compiler_source_sha') == pin and identity.get('source_status') == 'CLEAN_VCS' and (identity.get('vcs_modified') == 'false'), 'compiler identity differs')
    need(identity.get('version') == profile.get('compiler_version'), 'compiler version differs')
    need(identity.get('go_version') == identity.get('native_go_required') == go, 'compiler Go profile differs')
    return profile

def input_generation(inputs_root):
    proof = load(inputs_root / 'generation.provenance.jsonld')
    same(proof.get('@context'), {'prov': 'http://www.w3.org/ns/prov#',
         'xsd': 'http://www.w3.org/2001/XMLSchema#',
         '@vocab': 'urn:gooo:tool-catalog-inputs:'}, 'input generation JSON-LD context')
    need(proof.get('family') == 'gooo/tool-catalog-input-projection/v1'
         and proof.get('outputs') == 4 and proof.get('source_modified_by_producer') is False,
         'input generation scope differs')
    graph = proof['@graph']
    need(len(graph) == 8, 'input generation graph inventory differs')
    identifiers = [row['@id'] for row in graph]
    need(len(set(identifiers)) == len(identifiers), 'duplicate provenance occurrence ID')
    activities = [row for row in graph if row['@type'] == 'prov:Activity']
    need(len(activities) == 1, 'input generation activity differs')
    activity = activities[0]
    sources = [row for row in graph if row['@type'] == 'prov:Entity' and 'prov:wasGeneratedBy' not in row]
    outputs = [row for row in graph if row['@type'] == 'prov:Entity' and 'prov:wasGeneratedBy' in row]
    expected_sources = {'examples/tool-catalog-change/case-facts.json',
                        'tool-catalog-change.workspace.json',
                        'examples/tool-catalog-change/make-inputs.py'}
    need({row['path'] for row in sources} == expected_sources and len(sources) == 3,
         'input generation source inventory differs')
    same(activity['prov:used'], [{'@id': row['@id']} for row in sources], 'input generation used references')
    for row in sources:
        path = ROOT / row['path']
        need(row['sha256'] == sha(path)[7:] and row['bytes'] == path.stat().st_size,
             'input generation source bytes differ')
    expected_outputs = {profile + suffix for profile in ['catalog', 'controls']
                        for suffix in ['-inputs.json', '-oracle-inputs.json']}
    need({row['path'] for row in outputs} == expected_outputs and len(outputs) == 4,
         'input generation output inventory differs')
    for row in outputs:
        same(row['prov:wasGeneratedBy'], {'@id': activity['@id']}, 'input generation activity reference')
        same(row['prov:wasDerivedFrom'], activity['prov:used'], 'input generation derivation references')
        path = inputs_root / row['path']
        need(row['sha256'] == sha(path)[7:] and row['bytes'] == path.stat().st_size,
             'generated input bytes differ')

def check(execution_path, replay_path, expected_path, identity_path, catalog_path, inputs_root, mode='catalog'):
    profile = validate_identity(identity_path)
    input_generation(inputs_root)
    input_name = mode + '-inputs.json'
    oracle_name = mode + '-oracle-inputs.json'
    inputs = load(inputs_root / input_name)
    facts = [row[ENTRY] for row in inputs['inputs']]
    oracle_facts = load(inputs_root / oracle_name)
    same(facts, oracle_facts, 'oracle input facts')
    same(facts, load(HERE / 'case-facts.json')[mode], 'canonical input facts')
    catalogs = load(HERE / 'source-catalogs.json')
    same(load(catalog_path), catalogs, 'fresh source-catalog projection')
    need(catalogs.get('unresolved_reachable_factory_entries') == 0, 'source catalog unresolved')
    before = catalogs['before_metadata']
    after = catalogs['after_metadata']
    need(len(before) == 10 and len(after) == 27, 'this fixed source profile requires10/27 catalog entries')
    names = [row['name'] for row in after] + [row['name'] for row in before if row['name'] not in {x['name'] for x in after}]
    if mode == 'catalog':
        same([row['name'] for row in facts], names, 'catalog fact order')
    for (side, catalog) in [('before', before), ('after', after)] if mode == 'catalog' else []:
        lookup = {row['name']: row for row in catalog}
        need(len(lookup) == len(catalog), 'duplicate source tool name')
        for row in facts:
            name = row['name']
            same(row[side + '_present'], name in lookup, 'catalog presence')
            for key in ['service', 'risk', 'capability']:
                same(row[side + '_' + key], lookup[name][key] if name in lookup else '', 'catalog ' + key)
            for (key, value) in dict(allow_write=True, allow_gmail_send=False, allow_gmail_delete=False, readonly=False, policy_selectors=['gmail.*'], runtime_selectors=[]).items():
                same(row[side + '_' + key], value, 'fixed policy ' + key)
    for binding in load(HERE / 'source-pin.json')['files']:
        path = ROOT / binding['local_path']
        need(path.resolve().is_relative_to(ROOT.resolve()), 'source pin path escapes root')
        raw = path.read_bytes()
        need(len(raw) == binding['bytes'] and hashlib.sha256(raw).hexdigest() == binding['raw_sha256'], 'raw upstream source mismatch')
        need(hashlib.sha1(b'blob ' + str(len(raw)).encode() + b'\x00' + raw).hexdigest() == binding['blob_sha1'], 'upstream Git blob mismatch')
    count = 27 if mode == 'catalog' else 20
    expected = load(expected_path)
    need(type(expected) is list and len(expected) == len(facts) == count, 'expected-output count differs')
    for row in expected:
        need(type(row) is dict and set(row) == set(RESULT_TYPES), 'Go reference result fields differ')
        for (key, typ) in RESULT_TYPES.items():
            need(type(row[key]) is typ, 'Go reference result type differs: ' + key)
    execution = load(execution_path)
    replay = load(replay_path)
    need(replay.get('replayed_from_sha256') == sha(execution_path), 'replay parent differs')
    for (label, receipt) in [('execution', execution), ('replay', replay)]:
        need(receipt.get('decision') == 'OBSERVED', label + ': native observation missing')
        need(receipt.get('manifest_digest') == sha(ROOT / 'tool-catalog-change.workspace.json'), label + ': manifest differs')
        need(receipt.get('inputs_digest') == sha(inputs_root / input_name), label + ': inputs differ')
        image = receipt['result']['program']['workspace_image']
        same(image['entry'], {'package_path': 'app/tool-catalog-change', 'source': 'examples/tool-catalog-change/main.gooo', 'activity': 'CompareCatalogEntry', 'inputs': ['CatalogChange'], 'output': 'CatalogDelta'}, label + ' entry')
        need(len(image['packages']) == 2, label + ': unexpected package count')
        packages = {package['path']: package for package in image['packages']}
        need(set(packages) == set(SOURCES), label + ': package paths differ')
        for (path, name) in SOURCES.items():
            rows = packages[path]['sources']
            need(len(rows) == 1, label + ': source closure differs')
            need(rows[0]['filename'] == name and rows[0]['source_digest'] == source_digest(ROOT / name), label + ': source bytes differ')
        runtime = receipt['result']['runtime']
        need(runtime.get('producer_source_sha') == profile['compiler_revision'], label + ': producer differs')
        version = runtime.get('go_version', '').split()
        need(len(version) == 4 and version[:3] == ['go', 'version', profile['go_version']], label + ': Go profile differs')
        need(runtime.get('model_calls') == 0, label + ': model call observed')
        traces = runtime['traces']
        need(len(traces) == count, label + ': native result count differs')
        for (index, trace) in enumerate(traces):
            need(trace['case_index'] == index and len(trace['deliveries']) == 1, label + ': trace identity differs')
            delivery = trace['deliveries'][0]
            need(delivery['activity_id'] == 'gooo://tool-catalog-change/compare-entry/v1', label + ': activity differs')
            same(delivery['input'], facts[index], label + ' input' + str(index))
            same(delivery['actual'], expected[index], label + ' result' + str(index))
    selected_before = [row['name'] for row in expected if row['before_selected']]
    selected_after = [row['name'] for row in expected if row['after_selected']]
    added = [row['name'] for row in expected if row['added']]
    if mode == 'catalog':
        need(len(selected_before) == 3 and len(selected_after) == 16 and (len(added) == 13), 'reviewed catalog result differs')
    return {'status': 'FINITE_SOURCE_CATALOG_MATCHED' if mode == 'catalog' else 'FINITE_SELECTION_CONTROLS_MATCHED', 'profile': mode, 'source_catalogs': {'before': 10, 'after': 27} if mode == 'catalog' else None, 'selected': {'before': len(selected_before), 'after': len(selected_after)}, 'added': added, 'native_cases': count, 'replay_cases': count, 'typed_fields_per_layer': count * 14, 'compiler_source_sha': profile['compiler_revision'], 'execution_sha256': sha(execution_path), 'replay_sha256': sha(replay_path), 'scope': 'Unsigned consistency check of these finite source-catalog observations or synthetic controls and the separate Go reference output; not live registration, authentication or action enforcement.'}

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--identity', type=Path, required=True)
    parser.add_argument('--identity-only', action='store_true')
    parser.add_argument('--profile', choices=['catalog', 'controls'], default='catalog')
    for name in ['execution', 'replay', 'expected', 'catalog', 'generated-inputs']:
        parser.add_argument('--' + name, type=Path)
    args = parser.parse_args()
    if args.identity_only:
        need(all((getattr(args, k) is None for k in ['execution', 'replay', 'expected', 'catalog', 'generated_inputs'])), 'identity-only cannot evaluate saved records')
        profile = validate_identity(args.identity)
        print(json.dumps({'status': 'PINNED_COMPILER_IDENTITY_MATCHES', 'compiler_revision': profile['compiler_revision'], 'go_version': profile['go_version']}))
        return
    need(all((getattr(args, k) is not None for k in ['execution', 'replay', 'expected', 'catalog', 'generated_inputs'])), 'execution/replay/expected/catalog/generated-inputs paths required')
    print(json.dumps(check(args.execution, args.replay, args.expected, args.identity, args.catalog, args.generated_inputs, args.profile), indent=2))
if __name__ == '__main__':
    try:
        main()
    except (Refused, OSError, ValueError, KeyError, TypeError) as error:
        sys.exit('Refused: ' + str(error))
