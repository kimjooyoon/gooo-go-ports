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
PARENT_GENERATION = "sha256:af9452cce284a6b944adf50c62f6b9f5c1a0647d6f1794d3549dd5dae7ff43ee"
COMPILER = "83928fa4538245abe33b08549da47f8cbdc50ed2"
REPOSITORY = "https://github.com/kimjooyoon/gooo-go-ports"
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
    if count == 37:
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
    parent_files = sealed_source(source, 37, PARENT_GENERATION)
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
        parent_query = decode(recorder.success("parent-query", [go, "list", "-m", "-json",
                                                               PARENT + "@" + args.revision], work / "query", env))
        parent_version = module_identity(parent_query, PARENT, args.revision)
        original_query = decode(recorder.success("original-query", [go, "list", "-m", "-json",
                                                                   ORIGINAL + "@" + ORIGINAL_COMMIT], work / "query", env))
        module_identity(original_query, ORIGINAL, ORIGINAL_COMMIT, ORIGINAL_VERSION)
        report["resolved"] = {"parent": parent_query, "original": original_query}
        for label, module, resolved, expected, count in (
                ("parent", PARENT, parent_version, parent_files, 37),
                ("original", ORIGINAL, ORIGINAL_VERSION, original_files, 28)):
            download = decode(recorder.success(label + "-download", [go, "mod", "download", "-json",
                                                                      module + "@" + resolved], work / "query", env))
            inspect_zip(download, cache, module, resolved, expected,
                        parent_files["go.mod"] if label == "parent" else original_go_mod,
                        count, out, label)
            require(cached_file(download["GoMod"], cache).read_bytes()
                    == (parent_files["go.mod"] if label == "parent" else original_go_mod),
                    label + " downloaded go.mod differs")
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
            "summary_fields": 2, "public_asset_count": 37, "public_assets_exact": True,
            "public_generation_exact": True, "original_assets_exact": True,
            "original_generation_unchanged": True, "substitution_refused": True,
            "runtime_empty_path": True, "consumer_no_replace_or_workspace": True,
            "substitution_stdout_bytes": 0, "original_version": ORIGINAL_VERSION,
            "original_commit": ORIGINAL_COMMIT,
        }
        report["status"] = "PUBLIC_API_POLICY_CONSUMER_PASS"
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
        code = 0
    except (Exception, KeyboardInterrupt) as error:
        report["status"] = "FAIL"
        report.pop("acceptance", None)
        report["error"] = type(error).__name__ + ": " + str(error)
        diagnostic = last_step_diagnostic(out)
        if diagnostic is not None:
            report["last_step"] = diagnostic
    finally:
        report["completed_at_utc"] = utc()
        save_json(out / "verification.json", report)
        print(console_summary(report, out / "verification.json"))
    return code


if __name__ == "__main__":
    sys.exit(main())
