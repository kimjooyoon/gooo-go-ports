# Saved literal-text report checker

This command compares the supplied input-only native execution and saved replay
against the pinned Go `strings.Index` and `strings.Count` oracle. It validates
the input/source/compiler identities, native process observations, generation
report provenance, typed-plan binding and both replay identity hashes before
exporting the checked input/results JSON. The workflow instructions and domain
are in `examples/literal-text-report/README.md`.

The replay's `prior_result_sha256` is the hash of the original **decoded typed
Result**, marshaled by the pinned compiler's Go `encoding/json`. It is not the
execution file hash or a compacted/map-marshaled replacement. At compiler
`83928fa4538245abe33b08549da47f8cbdc50ed2`, this checker reproduces that encoding
for the canonical native CLI profile of this input-only workflow:

- Typed objects retain declared field order; supported `omitempty` fields may
  be absent. Normal process wait limits/deadlines support zero and nonzero
  observations, and nullable resource measurements retain null/zero values.
- Typed integers use their canonical signed 64-bit spelling. The three declared
  float64 metadata paths use Go's float64 decoding and canonical JSON encoding,
  including fractional route latency and completeness scores.
- `completeness_receipt.scope` recursively sorts map keys with `UseNumber`,
  preserving exact generic number tokens and array order. `status_counts` is
  decoded as integer counts and sorted as a map.
- Runtime `RawMessage` values preserve object order, number tokens, array order
  and existing string escapes. Go's default HTML escaping applies to the final
  typed serialization, including `<`, `>`, `&`, U+2028 and U+2029.

The embedded profile binds supported paths to pinned struct types/tags, rather
than the particular zero/nonzero measurements in the original pilot. Its
golden test is the immutable original 24-input execution: 95,075 typed-result
bytes, digest
`sha256:9283884c6026ae620152b2ac6353162acd5c6491e12737855ea6e1859bfa1ab1`.

The original-result serializer deliberately has a limited format. It rejects unknown/case-aliased typed
fields, reordered typed objects, noncanonical typed scalar spellings, new
optional provider/routing/weights/body-fill/policy layouts, duplicates, invalid
UTF-8, trailing documents, receipts over 32 MiB and nesting beyond 64 levels.
These format failures do not assert native compiler incompatibility. New
receipt profiles need source-derived serializer support and independent
validation. Finite `--cases` receipts are outside this input-only profile.

Synthetic rejection and serializer tests exercise the checker, not additional
native runs. Receipt metadata is unsigned; consistency checking is not a
cryptographic attestation against replacement of the whole evidence bundle.
