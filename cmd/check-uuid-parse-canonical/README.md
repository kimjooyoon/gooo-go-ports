# Saved UUID canonical parsing checker

Run from the repository root with the pinned Go 1.27.3 toolchain:

```sh
go run ./cmd/check-uuid-parse-canonical --snapshot . \
  --inputs examples/uuid-parse-canonical/inputs.json \
  --execution execution.json --replay replay.json --compiler-build compiler.json
```

The manifest is fixed to `uuid-parse-canonical.workspace.json` and its exact
three UUID/quote/Text sources. The command reads evidence and compares its
four returned fields with the byte-bound original Google UUID v1.6.0 `Parse`,
returned `UUID.String`, error text and `IsInvalidLengthError` host oracle.
It preserves partial returned UUID values on errors and accepts Parse's
permissive 38-byte delimiters. It does not implement parsing for the Gooo program.

Inputs are 1..128 decoded valid UTF-8 strings, each at most 1,024 bytes, in an
input-only JSON envelope at most 32,768 bytes. The single original input key is
`go/github.com/google/uuid/text:ParseCanonical`. Native deliveries use the scalar
`input` field. Both receipts must report `OBSERVED`, zero finite expectations,
zero model calls, pinned clean compiler identity, an explicitly selected matching
Go executable, and two successful native runs with matching stdout hashes.

The checker validates source/export/call bindings, generation report provenance,
source and generated digests, reported typecheck/deterministic replay flags,
the generated four-field Go ABI, declared plan fields, runtime-to-plan digest
binding, exact outer replay parent hash, and equal saved program/composition and
outputs. It does not invoke a compiler or re-execute the generated program.

The report uses `gooo/uuid-parse-canonical-checked/v1` with decision
`PASS_FINITE_GO_ORACLE_AND_SAVED_REPLAY` and an explicit `verification_scope`.
The inner replay `prior_result_sha256` is required to be present and a digest,
but is **NOT_CHECKED** against a decoded typed Result. A literal-report serializer
is not a UUID serializer. Typed-plan digest recomputation, compiler projection
reconstruction, unmodeled fields and typed serialization are also **NOT_CHECKED**.
The report does not export an unverified inner hash as a computed hash.

Synthetic tests exercise rejection and scope behavior; they are not native
observations. Evidence consistency is unsigned and is not an attestation against
replacement of the complete evidence bundle. These finite Text observations do
not cover arbitrary-byte `ParseBytes`, the binary UUID method set, every string,
the complete module, browser DOM behavior, adoption or performance.
