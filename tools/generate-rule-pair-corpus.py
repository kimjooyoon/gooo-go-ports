# Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause: see LICENSE.
"""Deterministic input-only112 rule-pair corpus; no policy expectations.

The first18 cases are a frozen byte/value/order seed, embedded so regeneration
requires no private proof path. Bit arithmetic below selects input addresses;
actual Contains/Overlaps/decision expectations belong to a separate Go oracle.
"""
from pathlib import Path
import argparse
import hashlib
import json
import sys

SEED_SHA256 = '9a4d5d0c9de36152c77c8e8f602c240bc5c6e55b8281252c6a29c5edad5a6db7'
FIELDS = ('a', 'b', 'c', 'd', 'bits', 'effect', 'scope', 'priority')
DOMAIN_FIELDS = FIELDS[:-1]
SCHEMA = 'gooo/ipv4-rule-pair-inputs/v1'
SEED_JSON = r'''{
  "schema": "gooo/ipv4-rule-pair-inputs/v1",
  "cases": [
    {
      "name": "invalid-left-precedes-scope-and-priority",
      "left": {
        "a": -9223372036854775808,
        "b": 0,
        "c": 0,
        "d": 0,
        "bits": 8,
        "effect": 1,
        "scope": 1,
        "priority": -9223372036854775808
      },
      "right": {
        "a": 10,
        "b": 0,
        "c": 0,
        "d": 0,
        "bits": 8,
        "effect": 0,
        "scope": 2,
        "priority": 9223372036854775807
      }
    },
    {
      "name": "invalid-right-octet-and-bits",
      "left": {
        "a": 10,
        "b": 0,
        "c": 0,
        "d": 0,
        "bits": 8,
        "effect": 1,
        "scope": 1,
        "priority": 0
      },
      "right": {
        "a": 10,
        "b": 0,
        "c": 0,
        "d": 9223372036854775807,
        "bits": 33,
        "effect": 0,
        "scope": 1,
        "priority": 0
      }
    },
    {
      "name": "invalid-effect",
      "left": {
        "a": 10,
        "b": 0,
        "c": 0,
        "d": 0,
        "bits": 8,
        "effect": -1,
        "scope": 1,
        "priority": 0
      },
      "right": {
        "a": 10,
        "b": 0,
        "c": 0,
        "d": 0,
        "bits": 8,
        "effect": 1,
        "scope": 1,
        "priority": 0
      }
    },
    {
      "name": "invalid-zero-scope",
      "left": {
        "a": 10,
        "b": 0,
        "c": 0,
        "d": 0,
        "bits": 8,
        "effect": 1,
        "scope": 0,
        "priority": 0
      },
      "right": {
        "a": 10,
        "b": 0,
        "c": 0,
        "d": 0,
        "bits": 8,
        "effect": 0,
        "scope": 1,
        "priority": 0
      }
    },
    {
      "name": "invalid-negative-scope",
      "left": {
        "a": 10,
        "b": 0,
        "c": 0,
        "d": 0,
        "bits": 8,
        "effect": 1,
        "scope": 1,
        "priority": 9223372036854775807
      },
      "right": {
        "a": 10,
        "b": 0,
        "c": 0,
        "d": 0,
        "bits": 8,
        "effect": 0,
        "scope": -9223372036854775808,
        "priority": 9223372036854775807
      }
    },
    {
      "name": "scope-mismatch-before-priority",
      "left": {
        "a": 10,
        "b": 1,
        "c": 2,
        "d": 3,
        "bits": 8,
        "effect": 1,
        "scope": 1,
        "priority": -9223372036854775808
      },
      "right": {
        "a": 10,
        "b": 20,
        "c": 30,
        "d": 40,
        "bits": 16,
        "effect": 0,
        "scope": 2,
        "priority": 9223372036854775807
      }
    },
    {
      "name": "priority-mismatch",
      "left": {
        "a": 10,
        "b": 1,
        "c": 2,
        "d": 3,
        "bits": 8,
        "effect": 1,
        "scope": 42,
        "priority": -9223372036854775808
      },
      "right": {
        "a": 10,
        "b": 20,
        "c": 30,
        "d": 40,
        "bits": 16,
        "effect": 0,
        "scope": 42,
        "priority": 9223372036854775807
      }
    },
    {
      "name": "disjoint-opposite-effects",
      "left": {
        "a": 10,
        "b": 1,
        "c": 2,
        "d": 3,
        "bits": 8,
        "effect": 1,
        "scope": 42,
        "priority": 0
      },
      "right": {
        "a": 11,
        "b": 20,
        "c": 30,
        "d": 40,
        "bits": 8,
        "effect": 0,
        "scope": 42,
        "priority": 0
      }
    },
    {
      "name": "disjoint-before-same-effect",
      "left": {
        "a": 10,
        "b": 1,
        "c": 2,
        "d": 3,
        "bits": 8,
        "effect": 1,
        "scope": 42,
        "priority": 0
      },
      "right": {
        "a": 11,
        "b": 20,
        "c": 30,
        "d": 40,
        "bits": 8,
        "effect": 1,
        "scope": 42,
        "priority": 0
      }
    },
    {
      "name": "overlap-same-allow",
      "left": {
        "a": 10,
        "b": 1,
        "c": 2,
        "d": 3,
        "bits": 8,
        "effect": 1,
        "scope": 42,
        "priority": 0
      },
      "right": {
        "a": 10,
        "b": 20,
        "c": 30,
        "d": 40,
        "bits": 16,
        "effect": 1,
        "scope": 42,
        "priority": 0
      }
    },
    {
      "name": "overlap-same-deny-31-and-32",
      "left": {
        "a": 192,
        "b": 0,
        "c": 2,
        "d": 1,
        "bits": 31,
        "effect": 0,
        "scope": 42,
        "priority": -9223372036854775808
      },
      "right": {
        "a": 192,
        "b": 0,
        "c": 2,
        "d": 0,
        "bits": 32,
        "effect": 0,
        "scope": 42,
        "priority": -9223372036854775808
      }
    },
    {
      "name": "conflict-more-specific-right",
      "left": {
        "a": 10,
        "b": 1,
        "c": 2,
        "d": 3,
        "bits": 8,
        "effect": 1,
        "scope": 42,
        "priority": 7
      },
      "right": {
        "a": 10,
        "b": 20,
        "c": 30,
        "d": 40,
        "bits": 16,
        "effect": 0,
        "scope": 42,
        "priority": 7
      }
    },
    {
      "name": "conflict-swapped-same-witness",
      "left": {
        "a": 10,
        "b": 20,
        "c": 30,
        "d": 40,
        "bits": 16,
        "effect": 0,
        "scope": 42,
        "priority": 7
      },
      "right": {
        "a": 10,
        "b": 1,
        "c": 2,
        "d": 3,
        "bits": 8,
        "effect": 1,
        "scope": 42,
        "priority": 7
      }
    },
    {
      "name": "conflict-non-byte-prefix",
      "left": {
        "a": 192,
        "b": 168,
        "c": 1,
        "d": 199,
        "bits": 25,
        "effect": 0,
        "scope": 42,
        "priority": 9223372036854775807
      },
      "right": {
        "a": 192,
        "b": 168,
        "c": 1,
        "d": 170,
        "bits": 26,
        "effect": 1,
        "scope": 42,
        "priority": 9223372036854775807
      }
    },
    {
      "name": "conflict-equal-default-prefixes-zero-witness",
      "left": {
        "a": 255,
        "b": 255,
        "c": 255,
        "d": 255,
        "bits": 0,
        "effect": 1,
        "scope": 9223372036854775807,
        "priority": 9223372036854775807
      },
      "right": {
        "a": 0,
        "b": 0,
        "c": 0,
        "d": 0,
        "bits": 0,
        "effect": 0,
        "scope": 9223372036854775807,
        "priority": 9223372036854775807
      }
    },
    {
      "name": "conflict-zero-host-32-is-valid",
      "left": {
        "a": 0,
        "b": 0,
        "c": 0,
        "d": 0,
        "bits": 32,
        "effect": 0,
        "scope": 42,
        "priority": 0
      },
      "right": {
        "a": 255,
        "b": 255,
        "c": 255,
        "d": 255,
        "bits": 0,
        "effect": 1,
        "scope": 42,
        "priority": 0
      }
    },
    {
      "name": "conflict-maximum-host-32",
      "left": {
        "a": 255,
        "b": 255,
        "c": 255,
        "d": 255,
        "bits": 32,
        "effect": 1,
        "scope": 42,
        "priority": 0
      },
      "right": {
        "a": 255,
        "b": 255,
        "c": 255,
        "d": 254,
        "bits": 0,
        "effect": 0,
        "scope": 42,
        "priority": 0
      }
    },
    {
      "name": "adjacent-25-disjoint",
      "left": {
        "a": 192,
        "b": 168,
        "c": 1,
        "d": 127,
        "bits": 25,
        "effect": 1,
        "scope": 42,
        "priority": 0
      },
      "right": {
        "a": 192,
        "b": 168,
        "c": 1,
        "d": 128,
        "bits": 25,
        "effect": 0,
        "scope": 42,
        "priority": 0
      }
    }
  ]
}
'''


