#!/usr/bin/env python3
"""Offline, finite checks of imported Gooo version policies and comparison edges.

Prepare the authenticated x/mod v0.29.0 module cache separately. This runner
never downloads, applies a release, or treats finite observations as full API
compatibility. It builds an independent Go oracle before constructing cases.
"""
import argparse
import base64
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
import zipfile

ROOT = Path(__file__).resolve().parents[2]
PIN = "205896a413e5dea92366309c35ae85e542d055f6"
MODULE = "golang.org/x/mod"
VERSION = "v0.29.0"
SUM = "h1:HV8lRxZC4l2cr3Zq1LvtOsi/ThTgWnUk/y64QSs8GwA="
MODSUM = "h1:NyhrlYXJ2H4eJiRy/WDBO6HMqZQ6q9nk4JzS3NuCK+w="
ORACLE = "github.com/kimjooyoon/gooo-go-ports/examples/version-eligibility/oracle"
ENTRY = "app/version-eligibility:AssessUpgrade"
AID = "gooo://go-ports/examples/version-eligibility/v1/assess-upgrade"
RID = "gooo://go-ports/examples/version-eligibility/v1/result"
FIELDS = ("current", "candidate", "current_valid", "candidate_valid",
          "candidate_vs_current", "precedence_newer", "eligible", "reason", "raw_changed")
TYPES = (str, str, bool, bool, int, bool, bool, str, bool)
SOURCES = ("ports/xmod/semver.gooo", "examples/version-eligibility/main.gooo")
MANIFEST = "version-eligibility.workspace.json"
EXAMPLE = "examples/version-eligibility"
MAX_OUTPUT = 2 << 20
# Current compiler provenance expands finite receipts beyond ordinary command output.
# All six fixed profiles retain <=12 cases and <=2 source files; the measured
# 12 execute/replay receipts peak at 10,428,916 bytes. Keep a finite 16 MiB receipt
# budget with margin, below the compiler's 32 MiB saved-receipt JSON ceiling.
MAX_RECEIPT = 16 << 20
MAX_WORK = 150 << 20


def policy(name):
    if name == "upgrade":
        return {"count": 8, "sources": SOURCES, "parameters": ("current", "candidate"), "fields": FIELDS, "types": TYPES,
                "entry": ENTRY, "aid": AID, "rid": RID, "manifest": MANIFEST,
                "inputs": "inputs.json", "reference": "reference-inputs.json"}
    if name == "compare-boundary":
        return {"count": 9, "sources": SOURCES[:1], "parameters": ("candidate", "current"),
                "entry": "go/golang.org/x/mod/semver:Compare", "aid": "gooo://golang.org/x/mod/semver/Compare",
                "manifest": "version-semver-compare.workspace.json", "scalar": True,
                "inputs": None, "reference": "boundary-reference-inputs.json"}
    if name in ("major", "prerelease"):
        activity = {"major": "Major", "prerelease": "Prerelease"}[name]
        return {"count": 9, "sources": SOURCES[:1], "parameters": ("version",),
                "entry": "go/golang.org/x/mod/semver:" + activity,
                "aid": "gooo://golang.org/x/mod/semver/" + activity,
                "manifest": "version-semver-" + name + ".workspace.json",
                "scalar": True, "scalar_type": str, "inputs": None,
                "reference": "projection-reference-inputs.json"}
    if name == "stable":
        selected = policy("minimum")
        selected.update(count=12,
                        fields=selected["fields"][:-1] + ("same_major", "candidate_stable", "policy_eligible", "policy_reason"),
                        types=selected["types"][:-1] + (bool, bool, bool, str),
                        entry="app/version-eligibility:AssessStableSameMajorUpdate",
                        aid=AID.replace("upgrade", "stable-same-major-update"),
                        rid=RID.replace("/result", "/stable-same-major-result"),
                        manifest="version-stable-same-major.workspace.json",
                        inputs="stable-inputs.json", reference="stable-reference-inputs.json")
        return selected
    need(name == "minimum", "unknown policy")
    return {"count": 9, "sources": SOURCES, "parameters": ("current", "candidate", "minimum"),
            "fields": ("current", "candidate", "minimum", "current_valid", "candidate_valid", "minimum_valid",
                       "candidate_vs_current", "candidate_vs_minimum", "precedence_newer", "meets_minimum",
                       "eligible", "reason", "raw_changed"),
            "types": (str, str, str, bool, bool, bool, int, int, bool, bool, bool, str, bool),
            "entry": "app/version-eligibility:AssessUpdate", "aid": AID.replace("upgrade", "update"),
            "rid": RID.replace("/result", "/update-result"), "manifest": "version-minimum.workspace.json",
            "inputs": "minimum-inputs.json", "reference": "minimum-reference-inputs.json"}


