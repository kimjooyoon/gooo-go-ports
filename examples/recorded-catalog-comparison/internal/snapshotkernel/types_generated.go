// Code generated from the source-bound Gooo typed plan; DO NOT EDIT.
package snapshotkernel

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

type Selection struct {
	ContextID  string `json:"context_id"`
	TargetName string `json:"target_name"`
}

type Decision struct {
	ContextID                 string `json:"context_id"`
	TargetName                string `json:"target_name"`
	State                     string `json:"state"`
	Reason                    string `json:"reason"`
	NextAction                string `json:"next_action"`
	ComparisonScope           string `json:"comparison_scope"`
	BeforePageCoverage        string `json:"before_page_coverage"`
	AfterPageCoverage         string `json:"after_page_coverage"`
	BeforeCoverageReason      string `json:"before_coverage_reason"`
	AfterCoverageReason       string `json:"after_coverage_reason"`
	BeforeProblemPageIndex    int64  `json:"before_problem_page_index"`
	AfterProblemPageIndex     int64  `json:"after_problem_page_index"`
	BeforeProblemRowIndex     int64  `json:"before_problem_row_index"`
	AfterProblemRowIndex      int64  `json:"after_problem_row_index"`
	BeforeIndex               int64  `json:"before_index"`
	AfterIndex                int64  `json:"after_index"`
	BeforeDefinitionID        string `json:"before_definition_id"`
	AfterDefinitionID         string `json:"after_definition_id"`
	BeforeDefinitionProfile   string `json:"before_definition_profile"`
	AfterDefinitionProfile    string `json:"after_definition_profile"`
	BeforeAnnotationID        string `json:"before_annotation_id"`
	AfterAnnotationID         string `json:"after_annotation_id"`
	ObservedPresenceRelation  string `json:"observed_presence_relation"`
	DefinitionRelation        string `json:"definition_relation"`
	AnnotationRelation        string `json:"annotation_relation"`
	ProfileRelation           string `json:"profile_relation"`
	OriginRelation            string `json:"origin_relation"`
	AuthorizationRelation     string `json:"authorization_relation"`
	BehaviorEquivalenceProven bool   `json:"behavior_equivalence_proven"`
	ExecutionAuthorized       bool   `json:"execution_authorized"`
	LiveEffectPerformed       bool   `json:"live_effect_performed"`
	RemoteAtomicity           string `json:"remote_atomicity"`
}
