#!/usr/bin/env python3
"""Bind the fourteen input-only FitsCrop observations and their saved replay."""
import hashlib
import json
from pathlib import Path
import sys

from receipt_json import loads_receipt

ENTRY = "example/fits-crop:FitsCrop"
ACTIVITY = "gooo://example/fits-crop/fits-crop"
RECTANGLE = "gooo://go/image/rectangle"
BOOLEAN = "gooo://std/boolean"
COORDINATES = ["min_x", "min_y", "max_x", "max_y"]
LO, HI = -(2**63), 2**63 - 1
SOURCES = {"go/image": ["ports/image/geometry.gooo"],
           "example/fits-crop": ["examples/fits-crop/main.gooo"]}
# Reviewed literal answers, independent of native generation and the Go oracle.
# See the separately reviewed fourteen-row semantic proposal and example table.
EXPECTED_ROWS = [True, False, False, False, False, False, False,
                 False, False, False, False, False, True, False]


def require(condition, message):
    if not condition:
        raise ValueError(message)


def digest(raw):
    return "sha256:" + hashlib.sha256(raw).hexdigest()


def exact(left, right):
    return json.dumps(left, sort_keys=True) == json.dumps(right, sort_keys=True)


def fields(value):
    return [{"id": RECTANGLE + "/" + name.replace("_", "-"),
             "name": name, "value": value[name]} for name in COORDINATES]


def source_digest(path):
    text = path.read_bytes().decode("utf-8")
    encoded = (json.dumps(text, ensure_ascii=False).replace("&", "\\u0026")
               .replace("<", "\\u003c").replace(">", "\\u003e")
               .replace("\u2028", "\\u2028").replace("\u2029", "\\u2029").encode())
    return digest(encoded)


def validate_rectangle(value):
    require(type(value) is dict and set(value) == set(COORDINATES), "rectangle fields")
    require(all(type(v) is int and LO <= v <= HI for v in value.values()), "exact int64 inputs")


def validate_inputs(raw, count=None):
    document = loads_receipt(raw)
    require(type(document) is dict and set(document) == {"schema", "inputs"}, "inputs document")
    require(document["schema"] == "gooo/body-composition-inputs/v1", "inputs schema")
    rows = document["inputs"]
    require(type(rows) is list and len(rows) > 0, "input rows")
    if count is not None:
        require(len(rows) == count, "documented input count")
    for row in rows:
        require(type(row) is dict and set(row) == {ENTRY + ".input0", ENTRY + ".input1"}, "input ports")
        for value in row.values():
            validate_rectangle(value)
    return rows


def scalar_fields(value):
    require(value is None or (type(value) is list and len(value) == 0), "Boolean scalar fields")


def boolean_result(value, expected):
    require(type(value) is bool, "Boolean output type")
    require(value is expected, "documented Boolean output")


def check_export(package, name, activity, inputs, output):
    exports = [e for e in package["exports"] if e.get("name") == name or e.get("id") == activity]
    require(len(exports) == 1, "export activity count")
    e = exports[0]
    require(e.get("name") == name and e.get("id") == activity and e.get("kind") == "activity",
            "export activity identity")
    require(exact(e.get("input_types"), inputs) and e.get("output_type") == output,
            "export type identity")


def check_bytes(root, execution_raw, replay_raw):
    try:
        _check_bytes(root, execution_raw, replay_raw)
    except (KeyError, IndexError, TypeError, AttributeError) as error:
        raise ValueError("malformed receipt structure") from error


