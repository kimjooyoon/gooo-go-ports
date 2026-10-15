#!/usr/bin/env python3
"""Guard the CI skip boundary using the workflow's explicit step structure.

This deliberately accepts only the current plain-scalar, fixed-indent step
layout. Unrecognized YAML layout fails rather than silently missing a guard.
It needs no YAML package on the CI runner.
"""

import hashlib
import re
import subprocess
import unittest
from pathlib import Path


WORKFLOW = Path(__file__).resolve().parent.parent / ".github/workflows/ci.yml"
FULL = "steps.change-scope.outputs.docs_only != 'true'"
DOCS = "steps.change-scope.outputs.docs_only == 'true'"
HEADER_SHA = "5cd59615e4403c288bd003ae603c7e55a813f42af0bbbb4715effebf8d093e9b"
# Reviewed browser delta: add API policy actual-Wasm/package checks after service-policy smoke.
# Events, native skip guards and deployment dependencies stay byte-identical.
BROWSER_SHA = "9842fdb57f69899aa53215451feeda87eca4ce90f4b1bb92ef18326608be3231"
PATH_STEP_SHA = 'b90f403ce804adea78dacebf554c45f3e21b1faf89b942e83f32aca2847f9df5'
REDELIVERY_STEP_SHA = '2ce6496cbee29d77c9334f1f32dc518c96a12c193e37ab569144bdc4fd1a864e'
IPV4_STEP_SHA = 'fc7ae14e887a878b0975c5704884d2b5618d83bad7a2b45414c5171b3de3b1e3'
IPV4_COMMAND = 'python3 -I -B tools/check-ipv4.py --repository . --compiler ./.gooo --go "$(go env GOROOT)/bin/go" --out out/ipv4 --timeout 180'
IPV4_READBACK = 'python3 -I -B tools/check-ipv4.py --verify-summary "$PWD/out/ipv4/summary.json" > out/ipv4/summary-readback.json'
IPV4_COMMON_COMMANDS = (
    'python3 -I -B tools/ipv4_receipts_test.py',
    'python3 -I -B tools/check-ipv4_test.py',
)
REDELIVERY_COMMAND = 'python3 -B tools/check-redelivery-policy.py --repository . --out out/redelivery-policy --go "$(go env GOROOT)/bin/go" --compiler ./.gooo --timeout 300'
REDELIVERY_COMMON_COMMANDS = (
    'go -C examples/automatic-redelivery/reference vet ./...',
    'go -C examples/automatic-redelivery/reference test -race -count=1 ./...',
    'python3 -I -B tools/check-redelivery-policy_test.py',
)
PATH_COMMAND = 'python3 -B tools/check-path-facts.py --repository . --out out/path-facts --go "$(go env GOROOT)/bin/go" --compiler ./.gooo --timeout 300'
PATH_COMMON_UNIT_BLOCK = '          # Full scope runs these13 Python controls inside the bounded sidecar once.\n          # Documentation-only PRs still run them as common non-native checks.\n          if [ "${{ steps.change-scope.outputs.docs_only }}" = "true" ]; then\n            python3 -I -B tools/check-path-facts_test.py\n          fi\n'
NATIVE_COMPARISON_COMMAND = "go run ./cmd/verify --workers 2 --case-profile uuid-compact-v1 --out out/ci --timing-out out/ci/timing.jsonl"
CASE_PROFILE_ASSERTION = '''            .case_profile == {
              schema: "gooo/native-case-profile/v1", name: "uuid-compact-v1",
              compiler_revision: "83928fa4538245abe33b08549da47f8cbdc50ed2", go_version: "go1.27.2",
              entries: ["go/github.com/google/uuid:Validate", "app/uuid-format-inspector:Inspect", "go/github.com/google/uuid/text:ParseCanonical"],
              serializer: "Go1.27.2 json.Marshal(caseDocument(batch)) plus exactly one LF",
              maximum_rows: 128, raw_bytes_including_LF: 30000,
              translated_compact_engine_bytes: 32768, maximum_selected_source_bytes: 10485760,
              lowering_guard: "Pinned839 source-derived nonexpansion upper bound; not measured translated bytes or an actual LoweredName.",
              scope: "Only UUID3 wire bytes/partitions change; original ordered cases, independent expected values, exact workspace and fresh execution/saved replay remain required. No legacy batch total, native-capacity acceptance, full-profile compatibility or speedup is inherited."
            } and
'''
TIMING_HEADER_ASSERTION = '''          head -n 1 out/ci/timing.jsonl | jq -e --slurpfile summary out/ci/summary.json '
            . as $header |
            .schema == "gooo/native-verification-timing/v1" and .kind == "header" and .seq == 0 and
            .case_profile == $summary[0].case_profile and
            .compiler_revision == "83928fa4538245abe33b08549da47f8cbdc50ed2" and .go_version == "go1.27.2" and
            .configured_workers == 2 and .expected_suites == 66 and .expected_cases == 159909 and
            .expected_boundaries_recomputed == false and
            .boundary_scope == "Actual contiguous batch coverage is observed. UUID3 use uuid-compact-v1 compact wire bytes and newly selected bounds; no original UUID batch boundaries or legacy2337 total are claimed preserved. Non-UUID suites retain nativeBatchEnd/save behavior. The recorder does not recompute expected partitions." and
            (.identity.source_files | length) == 100 and
            all(["cmd/verify/case_profile.go", "cmd/verify/case_profile_test.go", "cmd/verify/case_profile_integration_test.go"][];
              . as $path | ($header.identity.source_files[$path].bytes | type) == "number" and
              $header.identity.source_files[$path].bytes > 0 and
              ($header.identity.source_files[$path].sha256 | test("^[0-9a-f]{64}$")))
          '
'''
TIMING_CLAUSES = (
    '.schema == "gooo/native-verification-timing/v1"', '.kind == "run_end"',
    '.status == "COMPLETE"',
    '.complete == true and .aborted == false and .successful_close_return_required == true',
    '.seq == 2242', '.expected_suites == 66', '.successful_suites == 66',
    '.observed_batches == 2109', '.verified_batches == 2109', '.verified_cases == 159909',
    '.event_count == 2243', '.failed_suites == 0', '.canceled_suites == 0',
    '.not_started_suites == 0', '.missing_suite_finishes == 0',
    '.failed_batches == 0', '.incomplete_batches == 0', '.logger_errors == 0',
    '.source_bindings_unchanged == true',
)
TIMING_ASSERTION = ("          tail -n 1 out/ci/timing.jsonl | jq -e '\n"
                    + "\n".join("            " + clause + (" and" if index < len(TIMING_CLAUSES) - 1 else "")
                                for index, clause in enumerate(TIMING_CLAUSES))
                    + "\n          '\n")
