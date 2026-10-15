"""Reproduce input-only PayloadAdmissionFacts cases; never compute policy results."""
from pathlib import Path
import argparse
import hashlib
import itertools
import json

ROOT = Path(__file__).resolve().parents[1]
SEED = ROOT / "examples/payload-admission/manual17.json"
OUTPUT = ROOT / "examples/payload-admission/corpus.json"
SEED_SHA256 = "fc54b8c739824dc74ab3850201c8bb3f448e8a06a8406630b02963c522eeda8b"
NAMES = ("declared_length", "content_length_occurrences", "observed_body_bytes",
         "maximum_payload_bytes", "transfer_encoding_present",
         "body_observation_bound_to_request", "current_access")
MIN_INT64 = -(1 << 63)
MAX_INT64 = (1 << 63) - 1


def require(condition, message):
    if not condition:
        raise ValueError(message)


def unique_object(items):
    result = {}
    for key, value in items:
        require(key not in result, "duplicate JSON key")
        result[key] = value
    return result


def reject_number(value):
    raise ValueError("non-integral/non-finite JSON number: " + value)


def validate_inputs(value):
    require(type(value) is dict and set(value) == set(NAMES), "exact seven input names")
    text = value[NAMES[0]]
    require(type(text) is str and len(text.encode("utf-8", errors="strict")) <= 1024,
            "decoded valid UTF-8 Text at most1024 bytes")
    for name in NAMES[1:4]:
        require(type(value[name]) is int and MIN_INT64 <= value[name] <= MAX_INT64,
                "exact signed64 metadata: " + name)
    for name in NAMES[4:]:
        require(type(value[name]) is bool, "exact Boolean context: " + name)


def generate():
    raw = SEED.read_bytes()
    require(hashlib.sha256(raw).hexdigest() == SEED_SHA256, "original17 seed bytes changed")
    seed = json.loads(raw.decode("utf-8", errors="strict"), object_pairs_hook=unique_object,
                      parse_float=reject_number, parse_constant=reject_number)
    require(type(seed) is dict and set(seed) == {"schema", "cases"} and
            seed["schema"] == "gooo/payload-admission-inputs/v1" and
            type(seed["cases"]) is list and len(seed["cases"]) == 17, "original17 seed schema")
    rows, names, tuples = [], set(), set()

    def add(name, values, already_covered=False):
        require(type(name) is str and name and name not in names, "unique case name")
        validate_inputs(values)
        key = tuple(values[field] for field in NAMES)
        if key in tuples:
            # A boundary may already be one of the immutable original17.
            # Keep that first witness rather than repeat its identical tuple.
            require(already_covered, "duplicate original input tuple: " + name)
            return
        names.add(name)
        tuples.add(key)
        rows.append({"name": name, "inputs": {field: values[field] for field in NAMES}})

    def row(text="23", occurrences=1, observed=23, maximum=23,
            transfer=False, bound=True, access=True):
        return dict(zip(NAMES, (text, occurrences, observed, maximum, transfer, bound, access)))

    for case in seed["cases"]:
        require(type(case) is dict and set(case) == {"name", "inputs"}, "input-only seed row")
        add(case["name"], case["inputs"])

    # The full Boolean context product uses one unchanged valid observation.
    # It supplies inputs only; no eligible/reason/expected value is calculated.
    for transfer, bound, access in itertools.product((False, True), repeat=3):
        add("context-te%d-bound%d-access%d" % (transfer, bound, access),
            row("37", 1, 37, 37, transfer, bound, access))

    for field in NAMES[1:4]:
        for label, value in (("negative-one", -1), ("minimum", MIN_INT64), ("maximum", MAX_INT64)):
            values = row()
            values[field] = value
            add("metadata-" + field + "-" + label, values)
        for label, value in (("negative-one", -1), ("minimum", MIN_INT64)):
            values = row(transfer=True, bound=False, access=False)
            values[field] = value
            add("metadata-and-context-" + field + "-" + label, values)

    for occurrences in (0, 1, 2, MAX_INT64):
        for label, text in (("empty", ""), ("digits", "5"), ("comma", "5,5")):
            add("occurrences-%d-%s" % (occurrences, label), row(text, occurrences, 5, 7))

    spellings = (("empty", ""), ("plus-only", "+"), ("minus-only", "-"),
                 ("letter", "x"), ("leading-space", " 5"), ("trailing-space", "5 "),
                 ("tab", "\t5"), ("lf", "5\n"), ("crlf", "5\r\n"),
                 ("underscore", "5_0"), ("hex-prefix", "0x10"), ("fullwidth-digit", "５"),
                 ("unicode-minus", "−5"), ("nul", "5\x00"), ("comma", "5,5"),
                 ("plus", "+5"), ("negative-zero", "-0"), ("negative", "-5"),
                 ("leading-zero", "0005"), ("digits", "5"))
    for label, text in spellings:
        add("spelling-" + label, row(text, 1, 5, 10))

    boundaries = (("signed-maximum", "9223372036854775807"),
                  ("signed-maximum-plus-one", "9223372036854775808"),
                  ("signed-overflow-invalid-suffix", "9223372036854775808x"),
                  ("unsigned-maximum-invalid-suffix", "18446744073709551615x"),
                  ("unsigned-overflow-invalid-suffix", "18446744073709551616x"),
                  ("signed-minimum", "-9223372036854775808"),
                  ("signed-minimum-minus-one", "-9223372036854775809"),
                  ("negative-unsigned-overflow-invalid-suffix", "-18446744073709551616x"),
                  ("leading-zero-1024-bytes", "0" * 1023 + "5"))
    for label, text in boundaries:
        add("parse-boundary-" + label, row(text, 1, MAX_INT64, MAX_INT64), already_covered=True)

    for observed, maximum in itertools.product((4, 5, 6), repeat=2):
        add("body-%d-limit-%d" % (observed, maximum), row("5", 1, observed, maximum))

    for label, text in (("syntax", "x"), ("signed-spelling", "+9"),
                        ("unsigned-overflow-suffix", "18446744073709551616x")):
        for context, transfer, bound, access in (("access", False, True, False),
                                                ("binding", False, False, True),
                                                ("te", True, True, True),
                                                ("combined", True, False, False)):
            add("context-with-" + label + "-" + context,
                row(text, 1, 9, 9, transfer, bound, access))
    add("exact-large-leading-zero", row("0009007199254740993", 1, 9007199254740993, 9007199254740994))

    require(rows[:17] == seed["cases"], "original17 ordered values must remain first")
    return {"schema": seed["schema"], "cases": rows}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true", help="compare exact bytes without writing")
    args = parser.parse_args()
    corpus = generate()
    raw = (json.dumps(corpus, ensure_ascii=False, indent=2) + "\n").encode("utf-8")
    if args.check:
        require(OUTPUT.read_bytes() == raw, "payload corpus is stale; regenerate with this tool")
    else:
        OUTPUT.write_bytes(raw)
    print(json.dumps({"schema": "gooo/payload-input-generator/v1", "mode": "check" if args.check else "write",
                      "cases": len(corpus["cases"]), "seed_cases": 17, "bytes": len(raw),
                      "sha256": hashlib.sha256(raw).hexdigest(), "expected_values": 0}))


if __name__ == "__main__":
    main()
