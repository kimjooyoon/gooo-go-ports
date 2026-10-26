// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
package catalogrefresh

import (
	"bytes"
	"context"
	"errors"
	"math"
	"sync"
	"testing"
)

// mechanicsEligible is deliberately NOT a Gooo policy oracle. These tests use
// an eligible precheck to exercise the actual transaction races; the generated
// Gooo activity and its independent corpus must be verified separately.
func mechanicsEligible(f SnapshotFacts, e RefreshEvidence, s RefreshState) (RefreshDecision, error) {
	return RefreshDecision{State: "CANDIDATE_ELIGIBLE", Reason: "MECHANICS_TEST_ONLY",
		CandidateID: e.CandidateID, AttemptID: e.AttemptID,
		ExpectedRegistryVersion: s.ExpectedRegistryVersion, ExpectedScopeEpoch: s.ExpectedScopeEpoch,
		ExpectedInvalidationEpoch: s.ExpectedInvalidationEpoch, ExpectedLatestAttempt: s.ExpectedLatestAttempt,
		CandidatePageCount: int64(len(f.PageRequestCursorKinds)), CandidateRowCount: int64(len(f.OriginalNames)),
		Coverage: "COMPLETE", CoverageReason: "MECHANICS_TEST_ONLY", ProblemPageIndex: -1, ProblemRowIndex: -1,
		DecisionScope: "WHOLE_CANDIDATE_LOCAL_REPLACEMENT_PRECHECK", RemoteAtomicity: "UNKNOWN"}, nil
}

