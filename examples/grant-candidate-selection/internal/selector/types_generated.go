// Code generated from the Gooo typed plan; DO NOT EDIT.
package selector

type Input struct {
	Phase            string   `json:"phase"`
	ContextID        string   `json:"context_id"`
	CandidateIDs     []string `json:"candidate_ids"`
	AssessmentIDs    []string `json:"assessment_ids"`
	ContextIDs       []string `json:"context_ids"`
	Statuses         []string `json:"statuses"`
	Priorities       []int64  `json:"priorities"`
	BindingsVerified []bool   `json:"bindings_verified"`
}

type Decision struct {
	ContextID            string `json:"context_id"`
	State                string `json:"state"`
	Reason               string `json:"reason"`
	SelectedCandidateID  string `json:"selected_candidate_id"`
	SelectedAssessmentID string `json:"selected_assessment_id"`
	SelectedIndex        int64  `json:"selected_index"`
	SelectedPriority     int64  `json:"selected_priority"`
	CandidateCount       int64  `json:"candidate_count"`
	EligibleCount        int64  `json:"eligible_count"`
	InvalidIndex         int64  `json:"invalid_index"`
	UnknownIndex         int64  `json:"unknown_index"`
	ExecutionAuthorized  bool   `json:"execution_authorized"`
}
