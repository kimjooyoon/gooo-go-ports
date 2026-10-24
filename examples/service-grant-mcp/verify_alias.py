"""Finite stdlib MCP comparison against an already-built pinned alias CLI.

This reuses the legacy raw JSON-RPC client contract and owned-process cleanup
pattern. It does not generate a kernel, build a compiler, download a dependency,
or establish live server identity, effect authority, or external adoption.
"""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import selectors
import signal
import subprocess
import time

MODULE = "github.com/kimjooyoon/gooo-go-ports/examples/alias-origin-change"
VERSION = "v0.0.0-20261024154510-16d1e7864dc8"
MODULE_SUM = "h1:YCIMYxnE7+rqIumnIdG0e5JCkYOnaoLIHMCUJbdTKQA="
GENERATION_SHA256 = "b0c87b7865697c4b6433ef54e23defd7267ed8ea4da562cc8e1af24e283e7b06"
TOOL = "gooo_compare_alias_origin"
FLAG = "--alias-origin-change"
PROTOCOLS = ("2025-11-25", "2026-07-28")
ARGUMENT_LIMIT = 1 << 20
RESPONSE_LIMIT = (16 << 20) + (64 << 10)
STREAM_LIMIT = 64 << 20
PENDING_SIGNAL = None


class WireCancelled(BaseException):
    pass


def checkpoint(deadline=None):
    if PENDING_SIGNAL is not None:
        raise WireCancelled("wire client interrupted: " + str(PENDING_SIGNAL))
    if deadline is not None:
        assert time.monotonic() < deadline, "180-second trial deadline"


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def reference(path):
    raw = path.read_bytes()
    return {"path": str(path), "bytes": len(raw), "sha256": sha(raw)}


def save(path, value):
    path.write_text(json.dumps(value, indent=2, ensure_ascii=True) + "\n")


def lossless_json(raw):
    def unique(pairs):
        result = {}
        for name, value in pairs:
            assert name not in result, "duplicate response JSON member"
            result[name] = value
        return result

    def nonfinite(_):
        raise AssertionError("nonfinite response JSON number")

    return json.loads(raw, object_pairs_hook=unique, parse_constant=nonfinite)


def trial_inputs(module, encoded, scope="full"):
    """Small source example plus reproducible in-memory boundary cases."""
    example = (module / "example-input.json").read_bytes().rstrip(b"\r\n")
    base = lossless_json(example)
    unchanged = {**base, "after": base["before"]}
    ambiguous = {**base, "after": base["after"] + base["after"]}
    invalid = {**base, "target_alias": ""}
    maximum = b"{" + b" " * (ARGUMENT_LIMIT - len(example)) + example[1:]
    assert len(maximum) == ARGUMENT_LIMIT

    def expanded_report(rows):
        catalog = [{"server": "<" * 1024, "original_name": "<" * 1024,
                    "exposed_name": "tool-" + str(index)} for index in range(rows)]
        raw = encoded({"target_alias": "tool-0", "before": catalog, "after": catalog})
        assert len(raw) < ARGUMENT_LIMIT
        return raw

    cases = [("origin-changed", example, "ORIGIN_CHANGED"),
             ("unchanged", encoded(unchanged), "UNCHANGED"),
             ("ambiguous", encoded(ambiguous), "AMBIGUOUS"),
             ("invalid-input", encoded(invalid), "INVALID_INPUT"),
             ("argument-byte-limit", maximum, "ORIGIN_CHANGED"),
             ("large-accepted-report", expanded_report(160), "UNCHANGED")]
    row = base["before"][0]
    malformed = [
        ("duplicate-key", b'{"target_alias":"docs_search","target_alias":"other","before":[],"after":[]}'),
        ("null-catalog", b'{"target_alias":"docs_search","before":null,"after":[]}'),
        ("unpaired-surrogate", b'{"target_alias":"\\ud800","before":[],"after":[]}'),
        ("collection257", encoded({**base, "before": [row] * 257})),
        ("text-utf8-bytes1025", json.dumps({**base, "target_alias": "é" * 512 + "x"}, ensure_ascii=False).encode()),
        ("argument-byte-limit-plus-one", maximum[:-1] + b" }"),
        ("report-byte-limit", expanded_report(180))]
    if scope == "bundle-small":
        cases = cases[:4]
        malformed = malformed[:5]
    return cases, malformed


def load_legacy(path):
    spec = importlib.util.spec_from_file_location("alias_legacy_wire_client", path)
    legacy = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(legacy)
    return legacy


def saved_metadata(path):
    assert path.is_file() and not path.is_symlink()
    assert path.stat().st_size <= 2 << 20, "saved metadata byte limit"
    return lossless_json(path.read_bytes())


