#!/usr/bin/env python3
"""Check the documented input-only YCbCr example and its saved replay."""
import hashlib
import json
from pathlib import Path
import sys

from receipt_json import loads_receipt


def digest(raw):
    return "sha256:" + hashlib.sha256(raw).hexdigest()


def exact(left, right):
    return json.dumps(left, sort_keys=True) == json.dumps(right, sort_keys=True)


def main():
    if len(sys.argv) != 3:
        raise SystemExit("Usage: python3 tools/check-ycbcr-example.py EXECUTION REPLAY")
    fixture_raw = Path("examples/ycbcr-inspector/inputs.json").read_bytes()
    fixture = json.loads(fixture_raw)["inputs"]
    manifest_raw = Path("ycbcr-inspector.workspace.json").read_bytes()
    manifest = json.loads(manifest_raw)
    lock = json.loads(Path("upstream.json").read_bytes())
    execution_raw = Path(sys.argv[1]).read_bytes()
    execution = loads_receipt(execution_raw)
    replay = loads_receipt(Path(sys.argv[2]).read_bytes())
    assert replay["replayed_from_sha256"] == digest(execution_raw)
    assert replay["result"]["replay"]["model_calls"] == 0
    entity = "gooo://go-ports/examples/ycbcr-inspector/inspection"
    activity = "gooo://go-ports/examples/ycbcr-inspector/inspect-rgb"
    entry = "app/ycbcr-inspector:InspectRGB"
    names = ["in_byte_domain", "y", "cb", "cr", "red", "green", "blue",
             "red_delta", "green_delta", "blue_delta", "exact"]
    # Literal Go-reference results for the README's named inputs; no formula.
    expected_rows = [
        [True, 0, 128, 128, 0, 0, 0, 0, 0, 0, True],
        [True, 255, 128, 128, 255, 255, 255, 0, 0, 0, True],
        [True, 76, 85, 255, 254, 0, 0, -1, 0, 0, False],
        [True, 150, 44, 21, 0, 255, 1, 0, 0, 1, False],
        [True, 29, 255, 107, 0, 0, 254, 0, 0, -1, False],
        [True, 128, 128, 128, 128, 128, 128, 0, 0, 0, True],
        [True, 79, 101, 163, 128, 63, 31, 0, -1, -1, False],
        [True, 114, 178, 148, 142, 82, 203, 0, -1, 1, False],
        [True, 153, 88, 163, 202, 142, 82, 0, 0, -1, False],
        *[[False, -1, -1, -1, -1, -1, -1, -1, -1, -1, False] for _ in range(3)],
    ]
    assert len(fixture) == len(expected_rows) == 12
    sources = {p["path"]: p["sources"] for p in manifest["packages"]}
    for receipt in (execution, replay):
        assert receipt["schema"] == "gooo/workspace-body-execution-receipt/v1"
        assert receipt["decision"] == "OBSERVED" and not receipt.get("error")
        assert receipt["manifest_digest"] == digest(manifest_raw)
        assert receipt["inputs_digest"] == digest(fixture_raw)
        result = receipt["result"]
        for key, value in manifest["entry"].items():
            assert result["program"]["entry"][key] == value
        packages = result["program"]["workspace_image"]["packages"]
        assert len(packages) == len(sources) == 2
        assert {p["path"] for p in packages} == set(sources)
        for package in packages:
            expected_sources = sources[package["path"]]
            assert len(package["sources"]) == len(expected_sources)
            assert {s["filename"] for s in package["sources"]} == set(expected_sources)
            for source in package["sources"]:
                text = Path(source["filename"]).read_text()
                encoded = (json.dumps(text, ensure_ascii=False).replace("&", "\\u0026")
                           .replace("<", "\\u003c").replace(">", "\\u003e")
                           .replace("\u2028", "\\u2028").replace("\u2029", "\\u2029").encode())
                assert source["source_digest"] == digest(encoded)
            if package["path"] == "app/ycbcr-inspector":
                exported = [e for e in package["exports"] if e["name"] == "InspectRGB"]
                assert len(exported) == 1 and exported[0]["id"] == activity
                assert exported[0]["input_types"] == ["gooo://std/integer"] * 3
                assert exported[0]["output_type"] == entity
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
        assert report["activity_id"] == activity
        assert report["compiler_source_sha"] == lock["compiler_revision"]
        assert runtime["finite_total"] == runtime["finite_passed"] == runtime["model_calls"] == 0
        assert runtime["projection_replayed"] is True and runtime["runtime_replayed"] is True
        assert len(runtime["traces"]) == len(fixture)
        for index, (supplied, trace, row) in enumerate(zip(fixture, runtime["traces"], expected_rows)):
            assert trace["case_index"] == index and len(trace["deliveries"]) == 1
            observed = trace["deliveries"][0]
            assert observed["activity_id"] == activity and "input" not in observed
            original = [supplied[entry + ".input" + str(i)] for i in range(3)]
            assert all(type(v) is int and -(2**63) <= v < 2**63 for v in original)
            assert exact(observed["inputs"], [
                {"port": "input" + str(i), "entity_id": "gooo://std/integer", "value": v}
                for i, v in enumerate(original)
            ])
            actual = observed["actual"]
            assert exact(actual, dict(zip(names, row)))
            assert exact(observed["actual_fields"], [
                {"id": entity + "/" + n.replace("_", "-"), "name": n, "value": actual[n]}
                for n in names
            ])
    assert exact(execution["result"]["runtime"]["traces"], replay["result"]["runtime"]["traces"])
    assert execution["result"]["composition"]["source"] == replay["result"]["composition"]["source"]
    print("PASS: 12 documented YCbCr inputs, exact records, source bindings and saved replay")


if __name__ == "__main__":
    main()
