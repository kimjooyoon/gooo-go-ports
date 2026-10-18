#!/usr/bin/env python3
"""Tiny unsigned synthetic fixtures only. No Go, compiler, native or CI runs.

The public-reader fixture patches only current full-plan reconstruction and the
fixed host loader. Eight one-row suites exercise all seven exhaustive artifact
shapes; they are not the real82-suite population or evidence of native success.
Raw references, parsers, joined-partial coverage, cases/receipt/source/tool/wait
checks and the final frozen-file recheck use the production implementation.
"""
import copy
import importlib.util
import json
import os
import pathlib
import tempfile
import types
import unittest
from unittest import mock

ROOT=pathlib.Path(__file__).resolve().parents[1]
SPEC=importlib.util.spec_from_file_location('native_shard_reference',ROOT/'tools/native-shard-reference.py')
R=importlib.util.module_from_spec(SPEC);SPEC.loader.exec_module(R)
HOST_SPEC=importlib.util.spec_from_file_location('shard_reference_test_host',ROOT/'tools/check-header-addition-host.py')
H=importlib.util.module_from_spec(HOST_SPEC);HOST_SPEC.loader.exec_module(H)


def raw_json(value):
    return json.dumps(value,ensure_ascii=False,indent=2).encode()+b'\n'


class SyntheticFixture:
    """A consistency fixture with invented producer claims, never execution."""
    def __init__(self,directory):
        self.root=pathlib.Path(directory).resolve();self.evidence=self.root/'evidence';self.evidence.mkdir()
        self.index_path=self.root/'index.json';self.entries=[];self.source_bytes={};self.owners={}
        self.compiler=b'fake compiler bytes; never executed\n';(self.root/'.gooo').write_bytes(self.compiler)
        self.go={'bytes':15,'sha256':'a'*64};self.generated='package main\nfunc Fixture() int64 { return 1 }\n'
        run={'repository':'fixture/unsigned','run_id':'1','attempt':1,'tested_commit':'1'*40,'tested_tree':'2'*40,'compiler_revision':H.REV,'go_version':'go1.27.2','workers':3}
        pairs=[(R.ADDITION_PACKAGE,R.ADDITION_ACTIVITY),('go/unicode','SimpleFold'),('go/image/color','RGBToYCbCr'),('go/image/color','YCbCrToRGB'),*[( 'go/unicode',name) for name in ('To','ToUpper','ToLower','ToTitle')]]
        suites=[];workspaces=[];sources={}
        for original,(package,activity) in enumerate(pairs):
            arity=2 if activity == 'To' else 3 if activity in ('RGBToYCbCr','YCbCrToRGB') else 1
            name='ports/fixture-%02d.gooo'%original;entry_id='gooo://synthetic/'+activity
            text=('package fixture\nentity Integer id "gooo://std/integer"\nactivity '+activity+'('+', '.join(['Integer']*arity)+') -> Integer id "'+entry_id+'" computes `return 1`\n').encode()
            (self.root/name).parent.mkdir(parents=True,exist_ok=True);(self.root/name).write_bytes(text)
            self.source_bytes[name]=text;self.owners[name]=package
            sources[name]={'mode':'100644','bytes':len(text),'sha256':R.digest(text)}
            suite={'index':original,'package':package,'activity':activity,'cases':1,'batches':1,'source_paths':[name]};suites.append(suite)
            identity={'Package':package,'Digest':R.digest(R.go_json(text.decode(),html=True))}
            anchors=[{'name':name,'raw_sha256':R.digest(text),'identity':identity}]
            manifest={'schema':'gooo/package-workspace-manifest/v1','entry':{'package_path':package,'activity':activity},'packages':[{'path':package,'sources':[name]}]}
            workspaces.append({'original_index':original,'manifest_sha256':R.digest(raw_json(manifest)),'source_set_sha256':R.digest(R.go_json(anchors)),'entry_producer':entry_id,'sources':anchors})
            self.entries.append({'source':name,'id':entry_id,'inputs':['gooo://std/integer']*arity,'output':'gooo://std/integer'})
        self.plan={'schema':'gooo/native-ci-shard-plan/v1','plan_sha256':'sha256:'+'b'*64,'contract_sha256':'sha256:'+'c'*64,'run':run,'profiles':{'case':'fixture','workspace':'fixture'},'sources':sources,'suites':suites,'workspaces':workspaces,'shards':R.assignments(suites,1),'cases':8,'batches':8}
        self.partial={'schema':'gooo/native-ci-shard-partial/v1','plan_sha256':self.plan['plan_sha256'],'run':run,'index':0,'namespace':R.namespace(0),'sources':sources,'joined':True,'state':'SUCCEEDED','suites':[]}
        self.tools={'schema':'gooo/native-shard-tool-observations/v1','plan_sha256':self.plan['plan_sha256'],'run':run,'index':0,'goos':'linux','goarch':'amd64','go_version':'go1.27.2','before':{'compiler':{'bytes':len(self.compiler),'sha256':R.digest(self.compiler)[7:]},'go':self.go,'verifier':{'bytes':11,'sha256':'d'*64}},'joined':True,'cache_close_succeeded':True,'scope':'Invented fixture observations; not actual waited native work.'}
        self.tools['after']=copy.deepcopy(self.tools['before'])
        accounting={'status':'OBSERVED','goos':'linux','scope':'waited-command-accounting','user_cpu_ns':1,'system_cpu_ns':0,'maxrss_raw':2,'maxrss_unit':'KiB','maxrss_bytes':2048}
        self.commands={'schema':'gooo/native-shard-command-observations/v1','plan_sha256':self.plan['plan_sha256'],'run':run,'index':0,'namespace':R.namespace(0),'sources':sources,'goos':'linux','command_resources_schema':'gooo/native-command-resources/v1','joined':True,'complete':True,'state':'WAIT_ACCOUNTING_COMPLETE_NATIVE_UNACCEPTED','failure':'','native_acceptance':False,'producer_authenticated':False,'suites':[],'scope':'Invented synthetic wait records only.'}
        self.index={'schema':'gooo/native-shard-reference-index/v1','state':'RAW_REFERENCES_CHECKED_PRODUCER_UNAUTHENTICATED','native_acceptance':False,'raw_receipts_inspected':True,'producer_authenticated':False,'plan':self.plan,'shards':[],'suites':[],'scope':'Tiny unsigned test fixture; not actual full native evidence.'}
        for original,suite in enumerate(suites):
            entry=self.entries[original];workspace=workspaces[original];base=R.batch_namespace(0,original,0);key=suite['package']+':'+suite['activity']
            manifest={'schema':'gooo/package-workspace-manifest/v1','entry':{'package_path':suite['package'],'activity':suite['activity']},'packages':[{'path':suite['package'],'sources':suite['source_paths']}]}
            inputs={key:0} if len(entry['inputs']) == 1 else {key+'.input'+str(i):i for i in range(len(entry['inputs']))}
            cases={'schema':'gooo/body-composition-cases/v1','cases':[{'inputs':inputs,'expected':{key:1}}]};case_raw=raw_json(cases)
            producer='/invented/producer/%d/gooo.workspace.json'%original
            metadata={'index':0,'start':0,'end':1,'namespace':base,'cases_sha256':R.digest(case_raw),'manifest_sha256':workspace['manifest_sha256'],'source_set_sha256':workspace['source_set_sha256'],'entry_producer':entry['id'],'generated_sha256':R.digest(self.generated.encode())}
            delivery={'activity_id':entry['id'],'passed':True,'actual':1,'expected':1}
            if len(entry['inputs']) == 1:delivery['input']=0
            else:delivery['inputs']=[{'port':'input'+str(i),'entity_id':'gooo://std/integer','value':i} for i in range(len(entry['inputs']))]
            receipt={'schema':'gooo/workspace-body-execution-receipt/v1','decision':'PASS','manifest':producer,'manifest_digest':metadata['manifest_sha256'],'cases_digest':metadata['cases_sha256'],'result':{'program':{'entry':manifest['entry'],'workspace_image':{'packages':[{'path':suite['package'],'sources':[{'filename':suite['source_paths'][0],'source_digest':workspace['sources'][0]['identity']['Digest']}],'exports':[{'name':suite['activity'],'kind':'activity','id':entry['id'],'input_types':entry['inputs']}]}]}},'runtime':{'schema':'gooo/body-composition-runtime/v1','stage':'COMPLETE','producer_source_sha':H.REV,'go_version':'go version go1.27.2 linux/amd64','go_tool_sha256':'sha256:'+self.go['sha256'],'go_tool_selection':'explicit','model_calls':0,'finite_total':1,'finite_passed':1,'projection_replayed':True,'runtime_replayed':True,'generated_sha256':metadata['generated_sha256'],'traces':[{'case_index':0,'deliveries':[delivery]}]},'composition':{'source':self.generated,'generated_sha256':metadata['generated_sha256'],'steps':[{'generation':{'report':{'activity_id':entry['id']}}}]}}}
            execution=raw_json(receipt);replay=copy.deepcopy(receipt);replay['replayed_from_sha256']=R.digest(execution);replay['result']['replay']={'schema':'gooo/workspace-body-replay/v1','model_calls':0};replay=raw_json(replay)
            metadata.update(execution_sha256=R.digest(execution),replay_sha256=R.digest(replay),replayed_from=R.digest(execution))
            reference={'batch':metadata,'workspace':self.file(base+'/gooo.workspace.json',raw_json(manifest)),'cases':self.file(base+'/cases.json',case_raw),'execution':self.file(base+'/execution.json',execution),'replay':self.file(base+'/replay.json',replay),'sources':[{'name':suite['source_paths'][0],'file':self.file(base+'/'+suite['source_paths'][0],self.source_bytes[suite['source_paths'][0]])}],'producer_manifest_path':producer,'go_version':'go version go1.27.2 linux/amd64','go_binary':self.go}
            report_refs=[];reports=[];kind=R.exhaustive_kind(suite)
            if kind:
                report=R.exhaustive_expected(kind,metadata['generated_sha256']);report_sha=R.digest(R.go_json(report,sorted_map=True));reports=[{'kind':kind,'report':report,'sha256':report_sha}]
                directory='simple-fold-exhaustive' if kind == 'simple-fold' else kind.replace('/','-exhaustive-');prefix=R.namespace(0)+'/suite-%04d/'%original+directory+'/'
                module,test=('gooo-simple-fold-exhaustive','TestSimpleFoldExhaustive') if kind == 'simple-fold' else (('gooo-ycbcr-exhaustive','TestYCbCrExhaustive') if kind.startswith('ycbcr/') else ('gooo-case-mapping-exhaustive','TestCaseMappingExhaustive'))
                files=[('native/generated.go',self.generated.encode()),('native/exhaustive_test.go',('func '+test+'(t *testing.T) { /* '+metadata['generated_sha256']+' */ }\n').encode()),('native/go.mod',('module '+module+'\n\ngo 1.27.2\n').encode()),('native-test.txt',('--- PASS: '+test+' (0.00s)\nPASS\n').encode()),('exhaustive.json',raw_json(report))]
                report_refs=[{'kind':kind,'report_sha256':report_sha,'files':[self.file(prefix+name,raw) for name,raw in files]}]
            self.partial['suites'].append({'original_index':original,'package':suite['package'],'activity':suite['activity'],'state':'VERIFIED','error':'','batches':[metadata],'exhaustive':reports})
            self.commands['suites'].append({'original_index':original,'state':'VERIFIED','batches':[{'index':0,'namespace':base,'verified_prefix':True,'command_resources':{'execute':copy.deepcopy(accounting),'replay':copy.deepcopy(accounting)}}]})
            self.index['suites'].append({'original_index':original,'shard_index':0,'package':suite['package'],'activity':suite['activity'],'batches':[reference],'exhaustive':report_refs})
        self.compiler_identity={'schema':'gooo/build-identity/v1','version':'fixture-version','compiler_source_sha':H.REV,'vcs_revision':H.REV,'source_status':'CLEAN_VCS','vcs_modified':'false','go_version':'go1.27.2','native_go_required':'go1.27.2'}
        self.sync()

    def file(self,name,raw):
        path=self.evidence/name;path.parent.mkdir(parents=True,exist_ok=True);path.write_bytes(raw)
        return {'path':name,'bytes':len(raw),'sha256':R.digest(raw)}

    def sync(self):
        shard={'index':0,'namespace':R.namespace(0),'evidence_root':str(self.evidence),'partial':self.file('partial.json',raw_json(self.partial)),'compiler_identity':self.file('compiler.json',raw_json(self.compiler_identity)),'command_observations':self.file('command-observations.json',raw_json(self.commands)),'tool_observations':self.file('tool-observations.json',raw_json(self.tools))}
        self.index['shards']=[shard];self.index_path.write_bytes(raw_json(self.index))

    def validate_fixture_plan(self,root,reader,host,plan):
        # Deliberately replace ONLY the current full305-source/82-suite loading.
        # The real production validate_plan remains mandatory outside this test.
        R.need(R.equal(plan,self.plan),'fixture plan identity')
        for name,raw in self.source_bytes.items():R.need(reader.read(root/name) == raw,'synthetic original source drift')
        return plan,self.owners,self.source_bytes,self.entries,{'compiler_version':'fixture-version','compiler_revision':H.REV,'go_version':'go1.27.2'}

    def read(self):
        with mock.patch.object(R,'load_host',return_value=H),mock.patch.object(R,'validate_plan',side_effect=self.validate_fixture_plan):
            return R.read_reference(self.root,self.index_path)


