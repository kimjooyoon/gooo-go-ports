# Choose among service-grant proposals with Gooo

An automation can propose several narrower service grants while needing to
retain every supplied service request. This example evaluates each proposal
with the existing Gooo policy, then lets a new Gooo activity choose among the
eligible proposals using an explicit supplied preference.

From a reviewed checkout, use Go **1.27.3**:

```sh
GOTOOLCHAIN=local GOWORK=off go -C examples/grant-candidate-selection run -mod=readonly ./cmd/select \
  < examples/grant-candidate-selection/example-input.json
```

Building may download the pinned public Go module. The resulting executable
needs no Go, Gooo, Python, model or API key at runtime. This new module is under
development; current published MCP bundles do not expose this selection API.

The example has two requirements: weather and calculator. `weather-only` has
the lower preference value, but removes calculator, so Gooo refuses it. Gooo
selects `all-mcp`, whose proposed `mcp://*` grant preserves both requirements.
The report retains both complete assessments and their source bindings.
`selection.selected_assessment_id` identifies the exact Go-JSON bytes of the
selected proof. `input_sha256` identifies the exact received JSON document,
including whitespace, rather than a reserialized object.

## For an agent integrating this command

Use the standalone JSON schemas for [input](schemas/input.schema.json),
[stdout results](schemas/output.schema.json) and
[stderr diagnostics](schemas/diagnostic.schema.json). The
[recovery guide](agent-recovery.md) explains each state, stage/code and next
step without requiring routine human approval for this pure assessment.

Schemas describe structure. They do not replace the native byte parser, Gooo
admission/policy decisions, proof correspondence or host effect authority.
Contract tests exercise actual public API results and explicitly preserve those
boundaries. The schema validator is a pinned test dependency; the compiled
command still links only the existing policy module.

## Native Go API

Call `Assess(Input)` with `before`, ordered `required_requests` and ordered
`candidates`. Each candidate has `id`, `after` and nonnegative int64
`preference`. The public input contains no evaluation status, verification flag,
assessment ID, phase or source binding for a caller to assert. `AssessJSON`
additionally validates the exact JSON structure and records its byte digest.

Gooo admits 2-32 candidates, checks unique nonempty IDs up to 128 UTF-8 bytes,
and rejects negative preferences. Among eligible proposals, the lowest supplied
preference wins; equal values retain input order. This is a caller-declared
preference, not a minimum-permission or global-optimality calculation.

Go snapshots the supplied arrays, calls the existing generated Gooo policy for
each admitted candidate, and checks the returned facts and exact pinned
generation bindings. Only then does it derive private selector columns and
proof/context hashes. The selector and its raw column type live in an internal
package. The Gooo selection rule is not reimplemented in Go.

| Result | Meaning |
| --- | --- |
| `selection.state: PROPOSED` | A supplied candidate was eligible and selected by the supplied preference. |
| `selection.state: REFUSED` | No assessment was eligible; inspect every original candidate decision, including represented domain-invalid inputs. |
| `selection.state: INVALID_INPUT` | Gooo rejected candidate admission, such as a duplicate ID or more than 32 candidates. No policy fan-out occurred. |
| Diagnostic `state: UNKNOWN` | The host could not bind a dependency result or generation. There is no policy decision on stdout. |
| Diagnostic `state: INVALID_TRANSPORT` | Input encoding/structure or the documented I/O budget was rejected. There is no policy decision on stdout. |

Normal represented results exit zero. Input, binding and output-size failures
exit one after recording the stage/code on stderr, without a partial decision.
The command applies no grant and calls no service. These calculations need no
per-request human approval; current facts, authority and any atomic application
remain the integrating host's responsibility.

## Domain and evidence boundaries

Service patterns retain the existing bounded ASCII 128-byte
[policy domain](../agentmesh-service-policy/README.md#exact-represented-domain),
rather than arbitrary URL or individual-tool syntax. Ordered requirements and
duplicate positions are preserved. The Gooo policy owns its 256-request limit:
257 requests reach that policy and return a represented invalid assessment.
No string is trimmed, repaired or silently replaced.

Every supplied Go Text must be decoded valid UTF-8 and at most 1,024 bytes.
The CLI accepts one exact JSON object, at most 1 MiB, with no duplicate/unknown
keys, null records, unpaired surrogate escapes or non-int64 preferences. Its
complete serialized report is limited to 4 MiB before writing stdout; this wire
budget is separate from the source-owned candidate and requirement limits.

The kernel uses compiler `205896a413e5dea92366309c35ae85e542d055f6` and Go
1.27.3. Its Go code comes from actual source-bound native execution/replay
receipts; `generate.py` maps compiler-owned record/field identities into an
internal transport bridge. `reference/` is an independent validate/filter/stable
sort oracle for finite kernel cases. Hashes and unsigned reports identify bytes;
they do not authenticate live facts, an arbitrary producer or effect authority.

The current kernel has 82 finite cases, including admission, stable ties,
int64 boundaries, Unicode ID byte limits, invalid columns and unknown bindings.
The API tests cover real policy composition, preservation of every proof,
caller reuse, no partial result after a late binding failure, concurrency,
32 candidates with 256 requirements, malformed JSON and I/O boundaries.

## Reproduce the verification

With the exact reviewed compiler above and Go 1.27.3, write evidence to a fresh
output directory:

```sh
python3 -I -B examples/grant-candidate-selection/verify.py \
  --compiler /absolute/path/to/gooo --go /absolute/path/to/go \
  --out /absolute/path/to/new-selection-evidence
```

The verifier compares a fresh independent reference with all 82 checked-in
cases, executes and replays the Gooo kernel, checks generated files, runs Go
static and race checks, and builds the public command. It runs that command
with an empty PATH and no GOROOT, then independently compares and replays the
two actual selection results, retaining all 12 fields and original proof IDs.
Four altered-receipt controls must fail. Three malformed JSON controls must
exit one with no decision on stdout.

The compiler is never built or downloaded by this command. Building the
consumer may download its exact pinned public module. Optional `--go-cache` and
`--module-cache` reuse existing caches, which this verifier never removes;
no cold-cache performance or shared-cache size bound is claimed. Child process
receipts retain direct wait, stream EOF and process-group closure. Completed
owned command binaries are removed; raw results and summaries remain.

The full-scope policy CI job runs this verifier and retains its evidence. Local
observations do not establish external adoption or compatibility with every
client or platform. Each source revision still requires its own full-scope CI; previous source
observations are not rebound to a changed checkout.
