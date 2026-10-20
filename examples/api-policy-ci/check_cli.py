#!/usr/bin/env python3
"""Focused runtime checks for the fixed-profile CI Action.

Root supplies ONE already-built absolute binary; this driver never builds,
downloads, invokes a shell, calls Go/Gooo or runs the Action entrypoint. Expected
policy fields/bindings come from the two immutable public API commits listed in
MODULES and their frozen source bindings, never from this wrapper's first output.
"""
import copy
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import stat
import subprocess
import sys
import tempfile
import unittest

COMPILER = '205896a413e5dea92366309c35ae85e542d055f6'
ORIGINAL_BINDINGS = {'assets': {'adapter_generated.go': 'sha256:db23ed9739db691f80aedde15096eca0e36a8b3ecdc6cadf8c2b86414cec40ec',
            'bindings.go': 'sha256:a0836884bf4d0d25af5aa0c0265c859f09f4187950f72e859684a6f3bf1866e1',
            'bridge.go': 'sha256:bc6081d796f6084f628e59b9d3ed24bee799acaa96f72e522dcdc3ce823bd99b',
            'canonical/cel-api-policy.workspace.json': 'sha256:5f7e349217dbbe31129c80dd8a273d391943665998dc312f3726931d2da0a152',
            'canonical/cel-scalar.workspace.json': 'sha256:6ffa0b794dbd35fc23e7d6ec2886ab15471a8a5fb5ab2de52e94e03c08b5552a',
            'canonical/main.gooo': 'sha256:72d5f8542c35bb610d094648befa98bbd4c215cbbaac138e18cbb88f58932f53',
            'canonical/scalar_access.gooo': 'sha256:86a237641bd7b4de2722a00e1dd89e9f28d17ea953663e9ef1e2a435959c2b83',
            'canonical/source-pin.json': 'sha256:8e81cfee5a7253080263d2f0cc0ff2ab5a5d3c076a6ebf6f23c00936a04f799a',
            'cmd/evaluate/main.go': 'sha256:082425a7190766c6f7401a3324b50717f48712d4fa0d08ca372f914ac1348cf4',
            'cmd/host/main.go': 'sha256:7772b2b245c024d183df192db9fe3f2905aee5fe5cb189e76d073c80af9b105a',
            'compiler.json': 'sha256:9e61c23a1302e7390ba03af0606066a8cca1b13d5bd96864dc3c27a85c00dd5c',
            'generate.py': 'sha256:bb548961aadb7a0136ab87df95692dd72903546c31622ca38b6445222758349b',
            'input.go': 'sha256:096ccc6505b0a50c83383b13442583e23d596eae5d2bc5d0ec10fd2a7516b585',
            'internal/api/generated.go': 'sha256:3adc29c580be67955f4a250682b01db9e5f2690b4ceb4c818e5116b8cca840c8',
            'internal/scalar/generated.go': 'sha256:0accfc319e4ee132545f783b02fa3bc6b7de6d8cba119c3f8f1fa32dd675134a',
            'model.go': 'sha256:d4ad363d8468b7f502bc3559522af4902d40735cf4e142448354171249c6104c',
            'proof_input.go': 'sha256:49d664c7d38dc3fc39e3b90cf2c3a17dfb1ed36e20d24cca4346887cccb7935d',
            'receipts/api-body-codegen.json': 'sha256:5d011dd92288c8a3d2d7a0aa92298220bb076f7ff11307cffb26839bcfca7cf5',
            'receipts/api-execution.json': 'sha256:76db4fd299d5525f83737b5e8081a4e4f5d4986ad36c99acea128712a1b332c8',
            'receipts/api-lowered.gooo': 'sha256:d5fa9f010d47ed13f7d5e599cf4ec143ba4706fecaeaa78c0c656c01797579c0',
            'receipts/api-replay.json': 'sha256:605562dfde334b3f8e5da96e97b806929f1751a74a589adb056cf2e2deb7ce80',
            'receipts/api-seed-inputs.json': 'sha256:27bd345f439b9d4558f62582c4b58be8abebcdd1e985a9e7fa50c4478fffb1e7',
            'receipts/scalar-body-codegen.json': 'sha256:07a2fd3d73bfb8636496f3c09e88856c1e5af13e01013faa9fae21f84bd23770',
            'receipts/scalar-execution.json': 'sha256:9c962f52d91fa322a4d068536ba8d7e6678befeb9813ff99469a299a0bd10f2e',
            'receipts/scalar-lowered.gooo': 'sha256:7e10de7f6cf882611ef98abef655466713ac46e71e5a52ddf3411c8bb94c2a9c',
            'receipts/scalar-replay.json': 'sha256:c2135621c100d083e9bf8fa17731d337999ce931c4ae0028b04cc17158d3fac5',
            'receipts/scalar-seed-inputs.json': 'sha256:bbbd6a1769a1563d065be3e618fe2c030ad6392a348e0e69273edc60a6887891',
            'replay.go': 'sha256:f74015214af3d4f484c3047986a285f30059cf99a8f345ce695dc70e0baea0db'},
 'compiler_source_sha': '83928fa4538245abe33b08549da47f8cbdc50ed2',
 'generation_sha256': 'sha256:79b76bda704cb63b9390802766a66ac72ba02c04ae135600cb05de6797c97d46'}
