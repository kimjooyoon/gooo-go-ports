# Bounded URL component escaping

`go/net/url:PathEscape(Text)` and `go/net/url:QueryEscape(Text)` adapt the returned
string content of Go **1.27.2** `net/url.PathEscape` and `net/url.QueryEscape`.
Their implementation is in [`escape.gooo`](escape.gooo).

Each input must be decoded Unicode-scalar UTF-8 Text of **0..341 bytes**. The
largest encoded Text is **1,023 bytes**. This explicit port domain differs from
Go's arbitrary-byte, unbounded string input. Larger or malformed inputs must be
refused before binding; the pure Gooo activities do not validate external JSON.
Within the admitted domain there is no truncation, normalization or
decode-before-escape step. A direct call above 341 bytes violates the
precondition: the fixed loop can process a prefix and does not refuse it itself.
Use the strict adapter for external input.

Path escaping treats the input as one path segment; query escaping treats it as
one query value. A space becomes `%20` in a path segment and `+` in a query
value. A literal `+` remains `+` in a path segment and becomes `%2B` in a query
value. A literal `%` becomes `%25` in both contexts. The caller selects the
component context.

The [original URL Context Facts composition](../../../examples/url-context-facts/README.md)
returns both encodings together with unchanged input, byte lengths, change facts
and the first original percent-byte offset. That example includes a strict JSON
input adapter, explicit native execution and saved replay commands, and a finite
independent Go comparison. It is a sidecar to the existing native differential
catalog; another generation's receipts do not verify current source bytes.

The [source identity and modification notice](../../../examples/url-context-facts/NOTICE.md)
pins Go revision `022c8636110ebac86a9b88724cda168ed713c21f` and the complete
original source hashes. Adapted code retains the Go Authors' 2009 and 2025
copyright notices and the [Go BSD license](../../../third_party/GO-LICENSE).
This is component encoding, without URL parsing or unescaping, host/scheme
validation, authentication, authorization or network effects. Finite observations
do not establish full `net/url` compatibility or all-input correctness.
