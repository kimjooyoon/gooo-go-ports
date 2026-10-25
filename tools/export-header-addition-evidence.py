#!/usr/bin/env python3
# Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see ../LICENSE.
"""Project one checked native-shard activity into frozen B1 evidence records.

No compiler, oracle, model, network, response planner or publication is run.
The existing reader still checks its complete population before this view is
made. The selected scope is exactly 50 HeaderAddition cases and seven batches.
"""
import argparse
import copy
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import sys
import tempfile

HERE = Path(__file__).resolve().parent
ENTRY = 'app/diagnostic-header-addition:AssessAddition'
MAX_INDEX = 16 << 20
MAX_OUTPUT = 2 << 20
HOST = '8a89358362d376b265c6ee8f35ad76a342a8deb21d5fec9e2c736f083a60200c'
CONTRACT = 'c2a75cfb0a2b1742daf00c000fae7916cc393521f621f58ff74177d134376df1'
EVIDENCE_VALIDATOR = 'ef059151d72591820105acea251cef963d4af170e887721b2160b72ec34042fc'
PROVIDERS = {
    'cc05499ac8c03469dc9d44d18dbd2e5b9d5f80ccfff81061e5705f7df437803d',
    'ce9d617a18f7a0ff18ef244b5c4eb6c684819bb144a5350f4ff5bc69db50b14e',
    'a9a7abfc846935d75472dcbadb87b66da6380f52876d1a35e1fa8205871fa174',
}


class Refused(Exception):
    """Public fixed label; details stay in the caller's local diagnostic."""


def need(condition, code):
    if not condition:
        raise Refused(code)


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def same_json(a, b):
    # Match decoded run context with exact types, including bool versus integer.
    if type(a) is not type(b):
        return False
    if type(a) is dict:
        return a.keys() == b.keys() and all(same_json(a[k], b[k]) for k in a)
    if type(a) is list:
        return len(a) == len(b) and all(same_json(x, y) for x, y in zip(a, b))
    return a == b


def encoded(value):
    raw = (json.dumps(value, ensure_ascii=False, separators=(',', ':'),
                      allow_nan=False) + '\n').encode('utf-8')
    need(len(raw) <= MAX_OUTPUT, 'EXPORT_TOO_LARGE')
    return raw


def bounded(path, limit):
    need(not path.is_symlink() and path.is_file(), 'INPUT_NOT_REGULAR')
    with path.open('rb') as source:
        raw = source.read(limit + 1)
    need(0 < len(raw) <= limit, 'INPUT_SIZE_REFUSED')
    return raw


def load_code(path, name, allowed):
    raw = bounded(path, 128 << 10)
    need(digest(raw) in allowed, 'READER_PROFILE_UNSUPPORTED')
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    exec(compile(raw, str(path), 'exec'), module.__dict__)
    return module


def read_checked(source_root, index_path, evidence_roots):
    """Keep original reader obligations and explicit host-owned path scope."""
    source_root = source_root.resolve(strict=True)
    permitted = {str(path.resolve(strict=True)) for path in evidence_roots}
    need(permitted, 'EVIDENCE_ROOT_REQUIRED')
    code = {
        'check-header-addition-host.py': {HOST},
        'native_verification_contract.py': {CONTRACT},
        'native-shard-reference.py': PROVIDERS,
    }
    frozen = {}
    for name, allowed in code.items():
        raw = bounded(source_root / 'tools' / name, 128 << 10)
        need(digest(raw) in allowed, 'READER_PROFILE_UNSUPPORTED')
        frozen[name] = raw
    raw_index = bounded(index_path, MAX_INDEX)
    # A fresh cache prefix avoids loading a producer checkout's stale pyc.
    # The private byte-identical index copy prevents metadata/path-scope races.
    with tempfile.TemporaryDirectory(prefix='gooo-evidence-read-') as directory:
        work = Path(directory).resolve(strict=True)
        previous = sys.pycache_prefix, sys.dont_write_bytecode
        sys.pycache_prefix, sys.dont_write_bytecode = str(work / 'pycache'), True
        try:
            provider = load_code(HERE / 'native-shard-reference.py',
                                 'gooo_contribution_preflight', PROVIDERS)
            index = provider.load(raw_index)
            need(type(index) is dict and index.get('schema') ==
                 'gooo/native-shard-reference-index/v2', 'INDEX_PROFILE_UNSUPPORTED')
            shards = index.get('shards')
            need(type(shards) is list and 1 <= len(shards) <= 8,
                 'INDEX_PROFILE_UNSUPPORTED')
            for shard in shards:
                need(type(shard) is dict and type(shard.get('evidence_root')) is str
                     and shard['evidence_root'] in permitted,
                     'EVIDENCE_ROOT_NOT_PERMITTED')
            private_index = work / 'reference-index.json'
            private_index.write_bytes(raw_index)
            host = load_code(HERE / 'check-header-addition-host.py',
                             'gooo_contribution_host', {HOST})
            result = host.read_shard_verification(source_root, private_index)
            for name, raw in frozen.items():
                need(bounded(source_root / 'tools' / name, 128 << 10) == raw,
                     'READER_CHANGED')
        finally:
            sys.pycache_prefix, sys.dont_write_bytecode = previous
    return result, index, digest(raw_index)


