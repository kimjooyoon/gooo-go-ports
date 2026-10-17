package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	revision "github.com/kimjooyoon/gooo-go-ports/examples/api-policy-revision"
	original "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy"
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

// run emits a single complete JSON record only after all input, generation and
// native decision checks succeed. It has no dispatcher or compiler invocation.
func run(args []string, stdin io.Reader, stdout io.Writer) error {
	flags := flag.NewFlagSet("compare", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	mode := flags.String("mode", "compare", "baseline, compare, or replay")
	inputPath := flags.String("input", "", "read one JSON object from this path; otherwise stdin")
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
	if *mode != "baseline" && *mode != "compare" && *mode != "replay" {
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
	if *mode == "replay" {
		maxBytes = revision.MaxReportBytes
	}
	raw, err := io.ReadAll(io.LimitReader(reader, int64(maxBytes)+1))
	if err != nil {
		return refuse("INPUT_IO", "read_input", err)
	}
	if len(raw) > maxBytes {
		return refuse("INPUT_LIMIT_EXCEEDED", "read_input", fmt.Errorf("input exceeds %d MiB", maxBytes>>20))
	}
	var result any
	switch *mode {
	case "baseline":
		facts, err := original.DecodeAPI(raw)
		if err != nil {
			return refuse("INPUT_INVALID", "decode_facts", err)
		}
		result, err = revision.Baseline(facts)
		if err != nil {
			return policyRefusal("EVALUATION_FAILED", "evaluate_baseline", err)
		}
	case "compare":
		request, err := revision.DecodeCompareRequest(raw)
		if err != nil {
			return policyRefusal("INPUT_INVALID", "decode_request", err)
		}
		result, err = revision.Compare(request)
		if err != nil {
			return policyRefusal("COMPARISON_FAILED", "compare", err)
		}
	case "replay":
		report, err := revision.DecodeComparisonReport(raw)
		if err != nil {
			return policyRefusal("INPUT_INVALID", "decode_report", err)
		}
		result, err = revision.VerifyComparison(report)
		if err != nil {
			return policyRefusal("REPLAY_FAILED", "verify_comparison", err)
		}
	}
	output, err := json.Marshal(result)
	if err != nil {
		return refuse("OUTPUT_ENCODING_FAILED", "marshal_output", err)
	}
	output = append(output, '\n')
	written, err := stdout.Write(output)
	if err != nil {
		return refuse("OUTPUT_IO", "write_stdout", err)
	}
	if written != len(output) {
		return refuse("OUTPUT_IO", "write_stdout", io.ErrShortWrite)
	}
	return nil
}
