# Use the current path ports in a separate Gooo project

This walkthrough copies a working Gooo application into its own directory. Its
`Report` activity imports `path.Ext` and `path.IsAbs` and returns six required
fields: the original text, extension, absolute marker, both byte lengths and
whether an extension exists. The calculations stay in Gooo. The application has
no `go.mod`, Go implementation, model service or Git checkout of its own.

This guide fixes the ports source at
[`52512fa0d589ecdfc8a1fa51d5e1b4e0ce11bd7b`](https://github.com/kimjooyoon/gooo-go-ports/tree/52512fa0d589ecdfc8a1fa51d5e1b4e0ce11bd7b),
with compiler `83928fa4538245abe33b08549da47f8cbdc50ed2` and Go **1.27.2**.
The `0.6.32-dev` version label alone does not identify this combination. The older
[ValidRune walkthrough](use-in-your-project.md) deliberately uses preview.2 and
its older compiler; its receipts are separate from this example.

## Obtain the fixed source and compiler

Use Git, a POSIX shell and either Go with automatic toolchain selection or an
installed Go 1.27.2. The first setup may download the selected toolchain and
compiler sources. Choose a working directory where `gooo-go-ports-source` and
`my-gooo-path-app` do not exist. Run these commands in order in the same shell;
stop if any command fails.

```sh
export GOTOOLCHAIN=go1.27.2
export GOWORK=off
test "$(go env GOVERSION)" = go1.27.2

mkdir gooo-go-ports-source
git -C gooo-go-ports-source init
git -C gooo-go-ports-source remote add origin https://github.com/kimjooyoon/gooo-go-ports.git
git -C gooo-go-ports-source fetch --depth=1 origin 52512fa0d589ecdfc8a1fa51d5e1b4e0ce11bd7b
git -C gooo-go-ports-source checkout --detach FETCH_HEAD
test "$(git -C gooo-go-ports-source rev-parse HEAD)" = 52512fa0d589ecdfc8a1fa51d5e1b4e0ce11bd7b
./gooo-go-ports-source/tools/bootstrap.sh

GOOO_BIN="$(pwd)/gooo-go-ports-source/.gooo"
GOOO_GO="$(go env GOROOT)/bin/go"
"$GOOO_BIN" version --build --json
```

The build identity must report the compiler revision above, `CLEAN_VCS`,
`vcs_modified: "false"` and `go1.27.2`. `GOOO_BIN` and `GOOO_GO` are absolute
paths so changing into the application directory cannot select another compiler
or Go executable. Keep the source checkout: it supplies the compiler and the
independent verification tools.

## Copy a complete Gooo workspace

```sh
mkdir -p my-gooo-path-app/ports/path
mkdir -p my-gooo-path-app/examples/path-facts
mkdir -p my-gooo-path-app/third_party/go

cp gooo-go-ports-source/ports/path/path.gooo my-gooo-path-app/ports/path/
cp gooo-go-ports-source/examples/path-facts/main.gooo \
  gooo-go-ports-source/examples/path-facts/inputs.json \
  my-gooo-path-app/examples/path-facts/
cp gooo-go-ports-source/path-facts.workspace.json my-gooo-path-app/
cp gooo-go-ports-source/LICENSE gooo-go-ports-source/NOTICE \
  gooo-go-ports-source/upstream.json my-gooo-path-app/
cp gooo-go-ports-source/third_party/GO-LICENSE my-gooo-path-app/third_party/
cp gooo-go-ports-source/third_party/go/path.go.txt my-gooo-path-app/third_party/go/
printf '%s\n' 52512fa0d589ecdfc8a1fa51d5e1b4e0ce11bd7b > my-gooo-path-app/REVISION

cmp gooo-go-ports-source/ports/path/path.gooo my-gooo-path-app/ports/path/path.gooo
cmp gooo-go-ports-source/examples/path-facts/main.gooo my-gooo-path-app/examples/path-facts/main.gooo
cmp gooo-go-ports-source/path-facts.workspace.json my-gooo-path-app/path-facts.workspace.json
cmp gooo-go-ports-source/examples/path-facts/inputs.json my-gooo-path-app/examples/path-facts/inputs.json

test ! -e my-gooo-path-app/go.mod
test ! -e my-gooo-path-app/.git
cd my-gooo-path-app
```

The workspace registers exactly two packages:

| Package | Source inside the application | Purpose |
| --- | --- | --- |
| `go/path` | `ports/path/path.gooo` | The imported `Ext` and `IsAbs` activities |
| `app/path-facts` | `examples/path-facts/main.gooo` | The original six-field `Report` activity |

`path-facts.workspace.json` selects `app/path-facts:Report`. Its source contains
`import paths "go/path"`; `paths` is the local alias. Importing does not download
anything. Every source named by the manifest lives inside the copied application.
The compiler rejects source paths that escape that workspace.

Keep the copyright comments and copied license/provenance files together.
`REVISION` identifies the port snapshot. `upstream.json` records the original Go
source revision and hashes as well as the full repository index; it does not
install the other indexed ports. `third_party/go/path.go.txt` is an attributed
source reference, not an implementation executed by this application.

## Execute the example and replay its saved program

From `my-gooo-path-app`, run:

```sh
mkdir out
"$GOOO_BIN" package execute --json --go "$GOOO_GO" \
  --inputs examples/path-facts/inputs.json path-facts.workspace.json \
  > out/execution.json

"$GOOO_BIN" package replay --json --go "$GOOO_GO" \
  --receipt out/execution.json --inputs examples/path-facts/inputs.json \
  path-facts.workspace.json > out/replay.json
```

The supplied file contains 24 ordered inputs. Each input uses the key
`app/path-facts:Report`; no expected result is supplied to the compiler. Inspect
`result.runtime.traces[].deliveries[].actual` in each record. For example:

| Original text | `ext` | `is_abs` | `text_byte_length` | `ext_byte_length` | `has_extension` |
| --- | --- | --- | ---: | ---: | --- |
| empty string | empty string | false | 0 | 0 | false |
| `/` | empty string | true | 1 | 0 | false |
| `.` | `.` | false | 1 | 1 | true |
| `..` | `.` | false | 2 | 1 | true |
| `.env` | `.env` | false | 4 | 4 | true |
| `a.b.c` | `.c` | false | 5 | 2 | true |
| `dir.x/a` | empty string | false | 7 | 0 | false |

All six fields are required, including empty strings, zeroes and false values.
The `text` field retains the original input. Slash semantics apply on every OS;
backslash is ordinary text. Byte lengths count UTF-8 bytes, not displayed
characters. Inputs must be valid Unicode text of at most 1,024 UTF-8 bytes. These
are supplied string facts: the report neither reads a filesystem nor authorizes
access to one.

Both input-only records report `decision: "OBSERVED"` and zero finite
expectations. The replay's `replayed_from_sha256` binds the original execution
receipt bytes. These labels do not independently establish a Go-reference
comparison. The repository's strict checker separately rejects malformed raw
UTF-8, duplicate fields and unpaired JSON surrogate escapes; the bare pinned
CLI's escape normalization is a separate behavior.

## Edit and verify

Change the 24 example values in `examples/path-facts/inputs.json` to try your own
in-domain inputs. A saved program can be replayed with new inputs. If you change
Gooo source, package names, semantic IDs, source layout or the workspace entry,
create a fresh execution receipt first. Renaming `app/path-facts:Report` also
requires updating the manifest and input keys together.

To run the existing independent verification suite, return to the source
checkout and choose a new evidence directory:

```sh
cd ../gooo-go-ports-source
python3 -B tools/check-path-facts.py --repository . --go "$GOOO_GO" \
  --compiler "$GOOO_BIN" --out out/path-guide-verification --timeout 300
```

This optional command requires Python 3 and the complete source checkout. It
checks that checkout's unedited examples, not a modified external application.
It compares 24 actual CLI outputs and 24 saved-replay outputs for each of the
three entries, then separately compares all 9,841 `{a,.,/}` strings of lengths
0..8 through each fresh generated native function. The existing verifier expects
its exact source and manual corpus; editing your copy does not expand that
finite compatibility claim.

For this walkthrough, a separate native check ran the unchanged copied Report
workspace outside the repository, without a `go.mod` or `.git`: 24 actual CLI
outputs and 24 saved-replay outputs matched the Go reference on all six fields.
Its fresh generated Report function also matched 9,841 ordered inputs on 59,046
scalar comparisons. The source bytes and relative layout were preserved; this
observation does not cover arbitrary edits, all UTF-8 strings, the setup network
download or another compiler version.
