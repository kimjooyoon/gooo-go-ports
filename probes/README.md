# Reproducible language boundaries

These three small Gooo sources preserve the unsupported features encountered
when choosing Go libraries to port. They are deliberately kept outside `ports/`.
The loop and byte-index fixtures now support the upstream
[bounded traversal and Text byte-access request](https://github.com/kimjooyoon/meta-ontology-go/issues/1462)
for string-search ports. That request also records their unchanged rejection in
Gooo `0.6.29-dev`; the observations below retain this repository's pinned compiler.
It is a feature request, not an accepted implementation or an external customer report.
The bitwise fixture remains separate evidence without an upstream request here.

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

## Reproduce and keep the evidence

From the repository root, after building the pinned `.gooo` compiler:

```sh
./tools/probe.sh --out out/language-probes
```

The output directory must not exist. To choose another compiler explicitly:

```sh
./tools/probe.sh --compiler /absolute/path/to/gooo --out out/other-compiler-probes
```

The script records the actual compiler build identity, copies the three source
fixtures, and saves unmodified stdout and stderr for both commands. A
`summary.tsv` records their exit statuses and the checked error. The script
requires successful declaration checks, nonzero generation exits, and the exact
expected JSON `error` fields. Unexpected acceptance or a changed diagnostic
causes a failing script exit while retaining evidence for inspection.

For one fixture, the equivalent direct commands are:

```sh
./.gooo check probes/loop.gooo
./.gooo body-codegen --json --activity Probe probes/loop.gooo
```

The second command is expected to exit with status 1. No model or network
provider is needed for these syntax rejections, and no generated program runs.

## Why these boundaries matter for ports

Variable-length UTF decoding, parsing, and collection algorithms generally need
iteration. Byte indexing is a separate requirement from the supported Text
slicing operation. Bit operators are needed by many encoders, hashes, and binary
formats. The scalar UTF-16 ports in this repository replace specific guarded
shifts and masks with arithmetic; that does not establish general bit-operator
support. A future compiler version can use these sources to measure whether the
boundary changed, together with positive execution and replay cases for any
newly supported feature.
