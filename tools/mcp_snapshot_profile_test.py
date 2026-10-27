"""v4 source/dependency/layout refusals; synthetic archives never execute code."""
import copy
import importlib.util
import json
import os
from pathlib import Path
import shutil
import tempfile
import unittest

HERE=Path(__file__).resolve().parent
ROOT=HERE.parent


def imported(name,path):
    spec=importlib.util.spec_from_file_location(name,path)
    module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
    return module


P=imported('snapshot_context',HERE/'service_grant_mcp_container.py')
M=imported('snapshot_bundle',HERE/'service_grant_mcp_bundle.py')
F=imported('snapshot_contract',HERE/'mcp_snapshot_profile.py')
FIXTURE=imported('legacy_context_fixture',HERE/'service_grant_mcp_container_test.py')
BUILD=imported('legacy_build_fixture',HERE/'service_grant_mcp_bundle_test.py')


def fixture():
    files,bundle=FIXTURE.fixture(alias=True)
    frozen=F.read_profile(ROOT,M.member,M.load)
    shape,origin=F.output_shape(ROOT,M.member,M.load)
    generation=json.loads((ROOT/'examples/recorded-catalog-comparison/snapshot-generation.json').read_bytes())
    inventory=json.loads(files['NOTICE-INVENTORY.json'])
    for name in F.NOTICES:
        target='NOTICE-module-snapshot-'+name
        files[target]=(ROOT/'examples/recorded-catalog-comparison'/name).read_bytes()
        inventory.append({'file':target,'module':copy.deepcopy(F.DEPENDENCY),
                          'source_relative_path':name,'sha256':P.digest(files[target])})
    files['NOTICE-INVENTORY.json']=FIXTURE.encoded(inventory)
    bundle['build_info']['Deps'].append(copy.deepcopy(F.DEPENDENCY))
    files['BUILD-INFO.json']=FIXTURE.encoded(bundle['build_info'])
    files[F.SAMPLE]=(ROOT/'examples/service-grant-mcp/fixtures/snapshot.input.json').read_bytes()
    bundle.update(schema=F.SCHEMA,notice_texts=F.NOTICE_COUNT,profiles=F.profiles(P.PROFILES),
                  transport_source_profile=frozen,source_files={k:v['sha256'] for k,v in frozen['files'].items()})
    bundle['optional_profiles']['definition_snapshot']=F.optional(generation,shape,origin)
    return files,bundle


