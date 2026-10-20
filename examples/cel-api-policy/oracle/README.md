# Independent real-CEL reference

`cmd/oracle/main.go` compiles and evaluates the pinned Google codelab expression
using actual `cel.dev/cel-go@v0.32.0` with
`google.rpc.context.AttributeContext.Request`, not a copied Go implementation of
the Gooo formula. It evaluates comparison, absence and effective-principal
observations separately. `principal_defaulted` is original raw adapter-presence
metadata, not a CEL/protobuf scalar-presence property.

`source-pin.json` preserves the original Google codelab commit, source hashes,
license, and the separate CEL runtime dependency identity and sums. The Google
2020 source at revision `6d32ef54e4f124929730b15d5eb9b9205da58992` is retained
unchanged under `upstream/`. Its Apache-2.0 license is `upstream/LICENSE`; the
runtime release's complete license is also retained as `../LICENSE-APACHE-2.0`.

`cases.json` and `observations.json` are the unmodified retained 52-case corpus
and its archived real-CEL execution. They are baseline evidence, not a fresh run
on the current checkout. The public example's verifier runs this oracle anew and
compares current observations with those original bytes.
`api-composition-cases.json` and `api-spec-receipt.json` are 32 separately authored
expectations for the original API composition; they are not upstream CEL
observations of authentication, tenancy or external API effects.

The oracle module is deliberately separate from the dependency-free native
example. Setup with Go1.27.3 and `go mod download` in this directory, then run
`go run ./cmd/oracle --cases cases.json` with cached dependencies. Current output
contains a fresh execution timestamp. Full module graph, private implementation
proofs and built executables are not committed.
