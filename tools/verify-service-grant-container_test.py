"""Offline lifecycle regressions; no Docker daemon or native command is run."""
import copy
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tempfile
import unittest
import types
from unittest import mock

SUBJECT = Path(__file__).with_name("verify-service-grant-container.py")

class CreateReplyLost(RuntimeError):
    pass

class CleanupUnavailable(RuntimeError):
    pass

class FakeTrial:
    def __init__(self, out, primary, cleanup_error=None):
        self.out, self.image, self.context = out, "sha256:" + "a" * 64, out
        self.docker, self.label = "/not-a-real-docker", "io.gooo.test=fake"
        self.primary, self.cleanup_error = primary, cleanup_error
        self.removal_names, self.absence_names = [], []
        self.create_name = None

    def command(self, argv, accept_failure=False):
        if argv[:2] == ["container", "create"]:
            self.create_name = argv[argv.index("--name") + 1]
            intent = json.loads((self.out / "default/intent.json").read_text())
            assert intent["container_name"] == self.create_name
            # The daemon may have created it even though no CID was returned.
            raise self.primary
        if argv[:3] == ["container", "rm", "--force"]:
            self.removal_names.append(argv[3])
            assert argv[3] == self.create_name
            if self.cleanup_error is not None:
                raise self.cleanup_error
            return 0, (self.create_name + "\n").encode(), b""
        raise AssertionError("Unexpected fake command: " + repr(argv))

    def removed(self, name):
        self.absence_names.append(name)
        assert name == self.create_name
        if self.cleanup_error is not None:
            raise self.cleanup_error

    def inspect(self, name):
        raise AssertionError("No CID returned; only recovery-name removal is expected")

class ConstructorCleanupTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="fake-container-", dir=Path(__file__).parent)
        self.addCleanup(self.temp.cleanup)
        self.out = Path(self.temp.name)
        # Defense against accidentally launching any real Docker/native child.
        self.no_children = mock.patch("subprocess.Popen", side_effect=AssertionError("real subprocess forbidden"))
        self.no_children.start()
        self.addCleanup(self.no_children.stop)
        spec = importlib.util.spec_from_file_location("reviewed_container_smoke", SUBJECT)
        self.subject = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.subject)

    def test_lost_create_reply_removes_by_durable_name_and_preserves_primary(self):
        primary = CreateReplyLost("injected create reply lost")
        trial = FakeTrial(self.out, primary)
        with self.assertRaises(CreateReplyLost) as caught:
            self.subject.ContainerSession(trial, "default")
        self.assertIs(caught.exception, primary)
        self.assertEqual(trial.removal_names, [trial.create_name])
        self.assertEqual(trial.absence_names, [trial.create_name])
        result = json.loads((self.out / "default/result.json").read_text())
        self.assertIs(result["container_absence_verified"], True)
        self.assertIn("create", json.dumps(result["first_failure"]).lower())

    def test_cleanup_failure_cannot_replace_create_primary_or_claim_absence(self):
        primary = CreateReplyLost("injected create reply lost")
        cleanup = CleanupUnavailable("injected cleanup daemon unavailable")
        trial = FakeTrial(self.out, primary, cleanup)
        with self.assertRaises(CreateReplyLost) as caught:
            self.subject.ContainerSession(trial, "default")
        self.assertIs(caught.exception, primary)
        self.assertEqual(trial.removal_names, [trial.create_name])
        result = json.loads((self.out / "default/result.json").read_text())
        self.assertIsNot(result.get("container_absence_verified"), True)
        self.assertIn("injected create reply lost", json.dumps(result["first_failure"]))
        self.assertIn("injected cleanup daemon unavailable", json.dumps(result["cleanup_failure"]))

    def test_signal_during_cleanup_is_latched_until_after_removal(self):
        primary = CreateReplyLost("injected create reply lost")
        trial = FakeTrial(self.out, primary)
        original = trial.removed
        def removal_with_signal(name):
            self.subject.PENDING_SIGNAL = 15
            original(name)
        trial.removed = removal_with_signal
        with self.assertRaises(CreateReplyLost):
            self.subject.ContainerSession(trial, "default")
        result = json.loads((self.out / "default/result.json").read_text())
        self.assertTrue(result["container_absence_verified"])
        with self.assertRaises(KeyboardInterrupt):
            self.subject.checkpoint()

    def test_diagnostic_write_failure_keeps_first_setup_error_and_closes_pipes(self):
        stdout, stderr = io.BytesIO(), io.BytesIO()
        child = types.SimpleNamespace(pid=99999999, stdout=stdout, stderr=stderr,
                                      returncode=0, wait=lambda **kwargs: 0)
        trial = self.subject.Trial("/not-docker", "sha256:" + "a" * 64,
                                   self.out, self.out, "io.gooo.test=fake")
        with mock.patch.object(self.subject.subprocess, "Popen", return_value=child), \
             mock.patch.object(self.subject.os, "killpg", side_effect=ProcessLookupError), \
             mock.patch.object(Path, "write_bytes", side_effect=OSError("diagnostic disk failure")):
            with self.assertRaises(io.UnsupportedOperation) as caught:
                trial.command(["fake"])
        self.assertTrue(stdout.closed and stderr.closed)
        record = json.loads((self.out / "commands/1/process.json").read_text())
        self.assertIn("UnsupportedOperation", record["failure"])
        self.assertTrue(any("diagnostic disk failure" in note for note in record["secondary_diagnostic_errors"]))

