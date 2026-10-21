# Assess a version-bound service-grant change

This experimental [Gooo](https://github.com/kimjooyoon/meta-ontology-go)
composition checks whether a proposed grant change still uses the observed policy
version, narrows the supported grant and preserves a configured required request.
The decision requires neither a model call nor approval for each request.

[main.gooo](main.gooo) combines the existing strict validator facts,
Go-compatible `IfMatchCondition` and checked grant-update activities. It returns
an explicit twelve-field decision. The eleven imported Gooo sources remain at
their existing repository paths; the
[workspace](../../versioned-grant-change.workspace.json) does not duplicate them.

`ELIGIBLE` describes the supplied local facts. The application must obtain and
bind current facts, authenticate the writer and commit under its own atomic
version/authority checks. This example does not apply a grant or send HTTP.

## Verify the current sources

Use Python 3, Git and Go **1.27.3**. From the repository root, prepare the pinned
compiler and dependencies once:

```sh
set -eu
export GOTOOLCHAIN=local
./tools/bootstrap.sh
GOWORK=off go mod download
GOWORK=off go mod verify
```

Bootstrap fetches the pinned language source and builds `.gooo` from a clean
checkout. If that compiler and the pinned dependencies are already prepared,
run the verifier directly:

```sh
python3 -I -B examples/versioned-grant-change/verify.py \
  --root "$PWD" \
  --compiler "$PWD/.gooo" \
  --go "$(GOTOOLCHAIN=local go env GOROOT)/bin/go" \
  --module-cache "$(GOTOOLCHAIN=local go env GOMODCACHE)" \
  --out "$PWD/out/versioned-grant-change"
```

The output directory must be new. The verifier is intended to preserve the
ordered [48 independent cases](cases.json), check fresh component observations
from pinned Go references, execute the current Gooo graph and use the official
CLI to replay each original receipt. Each phase must compare all **288 input
values and 576 typed output fields**. Final subset/status/reason expectations
are explicit contract literals, not a Go business-policy implementation or
values generated from Gooo output.

The verifier reads the prepared module cache and copies the two pinned modules
into its own temporary directory. Execution is offline; the shared cache is not
modified or removed. Only the verifier's own temporary work is removed after
its observed processes close. Receipts, command output and source identities
remain in the new output directory. Eight mutations of that fresh evidence
also check refusals for stale sources, changed typed data and a wrong saved
replay parent without another native execution.

Read the new run's summary and retained evidence before claiming success.

The current205 verifier keeps the same48 cases in six batches. Its retained
source diagnostics require a16MiB command-output limit and a200MiB total
evidence limit. Other users of the shared command runner keep the8MiB default;
input, stderr, process, time and owned-work limits are unchanged.
These instructions and the source contract do not establish that a particular
checkout or edit passed. A changed source must receive fresh verification.

## Understand the automatic decision

The six inputs are selected If-Match, observed policy ETag, before grant,
candidate grant, required service type and required service name, in that order.
The following are expectations represented in the corpus; the required request
is `mcp://weather`:

| Before → candidate | Selected / current tag | Expected decision |
| --- | --- | --- |
| `*` → `mcp://*` | `"v7"` / `"v7"` | `ELIGIBLE` |
| `mcp://weather` → `mcp://*` | `"v7"` / `"v7"` | `REFUSED`, grant reason `WIDENING_WITNESS` |
| `mcp://*` → `mcp://other` | `"v7"` / `"v7"` | `REFUSED`, grant reason `REQUIRED_REQUEST_REMOVED` |
| `*` → `mcp://*` | `"v6"` / `"v7"` | `REFUSED/POLICY_VERSION_CHANGED` |
| `*` → `mcp://*` | empty / `"v7"` | `REFUSED/PRECONDITION_REQUIRED` |
| `*` → `mcp://*` | `W/"v7"` / `"v7"` | `REFUSED/IF_MATCH_NOT_SINGLE_STRONG` |

Malformed grant input has priority and preserves its original `INVALID_INPUT`
diagnostics. A successful final decision requires both raw validators to be
exactly one complete strong, header-valid tag, a known matching If-Match result
and an internally consistent eligible grant result. A wildcard, weak tag or
trailing text does not satisfy this new composition's strict prerequisite. The
underlying compatibility port retains its original Go behavior.

## Preserve the boundaries when editing

Each input is an unchanged decoded Unicode-scalar UTF-8 Text of 0–1024 bytes.
The four policy operands retain their existing checked 128-byte ASCII grammar:
supported raw overlength or noncanonical values produce a typed `INVALID_INPUT`.
They are not silently truncated or rejected at a new 128-byte ingress limit.

The joint evidence envelope also requires every complete Go case document,
including expected values, to fit 30,000 bytes and its source-derived lowered
upper bound to fit 32,768 bytes. Batches preserve contiguous occurrences and all
six operands; an oversized singleton is refused. Six 1024-byte NUL strings do
not fit. This is not a full six-by-1024 domain or a public network input adapter.

The [contract](contract.json) records the ordered ports, field IDs, source pins,
precedence and host responsibilities. When intentionally changing the policy,
review the source pins and independent expected cases together, including the
corpus digest in `contract.json` and `CORPUS_SHA` in `verify.py`, then use a new
output directory. Never regenerate expectations from the native result to make
a change pass. The official CLI owns inner canonical/replay verification.

The host must bind versions to all relevant policy, required-request and
authority configuration without ABA reuse, then re-evaluate current facts and
publish under one lock or equivalent compare-and-swap boundary. Durability,
distributed recovery and operation idempotency remain host concerns. The
[existing in-memory apply example](../agentmesh-service-policy/conditional-apply.md)
illustrates a separate application boundary; it does not implement this ETag
version protocol.

The composition uses the repository [BSD-style license](../../LICENSE).
Imported Go, x/net, bounded-loop and Agent Mesh notices remain in the original
sources and are linked from the contract. This is an independently developed
composition, not an official Go subpackage, Google policy API or endorsement.
