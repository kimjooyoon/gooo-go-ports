#!/usr/bin/env python3
"""Guard the CI skip boundary using the workflow's explicit step structure.

This deliberately accepts only the current plain-scalar, fixed-indent step
layout. Unrecognized YAML layout fails rather than silently missing a guard.
It needs no YAML package on the CI runner.
"""

import hashlib
import re
import unittest
from pathlib import Path


WORKFLOW = Path(__file__).resolve().parent.parent / ".github/workflows/ci.yml"
FULL = "steps.change-scope.outputs.docs_only != 'true'"
DOCS = "steps.change-scope.outputs.docs_only == 'true'"
HEADER_SHA = "5cd59615e4403c288bd003ae603c7e55a813f42af0bbbb4715effebf8d093e9b"
# Reviewed browser delta: add UUID ParseCanonical smoke after literal search.
# Events, native skip guards and deployment dependencies stay byte-identical.
BROWSER_SHA = "a97b7ac0ba9703e23c1f8a3acc09eba372b77c70085723ce798b20deb6172afa"
COMMON = {
    "Classify the exact checked PR tree", "Check the Go observer",
    "Check pinned Unicode table generation",
    "Build and identify the pinned compiler from a clean checkout",
    "Check Gooo declarations",
}
HEAVY = {
    "Retain unsupported-body probes",
    "Compare the ports with Go and replay every batch",
    "Execute and replay the Unicode case inspector",
    "Execute and replay the alpha matte example",
    "Execute and verify fixed-point bounds example",
    "Exhaustively compare generated Unicode classifications",
    "Execute and replay the input-only importing example",
    "Execute and replay the byte-summary input-only example",
    "Execute and replay the byte-rotation input-only example",
    "Execute and replay Unicode inspector input-only examples",
    "Execute and replay the color inspector input-only example",
    "Execute and replay the YCbCr inspector input-only example",
    "Execute and replay the duration quantizer input-only example",
    "Execute and replay the whitespace inspector input-only example",
    "Execute and replay the rectangle crop planner",
    "Execute and replay the direction-independent drag crop planner",
    "Execute and replay the native FitsCrop example",
    "Observe malformed FitsCrop inputs through the actual CLI",
    "Execute and replay the literal text report",
    "Observe malformed literal Text inputs through the actual CLI",
    "Verify the offline decision-policy experiment",
}
UPLOADS = {"Upload offline policy experiment evidence", "Upload execution and diagnostic evidence"}
DOC_STEPS = {"Report documentation-only native scope", "Preserve documentation scope and compiler identity"}
NATIVE_COMMAND = re.compile(
    r"(?:\.gooo\s+package\s+(?:execute|replay)|\./cmd/(?:verify|check-unicode|check-literal-text-report)\b|"
    r"tools/probe\.sh|tools/check-(?:fits-crop|literal-text)-inputs\.py|decision-policy-pilot/run\.sh)"
)
LITERAL_COMMANDS = {
    "Execute and replay the literal text report": {
        './.gooo package execute --json --go "$(go env GOROOT)/bin/go" --inputs examples/literal-text-report/inputs.json literal-text-report.workspace.json > out/literal-text-report/execution.json',
        './.gooo package replay --json --go "$(go env GOROOT)/bin/go" --receipt out/literal-text-report/execution.json --inputs examples/literal-text-report/inputs.json literal-text-report.workspace.json > out/literal-text-report/replay.json',
        "go run ./cmd/check-literal-text-report --snapshot . --inputs examples/literal-text-report/inputs.json --execution out/literal-text-report/execution.json --replay out/literal-text-report/replay.json --compiler-build out/compiler-build.json --out out/literal-text-report/checked-report.json",
    },
    "Observe malformed literal Text inputs through the actual CLI": {
        "python3 -B tools/check-literal-text-inputs.py out/literal-text-input-rejections",
    },
}


def literal_commands(chunk):
    # Continuation joining is deliberately limited to the reviewed fixed layout.
    joined = chunk.replace("\\\n          ", " ")
    return [" ".join(line.split()) for line in
            re.findall(r"^          ((?:\./\.gooo|go run \./cmd/|python3 ).*)$", joined, re.M)]


def require(condition, message):
    if not condition:
        raise ValueError(message)


def field(chunk, name):
    values = re.findall(r"^        " + re.escape(name) + r": (.+)$", chunk, re.M)
    require(len(values) <= 1, "duplicate step field: " + name)
    return values[0] if values else None


