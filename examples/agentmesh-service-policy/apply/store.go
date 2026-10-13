package apply

import (
	"crypto/rand"
	"fmt"
	"math"
	"strings"
	"sync"
	"unicode/utf8"
)

// Owner belongs only to the trusted embedding adapter. WriterForAuthenticated
// accepts an identity already authenticated by that adapter; it is not an
// authentication protocol and must not receive an untrusted claimed identity.
type Owner struct{ store *memoryStore }

// Writer binds one authenticated subject to one private store. Current
// authorization is rechecked under the store lock for every operation.
type Writer struct {
	store   *memoryStore
	subject string
}

type memoryStore struct {
	mu    sync.Mutex
	state *committedState
	// The default is the pinned generated evaluator. Package-private tests may
	// inject errors or contradictory records; no public API accepts a callback.
	eval func(Facts) (evaluationProof, error)
}

// Both this struct and its operation map are immutable after publication.
// Every receipt append allocates a new map; queries return only value structs.
type committedState struct {
	snapshot      Snapshot
	allowedWriter string
	operations    map[string]operationRecord
}

type operationRecord struct {
	subject  string
	proposal Proposal
	result   ApplyResult
}

func NewInMemoryOwner(target string, initial Config) (*Owner, error) {
	return newInMemoryOwner(target, initial, sourceGeneration)
}

// The source reader is private so tests can exercise binding failures without
// changing package globals or allowing callers to supply trusted identities.
func newInMemoryOwner(target string, initial Config, readGeneration func() (string, error)) (*Owner, error) {
	if err := boundedText("target", target, MaxIdentityBytes, true); err != nil {
		return nil, err
	}
	if err := validateConfig(initial); err != nil {
		return nil, err
	}
	target = strings.Clone(target)
	initial = ownedConfig(initial)
	if readGeneration == nil {
		return nil, ErrInvalidGeneration
	}
	generation, err := readGeneration()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidGeneration, err)
	}
	if !validGeneration(generation) {
		return nil, ErrInvalidGeneration
	}
	generation = strings.Clone(generation)
	var epoch Epoch
	if _, err := rand.Read(epoch[:]); err != nil {
		return nil, fmt.Errorf("create store epoch: %w", err)
	}
	if epoch == (Epoch{}) {
		return nil, fmt.Errorf("create store epoch: empty random epoch")
	}
	snapshot := Snapshot{
		Epoch: epoch, Target: target, Revision: 1, Pattern: initial.Pattern,
		RequiredType: initial.RequiredType, RequiredName: initial.RequiredName,
		GenerationSHA256: generation,
	}
	return &Owner{store: &memoryStore{
		state: &committedState{snapshot: snapshot, allowedWriter: initial.AllowedWriter,
			operations: make(map[string]operationRecord)},
		eval: evaluate,
	}}, nil
}

func (o *Owner) WriterForAuthenticated(subject string) (*Writer, error) {
	if o == nil || o.store == nil {
		return nil, ErrInvalidHandle
	}
	if err := boundedText("authenticated subject", subject, MaxIdentityBytes, true); err != nil {
		return nil, err
	}
	s := o.store
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == nil {
		return nil, ErrInvalidHandle
	}
	if subject != s.state.allowedWriter {
		return nil, ErrUnauthorized
	}
	return &Writer{store: s, subject: strings.Clone(subject)}, nil
}

// Update is the explicit owner-only administrative path. Even an equal-byte
// update advances the same revision, covering configuration, authorization and
// grant ABA. It never erases an operation record from this store incarnation.
func (o *Owner) Update(expectedRevision uint64, next Config) (Snapshot, error) {
	if o == nil || o.store == nil {
		return Snapshot{}, ErrInvalidHandle
	}
	if err := validateConfig(next); err != nil {
		return Snapshot{}, err
	}
	s := o.store
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == nil {
		return Snapshot{}, ErrInvalidHandle
	}
	current := s.state
	if expectedRevision != current.snapshot.Revision {
		return Snapshot{}, ErrRevisionConflict
	}
	if current.snapshot.Revision == math.MaxUint64 {
		return Snapshot{}, ErrRevisionExhausted
	}
	next = ownedConfig(next)
	snapshot := current.snapshot
	snapshot.Revision++
	snapshot.Pattern = next.Pattern
	snapshot.RequiredType = next.RequiredType
	snapshot.RequiredName = next.RequiredName
	nextState := &committedState{snapshot: snapshot, allowedWriter: next.AllowedWriter,
		operations: current.operations}
	s.state = nextState
	return snapshot, nil
}

