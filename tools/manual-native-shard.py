#!/usr/bin/env python3
"""Opt-in hosted full82 transport. Official metadata reconciliation is not attestation."""

import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import signal
import stat
import subprocess
import time
import urllib.error
import urllib.parse
import urllib.request
import zipfile
import zlib


REPOSITORY = "kimjooyoon/gooo-go-ports"
WORKFLOW = ".github/workflows/manual-native-shards.yml"
COMPILER = "83928fa4538245abe33b08549da47f8cbdc50ed2"
GO_VERSION = "go1.27.2"
PLAN_JOB = "Plan full native shards"
JOIN_JOB = "Join full native shards"
MAX_ZIP = 512 << 20
MAX_FILE = 64 << 20
MAX_EXTRACTED = 2 << 30
MAX_FILES = 100000
EXHAUSTIVE = {(35, "simple-fold"), (37, "ycbcr/RGBToYCbCr"), (38, "ycbcr/YCbCrToRGB"),
              (45, "case-mapping/To"), (46, "case-mapping/ToUpper"),
              (47, "case-mapping/ToLower"), (48, "case-mapping/ToTitle")}
INTEGRATION_SOURCES = (WORKFLOW, "tools/manual-native-shard.py",
                       "tools/manual-native-shard_test.py")
DEADLINE = None


def available(seconds):
    if DEADLINE is None:
        return seconds
    remaining = DEADLINE - time.monotonic()
    need(remaining > 0, "whole phase deadline reached; preservation reserve remains outside this budget")
    return min(seconds, remaining)


def interrupted(number, frame):
    raise InterruptedError("workflow signal " + str(number))


class Refused(ValueError):
    pass


def need(ok, message):
    if not ok:
        raise Refused(message)


def integer(value, lower=1):
    need(type(value) is int and value >= lower, "exact integer required")
    return value


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def file_ref(path):
    info = path.lstat()
    need(stat.S_ISREG(info.st_mode) and not path.is_symlink(), "regular file required")
    h = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1 << 20), b""):
            available(1)
            h.update(block)
    after = path.lstat()
    stable = ("st_dev", "st_ino", "st_mode", "st_size", "st_mtime_ns", "st_ctime_ns")
    need(all(getattr(after, key) == getattr(info, key) for key in stable),
         "file changed during hashing")
    return {"bytes": info.st_size, "sha256": h.hexdigest()}


def pairs(values):
    result = {}
    for key, value in values:
        need(key not in result, "duplicate JSON key")
        result[key] = value
    return result


def decode(raw):
    return json.loads(raw, object_pairs_hook=pairs,
                      parse_constant=lambda value: (_ for _ in ()).throw(Refused(value)))


def exact(left, right):
    if type(left) is not type(right):
        return False
    if type(left) is dict:
        return left.keys() == right.keys() and all(exact(left[k], right[k]) for k in left)
    if type(left) is list:
        return len(left) == len(right) and all(exact(a, b) for a, b in zip(left, right))
    return left == right


def read_json(path, cap=16 << 20):
    need(path.is_file() and not path.is_symlink() and path.stat().st_size <= cap,
         "bounded regular JSON file required")
    return decode(path.read_bytes())


def write_json(path, value):
    with path.open("x", encoding="utf-8") as output:
        json.dump(value, output, indent=2, allow_nan=False)
        output.write("\n")


def instant(value):
    need(type(value) is str, "official timestamp required")
    return datetime.datetime.fromisoformat(value.replace("Z", "+00:00"))


def validate_run(run, context):
    need(integer(run["id"]) == int(context["run_id"]) and
         integer(run["run_attempt"]) == context["attempt"] and
         run["head_sha"] == context["tested_commit"] and
         run["event"] == "workflow_dispatch" and run["path"] == WORKFLOW,
         "official run/attempt/event/workflow/source differs")
    need(run["repository"]["full_name"] == run["head_repository"]["full_name"] == REPOSITORY
         and integer(run["repository"]["id"]) == integer(run["head_repository"]["id"]),
         "foreign official repository")
    # A current run can still be queued while the official own-job record is running.
    # API.run_jobs separately requires that exact current job to be in progress.
    need("conclusion" in run and (run["status"], run["conclusion"]) in
         (("queued", None), ("in_progress", None), ("completed", "success")),
         "failed, cancelled or unknown run state")


