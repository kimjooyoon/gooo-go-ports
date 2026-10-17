# Reproduce an AutoBot action-line decision

This experimental [Gooo](https://github.com/kimjooyoon/meta-ontology-go) port
reproduces the pure Python `resolve_line` rule and its reason text from
[AutoBot-AI at a9c47e2](https://github.com/mrveiss/AutoBot-AI/blob/a9c47e296907f1e5f681c38c05146cc8d3c2e7a7/autobot_shared/ai_lines.py).
It is a bounded Python-to-Gooo adaptation, separate from the Go API compatibility
suites. The application and its dependency stack are not needed to run it.

The [Gooo source](../../ports/autobot/ai_lines.gooo) calculates all eight result
fields. [oracle.go](oracle.go) is an independent Go verification reference; it
is not an execution fallback. Source, license and modification details are in
[NOTICE.md](NOTICE.md) and [source-pin.json](source-pin.json).

## Supply facts and inspect one result

Prepare Git, a POSIX shell, Python 3, Go **1.27.2**, and the repository's pinned
Gooo compiler **0.6.32-dev**, revision
`83928fa4538245abe33b08549da47f8cbdc50ed2`, using the
[existing setup instructions](../../README.md#run-and-edit-locally).
Reuse matching tools; the optional prebuilt compiler still requires identity
validation and host Go. Run the following from the repository root:

```sh
set -eu
umask 077
export GOTOOLCHAIN=local GOWORK=off GOPROXY=off GOSUMDB=off
GOOO_COMPILER="${GOOO_COMPILER:-$PWD/.gooo}"
GOOO_GO="$(go env GOROOT)/bin/go"
run_dir="$(mktemp -d "${TMPDIR:-/tmp}/gooo-autobot.XXXXXX")"
go build -trimpath -o "$run_dir/prepare-input" ./cmd/autobot-line-input
printf '%s\n' '[ ["crm:send",true,"autonomous",true,true,"financial",false,false,false] ]' \
  > "$run_dir/request.json"
"$run_dir/prepare-input" < "$run_dir/request.json" > "$run_dir/inputs.json"
"$GOOO_COMPILER" package execute --json --go "$GOOO_GO" \
  --inputs "$run_dir/inputs.json" autobot-resolver.workspace.json \
  > "$run_dir/execution.json"
"$GOOO_COMPILER" package replay --json --go "$GOOO_GO" \
  --receipt "$run_dir/execution.json" --inputs "$run_dir/inputs.json" \
  autobot-resolver.workspace.json > "$run_dir/replay.json"
printf 'Saved execution and replay in %s\n' "$run_dir"
```

For these supplied facts, the upstream rule specifies `draft_then_send`, source
`baseline_floor`, stored value `autonomous`, floor value `financial`, and reason
`approval baseline category 'financial' can never be autonomous`. The output
also preserves the original key and both presence flags. Keep the fresh receipts
and any first diagnostic. Saving an output file alone does not establish success.

## Input and result contract

The converter reads an array of rows; each row has these nine positional values:

| Position | Fact | Type |
| --- | --- | --- |
| 0 | key | Text |
| 1 / 2 | stored_present / stored_value | Boolean / Text |
| 3 | mutating | Boolean |
| 4 / 5 | baseline_present / baseline_value | Boolean / Text |
| 6 | role_requires_approval | Boolean |
| 7 | store_unavailable | Boolean |
| 8 | scope_missing | Boolean |

All Text is unchanged Unicode-scalar UTF-8 of at most **1,024 decoded bytes**.
Absent stored/baseline values must be empty Text. A present stored value must be
exactly `autonomous`, `draft_then_send` or `never`. A present baseline may be
empty; it remains distinct from an absent baseline. Whitespace is not trimmed.

The converter accepts **1..128 rows**, at most **1 MiB** of raw request JSON,
and at most **32 KiB including the final LF** for the converted input envelope.
It validates every row before writing anything, rejects malformed or out-of-domain
input without a decision, and emits the existing `gooo/body-composition-inputs/v1`
format. It does not execute Gooo. Separate joint/translated-input limits can
still refuse a document; the converter does not promise arbitrary batches fit.
There is no automatic chunking or truncation.

The raw entry `gooo/autobot/ai-lines:ResolveLine` has these conditions as host
preconditions; it does not validate them itself. Bypassing the converter does
not establish a checked external-input boundary.

The record order is `key`, `line`, `source`, `stored_present`, `stored_value`,
`floor_present`, `floor_value`, `reason`. In the upstream vocabulary,
`autonomous` means act then report, `draft_then_send` means draft while a person
sends, and `never` means the AI must not act. This port returns those labels; it
does not impose or implement an approval lifecycle.

Missing scope refuses a mutating action before store availability is considered.
An unavailable store refuses before stored/default/floor selection. Stored
`never` is final; otherwise a nonempty baseline or a role floor can tighten an
`autonomous` result. The exact source and reason distinguish these paths.

## Verify the current port

The input-only [corpus](corpus.json) retains **192 state combinations**
(stored4 × mutating2 × baseline3 × role2 × unavailable2 × scope2) and **12 Text
edges**. This is finite branch-state coverage, not exhaustive Text coverage.
The complete 204-row corpus is for verification, not one native input batch.
The verifier processes it in eight-row groups and compares fresh upstream Python,
independent Go and Gooo results, including all eight typed fields, then replays
each saved Gooo receipt. Read that run's retained summary before claiming a pass.
After building the converter and selecting the tools above, use another fresh
output directory:

```sh
python3 -I -B examples/autobot-resolver/verify.py \
  --root "$PWD" --compiler "$GOOO_COMPILER" --go "$GOOO_GO" \
  --input-command "$run_dir/prepare-input" --out "$run_dir/verification"
```
Renamed sources and entries need fresh verification; another generation's
receipts do not establish the current source's result.

## Boundary

The caller supplies the facts. This port does not identify a tool, authenticate
its principal, resolve a scope, read a store or baseline, preserve historical
policy artifacts, authorize a current action or apply effects. A saved receipt
checks finite source-bound execution/replay; it is unsigned and is not a proof
of historical facts or current authority. This is not a complete AutoBot port,
an installed integration, or evidence of independent adoption.
