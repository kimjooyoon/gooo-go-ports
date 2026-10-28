"""Prepare exact original Linux reference locations for the image reader.

Only Go module download commands run here. Saved comparators and MCP/Gooo cases
never run. Original argv/cwd records are checked, not rewritten. The caller
supplies its owned runner-temp root; no destination is followed from a report.
"""

import argparse
import importlib.util
import json
import os
from pathlib import Path
import signal
import stat
import subprocess
import tempfile

SPEC = importlib.util.spec_from_file_location(
    "reference_runtime_context", Path(__file__).with_name("service_grant_mcp_container.py"))
C = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(C)


def load(path):
    return C.strict_json(C.read_regular(Path(path), 1 << 20), "reference runtime metadata")


def destinations(root):
    root = Path(root)
    C.require(root.is_absolute() and stat.S_ISDIR(root.lstat().st_mode), "runner temp must be a regular absolute directory")
    return {"server": root / "gooo-service-grant-mcp", "alias": root / "alias-cli-bin/compare",
            "snapshot": root / "snapshot-cli-bin/compare"}


def download(go, dependency, output, workdir):
    argv = [str(go), "mod", "download", "-json", dependency["Path"] + "@" + dependency["Version"]]
    env = os.environ.copy()
    env.update(GOTOOLCHAIN="local", GOWORK="off", GOENV="off", GOTELEMETRY="off",
               GOPROXY="https://proxy.golang.org", GOSUMDB="sum.golang.org", GOMAXPROCS="2")
    stdout, stderr = output.with_suffix(".stdout"), output.with_suffix(".stderr")
    with stdout.open("xb") as out, stderr.open("xb") as err:
        process = subprocess.Popen(argv, stdin=subprocess.DEVNULL, stdout=out, stderr=err,
                                   env=env, cwd=workdir, start_new_session=True)
        try:
            code = process.wait(timeout=45)
        except BaseException as original:
            try: os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError: pass
            except BaseException as cleanup_error: original.add_note("kill: " + repr(cleanup_error))
            try: process.wait(timeout=10)
            except BaseException as cleanup_error: original.add_note("wait: " + repr(cleanup_error))
            raise
        try:
            os.killpg(process.pid, 0)
        except ProcessLookupError:
            pass
        else:
            os.killpg(process.pid, signal.SIGKILL)
            raise C.Refusal("owned Go download process group remains")
    C.require(code == 0, "fixed public module download failed; inspect saved stderr")
    info = load(stdout)
    C.require({key: info.get(key) for key in ("Path", "Version", "Sum")} == dependency and
              "Error" not in info, "downloaded module identity differs")
    directory = Path(info["Dir"])
    C.require(directory.is_absolute() and stat.S_ISDIR(directory.lstat().st_mode),
              "downloaded module directory is not regular/absolute")
    output.write_text(json.dumps(info, sort_keys=True, indent=2) + "\n")
    return directory


def check_locations(references, go, targets, original_alias, original_snapshot):
    """Check planned caller locations against original metadata, without I/O."""
    C.require(original_alias["binary"]["path"] == str(targets["server"]) and
              original_alias["alias_cli"]["path"] == str(targets["alias"]) and
              original_alias["go_identity_reader"]["path"] == str(go),
              "caller locations differ from original recorded alias argv")
    records = {r["label"]: r["record"] for r in original_snapshot["reference_processes"]}
    for label, target in [("server-identity", targets["server"]), ("snapshot-cli-identity", targets["snapshot"])]:
        C.require(records[label]["argv"] == [str(go), "version", "-m", "-json", str(target)],
                  "caller locations differ from original snapshot identity argv")
    for label, record in records.items():
        C.require(record["cwd"] == str(references / "snapshot-wire" / label),
                  "caller reference root differs from original recorded cwd")
        if label not in ("server-identity", "snapshot-cli-identity"):
            C.require(record["argv"] == [str(targets["snapshot"]), "--definition-snapshot"],
                      "caller reference CLI differs from original recorded argv")


