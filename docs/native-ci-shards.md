# Experimental native CI shard contracts

The optional plan/select/reduce flags prepare and validate a versioned metadata
contract. Separate experimental execute/verify flags run native work and inspect
actual evidence. Neither path authenticates a GitHub producer or replaces the
existing required CI result. The default verifier and current CI workflow remain
unchanged. No CI speed improvement has been measured for this feature.

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

The separate `Manual full82 native shard experiment` workflow is opt-in through
`workflow_dispatch`. Supply the exact reviewed commit as `expected_source_sha`;
it must match the workflow and checked source SHA. This route leaves required
`ci.yml` unchanged. It has a planner, three matrix runners (`fail-fast: false`,
`max-parallel: 3`), and a joined inspection with both header-addition consumers.
Each executor still uses the contract's three workers, so the three runners can
have up to **nine suite callbacks** in total. This is not a global three-callback
limit. No local full-native execution or hosted speed comparison is implied.

The planner builds one pinned compiler and verifier, then transfers those exact
bytes to all workers and the join on homogeneous Linux amd64 runners. Go binary
bytes must also match. Every original82-suite/175,649-case/2,305-batch obligation
and all seven exhaustive reports remain; the join creates a fresh reference
index from the original transferred `partial.json` and raw evidence directories.
Original receipt bytes and producer manifest paths are never rewritten.
The seven exhaustive Go tests run through the existing suite verifier, and the
raw reader/exporter re-inspects their generated source, harness, module,
diagnostic and report. The 2,305 batch execute/replay pairs yield 4,610 separate
wait records; that number does not include the seven exhaustive Go test calls.

The transport helper reads the official current run, attempt jobs and artifact
metadata with `contents: read` and `actions: read`. It refuses incomplete or
failed workers and missing, duplicate, foreign or changed context, plan, tool or
artifact bindings. Names and producer handoffs do not directly attest which job
uploaded bytes: artifact SHA256/length, official source/run metadata, job state
and creation-time reconciliation are a separate evidence layer. The underlying
reader continues to return `NATIVE_RECEIPTS_CHECKED_PRODUCER_UNAUTHENTICATED`,
`producer_authenticated: false` and `native_acceptance: false`.

The first hosted attempt (`39133306424`, source `64b95b67`) failed before
completing the full comparison. One worker received `queued`/null run metadata;
that pair is now accepted only alongside the exact current job's separate
`in_progress`/null check. Failed, cancelled, unknown and foreign records still
refuse. Two other workers reported missing relative standard-library paths from
the shared `-trimpath` verifier. Every planner/worker/join child now receives an
explicit `GOROOT` from the selected canonical Go installation and its `bin`
first on `PATH`, replacing ambient `GOROOT`. The path is observed per job;
existing Go binary/version and source hashes remain the cross-runner identity.
A bounded local Go1.27.2 probe reproduced the empty root and verified both source
reads with the same binary and explicit root. This does not establish a successful
hosted full82 run or a speed improvement; both remain unverified after this fix.

Artifacts expire after one day. Downloads are capped at 512MiB per ZIP; each
extracted artifact is capped at 2GiB, 100,000 entries and 64MiB per file. These
finite experimental limits have not been measured against a hosted full shard
run; exceeding them fails and preserves the first diagnostic. Planner/worker/join
logs and original evidence are uploaded even on failure when available. The
join records actual command/download intervals and compressed/uncompressed
bytes; runner setup, queueing and upload intervals remain in official job
metadata. Independent runner clocks are not combined into the old timing
footer. Total runner time and storage may increase. Successful default full CI,
review and a first actual hosted full run remain necessary before making any
claim about this experimental route's operational benefit.

The planner/worker/join phase budgets are 9/38/42 minutes inside 15/45/50-minute
jobs, leaving separate setup and preservation time. SIGTERM/SIGINT interrupt the
owned command, kill its process group and wait for the direct child before
recording the first failure. Both command logs are sampled every 250ms with an
8MiB limit; owned work is sampled every 2s with 3GiB per producer or 8GiB at join.
Sampling and shutdown checks add overhead and are not kernel quotas. A hard
runner kill, exhausted disk or platform cancellation can still prevent upload;
no successful preservation is claimed without the resulting artifact.

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
Selection and metadata reduction emit metadata only. Native execution and raw
receipt inspection require the separate explicit commands below.

The plan hash binds the complete ledger, profiles, tested source identities,
compiler/toolchain/run context, workspace manifests, entry producers and
deterministic assignments. Its reader roster adds the explicit shard implementation
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

## Explicit native execution and raw receipt inspection

These experimental commands require the same clean, reviewed source and plan.
Use Go1.27.2 and the pinned compiler. Each output directory must be fresh, and
its parent must already exist. For each planned index, run separately:

```sh
go run ./cmd/verify --ci-shard-context /evidence/context.json \
  --ci-shard-plan /evidence/plan.json --ci-shard-index 0 \
  --ci-shard-native-compiler /tools/gooo \
  --ci-shard-execute-out /evidence/native-0
```

