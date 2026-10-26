# Whole-catalog refresh with Gooo

A failed refresh should leave the stored catalog intact. A successful complete
refresh should replace it once, including when the new catalog is deliberately
empty. This example gives those rules to Gooo and connects the generated decision
to an actual in-memory Go transaction.

The use case comes from the partial-refresh concern in
[Gemini CLI #29825](https://github.com/google-gemini/gemini-cli/issues/29825).
This is an independent synthetic example, not an integration with Gemini CLI or
evidence of adoption. It makes no network or model calls.

## Try the local consumer

From the repository root, with Go **1.27.3**:

```sh
go -C examples/catalog-refresh run ./cmd/catalog-refresh-demo
```

Three JSON lines demonstrate partial retention, complete replacement, and empty
replacement. Each includes the pure Gooo decision and a separate actual commit
receipt. A decision of `CANDIDATE_ELIGIBLE` may still result in `RETAINED` if the
transaction state changes before the final locked comparison.

The generated kernel is committed, so this consumer does not need a compiler at
runtime. To change the policy, edit
[`ports/tool-origin/catalog_refresh.gooo`](../../ports/tool-origin/catalog_refresh.gooo)
and run the source-bound native verifier described below.

## Embed the transaction

```go
registry, err := catalogrefresh.NewRegistry(previousStoredBytes)
if err != nil { return err }
attempt, err := registry.Begin(ctx)
if err != nil { return err }
receipt := registry.Refresh(attempt, savedPages, "SUCCESS")
// Inspect receipt.State: APPLIED or RETAINED.
```

Import `github.com/kimjooyoon/gooo-go-ports/examples/catalog-refresh`. The example
module uses a local `replace` for the shared saved-page projector; keep the two
example directories together when trying it from a checkout. `savedPages` is a
slice of `SnapshotPage`, whose `RequestJSON` and `ResponseJSON` fields contain the
original JSON-RPC page strings. Collection errors are explicit `ERROR`, `TIMEOUT`
or `CANCELLED` outcomes, including when some pages were saved successfully.

Call `Invalidate()` when a relevant catalog notification arrives and
`ChangeScope()` when connection, authorization or configuration scope changes.
`Begin` supersedes older attempts immediately, even if the new attempt later
fails. The host must deliver those events: the policy cannot discover omitted
notifications or authenticate a collector's assertions.

## What decides replacement

1. The host copies and stages page bytes away from active storage. It reuses the
   exact-key saved-page parser, computes descriptor IDs and binds every shape
   result to its corresponding row. The candidate digest covers the pages,
   projected facts, validation evidence and captured transaction state.
2. Gooo checks collector completion, a complete bounded page chain, duplicate
   names, supported definition profiles, every row's validation and binding, and
   expected/current registry, scope, invalidation and attempt tokens.
3. The generated activity returns 18 typed fields. It performs no mutation and
   always reports `execution_authorized=false`, `live_effect_performed=false`
   and `remote_atomicity=UNKNOWN`.
4. Under one lock, Go rechecks candidate identity, cancellation and the complete
   state tuple before replacing stored bytes. The receipt records the actual
   before/after version and digest separately from the earlier decision.

| Input or interleaving | Stored result |
| --- | --- |
| Later page fails or a terminal page is missing | Retained |
| A nonselected row fails validation | Retained |
| Complete terminal page with zero tools | Replaced with an empty catalog |
| Newer attempt starts while an older attempt is running | Older attempt cannot replace storage |
| Notification, scope change or cancellation is observed before commit | Retained |
| Same bytes are refreshed again | Fresh version; old attempt remains stale |

Selected-tool equality in the snapshot comparator is insufficient for this
whole-catalog decision: the comparator can compare an observed row while other
pages are still missing.

## Boundaries

The descriptor profile `gooo/mcp-tool-shape/v1` checks a bounded name, required
`inputSchema` object with exact `type: "object"`, optional `outputSchema` of that
shape, bounded title/description strings, and the known annotation hint types.
Unknown descriptor and schema properties remain part of the definition digest
but are not interpreted. This is not a complete JSON Schema validator.

The shared parser limits observations to 16 pages and 256 rows and rejects
malformed encodings, duplicate JSON keys and oversized input. It never silently
truncates a catalog. The Gooo CLI additionally checks bounded typed values before
entering the activity. Direct generated-function tests distinguish that layer.

This host is an in-process demonstration. Tokens are private to a registry
instance, never reused within it, and refuse counter exhaustion. There is no
durable storage, distributed transaction, live MCP collector or automatic retry.
The journal retains the latest 256 receipts; `DroppedReceipts()` reports discarded
history. Cancellation after the final commit check may arrive after replacement.
Keeping old bytes does not authorize exposing or calling revoked tools.

## Verification and provenance

The independent Go oracle uses page objects, a duplicate-name map and an ordered
guard table. Its 115 anchored cases retain nine typed-input boundary cases rather
than deleting them from coverage. The finite native verifier separately accounts
for 106 Gooo executions and their own saved replays, plus nine actual pre-body
input refusals. Consumer tests also compare all 115 cases with the generated
function and exercise the real stage/evaluate/commit path. Host interleaving tests
inject an evaluator to isolate transaction mechanics; those tests alone are not
Gooo policy verification.

```sh
go -C examples/catalog-refresh/reference test -race ./...
go -C examples/catalog-refresh test -race ./...
python3 -I -B examples/catalog-refresh/verify.py \
  --compiler "$PWD/.gooo" --go "$(go env GOROOT)/bin/go" \
  --go-cache "$(go env GOCACHE)" --module-cache "$(go env GOMODCACHE)" \
  --out /absolute/path/to/a-new-evidence-directory
```

The verifier pins compiler source, Go version, imported Gooo sources, workspace,
input cases and typed output identities. Generation checks each execution's own
replay parent and emits a PROV-O graph for its actual inputs and generated file
occurrences. These are retained local observations, not producer authentication.
The runtime receipt preserves evaluation and commit state separately; a later
commit outcome must not be relabelled as an input to its earlier pure decision.

This is original BSD-3-Clause Gooo/host code. The issue informs the requirement;
its reproducer was not copied. The shared projector remains part of this repo.