class StrictReaderTests(unittest.TestCase):
    def test_duplicate_unknown_truncated_and_nondecimal_json(self):
        for raw in (b'{"x":1,"x":2}',b'{"x":',b'{}{}',b'{"x":NaN}',b'{"x":1e0}',b'{"x":1.5}',b'{"x":-0}',b'{"x":9223372036854775808}'):
            with self.subTest(raw=raw),self.assertRaises(RuntimeError):R.load(raw)
        with self.assertRaises(RuntimeError):R.shape({'known':1,'unknown':2},('known',))

    def test_canonical_hash_uses_no_html_no_lf_and_escaped_line_separators(self):
        self.assertEqual(R.go_json({'x':'<&>\u2028\u2029'}),b'{"x":"<&>\\u2028\\u2029"}')
        self.assertEqual(R.go_json('<&>',html=True),b'"\\u003c\\u0026\\u003e"')
        self.assertEqual(R.go_json({'z':1,'a':2},sorted_map=True),b'{"a":2,"z":1}')
        for value in (True,False,0,-1):
            with self.subTest(value=value),self.assertRaises(RuntimeError):R.integer(value,1)

    def test_path_symlink_hardlink_cap_and_postread_drift(self):
        with tempfile.TemporaryDirectory() as directory:
            root=pathlib.Path(directory).resolve();path=root/'raw';path.write_bytes(b'bytes');reader=R.Reader();reader.read(path)
            path.write_bytes(b'other')
            with self.assertRaisesRegex(RuntimeError,'changed|frozen'):reader.finish()
            link=root/'link';link.symlink_to(path)
            with self.assertRaises(RuntimeError):R.Reader().read(link)
            alias=root/'alias';os.link(path,alias);reader=R.Reader();reader.read(path)
            with self.assertRaisesRegex(RuntimeError,'alias'):reader.read(alias)
            with self.assertRaisesRegex(RuntimeError,'bounded'):R.Reader().read(path,4)
        for name in ('/absolute','a//b','a/../b','a/./b','a\\b','x:y'):
            with self.subTest(name=name),self.assertRaises(RuntimeError):R.relative(name)

    def test_plan_uses_independent_balanced_original_order(self):
        suites=[{'index':0,'cases':8,'batches':2},{'index':1,'cases':10,'batches':3},{'index':2,'cases':9,'batches':3}]
        self.assertEqual(R.assignments(suites,2),[{'index':0,'namespace':R.namespace(0),'original_indices':[0,1]},{'index':1,'namespace':R.namespace(1),'original_indices':[2]}])


