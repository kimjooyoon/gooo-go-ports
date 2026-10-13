# Explain whether a service-grant change admits new requests

When an automation changes a grant, testing one familiar request can miss newly
admitted requests. For example, both `mcp://*.x` and `mcp://a.*` accept
`mcp://a.x`. The candidate also accepts `mcp://a.y`, which the old pattern denies.

`AnalyzeServiceGrant` in [refinement.gooo](refinement.gooo) analyzes the two
patterns and returns that concrete witness. Gooo parses the families, selects a
justified subset rule, builds candidate requests and calls the actual imported
[`MatchServicePattern`](../../ports/agentmesh/service_pattern.gooo) for the old
and candidate patterns. The default `AnalyzeServiceGrantChecked` entry validates
both raw patterns in Gooo before calling that core. Go preserves transport and carries the
result; the production host contains no replacement inclusion or matching
algorithm. No human approval step, model call or effect dispatch is involved.

## Try the native API

Build the CLI using the [example instructions](README.md), then run:

```sh
printf '%s\n' '[{"before":"mcp://*.x","after":"mcp://a.*"}]' |
  ./out/evaluate -mode refinement
```

The proof includes both raw patterns and complete source bindings. Its decision
for this input is:

```json
{
  "status": "WIDENING_WITNESS",
  "rule_id": "CHECKED_CANDIDATE_REQUEST",
  "supported_subset": false,
  "has_witness": true,
  "witness_type": "mcp",
  "witness_name": "a.y",
  "before_match": false,
  "after_match": true
}
```

