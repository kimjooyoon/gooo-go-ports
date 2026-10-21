"""Bounded fresh IPv4 native/replay and generated-Report sidecar.

The public path process lifecycle is copied into owned work and source-bound;
classification expectations come only from the freshly executed Go oracle.
"""
from pathlib import Path
from decimal import Decimal
import argparse
import hashlib
import json
import os
import shutil
import signal
import stat
import sys
import tempfile
import time
import types

REV = '205896a413e5dea92366309c35ae85e542d055f6'
GO_VERSION = 'go1.27.3'
LIFECYCLE_SHA ='363a85df7012cb3af26da99e1593cff350ba44d24a7e9bb598207cb9d5b1bcb1'
PROFILE_SHA = '46862aabf1934c1d2a39ad830c4a6cd05d2369c5014fea231d87b7861f4d3862'
CODEC_SHA = '82a01a92ac7e36af2ace8ac2a8270c5b31b14e57b122308e2dca344c7403a49c'
RECORD_PROFILE_SHA = 'sha256:b0bb85f0c35ebfdef38bcaccb7e3e549e2eec5c42e4abebf8f5babce5ec1931e'
NETIP_SHA = '32295f4761825d8c6fdade4dad093b08683bdd1945010bee8a90a298e18cf208'
CORPUS_SHA = '7fb8c9d44e8fa77807e9b5580ab8aa975a8775486d5dfb6daaef9be2472b4b08'
WORK_CAP, PRE_CAP, PROOF_CAP, FREE_MIN = 384 << 20, 320 << 20, 16 << 20, 5 << 30
# Keep the per-file16MiB limit. Current205's unchanged IPv4/prefix/pair batches
# and36 retained tamper copies total36,045,315 bytes before final ledgers.
TOTAL_PROOF_CAP = 64 << 20
STDOUT_CAP, STDERR_CAP = 8 << 20, 1 << 20
COUNTS = {'top_level_stages': 54, 'manual_execute_outputs': 352,
          'manual_saved_replay_outputs': 352, 'typed_scalar_fields_each_phase': 992,
          'original_report_echoes_each_phase': 256, 'native_internal_run_records': 60,
          'independent_oracle_cases': 202, 'generated_Report_calls': 202,
          'generated_typed_fields': 2222, 'generated_original_echoes': 808,
          'extra_CLI_outputs': 0, 'extra_saved_replay_outputs': 0,
          'strict_wire_refusals': 42, 'receipt_tamper_refusals': 18, 'automatic_retries': 0}
METHODS = ('IsPrivate', 'IsLoopback', 'IsUnspecified', 'IsMulticast', 'IsLinkLocalUnicast', 'IsGlobalUnicast')
REPORT = 'app/ipv4-address-facts:Report'
MANIFESTS = dict(zip(('go/net/netip:' + n for n in METHODS),
                    ('ipv4-is-private.workspace.json', 'ipv4-is-loopback.workspace.json',
                     'ipv4-is-unspecified.workspace.json', 'ipv4-is-multicast.workspace.json',
                     'ipv4-is-link-local-unicast.workspace.json', 'ipv4-is-global-unicast.workspace.json')))
MANIFESTS[REPORT] = 'ipv4-address-facts.workspace.json'
TEMPLATES = ('bridge-main.go.txt', 'support.go.txt', 'harness-main.go.txt', 'harness-test.go.txt')
BASE_SOURCES = ('LICENSE', 'third_party/GO-LICENSE', 'third_party/go/netip.go.txt', 'go.mod',
                'upstream.json', 'NOTICE', 'README.md', 'AGENTS.md', '.github/workflows/ci.yml',
                'ports/net/netip/ipv4.gooo', 'examples/ipv4-address-facts/main.gooo',
                'examples/ipv4-address-facts/corpus.json', 'examples/ipv4-address-facts/corpus-202.json',
                'examples/ipv4-address-facts/inputs.json', 'examples/ipv4-address-facts/README.md',
                'cmd/ipv4-oracle/main.go', 'cmd/ipv4-oracle/main_test.go',
                'cmd/check-path-facts/result_digest.go', 'cmd/check-path-facts/result-profile-83928fa.json',
                'cmd/check-path-facts/result-profile-go1273.json',
                'tools/check-path-facts.py', 'tools/check-ipv4.py', 'tools/check-ipv4_test.py',
                'tools/ipv4_receipts.py', 'tools/ipv4_receipts_test.py',
                'tools/check-ci-scope-workflow_test.py')
H = R = Q = None
PAIR_TEMPLATES = ("pair-harness-main.go.txt", "pair-harness-test.go.txt")
PAIR_SOURCES = ("tools/prefix_rule_pair_receipts.py", "tools/prefix_rule_pair_receipts_test.py",
 "ports/net/netip/prefix.gooo", "examples/ipv4-prefix-probe/main.gooo", "examples/ipv4-prefix-probe/corpus-18.json",
 "examples/ipv4-prefix-probe/contains-inputs.json", "examples/ipv4-prefix-probe/overlaps-inputs.json", "examples/ipv4-prefix-probe/README.md",
 "examples/ipv4-rule-pair/main.gooo", "examples/ipv4-rule-pair/contract.json", "examples/ipv4-rule-pair/corpus-18.json",
 "examples/ipv4-rule-pair/corpus-112.json", "examples/ipv4-rule-pair/inputs.json", "examples/ipv4-rule-pair/README.md",
 "tools/generate-rule-pair-corpus.py", "prefix-contains.workspace.json", "prefix-overlaps.workspace.json", "ipv4-rule-pair.workspace.json")
PAIR_COUNTS = {"manual_execute_outputs":54,"manual_saved_replay_outputs":54,"typed_scalar_fields_each_phase":234,
 "original_input_values_each_phase":630,"output_echoes":0,"native_internal_run_records":12,
 "generated_Report_calls":112,"generated_typed_fields":1232,"generated_input_values":1792,"extra_CLI_outputs":0,
 "extra_saved_replay_outputs":0,"wire_refusals":310,"tamper_refusals":18}

def source_names(include_pair=False):
 names=list(BASE_SOURCES)+list(MANIFESTS.values())+["tools/ipv4-native-source/"+n for n in TEMPLATES]
 names += ["cmd/ipv4-oracle/prefix_rule_pair.go", "cmd/ipv4-oracle/prefix_rule_pair_test.go"]
 if include_pair:names+=list(PAIR_SOURCES)+["tools/ipv4-native-source/"+n for n in PAIR_TEMPLATES]
 return names

def pair_specs():
 return [{"label":"prefix-contains","entry":"app/ipv4-prefix-probe:Contains"},
         {"label":"prefix-overlaps","entry":"app/ipv4-prefix-probe:Overlaps"},
         {"label":"rule-pair","entry":"app/ipv4-rule-pair:Report"}]


def require(condition, message):
    if not condition:
        raise ValueError(message)


def strict_object(items):
    out = {}
    for key, value in items:
        require(key not in out, 'Duplicate JSON field')
        out[key] = value
    return out


def read_json(raw, diagnostics=False):
    value = json.loads(raw.decode('utf-8'), object_pairs_hook=strict_object, parse_float=Decimal,
                       parse_constant=lambda _: (_ for _ in ()).throw(ValueError('Nonfinite JSON')))
    def no_decimal(item):
        require(type(item) is not Decimal, 'Noninteger metadata')
        if type(item) is dict:
            for child in item.values():
                no_decimal(child)
        elif type(item) is list:
            for child in item:
                no_decimal(child)
    if not diagnostics:
        no_decimal(value)
    return value


def load(path, diagnostics=False):
    return read_json(regular_bytes(Path(path), PROOF_CAP), diagnostics)


def regular_bytes(path, limit):
    require(path.is_absolute() and path.resolve() == path, 'Canonical absolute regular file required')
    before = path.lstat()
    require(stat.S_ISREG(before.st_mode) and 0 <= before.st_size <= limit, 'Regular file byte bound')
    raw = path.read_bytes()
    after = path.lstat()
    require((before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns) ==
            (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns) and len(raw) == before.st_size,
            'File changed during read')
    return raw


def ref(path):
    path = Path(path)
    raw = regular_bytes(path, 128 << 20)
    return {'path': str(path), 'bytes': len(raw), 'sha256': hashlib.sha256(raw).hexdigest()}


