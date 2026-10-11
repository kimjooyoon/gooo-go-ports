#!/usr/bin/env python3
"""Check the twelve documented DragCropPlan inputs and their saved replay."""
import hashlib
import json
from pathlib import Path
import sys

from receipt_json import loads_receipt


ENTRY = "example/crop-planner:DragCropPlan"
ACTIVITY = "gooo://example/crop-planner/drag-crop-plan"
RECTANGLE = "gooo://go/image/rectangle"
RESULT = "gooo://example/crop-planner/result"
COORDINATES = ["min_x", "min_y", "max_x", "max_y"]
RESULT_NAMES = COORDINATES + ["has_pixels"]
LO, HI = -(2**63), 2**63 - 1
SOURCES = {
    "go/image": ["ports/image/geometry.gooo", "ports/image/canon.gooo"],
    "example/crop-planner": ["examples/crop-planner/crop_planner.gooo",
                             "examples/crop-planner/drag_crop.gooo"],
}
# Independent literal results from docs/drag-crop.md, not a geometry algorithm.
EXPECTED_ROWS = [
    *[[100, 100, 200, 200, True] for _ in range(4)],
    [0, 40, 200, 480, True],
    *[[0, 0, 0, 0, False] for _ in range(3)],
    [-8, -30, 5, 4, True],
    [HI - 1, HI - 1, HI, HI, True],
    [LO, LO, LO + 1, LO + 1, True],
    [9007199254740993, -9007199254740992,
     9007199254740994, -9007199254740991, True],
]


def require(condition, message):
    if not condition:
        raise ValueError(message)


def digest(raw):
    return "sha256:" + hashlib.sha256(raw).hexdigest()


def exact(left, right):
    # Keep int64 exact and distinguish bool/float from integer.
    return json.dumps(left, sort_keys=True) == json.dumps(right, sort_keys=True)


def fields(entity, names, value):
    return [{"id": entity + "/" + name.replace("_", "-"), "name": name,
             "value": value[name]} for name in names]


def source_digest(path):
    # Workspace source_digest is the digest of Go's JSON-encoded source string.
    text = path.read_bytes().decode("utf-8")
    encoded = (json.dumps(text, ensure_ascii=False).replace("&", "\\u0026")
               .replace("<", "\\u003c").replace(">", "\\u003e")
               .replace("\u2028", "\\u2028").replace("\u2029", "\\u2029").encode())
    return digest(encoded)


