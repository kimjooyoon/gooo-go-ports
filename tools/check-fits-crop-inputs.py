#!/usr/bin/env python3
"""Observe malformed FitsCrop CLI inputs separately from Go reference cases."""
import copy
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys

from receipt_json import loads_receipt

SPEC = importlib.util.spec_from_file_location("fits_crop_checker", Path(__file__).with_name("check-fits-crop.py"))
CHECK = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(CHECK)


def negative_inputs():
    base = {"min_x": 1, "min_y": 2, "max_x": 5, "max_y": 6}
    for position in range(2):
        variants = []
        for field in CHECK.COORDINATES:
            variants.append(("missing-" + field, {k: v for k, v in base.items() if k != field},
                             'requires field "' + field + '"', None))
            for label, value in [("null", None), ("false", False), ("true", True),
                                 ("text", "1"), ("fraction", 1.5), ("float", 1.0)]:
                variants.append((label + "-" + field, dict(base, **{field: value}),
                                 "explicit Integer scalar required" if value is None else "Integer requires an exact int64", None))
            for label, value in [("high", 2**63), ("low", -(2**63) - 1)]:
                variants.append((label + "-" + field, dict(base, **{field: value}), "Integer requires an exact int64", None))
            variants.append(("duplicate-" + field, base, "duplicate or invalid JSON key", field))
        variants.append(("extra", dict(base, other=1), "contains an undeclared field", None))
        for label, value in [("null-record", None), ("array-record", []), ("text-record", "record"), ("integer-record", 0)]:
            variants.append((label, value, "explicit JSON object required", None))
        for name, value, diagnostic, duplicate in variants:
            values = [copy.deepcopy(base), copy.deepcopy(base)]
            values[position] = value
            encoded = [json.dumps(v, separators=(",", ":")) for v in values]
            if duplicate:
                token = json.dumps(duplicate) + ":" + str(base[duplicate])
                encoded[position] = encoded[position].replace(token, token + "," + token, 1)
            raw = ('{"schema":"gooo/body-composition-inputs/v1","inputs":[{' +
                   json.dumps(CHECK.ENTRY + ".input0") + ':' + encoded[0] + ',' +
                   json.dumps(CHECK.ENTRY + ".input1") + ':' + encoded[1] + '}]}\n').encode()
            yield {"name": name + "-input" + str(position), "position": position,
                   "duplicate": duplicate is not None, "diagnostic": diagnostic, "raw": raw}


def clean_environment():
    env = os.environ.copy()
    for key in list(env):
        if key.startswith("GOOO_") or key in ("OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY", "HF_TOKEN"):
            env.pop(key, None)
    env.update(GOTOOLCHAIN="local", GOENV="off", GOWORK="off", GOPROXY="off", GOSUMDB="off")
    return env


def binding(path):
    raw = path.read_bytes()
    return {"path": str(path), "bytes": len(raw), "sha256": CHECK.digest(raw)}


