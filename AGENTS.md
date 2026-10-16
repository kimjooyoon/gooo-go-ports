# Gooo porting contract

- Implement ported behavior in `ports/**/*.gooo`; Go owns oracle execution and evidence checking.
- Preserve upstream copyright, license, immutable source revision and source-file hashes.
- Document Go-to-Gooo type/domain and API differences explicitly. Do not hide unsupported semantics behind a Go wrapper.
- Keep compiler and Go toolchain identities pinned in upstream.json and bootstrap/CI.
- Use independent Go reference outputs, boundary cases, deterministic corpora and saved replay.
- Match receipt inputs, outputs, entry identities, source bindings and producer identities; a PASS label alone is insufficient.
- Record unsupported syntax as executable probes. Never equate declaration checking with generated native execution.
- Required checks: gofmt, go vet ./..., go test -race ./..., native differential verifier, and boundary probes.
- Narrow CI exception for documentation-only pull requests: the scope classifier must verify the actual clean checkout SHA, exact PR merge parents, and a nonempty base-to-tested-tree delta containing only content modifications to existing files in this exact six-path allowlist: `README.md`, `CONTRIBUTING.md`, `.agents/skills/gooo-first-program/SKILL.md`, `.agents/skills/gooo-first-program/agents/openai.yaml`, `docs/ai-agents.md`, and `context7.json`. Every changed file must have mode `100644` on both sides; additions, deletions, renames and mode changes require full native validation. Common Go/Python checks, generator checks, pinned compiler identity, and declarations remain required; only native differential/exhaustive, boundary-probe, native-example, malformed-input, and policy execution may be omitted. Record the omission explicitly without new native-port receipts or rebinding old observations. Browser validation remains full. Main and tag pushes always require full native validation. All other changes, including workflow, classifier, tests, pins, fixtures, all other nested documentation or agent files, or this contract, require full native validation; classification errors must never produce a documentation-only success.
- Keep observations finite and reproducible. Do not claim full-library compatibility from function-level observations.