class SelectedPackageEntityTests(unittest.TestCase):
    def test_actual_current_simple_fold_four_source_closure(self):
        expected=H.reviewed_contract(ROOT)
        suite=next(row for row in expected.document['suites']
                   if row['package'] == 'go/unicode' and row['activity'] == 'SimpleFold')
        self.assertEqual(suite['index'],35);self.assertEqual(len(suite['source_paths']),4)
        catalog=R.load((ROOT/'verification-catalog.json').read_bytes())
        owners={name:package['path'] for package in catalog['packages'] for name in package['sources']}
        sources={name:(ROOT/name).read_bytes() for name in suite['source_paths']}
        self.assertNotIn(b'entity Integer id',sources['ports/unicode/simple_fold.gooo'])
        self.assertIn(b'entity Integer id "gooo://std/integer"',sources['ports/unicode/space.gooo'])
        self.assertEqual(R.entry_contract(suite,owners,sources),
                         {'source':'ports/unicode/simple_fold.gooo','id':'gooo://go/unicode/SimpleFold',
                          'inputs':['gooo://std/integer'],'output':'gooo://std/integer'})

    def test_all_current82_entry_projections_and_five_imported_type_entries(self):
        expected=H.reviewed_contract(ROOT)
        catalog=R.load((ROOT/'verification-catalog.json').read_bytes())
        owners={name:package['path'] for package in catalog['packages'] for name in package['sources']}
        sources={name:(ROOT/name).read_bytes() for name in owners}
        entries={}
        for suite in expected.document['suites']:
            with self.subTest(index=suite['index'],activity=suite['activity']):
                entries[suite['index']]=R.entry_contract(suite,owners,sources)
        self.assertEqual(len(entries),82)
        rectangle='gooo://go/image/rectangle'
        for index,activity,inputs,output in [
            (34,'CropPlan',[rectangle,rectangle],'gooo://example/crop-planner/result'),
            (41,'DragCropPlan',[rectangle,rectangle],'gooo://example/crop-planner/result'),
            (44,'PixelBounds',[rectangle,rectangle],'gooo://example/fixed-pixel-bounds/result'),
            (51,'MattePixel',['gooo://go/image/color/nrgba','gooo://go/image/color/rgb'],'gooo://go-ports/examples/alpha-matte/result'),
            (58,'FitsCrop',[rectangle,rectangle],'gooo://std/boolean')]:
            self.assertEqual(expected.document['suites'][index]['activity'],activity)
            self.assertEqual(entries[index]['inputs'],inputs);self.assertEqual(entries[index]['output'],output)
        self.assertEqual(entries[35]['id'],'gooo://go/unicode/SimpleFold')

    def test_explicit_package_import_union_resolves_selected_dependency_entity(self):
        suite={'package':'go/example','activity':'Entry','source_paths':['entry.gooo','imports.gooo','dependency.gooo','foreign.gooo']}
        owners={'entry.gooo':'go/example','imports.gooo':'go/example','dependency.gooo':'go/dependency','foreign.gooo':'go/foreign'}
        sources={'entry.gooo':b'activity Entry(Integer) -> Integer id "gooo://example/Entry" computes `return input`\n',
                 'imports.gooo':b'import dependency "go/dependency"\n',
                 'dependency.gooo':b'entity Integer id "gooo://dependency/integer"\n',
                 'foreign.gooo':b'entity Integer id "gooo://foreign/integer"\n'}
        expected={'source':'entry.gooo','id':'gooo://example/Entry','inputs':['gooo://dependency/integer'],'output':'gooo://dependency/integer'}
        self.assertEqual(R.entry_contract(suite,owners,sources),expected)
        sources['imports.gooo']=b'import "go/dependency"\n'
        suite['source_paths'].reverse()
        self.assertEqual(R.entry_contract(suite,owners,sources),expected)

    def imported_fixture(self):
        suite={'package':'go/example','activity':'Entry','source_paths':['entry.gooo','a.gooo','b.gooo']}
        owners={'entry.gooo':'go/example','a.gooo':'go/a','b.gooo':'go/b'}
        sources={'entry.gooo':b'import a "go/a"\nimport b "go/b"\nactivity Entry(Integer) -> Integer id "gooo://example/Entry" computes `return input`\n',
                 'a.gooo':b'entity Integer id "gooo://shared/integer"\n',
                 'b.gooo':b'entity Integer id "gooo://shared/integer"\n'}
        return suite,owners,sources

    def test_imported_same_identity_deduplicates_and_local_type_has_priority(self):
        suite,owners,sources=self.imported_fixture()
        self.assertEqual(R.entry_contract(suite,owners,sources)['inputs'],['gooo://shared/integer'])
        sources['b.gooo']=b'entity Integer id "gooo://different/integer"\n'
        sources['entry.gooo']=b'entity Integer id "gooo://local/integer"\n'+sources['entry.gooo']
        entry=R.entry_contract(suite,owners,sources)
        self.assertEqual(entry['inputs'],['gooo://local/integer']);self.assertEqual(entry['output'],'gooo://local/integer')

    def test_distinct_imported_identities_are_ambiguous(self):
        suite,owners,sources=self.imported_fixture()
        sources['b.gooo']=b'entity Integer id "gooo://different/integer"\n'
        with self.assertRaisesRegex(RuntimeError,'ambiguous imported entity type'):
            R.entry_contract(suite,owners,sources)

    def test_missing_or_unselected_import_and_transitive_entity_refuse(self):
        suite,owners,sources=self.imported_fixture()
        for imported in ('go/missing','go/not-selected'):
            with self.subTest(imported=imported):
                changed=dict(sources);changed['entry.gooo']=('import dep "'+imported+'"\nactivity Entry(Integer) -> Integer id "gooo://example/Entry" computes `return input`\n').encode()
                if imported == 'go/not-selected':
                    changed['not-selected.gooo']=b'entity Integer id "gooo://unselected/integer"\n';owners['not-selected.gooo']=imported
                with self.assertRaisesRegex(RuntimeError,'imported package must be selected'):
                    R.entry_contract(suite,owners,changed)
        sources['entry.gooo']=b'import a "go/a"\nactivity Entry(Integer) -> Integer id "gooo://example/Entry" computes `return input`\n'
        sources['a.gooo']=b'import b "go/b"\n'
        with self.assertRaisesRegex(RuntimeError,'explicit in selected package or declared imports'):
            R.entry_contract(suite,owners,sources)

    def test_same_package_crossfile_entities_and_identical_repetition(self):
        suite={'package':'go/example','activity':'Entry','source_paths':['entry.gooo','shared.gooo','foreign.gooo']}
        owners={'entry.gooo':'go/example','shared.gooo':'go/example','foreign.gooo':'go/foreign'}
        sources={'entry.gooo':b'activity Entry(Integer) -> Integer id "gooo://example/Entry" computes `return input`\n',
                 'shared.gooo':b'entity Integer id "gooo://std/integer"\nentity Integer id "gooo://std/integer"\n',
                 'foreign.gooo':b'entity Integer id "gooo://foreign/integer"\nactivity Entry(Integer) -> Integer id "gooo://foreign/Entry" computes `return input`\n'}
        expected={'source':'entry.gooo','id':'gooo://example/Entry','inputs':['gooo://std/integer'],'output':'gooo://std/integer'}
        self.assertEqual(R.entry_contract(suite,owners,sources),expected)
        suite['source_paths'].reverse()
        self.assertEqual(R.entry_contract(suite,owners,sources),expected)

    def test_foreign_package_cannot_supply_missing_entry_type(self):
        suite={'package':'go/example','activity':'Entry','source_paths':['entry.gooo','foreign.gooo']}
        owners={'entry.gooo':'go/example','foreign.gooo':'go/foreign'}
        sources={'entry.gooo':b'activity Entry(Integer) -> Integer id "gooo://example/Entry" computes `return input`\n',
                 'foreign.gooo':b'entity Integer id "gooo://std/integer"\n'}
        with self.assertRaisesRegex(RuntimeError,'explicit in selected package'):
            R.entry_contract(suite,owners,sources)

    def test_same_package_conflicting_entity_ids_refuse(self):
        suite={'package':'go/example','activity':'Entry','source_paths':['entry.gooo','shared.gooo']}
        owners={'entry.gooo':'go/example','shared.gooo':'go/example'}
        sources={'entry.gooo':b'entity Integer id "gooo://std/integer"\nactivity Entry(Integer) -> Integer id "gooo://example/Entry" computes `return input`\n',
                 'shared.gooo':b'entity Integer id "gooo://conflicting/integer"\n'}
        with self.assertRaisesRegex(RuntimeError,'conflicting entity IDs'):
            R.entry_contract(suite,owners,sources)


