#!/usr/bin/env python3
"""Read an experimental, unsigned native-shard reference index. Stdlib only.

The current Go exporter first inspects original case partitions against its Go
oracles. This reader checks the resulting plan, current sources and every raw
reference again. It neither reruns those oracles nor authenticates a producer,
Git checkout, process exit or GitHub job. It never emits native acceptance or a
legacy full-run timing/footer. Producer manifest strings are data, never paths
used for local reads. Public API: read_reference(root, index_path) returns
consumer data plus a receipt accessor bound to the same frozen stored files.
"""
import hashlib
import importlib.util
import json
import math
import os
import pathlib
import re
import stat
import sys
import zlib

MAX_JSON = 16 << 20
MAX_FILE = 64 << 20
MAX_INT = (1 << 63) - 1
PLAN_FIELDS = ('schema','plan_sha256','contract_sha256','run','profiles','sources','suites','workspaces','shards','cases','batches')
RUN_FIELDS = ('repository','run_id','attempt','tested_commit','tested_tree','compiler_revision','go_version','workers')
SUITE_FIELDS = ('index','package','activity','cases','batches','source_paths')
WORKSPACE_FIELDS = ('original_index','manifest_sha256','source_set_sha256','entry_producer','sources')
BATCH_FIELDS = ('index','start','end','namespace','cases_sha256','manifest_sha256','source_set_sha256','entry_producer','generated_sha256','execution_sha256','replay_sha256','replayed_from')
PARTIAL_FIELDS = ('schema','plan_sha256','run','index','namespace','sources','joined','state','suites')
RECORD_FIELDS = ('original_index','package','activity','state','error','batches','exhaustive')
REFERENCE_SUITE_FIELDS = ('original_index','shard_index','package','activity','batches','exhaustive')
REFERENCE_BATCH_FIELDS = ('batch','workspace','cases','execution','replay','sources','producer_manifest_path','go_version','go_binary')
READER_SOURCES = (
    'cmd/verify/ci_shard_contract.go','cmd/verify/ci_shard_cli.go','cmd/verify/ci_shard_contract_test.go',
    'cmd/verify/ci_shard_receipt_storage.go','cmd/verify/ci_shard_receipt_storage_test.go',
    'cmd/verify/ci_shard_native_types.go','cmd/verify/ci_shard_execute.go','cmd/verify/ci_shard_execute_test.go',
    'cmd/verify/ci_shard_native_evidence.go','cmd/verify/ci_shard_native_evidence_test.go',
    'cmd/verify/ci_shard_native_cli.go','cmd/verify/ci_shard_native_cli_test.go',
    'cmd/verify/ci_shard_native_smoke_test.go',
    'cmd/verify/ci_shard_native_reference.go','cmd/verify/ci_shard_native_reference_test.go',
    'cmd/verify/ci_shard_command_observations.go','cmd/verify/ci_shard_command_observations_test.go',
    'tools/native-shard-reference.py','tools/native-shard-reference_test.py')
ADDITION_PACKAGE = 'app/diagnostic-header-addition'
ADDITION_ACTIVITY = 'AssessAddition'


def need(condition, message):
    if not condition:
        raise RuntimeError(message)


def shape(value, fields, message='object missing or unknown fields'):
    need(type(value) is dict and set(value) == set(fields), message)
    return {key:value[key] for key in fields}


def integer(value, minimum=0, maximum=MAX_INT):
    need(type(value) is int and minimum <= value <= maximum, 'bounded exact integer required')
    return value


def text(value, nonempty=True):
    need(type(value) is str and (not nonempty or bool(value)) and '\x00' not in value, 'valid string required')
    value.encode('utf-8', errors='strict')
    return value


def equal(a, b):
    if type(a) is not type(b):
        return False
    if type(a) is dict:
        return a.keys() == b.keys() and all(equal(a[key], b[key]) for key in a)
    if type(a) is list:
        return len(a) == len(b) and all(equal(x,y) for x,y in zip(a,b))
    return a == b


def digest(raw):
    return 'sha256:' + hashlib.sha256(raw).hexdigest()


def valid_digest(value):
    need(type(value) is str and re.fullmatch(r'sha256:[0-9a-f]{64}', value) is not None, 'prefixed SHA256 required')
    return value


def go_json(value, *, html=False, sorted_map=False):
    raw=json.dumps(value, ensure_ascii=False, separators=(',',':'), sort_keys=sorted_map)
    if html:
        raw=raw.replace('<','\\u003c').replace('>','\\u003e').replace('&','\\u0026')
    return raw.replace('\u2028','\\u2028').replace('\u2029','\\u2029').encode('utf-8')


def load(raw, *, metadata=True):
    # The stdlib scanner already supplies decimal integer tokens. Keep the
    # extra -0/ASCII and signed64 restrictions without repeated regex/type
    # validation; malformed token boundaries still fail in the scanner.
    def pairs(items):
        result={}
        for key,value in items:
            if key in result:
                raise RuntimeError('duplicate decoded JSON key: '+key)
            result[key]=value
        return result
    def number(value):
        if value == '-0' or not value.isascii():
            raise RuntimeError('canonical decimal integer token required')
        parsed=int(value)
        if parsed < minimum or parsed > MAX_INT:
            raise RuntimeError('bounded exact integer required')
        return parsed
    def finite(value):
        parsed=float(value);need(math.isfinite(parsed),'nonfinite raw JSON number')
        return parsed
    def reject(value):
        raise RuntimeError('nondecimal or nonfinite JSON number: '+value)
    need(type(raw) is bytes and 0 < len(raw) <= (MAX_JSON if metadata else MAX_FILE), 'bounded nonempty JSON required')
    try:
        minimum = -MAX_INT - 1
        return json.loads(raw.decode('utf-8', errors='strict'),object_pairs_hook=pairs,
                          parse_int=number,parse_float=reject if metadata else finite,parse_constant=reject)
    except (ValueError,UnicodeError,RecursionError) as error:
        raise RuntimeError('invalid strict JSON: '+type(error).__name__) from error

