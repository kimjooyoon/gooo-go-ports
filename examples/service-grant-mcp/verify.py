"""Independent stdlib wire client; no MCP SDK and no model/API calls."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import selectors
import signal
import subprocess
import time

ROOT = Path(__file__).resolve().parent
FIXTURES = ROOT/'fixtures'
BINARY = None
OUTPUT = None
BUILD_INFO = None
TOOL = 'assess_service_grant'


def encoded(value):
    return json.dumps(value, separators=(',', ':'), ensure_ascii=True).encode()


def exact(a, b):
    assert type(a) is type(b), (type(a).__name__, type(b).__name__)
    if isinstance(a, dict):
        assert a.keys() == b.keys()
        for key in a:
            exact(a[key], b[key])
    elif isinstance(a, list):
        assert len(a) == len(b)
        for x, y in zip(a, b):
            exact(x, y)
    else:
        assert a == b, (a, b)


class Session:
    def __init__(self, label, protocol, arguments=(), stream_limit=2 << 20):
        self.label, self.protocol = label, protocol
        self.env = os.environ.copy()
        self.env.update(PATH='', GOPROXY='off', GOSUMDB='off', GOTELEMETRY='off', MCPGODEBUG='')
        self.env.pop('GOROOT', None)
        self.work = OUTPUT/label
        self.work.mkdir(mode=0o700)
        self.stream_limit = stream_limit
        self.process = subprocess.Popen([str(BINARY), *arguments],
                                        cwd=self.work, env=self.env, stdin=subprocess.PIPE,
                                        stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                        start_new_session=True)
        self.selector = selectors.DefaultSelector()
        self.buffers = {'stdout': bytearray(), 'stderr': bytearray()}
        self.linebuf = bytearray()
        self.frames = []
        self.sent = bytearray()
        self.next_id = 1
        self.started = time.monotonic()
        for name in self.buffers:
            stream = getattr(self.process, name)
            os.set_blocking(stream.fileno(), False)
            self.selector.register(stream, selectors.EVENT_READ, name)

    def read(self):
        assert time.monotonic()-self.started < 30, 'session deadline'
        for key, _ in self.selector.select(0.05):
            raw = os.read(key.fd, 65536)
            if not raw:
                self.selector.unregister(key.fileobj)
            else:
                self.buffers[key.data].extend(raw)
                assert len(self.buffers[key.data]) <= self.stream_limit, 'session stream limit'
                if key.data == 'stdout':
                    self.linebuf.extend(raw)

    def metadata(self):
        return {'io.modelcontextprotocol/protocolVersion': self.protocol,
                'io.modelcontextprotocol/clientInfo': {'name': 'gooo-independent-wire-trial', 'version': '0'},
                'io.modelcontextprotocol/clientCapabilities': {}}

    def request(self, method, params=None, arguments=None):
        current = self.next_id
        self.next_id += 1
        params = dict(params or {})
        if self.protocol == '2026-07-28':
            params['_meta'] = self.metadata()
        wire = encoded(params)
        if arguments is not None:
            # Preserve duplicate members and escaped surrogate bytes in the actual wire.
            wire = wire[:-1] + (b',' if len(wire) > 2 else b'') + b'"arguments":' + arguments + b'}'
        frame = b'{"jsonrpc":"2.0","id":'+str(current).encode()+b',"method":'+encoded(method)+b',"params":'+wire+b'}\n'
        self.process.stdin.write(frame)
        self.process.stdin.flush()
        self.sent.extend(frame)
        while True:
            while b'\n' in self.linebuf:
                line, _, rest = self.linebuf.partition(b'\n')
                self.linebuf[:] = rest
                value = json.loads(line)
                assert value.get('jsonrpc') == '2.0', value
                self.frames.append(value)
                if value.get('id') == current:
                    return value
                assert 'id' not in value, value
            assert self.selector.get_map(), 'server closed without response'
            self.read()

    def notify_initialized(self):
        frame = b'{"jsonrpc":"2.0","method":"notifications/initialized"}\n'
        self.process.stdin.write(frame)
        self.process.stdin.flush()
        self.sent.extend(frame)

    def close(self, failure):
        drained = False
        stdin_close_error = None
        teardown_error = None
        try:
            try:
                self.process.stdin.close()
            except BrokenPipeError as error:
                # A previous failed write may leave buffered bytes to flush.
                # Retain the error, but still drain and join the actual child.
                stdin_close_error = error
            while self.selector.get_map():
                self.read()
            drained = True
            self.process.wait(timeout=2)
        except BaseException as error:
            teardown_error = error
        finally:
            for sig in (signal.SIGTERM, signal.SIGKILL):
                try:
                    os.killpg(self.process.pid, sig)
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
            runtime_files = sorted(x.name for x in self.work.iterdir())
            for name, data in self.buffers.items():
                (self.work/name).write_bytes(data)
            (self.work/'stdin').write_bytes(self.sent)
            self.process.stdout.close()
            self.process.stderr.close()
            self.selector.close()
            first_failure = failure
            if first_failure is None:
                if stdin_close_error is not None:
                    first_failure = repr(stdin_close_error)
                elif teardown_error is not None:
                    first_failure = repr(teardown_error)
            record = {'pid': self.process.pid, 'exit_code': self.process.returncode,
                      'failure': first_failure, 'streams_EOF': drained,
                      'stdin_close_error': repr(stdin_close_error) if stdin_close_error is not None else None,
                      'teardown_error': repr(teardown_error) if teardown_error is not None else None,
                      'waited': True, 'process_group_ESRCH': absent,
                      'elapsed_seconds': time.monotonic()-self.started,
                      'protocol_frames': len(self.frames), 'stderr_bytes': len(self.buffers['stderr']),
                      'all_stdout_frames_parsed': not self.linebuf,
                      'client_uses_MCP_SDK': False, 'runtime_PATH_empty': True,
                      'runtime_GOROOT_absent': True, 'model_calls': 0,
                      'server_created_work_files': runtime_files}
            (self.work/'process.json').write_text(json.dumps(record, indent=2)+'\n')
        # Cancellation still propagates after cleanup, including with an earlier
        # ordinary failure. Other secondary teardown failures must not replace it.
        if teardown_error is not None and not isinstance(teardown_error, Exception):
            raise teardown_error
        if failure is None:
            if stdin_close_error is not None:
                raise stdin_close_error
            if teardown_error is not None:
                raise teardown_error
            assert self.process.returncode == 0 and absent and drained and not self.linebuf and not self.buffers['stderr'] and not runtime_files, record


def described_text(schema):
    assert set(schema) == {'type', 'description', 'examples'}
    assert schema['type'] == 'string'
    assert type(schema['description']) is str and schema['description'].strip()
    assert type(schema['examples']) is list and schema['examples']
    assert all(type(example) is str for example in schema['examples'])


def listed(session):
    result = session.request('tools/list')['result']
    assert len(result['tools']) == 1
    tool = result['tools'][0]
    assert tool['name'] == TOOL
    schema = tool['inputSchema']
    assert set(schema['required']) == set(schema['properties']) == {'before','after','required_type','required_name'}
    assert schema['additionalProperties'] is False
    for field in schema['properties'].values():
        described_text(field)
    assert tool['annotations']['readOnlyHint'] is True and tool['annotations']['openWorldHint'] is False
    assert tool['outputSchema']['additionalProperties'] is False
    output = tool['outputSchema']
    assert set(output['required']) == set(output['properties']) == {'facts', 'decision', 'bindings'}
    exact(output['properties']['facts'], schema)
    decision = output['properties']['decision']
    expected = {name: {'type': 'string'} for name in ['status', 'reason', 'invalid_field', 'subset_rule_id']}
    expected['status']['enum'] = ['ELIGIBLE', 'REFUSED', 'INVALID_INPUT']
    expected.update({name: {'type': 'boolean'} for name in ['required_observed', 'required_before_match', 'required_after_match']})
    exact(decision['properties'], expected)
    assert set(decision['required']) == set(expected) and decision['additionalProperties'] is False
    bindings = output['properties']['bindings']
    assert set(bindings['required']) == set(bindings['properties']) == {'compiler_source_sha','generation_sha256','generation_origin','assets'}
    assert bindings['additionalProperties'] is False
    for key in ['compiler_source_sha','generation_sha256','generation_origin']:
        exact(bindings['properties'][key], {'type':'string'})
    exact(bindings['properties']['assets'], {'type':'object','additionalProperties':{'type':'string'}})
    return tool


def proof(session, index):
    raw = (FIXTURES/('control-'+str(index)+'.input')).read_bytes().strip()
    assert raw.startswith(b'[') and raw.endswith(b']')
    result = session.request('tools/call', {'name': TOOL}, raw[1:-1])['result']
    assert result.get('isError', False) is False
    want = json.loads((FIXTURES/('control-'+str(index)+'.stdout')).read_bytes())[0]
    exact(result['structuredContent'], want)
    assert len(result['content']) == 1 and result['content'][0]['type'] == 'text'
    exact(json.loads(result['content'][0]['text']), want)
    assert len(want['bindings']['assets']) == 89
    return {'fixture': index, 'status': want['decision']['status'],
            'proof_sha256': hashlib.sha256(encoded(want)).hexdigest()}


def main():
    binary_sha = hashlib.sha256(BINARY.read_bytes()).hexdigest()
    origin = json.loads((FIXTURES/'origin.json').read_bytes())
    for reference in origin['references']:
        raw = (FIXTURES/reference['path']).read_bytes()
        assert len(raw) == reference['bytes'] and hashlib.sha256(raw).hexdigest() == reference['sha256']
    build_identity = None
    if BUILD_INFO is not None:
        build_identity = json.loads(BUILD_INFO.read_bytes())
        assert build_identity['GoVersion'] == 'go1.27.3'
        assert build_identity['Main']['Path'] == 'github.com/kimjooyoon/gooo-go-ports/examples/service-grant-mcp'
        assert 'Replace' not in build_identity['Main']
        deps = {row['Path']:row for row in build_identity['Deps']}
        assert deps[origin['runtime_module']]['Version'] == origin['runtime_version']
        assert deps[origin['runtime_module']]['Sum'] == origin['runtime_sum']
        sdk = deps['github.com/modelcontextprotocol/go-sdk']
        assert sdk['Version'] == 'v1.8.0' and sdk['Sum'] == 'h1:KIvahhYqwtbeniWVPs3TcXEA7b8jEtwfBpOTAI+Urx4='
        assert all('Replace' not in row for row in deps.values())
        settings = {row['Key']:row['Value'] for row in build_identity['Settings']}
        assert settings['CGO_ENABLED'] == '0'
    observations = []
    for label, protocol in [('legacy-session', '2025-11-25'), ('current-session', '2026-07-28')]:
        session = Session(label, protocol)
        failure = None
        try:
            if protocol == '2025-11-25':
                init = session.request('initialize', {'protocolVersion': protocol,
                    'capabilities': {}, 'clientInfo': {'name': 'gooo-independent-wire-trial', 'version': '0'}})
                assert init['result']['protocolVersion'] == protocol
                exact(init['result']['serverInfo'], {'name':'gooo-service-grant','version':'0.1.0-dev'})
                session.notify_initialized()
            else:
                discovered = session.request('server/discover')['result']
                assert protocol in discovered['supportedVersions']
                exact(discovered['_meta']['io.modelcontextprotocol/serverInfo'], {'name':'gooo-service-grant','version':'0.1.0-dev'})
            tool = listed(session)
            # Construct an actual call from discovery metadata, not a second
            # handwritten example. It must equal the retained native fixture.
            example = {name: field['examples'][0]
                       for name, field in tool['inputSchema']['properties'].items()}
            expected_example = json.loads((FIXTURES/'control-1.input').read_bytes())[0]
            exact(example, expected_example)
            example_result = session.request('tools/call', {'name': TOOL}, encoded(example))['result']
            expected_proof = json.loads((FIXTURES/'control-1.stdout').read_bytes())[0]
            assert not example_result.get('isError', False)
            exact(example_result['structuredContent'], expected_proof)
            cases = [proof(session, i) for i in [1,2,3,1]]
            assert cases[0] == cases[3]
            invalid = [
                ('duplicate-key', (FIXTURES/'control-4.input').read_bytes().strip()[1:-1]),
                ('missing-field', b'{"before":"*","after":"mcp://*","required_type":"mcp"}'),
                ('extra-field', b'{"before":"*","after":"mcp://*","required_type":"mcp","required_name":"weather","decision":"ELIGIBLE"}'),
                ('null', b'{"before":"*","after":"mcp://*","required_type":"mcp","required_name":null}'),
                ('wrong-type', b'{"before":"*","after":"mcp://*","required_type":"mcp","required_name":1}'),
                ('unpaired-surrogate', b'{"before":"*","after":"mcp://*","required_type":"mcp","required_name":"\\ud800"}'),
                ('array', b'[]'),
                ('missing-arguments', None),
            ]
            refused = []
            for name, arguments in invalid:
                result = session.request('tools/call', {'name': TOOL}, arguments)['result']
                assert result['isError'] is True and 'structuredContent' not in result
                assert result['content'][0]['text'].startswith('INVALID_FACTS:')
                refused.append(name)
            unknown = session.request('tools/call', {'name': 'unknown_tool'}, b'{}')
            assert unknown['error']['code'] == -32602
            observations.append({'protocol': protocol, 'native_CLI_equal': cases,
                                 'discovery_example_native_CLI_equal': True,
                                 'strict_tool_errors': refused, 'unknown_tool_error': -32602,
                                 'input_schema': tool['inputSchema'], 'output_schema': tool['outputSchema']})
        except BaseException as error:
            failure = repr(error)
            raise
        finally:
            session.close(failure)
    assert hashlib.sha256(BINARY.read_bytes()).hexdigest() == binary_sha
    result = {'status': 'INDEPENDENT_WIRE_TRIAL_PASS', 'observations': observations,
              'binary_sha256':binary_sha, 'build_identity':build_identity,
              'sdk_client_calls': 0, 'model_calls': 0, 'external_adoption': False,
              'scope': 'Private maintainer-owned finite local stdio interoperability; not conformance for every MCP feature or client.'}
    (OUTPUT/'verification.json').write_text(json.dumps(result, indent=2)+'\n')
    print(json.dumps({'status': result['status'], 'protocols': len(observations),
                      'native_CLI_proof_comparisons': 8, 'strict_tool_errors': 16}))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--build-info', type=Path)
    args = parser.parse_args()
    BINARY = args.binary.resolve(strict=True)
    OUTPUT = args.out.absolute()
    BUILD_INFO = args.build_info.resolve(strict=True) if args.build_info is not None else None
    OUTPUT.mkdir(mode=0o700)
    try:
        main()
    except BaseException as error:
        (OUTPUT/'first-failure.json').write_text(json.dumps({'status':'FAIL','error':repr(error)})+'\n')
        raise
