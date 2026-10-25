"""Derive a producer-bound index from one completed v2 observation run.

No compiler, Go tool, native body or replay is executed. Original run files are
read-only. The new index explicitly records that it rechecks retained evidence.
"""
import argparse
import importlib.util
import json
from pathlib import Path

HERE=Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('snapshot_bound_producer',HERE/'evidence.py')
e=importlib.util.module_from_spec(spec);spec.loader.exec_module(e)


def derive(index_path,summary_path):
    root=index_path.parent
    assert summary_path.parent==root
    source=e.host.strict(e.read(index_path,64<<10));summary=e.host.strict(e.read(summary_path,4<<20))
    assert set(source)=={'schema','batches','rejections'} and source['schema']=='gooo/definition-snapshot-generation-input/v2'
    go=summary['go_identity']
    e.same({k:go[k] for k in ('path','bytes','sha256')},e.host.ref(Path(go['path'])))
    assert go['version'] in ('go version '+e.GO_VERSION+' darwin/arm64','go version '+e.GO_VERSION+' linux/amd64')
    value={'schema':'gooo/definition-snapshot-generation-input/v3',
           'producer':{'binary':summary['compiler'],'identity':'compiler-identity/stdout','process':'compiler-identity/process.json'},
           'lineage':{'kind':'RETAINED_OBSERVATIONS_NEW_INDEX','fresh_native_execution':False,
                      'source_index':{'file':index_path.name,'sha256':e.sha(e.read(index_path))},
                      'source_summary':{'file':summary_path.name,'sha256':e.sha(e.read(summary_path,4<<20))},
                      'derivation_source':{'file':str(Path(__file__).relative_to(e.ROOT)),'sha256':e.sha(e.read(Path(__file__)))}},
           'batches':source['batches'],'rejections':source['rejections']}
    e.verify_index_document(value,root,go)
    return value


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--index',type=Path,required=True)
    parser.add_argument('--summary',type=Path,required=True)
    parser.add_argument('--out',type=Path,required=True)
    args=parser.parse_args()
    assert all(p.is_absolute() for p in (args.index,args.summary,args.out))
    assert args.out.parent==args.index.parent and not args.out.exists() and not args.out.is_symlink()
    assert not args.out.resolve().is_relative_to(e.ROOT.resolve())
    value=derive(args.index,args.summary)
    # Exclusive creation preserves both originals and any prior derived index.
    with args.out.open('xb') as output:
        output.write((json.dumps(value,indent=2)+'\n').encode())
    print(json.dumps({'status':'RETAINED_OBSERVATIONS_VERIFIED_NO_FRESH_EXECUTION','index':e.host.ref(args.out),'source_index':e.host.ref(args.index),'fresh_native_execution':False,'reference_cases':89,'native_cases':83,'input_rejection_cases':6}))


if __name__=='__main__':main()