def check_ref(item):
    require(type(item) is dict and set(item) == {'path', 'bytes', 'sha256'} and
            type(item['path']) is str and type(item['bytes']) is int and item['bytes'] >= 0 and
            type(item['sha256']) is str and len(item['sha256']) == 64,
            'Malformed raw reference')
    require(ref(Path(item['path'])) == item, 'Current referenced bytes differ: ' + item['path'])


def save_new(path, value):
    raw = (json.dumps(value, indent=2) + '\n').encode('utf-8')
    with Path(path).open('xb') as out:
        out.write(raw)


def module_from(raw, path, name):
    sys.dont_write_bytecode = True
    module = types.ModuleType(name)
    module.__file__ = str(path)
    exec(compile(raw, str(path), 'exec'), module.__dict__)
    return module


def bind_modules(repository, lifecycle_path, include_pair=False):
    global H, R, Q
    raw = regular_bytes(lifecycle_path, 64 << 10)
    require(hashlib.sha256(raw).hexdigest() == LIFECYCLE_SHA, 'Public lifecycle source differs')
    H = module_from(raw, lifecycle_path, 'ipv4_owned_public_lifecycle')
    H.WORK_CAP, H.PRE_CAP = WORK_CAP, PRE_CAP
    H.STDOUT_CAP, H.STDERR_CAP = STDOUT_CAP, STDERR_CAP
    helper = repository / 'tools/ipv4_receipts.py'
    R = module_from(regular_bytes(helper, 128 << 10), helper, 'ipv4_public_receipts')
    require(R.MANIFESTS == MANIFESTS and R.METHODS == METHODS and R.REV == REV, 'Receipt helper entry/compiler API')
    if include_pair:
        qpath=repository/'tools/prefix_rule_pair_receipts.py'
        Q=module_from(regular_bytes(qpath,128<<10),qpath,'ipv4_public_pair_receipts');Q.configure(R)


def batch_specs():
    rows = []
    for method in METHODS:
        for start in (0, 24):
            rows.append({'label': method + '-' + str(start), 'entry': 'go/net/netip:' + method,
                         'indices': list(range(start, start + 24))})
    for start, count in ((0, 24), (24, 24), (48, 16)):
        rows.append({'label': 'Report-' + str(start), 'entry': REPORT,
                     'indices': list(range(start, start + count))})
    return rows


def stage_names(include_pair=False):
    names = ['go-root', 'compiler-build', 'build-oracle', 'build-bridge', 'oracle202']
    for b in batch_specs():
        names += [b['label'] + '-' + phase for phase in ('execute', 'replay', 'digest')]
    names += ['generated-abi', 'harness-tests', 'build-harness', 'generated202']
    if include_pair:
        names += ['prefix-rule-pair-oracle']
        for b in pair_specs():names += [b['label']+'-'+phase for phase in ('execute','replay','digest')]
        names += ['pair-generated-abi','pair-harness-tests','build-pair-harness','pair-generated112']
    return names


def expected_stage_specs(repository, output, work, tools, include_pair=False):
    go,compiler=tools[0]['path'],tools[1]['path']
    commands=[['go-root',[go,'env','-json','GOROOT','GOVERSION','GOOS','GOARCH']],
              ['compiler-build',[compiler,'version','--build','--json']],
              ['build-oracle',[go,'-C',str(work/'oracle'),'build','-trimpath','-o',str(work/'bin/oracle'),'.']],
              ['build-bridge',[go,'-C',str(work/'bridge'),'build','-trimpath','-o',str(work/'bin/bridge'),'.']],
              ['oracle202',[str(work/'bin/oracle'),'--corpus',str(repository/'examples/ipv4-address-facts/corpus-202.json')]]]
    for b in batch_specs():
        label=b['label'];inputs=str(output/(label+'-inputs.json'));manifest=str(repository/MANIFESTS[b['entry']])
        execution=str(output/(label+'-execute.stdout'))
        commands += [[label+'-execute',[compiler,'package','execute','--json','--go',go,'--inputs',inputs,manifest]],
                     [label+'-replay',[compiler,'package','replay','--json','--go',go,'--receipt',execution,'--inputs',inputs,manifest]],
                     [label+'-digest',[str(work/'bin/bridge'),'--receipt',execution]]]
    commands += [['generated-abi',[str(work/'bin/bridge'),'--check-generated',str(output/'generated.go.txt')]],
                 ['harness-tests',[go,'-C',str(work/'harness'),'test','-count=1','-timeout=45s','.']],
                 ['build-harness',[go,'-C',str(work/'harness'),'build','-trimpath','-o',str(work/'bin/harness'),'.']],
                 ['generated202',[str(work/'bin/harness'),'--boundary',str(output/'generated-boundary.json'),
                                  '--rows',str(output/'rows.jsonl'),'--summary',str(output/'generated-summary.json')]]]
    if include_pair:
        commands += [['prefix-rule-pair-oracle',[str(work/'bin/oracle'),'--profile','prefix-rule-pair','--prefix-corpus',str(repository/'examples/ipv4-prefix-probe/corpus-18.json'),'--corpus',str(repository/'examples/ipv4-rule-pair/corpus-112.json'),'--contract',str(repository/'examples/ipv4-rule-pair/contract.json')]]]
        for b in pair_specs():
            label=b['label'];ip=str(output/(label+'-inputs.json'));mp=str(repository/Q.MANIFESTS[b['entry']]);ex=str(output/(label+'-execute.stdout'))
            commands += [[label+'-execute',[compiler,'package','execute','--json','--go',go,'--inputs',ip,mp]],[label+'-replay',[compiler,'package','replay','--json','--go',go,'--receipt',ex,'--inputs',ip,mp]],[label+'-digest',[str(work/'bin/bridge'),'--receipt',ex,'--record-runtime']]]
        commands += [['pair-generated-abi',[str(work/'bin/bridge'),'--abi-profile','prefix-rule-pair','--check-generated',str(output/'pair-generated.go.txt')]],['pair-harness-tests',[go,'-C',str(work/'pair-harness'),'test','-count=1','-timeout=45s','.']],['build-pair-harness',[go,'-C',str(work/'pair-harness'),'build','-trimpath','-o',str(work/'bin/pair-harness'),'.']],['pair-generated112',[str(work/'bin/pair-harness'),'--boundary',str(output/'pair-generated-boundary.json'),'--rows',str(output/'pair-rows.jsonl'),'--summary',str(output/'pair-generated-summary.json')]]]
    return [{'name':name,'argv':argv,'cwd':str(repository),'expected_exit_code':0} for name,argv in commands]


def input_document(rows, spec):
    inputs = []
    for index in spec['indices']:
        inputs.append({spec['entry'] + '.input' + str(i): rows[index]['inputs'][field]
                       for i, field in enumerate(('a', 'b', 'c', 'd'))})
    raw = (json.dumps({'schema': 'gooo/body-composition-inputs/v1', 'inputs': inputs}, separators=(',', ':')) + '\n').encode()
    require(len(raw) <= 8192, 'Original input envelope exceeds8192bytes')
    return raw


def check_counts(summary):
    require(summary.get('schema') == ('gooo/ipv4-public-sidecar/v2' if summary.get('include_prefix_rule_pair') else 'gooo/ipv4-public-sidecar/v1') and
            summary.get('status') == ('IPV4_PREFIX_RULE_PAIR_CURRENT_NATIVE_REPLAY_GENERATED_PASS' if summary.get('include_prefix_rule_pair') else 'IPV4_CURRENT_NATIVE_REPLAY_GENERATED_PASS') and
            summary.get('compiler_revision') == REV and summary.get('go_version') == GO_VERSION,
            'Summary identity differs')
    for key, value in COUNTS.items():
        require(type(summary.get(key)) is int and summary[key] == (68 if key=='top_level_stages' and summary.get('include_prefix_rule_pair') else value), 'Summary count differs: ' + key)
    require(type(summary.get('include_prefix_rule_pair',False)) is bool,'Exact boolean mode')
    if summary.get('include_prefix_rule_pair'):
        for k,v in PAIR_COUNTS.items():require(type(summary['prefix_rule_pair'].get(k)) is int and summary['prefix_rule_pair'][k]==v,'Pair count differs: '+k)


