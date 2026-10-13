// Package servicepolicy transports bounded canonical ASCII facts to generated
// Gooo service-pattern and request-impact bodies. It authorizes no mesh call.
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
