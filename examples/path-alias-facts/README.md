# Original lexical path alias facts

`app/path-alias-facts:Report` imports the Gooo `path.Clean` and `path.IsAbs`
adaptations. Its eleven required fields retain both original Text inputs, both
cleaned values, both original absolute markers, two change markers, exact
equality, clean equality and `lexical_alias` (`same_clean && !exact_same`).
The [contract](contract.json) fixes all field identities and ordering.

An automated API policy diagnostic can attach `lexical_alias` and both original
paths to a duplicate-key warning instead of silently replacing one supplied path
with another. No human approval is required to calculate these facts. The host
must establish that its routing or policy-key namespace actually uses these slash
semantics and bind the observations to the same current context. This program
neither authenticates that context nor grants access. Equal clean values are not
authorization, application identity, filesystem equivalence or a safe-path proof.

Both inputs must be present decoded valid UTF-8 Text of at most 1,024 bytes each.
Slash semantics apply on every OS. Backslash, percent escapes, NUL, Unicode and
lookalike slash characters are retained; there is no URL decoding, Unicode
normalization, Windows path handling, symlink lookup or filesystem operation.
Longer values and arbitrary invalid-UTF-8 Go strings are outside the adaptation.
Strict fixture validation is separate from bare CLI Unicode decoding.

After the [repository setup](../../README.md), run both entries from the
repository root with the pinned compiler and Go 1.27.2. Choose a fresh output
directory so saved receipts remain distinct:

```sh
mkdir -p out
mkdir out/path-alias-example
./.gooo package execute --json --go "$(go env GOROOT)/bin/go" \
  --inputs examples/path-alias-facts/clean-inputs.json path-clean.workspace.json \
  > out/path-alias-example/clean-execution.json
./.gooo package replay --json --go "$(go env GOROOT)/bin/go" \
  --receipt out/path-alias-example/clean-execution.json \
  --inputs examples/path-alias-facts/clean-inputs.json path-clean.workspace.json \
  > out/path-alias-example/clean-replay.json
./.gooo package execute --json --go "$(go env GOROOT)/bin/go" \
  --inputs examples/path-alias-facts/inputs.json path-alias-facts.workspace.json \
  > out/path-alias-example/report-execution.json
./.gooo package replay --json --go "$(go env GOROOT)/bin/go" \
  --receipt out/path-alias-example/report-execution.json \
  --inputs examples/path-alias-facts/inputs.json path-alias-facts.workspace.json \
  > out/path-alias-example/report-replay.json
```

`corpus-120.json` contains input pairs only. Its first 24 anchors preserve empty,
root/dot/parent behavior, backslash, percent, Unicode, NUL and exact 1,024-byte
boundaries. Another 96 deterministic pairs probe slash/dot/backtracking spelling
changes. Not every pair is expected to be an alias. Independent actual Go
`path.Clean` and `path.IsAbs` calls supply the reference facts; expected outputs
are not copied from the Gooo implementation or historical receipts.

The normal verifier appends two suites: 9,841 standalone Clean inputs and these
120 pairs. Every case requires fresh native execution and saved replay, all
eleven fields and both original inputs. This is separate from the existing
three-entry `Ext`/`IsAbs`/six-field `PathFacts` sidecar, which remains unchanged.
Finite fixture counts and source partition checks are not completed native or
hosted CI acceptance, full-domain compatibility or a performance claim.

This original composition uses the repository BSD license. Its imported
[Clean adaptation](../../ports/path/README.md) retains Go BSD attribution and
immutable source pins.
