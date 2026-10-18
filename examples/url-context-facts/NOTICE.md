# URL component adaptation and original composition

The adapted URL escaping behavior retains:

Copyright 2009 The Go Authors. All rights reserved.

Copyright 2025 The Go Authors. All rights reserved.

The Go sources use the BSD-style terms reproduced in
[`third_party/GO-LICENSE`](../../third_party/GO-LICENSE). The original Gooo
`Report` composition uses the [repository's BSD license](../../LICENSE).
Neither license grants an endorsement by Go, Google, or their contributors.

## Immutable upstream identity

Go **1.27.2**, revision `022c8636110ebac86a9b88724cda168ed713c21f`:

| Original file | SHA-256 |
| --- | --- |
| [`src/net/url/url.go`](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/net/url/url.go) | `44c33f88526abf3b5955d6c97297be5c9d228e741b33e52c940dcac5a496e091` |
| [`src/net/url/gen_encoding_table.go`](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/net/url/gen_encoding_table.go) | `cd0f5020e4a5aa9741411afb4ab1bf73b9e9f40cd950e2bb9b369a773fdd9033` |
| [`src/net/url/encoding_table.go`](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/net/url/encoding_table.go) | `274e5ad203d6c38c41fe435eb9e5fc65ae1a38e98b44daa9b6355ba0d3fdac09` |
| [`LICENSE`](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/LICENSE) | `911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad` |

## Changes and boundary

[`ports/net/url/escape.gooo`](../../ports/net/url/escape.gooo) adapts the returned
string content of `PathEscape` and `QueryEscape` to bounded Gooo Text activities.
It expresses byte escaping as a bounded loop instead of Go's encoding table and
allocation implementation. It admits only Unicode-scalar UTF-8 Text of
0..341 bytes, with a separate strict external-input adapter. It does not provide
Go's arbitrary-byte string domain, parsing, unescaping, URL values, host handling,
or allocation identity.

[`main.gooo`](main.gooo) is an original composition: it retains the original
input, both encodings, byte lengths, change facts, context difference and the
first original percent-byte offset. The Go reference is independently written
verification code, without serving as a production fallback. The adapter checks
the external input domain and emits bindings; the Gooo source computes results.
