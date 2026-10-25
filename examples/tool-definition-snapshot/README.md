# Compare saved tool-definition observations

This example compares one original tool name in two retained
sets of `tools/list` pages. It does not require or construct a host alias,
connection identity, backend revision or authorization context. The existing
alias/collection checker remains separate.

The motivation is an actual internal documentation lookup: two retained
Context7 observations advertised different server versions while returning
the same two tool definitions. Their original caller had not retained a host
alias registry. The current recorded-catalog command requires that registry
even for its definition mode. A newly declared retrospective mapping is a
different input; it does not recover historical host facts.

The new activity is
`gooo/tool-origin/definition-snapshot:CompareDefinitionSnapshot`.
Its authored ABI is in `abi.json`. Generation checks the compiler's typed plan
against that ABI; the ABI document itself is not an execution receipt.

## Try the separate saved-page input

The generated evaluator is included in this checkout. Repository-wide CI for
the new feature remains required before publication.

From the repository root, using its pinned Go toolchain:

```sh
go -C examples/recorded-catalog-comparison run ./cmd/compare --definition-snapshot \
  < examples/tool-definition-snapshot/example-input.json > snapshot-report.json
```

The checked-in input is synthetic. It changes the selected tool's schema limit
from 10 to 20; the intended result is `OBSERVED_DEFINITION_CHANGED`. The separate
`origin_relation` and `authorization_relation` remain `NOT_ASSESSED`.

To compare your own retained observations, replace `target_name` and the two
`pages` arrays. Each page holds the original `request_json` and `response_json`
strings. Use extracted JSON-RPC payloads, preserving any HTTP/SSE carrier and
its byte-extraction record separately. No connection ID or exposed alias is
needed. `--definitions` remains the existing host/alias comparison mode;
its policy and availability flags are not accepted with `--definition-snapshot`.

The report keeps the exact input string, its digest, ordered page/row references,
and the Gooo inputs and decision. A report's raw input and metadata can contain
private tool descriptions; keep it with the original evidence in your own
workspace. A same-definition result only compares the supplied projection. It
does not authorize calling that tool or prove unchanged server behavior.

## Input and scope

`SnapshotFacts` holds per-page request/next cursor tags and values, ordered
original tool names, definition and annotation IDs, definition profiles, and
row-to-page indexes. `Selection` names the original tool and binds the facts
to the caller's comparison context. That context identifier identifies this
local computation; it is not a host, backend or permission identity.

The strict adapter must retain exact requests, responses, matching RPC IDs,
page and row references, and the existing
`gooo/mcp-tool-definition-json/v1` projection. That profile sorts JSON object
keys while preserving array order, JSON number spellings, unknown descriptor
fields, and the distinction between absent annotations and explicit null.
It does not establish JSON-Schema equivalence or interpret descriptions.

Gooo validates the facts, page chain and selected-name result. Page chains
distinguish `ABSENT`, `NULL` and `STRING` cursors. A `STRING` cursor, including
the empty string, needs a matching next page. `ABSENT` and `NULL` next cursors
are terminal but remain separate recorded values. A nonempty page set beginning
without a request cursor and ending at a terminal cursor is complete only as
an observed page chain. It does not prove remote atomicity or unchanged access.

When the name is observed once on each side, the selected descriptor and
annotation relation can be shown with `SAVED_ROWS_ONLY` scope even if the page
chain is incomplete. Missing pages may still contain other rows; no global
uniqueness or unchanged-catalog claim follows. An observed addition/removal
requires complete coverage on the side where the name is absent. Both-absent
requires both page sets complete. Any observed duplicate original name yields
`AMBIGUOUS_NAME`; no first duplicate is selected. Unsupported projection
profiles remain unassessed rather than being treated as equal.

The pinned native input boundary limits each text value to 1,024 UTF-8 bytes
and each repeated field to 256 values. Values outside those bounds are refused
before the Gooo activity runs. The reference corpus retains those cases as
separate input-rejection probes; a rejected input is not a domain decision.
Direct generated-function tests also exercise the activity's defensive checks,
but those tests do not imply that the native entry accepts out-of-bound values.

Origin and authorization remain `NOT_ASSESSED`. Behavior equivalence,
execution authorization and live effects remain false in every result.
The activity performs no live collection, tool call or human-approval step.

## Validation status

Local validation covers all 89 independent reference cases: 83 admissible
inputs ran through Gooo and each execution's own saved replay; six oversized
inputs were refused at the pinned native input boundary. The generated API
and CLI module passed `go vet` and race tests. Receipt mutation controls check
source, selected inputs, result fields, authority flags, coverage and paths.
The input-refusal records also bind the launched compiler path and binary
digest to its observed build identity. These checks concern retained local
observations; they do not authenticate a remote producer.

Repository-wide CI for this feature is still pending. The retained Context7
input pair motivated the interface but has not yet completed the new CLI and
native comparison. It is an internal use case, not external adoption.

The retained Context7 responses are complete matching SSE events within HTTP
response prefixes, not complete HTTP streams or stdio frames. An extraction
of their data fields must retain its original byte offsets and hashes as a
derived entity. HTTP Date and advertised server version must not be promoted
to missing capture-time or backend observations. No private capture is part
of the public synthetic reference corpus.

The final typed result has 32 fields, within the pinned record limit. For duplicate
names, each affected side reports the second occurrence in `problem_row_index`;
the first matching name can be found in that side's ordered public projection.
The internal status preserves both indexes. Both selected row indexes are `-1`
and selected identifiers are empty whenever either side has duplicate names.

The pipeline in `evidence.py`, `generate.py`, and `verify.py` follows
the existing owned-process runner. It requires a current independent Go oracle
output in `kernel.cases.json`, then checks every native batch and its own replay
against source digests, exact typed fields, manifest, compiler and Go identities.
The generator emits five `snapshotkernel`/snapshot metadata assets only after a
complete finite index is checked. Its PROV-O graph identifies file occurrences by
generation activity, role, path and content hash; equal bytes in copied inputs and
outputs remain different occurrences. Eight receipt/index mutation controls and
small graph regression controls are included.

`kernel.cases.json` comes from an actual invocation of the independent Go
reference. Normal verification compares the oracle's exact bytes and all five
generated assets; it does not write into the repository. The verifier covers
the finite domain; raw adapter/CLI checks and repository-wide required CI are
separate requirements. It does not inherit an older example's execution result.

`bind_producer.py` can derive a stronger index from the retained initial run.
It preserves the original index and summary, verifies their recorded compiler
identity and process observations, and marks the result as retained-observation
revalidation. This operation does not execute Gooo again or create new native
observations. Its source and original records are included in the subsequent
generation's PROV-O dependencies.