def check_generated_rows(path, oracle_rows):
    raw = regular_bytes(path, 2 << 20)
    require(raw.endswith(b'\n') and len(oracle_rows) == 202, 'Generated rows final LF/oracle count')
    lines = raw.splitlines()
    require(len(lines) == 202, 'Generated row count')
    for index, (line, expected) in enumerate(zip(lines, oracle_rows)):
        row = read_json(line)
        require(type(row) is dict and set(row) == {'index', 'name', 'cohort', 'inputs', 'actual'}, 'Generated row keys')
        require(type(row['index']) is int and row['index'] == index and
                R.exact({k: row[k] for k in ('index', 'name', 'cohort', 'inputs')},
                        {k: expected[k] for k in ('index', 'name', 'cohort', 'inputs')}), 'Generated row index/original input order')
        require(type(row['actual']) is dict and set(row['actual']) == set(R.FIELDS), 'Generated required11fields')
        require(all(type(row['actual'][k]) is int for k in R.FIELDS[:4]) and
                all(type(row['actual'][k]) is bool for k in R.FIELDS[4:]), 'Generated field exact kinds')
        require(R.exact(row['actual'], expected['expected']), 'Generated output differs from independent Go')
        require(all(R.exact(row['actual'][k], row['inputs'][k]) for k in R.FIELDS[:4]), 'Generated original echo')
    return {'rows': ref(path), 'generated_Report_calls': 202, 'typed_fields': 2222, 'original_echoes': 808}


