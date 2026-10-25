---
name: gooo-first-program
description: Edit a small .gooo program, execute it, replay the saved execution and compare the observed change when a user wants to learn or try Gooo. General Go development does not need this skill.
license: BSD-3-Clause; see LICENSE.
---

# Gooo first program

Produce an actual Gooo source edit and its execution/replay records. The compiler
generates Go to execute supported bodies. Use a local shell, Git and Go toolchain
selection on macOS or Linux; keep the user's existing files, changes and branch.

## Run the current tutorial

Run each command from the repository root in the same shell. Stop on a failed
command and retain its diagnostic before changing the source or setup.

1. **Read the selected checkout.** Reuse an existing `kimjooyoon/gooo-go-ports`
   checkout, or obtain a reviewed revision in a new task directory. Read its
   `AGENTS.md`, `examples/learn-gooo/README.md`, both `.gooo` files in that
   directory and `learn-gooo.workspace.json`. Follow that checkout's pins.
   The example imports `Clamp` and `InRange` from `limits.gooo` into
   `main.gooo`.

2. **Set up the matching compiler.** This current-source workflow targets Go
   **1.27.3**. Bootstrap may download Go and the pinned compiler source, then
   build the compiler:

   ```sh
   export GOTOOLCHAIN=go1.27.3
   export GOWORK=off
   ./tools/bootstrap.sh
   ```

   If a matching compiler is already available, validate its
   `version --build --json` record using this checkout's
   `tools/bootstrap-check.go` before skipping bootstrap. Substitute that
   executable's absolute path for `./.gooo` below. Selecting Go alone does not
   establish a matching compiler. Keep the user's global agent settings unchanged.

3. **Save the original execution in a fresh directory.**

   ```sh
   gooo_trial_dir="$(mktemp -d "${TMPDIR:-/tmp}/gooo-first-program.XXXXXX")"
   ./.gooo version --build --json > "$gooo_trial_dir/compiler-build.json"
   ./.gooo package execute --json --inputs examples/learn-gooo/inputs.json \
     learn-gooo.workspace.json > "$gooo_trial_dir/before.json"
   ```

   Inspect `result.runtime.traces[].deliveries[].actual`. The supplied score
   `75` returns `score: 75`, `remaining: 25`, `in_range: true` and
   `label: "sensor: high"`. A declaration `check` or saved generated Go is not
   an execution.

4. **Make the requested source change.** For the first tutorial edit, replace
   the single `if score >= 60` condition in
   `examples/learn-gooo/main.gooo` with `if score >= 80`. Keep the four input
   rows unchanged. Use the user's requested change instead if they specified one.

5. **Execute the edit and replay that new record.**

   ```sh
   ./.gooo package execute --json --inputs examples/learn-gooo/inputs.json \
     learn-gooo.workspace.json > "$gooo_trial_dir/after.json"
   ./.gooo package replay --json --receipt "$gooo_trial_dir/after.json" \
     --inputs examples/learn-gooo/inputs.json learn-gooo.workspace.json \
     > "$gooo_trial_dir/replay.json"
   ```

   For the supplied edit, score `75` changes to `sensor: low`; its other three
   fields and the other three supplied examples stay unchanged. Inspect the
   actual result. Keep the original record; it is not replay evidence for an edit.

6. **Compare and report the saved result.** For this exact threshold-60 to
   threshold-80 edit and the four supplied inputs, a checkout containing
   `cmd/check-tutorial-change` can check the three records together:

   ```sh
   go run ./cmd/check-tutorial-change \
     --before "$gooo_trial_dir/before.json" \
     --after "$gooo_trial_dir/after.json" \
     --replay "$gooo_trial_dir/replay.json" \
     --compiler-build "$gooo_trial_dir/compiler-build.json" \
     > "$gooo_trial_dir/comparison.json"
   ```

   The comparison checks typed fields, source digests, compiler identity and the
   edited replay's parent. It does not capture Git revision or support other
   source edits/input sets; for those, inspect the actual records directly.
   Report what ran, the source change, observed four-field results, compiler
   identity and paths of the retained records. Preserve the first failure
   instead of calling an incomplete attempt successful.

## Interpretation and limits

Input-only records report `OBSERVED`: finite execution observations, not
independent expected-output verification or a proof for all inputs. The saved
comparison is an unsigned consistency check, not execution authentication.

This example uses signed 64-bit integers and limits Text inputs and each
returned Text field to 1,024 UTF-8 bytes. Adding the label suffix can exceed the
output limit even when the input fits. Preserve the requested domain; do not
truncate values or replace Gooo behavior with handwritten Go.

The browser tutorial runs a fixed compiled program. Use native execution to
verify a source edit. For work beyond this example, read the checkout's language
and porting guides before promising support. The fixed API-policy Action cannot
load this edited tutorial or arbitrary policies. These records authorize no
live API effects.

## Other starting points

Read only the guidance needed for the actual task:

- **Reproduce the older snapshot:** use the
  [archived installation procedure](https://github.com/kimjooyoon/gooo-go-ports/blob/c3f6cdaf7c9f2f84bcb0f9e0076ade2d72eb3ec2/.agents/skills/gooo-first-program/SKILL.md#find-the-example)
  for `e1e279a`, compiler `83928fa`, Go **1.27.2** and the matching
  **v0.6.32-dev** archive. Verify its checksum and build identity using that
  snapshot's validator. It is a separate reproduction, not a Go 1.27.3 compiler.
- **Diagnose an already configured optional ADK failure:** if
  `gooo_discover_capability` reports `SOURCE_LOWER_FAILED` without a useful
  location, follow the
  [single diagnostic-only check](https://github.com/kimjooyoon/gooo-go-ports/blob/c3f6cdaf7c9f2f84bcb0f9e0076ade2d72eb3ec2/docs/ai-agents.md#choose-a-task-that-matches-the-current-capability).
  Preserve the original failure and source; this does not resume execution or
  change the checkout's pin.
- **Try API automation after the tutorial:** the
  [cooldown-facts trial](https://github.com/kimjooyoon/gooo-go-ports/blob/f8eae4204ef0aef6b8c3801cf6a1cf46da7a4f5a/docs/ai-agents.md#choose-a-task-that-matches-the-current-capability)
  has its own pinned setup. Keep this tutorial and its receipts; cooldown facts
  do not authorize retries.
