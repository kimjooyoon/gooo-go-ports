# Tool definitions behind a stable alias

Development example; not a released API or a completed hosted-CI result.

A saved tool can keep its exposed alias and declared origin while its description,
input schema, annotations or host policy changes. Comparing the alias alone cannot
show those changes. This example composes the existing origin and recorded-catalog
activities with an original Gooo activity that keeps the following observations
separate:

- the declared origin and the completeness of the saved collection;
- the supplied tool-definition projection;
- the supplied annotations, treated as claims rather than proven capabilities;
- the recorded authorization context and the host-supplied policy revision.

The policy lives in `ports/tool-origin/definition_binding.gooo`, imported through
`definition-binding.workspace.json`. Go supplies an independent finite reference
and strict transport projection. The host does not reimplement the new decision.

## Inputs from saved MCP frames

The sibling `recorded-catalog-comparison` module provides the development function
`ProjectDefinitionInputsJSON(raw, beforePolicy, afterPolicy)`. It accepts the same
bounded saved-observation JSON as `CompareJSON` and returns three ordered inputs
for `gooo/tool-origin/definition-binding:AssessDefinitionBinding`:

1. the existing origin columns;
2. the existing recorded-collection columns;
3. definition, profile, annotation-claim and policy columns bound to both contexts.

It uses the same strict transport parser: duplicate decoded keys, invalid UTF-8
or surrogate escapes, mismatched RPC IDs, unsupported frames and exceeded limits
are refused before projection. There are no live tool calls or effects. Returned
inputs are data for the Gooo activity, not an execution receipt or a decision.
The existing no-argument `compare` CLI continues to report the origin/context
result. `compare --definitions --before-policy REV --after-policy REV` additionally
evaluates these bindings using Gooo-generated Go. Omitting policy revisions keeps
them unobserved. The new API is `CompareDefinitionJSON(raw, beforePolicy, afterPolicy)`.

Each projected row retains its capture/page/tool index, original response hash,
and declared origin tuple. Parallel columns preserve the existing row order,
including non-target tools. A new context ID binds the raw input hash, projection
profile, columns and supplied policy revisions. Empty policy text means unobserved.
Neither a context ID nor a checksum authenticates the saved observation.

## Exact definition profile

Profile `gooo/mcp-tool-definition-json/v1` uses Go 1.27.3 `encoding/json` with
`UseNumber`, followed by `Marshal`. Object keys are sorted and JSON whitespace is
normalized. Array order, numeric token spellings and unknown descriptor fields
are preserved. This profile is not RFC 8785 or a proof of semantic equivalence.
For example, `1`, `1.0` and `1e0` intentionally produce different definition IDs.

The definition projection contains every field of the selected tool object except
`annotations`. The annotation projection contains `{"present":false}` when absent,
or `{"present":true,"value":...}` when present; absent and explicit `null` remain
different. Descriptor schema semantics and annotation truth are not established
by this transport projection. Instruction-like text remains opaque data.

The prefixed SHA-256 IDs hash these exact byte sequences:

```
profile + NUL + "definition"  + NUL + encoded descriptor
profile + NUL + "annotations" + NUL + encoded annotation-presence object
```

Literal vectors for `{"name":"search"}` without annotations are fixed in
`definition_test.go` in the sibling module.

## Gooo decisions

| Observation | Overall state |
| --- | --- |
| Invalid/unbound columns or malformed supplied digest | `INVALID_INPUT` |
| Origin not unchanged | `ORIGIN_NOT_UNCHANGED` |
| Recorded authorization context changed | `REEVALUATE_AUTHORIZATION` |
| Collection not established | `UNASSESSED` |
| Known definition difference | `REEVALUATE_DEFINITION` |
| Known annotation-claim difference | `REEVALUATE_CAPABILITY` |
| Known policy revision difference | `REEVALUATE_POLICY` |
| Unsupported profile or required binding not observed | `UNASSESSED` |
| All supplied bindings match | `BINDINGS_MATCH` |

Rows are evaluated in this precedence. A known definition difference remains a
reason to reevaluate even if an unrelated capability or policy is unobserved.
A known policy revision difference also remains visible when a definition profile
is unsupported or unobserved. The individual relation fields retain those missing observations. Unestablished
origin/collection context still prevents a complete binding result.

Every result keeps `behavior_equivalence_proven`, `execution_authorized` and
`live_effect_performed` false. `BINDINGS_MATCH` reports supplied equality only;
it is an input to the host's existing validation, not an authorization grant.
There is no new human approval lane.

## Independent reference and current evidence

From this directory, with Go 1.27.3 selected:

```sh
go -C reference run . -input "$PWD/reference-inputs.json" > /tmp/definition-binding-cases.json
```

The 60 synthetic requests have independent literal state/relation anchors. The Go
reference uses a separate priority table and hex decoder to construct complete
expected results. Existing parent origin/context observations are reused literal
fixtures; this is not a second independent oracle for those imported activities.

Local development verification used compiler
`205896a413e5dea92366309c35ae85e542d055f6` and Go 1.27.3. All 60 supplied cases
matched all 32 typed fields in native execution and their own saved replay, in
eight batches of at most eight cases. Twelve additional synthetic saved-frame
projections also matched the independent reference in native execution and replay.
These sets are separately counted; they are not production incidents or external
adoption. Original failed observations are retained outside the source tree.

`probes/nonzero-counted-loop.gooo` preserves an encountered syntax boundary.
At that compiler pin declaration checking succeeds, while `body-codegen --json
--activity Probe` refuses with `for must use the same named counter from zero`.
The implementation uses a supported zero-based bounded loop instead. Declaration
checking is not native execution.

## Reproduce the complete verification

From the repository root, with the pinned compiler already built:

```sh
python3 -I -B examples/tool-definition-binding/verify.py \
  --compiler "$PWD/.gooo" --go "$(go env GOROOT)/bin/go" \
  --go-cache "$(go env GOCACHE)" --module-cache "$(go env GOMODCACHE)" \
  --out /absolute/fresh/directory
```

The output directory must not exist and must be outside the checkout. The runner
verifies the exact compiler/toolchain identity, regenerates the independent Go
expectations, runs eight bounded native/replay kernel batches, regenerates all
five committed assets, and verifies their actual PROV-O derivation graph. It then
runs API/CLI race tests, compares 12 CLI reports with independent Go expectations,
and executes their projected inputs with the original Gooo source and saved
replay. Runtime CLI calls use an empty PATH and no GOROOT. No engine build, model,
service token or live collection is needed.

Receipt mutations for wrong parent, source, third input, field ID, authority and
input-only evidence are refused by both reader and generator. Coverage gaps and
metadata path escape are also refused. Original failures and owned process
closure remain visible in `summary.json`; no failed attempt is retried internally.

`generate.py --index /absolute/generation-input.json --gofmt /absolute/gofmt --check`
checks byte-for-byte regeneration. To inspect new assets without editing the
checkout, replace `--check` with `--out /absolute/fresh/assets`. The generated
projection changes only its package token; API types and record bindings come
from the compiler's typed plan. The independent corpus is copied mechanically
for the installable module's tests. PROV-O records the actual generator, helper,
selected pin fields, source, case partitions, original receipt bodies and Go tools.
This makes future regeneration dependencies explicit without claiming their
checksums authenticate a producer.

The workflow runs a fast Go consumer preflight and retains the full source-bound
verification/evidence upload for executable changes. Hosted checks and release
acceptance are still pending. Do not treat local observations as source-wide
safety, authenticated collection, live semantic equivalence or effect authority.
