# Explain a version upgrade decision

Give Gooo a current version, a candidate and a minimum requirement. It returns
eligibility, an explicit reason, and the facts behind that decision. For example,
`v2.0.0` is newer than `v1.0.0`, but a minimum of `v3.0.0` returns
`eligible=false` and `reason="below_minimum"`.

The parser and comparisons run in [Gooo](../../ports/xmod/README.md), imported
from a separate package. The Go implementation is an independent test reference.
Evaluation is automatic; no per-evaluation human approval step is added.

## Run the minimum-version rule

Use the repository's [pinned compiler and Go 1.27.2 setup](../../docs/ai-agents.md).
From the repository root, run the supplied eight inputs and save a replay:

```sh
./.gooo package execute --json --inputs examples/version-eligibility/minimum-inputs.json version-minimum.workspace.json > minimum-eligibility.json
./.gooo package replay --json --receipt minimum-eligibility.json --inputs examples/version-eligibility/minimum-inputs.json version-minimum.workspace.json > minimum-replay.json
```

Read `result.runtime.traces[].deliveries[].actual`. `AssessUpdate` returns:

- The original `current`, `candidate`, `minimum`, and their three validity facts.
- `candidate_vs_current`, `candidate_vs_minimum`, `precedence_newer`, and `meets_minimum`.
- `eligible`, `reason`, and `raw_changed`.

Eligibility requires all three versions valid, candidate precedence greater than
current, and candidate precedence at least minimum. The first applicable reason
wins: `invalid_current`, `invalid_candidate`, `invalid_minimum`, `not_newer`,
`below_minimum`, then `eligible`. Consume `eligible` with `reason`: an invalid
minimum sorts below a valid candidate, so `meets_minimum` alone is insufficient.

Edit matching values in `minimum-inputs.json` and
`minimum-reference-inputs.json` to try another case and check it independently.
Each input is decoded valid UTF-8 Text of at most 1,024 bytes, without trimming.
Shorthand `v1` and `v1.1` follows the pinned Go reference.

Build metadata does not change precedence: `v1.0.0+001` to `v1.0.0+002` has
`raw_changed=true` but is not newer. A higher-major prerelease can be eligible:
`v1.9.0` to `v2.0.0-rc.1` meets `v2.0.0-rc.0`. This rule does not enforce a
stable release channel or major-version compatibility. The supplied minimum is
data; this example does not fetch, authorize or perform a deployment.

## Compare without a minimum

`AssessUpgrade` preserves the same ordering and validity behavior but accepts
two inputs and returns nine facts. Eligibility requires both versions valid and
candidate precedence greater than current.

```sh
./.gooo package execute --json --inputs examples/version-eligibility/inputs.json version-eligibility.workspace.json > eligibility.json
./.gooo package replay --json --receipt eligibility.json --inputs examples/version-eligibility/inputs.json version-eligibility.workspace.json > replay.json
```

Update the corresponding values in `reference-inputs.json` when editing these
inputs. Both input-only workflows return `OBSERVED`. The finite checks below
instead compare actual outputs against independently generated expected values.

## Verify against the original Go package

Authenticate `golang.org/x/mod` v0.29.0 once through the official Go proxy and
checksum database. The nested oracle module pins its checksums in `go.sum`:

```sh
(cd examples/version-eligibility/oracle && GOTOOLCHAIN=local GOWORK=off GOENV=off GOPRIVATE= GONOSUMDB= GONOPROXY= GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org go mod download golang.org/x/mod@v0.29.0)
for policy in upgrade minimum compare-boundary; do
  python3 examples/version-eligibility/verify.py --policy "$policy" \
    --module-cache "$(GOENV=off go env GOMODCACHE)" --out "out/version-$policy"
done
```

| Profile | Entry | Cases | Output |
| --- | --- | --- | --- |
| `upgrade` (default) | `AssessUpgrade` | 8 current/candidate pairs | 9 facts |
| `minimum` | `AssessUpdate` | 8 current/candidate/minimum triples | 13 facts |
| `compare-boundary` | port `Compare` | 9 long version pairs | Integer order |

The checker builds the actual Go oracle, verifies its dependency build metadata
and checksums, and constructs expected cases from its output. Each profile must
produce finite `PASS` results and a fresh saved replay. It checks typed inputs,
outputs, source identities and the original receipt link. An existing output
directory is refused; raw results and diagnostics are retained, while private
work and caches are removed. Go module access uses offline settings after setup.

The boundary profile compares `candidate` as the left operand and `current` as
the right operand from `boundary-reference-inputs.json`. It covers the 509th
prerelease identifier, 508 versus 509 identifiers, numeric versus nonnumeric
identifiers, 1,024-byte equality and build metadata, with unequal pairs checked
in both directions. It protects the port's justified 512-identifier loop bound;
the byte scans still allow 1,024 bytes. See the [port contract](../../ports/xmod/README.md).

These are 25 cases across three entries/profiles; replay does not add distinct
cases. Local native execution and replay matched the original Go reference for
the supplied cases. This is finite evidence, not full-package or full-domain
compatibility. Source identities are in
`result.program.workspace_image.packages[].sources[].source_digest`, and
`replayed_from_sha256` identifies the original receipt bytes.

Original source, tests and the BSD license are preserved in
[`third_party/x-mod`](../../third_party/x-mod). [Provenance](../../ports/xmod/provenance.json)
records the immutable revision and hashes. For a first failure, share a public
synthetic input, command, compiler version and the first diagnostic or mismatch.
