package main

import (
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func allNativeSuites(t *testing.T) []suite {
	t.Helper()
	suites, err := corpus()
	if err != nil {
		t.Fatal(err)
	}
	unicode, err := unicodeInspectorCorpus(suites)
	if err != nil {
		t.Fatal(err)
	}
	colors, err := colorCorpus()
	if err != nil {
		t.Fatal(err)
	}
	return append(append(suites, unicode.Suites...), colors.Suites...)
}

func TestContractsMatchReviewedSourcesAndUnchangedCorpus(t *testing.T) {
	contracts := nativeActivityContracts()
	suites := allNativeSuites(t)
	if len(contracts) != 20 || len(suites) != 20 {
		t.Fatal("contract/suite roster changed")
	}
	seen := map[string]bool{}
	total := 0
	for _, s := range suites {
		key := s.Package + ":" + s.Activity
		if seen[key] {
			t.Fatal("duplicate suite contract", key)
		}
		seen[key] = true
		contract, err := contractFor(s)
		if err != nil {
			t.Fatal(err)
		}
		source, err := os.ReadFile("../../" + contract.Source)
		if err != nil {
			t.Fatal(err)
		}
		pattern := `(?m)^activity ` + regexp.QuoteMeta(s.Activity) + `\(([^)]*)\) -> [^\n]+? id "([^"]+)"`
		match := regexp.MustCompile(pattern).FindSubmatch(source)
		if len(match) != 3 || string(match[2]) != contract.ID {
			t.Fatalf("contract disagrees with reviewed declaration: %s", key)
		}
		inputs := strings.Split(string(match[1]), ",")
		if len(inputs) != len(contract.InputEntities) {
			t.Fatal("contract arity differs from source", key)
		}
		for i, input := range inputs {
			// These twenty native activities all declare the same Integer ID.
			if strings.TrimSpace(input) != "Integer" || contract.InputEntities[i] != integerEntity || !strings.Contains(string(source), `entity Integer id "`+integerEntity+`"`) {
				t.Fatal("contract input entity differs from source", key)
			}
		}
		for _, c := range s.Cases {
			if len(c.Inputs) != len(contract.InputEntities) {
				t.Fatal("corpus arity differs from contract", key)
			}
		}
		total += len(s.Cases)
	}
	if total != 5510 {
		t.Fatalf("corpus changed: %d", total)
	}
	if _, err := contractFor(suite{Package: "go/net/http", Activity: "Unknown"}); err == nil {
		t.Fatal("accepted missing contract")
	}
	if _, err := contractFor(suite{Package: "untrusted", Activity: "StatusText"}); err == nil {
		t.Fatal("accepted wrong package contract")
	}
}

func TestFrozenSourceIdentities(t *testing.T) {
	const name = "source.gooo"
	// HTML escaping, quotes, newlines and non-ASCII are deliberate: hashing
	// raw source bytes, rather than its JSON string, must not match.
	raw := []byte("package sample\n// 한글 <>& \"quote\"\n")
	packages := []sourcePackage{{"sample", []string{name}}}
	expected, err := snapshotSourceIdentities(packages, map[string][]byte{name: raw})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if expected[name].Package != "sample" || expected[name].Digest != contentDigest(encoded) || expected[name].Digest == contentDigest(raw) {
		t.Fatal("wrong source digest or package")
	}
	before := expected[name]
	raw[0] = 'X'
	if expected[name] != before {
		t.Fatal("frozen digest changed when source buffer changed")
	}
	for name, test := range map[string]struct {
		packages []sourcePackage
		sources  map[string][]byte
	}{
		"missing bytes":        {packages, nil},
		"empty roster":         {nil, map[string][]byte{}},
		"unclaimed bytes":      {packages, map[string][]byte{"source.gooo": raw, "extra.gooo": raw}},
		"duplicate package":    {append(packages, packages[0]), map[string][]byte{"source.gooo": raw}},
		"duplicate source":     {[]sourcePackage{{"sample", []string{"source.gooo", "source.gooo"}}}, map[string][]byte{"source.gooo": raw}},
		"cross-package source": {[]sourcePackage{{"sample", []string{"source.gooo"}}, {"other", []string{"source.gooo"}}}, map[string][]byte{"source.gooo": raw}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := snapshotSourceIdentities(test.packages, test.sources); err == nil {
				t.Fatal("accepted incomplete or ambiguous source snapshot")
			}
		})
	}
}

func TestWorkspaceBindingIncludesSourceOwner(t *testing.T) {
	s := suite{Package: "go/unicode/utf16", Activity: "DecodeRune"}
	contract, _ := contractFor(s)
	identities := fixtureSources(t, s)
	other := "ports/unicode/utf8.gooo"
	identities[other] = sourceIdentity{"go/unicode/utf8", contentDigest([]byte(`"other source"`))}
	image := fixtureWorkspace(t, s, identities)
	image.Packages = append(image.Packages, workspacePackage{Path: "go/unicode/utf8", Sources: []workspaceSource{{other, identities[other].Digest}}})
	if err := inspectWorkspace(image, s, contract, identities); err != nil {
		t.Fatal(err)
	}
	// Keep every filename/digest pair, but move each source to the wrong owner.
	image.Packages[0].Sources, image.Packages[1].Sources = image.Packages[1].Sources, image.Packages[0].Sources
	if err := inspectWorkspace(image, s, contract, identities); err == nil {
		t.Fatal("accepted unchanged source hashes under wrong packages")
	}
	if !reflect.DeepEqual(identities[contract.Source], fixtureSources(t, s)[contract.Source]) {
		t.Fatal("inspection mutated trusted expectation")
	}
}
