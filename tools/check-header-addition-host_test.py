#!/usr/bin/env python3
"""Source/expectation/provenance rejection tests. No Go or native invocation."""
import importlib.util
import ast
import contextlib
import io
import copy
import json
import pathlib
import tempfile
import unittest
from unittest import mock

ROOT=pathlib.Path(__file__).resolve().parents[1]
SPEC=importlib.util.spec_from_file_location('header_addition_host',ROOT/'tools/check-header-addition-host.py')
CHECK=importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(CHECK)

class CommandResourcesTests(unittest.TestCase):
    def accounting(self,goos='linux',status='OBSERVED'):
        value={'status':status,'goos':goos,'scope':'waited-command-accounting',
               'user_cpu_ns':0,'system_cpu_ns':1,'maxrss_raw':2,
               'maxrss_unit':'KiB' if goos=='linux' else 'bytes',
               'maxrss_bytes':2048 if goos=='linux' else 2}
        if status!='OBSERVED':
            value['reason']='OS wait accounting is unavailable'
            for key in ('user_cpu_ns','system_cpu_ns','maxrss_raw','maxrss_unit','maxrss_bytes'):value[key]=None
        return value

    def timing(self,goos='linux',status='OBSERVED'):
        return ({'command_resources_schema':'gooo/native-command-resources/v1','goos':goos},
                [{'command_resources':{phase:self.accounting(goos,status) for phase in ('execute','replay')}}])

    def test_command_resources_observed_linux_and_darwin_exact_units(self):
        for goos in ('linux','darwin'):
            header,batches=self.timing(goos)
            self.assertIsNone(CHECK.validate_command_resources(header,batches))
            # Zero and large valid accounting have identical acceptance. There is
            # no CPU/elapsed comparison, RSS budget or semantic PASS predicate.
            for phase in ('execute','replay'):
                value=batches[0]['command_resources'][phase]
                for key in ('user_cpu_ns','system_cpu_ns'):value[key]=9223372036854775807
                value['maxrss_raw']=9223372036854775807//1024 if goos=='linux' else 9223372036854775807
                value['maxrss_bytes']=value['maxrss_raw']*(1024 if goos=='linux' else 1)
            self.assertIsNone(CHECK.validate_command_resources(header,batches))
            for phase in ('execute','replay'):
                for key in ('user_cpu_ns','system_cpu_ns','maxrss_raw','maxrss_bytes'):batches[0]['command_resources'][phase][key]=0
            self.assertIsNone(CHECK.validate_command_resources(header,batches))

    def test_command_resources_unavailable_preserves_coverage_without_numbers(self):
        for goos in ('linux','darwin','freebsd'):
            header,batches=self.timing(goos,'NOT_AVAILABLE')
            self.assertIsNone(CHECK.validate_command_resources(header,batches))
        header,batches=self.timing()
        batches[0]['command_resources']['replay']=self.accounting('linux','NOT_AVAILABLE')
        self.assertIsNone(CHECK.validate_command_resources(header,batches))

    def test_command_resources_current_reader_rejects_legacy_and_unknown_schema(self):
        header,batches=self.timing()
        for schema in (None,'gooo/native-command-resources/v0',1,True,{}):
            h=copy.deepcopy(header)
            if schema is None:h.pop('command_resources_schema')
            else:h['command_resources_schema']=schema
            with self.subTest(schema=schema),self.assertRaises(RuntimeError):CHECK.validate_command_resources(h,batches)
        for goos in (None,'',' ',1,True):
            h=copy.deepcopy(header);h['goos']=goos
            with self.subTest(goos=goos),self.assertRaises(RuntimeError):CHECK.validate_command_resources(h,batches)

    def test_command_resources_every_batch_requires_both_exact_cli_objects(self):
        header,batches=self.timing()
        for phase in ('execute','replay'):
            for mutation in ('missing','null','list','extra'):
                bs=copy.deepcopy(batches);resources=bs[0]['command_resources']
                if mutation=='missing':resources.pop(phase)
                elif mutation=='extra':resources['other']=copy.deepcopy(resources[phase])
                else:resources[phase]=None if mutation=='null' else []
                with self.subTest(phase=phase,mutation=mutation),self.assertRaises(RuntimeError):CHECK.validate_command_resources(header,bs)
        for value in (None,[],{}):
            with self.subTest(value=value),self.assertRaises(RuntimeError):CHECK.validate_command_resources(header,[{'command_resources':value}])
        bs=copy.deepcopy(batches);bs.append({})
        with self.assertRaises(RuntimeError):CHECK.validate_command_resources(header,bs)
        with self.assertRaises(RuntimeError):CHECK.validate_command_resources(header,[None])

    def test_command_resources_exact_fields_reject_omission_addition_and_reason_on_observed(self):
        header,batches=self.timing()
        for phase in ('execute','replay'):
            for key in self.accounting():
                bs=copy.deepcopy(batches);bs[0]['command_resources'][phase].pop(key)
                with self.subTest(phase=phase,missing=key),self.assertRaises(RuntimeError):CHECK.validate_command_resources(header,bs)
            for key,value in (('reason',''),('user_cpu_s',0),('verified',True),('authorization','PASS')):
                bs=copy.deepcopy(batches);bs[0]['command_resources'][phase][key]=value
                with self.subTest(phase=phase,extra=key),self.assertRaises(RuntimeError):CHECK.validate_command_resources(header,bs)

    def test_command_resources_current_verified_batches_reject_unobserved_and_unknown_status(self):
        header,batches=self.timing()
        for phase in ('execute','replay'):
            for status in ('NOT_OBSERVED','PASS','',None,True,{}):
                bs=copy.deepcopy(batches);bs[0]['command_resources'][phase]=self.accounting('linux',status)
                with self.subTest(phase=phase,status=status),self.assertRaises(RuntimeError):CHECK.validate_command_resources(header,bs)

    def test_command_resources_all_observed_numbers_are_nonbool_int64(self):
        header,batches=self.timing()
        for phase in ('execute','replay'):
            for key in ('user_cpu_ns','system_cpu_ns','maxrss_raw','maxrss_bytes'):
                for value in (-1,9223372036854775808,True,False,1.0,'1',None):
                    bs=copy.deepcopy(batches);bs[0]['command_resources'][phase][key]=value
                    with self.subTest(phase=phase,key=key,value=value),self.assertRaises(RuntimeError):CHECK.validate_command_resources(header,bs)

    def test_command_resources_os_scope_and_raw_rss_conversion_cannot_be_rebound(self):
        for goos in ('linux','darwin'):
            header,batches=self.timing(goos)
            for phase in ('execute','replay'):
                for key,value in (('goos','darwin' if goos=='linux' else 'linux'),
                                  ('scope','process-tree'),('scope',''),
                                  ('maxrss_unit','bytes' if goos=='linux' else 'KiB'),
                                  ('maxrss_unit',None),('maxrss_bytes',2 if goos=='linux' else 2048),
                                  ('maxrss_raw',3),('maxrss_raw',9223372036854775807)):
                    bs=copy.deepcopy(batches);bs[0]['command_resources'][phase][key]=value
                    with self.subTest(goos=goos,phase=phase,key=key,value=value),self.assertRaises(RuntimeError):CHECK.validate_command_resources(header,bs)
        header,batches=self.timing('freebsd')
        with self.assertRaises(RuntimeError):CHECK.validate_command_resources(header,batches)

    def test_command_resources_unavailable_requires_nonempty_reason_and_all_null_values(self):
        header,batches=self.timing(status='NOT_AVAILABLE')
        for phase in ('execute','replay'):
            for key,value in [('reason',None),('reason',''),('reason',' '),('reason',True),
                              ('user_cpu_ns',0),('system_cpu_ns',0),('maxrss_raw',0),
                              ('maxrss_bytes',0),('maxrss_unit','KiB')]:
                bs=copy.deepcopy(batches);bs[0]['command_resources'][phase][key]=value
                with self.subTest(phase=phase,key=key,value=value),self.assertRaises(RuntimeError):CHECK.validate_command_resources(header,bs)
            bs=copy.deepcopy(batches);bs[0]['command_resources'][phase].pop('reason')
            with self.subTest(missing_reason=phase),self.assertRaises(RuntimeError):CHECK.validate_command_resources(header,bs)

    def test_command_resources_current_reader_calls_validation_unconditionally(self):
        import ast
        tree=ast.parse((ROOT/'tools/check-header-addition-host.py').read_text())
        reader=next(node for node in tree.body if isinstance(node,ast.FunctionDef) and node.name=='read_verification')
        calls=[node.value for node in reader.body if isinstance(node,ast.Expr)
               and isinstance(node.value,ast.Call) and isinstance(node.value.func,ast.Name)
               and node.value.func.id=='validate_command_resources']
        self.assertEqual(len(calls),1)
        self.assertEqual([node.id for node in calls[0].args],['header','batches'])
        self.assertFalse(calls[0].keywords)