def validate_jobs(document, context, names, completed):
    jobs = document["jobs"]
    need(type(jobs) is list and integer(document["total_count"]) == len(jobs) <= 100,
         "complete bounded official jobs page required")
    need(len({integer(j["id"]) for j in jobs}) == len(jobs) and
         len({j["name"] for j in jobs}) == len(jobs), "duplicate official job")
    known_names = {PLAN_JOB, JOIN_JOB, *("native-shard-" + str(i) for i in range(3))}
    for job in jobs:
        need(job["name"] in known_names and integer(job["run_id"]) == int(context["run_id"]) and
             integer(job["run_attempt"]) == context["attempt"] and
             job["head_sha"] == context["tested_commit"], "foreign official job")
    result = {}
    for name in names:
        found = [job for job in jobs if job["name"] == name]
        need(len(found) == 1, "missing or duplicate expected official job: " + name)
        job = found[0]
        if name in completed:
            need(job["status"] == "completed" and job["conclusion"] == "success",
                 "worker/planner failed, cancelled or unfinished: " + name)
        else:
            need(job["status"] == "in_progress" and job.get("conclusion") is None,
                 "current job is not running")
        result[name] = job
    return result


def validate_artifacts(document, context, expected, run, complete=True):
    artifacts = document["artifacts"]
    need(type(artifacts) is list and type(document["total_count"]) is int and
         document["total_count"] == len(artifacts) <= 100,
         "complete bounded official artifact page required")
    prefix = artifact_prefix(context)
    current = [a for a in artifacts if a["name"].startswith(prefix)]
    known = {prefix + "control", *(prefix + "shard-" + str(i) for i in range(3))}
    need(all(a["name"] in known for a in current) and
         len({a["name"] for a in current}) == len(current), "duplicate or foreign current-attempt artifact")
    selected = current if complete else [a for a in current if a["name"] in expected]
    need(len(selected) == len(expected) and {a["name"] for a in selected} == set(expected),
         "missing, duplicate or extra current-attempt artifact")
    need(len({integer(a["id"]) for a in selected}) == len(selected), "duplicate artifact ID")
    result = {}
    for artifact in selected:
        owner = artifact["workflow_run"]
        need(artifact["expired"] is False and owner["id"] == int(context["run_id"]) and
             owner["head_sha"] == context["tested_commit"] and
             owner["repository_id"] == owner["head_repository_id"] == run["repository"]["id"],
             "foreign artifact run/repository/source")
        need(type(artifact["size_in_bytes"]) is int and 0 < artifact["size_in_bytes"] <= MAX_ZIP
             and type(artifact["digest"]) is str and
             re.fullmatch(r"sha256:[0-9a-f]{64}", artifact["digest"]),
             "bounded artifact with official SHA256 required")
        job = expected[artifact["name"]]
        need(instant(job["started_at"]) <= instant(artifact["created_at"]) <=
             instant(job["completed_at"]) + datetime.timedelta(seconds=1),
             "artifact creation outside expected producer job")
        result[artifact["name"]] = artifact
    return result


def validate_plan(plan, context):
    need(plan["schema"] == "gooo/native-ci-shard-plan/v1" and exact(plan["run"], context),
         "foreign plan/context")
    need(type(plan["cases"]) is int and plan["cases"] == 175649 and
         type(plan["batches"]) is int and plan["batches"] == 2305 and
         len(plan["suites"]) == 82 and len(plan["workspaces"]) == 82,
         "full existing82-suite population required")
    need(type(plan["plan_sha256"]) is str and
         re.fullmatch(r"sha256:[0-9a-f]{64}", plan["plan_sha256"]), "plan digest required")
    shards = plan["shards"]
    need(len(shards) == 3 and all(type(s["index"]) is int for s in shards) and
         [s["index"] for s in shards] == [0, 1, 2], "exact three shards")
    indices = [i for shard in shards for i in shard["original_indices"]]
    need(all(type(i) is int for i in indices) and sorted(indices) == list(range(82)),
         "missing or duplicate original suite")
    need(all(type(s["index"]) is int for s in plan["suites"]) and
         [s["index"] for s in plan["suites"]] == list(range(82)), "original suite order")


def artifact_prefix(context):
    return "manual-native-" + context["run_id"] + "-a" + str(context["attempt"]) + "-"


