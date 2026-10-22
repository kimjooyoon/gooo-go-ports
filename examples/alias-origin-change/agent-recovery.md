# Alias-origin result handling

First distinguish a represented stdout result from a stderr diagnostic. Never convert a diagnostic, missing output or an unrecognized state into `UNCHANGED`.

| Decision state | Meaning | Next action |
| --- | --- | --- |
| `UNCHANGED` | The target exists exactly once on both sides with the same declared origin pair. | Retain the comparison as snapshot evidence; current execution policy remains separate. |
| `ORIGIN_CHANGED` | The target exists on both sides with a different server or original name. | Inspect the indexed before/after rows; invalidate any assumption that the old alias still names the same origin. |
| `REMOVED` | The target exists only before. | Remove or replan references to that alias against current supplied facts. |
| `ADDED` | The target exists only after. | Treat it as a new declared origin, subject to the host's current policy. |
| `ABSENT` | The target exists in neither supplied catalog. | Resolve the intended alias from the producer; do not invent a match. |
| `INVALID_INPUT` | Gooo rejected empty domain text or invalid internal columns. | Correct the producer input, preserving the original failure for diagnosis. The public row API cannot construct unequal column lengths. |
| `AMBIGUOUS` | At least one supplied catalog contains a duplicate exposed name. | Inspect `conflict_catalog` and its two indexes, repair the naming/catalog source, then compare fresh facts. |

Indexes are zero-based original row positions. Missing target indexes and unused conflict indexes are `-1`; unused conflict catalog is empty. `execution_authorized` is false in every represented result. A zero exit status does not make a catalog safe to execute.

| Diagnostic state / stage / code | Recovery |
| --- | --- |
| `INVALID_TRANSPORT / decode_json / INVALID_JSON_FACTS` | Emit exactly one JSON object with the documented keys, strings and arrays. Reject duplicate keys and lossy Unicode rather than repairing them. The same code covers the 1 MiB input bound. |
| `INVALID_TRANSPORT / decode_facts / COLLECTION_LIMIT` | Keep each supplied catalog within 256 rows. Do not silently truncate: that changes the comparison universe. |
| `INVALID_TRANSPORT / decode_facts / TEXT_LIMIT_OR_ENCODING` | Obtain valid UTF-8 text within the 1,024-byte bound. Do not trim identifiers to make them pass. |
| `INVALID_TRANSPORT / encode_result / OUTPUT_LIMIT_EXCEEDED` | Retain the failure; use smaller complete catalogs appropriate to the actual task. Do not treat partial facts as the full catalog. |
| `UNKNOWN / evaluate / KERNEL_FAILURE` | Stop using this comparison result; reproduce with the pinned source/toolchain and retain diagnostics. |
| `UNKNOWN / bind_result / KERNEL_CONTEXT_OR_AUTHORITY_MISMATCH` | Reject the result and investigate generation/context correspondence. |
| `UNKNOWN / command / IO_FAILURE` | Check input/output access and available storage. Discard incomplete output; rerun only after resolving the I/O cause. |

Diagnostics exit one. Input and binding failures produce no stdout decision. An output-device failure can interrupt a write; discard any partial JSON. There is no automatic network retry or external side effect in this module.

Validate schemas for structure, then use native parsing and Gooo for the represented calculation. A schema-valid digest is not proof that it matches the facts; unsigned bindings do not authenticate a producer. On a real host, enforce freshness, identity and effect authority separately. This is compatible with automated operation under enforced policy rather than requiring a person to approve each comparison.
