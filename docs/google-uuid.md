# Google UUID Validate

This function-level Gooo port follows Google's `github.com/google/uuid` v1.6.0
at revision `0f11ee6918f41a04c201eceeadf612a377bc7fbc`.
It preserves the four formats and the observable `Validate` return behavior over
valid UTF-8 Text inputs of at most 1,024 bytes. The version is a fixed source
choice, not a claim that v1.6.0 is the newest release.

## Public API mapping

Package path: `go/github.com/google/uuid`.
Activity: `Validate(Text) -> ValidationResult`.
Activity ID: `gooo://github.com/google/uuid/Validate`.
Result entity: `gooo://github.com/google/uuid/validation-result`.

| Required field | Type | Original Go observation |
| --- | --- | --- |
| `valid` | Boolean | `uuid.Validate(input) == nil` |
| `error` | Text | Exact `err.Error()`, or `""` for nil |
| `invalid_length` | Boolean | `uuid.IsInvalidLengthError(err)` |

This is an explicit error-interface-to-record adaptation. It preserves those
three observations and does not expose a Go error object, stack or generic
`errors.As` behavior. The field IDs are the result entity followed by `/valid`,
`/error` and `/invalid-length`, in that order.

The [importing inspector](../examples/uuid-format-inspector/README.md) calls this
port and appends the original `byte_length`. Its entry package is
`app/uuid-format-inspector`, activity `Inspect`, with ID
`gooo://go-ports/examples/uuid-format-inspector/inspect`. Its four fields are
`valid`, `error`, `invalid_length`, `byte_length`, in that order.

## Original formats and error order

- 36 bytes: `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`.
- 45 bytes: `urn:uuid:` followed by the 36-byte form. Prefix matching ignores case.
- 38 bytes: both braces around the 36-byte form.
- 32 bytes: hexadecimal without hyphens.

The original length dispatch takes place before format checks. A 38-byte string
with a wrong brace reports `invalid bracketed UUID format` before checking its
contents. A 45-byte string with a wrong prefix reports `invalid urn prefix: %q`
before body checks, quoting its original first nine bytes. Standard-form hyphens
are checked before hexadecimal pairs. Invalid lengths retain the decimal byte
length in `invalid UUID length: %d` and the invalid-length classification.
The port performs no trimming or normalization. Version and variant bits are
not restricted by the original `Validate`; all-zero and all-`f` forms can pass.

Integer-to-decimal text uses digit slices and concatenation inside Gooo. The
internal quoting implementation is the separate package
`go/github.com/google/uuid/internal/quote`, in
[quote.gooo](../ports/google/uuid/quote.gooo). It applies Go's quoted-string
escape rules to the first nine bytes inside the original full Text input.
A cut through a UTF-8 rune keeps each remaining byte as a lowercase `\x` escape.
For example, the first nine bytes of `abcdefgh가...` are quoted as
`"abcdefgh\xea"`. The broken prefix is never transmitted through another JSON
Text input, which would replace invalid UTF-8. This is an implementation detail
of the UUID port and is not counted as a separate public library function.

## Native Text boundary

The native input parser unmarshals a JSON string and checks the decoded string's
byte length against 1,024 before executing the body. It does not separately
reject invalid UTF-8 wire encodings: Go JSON decoding replaces invalid UTF-8 and
invalid UTF-16 surrogate spellings with U+FFFD. Comparisons for this port use
already-valid UTF-8 Go strings so original decoded inputs are preserved exactly.
Arbitrary invalid-byte Go strings, longer strings, UUID generation, `Parse`,
randomness, binary UUID values and full-module compatibility are outside this API.

## Source, checksums and licenses

The original module ZIP is 31,981 bytes with SHA256
`d0f02f377217f42702e259684e06441edbf5140dddcc34ba9bea56038b38a6ed`.
Its original module checksum is
`h1:NIvaJDMOsjHA8n1jAhLSgzrAzy1Hgr+hNrb57e+94F0=`.
All 31 upstream files, including `go.mod`, are preserved byte-for-byte under
[third_party/google-uuid](../third_party/google-uuid).
The [original file manifest](../third_party/google-uuid/provenance/original-module-files.json)
lists their individual sizes and SHA256 hashes. `uuid.go` retains Copyright 2018
Google Inc.; `util.go` retains Copyright 2016 Google Inc. The original BSD license
is preserved as [LICENSE](../third_party/google-uuid/LICENSE).

| Original file | SHA256 |
| --- | --- |
| `uuid.go` | `0edec8e34c6b6fe0db31b71a29069a09ed832e3fd04ee0175916b58f2b60e5c1` |
| `util.go` | `755f50afafb13dd89ae6177da7eb6dc31ae650ece2f9ba52821021e9f8de044e` |
| `LICENSE` | `0a8d61ed3cbfd5312326e8126c31ce9c627a283adc99131b56896d29ada04b2d` |

[source_digest.go](../third_party/google-uuid/source_digest.go) is an added,
clearly marked Gooo Go Ports verification helper. Its `//go:embed` values bind
those three original files into the compiled Go oracle. `PinnedSourceSHA256()`
returns exactly `uuid.go`, `util.go` and `LICENSE` digests for comparison with
independently pinned values. This helper does not implement or change UUID
validation. The original upstream `h1` covers the 31 original files; it does not
cover this added wrapper or provenance files, so this extended local tree is
not presented as the untouched checksum-addressed upstream module.

The quoting implementation lowers the complete pinned Go 1.27.2 `strconv.IsPrint`
tables into 741 Gooo intervals and retains the quoting source's Latin-1 rules.
Its unchanged source copies, hashes and original Go BSD license are under
[provenance/go-stdlib](../third_party/google-uuid/provenance/go-stdlib).
The source revision is `022c8636110ebac86a9b88724cda168ed713c21f`.

| Supporting Go source | SHA256 |
| --- | --- |
| `src/strconv/quote.go` | `6602960ec8777de9926c1d02fbf5cb0828d4aefa38083865f4af17b6892c09a2` |
| `src/strconv/isprint.go` | `e6f4b35fbf38f9c9c8b251e5d3cf94daa880789cb89825f11f9734ba4a42e9ff` |
| `LICENSE` | `911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad` |

Check deterministic helper generation without writing files:

```sh
python3 tools/gen-uuid-quote.py --check
```

The generator reads only byte-pinned local source copies, performs no download,
and changes no UUID behavior. Native comparisons use the original Go module as
an independent oracle; generated Gooo results and each receipt's own saved
replay must be checked separately. Function-level finite observations do not
establish every possible Text input, production safety or a performance advantage.
