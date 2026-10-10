# Use Unicode classification in your own Gooo project

This walkthrough builds a separate application that imports `unicode.IsLetter`
and `unicode.IsDigit`. Both calculations stay in the vendored Gooo sources. The
application returns a record for each integer input; no model or inference
service is needed.

Use Git, a POSIX shell, and Go with toolchain selection support, or an installed
Go **1.27.2**. Start in a working directory where `ports-source`, `app`, and
`proof` do not exist. Run the shell blocks in order in the same shell, and stop
if a command fails. Keep your shell in that starting directory throughout;
commands that need another directory use a subshell.

## Get the fixed source version and build its compiler

This guide uses ports commit
[`e1a8fa4f6ec0f41edbe8e97d4490c940edd92d61`](https://github.com/kimjooyoon/gooo-go-ports/commit/e1a8fa4f6ec0f41edbe8e97d4490c940edd92d61),
compiler pin `366ee6ef8f9d2b9b57331dfd26b6f908c0289851`, Go **1.27.2**, and
Unicode **17.0.0**. The original Go source revision is
`022c8636110ebac86a9b88724cda168ed713c21f`; its source-file hashes are recorded in
`upstream.json`. The older preview.2 snapshot used by the
[ValidRune walkthrough](use-in-your-project.md) does not contain these
classification ports. Use the commit below rather than that release or a moving
branch.

```sh
export GOTOOLCHAIN=go1.27.2 GOWORK=off
[ "$(go env GOVERSION)" = go1.27.2 ]
ports_revision=e1a8fa4f6ec0f41edbe8e97d4490c940edd92d61
mkdir proof
git init ports-source
git -C ports-source remote add origin https://github.com/kimjooyoon/gooo-go-ports.git
git -C ports-source fetch --depth=1 origin "$ports_revision"
git -C ports-source checkout --detach "$ports_revision"
[ "$(git -C ports-source rev-parse HEAD)" = "$ports_revision" ]
(cd ports-source && ./tools/bootstrap.sh) > proof/fresh-bootstrap.log 2>&1
ports-source/.gooo version --build --json > proof/compiler.json
```

The first run may download Go 1.27.2. `GOTOOLCHAIN` selects that exact version;
`GOWORK=off` prevents an enclosing Go workspace from affecting the build. Repeat
these exports if you continue in a new terminal.

Bootstrap fetches and builds the pinned compiler from a clean checkout. Its log
is saved in `proof/fresh-bootstrap.log`; inspect that file if bootstrap fails.
`proof/compiler.json` records the executable's build identity, including
`compiler_source_sha`, `go_version`, and `source_status: "CLEAN_VCS"`. A version
label such as `0.6.24-dev` alone does not identify the compiler source.

## Copy the dependency into the application

```sh
mkdir -p app/vendor/gooo-go-ports/ports/unicode app/vendor/gooo-go-ports/third_party
for file in ports/unicode/space.gooo ports/unicode/classification.gooo LICENSE NOTICE third_party/GO-LICENSE upstream.json; do
  cp "ports-source/$file" "app/vendor/gooo-go-ports/$file"
  cmp "ports-source/$file" "app/vendor/gooo-go-ports/$file"
done
printf '%s\n' "$ports_revision" > app/vendor/gooo-go-ports/REVISION
```

Keep **both** `space.gooo` and `classification.gooo`. The first file declares the
standard `Integer` and `Boolean` entities used by the second file. Registering
only `classification.gooo` fails with:

```text
PACKAGE_TYPE_UNKNOWN: go/unicode references undeclared entity type "Integer"
```

Each `cmp` checks that a copied file is byte-for-byte identical to the fixed
checkout. Keep the port copyright comments, licenses, `upstream.json`, and
`REVISION` with the dependency. `upstream.json` describes the original Go source
and the full port index; copying it does not install every indexed port.
`REVISION` identifies the Gooo port snapshot. The original Go source hashes and
the copied Gooo file bytes are separate provenance checks.

The application does not need a `go.mod`. Its Gooo imports are resolved by the
workspace manifest below, not by Go's module or vendor resolver. Go supplies the
compiler's native execution toolchain; no Go classification implementation is
added to the application.

## Add the Gooo application and workspace

```sh
cat > app/main.gooo <<'SOURCE'
package rune_properties
namespace external_rune_properties
import unicode "go/unicode"
entity Integer id "gooo://std/integer"
entity Properties id "gooo://external-adoption/rune-properties/result" fields {
    field in_rune_domain id "gooo://external-adoption/rune-properties/in-rune-domain" type boolean required one
    field letter id "gooo://external-adoption/rune-properties/letter" type boolean required one
    field digit id "gooo://external-adoption/rune-properties/digit" type boolean required one
}
activity Inspect(Integer) -> Properties id "gooo://external-adoption/rune-properties/inspect" computes `let domain = input >= -2147483648 && input <= 2147483647
return Properties{in_rune_domain: domain, letter: unicode.IsLetter(input), digit: unicode.IsDigit(input)}`
SOURCE
cat > app/gooo.workspace.json <<'MANIFEST'
{
  "schema": "gooo/package-workspace-manifest/v1",
  "entry": {"package_path": "app/rune-properties", "activity": "Inspect"},
  "packages": [
    {"path": "go/unicode", "sources": ["vendor/gooo-go-ports/ports/unicode/space.gooo", "vendor/gooo-go-ports/ports/unicode/classification.gooo"]},
    {"path": "app/rune-properties", "sources": ["main.gooo"]}
  ]
}
MANIFEST
```

The manifest registers both vendored files as one `go/unicode` package. The
`unicode` alias in `main.gooo` then resolves the two imported activities. The
application's input key is `app/rune-properties:Inspect`.

An import does not download a package. All manifest source paths must remain
inside the application's workspace. A source path such as
`../ports-source/ports/unicode/space.gooo` is rejected with `must stay inside the
workspace`; absolute source paths are also rejected. The compiler executable may
remain in the sibling `ports-source` directory, as it does here.

Keep the canonical `gooo://std/integer` entity ID so the caller and the imported
package agree on the input type. The application's own result and activity IDs
can be changed for your project. Copy the `computes` body as shown: its first
token follows the opening backtick immediately, and the closing backtick follows
the final token without a trailing newline.

## Supply integer inputs

These inputs include letters, decimal digits, combining marks, other numeric
characters, supplementary characters, range members and holes, invalid Unicode,
and values that would change meaning if narrowed before validation.

```sh
cat > app/inputs.json <<'INPUTS'
{
  "schema": "gooo/body-composition-inputs/v1",
  "inputs": [
    {"app/rune-properties:Inspect": 65},
    {"app/rune-properties:Inspect": 233},
    {"app/rune-properties:Inspect": 44032},
    {"app/rune-properties:Inspect": 66560},
    {"app/rune-properties:Inspect": 48},
    {"app/rune-properties:Inspect": 1632},
    {"app/rune-properties:Inspect": 120792},
    {"app/rune-properties:Inspect": 769},
    {"app/rune-properties:Inspect": 178},
    {"app/rune-properties:Inspect": 188},
    {"app/rune-properties:Inspect": 8555},
    {"app/rune-properties:Inspect": 95},
    {"app/rune-properties:Inspect": 748},
    {"app/rune-properties:Inspect": 749},
    {"app/rune-properties:Inspect": 750},
    {"app/rune-properties:Inspect": 55296},
    {"app/rune-properties:Inspect": 1114111},
    {"app/rune-properties:Inspect": 1114112},
    {"app/rune-properties:Inspect": -1},
    {"app/rune-properties:Inspect": -2147483648},
    {"app/rune-properties:Inspect": 2147483647},
    {"app/rune-properties:Inspect": 4294967361},
    {"app/rune-properties:Inspect": -4294967231},
    {"app/rune-properties:Inspect": 4294967344},
    {"app/rune-properties:Inspect": 9007199254740993}
  ]
}
INPUTS
```

Inputs are original integers, not strings or whole text. Preserve them exactly;
do not first narrow them to 32 bits or round them through a JavaScript `Number`.
For example, `9007199254740993` cannot be represented exactly as a JavaScript
`Number`, but the JSON integer above is read exactly by the native command.

The record fields mean:

| Field | Meaning |
| --- | --- |
| `in_rune_domain` | The original integer is within Go's signed int32 `rune` domain, −2147483648 through 2147483647. This does not assert Unicode scalar validity. |
| `letter` | Go's Unicode `Letter` classification for that rune. |
| `digit` | Go's Unicode decimal digit category `Nd`, rather than every numeric character. |

For integers outside int32, this Gooo port returns false without narrowing. The
application also marks `in_rune_domain` false. This is a documented rejection
extension, not an input accepted by Go's `rune` API. Within int32, negative
values, surrogates, and values above U+10FFFF remain valid rune inputs to the
classification API and return false for both properties.

## Execute and save a replayable record

Run these commands from the original starting directory:

```sh
(cd app && ../ports-source/.gooo package execute --json --inputs inputs.json gooo.workspace.json) > proof/execution.json
(cd app && ../ports-source/.gooo package replay --json --receipt ../proof/execution.json --inputs inputs.json gooo.workspace.json) > proof/replay.json
```

The subshells enter `app`; the receipts are saved in its sibling `proof`
directory. Read each record at
`result.runtime.traces[].deliveries[].actual`. The expected records, in input
order, are:

| Input | Meaning | `in_rune_domain` | `letter` | `digit` |
| --- | --- | --- | --- | --- |
| `65` | Latin letter A | true | true | false |
| `233` | Latin letter é | true | true | false |
| `44032` | Hangul letter 가 | true | true | false |
| `66560` | Supplementary Deseret letter U+10400 | true | true | false |
| `48` | ASCII decimal digit 0 | true | false | true |
| `1632` | Arabic-Indic decimal digit U+0660 | true | false | true |
| `120792` | Supplementary decimal digit U+1D7D8 | true | false | true |
| `769` | Combining acute accent U+0301 | true | false | false |
| `178` | Superscript two U+00B2 | true | false | false |
| `188` | Vulgar fraction one quarter U+00BC | true | false | false |
| `8555` | Roman numeral twelve U+216B | true | false | false |
| `95` | Underscore | true | false | false |
| `748` | Letter range member U+02EC | true | true | false |
| `749` | Hole between range members U+02ED | true | false | false |
| `750` | Letter range member U+02EE | true | true | false |
| `55296` | Surrogate U+D800 | true | false | false |
| `1114111` | Maximum Unicode code point U+10FFFF | true | false | false |
| `1114112` | Above the Unicode range | true | false | false |
| `-1` | Negative rune | true | false | false |
| `-2147483648` | Minimum signed int32 | true | false | false |
| `2147483647` | Maximum signed int32 | true | false | false |
| `4294967361` | Outside int32; narrowing would alias A | false | false | false |
| `-4294967231` | Outside int32; narrowing would alias A | false | false | false |
| `4294967344` | Outside int32; narrowing would alias 0 | false | false | false |
| `9007199254740993` | Outside int32 and exact JavaScript integer range | false | false | false |

`IsDigit` excludes the superscript, fraction, and Roman numeral examples.
`IsLetter` excludes the combining mark and underscore. This application
classifies one integer at a time; it does not validate an entire identifier,
perform normalization, group graphemes, or check confusable characters. For a
consumer that applies Go's identifier-character rules to one rune, see the
[identifier-rune example](../examples/identifier-rune/README.md).

## Understand what the records establish

Input-only records report `decision: "OBSERVED"` and have no Go-reference
expectations (`finite_total` is zero). They record what the compiled Gooo
application returned. The saved replay's `replayed_from_sha256` identifies the
original execution receipt bytes; replay reproduces the saved program and runs
it again.

The 25 inputs above are a usage example. A separate verification of this exact
external application compared the 21 inputs within int32 with Go 1.27.2:
42 function comparisons for execution and the same 42 for saved replay. The
four wider integers were checked separately as rejection-extension inputs,
without converting them to Go runes. That check also bound original inputs,
source files, entry and record-field identities, the compiler, and generated
source to the receipts. Running the commands in this guide alone does not repeat
that independent comparison or add to the repository's differential test count.
This was an agent-authored external workspace check, not an independent user
adoption report or a proof over the whole input domain.

You can change `app/inputs.json` and replay the saved program on new inputs.
If you change the Gooo source, dependency files, or workspace entry, create a
fresh execution record before replaying it. The application does not need its
own Git checkout; preserve its relative source layout and vendored provenance
when moving it.

For the full repository's independent Go comparisons, follow the
[verification workflow](../README.md#검증-범위) from the source checkout. That
verifier expects the complete repository, rather than only this external app.
