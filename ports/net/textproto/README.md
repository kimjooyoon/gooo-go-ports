# Bounded canonical MIME header keys

`go/net/textproto:CanonicalMIMEHeaderKey` adapts the returned Text content of Go
1.27.2 `net/textproto.CanonicalMIMEHeaderKey`, from pinned source
`022c8636110ebac86a9b88724cda168ed713c21f`.

The input is an explicitly present decoded valid UTF-8 Text of 0–1,024 bytes.
For a valid ASCII header token, an ASCII letter at the first byte or immediately
after a hyphen becomes uppercase; other ASCII letters become lowercase. For example,
`fOO-bAr` becomes `Foo-Bar`. Punctuation and digits other than a hyphen consume
the initial-uppercase position as the original Go function does.

Empty input and any input containing an invalid header-name byte return the
whole original Text unchanged. No partial prefix is converted. Thus `x bad`
stays `x bad`; the unchanged result does not establish a valid name. Unicode,
controls, spaces and NUL are supported diagnostic inputs within the Text bound,
and retain that identity behavior. Arbitrary-byte Go strings and longer strings
are outside this adaptation.

The [original eight-field composition](../../../examples/canonical-header-key/README.md)
keeps the input, canonical Text, change flag, validity, byte lengths and first
invalid byte witness. Its validity and witness come from the existing Gooo
Header Facts composition. All port and report calculations remain in Gooo.
There is no per-request human approval or external effect.

Missing or wrong-type input, malformed UTF-8, unpaired surrogate escapes and
longer decoded Text must be refused by the caller before the supported pure
call. The bundled CLI input documents are trusted examples; this addition does
not supply an external JSON preprocessor or establish generic CLI wire refusal.

[`canonical.gooo`](canonical.gooo) retains the original Go copyright notice.
[`provenance.json`](provenance.json) binds the exact
[original reader source](../../../third_party/go/net/textproto/reader.go.txt) and
[Go license](../../../third_party/GO-LICENSE). Returned Text content is the API
mapping; common-header interning, allocation and pointer identity are not exposed.
Finite references and saved replay do not establish all-string or whole-package
compatibility. This is not HTTP parsing, header repair, HTTP/2 validation,
authorization or network transmission.
