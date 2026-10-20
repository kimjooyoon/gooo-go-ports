# Edit a Gooo policy and compare its decisions

Change `after.gooo`, evaluate the same supplied facts with both versions, and
keep each result attached to its own source and official saved replay. This is
an experimental offline example, not live authorization or a trace importer.
The existing sealed API-policy consumer and Action remain separate.

Start in the repository root with the pinned **Gooo compiler83928fa** and
**Go1.27.3** already prepared; see [setup](../../README.md#run-and-edit-locally).
The script uses Python3's standard library and an existing repository process
helper. It does not install tools, download modules, or build an engine.
Use canonical absolute tool/output paths and a new output directory whose
parent exists. On macOS/Linux, for example:

```sh
(
  set -eu
  fixed_go="$(GOTOOLCHAIN=go1.27.3 go env GOROOT)/bin/go"
  python3 -I -B examples/editable-api-policy/run.py \
    --root "$PWD" --compiler "$PWD/.gooo" --go "$fixed_go" \
    --out "$HOME/gooo-policy-demo-1"
)
```

With the supplied policies, expect `UNCHANGED`, `NEWLY_DENIED`, `UNCHANGED`.
Only the ordinary user's `write` changes `allowed` and `reason` to
`WRITE_REQUIRES_DEPLOY_BOT`; a read and the deploy bot's write remain allowed.
Both complete fifteen-field decisions and changed field names are saved in
`summary.json`. Raw command stdout, stderr, arguments and process status remain
in separate subdirectories. These files include the original supplied facts;
review them before sharing.

After all existing checks and cleanup succeed, `source-comparison.json` joins
the same complete fifteen-field comparisons to the before/after source and
workspace hashes, the two common source files, and all four execution/replay
records. Each record has a relative output path, raw-file SHA256 and its replay
parent where applicable. `summary.json` references this local file by relative
path, byte count and raw SHA256. Raw source hashes are labelled separately from
the receipts' JSON-string source digests; these are different representations.
The policy-source unified diff retains CRLF and marks a missing final LF. Diff
computation is limited to128KiB combined source bytes and2,000 combined LF-based
lines. Larger edits retain full hashes with `NOT_RENDERED_LIMIT`; identical
sources report `IDENTICAL`. This is a display index, not a semantic proof or
permission to perform an action. It is not printed or uploaded automatically.
Review private source as well as facts before sharing any output.

Now edit the condition or reason in `after.gooo`, keeping the same nine scalar
inputs, fifteen-field record and call to `original.APIAccess`. For example,
replace `synthetic:deploy-bot` with `synthetic:user` and change the reason to
`WRITE_REQUIRES_USER`, then run the same command with **a different fresh
`--out`**. The user write now stays allowed and the deploy bot write is denied.
Keep reason labels consistent with your rule: the runner reports the literal
reason and does not infer its meaning. The actual comparison is accepted without
forcing the original demo outcome. `--check-demo` additionally checks the
separate original three-row literals and is intended for the unchanged demo
and CI; omit it after editing the rule.

## Supply your own named facts

Use `--facts /absolute/path/facts.json` to avoid assigning your values to
`input0` through `input8` by hand. The root is a JSON array of **one to three
objects**, each with exactly these nine fields. For example, save this public
synthetic write request as `facts.json`:

```json
[
  {
    "group_present": true,
    "group_value": "admin",
    "principal_present": true,
    "principal_value": "synthetic:user",
    "authenticated": true,
    "caller_tenant": "tenant-a",
    "resource_tenant": "tenant-a",
    "requested_action": "write",
    "allowed_action": "write"
  }
]
```

After editing `after.gooo`, use the same prepared tools and a fresh output:

```sh
(
  set -eu
  fixed_go="$(GOTOOLCHAIN=go1.27.3 go env GOROOT)/bin/go"
  python3 -I -B examples/editable-api-policy/run.py \
    --root "$PWD" --compiler "$PWD/.gooo" --go "$fixed_go" \
    --facts "$PWD/facts.json" --out "$HOME/gooo-policy-own-facts-1"
)
```

`--facts` and the existing native-envelope `--inputs` option are mutually
exclusive. Names map to the existing nine scalar positions; this adapter does
not evaluate policy, authenticate facts, fill absent values, trim or normalize
Text. The three presence/authentication fields must be Boolean, and the other
six must be Unicode-scalar UTF8 Text, each at most1,024bytes. Inactive Text values
are still required and preserved. Missing, unknown or duplicate keys are errors.
Both the original facts file and the converted native JSON, including its final
LF, must fit30,000bytes; JSON escaping can make the latter larger. Row order and
duplicates are preserved. The generated `facts-inputs.json` stays in the output,
and both input files' hashes are recorded in `summary.json`'s source/tool pins.
The same generated file is used by each policy's execution and its own replay.
CI supplies the same three predeclared demo cases through `demo-facts.json` and
`--facts`, keeping the existing full execution, own replay and demo assertions.
Declare your expected allowed/denied cases before running; inspect all changed
fields or send the first failure through the existing feedback form below.
Omit `--check-demo` when intentionally changing the facts or rule.

## Extract a helper with the separate source-inspection profile

This opt-in release profile retains its exact Go 1.27.2 compiler/SDK identity.
It is separate from the current-source Go 1.27.3 migration target above; the
old archive and saved observations have not become Go 1.27.3 builds.
Reproduce the following frozen release recipe with the [pre-migration source
snapshot](https://github.com/kimjooyoon/gooo-go-ports/tree/cc946350de37c48d509b1d383b8ce7e8a0062483) in a separate checkout.

The default compiler/profile keeps its existing one-activity-per-source check.
To extract a rule into a named helper, select **`--compiler-profile
source-inspection036`** and supply the official **Gooo0.6.36-dev** compiler at
source revision `31497af3beaef210b5a075867b0de76cf55ee3ca`. This is an explicit
consumer option; the repository-wide compiler pin remains83928fa.

In `after.gooo`, replace
`base.effective_principal != "synthetic:deploy-bot"` with
`!IsDeployBot(base.effective_principal)`, and append this declaration in the same
file. Keep the existing `EvaluateAPI` entry, imports and record unchanged:

```gooo
activity IsDeployBot(Text) -> Boolean id "gooo://editable-api-policy/is-deploy-bot/v1" computes `
return input == "synthetic:deploy-bot"
`
```

Prepare the matching [official release](https://github.com/kimjooyoon/meta-ontology-go/releases/tag/v0.6.36-dev)
for your platform. The following download is specifically **Linux amd64** and
checks that archive's SHA256 before extraction; Go1.27.2 must already be present:

```sh
(
  set -eu
  gooo_source_tools="$(mktemp -d "${TMPDIR:-/tmp}/gooo-source-inspection036.XXXXXX")"
  gooo_source_archive="$gooo_source_tools/gooo-linux-amd64.tar.gz"
  curl --fail --location --output "$gooo_source_archive" \
    https://github.com/kimjooyoon/meta-ontology-go/releases/download/v0.6.36-dev/gooo-linux-amd64.tar.gz
  printf '%s  %s\n' \
    9dbd1c08b855a3c27ae43a8050531afaba724d90b24d4f72041db3a8a4a285b0 \
    "$gooo_source_archive" | sha256sum --check --strict
  test "$(tar -tzf "$gooo_source_archive")" = gooo
  tar -xzf "$gooo_source_archive" -C "$gooo_source_tools" gooo
  fixed_go="$(GOTOOLCHAIN=go1.27.2 go env GOROOT)/bin/go"
  python3 -I -B examples/editable-api-policy/run.py \
    --root "$PWD" --compiler "$gooo_source_tools/gooo" --go "$fixed_go" \
    --compiler-profile source-inspection036 \
    --out "$HOME/gooo-policy-helpers-1" --check-demo
)
```

The runner checks the clean compiler build identity, exact source revision,
unreplaced SDK `v0.2.51-experimental`, Go `go1.27.2` identity and selected Go
binary hash. A fresh `package inspect --json` for each version must report
`PREPARED`, zero program executions and zero model predictions. Every execute
and saved replay must carry exactly that inspected program, including current
source images, activity IDs and body hashes. The source-inspection summary
records `prepared_inspections:2` and `package_commands:7`; its four positive
receipts remain input-only `OBSERVED` records.

Helpers and unused helpers in these selected sources are allowed in this
profile; extra imports or a different manifest, entry or nine-input/fifteen-field
ABI are not. For example, adding this unused helper to `after.gooo` should leave
the three supplied decisions unchanged. Use a fresh output directory on each run:

```gooo
activity UnusedHelper(Text) -> Boolean id "gooo://editable-api-policy/unused-helper/v1" computes `
return input == "unused"
`
```

`--check-demo` still applies to this behavior-preserving extraction. Omit it if
you deliberately change the rule. A control using the original `after.gooo`
and the helper-extracted version preserved all fifteen fields for the same three
inputs and their saved replays; comparing `before.gooo` with `after.gooo` also
includes the intentional write-rule change and cannot isolate extraction.
Prepared call plans come from the trusted compiler, not an independent semantic
oracle. The control retained46 `UNMAPPED` target/value source components out
of176 in both variants, with reason `SELECTED_RANGE_UNAVAILABLE`; matching
missingness does not establish complete field provenance or equivalence for
other inputs. Supplied authentication facts are still not live authority.

`--inputs /absolute/path/facts.json` accepts one to three rows using the exact
`demo-inputs.json` schema: three Boolean facts and six Unicode-scalar UTF8 Text
facts per row, each Text at most1024bytes and the whole document at most30000bytes.
Missing/unknown/duplicate fields, wrong types, isolated surrogates and trailing
JSON are errors. Facts are not trimmed or repaired. Valid individual strings
can still exceed the compiler's joint translated-input budget and be refused.
Do not fill in redacted facts or missing rate-limit state; this example does
not support stateful traces.

Each version gets one input-only `execute` and its own official `replay`.
Each command performs two fresh native runs. Input-only receipts remain
`OBSERVED` with `finite_total:0`; comparison and optional literal checks happen
in this consumer. The ordered observed records are also matched to both native
runs' reported stdout hashes. Replaying the before receipt against the after
workspace must be refused at the envelope/manifest stage; that expected exit1
is recorded separately. This control checks different manifests and does not
by itself test the changed-source guard under the same manifest. Source and tool
hashes are taken from the current run, so editing the rule is allowed between
runs. Editing sources during a run is refused. Inner canonical verification
belongs to the official replay command. No saved private proof is rebound.

The runner limits each command to90seconds, the whole run to300seconds, owned
work to1GiB and retained evidence to64MiB, with2GiB free required at start and
before commands. It uses offline Go settings and private caches, preserves the
actual HOME, and removes only its own temporary work after observed processes
close. Existing output directories are refused. Cold cache preparation is part
of the observed run; no performance guarantee or OS network sandbox is claimed.

The [Gooo API composition](../cel-api-policy/main.gooo) and
[CEL scalar adaptation](../../ports/cel/scalar_access.gooo) are reused in place.
See [NOTICE](NOTICE.md), the [local Apache-2.0 license](../../ports/cel/LICENSE-APACHE-2.0)
and [original source attribution](../cel-api-policy/NOTICE.md).

If setup or an edit fails, [report the first failing command and visible error](https://github.com/kimjooyoon/gooo-go-ports/issues/new?template=problem.yml).
No successful run or receipt is required to report a problem. Describe one real
task this comparison would help with, and remove private facts from any logs.
