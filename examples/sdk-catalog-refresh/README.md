# Collect MCP tool catalogs with Gooo

Keep the previous catalog when collection is incomplete or the SDK drops a tool
row. Replace it once when all original pages are available and still belong to
the current collection. A deliberately empty catalog can replace the previous one.

The Go MCP SDK intentionally filters invalid tool definitions. This adapter
keeps original IO frames and compares their admitted, ordered names with the SDK
result using an original Gooo rule. A difference invalidates the actual collection;
the existing Gooo whole-catalog rule then retains storage. No person approves
each refresh, and no tool is invoked.

## Try the local demo

From a matching source checkout, with Go **1.27.3**:

~~~sh
go -C examples/sdk-catalog-refresh run ./cmd/sdk-catalog-refresh-demo
~~~

The real SDK talks to a synthetic in-memory peer. Three JSON lines show complete
replacement, retention after SDK filtering, and actual empty replacement.
No external server, model or Gooo compiler is needed.

## Embed it in an agent host

~~~go
import refresh "github.com/kimjooyoon/gooo-go-ports/examples/sdk-catalog-refresh"

client, err := refresh.Connect(ctx, ownedConnection, previousCatalogBytes)
if err != nil { return err }
defer client.Close()
report, err := client.Refresh(ctx)
// Failed collection can return both an error and a RETAINED receipt.
// Inspect report.Receipt as well as err and report.CollectorState.
stored := client.Snapshot()
// stored.Bytes is the actual local catalog, with its version and SHA256.
~~~

The owned connection is an io.ReadWriteCloser carrying newline-delimited
JSON-RPC. Connect takes ownership, including on failure. Its Close must unblock
reads and writes. Keep the client for subsequent refreshes and close it when
finished. Each client has its own in-memory registry. Initial bytes are opaque;
no file or durable database is changed. Persistence remains the host's job.

The profile is tested with Go MCP SDK v1.8.0 and requires the **actually negotiated**
protocol 2025-11-25. Other negotiated versions are refused. The SDK session is
private. HTTP/SSE, cached-list protocols, batches and arbitrary string-ID clients
are outside this profile.

## Read the evidence

The report keeps original request/response payload bytes as base64, separately
from SDK names and their JSON digest. The line-feed delimiter is excluded;
payload whitespace and escapes remain unchanged. Request-write and response-
observation fields distinguish complete pairs from partial IO. Capture diagnostics
never present an oversized prefix as a complete original frame.

Each refresh acquires its lease before waiting for IO. New attempts supersede old
ones even if cancelled while queued. Queue cancellation leaves the active owner's
connection usable. Cancellation during active IO closes the owned connection to
unblock stalled writes; reconnect with a new client. Tool-list change notifications
invalidate before asynchronous SDK dispatch. Closure revokes the connection scope.

Bounds are 16 pages, 1 MiB capture, 64 KiB frame payloads, 1,024-byte cursors with
cycle detection, and 30 seconds for initialization or refresh. The Gooo row rule
allows at most 256 names of 1,024 bytes each. The separate catalog rule applies its
own bounds and refuses duplicate names. Unsupported input is never repaired into
a successful empty list.

The report embeds the row rule's actual generation origin: entry, public source,
manifest, typed plan and producer. This rule uses independent experimental
producer 1afb98b. The catalog transaction keeps producer 205896a, and the repository's
stable compiler pin is unchanged. Neither is an engine stable-release claim.

Equal names do not prove equal schemas, invocation permission, future effects or
remote atomicity. The local replacement decision and actual commit receipt remain
separate. Tests use synthetic peers; this is not an SDK feature or adoption claim.

## Change the Gooo rule

Edit [the original Gooo source](../../ports/tool-origin/sdk_row_visibility.gooo).
The [public manifest](../../sdk-catalog-row-visibility.workspace.json) binds wire
names to input0 and SDK names to input1. Keep that positional order. Go owns framing,
SDK calls and independent reference checks.

With Go 1.27.3, build the separate pinned compiler into a new directory outside
the checkout and verify the committed assets:

~~~sh
python3 -I -B examples/sdk-catalog-refresh/bootstrap.py --go "$(go env GOROOT)/bin/go" --out /absolute/new/profile-output
python3 -I -B examples/sdk-catalog-refresh/verify.py --compiler /absolute/new/profile-output/gooo --go "$(go env GOROOT)/bin/go" --go-cache "$(go env GOCACHE)" --module-cache "$(go env GOMODCACHE)" --out /absolute/new/verification-output
~~~

Bootstrap checks the exact revision, tag ancestry, clean source and build identity.
It may download public source and dependencies. Verification makes no network or
model call and builds no compiler. It runs eight native cases and their own saved
replay, verifies two oversized typed-input refusals before the Gooo body, checks ordered inputs and all nine output fields, rejects eight receipt
mutations, then regenerates the library from the actual public manifest and compares
its exact bytes and origin. It does not inherit an earlier PASS.

Output directories must be new. The write-generated option is only for the initial
all-absent pair of assets and never overwrites either file. Retain first-failure
output when changing the rule. Go tests separately compare 1,600 ordered-name pairs,
eight direct-function boundary pairs and the literal cases, alongside SDK lifecycle
scenarios. Oversized direct-function inputs are not native-input admission claims.
