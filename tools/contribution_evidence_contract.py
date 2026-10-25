"""Frozen B1 evidence contract only; no response planning or publication.

Validator adapted from Gooo Language Bridge contributors, MIT, commit6142c05.
The copied validate function only changes $ref handling to refuse resolution.
See tools/licenses/gooo-language-bridge-6142c05-MIT.txt and the adjacent contract
provenance. This module never follows a reference from evidence metadata.
"""
from pathlib import Path
import hashlib
import json
import re

MAX_BYTES = 128 * 1024
SCHEMA_SHA256 = '1b9c4d455554976ff8e1c3ccf3c533bea1de6bf5567e92d979c3f372b2ab9e12'


def schema():
    path = Path(__file__).with_name('contracts') / 'contribution-evidence-v1.schema.json'
    raw = path.read_bytes()
    if hashlib.sha256(raw).hexdigest() != SCHEMA_SHA256:
        raise ValueError('frozen contribution contract changed')
    return json.loads(raw)


def validate(value, schema, location='$'):
    """Validate the bounded JSON Schema subset used by these three fixed schemas."""
    if '$ref' in schema:
        raise ValueError('external schema references are unsupported')
    types = {'object': dict, 'array': list, 'string': str, 'boolean': bool,
             'integer': int, 'null': type(None)}
    declared = schema.get('type')
    if declared:
        allowed = declared if type(declared) is list else [declared]
        if type(value) not in [types[t] for t in allowed]:
            raise ValueError(location + ': wrong JSON type')
    if 'const' in schema and value != schema['const']:
        raise ValueError(location + ': schema/version mismatch')
    if 'enum' in schema and value not in schema['enum']:
        raise ValueError(location + ': unknown enum')
    if type(value) is dict:
        properties = schema['properties']
        if set(value) != set(schema['required']) or set(value) - set(properties):
            raise ValueError(location + ': missing or unknown fields')
        for key, item in value.items():
            validate(item, properties[key], location + '.' + key)
    elif type(value) is list:
        if not schema.get('minItems', 0) <= len(value) <= schema['maxItems']:
            raise ValueError(location + ': array budget')
        for i, item in enumerate(value):
            validate(item, schema['items'], location + '[' + str(i) + ']')
    elif type(value) is str:
        if not schema.get('minLength', 0) <= len(value) <= schema.get('maxLength', MAX_BYTES):
            raise ValueError(location + ': string budget')
        if 'pattern' in schema and not re.fullmatch(schema['pattern'], value):
            raise ValueError(location + ': identity format')
    elif type(value) is int:
        if not schema.get('minimum', value) <= value <= schema.get('maximum', value):
            raise ValueError(location + ': numeric budget')