REVISION_BINDINGS = {'assets': {'adapter_generated.go': 'sha256:df5f55e3179042b916b6d7a2a995e67b9596646689994573c24b241c3458c32f',
            'bindings.go': 'sha256:e558886594e8f8a73c503a420671dce6c6556915eed1be255ed390fb9b4cfb63',
            'candidate.workspace.json': 'sha256:6e074357fe49fc0cf1f00b6087ada7ab06a77dfc0910d6d227c85a26d53c81d4',
            'canonical/main.gooo': 'sha256:72d5f8542c35bb610d094648befa98bbd4c215cbbaac138e18cbb88f58932f53',
            'canonical/scalar_access.gooo': 'sha256:86a237641bd7b4de2722a00e1dd89e9f28d17ea953663e9ef1e2a435959c2b83',
            'cmd/compare/diagnostics.go': 'sha256:b92014b24cf1fbb302374f483772151d409ceab756067b95fd7059939f127b2e',
            'cmd/compare/main.go': 'sha256:301776cba828e1c79d39aae1864820a2f248386155bcbc6c90ed65503d1e5c3e',
            'compare.go': 'sha256:f019ba56fc40ba9ac0ffa9d35abf9ecda20534ede38a3efba5db0981d8ec8833',
            'compare.gooo': 'sha256:5f040d255508ea42250d24a4b79cbbdacd8a4f11286b8110a2e0cbc9df89646a',
            'compiler.json': 'sha256:503d1daed25690a552b207f0f8b3a19ebff309cfe8127f72d43be50059a4c428',
            'differences.workspace.json': 'sha256:ba83133424c08ed42c66e6096355661dab596349e45f990c1bdc2fcefc827a12',
            'generate.py': 'sha256:ace2eb2eabf4c5e1748565bf9876e73b5e07e2c1ac073f53b807c519ee3bb7bc',
            'go.mod': 'sha256:728ec08e68827e0425913aeed5e392b59a24ef5b67011fe3e8469e0424d8d9e4',
            'go.sum': 'sha256:8d8a5521a600a18b71783559af58cd9d6b9970dcc8b71c425fd3ba6e4f46dc82',
            'input.go': 'sha256:4099d8be17ef64edacf21e8473165fbc8cf25383e6b9f8645bdeb04e78f09b07',
            'internal/candidate/generated.go': 'sha256:1b242047e9e9b8669eee2f21b381c428d88b5418cb083e03ba9c5d74b68f25a5',
            'internal/flags/generated.go': 'sha256:475c9d9286514a0ea1cff53617c584b17f547b1cee27463775dfbfc9b9dc9b11',
            'internal/summary/generated.go': 'sha256:e8baa03e737b34764a12748d2b1b9b7bc80f3b82fea67c861238fc77fb879f0f',
            'model.go': 'sha256:30d890bed9db04e8eeaa36925c7ed91ae2a46dd00567cd81859908ee2be6eb5e',
            'profiles.json': 'sha256:64f45692d67d627715e092bec76ec244249fe05169463b64836530be39af48bb',
            'readonly.gooo': 'sha256:5e84daa87bf0458982ea863eec7fb5d583b3c42de73402fc0fe2748afdc18fb8',
            'receipts/candidate-body-codegen.json': 'sha256:e298b5e9f7eba89cb7abddcc0566fdb638eea68094a885b2732a78c5f135f784',
            'receipts/candidate-execution.json': 'sha256:83d1dc12e4606b674597335478a58f68166b4b7086ae2d7ab906601aee51a236',
            'receipts/candidate-lowered.gooo': 'sha256:4523dc1474219ba2389ea9c9642da31e02fea77bc464e4d1cb53f656ae2fb24c',
            'receipts/candidate-replay.json': 'sha256:4913eb6c23cd25a8ad3bb7cb2ec8d593f28c44cea348c2fdb9f1a098e61e89be',
            'receipts/candidate-seed-inputs.json': 'sha256:8b326457ad404e4ba07f5da188afcbe9dd186dc78fc7a74a6d9833bcf7c6b324',
            'receipts/flags-body-codegen.json': 'sha256:b6ed2e1c800cd6ff4ef2cfeb7af7c24924feb7a020f68ac9fdbaeed6a0a7ca04',
            'receipts/flags-execution.json': 'sha256:74b4979834da42c6aef980517fba9bda01aa523cb3310d7bfbff6d400b9c3da8',
            'receipts/flags-lowered.gooo': 'sha256:22c33ac8aebbf31a3fb1312a1b9d090002df53c2f4cd8e29cf0248ba5eddc003',
            'receipts/flags-replay.json': 'sha256:ac36e2d9298ac1a7c17e02e2708a4a991a521dc2d12cdbadf8c1c63b1b5beb65',
            'receipts/flags-seed-inputs.json': 'sha256:85e99923d2182829c8c1ecd602eac70d478e5eaa1efe1a1e201805e5ee3129b5',
            'receipts/summary-body-codegen.json': 'sha256:09203ccf28633d137717e1c6e718bfbcd22eebaa83e8fa57de073b7d48d8faa4',
            'receipts/summary-execution.json': 'sha256:fa3dab41fc3aaf9adff0fff32e5c689a744a788590ae856bb37b8491700f6003',
            'receipts/summary-lowered.gooo': 'sha256:0fe068fd6cca2672142b9fd8a6f9702be76af82ad10113ab5387ec8521dbde33',
            'receipts/summary-replay.json': 'sha256:f3848a8ca737510fd8b3d88178290365134764d33cff84514fd37e377cb1be06',
            'receipts/summary-seed-inputs.json': 'sha256:fb8bab4cf3b0ff769a4e32c8696fb6eaa15ce663006228dee5a8642948200041',
            'summary.workspace.json': 'sha256:049e9c2283f3ee9c5bfe527dc38cee971c274ba72e2d6913c4b7ffb7f790c5d4',
            'testdata/classifier-expected.json': 'sha256:88feb8298c9eccd790810d0a7df646ded6028403a7485eb1743934745a09be2d'},
 'compiler_source_sha': '205896a413e5dea92366309c35ae85e542d055f6',
 'generation_sha256': 'sha256:1904100e57ff22b00f4db1597c4f12ebb3c6893063d6deac69e2eb516452a2ca'}
