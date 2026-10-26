"""Package a verified saved-catalog CLI without rebuilding Go or Gooo.

This is a distribution check over retained, source-bound producer evidence. The
extracted executable is exercised on its native host; it does not create another
independent semantic oracle or authenticate the original producer.
"""
import argparse
import gzip
import importlib.util
import io
import json
import os
from pathlib import Path
import platform
import re
import selectors
import signal
import stat
import struct
import subprocess
import tarfile
import tempfile
import time
import zlib

SPEC = importlib.util.spec_from_file_location("catalog_archive_base", Path(__file__).with_name("api_policy_cli_bundle.py"))
BASE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(BASE)
need, sha, read, encoded, unique = BASE.need, BASE.sha, BASE.read, BASE.encoded, BASE.unique
COMMAND = "gooo-recorded-catalog"
MODULE = "github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison"
PROFILE = "examples/recorded-catalog-comparison/"
REGRESSION_ROOT = "examples/mcp-upgrade-regressions/"
REGRESSION_ARCHIVE = "mcp-upgrade-regressions/"
REGRESSION_CASES = ("output-schema-string-to-number", "same-definition-version-context-only", "missing-after-incomplete-page")
REGRESSION_STATES = ("OBSERVED_DEFINITION_CHANGED", "OBSERVED_DEFINITION_SAME", "UNASSESSED")
REGRESSION_FILES = ("before.request.json", "before.response.json", "after.request.json", "after.response.json",
                    "snapshot-input.json", "expected.json", "observed-decision.json", "context.json")
SOURCE_DIRS = ("examples/explicit-observation-availability/", "examples/tool-definition-snapshot/", REGRESSION_ROOT, PROFILE)
SOURCE_EXTRAS = {
    "upstream.json", "explicit-availability.workspace.json", "explicit-observation.workspace.json",
    "definition-snapshot.workspace.json", "ports/tool-origin/definition_snapshot.gooo",
    "ports/tool-origin/alias_origin.gooo", "ports/tool-origin/definition_binding.gooo",
    "ports/tool-origin/explicit_availability.gooo", "ports/tool-origin/explicit_observation.gooo",
    "ports/tool-origin/recorded_catalog.gooo", "examples/service-grant-many/verify.py",
    "examples/tool-definition-binding/evidence.py",
    REGRESSION_ROOT + "README.md", REGRESSION_ROOT + "LICENSE",
}
# Bind the complete runtime generation roster, not just an equal-size subset.
GENERATION_MANIFESTS = {PROFILE + name for name in (
    "generation.json", "definition-generation.json", "observation-generation.json",
    "availability-generation.json", "availability-observation-generation.json",
    "snapshot-generation.json",
)}


SNAPSHOT_ROOT = "examples/tool-definition-snapshot/"
SNAPSHOT_GENERATOR_SOURCES = {
    SNAPSHOT_ROOT + "generate.py", SNAPSHOT_ROOT + "evidence.py",
    PROFILE + "generate.py", "examples/service-grant-many/verify.py",
}
SNAPSHOT_GENERATED_ASSETS = {PROFILE + "internal/snapshotkernel/" + name for name in
                             ("projection_generated.go", "types_generated.go", "bridge_generated.go")} | {
    PROFILE + "snapshot-kernel-cases.json",
}
SNAPSHOT_REFERENCE_SOURCES = {
    "reference/main.go", "reference/go.mod", "reference/make_inputs.py",
    "reference-inputs.json", "case-labels.json", "input-boundaries.json", "abi.json",
}


def check_generation_roster(generations):
    need(set(generations) == GENERATION_MANIFESTS, "all exact runtime generation manifests required")


def check_snapshot_materials(generation, sources):
    """Bind the new generator's actual material claims to retained source bytes.

    This does not replace native receipt verification or authenticate a producer.
    """
    for group, names, prefix in (
        ("source", {"ports/tool-origin/definition_snapshot.gooo"}, ""),
        ("generator_source", SNAPSHOT_GENERATOR_SOURCES, ""),
        ("reference_source", SNAPSHOT_REFERENCE_SOURCES, SNAPSHOT_ROOT),
        ("generated_assets", SNAPSHOT_GENERATED_ASSETS, ""),
    ):
        values = generation.get(group)
        need(type(values) is dict and set(values) == names, "snapshot material roster differs")
        for name, digest in values.items():
            path = prefix + name
            need(path in sources and digest == "sha256:" + sources[path]["sha256"],
                 "snapshot material differs from bound source")
    for key, path in (("workspace_sha256", "definition-snapshot.workspace.json"),
                      ("cases_sha256", SNAPSHOT_ROOT + "kernel.cases.json")):
        need(path in sources and generation.get(key) == "sha256:" + sources[path]["sha256"],
             "snapshot material differs from bound source")


LABELS = ("supplied-unknown", "unobserved-unknown", "unobserved-other-text", "asymmetric-unobserved",
          "known-page-change", "unobserved-policy", "optional-timestamps", "backend-unobserved-same",
          "backend-unobserved-different", "backend-unobserved-asymmetric", "backend-unobserved-known-host",
          "backend-unobserved-known-connection", "backend-unobserved-known-name")
MAX_STREAM = 4 << 20
MAX_SOURCE = 4 << 20
LICENSES = {"LICENSE": "LICENSE", "NOTICE.md": PROFILE + "NOTICE.md",
            "GENERATOR-LICENSE-Gooo.txt": "tools/licenses/gooo-83928fa-MIT.txt"}