def unpack_receipt(raw, limit=MAX_FILE):
    need(type(raw) is bytes and 0 < len(raw) <= 8 << 20 and 0 < limit <= MAX_FILE, 'bounded stored receipt required')
    try:
        stream=zlib.decompressobj(16+zlib.MAX_WBITS)
        decoded=stream.decompress(raw,limit+1)
    except zlib.error as error:
        raise RuntimeError('invalid gzip receipt') from error
    need(0 < len(decoded) <= limit and stream.eof and not stream.unused_data and not stream.unconsumed_tail,
         'oversized, truncated, trailing or multiple-member gzip receipt')
    return decoded


def relative(value):
    text(value)
    need(not value.startswith('/') and '\\' not in value and ':' not in value and
         all(0x20 <= ord(c) < 0x7f for c in value) and
         all(part not in ('','.','..') for part in value.split('/')), 'canonical relative path required')
    return value


def absolute(value):
    if isinstance(value,pathlib.Path):
        value=str(value)
    text(value)
    path=pathlib.Path(value)
    need(path.is_absolute() and str(path) == value and path.resolve() == path,
         'canonical absolute path required: '+value)
    return path


def namespace(shard):
    return 'native-ci-shard-%03d' % shard


def batch_namespace(shard, original, batch):
    return namespace(shard)+'/suite-%04d/batch-%06d' % (original,batch)


class Reader:
    """Bounded regular files with before/after identity, aliases and final recheck."""
    def __init__(self,progress=None):
        self.pins={};self.files={};self.directories={};self.inodes={};self.progress=progress

    @staticmethod
    def identity(info):
        return (info.st_dev,info.st_ino,info.st_mode,info.st_size,info.st_mtime_ns)

    def directory(self,path):
        path=absolute(path)
        chain=[*reversed(path.parents),path]
        for item in chain:
            info=item.lstat()
            need(stat.S_ISDIR(info.st_mode) and not stat.S_ISLNK(info.st_mode), 'regular directory required: '+str(item))
            identity=(info.st_dev,info.st_ino,info.st_mode)
            need(str(item) not in self.directories or self.directories[str(item)] == identity, 'directory changed during inspection: '+str(item))
            self.directories[str(item)]=identity
        return path

    def read(self,path,limit=MAX_FILE):
        path=absolute(path);self.directory(path.parent);before=path.lstat()
        need(stat.S_ISREG(before.st_mode) and 0 < before.st_size <= limit, 'bounded regular input required: '+str(path))
        identity=self.identity(before);name=str(path)
        need(name not in self.files or self.files[name] == identity, 'file identity changed during inspection: '+name)
        inode=(before.st_dev,before.st_ino)
        need(inode not in self.inodes or self.inodes[inode] == name, 'hardlink path alias refused: '+name)
        fd=os.open(path,os.O_RDONLY|getattr(os,'O_NOFOLLOW',0))
        with os.fdopen(fd,'rb') as source:
            need(self.identity(os.fstat(source.fileno())) == identity, 'file changed before open: '+name)
            raw=source.read(limit+1)
            need(self.identity(os.fstat(source.fileno())) == identity, 'file changed during read: '+name)
        need(self.identity(path.lstat()) == identity and len(raw) == before.st_size <= limit, 'file changed after read: '+name)
        self.directory(path.parent)
        ref={'bytes':len(raw),'sha256':hashlib.sha256(raw).hexdigest()}
        need(name not in self.pins or equal(self.pins[name],ref), 'frozen bytes changed: '+name)
        self.files[name]=identity;self.inodes[inode]=name;self.pins[name]=ref
        if self.progress is not None:self.progress.diagnostic('record_read',len(raw))
        return raw

    def file(self,root,ref,expected=None,limit=MAX_FILE):
        compressed=type(ref) is dict and 'encoding' in ref
        fields=('path','bytes','sha256','encoding','stored_bytes','stored_sha256') if compressed else ('path','bytes','sha256')
        shape(ref,fields,'exact raw/stored file reference required')
        name=relative(ref['path']);integer(ref['bytes'],1,limit);valid_digest(ref['sha256'])
        logical=name[:-3] if compressed else name
        if compressed:
            need(ref['encoding'] == 'gzip' and name.endswith('.json.gz') and pathlib.PurePosixPath(logical).name in ('execution.json','replay.json'),
                 'gzip is only supported for explicitly referenced native receipts')
            integer(ref['stored_bytes'],1,8 << 20);valid_digest(ref['stored_sha256'])
        need(expected is None or logical == expected, 'raw file reference path differs from canonical namespace')
        root=self.directory(root);path=root/name
        counterpart=root/(logical if compressed else logical+'.gz')
        def unambiguous():
            if pathlib.PurePosixPath(logical).name not in ('execution.json','replay.json'):
                return
            try:
                counterpart.lstat()
            except FileNotFoundError:
                return
            raise RuntimeError('ambiguous plain and gzip receipt')
        unambiguous()
        stored=self.read(path,(8 << 20) if compressed else limit)
        if compressed:
            need(len(stored) == ref['stored_bytes'] and digest(stored) == ref['stored_sha256'], 'stored receipt bytes/hash differ')
            raw=unpack_receipt(stored,limit)
        else:
            raw=stored
        unambiguous()
        need(len(raw) == ref['bytes'] and digest(raw) == ref['sha256'], 'raw reference bytes/hash differ: '+name)
        if self.progress is not None:self.progress.diagnostic('record_logical',len(raw))
        return path,raw

    def finish(self):
        for name in list(self.files):
            self.read(pathlib.Path(name))
        for name,identity in self.directories.items():
            info=pathlib.Path(name).lstat()
            need((info.st_dev,info.st_ino,info.st_mode) == identity and stat.S_ISDIR(info.st_mode), 'directory changed before finish: '+name)


