"""Finite Docker stdio smoke; no model, compiler, service call or grant application.

The existing independent wire client's framing and full-proof comparisons are
reused, while Docker CLI and daemon-owned container lifecycles stay separate.
"""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import selectors
import shutil
import signal
import stat
import subprocess
import time
import uuid

ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location("native_wire", ROOT / "examples/service-grant-mcp/verify.py")
WIRE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(WIRE)
PENDING_SIGNAL = None


def checkpoint():
    if PENDING_SIGNAL is not None:
        raise KeyboardInterrupt("signal " + str(PENDING_SIGNAL))


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def save(path, value):
    path.write_text(json.dumps(value, indent=2) + "\n")


def note(error, message):
    if hasattr(error, "add_note"):
        error.add_note(message)


def registry_package(path):
    spec = importlib.util.spec_from_file_location('fixed_registry_metadata', ROOT / 'tools/publish-service-grant-registry.py')
    publisher = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(publisher)
    raw = path.read_bytes()
    publisher.require(len(raw) == publisher.METADATA_BYTES and sha(raw) == publisher.METADATA_SHA,
                      'registry descriptor bytes changed')
    doc = publisher.load(raw)
    publisher.check_metadata(doc)
    package = doc['packages'][0]
    flags = []
    for argument in package['runtimeArguments']:
        flags.append(argument['name'])
        if 'value' in argument:
            flags.append(argument['value'])
    return {'identifier': package['identifier'], 'runtime_argv': flags, 'descriptor_sha256': sha(raw)}


def create_arguments(trial, name):
    package = getattr(trial, 'registry_package', None)
    flags = package['runtime_argv'] if package else ['--platform=linux/amd64', '--network=none', '--read-only']
    image = package['identifier'] if package else trial.image
    return ['container', 'create', '--name', name, '--pull=never', '--interactive', *flags,
            '--cap-drop=ALL', '--security-opt=no-new-privileges', '--pids-limit=32',
            '--memory=128m', '--user=65532:65532', '--label', trial.label, image]


class Trial:
    def __init__(self, docker, image, context, out, label, registry_package=None):
        self.docker, self.image, self.context, self.out, self.label = docker, image, context, out, label
        self.registry_package = registry_package
        self.sequence = 0
        self.cleaning = False
        self.commands = out / "commands"
        self.commands.mkdir()

    def command(self, args, accept_failure=False):
        """Bound diagnostic output and join the exact CLI group on every path."""
        if not self.cleaning:
            checkpoint()
        self.sequence += 1
        folder = self.commands / str(self.sequence)
        folder.mkdir()
        argv = [self.docker, *args]
        save(folder / "intent.json", {"argv": argv})
        selector = selectors.DefaultSelector()
        buffers = {"stdout": bytearray(), "stderr": bytearray()}
        started, failure = time.monotonic(), None
        try:
            process = subprocess.Popen(argv, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                                       stderr=subprocess.PIPE, start_new_session=True)
        except BaseException:
            selector.close()
            raise
        try:
            save(folder / "started.json", {"PID": process.pid, "argv": argv})
            for name in buffers:
                stream = getattr(process, name)
                os.set_blocking(stream.fileno(), False)
                selector.register(stream, selectors.EVENT_READ, name)
            while selector.get_map():
                if not self.cleaning:
                    checkpoint()
                assert time.monotonic() - started < 15, "Docker command deadline"
                for key, _ in selector.select(0.05):
                    raw = os.read(key.fd, 65536)
                    if not raw:
                        selector.unregister(key.fileobj)
                    else:
                        buffers[key.data].extend(raw)
                        assert sum(map(len, buffers.values())) <= 1 << 20, "Docker diagnostic limit"
            process.wait(timeout=2)
        except BaseException as error:
            failure = error
            raise
        finally:
            for number in (signal.SIGTERM, signal.SIGKILL):
                try:
                    os.killpg(process.pid, number)
                except ProcessLookupError:
                    break
                try:
                    process.wait(timeout=0.5)
                except subprocess.TimeoutExpired:
                    pass
            process.wait(timeout=1)
            absent = False
            try:
                os.killpg(process.pid, 0)
            except ProcessLookupError:
                absent = True
            secondary = []
            for name, raw in buffers.items():
                try:
                    (folder / name).write_bytes(raw)
                except BaseException as error:
                    secondary.append(repr(error))
                try:
                    getattr(process, name).close()
                except BaseException as error:
                    secondary.append(repr(error))
            eof = not selector.get_map()
            try:
                selector.close()
            except BaseException as error:
                secondary.append(repr(error))
            try:
                save(folder / "process.json", {"PID": process.pid, "exit_code": process.returncode,
                     "waited": True, "EOF": eof, "process_group_absent": absent,
                     "failure": repr(failure) if failure is not None else None,
                     "secondary_diagnostic_errors": secondary})
            except BaseException as error:
                secondary.append(repr(error))
            if failure is not None:
                for diagnostic in secondary:
                    note(failure, "Docker CLI diagnostic: " + diagnostic)
            elif secondary:
                raise AssertionError(secondary)
        assert absent and eof, "Docker CLI closure incomplete"
        if not self.cleaning:
            checkpoint()
        if not accept_failure:
            assert process.returncode == 0 and not buffers["stderr"], bytes(buffers["stderr"])
        return process.returncode, bytes(buffers["stdout"]), bytes(buffers["stderr"])

    def inspect(self, name):
        _, raw, _ = self.command(["container", "inspect", name])
        rows = json.loads(raw)
        assert len(rows) == 1
        return rows[0]

    def removed(self, name):
        # A daemon failure is not evidence of absence. A successful filtered list is.
        _, raw, _ = self.command(["container", "ls", "--all", "--quiet", "--no-trunc",
                                  "--filter", "name=^/" + name + "$"])
        assert not raw.strip(), "owned container still exists"


