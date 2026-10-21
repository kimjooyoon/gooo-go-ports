#!/usr/bin/env python3
"""Observe 56 fresh malformed requests for LastIndex and the v1 marker report.

Reuse the reviewed request templates and receipt inspector without modifying
their legacy entry roster or observations. No Go-reference case is counted.
"""
import argparse
import importlib.util
import json
from pathlib import Path
import subprocess
import sys

TOOLS = Path(__file__).resolve().parent
sys.path.insert(0, str(TOOLS))
from receipt_json import loads_receipt

SPEC = importlib.util.spec_from_file_location("legacy_literal_rejections", TOOLS / "check-literal-text-inputs.py")
LEGACY = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(LEGACY)

ENTRIES = ("go/strings:LastIndex", "app/literal-final-marker-report:Report")
SOURCES = ("ports/strings/search.gooo", "ports/strings/literal.gooo",
           "ports/strings/last_index.gooo", "examples/literal-final-marker-report/main.gooo")
MANIFEST = "literal-final-marker-report.workspace.json"
OBSERVER_SOURCES = ("tools/check-last-index-inputs.py", "tools/check-literal-text-inputs.py", "tools/receipt_json.py")


def require(condition, message):
    if not condition:
        raise ValueError(message)


def negative_inputs():
    replacements = {"go/strings:Index": (ENTRIES[0], "lastindex"),
                    "app/literal-text-report:Report": (ENTRIES[1], "finalreport")}
    for old in LEGACY.negative_inputs():
        if old["entry"] not in replacements:
            continue
        entry, prefix = replacements[old["entry"]]
        case = dict(old)
        case["entry"] = entry
        case["name"] = prefix + "-" + old["name"].split("-", 1)[1]
        case["raw"] = old["raw"].replace(old["entry"].encode(), entry.encode())
        yield case


def validate_inputs(raw, entry):
    require(entry in ENTRIES, "untrusted entry")
    value = loads_receipt(raw.decode("utf-8", errors="strict"))
    require(type(value) is dict and set(value) == {"schema", "inputs"}, "input envelope fields")
    require(value["schema"] == LEGACY.SCHEMA, "input schema")
    rows = value["inputs"]
    require(type(rows) is list and 1 <= len(rows) <= 128, "input row count")
    keys = {entry + ".input0", entry + ".input1"}
    for row in rows:
        require(type(row) is dict and set(row) == keys, "exact input ports")
        for key in keys:
            require(type(row[key]) is str, "explicit Text scalar")
            require(len(row[key].encode("utf-8", errors="strict")) <= 1024, "decoded Text exceeds 1024 UTF-8 bytes")
    return rows


def trusted_manifest_bytes(raw):
    expected = {"schema": "gooo/package-workspace-manifest/v1",
                "entry": {"package_path": "app/literal-final-marker-report", "activity": "Report"},
                "packages": [{"path": "go/strings", "sources": list(SOURCES[:3])},
                             {"path": "app/literal-final-marker-report", "sources": [SOURCES[3]]}]}
    require(loads_receipt(raw) == expected, "exact v1 final-marker workspace closure")
    port = {"schema": expected["schema"], "entry": {"package_path": "go/strings", "activity": "LastIndex"},
            "packages": [{"path": "go/strings", "sources": [SOURCES[0], SOURCES[2]]}]}
    return {ENTRIES[0]: ("last-index.workspace.json", LEGACY.encode(port) + b"\n"),
            ENTRIES[1]: (MANIFEST, raw)}


