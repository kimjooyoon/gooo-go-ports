# Equal-priority IPv4 rule-pair facts

This original Gooo composition compares two supplied rules and returns an
11-field record. It imports the IPv4 prefix port; it is not a Go ACL package
adaptation. No per-pair human approval is part of this pure calculation.

Each Rule has eight required signed int64 Integers:

| Fields | Domain |
| --- | --- |
| `a,b,c,d` | Each octet in `0..255` |
| `bits` | Prefix length `0..32`; original host bits retained |
| `effect` | `0` DENY or `1` ALLOW |
| `scope` | Positive opaque same-context key supplied by the caller |
| `priority` | Any signed int64; only exact equality is considered |

The caller must assign scope consistently for the same tenant, action, principal,
protocol, port and other relevant context, with stable policy revision binding.
Gooo checks positive Integer/equality only. It does not establish that this
context is true, authenticated or fresh. Different scope or priority is explicitly
`comparable=false`; `conflict=false` then is not permission or a successful
conflict-free comparison. Invalid fields similarly return `valid=false`.

Required result fields are `valid`, `comparable`, `overlap`, `opposing_effect`,
`conflict`, `witness_valid`, `witness_a`, `witness_b`, `witness_c`, `witness_d`
and `reason`. Valid geometric overlap and opposite effects remain facts even
when the rules are not comparable. All 16 original integers are preserved in
receipt input ports and their typed field metadata, not duplicated in the output.
The 8-field Rule and 11-field result fit the pinned compiler's 16-field record limit.

Reasons have this precedence:

| Reason | Meaning |
| --- | --- |
| 0 | INVALID_INPUT: either Rule is outside its semantic domain |
| 1 | SCOPE_MISMATCH: valid scopes differ |
| 2 | PRIORITY_MISMATCH: valid scopes match but priorities differ |
| 3 | DISJOINT: comparable prefixes do not overlap |
| 4 | SAME_EFFECT: comparable overlapping rules agree on effect |
| 5 | CONFLICT: comparable overlapping rules have opposite effects |

Only conflict has `witness_valid=true`. Its address is the more-specific prefix's
canonical network address; equal bit lengths select the left prefix. Both
prefixes must contain it. Otherwise the four witness octets are zero with
`witness_valid=false`. A genuine `0.0.0.0` witness is distinguished by a true flag.
For example, opposite effects on unmasked `10.1.2.3/8` and `10.20.30.40/16` at
the same scope/priority conflict with witness `10.20.0.0`.

After the [repository setup](../../README.md), run from the repository root:

```sh
mkdir -p out/ipv4-rule-pair-example
./.gooo package execute --json --go "$(go env GOROOT)/bin/go" \
  --inputs examples/ipv4-rule-pair/inputs.json ipv4-rule-pair.workspace.json \
  > out/ipv4-rule-pair-example/execution.json
./.gooo package replay --json --go "$(go env GOROOT)/bin/go" \
  --receipt out/ipv4-rule-pair-example/execution.json \
  --inputs examples/ipv4-rule-pair/inputs.json ipv4-rule-pair.workspace.json \
  > out/ipv4-rule-pair-example/replay.json
```

`corpus-18.json` and `inputs.json` retain 18 manual input-only cases.
Validation requires fresh execution and comparison of the layers below.
`corpus-112.json` preserves those 18, adds 66 prefix probes (two for every length
0..32) and 28 invalid-domain probes (two for each of seven fields on both sides).
The second `/0` probe is a different host with valid overlap: a valid disjoint
IPv4 prefix cannot exist alongside `/0`. Priority is excluded from invalid-domain
probes because every signed int64 priority is valid. Names and ordered 16-integer
tuples are unique. Input-selection bit arithmetic does not supply policy results.

The fresh opt-in validation keeps its layers separate: 18 CLI outputs
and 18 saved-replay outputs for each of Contains, Overlaps and Report, then 112
actual calls to the fresh Gooo-generated Report function. That report layer must
compare 1,232 typed facts and bind 1,792 original input integers with an independent
Go reference using actual `PrefixFrom`, `Overlaps`, `Masked` and both `Contains`
checks for a witness. The 112 individual CLI/replay counts remain zero. Run the
[opt-in checker](../ipv4-prefix-probe/README.md) in a fresh output directory.
Recheck exact input-corpus bytes without rewriting files:

```sh
python3 -I -B tools/generate-rule-pair-corpus.py --check
```

All 33 prefix lengths being represented is not exhaustive IPv4, prefix-pair or
ACL coverage. Existing 66-suite/159,909-case/2,109-batch counters and other sidecars
remain separate. This contract excludes longest-prefix-wins, deny-overrides,
ordered/inherited ACL resolution, actual request authorization, connected-peer
verification, DNS, I/O, host adapters, atomic claims, ledgers and production safety.
Neither invalid nor non-comparable `conflict=false` establishes permission.
The [contract](contract.json) gives the exact types, field IDs and reason mapping.
This original composition uses the repository BSD license; its imported prefix
port preserves Go BSD attribution in its source and `third_party/GO-LICENSE`.
