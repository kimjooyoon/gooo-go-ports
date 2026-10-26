---
name: gooo-compare-tool-snapshots
description: Compare a selected MCP tool definition in saved before/after tools/list pages with Gooo during an upgrade or reconnect review. Use for retained JSON-RPC observations, not live collection or general MCP setup.
license: BSD-3-Clause; see LICENSE.
---

# Compare saved MCP tool definitions

Answer the user's comparison question with the observed relation, its limits,
or the first failure. This skill routes to the
[own-input guide](https://github.com/kimjooyoon/gooo-go-ports/blob/d80cb8637cd79acf80413f0fa56db7771583b661/docs/ai-agents.md#compare-your-own-saved-pages).

- Read that guide from a matching `kimjooyoon/gooo-go-ports` checkout, or use the
  [published native installation guide](https://github.com/kimjooyoon/gooo-go-ports/blob/d80cb8637cd79acf80413f0fa56db7771583b661/docs/ai-agents.md#install-the-native-snapshot-command).
  Prefer the fixed `recorded-catalog-cli-preview.2` archive when it matches the
  user's platform and requested setup; it needs no checkout or Go/Gooo installation.
  A separately installed skill includes no executable. For a source route, read
  the checkout's `AGENTS.md` and preserve existing files. Report a missing setup
  prerequisite instead of treating it as a comparison result.
- Prefer the user's saved pages and original tool name. Assemble the original
  request/response text with the guide's recipe; preserve additional pages and
  their order. Keep missing pages missing and carrier/extraction records separate.
- If the user wants a demonstration without their own saved pages, use the
  [three synthetic cases](https://github.com/kimjooyoon/gooo-go-ports/blob/d80cb8637cd79acf80413f0fa56db7771583b661/examples/mcp-upgrade-regressions/README.md).
  Label it a demo; it does not assess the user's server or prove SDK behavior.
- Follow the fixed-version native guide or source route. The preview.2 archive
  contains `mcp-upgrade-regressions/` and `START-HERE.txt`; preview.1 lacks
  `--definition-snapshot`. Source execution needs Go 1.27.3 and no Gooo compiler
  bootstrap. Editing the Gooo rules is a separate source-edit task requiring new
  execution and replay; consuming the archive does not change those rules.
- Preserve command stderr/status and the report. Exit zero does not mean a safe
  upgrade: report the Gooo decision, both page coverage fields and its next action.
  Missing from an incomplete page set does not prove removal; a changed definition
  does not prove incompatibility. Do not invent host facts or authorize tool calls.
- Report whether the result answered the user's question and any unresolved
  point. Offer the guide's existing feedback form using public details or a
  synthetic reproduction; keep private captures local. Do not submit automatically.

For a request to replace an entire catalog after refresh, use the source-based
[catalog refresh example](https://github.com/kimjooyoon/gooo-go-ports/blob/main/examples/catalog-refresh/README.md).
Read the guide from the matching source checkout before setup; native preview.2
does not contain this transaction.
The selected-row comparison cannot authorize whole-catalog replacement. That
example uses a separate Gooo decision and local commit receipt; it does not
collect live pages or grant permission to invoke retained tools.
