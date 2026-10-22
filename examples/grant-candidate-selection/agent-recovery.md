# Integrate a bounded grant proposal selector

Use this API when an automation has several supplied grant-change proposals and
must preserve the same ordered service-request list in every proposal. Gooo
admits and ranks candidates; the Go boundary evaluates the policy and binds the
actual returned facts and generation. The command changes no grant and invokes
no service. These assessments require no per-request human approval.

Start with the two-candidate weather/calculator example in the module README.
Keep both requirements. The lower-preference weather-only proposal is refused;
all-mcp is proposed because it preserves both. A supplied preference is not a
least-privilege proof. Preserve candidate order, duplicate requirements and the
complete proof for every candidate.

## Machine-readable transport shapes

- `input.schema.json` describes the native JSON arguments object.
- `output.schema.json` describes a represented result on stdout with exit zero.
- `diagnostic.schema.json` describes an error on stderr with exit one.

These are standalone JSON Schema draft 2020-12 documents, not registrations of
new tools in the released MCP bundles. The native parser still enforces raw
byte budgets, duplicate-key refusal, integer-token spelling, UTF-8 and surrogate
handling. A JSON Schema implementation may use imprecise numbers; use the
native parser for exact int64 boundaries. Schema conformance does not establish
fact correspondence, producer identity, freshness or authority.

The input schema deliberately leaves one candidate or a negative preference as
structurally representable: Gooo owns those admission decisions. A changed hash
with valid syntax deliberately passes the output schema; source/proof binding
must still be verified. The fixed JSON-schema validator corpus includes both
controls, as well as a refusal of `execution_authorized:true`.

## Choose the next action from the actual result

| Signal | Meaning | Next action |
| --- | --- | --- |
| `PROPOSED` | One internally evaluated candidate is eligible and won by supplied preference/order. | Retain the selected original assessment ID and every rejected proof. Pass the proposal to the integrating host's current-state and effect checks. |
| `REFUSED` | No assessment is eligible; original proofs may contain domain-invalid inputs. | Inspect each original status and first counterexample. Preserve requirements; do not drop a requirement to manufacture eligibility. |
| `INVALID_INPUT` | Gooo rejected candidate admission. No policy fan-out occurred. | Correct candidate count, stable unique IDs or preference according to the caller's actual intent. Preserve the original input and failure. |
| `UNKNOWN` diagnostic | The host could not bind a dependency result/generation, or execution failed. | Return no selection. Re-establish the reviewed dependency and facts before recomputing all candidates. This is distinct from policy refusal. |
| `INVALID_TRANSPORT` diagnostic | The byte/shape/output budget was refused. | Correct serialization only when original facts can be preserved losslessly. Do not infer missing facts, truncate policy text or use a partial report. |

The selector always returns `execution_authorized:false`. An integrating host
can apply changes without routine human approval only after its own authority,
current-fact and atomic-application preconditions are satisfied. This module
neither authenticates those facts nor supplies an effect executor.

## Diagnostic stage/code reference

| Stage | Codes | Interpretation |
| --- | --- | --- |
| `decode_input` | `INVALID_JSON_FACTS` | Raw JSON shape/encoding was not accepted; there is no policy result. |
| `admit_text` | `INVALID_UTF8`, `TEXT_LIMIT_EXCEEDED` | A supplied decoded Text is outside the Go boundary; `field` may identify it. |
| `bind_generation` | `GENERATION_UNREADABLE`, `GENERATION_PROFILE_MISMATCH`, `POLICY_BINDINGS_MISMATCH` | Embedded or dependency generation does not match the reviewed composition. |
| `bind_assessment` | `ASSESSMENT_BINDING_MISMATCH`, `ASSESSMENT_RESULT_INVALID` | The original result/facts/source or status was not usable; `index` identifies the candidate. |
| `evaluate_policy` | `DEPENDENCY_PANIC` | The evaluator failed; no partial result is returned. |
| `encode_result` | `OUTPUT_LIMIT_EXCEEDED` | The complete JSON report exceeded the wire budget; no decision is written. |
| `command` | `IO_FAILURE` | Command I/O failed. Stop consuming the stream; a physical output failure may leave incomplete bytes. |

Keep stderr separate from stdout, require the expected exit code, and parse one
complete document. Do not turn nonzero exit, interrupted output, an unknown
schema/state or a changed source binding into a successful choice. Retained
facts/diagnostics can contain caller data; share a minimal synthetic reproduction
rather than automatically uploading the original report.

The current composition uses Go 1.27.3 and compiler source
`205896a413e5dea92366309c35ae85e542d055f6` for source regeneration. Ordinary
compiled consumers need neither Gooo nor a model/API key at runtime. Full native
and replay verification is separate from structural schema validation.
