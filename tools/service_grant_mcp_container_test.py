import copy
import hashlib
import io
import json
import os
from pathlib import Path
import stat
import tarfile
import tempfile
import unittest

import importlib.util
SPEC = importlib.util.spec_from_file_location("container_context_under_test", Path(__file__).with_name("service_grant_mcp_container.py"))
P = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(P)

SOURCE = "694d485217c7ec9afac0437453038a66a0db0a33"


def encoded(value):
    return (json.dumps(value, sort_keys=True) + "\n").encode()


def fixture(alias=False):
    files = {P.BINARY: b"synthetic-not-executed-binary", "START-HERE.txt": b"start\n"}
    build = {"GoVersion": P.GO_VERSION, "Path": P.MODULE, "Settings": [
        {"Key": "CGO_ENABLED", "Value": "0"},
        {"Key": "GOOS", "Value": "linux"},
        {"Key": "GOARCH", "Value": "amd64"},
        {"Key": "vcs.revision", "Value": SOURCE},
        {"Key": "vcs.modified", "Value": "false"}]}
    files["BUILD-INFO.json"] = encoded(build)
    inventory = []
    for index in range(58):
        name = "NOTICE-module-%02d-LICENSE" % index
        files[name] = ("synthetic notice %d\n" % index).encode()
        inventory.append({"file": name, "sha256": P.digest(files[name])})
    files["NOTICE-INVENTORY.json"] = encoded(inventory)
    bundle = {"schema": P.SCHEMA, "platform": "linux/amd64", "source_commit": SOURCE,
              "notice_texts": 58, "build_info": build,
              "binary_sha256": P.digest(files[P.BINARY])}
    if alias:
        module = Path(__file__).resolve().parents[1] / "examples/alias-origin-change"
        many = json.loads((module.parent / "service-grant-many/generation.json").read_bytes())
        for name in P.ALIAS_NOTICES:
            target = "NOTICE-module-alias-" + name
            files[target] = (module / name).read_bytes()
            inventory.append({"file": target, "module": copy.deepcopy(P.ALIAS_DEPENDENCY),
                              "source_relative_path": name, "sha256": P.digest(files[target])})
        files["NOTICE-INVENTORY.json"] = encoded(inventory)
        build["Deps"] = [{"Path": "synthetic-unused-dependency-%d" % n} for n in range(11)] + [copy.deepcopy(P.ALIAS_DEPENDENCY)]
        files["BUILD-INFO.json"] = encoded(build)
        bundle.update(schema=P.ALIAS_SCHEMA, notice_texts=60, profiles=copy.deepcopy(P.PROFILES),
                      optional_profiles={"many_required": {
                          "argv": ["--many-required"], "additional_tool": "assess_service_grant_change",
                          "module": {"Path": "github.com/kimjooyoon/gooo-go-ports/examples/service-grant-many",
                                     "Version": "v0.0.0-20261020182808-7180a4a0f36a",
                                     "Sum": "h1:p3zs2m5zxSwlgn0Z3Wq1rl+cfcBN4JN2JhrmjhARgkk="},
                          "compiler_source_sha": "205896a413e5dea92366309c35ae85e542d055f6",
                          "generation_sha256": "sha256:86d714e4793f04d7d45d53e824cd7066ddee5f7d1e987697f26f4216a6b4e164",
                          "assets": many["assets"], "sources": many["public_sources"]}, "alias_origin_change": {
                          "argv": ["--alias-origin-change"], "additional_tool": "gooo_compare_alias_origin",
                          "module": copy.deepcopy(P.ALIAS_DEPENDENCY),
                          "compiler_source_sha": "205896a413e5dea92366309c35ae85e542d055f6",
                          "generation_sha256": "sha256:" + P.ALIAS_GENERATION,
                          "bindings": json.loads((module / "generation.json").read_text())}})
    return files, bundle


def pack(files, bundle, change=None):
    files = copy.deepcopy(files)
    bundle = copy.deepcopy(bundle)
    bundle["members"] = {name: {"bytes": len(raw), "sha256": P.digest(raw),
        "mode": 0o755 if name == P.BINARY else 0o644} for name, raw in files.items()}
    if change:
        change(files, bundle)
    files["BUNDLE.json"] = encoded(bundle)
    target = io.BytesIO()
    with tarfile.open(fileobj=target, mode="w:gz", format=tarfile.USTAR_FORMAT) as tf:
        for name, raw in sorted(files.items()):
            info = tarfile.TarInfo(name)
            info.mode = 0o755 if name == P.BINARY else 0o644
            info.size = len(raw)
            tf.addfile(info, io.BytesIO(raw))
    return target.getvalue()


