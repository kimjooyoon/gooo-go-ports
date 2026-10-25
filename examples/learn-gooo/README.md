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
export GOTOOLCHAIN=go1.27.3
export GOWORK=off
sh tools/bootstrap.sh
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

Extract the bundle into a new directory and enter it. Keep all nine files,
including the `tools/` directory, together:

```text
limits.gooo
main.gooo
gooo.workspace.json
inputs.json
README.md
NOTICE
tools/bootstrap.sh
tools/bootstrap-check.go
upstream.json
```

Use Git, a POSIX shell and Go toolchain selection, as for the repository checkout.
The bundled installer, identity checker and version pins are unchanged copies
from the repository. The installer fetches the exact compiler with its required
version-tag history, builds it and checks its build identity before publishing
`.gooo`. The first setup requires network access for the compiler and any missing
Go toolchain or dependencies; the source ZIP does not contain a prebuilt compiler.
Run the commands in order in the same shell, and stop if a command fails:

```sh
export GOTOOLCHAIN=go1.27.3
export GOWORK=off
sh tools/bootstrap.sh
./.gooo check limits.gooo
./.gooo check main.gooo
./.gooo package execute --inputs inputs.json gooo.workspace.json
./.gooo package execute --json --inputs inputs.json gooo.workspace.json \
  > execution.json
./.gooo package replay --json --receipt execution.json \
  --inputs inputs.json gooo.workspace.json > replay.json
```

Bootstrap prints the checked build record, including the pinned compiler
revision, Go 1.27.3 and `vcs_modified: "false"`. Calling it through `sh` also works
when ZIP extraction does not preserve executable file permissions. A successful
`check` alone validates declarations; `execute` compiles and runs the bodies.

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

From the downloaded bundle's directory, save and replay the edited program with:

```sh
./.gooo package execute --json --inputs inputs.json gooo.workspace.json \
  > threshold-80-execution.json
./.gooo package replay --json --receipt threshold-80-execution.json \
  --inputs inputs.json gooo.workspace.json > threshold-80-replay.json
```

Both paths replay the newly created record; the original receipt retains the
original program. To return to the original example, change `80` back to `60`.

## Compare the saved change

From a repository checkout containing `cmd/check-tutorial-change`, capture the
compiler's build information separately and compare the three saved records:

This comparison accepts records produced with the repository workspace commands
above. Records produced from the downloaded bundle use different source paths
and are not supported by this checker.

```sh
./.gooo version --build --json > out/learn-gooo/compiler-build.json
go run ./cmd/check-tutorial-change \
  --before out/learn-gooo/execution.json \
  --after out/learn-gooo/threshold-80-execution.json \
  --replay out/learn-gooo/threshold-80-replay.json \
  --compiler-build out/learn-gooo/compiler-build.json
```

The JSON summary joins the source digests, compiler identity, all four fields for
all four supplied inputs, and the link from replay to the edited record. It
checks this exact threshold-60 to threshold-80 example. A changed input, another
edit or an unsupported record format is refused with nonzero exit and no result
on stdout. Keep that diagnostic and the original records.

A source digest identifies the source represented in a record; it is separate
from a raw file checksum or repository Git commit. A local source edit can leave
Git HEAD unchanged. The full compiler build information is also separate from
the generated program's runtime build timings. These are unsigned, finite local
observations and consistency checks, not proof of execution authenticity,
correctness for every input or permission to perform an API action.

For an automated fresh run without editing repository sources, use
`python3 tools/verify-tutorial-change.py --out out/tutorial-change-new` from the
repository root after setup. This optional runner also needs Python 3, keeps the
three records and summary, and removes its own temporary workspace and Go cache.
Select an installed Go 1.27.3 executable on `PATH` or with `--go /path/to/go`;
the runner disables toolchain switching and module downloads. Choose a new
output directory each time. CI reuses the already checked original
tutorial record from the same job, then executes and replays the copied edit.

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
The browser UTF-8 binding turns isolated UTF-16 surrogates into U+FFFD and
shows the bound label explicitly. Native comparison inputs encode that bound
scalar Text; the compiler rejects raw unpaired JSON surrogate escapes. A valid
surrogate pair retains its
Unicode scalar value. No trimming, case conversion or Unicode normalization is
part of this example.

Browser results are local observations of the compiled program. Downloaded
build receipts describe the native build inputs; they are not new native
receipts for values typed into the page.

[tutorial]: https://github.com/adambard/learnxinyminutes-docs/pull/5629
[browser]: https://kimjooyoon.github.io/gooo-go-ports/tutorial.html
