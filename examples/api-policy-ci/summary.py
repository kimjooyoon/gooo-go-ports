#!/usr/bin/env python3
"""Present an Action-produced fixed-profile comparison without exposing facts.

The producer has already verified the complete typed comparison. Its expected
raw-file SHA256 is this presentation step's authority. This program checks the
transport shape and display vocabulary; it neither evaluates a policy nor
recomputes differences, classification, count, source bindings or authorization.
"""

import hashlib
import json
import os
import re
import stat
import sys


MAX_REPORT_BYTES = 16 << 20
MAX_OUTPUT_BYTES = 4096
MAX_TEXT_BYTES = 65536
FIELDS = (
    "allowed", "reason", "authenticated", "tenant_present", "tenant_matched",
    "action_present", "action_matched", "access_allowed", "access_error",
    "access_result", "group_matched", "principal_matched", "group_missing",
    "principal_defaulted", "effective_principal",
)
FACTS = (
    "group_present", "group_value", "principal_present", "principal_value",
    "authenticated", "caller_tenant", "resource_tenant", "requested_action",
    "allowed_action",
)
FACT_BOOLEANS = frozenset(("group_present", "principal_present", "authenticated"))
DECISION_TEXT = frozenset(("reason", "access_result", "effective_principal"))
CLASSIFICATIONS = frozenset((
    "NEWLY_ALLOWED", "NEWLY_DENIED", "REASON_CHANGED", "OBSERVATIONS_CHANGED",
    "UNCHANGED",
))
REASONS = frozenset((
    "ALLOW", "NOT_AUTHENTICATED", "TENANT_UNSPECIFIED", "TENANT_MISMATCH",
    "ACTION_UNSPECIFIED", "ACTION_MISMATCH", "ACCESS_ERROR", "ACCESS_DENIED",
    "ACTION_NOT_READ_ONLY",
))
ROOT_FIELDS = (
    "schema", "status", "scope", "before_profile", "candidate_profile",
    "before", "after", "differences", "summary", "classifier_bindings",
)


class Refused(Exception):
    """A fixed public label, never a caller-supplied error message."""


def require(condition, label="REPORT_SHAPE_REFUSED"):
    if not condition:
        raise Refused(label)


def exact_object(value, names):
    require(type(value) is dict and set(value) == set(names))
    return value


def text(value, limit=MAX_TEXT_BYTES):
    require(type(value) is str)
    try:
        require(len(value.encode("utf-8", "strict")) <= limit)
    except UnicodeError:
        raise Refused("REPORT_JSON_REFUSED") from None


def bindings(value):
    exact_object(value, ("compiler_source_sha", "assets", "generation_sha256"))
    text(value["compiler_source_sha"])
    text(value["generation_sha256"])
    assets = value["assets"]
    require(type(assets) is dict and 1 <= len(assets) <= 64)
    for name, digest in assets.items():
        text(name, 512)
        require(name != "")
        text(digest)


def proof(value):
    exact_object(value, ("facts", "decision", "bindings"))
    facts = exact_object(value["facts"], FACTS)
    for name in FACTS:
        if name in FACT_BOOLEANS:
            require(type(facts[name]) is bool)
        else:
            text(facts[name])
    decision = exact_object(value["decision"], FIELDS)
    for name in FIELDS:
        if name in DECISION_TEXT:
            text(decision[name])
        else:
            require(type(decision[name]) is bool)
    require(decision["reason"] in REASONS)
    bindings(value["bindings"])


def display_fields(report):
    exact_object(report, ROOT_FIELDS)
    expected = {
        "schema": "gooo/api-policy-revision-comparison/v1",
        "status": "COMPARED",
        "scope": "same_saved_facts_two_sealed_profiles",
        "before_profile": "cel-api-original-v1",
        "candidate_profile": "cel-api-readonly-v1",
    }
    for name, fixed in expected.items():
        require(type(report[name]) is str and report[name] == fixed)
    proof(report["before"])
    proof(report["after"])
    bindings(report["classifier_bindings"])
    differences = exact_object(report["differences"], FIELDS)
    for name in FIELDS:
        require(type(differences[name]) is bool)
    summary = exact_object(report["summary"], ("classification", "changed_count"))
    require(type(summary["classification"]) is str and
            summary["classification"] in CLASSIFICATIONS)
    require(type(summary["changed_count"]) is int and
            0 <= summary["changed_count"] <= len(FIELDS))
    # Project the already computed flags in fixed schema order. Do not compare
    # decisions, count true flags, or derive a replacement classification.
    names = [name for name in FIELDS if differences[name]]
    return (summary["classification"], summary["changed_count"],
            report["before"]["decision"]["reason"],
            report["after"]["decision"]["reason"], names)


