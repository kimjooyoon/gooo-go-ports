package aliasorigin

import (
	"encoding/json"
	"fmt"
	"github.com/kimjooyoon/gooo-go-ports/examples/alias-origin-change/internal/kernel"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func sample() Input {
	return Input{TargetAlias: "docs_search", Before: []Entry{{"docs", "search", "docs_search"}}, After: []Entry{{"issues", "search", "docs_search"}}}
}
func TestRowAPIForEveryRepresentableOriginalOracleCase(t *testing.T) {
	raw, err := os.ReadFile("kernel.cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Cases []struct {
			Inputs   map[string]kernel.Input `json:"inputs"`
			Expected map[string]Decision     `json:"expected"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	count := 0
	for i, c := range doc.Cases {
		x := c.Inputs["gooo/tool-origin:CompareAliasOrigin"]
		if x.ContextID == "" || len(x.BeforeServers) != len(x.BeforeAliases) || len(x.BeforeNames) != len(x.BeforeAliases) || len(x.AfterServers) != len(x.AfterAliases) || len(x.AfterNames) != len(x.AfterAliases) {
			continue
		}
		in := Input{TargetAlias: x.TargetAlias}
		for j, a := range x.BeforeAliases {
			in.Before = append(in.Before, Entry{x.BeforeServers[j], x.BeforeNames[j], a})
		}
		for j, a := range x.AfterAliases {
			in.After = append(in.After, Entry{x.AfterServers[j], x.AfterNames[j], a})
		}
		got, err := Compare(in)
		if err != nil {
			t.Fatalf("case%d: %v", i, err)
		}
		want := c.Expected["gooo/tool-origin:CompareAliasOrigin"]
		want.ContextID = "gooo/alias-origin/v1/" + got.FactsSHA256
		if !reflect.DeepEqual(got.Decision, want) {
			t.Fatalf("public case%d differs: %+v != %+v", i, got.Decision, want)
		}
		count++
	}
	if count != 104 {
		t.Fatalf("public row coverage changed: %d; kernel still must check111", count)
	}
}
func TestRawBytesFactsIdentityAndOwnedSnapshots(t *testing.T) {
	in := sample()
	raw, _ := json.Marshal(in)
	pretty, _ := json.MarshalIndent(in, "", "  ")
	a, err := CompareJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	b, err := CompareJSON(pretty)
	if err != nil {
		t.Fatal(err)
	}
	if a.Decision.State != "ORIGIN_CHANGED" || a.Decision.ExecutionAuthorized || a.InputSHA256 == b.InputSHA256 || a.FactsSHA256 != b.FactsSHA256 || a.Decision != b.Decision {
		t.Fatal("byte/fact identity or actual decision lost")
	}
	native, err := Compare(in)
	if err != nil {
		t.Fatal(err)
	}
	in.After[0].Server = "mutated"
	if native.Facts.After[0].Server != "issues" {
		t.Fatal("caller changed retained facts")
	}
	native.Bindings[0] = 'X'
	again, err := Compare(sample())
	if err != nil || !json.Valid(again.Bindings) {
		t.Fatal("caller changed embedded generation")
	}
}
func TestTransportNeverRepairsOrAcceptsCallerAuthority(t *testing.T) {
	raw, _ := json.Marshal(sample())
	text := string(raw)
	invalid := []string{text[:len(text)-1] + `,"execution_authorized":true}`, strings.Replace(text, `"target_alias":"docs_search"`, `"target_alias":"docs_search","target_alias":"other"`, 1), `{"target_alias":"docs_search","before":null,"after":[]}`, strings.Replace(text, `"docs_search"`, `"\ud800"`, 1), text + ` {}`, strings.Repeat(" ", MaxInputBytes+1)}
	for i, s := range invalid {
		if r, err := CompareJSON([]byte(s)); err == nil || !reflect.DeepEqual(r, Report{}) {
			t.Fatalf("bad transport%d produced result", i)
		}
	}
	large := sample()
	large.Before = make([]Entry, MaxEntries+1)
	if _, err := Compare(large); err == nil {
		t.Fatal("native collection bound bypassed")
	}
	bad := sample()
	bad.After[0].OriginalName = string([]byte{255})
	if _, err := Compare(bad); err == nil {
		t.Fatal("lossy UTF8 accepted")
	}
}
func TestMaximumCatalogAndConcurrentIndependentReports(t *testing.T) {
	in := Input{TargetAlias: "alias-255"}
	for i := 0; i < 256; i++ {
		e := Entry{fmt.Sprintf("srv-%d", i), "search", fmt.Sprintf("alias-%d", i)}
		in.Before = append(in.Before, e)
		in.After = append(in.After, e)
	}
	r, err := Compare(in)
	if err != nil || r.Decision.State != "UNCHANGED" || r.Decision.BeforeIndex != 255 || r.Decision.AfterIndex != 255 {
		t.Fatal("maximum original indices changed", r.Decision, err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			x, err := Compare(in)
			if err != nil || x.Decision != r.Decision {
				t.Error("concurrent report drift", err)
			}
		}()
	}
	wg.Wait()
}