class RegistryLaunchInputTests(unittest.TestCase):
    def subject(self):
        spec = importlib.util.spec_from_file_location('registry_launch_test', SUBJECT)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        return module

    def test_actual_descriptor_drives_fixed_reference_and_runtime_argv(self):
        module = self.subject()
        package = module.registry_package(SUBJECT.parents[1] / 'registry/service-grant-preview16.server.json')
        self.assertEqual(package['runtime_argv'], ['--network', 'none', '--read-only', '--platform', 'linux/amd64'])
        trial = types.SimpleNamespace(image='sha256:' + 'a' * 64, label='io.gooo.test=registry', registry_package=package)
        argv = module.create_arguments(trial, 'owned-name')
        self.assertEqual(argv[-1], package['identifier'])
        self.assertIn('@sha256:a83d8fa57657bb877453cce7ed11f1ff1000a84b1e80c65a4e1c0b20196d3ee4', argv[-1])
        self.assertEqual(argv[argv.index('--network') + 1], 'none')
        self.assertEqual(argv[argv.index('--platform') + 1], 'linux/amd64')
        self.assertIn('--read-only', argv)
        self.assertNotIn(trial.image, argv)

    def test_changed_descriptor_is_refused_without_a_subprocess(self):
        module = self.subject()
        with tempfile.TemporaryDirectory(dir=SUBJECT.parent) as temporary:
            path = Path(temporary) / 'changed.json'
            raw = (SUBJECT.parents[1] / 'registry/service-grant-preview16.server.json').read_bytes()
            path.write_bytes(raw.replace(b'"none"', b'"host"'))
            with mock.patch('subprocess.Popen', side_effect=AssertionError('must not spawn')):
                with self.assertRaisesRegex(RuntimeError, 'descriptor bytes changed'):
                    module.registry_package(path)