def check_diagnostic(value, schema):
    assert set(value) == set(schema["required"])
    for branch in schema["oneOf"]:
        properties = branch["properties"]
        if all(value[key] == rule["const"] if "const" in rule
               else value[key] in rule["enum"] for key, rule in properties.items()):
            return
    raise AssertionError("diagnostic is outside the pinned schema")


def check_proof(response, expected, raw, exact):
    assert "error" not in response, response
    result = response["result"]
    assert result.get("isError", False) is False, result
    exact(result["structuredContent"], expected)
    assert len(result["content"]) == 1 and set(result["content"][0]) == {"type", "text"}
    assert result["content"][0]["type"] == "text"
    exact(lossless_json(result["content"][0]["text"]), expected)
    assert result["structuredContent"]["input_sha256"] == "sha256:" + sha(raw)
    assert result["structuredContent"]["decision"]["execution_authorized"] is False


def check_refusal(response, diagnostic, schema, exact):
    if "error" in response:
        assert set(response) == {"jsonrpc", "id", "error"}
        error = response["error"]
        assert type(error.get("code")) is int and error["code"] == -32602
        assert set(error) <= {"code", "message"} and isinstance(error.get("message"), str)
        assert error["message"]
        return "MCP_INVALID_PARAMS"
    result = response["result"]
    assert result.get("isError") is True
    assert set(result) <= {"isError", "content", "_meta", "resultType"}
    assert "structuredContent" not in result
    assert len(result["content"]) == 1 and set(result["content"][0]) == {"type", "text"}
    assert result["content"][0]["type"] == "text"
    actual = lossless_json(result["content"][0]["text"])
    check_diagnostic(actual, schema)
    exact(actual, diagnostic)
    return actual["code"]


