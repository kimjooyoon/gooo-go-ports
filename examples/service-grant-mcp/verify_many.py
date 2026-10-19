"""Independent stdlib JSON-RPC trial for the explicitly enabled many tool.

Policy expectations come from the existing independent reference corpus, not
from the MCP handler. No Go/compiler/model calls are made by this client.
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

PENDING_SIGNAL = None


class WireCancelled(BaseException):
    pass


def checkpoint():
    if PENDING_SIGNAL is not None:
        raise WireCancelled("wire client interrupted: " + str(PENDING_SIGNAL))


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def reference(path):
    raw = path.read_bytes()
    return {"path": str(path), "bytes": len(raw), "sha256": sha(raw)}


def lossless_json(raw):
    def unique(pairs):
        value = {}
        for key, child in pairs:
            assert key not in value, "duplicate JSON-RPC response member"
            value[key] = child
        return value
    def nonfinite(_):
        raise AssertionError("nonfinite JSON-RPC response value")
    return json.loads(raw, object_pairs_hook=unique, parse_constant=nonfinite)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", required=True, type=Path)
    parser.add_argument("--legacy-verifier", type=Path, default=Path(__file__).resolve().with_name("verify.py"))
    parser.add_argument("--many-module", required=True, type=Path)
    parser.add_argument("--out", required=True, type=Path)
    args = parser.parse_args()
    assert args.binary.is_absolute() and args.binary.is_file()
    args.out.mkdir(mode=0o700, exist_ok=False)
    spec = importlib.util.spec_from_file_location("frozen_legacy_wire_client", args.legacy_verifier)
    legacy = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(legacy)
    legacy.BINARY, legacy.OUTPUT = args.binary, args.out
    generation_raw = (args.many_module / "generation.json").read_bytes()
    generation = json.loads(generation_raw)
    assert generation["compiler_source_sha"] == "de99b770b8b5adea608233c3c0664902336e3310"
    assert sha(generation_raw) == "3c49d16a549bc961c1712c122383e1c4d2dd232361a526d146f1a5347108918c"
    bindings = {"compiler_source_sha": generation["compiler_source_sha"],
                "generation_sha256": "sha256:" + sha(generation_raw),
                "sources": generation["public_sources"], "assets": generation["assets"]}
    assert len(bindings["sources"]) == 5 and len(bindings["assets"]) == 30
    expected_raw = (args.many_module / "reference-expected.json").read_bytes()
    assert "sha256:" + sha(expected_raw) == generation["assets"]["reference-expected.json"]
    expected = json.loads(expected_raw)
    assert len(expected) == 16

    class ManySession(legacy.Session):
        def __init__(self, label, protocol):
            self.label, self.protocol = label, protocol
            self.env = os.environ.copy()
            self.env.update(PATH="", GOPROXY="off", GOSUMDB="off", GOTELEMETRY="off", MCPGODEBUG="")
            self.env.pop("GOROOT", None)
            self.work = legacy.OUTPUT / label
            self.work.mkdir(mode=0o700)
            self.selector = selectors.DefaultSelector()
            self.buffers = {"stdout": bytearray(), "stderr": bytearray()}
            self.linebuf, self.sent, self.frames = bytearray(), bytearray(), []
            self.next_id, self.started = 1, time.monotonic()
            self.cleaning = False
            self.process = None
            try:
                mask = signal.pthread_sigmask(signal.SIG_BLOCK, {signal.SIGINT, signal.SIGTERM})
                try:
                    def restore_child_mask():
                        signal.pthread_sigmask(signal.SIG_SETMASK, mask)
                    self.process = subprocess.Popen([str(legacy.BINARY), "--many-required"],
                        cwd=self.work, env=self.env, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                        stderr=subprocess.PIPE, start_new_session=True, preexec_fn=restore_child_mask)
                    os.set_blocking(self.process.stdin.fileno(), False)
                    # Persist the owned child before unblocking cancellation.
                    with (args.out / (label + "-started.json")).open("x") as journal:
                        json.dump({"PID": self.process.pid,
                            "argv": [str(legacy.BINARY), "--many-required"],
                            "binary_sha256": sha(args.binary.read_bytes())}, journal)
                        journal.flush()
                        os.fsync(journal.fileno())
                    for name in self.buffers:
                        stream = getattr(self.process, name)
                        os.set_blocking(stream.fileno(), False)
                        self.selector.register(stream, selectors.EVENT_READ, name)
                finally:
                    signal.pthread_sigmask(signal.SIG_SETMASK, mask)
                checkpoint()
            except BaseException as original:
                record = {"failure": repr(original), "PID": None,
                          "waited": False, "EOF": False, "PG_ESRCH": False,
                          "secondary_cleanup_errors": []}
                # No partially constructed Session escapes with a live child.
                if self.process is not None:
                    record["PID"] = self.process.pid
                    for number in (signal.SIGTERM, signal.SIGKILL):
                        try:
                            os.killpg(self.process.pid, number)
                        except ProcessLookupError:
                            break
                        except BaseException as secondary:
                            record["secondary_cleanup_errors"].append(repr(secondary))
                        try:
                            self.process.wait(timeout=1)
                        except BaseException as secondary:
                            record["secondary_cleanup_errors"].append(repr(secondary))
                    try:
                        self.process.wait(timeout=1)
                        record["waited"] = True
                    except BaseException as secondary:
                        record["secondary_cleanup_errors"].append(repr(secondary))
                    for stream in (self.process.stdin, self.process.stdout, self.process.stderr):
                        try:
                            stream.close()
                        except BaseException:
                            pass
                    try:
                        os.killpg(self.process.pid, 0)
                    except ProcessLookupError:
                        record["PG_ESRCH"] = True
                    except BaseException as secondary:
                        record["secondary_cleanup_errors"].append(repr(secondary))
                    # Pipes were closed; kernel EOF was not independently drained.
                    record["EOF"] = "NOT_DRAINED_CONSTRUCTOR_FAILURE"
                try:
                    self.selector.close()
                except BaseException as secondary:
                    record["secondary_cleanup_errors"].append(repr(secondary))
                try:
                    (args.out / (label + "-constructor-failure.json")).write_text(json.dumps(record) + "\n")
                except BaseException:
                    pass
                raise

        def read(self):
            if not self.cleaning:
                checkpoint()
            return super().read()

        def request(self, method, params=None, arguments=None):
            checkpoint()
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
                checkpoint()
                while b"\n" in self.linebuf:
                    line, _, rest = self.linebuf.partition(b"\n")
                    self.linebuf[:] = rest
                    value = lossless_json(line)
                    assert isinstance(value, dict) and value.get("jsonrpc") == "2.0"
                    self.frames.append(value)
                    if "id" in value:
                        assert type(value["id"]) is int and value["id"] == current
                        return value
                    assert "result" not in value and "error" not in value
                assert self.selector.get_map(), "server closed without response"
                self.read()

        def send_frame(self, frame):
            offset = 0
            while offset < len(frame):
                checkpoint()
                assert time.monotonic() - self.started < 30, "bounded stdin session deadline"
                try:
                    count = os.write(self.process.stdin.fileno(), frame[offset:offset + 65536])
                except BlockingIOError:
                    # Drain stdout/stderr as well as waiting for the reader; a
                    # large valid request cannot bypass the session deadline.
                    self.read()
                    continue
                assert count > 0, "stdin write made no progress"
                self.sent.extend(frame[offset:offset + count])
                offset += count

        def notify_initialized(self):
            self.send_frame(b'{"jsonrpc":"2.0","method":"notifications/initialized"}\n')

        def close(self, failure):
            self.cleaning = True
            return super().close(failure)

    def proof(session, arguments, facts, decision):
        response = session.request("tools/call", {"name": "assess_service_grant_change"}, arguments)
        assert "error" not in response, response
        result = response["result"]
        assert not result.get("isError", False), result
        wanted = {"facts": facts, "decision": decision, "bindings": bindings}
        legacy.exact(result["structuredContent"], wanted)
        assert len(result["content"]) == 1 and result["content"][0]["type"] == "text"
        legacy.exact(json.loads(result["content"][0]["text"]), wanted)
        return {"status": decision["status"], "reason": decision["reason"],
                "facts_sha256": sha(legacy.encoded(facts)),
                "full_proof_sha256": sha(legacy.encoded(wanted))}

    observations = []
    for label, protocol in [("many-legacy-session", "2025-11-25"),
                            ("many-current-session", "2026-07-28")]:
        session, failure = ManySession(label, protocol), None
        try:
            if protocol == "2025-11-25":
                init = session.request("initialize", {"protocolVersion": protocol,
                    "capabilities": {}, "clientInfo": {"name": "gooo-many-stdlib-wire-trial", "version": "0"}})
                assert init["result"]["protocolVersion"] == protocol
                session.notify_initialized()
            else:
                discover = session.request("server/discover")["result"]
                assert protocol in discover["supportedVersions"]
            listed = session.request("tools/list")["result"]["tools"]
            assert len(listed) == 2
            by_name = {tool["name"]: tool for tool in listed}
            assert set(by_name) == {"assess_service_grant", "assess_service_grant_change"}
            tool = by_name["assess_service_grant_change"]
            schema = tool["inputSchema"]
            assert schema["additionalProperties"] is False
            assert set(schema["required"]) == set(schema["properties"]) == {"before", "after", "required_requests"}
            requests = schema["properties"]["required_requests"]
            assert requests["type"] == "array" and "minItems" not in requests and "maxItems" not in requests
            assert requests["items"]["additionalProperties"] is False
            assert set(requests["items"]["required"]) == set(requests["items"]["properties"]) == {"type", "name"}
            assert tool["annotations"]["readOnlyHint"] is True and tool["annotations"]["openWorldHint"] is False
            decision_schema = tool["outputSchema"]["properties"]["decision"]
            assert len(decision_schema["properties"]) == 12
            for name in ["invalid_index", "first_counterexample", "request_count", "checked_count"]:
                legacy.exact(decision_schema["properties"][name], {"type": "integer"})
            old_case = legacy.proof(session, 1)
            cases = []
            for case in expected:
                facts = case["facts"]
                if len(facts["required_types"]) != len(facts["required_names"]):
                    assert case["name"] == "length-mismatch"
                    continue  # Pair transport cannot express mismatched columns.
                arguments = {"before": facts["before"], "after": facts["after"],
                    "required_requests": [{"type": kind, "name": name}
                        for kind, name in zip(facts["required_types"], facts["required_names"])]}
                cases.append({"name": case["name"], **proof(session, legacy.encoded(arguments), facts, case["decision"])})
            assert len(cases) == 15
            # The policy, rather than JSON Schema or this adapter, rejects257.
            arguments257 = {"before": "*", "after": "mcp://*",
                            "required_requests": [{"type": "mcp", "name": "weather"}] * 257}
            facts257 = {"before": "*", "after": "mcp://*", "required_types": ["mcp"] * 257,
                        "required_names": ["weather"] * 257}
            decision257 = {"status": "INVALID_INPUT", "reason": "REQUIRED_REQUEST_LIMIT_EXCEEDED",
                "invalid_field": "required_types", "invalid_index": -1, "first_counterexample": -1,
                "request_count": 257, "checked_count": 0, "all_preserved": False,
                "supported_subset": False, "subset_rule_id": "", "counterexample_before_match": False,
                "counterexample_after_match": False}
            boundary = proof(session, legacy.encoded(arguments257), facts257, decision257)
            malformed = [
                ("later-duplicate", b'{"before":"*","after":"mcp://weather","required_requests":[{"type":"mcp","name":"calculator"},{"type":"mcp","name":"weather","name":"calculator"}]}'),
                ("later-null", b'{"before":"*","after":"mcp://weather","required_requests":[{"type":"mcp","name":"calculator"},null]}'),
                ("later-unknown", b'{"before":"*","after":"mcp://weather","required_requests":[{"type":"mcp","name":"calculator"},{"type":"mcp","name":"weather","decision":"ELIGIBLE"}]}'),
                ("later-surrogate", b'{"before":"*","after":"mcp://weather","required_requests":[{"type":"mcp","name":"calculator"},{"type":"mcp","name":"\\ud800"}]}'),
            ]
            first = expected[0]
            base = legacy.encoded({"before": first["facts"]["before"], "after": first["facts"]["after"],
                "required_requests": [{"type": t, "name": n} for t, n in zip(first["facts"]["required_types"], first["facts"]["required_names"])]})
            maximum = b"{" + b" " * ((1 << 20) - len(base)) + base[1:]
            assert len(maximum) == 1 << 20
            proof(session, maximum, first["facts"], first["decision"])
            malformed.append(("argument-byte-limit-plus-one", maximum[:-1] + b" }"))
            refused = []
            for name, arguments in malformed:
                response = session.request("tools/call", {"name": "assess_service_grant_change"}, arguments)
                if "error" in response:
                    assert set(response) == {"jsonrpc", "id", "error"}, response
                    error = response["error"]
                    assert isinstance(error, dict) and set(error) <= {"code", "message"}
                    assert type(error["code"]) is int and error["code"] == -32602
                    assert isinstance(error["message"], str) and error["message"]
                else:
                    result = response["result"]
                    assert type(result.get("isError")) is bool and result["isError"] is True
                    expected_keys = {"isError", "content"}
                    if protocol == "2026-07-28":
                        expected_keys |= {"_meta", "resultType"}
                        assert result["resultType"] == "complete"
                        legacy.exact(result["_meta"], {"io.modelcontextprotocol/serverInfo": {
                            "name": "gooo-service-grant", "version": "0.1.0-dev"}})
                    assert set(result) == expected_keys
                    assert len(result["content"]) == 1 and set(result["content"][0]) == {"type", "text"}
                    assert result["content"][0]["type"] == "text"
                    assert result["content"][0]["text"].startswith("INVALID_FACTS:"), result
                refused.append(name)
            again = expected[0]
            replay = proof(session, base, again["facts"], again["decision"])
            assert replay["full_proof_sha256"] == cases[0]["full_proof_sha256"]
            observations.append({"protocol": protocol, "legacy_tool_case": old_case,
                "reference_cases": cases, "native_policy257": boundary,
                "argument_bytes_at_limit": 1 << 20, "structural_failures_no_partial_proof": refused,
                "after_error_repeat_equal": True})
        except BaseException as error:
            failure = repr(error)
            raise
        finally:
            session.close(failure)
        checkpoint()
    report = {"status": "PRIVATE_MANY_MCP_TWO_PROTOCOL_WIRE_REFERENCE_CASES_ACCEPTED",
              "binary": reference(args.binary), "legacy_verifier": reference(args.legacy_verifier),
              "generation": reference(args.many_module / "generation.json"),
              "independent_expected": reference(args.many_module / "reference-expected.json"),
              "observations": observations, "actual_protocol_sessions": 2,
              "Go_compiler_or_model_calls": 0, "live_authority_or_application": False,
              "other_hosts_OSes_or_external_adoption_proven": False}
    path = args.out / "acceptance.json"
    path.write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps(reference(path)))


if __name__ == "__main__":
    def interrupted(number, _frame):
        global PENDING_SIGNAL
        if PENDING_SIGNAL is None:
            PENDING_SIGNAL = number

    signal.signal(signal.SIGTERM, interrupted)
    signal.signal(signal.SIGINT, interrupted)
    main()
