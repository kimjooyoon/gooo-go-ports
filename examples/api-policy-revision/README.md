# Compare two sealed API policies in Gooo

Keep a saved API decision tied to its original policy, then evaluate exactly the
same nine facts with a candidate policy. This example compares two explicit
profiles:

- `cel-api-original-v1`: the existing CEL-derived API policy, frozen unchanged.
- `cel-api-readonly-v1`: original Gooo composition that preserves every base
  denial, then denies an otherwise allowed action other than `read` with
  `ACTION_NOT_READ_ONLY`.

For the supplied synthetic `write` facts, the original allows and the candidate
denies. Exactly `allowed` and `reason` differ: `NEWLY_DENIED`, changed count `2`.
The synthetic `read` facts remain allowed with `UNCHANGED`, changed count `0`.
Both complete fifteen-field decisions remain in the report.

This is historical consistency and a hypothetical policy comparison. Supplied
authentication, tenant and action facts are not verified against a live system.
There is no API dispatcher, current authorization grant or per-call human
approval step. Consistently fabricated unsigned facts and observations can be
internally consistent; a report does not attest that an event occurred.

## Native quickstart

Start in the repository root with **Go 1.27.2 already installed and selected**.
The module uses the local frozen original module and standard-library-only
native code. Consuming the generated profiles requires no Gooo compiler or CEL
evaluator. Toolchain installation and its initial cache setup are separate.

Create a new private directory, build once, and run each mode in a fresh process:

```sh
export GOTOOLCHAIN=local GOWORK=off GOENV=off GOPROXY=off GOSUMDB=off
umask 077
revision_run="$(mktemp -d)"
go -C examples/api-policy-revision build -trimpath \
  -o "$revision_run/compare" ./cmd/compare

for action in read write; do
  "$revision_run/compare" -mode baseline \
    -input "examples/api-policy-revision/testdata/$action-facts.json" \
    > "$revision_run/$action.request.json"
  "$revision_run/compare" -mode compare \
    -input "$revision_run/$action.request.json" \
    > "$revision_run/$action.comparison.json"
  env PATH="" "$revision_run/compare" -mode replay \
    -input "$revision_run/$action.comparison.json" \
    > "$revision_run/$action.replayed.json"
done
```

The command also accepts one JSON object on stdin when `-input` is omitted.
`baseline` creates a **new** original proof for supplied facts; it does not
repair a changed saved proof. `compare` first verifies that saved original proof,
then runs the candidate and both Gooo classifiers. `replay` recomputes and checks
the entire saved report, including both facts, all decisions, differences,
summary and complete bindings.

Keep the printed/private directory path for later replay. Files can contain raw
group, principal and tenant facts; inspect them before sharing. Validation errors
produce a nonzero exit and no JSON result. Writing stdout or a redirected file
is not an atomic file transaction.

### Read failures from an AI agent or automation

Pass `--diagnostics=json` before the other options to receive one versioned JSON
refusal on stderr. Successful programs keep their existing stdout and exit zero;
normal policy decisions such as `DENY` and `NEWLY_DENIED` are successful results.
Failures still exit one. The default `--diagnostics=text` preserves the existing
human-readable error message.

```sh
"$revision_run/compare" --diagnostics=json -mode replay \
  -input "$revision_run/write.comparison.json" \
  > "$revision_run/replayed.json" 2> "$revision_run/refusal.json"
```

The refusal schema is `gooo/api-policy-revision-diagnostic/v1`, with `command`,
`mode`, `status: "REFUSED"`, and one `diagnostics` entry containing `severity`,
`code`, `stage`, and `message`. Branch on the stable code and stage, not the
message. Codes identify invalid arguments/input, input/output I/O, input limits,
profile/generation/saved-decision mismatches, and a changed saved report.
Unclassified evaluation failures retain their known operation boundary.

Options follow Go's flag parsing rules: the last parsed diagnostics option wins.
An unknown option or a positional argument stops parsing, so a diagnostics option
after that point is not selected; an invalid format falls back to text. Nothing
after `--` is interpreted as an option.

