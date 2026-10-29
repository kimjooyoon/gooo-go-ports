#!/usr/bin/env python3
# Copyright 2026 Gooo Go Ports contributors. BSD-style terms: ../LICENSE.
"""Check or export a pinned instruction-only plugin; never install or run it."""
import argparse
import copy
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys


ROOT = Path(__file__).resolve().parents[1]
PACKAGE = ROOT / "plugins/gooo-workflows"
SKILLS = ("gooo-first-program", "gooo-compare-tool-snapshots")
TAILS = ("SKILL.md", "LICENSE", "agents/openai.yaml")
MAPPING = tuple((f".agents/skills/{name}/{tail}", f"skills/{name}/{tail}")
                for name in SKILLS for tail in TAILS)
FILES = {target for _, target in MAPPING} | {
    "plugin.json", "SOURCE.json", "LICENSE", "assets/gooo.svg"}


def need(condition, message):
    if not condition:
        raise ValueError(message)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def encoded(value):
    return (json.dumps(value, indent=2, ensure_ascii=False) + "\n").encode()


def read(path):
    need(not path.is_symlink() and path.is_file(), f"regular file required: {path}")
    need(path.stat().st_size <= 1024 * 1024, f"file too large: {path}")
    return path.read_bytes()


def git(*args):
    return subprocess.check_output(["git", "-C", str(ROOT), *args], stderr=subprocess.PIPE)


def commit(ref):
    need(re.fullmatch(r"[0-9a-f]{40}", ref) is not None, "full immutable commit SHA required")
    need(git("rev-parse", "--verify", ref + "^{commit}").decode().strip() == ref,
         "source commit must exist in this local repository")
    return ref


def source_records(ref):
    payload, records = {}, []
    for source, target in MAPPING:
        raw = git("show", f"{ref}:{source}")
        payload[target] = raw
        records.append({"source_path": source, "package_path": target,
                        "source_git_blob": git("rev-parse", f"{ref}:{source}").decode().strip(),
                        "sha256": sha(raw), "bytes": len(raw)})
    payload["LICENSE"] = git("show", ref + ":LICENSE")
    return payload, records


def checked_package():
    paths = list(PACKAGE.rglob("*"))
    need(not any(path.is_symlink() for path in paths), "package symlinks are unsupported")
    found = {path.relative_to(PACKAGE).as_posix() for path in paths if path.is_file()}
    need(found == FILES, "exact ten-file package roster required")
    payload = {path: read(PACKAGE / path) for path in FILES}
    provenance = json.loads(payload["SOURCE.json"])
    manifest = json.loads(payload["plugin.json"])
    ref = commit(provenance["source_commit"])
    expected, records = source_records(ref)
    need(provenance["skills_copied_without_content_changes"] == records,
         "source mapping or recorded Git/hash identity differs")
    for path, raw in expected.items():
        need(payload[path] == raw, f"source bytes differ: {path}")
    payload.update(expected)
    need(provenance["root_license"] == {"source_path": "LICENSE", "sha256": sha(expected["LICENSE"])},
         "root license identity differs")
    need(provenance["asset"]["package_path"] == "assets/gooo.svg"
         and provenance["asset"]["sha256"] == sha(payload["assets/gooo.svg"]),
         "frozen asset identity differs")
    need(manifest["name"] == "gooo-workflows", "unexpected plugin name")
    need(manifest["extensions"]["com.openai"]["requires_local_executor"] is True,
         "local executor declaration required")
    return payload, provenance, manifest


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true", help="check the frozen checked-in snapshot")
    parser.add_argument("--output", type=Path, help="export to a new absolute directory; never overwrite")
    parser.add_argument("--source-ref", help="full commit SHA for an explicitly reviewed new candidate")
    parser.add_argument("--version", help="explicit x.y.z version for a new candidate")
    args = parser.parse_args()
    need(not (args.check and args.output), "choose check or export")
    need(args.output is not None or not (args.source_ref or args.version),
         "new source/version requires a separate output directory")
    payload, provenance, manifest = checked_package()
    ref = commit(args.source_ref or provenance["source_commit"])
    version = args.version or manifest["version"]
    need(re.fullmatch(r"(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)", version) is not None,
         "x.y.z version without leading zeros required")
    if ref != provenance["source_commit"]:
        need(args.version is not None and version != manifest["version"],
             "new source requires an explicit different version")
    if ref != provenance["source_commit"] or version != manifest["version"]:
        provenance, manifest = copy.deepcopy(provenance), copy.deepcopy(manifest)
        new_payload, records = source_records(ref)
        payload.update(new_payload)
        provenance["source_commit"] = ref
        provenance["skills_copied_without_content_changes"] = records
        provenance["root_license"] = {"source_path": "LICENSE", "sha256": sha(payload["LICENSE"])}
        manifest["version"] = version
        manifest["homepage"] = provenance["repository"] + "/blob/" + ref + "/docs/ai-agents.md"
        payload["SOURCE.json"], payload["plugin.json"] = encoded(provenance), encoded(manifest)
    if args.output is not None:
        output = args.output
        need(output.is_absolute() and not output.exists() and not output.is_symlink(),
             "output must be a new absolute directory")
        need(output.parent.is_dir(), "output parent must exist")
        need(not any(parent.samefile(PACKAGE) for parent in output.resolve().parents),
             "output must be outside the frozen source package")
        output.mkdir()
        for path, raw in sorted(payload.items()):
            target = output / path
            target.parent.mkdir(parents=True, exist_ok=True)
            with target.open("xb") as stream:
                stream.write(raw)
            target.chmod(0o644)
    print(json.dumps({"source_commit": ref, "version": version, "files": len(payload),
                      "output": str(args.output) if args.output else None,
                      "sha256": {path: sha(raw) for path, raw in sorted(payload.items())},
                      "scope": "Source snapshot only; no plugin installation, activation or workflow execution."},
                     indent=2))


if __name__ == "__main__":
    try:
        main()
    except (ValueError, KeyError, OSError, subprocess.CalledProcessError) as error:
        print(json.dumps({"error": str(error)}), file=sys.stderr)
        sys.exit(1)