class Attempt:
    def __init__(self, args):
        self.repo, self.out = args.repository.resolve(), args.out.resolve()
        self.go, self.compiler = args.go.resolve(), args.compiler.resolve()
        self.overall = args.timeout; self.include_pair = args.include_prefix_rule_pair
        require(type(self.overall) is int and 1 <= self.overall <= 180, 'Timeout must be1..180seconds')
        require(not self.out.exists() and self.out != self.repo and
                (self.repo not in self.out.parents or self.repo/'out' in self.out.parents),
                'New proof output must be external or below repository out/')
        # CI convention uses untracked out/ below the checkout; only its source
        # closure is frozen. A tracked source may never be selected as output.
        self.started = time.monotonic(); self.stages = []; self.stage_specs = []
        self.work = None; self.owner = None; self.owns_out = False
        self.groups = set(); self.children = {}; self.reaped = set(); self.secondary_errors = []
        self.pending_process = None; self.last_group_observations = []; self.work_cleanup_complete = False
        self.runtime_refs = []; self.producers = []; self.module_copies = []; self.modules = []
        names = source_names(self.include_pair)
        require(len(names) == len(set(names)), 'Source closure duplicate')
        self.sources = [ref(self.repo / name) for name in names]
        self.tools = [ref(self.go), ref(self.compiler), ref(Path(sys.executable).resolve()),
                      ref(self.go.parent.parent / 'src/net/netip/netip.go'), ref(self.go.parent.parent / 'LICENSE')]
        require(self.tools[3]['sha256'] == NETIP_SHA and
                ref(self.repo / 'third_party/go/netip.go.txt')['sha256'] == NETIP_SHA, 'Pinned/current netip source differs')
        require(ref(self.repo / 'cmd/check-path-facts/result_digest.go')['sha256'] == CODEC_SHA and
                ref(self.repo / 'cmd/check-path-facts/result-profile-go1273.json')['sha256'] == PROFILE_SHA,
                'Unchanged public Result codec/profile required')

    def prepare(self):
        self.out.mkdir(parents=True); self.owns_out = True
        self.work = Path(tempfile.mkdtemp(prefix='gooo-ipv4-')).resolve(); self.owner = self.work.stat()
        for name in ('cache', 'gopath', 'module-cache', 'tmp', 'bin', 'telemetry', 'config'):
            (self.work / name).mkdir()
        copied = self.work / 'public-path-lifecycle.py'
        copied.write_bytes(regular_bytes(self.repo / 'tools/check-path-facts.py', 64 << 10))
        self.module_copies.append({'copy': ref(copied), 'source': ref(self.repo / 'tools/check-path-facts.py')})
        bind_modules(self.repo, copied, self.include_pair)
        self.env = {key: os.environ[key] for key in ('HOME', 'USER', 'LOGNAME', 'SHELL', 'LANG', 'LC_ALL', 'LC_CTYPE') if key in os.environ}
        self.env.update(GOCACHE=str(self.work/'cache'), GOPATH=str(self.work/'gopath'), GOMODCACHE=str(self.work/'module-cache'),
                        GOTMPDIR=str(self.work/'tmp'), TMPDIR=str(self.work/'tmp'), GOROOT=str(self.go.parent.parent),
                        GOTOOLCHAIN='local', GOWORK='off', GOENV='off', GOFLAGS='', GOPROXY='off', GOSUMDB='off',
                        CGO_ENABLED='0', GOTELEMETRY='off', TEST_TELEMETRY_DIR=str(self.work/'telemetry'),
                        XDG_CONFIG_HOME=str(self.work/'config'), PATH=str(self.go.parent)+':/usr/bin:/bin:/usr/sbin:/sbin',
                        PYTHONDONTWRITEBYTECODE='1')
        mode = self.work/'telemetry/mode'; mode.write_text('off '+time.strftime('%Y-%m-%d',time.gmtime()),encoding='ascii')
        self.module_copies.append({'copy': ref(mode), 'source': None})
        self.freeze_path = self.out/'source-freeze.json'
        save_new(self.freeze_path, {'schema':'gooo/ipv4-public-source-freeze/v1','repository_root':str(self.repo),
                 'files':self.sources,'tools':self.tools,'overall_seconds':self.overall})
        self.freeze = ref(self.freeze_path)

    def frozen(self, live=True):
        for item in self.sources + self.tools + [self.freeze] + self.runtime_refs:
            check_ref(item)
        if live:
            for item in self.producers + [v['copy'] for v in self.module_copies]:
                check_ref(item)
            for module in self.modules:
                root = Path(module['root'])
                require(root.is_dir() and not root.is_symlink() and root.resolve() == root and
                        sorted(p.name for p in root.iterdir()) == sorted(module['names']), 'Owned module exact file roster')
                require(all(stat.S_ISREG(p.lstat().st_mode) for p in root.iterdir()), 'Owned module contains nonregular source')

    def budget(self, active=False):
        require(time.monotonic()-self.started < self.overall, 'Overall IPv4 verification timeout')
        require(H.logical(self.work, active=active) <= WORK_CAP, 'Owned work exceeds384MiB')
        require(H.logical(self.out) <= TOTAL_PROOF_CAP, 'Retained proof exceeds64MiB')
        require(shutil.disk_usage(self.work).free >= FREE_MIN, 'Require5GiB free')

    def invoke(self, name, argv, seconds=60):
        require(name == stage_names(self.include_pair)[len(self.stage_specs)], 'Unexpected process stage order')
        self.stage_specs.append({'name':name,'argv':list(map(str,argv)),'cwd':str(self.repo),'expected_exit_code':0})
        output=H.Attempt.invoke(self, name, list(map(str,argv)), seconds=seconds)
        terminal=H.load_process(Path(self.stages[-1]['path']))
        elapsed,limit=terminal['elapsed_seconds'],terminal['timeout_seconds']
        require(type(elapsed) in (int,Decimal) and type(limit) in (int,Decimal) and 0<=elapsed<limit,'Terminal child deadline exceeded')
        self.budget();return output

    def require_groups_gone(self):
        H.Attempt.require_groups_gone(self)

    def save_raw(self, name, raw):
        path = self.out/(name+'.raw')
        with path.open('xb') as output:
            output.write(raw)
        item = ref(path); self.runtime_refs.append(item); return item

    def save(self, name, value):
        path = self.out/name; save_new(path,value); item = ref(path); self.runtime_refs.append(item); return item

    def make_module(self, name, copies, text_files=None):
        root = self.work/name; root.mkdir()
        for target, source in copies:
            (root/target).write_bytes(regular_bytes(source, 128 << 20))
            self.module_copies.append({'copy':ref(root/target),'source':ref(source)})
        for target, raw in (text_files or {}).items():
            (root/target).write_bytes(raw)
            self.module_copies.append({'copy':ref(root/target),'source':None})
        self.modules.append({'root':str(root),'names':sorted(p.name for p in root.iterdir())})
        self.frozen(); return root

    def run(self):
        go_root_path = self.invoke('go-root',[self.go,'env','-json','GOROOT','GOVERSION','GOOS','GOARCH'])
        selected = load(go_root_path)
        require(set(selected)=={'GOROOT','GOVERSION','GOOS','GOARCH'} and selected['GOVERSION']==GO_VERSION and
                Path(selected['GOROOT']).resolve()==self.go.parent.parent, 'Actual selected Go toolchain differs')
        self.selected_version = 'go version '+GO_VERSION+' '+selected['GOOS']+'/'+selected['GOARCH']
        build_path = self.invoke('compiler-build',[self.compiler,'version','--build','--json'])
        build = load(build_path)
        for key,want in {'schema':'gooo/build-identity/v1','version':'0.6.37-dev','compiler_source_sha':REV,
                         'vcs_revision':REV,'vcs_modified':'false','source_status':'CLEAN_VCS',
                         'go_version':GO_VERSION,'native_go_required':GO_VERSION}.items():
            require(build.get(key)==want, 'Pinned compiler build identity differs: '+key)
        module_text = b'module gooo.local/ipv4-sidecar\n\ngo 1.27.3\n'
        oracle_module = self.make_module('oracle',[('main.go',self.repo/'cmd/ipv4-oracle/main.go'),('prefix_rule_pair.go',self.repo/'cmd/ipv4-oracle/prefix_rule_pair.go')],{'go.mod':module_text})
        bridge = self.make_module('bridge',[(n,self.repo/p) for n,p in (
            ('main.go','tools/ipv4-native-source/bridge-main.go.txt'),('support.go','tools/ipv4-native-source/support.go.txt'),
            ('result_digest.go','cmd/check-path-facts/result_digest.go'),('result-profile-83928fa.json','cmd/check-path-facts/result-profile-83928fa.json'),('result-profile-go1273.json','cmd/check-path-facts/result-profile-go1273.json'))],{'go.mod':module_text})
        self.oracle_binary=self.work/'bin/oracle';self.bridge_binary=self.work/'bin/bridge'
        self.invoke('build-oracle',[self.go,'-C',oracle_module,'build','-trimpath','-o',self.oracle_binary,'.']); self.producers.append(ref(self.oracle_binary))
        self.invoke('build-bridge',[self.go,'-C',bridge,'build','-trimpath','-o',self.bridge_binary,'.']); self.producers.append(ref(self.bridge_binary))
        corpus_path=self.repo/'examples/ipv4-address-facts/corpus-202.json'
        require(ref(corpus_path)['sha256']==CORPUS_SHA,'Public202 corpus differs')
        oracle_path=self.invoke('oracle202',[self.oracle_binary,'--corpus',corpus_path])
        self.oracle=load(oracle_path);require(self.oracle.get('schema')=='gooo/ipv4-independent-oracle/v1','Oracle schema')
        self.oracle_rows=self.oracle['cases'];checker=R.Evidence(self.repo,self.oracle_rows,self.selected_version)
        self.runtime_refs.append(ref(oracle_path))
        wire=R.run_wire_controls(self.save_raw)
        self.wire_ref=self.save('wire-controls.json',[{k:v for k,v in row.items() if k!='raw'} for row in wire])
        self.batches=[];total_fields=total_echoes=total_native=0
        for spec in batch_specs():
            label=spec['label'];entry=spec['entry'];manifest=self.repo/MANIFESTS[entry]
            raw=input_document(self.oracle_rows,spec);R.validate_inputs(raw,entry)
            input_path=self.out/(label+'-inputs.json')
            with input_path.open('xb') as stream:stream.write(raw)
            self.runtime_refs.append(ref(input_path))
            execution=self.invoke(label+'-execute',[self.compiler,'package','execute','--json','--go',self.go,'--inputs',input_path,manifest])
            exraw=regular_bytes(execution,STDOUT_CAP)
            checked=checker.inspect(exraw,entry,spec['indices'],input_path,manifest,'sha256:'+self.tools[0]['sha256'])
            replay=self.invoke(label+'-replay',[self.compiler,'package','replay','--json','--go',self.go,'--receipt',execution,'--inputs',input_path,manifest])
            dg_path=self.invoke(label+'-digest',[self.bridge_binary,'--receipt',execution]);dg=load(dg_path)
            require(dg.get('schema')=='gooo/ipv4-pinned-result-digest/v1' and dg.get('receipt')==ref(execution) and
                    dg.get('compiler_revision')==REV and R.is_digest(dg.get('typed_result_sha256')), 'Pinned digest bridge receipt binding')
            rraw=regular_bytes(replay,STDOUT_CAP)
            repeated=checker.inspect(rraw,entry,spec['indices'],input_path,manifest,'sha256:'+self.tools[0]['sha256'],original=exraw,prior_typed=dg['typed_result_sha256'])
            require(R.exact(checked,repeated),'Native/replay checked results differ')
            checked_ref=self.save(label+'-checked.json',checked)
            record={**spec,'inputs':ref(input_path),'manifest':ref(manifest),'execution':ref(execution),'replay':ref(replay),
                    'typed_digest':ref(dg_path),'checked':checked_ref}
            self.batches.append(record)
            total_fields+=checked['typed_scalar_fields'];total_echoes+=checked['original_echoes'];total_native+=checked['native_internal_runs']+repeated['native_internal_runs']
        require(total_fields==992 and total_echoes==256 and total_native==60,'Actual manual field/echo/native counts')
        report_batch=self.batches[12]
        exraw=regular_bytes(Path(report_batch['execution']['path']),STDOUT_CAP);rraw=regular_bytes(Path(report_batch['replay']['path']),STDOUT_CAP)
        prior=load(Path(report_batch['typed_digest']['path']))['typed_result_sha256']
        tampers=R.check_tampers(checker,exraw,rraw,REPORT,report_batch['indices'],Path(report_batch['inputs']['path']),self.repo/MANIFESTS[REPORT],
                               'sha256:'+self.tools[0]['sha256'],prior,self.save_raw,H.value_span)
        self.tamper_ref=self.save('tamper-controls.json',tampers)
        generated=self.generated_layer(corpus_path)
        pair = self.pair_layer() if self.include_pair else None
        self.audit_live()
        return {'schema':'gooo/ipv4-public-sidecar/v1','status':'VALUES_CHECKED_PENDING_CLEANUP','compiler_revision':REV,'go_version':GO_VERSION,
                **COUNTS,'top_level_stages':68 if self.include_pair else 54,**({'include_prefix_rule_pair':True,'prefix_rule_pair':pair} if self.include_pair else {}),'repository_root':str(self.repo),'selected_go_version':self.selected_version,'source_freeze':self.freeze,
                'oracle':ref(oracle_path),'batches':self.batches,'generated':generated,'wire_controls':self.wire_ref,'tamper_controls':self.tamper_ref,
                'stages':self.stages,'stage_specs':self.stage_specs,'retained_runtime_refs':self.runtime_refs,
                'producer_lifetime':{'retained_binaries':False,'before_cleanup':self.producers,'owned_module_copies':self.module_copies,
                                     'owned_modules':self.modules,'all_live_refs_matched_after_final_stage':True},
                'scope':'352 fresh CLI outputs and352 saved replay outputs; separate202 direct Report calls/2222fields/808echoes. No202 direct-entry transport/fullIPv4/security/performance claim.'}

    def generated_layer(self, corpus_path):
        reports=[load(Path(b['checked']['path'])) for b in self.batches[12:]]
        require(len(reports)==3 and len({r['generated_sha256'] for r in reports})==1 and
                all(r['generated_source']==reports[0]['generated_source'] for r in reports), 'Fresh Report sources differ across batches')
        generated_raw=reports[0]['generated_source'].encode('utf-8')
        require('sha256:'+hashlib.sha256(generated_raw).hexdigest()==reports[0]['generated_sha256'],'Generated fresh source hash')
        generated_path=self.out/'generated.go.txt'
        with generated_path.open('xb') as stream:stream.write(generated_raw)
        self.runtime_refs.append(ref(generated_path))
        abi_path=self.invoke('generated-abi',[self.bridge_binary,'--check-generated',generated_path]);abi=load(abi_path)
        require(abi=={'schema':'gooo/ipv4-generated-ABI/v1','generated_source':ref(generated_path),'functions':8,'record_fields':11,'input_ports':4},'Generated exact ABI report')
        module=self.make_module('harness',[(n,self.repo/'tools/ipv4-native-source'/src) for n,src in (
            ('main.go','harness-main.go.txt'),('support.go','support.go.txt'),('harness_test.go','harness-test.go.txt'))]+[('generated.go',generated_path)],
            {'go.mod':b'module gooo.local/ipv4-sidecar\n\ngo 1.27.3\n'})
        self.invoke('harness-tests',[self.go,'-C',module,'test','-count=1','-timeout=45s','.'],seconds=60)
        binary=self.work/'bin/harness'
        self.invoke('build-harness',[self.go,'-C',module,'build','-trimpath','-o',binary,'.']);self.producers.append(ref(binary))
        boundary={'schema':'gooo/ipv4-generated-boundary/v1','source_freeze':self.freeze,'generated_source':ref(module/'generated.go'),
                  'corpus':ref(corpus_path),'oracle':ref(self.out/'oracle202.stdout'),'abi':ref(abi_path),'go_tool':self.tools[0],'binary':ref(binary),
                  'module_sources':[ref(module/n) for n in ('go.mod','main.go','support.go','harness_test.go','generated.go')],
                  'report_receipts':[b[k] for b in self.batches[12:] for k in ('execution','replay')],
                  'report_checked':[b['checked'] for b in self.batches[12:]]}
        boundary_ref=self.save('generated-boundary.json',boundary)
        rows=self.out/'rows.jsonl';summary=self.out/'generated-summary.json'
        self.invoke('generated202',[binary,'--boundary',boundary_ref['path'],'--rows',rows,'--summary',summary])
        observed=load(summary)
        require(observed.get('schema')=='gooo/ipv4-generated202/v1' and observed.get('status')=='ACTUAL_FUNCTION_ROWS_WRITTEN_AND_READ_BACK' and
                all(type(observed.get(k)) is int and observed[k]==v for k,v in {'generated_Report_calls':202,'readback_rows':202,'extra_CLI_outputs':0,'extra_saved_replay_outputs':0}.items()) and
                observed.get('boundary')==boundary_ref and observed.get('rows')==ref(rows),'Actual generated harness report')
        checked=check_generated_rows(rows,self.oracle_rows)
        for p in (summary,rows):self.runtime_refs.append(ref(p))
        return {**checked,'summary':ref(summary),'boundary':boundary_ref,'generated_source':ref(generated_path),
                'independent_oracle_cases':202,'extra_CLI_outputs':0,'extra_saved_replay_outputs':0}

    def pair_layer(self):
        op=self.invoke('prefix-rule-pair-oracle',[self.oracle_binary,'--profile','prefix-rule-pair','--prefix-corpus',self.repo/'examples/ipv4-prefix-probe/corpus-18.json','--corpus',self.repo/'examples/ipv4-rule-pair/corpus-112.json','--contract',self.repo/'examples/ipv4-rule-pair/contract.json'])
        oracle=load(op);check_pair_oracle(oracle,self.repo);self.runtime_refs.append(ref(op))
        checker=Q.Evidence(self.repo,oracle,self.selected_version,R)
        wire=self.save('pair-wire-controls.json',Q.run_wire(self.save_raw));batches=[];fields=originals=internal=0
        for spec in pair_specs():
            label,entry=spec['label'],spec['entry'];manifest=self.repo/Q.MANIFESTS[entry]
            path=self.out/(label+'-inputs.json')
            with path.open('xb') as stream:stream.write(Q.input_document(checker.rows[entry][:18],entry))
            self.runtime_refs.append(ref(path))
            ex=self.invoke(label+'-execute',[self.compiler,'package','execute','--json','--go',self.go,'--inputs',path,manifest]);exraw=regular_bytes(ex,STDOUT_CAP)
            checked=checker.inspect(exraw,entry,path,manifest,'sha256:'+self.tools[0]['sha256'])
            replay=self.invoke(label+'-replay',[self.compiler,'package','replay','--json','--go',self.go,'--receipt',ex,'--inputs',path,manifest])
            dgpath=self.invoke(label+'-digest',[self.bridge_binary,'--receipt',ex,'--record-runtime']);dg=load(dgpath)
            require(dg.get('schema')=='gooo/ipv4-pinned-result-digest/v1' and dg.get('receipt')==ref(ex) and dg.get('compiler_revision')==REV and dg.get('runtime_profile')=='gooo/ipv4-record-runtime-result/v1' and dg.get('runtime_profile_sha256')==RECORD_PROFILE_SHA and R.is_digest(dg.get('typed_result_sha256')),'Pair pinned Result digest')
            repeated=checker.inspect(regular_bytes(replay,STDOUT_CAP),entry,path,manifest,'sha256:'+self.tools[0]['sha256'],original=exraw,prior_typed=dg['typed_result_sha256'])
            require(R.exact(checked,repeated),'Pair execute/replay values differ')
            batches.append({**spec,'inputs':ref(path),'manifest':ref(manifest),'execution':ref(ex),'replay':ref(replay),'typed_digest':ref(dgpath),'checked':self.save(label+'-checked.json',checked)})
            fields+=checked['typed_scalar_fields'];originals+=checked['original_input_values'];internal+=checked['native_internal_runs']+repeated['native_internal_runs']
        require(fields==234 and originals==630 and internal==12,'Observed pair-layer counters')
        batch=batches[-1];exraw=regular_bytes(Path(batch['execution']['path']),STDOUT_CAP);rraw=regular_bytes(Path(batch['replay']['path']),STDOUT_CAP);prior=load(Path(batch['typed_digest']['path']))['typed_result_sha256']
        tamper=self.save('pair-tamper-controls.json',Q.check_tampers(checker,exraw,rraw,Path(batch['inputs']['path']),Path(batch['manifest']['path']),'sha256:'+self.tools[0]['sha256'],prior,self.save_raw,H.value_span))
        checked=load(Path(batch['checked']['path']));raw=checked['generated_source'].encode();gp=self.out/'pair-generated.go.txt'
        require(R.digest(raw)==checked['generated_sha256'],'Pair fresh source SHA')
        with gp.open('xb') as stream:stream.write(raw)
        self.runtime_refs.append(ref(gp))
        abi=self.invoke('pair-generated-abi',[self.bridge_binary,'--abi-profile','prefix-rule-pair','--check-generated',gp])
        require(load(abi)=={'schema':'gooo/ipv4-rule-pair-generated-ABI/v1','generated_source':ref(gp),'functions':5,'records':4,'record_fields':28,'root_fields':11,'input_ports':2},'Pair exact generated ABI')
        module=self.make_module('pair-harness',[(n,self.repo/'tools/ipv4-native-source'/src) for n,src in (('main.go','pair-harness-main.go.txt'),('support.go','support.go.txt'),('harness_test.go','pair-harness-test.go.txt'))]+[('generated.go',gp)],{'go.mod':b'module gooo.local/ipv4-sidecar\n\ngo 1.27.3\n'})
        self.invoke('pair-harness-tests',[self.go,'-C',module,'test','-count=1','-timeout=45s','.'])
        binary=self.work/'bin/pair-harness';self.invoke('build-pair-harness',[self.go,'-C',module,'build','-trimpath','-o',binary,'.']);self.producers.append(ref(binary))
        boundary={'schema':'gooo/ipv4-rule-pair-generated-boundary/v1','source_freeze':self.freeze,'generated_source':ref(module/'generated.go'),'corpus':ref(self.repo/'examples/ipv4-rule-pair/corpus-112.json'),'oracle':ref(op),'abi':ref(abi),'go_tool':self.tools[0],'binary':ref(binary),'module_sources':[ref(module/n) for n in ('go.mod','main.go','support.go','harness_test.go','generated.go')],'report_receipts':[batch[k] for k in ('execution','replay')],'report_checked':[batch['checked']]}
        bound=self.save('pair-generated-boundary.json',boundary);rows=self.out/'pair-rows.jsonl';summary=self.out/'pair-generated-summary.json'
        self.invoke('pair-generated112',[binary,'--boundary',bound['path'],'--rows',rows,'--summary',summary]);observed=load(summary)
        require(observed.get('schema')=='gooo/ipv4-rule-pair-generated112/v1' and observed.get('status')=='ACTUAL_FUNCTION_ROWS_WRITTEN_AND_READ_BACK' and all(type(observed.get(k)) is int and observed[k]==v for k,v in {'generated_Report_calls':112,'readback_rows':112,'extra_CLI_outputs':0,'extra_saved_replay_outputs':0}.items()) and observed.get('boundary')==bound and observed.get('rows')==ref(rows),'Pair actual harness report')
        check_pair_rows(rows,oracle['pair_cases'])
        for path in (rows,summary):self.runtime_refs.append(ref(path))
        return {**PAIR_COUNTS,'oracle':ref(op),'batches':batches,'wire_controls':wire,'tamper_controls':tamper,'generated':{'rows':ref(rows),'summary':ref(summary),'boundary':bound,'generated_source':ref(gp)}}

    def audit_live(self):
        self.frozen();audit_stages(self.stages,self.stage_specs,self.freeze,self.repo,self.include_pair)
        require(R.exact(self.stage_specs,expected_stage_specs(self.repo,self.out,self.work,self.tools,self.include_pair)),
                'Source-derived exact54 stage commands')
        self.require_groups_gone();self.budget()

    def cleanup(self):
        if self.work is None:return
        self.require_groups_gone()
        current=self.work.lstat()
        require(stat.S_ISDIR(current.st_mode) and not self.work.is_symlink() and
                (current.st_dev,current.st_ino)==(self.owner.st_dev,self.owner.st_ino),'Owned work identity changed')
        size=H.logical(self.work);shutil.rmtree(self.work)
        save_new(self.out/'work-cleanup.json',{'owned_path':str(self.work),'owned_device':self.owner.st_dev,'owned_inode':self.owner.st_ino,
                 'logical_bytes_removed':size,'all_top_level_groups_reaped':True,'group_observations':self.last_group_observations,'removed':True})
        self.work_cleanup_complete=True


