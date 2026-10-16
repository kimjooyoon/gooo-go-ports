# Bounded HTTP header predicates

These two Gooo activities adapt `golang.org/x/net/http/httpguts` from x/net
v0.59.0, revision `540d04cfe5028e2655754591a4d3e08c586809f2`:

| Gooo entry | Result |
| --- | --- |
| `go/golang.org/x/net/http/httpguts:ValidHeaderFieldName` | Whether the nonempty Text contains only HTTP token bytes |
| `go/golang.org/x/net/http/httpguts:ValidHeaderFieldValue` | Whether Text contains no forbidden C0 control bytes or DEL; HTAB is allowed |

Each input must be decoded valid UTF-8 Text of at most 1,024 bytes. The fixed
Gooo scans cover that whole domain. Go accepts arbitrary byte strings and has no
1,024-byte bound; these activities do not expose that larger domain. Refuse
missing values, wrong types, malformed UTF-8 and longer inputs before invoking
Gooo. Unsupported input is not a supported `false` result.

An empty name returns `false`; an empty value returns `true`. Names are ASCII
tokens and preserve uppercase letters. Values preserve leading/trailing SP and
HTAB, and permit bytes at least 0x80, including UTF-8 text. No input is trimmed,
normalized, lowercased or sent. These checks do not implement HTTP/2 field-name
rules, message framing, duplicate-field policy, authentication or authorization.

The original [Header Facts composition](../../../examples/header-field-facts/README.md)
imports both activities and adds original input echoes, first-invalid byte
offsets/octet values and three descriptive flags. It documents strict external
JSON preprocessing before the native CLI and an automatic decision without
per-input human approval.

[`headers.gooo`](headers.gooo) retains the complete upstream BSD 3-Clause notice
and the MIT notice for adapted Gooo bounded-loop syntax. Immutable module/source
pins and adaptation scope are recorded in [`provenance.json`](provenance.json).
The finite public verifier must compare the current source with the independent
Go functions and check saved replay; source preparation is not completed native
validation or full-package compatibility.
