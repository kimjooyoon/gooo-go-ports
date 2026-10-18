package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"reflect"
	"runtime"
	"strings"

	revision "github.com/kimjooyoon/gooo-go-ports/examples/api-policy-revision"
	original "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy"
)

const (
	commandName      = "compare"
	diagnosticSchema = "gooo/api-policy-revision-diagnostic/v1"
	modeBaseline     = "baseline"
	modeCompare      = "compare"
	modeReplay       = "replay"
	maxBatchItems    = 128
)

func main() {
	os.Exit(execute(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func execute(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if err := run(args, stdin, stdout); err != nil {
		_ = writeRefusal(stderr, err)
		return 1
	}
	return 0
}

// run emits a single complete JSON record after the requested mode's checks.
// Describe checks embedded bindings without evaluating a policy; no mode has a
// dispatcher or compiler invocation.
func run(args []string, stdin io.Reader, stdout io.Writer) error {
	flags := flag.NewFlagSet(commandName, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	mode := flags.String("mode", modeCompare, "baseline, compare, or replay")
	inputPath := flags.String("input", "", "read one JSON object from this path; otherwise stdin")
	batch := flags.Bool("batch", false, "read an ordered JSON array of 1..128 existing mode inputs")
	describe := flags.Bool("describe", false, "describe embedded profiles and command limits as JSON without reading input or evaluating a policy")
	diagnostics := flags.String("diagnostics", "text", "stderr format: text or json")
	refuse := func(code, stage string, cause error) error {
		return &refusalError{code: code, stage: stage, mode: *mode, format: *diagnostics, cause: cause}
	}
	policyRefusal := func(code, stage string, cause error) error {
		code, stage = policyCause(code, stage, cause)
		return refuse(code, stage, cause)
	}
	if err := flags.Parse(args); err != nil {
		return refuse("ARGUMENT_INVALID", "parse_arguments", err)
	}
	if *diagnostics != "text" && *diagnostics != "json" {
		return refuse("ARGUMENT_INVALID", "parse_arguments", errors.New("diagnostics must be text or json"))
	}
	if flags.NArg() != 0 {
		return refuse("ARGUMENT_INVALID", "parse_arguments", errors.New("unexpected positional argument"))
	}
	descriptionSet, evaluationSet := false, false
	flags.Visit(func(option *flag.Flag) {
		descriptionSet = descriptionSet || option.Name == "describe"
		evaluationSet = evaluationSet || option.Name == "mode" || option.Name == "input" || option.Name == "batch"
	})
	if descriptionSet && evaluationSet {
		return refuse("ARGUMENT_INVALID", "parse_arguments", errors.New("describe cannot be combined with mode, input, or batch"))
	}
	emit := func(output []byte) error {
		written, err := stdout.Write(output)
		if err != nil {
			return refuse("OUTPUT_IO", "write_stdout", err)
		}
		if written != len(output) {
			return refuse("OUTPUT_IO", "write_stdout", io.ErrShortWrite)
		}
		return nil
	}
	if *describe {
		description, err := describeCommand()
		if err != nil {
			return refuse("DESCRIPTION_FAILED", "describe_embedded_bindings", err)
		}
		output, err := json.Marshal(description)
		if err != nil {
			return refuse("OUTPUT_ENCODING_FAILED", "marshal_output", err)
		}
		return emit(append(output, '\n'))
	}
	if *mode != modeBaseline && *mode != modeCompare && *mode != modeReplay {
		return refuse("ARGUMENT_INVALID", "parse_arguments", errors.New("mode must be baseline, compare, or replay"))
	}
	reader := stdin
	if *inputPath != "" {
		file, err := os.Open(*inputPath)
		if err != nil {
			return refuse("INPUT_IO", "open_input", err)
		}
		defer file.Close()
		reader = file
	}
	maxBytes := revision.MaxInputBytes
	if *mode == modeReplay {
		maxBytes = revision.MaxReportBytes
	}
	raw, err := io.ReadAll(io.LimitReader(reader, int64(maxBytes)+1))
	if err != nil {
		return refuse("INPUT_IO", "read_input", err)
	}
	if len(raw) > maxBytes {
		return refuse("INPUT_LIMIT_EXCEEDED", "read_input", fmt.Errorf("input exceeds %d MiB", maxBytes>>20))
	}
	evaluate := func(raw []byte) (any, error) {
		switch *mode {
		case modeBaseline:
			facts, err := original.DecodeAPI(raw)
			if err != nil {
				return nil, refuse("INPUT_INVALID", "decode_facts", err)
			}
			result, err := revision.Baseline(facts)
			if err != nil {
				return nil, policyRefusal("EVALUATION_FAILED", "evaluate_baseline", err)
			}
			return result, nil
		case modeCompare:
			request, err := revision.DecodeCompareRequest(raw)
			if err != nil {
				return nil, policyRefusal("INPUT_INVALID", "decode_request", err)
			}
			result, err := revision.Compare(request)
			if err != nil {
				return nil, policyRefusal("COMPARISON_FAILED", "compare", err)
			}
			return result, nil
		default: // The mode was validated before reading input.
			report, err := revision.DecodeComparisonReport(raw)
			if err != nil {
				return nil, policyRefusal("INPUT_INVALID", "decode_report", err)
			}
			result, err := revision.VerifyComparison(report)
			if err != nil {
				return nil, policyRefusal("REPLAY_FAILED", "verify_comparison", err)
			}
			return result, nil
		}
	}
	var output []byte
	if *batch {
		var rows []json.RawMessage
		if err := json.Unmarshal(raw, &rows); err != nil || len(rows) == 0 || len(rows) > maxBatchItems {
			return refuse("INPUT_INVALID", "decode_batch", errors.New("batch requires one JSON array of 1..128 mode inputs"))
		}
		limit := revision.MaxReportBytes
		if *mode == modeBaseline {
			limit = revision.MaxInputBytes
		}
		output = []byte{'['}
		for i, row := range rows {
			result, err := evaluate(row)
			if err != nil {
				var refusal *refusalError
				if errors.As(err, &refusal) {
					refusal.itemIndex = &i
				}
				return err
			}
			encoded, err := json.Marshal(result)
			if err != nil {
				return &refusalError{code: "OUTPUT_ENCODING_FAILED", stage: "marshal_output", mode: *mode, format: *diagnostics, cause: err, itemIndex: &i}
			}
			comma := 0
			if i != 0 {
				comma = 1
			}
			// Include the closing bracket and LF before appending any item.
			if len(output)+comma+len(encoded)+2 > limit {
				return &refusalError{code: "OUTPUT_LIMIT_EXCEEDED", stage: "check_output_limit", mode: *mode, format: *diagnostics, cause: fmt.Errorf("batch output exceeds %d MiB", limit>>20), itemIndex: &i}
			}
			if comma != 0 {
				output = append(output, ',')
			}
			output = append(output, encoded...)
		}
		output = append(output, ']', '\n')
	} else {
		result, err := evaluate(raw)
		if err != nil {
			return err
		}
		output, err = json.Marshal(result)
		if err != nil {
			return refuse("OUTPUT_ENCODING_FAILED", "marshal_output", err)
		}
		output = append(output, '\n')
	}
	return emit(output)
}

type describedField struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	MaxUTF8Bytes int    `json:"max_utf8_bytes,omitempty"`
}

type describedBatch struct {
	Minimum int `json:"minimum"`
	Maximum int `json:"maximum"`
}

func describeFields(t reflect.Type) ([]describedField, error) {
	var fields []describedField
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			nested, err := describeFields(field.Type)
			if err != nil {
				return nil, err
			}
			fields = append(fields, nested...)
			continue
		}
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			return nil, errors.New("unsupported fact or decision field")
		}
		entry := describedField{Name: name}
		switch field.Type.Kind() {
		case reflect.Bool:
			entry.Type = "boolean"
		case reflect.String:
			entry.Type = "string"
			// This bound belongs to raw fact strings, not returned decision strings.
			if t == reflect.TypeOf(original.Facts{}) || t == reflect.TypeOf(original.APIFacts{}) {
				entry.MaxUTF8Bytes = original.MaxTextBytes
			}
		default:
			return nil, errors.New("unsupported fact or decision type")
		}
		fields = append(fields, entry)
	}
	return fields, nil
}

