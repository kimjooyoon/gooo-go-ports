"""Offline public-release identity controls; no binary, Docker or network runs."""
import importlib.util
import io
import json
import shutil
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest import mock

SPEC = importlib.util.spec_from_file_location("public_oci_under_test", Path(__file__).with_name("service_grant_mcp_oci.py"))
P = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(P)


def encoded(value):
    return (json.dumps(value, sort_keys=True) + "\n").encode()


def synthetic_archive(platform="linux/amd64"):
    # A complete synthetic format fixture, never a native executable or receipt.
    binary = b"synthetic-not-executed"
    build = {"GoVersion": P.CONTEXT.GO_VERSION, "Path": P.CONTEXT.MODULE, "Settings": [
        {"Key": key, "Value": value} for key, value in [
            ("CGO_ENABLED", "0"), ("GOOS", platform.split("/")[0]),
            ("GOARCH", platform.split("/")[1]), ("vcs.revision", P.SOURCE_COMMIT),
            ("vcs.modified", "false")]]}
    files = {P.CONTEXT.BINARY: binary, "BUILD-INFO.json": encoded(build), "START-HERE.txt": b"start\n"}
    notices = []
    for i in range(58):
        name = "NOTICE-module-%02d-LICENSE" % i
        files[name] = ("synthetic notice %d\n" % i).encode()
        notices.append({"file": name, "sha256": P.CONTEXT.digest(files[name])})
    files["NOTICE-INVENTORY.json"] = encoded(notices)
    members = {name: {"bytes": len(raw), "sha256": P.CONTEXT.digest(raw),
                      "mode": 0o755 if name == P.CONTEXT.BINARY else 0o644}
               for name, raw in files.items()}
    files["BUNDLE.json"] = encoded({"schema": P.CONTEXT.SCHEMA, "platform": platform,
        "source_commit": P.SOURCE_COMMIT, "notice_texts": 58, "build_info": build,
        "binary_sha256": P.CONTEXT.digest(binary), "members": members})
    stream = io.BytesIO()
    with tarfile.open(fileobj=stream, mode="w:gz", format=tarfile.USTAR_FORMAT) as archive:
        for name, raw in sorted(files.items()):
            info = tarfile.TarInfo(name)
            info.mode = 0o755 if name == P.CONTEXT.BINARY else 0o644
            info.size = len(raw)
            archive.addfile(info, io.BytesIO(raw))
    return stream.getvalue(), P.CONTEXT.digest(binary)


