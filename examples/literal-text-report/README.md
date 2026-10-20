# Native literal-text report

Supply a small diagnostic or configuration snippet and a literal phrase. This
importing Gooo consumer returns the first UTF-8 byte offset, the number of
non-overlapping matches, both original inputs and their byte lengths. Save the
checked JSON report to retain the exact text, phrase and program identities.

Search and counting run in `ports/strings/literal.gooo`. The Go checker calls
the independent, pinned `strings.Index` and `strings.Count` references and checks
the native receipts; it does not provide the consumer's implementation.

Each decoded input is valid UTF-8 Text of at most **1,024 bytes**. The native
input route rejects larger arguments, malformed UTF-8 and unpaired JSON
surrogate escapes. The observed domain is scalar Text, not arbitrary Go byte
strings or preservation of malformed JSON transport. The search is
literal and case-sensitive, with no normalization, regex or grapheme handling.

Go's empty-phrase behavior is preserved: `Index(text, "")` is `0`, while
`Count(text, "")` is one plus the number of Unicode code points. For empty text
and empty phrase, the count is `1`. For `"aaaa"` and `"aa"`, the count is `2`;
for `"ababa"` and `"aba"`, it is `1`, because occurrences do not overlap.

## Run supplied inputs and export a checked report

Use the clean Gooo **0.6.37-dev** compiler at source
`205896a413e5dea92366309c35ae85e542d055f6`, with the Go **1.27.3** build identity
required by the current checkout. The repository's [setup guide](../../README.md#run-and-edit-locally) explains how to
obtain that pin. These commands assume `./.gooo` is already installed and the
matching Go toolchain is available. Run from the repository root and keep the
source files unchanged through execution, replay and checking.

```sh
export GOTOOLCHAIN=go1.27.3
export GOWORK=off
NATIVE_GO="$(go env GOROOT)/bin/go"
mkdir -p out/literal-text-report
./.gooo version --build --json > out/literal-text-report/compiler.json
./.gooo package execute --json --go "$NATIVE_GO" \
  --inputs examples/literal-text-report/inputs.json \
  literal-text-report.workspace.json > out/literal-text-report/execution.json
./.gooo package replay --json --go "$NATIVE_GO" \
  --receipt out/literal-text-report/execution.json \
  --inputs examples/literal-text-report/inputs.json \
  literal-text-report.workspace.json > out/literal-text-report/replay.json
go run ./cmd/check-literal-text-report --snapshot . \
  --inputs examples/literal-text-report/inputs.json \
  --execution out/literal-text-report/execution.json \
  --replay out/literal-text-report/replay.json \
  --compiler-build out/literal-text-report/compiler.json \
  --out out/literal-text-report/checked-report.json
```

The checker creates a new output file and refuses to overwrite an existing one.
Keep the source files, manifest, original input file, both receipts and compiler
identity with the checked report if you need to verify it later.

To use your own inputs, save a file with this shape and replace the `--inputs`
path in all three commands:

```json
{
  "schema": "gooo/body-composition-inputs/v1",
  "inputs": [
    {
      "app/literal-text-report:Report.input0": "가x가",
      "app/literal-text-report:Report.input1": "x"
    }
  ]
}
```

The returned record for that input is:

```json
{
  "text": "가x가",
  "needle": "x",
  "first_byte_offset": 3,
  "found": true,
  "non_overlapping_count": 1,
  "text_byte_length": 7,
  "needle_byte_length": 1
}
```

Offsets and lengths count UTF-8 bytes. The importing consumer's entry is
`app/literal-text-report:Report`, with the text at `.input0` and phrase at
`.input1`. The native receipt retains returned records at
`result.runtime.traces[].deliveries[].actual`. Input-only execution and saved
replay report `OBSERVED` with `finite_total=0`. The separate checker checks every
returned record against Go, preserving those original receipt counters.

## Search in the browser