def load(raw):
    return json.loads(raw.decode("utf-8"), object_pairs_hook=unique,
                      parse_constant=lambda _: (_ for _ in ()).throw(ValueError("nonfinite JSON")))


def exact(left, right):
    need(type(left) is type(right), "JSON type differs")
    if isinstance(left, dict):
        need(left.keys() == right.keys(), "JSON fields differ")
        for key in left:
            exact(left[key], right[key])
    elif isinstance(left, list):
        need(len(left) == len(right), "JSON length differs")
        for a, b in zip(left, right):
            exact(a, b)
    else:
        need(left == right, "JSON value differs")


def member(root, relative, cap=MAX_SOURCE):
    need(isinstance(relative, str) and relative and not relative.startswith("/")
         and "\\" not in relative and all(x not in ("", ".", "..") for x in relative.split("/")),
         "safe relative member required")
    current = root
    need(stat.S_ISDIR(current.lstat().st_mode), "regular root directory required")
    for part in relative.split("/")[:-1]:
        current /= part
        need(stat.S_ISDIR(current.lstat().st_mode), "directory symlink refused")
    return read(root / relative, cap)


def identity(raw):
    return {"bytes": len(raw), "sha256": sha(raw)}


def matches(raw, reference):
    need(type(reference) is dict and type(reference.get("bytes")) is int
         and identity(raw) == {key: reference.get(key) for key in ("bytes", "sha256")},
         "retained evidence identity differs")


def native_platform():
    target = {("Linux", "x86_64"): "linux/amd64", ("Darwin", "arm64"): "darwin/arm64"}.get(
        (platform.system(), platform.machine()))
    need(target is not None, "native Linux amd64 or Darwin arm64 required")
    return target


def binary_platform(raw):
    if raw[:4] == b"\xcf\xfa\xed\xfe":
        need(len(raw) >= 32 and struct.unpack("<I", raw[4:8])[0] == 0x0100000C
             and struct.unpack("<I", raw[12:16])[0] == 2, "Mach-O architecture/type differs")
        return "darwin/arm64"
    need(len(raw) >= 64 and raw[:6] == b"\x7fELF\x02\x01", "little-endian native executable required")
    need(struct.unpack("<HH", raw[16:20]) == (2, 62), "ELF architecture/type differs")
    offset = struct.unpack("<Q", raw[32:40])[0]
    size, count = struct.unpack("<HH", raw[54:58])
    need(size == 56 and 0 < count <= 64 and offset + size * count <= len(raw), "ELF headers differ")
    need(all(struct.unpack("<I", raw[offset+i*size:offset+i*size+4])[0] not in (2, 3)
             for i in range(count)), "dynamic ELF dependencies refused")
    return "linux/amd64"


def embedded_info(raw):
    """Read Go's inline BuildInfo; do not synthesize missing VCS/module data."""
    magic = b"\xff Go buildinf:"
    offset = raw.find(magic)
    need(offset >= 0 and raw.find(magic, offset + 1) == -1
         and raw[offset+14:offset+16] == bytes([8, 2]), "inline Go BuildInfo required")
    def string_at(index):
        value = shift = 0
        for _ in range(10):
            need(index < len(raw), "truncated BuildInfo length")
            byte = raw[index]; index += 1
            value |= (byte & 127) << shift
            if byte < 128:
                need(value <= MAX_SOURCE and index + value <= len(raw), "BuildInfo string exceeds bound")
                return raw[index:index+value], index + value
            shift += 7
        raise ValueError("invalid BuildInfo length")
    version, index = string_at(offset + 32)
    data, _ = string_at(index)
    need(data[:16].hex() == "3077af0c9274080241e1c107e6d618e6"
         and data[-16:].hex() == "f932433186182072008242104116d8f2", "BuildInfo framing differs")
    result = {"GoVersion": version.decode(), "Path": None, "Main": None, "Settings": []}
    for line in data[16:-16].decode().splitlines():
        fields = line.split("\t")
        if fields[0] == "path":
            need(len(fields) == 2 and result["Path"] is None, "duplicate/invalid build path")
            result["Path"] = fields[1]
        elif fields[0] == "mod":
            need(len(fields) in (3, 4) and result["Main"] is None, "duplicate/invalid main module")
            result["Main"] = {"Path": fields[1], "Version": fields[2]}
            if len(fields) == 4 and fields[3]:
                result["Main"]["Sum"] = fields[3]
        elif fields[0] == "build":
            need(len(fields) == 2 and "=" in fields[1], "invalid build setting")
            key, value = fields[1].split("=", 1)
            result["Settings"].append({"Key": key, "Value": value})
        else:
            raise ValueError("dependency, replacement or unsupported BuildInfo entry")
    return result


def build_identity(build, target, go_version):
    need(type(build) is dict and set(build) in ({"GoVersion", "Path", "Main", "Settings"},
                                               {"GoVersion", "Path", "Main", "Settings", "Deps"}),
         "BuildInfo fields differ")
    need(build.get("Deps", []) == [] and build["GoVersion"] == go_version
         and build["Path"] == MODULE + "/cmd/compare"
         and build["Main"] == {"Path": MODULE, "Version": "(devel)"}, "runtime module identity differs")
    settings = unique((x["Key"], x["Value"]) for x in build["Settings"])
    os_name, arch = target.split("/")
    expected = {"-buildmode": "exe", "-compiler": "gc", "-trimpath": "true", "CGO_ENABLED": "0",
                "GOOS": os_name, "GOARCH": arch, "GOARM64" if arch == "arm64" else "GOAMD64": "v8.0" if arch == "arm64" else "v1"}
    exact(settings, expected)
    return build


