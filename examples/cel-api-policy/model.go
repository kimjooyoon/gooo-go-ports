// Package nativepolicy transports bounded raw facts to generated Gooo bodies.
// It neither authenticates facts nor executes an external API operation.
package nativepolicy

type Facts struct {
	GroupPresent     bool   `json:"group_present"`
	GroupValue       string `json:"group_value"`
	PrincipalPresent bool   `json:"principal_present"`
	PrincipalValue   string `json:"principal_value"`
}

type APIFacts struct {
	Facts
	Authenticated   bool   `json:"authenticated"`
	CallerTenant    string `json:"caller_tenant"`
	ResourceTenant  string `json:"resource_tenant"`
	RequestedAction string `json:"requested_action"`
	AllowedAction   string `json:"allowed_action"`
}

type Decision struct {
	Allowed            bool   `json:"allowed"`
	EvaluationError    bool   `json:"evaluation_error"`
	Result             string `json:"result"`
	GroupMatched       bool   `json:"group_matched"`
	PrincipalMatched   bool   `json:"principal_matched"`
	GroupMissing       bool   `json:"group_missing"`
	PrincipalDefaulted bool   `json:"principal_defaulted"`
	EffectivePrincipal string `json:"effective_principal"`
}

type APIDecision struct {
	Allowed            bool   `json:"allowed"`
	Reason             string `json:"reason"`
	Authenticated      bool   `json:"authenticated"`
	TenantPresent      bool   `json:"tenant_present"`
	TenantMatched      bool   `json:"tenant_matched"`
	ActionPresent      bool   `json:"action_present"`
	ActionMatched      bool   `json:"action_matched"`
	AccessAllowed      bool   `json:"access_allowed"`
	AccessError        bool   `json:"access_error"`
	AccessResult       string `json:"access_result"`
	GroupMatched       bool   `json:"group_matched"`
	PrincipalMatched   bool   `json:"principal_matched"`
	GroupMissing       bool   `json:"group_missing"`
	PrincipalDefaulted bool   `json:"principal_defaulted"`
	EffectivePrincipal string `json:"effective_principal"`
}

type Bindings struct {
	CompilerSourceSHA string            `json:"compiler_source_sha"`
	Assets            map[string]string `json:"assets"`
	GenerationSHA256  string            `json:"generation_sha256"`
}

type Proof struct {
	Facts    Facts    `json:"facts"`
	Decision Decision `json:"decision"`
	Bindings Bindings `json:"bindings"`
}

type APIProof struct {
	Facts    APIFacts    `json:"facts"`
	Decision APIDecision `json:"decision"`
	Bindings Bindings    `json:"bindings"`
}
