package catalogrefresh

import (
	"bytes"
	"context"
	"encoding/json"
	host "github.com/kimjooyoon/gooo-go-ports/examples/catalog-refresh/host"
	"os"
	"reflect"
	"testing"
)

// Every independent reference case reaches the actual generated function here.
// The nine oversized inputs are direct-function extensions; the CLI separately
// rejects them at typed admission, before executing the Gooo body.
func TestGeneratedFunctionAgainstIndependentReference(t *testing.T) {
	raw, err := os.ReadFile("kernel.cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Inputs   map[string]json.RawMessage
			Expected map[string]json.RawMessage
		}
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	const entry = "gooo/tool-origin/catalog-refresh:DecideCatalogRefresh"
	if len(corpus.Cases) != 115 {
		t.Fatalf("unaccounted corpus: %d", len(corpus.Cases))
	}
	for i, c := range corpus.Cases {
		var f host.SnapshotFacts
		var e host.RefreshEvidence
		var s host.RefreshState
		var want host.RefreshDecision
		for _, v := range []struct {
			raw json.RawMessage
			dst any
		}{{c.Inputs[entry+".input0"], &f}, {c.Inputs[entry+".input1"], &e}, {c.Inputs[entry+".input2"], &s}, {c.Expected[entry], &want}} {
			if err := json.Unmarshal(v.raw, v.dst); err != nil {
				t.Fatalf("case %d: %v", i, err)
			}
		}
		got, err := evaluateGooo(f, e, s)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("case %d: got %#v; want %#v", i, got, want)
		}
		var expected, actual map[string]any
		json.Unmarshal(c.Expected[entry], &expected)
		encoded, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		json.Unmarshal(encoded, &actual)
		if len(actual) != 18 || !reflect.DeepEqual(actual, expected) {
			t.Fatalf("case %d: JSON ABI mismatch", i)
		}
	}
}

func page(tools, next string) []SnapshotPage {
	suffix := ""
	if next != "" {
		b, _ := json.Marshal(next)
		suffix = ",\"nextCursor\":" + string(b)
	}
	return []SnapshotPage{{RequestJSON: `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`, ResponseJSON: `{"jsonrpc":"2.0","id":1,"result":{"tools":` + tools + suffix + `}}`}}
}

func TestGeneratedDecisionControlsRealReplacement(t *testing.T) {
	const valid = `[{"name":"lookup","inputSchema":{"type":"object"}}]`
	for _, tc := range []struct {
		name, tools, next, collector, want string
		pagesNil                           bool
	}{
		{"complete", valid, "", "SUCCESS", "COMPLETE_CANDIDATE", false},
		{"partial-success", valid, "more", "SUCCESS", "INCOMPLETE_CATALOG", false},
		{"collector-error", valid, "more", "ERROR", "COLLECTOR_ERROR", false},
		{"collector-timeout", valid, "more", "TIMEOUT", "COLLECTOR_TIMEOUT", false},
		{"collector-cancelled", valid, "", "CANCELLED", "COLLECTOR_CANCELLED", false},
		{"duplicate-names", `[{"name":"x","inputSchema":{"type":"object"}},{"name":"x","inputSchema":{"type":"object"}}]`, "", "SUCCESS", "DUPLICATE_NAME", false},
		{"bad-unselected-row", `[{"name":"lookup","inputSchema":{"type":"object"}},{"name":"bad","inputSchema":false}]`, "", "SUCCESS", "ROW_REJECTED", false},
		{"empty-terminal", "[]", "", "SUCCESS", "COMPLETE_CANDIDATE", false},
		{"no-pages", "[]", "", "SUCCESS", "INCOMPLETE_CATALOG", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			initial := []byte("previous stored catalog")
			r, err := NewRegistry(initial)
			if err != nil {
				t.Fatal(err)
			}
			before := r.Snapshot()
			a, err := r.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			pages := page(tc.tools, tc.next)
			if tc.pagesNil {
				pages = nil
			}
			receipt := r.Refresh(a, pages, tc.collector)
			if receipt.Decision == nil || receipt.Decision.Reason != tc.want {
				t.Fatalf("unexpected receipt: %+v", receipt)
			}
			after := r.Snapshot()
			if tc.want == "COMPLETE_CANDIDATE" {
				if receipt.State != "APPLIED" || after.RegistryVersion == before.RegistryVersion {
					t.Fatalf("not replaced: %+v", receipt)
				}
			} else if receipt.State != "RETAINED" || !bytes.Equal(after.Bytes, initial) || after.RegistryVersion != before.RegistryVersion {
				t.Fatalf("stored catalog changed after refusal: %+v", receipt)
			}
			if receipt.Decision.ExecutionAuthorized || receipt.Decision.LiveEffectPerformed {
				t.Fatal("pure decision claimed an effect")
			}
		})
	}
}

