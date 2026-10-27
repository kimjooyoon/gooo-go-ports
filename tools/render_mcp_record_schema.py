"""Render the selected required-one scalar Gooo record into MCP output shape.

Not a general Gooo validator. Unknown record forms, aliases and non-scalar
fields are refused. UTF-8 byte limits are retained as source facets, never
reinterpreted as JSON Schema character counts.
"""
import argparse
import hashlib
import json
from pathlib import Path

RECORD='gooo://tool-origin/definition-snapshot/v1/result'
TYPES={'urn:gooo:type:string':'string','urn:gooo:type:integer':'integer','urn:gooo:type:boolean':'boolean'}

def strict_json(raw):
    def unique(pairs):
        result={}
        for key,value in pairs:
            if key in result:raise ValueError('duplicate JSON key: '+key)
            result[key]=value
        return result
    return json.loads(raw,object_pairs_hook=unique,
                      parse_constant=lambda _: (_ for _ in ()).throw(ValueError('nonfinite JSON')))

def render(receipt, record_id=RECORD):
    if receipt.get('schema')!='gooo/package-workspace-interface-receipt/v1' or receipt.get('decision')!='PASS':
        raise ValueError('successful declaration interface required')
    interface=receipt['interface']
    found=[d for p in interface['packages'] for d in p['declarations'] if d.get('id')==record_id]
    if len(found)!=1:raise ValueError('one stable record ID required')
    record=found[0]
    if record.get('kind')!='entity' or record.get('shape')!='record':raise ValueError('selected declaration is not a record')
    properties={};required=[];field_ids=set()
    if not record.get('fields'):raise ValueError('nonempty record required')
    for field in record['fields']:
        if field.get('presence')!='required' or field.get('cardinality')!='one' or field.get('type_id') not in TYPES or field.get('aliases'):
            raise ValueError('only canonical required-one scalar fields are represented')
        if set(field)-{'name','id','type_id','presence','cardinality','aliases','max_utf8_bytes'}:
            raise ValueError('unsupported field facets')
        name=field['name'];identifier=field['id']
        if type(name) is not str or not name or name in properties or type(identifier) is not str or not identifier or identifier in field_ids:
            raise ValueError('duplicate/empty field identity')
        properties[name]={'type':TYPES[field['type_id']],'$comment':'Gooo field '+identifier}
        required.append(name);field_ids.add(identifier)
    decision={'type':'object','properties':properties,'required':required,'additionalProperties':False,
              '$comment':'Shape of Gooo record '+record_id+'. Value/UTF-8 facets remain in the source contract.'}
    result={'$schema':'https://json-schema.org/draft/2020-12/schema','type':'object',
            'properties':{'schema':{'type':'string','const':'gooo/definition-snapshot-report/v1'},
                          'projection':{'type':'object'},'inputs':{'type':'object'},
                          'decision':decision,'bindings':{'type':'object'}},
            'required':['schema','projection','inputs','decision','bindings'],'additionalProperties':False}
    return result,record

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--interface',type=Path,required=True)
    parser.add_argument('--compiler-identity',type=Path,required=True)
    parser.add_argument('--workspace',type=Path,required=True)
    parser.add_argument('--root',type=Path,default=Path(__file__).resolve().parents[1])
    parser.add_argument('--out',type=Path,required=True)
    args=parser.parse_args();raw=args.interface.read_bytes();receipt=strict_json(raw);schema,record=render(receipt)
    identity=strict_json(args.compiler_identity.read_bytes())
    if identity['compiler_source_sha']!='1afb98b955fd8dea74650d3b24e693d0dab9eb5d' or identity['go_version']!='go1.27.3' or identity['source_status']!='CLEAN_VCS':raise ValueError('explicit interface-inspection profile differs')
    digest=lambda data:hashlib.sha256(data).hexdigest()
    generation_raw=(args.root/'examples/recorded-catalog-comparison/snapshot-generation.json').read_bytes()
    if digest(generation_raw)!='9b54075941cbc495f4f4128999269c7f6d6d32bf2217dd73df46105e266c2e82':raise ValueError('pinned kernel generation differs')
    generation=strict_json(generation_raw)
    if record['source'] not in generation['source']:raise ValueError('record source outside pinned kernel')
    source=args.root/record['source'];source_raw=source.read_bytes();workspace_raw=args.workspace.read_bytes()
    if 'sha256:'+digest(source_raw)!=generation['source'][record['source']]:raise ValueError('declaration source differs from generated kernel')
    if receipt['manifest_digest']!='sha256:'+digest(workspace_raw) or generation['workspace_sha256']!=receipt['manifest_digest']:raise ValueError('interface/workspace binding differs')
    schema_raw=(json.dumps(schema,sort_keys=True,indent=2)+'\n').encode()
    origin={'schema':'gooo/mcp-snapshot-output-schema-origin/v1','record_stable_id':RECORD,
            'workspace':args.workspace.relative_to(args.root).as_posix(),'workspace_sha256':digest(workspace_raw),
            'interface_sha256':digest(raw),'interface_digest':receipt['interface']['digest'],
            'source':record['source'],'source_sha256':digest(source_raw),'source_digest_encoding':'RAW_FILE_BYTES',
            'inspection_compiler_source_sha':identity['compiler_source_sha'],'inspection_go_version':identity['go_version'],
            'inspection_kind':'DECLARATION_INTERFACE_ONLY','program_execution':False,
            'kernel_generation_compiler_source_sha':'205896a413e5dea92366309c35ae85e542d055f6',
            'renderer':'tools/render_mcp_record_schema.py','renderer_sha256':digest(Path(__file__).read_bytes()),
            'schema_sha256':digest(schema_raw),'selected_record':record,
            'scope':'Required-one scalar output shape; source byte/value facets are not a JSON Schema maxLength. Kernel policy and generation pin unchanged.'}
    args.out.mkdir(parents=True,exist_ok=True)
    (args.out/'snapshot-output.schema.json').write_bytes(schema_raw)
    (args.out/'snapshot-output.origin.json').write_text(json.dumps(origin,sort_keys=True,indent=2)+'\n')
    print(json.dumps({'fields':len(record['fields']),'record_id':RECORD,'schema_sha256':digest(schema_raw),'program_execution':False}))

if __name__=='__main__':main()
