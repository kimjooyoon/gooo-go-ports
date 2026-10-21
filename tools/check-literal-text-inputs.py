#!/usr/bin/env python3
"""Observe bounded Text/shape rejection through the pinned CLI, without a Go oracle.

Only decoded valid UTF-8 strings and malformed request shapes are used here.
Wire-invalid UTF-8 and lone-surrogate decoder behavior are outside this roster;
replacement by a JSON decoder must never be reported as wire-byte preservation
or as a rejected decoded Text value.
"""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys

from receipt_json import loads_receipt

COMPILER = "205896a413e5dea92366309c35ae85e542d055f6"
VERSION = "0.6.37-dev"
GO = "go1.27.3"
SCHEMA = "gooo/body-composition-inputs/v1"
ENTRIES = ("go/strings:Index", "go/strings:Count", "app/literal-text-report:Report")
CUT_ENTRIES = ("go/strings:Cut", "go/strings:CutPrefix", "go/strings:CutSuffix", "app/literal-cut-report:Report")
IF_MATCH_ENTRY = "go/net/http/precondition:IfMatchCondition"
IF_MATCH_SOURCE = "ports/net/http/precondition.gooo"
ALL_ENTRIES = (*ENTRIES, *CUT_ENTRIES, IF_MATCH_ENTRY)
PORT_SOURCES = ("ports/strings/search.gooo", "ports/strings/literal.gooo")
SOURCES = (*PORT_SOURCES, "examples/literal-text-report/main.gooo")
CUT_PORT_SOURCES = (*PORT_SOURCES, "ports/strings/cut.gooo")
CUT_SOURCES = (*CUT_PORT_SOURCES, "examples/literal-cut-report/main.gooo")
FROZEN_FILES = ("literal-text-report.workspace.json", *SOURCES,
                "literal-cut-report.workspace.json", "ports/strings/cut.gooo", CUT_SOURCES[-1],
                "if-match.workspace.json", IF_MATCH_SOURCE)
RECEIPT_SCHEMA = "gooo/workspace-body-execution-receipt/v1"


def require(condition, message):
    if not condition:
        raise ValueError(message)


def digest(raw):
    return "sha256:" + hashlib.sha256(raw).hexdigest()


def encode(value):
    return json.dumps(value, ensure_ascii=False, separators=(",", ":")).encode("utf-8")


def validate_inputs(raw, entry):
    require(entry in ALL_ENTRIES, "untrusted entry")
    try:
        value = loads_receipt(raw.decode("utf-8", errors="strict"))
    except (UnicodeError, TypeError) as error:
        raise ValueError("input JSON must be valid UTF-8") from error
    require(type(value) is dict and set(value) == {"schema", "inputs"}, "input envelope fields")
    require(value["schema"] == SCHEMA, "input schema")
    rows = value["inputs"]
    require(type(rows) is list and 1 <= len(rows) <= 128, "input row count")
    keys = {entry + ".input0", entry + ".input1"}
    for row in rows:
        require(type(row) is dict and set(row) == keys, "exact input ports")
        for key in sorted(keys):
            require(type(row[key]) is str, "explicit Text scalar")
            try:
                size = len(row[key].encode("utf-8", errors="strict"))
            except UnicodeError as error:
                raise ValueError("decoded Text must be valid UTF-8") from error
            require(size <= 1024, "decoded Text exceeds 1024 UTF-8 bytes")
    return rows


