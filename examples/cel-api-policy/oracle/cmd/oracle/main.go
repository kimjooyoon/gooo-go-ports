// This independent harness evaluates the typed-protobuf CEL expression published
// by Google in the codelab pinned in source-pin.json. It does not duplicate its
// equality/OR decision in Go. Presence metadata is an original adapter contract.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	rpcpb "google.golang.org/genproto/googleapis/rpc/context/attribute_context"
	"google.golang.org/protobuf/types/known/structpb"
)

const expression = "request.auth.claims.group == 'admin' || request.auth.principal == 'user:me@acme.co'"
const maxTextBytes = 65536

type Facts struct {
	GroupPresent     bool   `json:"group_present"`
	GroupValue       string `json:"group_value"`
	PrincipalPresent bool   `json:"principal_present"`
	PrincipalValue   string `json:"principal_value"`
}

type Case struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Facts    Facts  `json:"facts"`
	Expected string `json:"expected"`
	Basis    string `json:"basis"`
}

type ValueObservation struct {
	Type      string `json:"type"`
	Value     any    `json:"value,omitempty"`
	Error     string `json:"error,omitempty"`
	EvalError string `json:"eval_error,omitempty"`
}

type Decision struct {
	Allowed            bool   `json:"allowed"`
	EvaluationError    bool   `json:"evaluation_error"`
	Result             string `json:"result"`
	GroupMatched       bool   `json:"group_matched"`
	PrincipalMatched   bool   `json:"principal_matched"`
	GroupMissing       bool   `json:"group_missing"`
	PrincipalDefaulted bool   `json:"principal_defaulted"`
	EffectivePrincipal string `json:"effective_principal"`
}

type Observation struct {
	Case
	Observed           Decision                    `json:"observed"`
	CEL                map[string]ValueObservation `json:"cel"`
	ExpectationMatches bool                        `json:"expectation_matches"`
}

type Report struct {
	Schema        string        `json:"schema"`
	Status        string        `json:"status"`
	ExecutedAt    string        `json:"executed_at_utc"`
	GoVersion     string        `json:"go_version"`
	Expression    string        `json:"expression"`
	Dependency    string        `json:"dependency"`
	InputDomain   string        `json:"input_domain"`
	PresenceBasis string        `json:"presence_basis"`
	Count         int           `json:"case_count"`
	Observations  []Observation `json:"observations"`
}

func validate(c Case) error {
	if c.ID == "" || c.Category == "" || c.Basis == "" {
		return errors.New("id/category/basis must be nonempty")
	}
	if c.Expected != "ALLOW" && c.Expected != "DENY" && c.Expected != "ERROR_NO_SUCH_KEY_GROUP" {
		return errors.New("unexpected authored result label")
	}
	for name, value := range map[string]string{"group_value": c.Facts.GroupValue, "principal_value": c.Facts.PrincipalValue} {
		if !utf8.ValidString(value) || len(value) > maxTextBytes {
			return fmt.Errorf("%s must be valid UTF-8 and at most %d bytes", name, maxTextBytes)
		}
	}
	return nil
}

func compile(env *cel.Env, expr string) (cel.Program, error) {
	ast, issues := env.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return nil, issues.Err()
	}
	return env.Program(ast)
}

func snapshot(val ref.Val, err error) ValueObservation {
	o := ValueObservation{}
	if err != nil {
		o.EvalError = err.Error()
	}
	if val == nil {
		o.Type = "nil"
		return o
	}
	o.Type = val.Type().TypeName()
	if types.IsError(val) {
		o.Error = fmt.Sprint(val.Value())
	} else {
		o.Value = val.Value()
	}
	return o
}