class UnsignedSyntheticReferenceTests(unittest.TestCase):
    def fixture(self):
        directory=tempfile.TemporaryDirectory();self.addCleanup(directory.cleanup)
        return SyntheticFixture(directory.name)

    def test_exact_seven_key_api_and_local_producer_path_separation(self):
        fixture=self.fixture();before={p:p.read_bytes() for p in fixture.evidence.rglob('*') if p.is_file()};result=fixture.read()
        self.assertEqual(set(result),{'pins','source_set','source_set_sha256','batches','go_version','go_binary','suite_count'})
        self.assertEqual(result['suite_count'],8);self.assertEqual(len(result['batches']),1)
        batch=result['batches'][0];self.assertIsInstance(batch['workspace'],pathlib.Path)
        self.assertNotEqual(str(batch['workspace']),batch['producer_manifest_path']);self.assertNotIn(batch['producer_manifest_path'],result['pins'])
        self.assertTrue(all(p.read_bytes() == raw for p,raw in before.items()))
        self.assertEqual(result['source_set_sha256'],R.digest(R.go_json(result['source_set'])))
        self.assertEqual(len(fixture.index['suites']),8);self.assertEqual(sum(len(s['exhaustive']) for s in fixture.index['suites']),7)

    def test_missing_duplicate_foreign_or_unjoined_records_refuse(self):
        for change in ('missing-suite','duplicate-suite','foreign-index','missing-batch','unjoined','failed','case-gap','namespace','extra-index-field'):
            with self.subTest(change=change):
                fixture=self.fixture()
                if change=='missing-suite':fixture.index['suites'].pop()
                elif change=='duplicate-suite':fixture.index['suites'][1]=copy.deepcopy(fixture.index['suites'][0])
                elif change=='foreign-index':fixture.partial['suites'][0]['original_index']=99
                elif change=='missing-batch':fixture.partial['suites'][0]['batches']=[]
                elif change=='unjoined':fixture.partial['joined']=False
                elif change=='failed':fixture.partial['state']='FAILED'
                elif change=='case-gap':fixture.partial['suites'][0]['batches'][0]['start']=1
                elif change=='namespace':fixture.partial['namespace']='foreign'
                else:fixture.index['unrecognized']=True
                fixture.sync()
                with self.assertRaises((RuntimeError,KeyError,ValueError)):fixture.read()

    def test_authority_and_typed_false_flags_cannot_be_upgraded(self):
        for container,key,value in [('index','native_acceptance',True),('index','producer_authenticated',True),('index','native_acceptance',0),('index','raw_receipts_inspected',1),('commands','native_acceptance',True),('commands','producer_authenticated',True)]:
            with self.subTest(container=container,key=key,value=value):
                fixture=self.fixture();getattr(fixture,container)[key]=value;fixture.sync()
                with self.assertRaises(RuntimeError):fixture.read()

    def test_raw_hash_original_parent_current_source_and_tool_drift_refuse(self):
        for change in ('raw-hash','saved-parent','source-copy','current-source','go-binary','compiler-binary','tool-after','compiler-identity'):
            with self.subTest(change=change):
                fixture=self.fixture();ref=fixture.index['suites'][0]['batches'][0];metadata=ref['batch']
                if change=='raw-hash':(fixture.evidence/ref['execution']['path']).write_bytes(b'changed raw receipt')
                elif change=='saved-parent':
                    doc=R.load((fixture.evidence/ref['replay']['path']).read_bytes(),metadata=False);doc['replayed_from_sha256']='sha256:'+'e'*64
                    ref['replay']=fixture.file(ref['replay']['path'],raw_json(doc));metadata['replay_sha256']=ref['replay']['sha256']
                elif change=='source-copy':ref['sources'][0]['file']=fixture.file(ref['sources'][0]['file']['path'],b'changed copied source')
                elif change=='current-source':(fixture.root/fixture.plan['suites'][0]['source_paths'][0]).write_bytes(b'changed original source')
                elif change=='go-binary':ref['go_binary']={'bytes':15,'sha256':'f'*64}
                elif change=='compiler-binary':fixture.tools['before']['compiler']['sha256']='e'*64;fixture.tools['after']=copy.deepcopy(fixture.tools['before'])
                elif change=='tool-after':fixture.tools['after']['go']['sha256']='e'*64
                else:fixture.compiler_identity['vcs_modified']='true'
                fixture.sync()
                with self.assertRaises(RuntimeError):fixture.read()

    def test_command_coverage_and_wait_units_refuse(self):
        for change in ('missing-suite','missing-replay','unobserved','prefix','rss-unit','failure'):
            with self.subTest(change=change):
                fixture=self.fixture();batch=fixture.commands['suites'][0]['batches'][0]
                if change=='missing-suite':fixture.commands['suites'].pop()
                elif change=='missing-replay':del batch['command_resources']['replay']
                elif change=='unobserved':batch['command_resources']['execute']['status']='NOT_OBSERVED'
                elif change=='prefix':batch['verified_prefix']=False
                elif change=='rss-unit':batch['command_resources']['execute']['maxrss_unit']='bytes'
                else:fixture.commands['failure']='wait error'
                fixture.sync()
                with self.assertRaises(RuntimeError):fixture.read()

    def test_reference_paths_and_exhaustive_raw_reports_refuse(self):
        for change in ('path-alias','absolute-path','missing-ref','unknown-ref','producer-local-mismatch','missing-exhaustive','report-contract','report-hash','harness-hash','exhaustive-failure'):
            with self.subTest(change=change):
                fixture=self.fixture();ref=fixture.index['suites'][0]['batches'][0];ex=fixture.index['suites'][1]['exhaustive'][0]
                if change=='path-alias':ref['workspace']['path']=ref['workspace']['path'].replace('/suite-','/../suite-',1)
                elif change=='absolute-path':ref['workspace']['path']='/absolute/gooo.workspace.json'
                elif change=='missing-ref':del ref['cases']
                elif change=='unknown-ref':ref['workspace']['unexpected']=False
                elif change=='producer-local-mismatch':ref['producer_manifest_path']=str(fixture.evidence/ref['workspace']['path'])
                elif change=='missing-exhaustive':fixture.index['suites'][1]['exhaustive']=[]
                elif change=='report-contract':fixture.partial['suites'][1]['exhaustive'][0]['report']['mismatches']=1
                elif change=='report-hash':ex['report_sha256']='sha256:'+'e'*64
                elif change=='harness-hash':(fixture.evidence/ex['files'][1]['path']).write_bytes(b'changed harness')
                else:ex['files'][3]=fixture.file(ex['files'][3]['path'],b'--- FAIL: TestSimpleFoldExhaustive (0.00s)\nFAIL\n')
                fixture.sync()
                with self.assertRaises(RuntimeError):fixture.read()


if __name__=='__main__':
    unittest.main()
