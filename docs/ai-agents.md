# Use Gooo with a coding agent

Start with a concrete task: compare saved MCP tool definitions, assess a service
grant change, or edit and execute a Gooo program. For saved before/after pages,
use the [own-input route](#compare-your-own-saved-pages) or the repository-local
[snapshot skill](../.agents/skills/gooo-compare-tool-snapshots/SKILL.md).
The [Gooo language and compiler](https://github.com/kimjooyoon/meta-ontology-go)
are developed separately; this repository supplies ports and runnable examples.

## Select the task before setup

| Requested work | Entry point | Result and boundary |
| --- | --- | --- |
| Assess a grant change for one required service | [Single-request MCP tool](#connect-the-service-grant-mcp-tool): `assess_service_grant` | One supplied type/name pair; seven decision fields and full bindings. |
| Preserve all required services while narrowing a grant | [Ordered-request MCP tool](#preserve-several-required-requests-through-mcp): start with `--many-required`, then call `assess_service_grant_change` | An ordered list, normally 1-256 pairs; twelve decision fields, including the first original counterexample index. Do not replace it with the single-request tool. |
| Check whether a previously selected tool alias changed origin | [Source-built alias MCP tool](#compare-a-tool-alias-before-reusing-it): start with `--alias-origin-change`, then call `gooo_compare_alias_origin` | Compare supplied catalogs and retain the original row indexes. Requires this source build; unavailable in preview16. |
| Compare the supplied API policy profiles | [Fixed-policy command](#compare-fixed-api-policies-without-a-compiler) | Compares the documented profiles; does not load an arbitrary edited policy. |
| Compare saved MCP tool catalogs | [Native CLI without a toolchain](recorded-catalog-cli.md) or [pinned source alternatives](#compare-saved-mcp-catalogs-without-a-checkout) | Separates alias-origin changes from unknown collection context. The native command includes explicit observation availability and missing-field diagnostics; it does not collect a live catalog or authorize a tool call. |
| Compare saved tool definitions when no host alias record exists | [Native preview.2 installation](#install-the-native-snapshot-command), [own-input recipe](#compare-your-own-saved-pages) or [agent task skill](../.agents/skills/gooo-compare-tool-snapshots/SKILL.md): `--definition-snapshot` | Select an original tool name from retained JSON-RPC pages. Preserve incomplete coverage and duplicate-name results; do not invent a host mapping. Native preview.2 includes this mode and three synthetic examples; the source route remains available. Preview.1 does not support it. |
| Edit and execute Gooo source | [First-program skill](#run-the-first-program-skill) | An actual source edit plus new execution and replay records; declaration checking alone is insufficient. |

The native MCP routes need no compiler at runtime. The optional
`--many-required` mode adds the list tool while retaining the single-request
tool; a host may separately filter its enabled tools. Use discovery to check
what the connected executable exposes.

The [service-pattern domain](../examples/agentmesh-service-policy/README.md#exact-represented-domain)
uses bounded ASCII service identifiers, rather than URL paths or individual MCP
tool names. Unsupported patterns must remain explicit failures or represented
`INVALID_INPUT`; do not repair inputs to obtain an eligible result.
`ELIGIBLE` and `REFUSED` are assessment outcomes, while an MCP error or absent
tool is a setup, input, transport or execution failure. Preserve the complete facts, typed decision
and source bindings. These calculations require no per-request human approval;
they neither authenticate supplied facts nor apply a live change. Report the
[actual first attempt](#report-a-first-attempt-or-a-real-task), including a
missing capability, rather than treating installation as task completion.

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
This immutable installation recipe retains its Go 1.27.2 profile; installation
uses the public Go proxy and checksum service. Current-source builds below target Go 1.27.3.
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

Current-source setup uses Go **1.27.3** with the checkout's matching compiler
and validator identities. The [full main CI at `b0e6715`](https://github.com/kimjooyoon/gooo-go-ports/actions/runs/39544747815)
passed native comparison and saved replay with compiler `205896a`; follow the
setup below for your own platform and preserve the actual result or first error.
The [pinned 0.6.32 release](https://github.com/kimjooyoon/meta-ontology-go/releases/tag/v0.6.32-dev)
and the [pre-migration source snapshot](https://github.com/kimjooyoon/gooo-go-ports/tree/cc946350de37c48d509b1d383b8ce7e8a0062483)
retain Go **1.27.2**. For that reproduction only, verify SHA256SUMS, extract fresh
and use that snapshot's identity validator before skipping bootstrap. Keep
existing files and receipts; the old archive is not a Go 1.27.3 compiler build.

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

### Read source identities after a policy edit

The [editable API policy workflow](../examples/editable-api-policy/README.md)
creates `source-comparison.json` after validating the execution and replay
records. Inspect the complete field path before comparing hashes:

| Document and field | Bytes identified |
| --- | --- |
| `source-comparison.json`: `variants.before.policy_source.raw_sha256` (or `after`) | Original policy file bytes, including its line endings. This field is unprefixed hex. |
| `source-comparison.json`: `variants.before.policy_source.receipt_json_string_digest` (or `after`) | SHA-256 of the source string encoded with Go JSON rules, with a `sha256:` prefix. This is the value checked against the matching package/source entry in the receipt. |
| Execution or replay receipt: `result.program.workspace_image.packages[].sources[].source_digest` | The Go JSON-encoded source string for that entry's package and filename; use both to select the entry. |
| Execution or replay receipt: `manifest_digest` | Exact workspace manifest JSON file bytes; not a reserialized object. |
| Execution or replay receipt: `inputs_digest` | Exact input JSON file bytes; not the source-string encoding or a reserialized object. |
| Replay receipt: `replayed_from_sha256` | Exact saved execution receipt bytes, including whitespace; not policy source bytes. |

Two digests of the same source can differ because they identify different byte
representations. Removing a `sha256:` prefix does not make those representations
equivalent. Keep the original source and receipt bytes; use the workflow's
existing checks rather than reserializing a receipt or normalizing source before
hashing. A matching hash identifies bytes, not the trustworthiness of supplied
facts or authority to apply a live change.

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
[pinned skill](https://github.com/kimjooyoon/gooo-go-ports/tree/fceb89f8b53eabd6687d16258d7cee89853c2cfe/.agents/skills/gooo-first-program)
and use a fresh project directory with no existing copy of this skill. Prepare
Node.js **22.20 or later**, npm and Git, then run from that directory:

```sh
DISABLE_TELEMETRY=1 npm exec --yes --ignore-scripts \
  --package=skills@1.7.4 -- \
  skills add https://github.com/kimjooyoon/gooo-go-ports/tree/fceb89f8b53eabd6687d16258d7cee89853c2cfe/.agents/skills/gooo-first-program \
  --skill gooo-first-program --agent codex --agent claude-code --agent cursor --copy --yes
```

This selects project-local copies. Codex and Cursor share
`.agents/skills/gooo-first-program`; Claude Code uses
`.claude/skills/gooo-first-program`. Remove agent selections you do not need.
Review those files and the source revision in `skills-lock.json`, then use the
explicit first-program task above. Installing instructions does not install
the compiler or execute a Gooo program.

A macOS trial used Node **24.19.0**, a private npm installation of `skills`
**1.7.4** checked against its published integrity value, and the exact source URL
and CLI arguments above. Both installed directories matched all three source
files byte-for-byte; the lock's source revision and folder hash were checked
independently. The command above obtains the CLI from npm rather than using the
private installation used in the trial. A separate prompt-only `skills use`
trial returned the exact instructions and supporting files without launching a
client. Client activation, task completion and independent adoption remain
unverified. See the [official CLI](https://github.com/vercel-labs/skills)
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
  To extract a same-file helper, follow its explicit `source-inspection037`
  compiler profile. Keep the `default` profile (compiler `205896a`) for the
  original demo. The selected
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
- **Edit a patch-line update rule in a separate checkout:** the separate
  [same-minor source example](https://gist.github.com/kimjooyoon/9ca55091bd18ddb6f06a563e99f68348#file-00-readme-md)
  preserves the released **0.6.36-dev**, compiler `31497af3` and Go 1.27.2 profile. Follow its
  isolated setup; this checkout uses compiler `205896a`. The program
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
  This frozen Go **1.27.2** recipe uses initial network downloads to build a pinned CLI for two fixed
  profiles, then compare and replay supplied facts. Its local trial used macOS arm64.
  Report the first failed command, exit status and diagnostic in the [feedback form](https://github.com/kimjooyoon/gooo-go-ports/issues/new?template=problem.yml); choose **Native setup, verification, or replay**.
- **Read a policy-comparison failure from an agent:** the repository's
  [comparison CLI](../examples/api-policy-revision/README.md#read-failures-from-an-ai-agent-or-automation)
  accepts `--diagnostics=json` before other options. Read its versioned error
  code and stage from stderr, keeping the process exit and stdout together.
  Normal policy denials remain successful domain results; diagnostic output
  does not authorize a retry or a live API effect.
- **Locate a source error through optional upstream ADK:** use an already configured
  [bridge at `b875196c`](https://github.com/kimjooyoon/meta-ontology-go/blob/b875196c4e3b2c36402a91346c41e320f877eed0/integrations/adk-go/README.md).
  This guidance is based on upstream source and parity-test review, with no new
  ADK or agent execution. This upstream adapter profile is separate from this
  checkout's pinned compiler; follow the linked adapter setup for its tools.
  Keep the original tool result and `exit_code`, then decode `report_json`:
  diagnostics use `schema_version: "gooo/diagnostics/v1"`, `status` and `diagnostics`;
  successful discovery uses `schema: "gooo/capability-discovery-report/v1"`.
  Its `PROGRESS` or `UNKNOWN` evidence does not establish implementation success.
  Use a source location supplied by `gooo_discover_capability` when available.
  If it reports `SOURCE_LOWER_FAILED` without a useful location, call
  `gooo_check_source` at most once with the same source-root-relative `path`,
  unchanged source, fixed binary and root. Its `semantic.lowering` diagnostic may
  identify the unresolved reference. If it supplies no location, report that
  location as unavailable. Preserve both results and the original failure even
  if the check succeeds. This adds diagnostic evidence without editing the input
  or retrying discovery; it does not execute the program or grant authority.
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

  For reproduction of the frozen Go 1.27.2 public-input trial, give your agent this task:

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
  order. The generated Go API and JSON CLI need Go 1.27.3, while source editing
  uses the current pinned `205896a4` compiler profile. The module README retains
  the earlier `de99b770` reproduction separately. To call this assessment
  from an MCP host, use the explicit
  [ordered-request mode](#preserve-several-required-requests-through-mcp).
  The default MCP tool still assesses one required request.
- **Choose among several supplied grant-change proposals:** use the
  [source-owned Gooo selector](../examples/grant-candidate-selection/README.md).
  Pass one `before` grant, the ordered `required_requests`, and candidate IDs,
  `after` grants and nonnegative preferences. The native API evaluates every
  proposal internally; callers cannot assert assessment status or verification.
  Gooo selects by supplied preference among eligible proposals and retains all
  original proofs. `PROPOSED` is a proposal and `execution_authorized` stays
  false. See the module's reproduction command and failure states. This checkout
  module is not a tool in the released MCP bundles.
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

## Record inputs for tool-origin comparisons

The [alias-origin module](../examples/alias-origin-change/README.md) compares
supplied rows; it does not collect a live catalog. Keep the collector's saved
records alongside the comparison so another agent can reconstruct its inputs.

| Input field | Where to obtain it |
| --- | --- |
| `server` | A stable, host-owned key for the connection and its meaningful backend binding revision. |
| `original_name` | The original tool name in the saved server descriptor. |
| `exposed_name` | The name actually presented to the model after the host's alias policy. |

As a collection recommendation, scope the `server` key to the host, connection
and backend binding. Use an unambiguous encoding or opaque ID; keep the full
configuration revision separately so unrelated settings do not change origin
identity. Preserve names and row order. Do not substitute the server's claimed
`serverInfo.name`: [MCP scopes tool-name uniqueness to one server and does not guarantee unique server names](https://modelcontextprotocol.io/specification/2026-07-28/server/tools#tool-names).

Record each page's original bytes or digest, cursor relationship, capture time,
protocol and a non-secret authorization-context identifier. The
[tool list can depend on authorization and supports pagination](https://modelcontextprotocol.io/specification/2026-07-28/server/tools#listing-tools).
Retain both page completion and whether the relevant configuration stayed
stable. Reaching the final cursor alone does not establish an atomic snapshot
of a changing remote catalog. If pages, context or consistency are unknown,
keep that uncertainty alongside the original comparison; a missing supplied
row is not proof that the live tool was removed.

For a [PROV-O](https://www.w3.org/TR/prov-o/#description-starting-point-terms)
record, model collection and comparison as activities using their actual saved
inputs, with their outputs generated by those activities. A later observation
is not derived from an earlier one merely because it came later. Compiler
source provenance does not establish that an external collection occurred.
This guidance adds no live collector, authentication or execution authority.

## Compare saved definitions without host alias records

Use this route when you retained `tools/list` request/response JSON but did not
save the host's exposed aliases. Follow the
[saved-definition input format](../examples/tool-definition-snapshot/README.md#try-the-separate-saved-page-input).
It needs `target_name` and before/after page arrays containing exact
`request_json` and `response_json` strings. Keep any HTTP/SSE carrier and its
extraction record separately; the command does not extract or collect them.

Ask your coding agent:

> Use the native preview.2 or this checkout's `--definition-snapshot` command to compare the original
> tool name in my two saved page sets. Preserve the original bytes and report
> the selected definition and annotation relations, page coverage, duplicate
> names and source bindings. Do not fill in missing host facts. Keep the full
> report locally and show the first error if the command fails.

### Install the native snapshot command

[recorded-catalog-cli-preview.2](https://github.com/kimjooyoon/gooo-go-ports/releases/tag/recorded-catalog-cli-preview.2)
contains `--definition-snapshot` and the three synthetic upgrade examples for
Apple Silicon macOS and Linux x86-64. Use this fixed version; GitHub's
`releases/latest` endpoint does not select prereleases. Download and verify the
archive, then keep `START-HERE.txt` and `manifest.json` with your reports:

```sh
case "$(uname -s)-$(uname -m)" in
  Linux-x86_64) platform=linux-amd64 ;;
  Darwin-arm64) platform=darwin-arm64 ;;
  *) printf '%s\n' 'No archive is provided for this platform.' >&2; exit 1 ;;
esac
release=https://github.com/kimjooyoon/gooo-go-ports/releases/download/recorded-catalog-cli-preview.2
asset="gooo-recorded-catalog-$platform.tar.gz"
native_dir="$(mktemp -d)" && cd "$native_dir" || exit 1
curl --fail --location --proto '=https' --tlsv1.2 --output "$asset" "$release/$asset" || exit 1
curl --fail --location --proto '=https' --tlsv1.2 --output "$asset.sha256" "$release/$asset.sha256" || exit 1
case "$platform" in
  linux-amd64) sha256sum -c "$asset.sha256" || exit 1 ;;
  darwin-arm64) shasum -a 256 -c "$asset.sha256" || exit 1 ;;
esac
tar -xzf "$asset" || exit 1
snapshot_cli="$PWD/gooo-recorded-catalog"
"$snapshot_cli" --version
```

Read the extracted `START-HERE.txt` for all three example commands. Comparisons
run locally without an account, model call or Go/Gooo installation. Downloads
need network access; the own-input assembler below uses Python 3. The archive
compares stored observations; it does not collect live pages or call tools.
To change the Gooo rules themselves, use the [source-edit workflow](#run-the-first-program-skill)
and preserve new execution and replay evidence. Running the archive does not edit Gooo.

### Compare your own saved pages

Choose the execution route before running the recipe. The source route uses an
existing checkout and Go **1.27.3**; its generated evaluator is already included,
so no Gooo compiler bootstrap is needed. For the [native preview.2 route](#install-the-native-snapshot-command),
replace `go -C examples/recorded-catalog-comparison run ./cmd/compare` below with
`"$snapshot_cli"` from the installation block, or the absolute path to that
executable in a new shell. Preview.1 does not support `--definition-snapshot`.

Replace the original tool name and four absolute paths with your retained
JSON-RPC `tools/list` payloads. Both routes use Python 3 to assemble the input
without parsing and rewriting the original page text, including CRLF line endings.

```sh
snapshot_dir="$(mktemp -d "${TMPDIR:-/tmp}/gooo-snapshot.XXXXXX")" || exit 1
if python3 - 'original.tool.name' \
  '/absolute/before-request.json' '/absolute/before-response.json' \
  '/absolute/after-request.json' '/absolute/after-response.json' \
  > "$snapshot_dir/input.json" 2> "$snapshot_dir/assembly.stderr" <<'PY'
import json
from pathlib import Path
import sys

def page(request, response):
    return {"request_json": Path(request).read_bytes().decode("utf-8"),
            "response_json": Path(response).read_bytes().decode("utf-8")}

target, before_request, before_response, after_request, after_response = sys.argv[1:]
json.dump({"target_name": target,
           "before": {"pages": [page(before_request, before_response)]},
           "after": {"pages": [page(after_request, after_response)]}}, sys.stdout)
PY
then
  assembly_status=0
else
  assembly_status=$?
fi
printf '%s\n' "$assembly_status" > "$snapshot_dir/assembly.exit"
if [ "$assembly_status" -ne 0 ]; then
  printf 'Input assembly failed; files retained at: %s\n' "$snapshot_dir" >&2
  cat "$snapshot_dir/assembly.stderr" >&2
  exit "$assembly_status"
fi
if GOTOOLCHAIN=local GOWORK=off \
  go -C examples/recorded-catalog-comparison run ./cmd/compare --definition-snapshot \
  < "$snapshot_dir/input.json" > "$snapshot_dir/report.json" 2> "$snapshot_dir/stderr"; then
  snapshot_status=0
else
  snapshot_status=$?
fi
printf '%s\n' "$snapshot_status" > "$snapshot_dir/exit-status"
printf 'Saved input, report and command status: %s\n' "$snapshot_dir"
if [ "$snapshot_status" -ne 0 ]; then
  cat "$snapshot_dir/stderr" >&2
  exit "$snapshot_status"
fi
```

Run the source route from the repository root; the native route needs no checkout.
If the command fails, retain its stderr and exit
status; no comparison result was established. On exit zero, read `decision.state`,
`decision.reason`, `decision.next_action`, the definition/annotation relations and both page
coverage fields. `UNASSESSED` is a returned comparison with insufficient facts,
not a command failure or permission to proceed.

This recipe retains one page per side. If you saved more pages, append their
original request/response pairs to each `pages` array in observed order. Keep a
retained `nextCursor` even when its next page is missing: absence from an
incomplete page set cannot establish that a tool was removed. The command
collects no new pages and invents no aliases or host metadata.

A useful first-use result is an answer to your own comparison question, a missing
observation, or the first setup failure. In the [existing feedback form](https://github.com/kimjooyoon/gooo-go-ports/issues/new?template=problem.yml),
choose **Native setup, verification, or replay** and describe the command/source,
your own versus synthetic input, result or first error, and what decision remains.
Sharing the original files is optional; keep private catalog contents local.

The [synthetic example](../examples/tool-definition-snapshot/example-input.json)
changes a schema limit from 10 to 20. The [feature's main CI](https://github.com/kimjooyoon/gooo-go-ports/actions/runs/40235109209)
passed the finite native/replay corpus and API tests at `9bbe157`; new revisions
retain their own verification. The earlier preview.1 archive and the immutable
Go installation recipes below do not contain this mode. `SAVED_ROWS_ONLY` compares the supplied rows; it does not establish
unchanged server behavior or authorize calling a tool. Origin and authorization
remain `NOT_ASSESSED`.

For an upgrade or reconnect review, try the [three saved-definition regression examples](../examples/mcp-upgrade-regressions/README.md). They cover an output-schema change, unchanged definitions with separate version context, and a missing tool in an incomplete page set. The inputs are synthetic and are also included in native preview.2.

## Compare saved MCP catalogs without a checkout

For Linux x86-64 or Apple Silicon macOS, start with the
[standalone CLI guide](recorded-catalog-cli.md). It covers the release archive,
checksum, fresh-directory installation, and comparison of your own saved input
without Go, Gooo, Python or a model account. Use an absolute path to your input,
keep the availability sidecar and report together, and retain stderr and the
exit status on failure. For feedback, include the release tag, archive name,
manifest source revision, whether the input was yours or the unchanged example,
and the decision or first failure. The guide links the existing feedback form.
For separate raw request/response files, the optional Python 3
[frame assembler](recorded-catalog-cli.md#assemble-saved-frame-files) prepares the
same input while retaining exact frame contents and host-supplied metadata. It
does not infer exposed aliases or missing capture context. Complete input files
still need only the native command.
The source commands below retain the older pinned profiles and their distinct
capabilities.

With Go **1.27.3**, install the comparison from an immutable public module version.
The temporary installation directory keeps its generic `compare` name separate
from other commands. Installation needs access to the Go module service; the
installed comparison reads the supplied input without a compiler, model or
credentials.

```sh
module=github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison
version=v0.0.0-20261025044220-4a0c0e681c42
install_dir="$(mktemp -d "${TMPDIR:-/tmp}/gooo-recorded-catalog.XXXXXX")"
GOBIN="$install_dir" GOTOOLCHAIN=local GOWORK=off \
  go install -trimpath "$module/cmd/compare@$version"
"$install_dir/compare" --help
"$install_dir/compare" --version > "$install_dir/build.json"
"$install_dir/compare" --example > "$install_dir/input.json"
"$install_dir/compare" < "$install_dir/input.json" > "$install_dir/report.json"
```

This is the checked source at
[`4a0c0e68`](https://github.com/kimjooyoon/gooo-go-ports/tree/4a0c0e681c42cec9db2b5ef2dab0931d0a38cf55/examples/recorded-catalog-comparison),
not a request to resolve the newest development revision. The module checksum is
`h1:2xhb092DURbIsgc0cUS12ZjMN21eyvhvdBub9UOHQHc=`. The saved `build.json`
records the running binary's Go version and `main_module.Version` and
`main_module.Sum`; these are build metadata, not authenticated execution evidence.
The three discovery options return without reading stdin. `--example` only
exports the embedded input; the final command separately runs its comparison.
The fresh directory keeps the executable and observations together without
overwriting existing reports.

For the bundled synthetic example, keep the complete saved report and inspect
`alias_decision.state` (`ORIGIN_CHANGED`), its before/after indexes (`0`/`1`), and
`collection_context.state` (`RECORDED_CONTEXT_COMPLETE`). The report also keeps
`contains_synthetic_fixture: true`, `remote_atomicity: UNKNOWN`, and false
`execution_authorized` and `live_removal_proven` fields. These results describe
the supplied example rather than a running server.

To use your own saved observations, provide an input file in the
[documented format](../examples/recorded-catalog-comparison/README.md#provide-your-saved-observations).
Keep the exported example and write your input to a separate file, then run:

```sh
"$install_dir/compare" < your-recorded-input.json > "$install_dir/own-report.json"
```

Preserve the synthetic marker when modifying sample data. For actual recordings,
retain unknown context instead of filling it with the example's IDs.
Retain diagnostics and the nonzero exit status on malformed input; do not turn
an empty output into a decision. Missing pages and unknown recording context
are represented results, not proof that a live tool was removed. This public
command runs the generated Gooo kernel; editing `.gooo` source still requires
the compiler and its native execution/replay workflow.

If you choose to share a [first-use result](https://github.com/kimjooyoon/gooo-go-ports/issues/new?template=problem.yml),
include the command, version, whether you used the example or your own input,
the observed decision or first error, and what you were trying to decide. Share
only public input; a setup failure or unsupported task is useful feedback too.

### Diagnose missing observations in a development snapshot

If your saved MCP catalog comparison is `UNASSESSED`, this development command
identifies missing observation paths and their next collection actions. It uses
[this saved-input format](https://github.com/kimjooyoon/gooo-go-ports/blob/853d14df61ad50c5b4aaca7065ee2b18168f1f9e/examples/recorded-catalog-comparison/README.md#provide-your-saved-observations).
The snapshot has a verified public-module installation and internal retained-input
trial; it is experimental, with whole-source acceptance and release still pending.

With Go **1.27.3**, install it separately from the command above:

```sh
diagnostic_dir="$(mktemp -d)"
GOBIN="$diagnostic_dir" go install \
  github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison/cmd/compare@853d14df61ad50c5b4aaca7065ee2b18168f1f9e
"$diagnostic_dir/compare" --version > diagnostics-version.json
if "$diagnostic_dir/compare" --definitions --diagnostics \
  < your-recorded-input.json > diagnostics.json 2> diagnostics.stderr; then
  printf '0\n' > diagnostics.exit
else
  printf '%s\n' "$?" > diagnostics.exit
fi
```

The resolved module version is `v0.0.0-20261025050343-853d14df61ad`, with checksum
`h1:gBzFkDMwXoUk1c/0dueK82hMaQN9KCVXotJM8hdkar4=`. Installation uses the public Go
module service. The installed command performs no collection, external tool or model call.

Keep the original input, command, version and complete output locally. On exit
`0`, inspect the unchanged `comparison.decision` and each `observations` entry's
`source_document`, `field_path`, `state` and `next_action`. `UNASSESSED` can be a
successful comparison result. On nonzero exit, retain the first stderr diagnostic.
`OBTAIN_OBSERVATION` means obtain the original observation if available; a new
capture belongs to a new record. Do not fill historical gaps with invented IDs
or current values. An unrecoverable gap remains unassessed; the report grants no
execution authority. See the [diagnostic profile and bounds](https://github.com/kimjooyoon/gooo-go-ports/blob/853d14df61ad50c5b4aaca7065ee2b18168f1f9e/examples/observation-diagnostics/README.md).

For optional feedback, reply to [the existing missing-context issue #224](https://github.com/kimjooyoon/gooo-go-ports/issues/224)
with whether this was your own input or the unchanged example, the exact version,
command and exit status, and the first useful field/action or setup diagnostic.
Say whether it helped you decide what to collect next. Keep private captures local;
share only public details, and identify any redacted or reduced reproduction.
A useful result or a first failure is enough.

### Declare unavailable observations in this checkout

For the standalone archive's commands, use the
[native availability guide](recorded-catalog-cli.md#keep-unavailable-information-explicit).
This subsection preserves the source-build workflow.

An opaque value such as `UNKNOWN` may be a real identifier or text retained for
an observation that was never made. The opt-in
[explicit availability profile](../examples/recorded-catalog-comparison/README.md#declare-unavailable-observations-explicitly)
keeps those cases separate without rewriting the saved input. Build the command
from this checkout; the frozen diagnostic installation above does not include
these options.

Run `compare --availability-template` with the saved input and any supplied
policy revisions to obtain a complete field roster. Its states start blank.
For each field, use the retained evidence to choose `SUPPLIED` or `UNOBSERVED`,
then pass the sidecar with `--definitions --diagnostics --availability` and the
same input and policy arguments. Do not fill every state with `SUPPLIED` merely
to make the report pass. Missing required observations prevent a complete-context
decision; independently observed changes and structural failures remain visible.

The template binds the original input and normalized policy; the report also
binds the exact sidecar bytes. Preserve them together. The report calculates observations and next steps; it
does not authenticate the collector or authorize a tool call. See the
[finite Gooo and independent Go verification scope](../examples/explicit-observation-availability/README.md).

## Connect the service-grant MCP tool

If you already use Homebrew on **macOS arm64 or Linux amd64**, install the
[project tap](https://github.com/kimjooyoon/homebrew-gooo):

```sh
brew install kimjooyoon/gooo/gooo-service-grant-mcp
printf '%s/bin/gooo-service-grant-mcp\n' "$(brew --prefix kimjooyoon/gooo/gooo-service-grant-mcp)"
```

Use the absolute executable path printed by the second command in the host
settings below. This installs the existing preview16 native executable and its
license texts, without building Go or Gooo. The tap is project-maintained and
is not part of Homebrew core. If you prefer a container, the
[immutable public Docker image](service-grant-container.md#use-the-public-immutable-image)
needs Docker with a Linux amd64 runtime; its connection settings include the
image reference and runtime arguments rather than a native executable path.
Neither route edits Gooo source; use the first-program workflow for that.

For a manual native installation, download the [native MCP preview.16](https://github.com/kimjooyoon/gooo-go-ports/releases/tag/v0.1.0-preview.16)
archive and matching `.sha256` file for **Linux amd64** or **macOS arm64**.
In the download directory, verify the checksum with `sha256sum -c FILE.sha256`
(Linux) or `shasum -a 256 -c FILE.sha256` (macOS), then extract the verified
archive. It contains `gooo-service-grant-mcp` and 58 license/attribution texts.
Running this executable needs no Go, Gooo compiler or Python installation.
Replace `/ABSOLUTE/PATH/gooo-service-grant-mcp` below with its absolute path;
pass no command arguments.

To build from source instead, use **Go 1.27.3** and the
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

In an earlier maintainer-owned macOS arm64 Codex CLI `0.163.0-alpha.10.2` session,
the preview.13 executable was downloaded from the public release and verified against
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

## Choose among grant-change proposals without a checkout

The rule is written in [Gooo](https://github.com/kimjooyoon/meta-ontology-go).
The command runs generated Go and needs no Gooo compiler, model or API key at runtime.
Use Go 1.27.3 for this pinned example.

Save the [two-candidate input](../examples/grant-candidate-selection/example-input.json)
as `proposals.json`, then run:

```sh
GOTOOLCHAIN=local GOWORK=off go run github.com/kimjooyoon/gooo-go-ports/examples/grant-candidate-selection/cmd/select@v0.0.0-20261022032513-52fb59a8ca9a < proposals.json
```

Go may access module and version metadata while building. The weather-only proposal
is refused because it removes calculator; `all-mcp` preserves both requests.
Inspect every assessment and `execution_authorized:false`. No grant is applied.
See the [schemas and recovery guide](../examples/grant-candidate-selection/agent-recovery.md).
This command is separate from the tools in currently released MCP bundles.
If it fails on your task, share a public synthetic input, the command version and
the diagnostic through [a GitHub issue](https://github.com/kimjooyoon/gooo-go-ports/issues/new).
Do not include private policies or credentials.

## Preserve several required requests through MCP

For reusable agent instructions, use the [service-grant review skill](../.agents/skills/gooo-review-service-grants/SKILL.md). It preserves the required-request list and complete results while comparing the user's proposals.

Use the same [preview.16 native executable](#connect-the-service-grant-mcp-tool)
with the [optional ordered-request mode](../examples/service-grant-mcp/README.md#optional-ordered-required-requests).
For a source-build fallback, use a reviewed checkout containing that mode and
[build the server with Go 1.27.3](../examples/service-grant-mcp/README.md#build-and-connect).
Older preview.13 bundles expose only `assess_service_grant` and cannot serve this
mode. The extracted or source-built executable needs no Go or Gooo at runtime.

Launch its absolute path with `--many-required`. For Codex, follow the official
[configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference)
and use this server entry instead of the default entry above; other stdio hosts
use the same command/args:

```toml
[mcp_servers.gooo_service_grant]
command = "/ABSOLUTE/PATH/gooo-service-grant-mcp"
args = ["--many-required"]
enabled_tools = ["assess_service_grant_change"]
```

Discover `assess_service_grant_change`, then give the agent this finite task:

> An automation needs both weather and calculator. Call
> `assess_service_grant_change` once with each arguments object:
>
> `{"before":"*","after":"mcp://*","required_requests":[{"type":"mcp","name":"weather"},{"type":"mcp","name":"calculator"}]}`
>
> `{"before":"*","after":"mcp://weather","required_requests":[{"type":"mcp","name":"weather"},{"type":"mcp","name":"calculator"}]}`
>
> Automatically select only an `ELIGIBLE` proposal, without asking for approval.
> Retain both complete results and report the rejected candidate's first original
> failure index. If the tool is absent or either assessment fails, report that
> failure rather than substituting the single-request tool. Do not apply a grant
> or call either service.

The first candidate is `ELIGIBLE`; the second is `REFUSED` with
`REQUIRED_REQUEST_REMOVED` at `first_counterexample: 1` (calculator). These are
argument **objects**, not CLI batch arrays. Preserve the ordered-column facts,
all 12 typed decision fields and full bindings, including 30 asset and five
source hashes. The preview.16 executable and current Go 1.27.3 source build use
`205896a4`; the archived preview.14 executable retains its `de99b770` generation. Preserve the
actual compiler and generation identities returned by the executable. Neither outcome
authenticates supplied facts or applies a change; the
[host application boundary](../examples/agentmesh-service-policy/conditional-apply.md)
still applies.

A separate maintainer-owned macOS arm64 Codex CLI `0.163.0-alpha.10.2` session
used the native executable built from source `694d485`, with `--many-required`,
and completed the two calls above. The tool results and final proofs matched all
four ordered-column facts, 12 typed decision fields, 30 asset hashes and five
Gooo source hashes. It selected `mcp://*` and reported the rejected proposal's
first counterexample at index 1. This internal synthetic observation did not use
a public preview.14 download and does not establish external adoption,
independent native-child termination or task completion in Claude Code or Cursor.

For [first-run feedback](https://github.com/kimjooyoon/gooo-go-ports/issues/new?template=problem.yml),
try your own public synthetic API/automation requirements and report the
host/version, candidates and ordered pairs, the selected proposal or first
failure, and any missing facts or patterns. The useful first success is an agent
selecting a narrowing that preserves every required pair; internal trials are
not external adoption.

## Compare a tool alias before reusing it

Use this when an agent previously selected a tool name and you want to compare
its declared server and original function before and after a catalog update.
With Go **1.27.3**, install this immutable source snapshot without a checkout:

```sh
install_dir="$(mktemp -d)"
GOBIN="$install_dir" go install -trimpath \
  github.com/kimjooyoon/gooo-go-ports/examples/service-grant-mcp@v0.0.0-20261024233450-c3f6cdaf7c9f
go version -m "$install_dir/service-grant-mcp"
```

Installation uses the public Go module service. The module checksum is
`h1:GFQHFwo5Z6L1L6B0oFExrEyoYyiPRJkDpNOBNOlX34I=`. Configure your host to run the
absolute path to that `service-grant-mcp` executable with
`args: ["--alias-origin-change"]`. The installed executable needs no Go or Gooo
compiler at runtime. To build a checkout instead, use the
[MCP source instructions](../examples/service-grant-mcp/README.md#optional-tool-alias-origin-comparison).
This mode is not in the preview16 executable, Homebrew package or container image.

Discover `gooo_compare_alias_origin` before calling it. A host that filters
enabled tools must allow this name. If it is absent, keep the executable
identity and setup diagnostic; do not substitute the service-grant tool.
Submit this arguments object to the discovered tool:

```json
{"target_alias":"docs_search","before":[{"server":"docs","original_name":"search","exposed_name":"docs_search"}],"after":[{"server":"issues","original_name":"query","exposed_name":"docs_search"}]}
```

The supplied origin changes from `docs/search` to `issues/query`, returning
`ORIGIN_CHANGED`. Retain the complete facts, input digest, nine-field decision
and source bindings. An `AMBIGUOUS` result identifies duplicate exposed names;
use its original row indexes and the [state-specific recovery guide](../examples/alias-origin-change/agent-recovery.md).
Keep malformed-input or transport errors separate from represented decisions.

These comparisons neither call the named tools nor authenticate live routing,
catalog completeness or authority; `execution_authorized` remains `false`.
Both optional flags may be supplied to expose all three tools. For a real
attempt, [report the first failure or useful result](#report-a-first-attempt-or-a-real-task)
with sanitized catalogs and the exact executable version.

A macOS arm64 internal trial installed this exact version from the public
module service. Its 20 source files matched the accepted source snapshot. Raw
stdio checks covered two protocol versions and four discovery modes, six normal
comparison cases and seven refusal cases, including the large-response limits.
The complete saved wire passed the existing validator, and all owned processes
closed. This is a synthetic transport observation, not a completed external-agent
task or external adoption.

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

On macOS or Linux, prepare Git, Python 3, a POSIX shell, Go **1.27.3**, and a
clean Gooo compiler at `205896a413e5dea92366309c35ae85e542d055f6`.
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
export GOTOOLCHAIN=go1.27.3
export GOWORK=off
go version
./.gooo version --build --json
```

Require compiler revision `205896a413e5dea92366309c35ae85e542d055f6`, Go
`go1.27.3` and build identity `vcs_modified: "false"`, as pinned in
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

## Replace a catalog after refresh

For agent clients that must preserve the previous catalog on a partial or failed
refresh, start with the [whole-catalog refresh example](../examples/catalog-refresh/README.md).
Its Gooo activity checks every candidate row and the complete saved page chain.
The Go consumer stages immutable input and rechecks the captured state under a
lock before replacing local storage. Try its three-case JSON demo with
`go -C examples/catalog-refresh run ./cmd/catalog-refresh-demo` (Go 1.27.3).
This source-based example makes no live calls and is separate from the released
snapshot comparison archive. Report both the Gooo decision and actual commit
result; `CANDIDATE_ELIGIBLE` alone is not a commit.