Preserve the process exit, stdout, and stderr together. A failed output write
can leave partial stdout; failed diagnostic delivery can leave partial stderr.
Neither is a valid complete record. Diagnostics do not authorize a retry or an
API effect. This CLI remains an offline computation with no dispatcher.

## Import the published policy from another Go project

With Go 1.27.2 installed and selected, this separate project imports the public
module at immutable commit `e723f9af6dc33ecfff002a951e4b09a310439338`.
Initial dependency downloads use the public Go proxy and checksum service.
There is no consumer-owned `replace`, workspace or Gooo compiler requirement.

```sh
(
  set -eu
  export GOTOOLCHAIN=local GOWORK=off GOENV=off
  export GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org
  export GOPRIVATE= GONOPROXY= GONOSUMDB=
  consumer_dir="$(mktemp -d)"
  cd "$consumer_dir"
  go mod init example.local/gooo-api-review
  go get github.com/kimjooyoon/gooo-go-ports/examples/api-policy-revision@e723f9af6dc33ecfff002a951e4b09a310439338
  cat > main.go <<'GO'
package main

import (
    "fmt"
    revision "github.com/kimjooyoon/gooo-go-ports/examples/api-policy-revision"
    original "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy"
)

func main() {
    facts, err := original.DecodeAPI([]byte(`{"group_present":true,"group_value":"admin","principal_present":true,"principal_value":"synthetic:user","authenticated":true,"caller_tenant":"tenant-a","resource_tenant":"tenant-a","requested_action":"write","allowed_action":"write"}`))
    if err != nil { panic(err) }
    saved, err := revision.Baseline(facts)
    if err != nil { panic(err) }
    report, err := revision.Compare(saved)
    if err != nil { panic(err) }
    fmt.Println(report.Summary.Classification, report.Summary.ChangedCount)
}
GO
  go run .
)
```

Expected output: `NEWLY_DENIED 2`. Both `allowed` and `reason` change; all nine
facts and both complete decisions remain in the report. This synthetic
comparison does not authenticate the facts or authorize an API action.

The parent now requires the original module at the actual Go-resolved version
`v0.0.0-20261014100709-8cddcc4e65af`. Repository builds retain the local frozen
replacement; dependency replacements are not inherited by an external project.
Go module ZIPs exclude nested modules, so the public parent ZIP omits
`profiles/original`; Go resolves that original module separately. Build and
generator reproduction still use the full source checkout described above.
`go install .../cmd/compare@version` is not a supported installation path.

Changing this dependency declaration creates a new parent generation even
though the Gooo policies and original 28 assets are unchanged. Original API
proofs retain their original generation. A saved comparison from the earlier
parent generation must be replayed with its original source version; it is not
migrated or silently rebound to this module version.

## Exact input and evidence contracts

The facts object has exactly nine fields: `group_present`, `group_value`,
`principal_present`, `principal_value`, `authenticated`, `caller_tenant`,
`resource_tenant`, `requested_action`, and `allowed_action`. Presence and
authentication fields are Boolean; the other fields are Text. Every decoded
Text is valid UTF-8 and at most **65,536 bytes**, including inactive placeholders.
The adapter preserves those placeholders; it does not trim, normalize or fold
their case. Invalid UTF-8 and isolated UTF-16 surrogate escapes are refused.

Each CLI input is one object. Raw facts and comparison requests are bounded to
**4 MiB**; saved comparison reports and the `replay` input are bounded to
**16 MiB**. The larger report limit accommodates both proofs and JSON-escaped
copies of all valid raw Text values without truncating or normalizing them.
Each nested original proof retains its original **4 MiB** decoder limit.
Missing, unknown, duplicate (including escape-equivalent), `null`, wrongly typed
or trailing fields are refused throughout the nested transport. Difference
records require all fifteen Boolean fields. Summaries require a known
classification and an integer count from `0` through `15`.

`baseline` returns `before_profile`, `candidate_profile` and `before`. An API
proof contains `facts`, its fifteen-field `decision`, and complete `bindings`.
Comparison reports use schema `gooo/api-policy-revision-comparison/v1`, status
`COMPARED`, and scope `same_saved_facts_two_sealed_profiles`. They additionally
retain `after`, fifteen `differences`, `summary`, and `classifier_bindings`.
Neither an allow nor a successful replay grants permission to execute now.