TIMING_SUMMARY_SHA = '449c77747290beaa0cb7d41c356e47199bbbc1a071e2f6da25a0c69eb8c0e52e'
COMMON = {
    "Classify the exact checked PR tree", "Check the Go observer",
    "Check pinned Unicode table generation",
    "Build and identify the pinned compiler from a clean checkout",
    "Check Gooo declarations",
}
HEAVY = {
    "Verify the isolated native path facts sidecar",
    "Verify the isolated native IPv4 address facts sidecar",
    "Verify the original automatic redelivery policy",
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
    "Verify the imported CEL API policy",
    "Verify the sealed API policy revision comparison",
    "Verify the public API policy consumer",
    "Verify the Agent Mesh service-pattern adaptation",
}
UPLOADS = {"Upload offline policy experiment evidence", "Upload execution and diagnostic evidence"}
DOC_STEPS = {"Report documentation-only native scope", "Preserve documentation scope and compiler identity"}
NATIVE_COMMAND = re.compile(
    r"(?:\.gooo\s+package\s+(?:execute|replay)|\./cmd/(?:verify|check-unicode|check-literal-text-report)\b|"
    r"tools/probe\.sh|tools/check-(?:fits-crop|literal-text)-inputs\.py|decision-policy-pilot/run\.sh|cel-api-policy/verify\.py|agentmesh-service-policy/verify\.py|api-policy-revision/verify\.py|tools/check-api-policy-public-consumer\.py|tools/check-path-facts\.py|tools/check-redelivery-policy\.py|tools/check-ipv4\.py)"
)
PUBLIC_CONSUMER_COMMAND = 'python3 -B tools/check-api-policy-public-consumer.py --go "$(go env GOROOT)/bin/go" --revision "$PUBLIC_API_REVISION" --out "$PWD/out/api-policy-public-consumer"'
PUBLIC_CONSUMER_CLAUSES = (
    '.schema == "gooo/api-policy-public-consumer/v1"',
    '.status == "PUBLIC_API_POLICY_CONSUMER_PASS"',
    '.acceptance.facts == 9', '.acceptance.positive_cases == 2',
    '.acceptance.decision_fields == 15', '.acceptance.difference_flags == 15',
    '.acceptance.summary_fields == 2', '.acceptance.public_asset_count == 36',
    '.acceptance.public_assets_exact == true', '.acceptance.public_generation_exact == true',
    '.acceptance.original_assets_exact == true', '.acceptance.original_generation_unchanged == true',
    '.acceptance.substitution_refused == true', '.acceptance.runtime_empty_path == true',
    '.acceptance.consumer_no_replace_or_workspace == true', '.acceptance.substitution_stdout_bytes == 0',
    '.acceptance.original_version == "v0.0.0-20261014100709-8cddcc4e65af"',
    '.acceptance.original_commit == "8cddcc4e65afb42431f8f42a86f29859ae2f4d69"',
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


# The common checks move byte-for-byte; both isolated jobs classify their own
# exact full-history checkout. These are source-integrity locks, not CI PASS.
OBSERVER_HEADER_SHA = '46cb0e7392dd3f3fb9562bccea6b27e5194ffcf59e1a58bab4c07b99aba08bd9'
NATIVE_HEADER_SHA = '5f4d0761ec793a582d7d23c06f9a758bd71fa66f5448b50d01739915052b213e'
OBSERVER_STEP_SHA = 'd816c8dde53f5b6d24f624f7cac9d32220860d9bd91c7fd787abeb69274e7d98'
GENERATOR_STEP_SHA = '081263b551a4d28aa8b46dc025fe951c62d91d13e164cb7927b70d7270aaf399'
CLASSIFIER_STEP_SHA = '1864b9e0f27cc73595dd863c71640050cf046b2ec67cfd02c6ba5a8ebd11a61a'
CHECKOUT_STEP_SHA = 'b60ba5d7bf95691b8baec3a3e402aeba552fbcb4a353d7de8946023e73e79603'
SETUP_STEP_SHA = '742468cc6b855d4f2c2d9a5d45000899bc244f524539bad21505ef94018dceb2'
MOVED = {'Check the Go observer', 'Check pinned Unicode table generation'}
JOB_ORDER = ('observer-checks', 'native-execution', 'native-ports', 'browser-demo', 'deploy-browser-demo')
AGGREGATE = """  native-ports:
    if: ${{ always() }}
    needs: [observer-checks, native-execution]
    runs-on: ubuntu-latest
    timeout-minutes: 2
    steps:
      - name: Require both independent verification jobs
        env:
          OBSERVER_RESULT: ${{ needs.observer-checks.result }}
          NATIVE_RESULT: ${{ needs.native-execution.result }}
        shell: bash
        run: |
          test "$OBSERVER_RESULT" = "success"
          test "$NATIVE_RESULT" = "success"

"""


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


def workflow_jobs(text):
    require(text.count("jobs:\n") == 1, "missing or repeated jobs map")
    raw = text.split("jobs:\n", 1)[1]
    starts = list(re.finditer(r"^  ([a-z][a-z0-9-]*):\n", raw, re.M))
    require(tuple(match[1] for match in starts) == JOB_ORDER, "unknown, repeated, missing or reordered job")
    require(len(starts) == len(re.findall(r"^  \S", raw, re.M)), "unrecognized job layout")
    return {match[1]: raw[match.start():starts[i + 1].start() if i + 1 < len(starts) else len(raw)]
            for i, match in enumerate(starts)}


def job_steps(chunk):
    require(chunk.count("    steps:\n") == 1, "missing or repeated steps map")
    header, steps_text = chunk.split("    steps:\n", 1)
    starts = list(re.finditer(r"^      - (name|uses): (.+)$", steps_text, re.M))
    require(len(starts) == len(re.findall(r"^      - ", steps_text, re.M)), "unrecognized step layout")
    named, unnamed = {}, []
    for index, match in enumerate(starts):
        step = steps_text[match.start():starts[index + 1].start() if index + 1 < len(starts) else len(steps_text)]
        if match[1] == "name":
            require(match[2] not in named, "duplicate step name")
            named[match[2]] = step
        else:
            unnamed.append(step)
    return header, named, unnamed


def named_steps(text):
    jobs = workflow_jobs(text)
    require(jobs['native-ports'] == AGGREGATE, "aggregate must always require both concrete jobs to succeed")
    all_named, native_unnamed = {}, []
    for job, header_sha in (('observer-checks', OBSERVER_HEADER_SHA), ('native-execution', NATIVE_HEADER_SHA)):
        chunk = jobs[job]
        header, named, unnamed = job_steps(chunk)
        # No job-level skip, needs cycle, matrix, write permissions or optional
        # result is allowed on either producer; exact existing timeout/env stay.
        require(hashlib.sha256(header.encode()).hexdigest() == header_sha, "producer job header changed: " + job)
        require('continue-on-error:' not in chunk, "producer failure became optional: " + job)
        require(len(unnamed) == 2, "producer requires checkout and pinned Go setup: " + job)
        require(hashlib.sha256(unnamed[0].encode()).hexdigest() == CHECKOUT_STEP_SHA and
                hashlib.sha256(unnamed[1].encode()).hexdigest() == SETUP_STEP_SHA, "producer checkout/Go pin changed: " + job)
        classifier = named.get('Classify the exact checked PR tree', '')
        require(hashlib.sha256(classifier.encode()).hexdigest() == CLASSIFIER_STEP_SHA, "producer exact checkout classifier changed: " + job)
        require(chunk.index(unnamed[0]) < chunk.index(classifier) < chunk.index(unnamed[1]), "producer setup order changed: " + job)
        expected = {'Classify the exact checked PR tree'} | MOVED if job == 'observer-checks' else (COMMON | HEAVY | UPLOADS | DOC_STEPS) - MOVED
        require(set(named) == expected, "producer step roster changed: " + job)
        if job == 'observer-checks':
            require(hashlib.sha256(named['Check the Go observer'].encode()).hexdigest() == OBSERVER_STEP_SHA and
                    hashlib.sha256(named['Check pinned Unicode table generation'].encode()).hexdigest() == GENERATOR_STEP_SHA,
                    "common observer/generator commands must move unchanged once")
            require(chunk.index(unnamed[1]) < chunk.index(named['Check the Go observer']) <
                    chunk.index(named['Check pinned Unicode table generation']), "observer check order changed")
        else:
            native_unnamed = unnamed
        for name, step in named.items():
            require(name not in all_named or name == 'Classify the exact checked PR tree', "duplicate cross-job step")
            all_named[name] = step
    return all_named, native_unnamed


def validate(text):
    header = text[:text.index("jobs:\n")]
    browser = text[text.index("  browser-demo:\n"):]
    # These independently audited sections retain their previous exact behavior.
    require(hashlib.sha256(header.encode()).hexdigest() == HEADER_SHA, "events or permissions changed")
    require(hashlib.sha256(browser.encode()).hexdigest() == BROWSER_SHA, "browser/deployment behavior changed")
    named, unnamed = named_steps(text)
    require(len(HEAVY) == 28, "full-scope native step count changed")
    require(set(named) == COMMON | HEAVY | UPLOADS | DOC_STEPS, "step roster changed without guard review")
    require(len(unnamed) == 2 and all(field(c, "if") is None for c in unnamed), "checkout/Go setup became conditional")
    require("fetch-depth: 0" in unnamed[0], "exact PR parents must be available")
    require("go-version: '1.27.2'" in unnamed[1], "Go toolchain pin changed")
    public = named["Verify the public API policy consumer"]
    native_job = workflow_jobs(text)['native-execution']
    require(native_job.index(unnamed[1]) < native_job.index(public) <
            native_job.index(named["Build and identify the pinned compiler from a clean checkout"]),
            "public consumer must run after native Go setup and before native compiler/execution")
    require("python3 -B tools/check-api-policy-public-consumer_test.py -v" in named["Check the Go observer"],
            "bounded public-consumer diagnostic checks must remain in the common Go observer step")
    for name in COMMON:
        require(field(named[name], "if") is None, "common check became conditional: " + name)
    for name in HEAVY:
        require(field(named[name], "if") == FULL, "missing full-execution guard: " + name)
    for name in UPLOADS:
        require(field(named[name], "if") == "always() && " + FULL, "native artifact can upload on docs path: " + name)
    comparison = named["Compare the ports with Go and replay every batch"]
    require(literal_commands(comparison) == [NATIVE_COMPARISON_COMMAND]
            and ".native_suite_workers == 2 and" in comparison,
            "full native CI must bind two workers and source-bound timing")
    require(comparison.count(CASE_PROFILE_ASSERTION) == 1,
            "native CI must require the exact explicit UUID compact profile identity")
    require(comparison.count(TIMING_HEADER_ASSERTION) == 1,
            "native timing must bind the same profile and complete source identity")
    require(comparison.count(TIMING_ASSERTION) == 1,
            "native timing must require the exact complete footer and full observed roster")
    summaries = re.findall(r"^          jq -c -s '\n.*?^          ' out/ci/timing.jsonl\n", comparison, re.M | re.S)
    require(len(summaries) == 1 and hashlib.sha256(summaries[0].encode()).hexdigest() == TIMING_SUMMARY_SHA,
            "native timing console summary must retain its exact source and actual input")
    require("continue-on-error:" not in comparison and "|| true" not in comparison,
            "native comparison or timing failure became optional")
    for token in (".ported_functions == 45 and .consumer_programs == 19 and .consumer_activities == 21 and",
                  "(.runs | length) == 66 and",
                  ".cases == 159909 and .passed == 159909 and",
                  "([.runs[].cases] | add) == 159909 and ([.runs[].batches] | add) == 2109 and",
                  "([.runs[:62][].cases] | add) == 159781 and ([.runs[:62][].batches] | add) == 2104 and",
                  "([.runs[:59][].cases] | add) == 131842 and ([.runs[:59][].batches] | add) == 1549 and",
                  "([.runs[:57][].cases] | add) == 130702 and ([.runs[:57][].batches] | add) == 1513 and",
                  "([.runs[:56][].cases] | add) == 108835 and ([.runs[:56][].batches] | add) == 1274 and",
                  "([.runs[:54][].cases] | add) == 65345 and ([.runs[:54][].batches] | add) == 882 and",
                  "all(.runs[]; .passed == .cases and .replay_equal == true)"):
        require(token in comparison, "original case roster or compact partition counts missing: " + token)
    for index, package, activity, cases, batches in (
            (54, "go/github.com/google/uuid", "Validate", 21745, 195),
            (55, "app/uuid-format-inspector", "Inspect", 21745, 197),
            (56, "go/github.com/google/uuid/text", "ParseCanonical", 21867, 239)):
        for token in (f'.runs[{index}].package == "{package}" and .runs[{index}].activity == "{activity}" and',
                      f".runs[{index}].cases == {cases} and .runs[{index}].passed == {cases} and .runs[{index}].batches == {batches} and"):
            require(token in comparison, "UUID compact suite identity/count missing: " + token)
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
    for index, package, activity, batches in ((62, "go/strings", "Cut", 1),
                                               (63, "go/strings", "CutPrefix", 1),
                                               (64, "go/strings", "CutSuffix", 1),
                                               (65, "app/literal-cut-report", "Report", 2)):
        for token in (f'.runs[{index}].package == "{package}" and .runs[{index}].activity == "{activity}" and',
                      f".runs[{index}].cases == 32 and .runs[{index}].passed == 32 and .runs[{index}].batches == {batches} and"):
            require(token in comparison, "Cut finite suite identity/count/batch missing: " + token)
    for token in (".literal_cut_reference_cases == 32", ".literal_cut_prefix_reference_cases == 32",
                  ".literal_cut_suffix_reference_cases == 32", ".literal_cut_report_reference_cases == 32",
                  '(.literal_cut_finite_provenance_scope | contains("Inner prior_result_sha256, full generation-report provenance and composition/runtime typed-plan equality are NOT_CHECKED")) and'):
        require(token in comparison, "Cut finite evidence scope missing: " + token)
    revision = named["Verify the sealed API policy revision comparison"]
    require('python3 -B examples/api-policy-revision/verify.py --compiler "$PWD/.gooo" --go "$(go env GOROOT)/bin/go" --out "$PWD/out/api-policy-revision"' in revision,
            "sealed revision native generator/test/fresh-process verifier missing")
    for token in ('.schema == "gooo/api-policy-revision-verification/v1"',
                  '.status == "FINITE_TWO_PROFILE_NATIVE_AND_SAVED_REPLAY_PASS"',
                  '.classifier_cases == 23', '.decision_fields == 15',
                  '.fresh_process_pairs == 2', '.altered_report_refused == true',
                  '.original_generation_preserved == true'):
        require(token in revision, "sealed revision acceptance missing: " + token)
    require("continue-on-error:" not in revision and "|| true" not in revision,
            "sealed revision failure became optional")
    public_consumer = named["Verify the public API policy consumer"]
    expected_public_consumer = (
        "      - name: Verify the public API policy consumer\n"
        "        if: " + FULL + "\n"
        "        env:\n"
        "          PUBLIC_API_REVISION: ${{ github.event.pull_request.head.sha || github.sha }}\n"
        "        run: |\n"
        "          " + PUBLIC_CONSUMER_COMMAND + "\n"
        "          jq -e '" + " and ".join(PUBLIC_CONSUMER_CLAUSES)
        + "' out/api-policy-public-consumer/verification.json\n"
    )
    require(public_consumer == expected_public_consumer,
            "public consumer must bind the actual PR head, exact Go command and every acceptance clause")
    path = named["Verify the isolated native path facts sidecar"]
    require(literal_commands(path) == [PATH_COMMAND] and hashlib.sha256(path.encode()).hexdigest() == PATH_STEP_SHA,
            "path sidecar must retain exact fresh producer/output/count/cleanup gates")
    require(named["Check the Go observer"].count(PATH_COMMON_UNIT_BLOCK) == 1
            and text.count("python3 -I -B tools/check-path-facts_test.py") == 1
            and text.count(PATH_COMMAND) == 1, "path controls must run inside full sidecar or docs-only common path exactly once")
    redelivery = named["Verify the original automatic redelivery policy"]
    require(hashlib.sha256(redelivery.encode()).hexdigest() == REDELIVERY_STEP_SHA
            and redelivery.count(REDELIVERY_COMMAND) == 1 and text.count(REDELIVERY_COMMAND) == 1,
            "redelivery sidecar must retain exact fresh two-layer/ref/cleanup gates once")
    require(text.index(named["Check Gooo declarations"]) < text.index(redelivery)
            < text.index(named["Retain unsupported-body probes"]), "redelivery producer order changed")
    for command in REDELIVERY_COMMON_COMMANDS:
        require(named["Check the Go observer"].count(command) == 1 and text.count(command) == 1,
                "redelivery unit/reference checks must run once as common checks")
    require(named["Check Gooo declarations"].count("./.gooo check examples/automatic-redelivery/main.gooo") == 1,
            "original redelivery declaration check missing")
    ipv4 = named["Verify the isolated native IPv4 address facts sidecar"]
    require(hashlib.sha256(ipv4.encode()).hexdigest() == IPV4_STEP_SHA
            and ipv4.count(IPV4_COMMAND) == text.count(IPV4_COMMAND) == 1
            and ipv4.count(IPV4_READBACK) == text.count(IPV4_READBACK) == 1,
            "IPv4 sidecar must retain fresh finite layers, current source readback and cleanup gates once")
    require(text.index(named["Check Gooo declarations"]) < text.index(ipv4)
            < text.index(named["Retain unsupported-body probes"]), "IPv4 producer order changed")
    for command in IPV4_COMMON_COMMANDS:
        require(named["Check the Go observer"].count(command) == text.count(command) == 1,
                "IPv4 Python checks must run once as common checks")
    for source in ("ports/net/netip/ipv4.gooo", "examples/ipv4-address-facts/main.gooo"):
        require(named["Check Gooo declarations"].count("./.gooo check " + source) == 1,
                "IPv4 declaration check missing: " + source)
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
    for token in ('.schema == "gooo/literal-text-input-rejections/v1"', ".planned_framework_inputs == 196",
                  ".observed_framework_rejections == 196", ".go_reference_cases == 0",
                  '"go/strings:Count"', '"go/strings:Index"', '"app/literal-text-report:Report"',
                  '"go/strings:Cut"', '"go/strings:CutPrefix"', '"go/strings:CutSuffix"', '"app/literal-cut-report:Report"',
                  "(.cases | length) == 196",
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
    for source in ("ports/strings/search.gooo", "ports/strings/literal.gooo", "examples/literal-text-report/main.gooo", "ports/strings/cut.gooo", "examples/literal-cut-report/main.gooo"):
        require("./.gooo check " + source in declarations, "literal declaration check missing: " + source)
    for source in ("ports/cel/scalar_access.gooo", "examples/cel-api-policy/main.gooo"):
        require("./.gooo check " + source in declarations, "CEL declaration check missing: " + source)
    for source in ("ports/agentmesh/service_pattern.gooo", "examples/agentmesh-service-policy/impact.gooo", "examples/agentmesh-service-policy/refinement.gooo"):
        require("./.gooo check " + source in declarations, "AgentMesh declaration check missing: " + source)
    for source in ("ports/path/path.gooo", "examples/path-facts/main.gooo"):
        require("./.gooo check " + source in declarations, "path declaration check missing: " + source)
    mesh = named["Verify the Agent Mesh service-pattern adaptation"]
    require('python3 -B examples/agentmesh-service-policy/verify.py --compiler "$PWD/.gooo" --go "$(go env GOROOT)/bin/go" --out "$PWD/out/agentmesh-service-policy"' in mesh,
            "AgentMesh verifier must bind pinned compiler and Go tool")
    for token in ('"gooo/agentmesh-native-verification/v1"', '"FINITE_FRESH_ORACLE_NATIVE_WORKSPACE_REPLAY_PASS"',
                  '.matcher.upstream_cases == 12', '.matcher.total_cases == 63', '.original_impact.cases == 16',
                  '.original_impact.fields_compared == 3', '.workspace_batches | length) == 5',
                  '.case_count] | add) == 79', '.bound_assets == .asset_tamper_refusals',
                  '.strict_json.max_count == 1024', '.strict_json.max_batch_bytes == 1048576',
                  '.strict_json.max_component_bytes == 128', '.verifier_helper_contracts.native_array_cardinality == true',
                  '.verifier_helper_contracts.all_raw_impact_facts_and_strict_types == true',
                  '.verifier_helper_contracts.complete_typed_workspace_delivery_identity == true',
                  '.verifier_helper_contracts.optimized_python_refused == true',
                  '.refinement.cases == 101', '.refinement.fields_compared == 8', '.refinement.family_cells == 36',
                  '.refinement.fresh_oracle_witnesses == 63', '.refinement.fresh_oracle_matcher_observations == 126',
                  '.refinement.workspace_batches | length) == 7', '.refinement.workspace_batches[].case_count] | add) == 101',
                  '.verifier_helper_contracts.all_raw_refinement_facts_and_strict_types == true',
                  '.verifier_helper_contracts.checked_witness_domain == true', 'out/agentmesh-service-policy/verification.json'):
        require(token in mesh, "AgentMesh receipt contract missing: " + token)
    cel = named["Verify the imported CEL API policy"]
    require("go -C examples/cel-api-policy/oracle mod download" in cel,
            "fresh CI runner must fetch pinned oracle dependencies before offline verification")
    require('python3 examples/cel-api-policy/verify.py --compiler "$PWD/.gooo" --go "$(go env GOROOT)/bin/go" --out "$PWD/out/cel-api-policy"' in cel,
            "CEL verifier must bind the pinned compiler and matching Go tool")
    for token in ('"gooo/cel-public-native-verification/v1"',
                  '"FINITE_REAL_CEL_IMPORTED_NATIVE_AND_REPLAY_PASS"',
                  "out/cel-api-policy/verification.json"):
        require(token in cel, "CEL receipt identity check missing: " + token)
    require('jq -e \'.schema == "gooo/cel-public-native-verification/v1" and .status == "FINITE_REAL_CEL_IMPORTED_NATIVE_AND_REPLAY_PASS"\' out/cel-api-policy/verification.json' in cel,
            "CEL receipt assertion must read the actual verifier output")
    for token in ('.saved_api_replay.status == "SAME_SOURCE_GENERATION_HISTORICAL_REPLAY_PASS"',
                  '.saved_api_replay.independent_cases == 32',
                  '.saved_api_replay.decision_fields_compared == 15',
                  '.saved_api_replay.saved_decision_mutations_rejected == 15',
                  '.saved_api_replay.saved_binding_asset_mutations_rejected == 28',
                  '.saved_api_replay.active_fact_mutations_rejected == 9',
                  '.saved_api_replay.prior_generation_rejected == true',
                  '.saved_api_replay.consistent_synthetic_proof_accepted == true',
                  '.saved_api_replay.consumer.fresh_process_save_reload_pairs == 2',
                  '.saved_api_replay.consumer.current_provider_calls == 0',
                  '.saved_api_replay.consumer.effect_calls == 0',
                  '.saved_api_replay.consumer.application_state_unchanged == true'):
        require(token in cel, "Saved API replay check missing: " + token)
    log = named["Report documentation-only native scope"]
    require("No new native-port execution/replay receipts" in log
            and "browser-demo job retains its separate" in log, "misleading execution scope log")


class WorkflowScopeTests(unittest.TestCase):
    def setUp(self):
        self.text = WORKFLOW.read_text()

    def test_public_consumer_stays_before_native_compilation_without_serial_observer_dependency(self):
        named, unnamed = named_steps(self.text)
        public = named["Verify the public API policy consumer"]
        without = self.text.replace(public, "", 1)
        for anchor in (named["Check the Go observer"], named["Compare the ports with Go and replay every batch"]):
            with self.subTest(anchor=anchor.splitlines()[0]), self.assertRaises(ValueError):
                validate(without.replace(anchor, anchor + public, 1))
        with self.assertRaises(ValueError):
            validate(without.replace(unnamed[1], public + unnamed[1], 1))

    def test_bounded_public_consumer_diagnostics_remain_tested(self):
        command = "          python3 -B tools/check-api-policy-public-consumer_test.py -v\n"
        self.assertIn(command, self.text)
        with self.assertRaises(ValueError):
            validate(self.text.replace(command, "", 1))

    def test_path_sidecar_cannot_weaken_producer_scope_counts_or_cleanup(self):
        named, _ = named_steps(self.text)
        chunk = named["Verify the isolated native path facts sidecar"]
        changes = (('--go "$(go env GOROOT)/bin/go"', '--go /usr/bin/go'),
                   ("--compiler ./.gooo", "--compiler /tmp/gooo"),
                   ("--repository .", "--repository /tmp/old-source"),
                   ("--timeout 300", "--timeout 900"),
                   ("--out out/path-facts", "--out out/old-path-facts"),
                   (".generated_native_entry_outputs == 29523", ".generated_native_entry_outputs >= 1"),
                   (".generated_native_scalar_comparisons == 78728", ".generated_native_scalar_comparisons == 29523"),
                   (".manual_saved_replay_outputs == 72", ".manual_saved_replay_outputs == 0"),
                   (".full_CLI_outputs == 0", ".full_CLI_outputs == 29523"),
                   ("(.stages | length) == 52", "(.stages | length) >= 1"),
                   ('(.stages | type) == "array"', '(.stages | type) == "object"'),
                   ("($ext_cleanup | length) == 1", "true"),
                   ('["ext", "isabs", "report"]', '["report"]'),
                   ("$f.owned_native_work_removed == false", "$f.owned_native_work_removed == true"),
                   ("$native[$i].removed == true", "true"),
                   ("$cleanup[0].all_top_level_groups_reaped == true", "true"),
                   (".strict_transport_refusals == 15", ".strict_transport_refusals == 0"),
                   (".receipt_tamper_refusals == 18", ".receipt_tamper_refusals == 0"),
                   ("' out/path-facts/summary.json", "' out/old/summary.json"),
                   ("        run: |", "        continue-on-error: true\n        run: |"))
        for before, after in changes:
            self.assertIn(before, chunk)
            with self.subTest(before=before), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, chunk.replace(before, after, 1), 1))
        with self.assertRaises(ValueError):validate(self.text.replace(chunk, "", 1))

    def test_path_declarations_and_docs_only_unit_branch_are_required(self):
        named, _ = named_steps(self.text)
        declarations = named["Check Gooo declarations"]
        for source in ("ports/path/path.gooo", "examples/path-facts/main.gooo"):
            token = "./.gooo check " + source
            self.assertIn(token, declarations)
            with self.subTest(source=source), self.assertRaises(ValueError):
                validate(self.text.replace(declarations, declarations.replace(token, "# missing " + source, 1), 1))
        for changed in ("", PATH_COMMON_UNIT_BLOCK.replace('= "true"', '= "false"'),
                        PATH_COMMON_UNIT_BLOCK + "          python3 -I -B tools/check-path-facts_test.py\n"):
            with self.subTest(changed=changed), self.assertRaises(ValueError):
                validate(self.text.replace(PATH_COMMON_UNIT_BLOCK, changed, 1))

    def test_redelivery_scope_producer_counts_and_refs_cannot_change(self):
        named, _ = named_steps(self.text)
        chunk = named["Verify the original automatic redelivery policy"]
        self.assertIn(REDELIVERY_COMMAND, chunk)
        for before, after in (("--repository .", "--repository /tmp/old"),
                              ('--go "$(go env GOROOT)/bin/go"', '--go /usr/bin/go'),
                              ("--compiler ./.gooo", "--compiler /tmp/old"),
                              ("--timeout 300", "--timeout 900"),
                              ("--out out/redelivery-policy", "--out out/old"),
                              ('"top_level_stages":70', '"top_level_stages":1'),
                              ('"manual_saved_replay_outputs":16', '"manual_saved_replay_outputs":0'),
                              ('"generated_native_outputs":12288', '"generated_native_outputs":16'),
                              ('"full_CLI_outputs":0', '"full_CLI_outputs":12288'),
                              ('"strict_transport_refusals":36', '"strict_transport_refusals":0'),
                              ('"receipt_tamper_refusals":18', '"receipt_tamper_refusals":0'),
                              ('assert digest.hexdigest() == sha', 'assert True'),
                              ('"retained_runtime_refs"', '"old_refs"'),
                              ('and not owned.exists()', 'and True'),
                              ("        run: |", "        continue-on-error: true\n        run: |")):
            self.assertIn(before, chunk)
            with self.subTest(before=before), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, chunk.replace(before, after, 1), 1))
        with self.assertRaises(ValueError): validate(self.text.replace(chunk, "", 1))

    def test_redelivery_common_checks_and_declaration_are_required_once(self):
        for command in REDELIVERY_COMMON_COMMANDS:
            line = "          " + command + "\n"
            self.assertEqual(self.text.count(line), 1)
            for changed in ("", line + line):
                with self.subTest(command=command, changed=changed), self.assertRaises(ValueError):
                    validate(self.text.replace(line, changed, 1))
        line = "          ./.gooo check examples/automatic-redelivery/main.gooo\n"
        with self.assertRaises(ValueError): validate(self.text.replace(line, "", 1))

    def test_real_workflow_policy(self):
        validate(self.text)

    def test_ipv4_layers_source_readback_and_limits_cannot_be_weakened(self):
        named, _ = named_steps(self.text)
        chunk = named["Verify the isolated native IPv4 address facts sidecar"]
        mutations = [("--timeout 180", "--timeout 900"),
                     ('--go "$(go env GOROOT)/bin/go"', '--go /usr/bin/go'),
                     ("--compiler ./.gooo", "--compiler /tmp/old"),
                     (IPV4_READBACK, "true"),
                     (".manual_saved_replay_outputs == 352", ".manual_saved_replay_outputs == 0"),
                     (".generated_Report_calls == 202", ".generated_Report_calls == 64"),
                     (".generated_typed_fields == 2222", ".generated_typed_fields >= 1"),
                     (".strict_wire_refusals == 42", ".strict_wire_refusals == 18"),
                     (".receipt_tamper_refusals == 18", ".receipt_tamper_refusals == 0"),
                     (".top_level_stages == 54", ".top_level_stages >= 1"),
                     (".automatic_retries == 0", ".automatic_retries == 1"),
                     ("CURRENT_SOURCE_AND_EVIDENCE_RECHECKED", "OLD_PASS"),
                     ("        run: |", "        continue-on-error: true\n        run: |")]
        for before, after in mutations:
            self.assertIn(before, chunk)
            with self.subTest(before=before), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, chunk.replace(before, after, 1), 1))
        with self.assertRaises(ValueError):
            validate(self.text.replace(chunk, "", 1))

    def test_ipv4_common_checks_and_declarations_are_required_once(self):
        for command in IPV4_COMMON_COMMANDS:
            line = "          " + command + "\n"
            for changed in ("", line + line):
                with self.subTest(command=command), self.assertRaises(ValueError):
                    validate(self.text.replace(line, changed, 1))
        for source in ("ports/net/netip/ipv4.gooo", "examples/ipv4-address-facts/main.gooo"):
            line = "          ./.gooo check " + source + "\n"
            with self.subTest(source=source), self.assertRaises(ValueError):
                validate(self.text.replace(line, "", 1))

    def test_full_native_ci_binds_two_workers_and_timing(self):
        for workers in (1, 3, 4, 5):
            for before, after in (("--workers 2 --case-profile", f"--workers {workers} --case-profile"),
                                  (".native_suite_workers == 2 and", f".native_suite_workers == {workers} and")):
                with self.subTest(workers=workers, before=before), self.assertRaises(ValueError):
                    validate(self.text.replace(before, after, 1))

    def test_compact_profile_cannot_be_missing_unknown_legacy_or_rebound(self):
        named, _ = named_steps(self.text)
        chunk = named["Compare the ports with Go and replay every batch"]
        mutations = [chunk.replace(" --case-profile uuid-compact-v1", replacement, 1)
                     for replacement in ("", " --case-profile legacy", " --case-profile uuid-compact-v2",
                                         " --case-profile uuid-compact-v1 --case-profile legacy")]
        for assertion in (CASE_PROFILE_ASSERTION, TIMING_HEADER_ASSERTION):
            self.assertIn(assertion, chunk)
            mutations.append(chunk.replace(assertion, "", 1))
            for line in assertion.splitlines()[1:-1]:
                mutations.append(chunk.replace(assertion, assertion.replace(line, "            true and", 1), 1))
        mutations.extend((
            chunk.replace('name: "uuid-compact-v1"', 'name: "legacy"', 1),
            chunk.replace("maximum_rows: 128", "maximum_rows: 129", 1),
            chunk.replace("raw_bytes_including_LF: 30000", "raw_bytes_including_LF: 32768", 1),
            chunk.replace("(.identity.source_files | length) == 100", "(.identity.source_files | length) >= 97", 1),
            chunk.replace("--slurpfile summary out/ci/summary.json", "--slurpfile summary out/old/summary.json", 1),
            chunk.replace(".case_profile == $summary[0].case_profile", ".case_profile == null", 1),
        ))
        for altered in mutations:
            self.assertNotEqual(chunk, altered)
            with self.subTest(altered=altered[:600]), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, altered, 1))

    def test_compact_uuid_exact_counts_and_full_case_replay_gate_cannot_be_weakened(self):
        named, _ = named_steps(self.text)
        chunk = named["Compare the ports with Go and replay every batch"]
        clauses = [
            "([.runs[:59][].cases] | add) == 131842 and ([.runs[:59][].batches] | add) == 1549 and",
            "([.runs[:57][].cases] | add) == 130702 and ([.runs[:57][].batches] | add) == 1513 and",
            "([.runs[:56][].cases] | add) == 108835 and ([.runs[:56][].batches] | add) == 1274 and",
            "([.runs[:54][].cases] | add) == 65345 and ([.runs[:54][].batches] | add) == 882 and",
            "all(.runs[]; .passed == .cases and .replay_equal == true)",
        ]
        for index, package, activity, cases, batches in (
                (54, "go/github.com/google/uuid", "Validate", 21745, 195),
                (55, "app/uuid-format-inspector", "Inspect", 21745, 197),
                (56, "go/github.com/google/uuid/text", "ParseCanonical", 21867, 239)):
            clauses.extend((
                f'.runs[{index}].package == "{package}" and .runs[{index}].activity == "{activity}" and',
                f".runs[{index}].cases == {cases} and .runs[{index}].passed == {cases} and .runs[{index}].batches == {batches} and",
            ))
        for clause in clauses:
            self.assertIn(clause, chunk)
            for replacement in ("true", clause.replace("==", ">=", 1)):
                self.assertNotEqual(clause, replacement)
                with self.subTest(clause=clause), self.assertRaises(ValueError):
                    validate(self.text.replace(chunk, chunk.replace(clause, replacement, 1), 1))

    def test_native_timing_completion_cannot_be_omitted_or_weakened(self):
        named, _ = named_steps(self.text)
        chunk = named["Compare the ports with Go and replay every batch"]
        mutations = [
            chunk.replace(" --timing-out out/ci/timing.jsonl", "", 1),
            chunk.replace("--timing-out out/ci/timing.jsonl", "--timing-out out/other.jsonl", 1),
            chunk.replace(TIMING_ASSERTION, "", 1),
            chunk.replace("tail -n 1 out/ci/timing.jsonl", "tail -n 1 out/other.jsonl", 1),
            chunk.replace("        run: |", "        continue-on-error: true\n        run: |", 1),
        ] + [chunk.replace(TIMING_ASSERTION, TIMING_ASSERTION.replace(clause, "true", 1), 1)
             for clause in TIMING_CLAUSES]
        for altered in mutations:
            with self.subTest(altered=altered[-600:]), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, altered, 1))

    def test_native_timing_console_summary_cannot_be_removed_or_rebound(self):
        named, _ = named_steps(self.text)
        chunk = named["Compare the ports with Go and replay every batch"]
        summary = re.search(r"^          jq -c -s '\n.*?^          ' out/ci/timing.jsonl\n", chunk, re.M | re.S)[0]
        for altered in (chunk.replace(summary, "", 1),
                        chunk.replace(summary, summary.replace("timing-summary/v1", "timing-summary/v0"), 1),
                        chunk.replace(summary, summary.replace("map(.duration_ns) | add", "map(.duration_ns) | length"), 1),
                        chunk.replace(summary, summary.replace("out/ci/timing.jsonl", "out/other.jsonl"), 1)):
            with self.subTest(altered=altered[-500:]), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, altered, 1))

    def test_cut_counts_original62_cases_and_compact_partitions_cannot_be_weakened(self):
        named, _ = named_steps(self.text)
        comparison = named["Compare the ports with Go and replay every batch"]
        clauses = (
            ".ported_functions == 45 and .consumer_programs == 19 and .consumer_activities == 21 and",
            "(.runs | length) == 66 and", ".cases == 159909 and .passed == 159909 and",
            "([.runs[].cases] | add) == 159909 and ([.runs[].batches] | add) == 2109 and",
            "([.runs[:62][].cases] | add) == 159781 and ([.runs[:62][].batches] | add) == 2104 and",
            ".literal_cut_reference_cases == 32", ".literal_cut_prefix_reference_cases == 32",
            ".literal_cut_suffix_reference_cases == 32", ".literal_cut_report_reference_cases == 32",
            '(.literal_cut_finite_provenance_scope | contains("Inner prior_result_sha256, full generation-report provenance and composition/runtime typed-plan equality are NOT_CHECKED")) and',
        )
        for clause in clauses:
            self.assertIn(clause, comparison)
            weakened = clause.replace("==", ">=", 1) if "==" in clause else clause.replace("NOT_CHECKED", "CHECKED", 1)
            for replacement in ("true", weakened):
                with self.subTest(clause=clause, replacement=replacement), self.assertRaises(ValueError):
                    validate(self.text.replace(comparison, comparison.replace(clause, replacement, 1), 1))
        for index, package, activity, batches in ((62, "go/strings", "Cut", 1),
                                                  (63, "go/strings", "CutPrefix", 1),
                                                  (64, "go/strings", "CutSuffix", 1),
                                                  (65, "app/literal-cut-report", "Report", 2)):
            clauses = (f'.runs[{index}].package == "{package}" and .runs[{index}].activity == "{activity}" and',
                       f".runs[{index}].cases == 32 and .runs[{index}].passed == 32 and .runs[{index}].batches == {batches} and")
            for clause in clauses:
                self.assertIn(clause, comparison)
                for replacement in ("true", clause.replace("==", ">=", 1)):
                    with self.subTest(index=index, clause=clause), self.assertRaises(ValueError):
                        validate(self.text.replace(comparison, comparison.replace(clause, replacement, 1), 1))

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
        for command in ("./.gooo package execute workspace.json", PUBLIC_CONSUMER_COMMAND):
            broken = self.text.replace("      - name: Check Gooo declarations\n",
                                       "      - name: Check Gooo declarations\n        run: " + command + "\n", 1)
            with self.subTest(command=command), self.assertRaises(ValueError):
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

    def test_cut_malformed_entries_counts_and_declarations_cannot_be_removed(self):
        named, _ = named_steps(self.text)
        malformed = named["Observe malformed literal Text inputs through the actual CLI"]
        for token in ('"go/strings:Cut"', '"go/strings:CutPrefix"', '"go/strings:CutSuffix"',
                      '"app/literal-cut-report:Report"', '.planned_framework_inputs == 196',
                      '.observed_framework_rejections == 196', '(.cases | length) == 196'):
            with self.subTest(token=token), self.assertRaises(ValueError):
                validate(self.text.replace(malformed, malformed.replace(token, 'true', 1), 1))
        declarations = named["Check Gooo declarations"]
        for name in ('ports/strings/cut.gooo', 'examples/literal-cut-report/main.gooo'):
            with self.subTest(source=name), self.assertRaises(ValueError):
                validate(self.text.replace(declarations, declarations.replace('./.gooo check ' + name, '# missing ' + name, 1), 1))

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
        policy_smoke = "          node tools/service-policy-smoke.cjs out/browser/site | tee out/browser/service-policy-smoke.json\n"
        self.assertEqual(self.text.count(policy_smoke), 1)
        for replacement in ("", policy_smoke.replace("service-policy-smoke.cjs", "browser-smoke.cjs")):
            with self.subTest(replacement=replacement), self.assertRaises(ValueError):
                validate(self.text.replace(policy_smoke, replacement, 1))
        named, _ = named_steps(self.text)
        public = named["Verify the public API policy consumer"]
        mutations = (
            ('${{ github.event.pull_request.head.sha || github.sha }}', '${{ github.sha }}'),
            ('--go "$(go env GOROOT)/bin/go"', '--go /usr/bin/go'),
            ('--revision "$PUBLIC_API_REVISION"', '--revision "$GITHUB_SHA"'),
            ('out/api-policy-public-consumer/verification.json', 'out/other.json'),
            ('        run: |', '        continue-on-error: true\n        run: |'),
        ) + tuple((clause, 'true') for clause in PUBLIC_CONSUMER_CLAUSES)
        for before, after in mutations:
            with self.subTest(public_consumer=before), self.assertRaises(ValueError):
                validate(self.text.replace(public, public.replace(before, after, 1), 1))

    def test_cel_native_verifier_cannot_use_another_producer_or_skip_receipt(self):
        named, _ = named_steps(self.text)
        chunk = named["Verify the imported CEL API policy"]
        for before, after in (('--compiler "$PWD/.gooo"', '--compiler /tmp/gooo'),
                              ('--go "$(go env GOROOT)/bin/go"', '--go /usr/bin/go'),
                              ('go -C examples/cel-api-policy/oracle mod download', 'go mod download'),
                              ('"FINITE_REAL_CEL_IMPORTED_NATIVE_AND_REPLAY_PASS"', '"PASS"'),
                              ('out/cel-api-policy/verification.json', 'out/other.json')):
            with self.subTest(before=before), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, chunk.replace(before, after, 1), 1))
        for before, after in (('"SAME_SOURCE_GENERATION_HISTORICAL_REPLAY_PASS"', '"PASS"'),
                              ('.saved_api_replay.independent_cases == 32', '.saved_api_replay.independent_cases >= 1'),
                              ('.saved_api_replay.decision_fields_compared == 15', '.saved_api_replay.decision_fields_compared == 1'),
                              ('.saved_api_replay.prior_generation_rejected == true', '.saved_api_replay.prior_generation_rejected == false'),
                              ('.saved_api_replay.consumer.effect_calls == 0', '.saved_api_replay.consumer.effect_calls >= 0')):
            with self.subTest(before=before), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, chunk.replace(before, after, 1), 1))
        misplaced = self.text.replace("      - name: Check Gooo declarations\n",
                                      "      - name: Check Gooo declarations\n        run: python3 examples/cel-api-policy/verify.py\n", 1)
        with self.assertRaises(ValueError):
            validate(misplaced)

    def test_agentmesh_verifier_cannot_skip_producer_or_typed_receipt_contract(self):
        named, _ = named_steps(self.text)
        chunk = named["Verify the Agent Mesh service-pattern adaptation"]
        for before, after in (('--compiler "$PWD/.gooo"', '--compiler /tmp/gooo'),
                              ('--go "$(go env GOROOT)/bin/go"', '--go /usr/bin/go'),
                              ('"FINITE_FRESH_ORACLE_NATIVE_WORKSPACE_REPLAY_PASS"', '"PASS"'),
                              ('.matcher.total_cases == 63', '.matcher.total_cases >= 1'),
                              ('.original_impact.fields_compared == 3', '.original_impact.fields_compared == 1'),
                              ('.refinement.cases == 101', '.refinement.cases >= 1'),
                              ('.refinement.fields_compared == 8', '.refinement.fields_compared == 3'),
                              ('.refinement.fresh_oracle_matcher_observations == 126', '.refinement.fresh_oracle_matcher_observations == 0'),
                              ('.verifier_helper_contracts.checked_witness_domain == true', '.verifier_helper_contracts.checked_witness_domain == false'),
                              ('.verifier_helper_contracts.complete_typed_workspace_delivery_identity == true', '.verifier_helper_contracts.complete_typed_workspace_delivery_identity == false'),
                              ('out/agentmesh-service-policy/verification.json', 'out/other.json')):
            with self.subTest(before=before), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, chunk.replace(before, after, 1), 1))
        misplaced = self.text.replace("      - name: Check Gooo declarations\n",
                                      "      - name: Check Gooo declarations\n        run: python3 -B examples/agentmesh-service-policy/verify.py\n", 1)
        with self.assertRaises(ValueError):
            validate(misplaced)

    def test_docs_artifact_cannot_include_native_receipts(self):
        named, _ = named_steps(self.text)
        chunk = named["Preserve documentation scope and compiler identity"]
        broken = chunk.replace("            out/compiler-build.json\n", "            out/ci/\n")
        with self.assertRaises(ValueError):
            validate(self.text.replace(chunk, broken, 1))


    def test_producer_jobs_are_parallel_exact_unconditional_scopes(self):
        validate(self.text)
        jobs = workflow_jobs(self.text)
        for name in ('observer-checks', 'native-execution'):
            chunk = jobs[name]
            header, _, _ = job_steps(chunk)
            self.assertNotIn('    needs:', header)
            self.assertNotIn('    if:', header)
            for addition in (
                    "    if: github.event_name == 'push'\n",
                    "    if: ${{ always() }}\n",
                    "    needs: [native-ports]\n",
                    "    needs: [observer-checks]\n",
                    "    needs: [native-execution]\n",
                    "    continue-on-error: true\n",
                    "    permissions:\n      contents: write\n",
                    "    strategy:\n      matrix:\n        optional: [true]\n"):
                changed = chunk.replace('    runs-on:', addition + '    runs-on:', 1)
                with self.subTest(job=name, addition=addition), self.assertRaises(ValueError):
                    validate(self.text.replace(chunk, changed, 1))
            for before, after in (("fetch-depth: 0", "fetch-depth: 1"),
                                  ("go-version: '1.27.2'", "go-version: '1.27.1'"),
                                  ('--expected-head "$GITHUB_SHA"', '--expected-head HEAD'),
                                  ('timeout-minutes: 40', 'timeout-minutes: 90')):
                self.assertIn(before, chunk)
                with self.subTest(job=name, before=before), self.assertRaises(ValueError):
                    validate(self.text.replace(chunk, chunk.replace(before, after, 1), 1))

    def test_job_and_step_rosters_refuse_unknown_missing_and_duplicate_entries(self):
        validate(self.text)
        jobs = workflow_jobs(self.text)
        for name in ('observer-checks', 'native-execution', 'native-ports'):
            with self.subTest(missing=name), self.assertRaises(ValueError):
                validate(self.text.replace(jobs[name], '', 1))
            with self.subTest(duplicate=name), self.assertRaises(ValueError):
                validate(self.text.replace(jobs[name], jobs[name] + jobs[name], 1))
        unknown = '  unreviewed-success:\n    runs-on: ubuntu-latest\n    steps:\n      - run: true\n'
        with self.assertRaises(ValueError):
            validate(self.text.replace('  browser-demo:\n', unknown + '  browser-demo:\n', 1))
        for name in ('observer-checks', 'native-execution'):
            chunk = jobs[name]
            extra = '      - name: Unreviewed work\n        run: go run ./cmd/verify\n'
            changed = chunk.replace('    steps:\n', '    steps:\n' + extra, 1)
            with self.subTest(extra_step=name), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, changed, 1))
        _, named, _ = job_steps(jobs['observer-checks'])
        classifier = named['Classify the exact checked PR tree']
        with self.assertRaises(ValueError):
            validate(self.text.replace(classifier, '', 1))
        for name in MOVED:
            step = named[name]
            native = jobs['native-execution'].replace('    steps:\n', '    steps:\n' + step, 1)
            with self.subTest(duplicate_common=name), self.assertRaises(ValueError):
                validate(self.text.replace(jobs['native-execution'], native, 1))
            with self.subTest(missing_common=name), self.assertRaises(ValueError):
                validate(self.text.replace(step, '', 1))

    def test_aggregate_graph_and_shell_cannot_skip_or_accept_failed_dependencies(self):
        validate(self.text)
        chunk = workflow_jobs(self.text)['native-ports']
        mutations = (
            ('    if: ${{ always() }}\n', ''),
            ('always()', 'success()'),
            ('always()', "github.event_name == 'pull_request'"),
            ('needs: [observer-checks, native-execution]', 'needs: [native-execution]'),
            ('needs: [observer-checks, native-execution]', 'needs: [observer-checks]'),
            ('needs: [observer-checks, native-execution]', 'needs: [observer-checks, native-ports]'),
            ('timeout-minutes: 2', 'timeout-minutes: 40'),
            ('        shell: bash', '        continue-on-error: true\n        shell: bash'),
            ('        shell: bash', '        shell: sh'),
            ('${{ needs.observer-checks.result }}', '${{ needs.native-execution.result }}'),
            ('${{ needs.native-execution.result }}', '${{ needs.observer-checks.result }}'),
            ('test "$OBSERVER_RESULT" = "success"', 'test "$OBSERVER_RESULT" != "failure"'),
            ('test "$NATIVE_RESULT" = "success"', 'test "$NATIVE_RESULT" != "failure"'),
            ('test "$OBSERVER_RESULT" = "success"', 'test "$OBSERVER_RESULT" = "success" || true'),
            ('test "$NATIVE_RESULT" = "success"', 'true'),
            ('    steps:\n', '    permissions:\n      contents: write\n    steps:\n'),
            ('    steps:\n', '    steps:\n      - uses: actions/checkout@main\n'),
        )
        for before, after in mutations:
            self.assertIn(before, chunk)
            with self.subTest(before=before, after=after), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, chunk.replace(before, after, 1), 1))

    def test_actual_aggregate_inline_gate_all25_outcomes(self):
        # Execute the candidate's actual two builtins, not a Python reimplementation
        # of their condition. No repository checkout, toolchain or network runs.
        validate(self.text)
        aggregate = workflow_jobs(self.text)['native-ports']
        self.assertIn('OBSERVER_RESULT: ${{ needs.observer-checks.result }}', aggregate)
        self.assertIn('NATIVE_RESULT: ${{ needs.native-execution.result }}', aggregate)
        raw_script = aggregate.split('        run: |\n', 1)[1]
        script = '\n'.join(line[10:] for line in raw_script.splitlines() if line)
        states = ('success', 'failure', 'cancelled', 'skipped', '')
        for observer in states:
            for native in states:
                environment = {'PATH': '/usr/bin:/bin', 'OBSERVER_RESULT': observer, 'NATIVE_RESULT': native}
                result = subprocess.run(['/bin/bash', '--noprofile', '--norc', '-e', '-o', 'pipefail', '-c', script],
                                        env=environment, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                                        stderr=subprocess.PIPE, timeout=5, check=False)
                with self.subTest(observer=observer, native=native):
                    self.assertEqual(result.returncode == 0, observer == native == 'success')
                    self.assertEqual(result.stdout, b'')
        for missing in ('OBSERVER_RESULT', 'NATIVE_RESULT'):
            environment = {'PATH': '/usr/bin:/bin', 'OBSERVER_RESULT': 'success', 'NATIVE_RESULT': 'success'}
            del environment[missing]
            result = subprocess.run(['/bin/bash', '--noprofile', '--norc', '-e', '-o', 'pipefail', '-c', script],
                                    env=environment, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                                    stderr=subprocess.PIPE, timeout=5, check=False)
            self.assertNotEqual(result.returncode, 0)



if __name__ == "__main__":
    unittest.main()
