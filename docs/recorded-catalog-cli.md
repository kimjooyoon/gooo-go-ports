# Use recorded-catalog-cli-preview.1

This versioned guide retains the preview.1 download and its alias, collection
context and observation-availability workflow. That executable does not support
`--definition-snapshot`. To compare saved tool definitions without host aliases,
use the [current saved-page guide](ai-agents.md#compare-your-own-saved-pages).
The [task guide](../README.md#choose-the-next-useful-task) links the available
routes; follow the instructions for the actual version you use.

`gooo-recorded-catalog` compares saved tool catalogs using rules written in Gooo.
It helps distinguish a changed tool origin from incomplete pages or observations
that were never recorded. The standalone executable needs no Go or Gooo
installation, model, account or network connection to compare local files.

Use the [recorded-catalog-cli-preview.1 archive](https://github.com/kimjooyoon/gooo-go-ports/releases/tag/recorded-catalog-cli-preview.1)
for Linux x86-64 or Apple Silicon macOS. This is a separate command; the
service-grant CLI and MCP preview bundles do not contain it. The
[source build](../examples/recorded-catalog-comparison/README.md#try-a-synthetic-example)
remains available with Go 1.27.3.

## Try an extracted archive

The following selects the matching archive, verifies its adjacent SHA-256
checksum, and extracts it into a fresh directory. These download commands need
network access; comparison of local files does not. Read `START-HERE.txt` after
extraction. No archive is provided for other operating systems or architectures.

```sh
case "$(uname -s)-$(uname -m)" in
  Linux-x86_64) platform=linux-amd64 ;;
  Darwin-arm64) platform=darwin-arm64 ;;
  *) printf '%s\n' 'No archive is provided for this platform.' >&2; exit 1 ;;
esac
release=https://github.com/kimjooyoon/gooo-go-ports/releases/download/recorded-catalog-cli-preview.1
asset="gooo-recorded-catalog-$platform.tar.gz"
work_dir="$(mktemp -d)" && cd "$work_dir" || exit 1
curl --fail --location --proto '=https' --tlsv1.2 --output "$asset" "$release/$asset" || exit 1
curl --fail --location --proto '=https' --tlsv1.2 --output "$asset.sha256" "$release/$asset.sha256" || exit 1
case "$platform" in
  linux-amd64) sha256sum -c "$asset.sha256" || exit 1 ;;
  darwin-arm64) shasum -a 256 -c "$asset.sha256" || exit 1 ;;
esac
tar -xzf "$asset" || exit 1
./gooo-recorded-catalog --version
./gooo-recorded-catalog --example > synthetic-observations.json
./gooo-recorded-catalog < synthetic-observations.json > report.json
```

The built-in example changes the host connection behind `docs_search`. Its
expected alias result is `ORIGIN_CHANGED`, with `RECORDED_CONTEXT_COMPLETE` for
the supplied collection. It is explicitly synthetic.

## Keep unavailable information explicit

### Assemble saved frame files

If your host saved each request and response in separate files, first use the
[local frame assembler](../tools/assemble_saved_frames.py) with Python 3. Prepare
a complete saved-observation plan in the [existing input format](../examples/recorded-catalog-comparison/README.md#provide-your-saved-observations),
using your retained host records for metadata and exposed aliases. In **every**
page, replace `request_json` and `response_json` with these file references:

```json
{
  "request_file": "frames/page-01.request.json",
  "response_file": "frames/page-01.response.json"
}
```

This fragment shows only the replacement fields; keep the page's `context` and
`declared_exposed_names`, and the complete before/after observations. Relative
paths resolve from the plan file's directory. Absolute paths are also accepted.
Keep the original files unchanged while assembling.

```sh
if python3 -I -B /absolute/path/to/gooo-go-ports/tools/assemble_saved_frames.py \
  /absolute/path/to/capture-plan.json \
  > saved-observations.json 2> assembly.stderr; then
  printf '0\n' > assembly.exit
else
  command_status=$?
  printf '%s\n' "$command_status" > assembly.exit
  cat assembly.stderr >&2
  exit "$command_status"
fi
```

The assembler reads regular UTF-8 files, preserving their exact contents,
including whitespace, escapes and CRLF. It changes only the page's file-reference
fields; outer JSON formatting may change, while metadata number tokens are
retained without floating-point conversion. Each frame is limited to 64 KiB, and
the plan and assembled input are each limited to 1 MiB. Missing files, invalid
UTF-8, ambiguous plan keys, mixed inline/file fields or exceeded bounds fail
before any output is written to stdout. An output-device failure can
leave a partial file; check the saved exit status. Inner JSON-RPC validation stays
with `gooo-recorded-catalog`, so malformed frames are preserved for its diagnostic.

For a Go MCP host, collect stdio bytes below the SDK's decoder through
`IOTransport.Reader` and `Writer`, keeping request/response boundaries and actual
I/O failures. [Go SDK v1.8.0's `LoggingTransport`](https://github.com/modelcontextprotocol/go-sdk/blob/3f3b699b2b67e1ed033a63d6651671dab53c2d32/mcp/transport.go)
re-encodes decoded messages, so its log is a derived representation. A byte
difference alone does not establish a semantic difference. Host aliases and
backend, authorization and configuration revisions require their own retained
host records; `tools/list` does not supply those facts. The assembler handles
already saved files and makes no live collection or historical reconstruction.

### Mark missing observations

Follow the immutable source-format link in `START-HERE.txt`. Keep your own input separate from the synthetic example. The installation above changes into a new temporary directory, so replace the example path below with the **absolute path** to your original saved-observation JSON. Both commands read that same file; do not edit it between creating the roster and comparing it.

```sh
input='/absolute/path/to/saved-observations.json'
case "$input" in
  /*) ;;
  *) printf '%s\n' 'Set input to an absolute path.' >&2; exit 1 ;;
esac
if [ ! -f "$input" ] || [ ! -r "$input" ]; then
  printf '%s\n' 'The saved input is not a readable file; check its absolute path.' >&2
  exit 1
fi
if ./gooo-recorded-catalog --availability-template \
  2> availability.stderr < "$input" > availability.json; then
  printf '0\n' > availability.exit
else
  command_status=$?
  printf '%s\n' "$command_status" > availability.exit
  cat availability.stderr >&2
  exit "$command_status"
fi
```

Before running the next block in the same shell and directory, set each blank
`value_state` in `availability.json` to `SUPPLIED` or `UNOBSERVED` using retained
evidence. Preserve the other fields. The commands omit optional policy revisions;
leave those values `UNOBSERVED`. The [input guide](../examples/recorded-catalog-comparison/README.md#declare-unavailable-observations-explicitly)
explains how to supply them consistently to both commands when known. A current
server value cannot establish what was observed in the past.

```sh
if ./gooo-recorded-catalog --definitions --diagnostics \
  --availability availability.json \
  2> explicit-report.stderr < "$input" > explicit-report.json; then
  printf '0\n' > explicit-report.exit
else
  command_status=$?
  printf '%s\n' "$command_status" > explicit-report.exit
  cat explicit-report.stderr >&2
  exit "$command_status"
fi
```

`UNASSESSED` is a represented result, not a command failure. Exit 1 means an input, comparison or I/O failure; exit 2 means an argument error. The `.stderr` and `.exit` files preserve each command's diagnostic and status; on a nonzero exit, stop and report that failure instead of treating a partial or empty output as a decision. Input plus sidecar must be at most 1 MiB.

Keep the input, availability file and report together. The sidecar binds the
exact input bytes, including whitespace. Missing observations can leave a result
unassessed while naming the next observation needed. The command does not collect
a live catalog or authorize tool execution, and does not require human approval
to compute a result.

## What the archive records

The packaging step reuses the executable from the existing native consumer
verification. It checks the source roster and executable hash, then extracts and
tests that same executable without a toolchain on its runtime search path.
It adds no compiler build or new Gooo execution to packaging.

The manifest retains the source revision, generated-source bindings and binary
hash. This verifier builds with `-buildvcs=false`, so `--version` may show
`(devel)`, no embedded Git revision and no module checksum. Those observations
are preserved. Neither build metadata nor the packaging manifest authenticates
the producer. The finite synthetic checks establish the reported cases, not all
possible inputs or production safety.

If you try it, share a useful result, unsupported task or first failure through
the [existing feedback form](https://github.com/kimjooyoon/gooo-go-ports/issues/new?template=problem.yml).
Include whether the input was your own or the unchanged example, platform,
release tag `recorded-catalog-cli-preview.1`, archive name, `source_revision` from
`manifest.json`, command mode, observed result or first stderr diagnostic and
saved exit status, and the decision you wanted to make. `--version` may show
`(devel)`; retain that actual value and use the tag, archive and manifest source
revision to identify the distribution. Keep private captures and credentials local.
