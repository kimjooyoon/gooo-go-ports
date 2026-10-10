package main

import (
	"fmt"
	"testing"
)

func TestInspectRejectsEntryIdentityTampering(t *testing.T) {
	raw, lock, sources := evidenceFixture(t)
	tests := map[string]func(*receipt){
		"missing entry export": func(e *receipt) {
			e.Result.Program.Image.Packages[1].Exports = e.Result.Program.Image.Packages[1].Exports[:1]
		},
		"wrong export name": func(e *receipt) { e.Result.Program.Image.Packages[1].Exports[1].Name = "Other" },
		"wrong export kind": func(e *receipt) { e.Result.Program.Image.Packages[1].Exports[1].Kind = "entity" },
		"missing export ID": func(e *receipt) { e.Result.Program.Image.Packages[1].Exports[1].ID = "" },
		"wrong export ID":   func(e *receipt) { e.Result.Program.Image.Packages[1].Exports[1].ID = "other" },
		"duplicate entry export": func(e *receipt) {
			p := &e.Result.Program.Image.Packages[1]
			p.Exports = append(p.Exports, p.Exports[1])
		},
		"entry ID reused under another name": func(e *receipt) {
			p := &e.Result.Program.Image.Packages[1]
			alias := p.Exports[1]
			alias.Name = "Alias"
			p.Exports = append(p.Exports, alias)
		},
		"missing export inputs": func(e *receipt) { e.Result.Program.Image.Packages[1].Exports[1].InputTypes = nil },
		"extra export input": func(e *receipt) {
			x := &e.Result.Program.Image.Packages[1].Exports[1]
			x.InputTypes = append(x.InputTypes, "gooo://std/integer")
		},
		"missing last export input": func(e *receipt) {
			x := &e.Result.Program.Image.Packages[1].Exports[1]
			x.InputTypes = x.InputTypes[:2]
		},
		"export input order": func(e *receipt) {
			x := &e.Result.Program.Image.Packages[1].Exports[1]
			x.InputTypes[0], x.InputTypes[1] = x.InputTypes[1], x.InputTypes[0]
		},
		"missing export output": func(e *receipt) { e.Result.Program.Image.Packages[1].Exports[1].OutputType = "" },
		"wrong export output":   func(e *receipt) { e.Result.Program.Image.Packages[1].Exports[1].OutputType = "gooo://std/integer" },
		"missing generation report": func(e *receipt) {
			e.Result.Composition.Steps[0].Generation.Report.ID = ""
		},
		"wrong generation report": func(e *receipt) { e.Result.Composition.Steps[0].Generation.Report.ID = "other" },
		"missing generation step": func(e *receipt) { e.Result.Composition.Steps = nil },
		"duplicate generation step": func(e *receipt) {
			c := &e.Result.Composition
			c.Steps = append(c.Steps, c.Steps[0])
		},
		"all activity IDs changed together": func(e *receipt) {
			e.Result.Program.Image.Packages[1].Exports[1].ID = "other"
			e.Result.Composition.Steps[0].Generation.Report.ID = "other"
			for i := range e.Result.Runtime.Traces {
				e.Result.Runtime.Traces[i].Deliveries[0].ID = "other"
			}
		},
		"delivery entity order": func(e *receipt) {
			inputs := e.Result.Runtime.Traces[0].Deliveries[0].Inputs
			inputs[0].Entity, inputs[1].Entity = inputs[1].Entity, inputs[0].Entity
		},
		"wrong entity in later trace": func(e *receipt) {
			e.Result.Runtime.Traces[1].Deliveries[0].Inputs[2].Entity = "other"
		},
	}
	for position := 0; position < 5; position++ {
		for _, replacement := range []string{"", "gooo://std/boolean"} {
			tests[fmt.Sprintf("export input%d entity %q", position, replacement)] = func(e *receipt) {
				e.Result.Program.Image.Packages[1].Exports[1].InputTypes[position] = replacement
			}
			tests[fmt.Sprintf("delivery input%d entity %q", position, replacement)] = func(e *receipt) {
				e.Result.Runtime.Traces[0].Deliveries[0].Inputs[position].Entity = replacement
			}
		}
		tests[fmt.Sprintf("export and delivery input%d entities changed together", position)] = func(e *receipt) {
			e.Result.Program.Image.Packages[1].Exports[1].InputTypes[position] = "other"
			for i := range e.Result.Runtime.Traces {
				e.Result.Runtime.Traces[i].Deliveries[0].Inputs[position].Entity = "other"
			}
		}
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			e := decodeReceipt(t, raw)
			mutate(&e)
			if _, err := inspectReceipt(testJSON(t, e), lock, sources, ""); err == nil {
				t.Fatal("accepted a receipt identity instead of the independent browser contract")
			}
		})
	}
}

func TestInspectBindsSourcePackageOwnership(t *testing.T) {
	raw, lock, sources := evidenceFixture(t)
	tests := map[string]func(*receipt){
		"sources swapped between packages": func(e *receipt) {
			p := e.Result.Program.Image.Packages
			p[0].Sources, p[1].Sources = p[1].Sources, p[0].Sources
		},
		"all sources moved into entry package": func(e *receipt) {
			p := e.Result.Program.Image.Packages
			p[1].Sources = append(p[1].Sources, p[0].Sources...)
			p[0].Sources = nil
		},
		"unknown package": func(e *receipt) { e.Result.Program.Image.Packages[0].Path = "other" },
		"missing package": func(e *receipt) {
			e.Result.Program.Image.Packages = e.Result.Program.Image.Packages[1:]
		},
		"duplicate entry package without extra source": func(e *receipt) {
			p := &e.Result.Program.Image.Packages
			duplicate := (*p)[1]
			duplicate.Sources = nil
			*p = append(*p, duplicate)
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			e := decodeReceipt(t, raw)
			mutate(&e)
			if _, err := inspectReceipt(testJSON(t, e), lock, sources, ""); err == nil {
				t.Fatal("accepted a source roster disconnected from the trusted manifest packages")
			}
		})
	}
}

func TestInspectFindsEntryByPackageAndExportName(t *testing.T) {
	raw, lock, sources := evidenceFixture(t)
	e := decodeReceipt(t, raw)
	p := e.Result.Program.Image.Packages
	other := p[1].Exports[1]
	other.ID = "gooo://another-package/dispatch"
	p[0].Exports = append(p[0].Exports, other)
	p[1].Exports[0], p[1].Exports[1] = p[1].Exports[1], p[1].Exports[0]
	p[0], p[1] = p[1], p[0]
	if got, err := inspectReceipt(testJSON(t, e), lock, sources, ""); err != nil || got != generatedFixture {
		t.Fatalf("valid reordered packages and exports: %q, %v", got, err)
	}
}