def run_owned(argv, input_raw, cwd, env, label, records, seconds=15, cap=MAX_STREAM, record_dir=None):
    """Bound stdin and both pipes concurrently, and join the owned process group."""
    need(len(input_raw) <= 1 << 20, "command input exceeds bound")
    start = time.monotonic()
    process = subprocess.Popen([str(x) for x in argv], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                               stderr=subprocess.PIPE, cwd=cwd, env=env, start_new_session=True)
    buffers = {"stdout": bytearray(), "stderr": bytearray()}
    sent = 0; failure = None
    try:
        with selectors.DefaultSelector() as selector:
            for name in buffers:
                stream = getattr(process, name); os.set_blocking(stream.fileno(), False)
                selector.register(stream, selectors.EVENT_READ, name)
            if input_raw:
                os.set_blocking(process.stdin.fileno(), False)
                selector.register(process.stdin, selectors.EVENT_WRITE, "stdin")
            else:
                process.stdin.close()
            while selector.get_map():
                need(time.monotonic() - start < seconds, "owned command timed out")
                for key, _ in selector.select(0.05):
                    if key.data == "stdin":
                        try:
                            sent += os.write(key.fd, input_raw[sent:sent + 65536])
                        except BrokenPipeError:
                            sent = len(input_raw)
                        if sent == len(input_raw):
                            selector.unregister(key.fileobj); key.fileobj.close()
                    else:
                        raw = os.read(key.fd, 65536)
                        if raw:
                            need(len(buffers[key.data]) + len(raw) <= cap, "owned command output exceeds bound")
                            buffers[key.data].extend(raw)
                        else:
                            selector.unregister(key.fileobj)
            process.wait(timeout=max(0.01, seconds - (time.monotonic() - start)))
    except BaseException as error:
        failure = error
    finally:
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        process.wait(timeout=5)
        for stream in (process.stdin, process.stdout, process.stderr):
            stream.close()
        group_gone = False
        for _ in range(100):
            try:
                os.killpg(process.pid, 0)
            except ProcessLookupError:
                group_gone = True; break
            time.sleep(0.01)
        record = {"label": label, "exit_code": process.returncode,
                  "stdout": identity(bytes(buffers["stdout"])), "stderr": identity(bytes(buffers["stderr"])),
                  "elapsed_seconds": time.monotonic() - start, "process_group_closed": group_gone,
                  "failure": type(failure).__name__ if failure else None}
        records.append(record)
        if record_dir is not None:
            need(re.fullmatch(r"[a-z0-9-]+", label), "safe control label required")
            directory = record_dir / label; directory.mkdir(parents=True, mode=0o700)
            for name, raw in buffers.items():
                (directory / name).write_bytes(bytes(raw))
            (directory / "process.json").write_bytes(encoded(record))
    need(group_gone, "owned process group remains")
    if failure:
        raise failure
    return bytes(buffers["stdout"]), bytes(buffers["stderr"]), process.returncode


def source_roster(tree):
    return SOURCE_EXTRAS | {name for name in tree if name.startswith(SOURCE_DIRS)
                           and Path(name).suffix in (".py", ".go", ".mod", ".json")
                           and "__pycache__" not in name.split("/")}


def check_sources(root, evidence, revision):
    need(re.fullmatch(r"[0-9a-f]{40}", revision), "full source revision required")
    env = {"PATH": os.environ.get("PATH", "/usr/bin:/bin"), "HOME": str(root), "LC_ALL": "C",
           "GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": os.devnull, "GIT_OPTIONAL_LOCKS": "0"}
    records = []
    def git(args):
        out, err, code = run_owned(["git", "-C", root, *args], b"", root, env, "source-git", records)
        need(code == 0 and not err, "immutable source Git read failed")
        return out
    raw_tree = git(["ls-tree", "-rz", revision])
    tree = {}
    for row in raw_tree.split(b"\0"):
        if not row:
            continue
        header, name = row.split(b"\t", 1); mode, kind, oid = header.decode().split()
        need(name.decode() not in tree, "duplicate Git source path")
        tree[name.decode()] = (mode, kind, oid)
    before = load(member(evidence, "source-before.json")); after = load(member(evidence, "source-after.json"))
    exact(before, after)
    need(set(before) == source_roster(tree), "complete source roster differs")
    public = {}
    for name in sorted(before):
        need(tree.get(name, ())[:2] == ("100644", "blob"), "regular tracked source required")
        raw = member(root, name); matches(raw, before[name])
        git_blob = git(["cat-file", "blob", tree[name][2]])
        need(git_blob == raw, "source bytes differ from immutable Git")
        public[name] = identity(raw)
    # Licenses are not executable producer inputs but must also match the revision.
    licenses = {}
    for label, name in LICENSES.items():
        need(tree.get(name, ())[:2] == ("100644", "blob"), "tracked license required")
        raw = member(root, name); need(git(["cat-file", "blob", tree[name][2]]) == raw, "license differs from revision")
        licenses[label] = raw
    return public, licenses


