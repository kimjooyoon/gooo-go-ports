"""Offline release role and trust-boundary controls; no asset/native execution."""

import copy
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock

SPEC = importlib.util.spec_from_file_location(
    "release_profile_under_test", Path(__file__).with_name("service_grant_release_profile.py"))
P = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(P)


class ReleaseProfiles(unittest.TestCase):
    def setUp(self):
        self.version = "v0.1.0-preview.16"
        self.profile = P.load_release(self.version)
        name, _ = P.RELEASES[self.version]
        self.document = json.loads((Path(P.__file__).with_name("profiles") / name).read_bytes())

    def test_existing_public_roles_preserved_exactly(self):
        # Independent historical release anchors, not just a second read of the profile.
        p = self.profile
        self.assertEqual(p["native_source"], "fceb89f8b53eabd6687d16258d7cee89853c2cfe")
        self.assertEqual(p["archive"]["bytes"], 7217754)
        self.assertEqual(p["archive"]["sha256"], "0ae9bc25eef2ad73535642a66b8b7602e9f64a148b3e60696e36048a3b4a0524")
        self.assertEqual(p["binary"]["sha256"], "a848a8747669e46e9afef6c0f1ed8b2014df4838cc370416f60ab9c6acccfc0a")
        self.assertEqual(p["bundle"], {"schema": "gooo/service-grant-mcp-native-bundle/v2",
                                      "runtime_members": 63, "notice_texts": 58})
        self.assertEqual(p["archive_url"], "https://github.com/kimjooyoon/gooo-go-ports/releases/download/v0.1.0-preview.16/gooo-service-grant-mcp-linux-amd64.tar.gz")
        self.assertEqual(p["provenance"]["prov:wasDerivedFrom"][0]["@id"],
                         "urn:sha256:" + p["archive"]["sha256"])

    def test_profile_mutation_does_not_change_later_loads(self):
        self.profile["archive"]["bytes"] = 1
        self.profile["bundle"]["runtime_members"] = 1
        self.assertEqual(P.load_release()["archive"]["bytes"], 7217754)
        self.assertEqual(P.load_release()["bundle"]["runtime_members"], 63)

    def test_unknown_versions_paths_and_urls_refused_before_filesystem(self):
        for version in ("v0.1.0-preview.999999", "../../private", "https://example.test/profile",
                        "v0.1.0-preview.16\nARCHIVE_URL=bad", "", True, None):
            with self.subTest(version=version), mock.patch.object(P.Path, "lstat") as lstat:
                with self.assertRaisesRegex(P.ProfileRefusal, "unreviewed"):
                    P.load_release(version)
                lstat.assert_not_called()

    def test_changed_roles_unknown_keys_and_scalar_types_refused(self):
        changes = [
            ("version", "v0.1.0-preview.17"), ("native_source", "main"),
            ("native_source", "0" * 40), ("platform", "darwin/arm64"),
            ("go_version", "go1.27.2"), ("archive_url", "https://example.test/replacement"),
            (("archive", "bytes"), True), (("archive", "bytes"), 1.5),
            (("archive", "bytes"), 0), (("archive", "bytes"), (16 << 20) + 1),
            (("archive", "name"), "../outside"), (("binary", "name"), "other"),
            (("archive", "sha256"), "0" * 64), (("binary", "sha256"), "SHA256:abc"),
            (("bundle", "schema"), "unknown"), (("bundle", "runtime_members"), True),
            (("bundle", "notice_texts"), -1), (("bundle", "notice_texts"), 1000),
        ]
        for path, value in changes:
            d = copy.deepcopy(self.document)
            if isinstance(path, tuple):
                d[path[0]][path[1]] = value
            else:
                d[path] = value
            with self.subTest(path=path, value=value), self.assertRaises(P.ProfileRefusal):
                P._validate(d, self.version)

    def test_duplicate_decoded_keys_rejected(self):
        with self.assertRaisesRegex(P.ProfileRefusal, "duplicate"):
            json.loads('{"version":1,"ver\\u0073ion":2}', object_pairs_hook=P.pairs)

    def test_hash_and_symlink_refused_on_selected_profile(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            profiles = root / "profiles"
            profiles.mkdir()
            path = profiles / "reviewed.json"
            raw = json.dumps(self.document).encode()
            path.write_bytes(raw)
            registry = {self.version: (path.name, hashlib.sha256(raw).hexdigest())}
            with mock.patch.object(P, "__file__", str(root / "loader.py")), mock.patch.object(P, "RELEASES", registry):
                self.assertEqual(P.load_release()["native_source"], self.document["native_source"])
                path.write_bytes(raw + b" ")
                with self.assertRaisesRegex(P.ProfileRefusal, "SHA256"):
                    P.load_release()
                path.unlink()
                elsewhere = root / "elsewhere.json"
                elsewhere.write_bytes(raw)
                path.symlink_to(elsewhere)
                with self.assertRaisesRegex(P.ProfileRefusal, "regular"):
                    P.load_release()

    def test_profile_directory_alias_and_oversized_source_refused(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            real = root / "real"
            real.mkdir()
            alias = root / "profiles"
            alias.symlink_to(real, target_is_directory=True)
            with mock.patch.object(P, "__file__", str(root / "loader.py")):
                with self.assertRaisesRegex(P.ProfileRefusal, "directory"):
                    P.load_release()
            alias.unlink()
            alias.mkdir()
            name, _ = P.RELEASES[self.version]
            (alias / name).write_bytes(b"x" * (P.PROFILE_LIMIT + 1))
            with mock.patch.object(P, "__file__", str(root / "loader.py")):
                with self.assertRaisesRegex(P.ProfileRefusal, "bounded"):
                    P.load_release()

    def test_ci_environment_has_fixed_roles_and_no_line_injection(self):
        env = P.environment(self.profile)
        self.assertEqual(env["ARCHIVE_BYTES"], "7217754")
        self.assertEqual(env["RUNTIME_MEMBERS"], "63")
        self.assertEqual(env["NOTICE_TEXTS"], "58")
        self.assertEqual(env["RELEASE_PROFILE_SHA256"], self.profile["profile_sha256"])
        self.profile["version"] = "v0.1.0-preview.16\nATTACK=1"
        with self.assertRaisesRegex(P.ProfileRefusal, "newline"):
            P.environment(self.profile)

    def v4_document(self):
        document = copy.deepcopy(self.document)
        document["bundle"] = {"schema": "gooo/service-grant-mcp-native-bundle/v4",
                              "runtime_members": 68, "notice_texts": 62}
        document["reference_packet"] = {"name": "gooo-mcp-saved-reference-linux-amd64.tar.gz",
                                        "bytes": 100, "sha256": "a" * 64}
        document["provenance"]["prov:wasDerivedFrom"].append({"@id": "urn:sha256:" + "a" * 64})
        return document

    def test_v4_reference_role_is_required_bounded_and_source_linked(self):
        document = self.v4_document()
        self.assertEqual(P._validate(document, self.version), document)
        for key, value in [("name", "../other"), ("bytes", True), ("bytes", (16 << 20) + 1),
                           ("sha256", "0" * 63)]:
            invalid = copy.deepcopy(document)
            invalid["reference_packet"][key] = value
            with self.subTest(key=key), self.assertRaises(P.ProfileRefusal):
                P._validate(invalid, self.version)
        del document["reference_packet"]
        with self.assertRaisesRegex(P.ProfileRefusal, "requires"):
            P._validate(document, self.version)

    def test_legacy_profile_cannot_gain_unreviewed_reference_material(self):
        document = self.v4_document()
        document["bundle"] = copy.deepcopy(self.document["bundle"])
        with self.assertRaisesRegex(P.ProfileRefusal, "legacy"):
            P._validate(document, self.version)

    def test_legacy_environment_does_not_activate_reference_downloads(self):
        env = P.environment(self.profile)
        self.assertEqual(env["HAS_NATIVE_REFERENCES"], "false")
        self.assertEqual(env["REFERENCE_PACKET_URL"], "")
        self.assertEqual(env["REFERENCE_PACKET_BYTES"], "")
        self.assertEqual(env["NATIVE_GO_SEMVER"], "1.27.3")


class PublicOCIConsumer(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        spec = importlib.util.spec_from_file_location(
            "profile_consumer_under_test", Path(__file__).with_name("service_grant_mcp_oci.py"))
        cls.oci = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(cls.oci)

    def test_selected_profile_cannot_invent_a_native_layout(self):
        p = self.oci.RELEASE.load_release()
        p["bundle"].update(runtime_members=68, notice_texts=62)
        with mock.patch.object(self.oci.RELEASE, "load_release", return_value=p):
            with self.assertRaisesRegex(self.oci.CONTEXT.Refusal, "known archive contract"):
                self.oci.release_profile(p["version"])

    def test_profile_cli_json_and_ci_env_are_same_selected_release(self):
        from contextlib import redirect_stdout
        output = io.StringIO()
        with redirect_stdout(output):
            self.oci.main(["profile", "--version", "v0.1.0-preview.16"])
        d = json.loads(output.getvalue())
        output = io.StringIO()
        with redirect_stdout(output):
            self.oci.main(["profile", "--version", "", "--format", "github-env"])
        env = dict(line.split("=", 1) for line in output.getvalue().splitlines())
        self.assertEqual(env, self.oci.RELEASE.environment(d))
        self.assertEqual(env["NATIVE_SOURCE"], "fceb89f8b53eabd6687d16258d7cee89853c2cfe")

    def test_workflow_binds_acquisition_publication_and_readback_to_the_profile(self):
        workflow = (Path(__file__).resolve().parents[1] /
                    ".github/workflows/service-grant-mcp-oci.yml").read_text()
        self.assertEqual(workflow.count('name: Load the reviewed native release profile'), 3)
        self.assertEqual(workflow.count('--format github-env >> "$GITHUB_ENV"'), 3)
        self.assertEqual(workflow.count('--max-filesize "$ARCHIVE_BYTES"'), 2)
        self.assertEqual(workflow.count('            "$ARCHIVE_URL"'), 2)
        self.assertIn("assert m['release_profile_sha256'] == os.environ['RELEASE_PROFILE_SHA256']", workflow)
        self.assertIn("assert report['release']['profile_sha256'] == os.environ['RELEASE_PROFILE_SHA256']", workflow)
        self.assertIn("len(report['checked_files']) == int(os.environ['RUNTIME_MEMBERS'])", workflow)
        self.assertIn("report['context']['notice_texts'] == int(os.environ['NOTICE_TEXTS'])", workflow)
        self.assertNotIn("--max-filesize 7217754", workflow)
        self.assertNotIn("NATIVE_SOURCE: fceb89", workflow)

    def test_existing_registry_descriptor_smoke_does_not_claim_a_new_profile(self):
        workflow = (Path(__file__).resolve().parents[1] /
                    ".github/workflows/service-grant-mcp-oci.yml").read_text()
        job = workflow.split("  registry-install-candidate:\n", 1)[1]
        self.assertIn("    needs: verify\n", job)
        self.assertIn("needs.verify.outputs.native-version == 'v0.1.0-preview.16'", job)

    def test_legacy_release_refuses_reference_preparation_without_opening_packet(self):
        with mock.patch.object(self.oci, "reviewed_inputs", return_value=(self.oci.DEFAULT, Path("native"))):
            with mock.patch.object(self.oci.REFERENCES, "prepare") as materialize:
                with self.assertRaisesRegex(self.oci.CONTEXT.Refusal, "no v4"):
                    self.oci.prepare_references("native", "ignored", "ignored", "v0.1.0-preview.16", "packet", "out")
                materialize.assert_not_called()

    def test_reference_preparer_uses_reviewed_packet_anchor(self):
        profile = copy.deepcopy(self.oci.DEFAULT)
        profile["bundle"]["schema"] = self.oci.CONTEXT.SNAPSHOT.SCHEMA
        profile["reference_packet"] = {"name": "gooo-mcp-saved-reference-linux-amd64.tar.gz",
                                       "sha256": "a" * 64, "bytes": 100}
        with mock.patch.object(self.oci, "reviewed_inputs", return_value=(profile, Path("native"))):
            with mock.patch.object(self.oci.REFERENCES, "prepare", return_value={"synthetic": True}) as materialize:
                result = self.oci.prepare_references("native", "ignored", "ignored", "test", "packet", "out")
                self.assertEqual(result, {"synthetic": True})
                materialize.assert_called_once_with(Path("packet"), "a" * 64, 100, Path("native"),
                    profile["archive"]["sha256"], profile["native_source"], Path("out"))

    def test_workflow_reuses_original_locations_and_full_client_in_both_image_paths(self):
        workflow = (Path(__file__).resolve().parents[1] /
                    ".github/workflows/service-grant-mcp-oci.yml").read_text()
        self.assertEqual(workflow.count("name: Acquire and materialize only the reviewed original reference packet"), 2)
        self.assertEqual(workflow.count("--out \"$PWD/out/service-grant-mcp\""), 2)
        self.assertEqual(workflow.count("--format github-env >> \"$GITHUB_ENV\""), 3)
        self.assertEqual(workflow.count("--max-filesize \"$REFERENCE_PACKET_BYTES\""), 2)
        self.assertEqual(workflow.count("mapfile -d '' -t reference_args"), 2)
        self.assertEqual(workflow.count('"${reference_args[@]}"'), 2)
        self.assertEqual(workflow.count("name: Remove only byte-verified owned original reference locations"), 2)
        self.assertIn("always() && env.HAS_NATIVE_REFERENCES == 'true'", workflow)
        self.assertNotIn("eval ", workflow)


if __name__ == "__main__":
    unittest.main()
