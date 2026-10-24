// Gooo owns alias/origin decisions; Go owns transport and facts snapshots.
package aliasorigin

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"github.com/kimjooyoon/gooo-go-ports/examples/alias-origin-change/internal/kernel"
	"unicode/utf8"
)

const MaxEntries = 256
const MaxTextBytes = 1024

type Entry struct {
	Server       string `json:"server"`
	OriginalName string `json:"original_name"`
	ExposedName  string `json:"exposed_name"`
}
type Input struct {
	TargetAlias string  `json:"target_alias"`
	Before      []Entry `json:"before"`
	After       []Entry `json:"after"`
}
type Decision = kernel.Decision
type Report struct {
	Facts       Input           `json:"facts"`
	FactsSHA256 string          `json:"facts_sha256"`
	InputSHA256 string          `json:"input_sha256,omitempty"`
	Decision    Decision        `json:"decision"`
	Bindings    json.RawMessage `json:"bindings"`
}
type Problem struct {
	State string `json:"state"`
	Stage string `json:"stage"`
	Code  string `json:"code"`
}

func (p *Problem) Error() string { return p.State + ":" + p.Stage + ":" + p.Code }

//go:embed generation.json
var generation []byte

func digest(b []byte) string { s := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(s[:]) }
func Compare(in Input) (report Report, returned error) {
	defer func() {
		if recover() != nil {
			report = Report{}
			returned = &Problem{"UNKNOWN", "evaluate", "KERNEL_FAILURE"}
		}
	}()
	if len(in.Before) > MaxEntries || len(in.After) > MaxEntries {
		return Report{}, &Problem{"INVALID_TRANSPORT", "decode_facts", "COLLECTION_LIMIT"}
	}
	valid := func(s string) bool { return utf8.ValidString(s) && len(s) <= MaxTextBytes }
	if !valid(in.TargetAlias) {
		return Report{}, &Problem{"INVALID_TRANSPORT", "decode_facts", "TEXT_LIMIT_OR_ENCODING"}
	}
	snapshot := func(rows []Entry) ([]Entry, error) {
		out := make([]Entry, len(rows))
		copy(out, rows)
		for _, r := range out {
			if !valid(r.Server) || !valid(r.OriginalName) || !valid(r.ExposedName) {
				return nil, &Problem{"INVALID_TRANSPORT", "decode_facts", "TEXT_LIMIT_OR_ENCODING"}
			}
		}
		return out, nil
	}
	before, err := snapshot(in.Before)
	if err != nil {
		return Report{}, err
	}
	after, err := snapshot(in.After)
	if err != nil {
		return Report{}, err
	}
	facts := Input{in.TargetAlias, before, after}
	raw, err := json.Marshal(facts)
	if err != nil {
		return Report{}, err
	}
	hash := digest(raw)
	columns := kernel.Input{ContextID: "gooo/alias-origin/v1/" + hash, TargetAlias: facts.TargetAlias, BeforeServers: []string{}, BeforeNames: []string{}, BeforeAliases: []string{}, AfterServers: []string{}, AfterNames: []string{}, AfterAliases: []string{}}
	for _, r := range before {
		columns.BeforeServers = append(columns.BeforeServers, r.Server)
		columns.BeforeNames = append(columns.BeforeNames, r.OriginalName)
		columns.BeforeAliases = append(columns.BeforeAliases, r.ExposedName)
	}
	for _, r := range after {
		columns.AfterServers = append(columns.AfterServers, r.Server)
		columns.AfterNames = append(columns.AfterNames, r.OriginalName)
		columns.AfterAliases = append(columns.AfterAliases, r.ExposedName)
	}
	choice := kernel.Evaluate(columns)
	if choice.ContextID != columns.ContextID || choice.ExecutionAuthorized {
		return Report{}, &Problem{"UNKNOWN", "bind_result", "KERNEL_CONTEXT_OR_AUTHORITY_MISMATCH"}
	}
	return Report{Facts: facts, FactsSHA256: hash, Decision: choice, Bindings: append(json.RawMessage(nil), generation...)}, nil
}
func CompareJSON(raw []byte) (Report, error) {
	top, err := object(raw, "target_alias", "before", "after")
	if err != nil {
		return Report{}, &Problem{"INVALID_TRANSPORT", "decode_json", "INVALID_JSON_FACTS"}
	}
	var input Input
	input.TargetAlias, err = text(top["target_alias"])
	if err != nil {
		return Report{}, &Problem{"INVALID_TRANSPORT", "decode_json", "INVALID_JSON_FACTS"}
	}
	for _, side := range []string{"before", "after"} {
		rows, err := array(top[side])
		if err != nil {
			return Report{}, &Problem{"INVALID_TRANSPORT", "decode_json", "INVALID_JSON_FACTS"}
		}
		entries := make([]Entry, len(rows))
		for i, row := range rows {
			fields, err := object(row, "server", "original_name", "exposed_name")
			if err != nil {
				return Report{}, &Problem{"INVALID_TRANSPORT", "decode_json", "INVALID_JSON_FACTS"}
			}
			for key, dest := range map[string]*string{"server": &entries[i].Server, "original_name": &entries[i].OriginalName, "exposed_name": &entries[i].ExposedName} {
				*dest, err = text(fields[key])
				if err != nil {
					return Report{}, &Problem{"INVALID_TRANSPORT", "decode_json", "INVALID_JSON_FACTS"}
				}
			}
		}
		if side == "before" {
			input.Before = entries
		} else {
			input.After = entries
		}
	}
	report, err := Compare(input)
	if err != nil {
		return Report{}, err
	}
	report.InputSHA256 = digest(raw)
	return report, nil
}
