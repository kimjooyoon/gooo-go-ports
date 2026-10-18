#!/usr/bin/env python3
"""Guard the CI skip boundary using the workflow's explicit step structure.

This deliberately accepts only the current plain-scalar, fixed-indent step
layout. Unrecognized YAML layout fails rather than silently missing a guard.
It needs no YAML package on the CI runner.
"""

import hashlib
import json
import re
import subprocess
import tempfile
import unittest
from pathlib import Path


WORKFLOW = Path(__file__).resolve().parent.parent / ".github/workflows/ci.yml"
FULL = "steps.change-scope.outputs.docs_only != 'true'"
PREFLIGHT_STEP_NAME = 'Verify lightweight API policy summary controls before native work'
PREFLIGHT_STEP_SHA = '6e31886b6e7f6f6edcb6681b651bf985c5388c35141dc1b01d4310f298045f93'
PREFLIGHT = {PREFLIGHT_STEP_NAME}
DOCS = "steps.change-scope.outputs.docs_only == 'true'"
HEADER_SHA = "5cd59615e4403c288bd003ae603c7e55a813f42af0bbbb4715effebf8d093e9b"
# The browser producer and deployment body retain their reviewed bytes.
# Native and the independent policy producer use the pinned Ubuntu 24.04 runner.
BROWSER_JOB_SHA = "45fc28d79fe032ed66dd0dcba493926d3e6d2d03db068e96e51a4edf20607e06"
DEPLOY_BODY_SHA = "0fed0b124228b17bd80d8f95c7c7e69ac2ad854f719ca846c3e60a1359a480c8"
PATH_STEP_SHA = 'b90f403ce804adea78dacebf554c45f3e21b1faf89b942e83f32aca2847f9df5'
REDELIVERY_STEP_SHA = '2ce6496cbee29d77c9334f1f32dc518c96a12c193e37ab569144bdc4fd1a864e'
IPV4_STEP_SHA = '6ce5d2f5e75f9ff9a7728718567ffb039c74232ef4e254ea244b7dc02fbc9ed8'
IPV4_COMMAND = 'python3 -I -B tools/check-ipv4.py --repository . --compiler ./.gooo --go "$(go env GOROOT)/bin/go" --out out/ipv4 --timeout 180 --include-prefix-rule-pair'
IPV4_READBACK = 'python3 -I -B tools/check-ipv4.py --verify-summary "$PWD/out/ipv4/summary.json" > out/ipv4/summary-readback.json'
IPV4_COMMON_COMMANDS = (
    'python3 -I -B tools/ipv4_receipts_test.py',
    'python3 -I -B tools/check-ipv4_test.py',
    'python3 -I -B tools/prefix_rule_pair_receipts_test.py',
    'python3 -I -B tools/generate-rule-pair-corpus.py --check',
)
IPV4_DECLARATIONS = (
    "ports/net/netip/ipv4.gooo", "examples/ipv4-address-facts/main.gooo",
    "ports/net/netip/prefix.gooo", "examples/ipv4-prefix-probe/main.gooo",
    "examples/ipv4-rule-pair/main.gooo",
)
REDELIVERY_COMMAND = 'python3 -B tools/check-redelivery-policy.py --repository . --out out/redelivery-policy --go "$(go env GOROOT)/bin/go" --compiler ./.gooo --timeout 300'
REDELIVERY_COMMON_COMMANDS = (
    'go -C examples/automatic-redelivery/reference vet ./...',
    'go -C examples/automatic-redelivery/reference test -race -count=1 ./...',
    'python3 -I -B tools/check-redelivery-policy_test.py',
)
PATH_COMMAND = 'python3 -B tools/check-path-facts.py --repository . --out out/path-facts --go "$(go env GOROOT)/bin/go" --compiler ./.gooo --timeout 300'
PATH_COMMON_UNIT_BLOCK = '          # Full scope runs these13 Python controls inside the bounded sidecar once.\n          # Documentation-only PRs still run them as common non-native checks.\n          if [ "${{ steps.change-scope.outputs.docs_only }}" = "true" ]; then\n            python3 -I -B tools/check-path-facts_test.py\n          fi\n'
URL_COMPONENT_STEP = 'Verify the isolated native URL component facts sidecar'
URL_COMPONENT_PRIOR_WORKFLOW_SHA = '4a71b0a2b33b7ddc3b951f93f32f0662f80658f54e82afe88c8b24a5aebce77d'
URL_COMPONENT_COMMAND = 'python3 -I -B tools/check-url-components.py --root "$PWD" --compiler "$PWD/.gooo" --go "$(go env GOROOT)/bin/go" --out "$PWD/out/url-components"'
URL_COMPONENT_COMMON = 'python3 -I -B tools/check-url-components_test.py'
URL_COMPONENT_DECLARATIONS = ('ports/net/url/escape.gooo', 'examples/url-context-facts/main.gooo')
URL_COMPONENT_PRIOR_DECLARATIONS_SHA = '1b6519f1807a7835281717384ad00afef2c17b7f3816713229fb84fceba03c4e'
URL_COMPONENT_DECLARATIONS_SHA = 'e8fc134018ea36713b23fc87fe86a2aacd27d2f96edddb0abc08c7a516b1b5dc'
URL_COMPONENT_CLAUSES = (
    '.schema == "gooo/url-component-verification/v1"',
    '.status == "NATIVE_ORACLE_AND_SAVED_REPLAY_MATCH"',
    '.input_rows == 128', '.unique_inputs == 128',
    '.typed_fields_per_layer == 1280', '.native_runs_per_layer == 2',
    '.finite_total == 0', '.model_calls == 0', '.catalog_extended == false',
    '(.negative_controls | length) == 27', '(.negative_controls | unique | length) == 27',
    '.processes_closed == true', '.owned_work_removed == true',
    "' out/url-components/summary.json",
)
# The exporter reads only reviewed source expectations. Actual native evidence is
# compared later; it is never an input to the expected-data command.
NATIVE_EXPECTED_COMMAND = 'python3 -I -B tools/native_verification_contract.py --root "$PWD" > out/native-contract-expected.json'
NATIVE_EXPECTED_UNIT = 'python3 -I -B tools/native_verification_contract_test.py -v'
NATIVE_EXPECTED_SLURP = '--slurpfile expected out/native-contract-expected.json'
NATIVE_EXPECTED_BINDING = "$expected[0] as $e |"
NATIVE_SOURCE_PATHS_ASSERTION = "(.identity.source_files | keys) == $e.source_paths"
NATIVE_ALL_POPULATION_ASSERTION = '            (. as $actual | all(range(0;$e.totals.suites); . as $i | $actual.runs[$i] as $r | $e.suites[$i] as $wanted |\n              $r.package == $wanted.package and $r.activity == $wanted.activity and\n              $r.cases == $wanted.cases and $r.passed == $wanted.cases and\n              $r.batches == $wanted.batches and $r.replay_equal == true)) and\n'
NATIVE_COMPARISON_COMMAND = "go run -mod=readonly ./cmd/verify --workers 3 --case-profile uuid-and-literal-report-compact-v1 --workspace-profile dependency-closure-v1 --suite-order largest-corpus-first --out out/ci --timing-out out/ci/timing.jsonl"
SUITE_ORDER_SUMMARY_ASSERTION = '''            .suite_order == "largest-corpus-first" and
            .planned_dispatch_indices == (.runs | to_entries | sort_by([-.value.cases, .key]) | map(.key)) and
'''
CASE_PROFILE_ASSERTION = '''            .case_profile == {
              schema: "gooo/native-case-profile/v1", name: "uuid-and-literal-report-compact-v1",
              compiler_revision: "83928fa4538245abe33b08549da47f8cbdc50ed2", go_version: "go1.27.2",
              entries: ["go/github.com/google/uuid:Validate", "app/uuid-format-inspector:Inspect", "go/github.com/google/uuid/text:ParseCanonical", "app/literal-text-report:Report"],
              basis_profile: "uuid-compact-v1",
              serializer: "Go1.27.2 json.Marshal(caseDocument(batch)) plus exactly one LF",
              maximum_rows: 128, raw_bytes_including_LF: 30000,
              translated_compact_engine_bytes: 32768, maximum_selected_source_bytes: 10485760,
              lowering_guard: "Pinned839 source-derived nonexpansion upper bound; not measured translated bytes or an actual LoweredName.",
              scope: "UUID3 retain the exact uuid-compact-v1 policy/wire bytes. Only literal-text Report adds compact wire bytes and new source-bounded partitions. Index/Count and decimal/payload policies remain unchanged. Original ordered cases, independent expectations, exact sources and fresh native execution/saved replay are required; no prior profile PASS, native capacity or speedup is inherited."
            } and
'''
WORKSPACE_PROFILE_ASSERTION = '''            .workspace_profile == {
              schema: "gooo/native-workspace-profile/v1", name: "dependency-closure-v1",
              basis: "legacy-selected-workspace",
              selection: "entry import closure over whole already-selected packages; original package/source order",
              compiler_revision: "83928fa4538245abe33b08549da47f8cbdc50ed2", go_version: "go1.27.2"
            } and
'''
TIMING_HEADER_ASSERTION = '          head -n 1 out/ci/timing.jsonl | jq -e --slurpfile summary out/ci/summary.json --slurpfile expected out/native-contract-expected.json \'\n            $expected[0] as $e |\n            . as $header |\n            .schema == "gooo/native-verification-timing/v1" and .kind == "header" and .seq == 0 and\n            .case_profile == $summary[0].case_profile and\n            .workspace_profile == $summary[0].workspace_profile and\n            .suite_order == "largest-corpus-first" and .suite_order == $summary[0].suite_order and\n            .planned_dispatch_indices == $summary[0].planned_dispatch_indices and\n            .compiler_revision == "83928fa4538245abe33b08549da47f8cbdc50ed2" and .go_version == "go1.27.2" and\n            .configured_workers == 3 and .expected_suites == $e.totals.suites and .expected_cases == $e.totals.cases and\n            .command_resources_schema == "gooo/native-command-resources/v1" and\n            .expected_boundaries_recomputed == false and\n            .boundary_scope == "Actual contiguous ranges only. UUID3 retain uuid-compact-v1 wire/partitions; literal-text Report uses a new compact writer/partition identity. No prior profile PASS or full batch total is inherited. Index/Count and other existing non-UUID suites retain their indented writer; decimal/payload retain their source-bounded pretty JSON policy. Fresh native/saved replay and the full summary gate remain required; no timing or speedup claim." and\n            (.identity.source_files | length) == $e.totals.source_paths and\n            (.identity.source_files | keys) == $e.source_paths and\n            all($header.identity.source_files[];\n              (.bytes | type) == "number" and .bytes > 0 and\n              (.sha256 | test("^[0-9a-f]{64}$")))\n          \'\n'
TIMING_CLAUSES = (
    '.schema == "gooo/native-verification-timing/v1"', '.kind == "run_end"',
    '.status == "COMPLETE"',
    '.complete == true and .aborted == false and .successful_close_return_required == true',
    '.seq == ($e.totals.timing_records - 1)', '.expected_suites == $e.totals.suites', '.successful_suites == $e.totals.suites',
    '.observed_batches == $e.totals.batches', '.verified_batches == $e.totals.batches', '.verified_cases == $e.totals.cases',
    '.event_count == $e.totals.timing_records', '.failed_suites == 0', '.canceled_suites == 0',
    '.not_started_suites == 0', '.missing_suite_finishes == 0',
    '.failed_batches == 0', '.incomplete_batches == 0', '.logger_errors == 0',
    '.source_bindings_unchanged == true',
)
TIMING_ASSERTION = ("          tail -n 1 out/ci/timing.jsonl | jq -e --slurpfile expected out/native-contract-expected.json '\n"
                    + "            $expected[0] as $e |\n"
                    + "\n".join("            " + clause + (" and" if index < len(TIMING_CLAUSES) - 1 else "")
                                for index, clause in enumerate(TIMING_CLAUSES))
                    + "\n          '\n")
