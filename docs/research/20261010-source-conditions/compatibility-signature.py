#!/usr/bin/env python3
"""Recount native batch behavior with exact integers; no compiler execution."""
import hashlib
import json
import pathlib
import sys


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=False).encode('utf-8')


root = pathlib.Path(sys.argv[1])
rows = {}
cases = 0
for execution in sorted(root.glob('*/*/execution.json')):
    batch = execution.parent
    data = json.loads(execution.read_bytes())
    manifest = json.loads((batch / 'gooo.workspace.json').read_bytes())
    traces = data['result']['runtime']['traces']
    cases += len(traces)
    rows[batch.relative_to(root).as_posix()] = {
        'cases': sha((batch / 'cases.json').read_bytes()),
        'manifest': sha((batch / 'gooo.workspace.json').read_bytes()),
        'sources': {name: sha((batch / name).read_bytes())
                    for package in manifest['packages'] for name in package['sources']},
        'generated_go': sha(data['result']['composition']['source'].encode('utf-8')),
        'program': sha(canonical(data['result']['program'])),
        'traces': sha(canonical(traces)),
    }
if len(rows) != 80 or cases != 5510:
    raise SystemExit('Expected exactly 80 batches and 5,510 traces')
print(json.dumps({'schema': 'gooo/native-behavior-signature/v1', 'batches': len(rows),
                  'cases': cases, 'sha256': sha(canonical(rows))}, sort_keys=True))
