"""Strict request/evidence checks using synthetic receipts; never run a compiler."""
import copy
import hashlib
import tempfile
import importlib.util
import json
from pathlib import Path
import unittest
from unittest import mock

ROOT = Path(__file__).resolve().parent.parent
SPEC = importlib.util.spec_from_file_location("literal_text_inputs", ROOT / "tools/check-literal-text-inputs.py")
PROBE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PROBE)


def receipt_fixture(case):
    path = Path("/frozen/" + case["entry"].rsplit(":", 1)[1] + ".workspace.json")
    manifest_raw = b"synthetic manifest bytes"
    error = case["expected_diagnostic"]
    if case["expected_rejection_stage"] == "INPUT_DOMAIN":
        activity = case["entry"].rsplit(":", 1)[1]
        key = "GoooPackage1Activity" + activity + ".input" + str(case["position"])
        error = "generate workspace activity bodies: case 0: " + ('input "' + key + '": ' if case["position"] is not None else "") + error
    receipt = {"schema": PROBE.RECEIPT_SCHEMA, "decision": "FAIL_CLOSED", "manifest": path.as_posix(),
               "manifest_digest": PROBE.digest(manifest_raw), "inputs_digest": PROBE.digest(case["raw"]), "error": error}
    return receipt, path, manifest_raw


class LiteralMalformedRequestTests(unittest.TestCase):
    def setUp(self):
        self.cases = list(PROBE.negative_inputs())

    def test_all_three_entries_and_both_arguments_have_independent_rejection_rosters(self):
        self.assertEqual(len(self.cases), 224)
        self.assertEqual(len({case["name"] for case in self.cases}), 224)
        for entry in PROBE.ALL_ENTRIES:
            selected = [case for case in self.cases if case["entry"] == entry]
            self.assertEqual(len(selected), 28)
            for position in range(2):
                selected_port = [case for case in selected if case["position"] == position]
                self.assertEqual(len(selected_port), 12)
                self.assertEqual(sum("duplicate" in case["name"] for case in selected_port), 2)
                self.assertEqual(sum("1025" in case["name"] for case in selected_port), 2)
            self.assertEqual(sum(case["expected_rejection_stage"] == "INPUT_JSON_DECODE" for case in selected), 7)
        for case in self.cases:
            with self.subTest(case=case["name"]), self.assertRaises(ValueError):
                PROBE.validate_inputs(case["raw"], case["entry"])

    def test_oversize_roster_counts_utf8_bytes_and_preserves_the_other_argument(self):
        for case in self.cases:
            if "1025" not in case["name"]:
                continue
            row = json.loads(case["raw"])["inputs"][0]
            key = case["entry"] + ".input" + str(case["position"])
            other = case["entry"] + ".input" + str(1 - case["position"])
            with self.subTest(case=case["name"]):
                self.assertEqual(len(row[key].encode("utf-8")), 1025)
                self.assertEqual(row[other], "tic" if case["position"] == 0 else "diagnostic")
                if "multibyte" in case["name"]:
                    self.assertEqual(len(row[key]), 513)

    def test_every_wrong_type_is_present_in_each_argument_without_coercion(self):
        for entry in PROBE.ALL_ENTRIES:
            for position in range(2):
                names = {case["name"].split("-", 1)[1].rsplit("-input", 1)[0]
                         for case in self.cases if case["entry"] == entry and case["position"] == position}
                self.assertTrue({"null", "false", "true", "integer", "fraction", "array", "object"} <= names)

    def test_supplied_example_and_exact_multibyte_boundary_are_in_domain(self):
        rows = PROBE.validate_inputs((ROOT / "examples/literal-text-report/inputs.json").read_bytes(), PROBE.ENTRIES[-1])
        self.assertEqual(len(rows), 24)
        for entry in PROBE.ALL_ENTRIES:
            raw = PROBE.encode({"schema": PROBE.SCHEMA, "inputs": [{entry + ".input0": "é" * 512,
                                                                      entry + ".input1": "🙂" * 256}]})
            accepted = PROBE.validate_inputs(raw, entry)[0]
            self.assertEqual(len(accepted[entry + ".input0"].encode()), 1024)
            self.assertEqual(len(accepted[entry + ".input1"].encode()), 1024)

    def test_wire_utf8_and_surrogates_are_preflight_issues_not_actual_cli_claims(self):
        entry = PROBE.ENTRIES[0]
        raw = PROBE.encode({"schema": PROBE.SCHEMA, "inputs": [{entry + ".input0": "PLACEHOLDER", entry + ".input1": ""}]})
        for replacement in (b"\xff", b"\\ud800"):
            with self.subTest(replacement=replacement), self.assertRaises(ValueError):
                PROBE.validate_inputs(raw.replace(b"PLACEHOLDER", replacement), entry)
        self.assertFalse(any("surrogate" in case["name"] or "wire-utf8" in case["name"] for case in self.cases))


