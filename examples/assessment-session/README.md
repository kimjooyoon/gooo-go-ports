# Compare captured test outcomes with declared intent

Run the `Assess` Gooo activity over two saved Go test streams. Gooo compares the
declared test plan with the supplied observations. The standard-library Go host
runs the session, prints compact values and retains the original records.

This example uses a separate development compiler profile at
[`276734167edc7f5fef18cbb79004ea9592553317`](https://github.com/kimjooyoon/meta-ontology-go/tree/276734167edc7f5fef18cbb79004ea9592553317).
Use Go **1.27.3** on macOS or Linux. The repository's compiler205/native17
profile continues to serve its existing workflows.

## Prepare the compiler

Reuse an available clean compiler from the revision above. To build one, choose
a new source directory and an output binary outside it:

```sh
go_tool=/absolute/path/to/go1.27.3/bin/go
engine=/absolute/path/to/new-engine276-source
compiler=/absolute/path/to/output/gooo
# The parent directory of the output binary must already exist.
git init "$engine"
git -C "$engine" remote add origin https://github.com/kimjooyoon/meta-ontology-go.git
git -C "$engine" fetch --depth=1 --no-tags origin 276734167edc7f5fef18cbb79004ea9592553317
git -C "$engine" checkout --detach 276734167edc7f5fef18cbb79004ea9592553317
GOTOOLCHAIN=local GOWORK=off "$go_tool" -C "$engine" build -o "$compiler" ./cmd/gooo
"$compiler" version --build --json
```

Keep the source checkout clean. The example checks the compiler's full source
revision, `source_status=CLEAN_VCS`, `vcs_modified=false` and
`go_version=go1.27.3` before starting the session. Its descriptive module version
can vary with local tag context.

## Run from any working directory

Set the checkout path and choose a new persistent records directory:

```sh
ports=/absolute/path/to/gooo-go-ports
records=/absolute/path/to/new-assessment-records
mkdir "$records"
GOTOOLCHAIN=local GOWORK=off "$go_tool" -C "$ports" run ./examples/assessment-session \
  --compiler "$compiler" --go "$go_tool" --out "$records"
GOOO_ASSESSMENT_RECORDS="$records" GOTOOLCHAIN=local GOWORK=off \
  "$go_tool" -C "$ports" test -count=1 ./examples/assessment-session
```

`--out` must already exist and be empty. The command preserves its records after
exit, including received frames and diagnostics from an incomplete attempt. Use
a new directory for another attempt.

## Read the two results

Each printed view contains `program_value`, `assessment_execution` and pointers
to the full evidence files, with byte counts and SHA-256 hashes.

| Case | Gooo program value | Expected-result score |
| --- | --- | --- |
| `red-correct` | `TEST_SET_MISMATCH`, failure observed | `PASS`, 1/1 |
| `red-intentional` | `MATCHED_EXPECTATION`, failure observed | `PASS`, 1/1 |
| `green-correct` | `MATCHED_EXPECTATION`, no failure observed | `PASS`, 1/1 |
| `wrong-expected` | The actual matched green value | `PROGRESS`, 0/1 |

`red-correct` has outer `PASS` because it accurately returns the authored
expected assessment: the declared domain test intent was not met. The final
control intentionally supplies an incorrect expected state and retains its
`PROGRESS`, 0/1 result even though the demonstration command exits successfully.

The example has two original captures, three intent/capture combinations and
four result cases. Each inner assessment selects three leaf tests and their
parent. One execute and three replays produce eight fresh native runs, one
native artifact build and three artifact reuses. Every replay uses the same
saved first receipt. The original OpenFGA tests are supplied fixtures; this
command makes no model call.

## Inspect the saved records

- `summary.json` records the finite counts and cleanup result.
- `views/` contains the compact displays.
- `*-response.jsonl` and `*-request.jsonl` retain the exact received and sent lines.
- `first-receipt.json` retains the inner receipt plus a caller-added LF. Its exact
  saved bytes are the replay parent.
- `profile/`, context files and identity records retain source, input, capture,
  request, compiler and Go references.

The 27 sample-dependent Go subtests read these fresh records. They cover malformed
values, source/input/parent mismatches, duplicate JSON keys, and capture/request
mixups. Without `GOOO_ASSESSMENT_RECORDS` or the `-records` test flag, repository
tests explicitly skip those controls. Large receipt fixtures are generated at
runtime.

The session deadline is 90 seconds. Limits are 8 MiB per frame, 1 MiB for stderr
and 32 KiB per printed view; identity probes have a 15-second deadline and 1 MiB
per output stream. The host closes and waits for its session and checks its
owned native work under `--out/native-work`. Other evidence files remain.

## Fixture source and scope

The six capture files are historical local OpenFGA test outputs and their
process/source-binding records. Their bytes are preserved. The relative
capture manifest records their original hashes; source IDs derive from the
source-binding file bytes. This example assesses those supplied records and
selected test nodes. Current OpenFGA behavior and capture-time authenticity are
outside that observation.

Two error traces retain a historical local source path. These are inert recorded
data and are not required on your machine.

See `SOURCE.json`, `LICENSE-MIT` and `LICENSE-OpenFGA` for source revisions and
attribution. New Go glue uses this repository's BSD license, the unchanged Gooo
activity retains MIT attribution, and the OpenFGA data retains Apache-2.0
attribution. File-binding checks are unsigned consistency checks; the output is
diagnostic assessment data.
