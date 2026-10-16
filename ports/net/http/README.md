# Bounded ETag helpers

`go/net/http/etag` is a Gooo adaptation of the **private** `scanETag` and
`etagStrongMatch` helpers in pinned Go `net/http`. It is not an official Go
subpackage. Implementations are in [etag.gooo](etag.gooo); Go is used only by
the independent verification oracle.

| Gooo activity | Original behavior | API mapping |
| --- | --- | --- |
| `ScanETag(Text)` | Trim ASCII SP/HT/LF/CR at both edges, then scan one tag | Go's `(etag, remain)` returns become a required two-field record |
| `ETagStrongMatch(Text, Text)` | Raw equality, nonempty, first byte is a double quote | Boolean; does not scan or validate either operand |

Each argument must be decoded Unicode-scalar, valid UTF-8 Text of at most 1,024
bytes. Empty strings, NUL, controls and grammar-invalid tags are included.
Arbitrary-byte Go strings and longer inputs are outside this port's domain.
No trimming beyond the scanner's four ASCII edge bytes, normalization,
unescaping or case folding is added. A backslash never escapes a quote.
A quoted empty tag is valid and differs from the empty failure sentinel.
The scanner preserves the exact suffix after the first complete tag.

`ETagStrongMatch` deliberately preserves Go's permissive private-helper
behavior: equal strings starting with a quote can match even when they are not
valid complete ETags. Use the [original facts composition](../../../examples/strong-validator-facts/README.md)
when a complete parsed tag and raw header validity are needed.

These pure activities do not enforce transport JSON validity or the byte bound.
Callers must preserve the stated decoded input domain. Do not assume the
compiler's JSON decoder rejects every duplicate key, malformed UTF-8 sequence
or unpaired surrogate. The verifier validates its fixed inputs before binding.

The finite corpus retains 192 scanner occurrences and 31 raw comparison pairs,
including the eight original Go scanner tests, all 128 ASCII positions, literal
backslashes, weak tags, invalid grammar, Unicode and 1,024-byte boundaries.
It is not exhaustive equivalence over all strings or a full `If-Match`, cache,
HTTP server, resource authentication or authorization implementation.

See [provenance.json](provenance.json) for immutable source hashes and the
[Go license](../../../third_party/GO-LICENSE).