def negative_inputs():
    """An ordered finite roster; request count is independent of oracle cases."""
    wrong = (("null", None), ("false", False), ("true", True), ("integer", 0),
             ("fraction", 1.5), ("array", []), ("object", {}))
    for entry in ALL_ENTRIES:
        prefix = entry.rsplit(":", 1)[1].lower()
        if entry == "app/literal-cut-report:Report":
            prefix = "cutreport"
        base = {entry + ".input0": "diagnostic", entry + ".input1": "tic"}

        def case(name, row, diagnostic, stage="INPUT_DOMAIN", position=None, raw=None):
            return {"name": prefix + "-" + name, "entry": entry, "position": position,
                    "expected_diagnostic": diagnostic, "expected_rejection_stage": stage,
                    "raw": raw if raw is not None else encode({"schema": SCHEMA, "inputs": [row]}) + b"\n"}

        for position in range(2):
            key = entry + ".input" + str(position)
            for name, value in (("ascii-1025", "a" * 1025), ("multibyte-1025", "é" * 512 + "a"), *wrong):
                row = dict(base, **{key: value})
                diagnostic = "explicit Text scalar required" if value is None else "Text requires a string of at most 1024 UTF-8 bytes"
                yield case(name + "-input" + str(position), row, diagnostic, position=position)
            row = dict(base)
            del row[key]
            yield case("missing-input" + str(position), row, "requires an explicit value", position=position)
            for name, duplicate_key in (("duplicate", encode(key)), ("duplicate-escaped", encode(key).replace(b"input", b"\\u0069nput"))):
                token = encode(key) + b":" + encode(base[key])
                duplicate = token + b"," + duplicate_key + b":" + encode(base[key])
                raw = encode({"schema": SCHEMA, "inputs": [base]}).replace(token, duplicate, 1) + b"\n"
                yield case(name + "-input" + str(position), base, "duplicate or invalid JSON key", "INPUT_JSON_DECODE", position, raw)
        yield case("extra-input2", dict(base, **{entry + ".input2": "extra"}),
                   "inputs and nonempty expectations must name declared activities")
        raw = encode({"schema": "gooo/wrong-inputs/v1", "inputs": [base]}) + b"\n"
        yield case("wrong-schema", base, "composition inputs require schema and 1..128 input rows", "INPUT_JSON_DECODE", raw=raw)
        for field, escaped in (("schema", b'"\\u0073chema"'), ("inputs", b'"\\u0069nputs"')):
            value = SCHEMA if field == "schema" else [base]
            token = encode(field) + b":" + encode(value)
            raw = encode({"schema": SCHEMA, "inputs": [base]}).replace(token, token + b"," + escaped + b":" + encode(value), 1) + b"\n"
            yield case("duplicate-" + field, base, "duplicate or invalid JSON key", "INPUT_JSON_DECODE", raw=raw)


def clean_environment():
    env = os.environ.copy()
    for key in list(env):
        if key.startswith("GOOO_") or key in ("OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY", "HF_TOKEN"):
            env.pop(key, None)
    env.update(GOTOOLCHAIN="local", GOENV="off", GOWORK="off", GOPROXY="off", GOSUMDB="off")
    return env


def binding(path):
    raw = path.read_bytes()
    return {"path": str(path), "bytes": len(raw), "sha256": digest(raw)}


def validate_identity(build, lock):
    require(lock["compiler_revision"] == COMPILER and lock["compiler_version"] == VERSION and lock["go_version"] == GO,
            "upstream pin differs from trusted rejection observer")
    require(build.get("schema") == "gooo/build-identity/v1" and build.get("version") == VERSION
            and build.get("compiler_source_sha") == build.get("vcs_revision") == COMPILER
            and build.get("source_status") == "CLEAN_VCS" and build.get("vcs_modified") == "false"
            and build.get("go_version") == build.get("native_go_required") == GO,
            "clean pinned compiler identity")


def inspect_extra_caller_port(case, receipt, error):
    # Exact generated extra-input2 fixtures only. The LastIndex observer reuses
    # this inspector with its separately pinned two-entry source closure.
    entries = (*ALL_ENTRIES, "go/strings:LastIndex", "app/literal-final-marker-report:Report")
    require(case["entry"] in entries and case["position"] is None
            and case["expected_rejection_stage"] == "INPUT_DOMAIN"
            and case["expected_diagnostic"] == "inputs and nonempty expectations must name declared activities",
            "declared extra caller-port fixture")
    entry = case["entry"]
    expected_raw = encode({"schema": SCHEMA, "inputs": [{entry + ".input0": "diagnostic",
                               entry + ".input1": "tic", entry + ".input2": "extra"}]}) + b"\n"
    require(case["raw"] == expected_raw, "exact declared extra caller-port input bytes")
    diagnostic = receipt.get("input_diagnostic")
    require(type(diagnostic) is dict and set(diagnostic) == {
        "stage", "code", "case_index", "key", "available_keys", "message"}, "structured caller-port diagnostic fields")
    require(diagnostic["stage"] == "INPUT_VALIDATION" and diagnostic["code"] == "UNKNOWN_CALLER_PORT",
            "structured caller-port diagnostic stage/code")
    require(type(diagnostic["case_index"]) is int and diagnostic["case_index"] == 0,
            "structured caller-port case index")
    require(diagnostic["key"] == entry + ".input2"
            and diagnostic["available_keys"] == [entry + ".input0", entry + ".input1"],
            "structured authored key and exact sorted available caller ports")
    message = diagnostic["message"]
    require(type(message) is str and bool(message.strip())
            and error == "generate workspace activity bodies: " + message,
            "structured diagnostic message and outer error binding")
    return diagnostic


