#!/usr/bin/env python3
"""File-boundary refusals; no compiler, Go, network or child process."""
import argparse
import importlib.util
import os
from pathlib import Path
import tempfile
import unittest
from unittest import mock

SPEC = importlib.util.spec_from_file_location('retry_after_verify_tests', Path(__file__).with_name('verify.py'))
V = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(V)


class FileBoundaryTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name).resolve()
        self.path = self.root / 'input'
        self.path.write_bytes(b'original')

    def test_unchanged_file_is_read_and_rechecked(self):
        frozen = V.Frozen()
        self.assertEqual(frozen.read(self.path, 8), b'original')
        frozen.check()
        self.assertEqual(frozen.pins[str(self.path)]['bytes'], 8)

    def test_oversize_file_is_refused_before_content_read(self):
        with self.assertRaisesRegex(RuntimeError, 'bounded regular'):
            V.Frozen().read(self.path, 7)

    def test_symlink_and_relative_paths_are_refused(self):
        link = self.root / 'link'
        link.symlink_to(self.path)
        for path in (link, Path('relative')):
            with self.subTest(path=path), self.assertRaisesRegex(RuntimeError, 'canonical absolute'):
                V.Frozen().read(path)

    def test_same_bytes_replaced_inode_is_refused(self):
        frozen = V.Frozen()
        frozen.read(self.path)
        replacement = self.root / 'replacement'
        replacement.write_bytes(b'original')
        replacement.replace(self.path)
        with self.assertRaisesRegex(RuntimeError, 'identity/metadata changed'):
            frozen.check()

    def test_in_place_content_drift_is_refused(self):
        frozen = V.Frozen()
        frozen.read(self.path)
        before = self.path.stat()
        self.path.write_bytes(b'mutation')
        # Restore mtime to exercise the content check rather than metadata alone.
        os.utime(self.path, ns=(before.st_atime_ns, before.st_mtime_ns))
        with self.assertRaisesRegex(RuntimeError, 'frozen input changed'):
            frozen.check()

    def test_existing_output_refused_without_import_or_overwrite(self):
        out = self.root / 'evidence'
        out.mkdir()
        sentinel = out / 'first-failure'
        sentinel.write_bytes(b'preserve')
        args = argparse.Namespace(root=str(self.root), compiler=str(self.path),
                                  go=str(self.path), out=str(out))
        with mock.patch.object(V, 'module', side_effect=AssertionError('must not import')):
            with self.assertRaisesRegex(RuntimeError, 'fresh output'):
                V.run(args)
        self.assertEqual(sentinel.read_bytes(), b'preserve')
        self.assertEqual(sorted(p.name for p in out.iterdir()), ['first-failure'])


if __name__ == '__main__':
    unittest.main()
