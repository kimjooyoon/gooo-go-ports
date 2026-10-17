# Use Gooo with a coding agent

This repository provides a small, source-based Gooo workflow for a local coding
agent. Start with a concrete task: edit a Gooo program, execute the changed
source, and save a new execution/replay pair.
The [Gooo language and compiler](https://github.com/kimjooyoon/meta-ontology-go)
are developed separately; this repository supplies ports and runnable examples.

## Compare fixed API policies without a compiler

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
| Claude Code | Use the explicit file-based task above. For native skill discovery, copy this skill directory into a new `.claude/skills/gooo-first-program` directory without replacing an existing skill. |

These are local repository or user-installed workflows. A public skill file does
not install itself into unrelated sessions. Follow the client's scope and trust
settings when copying or installing a skill. Official discovery guidance:
[Codex](https://learn.chatgpt.com/docs/build-skills),
[Cursor](https://cursor.com/docs/skills),
[Claude Code](https://code.claude.com/docs/en/skills).

The task changes actual Gooo source. The [browser tutorial](https://kimjooyoon.github.io/gooo-go-ports/tutorial.html)
runs a fixed compiled program; use the native workflow to verify a source edit.
The skill explains the signed-int64 and 1,024-byte Text limits of this example.
An input-only `OBSERVED` result is a finite execution observation, not a proof
for all inputs. Preserve a first failure and its diagnostic.

### Optional project-local skill installation

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

- **Write a Gooo program:** use the first-program skill, then the existing
  [local edit-and-replay guide](../examples/learn-gooo/README.md#make-one-visible-change).
- **Edit an API policy rule:** use the [editable source comparison](../examples/editable-api-policy/README.md).
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
- **Evaluate a selected If-Match condition:** use the [bounded compatibility core](../ports/net/http/README.md#selected-if-match-compatibility-core), `if-match.workspace.json` and its strict `cmd/if-match-input` handoff. Preserve both original Texts; `condition`0/1/2 means none/true/false. Header selection, authenticated current facts and applying a write are separate.
- **Edit and regression-check an isolated ETag project:** [export the current
  three-source project](../templates/etag-project/README.md), edit its Gooo
  `computes` bodies under the fixed public ABI, then compare247 original inputs
  with fresh Go expectations in seven batches. Exporting is a source copy;
  the separate checker verifies actual Gooo results and an optional19-row CRLF
  mutation witness. Saved-receipt replay is a separate workflow.
- **Automatically check a versioned service-grant tightening:** use the
  [versioned grant-change example](../examples/versioned-grant-change/README.md).
  Gooo combines strict version matching with supported grant narrowing and
  required-request preservation. Its verifier compares 48 fixed cases with fresh
  component references and saved replay. An ELIGIBLE result is not an APPLIED
  write; current authority and atomic application remain host responsibilities.
- **Import current ports into a separate Gooo project:** follow the
  [current path-import walkthrough](use-current-path-in-your-project.md) with
  its pinned toolchain, source mapping and saved execution/replay.

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
