# Whitespace inspector

Inspect one integer code point when a separator, pasted character, or encoding
boundary is difficult to see. This Gooo consumer imports `unicode.IsSpace`,
`utf8.ValidRune`, `utf8.RuneLen`, and `utf16.RuneLen`; all calculations stay in
Gooo. It accepts code points rather than strings and does not trim or scan text.

From the repository root with Go 1.27.2, Git and a POSIX shell:

```sh
export GOTOOLCHAIN=go1.27.2 GOWORK=off
./tools/bootstrap.sh
mkdir -p out/whitespace-demo
./.gooo package execute --json --inputs examples/whitespace-inspector/inputs.json \
  whitespace-inspector.workspace.json > out/whitespace-demo/execution.json
./.gooo package replay --json --receipt out/whitespace-demo/execution.json \
  --inputs examples/whitespace-inspector/inputs.json \
  whitespace-inspector.workspace.json > out/whitespace-demo/replay.json
```

The 21 [input-only examples](inputs.json) use `app/whitespace-inspector:Inspect` as
the single-input key. Their receipts report `OBSERVED`, without embedded expected
outputs. Read results at `result.runtime.traces[].deliveries[].actual`.

| Field | Meaning |
| --- | --- |
| `in_rune_domain` | Original input fits Go's signed int32 `rune` |
| `valid_scalar` | `utf8.ValidRune` for the original rune |
| `is_go_space` | `unicode.IsSpace` under the pinned Unicode 17.0.0 table |
| `is_ascii_space` | Go whitespace whose code point is between 0 and 127 |
| `utf8_bytes` | UTF-8 byte length, or −1 for an invalid scalar |
| `utf16_units` | UTF-16 code-unit length, or −1 for an invalid scalar |

For non-breaking space, input `160` (U+00A0):

```json
{"in_rune_domain":true,"valid_scalar":true,"is_go_space":true,"is_ascii_space":false,"utf8_bytes":2,"utf16_units":1}
```

Go's `IsSpace` includes U+0085 (NEL), U+00A0 (NBSP), U+2007 (figure space) and
U+202F (narrow no-break space). U+180E, U+200B (zero-width space), U+200E/U+200F,
U+2060 and U+FEFF (BOM) are not Go whitespace. Neither a character's name nor a
JavaScript regular expression or `trim()` determines this result. ASCII whitespace
here is tab through carriage return and ordinary space; U+001C..U+001F are false.

The port `go/unicode:IsSpace(Integer) -> Boolean` preserves the complete original
signed int32 domain, −2147483648 through 2147483647. Negative values, surrogates,
and values above U+10FFFF are accepted rune inputs and produce false; the consumer
still reports `in_rune_domain: true` for them. Integers outside int32 are an explicit
extension: the port returns false, and the consumer returns all Boolean fields
false with both lengths −1. Original inputs are never narrowed or wrapped.

Source and supporting tables are pinned in [upstream.json](../../upstream.json):
[graphic.go](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/unicode/graphic.go#L120),
[tables.go](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/unicode/tables.go#L9029),
and [letter.go](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/unicode/letter.go).
The complete 25-member White_Space property is expanded into comparisons in
[space.gooo](../../ports/unicode/space.gooo). Copyright and BSD license notices
remain in the source, [NOTICE](../../NOTICE), and [GO-LICENSE](../../third_party/GO-LICENSE).

The verifier adds 506 Go-reference cases and eight outside-int32 extension cases
for each of the port and this consumer. It includes every Latin-1 value, every
true property member and neighboring boundaries, modern nonspaces, invalid Unicode
and signed-domain boundaries, and deterministic samples. All 14,093 existing
cases are preserved. The 21 input-only examples are separate from these comparison
counts. Run `go run ./cmd/verify --out out/whitespace-check` using a new output
directory to compare with Go and replay saved programs.

These are finite observations over a function's original domain, not a claim of
whole-package compatibility or exhaustive execution of all int32 values. This
example does not normalize text, classify line breaking, or validate UTF-8 strings.