The [literal search page](https://kimjooyoon.github.io/gooo-go-ports/literal-search.html)
runs this same, unchanged Gooo consumer as a separate precompiled Go WebAssembly
module. Supply a text and phrase in Text mode, or enter each as one JSON string.
Pasting or dropping text captures its value before the textarea can normalize
CR or CRLF. The escaped previews show the exact captured strings.

The browser UTF-8 binding turns unpaired UTF-16 surrogates into U+FFFD. Native
comparison inputs encode this bound scalar Text; the compiler rejects raw
unpaired JSON escapes. Both the captured and bound strings remain visible and
are preserved in the downloaded observation. BOM, NUL, CR/LF, combining marks
and composed/decomposed Unicode remain distinct; no trimming or normalization
occurs. Each bound argument must fit in 1,024 UTF-8 bytes. Larger inputs are
rejected as a whole rather than shortened. This does not preserve a file's
invalid UTF-8 bytes or malformed JSON wire encoding.

The first position is a **UTF-8 byte offset**, not a JavaScript UTF-16 character
index. With `가x가` and `x`, it is `3`. The count does not overlap matches: with
`aaaaa` and `aa`, it is `2`. An empty phrase returns position `0` and one plus
the Unicode code-point count; an empty text and phrase therefore return count `1`.

Text and results stay in the tab. The page loads static program assets, sends
neither input to a server, and does not put them into a share URL. Editing an
input clears earlier results and its download. Failed, cancelled, timed-out or
superseded work cannot present a partial result as completed.

**Download observation JSON** preserves the captured input spelling, original
UTF-16 units, bound inputs, all seven returned fields and the program hashes and
pins. Its schema is `gooo/literal-search-observation/v1` and its kind is
`browser-observation`. It is unsigned and is not a native execution receipt,
saved native replay, signed attestation or Go-reference comparison. Sharing the
downloaded file is a separate action controlled by its recipient.

The browser build checks the original 24 input tuples and a separate 24-tuple
boundary roster with the independent pinned Go reference, native execution and
the existing strong saved-replay checker. The supplemental roster covers UTF-16
surrogate replacement, UTF-8 byte positions, exact 1,024-byte inputs, BOM,
controls, non-overlap and Unicode distinctions. It does not alter the 62-suite
native verification corpus. The browser smoke then exercises the actual shipped
Wasm callbacks for all 48 tuples plus six outside-domain, 24 wrong-type and
three arity rejections. Unit tests separately check input capture, Worker
program/request identity, stale-result prevention and failures.

## Source and validation scope

The original contracts are [Go strings.Index and Count](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/strings/strings.go),
with supporting [stringslite.Index](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/internal/stringslite/strings.go)
and [utf8.RuneCountInString](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/unicode/utf8/utf8.go).
Their fixed source hashes are in [upstream.json](../../upstream.json) and the
port header. Go's copyright and BSD license remain in the port and
[GO-LICENSE](../../third_party/GO-LICENSE).

The two ports use a forward scan with a literal bound of 1,024 and guard each
slice or byte access. Count keeps a separate next-match threshold to preserve
non-overlap without altering the loop counter. For an empty phrase, counting
non-continuation bytes equals Unicode code-point counting within the valid
UTF-8 domain. Include both `ports/strings/search.gooo` (shared declarations) and
`ports/strings/literal.gooo` when importing these ports elsewhere.

The isolated pilot compares the **24** supplied input tuples with independent
pinned Go outputs and saved native replay. They cover empty text and phrase,
overlap, absence, a longer phrase, NUL, CR/LF, multibyte offsets, canonically
distinct Unicode and exact 1,024-byte text/phrase boundaries. Checker rejection
tests cover altered inputs, source/body/export identities, producer/toolchain
metadata, generated bytes, native process completion, field order/values and
saved-receipt binding. Oversized inputs are outside-domain rejection observations.

The pilot checker repair has independent acceptance. Its pinned input-only
profile checks the original typed-result replay hash, full generation report
provenance and composition/runtime typed-plan identity, including normal timing
and fractional metadata. See the [checker format limits](../../cmd/check-literal-text-report/README.md).

The integrated catalog isolates Index/Count to the shared declarations and
literal port; Report adds only this importing consumer. The full verifier
contains **9,313 unique Text pairs per activity**, with the same ordered roster
for Index, Count and Report. It includes the supplied 24 pairs, 85×85 short
structured strings, every byte position 0..1,023, both-argument length limits,
Unicode/control/escaping and non-overlap anchors, and fixed PCG samples.
Actual serialized JSON plus LF derives **137/137/281** batches respectively,
each at most 30,000 bytes and 128 rows. The complete authored roster is
62 suites, 159,781 cases and 2,332 batches, preserving the original 59 suites,
131,842 cases and 1,777 batch files. These source-derived counts specify the
required finite coverage; inspect the current CI/native reports for completed
execution results.

The finite `--cases` verifier retains original input/source/producer/Go-output
and exact outer saved-receipt checks. Its inner prior-result hash, full generation
provenance and typed-plan binding are **NOT_CHECKED**. The separately required
input-only 24-pair strong checker has those bindings for its reviewed profile;
it complements the three full finite suites and must not serialize their
finite receipt shapes blindly.

The required malformed-input gate sends 84 framework requests through the real
CLI, covering both arguments for Index, Count and Report. Rejection observations
are separate from Go comparisons. An absent runtime stage/counter is
`NOT_REPORTED`, never inferred as zero; malformed wire UTF-8/surrogate replacement
is not claimed as rejection or original-byte preservation.

The finite corpus and pilot do not establish every supported input, whole-package
compatibility, performance or customer adoption. Completed execution evidence
must identify its exact source and compiler pins.