LITERAL_REPORT_COMPACT_SCOPE = 'Only app/literal-text-report:Report adds compact case JSON plus one LF, <=128 original rows, <=30000 raw bytes and a pinned839 source-derived nonexpanding-key <=32768 translated bound. Index/Count retain their prior indented writer and partitions. Native execution and saved replay must freshly inspect all9313 original Report pairs; count-only281-to235 is not native, full-CI or performance acceptance. Decimal/payload policies remain unchanged.'
TIMING_SUMMARY_SHA = 'ca262e77de47a75e3c458a84862b7d5eb13333d1c1af7c6e313ce20f99ee3ae8'
HEADER_ADDITION_NEW_SOURCE_PATHS = ('cmd/header-addition-input/main.go', 'cmd/header-addition-input/main_test.go', 'cmd/verify/header_addition_batches.go', 'cmd/verify/header_addition_batches_test.go', 'cmd/verify/header_addition_contracts.go', 'cmd/verify/header_addition_contracts_test.go', 'cmd/verify/header_addition_corpus.go', 'cmd/verify/header_addition_corpus_test.go', 'diagnostic-header-addition.workspace.json', 'examples/diagnostic-header-addition/README.md', 'examples/diagnostic-header-addition/contract.json', 'examples/diagnostic-header-addition/corpus.json', 'examples/diagnostic-header-addition/inputs.json', 'examples/diagnostic-header-addition/main.gooo', 'examples/diagnostic-header-addition/request.json', 'examples/header-addition-host/LICENSE', 'examples/header-addition-host/NOTICE', 'examples/header-addition-host/README.md', 'examples/header-addition-host/bindings.go.txt', 'examples/header-addition-host/bindings_test.go.txt', 'examples/header-addition-host/go.mod.txt', 'examples/header-addition-host/host.go.txt', 'examples/header-addition-host/host_test.go.txt', 'examples/header-addition-host/main.go.txt', 'examples/header-addition-host/provenance.json', 'internal/headeradditioninput/preflight.go', 'internal/headeradditioninput/preflight_test.go', 'tools/check-header-addition-host.py', 'tools/check-header-addition-host_test.py', 'tools/check-header-addition-inputs.py')
HEADER_ADDITION_INPUT_STEP_NAME = 'Verify strict addition inputs against the fresh native references'
HEADER_ADDITION_HOST_STEP_NAME = 'Verify the freshly generated atomic header-addition host'
HEADER_ADDITION_INPUT_COMMAND = 'python3 -I -B tools/check-header-addition-inputs.py --root "$PWD" --compiler "$PWD/.gooo" --input-command "$PWD/out/header-addition-input" --go "$(go env GOROOT)/bin/go" --verification-out "$PWD/out/ci" --timing "$PWD/out/ci/timing.jsonl" --out "$PWD/out/header-addition-input-route"'
HEADER_ADDITION_HOST_COMMAND = 'python3 -I -B tools/check-header-addition-host.py --root "$PWD" --verification-out "$PWD/out/ci" --timing "$PWD/out/ci/timing.jsonl" --go "$(go env GOROOT)/bin/go" --out "$PWD/out/header-addition-host"'
HEADER_ADDITION_HOST_UNIT = 'python3 -I -B tools/check-header-addition-host_test.py -v'
HEADER_ADDITION_INPUT_STEP_SHA = '83224a3ac6dad0d07f0015fa4be088535ecc83fa752d9ca7e65848b9a2014ecc'
HEADER_ADDITION_HOST_STEP_SHA = '27810b4983483229ee40a15eb0d3b4a77a3d0c36fc1da9256f4be979bf56cddc'
HEADER_ADDITION_CLAUSES = ('([.runs[:73][].cases] | add) == 171167 and ([.runs[:73][].batches] | add) == 2174 and', '.header_addition_reference_cases == 50 and .header_addition_fixed_batch_rows == 8 and', '.header_addition_record_types == 5 and .header_addition_pure_activities == 6 and .header_addition_callee_edges == 5 and', '.runs[73].package == "app/diagnostic-header-addition" and .runs[73].activity == "AssessAddition" and', '.runs[73].cases == 50 and .runs[73].passed == 50 and .runs[73].batches == 7 and', '.source_sha256["examples/diagnostic-header-addition/main.gooo"] == "cd499d4bda6b40937882c77d67774b573c84aa93ec407e9a95b82f755d41eae8" and', '.header_addition_scope == "Original deterministic12-field RequestFacts plus three bounded Text inputs return16 typed header-addition decision fields. Actual pinned x/net/httpguts and strconv references supply the50 finite expected results. No per-request human approval is required to compute; host authority/current binding and atomic final apply remain necessary." and', '.header_addition_provenance_scope == "Fresh native execution and saved replay bind five current sources, six pure activities, five record layouts, five callee edges, exact12 input/16 output fields, generated ABI and raw outer saved parents. Independent inner typed canonical prior_result_sha256 remains NOT_CHECKED; private pilot results are not fresh public acceptance." and')
HEADER_WIRE_STEP_NAME = 'Verify strict header input reaches the actual native CLI'
HEADER_WIRE_COMMAND = 'python3 -I -B tools/check-header-facts-inputs.py --root "$PWD" --compiler "$PWD/.gooo" --input-command "$PWD/out/header-facts-input" --go "$(go env GOROOT)/bin/go" --out "$PWD/out/header-facts-input-route"'
HEADER_WIRE_STEP_SHA = '04e719ee2669cf0cdde795740db470eba1a661bc98c7f02d3e1d6b98dfbb8d19'
HEADER_MODULE_PREFLIGHT = "          go mod download golang.org/x/net golang.org/x/text\n          go mod verify\n"
HEADER_SOURCE_SET_ASSERTION = '          jq -e -s --slurpfile summary out/ci/summary.json --slurpfile expected out/native-contract-expected.json \'\n            $expected[0] as $e |\n            . as $rows | $rows[0] as $header |\n            ([$rows[] | select(.kind == "suite_end")] | sort_by(.suite_index)) as $suites |\n            [$rows[] | select(.kind == "batch_end")] as $batches |\n            ($suites | length) == $e.totals.suites and ($batches | length) == $e.totals.batches and\n            [$suites[].suite_index] == [range(0;$e.totals.suites)] and\n            all(range(0;$e.totals.suites); . as $i | $suites[$i] as $s | $summary[0].runs[$i] as $r |\n              $s.package == $r.package and $s.activity == $r.activity and\n              $s.status == "VERIFIED" and $s.covered_cases == $r.cases and\n              $s.observed_batches == $r.batches and\n              ($s.source_set | keys) == ($e.suites[$i].source_paths | sort) and\n              ($s.source_set | type) == "object" and ($s.source_set | length) > 0 and\n              ($s.source_set_sha256 | test("^sha256:[0-9a-f]{64}$")) and\n              all($s.source_set | to_entries[]; .key as $path |\n                (.value.Package | type) == "string" and (.value.Package | length) > 0 and\n                (.value.Digest | test("^sha256:[0-9a-f]{64}$")) and\n                $header.identity.source_files[$path].sha256 == $header.catalog_source_sha256[$path] and\n                $header.catalog_source_sha256[$path] == $summary[0].source_sha256[$path])) and\n            all($batches[]; . as $b | $b.suite_index >= 0 and $b.suite_index < $e.totals.suites and\n              $b.source_set_sha256 == $suites[$b.suite_index].source_set_sha256) and\n            ($suites[70].source_set | keys) == ["ports/net/httpguts/headers.gooo"] and\n            ($suites[71].source_set | keys) == ["ports/net/httpguts/headers.gooo"] and\n            ($suites[72].source_set | keys) == ["examples/header-field-facts/main.gooo", "ports/net/httpguts/headers.gooo"] and\n            ($suites[73].source_set | keys) == ["examples/diagnostic-header-addition/main.gooo", "examples/header-field-facts/main.gooo", "examples/payload-admission/main.gooo", "ports/net/httpguts/headers.gooo", "ports/strconv/decimal.gooo"] and\n            ($suites[74].source_set | keys) == ["ports/net/httpguts/headers.gooo", "ports/net/textproto/canonical.gooo"] and\n            ($suites[75].source_set | keys) == ["examples/canonical-header-key/main.gooo", "examples/header-field-facts/main.gooo", "ports/net/httpguts/headers.gooo", "ports/net/textproto/canonical.gooo"] and\n            ($suites[76].source_set | keys) == ["ports/net/http/etag.gooo"] and\n            ($suites[77].source_set | keys) == ["ports/net/http/etag.gooo"] and\n            ($suites[78].source_set | keys) == ["examples/strong-validator-facts/main.gooo", "ports/net/http/etag.gooo", "ports/net/httpguts/headers.gooo"] and\n            ($suites[79].source_set | keys) == ["ports/net/http/precondition.gooo"] and\n            ($suites[80].source_set | keys) == ["ports/strings/last_index.gooo", "ports/strings/search.gooo"] and\n            ($suites[81].source_set | keys) == ["examples/literal-final-marker-report/main.gooo", "ports/strings/last_index.gooo", "ports/strings/literal.gooo", "ports/strings/search.gooo"]\n          \' out/ci/timing.jsonl\n'
HEADER_DECLARATIONS = ("ports/net/httpguts/headers.gooo", "examples/header-field-facts/main.gooo")
HEADER_CLAUSES = ('([.runs[:70][].cases] | add) == 170227 and ([.runs[:70][].batches] | add) == 2153 and', '.header_name_reference_cases == 232 and .header_value_reference_cases == 232 and .header_facts_reference_cases == 476 and', '.runs[70].package == "go/golang.org/x/net/http/httpguts" and .runs[70].activity == "ValidHeaderFieldName" and', '.runs[70].cases == 232 and .runs[70].passed == 232 and .runs[70].batches == 3 and', '.runs[71].package == "go/golang.org/x/net/http/httpguts" and .runs[71].activity == "ValidHeaderFieldValue" and', '.runs[71].cases == 232 and .runs[71].passed == 232 and .runs[71].batches == 3 and', '.runs[72].package == "app/header-field-facts" and .runs[72].activity == "Report" and', '.runs[72].cases == 476 and .runs[72].passed == 476 and .runs[72].batches == 15 and', '.source_sha256["ports/net/httpguts/headers.gooo"] == "a033b96ab0adb394a9d79137715d7a4170e22061e4ae9aa282f8ab3671da9f76" and', '.source_sha256["examples/header-field-facts/main.gooo"] == "4aa66012d230252bc120fdacaac555172e3a1d6c5654181aa07c7347257a805e" and', '.header_scope == "Bounded decoded validUTF8 Text<=1024 per field. Actual x/net/httpguts0.59 supplies predicate and prefix/single-byte witness references; original12-field diagnostics retain inputs. Fresh source/typed ABI/value/type/producer/raw outer saved-parent checks; independent inner typed canonical parent digest NOT_CHECKED. No arbitrary-length Go-string,HTTP2,authorization or production safety claim." and', '.header_provenance_scope == "Fresh Gooo execution and saved replay bind original Texts, exact typed12 fields, source images and generated ABI/calls, producers and raw outer receipt parents. Independent inner typed canonical prior_result_sha256 remains NOT_CHECKED; private pilot results are not fresh public acceptance." and')
CANONICAL_DECLARATIONS = ("ports/net/textproto/canonical.gooo", "examples/canonical-header-key/main.gooo")
CANONICAL_NEW_SOURCE_PATHS = ["canonical-header-key-report.workspace.json","canonical-header-key.workspace.json","cmd/verify/canonical_header_batches.go","cmd/verify/canonical_header_batches_test.go","cmd/verify/canonical_header_contracts.go","cmd/verify/canonical_header_contracts_test.go","cmd/verify/canonical_header_corpus.go","cmd/verify/canonical_header_corpus_test.go","examples/canonical-header-key/README.md","examples/canonical-header-key/contract.json","examples/canonical-header-key/corpus-accounting.json","examples/canonical-header-key/corpus.json","examples/canonical-header-key/inputs.json","examples/canonical-header-key/manual20.json","examples/canonical-header-key/port-inputs.json","ports/net/textproto/README.md","ports/net/textproto/provenance.json","third_party/GO-LICENSE","third_party/go/net/textproto/reader.go.txt","ports/net/textproto/canonical.gooo","examples/canonical-header-key/main.gooo"]
CANONICAL_CLAUSES = (
    "([.runs[:74][].cases] | add) == 171217 and ([.runs[:74][].batches] | add) == 2181 and",
    ".canonical_header_reference_cases == 1059 and .canonical_header_report_reference_cases == 1059 and",
    ".canonical_header_raw_input_occurrences == 1059 and .canonical_header_unique_keys == 1054 and",
    ".canonical_header_within_partition_duplicates == 0 and .canonical_header_cross_partition_duplicates == 5 and",
    ".canonical_header_report_fields == 8 and .canonical_header_port_source_count == 2 and .canonical_header_report_source_count == 4 and",
    ".runs[74].package == \"go/net/textproto\" and .runs[74].activity == \"CanonicalMIMEHeaderKey\" and",
    ".runs[74].cases == 1059 and .runs[74].passed == 1059 and .runs[74].batches == 22 and",
    ".runs[75].package == \"app/canonical-header-key\" and .runs[75].activity == \"Report\" and",
    ".runs[75].cases == 1059 and .runs[75].passed == 1059 and .runs[75].batches == 37 and",
    ".source_sha256[\"ports/net/textproto/canonical.gooo\"] == \"a3e09e26baa01cdafa0b27edb7eb3a32c09237b3d510bc1a80b17eb63969e1b5\" and",
    ".source_sha256[\"examples/canonical-header-key/main.gooo\"] == \"f7da15ccb5c52dff714f2338b9bedf79074d009ad07bfca9d8ddf2f81d119c06\" and",
    ".canonical_header_scope == \"Decoded validUTF8 Text0..1024bytes, including empty and invalid header names. Actual Go1.27.2 net/textproto.CanonicalMIMEHeaderKey supplies canonical Text; the importing original8-field Report retains exact original/canonical Text, changed/validity, byte lengths and first invalid offset/octet. All1059 ordered occurrences,1054 unique keys,zero within-partition and five cross-partition duplicates are retained per entry. No arbitrary-byte/unbounded Go string, HTTP parsing, authorization, effect, production or performance claim.\" and",
    ".canonical_header_provenance_scope == \"Fresh native execution and saved replay bind original Text, exact typed8 result fields and order, current2/4-source closures, lowered source/record/callee ABI, producer identities and raw outer saved parents. The scalar expectation is the actual pinned Go standard library; Report diagnostics reuse the independent existing header reference. Independent inner typed canonical prior_result_sha256 remains NOT_CHECKED; private pilot receipts are not fresh public acceptance.\" and",
)
ETAG_DECLARATIONS = ("ports/net/http/etag.gooo", "examples/strong-validator-facts/main.gooo")
ETAG_NEW_SOURCE_PATHS = (
    "etag-scan.workspace.json",
    "etag-strong-match.workspace.json",
    "strong-validator-facts.workspace.json",
    "cmd/verify/etag_batches.go",
    "cmd/verify/etag_batches_test.go",
    "cmd/verify/etag_contracts.go",
    "cmd/verify/etag_contracts_test.go",
    "cmd/verify/etag_corpus.go",
    "cmd/verify/etag_corpus_test.go",
    "cmd/verify/etag_reference.go",
    "cmd/verify/etag_native_test.go",
    "ports/net/http/README.md",
    "ports/net/http/provenance.json",
    "examples/strong-validator-facts/README.md",
    "examples/strong-validator-facts/contract.json",
    "examples/strong-validator-facts/corpus.json",
    "examples/strong-validator-facts/manual247.json",
    "examples/strong-validator-facts/inputs.json",
    "third_party/go/net/http/fs.go.txt",
    "third_party/go/net/http/fs_test.go.txt",
    "third_party/go/net/textproto/textproto.go.txt",
    "ports/net/http/etag.gooo",
    "examples/strong-validator-facts/main.gooo",
)
ETAG_CLAUSES = (
    "([.runs[:76][].cases] | add) == 173335 and ([.runs[:76][].batches] | add) == 2240 and",
    ".etag_scan_reference_cases == 192 and",
    ".etag_strong_reference_cases == 31 and",
    ".etag_report_reference_cases == 24 and",
    ".etag_scan_unique_inputs == 190 and",
    ".etag_scan_duplicate_occurrences == 2 and",
    ".etag_report_fields == 13 and",
    ".etag_port_source_count == 1 and",
    ".etag_report_source_count == 3 and",
    ".etag_scope == \"Bounded decoded Unicode-scalar validUTF8 Text0..1024bytes per operand, including empty/NUL/control/invalidETag grammar. Private pinned Go scanETag/etagStrongMatch are exposed as dedicated Gooo APIs; the original13field Report separates raw header validity from parsed tag facts. All192 scanner occurrences,31 raw pairs and24 facts inputs retained. No arbitrary-byte/unbounded strings, fullIfMatch, currentresource authority, effects or production-safety claim.\" and",
    ".etag_provenance_scope == \"Fresh native and saved replay bind original inputs, exact2/13 ordered typed fields, current1/3source closures, lowered bodies, generated ABI, Report3callee ordinals0/1/2, exact explicit Go binary and raw saved parent. All247 independent Go expectations are separately compared with literal anchors. Inner canonical/typed-plan hashing remains NOT_CHECKED; a label or private firstcase is not public acceptance.\" and",
    ".runs[76].package == \"go/net/http/etag\" and .runs[76].activity == \"ScanETag\" and",
    ".runs[76].cases == 192 and .runs[76].passed == 192 and .runs[76].batches == 3 and",
    ".runs[77].package == \"go/net/http/etag\" and .runs[77].activity == \"ETagStrongMatch\" and",
    ".runs[77].cases == 31 and .runs[77].passed == 31 and .runs[77].batches == 2 and",
    ".runs[78].package == \"app/strong-validator-facts\" and .runs[78].activity == \"Report\" and",
    ".runs[78].cases == 24 and .runs[78].passed == 24 and .runs[78].batches == 2 and",
    ".source_sha256[\"ports/net/http/etag.gooo\"] == \"2fc5b2c19acf8c040173859309dd687be37bca1c2c56b49efa293a8df98d42c4\" and",
    ".source_sha256[\"examples/strong-validator-facts/main.gooo\"] == \"a7002b9db59ff68e732b66ef27c86426b8fff325cfb15f2665a0d0cf04f17a01\" and",
)
PATH_ALIAS_DECLARATIONS = ("ports/path/clean.gooo", "examples/path-alias-facts/main.gooo")
PATH_ALIAS_CLAUSES = (
    '([.runs[:68][].cases] | add) == 160266 and ([.runs[:68][].batches] | add) == 2072 and',
    '.path_clean_reference_cases == 9841 and .path_alias_reference_cases == 120 and',
    '.runs[68].package == "go/path" and .runs[68].activity == "Clean" and',
    '.runs[68].cases == 9841 and .runs[68].passed == 9841 and .runs[68].batches == 77 and',
    '.runs[69].package == "app/path-alias-facts" and .runs[69].activity == "Report" and',
    '.runs[69].cases == 120 and .runs[69].passed == 120 and .runs[69].batches == 4 and',
    '(.source_sha256 | length) == 67 and',
    '.source_sha256["ports/path/path.gooo"] == "593caad2ac3b443d353775a9f4e168282af1ab70181bcbc483c4b379c9ee3de3" and',
    '.source_sha256["ports/path/clean.gooo"] == "8430fc094b000fbd25b9d081408a9e8cdb59fca942f7a5541e7695c0291b790d" and',
    '.source_sha256["examples/path-alias-facts/main.gooo"] == "5b0c8436bf95c923730af71a316d3897839d734c53c0223fffad5b555488182e" and',
    '.path_alias_scope == "Decoded valid UTF-8 Text up to1024 bytes, slash-based lexical Go path.Clean. Original11-field Report preserves both original inputs and exposes clean/absolute/change/equality/alias facts. Actual Go path.Clean and path.IsAbs supply independent finite expectations; no URL, router, filesystem, authorization or safety claim." and',
    '.path_alias_provenance_scope == "Fresh native execution and saved replay check original Texts, exact result/field identities and order, selected current source bytes, generated bodies, producer identities and raw outer receipt parents. This generic path does not independently check inner prior_result_sha256 or general callee ABI; private receipts are not fresh public acceptance." and',
)
IF_MATCH_NEW_SOURCE_PATHS = (
    'cmd/if-match-input/main.go',
    'cmd/if-match-input/main_test.go',
    'cmd/verify/if_match_contracts.go',
    'cmd/verify/if_match_corpus.go',
    'cmd/verify/if_match_reference.go',
    'cmd/verify/if_match_test.go',
    'if-match.workspace.json',
    'ports/net/http/if-match-corpus.json',
    'ports/net/http/if-match-provenance.json',
    'ports/net/http/if-match-request.json',
    'ports/net/http/precondition.gooo',
    'tools/check-if-match-inputs.py',
    'tools/check-if-match-inputs_test.py',
)
IF_MATCH_CLI_CLAUSES = (
    '(.planned_by_entry | keys) == ["app/literal-cut-report:Report", "app/literal-text-report:Report", "go/net/http/precondition:IfMatchCondition", "go/strings:Count", "go/strings:Cut", "go/strings:CutPrefix", "go/strings:CutSuffix", "go/strings:Index"] and',
    'all(.cases[:196][]; .entry != "go/net/http/precondition:IfMatchCondition") and',
    'all(.cases[196:][]; .entry == "go/net/http/precondition:IfMatchCondition") and',
)
IF_MATCH_UNIT = 'python3 -I -B tools/check-if-match-inputs_test.py'
IF_MATCH_STEP_NAME = 'Verify strict If-Match inputs against the fresh native references'
IF_MATCH_COMMAND = 'python3 -I -B tools/check-if-match-inputs.py --root "$PWD" --compiler "$PWD/.gooo" --input-command "$PWD/out/if-match-input" --go "$(go env GOROOT)/bin/go" --out "$PWD/out/if-match-input-route"'
IF_MATCH_DECLARATIONS = ("ports/net/http/precondition.gooo",)
LAST_INDEX_DECLARATIONS = ("ports/strings/last_index.gooo", "examples/literal-final-marker-report/main.gooo")
LAST_INDEX_NEW_SOURCE_PATHS = (
    "cmd/verify/last_index_corpus.go", "cmd/verify/last_index_corpus_test.go",
    "cmd/verify/last_index_contracts.go", "cmd/verify/last_index_contracts_test.go",
    "literal-final-marker-report.workspace.json",
    "examples/literal-final-marker-report/inputs.json", "examples/literal-final-marker-report/README.md",
    "tools/check-last-index-inputs.py", "tools/check-last-index-inputs_test.py",
    *LAST_INDEX_DECLARATIONS,
)
LAST_INDEX_CLAUSES = (
    '([.runs[:80][].cases] | add) == 173866 and ([.runs[:80][].batches] | add) == 2252 and',
    '.last_index_reference_cases == 1694 and .final_marker_report_reference_cases == 89 and',
    '.runs[80].package == "go/strings" and .runs[80].activity == "LastIndex" and',
    '.runs[80].cases == 1694 and .runs[80].passed == 1694 and .runs[80].batches == 50 and',
    '.runs[81].package == "app/literal-final-marker-report" and .runs[81].activity == "Report" and',
    '.runs[81].cases == 89 and .runs[81].passed == 89 and .runs[81].batches == 3 and',
    '.source_sha256["ports/strings/last_index.gooo"] == "87080f249a41bba3ef7d757968573c2b735b584f2d5306f23e2889c2411a9947" and',
    '.source_sha256["examples/literal-final-marker-report/main.gooo"] == "04f9b5b47f7c1c205a172cb8d40a8ecee5e3f264e9e0d387fb164ee6fe1bcf04" and',
    '.last_index_scope == "LastIndex finite corpus v1: 1694 unique decoded valid-UTF8 Text pairs, each argument <=1024 bytes. Independent pinned Go strings.LastIndex preserves the final literal byte offset, overlapping final matches, -1 on misses and byte length for an empty needle. Includes26 separately stated anchors, complete21x21 short pairs over [a,b,NUL,é] of0..2tokens, all1024 final byte offsets, length boundaries and64 fixed-PCG candidates, deduplicated in order. No arbitrary-byte or longer Go strings, normalization, whole-package, exhaustive-string, performance or adoption claim." and',
    '.final_marker_report_scope == "FinalMarkerReport v1 finite corpus: 89 unique representative pairs from26 manual anchors and64 fixed-PCG candidates. Go strings.Index/LastIndex/Count are independently called for all8 fields; original Texts and UTF8 byte lengths are preserved. LastIndex includes overlaps while Count remains non-overlapping. This separate v1 result does not extend or rebind the existing7-field literal-text Report, its9313 pairs, compact policy or saved evidence. No production enforcement or safety claim." and',
    '.last_index_finite_provenance_scope == "Fresh LastIndex and separate FinalMarkerReport v1 suites compare independent Go oracle values and preserve original Texts, exact scalar/8-field result identities and order, selected source images, entry and producer identities, generated bytes and raw outer saved-receipt bindings. Inner prior_result_sha256, full generation-report provenance and composition/runtime typed-plan equality are NOT_CHECKED by this finite path. Existing literal-text Report seven-field ABI, all9313 original pairs and compact policy remain unchanged." and',
)
IF_MATCH_CLAUSES = (
    '([.runs[:79][].cases] | add) == 173582 and ([.runs[:79][].batches] | add) == 2247 and',
    '.if_match_reference_cases == 284 and .if_match_input_ports == 2 and .if_match_result_fields == 1 and',
    '.if_match_port_source_count == 1 and',
    '.if_match_conditions == {condNone: 0, condTrue: 1, condFalse: 2} and',
    '.runs[79].package == "go/net/http/precondition" and .runs[79].activity == "IfMatchCondition" and',
    '.runs[79].cases == 284 and .runs[79].passed == 284 and .runs[79].batches == 5 and',
    '.source_sha256["ports/net/http/precondition.gooo"] == "9924cde4a2e01222658929a87007d6e34db85f2d4f49c057d110bb8d1bf05aeb" and',
    '.if_match_scope == "Selected If-Match and current ETag are two unchanged decoded Unicode-scalar validUTF8 Text0..1024bytes each, including empty/NUL/control/invalid grammar. Private Go checkIfMatch compatibility returns original condNone0/condTrue1/condFalse2 as one required int64 field. Initial empty differs from whitespace-only; star suffix, adjacent tags and early strong match are preserved. No Header.Get or multiple-field selection, HTTP adapter, authenticated current version, authorization, effects or CAS guarantee." and',
    '.if_match_provenance_scope == "All284 ordered input occurrences are retained; fresh expectations come only from the pinned original Go enum/checkIfMatch adapter with three explicit lookup substitutions and unchanged scanETag/etagStrongMatch/TrimString. Fresh native execution and saved replay bind original two inputs, one ordered integer result field, current one-source closure, lowered body/record/generated ABI, exact explicit Go binary and raw saved parent. Independent inner canonical/typed-plan hashing remains NOT_CHECKED; historical private receipts do not establish current-source acceptance." and',
)
IF_MATCH_ROUTE_CLAUSES = (
    '.schema == "gooo/if-match-input-route/v1"',
    '.status == "FIVE_INPUT_ROUTES_AND35_PRELAUNCH_REFUSALS_PASS"',
    '.failure == null',
    '.positive_requests == 5',
    '.prelaunch_refusals == 35',
    '.actual_Gooo_commands == 10',
    '.typed_fields_checked == 10',
    '(.observations | length) == 10',
    '.source_tool_unchanged == true',
    '.processes_closed == true',
)
IF_MATCH_STEP_SHA = '34322d6d1d7ce6e2cdbaea58e5a92ae765a9e3e27acddcbc0b45dc80aa037de0'

AUTOBOT_STEP = 'Verify original AutoBot resolver adaptation'
AUTOBOT_COMMAND = 'python3 -I -B examples/autobot-resolver/verify.py --root "$PWD" --compiler "$PWD/.gooo" --go "$(go env GOROOT)/bin/go" --input-command "$PWD/out/autobot-line-input" --out "$PWD/out/autobot-resolver"'
AUTOBOT_BUILD = 'go build -mod=readonly -o out/autobot-line-input ./cmd/autobot-line-input'
AUTOBOT_CONTROLS = 'python3 -I -B examples/autobot-resolver/verify_test.py'
AUTOBOT_CLAUSES = (
    '.schema == "gooo/autobot-resolver-verification/v1"',
    '.status == "ORIGINAL_PYTHON_GO_AND_GOOO_REPLAY_204_PASS"',
    '.failure == null and .stage == "complete"', '.stage == "complete"', '.rows == 204', '.batches == 26',
    '.Gooo_commands == 52', '.helper_commands == 26',
    '.input_echoes_each_phase == 1836', '.output_fields_each_phase == 1632',
    '.finite_total == 0', '.model_calls == 0', '.source_tool_unchanged == true',
    '.processes_closed == true', '.owned_work_removed == true',
    '(.verified | length) == 52', '(.commands | length) == 81',
    '([.verified[] | select(.operation == "execute") | .rows] | add) == 204',
    '([.verified[] | select(.operation == "replay") | .rows] | add) == 204',
    'all(.commands[]; .exit_code == 0 and .failure == null and .observed_owned_processes_remaining == [])',
    '.compiler_revision == "83928fa4538245abe33b08549da47f8cbdc50ed2"',
    "' out/autobot-resolver/summary.json",
)

EDITABLE_POLICY_STEP = 'Verify editable API policy source comparison'
EDITABLE_POLICY_COMMAND = 'python3 -I -B examples/editable-api-policy/run.py --root "$PWD" --compiler "$PWD/.gooo" --go "$(go env GOROOT)/bin/go" --out "$PWD/out/editable-api-policy" --check-demo'
EDITABLE_POLICY_HELPER_COMMAND = 'python3 -I -B tools/check-editable-policy-helpers.py --root "$PWD" --compiler "$PWD/out/source-inspection036/gooo" --go "$(go env GOROOT)/bin/go" --out "$PWD/out/editable-api-policy-helpers"'
EDITABLE_POLICY_SOURCE_SETUP = """          # The default pinned839 check above remains independent of this opt-in compiler.
          test ! -e out/source-inspection036
          mkdir out/source-inspection036
          curl --fail --location --silent --show-error --connect-timeout 15 --max-time 120 --output out/source-inspection036/gooo-linux-amd64.tar.gz https://github.com/kimjooyoon/meta-ontology-go/releases/download/v0.6.36-dev/gooo-linux-amd64.tar.gz
          printf '%s  %s\\n' '9dbd1c08b855a3c27ae43a8050531afaba724d90b24d4f72041db3a8a4a285b0' 'out/source-inspection036/gooo-linux-amd64.tar.gz' | sha256sum --check --strict
          test "$(tar -tzf out/source-inspection036/gooo-linux-amd64.tar.gz)" = gooo
          tar --no-same-owner --no-same-permissions -xzf out/source-inspection036/gooo-linux-amd64.tar.gz -C out/source-inspection036 gooo
"""
EDITABLE_POLICY_HELPER_CLAUSES = (
    '.schema == "gooo/editable-api-policy-helpers-CI/v1"',
    '.status == "HELPER_EXTRACTION_AND_OFFICIAL_REPLAY_PASS"',
    '.failure == null', '.variant_runs == 2', '.rows == 3', '.typed_fields_compared == 90',
    '.default_helper_refused == true', '.default_package_commands == 0',
    '.prepared_inspections == 4', '.positive_receipts == 8',
    '.source_tool_unchanged == true', '.processes_closed == true', '.owned_work_removed == true',
    '.model_calls == 0', '.compiler_revision == "31497af3beaef210b5a075867b0de76cf55ee3ca"',
    "' out/editable-api-policy-helpers/summary.json",
)
EDITABLE_POLICY_COMMON = 'python3 -I -B examples/editable-api-policy/run_test.py'
EDITABLE_POLICY_DECLARATIONS = ('examples/editable-api-policy/before.gooo', 'examples/editable-api-policy/after.gooo')
EDITABLE_POLICY_CLAUSES = (
    '.schema == "gooo/editable-api-policy-comparison/v1"',
    '.status == "EDITABLE_POLICY_COMPARISON_AND_OFFICIAL_REPLAY_PASS"',
    '.failure == null', '.rows == 3', '.positive_receipts == 4', '.package_commands == 5',
    '.input_echoes_checked == 108', '.typed_output_fields_checked == 180',
    '.finite_total == 0', '.model_calls == 0', '.demo_checked == true',
    '.cross_workspace_refused == true', '.cross_workspace_refusal_stage == "manifest"',
    '.source_tool_unchanged == true',
    '.processes_closed == true', '.owned_work_removed == true',
    '([.comparisons[].changed_count] == [0,2,0])',
    '([.comparisons[].status] == ["UNCHANGED","NEWLY_DENIED","UNCHANGED"])',
    '.compiler_revision == "83928fa4538245abe33b08549da47f8cbdc50ed2"',
    "' out/editable-api-policy/summary.json",
)

VERSIONED_GRANT_STEP = 'Verify versioned grant change composition'
VERSIONED_GRANT_COMMAND = 'python3 -I -B examples/versioned-grant-change/verify.py --root "$PWD" --compiler "$PWD/.gooo" --go "$(go env GOROOT)/bin/go" --module-cache "$(go env GOMODCACHE)" --out "$PWD/out/versioned-grant-change"'
VERSIONED_GRANT_COMMON_COMMAND = 'python3 -I -B examples/versioned-grant-change/verify_test.py'
VERSIONED_GRANT_DECLARATION = 'examples/versioned-grant-change/main.gooo'
VERSIONED_GRANT_CLAUSES = (
    '.schema == "gooo/versioned-grant-change-verification/v1"',
    '.status == "FINITE48_FRESH_REFERENCE_NATIVE_REPLAY_PASS"',
    '.failure == null',
    '.cases == 48',
    '.input_echoes_each_phase == 288',
    '.typed_fields_each_phase == 576',
    '.source_images == 12',
    '.model_calls == 0',
    '.source_tool_unchanged == true',
    '.processes_closed == true',
    '.owned_work_removed == true',
    '.compiler_revision == "83928fa4538245abe33b08549da47f8cbdc50ed2"',
    "' out/versioned-grant-change/summary.json",
)