class ContainerSession(WIRE.Session):
    def __init__(self, trial, mode):
        self.trial, self.label, self.protocol = trial, mode, "2025-11-25"
        self.work = trial.out / mode
        self.work.mkdir()
        self.name = "gooo-wire-" + uuid.uuid4().hex
        self.process = None
        self.cleaning = False
        self.selector = selectors.DefaultSelector()
        self.buffers = {"stdout": bytearray(), "stderr": bytearray()}
        self.linebuf, self.sent, self.frames = bytearray(), bytearray(), []
        self.next_id, self.started = 1, time.monotonic()
        # Persist a unique recovery name before create, including if its reply is lost.
        save(self.work / "intent.json", {"container_name": self.name, "image": trial.image})
        try:
            args = create_arguments(trial, self.name)
            if mode == "many":
                args.append("--many-required")
            _, raw, _ = trial.command(args)
            self.cid = raw.decode().strip()
            assert re.fullmatch(r"[0-9a-f]{64}", self.cid)
            row = trial.inspect(self.name)
            assert row["Id"] == self.cid and row["Image"] == trial.image
            config, host = row["Config"], row["HostConfig"]
            assert config["User"] == "65532:65532" and not config["Tty"]
            assert config["Entrypoint"] == ["/opt/gooo/gooo-service-grant-mcp"]
            assert config["Cmd"] == (["--many-required"] if mode == "many" else []) or (
                mode == "default" and config["Cmd"] is None)
            env = dict(item.split("=", 1) for item in config["Env"])
            assert env["PATH"] == "" and "GOROOT" not in env
            assert host["ReadonlyRootfs"] and host["NetworkMode"] == "none"
            assert host["PidsLimit"] == 32 and host["Memory"] == 128 << 20
            assert "ALL" in host["CapDrop"] and "no-new-privileges" in host["SecurityOpt"]
            save(self.work / "created.json", row)
            # Verify what this same container will execute, including all notices.
            copied = self.work / "copied-runtime"
            copied.mkdir()
            trial.command(["container", "cp", self.name + ":/opt/gooo/.", str(copied)])
            expected = trial.context / "runtime"
            assert {p.name for p in copied.iterdir()} == {p.name for p in expected.iterdir()}
            members = {}
            for path in copied.iterdir():
                assert stat.S_ISREG(path.lstat().st_mode)
                raw = path.read_bytes()
                assert raw == (expected / path.name).read_bytes()
                assert stat.S_IMODE(path.stat().st_mode) == stat.S_IMODE((expected / path.name).stat().st_mode)
                members[path.name] = {"bytes": len(raw), "sha256": sha(raw)}
            save(self.work / "actual-runtime.json", members)
            shutil.rmtree(copied)  # Only our fresh copy; the canonical bundle remains.
            self.process = subprocess.Popen([trial.docker, "container", "start", "--attach", "--interactive", self.name],
                stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)
            save(self.work / "started.json", {"CLI_PID": self.process.pid, "container_id": self.cid})
            for name in self.buffers:
                stream = getattr(self.process, name)
                os.set_blocking(stream.fileno(), False)
                self.selector.register(stream, selectors.EVENT_READ, name)
            checkpoint()
        except BaseException as error:
            try:
                self.close(repr(error))
            except BaseException as cleanup:
                note(error, "container cleanup: " + repr(cleanup))
            raise

    def read(self):
        if not self.cleaning:
            checkpoint()
        return super().read()

    def close(self, failure):
        # Signals latch while cleanup joins the CLI and removes the owned CID.
        self.cleaning = self.trial.cleaning = True
        cleanup_failure, terminal, removed = None, None, False
        try:
            if self.process is not None:
                try:
                    self.process.stdin.close()
                    while self.selector.get_map():
                        self.read()
                    self.process.wait(timeout=2)
                    assert self.process.returncode == 0 and not self.buffers["stderr"] and not self.linebuf
                finally:
                    for number in (signal.SIGTERM, signal.SIGKILL):
                        try:
                            os.killpg(self.process.pid, number)
                        except ProcessLookupError:
                            break
                        try:
                            self.process.wait(timeout=0.5)
                        except subprocess.TimeoutExpired:
                            pass
                    self.process.wait(timeout=1)
                    absent = False
                    try:
                        os.killpg(self.process.pid, 0)
                    except ProcessLookupError:
                        absent = True
                    eof = not self.selector.get_map()
                    save(self.work / "CLI-process.json", {"PID": self.process.pid,
                        "exit_code": self.process.returncode, "waited": True,
                        "EOF": eof, "process_group_absent": absent})
                    for name, raw in self.buffers.items():
                        (self.work / name).write_bytes(raw)
                        getattr(self.process, name).close()
                    (self.work / "stdin").write_bytes(self.sent)
                    assert absent and eof, "Docker attach closure incomplete"
                terminal = self.trial.inspect(self.name)
                save(self.work / "terminal.json", terminal)
                assert terminal["State"]["Status"] == "exited" and terminal["State"]["ExitCode"] == 0
        except BaseException as error:
            cleanup_failure = repr(error)
        finally:
            try:
                self.selector.close()
            except BaseException as error:
                cleanup_failure = cleanup_failure or repr(error)
            # CLI termination alone never implies daemon-owned container termination.
            try:
                code, raw, diagnostic = self.trial.command(["container", "rm", "--force", self.name], accept_failure=True)
                save(self.work / "removal.json", {"exit_code": code, "stdout": raw.decode(), "stderr": diagnostic.decode()})
                self.trial.removed(self.name)
                removed = True
            except BaseException as error:
                cleanup_failure = cleanup_failure or repr(error)
            try:
                save(self.work / "result.json", {"first_failure": failure or cleanup_failure,
                    "cleanup_failure": cleanup_failure, "container_absence_verified": removed})
            except BaseException as error:
                cleanup_failure = cleanup_failure or repr(error)
            self.trial.cleaning = False
        if failure is None and cleanup_failure is not None:
            raise AssertionError(cleanup_failure)


