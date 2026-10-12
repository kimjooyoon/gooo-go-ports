# Google UUID canonical parsing

`ParseCanonical` exposes the result of Google's `uuid.Parse` and the returned
UUID's `String` method through a Gooo record. It preserves the returned value
even when parsing fails, alongside the original error text and length-error
classification. Use the `parsed` field to decide whether parsing succeeded.

The original library is `github.com/google/uuid` v1.6.0 at revision
`0f11ee6918f41a04c201eceeadf612a377bc7fbc`. This is a separate Text API adaptation:
Go returns `(UUID, error)`, with UUID represented by a 16-byte array; Gooo returns
the array's canonical text representation and three error observations.

## API and fields

Package: `go/github.com/google/uuid/text`.
Activity: `ParseCanonical(Text) -> ParseCanonicalResult`.
Activity ID: `gooo://github.com/google/uuid/text/parse-canonical`.
Result entity: `gooo://github.com/google/uuid/text/parse-canonical-result`.

| Required field | Type | Original Go observation |
| --- | --- | --- |
| `parsed` | Boolean | `err == nil` after `uuid.Parse(input)` |
| `canonical_uuid` | Text | The returned `UUID.String()`, on success and error |
| `error` | Text | Exact `err.Error()`, or an empty string for nil |
| `invalid_length` | Boolean | `uuid.IsInvalidLengthError(err)` |

Fields have the IDs of the result entity followed by `/parsed`,
`/canonical-uuid`, `/error` and `/invalid-length`, in that order.

For `00112233445566778899aabbccddeeff`, the result is:

```json
{"parsed":true,"canonical_uuid":"00112233-4455-6677-8899-aabbccddeeff","error":"","invalid_length":false}
```

For `00112233445566778899aabbccddee0g`, parsing fails while the returned array
still has this text representation:

```json
{"parsed":false,"canonical_uuid":"00112233-4455-6677-8899-aabbccddeeff","error":"invalid UUID format","invalid_length":false}
```

A correctly shaped `canonical_uuid` therefore does not imply success. The value
on failure preserves the original return data; callers must check `parsed`.

## Parsing and format checking

The original [Parse implementation](https://github.com/google/uuid/blob/0f11ee6918f41a04c201eceeadf612a377bc7fbc/uuid.go#L68)
accepts hyphenated, URN, raw hexadecimal and 38-byte wrapped forms. For a
38-byte input, it examines the middle 36 bytes without requiring braces.
`[00112233-4455-6677-8899-aabbccddeeff]` can therefore parse successfully.
The separate [Validate port](google-uuid.md) requires the two braces for that
length and reports `invalid bracketed UUID format` for this input.

Length and URN-prefix checks come first. For a standard-form body, all four
hyphens are checked before decoding any pair; a bad hyphen returns the zero
UUID. A later hexadecimal error retains previously decoded bytes. In the raw
32-byte form, the failing pair's byte value is also written before the error
is returned. The [String implementation](https://github.com/google/uuid/blob/0f11ee6918f41a04c201eceeadf612a377bc7fbc/uuid.go#L244)
serializes all 16 returned bytes, including zero or partially decoded values.

The Gooo implementation calculates the bytes, text and error results directly.
It does not use `Validate` as a parsing gate or call Go's parser as a wrapper.

## Input and compatibility scope

The input is decoded valid UTF-8 Text of at most 1,024 bytes. Lengths and
positions are measured in UTF-8 bytes. Inputs longer than this native boundary
are rejected before this activity executes. JSON decoding can replace invalid
UTF-8 or lone surrogate spellings; this API does not preserve arbitrary Go byte
strings or malformed wire encodings.

This record preserves the selected observations of `Parse` plus `String`.
It does not expose Go's binary UUID array, its method set, `ParseBytes`, an error
object or generic `errors.As` behavior. Parsing neither generates a UUID nor
establishes uniqueness, authorization or a particular UUID version.

## Run the example and inspect the source

Follow the [execute and saved replay example](../examples/uuid-parse-canonical/README.md).
It includes the permissive wrapped form and the distinct partial values from
raw and hyphenated parse failures. This API runs in the local native toolchain;
the existing 21-function browser tools do not include it.

Read [parse_canonical.gooo](../ports/google/uuid/text/parse_canonical.gooo) and
the [original source, checksums and licenses](google-uuid.md#source-checksums-and-licenses).
The original 31 module files and their module checksum remain unchanged.
The added source-hash wrapper is separate from that original archive.

The native verifier compares 21,867 distinct decoded Text inputs directly with
the original `Parse` and `String` results and verifies saved replay separately.
It reuses the existing 21,745 UUID inputs and adds 122 distinct parser anchors
after deduplicating 136 additional examples. The new suite has 325 bounded JSON
batches; the preceding 56 suites and 1,416 batch boundaries remain unchanged.
Finite observations establish those cases, rather than exhaustive compatibility.
