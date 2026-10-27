"""Shared immutable v4 transport, dependency and declaration-shape contract.

This is package metadata and a source reader. It never executes a compiler,
comparison program, model, network request or service operation.
"""
import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import stat

SCHEMA='gooo/service-grant-mcp-native-bundle/v4'
CONTEXT_SCHEMA='gooo/service-grant-mcp-container-context/v3'
SOURCE='f563cfb594def60844aaecce06be9b6a063b9247'
PROFILE='tools/profiles/mcp-definition-snapshot-v4.json'
PROFILE_SHA256='3bdf202c12ad6a80ddbbe3e6afaf7bbf5e3e2cb34f2b7f9591c29bb29a303125'
PREFIX='examples/service-grant-mcp'
MODULE='github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison'
VERSION='v0.0.0-20261027155614-e4d44fd856ee'
SUM='h1:ATJzvyeAk2y4MazX9BXoZDYHb94ePod4Ufb2awKzSVE='
DEPENDENCY={'Path':MODULE,'Version':VERSION,'Sum':SUM}
NOTICES={'LICENSE':'099e1adf9145f6fe45357f004f503231a21b58244fdf105a34117d1a33b4bd48',
         'NOTICE.md':'d7c533049770a214b37f3f625673e713be30d3cd61a06cdbe2f34a36a720060f'}
GENERATION='9b54075941cbc495f4f4128999269c7f6d6d32bf2217dd73df46105e266c2e82'
PIN='205896a413e5dea92366309c35ae85e542d055f6'
TOOL='gooo_compare_saved_tool_definitions'
FLAG='--definition-snapshot'
SAMPLE='snapshot-example.json'
MEMBER_COUNT=68
NOTICE_COUNT=62


def require(value,message):
    if not value:raise ValueError(message)


def digest(raw):return hashlib.sha256(raw).hexdigest()


def profiles(legacy):
    result=copy.deepcopy(legacy)
    for name,parent in [('definition_snapshot','default'),('many_and_snapshot','many_required'),
                        ('alias_and_snapshot','alias_origin_change'),('all_optional','many_and_alias')]:
        result[name]={'argv':[*legacy[parent]['argv'],FLAG],
                      'tools':[*legacy[parent]['tools'],TOOL]}
    return result


def read_profile(root,member,load):
    raw=member(root,PROFILE)
    require(digest(raw)==PROFILE_SHA256,'snapshot transport profile bytes differ')
    profile=load(raw)
    require(profile['schema']=='gooo/mcp-transport-source-profile/v1' and
            profile['reference_source']==SOURCE and profile['source_prefix']==PREFIX and
            profile['module']=='github.com/kimjooyoon/gooo-go-ports/'+PREFIX and
            len(profile['files'])==26,'snapshot transport profile scope differs')
    for name,row in profile['files'].items():
        require(name.startswith(PREFIX+'/') and row['digest_encoding']=='RAW_FILE_BYTES' and
                row['@type']=='prov:Entity' and row['mode'] in ['100644','100755'],
                'snapshot source entity differs')
        content=member(root,name)
        require(len(content)==row['bytes'] and digest(content)==row['sha256'] and
                hashlib.sha1(b'blob '+str(len(content)).encode()+b'\0'+content).hexdigest()==row['git_blob_sha1'],
                'snapshot source bytes differ: '+name)
        require(stat.S_IMODE((root/name).lstat().st_mode)==int(row['mode'][-3:],8),
                'snapshot source file mode differs: '+name)
    return profile


def output_shape(root,member,load):
    raw=member(root,PREFIX+'/schemas/snapshot-output.schema.json')
    origin_raw=member(root,PREFIX+'/schemas/snapshot-output.origin.json')
    origin=load(origin_raw)
    require(origin['schema']=='gooo/mcp-snapshot-output-schema-origin/v1' and
            origin['inspection_kind']=='DECLARATION_INTERFACE_ONLY' and origin['program_execution'] is False and
            origin['inspection_compiler_source_sha']=='1afb98b955fd8dea74650d3b24e693d0dab9eb5d' and
            origin['inspection_go_version']=='go1.27.3' and
            origin['kernel_generation_compiler_source_sha']==PIN,'snapshot schema inspection profile differs')
    require(origin['source_digest_encoding']=='RAW_FILE_BYTES' and
            digest(member(root,origin['source']))==origin['source_sha256'] and
            digest(member(root,origin['workspace']))==origin['workspace_sha256'] and
            digest(member(root,origin['renderer']))==origin['renderer_sha256'] and
            digest(raw)==origin['schema_sha256'],'snapshot declaration/schema source binding differs')
    spec=importlib.util.spec_from_file_location('snapshot_record_schema',root/origin['renderer'])
    renderer=importlib.util.module_from_spec(spec);spec.loader.exec_module(renderer)
    rendered,_=renderer.render({'schema':'gooo/package-workspace-interface-receipt/v1','decision':'PASS',
        'interface':{'packages':[{'declarations':[origin['selected_record']]}]}})
    require(rendered==load(raw),'snapshot schema differs from retained Gooo declaration')
    return load(raw),origin


def optional(generation,shape,origin):
    return {'argv':[FLAG],'additional_tool':TOOL,'module':DEPENDENCY,
            'compiler_source_sha':PIN,'generation_sha256':'sha256:'+GENERATION,
            'bindings':generation,'output_schema':shape,'schema_origin':origin,
            'transport_source_profile_sha256':PROFILE_SHA256,'example_member':SAMPLE}
