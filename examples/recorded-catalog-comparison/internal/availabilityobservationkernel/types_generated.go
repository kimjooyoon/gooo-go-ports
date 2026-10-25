// Code generated from the source-bound Gooo typed plan; DO NOT EDIT.
package availabilityobservationkernel

type FieldInput struct {
	ContextID              string `json:"context_id"`
	FieldPath              string `json:"field_path"`
	FieldKind              string `json:"field_kind"`
	RecordingBasis         string `json:"recording_basis"`
	ValueState             string `json:"value_state"`
	Value                  string `json:"value"`
	ComparisonScope        string `json:"comparison_scope"`
	ExpectedValueState     string `json:"expected_value_state"`
	ExpectedValue          string `json:"expected_value"`
	ExpectedFieldPath      string `json:"expected_field_path"`
	SourceDocument         string `json:"source_document"`
	ExpectedSourceDocument string `json:"expected_source_document"`
}

type Result struct {
	ContextID                 string `json:"context_id"`
	FieldPath                 string `json:"field_path"`
	FieldKind                 string `json:"field_kind"`
	RecordingBasis            string `json:"recording_basis"`
	ValueState                string `json:"value_state"`
	Value                     string `json:"value"`
	ComparisonScope           string `json:"comparison_scope"`
	ExpectedValueState        string `json:"expected_value_state"`
	ExpectedValue             string `json:"expected_value"`
	ExpectedFieldPath         string `json:"expected_field_path"`
	SourceDocument            string `json:"source_document"`
	ExpectedSourceDocument    string `json:"expected_source_document"`
	State                     string `json:"state"`
	Reason                    string `json:"reason"`
	NextAction                string `json:"next_action"`
	RequiredForContext        bool   `json:"required_for_context"`
	KnownChange               bool   `json:"known_change"`
	ProducerAuthenticated     bool   `json:"producer_authenticated"`
	BehaviorEquivalenceProven bool   `json:"behavior_equivalence_proven"`
	ExecutionAuthorized       bool   `json:"execution_authorized"`
	LiveEffectPerformed       bool   `json:"live_effect_performed"`
}
