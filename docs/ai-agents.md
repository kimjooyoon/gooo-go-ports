# Use Gooo with a coding agent

This repository provides a small, source-based Gooo workflow for a local coding
agent. Start with a concrete task: edit a Gooo program, execute the changed
source, and save a new execution/replay pair.

## Run the first-program skill

The shared skill is
[gooo-first-program](../.agents/skills/gooo-first-program/SKILL.md). It uses the
existing [two-file tutorial](../examples/learn-gooo/README.md), its four supplied
inputs, and the repository's pinned compiler and Go toolchain. Setup needs Git,
a POSIX shell and Go toolchain selection on macOS or Linux.

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

## Choose a task that matches the current capability

- **Write a Gooo program:** use the first-program skill, then the existing
  [local edit-and-replay guide](../examples/learn-gooo/README.md#make-one-visible-change).
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
- **Inspect an API ETag before a conditional write:** use the
  [strong-validator workflow below](#inspect-etag-and-header-facts). It preserves
  raw input and separates parsed-tag strength, trailing content and raw header
  validity. It does not implement the write or full `If-Match` handling.
- **Import current ports into a separate Gooo project:** follow the
  [current path-import walkthrough](use-current-path-in-your-project.md) with
  its pinned toolchain, source mapping and saved execution/replay.

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
