#!/usr/bin/env python3
"""Source/expectation/provenance rejection tests. No Go or native invocation."""
import importlib.util
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
        self.assertEqual(len(roster),283)
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
        catalog=CHECK.load((ROOT/'verification-catalog.json').read_bytes())
        source=(ROOT/'cmd/verify/native_timing.go').read_text()
        token='"cmd/build-demo/api_policy_build.go", '
        self.assertEqual(source.count(token),1)
        for replacement in ('','"cmd/build-demo/substituted.go", '):
            mutant=source.replace(token,replacement,1)
            self.assertNotEqual(mutant,source)
            with tempfile.TemporaryDirectory() as directory:
                root=pathlib.Path(directory);(root/'cmd/verify').mkdir(parents=True)
                (root/'cmd/verify/native_timing.go').write_text(mutant)
                with self.subTest(replacement=replacement),self.assertRaises(RuntimeError):
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
        runs=[{'package':'legacy/'+str(i),'activity':'Old','cases':1,'passed':1,'batches':1,'replay_equal':True} for i in range(74)]
        runs[0].update(cases=171144,passed=171144,batches=2108)
        runs.extend([
            {'package':'go/net/textproto','activity':'CanonicalMIMEHeaderKey','cases':1059,'passed':1059,'batches':22,'replay_equal':True},
            {'package':'app/canonical-header-key','activity':'Report','cases':1059,'passed':1059,'batches':37,'replay_equal':True},
            {'package':'go/net/http/etag','activity':'ScanETag','cases':192,'passed':192,'batches':3,'replay_equal':True},
            {'package':'go/net/http/etag','activity':'ETagStrongMatch','cases':31,'passed':31,'batches':2,'replay_equal':True},
            {'package':'app/strong-validator-facts','activity':'Report','cases':24,'passed':24,'batches':2,'replay_equal':True},
            {'package':'go/net/http/precondition','activity':'IfMatchCondition','cases':284,'passed':284,'batches':5,'replay_equal':True},
            {'package':'go/strings','activity':'LastIndex','cases':1694,'passed':1694,'batches':50,'replay_equal':True},
            {'package':'app/literal-final-marker-report','activity':'Report','cases':89,'passed':89,'batches':3,'replay_equal':True}])
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
        ends=[{'suite_index':i,'source_set':{}} for i in range(82)]
        ends[74]['source_set']={name:None for name in ['ports/net/httpguts/headers.gooo','ports/net/textproto/canonical.gooo']}
        ends[75]['source_set']={name:None for name in ['examples/canonical-header-key/main.gooo','examples/header-field-facts/main.gooo','ports/net/httpguts/headers.gooo','ports/net/textproto/canonical.gooo']}
        for i in (76,77):ends[i]['source_set']={'ports/net/http/etag.gooo':None}
        ends[78]['source_set']={name:None for name in ['examples/strong-validator-facts/main.gooo','ports/net/http/etag.gooo','ports/net/httpguts/headers.gooo']}
        ends[79]['source_set']={'ports/net/http/precondition.gooo':None}
        ends[80]['source_set']={name:None for name in ['ports/strings/search.gooo','ports/strings/last_index.gooo']}
        ends[81]['source_set']={name:None for name in ['ports/strings/search.gooo','ports/strings/literal.gooo','ports/strings/last_index.gooo','examples/literal-final-marker-report/main.gooo']}
        return summary,planned,ends

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
        readers=[node for node in tree.body if isinstance(node,ast.FunctionDef) and node.name in ('read_verification','validate_timing_sources')]
        self.assertEqual(len(readers),2)
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

if __name__=='__main__':unittest.main()
