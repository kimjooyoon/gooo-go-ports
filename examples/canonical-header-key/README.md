# Canonical header keys with validity and byte witnesses

`app/canonical-header-key:Report` composes the original Gooo canonical-key port
with the existing Header Facts program. An API automation can retain the
canonical name while independently requiring `name_valid`. Computing the eight
facts needs no per-request human approval, model or external effect.

For `fOO-bAr`, the report contains `Foo-Bar`, `changed: true` and
`name_valid: true`. For `x bad`, both Text fields remain `x bad`,
`changed: false`, `name_valid: false`, and the first bad byte is offset 1,
octet 32. Canonical output alone is therefore insufficient for a validity check.

## Run the importing Gooo program

After the [repository setup](../../README.md#run-and-edit-locally), use the pinned
Gooo `83928fa` compiler and Go 1.27.2 from the repository root. The bundled
`inputs.json` is a trusted, single-Text CLI document for `fOO-bAr`:

```sh
set -eu
result_dir=$(mktemp -d)
./.gooo package execute --json --go "$(go env GOROOT)/bin/go" \
  --inputs examples/canonical-header-key/inputs.json \
  canonical-header-key-report.workspace.json > "$result_dir/execution.json"
./.gooo package replay --json --go "$(go env GOROOT)/bin/go" \
  --receipt "$result_dir/execution.json" \
  --inputs examples/canonical-header-key/inputs.json \
  canonical-header-key-report.workspace.json > "$result_dir/replay.json"
printf 'Saved local observations: %s\n' "$result_dir"
```

Inspect `result.runtime.traces[0].deliveries[0].actual`. The `--inputs` route has
no independent reference expectation; its execution is an observation. Saved
replay binds the same original source and input bytes. To run only the port,
use `port-inputs.json` with `canonical-header-key.workspace.json`.

| Field | Meaning |
| --- | --- |
| `key` | Complete unchanged decoded input Text |
| `canonical_key` | Text returned by the imported Gooo canonical-key port |
| `changed` | Whether the two Text values differ |
| `name_valid` | Existing bounded nonempty ASCII token predicate |
| `input_bytes`, `canonical_bytes` | Exact UTF-8 byte lengths |
| `name_invalid_offset` | First invalid byte's zero-based offset, otherwise -1 |
| `name_invalid_octet` | That byte's value 0–255, otherwise -1 |

Empty Text is invalid but has no bad byte: both witnesses are -1. The
[contract](contract.json) fixes all field IDs, order and types. Offsets count
bytes, not Unicode code points. The original Text is retained untrimmed; only
the canonical field applies ASCII casing. There is no Unicode normalization or
header repair.

## Independent finite inputs

`corpus.json` contains 1,059 original ordered occurrences and 1,054 distinct
keys. The five repeated key occurrences are retained. The
[accounting](corpus-accounting.json) records thirteen contiguous partitions;
`manual20.json` contains the unchanged twenty manually authored scalar anchors.
The corpus itself has only IDs and original keys, without expected outputs.
The native verifier must obtain current expectations from the actual pinned Go
function and independently compare all eight diagnostic fields and saved replay.
Earlier private observations do not substitute for fresh public validation.

Each key must be explicitly present decoded valid UTF-8 Text of at most 1,024
bytes. These trusted fixtures do not add an external JSON decoder. A host must
refuse unsupported transport/domain input before calling Gooo. In-domain invalid
header names are ordinary diagnostic inputs. No HTTP framing, request authority,
field occurrence policy, sending or production safety is established.
