# Automatic redelivery admission

This original Gooo activity decides what an API client may try next after an
operation was not sent, failed, succeeded, or has an unknown outcome. It returns
a decision, its reason, and the supplied facts. Verified cases can proceed
automatically; uncertain cases stop or refresh their evidence automatically.
No human approval is required by this policy.

The activity evaluates facts. An application must establish their authenticity
and freshness and atomically claim an eligible attempt before sending anything.

| Action | Meaning for the application |
| --- | --- |
| `STOP` | Do not claim or dispatch another attempt. Inspect the reason. |
| `REFRESH_OUTCOME` | Obtain current applicable evidence, then reassess. |
| `LOOKUP_RECORDED_RESULT` | Attempt an authorized lookup of the already applied operation's recorded result. A missing result may remain unresolved and never permits redelivery. |
| `REDELIVER_ELIGIBLE` | Attempt an atomic claim against the current ledger revision. Dispatch only after a successful claim and binding checks. |

## Try the Gooo activity

From a source checkout with the repository's pinned compiler and Go 1.27.2,
set `GOOO_GO` to the absolute path of the Go executable. Use a new output
directory for the execution and its saved replay:

```sh
mkdir -p out
mkdir out/redelivery-example
./.gooo package execute --json --go "$GOOO_GO" \
  --inputs examples/automatic-redelivery/inputs.json \
  automatic-redelivery.workspace.json > out/redelivery-example/execution.json

./.gooo package replay --json --go "$GOOO_GO" \
  --receipt out/redelivery-example/execution.json \
  --inputs examples/automatic-redelivery/inputs.json \
  automatic-redelivery.workspace.json > out/redelivery-example/replay.json
```

The starter supplies one `POST` operation with positive evidence that it was not
sent, a replayable body, current access and request binding, two available
application attempts, and no applicable provider retry guidance. The expected
decision is `REDELIVER_ELIGIBLE` with reason
`POSITIVE_NONAPPLICATION_EVIDENCE` and `attempts_if_claimed` equal to `1`.
This output does not itself decrement a durable counter or send a request.

The entry is `app/automatic-redelivery:AssessRedelivery`. Its nine positional
inputs are named `input0` through `input8` in the input file:

| Position | Fact | Domain |
| --- | --- | --- |
| 0 | Effective method | `GET`, `HEAD`, `OPTIONS`, `TRACE`, `POST`, `PUT`, `PATCH`, `DELETE` |
| 1 | Whole-operation delivery state | `NOT_SENT`, `OUTCOME_UNKNOWN`, `APPLIED_CONFIRMED`, `NOT_APPLIED_CONFIRMED` |
| 2 | Body can be reproduced | Boolean |
| 3 | The same nonempty idempotency key is bound | Boolean |
| 4 | The current applicable server repeat contract is verified | Boolean |
| 5 | Operation and request binding is current | Boolean |
| 6 | Current access is allowed | Boolean |
| 7 | Remaining application claims | Integer from `0` through `2` |
| 8 | Current applicable provider retry guidance | `ALLOW`, `VETO`, `ABSENT`, `UNKNOWN` |

Well-typed values outside these domains produce `STOP` and
`INPUT_DOMAIN_INVALID`. Incorrect JSON types and malformed input are separate
transport concerns. The result echoes all nine supplied values, including invalid
signed integer values, without subtracting from them.

## Rules that matter when the outcome is unknown

The specification in [contract.json](contract.json) defines thirteen ordered
rules. Invalid inputs, refused access and changed request binding are checked
first. An already applied operation selects recorded-result lookup before retry
guidance, budget or body checks.

`ALLOW` never overrides refused access, a changed request, an exhausted budget,
an unreplayable body or an unverified server contract. `VETO` prevents a new
claim. `ABSENT` means verified absence of applicable guidance; unavailable or
conflicting observations mean `UNKNOWN`.

An unknown outcome requires a verified repeat contract even for a read method.
For a write method, the same bound nonempty idempotency key is also required.
A later unsent attempt or an empty lookup result cannot erase an earlier
unresolved attempt: delivery state describes the whole logical operation.

## Application integration

The eight host obligations in the specification remain part of the integration
contract. In particular:

- Bind the operation identity to the effective method, target, body, sent key,
  parameters, account, context and API scope. Keep aggregate attempt history.
- Establish current authority, applicable provider guidance and server retention
  evidence. A caller-supplied Boolean is not proof. Unknown or nearly expired
  retention cannot justify repeating an unresolved write.
- After an eligible result, atomically claim against the current ledger revision
  while checking those bindings and generations again. A failed comparison or
  revocation causes zero dispatch and a fresh assessment.
- Own retry behavior in the SDK, transport and application, and record outcomes
  durably. The budget counts application claims, not physical network sends.

Saved replay reconstructs a prior decision. It grants no current access,
provider contract, claim or effect. This example supplies no transport, ledger,
authentication service or retry loop, and makes no exactly-once or production
safety guarantee.

## Evidence layers

`inputs.json` contains one starter case. `manual-inputs.json` retains sixteen
ordered CLI cases, including precedence checks and signed integer extremes.
`manual-witnesses.json` supplies the corresponding names and facts for the
independently authored Go reference in `reference/`.

The valid finite product has 12,288 inputs. Generated native comparisons over
that product and CLI execution with saved replay are distinct evidence layers;
neither substitutes for the other or establishes behavior beyond the declared
domain. A public validation result must be produced from the current sources.

All policy source and the independent contract-derived reference are covered by
the repository's BSD-3-Clause license.