def named_steps(text):
    start, end = text.index("  native-ports:\n"), text.index("  browser-demo:\n")
    native = text[start:end]
    header, steps_text = native.split("    steps:\n", 1)
    require(not re.search(r"^    if:", header, re.M), "native job must always report a result")
    starts = list(re.finditer(r"^      - (name|uses): (.+)$", steps_text, re.M))
    require(len(starts) == len(re.findall(r"^      - ", steps_text, re.M)), "unrecognized step layout")
    named = {}
    unnamed = []
    for index, match in enumerate(starts):
        chunk = steps_text[match.start():starts[index + 1].start() if index + 1 < len(starts) else len(steps_text)]
        if match[1] == "name":
            require(match[2] not in named, "duplicate step name")
            named[match[2]] = chunk
        else:
            unnamed.append(chunk)
    return named, unnamed


def validate(text):
    header = text[:text.index("jobs:\n")]
    browser = text[text.index("  browser-demo:\n"):]
    # These independently audited sections retain their previous exact behavior.
    require(hashlib.sha256(header.encode()).hexdigest() == HEADER_SHA, "events or permissions changed")
    require(hashlib.sha256(browser.encode()).hexdigest() == BROWSER_SHA, "browser/deployment behavior changed")
    named, unnamed = named_steps(text)
    require(len(HEAVY) == 21, "full-scope native step count changed")
    require(set(named) == COMMON | HEAVY | UPLOADS | DOC_STEPS, "step roster changed without guard review")
    require(len(unnamed) == 2 and all(field(c, "if") is None for c in unnamed), "checkout/Go setup became conditional")
    require("fetch-depth: 0" in unnamed[0], "exact PR parents must be available")
    require("go-version: '1.27.2'" in unnamed[1], "Go toolchain pin changed")
    for name in COMMON:
        require(field(named[name], "if") is None, "common check became conditional: " + name)
    for name in HEAVY:
        require(field(named[name], "if") == FULL, "missing full-execution guard: " + name)
    for name in UPLOADS:
        require(field(named[name], "if") == "always() && " + FULL, "native artifact can upload on docs path: " + name)
    comparison = named["Compare the ports with Go and replay every batch"]
    require("go run ./cmd/verify --workers 2 --out out/ci" in comparison
            and ".native_suite_workers == 2 and" in comparison,
            "full native CI must exercise and bind the two-worker setting")
    for token in (".ported_functions == 42 and .consumer_programs == 18 and .consumer_activities == 20 and",
                  "(.runs | length) == 62 and",
                  ".cases == 159781 and .passed == 159781 and",
                  "([.runs[].cases] | add) == 159781 and ([.runs[].batches] | add) == 2332 and",
                  "([.runs[:59][].cases] | add) == 131842 and ([.runs[:59][].batches] | add) == 1777 and"):
        require(token in comparison, "integrated roster or original 59-suite preservation missing: " + token)
    for index, package, activity, batches in ((59, "go/strings", "Index", 137),
                                               (60, "go/strings", "Count", 137),
                                               (61, "app/literal-text-report", "Report", 281)):
        for token in (f'.runs[{index}].package == "{package}" and .runs[{index}].activity == "{activity}" and',
                      f".runs[{index}].cases == 9313 and .runs[{index}].passed == 9313 and .runs[{index}].batches == {batches} and"):
            require(token in comparison, "literal finite suite identity/count missing: " + token)
    for token in (".literal_index_reference_cases == 9313", ".literal_count_reference_cases == 9313",
                  ".literal_report_reference_cases == 9313", ".literal_finite_provenance_scope",
                  "Inner prior_result_sha256, full generation-report provenance and composition/runtime typed-plan equality are NOT_CHECKED"):
        require(token in comparison, "literal finite evidence scope missing: " + token)
    for name, expected in LITERAL_COMMANDS.items():
        commands = literal_commands(named[name])
        require(len(commands) == len(expected) and set(commands) == expected,
                "literal workflow command allow-list changed: " + name)
        require("continue-on-error:" not in named[name] and "|| true" not in named[name],
                "literal gate failure became optional")
    literal = named["Execute and replay the literal text report"]
    for token in ('.decision == "OBSERVED"', ".result.runtime.finite_total == 0",
                  '.schema == "gooo/literal-text-report-checked/v1"',
                  '.decision == "PASS_FINITE_GO_ORACLE_AND_SAVED_REPLAY"',
                  ".native_observed_cases == 24", ".independent_go_oracle_cases == 24",
                  ".saved_replay_observed_cases == 24", ".native_receipt_expectation_cases == 0",
                  ".prior_result_sha256", ".execution_sha256", ".replay_sha256", ".source_bindings"):
        require(token in literal, "strong input-only Report evidence gate missing: " + token)
    malformed = named["Observe malformed literal Text inputs through the actual CLI"]
    for token in ('.schema == "gooo/literal-text-input-rejections/v1"', ".planned_framework_inputs == 84",
                  ".observed_framework_rejections == 84", ".go_reference_cases == 0",
                  '"go/strings:Count"', '"go/strings:Index"', '"app/literal-text-report:Report"',
                  ".planned_by_entry == .observed_by_entry", "all(.observed_by_entry[]; . == 28)",
                  ".verified_rejection_stage == .expected_rejection_stage", ".exit_code == 1", ".timed_out == false",
                  '.actual_runtime_stage == "NOT_REPORTED"', '.observed_counters == "NOT_REPORTED"',
                  '.observed_build == "NOT_REPORTED"', '.observed_runs == "NOT_REPORTED"'):
        require(token in malformed, "malformed actual CLI evidence gate missing: " + token)
    require(field(named["Report documentation-only native scope"], "if") == DOCS, "docs log guard")
    doc_artifact = named["Preserve documentation scope and compiler identity"]
    require(field(doc_artifact, "if") == "always() && " + DOCS, "docs artifact guard")
    require("path: |\n            out/ci-scope.json\n            out/compiler-build.json\n" in doc_artifact,
            "docs artifact must contain classification and identity only")
    require("name: gooo-go-ports-documentation-scope" in doc_artifact, "ambiguous docs artifact name")
    for name, chunk in named.items():
        if NATIVE_COMMAND.search(chunk):
            require(name in HEAVY and field(chunk, "if") == FULL, "unguarded native command: " + name)
    classifier = named["Classify the exact checked PR tree"]
    for token in ("id: change-scope", '"$GITHUB_EVENT_NAME"', '"$GITHUB_EVENT_PATH"',
                  '"$GITHUB_SHA"', '"$GITHUB_OUTPUT"', "--summary out/ci-scope.json"):
        require(token in classifier, "classifier identity/output input missing: " + token)
    observer = named["Check the Go observer"]
    for command in ("gofmt -l", "go vet ./...", "go test -race ./...",
                    "python3 tools/bootstrap.test.py", "python3 tools/receipt_json_test.py",
                    "python3 tools/check-drag-crop-example_test.py",
                    "python3 tools/check-fixed-pixel-bounds_test.py",
                    "python3 -B tools/check-alpha-matte_test.py -v",
                    "python3 -B tools/check-fits-crop_test.py -v",
                    "python3 -B tools/check-fits-crop-inputs_test.py -v",
                    "python3 -B tools/check-literal-text-inputs_test.py -v",
                    "python3 -B tools/classify-ci-scope_test.py -v",
                    "python3 -B tools/check-ci-scope-workflow_test.py -v"):
        require(command in observer, "common validation missing: " + command)
    for command in ("go run ./cmd/gen-unicode --check", "go run ./cmd/gen-simple-fold --check",
                    "go run ./cmd/gen-case-mapping --check", "python3 tools/gen-uuid-quote.py --check"):
        require(command in named["Check pinned Unicode table generation"], "generator check missing")
    identity = named["Build and identify the pinned compiler from a clean checkout"]
    for token in ("sh tools/bootstrap.sh", "version --build --json", '"0.6.32-dev"',
                  '"83928fa4538245abe33b08549da47f8cbdc50ed2"', '"CLEAN_VCS"', '"go1.27.2"'):
        require(token in identity, "compiler identity check changed")
    declarations = named["Check Gooo declarations"]
    require("./.gooo check ports/image/geometry.gooo" in declarations
            and "./.gooo check examples/fits-crop/main.gooo" in declarations
            and "tools/probe.sh" not in declarations, "declaration/probe separation")
    for source in ("ports/strings/search.gooo", "ports/strings/literal.gooo", "examples/literal-text-report/main.gooo"):
        require("./.gooo check " + source in declarations, "literal declaration check missing: " + source)
    log = named["Report documentation-only native scope"]
    require("No new native-port execution/replay receipts" in log
            and "browser-demo job retains its separate" in log, "misleading execution scope log")


