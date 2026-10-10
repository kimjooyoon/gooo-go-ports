package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// Synthetic receipts isolate consistency checks from compilation. The
// contract and source expectations are never obtained from a receipt.
func contractFixture(t *testing.T, s suite) ([]byte, suite, expectedIdentity) {
	t.Helper()
	raw, _, identity := fixture(t)
	var e envelope
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	s.Cases = s.Cases[:1]
	contract, err := contractFor(s)
	if err != nil {
		t.Fatal(err)
	}
	identity.Sources = fixtureSources(t, s)
	e.Result.Program.Entry.Package, e.Result.Program.Entry.Activity = s.Package, s.Activity
	e.Result.Program.Image = fixtureWorkspace(t, s, identity.Sources)
	e.Result.Composition.Steps[0].Generation.Report.ID = contract.ID
	d := &e.Result.Runtime.Traces[0].Deliveries[0]
	d.ID = contract.ID
	row := s.Cases[0]
	if len(row.Inputs) == 1 {
		d.Input, d.Inputs = mustJSON(t, row.Inputs[0]), nil
		d.InputFields = rectangleFixtureFields(t, contract.InputEntities[0], row.Inputs[0])
	} else {
		d.Input = nil
		for len(d.Inputs) < len(row.Inputs) {
			d.Inputs = append(d.Inputs, d.Inputs[0])
		}
		d.Inputs = d.Inputs[:len(row.Inputs)]
		for i, input := range row.Inputs {
			d.Inputs[i].Port = fmt.Sprintf("input%d", i)
			d.Inputs[i].Entity = contract.InputEntities[i]
			d.Inputs[i].Value = mustJSON(t, input)
			d.Inputs[i].Fields = rectangleFixtureFields(t, contract.InputEntities[i], input)
		}
	}
	d.Expected, d.Actual = mustJSON(t, row.Expected), mustJSON(t, row.Expected)
	for _, field := range whitespaceResultContract(s).Fields {
		d.ActualFields = append(d.ActualFields, recordFieldValue{field.ID, field.Name, mustJSON(t, row.Expected.(map[string]any)[field.Name])})
	}
	raw = mustJSON(t, e)
	if _, err := inspect(raw, s, false, identity); err != nil {
		t.Fatalf("valid contract fixture %s:%s: %v", s.Package, s.Activity, err)
	}
	return raw, s, identity
}

func TestAllNativeContractsAcceptConsistentReceipt(t *testing.T) {
	for _, s := range allNativeSuites(t) {
		t.Run(s.Package+":"+s.Activity, func(t *testing.T) { contractFixture(t, s) })
	}
}