def check_pair_oracle(oracle,repository):
    require(oracle.get('schema')=='gooo/ipv4-prefix-rule-pair-independent-oracle/v1' and oracle.get('go_version')==GO_VERSION and oracle.get('netip_source_sha256')==NETIP_SHA,'Combined oracle identity')
    for key,n in (('prefix_case_count',18),('pair_case_count',112)):
        require(type(oracle.get(key)) is int and oracle[key]==n,'Combined oracle population')
    for key,name in (('prefix_corpus','examples/ipv4-prefix-probe/corpus-18.json'),('pair_corpus','examples/ipv4-rule-pair/corpus-112.json'),('contract','examples/ipv4-rule-pair/contract.json')):
        require(oracle.get(key)==ref(repository/name),'Combined oracle current raw source binding')


def check_pair_rows(path,rows):
    raw=regular_bytes(path,2<<20);lines=raw.splitlines()
    require(raw.endswith(b'\n') and len(lines)==len(rows)==112,'Pair row count/LF')
    for i,(line,reference) in enumerate(zip(lines,rows)):
        row=read_json(line)
        require(type(row) is dict and set(row)=={'index','name','inputs','actual'} and type(row['index']) is int and row['index']==i and row['name']==reference['name'] and R.exact(row['inputs'],reference['inputs']),'Pair row original input/order')
        require(type(row['actual']) is dict and set(row['actual'])==set(Q.FACTS) and all(type(row['actual'][k]) is bool for k in Q.FACTS[:6]) and all(R.integer(row['actual'][k]) for k in Q.FACTS[6:]),'Pair row exact fact kinds')
        require(R.exact(row['actual'],reference['expected']),'Pair generated values differ from independent Go')