class WorkflowScopeTests(unittest.TestCase):
    def setUp(self):
        self.text = WORKFLOW.read_text()

    def test_real_workflow_policy(self):
        validate(self.text)

    def test_full_native_ci_binds_two_workers(self):
        for before, after in (("--workers 2 --out out/ci", "--workers 1 --out out/ci"),
                              (".native_suite_workers == 2 and", ".native_suite_workers == 1 and")):
            with self.subTest(before=before), self.assertRaises(ValueError):
                validate(self.text.replace(before, after, 1))

    def test_each_heavy_step_and_native_upload_requires_its_guard(self):
        named, _ = named_steps(self.text)
        for name in HEAVY | UPLOADS:
            with self.subTest(step=name):
                chunk = named[name]
                condition = field(chunk, "if")
                broken = chunk.replace("        if: " + condition + "\n", "", 1)
                with self.assertRaises(ValueError):
                    validate(self.text.replace(chunk, broken, 1))

    def test_common_failure_and_unknown_output_are_not_overridden(self):
        # No always()/continue-on-error on checks; != 'true' retains full work
        # for missing/false output. A failed classifier still fails the job.
        named, _ = named_steps(self.text)
        for name in COMMON:
            self.assertNotIn("continue-on-error:", named[name])
            self.assertIsNone(field(named[name], "if"))
        name = "Compare the ports with Go and replay every batch"
        broken = named[name].replace(FULL, "steps.change-scope.outputs.docs_only == 'false'")
        with self.assertRaises(ValueError):
            validate(self.text.replace(named[name], broken, 1))

    def test_native_command_cannot_be_hidden_in_common_step(self):
        broken = self.text.replace("      - name: Check Gooo declarations\n",
                                   "      - name: Check Gooo declarations\n        run: ./.gooo package execute workspace.json\n", 1)
        with self.assertRaises(ValueError):
            validate(broken)

    def test_each_literal_native_command_is_guarded_and_exact(self):
        named, _ = named_steps(self.text)
        for name, expected in LITERAL_COMMANDS.items():
            for command in expected:
                with self.subTest(step=name, command=command):
                    # Place the otherwise authorized command in a common step.
                    misplaced = self.text.replace("      - name: Check Gooo declarations\n",
                                                   "      - name: Check Gooo declarations\n        run: " + command + "\n", 1)
                    with self.assertRaises(ValueError):
                        validate(misplaced)
            chunk = named[name]
            for altered in (chunk.replace("--inputs examples/literal-text-report/inputs.json", "--inputs other.json", 1)
                            if "--inputs" in chunk else chunk.replace("out/literal-text-input-rejections", "out/other", 1),
                            chunk.replace("        run: |", "        continue-on-error: true\n        run: |", 1)):
                with self.subTest(step=name), self.assertRaises(ValueError):
                    validate(self.text.replace(chunk, altered, 1))

    def test_literal_receipt_scope_and_absent_runtime_cannot_be_weakened(self):
        for before, after in ((".independent_go_oracle_cases == 24", ".independent_go_oracle_cases >= 1"),
                              (".native_receipt_expectation_cases == 0", ".native_receipt_expectation_cases == 24"),
                              ('.observed_counters == "NOT_REPORTED"', ".observed_counters == 0"),
                              (".verified_rejection_stage == .expected_rejection_stage", "true"),
                              ("all(.observed_by_entry[]; . == 28)", "true"),
                              ("([.runs[:59][].cases] | add) == 131842", "true"),
                              (".literal_index_reference_cases == 9313", ".literal_index_reference_cases == 24"),
                              (".runs[61].batches == 281", ".runs[61].batches == 137"),
                              ("are NOT_CHECKED", "are CHECKED")):
            with self.subTest(before=before), self.assertRaises(ValueError):
                validate(self.text.replace(before, after, 1))

    def test_literal_report_requires_explicit_matching_go_tool(self):
        named, _ = named_steps(self.text)
        chunk = named["Execute and replay the literal text report"]
        selected = '--go "$(go env GOROOT)/bin/go"'
        self.assertEqual(chunk.count(selected), 2)
        for occurrence in (0, 1):
            start = chunk.find(selected) if occurrence == 0 else chunk.find(selected, chunk.find(selected) + len(selected))
            for replacement in ('', '--go /usr/bin/go'):
                changed = chunk[:start] + replacement + chunk[start + len(selected):]
                with self.subTest(occurrence=occurrence, replacement=replacement), self.assertRaises(ValueError):
                    validate(self.text.replace(chunk, changed, 1))

    def test_new_unreviewed_step_or_changed_browser_triggers_failure(self):
        extra = "      - name: Unreviewed native command\n        run: go run ./cmd/verify\n\n"
        with self.assertRaises(ValueError):
            validate(self.text.replace("  browser-demo:\n", extra + "  browser-demo:\n", 1))
        with self.assertRaises(ValueError):
            validate(self.text.replace("needs: [native-ports, browser-demo]", "needs: [browser-demo]"))

    def test_docs_artifact_cannot_include_native_receipts(self):
        named, _ = named_steps(self.text)
        chunk = named["Preserve documentation scope and compiler identity"]
        broken = chunk.replace("            out/compiler-build.json\n", "            out/ci/\n")
        with self.assertRaises(ValueError):
            validate(self.text.replace(chunk, broken, 1))


if __name__ == "__main__":
    unittest.main()
