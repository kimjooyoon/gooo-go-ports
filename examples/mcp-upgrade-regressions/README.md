# Compare saved definitions during an MCP upgrade

A saved `tools/list` response can help explain what changed during an upgrade or
reconnect. These three synthetic examples compare one tool name without needing
a host alias registry.

| Case | Saved observation | Expected `decision.state` |
| --- | --- | --- |
| `output-schema-string-to-number` | `outputSchema.properties.value.type` changes from `string` to `number`. | `OBSERVED_DEFINITION_CHANGED` |
| `same-definition-version-context-only` | The descriptor stays the same; fictional server version labels change only in `context.json`. | `OBSERVED_DEFINITION_SAME` |
| `missing-after-incomplete-page` | The tool is absent from the retained after page, but its `nextCursor` points to a page that was not saved. | `UNASSESSED` |

From the repository root, with Go **1.27.3** installed, run this source checkout.
Older released binaries do not include `--definition-snapshot`.

```sh
report_dir="$(mktemp -d)" || exit 1
for case_id in \
  output-schema-string-to-number \
  same-definition-version-context-only \
  missing-after-incomplete-page
do
  if GOTOOLCHAIN=local GOWORK=off \
    go -C examples/recorded-catalog-comparison run ./cmd/compare --definition-snapshot \
    < "examples/mcp-upgrade-regressions/$case_id/snapshot-input.json" \
    > "$report_dir/$case_id.json" 2> "$report_dir/$case_id.stderr"; then
    printf '%s: %s\n' "$case_id" "$report_dir/$case_id.json"
  else
    status=$?
    cat "$report_dir/$case_id.stderr" >&2
    exit "$status"
  fi
done
```

Open each report and compare its `decision` with that case's `expected.json`
anchors and the table above. `UNASSESSED` is a completed comparison with limited
observations, so this case also returns exit zero. Retain stderr and the exit
status for a command failure.

Each case includes the four original, locally authored request/response files
and the assembled `snapshot-input.json`. Keep their exact text when making your
own input; [the input guide](../tool-definition-snapshot/README.md#try-the-separate-saved-page-input)
explains the page format. `context.json` is explanatory synthetic context and is
not passed to the comparator. `observed-decision.json` records the original local
run; [packet.json](packet.json) identifies that source revision and payload hashes.
The original native receipts are not distributed here, and a current-source run
needs its own result.

These examples do not reproduce an SDK race or reconnect. A changed output
schema is an observed definition change, not proof of incompatible or changed
behavior. An incomplete after page cannot establish removal. Origin and
authorization remain unassessed, and no tool is called.

For [first-use feedback](https://github.com/kimjooyoon/gooo-go-ports/issues/new?template=problem.yml),
choose **Native setup, verification, or replay**. Include the command, whether
you used these unchanged examples or your own saved pages, the result or first
error, and the upgrade decision that still needs support. Keep private captures
local; a short description or synthetic reproduction is enough.
