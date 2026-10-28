import hashlib
import importlib.util
from pathlib import Path
import subprocess
import tempfile
import unittest

spec=importlib.util.spec_from_file_location('freeze_profile',Path(__file__).with_name('freeze_mcp_source_profile.py'))
M=importlib.util.module_from_spec(spec);spec.loader.exec_module(M)

class FrozenSource(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup)
        self.root=Path(self.temp.name);self.module=self.root/M.PREFIX;self.module.mkdir(parents=True)
        self.git('init','-q');self.git('config','user.name','source-test');self.git('config','user.email','source-test@example.invalid')
        (self.module/'main.go').write_text('package main\n')
        self.commit()
    def git(self,*args):return subprocess.check_output(['git','-C',str(self.root),*args]).decode().strip()
    def commit(self):
        self.git('add','.');self.git('commit','-qm','source');self.revision=self.git('rev-parse','HEAD')
    def test_immutable_source_ignores_working_edits_and_untracked(self):
        before=M.export(self.root,self.revision)
        (self.module/'main.go').write_text('working tree is different\n')
        (self.module/'untracked.go').write_text('not read\n')
        self.assertEqual(before,M.export(self.root,self.revision))
        self.assertEqual((self.module/'main.go').read_text(),'working tree is different\n')
    def test_raw_bytes_and_mode_are_explicit(self):
        row=M.export(self.root,self.revision)['files'][M.PREFIX+'/main.go']
        self.assertEqual(row['sha256'],hashlib.sha256(b'package main\n').hexdigest())
        self.assertEqual(row['mode'],'100644');self.assertEqual(row['digest_encoding'],'RAW_FILE_BYTES')
    def test_new_tracked_file_is_not_silently_omitted(self):
        old=M.export(self.root,self.revision)
        (self.module/'second.go').write_text('package main\n');self.commit()
        new=M.export(self.root,self.revision)
        self.assertEqual(len(old['files']),1);self.assertEqual(len(new['files']),2)
    def test_symlink_refused_without_following_target(self):
        outside=self.root/'outside';outside.write_text('retain private outside bytes\n')
        (self.module/'linked').symlink_to(outside);self.commit()
        with self.assertRaises(ValueError):M.export(self.root,self.revision)
        self.assertEqual(outside.read_text(),'retain private outside bytes\n')
    def test_mutable_revision_name_refused(self):
        with self.assertRaises(ValueError):M.export(self.root,'HEAD')
    def test_invalid_python_source_refused_before_export(self):
        (self.module/'verify.py').write_text('def broken(:\n');self.commit()
        with self.assertRaisesRegex(ValueError,'invalid Python source'):M.export(self.root,self.revision)

if __name__=='__main__':unittest.main(verbosity=2)
