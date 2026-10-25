# HeaderAddition evidence export prototype

This first slice projects the existing experimental native-shard reader's
`app/diagnostic-header-addition:AssessAddition` observations into the fixed
[bridge evidence/v1 contract](https://github.com/kimjooyoon/gooo-language-bridge/blob/6142c05e9aa32b5003dcd77cfbc87669f66ed402/bridges/contribution-response/evidence.schema.json).
Implementation and unit controls are local; real-artifact integration and the
required exact-revision hosted checks are still pending. It does not complete
[issue #211](https://github.com/kimjooyoon/gooo-go-ports/issues/211).

The exporter calls the existing full-population reader without changing its
checks. The reader must finish before the selected 50 typed rows and 14 original
execution/replay references become export input. It does not rerun Gooo, a Go
oracle or a corpus. It cannot accept a smaller smoke index, input-only records,
generation-only reports, or an incomplete shard population as successful native
evidence. Source/tool/entry/raw-parent checks and final file rechecks remain in
the existing reader.

## Input and output

Use an existing supported reference index, its exact source checkout and its
already-retained evidence directories. The source checkout's `.gooo` bytes must
match the producer's recorded compiler bytes; a current compiler for another
platform is not interchangeable. The supported reader/profile hashes are
explicit in the script. Unrecognized code is refused before it is loaded.

```sh
python3 -I -B tools/export-header-addition-evidence.py \
  --source-root /ABSOLUTE/EXACT-SOURCE-CHECKOUT \
  --reference-index /ABSOLUTE/RELOCATED-REFERENCE-INDEX.json \
  --evidence-root /ABSOLUTE/RETAINED-SHARD-0 \
  --evidence-root /ABSOLUTE/RETAINED-SHARD-1 \
  --evidence-root /ABSOLUTE/RETAINED-SHARD-2 \
  --run-failure /ABSOLUTE/ORIGINAL-FIRST-FAILURE.json \
  --out /ABSOLUTE/FRESH-OUTPUT
```

The failure option is optional; omitting it leaves overall workflow status
unassessed. A supplied first-failure record must bind the same run. Paths in an
index never grant access: every evidence root must be supplied independently by
the caller's host configuration. A private byte-identical index snapshot and a
fresh Python cache prefix are used during the existing reader call. No metadata
selects Python modules or commands.

On exit code `0`, `result/` is published as one directory containing a file-hash
manifest, the typed appendix, three separately schema-valid evidence files
(`native_execution`, `oracle_comparison`, `saved_replay`), and a short technical
preview generated from those same records. Typed values, case order, source
bindings and original-JSON receipt digests are retained. Gzip storage digests
remain separate. Raw receipts stay in the original evidence store.

`pass` describes only the selected saved observation's successful checks. It
does not mean the whole workflow succeeded. A same-run producer failure remains
visible even alongside completed selected observations. Git identities reported
by the producer remain in the detailed appendix; checked source/base/candidate
identities are unknown in the bridge records. Comparison status is
`not_compared`: saved Go expectations do not establish a code regression between
two revisions. Finite matches do not establish whole-package compatibility or
production impact.

The source reader remains unsigned/reference-only. `native_acceptance` and
`producer_authenticated` remain false. The export supplies no execution or
publication authority, and invokes no response planner, network API or model.
The short preview contains no original rows, local filesystem roots or raw
producer instructions. The detailed local appendix may contain original
provenance text and is not uploaded automatically.

On refusal there is no `result/` directory or successful evidence record. Stderr
contains a fixed JSON diagnostic; the first detailed error stays in the local
`refusal.json`. An existing output directory is never overwritten. Unit fixtures
are labelled synthetic and cannot replace the still-required real integration.

The fixed contract and small schema validator retain their upstream source
revision, hashes and MIT notice in `tools/contracts/` and `tools/licenses/`.