class FreshHostTests(unittest.TestCase):
    def test_duplicate_decoded_keys_and_nonfinite_refuse(self):
        for raw in [b'{"name":1,"na\\u006de":2}',b'{"x":NaN}',b'{"x":Infinity}',b'{}{}']:
            with self.subTest(raw=raw),self.assertRaises((RuntimeError,ValueError)):
                CHECK.load(raw)
    def test_nested_bool_is_not_integer(self):
        self.assertFalse(CHECK.equal({'value':[True]},{'value':[1]}))
        self.assertFalse(CHECK.equal({'revision':1.0},{'revision':1}))
        self.assertTrue(CHECK.equal({'revision':9223372036854775807},{'revision':9223372036854775807}))
    def test_go_json_source_digest_escapes(self):
        self.assertEqual(CHECK.go_json('<>&\u2028\u2029\n'),b'"\\u003c\\u003e\\u0026\\u2028\\u2029\\n"')
    def test_source_paths_are_local_and_unambiguous(self):
        for name in ['../source','/tmp/source','a/../b','a//b','./a','a\\b','']:
            with self.subTest(name=name),self.assertRaises(RuntimeError):CHECK.safe_relative(name)
        self.assertEqual(CHECK.safe_relative('examples/diagnostic-header-addition/main.gooo'),'examples/diagnostic-header-addition/main.gooo')
    def test_frozen_input_drift_names_the_changed_file(self):
        with tempfile.TemporaryDirectory() as directory:
            path=pathlib.Path(directory).resolve()/'input.json';path.write_bytes(b'first');pins={}
            CHECK.frozen_read(path,pins);path.write_bytes(b'changed')
            with self.assertRaisesRegex(RuntimeError,str(path)):CHECK.unchanged(pins)
    def test_current_roster_includes_all_host_inputs(self):
        catalog=CHECK.load((ROOT/'verification-catalog.json').read_bytes())
        roster=CHECK.source_roster(ROOT,catalog)
        self.assertTrue(set(CHECK.OWN_FILES).issubset(roster))
        self.assertIn('examples/diagnostic-header-addition/main.gooo',roster)
        self.assertIn('cmd/verify/header_addition_corpus.go',roster)
        self.assertIn('ports/autobot/ai_lines.gooo',roster)
        self.assertEqual(len(roster),293)
        self.assertEqual(len(catalog['packages']),56)
        self.assertEqual(sum(len(p['sources']) for p in catalog['packages']),67)
        for name in ('ports/net/textproto/canonical.gooo','examples/canonical-header-key/main.gooo','cmd/verify/canonical_header_contracts.go','cmd/verify/canonical_header_corpus_test.go','examples/canonical-header-key/manual20.json','third_party/go/net/textproto/reader.go.txt','cmd/verify/etag_contracts.go','cmd/verify/etag_native_test.go','ports/net/http/etag.gooo','examples/strong-validator-facts/main.gooo','examples/strong-validator-facts/manual247.json','third_party/go/net/http/fs.go.txt','etag-scan.workspace.json','etag-strong-match.workspace.json','strong-validator-facts.workspace.json'):
            self.assertIn(name,roster)
        for name in ('ports/strings/last_index.gooo','examples/literal-final-marker-report/main.gooo',
                     'literal-final-marker-report.workspace.json','examples/literal-final-marker-report/inputs.json',
                     'cmd/verify/last_index_corpus.go','cmd/verify/last_index_contracts.go',
                     'tools/check-last-index-inputs.py','tools/check-last-index-inputs_test.py'):
            self.assertIn(name,roster)

    def test_policy_builder_roster_entry_is_mandatory(self):
        # The full reviewed data, not a Go source regex, owns the flat roster.
        expected=CHECK.reviewed_contract(ROOT)
        catalog={'packages':[{'path':'synthetic/catalog-membership-only','sources':[
            row['path'] for row in expected.document['sources'] if 'catalog-source' in row['roles']]}]}
        source=(ROOT/CHECK.CONTRACT.REVIEWED_PATH).read_bytes()
        self.assertIn(b'"cmd/build-demo/api_policy_build.go"',source)
        for replacement in (b'cmd/build-demo/substituted.go',b'cmd/build-demo/api_policy_build.gx'):
            mutant=source.replace(b'cmd/build-demo/api_policy_build.go',replacement,1)
            with tempfile.TemporaryDirectory() as directory:
                root=pathlib.Path(directory);path=root/CHECK.CONTRACT.REVIEWED_PATH
                path.parent.mkdir(parents=True);path.write_bytes(mutant)
                with self.subTest(replacement=replacement),self.assertRaisesRegex(RuntimeError,'raw digest'):
                    CHECK.source_roster(root,catalog)

    def test_policy_builder_source_omission_substitution_and_bytes_are_refused(self):
        catalog=CHECK.load((ROOT/'verification-catalog.json').read_bytes())
        roster=CHECK.source_roster(ROOT,catalog)
        sources={name:CHECK.reference((ROOT/name).read_bytes()) for name in roster}
        read=lambda path:path.read_bytes()
        CHECK.validate_timing_sources(ROOT,catalog,sources,read)
        name='cmd/build-demo/api_policy_build.go'
        for mutation in ('missing','substituted','changed-digest'):
            mutant=copy.deepcopy(sources)
            if mutation=='changed-digest':mutant[name]['sha256']='0'*64
            else:
                reference=mutant.pop(name)
                if mutation=='substituted':mutant['cmd/build-demo/substituted.go']=reference
            self.assertNotEqual(mutant,sources)
            with self.subTest(mutation=mutation),self.assertRaises(RuntimeError):
                CHECK.validate_timing_sources(ROOT,catalog,mutant,read)

    def population(self):
        # Small synthetic metadata only, never a replacement native fixture/corpus.
        # Observations are synthetic shape fixtures copied from source-defined
        # metadata, not evidence of native execution or independently run cases.
        definition=CHECK.reviewed_contract(ROOT).document
        runs=[{key:row[key] for key in ('package','activity','cases','batches')} |
              {'passed':row['cases'],'replay_equal':True} for row in definition['suites']]
        summary={'runs':runs,'cases':175649,'passed':175649,
            'canonical_header_reference_cases':1059,'canonical_header_report_reference_cases':1059,
            'canonical_header_raw_input_occurrences':1059,'canonical_header_unique_keys':1054,
            'canonical_header_within_partition_duplicates':0,'canonical_header_cross_partition_duplicates':5,
            'canonical_header_report_fields':8,'canonical_header_port_source_count':2,'canonical_header_report_source_count':4,
            'canonical_header_scope':CHECK.CANONICAL_SUMMARY['canonical_header_scope'],
            'canonical_header_provenance_scope':CHECK.CANONICAL_SUMMARY['canonical_header_provenance_scope']}
        summary.update(CHECK.ETAG_SUMMARY)
        summary.update(CHECK.IF_MATCH_SUMMARY)
        summary.update(CHECK.LAST_INDEX_SUMMARY)
        planned=[{'index':i,'package':r['package'],'activity':r['activity'],'cases':r['cases']} for i,r in enumerate(runs)]
        ends=[{'suite_index':row['index'],'source_set':{name:None for name in row['source_paths']}}
              for row in definition['suites']]
        return summary,planned,ends

    def test_reviewed_catalog_membership_and_all_contract_inputs_are_required(self):
        expected=CHECK.reviewed_contract(ROOT)
        qualified=[row['path'] for row in expected.document['sources'] if 'catalog-source' in row['roles']]
        catalog={'packages':[{'path':'synthetic/membership-only','sources':qualified}]}
        roster=CHECK.source_roster(ROOT,catalog)
        self.assertEqual(len(roster),293)
        self.assertTrue(set(CHECK.OWN_FILES).issubset(roster))
        self.assertTrue(set(CHECK.CONTRACT.CONTRACT_FILES).issubset(roster))
        self.assertEqual(sum('catalog-source' in row['roles'] for row in expected.document['sources']),67)
        for mutation in ('missing','substitute','duplicate'):
            changed=copy.deepcopy(catalog)
            if mutation=='missing':changed['packages'][0]['sources'].pop()
            elif mutation=='substitute':changed['packages'][0]['sources'][0]='ports/substituted.gooo'
            else:changed['packages'][0]['sources'].append(qualified[0])
            with self.subTest(mutation=mutation),self.assertRaises(RuntimeError):CHECK.source_roster(ROOT,changed)

    def test_partial_case_expected_statement_keeps_case_dict_separate_from_ledger(self):
        # Exercise the actual deep-reader statement without pretending to have
        # the absent full native receipt/source fixture in this selected stage.
        import ast
        tree=ast.parse((ROOT/'tools/check-header-addition-host.py').read_text())
        reader=next(node for node in tree.body if isinstance(node,ast.FunctionDef) and node.name=='read_addition_evidence')
        statement=next(node for node in ast.walk(reader) if isinstance(node,ast.Expr)
                       and isinstance(node.value,ast.Call) and isinstance(node.value.func,ast.Name)
                       and node.value.func.id=='need' and len(node.value.args)==2
                       and isinstance(node.value.args[1],ast.Constant)
                       and node.value.args[1].value=='fresh full Go expected agrees with partial source specification')
        code=compile(ast.fix_missing_locations(ast.Module(body=[copy.deepcopy(statement)],type_ignores=[])),
                     '<actual partial source expectation statement>','exec')
        for value in (3,True,4):
            context={'need':CHECK.need,'equal':CHECK.equal,'expected':{'count':value},
                     'spec':{'expected':{'count':3}},'contract_expected':object()}
            if type(value) is int and value==3:exec(code,context)
            else:
                with self.subTest(value=value),self.assertRaises(RuntimeError):exec(code,context)

    def test_appended_owner_cannot_be_redefined_by_catalog_and_event_together(self):
        from types import SimpleNamespace
        root=pathlib.Path('/synthetic-read-only-root')
        self.assertEqual(len(CHECK.APPENDED_SOURCE_OWNERS),11)
        for name,owner in CHECK.APPENDED_SOURCE_OWNERS.items():
            raw=b'// synthetic source bytes only\n'
            catalog={'packages':[{'path':owner,'sources':[name]}]}
            definition=SimpleNamespace(document={'suites':[{'index':0,'source_paths':[name]}]})
            selected={name:{'Package':owner,'Digest':CHECK.digest(CHECK.go_json(raw.decode()))}}
            def event(mapping):return [{'suite_index':0,'source_set':mapping,
                'source_set_sha256':CHECK.digest(json.dumps(mapping,ensure_ascii=False,separators=(',',':')).encode())}]
            CHECK.validate_suite_sources(root,catalog,event(selected),lambda _:raw,definition)
            # Keep paths, bytes, map digest, and catalog/event agreement valid,
            # while changing both labels. The independent owner must refuse it.
            changed=copy.deepcopy(catalog);changed['packages'][0]['path']='go/forged-owner'
            selected[name]['Package']='go/forged-owner'
            with self.subTest(source=name),self.assertRaisesRegex(RuntimeError,'independent appended source owner'):
                CHECK.validate_suite_sources(root,changed,event(selected),lambda _:raw,definition)

    def test_every_suite_current_source_identity_and_owner_are_checked(self):
        # Two-row source-map fixture only: no receipt, compiler, or native run.
        from types import SimpleNamespace
        root=pathlib.Path('/synthetic-read-only-root')
        texts={'ports/a.gooo':'package a\n','ports/b.gooo':'package b\n'}
        catalog={'packages':[{'path':'go/a','sources':['ports/a.gooo']},
                             {'path':'go/b','sources':['ports/b.gooo']}]}
        rows=[{'index':0,'source_paths':['ports/a.gooo']},
              {'index':1,'source_paths':['ports/b.gooo','ports/a.gooo']}]
        definition=SimpleNamespace(document={'suites':rows})
        owner={name:pkg['path'] for pkg in catalog['packages'] for name in pkg['sources']}
        ends=[]
        for row in rows:
            selected={name:{'Package':owner[name],'Digest':CHECK.digest(CHECK.go_json(texts[name]))}
                      for name in sorted(row['source_paths'])}
            ends.append({'suite_index':row['index'],'source_set':selected,
                         'source_set_sha256':CHECK.digest(json.dumps(selected,ensure_ascii=False,separators=(',',':')).encode())})
        read=lambda path:texts[str(path.relative_to(root))].encode()
        CHECK.validate_suite_sources(root,catalog,ends,read,definition)
        for index in (0,1):
            for change in ('owner','digest','map-digest','missing','source-bytes'):
                observed=copy.deepcopy(ends);values=dict(texts)
                first=next(iter(observed[index]['source_set']))
                if change=='owner':observed[index]['source_set'][first]['Package']='go/substituted'
                elif change=='digest':observed[index]['source_set'][first]['Digest']='sha256:'+'0'*64
                elif change=='map-digest':observed[index]['source_set_sha256']='sha256:'+'0'*64
                elif change=='missing':observed[index]['source_set'].pop(first)
                else:values[first]+='// changed\n'
                changed_read=lambda path:values[str(path.relative_to(root))].encode()
                with self.subTest(index=index,change=change),self.assertRaises(RuntimeError):
                    CHECK.validate_suite_sources(root,catalog,observed,changed_read,definition)

    def test_every_legacy_suite_identity_count_and_closure_are_now_checked(self):
        summary,planned,ends=self.population()
        for index in (0,8,24,58,69,70,71,72,73):
            for mutation in ('activity','same-total-count','source'):
                s,p,e=copy.deepcopy((summary,planned,ends))
                if mutation=='activity':s['runs'][index]['activity']='Substitute'
                elif mutation=='same-total-count':
                    s['runs'][index]['cases']+=1;s['runs'][(index+1)%74]['cases']-=1
                else:e[index]['source_set']['ports/substituted.gooo']=None
                with self.subTest(index=index,mutation=mutation),self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)

    def test_current79_metadata_keeps_old76_and_three_exact_appended_entries(self):
        summary,planned,ends=self.population()
        CHECK.current_population(summary,planned,ends)
        self.assertEqual(sum(r['cases'] for r in summary['runs'][:74]),171217)
        self.assertEqual(sum(r['batches'] for r in summary['runs'][:74]),2181)
        self.assertEqual(sum(r['cases'] for r in summary['runs'][:76]),173335)
        self.assertEqual(sum(r['batches'] for r in summary['runs'][:76]),2240)
        for which in ('runs','planned','ends'):
            s,p,e=copy.deepcopy((summary,planned,ends))
            (s['runs'] if which=='runs' else p if which=='planned' else e).pop()
            with self.subTest(omitted=which),self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)
        for key,value in [('cases',171217),('passed',171217),('cases',173335),('passed',173335)]:
            s,p,e=copy.deepcopy((summary,planned,ends));s[key]=value
            with self.subTest(rollback=key),self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)

    def test_current79_metadata_rejects_old_prefix_and_appended_population_drift(self):
        summary,planned,ends=self.population()
        for key in ('cases','batches'):
            s,p,e=copy.deepcopy((summary,planned,ends));s['runs'][0][key]-=1
            with self.subTest(prefix=key),self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)
        for index,key,value in [
            (74,'cases',1054),(74,'batches',21),(75,'batches',36),(75,'activity','CanonicalMIMEHeaderKey'),
            (74,'replay_equal',False),(75,'passed',1058)]:
            s,p,e=copy.deepcopy((summary,planned,ends));s['runs'][index][key]=value
            with self.subTest(index=index,key=key),self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)
        s,p,e=copy.deepcopy((summary,planned,ends));p[75]['index']=74
        with self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)
        s,p,e=copy.deepcopy((summary,planned,ends));e[75]=copy.deepcopy(e[74])
        with self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)

    def test_current79_metadata_rejects_typed_counter_scope_and_closure_changes(self):
        summary,planned,ends=self.population()
        for key in CHECK.CANONICAL_SUMMARY:
            s,p,e=copy.deepcopy((summary,planned,ends));s.pop(key)
            with self.subTest(missing=key),self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)
            s,p,e=copy.deepcopy((summary,planned,ends));s[key]=True if type(s[key]) is int else s[key]+' changed'
            with self.subTest(changed=key),self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)
        for index in (74,75):
            s,p,e=copy.deepcopy((summary,planned,ends));e[index]['source_set'].pop('ports/net/httpguts/headers.gooo')
            e[index]['source_set']['ports/path/path.gooo']=None
            with self.subTest(substituted_source=index),self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)

    def test_current79_metadata_rejects_old76_and_forged_appended_entries(self):
        summary,planned,ends=self.population()
        for forged_totals in (False,True):
            s,p,e=copy.deepcopy((summary,planned,ends));s['runs']=s['runs'][:76];p=p[:76];e=e[:76]
            if not forged_totals:s['cases']=s['passed']=173335
            with self.subTest(forged_totals=forged_totals),self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)
        for index,key,value in [
            (76,'package','go/net/http'),(76,'activity','ETagStrongMatch'),(76,'cases',190),(76,'batches',2),
            (77,'activity','ScanETag'),(77,'passed',30),(77,'batches',1),(77,'replay_equal',False),
            (78,'package','go/net/http/etag'),(78,'cases',23),(78,'batches',1),(78,'passed',True)]:
            s,p,e=copy.deepcopy((summary,planned,ends));s['runs'][index][key]=value
            with self.subTest(index=index,key=key),self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)
        for index in (76,77,78):
            s,p,e=copy.deepcopy((summary,planned,ends));p[index]['index']=75
            with self.subTest(planned=index),self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)
            s,p,e=copy.deepcopy((summary,planned,ends));e[index]=copy.deepcopy(e[75])
            with self.subTest(end=index),self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)

    def test_current79_metadata_rejects_etag_counter_scope_and_source_changes(self):
        summary,planned,ends=self.population()
        for key in CHECK.ETAG_SUMMARY:
            s,p,e=copy.deepcopy((summary,planned,ends));s.pop(key)
            with self.subTest(missing=key),self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)
            s,p,e=copy.deepcopy((summary,planned,ends));s[key]=True if type(s[key]) is int else s[key]+' changed'
            with self.subTest(changed=key),self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)
        for index in (76,77,78):
            for mutation in ('missing','substituted','extra'):
                s,p,e=copy.deepcopy((summary,planned,ends))
                if mutation!='extra':e[index]['source_set'].pop('ports/net/http/etag.gooo')
                if mutation!='missing':e[index]['source_set']['ports/net/http.gooo']=None
                with self.subTest(source=index,mutation=mutation),self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)
        s,p,e=copy.deepcopy((summary,planned,ends));e[78]['source_set'].pop('ports/net/httpguts/headers.gooo')
        with self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)

    def test_appended_ifmatch_cannot_borrow_old79_acceptance_or_wrong_abi(self):
        summary,planned,ends=self.population()
        self.assertEqual(sum(r['cases'] for r in summary['runs'][:79]),173582)
        self.assertEqual(sum(r['batches'] for r in summary['runs'][:79]),2247)
        for key in CHECK.IF_MATCH_SUMMARY:
            s,p,e=copy.deepcopy((summary,planned,ends));s.pop(key)
            with self.subTest(missing=key),self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)
            s,p,e=copy.deepcopy((summary,planned,ends));s[key]=True
            with self.subTest(changed=key),self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)
        for key,value in [('cases',283),('batches',4),('passed',True),('activity','ETagStrongMatch')]:
            s,p,e=copy.deepcopy((summary,planned,ends));s['runs'][79][key]=value
            with self.subTest(key=key),self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)
        s,p,e=copy.deepcopy((summary,planned,ends));s['runs']=s['runs'][:79];p=p[:79];e=e[:79]
        with self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)
        s,p,e=copy.deepcopy((summary,planned,ends));e[79]['source_set']['ports/net/http/etag.gooo']=None
        with self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)

    def test_current82_keeps_exact_old80_prefix_and_two_new_entries(self):
        summary,planned,ends=self.population()
        CHECK.current_population(summary,planned,ends)
        self.assertEqual(len(summary['runs']),82)
        self.assertEqual(sum(r['cases'] for r in summary['runs'][:80]),173866)
        self.assertEqual(sum(r['batches'] for r in summary['runs'][:80]),2252)
        self.assertEqual(sum(r['cases'] for r in summary['runs']),175649)
        self.assertEqual(sum(r['batches'] for r in summary['runs']),2305)
        self.assertEqual(summary['runs'][80]['cases'],1694)
        self.assertEqual(summary['runs'][81]['cases'],89)
        # Old80 acceptance cannot become current by changing its outer totals.
        for forged_totals in (False,True):
            s,p,e=copy.deepcopy((summary,planned,ends));s['runs']=s['runs'][:80];p=p[:80];e=e[:80]
            if not forged_totals:s['cases']=s['passed']=173866
            with self.subTest(forged_totals=forged_totals),self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)
        for key,value in [('cases',173866),('passed',173866),('cases',True),('passed',175649.0)]:
            s,p,e=copy.deepcopy((summary,planned,ends));s[key]=value
            with self.subTest(rollback=key,value=value),self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)
        # Shift counts across the old80/new boundary while preserving full totals.
        for key in ('cases','batches'):
            s,p,e=copy.deepcopy((summary,planned,ends));s['runs'][0][key]-=1;s['runs'][80][key]+=1
            with self.subTest(prefix_transfer=key),self.assertRaisesRegex(RuntimeError,'complete80 prefix'):CHECK.current_population(s,p,e)

    def test_current82_rejects_appended_counts_order_and_closure_drift(self):
        summary,planned,ends=self.population()
        for index,key,value in [
            (80,'package','go/other'),(80,'activity','Index'),(80,'cases',1693),(80,'passed',1693),
            (80,'batches',49),(80,'replay_equal',False),(80,'passed',True),
            (81,'package','app/literal-text-report'),(81,'activity','LastIndex'),(81,'cases',88),
            (81,'passed',88),(81,'batches',2),(81,'replay_equal',False),(81,'passed',89.0)]:
            s,p,e=copy.deepcopy((summary,planned,ends));s['runs'][index][key]=value
            with self.subTest(index=index,key=key),self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)
        for index in (80,81):
            s,p,e=copy.deepcopy((summary,planned,ends));p[index]['index']=79
            with self.subTest(planned=index),self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)
            s,p,e=copy.deepcopy((summary,planned,ends));e[index]=copy.deepcopy(e[79])
            with self.subTest(end=index),self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)
            for mutation in ('missing','substituted','extra'):
                s,p,e=copy.deepcopy((summary,planned,ends))
                if mutation!='extra':e[index]['source_set'].pop('ports/strings/last_index.gooo')
                if mutation!='missing':e[index]['source_set']['ports/strings/cut.gooo']=None
                with self.subTest(source=index,mutation=mutation),self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)
        for key in ('runs','planned','ends'):
            s,p,e=copy.deepcopy((summary,planned,ends));items=s['runs'] if key=='runs' else p if key=='planned' else e
            items[80],items[81]=items[81],items[80]
            # Completion events may arrive out of order, unlike planned/runs.
            if key=='ends':CHECK.current_population(s,p,e)
            else:
                with self.subTest(swapped=key),self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)

    def test_current82_rejects_missing_or_changed_last_index_summary(self):
        summary,planned,ends=self.population()
        for key,value in CHECK.LAST_INDEX_SUMMARY.items():
            s,p,e=copy.deepcopy((summary,planned,ends));s.pop(key)
            with self.subTest(missing=key),self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)
            for replacement in ([True,value-1,float(value)] if type(value) is int else [value+' changed','NOT_CHECKED']):
                s,p,e=copy.deepcopy((summary,planned,ends));s[key]=replacement
                with self.subTest(changed=key,value=replacement),self.assertRaises(RuntimeError):CHECK.current_population(s,p,e)

    def verification(self):
        definitions,records=CHECK.contracts([(pkg,name,(ROOT/name).read_text()) for pkg,name in CHECK.PACKAGES])
        rows=[]
        for i in range(50):
            inputs={CHECK.ENTRY+'.input'+str(j):v for j,v in enumerate(['x-request-id','candidate','2',{}])}
            rows.append({'inputs':inputs,'expected':{CHECK.ENTRY:{'candidate_name':'independent-go-'+str(i)}},'actual':{'candidate_name':'do-not-inherit-gooo-actual'}})
        return {'records':records,'typed_plan':{'activities':[{'go_function':'GoooComposedActivity0'}]},'source':'package main\n// unchanged fresh generated bytes\n','rows':rows,'case_names':['case-'+str(i) for i in range(50)]}
    def test_render_uses_fresh_expected_and_preserves_generated_bytes(self):
        verification=self.verification()
        with tempfile.TemporaryDirectory() as directory:
            rendered=pathlib.Path(directory).resolve()/'rendered';result=CHECK.render(ROOT,verification,rendered)
            self.assertEqual((rendered/'assessor_generated.go').read_bytes(),verification['source'].encode())
            fixture=CHECK.load((rendered/'testdata/fresh-50.json').read_bytes())
            self.assertEqual(len(fixture['rows']),50)
            self.assertEqual(fixture['rows'][0]['expected'],{'candidate_name':'independent-go-0'})
            self.assertNotIn('do-not-inherit-gooo-actual',(rendered/'testdata/fresh-50.json').read_text())
            bindings=(rendered/'bindings_test.go').read_text()
            self.assertIn(CHECK.sha(verification['source'].encode()),bindings)
            self.assertIn(CHECK.sha((rendered/'testdata/fresh-50.json').read_bytes()),bindings)
            self.assertIn('no Gooo actual or private fixture inheritance',result['fixture']['origin'])
            CHECK.unchanged(result['rendered_pins'])
            (rendered/'assessor_generated.go').write_text('package main\n// substituted policy\n')
            with self.assertRaisesRegex(RuntimeError,'assessor_generated.go'):CHECK.unchanged(result['rendered_pins'])
    def test_unexpanded_or_changed_template_refuses(self):
        verification=self.verification()
        with tempfile.TemporaryDirectory() as directory:
            root=pathlib.Path(directory).resolve();example=root/CHECK.EXAMPLE;example.mkdir(parents=True)
            for name in CHECK.TEMPLATES:(example/name).write_text((ROOT/CHECK.EXAMPLE/name).read_text()+'\n@@UNRECOGNIZED@@\n')
            with self.assertRaisesRegex(RuntimeError,'unexpanded bridge template'):CHECK.render(root,verification,root/'rendered')
    def test_failed_process_snapshot_still_stops_known_live_child(self):
        class Child:
            pid=424242
            returncode=None
            waited=False
            def poll(self):return self.returncode
            def wait(self,timeout):self.waited=True;self.returncode=-9
        child=Child()
        with tempfile.TemporaryDirectory() as directory:
            out=pathlib.Path(directory).resolve();work=out/'owned';work.mkdir()
            runner=CHECK.OwnedCommands(out,work,out,{},{} )
            with mock.patch.object(CHECK.subprocess,'Popen',return_value=child),mock.patch.object(CHECK,'processes',side_effect=TimeoutError('snapshot failed')),mock.patch.object(CHECK.os,'killpg') as kill:
                with self.assertRaisesRegex(RuntimeError,'command failure'):runner.call('guard',['mocked-no-process'],1)
            kill.assert_called_once_with(child.pid,CHECK.signal.SIGKILL)
            self.assertTrue(child.waited)
            self.assertFalse(runner.closed)
            self.assertTrue(work.exists())

    def test_read_verification_has_no_process_or_filesystem_mutation(self):
        # Enforce the shared reader boundary independently of the host execution wrapper.
        import ast
        tree=ast.parse((ROOT/'tools/check-header-addition-host.py').read_text())
        readers=[node for node in tree.body if isinstance(node,ast.FunctionDef) and node.name in ('read_verification','validate_timing_sources','read_addition_evidence','read_shard_verification')]
        self.assertEqual(len(readers),4)
        forbidden={'mkdir','write_bytes','write_text','Popen','run','check_output','save','render','call','unlink','rmtree'}
        for reader in readers:
            for node in ast.walk(reader):
                if isinstance(node,ast.Call):
                    name=node.func.attr if isinstance(node.func,ast.Attribute) else node.func.id if isinstance(node.func,ast.Name) else ''
                    self.assertNotIn(name,forbidden)


    def test_owned_regular_files_count_exact_logical_bytes(self):
        with tempfile.TemporaryDirectory() as directory:
            work=pathlib.Path(directory);(work/'empty').write_bytes(b'');(work/'three').write_bytes(b'abc')
            nested=work/'nested';nested.mkdir();(nested/'seven').write_bytes(b'1234567')
            self.assertEqual(CHECK.owned_bytes(work),10)

    def test_owned_empty_directory_counts_zero(self):
        with tempfile.TemporaryDirectory() as directory:self.assertEqual(CHECK.owned_bytes(pathlib.Path(directory)),0)

    def test_owned_directory_removed_during_scandir_is_ignored(self):
        with tempfile.TemporaryDirectory() as directory:
            work=pathlib.Path(directory);(work/'keep').write_bytes(b'abc')
            nested=work/'vanishes';nested.mkdir();victim=nested/'file';victim.write_bytes(b'gone')
            original=CHECK.os.scandir;removed=[]
            def scan(path):
                if pathlib.Path(path)==nested and not removed:
                    victim.unlink();nested.rmdir();removed.append(True)
                return original(path)
            with mock.patch.object(CHECK.os,'scandir',side_effect=scan):self.assertEqual(CHECK.owned_bytes(work),3)
            self.assertEqual(removed,[True])

    def test_owned_file_removed_before_lstat_is_ignored(self):
        with tempfile.TemporaryDirectory() as directory:
            work=pathlib.Path(directory);(work/'keep').write_bytes(b'abc');victim=work/'vanishes';victim.write_bytes(b'gone')
            original=pathlib.Path.lstat;removed=[]
            def lstat(path,*args,**kwargs):
                if path==victim and not removed:victim.unlink();removed.append(True)
                return original(path,*args,**kwargs)
            with mock.patch.object(pathlib.Path,'lstat',lstat):self.assertEqual(CHECK.owned_bytes(work),3)
            self.assertEqual(removed,[True])

    def test_owned_scandir_permission_error_propagates(self):
        with tempfile.TemporaryDirectory() as directory:
            work=pathlib.Path(directory);nested=work/'private';nested.mkdir();original=CHECK.os.scandir
            failure=PermissionError(13,'controlled directory permission refusal',str(nested))
            def scan(path):
                if pathlib.Path(path)==nested:raise failure
                return original(path)
            with mock.patch.object(CHECK.os,'scandir',side_effect=scan),self.assertRaises(PermissionError) as caught:CHECK.owned_bytes(work)
            self.assertIs(caught.exception,failure)

    def test_owned_lstat_permission_error_propagates(self):
        with tempfile.TemporaryDirectory() as directory:
            work=pathlib.Path(directory);victim=work/'private';victim.write_bytes(b'data');original=pathlib.Path.lstat
            failure=PermissionError(13,'controlled file permission refusal',str(victim))
            def lstat(path,*args,**kwargs):
                if path==victim:raise failure
                return original(path,*args,**kwargs)
            with mock.patch.object(pathlib.Path,'lstat',lstat),self.assertRaises(PermissionError) as caught:CHECK.owned_bytes(work)
            self.assertIs(caught.exception,failure)

    def test_owned_other_scandir_error_propagates(self):
        with tempfile.TemporaryDirectory() as directory:
            failure=OSError(5,'controlled directory I/O refusal',directory)
            with mock.patch.object(CHECK.os,'scandir',side_effect=failure),self.assertRaises(OSError) as caught:CHECK.owned_bytes(pathlib.Path(directory))
            self.assertIs(caught.exception,failure)

    def test_owned_symlinks_are_not_followed_or_counted(self):
        with tempfile.TemporaryDirectory() as directory:
            root=pathlib.Path(directory);work=root/'work';work.mkdir();outside=root/'outside';outside.mkdir()
            (outside/'large').write_bytes(b'x'*8192);(work/'keep').write_bytes(b'abc')
            (work/'file-link').symlink_to(outside/'large');(work/'directory-link').symlink_to(outside,target_is_directory=True)
            (work/'broken-link').symlink_to(root/'absent');(work/'cycle').symlink_to(work,target_is_directory=True)
            self.assertEqual(CHECK.owned_bytes(work),3)

    def test_owned_guard_survives_a_disappearing_enumerated_directory(self):
        # Deterministic scheduling point in the real filesystem walk; no child is started.
        class Child:
            pid=424242
            returncode=None
            waited=False
            polls=0
            def poll(self):
                self.polls+=1
                if self.polls>=3:self.returncode=0
                return self.returncode
            def wait(self,timeout):self.waited=True;self.returncode=0
        child=Child()
        with tempfile.TemporaryDirectory() as directory:
            out=pathlib.Path(directory);work=out/'owned';work.mkdir();(work/'keep').write_bytes(b'abc')
            nested=work/'vanishes';nested.mkdir();victim=nested/'file';victim.write_bytes(b'gone')
            original=CHECK.os.scandir;removed=[]
            def scan(path):
                if pathlib.Path(path)==nested and not removed:
                    victim.unlink();nested.rmdir();removed.append(True)
                return original(path)
            runner=CHECK.OwnedCommands(out,work,out,{},{} )
            with mock.patch.object(CHECK.subprocess,'Popen',return_value=child),mock.patch.object(CHECK,'processes',return_value={}),mock.patch.object(CHECK.os,'scandir',side_effect=scan),mock.patch.object(CHECK.os,'killpg') as kill,mock.patch.object(CHECK.time,'sleep'):
                self.assertEqual(runner.call('guard',['mocked-no-process'],1),b'')
            self.assertEqual(removed,[True]);self.assertTrue(child.waited);self.assertTrue(runner.closed);kill.assert_not_called()
            self.assertEqual(runner.observations[0]['exit_code'],0);self.assertIsNone(runner.observations[0]['failure'])

    def test_owned_guard_still_refuses_over_one_gib(self):
        # A sparse file supplies a real st_size boundary without allocating1GiB data.
        class Child:
            pid=424242
            returncode=None
            waited=False
            def poll(self):return self.returncode
            def wait(self,timeout):self.waited=True;self.returncode=-9
        child=Child()
        with tempfile.TemporaryDirectory() as directory:
            out=pathlib.Path(directory);work=out/'owned';work.mkdir()
            with (work/'large').open('wb') as stream:stream.truncate((1<<30)+1)
            self.assertEqual(CHECK.owned_bytes(work),(1<<30)+1)
            runner=CHECK.OwnedCommands(out,work,out,{},{} )
            with mock.patch.object(CHECK.subprocess,'Popen',return_value=child),mock.patch.object(CHECK,'processes',return_value={}),mock.patch.object(CHECK.os,'killpg') as kill:
                with self.assertRaisesRegex(RuntimeError,'owned work1GiB bound'):runner.call('guard',['mocked-no-process'],1)
            kill.assert_called_once_with(child.pid,CHECK.signal.SIGKILL);self.assertTrue(child.waited)

class EvidenceModesTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        spec=importlib.util.spec_from_file_location('header_addition_inputs_for_modes',ROOT/'tools/check-header-addition-inputs.py')
        cls.inputs=importlib.util.module_from_spec(spec);spec.loader.exec_module(cls.inputs)

    def arguments(self,module,out):
        args=['--root',str(ROOT),'--go','/missing/go','--out',str(out)]
        if module is self.inputs:args+=['--compiler','/missing/compiler','--input-command','/missing/helper']
        return args

    def test_legacy_pair_and_explicit_shard_modes(self):
        for module in (CHECK,self.inputs):
            with self.subTest(module=module.__name__):
                args=self.arguments(module,pathlib.Path('/missing/out'))
                legacy=module.parse_args(args+['--verification-out','/legacy','--timing','/legacy/timing.jsonl'])
                self.assertEqual(legacy.verification_out,'/legacy');self.assertIsNone(legacy.shard_native_index)
                shard=module.parse_args(args+['--shard-native-index','/shard/index.json'])
                self.assertEqual(shard.shard_native_index,'/shard/index.json');self.assertIsNone(shard.verification_out);self.assertIsNone(shard.timing)

    def test_invalid_or_mixed_mode_refuses_before_output_or_process(self):
        invalid=[[],['--timing','/timing'],['--verification-out','/legacy'],
                 ['--verification-out','','--timing','/timing'],
                 ['--verification-out','/legacy','--timing',''],
                 ['--shard-native-index',''],
                 ['--shard-native-index','/index','--timing','/timing'],
                 ['--shard-native-index','/index','--verification-out','/legacy','--timing','/timing']]
        for module in (CHECK,self.inputs):
            for suffix in invalid:
                with self.subTest(module=module.__name__,suffix=suffix),tempfile.TemporaryDirectory() as directory:
                    out=pathlib.Path(directory).resolve()/'must-not-be-created'
                    with mock.patch.object(module.sys,'argv',['checker',*self.arguments(module,out),*suffix]),mock.patch.object(module.subprocess,'Popen',side_effect=AssertionError('no processes before mode validation')),contextlib.redirect_stderr(io.StringIO()):
                        with self.assertRaises(SystemExit) as failure:module.main()
                    self.assertEqual(failure.exception.code,2);self.assertFalse(out.exists())

    def test_shard_provider_is_explicit_and_has_no_legacy_fallback(self):
        with tempfile.TemporaryDirectory() as directory:
            root=pathlib.Path(directory).resolve();(root/'tools').mkdir();index=root/'index.json';index.write_bytes(b'{}')
            provider=root/'tools/native-shard-reference.py'
            with mock.patch.object(CHECK,'read_verification',side_effect=AssertionError('legacy fallback forbidden')):
                with self.assertRaisesRegex(RuntimeError,'regular input required'):CHECK.read_shard_verification(root,index)
                provider.write_text('def read_reference(root,index_path):\n    raise RuntimeError("provider refused fixture")\n')
                with self.assertRaisesRegex(RuntimeError,'provider refused fixture'):CHECK.read_shard_verification(root,index)
                provider.write_text('def read_reference(root,index_path):\n    return {"unexpected": True}\n')
                with self.assertRaisesRegex(RuntimeError,'exact shard reference provider API'):CHECK.read_shard_verification(root,index)

    def test_shard_wrapper_passes_only_provider_api_to_common_reader(self):
        with tempfile.TemporaryDirectory() as directory:
            root=pathlib.Path(directory).resolve();(root/'tools').mkdir();index=root/'index.json';index.write_bytes(b'{}')
            provider=root/'tools/native-shard-reference.py'
            provider.write_text('import hashlib\ndef read_reference(root,index_path):\n    p=root/"tools/native-shard-reference.py"\n    raw=p.read_bytes()\n    return {"pins":{str(p):{"bytes":len(raw),"sha256":hashlib.sha256(raw).hexdigest()}},"source_set":{},"source_set_sha256":"source-set-fixture","batches":[],"go_version":"go-fixture","go_binary":{},"suite_count":82}\n')
            with mock.patch.object(CHECK,'read_addition_evidence',return_value={'boundary_fixture':True}) as common:
                result=CHECK.read_shard_verification(root,index)
            self.assertEqual(result,{'boundary_fixture':True,'evidence_mode':'experimental-shard-native-reference'})
            args=common.call_args.args
            self.assertEqual(args[0],root);self.assertTrue(callable(args[1]));self.assertIn(str(index),args[2])
            self.assertEqual(args[3:],({},'source-set-fixture',[],'go-fixture',{},82))


