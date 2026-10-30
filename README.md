# Gooo Go Ports


[![M8ven Score](https://m8ven.ai/badge/mcp/kimjooyoon-gooo-go-ports-5tjh5l?v=817edc3621e8254bda91c8d8df7e474b)](https://m8ven.ai/mcp/kimjooyoon-gooo-go-ports-5tjh5l?s=readme)

[Gooo (meta-ontology-go)](https://github.com/kimjooyoon/meta-ontology-go) is an
experimental programming language with a Go backend. This repository contains
selected Go ports checked against Go reference results, and original Gooo
programs with saved execution records. Start with a task you can inspect and reproduce.

## Compare saved MCP tool definitions

Use this when you have `tools/list` requests and responses from before and after
an upgrade or reconnect. Select one original tool name. Gooo compares its saved
definition, reports missing page coverage and retains source bindings.

In a checkout, ask your coding agent:

> Follow `.agents/skills/gooo-compare-tool-snapshots/SKILL.md`. Compare one
> original tool name in my saved before/after pages. Keep the original files and
> full report. Show the observed change, missing coverage and the first failure.

Choose the entry that fits your setup:

| Setup | Start here |
| --- | --- |
| Local command, with no Go/Gooo installation | [Native CLI preview.2](docs/ai-agents.md#install-the-native-snapshot-command), then the [own-page guide](docs/ai-agents.md#compare-your-own-saved-pages). |
| Existing MCP host | [Native MCP preview17](docs/ai-agents.md#compare-saved-definitions-through-mcp). Use the executable's absolute path, `args: ["--definition-snapshot"]`, and discover `gooo_compare_saved_tool_definitions`. |
| MCP host with Docker | [Preview17 container instructions](https://github.com/kimjooyoon/gooo-go-ports/releases/tag/v0.1.0-preview.17#container-for-mcp-clients-linux-amd64). Use the published image digest and enable `--definition-snapshot` after the image argument. |
| APM with Docker, targeting Codex | [Explicit stdio connection](docs/ai-agents.md#install-the-snapshot-mcp-connection-with-apm), with a pinned image and project target. |
| Go 1.27.3 already installed | [Fixed public Go module](docs/ai-agents.md#install-the-snapshot-command-with-go); no checkout or Gooo compiler needed. |

The native archives support Apple Silicon macOS and Linux x86-64. Download the
fixed version and matching checksum, verify it and extract into a fresh
directory. Server execution makes no model call or provider connection. Your
MCP host keeps its own account, configuration and permission settings.

If you have no saved pages, [try the three synthetic examples](examples/mcp-upgrade-regressions/README.md):
a changed output schema, identical definitions, and an incomplete after page.
They demonstrate the output format without assessing your server.

Read `decision.state` together with both page coverage fields. Keep the exact
`request_json` and `response_json` strings and page order. Input/context hashes
bind the actual arguments received; a CLI file's outer whitespace can differ
from an MCP arguments object. Preserve each original record and its input
identity. Results describe supplied observations; live behavior, origin and
authority remain unassessed, and no remote tool is called.

<a id="run-and-edit-locally"></a>

## Change and run a Gooo program

To try the language itself, ask your coding agent:

> Follow `.agents/skills/gooo-first-program/SKILL.md`. Run the Gooo example,
> change its score threshold from 60 to 80, then save and replay a new execution.
> Show the actual changed result and keep my existing files.

The [two-file tutorial](examples/learn-gooo/README.md) imports Gooo functions,
executes four supplied inputs and retains the edited replay. For score `75`,
the supplied edit changes `sensor: high` to `sensor: low`; its other three
returned fields stay unchanged.

Use the checkout's pinned compiler and Go **1.27.3**, following the
[first-program skill](.agents/skills/gooo-first-program/SKILL.md). Initial setup
may download and build tools. Reuse an existing compiler after its identity
check. [Tessl](docs/ai-agents.md#install-the-skill-with-tessl) and
[Gemini CLI](docs/ai-agents.md#use-the-instructions-with-gemini-cli) have separate
installation instructions for the same task.

Bootstrap retains the pinned compiler source in `.compiler` for later builds,
alongside the `.gooo` executable. Allow space for that checkout, temporary build
files and Go's separate shared caches. Inspect your local sizes with
`du -sh .compiler .gooo`. For a disposable trial, keep it in its own directory
and preserve the source edits and result records before removing that directory.

Port examples use the same checkout setup; follow each example's workspace and
input instructions. The [detailed setup at this source snapshot](https://github.com/kimjooyoon/gooo-go-ports/blob/e821c7967fdcf5bd01ec0c921a6e74b3971b19fc/README.md#run-and-edit-locally)
retains the longer command reference.

<a id="bootstrap-recovery"></a>

If compiler setup fails, use the [bootstrap recovery reference](https://github.com/kimjooyoon/gooo-go-ports/blob/e821c7967fdcf5bd01ec0c921a6e74b3971b19fc/README.md#bootstrap-recovery)
for the pinned compiler. Keep an existing working compiler until its identity and
any replacement have been checked.

An input-only `OBSERVED` result is a finite execution observation. A source edit
needs a new execution and replay record. The [browser tutorial](https://kimjooyoon.github.io/gooo-go-ports/tutorial.html)
runs a fixed compiled program; use the native workflow for an edit.

<a id="choose-the-next-useful-task"></a>

## Other useful tasks

| Task | Example and scope |
| --- | --- |
| Compare captured test outcomes with declared intent | [Gooo assessment session](examples/assessment-session/README.md). Inspect the actual program value separately from its test expectation, and retain the original records. Uses the example's explicit development compiler profile. |
| Preserve a stored catalog during an incomplete refresh | [File-plan command and host integration](docs/ai-agents.md#replace-a-catalog-after-refresh). Keep original pages, collection outcome and stored bytes. |
| Preserve required requests when narrowing a service grant | [Ordered-request assessment](examples/service-grant-many/README.md). Retain order, duplicate requests and the first counterexample. No grant is applied. |
| Select among supplied grant-change candidates | [Gooo candidate selection](examples/grant-candidate-selection/README.md). Compare the same requirements and retain the original decisions. |
| Compare a tool alias's declared origin | [Native preview17 or source setup](docs/ai-agents.md#compare-a-tool-alias-before-reusing-it). Detect changed origins and duplicate exposed names from supplied catalogs. |
| Change an API policy in Gooo | [Editable API policy](examples/editable-api-policy/README.md). Compare the edited rules on the same facts and replay each policy. |
| Check fixed API policies in CI | [Policy Action](examples/api-policy-ci/README.md). Compare two sealed profiles on the same supplied facts and inspect newly denied requests. |

Each example documents its represented domain, input limits and unsupported
cases. These tools calculate assessments without per-request human approval;
they do not authenticate supplied facts or apply live API changes. Native,
Homebrew, container and Registry packages have their own versioned capabilities;
follow the selected route's instructions.

<a id="검증-범위"></a>

## Sources, verification and contribution

- [`ports/`](ports/) contains Gooo ported behavior; [`examples/`](examples/)
  contains original compositions and their instructions.
- [`upstream.json`](upstream.json) records upstream and compiler identities.
  Keep the matching source revision, toolchain and license notices together.
- [Detailed port catalog and verification records at this snapshot](https://github.com/kimjooyoon/gooo-go-ports/blob/e821c7967fdcf5bd01ec0c921a6e74b3971b19fc/README.md#이식한-함수)
  retain the previous reference material. Use each example's instructions for its
  current or explicitly frozen setup.
- [Contributor guide](CONTRIBUTING.md) and [porting contract](AGENTS.md) describe
  source changes, independent Go results, boundary cases and saved replay.
  Finite observations do not establish complete upstream-library compatibility.
- [Repository license](LICENSE). Upstream works retain their own notices and
  licenses; preserve them when using or extending a port.

[Report an actual use or first failure](https://github.com/kimjooyoon/gooo-go-ports/issues/new?template=problem.yml)
with the task, exact version, public input or smallest reproduction, and actual
result. Keep private source, credentials and personal paths out of the report.
For first-program attempts, select **Coding-agent first-program skill**. Reports
from your own tasks help decide what to build next.
