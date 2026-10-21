# Compare URL component contexts in Gooo

The original Gooo `app/url-context-facts:Report` preserves one input Text and
returns both a path-segment encoding and a query-value encoding. It imports
[`PathEscape` and `QueryEscape`](../../ports/net/url/README.md); all ten result
fields are calculated in Gooo. An independent Go oracle checks the results.

Choose the encoding for the component you are building:

| Original Text | `path_segment` | `query_value` |
| --- | --- | --- |
| `a b` | `a%20b` | `a+b` |
| `a+b` | `a+b` | `a%2Bb` |
| `%20` | `%2520` | `%2520` |

These contexts occur in real Go code: pinned `google/go-github` uses
[`url.PathEscape(env)` for an environment-name path segment](https://github.com/google/go-github/blob/e754d2c70ded9a27fce4973505fa2999a01f5a7e/github/actions_variables.go#L151)
and [`url.QueryEscape(message)` for the Octocat query value](https://github.com/google/go-github/blob/e754d2c70ded9a27fce4973505fa2999a01f5a7e/github/meta.go#L172).
These are workload examples, without implying that either project uses Gooo.

## Run one input and save replay

Complete the [repository setup](../../README.md#run-and-edit-locally) with Go
**1.27.3** and the pinned Gooo compiler revision
`205896a413e5dea92366309c35ae85e542d055f6`. From the repository root:

```sh
set -eu
umask 077
export GOTOOLCHAIN=go1.27.3 GOWORK=off
GOOO_COMPILER="${GOOO_COMPILER:-$PWD/.gooo}"
GOOO_GO="$(go env GOROOT)/bin/go"
run_dir="$(mktemp -d "${TMPDIR:-/tmp}/gooo-url-context.XXXXXX")"
printf '%s\n' '{"text":"a+b"}' | \
  "$GOOO_GO" run ./cmd/url-component-input > "$run_dir/inputs.json"
"$GOOO_COMPILER" package execute --json --go "$GOOO_GO" \
  --inputs "$run_dir/inputs.json" url-context-facts.workspace.json \
  > "$run_dir/execution.json"
"$GOOO_COMPILER" package replay --json --go "$GOOO_GO" \
  --receipt "$run_dir/execution.json" --inputs "$run_dir/inputs.json" \
  url-context-facts.workspace.json > "$run_dir/replay.json"
printf 'Saved execution and replay in %s\n' "$run_dir"
```

The adapter accepts a JSON object with exactly one `text` string and at most
**16 KiB of raw JSON**. It rejects missing, duplicate or
unknown keys, wrong types, malformed JSON or UTF-8, unpaired surrogate escapes,
and Text over **341 decoded UTF-8 bytes** before writing the binding document.
Accepted Text consists of Unicode scalars; empty Text is supported. It is
preserved without trimming, normalization, decoding or truncation. The adapter
does not calculate an encoded result or execute Gooo.

Report has one Text input. The generated envelope binds it to the bare activity
key `app/url-context-facts:Report`, with no `.input0` suffix. The raw Gooo entry
and imported activities require this domain as a caller precondition; use the
adapter when accepting external JSON.

The result record contains:

| Fields | Meaning |
| --- | --- |
| `original` | Unchanged input Text |
| `path_segment`, `query_value` | The two component encodings |
| `input_bytes`, `path_bytes`, `query_bytes` | UTF-8 byte lengths |
| `path_changed`, `query_changed` | Whether each result differs from the input |
| `contexts_differ` | Whether the two encodings differ |
| `original_percent_offset` | Zero-based byte offset of the first literal `%` in the original, or `-1` if absent |

## Check fresh execution

After selecting the tools above, use another fresh output directory:

```sh
python3 -I -B tools/check-url-components.py \
  --root "$PWD" --compiler "$GOOO_COMPILER" --go "$GOOO_GO" \
  --inputs "$run_dir/inputs.json" \
  --out "$run_dir/verification"
```

This checks the input just supplied; omit `--inputs` to check the default
128-input corpus instead.

The checker compares finite inputs with an independent Go reference and checks
saved Gooo replay. Inspect that run's retained results and first diagnostic;
historical receipts do not establish the result of changed sources. This
sidecar remains separate from the existing native differential catalog.

The **0..341-byte** input bound allows at most **1,023 output bytes per encoded
Text**. It is this port's bound, not a GitHub API limit. Arbitrary-byte Go strings
and longer inputs are outside its domain. The report does not parse or unescape
URLs, choose a destination context, validate hosts or schemes, authenticate,
authorize, or send an HTTP request. Finite comparisons do not establish
whole-library compatibility or all-input correctness. See [NOTICE.md](NOTICE.md)
for source identity and licenses.
