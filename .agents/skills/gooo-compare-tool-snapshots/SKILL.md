---
name: gooo-compare-tool-snapshots
description: Compare a selected MCP tool definition in saved before/after tools/list pages with Gooo during an upgrade or reconnect review. Use for retained JSON-RPC observations, not live collection or general MCP setup.
license: BSD-3-Clause; see LICENSE.
---

# Compare saved MCP tool definitions

Answer the user's comparison question with the observed relation, its limits,
or the first failure. This repository-local skill routes to the maintained
[own-input guide](../../../docs/ai-agents.md#compare-your-own-saved-pages).

- Locate a matching `kimjooyoon/gooo-go-ports` checkout and read its `AGENTS.md`
  plus that guide. A separately installed skill does not include the required
  repository resources. Use an existing checkout or obtain one within the user's
  requested setup, keeping existing files. If setup is unavailable, report that
  prerequisite. This comparison does not need a Gooo compiler installation.
- Prefer the user's saved pages and original tool name. Assemble the original
  request/response text with the guide's recipe; preserve additional pages and
  their order. Keep missing pages missing and carrier/extraction records separate.
- If the user wants a demonstration without their own saved pages, use the
  [three synthetic cases](../../../examples/mcp-upgrade-regressions/README.md).
  Label it a demo; it does not assess the user's server or prove SDK behavior.
- Follow the source route or an archive explicitly containing these examples.
  `recorded-catalog-cli-preview.1` lacks `--definition-snapshot`. Source execution
  needs Go 1.27.3; its generated evaluator needs no Gooo compiler bootstrap.
- Preserve command stderr/status and the report. Exit zero does not mean a safe
  upgrade: report the Gooo decision, both page coverage fields and its next action.
  Missing from an incomplete page set does not prove removal; a changed definition
  does not prove incompatibility. Do not invent host facts or authorize tool calls.
- Report whether the result answered the user's question and any unresolved
  point. Offer the guide's existing feedback form using public details or a
  synthetic reproduction; keep private captures local. Do not submit automatically.