class ConsumerReceiptReader:
    """Reopen only the addition receipts already checked by the same reader."""
    def __init__(self,reader,references):
        self.reader=reader
        self.references={name:(root,dict(ref),logical) for name,(root,ref,logical) in references.items()}

    def selected(self,path):
        name=str(absolute(path))
        need(name in self.references,'unverified consumer receipt path: '+name)
        return self.references[name]

    def read(self,path):
        root,ref,logical=self.selected(path)
        _,raw=self.reader.file(root,ref,logical)
        return raw

    def storage(self,path):
        root,ref,_=self.selected(path)
        return {'path':str(root/ref['path']),'encoding':ref.get('encoding','identity'),
                'bytes':ref.get('stored_bytes',ref['bytes']),
                'sha256':ref.get('stored_sha256',ref['sha256'])}

    def finish(self):
        # Recheck representation absence too: Reader.finish only pins files
        # that existed, so it cannot detect a newly added plain counterpart.
        for name in self.references:
            self.read(pathlib.Path(name))
        self.reader.finish()


def load_host(root,reader):
    # A fixed sibling module is the only dynamically loaded API. Index contents
    # can never choose module names, Python paths, commands or executable input.
    path=root/'tools/check-header-addition-host.py';raw=reader.read(path)
    spec=importlib.util.spec_from_file_location('gooo_shard_reference_host',path)
    need(spec is not None and spec.loader is not None, 'fixed sibling host module required')
    module=importlib.util.module_from_spec(spec);sys.modules[spec.name]=module
    exec(compile(raw,str(path),'exec'),module.__dict__)
    return module


def ordered_sources(sources):
    need(type(sources) is dict and sources, 'source map required')
    result={}
    for name in sorted(sources):
        relative(name);row=shape(sources[name],('mode','bytes','sha256'))
        need(row['mode'] in ('100644','100755'), 'regular source mode required')
        integer(row['bytes'],1,MAX_FILE);valid_digest(row['sha256']);result[name]=row
    return result


def ordered_run(run):
    run=shape(run,RUN_FIELDS)
    need(type(run['repository']) is str and re.fullmatch(r'[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+',run['repository']) is not None and '..' not in run['repository'], 'run repository shape')
    need(type(run['run_id']) is str and re.fullmatch(r'[1-9][0-9]{0,19}',run['run_id']) is not None, 'run ID shape')
    integer(run['attempt'],1,1000000)
    for key in ('tested_commit','tested_tree','compiler_revision'):
        need(type(run[key]) is str and re.fullmatch(r'[0-9a-f]{40}',run[key]) is not None, 'run revision shape')
    need(run['go_version'] == 'go1.27.3' and equal(run['workers'],3), 'pinned Go and three-worker run')
    return run


def assignments(suites,count):
    integer(count,1,min(16,len(suites)))
    result=[{'index':i,'namespace':namespace(i),'original_indices':[]} for i in range(count)];loads=[0]*count
    for row in sorted(suites,key=lambda row:(-row['batches'],-row['cases'],row['index'])):
        target=min(range(count),key=lambda i:(loads[i],i))
        result[target]['original_indices'].append(row['index']);loads[target]+=row['batches']
    for row in result:
        row['original_indices'].sort()
    return result


def entry_contract(suite,owners,source_bytes):
    # Pinned compiler839 resolves unqualified names from the local package first,
    # then from entity exports of spec.Imports. Omitted manifest imports are the
    # union of imports in every selected source of that package, not just the
    # source containing the entry. Imported entities are not transitively reexported.
    packages={}
    for name in suite['source_paths']:
        packages.setdefault(owners[name],[]).append((name,source_bytes[name].decode('utf-8',errors='strict')))
    need(suite['package'] in packages,'entry package must be selected')
    texts=packages[suite['package']]
    imports=set()
    for _,raw in texts:
        for imported in re.findall(r'(?m)^[ \t]*import[ \t]+(?:[A-Za-z_]\w*[ \t]+)?"([^"\n]+)"',raw):
            relative(imported)
            need(imported in packages,'explicit imported package must be selected: '+imported)
            imports.add(imported)
    def declared_entities(package):
        entities={}
        for _,raw in packages[package]:
            for entity,ident in re.findall(r'(?m)^entity\s+(\w+)\s+id\s+"([^"\n]*)"',raw):
                need(ident,'source entity requires stable ID: '+entity)
                need(entity not in entities or entities[entity] == ident,
                     'conflicting entity IDs in selected package: '+entity)
                entities[entity]=ident
        return entities
    local=declared_entities(suite['package'])
    dependencies=[declared_entities(package) for package in sorted(imports)]
    def resolve(name):
        if name in local:
            return local[name]
        identities={entities[name] for entities in dependencies if name in entities}
        need(identities,'source entry entities must be explicit in selected package or declared imports: '+name)
        need(len(identities) == 1,'ambiguous imported entity type: '+name)
        return next(iter(identities))
    selected=[]
    for name,raw in texts:
        for match in re.finditer(r'(?m)^activity\s+'+re.escape(suite['activity'])+r'\s*\(([^)]*)\)\s*->\s*(\w+)\s+id\s+"([^"\n]+)"',raw):
            types=[v.strip() for v in match[1].split(',')]
            need(types and all(types),'source entry input types required')
            selected.append({'source':name,'id':match[3],'inputs':[resolve(t) for t in types],'output':resolve(match[2])})
    need(len(selected) == 1, 'one current source-declared entry required')
    return selected[0]


def validate_source_roster(root,reader,expected,sources):
    sources=ordered_sources(sources)
    modes={row['path']:row['mode'] for row in expected.document['sources']}
    need(not set(modes).intersection(READER_SOURCES),'legacy and experimental reader rosters must not overlap')
    modes.update({name:'100644' for name in READER_SOURCES})
    need(set(sources) == set(modes) and len(modes) == expected.source_count+len(READER_SOURCES),
         'exact reviewed sources plus explicit reader sources')
    source_bytes={}
    for name,mode in modes.items():
        raw=reader.read(root/name);observed=sources[name]
        need(observed['mode'] == mode and bool((root/name).lstat().st_mode&stat.S_IXUSR) == (mode == '100755'),'current source mode differs: '+name)
        need(observed['bytes'] == len(raw) and observed['sha256'] == digest(raw),'current source bytes differ: '+name)
        source_bytes[name]=raw
    declaration=re.search(r'var ciShardReaderSources = \[\]string\{(.*?)\n\}',source_bytes['cmd/verify/ci_shard_contract.go'].decode(),re.S)
    need(declaration is not None and tuple(re.findall(r'"([^"\n]+)"',declaration[1])) == READER_SOURCES,'Go/Python explicit experimental reader roster differs')
    return sources,source_bytes


