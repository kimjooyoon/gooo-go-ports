// Code generated from the source-bound Gooo typed plan; DO NOT EDIT.
package refreshkernel

type SnapshotFacts struct {
	PageRequestCursorKinds  []string `json:"page_request_cursor_kinds"`
	PageRequestCursorValues []string `json:"page_request_cursor_values"`
	PageNextCursorKinds     []string `json:"page_next_cursor_kinds"`
	PageNextCursorValues    []string `json:"page_next_cursor_values"`
	OriginalNames           []string `json:"original_names"`
	DefinitionIDs           []string `json:"definition_ids"`
	DefinitionProfiles      []string `json:"definition_profiles"`
	AnnotationIDs           []string `json:"annotation_ids"`
	RowPageIndexes          []int64  `json:"row_page_indexes"`
}

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
