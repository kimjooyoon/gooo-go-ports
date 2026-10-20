# IPv4 prefix queries in Gooo

`PrefixContains(IPv4Prefix, IPv4Address)` and
`PrefixOverlaps(IPv4Prefix, IPv4Prefix)` adapt Go 1.27.2 `net/netip`.
The importing `app/ipv4-prefix-probe:Contains` and `Overlaps` activities run
these Gooo bodies. Go supplies an independent reference; it is not a fallback.

`IPv4Address` has four required Integer fields `a,b,c,d`; `IPv4Prefix` adds
`bits`. Each Integer is signed int64. Every octet must be `0..255`, and bits
must be `0..32`. Invalid domains return false before arithmetic or narrowing.
Missing, null, Boolean, fractional or beyond-int64 fields are not valid Integer
inputs and must not be defaulted or coerced.

Host bits stay in the original records. For example, `10.1.2.3/8` contains
`10.99.0.1`, and `192.168.1.99/24` overlaps `192.168.1.4/32`. `/0` includes every
valid IPv4 address; `/32` selects one address. `0.0.0.0` is valid IPv4 and differs
from Go's zero-value `Addr{}`. The adaptation does not represent that Go zero
value, IPv6, mapped IPv6, zones, text parsing, DNS or actual network operations.
A false query alone does not distinguish an invalid input from a disjoint range.

After the [repository setup](../../README.md), run from the repository root
with the pinned compiler and an explicit Go 1.27.3 executable:

```sh
mkdir -p out/ipv4-prefix-example
./.gooo package execute --json --go "$(go env GOROOT)/bin/go" \
  --inputs examples/ipv4-prefix-probe/contains-inputs.json \
  prefix-contains.workspace.json > out/ipv4-prefix-example/contains.json
./.gooo package replay --json --go "$(go env GOROOT)/bin/go" \
  --receipt out/ipv4-prefix-example/contains.json \
  --inputs examples/ipv4-prefix-probe/contains-inputs.json \
  prefix-contains.workspace.json > out/ipv4-prefix-example/contains-replay.json
```

For overlap, use `prefix-overlaps.workspace.json` and
`examples/ipv4-prefix-probe/overlaps-inputs.json`, saving to separate output files.
Both workspaces include `ports/net/netip/ipv4.gooo` and `prefix.gooo` in the same
`go/net/netip` package; Integer and Boolean are declared once in `ipv4.gooo`.

`corpus-18.json` is an input-only boundary set. Validation requires the
fresh execution, saved replay and independent comparison below.
The opt-in checker must produce fresh CLI execution and saved replay for both
18-row query entries and the separate 18-row rule-pair report, then compare the
report's 112-case generated-function layer with actual Go APIs:

```sh
python3 -I -B tools/check-ipv4.py --repository . --compiler ./.gooo \
  --go "$(go env GOROOT)/bin/go" --out out/ipv4 --timeout 180 \
  --include-prefix-rule-pair
```

Choose a new checker output directory for every run. Its original IPv4 layer
and existing 66-suite catalog retain their own counters. None of these finite
observations proves every IPv4/prefix input, a full ACL, reachability or permission.
See the [rule-pair composition](../ipv4-rule-pair/README.md) for explicit validity,
comparability, conflict reasons and a network-address witness.

The port retains the Go Authors' 2020 copyright and BSD terms in
`third_party/GO-LICENSE`. The pinned source reference is
`third_party/go/netip.go.txt`, revision `022c8636110ebac86a9b88724cda168ed713c21f`,
SHA-256 `32295f4761825d8c6fdade4dad093b08683bdd1945010bee8a90a298e18cf208`.
The importing query example is original repository code under `LICENSE`.