func syntheticPage(name string) SnapshotPage {
	tools := ""
	if name != "" {
		tools = `{"name":"` + name + `","inputSchema":{"type":"object"}}`
	}
	return SnapshotPage{RequestJSON: `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
		ResponseJSON: `{"jsonrpc":"2.0","id":1,"result":{"tools":[` + tools + `]}}`}
}

func newMechanicsRegistry(t *testing.T, e Evaluator) *Registry {
	t.Helper()
	r, err := NewRegistry([]byte("previous catalog bytes"), e)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func stageMechanics(t *testing.T, r *Registry, ctx context.Context, name string) (*Attempt, *Candidate) {
	t.Helper()
	a, err := r.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	c, err := a.Stage([]SnapshotPage{syntheticPage(name)}, "SUCCESS")
	if err != nil {
		t.Fatal(err)
	}
	return a, c
}

func requireUnchanged(t *testing.T, before StoredCatalog, after StoredCatalog) {
	t.Helper()
	if !bytes.Equal(before.Bytes, after.Bytes) || before.SHA256 != after.SHA256 || before.RegistryVersion != after.RegistryVersion {
		t.Fatalf("stored bytes/version changed: before=%+v after=%+v", before, after)
	}
}

func requireRetained(t *testing.T, receipt Receipt, reason string) {
	t.Helper()
	if receipt.State != "RETAINED" || receipt.Reason != reason || receipt.BeforeVersion != receipt.AfterVersion || receipt.BeforeSHA256 != receipt.AfterSHA256 {
		t.Fatalf("not retained with %s: %+v", reason, receipt)
	}
}

func TestMechanicsNewerBCommitsWhileEligibleAWaits(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var first sync.Once
	r := newMechanicsRegistry(t, func(f SnapshotFacts, e RefreshEvidence, s RefreshState) (RefreshDecision, error) {
		if f.OriginalNames[0] == "A" {
			first.Do(func() { close(entered) })
			<-release
		}
		return mechanicsEligible(f, e, s)
	})
	_, a := stageMechanics(t, r, context.Background(), "A")
	result := make(chan Receipt, 1)
	go func() { result <- r.Commit(a) }()
	<-entered
	_, b := stageMechanics(t, r, context.Background(), "B")
	if got := r.Commit(b); got.State != "APPLIED" {
		t.Fatalf("B did not apply: %+v", got)
	}
	committedB := r.Snapshot()
	close(release)
	got := <-result
	requireRetained(t, got, "REGISTRY_CHANGED_AT_COMMIT")
	if got.Decision == nil || got.Decision.State != "CANDIDATE_ELIGIBLE" {
		t.Fatal("lost precheck/actual-outcome distinction")
	}
	requireUnchanged(t, committedB, r.Snapshot())
	if len(r.Receipts()) != 2 || r.Receipts()[0].State != "APPLIED" || r.Receipts()[1].State != "RETAINED" {
		t.Fatal("journal does not match actual commit order")
	}
}

func TestMechanicsNotificationCancellationAndScopeBetweenPrecheckAndCAS(t *testing.T) {
	for _, change := range []string{"notification", "cancel", "scope", "cancel-and-notification"} {
		t.Run(change, func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			r := newMechanicsRegistry(t, func(f SnapshotFacts, e RefreshEvidence, s RefreshState) (RefreshDecision, error) {
				decision, err := mechanicsEligible(f, e, s)
				close(entered)
				<-release
				return decision, err
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			_, candidate := stageMechanics(t, r, ctx, "candidate")
			before := r.Snapshot()
			result := make(chan Receipt, 1)
			go func() { result <- r.Commit(candidate) }()
			<-entered
			want := ""
			switch change {
			case "notification":
				if err := r.Invalidate(); err != nil {
					t.Fatal(err)
				}
				want = "INVALIDATED_AT_COMMIT"
			case "scope":
				if err := r.ChangeScope(); err != nil {
					t.Fatal(err)
				}
				want = "SCOPE_CHANGED_AT_COMMIT"
			case "cancel":
				cancel()
				want = "CANCELLED_AT_COMMIT"
			case "cancel-and-notification":
				cancel()
				if err := r.Invalidate(); err != nil {
					t.Fatal(err)
				}
				want = "CANCELLED_AT_COMMIT"
			}
			close(release)
			got := <-result
			requireRetained(t, got, want)
			if got.Decision == nil || got.Decision.State != "CANDIDATE_ELIGIBLE" {
				t.Fatal("expected recorded eligible precheck")
			}
			requireUnchanged(t, before, r.Snapshot())
		})
	}
}

func TestMechanicsFailedBSupersedesA(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	r := newMechanicsRegistry(t, func(f SnapshotFacts, e RefreshEvidence, s RefreshState) (RefreshDecision, error) {
		decision, err := mechanicsEligible(f, e, s)
		if f.OriginalNames[0] == "A" {
			close(entered)
			<-release
		} else {
			// Scripted refusal, not a reimplementation of collector policy.
			decision.State, decision.Reason = "RETAIN_STORED_CATALOG", "SCRIPTED_B_FAILURE"
		}
		return decision, err
	})
	_, a := stageMechanics(t, r, context.Background(), "A")
	before := r.Snapshot()
	result := make(chan Receipt, 1)
	go func() { result <- r.Commit(a) }()
	<-entered
	_, b := stageMechanics(t, r, context.Background(), "B")
	requireRetained(t, r.Commit(b), "GOOO_RETAINED")
	close(release)
	requireRetained(t, <-result, "SUPERSEDED_AT_COMMIT")
	requireUnchanged(t, before, r.Snapshot())
}

func TestMechanicsMalformedBSupersedesA(t *testing.T) {
	r := newMechanicsRegistry(t, mechanicsEligible)
	_, a := stageMechanics(t, r, context.Background(), "A")
	b, err := r.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	before := r.Snapshot()
	requireRetained(t, r.Refresh(b, []SnapshotPage{{RequestJSON: "{", ResponseJSON: "{"}}, "SUCCESS"), "STAGING_FAILED")
	requireRetained(t, r.Commit(a), "SUPERSEDED_AT_COMMIT")
	requireUnchanged(t, before, r.Snapshot())
}

func TestMechanicsStagingAndObservationsAreImmutableCopies(t *testing.T) {
	r := newMechanicsRegistry(t, func(f SnapshotFacts, e RefreshEvidence, s RefreshState) (RefreshDecision, error) {
		decision, err := mechanicsEligible(f, e, s)
		f.OriginalNames[0], e.RowDefinitionIDs[0] = "adapter mutation", "adapter mutation"
		return decision, err
	})
	a, err := r.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	pages := []SnapshotPage{syntheticPage("immutable")}
	candidate, err := a.Stage(pages, "SUCCESS")
	if err != nil {
		t.Fatal(err)
	}
	want, wantID := candidate.Bytes(), candidate.ID()
	pages[0] = syntheticPage("caller mutation")
	facts, evidence := candidate.Inputs()
	facts.OriginalNames[0], evidence.RowDefinitionIDs[0] = "changed", "changed"
	view, envelope := candidate.Bytes(), candidate.Envelope()
	view[0], envelope[0] = '!', '!'
	got := r.Commit(candidate)
	if got.State != "APPLIED" || got.CandidateID != wantID || !bytes.Equal(r.Snapshot().Bytes, want) {
		t.Fatalf("mutation changed stage or commit: %+v", got)
	}
	snapshot := r.Snapshot()
	snapshot.Bytes[0] = '!'
	receipts := r.Receipts()
	receipts[0].Decision.State = "changed"
	got.Decision.State = "changed"
	if !bytes.Equal(r.Snapshot().Bytes, want) || r.Receipts()[0].Decision.State != "CANDIDATE_ELIGIBLE" {
		t.Fatal("observation mutated owned state")
	}
}

func TestMechanicsEmptyCompleteCandidateAndOneCommitOnly(t *testing.T) {
	r := newMechanicsRegistry(t, mechanicsEligible)
	_, candidate := stageMechanics(t, r, context.Background(), "")
	before := r.Snapshot()
	if got := r.Commit(candidate); got.State != "APPLIED" {
		t.Fatalf("empty candidate not applied: %+v", got)
	}
	after := r.Snapshot()
	if bytes.Equal(before.Bytes, after.Bytes) || after.RegistryVersion == before.RegistryVersion || !bytes.Equal(after.Bytes, candidate.Bytes()) {
		t.Fatal("empty replacement was not stored")
	}
	requireRetained(t, r.Commit(candidate), "REGISTRY_CHANGED_AT_COMMIT")
	requireUnchanged(t, after, r.Snapshot())
}

func TestMechanicsIdenticalBytesCannotABATheVersionOrEpoch(t *testing.T) {
	r := newMechanicsRegistry(t, mechanicsEligible)
	_, first := stageMechanics(t, r, context.Background(), "same")
	if r.Commit(first).State != "APPLIED" {
		t.Fatal("first apply failed")
	}
	one := r.Snapshot()
	_, stale := stageMechanics(t, r, context.Background(), "same")
	_, later := stageMechanics(t, r, context.Background(), "same")
	if r.Commit(later).State != "APPLIED" {
		t.Fatal("same-data apply failed")
	}
	two := r.Snapshot()
	if !bytes.Equal(one.Bytes, two.Bytes) || one.RegistryVersion == two.RegistryVersion {
		t.Fatal("identical data reused a generation")
	}
	requireRetained(t, r.Commit(stale), "REGISTRY_CHANGED_AT_COMMIT")
	seen := map[string]bool{two.ScopeEpoch: true, two.InvalidationEpoch: true, two.RegistryVersion: true, two.LatestAttempt: true}
	for range 3 {
		if err := r.ChangeScope(); err != nil {
			t.Fatal(err)
		}
		scope := r.Snapshot().ScopeEpoch
		if seen[scope] {
			t.Fatal("scope reused")
		}
		seen[scope] = true
		if err := r.Invalidate(); err != nil {
			t.Fatal(err)
		}
		invalidation := r.Snapshot().InvalidationEpoch
		if seen[invalidation] {
			t.Fatal("invalidation reused")
		}
		seen[invalidation] = true
	}
}

func TestMechanicsTokenExhaustionNeverWraps(t *testing.T) {
	r := newMechanicsRegistry(t, mechanicsEligible)
	_, candidate := stageMechanics(t, r, context.Background(), "A")
	r.mu.Lock()
	r.next = math.MaxUint64
	r.mu.Unlock()
	before := r.Snapshot()
	if _, err := r.Begin(context.Background()); err == nil {
		t.Fatal("exhausted begin succeeded")
	}
	if r.Invalidate() == nil || r.ChangeScope() == nil {
		t.Fatal("exhausted epoch update succeeded")
	}
	requireRetained(t, r.Commit(candidate), "TOKEN_EXHAUSTED")
	after := r.Snapshot()
	requireUnchanged(t, before, after)
	if before.ScopeEpoch != after.ScopeEpoch || before.InvalidationEpoch != after.InvalidationEpoch || before.LatestAttempt != after.LatestAttempt {
		t.Fatal("exhaustion changed tokens")
	}
}

func TestMechanicsAdapterFailuresAndAuthorityMismatchRetain(t *testing.T) {
	for _, mode := range []string{"error", "panic", "identity", "count", "authority", "foreign", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			r := newMechanicsRegistry(t, func(f SnapshotFacts, e RefreshEvidence, s RefreshState) (RefreshDecision, error) {
				if mode == "error" {
					return RefreshDecision{}, errors.New("scripted failure")
				}
				if mode == "panic" {
					panic("scripted panic")
				}
				d, err := mechanicsEligible(f, e, s)
				if mode == "identity" {
					d.CandidateID = "wrong"
				}
				if mode == "count" {
					d.CandidateRowCount++
				}
				if mode == "authority" {
					d.ExecutionAuthorized = true
				}
				return d, err
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			_, candidate := stageMechanics(t, r, ctx, "A")
			want := "DECISION_BINDING_MISMATCH"
			if mode == "error" || mode == "panic" {
				want = "EVALUATOR_FAILED"
			}
			if mode == "foreign" {
				other := newMechanicsRegistry(t, mechanicsEligible)
				_, candidate = stageMechanics(t, other, ctx, "A")
				want = "INVALID_CANDIDATE"
			}
			if mode == "cancelled" {
				cancel()
				want = "CANCELLED_BEFORE_EVALUATION"
			}
			before := r.Snapshot()
			requireRetained(t, r.Commit(candidate), want)
			requireUnchanged(t, before, r.Snapshot())
		})
	}
}
