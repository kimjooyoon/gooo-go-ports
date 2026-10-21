"""Strict expected-ledger loader and reviewed source-definition projection.

Generic FrozenGuard values belong to a separately reviewed caller. The production
wrapper freezes a reviewed full document as well as historical prefix/source
guards. It reads expected definitions only; no actual CI result is an input.
"""

from dataclasses import dataclass
import argparse
from pathlib import Path
import sys
import hashlib
import json
import re

MAX_BYTES = 1 << 20
MAX_INT64 = (1 << 63) - 1
SCHEMA = "gooo/native-verification-contract/v1"
PROFILES = {"case": "uuid-and-literal-report-compact-v1", "workspace": "dependency-closure-v1"}
ROLES = frozenset(("catalog-source", "explicit-observer-input", "catalog-definition",
                   "upstream-pins", "contract-reader"))


class ContractError(ValueError):
    pass


@dataclass(frozen=True)
class FrozenGuard:
    commit: str
    count: int
    sha256: str


@dataclass(frozen=True)
class LoadedContract:
    document: dict
    suite_count: int
    total_cases: int
    total_batches: int
    timing_events: int
    source_count: int
    baseline_count: int


def _need(condition, message):
    if not condition:
        raise ContractError(message)


def _pairs(items):
    result = {}
    for key, value in items:
        _need(key not in result, "duplicate decoded JSON key: " + key)
        result[key] = value
    return result


def _reject_number(_):
    raise ContractError("floating/exponent/nonfinite JSON numbers are forbidden")


def _parse_int(text):
    _need(re.fullmatch(r"0|[1-9][0-9]*", text) is not None, "expected unsigned decimal integer token")
    value = int(text)
    _need(-MAX_INT64 - 1 <= value <= MAX_INT64, "JSON integer outside int64")
    return value


def _object(value, keys):
    _need(type(value) is dict and set(value) == set(keys), "object missing or unknown fields")
    return value


def _integer(value, positive=False):
    _need(type(value) is int and (1 if positive else 0) <= value <= MAX_INT64,
          "expected exact decimal int64 in required range")
    return value


def _path(value):
    _need(type(value) is str and bool(value) and not value.startswith("/")
          and "\\" not in value and ":" not in value
          and all(0x20 <= ord(c) < 0x7f for c in value)
          and all(part not in ("", ".", "..") for part in value.split("/")),
          "unsafe canonical ASCII relative path")
    return value


def _strings(value):
    _need(type(value) is list and bool(value) and all(type(x) is str for x in value),
          "expected nonempty string array")
    _need(len(value) == len(set(value)), "duplicate string array member")
    return value


def _guard(value):
    _object(value, ("commit", "count", "sha256"))
    guard = FrozenGuard(value["commit"], value["count"], value["sha256"])
    _valid_guard(guard)
    return guard


def _valid_guard(guard):
    _need(type(guard) is FrozenGuard, "external guard type differs")
    _need(type(guard.commit) is str and re.fullmatch(r"[0-9a-f]{40}", guard.commit) is not None,
          "guard commit must be lowercase40hex")
    _integer(guard.count, positive=True)
    _need(type(guard.sha256) is str
          and re.fullmatch(r"sha256:[0-9a-f]{64}", guard.sha256) is not None,
          "guard digest must be sha256: plus lowercase64hex")


