# Inspect a service-grant change through MCP

This local stdio server lets a coding agent discover and call one tool,
`assess_service_grant`. It checks whether a supported narrowing preserves one
required service request. The decision is made by the existing [Gooo](https://github.com/kimjooyoon/meta-ontology-go) generation;
the Go adapter only handles MCP and exact fact transport.

For example, narrowing `*` to `mcp://*` retains `mcp://weather`. Narrowing the
same grant to `http://*` removes that request. Both are successful assessments,
with different decisions.

## Build and connect

Build from a reviewed checkout with **Go 1.27.2**:

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
[`agentmesh-service-policy` module](https://github.com/kimjooyoon/gooo-go-ports/tree/2aa0f96805f116c6e5ec2a20c22f62bb284a5abc/examples/agentmesh-service-policy)
`v0.0.0-20261018191951-2aa0f96805f1`. Its Gooo compiler source is
`83928fa4538245abe33b08549da47f8cbdc50ed2`; this adapter does not upgrade or
run the language engine. MCP transport uses the official Go SDK **v1.8.0**.
`go.sum` pins both modules and their dependency graph.

The files under `fixtures/` retain actual native CLI inputs and outputs.
`fixtures/origin.json` records their hashes, generation identity and Linux CI
origin. The verifier compares whole proof objects, including false/empty fields
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

The default command and `assess_service_grant` tool keep the original 839
profile, seven decision fields and 89 asset bindings. Start the same native
command with `--many-required` to additionally expose
`assess_service_grant_change`. Its arguments are one object, for example:

```json
{"before":"*","after":"mcp://*","required_requests":[{"type":"mcp","name":"weather"},{"type":"mcp","name":"calculator"}]}
```

The optional tool preserves request order and duplicates. It returns the complete
ordered-column facts, 12 typed decision fields and the separate de99 generation's
30 asset and five source-hash bindings. Domain-invalid requests and the 257-request
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
