# Native final-marker report

Find the final repeated diagnostic or configuration marker while retaining the
first position, the non-overlapping count and the exact supplied strings. This
Gooo program imports `strings.Index`, `strings.LastIndex` and `strings.Count` and
assembles an eight-field `FinalMarkerReportV1` record in Gooo.

For `"aaaaa"` and `"aa"`, the first position is `0`, the last position is `3`,
and the non-overlapping count is `2`. The last position includes overlapping
matches; it cannot be reconstructed from the count alone.

## Run the supplied inputs

Use the repository's [local setup](../../README.md#run-and-edit-locally) with
the clean Gooo compiler at `83928fa4538245abe33b08549da47f8cbdc50ed2`
(`0.6.32-dev`) and Go **1.27.2**. From the repository root:

```sh
export GOTOOLCHAIN=go1.27.2 GOWORK=off
NATIVE_GO="$(go env GOROOT)/bin/go"
mkdir -p out/literal-final-marker-report
./.gooo package execute --json --go "$NATIVE_GO" \
  --inputs examples/literal-final-marker-report/inputs.json \
  literal-final-marker-report.workspace.json \
  > out/literal-final-marker-report/execution.json
./.gooo package replay --json --go "$NATIVE_GO" \
  --receipt out/literal-final-marker-report/execution.json \
  --inputs examples/literal-final-marker-report/inputs.json \
  literal-final-marker-report.workspace.json \
  > out/literal-final-marker-report/replay.json
```

The 26 supplied pairs include overlap, repeated markers, Unicode byte offsets,
empty strings, NUL/CRLF and exact 1,024-byte boundaries. Returned records are at
`result.runtime.traces[].deliveries[].actual`. Input-only observations report
`OBSERVED`; their counters do not claim exhaustive validation.

For your own pair, replace the input path in both commands with a file like:

```json
{
  "schema": "gooo/body-composition-inputs/v1",
  "inputs": [
    {
      "app/literal-final-marker-report:Report.input0": "aaaaa",
      "app/literal-final-marker-report:Report.input1": "aa"
    }
  ]
}
```

The returned record for this pair is:

```json
{
  "text": "aaaaa",
  "needle": "aa",
  "first_byte_offset": 0,
  "last_byte_offset": 3,
  "found": true,
  "non_overlapping_count": 2,
  "text_byte_length": 5,
  "needle_byte_length": 2
}
```

Keep the original input file, source files, workspace and both receipts together
for saved replay. Replay re-evaluates the supplied input and checks its bound
program; it does not establish the truth or origin of diagnostic text.

## Domain and compatibility

Each argument is decoded valid UTF-8 Text of at most **1,024 bytes**. Larger
arguments are refused without truncation. Offsets count UTF-8 bytes, not Unicode
code points, graphemes or UTF-16 units. Search is literal and case-sensitive.
No normalization occurs. Malformed JSON wire bytes or surrogate sequences can
be replaced by the native decoder, so this is not arbitrary-byte Go string
compatibility or preservation of malformed transport.

An empty phrase has first position `0`, last position equal to the text's byte
length, and count equal to one plus its number of Unicode code points. An
absent nonempty phrase has both positions `-1`, `found=false`, and count `0`.

The separate versioned entry is
`gooo://go-ports/examples/literal-final-marker-report/v1/report`; the result is
`gooo://go-ports/examples/literal-final-marker-report/v1/result`. The existing
[seven-field Report](../literal-text-report/README.md), its entry identities,
source bytes, original inputs and historical evidence remain unchanged.

## Independent finite validation

The normal native verifier registers `go/strings:LastIndex` and
`app/literal-final-marker-report:Report` as separate suites. Go supplies the
independent `strings.LastIndex`, `strings.Index` and `strings.Count` reference
outputs; it does not implement the Gooo program. Every selected output field,
input, entry/source/producer binding and outer saved replay is checked through
the existing finite receipt path. That path's documented inner replay-hash and
typed-plan `NOT_CHECKED` limits still apply.

The LastIndex adaptation uses a guarded forward scan with a literal bound of
1,024 positions, retaining the last match. It preserves the bounded API result
without copying Go's optimized reverse traversal. See the [port](../../ports/strings/last_index.gooo),
[pinned source metadata](../../upstream.json), and Go's
[BSD license](../../third_party/GO-LICENSE). Finite validation is not a claim of
all-input correctness, whole-package compatibility, performance or external use.
