#!/usr/bin/env python3
"""Fresh input-only URL facts, independent pinned Go oracle, and saved replay.

No launch occurs on import. Native receipts remain OBSERVED: expected outputs
are compared here, outside the compiler. This sidecar does not enlarge the
separately reviewed 82-suite native catalog or claim full net/url compatibility.
"""
import argparse
import copy
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import stat
import sys
import tempfile


def sibling(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    value = importlib.util.module_from_spec(spec)
    sys.modules[name] = value
    spec.loader.exec_module(value)
    return value


H = sibling("url_component_owned_commands", Path(__file__).with_name("check-header-addition-host.py"))
REV = "205896a413e5dea92366309c35ae85e542d055f6"
PACKAGE = "app/url-context-facts"
ENTRY = PACKAGE + ":Report"
AID = "gooo://url-context-facts/report/v1"
OID = "gooo://url-context-facts/report-result/v1"
HELPER = "gooo://go/net/url/escape-component/v1"
WORKSPACE = "url-context-facts.workspace.json"
SOURCES = [("go/net/url", "ports/net/url/escape.gooo"),
           (PACKAGE, "examples/url-context-facts/main.gooo")]
FIELD_SPEC = [("original", "original", "string"),
              ("path_segment", "path-segment", "string"),
              ("query_value", "query-value", "string"),
              ("input_bytes", "input-bytes", "integer"),
              ("path_bytes", "path-bytes", "integer"),
              ("query_bytes", "query-bytes", "integer"),
              ("path_changed", "path-changed", "boolean"),
              ("query_changed", "query-changed", "boolean"),
              ("contexts_differ", "contexts-differ", "boolean"),
              ("original_percent_offset", "original-percent-offset", "integer")]
OWN = [WORKSPACE, "go.mod", "go.sum", "LICENSE", "third_party/GO-LICENSE",
       "upstream.json", "ports/net/url/provenance.json",
       "examples/url-context-facts/inputs.json", "tools/check-url-components.py",
       "tools/check-url-components_test.py", "tools/check-header-addition-host.py",
       "tools/native_verification_contract.py",
       "internal/urlcomponentinput/preflight.go", "internal/urlcomponentinput/preflight_test.go",
       "cmd/url-component-input/main.go", "cmd/url-component-input/main_test.go",
       "cmd/url-component-oracle/main.go", "cmd/url-component-oracle/main_test.go"]


def bounded_input(path):
    info = path.lstat()
    H.need(stat.S_ISREG(info.st_mode) and info.st_size <= 1 << 20, "bounded regular input file")
    with path.open("rb") as stream:
        H.need(os.path.samestat(info, os.fstat(stream.fileno())), "same opened input file")
        raw = stream.read((1 << 20) + 1)
    H.need(len(raw) <= 1 << 20 and os.path.samestat(info, path.lstat()), "bounded stable input file")
    return raw


def inputs(raw):
    H.need(len(raw) <= 1 << 20, "bounded corpus bytes")
    document = H.load(raw.decode("utf-8"))
    H.need(type(document) is dict and set(document) == {"schema", "inputs"}
           and document["schema"] == "gooo/body-composition-inputs/v1", "input-only schema")
    rows = document["inputs"]
    H.need(type(rows) is list and 1 <= len(rows) <= 128, "1..128 finite rows")
    for row in rows:
        H.need(type(row) is dict and set(row) == {ENTRY}
               and type(row[ENTRY]) is str, "one original Text input")
        H.need(len(row[ENTRY].encode("utf-8", "strict")) <= 341, "0..341 UTF-8 bytes")
    return rows


def definitions(sources):
    values = {}
    for package, filename in SOURCES:
        for activity in H.activities(sources[filename].decode("utf-8")):
            key = package + ":" + activity["name"]
            H.need(key not in values, "unique source activity")
            activity.update(package=package, filename=filename,
                            lowered="GoooPackage" + str(len(values)) + "Activity" + activity["name"])
            values[key] = activity
    H.need(list(values) == ["go/net/url:EscapeComponent", "go/net/url:PathEscape",
                            "go/net/url:QueryEscape", ENTRY], "four exact source activities")
    for key, activity in values.items():
        body = activity["body"]
        for other_key, other in values.items():
            if activity["package"] == other["package"]:
                body = re.sub(r"(?<![\w.])" + re.escape(other["name"]) + r"(?=\s*\()", other["lowered"], body)
            elif activity["package"] == PACKAGE and other["package"] == "go/net/url":
                body = body.replace("url." + other["name"], other["lowered"])
        activity["lowered_body"] = body
    return values


def validate(raw, rows, expected, manifest, raw_inputs, sources, go_version, go_sha,
             workspace, parent=None, program=None):
    receipt = H.load(raw)
    H.need(receipt["schema"] == "gooo/workspace-body-execution-receipt/v1"
           and receipt["decision"] == "OBSERVED" and not receipt.get("error"), "input-only observed receipt")
    H.need(receipt["manifest"] == str(workspace)
           and receipt["manifest_digest"] == H.digest(manifest)
           and receipt["inputs_digest"] == H.digest(raw_inputs)
           and "cases_digest" not in receipt, "exact manifest/input bytes and route")
    result = receipt["result"]
    H.need(result["schema"] == "gooo/workspace-body-execution/v1", "workspace execution schema")
    p, c, rt = result["program"], result["composition"], result["runtime"]
    defs = definitions(sources)
    root = defs[ENTRY]
    ref = {"package_path": PACKAGE, "activity": "Report", "lowered_name": root["lowered"]}
    H.need(p["schema"] == "gooo/workspace-body-program/v1" and H.equal(p["entry"], ref)
           and H.equal(p["activities"], [ref]), "exact lowered root")
    fields = H.FIELD.findall(sources[SOURCES[1][1]].decode("utf-8"))
    H.need(fields == [(name, OID + "/" + suffix, kind) for name, suffix, kind in FIELD_SPEC], "ten fixed source field identities/types")
    H.need(type(expected) is list and len(rows) == len(expected), "complete independent oracle population")
    for value in expected:
        H.need(type(value) is dict and set(value) == {name for name, _, _ in fields}, "exact oracle fields")
        for name, _, kind in fields:
            H.need(type(value[name]) is {"string": str, "integer": int, "boolean": bool}[kind], "oracle scalar type")
    H.need(rt["schema"] == "gooo/body-composition-runtime/v1" and rt["stage"] == "COMPLETE"
           and not rt.get("failure") and H.equal([rt["finite_total"], rt["finite_passed"], rt["model_calls"]], [0, 0, 0])
           and rt["runtime_replayed"] is True and rt["projection_replayed"] is True, "complete input-only native runtime")
    H.need(rt["producer_source_sha"] == REV and rt["go_version"] == go_version
           and rt["go_tool_selection"] == "explicit" and rt["go_tool_sha256"] == go_sha, "actual pinned producer/Go")
    H.need(rt["producer_module"]["path"] == "github.com/kimjooyoon/meta-ontology-go"
           and type(rt["producer_module"]["version"]) is str and rt["producer_module"]["version"], "producer module identity")
    for key in ("observed_projection_sha256", "observed_driver_sha256", "runtime_suite_sha256", "executable_sha256", "composition_sha256"):
        H.need(type(rt[key]) is str and re.fullmatch(r"sha256:[0-9a-f]{64}", rt[key]) is not None, "native digest syntax: " + key)
    H.need(len(rt["runs"]) == 2, "two fresh compiled runs")
    for process in [rt["toolchain"], rt["build"], *rt["runs"]]:
        H.need(process["started"] is True and process["completed"] is True
               and type(process["exit_code"]) is int and process["exit_code"] == 0
               and not any(process[k] for k in ("canceled", "timed_out", "output_truncated")), "joined successful native process")
    image = p["workspace_image"]
    seen = [(package["path"], source["filename"], source["source_digest"])
            for package in image["packages"] for source in package["sources"]]
    wanted = [(package, filename, H.digest(H.go_json(sources[filename].decode("utf-8")))) for package, filename in SOURCES]
    H.need(image["schema"] == "gooo/package-runtime-image/v1" and seen == wanted
           and len(image["packages"]) == 2 and image["init_order"] == [x[0] for x in SOURCES], "exact current source closure")
    H.need(H.equal(image["entry"], {"package_path": PACKAGE, "source": SOURCES[1][1],
                                   "activity": "Report", "inputs": ["Text"], "output": "URLContextFacts"}), "single-Text source ABI")
    lowered = p["lowered_gooo_source"]
    actual = H.activities(lowered)
    H.need(len(actual) == 4, "four lowered bodies")
    for d in defs.values():
        found = [a for a in actual if a["name"] == d["lowered"]]
        H.need(len(found) == 1 and all(H.equal(found[0][k], d[k]) for k in ("id", "inputs", "output"))
               and found[0]["body"] == d["lowered_body"], "source-resolved body/signature")
    pure = p["pure_calls"]
    H.need(pure["schema"] == "gooo/workspace-pure-calls/v1" and len(pure["activities"]) == 4, "pure participant closure")
    for d in defs.values():
        found = [a for a in pure["activities"] if a["activity"] == {"package_path": d["package"], "activity": d["name"], "lowered_name": d["lowered"]}]
        H.need(len(found) == 1 and found[0]["source"] == d["filename"]
               and found[0]["source_body_sha256"] == H.digest(d["body"].encode()), "source pure body identity")
    wanted_sites = []
    for caller, callee, surface in [(ENTRY, "go/net/url:PathEscape", "url.PathEscape"),
                                     (ENTRY, "go/net/url:QueryEscape", "url.QueryEscape"),
                                     ("go/net/url:PathEscape", "go/net/url:EscapeComponent", "EscapeComponent"),
                                     ("go/net/url:QueryEscape", "go/net/url:EscapeComponent", "EscapeComponent")]:
        body = defs[caller]["body"]
        start = body.index(surface)
        wanted_sites.append({"caller": caller, "callee": callee, "surface": "computes",
                             "start_byte": len(body[:start].encode()), "end_byte": len(body[:start + len(surface)].encode())})
    H.need(H.equal(pure["sites"], wanted_sites), "four original source call sites")
    H.need(c["schema"] == "gooo/body-composition/v1" and c["stage"] == "COMPLETE", "complete generated composition")
    H.need(H.digest(c["source"].encode()) == c["generated_sha256"] == rt["generated_sha256"]
           and H.digest(c["driver_source"].encode()) == c["driver_sha256"] == rt["driver_sha256"], "actual generated Go/driver binding")
    H.need(lowered == c["gooo_source"] and H.digest(lowered.encode()) == result["execution_source_sha256"]
           == c["original_source_sha256"] == c["selected_source_sha256"]
           == rt["original_source_sha256"] == rt["selected_source_sha256"], "actual lowered source binding")
    plan = c["plan"]
    record = {"name": "URLContextFacts", "id": OID, "go_name": "GoooRecord" + H.sha(OID.encode()),
              "fields": [{"name": name, "id": ident, "type_id": "urn:gooo:type:" + kind,
                          "go_name": "GoooField" + H.sha(ident.encode())} for name, ident, kind in fields]}
    H.need(plan["schema"] == "gooo/body-composition-plan/v1" and plan["entry_activity"] == root["lowered"]
           and H.equal(plan["record_types"], [record]) and H.equal(plan["edges"], [])
           and ("preparations" not in plan or H.equal(plan["preparations"], []))
           and ("preparations" not in c or H.equal(c["preparations"], []))
           and len(plan["activities"]) == 1, "single computes root and exact typed record")
    activity = plan["activities"][0]
    H.need(all(H.equal(activity[k], value) for k, value in {
        "name": root["lowered"], "id": AID, "input_type": "Text", "output_type": "URLContextFacts",
        "input_entity_id": "gooo://std/text", "output_entity_id": OID,
        "go_function": "GoooComposedActivity0", "input_from": -1, "assembling": False}.items()), "typed root ABI")
    H.need(plan["typed_plan_sha256"] == rt["typed_plan_sha256"], "same typed plan and native runtime")
    H.need(len(c["steps"]) == 1, "one source generation")
    step = c["steps"][0]
    generation, report = step["generation"], step["generation"]["report"]
    H.need(report["schema"] == "gooo/body-codegen-report/v3" and report["decision"] == "PASS"
           and report["activity"] == root["lowered"] and report["activity_id"] == AID
           and report["compiler_source_sha"] == REV and report["typecheck_passed"] is True
           and report["deterministic_replay"] is True and H.equal(report["repository_writes"], 0), "source generation identity")
    H.need(step["input_source_sha256"] == report["source_digest"] == H.digest(lowered.encode())
           and report["program_digest"] == H.digest(root["lowered_body"].encode())
           and report["generated_digest"] == report["replay_digest"] == H.digest(generation["source"].encode())
           and H.equal(report["record_types"], [record])
           and report["input_type"] == "string" and report["output_type"] == "URLContextFacts", "generation body/output/record ABI")
    closure = report["call_closure"]
    callees = [{"name": d["lowered"], "activity_id": d["id"], "program_sha256": H.digest(d["lowered_body"].encode())}
               for key, d in defs.items() if key != ENTRY]
    H.need(closure["schema"] == "gooo/pure-activity-call-closure/v1"
           and H.equal(closure["activities"], callees) and H.equal(closure["max_calls_per_invocation"], 4), "three source-bound callees/four calls")
    edges = [{"caller_id": defs[caller]["id"], "callee_id": defs[callee]["id"], "ordinal": ordinal}
             for caller, callee, ordinal in [(ENTRY, "go/net/url:PathEscape", 0),
                (ENTRY, "go/net/url:QueryEscape", 1), ("go/net/url:PathEscape", "go/net/url:EscapeComponent", 0),
                ("go/net/url:QueryEscape", "go/net/url:EscapeComponent", 0)]]
    key = lambda edge: (edge["caller_id"], edge["ordinal"], edge["callee_id"])
    H.need(H.equal(sorted(closure["edges"], key=key), sorted(edges, key=key)), "exact typed callee edges/ordinals")
    if parent is not None:
        H.need(H.equal(program, p) and receipt["replayed_from_sha256"] == H.digest(parent)
               and H.equal(result["replay"]["model_calls"], 0), "same program and raw saved parent")
        prior_runtime = H.load(parent)["result"]["runtime"]
        H.need(all(H.equal(rt[key], prior_runtime[key]) for key in
                   ("observed_projection_sha256", "observed_driver_sha256", "runtime_suite_sha256", "producer_module")),
               "saved observed source/driver/suite/producer identity")
    H.need(len(rt["traces"]) == len(rows), "all finite inputs")
    for index, (trace, row, value) in enumerate(zip(rt["traces"], rows, expected)):
        H.need(type(trace["case_index"]) is int and trace["case_index"] == index
               and len(trace["deliveries"]) == 1, "ordered root")
        delivery = trace["deliveries"][0]
        H.need(set(delivery) == {"activity_id", "input", "actual", "actual_fields"}, "single-input trace schema")
        H.need(delivery["activity_id"] == AID and type(delivery["input"]) is str
               and delivery["input"] == row[ENTRY], "original single typed input")
        H.need(H.equal(delivery["actual"], value), "all fields equal independent Go oracle")
        ordered = [{"id": ident, "name": name, "value": value[name]} for name, ident, kind in fields]
        H.need(H.equal(delivery["actual_fields"], ordered), "ordered field identities/types/values")
    H.need(all(key in rt for key in ("observed_projection_sha256", "observed_driver_sha256", "fault_sites", "outcomes")), "observed arithmetic driver")
    H.need(H.equal(rt["outcomes"], {name: 0 for name in ("matched", "mismatched", "faulted", "blocked", "unobserved")}), "input-only counters")
    H.need([(site["root_activity_id"], site["activity_id"], site["operator"], site["expression"])
            for site in rt["fault_sites"]] == [(AID, HELPER, "/", "octet / 16"), (AID, HELPER, "%", "octet % 16")], "two constant-divisor arithmetic sites")
    projection = c["source"].encode("utf-8")
    for site in rt["fault_sites"]:
        start, end = site["start"], site["end"]
        H.need(site["projection_sha256"] == c["generated_sha256"]
               and type(start) is int and type(end) is int and 0 <= start < end <= len(projection)
               and projection[start:end] == site["expression"].encode("utf-8")
               and type(site["expression_id"]) is str
               and re.fullmatch(r"sha256:[0-9a-f]{64}", site["expression_id"]) is not None,
               "arithmetic site projection and original byte position")
    # The pinned arithmetic driver emits this struct in this field order.
    # No Preparations means no called-input observers, despite four pure calls.
    stream = {"schema": "gooo/arithmetic-outcome-observation/v1",
              "outputs": [[{name: value[name] for name, _, _ in fields}] for value in expected],
              "faults": [[None] for _ in rows], "blocked": [[None] for _ in rows], "calls": []}
    text = json.dumps(stream, ensure_ascii=False, separators=(",", ":"), allow_nan=False)
    for old, new in (("&", "\\u0026"), ("<", "\\u003c"), (">", "\\u003e"), ("\u2028", "\\u2028"), ("\u2029", "\\u2029")):
        text = text.replace(old, new)
    H.need(all(run["stdout_sha256"] == H.digest(text.encode() + b"\n") for run in rt["runs"]), "both actual native stdout hashes")
    return p


def negative_controls(original, replay, arguments, program):
    """Mutate fresh successful observations, without re-executing a producer."""
    def change(path, value):
        def mutate(document):
            target = document
            for part in path[:-1]:
                target = target[part]
            target[path[-1]] = value
        return mutate
    r = ["result", "runtime"]
    p = ["result", "program"]
    c = ["result", "composition"]
    d = r + ["traces", 0, "deliveries", 0]
    mutations = [
        ("false-pass-envelope", change(["decision"], "PASS")),
        ("wrong-manifest", change(["manifest_digest"], "sha256:" + "0" * 64)),
        ("wrong-input-document", change(["inputs_digest"], "sha256:" + "0" * 64)),
        ("wrong-producer", change(r + ["producer_source_sha"], "0" * 40)),
        ("wrong-go-tool", change(r + ["go_tool_sha256"], "sha256:" + "0" * 64)),
        ("wrong-root-entry", change(p + ["entry", "package_path"], "other")),
        ("wrong-source-image", change(p + ["workspace_image", "packages", 0, "sources", 0, "source_digest"], "sha256:" + "0" * 64)),
        ("wrong-typed-plan", change(r + ["typed_plan_sha256"], "sha256:" + "0" * 64)),
        ("wrong-generated-source", change(c + ["source"], "package main\n")),
        ("wrong-field-type", change(c + ["plan", "record_types", 0, "fields", 0, "type_id"], "urn:gooo:type:boolean")),
        ("wrong-pure-call-body", change(p + ["pure_calls", "activities", 0, "source_body_sha256"], "sha256:" + "0" * 64)),
        ("missing-typed-callee", change(c + ["steps", 0, "generation", "report", "call_closure", "activities"], [])),
        ("no-native-traces", change(r + ["traces"], [])),
        ("boolean-case-index", change(r + ["traces", 0, "case_index"], False)),
        ("changed-original-input", change(d + ["input"], "different")),
        ("boolean-integer-result", change(d + ["actual", "input_bytes"], False)),
        ("wrong-field-identity", change(d + ["actual_fields", 0, "id"], "other")),
        ("wrong-native-stdout", change(r + ["runs", 0, "stdout_sha256"], "sha256:" + "0" * 64)),
        ("nonzero-native-exit", change(r + ["runs", 0, "exit_code"], 1)),
        ("unjoined-native-process", change(r + ["runs", 0, "completed"], False)),
        ("unexpected-model-call", change(r + ["model_calls"], 1)),
        ("wrong-producer-module", change(r + ["producer_module", "path"], "other")),
        ("malformed-observed-driver", change(r + ["observed_driver_sha256"], None)),
        ("wrong-site-projection", change(r + ["fault_sites", 0, "projection_sha256"], "sha256:" + "0" * 64)),
        ("wrong-site-position", change(r + ["fault_sites", 0, "start"], 0)),
        ("wrong-preparations-type", change(c + ["plan", "preparations"], False)),
    ]
    accepted = []
    for label, mutate in mutations:
        document = copy.deepcopy(H.load(original))
        mutate(document)
        try:
            validate(json.dumps(document).encode(), *arguments)
        except (RuntimeError, KeyError, TypeError, ValueError):
            accepted.append(label)
        else:
            raise RuntimeError("negative control was accepted: " + label)
    document = copy.deepcopy(H.load(replay))
    document["replayed_from_sha256"] = "sha256:" + "0" * 64
    try:
        validate(json.dumps(document).encode(), *arguments, original, program)
    except (RuntimeError, KeyError, TypeError, ValueError):
        accepted.append("wrong-raw-saved-parent")
    else:
        raise RuntimeError("wrong saved parent accepted")
    return accepted


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("root", "compiler", "go", "out"):
        parser.add_argument("--" + name, required=True)
    parser.add_argument("--inputs", help="optional strict public input-only corpus; default: checked-in 128 rows")
    args = parser.parse_args(argv)
    root, compiler, go, out = [H.canonical(v) for v in (args.root, args.compiler, args.go, args.out)]
    H.need(not out.exists() and root.is_dir(), "fresh output and existing root required")
    out.mkdir(parents=True)
    pins = {}
    read = lambda path: H.frozen_read(path, pins)
    summary = {"schema": "gooo/url-component-verification/v1", "status": "NOT_ACCEPTED",
               "catalog_extended": False, "finite_total": 0, "model_calls": 0}
    runner = work = owner = None
    failure = None
    try:
        H.need(shutil.disk_usage(out).free >= 5 << 30, "5GiB free-space guard")
        for name in OWN + [name for _, name in SOURCES]:
            read(root / name)
        read(compiler)
        go_sha = H.digest(read(go))
        input_path = H.canonical(args.inputs) if args.inputs else root / "examples/url-context-facts/inputs.json"
        raw_inputs = bounded_input(input_path)
        rows = inputs(raw_inputs)
        snapshot = out / "inputs.json"
        with snapshot.open("xb") as stream:
            stream.write(raw_inputs)
        read(snapshot)
        sources = {name: read(root / name) for _, name in SOURCES}
        workspace = root / WORKSPACE
        manifest = read(workspace)
        H.need(H.equal(H.load(manifest), {"schema": "gooo/package-workspace-manifest/v1",
            "entry": {"package_path": PACKAGE, "activity": "Report"},
            "packages": [{"path": package, "sources": [name]} for package, name in SOURCES]}), "fixed public source manifest")
        work = Path(tempfile.mkdtemp(prefix="owned-work-", dir=out))
        owner = work.lstat()
        (work / "tmp").mkdir()
        env = {k: os.environ[k] for k in ("PATH", "HOME", "SYSTEMROOT") if k in os.environ}
        env.update(GOTOOLCHAIN="local", GOWORK="off", GOENV="off", GOPROXY="off", GOSUMDB="off",
                   GOFLAGS="-mod=readonly", TMPDIR=str(work / "tmp"), CGO_ENABLED="0",
                   GOOO_LAYA_URL="", GOOO_LAYA_API_KEY="")
        runner = H.OwnedCommands(out, work, root, pins, env)
        go_version = runner.call("go-version", [go, "version"], 15).decode().strip()
        H.need(re.fullmatch(r"go version go1\.27\.3 (darwin/arm64|linux/amd64)", go_version) is not None, "pinned supported Go version")
        build = H.load(runner.call("compiler-identity", [compiler, "version", "--build", "--json"], 15))
        H.need(all(build[k] == v for k, v in {"version": "0.6.37-dev", "compiler_source_sha": REV,
            "source_status": "CLEAN_VCS", "vcs_modified": "false", "go_version": "go1.27.3", "native_go_required": "go1.27.3"}.items()), "clean pinned compiler build")
        expected = H.load(runner.call("independent-go-oracle", [go, "run", "./cmd/url-component-oracle", "--inputs", snapshot], 90))
        original = runner.call("execute", [compiler, "package", "execute", "--json", "--inputs", snapshot, "--go", go, workspace], 90)
        program = validate(original, rows, expected, manifest, raw_inputs, sources, go_version, go_sha, workspace)
        replay = runner.call("saved-replay", [compiler, "package", "replay", "--json", "--receipt", out / "execute/stdout",
            "--inputs", snapshot, "--go", go, workspace], 90)
        validate(replay, rows, expected, manifest, raw_inputs, sources, go_version, go_sha, workspace, original, program)
        controls = negative_controls(original, replay,
            (rows, expected, manifest, raw_inputs, sources, go_version, go_sha, workspace), program)
        H.unchanged(pins)
        H.need(bounded_input(input_path) == raw_inputs, "original input source unchanged")
        summary.update(status="NATIVE_ORACLE_AND_SAVED_REPLAY_MATCH", input_rows=len(rows),
                       unique_inputs=len({r[ENTRY] for r in rows}), typed_fields_per_layer=len(rows) * 10,
                       native_runs_per_layer=2, compiler_revision=REV, go_version=go_version,
                       execute=H.reference(original), replay=H.reference(replay),
                       oracle=H.reference((out / "independent-go-oracle/stdout").read_bytes()),
                       input_path=str(input_path), input=H.reference(raw_inputs),
                       negative_controls=controls,
                       scope="Finite Report observations call both Gooo port bodies; no separate scalar-entry observation, full net/url, arbitrary-byte/unbounded strings, authorization, production safety or performance claim. Inner canonical parent and typed-plan digest recomputation remain NOT_CHECKED; raw saved parent and plan equality are checked.")
    except BaseException as error:
        failure = str(error)
        summary.update(status="FIRST_FAILURE_PRESERVED_NOT_ACCEPTED", failure=failure)
    finally:
        summary.update(source_pins=pins, commands=runner.observations if runner else [],
                       processes_closed=runner.closed if runner else True, owned_work_removed=work is None)
        if work is not None and runner is not None and runner.closed:
            H.need(not work.is_symlink() and os.path.samestat(owner, work.lstat()), "owned temporary identity")
            summary["temporary_file_bytes_before_cleanup"] = H.owned_bytes(work)
            shutil.rmtree(work)
            summary["owned_work_removed"] = not work.exists()
        H.save(out / "summary.json", summary)
    print(json.dumps({"status": summary["status"], "summary": str(out / "summary.json"), "failure": failure}))
    return 1 if failure else 0


if __name__ == "__main__":
    raise SystemExit(main())
