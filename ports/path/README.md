# Bounded slash paths in Gooo

`go/path:Ext`, `go/path:IsAbs` and `go/path:Clean` adapt Go 1.27.2 `path`
functions. Inputs must be explicitly present decoded valid UTF-8 Text of at most
1,024 bytes. All behavior uses slash paths independently of the host OS.
`Clean` returns a lexical path: empty becomes `.`, repeated slashes and `.`
components collapse, rooted parents stop at `/`, and relative leading parents
remain. Backslash, percent escapes, embedded NUL and Unicode lookalikes remain
ordinary characters. There is no Unicode normalization.

Gooo implements the candidate behavior. `CleanPart` and `CleanState` are internal
helpers replacing Go's lazy buffer with bounded Text construction; they are not
additional ported APIs. `path.gooo` supplies the shared Text/Boolean declarations
and existing `IsAbs` implementation. The [Clean workspace](../../path-clean.workspace.json)
selects only these two source files.

The [original alias facts example](../../examples/path-alias-facts/README.md)
preserves both raw paths and explains exact equality, lexical cleaning and alias
facts. A matching clean value is not router or filesystem identity, permission or
a safe-path certificate. This adaptation does not decode URLs, use Windows
`filepath` semantics, resolve symlinks or access a filesystem.

`clean-corpus-9841.json` contains every string over the ordered alphabet
`[".", "/", "a"]` of lengths zero through eight, in shortlex order. It stores
inputs only. Independent pinned Go `path.Clean` computes each expected value;
fresh native execution and saved replay are required separately. The finite
population is not all UTF-8 Text up to the bound. Arbitrary invalid-UTF-8 Go
strings and longer values are outside the adaptation, and no bare CLI raw
malformed-Unicode refusal is claimed.

[Provenance](provenance.json) and the repository `upstream.json` fix the immutable
Go revision, source hashes and compiler. The Go Authors' copyright and BSD terms
remain in `third_party/GO-LICENSE`; `third_party/go/path.go.txt` is the existing
byte-exact upstream reference. This independent project has no Go or Google
endorsement.
