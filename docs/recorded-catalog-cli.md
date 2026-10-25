# Compare saved MCP catalogs without a toolchain

`gooo-recorded-catalog` compares saved tool catalogs using rules written in Gooo.
It helps distinguish a changed tool origin from incomplete pages or observations
that were never recorded. The standalone executable needs no Go or Gooo
installation, model, account or network connection to compare local files.

This is a separate command. The service-grant CLI and MCP preview bundles do not
contain it. Native archive publication is being prepared; there is no published
release download for this command yet. The [source build](../examples/recorded-catalog-comparison/README.md#try-a-synthetic-example)
remains available with Go 1.27.3.

## Try an extracted archive

Use the archive for your operating system and architecture. Verify its adjacent
SHA-256 checksum before extracting into a fresh directory, then read
`START-HERE.txt`. When packaging succeeds, its candidate appears among that run's
artifacts in the [verification workflow](https://github.com/kimjooyoon/gooo-go-ports/actions/workflows/ci.yml).
An artifact is not a release.

From the extracted directory:

```sh
./gooo-recorded-catalog --version
./gooo-recorded-catalog --example > synthetic-observations.json
./gooo-recorded-catalog < synthetic-observations.json > report.json
```

The built-in example changes the host connection behind `docs_search`. Its
expected alias result is `ORIGIN_CHANGED`, with `RECORDED_CONTEXT_COMPLETE` for
the supplied collection. It is explicitly synthetic.

## Keep unavailable information explicit

For your own saved input, create an availability roster:

```sh
./gooo-recorded-catalog --availability-template \
  < saved-observations.json > availability.json
```

Using the retained evidence, set every `value_state` to `SUPPLIED` or `UNOBSERVED`.
Keep the other fields unchanged. Optional policy revisions omitted from both
commands stay `UNOBSERVED`; the [input guide](../examples/recorded-catalog-comparison/README.md#declare-unavailable-observations-explicitly)
explains how to supply them when known. A current server value cannot establish
what was observed in the past.

```sh
./gooo-recorded-catalog --definitions --diagnostics \
  --availability availability.json \
  < saved-observations.json > explicit-report.json
```

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

If a first run fails, preserve the first stderr diagnostic and exit code. Report
the archive/source revision, platform, command mode and whether the input was
your own or the unchanged example through the
[existing feedback form](https://github.com/kimjooyoon/gooo-go-ports/issues/new?template=problem.yml).
Keep private captures and credentials local. A useful result or an unsupported
task is useful feedback too.
