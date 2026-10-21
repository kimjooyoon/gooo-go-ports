#!/usr/bin/env python3
# Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause.
"""Serialize the declared finite facts for Gooo and Go with actual lineage."""
import argparse
import datetime
import hashlib
import json
from pathlib import Path
import uuid

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
FIELDS = {'surface_id': str, 'rule_id': str, 'inventory_complete': bool,
          'backend_known': bool, 'backend_can_enforce': bool,
          'prepolicy_matches': int, 'remaining_matches': int,
          'earlier_denied_matches': int, 'construction_denied_matches': int}


def encode(value):
    return (json.dumps(value, ensure_ascii=False, indent=2) + '\n').encode()


def generate(out):
    start = datetime.datetime.now(datetime.timezone.utc).isoformat()
    names = ['examples/deny-rule-scope/case-facts.json', 'deny-rule-scope.workspace.json',
             'examples/deny-rule-scope/make-inputs.py']
    originals = {name: (ROOT / name).read_bytes() for name in names}
    facts = json.loads(originals[names[0]])
    assert type(facts) is list and len(facts) == 87
    for row in facts:
        assert type(row) is dict and row.keys() == FIELDS.keys()
        for name, typ in FIELDS.items():
            assert type(row[name]) is typ
            if typ is str:
                assert len(row[name].encode()) <= 1024
            if typ is int:
                assert -(1 << 63) <= row[name] < 1 << 63
    manifest = json.loads(originals[names[1]])
    assert manifest['schema'] == 'gooo/package-workspace-manifest/v1'
    entry = manifest['entry']['package_path'] + ':' + manifest['entry']['activity']
    assert entry == 'app/deny-rule-scope:DiagnoseDenyRule'
    assert out.is_absolute() and not out.exists() and not out.is_symlink()
    assert not out.resolve().is_relative_to(ROOT.resolve())
    out.mkdir(mode=0o700)
    outputs = {'inputs.json': encode({'schema': 'gooo/body-composition-inputs/v1',
                                      'inputs': [{entry: row} for row in facts]}),
               'oracle-inputs.json': encode(facts)}
    activity = 'urn:uuid:' + str(uuid.uuid4())
    graph = []
    for name, raw in originals.items():
        graph.append({'@id': 'urn:uuid:' + str(uuid.uuid4()), '@type': 'prov:Entity',
                      'path': name, 'bytes': len(raw), 'sha256': hashlib.sha256(raw).hexdigest()})
    used = [{'@id': row['@id']} for row in graph]
    for name, raw in outputs.items():
        (out / name).write_bytes(raw)
        graph.append({'@id': 'urn:uuid:' + str(uuid.uuid4()), '@type': 'prov:Entity',
                      'path': name, 'bytes': len(raw), 'sha256': hashlib.sha256(raw).hexdigest(),
                      'prov:wasGeneratedBy': {'@id': activity}, 'prov:wasDerivedFrom': used})
    graph.append({'@id': activity, '@type': 'prov:Activity', 'prov:used': used,
                  'prov:startedAtTime': {'@value': start, '@type': 'xsd:dateTime'},
                  'prov:endedAtTime': {'@value': datetime.datetime.now(datetime.timezone.utc).isoformat(),
                                       '@type': 'xsd:dateTime'}})
    proof = {'@context': {'prov': 'http://www.w3.org/ns/prov#', 'xsd': 'http://www.w3.org/2001/XMLSchema#',
                          '@vocab': 'urn:gooo:deny-rule-inputs:'},
             '@graph': graph, 'family': 'gooo/deny-rule-input-projection/v1'}
    (out / 'generation.provenance.jsonld').write_bytes(encode(proof))
    print(json.dumps({'cases': len(facts), 'outputs': len(outputs), 'model_calls': 0}))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, required=True)
    generate(parser.parse_args().out)