func (w *Writer) Snapshot() (Snapshot, error) {
	if w == nil || w.store == nil {
		return Snapshot{}, ErrInvalidHandle
	}
	s := w.store
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := w.authorizedLocked(); err != nil {
		return Snapshot{}, err
	}
	return s.state.snapshot, nil
}

// Apply holds one lock across current authorization, state selection, generated
// Gooo evaluation and the one publication point. Ordinary returned errors occur
// before publication. This does not promise rollback after transport failure,
// process loss or panic; callers must resolve the same operation identity.
func (w *Writer) Apply(p Proposal) (ApplyResult, error) {
	if w == nil || w.store == nil {
		return ApplyResult{}, ErrInvalidHandle
	}
	s := w.store
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := w.authorizedLocked(); err != nil {
		return ApplyResult{}, err
	}
	if err := validateProposal(p); err != nil {
		return ApplyResult{}, err
	}
	p = ownedProposal(p)
	current := s.state
	if p.Epoch != current.snapshot.Epoch {
		return w.result(p, current.snapshot, "UNKNOWN_OUTCOME", "DIFFERENT_EPOCH"), nil
	}
	if old, ok := current.operations[p.OperationID]; ok {
		if old.subject != w.subject || old.proposal != p {
			return w.result(p, current.snapshot, "CONFLICT", "OPERATION_ID_REUSED"), nil
		}
		result := old.result
		result.Replay = true
		return result, nil
	}
	if len(current.operations) >= MaxOperations {
		return w.result(p, current.snapshot, "REFUSED", "LEDGER_CAPACITY"), nil
	}
	var conflict string
	switch {
	case p.Target != current.snapshot.Target:
		conflict = "TARGET_MISMATCH"
	case p.GenerationSHA256 != current.snapshot.GenerationSHA256:
		conflict = "GENERATION_MISMATCH"
	case p.ExpectedRevision != current.snapshot.Revision:
		conflict = "STALE_REVISION"
	}
	if conflict != "" {
		result := w.result(p, current.snapshot, "CONFLICT", conflict)
		return s.publishLocked(current, current.snapshot, w.subject, p, result), nil
	}
	if current.snapshot.Revision == math.MaxUint64 {
		result := w.result(p, current.snapshot, "REFUSED", "REVISION_EXHAUSTED")
		return s.publishLocked(current, current.snapshot, w.subject, p, result), nil
	}
	if s.eval == nil {
		return ApplyResult{}, ErrInvalidHandle
	}
	facts := Facts{Before: current.snapshot.Pattern, After: p.Candidate,
		RequiredType: current.snapshot.RequiredType, RequiredName: current.snapshot.RequiredName}
	proof, err := s.eval(facts)
	if err != nil {
		return ApplyResult{}, err
	}
	if proof.Facts != facts || proof.Bindings.GenerationSHA256 != current.snapshot.GenerationSHA256 {
		return ApplyResult{}, ErrInvalidGeneration
	}
	decision := proof.Decision
	if err := decision.Validate(); err != nil {
		return ApplyResult{}, err
	}
	result := w.result(p, current.snapshot, "REFUSED", decision.Reason)
	result.HasDecision = true
	result.Decision = decision
	if decision.Status != "ELIGIBLE" {
		return s.publishLocked(current, current.snapshot, w.subject, p, result), nil
	}
	nextSnapshot := current.snapshot
	nextSnapshot.Pattern = p.Candidate
	nextSnapshot.Revision++
	result.Status = "APPLIED"
	result.Reason = "APPLIED"
	result.CommittedRevision = nextSnapshot.Revision
	return s.publishLocked(current, nextSnapshot, w.subject, p, result), nil
}

