"""Bind an owned go-list observation to the source-owned Gooo surface evaluator.

This adapter has no declared package list or membership/count algorithm. LF is
its only record separator. Empty template lines are omitted; every nonempty
line, including whitespace, CR and Unicode, is retained verbatim and in order.
Limits below are adapter resource bounds, not claims about all Gooo programs.
"""
import hashlib
import stat
from pathlib import Path

MAX_BYTES = 512 << 10
MAX_PATHS = 256
MAX_TEXT_BYTES = 1024
LIST_TEMPLATE = '{{if not .Standard}}{{.ImportPath}}{{end}}'
SOURCE_NAMES = (
    'go.mod', 'go.sum', 'upstream.json', 'runtime-package-surface.workspace.json',
    'tools/runtime_package_surface.py', 'cmd/check-runtime-package-surface/main.go',
    'ports/runtime-package-surface/contract.gooo',
    'ports/runtime-package-surface/inspect.gooo', 'ports/runtime-package-surface/LICENSE',
    'internal/runtimepackagesurface/projection_generated.go',
    'internal/runtimepackagesurface/types_generated.go',
    'internal/runtimepackagesurface/bridge_generated.go',
    'internal/runtimepackagesurface/generation.json',
    'examples/runtime-package-surface/generate.py',
    'examples/runtime-package-surface/evidence.py',
    'examples/runtime-package-surface/verify.py',
    'examples/runtime-package-surface/abi.json',
    'examples/runtime-package-surface/cases.json',
    'examples/runtime-package-surface/upstream.json',
    'examples/tool-definition-binding/evidence.py',
    'internal/runtimepackagesurface/evaluate_test.go',
    'examples/recorded-catalog-comparison/generate.py',
    'examples/service-grant-many/verify.py',
)


def source_roster(root):
    """Explicit files used by the root checker and its source generation chain."""
    return [Path(root) / name for name in SOURCE_NAMES]


def parse_observation(raw):
    if type(raw) is not bytes or len(raw) > MAX_BYTES:
        raise ValueError('bounded raw go-list bytes required')
    text = raw.decode('utf-8', errors='strict')
    if text and not text.endswith('\n'):
        raise ValueError('nonempty go-list output must end with LF')
    paths = [line for line in text.split('\n') if line != '']
    if len(paths) > MAX_PATHS or any(len(path.encode('utf-8')) > MAX_TEXT_BYTES for path in paths):
        raise ValueError('go-list observation exceeds adapter bounds')
    return {'origin_ref': 'sha256:' + hashlib.sha256(raw).hexdigest(), 'import_paths': paths}


def _snapshot(root, host):
    result = {}
    for path in source_roster(root):
        assert not path.is_symlink() and stat.S_ISREG(path.stat().st_mode), str(path)
        result[str(path.relative_to(root))] = dict(host.ref(path), mode=stat.S_IMODE(path.stat().st_mode))
    return result


def _process(out, label, argv, stdin, root, raw, host):
    directory = out / label
    assert (directory / 'stdout').read_bytes() == raw
    record = host.strict((directory / 'process.json').read_bytes())
    assert record['argv'] == [str(value) for value in argv] and record['cwd'] == str(root)
    assert record['exit_code'] == 0 and record['first_failure'] is None
    assert all(record[key] is True for key in ('waited', 'streams_EOF', 'PG_ESRCH'))
    assert type(record['PID']) is int and record['PID'] > 0 and record['PID'] == record['PGID']
    assert record['stdout'] == host.ref(directory / 'stdout')
    assert record['stderr'] == host.ref(directory / 'stderr')
    assert record['stdin'] == (host.ref(stdin) if stdin is not None else None)
    return host.ref(directory / 'process.json')


def _assessment(raw, observation, host):
    assert type(raw) is bytes and len(raw) <= MAX_BYTES
    value = host.strict(raw)
    assert type(value) is dict and set(value) == {
        'status', 'observation_origin', 'declared_paths', 'observed_paths',
        'missing_count', 'undeclared_count', 'duplicate_observation',
    }
    assert type(value['status']) is str and type(value['observation_origin']) is str
    for key in ('declared_paths', 'observed_paths'):
        assert type(value[key]) is list and all(type(path) is str for path in value[key])
    for key in ('missing_count', 'undeclared_count'):
        assert type(value[key]) is int and value[key] >= 0
    assert type(value['duplicate_observation']) is bool
    # This is observation binding, not a second implementation of membership.
    assert value['observation_origin'] == observation['origin_ref']
    assert value['observed_paths'] == observation['import_paths']
    return value


def verify(root, module, go, go_identity, environment, run, host, out, label):
    """Use the caller's owned-process callback; never launch an untracked child."""
    root, module, go, out = map(Path, (root, module, go, out))
    assert all(path.is_absolute() for path in (root, module, go, out))
    assert module.is_relative_to(root) and not out.is_relative_to(root)
    assert not go.is_symlink() and host.ref(go) == {key: go_identity[key] for key in ('path', 'bytes', 'sha256')}
    assert type(environment) is dict and environment and all(type(k) is str and type(v) is str for k, v in environment.items())
    env = dict(environment)
    sources = _snapshot(root, host)
    list_argv = [go, '-C', module, 'list', '-deps', '-f', LIST_TEMPLATE, './cmd/compare']
    check_argv = [go, '-C', root, 'run', './cmd/check-runtime-package-surface']
    binding_path = out / 'runtime-package-surface-binding.json'
    input_path = out / 'runtime-package-surface-observation.json'
    assert not binding_path.exists() and not input_path.exists()
    binding = {'schema': 'gooo/runtime-package-surface-observation-binding/v1',
               'status': 'OBSERVATION_STARTED_NOT_ACCEPTED', 'go_identity': go_identity,
               'environment': env, 'source_before': sources,
               'go_list_argv': [str(v) for v in list_argv],
               'checker_argv': [str(v) for v in check_argv],
               'record_separator': 'LF; empty template lines omitted; CR is data',
               'observations_are_authenticated': False}
    host.save(binding_path, binding)
    raw = run(label, list_argv, 90, environment=env)
    binding['go_list_process'] = _process(out, label, list_argv, None, root, raw, host)
    binding['raw_stdout'] = host.ref(out / label / 'stdout')
    host.save(binding_path, binding)
    observation = parse_observation(raw)
    host.save(input_path, observation)
    checker_label = label + '-source-assessment'
    result_raw = run(checker_label, check_argv, 90, stdin=input_path, environment=env)
    binding['checker_process'] = _process(out, checker_label, check_argv, input_path, root, result_raw, host)
    binding['observation'] = host.ref(input_path)
    binding['assessment'] = host.ref(out / checker_label / 'stdout')
    host.save(binding_path, binding)
    result = _assessment(result_raw, observation, host)
    binding['source_after'] = _snapshot(root, host)
    assert binding['source_after'] == sources
    assert host.ref(go) == {key: go_identity[key] for key in ('path', 'bytes', 'sha256')}
    binding['status'] = result['status']
    host.save(binding_path, binding)
    assert result['status'] == 'OBSERVED_EXACT', 'Gooo runtime package surface: ' + result['status']
    return {'assessment': result, 'binding': host.ref(binding_path)}
