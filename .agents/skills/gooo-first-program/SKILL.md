---
name: gooo-first-program
description: Write, run and replay a small Gooo program when a user wants to learn or try Gooo. Use the existing two-file example to make an observable source change; general Go development does not need this skill.
license: BSD-3-Clause; see LICENSE.
---

# Gooo first program

Help the user produce a Gooo source edit, its actual execution result and a new
saved execution/replay pair. Gooo is an experimental language; the compiler
generates Go to execute supported bodies. This workflow needs a local shell,
Git and Go toolchain selection on macOS or Linux.

## Find the example

Use an existing `kimjooyoon/gooo-go-ports` checkout when one is available. Read
its `AGENTS.md`, `examples/learn-gooo/README.md`, the two `.gooo` files in that
directory and `learn-gooo.workspace.json` before editing. Follow that checkout's
toolchain pins. Keep the user's existing changes and branch.

For a new workspace, the documented starting point is the immutable
[example guide](https://github.com/kimjooyoon/gooo-go-ports/blob/e1e279a25094f55a2ebd926a5e8393448eae418c/examples/learn-gooo/README.md).
Obtain that repository snapshot in a new task directory. The snapshot uses
compiler `83928fa4538245abe33b08549da47f8cbdc50ed2` and Go `1.27.2`.
Its `tools/bootstrap.sh` downloads and builds the pinned compiler; Go toolchain
selection may download Go too. Reuse a compatible installation where available.

Alternatively, use the matching [v0.6.32-dev archive and SHA256SUMS](https://github.com/kimjooyoon/meta-ontology-go/releases/tag/v0.6.32-dev).
Verify the matching archive checksum before extracting to a fresh directory.
From the example checkout, run the unpacked executable's `version --build --json`
into a new identity file, then `go run ./tools/bootstrap-check.go /absolute/path/to/identity.json`
with Go **1.27.2** selected. Preserve existing `.gooo`, `.compiler` and receipts.
Only after validation, use that executable's absolute path instead of `./.gooo`
and skip bootstrap below; native execution still needs Go **1.27.2**. Do not use latest
as a replacement for the exact compiler pin. The source-build route stays available.

Do not change the user's global agent configuration to run this example.

## Execute and edit

From the repository root, use the guide's setup and execution commands:

```sh
export GOTOOLCHAIN=go1.27.2
export GOWORK=off
./tools/bootstrap.sh
./.gooo package execute --inputs examples/learn-gooo/inputs.json \
  learn-gooo.workspace.json
```

If the user already has a compiler for this checkout, inspect its build identity
against the documented pins before reusing it. Bootstrap is unnecessary when a
matching executable is already present. A declaration `check` is not execution.

The example imports `Clamp` and `InRange` from `limits.gooo` into `main.gooo`.
It returns `score`, `remaining`, `in_range` and `label`. With the supplied inputs,
score `75` returns label `sensor: high`, score `75`, remaining `25` and
`in_range: true`. For the first source change, replace the single `if score >= 60`
condition in `examples/learn-gooo/main.gooo` with `if score >= 80` and execute
again. The score-75 label becomes `sensor: low`; the other three fields and the
other three supplied examples stay unchanged. Use the user's requested change
instead when they have specified one, and inspect its actual results.

Save the edited program's execution and replay under fresh filenames. The guide
shows `out/learn-gooo/threshold-80-execution.json` and
`out/learn-gooo/threshold-80-replay.json`; if those names already exist, choose
new names. Run `package execute --json`, then `package replay --json` with that
new execution record, the same inputs and the same workspace. Preserve the
original program's records rather than replaying them as evidence for an edit.
Stop at a failed command and report its first diagnostic before changing the
source or setup.

## Interpret the result

Inspect `result.runtime.traces[].deliveries[].actual` in the full execution
record. Report the changed source and observed four-field results, the compiler
identity actually used and the paths of the new execution/replay records.
Input-only runs report `OBSERVED`: they are finite execution observations, not
independent expected-output verification or proof for all inputs. State which
commands actually ran and preserve failures as failures.

For the supplied threshold-60 to threshold-80 edit, a checkout containing
`cmd/check-tutorial-change` can use the guide's **Compare the saved change**
command. Give it the original execution, edited execution, edited replay and
a separate `version --build --json` file. It checks the exact four supplied
inputs and summarizes their typed fields, source digests and replay links.
Keep the full records: an unsigned consistency summary does not authenticate
execution. It does not capture the repository Git revision or support other
edits; inspect those edits' actual records directly rather than forcing this
fixed comparison profile.

The example uses signed 64-bit integers. This pinned native profile limits Text
inputs and each returned Text field to 1,024 UTF-8 bytes. The output label adds a
suffix, so a label that fits as input can still exceed the output limit. Keep
the requested domain; do not silently truncate strings or replace Gooo behavior
with handwritten Go. The browser tutorial runs a fixed compiled program; use
native execution to verify edited source.

For a request outside this example, use the checkout's language and porting
documentation to assess support before promising an implementation. The fixed
API-policy Action compares two sealed profiles; it does not load this edited
tutorial or arbitrary policies. This example does not authorize live API effects.

For an API-automation follow-on after the tutorial, see the
[cooldown-facts trial](https://github.com/kimjooyoon/gooo-go-ports/blob/f8eae4204ef0aef6b8c3801cf6a1cf46da7a4f5a/docs/ai-agents.md#choose-a-task-that-matches-the-current-capability).
It uses a separate pinned checkout and optional feedback route. Keep this
tutorial and its receipts; the supplied cooldown facts do not authorize retries.