def observe(root, out, compiler, go_tool):
    require(root == TOOLS.parent, "root must identify this observer's checkout")
    out.mkdir(parents=True, exist_ok=False)
    frozen = {name: (root / name).read_bytes() for name in (*SOURCES, MANIFEST, *OBSERVER_SOURCES, "upstream.json")}
    prepared = trusted_manifest_bytes(frozen[MANIFEST])
    for name in SOURCES:
        path = out / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(frozen[name])
    manifests = {}
    for entry, (name, raw) in prepared.items():
        path = out / name
        path.write_bytes(raw)
        manifests[entry] = (path, raw)
    env = LEGACY.clean_environment()
    compiler_binding, go_binding = LEGACY.binding(compiler), LEGACY.binding(go_tool)
    version = subprocess.run([str(compiler), "version", "--build", "--json"], capture_output=True, timeout=30, env=env)
    (out / "compiler.json").write_bytes(version.stdout)
    (out / "compiler.stderr.txt").write_bytes(version.stderr)
    require(version.returncode == 0, "compiler identity command")
    LEGACY.validate_identity(loads_receipt(version.stdout), loads_receipt(frozen["upstream.json"]))
    go_version = subprocess.run([str(go_tool), "version"], capture_output=True, timeout=30, env=env)
    (out / "go-version.txt").write_bytes(go_version.stdout)
    require(go_version.returncode == 0 and go_version.stdout.decode().startswith("go version go1.27.3 "), "explicit Go1.27.3 executable")
    cases = list(negative_inputs())
    require(len(cases) == 56 and len({case["name"] for case in cases}) == 56
            and all(sum(case["entry"] == entry for case in cases) == 28 for entry in ENTRIES), "exact two-entry56request roster")
    rows, failures = [], []
    for case in cases:
        fixture = out / (case["name"] + ".inputs.json")
        fixture.write_bytes(case["raw"])
        try:
            validate_inputs(case["raw"], case["entry"])
        except (ValueError, TypeError, KeyError, UnicodeError) as error:
            preflight = str(error)
        else:
            failures.append(case["name"] + ": malformed fixture passed strict preflight")
            break
        manifest_path, manifest_raw = manifests[case["entry"]]
        command = [str(compiler), "package", "execute", "--json", "--go", str(go_tool),
                   "--inputs", str(fixture), str(manifest_path)]
        timed_out = False
        try:
            result = subprocess.run(command, cwd=out, capture_output=True, timeout=30, env=env)
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
               "input_binding": LEGACY.binding(fixture), "manifest_binding": LEGACY.binding(manifest_path),
               "receipt_binding": LEGACY.binding(receipt_path), "stderr_binding": LEGACY.binding(stderr_path)}
        rows.append(row)
        try:
            row.update(LEGACY.inspect_rejection(case, stdout, exit_code, timed_out, manifest_path, manifest_raw))
        except (ValueError, KeyError, TypeError) as error:
            failures.append(case["name"] + ": " + str(error))
            break
    require(all((root / name).read_bytes() == raw for name, raw in frozen.items()), "observer source changed during observation")
    require(all((out / name).read_bytes() == frozen[name] for name in SOURCES), "copied source changed during observation")
    require(all(path.read_bytes() == raw for path, raw in manifests.values()), "manifest changed during observation")
    require(LEGACY.binding(compiler) == compiler_binding and LEGACY.binding(go_tool) == go_binding, "executable changed during observation")
    summary = {"schema": "gooo/last-index-input-rejections/v1", "status": "PASS" if not failures and len(rows) == 56 else "FAIL",
               "planned_framework_inputs": 56, "planned_by_entry": {entry: 28 for entry in ENTRIES},
               "observed_framework_rejections": sum(row.get("rejection_verified") is True for row in rows),
               "observed_by_entry": {entry: sum(row["entry"] == entry and row.get("rejection_verified") is True for row in rows) for entry in ENTRIES},
               "go_reference_cases": 0, "compiler_revision": LEGACY.COMPILER, "compiler_version": LEGACY.VERSION, "go_version": LEGACY.GO,
               "compiler_identity_binding": LEGACY.binding(out / "compiler.json"), "compiler_executable_binding": compiler_binding,
               "go_executable_binding": go_binding, "source_bindings": [LEGACY.binding(out / name) for name in SOURCES],
               "observer_source_bindings": [LEGACY.binding(root / name) for name in OBSERVER_SOURCES],
               "upstream_binding": LEGACY.binding(root / "upstream.json"),
               "manifest_bindings": [LEGACY.binding(path) for path, _ in manifests.values()],
               "cases": rows, "failures": failures,
               "scope": "56 fresh malformed/domain requests for LastIndex and the separate v1 Report, with exact source/manifests/input bindings. No Go comparison or native execution is inferred from an absent Result. Runtime stages/counters stay NOT_REPORTED. Invalid wire UTF-8 and surrogate replacement are outside this roster. Legacy observer and its observed records remain unchanged."}
    (out / "summary.json").write_text(json.dumps(summary, ensure_ascii=False, indent=2) + "\n")
    require(summary["status"] == "PASS", "rejection observation failure: " + "; ".join(failures))
    return summary


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("root", "compiler", "out"):
        parser.add_argument("--" + name, type=Path, required=True)
    parser.add_argument("--go", dest="go_tool", type=Path, required=True)
    args = parser.parse_args()
    summary = observe(args.root.resolve(), args.out.resolve(), args.compiler.resolve(), args.go_tool.resolve())
    print("PASS: " + str(summary["observed_framework_rejections"]) + " fresh LastIndex/v1 Report rejections; Go reference cases0; absent runtime NOT_REPORTED")


if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError, subprocess.SubprocessError) as error:
        print(str(error), file=sys.stderr)
        raise SystemExit(1)
