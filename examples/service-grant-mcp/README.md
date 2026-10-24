# Inspect a service-grant change through MCP

This local stdio server lets a coding agent discover and call one tool,
`assess_service_grant`. It checks whether a supported narrowing preserves one
required service request. The decision is made by the existing [Gooo](https://github.com/kimjooyoon/meta-ontology-go) generation;
the Go adapter only handles MCP and exact fact transport.

For example, narrowing `*` to `mcp://*` retains `mcp://weather`. Narrowing the
same grant to `http://*` removes that request. Both are successful assessments,
with different decisions.

## Build and connect

Build from a reviewed checkout with **Go 1.27.3**:

```sh
GOTOOLCHAIN=local GOWORK=off CGO_ENABLED=0 \
  go -C examples/service-grant-mcp build -trimpath -o /absolute/private/bin/gooo-service-grant-mcp .
```

The build downloads the pinned public modules through the configured Go proxy
and checksum service. The resulting native executable needs no Go or Gooo
installation at runtime. Configure your MCP host to launch the absolute
executable path over stdio, without arguments. This server opens no HTTP port,
needs no API key and performs no model call.

The Python standard-library wire verifier exercises protocol versions
`2025-11-25` and `2026-07-28`. It checks discovery, tool schemas, repeated calls,
exact native CLI proof equality, strict input errors and protocol-only stdout.
The Go tests also compare concurrent handler calls with those complete native
proofs; the focused workflow runs them with the race detector on Linux and macOS.
Five Python regression checks use actual exited child processes and buffered
pipes to verify that a failed request retains its first error and diagnostics.
Injected cancellation exceptions check propagation after cleanup; they do not
establish operating-system signal behavior.
These observations do not establish that a task was completed inside any
particular agent application.

## Call the tool

Choose `assess_service_grant` for one required service request, or start the
server with `--many-required` and choose `assess_service_grant_change` for an
ordered list. Discovery includes descriptions and examples for every input
field, including each list item's `type` and `name`. These identify a service
such as `mcp://weather`, rather than an individual tool or a URL path.
Use `before` for the previous grant and `after` for the proposed replacement;
preserve supplied text instead of repairing an unsupported identifier.

Supply exactly four JSON strings as the tool arguments object:

```json
{
  "before": "*",
  "after": "mcp://*",
  "required_type": "mcp",
  "required_name": "weather"
}
```

The result includes the original `facts`, a seven-field `decision` and the full
compiler/generation/source `bindings`, in both MCP `structuredContent` and a JSON
text content block. Preserve the complete result.

| Result | Meaning |
| --- | --- |
| `ELIGIBLE` | This supported subset preserves the supplied required request. |
| `REFUSED` | The represented change fails the preservation or refinement check. |
| `INVALID_INPUT` | The strings do not meet the Gooo policy's canonical domain. |
| MCP `isError: true` | Transport, source-binding or execution failure; no policy decision is returned. |

Duplicate keys, missing or extra fields, null, wrong types and unpaired Unicode
surrogates are transport errors. The raw arguments object is limited to 4,096
bytes; a complete inbound stdio frame is separately limited to 64 KiB. These
transport limits are distinct from the policy's represented string domain.

The supplied previous grant and required request are not authenticated here.
The tool applies no grant, checks no live authority and contacts no service.
The [conditional application example](../agentmesh-service-policy/conditional-apply.md)
demonstrates the separate version/writer guards needed by an application that
automatically applies a change.

## Versions and verification

The runtime is pinned to the public
[`agentmesh-service-policy` module](https://github.com/kimjooyoon/gooo-go-ports/tree/7180a4a0f36a1b0d89470100d6ccf6efdb5fd38d/examples/agentmesh-service-policy)
`v0.0.0-20261020182808-7180a4a0f36a`. Its Gooo compiler source is
`205896a413e5dea92366309c35ae85e542d055f6`; this adapter does not upgrade or
run the language engine. MCP transport uses the official Go SDK **v1.8.0**.
`go.sum` pins both modules and their dependency graph.

The files under `fixtures/` retain actual native CLI inputs and outputs.
`fixtures/origin.json` records their hashes, actual public module, Go 1.27.3
build identity and platform. Earlier Linux observations are retained as history;
they do not certify these new bytes. The verifier compares whole proof objects, including false/empty fields
and all 89 asset bindings; it does not reconstruct a substitute policy result.

To verify a built command, choose a fresh output directory:

```sh
python3 -I -B examples/service-grant-mcp/verify.py \
  --binary /absolute/private/bin/gooo-service-grant-mcp \
  --out /absolute/private/new-mcp-verification
```

Report the platform, source version, command, exit status and first diagnostic
in the [first-run feedback form](https://github.com/kimjooyoon/gooo-go-ports/issues/new?template=problem.yml).
Choose **Native setup, verification, or replay** and use synthetic public facts.
Feedback about a real task or a missing capability is more useful than a claim
of compatibility with every MCP client or every service-pattern language.

## Optional ordered required requests

The default command and `assess_service_grant` tool use the current 205
profile, seven decision fields and 89 asset bindings. Start the same native
command with `--many-required` to additionally expose
`assess_service_grant_change`. Its arguments are one object, for example:

```json
{"before":"*","after":"mcp://*","required_requests":[{"type":"mcp","name":"weather"},{"type":"mcp","name":"calculator"}]}
```

The optional tool preserves request order and duplicates. It returns the complete
ordered-column facts, 12 typed decision fields and the many-request generation's
30 asset and five source-hash bindings. Both policy modules were generated with
compiler `205896a` using Go 1.27.3. Domain-invalid requests and the 257-request
policy limit return decisions; malformed JSON returns no partial proof. Supplied
facts are unsigned: this tool authenticates no authority and applies no grant.
The five Gooo source bodies remain repository sources, not embedded module files.

Run the existing default wire check and the optional check separately after
building the transport as above. The optional check reads the exact public
`service-grant-many` dependency selected by this module:

```sh
mcp_binary=/absolute/private/bin/gooo-service-grant-mcp
mkdir -p out
python3 -I -B examples/service-grant-mcp/verify.py \
  --binary "$mcp_binary" --out "$PWD/out/mcp-default-wire"
many_module="$(go -C examples/service-grant-mcp list -m -f '{{.Dir}}' \
  github.com/kimjooyoon/gooo-go-ports/examples/service-grant-many)"
python3 -I -B examples/service-grant-mcp/verify_many.py \
  --binary "$mcp_binary" \
  --legacy-verifier "$PWD/examples/service-grant-mcp/verify.py" \
  --many-module "$many_module" --out "$PWD/out/mcp-many-wire"
```

Both clients exercise the two documented protocol versions without a model.
The paired MCP input cannot express a mismatch between two column lengths; that
separate public Go API case remains covered in `service-grant-many` tests.

## Optional tool-alias origin comparison

Build this checkout's command and start it with `--alias-origin-change` to add
the read-only tool `gooo_compare_alias_origin`. This source profile is not in
the previously released preview16 executable or image. It reuses the public
`alias-origin-change` module at
`v0.0.0-20261024154510-16d1e7864dc8`, without invoking Go or Gooo at runtime.
The module's compiler pin is `205896a413e5dea92366309c35ae85e542d055f6` and its
embedded generation document is pinned by SHA-256
`b0c87b7865697c4b6433ef54e23defd7267ed8ea4da562cc8e1af24e283e7b06`.

The default command still exposes only `assess_service_grant`;
`--many-required` still exposes the same two service-grant tools. The alias
option alone exposes those default and alias tools (two total). Both options
together expose all three tools, in either argument order. Duplicate or unknown
options are errors. A host with an `enabled_tools` allowlist must also include
`gooo_compare_alias_origin` before the agent can discover it.

Use it when a name chosen by an agent might resolve to a different declared
server or original function after an update. Pass one exact arguments object:

```json
{"target_alias":"docs_search","before":[{"server":"docs","original_name":"search","exposed_name":"docs_search"}],"after":[{"server":"issues","original_name":"query","exposed_name":"docs_search"}]}
```

This returns `ORIGIN_CHANGED`: the supplied alias moved from `docs/search` to
`issues/query`. Review that change before relying on the previous tool choice.
`UNCHANGED` means the supplied origin pair is the same. `ADDED`, `REMOVED` and
`ABSENT` describe presence in the two supplied lists. `AMBIGUOUS` identifies a
duplicate exposed name anywhere in either catalog, even an unrelated alias;
repair those indexed rows and compare fresh facts. Empty domain text returns
`INVALID_INPUT`. See the [complete recovery guide](../alias-origin-change/agent-recovery.md)
for the next action for every state.

The adapter passes the received arguments bytes directly to `CompareJSON`.
It returns the complete `facts`, `facts_sha256`, `input_sha256`, nine-field
`decision` and `bindings` in both `structuredContent` and JSON text. The input
digest identifies the exact arguments object received through MCP, including
its whitespace; it does not identify the whole JSON-RPC request. Original row
positions are zero-based. Text is opaque: preserve spelling, order and Unicode
without trimming, case folding or normalization. The executable contains exact
snapshots of the pinned module's [input](../alias-origin-change/schemas/input.schema.json)
and [output](../alias-origin-change/schemas/output.schema.json) schemas for discovery;
wire checks compare these with the public module rather than defining a second
schema contract.

Each catalog allows at most 256 rows, each text at most 1,024 UTF-8 bytes, and
the raw arguments object at most 1 MiB. The alias profile allows another 64 KiB
for inbound JSON-RPC framing; the default 64 KiB and existing many-request
frame limits are unchanged. Invalid JSON, duplicate keys, null arrays, invalid
Unicode or transport bounds return `isError: true` with the public JSON
diagnostic in text and no partial report. These are separate from the seven
represented decision states.

The serialized report retains its separate 4 MiB limit. A complete outbound
JSON-RPC frame, including structured content, the escaped text copy, metadata,
request IDs, any batch framing and the final newline, is limited to
**16 MiB + 64 KiB** only in the alias profile. The larger bound accounts for two
copies of a report and additional JSON escaping and framing; a report size
alone is not a wire size. An oversized frame replaces its alias reports with
`isError: true` diagnostics whose state/stage/code are
`INVALID_TRANSPORT / encode_result / MCP_WIRE_LIMIT_EXCEEDED`, without writing
partial reports. Other tools' results in a mixed frame are preserved; an output
that cannot be reduced within the bound fails the connection without writing
that frame. This output-only diagnostic extends the module's CLI diagnostics.

These comparisons use supplied snapshots. They do not authenticate catalog
completeness, live server identity, SDK routing or current authority. Every
represented result retains `execution_authorized: false`. No named tool,
service or model is called.

The separate `verify_alias.py` wire check uses a built comparison CLI from the
same pinned public module as its complete-report reference, on protocols
`2025-11-25` and `2026-07-28`. Its default `--scope full` includes large accepted
responses and the independent 4 MiB report refusal. `--scope bundle-small`
checks the fixed small cases and all four discovery modes for an extracted
binary; it does not repeat or claim the large-response observations. Both
scopes retain command identities, exact streams and child-process closure.
These are finite internal protocol checks, not a completed task in an external
agent application or a published release.

After building the MCP command above, build its reference CLI from the same
pinned dependency in the MCP module context, then use a fresh evidence directory:

```sh
mcp_binary=/absolute/private/bin/gooo-service-grant-mcp
alias_cli=/absolute/private/bin/alias-compare
GOTOOLCHAIN=local GOWORK=off CGO_ENABLED=0 \
  go -C examples/service-grant-mcp build -trimpath -o "$alias_cli" \
  github.com/kimjooyoon/gooo-go-ports/examples/alias-origin-change/cmd/compare
alias_module="$(go -C examples/service-grant-mcp list -m -f '{{.Dir}}' \
  github.com/kimjooyoon/gooo-go-ports/examples/alias-origin-change)"
python3 -I -B examples/service-grant-mcp/verify_alias.py \
  --binary "$mcp_binary" --alias-cli "$alias_cli" \
  --alias-module "$alias_module" --go "$(go env GOROOT)/bin/go" \
  --out /absolute/private/new-alias-wire --scope full
```

The Go executable in this verification command only inspects the two binaries'
build identities. The actual CLI and MCP calls run with empty `PATH` and absent
`GOROOT`; no model or external service is used.
