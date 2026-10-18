#!/usr/bin/env python3
"""Convert one version request to Gooo inputs; compute no version decision."""
import json
import sys

MAX_RAW_BYTES = 32 << 10
MAX_TEXT_BYTES = 1024
FIELDS = ("current", "candidate", "minimum")
ENTRY = "app/version-eligibility:AssessUpdate"
ENTRIES = {"minimum": ENTRY, "stable": "app/version-eligibility:AssessStableSameMajorUpdate"}


def unique_pairs(items):
    """Use decoded keys, including escaped spellings, before object conversion."""
    result = {}
    for key, value in items:
        if key in result:
            raise ValueError("duplicate decoded JSON key")
        result[key] = value
    return result


def nonfinite(_value):
    raise ValueError("nonfinite JSON value")


def prepare(raw, policy="minimum"):
    if policy not in ENTRIES:
        raise ValueError("unsupported version policy")
    if len(raw) > MAX_RAW_BYTES:
        raise ValueError("request exceeds 32 KiB")
    request = json.loads(raw.decode("utf-8"), object_pairs_hook=unique_pairs,
                         parse_constant=nonfinite)
    if type(request) is not dict or set(request) != set(FIELDS):
        raise ValueError("require exactly current, candidate and minimum")
    for name in FIELDS:
        text = request[name]
        if type(text) is not str:
            raise ValueError(name + " requires Text")
        try:
            encoded = text.encode("utf-8")
        except UnicodeError as error:
            raise ValueError(name + " requires Unicode scalar values") from error
        if len(encoded) > MAX_TEXT_BYTES:
            raise ValueError(name + " exceeds 1024 UTF-8 bytes")
    # Preserve invalid version strings too: Gooo owns validity and eligibility.
    document = {"schema": "gooo/body-composition-inputs/v1", "inputs": [
        {ENTRIES[policy] + ".input" + str(i): request[name] for i, name in enumerate(FIELDS)}]}
    return (json.dumps(document, ensure_ascii=True, separators=(",", ":")) + "\n").encode()


def run(args, reader, writer, errors):
    if args and (len(args) != 2 or args[0] != "--policy" or args[1] not in ENTRIES):
        print("usage: request-inputs.py [--policy minimum|stable] < request.json > inputs.json", file=errors)
        return 2
    policy = args[1] if args else "minimum"
    try:
        output = prepare(reader.read(MAX_RAW_BYTES + 1), policy)
        if writer.write(output) != len(output):
            raise OSError("short output write")
    except (ValueError, UnicodeError, OSError, RecursionError) as error:
        print("version request refused: " + str(error), file=errors)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(run(sys.argv[1:], sys.stdin.buffer, sys.stdout.buffer, sys.stderr))
