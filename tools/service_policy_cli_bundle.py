"""Package and check the pinned, ordinary service-grant CLI on its native host.

No compiler bootstrap or Go build is performed here. The supplied executable must
come from the immutable public module. The four extracted-binary checks are finite
synthetic observations, not authentication or an application of a grant.
"""
import argparse
import base64
import gzip
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import platform
import selectors
import shutil
import signal
import stat
import subprocess
import tarfile
import time
import zipfile
import zlib

SPEC = importlib.util.spec_from_file_location("api_bundle", Path(__file__).with_name("api_policy_cli_bundle.py"))
BASE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(BASE)
need, sha, read, encoded, unique = BASE.need, BASE.sha, BASE.read, BASE.encoded, BASE.unique
MODULE = "github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy"
VERSION = 'v0.0.0-20261020182808-7180a4a0f36a'
REVISION = '7180a4a0f36a1b0d89470100d6ccf6efdb5fd38d'
MODULE_SUM = 'h1:wiJUz5Om53/Ng69iQj/43qBKD7DSuqlqrdGLr6HgZDA='
MOD_SUM = 'h1:vK0nyI5FZr9LCKe42lsier/OajQhOgiZXExeW5Hlhzk='
GENERATION = '3c86dd9273168f1fe4e0712aa7cfdfc075a211d05b64ff2fbd0b8c9135315a3b'
PIN = BASE.PIN
COMMAND = "gooo-service-grant-evaluate"
MAX_STREAM = 128 << 10
MAX_SOURCE = 2 << 20
MAX_ARCHIVE = BASE.MAX_ARCHIVE
LICENSES = {
    "LICENSE": ("LICENSE", "099e1adf9145f6fe45357f004f503231a21b58244fdf105a34117d1a33b4bd48"),
    "NOTICE-AgentMesh.md": ("examples/agentmesh-service-policy/NOTICE.md", None),
    "LICENSE-AgentMesh-Apache-2.0": ("examples/agentmesh-service-policy/LICENSE-APACHE-2.0", None),
    "GENERATOR-LICENSE-Gooo.txt": ("tools/licenses/gooo-83928fa-MIT.txt", "5177457bc30b96eae6f786d0fbe3d1fda382edef61ecef25eb6f446a8b35b4f3"),
}


def load(raw):
    return json.loads(raw.decode("utf-8"), object_pairs_hook=unique,
                      parse_constant=lambda value: (_ for _ in ()).throw(ValueError("nonfinite JSON")))


def native_platform():
    value = {("Linux", "x86_64"): "linux/amd64", ("Darwin", "arm64"): "darwin/arm64"}.get(
        (platform.system(), platform.machine()))
    need(value is not None, "native Linux amd64 or Darwin arm64 required")
    return value


def build_identity(build, target):
    need(type(build) is dict, "BuildInfo object required")
    settings = unique((row["Key"], row["Value"]) for row in build["Settings"])
    need(build["GoVersion"] == "go1.27.3" and build["Path"] == MODULE + "/cmd/evaluate"
         and settings.get("CGO_ENABLED") == "0"
         and settings.get("GOOS", "") + "/" + settings.get("GOARCH", "") == target,
         "native package, Go version or CGO differs")
    main = build["Main"]
    need({key: main.get(key) for key in ("Path", "Version", "Sum")} ==
         {"Path": MODULE, "Version": VERSION, "Sum": MODULE_SUM}
         and main.get("Replace") is None and build.get("Deps", []) == [],
         "immutable public module or dependency graph differs")
    return {"go_version": build["GoVersion"], "package": build["Path"], "main": main,
            "dependencies": [], "platform": target, "cgo_enabled": False}


def dirhash(files):
    # Go x/mod/sumdb/dirhash.Hash1: sorted content-digest, two spaces, name, LF.
    rows = "".join(sha(raw) + "  " + name + "\n" for name, raw in sorted(files.items()))
    return "h1:" + base64.b64encode(hashlib.sha256(rows.encode()).digest()).decode()


def member_read(directory, relative):
    path = directory
    for part in relative.split("/")[:-1]:
        path = path / part
        need(stat.S_ISDIR(path.lstat().st_mode), "regular module directory required")
    return read(directory / relative, MAX_SOURCE)