COMMON = {
    "Classify the exact checked PR tree", "Check the Go observer",
    "Check pinned Unicode table generation",
    "Build and identify the pinned compiler from a clean checkout",
    "Check Gooo declarations",
}
LAST_INDEX_INPUT_STEP_NAME = "Observe malformed LastIndex inputs through the actual CLI"
LAST_INDEX_INPUT_COMMAND = 'python3 -I -B tools/check-last-index-inputs.py --root "$PWD" --compiler "$PWD/.gooo" --go "$(go env GOROOT)/bin/go" --out "$PWD/out/last-index-input-rejections"'
LAST_INDEX_INPUT_UNIT = "python3 -I -B tools/check-last-index-inputs_test.py -v"
LAST_INDEX_INPUT_STEP_SHA = "f04c072dbdb48ea1a4ddddfae92d2c427685bc250dca2209ff02357c3c883082"
HEAVY = {
    URL_COMPONENT_STEP,
    LAST_INDEX_INPUT_STEP_NAME,
    AUTOBOT_STEP,
    EDITABLE_POLICY_STEP,
    VERSIONED_GRANT_STEP,
    IF_MATCH_STEP_NAME,
    HEADER_ADDITION_INPUT_STEP_NAME, HEADER_ADDITION_HOST_STEP_NAME,
    HEADER_WIRE_STEP_NAME,
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
    r"tools/probe\.sh|tools/check-(?:fits-crop|literal-text|last-index)-inputs\.py|decision-policy-pilot/run\.sh|cel-api-policy/verify\.py|agentmesh-service-policy/verify\.py|api-policy-revision/verify\.py|tools/check-api-policy-public-consumer\.py|tools/check-path-facts\.py|tools/check-url-components\.py|tools/check-redelivery-policy\.py|tools/check-ipv4\.py|tools/check-header-facts-inputs\.py|tools/check-header-addition-inputs\.py|tools/check-header-addition-host\.py|examples/versioned-grant-change/verify\.py|examples/editable-api-policy/run\.py|tools/check-editable-policy-helpers\.py|examples/autobot-resolver/verify\.py)"
)
PUBLIC_CONSUMER_COMMAND = 'python3 -B tools/check-api-policy-public-consumer.py --go "$(go env GOROOT)/bin/go" --revision "$PUBLIC_API_REVISION" --out "$PWD/out/api-policy-public-consumer" --bundle-cli'
PUBLIC_CONSUMER_CLAUSES = (
    '.schema == "gooo/api-policy-public-consumer/v1"',
    '.status == "PUBLIC_API_POLICY_CONSUMER_PASS"',
    '.acceptance.facts == 9', '.acceptance.positive_cases == 2',
    '.acceptance.decision_fields == 15', '.acceptance.difference_flags == 15',
    '.acceptance.summary_fields == 2', '.acceptance.public_asset_count == 38',
    '.acceptance.public_assets_exact == true', '.acceptance.public_generation_exact == true',
    '.acceptance.original_assets_exact == true', '.acceptance.original_generation_unchanged == true',
    '.acceptance.substitution_refused == true', '.acceptance.runtime_empty_path == true',
    '.acceptance.consumer_no_replace_or_workspace == true', '.acceptance.direct_install == true', '.acceptance.direct_install_cases == 2',
    '.acceptance.direct_install_tampered_report_refused == true',
    '.acceptance.direct_install_batch_rows == 3',
    '.acceptance.direct_install_batch_tampered_report_refused == true',
    '.acceptance.substitution_stdout_bytes == 0',
    '.acceptance.original_version == "v0.0.0-20261014100709-8cddcc4e65af"',
    '.acceptance.original_commit == "8cddcc4e65afb42431f8f42a86f29859ae2f4d69"',
    '.cli_bundle.status == "VALIDATED_NATIVE_CLI_BUNDLE"',
    '.cli_bundle.platform == "linux/amd64"',
    '.cli_bundle.archive_readback_verified == true',
    '(.cli_bundle.members | length) == 10',
    '(.cli_bundle.archive.bytes > 0 and .cli_bundle.archive.bytes <= 33554432)',
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
    LAST_INDEX_INPUT_STEP_NAME: {LAST_INDEX_INPUT_COMMAND},
}


# The common checks move byte-for-byte; both isolated jobs classify their own
# exact full-history checkout. These are source-integrity locks, not CI PASS.
OBSERVER_HEADER_SHA = '46cb0e7392dd3f3fb9562bccea6b27e5194ffcf59e1a58bab4c07b99aba08bd9'
NATIVE_HEADER_SHA = '584d7a22663deb57be052a69d58f2b19e6233a7eefb7d4ce6ad1d5b650d48e69'
# Add the URL sidecar's pure controls once. Removing that exact new line must
# recover all previously reviewed observer bytes, including its full race scope.
OBSERVER_PRIOR_STEP_SHA = '95a801fa3b7e40b3eb3c229d90a9f9e378ffab67557c3c6bfb563c175810b186'
OBSERVER_STEP_SHA = 'c7c6e45c3f63a12d119cd151a3d40e95efe734bd0247add839bbe2832f76f1c3'
OBSERVER_UPLOAD_NAME = 'Preserve Go observer test timing'
OBSERVER_UPLOADS = {OBSERVER_UPLOAD_NAME}
OBSERVER_UPLOAD_SHA = 'd5f0816bb5edd218d4e71c5ee870c0adfad7cc8f292c37b1b2093331b1fc4d3f'
GENERATOR_STEP_SHA = '081263b551a4d28aa8b46dc025fe951c62d91d13e164cb7927b70d7270aaf399'
CLASSIFIER_STEP_SHA = '1864b9e0f27cc73595dd863c71640050cf046b2ec67cfd02c6ba5a8ebd11a61a'
CHECKOUT_STEP_SHA = 'b60ba5d7bf95691b8baec3a3e402aeba552fbcb4a353d7de8946023e73e79603'
SETUP_STEP_SHA = '742468cc6b855d4f2c2d9a5d45000899bc244f524539bad21505ef94018dceb2'
MOVED = {'Check the Go observer', 'Check pinned Unicode table generation'}
ETAG_PROJECT_STEP = 'Verify the exported editable ETag project'
ETAG_PROJECT_COMMANDS = (
    'python3 -B tools/export-etag-project.py --out "$PWD/out/etag-project"',
    'go_executable="$(go env GOROOT)/bin/go"',
    '"$go_executable" -C out/etag-project mod download',
    '"$go_executable" -C out/etag-project mod verify',
    'GOPROXY=off GOSUMDB=off "$go_executable" -C out/etag-project vet -mod=readonly ./...',
    'GOPROXY=off GOSUMDB=off "$go_executable" -C out/etag-project run -race -mod=readonly . --compiler "$PWD/.gooo" --go "$go_executable" --out "$PWD/out/etag-project-evidence" --mutation-check',
)
ETAG_PROJECT_CLAUSES = (
    '.schema == "gooo/portable-etag-check/v1"', '.decision == "PASS"',
    '.baseline_rows == 247', '.verified_returned_scalars == 727', '.baseline_execute_commands == 7',
    '.mutation_checked == true', '.mutation_rows == 19', '.mutation_execute_commands == 1',
    '.source_tool_unchanged_after == true', '.first_failure == null',
    '.compiler_revision == "83928fa4538245abe33b08549da47f8cbdc50ed2"',
    '.corpus_sha256 == "sha256:ce9015cc12bc37e785761f4ee6907a77603082cd69b4d8edc80a634ff4478345"',
    '.mutation_witness == "CRLF-trim-does-not-sanitize-header: single_strong false→true"',
    "' out/etag-project-evidence/summary.json",
)
POLICY_HEAVY = {
    URL_COMPONENT_STEP,
    AUTOBOT_STEP,
    EDITABLE_POLICY_STEP,
    VERSIONED_GRANT_STEP,
    'Verify the offline decision-policy experiment',
    'Verify the imported CEL API policy',
    'Verify the sealed API policy revision comparison',
    'Verify the Agent Mesh service-pattern adaptation',
}
# These four source-independent producers move byte-for-byte to the existing
# policy runner. This roster does not add jobs, builds, cases or docs exemptions.
INDEPENDENT_STEPS = (
    'Verify the public API policy consumer',
    'Verify the isolated native path facts sidecar',
    'Verify the original automatic redelivery policy',
    'Verify the isolated native IPv4 address facts sidecar',
)
INDEPENDENT_HEAVY = set(INDEPENDENT_STEPS)
INDEPENDENT_ARTIFACT_PATHS = ('out/api-policy-public-consumer/', 'out/path-facts/',
                              'out/redelivery-policy/', 'out/ipv4/')
POLICY_PILOT_UPLOAD = 'Upload offline policy experiment evidence'
POLICY_UPLOAD = 'Upload policy execution and diagnostic evidence'
POLICY_PRIOR_UPLOAD_SHA = '797517ddd218879f190d48c8348a7e29a882ef51d6e6c4e6118b22cfb0f5881a'
POLICY_DOC_REPORT = 'Report documentation-only policy scope'
POLICY_DOC_ARTIFACT = 'Preserve documentation-only policy scope'
POLICY_HEADER_SHA = '3d5466a5dfca722bf38d6d81f26ff8f425188d473f6a0b9ae46cdd528d209cfe'
POLICY_SETUP_SHA = '5ccf03022b3caeab1cc1fd46bbb3fa8b54e4fc0addba08c7d39d17f449f51703'
POLICY_STEP_SHAS = {
    URL_COMPONENT_STEP: '55e04dd80bd317c698ae9326310b5830aebd07069b281b29ecd297cb5d4f5d6e',
    AUTOBOT_STEP: '7bb749f5983a2e35fdcde9067fc1646833bf2034b036e28b7e5f60f0a4fb7006',
    EDITABLE_POLICY_STEP: '91ecaa02b36b1a4f39fdeb3b932282373fa1e6137038e4b2e52a70290350a8cc',
    'Verify the isolated native IPv4 address facts sidecar': '6ce5d2f5e75f9ff9a7728718567ffb039c74232ef4e254ea244b7dc02fbc9ed8',
    'Verify the original automatic redelivery policy': '2ce6496cbee29d77c9334f1f32dc518c96a12c193e37ab569144bdc4fd1a864e',
    'Verify the isolated native path facts sidecar': 'b90f403ce804adea78dacebf554c45f3e21b1faf89b942e83f32aca2847f9df5',
    'Verify the public API policy consumer': '57b026d0eae0853a4e2f3ea4cfc431fbcf524cc75d340b70240ab19cbe84cfa0',
    'Build and identify the pinned compiler from a clean checkout': 'c7273984211d14f1b50f66438f1d90f68a42e8ad58be1d0d1d2a0213b0b4d2ae',
    'Verify the offline decision-policy experiment': '2d8e714f7dfb2963a682ce702485e3bccfaa7439cbaa67f7da6741aff6ce7c48',
    'Verify the imported CEL API policy': '064873f6b6288062964bde88c2f1f436bdc7316e586ae6066a4b3623f057b708',
    'Verify the sealed API policy revision comparison': 'd814d5880193ec157b5f08182302fa3b25904dfaecc6fde2e65fa6e936f5a2b3',
    'Verify the Agent Mesh service-pattern adaptation': '9af078924730a896c5c669e167803a2e3780e277cdcbe6231233b2cac97a471c',
    'Upload offline policy experiment evidence': 'e4adbb294f45d4dbe4af56338b800f0d637dda27ec83055f1d799d63dac60392',
    ETAG_PROJECT_STEP: 'f11ef3c03ed5cc096b30b3d44c8a20abeb73bc5e840b8033114d94208d64f4c7',
    VERSIONED_GRANT_STEP: '04e88171f2da9d7c19b58889ad27f71359456780fce7ef11dd03c3172cc7a860',
    'Upload policy execution and diagnostic evidence': 'bce999d657bbc2506d6863b5ba131e64630b6151d7635bc325d4b79865d98334',
    'Report documentation-only policy scope': '472165f458bfda43733f2a1ea6bc2236c19b425b5887f3417bbe1a9d9bcab6dd',
    'Preserve documentation-only policy scope': '185d1afbfd001734dc649c227526a84d5dd993bfcd9c1243e4a1966f13db60f7',
}
JOB_ORDER = ('observer-checks', 'native-execution', 'policy-native-checks', 'browser-demo', 'deploy-browser-demo')
DEPLOY_IF = "${{ always() && !cancelled() && github.event_name == 'push' && github.ref == 'refs/heads/main' && needs.observer-checks.result == 'success' && needs.native-execution.result == 'success' && needs.policy-native-checks.result == 'success' && needs.browser-demo.result == 'success' }}"
DEPLOY_NEEDS = '    needs: [observer-checks, native-execution, policy-native-checks, browser-demo]'


def excluded_source_projection(name):
    """A filter on the frozen expected set must never replace exact membership."""
    return TIMING_HEADER_ASSERTION.replace(
        NATIVE_SOURCE_PATHS_ASSERTION,
        "(.identity.source_files | keys) == ($e.source_paths - [" + json.dumps(name) + "])", 1)


def literal_commands(chunk):
    # Continuation joining is deliberately limited to the reviewed fixed layout.
    joined = chunk.replace("\\\n          ", " ")
    return [" ".join(line.split()) for line in
            re.findall(r"^          ((?:\./\.gooo|go run (?:-mod=readonly )?\./cmd/|python3 ).*)$", joined, re.M)]


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
        expected = {'Classify the exact checked PR tree'} | MOVED | OBSERVER_UPLOADS if job == 'observer-checks' else (COMMON | HEAVY | PREFLIGHT | UPLOADS | DOC_STEPS) - MOVED - POLICY_HEAVY - INDEPENDENT_HEAVY - {POLICY_PILOT_UPLOAD}
        require(set(named) == expected, "producer step roster changed: " + job)
        if job == 'observer-checks':
            require(hashlib.sha256(named['Check the Go observer'].encode()).hexdigest() == OBSERVER_STEP_SHA and
                    hashlib.sha256(named['Check pinned Unicode table generation'].encode()).hexdigest() == GENERATOR_STEP_SHA,
                    "common observer/generator commands changed without guard review")
            observer = named['Check the Go observer']
            url_unit_line = '          ' + URL_COMPONENT_COMMON + '\n'
            require(observer.count(url_unit_line) == 1 and
                    hashlib.sha256(observer.replace(url_unit_line, '', 1).encode()).hexdigest() == OBSERVER_PRIOR_STEP_SHA,
                    'URL pure controls may only add one unconditional line to the prior observer bytes')
            require(hashlib.sha256(named[OBSERVER_UPLOAD_NAME].encode()).hexdigest() == OBSERVER_UPLOAD_SHA,
                    "observer timing artifact pin, paths or failure retention changed")
            require(chunk.index(unnamed[1]) < chunk.index(named['Check the Go observer']) <
                    chunk.index(named['Check pinned Unicode table generation']) <
                    chunk.index(named[OBSERVER_UPLOAD_NAME]), "observer check/artifact order changed")
        else:
            native_unnamed = unnamed
        for name, step in named.items():
            require(name not in all_named or name == 'Classify the exact checked PR tree', "duplicate cross-job step")
            all_named[name] = step
    policy_named, _ = policy_steps(jobs['policy-native-checks'])
    for name in POLICY_HEAVY | INDEPENDENT_HEAVY | {POLICY_PILOT_UPLOAD}:
        require(name not in all_named, 'policy producer was duplicated in another job: ' + name)
        all_named[name] = policy_named[name]
    return all_named, native_unnamed


def policy_steps(chunk):
    header, named, unnamed = job_steps(chunk)
    require(hashlib.sha256(header.encode()).hexdigest() == POLICY_HEADER_SHA,
            'policy producer header must be independent, unconditional and pinned')
    require('continue-on-error:' not in chunk, 'policy producer failure became optional')
    require(len(unnamed) == 2 and
            hashlib.sha256(unnamed[0].encode()).hexdigest() == CHECKOUT_STEP_SHA and
            hashlib.sha256(unnamed[1].encode()).hexdigest() == POLICY_SETUP_SHA,
            'policy producer requires exact checkout and full-only pinned Go setup')
    expected = {'Classify the exact checked PR tree',
                'Build and identify the pinned compiler from a clean checkout',
                POLICY_PILOT_UPLOAD, POLICY_UPLOAD, POLICY_DOC_REPORT, POLICY_DOC_ARTIFACT, ETAG_PROJECT_STEP} | POLICY_HEAVY | INDEPENDENT_HEAVY
    require(set(named) == expected, 'policy producer exact step roster changed')
    classifier = named['Classify the exact checked PR tree']
    identity = named['Build and identify the pinned compiler from a clean checkout']
    require(hashlib.sha256(classifier.encode()).hexdigest() == CLASSIFIER_STEP_SHA,
            'policy classifier must bind its own actual checked tree')
    require(set(POLICY_STEP_SHAS) == expected - {'Classify the exact checked PR tree'},
            'policy exact reviewed step digest roster changed')
    for name, sha in POLICY_STEP_SHAS.items():
        require(hashlib.sha256(named[name].encode()).hexdigest() == sha,
                'policy step must retain its exact source/commands/gates: ' + name)
    public = named[INDEPENDENT_STEPS[0]]
    require(chunk.index(unnamed[0]) < chunk.index(classifier) < chunk.index(unnamed[1]) < chunk.index(public) < chunk.index(identity),
            'policy checkout/classifier/Go/public-consumer/compiler order changed')
    ordered = (URL_COMPONENT_STEP,) + INDEPENDENT_STEPS[1:] + ('Verify the offline decision-policy experiment', 'Verify the imported CEL API policy',
               'Verify the sealed API policy revision comparison', 'Verify the Agent Mesh service-pattern adaptation',
               ETAG_PROJECT_STEP, VERSIONED_GRANT_STEP, EDITABLE_POLICY_STEP, AUTOBOT_STEP, POLICY_PILOT_UPLOAD, POLICY_UPLOAD, POLICY_DOC_REPORT, POLICY_DOC_ARTIFACT)
    previous = chunk.index(identity)
    for name in ordered:
        require(chunk.index(named[name]) > previous, 'policy producer/evidence/report order changed: ' + name)
        previous = chunk.index(named[name])
    require(field(unnamed[1], 'if') == FULL and field(identity, 'if') == FULL,
            'policy Go setup and compiler build must stay full-only')
    for name in POLICY_HEAVY | INDEPENDENT_HEAVY:
        require(field(named[name], 'if') == FULL, 'policy native producer must stay full-only: ' + name)
    url_components = named[URL_COMPONENT_STEP]
    require(identity + url_components + named[INDEPENDENT_STEPS[1]] in chunk,
            'URL sidecar must immediately follow the pinned compiler and precede the unchanged path sidecar')
    require(field(url_components, 'timeout-minutes') == '5' and
            literal_commands(url_components) == [URL_COMPONENT_COMMAND],
            'URL sidecar must retain its finite timeout and current root/compiler/Go/fresh output command')
    for token in URL_COMPONENT_CLAUSES:
        require(url_components.count(token) == 1, 'URL component summary gate missing: ' + token)
    require('set -euo pipefail' in url_components and '|| true' not in url_components,
            'URL component execution and summary must preserve the first failure')
    exported = named[ETAG_PROJECT_STEP]
    require(field(exported, 'if') == FULL and field(exported, 'timeout-minutes') == '8',
            'exported ETag check must keep the fail-closed full-only scope and finite timeout')
    for token in ETAG_PROJECT_COMMANDS + ETAG_PROJECT_CLAUSES:
        require(exported.count(token) == 1, 'exported ETag command/source/count gate missing: ' + token)
    require('set -euo pipefail' in exported and 'export GOENV=off' in exported,
            'exported ETag dependency/source checks must fail on first failure')

    versioned = named[VERSIONED_GRANT_STEP]
    require(field(versioned, 'if') == FULL and field(versioned, 'timeout-minutes') == '8',
            'versioned grant check must stay full-only with its bounded timeout')
    require(literal_commands(versioned) == [VERSIONED_GRANT_COMMAND],
            'versioned grant must use current root/compiler/Go/module cache and fresh output')
    for token in VERSIONED_GRANT_CLAUSES:
        require(versioned.count(token) == 1, 'versioned grant summary gate missing: ' + token)
    require('set -euo pipefail' in versioned and '|| true' not in versioned,
            'versioned grant execution and summary must preserve the first failure')

    editable = named[EDITABLE_POLICY_STEP]
    require(field(editable, 'if') == FULL and field(editable, 'timeout-minutes') == '12',
            'editable policy must stay full-only with a bounded timeout')
    require(literal_commands(editable) == [EDITABLE_POLICY_COMMAND, EDITABLE_POLICY_HELPER_COMMAND],
            'editable policy must retain default839 then separately inspect036 and replay current sources')
    require(editable.count(EDITABLE_POLICY_SOURCE_SETUP) == 1,
            'source-inspection036 must use the exact official archive, SHA, sole member and separate tools path')
    default_editable, helper_editable = editable.split(EDITABLE_POLICY_SOURCE_SETUP)
    for token in EDITABLE_POLICY_CLAUSES:
        require(default_editable.count(token) == 1, 'default editable policy gate missing: ' + token)
    for token in EDITABLE_POLICY_HELPER_CLAUSES:
        require(helper_editable.count(token) == 1, 'helper editable policy gate missing: ' + token)
    require('set -euo pipefail' in editable and '|| true' not in editable,
            'editable policy must preserve the first failure')

    autobot = named[AUTOBOT_STEP]
    require(field(autobot, 'if') == FULL and field(autobot, 'timeout-minutes') == '6',
            'AutoBot adaptation must stay full-only with a bounded timeout')
    require(literal_commands(autobot) == [AUTOBOT_CONTROLS, AUTOBOT_COMMAND] and
            autobot.count(AUTOBOT_BUILD) == autobot.count(AUTOBOT_CONTROLS) == 1,
            'AutoBot must use current pure controls, actual converter, compiler and explicit Go')
    for token in AUTOBOT_CLAUSES:
        require(autobot.count(token) == 1, 'AutoBot original/native/replay gate missing: ' + token)
    require('set -euo pipefail' in autobot and '|| true' not in autobot,
            'AutoBot must preserve the first failure')

    for name in (POLICY_PILOT_UPLOAD, POLICY_UPLOAD):
        require(field(named[name], 'if') == 'always() && ' + FULL,
                'policy evidence must upload on failure but never on docs-only: ' + name)
    require(field(named[POLICY_DOC_REPORT], 'if') == DOCS and
            field(named[POLICY_DOC_ARTIFACT], 'if') == 'always() && ' + DOCS,
            'policy docs-only scope must be explicit and separate')
    require('name: gooo-go-ports-policy-evidence\n' in named[POLICY_UPLOAD] and
            'path: |\n            out/ci-scope.json\n            out/compiler-build.json\n            out/cel-api-policy/\n            out/api-policy-revision/\n            out/agentmesh-service-policy/\n            out/etag-project/\n            out/etag-project-evidence/\n            out/api-policy-public-consumer/\n            out/api-policy-original-download.json\n            out/url-components/\n            out/path-facts/\n            out/redelivery-policy/\n            out/ipv4/\n            out/versioned-grant-change/\n            out/editable-api-policy/\n            out/editable-api-policy-helpers/\n            out/autobot-resolver/\n' in named[POLICY_UPLOAD] and
            'if-no-files-found: warn\n' in named[POLICY_UPLOAD] and 'retention-days: 14\n' in named[POLICY_UPLOAD],
            'policy output artifacts must include current identity, original policy outputs and both exported ETag directories')
    url_artifact_line = '            out/url-components/\n'
    require(named[POLICY_UPLOAD].count(url_artifact_line) == 1 and
            hashlib.sha256(named[POLICY_UPLOAD].replace(url_artifact_line, '', 1).encode()).hexdigest() == POLICY_PRIOR_UPLOAD_SHA,
            'URL artifact may only add one path to the complete prior failure-safe policy upload')
    require('name: gooo-go-ports-policy-documentation-scope\n' in named[POLICY_DOC_ARTIFACT] and
            'path: out/ci-scope.json\n' in named[POLICY_DOC_ARTIFACT] and
            'if-no-files-found: error\n' in named[POLICY_DOC_ARTIFACT] and 'retention-days: 14\n' in named[POLICY_DOC_ARTIFACT],
            'policy docs artifact must contain only its own classifier scope')
    return named, unnamed


def deployment_expression(text):
    deploy = workflow_jobs(text)['deploy-browser-demo']
    conditions = re.findall(r"^    if: (.+)$", deploy, re.M)
    dependencies = re.findall(r"^    needs: (.+)$", deploy, re.M)
    require(conditions == [DEPLOY_IF], "deployment must explicitly require all four successes on uncanceled main pushes")
    require(dependencies == [DEPLOY_NEEDS.strip().removeprefix('needs: ')],
            "deployment must directly need each producer exactly once")
    require('continue-on-error:' not in deploy, "deployment failure became optional")
    return conditions[0]