def check_snapshot_regressions(root, sources, license_raw):
    """Bind the frozen packet; historical observed decisions are regression baselines."""
    payload_names = {case + "/" + name for case in REGRESSION_CASES for name in REGRESSION_FILES} | {"LICENSE"}
    all_names = payload_names | {"README.md", "packet.json"}
    need({name for name in sources if name.startswith(REGRESSION_ROOT)} ==
         {REGRESSION_ROOT + name for name in all_names}, "exact snapshot packet source roster required")
    files = {}
    for name in sorted(all_names):
        raw = member(root, REGRESSION_ROOT + name)
        matches(raw, sources[REGRESSION_ROOT + name])
        files[name] = raw
    packet = load(files["packet.json"])
    need(packet.get("schema") == "gooo/synthetic-upgrade-regression-packet/v1" and
         packet.get("status") == "SYNTHETIC_FIXTURE_PACKET" and packet.get("license") == "BSD-3-Clause",
         "synthetic snapshot packet required")
    need(re.fullmatch(r"[0-9a-f]{40}", packet["source_commit"]) is not None, "original packet revision required")
    listed = unique((item["path"], item) for item in packet["files"])
    need(set(listed) == payload_names, "snapshot payload roster differs")
    for name in payload_names:
        matches(files[name], listed[name])
    need(files["LICENSE"] == license_raw, "snapshot packet license differs")
    need([row["case"] for row in packet["cases"]] == list(REGRESSION_CASES), "three snapshot cases required")
    rows = []
    for case, state, declaration in zip(REGRESSION_CASES, REGRESSION_STATES, packet["cases"]):
        need(declaration["input"] == case + "/snapshot-input.json" and
             declaration["expectations"] == case + "/expected.json" and
             declaration["observed_decision"] == case + "/observed-decision.json" and
             declaration["state"] == state and declaration["evidence_kind"] == "SYNTHETIC_FIXTURE",
             "snapshot case declaration differs")
        raw = files[declaration["input"]]; value = load(raw)
        need(set(value) == {"target_name", "before", "after"}, "snapshot input must exclude explanatory context")
        expected = load(files[declaration["expectations"]])
        decision = load(files[declaration["observed_decision"]])
        context = load(files[case + "/context.json"])
        need(expected["evidence_kind"] == "SYNTHETIC_FIXTURE" and
             expected["expectation_kind"] == "MANUALLY_AUTHORED_SEMANTIC_ANCHORS_NOT_EXECUTION" and
             expected["execution_performed"] is False and context["not_passed_to_comparator"] is True and
             context["captured_from_server"] is False and context["upstream_reproduction_executed"] is False,
             "snapshot fixture scope differs")
        shape = expected["decision_shape"]
        need(shape["field_count"] == len(shape["exact_fields"]) == len(set(shape["exact_fields"])) == 32 and
             set(decision) == set(shape["exact_fields"]) and len(expected["anchors"]) == 21,
             "complete snapshot decision and authored anchors required")
        for key, anchor in expected["anchors"].items():
            exact(decision[key], anchor)
        need(decision["state"] == state and decision["context_id"] == "gooo/definition-snapshot/v1/sha256:" + sha(raw),
             "snapshot baseline binding differs")
        for side in ("before", "after"):
            need(set(value[side]) == {"pages"} and len(value[side]["pages"]) == 1, "one retained snapshot page required")
            page = value[side]["pages"][0]
            need(set(page) == {"request_json", "response_json"}, "snapshot raw page shape differs")
            for kind in ("request", "response"):
                need(page[kind + "_json"].encode("utf-8") == files[case + "/" + side + "." + kind + ".json"],
                     "snapshot frame bytes differ")
        rows.append({"case": case, "input": raw, "expected": decision, "anchors": expected["anchors"]})
    return {"files": files, "rows": rows, "original_source_commit": packet["source_commit"]}


def check_snapshot_report(raw_report, raw_input, row, generation):
    report = load(raw_report)
    need(report["schema"] == "gooo/definition-snapshot-report/v1", "snapshot report schema differs")
    exact(report["decision"], row["expected"])
    for key, anchor in row["anchors"].items():
        exact(report["decision"][key], anchor)
    exact(report["bindings"], generation)
    projection = report["projection"]
    need(projection["input_json"].encode("utf-8") == raw_input and
         projection["input_sha256"] == "sha256:" + sha(raw_input), "snapshot original input binding differs")
    need(projection["context_id"] == row["expected"]["context_id"] and
         projection["target_name"] == row["expected"]["target_name"], "snapshot selection binding differs")
    value = load(raw_input)
    for side in ("before", "after"):
        pages = projection[side]["pages"]
        need(len(pages) == 1 and pages[0]["page_index"] == 0, "snapshot projected page differs")
        for kind in ("request", "response"):
            need(pages[0][kind + "_sha256"] == "sha256:" + sha(value[side]["pages"][0][kind + "_json"].encode("utf-8")),
                 "snapshot original frame binding differs")