def validate_plan(root,reader,H,plan):
    p=shape(plan,PLAN_FIELDS);need(p['schema'] == 'gooo/native-ci-shard-plan/v1','current shard plan schema')
    valid_digest(p['plan_sha256']);valid_digest(p['contract_sha256']);p['run']=ordered_run(p['run'])
    expected=H.reviewed_contract(root,reader.read)
    need((expected.suite_count,expected.total_cases,expected.total_batches,expected.source_count) == (82,175649,2305,293), 'current reviewed full population required')
    need(equal(p['profiles'],expected.document['profiles']) and equal(p['suites'],expected.document['suites']) and equal(p['cases'],expected.total_cases) and equal(p['batches'],expected.total_batches),'exact reviewed profiles, original suites and totals')
    p['profiles']=shape(p['profiles'],('case','workspace'))
    p['suites']=[shape(row,SUITE_FIELDS) for row in p['suites']]
    ledger=reader.read(root/H.CONTRACT.REVIEWED_PATH,1<<20)
    need(digest(ledger) == p['contract_sha256'],'current reviewed contract raw hash')
    p['sources'],source_bytes=validate_source_roster(root,reader,expected,p['sources'])
    lock=load(source_bytes['upstream.json'])
    need(lock['compiler_revision'] == H.REV == p['run']['compiler_revision'] and lock['go_version'] == p['run']['go_version'],'current upstream producer pins')
    catalog=load(source_bytes['verification-catalog.json']);H.source_roster(root,catalog,expected)
    owners={name:pkg['path'] for pkg in catalog['packages'] for name in pkg['sources']}
    for name,owner in H.APPENDED_SOURCE_OWNERS.items():need(owners.get(name) == owner,'independently stated appended source owner: '+name)
    need(type(p['workspaces']) is list and len(p['workspaces']) == len(p['suites']),'complete current workspace anchors')
    workspaces=[];entries=[]
    for i,suite in enumerate(p['suites']):
        w=shape(p['workspaces'][i],WORKSPACE_FIELDS)
        need(equal(w['original_index'],i) and type(w['sources']) is list and len(w['sources']) == len(suite['source_paths']),'workspace original index/source count')
        anchors=[];packages={}
        for name,saved in zip(suite['source_paths'],w['sources']):
            a=shape(saved,('name','raw_sha256','identity'));identity=shape(a['identity'],('Package','Digest'))
            wanted={'Package':owners[name],'Digest':digest(go_json(source_bytes[name].decode('utf-8'),html=True))}
            need(a['name'] == name and a['raw_sha256'] == p['sources'][name]['sha256'] and equal(identity,wanted),'workspace current source identity')
            a['identity']=identity;anchors.append(a);packages.setdefault(owners[name],[]).append(name)
        entry=entry_contract(suite,owners,source_bytes);entries.append(entry)
        manifest={'schema':'gooo/package-workspace-manifest/v1','entry':{'package_path':suite['package'],'activity':suite['activity']},'packages':[{'path':pkg,'sources':names} for pkg,names in packages.items()]}
        # Go MarshalIndent has two-space indent, HTML escaping and one final LF.
        manifest_raw=json.dumps(manifest,ensure_ascii=False,indent=2).replace('<','\\u003c').replace('>','\\u003e').replace('&','\\u0026').replace('\u2028','\\u2028').replace('\u2029','\\u2029').encode()+b'\n'
        need(w['manifest_sha256'] == digest(manifest_raw) and w['source_set_sha256'] == digest(go_json(anchors)) and w['entry_producer'] == entry['id'],'current manifest/source-anchor/entry binding')
        w['sources']=anchors;workspaces.append(w)
    p['workspaces']=workspaces
    need(type(p['shards']) is list and equal(p['shards'],assignments(p['suites'],len(p['shards']))),'exact deterministic full suite partition')
    p['shards']=[shape(row,('index','namespace','original_indices')) for row in p['shards']]
    before=p['plan_sha256'];p['plan_sha256']=''
    need(digest(go_json(p)) == before,'Go typed-order plan canonical hash')
    p['plan_sha256']=before
    need(equal(p['suites'][73],{'index':73,'package':ADDITION_PACKAGE,'activity':ADDITION_ACTIVITY,'cases':50,'batches':7,'source_paths':p['suites'][73]['source_paths']}),'original addition73 obligation')
    return p,owners,source_bytes,entries,lock


def binary_ref(ref):
    value=shape(ref,('bytes','sha256'));integer(value['bytes'],1,MAX_INT)
    need(type(value['sha256']) is str and re.fullmatch(r'[0-9a-f]{64}',value['sha256']) is not None,'plain tool SHA256 required')
    return value


def validate_tools(doc,plan,shard,compiler_raw):
    shape(doc,('schema','plan_sha256','run','index','goos','goarch','go_version','before','after','joined','cache_close_succeeded','scope'))
    need(doc['schema'] == 'gooo/native-shard-tool-observations/v1' and doc['plan_sha256'] == plan['plan_sha256'] and equal(doc['run'],plan['run']) and equal(doc['index'],shard),'tool plan/run/shard binding')
    need(doc['goos'] in ('linux','darwin') and doc['goarch'] in ('amd64','arm64') and doc['go_version'] == plan['run']['go_version'] and doc['joined'] is True and doc['cache_close_succeeded'] is True,'actual joined tool platform/cache observations')
    text(doc['scope']);shape(doc['before'],('compiler','go','verifier'));shape(doc['after'],('compiler','go','verifier'))
    for ref in [*doc['before'].values(),*doc['after'].values()]:
        binary_ref(ref)
    need(equal(doc['before'],doc['after']),'actual tool bytes changed before/after')
    need(equal(doc['before']['compiler'],{'bytes':len(compiler_raw),'sha256':hashlib.sha256(compiler_raw).hexdigest()}),'current compiler bytes differ from actual observation')
    return doc


