// Code generated from the Gooo typed plan; DO NOT EDIT.
package kernel

type Input struct {
	ContextID     string   `json:"context_id"`
	TargetAlias   string   `json:"target_alias"`
	BeforeServers []string `json:"before_servers"`
	BeforeNames   []string `json:"before_names"`
	BeforeAliases []string `json:"before_aliases"`
	AfterServers  []string `json:"after_servers"`
	AfterNames    []string `json:"after_names"`
	AfterAliases  []string `json:"after_aliases"`
}

type Decision struct {
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