def check_evidence(root, evidence, synthetic_export, revision):
    summary = load(member(evidence, "summary.json"))
    need(summary.get("schema") == "gooo/explicit-availability-consumer-verification/v1"
         and summary.get("status") == "PASS" and summary.get("first_failure") is None,
         "successful consumer evidence required")
    for key in ("source_unchanged", "cli_report_equal", "finite_native_verified", "all_direct_wait_EOF_PG_ESRCH"):
        need(summary.get(key) is True, "consumer evidence is incomplete")
    need(summary.get("native_acceptance") is False and summary.get("external_adoption") is False
         and summary.get("live_effect_performed") is False and type(summary.get("model_calls")) is int
         and summary["model_calls"] == 0, "producer scope differs")
    profiles = summary.get("profiles", [])
    public = [p for p in profiles if p.get("kind") == "synthetic"]
    need(len(public) == 1 and type(public[0].get("records")) is int and public[0]["records"] == 13
         and type(public[0].get("diagnostics_independently_checked")) is int
         and public[0]["diagnostics_independently_checked"] == 442, "public consumer profile differs")
    processes = unique((p["label"], p) for p in summary["processes"])
    need(processes and all(p.get("exit_code") == 0 and type(p.get("exit_code")) is int
         and p.get("first_failure") is None and all(p.get(k) is True for k in ("waited", "streams_EOF", "PG_ESRCH"))
         for p in processes.values()), "consumer processes did not close successfully")
    binary = member(evidence, "compare", BASE.MAX_BINARY); matches(binary, summary["binary"])
    raw_info = member(evidence, "CLI-build-info/stdout"); matches(raw_info, processes["CLI-build-info"]["stdout"])
    need(member(evidence, "CLI-build-info/stderr") == b"", "BuildInfo command diagnostic differs")
    target = native_platform(); need(binary_platform(binary) == target, "native target differs")
    go_mod = member(root, PROFILE + "go.mod").decode()
    match = re.search(r"(?m)^go (\d+\.\d+\.\d+)$", go_mod); need(match is not None, "exact Go version required")
    go_version = "go" + match.group(1)
    pins = load(member(root, "upstream.json"))
    need(go_version == pins["go_version"] and re.fullmatch(r"[0-9a-f]{40}", pins["compiler_revision"]),
         "pinned producer toolchain differs")
    build = load(raw_info); embedded = embedded_info(binary)
    if build.get("Deps") == []:
        build = {k: v for k, v in build.items() if k != "Deps"}
    exact(build, embedded); build_identity(build, target, go_version)
    raw_export = read(synthetic_export, MAX_STREAM); matches(raw_export, summary["exports"][0])
    rows = load(raw_export)
    need(type(rows) is list and [r.get("label") for r in rows] == list(LABELS), "exact public thirteen records required")
    expected_keys = {"label", "raw_input_json", "availability_json", "before_policy_revision", "after_policy_revision",
                     "report", "expected_state", "expected_collection_state"}
    for row in rows:
        need(set(row) == expected_keys, "public export shape differs")
        raw, sidecar = row["raw_input_json"].encode(), row["availability_json"].encode()
        need(len(raw) + len(sidecar) <= 1 << 20, "public record exceeds input bound")
        projection = row["report"]["projection"]
        need(projection["input_sha256"] == "sha256:" + sha(raw)
             and projection["availability_sha256"] == "sha256:" + sha(sidecar), "public record bindings differ")
        proc = processes[row["label"] + "-CLI"]
        matches(raw, proc["stdin"])
        need(member(evidence, row["label"] + "-raw.json") == raw, "saved public input differs")
        saved = member(evidence, row["label"] + "-CLI/stdout"); matches(saved, proc["stdout"])
        exact(load(saved), row["report"])
        saved_err = member(evidence, row["label"] + "-CLI/stderr"); matches(saved_err, proc["stderr"])
        need(saved_err == b"", "public CLI diagnostic differs")
        need(sidecar == member(evidence, row["label"] + "-availability.json"), "saved sidecar bytes differ")
        original_parent = str(Path(summary["binary"]["path"]).parent)
        exact(proc["argv"], [summary["binary"]["path"], "--definitions", "--diagnostics", "--availability",
                            original_parent + "/" + row["label"] + "-availability.json", "--before-policy",
                            row["before_policy_revision"], "--after-policy", row["after_policy_revision"]])
        exact(row["report"]["bindings"], load(member(root, PROFILE + "availability-generation.json")))
        exact(row["report"]["observation_bindings"], load(member(root, PROFILE + "availability-observation-generation.json")))
    sources, licenses = check_sources(root, evidence, revision)
    generations = {}
    for name in sorted(sources):
        if name.startswith(PROFILE) and name.endswith("generation.json"):
            raw_generation = member(root, name); generation = load(raw_generation)
            need(generation["compiler_source_sha"] == pins["compiler_revision"]
                 and generation["go_version"] == go_version, "generation toolchain differs")
            for asset, digest in generation["generated_assets"].items():
                relative = asset if asset.startswith(PROFILE) else PROFILE + asset
                need(relative in sources and digest == "sha256:" + sources[relative]["sha256"],
                     "generation asset differs from bound source")
            if name == PROFILE + "snapshot-generation.json":
                check_snapshot_materials(generation, sources)
            generations[name] = identity(raw_generation)
    check_generation_roster(generations)
    snapshots = check_snapshot_regressions(root, sources, licenses["LICENSE"])
    snapshot_generation = load(member(root, PROFILE + "snapshot-generation.json"))
    return {"binary": binary, "build": build, "platform": target, "rows": rows, "sources": sources,
            "licenses": licenses, "public_export": identity(raw_export),
            "compiler_source_sha": pins["compiler_revision"], "generations": generations,
            "snapshots": snapshots, "snapshot_generation": snapshot_generation}


def write_archive(path, files):
    with path.open("xb") as output:
        with gzip.GzipFile(filename="", mode="wb", fileobj=output, mtime=0) as compressed:
            with tarfile.open(fileobj=compressed, mode="w", format=tarfile.USTAR_FORMAT) as archive:
                for name, (raw, mode) in sorted(files.items()):
                    need(type(name) is str and name and all(
                        part not in ("", ".", "..") and re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]*", part)
                        for part in name.split("/")), "safe relative archive member required")
                    member_info = tarfile.TarInfo(name); member_info.size = len(raw); member_info.mode = mode
                    archive.addfile(member_info, io.BytesIO(raw))
    return verify_archive(path, files)


