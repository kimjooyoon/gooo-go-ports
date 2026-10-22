# Source and attribution

The candidate admission and selection activity in
`ports/service-policy/candidate_selection.gooo` is original Gooo Go Ports work
under the repository BSD-3-Clause license. The generated internal Go kernel is
derived from that source by the pinned Gooo compiler, rather than a handwritten
Go selection algorithm.

The public API imports the immutable `service-grant-many` module at
`v0.0.0-20261020182808-7180a4a0f36a`. Its generation and original attribution are
retained in `policy-generation.json` and that module's notices, including the
Apache-2.0 attribution for the adapted Google AgentMesh service-pattern sources.
This example does not copy those policy implementations into a new Go wrapper.

The lossless JSON transport helper functions come from the same pinned module's
BSD-3-Clause `input.go`; `transport-source-provenance.json` records the exact
source bytes, selected functions and mechanical budget-name transformation.
