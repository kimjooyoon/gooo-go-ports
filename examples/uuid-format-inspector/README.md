# UUID format inspector

Check the four UUID text formats accepted by Google's `google/uuid` v1.6.0 and
read the original format error. This importing Gooo program calls
`go/github.com/google/uuid:Validate(Text)` and returns four required fields:

| Field | Meaning |
| --- | --- |
| `valid` | Whether original `uuid.Validate` returns nil |
| `error` | The exact original error text, or an empty string on success |
| `invalid_length` | The original `uuid.IsInvalidLengthError` classification |
| `byte_length` | UTF-8 byte length of the original input |

The input is valid UTF-8 Text of at most 1,024 bytes. The four forms are a
36-byte hyphenated UUID, a 45-byte `urn:uuid:` form, a 38-byte braced form, and
32 hexadecimal bytes. The URN prefix is case-insensitive. Both braces and all
required hyphens are checked. Hexadecimal letters can be uppercase or lowercase.
All-zero and all-`f` values are accepted just as the original format validator
accepts them; format validity does not establish uniqueness or an authorization.

For `00112233-4455-6677-8899-aabbccddeeff`, the expected record is:

```json
{"valid":true,"error":"","invalid_length":false,"byte_length":36}
```

For an empty input:

```json
{"valid":false,"error":"invalid UUID length: 0","invalid_length":true,"byte_length":0}
```

## Execute and replay

Use Git, a POSIX shell and a Go installation with toolchain selection support.
From this repository's root, bootstrap builds the pinned Gooo 0.6.32-dev compiler
`83928fa4538245abe33b08549da47f8cbdc50ed2` using Go 1.27.2:

```sh
export GOTOOLCHAIN=go1.27.2
export GOWORK=off
./tools/bootstrap.sh
mkdir -p out/uuid-format-inspector
./.gooo package execute --json --inputs examples/uuid-format-inspector/inputs.json \
  uuid-format-inspector.workspace.json > out/uuid-format-inspector/execution.json
./.gooo package replay --json --receipt out/uuid-format-inspector/execution.json \
  --inputs examples/uuid-format-inspector/inputs.json uuid-format-inspector.workspace.json \
  > out/uuid-format-inspector/replay.json
```

Leave `--go` unset so native commands can select an installed matching Go 1.27.2
executable, including the toolchain cache populated during setup. A PATH launcher
for a different Go version is not necessarily that executable. Bootstrap keeps
an older compiler checkout intact; follow the repository's upgrade guidance
rather than replacing old receipts or their compiler.

The [12 input-only examples](inputs.json) include all four accepted forms,
all-zero and all-`f` values, invalid braces, a wrong hyphen, invalid hexadecimal,
prefix-error priority, a multibyte character cut by the first-nine-byte prefix,
and the exact 1,024-byte native input boundary. The input key is
`app/uuid-format-inspector:Inspect`. Actual records are at
`result.runtime.traces[].deliveries[].actual`.
Input-only receipts report `OBSERVED`; they do not include independent expected
outputs. Saved replay executes the recorded program again without model calls.

## API and provenance

The original [`google/uuid` Validate source](https://github.com/google/uuid/blob/0f11ee6918f41a04c201eceeadf612a377bc7fbc/uuid.go#L195)
uses a Go error interface. The Gooo port explicitly maps nil/error into
`{valid,error,invalid_length}` and preserves error order and text over the stated
input domain. This consumer adds only original byte length; all four fields are
computed in Gooo. It generates no UUID, calls no randomness or network service,
and does not parse a 16-byte UUID value.

See [Google UUID scope and source records](../../docs/google-uuid.md) for the
original module checksum, source bindings, licenses and internal quoting helper.
Go strings can contain invalid UTF-8 and have no 1,024-byte length limit; this
native Text interface does not preserve that whole domain. JSON decoding repairs
invalid UTF-8 and lone surrogate spellings before this interface; they must not
be presented as preserved arbitrary Go bytes.
