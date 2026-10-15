# Payload admission facts

This original Gooo composition imports the decimal parser and returns a
14-field explanation for seven caller-supplied observations. It computes a pure,
deterministic eligibility decision without per-request human approval or effects.
It does not read a request, enforce authorization, consume a quota or send a body.

The inputs are positional in the workspace CLI:

| Port | Named fact | Domain |
| --- | --- | --- |
| `input0` | `declared_length` | Original decoded valid UTF-8 Text, at most 1,024 bytes |
| `input1` | `content_length_occurrences` | Exact signed int64 count of all original field occurrences |
| `input2` | `observed_body_bytes` | Exact signed int64 complete observed body length |
| `input3` | `maximum_payload_bytes` | Exact signed int64 limit for the applicable context |
| `input4` | `transfer_encoding_present` | Exact Boolean |
| `input5` | `body_observation_bound_to_request` | Exact Boolean assertion by the host |
| `input6` | `current_access` | Exact Boolean assertion by the host |

The result echoes all seven inputs and adds `parse_error_kind`, `parsed_value`,
`unsigned_decimal`, `length_matches`, `within_budget`, `eligible` and `reason`.
Parser facts are computed even if an earlier metadata or context refusal wins.
`parse_error_kind` exposes success, syntax and range as `0`, `1` and `2`;
`parsed_value` preserves the parser's exact int64, including range saturation.
Saturation never makes a length match. All integer inputs remain exact, including
values above 2^53 and signed int64 extrema; there is no float conversion.

`unsigned_decimal` requires a nonempty sequence of ASCII digits. Leading zeros
are allowed; Go-valid `+1` and `-0` are refused by this policy. Repeated fields
are refused even when their values agree. One comma-containing field remains one
occurrence with its complete raw value and is not split into a valid decimal.

Reasons follow this first-match order:

| Code | Reason |
| --- | --- |
| 1 | Invalid negative metadata, or zero occurrences with a nonempty value |
| 2 | Current access denied |
| 3 | Body observation unbound |
| 4 | Transfer-Encoding present |
| 5 | Content-Length missing |
| 6 | Content-Length repeated |
| 12 | Defensive unsupported or inconsistent imported parser facts |
| 7 | Decimal syntax error |
| 8 | Decimal range error |
| 9 | Unsigned decimal spelling required |
| 10 | Declared and observed lengths differ |
| 11 | Observed body exceeds the limit |
| 0 | Eligible within all supplied facts |

`eligible` is true exactly for reason zero. Other derived facts are descriptive
and may be true in a refused report. Reason `12` is defensive and cannot be
reached by the fixed compliant parser in its stated domain; these input fixtures
do not claim to exercise it. No report is invented after a compiler/import refusal.

The host must truthfully bind the raw header value, original occurrence count,
TE presence, complete body observation, applicable limit and current authority to
the same request, body, principal, operation and policy context. Unknown access
must be false; unavailable or unverified observations must be unbound. Do not
invent an unknown count, body length or limit as zero. Gooo cannot establish
that a binding flag is true, authenticate those facts or prevent their changing
after calculation. Enforcing hosts must handle current authority, revocation,
TOCTOU, actual framing, body completeness and atomic quota/reservation needs.

This deliberately restrictive single-Content-Length/no-TE policy refuses some
otherwise valid HTTP requests. It does not implement `net/http`, RFC framing,
chunked decoding, duplicate-header reconciliation or request-smuggling prevention,
and it is not a production security guarantee.

After the [repository setup](../../README.md), run from the repository root:

```sh
mkdir -p out/payload-admission-example
./.gooo package execute --json --go "$(go env GOROOT)/bin/go" \
  --inputs examples/payload-admission/inputs.json payload-admission.workspace.json \
  > out/payload-admission-example/execution.json
./.gooo package replay --json --go "$(go env GOROOT)/bin/go" \
  --receipt out/payload-admission-example/execution.json \
  --inputs examples/payload-admission/inputs.json payload-admission.workspace.json \
  > out/payload-admission-example/replay.json
```

`manual17.json` preserves the original ordered witnesses. `corpus.json` keeps
those first and expands to 101 unique input-only tuples: all eight Boolean
contexts for one observation, signed metadata extremes, occurrence/spelling
combinations, syntax/range boundaries, mismatched length/budget combinations and
context-versus-parse precedence. The generator reuses an existing seed witness
when a boundary tuple is identical; it never substitutes expected values.
Recheck exact input bytes without writing:

```sh
python3 -I -B tools/generate-payload-corpus.py --check
```

Fresh public verification must compare every result with an independent pinned
Go reference using actual `strconv.ParseInt`, preserve all seven typed inputs
and validate saved replay. Source preparation and generator checks are not native
acceptance. The finite corpus does not cover all contexts, raw malformed JSON,
all int64/Text values or host enforcement. The [contract](contract.json) fixes
the exact field IDs and semantics. This original composition uses the repository
BSD license; its imported [parser](../../ports/strconv/README.md) retains Go BSD
attribution and immutable source pins.