class AdditionEvidenceBoundaryTests(unittest.TestCase):
    """Selected reader-boundary fixtures, not fabricated native acceptance.

    The deep ABI receipt validator is mocked here so that these tests isolate
    local/producer paths and unchanged independent case-document consumption.
    The production common reader always calls that original validator14 times.
    """
    def fixture(self,directory):
        base=pathlib.Path(directory).resolve();manifest=(ROOT/'diagnostic-header-addition.workspace.json').read_bytes()
        corpus=CHECK.load((ROOT/'examples/diagnostic-header-addition/corpus.json').read_bytes())
        source_set={name:{'Package':pkg,'Digest':CHECK.digest(CHECK.go_json((ROOT/name).read_text()))} for pkg,name in sorted(CHECK.PACKAGES,key=lambda x:x[1])}
        rows=[];batches=[];originals=[];replays=[]
        for i,spec in enumerate(corpus['cases']):
            inp=spec['input'];expected=copy.deepcopy(spec['expected'])
            # Additional oracle-only fixture data must survive unchanged and
            # must never be filled from a producer's actual delivery values.
            expected['independent_fixture_sentinel']='oracle-'+str(i)
            rows.append({'inputs':{CHECK.ENTRY+'.input'+str(j):v for j,v in enumerate([inp['name'],inp['value'],inp['declared_length'],inp['facts']])},'expected':{CHECK.ENTRY:expected}})
        for index in range(7):
            batchdir=base/('local-blob-%03d'%index);batchdir.mkdir()
            workspace=batchdir/'gooo.workspace.json';workspace.write_bytes(manifest)
            for _,name in CHECK.PACKAGES:
                target=batchdir/name;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes((ROOT/name).read_bytes())
            start,end=index*8,min(index*8+8,50)
            cases=json.dumps({'schema':'gooo/body-composition-cases/v1','cases':rows[start:end]},ensure_ascii=False,separators=(',',':')).encode()
            (batchdir/'cases.json').write_bytes(cases)
            original=('original fixture %d\n'%index).encode();replay=('saved replay fixture %d\n'%index).encode()
            (batchdir/'execution.json').write_bytes(original);(batchdir/'replay.json').write_bytes(replay)
            originals.append(original);replays.append(replay)
            batches.append({'batch_index':index,'start':start,'end':end,'manifest_digest':CHECK.digest(manifest),'cases_digest':CHECK.digest(cases),'original_receipt_sha256':CHECK.digest(original),'workspace':workspace,'producer_manifest_path':'/original/runner/evidence/%03d/gooo.workspace.json'%index})
        return {'batches':batches,'source_set':source_set,'source_set_sha256':CHECK.digest(json.dumps(source_set,ensure_ascii=False,separators=(',',':')).encode()),'rows':rows,'originals':originals,'replays':replays}

    def read(self,fixture):
        pins={};read=lambda p:CHECK.frozen_read(p,pins)
        return CHECK.read_addition_evidence(ROOT,read,pins,fixture['source_set'],fixture['source_set_sha256'],fixture['batches'],'go version go1.27.3 linux/amd64',{'bytes':1,'sha256':'a'*64},82)

    def observed(self,*args):
        return {'source':'package main\n// boundary fixture only\n','plan':{'fixture':'independent-current-plan'},'tool_selection':'explicit'}

    def rewrite_cases(self,fixture,mutate):
        batch=fixture['batches'][0];path=batch['workspace'].parent/'cases.json';doc=CHECK.load(path.read_bytes());mutate(doc['cases'][0])
        raw=json.dumps(doc,ensure_ascii=False,separators=(',',':')).encode();path.write_bytes(raw);batch['cases_digest']=CHECK.digest(raw)

    def test_local_blobs_and_original_manifest_strings_remain_distinct(self):
        with tempfile.TemporaryDirectory() as directory:
            fixture=self.fixture(directory)
            with mock.patch.object(CHECK,'receipt',side_effect=self.observed) as validator:result=self.read(fixture)
            self.assertEqual(len(validator.call_args_list),14);self.assertTrue(CHECK.equal(result['rows'],fixture['rows']))
            self.assertEqual(len(result['evidence']),14);self.assertEqual(result['fresh_addition_cases'],50)
            for index,batch in enumerate(fixture['batches']):
                execute=validator.call_args_list[2*index].args;replay=validator.call_args_list[2*index+1].args
                self.assertNotEqual(str(batch['workspace']),batch['producer_manifest_path'])
                self.assertEqual(execute[9],batch['producer_manifest_path']);self.assertEqual(replay[9],batch['producer_manifest_path'])
                self.assertIsNone(execute[10]);self.assertEqual(replay[10],fixture['originals'][index])
                self.assertEqual(execute[0],fixture['originals'][index]);self.assertEqual(replay[0],fixture['replays'][index])
                self.assertEqual(execute[2],(batch['workspace'].parent/'cases.json').read_bytes())
                self.assertIn(str(batch['workspace']),result['pins']);self.assertNotIn(batch['producer_manifest_path'],result['pins'])
                self.assertEqual((batch['workspace'].parent/'execution.json').read_bytes(),fixture['originals'][index])

    def test_independent_original_inputs_and_partial_expectations_cannot_drift(self):
        changes=[('inputs',lambda row:row['inputs'].__setitem__(CHECK.ENTRY+'.input0','forged')),
                 ('expectation',lambda row:row['expected'][CHECK.ENTRY].__setitem__('eligible',not row['expected'][CHECK.ENTRY]['eligible']))]
        for label,change in changes:
            with self.subTest(change=label),tempfile.TemporaryDirectory() as directory:
                fixture=self.fixture(directory);self.rewrite_cases(fixture,change)
                with mock.patch.object(CHECK,'receipt',side_effect=AssertionError('reject before ABI validation')),self.assertRaisesRegex(RuntimeError,'fresh case inputs|partial source specification'):self.read(fixture)

    def test_copied_source_case_digest_raw_parent_and_batch_identity_refuse(self):
        for label in ('source','case-bytes','parent','batch-index','producer-path-type','local-path-type'):
            with self.subTest(change=label),tempfile.TemporaryDirectory() as directory:
                fixture=self.fixture(directory);batch=fixture['batches'][0];batchdir=batch['workspace'].parent
                if label=='source':(batchdir/CHECK.PACKAGES[0][1]).write_bytes(b'changed source')
                elif label=='case-bytes':(batchdir/'cases.json').write_bytes((batchdir/'cases.json').read_bytes()+b' ')
                elif label=='parent':(batchdir/'execution.json').write_bytes(b'changed original raw receipt')
                elif label=='batch-index':batch['batch_index']=True
                elif label=='producer-path-type':batch['producer_manifest_path']=batch['workspace']
                else:batch['workspace']=str(batch['workspace'])
                with mock.patch.object(CHECK,'receipt',side_effect=AssertionError('must refuse before receipt')),self.assertRaises(RuntimeError):self.read(fixture)

    def test_production_receipt_compares_producer_manifest_string(self):
        # Execute the real early binding guard, before any synthetic result body.
        raw=json.dumps({'schema':'gooo/workspace-body-execution-receipt/v1','decision':'PASS','manifest':'/producer/gooo.workspace.json','manifest_digest':CHECK.digest(b'{}'),'cases_digest':CHECK.digest(b'{}')}).encode()
        with self.assertRaisesRegex(RuntimeError,'exact manifest/case bytes'):
            CHECK.receipt(raw,[],b'{}',b'{}',[],'go version go1.27.3 linux/amd64','a'*64,{CHECK.ENTRY:{}},{},'/local/blob/gooo.workspace.json')
        # With the original producer path this guard passes to the next required
        # field; this is a guard test, not a complete native receipt fixture.
        with self.assertRaises(KeyError) as failure:
            CHECK.receipt(raw,[],b'{}',b'{}',[],'go version go1.27.3 linux/amd64','a'*64,{CHECK.ENTRY:{}},{},'/producer/gooo.workspace.json')
        self.assertEqual(failure.exception.args,('result',))