def verify_archive(path, files):
    raw = read(path, BASE.MAX_ARCHIVE)
    decoder = zlib.decompressobj(16 + zlib.MAX_WBITS)
    decoded = decoder.decompress(raw, BASE.MAX_ARCHIVE + 1)
    need(len(decoded) <= BASE.MAX_ARCHIVE and decoder.eof and not decoder.unused_data
         and not decoder.unconsumed_tail, "single bounded gzip member required")
    return BASE.verify_archive(path, files)


def archive_files(checked, revision, producer_url):
    chosen = {r["label"]: r for r in checked["rows"]}
    supplied, unobserved = chosen["supplied-unknown"], chosen["unobserved-unknown"]
    need(supplied["raw_input_json"] == unobserved["raw_input_json"], "paired example input differs")
    need(supplied["before_policy_revision"] == supplied["after_policy_revision"] == "p"
         and unobserved["before_policy_revision"] == unobserved["after_policy_revision"] == "p", "example policy differs")
    guide = f"""Gooo saved-catalog comparison (experimental)

Run on {checked['platform']}; no Go/Gooo installation is required.
Keep the archive checksum, manifest, original input and sidecar with your report.

./{COMMAND} --version
./{COMMAND} --example > synthetic-observations.json
./{COMMAND} < synthetic-observations.json > report.json
./{COMMAND} --definitions --diagnostics --availability supplied-availability.json --before-policy p --after-policy p < availability-example.json > supplied-report.json
./{COMMAND} --definitions --diagnostics --availability unobserved-availability.json --before-policy p --after-policy p < availability-example.json > unobserved-report.json

Compare a saved definition without host aliases using the three included packets:
for case_id in output-schema-string-to-number same-definition-version-context-only missing-after-incomplete-page; do
  if ./{COMMAND} --definition-snapshot < "mcp-upgrade-regressions/$case_id/snapshot-input.json" > "$case_id.report.json" 2> "$case_id.stderr"; then
    printf '0\\n' > "$case_id.exit"
  else
    command_status=$?
    printf '%s\\n' "$command_status" > "$case_id.exit"
    cat "$case_id.stderr" >&2
    exit "$command_status"
  fi
done
Expected decision.state: OBSERVED_DEFINITION_CHANGED, OBSERVED_DEFINITION_SAME,
and UNASSESSED, respectively. All three are synthetic and exit zero on a completed
comparison. Read mcp-upgrade-regressions/README.md for input format and feedback.
observed-decision.json and packet.json preserve the original 85d96c7 regression
baseline. The archive manifest identifies this binary; packaging checks are retained
separately in the producer's verification.json, outside the archive.
A definition change does not prove changed behavior; incomplete pages cannot prove
removal. Origin and authorization stay unassessed, and no tool is called.

The paired example keeps the same text UNKNOWN: supplied observations yield
BINDINGS_MATCH; unavailable context yields UNASSESSED. Both are synthetic.

For your retained input, first create a source-bound roster:
./{COMMAND} --availability-template < saved-observations.json > availability.json
Fill every blank value_state with SUPPLIED or UNOBSERVED using retained evidence.
Then run:
./{COMMAND} --definitions --diagnostics --availability availability.json < saved-observations.json > explicit-report.json
Use identical policy arguments for template and comparison if known; otherwise
omit them and mark both policy entries UNOBSERVED. Never invent historical IDs.

Exit 0 returns a complete report, including unassessed domain results. Exit 1 is
an input/comparison/I/O failure; exit 2 is an argument error. Keep stderr on failure.
Input plus sidecar <=1 MiB; report <=4 MiB. No live collection, tool execution,
model call or execution authorization occurs. A sidecar is not authentication.

Source and full format:
https://github.com/kimjooyoon/gooo-go-ports/blob/{revision}/examples/recorded-catalog-comparison/README.md
Language: https://github.com/kimjooyoon/meta-ontology-go
Feedback (own input or unchanged example, revision, useful result or first failure):
https://github.com/kimjooyoon/gooo-go-ports/issues/new?template=problem.yml
Keep private inputs and credentials local.

This reuses a retained verified executable. BuildInfo's (devel) and absent VCS
identity are preserved. The manifest binds public source and binary hashes; it
is a producer statement, not an authenticated build or production-safety claim.
"""
    files = {COMMAND: (checked["binary"], 0o755), "START-HERE.txt": (guide.encode(), 0o644),
             "availability-example.json": (supplied["raw_input_json"].encode(), 0o644),
             "supplied-availability.json": (supplied["availability_json"].encode(), 0o644),
             "unobserved-availability.json": (unobserved["availability_json"].encode(), 0o644)}
    files.update({name: (raw, 0o644) for name, raw in checked["licenses"].items()})
    files.update({REGRESSION_ARCHIVE + name: (raw, 0o644) for name, raw in checked["snapshots"]["files"].items()})
    manifest = {"schema": "gooo/recorded-catalog-cli-distribution/v1", "command": COMMAND,
                "platform": checked["platform"], "source_revision": revision, "build_info": checked["build"],
                "producer_url": producer_url, "compiler_source_sha": checked["compiler_source_sha"],
                "generation_manifests": checked["generations"],
                "packaging_sources": checked["packaging_sources"],
                "source_files": checked["sources"], "public_consumer_export": checked["public_export"],
                "public_consumer_records": 13, "public_field_diagnostics": 442,
                "snapshot_regressions": {"cases": 3, "decision_fields": 32,
                    "archive_root": REGRESSION_ARCHIVE, "source_root": REGRESSION_ROOT,
                    "original_validation_source_commit": checked["snapshots"]["original_source_commit"],
                    "scope": "Historical full decisions are frozen regression baselines, checked with separate authored anchors; original native validation is not inherited."},
                "members": {name: {**identity(raw), "mode": mode} for name, (raw, mode) in sorted(files.items())},
                "scope": "Retained producer evidence, thirteen public synthetic availability records, and three synthetic snapshot regressions against full frozen decisions plus authored anchors. No new independent oracle, Gooo execution/replay, producer authentication, external adoption or execution authorization.",
                "private_observations_included": False, "recompiled": False}
    # This records the packaging relationship and the producer's source binding.
    # Archive/manifest hashes are deliberately absent: including either here
    # would create a self-reference. No new native execution is asserted.
    source_id = "urn:gooo:catalog-source:" + revision
    binary_id = "urn:sha256:" + sha(checked["binary"])
    member_id = "urn:gooo:catalog-distribution-members:" + sha(encoded(manifest["members"]))
    packaging_id = "urn:gooo:catalog-packaging-source:" + sha(encoded(checked["packaging_sources"]))
    activity_id = member_id + ":package:" + sha(encoded(checked["packaging_sources"]))
    generation_ids = ["urn:sha256:" + value["sha256"] for value in checked["generations"].values()]
    manifest["provenance"] = {
        "@context": {"prov": "http://www.w3.org/ns/prov#"},
        "@graph": [
            {"@id": source_id, "@type": "prov:Entity"},
            {"@id": packaging_id, "@type": "prov:Entity"},
            *[{"@id": value, "@type": "prov:Entity"} for value in generation_ids],
            {"@id": binary_id, "@type": "prov:Entity", "prov:wasDerivedFrom": [{"@id": source_id}, *[{"@id": value} for value in generation_ids]]},
            {"@id": activity_id, "@type": "prov:Activity", "prov:used": [{"@id": binary_id}, {"@id": source_id}, {"@id": packaging_id}]},
            {"@id": member_id, "@type": "prov:Entity", "prov:wasGeneratedBy": {"@id": activity_id},
             "prov:wasDerivedFrom": {"@id": binary_id}},
        ],
        "scope": "The source/generation relation is retained producer evidence, not authenticated reproducible-build proof. Content-addressed packaging derivation IDs describe the inputs and member set, not unique process occurrences. Packaging does not compile the executable; actual process observations are in verification.json outside the archive.",
    }
    files["manifest.json"] = (encoded(manifest), 0o644)
    return files