func TestGeneratedDecisionWithChangedTransactionState(t *testing.T) {
	for _, change := range []string{"scope", "invalidate", "supersede", "cancel"} {
		t.Run(change, func(t *testing.T) {
			r, err := NewRegistry([]byte("old"))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			a, err := r.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			c, err := a.Stage(page(`[{"name":"lookup","inputSchema":{"type":"object"}}]`, ""), "SUCCESS")
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "scope":
				err = r.ChangeScope()
			case "invalidate":
				err = r.Invalidate()
			case "supersede":
				_, err = r.Begin(context.Background())
			case "cancel":
				cancel()
			}
			if err != nil {
				t.Fatal(err)
			}
			receipt := r.Commit(c)
			if receipt.State != "RETAINED" || string(r.Snapshot().Bytes) != "old" {
				t.Fatalf("changed state committed: %+v", receipt)
			}
			if change != "cancel" {
				want := map[string]string{"scope": "SCOPE_CHANGED", "invalidate": "INVALIDATED", "supersede": "SUPERSEDED_ATTEMPT"}[change]
				if receipt.Decision == nil || receipt.Decision.Reason != want {
					t.Fatalf("wrong Gooo refusal: %+v", receipt)
				}
			}
		})
	}
}

// These interleavings keep the actual generated Gooo evaluator. The hook only
// changes host state after evaluation and before the final commit comparison.
func TestActualGoooDecisionDoesNotBypassFinalComparison(t *testing.T) {
	for _, change := range []string{"scope", "invalidate", "supersede", "cancel", "newer-commit"} {
		t.Run(change, func(t *testing.T) {
			var r *host.Registry
			var slowAttempt string
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			evaluator := func(f host.SnapshotFacts, e host.RefreshEvidence, s host.RefreshState) (host.RefreshDecision, error) {
				decision, err := evaluateGooo(f, e, s)
				if err != nil {
					return decision, err
				}
				if e.AttemptID != slowAttempt {
					return decision, nil
				}
				switch change {
				case "scope":
					err = r.ChangeScope()
				case "invalidate":
					err = r.Invalidate()
				case "supersede":
					_, err = r.Begin(context.Background())
				case "cancel":
					cancel()
				case "newer-commit":
					newer, beginErr := r.Begin(context.Background())
					if beginErr != nil {
						return decision, beginErr
					}
					applied := r.Refresh(newer, page(`[{"name":"newer","inputSchema":{"type":"object"}}]`, ""), "SUCCESS")
					if applied.State != "APPLIED" {
						t.Fatalf("newer refresh did not commit: %+v", applied)
					}
				}
				return decision, err
			}
			var err error
			r, err = host.NewRegistry([]byte("old"), evaluator)
			if err != nil {
				t.Fatal(err)
			}
			a, err := r.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			slowAttempt = a.ID()
			result := r.Refresh(a, page(`[{"name":"older","inputSchema":{"type":"object"}}]`, ""), "SUCCESS")
			if result.Decision == nil || result.Decision.State != "CANDIDATE_ELIGIBLE" {
				t.Fatalf("expected actual eligible precheck: %+v", result)
			}
			reasons := map[string]string{"scope": "SCOPE_CHANGED_AT_COMMIT", "invalidate": "INVALIDATED_AT_COMMIT", "supersede": "SUPERSEDED_AT_COMMIT", "cancel": "CANCELLED_AT_COMMIT", "newer-commit": "REGISTRY_CHANGED_AT_COMMIT"}
			if result.State != "RETAINED" || result.Reason != reasons[change] || result.BeforeVersion != result.AfterVersion || result.BeforeSHA256 != result.AfterSHA256 {
				t.Fatalf("stale decision bypassed commit comparison: %+v", result)
			}
			if change == "newer-commit" {
				if !bytes.Contains(r.Snapshot().Bytes, []byte("newer")) || bytes.Contains(r.Snapshot().Bytes, []byte("older")) {
					t.Fatal("older refresh overwrote newer bytes")
				}
			} else if string(r.Snapshot().Bytes) != "old" {
				t.Fatal("rejected candidate replaced old storage")
			}
		})
	}
}

func TestActualGoooCommitsEntireTwoPageCandidateOnce(t *testing.T) {
	r, err := NewRegistry([]byte("old"))
	if err != nil {
		t.Fatal(err)
	}
	a, err := r.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	pages := page(`[{"name":"first","inputSchema":{"type":"object"}}]`, "next")
	pages = append(pages, SnapshotPage{RequestJSON: `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{"cursor":"next"}}`, ResponseJSON: `{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"second","inputSchema":{"type":"object"}}]}}`})
	c, err := a.Stage(pages, "SUCCESS")
	if err != nil {
		t.Fatal(err)
	}
	first := r.Commit(c)
	if first.State != "APPLIED" || first.Decision == nil || first.Decision.CandidatePageCount != 2 || first.Decision.CandidateRowCount != 2 || !bytes.Equal(r.Snapshot().Bytes, c.Bytes()) {
		t.Fatalf("complete multi-page catalog was not committed: %+v", first)
	}
	applied := r.Snapshot()
	again := r.Commit(c)
	if again.State != "RETAINED" || again.Decision == nil || again.Decision.Reason != "REGISTRY_CHANGED" || r.Snapshot().RegistryVersion != applied.RegistryVersion || !bytes.Equal(r.Snapshot().Bytes, applied.Bytes) {
		t.Fatalf("same candidate committed twice: %+v", again)
	}
}