def prepare(references, context, go, runner_temp):
    references, context, go = map(Path, (references, context, go))
    C.require(all(p.is_absolute() for p in (references, context, go)), "absolute reference/context/Go paths required")
    prepared = load(references / "prepared-references.json")
    C.require(prepared["schema"] == "gooo/prepared-saved-native-reference-packet/v1" and
              prepared["status"] == "PINNED_REFERENCE_BYTES_MATERIALIZED_NO_EXECUTION",
              "reference preparation identity differs")
    go_raw = C.read_regular(go, 32 << 20)
    C.require(prepared["original_observation_go_tool"] == {"bytes": len(go_raw), "sha256": C.digest(go_raw)},
              "Go identity reader bytes differ from original observation")
    native = load(context / "context.json")
    C.require(native["archive_sha256"] == prepared["native_archive_sha256"] and
              native["source_commit"] == prepared["native_source"], "native context differs from prepared references")
    source_server = context / "runtime/gooo-service-grant-mcp"
    server_raw = C.read_regular(source_server, 32 << 20)
    C.require(C.digest(server_raw) == native["binary_sha256"], "context executable byte identity differs")
    targets = destinations(runner_temp)
    original_alias = load(references / "alias-wire/acceptance.json")
    original_snapshot = load(references / "snapshot-wire/acceptance.json")
    check_locations(references, go, targets, original_alias, original_snapshot)
    for target in targets.values():
        C.require(not os.path.lexists(target), "refuse to overwrite an existing reference location")
    for key in ("alias", "snapshot"):
        C.require(not os.path.lexists(targets[key].parent), "reference CLI directory must be fresh")
        meta = prepared["references"][key]
        dependency = C.ALIAS_DEPENDENCY if key == "alias" else C.SNAPSHOT.DEPENDENCY
        C.require(meta["module"] == dependency, "unreviewed reference module selection")
        raw = C.read_regular(references / meta["file"], 16 << 20)
        C.require(len(raw) == meta["bytes"] and C.digest(raw) == meta["sha256"], "reference CLI bytes differ")
    state_path = references / "runtime-state.json"
    C.require(not state_path.exists(), "reference runtime state must be fresh")
    state = {"schema": "gooo/materialized-reference-runtime/v1", "status": "PREPARING",
             "runner_temp": str(Path(runner_temp)), "references": str(references), "go": str(go),
             "created": [], "arguments": [], "reference_cases_or_MCP_executed": False}
    def save(): state_path.write_text(json.dumps(state, sort_keys=True, indent=2) + "\n")
    save()
    try:
        modules = {}
        # Run outside the checkout so explicit module acquisition cannot edit
        # its go.mod/go.sum. No reference case is executed in this directory.
        with tempfile.TemporaryDirectory(prefix="gooo-reference-download-", dir=runner_temp) as workdir:
            for key, dependency in [("alias", C.ALIAS_DEPENDENCY), ("snapshot", C.SNAPSHOT.DEPENDENCY)]:
                modules[key] = download(go, dependency, references / (key + "-module.json"), workdir)
        for key in ("server", "alias", "snapshot"):
            target = targets[key]
            if key != "server": target.parent.mkdir(mode=0o700)
            raw = server_raw if key == "server" else C.read_regular(references / (key + "-compare"), 16 << 20)
            C.write_new(target, raw, 0o755)
            state["created"].append({"role": key, "bytes": len(raw), "sha256": C.digest(raw)})
            save()
        state["arguments"] = ["--alias-wire-report", str(references / "alias-wire/acceptance.json"),
            "--alias-module", str(modules["alias"]), "--alias-cli", str(targets["alias"]), "--go", str(go),
            "--snapshot-wire-report", str(references / "snapshot-wire/acceptance.json"),
            "--snapshot-module", str(modules["snapshot"]), "--snapshot-cli", str(targets["snapshot"]),
            "--snapshot-source-binary", str(targets["server"])]
        state["status"] = "ORIGINAL_LOCATIONS_AND_MODULES_READY_NO_REFERENCE_EXECUTION"
        save()
        return state
    except BaseException as error:
        state["status"] = "FIRST_FAILURE_NO_AUTOMATIC_RETRY"
        state["first_failure"] = {"type": type(error).__name__, "message": str(error)[:2000]}
        try: save()
        except BaseException as save_error: error.add_note("failure record: " + repr(save_error))
        raise


def cleanup(references, runner_temp):
    references = Path(references)
    state_path = references / "runtime-state.json"
    if not state_path.exists(): return {"status": "NO_CREATED_REFERENCE_STATE"}
    state = load(state_path)
    C.require(state["schema"] == "gooo/materialized-reference-runtime/v1" and
              state["references"] == str(references) and state["runner_temp"] == str(Path(runner_temp)),
              "reference cleanup ownership root differs")
    targets = destinations(runner_temp)
    for item in state["created"]:
        C.require(item["role"] in targets, "unknown reference cleanup role")
        path = targets[item["role"]]
        if not os.path.lexists(path): continue
        raw = C.read_regular(path, 32 << 20)
        C.require(len(raw) == item["bytes"] and C.digest(raw) == item["sha256"],
                  "owned reference bytes changed; refuse deletion")
        path.unlink()
    for key in ("alias", "snapshot"):
        directory = targets[key].parent
        if directory.exists(): directory.rmdir()
    state["cleanup"] = "OWNED_REFERENCE_FILES_REMOVED"
    state_path.write_text(json.dumps(state, sort_keys=True, indent=2) + "\n")
    return state


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    prepare_cmd = sub.add_parser("prepare")
    prepare_cmd.add_argument("--references", type=Path, required=True)
    prepare_cmd.add_argument("--context", type=Path, required=True)
    prepare_cmd.add_argument("--go", type=Path, required=True)
    prepare_cmd.add_argument("--runner-temp", type=Path, required=True)
    clean = sub.add_parser("cleanup")
    clean.add_argument("--references", type=Path, required=True)
    clean.add_argument("--runner-temp", type=Path, required=True)
    arguments = sub.add_parser("arguments")
    arguments.add_argument("--references", type=Path, required=True)
    args = parser.parse_args(argv)
    if args.command == "prepare": result = prepare(args.references, args.context, args.go, args.runner_temp)
    elif args.command == "cleanup": result = cleanup(args.references, args.runner_temp)
    else:
        state = load(args.references / "runtime-state.json")
        C.require(state["status"] == "ORIGINAL_LOCATIONS_AND_MODULES_READY_NO_REFERENCE_EXECUTION" and
                  len(state["arguments"]) == 16, "reference arguments are not ready")
        import sys
        sys.stdout.buffer.write(b"\0".join(x.encode() for x in state["arguments"]) + b"\0")
        return
    print(json.dumps(result))


if __name__ == "__main__": main()
