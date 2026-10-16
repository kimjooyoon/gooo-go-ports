# Diagnostic header addition plan

This original Gooo composition combines the header and payload reports into one
16-field decision about adding a route-approved `x-request-id` or
`x-correlation-id`. It computes a pure, automatic plan without per-request human
approval. It does not modify or send a request.

The four inputs are candidate name Text, candidate value Text, original
Content-Length Text and the 12-field `RequestFacts` record in
[the contract](contract.json). All four Text values, including the record's
`allowed_header_name`, must be decoded valid UTF-8 of at most 1,024 bytes.
Integers remain exact signed int64 and Booleans are exact. Empty or header-invalid
in-domain Text and negative or exhausted metadata are policy inputs, rather than
transport errors. Neither Text nor field names are normalized.

The route policy permits a nonempty ASCII value of at most 128 bytes, with no
leading or trailing SP/HTAB. Internal space and HTAB are deliberately permitted.
The candidate name must exactly equal the current route's lowercase allowed
name, with zero existing occurrences counted case-insensitively by the host.
These restrictions are application policy, not requirements of the imported
HTTP predicates. The value is an opaque diagnostic token; no format, randomness,
uniqueness or suitability as an authentication credential is established.

`status` is `INVALID_INPUT`, `REFUSED` or `ELIGIBLE`; `reason` records the first
failed guard. Payload and raw header diagnostics are computed even when an
earlier context refusal wins. The result preserves candidate name/value and the
exact observed revision. Either request revision at signed64 maximum is refused:
an eligible plan must leave room for an atomic revision advance. See the contract
for all 16 fields, byte-offset sentinels and exact refusal precedence.

From the repository root, after the pinned repository setup:

```sh
mkdir -p out/diagnostic-header-addition
go run ./cmd/header-addition-input \
  < examples/diagnostic-header-addition/request.json \
  > out/diagnostic-header-addition/inputs.json
./.gooo package execute --json --go "$(go env GOROOT)/bin/go" \
  --inputs out/diagnostic-header-addition/inputs.json \
  diagnostic-header-addition.workspace.json \
  > out/diagnostic-header-addition/execution.json
./.gooo package replay --json --go "$(go env GOROOT)/bin/go" \
  --receipt out/diagnostic-header-addition/execution.json \
  --inputs out/diagnostic-header-addition/inputs.json \
  diagnostic-header-addition.workspace.json \
  > out/diagnostic-header-addition/replay.json
```

The input command accepts exactly `name`, `value`, `declared_length` and `facts`;
the record must contain exactly its 12 fields. It rejects malformed/trailing
JSON, invalid UTF-8, lone surrogate escapes, decoded duplicate keys, missing or
extra fields, null/wrong types, fractional/exponent/out-of-int64 integers,
Text over 1,024 decoded bytes and raw requests over 32 KiB. A transport refusal
produces no input document. It emits the actual `gooo/body-composition-inputs/v1`
top-level `inputs` array and performs no policy calculation or Gooo fallback.
The checked-in `request.json` and `inputs.json` represent the same first frozen
witness. The command should be connected to Gooo only after it exits successfully.

The record must come from a trusted host's own current request, body, authority
and route/policy observations. Receiving an untrusted person's asserted facts
does not make them true. If a host applies an eligible plan, it must atomically
recheck the request revision, current authority/policy and still-absent field,
insert exactly the assessed name/value and advance the revision. This Gooo
composition and the input command implement none of those effects or locks.
The deliberately restrictive payload policy requires an already observed complete
body and refuses TE/chunked requests. Do not buffer a streaming body merely to
add a diagnostic field. There is no HTTP parsing/framing, general header repair,
HTTP/2 compatibility, authorization proof or production-safety claim here.

`corpus.json` preserves 50 ordered inputs and four partial specification assertions
per input. Its historical `SPECIFICATION_ONLY_NOT_EXECUTED` marker describes the
specification file; it does not report current verification. The verifier executes
actual pinned `httpguts.ValidHeaderFieldName`, `ValidHeaderFieldValue` and
`strconv.ParseInt(text, 10, 64)` and computes all 16 expected fields freshly.
It then compares current Gooo native results and saved replay, rather than
loading a private run's output. Fresh public acceptance is required for every
source change. Finite observations do not establish all-input compatibility;
independent inner typed canonical parent digest acceptance remains `NOT_CHECKED`.

This original composition uses the repository [BSD-style license](../../LICENSE).
Imported [header facts](../header-field-facts/README.md),
[payload facts](../payload-admission/README.md),
[httpguts](../../ports/net/httpguts/README.md) and
[decimal parser](../../ports/strconv/README.md) preserve their own contracts,
upstream attribution and immutable source pins.
