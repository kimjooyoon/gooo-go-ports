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

From the repository root, with Go 1.27.3 selected:

```sh
export GOTOOLCHAIN=go1.27.3
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
projections, transport, adapter and the saved-proof verifier/host example. Hash
consistency identifies the **same bound source generation** and its generation
producer. It does not identify a later host executable's binary or build
toolchain, authenticate an issuer, or establish that supplied facts occurred.
`generate.py --check` additionally compares mirrors with the canonical repository
sources.

## Save and verify a historical API decision in an ordinary Go host

The exported `EvaluateAPI(APIFacts)` returns an `APIProof` containing raw facts,
all fifteen observations and complete source bindings. `DecodeAPIProof` strictly
decodes a saved proof. `VerifyAPIProof(APIProof)` validates the current embedded
generation first, requires the exact compiler pin, generation-manifest digest
and complete asset roster, then re-evaluates the facts through the generated
Gooo policy and compares **all fifteen fields**. A changed decision returns
`ErrAPIDecisionMismatch`; a different bound source generation returns
`ErrAPIGenerationMismatch` before evaluating saved facts.

The result is `HISTORICALLY_CONSISTENT` with scope
`same_bound_source_generation`. A historical allow is an observation of saved
facts, **not current authorization**. Replay has no current-fact provider,
authentication provider, dispatcher or effect callback. Missing-field raw
placeholders are preserved, domain-checked and ignored exactly as in the policy;
changing an inactive placeholder may legitimately remain consistent. A caller
who changes both facts and observations consistently can construct another
accepted unsigned proof. This checks consistency, not factual history or issuer
trust.

[`cmd/host`](cmd/host/main.go) is a runnable Go embedding example. It evaluates a
synthetic allow/deny observation, exclusively creates a new `0600` proof in a
caller-selected private directory, then reloads it in another process. It refuses
existing targets, permissive files and a symlink at the proof or final parent.
The caller must own/trust the directory and its ancestors; this example does not
authenticate filesystem ownership. Its instrumented current-fact provider and
effect method remain at zero calls, and application state remains unchanged for
both saved allow and deny observations. Raw group/principal/tenant facts belong
in private storage, not a public API response or automatic upload.

From the repository root with Go 1.27.3 already selected:

```sh
umask 077
proof_dir="$(mktemp -d)"
go -C examples/cel-api-policy build -trimpath -o "$proof_dir/host" ./cmd/host
go -C examples/cel-api-policy build -trimpath -o "$proof_dir/evaluate" ./cmd/evaluate
"$proof_dir/host" -mode save -decision allow -proof "$proof_dir/allow.json"
"$proof_dir/host" -mode save -decision deny -proof "$proof_dir/deny.json"
env PATH="" "$proof_dir/host" -mode verify -proof "$proof_dir/allow.json"
env PATH="" "$proof_dir/host" -mode verify -proof "$proof_dir/deny.json"
"$proof_dir/evaluate" -mode original-api \
  < examples/cel-api-policy/api-demo.json > "$proof_dir/api-batch.json"
env PATH="" "$proof_dir/evaluate" -mode replay-api < "$proof_dir/api-batch.json"
```

Compilation/setup happens before replay. The prebuilt host's fresh reload needs
no Gooo compiler, CEL evaluator, dependency/toolchain downloads or network calls.
`replay-api` accepts a JSON array of 1..1024 complete proofs, at most 4 MiB. Every
nested envelope/fact/decision/binding field has an exact type and key; asset maps
have bounded string keys/values and must match the complete generation roster
before replay succeeds. Duplicate keys (including escaped duplicates), missing,
unknown, `null`, wrong types, trailing JSON, invalid UTF-8, isolated surrogates and
oversized text are refused. A malformed or inconsistent later member fails the
whole batch without partial result output.

This first version deliberately requires the same bound source generation.
Changes to bound runtime code change the generation even when policy outputs
match. Proofs from the preceding PR #73 source generation are explicitly
rejected. Archived-artifact replay, schema migration, signatures and current
authorization are separate features.

## Reproduce the independent checks

Setup may download the pinned compiler and pinned oracle dependencies. Actual
verification runs with automatic toolchain/dependency networking disabled:

```sh
export GOTOOLCHAIN=go1.27.3
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

The generator obtains the exact flattened Gooo source and lowered entry name
from **actual package seed execution**, verifies the
imported pure-call closure, saves its replay, and invokes body-codegen on those
compiler-owned bytes. Scalar/API projections use separate Go packages. The
canonical declared/export/body-codegen activity IDs are retained; the pinned
compiler's `pure_calls` metadata also reports namespace-derived IDs. Both actual
identity forms are retained in the complete receipts; they are not rewritten to
look identical. Fresh cross-platform generation compares all semantic/source/
producer fields, normalizing only recorded route latency and the generating
process's GOOS/GOARCH observation.

Workspace execution/replay independently checks all fields of 48 scalar and all
32 API tuples in deterministic batches of at most 16, each at most 32 KiB. This
is a consumer batching policy. The current compiler's aggregate input JSON limit
is 1 MiB (1,048,576 bytes), separately from 128 rows and the default 1,024 UTF-8
byte Text bound. The complete 48/32-case source suites fit that aggregate JSON
cap. Two size-refusal probes preserve their valid cases and add only trailing
whitespace to reach 1 MiB + 1 byte. Each must fail closed without a result and
report the exact current size limit. Four 65,536-byte scalar cases remain in the
full native comparison; their framework rejection receipts and a separate
1,025-byte Text probe are also recorded. These overlapping workspace and native
checks are **not** added together as unique cases or to the repository's existing
port corpus.

Checks also include fresh generation, race-enabled native tests, vet, strict JSON
and typed-domain refusal, each of 28 embedded-asset and 28 saved binding-digest
tamper cases, manifest tampering, and rehashed source/projection inconsistencies.
All 32 authored API cases are saved/strictly decoded/replayed and compared with
their independent fifteen-field records. Separate replay checks change every
decision field, all nine active raw facts and generation identities, exercise
the nested malformed schema and transactional batch refusal, preserve absent
placeholder behavior, reject the prior generation, and demonstrate consistent
synthetic forgery as a trust limit. Two private save/reload pairs execute the
actual prebuilt Go host in fresh processes with an empty executable search path.
These repeated observations do not add unique cases to the existing port corpus.

Original Google source/license pins and the independently executed oracle are in
[`oracle/`](oracle/README.md). Source attribution is in [NOTICE.md](NOTICE.md).
