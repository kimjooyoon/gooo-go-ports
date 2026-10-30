# Reproduce the legacy tutorial snapshot

Use this procedure only to reproduce the older tutorial. The current tutorial
targets Go **1.27.3**; this archive is a separate Go **1.27.2** reproduction.

Keep these identities together:

| Component | Frozen identity |
| --- | --- |
| Example checkout | `e1e279a25094f55a2ebd926a5e8393448eae418c` |
| Compiler source | `83928fa4538245abe33b08549da47f8cbdc50ed2` |
| Go toolchain | `go1.27.2` |
| Compiler archive | `v0.6.32-dev`, matching the host platform |

1. Reuse a matching `kimjooyoon/gooo-go-ports` checkout or obtain that exact
   revision in a new task directory. Preserve the user's changes and branch.
   Read its `AGENTS.md`, `examples/learn-gooo/README.md`, both tutorial `.gooo`
   files and `learn-gooo.workspace.json` before editing. The immutable
   [example guide](https://github.com/kimjooyoon/gooo-go-ports/blob/e1e279a25094f55a2ebd926a5e8393448eae418c/examples/learn-gooo/README.md)
   documents that snapshot.

2. Obtain the matching archive and `SHA256SUMS` from the
   [v0.6.32-dev release](https://github.com/kimjooyoon/meta-ontology-go/releases/tag/v0.6.32-dev).
   Verify the archive against its matching checksum **before** extracting into
   a fresh directory. Preserve existing `.gooo`, `.compiler` and receipts; do
   not replace an installed compiler to test the archive.

3. From the example checkout, select Go **1.27.2** and save the unpacked
   executable's identity to a fresh file. Set `gooo_legacy_compiler` to its
   actual absolute path before running:

   ```sh
   export GOTOOLCHAIN=go1.27.2
   export GOWORK=off
   gooo_legacy_dir="$(mktemp -d "${TMPDIR:-/tmp}/gooo-legacy.XXXXXX")"
   "$gooo_legacy_compiler" version --build --json \
     > "$gooo_legacy_dir/compiler-build.json"
   go run ./tools/bootstrap-check.go "$gooo_legacy_dir/compiler-build.json"
   ```

   Stop on failure and preserve its first diagnostic. Use the snapshot's own
   validator; selecting Go alone does not establish compiler identity.

4. Only after successful validation, substitute the validated executable's
   absolute path for `./.gooo` and skip that snapshot's bootstrap. Native
   execution still requires Go **1.27.2**. Save and replay its execution:

   ```sh
   "$gooo_legacy_compiler" package execute --json \
     --inputs examples/learn-gooo/inputs.json learn-gooo.workspace.json \
     > "$gooo_legacy_dir/execution.json"
   "$gooo_legacy_compiler" package replay --json \
     --receipt "$gooo_legacy_dir/execution.json" \
     --inputs examples/learn-gooo/inputs.json learn-gooo.workspace.json \
     > "$gooo_legacy_dir/replay.json"
   ```

   Inspect `result.runtime.traces[].deliveries[].actual`. Input-only records
   are finite `OBSERVED` results, not independent expected-output verification.
   Keep source, input and records together under fresh paths. For a requested
   source edit, save and replay a new execution; the original record is not
   evidence of the edit.

Never substitute `latest`, relabel this archive as a Go 1.27.3 compiler, or use
it with the current tutorial's compiler/validator pins. Keep global agent
settings unchanged. Provenance: the immutable
[archived installation procedure](https://github.com/kimjooyoon/gooo-go-ports/blob/c3f6cdaf7c9f2f84bcb0f9e0076ade2d72eb3ec2/.agents/skills/gooo-first-program/SKILL.md#find-the-example).
