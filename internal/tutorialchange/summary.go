package tutorialchange

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
)

type RawRecord struct {
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type Source struct {
	Filename            string `json:"filename"`
	BeforeRawSHA256     string `json:"before_raw_file_sha256"`
	AfterRawSHA256      string `json:"after_raw_file_sha256"`
	BeforeReceiptDigest string `json:"before_receipt_json_string_digest"`
	AfterReceiptDigest  string `json:"after_receipt_json_string_digest"`
	Changed             bool   `json:"changed"`
}

type Field struct {
	Name    string `json:"name"`
	ID      string `json:"id"`
	Type    string `json:"type"`
	Before  any    `json:"before"`
	After   any    `json:"after"`
	Replay  any    `json:"replay"`
	Changed bool   `json:"changed"`
}

type Case struct {
	Index  int     `json:"case_index"`
	Inputs []any   `json:"inputs"`
	Fields []Field `json:"fields"`
}

// Summary keeps receipt metadata and separately observed compiler BuildInfo in
// distinct objects. No Git command is run, so repository_revision is unknown.
type Summary struct {
	Schema                   string               `json:"schema"`
	Status                   string               `json:"status"`
	Profile                  string               `json:"profile"`
	RepositoryRevision       *string              `json:"repository_revision"`
	RepositoryRevisionStatus string               `json:"repository_revision_status"`
	RawRecords               map[string]RawRecord `json:"raw_records"`
	ReceiptCompiler          map[string]any       `json:"receipt_compiler"`
	SeparateCompilerBuild    map[string]any       `json:"separate_compiler_build"`
	ManifestDigest           string               `json:"manifest_digest"`
	InputsDigest             string               `json:"inputs_digest"`
	ReplayRawParent          string               `json:"replay_raw_parent_sha256"`
	ReplayTypedResultParent  string               `json:"replay_typed_result_parent_sha256"`
	Sources                  []Source             `json:"sources"`
	Cases                    []Case               `json:"cases"`
	Limits                   []string             `json:"limits"`
}

func checkSourceFiles(records []map[string]any) error {
	before, _ := profileFiles.ReadFile("testdata/main-before.gooo")
	after, _ := profileFiles.ReadFile("testdata/main-after.gooo")
	if bytes.Count(before, []byte("if score >= 60")) != 1 ||
		!bytes.Equal(after, bytes.Replace(before, []byte("if score >= 60"), []byte("if score >= 80"), 1)) {
		return fmt.Errorf("embedded tutorial source edit differs")
	}
	limits, _ := profileFiles.ReadFile("testdata/limits.gooo")
	workspace, _ := profileFiles.ReadFile("testdata/workspace.json")
	inputs, _ := profileFiles.ReadFile("testdata/inputs.json")
	for i, record := range records {
		if record["manifest_digest"] != digest(workspace) || record["inputs_digest"] != digest(inputs) {
			return fmt.Errorf("fixed original manifest/input binding differs")
		}
		main := before
		if i != 0 {
			main = after
		}
		image := object(object(object(record["result"])["program"])["workspace_image"])
		pkgs := array(image["packages"])
		if len(pkgs) != 2 {
			return fmt.Errorf("two original source packages required")
		}
		for j, source := range [][]byte{limits, main} {
			encoded, err := json.Marshal(string(source))
			if err != nil {
				return err
			}
			files := array(object(pkgs[j])["sources"])
			if len(files) != 1 || object(files[0])["source_digest"] != digest(encoded) {
				return fmt.Errorf("original file JSON-string source digest differs")
			}
		}
	}
	return nil
}

func summarize(raws [][]byte, compilerRaw []byte, records []map[string]any, build map[string]any, priorResult string) Summary {
	first := records[0]
	runtime := object(object(first["result"])["runtime"])
	out := Summary{
		Schema: "gooo/tutorial-change-summary/v1", Status: "CHECKED_FINITE_OBSERVATIONS",
		Profile: "learn-gooo/threshold-60-to-80/compiler839", RepositoryRevisionStatus: "NOT_CAPTURED",
		RawRecords:            map[string]RawRecord{},
		ReceiptCompiler:       map[string]any{"source_revision": runtime["producer_source_sha"], "module": runtime["producer_module"], "go_version": runtime["go_version"], "go_tool_sha256": runtime["go_tool_sha256"]},
		SeparateCompilerBuild: build,
		ManifestDigest:        first["manifest_digest"].(string), InputsDigest: first["inputs_digest"].(string),
		ReplayRawParent: records[2]["replayed_from_sha256"].(string), ReplayTypedResultParent: priorResult,
		Limits: []string{
			"Offline consistency of supplied records only; this does not authenticate execution, signatures or the record producer.",
			"Exactly the pinned839 tutorial, four original inputs and the sole threshold60-to80 edit; not a general receipt validator or all-input correctness proof.",
			"OBSERVED input-only execution has finite_total0/finite_passed0; no Gooo execution, model call or live effect is performed by this checker.",
			"Repository revision was not supplied; a local source edit can leave Git HEAD unchanged. File-byte SHA, JSON-string source digest and compiler revision are distinct coordinates.",
			"The separate compiler build JSON is not runtime.build process accounting. Inner semantic/completeness declarations are constrained to this frozen profile, not independently proved for arbitrary programs.",
		},
	}
	for i, name := range []string{"before", "after", "replay"} {
		out.RawRecords[name] = RawRecord{len(raws[i]), digest(raws[i])}
	}
	out.ReceiptCompiler["go_tool_selection_by_record"] = map[string]any{
		"before": stringAt(records[0], "result", "runtime", "go_tool_selection"),
		"after":  stringAt(records[1], "result", "runtime", "go_tool_selection"),
		"replay": stringAt(records[2], "result", "runtime", "go_tool_selection"),
	}
	out.RawRecords["compiler_build"] = RawRecord{len(compilerRaw), digest(compilerRaw)}
	for j, file := range []string{"limits.gooo", "main-before.gooo"} {
		before, _ := profileFiles.ReadFile("testdata/" + file)
		after := before
		if j == 1 {
			after, _ = profileFiles.ReadFile("testdata/main-after.gooo")
		}
		get := func(r map[string]any) map[string]any {
			image := object(object(object(r["result"])["program"])["workspace_image"])
			return object(array(object(array(image["packages"])[j])["sources"])[0])
		}
		a, b := get(records[0]), get(records[1])
		out.Sources = append(out.Sources, Source{a["filename"].(string), digest(before), digest(after), a["source_digest"].(string), b["source_digest"].(string), !bytes.Equal(before, after)})
	}
	fieldNames := []string{"score", "remaining", "in_range", "label"}
	fieldIDs := []string{"urn:learn-gooo:score", "urn:learn-gooo:remaining", "urn:learn-gooo:in-range-field", "urn:learn-gooo:label"}
	fieldTypes := []string{"integer", "integer", "boolean", "string"}
	for i := 0; i < 4; i++ {
		delivery := func(record map[string]any) map[string]any {
			traces := array(object(object(record["result"])["runtime"])["traces"])
			return object(array(object(traces[i])["deliveries"])[0])
		}
		a, b, replay := delivery(records[0]), delivery(records[1]), delivery(records[2])
		row := Case{Index: i, Inputs: array(a["inputs"])}
		for j, name := range fieldNames {
			before, after, replayValue := object(a["actual"])[name], object(b["actual"])[name], object(replay["actual"])[name]
			row.Fields = append(row.Fields, Field{name, fieldIDs[j], fieldTypes[j], before, after, replayValue, !equalValue(before, after)})
		}
		out.Cases = append(out.Cases, row)
	}
	return out
}

func equalValue(a, b any) bool { return reflect.DeepEqual(a, b) }
