# Preserve all required service requests before narrowing a grant

This example evaluates one proposed grant change against an ordered workflow
requirement list. Gooo owns the domain checks, the 256-request policy limit, all-item
preservation, first counterexample, and supported-subset decision. Nothing here
authenticates live permissions or applies a grant.

From a checkout with Go 1.27.2, run this JSON **array** (no compiler installation is
needed to build or run the committed generated-Go consumer):

```sh
cd examples/service-grant-many
go run ./cmd/evaluate <<'JSON'
[{"before":"*","after":"mcp://*","required_requests":[{"type":"mcp","name":"weather"},{"type":"mcp","name":"calculator"}]}]
JSON
```

Each result echoes the four canonical facts, returns all 12 decision fields and
binds the generated policy/source assets. The request above is `ELIGIBLE` with
`SUBSET_AND_ALL_REQUIRED_REQUESTS_PRESERVED`. Changing `after` to
`mcp://weather` refuses the change with `REQUIRED_REQUEST_REMOVED` and
`first_counterexample: 1`. Order and duplicate positions are preserved. The
reported index addresses the original supplied requirement list.

Go callers can use `Evaluate(Input)` or `Assess(Input)` with ordered
`RequiredRequests`. `EvaluateColumns(RequestColumns)` exposes the original four
facts directly, including length-mismatch diagnostics. The CLI's `-columns`
option accepts `before`, `after`, `required_types`, `required_names` instead of
`required_requests`. It is also the exact corpus transport used by verification.
`AssessColumns` copies caller-owned requirement slices so later caller reuse
does not change its retained facts. Returned Go structs remain ordinary mutable
values. Both routes call the same generated Gooo activity. The pair-to-column loop is
transport only; Go does not decide preservation or subset eligibility.

`ELIGIBLE`, `REFUSED` and represented `INVALID_INPUT` are normal results and exit 0.
Malformed structure (duplicate/missing/extra keys, wrong types, null arrays or
strings, invalid UTF-8 and unpaired surrogate escapes) exits 1 with no decision
stdout. All records are decoded before evaluation/output, so a structurally bad
later record does not emit earlier decisions. The 1 MiB batch budget and 1..128-case
limit are wire limits. They do not truncate the policy corpus or impose a Go 256
request filter: 257 requests reach Gooo and return
`INVALID_INPUT / REQUIRED_REQUEST_LIMIT_EXCEEDED / checked_count:0`.

Types, names and supported patterns retain the existing bounded ASCII 128-byte
service domain. Empty/unequal columns and later invalid items are represented
Gooo decisions, not valid grants. The complete 12 fields are `status`, `reason`,
`invalid_field`, `invalid_index`, `first_counterexample`, `request_count`,
`checked_count`, `all_preserved`, `supported_subset`, `subset_rule_id`,
`counterexample_before_match`, `counterexample_after_match`.

## Reproduce source generation and verification

Only this maintainer procedure needs the separately pinned development compiler
`de99b770b8b5adea608233c3c0664902336e3310` (v0.6.35-dev) and Go 1.27.2. Check
`compiler-pin.json` and the matching released archive SHA; root `.gooo` and the
repository's 839 default pin/catalog remain separate. No language-engine build or
model call is needed. Download/bootstrap is not performed by the verifier.

```sh
python3 -I -B verify.py --compiler /absolute/path/to/pinned-de99-gooo \
  --go /absolute/path/to/go1.27.2/bin/go --out /absolute/new/owned/verification
```

The verifier genuinely executes and saved-replays the original 1+15 batches,
checks every four-fact input and 12 typed output plus source/entry/producer/plan
bindings, then checks committed generation. It independently compiles/runs the
Go reference on 17 cases (the original 16 plus the explicit 257 policy boundary), runs
module vet/race, and checks the compiled CLI on all 17 facts and malformed-later
controls. The default ordered-pair route also checks the 16 representable
cases; the unequal-column case remains an explicit `-columns` diagnostic.
The supplied Go version and file SHA are checked against all four fresh receipts,
and only Darwin arm64 and Linux amd64 are supported by this verifier. Each
command is bounded to 180 seconds (Go checks: 90 seconds), 32 MiB of combined
stdout/stderr, and a sampled 512 MiB combined owned cache/TMP budget. Its output is bounded and retains first failures and direct process
wait/EOF/group status. First failure stops the run; there is no automatic retry.

To intentionally regenerate after a policy/ID/source change, use the same
command with `--write-generated`. It still performs genuine source execution and
replay; it does not synthesize receipts. `generate.py --receipts <four-call-out>
--check` is a saved-data check, invokes no compiler, and explicitly reports that
scope. `generation.json` records both the original composition projection SHA
and transformed projection SHA. The only projection transformation is the first
package token `main`→`servicegrantmany`; the bridge maps compiler-owned plan IDs.
Changing source requires new genuine receipts, not rebinding a private old PASS.

## Inspect the selected program without another execution

After verification, the pinned compiler can inspect static value connections in
one retained execution record:

```sh
/absolute/path/to/pinned-de99-gooo package origins \
  --receipt /absolute/new/owned/verification/normal-execute/stdout \
  --activity gooo://service-policy/multi-required/assess/v1 \
  --follow --limit 128 --json
```

This reads the saved selected program; it does not execute the policy or call a
model. Inspect `report.value_flow.analysis_status` and `truncated`: the finite
128-row observation is `PARTIAL`. These are static connections, not complete
runtime causality. To bind the full selected source, compare
`result.composition.gooo_source` with its `selected_source_sha256` in the original
record. Graph `body_sha256` values identify compiler-normalized bodies, and path
positions refer to the decoded activity body, not original file offsets.

The independent reference is a separate nested module and is never imported by
production. Its original matcher bytes, upstream pin, adaptation and licenses
are retained under `reference/` and `NOTICE.md`.

Function results and observation completeness are separate. The full 15-case batch
currently reaches local recording caps: `PARTIAL`, with 314433 writes and 865219
uses dropped, while collection reads are 2100/drop 0 and field summaries COMPLETE.
Preserve actual statuses/counters from each new run; do not shrink the original
16-case corpus or interpret these other COMPLETE summaries as complete causal
lineage. Earlier private receipts remain historical.

Required source CI runs this verifier with the pinned Linux amd64 compiler.
Local Darwin checks alone do not establish Linux execution. These finite checks
do not establish external adoption, full AgentMesh/CEL compatibility or current
authority to apply a grant.