def inspect_rejection(case, stdout, exit_code, timed_out, manifest_path, manifest_raw):
    require(not timed_out, "actual CLI timed out")
    receipt = loads_receipt(stdout)
    require(type(receipt) is dict and receipt.get("schema") == RECEIPT_SCHEMA, "execution receipt schema")
    require(exit_code == 1 and receipt.get("decision") == "FAIL_CLOSED", "actual CLI rejection exit/decision")
    require(receipt.get("manifest") == manifest_path.as_posix(), "requested manifest path")
    require(receipt.get("manifest_digest") == digest(manifest_raw), "original manifest binding")
    require(receipt.get("inputs_digest") == digest(case["raw"]), "original malformed input binding")
    error = receipt.get("error")
    require(type(error) is str, "actual input diagnostic type")
    structured = None
    if case["name"].endswith("-extra-input2"):
        structured = inspect_extra_caller_port(case, receipt, error)
    else:
        require(case["expected_diagnostic"] in error, "actual input diagnostic")
    if case["expected_rejection_stage"] == "INPUT_DOMAIN" and structured is None:
        require(error.startswith("generate workspace activity bodies: case 0:"), "input-domain rejection route")
        if case["position"] is not None:
            require('.input' + str(case["position"]) + '"' in error, "actual rejected input position")
    elif structured is None:
        require(not error.startswith("generate workspace activity bodies:"), "JSON rejection must precede body generation")
    # The pinned input decoder/domain route returns no Result. An unexpected
    # Result is rejected, rather than guessed safe from selected counter fields.
    require("result" not in receipt, "malformed input returned unexpected workspace/runtime result")
    return {"decision": receipt["decision"], "error": error,
            "verified_rejection_stage": case["expected_rejection_stage"],
            "result_present": False, "runtime_present": False,
            "actual_runtime_stage": "NOT_REPORTED", "observed_counters": "NOT_REPORTED",
            "observed_build": "NOT_REPORTED", "observed_runs": "NOT_REPORTED",
            "rejection_verified": True,
            "structured_input_diagnostic_verified": structured is not None,
            "native_execution_claim": "NOT_REPORTED: no zero-counter or native-execution claim inferred from absent runtime"}


