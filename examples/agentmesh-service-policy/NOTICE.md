The bounded matcher is adapted from Google Agent Mesh at immutable revision
`9fd1a41a15174df0aaef601c942410c2116bff91`.

- `api/tar.go`: `MatchServicePattern`
- `api/network.go`: `ParseServiceTarget` and `rfc3986URIRegex`
- `api/tar_test.go`: the twelve `TestMatchServicePattern` input/expected tuples

Original files and `Copyright 2026 Google LLC` notices are preserved byte-for-byte
as `.go.txt` source archives. The original Apache-2.0 license is preserved in
`upstream/LICENSE` and `LICENSE-APACHE-2.0`. The standard-library oracle copies
the exact original function bytes; its wrapper and the original Gooo request-level
impact composition are distinct from the upstream matcher.

This example is independently developed. It does not imply Google endorsement
or complete Agent Mesh compatibility.