def observe(root, out, compiler):
    out.mkdir(parents=True, exist_ok=False)
    source_paths = [name for names in CHECK.SOURCES.values() for name in names]
    frozen = {name: (root / name).read_bytes() for name in ["fits-crop.workspace.json", *source_paths]}
    for name, raw in frozen.items():
        path = out / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(raw)
    manifest_path = out / "fits-crop.workspace.json"
    lock = loads_receipt((root / "upstream.json").read_bytes())
    env = clean_environment()
    version = subprocess.run([str(compiler), "version", "--build", "--json"], capture_output=True, timeout=30, env=env)
    (out / "compiler.json").write_bytes(version.stdout)
    (out / "compiler.stderr.txt").write_bytes(version.stderr)
    CHECK.require(version.returncode == 0, "compiler identity command")
    build = loads_receipt(version.stdout)
    CHECK.require(build["compiler_source_sha"] == lock["compiler_revision"]
                  and build["go_version"] == lock["go_version"] and build["vcs_modified"] == "false", "clean pinned compiler identity")
    cases = list(negative_inputs())
    rows, failures = [], []
    for case in cases:
        raw, prefix = case["raw"], case["name"]
        fixture = out / (prefix + ".inputs.json")
        fixture.write_bytes(raw)
        try:
            CHECK.validate_inputs(raw, 1)
        except (ValueError, TypeError, KeyError) as error:
            preflight = str(error)
        else:
            failures.append(prefix + ": malformed fixture passed trusted preflight")
            break
        command = [str(compiler), "package", "execute", "--json", "--inputs", str(fixture), str(manifest_path)]
        timed_out = False
        try:
            result = subprocess.run(command, capture_output=True, timeout=30, env=env)
            stdout, stderr, exit_code = result.stdout, result.stderr, result.returncode
        except subprocess.TimeoutExpired as error:
            timed_out = True
            stdout, stderr, exit_code = error.stdout or b"", error.stderr or b"", None
        receipt_path, stderr_path = out / (prefix + ".receipt.json"), out / (prefix + ".stderr.txt")
        receipt_path.write_bytes(stdout)
        stderr_path.write_bytes(stderr)
        row = {"case": prefix, "position": case["position"], "trusted_preflight_error": preflight,
               "command": command, "exit_code": exit_code, "timed_out": timed_out, "input_binding": binding(fixture),
               "receipt_binding": binding(receipt_path), "stderr_binding": binding(stderr_path)}
        rows.append(row)
        try:
            CHECK.require(not timed_out, "actual CLI timed out")
            receipt = loads_receipt(stdout)
            CHECK.require(exit_code == 1 and receipt["decision"] == "FAIL_CLOSED", "actual CLI rejection")
            CHECK.require(case["diagnostic"] in receipt["error"], "actual input diagnostic")
            if not case["duplicate"]:
                CHECK.require(".input" + str(case["position"]) in receipt["error"], "actual input position")
                CHECK.require(receipt["inputs_digest"] == CHECK.digest(raw), "original malformed input binding")
                CHECK.require(receipt["manifest_digest"] == CHECK.digest(frozen["fits-crop.workspace.json"]), "original manifest binding")
            returned = receipt.get("result")
            runtime = returned.get("runtime") if type(returned) is dict else None
            row.update(decision=receipt["decision"], error=receipt["error"], result_present="result" in receipt,
                       runtime_present=type(runtime) is dict,
                       actual_runtime_stage=runtime.get("stage") if type(runtime) is dict else None,
                       observed_counters={k: runtime[k] for k in ("finite_total", "finite_passed", "model_calls")
                                          if type(runtime) is dict and k in runtime},
                       observed_build=runtime.get("build") if type(runtime) is dict else None,
                       observed_runs=runtime.get("runs") if type(runtime) is dict else None,
                       native_execution_claim="No native execution or zero-counter claim inferred from missing runtime metadata")
            if type(runtime) is dict:
                CHECK.require(runtime.get("stage") not in ("BUILD", "EXECUTE_1", "EXECUTE_2", "COMPLETE"),
                              "malformed input reached native execution stage")
                build_observation = runtime.get("build")
                CHECK.require(type(build_observation) is not dict or build_observation.get("started") is not True,
                              "malformed input started a native build")
                CHECK.require(not any(type(run) is dict and run.get("started") is True for run in runtime.get("runs", []) or []),
                              "malformed input started a native process")
            row["rejection_verified"] = True
        except (ValueError, KeyError, TypeError) as error:
            failures.append(prefix + ": " + str(error))
            break
    CHECK.require(all((root / name).read_bytes() == raw and (out / name).read_bytes() == raw
                      for name, raw in frozen.items()), "frozen source changed during rejection probes")
    summary = {"status": "PASS" if not failures and len(rows) == len(cases) else "FAIL",
               "planned_framework_inputs": len(cases), "observed_framework_rejections": sum(r.get("rejection_verified") is True for r in rows),
               "go_reference_cases": 0, "compiler_revision": lock["compiler_revision"],
               "compiler_identity_binding": binding(out / "compiler.json"),
               "source_bindings": [binding(out / name) for name in frozen], "cases": rows, "failures": failures,
               "scope": "Malformed-record observations are separate from Go oracle successes. Stage/counters are recorded only when returned; missing runtime does not fabricate a zero model-call or native-execution claim. Eight duplicate-wire cases have strict preflight rejection and separately observed CLI behavior. Stop on the first incompatibility."}
    (out / "summary.json").write_text(json.dumps(summary, indent=2) + "\n")
    CHECK.require(summary["status"] == "PASS", "rejection probe failure: " + "; ".join(failures))
    return summary


def main():
    if len(sys.argv) != 2:
        raise SystemExit("Usage: python3 tools/check-fits-crop-inputs.py NEW_OUTPUT_DIRECTORY")
    summary = observe(Path.cwd(), Path(sys.argv[1]).resolve(), Path(".gooo").resolve())
    print("PASS: " + str(summary["observed_framework_rejections"]) + " malformed FitsCrop CLI inputs; Go reference cases 0; actual stages retained")


if __name__ == "__main__":
    main()
