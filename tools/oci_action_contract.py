"""Bind real attestation step references to the reviewed credential reader plan.

This is a narrow action-reference check, not a general YAML parser. Workflow
syntax is checked separately; comments and inline Python strings are not steps.
"""
import json
from pathlib import Path
import re

REFERENCE = re.compile(r'^\s*(?:-\s+)?uses:\s+(actions/attest@\S+?)(?:\s+#.*)?\s*$')


def check(text, expected, count):
    refs = [match.group(1) for line in text.splitlines()
            for match in [REFERENCE.fullmatch(line)] if match]
    if len(refs) != count or any(ref != expected for ref in refs):
        raise ValueError('ATTEST_ACTION_CONTRACT_MISMATCH')
    return refs


def main():
    root = Path(__file__).resolve().parents[1]
    contract = json.loads((root / 'registry/oci-recovery/contract.json').read_bytes())
    for name, count in [('service-grant-mcp-oci.yml', 2),
                        ('service-grant-oci-proof-recovery.yml', 1)]:
        check((root / '.github/workflows' / name).read_text(), contract['action_ref'], count)
    print('ATTEST_ACTION_CONTRACT_MATCH')


if __name__ == '__main__':
    main()