def validate_partial(doc,plan,shard):
    shape(doc,PARTIAL_FIELDS)
    assignment=plan['shards'][shard]
    need(doc['schema'] == 'gooo/native-ci-shard-partial/v1' and doc['plan_sha256'] == plan['plan_sha256'] and equal(doc['run'],plan['run']) and equal(doc['index'],shard) and doc['namespace'] == assignment['namespace'] and equal(doc['sources'],plan['sources']),'partial plan/run/source/namespace binding')
    need(doc['joined'] is True and doc['state'] == 'SUCCEEDED' and type(doc['suites']) is list and len(doc['suites']) == len(assignment['original_indices']),'all native shard work joined and succeeded')
    for original,record in zip(assignment['original_indices'],doc['suites']):
        shape(record,RECORD_FIELDS);suite=plan['suites'][original];workspace=plan['workspaces'][original]
        need(equal(record['original_index'],original) and record['package'] == suite['package'] and record['activity'] == suite['activity'] and record['state'] == 'VERIFIED' and record['error'] == '','exact successful original suite record')
        need(type(record['batches']) is list and len(record['batches']) == suite['batches'],'complete original batch count')
        cursor=0
        for index,batch in enumerate(record['batches']):
            shape(batch,BATCH_FIELDS);integer(batch['end'],cursor+1,suite['cases'])
            need(equal(batch['index'],index) and equal(batch['start'],cursor) and batch['namespace'] == batch_namespace(shard,original,index),'contiguous original case union and canonical batch namespace')
            for key in ('cases_sha256','manifest_sha256','source_set_sha256','generated_sha256','execution_sha256','replay_sha256','replayed_from'):valid_digest(batch[key])
            need(batch['manifest_sha256'] == workspace['manifest_sha256'] and batch['source_set_sha256'] == workspace['source_set_sha256'] and batch['entry_producer'] == workspace['entry_producer'] and batch['replayed_from'] == batch['execution_sha256'],'batch workspace/source/entry/raw-parent bindings')
            cursor=batch['end']
        need(cursor == suite['cases'],'original case union must be complete')
        need(type(record['exhaustive']) is list,'exhaustive report array required')
    return doc


def validate_commands(doc,plan,partial,tools,H):
    shape(doc,('schema','plan_sha256','run','index','namespace','sources','goos','command_resources_schema','joined','complete','state','failure','native_acceptance','producer_authenticated','suites','scope'))
    need(doc['schema'] == 'gooo/native-shard-command-observations/v1' and doc['plan_sha256'] == plan['plan_sha256'] and equal(doc['run'],plan['run']) and equal(doc['index'],partial['index']) and doc['namespace'] == partial['namespace'] and equal(doc['sources'],plan['sources']),'command observation plan/run/source/namespace binding')
    need(doc['native_acceptance'] is False and doc['producer_authenticated'] is False and doc['joined'] is True and doc['complete'] is True and doc['state'] == 'WAIT_ACCOUNTING_COMPLETE_NATIVE_UNACCEPTED' and doc['failure'] == '','complete unauthenticated waited-command observation')
    need(doc['goos'] == tools['goos'] and doc['command_resources_schema'] == 'gooo/native-command-resources/v1','command/tool platform and accounting schema');text(doc['scope'])
    need(type(doc['suites']) is list and len(doc['suites']) == len(partial['suites']),'all command observation suites')
    resources=[]
    for observed,record in zip(doc['suites'],partial['suites']):
        shape(observed,('original_index','state','batches'))
        need(equal(observed['original_index'],record['original_index']) and observed['state'] == 'VERIFIED' and type(observed['batches']) is list and len(observed['batches']) == len(record['batches']),'exact command suite/batch coverage')
        for value,batch in zip(observed['batches'],record['batches']):
            shape(value,('index','namespace','verified_prefix','command_resources'))
            need(equal(value['index'],batch['index']) and value['namespace'] == batch['namespace'] and value['verified_prefix'] is True,'verified execute/replay wait prefix')
            resources.append({'command_resources':value['command_resources']})
            for metric in value['command_resources'].values():
                if metric.get('status') == 'NOT_AVAILABLE':need(len(metric.get('reason','').encode('utf-8')) <= 4096,'bounded unavailable accounting reason')
    H.validate_command_resources({'goos':doc['goos'],'command_resources_schema':doc['command_resources_schema']},resources)


def validate_compiler(doc,lock):
    need(type(doc) is dict,'compiler build identity object required')
    for key,value in {'schema':'gooo/build-identity/v1','version':lock['compiler_version'],'compiler_source_sha':lock['compiler_revision'],'vcs_revision':lock['compiler_revision'],'source_status':'CLEAN_VCS','vcs_modified':'false','go_version':lock['go_version'],'native_go_required':lock['go_version']}.items():
        need(equal(doc.get(key),value),'actual compiler build identity differs: '+key)


