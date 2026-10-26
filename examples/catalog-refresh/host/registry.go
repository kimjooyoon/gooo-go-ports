// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
package catalogrefresh

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strconv"
	"sync"
)

type tuple struct{ version, scope, invalidation, latest string }

// Registry owns its tokens and data. No API accepts a replacement version or
// epoch from a caller. A counter never wraps; exhaustion refuses further changes.
// Receipts and the pointer swap are recorded under the same mutex.
type Registry struct {
	mu              sync.Mutex
	namespace       string
	next            uint64
	state           tuple
	catalog         []byte
	evaluate        Evaluator
	receipts        []Receipt
	receiptSequence uint64
	droppedReceipts uint64
}

// Attempt captures the registry state and original context once. Its fields
// cannot be rewritten by a collector or by the generated evaluation adapter.
type Attempt struct {
	registry *Registry
	ctx      context.Context
	id       string
	expected tuple
}

func (a *Attempt) ID() string { return a.id }

func NewRegistry(initial []byte, evaluator Evaluator) (*Registry, error) {
	if evaluator == nil {
		return nil, errors.New("generated Gooo evaluator is required")
	}
	if len(initial) > 1<<20 {
		return nil, errors.New("initial stored bytes exceed limit")
	}
	var namespace [16]byte
	if _, err := rand.Read(namespace[:]); err != nil {
		return nil, err
	}
	r := &Registry{namespace: hex.EncodeToString(namespace[:]), catalog: bytes.Clone(initial), evaluate: evaluator}
	// A newly constructed counter cannot exhaust during these four allocations.
	r.state.version, _ = r.fresh("version")
	r.state.scope, _ = r.fresh("scope")
	r.state.invalidation, _ = r.fresh("invalidation")
	r.state.latest, _ = r.fresh("no-attempt")
	return r, nil
}

func (r *Registry) fresh(kind string) (string, error) {
	if r.next == math.MaxUint64 {
		return "", errors.New("token counter exhausted")
	}
	r.next++
	return r.namespace + "/" + kind + "/" + strconv.FormatUint(r.next, 10), nil
}

func (r *Registry) Begin(ctx context.Context) (*Attempt, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	id, err := r.fresh("attempt")
	if err != nil {
		return nil, err
	}
	// Beginning B supersedes A even when B later fails to collect or project.
	r.state.latest = id
	return &Attempt{registry: r, ctx: ctx, id: id, expected: r.state}, nil
}

// Invalidate records a relevant change notification, including one that leaves
// the stored bytes untouched. The stored catalog is diagnostic data, not proof
// that previously exposed tools are still authorized.
func (r *Registry) Invalidate() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	token, err := r.fresh("invalidation")
	if err != nil {
		return err
	}
	r.state.invalidation = token
	return nil
}

// ChangeScope records reconnect/auth/configuration scope change even when its
// human-readable label or catalog bytes happen to match an earlier state.
func (r *Registry) ChangeScope() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	token, err := r.fresh("scope")
	if err != nil {
		return err
	}
	r.state.scope = token
	return nil
}

func (r *Registry) Snapshot() StoredCatalog {
	r.mu.Lock()
	defer r.mu.Unlock()
	return StoredCatalog{Bytes: bytes.Clone(r.catalog), SHA256: digest(r.catalog),
		RegistryVersion: r.state.version, ScopeEpoch: r.state.scope,
		InvalidationEpoch: r.state.invalidation, LatestAttempt: r.state.latest}
}

func (r *Registry) Receipts() []Receipt {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]Receipt, len(r.receipts))
	for i, receipt := range r.receipts {
		result[i] = cloneReceipt(receipt)
	}
	return result
}

// DroppedReceipts counts older journal entries no longer in the bounded recent
// history. This in-memory demonstration is not a durable audit log.
func (r *Registry) DroppedReceipts() uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.droppedReceipts
}

// Refresh includes staging failures in the actual outcome journal. Stage/Commit
// are also public to demonstrate a pause between collection and replacement.
func (r *Registry) Refresh(a *Attempt, pages []SnapshotPage, collectorState string) Receipt {
	if a == nil || a.registry != r {
		return r.reject(a, nil, "INVALID_ATTEMPT", nil)
	}
	candidate, err := a.Stage(pages, collectorState)
	if err != nil {
		return r.reject(a, nil, "STAGING_FAILED", err)
	}
	return r.Commit(candidate)
}

func (r *Registry) reject(a *Attempt, c *Candidate, reason string, err error) Receipt {
	r.mu.Lock()
	defer r.mu.Unlock()
	receipt := r.newReceipt(a, c)
	receipt.Reason = reason
	if err != nil {
		receipt.Error = err.Error()
	}
	return r.record(receipt)
}

