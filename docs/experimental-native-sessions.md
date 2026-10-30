# Experimental native sessions with compressed evidence

Use `experimental-session-276-gzip-v1` to run the full legacy port corpus through
caller-owned compiler276 sessions on macOS or Linux. Go still checks the original
inputs, oracle results, sources and each batch’s own saved replay. Each execute
and replay request performs two fresh native runs; an artifact reuse never reuses
a previous outcome.

This is a separate development profile. The default stateless compiler205
workflow and plain `experimental-session-276-v1` evidence format remain unchanged.

## Prepare a clean compiler276

Use Git and Go **1.27.3**. Reuse a matching clean compiler, or choose a new engine
source directory and a binary path outside it. The binary’s parent must exist;
the first build may download its pinned modules.

```sh
go_tool=/absolute/path/to/go1.27.3/bin/go
engine=/absolute/path/to/new-engine276-source
compiler=/absolute/path/to/tools/gooo276
revision=276734167edc7f5fef18cbb79004ea9592553317

git init "$engine"
git -C "$engine" remote add origin https://github.com/kimjooyoon/meta-ontology-go.git
git -C "$engine" fetch --depth=1 --no-tags origin "$revision"
git -C "$engine" checkout --detach "$revision"
test -z "$(git -C "$engine" status --porcelain=v1 --untracked-files=all --ignored)"
GOTOOLCHAIN=local GOWORK=off "$go_tool" -C "$engine" build \
  -mod=readonly -o "$compiler" ./cmd/gooo
"$compiler" version --build --json
test -z "$(git -C "$engine" status --porcelain=v1 --untracked-files=all --ignored)"
```

Require `compiler_source_sha` and `vcs_revision` to equal the revision above,
`source_status: "CLEAN_VCS"`, `vcs_modified: "false"`, and both `go_version` and
`native_go_required` to equal `go1.27.3`. The descriptive module version can vary
with tag context. Keep this compiler separate from the checkout’s pinned `.gooo`;
`tools/bootstrap.sh` continues to prepare compiler205.

## Run the full legacy corpus

Use a ports checkout containing this profile. Choose a new persistent output
path outside the engine source; do not create the records directory first.

```sh
ports=/absolute/path/to/gooo-go-ports
records=/absolute/path/to/new-native-session-records
GOTOOLCHAIN=local GOWORK=off "$go_tool" -C "$ports" run -mod=readonly ./cmd/verify \
  --execution-profile experimental-session-276-gzip-v1 \
  --compiler "$compiler" --workers 4 --fail-fast --timeout 20m --out "$records"
```

The command keeps the full original suite roster and legacy case/workspace
defaults. Compact case profiles, dependency-closure workspaces, `--timing-out`,
browser fixtures and CI shard modes are refused. Each suite closes and waits for
its own session before its callback returns. Keep incomplete or failed records
with their actual result; a partial run does not qualify the full corpus.

## Read the evidence

The saved references distinguish two identities:

- `bytes` and `sha256` describe the exact decoded original JSON bytes, including
  the saved LF. These bytes retain the replay-parent identity.
- With `encoding: "gzip"`, `path` names the `.gz` file; `stored_bytes` and
  `stored_sha256` describe that actual compressed blob.

Frames use gzip; a storage failure retains a plain fallback when possible and
fails the request. Batch `execution.json` and `replay.json` are compacted only
after both receipts, their parent binding and applicable exhaustive checks pass.
Both gzip copies are read back before the plain originals are removed. Plain session references retain the original three fields.

Inspect `summary.json`, `session-observations.json`, source-identity records and
the per-suite owner/process records together. File hashes are unsigned
consistency evidence. Current native build/run records remain separate from the
historical build of a reused artifact. A reused Go toolchain likewise retains its
historical version check without inventing a current version process. Read actual
hit/miss counts from receipts.
No model credentials are needed, and runtime/replay receipts must report zero
model calls. Whole waited-session CPU observations are not per-request OS
resource measurements; unavailable per-request wait resources remain
`NOT_OBSERVED` with null values. No performance or hosted-model claim follows.

## Check a result

Retain the exit status, stderr, exact ports revision, compiler build identity and
Go version/platform with the records. A successful full legacy run has all 82
suites and 175,649 cases verified, with per-suite legacy batches totaling 2,579.
Check the matching source identities and all owner cleanup outcomes as well.
A saved partial prefix, another platform’s result or successful Go unit tests
does not establish this complete native result.

## Optional Linux workflow

`Manual full82 experimental native session` is a separate manual workflow on
Ubuntu 24.04, Linux amd64. Select a reviewed repository ref and supply its full
40-character commit as `expected_source_sha`. The workflow, checked-out source
and input must identify the same commit. It builds the pinned compiler276 and
consumer verifier with Go 1.27.3, then runs the original legacy corpus once with
four workers. It does not replace required CI or change the default compiler pin.

The controller requests an orderly session close on cancellation or failure,
then checks recorded process identities before stopping remaining owned groups.
It records the original exit status and bounded cleanup outcome. Storage samples
can cancel a run; they are observations rather than filesystem quotas. A hard
runner termination can still prevent cleanup or artifact upload.

The one-day diagnostic artifact contains bounded logs, source/tool identities,
summaries and owner lifecycle records. Each command stream retains at most a
1 MiB prefix, and the upload is capped at 64 MiB. Exceeding a bound fails the
check and records truncation; a retained prefix is not a complete log. Raw batch
receipts and transport frames are excluded, so this artifact cannot support a
later independent audit of every original receipt. Read its inventory and
failure records before interpreting a successful summary. This route makes no
speed comparison or per-request OS resource claim.
