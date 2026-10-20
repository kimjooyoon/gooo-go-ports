# Inspect one Unicode rune's case mappings

This importing Gooo program returns the original integer and its simple
uppercase, lowercase and titlecase mappings. It calls the ported Go
`unicode.ToUpper`, `ToLower` and `ToTitle`, which share the ported `unicode.To`
implementation. `utf8.ValidRune` distinguishes a Unicode scalar from other
values in Go's signed int32 `rune` type.

For example, U+01C6 `ǆ` has three different results:

| Input | Upper | Lower | Title |
| --- | --- | --- | --- |
| `ǆ` U+01C6 | `Ǆ` U+01C4 | `ǆ` U+01C6 | `ǅ` U+01C5 |
| `a` U+0061 | `A` U+0041 | `a` U+0061 | `A` U+0041 |
| `İ` U+0130 | `İ` U+0130 | `i` U+0069 | `İ` U+0130 |
| `ı` U+0131 | `I` U+0049 | `ı` U+0131 | `I` U+0049 |
| `ß` U+00DF | `ß` U+00DF | `ß` U+00DF | `ß` U+00DF |
| combining U+0345 | `Ι` U+0399 | U+0345 | `Ι` U+0399 |

These are individual rune mappings from Go 1.27.2 / Unicode 17.0.0. The program
does not transform a whole string, apply Turkish locale rules, expand `ß` to
`SS`, title-case words, normalize text or decide which identifiers are safe.
It does not filter on `IsLetter`: the combining character U+0345 also changes
case. `SimpleFold` has a different purpose and may return a different next
member of a folding cycle.

## Execute and replay

Run from this repository's root with Git, a POSIX shell and Go toolchain
selection enabled:

```sh
export GOTOOLCHAIN=go1.27.3
export GOWORK=off
./tools/bootstrap.sh
go run ./cmd/gen-case-mapping --check
mkdir -p out
./.gooo package execute --json --inputs examples/case-inspector/inputs.json \
  case-inspector.workspace.json > out/case-inspector.json
./.gooo package replay --json --receipt out/case-inspector.json \
  --inputs examples/case-inspector/inputs.json case-inspector.workspace.json \
  > out/case-inspector-replay.json
```

The compiler is pinned by `upstream.json` and `tools/bootstrap.sh`. Read
`result.runtime.traces[].deliveries[].actual` in the execution receipt. The
first input, U+01C6 (454), returns:

```json
{
  "in_rune_domain": true,
  "valid_scalar": true,
  "original": 454,
  "upper": 452,
  "lower": 454,
  "title": 453
}
```

The 26 supplied inputs contain no expected outputs. Their receipt therefore
reports `OBSERVED`: it records execution, not an independent comparison. Saved
replay reproduces the recorded execution. To compare with the pinned Go
standard library and check receipt bindings, use a new output directory:

```sh
go run ./cmd/verify --out out/case-mapping-finite
```

The verifier retains the previous port corpus, adds case-mapping and importing
consumer cases, and replays every batch. It separately compares the unchanged
generated Go functions for each value in 0..U+10FFFF. That interval includes
surrogates; it does not cover all signed int32 values. Other int32 values,
invalid case kinds and int64 extensions are checked with finite samples and
reported separately.

## Exact domains and API differences

`in_rune_domain` means the input fits signed int32. It does not mean the input
is valid Unicode. For `-1`, a surrogate, or U+110000, it is true while
`valid_scalar` is false, and all three mappings preserve the original integer.
An input outside int32 returns both flags false and all integer fields
unchanged. This is an explicit extension of the Gooo `Integer` API; it is not
compared to a Go call after truncating the original value to `rune`.

The lower-level `unicode.To(kind, value)` port accepts Go's case constants as
integers: 0 = upper, 1 = lower, 2 = title. Its first input preserves the full
signed int64 `int` domain on the pinned 64-bit Go targets. An invalid kind
returns U+FFFD (65533), even for an invalid Unicode value. That check comes
first. For a valid kind, an outside-int32 value is returned unchanged as a
separate extension. `ToUpper`, `ToLower` and `ToTitle` each accept one original
integer and select their corresponding kind.

The three wrappers share one generated table implementation in Gooo. The
generator reads all pinned `CaseRanges` and their three deltas, checking the
original source hashes. Alternating upper/lower ranges use parity relative to
each range's first value, including ranges whose first value is odd.
Titlecase uses its own delta rather than an alias for uppercase.

## Reuse the native package

Copy `ports/unicode/space.gooo` and `ports/unicode/case_mapping.gooo` from the
same fixed repository revision. Keep their copyright headers, `LICENSE`,
`NOTICE`, `third_party/GO-LICENSE` and `upstream.json`. Register them together:

```json
{"path":"go/unicode","sources":["ports/unicode/space.gooo","ports/unicode/case_mapping.gooo"]}
```

The generated file shares the scalar declarations in `space.gooo`. Import
`unicode "go/unicode"` to call the functions; imports resolve local workspace
sources and do not download dependencies. The inspector also imports
`ports/unicode/utf8.gooo` for `ValidRune`. The complete
[workspace](../../case-inspector.workspace.json) and [consumer](main.gooo)
show these dependencies and the six-field record.

The case-mapping family is a native port. The existing browser function demo
has its own smaller function roster; this example does not add browser case
conversion.
