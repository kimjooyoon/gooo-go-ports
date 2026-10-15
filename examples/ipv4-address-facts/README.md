# IPv4 facts in Gooo

Six predicates adapt Go 1.27.2 `net/netip`: `IsPrivate`, `IsLoopback`,
`IsUnspecified`, `IsMulticast`, `IsLinkLocalUnicast` and `IsGlobalUnicast`.
The original Gooo `app/ipv4-address-facts:Report` imports these activities and
returns eleven required fields: `a`, `b`, `c`, `d`, `valid`, `is_private`,
`is_loopback`, `is_unspecified`, `is_multicast`, `is_link_local_unicast` and
`is_global_unicast`. Gooo computes the facts; Go supplies independent references.

Each activity accepts four explicit signed int64 Integer values rather than a
Go `netip.Addr` receiver. All four must be in `0..255`. An out-of-range int64
returns false from each predicate; Report preserves all four original integers
and returns `valid=false` and six false flags. This is an explicit Gooo domain
extension, not Go's invalid `Addr{}` behavior. Boolean, floating point, string,
null and beyond-int64 tokens must not be coerced into octets. Use Report's
`valid` field when the distinction matters: `255.255.255.255` is a valid tuple
with six false flags too.

Flags can overlap: `10.0.0.1` is both private and global unicast. Go also treats
some CGNAT, documentation and reserved addresses as global unicast. These facts
do not describe reachability or grant permission to connect to a destination.
Parsing, IPv6, mapped addresses, zones, DNS, redirects and network effects are
outside this adaptation.

After the [repository setup](../../README.md), run the one-row starter from the
repository root with the pinned compiler and explicit Go 1.27.2 executable:

```sh
./.gooo package execute --json --go "$(go env GOROOT)/bin/go" \
  --inputs examples/ipv4-address-facts/inputs.json ipv4-address-facts.workspace.json
```

To compare fresh native results with Go and check saved replay, choose a new
output directory for each run:

```sh
python3 -B tools/check-ipv4.py --repository . \
  --go "$(go env GOROOT)/bin/go" --compiler ./.gooo \
  --out out/ipv4-address-facts --timeout 180
```

The checker keeps two finite layers separate:

- The original `corpus.json` has 64 tuples. Each of the six scalar entries uses
  its first 48 (32 valid boundaries and 16 invalid-domain inputs); Report uses
  all 64. Fifteen batches produce 352 CLI outputs and 352 saved-replay outputs,
  with 992 returned scalar field comparisons in each phase.
- `corpus-202.json` preserves those 64 rows and adds 138 invalid tuples. It puts
  signed extrema on otherwise-positive private, loopback, multicast and
  link-local witnesses, and includes values which would wrap into positive
  addresses if narrowed before validation. The checker compiles the freshly
  verified generated Report source unchanged and calls it on all 202 tuples,
  comparing 2,222 typed fields, including 808 original input echoes, with a fresh
  independent Go reference. These are generated native function calls, not 202
  individual CLI, saved-replay or direct scalar-entry observations.

The public Go reference reads this exact 202-row input-only corpus. For its 48
valid tuples it calls actual `netip.AddrFrom4` and the six standard-library
methods. For 154 invalid tuples it checks the integer domain before any byte
conversion and applies the separate unchanged-echo/false extension. No Go
classification formulas are copied into the reference evaluator.

The checker must validate typed results, source/entry/compiler/Go identities,
both raw-parent and typed-result replay links, and its strict input and receipt
mutation controls before reporting a match. Input-only compiler receipts remain
`OBSERVED` with zero finite expectations. Existing 66-suite verification stays
separate; these finite comparisons do not establish all IPv4 or int64 inputs,
whole-library compatibility, security authorization, production safety or a
speedup. Historical receipts do not replace fresh execution.

The adapted predicates retain the Go Authors' copyright and BSD terms in
`third_party/GO-LICENSE`. `third_party/go/netip.go.txt` is the byte-exact Go 1.27.2
source at revision `022c8636110ebac86a9b88724cda168ed713c21f`, SHA-256
`32295f4761825d8c6fdade4dad093b08683bdd1945010bee8a90a298e18cf208`.
The original Report uses the repository's BSD license. This independent project
has no Go or Google endorsement.
