#!/usr/bin/env python3
# Copyright 2026 Gooo Go Ports contributors. BSD-style terms: ../LICENSE.
"""Copy seven current repository files and six templates into a new project.

No compiler, Go command, dependency installation, network access, or deletion.
The resulting project has its own Go module and separately runnable check/CI;
export success is not a native verification result.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import stat
import sys


SOURCE_FILES = (
    ("ports/net/http/etag.gooo", "ports/net/http/etag.gooo"),
    ("ports/net/httpguts/headers.gooo", "ports/net/httpguts/headers.gooo"),
    ("examples/strong-validator-facts/main.gooo", "examples/strong-validator-facts/main.gooo"),
    ("strong-validator-facts.workspace.json", "strong-validator-facts.workspace.json"),
    ("examples/strong-validator-facts/corpus.json", "corpus.json"),
    ("LICENSE", "LICENSE"),
    ("third_party/GO-LICENSE", "third_party/GO-LICENSE"),
)
TEMPLATE_FILES = (
    "check.go", "go.mod", "go.sum", "README.md", "NOTICE", ".github/workflows/ci.yml",
)
MAX_FILE_BYTES = 10 * 1024 * 1024


def need(condition, message):
    if not condition:
        raise ValueError(message)


def sha256(raw):
    return hashlib.sha256(raw).hexdigest()


def real_path(path, directory=False):
    need(path.is_absolute() and Path(os.path.abspath(path)) == path,
         f"absolute clean path required: {path}")
    for part in (*reversed(path.parents), path):
        info = part.lstat()
        need(not stat.S_ISLNK(info.st_mode), f"symlinks are unsupported: {part}")
        if part != path or directory:
            need(stat.S_ISDIR(info.st_mode), f"existing directory required: {part}")
    return path


def identity(info):
    return (info.st_dev, info.st_ino, info.st_mode, info.st_size,
            info.st_mtime_ns, info.st_ctime_ns)


def snapshot(path):
    real_path(path)
    before = path.lstat()
    need(stat.S_ISREG(before.st_mode) and before.st_size <= MAX_FILE_BYTES,
         f"bounded regular file required: {path}")
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(fd, "rb") as source:
        opened = os.fstat(source.fileno())
        need(identity(opened) == identity(before), f"file changed while opening: {path}")
        raw = source.read(MAX_FILE_BYTES + 1)
        after = os.fstat(source.fileno())
    need(len(raw) <= MAX_FILE_BYTES and identity(after) == identity(before)
         and identity(path.lstat()) == identity(before), f"file changed while reading: {path}")
    return raw, identity(before)


def export(args):
    script = real_path(Path(os.path.abspath(__file__)))
    template_root = real_path(script.parent.parent, directory=True)
    source_root = real_path(args.source_root or template_root, directory=True)
    out = args.out
    need(out.is_absolute() and Path(os.path.abspath(out)) == out,
         "--out must be an absolute clean new directory")
    real_path(out.parent, directory=True)
    need(not os.path.lexists(out), f"output already exists; preserved: {out}")
    need(out != source_root and out not in source_root.parents
         and out != template_root and out not in template_root.parents,
         "output cannot contain the input repository/templates")

    members = [(source_root / src, dest) for src, dest in SOURCE_FILES]
    members += [(template_root / "templates/etag-project" / name, name)
                for name in TEMPLATE_FILES]
    need(len(members) == 13 and len({dest for _, dest in members}) == 13,
         "exact thirteen output files required")
    # Freeze every original input and the exporter itself before making output.
    frozen = {path: snapshot(path) for path, _ in members}
    frozen[script] = snapshot(script)
    need(sum(len(frozen[source_root / name][0]) for name, _ in SOURCE_FILES[:3])
         <= MAX_FILE_BYTES, "selected Gooo sources exceed10MiB")

    os.mkdir(out, 0o700)  # Exclusive creation; never reuse or remove an existing path.
    outputs = []
    for source, destination in members:
        target = out / destination
        target.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
        raw = frozen[source][0]
        fd = os.open(target, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o644)
        with os.fdopen(fd, "wb") as handle:
            need(handle.write(raw) == len(raw), f"incomplete output write: {target}")
        copied, _ = snapshot(target)
        need(copied == raw, f"output bytes differ: {target}")
        outputs.append({"source": str(source), "path": destination,
                        "bytes": len(raw), "sha256": sha256(raw)})

    roster = sorted(path.relative_to(out).as_posix()
                    for path in out.rglob("*") if path.is_file())
    need(roster == sorted(destination for _, destination in members),
         "unexpected output file roster")
    for path, before in frozen.items():
        need(snapshot(path) == before, f"input changed during export: {path}")
    # stdout is one export manifest only. No extra metadata file changes the13-file kit.
    return {
        "schema": "gooo/etag-project-export/v1",
        "decision": "EXPORTED",
        "out": str(out),
        "source_root": str(source_root),
        "template_root": str(template_root),
        "exporter_sha256": sha256(frozen[script][0]),
        "files": outputs,
        "file_count": 13,
        "input_files_unchanged": True,
        "compiler_executed": False,
        "scope": "source copy only; run the exported checker separately for fresh native results",
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", required=True, type=Path,
                        help="new absolute directory, with an existing real parent")
    parser.add_argument("--source-root", type=Path,
                        help="optional existing real absolute repository root; only seven fixed files are read")
    args = parser.parse_args()
    try:
        manifest = export(args)
    except (OSError, ValueError) as error:
        print(f"export-etag-project: {error}; no paths were removed", file=sys.stderr)
        return 1
    json.dump(manifest, sys.stdout, indent=2)
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