def need(value, message):
    if not value:
        raise ValueError(message)


def pairs(items):
    result = {}
    for key, value in items:
        need(key not in result, "duplicate decoded JSON key")
        result[key] = value
    return result


def load(raw, limit=None):
    if limit is None:
        limit = MAX_OUTPUT
    need(len(raw) <= limit, "JSON exceeds bounded profile")
    return json.loads(raw.decode("utf-8"), object_pairs_hook=pairs,
                      parse_constant=lambda _: need(False, "nonfinite JSON"))


def sha(raw):
    return "sha256:" + hashlib.sha256(raw).hexdigest()


def read(path, limit=MAX_OUTPUT):
    need(stat.S_ISREG(path.lstat().st_mode), "regular file required: " + str(path))
    need(path.stat().st_size <= limit, "file exceeds bounded profile")
    raw = path.read_bytes()
    need(len(raw) <= limit, "file grew over cap")
    return raw


def source_digest(path):
    text = json.dumps(read(path).decode(), ensure_ascii=False)
    for original, escaped in (("<", "\\u003c"), (">", "\\u003e"), ("&", "\\u0026"),
                              ("\u2028", "\\u2028"), ("\u2029", "\\u2029")):
        text = text.replace(original, escaped)
    return sha(text.encode())


def identity(path):
    raw = read(path, 64 << 20)
    return {"bytes": len(raw), "sha256": sha(raw)}


def write(path, value):
    path.write_text(json.dumps(value, indent=2, allow_nan=False) + "\n", encoding="utf-8")


def same(a, b):
    if type(a) is not type(b):
        return False
    if isinstance(a, dict):
        return a.keys() == b.keys() and all(same(a[k], b[k]) for k in a)
    if isinstance(a, list):
        return len(a) == len(b) and all(same(x, y) for x, y in zip(a, b))
    return a == b


def input_key(selected, position):
    # Gooo unary roots use the activity name; only multi-input roots have suffixes.
    return selected["entry"] + ("" if len(selected["parameters"]) == 1 else ".input" + str(position))


def synchronized(reference, inputs, selected=None):
    selected = selected or policy("upgrade")
    need(type(reference) is list and len(reference) == selected["count"], "exact finite reference anchors required")
    if selected["inputs"] is not None:
        need(set(inputs) == {"schema", "inputs"} and inputs["schema"] == "gooo/body-composition-inputs/v1",
             "input-only document required")
    wanted = []
    names = set()
    for row in reference:
        need(set(row) == {"name", *selected["parameters"]} and type(row["name"]) is str and
             row["name"] not in names, "unique named reference inputs")
        names.add(row["name"])
        for key in selected["parameters"]:
            need(type(row[key]) is str and len(row[key].encode("utf-8")) <= 1024,
                 "decoded UTF-8 Text 0..1024 bytes required")
        wanted.append({input_key(selected, i): row[key]
                       for i, key in enumerate(selected["parameters"])})
    if selected["inputs"] is not None:
        need(same(inputs["inputs"], wanted), "Gooo and reference input values/order differ")
    return wanted


def hash1(files):
    summary = b"".join((hashlib.sha256(data).hexdigest() + "  " + name + "\n").encode()
                       for name, data in sorted(files.items()))
    return "h1:" + base64.b64encode(hashlib.sha256(summary).digest()).decode()


