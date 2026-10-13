The bounded matcher is adapted from Google Agent Mesh at immutable revision
`9fd1a41a15174df0aaef601c942410c2116bff91`.

- `api/tar.go`: `MatchServicePattern`
- `api/network.go`: `ParseServiceTarget` and `rfc3986URIRegex`
- `api/tar_test.go`: the twelve `TestMatchServicePattern` input/expected tuples

Original files and `Copyright 2026 Google LLC` notices are preserved byte-for-byte
as `.go.txt` source archives. The original Apache-2.0 license is preserved in
`upstream/LICENSE` and `LICENSE-APACHE-2.0`. The standard-library oracle copies
the exact original function bytes; its wrapper and the original Gooo request-level
impact and single-pattern refinement compositions are distinct from the upstream
matcher. The refinement rules and candidate-witness construction are original
Gooo code; they are not a Google Agent Mesh API or policy applier. The canonical
domain predicates and checked input/result wrappers are also original Gooo code.

This example is independently developed. It does not imply Google endorsement
or complete Agent Mesh compatibility.