def module_source(module_dir, module_zip, root):
    need(stat.S_ISDIR(module_dir.lstat().st_mode), "regular module directory required")
    prefix = MODULE + "@" + VERSION + "/"
    raw = read(module_zip, 4 << 20)
    files = {}
    with zipfile.ZipFile(io.BytesIO(raw)) as archive:
        rows = archive.infolist()
        need(len(rows) == 112 and sum(row.file_size for row in rows) <= 16 << 20,
             "complete public module roster or size differs")
        for row in rows:
            name = row.filename
            relative = name.removeprefix(prefix)
            need(name.startswith(prefix) and relative and name not in files
                 and "\\" not in name and "\n" not in name and "\r" not in name
                 and all(part not in ("", ".", "..") for part in relative.split("/"))
                 and not row.is_dir() and row.file_size <= MAX_SOURCE
                 and (row.external_attr >> 16) & 0o170000 in (0, stat.S_IFREG),
                 "unsafe, duplicate or nonregular module member")
            content = archive.read(row)  # Includes the ZIP CRC/size check.
            need(member_read(module_dir, relative) == content, "installed module differs from ZIP")
            files[name] = content
    need(dirhash(files) == MODULE_SUM, "public module h1 differs")
    relative_files = {name[len(prefix):]: raw for name, raw in files.items()}
    # Reject injected, unsealed source as well as missing files, without following links.
    actual = set()
    for current, dirs, names in os.walk(module_dir, followlinks=False):
        for name in dirs:
            need(not (Path(current) / name).is_symlink(), "module directory symlink refused")
        for name in names:
            path = Path(current) / name
            need(stat.S_ISREG(path.lstat().st_mode), "nonregular installed module file")
            actual.add(path.relative_to(module_dir).as_posix())
    need(actual == set(relative_files), "installed module file roster differs")
    need(dirhash({"go.mod": relative_files["go.mod"]}) == MOD_SUM, "go.mod h1 differs")
    generation_raw = relative_files["generation.json"]
    need(sha(generation_raw) == GENERATION, "pinned generation differs")
    generation = load(generation_raw)
    need(generation.get("schema") == "gooo/agentmesh-native-generation/v1"
         and generation["origin"] == "fresh_pinned_generation"
         and type(generation["assets"]) is dict and len(generation["assets"]) == 89
         and type(generation["public_sources"]) is dict and len(generation["public_sources"]) == 16,
         "generation scope differs")
    for name, digest in generation["assets"].items():
        need(type(digest) is str and digest == "sha256:" + sha(relative_files[name]),
             "generation member bytes differ")
    for name, digest in generation["public_sources"].items():
        need(type(digest) is str and digest == "sha256:" + sha(member_read(root, name)),
             "public generator source bytes differ")
    need(read(root / "examples/agentmesh-service-policy/generation.json", MAX_SOURCE) == generation_raw,
         "packaging checkout has another generation")
    return generation, relative_files


def controls():
    facts = {"before": "*", "after": "mcp://*", "required_type": "mcp", "required_name": "weather"}
    rows = []
    for name, change, status, reason, field, flags, rule in (
        ("eligible-facts.json", {}, "ELIGIBLE", "SUBSET_AND_REQUIRED_REQUEST_PRESERVED", "", (True, True, True), "OLD_GLOBAL"),
        ("required-removed-facts.json", {"after": "http://*"}, "REFUSED", "REQUIRED_REQUEST_REMOVED", "", (True, True, False), "OLD_GLOBAL"),
        ("domain-invalid-facts.json", {"required_name": "weather/forecast"}, "INVALID_INPUT", "INVALID_CANONICAL_INPUT", "required_name", (False, False, False), ""),
    ):
        value = {**facts, **change}
        raw = (json.dumps([value], separators=(",", ":")) + "\n").encode()
        decision = dict(zip(("status", "reason", "invalid_field", "required_observed", "required_before_match", "required_after_match", "subset_rule_id"),
                            (status, reason, field, *flags, rule)))
        rows.append((name, raw, value, decision))
    rows.append(("duplicate-key.json", b'[{"before":"*","before":"*","after":"mcp://*","required_type":"mcp","required_name":"weather"}]\n', None, None))
    return rows


