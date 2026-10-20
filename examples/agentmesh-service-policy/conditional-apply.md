# Apply a service-grant change without per-change approval

An automation that replaces `*` with `mcp://*` should be able to preserve its
required `mcp://weather` request while narrowing access. It should also refuse an
expansion and detect when another update made its proposal stale.

This example makes that decision in [Gooo](https://github.com/kimjooyoon/meta-ontology-go).
The [grant-update activity](conditional_apply.gooo) composes the checked impact
and refinement activities. A Go [application host](apply/store.go) owns the
current grant, writer authorization and operation records. There is no human
confirmation in the application path.

## Run the four-case demo

For caller-supplied facts without a state change, use the
[`evaluate -mode grant-update` stdin CLI](README.md#use-the-generated-ordinary-go-library-and-stdin-cli).
It accepts a bounded JSON array and returns the pure Gooo decisions, facts and
source bindings. Policy refusals and invalid-domain decisions exit successfully;
malformed JSON structure refuses the whole batch without stdout.

With Go 1.27.3 already installed, run from `examples/agentmesh-service-policy`:

```sh
GOWORK=off GOTOOLCHAIN=local go run -mod=readonly ./cmd/apply-demo
```

The generated Gooo code runs as ordinary Go. The demo uses one in-memory store
and prints:

```json
{"case":"safe-tightening","status":"APPLIED","reason":"APPLIED","observed_revision":1,"committed_revision":2}
{"case":"stale-proposal","status":"CONFLICT","reason":"STALE_REVISION","observed_revision":2,"committed_revision":0}
{"case":"widening-refused","status":"REFUSED","reason":"WIDENING_WITNESS","observed_revision":2,"committed_revision":0}
{"case":"recover-dropped-response","status":"APPLIED","reason":"APPLIED","observed_revision":2,"committed_revision":3}
```

The final grant is `mcp://weather` at revision 3. The fourth case discards a
completed response and looks up the original operation; it does not perform a
new policy write.

## Embed it in a Go application

Import `github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy/apply`
from the same module as the pure policy APIs. The demo shows the complete flow:

1. The trusted application creates an owner with a target, current pattern,
   required request and authorized writer. It keeps the owner private.
2. After authenticating a caller, the application obtains a writer with
   `WriterForAuthenticated`. That method consumes an established identity;
   authentication belongs to the application.
3. The writer reads a snapshot. A proposal binds the candidate and operation ID
   to that snapshot's target, epoch, revision and generation identity.
4. `Apply` checks current authority and the proposal under the store lock,
   evaluates Gooo, and publishes the policy and recorded result together.

`EvaluateGrantUpdate` in the module root provides the pure decision, exact input
facts and source bindings without changing state. Its result has seven fields.
`ELIGIBLE` means the candidate satisfies the represented policy conditions;
`APPLIED` means the host committed the transition. Malformed inputs produce
`INVALID_INPUT`. A widening witness, an unproved inclusion, a removed required
request, or an inconsistent dependency result cannot become an eligible change.

Administrative updates advance the revision, including changes to writer or
required-request configuration. A stale proposal returns a conflict. The caller
must obtain and assess the new state before proposing another operation.

## Handle an uncertain response

Keep the same complete proposal and call `Resolve`. `KNOWN` returns its recorded
result, even after a later policy change. Reusing an operation ID with changed
proposal fields returns a conflict. Current writer authority is checked on
lookup as well as application.

`NOT_OBSERVED_AT_THIS_LOOKUP` only describes that lookup: a concurrent attempt
could still commit later. A different store epoch produces `UNKNOWN_OUTCOME`.
The store retains at most 1,024 operation records and refuses new IDs at capacity;
it does not evict earlier records to make a retry appear new.

## Scope and verification

The Gooo rules use the existing bounded canonical ASCII service-pattern grammar.
They compare one old grant with one candidate and preserve one configured
request. A preserved request does not establish availability for every workload.

This host changes a local in-memory store. State and records are lost on restart.
An integration that writes to a remote policy service needs that service's
authentication, durable recovery and atomic update mechanism. The local example
does not establish those properties for an external service.

The independent corpus contains 21 Gooo decisions, including invalid input and
128/129-byte boundaries. The verifier compares native results with direct Gooo
execution and saved replay. Host tests exercise concurrent updates, authorization
revocation, stale revisions, operation identity and lost-response lookup.
Injected unknown or contradictory decisions test host refusal; they are not
claims that the current Gooo rules produce those decisions.

See the [regeneration commands](README.md#regenerate-and-independently-verify)
and preserved [source attribution](NOTICE.md).