The ordinary Go API is
`EvaluateRefinement(RefinementFacts{Before: oldPattern, After: candidatePattern})`.
It executes generated Gooo code without a compiler or network call at runtime.
To reproduce execution with Gooo itself, the root workspace is
[`service-grant-refinement.workspace.json`](../../service-grant-refinement.workspace.json).
That default now returns ten fields, adding `domain_status` and `invalid_field`
to the eight semantic fields. Require `domain_status=VALID_INPUT` before using
them. The native `EvaluateRefinement` API retains its eight-field decision and
returns an error for invalid input; `EvaluateRefinementChecked` exposes the
explicit domain result. See the [direct workspace migration](README.md#direct-gooo-workspace-migration).
The generator and verifier produce fresh native, workspace and saved-replay evidence
with the pinned compiler.

## Outcomes and input contract

| Status | Meaning | Other fields |
| --- | --- | --- |
| `SUPPORTED_SUBSET` | A recognized rule justifies that every candidate request is also allowed by the old pattern in the represented domain | `supported_subset=true`, named rule, `has_witness=false` |
| `WIDENING_WITNESS` | The returned request was actually checked: old denies it and candidate allows it | `supported_subset=false`, `has_witness=true`, rule `CHECKED_CANDIDATE_REQUEST`, both matcher observations |
| `UNPROVEN` | No recognized rule or checked witness supports a decision | `supported_subset=false`, empty rule, `has_witness=false` |

When `has_witness=false`, both witness strings are empty and both match fields
are false sentinels. They are not observations about an empty request. Unknown,
invalid and witnessed-widening results cannot support automatic eligibility.
The host rejects malformed or contradictory decision shapes as errors.

The exact grammar and independent 128-byte caps are in [README.md](README.md#exact-represented-domain).
Each full pattern, request type and request name has its own limit. There is no
128-byte cap on a concatenated `type://name` request. Comparisons are ASCII and
case-sensitive; custom schemes are allowed. The two input fields must both be
validated, even when one would select an empty/global shortcut. An emitted
witness must also satisfy the request grammar. Duplicate, missing, non-string
and extra fields, including caller-supplied decisions, are errors.

The checked Gooo entry and default workspace enforce this grammar themselves.
An invalid pattern returns `domain_status=INVALID_INPUT`, `status=INVALID_INPUT`,
and the first invalid field (`before`, then `after`); all other fields are empty
or false sentinels. It does not return a subset theorem, widening witness or
`UNPROVEN`. The low-level `AnalyzeServiceGrant` and explicit
`service-grant-refinement-raw.workspace.json` retain the caller precondition.
The analyzer is not a general URI parser, and its conclusions do not extend to
out-of-domain Text passed directly to that raw compatibility entry.

## Six-family inclusion matrix

Rows are old patterns, columns are candidates. `yes` means inclusion always
holds, `widen` means it does not, and a condition means inclusion holds exactly
when that condition is true. A false condition has a widening witness.

| Old / candidate | Empty | Global `*` | Typed-all | Exact | Prefix | Suffix |
| --- | --- | --- | --- | --- | --- | --- |
| Empty | yes | widen | widen | widen | widen | widen |
| Global `*` | yes | yes | yes | yes | yes | yes |
| Typed-all | yes | widen | same type | same type | same type | same type |
| Exact | yes | widen | widen | same type and name | widen | widen |
| Prefix | yes | widen | widen | old matches exact candidate | prefix hierarchy | widen |
| Suffix | yes | widen | widen | old matches exact candidate | widen | suffix hierarchy |

A hierarchy requires the same type. A candidate prefix base must equal the old
base or begin with `oldBase + "."`; a candidate suffix base must equal the old
base or end with `"." + oldBase`. Dot boundaries matter: `a2` is not below `a`.

The implementation checks seven rules in this order:

1. `CANDIDATE_EMPTY`: the candidate admits no requests.
2. `IDENTICAL_PATTERNS`: both patterns are byte-identical.
3. `OLD_GLOBAL`: the old pattern accepts every nonempty valid request.
4. `OLD_TYPED_ALL`: the old pattern accepts all names of the candidate's type.
5. `EXACT_CANDIDATE_ACCEPTED_BY_OLD`: the imported matcher accepts the candidate's sole request.
6. `PREFIX_HIERARCHY`: the candidate prefix is at or below the old prefix.
7. `SUFFIX_HIERARCHY`: the candidate suffix is at or below the old suffix.

These arguments concern access inclusion only. An empty candidate satisfies
inclusion while removing access to every required service.

## Why two candidate requests suffice in this grammar

If no positive rule applies, Gooo checks the following candidates in order.
It reports the first whose actual matcher results are candidate=true and
old=false. It never concludes inclusion from probes alone.

| Candidate family | First `(type, name)` | Second `(type, name)` |
| --- | --- | --- |
| Empty | none | none |
| Global | `(a, x)` | `(b, x)` |
| Typed-all | `(type, x)` | `(type, y)` |
| Exact | `(type, name)` | none |
| Prefix with base `q` | `(type, q.x)` | `(type, q.y)` |
| Suffix with base `q` | `(type, x.q)` | `(type, y.q)` |

Every listed request is accepted by its candidate. A typed old pattern cannot
accept both global seeds because their types differ. An exact old pattern
cannot accept two distinct names. Dot-anchored old prefixes and suffixes cannot
accept the typed-all seeds `x` and `y`, which contain no dot.

The longest common prefix of `q.x` and `q.y` is `q.`. An old prefix accepting
both must therefore be an ancestor of `q`, precisely the prefix rule. The
suffix argument is symmetric. A fixed nonempty old suffix cannot accept both
prefix seeds with their different final characters; a fixed nonempty old prefix
cannot accept both suffix seeds with their different first characters. Exact
candidates reduce to checking one request.

For a wildcard pattern, `len(type) + 3 + len(q) + 2 <= 128` and a nonempty type
imply `len(q) <= 122`. Each extended witness name is therefore at most 124 bytes,
within its independent 128-byte limit. Exact, typed-all and global seeds also
fit. The length bound does not exhaust the two-witness construction.

Together these arguments justify the matrix for this specific domain. They are
reviewed mathematical reasoning, not a mechanized proof of the compiler or
native implementation. Finite execution tests separately check the code,
including both-seed overlap traps and boundaries. `UNPROVEN` remains a defensive
fallback if neither a recognized rule nor a checked witness is available.

## Before applying a real policy change

This pure analyzer neither authorizes a caller nor applies a change. An
unattended host must separately establish the caller's authority and trusted
current old policy, compare its version atomically when applying a change, and
enforce required-service availability. A previous result is not authorization
to change a policy that has since changed. Roles, unions of grants, credentials
and the complete Agent Mesh authorization engine are outside this example.
