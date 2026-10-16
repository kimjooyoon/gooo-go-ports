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
import unittest
from pathlib import Path


WORKFLOW = Path(__file__).resolve().parent.parent / ".github/workflows/ci.yml"
FULL = "steps.change-scope.outputs.docs_only != 'true'"
PREFLIGHT_STEP_NAME = 'Verify lightweight API policy summary controls before native work'
PREFLIGHT_STEP_SHA = '6e31886b6e7f6f6edcb6681b651bf985c5388c35141dc1b01d4310f298045f93'
PREFLIGHT = {PREFLIGHT_STEP_NAME}
DOCS = "steps.change-scope.outputs.docs_only == 'true'"
HEADER_SHA = "5cd59615e4403c288bd003ae603c7e55a813f42af0bbbb4715effebf8d093e9b"
# The observer/browser producers and deployment body retain their reviewed bytes.
# Native alone uses the explicit workspace profile and a pinned Ubuntu 24.04 runner.
BROWSER_JOB_SHA = "1aee9cc0cd8f75b2e4a438712aaf87bc8e1007a1318cb997ede369b6f6ce4e10"
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
NATIVE_COMPARISON_COMMAND = "go run -mod=readonly ./cmd/verify --workers 2 --case-profile uuid-and-literal-report-compact-v1 --workspace-profile dependency-closure-v1 --out out/ci --timing-out out/ci/timing.jsonl"
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
TIMING_HEADER_ASSERTION = '''          head -n 1 out/ci/timing.jsonl | jq -e --slurpfile summary out/ci/summary.json '
            . as $header |
            .schema == "gooo/native-verification-timing/v1" and .kind == "header" and .seq == 0 and
            .case_profile == $summary[0].case_profile and
            .workspace_profile == $summary[0].workspace_profile and
            .compiler_revision == "83928fa4538245abe33b08549da47f8cbdc50ed2" and .go_version == "go1.27.2" and
            .configured_workers == 2 and .expected_suites == 79 and .expected_cases == 173582 and
            .expected_boundaries_recomputed == false and
            .boundary_scope == "Actual contiguous ranges only. UUID3 retain uuid-compact-v1 wire/partitions; literal-text Report uses a new compact writer/partition identity. No prior profile PASS or full batch total is inherited. Index/Count and other existing non-UUID suites retain their indented writer; decimal/payload retain their source-bounded pretty JSON policy. Fresh native/saved replay and the full summary gate remain required; no timing or speedup claim." and
            (.identity.source_files | length) == 251 and
            (.identity.source_files | keys) == [".github/workflows/ci.yml", "CONTRIBUTING.md", "NOTICE", "README.md", "canonical-header-key-report.workspace.json", "canonical-header-key.workspace.json", "cmd/build-demo/api_policy.go", "cmd/build-demo/api_policy_test.go", "cmd/build-demo/main.go", "cmd/header-addition-input/main.go", "cmd/header-addition-input/main_test.go", "cmd/header-facts-input/main.go", "cmd/header-facts-input/main_test.go", "cmd/verify/canonical_header_batches.go", "cmd/verify/canonical_header_batches_test.go", "cmd/verify/canonical_header_contracts.go", "cmd/verify/canonical_header_contracts_test.go", "cmd/verify/canonical_header_corpus.go", "cmd/verify/canonical_header_corpus_test.go", "cmd/verify/case_exhaustive_test.go", "cmd/verify/case_profile.go", "cmd/verify/case_profile_integration_test.go", "cmd/verify/case_profile_test.go", "cmd/verify/contracts.go", "cmd/verify/contracts_test.go", "cmd/verify/decimal_payload_batches.go", "cmd/verify/decimal_payload_batches_test.go", "cmd/verify/decimal_payload_contracts_test.go", "cmd/verify/decimal_payload_corpus.go", "cmd/verify/decimal_payload_corpus_test.go", "cmd/verify/etag_batches.go", "cmd/verify/etag_batches_test.go", "cmd/verify/etag_contracts.go", "cmd/verify/etag_contracts_test.go", "cmd/verify/etag_corpus.go", "cmd/verify/etag_corpus_test.go", "cmd/verify/etag_native_test.go", "cmd/verify/etag_reference.go", "cmd/verify/external_oracles.go", "cmd/verify/external_oracles_test.go", "cmd/verify/header_addition_batches.go", "cmd/verify/header_addition_batches_test.go", "cmd/verify/header_addition_contracts.go", "cmd/verify/header_addition_contracts_test.go", "cmd/verify/header_addition_corpus.go", "cmd/verify/header_addition_corpus_test.go", "cmd/verify/header_contracts.go", "cmd/verify/header_contracts_test.go", "cmd/verify/header_corpus.go", "cmd/verify/header_corpus_test.go", "cmd/verify/inspect.go", "cmd/verify/legacy_workspace_test.go", "cmd/verify/literal_cut_contracts.go", "cmd/verify/literal_cut_corpus.go", "cmd/verify/literal_cut_corpus_test.go", "cmd/verify/literal_cut_integration_test.go", "cmd/verify/literal_report_case_profile_test.go", "cmd/verify/main.go", "cmd/verify/native_build_cache.go", "cmd/verify/native_build_cache_test.go", "cmd/verify/native_correctness_cache_test.go", "cmd/verify/native_suite.go", "cmd/verify/native_timing.go", "cmd/verify/path_alias_batches.go", "cmd/verify/path_alias_batches_test.go", "cmd/verify/path_alias_contracts.go", "cmd/verify/path_alias_contracts_test.go", "cmd/verify/path_alias_corpus.go", "cmd/verify/path_alias_corpus_test.go", "cmd/verify/simple_fold_exhaustive_test.go", "cmd/verify/string_byte_batches.go", "cmd/verify/suite_workers.go", "cmd/verify/suite_workers_test.go", "cmd/verify/testdata/literal-cut-original62.json", "cmd/verify/whitespace_contracts.go", "cmd/verify/workspace_catalog.go", "cmd/verify/workspace_catalog_test.go", "cmd/verify/workspace_profile.go", "cmd/verify/workspace_profile_test.go", "cmd/verify/ycbcr_exhaustive_test.go", "decimal-int64.workspace.json", "diagnostic-header-addition.workspace.json", "etag-scan.workspace.json", "etag-strong-match.workspace.json", "examples/agentmesh-service-policy/checked_impact.gooo", "examples/agentmesh-service-policy/checked_matcher.gooo", "examples/agentmesh-service-policy/checked_refinement.gooo", "examples/agentmesh-service-policy/conditional_apply.gooo", "examples/agentmesh-service-policy/domain.gooo", "examples/agentmesh-service-policy/impact.gooo", "examples/agentmesh-service-policy/refinement.gooo", "examples/alpha-matte/main.gooo", "examples/byte-rotation/main.gooo", "examples/byte-summary/main.gooo", "examples/canonical-header-key/README.md", "examples/canonical-header-key/contract.json", "examples/canonical-header-key/corpus-accounting.json", "examples/canonical-header-key/corpus.json", "examples/canonical-header-key/inputs.json", "examples/canonical-header-key/main.gooo", "examples/canonical-header-key/manual20.json", "examples/canonical-header-key/port-inputs.json", "examples/case-inspector/main.gooo", "examples/cel-api-policy/main.gooo", "examples/color-inspector/main.gooo", "examples/crop-planner/crop_planner.gooo", "examples/crop-planner/drag_crop.gooo", "examples/diagnostic-header-addition/README.md", "examples/diagnostic-header-addition/contract.json", "examples/diagnostic-header-addition/corpus.json", "examples/diagnostic-header-addition/inputs.json", "examples/diagnostic-header-addition/main.gooo", "examples/diagnostic-header-addition/request.json", "examples/duration-quantizer/main.gooo", "examples/fits-crop/main.gooo", "examples/fixed-pixel-bounds/main.gooo", "examples/header-addition-host/LICENSE", "examples/header-addition-host/NOTICE", "examples/header-addition-host/README.md", "examples/header-addition-host/bindings.go.txt", "examples/header-addition-host/bindings_test.go.txt", "examples/header-addition-host/go.mod.txt", "examples/header-addition-host/host.go.txt", "examples/header-addition-host/host_test.go.txt", "examples/header-addition-host/main.go.txt", "examples/header-addition-host/provenance.json", "examples/header-field-facts/README.md", "examples/header-field-facts/contract.json", "examples/header-field-facts/corpus.json", "examples/header-field-facts/inputs.json", "examples/header-field-facts/main.gooo", "examples/header-field-facts/name-inputs.json", "examples/header-field-facts/request.json", "examples/header-field-facts/unicode-request.json", "examples/header-field-facts/valid-request.json", "examples/header-field-facts/value-inputs.json", "examples/identifier-rune/main.gooo", "examples/literal-cut-report/README.md", "examples/literal-cut-report/inputs.json", "examples/literal-cut-report/main.gooo", "examples/literal-cut-report/manual-contract.json", "examples/literal-text-report/main.gooo", "examples/path-alias-facts/README.md", "examples/path-alias-facts/anchor-pairs-24.json", "examples/path-alias-facts/clean-inputs.json", "examples/path-alias-facts/contract.json", "examples/path-alias-facts/corpus-120.json", "examples/path-alias-facts/inputs.json", "examples/path-alias-facts/main.gooo", "examples/payload-admission/README.md", "examples/payload-admission/contract.json", "examples/payload-admission/corpus.json", "examples/payload-admission/inputs.json", "examples/payload-admission/main.gooo", "examples/payload-admission/manual17.json", "examples/response.gooo", "examples/simple-fold/main.gooo", "examples/string-byte-search/main.gooo", "examples/strong-validator-facts/README.md", "examples/strong-validator-facts/contract.json", "examples/strong-validator-facts/corpus.json", "examples/strong-validator-facts/inputs.json", "examples/strong-validator-facts/main.gooo", "examples/strong-validator-facts/manual247.json", "examples/unicode-inspector/main.gooo", "examples/uuid-format-inspector/main.gooo", "examples/whitespace-inspector/main.gooo", "examples/ycbcr-inspector/main.gooo", "go.mod", "go.sum", "header-field-facts.workspace.json", "header-field-name.workspace.json", "header-field-value.workspace.json", "internal/headeradditioninput/preflight.go", "internal/headeradditioninput/preflight_test.go", "internal/headerinput/preflight.go", "internal/headerinput/preflight_test.go", "literal-cut-report.workspace.json", "path-alias-facts.workspace.json", "path-clean.workspace.json", "payload-admission.workspace.json", "ports/agentmesh/service_pattern.gooo", "ports/cel/scalar_access.gooo", "ports/google/uuid/quote.gooo", "ports/google/uuid/text/parse_canonical.gooo", "ports/google/uuid/validate.gooo", "ports/image/canon.gooo", "ports/image/color.gooo", "ports/image/fixed.gooo", "ports/image/geometry.gooo", "ports/image/nrgba.gooo", "ports/image/ycbcr.gooo", "ports/math/bits.gooo", "ports/net/http.gooo", "ports/net/http/README.md", "ports/net/http/etag.gooo", "ports/net/http/provenance.json", "ports/net/httpguts/README.md", "ports/net/httpguts/headers.gooo", "ports/net/httpguts/provenance.json", "ports/net/textproto/README.md", "ports/net/textproto/canonical.gooo", "ports/net/textproto/provenance.json", "ports/path/README.md", "ports/path/clean-corpus-9841.json", "ports/path/clean.gooo", "ports/path/path.gooo", "ports/path/provenance.json", "ports/strconv/README.md", "ports/strconv/corpus-256.json", "ports/strconv/decimal.gooo", "ports/strconv/inputs.json", "ports/strconv/provenance.json", "ports/strings/cut.gooo", "ports/strings/literal.gooo", "ports/strings/search.gooo", "ports/time/duration.gooo", "ports/unicode/case_mapping.gooo", "ports/unicode/classification.gooo", "ports/unicode/simple_fold.gooo", "ports/unicode/space.gooo", "ports/unicode/utf16.gooo", "ports/unicode/utf8.gooo", "site/api-policy-worker.js", "site/api-policy.css", "site/api-policy.html", "site/api-policy.js", "strong-validator-facts.workspace.json", "third_party/GO-LICENSE", "third_party/go/net/http/fs.go.txt", "third_party/go/net/http/fs_test.go.txt", "third_party/go/net/textproto/reader.go.txt", "third_party/go/net/textproto/textproto.go.txt", "third_party/x-net/LICENSE", "third_party/x-net/http/httpguts/guts.go.txt", "third_party/x-net/http/httpguts/httplex.go.txt", "third_party/x-net/http/httpguts/httplex_test.go.txt", "tools/api-policy-app.test.cjs", "tools/api-policy-browser-bridge.go.txt", "tools/api-policy-browser-smoke.mjs", "tools/check-ci-scope-workflow_test.py", "tools/check-header-addition-host.py", "tools/check-header-addition-host_test.py", "tools/check-header-addition-inputs.py", "tools/check-header-facts-inputs.py", "tools/check-literal-text-inputs.py", "tools/check-literal-text-inputs_test.py", "tools/classify-ci-scope.py", "tools/generate-payload-corpus.py", "upstream.json", "verification-catalog.json"] and
            all($header.identity.source_files[];
              (.bytes | type) == "number" and .bytes > 0 and
              (.sha256 | test("^[0-9a-f]{64}$")))
          '
'''
TIMING_CLAUSES = (
    '.schema == "gooo/native-verification-timing/v1"', '.kind == "run_end"',
    '.status == "COMPLETE"',
    '.complete == true and .aborted == false and .successful_close_return_required == true',
    '.seq == 2406', '.expected_suites == 79', '.successful_suites == 79',
    '.observed_batches == 2247', '.verified_batches == 2247', '.verified_cases == 173582',
    '.event_count == 2407', '.failed_suites == 0', '.canceled_suites == 0',
    '.not_started_suites == 0', '.missing_suite_finishes == 0',
    '.failed_batches == 0', '.incomplete_batches == 0', '.logger_errors == 0',
    '.source_bindings_unchanged == true',
)
TIMING_ASSERTION = ("          tail -n 1 out/ci/timing.jsonl | jq -e '\n"
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
HEADER_ADDITION_HOST_STEP_SHA = '2554053bd3d773090cdf96f73cb7d6ab8b3e89ca7362ce688634f1733aa2e101'
HEADER_ADDITION_CLAUSES = ('([.runs[:73][].cases] | add) == 171167 and ([.runs[:73][].batches] | add) == 2174 and', '.header_addition_reference_cases == 50 and .header_addition_fixed_batch_rows == 8 and', '.header_addition_record_types == 5 and .header_addition_pure_activities == 6 and .header_addition_callee_edges == 5 and', '.runs[73].package == "app/diagnostic-header-addition" and .runs[73].activity == "AssessAddition" and', '.runs[73].cases == 50 and .runs[73].passed == 50 and .runs[73].batches == 7 and', '.source_sha256["examples/diagnostic-header-addition/main.gooo"] == "cd499d4bda6b40937882c77d67774b573c84aa93ec407e9a95b82f755d41eae8" and', '.header_addition_scope == "Original deterministic12-field RequestFacts plus three bounded Text inputs return16 typed header-addition decision fields. Actual pinned x/net/httpguts and strconv references supply the50 finite expected results. No per-request human approval is required to compute; host authority/current binding and atomic final apply remain necessary." and', '.header_addition_provenance_scope == "Fresh native execution and saved replay bind five current sources, six pure activities, five record layouts, five callee edges, exact12 input/16 output fields, generated ABI and raw outer saved parents. Independent inner typed canonical prior_result_sha256 remains NOT_CHECKED; private pilot results are not fresh public acceptance." and')
HEADER_WIRE_STEP_NAME = 'Verify strict header input reaches the actual native CLI'
HEADER_WIRE_COMMAND = 'python3 -I -B tools/check-header-facts-inputs.py --root "$PWD" --compiler "$PWD/.gooo" --input-command "$PWD/out/header-facts-input" --go "$(go env GOROOT)/bin/go" --out "$PWD/out/header-facts-input-route"'
HEADER_WIRE_STEP_SHA = '04e719ee2669cf0cdde795740db470eba1a661bc98c7f02d3e1d6b98dfbb8d19'
HEADER_MODULE_PREFLIGHT = "          go mod download golang.org/x/net golang.org/x/text\n          go mod verify\n"
HEADER_SOURCE_SET_ASSERTION = "          jq -e -s --slurpfile summary out/ci/summary.json '\n            . as $rows | $rows[0] as $header |\n            ([$rows[] | select(.kind == \"suite_end\")] | sort_by(.suite_index)) as $suites |\n            [$rows[] | select(.kind == \"batch_end\")] as $batches |\n            ($suites | length) == 79 and ($batches | length) == 2247 and\n            [$suites[].suite_index] == [range(0;79)] and\n            all(range(0;79); . as $i | $suites[$i] as $s | $summary[0].runs[$i] as $r |\n              $s.package == $r.package and $s.activity == $r.activity and\n              $s.status == \"VERIFIED\" and $s.covered_cases == $r.cases and\n              $s.observed_batches == $r.batches and\n              ($s.source_set | type) == \"object\" and ($s.source_set | length) > 0 and\n              ($s.source_set_sha256 | test(\"^sha256:[0-9a-f]{64}$\")) and\n              all($s.source_set | to_entries[]; .key as $path |\n                (.value.Package | type) == \"string\" and (.value.Package | length) > 0 and\n                (.value.Digest | test(\"^sha256:[0-9a-f]{64}$\")) and\n                $header.identity.source_files[$path].sha256 == $header.catalog_source_sha256[$path] and\n                $header.catalog_source_sha256[$path] == $summary[0].source_sha256[$path])) and\n            all($batches[]; . as $b | $b.suite_index >= 0 and $b.suite_index < 79 and\n              $b.source_set_sha256 == $suites[$b.suite_index].source_set_sha256) and\n            ($suites[70].source_set | keys) == [\"ports/net/httpguts/headers.gooo\"] and\n            ($suites[71].source_set | keys) == [\"ports/net/httpguts/headers.gooo\"] and\n            ($suites[72].source_set | keys) == [\"examples/header-field-facts/main.gooo\", \"ports/net/httpguts/headers.gooo\"] and\n            ($suites[73].source_set | keys) == [\"examples/diagnostic-header-addition/main.gooo\", \"examples/header-field-facts/main.gooo\", \"examples/payload-admission/main.gooo\", \"ports/net/httpguts/headers.gooo\", \"ports/strconv/decimal.gooo\"] and\n            ($suites[74].source_set | keys) == [\"ports/net/httpguts/headers.gooo\", \"ports/net/textproto/canonical.gooo\"] and\n            ($suites[75].source_set | keys) == [\"examples/canonical-header-key/main.gooo\", \"examples/header-field-facts/main.gooo\", \"ports/net/httpguts/headers.gooo\", \"ports/net/textproto/canonical.gooo\"] and\n            ($suites[76].source_set | keys) == [\"ports/net/http/etag.gooo\"] and\n            ($suites[77].source_set | keys) == [\"ports/net/http/etag.gooo\"] and\n            ($suites[78].source_set | keys) == [\"examples/strong-validator-facts/main.gooo\", \"ports/net/http/etag.gooo\", \"ports/net/httpguts/headers.gooo\"]\n          ' out/ci/timing.jsonl\n"
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
    '(.source_sha256 | length) == 63 and',
    '.source_sha256["ports/path/path.gooo"] == "593caad2ac3b443d353775a9f4e168282af1ab70181bcbc483c4b379c9ee3de3" and',
    '.source_sha256["ports/path/clean.gooo"] == "8430fc094b000fbd25b9d081408a9e8cdb59fca942f7a5541e7695c0291b790d" and',
    '.source_sha256["examples/path-alias-facts/main.gooo"] == "5b0c8436bf95c923730af71a316d3897839d734c53c0223fffad5b555488182e" and',
    '.path_alias_scope == "Decoded valid UTF-8 Text up to1024 bytes, slash-based lexical Go path.Clean. Original11-field Report preserves both original inputs and exposes clean/absolute/change/equality/alias facts. Actual Go path.Clean and path.IsAbs supply independent finite expectations; no URL, router, filesystem, authorization or safety claim." and',
    '.path_alias_provenance_scope == "Fresh native execution and saved replay check original Texts, exact result/field identities and order, selected current source bytes, generated bodies, producer identities and raw outer receipt parents. This generic path does not independently check inner prior_result_sha256 or general callee ABI; private receipts are not fresh public acceptance." and',
)
COMMON = {
    "Classify the exact checked PR tree", "Check the Go observer",
    "Check pinned Unicode table generation",
    "Build and identify the pinned compiler from a clean checkout",
    "Check Gooo declarations",
}
HEAVY = {
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
    r"tools/probe\.sh|tools/check-(?:fits-crop|literal-text)-inputs\.py|decision-policy-pilot/run\.sh|cel-api-policy/verify\.py|agentmesh-service-policy/verify\.py|api-policy-revision/verify\.py|tools/check-api-policy-public-consumer\.py|tools/check-path-facts\.py|tools/check-redelivery-policy\.py|tools/check-ipv4\.py|tools/check-header-facts-inputs\.py|tools/check-header-addition-inputs\.py|tools/check-header-addition-host\.py)"
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
NATIVE_HEADER_SHA = '584d7a22663deb57be052a69d58f2b19e6233a7eefb7d4ce6ad1d5b650d48e69'
# Add the record adapter controls, corpus regeneration and host-reader controls;
# every previously required observer command remains byte-identical and once.
OBSERVER_STEP_SHA = '9752b431a2e7102064c3dd42ce56d1ee300170963cd3b16d2e5ed9d4d884f31c'
GENERATOR_STEP_SHA = '081263b551a4d28aa8b46dc025fe951c62d91d13e164cb7927b70d7270aaf399'
CLASSIFIER_STEP_SHA = '1864b9e0f27cc73595dd863c71640050cf046b2ec67cfd02c6ba5a8ebd11a61a'
CHECKOUT_STEP_SHA = 'b60ba5d7bf95691b8baec3a3e402aeba552fbcb4a353d7de8946023e73e79603'
SETUP_STEP_SHA = '742468cc6b855d4f2c2d9a5d45000899bc244f524539bad21505ef94018dceb2'
MOVED = {'Check the Go observer', 'Check pinned Unicode table generation'}
JOB_ORDER = ('observer-checks', 'native-execution', 'browser-demo', 'deploy-browser-demo')
DEPLOY_IF = "${{ always() && !cancelled() && github.event_name == 'push' && github.ref == 'refs/heads/main' && needs.observer-checks.result == 'success' && needs.native-execution.result == 'success' && needs.browser-demo.result == 'success' }}"
DEPLOY_NEEDS = '    needs: [observer-checks, native-execution, browser-demo]'


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
        expected = {'Classify the exact checked PR tree'} | MOVED if job == 'observer-checks' else (COMMON | HEAVY | PREFLIGHT | UPLOADS | DOC_STEPS) - MOVED
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


def deployment_expression(text):
    deploy = workflow_jobs(text)['deploy-browser-demo']
    conditions = re.findall(r"^    if: (.+)$", deploy, re.M)
    dependencies = re.findall(r"^    needs: (.+)$", deploy, re.M)
    require(conditions == [DEPLOY_IF], "deployment must explicitly require all three successes on uncanceled main pushes")
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
    require(len(HEAVY) == 31, "full-scope native step count changed")
    require(set(named) == COMMON | HEAVY | PREFLIGHT | UPLOADS | DOC_STEPS, "step roster changed without guard review")
    require(len(unnamed) == 2 and all(field(c, "if") is None for c in unnamed), "checkout/Go setup became conditional")
    require("fetch-depth: 0" in unnamed[0], "exact PR parents must be available")
    require("go-version: '1.27.2'" in unnamed[1], "Go toolchain pin changed")
    public = named["Verify the public API policy consumer"]
    native_job = workflow_jobs(text)['native-execution']
    preflight = named[PREFLIGHT_STEP_NAME]
    require(hashlib.sha256(preflight.encode()).hexdigest() == PREFLIGHT_STEP_SHA,
            "lightweight summary controls, source buffers, limits or first-failure evidence changed")
    require(native_job.index(named['Classify the exact checked PR tree']) < native_job.index(preflight) <
            native_job.index(unnamed[1]), "summary preflight must precede Go setup and all native work")
    require(native_job.index(unnamed[1]) < native_job.index(public) <
            native_job.index(named["Build and identify the pinned compiler from a clean checkout"]),
            "public consumer must run after native Go setup and before native compiler/execution")
    require("python3 -B tools/check-api-policy-public-consumer_test.py -v" in named["Check the Go observer"],
            "bounded public-consumer diagnostic checks must remain in the common Go observer step")
    for name in COMMON:
        require(field(named[name], "if") is None, "common check became conditional: " + name)
    for name in HEAVY | PREFLIGHT:
        require(field(named[name], "if") == FULL, "missing full-execution guard: " + name)
    for name in UPLOADS:
        require(field(named[name], "if") == "always() && " + FULL, "native artifact can upload on docs path: " + name)
    comparison = named["Compare the ports with Go and replay every batch"]
    require(literal_commands(comparison) == [NATIVE_COMPARISON_COMMAND]
            and ".native_suite_workers == 2 and" in comparison,
            "full native CI must bind two workers and source-bound timing")
    require(comparison.count(CASE_PROFILE_ASSERTION) == 1,
            "native CI must require the exact explicit UUID compact profile identity")
    require(comparison.count(WORKSPACE_PROFILE_ASSERTION) == 1,
            "native CI must require the exact explicit dependency closure workspace profile identity")
    require(comparison.count(TIMING_HEADER_ASSERTION) == 1,
            "native timing must bind the same profile and complete source identity")
    require(comparison.count(TIMING_ASSERTION) == 1,
            "native timing must require the exact complete footer and full observed roster")
    summaries = re.findall(r"^          jq -c -s '\n.*?^          ' out/ci/timing.jsonl\n", comparison, re.M | re.S)
    require(len(summaries) == 1 and hashlib.sha256(summaries[0].encode()).hexdigest() == TIMING_SUMMARY_SHA,
            "native timing console summary must retain its exact source and actual input")
    require("continue-on-error:" not in comparison and "|| true" not in comparison,
            "native comparison or timing failure became optional")
    for token in (".ported_functions == 52 and .consumer_programs == 25 and .consumer_activities == 27 and",
                  "(.runs | length) == 79 and",
                  ".cases == 173582 and .passed == 173582 and",
                  "([.runs[].cases] | add) == 173582 and ([.runs[].batches] | add) == 2247 and",
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
            "native timing must bind all79 exact source maps and summary suite identities")
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
            native_job.index(named["Verify the isolated native path facts sidecar"]),
            "actual header wire route must run after declarations and before other native sidecars")
    addition_input, addition_host = named[HEADER_ADDITION_INPUT_STEP_NAME], named[HEADER_ADDITION_HOST_STEP_NAME]
    require(hashlib.sha256(addition_input.encode()).hexdigest() == HEADER_ADDITION_INPUT_STEP_SHA
            and literal_commands(addition_input) == [HEADER_ADDITION_INPUT_COMMAND]
            and text.count(HEADER_ADDITION_INPUT_COMMAND) == 1, "addition input route must retain exact helper build/fresh timing/argv/128 typed-field acceptance once")
    require(hashlib.sha256(addition_host.encode()).hexdigest() == HEADER_ADDITION_HOST_STEP_SHA
            and literal_commands(addition_host) == [HEADER_ADDITION_HOST_COMMAND]
            and text.count(HEADER_ADDITION_HOST_COMMAND) == 1
            and named["Check the Go observer"].count(HEADER_ADDITION_HOST_UNIT) == text.count(HEADER_ADDITION_HOST_UNIT) == 1, "addition host must retain exact fresh79 receipt/timing/ABI50/test/atomic acceptance once")
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
    for source in IPV4_DECLARATIONS:
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
    require("Allowlisted static-document content-only PR" in log
            and "No new native-port execution/replay receipts" in log
            and "browser-demo job retains its separate" in log, "misleading execution scope log")


