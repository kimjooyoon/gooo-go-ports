# Optional integrations

Read only the section matching the user's task. These are conditional follow-ons;
keep the first-program source and receipts. Neither changes the current
tutorial's Go **1.27.3** pins or authorizes live API effects.

## Diagnose an already configured ADK source failure

Use only an already configured
[upstream bridge at `b875196c`](https://github.com/kimjooyoon/meta-ontology-go/blob/b875196c4e3b2c36402a91346c41e320f877eed0/integrations/adk-go/README.md).
Its adapter setup is separate from the checkout's pinned compiler. This route
comes from upstream source and parity-test review; it is not evidence of a new
ADK or agent execution. The immutable
[diagnostic guidance](https://github.com/kimjooyoon/gooo-go-ports/blob/c3f6cdaf7c9f2f84bcb0f9e0076ade2d72eb3ec2/docs/ai-agents.md#choose-a-task-that-matches-the-current-capability)
records its scope.

- Preserve the original tool result and `exit_code`, then decode `report_json`.
  Diagnostics use `schema_version: "gooo/diagnostics/v1"`, `status` and
  `diagnostics`; successful discovery uses
  `schema: "gooo/capability-discovery-report/v1"`. `PROGRESS` or `UNKNOWN`
  evidence does not establish implementation success.
- Prefer a source location already supplied by `gooo_discover_capability`.
  Only when it reports `SOURCE_LOWER_FAILED` without a useful location, call
  `gooo_check_source` **at most once**, with the same source-root-relative
  `path`, unchanged source, fixed binary and root. Its `semantic.lowering`
  diagnostic may locate the unresolved reference.
- Preserve both results and the original failure/exit status even if the check
  succeeds. If neither supplies a location, report it as unavailable. This is
  diagnostic evidence only: do not retry discovery, edit inputs, resume
  execution or change the checkout's compiler pin. The check does not execute
  the program or grant authority.

## Try the frozen cooldown-facts follow-on

Use this only for a requested API-automation follow-on. The
[public trial](https://github.com/kimjooyoon/gooo-go-ports/blob/f8eae4204ef0aef6b8c3801cf6a1cf46da7a4f5a/docs/ai-agents.md#choose-a-task-that-matches-the-current-capability)
uses its own checkout at `2b2d972f40057b1c9339c89afcb8b3be161a60c9`, compiler
`83928fa4538245abe33b08549da47f8cbdc50ed2` and Go **1.27.2**. Follow that
snapshot's pins, validator and
[Retry-After guide](https://github.com/kimjooyoon/gooo-go-ports/blob/2b2d972f40057b1c9339c89afcb8b3be161a60c9/examples/retry-after-facts/README.md);
do not switch the current tutorial's checkout or overwrite its compiler.

1. Reuse that exact checkout or obtain it in a separate task directory. Read
   its `AGENTS.md`. Bootstrap may download and build the pinned compiler when
   no matching installation is available. Preserve the repository source,
   its 113-input corpus and earlier receipts.

2. From that checkout root, select Go **1.27.2**, create fresh paths, validate
   compiler identity with that snapshot's validator, and copy the request.
   Stop and preserve the first failure before continuing:

   ```sh
   export GOTOOLCHAIN=go1.27.2
   export GOWORK=off
   gooo_cooldown_dir="$(mktemp -d "${TMPDIR:-/tmp}/gooo-cooldown.XXXXXX")"
   ./.gooo version --build --json > "$gooo_cooldown_dir/compiler-build.json"
   go run ./tools/bootstrap-check.go "$gooo_cooldown_dir/compiler-build.json"
   cp examples/retry-after-facts/request.json "$gooo_cooldown_dir/request.json"
   ```

   Change only `elapsed_seconds` from `59` to `60` in the copied request.
   Keep all six original observations; do not normalize the header or inputs.

3. Run the strict one-request transport gate before native execution, then
   replay the saved execution using the same inputs, workspace and selected
   Go executable. Stop at the first failure and retain command, exit status
   and diagnostic:

   ```sh
   gooo_cooldown_go="$(go env GOROOT)/bin/go"
   python3 -I -B examples/retry-after-facts/request_inputs.py \
     "$gooo_cooldown_dir/request.json" > "$gooo_cooldown_dir/inputs.json"
   ./.gooo package execute --json --go "$gooo_cooldown_go" \
     --inputs "$gooo_cooldown_dir/inputs.json" \
     retry-after-facts.workspace.json > "$gooo_cooldown_dir/execution.json"
   ./.gooo package replay --json --go "$gooo_cooldown_go" \
     --receipt "$gooo_cooldown_dir/execution.json" \
     --inputs "$gooo_cooldown_dir/inputs.json" \
     retry-after-facts.workspace.json > "$gooo_cooldown_dir/replay.json"
   ```

   The gate rejects missing, wrong-type, duplicate or unknown keys, malformed
   UTF-8, unpaired surrogates and headers over 64 UTF-8 bytes before invoking
   the compiler; its raw request limit is 4,096 bytes. Use the copied request,
   not the full 113-input corpus, which exceeds this compiler's 32 KiB limit.

4. Inspect all fourteen actual fields at
   `result.runtime.traces[0].deliveries[0].actual`. At this boundary expect
   `state="READY_TO_REASSESS"`, `reason="COOLDOWN_ELAPSED"` and
   `remaining_seconds=0`; preserve the six inputs and every result field.
   Report what actually ran or the first failure, actual checkout revision,
   compiler/Go identity and agent/OS when available. Include one API or
   automation task this helps with, or say it does not help.

These facts do not send a request, start a timer or authorize a retry. Saved
replay does not refresh supplied facts; current request binding, guidance and
a host-owned atomic claim remain separate. Input-only `OBSERVED` is not the
full guide's independent Go-reference verification.

Sharing feedback is optional and requires the user's instruction. Use the
[existing feedback form](https://github.com/kimjooyoon/gooo-go-ports/issues/new?template=problem.yml),
category **Native setup, verification, or replay**, only if requested. Use the
synthetic public inputs, keep raw receipts and local paths private, and review
diagnostics before sharing. Upload or attach nothing automatically.
