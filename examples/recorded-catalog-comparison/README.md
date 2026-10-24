# Compare saved MCP catalogs with Gooo

An agent may see the same exposed tool name after its host has connected it to a
different server. It may also see fewer tools because a saved page is missing.
This experimental example keeps those two questions separate: what changed in
the supplied alias rows, and whether the recorded collection context is complete
enough for that comparison.

The Gooo composition imports the existing alias-origin activity and adds rules
for declared connections, pagination, configuration changes and authorization
scope. Go preserves and decodes saved JSON-RPC frames, projects their names and
host-declared aliases, and calls the generated Gooo kernel. It performs no network
collection or tool execution.

## Try a synthetic example

From this repository with Go 1.27.3:

```sh
go -C examples/recorded-catalog-comparison build -o /tmp/gooo-recorded-catalog ./cmd/compare
/tmp/gooo-recorded-catalog < examples/recorded-catalog-comparison/example-input.json
```

The example contains two connections that both claim the server name `docs` and
both list a tool named `search`. The host changes which connection is exposed as
`docs_search`. The expected alias state is `ORIGIN_CHANGED`; the expected
collection state is `RECORDED_CONTEXT_COMPLETE`. Both sides are explicitly marked
`SYNTHETIC_FIXTURE`. This result is an exercise of the supplied contract, not an
observation of a running service.

The executable uses its generated kernel at runtime. It does not need the Gooo
compiler, a model, credentials or network access to compare a saved input. This
example has its own CLI and Go package; released MCP bundles do not expose it.

## Provide your saved observations

Use the shape in [example-input.json](example-input.json). Every outer field is
required, including empty arrays. A comparison contains `target_alias`, `before`
and `after`. Each side declares its intended connections separately from its
actual captures, so a missing capture remains visible.

Each capture includes:

- Host-owned `host_id` and `connection_id`, plus an informational
  `server_name_claim`. A claimed server name is not a unique origin identifier.
- Begin, end and per-page context: meaningful backend binding revision,
  authorization context ID, relevant configuration revision and protocol version.
  Use non-secret opaque IDs, not tokens or credential contents. `captured_at` is
  preserved text and does not establish freshness.
- The stated recording basis: `CAPTURE_TIME`, `RETROSPECTIVE` or `UNKNOWN`.
  Retrospective metadata remains uncertain even when its values look consistent.
- `SYNTHETIC_FIXTURE` or `RECORDED_OBSERVATION`, and a configuration watch record.
  Equal endpoint samples alone cannot establish continuous stability.
- Ordered saved `tools/list` request and response strings, preserving their exact
  UTF-8 JSON bytes, with one `declared_exposed_names` entry for every returned tool.
  These aliases are host declarations, not proof of what a model received.

The importer requires matching typed JSON-RPC IDs and preserves unknown envelope,
parameter and descriptor fields in the original frame strings. It extracts
cursors from those frames. An absent request cursor starts a chain; request
`cursor: null` is refused. An absent or null response `nextCursor` ends the chain.
The empty string is a real continuation token. A response marked
`resultType: complete` does not establish that all pages were collected.

This is a narrow saved-frame profile: JSON-RPC 2.0 `tools/list`, string or int64
integer IDs, successful result objects, and absent or `complete` result type.
It is not a general MCP protocol validator. A saved RPC error produces a bounded
diagnostic containing the original response instead of an empty tool list.

## Read the two decisions together

| Saved observation | Alias result | Collection result |
| --- | --- | --- |
| Same alias, different host connection | `ORIGIN_CHANGED` | `RECORDED_CONTEXT_COMPLETE` |
| Same connection, changed backend between stable captures | `ORIGIN_CHANGED` | `RECORDED_CONTEXT_COMPLETE` |
| Same origin, old exposed alias removed | `REMOVED` | `RECORDED_CONTEXT_COMPLETE` |
| Middle page missing | Result over remaining supplied rows | `INCOMPLETE_COLLECTION` |
| Relevant configuration changed during capture | Result over supplied rows | `UNSTABLE_CONFIGURATION` |
| Host mapping added retrospectively | Result over supplied rows | `COLLECTION_CONTEXT_UNKNOWN` |

`RECORDED_CONTEXT_COMPLETE` means the saved roster, page chain and stated context
conditions passed. The recording basis and continuous-watch fields are supplied
observations, not authenticated attestations. Remote atomicity remains `UNKNOWN`.
`REMOVED` means absent from the supplied rows; `live_removal_proven` and both
`execution_authorized` fields remain false. An autonomous host can use the result
to choose a next collection or comparison step without requiring per-request
human approval; execution permission requires its own current authority checks.

`authorization_relation` compares the declared catalog scope and begin/end auth
IDs of connections retained across both sides. A temporary auth change seen on a
page is also retained as unstable collection context; `SAME_RECORDED` alone does
not assert that all page contexts matched.

The report preserves the original facts, exact-input and Go-JSON facts hashes,
every request/response hash, row positions and origin tuples. Origin identity uses
the Go-JSON tuple `[host_id, connection_id, backend_binding_revision]`. Server-name
claims, auth IDs and unrelated configuration values do not change that identity.
The complete observation is bound separately, so those fields remain traceable.
These encodings are not RFC 8785 canonicalization or producer authentication.

## Bounds and failure behavior

Inputs are limited to 1 MiB; reports to 4 MiB. Each side may contain eight declared
connections and eight captures, each capture sixteen pages, each saved request or
response 64 KiB, and each side 256 projected tool rows. Context/name/cursor strings
are bounded to 1,024 UTF-8 bytes. Opaque descriptor and diagnostic contents are
bounded by their complete frame. Duplicate JSON keys, lossy Unicode, shape errors,
unsupported types and over-limit input are refused without partial stdout
decisions or silent truncation. Domain context states are successful comparison
results and include the next action selected by Gooo.

## Verification and source

[consumer-cases.json](consumer-cases.json) provides six synthetic first-use cases
with independently stated public expectations. The verifier executes and replays
the pinned source, checks freshly generated kernel assets, runs the module tests
and checks actual CLI consumers. Its result and retained receipts determine
acceptance; this document alone is not evidence that a checkout passed.

The original behavior is in
[`recorded_catalog.gooo`](../../ports/tool-origin/recorded_catalog.gooo), composed
with the unchanged [`alias_origin.gooo`](../../ports/tool-origin/alias_origin.gooo).
Generation records bind both sources and the pinned compiler. Replaying saved
observations is a new comparison, not a new collection. Downstream provenance
should record the actual inputs used and report generated, without inventing a
capture activity or a derivation from an earlier observation merely because it
occurred earlier.

The saved format follows the MCP [tools](https://modelcontextprotocol.io/specification/2026-07-28/server/tools)
and [pagination](https://modelcontextprotocol.io/specification/2026-07-28/server/utilities/pagination)
specifications. Provenance terminology follows [W3C PROV-O](https://www.w3.org/TR/prov-o/).
