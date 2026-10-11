# Run the Gooo tutorial example

This example clamps an integer score to 0..100 and returns a record containing
`score`, `remaining`, `in_range` and `label`. The two Gooo files are the same
sources as the [English tutorial contribution][tutorial]. That contribution is
pending independent review; this example does not imply the site's endorsement.

[Try the compiled example in your browser][browser]. Change the score and label
to run the existing program. To change its code, download the source bundle or
use the files here and run the native commands below. The browser runs a fixed
Gooo program compiled to WebAssembly; it does not compile source edits.

## Read the program

- `limits.gooo` declares Integer and Boolean types by their builtin identities
  and defines `Clamp` and `InRange`.
- `main.gooo` imports those activities, uses a local variable and a conditional,
  and returns a four-field `Summary` record. A label such as `sensor` becomes
  `sensor: low` for a clamped score below 60, or `sensor: high` otherwise.
- `gooo.workspace.json` maps the two import paths to these source files.
- `inputs.json` supplies four invocations, without expected outputs.

| Score | Label | Returned score | Remaining | In range | Returned label |
| --- | --- | --- | --- | --- | --- |
| -5 | sensor | 0 | 100 | false | sensor: low |
| 42 | sensor | 42 | 58 | true | sensor: low |
| 75 | sensor | 75 | 25 | true | sensor: high |
| 105 | sensor | 100 | 0 | false | sensor: high |

These are tutorial examples, not additional Go standard-library ports or
production decision rules. The existing port-compatibility counts are unchanged.

## Run from this repository

Use Git, a POSIX shell and Go toolchain selection, from the repository root:

```sh
export GOTOOLCHAIN=go1.27.2
export GOWORK=off
./tools/bootstrap.sh
./.gooo package execute --inputs examples/learn-gooo/inputs.json \
  learn-gooo.workspace.json
mkdir -p out/learn-gooo
./.gooo package execute --json --inputs examples/learn-gooo/inputs.json \
  learn-gooo.workspace.json > out/learn-gooo/execution.json
./.gooo package replay --json --receipt out/learn-gooo/execution.json \
  --inputs examples/learn-gooo/inputs.json learn-gooo.workspace.json \
  > out/learn-gooo/replay.json
```

The result of each entry invocation is at
`result.runtime.traces[].deliveries[].actual`. These input-only records report
`OBSERVED`. They record execution; they do not compare with independent Go
reference outputs or prove correctness for all inputs.

## Run the downloaded source bundle

Extract the bundle and enter the directory containing `limits.gooo`, `main.gooo`,
`gooo.workspace.json`, `inputs.json`, `README.md` and `NOTICE`. Keep the files
together, then fetch a shallow snapshot of the exact compiler used by this
example and build it. Run the commands in order in the same shell, and stop if
a command fails. Start without an existing `compiler` directory:

```sh
export GOTOOLCHAIN=go1.27.2
export GOWORK=off
mkdir compiler
git -C compiler init
git -C compiler remote add origin https://github.com/kimjooyoon/meta-ontology-go.git
git -C compiler fetch --depth=1 origin 83928fa4538245abe33b08549da47f8cbdc50ed2
git -C compiler checkout --detach FETCH_HEAD
cd compiler
go build -o ../gooo ./cmd/gooo
cd ..
./gooo version --build --json
./gooo check limits.gooo
./gooo check main.gooo
./gooo package execute --inputs inputs.json gooo.workspace.json
./gooo package execute --json --inputs inputs.json gooo.workspace.json \
  > execution.json
./gooo package replay --json --receipt execution.json \
  --inputs inputs.json gooo.workspace.json > replay.json
```

The build record should report the pinned compiler revision, Go 1.27.2 and
`vcs_modified: "false"`. A successful `check` alone validates declarations;
the `execute` command is what compiles and runs the bodies.

## Make one visible change

In `main.gooo` (`examples/learn-gooo/main.gooo` in a repository checkout), change
only the single `if score >= 60` condition to `if score >= 80`. Run the same four
inputs again. The score `75` result changes from `sensor: high` to `sensor: low`;
its `score: 75`, `remaining: 25` and `in_range: true` fields stay the same. The
other three original examples are unchanged. This is an observation of these
four inputs, not a proof for all inputs.

Keep the original receipts. From the repository root, save and replay the edited
program under new names:

```sh
mkdir -p out/learn-gooo
./.gooo package execute --json --inputs examples/learn-gooo/inputs.json \
  learn-gooo.workspace.json > out/learn-gooo/threshold-80-execution.json
./.gooo package replay --json --receipt out/learn-gooo/threshold-80-execution.json \
  --inputs examples/learn-gooo/inputs.json learn-gooo.workspace.json \
  > out/learn-gooo/threshold-80-replay.json
```

For the downloaded bundle, use `./gooo`, `inputs.json` and `gooo.workspace.json`
instead, with new `threshold-80-execution.json` and `threshold-80-replay.json`
filenames. Replay the newly created record; the original receipt retains the
original program. To return to the original example, change `80` back to `60`.

## Inputs and transport

Scores accept the full signed 64-bit range. The browser sends and displays them
as decimal strings so values outside JavaScript's safe integer range stay exact.
It does not wrap, clamp or round the input before Gooo receives it. The clamping
operation belongs to the example itself.

The native profile limits both a Text input and each returned Text record field
to 1,024 UTF-8 bytes. The returned label includes the suffix, so a label that
fits as an input can still make execution fail when its output exceeds that
limit. The browser follows these same limits.

JSON string mode can represent CR, LF, NUL and explicit surrogate escapes.
Go's JSON input binding turns an isolated surrogate escape into U+FFFD; the
browser shows the bound label explicitly. A valid surrogate pair retains its
Unicode scalar value. No trimming, case conversion or Unicode normalization is
part of this example.

Browser results are local observations of the compiled program. Downloaded
build receipts describe the native build inputs; they are not new native
receipts for values typed into the page.

[tutorial]: https://github.com/adambard/learnxinyminutes-docs/pull/5629
[browser]: https://kimjooyoon.github.io/gooo-go-ports/tutorial.html
