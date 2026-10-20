# Original lexical path alias facts

Check whether two supplied path spellings collapse to the same slash path while
keeping both originals. For example, `/api//v1/../v2` and `/api/v2` return
`lexical_alias: true`. This can inform a duplicate policy-key diagnostic when the
host has established that its namespace uses these slash semantics.

`app/path-alias-facts:Report` imports the Gooo `path.Clean` and `path.IsAbs`
adaptations. Its eleven required fields retain both original Text inputs, both
cleaned values, both original absolute markers, two change markers, exact
equality, clean equality and `lexical_alias` (`same_clean && !exact_same`).
The [contract](contract.json) fixes all field identities and ordering.

An automated API policy diagnostic can attach `lexical_alias` and both original
paths to a duplicate-key warning instead of silently replacing one supplied path
with another. No human approval is required to calculate these facts. The host
must establish that its routing or policy-key namespace actually uses these slash
semantics and bind the observations to the same current context. This program
neither authenticates that context nor grants access. Equal clean values are not
authorization, application identity, filesystem equivalence or a safe-path proof.

Both inputs must be present decoded valid UTF-8 Text of at most 1,024 bytes each.
Slash semantics apply on every OS. Backslash, percent escapes, NUL, Unicode and
lookalike slash characters are retained; there is no URL decoding, Unicode
normalization, Windows path handling, symlink lookup or filesystem operation.
Longer values and arbitrary invalid-UTF-8 Go strings are outside the adaptation.
Strict fixture validation is separate from bare CLI Unicode decoding.

## Run and inspect three pairs

Complete the [repository setup](../../README.md#run-and-edit-locally), then run
these commands from the repository root in the same shell with the pinned
compiler and Go 1.27.3. Stop if a command fails. The new output directory keeps
these receipts separate from earlier runs:

```sh
(
set -eu
mkdir -p out
mkdir out/path-alias-example
./.gooo package execute --json --go "$(go env GOROOT)/bin/go" \
  --inputs examples/path-alias-facts/clean-inputs.json path-clean.workspace.json \
  > out/path-alias-example/clean-execution.json
./.gooo package replay --json --go "$(go env GOROOT)/bin/go" \
  --receipt out/path-alias-example/clean-execution.json \
  --inputs examples/path-alias-facts/clean-inputs.json path-clean.workspace.json \
  > out/path-alias-example/clean-replay.json
./.gooo package execute --json --go "$(go env GOROOT)/bin/go" \
  --inputs examples/path-alias-facts/inputs.json path-alias-facts.workspace.json \
  > out/path-alias-example/report-execution.json
./.gooo package replay --json --go "$(go env GOROOT)/bin/go" \
  --receipt out/path-alias-example/report-execution.json \
  --inputs examples/path-alias-facts/inputs.json path-alias-facts.workspace.json \
  > out/path-alias-example/report-replay.json
)
```

The supplied Report inputs produce these lexical facts. `left` and `right`
retain the raw strings; each actual record contains all eleven contract fields.

| `left` | `right` | `left_clean` | `right_clean` | `lexical_alias` |
| --- | --- | --- | --- | --- |
| `/api//v1/../v2` | `/api/v2` | `/api/v2` | `/api/v2` | true |
| `a/b/..` | `a` | `a` | `a` | true |
| `/account/A` | `/account/B` | `/account/A` | `/account/B` | false |

Open `out/path-alias-example/report-execution.json` and inspect
`result.runtime.traces[].deliveries[].actual`; the replay uses the same field
path. If Python 3 is already available, this optional standard-library command
prints the three full records without installing a package:

```sh
python3 - <<'PYTHON'
import json
with open("out/path-alias-example/report-execution.json") as file:
    receipt = json.load(file)
for trace in receipt["result"]["runtime"]["traces"]:
    for delivery in trace["deliveries"]:
        print(json.dumps(delivery["actual"], ensure_ascii=False))
PYTHON
```

These input-only receipts record observations. Their results do not themselves
constitute an independent Go comparison or permission to use a path.

## Change a copied input

Keep the original fixture and saved receipts. Create a new input copy:

```sh
(
set -eu
mkdir out/path-alias-changed
cp examples/path-alias-facts/inputs.json out/path-alias-changed/inputs.json
)
```

In that new copy, replace only the first object inside `inputs` with the third
supplied pair shown below. Keep the other two objects and the outer schema:

```json
{
  "app/path-alias-facts:Report.input0": "/account/A",
  "app/path-alias-facts:Report.input1": "/account/B"
}
```

The first result now uses that existing non-alias pair: `lexical_alias` changes
from true to false. The other two results keep their original inputs and facts.
Create a fresh execution record for the changed inputs, then replay that new
record with the same changed inputs:

```sh
(
set -eu
./.gooo package execute --json --go "$(go env GOROOT)/bin/go" \
  --inputs out/path-alias-changed/inputs.json path-alias-facts.workspace.json \
  > out/path-alias-changed/report-execution.json
./.gooo package replay --json --go "$(go env GOROOT)/bin/go" \
  --receipt out/path-alias-changed/report-execution.json \
  --inputs out/path-alias-changed/inputs.json path-alias-facts.workspace.json \
  > out/path-alias-changed/report-replay.json
)
```

The earlier replay commands use the original inputs; the new replay uses the
changed copy and its own saved execution. Keep the original fixture and receipts.
For the optional inspection command, use the new `report-execution.json` path.

## Verification scope

`corpus-120.json` contains input pairs only. Its first 24 anchors preserve empty,
root/dot/parent behavior, backslash, percent, Unicode, NUL and exact 1,024-byte
boundaries. Another 96 deterministic pairs probe slash/dot/backtracking spelling
changes. Not every pair is expected to be an alias. Independent actual Go
`path.Clean` and `path.IsAbs` calls supply the reference facts; expected outputs
are not copied from the Gooo implementation or historical receipts.

The normal verifier appends two suites: 9,841 standalone Clean inputs and these
120 pairs. Every case requires fresh native execution and saved replay. Report
pairs additionally require all eleven fields and both original inputs. This is separate from the existing
three-entry `Ext`/`IsAbs`/six-field `PathFacts` sidecar, which remains unchanged.
Finite fixture counts and source partition checks are not completed native or
hosted CI acceptance, full-domain compatibility or a performance claim.

This original composition uses the repository BSD license. Its imported
[Clean adaptation](../../ports/path/README.md) retains Go BSD attribution and
immutable source pins.
