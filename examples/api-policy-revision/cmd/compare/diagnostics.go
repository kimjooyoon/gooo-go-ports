package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	revision "github.com/kimjooyoon/gooo-go-ports/examples/api-policy-revision"
	original "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy"
)

// Refusal context describes an observed boundary, without changing its cause.
type refusalError struct {
	code, stage, mode, format string
	cause                     error
	itemIndex                 *int
}

func (err *refusalError) Error() string { return err.cause.Error() }
func (err *refusalError) Unwrap() error { return err.cause }

func policyCause(code, stage string, cause error) (string, string) {
	switch {
	case errors.Is(cause, revision.ErrProfileMismatch):
		return "PROFILE_MISMATCH", "check_profiles"
	case errors.Is(cause, original.ErrAPIGenerationMismatch):
		return "GENERATION_MISMATCH", "verify_original"
	case errors.Is(cause, original.ErrAPIDecisionMismatch):
		return "SAVED_DECISION_MISMATCH", "verify_original"
	case errors.Is(cause, revision.ErrComparisonMismatch):
		return "REPORT_MISMATCH", "verify_comparison"
	default:
		return code, stage
	}
}

type diagnostic struct {
	Severity  string `json:"severity"`
	Code      string `json:"code"`
	Stage     string `json:"stage"`
	Message   string `json:"message"`
	ItemIndex *int   `json:"item_index,omitempty"`
}

// A diagnostic is optional output. Its failure cannot replace the refusal or
// trigger another evaluation; the caller still exits with failure.
func writeRefusal(stderr io.Writer, cause error) error {
	var refusal *refusalError
	if !errors.As(cause, &refusal) || refusal.format != "json" {
		raw := []byte(fmt.Sprintln("API revision comparison refused:", cause))
		return errors.Join(cause, writeDiagnosticRecord(stderr, raw))
	}
	envelope := struct {
		Schema      string       `json:"schema"`
		Command     string       `json:"command"`
		Mode        string       `json:"mode"`
		Status      string       `json:"status"`
		Diagnostics []diagnostic `json:"diagnostics"`
	}{
		Schema: "gooo/api-policy-revision-diagnostic/v1", Command: "compare",
		Mode: refusal.mode, Status: "REFUSED",
		Diagnostics: []diagnostic{{Severity: "error", Code: refusal.code, Stage: refusal.stage, Message: cause.Error(), ItemIndex: refusal.itemIndex}},
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		return errors.Join(cause, err)
	}
	raw = append(raw, '\n')
	return errors.Join(cause, writeDiagnosticRecord(stderr, raw))
}

func writeDiagnosticRecord(stderr io.Writer, raw []byte) error {
	written, err := stderr.Write(raw)
	if err == nil && written != len(raw) {
		err = io.ErrShortWrite
	}
	return err
}