The original profile has **28 unchanged sealed assets** and the original raw
generation manifest digest
`sha256:79b76bda704cb63b9390802766a66ac72ba02c04ae135600cb05de6797c97d46`.
The candidate and classifiers have their own **37-asset generation**, including
the exact native projections and adapter. Both complete rosters are checked.
The original `VerifyAPIProof` remains strict: it still rejects a candidate or
unavailable older generation. Exactly the two named profiles are available;
there is no arbitrary runtime loader, inferred legacy profile or schema migration.

## Where the semantics live

[`readonly.gooo`](readonly.gooo) imports the original API policy and owns the
candidate decision and denial priority. [`compare.gooo`](compare.gooo) owns all
fifteen field comparisons, classification and changed count. Its two scalar-record
outputs stay within the pinned compiler's record limits. Go validates exact
transport and bound generations, carries the facts, and checks complete evidence;
it has no policy or difference-calculation fallback.

The readonly example demonstrates `UNCHANGED` and `NEWLY_DENIED`. The separate
**23 synthetic classifier pairs** exercise all fields and the additional
`NEWLY_ALLOWED`, `REASON_CHANGED` and `OBSERVATIONS_CHANGED` branches. Those pairs
are classifier inputs, not additional readonly policy paths, external requests,
ported-function cases or evidence of independent adoption.

## Reproduce the finite validation

This developer check also needs Python 3, the installed race-test C toolchain,
and an existing compiler from source
`83928fa4538245abe33b08549da47f8cbdc50ed2`, version `0.6.32-dev`, runtime
`v0.2.41-experimental`, built with Go `1.27.2`. Other compiler/runtime identities
are not substitutes. From the repository root, pass the actual installed Go
executable and choose an output directory that does not already exist:

```sh
python3 examples/api-policy-revision/verify.py \
  --compiler "$PWD/.gooo" \
  --go "$(go env GOROOT)/bin/go" \
  --out "$PWD/out/api-policy-revision-validation-unique-name"
```

The verifier runs generation checking, `go vet`, and the entire race-enabled
module test suite once, including maximum-length escaped-fact roundtrip and
oversized-input refusal checks. It then builds a temporary standalone CLI and runs
baseline, comparison and replay as three fresh processes for each supplied fact
case. Those consumer processes use an absolute binary, empty executable search
path and fresh temporary working directories. It checks every raw fact, all
fifteen before/after observations, all flags, counts, classifications and both
binding rosters against independent literal expectations. A changed report must
be refused with exit `1` and empty stdout. It also confirms source files and the
original generation remained unchanged.

`verification.json` and exact process outputs are private evidence. The expected
successful status is `FINITE_TWO_PROFILE_NATIVE_AND_SAVED_REPLAY_PASS`; it is
written only after those checks finish. Timing fields describe that machine and
its existing setup/cache. They are not an independent first-user measurement or
a promise of a ten-minute installation. Empty `PATH` is an observed process
configuration, not system-call instrumentation proving zero network, compiler
or external-effect calls.

## Frozen source and attribution

[`profiles.json`](profiles.json) pins the original module copied from
[`gooo-go-ports` commit `8cddcc4`](https://github.com/kimjooyoon/gooo-go-ports/tree/8cddcc4e65afb42431f8f42a86f29859ae2f4d69/examples/cel-api-policy).
Its original module path, internal imports, 28 asset bytes and raw generation
manifest are preserved under [`profiles/original`](profiles/original).
Only this new parent module supplies a local `replace`. Original `go.mod` and
license/notice packaging files are bound separately rather than retroactively
added to an old proof's asset roster.

Google's bounded CEL codelab adaptation and the original Gooo API policy retain
their [source notice](profiles/original/NOTICE.md), [upstream license](profiles/original/oracle/upstream/LICENSE)
and [CEL runtime release license](profiles/original/LICENSE-APACHE-2.0).
The new readonly policy, classifiers and comparison transport are original Gooo
ecosystem research, with AI-assisted development. This example implies no
Google/CEL/AgentMesh endorsement and implements no live mesh session, token
verification, tool-definition continuity or real API effect.
