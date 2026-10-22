---
name: gooo-review-service-grants
description: Compare service-grant narrowing proposals with Gooo, preserve every ordered required request, and select eligible candidates by a supplied preference. Use for supported service-pattern reviews through the native Go command or connected MCP tool.
---

# Review service grants with Gooo

Use the actual Gooo assessment to compare proposed grants against the user's required requests. The [Gooo language and compiler](https://github.com/kimjooyoon/meta-ontology-go) produce the policy and candidate-selection logic; the native command or local MCP server transports the supplied facts and complete result. This review does not apply a grant or contact the services.

## Choose an available execution path

Preserve an explicitly requested MCP or native workflow. For automatic choice among 2-32 proposals with explicit nonnegative integer preferences, use the [native candidate selector](../../../examples/grant-candidate-selection/README.md) and its input schema. It takes one `before`, the ordered `required_requests`, and candidates containing only `id`, `after` and `preference`. With Go 1.27.3, the pinned public command is:

```sh
GOTOOLCHAIN=local GOWORK=off go run github.com/kimjooyoon/gooo-go-ports/examples/grant-candidate-selection/cmd/select@v0.0.0-20261022032513-52fb59a8ca9a < proposals.json
```

The lowest supplied preference wins among eligible candidates; ties retain input order. Do not invent a preference, treat it as least privilege, truncate candidates to fit the limit, or include caller-declared proof/authority flags. If preferences are absent, show the eligible set or ask for the intended ranking. An unavailable native path is a limitation, not a Gooo result.

For a connected MCP review, use the following path. Released MCP bundles do not expose candidate selection: `assess_service_grant_change` assesses one proposal against all ordered requirements. Do not invent a selector tool or confuse its `ELIGIBLE` status with native selection's `PROPOSED` state.

## Get the MCP assessment tool

Discover `assess_service_grant_change` in the connected MCP tools and inspect its actual schema. It requires the native server's `--many-required` mode. If setup is needed, follow the repository's [ordered-request MCP setup](../../../docs/ai-agents.md#preserve-several-required-requests-through-mcp); record the actual executable version and returned bindings. Existing preview.13 binaries expose only the single-request tool.

For several required requests, do not substitute `assess_service_grant` or independently combine single-request results. If the ordered-request tool is unavailable, report that limitation and the setup needed; do not invent an assessment. A model's reasoning is not a Gooo execution result.

## Assess the supplied proposals

Call the tool once for each proposed `after`, retaining the exact same `before` and `required_requests` for that comparison. Preserve request order and duplicates. Use JSON argument objects, not CLI batch arrays. Do not normalize case, trim strings, deduplicate requests, or translate another permission language into service patterns without an explicit domain conversion.

For example, an automation requiring both weather and calculator has these two proposals:

```json
{"before":"*","after":"mcp://*","required_requests":[{"type":"mcp","name":"weather"},{"type":"mcp","name":"calculator"}]}
```

```json
{"before":"*","after":"mcp://weather","required_requests":[{"type":"mcp","name":"weather"},{"type":"mcp","name":"calculator"}]}
```

For these exact supplied facts, the first proposal is `ELIGIBLE`. The second is `REFUSED` with `REQUIRED_REQUEST_REMOVED` and `first_counterexample: 1`, referring to calculator. Evaluate the user's actual facts rather than returning this example's answer.

## Keep the result useful

Save the original inputs and complete returned JSON in the task's output area, preserving previous results. Retain the ordered-column `facts`, all 12 typed `decision` fields, and complete compiler/generation/source `bindings`, including false and empty fields. Keep the actual returned identities; do not relabel an older binary with a current checkout's compiler version. When both are available, the text JSON and `structuredContent` should represent the same full object.

For the native selector, preserve the complete stdout in a fresh task output file instead of loading every proof into model context. Start with `result.selection`, then read the required entries in `result.assessments` and their original `proof` objects. Keep the full report and its `input_sha256`, `result.context_id`, `result.bindings` and `selection.selected_assessment_id`; a small display or file digest is not a replacement proof. Use the [native recovery guide](../../../examples/grant-candidate-selection/agent-recovery.md) for diagnostic stage/code failures. A nonzero exit supplies no policy result on stdout.

Summarize each proposal's status and reason, the original zero-based `first_counterexample` when present, and the saved proof location. `ELIGIBLE`, `REFUSED`, and represented `INVALID_INPUT` are assessment outcomes. A protocol/tool error has no usable policy result: report its first diagnostic, without a partial proof or a guessed decision.

For native selection, report the actual returned choice and preference semantics. When reviewing only MCP assessments, choose only among actual `ELIGIBLE` results using the user's stated preference; otherwise show the eligible set. Do not present a host-side comparison as execution of the Gooo selection activity. Calculating and selecting these bounded assessments needs no per-request human approval. Supplied facts remain unsigned, and `ELIGIBLE` is not proof of current host authority or an applied change. The [host application example](../../../examples/agentmesh-service-policy/conditional-apply.md) explains that separate boundary.

For an unsupported real task or failed first run, the [feedback form](https://github.com/kimjooyoon/gooo-go-ports/issues/new?template=problem.yml) accepts the first error and intended use. Share only public synthetic facts; submitting feedback remains the user's choice.
