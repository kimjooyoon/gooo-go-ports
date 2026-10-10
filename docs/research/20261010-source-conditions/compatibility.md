# Candidate compiler compatibility observation

On 2026-10-10, the existing Gooo Go ports passed their finite compatibility checks
with compiler source `92dc06b38f72518443f8a90e5c98a645f0c756f1`. This is an isolated
observation, not a compiler upgrade: the production pin remains
`366ee6ef8f9d2b9b57331dfd26b6f908c0289851` and the project still provides 15 ports.

The baseline is [main CI run 38015259565](https://github.com/kimjooyoon/gooo-go-ports/actions/runs/38015259565)
at ports revision `e650b7900d29b5590d4f3daebba1f2d88b519c52`. Its complete original
native and browser records are downloadable without an Actions login from
[v0.1.0-preview.1](https://github.com/kimjooyoon/gooo-go-ports/releases/tag/v0.1.0-preview.1).
The candidate experiment used ports revision `dc0694203c9749ffb5d710f8803579731ad39f2e`.
Between these ports revisions, only the issue form, CONTRIBUTING, README and site
HTML changed. Port sources, Go reference cases, manifests and verifier contracts
did not change.

The candidate was built from a fresh, clean Git checkout with Go 1.27.2 on
macOS/arm64. It reports `0.6.24-dev`, `CLEAN_VCS`, and decision runtime
`v0.2.28-experimental` (the baseline uses `v0.2.26-experimental`). The isolated
ports copy changed exactly one lock field, `compiler_revision`. Its browser
metadata correctly records `source_dirty: true`; no production pin was edited.

| Observation | Result |
| --- | --- |
| Existing native corpus and saved replay | 5,510/5,510 cases, 20 suites, 80 batches |
| Comparison with original CI | All 80 case files, manifests, copied sources, program objects, generated Go and complete runtime traces match |
| Browser execution | 3,137 reference comparisons and shared-input roundtrips; 55 rejection checks pass |
| Browser artifacts | Generated Go and Wasm bytes match the original CI; fixture inputs and expectations match |
| Unsupported syntax probes | `for`, byte indexing and bitwise `&` still reject at body generation; declaration checks pass |
| Ports checks | Non-mutating gofmt check, `go vet ./...`, and `go test -race ./...` pass |

Each candidate replay was checked against its own execution receipt SHA-256.
Compiler identity, host, timing and resulting provenance hashes differ; these
differences were retained, not erased to make whole receipts appear identical.
Browser fixture metadata changes only its compiler revision. Site metadata also
records the new ports revision, dirty status and execution digest.

An independent read-only audit repeated the comparison across all 160 native
execution/replay records, checked the input expectations and digest bindings,
and found no discrepancy. It also matched the browser's 65 seed traces and the
single cross-compiler replay described below. The auditor inspected the browser
smoke log; the actual Wasm smoke was run by the experiment runner once.

The [public summary](compatibility.json) and hashes are derived observations. Candidate raw receipts
remain local and are not published with this note; the complete baseline raw
records remain available in the release linked above. The commands below allow
a reader to create a new candidate observation and compare it with that baseline.

One additional cross-compiler replay used the original release's `RGBToCMYK/000`
receipt after relocation. All 64 cases passed with identical full traces and
generated Go. The original producer is `366ee6…`; the new replay records
`92dc06b…` and links to the original receipt bytes. This tests one archived batch,
not all 80 archived batches or general compatibility between compiler versions.

The engine's own test suite was not run in this experiment. No new loop, byte
indexing or bitwise support was observed. These finite results do not establish
package-wide compatibility, performance improvements or an upgrade recommendation.

## Reproduce in a separate directory

Prerequisites: Go 1.27.2, Git, a POSIX shell, Python 3, and Node.js 22 or newer.
Network access is needed for the two repositories and Go dependencies.
All changes below stay in a newly created temporary directory.

```sh
set -eu
export GOTOOLCHAIN=local GOWORK=off
test "$(go env GOVERSION)" = go1.27.2
work=$(mktemp -d "${TMPDIR:-/tmp}/gooo-compat.XXXXXX")
candidate=92dc06b38f72518443f8a90e5c98a645f0c756f1
ports_revision=dc0694203c9749ffb5d710f8803579731ad39f2e
git init "$work/engine"
git -C "$work/engine" remote add origin https://github.com/kimjooyoon/meta-ontology-go.git
git -C "$work/engine" fetch --depth=1 origin "$candidate"
git -C "$work/engine" checkout --detach FETCH_HEAD
test "$(git -C "$work/engine" rev-parse HEAD)" = "$candidate"
test -z "$(git -C "$work/engine" status --porcelain)"
(cd "$work/engine" && go build -trimpath -o "$work/candidate-gooo" ./cmd/gooo)
"$work/candidate-gooo" version --build --json > "$work/candidate-build.json"
git init "$work/ports"
git -C "$work/ports" remote add origin https://github.com/kimjooyoon/gooo-go-ports.git
git -C "$work/ports" fetch --depth=1 origin "$ports_revision"
git -C "$work/ports" checkout --detach FETCH_HEAD
cd "$work/ports"
python3 - <<'PY'
import json, pathlib
identity = json.loads(pathlib.Path('../candidate-build.json').read_text())
candidate = '92dc06b38f72518443f8a90e5c98a645f0c756f1'
baseline = '366ee6ef8f9d2b9b57331dfd26b6f908c0289851'
assert identity['compiler_source_sha'] == candidate
assert identity['source_status'] == 'CLEAN_VCS'
assert identity['version'] == '0.6.24-dev'
lock = pathlib.Path('upstream.json')
raw = lock.read_text()
assert json.loads(raw)['compiler_revision'] == baseline and raw.count(baseline) == 1
lock.write_text(raw.replace(baseline, candidate))
PY
ln -s "$work/candidate-gooo" .gooo
git diff -- upstream.json
go run ./cmd/verify --compiler "$work/candidate-gooo" --out out/native
./tools/probe.sh --compiler "$work/candidate-gooo" --out out/probes
go run ./cmd/build-demo --out out/browser
node tools/browser-smoke.cjs out/browser/site
go vet ./...
go test -race ./...
```

Keep the browser output at `out/browser`: its generated Go module uses the
relative path back to this ports checkout. Output directories must be new.

## Compare and replay the published baseline

Extract the release's native-evidence and browser-demo archives into `$work`.
Their SHA-256 values are in [compatibility-hashes.json](compatibility-hashes.json) and the release's `SHA256SUMS`.
The optional `compatibility-signature.py` reads records without running a compiler:
run it against `out/native` and against the extracted `evidence/ci` directory.
Both observations produce the native behavior digest in [compatibility-hashes.json](compatibility-hashes.json).
It includes every batch's original input/manifest/source bytes, generated Go,
program object and exact-integer full traces. It excludes environment and timing
metadata; it is a comparison digest, not a replacement for receipt validation.

After extracting the native archive, reproduce the single old-receipt observation:

```sh
batch="$work/gooo-go-ports-v0.1.0-preview.1-native-evidence/evidence/ci/go-image-color-RGBToCMYK/000"
"$work/candidate-gooo" package replay --json \
  --receipt "$batch/execution.json" --cases "$batch/cases.json" \
  "$batch/gooo.workspace.json" > "$work/old-release-replay.json"
```

Keep the original receipt bytes unchanged. Record the new command's exit status,
producer identity and `replayed_from_sha256`; do not treat rejection by another
compiler version as permission to alter the original record.
