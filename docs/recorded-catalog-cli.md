# Compare saved MCP catalogs without a toolchain

`gooo-recorded-catalog` compares saved tool catalogs using rules written in Gooo.
It helps distinguish a changed tool origin from incomplete pages or observations
that were never recorded. The standalone executable needs no Go or Gooo
installation, model, account or network connection to compare local files.

Use the [experimental standalone release](https://github.com/kimjooyoon/gooo-go-ports/releases/tag/recorded-catalog-cli-preview.1)
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