class AliasProofTests(unittest.TestCase):
    """Exercise the real proof checker without any Docker or native child.

    The fixed documented example is a transport fixture, not another alias
    comparison implementation. Mutations change complete received reports,
    including matching text copies, so passing cannot depend on selected fields.
    """
    def setUp(self):
        self.no_children = mock.patch('subprocess.Popen',
            side_effect=AssertionError('real subprocess forbidden'))
        self.no_children.start()
        self.addCleanup(self.no_children.stop)
        spec = importlib.util.spec_from_file_location('alias_container_test', SUBJECT)
        self.subject = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.subject)
        module = SUBJECT.parents[1] / 'examples/alias-origin-change'
        raw = b'  ' + (module / 'example-input.json').read_bytes().strip() + b' '
        facts = json.loads(raw)
        fact_bytes = json.dumps(facts, separators=(',', ':')).encode()
        digest = 'sha256:' + hashlib.sha256(fact_bytes).hexdigest()
        proof = {
            'facts': facts, 'facts_sha256': digest,
            'input_sha256': 'sha256:' + hashlib.sha256(raw).hexdigest(),
            'decision': {'context_id': 'gooo/alias-origin/v1/' + digest,
                'state': 'ORIGIN_CHANGED', 'reason': 'DIFFERENT_DECLARED_ORIGIN',
                'before_index': 0, 'after_index': 0, 'conflict_catalog': '',
                'conflict_first_index': -1, 'conflict_second_index': -1,
                'execution_authorized': False},
            'bindings': json.loads((module / 'generation.json').read_bytes()),
        }
        rejected = b'{"target_alias":"docs_search","before":null,"after":[]}'
        diagnostic = {'state': 'INVALID_TRANSPORT', 'stage': 'decode_json',
                      'code': 'INVALID_JSON_FACTS'}
        self.reference = {
            'schemas': {name: json.loads((module / 'schemas' / (name + '.schema.json')).read_bytes())
                        for name in ('input', 'output', 'diagnostic')},
            'cases': [('origin-changed', raw, proof)],
            'malformed': [('null-catalog', rejected, diagnostic)],
            'evidence': {},
        }

    def session(self, mode='alias', protocol='2025-11-25', mutate=None):
        test = self
        reference = self.reference

        class FakeSession:
            def __init__(self):
                self.mode, self.protocol = mode, protocol
                self.label = mode + '-' + protocol
                self.calls = []
                self.planned = [('proof', raw, wanted) for _, raw, wanted in reference['cases']]
                self.planned += [('error', raw, wanted) for _, raw, wanted in reference['malformed']]
                self.planned += [('repeat', reference['cases'][0][1], reference['cases'][0][2])]
                self.position = 0

            def request(self, method, params=None, arguments=None):
                self.calls.append((method, copy.deepcopy(params), arguments))
                if method == 'tools/list':
                    test.assertIsNone(arguments)
                    tools = [{'name': 'assess_service_grant'}]
                    if mode == 'both':
                        tools.append({'name': 'assess_service_grant_change'})
                    tools.append({'name': 'gooo_compare_alias_origin',
                        'description': 'Compare the supplied declared origins.',
                        'inputSchema': copy.deepcopy(reference['schemas']['input']),
                        'outputSchema': copy.deepcopy(reference['schemas']['output']),
                        'annotations': {'readOnlyHint': True, 'idempotentHint': True,
                                        'openWorldHint': False}})
                    kind, result = 'discovery', {'tools': tools}
                else:
                    test.assertEqual(method, 'tools/call')
                    test.assertEqual(params['name'], 'gooo_compare_alias_origin')
                    test.assertLess(self.position, len(self.planned))
                    kind, wanted_raw, wanted = self.planned[self.position]
                    self.position += 1
                    test.assertEqual(arguments, wanted_raw, 'the original argument bytes changed')
                    if kind == 'error':
                        result = {'isError': True, 'content': [{'type': 'text',
                            'text': json.dumps(wanted, separators=(',', ':'))}]}
                    else:
                        result = {'structuredContent': copy.deepcopy(wanted),
                            'content': [{'type': 'text', 'text': json.dumps(wanted, separators=(',', ':'))}]}
                    if protocol == '2026-07-28':
                        result['resultType'] = 'complete'
                if protocol == '2026-07-28':
                    result['_meta'] = {'io.modelcontextprotocol/serverInfo': {
                        'name': 'gooo-service-grant', 'version': '0.1.0-dev'}}
                response = {'jsonrpc': '2.0', 'id': len(self.calls), 'result': result}
                if mutate is not None:
                    mutate(kind, response)
                return response

        return FakeSession()

    def test_complete_reports_and_raw_inputs_survive_both_modes_and_protocols(self):
        for mode in ('alias', 'both'):
            for protocol in ('2025-11-25', '2026-07-28'):
                with self.subTest(mode=mode, protocol=protocol):
                    session = self.session(mode, protocol)
                    self.subject.alias_proofs(session, self.reference)
                    self.assertEqual(session.position, len(session.planned))
                    self.assertEqual([call[0] for call in session.calls],
                        ['tools/list', 'tools/call', 'tools/call', 'tools/call'])

    def test_changed_complete_proof_is_rejected_even_when_text_agrees(self):
        def change(path, value):
            def mutate(proof):
                target = proof
                for key in path[:-1]:
                    target = target[key]
                target[path[-1]] = value
            return mutate

        mutations = {
            'declared-origin': change(('facts', 'after', 0, 'server'), 'other-server'),
            'received-bytes': change(('input_sha256',), 'sha256:' + '0' * 64),
            'facts-digest': change(('facts_sha256',), 'sha256:' + '1' * 64),
            'context': change(('decision', 'context_id'), 'gooo/alias-origin/v1/sha256:' + '2' * 64),
            'reason': change(('decision', 'reason'), 'SAME_DECLARED_ORIGIN'),
            'original-index': change(('decision', 'before_index'), 1),
            'boolean-index': change(('decision', 'before_index'), True),
            'authority': change(('decision', 'execution_authorized'), True),
            'source-binding': change(('bindings', 'source', 'ports/tool-origin/alias_origin.gooo'), 'sha256:' + '3' * 64),
            'generation-binding': change(('bindings', 'generator_sha256'), 'sha256:' + '4' * 64),
            'missing-field': lambda proof: proof['decision'].pop('conflict_catalog'),
        }
        for name, alter in mutations.items():
            with self.subTest(mutation=name):
                def mutate(kind, response):
                    if kind == 'proof':
                        result = response['result']
                        alter(result['structuredContent'])
                        result['content'][0]['text'] = json.dumps(result['structuredContent'])
                with self.assertRaises(AssertionError):
                    self.subject.alias_proofs(self.session(mutate=mutate), self.reference)

    def test_changed_text_copy_is_rejected(self):
        def mutate(kind, response):
            if kind == 'proof':
                text = json.loads(response['result']['content'][0]['text'])
                text['decision']['state'] = 'UNCHANGED'
                response['result']['content'][0]['text'] = json.dumps(text)
        with self.assertRaises(AssertionError):
            self.subject.alias_proofs(self.session(mutate=mutate), self.reference)

    def test_discovery_must_keep_exact_schema_and_required_tools(self):
        for alteration in ('input-schema', 'output-schema', 'missing-tool', 'extra-tool', 'writable-tool'):
            with self.subTest(alteration=alteration):
                def mutate(kind, response):
                    if kind != 'discovery':
                        return
                    tools = response['result']['tools']
                    alias = tools[-1]
                    if alteration == 'input-schema':
                        alias['inputSchema']['additionalProperties'] = True
                    elif alteration == 'output-schema':
                        alias['outputSchema']['required'].remove('bindings')
                    elif alteration == 'missing-tool':
                        tools.pop(0)
                    elif alteration == 'extra-tool':
                        tools.append({'name': 'unreviewed_tool'})
                    else:
                        alias['annotations']['readOnlyHint'] = False
                with self.assertRaises(AssertionError):
                    self.subject.alias_proofs(self.session('both', mutate=mutate), self.reference)

    def test_refusal_cannot_include_a_proof_or_changed_diagnostic(self):
        for alteration in ('partial-proof', 'changed-diagnostic', 'unknown-diagnostic', 'not-error'):
            with self.subTest(alteration=alteration):
                def mutate(kind, response):
                    if kind != 'error':
                        return
                    result = response['result']
                    if alteration == 'partial-proof':
                        result['structuredContent'] = copy.deepcopy(self.reference['cases'][0][2])
                    elif alteration == 'not-error':
                        result['isError'] = False
                    else:
                        diagnostic = {'state': 'INVALID_TRANSPORT', 'stage': 'decode_facts',
                            'code': 'TEXT_LIMIT_OR_ENCODING' if alteration == 'changed-diagnostic' else 'INVENTED_CODE'}
                        result['content'][0]['text'] = json.dumps(diagnostic)
                with self.assertRaises(AssertionError):
                    self.subject.alias_proofs(self.session(protocol='2026-07-28', mutate=mutate), self.reference)

    def test_first_valid_call_after_error_must_still_match(self):
        def mutate(kind, response):
            if kind == 'repeat':
                report = response['result']['structuredContent']
                report['bindings']['scope'] = 'changed after failed input'
                response['result']['content'][0]['text'] = json.dumps(report)
        with self.assertRaises(AssertionError):
            self.subject.alias_proofs(self.session(mutate=mutate), self.reference)

    def test_current_protocol_completion_and_server_metadata_are_required(self):
        for alteration in ('missing-completion', 'wrong-completion', 'wrong-server'):
            with self.subTest(alteration=alteration):
                def mutate(kind, response):
                    if kind != 'proof':
                        return
                    result = response['result']
                    if alteration == 'missing-completion':
                        result.pop('resultType')
                    elif alteration == 'wrong-completion':
                        result['resultType'] = 'deferred'
                    else:
                        result['_meta']['io.modelcontextprotocol/serverInfo']['name'] = 'another-server'
                with self.assertRaises(AssertionError):
                    self.subject.alias_proofs(self.session(protocol='2026-07-28', mutate=mutate), self.reference)

    def test_bundle_schema_selection_rejects_unknown_duplicate_and_oversized_json(self):
        with tempfile.TemporaryDirectory() as temporary:
            context = Path(temporary)
            (context / 'runtime').mkdir()
            path = context / 'runtime/BUNDLE.json'
            for schema in (self.subject.V2_SCHEMA, self.subject.V3_SCHEMA):
                path.write_text(json.dumps({'schema': schema}))
                self.assertEqual(self.subject.bundle_schema(context), schema)
            for raw in (b'{"schema":"invented/v4"}',
                        ('{"schema":"' + self.subject.V2_SCHEMA + '","schema":"' + self.subject.V3_SCHEMA + '"}').encode(),
                        b' ' * ((512 << 10) + 1)):
                path.write_bytes(raw)
                with self.assertRaises(AssertionError):
                    self.subject.bundle_schema(context)

    def test_alias_reference_requires_absolute_inputs_and_a_full_saved_trial(self):
        with tempfile.TemporaryDirectory() as temporary:
            folder = Path(temporary)
            inputs = [folder, folder / 'acceptance.json', folder / 'module', folder / 'cli', folder / 'go']
            with mock.patch.object(self.subject.ALIAS, 'validate_saved') as validate:
                for index in range(len(inputs)):
                    for bad in (None, Path('relative')):
                        changed = inputs.copy()
                        changed[index] = bad
                        with self.assertRaises(AssertionError):
                            self.subject.load_alias_reference(*changed)
                changed = inputs.copy()
                changed[1] = folder / 'another-report.json'
                with self.assertRaises(AssertionError):
                    self.subject.load_alias_reference(*changed)
                validate.assert_not_called()
            primary = AssertionError('saved full reference rejected')
            with mock.patch.object(self.subject.ALIAS, 'validate_saved', side_effect=primary) as validate:
                with self.assertRaises(AssertionError) as caught:
                    self.subject.load_alias_reference(*inputs)
                self.assertIs(caught.exception, primary)
                validate.assert_called_once_with(folder,
                    folder / 'runtime/gooo-service-grant-mcp', folder / 'cli', folder / 'module',
                    folder / 'go', self.subject.ROOT / 'examples/service-grant-mcp/verify.py',
                    expected_scope='full')

    def test_saved_reference_capture_tampering_after_validation_is_rejected(self):
        # Model only the prior validator's returned snapshot. The helper under
        # test really reopens and hashes the raw captures; no CLI is substituted
        # for the upstream validation or launched from this test.
        with tempfile.TemporaryDirectory() as temporary:
            folder = Path(temporary)
            report_path = folder / 'acceptance.json'
            report_path.write_text('{"scope":"full"}')
            module = SUBJECT.parents[1] / 'examples/alias-origin-change'
            normal_name, normal_raw, proof = self.reference['cases'][0]
            bad_name, bad_raw, diagnostic = self.reference['malformed'][0]
            captures = {
                normal_name: {'stdin': normal_raw, 'stdout': json.dumps(proof).encode(), 'stderr': b''},
                bad_name: {'stdin': bad_raw, 'stdout': b'', 'stderr': json.dumps(diagnostic).encode()},
            }
            records = []
            for name, channels in captures.items():
                directory = folder / ('reference-' + name)
                directory.mkdir()
                record = {'label': directory.name}
                for channel, raw in channels.items():
                    (directory / channel).write_bytes(raw)
                    record[channel] = {'bytes': len(raw), 'sha256': hashlib.sha256(raw).hexdigest()}
                records.append(record)
            report = {'processes': records, 'binary': {'sha256': '0' * 64},
                      'alias_cli': {'sha256': '1' * 64}}
            inputs = [folder, report_path, module, folder / 'cli', folder / 'go']
            selected = ([(normal_name, normal_raw, proof['decision']['state'])], [(bad_name, bad_raw)])
            with mock.patch.object(self.subject.ALIAS, 'validate_saved', return_value=report), \
                 mock.patch.object(self.subject.ALIAS, 'trial_inputs', return_value=selected):
                loaded = self.subject.load_alias_reference(*inputs)
                self.assertEqual(loaded['cases'], self.reference['cases'])
                self.assertEqual(loaded['malformed'], self.reference['malformed'])
                for name, channels in captures.items():
                    for channel, raw in channels.items():
                        with self.subTest(case=name, channel=channel):
                            path = folder / ('reference-' + name) / channel
                            path.write_bytes(raw + b' ')
                            try:
                                with self.assertRaises(AssertionError):
                                    self.subject.load_alias_reference(*inputs)
                            finally:
                                path.write_bytes(raw)

    def test_nonblocking_input_retries_and_leaves_success_recording_to_client(self):
        session = types.SimpleNamespace(started=100, sent=bytearray(), read=mock.Mock())
        stream = types.SimpleNamespace(fileno=lambda: 42, close=mock.Mock())
        with mock.patch.object(self.subject.os, 'set_blocking') as set_blocking, \
             mock.patch.object(self.subject.os, 'write', side_effect=[BlockingIOError(), 2, 4]) as write, \
             mock.patch.object(self.subject.time, 'monotonic', return_value=101):
            wrapped = self.subject.AliasContainerInput(session, stream)
            self.assertEqual(wrapped.write(b'abcdef'), 6)
            wrapped.flush()
            wrapped.close()
        set_blocking.assert_called_once_with(42, False)
        self.assertEqual(write.call_args_list, [mock.call(42, b'abcdef'),
                         mock.call(42, b'abcdef'), mock.call(42, b'cdef')])
        session.read.assert_called_once_with()
        self.assertEqual(session.sent, b'')
        stream.close.assert_called_once_with()

    def test_nonblocking_input_preserves_only_delivered_prefix_and_original_error(self):
        primary = RuntimeError('injected read failure after partial stdin')
        session = types.SimpleNamespace(started=100, sent=bytearray(b'previous'),
                                        read=mock.Mock(side_effect=primary))
        stream = types.SimpleNamespace(fileno=lambda: 42)
        with mock.patch.object(self.subject.os, 'set_blocking'), \
             mock.patch.object(self.subject.os, 'write', side_effect=[2, BlockingIOError()]) as write, \
             mock.patch.object(self.subject.time, 'monotonic', return_value=101):
            wrapped = self.subject.AliasContainerInput(session, stream)
            with self.assertRaises(RuntimeError) as caught:
                wrapped.write(b'abcdef')
        self.assertIs(caught.exception, primary)
        self.assertEqual(session.sent, b'previousab')
        self.assertEqual(write.call_args_list, [mock.call(42, b'abcdef'), mock.call(42, b'cdef')])

    def test_nonblocking_input_deadline_stops_before_another_write(self):
        session = types.SimpleNamespace(started=100, sent=bytearray(), read=mock.Mock())
        stream = types.SimpleNamespace(fileno=lambda: 42, close=mock.Mock())
        with mock.patch.object(self.subject.os, 'set_blocking'), \
             mock.patch.object(self.subject.os, 'write', side_effect=BlockingIOError()) as write, \
             mock.patch.object(self.subject.time, 'monotonic', side_effect=[101, 131]):
            wrapped = self.subject.AliasContainerInput(session, stream)
            with self.assertRaisesRegex(AssertionError, 'alias container stdin deadline'):
                wrapped.write(b'abcdef')
            wrapped.close()
        write.assert_called_once_with(42, b'abcdef')
        session.read.assert_called_once_with()
        self.assertEqual(session.sent, b'')
        stream.close.assert_called_once_with()

    def test_alias_stdout_limits_retain_only_one_overflow_sentinel(self):
        for prior_stream, prior_total, message in (
                (512 << 10, 512 << 10, 'alias container stream limit'),
                (0, 1 << 20, 'combined alias container stdout limit')):
            with self.subTest(prior_stream=prior_stream, prior_total=prior_total):
                session = object.__new__(self.subject.ContainerSession)
                session.mode, session.cleaning, session.started = 'alias', False, 100
                session.buffers = {'stdout': bytearray(b'x' * prior_stream), 'stderr': bytearray()}
                session.linebuf = bytearray()
                session.trial = types.SimpleNamespace(alias_stdout_bytes=prior_total)
                key = types.SimpleNamespace(fd=42, data='stdout', fileobj=None)
                session.selector = types.SimpleNamespace(select=mock.Mock(return_value=[(key, 1)]))
                with mock.patch.object(self.subject.os, 'read', return_value=b'!') as read, \
                     mock.patch.object(self.subject.time, 'monotonic', return_value=101):
                    with self.assertRaisesRegex(AssertionError, message):
                        session.read()
                read.assert_called_once_with(42, 1)
                self.assertEqual(len(session.buffers['stdout']), prior_stream + 1)
                self.assertEqual(session.linebuf, b'!')
                self.assertEqual(session.trial.alias_stdout_bytes, prior_total + 1)