// Commit asks the injected Gooo activity about copied inputs outside the lock,
// then checks identity, cancellation and the full tuple under the same lock as
// the swap. A cancellation already visible at that final check refuses the swap;
// cancellation concurrent with/after the commit linearization can occur later.
func (r *Registry) Commit(c *Candidate) Receipt {
	if c == nil || c.attempt == nil || c.attempt.registry != r {
		return r.reject(nil, c, "INVALID_CANDIDATE", nil)
	}
	a := c.attempt
	r.mu.Lock()
	evaluationState := stateBetween(a.expected, r.state)
	r.mu.Unlock()
	if err := a.ctx.Err(); err != nil {
		return r.reject(a, c, "CANCELLED_BEFORE_EVALUATION", err)
	}
	facts, evidence := c.Inputs()
	decision, evaluationError := evaluateSafely(r.evaluate, facts, evidence, evaluationState)

	r.mu.Lock()
	defer r.mu.Unlock()
	receipt := r.newReceipt(a, c)
	receipt.EvaluationState = evaluationState
	if evaluationError == nil {
		receipt.Decision = &decision
	}
	switch {
	case c.id == "" || c.id != digest(c.envelope) || c.id != c.evidence.CandidateID ||
		c.evidence.AttemptID != a.id || a.expected.latest != a.id:
		receipt.Reason = "CANDIDATE_BINDING_MISMATCH"
	case a.ctx.Err() != nil:
		receipt.Reason = "CANCELLED_AT_COMMIT"
		receipt.Error = a.ctx.Err().Error()
	case evaluationError != nil:
		receipt.Reason = "EVALUATOR_FAILED"
		receipt.Error = evaluationError.Error()
	case !decisionBound(decision, c):
		receipt.Reason = "DECISION_BINDING_MISMATCH"
	case decision.State != "CANDIDATE_ELIGIBLE":
		receipt.Reason = "GOOO_RETAINED"
	case r.state.version != a.expected.version:
		receipt.Reason = "REGISTRY_CHANGED_AT_COMMIT"
	case r.state.scope != a.expected.scope:
		receipt.Reason = "SCOPE_CHANGED_AT_COMMIT"
	case r.state.invalidation != a.expected.invalidation:
		receipt.Reason = "INVALIDATED_AT_COMMIT"
	case r.state.latest != a.expected.latest:
		receipt.Reason = "SUPERSEDED_AT_COMMIT"
	default:
		version, err := r.fresh("version")
		if err != nil {
			receipt.Reason, receipt.Error = "TOKEN_EXHAUSTED", err.Error()
			break
		}
		r.catalog = bytes.Clone(c.catalog)
		r.state.version = version
		receipt.State, receipt.Reason = "APPLIED", "LOCAL_CATALOG_REPLACED"
		receipt.AfterVersion, receipt.AfterSHA256 = version, digest(r.catalog)
	}
	return r.record(receipt)
}

func evaluateSafely(e Evaluator, f SnapshotFacts, evidence RefreshEvidence, state RefreshState) (decision RefreshDecision, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("evaluator panic: %v", recovered)
		}
	}()
	return e(f, evidence, state)
}

// This checks the adapter's output binding/declared authority, not candidate
// eligibility policy. Gooo is responsible for interpreting all candidate facts.
func decisionBound(d RefreshDecision, c *Candidate) bool {
	a := c.attempt
	return (d.State == "CANDIDATE_ELIGIBLE" || d.State == "RETAIN_STORED_CATALOG") &&
		d.Reason != "" && d.CandidateID == c.id && d.AttemptID == a.id &&
		d.CandidatePageCount == int64(len(c.facts.PageRequestCursorKinds)) &&
		d.CandidateRowCount == int64(len(c.facts.OriginalNames)) &&
		d.ExpectedRegistryVersion == a.expected.version && d.ExpectedScopeEpoch == a.expected.scope &&
		d.ExpectedInvalidationEpoch == a.expected.invalidation && d.ExpectedLatestAttempt == a.expected.latest &&
		d.DecisionScope == "WHOLE_CANDIDATE_LOCAL_REPLACEMENT_PRECHECK" &&
		!d.ExecutionAuthorized && !d.LiveEffectPerformed && d.RemoteAtomicity == "UNKNOWN"
}

func stateBetween(expected, current tuple) RefreshState {
	return RefreshState{ExpectedRegistryVersion: expected.version, CurrentRegistryVersion: current.version,
		ExpectedScopeEpoch: expected.scope, CurrentScopeEpoch: current.scope,
		ExpectedInvalidationEpoch: expected.invalidation, CurrentInvalidationEpoch: current.invalidation,
		ExpectedLatestAttempt: expected.latest, CurrentLatestAttempt: current.latest}
}

// newReceipt and record require r.mu. Before/after refer to storage at the actual
// commit attempt, which may already contain a newer successful refresh.
func (r *Registry) newReceipt(a *Attempt, c *Candidate) Receipt {
	receipt := Receipt{State: "RETAINED", BeforeVersion: r.state.version, AfterVersion: r.state.version,
		BeforeSHA256: digest(r.catalog), AfterSHA256: digest(r.catalog)}
	if a != nil && a.registry == r {
		receipt.AttemptID, receipt.CommitState = a.id, stateBetween(a.expected, r.state)
	}
	if c != nil {
		receipt.CandidateID = c.id
	}
	return receipt
}

func (r *Registry) record(receipt Receipt) Receipt {
	r.receiptSequence++
	receipt.Sequence = r.receiptSequence
	if len(r.receipts) == 256 {
		copy(r.receipts, r.receipts[1:])
		r.receipts = r.receipts[:255]
		r.droppedReceipts++
	}
	r.receipts = append(r.receipts, cloneReceipt(receipt))
	return cloneReceipt(receipt)
}

func cloneReceipt(receipt Receipt) Receipt {
	if receipt.Decision != nil {
		decision := *receipt.Decision
		receipt.Decision = &decision
	}
	return receipt
}
