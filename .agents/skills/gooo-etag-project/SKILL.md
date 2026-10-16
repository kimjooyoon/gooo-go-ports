---
name: gooo-etag-project
description: Export, edit and independently check Gooo HTTP ETag scanner, strong comparison or strong-validator facts against Go. Use when the user requests this Gooo porting workflow; general Go development does not need it.
---

# Gooo ETag project

Produce the user's requested Gooo body edit and its actual comparison with the
original Go behavior. The existing exporter creates a standalone 13-file project;
it needs Python 3 and existing compatible Go/Gooo executables on macOS or Linux.

## Locate and export

Use this loaded `SKILL.md` file's absolute location, supplied by the client, to
find the bundle root: it is three directories above the containing
`.agents/skills/gooo-etag-project/` directory. Do not use the consumer's working
directory as the bundle root. Check that `tools/export-etag-project.py` and
`templates/etag-project/README.md` exist there. Read that README, the relevant
Gooo sources and notices, and the bundle's `AGENTS.md` before changing code.

The installed plugin bundle is a source resource. Export into a new directory
outside it; keep the plugin cache and the user's existing checkout unchanged:

```sh
python3 "$BUNDLE_ROOT/tools/export-etag-project.py" --out "$NEW_PROJECT"
```

Set both variables to actual absolute paths; `NEW_PROJECT` must not exist and its
parent must exist. Inspect the export manifest: copying files is not execution.
If a standalone skill installation lacks those bundled files, use an existing
compatible `gooo-go-ports` checkout or the immutable
[export guide](https://github.com/kimjooyoon/gooo-go-ports/blob/6b51df85fac103f0dd7e9043480047e460fe9855/templates/etag-project/README.md)
to obtain that snapshot as part of the user's requested setup. Do not guess paths
or write into the installed skill.

## Run and edit

Read the exported `README.md` and `NOTICE`. Reuse absolute, non-symlink paths to
Go **1.27.2** and a clean Gooo compiler at
`83928fa4538245abe33b08549da47f8cbdc50ed2`; the checker checks their actual
identities. The pinned Go dependencies must already be cached. When setup is
needed, explain it and perform it within the user's requested setup scope, in a
separate workspace. Do not bootstrap or build tools in the plugin bundle.

From the exported project, run the ordinary checker with a fresh output path:

```sh
GOTOOLCHAIN=local GOWORK=off GOENV=off GOPROXY=off GOSUMDB=off \
  "$GO_EXE" run -mod=readonly . \
  --compiler "$GOOO_EXE" --go "$GO_EXE" --out "$NEW_OUT"
```

Inspect the first run before editing. Make the requested change in `computes` of
`ports/net/http/etag.gooo`, `ports/net/httpguts/headers.gooo` or
`examples/strong-validator-facts/main.gooo`, retaining the public signatures and
record fields. Preserve existing user changes. Run again under a new output
path. The checker permits changed body bytes and creates fresh Go expectations;
it does not replay an old PASS for the edited source.

The baseline covers all 247 frozen inputs and 727 returned scalar values in
seven batches. Compare actual typed Gooo values, not just command exit status:
a native CLI can exit zero while reporting a comparison failure. Preserve the
first diagnostic, raw records and source edit when a check fails. If the user
intentionally changes behavior, report the observed difference from Go instead
of altering the frozen corpus or expectations to obtain PASS.

For an unchanged original Report expression, `--mutation-check` additionally
checks a separate 19-row workspace where removing `header_valid` must change the
CRLF row's `single_strong` field. It is optional, applies only to that original
expression, and does not establish 247 mutated results.

## Report the result

Give the edited source path, actual comparison result, tool identities and new
evidence paths. Inputs retain their original Unicode-scalar UTF-8 bytes, including
empty, NUL and controls, with an independent 0..1024-byte bound per Text operand.
Do not silently trim, repair or truncate inputs, or replace a Gooo result with Go.
These are finite execution observations. The facts project does not implement
full If-Match, authorization or an atomic HTTP write.
