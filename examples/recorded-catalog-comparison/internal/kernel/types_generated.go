// Code generated from the Gooo typed plan; DO NOT EDIT.
package kernel

type OriginInput struct {
	ContextID     string   `json:"context_id"`
	TargetAlias   string   `json:"target_alias"`
	BeforeServers []string `json:"before_servers"`
	BeforeNames   []string `json:"before_names"`
	BeforeAliases []string `json:"before_aliases"`
	AfterServers  []string `json:"after_servers"`
	AfterNames    []string `json:"after_names"`
	AfterAliases  []string `json:"after_aliases"`
}

type CollectionInput struct {
	ContextID                           string   `json:"context_id"`
	AliasContextID                      string   `json:"alias_context_id"`
	BeforeCatalogAuthorizationContextID string   `json:"before_catalog_authorization_context_id"`
	AfterCatalogAuthorizationContextID  string   `json:"after_catalog_authorization_context_id"`
	DeclaredSides                       []string `json:"declared_sides"`
	DeclaredHostIDs                     []string `json:"declared_host_ids"`
	DeclaredConnectionIDs               []string `json:"declared_connection_ids"`
	CaptureSides                        []string `json:"capture_sides"`
	CaptureHostIDs                      []string `json:"capture_host_ids"`
	CaptureConnectionIDs                []string `json:"capture_connection_ids"`
	CaptureRecordingBases               []string `json:"capture_recording_bases"`
	CaptureEvidenceKinds                []string `json:"capture_evidence_kinds"`
	CaptureWatchCoverages               []string `json:"capture_watch_coverages"`
	CaptureRemoteAtomicities            []string `json:"capture_remote_atomicities"`
	CaptureBeginBindings                []string `json:"capture_begin_bindings"`
	CaptureEndBindings                  []string `json:"capture_end_bindings"`
	CaptureBeginAuthContexts            []string `json:"capture_begin_auth_contexts"`
	CaptureEndAuthContexts              []string `json:"capture_end_auth_contexts"`
	CaptureBeginConfigRevisions         []string `json:"capture_begin_config_revisions"`
	CaptureEndConfigRevisions           []string `json:"capture_end_config_revisions"`
	CaptureBeginProtocols               []string `json:"capture_begin_protocols"`
	CaptureEndProtocols                 []string `json:"capture_end_protocols"`
	CaptureRelevantChangeObserved       []bool   `json:"capture_relevant_change_observed"`
	PageCaptureIndexes                  []int64  `json:"page_capture_indexes"`
	PageRequestCursorKinds              []string `json:"page_request_cursor_kinds"`
	PageRequestCursorValues             []string `json:"page_request_cursor_values"`
	PageNextCursorKinds                 []string `json:"page_next_cursor_kinds"`
	PageNextCursorValues                []string `json:"page_next_cursor_values"`
	PageBindings                        []string `json:"page_bindings"`
	PageAuthContexts                    []string `json:"page_auth_contexts"`
	PageConfigRevisions                 []string `json:"page_config_revisions"`
	PageProtocols                       []string `json:"page_protocols"`
}

type AliasDecision struct {
	ContextID           string `json:"context_id"`
	State               string `json:"state"`
	Reason              string `json:"reason"`
	BeforeIndex         int64  `json:"before_index"`
	AfterIndex          int64  `json:"after_index"`
	ConflictCatalog     string `json:"conflict_catalog"`
	ConflictFirstIndex  int64  `json:"conflict_first_index"`
	ConflictSecondIndex int64  `json:"conflict_second_index"`
	ExecutionAuthorized bool   `json:"execution_authorized"`
}

type CollectionDecision struct {
	ContextID                  string   `json:"context_id"`
	AliasContextID             string   `json:"alias_context_id"`
	State                      string   `json:"state"`
	Reason                     string   `json:"reason"`
	NextAction                 string   `json:"next_action"`
	BeforePaginationState      string   `json:"before_pagination_state"`
	AfterPaginationState       string   `json:"after_pagination_state"`
	BeforeConfigurationState   string   `json:"before_configuration_state"`
	AfterConfigurationState    string   `json:"after_configuration_state"`
	AuthorizationRelation      string   `json:"authorization_relation"`
	RecordingBasisState        string   `json:"recording_basis_state"`
	RemoteAtomicity            string   `json:"remote_atomicity"`
	FirstProblemSide           string   `json:"first_problem_side"`
	FirstProblemCode           string   `json:"first_problem_code"`
	FirstProblemCaptureIndex   int64    `json:"first_problem_capture_index"`
	FirstProblemPageIndex      int64    `json:"first_problem_page_index"`
	FirstProblemDeclaredIndex  int64    `json:"first_problem_declared_index"`
	ContainsSyntheticFixture   bool     `json:"contains_synthetic_fixture"`
	LiveRemovalProven          bool     `json:"live_removal_proven"`
	ExecutionAuthorized        bool     `json:"execution_authorized"`
	CapturePaginationStates    []string `json:"capture_pagination_states"`
	CaptureConfigurationStates []string `json:"capture_configuration_states"`
	CaptureRecordingBases      []string `json:"capture_recording_bases"`
	CaptureEvidenceKinds       []string `json:"capture_evidence_kinds"`
}

type Decision struct {
	Alias      AliasDecision      `json:"alias"`
	Collection CollectionDecision `json:"collection"`
}