def audit_pair(summary,repository,output,work,frozen,copies):
    pair=summary['prefix_rule_pair'];check_ref(pair['oracle']);oracle=load(Path(pair['oracle']['path']));check_pair_oracle(oracle,repository)
    checker=Q.Evidence(repository,oracle,summary['selected_go_version'],R)
    require(len(pair['batches'])==3,'Exact3 pair-layer batches')
    for spec,b in zip(pair_specs(),pair['batches']):
        require(all(R.exact(b[k],v) for k,v in spec.items()),'Pair batch identity/order')
        for k in ('inputs','manifest','execution','replay','typed_digest','checked'):check_ref(b[k])
        exraw=regular_bytes(Path(b['execution']['path']),STDOUT_CAP);rraw=regular_bytes(Path(b['replay']['path']),STDOUT_CAP);dg=load(Path(b['typed_digest']['path']))
        require(dg.get('schema')=='gooo/ipv4-pinned-result-digest/v1' and dg.get('compiler_revision')==REV and dg.get('receipt')==b['execution'] and dg.get('runtime_profile')=='gooo/ipv4-record-runtime-result/v1' and dg.get('runtime_profile_sha256')==RECORD_PROFILE_SHA and R.is_digest(dg.get('typed_result_sha256')),'Pair retained typed digest')
        checked=checker.inspect(exraw,b['entry'],Path(b['inputs']['path']),Path(b['manifest']['path']),'sha256:'+frozen['tools'][0]['sha256'])
        repeated=checker.inspect(rraw,b['entry'],Path(b['inputs']['path']),Path(b['manifest']['path']),'sha256:'+frozen['tools'][0]['sha256'],original=exraw,prior_typed=dg['typed_result_sha256'])
        require(R.exact(checked,repeated) and R.exact(checked,load(Path(b['checked']['path']))),'Pair retained receipt/checked values')
    check_ref(pair['wire_controls']);wire=load(Path(pair['wire_controls']['path']));cases=Q.wire_cases();require(len(wire)==310,'Pair310 controls')
    for case,observed in zip(cases,wire):
        check_ref(observed['rawref']);raw=regular_bytes(Path(observed['rawref']['path']),8193)
        require(raw==case['raw'] and all(R.exact(observed[k],case[k]) for k in ('name','entry','code')) and observed['observed_diagnostic']==case['code'],'Pair wire raw/intended gate')
        try:Q.validate_inputs(raw,case['entry'])
        except R.InputError as exc:require(exc.code==case['code'],'Pair retained wire intended diagnostic')
        else:raise ValueError('Pair malformed transport accepted')
    batch=pair['batches'][-1];exraw=regular_bytes(Path(batch['execution']['path']),STDOUT_CAP);rraw=regular_bytes(Path(batch['replay']['path']),STDOUT_CAP)
    check_ref(pair['tamper_controls']);tamper=load(Path(pair['tamper_controls']['path']));cases=Q.tamper_cases(exraw,rraw,H.value_span);require(len(tamper)==18,'Pair18 tamper controls')
    prior=load(Path(batch['typed_digest']['path']))['typed_result_sha256']
    for case,observed in zip(cases,tamper):
        check_ref(observed['rawref']);raw=regular_bytes(Path(observed['rawref']['path']),STDOUT_CAP)
        require(raw==case['raw'] and all(R.exact(observed[k],case[k]) for k in ('name','target','code')) and observed['observed_diagnostic']==case['code'],'Pair tamper raw/intended gate')
        try:checker.inspect(raw,Q.REPORT,Path(batch['inputs']['path']),Path(batch['manifest']['path']),'sha256:'+frozen['tools'][0]['sha256'],original=exraw if case['target']=='replay' else None,prior_typed=prior if case['target']=='replay' else None)
        except R.ReceiptError as exc:require(exc.code==case['code'],'Pair retained tamper diagnostic')
        else:raise ValueError('Pair receipt tamper accepted')
    generated=pair['generated']
    for key in ('rows','summary','boundary','generated_source'):check_ref(generated[key])
    check_pair_rows(Path(generated['rows']['path']),oracle['pair_cases'])
    bound=load(Path(generated['boundary']['path']));native=load(Path(generated['summary']['path']))
    require(native.get('schema')=='gooo/ipv4-rule-pair-generated112/v1' and native.get('status')=='ACTUAL_FUNCTION_ROWS_WRITTEN_AND_READ_BACK' and native.get('boundary')==generated['boundary'] and native.get('rows')==generated['rows'] and all(type(native.get(k)) is int and native[k]==v for k,v in {'generated_Report_calls':112,'readback_rows':112,'extra_CLI_outputs':0,'extra_saved_replay_outputs':0}.items()),'Pair native summary bridge')
    require(bound.get('schema')=='gooo/ipv4-rule-pair-generated-boundary/v1' and bound.get('source_freeze')==summary['source_freeze'] and bound.get('oracle')==pair['oracle'] and bound.get('corpus')==ref(repository/'examples/ipv4-rule-pair/corpus-112.json') and bound.get('go_tool')==frozen['tools'][0] and bound.get('binary')==summary['producer_lifetime']['before_cleanup'][3],'Pair current boundary refs')
    require(bound['module_sources']==[next(v['copy'] for v in copies if v['copy']['path']==str(work/'pair-harness'/n)) for n in ('go.mod','main.go','support.go','harness_test.go','generated.go')] and bound['generated_source']==bound['module_sources'][-1] and bound['report_receipts']==[batch[k] for k in ('execution','replay')] and bound['report_checked']==[batch['checked']],'Pair compiled source/receipt lifetime bridge')
    require(generated['generated_source']['bytes']==bound['generated_source']['bytes'] and generated['generated_source']['sha256']==bound['generated_source']['sha256'],'Pair retained exact generated bytes')
    check_ref(bound['abi']);abi=load(Path(bound['abi']['path']))
    require(abi=={'schema':'gooo/ipv4-rule-pair-generated-ABI/v1','generated_source':generated['generated_source'],'functions':5,'records':4,'record_fields':28,'root_fields':11,'input_ports':2},'Pair retained exact ABI report')


