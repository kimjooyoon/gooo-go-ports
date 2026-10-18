# Retry-After time facts prototype

This original Gooo composition calculates bounded cooldown facts from six supplied
observations. It imports the existing [decimal parser](../../ports/strconv/decimal.gooo);
Gooo calculates the grammar, policy membership, ordered reason and remaining time.
The [frozen v2 contract](contract.json) defines all fourteen result fields and rule
priority. The portable verifier compares113 original inputs with an independently
authored Go reference using pinned compiler839 and Go1.27.2. It checks all fourteen
fields in fresh native execution and saved replay, source bodies, generated ABI,
tool identities and exact raw saved parents, then requires eleven deliberate
receipt mutations to fail at their intended boundaries. This remains a bounded
prototype; hosted CI acceptance must be checked on the current commit.

The entry is `app/retry-after-facts:AssessDelay` in the repository-root
[`retry-after-facts.workspace.json`](../../retry-after-facts.workspace.json).
Its six positional inputs are:

| Input | Supplied observation |
| --- | --- |
| `input0` | Original selected header Text, valid Unicode-scalar UTF-8, 0–64 bytes |
| `input1` | Header presence confirmed: Boolean |
| `input2` | Observed elapsed seconds: exact signed-int64 Integer |
| `input3` | Local maximum wait seconds: exact signed-int64 Integer |
| `input4` | Current request binding: Boolean |
| `input5` | Applicable guidance: Boolean |

Binding keys use `app/retry-after-facts:AssessDelay.input0` through `.input5`.
The host must reject missing, wrong-type, duplicate or unknown request keys,
malformed UTF-8, unpaired Unicode surrogates and headers over64 bytes **before**
invoking the compiler. Direct raw native calls outside this declared domain are
unsupported. [request_inputs.py](request_inputs.py) supplies a transport-only
gate for one request with the six original field names. It checks at most4096
raw bytes, then emits one positional input row; it computes no policy result.
Its refusal tests also verify that semantic-invalid signed integers reach Gooo
unchanged instead of being normalized or mistaken for transport failures.

For a checked first run, bootstrap the repository's pinned compiler as described
in the root README, install Go1.27.2, and run from the checkout root:

```sh
mkdir -p out
python3 -I -B examples/retry-after-facts/verify.py \
  --root "$PWD" --compiler "$PWD/.gooo" \
  --go "$(go env GOROOT)/bin/go" --out "$PWD/out/retry-after-facts"
```

The output directory must be new. The verifier runs the actual starter transport
gate, binds that request into the unchanged113-input corpus, obtains fresh Go
expectations separately, executes two bounded input batches, and replays each
saved receipt. Expectations are never supplied to the Gooo compiler. Inspect
`summary.json`; success is `NATIVE_ORACLE_AND_SAVED_REPLAY_MATCH`, not the
compiler's input-only `OBSERVED` label alone. Raw receipts, command logs and first
failures remain in the output. Only owned temporary work is removed after child
processes close. Downloads are disabled; the existing Go cache is reused.

The whole check has a300-second command budget and a1GiB owned temporary-file
cap. It supports Linux/amd64 and Darwin/arm64 with Go1.27.2. It is not a complete
toolchain attestation: [receipts.py](receipts.py) explicitly lists inner canonical
digests, regenerated instrumentation and native executable hashes it does not
independently check. The full-only CI sidecar requires this same summary and
retains its evidence on failure; ordinary native catalog coverage is unchanged.

From the repository root, with the pinned compiler and an absolute Go1.27.2
executable in `GOOO_GO`, use a fresh output directory:

```sh
mkdir -p out
mkdir out/retry-after-example
python3 -I -B examples/retry-after-facts/request_inputs.py \
  examples/retry-after-facts/request.json > out/retry-after-example/inputs.json
./.gooo package execute --json --go "$GOOO_GO" \
  --inputs out/retry-after-example/inputs.json \
  retry-after-facts.workspace.json > out/retry-after-example/execution.json
./.gooo package replay --json --go "$GOOO_GO" \
  --receipt out/retry-after-example/execution.json \
  --inputs out/retry-after-example/inputs.json \
  retry-after-facts.workspace.json > out/retry-after-example/replay.json
```

The starter has a60-second delay and59 elapsed seconds. Its expected state is
`WAIT`, reason `COOLDOWN_ACTIVE`, with one remaining second. Reassess using a
new current observation after waiting; this output does not start a timer.

The113-case `inputs.json` is a full corpus, not the starter. Its33,420 bytes
exceed compiler839's32KiB input-document limit. Capability validation partitioned
the same ordered corpus into57 and56 inputs, without dropping cases. This is
that pinned compiler's compatibility limit, not a limit asserted for newer
engine sources or releases.

Elapsed seconds may span the nonnegative signed-int64 range. The local wait
policy accepts0–86,400 seconds; this is a prototype policy choice. Negative
elapsed values or a maximum outside that range produce `DOMAIN_REFUSED`.
The header is preserved without trimming or rewriting. Only nonempty ASCII
digits form delta-seconds grammar; leading zeros are retained. HTTP-date text,
signs and Unicode digits do not enter this profile.

The imported parser always runs, including when metadata or binding is invalid.
Its value and syntax/range diagnosis are retained. `delay_in_policy` describes
numeric/grammar membership independently of confirmation flags; it is not trusted
guidance. Earlier refusal reasons take precedence exactly as specified in the
contract. Only admitted `WAIT` subtracts elapsed seconds from a bounded delay.
Unknown or refused states retain `remaining_seconds=-1`.

`READY_TO_REASSESS` means obtain current facts and assess again. It never creates
provider `ALLOW` or `ABSENT`, dispatch eligibility, a server repeat contract,
authority or a durable claim. An application must establish the observation's
request/whole-operation binding, presence, applicability and elapsed time.
A supplied Boolean is not authentication, and saved replay cannot refresh facts.
The existing [automatic-redelivery policy](../automatic-redelivery/README.md)
retains its separate outcome, access, key, budget and atomic-claim obligations.

This prototype performs no sleep, clock read, timestamp arithmetic, network
request, scheduling or retry-loop effect. It does not replace a Go SDK or claim
full HTTP Retry-After, HTTP-date, repeated-header selection or rate-limit support.
No new compiler pin or engine feature is assumed. Finite observations do not
establish current provider evidence, producer authentication, production safety
or a live scheduler integration. The separate82-suite native catalog is unchanged.
