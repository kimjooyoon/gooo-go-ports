# Diagnose a deny rule within a declared tool surface

This original Gooo policy explains whether an agent tool deny rule warrants an
unmatched-rule warning. It distinguishes a complete inventory with no matches,
an incomplete inventory, a backend that cannot enforce the rule, and matching
tools already removed by earlier denials. The Gooo activity owns the decision;
the caller supplies the scoped facts.

A zero count alone is insufficient. For example, a tool removed during
construction should not make a working deny rule look inactive. This distinction
is motivated by the [OpenClaw unmatched deny discussion](https://github.com/openclaw/openclaw/issues/137470).
The example evaluates supplied facts and does not modify OpenClaw or permissions.

## Run the example

From the repository root, use Bash, Python 3, Git and the Go toolchain declared in
`go.mod`. Setup may download that toolchain and the pinned Gooo compiler.

```sh
export GOTOOLCHAIN="$(awk '$1 == "go" {print "go" $2}' go.mod)"
bash tools/bootstrap.sh
go_sdk="$(go env GOROOT)/bin/go"
scope_parent="$(mktemp -d "${TMPDIR:-/tmp}/gooo-scope.XXXXXXXX")"
scope_parent="$(cd "$scope_parent" && pwd -P)"
bash examples/deny-rule-scope/verify.sh "$PWD/.gooo" "$go_sdk" "$scope_parent/result"
```

`result/verification.json` reports 87 native cases, 87 saved replay cases and
nine rejected receipt mutations. `reference.json` contains the complete facts
and decisions. Preserve the execution, replay and input-generation records when
reproducing a result. Execution and replay are checked against a separate Go
reference; declaration checking alone is insufficient.

## Interpret the result

| Status | Meaning |
| --- | --- |
| `INVALID_FACTS` | Empty scope or rule identity, contradictory backend facts, an out-of-range count, or an inconsistent partition. |
| `UNRESOLVED_BACKEND` | The caller has not established the backend's enforcement contract. |
| `OUTSIDE_ENFORCEABLE_SURFACE` | The caller's known backend contract cannot enforce this rule. |
| `UNRESOLVED_INVENTORY` | A complete inventory before policy filtering is still needed. |
| `UNMATCHED_DENY` | The complete enforceable surface contains no matching tools; `warn_unmatched` is true. |
| `MATCHES_PRIOR_DENIALS` | Every matching tool is accounted for by an earlier or construction-time denial. |
| `MATCHES_CURRENT_SURFACE` | Matching tools remain at the inspected layer; the result does not assert execution-time enforcement. |

For the same zero-count input, changing `inventory_complete` from false to true
changes `UNRESOLVED_INVENTORY` to `UNMATCHED_DENY` only when the backend is known
and can enforce the rule. A caller must establish that fact before changing it.

## Supply host facts

Bind `surface_id` to the backend and inventory snapshot, and `rule_id` to the
expanded rule. Counts refer to distinct matching tool identities and must form
disjoint sets:

```text
remaining_matches + earlier_denied_matches + construction_denied_matches
    = prepolicy_matches
```

Each count is limited to 0 through 4096. The host owns raw glob matching, aliases,
group expansion, inventory discovery and the truth of this partition. An
arithmetic equality does not prove that the sets are disjoint or complete.
See [the input contract](contract.json) for each field.

The finite fixtures cover all eight boolean combinations and every three-way
partition with total 0 through 2, plus seven domain cases. They demonstrate
the scoped decision policy, not a live OpenClaw integration. Backend enforcement
and warning placement remain host responsibilities.