func TestInspectRejectsCorrelatedAndWorkspaceIdentityTampering(t *testing.T) {
	raw, s, identity := fixture(t)
	if _, err := inspect(raw, s, false, identity); err != nil {
		t.Fatal(err)
	}
	wrong := "gooo://untrusted/wrong-activity"
	tests := map[string]func(*envelope){
		"generation ID only": func(e *envelope) { e.Result.Composition.Steps[0].Generation.Report.ID = wrong },
		"delivery ID only":   func(e *envelope) { e.Result.Runtime.Traces[0].Deliveries[0].ID = wrong },
		"both activity IDs": func(e *envelope) {
			e.Result.Composition.Steps[0].Generation.Report.ID = wrong
			e.Result.Runtime.Traces[0].Deliveries[0].ID = wrong
		},
		"all activity IDs including export": func(e *envelope) {
			e.Result.Composition.Steps[0].Generation.Report.ID = wrong
			e.Result.Runtime.Traces[0].Deliveries[0].ID = wrong
			e.Result.Program.Image.Packages[0].Exports[0].ID = wrong
		},
		"missing selected export":    func(e *envelope) { e.Result.Program.Image.Packages[0].Exports = nil },
		"wrong selected export name": func(e *envelope) { e.Result.Program.Image.Packages[0].Exports[0].Name = "Other" },
		"wrong selected export kind": func(e *envelope) { e.Result.Program.Image.Packages[0].Exports[0].Kind = "entity" },
		"wrong selected export ID":   func(e *envelope) { e.Result.Program.Image.Packages[0].Exports[0].ID = wrong },
		"missing export input types": func(e *envelope) { e.Result.Program.Image.Packages[0].Exports[0].InputTypes = nil },
		"wrong export input type":    func(e *envelope) { e.Result.Program.Image.Packages[0].Exports[0].InputTypes[0] = "gooo://std/text" },
		"wrong export input arity":   func(e *envelope) { e.Result.Program.Image.Packages[0].Exports[0].InputTypes = []string{integerEntity} },
		"duplicate selected export": func(e *envelope) {
			p := &e.Result.Program.Image.Packages[0]
			p.Exports = append(p.Exports, p.Exports[0])
		},
		"duplicate selected ID under another name": func(e *envelope) {
			p := &e.Result.Program.Image.Packages[0]
			other := p.Exports[0]
			other.Name = "Alias"
			p.Exports = append(p.Exports, other)
		},
		"missing workspace package":   func(e *envelope) { e.Result.Program.Image.Packages = nil },
		"duplicate workspace package": func(e *envelope) { p := &e.Result.Program.Image.Packages; *p = append(*p, (*p)[0]) },
		"unexpected empty workspace package": func(e *envelope) {
			p := &e.Result.Program.Image.Packages
			*p = append(*p, workspacePackage{Path: "untrusted"})
		},
		"missing workspace source": func(e *envelope) { e.Result.Program.Image.Packages[0].Sources = nil },
		"duplicate workspace source": func(e *envelope) {
			p := &e.Result.Program.Image.Packages[0]
			p.Sources = append(p.Sources, p.Sources[0])
		},
		"wrong workspace source name": func(e *envelope) { e.Result.Program.Image.Packages[0].Sources[0].Filename = "untrusted.gooo" },
		"wrong workspace source digest": func(e *envelope) {
			e.Result.Program.Image.Packages[0].Sources[0].Digest = "sha256:" + strings.Repeat("0", 64)
		},
		"wrong workspace source owner": func(e *envelope) { e.Result.Program.Image.Packages[0].Path = "go/unicode/utf8" },
		"missing generated source":     func(e *envelope) { e.Result.Composition.Source = "" },
		"changed generated source":     func(e *envelope) { e.Result.Composition.Source += "\n// changed\n" },
		"both generated hashes changed": func(e *envelope) {
			e.Result.Composition.SHA = "sha256:" + strings.Repeat("0", 64)
			e.Result.Runtime.Generated = e.Result.Composition.SHA
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			var e envelope
			if err := json.Unmarshal(raw, &e); err != nil {
				t.Fatal(err)
			}
			mutate(&e)
			if _, err := inspect(mustJSON(t, e), s, false, identity); err == nil {
				t.Fatal("accepted identity/source tampering")
			}
		})
	}
	identity.Sources = nil
	if _, err := inspect(raw, s, false, identity); err == nil {
		t.Fatal("accepted missing trusted sources")
	}
	s.Package = "unregistered"
	if _, err := inspect(raw, s, false, identity); err == nil {
		t.Fatal("accepted missing activity contract")
	}
}

func TestInputEntityIdentityForAllMultiInputArities(t *testing.T) {
	for _, s := range allNativeSuites(t) {
		if len(s.Cases[0].Inputs) == 1 {
			continue
		}
		raw, s, identity := contractFixture(t, s)
		for i := range s.Cases[0].Inputs {
			for _, entity := range []string{"", "gooo://std/text", "gooo://untrusted/integer"} {
				t.Run(fmt.Sprintf("%s/%s/input%d/%s", s.Package, s.Activity, i, entity), func(t *testing.T) {
					var e envelope
					if err := json.Unmarshal(raw, &e); err != nil {
						t.Fatal(err)
					}
					e.Result.Runtime.Traces[0].Deliveries[0].Inputs[i].Entity = entity
					if _, err := inspect(mustJSON(t, e), s, false, identity); err == nil {
						t.Fatal("accepted wrong or missing input entity")
					}
				})
			}
		}
	}
}

func TestSingleInputEntityUsesIndependentExportContract(t *testing.T) {
	s := suite{Package: "go/net/http", Activity: "StatusText", Cases: []testCase{{Inputs: []any{int64(404)}, Expected: "Not Found"}}}
	raw, s, identity := contractFixture(t, s)
	var e envelope
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	// Unary deliveries have only `input`, so its type must be checked at the
	// selected export against the independent declaration contract.
	if len(e.Result.Runtime.Traces[0].Deliveries[0].Inputs) != 0 {
		t.Fatal("unexpected unary schema")
	}
	e.Result.Program.Image.Packages[0].Exports[0].InputTypes = []string{"gooo://std/text"}
	if _, err := inspect(mustJSON(t, e), s, false, identity); err == nil {
		t.Fatal("accepted wrong unary input entity")
	}
}