def inspect_receipt(raw,cases,metadata,workspace,producer_path,go_version,go_binary,entry,suite,source_map,replay_parent=None):
    doc=load(raw,metadata=False)
    need(doc.get('schema') == 'gooo/workspace-body-execution-receipt/v1' and doc.get('decision') == 'PASS' and not doc.get('error'),'raw native PASS envelope')
    need(doc.get('manifest') == producer_path and doc.get('manifest_digest') == metadata['manifest_sha256'] and doc.get('cases_digest') == metadata['cases_sha256'] and 'inputs_digest' not in doc,'raw native manifest/cases binding')
    result=doc['result'];program=result['program'];runtime=result['runtime'];composition=result['composition']
    need(program['entry']['package_path'] == suite['package'] and program['entry']['activity'] == suite['activity'],'raw native original entry')
    seen={};selected=[];packages=set()
    for package in program['workspace_image']['packages']:
        need(package['path'] not in packages and package['sources'],'nonempty unique raw workspace packages')
        packages.add(package['path'])
        for source in package['sources']:
            need(source['filename'] not in seen,'duplicate raw workspace image source')
            seen[source['filename']]={'Package':package['path'],'Digest':source['source_digest']}
        if package['path'] == suite['package']:
            selected.extend(export for export in package['exports'] if export['name'] == suite['activity'] or export['id'] == entry['id'])
    need(packages == {v['Package'] for v in source_map.values()} and equal(seen,source_map) and len(selected) == 1 and selected[0]['name'] == suite['activity'] and selected[0]['id'] == entry['id'] and selected[0]['kind'] == 'activity' and equal(selected[0]['input_types'],entry['inputs']),'raw workspace image/entry typed source identity')
    need(runtime['schema'] == 'gooo/body-composition-runtime/v1' and runtime['stage'] == 'COMPLETE' and not runtime.get('failure') and runtime['producer_source_sha'] == workspace['compiler_revision'] and runtime['go_version'] == go_version and runtime['go_tool_sha256'] == 'sha256:'+go_binary['sha256'],'raw runtime compiler/Go binary identity')
    need(runtime['go_tool_selection'] in ('explicit','path','compiler_goroot','toolchain_cache','path_fallback'),'raw Go tool selection')
    need(equal(runtime['model_calls'],0) and equal(runtime['finite_total'],len(cases)) and equal(runtime['finite_passed'],len(cases)) and runtime['projection_replayed'] is True and runtime['runtime_replayed'] is True,'raw finite complete zero-model execution')
    need(type(composition['source']) is str and composition['source'] and digest(composition['source'].encode()) == metadata['generated_sha256'] == composition['generated_sha256'] == runtime['generated_sha256'],'raw actual generated source hash')
    need(len(composition['steps']) == 1 and composition['steps'][0]['generation']['report']['activity_id'] == entry['id'],'raw source-declared generation entry')
    traces=runtime['traces'];need(type(traces) is list and len(traces) == len(cases),'raw finite case trace count')
    key=suite['package']+':'+suite['activity']
    for index,(row,trace) in enumerate(zip(cases,traces)):
        shape(row,('inputs','expected'));need(type(row['inputs']) is dict and set(row['expected']) == {key},'original raw case input/oracle shape')
        need(equal(trace['case_index'],index) and len(trace['deliveries']) == 1,'raw trace original index/delivery count')
        delivery=trace['deliveries'][0];need(delivery['activity_id'] == entry['id'] and delivery['passed'] is True and equal(delivery['actual'],row['expected'][key]) and equal(delivery['expected'],row['expected'][key]),'raw actual/expected must equal preserved Go case values')
        if len(entry['inputs']) == 1:
            need(set(row['inputs']) == {key} and not delivery.get('inputs') and equal(delivery['input'],row['inputs'][key]),'raw original single input')
        else:
            need(set(row['inputs']) == {key+'.input'+str(i) for i in range(len(entry['inputs']))} and not delivery.get('input') and len(delivery['inputs']) == len(entry['inputs']),'raw original multi-input arity')
            for i,value in enumerate(delivery['inputs']):
                need(value['port'] == 'input'+str(i) and value['entity_id'] == entry['inputs'][i] and equal(value['value'],row['inputs'][key+'.input'+str(i)]),'raw ordered original input/entity')
    if replay_parent is None:
        need(not doc.get('replayed_from_sha256') and not result.get('replay'),'original cannot claim saved replay')
    else:
        need(doc.get('replayed_from_sha256') == digest(replay_parent) == metadata['replayed_from'] and result['replay']['schema'] == 'gooo/workspace-body-replay/v1' and equal(result['replay']['model_calls'],0),'exact raw saved replay parent')
    return composition['source']


def exhaustive_kind(suite):
    key=suite['package']+':'+suite['activity']
    if key == 'go/unicode:SimpleFold':return 'simple-fold'
    if key in ('go/image/color:RGBToYCbCr','go/image/color:YCbCrToRGB'):return 'ycbcr/'+suite['activity']
    if suite['package'] == 'go/unicode' and suite['activity'] in ('To','ToUpper','ToLower','ToTitle'):return 'case-mapping/'+suite['activity']
    return ''


def exhaustive_expected(kind,generated):
    outside=[-2147483648,-2147483647,-65536,-65461,-1,1114112,1114187,2147483646,2147483647]
    extension=[-9223372036854775808,-9007199254740993,-4294967371,-4294967221,-2147483649,2147483648,4294967296,4294967371,9007199254740993,9223372036854775806,9223372036854775807]
    if kind == 'simple-fold':
        return {'schema':'gooo/simple-fold-exhaustive-native/v1','go_version':'go1.27.3','unicode_version':'17.0.0','generated_sha256':generated,'unicode_range_inputs':1114112,'unicode_range_comparisons':1114112,'outside_unicode_int32_inputs':outside,'outside_unicode_int32_comparisons':len(outside),'outside_int32_extension_inputs':extension,'outside_int32_extension_comparisons':len(extension),'mismatches':0}
    operation=kind.split('/')[1]
    if kind.startswith('ycbcr/'):
        entity,oracle=('ycbcr','sha256:c7a38f2fd3b7dccee233499f58c95ca122dd4b833fe4e2908f4681386e643098') if operation == 'RGBToYCbCr' else ('rgb','sha256:ad0d598e5b003c424acbc9e5d8a3cc8fe965d7f22ecc3903c425a929ba742682')
        return {'schema':'gooo/ycbcr-exhaustive-native/v1','go_version':'go1.27.3','operation':operation,'activity_id':'gooo://go/image/color/'+operation,'output_entity_id':'gooo://go/image/color/'+entity,'generated_sha256':generated,'input_min':0,'input_max':255,'input_arity':3,'input_order':'input0-major,input1-middle,input2-minor','input_count':16777216,'comparison_count':16777216,'component_comparison_count':50331648,'output_sha256':oracle,'oracle_sha256':oracle,'mismatches':0}
    cases={'To':[0,1,2],'ToUpper':[0],'ToLower':[1],'ToTitle':[2]}[operation]
    invalid=[-9223372036854775808,-9007199254740993,-4294967296,-2147483649,-1,3,4,2147483648,4294967296,9007199254740993,9223372036854775807] if operation == 'To' else []
    runes=[-2147483648,-1,0,65,97,453,55296,1114111,1114112,2147483647] if operation == 'To' else []
    return {'schema':'gooo/case-mapping-exhaustive-native/v1','go_version':'go1.27.3','unicode_version':'17.0.0','operation':operation,'activity_id':'gooo://go/unicode/'+operation,'input_entity_ids':['gooo://std/integer']*(2 if operation == 'To' else 1),'output_entity_id':'gooo://std/integer','generated_sha256':generated,'valid_cases':cases,'unicode_range_inputs':1114112,'unicode_range_comparisons':1114112*len(cases),'outside_unicode_int32_inputs':outside,'outside_unicode_int32_comparisons':len(outside)*len(cases),'outside_int32_extension_inputs':extension,'outside_int32_extension_comparisons':len(extension)*len(cases),'invalid_cases':invalid,'invalid_case_original_rune_inputs':runes,'invalid_case_original_rune_comparisons':len(invalid)*len(runes),'invalid_case_extension_comparisons':len(invalid)*len(extension),'mismatches':0}