def smoke(directory, checked, records, controls):
    binary = directory / COMMAND
    env = {"PATH": "", "HOME": str(directory), "TMPDIR": str(directory), "LANG": "C", "LC_ALL": "C"}
    def run(label, args, raw=b""):
        return run_owned([binary, *args], raw, directory, env, label, records, record_dir=controls)
    out, err, code = run("version", ["--version"])
    build = checked["build"]
    expected = {"schema": "gooo/recorded-catalog-command-build/v1", "build_info_available": True,
                "go_version": build["GoVersion"], "package_path": build["Path"],
                # runtime/debug.Module omits empty Sum and nil Replace in JSON.
                "main_module": build["Main"], "vcs": {}}
    need(code == 0 and not err, "version command failed"); exact(load(out), expected)
    out, err, code = run("example", ["--example"])
    need(code == 0 and not err, "example command failed")
    need(sha(out) == checked["sources"][PROFILE + "example-input.json"]["sha256"], "embedded example differs")
    default, err, code = run("default-example", [], out)
    need(code == 0 and not err, "default synthetic comparison failed")
    default_report = load(default)
    alias, collection = default_report["alias_decision"], default_report["collection_context"]
    need(alias["state"] == "ORIGIN_CHANGED" and collection["state"] == "RECORDED_CONTEXT_COMPLETE"
         and collection["contains_synthetic_fixture"] is True and collection["remote_atomicity"] == "UNKNOWN"
         and alias["execution_authorized"] is False and collection["execution_authorized"] is False
         and collection["live_removal_proven"] is False, "default example decisions or uncertainty differ")
    for row in checked["snapshots"]["rows"]:
        raw = member(directory, REGRESSION_ARCHIVE + row["case"] + "/snapshot-input.json")
        need(raw == row["input"], "extracted snapshot input differs")
        out, err, code = run("snapshot-" + row["case"], ["--definition-snapshot"], raw)
        need(code == 0 and not err, "snapshot comparison failed")
        check_snapshot_report(out, raw, row, checked["snapshot_generation"])
    for row in checked["rows"]:
        sidecar = directory / "control-availability.json"; sidecar.write_text(row["availability_json"])
        args = ["--definitions", "--diagnostics", "--availability", sidecar,
                "--before-policy", row["before_policy_revision"], "--after-policy", row["after_policy_revision"]]
        out, err, code = run(row["label"], args, row["raw_input_json"].encode())
        need(code == 0 and not err, "public comparison failed"); exact(load(out), row["report"])
    row = checked["rows"][0]; raw = row["raw_input_json"].encode()
    out, err, code = run("blank-template", ["--availability-template", "--before-policy", "p", "--after-policy", "p"], raw)
    need(code == 0 and not err, "template failed")
    template = load(out); supplied = load(row["availability_json"].encode())
    need(template["schema"] == supplied["schema"] and template["input_sha256"] == supplied["input_sha256"]
         and template["policy_sha256"] == supplied["policy_sha256"], "template binding differs")
    need(len(template["fields"]) == len(supplied["fields"]) and all(f["value_state"] == "" for f in template["fields"]), "template states differ")
    exact([{k: v for k, v in f.items() if k != "value_state"} for f in template["fields"]],
          [{k: v for k, v in f.items() if k != "value_state"} for f in supplied["fields"]])
    sidecar.write_text(row["availability_json"])
    args = ["--definitions", "--diagnostics", "--availability", sidecar, "--before-policy", "p", "--after-policy", "p"]
    for label, arguments, input_raw, exit_code, diagnostic in (
        ("changed-input-binding", args, raw + b" ", 1, {"state": "INVALID_TRANSPORT", "stage": "decode_json", "code": "AVAILABILITY_SOURCE_BINDING"}),
        ("invalid-argument", ["--not-a-supported-option"], b"", 2, {"state": "INVALID_ARGUMENT", "stage": "parse_arguments", "code": "UNSUPPORTED_ARGUMENTS"}),
    ):
        out, err, code = run(label, arguments, input_raw)
        need(code == exit_code and not out, "refusal exit/output differs"); exact(load(err), diagnostic)
    changed = dict(supplied, input_sha256="sha256:" + "0" * 64)
    sidecar.write_bytes(encoded(changed))
    out, err, code = run("changed-sidecar-binding", args, raw)
    need(code == 1 and not out, "sidecar mismatch was not refused")
    exact(load(err), {"state": "INVALID_TRANSPORT", "stage": "decode_json", "code": "AVAILABILITY_SOURCE_BINDING"})