def validate_saved(out, binary, alias_cli, alias_module, go, legacy_verifier, expected_scope="full"):
    """Recheck retained raw evidence and fixed cases without starting processes.

    Callers supply the expected artifact paths rather than following paths from
    an untrusted report. A returned report establishes finite byte correspondence,
    not authenticity of an unsigned producer or permission to invoke a tool.
    """
    assert expected_scope in {"full", "bundle-small"}
    paths = [Path(item) for item in (out, binary, alias_cli, alias_module, go, legacy_verifier)]
    assert all(path.is_absolute() for path in paths)
    out, binary, alias_cli, alias_module, go, legacy_verifier = paths
    legacy = load_legacy(legacy_verifier)
    report = saved_metadata(out / "acceptance.json")
    assert report["schema"] == "gooo/alias-origin-mcp-wire-verification/v1"
    assert report["status"] == "INTERNAL_ALIAS_MCP_TWO_PROTOCOL_COMPARISON_PASS"
    assert report["scope"] == expected_scope and report["first_failure"] is None
    assert report["all_direct_wait_EOF_PG_ESRCH"] is True
    assert report["actual_protocol_sessions"] == 8
    for name, path in (("binary", binary), ("alias_cli", alias_cli),
                       ("go_identity_reader", go), ("legacy_verifier", legacy_verifier)):
        actual = reference(path)
        assert all(report[name][key] == actual[key] for key in ("bytes", "sha256"))
    module = report["alias_module"]
    assert module["module"] == MODULE and module["version"] == VERSION
    generation_raw = (alias_module / "generation.json").read_bytes()
    assert sha(generation_raw) == GENERATION_SHA256
    generation = lossless_json(generation_raw)
    schemas = {name: lossless_json((alias_module / "schemas" / (name + ".schema.json")).read_bytes())
               for name in ("input", "output", "diagnostic")}
    for name, path in (("generation", alias_module / "generation.json"), ("go_mod", alias_module / "go.mod")):
        actual = reference(path)
        assert all(module[name][key] == actual[key] for key in ("bytes", "sha256"))
    for name in schemas:
        actual = reference(alias_module / "schemas" / (name + ".schema.json"))
        assert all(module["schemas"][name][key] == actual[key] for key in ("bytes", "sha256"))
    cases, malformed = trial_inputs(alias_module, legacy.encoded, expected_scope)
    legacy.exact(report["executed_cases"], {"reference_normal": [row[0] for row in cases],
        "reference_rejected": [row[0] for row in malformed],
        "large_reports_exercised": expected_scope == "full"})
    records = {row["label"]: row for row in report["processes"]}
    assert len(records) == len(report["processes"])
    assert len(records) == 2 + len(cases) + len(malformed) + 8
    total_stdout = 0

    def capture(label, channel):
        nonlocal total_stdout
        assert Path(label).name == label
        row = records[label]
        assert row["waited"] is True and row["streams_EOF"] is True and row["process_group_ESRCH"] is True
        assert row["first_failure"] is None and not row["secondary_cleanup_errors"]
        assert row["runtime_PATH_empty"] is True and row["runtime_GOROOT_absent"] is True
        assert row["server_created_work_files"] == []
        legacy.exact(saved_metadata(out / label / "process.json"), row)
        path = out / label / channel
        assert path.is_file() and not path.is_symlink()
        limit = ((512 << 10) if expected_scope == "bundle-small" else RESPONSE_LIMIT) if channel == "stdout" else STREAM_LIMIT
        assert path.stat().st_size <= limit
        raw = path.read_bytes()
        assert row[channel]["bytes"] == len(raw) and row[channel]["sha256"] == sha(raw)
        if channel == "stdout":
            total_stdout += len(raw)
        return raw

    for key in ("server", "alias_cli"):
        label = key + "-build-identity"
        raw = capture(label, "stdout")
        assert capture(label, "stdin") == b"" and capture(label, "stderr") == b""
        assert records[label]["exit_code"] == 0 and records[label]["protocol"] is None
        artifact_key = "binary" if key == "server" else "alias_cli"
        assert records[label]["argv"] == [report["go_identity_reader"]["path"], "version", "-m", "-json", report[artifact_key]["path"]]
        legacy.exact(lossless_json(raw), report["build_identities"][key])
    build = report["build_identities"]["server"]
    assert build["Path"] == MODULE.rsplit("/", 1)[0] + "/service-grant-mcp"
    dependency = next(row for row in build["Deps"] if row["Path"] == MODULE)
    assert dependency["Version"] == VERSION and dependency["Sum"] == MODULE_SUM
    cli_build = report["build_identities"]["alias_cli"]
    assert cli_build["Path"] == MODULE + "/cmd/compare" and cli_build["Main"]["Path"] == MODULE
    assert cli_build["Main"]["Version"] == VERSION and cli_build["Main"]["Sum"] == MODULE_SUM
    for identity in (build, cli_build):
        assert identity["GoVersion"] == "go1.27.3" and "Replace" not in identity["Main"]
        assert all("Replace" not in row for row in identity.get("Deps", []))

    expected, diagnostics = {}, {}
    for name, raw, state in cases:
        label = "reference-" + name
        assert records[label]["argv"] == [report["alias_cli"]["path"]]
        assert capture(label, "stdin") == raw and capture(label, "stderr") == b""
        assert records[label]["exit_code"] == 0
        output = capture(label, "stdout")
        if name == "large-accepted-report":
            assert 3 << 20 < len(output) - 1 <= 4 << 20
        value = lossless_json(output)
        assert value["decision"]["state"] == state
        assert value["input_sha256"] == "sha256:" + sha(raw)
        legacy.exact(value["facts"], lossless_json(raw))
        legacy.exact(value["bindings"], generation)
        expected[name] = (raw, value)
    for name, raw in malformed:
        label = "reference-" + name
        assert records[label]["argv"] == [report["alias_cli"]["path"]]
        assert capture(label, "stdin") == raw and capture(label, "stdout") == b""
        assert records[label]["exit_code"] == 1
        value = lossless_json(capture(label, "stderr"))
        check_diagnostic(value, schemas["diagnostic"])
        assert value["state"] == "INVALID_TRANSPORT"
        if name == "report-byte-limit":
            assert value["stage"] == "encode_result" and value["code"] == "OUTPUT_LIMIT_EXCEEDED"
        diagnostics[name] = (raw, value)

    observations = {(row["protocol"], row["mode"]): row for row in report["observations"]}
    assert len(observations) == len(report["observations"]) == 8
    maximum_response = 0
    modes = {"default": ([], {legacy.TOOL}),
        "many": (["--many-required"], {legacy.TOOL, "assess_service_grant_change"}),
        "alias": ([FLAG], {legacy.TOOL, TOOL}),
        "both": (["--many-required", FLAG], {legacy.TOOL, "assess_service_grant_change", TOOL})}
    for protocol in PROTOCOLS:
        for mode, (flags, names) in modes.items():
            label = mode + "-" + protocol
            record, observation = records[label], observations[(protocol, mode)]
            assert record["protocol"] == protocol and record["exit_code"] == 0
            assert record["argv"] == [report["binary"]["path"], *flags] and observation["argv"] == flags
            assert observation["tool_count"] == len(names) and set(observation["tool_names"]) == names
            incoming, outgoing = capture(label, "stdin"), capture(label, "stdout")
            assert capture(label, "stderr") == b""
            assert incoming.endswith(b"\n") and outgoing.endswith(b"\n")
            requests = []
            for raw in incoming.splitlines():
                # Deliberately malformed argument objects must reach the native
                # parser. Validate the RPC envelope separately from those bytes.
                if b'"arguments":' in raw:
                    prefix, tail = raw.split(b'"arguments":', 1)
                    assert tail.endswith(b"}}")
                    value = lossless_json(prefix + b'"arguments":{}}}')
                else:
                    value = lossless_json(raw)
                if "id" in value:
                    requests.append((raw, value))
                else:
                    assert protocol == "2025-11-25" and value["method"] == "notifications/initialized"
            raw_responses = outgoing.splitlines()
            responses = [lossless_json(raw) for raw in raw_responses]
            assert len(responses) == len(requests) == len(record["response_frames_including_newline"])
            responses_by_id = {value["id"]: value for value in responses}
            raw_responses_by_id = {value["id"]: raw for value, raw in zip(responses, raw_responses)}
            assert len(responses_by_id) == len(responses)
            frames = []
            call_index = 0
            proof_summaries, refusal_summaries = [], []
            selected = [row for row in cases if mode == "alias" or row[0] != "large-accepted-report"]
            rejected = [row for row in malformed if mode == "alias" or row[0] != "report-byte-limit"]
            wanted_calls = [(name, raw, True) for name, raw, _ in selected]
            wanted_calls += [(name, raw, False) for name, raw in rejected]
            wanted_calls += [(cases[0][0], cases[0][1], True)]
            expected_methods = ["initialize" if protocol == "2025-11-25" else "server/discover", "tools/list"]
            expected_methods += ["tools/call"] * (len(wanted_calls) if TOOL in names else 0)
            assert [value["method"] for _, value in requests] == expected_methods
            assert [value["id"] for _, value in requests] == list(range(1, len(requests) + 1))
            for raw_response in raw_responses:
                maximum_response = max(maximum_response, len(raw_response) + 1)
                assert len(raw_response) + 1 <= RESPONSE_LIMIT
            for raw_request, request in requests:
                response = responses_by_id[request["id"]]
                assert response["jsonrpc"] == request["jsonrpc"] == "2.0"
                raw_response = raw_responses_by_id[request["id"]]
                frames.append({"id": request["id"], "method": request["method"], "bytes": len(raw_response) + 1})
                if request["method"] == "tools/list":
                    listed = response["result"]["tools"]
                    assert len(listed) == len(names) and {tool["name"] for tool in listed} == names
                    if TOOL in names:
                        tool = next(tool for tool in listed if tool["name"] == TOOL)
                        legacy.exact(tool["inputSchema"], schemas["input"])
                        legacy.exact(tool["outputSchema"], schemas["output"])
                        assert tool["annotations"]["readOnlyHint"] is True and tool["annotations"]["openWorldHint"] is False
                elif request["method"] == "tools/call":
                    assert TOOL in names and request["params"]["name"] == TOOL
                    name, expected_raw, success = wanted_calls[call_index]
                    call_index += 1
                    assert b'"arguments":' in raw_request and raw_request.endswith(b"}}")
                    actual_raw = raw_request.split(b'"arguments":', 1)[1][:-2]
                    assert actual_raw == expected_raw
                    if success:
                        check_proof(response, expected[name][1], actual_raw, legacy.exact)
                        if call_index <= len(selected):
                            proof_summaries.append({"case": name,
                                "state": expected[name][1]["decision"]["state"],
                                "input_bytes": len(actual_raw), "input_sha256": sha(actual_raw),
                                "full_report_sha256": sha(legacy.encoded(expected[name][1])),
                                "actual_response_bytes": len(raw_response) + 1})
                    else:
                        kind = check_refusal(response, diagnostics[name][1], schemas["diagnostic"], legacy.exact)
                        refusal_summaries.append({"case": name, "input_bytes": len(actual_raw),
                            "input_sha256": sha(actual_raw), "observed_error": kind,
                            "partial_report": False, "actual_response_bytes": len(raw_response) + 1})
                elif request["method"] == "initialize":
                    assert protocol == "2025-11-25" and response["result"]["protocolVersion"] == protocol
                else:
                    assert request["method"] == "server/discover" and protocol in response["result"]["supportedVersions"]
            legacy.exact(frames, record["response_frames_including_newline"])
            assert call_index == (len(wanted_calls) if TOOL in names else 0)
            if TOOL in names:
                assert observation["after_error_repeat_equal"] is True
                legacy.exact(observation["complete_report_comparisons"], proof_summaries)
                legacy.exact(observation["transport_failures_no_partial_report"], refusal_summaries)
    assert report["observed_max_response_bytes"] == maximum_response
    assert report["captured_stdout_bytes"] == total_stdout
    assert report["response_limit_including_jsonrpc_and_newline_bytes"] == RESPONSE_LIMIT
    assert report["argument_limit_bytes"] == ARGUMENT_LIMIT
    assert total_stdout <= ((1 << 20) if expected_scope == "bundle-small" else (32 << 20))
    return report


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", required=True, type=Path)
    parser.add_argument("--alias-cli", required=True, type=Path,
                        help="Already-built cmd/compare from the pinned public module")
    parser.add_argument("--alias-module", required=True, type=Path)
    parser.add_argument("--go", required=True, type=Path,
                        help="Absolute Go executable; used only to inspect build identities")
    parser.add_argument("--legacy-verifier", type=Path,
                        default=Path(__file__).resolve().with_name("verify.py"))
    parser.add_argument("--out", required=True, type=Path)
    parser.add_argument("--scope", choices=("full", "bundle-small"), default="full")
    args = parser.parse_args()
    for path in (args.binary, args.alias_cli, args.go, args.alias_module, args.legacy_verifier, args.out):
        assert path.is_absolute(), "use absolute paths"
    for path in (args.binary, args.alias_cli, args.go, args.legacy_verifier):
        assert path.is_file(), str(path)
    assert args.alias_module.is_dir()
    args.out.mkdir(mode=0o700, exist_ok=False)
    legacy = load_legacy(args.legacy_verifier)
    deadline = time.monotonic() + 180
    processes, observations, expected_cases, malformed_cases = [], [], [], []
    first_failure, identity, schemas = None, {}, {}
    captured_stdout = 0

    class OwnedSession(legacy.Session):
        """Raw-wire legacy client with bounded writes and retained closure."""
        def __init__(self, label, argv, protocol=None):
            self.label, self.protocol = label, protocol
            self.work = args.out / label
            self.work.mkdir(mode=0o700)
            self.argv = [str(item) for item in argv]
            self.env = os.environ.copy()
            self.env.update(PATH="", GOPROXY="off", GOSUMDB="off", GOTELEMETRY="off",
                            GOTOOLCHAIN="local", GOWORK="off", MCPGODEBUG="")
            self.env.pop("GOROOT", None)
            self.selector = selectors.DefaultSelector()
            self.buffers = {"stdout": bytearray(), "stderr": bytearray()}
            self.linebuf, self.sent, self.frames = bytearray(), bytearray(), []
            self.response_bytes = []
            self.next_id, self.started, self.cleaning = 1, time.monotonic(), False
            self.process = None
            try:
                checkpoint(deadline)
                mask = signal.pthread_sigmask(signal.SIG_BLOCK, {signal.SIGINT, signal.SIGTERM})
                try:
                    def restore_child_mask():
                        signal.pthread_sigmask(signal.SIG_SETMASK, mask)
                    self.process = subprocess.Popen(self.argv, cwd=self.work, env=self.env,
                        stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                        start_new_session=True, preexec_fn=restore_child_mask)
                    os.set_blocking(self.process.stdin.fileno(), False)
                    with (args.out / (label + "-started.json")).open("x") as journal:
                        json.dump({"PID": self.process.pid, "PGID": self.process.pid,
                                   "argv": self.argv}, journal)
                        journal.flush()
                        os.fsync(journal.fileno())
                    for name in self.buffers:
                        stream = getattr(self.process, name)
                        os.set_blocking(stream.fileno(), False)
                        self.selector.register(stream, selectors.EVENT_READ, name)
                finally:
                    signal.pthread_sigmask(signal.SIG_SETMASK, mask)
                checkpoint(deadline)
            except BaseException as original:
                try:
                    self.close(repr(original), expected_exit=None)
                except BaseException as cleanup_error:
                    try:
                        save(self.work / "constructor-failure.json", {"first_failure": repr(original),
                            "secondary_cleanup_failure": repr(cleanup_error)})
                    except BaseException:
                        pass
                raise

        def read(self):
            nonlocal captured_stdout
            if not self.cleaning:
                checkpoint(deadline)
                assert time.monotonic() - self.started < 30, "bounded process deadline"
            for key, _ in self.selector.select(0.05):
                raw = os.read(key.fd, 65536)
                if not raw:
                    self.selector.unregister(key.fileobj)
                else:
                    self.buffers[key.data].extend(raw)
                    assert sum(map(len, self.buffers.values())) <= STREAM_LIMIT, "bounded captured streams"
                    if key.data == "stdout":
                        captured_stdout += len(raw)
                        process_cap = (512 << 10) if args.scope == "bundle-small" else RESPONSE_LIMIT
                        total_cap = (1 << 20) if args.scope == "bundle-small" else (32 << 20)
                        assert len(self.buffers["stdout"]) <= process_cap, "per-process stdout limit"
                        assert captured_stdout <= total_cap, "combined stdout limit"
                        self.linebuf.extend(raw)

        def send_frame(self, frame):
            offset = 0
            while offset < len(frame):
                checkpoint(deadline)
                assert time.monotonic() - self.started < 30, "bounded stdin deadline"
                try:
                    count = os.write(self.process.stdin.fileno(), frame[offset:offset + 65536])
                except BlockingIOError:
                    self.read()
                    continue
                assert count > 0, "stdin write made no progress"
                self.sent.extend(frame[offset:offset + count])
                offset += count

        def notify_initialized(self):
            self.send_frame(b'{"jsonrpc":"2.0","method":"notifications/initialized"}\n')

        def request(self, method, params=None, arguments=None):
            checkpoint(deadline)
            current = self.next_id
            self.next_id += 1
            params = dict(params or {})
            if self.protocol == "2026-07-28":
                params["_meta"] = self.metadata()
            wire = legacy.encoded(params)
            if arguments is not None:
                wire = wire[:-1] + (b"," if len(wire) > 2 else b"") + b'"arguments":' + arguments + b"}"
            frame = (b'{"jsonrpc":"2.0","id":' + str(current).encode()
                     + b',"method":' + legacy.encoded(method) + b',"params":' + wire + b"}\n")
            self.send_frame(frame)
            while True:
                checkpoint(deadline)
                while b"\n" in self.linebuf:
                    line, _, rest = self.linebuf.partition(b"\n")
                    self.linebuf[:] = rest
                    assert len(line) + 1 <= RESPONSE_LIMIT, "actual response frame exceeds cap"
                    value = lossless_json(line)
                    assert isinstance(value, dict) and value.get("jsonrpc") == "2.0"
                    self.frames.append(value)
                    if "id" in value:
                        assert type(value["id"]) is int and value["id"] == current
                        self.response_bytes.append({"id": current, "method": method, "bytes": len(line) + 1})
                        return value
                    assert "result" not in value and "error" not in value
                assert self.selector.get_map(), "server closed without response"
                self.read()

        def close(self, failure, expected_exit=0, allow_stderr=False):
            self.cleaning = True
            secondary, waited, drained, absent = [], False, False, False
            cleanup_deadline = time.monotonic() + 3
            process = self.process
            if process is not None:
                try:
                    if not process.stdin.closed:
                        process.stdin.close()
                    while self.selector.get_map() and time.monotonic() < cleanup_deadline:
                        self.read()
                    drained = not self.selector.get_map()
                    process.wait(timeout=max(0.01, cleanup_deadline - time.monotonic()))
                    waited = True
                except BaseException as error:
                    secondary.append(repr(error))
                for number in (signal.SIGTERM, signal.SIGKILL):
                    try:
                        os.killpg(process.pid, number)
                    except ProcessLookupError:
                        break
                    except BaseException as error:
                        secondary.append(repr(error))
                    try:
                        process.wait(timeout=1)
                        waited = True
                    except BaseException as error:
                        secondary.append(repr(error))
                try:
                    process.wait(timeout=1)
                    waited = True
                    while self.selector.get_map() and time.monotonic() < cleanup_deadline + 3:
                        self.read()
                    drained = not self.selector.get_map()
                    try:
                        os.killpg(process.pid, 0)
                    except ProcessLookupError:
                        absent = True
                except BaseException as error:
                    secondary.append(repr(error))
                for stream in (process.stdin, process.stdout, process.stderr):
                    try:
                        stream.close()
                    except BaseException as error:
                        secondary.append(repr(error))
            self.selector.close()
            runtime_files = sorted(item.name for item in self.work.iterdir())
            for name, raw in self.buffers.items():
                (self.work / name).write_bytes(raw)
            (self.work / "stdin").write_bytes(self.sent)
            record = {"label": self.label, "argv": self.argv, "protocol": self.protocol,
                "PID": process.pid if process is not None else None,
                "exit_code": process.returncode if process is not None else None,
                "first_failure": failure or (secondary[0] if secondary else None),
                "secondary_cleanup_errors": secondary, "waited": waited,
                "streams_EOF": drained, "process_group_ESRCH": absent,
                "elapsed_seconds": time.monotonic() - self.started,
                "stdin": reference(self.work / "stdin"),
                "stdout": reference(self.work / "stdout"),
                "stderr": reference(self.work / "stderr"),
                "response_frames_including_newline": self.response_bytes,
                "server_created_work_files": runtime_files,
                "runtime_PATH_empty": True, "runtime_GOROOT_absent": True}
            save(self.work / "process.json", record)
            processes.append(record)
            if failure is None:
                assert not secondary and waited and drained and absent, record
                assert process.returncode == expected_exit, record
                assert allow_stderr or not self.buffers["stderr"], record
                assert not runtime_files, record
                if self.protocol is not None:
                    assert not self.linebuf, "partial JSON-RPC output"

    def command(label, argv, raw=None, expected_exit=0, allow_stderr=False):
        session, failure = OwnedSession(label, argv), None
        try:
            if raw is not None:
                session.send_frame(raw)
            session.process.stdin.close()
            while session.selector.get_map():
                session.read()
            session.process.wait(timeout=2)
            assert session.process.returncode == expected_exit
            return bytes(session.buffers["stdout"]), bytes(session.buffers["stderr"])
        except BaseException as error:
            failure = repr(error)
            raise
        finally:
            session.close(failure, expected_exit, allow_stderr)

    def proof(session, case):
        response = session.request("tools/call", {"name": TOOL}, case["raw"])
        check_proof(response, case["expected"], case["raw"], legacy.exact)
        return {"case": case["name"], "state": case["expected"]["decision"]["state"],
                "input_bytes": len(case["raw"]), "input_sha256": sha(case["raw"]),
                "full_report_sha256": sha(legacy.encoded(case["expected"])),
                "actual_response_bytes": session.response_bytes[-1]["bytes"]}

    def refusal(session, case):
        response = session.request("tools/call", {"name": TOOL}, case["raw"])
        kind = check_refusal(response, case["diagnostic"], schemas["diagnostic"], legacy.exact)
        return {"case": case["name"], "input_bytes": len(case["raw"]),
                "input_sha256": sha(case["raw"]), "observed_error": kind,
                "partial_report": False, "actual_response_bytes": session.response_bytes[-1]["bytes"]}

    try:
        for name in ("input", "output", "diagnostic"):
            schemas[name] = lossless_json((args.alias_module / "schemas" / (name + ".schema.json")).read_bytes())
        generation_raw = (args.alias_module / "generation.json").read_bytes()
        assert sha(generation_raw) == GENERATION_SHA256
        generation = lossless_json(generation_raw)
        assert generation["compiler_source_sha"] == "205896a413e5dea92366309c35ae85e542d055f6"
        assert generation["go_version"] == "go1.27.3"
        for key, binary in (("server", args.binary), ("alias_cli", args.alias_cli)):
            raw, _ = command(key + "-build-identity", [args.go, "version", "-m", "-json", binary])
            build = lossless_json(raw)
            assert build["GoVersion"] == "go1.27.3"
            assert "Replace" not in build["Main"]
            assert all("Replace" not in row for row in build.get("Deps", []))
            identity[key] = build
        assert identity["server"]["Path"] == MODULE.rsplit("/", 1)[0] + "/service-grant-mcp"
        assert identity["alias_cli"]["Path"] == MODULE + "/cmd/compare"
        assert identity["alias_cli"]["Main"]["Path"] == MODULE
        assert identity["alias_cli"]["Main"]["Version"] == VERSION
        assert identity["alias_cli"]["Main"]["Sum"] == MODULE_SUM
        dependency = next(row for row in identity["server"]["Deps"] if row["Path"] == MODULE)
        assert dependency["Version"] == VERSION and dependency["Sum"] == MODULE_SUM

        cases, malformed = trial_inputs(args.alias_module, legacy.encoded, args.scope)
        for name, raw, state in cases:
            stdout, _ = command("reference-" + name, [args.alias_cli], raw)
            if name == "large-accepted-report":
                assert 3 << 20 < len(stdout) - 1 <= 4 << 20
            expected = lossless_json(stdout)
            assert expected["decision"]["state"] == state
            assert expected["input_sha256"] == "sha256:" + sha(raw)
            legacy.exact(expected["facts"], lossless_json(raw))
            legacy.exact(expected["bindings"], generation)
            expected_cases.append({"name": name, "raw": raw, "expected": expected})

        for name, raw in malformed:
            stdout, stderr = command("reference-" + name, [args.alias_cli], raw, 1, True)
            assert stdout == b"", "rejected CLI input returned a partial report"
            diagnostic = lossless_json(stderr)
            check_diagnostic(diagnostic, schemas["diagnostic"])
            assert diagnostic["state"] == "INVALID_TRANSPORT"
            if name == "report-byte-limit":
                assert diagnostic["stage"] == "encode_result" and diagnostic["code"] == "OUTPUT_LIMIT_EXCEEDED"
            malformed_cases.append({"name": name, "raw": raw, "diagnostic": diagnostic})

        modes = [("default", [], {legacy.TOOL}),
                 ("many", ["--many-required"], {legacy.TOOL, "assess_service_grant_change"}),
                 ("alias", [FLAG], {legacy.TOOL, TOOL}),
                 ("both", ["--many-required", FLAG], {legacy.TOOL, "assess_service_grant_change", TOOL})]
        for protocol in PROTOCOLS:
            for mode, flags, names in modes:
                session, failure = OwnedSession(mode + "-" + protocol, [args.binary, *flags], protocol), None
                try:
                    if protocol == "2025-11-25":
                        init = session.request("initialize", {"protocolVersion": protocol,
                            "capabilities": {}, "clientInfo": {"name": "gooo-alias-stdlib-wire-trial", "version": "0"}})
                        assert init["result"]["protocolVersion"] == protocol
                        session.notify_initialized()
                    else:
                        assert protocol in session.request("server/discover")["result"]["supportedVersions"]
                    listed = session.request("tools/list")["result"]["tools"]
                    assert len(listed) == len(names)
                    by_name = {tool["name"]: tool for tool in listed}
                    assert set(by_name) == names
                    observation = {"protocol": protocol, "mode": mode, "argv": flags,
                                   "tool_count": len(listed), "tool_names": sorted(names)}
                    if TOOL in names:
                        tool = by_name[TOOL]
                        assert isinstance(tool.get("description"), str) and tool["description"].strip()
                        assert tool["annotations"]["readOnlyHint"] is True
                        assert tool["annotations"]["openWorldHint"] is False
                        legacy.exact(tool["inputSchema"], schemas["input"])
                        legacy.exact(tool["outputSchema"], schemas["output"])
                        selected = [case for case in expected_cases
                                    if mode == "alias" or case["name"] != "large-accepted-report"]
                        observation["complete_report_comparisons"] = [proof(session, case) for case in selected]
                        rejected = [case for case in malformed_cases
                                    if mode == "alias" or case["name"] != "report-byte-limit"]
                        observation["transport_failures_no_partial_report"] = [refusal(session, case) for case in rejected]
                        again = proof(session, expected_cases[0])
                        assert again["full_report_sha256"] == observation["complete_report_comparisons"][0]["full_report_sha256"]
                        observation["after_error_repeat_equal"] = True
                    observations.append(observation)
                except BaseException as error:
                    failure = repr(error)
                    raise
                finally:
                    session.close(failure)
    except BaseException as error:
        first_failure = {"type": type(error).__name__, "message": str(error)[:4096]}
        raise
    finally:
        closed = bool(processes) and all(row["waited"] and row["streams_EOF"] and row["process_group_ESRCH"] for row in processes)
        report = {"schema": "gooo/alias-origin-mcp-wire-verification/v1",
            "status": "INTERNAL_ALIAS_MCP_TWO_PROTOCOL_COMPARISON_PASS" if first_failure is None and closed else "FIRST_FAILURE_NO_RETRY",
            "first_failure": first_failure, "binary": reference(args.binary),
            "alias_cli": reference(args.alias_cli), "go_identity_reader": reference(args.go),
            "legacy_verifier": reference(args.legacy_verifier), "build_identities": identity,
            "alias_module": {"path": str(args.alias_module), "module": MODULE, "version": VERSION,
                "generation": reference(args.alias_module / "generation.json"),
                "go_mod": reference(args.alias_module / "go.mod"),
                "schemas": {name: reference(args.alias_module / "schemas" / (name + ".schema.json")) for name in schemas}},
            "observations": observations, "processes": processes,
            "executed_cases": {"reference_normal": [row["name"] for row in expected_cases],
                "reference_rejected": [row["name"] for row in malformed_cases],
                "large_reports_exercised": args.scope == "full"},
            "actual_protocol_sessions": sum(row["label"].endswith(PROTOCOLS) for row in processes),
            "all_direct_wait_EOF_PG_ESRCH": closed,
            "captured_stdout_bytes": captured_stdout,
            "argument_limit_bytes": ARGUMENT_LIMIT,
            "response_limit_including_jsonrpc_and_newline_bytes": RESPONSE_LIMIT,
            "observed_max_response_bytes": max((frame["bytes"] for row in processes for frame in row["response_frames_including_newline"]), default=0),
            "compiler_builds": 0, "Go_builds": 0, "model_calls": 0,
            "live_authority_or_server_identity_proven": False,
            "other_hosts_OSes_or_external_adoption_proven": False,
            "scope": args.scope,
            "scope_description": "Finite internal comparison of supplied facts through raw stdio; existing CLI is the transport reference, not a new independent kernel oracle. bundle-small omits the large-report and 1 MiB argument-boundary observations."}
        save(args.out / "acceptance.json", report)
        if first_failure is None and closed:
            try:
                validate_saved(args.out, args.binary, args.alias_cli, args.alias_module,
                               args.go, args.legacy_verifier, expected_scope=args.scope)
                checkpoint(deadline)
                report["saved_evidence_validated"] = True
                save(args.out / "acceptance.json", report)
            except BaseException as error:
                report["status"] = "FIRST_FAILURE_NO_RETRY"
                report["first_failure"] = {"type": type(error).__name__, "message": str(error)[:4096]}
                save(args.out / "acceptance.json", report)
                raise
        print(json.dumps(reference(args.out / "acceptance.json")))


if __name__ == "__main__":
    def interrupted(number, _frame):
        global PENDING_SIGNAL
        if PENDING_SIGNAL is None:
            PENDING_SIGNAL = number

    signal.signal(signal.SIGTERM, interrupted)
    signal.signal(signal.SIGINT, interrupted)
    main()