MODULES = [
    {'path': 'github.com/kimjooyoon/gooo-go-ports/examples/api-policy-revision',
     'version': 'v0.0.0-20261020182808-7180a4a0f36a',
     'sum': 'h1:2W/fIUaBsBzSCyWFMb0IKzItYZ4fnHVdyKEBVcXOeqE='},
    {'path': 'github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy',
     'version': 'v0.0.0-20261014100709-8cddcc4e65af',
     'sum': 'h1:fvrka7xThFrx1JkbTEp+lZ7hjcUsK4csF/XbpmF4VuM='},
]
DECISION_FIELDS = ['allowed', 'reason', 'authenticated', 'tenant_present', 'tenant_matched',
                   'action_present', 'action_matched', 'access_allowed', 'access_error',
                   'access_result', 'group_matched', 'principal_matched', 'group_missing',
                   'principal_defaulted', 'effective_principal']


def encoded(value):
    return (json.dumps(value, ensure_ascii=False, separators=(',', ':')) + '\n').encode()


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def facts(action='read'):
    return {'group_present': True, 'group_value': 'admin', 'principal_present': True,
            'principal_value': 'synthetic:user', 'authenticated': True,
            'caller_tenant': 'tenant-a', 'resource_tenant': 'tenant-a',
            'requested_action': action, 'allowed_action': action}