def require(condition, message):
    if not condition:
        raise ValueError(message)


def strict_object(items):
    value = {}
    for key, child in items:
        require(key not in value, 'duplicate decoded field')
        value[key] = child
    return value


def refuse_number(token):
    raise ValueError('integer tokens only: ' + token)


def read_seed():
    raw = SEED_JSON.encode('utf-8')
    require(hashlib.sha256(raw).hexdigest() == SEED_SHA256, 'original18 raw seed changed')
    doc = json.loads(SEED_JSON, object_pairs_hook=strict_object,
                     parse_float=refuse_number, parse_constant=refuse_number)
    require(list(doc) == ['schema', 'cases'] and doc['schema'] == SCHEMA,
            'original18 document shape')
    require(type(doc['cases']) is list and len(doc['cases']) == 18, 'original18 count')
    return doc


def rule(octets, bits, effect, scope, priority):
    return dict(zip(FIELDS, (*octets, bits, effect, scope, priority)))


def octets(packed):
    require(type(packed) is int and 0 <= packed <= 0xffffffff, 'input address integer bound')
    return tuple((packed >> shift) & 255 for shift in (24, 16, 8, 0))


def invalid_fields(value):
    # Input-domain validation only; no rule relation or policy is evaluated.
    bad = [name for name in FIELDS[:4] if not 0 <= value[name] <= 255]
    if not 0 <= value['bits'] <= 32:
        bad.append('bits')
    if value['effect'] not in (0, 1):
        bad.append('effect')
    if value['scope'] <= 0:
        bad.append('scope')
    return bad


