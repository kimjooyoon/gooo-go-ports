# Recorded observation diagnostics in Gooo

This development example explains missing context in saved MCP catalog comparisons.
The original comparison stays unchanged. A separate report identifies the source
field, supplied value, availability, comparison reference and next action, using
rules in `ports/tool-origin/observation_diagnostics.gooo` compiled to Go.

This source is under development. It has no release or source-wide acceptance yet.
The initial internal use is a historical Compiler Explorer integration task, not
external adoption. It requires no human approval lane, model, token or live service.

## Use the generated consumer

Build the existing `examples/recorded-catalog-comparison/cmd/compare` module from
this source, then pass the same saved input that the existing comparison accepts:

```sh
compare --definitions --diagnostics < saved-observations.json > diagnostics.json
```

The Go APIs are `ProjectObservationInputsJSON` and
`CompareDefinitionDiagnosticsJSON`. Optional policy revisions remain the existing
`--before-policy` and `--after-policy` text arguments. With no `--diagnostics`, the
existing comparison report and behavior remain unchanged.

Keep the original input file with the report. Input/frame hashes cannot reconstruct
those bytes. The report includes the original definition comparison separately;
none of its observations authenticate a producer, prove behavior equivalence,
authorize execution or perform an effect.

## Interpret the source and availability

`source_document=RECORDED_INPUT` addresses a JSON Pointer in the original input.
`SUPPLIED_POLICY` addresses the report projection's separate `supplied_policy`
object (`/before_revision` or `/after_revision`). These are normalized API argument
values, not additional fields allowed in the strict stdin schema. String APIs do
not establish whether a historical CLI option was present; retain original argv
separately when that fact matters.

The original Gooo activity supports `SUPPLIED`, `UNOBSERVED` and `LEGACY_TEXT`
value declarations. The saved-JSON adapter copies text with `LEGACY_TEXT`; Gooo
resolves empty legacy text to `UNOBSERVED`, without inserting an invented ID.
Explicit `UNOBSERVED` with nonempty text and `SUPPLIED` with empty text are invalid
declarations. A nonempty literal such as `UNKNOWN` is supplied opaque text, never
an authenticated revision or a replacement created by this consumer.

| Field result | Meaning | Next action |
| --- | --- | --- |
| `UNASSESSED` / required observation unavailable | A required revision/context value was not recorded | Obtain an original observation or recollect within the host's authorized scope; otherwise retain unassessed |
| `UNASSESSED` / comparison observation unavailable | A supplied value lacks the comparison observation | Obtain that observation; do not compare empty strings as evidence of equality |
| `KNOWN_MISMATCH` | Two supplied context values differ | Recollect unstable same-capture context or reevaluate a between-catalog change |
| `INVALID_DECLARATION` | Inconsistent kind, document, pointer, availability or comparison declaration | Correct the declaration; this is distinct from an unrecoverable historical gap |
| `OPTIONAL_UNOBSERVED` | An optional timestamp/policy revision is unavailable | Preserve that absence |
| `SUPPLIED_ONLY` or `SAME_SUPPLIED` | A supplied claim, optionally compared with another supplied claim | Continue the original domain validation; no authority or behavior proof follows |

Timestamp values are retained without equality comparisons: time normally changes
between capture phases. End/page context values compare against begin in the same
side/capture. After catalog authorization and policy values reference their before
counterparts. Known mismatches remain visible when another field is unobserved.
Retrospectively copied protocol labels remain retrospective claims; repeating an
initialize version does not prove per-phase/page capture-time context.

## Bounds and evidence

The strict transport retains its existing 1 MiB input, 4 MiB report, eight
connections per side and sixteen pages per capture limits. The projection includes
all fields, including supplied and optional values, up to 1,444 field observations.
The generated source uses twelve typed input fields and twenty-one output fields.
It does not add a collector, network call, model, dependency download or compiler at
consumer runtime.

The finite independent Go corpus has 51 cases, including missing required facts,
optional facts, explicit availability contradictions, JSON Pointer escapes,
source-document mismatch and known changes. Original Gooo execution and each
record's own saved replay remain separate from declaration checking. Four
synthetic saved-frame consumers supply 136 field cases. An actual historical
internal task's 34 fields and twenty missing paths are retained separately from
these fixtures and from any external adoption claim.

Reproduce complete local verification with the pinned compiler already built:

```sh
python3 -I -B examples/observation-diagnostics/verify.py \
  --compiler "$PWD/.gooo" --go "$(go env GOROOT)/bin/go" \
  --go-cache "$(go env GOCACHE)" --module-cache "$(go env GOMODCACHE)" \
  --out /absolute/new/evidence-directory
```

The verifier checks source/typed-field/producer identities, all native and saved
replay cases, exact generation of five assets, generation-scoped PROV-O file
occurrences, consumer race tests, standalone CLI reports and malformed evidence
controls. Runtime CLI tests use empty PATH and no GOROOT. Shared caches are
optional and disclosed; this example makes no cold-cache or CI speedup claim.
