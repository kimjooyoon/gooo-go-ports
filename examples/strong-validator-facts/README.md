# Strong validator facts

This original Gooo composition combines the ETag scanner, its raw strong-match
helper and the existing `x/net/httpguts` header-value port. It returns the
unchanged input, parsed value and reasons that another program can inspect.
No model call or per-call human approval is needed to compute these local facts.

```sh
./.gooo package execute --json --go "$(go env GOROOT)/bin/go" \
  --inputs examples/strong-validator-facts/inputs.json \
  strong-validator-facts.workspace.json
```

The manifest selects exactly the ETag package, header-value package and this
example. The default workspace is not required. The four example inputs show a
strong tag, a weak tag, trailing list content and an invalid raw header value.

| Field | Meaning |
| --- | --- |
| `raw` | Unchanged decoded input |
| `tag`, `remainder` | Exact Go scanner outputs after ASCII edge trimming |
| `header_valid` | Header-value validity of **raw**, before scanner trimming |
| `present` | Scanner returned a nonempty tag, including a quoted empty tag |
| `weak` | Parsed tag starts with exact uppercase `W/` |
| `complete` | A tag is present and no trimmed remainder exists |
| `strong` | Original strong-match helper applied to the parsed tag twice |
| `single_strong` | Valid raw header, present complete parsed tag and strong classification |
| `input_bytes`, `tag_bytes`, `remainder_bytes` | UTF-8 byte lengths, not rune counts |
| `reason` | First applicable reason from the table below |

Reason precedence is `HEADER_VALUE_INVALID`, `NO_VALID_TAG`, `TRAILING_CONTENT`,
`WEAK_TAG`, then `SINGLE_STRONG_TAG`. For example, CRLF around a tag can be trimmed
by the scanner while still making the original raw header invalid. The result
keeps those two facts separate.

Inputs are unchanged decoded Unicode-scalar, valid UTF-8 Text from zero through
1,024 bytes, including controls and grammar-invalid tags. Transport JSON validity
and this byte bound are caller preconditions; the pure body does not enforce
them. See the [port domain and API differences](../../ports/net/http/README.md).

`single_strong` is a local classification. It does not bind the tag to a current
resource, compare an external validator, authenticate a caller, authorize a write
or make a conditional update atomic. This example performs no HTTP or other
external effect. It is not complete `If-Match` list/wildcard handling.

Verification retains all 192 scanner, 31 comparison and 24 composition inputs.
Fresh independent Go results are checked against separately retained literal
anchors before native Gooo execution and official saved replay. Duplicate input
occurrences are retained. These are finite observations, not a production-safety
or whole-library compatibility claim.

To run the optional focused native check on **all** 247 inputs, supply a pinned
compiler and a new evidence directory:

```sh
GOOO_ETAG_TEST_COMPILER="$PWD/.gooo" GOOO_ETAG_TEST_OUT="$PWD/out/etag-native" \
  go test ./cmd/verify -run '^TestETagNativeFullCorpus$' -count=1 -v
```

The normal full verifier and repository CI remain required for changes. The
optional check does not replace them. Source licenses, Go revisions and type
mappings are in [the port provenance](../../ports/net/http/provenance.json) and
[contract.json](contract.json).