def unique_object(pairs):
    result = {}
    for name, value in pairs:
        if name in result:
            raise Refused("REPORT_JSON_REFUSED")
        # Escaped surrogate object keys must not survive strict JSON decoding.
        name.encode("utf-8", "strict")
        if type(value) is str:
            value.encode("utf-8", "strict")
        result[name] = value
    return result


def no_constant(_value):
    raise Refused("REPORT_JSON_REFUSED")


def decode(raw):
    try:
        return json.loads(raw.decode("utf-8", "strict"),
                          object_pairs_hook=unique_object,
                          parse_constant=no_constant)
    except Refused:
        raise
    except (ValueError, UnicodeError, RecursionError, OverflowError):
        raise Refused("REPORT_JSON_REFUSED") from None


def same_file(left, right):
    return left.st_dev == right.st_dev and left.st_ino == right.st_ino


def unchanged(left, right):
    return (same_file(left, right) and left.st_size == right.st_size and
            left.st_mtime_ns == right.st_mtime_ns and
            left.st_ctime_ns == right.st_ctime_ns)


def output_separate(report_info):
    try:
        require(not same_file(report_info, os.fstat(1)), "OUTPUT_REFUSED")
    except OSError:
        raise Refused("OUTPUT_REFUSED") from None


def render(values):
    classification, count, before, after, names = values
    changed = ", ".join("`" + name + "`" for name in names) if names else "(none)"
    # Every interpolated string above comes from an exact, closed allowlist.
    # No original facts, decision values, source bindings or arbitrary text is
    # included, even when those unused parts contain valid markup or secrets.
    result = (
        "### Gooo readonly policy review\n\n"
        "| Result | Value |\n"
        "| --- | --- |\n"
        "| Classification | `" + classification + "` |\n"
        "| Changed fields | " + str(count) + " |\n"
        "| Before reason | `" + before + "` |\n"
        "| After reason | `" + after + "` |\n\n"
        "Changed field names: " + changed + ".\n\n"
        "Two supplied sealed profiles; this summary does not authorize a live action.\n"
    ).encode("utf-8")
    require(len(result) <= MAX_OUTPUT_BYTES, "OUTPUT_REFUSED")
    return result


def arguments(args):
    require(len(args) == 4, "ARGUMENTS_INVALID")
    require(set(args[::2]) == {"--report", "--report-sha256"}, "ARGUMENTS_INVALID")
    values = dict(zip(args[::2], args[1::2]))
    path, digest = values["--report"], values["--report-sha256"]
    require(path != "" and len(path) <= 4096 and "\x00" not in path,
            "ARGUMENTS_INVALID")
    require(re.fullmatch(r"[0-9a-f]{64}", digest) is not None,
            "ARGUMENTS_INVALID")
    return path, digest


def present(args):
    path, digest = arguments(args)
    descriptor = None
    try:
        descriptor = os.open(path, os.O_RDONLY | os.O_NONBLOCK | os.O_NOFOLLOW)
        before = os.fstat(descriptor)
        require(stat.S_ISREG(before.st_mode) and 0 <= before.st_size <= MAX_REPORT_BYTES,
                "REPORT_READ_REFUSED")
        output_separate(before)
        parts = []
        remaining = MAX_REPORT_BYTES + 1
        while remaining:
            part = os.read(descriptor, min(65536, remaining))
            if not part:
                break
            parts.append(part)
            remaining -= len(part)
        raw = b"".join(parts)
        after = os.fstat(descriptor)
        require(len(raw) <= MAX_REPORT_BYTES and len(raw) == after.st_size and
                unchanged(before, after), "REPORT_READ_REFUSED")
        require(hashlib.sha256(raw).hexdigest() == digest, "REPORT_DIGEST_MISMATCH")
        output = render(display_fields(decode(raw)))
        # Keep the original descriptor open and refuse direct/hardlink append
        # redirection into the input before the single buffered stdout write.
        require(unchanged(after, os.fstat(descriptor)), "REPORT_READ_REFUSED")
        output_separate(after)
        try:
            require(os.write(1, output) == len(output), "OUTPUT_REFUSED")
        except OSError:
            raise Refused("OUTPUT_REFUSED") from None
    except OSError:
        raise Refused("REPORT_READ_REFUSED") from None
    finally:
        if descriptor is not None:
            os.close(descriptor)


def main():
    try:
        present(sys.argv[1:])
        return 0
    except Refused as error:
        label = str(error)
    except (ValueError, UnicodeError, RecursionError, OverflowError, MemoryError):
        label = "REPORT_JSON_REFUSED"
    except Exception:
        # Refuse unexpected local transport failures without a traceback or
        # exception text that might include caller paths or report content.
        label = "REPORT_READ_REFUSED"
    try:
        os.write(2, ("Gooo policy summary: " + label + "\n").encode("ascii"))
    except OSError:
        pass
    return 2


if __name__ == "__main__":
    raise SystemExit(main())