def seed_module(cache, destination):
    source = cache / "cache/download/golang.org/x/mod/@v"
    raw = {ext: read(source / (VERSION + "." + ext)) for ext in ("zip", "mod", "info")}
    info = load(raw["info"])
    need(info["Version"] == VERSION and info["Origin"]["Hash"] ==
         "bba3e065a67271df90253c78c98f2cea7f572948", "cached module origin differs")
    import io
    with zipfile.ZipFile(io.BytesIO(raw["zip"])) as archive:
        members = archive.infolist()
        need(len({x.filename for x in members}) == len(members) and
             sum(x.file_size for x in members) <= 8 << 20, "bounded unique module ZIP")
        need(all(x.filename.startswith(MODULE + "@" + VERSION + "/") and
                 ".." not in Path(x.filename).parts and not x.flag_bits & 1 and
                 stat.S_IFMT(x.external_attr >> 16) in (0, stat.S_IFREG) for x in members),
             "regular module ZIP members required")
        need(hash1({x.filename: archive.read(x) for x in members}) == SUM,
             "cached module ZIP h1 differs")
    need(hash1({"go.mod": raw["mod"]}) == MODSUM, "cached module go.mod h1 differs")
    destination.mkdir(parents=True)
    for ext, data in raw.items():
        (destination / (VERSION + "." + ext)).write_bytes(data)
    # Do not copy ziphash: Go must check the archive against the fixed go.sum.


def build_info(value):
    need(value["GoVersion"] == "go1.27.3" and value["Path"] == ORACLE and
         value["Main"]["Path"] == ORACLE and not value["Main"].get("Replace") and
         same(value["Deps"], [{"Path": MODULE, "Version": VERSION, "Sum": SUM}]),
         "oracle BuildInfo module/version/h1/replacement differs")


