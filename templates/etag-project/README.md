# Editable Gooo ETag project

In a gooo-go-ports checkout, export the current sources to a new directory:

```sh
set -eu
python3 tools/export-etag-project.py --out /absolute/existing/parent/gooo-etag
cd /absolute/existing/parent/gooo-etag
```

The exporter copies seven current source/data/license files and six templates.
It refuses an existing output path and prints a JSON export manifest. An export
only copies source; it does not run Gooo. This exported project then works
without the original checkout or repository-wide verifier.

## Prepare and run

Use existing absolute, non-symlink paths to Go **1.27.2** and a clean Gooo compiler
at `83928fa4538245abe33b08549da47f8cbdc50ed2`. On macOS or Linux, set `GO_EXE`
and `GOOO_EXE` below to those paths. The pinned x/net/x/text dependencies must be
in the local Go module cache. If they are missing, prepare them once with network
access before the offline check, using the same selected Go executable:

```sh
set -eu
GO_EXE=/absolute/path/to/go1.27.2/bin/go
GOTOOLCHAIN=local GOWORK=off GOENV=off "$GO_EXE" mod download
GOTOOLCHAIN=local GOWORK=off GOENV=off "$GO_EXE" mod verify
```

Compiler installation is a separate setup step. A matching compiler from the
original checkout's `tools/bootstrap.sh` can be reused. The included CI shows
how to build it outside a clean pinned engine checkout.

```sh
set -eu
GO_EXE=/absolute/path/to/go1.27.2/bin/go
GOOO_EXE=/absolute/path/to/gooo
NEW_OUT=/absolute/existing/parent/new-etag-run
GOTOOLCHAIN=local GOWORK=off GOENV=off GOPROXY=off GOSUMDB=off \
  "$GO_EXE" run -mod=readonly . \
  --compiler "$GOOO_EXE" --go "$GO_EXE" --out "$NEW_OUT"
```

The checker generates fresh Go expectations from the unchanged input-only
corpus, then makes seven `package execute --cases` calls: scanner192 rows in
55/15/122 partitions, strong comparison31 in28/3, and Report24 in19/5. It compares
all247 original inputs and727 returned scalar values, exact record fields and
types, current source/entry bindings, clean compiler identity, explicit Go tool
identity, and the two internal native executions in each fresh CLI call. It
never substitutes the Go oracle for a Gooo result. No saved-receipt replay or
engine canonical hash implementation is included.

Local validation of the starting source observed those247 comparisons passing
with the pinned compiler/Go, plus the optional19-row CRLF mutation witness.
These are finite local observations. The exported CI has not yet been run;
future edits, other platforms, production safety and execution speed are not
established by that observation.

## Edit the Gooo bodies

Keep the public package/activity/entity signatures and record fields, then edit
`computes` in `ports/net/http/etag.gooo`, `ports/net/httpguts/headers.gooo` or
`examples/strong-validator-facts/main.gooo`. Rerun with a new output path.
Current source bytes are snapshotted before/after; old body hashes do not block
edits. The frozen247 rows preserve IDs, order and duplicate occurrences.

For the original Report expression, add `--mutation-check`. After247 baseline
rows pass, it removes only `header_valid` from `single_strong` in a separate
workspace, reusing the first19 Report cases. One extra CLI call must expose the
actual CRLF row6 `single_strong` change from false to true, with all other fields
unchanged. A CLI exit status alone is insufficient; the negative CLI can return
zero with a non-PASS decision. This checks19 mutated rows, not247. The optional
literal mutation is unavailable after rewriting that expression; the normal
baseline still allows equivalent computes edits under the fixed public ABI.

## Scope and evidence

Inputs remain Unicode-scalar validUTF8 Text of0..1024 bytes each, including empty,
NUL, controls and invalid grammar. Strong operands have independent bounds.
The caller does not trim, normalize, repair or truncate data. The Go scanner's
edge trimming is separate from raw header validation. The thirteen facts do
not implement full If-Match, resource authenticity, permission to write, HTTP
effects or an atomic conditional update.

Each child command has90 seconds, stdout1MiB and stderr256KiB limits; the checker
has240 seconds overall. Handled signals, timeout and overflow terminate its own
child process group and wait. Raw streams, process records, fresh cases and the
first failure remain in a newly created output directory. Current source/tool
bytes and actual pinned Go/x/net oracle source files are checked before/after.
The checker never deletes directories or shared caches and does not retry.
Inspect and manage your own retained evidence. See `NOTICE`, `LICENSE` and
`third_party/GO-LICENSE` for the original source notices and dependency pins.