class SnapshotSavedCaptureTests(unittest.TestCase):
    """Isolate retained-byte reads; the policy validator is deliberately mocked."""
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup)
        self.root=Path(self.temp.name)
        spec=importlib.util.spec_from_file_location('snapshot_container_test',SUBJECT)
        self.subject=importlib.util.module_from_spec(spec);spec.loader.exec_module(self.subject)
        self.no_children=mock.patch('subprocess.Popen',side_effect=AssertionError('real subprocess forbidden'))
        self.no_children.start();self.addCleanup(self.no_children.stop)
        self.context=self.root/'context';self.context.mkdir()
        self.report_path=self.root/'acceptance.json';self.report_path.write_text('{}')
        self.raw=b'{"synthetic":"capture-reader-only"}'
        self.normal=[('same',self.raw,'OBSERVED_DEFINITION_SAME')]
        self.malformed=[('bad',b'null')]
        self.report={'schemas':{'input':{},'output':{}},'binary':{'sha256':'0'*64},
                     'snapshot_cli':{'sha256':'1'*64},'reference_processes':[]}
        for name,stdin,stdout,stderr in [
                ('same',self.raw,b'{"decision":{"state":"OBSERVED_DEFINITION_SAME"}}',b''),
                ('bad',b'null',b'',b'{"code":"INVALID_JSON_FACTS"}')]:
            folder=self.root/('reference-'+name);folder.mkdir()
            record={}
            for channel,raw in [('stdin',stdin),('stdout',stdout),('stderr',stderr)]:
                (folder/channel).write_bytes(raw)
                record[channel]={'bytes':len(raw),'sha256':self.subject.sha(raw)}
            self.report['reference_processes'].append({'label':'reference-'+name,'record':record})
        self.cases=mock.patch.object(self.subject.SNAPSHOT,'cases',return_value=(self.normal,self.malformed))
        self.cases.start();self.addCleanup(self.cases.stop)

    def read(self):
        return self.subject.load_snapshot_reference(self.context,self.report_path,
            self.root/'module',self.root/'compare',self.root/'go')

    def test_saved_capture_reader_uses_original_bytes_without_producer(self):
        with mock.patch.object(self.subject.SNAPSHOT,'validate_saved',return_value=self.report):
            result=self.read()
        self.assertEqual(result['cases'][0][1],self.raw)
        self.assertEqual(len(result['malformed']),1)

    def test_capture_change_after_validation_cannot_become_new_reference(self):
        def validate(*args):
            (self.root/'reference-same/stdout').write_bytes(
                b'{"decision":{"state":"OBSERVED_DEFINITION_SAME","extra":"late change"}}')
            return self.report
        with mock.patch.object(self.subject.SNAPSHOT,'validate_saved',side_effect=validate):
            with self.assertRaisesRegex(AssertionError,'capture changed'):
                self.read()

    def test_v4_reference_paths_must_be_absolute(self):
        with self.assertRaises(AssertionError):
            self.subject.load_snapshot_reference(self.context,Path('relative/acceptance.json'),
                self.root/'module',self.root/'compare',self.root/'go')


if __name__ == "__main__":
    unittest.main()