// Resolve is a lookup, never a replay evaluator, retry, rebase or write.
func (w *Writer) Resolve(p Proposal) (Resolution, error) {
	if w == nil || w.store == nil {
		return Resolution{}, ErrInvalidHandle
	}
	s := w.store
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := w.authorizedLocked(); err != nil {
		return Resolution{}, err
	}
	if err := validateProposal(p); err != nil {
		return Resolution{}, err
	}
	p = ownedProposal(p)
	current := s.state
	resolution := Resolution{Schema: ResolutionSchema, Subject: w.subject,
		Proposal: p, Observed: current.snapshot}
	if p.Epoch != current.snapshot.Epoch {
		resolution.Status, resolution.Reason = "UNKNOWN_OUTCOME", "DIFFERENT_EPOCH"
		return resolution, nil
	}
	old, ok := current.operations[p.OperationID]
	if !ok {
		resolution.Status, resolution.Reason = "NOT_OBSERVED_AT_THIS_LOOKUP", "OPERATION_NOT_RECORDED"
		return resolution, nil
	}
	if old.subject != w.subject || old.proposal != p {
		resolution.Status, resolution.Reason = "CONFLICT", "OPERATION_ID_REUSED"
		return resolution, nil
	}
	resolution.Status, resolution.Reason = "KNOWN", "OPERATION_RECORDED"
	resolution.HasResult = true
	resolution.Result = old.result
	resolution.Result.Replay = true
	return resolution, nil
}

func (w *Writer) authorizedLocked() error {
	if w.store.state == nil {
		return ErrInvalidHandle
	}
	if w.subject == "" || w.subject != w.store.state.allowedWriter {
		return ErrUnauthorized
	}
	return nil
}

func (w *Writer) result(p Proposal, observed Snapshot, status, reason string) ApplyResult {
	return ApplyResult{Schema: ApplySchema, Status: status, Reason: reason,
		Subject: w.subject, Proposal: p, Observed: observed}
}

// The caller holds mu and has established an unused ID and available capacity.
// The old map remains immutable even on an allocation failure before publication.
func (s *memoryStore) publishLocked(current *committedState, next Snapshot, subject string, p Proposal, result ApplyResult) ApplyResult {
	result.Recorded = true
	operations := make(map[string]operationRecord, len(current.operations)+1)
	for id, record := range current.operations {
		operations[id] = record
	}
	operations[p.OperationID] = operationRecord{subject: subject, proposal: p, result: result}
	nextState := &committedState{snapshot: next, allowedWriter: current.allowedWriter, operations: operations}
	s.state = nextState
	return result
}

func validateProposal(p Proposal) error {
	for _, item := range []struct{ name, value string }{
		{"operation identity", p.OperationID}, {"target", p.Target},
	} {
		if err := boundedText(item.name, item.value, MaxIdentityBytes, true); err != nil {
			return err
		}
	}
	if p.ExpectedRevision == 0 {
		return fmt.Errorf("%w: expected revision must be positive", ErrInvalidEnvelope)
	}
	if err := boundedText("generation", p.GenerationSHA256, MaxGenerationBytes, true); err != nil {
		return err
	}
	return boundedText("candidate", p.Candidate, MaxPolicyBytes, false)
}

func validateConfig(c Config) error {
	if err := boundedText("allowed writer", c.AllowedWriter, MaxIdentityBytes, true); err != nil {
		return err
	}
	for _, item := range []struct{ name, value string }{
		{"pattern", c.Pattern}, {"required type", c.RequiredType}, {"required name", c.RequiredName},
	} {
		if err := boundedText(item.name, item.value, MaxPolicyBytes, false); err != nil {
			return err
		}
	}
	return nil
}

func boundedText(name, value string, limit int, nonempty bool) error {
	if !utf8.ValidString(value) || len(value) > limit || (nonempty && value == "") {
		return fmt.Errorf("%w: %s", ErrInvalidEnvelope, name)
	}
	return nil
}

// Retained short substrings must not keep an arbitrarily large caller backing
// allocation alive. Clone only after bounded UTF-8 validation; bytes stay exact.
func ownedConfig(c Config) Config {
	c.Pattern = strings.Clone(c.Pattern)
	c.RequiredType = strings.Clone(c.RequiredType)
	c.RequiredName = strings.Clone(c.RequiredName)
	c.AllowedWriter = strings.Clone(c.AllowedWriter)
	return c
}

func ownedProposal(p Proposal) Proposal {
	p.OperationID = strings.Clone(p.OperationID)
	p.Target = strings.Clone(p.Target)
	p.Candidate = strings.Clone(p.Candidate)
	p.GenerationSHA256 = strings.Clone(p.GenerationSHA256)
	return p
}
