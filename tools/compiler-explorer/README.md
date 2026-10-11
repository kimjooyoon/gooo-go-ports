# Experimental Compiler Explorer adapter

This directory carries a reproducible prototype for inspecting generated Go from
one [Gooo](https://github.com/kimjooyoon/meta-ontology-go) source file. It is an
independent Gooo ecosystem contribution. The
[upstream language request](https://github.com/compiler-explorer/compiler-explorer/issues/9258)
is under review; this directory does not establish support on godbolt.org.

The adapter runs only `gooo body-codegen --activity NAME` and preserves the
generated Go, including semantic-ID comments. It rejects execution, binary
output, additional files, libraries, tools, compiler overrides and other user
flags. It does not compile or execute the generated Go. Loop bodies and other
unsupported statements produce ordinary compiler diagnostics.

`pins.json` identifies the exact Compiler Explorer base, patch, resulting source
files, Node/npm versions and published Gooo Linux archive. The compiler matches
this repository's pinned source: `90ff2bb5aeffc2ac553496b646d14db1e8d33fdb`,
Gooo `0.6.29-dev`, built with Go `1.27.2`. This is a development release.

On Linux x86_64 with Node 22.23.1, npm 11.2.0, Git, curl, Python 3 and GNU
binutils, run from this repository's root:

```sh
bash tools/compiler-explorer/verify-linux.sh
```

The verifier downloads the pinned source and compiler into a disposable scratch
directory. It verifies their identities, applies the patch, checks types/style
and tests, builds the frontend, and starts a loopback-only CE server for eleven
actual-compiler API cases. Successful generated Go must match the saved CLI
outputs byte for byte; invalid requests must fail without stale generated code.
The server is stopped after the run. No native Go installation is needed for
this source-generation proof.

The [Linux workflow](../../.github/workflows/compiler-explorer.yml) preserves
identity, checks, raw observations and a summary under `out/compiler-explorer-linux`.
Its test results are evidence only after the run succeeds. This proof does not
claim hosted installation, customer adoption, full Gooo syntax support, native
execution or compatibility beyond the recorded cases.

The patch is distributed under Compiler Explorer's BSD-2-Clause license,
retained in `LICENSE-compiler-explorer` and the source headers. Compiler Explorer
is an independent upstream project. The remaining verifier files follow this
repository's license.