def project(data, index, index_sha, failure=None, failure_sha=None, *, origin='frozen_observation'):
    """Private view over checked data; unit fixtures must say synthetic."""
    need(data.get('entry') == ENTRY and data.get('evidence_mode') ==
         'experimental-shard-native-reference', 'SELECTED_ACTIVITY_UNSUPPORTED')
    need(type(data.get('fresh_addition_cases')) is int and
         data['fresh_addition_cases'] == 50 and data.get('fresh_addition_batches') == 7
         and len(data['rows']) == len(data['case_names']) == 50,
         'SELECTED_SCOPE_UNSUPPORTED')
    need(origin in ('frozen_observation', 'synthetic'), 'ORIGIN_UNSUPPORTED')
    evidence = data['evidence']
    need(len(evidence) == 14, 'SELECTED_SCOPE_UNSUPPORTED')
    rows = []
    for number, item in enumerate(evidence):
        need(item['batch'] == number // 2 and item['operation'] ==
             ('execute' if number % 2 == 0 else 'replay'), 'RECEIPT_ORDER_REFUSED')
        need(item['receipt_sha256_scope'] == 'original-json' and
             re.fullmatch('[0-9a-f]{64}', item['receipt_sha256']) is not None,
             'RECEIPT_IDENTITY_REFUSED')
        storage = item['receipt_storage']
        rows.append({key: copy.deepcopy(item[key]) for key in
                     ('batch', 'operation', 'receipt_sha256', 'cases_digest',
                      'original_receipt_sha256', 'receipt_sha256_scope')})
        rows[-1]['storage'] = {key: storage[key] for key in
                               ('encoding', 'bytes', 'sha256')}
    run = index['plan']['run']
    if failure is not None:
        need(type(failure) is dict and set(failure) ==
             {'schema', 'type', 'phase', 'error', 'context', 'native_acceptance'}
             and failure['schema'] == 'gooo/manual-native-shard-first-failure/v1'
             and failure['native_acceptance'] is False and same_json(failure['context'], run),
             'FAILURE_CONTEXT_REFUSED')
        need(all(type(failure[key]) is str and 0 < len(failure[key]) <= 4096
                 for key in ('type', 'phase', 'error')) and type(failure_sha) is str
             and re.fullmatch('[0-9a-f]{64}', failure_sha), 'FAILURE_CONTEXT_REFUSED')
    appendix = {
        'schema': 'gooo/header-addition-contribution-appendix/v1',
        'entry': ENTRY, 'origin': origin, 'reference_index_sha256': index_sha,
        'reported_run': copy.deepcopy(run),
        'reader_state': {'native_acceptance': False,
                         'producer_authenticated': False,
                         'raw_receipts_inspected': True},
        'selected_cases': 50, 'selected_batches': 7,
        'complete_reader_suite_count': data['fresh_native_suites'],
        'rows': copy.deepcopy(data['rows']), 'case_names': list(data['case_names']),
        'source_set': copy.deepcopy(data['source_set']),
        'typed_plan': copy.deepcopy(data['typed_plan']),
        'records': copy.deepcopy(data['records']),
        'generated_source_utf8_sha256': digest(data['source'].encode('utf-8')),
        'go_version': data['go_version'], 'go_binary': copy.deepcopy(data['go_binary']),
        'tool_selections': list(data['tool_selections']),
        'receipts': rows, 'producer_first_failure': copy.deepcopy(failure),
        'producer_first_failure_raw_sha256': failure_sha,
    }
    appendix_raw = encoded(appendix)
    appendix_sha = digest(appendix_raw)
    common = {
        'schema': 'gooo/contribution-evidence/v1', 'origin': origin,
        'identities': {name: None for name in
                       ('source_commit', 'source_blob', 'base_commit',
                        'candidate_commit', 'reference_commit')},
        'references': [{'id': 'selected-typed-appendix',
                        'url': 'urn:sha256:' + appendix_sha, 'sha256': appendix_sha}],
        'uncertainty': [
            'Unsigned reference-only reader: native_acceptance=false and producer_authenticated=false.',
            'Reported Git identities stay in the appendix; the reader does not authenticate a Git commit or GitHub job.',
            'Selected saved observations are not whole-package compatibility, production impact, or current live execution.',
            'Overall workflow/host-consumer success is not established by this export.',
        ],
        'failures': ([] if failure is None else [
            'A same-run producer first-failure record is retained in the bound appendix; do not describe the whole run as successful.']),
        'exclusions': ['No base/candidate regression comparison.',
                       'No producer authentication, native acceptance, execution or publication permission.',
                       'No fresh compiler, oracle, model, network or tool execution by the exporter.'],
    }
    common['identities'].update(reference_name='Recorded Go case expectations',
                                 reference_version=data['go_version'])
    records = []
    for phase, scope in (
            ('native_execution', '50 recorded native results matched the supplied typed expectations in seven batches.'),
            ('oracle_comparison', 'Equality with 50 saved Go expected rows; no fresh Go oracle is run by this exporter.'),
            ('saved_replay', 'Seven recorded replays preserve the original receipt parents, typed inputs and results.')):
        record = copy.deepcopy(common)
        record['id'] = 'urn:gooo:ports:header-addition:' + appendix_sha + ':' + phase
        record['execution'] = {
            'phase': phase, 'outcome': 'pass', 'tested_scope': [ENTRY, scope,
                'Recorded bindings: compiler source ' + run['compiler_revision']
                + '; ' + data['go_version'] + '; five-package source closure in appendix.',
                'Domain: the repository fixed 50-case HeaderAddition corpus; not arbitrary live HTTP work.'],
            'comparison': 'not_compared', 'base_outcome': 'not_compared',
            'candidate_outcome': 'not_compared'}
        records.append(record)
    return records, appendix_raw


