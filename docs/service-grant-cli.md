# Service-grant checks without a Go installation

The native `gooo-service-grant-evaluate` command reads supplied service-pattern
facts from standard input. It reports whether the supported narrowing preserves
one required request. It applies no grant and contacts no service.

Native bundles target **Linux amd64** and **macOS arm64**. This distribution is
currently a release candidate: the packaging workflow produces a checked Linux
artifact on relevant pull requests and manual runs. A published release download
will be linked here after both native artifacts are reviewed. The existing
[Go installation route](../examples/agentmesh-service-policy/README.md#use-the-generated-ordinary-go-library-and-stdin-cli)
remains available.

Download the archive for your machine and its matching `.tar.gz.sha256` file
from the same release or reviewed workflow artifact. Keep them together. Check
the checksum before extracting into a fresh private directory:

```sh
# Linux amd64
sha256sum -c gooo-service-grant-evaluate-linux-amd64.tar.gz.sha256
# macOS arm64
shasum -a 256 -c gooo-service-grant-evaluate-darwin-arm64.tar.gz.sha256
```

Stop if that check fails. Extract only the matching archive you checked:

```sh
umask 077
grant_run="$(mktemp -d)" && test -n "$grant_run" && test -d "$grant_run" &&
  tar -xzf gooo-service-grant-evaluate-linux-amd64.tar.gz -C "$grant_run"
# On macOS, use the darwin-arm64 archive in the command above.
```

From the extracted directory, run:

```sh
./gooo-service-grant-evaluate -mode grant-update < eligible-facts.json
./gooo-service-grant-evaluate -mode grant-update < required-removed-facts.json
./gooo-service-grant-evaluate -mode grant-update < domain-invalid-facts.json
```

The respective decisions are `ELIGIBLE`, `REFUSED` and `INVALID_INPUT`, all with
exit status 0. Each result preserves the four supplied facts, seven typed decision
fields and compiler/generation/source bindings. `duplicate-key.json` instead
causes a structural error: exit 1, no result on stdout and a diagnostic on stderr.
Preserve exit status and both streams. Use copies of these inputs to try your own
synthetic requests; an `ELIGIBLE` decision does not authenticate those facts.

No Go, Gooo compiler or Python installation is required by the extracted command.
`BUNDLE.json` records the immutable public module version, source checksums,
platform, executable identity and the four finite checks. The BSD-3-Clause ports
license, AgentMesh Apache-2.0 attribution and Go license accompany the executable.
The Gooo license is included as a generation-tool reference; no language engine
binary is included. These checks do not establish live authorization or external
adoption.