def literal_report(input_facts, *, action='read', authenticated=True):
    # Literal observations for these few specified cases, not a general Python
    # implementation of either policy or a substitute for Gooo evaluation.
    before = {'allowed': authenticated, 'reason': 'ALLOW' if authenticated else 'NOT_AUTHENTICATED',
              'authenticated': authenticated, 'tenant_present': True, 'tenant_matched': True,
              'action_present': True, 'action_matched': True, 'access_allowed': True,
              'access_error': False, 'access_result': 'ALLOW', 'group_matched': True,
              'principal_matched': False, 'group_missing': False, 'principal_defaulted': False,
              'effective_principal': 'synthetic:user'}
    after = copy.deepcopy(before)
    differences = {name: False for name in DECISION_FIELDS}
    summary = {'classification': 'UNCHANGED', 'changed_count': 0}
    if action == 'write' and authenticated:
        after.update(allowed=False, reason='ACTION_NOT_READ_ONLY')
        differences.update(allowed=True, reason=True)
        summary = {'classification': 'NEWLY_DENIED', 'changed_count': 2}
    return {'schema': 'gooo/api-policy-revision-comparison/v1', 'status': 'COMPARED',
            'scope': 'same_saved_facts_two_sealed_profiles',
            'before_profile': 'cel-api-original-v1', 'candidate_profile': 'cel-api-readonly-v1',
            'before': {'facts': input_facts, 'decision': before, 'bindings': copy.deepcopy(ORIGINAL_BINDINGS)},
            'after': {'facts': input_facts, 'decision': after, 'bindings': copy.deepcopy(REVISION_BINDINGS)},
            'differences': differences, 'summary': summary, 'classifier_bindings': copy.deepcopy(REVISION_BINDINGS)}


def exact_types(value, expected):
    """bool is never accepted as int by these test comparisons."""
    if type(value) is not type(expected):
        return False
    if isinstance(expected, dict):
        return set(value) == set(expected) and all(exact_types(value[k], v) for k, v in expected.items())
    if isinstance(expected, list):
        return len(value) == len(expected) and all(exact_types(a, b) for a, b in zip(value, expected))
    return value == expected