def preview(records):
    lines = ['HeaderAddition saved evidence (reference-only)', ENTRY,
             records[0]['execution']['tested_scope'][2],
             records[0]['execution']['tested_scope'][3], '']
    for record in records:
        e = record['execution']
        lines.append(e['phase'] + ': ' + e['outcome'] + ' — ' + e['tested_scope'][1])
    lines += ['', 'Uncertainty:', *records[0]['uncertainty'],
              '', 'Failures:', *(records[0]['failures'] or [
                  'No producer failure record supplied; overall run status remains unassessed.']),
              '', 'Exclusions:', *records[0]['exclusions']]
    return ('\n'.join(lines) + '\n').encode('utf-8')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source-root', type=Path, required=True)
    parser.add_argument('--reference-index', type=Path, required=True)
    parser.add_argument('--evidence-root', type=Path, action='append', required=True)
    parser.add_argument('--run-failure', type=Path)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    owns_output = False
    try:
        need(not args.out.exists(), 'OUTPUT_NOT_FRESH')
        need(not args.out.resolve().is_relative_to(args.source_root.resolve()),
             'OUTPUT_INSIDE_SOURCE_ROOT')
        args.out.mkdir(mode=0o700)
        owns_output = True
        data, index, index_sha = read_checked(args.source_root, args.reference_index,
                                               args.evidence_root)
        failure = None
        failure_sha = None
        if args.run_failure:
            raw = bounded(args.run_failure, 64 << 10)
            # Use the same strict parser as the existing provider.
            provider = load_code(HERE / 'native-shard-reference.py',
                                 'gooo_contribution_failure_json', PROVIDERS)
            failure = provider.load(raw)
            failure_sha = digest(raw)
        records, appendix = project(data, index, index_sha, failure, failure_sha)
        contract = load_code(HERE / 'contribution_evidence_contract.py',
                             'gooo_frozen_contribution_contract', {EVIDENCE_VALIDATOR})
        schema = contract.schema()
        for record in records:
            contract.validate(record, schema)
        files = {'appendix.json': appendix, 'technical-preview.txt': preview(records)}
        for record in records:
            files[record['execution']['phase'] + '.evidence.json'] = encoded(record)
        need(sum(len(raw) for raw in files.values()) <= 4 << 20, 'EXPORT_TOO_LARGE')
        manifest = {'schema': 'gooo/header-addition-contribution-export/v1',
                    'state': 'EXPORTED_REFERENCE_OBSERVATIONS',
                    'contract_schema_sha256': contract.SCHEMA_SHA256,
                    'files': {name: {'bytes': len(raw), 'sha256': digest(raw)}
                              for name, raw in files.items()},
                    'native_acceptance': False, 'producer_authenticated': False}
        with tempfile.TemporaryDirectory(prefix='.staged-', dir=args.out) as directory:
            stage = Path(directory) / 'result'
            stage.mkdir(mode=0o700)
            for name, raw in {**files, 'manifest.json': encoded(manifest)}.items():
                with (stage / name).open('xb') as target:
                    os.chmod(target.name, 0o600)
                    target.write(raw)
                    target.flush()
                    os.fsync(target.fileno())
            os.rename(stage, args.out / 'result')
        # The published manifest is the output, so a separate stdout
        # notification cannot turn a complete export into a claimed refusal.
        return 0
    except Exception as error:
        # Raw paths or metadata text are not public prose or instructions.
        code = str(error) if isinstance(error, Refused) else 'SOURCE_EVIDENCE_REFUSED'
        diagnostic_write_failed = False
        if owns_output:
            detail = str(error)
            diagnostic = {'state': 'REFUSED', 'code': code,
                          'error_type': type(error).__name__, 'detail': detail[:2000],
                          'detail_truncated': len(detail) > 2000,
                          'native_acceptance': False, 'producer_authenticated': False}
            try:
                with (args.out / 'refusal.json').open('xb') as target:
                    os.chmod(target.name, 0o600)
                    target.write(encoded(diagnostic))
            except OSError:
                diagnostic_write_failed = True
        print(json.dumps({'state': 'REFUSED', 'code': code,
                          'native_acceptance': False,
                          'diagnostic_write_failed': diagnostic_write_failed}), file=sys.stderr)
        return 1


if __name__ == '__main__':
    raise SystemExit(main())