The executor validates the compiler build identity, uses the existing suite
verifier and three-worker coordinator, waits for callbacks and closes its one
runner-private cache before re-inspecting raw files. Original case generators,
profile-specific partitions, execute/replay and the seven exhaustive checks are
reused. Every batch uses its original index in a separate shard/suite namespace.
Failed or incomplete evidence and the first diagnostic remain retained; a
partial is not the old full-run summary or timing footer.

After all runners have joined, verify their original partials and evidence:

```sh
go run ./cmd/verify --ci-shard-context /evidence/context.json \
  --ci-shard-plan /evidence/plan.json \
  --ci-shard-partial /evidence/native-0/partial.json \
  --ci-shard-partial /evidence/native-1/partial.json \
  --ci-shard-partial /evidence/native-2/partial.json \
  --ci-shard-evidence /evidence/native-0 \
  --ci-shard-evidence /evidence/native-1 \
  --ci-shard-evidence /evidence/native-2 \
  --ci-shard-reference-out /evidence/native-reference.json \
  --ci-shard-verify-out /evidence/native-verification.json
```

Pair the partials and evidence directories in the same order. The reader
regenerates actual Go expectations and exact original case, manifest and copied
source bytes. It reuses execution/replay inspection, checks the raw saved parent,
and inspects each required exhaustive report, generated program and harness.
Embedded report hashes describe canonical decoded JSON, so indenting a partial
does not change its meaning. Compiler JSON and per-receipt producer identities
are checked; these do not independently authenticate a job or joined resources.
The reader refuses symlink descendants, outside paths, duplicate or missing
namespaces and changed inspected bytes.

A successful raw verification has state
`NATIVE_RECEIPTS_CHECKED_PRODUCER_UNAUTHENTICATED`, with
`raw_receipts_inspected: true`, `producer_authenticated: false` and
`native_acceptance: false`. Complete metadata alone cannot produce this result.
No workflow consumes it as an accepted replacement for the full CI gate.

The optional `--ci-shard-reference-out` additionally writes a separate
`gooo/native-shard-reference-index/v1` for downstream consumers. It requires
each original paired `partial.json`, complete actual execute/replay wait
observations and unchanged compiler/Go/verifier binary snapshots. Missing
observations remain `NOT_OBSERVED`; they are not filled with invented
`NOT_AVAILABLE` values. Wait CPU/RSS observations describe the already-waited
CLI and do not measure simultaneous memory or the actual child environment.
These files retain their own schema and do not replace a legacy timing footer.

Every reference binds the local shard root, safe relative blob path, length and
raw hash. The receipt's original producer manifest path is a separate string;
consumers compare it as data and never open it. Original receipts, manifests,
case bytes and sources are not rewritten after artifact transfer. Exhaustive
generated source, harness, module, diagnostic and report are semantically
re-inspected with the same reader that exports their byte references.

The header-addition consumers have an explicit experimental route:

```sh
python3 -I -B tools/check-header-addition-host.py --root "$PWD" \
  --shard-native-index /evidence/native-reference.json \
  --go /tools/go --out /evidence/fresh-host

python3 -I -B tools/check-header-addition-inputs.py --root "$PWD" \
  --shard-native-index /evidence/native-reference.json \
  --compiler "$PWD/.gooo" --input-command /tools/header-addition-input \
  --go /tools/go --out /evidence/fresh-input-route
```

These routes require the complete current source/plan and original case/batch
population before selecting the seven addition batches. They share the
existing receipt/ABI/Go-expectation and atomic-host checks. Their provider
checks raw-file consistency; it does not independently rerun every Go oracle
or authenticate a GitHub producer. The normal `--verification-out` plus
`--timing` route keeps its full legacy checks. Mixed modes and incomplete
shard references are refused; there is no fallback between them.

The optional build-tagged `gooo_shard_native_smoke` test exercises one small
StatusText fixture with an actual pinned compiler and independent Go values.
It is separate from the normal observer tests and the full82-suite CLI plan.
It neither proves the full partition executed nor measures a speed improvement.

The optional multi-runner mode uses `fail-fast: false`, joins every runner and
then reduces the complete set. This differs from the existing single-process
coordinator's cancellation timing: other runners may finish additional work
after a failure. No immediate global-cancellation equivalence is claimed.

The manual comparison uses a separate, non-required experimental workflow
on the same exact source, keeping the current full gate. Before replacing that
gate, the experimental addition-input and atomic-host routes must be validated
against a complete actual shard run and its original batch evidence. Their
normal route still uses the flat `out/ci` layout and one process's timing footer. Combining runner clocks
into that footer or writing a substitute legacy summary would not preserve the
contract. Authenticated run/job/artifact provenance and a measured end-to-end
benefit, including evidence transfer and reduction, are also required.
The first dispatch remains an explicitly reviewed experiment: the complete
hosted partition and failure paths have not been observed by these local
transport tests. Official metadata reconciliation does not complete the separate
producer-authentication requirement. Retain every original required check;
workflow changes need review and actual time/resource/setup-cost observations.
