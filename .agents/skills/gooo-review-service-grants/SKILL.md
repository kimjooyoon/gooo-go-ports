---
name: gooo-review-service-grants
description: Review service-grant narrowing proposals with Gooo for one required request or an ordered list, and select eligible candidates by a supplied preference. Use for supported service-pattern reviews through the native Go command or connected MCP tools.
---

# Review service grants with Gooo

Use the actual Gooo assessment to compare proposed grants against the user's required requests. The [Gooo language and compiler](https://github.com/kimjooyoon/meta-ontology-go) produce the policy and candidate-selection logic; the native command or local MCP server transports the supplied facts and complete result. This review does not apply a grant or contact the services.

## Choose an available execution path

Preserve an explicitly requested MCP or native workflow. For automatic choice among 2-32 proposals with explicit nonnegative integer preferences, use the [native candidate selector](https://github.com/kimjooyoon/gooo-go-ports/blob/52fb59a8ca9a420d646419934af923ca042fa28e/examples/grant-candidate-selection/README.md) and its input schema. It takes one `before`, the ordered `required_requests`, and candidates containing only `id`, `after` and `preference`. With Go 1.27.3, the pinned public command is:

```sh
GOTOOLCHAIN=local GOWORK=off go run github.com/kimjooyoon/gooo-go-ports/examples/grant-candidate-selection/cmd/select@v0.0.0-20261022032513-52fb59a8ca9a < proposals.json
```

The lowest supplied preference wins among eligible candidates; ties retain input order. Do not invent a preference, treat it as least privilege, truncate candidates to fit the limit, or include caller-declared proof/authority flags. If preferences are absent, show the eligible set or ask for the intended ranking. An unavailable native path is a limitation, not a Gooo result.

For a connected MCP review, choose the assessment tool below. Each MCP call assesses one proposal; released MCP bundles do not expose candidate selection. Do not invent a selector tool or confuse an assessment's `ELIGIBLE` status with native selection's `PROPOSED` state.

## Get the MCP assessment tool

Discover the connected tools and inspect their actual schemas before choosing:

- **One required request:** use `assess_service_grant` when available. It takes exactly four strings: `before`, `after`, `required_type`, and `required_name`. The default server exposes it without extra arguments, including in preview.13. A single-request review needs no `--many-required` setup. If the host exposes only `assess_service_grant_change`, its schema also accepts a one-element `required_requests` list; preserve an explicitly requested ordered workflow.
- **An ordered request list:** use `assess_service_grant_change` with `before`, `after`, and `required_requests`, whose items contain `type` and `name`. Preserve order and duplicates. An explicitly supplied empty list or a list beyond the policy limit belongs to this tool too: return its actual `INVALID_INPUT` decision without inventing, dropping or splitting requirements.

If a required tool is missing, inspect the configured server arguments and the host's enabled-tool filter. In preview.16, `--many-required` adds the ordered tool alongside the single-request tool, but a host can hide either one. Use the [default connection guide](https://github.com/kimjooyoon/gooo-go-ports/blob/98da0fe1e41131366d7d4390c4dc7964bd62d5bd/docs/ai-agents.md#connect-the-service-grant-mcp-tool) or [ordered-request setup](https://github.com/kimjooyoon/gooo-go-ports/blob/98da0fe1e41131366d7d4390c4dc7964bd62d5bd/docs/ai-agents.md#preserve-several-required-requests-through-mcp) only when needed. Preview.13 cannot expose the ordered tool. Record the actual executable version and returned bindings.

For several required requests, do not substitute `assess_service_grant` or independently combine single-request results. If the ordered-request tool is unavailable, report that limitation and the setup needed; do not invent an assessment. A model's reasoning is not a Gooo execution result.

## Assess the supplied proposals

Call the selected tool once for each proposed `after`, retaining the exact same `before` and required request facts. Use JSON argument objects, not CLI batch arrays. Keep the original arguments for another call: the ordered tool returns `facts.required_types` and `facts.required_names` columns, which are not its `required_requests` input schema. Do not normalize case, trim strings, deduplicate requests, or translate another permission language into service patterns without an explicit domain conversion.

For one required weather service, call `assess_service_grant` with:

```json
{"before":"*","after":"mcp://*","required_type":"mcp","required_name":"weather"}
```

For an automation requiring both weather and calculator, call `assess_service_grant_change` with each proposal:

```json
{"before":"*","after":"mcp://*","required_requests":[{"type":"mcp","name":"weather"},{"type":"mcp","name":"calculator"}]}
```

```json
{"before":"*","after":"mcp://weather","required_requests":[{"type":"mcp","name":"weather"},{"type":"mcp","name":"calculator"}]}
```

For these exact supplied facts, the first proposal is `ELIGIBLE`. The second is `REFUSED` with `REQUIRED_REQUEST_REMOVED` and `first_counterexample: 1`, referring to calculator. Evaluate the user's actual facts rather than returning this example's answer.

## Keep the result useful

Save the original inputs and complete returned JSON in the task's output area, preserving previous results. Both tools return `facts`, `decision`, and `bindings`: retain all seven typed decision fields for `assess_service_grant`, or the ordered-column facts and all 12 typed decision fields for `assess_service_grant_change`. Preserve complete compiler/generation/source bindings, including false and empty fields. Do not add ordered-tool fields such as `first_counterexample` to a single-request proof. Keep the actual returned identities; do not relabel an older binary with a current checkout's compiler version. When both are available, the text JSON and `structuredContent` should represent the same full object.

For the native selector, preserve the complete stdout in a fresh task output file instead of loading every proof into model context. Start with `result.selection`, then read the required entries in `result.assessments` and their original `proof` objects. Keep the full report and its `input_sha256`, `result.context_id`, `result.bindings` and `selection.selected_assessment_id`; a small display or file digest is not a replacement proof. Use the [native recovery guide](https://github.com/kimjooyoon/gooo-go-ports/blob/52fb59a8ca9a420d646419934af923ca042fa28e/examples/grant-candidate-selection/agent-recovery.md) for diagnostic stage/code failures. A nonzero exit supplies no policy result on stdout.

Summarize each proposal's status and reason, the original zero-based `first_counterexample` when present, and the saved proof location. `ELIGIBLE`, `REFUSED`, and represented `INVALID_INPUT` are assessment outcomes. A protocol/tool error has no usable policy result: report its first diagnostic, without a partial proof or a guessed decision.

For native selection, report the actual returned choice and preference semantics. When reviewing only MCP assessments, choose only among actual `ELIGIBLE` results using the user's stated preference; otherwise show the eligible set. Do not present a host-side comparison as execution of the Gooo selection activity. Calculating and selecting these bounded assessments needs no per-request human approval. Supplied facts remain unsigned, and `ELIGIBLE` is not proof of current host authority or an applied change. The [host application example](https://github.com/kimjooyoon/gooo-go-ports/blob/52fb59a8ca9a420d646419934af923ca042fa28e/examples/agentmesh-service-policy/conditional-apply.md) explains that separate boundary.

For an unsupported real task or failed first run, the [feedback form](https://github.com/kimjooyoon/gooo-go-ports/issues/new?template=problem.yml) accepts the first error and intended use. Share only public synthetic facts; submitting feedback remains the user's choice.