def trusted_manifests(frozen):
    """Validate both exact closures before any subprocess; old bytes stay old."""
    require(ENTRIES == ("go/strings:Index", "go/strings:Count", "app/literal-text-report:Report")
            and CUT_ENTRIES == ("go/strings:Cut", "go/strings:CutPrefix", "go/strings:CutSuffix", "app/literal-cut-report:Report")
            and IF_MATCH_ENTRY == "go/net/http/precondition:IfMatchCondition"
            and IF_MATCH_SOURCE == "ports/net/http/precondition.gooo"
            and ALL_ENTRIES == (*ENTRIES, *CUT_ENTRIES, IF_MATCH_ENTRY), "complete ordered eight-entry rejection roster")
    require(PORT_SOURCES == ("ports/strings/search.gooo", "ports/strings/literal.gooo")
            and SOURCES == (*PORT_SOURCES, "examples/literal-text-report/main.gooo")
            and CUT_PORT_SOURCES == (*PORT_SOURCES, "ports/strings/cut.gooo")
            and CUT_SOURCES == (*CUT_PORT_SOURCES, "examples/literal-cut-report/main.gooo"), "exact separate old/new source closures")
    require(set(frozen) == {"literal-text-report.workspace.json", *SOURCES,
                            "literal-cut-report.workspace.json", "ports/strings/cut.gooo", CUT_SOURCES[-1],
                            "if-match.workspace.json", IF_MATCH_SOURCE},
            "complete frozen source/manifest roster")
    original_manifest = loads_receipt(frozen["literal-text-report.workspace.json"])
    require(original_manifest == {"schema": "gooo/package-workspace-manifest/v1",
                                 "entry": {"package_path": "app/literal-text-report", "activity": "Report"},
                                 "packages": [{"path": "go/strings", "sources": list(PORT_SOURCES)},
                                              {"path": "app/literal-text-report", "sources": [SOURCES[-1]]}]},
            "trusted three-source report manifest")
    new_manifest = loads_receipt(frozen["literal-cut-report.workspace.json"])
    require(new_manifest == {"schema": "gooo/package-workspace-manifest/v1",
                            "entry": {"package_path": "app/literal-cut-report", "activity": "Report"},
                            "packages": [{"path": "go/strings", "sources": list(CUT_PORT_SOURCES)},
                                         {"path": "app/literal-cut-report", "sources": [CUT_SOURCES[-1]]}]},
            "trusted separate Cut report manifest")
    if_match_manifest = loads_receipt(frozen["if-match.workspace.json"])
    require(if_match_manifest == {"schema":"gooo/package-workspace-manifest/v1",
        "entry":{"package_path":"go/net/http/precondition","activity":"IfMatchCondition"},
        "packages":[{"path":"go/net/http/precondition","sources":[IF_MATCH_SOURCE]}]},
        "trusted one-source IfMatch manifest")
    manifests = {}
    for entry in ALL_ENTRIES:
        package, activity = entry.rsplit(":", 1)
        if entry == IF_MATCH_ENTRY:
            name, raw = "if-match.workspace.json", frozen["if-match.workspace.json"]
        elif activity == "Report":
            name = "literal-text-report.workspace.json" if entry in ENTRIES else "literal-cut-report.workspace.json"
            raw = frozen[name]
        else:
            name = activity.lower() + ".workspace.json"
            sources = PORT_SOURCES if entry in ENTRIES else CUT_PORT_SOURCES
            raw = encode({"schema": "gooo/package-workspace-manifest/v1",
                          "entry": {"package_path": package, "activity": activity},
                          "packages": [{"path": "go/strings", "sources": list(sources)}]}) + b"\n"
        manifests[entry] = (name, raw)
    require(len(manifests) == 8 and len({name for name, _ in manifests.values()}) == 8,
            "distinct complete manifest roster")
    return manifests


