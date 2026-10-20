# Service-pattern matches, change impact and refinement in Gooo

This example adapts one pure function from
[Google Agent Mesh](https://github.com/google/agentmesh/tree/9fd1a41a15174df0aaef601c942410c2116bff91):
`api.MatchServicePattern`. It also composes that adaptation twice in an original
Gooo activity that compares the old and candidate pattern for the **same supplied
request**. Gooo computes every match, all three impact fields and the input
grammar checks. The default Gooo workspaces and ordinary Go APIs use checked
entrypoints. Go preserves exact JSON Text, carries facts and checks source consistency.

For a request `mcp://weather`, changing `mcp://calculator` to `mcp://*` reports
`before_match=false`, `after_match=true`, `change=WIDENED`. The four request-level
states are `WIDENED`, `REVOKED`, `UNCHANGED_ALLOW` and `UNCHANGED_DENY`.
`WIDENED` describes this request only; it does not prove global grant inclusion,
find every widened request, or authorize an actual mesh call. No human approval
step or current-state/effect callback is present in these pure calculations.

For a comparison of the two whole patterns, use the original
[refinement analyzer](refinement.md). Gooo recognizes seven subset rules and
otherwise constructs up to two candidate requests, checking each against both
patterns through the imported matcher. It reports a checked widening witness or
`UNPROVEN`; absence of a witness never becomes a subset certificate. This is a
separate calculation from the four request-level impact states above.

To use those calculations in a state-changing workflow, see the original
[automatic application example](conditional-apply.md). Its Gooo activity checks
that a candidate grant is a recognized subset and preserves one configured
required request. The `apply` package owns the current state, rechecks writer
authority and revision, and commits the local policy and operation result
together. The example requires no per-change human confirmation.

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
are refused by [Gooo predicates](domain.gooo) before the core matcher or analyzer
runs, including empty/global/identical-pattern shortcuts. The full upstream parser
accepts forms outside this domain; they are not passed through a Go fallback.
This Gooo adaptation is not a complete RFC 3986 parser.

The default workspaces return `domain_status=VALID_INPUT` with an empty
`invalid_field`, or `domain_status=INVALID_INPUT` with the first invalid argument
name. On invalid input, match fields are false sentinels; impact `change` and
refinement `status` are `INVALID_INPUT`. They are not computed denial,
`UNCHANGED_DENY` or `UNPROVEN`. Require exactly `VALID_INPUT` before interpreting
semantic fields. Unknown, incomplete or contradictory records are errors.

The original low-level Gooo functions retain their canonical-input precondition
for compatibility. Use the checked entries below for direct Gooo imports.

## Use the generated ordinary Go library and stdin CLI

With Go 1.27.3 selected, run these commands from this example directory:

```sh
export GOTOOLCHAIN=go1.27.3
export GOWORK=off
mkdir -p out
go build -trimpath -o ./out/evaluate ./cmd/evaluate
./out/evaluate -mode matcher < matcher-demo.json
./out/evaluate -mode impact < impact-demo.json
./out/evaluate -mode refinement < refinement-demo.json
```

To assess a grant change from supplied facts, use `grant-update`:

```sh
printf '%s\n' '[{"before":"*","after":"mcp://*","required_type":"mcp","required_name":"weather"}]' \
  | ./out/evaluate -mode grant-update > ./out/grant-update.json
```

Each result preserves the four facts, all seven `GrantUpdateDecision` fields and
complete source bindings. This example returns `ELIGIBLE` with reason
`SUBSET_AND_REQUIRED_REQUEST_PRESERVED` and subset rule `OLD_GLOBAL`: the candidate
is a recognized subset and still permits the required request. A widening
candidate returns `REFUSED`; a candidate that removes the required request also
returns `REFUSED`. Neither result changes a grant or calls a service. See the
[application host](conditional-apply.md) for current-state and atomic-write checks.

`DecodeGrantUpdate` accepts exactly `before`, `after`, `required_type` and
`required_name` as strings. It validates only the JSON transport. The generated
Gooo activity evaluates the service-pattern domain, including empty required
components, Unicode, slash-containing names and the 128-byte limit. These inputs
produce an `INVALID_INPUT` decision, with false observation sentinels and the
first invalid field. `ELIGIBLE`, `REFUSED` and `INVALID_INPUT` all exit successfully
with a proof array; malformed JSON structure exits nonzero with no stdout.

The module is
`github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy`. An ordinary
Go host can import it and call `EvaluateMatcher(MatcherFacts)` or
`EvaluateImpact(ImpactFacts)`. The latter returns an `ImpactProof` with the exact
raw facts, three generated observations and complete source bindings. Runtime
evaluation calls the generated native Gooo projections, not a compiler or the Go
reference oracle. It makes no network calls.

`EvaluateRefinement(RefinementFacts{Before: old, After: candidate})` returns the
two raw patterns, all eight `RefinementDecision` fields and source bindings.
`DecodeRefinement` accepts exactly the `before` and `after` string fields. Both
patterns are validated inside generated Gooo before core evaluation, including
empty-candidate and global-old shortcuts. Any returned witness is checked against
the same Gooo grammar. See the [outcome contract and inclusion matrix](refinement.md).

For a typed refusal, call `EvaluateMatcherChecked`, `EvaluateImpactChecked` or
`EvaluateRefinementChecked`. Each returns `Facts`, a checked `Decision`, and
`Bindings`. A represented malformed string produces `INVALID_INPUT`; malformed
UTF-8, source-binding or record-shape failures return an error. The existing
matcher/impact/refinement `Evaluate*`, `Validate*`, `Decode*` and CLI modes keep invalid-input errors and their
normal-input proof shapes while using generated Gooo checks. No production Go
regex duplicates the grammar.

The CLI accepts a JSON array of 1..1024 exact string-only fact objects, at most
1 MiB. Extra/duplicate keys (including escaped duplicates), missing fields,
`null`, wrong types, trailing JSON, malformed UTF-8 and unpaired surrogate
escapes are refused by structural decoding. Valid Unicode, paired surrogate
escapes, escaped controls and oversized components reach the Gooo grammar check
and are refused there. In `grant-update` mode that refusal is an `INVALID_INPUT`
proof; the other three CLI modes return an error. Each fact object is limited to
4,096 serialized bytes.
Every member is decoded and evaluated before the first output write. A structural
or evaluation error in a later member emits no result; in `grant-update` mode a
represented invalid-domain input remains a normal proof-array member. Facts may identify
private services: keep saved output in a caller-owned private location and review
it before sharing.

## Direct Gooo workspace migration

The existing three workspace filenames now select checked entries. This is an
intentional output-schema change for direct workspace callers. The native
`Evaluate*` and CLI normal-input proof shapes remain compatible.

| Default root workspace | Entry (`package:activity`) | Output |
| --- | --- | --- |
| `agentmesh-matcher.workspace.json` | `gooo/service-policy/checked-matcher:MatchServicePatternChecked` | 3 fields: domain status, invalid field, matched |
| `service-grant-impact.workspace.json` | `gooo/service-policy/checked-impact:CompareServiceGrantChecked` | 5 fields: domain status, invalid field, original 3 impact fields |
| `service-grant-refinement.workspace.json` | `gooo/service-policy/checked-refinement:AnalyzeServiceGrantChecked` | 10 fields: domain status, invalid field, original 8 refinement fields |

Input ports are the full entry in the table plus `.input0`, `.input1`, etc.; the
output port is the entry itself. Matcher inputs are `(pattern, req_type, req_name)`,
impact inputs `(before, after, req_type, req_name)`, and refinement inputs
`(before, after)`. Validation follows that order and reports the first failure.
The activity IDs are `gooo://service-policy/checked-{matcher,impact,refinement}/evaluate/v1`;
the record IDs use the same prefix with `/decision/v1`.

For example, a raw refinement input with `before="mcp://a/path"` and `after=""`
returns this record before the core's empty-candidate shortcut can run:

```json
{"domain_status":"INVALID_INPUT","invalid_field":"before","status":"INVALID_INPUT","rule_id":"","supported_subset":false,"has_witness":false,"witness_type":"","witness_name":"","before_match":false,"after_match":false}
```

Use the pinned compiler and Go 1.27.3 from the repository root:

```sh
go_tool="$(go env GOROOT)/bin/go"
./.gooo package execute --json --go "$go_tool" \
  --cases examples/agentmesh-service-policy/receipts/checked-matcher-seed-cases.json \
  agentmesh-matcher.workspace.json
./.gooo package execute --json --go "$go_tool" \
  --cases examples/agentmesh-service-policy/receipts/checked-impact-seed-cases.json \
  service-grant-impact.workspace.json
./.gooo package execute --json --go "$go_tool" \
  --cases examples/agentmesh-service-policy/receipts/checked-refinement-seed-cases.json \
  service-grant-refinement.workspace.json
```

Compatibility is explicit: `agentmesh-matcher-raw.workspace.json`,
`service-grant-impact-raw.workspace.json` and
`service-grant-refinement-raw.workspace.json` preserve the original Boolean,
3-field and 8-field results. They require already-canonical input; malformed-input
safety does not apply to these raw routes. The corresponding local mirrors use
`matcher-raw-workspace.json`, `impact-raw-workspace.json` and
`refinement-raw-workspace.json`. Previous contracts are pinned at
[matcher](https://github.com/kimjooyoon/gooo-go-ports/blob/0c6a55f7021128358be05615f033c4f8c7678bef/agentmesh-matcher.workspace.json),
[impact](https://github.com/kimjooyoon/gooo-go-ports/blob/0c6a55f7021128358be05615f033c4f8c7678bef/service-grant-impact.workspace.json)
and [refinement](https://github.com/kimjooyoon/gooo-go-ports/blob/0c6a55f7021128358be05615f033c4f8c7678bef/service-grant-refinement.workspace.json).
Historical receipts keep their original identities and do not establish checked execution.

## Regenerate and independently verify

From this example directory, use an already-installed clean compiler at
`83928fa4538245abe33b08549da47f8cbdc50ed2` and Go 1.27.3. Setup is separate;
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
Regeneration preserves an existing public body-codegen record byte-for-byte
when only those two observational fields differ. Its original metadata stays
intact; the fresh command output remains separately available under `--out`.

`generation.json` binds a fresh generation to the canonical
[`ports/agentmesh/service_pattern.gooo`](../../ports/agentmesh/service_pattern.gooo),
the original `impact.gooo` and `refinement.gooo`, the new domain predicates and
checked entries, their exact default/raw repository workspaces, and the preserved
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
The refinement corpus adds 101 cases covering all 36 family combinations,
conditional inclusion rules, both witness seeds and boundary inputs. All eight
decision fields are compared through native execution and seven workspace/replay
batches. The extracted upstream oracle independently checks both matches for
each of the 63 emitted witnesses. These 126 oracle observations and the repeated
workspace cases are checks on the same corpus, not extra independent cases.
The migrated default workspace observations also compare every domain field.
A separate checked corpus exercises malformed-pattern cross-products, shortcut
traps, first-invalid ordering, Unicode and independent length bounds directly
through Gooo and native checked APIs. Predicate observations include all 128 ASCII
values in four positions and explicit boundary cases, compared with a frozen
pre-migration Go reference used only by verification. Native validation is not
used as a prefilter for direct Gooo malformed-input cases.

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
