# Slash path facts in Gooo

`go/path:Ext` and `go/path:IsAbs` adapt the corresponding Go 1.27.2 functions.
The original importing program, `app/path-facts:Report`, returns six required
fields: the original text, extension, absolute-path marker, original and
extension byte lengths, and whether an extension exists. Gooo computes every
classification and derived value; Go supplies the independent reference checks.

Inputs must be explicitly present, valid UTF-8 Text of at most 1,024 bytes.
Slash semantics apply on every operating system. Backslash is an ordinary
character; dots in directory names do not become file extensions. Dotfiles and
trailing dots retain Go's extension behavior. The report preserves the input
without cleaning or normalization and does not access or authorize a filesystem.
Arbitrary invalid-UTF-8 Go strings and longer inputs are outside this adaptation.

After following the [repository setup](../../README.md), use its pinned compiler
and Go 1.27.2 to run the example from the repository root:

```sh
./.gooo package execute --json --go "$(go env GOROOT)/bin/go" \
  --inputs examples/path-facts/inputs.json path-facts.workspace.json
```

To compare the results with Go and check fresh saved replay, choose a new output
directory for each run:

```sh
python3 -B tools/check-path-facts.py --repository . \
  --go "$(go env GOROOT)/bin/go" --compiler ./.gooo \
  --out out/path-facts --timeout 300
```

The checker validates two distinct layers:

- All 24 boundary inputs pass through actual CLI execution and saved replay for
  each of the three entries: 72 typed outputs in each phase.
- The checker extracts the freshly verified generated source, compiles its
  function bodies unchanged, and compares all 9,841 strings over `{a,.,/}` of
  lengths 0 through 8 for each entry: 29,523 generated native outputs and 78,728
  scalar comparisons, including all six Report fields. These are direct function
  calls, not 9,841 individual CLI or saved-replay observations.

It retains typed actual/reference rows, source and harness hashes, entry and
producer identities, and replay links. Fifteen strict input controls and eighteen
saved-receipt tamper controls must also be refused. The strict input boundary
rejects malformed raw UTF-8 and unpaired JSON surrogate escapes; the pinned bare
CLI's escape normalization is a separate observation. Native input-only receipts
remain `OBSERVED` with zero finite expectations. The separate checker establishes
its finite comparison result and writes success only after checking retained
evidence and removing its owned temporary work.

The existing verifier's 66 suites, 159,909 cases, 2,109 CI-profile batches and seven
exhaustive checks remain separate. These finite observations do not establish
whole-package or full UTF-8-domain compatibility, filesystem safety, or a speedup.
Historical receipts are not reused as fresh execution results.

The adaptation retains the Go Authors' copyright and BSD terms in
`third_party/GO-LICENSE`. `third_party/go/path.go.txt` is the byte-exact source
reference; `upstream.json` records its immutable revision and SHA-256. The original
Report uses the repository's BSD license. This independent project has no Go or
Google endorsement.
