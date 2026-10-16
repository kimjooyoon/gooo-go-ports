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


## Selected If-Match compatibility core

[`go/net/http/precondition`](precondition.gooo) exposes `IfMatchCondition(Text,
Text) -> IfMatchResult`. The first Text is one **already selected** If-Match
value; the second is the current ETag supplied by the caller. Its one required
integer field `condition` retains Go's private enum: **0 = no condition**,
**1 = true**, **2 = false**. This dedicated package is a Gooo API, rather than
an official Go subpackage.

The bounded state loop preserves the original Go behavior. Initially empty
If-Match yields0; whitespace/comma-only yields2. `*suffix` yields1 immediately,
even with empty current ETag. Adjacent quoted tags need no comma, and a strong
match can return before an invalid trailing token. Weak tags do not strongly
match. Neither input is normalized or repaired; current ETag is not parsed or
trimmed. Treat raw header validity as a separate fact.

Both Texts use the same unchanged Unicode-scalar UTF-8, 0–1,024-byte domain as
the ETag port, including empty, NUL, CR/LF and invalid grammar. The pure Gooo
body cannot enforce external JSON decoding. Use the strict before-binding
adapter for external requests. From the repository root, after the pinned Go
and compiler are ready:

```sh
set -eu
mkdir -p out
go build -mod=readonly -o out/if-match-input ./cmd/if-match-input
out/if-match-input < ports/net/http/if-match-request.json > out/if-match.inputs.json
./.gooo package execute --json --go "$(go env GOROOT)/bin/go" \
  --inputs "$PWD/out/if-match.inputs.json" "$PWD/if-match.workspace.json" \
  > out/if-match.execute.json
./.gooo package replay --json --go "$(go env GOROOT)/bin/go" \
  --inputs "$PWD/out/if-match.inputs.json" --receipt "$PWD/out/if-match.execute.json" \
  "$PWD/if-match.workspace.json" > out/if-match.replay.json
```

The request accepts exactly `if_match` and `current_etag` string keys. It refuses
wrong/missing/duplicate keys, invalid UTF-8, unpaired Unicode surrogates and
values over1,024 UTF-8 bytes before the caller binds inputs. It deliberately
accepts header-invalid controls and grammar-invalid Text without alteration.
The adapter neither runs Gooo nor supplies an expected result.

The [input-only corpus](if-match-corpus.json) retains284 occurrences, including
four original Go tests. Fresh Go expectations are compared with Gooo execution
and official saved replay in five bounded batches. The strict route's five
input-only anchors and35 prelaunch refusals are separate transport checks;
the direct-CLI malformed comparison adds28 shape/domain controls to the
existing196, without claiming wire-invalid UTF-8/surrogate refusal by the
compiler. Input-only receipts are `OBSERVED`, with finite counts0; they are not
additional differential cases.

This core does not select repeated HTTP fields, observe/authenticate the current
resource, enforce authorization or execute a conditional/CAS write. See the
[exact pins, enum mapping and scope](if-match-provenance.json).