def inspect_exhaustive(reader,evidence_root,suite,shard,saved,references,generated):
    kind=exhaustive_kind(suite)
    need(type(references) is list and len(references) == len(saved) == (1 if kind else 0),'exact original exhaustive report roster')
    if not kind:return
    record=shape(saved[0],('kind','report','sha256'));ref=shape(references[0],('kind','report_sha256','files'))
    need(record['kind'] == ref['kind'] == kind,'original exhaustive kind')
    expected=exhaustive_expected(kind,generated)
    need(equal(record['report'],expected) and record['sha256'] == ref['report_sha256'] == digest(go_json(record['report'],sorted_map=True)),'exact exhaustive report contract/canonical decoded hash')
    directory='simple-fold-exhaustive' if kind == 'simple-fold' else kind.replace('/','-exhaustive-')
    prefix=namespace(shard)+'/suite-%04d/'%suite['index']+directory+'/'
    names=('native/generated.go','native/exhaustive_test.go','native/go.mod','native-test.txt','exhaustive.json')
    need(type(ref['files']) is list and len(ref['files']) == len(names),'five actual exhaustive artifacts')
    raw={}
    for name,file in zip(names,ref['files']):
        _,raw[name]=reader.file(evidence_root,file,prefix+name,1<<20 if name in ('native-test.txt','exhaustive.json') else MAX_FILE)
    need(digest(raw['native/generated.go']) == generated,'exhaustive actual generated source bytes')
    module,test=('gooo-simple-fold-exhaustive','TestSimpleFoldExhaustive') if kind == 'simple-fold' else (('gooo-ycbcr-exhaustive','TestYCbCrExhaustive') if kind.startswith('ycbcr/') else ('gooo-case-mapping-exhaustive','TestCaseMappingExhaustive'))
    need(raw['native/go.mod'] == ('module '+module+'\n\ngo 1.27.3\n').encode(),'exhaustive pinned module bytes')
    need(('func '+test+'(').encode() in raw['native/exhaustive_test.go'] and generated.encode() in raw['native/exhaustive_test.go'],'exhaustive harness source/generated binding')
    log=raw['native-test.txt'];need(('--- PASS: '+test+' ').encode() in log and b'\nPASS\n' in log and b'--- FAIL:' not in log and b'panic:' not in log,'exhaustive actual diagnostic')
    need(equal(load(raw['exhaustive.json']),expected),'actual exhaustive JSON must equal preserved partial report')


