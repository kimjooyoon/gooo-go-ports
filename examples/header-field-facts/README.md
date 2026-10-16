# Header facts for automatic API checks

`app/header-field-facts:Report` imports the two Gooo header predicates and returns
twelve required fields. An API automation can require `name_valid && value_valid`
and retain the first offending byte for its diagnostic. Computing these facts
needs no per-request human approval, network call or model. The program preserves
both decoded strings and performs no trimming, normalization or HTTP operation.

## Run one external JSON request

After the [repository setup](../../README.md#run-and-edit-locally), use the pinned
Gooo `83928fa` compiler and Go 1.27.2 from the repository root:

```sh
set -eu
mkdir -p out/header-field-facts-example
go run ./cmd/header-facts-input \
  < examples/header-field-facts/request.json \
  > out/header-field-facts-example/inputs.json
./.gooo package execute --json --go "$(go env GOROOT)/bin/go" \
  --inputs out/header-field-facts-example/inputs.json header-field-facts.workspace.json \
  > out/header-field-facts-example/execution.json
./.gooo package replay --json --go "$(go env GOROOT)/bin/go" \
  --receipt out/header-field-facts-example/execution.json \
  --inputs out/header-field-facts-example/inputs.json header-field-facts.workspace.json \
  > out/header-field-facts-example/replay.json
```

Only continue after preprocessing succeeds. The command accepts exactly one JSON
object with exactly two string keys, `name` and `value`, and writes a
`gooo/body-composition-inputs/v1` document. It maps them to
`app/header-field-facts:Report.input0` and `.input1`. It rejects missing, extra or
duplicate keys (including escaped duplicates), non-string/null values, invalid
JSON or trailing content, malformed UTF-8, lone surrogate escapes, either
decoded string exceeding 1,024 UTF-8 bytes and a raw request exceeding 16 KiB.
It does not silently replace malformed text or truncate input.

Header-invalid strings are supported diagnostic inputs, so the transport accepts
empty strings, CR/LF and non-token names within the stated domain. The Gooo
activities decide the two header predicates. A transport refusal has no header
report and must not be converted into an empty or valid request. The helper
checks input structure; the Gooo source owns header computation.

`request.json` contains `{"name":"X Request","value":"ok\r\nInjected: yes"}`.
Inspect `result.runtime.traces[0].deliveries[0].actual` in the execution record:

| Field | Result | Meaning |
| --- | --- | --- |
| `name`, `value` | The two complete decoded strings | Original decoded input, including the space and CR/LF |
| `name_valid`, `value_valid` | `false`, `false` | Both contain a forbidden byte |
| `name_invalid_offset`, `name_invalid_octet` | `1`, `32` | The first name error is SP at zero-based byte offset 1 |
| `value_invalid_offset`, `value_invalid_octet` | `2`, `13` | The first value error is CR at byte offset 2 |
| `name_empty`, `name_has_uppercase` | `false`, `true` | Uppercase scanning continues even when an earlier byte is invalid |
| `value_has_obs_text`, `value_edge_whitespace` | `false`, `false` | No byte is at least 128 and neither edge is SP/HTAB |

Use a fresh output directory and replace `request.json` with `valid-request.json`
or `unicode-request.json` to compare an ordinary token and a UTF-8 field value.
The Unicode example preserves `café`, reports `value_has_obs_text: true`, and
retains the upstream value predicate. The trusted `inputs.json` fixture contains
all three demonstrations in CLI format; external callers should use the strict
preprocessor rather than construct that format from unchecked input.

The `--inputs` route observes native output; it has no reference expectation and
its receipt decision is `OBSERVED`. Saved replay binds the same workspace and
input bytes. It is separate from differential verification using independent
Go expectations.

## Domain and result contract

Both arguments must be decoded valid UTF-8 Text of at most 1,024 bytes each.
`input0` is the name and `input1` is the value. Offsets count bytes, never Unicode
code points. String echoes preserve decoded values, not the original JSON escape
spelling. The [contract](contract.json) fixes all twelve field names and IDs.

- `name_valid` and `value_valid` retain the bounded upstream header predicates.
- `name_empty` describes the empty-name special case.
- `name_has_uppercase` scans all name bytes for ASCII A–Z, even after an error.
- `value_has_obs_text` reports any value byte at least 128.
- `value_edge_whitespace` reports SP or HTAB at either edge, without removing it.
- Each `*_invalid_offset` and `*_invalid_octet` identifies the first forbidden
  byte, or is `-1` when no such byte exists. An empty name is invalid but contains
  no invalid byte, so its two witnesses are both `-1`.

The composition exposes facts for an explicit automatic policy; it does not
establish authority, field occurrence count, request binding, HTTP framing or
safe transmission. In particular, uppercase names are valid here, surrounding
SP/HTAB in values is allowed, and obs-text is descriptive. A host applying a
stricter protocol or effect must supply that policy and enforce its result.

## Reproducible finite scope

`corpus.json` is an unchanged input-only corpus: 232 name strings, 232 value
strings and 476 ordered pairs, for 940 differential cases. It includes every
single ASCII byte, empty and UTF-8 witnesses, selected long strings and exact
1,024-byte boundaries. The corpus hash is
`16d88045a78fe43f239f7b22ad12635a852529abb74733aa82b5073fdd892053`.
Go references call the pinned upstream APIs; Gooo must compute the predicates
and original diagnostics. Execute and saved replay are checked separately for
the exact typed fields, input/source/tool identities and reference agreement.

See the current CI/native reports for results on this exact source revision.
The corpus is finite: it is not all UTF-8 strings, every byte position/content
combination, arbitrary invalid UTF-8 or the whole `httpguts` package. The existing
late-CR witness and the earlier-error/uppercase witness are separate pairs;
there is no pair combining a late error followed by later uppercase. No CI
speedup or production security guarantee follows from this scope.

The [ported predicates](../../ports/net/httpguts/README.md) retain upstream
copyright, license and immutable provenance. This original importing composition
uses the repository license.
