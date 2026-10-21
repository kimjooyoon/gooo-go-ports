# Review tool exposure across a source upgrade

An unchanged tool selector can select new tools after an upgrade. This experiment
uses Gooo to explain that difference for two immutable `openclaw/gogcli` source
catalogs, while retaining each tool's presence, service, risk and capability.

The supplied selected policy is `gmail.*` with ordinary writes enabled, separate
send/delete capabilities disabled, read-only mode disabled and no runtime
selector. The reviewed source catalog grows from 10 to 27 tools. Under that same
policy, selected tools grow from 3 to 16: 13 newly listed tools are selected,
while four new send/delete tools remain unselected. This is documented upstream
behavior, not a vulnerability report or an observed live account configuration.

The [Gooo comparison](main.gooo) imports the
[ported Gooo selection kernel](../../ports/gogcli/tool_selection.gooo). Go owns
source metadata extraction and an independent reference calculation; the
comparison and selection behavior execute in Gooo. The two source revisions are
[`5784409`](https://github.com/openclaw/gogcli/tree/578440918818e24aec6291cfe78cbb915091d7ae)
and [`ec34fed`](https://github.com/openclaw/gogcli/tree/ec34fedc23dd9d3e90f0401144cec42e06a50783).
Original source bytes and MIT notices are in `reference/upstream/` as `.go.txt`
files so the repository's Go build does not compile a second upstream project.
[source-pin.json](source-pin.json) binds every retained file to its revision,
Git blob ID and raw SHA-256.

## Run both reviewed profiles

Use an already-built compiler matching the checkout's `upstream.json` and a
matching Go executable. The root `go.mod` and the compiler identity must agree
with those pins. Python 3.9+, Bash and a local Go toolchain are required.
Repository compiler setup is separate; this verifier downloads nothing.

From the repository root, supply absolute paths and a fresh output directory
outside the checkout whose parent already exists:

```sh
bash examples/tool-catalog-change/verify.sh \
  /absolute/path/to/pinned-gooo \
  /absolute/path/to/matching-go/bin/go \
  /absolute/new/tool-catalog-observation
```

The script first validates the actual compiler identity against root facts. It
then regenerates the metadata of both source catalogs, computes separate Go
reference results, and executes and replays the importing Gooo program. It runs
two profiles:

| Profile | Inputs | Checked result |
| --- | --- | --- |
| `catalog` | All 27 source-derived tool entries | The same supplied policy selects 3 before and 16 after; 13 additions and no removals. |
| `controls` | 20 synthetic cases | Read-only and write ceilings, separate send/delete capabilities, unknown capabilities, selector intersection, presence changes and metadata changes. |

Both transport formats are generated together from [case-facts.json](case-facts.json).
`make-inputs.py` validates both profiles before creating the output directory,
then writes four input files under `inputs/`. Its `generation.provenance.jsonld`
records source and output hashes, explicit PROV-O node references and a distinct
actual generation activity. This records input serialization; the native
execution and result checks remain separate stages. Root compiler and Go pins
are read from the checkout's existing facts rather than copied into this example.

Each result contains 14 typed fields. The checker compares every field with the
Go reference, links the replay to the exact execution bytes, and checks the
manifest, input, both Gooo sources and producer identities. Keep all records in
the output directory. A failure stops at its first stage and preserves stdout
and stderr. Native receipts remain `OBSERVED`; the separate comparison establishes
finite agreement with the Go reference, without authenticating unsigned records.

Read `catalog-check.json` and `controls-check.json` for the summaries. The full
`catalog-execution.json` and `catalog-replay.json` retain the 27 tool reports.
`source-catalog.json` records the complete metadata reachable through the two
reviewed `mcpAllTools` expressions. The metadata projector recognizes a closed
source shape. Three factories with computed names are supported only at their
reviewed exact function hashes; an unknown or changed factory is refused.

## Interpret or extend a result

`ADDED_TO_SELECTED_CATALOG` means the supplied after-catalog entry is selected
where its before entry was absent or unselected. `REMOVED_FROM_SELECTED_CATALOG`
means the reverse. `REMAINS_SELECTED` and `REMAINS_UNSELECTED` preserve unchanged
selection. Keep `before_present` and `after_present` to distinguish a missing
tool from a tool excluded by a policy or capability.

These are source-catalog observations under supplied kernel inputs. Account
resolution, configuration normalization, baked command restrictions, runtime
registration and tool invocation are separate host operations. The program
performs no Google API call, account access or model call. It automatically
calculates the supplied facts without adding an approval step.

The checker accepts the two reviewed fixture profiles. For another policy or
catalog, add a separate profile with its source facts and independent Go expected
results. The source-extraction adapter must be reviewed when upstream structure
or a bound factory changes. See the port's
[domain and Go-to-Gooo differences](../../ports/gogcli/README.md).

For a useful first-use report, share a public synthetic task, platform, checkout
revision, the first failed stage and diagnostic, or a tool whose selection was
unexpected through the [existing feedback form](https://github.com/kimjooyoon/gooo-go-ports/issues/new?template=problem.yml).
