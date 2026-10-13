# Apply a checked service-grant change automatically

This package applies a change when Gooo proves it stays within the current
single grant and preserves one configured required request. The Go host checks
the current writer and revision, then records the policy change and operation
result together in memory. No human confirmation is part of this runtime path.

## Run the example

Use an installed Go 1.27.2 toolchain. From the parent
`examples/agentmesh-service-policy` module directory, run:

```sh
GOWORK=off GOTOOLCHAIN=local go run -mod=readonly ./cmd/apply-demo
GOWORK=off GOTOOLCHAIN=local go test -mod=readonly -count=1 ./apply
```

The demo prints exactly these four lines and ends with `mcp://weather` at
revision 3:

```json
{"case":"safe-tightening","status":"APPLIED","reason":"APPLIED","observed_revision":1,"committed_revision":2}
{"case":"stale-proposal","status":"CONFLICT","reason":"STALE_REVISION","observed_revision":2,"committed_revision":0}
{"case":"widening-refused","status":"REFUSED","reason":"WIDENING_WITNESS","observed_revision":2,"committed_revision":0}
{"case":"recover-dropped-response","status":"APPLIED","reason":"APPLIED","observed_revision":2,"committed_revision":3}
```

An external Go application imports
`github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy/apply`.
Use the parent module at a pinned version, or copy that complete module when
running from source. Copying this `apply` directory alone is insufficient:
its policy implementation and source bindings belong to the parent module.
Runtime execution uses generated Gooo and needs no compiler or network calls.
The [parent README](../README.md) explains generation and the finite grammar.

## Use the result correctly

The embedding application authenticates callers before creating a writer.
`WriterForAuthenticated` accepts an already established subject; it does not
verify a claimed identity. Keep the owner and administrative configuration
inside the trusted application. Build a proposal from `Snapshot`, retaining
its epoch, target, revision and generation, and add an operation ID and candidate.
`Apply` accepts no caller-supplied prior grant, proof, or execution callback.

`ELIGIBLE` is Gooo's pure judgment. `APPLIED` means the host committed that
judgment against its current state. `INVALID_INPUT` is distinguishable through
`HasDecision` and `Decision.Status`; it never becomes an ordinary computed denial.
A stale revision produces `CONFLICT`; do not silently rebase the proposal.

If a response is lost, retain the exact proposal and operation ID and call
`Resolve`. A known result recovers the recorded outcome. An outcome not observed
at that lookup is not proof that a concurrent attempt cannot commit later.
Current authorization is checked even when retrieving an earlier result.
The store retains at most 1,024 operation records and refuses new IDs when full.

This reference changes one local in-memory store. State and operation records
are lost on restart. It does not authenticate remote callers, perform external
policy writes, or provide durable or distributed commits. Preserving one required
request does not establish availability for every workload. Source-binding
checks assume a trusted executable; they do not authenticate a remote issuer.
