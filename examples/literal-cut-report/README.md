# Native literal Cut report

Supply text and a literal separator or affix. The importing Gooo `Report` calls
`Cut`, `CutPrefix` and `CutSuffix` on the same original inputs and returns eleven
required fields: both inputs, every returned component and flag, and both UTF-8
byte lengths. For `one::two::three` and `::`, `Cut` returns `one`,
`two::three`, `true`; the prefix and suffix operations both return the unchanged
text and `false`. They remove an affix only at its corresponding edge.

An empty separator or affix succeeds, including two empty inputs. A missing
separator returns the whole text as `before`, an empty `after`, and `false`.
Literal matching preserves case, whitespace, NUL, CR/LF, BOM, combining marks and
composed/decomposed Unicode. It does not trim, normalize or interpret a regex.

Both arguments must be explicitly supplied decoded valid UTF-8 Text of at most
**1,024 bytes each**. Go permits arbitrary byte strings and has no such limit;
those wider inputs are outside this adaptation. The compiler's JSON decoder
may replace malformed wire UTF-8 or lone surrogates, so this contract does not
promise original malformed JSON bytes or classify replacement as a refusal.
Go multiple returns become `CutResult{before,after,found}` and
`AffixResult{text,found}` records; there is no generic Go tuple ABI.

## Execute supplied inputs and saved replay

Use the repository's existing clean compiler pin
`205896a413e5dea92366309c35ae85e542d055f6` (Gooo `0.6.37-dev`) and Go
**1.27.3**, as described in the [setup guide](../../README.md#run-and-edit-locally).
Run from the repository root. These commands assume the matching compiler and
toolchain are already installed; they do not download or upgrade either.

```sh
export GOTOOLCHAIN=local
export GOWORK=off
NATIVE_GO="$(go env GOROOT)/bin/go"
mkdir -p out
mkdir out/literal-cut-report
./.gooo package execute --json --go "$NATIVE_GO" \
  --inputs examples/literal-cut-report/inputs.json \
  literal-cut-report.workspace.json > out/literal-cut-report/execution.json
./.gooo package replay --json --go "$NATIVE_GO" \
  --receipt out/literal-cut-report/execution.json \
  --inputs examples/literal-cut-report/inputs.json \
  literal-cut-report.workspace.json > out/literal-cut-report/replay.json
```

Use a new output directory and keep the original inputs, all source files,
workspace manifest, compiler identity and both receipts. Input-only execution
reports `OBSERVED` and `finite_total=0`; that counter is not an independent Go
comparison score. A saved-report viewer is not a generated-program replay.

The supplied file contains the original **32 authored input pairs**, including
empty, miss, prefix/suffix/middle, repeated/overlapping separators, exact
1,024-byte boundaries, NUL, CRLF, BOM and multibyte/combining text. To supply a
different pair, use the same exact input envelope and replace both `--inputs`
paths:

```json
{
  "schema": "gooo/body-composition-inputs/v1",
  "inputs": [{
    "app/literal-cut-report:Report.input0": "one::two::three",
    "app/literal-cut-report:Report.input1": "::"
  }]
}
```

The report for that tuple is:

```json
{
  "text": "one::two::three",
  "needle": "::",
  "cut_before": "one",
  "cut_after": "two::three",
  "cut_found": true,
  "prefix_after": "one::two::three",
  "prefix_found": false,
  "suffix_before": "one::two::three",
  "suffix_found": false,
  "text_byte_length": 15,
  "needle_byte_length": 2
}
```

## Independent original-Go checks

The existing full verifier appends four suites: `Cut`, `CutPrefix`, `CutSuffix`
and this importing `Report`, with the same 32 original pairs in each. Go calls
the pinned standard functions independently for each suite and compares every
returned field. Go does not implement the candidate path: the authored Gooo
bodies make the split and affix decisions, and the importing Gooo program builds
the report.

```sh
go run ./cmd/verify --compiler ./.gooo --workers 2 \
  --out out/cut-full-verification \
  --timing-out out/cut-full-verification/timing.jsonl
```

This is the complete native corpus, including all previous suites and exhaustive
checks, rather than a short example-only command. Its new directory retains
case documents, exact manifests and sources, compiler identity, native execution
and saved replay for each byte-bounded batch, with the final summary and timing
footer. Existing 62 suites, 159,781 cases and 2,332 batch boundaries remain a
prefix. The actual serialized-partition check gives **1/1/1/2 batches** for
Cut/CutPrefix/CutSuffix/Report respectively. CI fixes the complete required roster
at **66 suites, 159,909 case records and 2,337 replay batches**, with exactly
**2,471 timing events** and a complete failure-free footer. A partition check
measures the case-document boundaries; it does not execute generated programs
or establish that the new public entry has completed native CI and saved replay.

The generic finite verifier checks original input values and order, required
field IDs/order/values, workspace/source/producer/toolchain identities, generated
bytes and the outer saved-receipt binding. Its inner `prior_result_sha256`, full
generation-report provenance and composition/runtime typed-plan equality are
**NOT_CHECKED**. The earlier private input-only pilot used its original source
and a separate observer; it does not supply fresh acceptance for this renamed
public entry, and that observer is not duplicated into this package.

The required malformed Text gate also includes both arguments of these four
entries. Its invalid-request counts remain separate from the 128 new positive
case records. A missing runtime is `NOT_REPORTED`, never an invented zero-run
counter. Existing literal-search and Node/Wasm programs are unchanged.

## Source and license

The original [Go 1.27.2 strings functions](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/strings/strings.go)
delegate to [stringslite](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/internal/stringslite/strings.go).
The immutable source SHA256 values are retained in
[upstream.json](../../upstream.json) and [the Gooo port](../../ports/strings/cut.gooo).
Go copyright and its BSD license are preserved in that header and
[GO-LICENSE](../../third_party/GO-LICENSE).

Import all three string source files in order: `search.gooo` provides shared
declarations, `literal.gooo` provides Gooo `Index`, and `cut.gooo` implements the
three record-returning APIs. The new isolated workspace contains only these
sources and this consumer. It does not add new records or sources to any prior
suite's workspace. Finite observations establish neither all allowed strings,
whole-library compatibility, deployment safety, performance nor user demand.