def _digest(value):
    # Paths/packages/activities are ASCII, so U+2028/U+2029 differences cannot
    # occur. ensure_ascii=False also leaves HTML characters unescaped, as Go.
    raw = json.dumps(value, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
    return "sha256:" + hashlib.sha256(raw).hexdigest()


def _add(a, b):
    _need(0 <= a <= MAX_INT64 and 0 <= b <= MAX_INT64 - a, "int64 total overflow")
    return a + b


def load_contract(raw, frozen_prefix, frozen_sources):
    """Load expected data only; current Git/corpus/actual CI equality is external."""
    _valid_guard(frozen_prefix)
    _valid_guard(frozen_sources)
    _need(type(raw) is bytes and len(raw) <= MAX_BYTES, "contract must be bytes <=1MiB")
    try:
        doc = json.loads(raw.decode("utf-8", errors="strict"), object_pairs_hook=_pairs,
                         parse_int=_parse_int, parse_float=_reject_number,
                         parse_constant=_reject_number)
    except (ValueError, UnicodeError, RecursionError) as error:
        if isinstance(error, ContractError):
            raise
        raise ContractError("invalid strict JSON: " + type(error).__name__) from error
    _object(doc, ("schema", "profiles", "prefix", "suites", "source_baseline", "sources"))
    _need(doc["schema"] == SCHEMA, "unknown schema")
    _object(doc["profiles"], ("case", "workspace"))
    _need(doc["profiles"] == PROFILES, "unexpected exact profiles")
    _need(_guard(doc["prefix"]) == frozen_prefix, "prefix differs from external guard")
    _need(_guard(doc["source_baseline"]) == frozen_sources, "source baseline differs from external guard")
    rows = doc["sources"]
    _need(type(rows) is list and bool(rows), "sources must be nonempty array")
    qualified = set()
    baseline = []
    prior = None
    for row in rows:
        _object(row, ("path", "mode", "owner", "roles", "baseline"))
        path = _path(row["path"])
        _need(prior is None or prior < path, "source paths must be strictly sorted")
        prior = path
        _need(type(row["mode"]) is str and row["mode"] in ("100644", "100755"), "nonregular Git mode")
        _need(row["owner"] == "native-ci", "unknown source owner")
        roles = _strings(row["roles"])
        _need(roles == sorted(roles) and all(role in ROLES for role in roles), "unknown or unsorted roles")
        _need(type(row["baseline"]) is bool, "baseline must be a boolean")
        if "catalog-source" in roles:
            qualified.add(path)
        if row["baseline"]:
            baseline.append({"path": path, "mode": row["mode"]})
    _need(len(baseline) == frozen_sources.count, "baseline count differs")
    _need(_digest(baseline) == frozen_sources.sha256, "baseline path/mode digest differs")
    suites = doc["suites"]
    _need(type(suites) is list and len(suites) >= frozen_prefix.count, "missing frozen prefix suites")
    keys = set()
    canonical = []
    total_cases = total_batches = 0
    for index, row in enumerate(suites):
        _object(row, ("index", "package", "activity", "cases", "batches", "source_paths"))
        _need(_integer(row["index"]) == index, "noncontiguous suite index")
        package = _path(row["package"])
        activity = row["activity"]
        _need(type(activity) is str and re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", activity) is not None,
              "activity must be ASCII identifier")
        key = (package, activity)
        _need(key not in keys, "duplicate package:activity")
        keys.add(key)
        cases = _integer(row["cases"], positive=True)
        batches = _integer(row["batches"], positive=True)
        _need(batches <= cases, "batches exceed cases")
        sources = _strings(row["source_paths"])
        for path in sources:
            _path(path)
            _need(path in qualified, "suite source missing or lacks catalog-source role")
        canonical.append({"index": index, "package": package, "activity": activity,
                          "cases": cases, "batches": batches, "source_paths": sources})
        total_cases = _add(total_cases, cases)
        total_batches = _add(total_batches, batches)
    _need(_digest(canonical[:frozen_prefix.count]) == frozen_prefix.sha256, "suite prefix digest differs")
    events = _add(_add(_add(2, len(suites)), len(suites)), total_batches)
    return LoadedContract(doc, len(suites), total_cases, total_batches, events, len(rows), len(baseline))


REVIEWED_PATH = "cmd/verify/testdata/native-verification-contract-v1.json"
REVIEWED_RAW_SHA256 = "f9549c87045a92db2b21215263021797badf3a655327b7d6b286acced4884d78"
REVIEWED_PREFIX = FrozenGuard("b19b5517f8e9716affb0f75fd07045b8217cdaf1", 80,
    "sha256:9020c39f7746e766ef38b6d5854d97c14fcd386638d038937cf8f178aec52478")
# The source baseline binds immutable main 057bc6d8 path/mode identities.
# Loading expected definitions does not establish runtime success.
# A later source revision requires explicit baseline metadata/guard and raw-hash updates.
REVIEWED_SOURCES = FrozenGuard("057bc6d8d328a6ab2e7a3b37845a7450a6b19d0b", 283,
    "sha256:2b7247008db8ee9d7d92374bb3d01a317e3e35c2df009cfe943ac153037f2184")
CONTRACT_FILES = (
    "cmd/verify/native_verification_contract.go",
    "cmd/verify/native_verification_contract_test.go",
    REVIEWED_PATH,
    "tools/native_verification_contract.py",
    "tools/native_verification_contract_test.py",
)


def load_reviewed_contract(root, *, read_bytes=None):
    """Load the reviewed current definition, never learn it from actual results.

    Updating this expected definition requires a reviewed explicit raw digest
    change. A generic valid old80 prefix is not the reviewed current population.
    read_bytes lets a read-only caller retain the same raw input in its pins.
    """
    path = Path(root) / REVIEWED_PATH
    if read_bytes is None:
        with path.open("rb") as source:
            raw = source.read(MAX_BYTES + 1)
    else:
        raw = read_bytes(path)
    _need(type(raw) is bytes and len(raw) <= MAX_BYTES, "reviewed contract must be bytes <=1MiB")
    _need(hashlib.sha256(raw).hexdigest() == REVIEWED_RAW_SHA256,
          "reviewed full contract raw digest differs")
    result = load_contract(raw, REVIEWED_PREFIX, REVIEWED_SOURCES)
    # The full raw guard already freezes these rows. This explicit membership
    # makes their role in the source observer clear to all production callers.
    by_path = {row["path"]: row for row in result.document["sources"]}
    for name in CONTRACT_FILES:
        _need(name in by_path and "contract-reader" in by_path[name]["roles"],
              "reviewed definition must observe every contract reader")
    return result


def expected_projection(contract):
    """Project only independently reviewed expectations; preserve row order."""
    doc = contract.document
    prefix = doc["suites"][:doc["prefix"]["count"]]
    return {
        "schema": "gooo/native-verification-expected/v1",
        "profiles": dict(doc["profiles"]),
        "totals": {"suites": contract.suite_count, "cases": contract.total_cases,
                   "batches": contract.total_batches, "timing_records": contract.timing_events,
                   "source_paths": contract.source_count},
        "prefix": {"count": len(prefix), "cases": sum(row["cases"] for row in prefix),
                   "batches": sum(row["batches"] for row in prefix)},
        "suites": [{key: row[key] for key in
                    ("index", "package", "activity", "cases", "batches", "source_paths")}
                   for row in doc["suites"]],
        "source_paths": [row["path"] for row in doc["sources"]],
    }


def main(argv=None):
    parser = argparse.ArgumentParser(description="Export reviewed expected native verification data")
    parser.add_argument("--root", type=Path, required=True)
    args = parser.parse_args(argv)
    try:
        result = expected_projection(load_reviewed_contract(args.root))
        # Build the complete output before emitting it. Invalid data emits no
        # stdout JSON and is never replaced with observations or old defaults.
        rendered = json.dumps(result, ensure_ascii=False, separators=(",", ":")) + "\n"
    except (OSError, ContractError) as error:
        print("native verification expected contract: " + str(error), file=sys.stderr)
        return 1
    sys.stdout.write(rendered)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
