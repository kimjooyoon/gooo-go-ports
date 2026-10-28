"""Unsupported declarations must never become plausible MCP output contracts."""
import copy
import importlib.util
import json
from pathlib import Path
import unittest

SPEC=importlib.util.spec_from_file_location('record_schema',Path(__file__).with_name('render_mcp_record_schema.py'))
renderer=importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(renderer)


class RecordSchemaTests(unittest.TestCase):
    def setUp(self):
        root=Path(__file__).resolve().parents[1]
        self.origin=json.loads((root/'examples/service-grant-mcp/schemas/snapshot-output.origin.json').read_text())
        self.record=self.origin['selected_record']
        self.receipt={'schema':'gooo/package-workspace-interface-receipt/v1','decision':'PASS',
                      'interface':{'packages':[{'declarations':[copy.deepcopy(self.record)]}]}}

    def test_actual_record_retains_field_identity_order_and_types(self):
        schema,record=renderer.render(self.receipt)
        shape=schema['properties']['decision']
        self.assertEqual(len(shape['required']),32)
        self.assertEqual(shape['required'],[f['name'] for f in record['fields']])
        for field in record['fields']:
            self.assertEqual(shape['properties'][field['name']],
                             {'type':renderer.TYPES[field['type_id']],'$comment':'Gooo field '+field['id']})

    def test_selects_stable_id_not_name_or_position(self):
        expected=renderer.render(self.receipt)[0]
        decoy=copy.deepcopy(self.record);decoy['id']='urn:other-record'
        decoy['fields']=[]
        self.receipt['interface']['packages'].insert(0,{'declarations':[decoy]})
        self.assertEqual(renderer.render(self.receipt)[0],expected)
        self.receipt['interface']['packages'].reverse()
        self.assertEqual(renderer.render(self.receipt)[0],expected)

    def test_duplicate_and_missing_stable_ids_refused(self):
        declarations=self.receipt['interface']['packages'][0]['declarations']
        declarations.append(copy.deepcopy(self.record))
        with self.assertRaises(ValueError):renderer.render(self.receipt)
        declarations.clear()
        with self.assertRaises(ValueError):renderer.render(self.receipt)

    def test_unsupported_fields_do_not_silently_weaken_shape(self):
        changes=[{'presence':'optional'},{'cardinality':'many'},{'type_id':'urn:gooo:type:number'},
                 {'aliases':['other']},{'future_constraint':True},{'name':''},{'id':''}]
        for change in changes:
            with self.subTest(change=change):
                receipt=copy.deepcopy(self.receipt)
                receipt['interface']['packages'][0]['declarations'][0]['fields'][0].update(change)
                with self.assertRaises(ValueError):renderer.render(receipt)

    def test_duplicate_field_names_or_ids_refused(self):
        for key in ['name','id']:
            receipt=copy.deepcopy(self.receipt)
            fields=receipt['interface']['packages'][0]['declarations'][0]['fields']
            fields[1][key]=fields[0][key]
            with self.assertRaises(ValueError):renderer.render(receipt)

    def test_byte_facets_are_not_relabelled_character_limits(self):
        self.receipt['interface']['packages'][0]['declarations'][0]['fields'][0]['max_utf8_bytes']=128
        schema,record=renderer.render(self.receipt)
        self.assertEqual(record['fields'][0]['max_utf8_bytes'],128)
        self.assertNotIn('maxLength',json.dumps(schema))

    def test_partial_receipts_and_other_shapes_refused(self):
        for change in [{'decision':'FAIL_CLOSED'},{'schema':'other'}]:
            receipt=copy.deepcopy(self.receipt);receipt.update(change)
            with self.assertRaises(ValueError):renderer.render(receipt)
        self.receipt['interface']['packages'][0]['declarations'][0]['shape']='opaque'
        with self.assertRaises(ValueError):renderer.render(self.receipt)

    def test_duplicate_or_nonfinite_json_refused(self):
        for raw in ['{"decision":"PASS","decision":"FAIL_CLOSED"}','{"x":NaN}','{"x":Infinity}']:
            with self.assertRaises(ValueError):renderer.strict_json(raw)


if __name__=='__main__':unittest.main()
