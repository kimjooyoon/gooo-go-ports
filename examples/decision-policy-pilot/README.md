# Offline decision-policy pilot

A small synthetic example of a Gooo policy between a Choice observation and a
mock action. It uses no provider API, API key, paid inference, customer data, or
real data export. Authentication and trusted context are supplied by fixtures.

This is a separate example with its own Go module. Its 49 fixtures are **not**
added to the repository's ported-function count or native port comparison corpus.
The policy is an original demonstration, not a port of a provider SDK.

## Run

From the repository root, with Git and Go 1.27.2 installed on `PATH`:

```sh
./tools/bootstrap.sh
./examples/decision-policy-pilot/run.sh
```

Bootstrap fetches and builds the repository's pinned compiler. The pilot then
uses that existing `../../.gooo` binary read-only, relative to its own directory.
It checks revision `83928fa4538245abe33b08549da47f8cbdc50ed2`, version
`0.6.32-dev`, clean VCS state, and Go `1.27.2` against actual build information.

If your `go` launcher uses automatic toolchain selection, select the toolchain
for setup and pass its installed executable to the offline pilot:

```sh
export GOTOOLCHAIN=go1.27.2
./tools/bootstrap.sh
PILOT_GO_BIN="$(go env GOROOT)/bin/go" ./examples/decision-policy-pilot/run.sh
```

