# Semantic versions in Gooo

This port implements `golang.org/x/mod/semver.IsValid`, `Compare`, `Major` and
`Prerelease` in Gooo.
Import `go/golang.org/x/mod/semver` in a package workspace. The
[version eligibility example](../../examples/version-eligibility/README.md)
shows a separate importing package, a candidate decision, and a minimum-version
rule with an explicit refusal reason.

| Function | Result | Contract |
| --- | --- | --- |
| `IsValid(Text)` | Boolean | Accepts Go's `v`-prefixed semantic versions, including shorthand `v1` and `v1.2`. |
| `Compare(Text, Text)` | Integer | Returns -1, 0 or 1. Build metadata does not affect ordering. Invalid versions sort below valid versions; two invalid versions compare equal. |
| `Major(Text)` | Text | Returns the `v`-prefixed major, such as `v2`; returns empty Text for an invalid version. |
| `Prerelease(Text)` | Text | Returns the hyphen-prefixed prerelease without build metadata, such as `-rc.1`; returns empty Text for an invalid version or no prerelease. |

Each argument is decoded valid UTF-8 Text of at most 1,024 bytes. Inputs are
preserved without trimming or normalization. This bounded Text domain differs
from Go's unrestricted byte strings. Other `x/mod/semver` APIs are not ported.

`Major` keeps all valid major digits as Text within this byte limit; it does
not convert them to an integer. `Prerelease` alone
cannot distinguish invalid input from a stable version: check `IsValid` too.
The example's stable same-major filter derives these facts in Gooo while
retaining the original minimum-version decision separately. Same major and no
prerelease do not establish API compatibility or safe deployment, including for
`v0` minor updates.

The parser's byte scans retain their 1,024-step bound. Prerelease comparison
uses at most 512 identifier steps: a prerelease with `n` nonempty identifiers
needs at least `2*n` bytes including its leading hyphen and separators. A valid
full version needs a six-byte core too, leaving room for at most 509 identifiers.
`Compare` validates the complete version before using that helper. Internal
parser activities are implementation details, not additional supported APIs.

The independent oracle imports the original Go package at
[`v0.29.0`, commit `bba3e065`](https://github.com/golang/mod/tree/bba3e065a67271df90253c78c98f2cea7f572948/semver).
[Provenance](provenance.json) records immutable source hashes and module
checksums. Original source, tests and the BSD license are preserved under
[`third_party/x-mod`](../../third_party/x-mod).

Follow the example's finite verification commands to build that oracle, compare
its outputs with Gooo, and replay each execution. These checks cover the supplied
cases; they do not establish full-library or full-domain compatibility.
