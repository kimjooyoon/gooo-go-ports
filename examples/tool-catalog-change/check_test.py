#!/usr/bin/env python3
# Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
"""Refusal controls using the actual records from verify.sh, with no reruns."""
import argparse
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest

HERE = Path(__file__).resolve().parent
SOURCE_ROOT = HERE.parents[1]
RECORDS = None
spec = importlib.util.spec_from_file_location("catalog_check", HERE / "check.py")
checker = importlib.util.module_from_spec(spec)
spec.loader.exec_module(checker)
generator_spec = importlib.util.spec_from_file_location("catalog_inputs", HERE / "make-inputs.py")
generator = importlib.util.module_from_spec(generator_spec)
generator_spec.loader.exec_module(generator)


class SavedRecordRefusals(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="gooo-catalog-refusal-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name) / "source"
        paths = {
            "upstream.json",
            "go.mod",
            "tool-catalog-change.workspace.json",
            "ports/gogcli/tool_selection.gooo",
            "examples/tool-catalog-change/main.gooo",
            "examples/tool-catalog-change/make-inputs.py",
        }
        paths.update(str(path.relative_to(SOURCE_ROOT)) for path in HERE.glob("*.json"))
        pins = json.loads((HERE / "source-pin.json").read_bytes())
        paths.update(binding["local_path"] for binding in pins["files"])
        for relative in sorted(paths):
            target = self.root / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(SOURCE_ROOT / relative, target)
        checker.ROOT = self.root
        checker.HERE = self.root / "examples/tool-catalog-change"
        generator.ROOT = self.root
        self.inputs = Path(self.temp.name) / "inputs"
        shutil.copytree(RECORDS / "inputs", self.inputs)
        self.replay = Path(self.temp.name) / "replay.json"
        self.replay.write_bytes((RECORDS / "catalog-replay.json").read_bytes())

    def check(self):
        return checker.check(
            RECORDS / "catalog-execution.json",
            self.replay,
            RECORDS / "catalog-reference.json",
            RECORDS / "identity.json",
            RECORDS / "source-catalog.json",
            self.inputs,
        )

    def change_replay(self, mutate):
        value = json.loads(self.replay.read_bytes())
        mutate(value)
        self.replay.write_text(json.dumps(value) + "\n")

    def test_replay_parent_is_not_a_label(self):
        self.change_replay(lambda value: value.update(replayed_from_sha256="sha256:" + "0" * 64))
        with self.assertRaisesRegex(checker.Refused, "replay parent differs"):
            self.check()

    def test_integer_cannot_replace_false(self):
        self.change_replay(lambda value: value["result"]["runtime"]["traces"][0]["deliveries"][0]["actual"].update(removed=0))
        with self.assertRaisesRegex(checker.Refused, "wrong type"):
            self.check()

    def test_empty_result_field_cannot_be_omitted(self):
        self.change_replay(lambda value: value["result"]["runtime"]["traces"][0]["deliveries"][0]["actual"].pop("before_capability"))
        with self.assertRaisesRegex(checker.Refused, "missing or extra fields"):
            self.check()

    def test_unsupported_authority_field_is_not_a_decision(self):
        self.change_replay(lambda value: value["result"]["runtime"]["traces"][0]["deliveries"][0]["actual"].update(authorized=True))
        with self.assertRaisesRegex(checker.Refused, "missing or extra fields"):
            self.check()

    def test_raw_source_crlf_change_cannot_reuse_lf_receipt(self):
        path = self.root / "ports/gogcli/tool_selection.gooo"
        before = path.read_bytes()
        self.assertNotIn(b"\r", before)
        path.write_bytes(before.replace(b"\n", b"\r\n"))
        with self.assertRaisesRegex(checker.Refused, "source bytes differ"):
            self.check()

    def test_saved_catalog_cannot_replace_fresh_source_projection(self):
        path = checker.HERE / "source-catalogs.json"
        value = json.loads(path.read_bytes())
        value["after_metadata"][0]["name"] += "_changed"
        path.write_text(json.dumps(value) + "\n")
        with self.assertRaisesRegex(checker.Refused, "fresh source-catalog projection"):
            self.check()

    def test_root_go_fact_change_cannot_relabel_old_profile(self):
        path = self.root / "upstream.json"
        value = json.loads(path.read_bytes())
        self.assertNotEqual(value["go_version"], "go1.99.0")
        value["go_version"] = "go1.99.0"
        path.write_text(json.dumps(value) + "\n")
        with self.assertRaisesRegex(checker.Refused, "root module and toolchain pins differ"):
            self.check()

    def test_wrong_prov_namespace_cannot_relabel_generation(self):
        path = self.inputs / "generation.provenance.jsonld"
        value = json.loads(path.read_bytes())
        value["@context"]["prov"] = "urn:wrong:"
        path.write_text(json.dumps(value) + "\n")
        with self.assertRaisesRegex(checker.Refused, "JSON-LD context"):
            self.check()

    def test_last_profile_type_error_generates_no_output(self):
        path = checker.HERE / "case-facts.json"
        value = json.loads(path.read_bytes())
        value["controls"][-1]["after_readonly"] = "false"
        path.write_text(json.dumps(value) + "\n")
        out = Path(self.temp.name) / "refused-inputs"
        with self.assertRaisesRegex(ValueError, "wrong fact type"):
            generator.generate(out)
        self.assertFalse(out.exists())

    def test_unknown_fact_field_generates_no_output(self):
        path = checker.HERE / "case-facts.json"
        value = json.loads(path.read_bytes())
        value["controls"][-1]["authorized"] = True
        path.write_text(json.dumps(value) + "\n")
        out = Path(self.temp.name) / "refused-inputs"
        with self.assertRaisesRegex(ValueError, "case fields differ"):
            generator.generate(out)
        self.assertFalse(out.exists())

    def test_existing_output_is_preserved(self):
        out = Path(self.temp.name) / "existing"
        out.mkdir()
        sentinel = out / "keep"
        sentinel.write_bytes(b"keep")
        with self.assertRaisesRegex(ValueError, "output must be fresh"):
            generator.generate(out)
        self.assertEqual(list(out.iterdir()), [sentinel])
        self.assertEqual(sentinel.read_bytes(), b"keep")

    def test_duplicate_fact_key_is_refused_before_output(self):
        path = checker.HERE / "case-facts.json"
        raw = path.read_text()
        old = '"name": "gmail_search"'
        self.assertIn(old, raw)
        path.write_text(raw.replace(old, old + ', "name": "gmail_search"', 1))
        out = Path(self.temp.name) / "refused-inputs"
        with self.assertRaisesRegex(ValueError, "duplicate JSON key"):
            generator.generate(out)
        self.assertFalse(out.exists())

    def test_repeat_bytes_have_distinct_generation_occurrences(self):
        first = Path(self.temp.name) / "first"
        second = Path(self.temp.name) / "second"
        generator.generate(first)
        generator.generate(second)
        for profile in ["catalog", "controls"]:
            for suffix in ["-inputs.json", "-oracle-inputs.json"]:
                self.assertEqual((first / (profile + suffix)).read_bytes(),
                                 (second / (profile + suffix)).read_bytes())
        one = json.loads((first / "generation.provenance.jsonld").read_bytes())
        two = json.loads((second / "generation.provenance.jsonld").read_bytes())
        self.assertFalse({row["@id"] for row in one["@graph"]} &
                         {row["@id"] for row in two["@graph"]})
        for graph in [one["@graph"], two["@graph"]]:
            for node in graph:
                for parent in node.get("prov:wasDerivedFrom", []):
                    self.assertEqual(set(parent), {"@id"})
                    self.assertNotEqual(parent["@id"], node["@id"])


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--records", type=Path, required=True)
    args, rest = parser.parse_known_args()
    RECORDS = args.records.resolve()
    unittest.main(argv=[__file__, *rest])
