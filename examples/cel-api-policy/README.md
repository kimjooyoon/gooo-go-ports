# A bounded CEL rule composed with an original Gooo API policy

This example imports `cel.ScalarAccess` from
[`ports/cel/scalar_access.gooo`](../../ports/cel/scalar_access.gooo). It adapts one
Google CEL codelab expression over a typed protobuf request:

```cel
request.auth.claims.group == 'admin' ||
  request.auth.principal == 'user:me@acme.co'
```

The Gooo port preserves the distinction that makes this example useful: a
missing `claims.group` map key errors unless the principal branch is true; an
omitted protobuf string `principal` defaults to empty. The supplied raw value of
an absent field is ignored. Eight returned observations include the decision,
error, both comparisons, group absence, original principal-presence metadata,
and effective principal. The latter presence metadata is an adapter contract,
not a CEL/protobuf field-presence observation.

[`main.gooo`](main.gooo) imports that port and adds an original API policy. Gooo
checks a supplied authentication fact, nonempty and equal caller/resource tenants,
and nonempty and equal requested/allowed actions. It returns all fifteen decision
observations and these reasons in priority order:
`NOT_AUTHENTICATED`, `TENANT_UNSPECIFIED`, `TENANT_MISMATCH`,
`ACTION_UNSPECIFIED`, `ACTION_MISMATCH`, `ACCESS_ERROR`, `ACCESS_DENIED`, `ALLOW`.
The Go adapter only validates/transports raw facts and serializes generated results;
it contains no fallback policy calculation.

This is one bounded rule, **not a CEL interpreter or an exported upstream API**.
Only absent/string group and present/string or omitted/default principal values
are represented; unknowns, non-string groups and arbitrary missing intermediate
protobuf objects are outside the domain. Authentication and trusted context are
supplied synthetic server facts. This example does not authenticate a token,
perform a real API action, or prove whole-domain or production safety. A host
must establish trust/current state and apply external effects atomically. These
pure decisions require no approval for each call.

## Run the already-generated native CLI

From the repository root, with Go 1.27.2 selected:

```sh
export GOTOOLCHAIN=go1.27.2
export GOWORK=off
go -C examples/cel-api-policy run ./cmd/evaluate -mode scalar \
  < examples/cel-api-policy/scalar-demo.json
go -C examples/cel-api-policy run ./cmd/evaluate -mode original-api \
  < examples/cel-api-policy/api-demo.json
```

The CLI accepts a JSON array of 1..1024 exact raw fact objects, at most 4 MiB in
total. Boolean and string fields are mandatory; extra/duplicate keys, `null`,
wrong types, trailing JSON, malformed UTF-8 and unpaired surrogate escapes are
rejected before any batch result is emitted. Each decoded text is at most 65,536
UTF-8 bytes, including ignored raw placeholders. Typed entrypoints enforce the
same text bound. This domain belongs to the native adapter; the compiler's
workspace input transport has separate limits described below.

The returned hash bindings cover the canonical source/workspace mirrors, actual
compiler-owned lowering and seed execution/replay, body-codegen reports,
projections, transport and adapter. Hash consistency does not authenticate an
issuer or sign an executable. `generate.py --check` additionally compares the
mirrors with the canonical repository sources.

## Reproduce the independent checks

Setup may download the pinned compiler and pinned oracle dependencies. Actual
verification runs with automatic toolchain/dependency networking disabled:

```sh
export GOTOOLCHAIN=go1.27.2
export GOWORK=off
./tools/bootstrap.sh
go -C examples/cel-api-policy/oracle mod download
go_tool="$(go env GOROOT)/bin/go"
python3 examples/cel-api-policy/verify.py \
  --compiler "$PWD/.gooo" --go "$go_tool" \
  --out "$PWD/out/cel-api-policy"
```

The output directory contains `verification.json` and the actual current process
outputs. It is private by default and may include local paths/input facts; review
it before sharing. The verifier freshly evaluates all 52 retained scalar cases
using real `cel.dev/cel-go@v0.32.0`, compares every one of eight output fields with
native Gooo, and compares all fifteen outputs for 32 independently authored
original API cases. Archived observations remain unchanged; current executions
are written to the output directory.

The pinned compiler `83928fa4538245abe33b08549da47f8cbdc50ed2` has no `package
inspect` command. The generator therefore obtains the exact flattened Gooo source
and lowered entry name from **actual package seed execution**, verifies the
imported pure-call closure, saves its replay, and invokes body-codegen on those
compiler-owned bytes. Scalar/API projections use separate Go packages. The
canonical declared/export/body-codegen activity IDs are retained; the pinned
compiler's `pure_calls` metadata also reports namespace-derived IDs. Both actual
identity forms are retained in the complete receipts; they are not rewritten to
look identical. Fresh cross-platform generation compares all semantic/source/
producer fields, normalizing only recorded route latency and the generating
process's GOOS/GOARCH observation.

Workspace execution/replay independently checks all fields of 48 scalar and all
32 API tuples in deterministic batches of at most 16. Its text input limit is
1,024 UTF-8 bytes and its JSON transport limit is 32 KiB. Four 65,536-byte scalar
cases remain in the full native comparison; their actual framework rejection
receipts and a separate 1,025-byte text probe are recorded. The larger complete
pretty-printed workspace suites also produce actual size-limit probes before
batching. These overlapping workspace and native checks are **not** added
together as unique cases or to the repository's existing port corpus.

Checks also include fresh generation, race-enabled native tests, vet, strict JSON
and typed-domain refusal, each of 25 bound-asset tamper cases, manifest tampering,
and rehashed source/projection inconsistencies.

Original Google source/license pins and the independently executed oracle are in
[`oracle/`](oracle/README.md). Source attribution is in [NOTICE.md](NOTICE.md).
