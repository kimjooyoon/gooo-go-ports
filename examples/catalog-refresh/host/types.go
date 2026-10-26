// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
// Package catalogrefresh demonstrates a local catalog replacement transaction.
// It performs no live collection, remote atomic snapshot or tool invocation.
package catalogrefresh

import recordedcatalog "github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison"

const ValidatorProfile = "gooo/mcp-tool-shape/v1"

// SnapshotFacts and SnapshotPage reuse the strict exact-key saved-page projection.
type SnapshotFacts = recordedcatalog.SnapshotFields
type SnapshotPage = recordedcatalog.SnapshotPage

type RefreshEvidence struct {
	AttemptID           string   `json:"attempt_id"`
	CandidateID         string   `json:"candidate_id"`
	CollectorState      string   `json:"collector_state"`
	BindingState        string   `json:"binding_state"`
	ValidatorProfile    string   `json:"validator_profile"`
	RowDefinitionIDs    []string `json:"row_definition_ids"`
	RowValidationStates []string `json:"row_validation_states"`
}

type RefreshState struct {
	ExpectedRegistryVersion   string `json:"expected_registry_version"`
	CurrentRegistryVersion    string `json:"current_registry_version"`
	ExpectedScopeEpoch        string `json:"expected_scope_epoch"`
	CurrentScopeEpoch         string `json:"current_scope_epoch"`
	ExpectedInvalidationEpoch string `json:"expected_invalidation_epoch"`
	CurrentInvalidationEpoch  string `json:"current_invalidation_epoch"`
	ExpectedLatestAttempt     string `json:"expected_latest_attempt"`
	CurrentLatestAttempt      string `json:"current_latest_attempt"`
}

type RefreshDecision struct {
	State                     string `json:"state"`
	Reason                    string `json:"reason"`
	CandidateID               string `json:"candidate_id"`
	AttemptID                 string `json:"attempt_id"`
	ExpectedRegistryVersion   string `json:"expected_registry_version"`
	ExpectedScopeEpoch        string `json:"expected_scope_epoch"`
	ExpectedInvalidationEpoch string `json:"expected_invalidation_epoch"`
	ExpectedLatestAttempt     string `json:"expected_latest_attempt"`
	CandidatePageCount        int64  `json:"candidate_page_count"`
	CandidateRowCount         int64  `json:"candidate_row_count"`
	Coverage                  string `json:"coverage"`
	CoverageReason            string `json:"coverage_reason"`
	ProblemPageIndex          int64  `json:"problem_page_index"`
	ProblemRowIndex           int64  `json:"problem_row_index"`
	DecisionScope             string `json:"decision_scope"`
	ExecutionAuthorized       bool   `json:"execution_authorized"`
	LiveEffectPerformed       bool   `json:"live_effect_performed"`
	RemoteAtomicity           string `json:"remote_atomicity"`
}

// Evaluator is the narrow generated-Gooo adapter boundary. Production callers
// must supply the generated activity, not a host-authored eligibility predicate.
// Inputs are private copies, so the adapter cannot mutate the staged candidate.
type Evaluator func(SnapshotFacts, RefreshEvidence, RefreshState) (RefreshDecision, error)

// StoredCatalog is a copied observation of local storage. Retention is not
// permission to expose or invoke a tool, and scope changes may revoke such use.
type StoredCatalog struct {
	Bytes             []byte `json:"bytes"`
	SHA256            string `json:"sha256"`
	RegistryVersion   string `json:"registry_version"`
	ScopeEpoch        string `json:"scope_epoch"`
	InvalidationEpoch string `json:"invalidation_epoch"`
	LatestAttempt     string `json:"latest_attempt"`
}

// Receipt separates the actual locked storage outcome from the pure decision.
// Decision can remain CANDIDATE_ELIGIBLE when a later CAS refuses the swap.
type Receipt struct {
	Sequence        uint64           `json:"sequence"`
	State           string           `json:"state"`
	Reason          string           `json:"reason"`
	AttemptID       string           `json:"attempt_id"`
	CandidateID     string           `json:"candidate_id"`
	BeforeVersion   string           `json:"before_version"`
	AfterVersion    string           `json:"after_version"`
	BeforeSHA256    string           `json:"before_sha256"`
	AfterSHA256     string           `json:"after_sha256"`
	EvaluationState RefreshState     `json:"evaluation_state"`
	CommitState     RefreshState     `json:"commit_state"`
	Decision        *RefreshDecision `json:"decision,omitempty"`
	Error           string           `json:"error,omitempty"`
}