class LiteralRejectionReceiptTests(unittest.TestCase):
    def test_each_expected_route_keeps_absent_runtime_not_reported(self):
        for case in PROBE.negative_inputs():
            receipt, path, raw = receipt_fixture(case)
            with self.subTest(case=case["name"]):
                observed = PROBE.inspect_rejection(case, PROBE.encode(receipt), 1, False, path, raw)
                self.assertTrue(observed["rejection_verified"])
                self.assertFalse(observed["result_present"])
                self.assertFalse(observed["runtime_present"])
                for key in ("actual_runtime_stage", "observed_counters", "observed_build", "observed_runs"):
                    self.assertEqual(observed[key], "NOT_REPORTED")

    def test_valid_rejection_label_cannot_hide_wrong_request_route_or_runtime(self):
        case = next(iter(PROBE.negative_inputs()))
        receipt, path, raw = receipt_fixture(case)
        mutations = {
            "schema": lambda r: r.update(schema="wrong"),
            "decision": lambda r: r.update(decision="OBSERVED"),
            "manifest path": lambda r: r.update(manifest="/other.workspace.json"),
            "manifest digest": lambda r: r.update(manifest_digest=PROBE.digest(b"other")),
            "request digest": lambda r: r.update(inputs_digest=PROBE.digest(b"other")),
            "diagnostic": lambda r: r.update(error="unrelated failure"),
            "route": lambda r: r.update(error=r["error"].replace("generate workspace activity bodies: case 0:", "decode inputs:")),
            "case index": lambda r: r.update(error=r["error"].replace("case 0:", "case 1:")),
            "argument": lambda r: r.update(error=r["error"].replace(".input0", ".input1")),
            "empty result": lambda r: r.update(result={}),
            "zero counters": lambda r: r.update(result={"runtime": {"stage": "COMPLETE", "finite_total": 0,
                                                                     "finite_passed": 0, "model_calls": 0}}),
        }
        for name, mutate in mutations.items():
            altered = copy.deepcopy(receipt)
            mutate(altered)
            with self.subTest(name=name), self.assertRaises(ValueError):
                PROBE.inspect_rejection(case, PROBE.encode(altered), 1, False, path, raw)
        for exit_code, timed_out in ((0, False), (2, False), (None, True), (1, True)):
            with self.subTest(exit_code=exit_code, timed_out=timed_out), self.assertRaises(ValueError):
                PROBE.inspect_rejection(case, PROBE.encode(receipt), exit_code, timed_out, path, raw)

    def test_decode_failure_cannot_be_relabelled_as_a_later_body_failure(self):
        case = next(c for c in PROBE.negative_inputs() if c["expected_rejection_stage"] == "INPUT_JSON_DECODE")
        receipt, path, raw = receipt_fixture(case)
        receipt["error"] = "generate workspace activity bodies: " + receipt["error"]
        with self.assertRaisesRegex(ValueError, "precede body generation"):
            PROBE.inspect_rejection(case, PROBE.encode(receipt), 1, False, path, raw)

    def test_duplicate_receipt_keys_are_not_collapsed(self):
        case = next(iter(PROBE.negative_inputs()))
        receipt, path, raw = receipt_fixture(case)
        data = PROBE.encode(receipt).replace(b'"decision":"FAIL_CLOSED"',
                                           b'"decision":"OBSERVED","\\u0064ecision":"FAIL_CLOSED"')
        with self.assertRaisesRegex(ValueError, "duplicate"):
            PROBE.inspect_rejection(case, data, 1, False, path, raw)

    def test_clean_compiler_pin_is_independent_of_request_labels(self):
        lock = {"compiler_revision": PROBE.COMPILER, "compiler_version": PROBE.VERSION, "go_version": PROBE.GO}
        build = {"schema": "gooo/build-identity/v1", "version": PROBE.VERSION, "compiler_source_sha": PROBE.COMPILER,
                 "vcs_revision": PROBE.COMPILER, "source_status": "CLEAN_VCS", "vcs_modified": "false",
                 "go_version": PROBE.GO, "native_go_required": PROBE.GO}
        PROBE.validate_identity(build, lock)
        for field in build:
            altered = dict(build, **{field: "different"})
            with self.subTest(field=field), self.assertRaises(ValueError):
                PROBE.validate_identity(altered, lock)
        for field in lock:
            with self.subTest(lock_field=field), self.assertRaises(ValueError):
                PROBE.validate_identity(build, dict(lock, **{field: "different"}))

    def test_runner_environment_does_not_reuse_model_or_toolchain_configuration(self):
        with mock.patch.dict(PROBE.os.environ, {"GOOO_LAYA_URL": "external", "OPENAI_API_KEY": "secret",
                                                "GOTOOLCHAIN": "auto", "GOWORK": "/other/go.work"}):
            env = PROBE.clean_environment()
        self.assertNotIn("GOOO_LAYA_URL", env)
        self.assertNotIn("OPENAI_API_KEY", env)
        self.assertEqual({key: env[key] for key in ("GOTOOLCHAIN", "GOWORK", "GOENV", "GOPROXY", "GOSUMDB")},
                         {"GOTOOLCHAIN": "local", "GOWORK": "off", "GOENV": "off", "GOPROXY": "off", "GOSUMDB": "off"})