def validate_reference(reference, plan):
    need(reference["schema"] == "gooo/native-shard-reference-index/v1" and
         reference["state"] == "RAW_REFERENCES_CHECKED_PRODUCER_UNAUTHENTICATED" and
         reference["raw_receipts_inspected"] is True and reference["producer_authenticated"] is False and
         reference["native_acceptance"] is False and exact(reference["plan"], plan),
         "current raw reference and scope required")
    suites = reference["suites"]
    need(len(suites) == 82 and sorted(s["original_index"] for s in suites) == list(range(82)) and
         len(reference["shards"]) == 3, "complete original reference population")
    reports = [(s["original_index"], e["kind"]) for s in suites for e in s["exhaustive"]]
    need(len(reports) == 7 and set(reports) == EXHAUSTIVE,
         "all seven original exhaustive reports must be re-inspected and exported")


def archive_member(name):
    path = PurePosixPath(name)
    need(type(name) is str and name and "\\" not in name and "\0" not in name and
         not path.is_absolute() and all(p not in ("", ".", "..") for p in name.rstrip("/").split("/")),
         "unsafe archive member")
    return path


def extract(archive, target):
    target.mkdir(mode=0o700)
    total = 0
    with zipfile.ZipFile(archive) as source:
        members = source.infolist()
        need(0 < len(members) <= MAX_FILES and len({m.filename for m in members}) == len(members),
             "bounded unique archive members required")
        for member in members:
            relative = archive_member(member.filename)
            mode = member.external_attr >> 16
            need(not member.flag_bits & 1 and member.compress_type in (0, 8) and
                 (not stat.S_IFMT(mode) or stat.S_ISREG(mode) or stat.S_ISDIR(mode)),
                 "encrypted, symlink or special archive member refused")
            total += member.file_size
            need(0 <= member.file_size <= MAX_FILE and total <= MAX_EXTRACTED,
                 "artifact uncompressed size cap")
            path = target / relative
            if member.is_dir():
                path.mkdir(parents=True, exist_ok=True, mode=0o700)
                continue
            path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
            try:
                with source.open(member) as incoming, path.open("xb") as output:
                    for block in iter(lambda: incoming.read(1 << 20), b""):
                        available(1)
                        output.write(block)
            except (zipfile.BadZipFile, zlib.error) as error:
                raise Refused("artifact member decompression/CRC failed") from error
            need(path.stat().st_size == member.file_size, "extracted size differs")
    return total


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args):
        return None


class API:
    def __init__(self, context, out):
        self.context, self.out = context, out
        self.token = os.environ.get("GH_TOKEN", "")
        need(self.token, "read-only GitHub Actions token required")
        self.calls = []
        self.opener = urllib.request.build_opener(NoRedirect)

    def request(self, path):
        need(len(self.calls) < 20 and not path.startswith("/"), "bounded API path/call count")
        return urllib.request.Request("https://api.github.com/repos/" + REPOSITORY + "/" + path,
                                      headers={"Authorization": "Bearer " + self.token,
                                               "Accept": "application/vnd.github+json",
                                               "Accept-Encoding": "identity",
                                               "X-GitHub-Api-Version": "2022-11-28"})

    def get(self, path, name):
        with self.opener.open(self.request(path), timeout=available(30)) as response:
            need(response.status == 200, "official API status")
            raw = response.read((2 << 20) + 1)
        need(len(raw) <= 2 << 20, "API metadata cap")
        self.calls.append({"path": path, "bytes": len(raw), "sha256": digest(raw)})
        need(sum(c["bytes"] for c in self.calls if "path" in c) <= 8 << 20,
             "aggregate API metadata cap")
        (self.out / (str(len(self.calls)) + "-" + name)).write_bytes(raw)
        return decode(raw)

    def run_jobs(self, names, completed):
        route = "actions/runs/" + self.context["run_id"] + "/attempts/" + str(self.context["attempt"])
        run = self.get(route, "official-run.json")
        validate_run(run, self.context)
        document = self.get(route + "/jobs?per_page=100", "official-jobs.json")
        return run, validate_jobs(document, self.context, names, completed)

    def artifacts(self, expected, run, complete=True):
        document = self.get("actions/runs/" + self.context["run_id"] +
                            "/artifacts?per_page=100", "official-artifacts.json")
        return validate_artifacts(document, self.context, expected, run, complete)

    def download(self, artifact, target):
        started = time.monotonic_ns()
        try:
            response = self.opener.open(self.request("actions/artifacts/" + str(artifact["id"]) + "/zip"), timeout=available(30))
        except urllib.error.HTTPError as error:
            need(error.code == 302, "official artifact redirect required")
            url = error.headers["Location"]
            error.close()
        else:
            response.close()
            raise Refused("official artifact redirect required")
        parsed = urllib.parse.urlsplit(url)
        host = parsed.hostname or ""
        need(parsed.scheme == "https" and parsed.username is None and parsed.password is None and
             parsed.port in (None, 443) and
             (re.fullmatch(r"productionresultssa[0-9]+\.blob\.core\.windows\.net", host) or
              host.endswith(".actions.githubusercontent.com") or host.endswith(".githubusercontent.com")),
             "unexpected official artifact redirect host")
        # No GitHub token is sent to storage. The signed URL stays in memory.
        amount = 0
        h = hashlib.sha256()
        with self.opener.open(urllib.request.Request(url, headers={"Accept-Encoding": "identity"}), timeout=available(30)) as response:
            need(response.status == 200 and not response.headers.get("Content-Encoding") and
                 int(response.headers["Content-Length"]) == artifact["size_in_bytes"], "artifact response size")
            with target.open("xb") as output:
                while True:
                    available(1)
                    block = response.read(min(1 << 20, artifact["size_in_bytes"] + 1 - amount))
                    if not block:
                        break
                    amount += len(block)
                    need(amount <= artifact["size_in_bytes"], "artifact response overflow")
                    output.write(block)
                    h.update(block)
        need(amount == artifact["size_in_bytes"] and "sha256:" + h.hexdigest() == artifact["digest"],
             "official artifact digest/length differs")
        self.calls.append({"artifact_id": artifact["id"], "bytes": amount,
                           "sha256": h.hexdigest(), "download_elapsed_ns": time.monotonic_ns() - started})


