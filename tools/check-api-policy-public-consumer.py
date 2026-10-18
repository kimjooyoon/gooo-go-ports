#!/usr/bin/env python3
"""Observe a real public Go consumer; never publish or invoke a Gooo compiler.

The fixture checks two synthetic, unsigned historical comparisons. Hashes here
establish consistency with the checked-out sources, not code authenticity or
current authorization. A consumer-owned replacement checks known-generation
substitution refusal; it does not model an adversary controlling executable code.
"""

import argparse
import datetime
import hashlib
import importlib.util
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import signal
import stat
import subprocess
import sys
import tempfile
import time
import zipfile


PARENT = "github.com/kimjooyoon/gooo-go-ports/examples/api-policy-revision"
ORIGINAL = "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy"
ORIGINAL_COMMIT = "8cddcc4e65afb42431f8f42a86f29859ae2f4d69"
ORIGINAL_VERSION = "v0.0.0-20261014100709-8cddcc4e65af"
ORIGINAL_GENERATION = "sha256:79b76bda704cb63b9390802766a66ac72ba02c04ae135600cb05de6797c97d46"
PARENT_GENERATION = "sha256:1bbfaad1219950009acc4460490913eecf230e38338d734a771e171c59c9c760"
COMPILER = "83928fa4538245abe33b08549da47f8cbdc50ed2"
REPOSITORY = "https://github.com/kimjooyoon/gooo-go-ports"
# Exact owned fixture bytes and their independently reviewed source identities.
# These rows describe inputs; every source still compiles and executes its caller.
KNOWN_CONSUMER_FIXTURES = {
    "8c9f27b3993d0810431cf33817ad6cb1ed726738eb90a68dfc52c4876c99713f": {
        "compiler_source_sha": "83928fa4538245abe33b08549da47f8cbdc50ed2",
        "original_generation_sha256": "sha256:79b76bda704cb63b9390802766a66ac72ba02c04ae135600cb05de6797c97d46",
        "revision_generation_sha256": "sha256:a820e1c4cc249086472e792d31d12a9628bdfb56899bfc705efeca960893b64e",
    },
    "ed24455944545110d0aae02209845086b5a8aafc9ea408a07240ee576cd59617": {
        "compiler_source_sha": "83928fa4538245abe33b08549da47f8cbdc50ed2",
        "original_generation_sha256": "sha256:79b76bda704cb63b9390802766a66ac72ba02c04ae135600cb05de6797c97d46",
        "revision_generation_sha256": "sha256:af9452cce284a6b944adf50c62f6b9f5c1a0647d6f1794d3549dd5dae7ff43ee",
    },
    "ca4fc8ab3a120898ae43dd5f46bf1d4273502a77563211871ccaaa5b71662bd2": {
        "compiler_source_sha": "83928fa4538245abe33b08549da47f8cbdc50ed2",
        "original_generation_sha256": "sha256:79b76bda704cb63b9390802766a66ac72ba02c04ae135600cb05de6797c97d46",
        "revision_generation_sha256": "sha256:24f37267a7f4ad723b7228eeecd55fdc151c8534fa8a95bca19bb73d9b3973d0",
    },
    "a352120b8c419a1c56bfabcd937ce7e8c671b7356f02bd47c05595579a3dcabd": {
        "compiler_source_sha": "83928fa4538245abe33b08549da47f8cbdc50ed2",
        "original_generation_sha256": "sha256:79b76bda704cb63b9390802766a66ac72ba02c04ae135600cb05de6797c97d46",
        "revision_generation_sha256": "sha256:1bbfaad1219950009acc4460490913eecf230e38338d734a771e171c59c9c760",
    },
}

FIELDS = (
    "allowed", "reason", "authenticated", "tenant_present", "tenant_matched",
    "action_present", "action_matched", "access_allowed", "access_error",
    "access_result", "group_matched", "principal_matched", "group_missing",
    "principal_defaulted", "effective_principal",
)
MAX_STDOUT = 32 << 20
MAX_STDERR = 8 << 20
MAX_ZIP = 16 << 20
MAX_DECODED = 64 << 20
MAX_MEMBERS = 1024
MAX_FAILURE_STDERR_PREFIX = 2048
MAX_FAILURE_ERROR_PREFIX = 512
MAX_FAILURE_CONSOLE = 32 << 10


class Refused(Exception):
    pass


def require(condition, message):
    if not condition:
        raise Refused(message)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def utc():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def strict_pairs(pairs):
    result = {}
    for key, value in pairs:
        require(key not in result, "duplicate JSON member")
        result[key] = value
    return result


def invalid_constant(value):
    raise Refused("non-JSON number: " + value)


def decode(raw):
    return json.loads(raw, object_pairs_hook=strict_pairs,
                      parse_constant=invalid_constant)


def same_typed_json(actual, expected):
    if type(actual) is not type(expected):
        return False
    if isinstance(expected, dict):
        return (actual.keys() == expected.keys()
                and all(same_typed_json(actual[key], value) for key, value in expected.items()))
    if isinstance(expected, list):
        return (len(actual) == len(expected)
                and all(same_typed_json(left, right) for left, right in zip(actual, expected)))
    return actual == expected