class WorkflowScopeTests(unittest.TestCase):
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

    def test_full_native_ci_binds_two_workers_and_timing(self):
        for workers in (1, 3, 4, 5):
            for before, after in (("--workers 2 --case-profile", f"--workers {workers} --case-profile"),
                                  (".native_suite_workers == 2 and", f".native_suite_workers == {workers} and")):
                with self.subTest(workers=workers, before=before), self.assertRaises(ValueError):
                    validate(self.text.replace(before, after, 1))

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
            chunk.replace("(.identity.source_files | length) == 251", "(.identity.source_files | length) >= 97", 1),
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
                ("(.identity.source_files | length) == 251", "(.identity.source_files | length) == 140"),
                ("(.identity.source_files | length) == 251", "(.identity.source_files | length) >= 140"),
                ("(.identity.source_files | keys) ==", "(.identity.source_files | values) =="),
                ("all($header.identity.source_files[];", "all($header.identity.source_files | del(.\"cmd/verify/workspace_profile.go\") | .[];"),
                ("(.bytes | type) == \"number\" and .bytes > 0", "(.bytes | type) == \"number\"")):
            self.assertIn(before, chunk)
            mutations.append(chunk.replace(before, after, 1))
        # Guard every old input as well as each new selector/test input. A count
        # alone would permit substituting a different source at the same length.
        for name in ("cmd/verify/workspace_profile.go", "cmd/verify/workspace_profile_test.go", "cmd/verify/main.go"):
            token = json.dumps(name) + ", "
            self.assertIn(token, TIMING_HEADER_ASSERTION)
            mutations.append(chunk.replace(TIMING_HEADER_ASSERTION,
                                           TIMING_HEADER_ASSERTION.replace(token, "", 1), 1))
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
        native = jobs["native-execution"]
        self.assertIn("    runs-on: ubuntu-24.04\n", native)
        self.assertIn("    runs-on: ubuntu-latest\n", jobs["observer-checks"])
        self.assertIn("    runs-on: ubuntu-latest\n", jobs["browser-demo"])
        for runner in ("ubuntu-latest", "ubuntu-26.04", "ubuntu-22.04"):
            changed = native.replace("runs-on: ubuntu-24.04", "runs-on: " + runner, 1)
            self.assertNotEqual(changed, native)
            with self.subTest(runner=runner), self.assertRaises(ValueError):
                validate(self.text.replace(native, changed, 1))



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
        # Old76 acceptance counts cannot replace the current79 requirements.
        for before, after in (
                ('(.runs | length) == 79', '(.runs | length) == 76'),
                ('.cases == 173582 and .passed == 173582', '.cases == 173335 and .passed == 173335'),
                ('([.runs[].batches] | add) == 2247', '([.runs[].batches] | add) == 2240'),
                ('.expected_suites == 79', '.expected_suites == 76'),
                ('.successful_suites == 79', '.successful_suites == 76'),
                ('.expected_cases == 173582', '.expected_cases == 173335'),
                ('.verified_cases == 173582', '.verified_cases == 173335'),
                ('.observed_batches == 2247', '.observed_batches == 2240'),
                ('.verified_batches == 2247', '.verified_batches == 2240'),
                ('.event_count == 2407', '.event_count == 2394'),
                ('.seq == 2406', '.seq == 2393'),
                ('(.identity.source_files | length) == 251', '(.identity.source_files | length) == 228'),
                ('(.source_sha256 | length) == 63', '(.source_sha256 | length) == 61')):
            self.assertIn(before, comparison)
            with self.subTest(rollback=before), self.assertRaises(ValueError):
                validate(self.text.replace(comparison, comparison.replace(before, after, 1), 1))
        for name in ETAG_NEW_SOURCE_PATHS:
            token = json.dumps(name) + ", "
            self.assertIn(token, TIMING_HEADER_ASSERTION)
            weakened = TIMING_HEADER_ASSERTION.replace(token, "", 1)
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
                ('FRESH79_BOUND_ABI50_AND_ATOMIC_HOST_PASS', 'FRESH76_BOUND_ABI50_AND_ATOMIC_HOST_PASS'),
                ('.fresh_native_suites == 79', '.fresh_native_suites == 76')):
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
                ('(.runs | length) == 79', '(.runs | length) == 74'),
                ('.cases == 173582 and .passed == 173582', '.cases == 171217 and .passed == 171217'),
                ('([.runs[].batches] | add) == 2247', '([.runs[].batches] | add) == 2181'),
                ('.expected_suites == 79', '.expected_suites == 74'),
                ('.successful_suites == 79', '.successful_suites == 74'),
                ('.expected_cases == 173582', '.expected_cases == 171217'),
                ('.verified_cases == 173582', '.verified_cases == 171217'),
                ('.observed_batches == 2247', '.observed_batches == 2181'),
                ('.verified_batches == 2247', '.verified_batches == 2181'),
                ('.event_count == 2407', '.event_count == 2331'),
                ('.seq == 2406', '.seq == 2330'),
                ('(.identity.source_files | length) == 251', '(.identity.source_files | length) == 207'),
                ('(.source_sha256 | length) == 63', '(.source_sha256 | length) == 59')):
            self.assertIn(before, comparison)
            with self.subTest(before=before), self.assertRaises(ValueError):
                validate(self.text.replace(comparison, comparison.replace(before, after, 1), 1))
        for name in CANONICAL_NEW_SOURCE_PATHS:
            token = json.dumps(name) + ", "
            self.assertIn(token, TIMING_HEADER_ASSERTION)
            weakened = TIMING_HEADER_ASSERTION.replace(token, "", 1)
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
                ('FRESH79_BOUND_ABI50_AND_ATOMIC_HOST_PASS', 'FRESH74_BOUND_ABI50_AND_ATOMIC_HOST_PASS'),
                ('.fresh_native_suites == 79', '.fresh_native_suites == 74')):
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
                ('(.runs | length) == 79', '(.runs | length) == 73'),
                ('.cases == 173582 and .passed == 173582', '.cases == 171167 and .passed == 171167'),
                ('([.runs[].batches] | add) == 2247', '([.runs[].batches] | add) == 2174'),
                ('.expected_suites == 79', '.expected_suites == 73'),
                ('.successful_suites == 79', '.successful_suites == 73'),
                ('.verified_cases == 173582', '.verified_cases == 171167'),
                ('.observed_batches == 2247', '.observed_batches == 2174'),
                ('.verified_batches == 2247', '.verified_batches == 2174'),
                ('.event_count == 2407', '.event_count == 2322'),
                ('.seq == 2406', '.seq == 2321'),
                ('(.identity.source_files | length) == 251', '(.identity.source_files | length) == 177')):
            self.assertIn(before, comparison)
            with self.subTest(before=before), self.assertRaises(ValueError):
                validate(self.text.replace(comparison, comparison.replace(before, after, 1), 1))
        for name in HEADER_ADDITION_NEW_SOURCE_PATHS:
            token = json.dumps(name) + ", "
            self.assertIn(token, TIMING_HEADER_ASSERTION)
            weakened = TIMING_HEADER_ASSERTION.replace(token, "", 1)
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
                (HEADER_ADDITION_HOST_STEP_NAME, '.fresh_native_suites == 79', '.fresh_native_suites == 73'),
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
                ('.ported_functions == 52 and .consumer_programs == 25 and .consumer_activities == 27 and',
                 '.ported_functions == 47 and .consumer_programs == 21 and .consumer_activities == 23 and'),
                ('(.runs | length) == 79 and', '(.runs | length) == 70 and'),
                ('.cases == 173582 and .passed == 173582 and', '.cases == 170227 and .passed == 170227 and'),
                ('.expected_suites == 79', '.expected_suites == 70'),
                ('.successful_suites == 79', '.successful_suites == 70'),
                ('.expected_cases == 173582', '.expected_cases == 170227'),
                ('.verified_cases == 173582', '.verified_cases == 170227'),
                ('.observed_batches == 2247', '.observed_batches == 2153'),
                ('.verified_batches == 2247', '.verified_batches == 2153'),
                ('.event_count == 2407', '.event_count == 2295'),
                ('.seq == 2406', '.seq == 2294'),
                ('(.source_sha256 | length) == 63', '(.source_sha256 | length) == 56'),
                ('(.identity.source_files | length) == 251', '(.identity.source_files | length) == 142'),
                ('original12-field diagnostics', 'original11-field diagnostics'),
                ('exact typed12 fields', 'exact typed11 fields'),
                ('independent inner typed canonical parent digest NOT_CHECKED', 'independent inner typed canonical parent digest CHECKED')):
            self.assertIn(before, comparison)
            with self.subTest(before=before), self.assertRaises(ValueError):
                validate(self.text.replace(comparison, comparison.replace(before, after, 1), 1))
        # All251 identities are an exact roster; a count alone permits substitution.
        for name in ("cmd/header-facts-input/main.go", "cmd/verify/header_contracts.go",
                     "cmd/verify/header_contracts_test.go", "cmd/verify/legacy_workspace_test.go",
                     "cmd/verify/header_corpus.go",
                     "cmd/verify/header_corpus_test.go", "cmd/verify/external_oracles.go",
                     "cmd/verify/external_oracles_test.go", "go.mod", "go.sum",
                     "examples/header-field-facts/contract.json", "examples/header-field-facts/corpus.json",
                     "ports/net/httpguts/headers.gooo", "ports/net/httpguts/provenance.json",
                     "tools/check-header-facts-inputs.py"):
            token = json.dumps(name) + ", "
            self.assertIn(token, TIMING_HEADER_ASSERTION)
            weakened = TIMING_HEADER_ASSERTION.replace(token, "", 1)
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
                ('.expected_suites == 79', '.expected_suites == 68'),
                ('.successful_suites == 79', '.successful_suites == 68'),
                ('.verified_cases == 173582', '.verified_cases == 160266'),
                ('.observed_batches == 2247', '.observed_batches == 2072'),
                ('.verified_batches == 2247', '.verified_batches == 2072'),
                ('.event_count == 2407', '.event_count == 2210'),
                ('.seq == 2406', '.seq == 2209')):
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
            ("([.runs[].batches] | add) == 2247", "([.runs[].batches] | add) == 2072"),
            (".event_count == 2407", ".event_count == 2210"),
            (".seq == 2406", ".seq == 2209"),
            ("(.identity.source_files | length) == 251", "(.identity.source_files | length) == 120"),
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
            ".ported_functions == 52 and .consumer_programs == 25 and .consumer_activities == 27 and",
            "(.runs | length) == 79 and", ".cases == 173582 and .passed == 173582 and",
            "([.runs[].cases] | add) == 173582 and ([.runs[].batches] | add) == 2247 and",
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
                              (".runs[61].batches == 235", ".runs[61].batches == 137"),
                              ("are NOT_CHECKED", "are CHECKED")):
            self.assertIn(before, self.text)
            altered = self.text.replace(before, after, 1)
            self.assertNotEqual(self.text, altered)
            with self.subTest(before=before), self.assertRaises(ValueError):
                validate(altered)

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
            ("needs.observer-checks.result == 'success'", "needs.observer-checks.result != 'failure'"),
            ("needs.native-execution.result == 'success'", "needs.native-execution.result != 'failure'"),
            ("needs.browser-demo.result == 'success'", "needs.browser-demo.result != 'failure'"),
            (" && needs.observer-checks.result == 'success'", ''),
            (" && needs.native-execution.result == 'success'", ''),
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
        for name in ('observer-checks', 'native-execution', 'browser-demo'):
            changed = self.text.replace('  ' + name + ':\n', '  native-ports:\n', 1)
            self.assertNotEqual(changed, self.text)
            with self.subTest(rename_only=name), self.assertRaises(ValueError):
                validate(changed)

    def test_source_derived_deployment_conjunction_all125_producer_outcomes(self):
        # Exercise a narrow translation of the actual workflow expression.
        # This is not an execution of GitHub's status-expression interpreter.
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
                for browser in states:
                    environment = dict(base, OBSERVER_RESULT=observer, NATIVE_RESULT=native, BROWSER_RESULT=browser)
                    with self.subTest(observer=observer, native=native, browser=browser):
                        self.assertEqual(outcome(environment), observer == native == browser == 'success')
        success = dict(base, OBSERVER_RESULT='success', NATIVE_RESULT='success', BROWSER_RESULT='success')
        for key, value in (('WORKFLOW_CANCELLED', 'true'), ('EVENT_NAME', 'pull_request'),
                           ('GITHUB_REF', 'refs/tags/v1'), ('GITHUB_REF', 'refs/heads/other')):
            with self.subTest(context=(key, value)):
                self.assertFalse(outcome(dict(success, **{key: value})))
        for missing in ('OBSERVER_RESULT', 'NATIVE_RESULT', 'BROWSER_RESULT'):
            environment = dict(success); del environment[missing]
            with self.subTest(missing=missing):
                self.assertFalse(outcome(environment))



if __name__ == "__main__":
    unittest.main()