def go_installation_root(go):
    need(go.is_absolute() and go.resolve() == go and go.is_file() and
         not go.is_symlink() and go.name == "go" and go.parent.name == "bin",
         "canonical selected Go installation executable required")
    root = go.parent.parent
    need((root / "bin/go").samefile(go) and (root / "src").is_dir() and
         (root / "pkg/tool").is_dir(), "selected Go installation layout required")
    for relative in ("src/math/bits/make_tables.go", "src/image/draw/draw.go"):
        source = root / relative
        need(source.is_file() and not source.is_symlink(), "selected Go standard library source required")
    return root


def child_environment(go):
    # Never inherit an unrelated GOROOT or pass workflow API/runtime credentials.
    root = go_installation_root(go)
    allowed = ("PATH", "HOME", "GOCACHE", "GOMODCACHE", "TMPDIR")
    env = {k: os.environ[k] for k in allowed if k in os.environ}
    env.update(GOROOT=str(root), GOTOOLCHAIN="local", GOWORK="off", GOENV="off", CGO_ENABLED="0",
               GOMAXPROCS="2", GOFLAGS="-mod=readonly -p=1")
    env["PATH"] = str(go.parent) + (os.pathsep + env["PATH"] if env.get("PATH") else "")
    return env


def owned_size(root):
    size = 0
    count = 0
    for directory, directories, files in os.walk(root):
        for name in directories + files:
            try:
                info = (Path(directory) / name).lstat()
            except FileNotFoundError:
                continue
            need(not stat.S_ISLNK(info.st_mode), "symlink inside owned work")
            if stat.S_ISREG(info.st_mode):
                size += info.st_size
                count += 1
                need(count <= MAX_FILES, "owned file count cap")
    return size


