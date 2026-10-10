# Inspect pasted Unicode text

Open the [Unicode text inspector](https://kimjooyoon.github.io/gooo-go-ports/text-inspector.html)
to examine copied text without installing a compiler. For example, the built-in
mixed-text input contains `A`, a non-breaking space, `B`, CR, LF, a zero-width
space, and an emoji. Its seven rows show exactly where each item occurs.

The application calls four existing Gooo ports for every decoded item:
`utf8.ValidRune`, `unicode.IsSpace`, `utf8.RuneLen`, and `utf16.RuneLen`.
Their calculations run in the same precompiled WebAssembly program as the
single-function demo. A separate browser Worker keeps this work off the page's
main thread. The function count remains 19; this browser workflow does not add
a native Gooo consumer or implement a whole-string Go API.

## Supply text and inspect the captured input

Use text mode to type or paste. Pasting or dropping text replaces the complete
captured input. The application reads the clipboard or drop payload before the
browser's textarea can normalize CR and CRLF. It then clears the capture box
and shows an escaped, read-only preview of the exact received string. Typing
after a capture starts a new input. Typing in a normal browser textarea uses
its normal line-feed handling.

JSON string mode provides an explicit way to enter control code units and lone
surrogates, for example `"\r\n\uD800"`. Supply one complete JSON string, including
the surrounding quotes. The escaped preview is the authoritative captured input
in both modes. Neither mode trims text or normalizes Unicode. JSON source is
limited to 4,096 UTF-16 code units before parsing; the decoded text must also
fit the limits below.

The browser application accepts at most 256 decoded items and 512 original
UTF-16 code units. An input above either limit is rejected as a whole. These are
page processing limits, separate from the original integer domains of the Go
ports. Replacing or clearing an input removes prior results; a failed or
interrupted run does not leave partial results presented as a completed check.

## Read the results

- Positions are zero-based UTF-16 offsets in the string received by the browser.
  A valid surrogate pair is one decoded code point occupying two original units.
  A lone surrogate is preserved as one item occupying one original unit.
- CR and LF are separate items. Combining marks, regional indicators and ZWJ
  sequences also appear separately; the row count is not a count of visible
  letters or grapheme clusters.
- Go whitespace uses Unicode 17.0.0 White_Space: NEL and NBSP are included,
  while BOM and U+200B are excluded. This is not a visibility or safety verdict.
- Original UTF-16 units describe the input transport. Gooo's UTF-16 encoding
  length describes whether the item can encode a valid scalar. For a lone
  surrogate, the former is 1 and the latter is -1. UTF-8 length is also -1.
  Encoding totals are unavailable when any item is an invalid scalar; replacement
  characters are not silently substituted to produce a total.

Text is processed locally in this tab. The application loads its static program
and metadata, and does not send captured text to a server or put it in a shared
URL. It examines the string supplied by the browser; it cannot reconstruct
changes an originating app or clipboard already made, or diagnose the original
file's invalid UTF-8 bytes.

## How the application is checked

The Worker checks the program bytes against the bundled Wasm hash and checks
each runtime response's operation, original input and output shape before using
it. JavaScript decodes the input and displays or totals the returned values;
the four per-item judgments come from compiled Gooo.

The existing native execution/replay evidence and browser reference corpus still
cover the same ports. Additional application fixtures use numeric UTF-16 units
and explicitly expected decoded positions, with the four reference outputs
computed independently by the pinned Go toolchain. Numeric units preserve lone
surrogates that decoding a JSON string into a Go string would replace.
The application checks are reported separately from the port corpus.

From the repository root, after bootstrap:

```sh
node --test tools/*.test.cjs
go run ./cmd/build-demo --out out/browser-text
node tools/browser-smoke.cjs out/browser-text/site
node tools/text-inspector-smoke.cjs out/browser-text/site
```

The output directory must be new. Use the repository's pinned Go 1.27.2.
Browser tests also cover raw paste handling, rejected oversized inputs, malformed
runtime responses, input replacement during execution, and failed runs clearing
old results. These are finite observations, not whole-library compatibility.
