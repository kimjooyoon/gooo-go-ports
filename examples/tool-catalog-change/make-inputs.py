#!/usr/bin/env python3
# Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
"""Serialize canonical cases into detached Gooo/Go inputs with PROV-O lineage."""
import argparse
import datetime
import hashlib
import json
from pathlib import Path
import sys
import uuid

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
FAMILY = "gooo/tool-catalog-input-projection/v1"


def need(condition, message):
    if not condition:
        raise ValueError(message)


def load(raw):
    def unique(rows):
        result = {}
        for key, value in rows:
            need(key not in result, "duplicate JSON key")
            result[key] = value
        return result
    return json.loads(raw, object_pairs_hook=unique,
                      parse_constant=lambda _: need(False, "nonfinite JSON"))


def encode(value):
    return (json.dumps(value, ensure_ascii=False, indent=2) + "\n").encode()


def generate(out):
    started = datetime.datetime.now(datetime.timezone.utc).isoformat()
    names = ["examples/tool-catalog-change/case-facts.json",
             "tool-catalog-change.workspace.json",
             "examples/tool-catalog-change/make-inputs.py"]
    inputs = {name: (ROOT / name).read_bytes() for name in names}
    data = load(inputs[names[0]])
    need(type(data) is dict and set(data) == {"schema", "catalog", "controls", "control_labels"}
         and data["schema"] == "gooo/tool-catalog-case-facts/v1", "unsupported canonical facts")
    manifest = load(inputs[names[1]])
    need(manifest.get("schema") == "gooo/package-workspace-manifest/v1", "unsupported workspace")
    entry = manifest["entry"]["package_path"] + ":" + manifest["entry"]["activity"]
    fields = {"name": str}
    for side in ("before", "after"):
        fields.update({side + "_" + key: typ for key, typ in {
            "present": bool, "service": str, "risk": str, "capability": str,
            "allow_write": bool, "allow_gmail_send": bool, "allow_gmail_delete": bool,
            "readonly": bool, "policy_selectors": list, "runtime_selectors": list}.items()})
    outputs = {}
    for profile, count in [("catalog", 27), ("controls", 20)]:
        rows = data[profile]
        need(type(rows) is list and len(rows) == count, "reviewed profile count differs")
        for row in rows:
            need(type(row) is dict and set(row) == set(fields), "case fields differ")
            for key, typ in fields.items():
                value = row[key]
                need(type(value) is typ, "wrong fact type: " + key)
                texts = value if typ is list else ([value] if typ is str else [])
                if typ is list:
                    need(len(value) <= 256, "selector list exceeds represented domain")
                for text in texts:
                    need(type(text) is str and len(text.encode("utf-8")) <= 1024,
                         "fact text exceeds represented domain")
        outputs[profile + "-inputs.json"] = encode({
            "schema": "gooo/body-composition-inputs/v1", "inputs": [{entry: row} for row in rows]})
        outputs[profile + "-oracle-inputs.json"] = encode(rows)
    need(type(data["control_labels"]) is list and len(data["control_labels"]) == 20
         and all(type(label) is str for label in data["control_labels"]), "control labels differ")
    need(out.is_absolute() and not out.exists() and not out.is_symlink()
         and not out.resolve().is_relative_to(ROOT.resolve()),
         "output must be fresh, absolute and outside the checkout")
    out.mkdir(mode=0o700)
    activity = "urn:uuid:" + str(uuid.uuid4())
    graph = []
    used = []
    for index, (name, raw) in enumerate(inputs.items()):
        identifier = activity + ":input:" + str(index)
        graph.append({"@id": identifier, "@type": "prov:Entity", "path": name,
                      "bytes": len(raw), "sha256": hashlib.sha256(raw).hexdigest()})
        used.append({"@id": identifier})
    for name, raw in outputs.items():
        (out / name).write_bytes(raw)
        graph.append({"@id": activity + ":output:" + name, "@type": "prov:Entity",
                      "path": name, "bytes": len(raw), "sha256": hashlib.sha256(raw).hexdigest(),
                      "prov:wasGeneratedBy": {"@id": activity}, "prov:wasDerivedFrom": used})
    graph.append({"@id": activity, "@type": "prov:Activity", "prov:used": used,
                  "rule": FAMILY,
                  "prov:startedAtTime": {"@value": started, "@type": "xsd:dateTime"},
                  "prov:endedAtTime": {"@value": datetime.datetime.now(datetime.timezone.utc).isoformat(),
                                       "@type": "xsd:dateTime"}})
    proof = {"@context": {"prov": "http://www.w3.org/ns/prov#",
                          "xsd": "http://www.w3.org/2001/XMLSchema#",
                          "@vocab": "urn:gooo:tool-catalog-inputs:"},
             "@graph": graph, "family": FAMILY, "outputs": 4,
             "scope": "Canonical-case serialization only. Source-catalog checking, native execution, verification and application are separate activities.",
             "source_modified_by_producer": False}
    (out / "generation.provenance.jsonld").write_bytes(encode(proof))
    return {"status": "DETACHED_INPUTS_GENERATED", "outputs": 4, "cases": 47}


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args()
    try:
        print(json.dumps(generate(args.out)))
    except (OSError, ValueError, TypeError, KeyError) as error:
        sys.exit("Refused: " + str(error))