def deployment_condition_shell(expression):
    # A limited source-derived conjunction model, not GitHub's expression
    # evaluator. The public workflow must still be validated by actual CI.
    match = re.fullmatch(r"\$\{\{ (.+) \}\}", expression)
    require(match is not None, "explicit deployment expression required")
    translations = {
        'always()': 'true',
        '!cancelled()': 'test "$WORKFLOW_CANCELLED" = false',
        "github.event_name == 'push'": 'test "$EVENT_NAME" = push',
        "github.ref == 'refs/heads/main'": 'test "$GITHUB_REF" = refs/heads/main',
        "needs.observer-checks.result == 'success'": 'test "$OBSERVER_RESULT" = success',
        "needs.native-execution.result == 'success'": 'test "$NATIVE_RESULT" = success',
        "needs.policy-native-checks.result == 'success'": 'test "$POLICY_RESULT" = success',
        "needs.browser-demo.result == 'success'": 'test "$BROWSER_RESULT" = success',
    }
    clauses = match[1].split(' && ')
    require(clauses == list(translations), "unknown/repeated/reordered deployment conjunction")
    return ' &&\n'.join(translations[clause] for clause in clauses)


def validate(text):
    header = text[:text.index("jobs:\n")]
    jobs = workflow_jobs(text)
    # Preserve the browser producer byte-for-byte, separately from the changed
    # deploy prefix. The old deployment runner/permissions/body is also exact.
    require(hashlib.sha256(header.encode()).hexdigest() == HEADER_SHA, "events or permissions changed")
    require(hashlib.sha256(jobs['browser-demo'].encode()).hexdigest() == BROWSER_JOB_SHA,
            "browser producer behavior changed")
    deployment_expression(text)
    deploy = jobs['deploy-browser-demo']
    prefix = '  deploy-browser-demo:\n    if: ' + DEPLOY_IF + '\n' + DEPLOY_NEEDS + '\n'
    require(deploy.startswith(prefix) and
            hashlib.sha256(deploy[len(prefix):].encode()).hexdigest() == DEPLOY_BODY_SHA,
            "deployment may change only its reviewed direct dependency/status prefix")
    named, unnamed = named_steps(text)
    require(len(HEAVY) == 37, "full-scope native step count changed")
    # These72 are declarations in the source, not proof of hosted job outcomes.
    require(len(re.findall(r'^      - (?:name|uses):', text, re.M)) == 72,
            'reviewed workflow source step count changed')
    require(set(named) == COMMON | HEAVY | PREFLIGHT | UPLOADS | DOC_STEPS | OBSERVER_UPLOADS,
            "step roster changed without guard review")
    require(len(unnamed) == 2 and all(field(c, "if") is None for c in unnamed), "checkout/Go setup became conditional")
    require("fetch-depth: 0" in unnamed[0], "exact PR parents must be available")
    require("go-version: '1.27.2'" in unnamed[1], "Go toolchain pin changed")
    public = named["Verify the public API policy consumer"]
    native_job = workflow_jobs(text)['native-execution']
    policy_job = workflow_jobs(text)['policy-native-checks']
    policy_named, policy_unnamed = policy_steps(policy_job)
    prior_workflow = text.replace(policy_named[URL_COMPONENT_STEP], '', 1).replace(
        '          ' + URL_COMPONENT_COMMON + '\n', '', 1).replace(
        '            out/url-components/\n', '', 1)
    for source in URL_COMPONENT_DECLARATIONS:
        prior_workflow = prior_workflow.replace('          ./.gooo check ' + source + '\n', '', 1)
    require(hashlib.sha256(prior_workflow.encode()).hexdigest() == URL_COMPONENT_PRIOR_WORKFLOW_SHA,
            'URL integration may only add its native step, common unit line, two declarations and artifact path to the prior workflow bytes')
    preflight = named[PREFLIGHT_STEP_NAME]
    require(hashlib.sha256(preflight.encode()).hexdigest() == PREFLIGHT_STEP_SHA,
            "lightweight summary controls, source buffers, limits or first-failure evidence changed")
    require(native_job.index(named['Classify the exact checked PR tree']) < native_job.index(preflight) <
            native_job.index(unnamed[1]), "summary preflight must precede Go setup and all native work")
    require(policy_job.index(policy_unnamed[1]) < policy_job.index(public) <
            policy_job.index(policy_named["Build and identify the pinned compiler from a clean checkout"]),
            "public consumer must run after its policy Go setup and before its policy compiler/execution")
    require("python3 -B tools/check-api-policy-public-consumer_test.py -v" in named["Check the Go observer"],
            "bounded public-consumer diagnostic checks must remain in the common Go observer step")
    for name in COMMON:
        require(field(named[name], "if") is None, "common check became conditional: " + name)
    for name in HEAVY | PREFLIGHT:
        require(field(named[name], "if") == FULL, "missing full-execution guard: " + name)
    for name in UPLOADS:
        require(field(named[name], "if") == "always() && " + FULL, "native artifact can upload on docs path: " + name)
    require(field(named[OBSERVER_UPLOAD_NAME], "if") == "always()",
            "observer test diagnostics must survive a failed common check")
    comparison = named["Compare the ports with Go and replay every batch"]
    require(literal_commands(comparison) == [NATIVE_EXPECTED_COMMAND, NATIVE_COMPARISON_COMMAND]
            and ".native_suite_workers == 3 and" in comparison,
            "full native CI must bind three workers and source-bound timing")
    require(text.count(NATIVE_EXPECTED_COMMAND) == comparison.count(NATIVE_EXPECTED_COMMAND) == 1
            and comparison.startswith("      - name: Compare the ports with Go and replay every batch\n"
                                      "        if: " + FULL + "\n        run: |\n          "
                                      + NATIVE_EXPECTED_COMMAND + "\n"),
            "native expected source export must run exactly once first, with root-only input and fixed stdout")
    require(comparison.index(NATIVE_EXPECTED_COMMAND) < comparison.index(HEADER_MODULE_PREFLIGHT)
            < comparison.index(NATIVE_COMPARISON_COMMAND),
            "source expectations must precede module queries and the native verifier")
    require(comparison.count(NATIVE_EXPECTED_SLURP) == 4
            and comparison.count(NATIVE_EXPECTED_BINDING) == 4
            and comparison.count("          jq -e " + NATIVE_EXPECTED_SLURP + " '\n"
                                 "            " + NATIVE_EXPECTED_BINDING + "\n") == 1,
            "all four native gates must import the same source-only expected file")
    require(comparison.count(NATIVE_ALL_POPULATION_ASSERTION) == 1,
            "every native suite identity/case/batch/replay must match its ordered expected row")
    observer = named["Check the Go observer"]
    require(literal_commands(observer).count(URL_COMPONENT_COMMON) == 1 and
            text.count(URL_COMPONENT_COMMON) == 1,
            'URL component pure controls must run once unconditionally as common checks')
    require(literal_commands(observer).count(NATIVE_EXPECTED_UNIT) == 1
            and text.count(NATIVE_EXPECTED_UNIT) == 1,
            "native expected-reader controls must run once as an unconditional common check")
    require(comparison.count(CASE_PROFILE_ASSERTION) == 1,
            "native CI must require the exact explicit UUID compact profile identity")
    require(comparison.count(WORKSPACE_PROFILE_ASSERTION) == 1,
            "native CI must require the exact explicit dependency closure workspace profile identity")
    require(comparison.count(SUITE_ORDER_SUMMARY_ASSERTION) == 1,
            "native CI must bind descending case-count dispatch with original-index ties and catalog results")
    require(comparison.count(TIMING_HEADER_ASSERTION) == 1,
            "native timing must bind the same profile and complete source identity")
    require(comparison.count(TIMING_ASSERTION) == 1,
            "native timing must require the exact complete footer and full observed roster")
    summaries = re.findall(r"^          jq -c -s '\n.*?^          ' out/ci/timing.jsonl\n", comparison, re.M | re.S)
    require(len(summaries) == 1 and hashlib.sha256(summaries[0].encode()).hexdigest() == TIMING_SUMMARY_SHA,
            "native timing console summary must retain its exact source and actual input")
    require("continue-on-error:" not in comparison and "|| true" not in comparison,
            "native comparison or timing failure became optional")
    for token in (".ported_functions == 54 and .consumer_programs == 26 and .consumer_activities == 28 and",
                  "(.runs | length) == $e.totals.suites and",
                  ".cases == $e.totals.cases and .passed == $e.totals.cases and",
                  "([.runs[].cases] | add) == $e.totals.cases and ([.runs[].batches] | add) == $e.totals.batches and",
                  "([.runs[:62][].cases] | add) == 159781 and ([.runs[:62][].batches] | add) == 2058 and",
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
                                               (61, "app/literal-text-report", "Report", 235)):
        for token in (f'.runs[{index}].package == "{package}" and .runs[{index}].activity == "{activity}" and',
                      f".runs[{index}].cases == 9313 and .runs[{index}].passed == 9313 and .runs[{index}].batches == {batches} and"):
            require(token in comparison, "literal finite suite identity/count missing: " + token)
    require(".literal_report_compact_scope == " + json.dumps(LITERAL_REPORT_COMPACT_SCOPE) + " and" in comparison,
            "literal Report compact scope is missing or rebound")
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
    for token in ('([.runs[:66][].cases] | add) == 159909 and ([.runs[:66][].batches] | add) == 2063 and', '.decimal_reference_cases == 256 and .payload_admission_reference_cases == 101 and', '(.decimal_payload_provenance_scope | contains("Inner prior_result_sha256 is NOT_CHECKED")) and', '.runs[66].package == "go/strconv" and .runs[66].activity == "ParseIntDecimal64" and', '.runs[66].cases == 256 and .runs[66].passed == 256 and .runs[66].batches == 5 and', '.runs[67].package == "app/payload-admission" and .runs[67].activity == "Report" and', '.runs[67].cases == 101 and .runs[67].passed == 101 and .runs[67].batches == 4 and'):
        require(token in comparison, "decimal/payload finite roster or evidence scope missing: " + token)
    require(named["Check the Go observer"].count("python3 -I -B tools/generate-payload-corpus.py --check") == 1, "payload input generator missing")
    for source in ("ports/strconv/decimal.gooo", "examples/payload-admission/main.gooo"):
        require(named["Check Gooo declarations"].count("./.gooo check " + source) == 1, "decimal/payload declaration missing")
    require(comparison.count(HEADER_MODULE_PREFLIGHT) == 1 and
            comparison.index(HEADER_MODULE_PREFLIGHT) < comparison.index(NATIVE_COMPARISON_COMMAND),
            "native header dependencies must authenticate once before readonly verifier build")
    require(comparison.count(HEADER_SOURCE_SET_ASSERTION) == 1,
            "native timing must bind all82 exact source maps and summary suite identities")
    for token in LAST_INDEX_CLAUSES:
        require(comparison.count(token) == 1, "LastIndex exact appended source/count/scope/old80 prefix gate missing: " + token)
    for token in IF_MATCH_CLAUSES:
        require(comparison.count(token) == 1, "If-Match exact source/count/enum/prefix gate missing: " + token)
    for source in IF_MATCH_DECLARATIONS:
        require(named["Check Gooo declarations"].count("./.gooo check " + source) == 1, "If-Match declaration missing or duplicated")
    for token in ETAG_CLAUSES:
        require(comparison.count(token) == 1, "ETag exact appended source/field/count/scope gate missing: " + token)
    for source in ETAG_DECLARATIONS:
        require(named["Check Gooo declarations"].count("./.gooo check " + source) == 1, "ETag declaration missing or duplicated")
    for token in CANONICAL_CLAUSES:
        require(comparison.count(token) == 1, "canonical exact appended source/field/count/scope gate missing: " + token)
    for source in CANONICAL_DECLARATIONS:
        require(named["Check Gooo declarations"].count("./.gooo check " + source) == 1, "canonical declaration missing or duplicated")
    for token in HEADER_ADDITION_CLAUSES:
        require(comparison.count(token) == 1, "addition exact74th source/typed ABI/field/case/batch gate missing: " + token)
    require(named["Check Gooo declarations"].count("./.gooo check examples/diagnostic-header-addition/main.gooo") == 1, "addition declaration missing or duplicated")
    for token in HEADER_CLAUSES:
        require(comparison.count(token) == 1, "header exact source, field scope or appended roster missing: " + token)
    for source in HEADER_DECLARATIONS:
        require(named["Check Gooo declarations"].count("./.gooo check " + source) == 1, "header declaration missing or duplicated")
    for token in PATH_ALIAS_CLAUSES:
        require(comparison.count(token) == 1, "path Clean/alias exact source, scope or appended roster missing: " + token)
    for source in PATH_ALIAS_DECLARATIONS:
        require(named["Check Gooo declarations"].count("./.gooo check " + source) == 1, "path Clean/alias declaration missing or duplicated")
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
    wire = named[HEADER_WIRE_STEP_NAME]
    require(hashlib.sha256(wire.encode()).hexdigest() == HEADER_WIRE_STEP_SHA
            and literal_commands(wire) == [HEADER_WIRE_COMMAND]
            and text.count(HEADER_WIRE_COMMAND) == 1,
            "header wire stage must retain the exact readonly helper build, actual CLI argv and acceptance counts once")
    require(native_job.index(named["Check Gooo declarations"]) < native_job.index(wire) <
            native_job.index(comparison),
            "actual header wire route must run after declarations and before the full native comparison")
    if_match_input = named[IF_MATCH_STEP_NAME]
    require(hashlib.sha256(if_match_input.encode()).hexdigest() == IF_MATCH_STEP_SHA
            and literal_commands(if_match_input) == [IF_MATCH_UNIT, IF_MATCH_COMMAND]
            and text.count(IF_MATCH_COMMAND) == 1 and text.count(IF_MATCH_UNIT) == 1
            and field(if_match_input, "timeout-minutes") == "8",
            "If-Match strict route must retain exact readonly helper/actual CLI/count/source/closure gates once")
    for token in IF_MATCH_ROUTE_CLAUSES:
        require(if_match_input.count(token) == 1, "If-Match input-only route gate missing: " + token)
    require(native_job.index(comparison) < native_job.index(if_match_input) < native_job.index(named[HEADER_ADDITION_INPUT_STEP_NAME]),
            "If-Match strict route requires the fresh normal differential before addition/host examples")
    addition_input, addition_host = named[HEADER_ADDITION_INPUT_STEP_NAME], named[HEADER_ADDITION_HOST_STEP_NAME]
    require(hashlib.sha256(addition_input.encode()).hexdigest() == HEADER_ADDITION_INPUT_STEP_SHA
            and literal_commands(addition_input) == [HEADER_ADDITION_INPUT_COMMAND]
            and text.count(HEADER_ADDITION_INPUT_COMMAND) == 1, "addition input route must retain exact helper build/fresh timing/argv/128 typed-field acceptance once")
    require(hashlib.sha256(addition_host.encode()).hexdigest() == HEADER_ADDITION_HOST_STEP_SHA
            and literal_commands(addition_host) == [HEADER_ADDITION_HOST_COMMAND]
            and text.count(HEADER_ADDITION_HOST_COMMAND) == 1
            and named["Check the Go observer"].count(HEADER_ADDITION_HOST_UNIT) == text.count(HEADER_ADDITION_HOST_UNIT) == 1, "addition host must retain exact fresh82 receipt/timing/ABI50/test/atomic acceptance once")
    require(native_job.index(comparison) < native_job.index(addition_input) < native_job.index(addition_host) < native_job.index(named["Execute and replay the Unicode case inspector"]), "addition routes/host require the fresh normal differential before all later examples")
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
    require(policy_job.index(policy_named["Build and identify the pinned compiler from a clean checkout"]) < policy_job.index(redelivery)
            < policy_job.index(policy_named["Verify the offline decision-policy experiment"]), "redelivery requires its own pinned policy compiler before existing policy work")
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
    require(policy_job.index(policy_named["Build and identify the pinned compiler from a clean checkout"]) < policy_job.index(ipv4)
            < policy_job.index(policy_named["Verify the offline decision-policy experiment"]), "IPv4 requires its own pinned policy compiler before existing policy work")
    for command in IPV4_COMMON_COMMANDS:
        require(named["Check the Go observer"].count(command) == text.count(command) == 1,
                "IPv4 Python checks must run once as common checks")
    for source in IPV4_DECLARATIONS:
        require(named["Check Gooo declarations"].count("./.gooo check " + source) == 1,
                "IPv4 declaration check missing: " + source)
    for name, expected in LITERAL_COMMANDS.items():
        commands = literal_commands(named[name])
        require(len(commands) == len(expected) and set(commands) == expected,
                "literal workflow command allow-list changed: " + name)
        require("continue-on-error:" not in named[name] and "|| true" not in named[name],
                "literal gate failure became optional")
    last_index_inputs = named[LAST_INDEX_INPUT_STEP_NAME]
    require(hashlib.sha256(last_index_inputs.encode()).hexdigest() == LAST_INDEX_INPUT_STEP_SHA,
            "LastIndex malformed gate must preserve exact56 fresh CLI refusals and source/input bindings")
    require(field(last_index_inputs, "timeout-minutes") == "8" and
            literal_commands(named["Check the Go observer"]).count(LAST_INDEX_INPUT_UNIT) == 1,
            "LastIndex malformed controls must run once unconditionally with a bounded native timeout")
    require(native_job.index(named["Observe malformed literal Text inputs through the actual CLI"]) <
            native_job.index(last_index_inputs) <
            native_job.index(named["Observe malformed FitsCrop inputs through the actual CLI"]),
            "LastIndex malformed observations must remain separate after the unchanged literal Text gate")
    require(native_job.count(named["Observe malformed literal Text inputs through the actual CLI"] +
                             last_index_inputs + named["Observe malformed FitsCrop inputs through the actual CLI"]) == 1,
            "LastIndex malformed stage must be inserted directly between the original two input-refusal stages")
    literal = named["Execute and replay the literal text report"]
    for token in ('.decision == "OBSERVED"', ".result.runtime.finite_total == 0",
                  '.schema == "gooo/literal-text-report-checked/v1"',
                  '.decision == "PASS_FINITE_GO_ORACLE_AND_SAVED_REPLAY"',
                  ".native_observed_cases == 24", ".independent_go_oracle_cases == 24",
                  ".saved_replay_observed_cases == 24", ".native_receipt_expectation_cases == 0",
                  ".prior_result_sha256", ".execution_sha256", ".replay_sha256", ".source_bindings"):
        require(token in literal, "strong input-only Report evidence gate missing: " + token)
    malformed = named["Observe malformed literal Text inputs through the actual CLI"]
    for token in IF_MATCH_CLI_CLAUSES:
        require(malformed.count(token) == 1, "If-Match separate CLI shape/old196 prefix gate missing: " + token)
    for token in ('.schema == "gooo/literal-text-input-rejections/v1"', ".planned_framework_inputs == 224",
                  ".observed_framework_rejections == 224", ".go_reference_cases == 0",
                  '"go/strings:Count"', '"go/strings:Index"', '"app/literal-text-report:Report"',
                  '"go/strings:Cut"', '"go/strings:CutPrefix"', '"go/strings:CutSuffix"', '"app/literal-cut-report:Report"',
                  "(.cases | length) == 224",
                  '"go/net/http/precondition:IfMatchCondition"',
                  'all(.cases[:196][]; .entry != "go/net/http/precondition:IfMatchCondition") and',
                  'all(.cases[196:][]; .entry == "go/net/http/precondition:IfMatchCondition") and',
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
    for command in ("gofmt -l", "go vet ./...", "go test -race -timeout=15m -json ./...",
                    "python3 tools/bootstrap.test.py", "python3 tools/receipt_json_test.py",
                    "python3 tools/check-drag-crop-example_test.py",
                    "python3 tools/check-fixed-pixel-bounds_test.py",
                    "python3 -B tools/check-alpha-matte_test.py -v",
                    "python3 -B tools/check-fits-crop_test.py -v",
                    "python3 -B tools/check-fits-crop-inputs_test.py -v",
                    "python3 -B tools/check-literal-text-inputs_test.py -v",
                    LAST_INDEX_INPUT_UNIT,
                    "python3 -B tools/classify-ci-scope_test.py -v",
                    "python3 -B tools/check-ci-scope-workflow_test.py -v"):
        require(command in observer, "common validation missing: " + command)
    for command in ("go run ./cmd/gen-unicode --check", "go run ./cmd/gen-simple-fold --check",
                    "go run ./cmd/gen-case-mapping --check", "python3 tools/gen-uuid-quote.py --check"):
        require(command in named["Check pinned Unicode table generation"], "generator check missing")
    require(literal_commands(observer).count(EDITABLE_POLICY_COMMON) == 1,
            'editable policy pure controls must run once unconditionally')
    for source in EDITABLE_POLICY_DECLARATIONS:
        require(literal_commands(named['Check Gooo declarations']).count('./.gooo check ' + source) == 1,
                'editable policy declarations must be unconditional')
    require(literal_commands(observer).count(VERSIONED_GRANT_COMMON_COMMAND) == 1,
            'versioned grant pure controls must run once unconditionally as common checks')
    require(literal_commands(named['Check Gooo declarations']).count('./.gooo check ' + VERSIONED_GRANT_DECLARATION) == 1,
            'versioned grant declaration must run once in the unconditional declaration step')
    identity = named["Build and identify the pinned compiler from a clean checkout"]
    for token in ("sh tools/bootstrap.sh", "version --build --json", '"0.6.32-dev"',
                  '"83928fa4538245abe33b08549da47f8cbdc50ed2"', '"CLEAN_VCS"', '"go1.27.2"'):
        require(token in identity, "compiler identity check changed")
    declarations = named["Check Gooo declarations"]
    prior_declarations = declarations
    require(hashlib.sha256(declarations.encode()).hexdigest() == URL_COMPONENT_DECLARATIONS_SHA,
            'URL declarations must retain their exact reviewed common step')
    for source in URL_COMPONENT_DECLARATIONS:
        line = '          ./.gooo check ' + source + '\n'
        require(declarations.count(line) == 1,
                'URL declaration must run once unconditionally: ' + source)
        prior_declarations = prior_declarations.replace(line, '', 1)
    require(hashlib.sha256(prior_declarations.encode()).hexdigest() == URL_COMPONENT_PRIOR_DECLARATIONS_SHA,
            'URL declarations may only add two commands to all prior declaration bytes')
    require("./.gooo check ports/image/geometry.gooo" in declarations
            and "./.gooo check examples/fits-crop/main.gooo" in declarations
            and "tools/probe.sh" not in declarations, "declaration/probe separation")
    for source in ("ports/strings/search.gooo", "ports/strings/literal.gooo", "examples/literal-text-report/main.gooo", "ports/strings/cut.gooo", "examples/literal-cut-report/main.gooo", "ports/strings/last_index.gooo", "examples/literal-final-marker-report/main.gooo"):
        require(literal_commands(declarations).count("./.gooo check " + source) == 1,
                "literal declaration check missing or duplicated: " + source)
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
    require("Allowlisted static-document content-only PR" in log
            and "No new native-port execution/replay receipts" in log
            and "browser-demo job retains its separate" in log, "misleading execution scope log")


class WorkflowScopeTests(unittest.TestCase):
    def test_url_component_source_scope_order_and_original_workflow_bytes_are_exact(self):
        validate(self.text)
        jobs = workflow_jobs(self.text)
        policy, _ = policy_steps(jobs['policy-native-checks'])
        step = policy[URL_COMPONENT_STEP]
        identity = policy['Build and identify the pinned compiler from a clean checkout']
        path = policy['Verify the isolated native path facts sidecar']
        self.assertIn(identity + step + path, jobs['policy-native-checks'])
        # A source roster is not a claim that GitHub has executed these72 steps.
        self.assertEqual(len(re.findall(r'^      - (?:name|uses):', self.text, re.M)), 72)
        self.assertEqual(len(HEAVY), 37)
        prior = self.text.replace(step, '', 1).replace(
            '          ' + URL_COMPONENT_COMMON + '\n', '', 1).replace(
            '            out/url-components/\n', '', 1)
        for source in URL_COMPONENT_DECLARATIONS:
            prior = prior.replace('          ./.gooo check ' + source + '\n', '', 1)
        self.assertEqual(hashlib.sha256(prior.encode()).hexdigest(), URL_COMPONENT_PRIOR_WORKFLOW_SHA)
        for replacement in ('', step + step,
                            step.replace(FULL, DOCS, 1),
                            step.replace(FULL, "steps.change-scope.outputs.docs_only == 'false'", 1),
                            step.replace('        if: ' + FULL + '\n', '', 1),
                            step.replace('timeout-minutes: 5', 'timeout-minutes: 40', 1),
                            step.replace('        run: |', '        continue-on-error: true\n        run: |', 1)):
            with self.subTest(step_mutation=replacement[:100]), self.assertRaises(ValueError):
                validate(self.text.replace(step, replacement, 1))
        without = self.text.replace(step, '', 1)
        _, native, _ = job_steps(jobs['native-execution'])
        for anchor in (identity, policy['Verify the original automatic redelivery policy'],
                       native['Compare the ports with Go and replay every batch']):
            with self.subTest(wrong_owner_or_order=anchor[:80]), self.assertRaises(ValueError):
                validate(without.replace(anchor, step + anchor, 1))

    def test_url_component_current_command_and_every_finite_summary_gate_are_required(self):
        policy, _ = policy_steps(workflow_jobs(self.text)['policy-native-checks'])
        step = policy[URL_COMPONENT_STEP]
        self.assertTrue(NATIVE_COMMAND.search(URL_COMPONENT_COMMAND))
        self.assertEqual(literal_commands(step), [URL_COMPONENT_COMMAND])
        for token in (URL_COMPONENT_COMMAND,) + URL_COMPONENT_CLAUSES:
            self.assertEqual(step.count(token), 1)
            with self.subTest(missing_command_or_gate=token), self.assertRaises(ValueError):
                validate(self.text.replace(step, step.replace(token, '', 1), 1))
        for before, after in (('--root "$PWD"', '--root out/old-snapshot'),
                              ('--compiler "$PWD/.gooo"', '--compiler /tmp/unbound'),
                              ('--go "$(go env GOROOT)/bin/go"', '--go go'),
                              ('--out "$PWD/out/url-components"', '--out out/old-receipts'),
                              ('.input_rows == 128', '.input_rows == 18'),
                              ('.unique_inputs == 128', '.unique_inputs == 1'),
                              ('.typed_fields_per_layer == 1280', '.typed_fields_per_layer == 128'),
                              ('.native_runs_per_layer == 2', '.native_runs_per_layer == 1'),
                              ('.finite_total == 0', '.finite_total > 0'),
                              ('.model_calls == 0', '.model_calls >= 0'),
                              ('.catalog_extended == false', '.catalog_extended == true'),
                              ('(.negative_controls | length) == 27', '(.negative_controls | length) > 0'),
                              ('(.negative_controls | unique | length) == 27', '(.negative_controls | unique | length) == 1'),
                              ('.processes_closed == true', '.processes_closed == false'),
                              ('.owned_work_removed == true', '.owned_work_removed == false'),
                              ('set -euo pipefail', 'set +e'),
                              (URL_COMPONENT_COMMAND, URL_COMPONENT_COMMAND + ' || true')):
            self.assertIn(before, step)
            with self.subTest(weakened_gate=before), self.assertRaises(ValueError):
                validate(self.text.replace(step, step.replace(before, after, 1), 1))

    def test_url_component_common_controls_are_unconditional_and_not_native_execution(self):
        observer = named_steps(self.text)[0]['Check the Go observer']
        self.assertFalse(NATIVE_COMMAND.search(URL_COMPONENT_COMMON))
        line = '          ' + URL_COMPONENT_COMMON + '\n'
        self.assertEqual(observer.count(line), 1)
        self.assertEqual(hashlib.sha256(observer.replace(line, '', 1).encode()).hexdigest(),
                         OBSERVER_PRIOR_STEP_SHA)
        for replacement in ('', line + line,
                            '          if false; then\n  ' + line + '          fi\n',
                            '          ' + URL_COMPONENT_COMMAND + '\n'):
            with self.subTest(common_mutation=replacement[:100]), self.assertRaises(ValueError):
                validate(self.text.replace(observer, observer.replace(line, replacement, 1), 1))

    def test_url_component_artifact_is_complete_failure_safe_and_separate_from_docs(self):
        policy, _ = policy_steps(workflow_jobs(self.text)['policy-native-checks'])
        upload, docs = policy[POLICY_UPLOAD], policy[POLICY_DOC_ARTIFACT]
        line = '            out/url-components/\n'
        self.assertEqual(upload.count(line), 1)
        self.assertEqual(hashlib.sha256(upload.replace(line, '', 1).encode()).hexdigest(),
                         POLICY_PRIOR_UPLOAD_SHA)
        for replacement in ('', line + line):
            with self.subTest(artifact_path=replacement), self.assertRaises(ValueError):
                validate(self.text.replace(upload, upload.replace(line, replacement, 1), 1))
        with self.assertRaises(ValueError):
            validate(self.text.replace(docs, docs.replace('path: out/ci-scope.json', 'path: out/url-components/', 1), 1))
        for condition in ('', FULL, DOCS, 'always()'):
            changed = upload.replace('        if: always() && ' + FULL + '\n',
                                     '        if: ' + condition + '\n' if condition else '', 1)
            with self.subTest(upload_condition=condition), self.assertRaises(ValueError):
                validate(self.text.replace(upload, changed, 1))

    def test_url_component_declarations_are_unconditional_unique_and_preserve_prior_bytes(self):
        declarations = named_steps(self.text)[0]['Check Gooo declarations']
        self.assertIsNone(field(declarations, 'if'))
        prior = declarations
        for source in URL_COMPONENT_DECLARATIONS:
            command = './.gooo check ' + source
            line = '          ' + command + '\n'
            self.assertFalse(NATIVE_COMMAND.search(command))
            self.assertEqual(literal_commands(declarations).count(command), 1)
            prior = prior.replace(line, '', 1)
            for replacement in ('', line + line,
                                '          if false; then\n  ' + line + '          fi\n'):
                with self.subTest(declaration=source, mutation=replacement[:100]), self.assertRaises(ValueError):
                    validate(self.text.replace(declarations, declarations.replace(line, replacement, 1), 1))
        self.assertEqual(hashlib.sha256(prior.encode()).hexdigest(), URL_COMPONENT_PRIOR_DECLARATIONS_SHA)
        for condition in (FULL, DOCS, "steps.change-scope.outputs.docs_only == 'false'"):
            changed = declarations.replace('        run: |', '        if: ' + condition + '\n        run: |', 1)
            with self.subTest(declarations_condition=condition), self.assertRaises(ValueError):
                validate(self.text.replace(declarations, changed, 1))

    def test_tutorial_change_requires_same_job_original_and_exact_saved_replay_summary(self):
        browser = workflow_jobs(self.text)["browser-demo"]
        _, named, _ = job_steps(browser)
        comparison = named["Verify the edited tutorial and saved replay summary"]
        artifact = named["Preserve tutorial edit comparison evidence"]
        command = "python3 tools/verify-tutorial-change.py --out out/tutorial-change"
        original = "--before out/browser/site/tutorial/execution.json"
        self.assertEqual(comparison.count(command), 1)
        self.assertEqual(comparison.count(original), 1)
        self.assertLess(browser.index(named["Build the actual Gooo composition for the browser"]),
                        browser.index(comparison))
        self.assertLess(browser.index(comparison), browser.index(artifact))
        self.assertEqual(field(artifact, "if"), "always()")
        clauses = (
            '.schema == "gooo/tutorial-change-run/v1"',
            '.status == "TUTORIAL_CHANGE_AND_REPLAY_CHECKED"',
            '.new_Gooo_commands == 2 and .model_calls == 0',
            '.reused_before != null and (.commands | length) == 6',
            '.go_binary_unchanged == true and .all_owned_process_groups_closed == true',
            '.all_owned_processes_waited == true and .owned_work_removed == true',
            '.finalization_errors == []',
            '.exit == 0 and .process_group_closed == true',
            '.output_complete == true and .stderr.bytes == 0',
            '.schema == "gooo/tutorial-change-summary/v1"',
            '.status == "CHECKED_FINITE_OBSERVATIONS"',
            '.receipt_compiler.source_revision == "83928fa4538245abe33b08549da47f8cbdc50ed2"',
            '.repository_revision == null and .repository_revision_status == "NOT_CAPTURED"',
            '.replay_raw_parent_sha256 == .raw_records.after.sha256',
            '(.sources | length) == 2 and .sources[0].changed == false and .sources[1].changed == true',
            '(.cases | length) == 4 and all(.cases[]; (.fields | length) == 4)',
            '([.cases[].fields[] | select(.changed)] | length) == 1',
            '.cases[2].fields[3].name == "label"',
            '.cases[2].fields[3].before == "sensor: high"',
            '.cases[2].fields[3].after == "sensor: low"',
            '.cases[2].fields[3].replay == "sensor: low"',
        )
        mutations = [
            self.text.replace(comparison, "", 1),
            self.text.replace(artifact, "", 1),
            self.text.replace(comparison, comparison.replace(command, "true # " + command, 1), 1),
            self.text.replace(comparison, comparison.replace(original, "", 1), 1),
        ]
        for wrong in ("out/previous-run/execution.json", "out/browser/site/tutorial/replay.json",
                      "out/browser/site/tutorial/boundary-execution.json"):
            mutations.append(self.text.replace(comparison,
                comparison.replace(original, "--before " + wrong, 1), 1))
        for clause in clauses:
            self.assertEqual(comparison.count(clause), 1)
            mutations.append(self.text.replace(comparison,
                comparison.replace(clause, "true", 1), 1))
        for altered in mutations:
            self.assertNotEqual(altered, self.text)
            with self.subTest(tutorial_change=altered[altered.index("  browser-demo:"):][:400]), self.assertRaises(ValueError):
                validate(altered)

    def test_native_expected_export_is_exact_root_only_once_and_before_any_native_query(self):
        named, _ = named_steps(self.text)
        comparison = named["Compare the ports with Go and replay every batch"]
        line = "          " + NATIVE_EXPECTED_COMMAND + "\n"
        self.assertEqual(comparison.count(line), 1)
        self.assertEqual(literal_commands(comparison), [NATIVE_EXPECTED_COMMAND, NATIVE_COMPARISON_COMMAND])
        mutations = [comparison.replace(line, "", 1), comparison.replace(line, line + line, 1)]
        for replacement in (NATIVE_EXPECTED_COMMAND + " || true", "true # " + NATIVE_EXPECTED_COMMAND,
                            NATIVE_EXPECTED_COMMAND.replace(' --root "$PWD"', ''),
                            NATIVE_EXPECTED_COMMAND.replace(' --root "$PWD"', ' --root "$PWD" --actual out/ci/summary.json'),
                            NATIVE_EXPECTED_COMMAND.replace(' --root "$PWD"', ' --root out/ci'),
                            NATIVE_EXPECTED_COMMAND.replace('out/native-contract-expected.json', 'out/ci/summary.json'),
                            NATIVE_EXPECTED_COMMAND.replace('out/native-contract-expected.json', 'out/other-expected.json')):
            mutations.append(comparison.replace(NATIVE_EXPECTED_COMMAND, replacement, 1))
        for anchor in (HEADER_MODULE_PREFLIGHT, "          " + NATIVE_COMPARISON_COMMAND + "\n"):
            without = comparison.replace(line, "", 1)
            mutations.append(without.replace(anchor, anchor + line, 1))
        mutations.append(comparison.replace(line, "          if false; then\n" + line + "          fi\n", 1))
        for altered in mutations:
            self.assertNotEqual(altered, comparison)
            with self.subTest(export=altered[:1000]), self.assertRaises(ValueError):
                validate(self.text.replace(comparison, altered, 1))

    def test_native_expected_four_imports_and_ordered_all_row_gate_are_mandatory(self):
        named, _ = named_steps(self.text)
        comparison = named["Compare the ports with Go and replay every batch"]
        self.assertEqual(comparison.count(NATIVE_EXPECTED_SLURP), 4)
        self.assertEqual(comparison.count(NATIVE_EXPECTED_BINDING), 4)
        self.assertEqual(comparison.count(NATIVE_ALL_POPULATION_ASSERTION), 1)
        for before in (NATIVE_EXPECTED_SLURP, NATIVE_EXPECTED_BINDING):
            starts = [m.start() for m in re.finditer(re.escape(before), comparison)]
            self.assertEqual(len(starts), 4)
            for at in starts:
                for after in ("", before.replace("out/native-contract-expected.json", "out/ci/summary.json")
                              if before == NATIVE_EXPECTED_SLURP else ". as $e |"):
                    altered = comparison[:at] + after + comparison[at + len(before):]
                    with self.subTest(import_at=at, replacement=after), self.assertRaises(ValueError):
                        validate(self.text.replace(comparison, altered, 1))
        for after in ("", "            true and\n", NATIVE_ALL_POPULATION_ASSERTION.replace("$r.batches == $wanted.batches", "true"),
                      NATIVE_ALL_POPULATION_ASSERTION.replace("$r.package == $wanted.package", "true"),
                      NATIVE_ALL_POPULATION_ASSERTION.replace("$r.replay_equal == true", "true"),
                      NATIVE_ALL_POPULATION_ASSERTION.replace("$actual.runs[$i]", ".runs[$i]")):
            altered = comparison.replace(NATIVE_ALL_POPULATION_ASSERTION, after, 1)
            self.assertNotEqual(altered, comparison)
            with self.subTest(population=after), self.assertRaises(ValueError):
                validate(self.text.replace(comparison, altered, 1))

    def test_native_expected_source_membership_and_common_reader_unit_cannot_be_optional(self):
        named, _ = named_steps(self.text)
        comparison = named["Compare the ports with Go and replay every batch"]
        membership = '              ($s.source_set | keys) == ($e.suites[$i].source_paths | sort) and\n'
        self.assertEqual(comparison.count(membership), 1)
        for after in ("", '              true and\n', membership.replace("$e.suites[$i]", "$e.suites[0]"),
                      membership.replace("==", ">=", 1)):
            altered = comparison.replace(membership, after, 1)
            with self.subTest(membership=after), self.assertRaises(ValueError):
                validate(self.text.replace(comparison, altered, 1))
        observer = named["Check the Go observer"]
        line = "          " + NATIVE_EXPECTED_UNIT + "\n"
        self.assertEqual(observer.count(line), 1)
        for after in ("", line + line, line.rstrip("\n") + " || true\n",
                      '          if false; then\n' + line + '          fi\n'):
            with self.subTest(reader_unit=after), self.assertRaises(ValueError):
                validate(self.text.replace(observer, observer.replace(line, after, 1), 1))

    def setUp(self):
        self.text = WORKFLOW.read_text()

    def test_summary_preflight_is_exact_full_only_and_precedes_go_setup(self):
        named, unnamed = named_steps(self.text)
        chunk = named[PREFLIGHT_STEP_NAME]
        for replacement in ("", chunk + chunk,
                            chunk.replace("        if: " + FULL + "\n", "", 1),
                            chunk.replace("'90s'", "'900s'", 1),
                            chunk.replace("before = snapshot('before')", "before = {}", 1),
                            chunk.replace("after = snapshot('after')", "after = {}", 1),
                            chunk.replace("            break\n", "            pass\n", 1),
                            chunk.replace("raise SystemExit(first_exit)", "raise SystemExit(0)", 1)):
            self.assertNotEqual(replacement, chunk)
            with self.subTest(replacement=replacement[:80]), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, replacement, 1))
        without = self.text.replace(chunk, "", 1)
        for anchor in (unnamed[0], unnamed[1], named["Verify the public API policy consumer"]):
            moved = without.replace(anchor, anchor + chunk, 1)
            with self.subTest(anchor=anchor.splitlines()[0]), self.assertRaises(ValueError):
                validate(moved)

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

    def test_observer_json_scope_and_failure_propagation_cannot_be_weakened(self):
        observer = named_steps(self.text)[0]['Check the Go observer']
        for before, after in (
            ('-race -timeout=15m -json ./...', '-race -timeout=15m ./...'),
            ('-race -timeout=15m -json ./...', '-timeout=15m -json ./...'),
            ('-timeout=15m', '-timeout=10m'),
            ('-json ./...', '-json ./cmd/verify'),
            ('-json ./...', '-json -run TestShort ./...'),
            ('tee out/observer-tests/events.jsonl', 'cat'),
            ('exit "${observer_test_status[0]}"', 'exit 0'),
            ('exit "${observer_test_status[1]}"', 'exit 0'),
        ):
            self.assertIn(before, observer)
            with self.subTest(change=(before, after)), self.assertRaises(ValueError):
                validate(self.text.replace(observer, observer.replace(before, after, 1), 1))

    def test_observer_json_artifact_is_exact_and_failure_safe(self):
        artifact = named_steps(self.text)[0][OBSERVER_UPLOAD_NAME]
        for before, after in (
            ('if: always()', 'if: success()'),
            ('@cf430e030ddbb5b0abf93d22962f4752f3646cd9', '@main'),
            ('out/observer-tests/events.jsonl', 'out/'),
            ('            out/observer-tests/stderr.log\n', ''),
            ('            out/observer-tests/exit-code.txt\n', ''),
            ('retention-days: 7', 'retention-days: 1'),
        ):
            self.assertIn(before, artifact)
            with self.subTest(change=(before, after)), self.assertRaises(ValueError):
                validate(self.text.replace(artifact, artifact.replace(before, after, 1), 1))
        for changed in (self.text.replace(artifact, '', 1),
                        self.text.replace(artifact, artifact + artifact, 1)):
            with self.assertRaises(ValueError):
                validate(changed)

    def test_observer_json_bash_preserves_go_and_diagnostic_failures(self):
        # Run only the actual new Bash block with tiny Go/tee stubs, never Go.
        observer = named_steps(self.text)[0]['Check the Go observer']
        start = observer.index('          mkdir -p out/observer-tests\n')
        end = observer.index('          go -C examples/automatic-redelivery/reference vet', start)
        script = '\n'.join(line[10:] for line in observer[start:end].splitlines())
        go_stub = ('#!/bin/bash\n'
                   'printf "%s\\n" "$*" > go-argv.txt\n'
                   'printf \'{"Action":"pass"}\\n\'\n'
                   'printf "go diagnostic\\n" >&2\n'
                   'exit "$TEST_GO_STATUS"\n')
        tee_stub = ('#!/bin/bash\n/usr/bin/tee "$@" || exit $?\n'
                    'exit "$TEST_TEE_STATUS"\n')
        cases = [(go, tee, fault) for go in (0, 1, 66)
                 for tee, fault in ((0, ''), (7, ''), (0, 'status-write'), (0, 'cat'))]
        for go_status, tee_status, fault in cases:
            with self.subTest(go=go_status, tee=tee_status, diagnostic=fault), tempfile.TemporaryDirectory() as folder:
                root = Path(folder)
                for name, contents in (('go', go_stub), ('tee', tee_stub)):
                    target = root / name
                    target.write_text(contents)
                    target.chmod(0o755)
                prefix = ''
                if fault == 'status-write':
                    (root / 'out/observer-tests/exit-code.txt').mkdir(parents=True)
                elif fault == 'cat':
                    prefix = 'cat() { return 75; }\n'
                environment = {'PATH': folder + ':/usr/bin:/bin',
                               'TEST_GO_STATUS': str(go_status), 'TEST_TEE_STATUS': str(tee_status)}
                result = subprocess.run(['/bin/bash', '--noprofile', '--norc', '-e', '-o', 'pipefail', '-c',
                                         prefix + script + '\nbuiltin printf reached > after-test.txt\n'],
                                        cwd=folder, env=environment, stdin=subprocess.DEVNULL,
                                        stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=5, check=False)
                expected = go_status or (1 if fault else tee_status)
                self.assertEqual(result.returncode, expected)
                self.assertEqual(result.stdout, b'{"Action":"pass"}\n')
                self.assertEqual((root / 'out/observer-tests/events.jsonl').read_bytes(), result.stdout)
                self.assertEqual((root / 'out/observer-tests/stderr.log').read_bytes(), b'go diagnostic\n')
                if fault == 'status-write':
                    self.assertTrue((root / 'out/observer-tests/exit-code.txt').is_dir())
                    self.assertIn(b'exit-code.txt', result.stderr)
                    self.assertTrue(result.stderr.endswith(b'go diagnostic\n'))
                else:
                    self.assertEqual((root / 'out/observer-tests/exit-code.txt').read_text(), str(go_status) + '\n')
                    self.assertEqual(result.stderr, b'' if fault == 'cat' else b'go diagnostic\n')
                self.assertEqual((root / 'after-test.txt').exists(), expected == 0)
                self.assertEqual((root / 'go-argv.txt').read_text(), 'test -race -timeout=15m -json ./...\n')

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
                     (".top_level_stages == 68", ".top_level_stages >= 1"),
                     (" --include-prefix-rule-pair", ""),
                     (".prefix_rule_pair.manual_saved_replay_outputs == 54", ".prefix_rule_pair.manual_saved_replay_outputs == 0"),
                     (".prefix_rule_pair.original_input_values_each_phase == 630", ".prefix_rule_pair.original_input_values_each_phase >= 1"),
                     (".prefix_rule_pair.generated_Report_calls == 112", ".prefix_rule_pair.generated_Report_calls == 18"),
                     (".prefix_rule_pair.generated_typed_fields == 1232", ".prefix_rule_pair.generated_typed_fields >= 1"),
                     (".prefix_rule_pair.wire_refusals == 310", ".prefix_rule_pair.wire_refusals == 0"),
                     (".prefix_rule_pair.tamper_refusals == 18", ".prefix_rule_pair.tamper_refusals == 0"),
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
        for source in IPV4_DECLARATIONS:
            line = "          ./.gooo check " + source + "\n"
            with self.subTest(source=source), self.assertRaises(ValueError):
                validate(self.text.replace(line, "", 1))

    def test_full_native_ci_binds_three_workers_and_timing(self):
        for workers in (1, 2, 4, 5):
            for before, after in (("--workers 3 --case-profile", f"--workers {workers} --case-profile"),
                                  (".native_suite_workers == 3 and", f".native_suite_workers == {workers} and"),
                                  (".configured_workers == 3 and", f".configured_workers == {workers} and")):
                with self.subTest(workers=workers, before=before), self.assertRaises(ValueError):
                    validate(self.text.replace(before, after, 1))

    def test_suite_order_is_explicit_unique_and_cannot_be_changed_or_optional(self):
        named, _ = named_steps(self.text)
        chunk = named["Compare the ports with Go and replay every batch"]
        flag = " --suite-order largest-corpus-first"
        self.assertEqual(chunk.count(flag), 1)
        for replacement in ("", " --suite-order catalog", " --suite-order unknown",
                            " --suite-order smallest-corpus-first", flag + " --suite-order catalog"):
            altered = chunk.replace(flag, replacement, 1)
            self.assertNotEqual(altered, chunk)
            with self.subTest(flag=replacement), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, altered, 1))
        for replacement in (NATIVE_COMPARISON_COMMAND + " || true",
                            "true # " + NATIVE_COMPARISON_COMMAND):
            altered = chunk.replace(NATIVE_COMPARISON_COMMAND, replacement, 1)
            with self.subTest(command=replacement), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, altered, 1))

    def test_suite_order_summary_and_timing_must_bind_the_same_exact_plan(self):
        named, _ = named_steps(self.text)
        chunk = named["Compare the ports with Go and replay every batch"]
        self.assertEqual(chunk.count(SUITE_ORDER_SUMMARY_ASSERTION), 1)
        mutations = [chunk.replace(SUITE_ORDER_SUMMARY_ASSERTION, "", 1)]
        for before, after in (('.suite_order == "largest-corpus-first"', '.suite_order == "catalog"'),
                              ('sort_by([-.value.cases, .key])', 'sort_by([.value.cases, .key])'),
                              ('sort_by([-.value.cases, .key])', 'sort_by([-.value.cases, -.key])'),
                              ('.planned_dispatch_indices == (.runs', '.planned_dispatch_indices != (.runs'),
                              ('.suite_order == $summary[0].suite_order', '.suite_order == "catalog"'),
                              ('.planned_dispatch_indices == $summary[0].planned_dispatch_indices',
                               '.planned_dispatch_indices == null')):
            self.assertIn(before, chunk)
            mutations.append(chunk.replace(before, after, 1))
        for altered in mutations:
            self.assertNotEqual(chunk, altered)
            with self.subTest(altered=altered[:700]), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, altered, 1))

    def test_compact_profile_cannot_be_missing_unknown_legacy_or_rebound(self):
        named, _ = named_steps(self.text)
        chunk = named["Compare the ports with Go and replay every batch"]
        mutations = [chunk.replace(" --case-profile uuid-and-literal-report-compact-v1", replacement, 1)
                     for replacement in ("", " --case-profile legacy", " --case-profile uuid-compact-v1", " --case-profile uuid-and-literal-report-compact-v2",
                                         " --case-profile uuid-and-literal-report-compact-v1 --case-profile legacy")]
        for assertion in (CASE_PROFILE_ASSERTION, TIMING_HEADER_ASSERTION):
            self.assertIn(assertion, chunk)
            mutations.append(chunk.replace(assertion, "", 1))
            for line in assertion.splitlines()[1:-1]:
                mutations.append(chunk.replace(assertion, assertion.replace(line, "            true and", 1), 1))
        mutations.extend((
            chunk.replace('name: "uuid-and-literal-report-compact-v1"', 'name: "legacy"', 1),
            chunk.replace('name: "uuid-and-literal-report-compact-v1"', 'name: "uuid-compact-v1"', 1),
            chunk.replace('basis_profile: "uuid-compact-v1"', 'basis_profile: "legacy"', 1),
            chunk.replace('"app/literal-text-report:Report"', '"go/strings:Index"', 1),
            chunk.replace(".literal_report_compact_scope == " + json.dumps(LITERAL_REPORT_COMPACT_SCOPE) + " and", "true and", 1),
            chunk.replace("maximum_rows: 128", "maximum_rows: 129", 1),
            chunk.replace("raw_bytes_including_LF: 30000", "raw_bytes_including_LF: 32768", 1),
            chunk.replace("(.identity.source_files | length) == $e.totals.source_paths", "(.identity.source_files | length) >= 97", 1),
            chunk.replace("--slurpfile summary out/ci/summary.json", "--slurpfile summary out/old/summary.json", 1),
            chunk.replace(".case_profile == $summary[0].case_profile", ".case_profile == null", 1),
        ))
        for altered in mutations:
            self.assertNotEqual(chunk, altered)
            with self.subTest(altered=altered[:600]), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, altered, 1))

    def test_workspace_dependency_closure_flag_identity_source_map_and_console_are_required(self):
        named, _ = named_steps(self.text)
        chunk = named["Compare the ports with Go and replay every batch"]
        validate(self.text)
        mutations = [chunk.replace(" --workspace-profile dependency-closure-v1", replacement, 1)
                     for replacement in ("", " --workspace-profile legacy", " --workspace-profile unknown",
                                         " --workspace-profile dependency-closure-v1 --workspace-profile legacy")]
        self.assertEqual(chunk.count(WORKSPACE_PROFILE_ASSERTION), 1)
        mutations.append(chunk.replace(WORKSPACE_PROFILE_ASSERTION, "", 1))
        for line in WORKSPACE_PROFILE_ASSERTION.splitlines()[1:-1]:
            mutations.append(chunk.replace(WORKSPACE_PROFILE_ASSERTION,
                                           WORKSPACE_PROFILE_ASSERTION.replace(line, "            true and", 1), 1))
        for replacement in ("            .workspace_profile == null and\n",
                            WORKSPACE_PROFILE_ASSERTION.replace('name: "dependency-closure-v1"', 'name: "legacy"'),
                            WORKSPACE_PROFILE_ASSERTION.replace('basis: "legacy-selected-workspace"', 'basis: "other"')):
            mutations.append(chunk.replace(WORKSPACE_PROFILE_ASSERTION, replacement, 1))
        for before, after in (
                (".workspace_profile == $summary[0].workspace_profile and", "true and"),
                (".workspace_profile == $summary[0].workspace_profile", ".workspace_profile == null"),
                (".workspace_profile == $summary[0].workspace_profile", ".workspace_profile == $summary[0].case_profile"),
                ("(.identity.source_files | length) == $e.totals.source_paths", "(.identity.source_files | length) == 140"),
                ("(.identity.source_files | length) == $e.totals.source_paths", "(.identity.source_files | length) >= 140"),
                ("(.identity.source_files | keys) ==", "(.identity.source_files | values) =="),
                ("all($header.identity.source_files[];", "all($header.identity.source_files | del(.\"cmd/verify/workspace_profile.go\") | .[];"),
                ("(.bytes | type) == \"number\" and .bytes > 0", "(.bytes | type) == \"number\"")):
            self.assertIn(before, chunk)
            mutations.append(chunk.replace(before, after, 1))
        # Guard every old input as well as each new selector/test input. A count
        # alone would permit substituting a different source at the same length.
        for name in ("cmd/verify/workspace_profile.go", "cmd/verify/workspace_profile_test.go", "cmd/verify/main.go"):
            altered_header = excluded_source_projection(name)
            self.assertNotEqual(altered_header, TIMING_HEADER_ASSERTION)
            mutations.append(chunk.replace(TIMING_HEADER_ASSERTION, altered_header, 1))
        summary = re.search(r"^          jq -c -s '\n.*?^          ' out/ci/timing.jsonl\n", chunk, re.M | re.S)[0]
        for field in ("source_set", "source_set_sha256"):
            token = ", " + field
            self.assertIn(token, summary)
            mutations.append(chunk.replace(summary, summary.replace(token, "", 1), 1))
        for altered in mutations:
            self.assertNotEqual(altered, chunk)
            with self.subTest(altered=altered[:700]), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, altered, 1))

    def test_only_native_execution_runner_is_pinned_to_reviewed_ubuntu_24_04(self):
        jobs = workflow_jobs(self.text)
        self.assertIn("    runs-on: ubuntu-latest\n", jobs["observer-checks"])
        self.assertIn("    runs-on: ubuntu-latest\n", jobs["browser-demo"])
        # The original runner check now also covers the independent policy job.
        for name in ("native-execution", "policy-native-checks"):
            chunk = jobs[name]
            self.assertIn("    runs-on: ubuntu-24.04\n", chunk)
            for runner in ("ubuntu-latest", "ubuntu-26.04", "ubuntu-22.04"):
                changed = chunk.replace("runs-on: ubuntu-24.04", "runs-on: " + runner, 1)
                self.assertNotEqual(changed, chunk)
                with self.subTest(job=name, runner=runner), self.assertRaises(ValueError):
                    validate(self.text.replace(chunk, changed, 1))



    def test_policy_builder_source_identity_is_mandatory(self):
        name = "cmd/build-demo/api_policy_build.go"
        omissions = excluded_source_projection(name)
        substituted = TIMING_HEADER_ASSERTION.replace(
            NATIVE_SOURCE_PATHS_ASSERTION,
            '(.identity.source_files | keys) == ($e.source_paths | map(if . == '
            + json.dumps(name) + ' then "cmd/build-demo/substituted.go" else . end))', 1)
        for altered in (omissions, substituted):
            self.assertNotEqual(altered, TIMING_HEADER_ASSERTION)
            with self.subTest(replacement=altered), self.assertRaises(ValueError):
                validate(self.text.replace(TIMING_HEADER_ASSERTION, altered, 1))

    def test_last_index82_exact_finite_roster_and_unchanged_old80_prefix_are_required(self):
        named, _ = named_steps(self.text)
        comparison = named["Compare the ports with Go and replay every batch"]
        for clause in LAST_INDEX_CLAUSES:
            self.assertEqual(comparison.count(clause), 1)
            for replacement in ("true and", clause.replace("==", ">=", 1)):
                self.assertNotEqual(clause, replacement)
                with self.subTest(clause=clause, replacement=replacement), self.assertRaises(ValueError):
                    validate(self.text.replace(comparison, comparison.replace(clause, replacement, 1), 1))
        # A previous80-suite pass cannot accept either new source-bound entry.
        for before, after in (
                ('(.runs | length) == $e.totals.suites', '(.runs | length) == 80'),
                ('.ported_functions == 54 and .consumer_programs == 26 and .consumer_activities == 28 and',
                 '.ported_functions == 53 and .consumer_programs == 25 and .consumer_activities == 27 and'),
                ('.cases == $e.totals.cases and .passed == $e.totals.cases', '.cases == 173866 and .passed == 173866'),
                ('([.runs[].cases] | add) == $e.totals.cases', '([.runs[].cases] | add) == 173866'),
                ('([.runs[].batches] | add) == $e.totals.batches', '([.runs[].batches] | add) == 2252'),
                ('.expected_suites == $e.totals.suites', '.expected_suites == 80'),
                ('.successful_suites == $e.totals.suites', '.successful_suites == 80'),
                ('.expected_cases == $e.totals.cases', '.expected_cases == 173866'),
                ('.verified_cases == $e.totals.cases', '.verified_cases == 173866'),
                ('.observed_batches == $e.totals.batches', '.observed_batches == 2252'),
                ('.verified_batches == $e.totals.batches', '.verified_batches == 2252'),
                ('.event_count == $e.totals.timing_records', '.event_count == 2414'),
                ('.seq == ($e.totals.timing_records - 1)', '.seq == 2413'),
                ('(.identity.source_files | length) == $e.totals.source_paths', '(.identity.source_files | length) == 265'),
                ('(.source_sha256 | length) == 67', '(.source_sha256 | length) == 65')):
            self.assertIn(before, comparison)
            with self.subTest(rollback=before), self.assertRaises(ValueError):
                validate(self.text.replace(comparison, comparison.replace(before, after, 1), 1))
        for name in LAST_INDEX_NEW_SOURCE_PATHS:
            altered_header = excluded_source_projection(name)
            self.assertNotEqual(altered_header, TIMING_HEADER_ASSERTION)
            with self.subTest(source=name), self.assertRaises(ValueError):
                validate(self.text.replace(TIMING_HEADER_ASSERTION, altered_header, 1))

    def test_if_match80_exact_population_sources_and_old79_prefix_are_required(self):
        named, _ = named_steps(self.text)
        comparison = named["Compare the ports with Go and replay every batch"]
        validate(self.text)
        for clause in IF_MATCH_CLAUSES:
            self.assertEqual(comparison.count(clause), 1)
            for replacement in ("true and", clause.replace("==", ">=", 1)):
                self.assertNotEqual(clause, replacement)
                with self.subTest(clause=clause, replacement=replacement), self.assertRaises(ValueError):
                    validate(self.text.replace(comparison, comparison.replace(clause, replacement, 1), 1))
        for before, after in (
                ('(.runs | length) == $e.totals.suites', '(.runs | length) == 79'),
                ('.cases == $e.totals.cases and .passed == $e.totals.cases', '.cases == 173582 and .passed == 173582'),
                ('([.runs[].batches] | add) == $e.totals.batches', '([.runs[].batches] | add) == 2247'),
                ('.expected_suites == $e.totals.suites', '.expected_suites == 79'),
                ('.successful_suites == $e.totals.suites', '.successful_suites == 79'),
                ('.expected_cases == $e.totals.cases', '.expected_cases == 173582'),
                ('.verified_cases == $e.totals.cases', '.verified_cases == 173582'),
                ('.observed_batches == $e.totals.batches', '.observed_batches == 2247'),
                ('.verified_batches == $e.totals.batches', '.verified_batches == 2247'),
                ('.event_count == $e.totals.timing_records', '.event_count == 2407'),
                ('.seq == ($e.totals.timing_records - 1)', '.seq == 2406'),
                ('(.identity.source_files | length) == $e.totals.source_paths', '(.identity.source_files | length) == 251'),
                ('(.source_sha256 | length) == 67', '(.source_sha256 | length) == 63')):
            self.assertIn(before, comparison)
            with self.subTest(rollback=before), self.assertRaises(ValueError):
                validate(self.text.replace(comparison, comparison.replace(before, after, 1), 1))
        for name in IF_MATCH_NEW_SOURCE_PATHS:
            altered_header = excluded_source_projection(name)
            self.assertNotEqual(altered_header, TIMING_HEADER_ASSERTION)
            with self.subTest(source=name), self.assertRaises(ValueError):
                validate(self.text.replace(TIMING_HEADER_ASSERTION, altered_header, 1))
        for source in IF_MATCH_DECLARATIONS:
            line = "          ./.gooo check " + source + "\n"
            self.assertEqual(self.text.count(line), 1)
            for replacement in ("", line + line):
                with self.subTest(declaration=source), self.assertRaises(ValueError):
                    validate(self.text.replace(line, replacement, 1))

    def test_if_match_route_requires_full_scope_inputs_and_separate_cli_shape_controls(self):
        named, _ = named_steps(self.text)
        chunk = named[IF_MATCH_STEP_NAME]
        validate(self.text)
        for changed in ("", chunk + chunk,
                        chunk.replace("        if: " + FULL, "        if: " + DOCS, 1),
                        chunk.replace("        if: " + FULL, "        if: always()", 1),
                        chunk.replace("        timeout-minutes: 8", "        timeout-minutes: 80", 1),
                        chunk.replace("go build -mod=readonly -o out/if-match-input ./cmd/if-match-input", "true", 1),
                        chunk.replace(IF_MATCH_UNIT, "true", 1),
                        chunk.replace(IF_MATCH_COMMAND, IF_MATCH_COMMAND.replace('--go "$(go env GOROOT)/bin/go"', ''), 1),
                        chunk.replace("        run: |", "        continue-on-error: true\n        run: |", 1)):
            self.assertNotEqual(chunk, changed)
            with self.subTest(changed=changed[:300]), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, changed, 1))
        for clause in IF_MATCH_ROUTE_CLAUSES:
            with self.subTest(route_clause=clause), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, chunk.replace(clause, "true", 1), 1))
        malformed = named["Observe malformed literal Text inputs through the actual CLI"]
        for clause in IF_MATCH_CLI_CLAUSES:
            self.assertEqual(malformed.count(clause), 1)
            with self.subTest(cli_clause=clause), self.assertRaises(ValueError):
                validate(self.text.replace(malformed, malformed.replace(clause, "true and", 1), 1))

    def test_etag79_exact_population_sources_fields_and_old76_prefix_cannot_be_weakened(self):
        named, _ = named_steps(self.text)
        comparison = named["Compare the ports with Go and replay every batch"]
        validate(self.text)
        for clause in ETAG_CLAUSES:
            self.assertEqual(comparison.count(clause), 1)
            for replacement in ("true and", clause.replace("==", ">=", 1)):
                self.assertNotEqual(clause, replacement)
                with self.subTest(clause=clause, replacement=replacement), self.assertRaises(ValueError):
                    validate(self.text.replace(comparison, comparison.replace(clause, replacement, 1), 1))
        # Old76 acceptance counts cannot replace the current80 requirements.
        for before, after in (
                ('(.runs | length) == $e.totals.suites', '(.runs | length) == 76'),
                ('.cases == $e.totals.cases and .passed == $e.totals.cases', '.cases == 173335 and .passed == 173335'),
                ('([.runs[].batches] | add) == $e.totals.batches', '([.runs[].batches] | add) == 2240'),
                ('.expected_suites == $e.totals.suites', '.expected_suites == 76'),
                ('.successful_suites == $e.totals.suites', '.successful_suites == 76'),
                ('.expected_cases == $e.totals.cases', '.expected_cases == 173335'),
                ('.verified_cases == $e.totals.cases', '.verified_cases == 173335'),
                ('.observed_batches == $e.totals.batches', '.observed_batches == 2240'),
                ('.verified_batches == $e.totals.batches', '.verified_batches == 2240'),
                ('.event_count == $e.totals.timing_records', '.event_count == 2394'),
                ('.seq == ($e.totals.timing_records - 1)', '.seq == 2393'),
                ('(.identity.source_files | length) == $e.totals.source_paths', '(.identity.source_files | length) == 228'),
                ('(.source_sha256 | length) == 67', '(.source_sha256 | length) == 61')):
            self.assertIn(before, comparison)
            with self.subTest(rollback=before), self.assertRaises(ValueError):
                validate(self.text.replace(comparison, comparison.replace(before, after, 1), 1))
        for name in ETAG_NEW_SOURCE_PATHS:
            weakened = excluded_source_projection(name)
            self.assertNotEqual(weakened, TIMING_HEADER_ASSERTION)
            with self.subTest(source=name), self.assertRaises(ValueError):
                validate(self.text.replace(TIMING_HEADER_ASSERTION, weakened, 1))
        for source in ETAG_DECLARATIONS:
            line = "          ./.gooo check " + source + "\n"
            self.assertEqual(self.text.count(line), 1)
            for replacement in ("", line + line):
                with self.subTest(declaration=source), self.assertRaises(ValueError):
                    validate(self.text.replace(line, replacement, 1))
        host = named[HEADER_ADDITION_HOST_STEP_NAME]
        for before, after in (
                ('FRESH82_BOUND_ABI50_AND_ATOMIC_HOST_PASS', 'FRESH76_BOUND_ABI50_AND_ATOMIC_HOST_PASS'),
                ('.fresh_native_suites == 82', '.fresh_native_suites == 76')):
            self.assertIn(before, host)
            with self.subTest(host=before), self.assertRaises(ValueError):
                validate(self.text.replace(host, host.replace(before, after, 1), 1))

    def test_canonical76_exact_population_sources_fields_and_history_cannot_be_weakened(self):
        named, _ = named_steps(self.text)
        comparison = named["Compare the ports with Go and replay every batch"]
        validate(self.text)
        for clause in CANONICAL_CLAUSES:
            self.assertEqual(comparison.count(clause), 1)
            for replacement in ("true and", clause.replace("==", ">=", 1)):
                self.assertNotEqual(clause, replacement)
                with self.subTest(clause=clause, replacement=replacement), self.assertRaises(ValueError):
                    validate(self.text.replace(comparison, comparison.replace(clause, replacement, 1), 1))
        # The old74 prefix stays mandatory while the whole current population grows.
        # Each rollback target below remains an intentional negative, not a new PASS.
        for before, after in (
                ('(.runs | length) == $e.totals.suites', '(.runs | length) == 74'),
                ('.cases == $e.totals.cases and .passed == $e.totals.cases', '.cases == 171217 and .passed == 171217'),
                ('([.runs[].batches] | add) == $e.totals.batches', '([.runs[].batches] | add) == 2181'),
                ('.expected_suites == $e.totals.suites', '.expected_suites == 74'),
                ('.successful_suites == $e.totals.suites', '.successful_suites == 74'),
                ('.expected_cases == $e.totals.cases', '.expected_cases == 171217'),
                ('.verified_cases == $e.totals.cases', '.verified_cases == 171217'),
                ('.observed_batches == $e.totals.batches', '.observed_batches == 2181'),
                ('.verified_batches == $e.totals.batches', '.verified_batches == 2181'),
                ('.event_count == $e.totals.timing_records', '.event_count == 2331'),
                ('.seq == ($e.totals.timing_records - 1)', '.seq == 2330'),
                ('(.identity.source_files | length) == $e.totals.source_paths', '(.identity.source_files | length) == 207'),
                ('(.source_sha256 | length) == 67', '(.source_sha256 | length) == 59')):
            self.assertIn(before, comparison)
            with self.subTest(before=before), self.assertRaises(ValueError):
                validate(self.text.replace(comparison, comparison.replace(before, after, 1), 1))
        for name in CANONICAL_NEW_SOURCE_PATHS:
            weakened = excluded_source_projection(name)
            self.assertNotEqual(weakened, TIMING_HEADER_ASSERTION)
            with self.subTest(source=name), self.assertRaises(ValueError):
                validate(self.text.replace(TIMING_HEADER_ASSERTION, weakened, 1))
        for source in CANONICAL_DECLARATIONS:
            line = "          ./.gooo check " + source + "\n"
            self.assertEqual(self.text.count(line), 1)
            for replacement in ("", line + line):
                with self.subTest(declaration=source), self.assertRaises(ValueError):
                    validate(self.text.replace(line, replacement, 1))
        host = named[HEADER_ADDITION_HOST_STEP_NAME]
        for before, after in (
                ('FRESH82_BOUND_ABI50_AND_ATOMIC_HOST_PASS', 'FRESH74_BOUND_ABI50_AND_ATOMIC_HOST_PASS'),
                ('.fresh_native_suites == 82', '.fresh_native_suites == 74')):
            self.assertIn(before, host)
            with self.subTest(host=before), self.assertRaises(ValueError):
                validate(self.text.replace(host, host.replace(before, after, 1), 1))

    def test_addition74_exact_roster_fields_batches_and_fresh_host_routes_are_required(self):
        named, _ = named_steps(self.text)
        comparison = named["Compare the ports with Go and replay every batch"]
        validate(self.text)
        for clause in HEADER_ADDITION_CLAUSES:
            self.assertEqual(comparison.count(clause), 1)
            for replacement in ("true and", clause.replace("==", ">=", 1)):
                with self.subTest(clause=clause), self.assertRaises(ValueError):
                    validate(self.text.replace(comparison, comparison.replace(clause, replacement, 1), 1))
        for before, after in (
                ('(.runs | length) == $e.totals.suites', '(.runs | length) == 73'),
                ('.cases == $e.totals.cases and .passed == $e.totals.cases', '.cases == 171167 and .passed == 171167'),
                ('([.runs[].batches] | add) == $e.totals.batches', '([.runs[].batches] | add) == 2174'),
                ('.expected_suites == $e.totals.suites', '.expected_suites == 73'),
                ('.successful_suites == $e.totals.suites', '.successful_suites == 73'),
                ('.verified_cases == $e.totals.cases', '.verified_cases == 171167'),
                ('.observed_batches == $e.totals.batches', '.observed_batches == 2174'),
                ('.verified_batches == $e.totals.batches', '.verified_batches == 2174'),
                ('.event_count == $e.totals.timing_records', '.event_count == 2322'),
                ('.seq == ($e.totals.timing_records - 1)', '.seq == 2321'),
                ('(.identity.source_files | length) == $e.totals.source_paths', '(.identity.source_files | length) == 177')):
            self.assertIn(before, comparison)
            with self.subTest(before=before), self.assertRaises(ValueError):
                validate(self.text.replace(comparison, comparison.replace(before, after, 1), 1))
        for name in HEADER_ADDITION_NEW_SOURCE_PATHS:
            weakened = excluded_source_projection(name)
            self.assertNotEqual(weakened, TIMING_HEADER_ASSERTION)
            with self.subTest(source=name), self.assertRaises(ValueError):
                validate(self.text.replace(TIMING_HEADER_ASSERTION, weakened, 1))
        line = "          ./.gooo check examples/diagnostic-header-addition/main.gooo\n"
        with self.assertRaises(ValueError):
            validate(self.text.replace(line, "", 1))
        for name in (HEADER_ADDITION_INPUT_STEP_NAME, HEADER_ADDITION_HOST_STEP_NAME):
            chunk = named[name]
            mutations = ["", chunk + chunk, chunk.replace("        if: " + FULL + "\n", "", 1)]
            for before, after in (
                    ('--verification-out "$PWD/out/ci"', '--verification-out /tmp/old'),
                    ('--timing "$PWD/out/ci/timing.jsonl"', '--timing /tmp/old.jsonl'),
                    ('--go "$(go env GOROOT)/bin/go"', '--go /usr/bin/go')):
                self.assertIn(before, chunk)
                mutations.append(chunk.replace(before, after, 1))
            for altered in mutations:
                with self.subTest(step=name, altered=altered), self.assertRaises(ValueError):
                    validate(self.text.replace(chunk, altered, 1))
        for name, before, after in (
                (HEADER_ADDITION_INPUT_STEP_NAME, 'go build -mod=readonly -o out/header-addition-input ./cmd/header-addition-input', 'true'),
                (HEADER_ADDITION_INPUT_STEP_NAME, '--input-command "$PWD/out/header-addition-input"', '--input-command /tmp/old-helper'),
                (HEADER_ADDITION_INPUT_STEP_NAME, '.positive_requests == 4', '.positive_requests >= 1'),
                (HEADER_ADDITION_INPUT_STEP_NAME, '.prelaunch_refusals == 16', '.prelaunch_refusals == 0'),
                (HEADER_ADDITION_INPUT_STEP_NAME, '.actual_Gooo_commands == 8', '.actual_Gooo_commands == 0'),
                (HEADER_ADDITION_INPUT_STEP_NAME, '.typed_fields_checked == 128', '.typed_fields_checked == 16'),
                (HEADER_ADDITION_HOST_STEP_NAME, '.fresh_native_suites == 82', '.fresh_native_suites == 73'),
                (HEADER_ADDITION_HOST_STEP_NAME, '.fresh_addition_cases == 50', '.fresh_addition_cases == 1'),
                (HEADER_ADDITION_HOST_STEP_NAME, '.fresh_addition_batches == 7', '.fresh_addition_batches == 1'),
                (HEADER_ADDITION_HOST_STEP_NAME, '.ABI_generated_calls == 50', '.ABI_generated_calls == 1'),
                (HEADER_ADDITION_HOST_STEP_NAME, '.ABI_typed_fields == 800', '.ABI_typed_fields == 16'),
                (HEADER_ADDITION_HOST_STEP_NAME, '.host_test_functions == 18', '.host_test_functions == 1'),
                (HEADER_ADDITION_HOST_STEP_NAME, '.demo_status == "APPLIED"', 'true'),
                (HEADER_ADDITION_HOST_STEP_NAME, '.processes_closed == true', 'true'),
                (HEADER_ADDITION_HOST_STEP_NAME, '.source_bindings_unchanged == true', 'true')):
            chunk = named[name]
            self.assertIn(before, chunk)
            with self.subTest(step=name, before=before), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, chunk.replace(before, after, 1), 1))
        # Source-only host-reader controls remain common, including docs-only PRs.
        line = "          " + HEADER_ADDITION_HOST_UNIT + "\n"
        self.assertEqual(self.text.count(line), 1)
        for replacement in ("", line + line):
            with self.subTest(common_host_unit=replacement), self.assertRaises(ValueError):
                validate(self.text.replace(line, replacement, 1))
        # Current receipt consumers must never move ahead of their fresh producer.
        for name in (HEADER_ADDITION_INPUT_STEP_NAME, HEADER_ADDITION_HOST_STEP_NAME):
            chunk = named[name]
            reordered = self.text.replace(chunk, "", 1).replace(comparison, chunk + comparison, 1)
            with self.subTest(order=name), self.assertRaises(ValueError):
                validate(reordered)

    def test_actual_header_input_route_stage_argv_and_summary_cannot_be_dropped_or_weakened(self):
        named, _ = named_steps(self.text)
        chunk = named[HEADER_WIRE_STEP_NAME]
        validate(self.text)
        mutations = ["", chunk + chunk]
        for before, after in (
                ("        if: " + FULL + "\n", ""),
                ("go build -mod=readonly -o out/header-facts-input ./cmd/header-facts-input", "true"),
                ("--root \"$PWD\"", "--root /tmp/old"),
                ("--compiler \"$PWD/.gooo\"", "--compiler /tmp/old-gooo"),
                ("--input-command \"$PWD/out/header-facts-input\"", "--input-command /tmp/old-helper"),
                ("--go \"$(go env GOROOT)/bin/go\"", "--go /usr/bin/go"),
                ("--out \"$PWD/out/header-facts-input-route\"", "--out /tmp/old-results"),
                (".status == \"THREE_INPUT_ROUTES_AND16_PRELAUNCH_REFUSALS_PASS\"", "true"),
                (".failure == null", "true"),
                (".positive_requests == 3", ".positive_requests >= 1"),
                (".prelaunch_refusals == 16", ".prelaunch_refusals >= 1"),
                (".actual_Gooo_commands == 6", ".actual_Gooo_commands == 0"),
                (".typed_fields_checked == 72", ".typed_fields_checked == 12"),
                ("(.observations | length) == 6", "(.observations | length) >= 1"),
                ("' out/header-facts-input-route/summary.json", "' out/old/summary.json")):
            self.assertIn(before, chunk)
            mutations.append(chunk.replace(before, after, 1))
        for altered in mutations:
            with self.subTest(altered=altered), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, altered, 1))
        declaration = named["Check Gooo declarations"]
        misplaced = declaration + "          " + HEADER_WIRE_COMMAND + "\n"
        with self.assertRaises(ValueError):
            validate(self.text.replace(declaration, misplaced, 1))

    def test_header73_roster_sources_fields_modules_and_timing_gates_are_required(self):
        named, _ = named_steps(self.text)
        comparison = named["Compare the ports with Go and replay every batch"]
        validate(self.text)
        for clause in HEADER_CLAUSES:
            self.assertEqual(comparison.count(clause), 1)
            for replacement in ("true and", clause.replace("==", ">=", 1)):
                if replacement == clause:
                    continue
                with self.subTest(clause=clause, replacement=replacement), self.assertRaises(ValueError):
                    validate(self.text.replace(comparison, comparison.replace(clause, replacement, 1), 1))
        for before, after in (
                ('.ported_functions == 54 and .consumer_programs == 26 and .consumer_activities == 28 and',
                 '.ported_functions == 47 and .consumer_programs == 21 and .consumer_activities == 23 and'),
                ('(.runs | length) == $e.totals.suites and', '(.runs | length) == 70 and'),
                ('.cases == $e.totals.cases and .passed == $e.totals.cases and', '.cases == 170227 and .passed == 170227 and'),
                ('.expected_suites == $e.totals.suites', '.expected_suites == 70'),
                ('.successful_suites == $e.totals.suites', '.successful_suites == 70'),
                ('.expected_cases == $e.totals.cases', '.expected_cases == 170227'),
                ('.verified_cases == $e.totals.cases', '.verified_cases == 170227'),
                ('.observed_batches == $e.totals.batches', '.observed_batches == 2153'),
                ('.verified_batches == $e.totals.batches', '.verified_batches == 2153'),
                ('.event_count == $e.totals.timing_records', '.event_count == 2295'),
                ('.seq == ($e.totals.timing_records - 1)', '.seq == 2294'),
                ('(.source_sha256 | length) == 67', '(.source_sha256 | length) == 56'),
                ('(.identity.source_files | length) == $e.totals.source_paths', '(.identity.source_files | length) == 142'),
                ('original12-field diagnostics', 'original11-field diagnostics'),
                ('exact typed12 fields', 'exact typed11 fields'),
                ('independent inner typed canonical parent digest NOT_CHECKED', 'independent inner typed canonical parent digest CHECKED')):
            self.assertIn(before, comparison)
            with self.subTest(before=before), self.assertRaises(ValueError):
                validate(self.text.replace(comparison, comparison.replace(before, after, 1), 1))
        # All283 identities are an exact roster; a count alone permits substitution.
        for name in ("cmd/header-facts-input/main.go", "cmd/verify/header_contracts.go",
                     "cmd/verify/header_contracts_test.go", "cmd/verify/legacy_workspace_test.go",
                     "cmd/verify/header_corpus.go",
                     "cmd/verify/header_corpus_test.go", "cmd/verify/external_oracles.go",
                     "cmd/verify/external_oracles_test.go", "go.mod", "go.sum",
                     "examples/header-field-facts/contract.json", "examples/header-field-facts/corpus.json",
                     "ports/net/httpguts/headers.gooo", "ports/net/httpguts/provenance.json",
                     "tools/check-header-facts-inputs.py"):
            weakened = excluded_source_projection(name)
            self.assertNotEqual(weakened, TIMING_HEADER_ASSERTION)
            with self.subTest(source=name), self.assertRaises(ValueError):
                validate(self.text.replace(TIMING_HEADER_ASSERTION, weakened, 1))
        self.assertEqual(comparison.count(HEADER_SOURCE_SET_ASSERTION), 1)
        weakened = [comparison.replace(HEADER_SOURCE_SET_ASSERTION, "", 1)]
        for line in HEADER_SOURCE_SET_ASSERTION.splitlines()[1:-1]:
            weakened.append(comparison.replace(HEADER_SOURCE_SET_ASSERTION,
                            HEADER_SOURCE_SET_ASSERTION.replace(line + "\n", "", 1), 1))
        for altered in weakened:
            self.assertNotEqual(comparison, altered)
            with self.subTest(source_map=altered[-500:]), self.assertRaises(ValueError):
                validate(self.text.replace(comparison, altered, 1))
        for replacement in ("", HEADER_MODULE_PREFLIGHT + HEADER_MODULE_PREFLIGHT,
                            HEADER_MODULE_PREFLIGHT.replace("go mod verify", "true"),
                            HEADER_MODULE_PREFLIGHT.replace("golang.org/x/text", "golang.org/x/image")):
            with self.subTest(preflight=replacement), self.assertRaises(ValueError):
                validate(self.text.replace(HEADER_MODULE_PREFLIGHT, replacement, 1))
        for replacement in ("", "-mod=mod ", "-mod=vendor "):
            altered = NATIVE_COMPARISON_COMMAND.replace("-mod=readonly ", replacement, 1)
            with self.subTest(readonly=replacement), self.assertRaises(ValueError):
                validate(self.text.replace(NATIVE_COMPARISON_COMMAND, altered, 1))
        for source in HEADER_DECLARATIONS:
            line = "          ./.gooo check " + source + "\n"
            self.assertEqual(self.text.count(line), 1)
            for replacement in ("", line + line):
                with self.subTest(declaration=source), self.assertRaises(ValueError):
                    validate(self.text.replace(line, replacement, 1))

    def test_path_clean_alias_exact_appended_roster_sources_and_scope_are_required(self):
        named, _ = named_steps(self.text)
        comparison = named["Compare the ports with Go and replay every batch"]
        # Establish the full current positive source before intended mutations.
        validate(self.text)
        for clause in PATH_ALIAS_CLAUSES:
            self.assertEqual(comparison.count(clause), 1)
            weakened = clause.replace("==", ">=", 1)
            for replacement in ("true and", weakened):
                self.assertNotEqual(clause, replacement)
                altered = comparison.replace(clause, replacement, 1)
                with self.subTest(clause=clause, replacement=replacement), self.assertRaises(ValueError):
                    validate(self.text.replace(comparison, altered, 1))
        for before, after in (
                ('.runs[68].batches == 77', '.runs[68].batches == 76'),
                ('.runs[69].batches == 4', '.runs[69].batches == 1'),
                ('.expected_suites == $e.totals.suites', '.expected_suites == 68'),
                ('.successful_suites == $e.totals.suites', '.successful_suites == 68'),
                ('.verified_cases == $e.totals.cases', '.verified_cases == 160266'),
                ('.observed_batches == $e.totals.batches', '.observed_batches == 2072'),
                ('.verified_batches == $e.totals.batches', '.verified_batches == 2072'),
                ('.event_count == $e.totals.timing_records', '.event_count == 2210'),
                ('.seq == ($e.totals.timing_records - 1)', '.seq == 2209')):
            self.assertIn(before, comparison)
            with self.subTest(before=before), self.assertRaises(ValueError):
                validate(self.text.replace(comparison, comparison.replace(before, after, 1), 1))
        for source in PATH_ALIAS_DECLARATIONS:
            line = "          ./.gooo check " + source + "\n"
            self.assertEqual(self.text.count(line), 1)
            for replacement in ("", line + line):
                with self.subTest(source=source, replacement=replacement), self.assertRaises(ValueError):
                    validate(self.text.replace(line, replacement, 1))

    def test_literal_report_combined_profile_counts_cannot_reuse_previous_profile(self):
        named, _ = named_steps(self.text)
        comparison = named["Compare the ports with Go and replay every batch"]
        changes = (
            (".runs[61].batches == 235", ".runs[61].batches == 281"),
            ("([.runs[:62][].batches] | add) == 2058", "([.runs[:62][].batches] | add) == 2104"),
            ("([.runs[:66][].batches] | add) == 2063", "([.runs[:66][].batches] | add) == 2109"),
            ("([.runs[].batches] | add) == $e.totals.batches", "([.runs[].batches] | add) == 2072"),
            (".event_count == $e.totals.timing_records", ".event_count == 2210"),
            (".seq == ($e.totals.timing_records - 1)", ".seq == 2209"),
            ("(.identity.source_files | length) == $e.totals.source_paths", "(.identity.source_files | length) == 120"),
        )
        for before, after in changes:
            self.assertIn(before, comparison)
            altered = comparison.replace(before, after, 1)
            self.assertNotEqual(comparison, altered)
            with self.subTest(before=before), self.assertRaises(ValueError):
                validate(self.text.replace(comparison, altered, 1))

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
            ".ported_functions == 54 and .consumer_programs == 26 and .consumer_activities == 28 and",
            "(.runs | length) == $e.totals.suites and", ".cases == $e.totals.cases and .passed == $e.totals.cases and",
            "([.runs[].cases] | add) == $e.totals.cases and ([.runs[].batches] | add) == $e.totals.batches and",
            "([.runs[:62][].cases] | add) == 159781 and ([.runs[:62][].batches] | add) == 2058 and",
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
        for name in HEAVY | PREFLIGHT | UPLOADS:
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
            if name == LAST_INDEX_INPUT_STEP_NAME:
                rebound_input = chunk.replace("out/last-index-input-rejections", "out/other", 1)
            elif "--inputs" in chunk:
                rebound_input = chunk.replace("--inputs examples/literal-text-report/inputs.json", "--inputs other.json", 1)
            else:
                rebound_input = chunk.replace("out/literal-text-input-rejections", "out/other", 1)
            for altered in (rebound_input,
                            chunk.replace("        run: |", "        continue-on-error: true\n        run: |", 1)):
                self.assertNotEqual(chunk, altered)
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
                              (".runs[61].batches == 235", ".runs[61].batches == 137"),
                              ("are NOT_CHECKED", "are CHECKED")):
            self.assertIn(before, self.text)
            altered = self.text.replace(before, after, 1)
            self.assertNotEqual(self.text, altered)
            with self.subTest(before=before), self.assertRaises(ValueError):
                validate(altered)

    def test_last_index_malformed_gate_requires_fresh_bounded_full_scope_observations(self):
        named, _ = named_steps(self.text)
        chunk = named[LAST_INDEX_INPUT_STEP_NAME]
        for changed in ("", chunk + chunk,
                        chunk.replace("        if: " + FULL, "        if: " + DOCS, 1),
                        chunk.replace("        if: " + FULL, "        if: always()", 1),
                        chunk.replace("timeout-minutes: 8", "timeout-minutes: 80", 1),
                        chunk.replace(LAST_INDEX_INPUT_COMMAND, "true", 1),
                        chunk.replace('--compiler "$PWD/.gooo"', '--compiler "$PWD/old-gooo"', 1),
                        chunk.replace('--go "$(go env GOROOT)/bin/go"', "", 1),
                        chunk.replace("        run: |", "        continue-on-error: true\n        run: |", 1)):
            self.assertNotEqual(chunk, changed)
            with self.subTest(changed=changed[:300]), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, changed, 1))
        for before, after in (
                ('gooo/last-index-input-rejections/v1', 'gooo/literal-text-input-rejections/v1'),
                ('.planned_framework_inputs == 56', '.planned_framework_inputs >= 1'),
                ('.observed_framework_rejections == 56', '.observed_framework_rejections == 28'),
                ('"go/strings:LastIndex": 28', '"go/strings:Index": 28'),
                ('"app/literal-final-marker-report:Report": 28', '"app/literal-text-report:Report": 28'),
                ('.planned_by_entry == .observed_by_entry', 'true'),
                ('(.source_bindings | length) == 4', '(.source_bindings | length) >= 1'),
                ('(.manifest_bindings | length) == 2', '(.manifest_bindings | length) == 1'),
                ('(.observer_source_bindings | length) == 3', '(.observer_source_bindings | length) == 1'),
                ('.compiler_identity_binding, .compiler_executable_binding, .go_executable_binding, .upstream_binding', '.compiler_identity_binding'),
                ('(.cases | length) == 56', '(.cases | length) == 28'),
                ('.decision == "FAIL_CLOSED"', 'true'),
                ('.verified_rejection_stage == .expected_rejection_stage', 'true'),
                ('.exit_code == 1', '.exit_code == 0'),
                ('.result_present == false and .runtime_present == false', 'true'),
                ('.actual_runtime_stage == "NOT_REPORTED"', '.actual_runtime_stage == "SAFE"'),
                ('.input_binding, .manifest_binding, .receipt_binding', '.receipt_binding'),
                ("' out/last-index-input-rejections/summary.json", "' out/old/summary.json")):
            self.assertIn(before, chunk)
            with self.subTest(gate=before), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, chunk.replace(before, after, 1), 1))
        without = self.text.replace(chunk, "", 1)
        for name in ("Observe malformed literal Text inputs through the actual CLI",
                     "Observe malformed FitsCrop inputs through the actual CLI"):
            anchor = named[name]
            changed = without.replace(anchor, chunk + anchor if "literal Text" in name else anchor + chunk, 1)
            self.assertNotEqual(changed, self.text)
            with self.subTest(stage_order=name), self.assertRaises(ValueError):
                validate(changed)

    def test_last_index_common_controls_and_declarations_cannot_be_skipped_or_duplicated(self):
        for line in ("          " + LAST_INDEX_INPUT_UNIT + "\n",
                     "          ./.gooo check ports/strings/last_index.gooo\n",
                     "          ./.gooo check examples/literal-final-marker-report/main.gooo\n"):
            self.assertEqual(self.text.count(line), 1)
            for replacement in ("", line + line):
                with self.subTest(control=line, mutation=replacement), self.assertRaises(ValueError):
                    validate(self.text.replace(line, replacement, 1))

    def test_cut_malformed_entries_counts_and_declarations_cannot_be_removed(self):
        named, _ = named_steps(self.text)
        malformed = named["Observe malformed literal Text inputs through the actual CLI"]
        for token in ('"go/strings:Cut"', '"go/strings:CutPrefix"', '"go/strings:CutSuffix"',
                      '"app/literal-cut-report:Report"', '.planned_framework_inputs == 224',
                      '.observed_framework_rejections == 224', '(.cases | length) == 224'):
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
        validate(self.text)
        extra = "      - name: Unreviewed native command\n        run: go run ./cmd/verify\n\n"
        with self.assertRaises(ValueError):
            validate(self.text.replace("  browser-demo:\n", extra + "  browser-demo:\n", 1))
        self.assertIn(DEPLOY_NEEDS, self.text)
        changed = self.text.replace(DEPLOY_NEEDS, '    needs: [browser-demo]', 1)
        self.assertNotEqual(changed, self.text)
        with self.assertRaises(ValueError):
            validate(changed)
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

    def test_docs_log_cannot_describe_the_old_root_only_scope(self):
        old = "Root README/CONTRIBUTING content-only PR"
        current = "Allowlisted static-document content-only PR"
        self.assertIn(current, self.text)
        with self.assertRaises(ValueError):
            validate(self.text.replace(current, old, 1))

    def test_docs_artifact_cannot_include_native_receipts(self):
        named, _ = named_steps(self.text)
        chunk = named["Preserve documentation scope and compiler identity"]
        broken = chunk.replace("            out/compiler-build.json\n", "            out/ci/\n")
        with self.assertRaises(ValueError):
            validate(self.text.replace(chunk, broken, 1))

    def test_policy_producers_remain_exactly_once_in_their_independent_job(self):
        validate(self.text)
        jobs = workflow_jobs(self.text)
        policy, _ = policy_steps(jobs['policy-native-checks'])
        _, native, _ = job_steps(jobs['native-execution'])
        for name in POLICY_HEAVY | {POLICY_PILOT_UPLOAD}:
            chunk = policy[name]
            self.assertNotIn(name, native)
            self.assertEqual(self.text.count(chunk), 1)
            for replacement in ('', chunk + chunk):
                with self.subTest(step=name, replacement=replacement[:80]), self.assertRaises(ValueError):
                    validate(self.text.replace(chunk, replacement, 1))
            without = self.text.replace(chunk, '', 1)
            anchor = native['Upload execution and diagnostic evidence']
            with self.subTest(moved_back_to_native=name), self.assertRaises(ValueError):
                validate(without.replace(anchor, chunk + anchor, 1))
        self.assertEqual(self.text.count('      - name: Classify the exact checked PR tree\n'), 3)

    def test_policy_setup_identity_and_native_steps_keep_fail_closed_full_scope(self):
        jobs = workflow_jobs(self.text)
        policy, unnamed = policy_steps(jobs['policy-native-checks'])
        identity = policy['Build and identify the pinned compiler from a clean checkout']
        for name, chunk in [('Go setup', unnamed[1]), ('compiler identity', identity)] + [
                (name, policy[name]) for name in sorted(POLICY_HEAVY)]:
            self.assertEqual(field(chunk, 'if'), FULL)
            # Missing/unknown classifier output must retain the original full
            # path; equality to false would silently omit that work.
            for replacement in ('', 'steps.change-scope.outputs.docs_only == \'false\'',
                                DOCS, 'always()', "github.event_name == 'push'"):
                altered = chunk.replace('        if: ' + FULL + '\n',
                                        ('        if: ' + replacement + '\n') if replacement else '', 1)
                with self.subTest(step=name, condition=replacement), self.assertRaises(ValueError):
                    validate(self.text.replace(chunk, altered, 1))
        for before, after in (('sh tools/bootstrap.sh', 'true'),
                              ('version --build --json', 'version --json'),
                              ('"83928fa4538245abe33b08549da47f8cbdc50ed2"', '"old-source"'),
                              ('"CLEAN_VCS"', '"MODIFIED_VCS"'),
                              ('"go1.27.2"', '"go1.27.1"'),
                              ('        run: |', '        continue-on-error: true\n        run: |')):
            self.assertIn(before, identity)
            with self.subTest(identity=before), self.assertRaises(ValueError):
                validate(self.text.replace(identity, identity.replace(before, after, 1), 1))

    def test_policy_evidence_and_documentation_scopes_cannot_mix_or_disappear(self):
        jobs = workflow_jobs(self.text)
        policy, _ = policy_steps(jobs['policy-native-checks'])
        for name in (POLICY_PILOT_UPLOAD, POLICY_UPLOAD):
            chunk = policy[name]
            for condition in ('', FULL, 'always()', 'always() && ' + DOCS):
                altered = chunk.replace('        if: always() && ' + FULL + '\n',
                                        ('        if: ' + condition + '\n') if condition else '', 1)
                with self.subTest(upload=name, condition=condition), self.assertRaises(ValueError):
                    validate(self.text.replace(chunk, altered, 1))
        upload = policy[POLICY_UPLOAD]
        for token in ('            out/ci-scope.json\n', '            out/compiler-build.json\n',
                      '            out/cel-api-policy/\n', '            out/api-policy-revision/\n',
                      '            out/api-policy-original-download.json\n',
                      '            out/agentmesh-service-policy/\n'):
            self.assertIn(token, upload)
            with self.subTest(missing_path=token), self.assertRaises(ValueError):
                validate(self.text.replace(upload, upload.replace(token, '', 1), 1))
        for before, after in (('name: gooo-go-ports-policy-evidence', 'name: gooo-go-ports-evidence'),
                              ('if-no-files-found: warn', 'if-no-files-found: ignore'),
                              ('retention-days: 14', 'retention-days: 1'),
                              ('            out/agentmesh-service-policy/\n', '            out/\n')):
            with self.subTest(upload_field=before), self.assertRaises(ValueError):
                validate(self.text.replace(upload, upload.replace(before, after, 1), 1))
        report, artifact = policy[POLICY_DOC_REPORT], policy[POLICY_DOC_ARTIFACT]
        for name, chunk in ((POLICY_UPLOAD, upload), (POLICY_DOC_REPORT, report), (POLICY_DOC_ARTIFACT, artifact)):
            with self.subTest(missing_step=name), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, '', 1))
        for before, after in (('        if: ' + DOCS, '        if: ' + FULL),
                              ('produced no native policy execution/replay receipts', 'produced fresh native policy execution/replay receipts'),
                              ('existing observations are not rebound', 'existing observations are rebound')):
            self.assertIn(before, report)
            with self.subTest(docs_report=before), self.assertRaises(ValueError):
                validate(self.text.replace(report, report.replace(before, after, 1), 1))
        for before, after in (('always() && ' + DOCS, 'always()'),
                              ('name: gooo-go-ports-policy-documentation-scope', 'name: gooo-go-ports-policy-evidence'),
                              ('path: out/ci-scope.json', 'path: out/compiler-build.json'),
                              ('path: out/ci-scope.json', 'path: out/'),
                              ('if-no-files-found: error', 'if-no-files-found: warn'),
                              ('retention-days: 14', 'retention-days: 1')):
            with self.subTest(docs_artifact=before), self.assertRaises(ValueError):
                validate(self.text.replace(artifact, artifact.replace(before, after, 1), 1))


    def test_autobot_original_oracles_actual_converter_and_both_native_phases_are_required(self):
        validate(self.text)
        policy, _ = policy_steps(workflow_jobs(self.text)['policy-native-checks'])
        step = policy[AUTOBOT_STEP]
        self.assertTrue(NATIVE_COMMAND.search(AUTOBOT_COMMAND))
        self.assertFalse(NATIVE_COMMAND.search(AUTOBOT_CONTROLS))
        for token in (AUTOBOT_COMMAND, AUTOBOT_BUILD, AUTOBOT_CONTROLS) + AUTOBOT_CLAUSES:
            self.assertEqual(step.count(token), 1)
            with self.subTest(missing=token), self.assertRaises(ValueError):
                validate(self.text.replace(step, step.replace(token, '', 1), 1))
        for before, after in ((AUTOBOT_COMMAND, AUTOBOT_COMMAND + ' || true'),
                              ('timeout-minutes: 6', 'timeout-minutes: 40'),
                              ('        run: |', '        continue-on-error: true\n        run: |')):
            with self.subTest(weakened=before), self.assertRaises(ValueError):
                validate(self.text.replace(step, step.replace(before, after, 1), 1))
        upload = policy[POLICY_UPLOAD]
        with self.assertRaises(ValueError):
            validate(self.text.replace(upload, upload.replace('            out/autobot-resolver/\n', '', 1), 1))

    def test_editable_policy_native_and_refusal_checks_cannot_be_skipped(self):
        validate(self.text)
        policy, _ = policy_steps(workflow_jobs(self.text)['policy-native-checks'])
        step = policy[EDITABLE_POLICY_STEP]
        self.assertTrue(NATIVE_COMMAND.search(EDITABLE_POLICY_COMMAND))
        self.assertFalse(NATIVE_COMMAND.search(EDITABLE_POLICY_COMMON))
        for token in (EDITABLE_POLICY_COMMAND,) + EDITABLE_POLICY_CLAUSES:
            with self.subTest(missing=token), self.assertRaises(ValueError):
                validate(self.text.replace(step, step.replace(token, '', 1), 1))
        for replacement in ('', step + step, step.replace(FULL, DOCS, 1),
                            step.replace('timeout-minutes: 12', 'timeout-minutes: 40', 1),
                            step.replace('        run: |', '        continue-on-error: true\n        run: |', 1)):
            with self.subTest(step=replacement[:80]), self.assertRaises(ValueError):
                validate(self.text.replace(step, replacement, 1))
        named, _ = named_steps(self.text)
        for chunk, command in [(named['Check the Go observer'], EDITABLE_POLICY_COMMON)] + [
                (named['Check Gooo declarations'], './.gooo check ' + source)
                for source in EDITABLE_POLICY_DECLARATIONS]:
            with self.subTest(common=command), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, chunk.replace(command, '', 1), 1))
        upload = policy[POLICY_UPLOAD]
        with self.assertRaises(ValueError):
            validate(self.text.replace(upload, upload.replace('            out/editable-api-policy/\n', '', 1), 1))

    def test_editable_helper_official_archive_and_each_observation_gate_are_required(self):
        policy, _ = policy_steps(workflow_jobs(self.text)['policy-native-checks'])
        step = policy[EDITABLE_POLICY_STEP]
        self.assertEqual(literal_commands(step), [EDITABLE_POLICY_COMMAND, EDITABLE_POLICY_HELPER_COMMAND])
        self.assertTrue(NATIVE_COMMAND.search(EDITABLE_POLICY_HELPER_COMMAND))
        self.assertEqual(step.count(EDITABLE_POLICY_SOURCE_SETUP), 1)
        for token in (EDITABLE_POLICY_HELPER_COMMAND, EDITABLE_POLICY_SOURCE_SETUP) + EDITABLE_POLICY_HELPER_CLAUSES:
            with self.subTest(missing_helper_gate=token), self.assertRaises(ValueError):
                validate(self.text.replace(step, step.replace(token, '', 1), 1))
        for old, new in (('9dbd1c08b855a3c27ae43a8050531afaba724d90b24d4f72041db3a8a4a285b0', '0' * 64),
                         ('https://github.com/kimjooyoon/meta-ontology-go/releases/download/v0.6.36-dev/gooo-linux-amd64.tar.gz', 'https://example.invalid/unpinned.tar.gz'),
                         ('sha256sum --check --strict', 'true'),
                         ('--compiler "$PWD/out/source-inspection036/gooo"', '--compiler "$PWD/.gooo"'),
                         ('.default_package_commands == 0', '.default_package_commands == 1'),
                         ('.prepared_inspections == 4', '.prepared_inspections == 0'),
                         ('.typed_fields_compared == 90', '.typed_fields_compared == 45')):
            self.assertIn(old, step)
            with self.subTest(weakened_helper=old), self.assertRaises(ValueError):
                validate(self.text.replace(step, step.replace(old, new, 1), 1))

    def test_editable_helper_artifacts_are_retained_on_failure_but_not_docs_only(self):
        policy, _ = policy_steps(workflow_jobs(self.text)['policy-native-checks'])
        upload = policy[POLICY_UPLOAD]
        token = '            out/editable-api-policy-helpers/\n'
        self.assertEqual(upload.count(token), 1)
        self.assertNotIn(token, policy[POLICY_DOC_ARTIFACT])
        with self.assertRaises(ValueError):
            validate(self.text.replace(upload, upload.replace(token, '', 1), 1))

    def test_versioned_grant_step_is_owned_full_only_and_after_export(self):
        validate(self.text)
        jobs = workflow_jobs(self.text)
        policy, _ = policy_steps(jobs['policy-native-checks'])
        _, native, _ = job_steps(jobs['native-execution'])
        chunk = policy[VERSIONED_GRANT_STEP]
        self.assertEqual(self.text.count(chunk), 1)
        self.assertNotIn(VERSIONED_GRANT_STEP, native)
        self.assertLess(jobs['policy-native-checks'].index(policy[ETAG_PROJECT_STEP]),
                        jobs['policy-native-checks'].index(chunk))
        for replacement in ('', chunk + chunk,
                            chunk.replace('        if: ' + FULL + '\n', '', 1),
                            chunk.replace(FULL, DOCS, 1),
                            chunk.replace(FULL, "steps.change-scope.outputs.docs_only == 'false'", 1),
                            chunk.replace('timeout-minutes: 8', 'timeout-minutes: 40', 1),
                            chunk.replace('        run: |', '        continue-on-error: true\n        run: |', 1)):
            with self.subTest(step_mutation=replacement[:90]), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, replacement, 1))
        without = self.text.replace(chunk, '', 1)
        for anchor in (policy[ETAG_PROJECT_STEP], native['Compare the ports with Go and replay every batch']):
            with self.subTest(wrong_owner_or_order=anchor[:80]), self.assertRaises(ValueError):
                validate(without.replace(anchor, chunk + anchor, 1))

    def test_versioned_grant_current_command_and_every_summary_gate_are_required(self):
        policy, _ = policy_steps(workflow_jobs(self.text)['policy-native-checks'])
        chunk = policy[VERSIONED_GRANT_STEP]
        self.assertEqual(literal_commands(chunk), [VERSIONED_GRANT_COMMAND])
        for token in (VERSIONED_GRANT_COMMAND,) + VERSIONED_GRANT_CLAUSES:
            self.assertEqual(chunk.count(token), 1)
            with self.subTest(missing_command_or_gate=token), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, chunk.replace(token, '', 1), 1))
        for before, after in (('--module-cache "$(go env GOMODCACHE)"', '--module-cache /tmp/unbound'),
                              ('--compiler "$PWD/.gooo"', '--compiler /tmp/unbound'),
                              ('.cases == 48', '.cases == 47'),
                              ('.input_echoes_each_phase == 288', '.input_echoes_each_phase == 282'),
                              ('.typed_fields_each_phase == 576', '.typed_fields_each_phase == 564'),
                              ('.source_images == 12', '.source_images == 1'),
                              ('.failure == null', '.failure != null'),
                              ('.processes_closed == true', '.processes_closed == false'),
                              ('.owned_work_removed == true', '.owned_work_removed == false')):
            with self.subTest(weakened_gate=before), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, chunk.replace(before, after, 1), 1))
        self.assertNotRegex(chunk, r'\.(?:batches|batch_count|commands|command_count)\s*==')

    def test_versioned_grant_common_checks_and_declaration_cannot_be_skipped(self):
        named, _ = named_steps(self.text)
        observer = named['Check the Go observer']
        declarations = named['Check Gooo declarations']
        self.assertTrue(NATIVE_COMMAND.search(VERSIONED_GRANT_COMMAND))
        self.assertFalse(NATIVE_COMMAND.search(VERSIONED_GRANT_COMMON_COMMAND))
        self.assertEqual(literal_commands(observer).count(VERSIONED_GRANT_COMMON_COMMAND), 1)
        self.assertEqual(literal_commands(declarations).count('./.gooo check ' + VERSIONED_GRANT_DECLARATION), 1)
        for chunk, command in ((observer, VERSIONED_GRANT_COMMON_COMMAND),
                               (declarations, './.gooo check ' + VERSIONED_GRANT_DECLARATION)):
            line = '          ' + command + '\n'
            for replacement in ('', line + line,
                                '          if false; then\n  ' + line + '          fi\n'):
                with self.subTest(common_or_declaration=command, mutation=replacement[:80]), self.assertRaises(ValueError):
                    validate(self.text.replace(chunk, chunk.replace(line, replacement, 1), 1))
        with self.assertRaises(ValueError):
            validate(self.text.replace(observer, observer.replace(VERSIONED_GRANT_COMMON_COMMAND, VERSIONED_GRANT_COMMAND, 1), 1))

    def test_versioned_grant_artifact_is_failure_safe_and_never_documentation(self):
        policy, _ = policy_steps(workflow_jobs(self.text)['policy-native-checks'])
        upload, docs = policy[POLICY_UPLOAD], policy[POLICY_DOC_ARTIFACT]
        line = '            out/versioned-grant-change/\n'
        self.assertEqual(upload.count(line), 1)
        for replacement in ('', line + line):
            with self.subTest(artifact_path=replacement), self.assertRaises(ValueError):
                validate(self.text.replace(upload, upload.replace(line, replacement, 1), 1))
        with self.assertRaises(ValueError):
            validate(self.text.replace(docs, docs.replace('path: out/ci-scope.json', 'path: out/versioned-grant-change/', 1), 1))
        for condition in ('', FULL, DOCS, 'always()'):
            changed = upload.replace('        if: always() && ' + FULL + '\n',
                                     '        if: ' + condition + '\n' if condition else '', 1)
            with self.subTest(upload_condition=condition), self.assertRaises(ValueError):
                validate(self.text.replace(upload, changed, 1))

    def test_exported_etag_requires_full_scope_and_fresh_source_commands(self):
        validate(self.text)
        jobs = workflow_jobs(self.text)
        policy, _ = policy_steps(jobs['policy-native-checks'])
        chunk = policy[ETAG_PROJECT_STEP]
        self.assertEqual(field(chunk, 'if'), FULL)
        for condition in ('', DOCS, "steps.change-scope.outputs.docs_only == 'false'", 'always()'):
            altered = chunk.replace('        if: ' + FULL + '\n',
                                    ('        if: ' + condition + '\n') if condition else '', 1)
            with self.subTest(condition=condition), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, altered, 1))
        for token in ETAG_PROJECT_COMMANDS + ETAG_PROJECT_CLAUSES:
            self.assertEqual(chunk.count(token), 1)
            with self.subTest(missing_command_or_gate=token), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, chunk.replace(token, '', 1), 1))
        for replacement in ('', chunk + chunk):
            with self.subTest(step_roster=replacement[:40]), self.assertRaises(ValueError):
                validate(self.text.replace(chunk, replacement, 1))

    def test_exported_etag_artifacts_cannot_be_dropped_or_mixed_with_docs(self):
        validate(self.text)
        policy, _ = policy_steps(workflow_jobs(self.text)['policy-native-checks'])
        upload = policy[POLICY_UPLOAD]
        for path in ('out/etag-project/', 'out/etag-project-evidence/'):
            line = '            ' + path + '\n'
            self.assertEqual(upload.count(line), 1)
            with self.subTest(missing_artifact=path), self.assertRaises(ValueError):
                validate(self.text.replace(upload, upload.replace(line, '', 1), 1))
            docs = policy[POLICY_DOC_ARTIFACT]
            altered = docs.replace('path: out/ci-scope.json', 'path: ' + path, 1)
            with self.subTest(docs_artifact=path), self.assertRaises(ValueError):
                validate(self.text.replace(docs, altered, 1))

    def test_independent_sidecars_move_exactly_once_with_same_full_scope(self):
        validate(self.text)
        jobs = workflow_jobs(self.text)
        _, native, _ = job_steps(jobs['native-execution'])
        policy, _ = policy_steps(jobs['policy-native-checks'])
        for name in INDEPENDENT_STEPS:
            chunk = policy[name]
            self.assertNotIn(name, native)
            self.assertEqual(self.text.count(chunk), 1)
            self.assertEqual(field(chunk, 'if'), FULL)
            for replacement in ('', chunk + chunk,
                                chunk.replace('        if: ' + FULL + '\n', '', 1),
                                chunk.replace(FULL, DOCS, 1),
                                chunk.replace(FULL, "steps.change-scope.outputs.docs_only == 'false'", 1),
                                chunk.replace('        run: |', '        continue-on-error: true\n        run: |', 1)):
                with self.subTest(step=name, mutation=replacement[:80]), self.assertRaises(ValueError):
                    validate(self.text.replace(chunk, replacement, 1))
            without = self.text.replace(chunk, '', 1)
            anchor = native['Compare the ports with Go and replay every batch']
            with self.subTest(moved_back_to_native=name), self.assertRaises(ValueError):
                validate(without.replace(anchor, chunk + anchor, 1))
        self.assertEqual(tuple(jobs), JOB_ORDER)
        self.assertEqual(self.text.count('          sh tools/bootstrap.sh\n'), 3)
        self.assertEqual(self.text.count('      - name: Classify the exact checked PR tree\n'), 3)
        self.assertEqual(deployment_expression(self.text), DEPLOY_IF)

    def test_independent_sidecar_artifacts_are_complete_failure_safe_and_not_docs(self):
        validate(self.text)
        policy, _ = policy_steps(workflow_jobs(self.text)['policy-native-checks'])
        upload, docs = policy[POLICY_UPLOAD], policy[POLICY_DOC_ARTIFACT]
        self.assertEqual(field(upload, 'if'), 'always() && ' + FULL)
        for path in INDEPENDENT_ARTIFACT_PATHS:
            line = '            ' + path + '\n'
            self.assertEqual(upload.count(line), 1)
            with self.subTest(missing_path=path), self.assertRaises(ValueError):
                validate(self.text.replace(upload, upload.replace(line, '', 1), 1))
            with self.subTest(docs_path=path), self.assertRaises(ValueError):
                validate(self.text.replace(docs, docs.replace('path: out/ci-scope.json', 'path: ' + path, 1), 1))
        for condition in ('', FULL, DOCS, 'always()'):
            old = '        if: always() && ' + FULL + '\n'
            changed = upload.replace(old, '        if: ' + condition + '\n' if condition else '', 1)
            with self.subTest(upload_condition=condition), self.assertRaises(ValueError):
                validate(self.text.replace(upload, changed, 1))

    def test_producer_jobs_are_parallel_exact_unconditional_scopes(self):
        validate(self.text)
        jobs = workflow_jobs(self.text)
        for name in ('observer-checks', 'native-execution', 'policy-native-checks'):
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
                    "    needs: [policy-native-checks]\n",
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
        for name in JOB_ORDER:
            with self.subTest(missing=name), self.assertRaises(ValueError):
                validate(self.text.replace(jobs[name], '', 1))
            with self.subTest(duplicate=name), self.assertRaises(ValueError):
                validate(self.text.replace(jobs[name], jobs[name] + jobs[name], 1))
        for left, right in zip(JOB_ORDER, JOB_ORDER[1:]):
            adjacent = jobs[left] + jobs[right]
            self.assertIn(adjacent, self.text)
            changed = self.text.replace(adjacent, jobs[right] + jobs[left], 1)
            self.assertNotEqual(changed, self.text)
            with self.subTest(reordered=(left, right)), self.assertRaises(ValueError):
                validate(changed)
        unknown = '  unreviewed-success:\n    runs-on: ubuntu-latest\n    steps:\n      - run: true\n'
        with self.assertRaises(ValueError):
            validate(self.text.replace('  browser-demo:\n', unknown + '  browser-demo:\n', 1))
        for name in ('observer-checks', 'native-execution', 'policy-native-checks'):
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

    def test_direct_deployment_graph_and_status_cannot_skip_or_accept_failed_producers(self):
        validate(self.text)
        jobs = workflow_jobs(self.text)
        self.assertNotIn('native-ports', jobs)
        deploy = jobs['deploy-browser-demo']
        mutations = (
            ('    if: ' + DEPLOY_IF + '\n', ''),
            ('always()', 'success()'),
            ('!cancelled() && ', ''),
            ("github.event_name == 'push' && ", ''),
            ("github.ref == 'refs/heads/main' && ", ''),
            (DEPLOY_NEEDS, '    needs: [native-execution, browser-demo]'),
            (DEPLOY_NEEDS, '    needs: [observer-checks, browser-demo]'),
            (DEPLOY_NEEDS, '    needs: [observer-checks, native-execution]'),
            (DEPLOY_NEEDS, '    needs: [observer-checks, native-execution, native-ports]'),
            (DEPLOY_NEEDS, '    needs: [observer-checks, native-execution, native-execution]'),
            (DEPLOY_NEEDS, '    needs: [observer-checks, native-execution, browser-demo, deploy-browser-demo]'),
            (DEPLOY_NEEDS, '    needs: [observer-checks, native-execution, browser-demo]'),
            (DEPLOY_NEEDS, '    needs: [observer-checks, native-execution, policy-native-checks, policy-native-checks]'),
            ("needs.observer-checks.result == 'success'", "needs.observer-checks.result != 'failure'"),
            ("needs.native-execution.result == 'success'", "needs.native-execution.result != 'failure'"),
            ("needs.policy-native-checks.result == 'success'", "needs.policy-native-checks.result != 'failure'"),
            ("needs.browser-demo.result == 'success'", "needs.browser-demo.result != 'failure'"),
            (" && needs.observer-checks.result == 'success'", ''),
            (" && needs.native-execution.result == 'success'", ''),
            (" && needs.policy-native-checks.result == 'success'", ''),
            (" && needs.browser-demo.result == 'success'", ''),
            (' && ', ' || '),
            ('    runs-on:', '    continue-on-error: true\n    runs-on:'),
            ('      pages: write', '      contents: write\n      pages: write'),
            ('    steps:\n', '    steps:\n      - run: true\n'),
        )
        for before, after in mutations:
            self.assertIn(before, deploy)
            changed = self.text.replace(deploy, deploy.replace(before, after, 1), 1)
            self.assertNotEqual(changed, self.text)
            with self.subTest(before=before, after=after), self.assertRaises(ValueError):
                validate(changed)
        legacy = ('  native-ports:\n    runs-on: ubuntu-latest\n    steps:\n'
                  '      - name: Misleading success\n        run: true\n\n')
        with self.assertRaises(ValueError):
            validate(self.text.replace('  browser-demo:\n', legacy + '  browser-demo:\n', 1))
        for name in ('observer-checks', 'native-execution', 'policy-native-checks', 'browser-demo'):
            changed = self.text.replace('  ' + name + ':\n', '  native-ports:\n', 1)
            self.assertNotEqual(changed, self.text)
            with self.subTest(rename_only=name), self.assertRaises(ValueError):
                validate(changed)

    def test_source_derived_deployment_conjunction_all125_producer_outcomes(self):
        # Exercise a narrow translation of the actual workflow expression.
        # This is not an execution of GitHub's status-expression interpreter.
        # Retain the original125 outcome submatrix and extend it to all625
        # combinations after adding the fourth required producer.
        validate(self.text)
        expression = deployment_expression(self.text)
        script = deployment_condition_shell(expression)
        states = ('success', 'failure', 'cancelled', 'skipped', '')
        base = {'PATH': '/usr/bin:/bin', 'WORKFLOW_CANCELLED': 'false',
                'EVENT_NAME': 'push', 'GITHUB_REF': 'refs/heads/main'}

        def outcome(environment):
            result = subprocess.run(['/bin/bash', '--noprofile', '--norc', '-e', '-o', 'pipefail', '-c', script],
                                    env=environment, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                                    stderr=subprocess.PIPE, timeout=5, check=False)
            self.assertEqual(result.stdout, b'')
            return result.returncode == 0

        for observer in states:
            for native in states:
                for policy in states:
                    for browser in states:
                        environment = dict(base, OBSERVER_RESULT=observer, NATIVE_RESULT=native,
                                           POLICY_RESULT=policy, BROWSER_RESULT=browser)
                        with self.subTest(observer=observer, native=native, policy=policy, browser=browser):
                            self.assertEqual(outcome(environment), observer == native == policy == browser == 'success')
        success = dict(base, OBSERVER_RESULT='success', NATIVE_RESULT='success',
                       POLICY_RESULT='success', BROWSER_RESULT='success')
        for key, value in (('WORKFLOW_CANCELLED', 'true'), ('EVENT_NAME', 'pull_request'),
                           ('GITHUB_REF', 'refs/tags/v1'), ('GITHUB_REF', 'refs/heads/other')):
            with self.subTest(context=(key, value)):
                self.assertFalse(outcome(dict(success, **{key: value})))
        for missing in ('OBSERVER_RESULT', 'NATIVE_RESULT', 'POLICY_RESULT', 'BROWSER_RESULT'):
            environment = dict(success); del environment[missing]
            with self.subTest(missing=missing):
                self.assertFalse(outcome(environment))



if __name__ == "__main__":
    unittest.main()