def save_json(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n")
    path.chmod(0o600)


def file_ref(path):
    raw = path.read_bytes()
    return {"path": str(path), "bytes": len(raw), "sha256": sha(raw)}


def safe_name(name):
    path = PurePosixPath(name)
    require(bool(name) and not path.is_absolute() and "\\" not in name,
            "unsafe source path")
    require(all(part not in ("", ".", "..") for part in name.split("/")),
            "unsafe source path component")
    return name


def read_regular(root, name):
    safe_name(name)
    current = root
    require(not current.is_symlink(), "source root is a symlink")
    for part in PurePosixPath(name).parts:
        current = current / part
        require(not current.is_symlink(), "source path contains a symlink")
    require(current.is_file(), "required source is not a regular file: " + name)
    return current.read_bytes()


def sealed_source(root, count, expected_generation):
    manifest = read_regular(root, "generation.json")
    require("sha256:" + sha(manifest) == expected_generation,
            "checked-out generation differs from consumer contract")
    document = decode(manifest)
    assets = document.get("assets")
    require(isinstance(assets, dict) and len(assets) == count,
            "wrong sealed asset roster")
    result = {"generation.json": manifest}
    for name, digest in assets.items():
        require(isinstance(name, str) and isinstance(digest, str), "bad asset binding")
        raw = read_regular(root, name)
        require(digest == "sha256:" + sha(raw), "checked-out asset differs: " + name)
        result[name] = raw
    if count == 38:
        require(document.get("schema") == "gooo/api-policy-revision-generation/v1"
                and document.get("compiler_source_sha") == COMPILER,
                "unexpected parent generation schema/compiler identity")
    else:
        require(count == 28 and document.get("schema") == "gooo/cel-public-generation/v1"
                and set(document) == {"schema", "assets"},
                "unexpected frozen original generation schema")
    compiler = decode(result["compiler.json"])
    require(compiler.get("schema") == "gooo/build-identity/v1"
            and compiler.get("compiler_source_sha") == COMPILER
            and compiler.get("vcs_revision") == COMPILER
            and compiler.get("source_status") == "CLEAN_VCS"
            and compiler.get("vcs_modified") == "false"
            and compiler.get("go_version") == "go1.27.2"
            and compiler.get("native_go_required") == "go1.27.2",
            "unexpected sealed compiler build identity")
    return result


class Recorder:
    def __init__(self, out):
        self.out = out
        self.steps = []
        self.deadline = time.monotonic() + 900

    @staticmethod
    def stop(process):
        if process.poll() is None:
            try:
                os.killpg(process.pid, signal.SIGTERM)
            except ProcessLookupError:
                pass
            try:
                process.wait(timeout=2)
            except subprocess.TimeoutExpired:
                try:
                    os.killpg(process.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
                process.wait(timeout=5)

    def run(self, name, argv, cwd, env, timeout=180):
        require(time.monotonic() < self.deadline, "overall observation deadline expired")
        number = len(self.steps) + 1
        stdout_path = self.out / (f"{number:02d}-" + name + ".stdout")
        stderr_path = self.out / (f"{number:02d}-" + name + ".stderr")
        record = {"name": name, "command": [str(arg) for arg in argv],
                  "cwd": str(cwd), "started_at_utc": utc()}
        self.steps.append(record)
        process = None
        failure = None
        try:
            with stdout_path.open("xb") as stdout, stderr_path.open("xb") as stderr:
                stdout_path.chmod(0o600)
                stderr_path.chmod(0o600)
                process = subprocess.Popen(argv, cwd=cwd, env=env, stdin=subprocess.DEVNULL,
                                           stdout=stdout, stderr=stderr, start_new_session=True)
                deadline = min(self.deadline, time.monotonic() + timeout)
                while process.poll() is None:
                    if (time.monotonic() >= deadline or stdout_path.stat().st_size > MAX_STDOUT
                            or stderr_path.stat().st_size > MAX_STDERR):
                        failure = "subprocess time/output bound exceeded"
                        self.stop(process)
                        break
                    time.sleep(0.05)
                record["exit_code"] = process.wait(timeout=5)
        except BaseException:
            if process is not None:
                self.stop(process)
            raise
        finally:
            record["completed_at_utc"] = utc()
            if stdout_path.exists():
                record["stdout"] = file_ref(stdout_path)
            if stderr_path.exists():
                record["stderr"] = file_ref(stderr_path)
            save_json(self.out / "steps.json", self.steps)
        require(failure is None, failure or "subprocess refused")
        require(stdout_path.stat().st_size <= MAX_STDOUT and stderr_path.stat().st_size <= MAX_STDERR,
                "subprocess final output bound exceeded")
        return record["exit_code"], stdout_path.read_bytes(), stderr_path.read_bytes()

    def success(self, name, argv, cwd, env, timeout=180):
        code, stdout, _ = self.run(name, argv, cwd, env, timeout)
        require(code == 0, name + " returned nonzero; inspect saved step logs")
        return stdout

    def public_module_query(self, name, go, module, revision, cwd, env):
        require((name, module) in (("parent-query", PARENT), ("original-query", ORIGINAL))
                and re.fullmatch(r"[0-9a-f]{40}", revision) is not None,
                "public query must retain a known module and exact source SHA")
        require(env.get("GOPROXY") == "https://proxy.golang.org"
                and env.get("GOSUMDB") == "sum.golang.org",
                "public query must retain the authenticated proxy environment")
        argv = [go, "list", "-m", "-json", module + "@" + revision]
        code, stdout, stderr = self.run(name, argv, cwd, env)
        transient = (f"go: {module}@{revision}: invalid version: reading "
                     f"https://proxy.golang.org/{module}/@v/{revision}.info: 404 Not Found\n"
                     "\tserver response: not found: fetch timed out\n").encode()
        if code == 1 and not stdout and stderr == transient:
            # Keep the first numbered streams/step; retry only this exact remote
            # lookup, using the same SHA, arguments, proxy and checksum policy.
            require(time.monotonic() + 5 < self.deadline,
                    "overall deadline leaves no room for public query backoff")
            time.sleep(5)
            name += "-retry-1"
            code, stdout, _ = self.run(name, argv, cwd, env)
        require(code == 0, name + " returned nonzero; inspect saved step logs")
        return stdout


def module_identity(value, path, revision, version=None):
    require(isinstance(value, dict) and value.get("Path") == path,
            "module resolver returned a different module")
    require(value.get("GoVersion") == "1.27.2", "module requires an unexpected Go version")
    require(isinstance(value.get("Version"), str) and value["Version"],
            "resolver omitted actual version")
    if version is not None:
        require(value["Version"] == version, "original resolver version differs")
    origin = value.get("Origin")
    require(isinstance(origin, dict) and origin.get("VCS") == "git"
            and origin.get("URL") == REPOSITORY and origin.get("Hash") == revision
            and origin.get("Subdir") == path.split("gooo-go-ports/", 1)[1],
            "resolver origin does not bind the requested repository/commit/subdirectory")
    require(not value.get("Replace"), "resolver unexpectedly substituted a module")
    return value["Version"]


def check_install_build_info(value, parent_version, parent_sum, original_sum):
    require(isinstance(value, dict) and value.get("GoVersion") == "go1.27.2"
            and value.get("Path") == PARENT + "/cmd/compare",
            "installed command toolchain/package identity differs")
    main = value.get("Main")
    dependencies = value.get("Deps")
    require(isinstance(main, dict) and main.get("Path") == PARENT
            and main.get("Version") == parent_version and main.get("Sum") == parent_sum
            and main.get("Replace") is None,
            "installed command main module differs from authenticated download")
    require(isinstance(dependencies, list) and len(dependencies) == 1,
            "installed command has an unexpected dependency graph")
    original = dependencies[0]
    require(isinstance(original, dict) and original.get("Path") == ORIGINAL
            and original.get("Version") == ORIGINAL_VERSION
            and original.get("Sum") == original_sum and original.get("Replace") is None,
            "installed command original module differs from authenticated download")
    return {"go_version": value["GoVersion"], "package": value["Path"],
            "main": {key: main[key] for key in ("Path", "Version", "Sum")},
            "dependencies": [{key: original[key] for key in ("Path", "Version", "Sum")}],
            "no_replacements": True}


def install_versioned_cli(recorder, go, parent_version, public_sources, work, env):
    install_cwd, private_bin = work / "install-cwd", work / "private-bin"
    install_cwd.mkdir(mode=0o700)
    private_bin.mkdir(mode=0o700)
    require(not list(install_cwd.iterdir()) and not list(private_bin.iterdir()),
            "versioned installation directories are not empty")
    install_env = dict(env, GOBIN=str(private_bin))
    target = PARENT + "/cmd/compare@" + parent_version
    recorder.success("direct-install", [go, "install", target], install_cwd, install_env, 300)
    require(not list(install_cwd.iterdir()), "versioned installation changed the empty working directory")
    binary = private_bin / ("compare.exe" if os.name == "nt" else "compare")
    require(binary.is_file() and not binary.is_symlink() and os.access(binary, os.X_OK),
            "versioned installation did not produce a regular executable")
    build_info_raw = recorder.success("direct-install-build-info", [go, "version", "-m", "-json", binary],
                                      install_cwd, install_env, 30)
    build_info = check_install_build_info(decode(build_info_raw), parent_version,
                                         public_sources["parent"]["sum"], public_sources["original"]["sum"])
    return binary, {"target": target, "working_directory": str(install_cwd),
                    "gobin": str(private_bin), "working_directory_empty": True,
                    "binary": file_ref(binary), "build_info": build_info,
                    "build_info_source": recorder.steps[-1]["stdout"]}


def installed_cli_output(recorder, binary, mode, input_path, work, env, label):
    code, stdout, stderr = recorder.run(label, [binary, "--diagnostics=json", "--mode", mode,
                                               "--input", input_path], work / "fresh-cwd", env, 30)
    require(code == 0 and not stderr, label + " failed or emitted stderr")
    return stdout


def check_installed_cli(recorder, binary, observed, work, env):
    inputs = work / "direct-install-inputs"
    inputs.mkdir(mode=0o700)
    cases = []
    for case in observed["cases"]:
        action, expected = case["action"], case["report"]
        facts_path, request_path = inputs / (action + "-facts.json"), inputs / (action + "-request.json")
        report_path = inputs / (action + "-report.json")
        save_json(facts_path, expected["before"]["facts"])
        baseline_raw = installed_cli_output(recorder, binary, "baseline", facts_path, work, env,
                                            "direct-" + action + "-baseline")
        baseline = {"before_profile": expected["before_profile"],
                    "candidate_profile": expected["candidate_profile"], "before": expected["before"]}
        require(same_typed_json(decode(baseline_raw), baseline), "installed baseline differs from checked consumer")
        request_path.write_bytes(baseline_raw)
        request_path.chmod(0o600)
        compared_raw = installed_cli_output(recorder, binary, "compare", request_path, work, env,
                                            "direct-" + action + "-compare")
        require(same_typed_json(decode(compared_raw), expected), "installed comparison differs from checked consumer")
        report_path.write_bytes(compared_raw)
        report_path.chmod(0o600)
        replay_raw = installed_cli_output(recorder, binary, "replay", report_path, work, env,
                                          "direct-" + action + "-replay")
        require(replay_raw == compared_raw and same_typed_json(decode(replay_raw), expected),
                "installed saved replay bytes or complete report differ")
        cases.append({"action": action, "baseline_complete": True, "comparison_complete": True,
                      "replay_bytes_exact": True, "report_sha256": sha(compared_raw)})
    write_report = next(case["report"] for case in observed["cases"] if case["action"] == "write")
    tampered = decode(json.dumps(write_report).encode())
    tampered["differences"]["reason"] = not tampered["differences"]["reason"]
    tampered_path = inputs / "write-tampered-report.json"
    save_json(tampered_path, tampered)
    code, stdout, stderr = recorder.run("direct-write-tampered-replay",
                                       [binary, "--diagnostics=json", "--mode", "replay", "--input", tampered_path],
                                       work / "fresh-cwd", env, 30)
    require(code == 1 and not stdout, "installed CLI accepted a tampered report or emitted a result")
    diagnostic = decode(stderr)
    require(isinstance(diagnostic, dict)
            and set(diagnostic) == {"schema", "command", "mode", "status", "diagnostics"}
            and diagnostic["schema"] == "gooo/api-policy-revision-diagnostic/v1"
            and diagnostic["command"] == "compare" and diagnostic["mode"] == "replay"
            and diagnostic["status"] == "REFUSED", "installed refusal diagnostic context differs")
    details = diagnostic["diagnostics"]
    require(isinstance(details, list) and len(details) == 1 and isinstance(details[0], dict)
            and set(details[0]) == {"severity", "code", "stage", "message"}
            and details[0]["severity"] == "error" and details[0]["code"] == "REPORT_MISMATCH"
            and details[0]["stage"] == "verify_comparison"
            and isinstance(details[0]["message"], str) and bool(details[0]["message"]),
            "installed tamper refusal code/stage differs")
    return {"cases": cases, "runtime_empty_path": env["PATH"] == "",
            "batch": check_installed_batch(recorder, binary, observed, work, env),
            "tampered_report": {"field": "differences.reason", "exit_code": code,
                                "stdout_bytes": len(stdout), "diagnostic": diagnostic}}


def check_installed_batch(recorder, binary, observed, work, env):
    # Compare each complete result with the independently executed Go caller,
    # including an intentional duplicate to check order/multiplicity retention.
    by_action = {case["action"]: case["report"] for case in observed["cases"]}
    expected = [by_action[action] for action in ("write", "read", "write")]
    input_path = work / "batch-facts.json"
    save_json(input_path, [report["before"]["facts"] for report in expected])
    expected_baseline = [{"before_profile": report["before_profile"],
                          "candidate_profile": report["candidate_profile"], "before": report["before"]}
                         for report in expected]
    compared = None
    for mode in ("baseline", "compare", "replay"):
        code, stdout, stderr = recorder.run("batch-" + mode,
            [binary, "--diagnostics=json", "--batch", "--mode", mode, "--input", input_path],
            work / "fresh-cwd", env, 30)
        require(code == 0 and not stderr and same_typed_json(decode(stdout),
                expected_baseline if mode == "baseline" else expected), "installed batch " + mode + " differs")
        if mode == "replay":
            require(stdout == compared, "installed batch replay bytes differ")
        if mode == "compare":
            compared = stdout
        input_path = work / ("batch-" + mode + ".json")
        input_path.write_bytes(stdout)
        input_path.chmod(0o600)
    tampered = decode(compared)
    tampered[2]["differences"]["reason"] = not tampered[2]["differences"]["reason"]
    tampered_path = work / "batch-tampered.json"
    save_json(tampered_path, tampered)
    code, stdout, stderr = recorder.run("batch-late-tampered-replay",
        [binary, "--diagnostics=json", "--batch", "--mode", "replay", "--input", tampered_path],
        work / "fresh-cwd", env, 30)
    require(code == 1 and not stdout, "installed batch emitted partial results after a late refusal")
    diagnostic = decode(stderr)
    details = diagnostic.get("diagnostics")
    require(type(diagnostic) is dict
            and set(diagnostic) == {"schema", "command", "mode", "status", "diagnostics"}
            and diagnostic.get("schema") == "gooo/api-policy-revision-diagnostic/v1"
            and diagnostic.get("command") == "compare" and diagnostic.get("mode") == "replay"
            and diagnostic.get("status") == "REFUSED" and type(details) is list and len(details) == 1,
            "installed batch refusal context differs")
    detail = details[0]
    require(type(detail) is dict and set(detail) == {"severity", "code", "stage", "message", "item_index"}
            and detail.get("severity") == "error" and type(detail.get("message")) is str and bool(detail["message"])
            and detail.get("code") == "REPORT_MISMATCH" and detail.get("stage") == "verify_comparison"
            and type(detail.get("item_index")) is int and detail["item_index"] == 2,
            "installed batch first failing index/code differs")
    return {"rows": 3, "order": ["write", "read", "write"], "complete_records_exact": True,
            "replay_bytes_exact": True, "comparison_sha256": sha(compared),
            "late_tampered_report": {"exit_code": code, "stdout_bytes": len(stdout), "diagnostic": diagnostic}}


def cached_file(value, cache):
    path = Path(value)
    require(path.is_absolute() and path.is_file() and not path.is_symlink(),
            "download metadata does not name a regular cache file")
    require(path.resolve().is_relative_to(cache.resolve()), "download escaped module cache")
    return path


def inspect_zip(download, cache, module, version, expected, go_mod, asset_count, out, label):
    require(download.get("Path") == module and download.get("Version") == version
            and not download.get("Error"), "download metadata identity/error mismatch")
    for key in ("Sum", "GoModSum"):
        require(isinstance(download.get(key), str)
                and re.fullmatch(r"h1:[A-Za-z0-9+/]{43}=", download[key]),
                "download lacks an authenticated Go module checksum")
    archive = cached_file(download.get("Zip", ""), cache)
    require(archive.stat().st_size <= MAX_ZIP, "module archive exceeds compressed bound")
    prefix = module + "@" + version + "/"
    found = {}
    roster = []
    runtime_names = set()
    go_mod_seen = False
    with zipfile.ZipFile(archive) as zipped:
        infos = zipped.infolist()
        require(len(infos) <= MAX_MEMBERS, "module archive member bound exceeded")
        require(sum(info.file_size for info in infos) <= MAX_DECODED,
                "module archive decoded bound exceeded")
        seen = set()
        for info in infos:
            require(info.filename.startswith(prefix), "module archive root mismatch")
            name = info.filename[len(prefix):]
            require(info.filename not in seen, "duplicate module archive member")
            seen.add(info.filename)
            if info.is_dir():
                safe_name(name.rstrip("/"))
                continue
            safe_name(name)
            require(not stat.S_ISLNK(info.external_attr >> 16), "symlink in module archive")
            require(info.file_size <= MAX_DECODED, "oversized module archive member")
            raw = zipped.read(info)
            require(len(raw) == info.file_size, "module member decoded length differs")
            if name == "go.mod":
                require(raw == go_mod, "module zip go.mod differs")
                go_mod_seen = True
            roster.append({"path": name, "bytes": len(raw), "sha256": sha(raw)})
            if name.endswith(".go") and not name.endswith("_test.go"):
                runtime_names.add(name)
            if name in expected:
                require(raw == expected[name], "public module source differs: " + name)
                found[name] = raw
            if module == PARENT:
                require(not name.startswith("profiles/original/"),
                        "nested original unexpectedly included in parent module zip")
    require(set(found) == set(expected), "public module is missing required source assets")
    require(go_mod_seen, "public module zip omits its source go.mod")
    expected_runtime = {name for name in expected if name.endswith(".go") and not name.endswith("_test.go")}
    require(runtime_names == expected_runtime, "public module has unsealed runtime Go source")
    require(len(expected) == asset_count + 1, "unexpected source inspection roster")
    evidence = {"module": module, "version": version, "archive": file_ref(archive),
                "sum": download["Sum"], "go_mod_sum": download["GoModSum"],
                "members": roster, "sealed_assets": asset_count,
                "generation_exact": True, "all_sealed_bytes_exact": True}
    save_json(out / (label + "-public-source.json"), evidence)
    return evidence


def json_stream(raw):
    text = raw.decode("utf-8")
    decoder = json.JSONDecoder(object_pairs_hook=strict_pairs, parse_constant=invalid_constant)
    result = []
    index = 0
    while index < len(text):
        while index < len(text) and text[index].isspace():
            index += 1
        if index == len(text):
            break
        value, index = decoder.raw_decode(text, index)
        result.append(value)
    return result


def verify_fixture_identities(raw, wanted):
    """Bind this fixed fixture's raw source to independently read manifests.

    Unknown source bytes require a reviewed source contract. This function does
    not parse Go, attest code authenticity or replace fresh native verification.
    """
    source_hash = sha(raw)
    known = KNOWN_CONSUMER_FIXTURES.get(source_hash)
    require(known is not None, "consumer fixture source hash is not registered: " + source_hash)
    require(same_typed_json(known, wanted),
            "consumer fixture identity tuple differs from current source manifests")


def check_observations(raw, parent_assets, original_assets):
    observed = decode(raw)
    require(set(observed) == {"schema", "scope", "cases"}
            and observed["schema"] == "gooo/api-policy-consumer-observation/v1"
            and observed["scope"] == "synthetic_unsigned_historical_hypothetical_readonly",
            "consumer output schema/scope differs")
    cases = observed["cases"]
    require(isinstance(cases, list) and len(cases) == 2, "consumer did not record both cases")
    for case, action in zip(cases, ("read", "write")):
        require(set(case) == {"action", "report"} and case["action"] == action,
                "consumer case identity differs")
        report = case["report"]
        require(set(report) == {"schema", "status", "scope", "before_profile", "candidate_profile",
                               "before", "after", "differences", "summary", "classifier_bindings"},
                "comparison report shape differs")
        require(report["schema"] == "gooo/api-policy-revision-comparison/v1"
                and report["status"] == "COMPARED"
                and report["scope"] == "same_saved_facts_two_sealed_profiles"
                and report["before_profile"] == "cel-api-original-v1"
                and report["candidate_profile"] == "cel-api-readonly-principal-v1", "comparison identities differ")
        facts = {"group_present": True, "group_value": "admin", "principal_present": True,
                 "principal_value": "synthetic:user", "authenticated": True,
                 "caller_tenant": "tenant-a", "resource_tenant": "tenant-a",
                 "requested_action": action, "allowed_action": action}
        before = {"allowed": True, "reason": "ALLOW", "authenticated": True,
                  "tenant_present": True, "tenant_matched": True, "action_present": True,
                  "action_matched": True, "access_allowed": True, "access_error": False,
                  "access_result": "ALLOW", "group_matched": True, "principal_matched": False,
                  "group_missing": False, "principal_defaulted": False,
                  "effective_principal": "synthetic:user"}
        after = dict(before)
        flags = dict.fromkeys(FIELDS, False)
        summary = {"classification": "UNCHANGED", "changed_count": 0}
        if action == "write":
            after.update(allowed=False, reason="ACTION_NOT_READ_ONLY")
            flags.update(allowed=True, reason=True)
            summary = {"classification": "NEWLY_DENIED", "changed_count": 2}
        for side, decision, assets, generation in (
                ("before", before, original_assets, ORIGINAL_GENERATION),
                ("after", after, parent_assets, PARENT_GENERATION)):
            proof = report[side]
            require(set(proof) == {"facts", "decision", "bindings"}, "proof shape differs")
            require(same_typed_json(proof["facts"], facts) and same_typed_json(proof["decision"], decision),
                    "consumer raw facts or complete decision differ")
            expected_bindings = {"compiler_source_sha": COMPILER,
                                 "generation_sha256": generation, "assets": assets}
            require(proof["bindings"] == expected_bindings, "complete proof source bindings differ")
        require(same_typed_json(report["differences"], flags) and same_typed_json(report["summary"], summary),
                "complete classifier flags or summary differ")
        require(report["classifier_bindings"] == report["after"]["bindings"],
                "complete classifier bindings differ")
    return observed


def cleanup_error(function, path, _error):
    # Go's default module cache contains readonly directories as well as files.
    os.chmod(Path(path).parent, 0o700)
    if not Path(path).is_symlink():
        os.chmod(path, 0o700)
    function(path)


def verify(args, out, report):
    root = Path(__file__).resolve().parents[1]
    source = root / "examples/api-policy-revision"
    fixture = root / "testdata/api-policy-consumer/main.go.txt"
    require(Path(args.go).is_absolute(), "--go must be an absolute installed executable")
    go = Path(args.go).resolve(strict=True)
    require(go.is_file() and os.access(go, os.X_OK), "--go is not an installed executable")
    require(re.fullmatch(r"[0-9a-f]{40}", args.revision) is not None,
            "--revision must be a full lowercase commit SHA")
    parent_files = sealed_source(source, 38, PARENT_GENERATION)
    original_files = sealed_source(source / "profiles/original", 28, ORIGINAL_GENERATION)
    parent_assets = decode(parent_files["generation.json"])["assets"]
    original_assets = decode(original_files["generation.json"])["assets"]
    require(f"require {ORIGINAL} {ORIGINAL_VERSION}\n".encode() in parent_files["go.mod"],
            "parent does not require the real original module version")
    original_go_mod = read_regular(source / "profiles/original", "go.mod")
    fixture_raw = read_regular(root, "testdata/api-policy-consumer/main.go.txt")
    report["source"] = {"helper": file_ref(Path(__file__).resolve()), "consumer_fixture": file_ref(fixture),
                        "current_parent_generation": PARENT_GENERATION,
                        "current_original_generation": ORIGINAL_GENERATION}
    wanted_fixture_identities = {
        "compiler_source_sha": decode(parent_files["generation.json"])["compiler_source_sha"],
        "original_generation_sha256": "sha256:" + sha(original_files["generation.json"]),
        "revision_generation_sha256": "sha256:" + sha(parent_files["generation.json"]),
    }
    verify_fixture_identities(fixture_raw, wanted_fixture_identities)
    report["consumer_fixture_identities_verified"] = True
    report["consumer_fixture_source_contract"] = {
        "source_sha256": sha(fixture_raw), "identities": wanted_fixture_identities,
    }
    recorder = Recorder(out)
    work = Path(tempfile.mkdtemp(prefix="gooo-public-consumer-"))
    work.chmod(0o700)
    report["temporary_work_directory"] = str(work)
    try:
        for name in ("home", "tmp", "gopath", "build-cache", "query", "consumer", "fresh-cwd"):
            (work / name).mkdir(mode=0o700)
        if args.module_cache:
            cache = Path(args.module_cache)
            require(cache.is_absolute() and cache.is_dir() and not cache.is_symlink(),
                    "--module-cache must name an existing absolute directory")
            cache = cache.resolve()
        else:
            cache = work / "module-cache"
            cache.mkdir(mode=0o700)
        report["module_cache"] = {"path": str(cache), "retained_explicit_cache": bool(args.module_cache)}
        env = {"PATH": str(go.parent), "HOME": str(work / "home"), "TMPDIR": str(work / "tmp"),
               "GOPATH": str(work / "gopath"), "GOCACHE": str(work / "build-cache"),
               "GOMODCACHE": str(cache), "GOTOOLCHAIN": "local", "GOENV": "off", "GOWORK": "off",
               "GOPROXY": "https://proxy.golang.org", "GOSUMDB": "sum.golang.org",
               "GONOSUMDB": "", "GONOPROXY": "", "GOPRIVATE": "", "GOINSECURE": "",
               "GOTELEMETRY": "off", "CGO_ENABLED": "0"}
        report["go_environment"] = env
        version = recorder.success("installed-go", [go, "version"], work / "query", env)
        require(re.fullmatch(rb"go version go1\.27\.2 [a-z0-9]+/[a-z0-9]+\n", version) is not None,
                "installed Go is not exactly 1.27.2")
        report["go_toolchain"] = {"executable": file_ref(go), "version": version.decode().strip()}
        parent_query = decode(recorder.public_module_query("parent-query", go, PARENT,
                                                          args.revision, work / "query", env))
        parent_version = module_identity(parent_query, PARENT, args.revision)
        original_query = decode(recorder.public_module_query("original-query", go, ORIGINAL,
                                                            ORIGINAL_COMMIT, work / "query", env))
        module_identity(original_query, ORIGINAL, ORIGINAL_COMMIT, ORIGINAL_VERSION)
        report["resolved"] = {"parent": parent_query, "original": original_query}
        public_sources = {}
        for label, module, resolved, expected, count in (
                ("parent", PARENT, parent_version, parent_files, 38),
                ("original", ORIGINAL, ORIGINAL_VERSION, original_files, 28)):
            download = decode(recorder.success(label + "-download", [go, "mod", "download", "-json",
                                                                      module + "@" + resolved], work / "query", env))
            public_sources[label] = inspect_zip(download, cache, module, resolved, expected,
                                               parent_files["go.mod"] if label == "parent" else original_go_mod,
                                               count, out, label)
            require(cached_file(download["GoMod"], cache).read_bytes()
                    == (parent_files["go.mod"] if label == "parent" else original_go_mod),
                    label + " downloaded go.mod differs")
        installed_binary, report["direct_install"] = install_versioned_cli(
            recorder, go, parent_version, public_sources, work, env)
        consumer = work / "consumer"
        (consumer / "main.go").write_bytes(fixture_raw)
        (consumer / "go.mod").write_text("module example.invalid/gooo-api-policy-public-consumer\n\ngo 1.27.2\n\n"
                                        + "require " + PARENT + " " + parent_version + "\n")
        recorder.success("consumer-tidy", [go, "mod", "tidy"], consumer, env)
        modules = json_stream(recorder.success("consumer-selected-modules", [go, "list", "-m", "-json", "all"], consumer, env))
        require(len(modules) == 3 and all(not item.get("Replace") for item in modules),
                "consumer module graph includes replacement or unexpected dependency")
        selected = {item["Path"]: item for item in modules}
        require(set(selected) == {"example.invalid/gooo-api-policy-public-consumer", PARENT, ORIGINAL}
                and selected[PARENT].get("Version") == parent_version
                and selected[ORIGINAL].get("Version") == ORIGINAL_VERSION,
                "actual selected module graph differs")
        require("replace" not in (consumer / "go.mod").read_text()
                and not (consumer / "go.work").exists(), "consumer gained local module overrides")
        save_json(out / "selected-modules.json", modules)
        (out / "consumer.go.mod").write_bytes((consumer / "go.mod").read_bytes())
        (out / "consumer.go.sum").write_bytes((consumer / "go.sum").read_bytes())
        binary = work / "consumer-positive"
        recorder.success("consumer-build", [go, "build", "-mod=readonly", "-trimpath", "-o", binary, "."], consumer, env, 300)
        report["positive_binary"] = file_ref(binary)
        run_env = {"PATH": "", "HOME": str(work / "home"), "TMPDIR": str(work / "tmp")}
        code, stdout, stderr = recorder.run("consumer-positive", [binary], work / "fresh-cwd", run_env, 30)
        require(code == 0 and not stderr, "actual external consumer failed or emitted stderr")
        observed = check_observations(stdout, parent_assets, original_assets)
        save_json(out / "observations.json", observed)
        report["direct_install"].update(check_installed_cli(recorder, installed_binary, observed, work, run_env))

        # Copy only the source-owned exact 33-file distribution. Never alter the
        # downloaded module/cache. This exercises a consumer main's own replace.
        alternate = consumer / "alternate-original"
        alternate.mkdir(mode=0o700)
        profile = decode(read_regular(source, "profiles.json"))["original"]
        packaging = profile["packaging_assets"]
        require(isinstance(packaging, dict) and len(packaging) == 4, "frozen packaging roster differs")
        frozen = dict(original_files)
        for name, digest in packaging.items():
            raw = read_regular(source / "profiles/original", name)
            require(digest == "sha256:" + sha(raw), "frozen packaging differs")
            frozen[name] = raw
        require(len(frozen) == 33, "alternate fixture is not the exact 33-file distribution")
        disk_names = set()
        for path in (source / "profiles/original").rglob("*"):
            require(not path.is_symlink(), "frozen distribution contains a symlink")
            if path.is_file():
                disk_names.add(path.relative_to(source / "profiles/original").as_posix())
        require(disk_names == set(frozen), "frozen physical distribution roster differs")
        for name, raw in frozen.items():
            target = alternate / name
            target.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
            target.write_bytes(raw)
        (alternate / "generation.json").write_bytes(frozen["generation.json"] + b"\n")
        changed = [name for name, raw in frozen.items() if (alternate / name).read_bytes() != raw]
        require(changed == ["generation.json"], "alternate fixture changes more than raw generation whitespace")
        with (consumer / "go.mod").open("a") as stream:
            stream.write("\nreplace " + ORIGINAL + " => ./alternate-original\n")
        offline = dict(env, GOPROXY="off", GOSUMDB="off")
        substituted_binary = work / "consumer-substituted"
        recorder.success("substituted-build-offline", [go, "build", "-mod=readonly", "-trimpath", "-o",
                                                        substituted_binary, "."], consumer, offline, 300)
        report["substituted_binary"] = file_ref(substituted_binary)
        code, stdout, stderr = recorder.run("consumer-substituted", [substituted_binary], work / "fresh-cwd", run_env, 30)
        require(code == 1 and not stdout
                and b"revision bindings: API revision source bindings invalid" in stderr,
                "known-generation substitution was not refused at the expected binding gate")
        report["substitution"] = {"scope": "consumer_owned_known_generation_substitution_not_adversarial_code_authenticity",
                                  "changed_files": changed, "baseline_generation_sha256": sha(frozen["generation.json"]),
                                  "substituted_generation_sha256": sha(frozen["generation.json"] + b"\n"),
                                  "exit_code": code, "stdout_bytes": len(stdout), "stderr_bytes": len(stderr)}
        report["acceptance"] = {
            "facts": 9, "positive_cases": 2, "decision_fields": 15, "difference_flags": 15,
            "summary_fields": 2, "public_asset_count": 38, "public_assets_exact": True,
            "public_generation_exact": True, "original_assets_exact": True,
            "original_generation_unchanged": True, "substitution_refused": True,
            "runtime_empty_path": True, "consumer_no_replace_or_workspace": True,
            "direct_install": True, "direct_install_cases": 2,
            "direct_install_tampered_report_refused": True,
            "direct_install_batch_rows": 3, "direct_install_batch_tampered_report_refused": True,
            "substitution_stdout_bytes": 0, "original_version": ORIGINAL_VERSION,
            "original_commit": ORIGINAL_COMMIT,
        }
        report["status"] = "PUBLIC_API_POLICY_CONSUMER_PASS"
        if getattr(args, "bundle_cli", False):
            # Retain only the exact installed executable already checked above.
            # Packaging occurs in main only after private-work cleanup succeeds.
            retained = out / ".validated-cli"
            with installed_binary.open("rb") as source_file, retained.open("xb") as target_file:
                shutil.copyfileobj(source_file, target_file)
            retained.chmod(0o700)
            copied = file_ref(retained)
            require(all(copied[key] == report["direct_install"]["binary"][key]
                        for key in ("bytes", "sha256")), "retained CLI differs from the tested executable")
            report["bundle_facts"] = {case["action"]: case["report"]["before"]["facts"]
                                      for case in observed["cases"]}
    finally:
        shutil.rmtree(work, onerror=cleanup_error)
        report["temporary_sources_binaries_build_cache_removed"] = not work.exists()


def bounded_text(value, limit):
    if not isinstance(value, str):
        return ""
    return value.encode("utf-8", errors="replace")[:limit].decode("utf-8", errors="replace")


def last_step_diagnostic(out):
    """Best-effort, bounded local diagnostics; never change a failed decision."""
    try:
        path = out / "steps.json"
        if not path.exists():
            return None
        require(path.is_file() and not path.is_symlink(), "unsafe diagnostic step file")
        with path.open("rb") as stream:
            raw = stream.read((1 << 20) + 1)
        require(len(raw) <= 1 << 20, "diagnostic step file exceeds bound")
        steps = decode(raw)
        require(isinstance(steps, list), "diagnostic steps must be an array")
        if not steps:
            return None
        step = steps[-1]
        require(isinstance(step, dict), "diagnostic step must be an object")
        exit_code = step.get("exit_code")
        detail = {"name": bounded_text(step.get("name"), 64),
                  "exit_code": exit_code if type(exit_code) is int and -255 <= exit_code <= 255 else None}
        stderr = step.get("stderr")
        if not isinstance(stderr, dict):
            detail["stderr_unavailable"] = True
            return detail
        stderr_path = Path(stderr.get("path", ""))
        require(stderr_path.parent == out and stderr_path.is_file()
                and not stderr_path.is_symlink(), "unsafe diagnostic stderr path")
        with stderr_path.open("rb") as stream:
            prefix = stream.read(MAX_FAILURE_STDERR_PREFIX + 1)
        detail.update(stderr_prefix=prefix[:MAX_FAILURE_STDERR_PREFIX].decode("utf-8", errors="replace"),
                      stderr_truncated=len(prefix) > MAX_FAILURE_STDERR_PREFIX,
                      stderr_prefix_limit_bytes=MAX_FAILURE_STDERR_PREFIX)
        return detail
    except Exception as error:
        return {"unavailable": type(error).__name__}


def console_summary(report, verification):
    summary = {"status": report["status"], "verification": str(verification)}
    if report["status"] != "FAIL":
        return json.dumps(summary)
    if report["status"] == "FAIL":
        summary["verification"] = bounded_text(summary["verification"], 1024)
        summary["error"] = bounded_text(report.get("error"), MAX_FAILURE_ERROR_PREFIX)
        if report.get("last_step") is not None:
            summary["last_step"] = report["last_step"]
    # Escaped controls stay on one JSON line; double colons cannot form an
    # injected GitHub workflow command in third-party subprocess stderr.
    encoded = json.dumps(summary, ensure_ascii=True).replace("::", r"\u003a\u003a")
    if len(encoded.encode("utf-8")) > MAX_FAILURE_CONSOLE:
        encoded = json.dumps({"status": report["status"],
                              "verification": bounded_text(str(verification), 1024),
                              "error": "Diagnostic summary exceeds bound; inspect saved verification.json"})
    return encoded


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--go", required=True, help="absolute installed Go 1.27.2 executable")
    parser.add_argument("--revision", required=True, help="exact public parent full commit SHA")
    parser.add_argument("--out", required=True, help="new directory for stable evidence")
    parser.add_argument("--module-cache", help="existing absolute cache to reuse and retain")
    parser.add_argument("--bundle-cli", action="store_true",
                        help="package the already tested native CLI after successful private-work cleanup")
    args = parser.parse_args()
    out = Path(args.out).absolute()
    require(not out.exists() and not out.is_symlink(), "--out must be a new directory")
    out.mkdir(mode=0o700, parents=True)
    report = {"schema": "gooo/api-policy-public-consumer/v1", "status": "FAIL",
              "started_at_utc": utc(), "requested_revision": args.revision,
              "scope": "finite_synthetic_unsigned_historical_hypothetical_readonly",
              "no_gooo_compiler_or_toolchain_download": True,
              "authority_or_effect_claim": False}
    code = 1
    try:
        verify(args, out, report)
        require(report.get("temporary_sources_binaries_build_cache_removed") is True,
                "temporary cleanup incomplete")
        if args.bundle_cli:
            helper_path = Path(__file__).with_name("api_policy_cli_bundle.py")
            spec = importlib.util.spec_from_file_location("api_policy_cli_bundle", helper_path)
            helper = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(helper)
            report["cli_bundle"] = helper.make_bundle(out / ".validated-cli", out, report,
                                                       Path(args.go).resolve(), helper_path.parents[1])
        code = 0
    except (Exception, KeyboardInterrupt) as error:
        report["status"] = "FAIL"
        report.pop("acceptance", None)
        report["error"] = type(error).__name__ + ": " + str(error)
        diagnostic = last_step_diagnostic(out)
        if diagnostic is not None:
            report["last_step"] = diagnostic
    finally:
        try:
            (out / ".validated-cli").unlink(missing_ok=True)
        except OSError as error:
            code = 1
            report["status"] = "FAIL"
            report.pop("acceptance", None)
            report["retained_cli_cleanup_error"] = type(error).__name__ + ": " + str(error)
            report.setdefault("error", report["retained_cli_cleanup_error"])
            if "cli_bundle" in report:
                report["unaccepted_cli_bundle"] = report.pop("cli_bundle")
        report["completed_at_utc"] = utc()
        save_json(out / "verification.json", report)
        print(console_summary(report, out / "verification.json"))
    return code


if __name__ == "__main__":
    sys.exit(main())