def audit_stages(stages,specs,freeze,repository,include_pair=False):
    require(type(stages) is list and type(specs) is list and len(stages)==len(specs)==(68 if include_pair else 54),'Exact retained stages')
    require(len({s['path'] for s in stages})==(68 if include_pair else 54) and [s['name'] for s in specs]==stage_names(include_pair),'Unique ordered process ledger roster')
    for item,spec in zip(stages,specs):
        check_ref(item);p=H.load_process(Path(item['path']))
        for key,want in {'name':spec['name'],'argv':spec['argv'],'cwd':str(repository),'expected_exit_code':0,'exit_code':0,
                         'automatic_retries':0,'status':'COMPLETED','process_reaped':True,'owned_group_remaining':False,
                         'owned_group_state':{'state':'absent','error':None},'abort':None,'primary_error':None,
                         'secondary_errors':[],'termination_errors':[],'source_tools_unchanged_after':True,
                         'source_freeze_before':freeze,'source_freeze_after':freeze}.items():
            require(R.exact(p.get(key),want),'Process ledger differs: '+key)
        require(type(p.get('pid')) is int and p['pid']>0 and type(p.get('owned_pgid')) is int and p['owned_pgid']==p['pid'],'Owned process/group identity')
        for key in ('stdout','stderr','started_ledger'):check_ref(p[key])
        started=load(Path(p['started_ledger']['path']),diagnostics=True)
        for key in ('name','argv','cwd','pid','owned_pgid','source_freeze_before','timeout_seconds','automatic_retries','expected_exit_code'):
            require(R.exact(started.get(key),p.get(key)),'STARTED/final process binding: '+key)
        require(started.get('status')=='STARTED' and p['stdout']['bytes']<=STDOUT_CAP and p['stderr']['bytes']<=STDERR_CAP,'Process stream/status bound')


def verify_summary(path):
    path=path.resolve();require(not (path.parent/'failure.json').exists(),'Failed attempt cannot be accepted')
    summary=load(path);check_counts(summary);include_pair=summary.get("include_prefix_rule_pair",False)
    repository=Path(summary['repository_root']);freeze=summary['source_freeze'];check_ref(freeze)
    frozen=load(Path(freeze['path']))
    require(frozen.get('schema')=='gooo/ipv4-public-source-freeze/v1' and frozen.get('repository_root')==str(repository),'Current source freeze identity')
    names=source_names(include_pair)
    require([v['path'] for v in frozen['files']]==[str(repository/n) for n in names],'Exact current source closure roster')
    for item in frozen['files']+frozen['tools']:check_ref(item)
    require(len(frozen['tools'])==5 and frozen['tools'][3]['sha256']==NETIP_SHA and
            type(frozen.get('overall_seconds')) is int and 1<=frozen['overall_seconds']<=180,'Pinned source/tool/budget shape')
    require(Path(__file__).resolve()==repository/'tools/check-ipv4.py','Readback must use current bound runner')
    bind_modules(repository,repository/'tools/check-path-facts.py',include_pair)
    check_ref(summary['work_cleanup']);clean=load(Path(summary['work_cleanup']['path']));work=Path(clean['owned_path'])
    require(R.exact(summary['stage_specs'],expected_stage_specs(repository,path.parent,work,frozen['tools'],include_pair)),
            'Retained stage commands differ from current source plan')
    audit_stages(summary['stages'],summary['stage_specs'],freeze,repository,include_pair)
    for item in summary['retained_runtime_refs']:check_ref(item)
    oracle=load(Path(summary['oracle']['path']));check_ref(summary['oracle'])
    checker=R.Evidence(repository,oracle['cases'],summary['selected_go_version'])
    require(len(summary['batches'])==15,'Exact manual batch records')
    for spec,b in zip(batch_specs(),summary['batches']):
        require(all(R.exact(b[k],spec[k]) for k in spec),'Manual batch identity/order')
        for key in ('inputs','manifest','execution','replay','typed_digest','checked'):check_ref(b[key])
        exraw=regular_bytes(Path(b['execution']['path']),STDOUT_CAP);rraw=regular_bytes(Path(b['replay']['path']),STDOUT_CAP)
        dg=load(Path(b['typed_digest']['path']))
        require(dg.get('schema')=='gooo/ipv4-pinned-result-digest/v1' and dg.get('compiler_revision')==REV and
                dg.get('receipt')==b['execution'] and R.is_digest(dg.get('typed_result_sha256')),'Retained typed digest binding')
        original=checker.inspect(exraw,b['entry'],b['indices'],Path(b['inputs']['path']),Path(b['manifest']['path']),'sha256:'+frozen['tools'][0]['sha256'])
        repeated=checker.inspect(rraw,b['entry'],b['indices'],Path(b['inputs']['path']),Path(b['manifest']['path']),'sha256:'+frozen['tools'][0]['sha256'],original=exraw,prior_typed=dg['typed_result_sha256'])
        require(R.exact(original,repeated) and R.exact(original,load(Path(b['checked']['path']))),'Retained receipt or checked values changed')
    wire=load(Path(summary['wire_controls']['path']));check_ref(summary['wire_controls'])
    expected_wire=R.wire_controls();require(len(wire)==42,'Retained42 wire controls')
    for expected,observed in zip(expected_wire,wire):
        check_ref(observed['rawref']);raw=regular_bytes(Path(observed['rawref']['path']),8193)
        require(raw==expected['raw'] and observed['name']==expected['name'] and observed['expected_diagnostic']==expected['expected_diagnostic'] and
                observed['observed_diagnostic']==expected['expected_diagnostic'] and type(observed['baseline_entry_count']) is int and observed['baseline_entry_count']==7,'Wire intended-gate evidence')
        try:R.validate_inputs(raw,REPORT)
        except R.InputError as exc:require(exc.code==expected['expected_diagnostic'],'Retained wire diagnostic changed')
        else:raise ValueError('Retained malformed input accepted')
    tamper=load(Path(summary['tamper_controls']['path']));check_ref(summary['tamper_controls']);require(len(tamper)==18,'Retained18 tamper controls')
    first=summary['batches'][12];exraw=regular_bytes(Path(first['execution']['path']),STDOUT_CAP);rraw=regular_bytes(Path(first['replay']['path']),STDOUT_CAP)
    cases=R.tamper_cases(exraw,rraw,H.value_span)
    prior=load(Path(first['typed_digest']['path']))['typed_result_sha256']
    for case,observed in zip(cases,tamper):
        check_ref(observed['rawref']);raw=regular_bytes(Path(observed['rawref']['path']),STDOUT_CAP)
        require(raw==case['raw'] and observed['name']==case['name'] and observed['target']==case['target'] and observed['expected_diagnostic']==observed['observed_diagnostic']==case['expected_diagnostic'],'Tamper raw/intended-gate evidence')
        try:checker.inspect(raw,REPORT,first['indices'],Path(first['inputs']['path']),Path(first['manifest']['path']),'sha256:'+frozen['tools'][0]['sha256'],original=exraw if case['target']=='replay' else None,prior_typed=prior if case['target']=='replay' else None)
        except R.ReceiptError as exc:require(exc.code==case['expected_diagnostic'],'Retained tamper diagnostic changed')
        else:raise ValueError('Retained receipt tamper accepted')
    generated=summary['generated']
    for key in ('rows','summary','boundary','generated_source'):check_ref(generated[key])
    actual=check_generated_rows(Path(generated['rows']['path']),oracle['cases'])
    require(all(R.exact(actual[k],generated[k]) for k in actual),'Retained generated counts/rows')
    bound=load(Path(generated['boundary']['path']));native_summary=load(Path(generated['summary']['path']))
    require(native_summary.get('boundary')==generated['boundary'] and native_summary.get('rows')==generated['rows'] and
            type(native_summary.get('generated_Report_calls')) is int and native_summary['generated_Report_calls']==202 and
            type(native_summary.get('readback_rows')) is int and native_summary['readback_rows']==202,'Generated summary bridge')
    require(bound.get('source_freeze')==freeze and bound.get('oracle')==summary['oracle'],'Generated current source/oracle boundary')
    cleanup=summary['work_cleanup'];check_ref(cleanup);clean=load(Path(cleanup['path']))
    require(clean.get('removed') is True and clean.get('all_top_level_groups_reaped') is True and
            not Path(clean['owned_path']).exists() and type(clean['owned_device']) is int and type(clean['owned_inode']) is int and
            all(v['state']=='absent' and v['direct_child_reaped'] is True for v in clean['group_observations']), 'Final owned cleanup incomplete')
    require({v['pgid'] for v in clean['group_observations']}==
            {H.load_process(Path(v['path']))['owned_pgid'] for v in summary['stages']},'Cleanup exact process-group inventory')
    lifetime=summary['producer_lifetime'];require(lifetime['retained_binaries'] is False and
            lifetime.get('all_live_refs_matched_after_final_stage') is True,'Producer lifetime observation incomplete')
    work=Path(clean['owned_path'])
    require([v['path'] for v in lifetime['before_cleanup']]==[str(work/'bin'/n) for n in (('oracle','bridge','harness','pair-harness') if include_pair else ('oracle','bridge','harness'))],
            'Exact three producer binary lifetime roster')
    module_names={'oracle':{'go.mod','main.go','prefix_rule_pair.go'},'bridge':{'go.mod','main.go','support.go','result_digest.go','result-profile-83928fa.json','result-profile-go1273.json'},
                  'harness':{'go.mod','main.go','support.go','harness_test.go','generated.go'}}
    if include_pair:module_names['pair-harness']={'go.mod','main.go','support.go','harness_test.go','generated.go'}
    # Include both archival and current profiles in the bridge's exact roster.
    # Bind paths as well as counts so a same-count substituted copy is refused.
    expected_copies={str(work/'public-path-lifecycle.py'),str(work/'telemetry/mode')}
    expected_copies.update(str(work/module/name) for module,names in module_names.items() for name in names)
    require(len(lifetime['owned_modules'])==(4 if include_pair else 3),'Exact owned module roots')
    for module in lifetime['owned_modules']:
        root=Path(module['root']);require(root.parent==work and root.name in module_names and
                set(module['names'])==module_names.pop(root.name),'Owned module source roster binding')
    require(not module_names,'Missing owned module source roster')
    copies=lifetime['owned_module_copies']
    require(len(copies)==len(expected_copies) and {v['copy']['path'] for v in copies}==expected_copies,'Owned-copy exact inventory')
    for item in copies:
        if item['source'] is not None:
            check_ref(item['source'])
            require(item['copy']['bytes']==item['source']['bytes'] and item['copy']['sha256']==item['source']['sha256'],
                    'Byte-exact owned source copy changed')
    require(bound['module_sources']==[next(v['copy'] for v in copies if v['copy']['path']==str(work/'harness'/n))
                for n in ('go.mod','main.go','support.go','harness_test.go','generated.go')] and
            bound['generated_source']==bound['module_sources'][-1] and bound['binary']==lifetime['before_cleanup'][2] and
            bound['go_tool']==frozen['tools'][0] and
            bound['report_receipts']==[b[k] for b in summary['batches'][12:] for k in ('execution','replay')] and
            bound['report_checked']==[b['checked'] for b in summary['batches'][12:]],'Generated module/receipt/binary boundary bridge')
    if include_pair:audit_pair(summary,repository,path.parent,work,frozen,copies)
    for item in lifetime['before_cleanup']+[v['copy'] for v in lifetime['owned_module_copies']]:
        require(work in Path(item['path']).parents and not Path(item['path']).exists(),'Producer/module lifetime boundary')
    require(H.logical(path.parent)<=TOTAL_PROOF_CAP and shutil.disk_usage(path.parent).free>=FREE_MIN,'Final proof/free-space bound')
    return {'schema':'gooo/ipv4-summary-readback/v2' if include_pair else 'gooo/ipv4-summary-readback/v1','status':'CURRENT_SOURCE_AND_EVIDENCE_RECHECKED','summary':ref(path),
            'source_freeze':freeze,'retained_runtime_ref_count':len(summary['retained_runtime_refs']),'top_level_stages':68 if include_pair else 54,**({'prefix_rule_pair':PAIR_COUNTS} if include_pair else {}),
            'manual_execute_outputs':352,'manual_saved_replay_outputs':352,'generated_Report_calls':202,'generated_typed_fields':2222,
            'generated_original_echoes':808,'strict_wire_refusals':42,'receipt_tamper_refusals':18}


