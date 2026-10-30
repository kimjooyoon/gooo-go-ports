---
name: gooo-first-program
description: Edit a small .gooo program, execute it, replay the saved execution and compare the observed change when a user wants to learn or try Gooo. General Go development does not need this skill.
license: BSD-3-Clause; see LICENSE.
---

# Gooo first program

Produce a Gooo source edit and its execution/replay records. The compiler
generates Go to execute supported bodies. Use a local shell, Git and Go toolchain
selection on macOS or Linux; preserve the user's files, changes and branch.

## Run the current tutorial

Run commands from the repository root in the same shell. Stop at the first
failure and retain its diagnostic before changing the source or setup.

1. **Read the selected checkout.** Reuse an existing `kimjooyoon/gooo-go-ports`
   checkout, or obtain a reviewed revision in a new task directory. Read its
   `AGENTS.md`, `examples/learn-gooo/README.md`, both `.gooo` files in that
   directory and `learn-gooo.workspace.json`. Follow that checkout's pins; the
   example imports `Clamp` and `InRange` from `limits.gooo` into `main.gooo`.

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
   executable's absolute path for `./.gooo` below. Go selection alone does not
   establish a matching compiler. Keep global agent settings unchanged.

3. **Save the original execution in a fresh directory.**

   ```sh
   gooo_trial_dir="$(mktemp -d "${TMPDIR:-/tmp}/gooo-first-program.XXXXXX")"
   ./.gooo version --build --json > "$gooo_trial_dir/compiler-build.json"
   ./.gooo package execute --json --inputs examples/learn-gooo/inputs.json \
     learn-gooo.workspace.json > "$gooo_trial_dir/before.json"
   ```

   Inspect `result.runtime.traces[].deliveries[].actual`: supplied score `75`
   returns `score: 75`, `remaining: 25`, `in_range: true` and
   `label: "sensor: high"`. Declaration `check` or saved generated Go is not execution.

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

   This checks typed fields, source digests, compiler identity and the edited
   replay's parent. It does not capture Git revision or support other edits/input
   sets; inspect those records directly. Report what ran, the source change,
   observed four-field results, compiler identity and retained record paths.
   Preserve the first failure; an incomplete attempt is not success.

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

## Conditional guidance

Read only the reference needed for the requested task:

- To reproduce the frozen Go 1.27.2 tutorial archive, read
  [legacy snapshot](references/legacy-snapshot.md).
- For an already configured ADK source diagnostic or a requested cooldown-facts
  follow-on, read the relevant section of
  [optional integrations](references/optional-integrations.md).
- To inspect a Gooo assessment over saved test outcomes, read
  [captured test assessment](references/captured-test-assessment.md).