def check(root, execution_path, replay_path):
    fixture_raw = (root / "examples/crop-planner/drag_inputs.json").read_bytes()
    fixture_document = loads_receipt(fixture_raw)
    require(fixture_document["schema"] == "gooo/body-composition-inputs/v1", "inputs schema")
    fixture = fixture_document["inputs"]
    manifest_raw = (root / "drag-crop.workspace.json").read_bytes()
    manifest = loads_receipt(manifest_raw)
    require(manifest["schema"] == "gooo/package-workspace-manifest/v1", "manifest schema")
    require(exact(manifest["entry"], {"package_path": "example/crop-planner",
                                      "activity": "DragCropPlan"}), "manifest entry")
    require(len(manifest["packages"]) == len(SOURCES), "manifest package count")
    require({p["path"]: p["sources"] for p in manifest["packages"]} == SOURCES,
            "manifest source roster")
    lock = loads_receipt((root / "upstream.json").read_bytes())
    execution_raw = execution_path.read_bytes()
    execution = loads_receipt(execution_raw)
    replay = loads_receipt(replay_path.read_bytes())
    require(replay["replayed_from_sha256"] == digest(execution_raw), "saved replay binding")
    require(exact(replay["result"]["replay"]["model_calls"], 0), "replay model calls")
    require(len(fixture) == len(EXPECTED_ROWS) == 12, "documented input count")
    hashes = {name: source_digest(root / name) for names in SOURCES.values() for name in names}
    for supplied in fixture:
        require(set(supplied) == {ENTRY + ".input0", ENTRY + ".input1"}, "input ports")
        for value in supplied.values():
            require(isinstance(value, dict) and set(value) == set(COORDINATES), "rectangle fields")
            require(all(type(v) is int and LO <= v <= HI for v in value.values()), "exact int64 inputs")
    for receipt in (execution, replay):
        require(receipt["schema"] == "gooo/workspace-body-execution-receipt/v1", "receipt schema")
        require(receipt["decision"] == "OBSERVED" and not receipt.get("error"), "observed execution")
        require(receipt["manifest_digest"] == digest(manifest_raw), "manifest digest")
        require(receipt["inputs_digest"] == digest(fixture_raw), "input digest")
        result = receipt["result"]
        for key, value in manifest["entry"].items():
            require(result["program"]["entry"][key] == value, "entry identity")
        packages = result["program"]["workspace_image"]["packages"]
        require(len(packages) == len(SOURCES) and {p["path"] for p in packages} == set(SOURCES),
                "workspace package roster")
        for package in packages:
            names = SOURCES[package["path"]]
            require(len(package["sources"]) == len(names), "workspace source count")
            require({s["filename"] for s in package["sources"]} == set(names), "workspace source roster")
            for source in package["sources"]:
                require(source["source_digest"] == hashes[source["filename"]], "source digest")
            if package["path"] == "example/crop-planner":
                exported = [e for e in package["exports"]
                            if e.get("name") == "DragCropPlan" or e.get("id") == ACTIVITY]
                require(len(exported) == 1 and exported[0].get("name") == "DragCropPlan"
                        and exported[0].get("id") == ACTIVITY and exported[0].get("kind") == "activity",
                        "export activity identity")
                require(exported[0]["input_types"] == [RECTANGLE, RECTANGLE], "export input identities")
                require(exported[0]["output_type"] == RESULT, "export output identity")
        runtime, composition = result["runtime"], result["composition"]
        require(runtime["schema"] == "gooo/body-composition-runtime/v1", "runtime schema")
        require(runtime["stage"] == composition["stage"] == "COMPLETE", "complete execution")
        require(runtime.get("failure") is None, "runtime failure")
        require(runtime["producer_source_sha"] == lock["compiler_revision"], "compiler identity")
        version = runtime["go_version"].split()
        require(len(version) == 4 and version[:3] == ["go", "version", lock["go_version"]]
                and version[3].split("/")[-1] in ("amd64", "arm64"), "64-bit Go toolchain identity")
        require(runtime["generated_sha256"] == composition["generated_sha256"]
                == digest(composition["source"].encode()), "generated source binding")
        require(len(composition["steps"]) == 1, "composition step count")
        report = composition["steps"][0]["generation"]["report"]
        require(report["activity_id"] == ACTIVITY, "generation activity identity")
        require(report["compiler_source_sha"] == lock["compiler_revision"], "generation compiler identity")
        for name in ("finite_total", "finite_passed", "model_calls"):
            require(exact(runtime[name], 0), "input-only runtime counters")
        require(runtime["projection_replayed"] is True and runtime["runtime_replayed"] is True,
                "runtime replay checks")
        require(len(runtime["traces"]) == len(fixture), "trace count")
        for index, (supplied, trace, row) in enumerate(zip(fixture, runtime["traces"], EXPECTED_ROWS)):
            require(exact(trace["case_index"], index) and len(trace["deliveries"]) == 1, "case order")
            observed = trace["deliveries"][0]
            require(observed["activity_id"] == ACTIVITY and "input" not in observed, "delivery identity")
            original = [supplied[ENTRY + ".input" + str(i)] for i in range(2)]
            require(exact(observed["inputs"], [
                {"port": "input" + str(i), "entity_id": RECTANGLE, "value": value,
                 "fields": fields(RECTANGLE, COORDINATES, value)}
                for i, value in enumerate(original)
            ]), "exact original input records")
            actual = observed["actual"]
            require(isinstance(actual, dict) and set(actual) == set(RESULT_NAMES), "output fields")
            require(all(type(actual[n]) is int and LO <= actual[n] <= HI for n in COORDINATES),
                    "exact int64 outputs")
            require(type(actual["has_pixels"]) is bool, "Boolean output")
            require(exact(actual, dict(zip(RESULT_NAMES, row))), "documented output")
            require(exact(observed["actual_fields"], fields(RESULT, RESULT_NAMES, actual)), "output field identities")
    require(exact(execution["result"]["runtime"]["traces"], replay["result"]["runtime"]["traces"]),
            "saved replay traces")
    require(execution["result"]["composition"]["source"] == replay["result"]["composition"]["source"],
            "saved replay generated source")


def main():
    if len(sys.argv) != 3:
        raise SystemExit("Usage: python3 tools/check-drag-crop-example.py EXECUTION REPLAY")
    check(Path.cwd(), Path(sys.argv[1]), Path(sys.argv[2]))
    print("PASS: 12 documented DragCropPlan inputs, exact int64 fields, four source bindings and saved replay")


if __name__ == "__main__":
    main()