class PinScanCancellationTests(unittest.TestCase):
    def test_clock_cancellation_propagates_before_or_after_validation(self):
        for kind in (KeyboardInterrupt,SystemExit,RuntimeError):
            for after in (False,True):
                observer=CHECK.PinScanObservations();failure=kind('cancel')
                if kind is RuntimeError:observer.cancellation=failure
                operation=mock.Mock(return_value='validated')
                clock=[1,failure] if after else [failure]
                with self.subTest(kind=kind,after=after),mock.patch.object(CHECK.time,'monotonic_ns',side_effect=clock),self.assertRaises(kind) as caught:
                    observer.observe('scan','pre',operation)
                self.assertIs(caught.exception,failure);self.assertEqual(operation.call_count,int(after))

    def test_metadata_collection_does_not_swallow_marked_signal(self):
        observer=CHECK.PinScanObservations();failure=RuntimeError('received signal 15');observer.cancellation=failure
        pins=mock.MagicMock();pins.__len__.side_effect=failure;operation=mock.Mock()
        with self.assertRaises(RuntimeError) as caught:observer.observe('scan','pre',operation,pins)
        self.assertIs(caught.exception,failure);operation.assert_not_called()

    def test_ordinary_clock_failure_is_optional_and_first_scan_error_wins(self):
        for secondary in (OSError('clock'),RuntimeError('ordinary clock'),KeyboardInterrupt('later cancel'),SystemExit(9)):
            observer=CHECK.PinScanObservations();primary=RuntimeError('original source drift')
            with self.subTest(secondary=secondary),mock.patch.object(CHECK.time,'monotonic_ns',side_effect=[1,secondary]),self.assertRaises(RuntimeError) as caught:
                observer.observe('scan','post',mock.Mock(side_effect=primary))
            self.assertIs(caught.exception,primary)
        observer=CHECK.PinScanObservations()
        with mock.patch.object(CHECK.time,'monotonic_ns',side_effect=[RuntimeError('clock'),OSError('clock')]):
            self.assertEqual(observer.observe('scan','pre',lambda:'validated'),'validated')

    def test_successful_main_propagates_logger_cancellation(self):
        for kind in (KeyboardInterrupt,SystemExit,RuntimeError):
            failure=kind('cancel');seen={}
            def completed(observer):observer.output=pathlib.Path('/unused');seen['observer']=observer;return 0
            def writer(*_):
                if kind is RuntimeError:seen['observer'].cancellation=failure
                raise failure
            with self.subTest(kind=kind),mock.patch.object(CHECK,'_main',side_effect=completed),mock.patch.object(CHECK,'save',side_effect=writer),self.assertRaises(kind) as caught:CHECK.main()
            self.assertIs(caught.exception,failure)

    def test_prior_main_exception_or_failed_exit_precedes_logger_error(self):
        for secondary in (OSError('write'),KeyboardInterrupt('later cancel'),SystemExit(9)):
            primary=RuntimeError('original validation');seen={}
            def refused(observer):observer.output=pathlib.Path('/unused');seen['observer']=observer;raise primary
            with self.subTest(secondary=secondary),mock.patch.object(CHECK,'_main',side_effect=refused),mock.patch.object(CHECK,'save',side_effect=secondary),self.assertRaises(RuntimeError) as caught:CHECK.main()
            self.assertIs(caught.exception,primary)
            def failed(observer):observer.output=pathlib.Path('/unused');return 1
            with mock.patch.object(CHECK,'_main',side_effect=failed),mock.patch.object(CHECK,'save',side_effect=secondary):self.assertEqual(CHECK.main(),1)

    def test_installed_signal_handler_marks_clock_cancellation(self):
        for number in (CHECK.signal.SIGTERM,CHECK.signal.SIGINT):
            for after in (False,True):
                with self.subTest(number=number,after=after),tempfile.TemporaryDirectory() as directory:
                    root=pathlib.Path(directory).resolve();args=mock.Mock(root=str(root),go=str(root/'go'),out='out',shard_native_index='index',verification_out=None,timing=None);calls=[]
                    def clock():
                        calls.append('clock')
                        if len(calls)==(2 if after else 1):CHECK.signal.getsignal(number)(number,None)
                        return 1
                    with mock.patch.object(CHECK,'parse_args',return_value=args),mock.patch.object(CHECK,'read_shard_verification',return_value={'pins':{}}),mock.patch.object(CHECK.time,'monotonic_ns',side_effect=clock),contextlib.redirect_stdout(io.StringIO()):
                        self.assertEqual(CHECK.main(),1)
                    summary=json.loads((root/'out/summary.json').read_text())
                    self.assertEqual(summary['status'],'FIRST_FAILURE_PRESERVED');self.assertEqual(summary['failure'],'received signal '+str(number))
        for filename in ('check-header-addition-host.py','check-header-addition-inputs.py'):
            tree=ast.parse((ROOT/'tools'/filename).read_text());main=next(n for n in tree.body if isinstance(n,ast.FunctionDef) and n.name=='_main')
            handler=next(n for n in main.body if isinstance(n,ast.FunctionDef) and n.name=='interrupted')
            for number in (CHECK.signal.SIGTERM,CHECK.signal.SIGINT):
                for after in (False,True):
                    observer=CHECK.PinScanObservations();scope={'pin_scans':observer,'scan_state':{'observer':observer}}
                    exec(compile(ast.Module(body=[handler],type_ignores=[]),'<actual signal handler>','exec'),scope)
                    ticks=[];operation=mock.Mock(return_value='validated')
                    def clock():
                        ticks.append(1)
                        if len(ticks)==(2 if after else 1):scope['interrupted'](number,None)
                        return 1
                    with self.subTest(handler=filename,number=number,after=after),mock.patch.object(CHECK.time,'monotonic_ns',side_effect=clock),self.assertRaises(RuntimeError) as caught:
                        observer.observe('scan','pre',operation)
                    self.assertIs(caught.exception,observer.cancellation);self.assertEqual(operation.call_count,int(after))
            if filename.endswith('inputs.py'):
                scope={'scan_state':{}};exec(compile(ast.Module(body=[handler],type_ignores=[]),'<input handler before observer>','exec'),scope)
                with self.assertRaisesRegex(RuntimeError,'received signal 15'):scope['interrupted'](int(CHECK.signal.SIGTERM),None)
                self.assertEqual(scope['scan_state'],{})

    def test_inputs_wrapper_preserves_prior_failure_and_propagates_new_cancel(self):
        spec=importlib.util.spec_from_file_location('header_inputs_cancel',ROOT/'tools/check-header-addition-inputs.py');inputs=importlib.util.module_from_spec(spec);spec.loader.exec_module(inputs)
        for outcome in (0,1,RuntimeError('original input failure')):
            primary=outcome if isinstance(outcome,BaseException) else None;cancel=KeyboardInterrupt('logger cancel')
            def run(state):
                observer=CHECK.PinScanObservations();observer.output=pathlib.Path('/unused');state['observer']=observer
                if primary is not None:raise primary
                return outcome
            with self.subTest(outcome=outcome),mock.patch.object(inputs,'_main',side_effect=run),mock.patch.object(CHECK,'save',side_effect=cancel):
                if outcome==0:
                    with self.assertRaises(KeyboardInterrupt) as caught:inputs.main()
                    self.assertIs(caught.exception,cancel)
                elif primary is not None:
                    with self.assertRaises(RuntimeError) as caught:inputs.main()
                    self.assertIs(caught.exception,primary)
                else:self.assertEqual(inputs.main(),1)


if __name__=='__main__':unittest.main()
