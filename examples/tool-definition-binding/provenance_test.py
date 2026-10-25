"""Regression controls for the generated file-occurrence graph.

These small graphs test identity/edge handling only, never native execution.
"""
import copy
import importlib.util
import json
import unittest
from pathlib import Path
from unittest.mock import patch

HERE=Path(__file__).resolve().parent

def load(name,filename):
    spec=importlib.util.spec_from_file_location(name,HERE/filename)
    module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
    return module

verifier=load('definition_provenance_verifier','verify.py')
generator=load('definition_provenance_generator','generate.py')

def graph():
    return [
        {'@id':'input','@type':'prov:Entity','path':'input.json','sha256':'same'},
        {'@id':'activity','@type':'prov:Activity','prov:used':[{'@id':'input'}]},
        {'@id':'output','@type':'prov:Entity','path':'copy.json','sha256':'same','prov:wasGeneratedBy':{'@id':'activity'},'prov:wasDerivedFrom':[{'@id':'input'}]},
    ]

class ProvenanceTests(unittest.TestCase):
    def test_copy_with_equal_bytes_and_distinct_occurrences_is_accepted(self):
        verifier.provenance_graph(graph())

    def test_duplicate_ID_reproduces_the_content_only_copy_bug(self):
        value=graph();value[2]['@id']='input'
        with self.assertRaisesRegex(AssertionError,'duplicate provenance ID'):
            verifier.provenance_graph(value)

    def test_self_derivation_is_refused_without_a_duplicate_node(self):
        value=graph();value[2]['prov:wasDerivedFrom']=[{'@id':'output'}]
        with self.assertRaisesRegex(AssertionError,'self-referential'):
            verifier.provenance_graph(value)

    def test_two_entity_cycle_is_refused(self):
        value=graph();value[0]['prov:wasDerivedFrom']=[{'@id':'output'}]
        with self.assertRaisesRegex(AssertionError,'cyclic provenance dependencies'):
            verifier.provenance_graph(value)

    def test_activity_cannot_use_its_own_output(self):
        value=graph();value[1]['prov:used'].append({'@id':'output'})
        with self.assertRaisesRegex(AssertionError,'cyclic provenance dependencies'):
            verifier.provenance_graph(value)

    def test_missing_reference_is_refused(self):
        value=graph();value[2]['prov:wasDerivedFrom']=[{'@id':'missing'}]
        with self.assertRaisesRegex(AssertionError,'unresolved provenance reference'):
            verifier.provenance_graph(value)

    def test_generation_must_reference_an_activity(self):
        value=graph();value[2]['prov:wasGeneratedBy']={'@id':'input'}
        with self.assertRaisesRegex(AssertionError,'wrong provenance reference type'):
            verifier.provenance_graph(value)

    def test_usage_must_reference_an_entity(self):
        value=graph();value.append({'@id':'other','@type':'prov:Activity'})
        value[1]['prov:used']=[{'@id':'other'}]
        with self.assertRaisesRegex(AssertionError,'wrong provenance reference type'):
            verifier.provenance_graph(value)

    def test_entity_cannot_own_activity_usage(self):
        value=graph();value[0]['prov:used']=[]
        with self.assertRaises(AssertionError):verifier.provenance_graph(value)

    def test_order_does_not_change_graph_acceptance(self):
        verifier.provenance_graph(list(reversed(graph())))

    def generated_graph(self):
        index=Path('/virtual/index.json');gofmt=Path('/virtual/bin/gofmt')
        def read(path,*_):return json.dumps({'batches':[]}).encode() if path==index else b'same bytes'
        with patch.object(generator.e,'read',side_effect=read):
            return generator.provenance(index,gofmt,{'copy.json':b'same bytes'},[])

    def test_actual_generator_keeps_equal_content_inputs_and_output_distinct(self):
        value=self.generated_graph();verifier.provenance_graph(value['@graph'])
        entities=[n for n in value['@graph'] if n['@type']=='prov:Entity']
        copied=[n for n in entities if n['sha256']==generator.e.sha(b'same bytes')]
        self.assertGreater(len(copied),2)
        self.assertEqual(len(copied),len({n['@id'] for n in copied}))
        output=next(n for n in entities if n['path']=='copy.json')
        self.assertEqual(output['role'],'output')
        self.assertNotIn({'@id':output['@id']},output['prov:wasDerivedFrom'])

    def test_separate_generations_do_not_alias_file_occurrences(self):
        first=self.generated_graph();second=self.generated_graph()
        self.assertTrue({n['@id'] for n in first['@graph']}.isdisjoint(n['@id'] for n in second['@graph']))
        verifier.provenance_graph(copy.deepcopy(first['@graph'])+second['@graph'])

if __name__=='__main__':unittest.main()
