# Reproducible language boundaries

These three small Gooo sources preserve the original boundaries encountered
when choosing Go libraries to port. They are deliberately kept outside `ports/`,
and their bytes remain unchanged as compiler support develops.
The loop and byte-index fixtures informed the upstream
[bounded traversal and Text byte-access request](https://github.com/kimjooyoon/meta-ontology-go/issues/1462)
for string-search ports, which has since been completed. That request also records
their earlier rejection in Gooo `0.6.29-dev`. The historical table below retains
its original compiler snapshot; the current observations are recorded separately.
This request came from port development and does not establish external customer demand.
The bitwise fixture remains separate evidence without an upstream request here.

## Historical generation observations

Observed with clean Gooo `0.6.24-dev` compiler source
`366ee6ef8f9d2b9b57331dfd26b6f908c0289851`, built with Go `1.27.2`:

| Source | `gooo check` exit | `body-codegen` exit | Exact generation error |
| --- | ---: | ---: | --- |
| `loop.gooo` | 0 | 1 | `unsupported activity statement *ast.ForStmt` |
| `byte-index.gooo` | 0 | 1 | `unsupported activity expression *ast.IndexExpr` |
| `bitwise.gooo` | 0 | 1 | `unsupported binary operator &` |

Each generation receipt reports `decision: FAIL_CLOSED`. `gooo check` accepts the
declarations, types, and embedded `computes` text in these examples. Its success
does not establish that the embedded body can be generated or executed.
`body-codegen` applies the pure-body syntax filter and rejects these specific
statements or expressions. Even successful body generation needs a separate
native execution check to establish behavior on supplied inputs.

## Current generation boundaries

The unchanged fixtures were observed with clean Gooo `0.6.32-dev` source
`83928fa4538245abe33b08549da47f8cbdc50ed2`, built with Go `1.27.2`:

| Source | `gooo check` exit | `body-codegen` exit | Generation decision / diagnostic |
| --- | ---: | ---: | --- |
| `loop.gooo` | 0 | 1 | `FAIL_CLOSED`: `for requires an existing counter initialized to 0, counter < literal bound, and counter = counter + 1` |
| `byte-index.gooo` | 0 | 0 | Wrapped report `PASS`; generated Go retains the bare `input[0]` access |
| `bitwise.gooo` | 0 | 1 | `FAIL_CLOSED`: `unsupported binary operator &` |

The condition-only loop fixture does not use the supported counted-loop shape.
The current compiler accepts the byte-index fixture for generation, but that
fixture assumes nonempty Text. A separate native observation with empty Text
reached `EXECUTE_1` and the child process exited with code 2 after an index-out-of-range
panic. Native compilation and process startup had already completed.
With `가x`, the same unchanged source returned the first unsigned UTF-8 byte,
`234` (`0xEA`), and its saved replay matched. These two observations establish
those inputs only; generation acceptance does not establish index safety.

The new [IndexByte port](../ports/strings/search.gooo) guards the Text length before
each byte access inside a bounded traversal. Its independent Go comparisons and
importing consumer belong to the native verification workflow. This probe runner
does not execute either program or count those comparisons.

## Reproduce and keep the evidence

From the repository root, with Python 3 and the pinned `.gooo` compiler built:

```sh
./tools/probe.sh --out out/language-probes
```

The output directory must not exist. To choose another build of the exact compiler
pin in `upstream.json` explicitly:

```sh
./tools/probe.sh --compiler /absolute/path/to/gooo --out out/other-compiler-probes
```

The script records the actual compiler build identity, copies `upstream.json`
and the three source fixtures, and saves unmodified stdout and stderr for both
commands. `command-statuses.tsv` retains actual command exits even if validation
fails. Validated `summary.tsv` and `summary.json` describe generation boundaries.
The validator checks the clean compiler revision, version and Go toolchain against
the expected pin, exact copied source bindings, and receipt producer fields.
It accepts the byte-index command's wrapped `PASS` report only with matching
activity identity, typecheck/replay flags, and generated Go digests. The loop and
bitwise commands must return raw `FAIL_CLOSED` reports with the current exact
diagnostics and no generated source. Missing bindings or changed outcomes cause
a failing script exit while retaining evidence for inspection.

For one fixture, the equivalent direct commands are:

```sh
./.gooo check probes/loop.gooo
./.gooo body-codegen --json --activity Probe probes/loop.gooo
```

The second command is expected to exit with status 1 and the counted-loop
diagnostic above. No model or network provider is needed for these generation
observations. The runner executes no generated Go; its successful exit describes
the checked generation contracts, without establishing runtime safety.

## Why these boundaries matter for ports

Variable-length UTF decoding, parsing, and collection algorithms generally need
iteration with explicit termination bounds and guarded byte access. Supported
Text byte indexing counts UTF-8 bytes rather than code points. Bit operators are
needed by many encoders, hashes, and binary
formats. The scalar UTF-16 ports in this repository replace specific guarded
shifts and masks with arithmetic; that does not establish general bit-operator
support. Keep earlier rejection records associated with their original compiler.
Future boundary changes require fresh generation observations and separate
positive execution, failure and replay cases for the affected behavior.
