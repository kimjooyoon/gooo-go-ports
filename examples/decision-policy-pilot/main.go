package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

const defaultCompiler = "../../.gooo"

type CaseResult struct {
	ID          string  `json:"id"`
	Outcome     Outcome `json:"outcome"`
	SpyCalls    int     `json:"mock_spy_calls"`
	InputDigest string  `json:"original_envelope_sha256"`
	Error       string  `json:"input_error,omitempty"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	compiler := flag.String("compiler", defaultCompiler, "read-only pinned compiler")
	verify := flag.String("verify", "", "verify a saved output directory without executing policy or spy")
	flag.Parse()
	if *verify != "" {
		return verifySaved(*verify)
	}
	if err := os.MkdirAll("out", 0755); err != nil {
		return err
	}
	out, err := os.MkdirTemp("out", "run-")
	if err != nil {
		return err
	}
	out, err = filepath.Abs(out)
	if err != nil {
		return err
	}
	fmt.Println("Evidence:", out)
	for _, name := range []string{"policy.gooo", "gooo.workspace.json", "fixtures.json", "contract.go", "evidence.go", "main.go", "saved.go", "contract_test.go", "README.md", "go.mod", "run.sh"} {
		raw, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(out, name), raw, 0644); err != nil {
			return err
		}
	}
	fixtures, valid, suite, err := loadFixtures(filepath.Join(out, "fixtures.json"))
	if err != nil {
		return err
	}
	cases := append(marshal(suite), '\n')
	if err = save(out, "cases.json", cases); err != nil {
		return err
	}
	identity, err := command(out, "compiler", *compiler, "version", "--build", "--json")
	if err != nil {
		return err
	}
	if err = checkIdentity(identity); err != nil {
		return err
	}
	compilerBytes, err := os.ReadFile(*compiler)
	if err != nil {
		return err
	}
	if err = save(out, "compiler-sha256.txt", []byte(digest(compilerBytes)+"\n")); err != nil {
		return err
	}
	manifestPath := filepath.Join(out, "gooo.workspace.json")
	casesPath := filepath.Join(out, "cases.json")
	_, err = command(out, "execution", *compiler, "package", "execute", "--json", "--cases", casesPath, manifestPath)
	if err != nil {
		return fmt.Errorf("no mock dispatch: %w", err)
	}
	_, err = command(out, "replay", *compiler, "package", "replay", "--json", "--receipt", filepath.Join(out, "execution.json"), "--cases", casesPath, manifestPath)
	if err != nil {
		return fmt.Errorf("no mock dispatch: %w", err)
	}
	actual, err := verifyReceipts(out, valid)
	if err != nil {
		return fmt.Errorf("no mock dispatch: %w", err)
	}
	// Historical replay above verifies observations only; it never dispatches.
	rows := make([]CaseResult, 0, len(fixtures))
	j := 0
	allows := 0
	for _, f := range fixtures {
		s := spy{}
		_, inputErr := decodeEnvelope(f.raw())
		result := CaseResult{ID: f.ID, InputDigest: digest(f.raw())}
		if inputErr != nil {
			result.Outcome = Outcome{false, "reject", "invalid_input"}
			result.Error = inputErr.Error()
			_ = dispatch(Outcome{}, inputErr, &s)
		} else {
			result.Outcome = actual[j]
			j++
			if err = dispatch(result.Outcome, nil, &s); err != nil {
				return err
			}
		}
		result.SpyCalls = s.calls
		allows += s.calls
		if result.Outcome != f.Expected.Outcome || s.calls != f.Expected.SpyCalls {
			return fmt.Errorf("%s: manual outcome/spy mismatch", f.ID)
		}
		rows = append(rows, result)
	}
	if err = save(out, "observations.json", append(marshal(rows), '\n')); err != nil {
		return err
	}
	// Exercise an actual compiler failure with a missing required context field.
	bad := Suite{Schema: suite.Schema, Cases: []Case{suite.Cases[0]}}
	bad.Cases[0].Inputs = map[string]any{}
	for k, v := range suite.Cases[0].Inputs {
		bad.Cases[0].Inputs[k] = v
	}
	var missing map[string]any
	_ = json.Unmarshal(marshal(bad.Cases[0].Inputs[entry+".input1"]), &missing)
	delete(missing, "now")
	bad.Cases[0].Inputs[entry+".input1"] = missing
	if err = save(out, "missing-context-cases.json", append(marshal(bad), '\n')); err != nil {
		return err
	}
	_, failed := command(out, "missing-context", *compiler, "package", "execute", "--json", "--cases", filepath.Join(out, "missing-context-cases.json"), manifestPath)
	errorSpy := spy{}
	if failed == nil {
		return errors.New("missing-context compiler probe unexpectedly succeeded")
	}
	_ = dispatch(Outcome{true, "allow", "policy_satisfied"}, failed, &errorSpy)
	if errorSpy.calls != 0 {
		return errors.New("compiler error dispatched")
	}
	if err = verifyFailureProbe(out); err != nil {
		return err
	}
	summary := map[string]any{"schema": "decision-policy-pilot/summary/v1", "status": "PASS", "synthetic_context": true, "provider_mode": "offline provider-neutral Choice fixtures", "compiler_revision": compilerRevision, "go_version": "go1.27.2", "fixtures": len(fixtures), "gooo_reference_cases": len(valid), "manual_expected_cases": len(fixtures), "host_input_rejections": len(fixtures) - len(valid), "mock_dispatches": allows, "replay_dispatches": 0, "actual_compiler_failure_probes": 1, "compiler_failure_dispatches": errorSpy.calls, "external_api_calls": 0, "real_exports": 0, "performance_measured": false, "scope": "Finite synthetic observations; no customer, provider accuracy, authentication, transaction or production safety claim."}
	if err = save(out, "summary.json", append(marshal(summary), '\n')); err != nil {
		return err
	}
	if err = writeChecksums(out); err != nil {
		return err
	}
	if err = verifySaved(out); err != nil {
		return err
	}
	fmt.Printf("PASS: %d Gooo/Go/manual matches, %d input rejections, %d mock calls, replay/error dispatches 0\n", len(valid), len(fixtures)-len(valid), allows)
	return nil
}
func save(out, name string, raw []byte) error {
	return os.WriteFile(filepath.Join(out, name), raw, 0644)
}
func command(out, name, bin string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = append(os.Environ(), "GOOO_LAYA_URL=", "GOOO_LAYA_API_KEY=")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	raw, err := cmd.Output()
	if writeErr := save(out, name+".json", raw); writeErr != nil {
		return nil, writeErr
	}
	if writeErr := save(out, name+".stderr.txt", []byte(stderr.String())); writeErr != nil {
		return nil, writeErr
	}
	exit := 0
	if err != nil {
		exit = -1
		var e *exec.ExitError
		if errors.As(err, &e) {
			exit = e.ExitCode()
		}
	}
	if writeErr := save(out, name+".exit.txt", []byte(fmt.Sprint(exit))); writeErr != nil {
		return nil, writeErr
	}
	if writeErr := save(out, name+".command.json", append(marshal(append([]string{bin}, args...)), '\n')); writeErr != nil {
		return nil, writeErr
	}
	if err != nil {
		return raw, fmt.Errorf("%s failed: %w (saved stderr)", name, err)
	}
	return raw, nil
}
func checkIdentity(raw []byte) error {
	var b struct {
		Revision string `json:"compiler_source_sha"`
		Go       string `json:"go_version"`
		Native   string `json:"native_go_required"`
		Version  string `json:"version"`
		Modified string `json:"vcs_modified"`
		Status   string `json:"source_status"`
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		return err
	}
	if b.Revision != compilerRevision || b.Go != "go1.27.2" || b.Native != "go1.27.2" || b.Version != "0.6.24-dev" || b.Modified != "false" || b.Status != "CLEAN_VCS" {
		return errors.New("compiler identity mismatch")
	}
	return nil
}
func loadFixtures(path string) ([]Fixture, []Fixture, Suite, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, Suite{}, err
	}
	var all []Fixture
	if err = json.Unmarshal(raw, &all); err != nil {
		return nil, nil, Suite{}, err
	}
	valid := []Fixture{}
	suite := Suite{Schema: "gooo/body-composition-cases/v1"}
	seen := map[string]bool{}
	for _, f := range all {
		if f.ID == "" || seen[f.ID] || (f.RawInput == "") == (len(f.Input) == 0) {
			return nil, nil, suite, errors.New("invalid fixture identity/input")
		}
		seen[f.ID] = true
		e, err := decodeEnvelope(f.raw())
		if err != nil {
			if f.Expected != (Expected{Outcome{false, "reject", "invalid_input"}, 0}) {
				return nil, nil, suite, fmt.Errorf("%s unexpected input rejection: %v", f.ID, err)
			}
			continue
		}
		oracle := reference(e)
		if f.Expected.Outcome != oracle || f.Expected.SpyCalls != boolInt(oracle.Allowed) {
			return nil, nil, suite, fmt.Errorf("%s Go reference/manual disagreement", f.ID)
		}
		valid = append(valid, f)
		suite.Cases = append(suite.Cases, Case{map[string]any{entry + ".input0": e.Request, entry + ".input1": e.Context, entry + ".input2": e.Choice}, map[string]Outcome{entry: oracle}})
	}
	if len(all) != 49 || len(valid) != 31 {
		return nil, nil, suite, errors.New("fixture inventory changed; review manual counts")
	}
	return all, valid, suite, nil
}
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
func verifyReceipts(out string, valid []Fixture) ([]Outcome, error) {
	read := func(name string) ([]byte, error) { return os.ReadFile(filepath.Join(out, name)) }
	manifest, err := read("gooo.workspace.json")
	if err != nil {
		return nil, err
	}
	source, err := read("policy.gooo")
	if err != nil {
		return nil, err
	}
	cases, err := read("cases.json")
	if err != nil {
		return nil, err
	}
	execution, err := read("execution.json")
	if err != nil {
		return nil, err
	}
	replay, err := read("replay.json")
	if err != nil {
		return nil, err
	}
	a, err := inspectReceipt(execution, manifest, source, cases, valid, nil)
	if err != nil {
		return nil, err
	}
	b, err := inspectReceipt(replay, manifest, source, cases, valid, execution)
	if err != nil {
		return nil, err
	}
	var left, right Receipt
	_ = json.Unmarshal(execution, &left)
	_ = json.Unmarshal(replay, &right)
	if !reflect.DeepEqual(a, b) || !exact(marshal(left.Result.Runtime.Traces), right.Result.Runtime.Traces) || left.Result.Composition.Source != right.Result.Composition.Source {
		return nil, errors.New("saved replay differs")
	}
	return a, nil
}
