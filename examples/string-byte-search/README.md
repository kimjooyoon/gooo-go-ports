# String byte search

Search for one byte in a small UTF-8 text and see its first byte offset.
This importing Gooo consumer calls `go/strings:IndexByte(Text, Integer)`;
the search loop and all result fields are computed in Gooo.

The source-owned loop has a literal bound of 1,024. It checks the text length
before each byte access, so empty text and an absent byte return −1. Native
workspace inputs must be valid UTF-8 Text of at most 1,024 bytes, including NUL.
Longer text fails closed before native execution. Go strings can contain arbitrary
bytes and have no such length bound; this port does not preserve that whole domain.

`Search(Text, Integer)` takes the text first and the original needle integer second.
The original Go `byte` domain is 0..255. An Integer outside that domain returns −1
as an explicit extension, without narrowing or wrapping to `byte`.

| Field | Meaning |
| --- | --- |
| `offset` | First matching UTF-8 byte position, or −1 for absence or an out-of-byte needle |
| `found` | Whether `offset` is nonnegative |
| `in_byte_domain` | Whether the original needle is between 0 and 255 |
| `byte_length` | UTF-8 byte length of the original text, also retained for an invalid needle |

Offsets count bytes. In `가x`, the byte sequence is `EA B0 80 78`:
needle `120` finds `x` at offset `3`, while needle `176` finds the second byte
of `가` at offset `1`. Searching for a continuation byte is permitted. The
consumer does not normalize text, count code points or graphemes, or provide a
whole-string Unicode validator.

With Go 1.27.2 on PATH and Gooo 0.6.32-dev from clean compiler source
`83928fa4538245abe33b08549da47f8cbdc50ed2`, run from the repository root:

```sh
export GOTOOLCHAIN=go1.27.2
export GOWORK=off
./tools/bootstrap.sh
mkdir -p out/string-byte-search
./.gooo package execute --json --inputs examples/string-byte-search/inputs.json \
  --go "$(command -v go)" string-byte-search.workspace.json \
  > out/string-byte-search/execution.json
./.gooo package replay --json --receipt out/string-byte-search/execution.json \
  --inputs examples/string-byte-search/inputs.json --go "$(command -v go)" \
  string-byte-search.workspace.json > out/string-byte-search/replay.json
```

The 17 [input-only examples](inputs.json) include empty and repeated text,
an absent byte, NUL, UTF-8 leading and continuation bytes, the exact 1,024-byte
boundary, and four outside-domain needle values. Multiple input keys are
`app/string-byte-search:Search.input0` and `.input1`. Receipts report `OBSERVED`
with `finite_total=0`, rather than embedding runtime expectations. Read actual
records at `result.runtime.traces[].deliveries[].actual`. Saved replay executes
the recorded program again; neither command requests a model.

For text `가x` and needle `120`:

```json
{"offset":3,"found":true,"in_byte_domain":true,"byte_length":4}
```

The original Go contract is [strings.IndexByte](https://pkg.go.dev/strings#IndexByte),
from [src/strings/strings.go, line 102](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/strings/strings.go#L102)
at fixed Go revision `022c8636110ebac86a9b88724cda168ed713c21f`.
The source file SHA256 is
`a81bdf73a308277c322e35d0c602e3813f57b6f769c091f8f8962c8884a45ea3`.
Go's copyright and BSD license remain in [search.gooo](../../ports/strings/search.gooo)
and the existing [GO-LICENSE](../../third_party/GO-LICENSE).

The bounded-loop form derives from the MIT-licensed
[Gooo bounded-text-search example](https://github.com/kimjooyoon/meta-ontology-go/blob/c56007f5881296cb3181ff056c50a4ae917a9850/examples/bounded-text-search/search.gooo.fixture).
Its fixed source revision is `c56007f5881296cb3181ff056c50a4ae917a9850`;
Copyright (c) 2026 Kim Joo-yoon. The complete MIT notice is retained in
[search.gooo](../../ports/strings/search.gooo). The example source and the compiler
used for this consumer have distinct recorded identities.

This is an experimental function-level port over the stated Text and byte domain.
Finite execution and replay observations do not establish whole-package
compatibility, arbitrary-byte Go string support, production safety or performance.