class Preparation(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(dir=Path(__file__).parent)
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.dockerfile = self.root / "Dockerfile.input"
        self.dockerfile.write_bytes(b"FROM scratch\nCOPY runtime/ /opt/gooo/\n")
        self.files, self.bundle = fixture()
        self.out = self.root / "context"

    def run_prepare(self, raw, expected=None):
        archive = self.root / "bundle.tar.gz"
        archive.write_bytes(raw)
        return P.prepare(archive, expected or P.digest(raw), SOURCE, self.out, self.dockerfile)

    def refused(self, raw, pattern, expected=None):
        with self.assertRaisesRegex(P.Refusal, pattern):
            self.run_prepare(raw, expected)
        self.assertFalse(self.out.exists(), "refusal must precede output creation")

    def test_valid_context_and_fresh_output_modes(self):
        raw = pack(self.files, self.bundle)
        old_umask = os.umask(0o077)
        try:
            result = self.run_prepare(raw)
        finally:
            os.umask(old_umask)
        self.assertEqual(result["runtime_members"], 63)
        self.assertEqual(result["notice_texts"], 58)
        self.assertFalse(result["producer_authenticated"])
        self.assertFalse(result["binary_build_info_independently_parsed"])
        self.assertEqual(len(list((self.out / "runtime").iterdir())), 63)
        for path in [self.out, self.out / "runtime", self.out / "runtime" / P.BINARY]:
            self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o755)
        for path in [self.out / "Dockerfile", self.out / "context.json",
                     self.out / "runtime" / "BUNDLE.json"]:
            self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o644)
        with self.assertRaises(FileExistsError):
            self.run_prepare(raw)

    def test_invalid_expected_hash_and_wrong_archive_hash(self):
        raw = pack(self.files, self.bundle)
        self.refused(raw, "expected archive SHA256", "not-a-sha")
        self.refused(raw, "archive SHA256 mismatch", "0" * 64)

    def test_alias_layout_requires_all_bound_notices_and_profiles(self):
        files, bundle = fixture(alias=True)
        result = self.run_prepare(pack(files, bundle))
        self.assertEqual(result["runtime_members"], 65)
        self.assertEqual(result["notice_texts"], 60)
        actual = json.loads((self.out / "runtime/BUNDLE.json").read_text())
        self.assertEqual(actual["profiles"], P.PROFILES)

    def test_alias_schema_cannot_relabel_legacy_or_extra_members(self):
        for alias in (False, True):
            with self.subTest(alias=alias):
                files, bundle = fixture(alias=alias)
                bundle["schema"] = P.SCHEMA if alias else P.ALIAS_SCHEMA
                self.refused(pack(files, bundle), "population differs")

    def test_alias_pin_generation_and_profile_changes_refuse_before_output(self):
        changes = [
            lambda b: b["profiles"]["default"]["tools"].append("gooo_compare_alias_origin"),
            lambda b: b["profiles"]["many_and_alias"].update(argv=["--alias-origin-change"]),
            lambda b: b["optional_profiles"]["many_required"].update(argv=[]),
            lambda b: b["optional_profiles"]["alias_origin_change"]["module"].update(Sum="h1:changed"),
            lambda b: b["optional_profiles"]["alias_origin_change"]["bindings"].update(compiler_source_sha="0" * 40),
            lambda b: b["build_info"]["Deps"][-1].update(Replace={"Path": "local"}),
        ]
        for change in changes:
            files, bundle = fixture(alias=True)
            change(bundle)
            files["BUILD-INFO.json"] = encoded(bundle["build_info"])
            self.refused(pack(files, bundle), "profiles|identity|binding|dependency")

    def test_alias_notice_cannot_be_swapped_for_an_unrelated_valid_notice(self):
        files, bundle = fixture(alias=True)
        inventory = json.loads(files["NOTICE-INVENTORY.json"])
        inventory[-1]["module"]["Version"] = "v0.0.0-other"
        files["NOTICE-INVENTORY.json"] = encoded(inventory)
        self.refused(pack(files, bundle), "notice source pin")

    def test_unsafe_duplicate_and_nonregular_tar_records(self):
        for kind in ["../outside", "/absolute", "nested/name", "duplicate",
                     "symlink", "hardlink", "directory", "pax"]:
            with self.subTest(kind=kind):
                target = io.BytesIO()
                with tarfile.open(fileobj=target, mode="w:gz", format=tarfile.USTAR_FORMAT) as tf:
                    info = tarfile.TarInfo("first")
                    info.mode = 0o644
                    info.size = 1
                    tf.addfile(info, io.BytesIO(b"x"))
                    info = tarfile.TarInfo(kind if "/" in kind else "second")
                    info.mode = 0o644
                    info.size = 0
                    if kind == "duplicate":
                        info.name = "first"
                    elif kind in ["symlink", "hardlink"]:
                        info.type = tarfile.SYMTYPE if kind == "symlink" else tarfile.LNKTYPE
                        info.linkname = "first"
                    elif kind == "directory":
                        info.type = tarfile.DIRTYPE
                    elif kind == "pax":
                        info.type = tarfile.XHDTYPE
                    tf.addfile(info)
                self.refused(target.getvalue(), "unsafe|duplicate|nonregular")
                self.assertFalse((self.root / "outside").exists())

    def test_missing_notice_inventory_reference(self):
        inventory = json.loads(self.files["NOTICE-INVENTORY.json"])
        inventory[0]["file"] = "NOTICE-module-missing-LICENSE"
        self.files["NOTICE-INVENTORY.json"] = encoded(inventory)
        self.refused(pack(self.files, self.bundle), "missing notice")

    def test_manifest_member_hash_and_mode_are_checked(self):
        for field, value, pattern in [("sha256", "0" * 64, "hash mismatch"),
                                      ("mode", 0o755, "mode mismatch")]:
            with self.subTest(field=field):
                def change(files, bundle):
                    bundle["members"]["START-HERE.txt"][field] = value
                self.refused(pack(self.files, self.bundle, change), pattern)

    def test_buildinfo_cgo_and_source_must_match(self):
        for key, value in [("CGO_ENABLED", "1"), ("vcs.revision", "0" * 40)]:
            with self.subTest(key=key):
                files, bundle = fixture()
                for setting in bundle["build_info"]["Settings"]:
                    if setting["Key"] == key:
                        setting["Value"] = value
                files["BUILD-INFO.json"] = encoded(bundle["build_info"])
                self.refused(pack(files, bundle), "build-info platform, CGO")


if __name__ == "__main__":
    unittest.main()
