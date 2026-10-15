# Signed decimal int64 parsing in Gooo

`go/strconv:ParseIntDecimal64` adapts Go 1.27.2
`strconv.ParseInt(original, 10, 64)` to a pure Gooo activity. Its compatible domain
is decoded, valid UTF-8 Text of at most 1,024 bytes. The Gooo body owns parsing;
Go supplies independent reference values during verification.

The required `DecimalInt64Result` record has four fields:

| Field | Meaning |
| --- | --- |
| `input` | Original decoded Text, unchanged |
| `value` | Exact signed int64 returned by the specialized Go operation |
| `parsed` | True exactly when `error_kind` is zero |
| `error_kind` | `0` success, `1` syntax, `2` range |

An optional leading ASCII `+` or `-` and leading zeros follow Go's base-10
syntax. Empty/sign-only input, whitespace, underscores, radix prefixes, NUL and
Unicode digit/sign lookalikes do not become decimal digits. The result preserves
Go's signed range saturation and scan precedence: a signed overflow followed by
an invalid suffix can be a syntax error, while an earlier uint64 overflow can
already have established a range error. Prevalidating the entire string or
stopping at the first signed overflow would change that behavior.

This port does not expose configurable bases or bit sizes, `ParseUint`, or Go's
`NumError` object. Its explicit body guard returns separate kind `3` for Text
longer than 1,024 bytes, before the bounded scan. That is outside the compatible
domain; native Text validation may refuse such input before the body runs.
Native observation of kind `3` is not claimed. Go JSON decoding may replace
malformed raw UTF-8 or escaped surrogate sequences; the contract concerns decoded
valid Text and does not promise strict raw-wire preservation or rejection.

After the [repository setup](../../README.md), run from the repository root:

```sh
mkdir -p out/decimal-int64-example
./.gooo package execute --json --go "$(go env GOROOT)/bin/go" \
  --inputs ports/strconv/inputs.json decimal-int64.workspace.json \
  > out/decimal-int64-example/execution.json
./.gooo package replay --json --go "$(go env GOROOT)/bin/go" \
  --receipt out/decimal-int64-example/execution.json \
  --inputs ports/strconv/inputs.json decimal-int64.workspace.json \
  > out/decimal-int64-example/replay.json
```

`corpus-256.json` contains input-only finite cases, including exact int64
boundaries, signed-versus-unsigned overflow with invalid suffixes, signs,
whitespace, Unicode, NUL, escaping and 1,024-byte strings. No expected records
are stored in it. Fresh native verification compares the Gooo output with actual
pinned `strconv.ParseInt` values and checks saved replay. This is not exhaustive
Text or whole-package `strconv` compatibility. The importing
[payload example](../../examples/payload-admission/README.md) adds stricter
unsigned spelling and caller-supplied policy facts; parse success alone is not
payload eligibility.

The original Go copyright headers are preserved in `decimal.gooo`.
[Provenance](provenance.json) pins both upstream source files and their hashes;
the Go BSD license is retained in `third_party/GO-LICENSE` at the repository root.
