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

Use the single-request workflow below to try your own versions. To extend the
independent fixture checks, edit matching values in `minimum-inputs.json` and
`minimum-reference-inputs.json`.
Each input is decoded valid UTF-8 Text of at most 1,024 bytes, without trimming.
Shorthand `v1` and `v1.1` follows the pinned Go reference.

Build metadata does not change precedence: `v1.0.0+001` to `v1.0.0+002` has
`raw_changed=true` but is not newer. A higher-major prerelease can be eligible:
`v1.9.0` to `v2.0.0-rc.1` meets `v2.0.0-rc.0`. This rule does not enforce a
stable release channel or major-version compatibility. The supplied minimum is
data; this example does not fetch, authorize or perform a deployment.

## Try your own current, candidate and minimum

Create one `request.json`; the eight-case fixtures and Go reference inputs do not
need editing:

```json
{"current":"v1.0.0","candidate":"v2.0.0","minimum":"v3.0.0"}
```

From the repository root, convert that request, execute Gooo, save a fresh replay
and display its thirteen facts. The new temporary directory preserves existing
files; `set -e` stops before execution if conversion fails.

```sh
set -e
version_request_dir="$(mktemp -d)"
python3 examples/version-eligibility/request-inputs.py < request.json > "$version_request_dir/inputs.json"
./.gooo package execute --json --inputs "$version_request_dir/inputs.json" version-minimum.workspace.json > "$version_request_dir/execution.json"
./.gooo package replay --json --receipt "$version_request_dir/execution.json" --inputs "$version_request_dir/inputs.json" version-minimum.workspace.json > "$version_request_dir/replay.json"
python3 -c 'import json, sys; print(json.dumps(json.load(sys.stdin)["result"]["runtime"]["traces"][0]["deliveries"][0]["actual"], ensure_ascii=False, indent=2))' < "$version_request_dir/replay.json"
```

This request produces `eligible=false` with `reason="below_minimum"`: the candidate
is newer than current, but below the supplied minimum. Python displays the result
object and exits successfully even when its `eligible` field is false. It does
not validate the receipt or grant authority. Input-only Gooo receipts remain
`OBSERVED`; the finite oracle test profiles are separate.

The converter accepts exactly three strings, each at most 1024 UTF-8 bytes, in a
JSON document of at most 32 KiB. It rejects duplicate decoded keys, missing or
unknown keys, non-string values, malformed/trailing JSON, invalid UTF-8 and
unpaired surrogate escapes. An input validation failure prints a diagnostic to
stderr, returns exit 1 and emits no inputs. Incorrect command arguments return exit 2.
Omitting `--policy` selects `minimum`; `--policy minimum` is equivalent. Only
`minimum` and `stable` are accepted by this single-request converter.
Invalid version syntax, empty strings and surrounding whitespace are preserved
as Text so Gooo can return its own validity and reason fields. The converter
does not call Gooo, import the Go oracle, compute eligibility or apply an upgrade.

## Keep stable candidates within the current major

`AssessStableSameMajorUpdate` applies a fixed filter after the minimum-version
rule: the candidate must also have the same major as current and no prerelease
suffix. For example:

```json
{"current":"v1.9.0","candidate":"v2.0.0-rc.1","minimum":"v1.0.0"}
```

Save this as `request.json`, then select the stable entry. Conversion still
validates only the input format; Gooo computes every version and policy fact.

```sh
set -e
stable_request_dir="$(mktemp -d)"
python3 examples/version-eligibility/request-inputs.py --policy stable < request.json > "$stable_request_dir/inputs.json"
./.gooo package execute --json --inputs "$stable_request_dir/inputs.json" version-stable-same-major.workspace.json > "$stable_request_dir/execution.json"
./.gooo package replay --json --receipt "$stable_request_dir/execution.json" --inputs "$stable_request_dir/inputs.json" version-stable-same-major.workspace.json > "$stable_request_dir/replay.json"
python3 -c 'import json, sys; print(json.dumps(json.load(sys.stdin)["result"]["runtime"]["traces"][0]["deliveries"][0]["actual"], ensure_ascii=False, indent=2))' < "$stable_request_dir/replay.json"
```

Consume **`policy_eligible` with `policy_reason`** for this filter. The example
has baseline `eligible=true` and `reason="eligible"`, but `same_major=false`,
`candidate_stable=false`, `policy_eligible=false`, and
`policy_reason="different_major"`. The original `eligible` and `reason` retain
their minimum-only meanings. Policy refusal order is the original baseline
reason first, then `different_major`, then `prerelease`, otherwise `eligible`.

The new result has 16 fields: the baseline's first 12 fields (excluding
`raw_changed`), followed by `same_major`, `candidate_stable`, `policy_eligible`
and `policy_reason`. `AssessUpdate` continues to return its original 13 fields.
The stable profile's 12 supplied cases have been locally executed against the
original Go reference and freshly replayed. Use `--policy stable` with the
finite checker below; this is finite evidence, not all-input correctness.

Here, stable means a valid candidate without a prerelease suffix. Build metadata
does not make it a prerelease. `v0.1.0` to `v0.2.0` passes this same-major filter;
that does not establish API compatibility, security, or deployment safety.
The filter selects candidate facts and performs no merge or deployment.
Input-only receipts remain `OBSERVED`, and the display command does not validate
them or grant authority.

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
for policy in upgrade minimum compare-boundary stable major prerelease; do
  python3 examples/version-eligibility/verify.py --policy "$policy" \
    --module-cache "$(GOENV=off go env GOMODCACHE)" --out "out/version-$policy"
done
```

| Profile | Entry | Cases | Output |
| --- | --- | --- | --- |
| `upgrade` (default) | `AssessUpgrade` | 8 current/candidate pairs | 9 facts |
| `minimum` | `AssessUpdate` | 8 current/candidate/minimum triples | 13 facts |
| `compare-boundary` | port `Compare` | 9 long version pairs | Integer order |
| `stable` | `AssessStableSameMajorUpdate` | 12 current/candidate/minimum triples | 16 facts |
| `major` | port `Major` | 9 version strings | Text major |
| `prerelease` | port `Prerelease` | The same 9 version strings | Text prerelease |

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

The extraction profiles share nine inputs, including invalid syntax, shorthand,
Unicode, and 1,024-byte major, prerelease and build metadata. Unary Gooo roots
use the entry name as their input key; multi-input policies use `.input0`,
`.input1` and `.input2`. The checker constructs these keys.

These are 55 function/policy cases across six profiles; replay does not add
distinct cases. Local native execution and replay matched the original Go reference for
the supplied cases. This is finite evidence, not full-package or full-domain
compatibility. Source identities are in
`result.program.workspace_image.packages[].sources[].source_digest`, and
`replayed_from_sha256` identifies the original receipt bytes.

Original source, tests and the BSD license are preserved in
[`third_party/x-mod`](../../third_party/x-mod). [Provenance](../../ports/xmod/provenance.json)
records the immutable revision and hashes. For a first failure, share a public
synthetic input, command, compiler version and the first diagnostic or mismatch.
