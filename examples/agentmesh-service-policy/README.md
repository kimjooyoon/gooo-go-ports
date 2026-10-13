# Service-pattern matches and request-level grant change impact in Gooo

This example adapts one pure function from
[Google Agent Mesh](https://github.com/google/agentmesh/tree/9fd1a41a15174df0aaef601c942410c2116bff91):
`api.MatchServicePattern`. It also composes that adaptation twice in an original
Gooo activity that compares the old and candidate pattern for the **same supplied
request**. Gooo computes every match and all three impact fields. Go validates
the bounded transport domain, carries facts and checks source consistency.

For a request `mcp://weather`, changing `mcp://calculator` to `mcp://*` reports
`before_match=false`, `after_match=true`, `change=WIDENED`. The four request-level
states are `WIDENED`, `REVOKED`, `UNCHANGED_ALLOW` and `UNCHANGED_DENY`.
`WIDENED` describes this request only; it does not prove global grant inclusion,
find every widened request, or authorize an actual mesh call. No human approval
step or current-state/effect callback is present in these pure calculations.

This is **not the complete Agent Mesh authorization engine**. It includes no
Biscuit/OIDC/token verification, roles, task attenuation, HTTP/MCP/A2A wire
integration, attested identities, live provider lookup or dispatched effect.
Optional CEL eligibility composition and native saved-proof verification are
separate future work. A host must establish trusted facts and current state
before a real operation. This finite example is not production-safety evidence.

## Exact represented domain

Each full pattern, request type and request name is at most 128 ASCII bytes.
The pattern is empty, `*`, `type://*`, `type://name`, `type://*.suffix` or
`type://prefix.*`. Type syntax is `[A-Za-z][A-Za-z0-9+.-]*`, including custom and
case-sensitive schemes; it is not limited to the predefined mesh service types.
Names are nonempty dot-separated sequences of ASCII letters, digits, underscore
or hyphen. Empty request components are represented and match false. Comparisons
preserve case and the wildcard's dot boundary: `*.example` does not match
`example` or `evilexample`, and `gemini.*` does not match `gemini` or `gemini2.pro`.

Paths, query/fragment, user info, ports, legacy `type:name`, mixed wildcards,
empty interior dot labels, non-ASCII/control characters and oversized components
are refused before generated execution. The full upstream parser accepts forms
outside this transport domain; they are deliberately not passed through a Go
fallback. This Gooo adaptation is not a complete RFC 3986 parser.

The canonical domain is a precondition of the Gooo port. The native Go API
and CLI enforce it. Direct Gooo imports and workspace Text inputs do not
enforce this grammar; the workspace observations below use validated corpus
inputs. Importers must preserve that precondition.

## Use the generated ordinary Go library and stdin CLI

With Go 1.27.2 selected, run these commands from this example directory:

```sh
export GOTOOLCHAIN=go1.27.2
export GOWORK=off
mkdir -p out
go build -trimpath -o ./out/evaluate ./cmd/evaluate
./out/evaluate -mode matcher < matcher-demo.json
./out/evaluate -mode impact < impact-demo.json
```

The module is
`github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy`. An ordinary
Go host can import it and call `EvaluateMatcher(MatcherFacts)` or
`EvaluateImpact(ImpactFacts)`. The latter returns an `ImpactProof` with the exact
raw facts, three generated observations and complete source bindings. Runtime
evaluation calls the generated native Gooo projections, not a compiler or the Go
reference oracle. It makes no network calls.

The CLI accepts a JSON array of 1..1024 exact string-only fact objects, at most
1 MiB. Extra/duplicate keys (including escaped duplicates), missing fields,
`null`, wrong types, trailing JSON, malformed UTF-8, surrogate/non-ASCII/control
text and out-of-domain syntax are refused. Every member is decoded and evaluated
before output; a bad later member emits no partial result. Facts may identify
private services: keep saved output in a caller-owned private location and review
it before sharing.

## Regenerate and independently verify

From this example directory, use an already-installed clean compiler at
`83928fa4538245abe33b08549da47f8cbdc50ed2` and Go 1.27.2. Setup is separate;
generation and verification disable automatic dependency/toolchain downloads.

```sh
go_tool="$(go env GOROOT)/bin/go"
python3 generate.py --compiler /path/to/pinned/gooo --go "$go_tool" \
  --out "$PWD/out/generation"
python3 verify.py --compiler /path/to/pinned/gooo --go "$go_tool" \
  --out "$PWD/out/verification"
```

The generator obtains lowering and entry identities from actual package
execution, saves its replay privately under `--out`, and invokes body-codegen on
the compiler-owned lowered bytes. It writes path-free program snapshots, exact
native projections and a complete asset manifest. Its check mode compares a
fresh route with saved sources and producer fields, normalizing only route
latency and the producing process's GOOS/GOARCH observation.

`generation.json` binds a fresh generation to the canonical
[`ports/agentmesh/service_pattern.gooo`](../../ports/agentmesh/service_pattern.gooo),
the original `impact.gooo`, their actual repository workspaces, and the preserved
Apache license. The local matcher and workspace files are exact source mirrors;
`generate.py` and its check mode refuse divergence. Public runtime bindings require
`fresh_pinned_generation`. The retained research prototype remains separate.
Program snapshots are derived compiler records, not rewritten execute/replay
receipts. Original prototype execution evidence stays separate. New generation
and workspace receipts in `--out` are current actual outputs.

The verifier freshly runs the exact extracted Go oracle for all 63 matcher
tuples, including all 12 official test cases, and for the before/after matches of
16 independently authored impact cases. It compares all three original impact
observations and separately executes/replays 63 matcher and 16 impact workspace
tuples in deterministic batches of at most 16. These are overlapping observations,
not 158 unique cases. It also checks strict transport, count/byte boundaries,
every embedded asset mutation and rehashed source/projection inconsistencies.
Pure verifier regressions additionally reject truncated arrays, altered raw facts,
Boolean-to-number substitutions, and changed activity/port/entity/field identities.
Those small historical trace fixtures test the verifier; they are not fresh
public native observations. Optimized Python is refused before invoking any tool.

Source bindings cover pinned upstream bytes, unchanged Gooo sources, actual
compiler-owned program/lowering/body records, generated code and runtime
adapter/parser/CLI source. Hash consistency assumes a trusted executable; it
does not authenticate an issuer or identify a later host binary/build toolchain.
Finite observations do not establish whole-domain compatibility or live mesh
safety.

Original upstream files are retained as non-buildable `.go.txt` archives under
[`upstream/`](upstream). Exact function extraction, regex, immutable revision and
source hashes are recorded in [`source-pin.json`](source-pin.json) and
[`oracle/extraction.json`](oracle/extraction.json). The separate oracle module has byte-exact evidence mirrors in `evidence/`;
`generate.py --check` compares every mirror with the actual oracle source. It
executes the copied upstream functions; it is not imported by the native library.
See [NOTICE.md](NOTICE.md) and the original [Apache license](LICENSE-APACHE-2.0).
