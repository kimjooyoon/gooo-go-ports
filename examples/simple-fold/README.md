# Compare two Unicode simple-fold classes

This importing Gooo program compares two original code-point integers using the
ported `unicode.SimpleFold`. For example, `K`, `k` and the Kelvin sign `K` belong
to the same simple-fold class even though a single lowercase comparison does
not express the complete cycle. `SimpleFold` returns the next member, wrapping
to the first member; it does not return one canonical spelling.

The result contains `in_rune_domain`, `equivalent`, `left_next` and `right_next`.
The two next-member fields retain exact integers. The data is pinned to Go
1.27.2 / Unicode 17.0.0, and the compiler remains pinned by `tools/bootstrap.sh`.

| Left / right | Equivalent | Left next / right next |
| --- | --- | --- |
| `A` / `a` | true | `a` / `A` |
| `K` / `K` | true | `k` / `K` |
| `s` / `ſ` | true | `ſ` / `S` |
| `Σ` / `ς` | true | `ς` / `σ` |
| U+0398 / U+03F4 (four-member theta cycle) | true | U+03B8 / U+0398 |
| `İ` / `I` | false | `İ` / `i` |
| `ı` / `i` | false | `ı` / `I` |
| `ß` / `ẞ` | true | `ẞ` / `ß` |
| `ß` / `s` | false | `ẞ` / `ſ` |
| `é` / `e` | false | `É` / `E` |

This compares two integers. It does not process strings, perform full folding
such as `ß` to `ss`, select locale-specific Turkish rules, normalize text,
remove accents or check confusable characters. It is not identifier validation.

## Run and replay

From this repository's root, with Git, a POSIX shell and Go toolchain selection:

```sh
export GOTOOLCHAIN=go1.27.2
export GOWORK=off
./tools/bootstrap.sh
go run ./cmd/gen-simple-fold --check
mkdir -p out
./.gooo package execute --json --inputs examples/simple-fold/inputs.json \
  simple-fold.workspace.json > out/simple-fold.json
./.gooo package replay --json --receipt out/simple-fold.json \
  --inputs examples/simple-fold/inputs.json simple-fold.workspace.json \
  > out/simple-fold-replay.json
```

The 24 supplied input pairs include the examples above, Cherokee and Deseret,
negative values, surrogate values, int32 limits and integers wider than int32.
They contain no expected outputs; use the finite verifier for Go comparisons:

```sh
go run ./cmd/verify --out out/simple-fold-finite
```

The verifier output directory must be new. It checks original input bindings,
source and activity identities, independent Go outputs and saved replay.
The SimpleFold corpus includes every changing code point, every original
CaseRanges boundary, Latin-1, invalid Unicode and deterministic samples.
Consumer cases include every ordered pair in each three- and four-member cycle.
These are finite native observations, not exhaustive int32 equivalence.

## Domain and reuse

Go's `rune` type is signed int32, including negatives and values that are not
Unicode scalar values. Both inputs are checked before conversion. Invalid
Unicode values preserve Go's identity mapping: `Compare(-1, -1)` returns
`true, true, -1, -1`; a surrogate compared with itself also reports equivalent.
`in_rune_domain` therefore means the **int32 type domain**, not valid Unicode.

If either input is outside int32, the consumer returns `false, false` followed
by the two original integers unchanged. This is a separate rejection extension,
not a Go-reference comparison. The port itself returns an outside-int32 Integer
unchanged as its separate extension. No input is wrapped, clamped or narrowed.

To import the port in another workspace, copy `ports/unicode/space.gooo` and
`ports/unicode/simple_fold.gooo` together from the same fixed Git revision,
retaining their copyright headers and `LICENSE`, `NOTICE`,
`third_party/GO-LICENSE` and `upstream.json`. Register both sources:

```json
{"path":"go/unicode","sources":["ports/unicode/space.gooo","ports/unicode/simple_fold.gooo"]}
```

The generated file shares the Integer declaration in `space.gooo`. Import
`unicode "go/unicode"`, then call `unicode.SimpleFold(input)`. Imports resolve
workspace files; they do not download a dependency. The complete
[workspace](../../simple-fold.workspace.json) and [consumer](main.gooo) show the
four-field comparison result.

The pinned fold cycles have at most four members. The consumer checks the
original left value and three successors, with no loop or host-language folding
implementation. A separate Go-only traversal over 0..U+10FFFF checks that bound;
it does not count as exhaustive native Gooo execution.