def execute_attempt(args):
    attempt=None;summary=None;primary=None;secondary=[]
    previous={}
    def interrupted(signum,frame):raise RuntimeError('Interrupted by signal'+str(signum))
    try:
        for sig in (signal.SIGINT,signal.SIGTERM):previous[sig]=signal.signal(sig,interrupted)
        attempt=Attempt(args);attempt.prepare();summary=attempt.run()
    except BaseException as exc:primary=exc
    if attempt is not None:
        try:attempt.cleanup()
        except BaseException as exc:
            secondary.append({'operation':'owned-work-cleanup','error':repr(exc)})
            if primary is None:primary=exc
    if primary is None:
        try:
            attempt.frozen(live=False);summary['status']='IPV4_PREFIX_RULE_PAIR_CURRENT_NATIVE_REPLAY_GENERATED_PASS' if attempt.include_pair else 'IPV4_CURRENT_NATIVE_REPLAY_GENERATED_PASS'
            if attempt.include_pair:summary['schema']='gooo/ipv4-public-sidecar/v2'
            summary['work_cleanup']=ref(attempt.out/'work-cleanup.json')
            require(time.monotonic()-attempt.started<attempt.overall,'Overall budget expired before completion')
            save_new(attempt.out/'summary.json',summary)
            verify_summary(attempt.out/'summary.json')
            require(time.monotonic()-attempt.started<attempt.overall,'Overall budget expired after final audit')
        except BaseException as exc:primary=exc
    for sig,handler in previous.items():signal.signal(sig,handler)
    if primary is not None:
        if attempt is not None and attempt.owns_out:
            failure={'status':'FAILED_OR_INTERRUPTED','primary_error':repr(primary),'secondary_errors':secondary+attempt.secondary_errors,
                     'stages':attempt.stages,'unpersisted_process':attempt.pending_process,'owned_work_path':str(attempt.work),
                     'owned_work_cleanup_confirmed':attempt.work_cleanup_complete,'last_group_observations':attempt.last_group_observations,'automatic_retries':0}
            try:save_new(attempt.out/'failure.json',failure)
            except BaseException as exc:
                try:sys.stderr.write('Failure ledger write also failed: '+repr(exc)+'\n')
                except BaseException:pass
        raise primary.with_traceback(primary.__traceback__)
    return summary


def main():
    if not __debug__:raise ValueError('Optimized Python is unsupported for verification')
    parser=argparse.ArgumentParser()
    parser.add_argument('--verify-summary',type=Path)
    parser.add_argument('--include-prefix-rule-pair',action='store_true')
    for key in ('repository','out','go','compiler'):parser.add_argument('--'+key,type=Path)
    parser.add_argument('--timeout',type=int,default=180)
    args=parser.parse_args()
    if args.verify_summary is not None:
        require(not args.include_prefix_rule_pair and all(getattr(args,k) is None for k in ('repository','out','go','compiler')),'Readback mode forbids producer arguments')
        result=verify_summary(args.verify_summary)
    else:
        require(all(getattr(args,k) is not None for k in ('repository','out','go','compiler')),'Require repository,out,go,compiler')
        result=execute_attempt(args)
    print(json.dumps(result if args.verify_summary is not None else {'schema':result['schema'],'status':result['status']},separators=(',',':')))


if __name__=='__main__':
    try:main()
    except Exception as exc:sys.stderr.write('IPV4_CHECK_FAILED: '+str(exc)+'\n');sys.exit(1)
