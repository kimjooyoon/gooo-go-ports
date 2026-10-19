# Source and license notice

The original ordered multi-required-request policy in
`../../ports/service-policy/multi_required.gooo` is governed by the repository
BSD-3-Clause LICENSE, copied byte-exact to `LICENSE`. It does not apply a grant, authenticate
live authority, or promise full AgentMesh/CEL compatibility.

The matcher at `../../ports/agentmesh/service_pattern.gooo` is the existing
bounded Google Agent Mesh adaptation, byte-exact from gooo-go-ports 00262a205c93eff646c53167bbcb5b06cd001c21.
The original Google LLC copyright and Apache-2.0 license remain preserved at
module-local `LICENSE-APACHE-2.0` (byte-exact copy); immutable upstream is
https://github.com/google/agentmesh/blob/9fd1a41a15174df0aaef601c942410c2116bff91/api/tar.go .
The preserved source pin is `../agentmesh-service-policy/source-pin.json`.
Original source archives remain available in the existing public example:
https://github.com/kimjooyoon/gooo-go-ports/tree/00262a205c93eff646c53167bbcb5b06cd001c21/examples/agentmesh-service-policy/upstream .
The domain, refinement and checked-refinement files beside that source pin are
original Gooo policy dependencies copied byte-exact, covered by repository BSD.
Their imported IDs and behavior are unchanged.

Compiler de99/Go 1.27.2 is a separate example profile; it does not replace the
repository's 839 pin. Compiler binaries are not bundled into this source example.
Source hashes and copy provenance are recorded in source-provenance.json.
The existing 16 reference decision objects are retained expectations from an
accepted independent Go oracle. The initial public-ID generation phase reused those frozen expectations; its
record of zero fresh oracle runs remains historical. The separate test-only
reference module permits a fresh 17-case comparison, including the 257-request
limit. Private receipts are not receipts for these public IDs. Actual lineage
limitations remain reported separately.
