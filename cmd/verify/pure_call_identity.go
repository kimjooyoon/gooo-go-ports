package main

import "fmt"

// These profiles are bound to the independently selected compiler source,
// never to IDs or a producer claim read from a receipt. Source839 derived both
// names; source205 preserves these ports' independently validated authored IDs.
// Explicit session276 keeps exactly the same body_calls.go identity producer
// as205 (raw SHA256 e4d5e38b7c591c8c3ffdbddc6da33fa4791dd122528c5e2ed23977b73422ed50).
type pureCallIdentityProfile bool

const legacyPureCallIdentityProfile pureCallIdentityProfile = false
const authoredPureCallCompilerRevision = "205896a413e5dea92366309c35ae85e542d055f6"
const legacyPureCallCompilerRevision = "83928fa4538245abe33b08549da47f8cbdc50ed2"

func selectPureCallIdentityProfile(revision string) (pureCallIdentityProfile, error) {
	switch revision {
	case authoredPureCallCompilerRevision, nativeSessionCompilerRevision:
		return true, nil
	case legacyPureCallCompilerRevision:
		return legacyPureCallIdentityProfile, nil
	default:
		return false, fmt.Errorf("unsupported pure-call compiler identity %q", revision)
	}
}
func (p pureCallIdentityProfile) original(namespace, name, authoredID string) string {
	if p {
		return authoredID
	}
	return decimalPayloadDerivedID(namespace, name)
}
func (p pureCallIdentityProfile) lowered(name, authoredID string) string {
	if p {
		return authoredID
	}
	return decimalPayloadDerivedID("gooo_workspace", name)
}
