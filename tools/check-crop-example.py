#!/usr/bin/env python3
"""Check input-only CropPlan receipts; differential Go cases live in cmd/verify."""
import hashlib
import json
from pathlib import Path
import sys

from receipt_json import loads_receipt


def digest(raw):
    return "sha256:" + hashlib.sha256(raw).hexdigest()


def fields(entity, names, value):
    return [{"id": entity + "/" + name.replace("_", "-"), "name": name,
             "value": value[name]} for name in names]


def exact(left, right):
    # Python equality alone equates True with 1 and 1.0 with 1.
    return json.dumps(left, sort_keys=True) == json.dumps(right, sort_keys=True)


def main():
    if len(sys.argv) != 3:
        raise SystemExit("Usage: python3 tools/check-crop-example.py EXECUTION REPLAY")
    fixture_raw = Path("examples/crop-planner/inputs.json").read_bytes()
    fixture = json.loads(fixture_raw)["inputs"]
    manifest_raw = Path("crop-planner.workspace.json").read_bytes()
    manifest = json.loads(manifest_raw)
    lock = json.loads(Path("upstream.json").read_bytes())
    execution_raw = Path(sys.argv[1]).read_bytes()
    execution = loads_receipt(execution_raw)
    replay = loads_receipt(Path(sys.argv[2]).read_bytes())
    assert replay["replayed_from_sha256"] == digest(execution_raw)
    assert replay["result"]["replay"]["model_calls"] == 0
    coordinate_names = ["min_x", "min_y", "max_x", "max_y"]
    result_names = coordinate_names + ["has_pixels"]
    rectangle = "gooo://go/image/rectangle"
    result_entity = "gooo://example/crop-planner/result"
    entry = "example/crop-planner:CropPlan"
    activity_id = "gooo://example/crop-planner/crop-plan"
    # Literal values from the example's documented table, not a geometry implementation.
    lo, hi = -(2**63), 2**63 - 1
    expected_rows = [
        [0, 40, 200, 480, True], [0, 0, 640, 480, True],
        [100, 100, 200, 200, True],
        *[[0, 0, 0, 0, False] for _ in range(5)],
        [-8, -30, 5, 4, True], [hi - 1, hi - 1, hi, hi, True],
        [lo, lo, lo + 1, lo + 1, True],
        [9007199254740993, -9007199254740992,
         9007199254740994, -9007199254740991, True],
    ]
    assert len(fixture) == len(expected_rows) == 12
    source_names = {p["path"]: p["sources"][0] for p in manifest["packages"]}
    for receipt in (execution, replay):
        assert receipt["schema"] == "gooo/workspace-body-execution-receipt/v1"
        assert receipt["decision"] == "OBSERVED" and not receipt.get("error")
        assert receipt["manifest_digest"] == digest(manifest_raw)
        assert receipt["inputs_digest"] == digest(fixture_raw)
        result = receipt["result"]
        for key, value in manifest["entry"].items():
            assert result["program"]["entry"][key] == value
        packages = result["program"]["workspace_image"]["packages"]
        assert len(packages) == 2 and {p["path"] for p in packages} == set(source_names)
        for package in packages:
            filename = source_names[package["path"]]
            text = Path(filename).read_text()
            encoded = (json.dumps(text, ensure_ascii=False).replace("&", "\\u0026")
                       .replace("<", "\\u003c").replace(">", "\\u003e")
                       .replace("\u2028", "\\u2028").replace("\u2029", "\\u2029").encode())
            assert len(package["sources"]) == 1
            source = package["sources"][0]
            assert source["filename"] == filename and source["source_digest"] == digest(encoded)
            if package["path"] == "example/crop-planner":
                exported = [e for e in package["exports"] if e["name"] == "CropPlan"]
                assert len(exported) == 1 and exported[0]["id"] == activity_id
                assert exported[0]["input_types"] == [rectangle, rectangle]
                assert exported[0]["output_type"] == result_entity
        runtime, composition = result["runtime"], result["composition"]
        assert runtime["schema"] == "gooo/body-composition-runtime/v1"
        assert runtime["stage"] == composition["stage"] == "COMPLETE"
        assert runtime.get("failure") is None
        assert runtime["producer_source_sha"] == lock["compiler_revision"]
        version = runtime["go_version"].split()
        assert len(version) == 4 and version[:3] == ["go", "version", lock["go_version"]]
        assert runtime["generated_sha256"] == composition["generated_sha256"] == digest(composition["source"].encode())
        assert len(composition["steps"]) == 1
        report = composition["steps"][0]["generation"]["report"]
        assert report["activity_id"] == activity_id and report["compiler_source_sha"] == lock["compiler_revision"]
        assert runtime["finite_total"] == runtime["finite_passed"] == runtime["model_calls"] == 0
        assert runtime["projection_replayed"] is True and runtime["runtime_replayed"] is True
        assert len(runtime["traces"]) == len(fixture)
        for index, (supplied, trace, expected_row) in enumerate(zip(fixture, runtime["traces"], expected_rows)):
            assert trace["case_index"] == index and len(trace["deliveries"]) == 1
            observed = trace["deliveries"][0]
            assert observed["activity_id"] == activity_id and "input" not in observed
            original = [supplied[entry + ".input0"], supplied[entry + ".input1"]]
            for value in original:
                assert set(value) == set(coordinate_names)
                assert all(type(v) is int and lo <= v <= hi for v in value.values())
            assert exact(observed["inputs"], [
                {"port": "input" + str(i), "entity_id": rectangle, "value": value,
                 "fields": fields(rectangle, coordinate_names, value)}
                for i, value in enumerate(original)
            ])
            actual = observed["actual"]
            assert set(actual) == set(result_names)
            assert all(type(actual[n]) is int for n in coordinate_names)
            assert type(actual["has_pixels"]) is bool
            assert actual == dict(zip(result_names, expected_row))
            assert exact(observed["actual_fields"], fields(result_entity, result_names, actual))
    assert execution["result"]["runtime"]["traces"] == replay["result"]["runtime"]["traces"]
    assert execution["result"]["composition"]["source"] == replay["result"]["composition"]["source"]
    print("PASS: 12 documented CropPlan inputs, exact int64 fields, source bindings and saved replay")


if __name__ == "__main__":
    main()