def _check_bytes(root, execution_raw, replay_raw):
    fixture_raw = (root / "examples/fits-crop/inputs.json").read_bytes()
    fixture = validate_inputs(fixture_raw, len(EXPECTED_ROWS))
    manifest_raw = (root / "fits-crop.workspace.json").read_bytes()
    manifest = loads_receipt(manifest_raw)
    require(manifest["schema"] == "gooo/package-workspace-manifest/v1", "manifest schema")
    require(exact(manifest["entry"], {"package_path": "example/fits-crop", "activity": "FitsCrop"}), "manifest entry")
    require(len(manifest["packages"]) == 2
            and {p["path"]: p["sources"] for p in manifest["packages"]} == SOURCES, "manifest source roster")
    lock = loads_receipt((root / "upstream.json").read_bytes())
    hashes = {name: source_digest(root / name) for names in SOURCES.values() for name in names}
    execution, replay = loads_receipt(execution_raw), loads_receipt(replay_raw)
    require(replay.get("replayed_from_sha256") == digest(execution_raw), "saved replay binding")
    require(exact(replay["result"]["replay"]["model_calls"], 0), "replay model calls")
    require(execution.get("replayed_from_sha256") in (None, "")
            and execution["result"].get("replay") is None, "unexpected execution replay metadata")
    for receipt in (execution, replay):
        require(receipt["schema"] == "gooo/workspace-body-execution-receipt/v1", "receipt schema")
        require(receipt["decision"] == "OBSERVED" and not receipt.get("error"), "observed execution")
        require(receipt["manifest_digest"] == digest(manifest_raw), "manifest digest")
        require(receipt["inputs_digest"] == digest(fixture_raw), "input digest")
        require(receipt.get("cases_digest") in (None, ""), "input-only case file")
        result = receipt["result"]
        entry = result["program"]["entry"]
        require(all(exact(entry.get(key), value) for key, value in manifest["entry"].items()), "entry identity")
        packages = result["program"]["workspace_image"]["packages"]
        require(len(packages) == 2 and {p["path"] for p in packages} == set(SOURCES), "workspace package roster")
        for package in packages:
            names = SOURCES[package["path"]]
            require(len(package["sources"]) == len(names)
                    and {s["filename"] for s in package["sources"]} == set(names), "workspace source roster")
            for source in package["sources"]:
                require(source["source_digest"] == hashes[source["filename"]], "source digest")
            if package["path"] == "example/fits-crop":
                check_export(package, "FitsCrop", ACTIVITY, [RECTANGLE] * 2, BOOLEAN)
            else:
                check_export(package, "RectangleIn", "gooo://go/image/rectangle-in", [RECTANGLE] * 2, BOOLEAN)
                check_export(package, "RectangleEmpty", "gooo://go/image/rectangle-empty", [RECTANGLE], BOOLEAN)
        runtime, composition = result["runtime"], result["composition"]
        require(runtime["schema"] == "gooo/body-composition-runtime/v1", "runtime schema")
        require(runtime["stage"] == composition["stage"] == "COMPLETE" and runtime.get("failure") is None,
                "complete execution")
        require(runtime["producer_source_sha"] == lock["compiler_revision"], "compiler identity")
        version = runtime["go_version"].split()
        require(len(version) == 4 and version[:3] == ["go", "version", lock["go_version"]]
                and version[3].split("/")[-1] in ("amd64", "arm64"), "64-bit Go toolchain identity")
        require(runtime["generated_sha256"] == composition["generated_sha256"]
                == digest(composition["source"].encode()), "generated source binding")
        require(len(composition["steps"]) == 1, "composition step count")
        report = composition["steps"][0]["generation"]["report"]
        require(report["activity_id"] == ACTIVITY, "generation activity identity")
        lowered = entry.get("lowered_name")
        require(type(lowered) is str and bool(lowered.strip()) and lowered == report.get("activity"),
                "entry lowered activity binding")
        require(report["compiler_source_sha"] == lock["compiler_revision"], "generation compiler identity")
        for name in ("finite_total", "finite_passed", "model_calls"):
            require(type(runtime[name]) is int and runtime[name] == 0, "input-only runtime counters")
        require(runtime["projection_replayed"] is True and runtime["runtime_replayed"] is True, "runtime replay checks")
        require(len(runtime["traces"]) == len(fixture), "trace count")
        for index, (supplied, trace, expected) in enumerate(zip(fixture, runtime["traces"], EXPECTED_ROWS)):
            require(type(trace["case_index"]) is int and trace["case_index"] == index
                    and len(trace["deliveries"]) == 1, "case order")
            observed = trace["deliveries"][0]
            require(observed["activity_id"] == ACTIVITY and "input" not in observed, "delivery identity")
            scalar_fields(observed.get("input_fields"))
            original = [supplied[ENTRY + ".input" + str(i)] for i in range(2)]
            require(exact(observed["inputs"], [
                {"port": "input" + str(i), "entity_id": RECTANGLE, "value": value, "fields": fields(value)}
                for i, value in enumerate(original)]), "exact original input records")
            boolean_result(observed["actual"], expected)
            scalar_fields(observed.get("actual_fields"))
            require(observed.get("expected") is None and observed.get("passed") is None, "input-only case score")
    require(exact(execution["result"]["runtime"]["traces"], replay["result"]["runtime"]["traces"]), "saved replay traces")
    require(execution["result"]["composition"]["source"] == replay["result"]["composition"]["source"], "saved replay generated source")


def check(root, execution_path, replay_path):
    check_bytes(root, execution_path.read_bytes(), replay_path.read_bytes())


def main():
    if len(sys.argv) != 3:
        raise SystemExit("Usage: python3 tools/check-fits-crop.py EXECUTION REPLAY")
    check(Path.cwd(), Path(sys.argv[1]), Path(sys.argv[2]))
    print("PASS: checker verified 14 OBSERVED FitsCrop Boolean outputs, two source bindings and saved replay; finite scores remain zero")


if __name__ == "__main__":
    main()
