// Package revisionpolicy compares sealed Gooo policies over the same saved facts.
// It establishes unsigned consistency, never current authorization or an effect.
package revisionpolicy

import original "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy"

const (
	OriginalProfile  = "cel-api-original-v1"
	CandidateProfile = "cel-api-readonly-v1"
	ComparisonSchema = "gooo/api-policy-revision-comparison/v1"
	ComparisonStatus = "COMPARED"
	ComparisonScope  = "same_saved_facts_two_sealed_profiles"
	MaxInputBytes    = original.MaxBatchBytes
	// Reports retain two proofs and JSON-escaped copies of their raw facts.
	MaxReportBytes = 16 << 20
)

type CompareRequest struct {
	BeforeProfile    string            `json:"before_profile"`
	CandidateProfile string            `json:"candidate_profile"`
	Before           original.APIProof `json:"before"`
}

// Each flag describes a comparison made by the generated Gooo classifier.
type DifferenceFlags struct {
	Allowed            bool `json:"allowed"`
	Reason             bool `json:"reason"`
	Authenticated      bool `json:"authenticated"`
	TenantPresent      bool `json:"tenant_present"`
	TenantMatched      bool `json:"tenant_matched"`
	ActionPresent      bool `json:"action_present"`
	ActionMatched      bool `json:"action_matched"`
	AccessAllowed      bool `json:"access_allowed"`
	AccessError        bool `json:"access_error"`
	AccessResult       bool `json:"access_result"`
	GroupMatched       bool `json:"group_matched"`
	PrincipalMatched   bool `json:"principal_matched"`
	GroupMissing       bool `json:"group_missing"`
	PrincipalDefaulted bool `json:"principal_defaulted"`
	EffectivePrincipal bool `json:"effective_principal"`
}

type Summary struct {
	Classification string `json:"classification"`
	ChangedCount   int    `json:"changed_count"`
}

type ComparisonReport struct {
	Schema             string            `json:"schema"`
	Status             string            `json:"status"`
	Scope              string            `json:"scope"`
	BeforeProfile      string            `json:"before_profile"`
	CandidateProfile   string            `json:"candidate_profile"`
	Before             original.APIProof `json:"before"`
	After              original.APIProof `json:"after"`
	Differences        DifferenceFlags   `json:"differences"`
	Summary            Summary           `json:"summary"`
	ClassifierBindings original.Bindings `json:"classifier_bindings"`
}
