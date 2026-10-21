#!/usr/bin/env python3
# Copyright (c) 2026 Gooo Go Ports contributors. All rights reserved.
# Use of this source code is governed by a BSD-style license in LICENSE.
"""Check two current reproduction guides or generate detached doc candidates.

This is a bounded documentation consumer, not dependency migration authority.
It never imports the generators, runs Gooo, edits source, or changes old receipts.
"""
import argparse
import ast
import datetime
import hashlib
import json
from pathlib import Path
import re
import sys
import uuid
from urllib.parse import quote

ROOT = Path(__file__).resolve().parents[1]
MODULES = ('agentmesh-service-policy', 'service-grant-many')
START = b'<!-- gooo-current-toolchain:start -->\n'
END = b'<!-- gooo-current-toolchain:end -->'
FAMILY = 'gooo/current-generation-docs/v1'


class Refused(ValueError):
    pass


def need(condition, message):
    if not condition:
        raise Refused(message)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def strict(raw):
    def unique(rows):
        result = {}
        for key, value in rows:
            need(key not in result, 'duplicate JSON key')
            result[key] = value
        return result
    return json.loads(raw, object_pairs_hook=unique,
                      parse_constant=lambda _: (_ for _ in ()).throw(Refused('nonfinite JSON')))


def read(root, relative, inputs):
    path = root / relative
    need(not path.is_symlink() and path.is_file(), 'missing or linked input: ' + relative)
    need(path.resolve().is_relative_to(root.resolve()), 'input escapes source root')
    raw = path.read_bytes()
    need(len(raw) <= 256 * 1024, 'input exceeds documentation cap: ' + relative)
    inputs[relative] = raw
    return raw


def literal(tree, name):
    values = [node.value for node in tree.body if isinstance(node, ast.Assign)
              and any(isinstance(t, ast.Name) and t.id == name for t in node.targets)]
    need(len(values) == 1, 'missing or ambiguous literal: ' + name)
    try:
        return ast.literal_eval(values[0])
    except (ValueError, TypeError, SyntaxError) as error:
        raise Refused('nonliteral fact: ' + name) from error


