// Original transport and public types; Gooo owns every policy decision.
package servicegrantmany

type Request struct {
	Type string `json:"type"`
	Name string `json:"name"`
}
type Input struct {
	Before           string    `json:"before"`
	After            string    `json:"after"`
	RequiredRequests []Request `json:"required_requests"`
}
type RequestColumns struct {
	Before        string   `json:"before"`
	After         string   `json:"after"`
	RequiredTypes []string `json:"required_types"`
	RequiredNames []string `json:"required_names"`
}
type Decision struct {
	Status                    string `json:"status"`
	Reason                    string `json:"reason"`
	InvalidField              string `json:"invalid_field"`
	InvalidIndex              int64  `json:"invalid_index"`
	FirstCounterexample       int64  `json:"first_counterexample"`
	RequestCount              int64  `json:"request_count"`
	CheckedCount              int64  `json:"checked_count"`
	AllPreserved              bool   `json:"all_preserved"`
	SupportedSubset           bool   `json:"supported_subset"`
	SubsetRuleID              string `json:"subset_rule_id"`
	CounterexampleBeforeMatch bool   `json:"counterexample_before_match"`
	CounterexampleAfterMatch  bool   `json:"counterexample_after_match"`
}

// Columns projects ordered transport pairs; it does not validate policy or
// sort, deduplicate, filter, or refuse257 requests ahead of Gooo.
func (input Input) Columns() RequestColumns {
	out := RequestColumns{Before: input.Before, After: input.After,
		RequiredTypes: make([]string, len(input.RequiredRequests)),
		RequiredNames: make([]string, len(input.RequiredRequests))}
	for i, request := range input.RequiredRequests {
		out.RequiredTypes[i] = request.Type
		out.RequiredNames[i] = request.Name
	}
	return out
}
func Evaluate(input Input) Decision { return EvaluateColumns(input.Columns()) }

type Result struct {
	Facts    RequestColumns `json:"facts"`
	Decision Decision       `json:"decision"`
	Bindings Bindings       `json:"bindings"`
}

func AssessColumns(input RequestColumns) Result {
	// Snapshot caller-owned columns before evaluating and retaining the facts.
	facts := input
	if input.RequiredTypes != nil {
		facts.RequiredTypes = make([]string, len(input.RequiredTypes))
		copy(facts.RequiredTypes, input.RequiredTypes)
	}
	if input.RequiredNames != nil {
		facts.RequiredNames = make([]string, len(input.RequiredNames))
		copy(facts.RequiredNames, input.RequiredNames)
	}
	return Result{Facts: facts, Decision: EvaluateColumns(facts), Bindings: SourceBindings()}
}
func Assess(input Input) Result { return AssessColumns(input.Columns()) }
