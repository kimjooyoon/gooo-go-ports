#!/usr/bin/env python3
"""Check the 24 documented alpha-matte inputs and their saved replay."""
import hashlib
import json
from pathlib import Path
import sys

from receipt_json import loads_receipt


ENTRY = "app/alpha-matte:MattePixel"
ACTIVITY = "gooo://go-ports/examples/alpha-matte/matte-pixel"
NRGBA = "gooo://go/image/color/nrgba"
RGB = "gooo://go/image/color/rgb"
RESULT = "gooo://go-ports/examples/alpha-matte/result"
CHANNELS = ["red", "green", "blue"]
INPUT_NAMES = [CHANNELS + ["alpha"], CHANNELS]
INPUT_ENTITIES = [NRGBA, RGB]
RESULT_NAMES = ["in_byte_domain", "premultiplied_red", "premultiplied_green",
                "premultiplied_blue", "premultiplied_alpha", "red", "green", "blue"]
LO, HI = -(2**63), 2**63 - 1
COMPILER = "90ff2bb5aeffc2ac553496b646d14db1e8d33fdb"
SOURCES = {
    "go/image/color": ["ports/image/color.gooo", "ports/image/nrgba.gooo"],
    "app/alpha-matte": ["examples/alpha-matte/main.gooo"],
}
# Fixed original seven-channel tuples: foreground RGBA, then opaque background RGB.
# Pinning inputs as well as answers rejects a coordinated rewrite of the fixture
# and receipt, including a narrowed int64 rejection value.
INPUT_ROWS = [
    [255, 0, 0, 0, 10, 20, 30], [12, 34, 56, 0, 255, 255, 255],
    [13, 77, 201, 255, 201, 77, 13], [255, 0, 0, 128, 0, 0, 255],
    [255, 0, 0, 127, 0, 0, 255], [255, 255, 255, 1, 0, 0, 0],
    [255, 255, 255, 254, 0, 0, 0], [1, 0, 0, 128, 0, 0, 0],
    [1, 1, 1, 1, 255, 255, 255], [255, 1, 128, 1, 13, 77, 201],
    [0, 254, 127, 254, 13, 77, 201], [0, 0, 0, 128, 255, 255, 255],
    [255, 255, 255, 128, 0, 0, 0], [13, 77, 201, 128, 13, 77, 201],
    [127, 127, 127, 127, 128, 128, 128], [128, 128, 128, 128, 127, 127, 127],
    [0, 0, 0, 255, 255, 255, 255], [255, 255, 255, 255, 0, 0, 0],
    [-1, 128, 255, 128, 13, 77, 201], [255, 128, 1, 256, 13, 77, 201],
    [255, 128, 1, 128, 13, 77, 256], [255, 128, 1, 128, 13, -1, 201],
    [HI, 128, 1, 128, 13, 77, 201], [255, 128, 1, 128, 13, 77, LO],
]
# Literal outputs from pinned Go color.NRGBA.RGBA and draw.Draw(..., draw.Over).
# No production arithmetic is used as an oracle here. Invalid rows are the
# explicitly separate Gooo all-minus-one extension, not Go uint8 conversions.
EXPECTED_ROWS = [
    [True, 0, 0, 0, 0, 10, 20, 30],
    [True, 0, 0, 0, 0, 255, 255, 255],
    [True, 3341, 19789, 51657, 65535, 13, 77, 201],
    [True, 32896, 0, 0, 32896, 128, 0, 127],
    [True, 32639, 0, 0, 32639, 127, 0, 128],
    [True, 257, 257, 257, 257, 1, 1, 1],
    [True, 65278, 65278, 65278, 65278, 254, 254, 254],
    [True, 129, 0, 0, 32896, 0, 0, 0],
    [True, 1, 1, 1, 257, 254, 254, 254],
    [True, 257, 1, 129, 257, 14, 77, 201],
    [True, 0, 65022, 32511, 65278, 0, 254, 127],
    [True, 0, 0, 0, 32896, 127, 127, 127],
    [True, 32896, 32896, 32896, 32896, 128, 128, 128],
    [True, 1677, 9933, 25929, 32896, 13, 77, 201],
    [True, 16255, 16255, 16255, 32639, 127, 127, 127],
    [True, 16512, 16512, 16512, 32896, 127, 127, 127],
    [True, 0, 0, 0, 65535, 0, 0, 0],
    [True, 65535, 65535, 65535, 65535, 255, 255, 255],
    [False, -1, -1, -1, -1, -1, -1, -1],
    [False, -1, -1, -1, -1, -1, -1, -1],
    [False, -1, -1, -1, -1, -1, -1, -1],
    [False, -1, -1, -1, -1, -1, -1, -1],
    [False, -1, -1, -1, -1, -1, -1, -1],
    [False, -1, -1, -1, -1, -1, -1, -1],
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
    fixture_raw = (root / "examples/alpha-matte/inputs.json").read_bytes()
    fixture_document = loads_receipt(fixture_raw)
    require(fixture_document["schema"] == "gooo/body-composition-inputs/v1", "inputs schema")
    fixture = fixture_document["inputs"]
    manifest_raw = (root / "alpha-matte.workspace.json").read_bytes()
    manifest = loads_receipt(manifest_raw)
    require(manifest["schema"] == "gooo/package-workspace-manifest/v1", "manifest schema")
    require(exact(manifest["entry"], {"package_path": "app/alpha-matte",
                                      "activity": "MattePixel"}), "manifest entry")
    require(len(manifest["packages"]) == len(SOURCES), "manifest package count")
    require({p["path"]: p["sources"] for p in manifest["packages"]} == SOURCES,
            "manifest source roster")
    lock = loads_receipt((root / "upstream.json").read_bytes())
    require(lock["compiler_revision"] == COMPILER and lock["compiler_version"] == "0.6.29-dev"
            and lock["go_version"] == "go1.27.2"
            and lock["go_revision"] == "022c8636110ebac86a9b88724cda168ed713c21f", "pinned toolchain lock")
    execution_raw = execution_path.read_bytes()
    execution = loads_receipt(execution_raw)
    replay = loads_receipt(replay_path.read_bytes())
    require(replay["replayed_from_sha256"] == digest(execution_raw), "saved replay binding")
    require(exact(replay["result"]["replay"]["model_calls"], 0), "replay model calls")
    require("replayed_from_sha256" not in execution and execution["result"].get("replay") is None,
            "unexpected replay metadata on original execution")
    require(len(fixture) == len(INPUT_ROWS) == len(EXPECTED_ROWS) == 24, "documented input count")
    hashes = {name: source_digest(root / name) for names in SOURCES.values() for name in names}
    for supplied, row in zip(fixture, INPUT_ROWS):
        require(set(supplied) == {ENTRY + ".input0", ENTRY + ".input1"}, "input ports")
        for i in range(2):
            value = supplied[ENTRY + ".input" + str(i)]
            require(isinstance(value, dict) and set(value) == set(INPUT_NAMES[i]), "pixel input fields")
            require(all(type(v) is int and LO <= v <= HI for v in value.values()), "exact int64 inputs")
            expected = dict(zip(INPUT_NAMES[i], row[:4] if i == 0 else row[4:]))
            require(exact(value, expected), "documented original input")
    for receipt in (execution, replay):
        require(receipt["schema"] == "gooo/workspace-body-execution-receipt/v1", "receipt schema")
        require(receipt["decision"] == "OBSERVED" and not receipt.get("error"), "observed execution")
        require(receipt["manifest_digest"] == digest(manifest_raw), "manifest digest")
        require(receipt["inputs_digest"] == digest(fixture_raw) and "cases_digest" not in receipt, "input-only digest")
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
            if package["path"] == "app/alpha-matte":
                exported = [e for e in package["exports"]
                            if e.get("name") == "MattePixel" or e.get("id") == ACTIVITY]
                require(len(exported) == 1 and exported[0].get("name") == "MattePixel"
                        and exported[0].get("id") == ACTIVITY and exported[0].get("kind") == "activity",
                        "export activity identity")
                require(exported[0]["input_types"] == INPUT_ENTITIES, "export input identities")
                require(exported[0]["output_type"] == RESULT, "export output identity")
        runtime, composition = result["runtime"], result["composition"]
        require(runtime["schema"] == "gooo/body-composition-runtime/v1", "runtime schema")
        require(runtime["stage"] == composition["stage"] == "COMPLETE", "complete execution")
        require(runtime.get("failure") is None, "runtime failure")
        require(runtime["producer_source_sha"] == lock["compiler_revision"], "compiler identity")
        version = runtime["go_version"].split()
        require(len(version) == 4 and version[:3] == ["go", "version", lock["go_version"]]
                and version[3].split("/")[-1] in ("amd64", "arm64"), "64-bit Go toolchain identity")
        require(isinstance(composition["source"], str) and bool(composition["source"]), "nonempty generated source")
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
            require(observed["activity_id"] == ACTIVITY and "input" not in observed
                    and observed.get("input_fields") in (None, []), "delivery identity")
            original = [supplied[ENTRY + ".input" + str(i)] for i in range(2)]
            require(exact(observed["inputs"], [
                {"port": "input" + str(i), "entity_id": INPUT_ENTITIES[i], "value": value,
                 "fields": fields(INPUT_ENTITIES[i], INPUT_NAMES[i], value)}
                for i, value in enumerate(original)
            ]), "exact original input records")
            actual = observed["actual"]
            require(isinstance(actual, dict) and set(actual) == set(RESULT_NAMES), "output fields")
            require(all(type(actual[n]) is int and LO <= actual[n] <= HI for n in RESULT_NAMES[1:]),
                    "exact int64 outputs")
            require(type(actual["in_byte_domain"]) is bool, "Boolean output")
            valid = all(0 <= value <= 255 for record in original for value in record.values())
            require(actual["in_byte_domain"] is valid, "byte-domain result")
            if not valid:
                require(all(actual[name] == -1 for name in RESULT_NAMES[1:]), "domain rejection sentinel")
            require(exact(actual, dict(zip(RESULT_NAMES, row))), "documented output")
            require(exact(observed["actual_fields"], fields(RESULT, RESULT_NAMES, actual)), "output field identities")
    require(exact(execution["result"]["runtime"]["traces"], replay["result"]["runtime"]["traces"]),
            "saved replay traces")
    require(execution["result"]["composition"]["source"] == replay["result"]["composition"]["source"],
            "saved replay generated source")


def main():
    if len(sys.argv) != 3:
        raise SystemExit("Usage: python3 tools/check-alpha-matte.py EXECUTION REPLAY")
    check(Path.cwd(), Path(sys.argv[1]), Path(sys.argv[2]))
    print("PASS: 24 documented MattePixel inputs (18 Go references, 6 domain rejections), exact record fields, three source bindings and saved replay")


if __name__ == "__main__":
    main()
