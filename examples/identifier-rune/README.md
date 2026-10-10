# Classify an identifier character

This importing Gooo example uses `unicode.IsLetter` and `unicode.IsDigit` to
classify one signed int32 rune for Go's lexical identifier character rules.
It returns `in_rune_domain`, `can_start`, `can_continue`, and `is_decimal_digit`.
An underscore or Unicode letter may start; a decimal digit may also continue.
The [Go specification](https://go.dev/ref/spec#Identifiers) defines these rules.

The Unicode categories are pinned to Go 1.27.2 / Unicode 17.0.0. `IsDigit` means
category **Nd**, not every numeric character. For example:

| Input | Can start | Can continue | Decimal digit |
| --- | --- | --- | --- |
| `A`, `é`, `가`, `𐐀` | true | true | false |
| `_` | true | true | false |
| `1`, `١`, `𝟎` | false | true | true |
| `²`, `¼`, `Ⅷ`, U+0300 combining grave accent | false | false | false |

This classifies one original integer, not a string. It does not reject reserved
words, perform normalization, implement Unicode XID rules, or assess confusables
and security. In particular, `é` and a sequence of `e` plus a combining accent
are different inputs. Go's blank identifier has additional context-specific rules.

## Try the classification in a browser

The [text inspector](https://kimjooyoon.github.io/gooo-go-ports/text-inspector.html)
shows the same `IsLetter` and `IsDigit` ports for each decoded code point. Choose
**Letters + decimal digits** to compare letters, Nd digits and other numbers.
This browser view does not call this native `Classify` consumer or determine
whether a complete string is a valid identifier. The
[function demo](https://kimjooyoon.github.io/gooo-go-ports/) also accepts original
signed int32 inputs, including negative and non-scalar values.

## Run and replay

From the repository root, with Git, a POSIX shell, and Go toolchain selection:

```sh
export GOTOOLCHAIN=go1.27.2
export GOWORK=off
./tools/bootstrap.sh
go run ./cmd/gen-unicode --check
mkdir -p out
./.gooo package execute --json --inputs examples/identifier-rune/inputs.json \
  identifier-rune.workspace.json > out/identifier-rune.json
./.gooo package replay --json --receipt out/identifier-rune.json \
  --inputs examples/identifier-rune/inputs.json identifier-rune.workspace.json \
  > out/identifier-rune-replay.json
```

The supplied inputs include Latin-1, supplementary-plane letters and digits,
combining marks, numeric non-digits, invalid Unicode, int32 limits, and wider
integers that would alias a letter if incorrectly narrowed. Input-only runs do
not prove equivalence; use the verification commands below for Go comparisons.

All original signed int32 inputs are preserved. A negative rune, surrogate or
value above U+10FFFF is in the rune type's domain but cannot start or continue.
An Integer outside int32 returns all four fields as false, including
`in_rune_domain`; this is a separate extension, not a Go-reference observation.

## Import in another workspace

Copy `ports/unicode/space.gooo` and `ports/unicode/classification.gooo` together,
with their copyright headers, `LICENSE`, `NOTICE`, `third_party/GO-LICENSE` and
`upstream.json`, from the same fixed Git revision. The generated file shares the
Integer and Boolean entity declarations in `space.gooo`. Register both sources:

```json
{"path":"go/unicode","sources":["ports/unicode/space.gooo","ports/unicode/classification.gooo"]}
```

Then `import unicode "go/unicode"` and call `unicode.IsLetter(input)` or
`unicode.IsDigit(input)` in a Gooo activity. Imports resolve workspace files;
they do not download a dependency. See the complete
[workspace](../../identifier-rune.workspace.json) and [consumer](main.gooo).

## Regenerate and verify

```sh
go run ./cmd/gen-unicode
go run ./cmd/gen-unicode --check
go run ./cmd/verify --out out/classification-finite
go run ./cmd/check-unicode --out out/classification-exhaustive
```

Each output directory must be new. The generator checks the pinned toolchain,
Unicode version, source-file hashes and table shape, then turns every Letter
and Digit range into balanced Gooo branches. Range strides are retained;
no runtime Go classification wrapper performs the port's calculation.

The finite verifier checks range boundaries, stride holes, Latin-1, named
distinctions, integer limits and fixed samples through native execution and
saved replay. The separate exhaustive check builds the unmodified generated Go
from the importing program and compares both ports over **0..U+10FFFF**:
1,114,112 inputs and 2,228,224 function comparisons. It also checks the consumer.
The rest of signed int32 and the outside-int32 extension are tested with finite
boundaries and samples; they are not described as exhaustively tested. These
counts remain separate from receipt-backed finite observations.

The checked-in classification source is 76,653 bytes. With its shared entity
source and this consumer, the dedicated workspace sources total 79,206 bytes;
the generated Go composition is 79,683 bytes. They fit the pinned compiler's
128 KiB Gooo-source and 256 KiB composition bounds. A local 21-case execution
and replay each produced about 546 KiB of JSON evidence. These are size
observations for this pinned program, not performance guarantees.
