# Go and JavaScript disagree on Unicode whitespace

A JavaScript form and a Go backend can disagree about whether an input is blank even when both call their standard trimming function. The two useful test cases are `U+0085` and `U+FEFF`.

This executable comparison uses **Go 1.27.2** and **Node.js 22.23.1**. The scripts and complete result sets are included in this directory. Prepared by **asketeddy**, with OpenAI Codex assistance; project affiliation is described at the end.

## Four characters to put in a regression test

Each row is a string containing exactly one character. `true` means trimming that string produces the empty string.

| Input | Go `strings.TrimSpace(s) == ""` | JavaScript `s.trim() === ""` |
| --- | --- | --- |
| `U+0085` NEXT LINE | `true` | `false` |
| `U+FEFF` ZERO WIDTH NO-BREAK SPACE | `false` | `true` |
| `U+200B` ZERO WIDTH SPACE | `false` | `false` |
| `U+00A0` NO-BREAK SPACE | `true` | `true` |

For example, if a frontend allows an input when `s.trim() !== ""`, a string containing only `U+0085` passes that check. A Go backend using `strings.TrimSpace(s) != ""` rejects it. `U+FEFF` reverses that result. This is a reproducible validation mismatch, not a reported production incident or a security finding.

## The definitions differ

Go's [`unicode.IsSpace`](https://pkg.go.dev/unicode#IsSpace) follows Unicode's `White_Space` property. The Go script also checks that `strings.TrimSpace` agrees with it for every single-scalar input in this experiment.

ECMAScript's [`trim`](https://tc39.es/ecma262/multipage/text-processing.html#sec-string.prototype.trim) uses the union of its `WhiteSpace` and `LineTerminator` sets. Its [white space definition](https://tc39.es/ecma262/multipage/ecmascript-language-lexical-grammar.html#sec-white-space) explicitly includes `U+FEFF`; `U+0085` belongs to neither set. JavaScript `\s` follows these ECMAScript sets too.

The JavaScript Unicode property escape `\p{White_Space}` is a different choice from `\s`. In the tested versions, `/^\p{White_Space}$/u` agrees with Go's `unicode.IsSpace` for every tested scalar value.

Choose the intended definition explicitly. For a JavaScript blank-input check using Unicode `White_Space`, an illustrative predicate is:

```js
const isBlank = s => s === "" || /^\p{White_Space}+$/u.test(s);
```

Use the same definition in the backend, and keep the four cases above in both test suites. Runtime upgrades can change Unicode data, so pin or recheck the versions relevant to your application.

## Reproduce the comparison

Download the files in this directory together. From that directory, with Go, Node.js 22.23.1 and Python 3.9 or later available, run:

```sh
GOTOOLCHAIN=go1.27.2 go run main.go > go-results.json
node compare.cjs > js-results.json
python3 verify.py
```

Go toolchain selection may download Go 1.27.2 if it is not cached. `verify.py` checks the recorded runtime versions, so a different Node version intentionally requires a new observation and an explicit verifier update.

The Go and JavaScript scripts independently enumerate **1,112,064 Unicode scalar values**: `U+0000` through `U+10FFFF`, excluding the surrogate range `U+D800` through `U+DFFF`. Each runtime trims 25 single-character strings to empty. Their sets share 24 values; Go additionally includes `U+0085`, while JavaScript additionally includes `U+FEFF`.

The verifier compares both independently produced sets, checks the four manually specified examples, checks JavaScript's Unicode property set against Go, and records source/result SHA-256 hashes in `comparison.json`. These hashes help identify the exact files; they are not signatures.

This experiment covers single-scalar strings in those runtime versions. It does not measure performance or validate normalization, grapheme handling, malformed UTF-8 or every possible multi-character string.

## Why this example lives beside Gooo work

I maintain [Gooo](https://github.com/kimjooyoon/meta-ontology-go), an experimental language, and its Go ports. Preserving Go's contract matters when a Gooo function runs inside a JavaScript host: substituting the host's `\s` predicate would change these results.

The [Unicode text inspector](https://kimjooyoon.github.io/gooo-go-ports/text-inspector.html) exposes this distinction through the compiled Gooo port. Its [source](https://github.com/kimjooyoon/gooo-go-ports/blob/75d0655101d1468f9a94eb10571d34378b98293b/ports/unicode/space.gooo) is available to inspect. The exhaustive comparison in this note is between Go and JavaScript; it is **not** an exhaustive execution of Gooo and adds no new Gooo compatibility claim.

The original code and text in this note are available under the included MIT license.
