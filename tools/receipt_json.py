"""Decode receipt bytes without collapsing repeated decoded JSON object keys."""
import json


def _unique_object(pairs):
    result = {}
    for name, value in pairs:
        if name in result:
            raise ValueError(f"duplicate receipt JSON key: {name!r}")
        result[name] = value
    return result


def loads_receipt(raw):
    # Keep json.loads number, encoding and trailing-data behavior unchanged.
    # The hook visits every object, including objects inside arrays/unknown fields.
    return json.loads(raw, object_pairs_hook=_unique_object)