def observe(root, out, compiler):
    out.mkdir(parents=True, exist_ok=False)
    frozen = {name: (root / name).read_bytes() for name in FROZEN_FILES}
    prepared = trusted_manifests(frozen)
    for name, raw in frozen.items():
        path = out / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(raw)
    manifests = {}
    for entry, (name, raw) in prepared.items():
        path = out / name
        if name not in frozen:
            path.write_bytes(raw)
        require(path.read_bytes() == raw, "copied original manifest bytes")
        manifests[entry] = (path, raw)
    lock = loads_receipt((root / "upstream.json").read_bytes())
    env = clean_environment()
    executable_binding = binding(compiler)
    version = subprocess.run([str(compiler), "version", "--build", "--json"], capture_output=True, timeout=30, env=env)
    (out / "compiler.json").write_bytes(version.stdout)
    (out / "compiler.stderr.txt").write_bytes(version.stderr)
    require(version.returncode == 0, "compiler identity command")
    validate_identity(loads_receipt(version.stdout), lock)
    cases = list(negative_inputs())
    require(len(cases) == 224 and len({case["name"] for case in cases}) == 224
            and all(sum(case["entry"] == entry for case in cases) == 28 for entry in ALL_ENTRIES),
            "frozen legacy84 plus Cut112 and IfMatch28 rejection request roster")
    rows, failures = [], []
    for case in cases:
        fixture = out / (case["name"] + ".inputs.json")
        fixture.write_bytes(case["raw"])
        try:
            validate_inputs(case["raw"], case["entry"])
        except (ValueError, TypeError, KeyError) as error:
            preflight = str(error)
        else:
            failures.append(case["name"] + ": malformed fixture passed strict preflight")
            break
        manifest_path, manifest_raw = manifests[case["entry"]]
        command = [str(compiler), "package", "execute", "--json", "--inputs", str(fixture), str(manifest_path)]
        timed_out = False
        try:
            result = subprocess.run(command, capture_output=True, timeout=30, env=env)
            stdout, stderr, exit_code = result.stdout, result.stderr, result.returncode
        except subprocess.TimeoutExpired as error:
            timed_out = True
            stdout, stderr, exit_code = error.stdout or b"", error.stderr or b"", None
        receipt_path = out / (case["name"] + ".receipt.json")
        stderr_path = out / (case["name"] + ".stderr.txt")
        receipt_path.write_bytes(stdout)
        stderr_path.write_bytes(stderr)
        row = {"case": case["name"], "entry": case["entry"], "position": case["position"],
               "expected_rejection_stage": case["expected_rejection_stage"], "expected_diagnostic": case["expected_diagnostic"],
               "trusted_preflight_error": preflight, "command": command, "exit_code": exit_code, "timed_out": timed_out,
               "input_binding": binding(fixture), "manifest_binding": binding(manifest_path),
               "receipt_binding": binding(receipt_path), "stderr_binding": binding(stderr_path)}
        rows.append(row)
        try:
            row.update(inspect_rejection(case, stdout, exit_code, timed_out, manifest_path, manifest_raw))
        except (ValueError, KeyError, TypeError) as error:
            failures.append(case["name"] + ": " + str(error))
            break
    require(all((root / name).read_bytes() == raw and (out / name).read_bytes() == raw for name, raw in frozen.items()),
            "frozen source changed during rejection observations")
    require(all(path.read_bytes() == raw for path, raw in manifests.values()), "generated manifest changed during observations")
    require(binding(compiler) == executable_binding, "compiler executable changed during observations")
    summary = {"schema": "gooo/literal-text-input-rejections/v1", "status": "PASS" if not failures and len(rows) == len(cases) else "FAIL",
               "planned_framework_inputs": len(cases), "planned_by_entry": {entry: sum(c["entry"] == entry for c in cases) for entry in ALL_ENTRIES},
               "observed_framework_rejections": sum(r.get("rejection_verified") is True for r in rows),
               "observed_by_entry": {entry: sum(r["entry"] == entry and r.get("rejection_verified") is True for r in rows) for entry in ALL_ENTRIES},
               "go_reference_cases": 0, "compiler_revision": COMPILER, "compiler_version": VERSION, "go_version": GO,
               "compiler_identity_binding": binding(out / "compiler.json"), "compiler_executable_binding": executable_binding,
               "source_bindings": [binding(out / name) for name in frozen],
               "manifest_bindings": [binding(path) for path, _ in manifests.values()], "cases": rows, "failures": failures,
               "scope": "Finite malformed request/domain observations: original84 for Index/Count/literal-text Report followed by112 for Cut/CutPrefix/CutSuffix/literal-cut Report, then28 for IfMatchCondition, with separate exact source closures; no Go comparisons. Absent native stages/counters/builds/runs remain NOT_REPORTED. Wire-invalid UTF-8 and surrogate replacement are not included or claimed as rejected. Stop on the first incompatibility."}
    (out / "summary.json").write_text(json.dumps(summary, ensure_ascii=False, indent=2) + "\n")
    require(summary["status"] == "PASS", "rejection observation failure: " + "; ".join(failures))
    return summary


def main():
    if len(sys.argv) != 2:
        raise SystemExit("Usage: python3 tools/check-literal-text-inputs.py NEW_OUTPUT_DIRECTORY")
    summary = observe(Path.cwd(), Path(sys.argv[1]).resolve(), Path(".gooo").resolve())
    print("PASS: " + str(summary["observed_framework_rejections"]) + " malformed literal Text requests; Go reference cases 0; absent runtime NOT_REPORTED")


if __name__ == "__main__":
    main()
