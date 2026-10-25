"""Adapter tests use owned-run records without executing Go or a native program."""
import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

SPEC = importlib.util.spec_from_file_location('runtime_surface', Path(__file__).with_name('runtime_package_surface.py'))
surface = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(surface)


class Host:
    @staticmethod
    def ref(path):
        raw = path.read_bytes()
        return {'path': str(path), 'bytes': len(raw), 'sha256': hashlib.sha256(raw).hexdigest()}

    @staticmethod
    def save(path, value):
        path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')

    @staticmethod
    def strict(raw):
        def pairs(items):
            result = {}
            for key, value in items:
                if key in result:
                    raise ValueError('duplicate key')
                result[key] = value
            return result
        return json.loads(raw, object_pairs_hook=pairs)


class ObservationTests(unittest.TestCase):
    def test_exact_bytes_order_duplicates_and_spelling(self):
        raw = 'Example/X \n\n한글/경로\r\nExample/X \n'.encode()
        observed = surface.parse_observation(raw)
        self.assertEqual(observed['import_paths'], ['Example/X ', '한글/경로\r', 'Example/X '])
        self.assertEqual(observed['origin_ref'], 'sha256:' + hashlib.sha256(raw).hexdigest())
        self.assertNotEqual(observed['origin_ref'], surface.parse_observation(raw.replace(b'\n\n', b'\n'))['origin_ref'])

    def test_empty_template_output_is_empty_observation(self):
        self.assertEqual(surface.parse_observation(b'')['import_paths'], [])
        self.assertEqual(surface.parse_observation(b'\n\n')['import_paths'], [])
        self.assertEqual(surface.parse_observation(b' \n')['import_paths'], [' '])

    def test_rejects_invalid_encoding_unterminated_and_resource_overflow(self):
        for raw in (b'\xff\n', b'path', b'x' * (surface.MAX_BYTES + 1),
                    b'x\n' * (surface.MAX_PATHS + 1), b'x' * (surface.MAX_TEXT_BYTES + 1) + b'\n'):
            with self.subTest(raw=raw[:15]), self.assertRaises((ValueError, UnicodeError)):
                surface.parse_observation(raw)


class BoundRunTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name)
        self.root = self.base / 'source'
        self.out = self.base / 'evidence'
        self.out.mkdir()
        self.module = self.root / 'examples/recorded-catalog-comparison'
        self.module.mkdir(parents=True)
        for path in surface.source_roster(self.root):
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(str(path.relative_to(self.root)))
        self.go = self.base / 'go'
        self.go.write_bytes(b'synthetic Go identity; never executed')
        self.identity = dict(Host.ref(self.go), version='go version fixture')
        self.env = {'GOTOOLCHAIN': 'local', 'GOWORK': 'off', 'GOPROXY': 'off'}
        self.raw = b'example.org/first\nexample.org/second\n'
        self.calls = []
        self.result_change = lambda value: value
        self.process_change = lambda label, value: value
        self.source_change = False

    def run_owned(self, label, argv, seconds, stdin=None, environment=None):
        self.calls.append((label, [str(x) for x in argv], copy.deepcopy(environment)))
        directory = self.out / label
        directory.mkdir()
        if stdin is None:
            raw = self.raw
        else:
            request = Host.strict(stdin.read_bytes())
            value = {'status': 'OBSERVED_EXACT', 'observation_origin': request['origin_ref'],
                     'declared_paths': ['example.org/first', 'example.org/second'],
                     'observed_paths': request['import_paths'], 'missing_count': 0,
                     'undeclared_count': 0, 'duplicate_observation': False}
            raw = json.dumps(self.result_change(value)).encode()
            if self.source_change:
                (self.root / 'cmd/check-runtime-package-surface/main.go').write_text('changed during assessment')
        (directory / 'stdout').write_bytes(raw)
        (directory / 'stderr').write_bytes(b'')
        process = {'argv': [str(x) for x in argv], 'cwd': str(self.root), 'exit_code': 0,
                   'first_failure': None, 'waited': True, 'streams_EOF': True, 'PG_ESRCH': True,
                   'PID': 123, 'PGID': 123, 'stdout': Host.ref(directory / 'stdout'),
                   'stderr': Host.ref(directory / 'stderr'), 'stdin': Host.ref(stdin) if stdin else None}
        Host.save(directory / 'process.json', self.process_change(label, process))
        return raw

    def verify(self):
        return surface.verify(self.root, self.module, self.go, self.identity, self.env,
                              self.run_owned, Host, self.out, 'runtime-dependencies')

    def test_binds_raw_command_go_environment_sources_and_assessment(self):
        result = self.verify()
        binding = Host.strict((self.out / 'runtime-package-surface-binding.json').read_bytes())
        self.assertEqual(binding['environment'], self.env)
        self.assertEqual(binding['go_identity'], self.identity)
        self.assertEqual(binding['source_before'], binding['source_after'])
        self.assertEqual(set(binding['source_before']), set(surface.SOURCE_NAMES))
        self.assertEqual(binding['raw_stdout'], Host.ref(self.out / 'runtime-dependencies/stdout'))
        self.assertEqual(result['assessment']['observation_origin'], 'sha256:' + hashlib.sha256(self.raw).hexdigest())
        self.assertEqual(len(self.calls), 2)
        self.assertEqual(self.calls[1][1], [str(self.go), '-C', str(self.root), 'run', './cmd/check-runtime-package-surface'])
        self.assertTrue(all(call[2] == self.env for call in self.calls))

    def test_changed_go_list_process_cannot_be_accepted(self):
        self.process_change = lambda label, value: dict(value, argv=['different-command'])
        with self.assertRaises(AssertionError):
            self.verify()
        self.assertEqual(len(self.calls), 1)

    def test_observed_order_cannot_be_rewritten(self):
        self.result_change = lambda value: dict(value, observed_paths=list(reversed(value['observed_paths'])))
        with self.assertRaises(AssertionError):
            self.verify()

    def test_origin_cannot_be_rebound(self):
        self.result_change = lambda value: dict(value, observation_origin='sha256:' + '0' * 64)
        with self.assertRaises(AssertionError):
            self.verify()

    def test_source_mutation_cannot_be_accepted(self):
        self.source_change = True
        with self.assertRaises(AssertionError):
            self.verify()

    def test_Gooo_refusal_is_retained_and_not_recomputed(self):
        self.result_change = lambda value: dict(value, status='MISSING_PACKAGES', missing_count=1)
        with self.assertRaisesRegex(AssertionError, 'MISSING_PACKAGES'):
            self.verify()
        binding = Host.strict((self.out / 'runtime-package-surface-binding.json').read_bytes())
        self.assertEqual(binding['status'], 'MISSING_PACKAGES')
        self.assertEqual(binding['assessment'], Host.ref(self.out / 'runtime-dependencies-source-assessment/stdout'))

    def test_no_declaration_is_sent_to_checker(self):
        self.verify()
        request = Host.strict((self.out / 'runtime-package-surface-observation.json').read_bytes())
        self.assertEqual(set(request), {'origin_ref', 'import_paths'})


if __name__ == '__main__':
    unittest.main()
