# Use a port in your own Gooo project

This walkthrough builds a separate application that imports `utf8.ValidRune`.
It includes the complete workspace and input files; the port's calculation stays
in Gooo. No model or inference service is needed.

Use Git, a POSIX shell, and Go with toolchain selection support, or an installed
Go **1.27.2**. The commands explicitly select that exact toolchain for this shell;
the first run may download it. Start in a working directory where neither
`gooo-go-ports-source` nor `my-gooo-app` exists. Run the commands in order in the
same shell, and stop if a command fails.

## Get a fixed source version and build its compiler

This guide uses ports commit `f618012e973ad5bcfb8b14f7baa72462f6629191` and its
compiler pin `366ee6ef8f9d2b9b57331dfd26b6f908c0289851`. The compiler is built from
that clean source checkout. A moving branch or the `0.6.24-dev` version label
alone does not identify this combination.

```sh
export GOTOOLCHAIN=go1.27.2
export GOWORK=off
test "$(go env GOVERSION)" = go1.27.2
mkdir gooo-go-ports-source
git -C gooo-go-ports-source init
git -C gooo-go-ports-source remote add origin https://github.com/kimjooyoon/gooo-go-ports.git
git -C gooo-go-ports-source fetch --depth=1 origin f618012e973ad5bcfb8b14f7baa72462f6629191
git -C gooo-go-ports-source checkout --detach FETCH_HEAD
test "$(git -C gooo-go-ports-source rev-parse HEAD)" = f618012e973ad5bcfb8b14f7baa72462f6629191
./gooo-go-ports-source/tools/bootstrap.sh
```

`GOTOOLCHAIN` selects Go 1.27.2 even when a newer Go is installed; a module's
minimum Go version alone would not downgrade it. `GOWORK=off` keeps an enclosing
Go workspace from affecting these commands. Repeat these two exports if you
continue in a new terminal. Keep this source directory beside your application:
it supplies the compiler executable,
while the application's imported Gooo sources will live inside its own workspace.

## Create the application and copy the dependency

```sh
mkdir my-gooo-app
cd my-gooo-app
test "$(go env GOVERSION)" = go1.27.2
mkdir -p vendor/gooo-go-ports/ports/unicode vendor/gooo-go-ports/third_party
cp ../gooo-go-ports-source/ports/unicode/utf8.gooo vendor/gooo-go-ports/ports/unicode/
cp ../gooo-go-ports-source/LICENSE ../gooo-go-ports-source/NOTICE \
  ../gooo-go-ports-source/upstream.json vendor/gooo-go-ports/
cp ../gooo-go-ports-source/third_party/GO-LICENSE vendor/gooo-go-ports/third_party/
printf '%s\n' f618012e973ad5bcfb8b14f7baa72462f6629191 > vendor/gooo-go-ports/REVISION
cmp ../gooo-go-ports-source/ports/unicode/utf8.gooo vendor/gooo-go-ports/ports/unicode/utf8.gooo
```

This Gooo application does not need a `go.mod`. Its imports are resolved by the
workspace below, not by Go's module or vendor resolver. Go is the compiler's
native execution toolchain; no Go implementation of `ValidRune` is added to your
application.

Keep the copied port's copyright comments and the license files together.
`upstream.json` records the original Go source revision, hashes, and the full
ports index; copying it does not install every listed port. `REVISION` identifies
the Gooo port snapshot. The final `cmp` checks that the copied Gooo source is
byte-for-byte identical to that checkout. The original Go hashes in
`upstream.json` are a different kind of evidence.

## Add your Gooo source, workspace, and inputs

```sh
cat > main.gooo <<'EOF'
package checker
namespace checker
import utf8 "go/unicode/utf8"
entity Integer id "gooo://std/integer"
entity Boolean id "gooo://std/boolean"
activity Valid(Integer) -> Boolean id "gooo://example/checker/valid" computes `return utf8.ValidRune(input)`
EOF

cat > gooo.workspace.json <<'EOF'
{
  "schema": "gooo/package-workspace-manifest/v1",
  "entry": {"package_path": "app/checker", "activity": "Valid"},
  "packages": [
    {"path": "go/unicode/utf8", "sources": ["vendor/gooo-go-ports/ports/unicode/utf8.gooo"]},
    {"path": "app/checker", "sources": ["main.gooo"]}
  ]
}
EOF

cat > inputs.json <<'EOF'
{
  "schema": "gooo/body-composition-inputs/v1",
  "inputs": [
    {"app/checker:Valid": 65},
    {"app/checker:Valid": 55296},
    {"app/checker:Valid": 65533},
    {"app/checker:Valid": 1114111},
    {"app/checker:Valid": 1114112},
    {"app/checker:Valid": -2147483648},
    {"app/checker:Valid": 2147483647}
  ]
}
EOF
```

These names serve different purposes:

| Name | Meaning |
| --- | --- |
| `utf8` | Alias used in your source, as in `utf8.ValidRune(input)` |
| `go/unicode/utf8` | Imported package path, registered in `packages` |
| `vendor/gooo-go-ports/ports/unicode/utf8.gooo` | Actual source file, relative to the workspace manifest |
| `app/checker` and `Valid` | Your application's package path and entry activity |
| `app/checker:Valid` | Input key for this single-argument entry |

An import does not download a package. Every source path must stay inside the
workspace: absolute paths and `../` references to a sibling checkout are rejected
by the pinned compiler. This is why the port is copied into the application.
Keep the standard Integer and Boolean entity IDs so the caller and port agree
on their types. The example's own activity ID can be replaced with one for your
project when you change its source.

The inputs are integer Unicode code points, not text strings. This port preserves
Go's signed int32 `rune` domain; use values from −2147483648 to 2147483647 without
narrowing them first. `ValidRune` tests whether a code point is a Unicode scalar
value. It does not test whether an entire byte string is valid UTF-8.

## Run it and save a replayable record

From `my-gooo-app`, run:

```sh
../gooo-go-ports-source/.gooo package execute --inputs inputs.json gooo.workspace.json
```

The output is one JSON Boolean per input, in order:

```text
true
false
true
true
false
false
false
```

`55296` is a surrogate code point. `65533` is the valid replacement character
U+FFFD. `1114111` is the largest Unicode scalar value, while `1114112` exceeds
the Unicode range. The int32 endpoints are also invalid scalar values.

Save the full execution and replay records with:

```sh
mkdir out
../gooo-go-ports-source/.gooo package execute --json --inputs inputs.json \
  gooo.workspace.json > out/execution.json
../gooo-go-ports-source/.gooo package replay --json --receipt out/execution.json \
  --inputs inputs.json gooo.workspace.json > out/replay.json
```

Input-only records report `decision: "OBSERVED"`. They contain actual values at
`result.runtime.traces[].deliveries[].actual`; they do not claim to have passed
Go-reference comparisons. These seven inputs are a usage example and do not add
to this repository's 5,510-case differential verification count. The replay's
`replayed_from_sha256` identifies the original execution receipt bytes.

You can edit `inputs.json` and replay the saved program on the new inputs. If you
change the Gooo source, dependency, or workspace entry, create a fresh execution
record before replaying it. The application does not need its own Git checkout;
keep the relative source layout and vendored provenance when moving or sharing it.

For a multi-argument consumer, see the [color inspector](../examples/color-inspector/README.md)
and its input keys ending in `.input0`, `.input1`, and `.input2`. For the full
repository's independent Go comparisons, use the [verification workflow](../README.md#검증-범위)
from the source checkout; that verifier expects the complete repository.