def facts(root, module, inputs):
    prefix = 'examples/' + module + '/'
    identity = strict(read(root, prefix + 'compiler.json', inputs))
    generation = strict(read(root, prefix + 'generation.json', inputs))
    need(type(identity) is dict and type(generation) is dict, 'identity and generation must be objects')
    need(type(generation.get('assets')) is dict, 'generation assets must be an object')
    revision, go, version = (identity.get(k) for k in ('compiler_source_sha', 'go_version', 'version'))
    need(isinstance(revision, str) and re.fullmatch('[0-9a-f]{40}', revision), 'invalid compiler revision')
    need(isinstance(go, str) and re.fullmatch(r'go1\.[0-9]+\.[0-9]+', go), 'invalid Go version')
    need(isinstance(version, str) and re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+(?:[-+][A-Za-z0-9.-]+)?', version), 'invalid compiler version')
    need(identity.get('source_status') == 'CLEAN_VCS' and identity.get('vcs_modified') == 'false', 'compiler source is not clean')
    need(identity.get('native_go_required') == go, 'compiler runtime Go mismatch')
    need(generation.get('origin') == 'fresh_pinned_generation', 'generation origin mismatch')
    if module == 'agentmesh-service-policy':
        need(generation.get('schema') == 'gooo/agentmesh-native-generation/v1'
             and set(generation) == {'assets', 'origin', 'public_sources', 'schema'},
             'unsupported Agent Mesh generation shape')
        # This schema binds compiler.json through assets, not a top-level PIN.
    else:
        need(generation.get('schema') == 'gooo/service-grant-many-generation/v1'
             and generation.get('compiler_source_sha') == revision
             and generation.get('go_version') == go, 'generation compiler/Go mismatch')
    for filename in ('compiler.json', 'go.mod', 'generate.py', 'verify.py'):
        raw = read(root, prefix + filename, inputs)
        need(generation.get('assets', {}).get(filename) == 'sha256:' + sha(raw), 'generation asset mismatch: ' + prefix + filename)
    go_lines = re.findall(rb'^go ([^\r\n]+)$', inputs[prefix + 'go.mod'], re.M)
    need(go_lines == [go[2:].encode()], 'module Go mismatch')
    for filename in ('generate.py', 'verify.py'):
        tree = ast.parse(inputs[prefix + filename], filename=prefix + filename)
        if filename == 'verify.py' and module == 'service-grant-many':
            profile = literal(tree, 'ACTIVE_COMPILER_PROFILE')
            need(isinstance(profile, dict) and profile.get('compiler_revision') == revision
                 and profile.get('compiler_version') == version and profile.get('go_version') == go,
                 'active verifier profile mismatch')
        else:
            need(literal(tree, 'PIN') == revision, 'generator/verifier compiler mismatch')
        go_literals = {node.value for node in ast.walk(tree) if isinstance(node, ast.Constant)
                       and isinstance(node.value, str) and re.fullmatch(r'go1\.[0-9]+\.[0-9]+', node.value)}
        need(go_literals == {go}, 'missing or conflicting active Go literals')
    return revision, go, version


def render(module, revision, go, version):
    common = (f'Use an already-installed clean Gooo **{version}** compiler at\n'
              f'`{revision}` and **Go {go[2:]}**. These current\n'
              'identities come from [compiler.json](compiler.json), are bound by\n'
              '[generation.json](generation.json), and agree with the generator and\n'
              'verifier profiles. Setup is separate; verification does not install a\n'
              'compiler or download a toolchain.\n')
    if module == 'agentmesh-service-policy':
        return (common + '\nRun these commands from this example directory:\n').encode()
    return (common + '\nFrom this example directory, choose a fresh output directory:\n\n'
            '```sh\npython3 -I -B verify.py --compiler /absolute/path/to/pinned-gooo \\\n'
            f'  --go /absolute/path/to/{go}/bin/go --out /absolute/new/owned/verification\n'
            '```\n').encode()


def plan(root):
    inputs, outputs = {}, {}
    for module in MODULES:
        revision, go, version = facts(root, module, inputs)
        relative = 'examples/' + module + '/README.md'
        raw = read(root, relative, inputs)
        need(raw.count(START) == raw.count(END) == 1, 'missing or duplicate doc markers: ' + relative)
        before, rest = raw.split(START)
        old, after = rest.split(END)
        outputs[relative] = before + START + render(module, revision, go, version) + END + after
    return inputs, outputs


def generate(root, out):
    started = datetime.datetime.now(datetime.timezone.utc).isoformat()
    inputs, outputs = plan(root)  # Validate every consumer before creating output.
    need(out.is_absolute() and not out.exists() and not out.is_symlink(), 'output must be a fresh absolute directory')
    need(not out.resolve().is_relative_to(root.resolve()), 'output must be outside the source tree')
    out.mkdir()
    activity_id = 'urn:uuid:' + str(uuid.uuid4())
    graph, used = [], []
    producer = Path(__file__).read_bytes()
    for name, raw in [*inputs.items(), ('producer:current-toolchain-docs.py', producer)]:
        entity_id = activity_id + ':input:' + quote(name, safe='')
        graph.append({'@id': entity_id, '@type': 'prov:Entity', 'path': name,
                      'bytes': len(raw), 'sha256': sha(raw)})
        used.append({'@id': entity_id})
    for name, raw in outputs.items():
        target = out / name
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(raw)
        graph.append({'@id': activity_id + ':output:' + quote(name, safe=''), '@type': 'prov:Entity',
                      'path': name, 'bytes': len(raw), 'sha256': sha(raw),
                      'prov:wasGeneratedBy': {'@id': activity_id},
                      'prov:wasDerivedFrom': [{'@id': activity_id + ':input:' + quote(key, safe='')}
                                             for key in inputs if key.startswith(name.rsplit('/', 1)[0] + '/')]})
    graph.append({'@id': activity_id, '@type': 'prov:Activity', 'prov:used': used,
                  'prov:startedAtTime': {'@value': started, '@type': 'xsd:dateTime'},
                  'prov:endedAtTime': {'@value': datetime.datetime.now(datetime.timezone.utc).isoformat(), '@type': 'xsd:dateTime'},
                  'rule': FAMILY, 'python_version': sys.version.split()[0]})
    proof = {'@context': {'prov': 'http://www.w3.org/ns/prov#', 'xsd': 'http://www.w3.org/2001/XMLSchema#', '@vocab': 'urn:gooo:doc-facts:'},
             '@graph': graph, 'family': FAMILY, 'consumer_count': len(outputs),
             'scope': 'Detached documentation generation only; no compiler execution, migration, source edit or deployment.',
             'outside_consumer_inventory': 'UNKNOWN', 'source_modified': False}
    (out / 'generation.provenance.jsonld').write_text(json.dumps(proof, indent=2) + '\n')
    return outputs



def check(root):
    inputs, outputs = plan(root)
    need(all(inputs[name] == raw for name, raw in outputs.items()),
         'current toolchain documentation drift; generate detached candidates with --out')
    return len(outputs)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, default=ROOT)
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument('--check', action='store_true')
    mode.add_argument('--out', type=Path)
    args = parser.parse_args()
    if args.check:
        count = check(args.root)
        print(json.dumps({'status': 'CURRENT_TOOLCHAIN_DOCS_MATCH', 'family': FAMILY, 'consumers': count, 'compiler_executions': 0}))
    else:
        outputs = generate(args.root, args.out)
        print(json.dumps({'status': 'DETACHED_DOC_CANDIDATES_GENERATED', 'family': FAMILY, 'consumers': len(outputs), 'source_modified': False}))


if __name__ == '__main__':
    try:
        main()
    except (Refused, OSError, ValueError, SyntaxError) as error:
        raise SystemExit('Refused: ' + str(error)) from error
