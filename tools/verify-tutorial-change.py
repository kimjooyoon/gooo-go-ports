#!/usr/bin/env python3
"""Generate the tutorial edit/replay records, then check them with the Go reader.

Only the two copied tutorial sources are edited. Existing repository sources and
receipts remain intact. The resulting comparison is unsigned local evidence.
"""

import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import selectors
import shutil
import signal
import stat
import subprocess
import sys
import tempfile
import time


ROOT = Path(__file__).resolve().parent.parent
PIN = "83928fa4538245abe33b08549da47f8cbdc50ed2"
SOURCE_NAMES = ("examples/learn-gooo/limits.gooo", "examples/learn-gooo/main.gooo")
MAX_OUTPUT = 2 << 20
MAX_CACHE = 512 << 20


def require(value, message):
    if not value:
        raise ValueError(message)


def identity(path):
    path = Path(path)
    require(stat.S_ISREG(path.lstat().st_mode), "expected a regular file: " + str(path))
    raw = path.read_bytes()
    return {"bytes": len(raw), "sha256": "sha256:" + hashlib.sha256(raw).hexdigest()}


def write_json(path, value):
    path.write_text(json.dumps(value, indent=2, allow_nan=False) + "\n", encoding="utf-8")
    path.chmod(0o600)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", required=True, type=Path, help="new evidence directory; never overwritten")
    parser.add_argument("--compiler", type=Path, default=ROOT / ".gooo")
    parser.add_argument("--go", type=Path, help="installed Go1.27.2 executable; defaults to go on PATH")
    parser.add_argument("--before", type=Path, help="reuse an already produced original four-input record")
    args = parser.parse_args()
    compiler = args.compiler.absolute()
    require(compiler.is_file(), "build the pinned compiler with tools/bootstrap.sh first")
    selected_go = args.go or shutil.which("go")
    require(selected_go is not None, "select an installed Go1.27.2 executable")
    go = Path(selected_go).resolve()
    require(go.is_file(), "Go executable is missing")
    before_path = args.before.absolute() if args.before else None
    if before_path:
        require(before_path.is_file() and not before_path.is_symlink(), "original record must be a regular file")
        require(before_path.stat().st_size <= MAX_OUTPUT, "original record exceeds the bounded profile")
    out = args.out.absolute()
    require(not out.exists(), "output already exists; choose a new directory to preserve prior records")
    out.mkdir(mode=0o700, parents=True)
    work = Path(tempfile.mkdtemp(prefix="owned-work-", dir=out))
    stage = work / "workspace"
    cache = work / "go-cache"
    tmp = work / "tmp"
    module_cache = work / "go-mod-cache"
    go_path = work / "go-path"
    env = dict(os.environ, PATH=str(go.parent) + os.pathsep + os.environ.get("PATH", ""),
               GOTOOLCHAIN="local", GOENV="off", GOFLAGS="", GOVCS="*:off", GOWORK="off",
               GOCACHE=str(cache), GOMODCACHE=str(module_cache), GOPATH=str(go_path),
               GOTMPDIR=str(tmp), TMPDIR=str(tmp), GOPROXY="off", GOSUMDB="off")
    deadline = time.monotonic() + 480
    processes = []
    commands = []
    sources = {}
    report = {"schema": "gooo/tutorial-change-run/v1", "status": "IN_PROGRESS", "compiler_pin": PIN,
              "commands": commands, "model_calls": 0, "new_Gooo_commands": 0,
              "reused_before": None, "scope": "Synthetic tutorial execution and offline record consistency; no signatures or effect authorization."}
    received_signals = []

    def interrupted(signum, _frame):
        # Selectors may swallow InterruptedError as EINTR. Keep the handler
        # nonthrowing, including during finalization, and check in normal flow.
        if len(received_signals) < 16:
            received_signals.append(signum)

    handlers = {signum: signal.signal(signum, interrupted) for signum in (signal.SIGTERM, signal.SIGINT)}

    def cache_bytes():
        return sum(p.stat().st_size for p in cache.rglob("*") if p.is_file() and not p.is_symlink())

    def group_alive(pid):
        try:
            os.killpg(pid, 0)
        except ProcessLookupError:
            return False
        return True

    def close_group(proc):
        if group_alive(proc.pid):
            try:
                os.killpg(proc.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
        proc.wait(timeout=10)
        until = time.monotonic() + 5
        while group_alive(proc.pid) and time.monotonic() < until:
            time.sleep(0.02)
        return not group_alive(proc.pid)

    def run(label, argv, cwd):
        require(not received_signals, "received signal " + str(received_signals))
        require(time.monotonic() < deadline, "overall tutorial run deadline")
        record = {"label": label, "argv": [str(a) for a in argv], "started_at_utc": datetime.datetime.now(datetime.timezone.utc).isoformat()}
        commands.append(record)
        stdout = out / (label + ".stdout")
        stderr = out / (label + ".stderr")
        proc = subprocess.Popen(record["argv"], cwd=cwd, env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)
        processes.append(proc)
        record["pid"] = proc.pid
        write_json(out / "progress.json", report)
        selector = selectors.DefaultSelector()
        selector.register(proc.stdout, selectors.EVENT_READ, "stdout")
        selector.register(proc.stderr, selectors.EVENT_READ, "stderr")
        sizes = {"stdout": 0, "stderr": 0}
        started = time.monotonic()
        try:
            with stdout.open("xb") as so, stderr.open("xb") as se:
                streams = {"stdout": so, "stderr": se}
                while selector.get_map():
                    require(not received_signals, "received signal " + str(received_signals))
                    require(time.monotonic() < min(deadline, started + 180), "bounded command deadline: " + label)
                    for key, _ in selector.select(1):
                        data = os.read(key.fd, 65536)
                        if not data:
                            selector.unregister(key.fileobj)
                            continue
                        sizes[key.data] += len(data)
                        require(sizes[key.data] <= MAX_OUTPUT, "bounded command output: " + label)
                        streams[key.data].write(data)
                record["exit"] = proc.wait(timeout=max(1, min(deadline, started + 180) - time.monotonic()))
            require(not received_signals, "received signal " + str(received_signals))
            require(record["exit"] == 0, "command failed; inspect preserved stderr: " + label)
        finally:
            record["process_group_closed"] = close_group(proc)
            record["exit"] = proc.returncode
            selector.close()
            proc.stdout.close()
            proc.stderr.close()
            record.update(elapsed_seconds=time.monotonic() - started, stdout=identity(stdout), stderr=identity(stderr),
                          observed_stream_bytes=sizes, output_complete=all(size <= MAX_OUTPUT for size in sizes.values()))
            write_json(out / "progress.json", report)
            require(record["process_group_closed"], "owned process group not closed: " + label)
        require(cache_bytes() <= MAX_CACHE, "owned compiler cache exceeds512MiB")
        return stdout

    failure = None
    try:
        for directory in (stage, cache, tmp, module_cache, go_path):
            directory.mkdir()
        report["compiler_binary"] = identity(compiler)
        report["go_binary"] = identity(go)
        for name in (*SOURCE_NAMES, "learn-gooo.workspace.json", "examples/learn-gooo/inputs.json"):
            raw = (ROOT / name).read_bytes()
            target = stage / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(raw)
            sources[name] = identity(ROOT / name)
            saved = out / "bound-inputs" / name
            saved.parent.mkdir(parents=True, exist_ok=True)
            saved.write_bytes(raw)
        report["source_inputs"] = sources
        main_source = (stage / SOURCE_NAMES[1]).read_bytes()
        require(main_source.count(b"if score >= 60") == 1, "expected one original threshold condition")
        version = run("go-version", [go, "version"], ROOT).read_text().strip()
        require(version.startswith("go version go1.27.2 "), "select Go1.27.2 before running this tool")
        build = run("compiler-build", [compiler, "version", "--build", "--json"], ROOT)
        compiler_build = json.loads(build.read_bytes())
        require(compiler_build.get("compiler_source_sha") == PIN and compiler_build.get("vcs_modified") == "false", "expected the pinned clean compiler build")
        checker = work / "check-tutorial-change"
        run("build-checker", [go, "build", "-o", checker, "./cmd/check-tutorial-change"], ROOT)
        argv = [compiler, "package", "execute", "--json", "--inputs", "examples/learn-gooo/inputs.json", "learn-gooo.workspace.json"]
        if before_path:
            before = out / "before.stdout"
            before.write_bytes(before_path.read_bytes())
            report["reused_before"] = {"path": str(before_path), **identity(before)}
        else:
            before = run("before", argv, stage)
            report["new_Gooo_commands"] += 1
        for phase in ("before", "after"):
            for name in SOURCE_NAMES:
                target = out / "bound-source" / phase / name
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_bytes((stage / name).read_bytes())
            if phase == "before":
                (stage / SOURCE_NAMES[1]).write_bytes(main_source.replace(b"if score >= 60", b"if score >= 80"))
        after = run("after", argv, stage)
        report["new_Gooo_commands"] += 1
        replay = run("replay", [compiler, "package", "replay", "--json", "--receipt", after, "--inputs", "examples/learn-gooo/inputs.json", "learn-gooo.workspace.json"], stage)
        report["new_Gooo_commands"] += 1
        summary = run("check", [checker, "--before", before, "--after", after, "--replay", replay, "--compiler-build", build], ROOT)
        parsed = json.loads(summary.read_bytes())
        require(not received_signals, "received signal " + str(received_signals))
        (out / "summary.json").write_bytes(summary.read_bytes())
        report.update(status="TUTORIAL_CHANGE_AND_REPLAY_CHECKED", summary=identity(out / "summary.json"), comparison_schema=parsed.get("schema"))
    except Exception as error:
        failure = error
        report.update(status="FAILED", error=str(error))
    finally:
        finalization_errors = []
        groups_closed = []
        for proc in processes:
            try:
                groups_closed.append(close_group(proc))
            except Exception as error:
                groups_closed.append(False)
                finalization_errors.append("process group closure: " + str(error))
        report["all_owned_process_groups_closed"] = all(groups_closed)

        def unchanged(path, expected):
            try:
                return expected is not None and identity(path) == expected
            except Exception as error:
                finalization_errors.append("identity read: " + str(error))
                return False

        report["repository_sources_unchanged"] = all(unchanged(ROOT / name, expected) for name, expected in sources.items())
        report["compiler_binary_unchanged"] = unchanged(compiler, report.get("compiler_binary"))
        report["go_binary_unchanged"] = unchanged(go, report.get("go_binary"))
        report["all_owned_processes_waited"] = all(proc.poll() is not None for proc in processes)
        try:
            report["private_cache_bytes_before_cleanup"] = cache_bytes()
            require(not work.is_symlink() and work.resolve().parent == out.resolve(), "owned cleanup boundary")
            if report["all_owned_process_groups_closed"]:
                shutil.rmtree(work)
        except Exception as error:
            finalization_errors.append("owned cleanup: " + str(error))
        report["owned_work_removed"] = not work.exists()
        report["finalization_errors"] = finalization_errors
        report["received_signals"] = received_signals
        identities_unchanged = all(report[key] for key in
                                   ("repository_sources_unchanged", "compiler_binary_unchanged", "go_binary_unchanged"))
        if finalization_errors or not report["owned_work_removed"] or not identities_unchanged:
            report["status"] = "FAILED"
            failure = failure or ValueError("owned finalization incomplete; inspect run.json")
        if received_signals:
            report["status"] = "FAILED"
            report.setdefault("error", "received signal " + str(received_signals))
            failure = failure or ValueError(report["error"])
        report["completed_at_utc"] = datetime.datetime.now(datetime.timezone.utc).isoformat()
        write_json(out / "run.json", report)
        for signum, handler in handlers.items():
            signal.signal(signum, handler)
    require(report["repository_sources_unchanged"], "repository source changed during this observation")
    require(report["compiler_binary_unchanged"], "compiler binary changed during this observation")
    require(report["go_binary_unchanged"], "Go binary changed during this observation")
    require(report["all_owned_process_groups_closed"], "owned process group remained live; temporary work retained")
    require(not received_signals, "received signal " + str(received_signals))
    if failure:
        raise failure
    print(json.dumps({"status": report["status"], "summary": str(out / "summary.json"), "new_Gooo_commands": report["new_Gooo_commands"]}))


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print("tutorial change check failed: " + str(error), file=sys.stderr)
        raise SystemExit(1)