def produce(root, evidence, synthetic_export, revision, out, producer_url=None):
    need(not out.exists() and not out.is_symlink(), "new output directory required")
    need(producer_url is None or re.fullmatch(r"https://github\.com/kimjooyoon/gooo-go-ports/actions/runs/[1-9][0-9]*", producer_url), "producer URL must name this repository's run")
    out.mkdir(parents=True, mode=0o700)
    records = []; candidate = None; final = None; checksum = None
    try:
        checked = check_evidence(root, evidence, synthetic_export, revision)
        def packaging_sources():
            return {"tools/" + path.name: identity(read(path, MAX_SOURCE)) for path in
                    (Path(__file__).resolve(), Path(BASE.__file__).resolve())}
        checked["packaging_sources"] = packaging_sources()
        files = archive_files(checked, revision, producer_url)
        candidate = out / "candidate.tar.gz"
        write_archive(candidate, files)
        with tempfile.TemporaryDirectory(prefix="extracted-", dir=out) as temporary:
            directory = Path(temporary)
            with tarfile.open(candidate, "r:gz") as archive:
                for name, (raw, mode) in files.items():
                    content = archive.extractfile(name).read(len(raw) + 1); need(content == raw, "extracted content differs")
                    path = directory / name
                    path.parent.mkdir(parents=True, exist_ok=True)
                    path.write_bytes(content); path.chmod(mode)
            smoke(directory, checked, records, out / "controls")
        # Reject source/binary mutation during the extracted runtime checks.
        again = check_evidence(root, evidence, synthetic_export, revision)
        exact(again["sources"], checked["sources"]); need(again["binary"] == checked["binary"], "binary changed during packaging")
        exact(packaging_sources(), checked["packaging_sources"])
        name = COMMAND + "-" + checked["platform"].replace("/", "-") + ".tar.gz"
        final = out / name; candidate.rename(final); candidate = None
        archive = verify_archive(final, files)
        checksum = out / (name + ".sha256"); checksum.write_text(archive["sha256"] + "  " + name + "\n")
        report = {"schema": "gooo/recorded-catalog-cli-bundle-check/v1", "status": "EXTRACTED_NATIVE_CATALOG_BUNDLE_PASS",
                  "source_revision": revision, "platform": checked["platform"], "archive": identity(read(final, BASE.MAX_ARCHIVE)),
                  "archive_name": name, "checksum_name": checksum.name, "members": sorted(files),
                  "public_consumer_reports": 13, "snapshot_regression_cases": 3, "snapshot_decision_fields": 32,
                  "snapshot_original_source_commit": checked["snapshots"]["original_source_commit"], "processes": records, "all_process_groups_closed": all(r["process_group_closed"] for r in records),
                  "runtime_PATH_empty": True, "runtime_GOROOT_absent": True, "recompiled": False,
                  "new_Gooo_execution_or_replay": False, "external_adoption": False,
                  "scope": "Packaging checks against retained independently checked availability outputs and three frozen snapshot regression decisions plus authored anchors; no additional independent semantic oracle or producer authentication."}
        (out / "verification.json").write_bytes(encoded(report))
        return report
    except BaseException as error:
        for path in (candidate, final, checksum):
            if path is not None and path.exists():
                path.unlink()
        (out / "first-failure.json").write_bytes(encoded({"status": "FIRST_FAILURE_NO_PUBLISHABLE_ARCHIVE",
             "error_type": type(error).__name__, "message": str(error)[:2048], "processes": records}))
        raise


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("root", "consumer-evidence", "synthetic-export", "out"):
        parser.add_argument("--" + name, type=Path, required=True)
    parser.add_argument("--source-revision", required=True)
    parser.add_argument("--producer-url")
    args = parser.parse_args()
    report = produce(args.root.absolute(), args.consumer_evidence.absolute(), args.synthetic_export.absolute(),
                     args.source_revision, args.out.absolute(), args.producer_url)
    print(json.dumps({"status": report["status"], "archive": report["archive_name"], "platform": report["platform"]}))


if __name__ == "__main__":
    main()