func observe(c Case, programs map[string]cel.Program) (Observation, error) {
	if err := validate(c); err != nil {
		return Observation{}, err
	}
	fields := map[string]*structpb.Value{}
	if c.Facts.GroupPresent {
		fields["group"] = structpb.NewStringValue(c.Facts.GroupValue)
	}
	auth := &rpcpb.AttributeContext_Auth{Claims: &structpb.Struct{Fields: fields}}
	// Leaving the proto3 string field unset is different from a missing map key.
	// An absent scalar placeholder is ignored, even when it names a privileged user.
	if c.Facts.PrincipalPresent {
		auth.Principal = c.Facts.PrincipalValue
	}
	activation := map[string]any{"request": &rpcpb.AttributeContext_Request{Auth: auth}}
	obs := Observation{Case: c, CEL: map[string]ValueObservation{}}
	values := map[string]ref.Val{}
	errorsByName := map[string]error{}
	for name, program := range programs {
		val, _, err := program.Eval(activation)
		values[name], errorsByName[name] = val, err
		obs.CEL[name] = snapshot(val, err)
	}
	final := values["decision"]
	d := Decision{PrincipalDefaulted: !c.Facts.PrincipalPresent}
	if errorsByName["decision"] != nil || types.IsError(final) {
		if !strings.Contains(obs.CEL["decision"].Error+obs.CEL["decision"].EvalError, "no such key: group") {
			return Observation{}, fmt.Errorf("unexpected CEL error: %+v", obs.CEL["decision"])
		}
		d.EvaluationError, d.Result = true, "ERROR_NO_SUCH_KEY_GROUP"
	} else if final == types.True {
		d.Allowed, d.Result = true, "ALLOW"
	} else if final == types.False {
		d.Result = "DENY"
	} else {
		return Observation{}, fmt.Errorf("non-boolean/non-error CEL result: %+v", obs.CEL["decision"])
	}
	d.GroupMatched = values["group_comparison"] == types.True
	d.PrincipalMatched = values["principal_comparison"] == types.True
	if errorsByName["group_missing"] != nil {
		return Observation{}, fmt.Errorf("group presence evaluation failed: %w", errorsByName["group_missing"])
	}
	d.GroupMissing = values["group_missing"] == types.True
	principal, ok := values["effective_principal"].(types.String)
	if !ok || errorsByName["effective_principal"] != nil {
		return Observation{}, errors.New("principal was not evaluated as the declared proto string")
	}
	d.EffectivePrincipal = string(principal)
	obs.Observed = d
	obs.ExpectationMatches = d.Result == c.Expected
	return obs, nil
}

func main() {
	casePath := flag.String("cases", "cases.json", "JSON array of independent cases, or - for stdin")
	outPath := flag.String("out", "-", "JSON report destination, or - for stdout")
	flag.Parse()
	var in io.Reader = os.Stdin
	if *casePath != "-" {
		file, err := os.Open(*casePath)
		if err != nil {
			fail(err)
		}
		defer file.Close()
		in = file
	}
	decoder := json.NewDecoder(in)
	decoder.DisallowUnknownFields()
	var cases []Case
	if err := decoder.Decode(&cases); err != nil {
		fail(err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		fail(errors.New("trailing JSON/data is not permitted"))
	}
	env, err := cel.NewEnv(cel.Types(&rpcpb.AttributeContext_Request{}), cel.Variable("request", cel.ObjectType("google.rpc.context.AttributeContext.Request")))
	if err != nil {
		fail(err)
	}
	programs := map[string]cel.Program{}
	for name, expr := range map[string]string{
		"decision":             expression,
		"group_comparison":     "request.auth.claims.group == 'admin'",
		"principal_comparison": "request.auth.principal == 'user:me@acme.co'",
		"group_missing":        "!has(request.auth.claims.group)",
		"effective_principal":  "request.auth.principal",
	} {
		program, err := compile(env, expr)
		if err != nil {
			fail(fmt.Errorf("compile %s: %w", name, err))
		}
		programs[name] = program
	}
	report := Report{Schema: "gooo-cel-typed-proto-oracle/v1", Status: "EXECUTED_REAL_CEL_EVALUATOR", ExecutedAt: time.Now().UTC().Format(time.RFC3339Nano), GoVersion: runtime.Version(), Expression: expression, Dependency: "cel.dev/cel-go@v0.32.0", InputDomain: "group absent/string; principal present string/omitted proto default; each raw string valid UTF-8 <=65536 bytes; unknown/non-string group out of scope", PresenceBasis: "group_missing is real CEL !has(...) evaluation; principal_defaulted is original adapter presence metadata, not a proto3 string/CEL property"}
	seen := map[string]bool{}
	for _, c := range cases {
		if seen[c.ID] {
			fail(fmt.Errorf("duplicate case id %s", c.ID))
		}
		seen[c.ID] = true
		observation, err := observe(c, programs)
		if err != nil {
			fail(fmt.Errorf("case %s: %w", c.ID, err))
		}
		if !observation.ExpectationMatches {
			fail(fmt.Errorf("case %s: expected %s; actual %s", c.ID, c.Expected, observation.Observed.Result))
		}
		report.Observations = append(report.Observations, observation)
	}
	report.Count = len(report.Observations)
	var out io.Writer = os.Stdout
	if *outPath != "-" {
		file, err := os.Create(*outPath)
		if err != nil {
			fail(err)
		}
		defer file.Close()
		out = file
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(report); err != nil {
		fail(err)
	}
}

func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