Setup may download the toolchain and compiler dependencies. The pilot itself
sets `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, `GOENV=off`, and
`GOWORK=off`; it does not download dependencies or call a provider. You may set
`PILOT_GO_BIN` to another installed Go 1.27.2 executable or pass `--compiler` with
another build of the exact pinned compiler. Other versions are rejected.

Each run creates a new `examples/decision-policy-pilot/out/run-*` directory and
prints its exact path. Cache and temporary files stay inside this example's
ignored `.cache/` and `.tmp/` directories. Expected output:

```text
PASS: 31 Gooo/Go/manual matches, 18 input rejections, 7 mock calls, replay/error dispatches 0
```

To check a saved directory, use the run name printed above. The path is relative
to this example, because `run.sh` changes to its own directory:

```sh
./examples/decision-policy-pilot/run.sh --verify out/run-YOUR_RUN_NUMBER
```

Use the same `PILOT_GO_BIN` override if required. Verification reads saved
receipts, hashes, original inputs, manual expectations and Go reference values.
It starts no new Gooo execution, provider call, or mock dispatch. Building the
Go verifier itself is separate from executing the policy.

## Input and policy

[fixtures.json](fixtures.json) contains 49 synthetic cases with manually specified
expected results. Each input envelope requires three objects:

| Object | Fields |
| --- | --- |
| request | `action`, `tenant`, `item_count` |
| context | `authenticated`, `tenant`, `allowed_action`, `item_limit`, `now`, `expires_at` |
| decision | `status`, `choice` |

This is a **provider-neutral Choice envelope**, with locally defined `allow`,
`review`, and `deny` labels. It is not the TypeSafe wire API, a probability
validator, or a Score/Noul adapter. No real provider responses are used.

[policy.gooo](policy.gooo) applies these rules in order; the first applicable rule
supplies the reason:

1. Deny when `authenticated=false`.
2. Deny when either tenant is empty or the tenants differ.
3. Deny unless the action is `export_records` and matches `allowed_action`.
4. Deny when count or limit is not positive, or count exceeds the limit.
5. Deny when `expires_at <= now`, including the exact expiry boundary.
6. Review when status is anything other than `observed`.
7. Deny unknown choices; otherwise deny `deny`, review `review`, or allow `allow`.

The result has `allowed`, `disposition`, and `reason`. The Go host first checks
required fields, exact JSON types, duplicate/extra keys, null values and integer
bounds. Inputs are limited to 16 KiB and individual text values to 1024 UTF-8
bytes. Arrays and additional nested fields are not accepted. Signed int64 values
remain exact, including values beyond 2^53; no float conversion is used.
Invalid UTF-8 and isolated UTF-16 surrogate escapes are rejected before decoding,
so replacement-character decoding cannot merge distinct tenant inputs. There is
no Unicode normalization or case folding.

The 31 well-typed cases cover each policy rule, failure precedence, exact limits,
int64 endpoints, values beyond 2^53, expiry boundaries and Unicode tenants. The
18 input rejections cover missing/null fields, wrong types, int64 overflow,
extra/duplicate keys, oversized text and trailing JSON. Host rejection is recorded
as `reject/invalid_input`, separately from a policy decision.

## Evidence and mock dispatch

The independent Go reference in [contract.go](contract.go) executes no Gooo or
generated code. It first checks each fixture's manual expectation. Those values
then become Gooo execution cases. The runner executes Gooo and replays the saved
execution against the same source and input snapshot.

The verifier checks original input values, ordering, types and field IDs; entry
identity and typed signature; source, manifest and case bindings; generated code
hashes; compiler identity; actual and expected results; and the saved replay's
exact predecessor. It also requires explicit zero model-call observations.

Only the **verified actual Gooo result** reaches the mock spy. The spy increments
an in-memory counter and performs no export. Each of the seven allowed cases
calls it exactly once; denied, reviewed and invalid inputs call it zero times.
A separate actual compiler failure probe omits a required context field and
confirms zero dispatch. The 31 Gooo/Go comparisons, 18 host rejections and one
compiler failure probe remain separate counts.

Saved replay never dispatches. It checks a historical decision using the frozen
context and time; a saved allow does not grant permission to execute now.

The output directory retains source/fixture copies, exact commands, compiler
identity, cases, execute/replay receipts, rejection evidence, mock observations,
`summary.json`, and `checksums.json`. Checksums establish local byte consistency,
not signatures or independent attestation. The mock counter is a local process
observation, not evidence of an external system's behavior.

## Tests

Run from this directory with an installed Go 1.27.2 executable. Use the actual
successful output directory for `PILOT_EVIDENCE` so receipt-tampering tests run:

```sh
export GOTOOLCHAIN=local GOWORK=off GOENV=off GOPROXY=off GOSUMDB=off
mkdir -p .tmp .cache/go-build
export TMPDIR="$PWD/.tmp" GOCACHE="$PWD/.cache/go-build"
go vet ./...
PILOT_EVIDENCE="$PWD/out/run-YOUR_RUN_NUMBER" go test -race ./...
```

If Go 1.27.2 is not on `PATH`, replace `go` above with `"$PILOT_GO_BIN"`. Tests
cover surrogate handling, inconsistent dispatch results, missing/null model-call
counts and changes to input values/order, field identities, outputs, sources,
compiler identity, generated code, cases and replay origin. The receipt tests
explicitly skip when `PILOT_EVIDENCE` is absent.

## Limits and existing assets

Authentication, tenant ownership, limits, current time and expiry are **synthetic
fixture assertions**. Real authentication, trusted-context collection, permission
services, clock trust, freshness, idempotency, TOCTOU protection, transactions and
external effects are unimplemented. This does not establish provider accuracy,
prompt-injection resistance, prevention of every error, correctness over every
input, customer demand, or commercial viability.

**Performance is not measured.** The current CLI builds code and executes it
twice per call. Internal receipt timings are not a performance comparison or
service latency measurement. There is no zero-latency or production-readiness
claim.

Existing public projects [gooo-jev](https://github.com/kimjooyoon/gooo-jev),
[jev-gooo](https://github.com/kimjooyoon/jev-gooo),
[gooo-jev-runtime](https://github.com/kimjooyoon/gooo-jev-runtime), and
[gooo-jev-lab](https://github.com/kimjooyoon/gooo-jev-lab) are possible future
integration inputs. This example imports none of their APIs and claims no
compatibility with the current TypeSafe wire API. The reviewed gooo-jev decision
receipt is explicitly non-authorizing; it must not be treated as execution
permission or an interchangeable current Score/Noul response. Any reuse requires
checking the exact revision and input/output semantics separately.

## Focused workflow feedback

This is a gooo-go-ports ecosystem experiment developed with AI assistance. We are
looking for a concrete workflow to compare with direct Go code or existing tools;
we have not demonstrated customer adoption or paid outcomes.

If you recently checked a model's decision against business rules or manually
reconstructed why a decision was made, [open a feedback issue](https://github.com/kimjooyoon/gooo-go-ports/issues/new).
Describe the task, the manual check or replay step, and your current Go or other
tool workaround. What would need to improve for an additional policy layer to be
useful? Share only non-sensitive information that you can make public. A specific
example of existing work is more useful than a general language feature request.
