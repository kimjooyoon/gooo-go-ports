# CI timing and suite order

The full native verifier supports two dispatch orders. The default
`--suite-order catalog` uses the original suite order.
`--suite-order largest-corpus-first` dispatches larger case corpora first,
breaking equal case counts by their original catalog index. It uses only the
current authored corpus lengths; it does not download previous timing data or
predict an engine execution time.

CI explicitly selects the latter with three workers:

```sh
go run -mod=readonly ./cmd/verify --workers 3 \
  --case-profile uuid-and-literal-report-compact-v1 \
  --workspace-profile dependency-closure-v1 \
  --suite-order largest-corpus-first \
  --out out/ci --timing-out out/ci/timing.jsonl
```

The required population remains 82 suites, 175,649 cases and 2,305 batches under
these profiles, using the pinned `83928fa4538245abe33b08549da47f8cbdc50ed2`
compiler and Go 1.27.3. The selector changes dispatch order only. It preserves
each suite's source workspace, inputs, expectations and batch boundaries;
execution, inspection, saved replay, receipt binding and exhaustive checks remain
required. Results and suite indices remain in catalog order. All 71 workflow
step declarations, scope conditions and required artifact uploads remain in
place.

## Failure order

The returned suite failure follows the original catalog order. If a large suite
at a later catalog index fails first in elapsed time, pending lower indices must
still run. If one of those fails, its lower index takes precedence. Work above
the lowest known failed index is skipped if it has not started. Active
higher-index work receives cancellation and is joined before return. This preserves
the serial verifier's earliest catalog failure even when dispatch order changes;
whole-run cancellation still takes precedence. The selector
does not promise the same completion order or the same partial diagnostics
as a serial run. Every started worker must finish before owned cache cleanup.

## Why test another order

The original completed [main158 run 39103357232, attempt 1](https://github.com/kimjooyoon/gooo-go-ports/actions/runs/39103357232/attempts/1)
used the same 82 suites, 175,649 cases, 2,305 batches and three workers in catalog
order. Its full comparison workflow step took about 639 seconds, while the
native timing recorder observed 628.846743092 seconds. The recorder and the
workflow step cover different intervals.

The retained timing records show a long final suite: literal-text `Report`
started at about 509.61 seconds, ran for 119.19 seconds, and finished about
29.72 seconds after the next-to-last suite. Execute plus replay accounted for
97.1% of cumulative batch worker time. Worker intervals overlap, so cumulative
suite or phase durations must not be treated as additive wall-clock shares.

A fixed-duration calculation replayed those observed suite durations on three
workers. It modeled catalog dispatch at 628.57 seconds and descending case-count
dispatch at 608.03 seconds, a difference of 20.54 seconds. This is arithmetic
over one completed run, not a measured execution of the new order. The model
holds every suite duration fixed and excludes changed CPU contention, cache
effects, dispatch overhead and failure behavior. Case count is a scheduling
heuristic, not an execution-cost estimate or a promised 20-second saving.

## Acceptance and measurement limits

The new order needs tests for deterministic case-count ordering, ties, catalog
result order and the earliest-failure behavior above. Hosted CI must still
complete the full original population with fresh native execution and replay;
a different dispatch list or a successful fixed-duration calculation is not
native acceptance.

The summary and timing header identify `suite_order` and
`planned_dispatch_indices`. The array is a planned sequence of original suite
indices, not a record of actual start times; concurrent workers may start or
finish in another order. CI derives the expected plan from the catalog-ordered
summary rows using descending case count and ascending original index, then
requires the timing header to carry the same plan. The baseline's timing
records were obtained from bounded ranges of its official native artifact, with
the selected member's filename, offsets, raw size and CRC checked. The entire
native ZIP digest and names outside the read central-directory suffix were not
verified. Its separate small observer-events ZIP matched the official digest.
These observations support the experiment design; they do not establish a
speedup, arbitrary-input equivalence or a change to the compiler's guarantees.
