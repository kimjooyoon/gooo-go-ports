// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
// Package catalogrefresh connects the generated Gooo whole-candidate decision
// to an immutable, in-memory catalog transaction. No external service is called.
package catalogrefresh

import (
	host "github.com/kimjooyoon/gooo-go-ports/examples/catalog-refresh/host"
	"github.com/kimjooyoon/gooo-go-ports/examples/catalog-refresh/internal/refreshkernel"
)

type Registry = host.Registry
type Attempt = host.Attempt
type Candidate = host.Candidate
type SnapshotPage = host.SnapshotPage
type Receipt = host.Receipt

// NewRegistry wires the actual generated Gooo activity. Initial bytes are opaque
// existing storage; they are never treated as proof of current authorization.
func NewRegistry(initial []byte) (*Registry, error) {
	return host.NewRegistry(initial, evaluateGooo)
}

// These conversions require identical field names, order and types at compile
// time. No Gooo eligibility policy is reimplemented in this adapter.
func evaluateGooo(f host.SnapshotFacts, e host.RefreshEvidence, s host.RefreshState) (host.RefreshDecision, error) {
	return host.RefreshDecision(refreshkernel.Evaluate(refreshkernel.SnapshotFacts(f), refreshkernel.RefreshEvidence(e), refreshkernel.RefreshState(s))), nil
}
