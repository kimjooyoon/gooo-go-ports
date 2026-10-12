# UUID canonical parsing example

Run Google's UUID parser return behavior through the Gooo `ParseCanonical`
Text adaptation. The result contains `parsed`, `canonical_uuid`, `error` and
`invalid_length`. Check `parsed` before using the returned text: an original
parse error can still return a nonzero UUID value with a correctly shaped string.

Use Git, a POSIX shell and a Go installation with toolchain selection support.
From this repository's root, build the pinned Gooo 0.6.32-dev compiler with
Go 1.27.2, then execute and replay the example:

```sh
export GOTOOLCHAIN=go1.27.2
export GOWORK=off
./tools/bootstrap.sh
mkdir -p out/uuid-parse-canonical
./.gooo package execute --json --inputs examples/uuid-parse-canonical/inputs.json \
  uuid-parse-canonical.workspace.json > out/uuid-parse-canonical/execution.json
./.gooo package replay --json --receipt out/uuid-parse-canonical/execution.json \
  --inputs examples/uuid-parse-canonical/inputs.json uuid-parse-canonical.workspace.json \
  > out/uuid-parse-canonical/replay.json
```

Leave `--go` unset so the native commands can select the installed matching
Go 1.27.2 executable, including the toolchain cache populated during setup.
Bootstrap preserves an older compiler checkout; follow the repository's
[upgrade guidance](../../README.md#run-and-edit-locally) for an existing checkout.

The 12 input examples include the four forms, uppercase hexadecimal, arbitrary
38-byte delimiters, raw and standard-form partial failures, an empty input and
a bad URN prefix. The input key is
`go/github.com/google/uuid/text:ParseCanonical`; returned records are at
`result.runtime.traces[].deliveries[].actual`.

These input-only receipts report `OBSERVED`, without independent expected
outputs. Saved replay executes the recorded program again without model calls.
For the full API mapping and input limits, read
[Google UUID canonical parsing](../../docs/google-uuid-parse-canonical.md).