class CLITests(unittest.TestCase):
    binary = None

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='gooo-policy-cli-check-')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()
        self.sequence = 0

    def prepare(self, raw=None, *, name='facts.json'):
        self.sequence += 1
        base = self.root / ('case-%03d' % self.sequence); base.mkdir(mode=0o700)
        workspace, work = base / 'workspace', base / 'private-work'
        workspace.mkdir(mode=0o700); work.mkdir(mode=0o700)
        path = workspace / name; path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(encoded(facts()) if raw is None else raw)
        output = base / 'github-output'; output.write_bytes(b''); output.chmod(0o600)
        return {'base': base, 'workspace': workspace, 'work': work, 'input': path,
                'name': name, 'output': output, 'original_input': path.read_bytes()}

    def invoke(self, case, *, mode='compare', fail='false', name=None, output=None, work=None):
        environment = {'PATH': '', 'LANG': 'C.UTF-8', 'TZ': 'UTC',
                       'GOTOOLCHAIN': 'local', 'GOWORK': 'off', 'GOENV': 'off',
                       'GOPROXY': 'off', 'GOSUMDB': 'off', 'GOMAXPROCS': '2',
                       'RUNNER_TEMP': str(case['base']),
                       'GITHUB_WORKSPACE': str(case['workspace']),
                       'GOOO_ACTION_WORK': str(case['work'] if work is None else work),
                       'GITHUB_OUTPUT': str(case['output'] if output is None else output),
                       'GOOO_ACTION_MODE': mode, 'GOOO_ACTION_FAIL_NEWLY_DENIED': fail,
                       'GOOO_ACTION_INPUT': case['name'] if name is None else name}
        child = subprocess.Popen([str(self.binary)], cwd=case['workspace'], env=environment,
                                 stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                                 stderr=subprocess.PIPE, start_new_session=True)
        try:
            stdout, stderr = child.communicate(timeout=15)
        except subprocess.TimeoutExpired:
            try:os.killpg(child.pid, signal.SIGKILL)
            except ProcessLookupError:pass
            child.communicate(timeout=3)
            self.fail('built CLI exceeded the 15-second focused check bound')
        self.assertLessEqual(len(stdout), 4096)
        self.assertLessEqual(len(stderr), 4096)
        return child.returncode, stdout, stderr

    def assert_refused(self, case, *, label=None, no_result=True, **kwargs):
        code, stdout, stderr = self.invoke(case, **kwargs)
        self.assertEqual(code, 2)
        self.assertEqual(stdout, b'')
        self.assertRegex(stderr.decode(), r'^Gooo policy action: [A-Z_]+\n$')
        if label is not None:self.assertEqual(stderr, ('Gooo policy action: ' + label + '\n').encode())
        self.assertEqual(case['input'].read_bytes(), case['original_input'])
        self.assertEqual(case['output'].read_bytes(), case.get('output_prefix', b''))
        if no_result:self.assertFalse((case['work'] / 'result').exists())
        for secret in ['synthetic:user', 'tenant-a', 'DO_NOT_PRINT_FACTS']:
            self.assertNotIn(secret.encode(), stdout + stderr)
        return code, stdout, stderr

    def assert_saved(self, case, expected, *, mode='compare', fail='false', code=0):
        actual_code, stdout, stderr = self.invoke(case, mode=mode, fail=fail)
        self.assertEqual(actual_code, code)
        self.assertEqual(stderr, b'' if code == 0 else b'Gooo policy action: REQUESTED_CI_NEWLY_DENIED_FAILURE\n')
        output_raw = case['output'].read_bytes(); prefix = case.get('output_prefix', b'')
        self.assertTrue(output_raw.startswith(prefix))
        lines = output_raw[len(prefix):].decode().splitlines()
        self.assertEqual(len(lines), 5)
        pairs = dict(line.split('=', 1) for line in lines)
        self.assertEqual(set(pairs), {'classification', 'changed-count', 'report-path', 'receipt-path', 'report-sha256'})
        report_path = case['work'] / 'result/comparison.json'
        receipt_path = case['work'] / 'result/receipt.json'
        self.assertEqual(pairs['report-path'], str(report_path))
        self.assertEqual(pairs['receipt-path'], str(receipt_path))
        report_raw = report_path.read_bytes(); report = json.loads(report_raw)
        self.assertTrue(exact_types(report, expected), 'complete literal typed policy report mismatch')
        self.assertEqual(pairs['classification'], expected['summary']['classification'])
        self.assertEqual(pairs['changed-count'], str(expected['summary']['changed_count']))
        self.assertEqual(pairs['report-sha256'], sha(report_raw))
        self.assertTrue(exact_types(json.loads(stdout), {
            'classification': expected['summary']['classification'],
            'changed_count': expected['summary']['changed_count'], 'report_sha256': sha(report_raw)}))
        receipt = json.loads(receipt_path.read_bytes())
        self.assertEqual(receipt['schema'], 'gooo/api-policy-action-observation/v1')
        self.assertEqual(receipt['mode'], mode)
        self.assertEqual(receipt['input_sha256'], sha(case['original_input']))
        self.assertEqual(receipt['input_bytes'], len(case['original_input']))
        self.assertEqual(receipt['report_sha256'], sha(report_raw))
        self.assertEqual(receipt['report_bytes'], len(report_raw))
        self.assertTrue(exact_types(receipt['compiled_modules'], MODULES))
        self.assertEqual(receipt['compiled_go'], 'go1.27.3')
        self.assertEqual(receipt['compiled_os'], 'linux')
        self.assertRegex(receipt['compiled_arch'], r'^[a-z0-9]+$')
        self.assertEqual(receipt['saved_replay'], 'COMPLETE_TYPED_REPORT_RECOMPUTED')
        self.assertIn('unsigned consistency', receipt['scope'])
        self.assertEqual(stat.S_IMODE((case['work'] / 'result').stat().st_mode), 0o700)
        self.assertEqual(stat.S_IMODE(report_path.stat().st_mode), 0o600)
        self.assertEqual(stat.S_IMODE(receipt_path.stat().st_mode), 0o600)
        self.assertEqual(case['input'].read_bytes(), case['original_input'])
        for secret in ['synthetic:user', 'tenant-a']:
            self.assertNotIn(secret.encode(), stdout + stderr + case['output'].read_bytes())
        return report_raw

    def test_real_read_is_unchanged_all_fields(self):
        row = facts('read'); self.assert_saved(self.prepare(encoded(row)), literal_report(row))

    def test_real_write_is_newly_denied_all_fields(self):
        row = facts('write'); self.assert_saved(self.prepare(encoded(row)), literal_report(row, action='write'))

    def test_existing_output_lines_are_preserved_before_five_new_outputs(self):
        row = facts(); case = self.prepare(encoded(row))
        case['output_prefix'] = b'previous-step-value=KEEP\n'
        case['output'].write_bytes(case['output_prefix'])
        self.assert_saved(case, literal_report(row))

    def test_baseline_denial_stays_denied_without_ci_failure(self):
        row = facts('write'); row['authenticated'] = False
        self.assert_saved(self.prepare(encoded(row)), literal_report(row, action='write', authenticated=False), fail='true')

    def test_newly_denied_exit_three_still_saves_complete_outputs(self):
        row = facts('write'); self.assert_saved(self.prepare(encoded(row)), literal_report(row, action='write'), fail='true', code=3)

    def test_read_with_fail_flag_still_succeeds(self):
        row = facts(); self.assert_saved(self.prepare(encoded(row)), literal_report(row), fail='true')

    def test_saved_report_replay_preserves_complete_report(self):
        row = facts('write'); expected = literal_report(row, action='write')
        saved = self.assert_saved(self.prepare(encoded(row)), expected)
        self.assertEqual(self.assert_saved(self.prepare(saved, name='saved.json'), expected, mode='replay'), saved)

    def test_replay_refuses_typed_decision_flags_and_binding_tamper(self):
        good = literal_report(facts('write'), action='write')
        mutations = [lambda r:r['after']['decision'].update(reason='FORGED'),
                     lambda r:r['differences'].update(allowed=False),
                     lambda r:r['summary'].update(changed_count=1),
                     lambda r:r['classifier_bindings'].update(generation_sha256='sha256:' + '0' * 64)]
        for mutation in mutations:
            changed = copy.deepcopy(good); mutation(changed)
            with self.subTest(mutation=mutation):
                case = self.prepare(encoded(changed)); self.assert_refused(case, mode='replay', label='REPLAY_REFUSED')
                self.assertEqual(case['output'].read_bytes(), b'')

    def test_boolean_integer_confusion_refuses_facts_and_report(self):
        row = facts(); row['authenticated'] = 1
        self.assert_refused(self.prepare(encoded(row)), label='FACTS_INVALID')
        report = literal_report(facts()); report['before']['decision']['allowed'] = 1
        self.assert_refused(self.prepare(encoded(report)), mode='replay', label='REPORT_INVALID')

    def test_bad_json_duplicate_unknown_missing_and_text_bound(self):
        raw = encoded(facts()).rstrip()
        missing = facts(); del missing['group_present']
        unknown = facts(); unknown['extra'] = 'DO_NOT_PRINT_FACTS'
        text_bound = facts(); text_bound['principal_value'] = 'x' * 65537
        probes = [b'{bad JSON', b'\xff', b'null', raw + b' {}',
                  raw[:-1] + b',"\\u0061uthenticated":false}',
                  encoded(missing), encoded(unknown), encoded(text_bound)]
        for probe in probes:
            with self.subTest(size=len(probe)):
                self.assert_refused(self.prepare(probe), label='FACTS_INVALID')

    def test_replay_bad_json_and_duplicate_key_refuse(self):
        raw = encoded(literal_report(facts())).rstrip()
        for probe in [b'not json', raw[:-1] + b',"status":"COMPARED"}']:
            with self.subTest(size=len(probe)):
                self.assert_refused(self.prepare(probe), mode='replay', label='REPORT_INVALID')

    def test_facts_and_saved_report_byte_caps_refuse_before_decode(self):
        for mode, cap in [('compare', 4 << 20), ('replay', 16 << 20)]:
            case = self.prepare()
            with case['input'].open('r+b') as stream:stream.truncate(cap + 1)
            case['original_input'] = case['input'].read_bytes()
            with self.subTest(mode=mode):self.assert_refused(case, mode=mode, label='INPUT_FILE_REFUSED')

    def test_absolute_traversal_newline_and_missing_input_refuse(self):
        for name in ['/etc/passwd', '../facts.json', 'facts.json\nignored', 'missing.json']:
            with self.subTest(name=name):self.assert_refused(self.prepare(), name=name, label='INPUT_FILE_REFUSED')

    def test_direct_intermediate_and_cleaned_away_symlink_components_refuse(self):
        case = self.prepare(); (case['workspace'] / 'link.json').symlink_to('facts.json')
        self.assert_refused(case, name='link.json', label='INPUT_FILE_REFUSED')
        case = self.prepare(); (case['workspace'] / 'inside').mkdir()
        (case['workspace'] / 'link').symlink_to('inside', target_is_directory=True)
        self.assert_refused(case, name='link/../facts.json', label='INPUT_FILE_REFUSED')
        case = self.prepare(name='real/facts.json')
        (case['workspace'] / 'alias').symlink_to('real', target_is_directory=True)
        self.assert_refused(case, name='alias/facts.json', label='INPUT_FILE_REFUSED')

    def test_input_directory_and_fifo_refuse_without_waiting(self):
        case = self.prepare(); (case['workspace'] / 'directory').mkdir()
        self.assert_refused(case, name='directory', label='INPUT_FILE_REFUSED')
        case = self.prepare(); os.mkfifo(case['workspace'] / 'pipe')
        self.assert_refused(case, name='pipe', label='INPUT_FILE_REFUSED')

    def test_shell_metacharacter_filename_is_literal_data(self):
        # Direct CLI/env transport only; this is not an executed Bash/Action test.
        name = 'facts;$(touch OWNED_PWNED)`touch OWNED_PWNED` $HOME.json'
        row = facts(); case = self.prepare(encoded(row), name=name)
        self.assert_saved(case, literal_report(row))
        self.assertFalse((case['workspace'] / 'OWNED_PWNED').exists())

    def test_output_missing_directory_symlink_fifo_and_hardlink_refuse(self):
        for kind in ['missing', 'directory', 'symlink', 'fifo', 'hardlink']:
            case = self.prepare(); output = case['base'] / ('invalid-' + kind)
            target = case['base'] / 'sentinel'; target.write_bytes(b'KEEP-OUTPUT-SENTINEL\n')
            if kind == 'directory':output.mkdir()
            elif kind == 'symlink':output.symlink_to(target)
            elif kind == 'fifo':os.mkfifo(output)
            elif kind == 'hardlink':os.link(target, output)
            with self.subTest(kind=kind):
                self.assert_refused(case, output=output)
                self.assertEqual(target.read_bytes(), b'KEEP-OUTPUT-SENTINEL\n')
                self.assertEqual(case['output'].read_bytes(), b'')

    def test_input_output_path_and_hardlink_overlap_never_mutate_input(self):
        for hardlink in [False, True]:
            case = self.prepare(); output = case['input']
            if hardlink:
                output = case['base'] / 'input-hardlink'; os.link(case['input'], output)
            with self.subTest(hardlink=hardlink):
                self.assert_refused(case, output=output)
                self.assertEqual(output.read_bytes(), case['original_input'])

    def test_future_result_output_overlap_refuses_without_success_stdout(self):
        case = self.prepare()
        self.assert_refused(case, output=case['work'] / 'result/comparison.json')

    def test_existing_result_directory_is_not_overwritten(self):
        case = self.prepare(); result = case['work'] / 'result'; result.mkdir(mode=0o700)
        sentinel = result / 'comparison.json'; sentinel.write_bytes(b'CALLER-EXISTING-RESULT')
        self.assert_refused(case, label='RESULT_DIRECTORY_EXISTS_OR_REFUSED', no_result=False)
        self.assertEqual(sentinel.read_bytes(), b'CALLER-EXISTING-RESULT')
        self.assertEqual(case['output'].read_bytes(), b'')

    def test_invalid_configuration_and_private_work_permissions_refuse(self):
        self.assert_refused(self.prepare(), mode='not-a-mode', label='INPUT_CONFIGURATION_INVALID')
        self.assert_refused(self.prepare(), fail='yes', label='INPUT_CONFIGURATION_INVALID')
        case = self.prepare(); case['work'].chmod(0o755)
        self.assert_refused(case, label='PRIVATE_WORK_REFUSED')


def main():
    if len(sys.argv) != 2:
        raise SystemExit('usage: python3 -B check_cli.py /absolute/already-built-linux-binary')
    binary = Path(sys.argv[1])
    if not binary.is_absolute() or binary.resolve() != binary or not binary.is_file() or not os.access(binary, os.X_OK):
        raise SystemExit('Supply a canonical absolute executable built by root; this driver never builds it.')
    CLITests.binary = binary
    result = unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(CLITests))
    raise SystemExit(0 if result.wasSuccessful() else 1)


if __name__ == '__main__':main()
