# Whole-catalog refresh with Gooo

A failed refresh should leave the stored catalog intact. A successful complete
refresh should replace it once, including when the new catalog is deliberately
empty. This example gives those rules to Gooo and connects the generated decision
to an actual in-memory Go transaction.

The use case comes from the partial-refresh concern in
[Gemini CLI #29825](https://github.com/google-gemini/gemini-cli/issues/29825).
This is an independent synthetic example, not an integration with Gemini CLI or
evidence of adoption. It makes no network or model calls.

## Evaluate your saved pages

With Go **1.27.3**, run this pinned preview from any directory:

```sh
go run github.com/kimjooyoon/gooo-go-ports/examples/catalog-refresh/cmd/catalog-refresh@9c0bf20da69cb4b7084e526f75910d026ca2fbb7 < saved-refresh.json > report.json
```

No repository checkout, local replacement or Gooo compiler is needed. The first
run may download Go modules; evaluating the supplied pages uses no MCP or model
connection. In a checkout that contains this command, the equivalent development
command is `go -C examples/catalog-refresh run ./cmd/catalog-refresh`.

The command reads exactly one JSON object from standard input. All three fields
are required; unknown or duplicate fields are rejected:

```json
{
  "previous_catalog_base64": "b2xkIGNhdGFsb2c=",
  "collector_state": "SUCCESS",
  "pages": [
    {
      "request_json": "{\"jsonrpc\":\"2.0\",\"id\":73,\"method\":\"tools/list\",\"params\":{}}",
      "response_json": "{\"jsonrpc\":\"2.0\",\"id\":73,\"result\":{\"tools\":[]}}"
    }
  ]
}
```

This synthetic input deliberately replaces the previous bytes `old catalog`
with a complete empty catalog. For your task, supply your own original ordered
request/response strings and the collection outcome: `SUCCESS`, `ERROR`,
`TIMEOUT`, or `CANCELLED`. Do not label missing or failed collection successful.
An empty `pages` array means no observed pages, not a successful empty catalog.
Use canonical padded base64 for the previous bytes (`""` for empty bytes); they
may be opaque or contain non-UTF-8 data. The complete input is limited to 1 MiB,
including base64, and the existing page, row and frame limits still apply.

The JSON report contains the actual `receipt` and `stored_catalog`. Its `bytes`
field is base64: decode it to recover exactly the retained bytes, or the original
page-string bundle after replacement. The report binds the exact command input
with `input_sha256` and labels `collector_state_basis` as `CALLER_SUPPLIED`.
The command cannot verify that a collector's outcome describes real events.

A valid request with incomplete pages or failed collection returns `RETAINED`
and exits zero; a complete candidate may return `APPLIED`. Malformed envelopes
**or malformed saved JSON-RPC frames** are rejected before registry creation:
exit 1, a JSON diagnostic on stderr and no result on stdout. No `RETAINED`
receipt is emitted for those parsing failures. Invalid arguments exit 2. Keep
input, stderr, exit status and output together; exit zero is not authorization.

Every invocation creates a new in-memory registry. Its tokens and versions are
local to that invocation, not durable versions or reproducible replay IDs. The
command does not change input files, save a catalog, collect live pages or
invoke tools. Applications embedding the library retain responsibility for
notifications, connection/authentication changes and their actual storage.

### Read original saved files

With Go **1.27.3**, evaluate saved files without manually escaping their
contents or encoding previous bytes. This pinned preview works from any directory:

```sh
go run github.com/kimjooyoon/gooo-go-ports/examples/catalog-refresh/cmd/catalog-refresh@7edebfce6db4694af12145701582ba326041301e --plan /absolute/path/to/refresh-plan.json > report.json
```

No checkout or Gooo compiler is required. In a matching source checkout, the
development equivalent is `go -C examples/catalog-refresh run ./cmd/catalog-refresh --plan /absolute/path/to/refresh-plan.json`.

The pinned `9c0bf20d` preview above predates `--plan`; it continues to accept the
stdin format. A file plan has exactly these fields:

```json
{
  "previous_catalog_file": "previous-catalog.bin",
  "collector_state": "TIMEOUT",
  "pages": [
    {
      "request_file": "frames/page-01.request.json",
      "response_file": "frames/page-01.response.json"
    }
  ]
}
```

Paths are relative to the supplied plan file's directory; explicit absolute
paths and symlinks to regular files are also accepted. The previous catalog may
contain arbitrary binary bytes, including an empty file. Request and response
files must be UTF-8; their complete strings, whitespace, escapes and order are
preserved. They must contain saved JSON-RPC frames, not HTTP or SSE carriers.
Keep transport extraction records separately. The command neither extracts nor
repairs frames and does not infer whether collection succeeded.

The plan must explicitly supply `SUCCESS`, `ERROR`, `TIMEOUT` or `CANCELLED`.
Unknown, duplicate, missing, incorrectly cased or incorrectly typed fields fail.
At most 16 page pairs are accepted, with 64 KiB per frame, 1 MiB for the plan and
1 MiB for the assembled input including base64. Only regular files are read;
missing files, nonregular files, observed file changes and exceeded bounds fail
before evaluation. File reads are local snapshots, not an atomic collection or
an attestation of the collector outcome. Keep source files unchanged during use.

`--plan` does not read stdin or write files. It assembles the ordinary input and
applies the same validator and Gooo evaluation as the stdin route. The optional
report field `file_plan_sha256` hashes the exact original plan bytes;
`input_sha256` hashes the assembled JSON envelope, not the plan. Decode
`stored_catalog.bytes` as above and retain the plan, referenced files, report,
stderr and exit status together. Shell output redirection can create a partial
file if writing fails; successful assembly does not imply a durable catalog swap.

## Try the local consumer

From the repository root, with Go **1.27.3**:

```sh
go -C examples/catalog-refresh run ./cmd/catalog-refresh-demo
```

Three JSON lines demonstrate partial retention, complete replacement, and empty
replacement. Each includes the pure Gooo decision and a separate actual commit
receipt. A decision of `CANDIDATE_ELIGIBLE` may still result in `RETAINED` if the
transaction state changes before the final locked comparison.

The released `recorded-catalog-cli-preview.2` command compares selected saved
tool definitions; it does not contain this whole-catalog transaction. Use the
Go module below to embed the transaction.

The generated kernel is committed, so this consumer does not need a Gooo
compiler. To change the policy, edit
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

Import `github.com/kimjooyoon/gooo-go-ports/examples/catalog-refresh` using
Go 1.27.3. This pinned preview revision works in a separate app without a
repository checkout or local replacements. For a new app without a `go.mod`,
first run `go mod init example.com/my-catalog-client`. After adding your source
with that import, run:

```sh
go get github.com/kimjooyoon/gooo-go-ports/examples/catalog-refresh@5699ab44a9a355a579bc08904e198eb6cf5554dd
go mod tidy
go run .
```

That commit resolves to `v0.0.0-20261026144904-5699ab44a9a3`; it is a pinned
preview, not a stable release. The [complete demo program](cmd/catalog-refresh-demo/main.go)
provides a runnable consumer with partial, complete and complete-empty catalogs.
When switching from the earlier local recipe, remove both example-module
replacements from your app first; otherwise Go continues using those local files.

### Work with local Gooo rules

For source development, use a matching checkout and replace only the `catalog-refresh`
module in your separate consumer. Its `go.mod` pins the shared saved-page
projector to the public module version
`v0.0.0-20261026003237-fd614f475ba1`; Go downloads and verifies that dependency
through the configured module proxy and checksum service.

After adding your Go source with the import above, run these commands from that
app's directory (set `GOOO_PORTS` to the absolute path of your checkout):

```sh
GOOO_PORTS="/absolute/path/to/gooo-go-ports"
go mod edit -require=github.com/kimjooyoon/gooo-go-ports/examples/catalog-refresh@v0.0.0
go mod edit -replace="github.com/kimjooyoon/gooo-go-ports/examples/catalog-refresh=$GOOO_PORTS/examples/catalog-refresh"
go mod tidy
go run .
```

`v0.0.0` is a placeholder for the local `catalog-refresh` replacement, not a
published release of this module. Use Go 1.27.3. `go mod tidy` records the pinned
shared-projector requirement; run it after adding the consuming source so that
the import is present. If migrating from the earlier two-replacement recipe,
remove its `recorded-catalog-comparison` replacement first so this route uses
the pinned public dependency.

`savedPages` is a slice of `SnapshotPage`, whose `RequestJSON` and `ResponseJSON`
fields contain the original JSON-RPC page strings. Collection errors are explicit
`ERROR`, `TIMEOUT` or `CANCELLED` outcomes, including when some pages were saved
successfully.

Read stored state with `stored := registry.Snapshot()`. After a successful
replacement, `stored.Bytes` is a JSON page bundle of the form
`{"pages":[{"request_json":"...","response_json":"..."}]}`, with the original frame
strings preserved. It is not a flattened tool array or an executable tool
registry. `stored.SHA256` and `stored.RegistryVersion` identify that stored value.
A refusal preserves the previous bytes and version, including opaque initial
bytes supplied to `NewRegistry`. Deriving an agent's usable tools and enforcing
current authorization remain responsibilities of the consuming application.

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
