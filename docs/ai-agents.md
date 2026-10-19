# Use Gooo with a coding agent

This repository provides a small, source-based Gooo workflow for a local coding
agent. Start with a concrete task: edit a Gooo program, execute the changed
source, and save a new execution/replay pair.
The [Gooo language and compiler](https://github.com/kimjooyoon/meta-ontology-go)
are developed separately; this repository supplies ports and runnable examples.

## Check a service-grant change without a toolchain

Start with [native preview.12](https://github.com/kimjooyoon/gooo-go-ports/releases/tag/v0.1.0-preview.12)
for **Linux amd64 or macOS arm64**. Download the matching archive and `.sha256`
file, verify the checksum using the release instructions, then extract into a
fresh private directory. The executable needs no Go, Gooo or Python installation.
From the extracted directory, submit one request as a **JSON array**:

```sh
printf '%s\n' '[{"before":"*","after":"mcp://*","required_type":"mcp","required_name":"weather"}]' |
  ./gooo-service-grant-evaluate -mode grant-update
```

This narrows `*` to `mcp://*` while preserving `mcp://weather`, returning
`ELIGIBLE` with reason `SUBSET_AND_REQUIRED_REQUEST_PRESERVED`. Retain the
original facts, all seven `decision` fields and the complete source/generation
bindings. The bundle also includes `required-removed-facts.json` (`REFUSED`) and
`domain-invalid-facts.json` (`INVALID_INPUT`). These are represented decisions
with exit zero. Structurally rejected JSON, such as duplicate keys, exits one
with no result on stdout; preserve stderr and do not report it as a policy refusal.

This [Gooo](https://github.com/kimjooyoon/meta-ontology-go) calculation preserves
one required request per change within its documented service-pattern domain.
It checks supplied facts only: no grant is applied, no authority is
authenticated and no service is called. Use the [MCP connection below](#connect-the-service-grant-mcp-tool)
for an agent-callable version of the same assessment, or send
[first-run feedback](https://github.com/kimjooyoon/gooo-go-ports/issues/new?template=problem.yml)
with your platform, release, first failed command and diagnostic.

## Compare fixed API policies without a compiler

For an optional look at decision output, use the
[native preview.11 bundles](https://github.com/kimjooyoon/gooo-go-ports/releases/tag/v0.1.0-preview.11).
Select Linux amd64 or macOS arm64, download the archive and its matching checksum,
verify it, then extract into a fresh private directory. A local shell, tar and a
checksum tool are needed; running the bundle needs no Go, Gooo or Python.
An agent needs local file and process tools. Follow `START-HERE.txt` to compare
the supplied read/write batch, edit synthetic copies of the nine-field facts,
and replay the saved comparison. A policy denial is exit zero; preserve stderr
and the nonzero exit for a failure. Use the [existing feedback form](https://github.com/kimjooyoon/gooo-go-ports/issues/new?template=problem.yml),
select **Native setup, verification, or replay**, and say whether this helps a real
task or report the first failed command and diagnostic. Share public data only.
The profiles are fixed, facts are unsigned and the command performs no API action.
To edit Gooo source, use the first-program compiler workflow below.

For a review of the supplied API policy profiles, start with the
[immutable command quickstart](../examples/api-policy-revision/README.md#install-the-command-without-cloning).
It installs into a private bin directory without cloning or installing Gooo.
Go 1.27.2 is required; installation uses the public Go proxy and checksum service.
After installation, the same command creates a baseline, compares the same nine
supplied facts, and checks the complete saved report with `-mode replay`.

An agent can use `--diagnostics=json` and inspect stdout, stderr and the exit
code separately. `NEWLY_DENIED` is a successful policy comparison; a tampered
saved report exits one, emits no result, and reports `REPORT_MISMATCH` at
`verify_comparison` on stderr. Keep the first failure and exact installed version
for a synthetic reproduction. Do not treat an error as a policy decision.

For a build of the current source, first run `compare --describe` without input
to obtain JSON capability help: the fixed profiles, embedded generation hashes,
nine fact fields, mode and batch limits, and generation compiler pin. See the
[current-command example](../examples/api-policy-revision/README.md#describe-the-current-command).
This reads no stdin and executes no policy or external tool. It is declared
capability metadata, not producer authentication or new native acceptance.
Do not combine it with evaluation flags. The older immutable installation
above and published bundles may not support it; keep their failure and exact
identity instead of assuming that they match the current source.

This route evaluates two fixed Gooo profiles. To change Gooo source or build a
new policy generation, use the compiler workflow below. A successful comparison
or replay does not grant permission to execute an API action.

## Run the first-program skill

The shared skill is
[gooo-first-program](../.agents/skills/gooo-first-program/SKILL.md). It uses the
existing [two-file tutorial](../examples/learn-gooo/README.md), its four supplied
inputs, and the repository's pinned compiler and Go toolchain. Setup needs Git,
a POSIX shell and Go toolchain selection on macOS or Linux.

To avoid building the pinned compiler, use the [pinned 0.6.32 release](https://github.com/kimjooyoon/meta-ontology-go/releases/tag/v0.6.32-dev)
archive and its SHA256SUMS: verify first, extract into a fresh directory, then
check `version --build --json` with the checkout's `tools/bootstrap-check.go`.
Keep existing compiler files untouched and use the checked executable by absolute
path, skipping bootstrap only after validation. Host Go **1.27.2** is still required.

After opening a checkout in your agent, ask:

> Follow `.agents/skills/gooo-first-program/SKILL.md`. Run the Gooo example,
> change its score threshold from 60 to 80, then save and replay a new execution.
> Show the actual changed result and keep my existing files.

| Client | Repository workflow |
| --- | --- |
| Codex | Discovers repository `.agents/skills` metadata; explicitly select `$gooo-first-program` or use the task above. |
| Cursor | Supports repository `.agents/skills`; use its skill selector or the task above. |
| GitHub Copilot CLI | Documents repository `.agents/skills` support; use the explicit task above. Copilot activation has not been tested here. |
| Claude Code | Use the explicit file-based task above. For native skill discovery, copy this skill directory into a new `.claude/skills/gooo-first-program` directory without replacing an existing skill. |
| Gemini CLI | The optional extension below loads the same first-program file as extension context. |

These are local repository or user-installed workflows. A public skill file does
not install itself into unrelated sessions. Follow the client's scope and trust
settings when copying or installing a skill. Official discovery guidance:
[Codex](https://learn.chatgpt.com/docs/build-skills),
[Cursor](https://cursor.com/docs/skills),
[GitHub Copilot CLI](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-skills),
[Claude Code](https://code.claude.com/docs/en/skills).

The task changes actual Gooo source. The [browser tutorial](https://kimjooyoon.github.io/gooo-go-ports/tutorial.html)
runs a fixed compiled program; use the native workflow to verify a source edit.
The skill explains the signed-int64 and 1,024-byte Text limits of this example.
An input-only `OBSERVED` result is a finite execution observation, not a proof
for all inputs. Preserve a first failure and its diagnostic.

Newer checkouts also provide a [saved-change comparison command](../examples/learn-gooo/README.md#compare-the-saved-change).
For the supplied threshold edit, it joins both source identities, the compiler,
every changed and unchanged field, and the edited replay parent in one JSON
summary. Run it after saving all three records. It checks the tutorial's bounded
record profile; it does not execute an agent or authorize effects. The older
immutable installation snapshots below retain their original manual workflow.

### Use the instructions with Gemini CLI

The root [extension manifest](../gemini-extension.json) points directly to the
existing first-program file. It supplies extension context, rather than a lazily
activated Gemini Agent Skill. There is one maintained copy of the instructions.

Gemini CLI installs extensions in its user profile and enables them globally by
default. Review this repository and the installation scope, then use its
[extension installation flow](https://geminicli.com/docs/extensions/):

```sh
gemini extensions install https://github.com/kimjooyoon/gooo-go-ports \
  --ref ab76fcdab771d65099ae19c68854438f20dc7c06
gemini extensions list
```

Check that `gooo-first-program` is listed, then start Gemini CLI in your Gooo
task checkout and use the first-program task above. The command pins the extension
to the tested commit; its instructions retain their own immutable tutorial
snapshot, compiler revision and Go version. Use the documented native setup to
run or edit Gooo source and preserve existing project files.

The manifest adds no MCP server or hooks. Gemini CLI **0.64.0** validated this
manifest and linked the local extension in a fresh Gemini CLI profile on macOS.
Its JSON list reported one active extension with the existing first-program
context path and no registered Agent Skills. A separate remote installation at
the pinned commit matched the manifest and canonical context byte-for-byte and
reported that same active context path. Model context injection and a
Gemini-completed Gooo task remain unverified. A gallery listing alone does not
establish that an agent completed the task.

### Install the skill with Tessl

If Tessl is already installed and available on your `PATH`, start in a fresh
project directory and install only the first-program skill for Codex:

```sh
tessl install \
  https://github.com/kimjooyoon/gooo-go-ports/tree/main/.agents/skills/gooo-first-program \
  --skill gooo-first-program --agent codex --yes
```

This is a direct GitHub-source installation; a Tessl registry listing or login
is not required. Review the source before installation. Tessl can initialize the
new project, create `AGENTS.md` and `.codex/config.toml`, and add its project-local
skill links and `.tessl` files. Use an empty directory to keep existing agent
configuration untouched. Inspect `tessl.json` for the resolved source commit;
the `main` URL follows the current branch rather than pinning a future update.

Then ask your agent to follow the installed
`.agents/skills/tessl__gooo-first-program/SKILL.md` and perform the first-program
task above. Installation supplies instructions; the skill still guides separate
example/toolchain setup and actual Gooo execution.

A macOS trial with Tessl **0.116.0** used the same branch/folder URL format,
resolved [source commit `495f6a4`](https://github.com/kimjooyoon/gooo-go-ports/commit/495f6a4942243168688e3ea8d57698b003c3dd4e),
and installed the skill, UI metadata and BSD license byte-for-byte. Its two
Codex skill links stayed inside the fresh project. Client activation and
independent use were not tested. Public registry searches by skill name and
repository URL did not return this skill in that trial; use the direct source
route. See the official [installation command](https://docs.tessl.io/reference/cli-commands#tessl-install)
and [registry MCP tools](https://docs.tessl.io/reference/mcp-tools).

### Install the skill with GitHub CLI

GitHub CLI's preview `skill` commands can fetch this skill directly. This route
was tested with `gh` **2.97.0**. Start in a fresh Git project, or a project with
no existing `.agents/skills/gooo-first-program`, and review the pinned content:

```sh
gh skill preview kimjooyoon/gooo-go-ports \
  .agents/skills/gooo-first-program@9ac1f5786999be9affec14bd4dc73ce3fc3e97f0 \
  --allow-hidden-dirs
```

Then install for Codex at project scope and inspect the installed version:

```sh
gh skill install kimjooyoon/gooo-go-ports .agents/skills/gooo-first-program \
  --pin 9ac1f5786999be9affec14bd4dc73ce3fc3e97f0 \
  --allow-hidden-dirs --agent codex --scope project
gh skill list --agent codex --scope project \
  --json skillName,sourceURL,scope,version,pinned,path
```

Use the exact path and pin: the skill lives under the hidden `.agents`
directory, which requires the explicit flag. The installed instructions still
guide a separate example checkout and pinned compiler setup; installation
alone does not run an agent, install Gooo or execute the example.

One fresh macOS project installation returned this source version with
`pinned: true` and installed two files. GitHub CLI added source metadata to
`SKILL.md` and removed its first body blank line; the remaining body and the
agent manifest matched the pinned source. Client activation and independent
use were not tested. See the official
[preview](https://cli.github.com/manual/gh_skill_preview),
[install](https://cli.github.com/manual/gh_skill_install) and
[list](https://cli.github.com/manual/gh_skill_list) commands. In this CLI version,
keyword search does not expose an opt-in for the supported hidden paths; an
empty search result does not determine whether the file is indexed. Use the
explicit preview and install commands above.

### Optional project-local installation with npm

To start outside a repository checkout, review the
[pinned skill](https://github.com/kimjooyoon/gooo-go-ports/tree/c4d0c54ffc51cd2cc015d81a828bb1e96fbe1b79/.agents/skills/gooo-first-program)
and use a fresh project directory with no existing copy of this skill. Prepare
Node.js **22.20 or later**, npm and Git, then run from that directory:

```sh
DISABLE_TELEMETRY=1 npm exec --yes --ignore-scripts \
  --package=skills@1.7.2 -- \
  skills add https://github.com/kimjooyoon/gooo-go-ports/tree/c4d0c54ffc51cd2cc015d81a828bb1e96fbe1b79/.agents/skills/gooo-first-program \
  --skill gooo-first-program --agent codex --copy --yes
```

This selects a project-local copy for Codex. It writes the skill under
`.agents/skills/gooo-first-program` and records the source commit in
`skills-lock.json`. Review those files, then use the explicit first-program task
above. The skill guides a separate checkout, tool setup and actual Gooo run;
installing its instructions does not install the compiler.

One macOS sandbox installation used the SRI-verified `skills` **1.7.2** tarball
with `npm exec` and the exact source URL and flags above. Both installed files
matched the pinned public Git blobs. The command above obtains that CLI version
from npm instead of the local tarball used in the test. Client activation and
independent adoption remain unverified. See the
[official CLI](https://github.com/vercel-labs/skills/tree/671e8c320810d36fed80fac5f1a2c1bf7e82d812)
for installation scope and the
[telemetry opt-out](https://www.skills.sh/docs/cli#telemetry).

### Report a first attempt or a real task

Use the [first-run feedback form](https://github.com/kimjooyoon/gooo-go-ports/issues/new?template=problem.yml)
and select **Coding-agent first-program skill**. A first setup error is enough;
you do not need a completed run or all tool versions to report it. Name your
agent, the step reached, the command or source edit, and the actual result or
first diagnostic when available. Share public inputs and source only.

For the supplied task, score `75` should change from `sensor: high` to
`sensor: low` after the threshold edit, while its other three fields stay the
same. Save a fresh execution and replay for the edited source. Tell us whether
this helped with a real task, and what input, output or automation is missing.
We use that report to separate setup and workflow problems from a reproducible
compiler issue before passing a minimal case to the language repository.

## Choose a task that matches the current capability

- **Prepare an API component without losing the original:** use the
  [URL context facts](../examples/url-context-facts/README.md) workspace with
  `{"text":"a+b /%20"}`. Run the strict input adapter, execute the ten-field
  Gooo Report, and save a replay. Compare `path_segment` with `query_value` and
  select the field matching the endpoint's declared component type. The
  independent Go verifier accepts an optional `--inputs` document, so check your
  own admitted inputs as well as the checked-in finite corpus. Text is limited
  to 341 UTF-8 bytes; the report neither parses a URL nor grants permission to
  send a request. Report the first failed command or whether these facts helped
  with an actual API client task.

- **Write a Gooo program:** use the first-program skill, then the existing
  [local edit-and-replay guide](../examples/learn-gooo/README.md#make-one-visible-change).
- **Edit an API policy rule:** use the [editable source comparison](../examples/editable-api-policy/README.md).
  To extract a same-file helper, follow its explicit `source-inspection036`
  compiler profile. Keep the default839 path for the original demo. The selected
  compiler prepares each current call plan before execution; every native
  execution and saved replay must contain that same full program. This checks
  source/result binding for the supplied facts, not live authorization or full
  field provenance. Omit `--check-demo` only when deliberately changing behavior.
  Keep the supplied inputs fixed, edit `after.gooo`, and run the comparison into
  a new output directory. Inspect changed fields and replay each source's own
  execution. The initial three synthetic requests demonstrate a write rule
  that keeps access for a deploy bot. Report a setup failure or the real task
  this helps with through the feedback form above.
- **Reproduce an AutoBot action-line decision:** use the
  [bounded Python rule adaptation](../examples/autobot-resolver/README.md).
  Validate supplied nine-scalar rows with the input converter, then execute and
  replay the Gooo source. Inspect the eight fields, including stored/floor
  presence and the exact reason. This does not query a live policy store or
  approve/apply an action; the Go oracle is for verification only.
- **Review a proposed dependency update:** follow the
  [single-request version guide](../examples/version-eligibility/README.md#try-your-own-current-candidate-and-minimum).
  `minimum` requires valid versions, a candidate newer than current and at least
  the minimum; read `eligible` with `reason`. `stable` additionally requires the
  same major and no prerelease; read `policy_eligible` with `policy_reason`.
  For current `v1.9.0`, candidate `v2.0.0-rc.1` and minimum `v1.0.0`, the baseline
  returns `eligible=true`, while `stable` returns `policy_eligible=false` with
  `policy_reason="different_major"`. These decisions do not apply an update.
  Tell us whether a successful run helps your real task, or share the first
  failure, through the [existing feedback form](https://github.com/kimjooyoon/gooo-go-ports/issues/new?template=problem.yml).
  Select **Native setup, verification, or replay** and use public synthetic inputs.
- **Edit a patch-line update rule with newer Gooo:** the separate
  [same-minor source example](https://gist.github.com/kimjooyoon/9ca55091bd18ddb6f06a563e99f68348#file-00-readme-md)
  uses released **0.6.36-dev**, compiler `31497af3` and Go 1.27.2. Follow its
  isolated setup; this checkout still uses compiler `83928fa`. The program
  adapts Go `semver.MajorMinor` to a result record whose `value` field explicitly
  permits 1,026 UTF-8 bytes, preserving the longest supported shorthand input.
  An importing Gooo rule requires a newer stable candidate in the same
  major.minor line and at least the supplied minimum. `v0.1.0` to `v0.2.0`
  returns `different_minor_line`; `v0.1.0` to `v0.1.1` can be eligible.
  The source, original Go oracle and 27 finite cases are public. Read
  `policy_eligible` with `policy_reason`; same-line eligibility does not establish
  compatibility or authorize an update. Retained author-run receipt hashes are
  documented, and the commands let you produce your own execution and replay.
- **Port a Go function:** use the [contribution contract](../CONTRIBUTING.md)
  and [porting instructions](../AGENTS.md). Preserve the original API contract,
  full input domain, upstream license and independent Go reference. Unsupported
  operations need an explicit finding; a shortened domain or handwritten Go
  fallback does not establish a Gooo port.
- **Explain an API policy change in CI:** use the
  [API policy Action](../examples/api-policy-ci/README.md). Its two sealed
  original/read-only profiles compare nine supplied facts. Optional `summary`
  displays fixed reason labels and changed-field names while preserving a
  requested failure. The Action cannot load arbitrary policies or authorize
  live API effects.
- **Compare API policies without a checkout:** use the [Go CLI quickstart](https://gist.github.com/kimjooyoon/a46e67c4830fdfb67a9b75e45aaceee8).
  With Go **1.27.2** and initial network downloads, build a pinned CLI for two fixed
  profiles, then compare and replay supplied facts. Its local trial used macOS arm64.
  Report the first failed command, exit status and diagnostic in the [feedback form](https://github.com/kimjooyoon/gooo-go-ports/issues/new?template=problem.yml); choose **Native setup, verification, or replay**.
- **Read a policy-comparison failure from an agent:** the repository's
  [comparison CLI](../examples/api-policy-revision/README.md#read-failures-from-an-ai-agent-or-automation)
  accepts `--diagnostics=json` before other options. Read its versioned error
  code and stage from stderr, keeping the process exit and stdout together.
  Normal policy denials remain successful domain results; diagnostic output
  does not authorize a retry or a live API effect.
- **Inspect an API ETag before a conditional write:** use the
  [strong-validator workflow below](#inspect-etag-and-header-facts). It preserves
  raw input and separates parsed-tag strength, trailing content and raw header
  validity. It does not implement the write or full `If-Match` handling.
- **Inspect cooldown facts before reassessing an API operation:** use the
  [Retry-After facts prototype](../examples/retry-after-facts/README.md). Its
  single verifier command runs the strict starter gate,113 original inputs,
  fresh Go expectations, native execution, saved replay and eleven mutation
  controls. Preserve the original six observations and all fourteen result
  fields. `READY_TO_REASSESS` does not grant a retry or refresh supplied facts;
  current request binding, guidance and a host-owned atomic claim remain separate.

  For one public-input trial, give your agent this task:

  > Reuse a `kimjooyoon/gooo-go-ports` checkout matching
  > `2b2d972f40057b1c9339c89afcb8b3be161a60c9`, or use a separate task directory
  > at that revision, with its pinned compiler `83928fa` and Go **1.27.2**.
  > Copy `examples/retry-after-facts/request.json` into a fresh output directory
  > and change only `elapsed_seconds` from `59` to `60`. Keep the repository
  > source and 113-input corpus unchanged. Follow the README's one-request strict
  > gate, native execution and saved replay, using fresh paths and the same input.
  > Expected at this boundary: `state="READY_TO_REASSESS"`,
  > `reason="COOLDOWN_ELAPSED"`, `remaining_seconds=0`. Report what actually ran.
  > Return the six public input values and all fourteen actual fields at
  > `result.runtime.traces[0].deliveries[0].actual`, or the first failed command,
  > exit status and diagnostic. Include the actual checkout revision,
  > compiler/Go identity and agent/OS when available, plus one real API or
  > automation task this helps with—or say that it does not help.
  > A setup failure or missing version detail is still useful feedback.

  If you want to share feedback, put that short report in the
  [existing feedback form](https://github.com/kimjooyoon/gooo-go-ports/issues/new?template=problem.yml),
  choosing **Native setup, verification, or replay**. Use the supplied synthetic
  input; keep raw receipts and local paths private. Review any diagnostic before
  sharing, and upload or attach nothing automatically. These facts do not send
  a request, start a timer or authorize a retry.

  If you use the Tessl skill published from `146e07c`, it directs new workspaces
  to the older `e1e279a` tutorial snapshot. This checkout task uses the separate
  revision above; that skill version alone does not supply this Retry-After example.
- **Evaluate a selected If-Match condition:** use the [bounded compatibility core](../ports/net/http/README.md#selected-if-match-compatibility-core), `if-match.workspace.json` and its strict `cmd/if-match-input` handoff. Preserve both original Texts; `condition`0/1/2 means none/true/false. Header selection, authenticated current facts and applying a write are separate.
- **Edit and regression-check an isolated ETag project:** [export the current
  three-source project](../templates/etag-project/README.md), edit its Gooo
  `computes` bodies under the fixed public ABI, then compare247 original inputs
  with fresh Go expectations in seven batches. Exporting is a source copy;
  the separate checker verifies actual Gooo results and an optional19-row CRLF
  mutation witness. Saved-receipt replay is a separate workflow.
- **Assess a service-grant change from supplied JSON facts:** use the
  [grant-update stdin CLI](../examples/agentmesh-service-policy/README.md#use-the-generated-ordinary-go-library-and-stdin-cli).
  Supply `before`, `after`, `required_type` and `required_name`. Inspect all seven
  Gooo decision fields and exact input/source bindings. Valid policy refusals and
  `INVALID_INPUT` domain outcomes exit successfully; malformed JSON structure
  refuses the whole batch without stdout. This pure check does not apply a grant
  or call a service.
- **Preserve an ordered workflow requirement list:** use the
  [multi-request Gooo composition](../examples/service-grant-many/README.md).
  Supply `before`, `after` and ordered `required_requests` pairs. Inspect the first
  failing original index and all 12 decision fields; preserve duplicates and input
  order. The generated Go API and JSON CLI need Go 1.27.2, while source editing
  uses the separate pinned `de99b770` compiler profile. This is a separate API;
  the MCP tool below still assesses one required request.
- **Discover the same assessment through MCP:** build the
  [local service-grant MCP server](../examples/service-grant-mcp/README.md)
  and configure your host to launch its absolute executable path over stdio.
  The `assess_service_grant` tool takes the four facts as one arguments object
  and returns the complete Gooo proof. `ELIGIBLE`, `REFUSED` and `INVALID_INPUT`
  are policy outcomes; `isError` means no decision was returned. The independent
  wire verifier compares actual native CLI fixtures. See the
  [host setup and finite Codex observation](#connect-the-service-grant-mcp-tool);
  task completion in other hosts remains unverified.
- **Automatically check a versioned service-grant tightening:** use the
  [versioned grant-change example](../examples/versioned-grant-change/README.md).
  Gooo combines strict version matching with supported grant narrowing and
  required-request preservation. Its verifier compares 48 fixed cases with fresh
  component references and saved replay. An ELIGIBLE result is not an APPLIED
  write; current authority and atomic application remain host responsibilities.
- **Import current ports into a separate Gooo project:** follow the
  [current path-import walkthrough](use-current-path-in-your-project.md) with
  its pinned toolchain, source mapping and saved execution/replay.

## Connect the service-grant MCP tool

Download the [native MCP preview.13](https://github.com/kimjooyoon/gooo-go-ports/releases/tag/v0.1.0-preview.13)
archive and matching `.sha256` file for **Linux amd64** or **macOS arm64**.
In the download directory, verify the checksum with `sha256sum -c FILE.sha256`
(Linux) or `shasum -a 256 -c FILE.sha256` (macOS), then extract the verified
archive. It contains `gooo-service-grant-mcp` and 55 license/attribution texts.
Running this executable needs no Go, Gooo compiler or Python installation.
Replace `/ABSOLUTE/PATH/gooo-service-grant-mcp` below with its absolute path;
pass no command arguments.

To build from source instead, use **Go 1.27.2** and the
[MCP module's build command](../examples/service-grant-mcp/README.md#build-and-connect).
Initial source setup downloads pinned modules. The preview.12
`gooo-service-grant-evaluate` CLI is **not an MCP server**.
The local stdio server opens no HTTP port, needs no API key and makes no model
call; the host retains its own authentication and permission settings.
Decisions come from the pinned [Gooo](https://github.com/kimjooyoon/meta-ontology-go)
generation without running the language engine.

For [Codex MCP configuration](https://learn.chatgpt.com/docs/extend/mcp?surface=cli),
merge this server entry using the client's documented configuration scope:

```toml
[mcp_servers.gooo_service_grant]
command = "/ABSOLUTE/PATH/gooo-service-grant-mcp"
enabled_tools = ["assess_service_grant"]
```

For [Claude Code local stdio](https://code.claude.com/docs/en/mcp), register it
in the project-local scope:

```sh
claude mcp add --transport stdio --scope local gooo-service-grant -- /ABSOLUTE/PATH/gooo-service-grant-mcp
```

For [Cursor](https://cursor.com/docs/mcp), merge this entry into the project's
`.cursor/mcp.json`, preserving existing entries:

```json
{
  "mcpServers": {
    "gooo-service-grant": {
      "type": "stdio",
      "command": "/ABSOLUTE/PATH/gooo-service-grant-mcp"
    }
  }
}
```

The [Cursor CLI](https://cursor.com/docs/cli/mcp) uses the same configuration;
`agent mcp list-tools gooo-service-grant` inspects the configured server's tools.

Then ask the host to perform one finite task:

> Call `assess_service_grant` with
> `{"before":"mcp://weather.*","after":"mcp://weather.forecast","required_type":"mcp","required_name":"weather.forecast"}`,
> then call it once more with only `after` changed to `mcp://weather.health`.
> Report both decisions and retain the complete tool results, including their
> supplied facts and source bindings.

These are tool argument **objects**, unlike the standalone CLI's JSON array.
The represented statuses are `ELIGIBLE` and `REFUSED`; neither applies a grant.
Authentication, current facts and atomic application remain
[host responsibilities](../examples/agentmesh-service-policy/conditional-apply.md).

One maintainer-owned macOS arm64 Codex CLI `0.163.0-alpha.10.2` session used the
preview.13 executable downloaded from the public release and verified against
its SHA-256. It connected directly, without a Python stdio recorder, and
completed these two calls. Both copies of each tool result and the two complete
final proofs matched all four facts, seven typed decision fields and 89 asset
bindings, including false/empty fields. The test explicitly asked for complete
final proofs; the shorter reporting prompt above avoids requiring another copy
of every source hash. This is a synthetic maintainer observation, not external
adoption or a registry installation. Claude Code and Cursor connection examples
follow their official documentation; task completion in those hosts remains
unverified. Other client capabilities and independent native-child termination
were not proven by the host output.

For useful [first-run feedback](https://github.com/kimjooyoon/gooo-go-ports/issues/new?template=problem.yml),
include the host/version, OS and server source version. Say whether registration,
tool discovery, the call or returning the complete result failed. Preserve the
first command, exit status and diagnostic, and use public synthetic facts.
Did it help one real API or automation task, or which service pattern could you
not express? The command contacts no service and uploads no feedback itself.

## Optional Claude Code ETag workflow

Use the experimental `gooo-porting` adapter for requested Gooo ETag body edits
checked against Go. The [Gooo language and compiler](https://github.com/kimjooyoon/meta-ontology-go)
are developed in `meta-ontology-go`.

In a Claude Code session (2.1.275 or later), explicitly add this GitHub
marketplace and install the adapter:

```text
/plugin install gooo-porting --marketplace kimjooyoon/gooo-go-ports
```

Review the source and choose an installation scope in the
[official install flow](https://code.claude.com/docs/en/plugins/install#add-a-marketplace-and-install-in-one-command).

On macOS or Linux, prepare Git, Python 3, a POSIX shell, Go **1.27.2**, and a
clean Gooo compiler at `83928fa4538245abe33b08549da47f8cbdc50ed2`.
Reuse matching tools and prepare the pinned Go modules once using the
[project setup guide](../templates/etag-project/README.md#prepare-and-run).
Keep setup and exported projects outside the installed plugin bundle.

Then request a concrete edit with the
[gooo-etag-project skill](../.agents/skills/gooo-etag-project/SKILL.md), for example:

> Use `gooo-etag-project`. Export to a new directory outside the plugin bundle,
> run the baseline, then reorder the Report's `single_strong` boolean checks
> without changing their behavior. Compare the edit with Go, preserve my
> existing files, and show the source changes, actual results and new evidence.

Claude Code 2.1.301 checked the marketplace and plugin manifests; real Claude
installation, skill loading and activation have not yet been verified.

## Inspect ETag and header facts

For an agent task such as “inspect this API's ETag or `If-Match` data in Gooo,”
start with [strong-validator facts](../examples/strong-validator-facts/README.md),
its [Gooo source](../examples/strong-validator-facts/main.gooo),
[inputs](../examples/strong-validator-facts/inputs.json) and root
`strong-validator-facts.workspace.json`. This original thirteen-field composition
imports the bounded Go ETag helpers and header-value predicate. Keep its three
source packages together; it does not need the default workspace.

From the repository root, select the pinned toolchain and inspect an existing
compiler before reuse:

```sh
export GOTOOLCHAIN=go1.27.2
export GOWORK=off
go version
./.gooo version --build --json
```

Require compiler revision `83928fa4538245abe33b08549da47f8cbdc50ed2`, Go
`go1.27.2` and build identity `vcs_modified: "false"`, as pinned in
`upstream.json`. If this fresh checkout has no matching compiler, run
`./tools/bootstrap.sh` and inspect the build identity again. Bootstrap downloads
and builds the pinned compiler and may download Go; reuse a matching installation
when available. Preserve an existing unmatched compiler and its receipts by
following [bootstrap recovery](../README.md#bootstrap-recovery).

Run these commands in order, stopping at the first failure. A new directory
preserves earlier records. Pass the same selected Go executable explicitly to
execution and replay:

```sh
set -eu
mkdir -p out
evidence_dir="$(mktemp -d "$PWD/out/strong-validator-facts.XXXXXX")"
go_executable="$(go env GOROOT)/bin/go"
./.gooo package execute --json --go "$go_executable" \
  --inputs examples/strong-validator-facts/inputs.json \
  strong-validator-facts.workspace.json > "$evidence_dir/execution.json"
./.gooo package replay --json --go "$go_executable" \
  --receipt "$evidence_dir/execution.json" \
  --inputs examples/strong-validator-facts/inputs.json \
  strong-validator-facts.workspace.json > "$evidence_dir/replay.json"
```

Inspect `result.runtime.traces[].deliveries[].actual` in both records. These
four supplied input-only runs report `OBSERVED`; they are separate from the
repository's independent Go-reference corpus checks. Expected distinguishing
fields are:

| Raw input as a JSON string | `header_valid` | `single_strong` | `reason` |
| --- | --- | --- | --- |
| `"\"item-v1\""` | true | true | `SINGLE_STRONG_TAG` |
| `"W/\"item-v1\""` | true | false | `WEAK_TAG` |
| `"\"item-v1\", \"other\""` | true | false | `TRAILING_CONTENT` |
| `"\r\n\"item-v1\""` | false | false | `HEADER_VALUE_INVALID` |

Retain the original `raw`, parsed `tag` and `remainder` alongside the flags and
UTF-8 byte lengths. The CRLF example can have a complete parsed tag while its
raw header is invalid. The low-level `ETagStrongMatch` is raw equality plus an
opening-quote check; it does not validate complete ETag grammar.

Each argument must already be decoded Unicode-scalar, valid UTF-8 Text of
0–1,024 bytes. Empty strings, NUL, controls and invalid ETag grammar remain in
the supported domain. Callers own transport validity and this byte bound; do
not assume the CLI rejects every duplicate key, malformed UTF-8 sequence or
unpaired surrogate. Do not trim, repair or truncate an external value to make
it fit. No per-call human approval is required to compute these facts. They do
not establish current-resource matching, authorization, full `If-Match`
list/wildcard handling or an atomic write.

Report the compiler identity, source change, actual result, saved record paths
and any unsupported behavior. An agent's own trial is tooling verification;
independent adoption requires a separate user's actual use.