def prefix_probes():
    rows = []
    anchor = (198 << 24) | (18 << 16) | (73 << 8) | 165
    for bits in range(33):
        # Input construction only. Keep original noncanonical left host bits.
        host_mask = (1 << (32 - bits)) - 1
        last_host = anchor | host_mask
        base_priority = 1000000 + 2 * bits
        left = rule(octets(anchor), bits, 1, 83197201, base_priority)
        right = rule(octets(last_host), 32, 0, 83197201, base_priority)
        rows.append({'name': 'prefix-%02d-inside-last-host-right32' % bits,
                     'left': left, 'right': right})
        # /0 has no disjoint valid IPv4 network. Its second probe deliberately
        # chooses another valid host and is labelled as overlap, not disjoint.
        different = anchor ^ (1 if bits == 0 else 1 << (32 - bits))
        second_priority = base_priority + 1
        left = rule(octets(anchor), bits, 1, 83197201, second_priority)
        right = rule(octets(different), 32, 0, 83197201, second_priority)
        suffix = 'other-host-valid-overlap' if bits == 0 else 'different-network-right32'
        rows.append({'name': 'prefix-%02d-%s' % (bits, suffix),
                     'left': left, 'right': right})
    return rows


def invalid_probes():
    choices = {'a': (-1, 256), 'b': (-1, 256), 'c': (-1, 256), 'd': (-1, 256),
               'bits': (-1, 33), 'effect': (-1, 2), 'scope': (0, -1)}
    rows = []
    for side in ('left', 'right'):
        for field in DOMAIN_FIELDS:
            for slot, invalid in enumerate(choices[field]):
                priority = 2000000 + len(rows)
                left = rule((203, 0, 113, 73), 24, 1, 83197202, priority)
                right = rule((203, 0, 113, 201), 32, 0, 83197202, priority)
                row = {'name': 'domain-%s-%s-invalid-%d' % (side, field, slot),
                       'left': left, 'right': right}
                row[side][field] = invalid
                other = 'right' if side == 'left' else 'left'
                require(invalid_fields(row[side]) == [field] and not invalid_fields(row[other]),
                        'exactly one intended domain guard per extra invalid probe')
                rows.append(row)
    return rows


