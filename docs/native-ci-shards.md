# Native CI shard metadata contract (preparation only)

The optional `ci-shard-*` verifier flags prepare and validate a versioned
metadata contract. They do not execute a shard, inspect native receipt bytes,
or establish native acceptance. The default verifier and current CI workflow
remain unchanged. No CI speed improvement has been measured for this feature.

The planner reads the current embedded and on-disk expected ledger. It retains
every suite's original index, case/batch totals, source roster and selected
workspace. The present ledger has 82 suites, 175,649 cases and 2,305 batches;
these numbers are derived from the ledger rather than fixed in the planner.
Suites are assigned whole, ordered by descending batch count, then case count,
then original index, to the shard with the lowest assigned batch count. Ties
choose the lowest shard index. Each assignment retains catalog index order.
This deterministic heuristic is not a runtime estimate.

An independently trusted context file supplies `repository`, `run_id`,
`attempt`, `tested_commit`, `tested_tree`, `compiler_revision`, `go_version`
and `workers`. The checkout must be clean and match that commit and tree.
Current pins require `go1.27.2`, compiler revision
`83928fa4538245abe33b08549da47f8cbdc50ed2`, and three workers per planned runner.
Run ID, attempt and repository are caller assertions: reading a JSON file does
not authenticate GitHub or an artifact's producer. A future CI integration
must obtain and cross-check those values from trusted run/job metadata.

Run from the repository root. Write the context and new outputs outside the
checkout so they do not violate its clean-source precondition. After this code
is committed and independently reviewed, the metadata commands are:

```sh
go run ./cmd/verify --ci-shard-context /evidence/context.json \
  --ci-shard-count 3 --ci-shard-plan-out /evidence/plan.json

go run ./cmd/verify --ci-shard-context /evidence/context.json \
  --ci-shard-plan /evidence/plan.json --ci-shard-index 0 \
  --ci-shard-select-out /evidence/selection-0.json

go run ./cmd/verify --ci-shard-context /evidence/context.json \
  --ci-shard-plan /evidence/plan.json \
  --ci-shard-partial /evidence/partial-0.json \
  --ci-shard-partial /evidence/partial-1.json \
  --ci-shard-partial /evidence/partial-2.json \
  --ci-shard-reduce-out /evidence/reduction.json
```

Each command requires exactly one output mode and rejects incompatible full
execution flags before reading context/source data. Explicit shard flags with
empty/default values, parse errors and metadata positional arguments are also
rejected before source work; they never fall through to the full native path.
Inputs are bounded regular
files; symlink inputs, ambiguous JSON keys and overwriting outputs are rejected.
Selection emits metadata only. There is currently no command that produces
partials by running native suites, so the reducer command is not an end-to-end
native verification procedure.

The plan hash binds the complete ledger, profiles, tested source identities,
compiler/toolchain/run context, workspace manifests, entry producers and
deterministic assignments. Its reader roster adds the three shard implementation
and test files without changing the historical full-mode source ledger. Reduce
and select regenerate the plan independently from the clean checkout; a plan
with a matching self-declared hash alone is insufficient.
Each source's bytes and mode are checked against the immutable tested Git tree.
HEAD/tree/clean status and every snapshotted source's bytes/mode are checked
again after planning and immediately before writing each metadata output.

The separate `gooo/native-ci-shard-partial/v1` schema requires every planned
shard exactly once, its exact namespace and source/run identity, an explicit
joined state, and every assigned original suite exactly once. Each reported
batch retains ordered indices, contiguous case intervals, a deterministic
shard/suite/batch output namespace, workspace/source-set/entry-producer anchors
and well-formed execution/replay digests with a matching replay parent. All
seven existing exhaustive report contracts remain required for their reported
verified suites, including exact fields and generated-source bindings.

These checks establish metadata consistency only. The current ledger provides
suite counts, not original batch case bytes or their expected hashes. The
reducer does not recompute those bytes, check exact historical per-batch
boundaries, inspect execution or replay files, compare native values with Go,
authenticate joined resources, or prove that a digest names a real receipt.
`VERIFIED` in a partial is a reported producer state, not reducer-issued native
acceptance. Complete synthetic metadata can therefore satisfy this contract.

The only successful reduction state is
`CONTRACT_COMPLETE_NATIVE_UNCHECKED`, with `native_acceptance: false` and
`raw_receipts_inspected: false`. It never writes the original full summary,
timing footer, `COMPLETE` or a native `PASS`. Missing, duplicate, foreign,
unfinished or internally inconsistent partials fail closed. After all valid
joined shards are available, failures select the lowest original suite index;
lower indices within a failed shard must have completed first.

A future multi-runner mode must use `fail-fast: false`, join every runner and
then reduce the complete set. This differs from the existing single-process
coordinator's cancellation timing: other runners may finish additional work
after a failure. No immediate global-cancellation equivalence is claimed.
Before dispatch is enabled, a native adapter and independent raw receipt/case
reducer must preserve the original batch bytes, values, replay and exhaustive
checks, and verify producer/job/source metadata and complete artifact sets.
Workflow changes and their required checks need separate review and measurement.