class Commands:
    def __init__(self, root, out, go, owned_limit=3 << 30):
        self.root, self.out, self.records = root, out, []
        self.go, self.go_root = go, go_installation_root(go)
        self.owned_limit = owned_limit

    def run(self, label, args, seconds):
        started = time.monotonic_ns()
        record = {"label": label, "argv": [str(a) for a in args], "state": "STARTED",
                  "goroot": str(self.go_root)}
        self.records.append(record)
        with (self.out / (label + ".stdout")).open("xb") as stdout, (self.out / (label + ".stderr")).open("xb") as stderr:
            process = subprocess.Popen(args, cwd=self.root, env=child_environment(self.go),
                                       stdout=stdout, stderr=stderr, start_new_session=True)
            try:
                stop = time.monotonic() + available(seconds)
                sampled = 0
                while True:
                    available(1)
                    need(time.monotonic() < stop, "command deadline: " + label)
                    need(os.fstat(stdout.fileno()).st_size <= 8 << 20 and
                         os.fstat(stderr.fileno()).st_size <= 8 << 20, "live stdout/stderr cap: " + label)
                    if time.monotonic() >= sampled:
                        need(owned_size(self.out) <= self.owned_limit, "sampled owned byte cap")
                        sampled = time.monotonic() + 2
                    try:
                        result = process.wait(timeout=min(0.25, max(0.001, stop - time.monotonic())))
                        break
                    except subprocess.TimeoutExpired:
                        pass
            except BaseException:
                try:
                    os.killpg(process.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
                process.wait()
                record.update(state="INTERRUPTED_OR_BOUNDED_FAILURE", exit=process.returncode,
                              elapsed_ns=time.monotonic_ns() - started, direct_child_waited=True)
                raise
        record.update({"state": "RETURNED", "exit": result,
                             "elapsed_ns": time.monotonic_ns() - started,
                             "direct_child_waited": True})
        need((self.out / (label + ".stdout")).stat().st_size <= 8 << 20 and
             (self.out / (label + ".stderr")).stat().st_size <= 8 << 20,
             "command stdout exceeds bounded in-memory read; original file retained")
        record.update(stdout=file_ref(self.out / (label + ".stdout")),
                      stderr=file_ref(self.out / (label + ".stderr")))
        need(result == 0, "command failed: " + label)
        return (self.out / (label + ".stdout")).read_bytes()


def checkout(root, expected):
    def git(*args):
        return subprocess.check_output(["git", "-C", str(root), *args],
                                       env=dict(os.environ, GIT_NO_LAZY_FETCH="1"), timeout=30).decode().strip()
    need(git("rev-parse", "HEAD") == expected and
         not git("status", "--porcelain=v1", "--untracked-files=all"), "exact clean checkout required")
    return git("rev-parse", "HEAD^{tree}")


def current_context(root):
    source = os.environ.get("EXPECTED_SOURCE_SHA", "")
    need(re.fullmatch(r"[0-9a-f]{40}", source) and source == os.environ.get("GITHUB_SHA") ==
         os.environ.get("SOURCE_WORKFLOW_SHA"), "explicit exact workflow/source commit required")
    need(os.environ.get("GITHUB_REPOSITORY") == REPOSITORY and
         os.environ.get("GITHUB_EVENT_NAME") == "workflow_dispatch", "manual repository event required")
    return {"repository": REPOSITORY, "run_id": str(integer(int(os.environ["GITHUB_RUN_ID"]))),
            "attempt": integer(int(os.environ["GITHUB_RUN_ATTEMPT"])),
            "tested_commit": source, "tested_tree": checkout(root, source),
            "compiler_revision": COMPILER, "go_version": GO_VERSION, "workers": 3}


def shared_tools(root, control, commands, go):
    metadata = read_json(control / "control.json")
    need(metadata["schema"] == "gooo/manual-native-shard-control/v1" and
         exact(metadata["context"], current_context(root)) and metadata["integration_sources"] ==
         {p: file_ref(root / p) for p in INTEGRATION_SOURCES}, "shared source context differs")
    need(exact(read_json(control / "context.json"), metadata["context"]), "shared context file differs")
    for name in ("context.json", "plan.json", "compiler", "verify", "compiler-build.json"):
        need(file_ref(control / name) == metadata["files"][name], "shared tool/plan bytes differ")
    need(file_ref(go) == metadata["go_binary"], "actual Go binary differs from planner")
    need(commands.run("go-version", [str(go), "version"], 15).decode().strip() == "go version go1.27.2 linux/amd64", "pinned Linux Go required")
    validate_plan(read_json(control / "plan.json"), metadata["context"])
    compiler = root / ".gooo"
    need(not compiler.exists() and not compiler.is_symlink(), "fresh compiler destination required")
    shutil.copyfile(control / "compiler", compiler)
    compiler.chmod(0o700)
    (control / "verify").chmod(0o700)
    identity = decode(commands.run("compiler-identity", [str(compiler), "version", "--build", "--json"], 20))
    need(identity == read_json(control / "compiler-build.json"), "shared compiler build identity differs")
    return metadata


def fetch_control(api, context, jobs, run, work):
    name = artifact_prefix(context) + "control"
    artifacts = api.artifacts({name: jobs[PLAN_JOB]}, run, complete=False)
    archive = work / "control.zip"
    api.download(artifacts[name], archive)
    extract(archive, work / "control")
    control = read_json(work / "control/control.json")
    need(control["planner_job_id"] == jobs[PLAN_JOB]["id"], "foreign planner claim")
    return artifacts[name]


def handoff_matches(document, context, plan, control, index, job, artifacts):
    need(document["schema"] == "gooo/manual-native-shard-handoff/v1" and
         exact(document["context"], context) and type(document["index"]) is int and
         document["index"] == index and integer(document["job_id"]) == job["id"] and
         document["job_name"] == job["name"] and document["status"] == "COMMAND_SUCCEEDED",
         "foreign or failed shard handoff")
    need(exact(document["plan_file"], file_ref(plan)) and exact(document["control_artifact"],
         {k: artifacts[k] for k in ("id", "digest", "size_in_bytes")}) and
         exact(document["compiler_binary"], control["files"]["compiler"]) and
         exact(document["go_binary"], control["go_binary"]) and
         exact(document["verifier_binary"], control["files"]["verify"]), "foreign plan/compiler/Go/tool handoff")


def plan_phase(root, work, context, api, commands, go):
    run, jobs = api.run_jobs([PLAN_JOB], set())
    need(commands.run("go-version", [str(go), "version"], 15).decode().strip() == "go version go1.27.2 linux/amd64", "pinned Linux Go required")
    commands.run("bootstrap", ["sh", "tools/bootstrap.sh"], 300)
    identity = decode(commands.run("compiler-identity", [str(root / ".gooo"), "version", "--build", "--json"], 20))
    for key, value in {"schema": "gooo/build-identity/v1", "compiler_source_sha": COMPILER,
                       "vcs_revision": COMPILER, "version": "0.6.32-dev", "source_status": "CLEAN_VCS",
                       "vcs_modified": "false", "go_version": GO_VERSION, "native_go_required": GO_VERSION}.items():
        need(identity[key] == value, "pinned compiler build differs")
    shutil.copyfile(root / ".gooo", work / "compiler")
    write_json(work / "compiler-build.json", identity)
    commands.run("build-verifier", [str(go), "build", "-trimpath", "-o", str(work / "verify"), "./cmd/verify"], 180)
    (work / "verify").chmod(0o700)
    write_json(work / "context.json", context)
    commands.run("plan-full82", [str(work / "verify"), "--ci-shard-context", str(work / "context.json"),
                                "--ci-shard-count", "3", "--ci-shard-plan-out", str(work / "plan.json")], 180)
    validate_plan(read_json(work / "plan.json"), context)
    write_json(work / "control.json", {"schema": "gooo/manual-native-shard-control/v1", "context": context,
               "planner_job_id": jobs[PLAN_JOB]["id"], "go_binary": file_ref(go),
               "integration_sources": {p: file_ref(root / p) for p in INTEGRATION_SOURCES},
               "files": {p: file_ref(work / p) for p in ("context.json", "plan.json", "compiler", "verify", "compiler-build.json")},
               "producer_authenticated": False, "native_acceptance": False})


def worker_phase(root, work, context, api, commands, go, index):
    own = "native-shard-" + str(index)
    run, jobs = api.run_jobs([PLAN_JOB, own], {PLAN_JOB})
    artifact = fetch_control(api, context, jobs, run, work)
    control = shared_tools(root, work / "control", commands, go)
    commands.run("execute-full-shard", [str(work / "control/verify"), "--ci-shard-context", str(work / "control/context.json"),
                 "--ci-shard-plan", str(work / "control/plan.json"), "--ci-shard-index", str(index),
                 "--ci-shard-native-compiler", str(root / ".gooo"), "--ci-shard-execute-out", str(work / "evidence")], 1860)
    need(file_ref(root / ".gooo") == control["files"]["compiler"] and file_ref(go) == control["go_binary"], "worker tools changed")
    write_json(work / "handoff.json", {"schema": "gooo/manual-native-shard-handoff/v1", "context": context,
               "index": index, "job_id": jobs[own]["id"], "job_name": own, "status": "COMMAND_SUCCEEDED",
               "plan_file": file_ref(work / "control/plan.json"),
               "control_artifact": {k: artifact[k] for k in ("id", "digest", "size_in_bytes")},
               "compiler_binary": file_ref(root / ".gooo"), "go_binary": file_ref(go),
               "verifier_binary": file_ref(work / "control/verify"), "producer_authenticated": False})
    # Preserve the original evidence, not three duplicate tool archives in every shard artifact.
    shutil.rmtree(work / "control")
    (work / "control.zip").unlink()


def join_phase(root, work, context, api, commands, go):
    workers = ["native-shard-" + str(i) for i in range(3)]
    run, jobs = api.run_jobs([PLAN_JOB, JOIN_JOB] + workers, {PLAN_JOB, *workers})
    control_artifact = fetch_control(api, context, jobs, run, work)
    control = shared_tools(root, work / "control", commands, go)
    expected = {artifact_prefix(context) + "control": jobs[PLAN_JOB]}
    expected.update({artifact_prefix(context) + "shard-" + str(i): jobs[workers[i]] for i in range(3)})
    artifacts = api.artifacts(expected, run)
    need(artifacts[artifact_prefix(context) + "control"] == control_artifact, "planner artifact changed")
    partials, evidence = [], []
    raw_bytes = 0
    for index in range(3):
        artifact = artifacts[artifact_prefix(context) + "shard-" + str(index)]
        archive, target = work / ("shard-" + str(index) + ".zip"), work / ("shard-" + str(index))
        api.download(artifact, archive)
        raw_bytes += extract(archive, target)
        handoff_matches(read_json(target / "handoff.json"), context, work / "control/plan.json",
                        control, index, jobs[workers[index]], control_artifact)
        partials += ["--ci-shard-partial", str(target / "evidence/partial.json")]
        evidence += ["--ci-shard-evidence", str(target / "evidence")]
    base = [str(work / "control/verify"), "--ci-shard-context", str(work / "control/context.json"),
            "--ci-shard-plan", str(work / "control/plan.json")]
    commands.run("reduce-metadata", base + partials + ["--ci-shard-reduce-out", str(work / "reduction.json")], 300)
    reduced = read_json(work / "reduction.json")
    need(reduced["state"] == "CONTRACT_COMPLETE_NATIVE_UNCHECKED" and reduced["native_acceptance"] is False and
         reduced["raw_receipts_inspected"] is False and exact(reduced["run"], context) and
         reduced["cases"] == 175649 and reduced["batches"] == 2305 and len(reduced["suites"]) == 82,
         "metadata reduction scope differs")
    commands.run("inspect-original-raw", base + partials + evidence + ["--ci-shard-reference-out", str(work / "native-reference.json"),
                 "--ci-shard-verify-out", str(work / "native-verification.json")], 1200)
    verified = read_json(work / "native-verification.json")
    need(verified["state"] == "NATIVE_RECEIPTS_CHECKED_PRODUCER_UNAUTHENTICATED" and
         verified["native_acceptance"] is False and verified["producer_authenticated"] is False and
         verified["raw_receipts_inspected"] is True and exact(verified["run"], context) and
         verified["plan_sha256"] == read_json(work / "control/plan.json")["plan_sha256"] and
         verified["cases"] == 175649 and verified["batches"] == 2305, "raw verifier scope/totals differ")
    reference = work / "native-reference.json"
    validate_reference(read_json(reference), read_json(work / "control/plan.json"))
    wait_records = 0
    wait_statuses = {}
    for index in range(3):
        observations = read_json(work / ("shard-" + str(index)) / "evidence/command-observations.json")
        for suite in observations["suites"]:
            for batch in suite["batches"]:
                for phase in ("execute", "replay"):
                    status = batch["command_resources"][phase]["status"]
                    need(status in ("OBSERVED", "NOT_AVAILABLE"), "missing actual wait record")
                    wait_statuses[status] = wait_statuses.get(status, 0) + 1
                    wait_records += 1
    need(wait_records == 4610, "all2305 execute/replay wait pairs required")
    commands.run("header-host-consumer", ["python3", "-I", "-B", "tools/check-header-addition-host.py", "--root", str(root),
                 "--shard-native-index", str(reference), "--go", str(go), "--out", str(work / "fresh-host")], 300)
    commands.run("build-input-helper", [str(go), "build", "-trimpath", "-o", str(work / "header-addition-input"), "./cmd/header-addition-input"], 120)
    commands.run("header-input-consumer", ["python3", "-I", "-B", "tools/check-header-addition-inputs.py", "--root", str(root),
                 "--shard-native-index", str(reference), "--compiler", str(root / ".gooo"), "--input-command", str(work / "header-addition-input"),
                 "--go", str(go), "--out", str(work / "fresh-input-route")], 300)
    need(read_json(work / "fresh-host/summary.json")["status"] == "EXPERIMENTAL_SHARD_REFERENCE_ABI50_AND_ATOMIC_HOST_PASS" and
         read_json(work / "fresh-input-route/summary.json")["status"] == "FOUR_INPUT_ROUTES_AND16_PRELAUNCH_REFUSALS_PASS", "consumer summary differs")
    write_json(work / "result.json", {"schema": "gooo/manual-full-native-shard-experiment/v1",
               "state": "FULL82_RAW_REINSPECTED_AND_BOTH_CONSUMERS_CHECKED", "run": context,
               "official_run_jobs_artifacts_reconciled": True, "artifact_producer_job_directly_attested": False,
               "producer_authenticated": False, "native_acceptance": False,
               "suites": 82, "cases": 175649, "batches": 2305, "matrix_runners": 3,
               "exhaustive_reports_reinspected": 7, "batch_execute_replay_wait_records": wait_records,
               "batch_wait_record_statuses": wait_statuses,
               "exhaustive_Go_tests_included_in_batch_wait_count": False,
               "workers_per_runner": 3, "max_suite_callbacks_across_runners": 9,
               "compressed_input_artifact_bytes": sum(a["size_in_bytes"] for a in artifacts.values()),
               "extracted_worker_artifact_bytes": raw_bytes,
               "scope": "Separate manual experiment. Original required full CI unchanged. Official metadata and artifact digest/length reconciliation is not cryptographic producer/kernel authentication. No speedup, capacity, production or replacement-gate claim."})


def main():
    global DEADLINE
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("phase", choices=("plan", "worker", "join"))
    parser.add_argument("--root", required=True, type=Path)
    parser.add_argument("--out", required=True, type=Path)
    parser.add_argument("--go", required=True, type=Path)
    parser.add_argument("--index", type=int)
    args = parser.parse_args()
    root, work, go = args.root, args.out, args.go
    need(root.is_absolute() and root.resolve() == root and root.is_dir() and
         work.is_absolute() and work.resolve() == work and not work.exists() and
         work.parent.is_dir() and not work.is_relative_to(root) and
         work.is_relative_to(Path(os.environ["RUNNER_TEMP"]).resolve()), "fresh outside-checkout owned output required")
    need(go.is_absolute() and go.is_file() and not go.is_symlink(), "explicit regular Go executable required")
    need((args.phase == "worker" and type(args.index) is int and 0 <= args.index < 3) or
         (args.phase != "worker" and args.index is None), "exact worker index only")
    work.mkdir(mode=0o700)
    owned_limit = (8 << 30) if args.phase == "join" else (3 << 30)
    context, api, commands = None, None, None
    started = time.monotonic_ns()
    DEADLINE = time.monotonic() + {"plan": 540, "worker": 2280, "join": 2520}[args.phase]
    handlers = {number: signal.signal(number, interrupted) for number in (signal.SIGTERM, signal.SIGINT)}
    try:
        context = current_context(root)
        commands = Commands(root, work, go, owned_limit)
        api = API(context, work)
        if args.phase == "plan":
            plan_phase(root, work, context, api, commands, go)
        elif args.phase == "worker":
            worker_phase(root, work, context, api, commands, go, args.index)
        else:
            join_phase(root, work, context, api, commands, go)
        need(checkout(root, context["tested_commit"]) == context["tested_tree"], "checkout changed")
    except BaseException as error:
        write_json(work / "first-failure.json", {"schema": "gooo/manual-native-shard-first-failure/v1",
                   "type": type(error).__name__, "phase": args.phase, "error": str(error),
                   "context": context, "native_acceptance": False})
        raise
    finally:
        try:
            write_json(work / "observations.json", {"schema": "gooo/manual-native-shard-observations/v1",
                   "phase": args.phase, "elapsed_ns": time.monotonic_ns() - started,
                   "commands": commands.records if commands else [], "transfers": api.calls if api else [],
                   "producer_authenticated": False, "native_acceptance": False,
                   "selected_go": str(go),
                   "selected_goroot": str(commands.go_root) if commands else None,
                   "selected_goroot_validated": commands is not None,
                   "sampled_owned_byte_limit": owned_limit,
                   "phase_budget_seconds": {"plan": 540, "worker": 2280, "join": 2520}[args.phase],
                   "scope": "Observed command/download intervals and bytes include monitoring overhead. Logs are sampled every250ms and owned work every2s, not kernel quotas. Independent runner clocks are not combined into a legacy timing footer or a speed comparison. Hard runner kill cannot guarantee diagnostic upload."})
        finally:
            for number, handler in handlers.items():
                signal.signal(number, handler)


if __name__ == "__main__":
    main()