def validate_rows(rows):
    require(len(rows) == 112, 'exact112 cases')
    names, tuples = set(), set()
    for row in rows:
        require(type(row) is dict and list(row) == ['name', 'left', 'right'],
                'input-only case shape')
        name = row['name']
        require(type(name) is str and 0 < len(name.encode('utf-8')) <= 128 and name not in names,
                'unique bounded case name')
        names.add(name)
        for side in ('left', 'right'):
            value = row[side]
            require(type(value) is dict and tuple(value) == FIELDS,
                    'exact ordered8-field Rule input')
            require(all(type(n) is int and -(1 << 63) <= n < (1 << 63) for n in value.values()),
                    'raw signed-int64 input values without coercion')
        key = tuple(row[side][field] for side in ('left', 'right') for field in FIELDS)
        require(key not in tuples, 'duplicate ordered16-integer tuple')
        tuples.add(key)
    return len(names), len(tuples)


def build():
    seed = read_seed()
    original = seed['cases']
    prefix = prefix_probes()
    invalid = invalid_probes()
    require(len(prefix) == 66 and len(invalid) == 28, 'fixed extra group counts')
    require(all(not invalid_fields(row[side]) for row in prefix for side in ('left', 'right')),
            'all66 prefix probes have valid input domains')
    rows = original + prefix + invalid
    names, tuples = validate_rows(rows)
    doc = {'schema': SCHEMA, 'cases': rows}
    raw = (json.dumps(doc, indent=2, ensure_ascii=True, allow_nan=False) + '\n').encode('utf-8')
    require(raw.startswith(SEED_JSON[:SEED_JSON.rfind('\n  ]')].encode('utf-8') + b',\n'),
            'original18 literal bytes/value/order prefix changed')
    require(len(raw) <= 96 << 10, 'bounded input-only corpus size')
    return raw, {'original_cases': 18, 'prefix_probes': 66, 'invalid_guard_probes': 28,
                 'cases': 112, 'unique_names': names, 'unique_ordered_tuples': tuples}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path,
                        default=Path(__file__).resolve().parent.parent / 'examples/ipv4-rule-pair/corpus-112.json')
    parser.add_argument('--check', action='store_true', help='Compare exact corpus bytes without writing')
    args = parser.parse_args()
    raw, counts = build()
    if args.check:
        require(args.output.read_bytes() == raw, 'checked corpus bytes differ from deterministic generation')
    else:
        with args.output.open('xb') as target:
            target.write(raw)
    print(json.dumps({'schema': 'gooo/rule-pair-input-generation/v1', 'output_bytes': len(raw),
                      'output_sha256': hashlib.sha256(raw).hexdigest(), **counts}, sort_keys=True))


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        sys.stderr.write('CORPUS_GENERATION_FAILED: ' + str(error) + '\n')
        sys.exit(1)