class Runner:
    def __init__(self, out, work, env):
        self.out, self.work, self.env = out, work, env
        self.deadline = time.monotonic() + 300
        self.commands, self.processes, self.signals = [], [], []
        self.peak_bytes = 0

    def interrupted(self, signum, _frame):
        self.signals.append(signum)

    def run(self, label, argv, cwd, stdin=None):
        need(not self.signals, "interrupted")
        output_limits = {"stdout": MAX_RECEIPT if label in ("execute", "replay") else MAX_OUTPUT,
                         "stderr": MAX_OUTPUT}
        record = {"label": label, "argv": list(map(str, argv)), "output_limits": output_limits}
        self.commands.append(record)
        started = time.monotonic()
        input_file = stdin.open("rb") if stdin else None
        try:
            proc = subprocess.Popen(record["argv"], cwd=cwd, env=self.env,
                                    stdin=input_file or subprocess.DEVNULL,
                                    stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)
        finally:
            if input_file:
                input_file.close()
        self.processes.append(proc)
        selector = selectors.DefaultSelector()
        for name in ("stdout", "stderr"):
            selector.register(getattr(proc, name), selectors.EVENT_READ, name)
        sizes = {"stdout": 0, "stderr": 0}
        try:
            with (self.out / (label + ".stdout")).open("xb") as out, (self.out / (label + ".stderr")).open("xb") as err:
                streams = {"stdout": out, "stderr": err}
                while selector.get_map():
                    need(not self.signals and time.monotonic() < min(self.deadline, started + 90),
                         "command deadline or interruption: " + label)
                    total = 0
                    for base, _, files in os.walk(self.work):
                        for name in files:
                            try:
                                total += (Path(base) / name).stat().st_size
                            except FileNotFoundError:
                                pass  # Go removes temporary build files concurrently.
                    self.peak_bytes = max(self.peak_bytes, total)
                    need(total <= MAX_WORK, "private work exceeds 150 MiB")
                    for key, _ in selector.select(0.2):
                        chunk = os.read(key.fd, 65536)
                        if not chunk:
                            selector.unregister(key.fileobj)
                            continue
                        sizes[key.data] += len(chunk)
                        need(sizes[key.data] <= output_limits[key.data], "bounded command output: " + label)
                        streams[key.data].write(chunk)
                record["exit"] = proc.wait(timeout=max(1, min(self.deadline, started + 90) - time.monotonic()))
                need(record["exit"] == 0, "command failed: " + label)
        finally:
            try:
                os.killpg(proc.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            proc.wait(timeout=10)
            selector.close()
            proc.stdout.close()
            proc.stderr.close()
            record.update(exit=proc.returncode, waited=True, elapsed_seconds=time.monotonic() - started)
            for name in ("stdout", "stderr"):
                path = self.out / (label + "." + name)
                if path.exists():
                    record[name] = identity(path)
        diagnostics = read(self.out / (label + ".stderr"))
        cached = b"go: downloading golang.org/x/mod v0.29.0\n"
        need(not diagnostics or label == "oracle-build" and diagnostics == cached,
             "unexpected command diagnostics: " + label)
        return read(self.out / (label + ".stdout"), output_limits["stdout"])


def receipt(raw, expected, rows, source_ids, manifest, cases, go_identity, version, selected=None):
    selected = selected or policy("upgrade")
    scalar, count = selected.get("scalar", False), selected["count"]
    e = load(raw, MAX_RECEIPT)
    need(e["schema"] == "gooo/workspace-body-execution-receipt/v1" and e["decision"] == "PASS" and
         e.get("error", "") == "" and e["manifest_digest"] == manifest and e["cases_digest"] == cases,
         "finite receipt/manifest/cases binding")
    result = e["result"]
    rt, comp = result["runtime"], result["composition"]
    need(rt["stage"] == "COMPLETE" and rt.get("failure") is None and rt["producer_source_sha"] == PIN and
         rt["go_version"] == version and rt["go_tool_sha256"] == go_identity["sha256"] and
         same(rt["producer_module"], {"path": "github.com/kimjooyoon/meta-ontology-go",
              "version": "v0.6.37-dev.0.20261020180505-205896a413e5"}) and
         same([rt["finite_total"], rt["finite_passed"], rt["model_calls"]], [count, count, 0]) and
         rt["projection_replayed"] is True and rt["runtime_replayed"] is True, "native producer/counters/replay")
    image = result["program"]["workspace_image"]
    need(result["program"]["entry"]["activity"] == selected["entry"].split(":")[1], "policy entry activity")
    packages = ["go/golang.org/x/mod/semver"] + ([] if scalar else ["app/version-eligibility"])
    need(image["init_order"] == packages and len(image["packages"]) == len(packages) and
         (scalar or image["packages"][1]["imports"] == ["go/golang.org/x/mod/semver"]),
         "imported two-package source closure")
    source_rows = [s for p in image["packages"] for s in p["sources"]]
    observed = {s["filename"]: s["source_digest"] for s in source_rows}
    need(len(source_rows) == len(observed) == len(source_ids) and same(observed, source_ids), "original source-image digest bindings")
    for key, text in (("generated_sha256", comp["source"]), ("driver_sha256", comp["driver_source"])):
        need(comp[key] == rt[key] == sha(text.encode()), "generated body/driver binding")
    lower = result["program"]["lowered_gooo_source"]
    need(result["execution_source_sha256"] == rt["selected_source_sha256"] ==
         comp["selected_source_sha256"] == sha(lower.encode()) and comp["gooo_source"] == lower and
         rt["typed_plan_sha256"] == comp["plan"]["typed_plan_sha256"], "lowered source/plan binding")
    need(len(rt["traces"]) == count, "finite trace roster")
    if not scalar:
        fields, types = selected["fields"], selected["types"]
        record = [x for x in comp["plan"]["record_types"] if x["id"] == selected["rid"]]
        uris = ["urn:gooo:type:" + {str: "string", bool: "boolean", int: "integer"}[t] for t in types]
        need(len(record) == 1 and [x["name"] for x in record[0]["fields"]] == list(fields) and
             [x["type_id"] for x in record[0]["fields"]] == uris, "policy typed ABI")
    for i, trace in enumerate(rt["traces"]):
        need(type(trace["case_index"]) is int and trace["case_index"] == i and len(trace["deliveries"]) == 1,
             "ordered finite trace cases")
        d = trace["deliveries"][0]
        actual_inputs = [{"port": "input" if len(selected["parameters"]) == 1 else "input" + str(j),
                          "entity_id": "gooo://std/text", "value": rows[i][input_key(selected, j)]}
                         for j in range(len(selected["parameters"]))]
        inputs_match = ("input" in d and same(d["input"], actual_inputs[0]["value"]) and not d.get("inputs")
                        if len(selected["parameters"]) == 1 else same(d.get("inputs"), actual_inputs))
        need(d["activity_id"] == selected["aid"] and d["passed"] is True and same(d["actual"], expected[i]) and
             same(d["expected"], expected[i]) and inputs_match,
             "actual/expected/input facts differ from independent oracle")
        if scalar:
            need(type(d["actual"]) is selected.get("scalar_type", int) and not d.get("actual_fields"),
                 "typed scalar output without record fields")
            continue
        need([f["name"] for f in d["actual_fields"]] == list(fields), "ordered actual fields")
        for name, kind, field in zip(fields, types, d["actual_fields"]):
            need(field["id"] == selected["rid"] + "/" + name and type(field["value"]) is kind and
                 same(field["value"], expected[i][name]), "field identity/value/type")
    return e


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", required=True, type=Path)
    parser.add_argument("--policy", choices=("upgrade", "minimum", "compare-boundary", "stable", "major", "prerelease"), default="upgrade")
    parser.add_argument("--compiler", type=Path, default=ROOT / ".gooo")
    parser.add_argument("--go", type=Path)
    parser.add_argument("--module-cache", type=Path, required=True, help="already authenticated Go module cache")
    args = parser.parse_args()
    selected = policy(args.policy)
    manifest = selected["manifest"]
    inputs_name = EXAMPLE + "/" + selected["inputs"] if selected["inputs"] else None
    reference_name = EXAMPLE + "/" + selected["reference"]
    out = args.out.absolute()
    need(not out.exists(), "output exists; choose a new evidence directory")
    go = Path(args.go or shutil.which("go") or "").resolve()
    compiler = args.compiler.absolute()
    names = [*selected["sources"], manifest, reference_name, EXAMPLE + "/oracle/main.go",
             EXAMPLE + "/oracle/go.mod", EXAMPLE + "/oracle/go.sum"]
    originals = {name: identity(ROOT / name) for name in names + ([inputs_name] if inputs_name else [])}
    tools = {"go": identity(go), "compiler": identity(compiler)}
    out.mkdir(mode=0o700, parents=True)
    work = Path(tempfile.mkdtemp(prefix="owned-", dir=out))
    env = {"HOME": os.environ["HOME"], "PATH": str(go.parent) + os.pathsep + "/usr/bin:/bin",
           "GOTOOLCHAIN": "local", "GOWORK": "off", "GOENV": "off", "GOFLAGS": "-mod=readonly",
           "GOPROXY": "off", "GOSUMDB": "off", "GOVCS": "*:off", "GOCACHE": str(work / "cache"),
           "GOMODCACHE": str(work / "modules"), "GOPATH": str(work / "gopath"), "TMPDIR": str(work / "tmp"),
           "GOTMPDIR": str(work / "tmp")}
    runner = Runner(out, work, env)
    handlers = {sig: signal.signal(sig, runner.interrupted) for sig in (signal.SIGTERM, signal.SIGINT)}
    report = {"schema": "gooo/version-eligibility-check/v1", "status": "FAILED", "commands": runner.commands}
    failure = None
    try:
        for name in ("cache", "modules", "gopath", "tmp"):
            (work / name).mkdir()
        seed_module(args.module_cache, work / "modules/cache/download/golang.org/x/mod/@v")
        oracle = work / "oracle"
        oracle.mkdir()
        for name in ("main.go", "go.mod", "go.sum"):
            (oracle / name).write_bytes(read(ROOT / EXAMPLE / "oracle" / name))
        need((oracle / "go.sum").read_text() == MODULE + " " + VERSION + " " + SUM + "\n" +
             MODULE + " " + VERSION + "/go.mod " + MODSUM + "\n", "fixed two-line oracle checksums")
        reference_path = ROOT / reference_name
        rows = synchronized(load(read(reference_path)), load(read(ROOT / inputs_name)) if inputs_name else None, selected)
        version = runner.run("go-version", [go, "version"], work).decode().strip()
        need(version.startswith("go version go1.27.3 "), "Go 1.27.3 required")
        build = load(runner.run("compiler-build", [compiler, "version", "--build", "--json"], work))
        need(build["compiler_source_sha"] == build["vcs_revision"] == PIN and
             build["vcs_modified"] == "false" and build["source_status"] == "CLEAN_VCS" and
             build["go_version"] == build["native_go_required"] == "go1.27.3" and
             same(build["module"], {"path": "github.com/kimjooyoon/meta-ontology-go",
                  "version": "v0.6.37-dev.0.20261020180505-205896a413e5"}) and
             same(build["decision_runtime"], {"module_path": "github.com/kimjooyoon/gooo-decision-runtime",
                  "version": "v0.2.51-experimental", "replaced": False}), "clean pinned compiler required")
        binary = work / "oracle-bin"
        runner.run("oracle-build", [go, "build", "-buildvcs=false", "-trimpath", "-o", binary, "."], oracle)
        build_info(load(runner.run("oracle-buildinfo", [go, "version", "-m", "-json", binary], work)))
        scalar, count = selected.get("scalar", False), selected["count"]
        expected = load(runner.run("oracle", [binary, "--policy", "upgrade" if args.policy == "compare-boundary" else args.policy], work, reference_path))
        if selected.get("scalar_type") is str:
            need(len(expected) == count and all(type(row) is str for row in expected),
                 "independently typed finite Text oracle rows")
        else:
            fields, types = (FIELDS, TYPES) if scalar else (selected["fields"], selected["types"])
            need(len(expected) == count and all(set(row) == set(fields) and all(type(row[n]) is t for n, t in zip(fields, types))
                                          for row in expected), "independently typed finite oracle rows")
            if scalar:
                expected = [row["candidate_vs_current"] for row in expected]
        cases = out / "cases.json"
        write(cases, {"schema": "gooo/body-composition-cases/v1", "cases": [
            {"inputs": row, "expected": {selected["entry"]: want}} for row, want in zip(rows, expected)]})
        if scalar:
            # A source-offset suffix cannot have more digits than source bytes.
            # This bounds the compiler's rewritten activity key without expansion.
            lower = "GoooPackage" + str(originals[SOURCES[0]]["bytes"]) + "Activity" + selected["entry"].split(":")[1]
            need(len(lower) <= len(selected["entry"]) and cases.stat().st_size <= 30000 and
                 cases.stat().st_size <= 32768, "boundary case raw/translated byte caps")
        source_ids = {name: source_digest(ROOT / name) for name in selected["sources"]}
        before = runner.run("execute", [compiler, "package", "execute", "--json", "--cases", cases, manifest], ROOT)
        check_args = (expected, rows, source_ids, originals[manifest]["sha256"], identity(cases)["sha256"], tools["go"], version, selected)
        original = receipt(before, *check_args)
        after = runner.run("replay", [compiler, "package", "replay", "--json", "--receipt", out / "execute.stdout",
                                     "--cases", cases, manifest], ROOT)
        replay = receipt(after, *check_args)
        need(replay["replayed_from_sha256"] == sha(before) and replay["result"]["replay"]["model_calls"] == 0 and
             same(original["result"]["program"], replay["result"]["program"]) and
             all(original["result"]["runtime"][k] == replay["result"]["runtime"][k]
                 for k in ("generated_sha256", "driver_sha256", "typed_plan_sha256")) and
             same(original["result"]["runtime"]["traces"], replay["result"]["runtime"]["traces"]), "fresh saved replay parent/program/typed traces")
        status = "FINITE_9_BOUNDARY_CASES_AND_FRESH_REPLAY_CHECKED" if args.policy == "compare-boundary" else f"FINITE_{count}_CASES_AND_FRESH_REPLAY_CHECKED"
        report.update(status=status, compiler_build=build, source_inputs=originals, tools=tools,
                      cases=identity(cases), checked=count, policy=args.policy,
                      limits="Finite cases, not full SemVer compatibility, signed execution or deployment authority.")
    except Exception as error:
        failure = error
        report["error"] = str(error)
    finally:
        report["all_processes_waited"] = all(p.poll() is not None for p in runner.processes)
        def unchanged(path, expected):
            try:
                return identity(path) == expected
            except (OSError, ValueError):
                return False
        report["sources_unchanged"] = all(unchanged(ROOT / name, value) for name, value in originals.items())
        report["tools_unchanged"] = unchanged(go, tools["go"]) and unchanged(compiler, tools["compiler"])
        report["peak_private_work_bytes"] = runner.peak_bytes
        if report["all_processes_waited"]:
            for base, _, _ in os.walk(work):
                os.chmod(base, 0o700)
            shutil.rmtree(work)
        report["owned_work_removed"] = not work.exists()
        if runner.signals or not all(report[k] for k in ("sources_unchanged", "tools_unchanged", "all_processes_waited", "owned_work_removed")):
            report["status"] = "FAILED"
            failure = failure or ValueError("interrupted or final identity/cleanup failure")
        write(out / "summary.json", report)
        for sig, handler in handlers.items():
            signal.signal(sig, handler)
    if failure:
        raise failure
    print(json.dumps({"status": report["status"], "policy": args.policy, "cases": selected["count"], "summary": str(out / "summary.json")}))


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print("version eligibility check failed: " + str(error), file=sys.stderr)
        sys.exit(1)