func describeCommand() (any, error) {
	bindings, err := revision.DescribeBindings()
	if err != nil {
		return nil, err
	}
	facts, err := describeFields(reflect.TypeOf(original.APIFacts{}))
	if err != nil {
		return nil, err
	}
	decisions, err := describeFields(reflect.TypeOf(original.APIDecision{}))
	if err != nil {
		return nil, err
	}
	type modeDescription struct {
		Mode             string `json:"mode"`
		InputRecordType  string `json:"input_record_type"`
		OutputRecordType string `json:"output_record_type"`
		InputBytes       int    `json:"input_bytes"`
		BatchOutputBytes int    `json:"batch_output_bytes_including_LF"`
	}
	return struct {
		Schema                  string                       `json:"schema"`
		Status                  string                       `json:"status"`
		Scope                   string                       `json:"scope"`
		Command                 string                       `json:"command"`
		ExecutableGoVersion     string                       `json:"executable_go_version"`
		Modes                   []modeDescription            `json:"modes"`
		Batch                   describedBatch               `json:"batch_items"`
		Facts                   []describedField             `json:"required_raw_fact_fields"`
		Decisions               []describedField             `json:"decision_fields"`
		Profiles                revision.EmbeddedDescription `json:"embedded_profiles"`
		ComparisonSchema        string                       `json:"comparison_schema"`
		DiagnosticSchema        string                       `json:"diagnostic_schema"`
		TextScope               string                       `json:"text_scope"`
		InputScope              string                       `json:"input_scope"`
		OutputScope             string                       `json:"output_scope"`
		DynamicPolicyLoading    bool                         `json:"dynamic_policy_loading"`
		CompilerRuntimeRequired bool                         `json:"compiler_runtime_required"`
		BundleArchiveChecked    bool                         `json:"bundle_archive_checked"`
		InstalledEngineChecked  bool                         `json:"installed_engine_checked"`
		PolicyExecuted          bool                         `json:"policy_executed"`
		ProducerAuthenticated   bool                         `json:"producer_authenticated"`
		NativeAcceptance        bool                         `json:"native_acceptance"`
	}{
		Schema: "gooo/api-policy-revision-description/v1", Status: "DECLARED_CAPABILITIES",
		Scope: "embedded_bindings_checked_introspection_only", Command: commandName, ExecutableGoVersion: runtime.Version(),
		Modes: []modeDescription{
			{modeBaseline, reflect.TypeOf(original.APIFacts{}).Name(), reflect.TypeOf(revision.CompareRequest{}).Name(), revision.MaxInputBytes, revision.MaxInputBytes},
			{modeCompare, reflect.TypeOf(revision.CompareRequest{}).Name(), reflect.TypeOf(revision.ComparisonReport{}).Name(), revision.MaxInputBytes, revision.MaxReportBytes},
			{modeReplay, reflect.TypeOf(revision.ComparisonReport{}).Name(), reflect.TypeOf(revision.ComparisonReport{}).Name(), revision.MaxReportBytes, revision.MaxReportBytes},
		},
		Batch: describedBatch{1, maxBatchItems}, Facts: facts, Decisions: decisions, Profiles: bindings,
		ComparisonSchema: revision.ComparisonSchema, DiagnosticSchema: diagnosticSchema,
		TextScope:   "Raw facts are Unicode-scalar UTF-8; no trimming, normalization, repair, or authentication.",
		InputScope:  "Exactly typed fields; unknown, missing, duplicate, null, invalid Unicode and trailing JSON refused. Batch preserves ordered duplicates.",
		OutputScope: "One JSON record with LF; arrays buffered before writing. A failed output write can be partial. A policy denial is a successful result, not a process failure.",
	}, nil
}
