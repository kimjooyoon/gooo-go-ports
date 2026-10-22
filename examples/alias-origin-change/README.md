# Compare a tool alias before and after a catalog change

An agent can keep seeing `docs_search` while that name starts targeting a different server or original tool. This example compares two supplied catalogs and reports that change using an original Gooo activity. It also detects duplicate exposed names, additions and removals.

From the repository root of a reviewed checkout, use Go **1.27.3**:

```sh
GOTOOLCHAIN=local GOWORK=off go -C examples/alias-origin-change run -mod=readonly ./cmd/compare \
  < examples/alias-origin-change/example-input.json
```

The example returns `decision.state: ORIGIN_CHANGED`: `docs/search` became `issues/search` under the same `docs_search` alias. Both original positions are zero. `execution_authorized` is always false. Exit zero means a represented comparison result, including ambiguity or invalid domain input; it does not mean execution is allowed.

Building may download the pinned test module during verification. The compiled command itself has no third-party runtime dependency and needs no Go, Gooo, Python, model or API key at runtime. This module is under development; released MCP bundles do not expose it. No published version or live ADK adapter is implied by the checkout command.

## Agent contract

Send one JSON object with exactly `target_alias`, `before` and `after`. Each catalog is an array of records with exactly `server`, `original_name` and `exposed_name`. Use the standalone schemas for [input](schemas/input.schema.json), [stdout](schemas/output.schema.json) and [stderr](schemas/diagnostic.schema.json), then follow the [state and recovery guide](agent-recovery.md).

The result preserves supplied facts, their digest, the exact received JSON digest, all nine Gooo decision fields and pinned generation bindings. `input_sha256` includes received whitespace. `facts_sha256` identifies the Go JSON serialization of the captured facts, including Go's HTML and Unicode separator escaping. It is not an RFC 8785 digest. `decision.context_id` binds that facts digest.

The Go API exposes `Compare(Input)` and `CompareJSON([]byte)`. `Compare` snapshots the row slices and treats nil catalogs as empty. The JSON API rejects null catalogs. Callers must not concurrently mutate a slice while the snapshot is being taken. The internal parallel-column kernel is not the public API.

## Exact represented domain

Each catalog has at most 256 rows. Each text is valid UTF-8 of at most 1,024 bytes. Text is opaque: no trimming, case folding, Unicode normalization or server-name parsing occurs. Origin equality compares the pair `(server, original_name)` without delimiter concatenation. The code checks duplicate exposed names throughout both supplied catalogs, including duplicate rows with identical origins. An unrelated duplicate still makes the result ambiguous.

Empty target or row text reaches Gooo and produces `INVALID_INPUT`. Invalid JSON, unknown/duplicate keys, null arrays, unpaired surrogate escapes, oversized text or 257 rows are transport failures. The CLI accepts at most 1 MiB and limits its serialized report to 4 MiB before writing stdout. JSON Schema string lengths count characters; the native UTF-8 byte bound is separately enforced.

Gooo owns admission and comparison in [`ports/tool-origin/alias_origin.gooo`](../../ports/tool-origin/alias_origin.gooo). Go owns strict transport, snapshots, hashes and independent reference evaluation. The generated kernel comes from actual native execution/replay with compiler `205896a413e5dea92366309c35ae85e542d055f6` and Go 1.27.3; comparison is not reimplemented in the public Go wrapper.

These are supplied snapshots. The result does not prove catalog completeness, live server identity, caller authority or the target used by an SDK. It makes no ADK compatibility or vulnerability claim. An integrating host can perform this pure calculation without routine human approval; actual effects require its separately enforced current policy and execution boundary.

## Reproduce the verification

Provide the exact compiler above and Go 1.27.3. The compiler is not built or downloaded by this verifier. Evidence must go to a fresh directory outside the checkout:

```sh
python3 -I -B examples/alias-origin-change/verify.py \
  --compiler /absolute/path/to/gooo --go /absolute/path/to/go \
  --out /absolute/path/to/new-alias-origin-evidence
```

The verifier checks an independent map-based Go reference against 111 kernel cases, freshly executes and replays all cases, checks all generated assets, and runs module integrity, static, race and schema checks. It builds the CLI and runs three real inputs with empty PATH and absent GOROOT, including opaque Unicode and ambiguous aliases. All nine fields are compared with the independent reference and fresh native replay. Four altered receipts and three malformed inputs must be rejected.

Optional `--go-cache` and `--module-cache` reuse existing caches without removing them; no cold-cache performance or shared-cache quota is claimed. Child-process evidence retains waits, stream EOF and process-group closure. Completed owned command binaries are removed after closure.

`generate.py --out` emits stable generated assets plus an actual local PROV-O derivation file. It records source, workspace, cases, generator, schema dependency, receipts and formatter identities. Generation metadata binds the canonical projection of the two root fields actually consumed by this renderer: `compiler_revision` and `go_version`. The PROV-O graph retains both the actual root document and its selected projection, plus the actual Go binary used for receipt correspondence. A dependency regression checks that unrelated root metadata and JSON formatting preserve all seven generated assets while relevant pin, source and case changes reject old receipts. This prevents artifact churn; it does not skip CI or establish a runtime saving. Hashes and unsigned provenance identify observed bytes, not a trusted producer or current authorization. Local verification does not establish hosted CI acceptance, external adoption or general client compatibility.
