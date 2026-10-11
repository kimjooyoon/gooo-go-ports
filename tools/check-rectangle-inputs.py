#!/usr/bin/env python3
"""Preserve actual framework rejection evidence, separate from Go comparisons."""
import copy
import hashlib
import json
from pathlib import Path
import subprocess
import sys

from receipt_json import loads_receipt


def main():
    if len(sys.argv) != 2:
        raise SystemExit("Usage: python3 tools/check-rectangle-inputs.py NEW_OUTPUT_DIRECTORY")
    out = Path(sys.argv[1])
    out.mkdir(parents=True, exist_ok=False)
    source = Path("ports/image/geometry.gooo").read_bytes()
    (out / "geometry.gooo").write_bytes(source)
    manifest = {
        "schema": "gooo/package-workspace-manifest/v1",
        "entry": {"package_path": "go/image", "activity": "RectangleOverlaps"},
        "packages": [{"path": "go/image", "sources": ["geometry.gooo"]}],
    }
    manifest_path = out / "gooo.workspace.json"
    manifest_path.write_text(json.dumps(manifest, indent=2) + "\n")
    compiler = str(Path(".gooo").resolve())
    identity = subprocess.run([compiler, "version", "--build", "--json"],
                              check=True, capture_output=True).stdout
    (out / "compiler.json").write_bytes(identity)
    lock = json.loads(Path("upstream.json").read_bytes())
    build = json.loads(identity)
    assert build["compiler_source_sha"] == lock["compiler_revision"]
    assert build["go_version"] == lock["go_version"] and build["vcs_modified"] == "false"
    base = {"min_x": 1, "min_y": 2, "max_x": 5, "max_y": 6}
    bad_values = [
        ("missing", {k: v for k, v in base.items() if k != "max_y"}, 'requires field "max_y"'),
        ("text", dict(base, min_x="1"), "Integer requires an exact int64"),
        ("high", dict(base, min_x=2**63), "Integer requires an exact int64"),
        ("low", dict(base, min_x=-(2**63) - 1), "Integer requires an exact int64"),
        ("fraction", dict(base, min_x=1.5), "Integer requires an exact int64"),
        ("null-field", dict(base, min_x=None), "explicit Integer scalar required"),
        ("extra", dict(base, other=1), "contains an undeclared field"),
        ("null-record", None, "explicit JSON object required"),
        ("duplicate", base, "duplicate or invalid JSON key"),
    ]
    rows = []
    for name, value, diagnostic in bad_values:
        for position in range(2):
            values = [copy.deepcopy(base), copy.deepcopy(base)]
            values[position] = value
            # Assemble the duplicate-key fixture as text, since dictionaries cannot retain it.
            encoded = [json.dumps(v) for v in values]
            if name == "duplicate":
                encoded[position] = encoded[position].replace('"min_x": 1', '"min_x": 1, "min_x": 2', 1)
            raw = ('{"schema":"gooo/body-composition-inputs/v1","inputs":[{'
                   '"go/image:RectangleOverlaps.input0":' + encoded[0] + ','
                   '"go/image:RectangleOverlaps.input1":' + encoded[1] + '}]}\n').encode()
            prefix = name + "-" + str(position)
            fixture = out / (prefix + ".json")
            fixture.write_bytes(raw)
            command = [compiler, "package", "execute", "--json", "--inputs", str(fixture), str(manifest_path)]
            result = subprocess.run(command, capture_output=True, timeout=30)
            (out / (prefix + ".receipt.json")).write_bytes(result.stdout)
            (out / (prefix + ".stderr.txt")).write_bytes(result.stderr)
            receipt = loads_receipt(result.stdout)
            assert result.returncode == 1 and receipt["decision"] == "FAIL_CLOSED", prefix
            assert diagnostic in receipt["error"], (prefix, receipt["error"])
            if name != "duplicate":
                assert ".input" + str(position) in receipt["error"]
                assert receipt["inputs_digest"] == "sha256:" + hashlib.sha256(raw).hexdigest()
                assert receipt["manifest_digest"] == "sha256:" + hashlib.sha256(manifest_path.read_bytes()).hexdigest()
            rows.append({"case": prefix, "exit": result.returncode, "error": receipt["error"]})
    summary = {"status": "PASS", "framework_rejections": len(rows),
               "go_reference_cases": 0, "source_sha256": hashlib.sha256(source).hexdigest(),
               "compiler_revision": lock["compiler_revision"], "cases": rows}
    (out / "summary.json").write_text(json.dumps(summary, indent=2) + "\n")
    print("PASS: 18 malformed rectangle records rejected before native execution")


if __name__ == "__main__":
    main()