class SnapshotContract(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup)
        self.root=Path(self.temp.name)

    def validate(self,files,bundle):
        raw=FIXTURE.pack(files,bundle)
        members,modes,_=P.unpack_checked(raw)
        return P.validate_manifest(members,modes,FIXTURE.SOURCE)

    def test_real_source_and_declared_shape_read_without_execution(self):
        frozen=F.read_profile(ROOT,M.member,M.load)
        shape,origin=F.output_shape(ROOT,M.member,M.load)
        self.assertEqual(len(frozen['files']),26)
        self.assertEqual(len(shape['properties']['decision']['required']),32)
        self.assertFalse(origin['program_execution'])

    def test_v4_flat_archive_prepares_exact_runtime_and_eight_profiles(self):
        files,bundle=fixture();raw=FIXTURE.pack(files,bundle)
        archive=self.root/'bundle.tar.gz';archive.write_bytes(raw)
        dockerfile=self.root/'Dockerfile';dockerfile.write_bytes(b'FROM scratch\nCOPY runtime/ /opt/gooo/\n')
        out=self.root/'context'
        result=P.prepare(archive,P.digest(raw),FIXTURE.SOURCE,out,dockerfile)
        self.assertEqual(result['schema'],F.CONTEXT_SCHEMA)
        self.assertEqual(result['runtime_members'],68)
        self.assertEqual(result['notice_texts'],62)
        self.assertEqual(len(list((out/'runtime').iterdir())),68)
        self.assertEqual(len(bundle['profiles']),8)
        self.assertEqual(bundle['profiles']['default'],P.PROFILES['default'])

    def test_new_profile_cannot_lose_opt_in_or_authority_boundaries(self):
        files,base=fixture()
        for mutate in [lambda b:b['profiles']['default']['argv'].append(F.FLAG),
                       lambda b:b['optional_profiles']['definition_snapshot'].update(argv=[]),
                       lambda b:b['optional_profiles']['definition_snapshot']['bindings'].update(compiler_source_sha='other'),
                       lambda b:b['optional_profiles']['definition_snapshot']['schema_origin'].update(program_execution=True),
                       lambda b:b['transport_source_profile'].update(reference_source='0'*40)]:
            bundle=copy.deepcopy(base);mutate(bundle)
            with self.assertRaises(P.Refusal):self.validate(files,bundle)

    def test_mutated_example_or_relabelled_notice_cannot_hide_behind_new_manifest(self):
        files,bundle=fixture()
        changed=copy.deepcopy(files);changed[F.SAMPLE]+=b' '
        with self.assertRaises(P.Refusal):self.validate(changed,bundle)
        changed=copy.deepcopy(files);inventory=json.loads(changed['NOTICE-INVENTORY.json'])
        row=next(x for x in inventory if x.get('module')==F.DEPENDENCY)
        row['module']['Version']='latest'
        changed['NOTICE-INVENTORY.json']=FIXTURE.encoded(inventory)
        with self.assertRaises(P.Refusal):self.validate(changed,bundle)

    def test_v4_dependency_and_legacy_v3_do_not_share_mutable_global_state(self):
        build=BUILD.build()
        dep={'dependency':F.DEPENDENCY,'notices':F.NOTICES}
        deps=sorted([*M.DEPENDENCIES,dep],key=lambda x:x['dependency']['Path'])
        token=M.CURRENT_PROFILE.set({'dependencies':deps,'source_files':{}})
        try:
            build['Deps']=copy.deepcopy([x['dependency'] for x in deps])
            M.build_identity(build,'darwin/arm64',BUILD.COMMIT)
            index=next(i for i,x in enumerate(build['Deps']) if x['Path']==F.MODULE)
            for change in [{'Version':'latest'},{'Sum':'h1:wrong'},{'Replace':{'Path':'local'}}]:
                altered=copy.deepcopy(build);altered['Deps'][index].update(change)
                with self.assertRaises(ValueError):M.build_identity(altered,'darwin/arm64',BUILD.COMMIT)
        finally:M.CURRENT_PROFILE.reset(token)
        self.assertEqual(len(M.dependencies()),12)
        M.build_identity(BUILD.build(),'darwin/arm64',BUILD.COMMIT)

    def test_profile_context_resets_even_when_packaging_refuses(self):
        out=self.root/'out';out.mkdir()
        with self.assertRaises(FileNotFoundError):
            M.produce(self.root/'missing-binary',self.root/'build',self.root/'wire',
                self.root/'cache',self.root/'go',ROOT,out,F.SOURCE,
                snapshot_wire_path=self.root/'snapshot/acceptance.json',snapshot_cli=self.root/'compare')
        self.assertIsNone(M.CURRENT_PROFILE.get())
        self.assertEqual(M.source_files(),M.SOURCE_FILES)

    def test_public_snapshot_zip_and_installed_source_are_both_required(self):
        cache=Path(os.environ.get('GOMODCACHE',Path.home()/'go/pkg/mod'))
        source=cache/(F.MODULE+'@'+F.VERSION)
        archive=cache/'cache/download'/F.MODULE/'@v'/(F.VERSION+'.zip')
        if not archive.exists():self.skipTest('download/build the pinned MCP module first')
        generation,_=M.snapshot_module_source(cache,ROOT,F)
        self.assertEqual(generation['compiler_source_sha'],F.PIN)
        private=self.root/'cache';target=private/(F.MODULE+'@'+F.VERSION)
        target.parent.mkdir(parents=True);shutil.copytree(source,target)
        copied=private/archive.relative_to(cache);copied.parent.mkdir(parents=True);shutil.copyfile(archive,copied)
        path=target/'snapshot.go'
        if not path.exists():path=next(p for p in target.glob('*.go') if not p.name.endswith('_test.go'))
        path.chmod(0o600);path.write_bytes(path.read_bytes()+b'\n// changed installed source\n')
        with self.assertRaises(ValueError):M.snapshot_module_source(private,ROOT,F)


if __name__=='__main__':unittest.main(verbosity=2)
