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
- **Import current ports into a separate Gooo project:** follow the
  [current path-import walkthrough](use-current-path-in-your-project.md) with
  its pinned toolchain, source mapping and saved execution/replay.

Report the compiler identity, source change, actual result, saved record paths
and any unsupported behavior. An agent's own trial is tooling verification;
independent adoption requires a separate user's actual use.