class PublicReleaseIdentity(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(dir=Path(__file__).parent)
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.bundle = self.root / "local-archive"
        self.out = self.root / "context"
        self.dockerfile = self.root / "Dockerfile.input"
        self.dockerfile.write_bytes(b"FROM scratch\nCOPY runtime/ /opt/gooo/\n")
        self.patch = mock.patch.object(P, "DOCKERFILE", self.dockerfile)
        self.patch.start()
        self.addCleanup(self.patch.stop)

    def prepare(self, **changes):
        args = {"bundle": self.bundle, "sha256": P.ARCHIVE_SHA256,
                "source_commit": P.SOURCE_COMMIT, "version": P.VERSION, "out": self.out}
        args.update(changes)
        return P.prepare_public(**args)

    def synthetic_pins(self, raw, binary_hash):
        # Tests replace the reviewed whole-byte anchors only inside this mock
        # scope; production CLI has no alternate pin or generic URL option.
        return mock.patch.multiple(P, ARCHIVE_BYTES=len(raw),
                                   ARCHIVE_SHA256=P.CONTEXT.digest(raw), BINARY_SHA256=binary_hash)

    def test_fixed_release_metadata(self):
        self.assertEqual(P.VERSION, "v0.1.0-preview.16")
        self.assertEqual(P.SOURCE_COMMIT, "fceb89f8b53eabd6687d16258d7cee89853c2cfe")
        self.assertEqual(P.ARCHIVE_BYTES, 7217754)
        self.assertEqual(P.ARCHIVE_SHA256, "0ae9bc25eef2ad73535642a66b8b7602e9f64a148b3e60696e36048a3b4a0524")
        self.assertEqual(P.BINARY_SHA256, "a848a8747669e46e9afef6c0f1ed8b2014df4838cc370416f60ab9c6acccfc0a")
        self.assertEqual(P.PLATFORM, "linux/amd64")
        self.assertEqual(P.ARCHIVE_URL, "https://github.com/kimjooyoon/gooo-go-ports/releases/download/v0.1.0-preview.16/gooo-service-grant-mcp-linux-amd64.tar.gz")

    def test_documented_archive_identity_matches_the_enforced_release(self):
        # This checks a separate human-facing consumer. The production pins
        # already have independent exact-value assertions above.
        doc = (Path(__file__).resolve().parents[1] /
               "docs/service-grant-oci-publisher.md").read_text()
        expected_rows = [
            f"| Archive | `{P.ARCHIVE_NAME}`, {P.ARCHIVE_BYTES:,} bytes |",
            f"| Archive SHA256 | `{P.ARCHIVE_SHA256}` |",
            f"| Native source | `{P.SOURCE_COMMIT}` |",
            f"| Native executable SHA256 | `{P.BINARY_SHA256}` |",
            f"| Platform | `{P.PLATFORM}`; no multi-platform claim |",
            f"| Go build identity | `{P.CONTEXT.GO_VERSION}`, CGO disabled |",
        ]
        for row in expected_rows:
            with self.subTest(row=row):
                self.assertEqual(doc.count(row), 1)
        self.assertIn(P.RELEASE_URL, doc)

    def test_foreign_version_source_or_checksum_refused_before_preparer(self):
        for kwargs in [{"version": "v0.1.0-preview.13"}, {"source_commit": "0" * 40},
                       {"sha256": "0" * 64}]:
            with self.subTest(kwargs=kwargs), mock.patch.object(P.CONTEXT, "prepare") as delegate:
                with self.assertRaises(P.CONTEXT.Refusal):
                    self.prepare(**kwargs)
                delegate.assert_not_called()
                self.assertFalse(self.out.exists())

    def test_size_and_symlink_refused_before_preparer(self):
        self.bundle.write_bytes(b"not the public archive")
        with mock.patch.object(P.CONTEXT, "prepare") as delegate:
            with self.assertRaisesRegex(P.CONTEXT.Refusal, "byte count"):
                self.prepare()
            alias = self.root / "alias"
            alias.symlink_to(self.bundle)
            with self.assertRaisesRegex(P.CONTEXT.Refusal, "non-symlink"):
                self.prepare(bundle=alias)
            delegate.assert_not_called()
        self.assertFalse(self.out.exists())

    def test_complete_context_reuses_existing_validator_and_preserves_notices(self):
        raw, binary_hash = synthetic_archive()
        self.bundle.write_bytes(raw)
        with self.synthetic_pins(raw, binary_hash), mock.patch.object(P.CONTEXT, "prepare", wraps=P.CONTEXT.prepare) as delegate:
            result = self.prepare()
            delegate.assert_called_once_with(self.bundle, P.ARCHIVE_SHA256, P.SOURCE_COMMIT, self.out, self.dockerfile)
        self.assertEqual(result["context"]["runtime_members"], 63)
        self.assertEqual(result["context"]["notice_texts"], 58)
        self.assertEqual(len(list((self.out / "runtime").iterdir())), 63)
        inventory = json.loads((self.out / "runtime/NOTICE-INVENTORY.json").read_bytes())
        self.assertEqual(len(inventory), 58)
        for notice in inventory:
            self.assertEqual(P.CONTEXT.digest((self.out / "runtime" / notice["file"]).read_bytes()), notice["sha256"])
        self.assertEqual((self.out / "Dockerfile").read_bytes(), self.dockerfile.read_bytes())
        self.assertFalse(result["binary_rebuilt"])
        self.assertFalse(result["image_built_or_published"])
        self.assertFalse(result["runtime_or_client_verification_performed"])
        with self.synthetic_pins(raw, binary_hash), self.assertRaises(FileExistsError):
            self.prepare()

    def test_same_size_corruption_still_refused_by_archive_hash(self):
        raw, binary_hash = synthetic_archive()
        changed = bytearray(raw)
        changed[-1] ^= 1
        self.bundle.write_bytes(changed)
        with self.synthetic_pins(raw, binary_hash), self.assertRaisesRegex(P.CONTEXT.Refusal, "archive SHA256 mismatch"):
            self.prepare()
        self.assertFalse(self.out.exists())

    def test_existing_manifest_platform_gate_is_not_bypassed(self):
        raw, binary_hash = synthetic_archive("darwin/amd64")
        self.bundle.write_bytes(raw)
        with self.synthetic_pins(raw, binary_hash), self.assertRaisesRegex(P.CONTEXT.Refusal, "platform"):
            self.prepare()
        self.assertFalse(self.out.exists())


    def copied_runtime(self):
        raw, binary_hash = synthetic_archive()
        self.bundle.write_bytes(raw)
        with self.synthetic_pins(raw, binary_hash):
            P.prepare_public(self.bundle, P.ARCHIVE_SHA256, P.SOURCE_COMMIT, P.VERSION, self.root / "seed")
        runtime = self.root / "copied-runtime"
        shutil.copytree(self.root / "seed/runtime", runtime)
        return raw, binary_hash, runtime

    def verify(self, runtime, out):
        return P.verify_runtime(self.bundle, P.ARCHIVE_SHA256, P.SOURCE_COMMIT, P.VERSION, runtime, out)

    def test_runtime_identity_compares_all_members_and_records_no_execution(self):
        raw, binary_hash, runtime = self.copied_runtime()
        with self.synthetic_pins(raw, binary_hash):
            result = self.verify(runtime, self.out)
            with self.assertRaises(FileExistsError):
                self.verify(runtime, self.out)
        self.assertEqual(result["checked_members"], 63)
        self.assertEqual({row["name"] for row in result["checked_files"]},
                         {path.name for path in runtime.iterdir()})
        self.assertEqual(json.loads((self.out / "runtime-identity.json").read_bytes()), result)
        self.assertTrue(result["byte_identity_only"])
        for field in ("docker_execution_performed", "producer_authenticated", "binary_executed",
                      "client_compatibility_or_adoption_established"):
            self.assertFalse(result[field])

    def test_runtime_refuses_extra_missing_links_binary_notice_or_mode_edits(self):
        raw, binary_hash, original = self.copied_runtime()
        for kind, pattern in [("extra", "roster"), ("missing", "roster"),
                              ("symlink", "not regular"), ("binary", "bytes differ"),
                              ("notice", "bytes differ"), ("mode", "mode differs")]:
            with self.subTest(kind=kind):
                runtime = self.root / ("changed-" + kind)
                shutil.copytree(original, runtime)
                target = runtime / "NOTICE-module-00-LICENSE"
                if kind == "extra":
                    (runtime / "unexpected").write_bytes(b"extra")
                elif kind == "missing":
                    target.unlink()
                elif kind == "symlink":
                    target.unlink()
                    target.symlink_to(runtime / "NOTICE-module-01-LICENSE")
                elif kind in ("binary", "notice"):
                    if kind == "binary":
                        target = runtime / P.CONTEXT.BINARY
                    changed = bytearray(target.read_bytes())
                    changed[0] ^= 1
                    target.write_bytes(changed)
                else:
                    target.chmod(0o600)
                out = self.root / ("refused-" + kind)
                with self.synthetic_pins(raw, binary_hash), self.assertRaisesRegex(P.CONTEXT.Refusal, pattern):
                    self.verify(runtime, out)
                self.assertFalse((out / "runtime-identity.json").exists())

    def test_runtime_directory_alias_and_nested_output_are_refused(self):
        raw, binary_hash, runtime = self.copied_runtime()
        alias = self.root / "runtime-alias"
        alias.symlink_to(runtime, target_is_directory=True)
        with self.synthetic_pins(raw, binary_hash):
            with self.assertRaisesRegex(P.CONTEXT.Refusal, "non-symlink directory"):
                self.verify(alias, self.out)
            with self.assertRaisesRegex(P.CONTEXT.Refusal, "outside the runtime"):
                self.verify(runtime, runtime / "comparison")
        self.assertFalse(self.out.exists())
        self.assertFalse((runtime / "comparison").exists())


if __name__ == "__main__":
    unittest.main()
