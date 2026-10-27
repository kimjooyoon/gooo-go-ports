"""Export a complete immutable MCP source roster; never rewrite a source file.

The result describes raw-file identities and their derivation. It does not claim
compiler execution, a trusted producer, or permission to publish a package.
"""
import argparse
import ast
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
from urllib.parse import quote

PREFIX='examples/service-grant-mcp'

def export(repo, revision):
    if not re.fullmatch('[0-9a-f]{40}', revision): raise ValueError('one immutable SHA-1 commit required')
    env=dict(os.environ,GIT_NO_REPLACE_OBJECTS='1',GIT_NO_LAZY_FETCH='1',GIT_OPTIONAL_LOCKS='0')
    def git(*args):return subprocess.check_output(['git','-C',str(repo),*args],env=env)
    if git('rev-parse','--verify',revision+'^{commit}').decode().strip()!=revision:raise ValueError('commit differs')
    files={}
    for row in git('ls-tree','-rz','--full-tree',revision,'--',PREFIX).split(b'\0'):
        if not row:continue
        header,name=row.split(b'\t',1);mode,kind,oid=header.decode('ascii').split();path=name.decode('utf-8')
        if kind!='blob' or mode not in ('100644','100755'):raise ValueError('regular tracked source required')
        if not path.startswith(PREFIX+'/') or any(x in ('','.','..') for x in path.split('/')):raise ValueError('source path')
        size=int(git('cat-file','-s',oid))
        if size>2<<20:raise ValueError('source exceeds finite export bound')
        raw=git('cat-file','blob',oid)
        if len(raw)!=size or hashlib.sha1(b'blob '+str(size).encode()+b'\0'+raw).hexdigest()!=oid:raise ValueError('Git blob identity')
        if path.endswith('.py'):
            try:ast.parse(raw,filename=path)
            except (SyntaxError,UnicodeError) as error:raise ValueError('invalid Python source in immutable profile: '+path) from error
        identity='urn:gooo:git-file:'+revision+':'+quote(path,safe='')
        files[path]={'@id':identity,'@type':'prov:Entity','git_blob_sha1':oid,'mode':mode,'bytes':size,
                     'sha256':hashlib.sha256(raw).hexdigest(),'digest_encoding':'RAW_FILE_BYTES'}
    if not files or len(files)>128:raise ValueError('nonempty finite source roster required')
    return {'@context':{'prov':'http://www.w3.org/ns/prov#'},'schema':'gooo/mcp-transport-source-profile/v1',
            'reference_source':revision,'module':'github.com/kimjooyoon/gooo-go-ports/'+PREFIX,
            'source_prefix':PREFIX,'files':files,'prov:wasDerivedFrom':[{'@id':x['@id']} for x in files.values()],
            'scope':'Complete tracked regular file roster in the selected MCP module. Working-tree/untracked files are not read. No compiler execution or authentication asserted.'}

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root',type=Path,default=Path(__file__).resolve().parents[1])
    parser.add_argument('--revision',required=True)
    parser.add_argument('--out',type=Path,required=True)
    args=parser.parse_args()
    if args.out.resolve().is_relative_to((args.root/PREFIX).resolve()):raise ValueError('profile must live outside its own source roster')
    result=export(args.root,args.revision)
    with args.out.open('x') as stream:stream.write(json.dumps(result,sort_keys=True,indent=2)+'\n')
    print(json.dumps({'files':len(result['files']),'reference_source':result['reference_source'],
                      'profile_sha256':hashlib.sha256(args.out.read_bytes()).hexdigest(),'source_mutations':0}))

if __name__=='__main__':main()