def many_proofs(session):
    folder = ROOT / "examples/service-grant-many"
    raw = (folder / "generation.json").read_bytes()
    assert sha(raw) == "86d714e4793f04d7d45d53e824cd7066ddee5f7d1e987697f26f4216a6b4e164"
    generation = json.loads(raw)
    bindings = {"compiler_source_sha": generation["compiler_source_sha"],
        "generation_sha256": "sha256:" + sha(raw), "sources": generation["public_sources"], "assets": generation["assets"]}
    assert len(bindings["sources"]) == 5 and len(bindings["assets"]) == 30
    raw = (folder / "reference-expected.json").read_bytes()
    assert "sha256:" + sha(raw) == generation["assets"]["reference-expected.json"]
    tools = session.request("tools/list")["result"]["tools"]
    assert len(tools) == 2 and {t["name"] for t in tools} == {WIRE.TOOL, "assess_service_grant_change"}
    tool = next(t for t in tools if t["name"] == "assess_service_grant_change")
    schema = tool["inputSchema"]
    assert schema["additionalProperties"] is False
    assert set(schema["required"]) == set(schema["properties"]) == {"before", "after", "required_requests"}
    for key in ("before", "after"):
        WIRE.described_text(schema["properties"][key])
    requests = schema["properties"]["required_requests"]
    assert requests["type"] == "array" and "minItems" not in requests and "maxItems" not in requests
    assert type(requests['description']) is str and requests['description'].strip()
    assert requests["items"]["additionalProperties"] is False
    assert set(requests["items"]["required"]) == set(requests["items"]["properties"]) == {"type", "name"}
    for key in ("type", "name"):
        WIRE.described_text(requests["items"]["properties"][key])
    assert tool["annotations"]["readOnlyHint"] is True and tool["annotations"]["openWorldHint"] is False
    decision = tool["outputSchema"]["properties"]["decision"]
    assert len(decision["properties"]) == 12 and decision["additionalProperties"] is False
    assert set(decision["required"]) == set(decision["properties"])
    for key in ("invalid_index", "first_counterexample", "request_count", "checked_count"):
        WIRE.exact(decision["properties"][key], {"type": "integer"})
    results = []
    for case in json.loads(raw)[:2]:
        facts = case["facts"]
        arguments = {"before": facts["before"], "after": facts["after"], "required_requests": [
            {"type": kind, "name": name} for kind, name in zip(facts["required_types"], facts["required_names"])]}
        result = session.request("tools/call", {"name": tool["name"]}, WIRE.encoded(arguments))["result"]
        assert not result.get("isError", False)
        wanted = {"facts": facts, "decision": case["decision"], "bindings": bindings}
        WIRE.exact(result["structuredContent"], wanted)
        assert len(result["content"]) == 1 and result["content"][0]["type"] == "text"
        WIRE.exact(json.loads(result["content"][0]["text"]), wanted)
        results.append({"case": case["name"], "proof_sha256": sha(WIRE.encoded(wanted))})
    return results


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--docker", required=True)
    parser.add_argument("--image", required=True, help="Exact local sha256 image ID, never a mutable tag")
    parser.add_argument("--context", type=Path, required=True)
    parser.add_argument("--registry-metadata", type=Path, help="Optional byte-pinned preview16 OCI descriptor; not generic untrusted metadata")
    parser.add_argument("--out", type=Path, required=True)
    parser.add_argument("--label", default="io.gooo.container-smoke=local")
    args = parser.parse_args()
    assert re.fullmatch(r"sha256:[0-9a-f]{64}", args.image)
    docker = str(Path(args.docker).resolve(strict=True))
    context = args.context.resolve(strict=True)
    package = registry_package(args.registry_metadata) if args.registry_metadata else None
    args.out.mkdir(parents=True, exist_ok=False)
    trial = Trial(docker, args.image, context, args.out, args.label, package)
    if package:
        _, raw, _ = trial.command(['image', 'inspect', package['identifier']])
        images = json.loads(raw)
        assert len(images) == 1 and images[0]['Id'] == args.image
        assert images[0]['Os'] == 'linux' and images[0]['Architecture'] == 'amd64'
        assert package['identifier'] in images[0]['RepoDigests']
        save(args.out / 'registry-install-input.json', {**package, 'local_image_id': args.image,
             'scope': 'Descriptor-driven finite headless client; additional lifecycle/resource guards applied. No AI host or outside adoption claim.'})
    observations = []
    try:
        for mode in ("default", "many"):
            checkpoint()
            session, failure = ContainerSession(trial, mode), None
            try:
                init = session.request("initialize", {"protocolVersion": session.protocol,
                    "capabilities": {}, "clientInfo": {"name": "gooo-container-smoke", "version": "1"}})
                assert init["result"]["protocolVersion"] == session.protocol
                WIRE.exact(init["result"]["serverInfo"], {"name": "gooo-service-grant", "version": "0.1.0-dev"})
                session.notify_initialized()
                if mode == "default":
                    WIRE.listed(session)
                    cases = [WIRE.proof(session, i) for i in (1, 2, 3)]
                    result = session.request("tools/call", {"name": WIRE.TOOL}, b'{"before":"*"}')["result"]
                    assert result["isError"] is True and "structuredContent" not in result
                else:
                    cases = many_proofs(session)
                observations.append({"mode": mode, "proofs": cases})
            except BaseException as error:
                failure = repr(error)
                raise
            finally:
                # The original protocol failure stays primary if cleanup also fails.
                session.close(failure)
            checkpoint()
        report = {"status": "CONTAINER_STDIO_SMOKE_PASS", "image": args.image,
            "binary_sha256": sha((context / "runtime/gooo-service-grant-mcp").read_bytes()),
            "observations": observations, "model_calls": 0, "network": "none", "rootfs": "read-only",
            "external_adoption": False, "Glama_approval_or_all_client_conformance": False}
        save(args.out / "verification.json", report)
        print(json.dumps(report))
    except BaseException as error:
        try:
            save(args.out / "first-failure.json", {"error": repr(error)})
        except BaseException as diagnostic:
            note(error, "could not retain first failure: " + repr(diagnostic))
        raise


if __name__ == "__main__":
    def interrupted(number, _frame):
        global PENDING_SIGNAL
        if PENDING_SIGNAL is None:
            PENDING_SIGNAL = number
    signal.signal(signal.SIGTERM, interrupted)
    signal.signal(signal.SIGINT, interrupted)
    main()