def checked_result(stdout, stderr, code, facts, decision, generation):
    if facts is None:
        need(code == 1 and stdout == b"" and stderr == b"case0: expected exact JSON Text service facts\n",
             "structural refusal differs")
        return
    need(code == 0 and stderr == b"", "valid input failed")
    rows = load(stdout)
    need(type(rows) is list and len(rows) == 1 and type(rows[0]) is dict
         and set(rows[0]) == {"facts", "decision", "bindings"}, "exact result record required")
    row = rows[0]
    need(row["facts"] == facts and type(row["facts"]) is dict and set(row["facts"]) == set(facts)
         and all(type(value) is str for value in row["facts"].values()), "exact four Text facts required")
    need(type(row["decision"]) is dict and set(row["decision"]) == set(decision)
         and all(type(row["decision"][key]) is type(value) and row["decision"][key] == value
                 for key, value in decision.items()), "exact seven typed decision fields required")
    need(row["bindings"] == {"compiler_source_sha": PIN, "generation_sha256": "sha256:" + GENERATION,
         "generation_origin": "fresh_pinned_generation", "assets": generation["assets"]},
         "complete compiler/generation/89-asset bindings differ")


def run_owned(argv, input_raw, cwd, env, out, label, deadline):
    """Bound both pipes while joining the sole owned process group on every path."""
    start = time.monotonic()
    process = subprocess.Popen([str(arg) for arg in argv], stdin=subprocess.PIPE,
                               stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                               cwd=cwd, env=env, start_new_session=True)
    buffers = {"stdout": bytearray(), "stderr": bytearray()}
    failure = None
    try:
        process.stdin.write(input_raw)
        process.stdin.close()
        with selectors.DefaultSelector() as selector:
            for name in buffers:
                stream = getattr(process, name)
                os.set_blocking(stream.fileno(), False)
                selector.register(stream, selectors.EVENT_READ, name)
            while selector.get_map():
                need(time.monotonic() < min(deadline, start + 10), "owned command timed out")
                for key, _ in selector.select(0.05):
                    raw = os.read(key.fd, 65536)
                    if raw:
                        room = MAX_STREAM - len(buffers[key.data])
                        buffers[key.data].extend(raw[:room])
                        need(len(raw) <= room, "owned command stream exceeds bound")
                    else:
                        selector.unregister(key.fileobj)
        process.wait(timeout=max(0.01, min(deadline, start + 10) - time.monotonic()))
    except BaseException as error:
        failure = error
    finally:
        for sig in (signal.SIGTERM, signal.SIGKILL):
            try:
                os.killpg(process.pid, sig)
            except ProcessLookupError:
                break
            try:
                process.wait(timeout=0.5)
            except subprocess.TimeoutExpired:
                pass
        try:
            process.wait(timeout=1)
            try:
                os.killpg(process.pid, 0)
            except ProcessLookupError:
                pass
            else:
                raise ValueError("owned process group remains")
        except BaseException as error:
            if failure is None:
                failure = error
            elif hasattr(failure, "add_note"):
                failure.add_note(str(error))
        def preserve(action):
            nonlocal failure
            try:
                action()
            except BaseException as diagnostic_error:
                if failure is None:
                    failure = diagnostic_error
                elif hasattr(failure, "add_note"):
                    failure.add_note("command diagnostic/close: " + str(diagnostic_error))
        for stream in (process.stdin, process.stdout, process.stderr):
            preserve(stream.close)
        for name, raw in buffers.items():
            preserve(lambda name=name, raw=raw: (out / (label + "." + name)).write_bytes(raw))
        record = {"argv": [Path(str(argv[0])).name, *map(str, argv[1:])], "exit_code": process.returncode,
                  "elapsed_seconds": time.monotonic() - start,
                  "stdout_bytes": len(buffers["stdout"]), "stderr_bytes": len(buffers["stderr"]),
                  "stdout_sha256": sha(buffers["stdout"]), "stderr_sha256": sha(buffers["stderr"]),
                  "waited": process.returncode is not None, "streams_eof": failure is None,
                  "process_group_absent": failure is None}
        preserve(lambda: (out / (label + ".json")).write_bytes(encoded(record)))
    if failure is not None:
        raise failure
    return bytes(buffers["stdout"]), bytes(buffers["stderr"]), process.returncode, record


