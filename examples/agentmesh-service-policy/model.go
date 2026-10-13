// Package servicepolicy transports bounded canonical ASCII facts to generated
// Gooo service-pattern, request-impact and single-grant refinement bodies. It authorizes no mesh call.
package servicepolicy

type MatcherFacts struct {
	Pattern string `json:"pattern"`
	ReqType string `json:"req_type"`
	ReqName string `json:"req_name"`
}

type ImpactFacts struct {
	Before  string `json:"before"`
	After   string `json:"after"`
	ReqType string `json:"req_type"`
	ReqName string `json:"req_name"`
}

type ImpactDecision struct {
	BeforeMatch bool   `json:"before_match"`
	AfterMatch  bool   `json:"after_match"`
	Change      string `json:"change"`
}

type Bindings struct {
	CompilerSourceSHA string            `json:"compiler_source_sha"`
	GenerationSHA256  string            `json:"generation_sha256"`
	GenerationOrigin  string            `json:"generation_origin"`
	Assets            map[string]string `json:"assets"`
}

type MatcherProof struct {
	Facts    MatcherFacts `json:"facts"`
	Matched  bool         `json:"matched"`
	Bindings Bindings     `json:"bindings"`
}

type ImpactProof struct {
	Facts    ImpactFacts    `json:"facts"`
	Decision ImpactDecision `json:"decision"`
	Bindings Bindings       `json:"bindings"`
}

// RefinementFacts compares one current pattern and one candidate pattern.
type RefinementFacts struct {
	Before string `json:"before"`
	After  string `json:"after"`
}

// RefinementDecision carries a recognized subset rule or a checked widening
// request. It does not attest current policy authority or availability.
type RefinementDecision struct {
	Status          string `json:"status"`
	RuleID          string `json:"rule_id"`
	SupportedSubset bool   `json:"supported_subset"`
	HasWitness      bool   `json:"has_witness"`
	WitnessType     string `json:"witness_type"`
	WitnessName     string `json:"witness_name"`
	BeforeMatch     bool   `json:"before_match"`
	AfterMatch      bool   `json:"after_match"`
}

type RefinementProof struct {
	Facts    RefinementFacts    `json:"facts"`
	Decision RefinementDecision `json:"decision"`
	Bindings Bindings           `json:"bindings"`
}

// Checked decisions distinguish malformed input from a computed negative match.
// Interpret semantic fields only when DomainStatus is exactly VALID_INPUT.
type CheckedMatcherDecision struct {
	DomainStatus string `json:"domain_status"`
	InvalidField string `json:"invalid_field"`
	Matched      bool   `json:"matched"`
}

type CheckedImpactDecision struct {
	DomainStatus string `json:"domain_status"`
	InvalidField string `json:"invalid_field"`
	BeforeMatch  bool   `json:"before_match"`
	AfterMatch   bool   `json:"after_match"`
	Change       string `json:"change"`
}

type CheckedRefinementDecision struct {
	DomainStatus string `json:"domain_status"`
	InvalidField string `json:"invalid_field"`
	RefinementDecision
}

type CheckedMatcherProof struct {
	Facts    MatcherFacts           `json:"facts"`
	Decision CheckedMatcherDecision `json:"decision"`
	Bindings Bindings               `json:"bindings"`
}

type CheckedImpactProof struct {
	Facts    ImpactFacts           `json:"facts"`
	Decision CheckedImpactDecision `json:"decision"`
	Bindings Bindings              `json:"bindings"`
}

type CheckedRefinementProof struct {
	Facts    RefinementFacts           `json:"facts"`
	Decision CheckedRefinementDecision `json:"decision"`
	Bindings Bindings                  `json:"bindings"`
}