# Frozen pre-Cut generator. Its independent three-entry byte roster is used only
# in these synthetic tests; it does not invoke a compiler or a subprocess.
def frozen_legacy_negative_inputs():
    ENTRIES = ("go/strings:Index", "go/strings:Count", "app/literal-text-report:Report")
    SCHEMA = "gooo/body-composition-inputs/v1"
    def encode(value):
        return json.dumps(value, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
    """An ordered finite roster; request count is independent of oracle cases."""
    wrong = (("null", None), ("false", False), ("true", True), ("integer", 0),
             ("fraction", 1.5), ("array", []), ("object", {}))
    for entry in ENTRIES:
        prefix = entry.rsplit(":", 1)[1].lower()
        base = {entry + ".input0": "diagnostic", entry + ".input1": "tic"}

        def case(name, row, diagnostic, stage="INPUT_DOMAIN", position=None, raw=None):
            return {"name": prefix + "-" + name, "entry": entry, "position": position,
                    "expected_diagnostic": diagnostic, "expected_rejection_stage": stage,
                    "raw": raw if raw is not None else encode({"schema": SCHEMA, "inputs": [row]}) + b"\n"}

        for position in range(2):
            key = entry + ".input" + str(position)
            for name, value in (("ascii-1025", "a" * 1025), ("multibyte-1025", "é" * 512 + "a"), *wrong):
                row = dict(base, **{key: value})
                diagnostic = "explicit Text scalar required" if value is None else "Text requires a string of at most 1024 UTF-8 bytes"
                yield case(name + "-input" + str(position), row, diagnostic, position=position)
            row = dict(base)
            del row[key]
            yield case("missing-input" + str(position), row, "requires an explicit value", position=position)
            for name, duplicate_key in (("duplicate", encode(key)), ("duplicate-escaped", encode(key).replace(b"input", b"\\u0069nput"))):
                token = encode(key) + b":" + encode(base[key])
                duplicate = token + b"," + duplicate_key + b":" + encode(base[key])
                raw = encode({"schema": SCHEMA, "inputs": [base]}).replace(token, duplicate, 1) + b"\n"
                yield case(name + "-input" + str(position), base, "duplicate or invalid JSON key", "INPUT_JSON_DECODE", position, raw)
        yield case("extra-input2", dict(base, **{entry + ".input2": "extra"}),
                   "inputs and nonempty expectations must name declared activities")
        raw = encode({"schema": "gooo/wrong-inputs/v1", "inputs": [base]}) + b"\n"
        yield case("wrong-schema", base, "composition inputs require schema and 1..128 input rows", "INPUT_JSON_DECODE", raw=raw)
        for field, escaped in (("schema", b'"\\u0073chema"'), ("inputs", b'"\\u0069nputs"')):
            value = SCHEMA if field == "schema" else [base]
            token = encode(field) + b":" + encode(value)
            raw = encode({"schema": SCHEMA, "inputs": [base]}).replace(token, token + b"," + escaped + b":" + encode(value), 1) + b"\n"
            yield case("duplicate-" + field, base, "duplicate or invalid JSON key", "INPUT_JSON_DECODE", raw=raw)


class LiteralCutAppendOnlyRequestTests(unittest.TestCase):
    def test_legacy84_raw_hashes_and_order_are_an_unchanged_prefix(self):
        original = list(frozen_legacy_negative_inputs())
        current = list(PROBE.negative_inputs())
        self.assertEqual(len(original), 84)
        self.assertEqual(len(current), 224)
        self.assertEqual([row["name"] for row in current[:84]], [row["name"] for row in original])
        for before, after in zip(original, current[:84]):
            with self.subTest(case=before["name"]):
                self.assertEqual(before, after)
                self.assertEqual(hashlib.sha256(before["raw"]).hexdigest(), hashlib.sha256(after["raw"]).hexdigest())
        self.assertEqual(len({row["name"] for row in current}), 224)
        self.assertTrue(all(row["entry"] in PROBE.CUT_ENTRIES for row in current[84:196]))
        self.assertEqual(current[84]["entry"], "go/strings:Cut")
        self.assertEqual(current[195]["entry"], "app/literal-cut-report:Report")
        self.assertTrue(all(row["name"].startswith("cutreport-") for row in current if row["entry"] == "app/literal-cut-report:Report"))
        for entry in PROBE.CUT_ENTRIES:
            selected = [row for row in current[84:] if row["entry"] == entry]
            self.assertEqual(len(selected), 28)
            for position in range(2):
                self.assertEqual(sum(row["position"] == position for row in selected), 12)
            self.assertEqual(sum(row["expected_rejection_stage"] == "INPUT_JSON_DECODE" for row in selected), 7)

    def test_new_entries_keep_all_original_rejection_binding_checks(self):
        for entry in PROBE.CUT_ENTRIES:
            case = next(row for row in PROBE.negative_inputs() if row["entry"] == entry)
            receipt, path, raw = receipt_fixture(case)
            for label, change in {
                "manifest": lambda value: value.update(manifest_digest=PROBE.digest(b"other manifest")),
                "input": lambda value: value.update(inputs_digest=PROBE.digest(b"other input")),
                "path": lambda value: value.update(manifest="/other/Report.workspace.json"),
                "route": lambda value: value.update(error="decode inputs: " + value["error"]),
                "result": lambda value: value.update(result={"runtime":{"stage":"COMPLETE","model_calls":0}}),
            }.items():
                altered = copy.deepcopy(receipt)
                change(altered)
                with self.subTest(entry=entry, alteration=label), self.assertRaises(ValueError):
                    PROBE.inspect_rejection(case, PROBE.encode(altered), 1, False, path, raw)


class LiteralCutSeparateManifestTests(unittest.TestCase):
    def frozen(self):
        frozen = {name: ("synthetic source " + name + "\n").encode() for name in PROBE.FROZEN_FILES}
        old = {"schema":"gooo/package-workspace-manifest/v1",
               "entry":{"package_path":"app/literal-text-report","activity":"Report"},
               "packages":[{"path":"go/strings","sources":list(PROBE.PORT_SOURCES)},
                           {"path":"app/literal-text-report","sources":[PROBE.SOURCES[-1]]}]}
        new = {"schema":"gooo/package-workspace-manifest/v1",
               "entry":{"package_path":"app/literal-cut-report","activity":"Report"},
               "packages":[{"path":"go/strings","sources":list(PROBE.CUT_PORT_SOURCES)},
                           {"path":"app/literal-cut-report","sources":[PROBE.CUT_SOURCES[-1]]}]}
        frozen["literal-text-report.workspace.json"] = (json.dumps(old, indent=2)+"\n").encode()
        frozen["literal-cut-report.workspace.json"] = (json.dumps(new, indent=2)+"\n").encode()
        frozen["if-match.workspace.json"] = PROBE.encode({"schema":"gooo/package-workspace-manifest/v1", "entry":{"package_path":"go/net/http/precondition","activity":"IfMatchCondition"}, "packages":[{"path":"go/net/http/precondition","sources":[PROBE.IF_MATCH_SOURCE]}]})+b"\n"
        return frozen

    def test_old_three_source_bytes_and_new_closures_are_separate(self):
        frozen = self.frozen()
        before = dict(frozen)
        manifests = PROBE.trusted_manifests(frozen)
        self.assertEqual(list(manifests), list(PROBE.ALL_ENTRIES))
        self.assertEqual(len({name for name, raw in manifests.values()}), 8)
        self.assertEqual(manifests["app/literal-text-report:Report"], ("literal-text-report.workspace.json",frozen["literal-text-report.workspace.json"]))
        self.assertEqual(manifests["app/literal-cut-report:Report"], ("literal-cut-report.workspace.json",frozen["literal-cut-report.workspace.json"]))
        for entry, (name, raw) in manifests.items():
            manifest = json.loads(raw)
            self.assertEqual(manifest["entry"], dict(zip(("package_path","activity"),entry.rsplit(":",1))))
            if entry == PROBE.IF_MATCH_ENTRY:
                self.assertEqual(manifest["packages"], [{"path":"go/net/http/precondition","sources":[PROBE.IF_MATCH_SOURCE]}])
                continue
            expected = list(PROBE.PORT_SOURCES if entry in PROBE.ENTRIES else PROBE.CUT_PORT_SOURCES)
            self.assertEqual(manifest["packages"][0], {"path":"go/strings","sources":expected})
            if entry.rsplit(":",1)[1] == "Report":
                expected_source = PROBE.SOURCES[-1] if entry in PROBE.ENTRIES else PROBE.CUT_SOURCES[-1]
                self.assertEqual(manifest["packages"][1], {"path":entry.rsplit(":",1)[0],"sources":[expected_source]})
            else:
                self.assertEqual(len(manifest["packages"]), 1)
        self.assertEqual(frozen, before)
        for entry in PROBE.ENTRIES[:2]:
            package, activity = entry.rsplit(":",1)
            original_raw = PROBE.encode({"schema":"gooo/package-workspace-manifest/v1", "entry":{"package_path":package,"activity":activity}, "packages":[{"path":"go/strings","sources":list(PROBE.PORT_SOURCES)}]})+b"\n"
            self.assertEqual(manifests[entry][1], original_raw)

    def test_missing_new_entry_or_source_cannot_silently_reduce_the_gate(self):
        frozen = self.frozen()
        for missing in ("literal-cut-report.workspace.json","ports/strings/cut.gooo",PROBE.CUT_SOURCES[-1],"if-match.workspace.json",PROBE.IF_MATCH_SOURCE):
            altered = dict(frozen)
            del altered[missing]
            with self.subTest(missing=missing), self.assertRaises(ValueError):
                PROBE.trusted_manifests(altered)
        with mock.patch.object(PROBE,"ALL_ENTRIES",PROBE.ALL_ENTRIES[:-1]), self.assertRaises(ValueError):
            PROBE.trusted_manifests(frozen)
        with mock.patch.object(PROBE,"CUT_PORT_SOURCES",PROBE.PORT_SOURCES), self.assertRaises(ValueError):
            PROBE.trusted_manifests(frozen)

    def test_wrong_or_cross_contaminated_manifest_closures_refuse_before_any_cli(self):
        for name, mode in (("literal-text-report.workspace.json","old-contamination"),
                           ("literal-cut-report.workspace.json","cut-missing"),
                           ("literal-cut-report.workspace.json","wrong-consumer"),
                           ("literal-cut-report.workspace.json","wrong-entry")):
            frozen = self.frozen()
            value = json.loads(frozen[name])
            if mode == "old-contamination":
                value["packages"][0]["sources"].append("ports/strings/cut.gooo")
            elif mode == "cut-missing":
                value["packages"][0]["sources"].remove("ports/strings/cut.gooo")
            elif mode == "wrong-consumer":
                value["packages"][1]["sources"] = [PROBE.SOURCES[-1]]
            else:
                value["entry"]["package_path"] = "app/literal-text-report"
            frozen[name] = PROBE.encode(value)+b"\n"
            with tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)/"root"
                root.mkdir()
                for file, raw in frozen.items():
                    target = root/file
                    target.parent.mkdir(parents=True,exist_ok=True)
                    target.write_bytes(raw)
                with self.subTest(mode=mode), mock.patch.object(PROBE.subprocess,"run",side_effect=AssertionError("no CLI allowed")) as run:
                    with self.assertRaises(ValueError):
                        PROBE.observe(root,Path(temporary)/"new-output",Path("/never-run-compiler"))
                    run.assert_not_called()
                self.assertEqual((root/name).read_bytes(),frozen[name])


class IfMatchAppendOnlyCLIRequestTests(unittest.TestCase):
    def test_original196_then_exact_new28_without_wire_unicode_claim(self):
        rows=list(PROBE.negative_inputs())
        self.assertEqual(len(rows),224)
        self.assertEqual([row['entry'] for row in rows[:196]], [entry for entry in (*PROBE.ENTRIES,*PROBE.CUT_ENTRIES) for _ in range(28)])
        self.assertEqual([row['entry'] for row in rows[196:]], [PROBE.IF_MATCH_ENTRY]*28)
        self.assertEqual(len({row['name'] for row in rows}),224)
        self.assertFalse(any('surrogate' in row['name'] or 'wire-utf8' in row['name'] for row in rows))

if __name__ == "__main__":
    unittest.main()
