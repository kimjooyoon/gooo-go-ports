// Code generated from the source-bound Gooo typed plan; DO NOT EDIT.
package definitionkernel

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

type DefinitionInput struct {
	ContextID                string   `json:"context_id"`
	AliasContextID           string   `json:"alias_context_id"`
	CollectionContextID      string   `json:"collection_context_id"`
	BeforeDefinitionIDs      []string `json:"before_definition_ids"`
	AfterDefinitionIDs       []string `json:"after_definition_ids"`
	BeforeDefinitionProfiles []string `json:"before_definition_profiles"`
	AfterDefinitionProfiles  []string `json:"after_definition_profiles"`
	BeforeCapabilityIDs      []string `json:"before_capability_ids"`
	AfterCapabilityIDs       []string `json:"after_capability_ids"`
	BeforePolicyRevision     string   `json:"before_policy_revision"`
	AfterPolicyRevision      string   `json:"after_policy_revision"`
}

type Decision struct {
	ContextID                  string `json:"context_id"`
	OriginContextID            string `json:"origin_context_id"`
	CollectionContextID        string `json:"collection_context_id"`
	OriginState                string `json:"origin_state"`
	OriginReason               string `json:"origin_reason"`
	CollectionState            string `json:"collection_state"`
	CollectionReason           string `json:"collection_reason"`
	BeforeIndex                int64  `json:"before_index"`
	AfterIndex                 int64  `json:"after_index"`
	CollectionFirstProblemCode string `json:"collection_first_problem_code"`
	DefinitionRelation         string `json:"definition_relation"`
	DefinitionProfileRelation  string `json:"definition_profile_relation"`
	CapabilityRelation         string `json:"capability_relation"`
	AuthorizationRelation      string `json:"authorization_relation"`
	PolicyRelation             string `json:"policy_relation"`
	BeforeDefinitionID         string `json:"before_definition_id"`
	AfterDefinitionID          string `json:"after_definition_id"`
	BeforeDefinitionProfile    string `json:"before_definition_profile"`
	AfterDefinitionProfile     string `json:"after_definition_profile"`
	BeforeCapabilityID         string `json:"before_capability_id"`
	AfterCapabilityID          string `json:"after_capability_id"`
	BeforePolicyRevision       string `json:"before_policy_revision"`
	AfterPolicyRevision        string `json:"after_policy_revision"`
	State                      string `json:"state"`
	Reason                     string `json:"reason"`
	NextAction                 string `json:"next_action"`
	DeclaredBindingsMatch      bool   `json:"declared_bindings_match"`
	BehaviorEquivalenceProven  bool   `json:"behavior_equivalence_proven"`
	ExecutionAuthorized        bool   `json:"execution_authorized"`
	LiveEffectPerformed        bool   `json:"live_effect_performed"`
	ContainsSyntheticFixture   bool   `json:"contains_synthetic_fixture"`
	RemoteAtomicity            string `json:"remote_atomicity"`
}