def read_reference(root,index_path,progress=None):
    """Return consumer data and a bounded receipt accessor after full checks."""
    root,index_path=map(absolute,(root,index_path));reader=Reader(progress);reader.directory(root)
    H=load_host(root,reader)
    index=load(reader.read(index_path,MAX_JSON))
    shape(index,('schema','state','native_acceptance','raw_receipts_inspected','producer_authenticated','plan','shards','suites','scope'))
    need(index['schema'] == 'gooo/native-shard-reference-index/v2' and index['state'] == 'RAW_REFERENCES_CHECKED_PRODUCER_UNAUTHENTICATED' and index['native_acceptance'] is False and index['raw_receipts_inspected'] is True and index['producer_authenticated'] is False,'experimental unauthenticated reference state only')
    text(index['scope'])
    def inspect_plan():return validate_plan(root,reader,H,index['plan'])
    plan,owners,source_bytes,entries,lock=inspect_plan() if progress is None else progress.measured('reference-plan','reference',inspect_plan)
    need(type(index['shards']) is list and len(index['shards']) == len(plan['shards']) and type(index['suites']) is list and len(index['suites']) == len(plan['suites']),'complete reference shard/suite population')
    compiler_raw=reader.read(root/'.gooo');partials={};tools={};roots={};records={}
    for expected,shard in zip(plan['shards'],index['shards']):
        shape(shard,('index','namespace','evidence_root','partial','compiler_identity','command_observations','tool_observations'))
        number=expected['index'];need(equal(shard['index'],number) and shard['namespace'] == expected['namespace'],'exact ordered reference shard identity')
        evidence_root=reader.directory(absolute(shard['evidence_root']))
        need(evidence_root not in roots.values(),'distinct canonical shard evidence roots');roots[number]=evidence_root
        _,raw=reader.file(evidence_root,shard['partial'],'partial.json',MAX_JSON);partial=validate_partial(load(raw),plan,number);partials[number]=partial
        _,raw=reader.file(evidence_root,shard['compiler_identity'],'compiler.json',1<<20);validate_compiler(load(raw),lock)
        _,raw=reader.file(evidence_root,shard['tool_observations'],'tool-observations.json',MAX_JSON);tool=validate_tools(load(raw),plan,number,compiler_raw);tools[number]=tool
        _,raw=reader.file(evidence_root,shard['command_observations'],'command-observations.json',MAX_JSON);validate_commands(load(raw),plan,partial,tool,H)
        for record in partial['suites']:records[record['original_index']]=(number,record)
    need(set(records) == set(range(len(plan['suites']))),'all original suites exactly once across joined shards')
    addition=None;total_batches=0;total_cases=0;total_exhaustive=0
    if progress is not None:progress.diagnostic('checkpoint','reference-batches','STARTED')
    for original,reference in enumerate(index['suites']):
        shape(reference,REFERENCE_SUITE_FIELDS);suite=plan['suites'][original];workspace=plan['workspaces'][original];shard,record=records[original]
        need(equal(reference['original_index'],original) and equal(reference['shard_index'],shard) and reference['package'] == suite['package'] and reference['activity'] == suite['activity'],'exact ordered reference original suite')
        need(type(reference['batches']) is list and len(reference['batches']) == len(record['batches']),'complete referenced original batch population')
        source_map={a['name']:a['identity'] for a in workspace['sources']};source_map={name:source_map[name] for name in sorted(source_map)}
        anchors=[];receipt_files={};first_generated=None;tool=tools[shard];go_version='go version '+tool['go_version']+' '+tool['goos']+'/'+tool['goarch'];go_binary=tool['before']['go']
        for metadata,ref in zip(record['batches'],reference['batches']):
            shape(ref,REFERENCE_BATCH_FIELDS);need(equal(ref['batch'],metadata),'reference batch must equal original partial record')
            base=metadata['namespace'];batch=metadata['index'];evidence_root=roots[shard]
            local_manifest,manifest_raw=reader.file(evidence_root,ref['workspace'],base+'/gooo.workspace.json')
            _,case_raw=reader.file(evidence_root,ref['cases'],base+'/cases.json',32<<20)
            _,execution=reader.file(evidence_root,ref['execution'],base+'/execution.json')
            _,replay=reader.file(evidence_root,ref['replay'],base+'/replay.json')
            need(digest(manifest_raw) == metadata['manifest_sha256'] and digest(case_raw) == metadata['cases_sha256'] and digest(execution) == metadata['execution_sha256'] and digest(replay) == metadata['replay_sha256'],'batch raw reference/partial hashes')
            manifest=load(manifest_raw);shape(manifest,('schema','entry','packages'))
            need(manifest['schema'] == 'gooo/package-workspace-manifest/v1' and equal(manifest['entry'],{'package_path':suite['package'],'activity':suite['activity']}),'actual original workspace manifest entry')
            copied=[]
            for pkg in manifest['packages']:
                shape(pkg,('path','sources'));copied.extend((name,pkg['path']) for name in pkg['sources'])
            need(equal(copied,[(a['name'],a['identity']['Package']) for a in workspace['sources']]),'actual exact ordered manifest source closure')
            need(type(ref['sources']) is list and len(ref['sources']) == len(workspace['sources']),'all referenced copied sources')
            for anchor,copy in zip(workspace['sources'],ref['sources']):
                shape(copy,('name','file'));need(copy['name'] == anchor['name'],'original source ref order')
                _,raw=reader.file(evidence_root,copy['file'],base+'/'+anchor['name'])
                need(raw == source_bytes[anchor['name']] and digest(raw) == anchor['raw_sha256'],'copied source equals current original bytes')
            producer=text(ref['producer_manifest_path']);need(producer.startswith('/'),'original absolute producer manifest data required')
            need(ref['go_version'] == go_version and equal(binary_ref(ref['go_binary']),go_binary),'reference actual Go tool binding')
            cases=load(case_raw,metadata=False);shape(cases,('schema','cases'))
            need(cases['schema'] == 'gooo/body-composition-cases/v1' and type(cases['cases']) is list and len(cases['cases']) == metadata['end']-metadata['start'],'exact preserved original case document interval')
            producer_workspace={**workspace,'compiler_revision':plan['run']['compiler_revision']}
            generated=inspect_receipt(execution,cases['cases'],metadata,producer_workspace,producer,go_version,go_binary,entries[original],suite,source_map)
            replay_generated=inspect_receipt(replay,cases['cases'],metadata,producer_workspace,producer,go_version,go_binary,entries[original],suite,source_map,execution)
            need(generated == replay_generated and (first_generated is None or generated == first_generated),'same exact generated source across original batches and saved replay')
            first_generated=generated
            anchors.append({'batch_index':batch,'start':metadata['start'],'end':metadata['end'],'manifest_digest':metadata['manifest_sha256'],'cases_digest':metadata['cases_sha256'],'original_receipt_sha256':metadata['execution_sha256'],'workspace':local_manifest,'producer_manifest_path':producer})
            for key in ('execution','replay'):
                logical=base+'/'+key+'.json'
                receipt_files[str(evidence_root/logical)]=(evidence_root,ref[key],logical)
            total_batches+=1;total_cases+=len(cases['cases'])
            if progress is not None:progress.diagnostic('batch_completed',shard,original,batch,len(cases['cases']))
        inspect_exhaustive(reader,roots[shard],suite,shard,record['exhaustive'],reference['exhaustive'],record['batches'][0]['generated_sha256'])
        total_exhaustive+=len(reference['exhaustive'])
        if suite['package'] == ADDITION_PACKAGE and suite['activity'] == ADDITION_ACTIVITY:
            need(addition is None,'one original addition suite')
            addition={'source_set':source_map,'source_set_sha256':digest(go_json(source_map)),'batches':anchors,'go_version':go_version,'go_binary':go_binary,
                      'receipt_reader':ConsumerReceiptReader(reader,receipt_files)}
    need(total_cases == plan['cases'] and total_batches == plan['batches'] and total_exhaustive == 7,'all original cases/batches/seven exhaustive reports checked')
    need(addition is not None,'original addition consumer suite required')
    if progress is None:reader.finish()
    else:
        progress.diagnostic('checkpoint','reference-batches','COMPLETED')
        progress.measured('provider-final','reference',reader.finish,reader.pins)
    return {'pins':reader.pins,**addition,'suite_count':len(plan['suites'])}