def archive_files(command, build, generation, source, root, go, packaging_sha):
    target = build["platform"]
    text = ("Gooo service-grant assessment — " + target + "\n"
            "No Go, Gooo compiler or Python installation is needed.\n"
            "From this extracted directory, run:\n"
            "./gooo-service-grant-evaluate -mode grant-update < eligible-facts.json\n"
            "Repeat with required-removed-facts.json or domain-invalid-facts.json.\n"
            "ELIGIBLE, REFUSED and INVALID_INPUT are decisions (exit 0).\n"
            "duplicate-key.json is a structural error: exit 1, empty stdout.\n"
            "These are supplied unsigned facts; this command applies no grant or API effect.\n"
            "Runtime source: " + MODULE + "@" + VERSION + "\n"
            "LICENSE is the ports BSD-3-Clause license; AgentMesh and Go notices are included.\n"
            "GENERATOR-LICENSE-Gooo.txt concerns the generation tool; no engine binary is included.\n")
    files = {COMMAND: (command, 0o755), "START-HERE.txt": (text.encode(), 0o644)}
    for name, raw, _, _ in controls():
        files[name] = (raw, 0o644)
    for name, (relative, digest) in LICENSES.items():
        raw = read(root / relative, MAX_SOURCE)
        need(digest is None or sha(raw) == digest, "pinned license differs")
        if name == "LICENSE":
            need(raw == source["LICENSE"], "module/root BSD license differs")
        elif name == "NOTICE-AgentMesh.md":
            need(raw == source["NOTICE.md"], "AgentMesh attribution differs")
        elif name == "LICENSE-AgentMesh-Apache-2.0":
            need(raw == source["LICENSE-APACHE-2.0"] == source["upstream/LICENSE"], "upstream license differs")
        files[name] = (raw, 0o644)
    files["LICENSE-Go"] = (read(go.parent.parent / "LICENSE", MAX_SOURCE), 0o644)
    manifest = {"schema": "gooo/service-grant-cli-bundle/v1", "platform": target,
                "source_commit": REVISION, "packaging_checkout_commit": packaging_sha,
                "module": {"path": MODULE, "version": VERSION, "sum": MODULE_SUM, "go_mod_sum": MOD_SUM},
                "build_info": build, "compiler_source_sha": PIN, "generation_sha256": "sha256:" + GENERATION,
                "generation_origin": generation["origin"], "assets": generation["assets"],
                "finite_checks": {"extracted_binary_controls": 4, "unsigned_supplied_facts": True,
                                  "grant_applied": False, "external_adoption": False,
                                  "producer_authenticated": False, "native_acceptance": False},
                "members": {name: {"bytes": len(raw), "sha256": sha(raw), "mode": mode}
                            for name, (raw, mode) in sorted(files.items())}}
    files["BUNDLE.json"] = (encoded(manifest), 0o644)
    need(len(files) == 12, "exact twelve archive members required")
    return files


def verify_archive(path, files):
    raw = read(path, MAX_ARCHIVE)
    decoder = zlib.decompressobj(31)
    expanded = decoder.decompress(raw, MAX_ARCHIVE + 1)
    need(len(expanded) <= MAX_ARCHIVE and decoder.eof and not decoder.unused_data
         and not decoder.unconsumed_tail, "single bounded gzip stream required")
    return BASE.verify_archive(path, files)


def write_archive(path, files):
    with path.open("xb") as target:
        with gzip.GzipFile(filename="", fileobj=target, mode="wb", mtime=0) as compressed:
            with tarfile.open(fileobj=compressed, mode="w", format=tarfile.USTAR_FORMAT) as archive:
                for name, (raw, mode) in sorted(files.items()):
                    member = tarfile.TarInfo(name)
                    member.size, member.mode = len(raw), mode
                    archive.addfile(member, io.BytesIO(raw))
    return verify_archive(path, files)


