# Atomic diagnostic header addition host

This bounded in-memory example asks the generated Gooo `AssessAddition` function whether an exact diagnostic header may be added. The owner re-reads current request, policy and authority, calls that function, and appends the header plus advances the revision while holding one lock. No per-request human approval is required. The exported advisory decision is never trusted by `Apply`.

The `.go.txt` files are templates, not an alternate Go policy implementation. After the complete differential suite finishes, run:

```sh
python3 -I -B tools/check-header-addition-host.py --root . --verification-out out/ci --timing out/ci/timing.jsonl --go "$(go env GOROOT)/bin/go" --out out/header-addition-host
```

The checker requires the current 74-suite source/timing bindings, reads all seven original execute receipts and saved replays, extracts identical generated source without a package rewrite, and makes the 50-case ABI fixture from fresh independent Go expected values. It records exact source and expectation provenance and freezes rendered inputs before checking the typed bridge, vet, race tests, build and demo. Generated policy source and the fixture are outputs, not checked-in authority.

The tests cover duplicate and concurrent application, stale request/policy/body observations, changed authority, forged advisory decisions and wrong candidates, copied input/output slices, case-insensitive header counting, preserved raw Content-Length/body bytes and signed64 exhaustion.

The embedding adapter must authenticate subjects and truthfully supply current observations. This example owns one local request in memory: it does not authenticate network clients, write to a remote API, persist transactions or provide distributed atomicity. A Gooo `ELIGIBLE` result is a pure bounded assessment; the owner still enforces its storage bounds and atomic application.

Temporary caches and binaries are unique to this check and removed only after observed owned processes close. Rendered source, frozen fixture, evidence references and logs remain available for review. Process observation polls ancestry and unique owned paths; it is not a kernel fork trace.
