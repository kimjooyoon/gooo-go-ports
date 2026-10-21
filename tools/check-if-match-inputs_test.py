"""Synthetic transport/evidence controls only; no helper, compiler or Go runs."""
import copy
import importlib.util
import json
from pathlib import Path
import unittest

PATH=Path(__file__).with_name('check-if-match-inputs.py')
SPEC=importlib.util.spec_from_file_location('if_match_inputs',PATH)
CHECK=importlib.util.module_from_spec(SPEC);SPEC.loader.exec_module(CHECK)


class IfMatchTransportTests(unittest.TestCase):
    def test_exact35_roster_preserves_independent_wrongtype_unicode_and_boundaries(self):
        rows=list(CHECK.negative_requests())
        self.assertEqual(len(rows),35);self.assertEqual(len({n for n,_ in rows}),35)
        for port,key in enumerate(CHECK.KEYS):
            selected=[(n,r) for n,r in rows if n.startswith('wrong-'+str(port)+'-')]
            self.assertEqual(len(selected),7)
            self.assertEqual([type(json.loads(r)[key]) for _,r in selected],[type(None),bool,bool,int,float,list,dict])
            for kind in ('ascii','multibyte'):
                r=next(r for n,r in rows if n==f'over1024-{port}-{kind}')
                self.assertEqual(len(json.loads(r)[key].encode('utf-8')),1025)
                self.assertEqual(json.loads(r)[CHECK.KEYS[1-port]],'"a"')
            for kind in ('plain','escaped'):
                r=next(r for n,r in rows if n==f'duplicate-{port}-{kind}')
                pairs=json.loads(r,object_pairs_hook=lambda p:p)
                self.assertEqual(sum(k==key for k,v in pairs),2)
        self.assertTrue(any(b'\xff' in r for _,r in rows))
        self.assertTrue(any(b'\\ud800' in r for _,r in rows))
        both=json.loads(dict(rows)['both-over1024'])
        self.assertEqual([len(both[k].encode()) for k in CHECK.KEYS],[1025,1025])

    def test_positive_roster_keeps_empty_crlf_star_suffix_nul_and_unicode(self):
        rows=CHECK.positive_requests();self.assertEqual(len(rows),5)
        self.assertEqual(rows[0],('', '"x"',0))
        self.assertEqual(rows[1],('\r\n"x"\t','"x"',1))
        self.assertEqual(rows[2],('*garbage','\x00bad',1))
        self.assertEqual([len(x.encode()) for x in rows[3][:2]],[1024,1024])
        for selected,current,condition in rows:
            self.assertIs(type(condition),int)
            for text in (selected,current): self.assertLessEqual(len(text.encode('utf-8',errors='strict')),1024)

    def receipt(self):
        workspace=Path('/frozen/if-match.workspace.json');manifest=b'current manifest';trusted=b'current input';image='sha256:source';go='go version go1.27.3 linux/amd64';gosha='tool'
        supplied=('\r\n"x"\t','"x"');condition=1
        d={'activity_id':CHECK.ID,'inputs':[{'port':'input'+str(i),'entity_id':'gooo://std/text','value':s} for i,s in enumerate(supplied)],'actual':{'condition':condition},'actual_fields':[{'id':CHECK.RESULT+'/condition','name':'condition','value':condition}]}
        r={'schema':'gooo/workspace-body-execution-receipt/v1','decision':'OBSERVED','manifest':str(workspace),'manifest_digest':CHECK.H.digest(manifest),'inputs_digest':CHECK.H.digest(trusted),'result':{'schema':'gooo/workspace-body-execution/v1','program':{'entry':{'package_path':CHECK.PACKAGE,'activity':CHECK.ACTIVITY},'workspace_image':{'schema':'gooo/package-runtime-image/v1','init_order':[CHECK.PACKAGE],'entry':{'package_path':CHECK.PACKAGE,'source':CHECK.SOURCE,'activity':CHECK.ACTIVITY,'inputs':['Text','Text'],'output':'IfMatchResult'},'packages':[{'path':CHECK.PACKAGE,'sources':[{'filename':CHECK.SOURCE,'source_digest':image}]}]}},'runtime':{'schema':'gooo/body-composition-runtime/v1','stage':'COMPLETE','finite_total':0,'finite_passed':0,'model_calls':0,'projection_replayed':True,'runtime_replayed':True,'producer_source_sha':CHECK.H.REV,'go_version':go,'go_tool_selection':'explicit','go_tool_sha256':'sha256:'+gosha,'traces':[{'case_index':0,'deliveries':[d]}]}}}
        args=(trusted,supplied,condition,workspace,manifest,image,go,gosha)
        return r,args

    def test_exact_inputonly_typed_receipt_and_raw_saved_parent(self):
        r,args=self.receipt();checked=CHECK.inspect_receipt(CHECK.encode(r),*args)
        self.assertEqual(checked['typed_fields'],1);self.assertEqual(checked['finite_total'],0)
        prior=CHECK.encode(r);r['replayed_from_sha256']=CHECK.H.digest(prior);r['result']['replay']={'schema':'gooo/workspace-body-replay/v1','model_calls':0}
        CHECK.inspect_receipt(CHECK.encode(r),*args,prior=prior)
        with self.assertRaises(RuntimeError): CHECK.inspect_receipt(CHECK.encode(r),*args,prior=prior+b' ')

    def test_labels_cannot_hide_transport_type_source_tool_or_finite_drift(self):
        r,args=self.receipt()
        changes=[lambda r:r.update(decision='PASS'), lambda r:r.update(inputs_digest='sha256:other'),lambda r:r.update(manifest='/other'),lambda r:r['result']['program']['workspace_image']['packages'][0]['sources'][0].update(source_digest='sha256:other'),lambda r:r['result']['program']['workspace_image']['entry'].update(inputs=['Text']),lambda r:r['result']['runtime'].update(finite_total=1),lambda r:r['result']['runtime'].update(finite_passed=False),lambda r:r['result']['runtime'].update(go_tool_selection='path'),lambda r:r['result']['runtime'].update(go_tool_sha256='sha256:other'),lambda r:r['result']['runtime']['traces'][0]['deliveries'][0]['inputs'][0].update(value='"x"'),lambda r:r['result']['runtime']['traces'][0]['deliveries'][0].update(actual={'condition':True}),lambda r:r['result']['runtime']['traces'][0]['deliveries'][0].update(actual={'condition':1.0}),lambda r:r['result']['runtime']['traces'][0]['deliveries'][0].update(expected={'condition':1}),lambda r:r['result']['runtime']['traces'][0]['deliveries'][0]['actual_fields'][0].update(id=CHECK.RESULT+'/wrong')]
        for i,change in enumerate(changes):
            altered=copy.deepcopy(r);change(altered)
            with self.subTest(control=i),self.assertRaises(RuntimeError): CHECK.inspect_receipt(CHECK.encode(altered),*args)
        raw=CHECK.encode(r).replace(b'"decision":"OBSERVED"',b'"decision":"PASS","decision":"OBSERVED"')
        with self.assertRaises(RuntimeError): CHECK.inspect_receipt(raw,*args)


if __name__=='__main__':unittest.main()