def produce(binary, module_dir, module_zip, go, root, out, packaging_sha):
    need(len(packaging_sha) == 40 and all(c in "0123456789abcdef" for c in packaging_sha), "packaging SHA required")
    need(out.is_dir() and not out.is_symlink() and not any(out.iterdir()), "fresh owned output directory required")
    work = out / "work"
    created = []
    failure = None
    result = None
    try:
        work.mkdir(mode=0o700)
        command = read(binary, BASE.MAX_BINARY)
        need(binary.stat().st_mode & 0o111, "native executable required")
        generation, source = module_source(module_dir, module_zip, root)
        target = native_platform()
        env = {"PATH": "", "HOME": str(work), "TMPDIR": str(work), "LANG": "C", "LC_ALL": "C",
               "GOTOOLCHAIN": "local", "GOENV": "off", "GOWORK": "off", "GOPROXY": "off",
               "GOSUMDB": "off", "GOTELEMETRY": "off", "GOMAXPROCS": "2"}
        deadline = time.monotonic() + 60
        stdout, stderr, code, _ = run_owned([go, "version", "-m", "-json", binary], b"", work, env, out, "build-info", deadline)
        need(code == 0 and not stderr, "cannot read native BuildInfo")
        build = build_identity(load(stdout), target)
        files = archive_files(command, build, generation, source, root, go, packaging_sha)
        name = "gooo-service-grant-evaluate-" + target.replace("/", "-") + ".tar.gz"
        candidate = work / name
        write_archive(candidate, files)
        extracted = work / COMMAND
        with tarfile.open(candidate, "r:gz") as archive:
            extracted.write_bytes(archive.extractfile(COMMAND).read(BASE.MAX_BINARY + 1))
        need(read(extracted, BASE.MAX_BINARY) == command, "extracted command differs")
        extracted.chmod(0o700)
        observations = []
        for index, (case, raw, facts, decision) in enumerate(controls()):
            label = "control-" + str(index + 1)
            (out / (label + ".input")).write_bytes(raw)
            stdout, stderr, code, record = run_owned([extracted, "-mode", "grant-update"], raw, work, env, out, label, deadline)
            checked_result(stdout, stderr, code, facts, decision, generation)
            observations.append({"input": case, **record})
        need(read(extracted, BASE.MAX_BINARY) == command == read(binary, BASE.MAX_BINARY), "tested binary changed")
        need(module_source(module_dir, module_zip, root)[0] == generation, "module changed during checks")
        archive_raw = read(candidate, MAX_ARCHIVE)
        shutil.rmtree(work)
        need(not work.exists(), "owned work cleanup incomplete")
        final = out / name
        with final.open("xb") as stream:
            created.append(final)
            stream.write(archive_raw)
        checked = verify_archive(final, files)
        checksums = out / (name + ".sha256")
        checksum_raw = (checked["sha256"] + "  " + name + "\n").encode()
        with checksums.open("xb") as stream:
            created.append(checksums)
            stream.write(checksum_raw)
        need(read(checksums, 4096) == checksum_raw, "checksum readback differs")
        result = {"schema": "gooo/service-grant-cli-verification/v1", "status": "NATIVE_BUNDLE_FOUR_CONTROLS_PASS",
                  "platform": target, "source_commit": REVISION, "packaging_checkout_commit": packaging_sha,
                  "module_sum": MODULE_SUM, "go_mod_sum": MOD_SUM, "binary_sha256": sha(command),
                  "build_info": build, "go_executable_sha256": sha(read(go, 64 << 20)), "archive": {key: value for key, value in checked.items() if key != "path"},
                  "archive_name": name, "checksums_name": checksums.name, "members": sorted(files),
                  "controls": observations, "runtime_empty_path": True, "runtime_goroot_absent": True,
                  "owned_work_removed": True, "all_process_groups_closed": True,
                  "engine_built": False, "external_adoption": False, "grant_applied": False,
                  "producer_authenticated": False, "native_acceptance": False}
        (out / "verification.json").write_bytes(encoded(result))
    except BaseException as error:
        failure = error
        for path in reversed(created):
            try:
                path.unlink()
            except OSError as cleanup_error:
                if hasattr(error, "add_note"):
                    error.add_note(str(cleanup_error))
    finally:
        if work.exists():
            try:
                shutil.rmtree(work)
            except BaseException as error:
                if failure is None:
                    failure = error
                elif hasattr(failure, "add_note"):
                    failure.add_note("owned cleanup: " + str(error))
        if failure is not None:
            try:
                (out / "first-failure.json").write_bytes(encoded({"status": "REFUSED", "error": str(failure),
                    "type": type(failure).__name__, "owned_work_removed": not work.exists()}))
            except BaseException as diagnostic_error:
                if hasattr(failure, "add_note"):
                    failure.add_note("failure diagnostic: " + str(diagnostic_error))
    if failure is not None:
        raise failure
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("binary", "module-dir", "module-zip", "go", "root", "out"):
        parser.add_argument("--" + name, type=Path, required=True)
    parser.add_argument("--packaging-sha", required=True)
    args = parser.parse_args()
    def interrupted(number, _frame):
        raise RuntimeError("interrupted by signal " + str(number))
    signal.signal(signal.SIGTERM, interrupted)
    signal.signal(signal.SIGINT, interrupted)
    args.out.mkdir(mode=0o700, parents=True, exist_ok=True)
    produce(args.binary.absolute(), args.module_dir.absolute(), args.module_zip.absolute(),
            args.go.resolve(strict=True), args.root.absolute(), args.out.absolute(), args.packaging_sha)


if __name__ == "__main__":
    main()
