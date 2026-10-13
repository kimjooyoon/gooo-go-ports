package apply

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
)

func hostFixture(t *testing.T, pattern string) (*Owner, *Writer, Snapshot) {
	t.Helper()
	owner, err := NewInMemoryOwner("grant/tools", Config{Pattern: pattern, RequiredType: "mcp", RequiredName: "tools.weather", AllowedWriter: "automation"})
	if err != nil {
		t.Fatal(err)
	}
	writer, err := owner.WriterForAuthenticated("automation")
	if err != nil {
		t.Fatal(err)
	}
	snapshot := hostSnapshot(t, writer)
	generation, err := sourceGeneration()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Target != "grant/tools" || snapshot.Revision != 1 || snapshot.Epoch == (Epoch{}) || snapshot.GenerationSHA256 != generation {
		t.Fatalf("unexpected initial snapshot: %+v", snapshot)
	}
	return owner, writer, snapshot
}

func hostSnapshot(t *testing.T, writer *Writer) Snapshot {
	t.Helper()
	value, err := writer.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func hostProposal(snapshot Snapshot, operation, candidate string) Proposal {
	return Proposal{Epoch: snapshot.Epoch, OperationID: operation, Target: snapshot.Target, ExpectedRevision: snapshot.Revision, Candidate: candidate, GenerationSHA256: snapshot.GenerationSHA256}
}

func hostApply(t *testing.T, writer *Writer, proposal Proposal) ApplyResult {
	t.Helper()
	value, err := writer.Apply(proposal)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func hostUnchanged(t *testing.T, writer *Writer, want Snapshot) {
	t.Helper()
	if got := hostSnapshot(t, writer); got != want {
		t.Fatalf("policy/config/revision changed: got %+v, want %+v", got, want)
	}
}

func TestHostGeneratedGoooEligibilityAndRefusals(t *testing.T) {
	// These are real calls to the generated Gooo composition. No host matcher or
	// subset implementation supplies the expected semantics.
	cases := []struct {
		name, before, candidate, status, reason, rule string
		beforeMatch, afterMatch                       bool
	}{
		{"useful-tightening", "mcp://*", "mcp://tools.*", "APPLIED", "APPLIED", "OLD_TYPED_ALL", true, true},
		{"widening", "mcp://tools.*", "mcp://*", "REFUSED", "WIDENING_WITNESS", "", true, true},
		{"malformed", "mcp://*", "mcp://a.*.*", "REFUSED", "INVALID_CANONICAL_INPUT", "", false, false},
		{"required-removed", "mcp://*", "mcp://calculator", "REFUSED", "REQUIRED_REQUEST_REMOVED", "OLD_TYPED_ALL", true, false},
		{"empty-candidate", "mcp://*", "", "REFUSED", "REQUIRED_REQUEST_REMOVED", "CANDIDATE_EMPTY", true, false},
		{"current-invariant-broken", "mcp://calculator", "mcp://*", "REFUSED", "CURRENT_REQUIRED_REQUEST_DENIED", "", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, writer, snapshot := hostFixture(t, tc.before)
			proposal := hostProposal(snapshot, tc.name, tc.candidate)
			got := hostApply(t, writer, proposal)
			if got.Schema != "gooo/conditional-apply-host-result/v1" || got.Status != tc.status || got.Reason != tc.reason || got.Replay || !got.Recorded || !got.HasDecision || got.Subject != "automation" || got.Proposal != proposal || got.Observed != snapshot {
				t.Fatalf("incorrect result envelope: %+v", got)
			}
			if got.Decision.RequiredBeforeMatch != tc.beforeMatch || got.Decision.RequiredAfterMatch != tc.afterMatch || got.Decision.SubsetRuleID != tc.rule {
				t.Fatalf("incorrect Gooo decision: %+v", got.Decision)
			}
			if tc.name == "malformed" {
				if got.Decision.Status != "INVALID_INPUT" || got.Decision.InvalidField != "after" || got.Decision.RequiredObserved {
					t.Fatalf("malformed is not ordinary denial: %+v", got.Decision)
				}
			} else if !got.Decision.RequiredObserved {
				t.Fatalf("missing actual required observations: %+v", got.Decision)
			}
			if tc.status == "APPLIED" {
				after := hostSnapshot(t, writer)
				if got.Decision.Status != "ELIGIBLE" || got.Decision.Reason != "SUBSET_AND_REQUIRED_REQUEST_PRESERVED" || after.Pattern != tc.candidate || after.Revision != snapshot.Revision+1 || got.CommittedRevision != after.Revision || after.RequiredType != snapshot.RequiredType || after.RequiredName != snapshot.RequiredName {
					t.Fatalf("incorrect commit: %+v / %+v", got, after)
				}
			} else {
				if got.CommittedRevision != 0 {
					t.Fatalf("refusal reports commit: %+v", got)
				}
				hostUnchanged(t, writer, snapshot)
			}
		})
	}
}

func TestHostSameSnapshotRaceHasOneWinner(t *testing.T) {
	_, writer, before := hostFixture(t, "mcp://*")
	proposals := []Proposal{hostProposal(before, "race-prefix", "mcp://tools.*"), hostProposal(before, "race-suffix", "mcp://*.weather")}
	results := make([]ApplyResult, 2)
	errs := make([]error, 2)
	start := make(chan struct{})
	var ready, done sync.WaitGroup
	ready.Add(2)
	done.Add(2)
	for i := range proposals {
		go func(i int) {
			defer done.Done()
			ready.Done()
			<-start
			results[i], errs[i] = writer.Apply(proposals[i])
		}(i)
	}
	ready.Wait()
	close(start)
	done.Wait()
	winner := -1
	for i, result := range results {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		if result.Status == "APPLIED" {
			if winner != -1 {
				t.Fatal("two same-revision operations applied")
			}
			winner = i
		} else if result.Status != "CONFLICT" || result.Reason != "STALE_REVISION" || result.CommittedRevision != 0 {
			t.Fatalf("unexpected losing result: %+v", result)
		}
	}
	if winner < 0 {
		t.Fatalf("no useful proposal applied: %+v", results)
	}
	after := hostSnapshot(t, writer)
	if after.Revision != before.Revision+1 || after.Pattern != proposals[winner].Candidate {
		t.Fatalf("final state does not identify the sole winner: %+v", after)
	}
	for i, proposal := range proposals {
		resolved, err := writer.Resolve(proposal)
		if err != nil || resolved.Status != "KNOWN" || !resolved.HasResult || resolved.Result.Proposal != proposal || resolved.Result.Status != results[i].Status || resolved.Result.CommittedRevision != results[i].CommittedRevision {
			t.Fatalf("operation/policy publication diverged: %+v, %v", resolved, err)
		}
	}
}

func TestHostABATrustedUpdateInvalidatesOldSnapshot(t *testing.T) {
	owner, writer, before := hostFixture(t, "mcp://*")
	proposal := hostProposal(before, "before-aba", "mcp://tools.*")
	first, err := owner.Update(before.Revision, Config{"mcp://tools.*", "mcp", "tools.weather", "automation"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := owner.Update(first.Revision, Config{"mcp://*", "mcp", "tools.weather", "automation"})
	if err != nil {
		t.Fatal(err)
	}
	if second.Pattern != before.Pattern || second.Revision != before.Revision+2 {
		t.Fatalf("ABA did not advance revision: %+v", second)
	}
	got := hostApply(t, writer, proposal)
	if got.Status != "CONFLICT" || got.Reason != "STALE_REVISION" || got.HasDecision {
		t.Fatalf("stale ABA proposal was evaluated or applied: %+v", got)
	}
	hostUnchanged(t, writer, second)
}

func TestHostRequiredConfigurationAndEqualUpdatesAdvanceRevision(t *testing.T) {
	for _, required := range []string{"tools.weather", "tools.clock"} {
		t.Run(required, func(t *testing.T) {
			owner, writer, before := hostFixture(t, "mcp://*")
			proposal := hostProposal(before, "old-config", "mcp://tools.weather")
			after, err := owner.Update(before.Revision, Config{before.Pattern, "mcp", required, "automation"})
			if err != nil {
				t.Fatal(err)
			}
			if after.Revision != before.Revision+1 || after.RequiredName != required {
				t.Fatalf("configuration update not versioned: %+v", after)
			}
			result := hostApply(t, writer, proposal)
			if result.Status != "CONFLICT" || result.Reason != "STALE_REVISION" || result.HasDecision {
				t.Fatalf("old configuration reused: %+v", result)
			}
			hostUnchanged(t, writer, after)
			if _, err := owner.Update(before.Revision, Config{before.Pattern, "mcp", "tools.weather", "automation"}); err == nil {
				t.Fatal("trusted stale update succeeded")
			}
			hostUnchanged(t, writer, after)
		})
	}
}

func TestHostAuthorizationUpdateRevokesAlreadyIssuedWriter(t *testing.T) {
	owner, writer, before := hostFixture(t, "mcp://*")
	proposal := hostProposal(before, "old-identity", "mcp://tools.*")
	original := hostApply(t, writer, proposal)
	current := hostSnapshot(t, writer)
	changed, err := owner.Update(current.Revision, Config{current.Pattern, current.RequiredType, current.RequiredName, "replacement"})
	if err != nil {
		t.Fatal(err)
	}
	if changed.Revision != current.Revision+1 {
		t.Fatal("authorization change did not advance revision")
	}
	if got, err := writer.Snapshot(); err == nil || got != (Snapshot{}) {
		t.Fatalf("revoked writer read state: %+v, %v", got, err)
	}
	if got, err := writer.Apply(proposal); err == nil || got != (ApplyResult{}) {
		t.Fatalf("revoked duplicate leaked receipt or wrote: %+v, %v", got, err)
	}
	if got, err := writer.Resolve(proposal); err == nil || got != (Resolution{}) {
		t.Fatalf("revoked resolution leaked receipt: %+v, %v", got, err)
	}
	replacement, err := owner.WriterForAuthenticated("replacement")
	if err != nil {
		t.Fatal(err)
	}
	rebound := hostApply(t, replacement, proposal)
	if rebound.Status != "CONFLICT" || rebound.Reason != "OPERATION_ID_REUSED" || rebound.HasDecision || rebound.Replay || rebound.CommittedRevision != 0 {
		t.Fatalf("operation identity was transferable: %+v (original %+v)", rebound, original)
	}
	hostUnchanged(t, replacement, changed)
}

func TestHostDuplicateReturnsOriginalAfterLaterUpdate(t *testing.T) {
	_, writer, before := hostFixture(t, "mcp://*")
	first := hostProposal(before, "original-operation", "mcp://tools.*")
	original := hostApply(t, writer, first)
	later := hostApply(t, writer, hostProposal(hostSnapshot(t, writer), "later-operation", "mcp://tools.weather"))
	if later.Status != "APPLIED" {
		t.Fatal(later)
	}
	current := hostSnapshot(t, writer)
	duplicate := hostApply(t, writer, first)
	want := original
	want.Replay = true
	if duplicate != want {
		t.Fatalf("duplicate did not return exact original outcome: %+v, want %+v", duplicate, want)
	}
	if duplicate.CommittedRevision == current.Revision {
		t.Fatal("duplicate substituted current revision for original commit")
	}
	hostUnchanged(t, writer, current)
}

func TestHostChangedOperationPayloadConflicts(t *testing.T) {
	for _, field := range []string{"candidate", "target", "revision", "generation"} {
		t.Run(field, func(t *testing.T) {
			_, writer, before := hostFixture(t, "mcp://*")
			proposal := hostProposal(before, "fixed-operation", "mcp://tools.*")
			original := hostApply(t, writer, proposal)
			if original.Status != "APPLIED" {
				t.Fatal(original)
			}
			changed := proposal
			switch field {
			case "candidate":
				changed.Candidate = "mcp://tools.weather"
			case "target":
				changed.Target = "grant/other"
			case "revision":
				changed.ExpectedRevision++
			case "generation":
				changed.GenerationSHA256 = "sha256:" + strings.Repeat("0", 64)
			}
			current := hostSnapshot(t, writer)
			got := hostApply(t, writer, changed)
			if got.Status != "CONFLICT" || got.Reason != "OPERATION_ID_REUSED" || got.Replay || got.HasDecision || got.CommittedRevision != 0 {
				t.Fatalf("changed payload reused receipt: %+v", got)
			}
			resolution, err := writer.Resolve(changed)
			if err != nil || resolution.Status != "CONFLICT" || resolution.Reason != "OPERATION_ID_REUSED" || resolution.HasResult {
				t.Fatalf("changed payload resolved original: %+v, %v", resolution, err)
			}
			hostUnchanged(t, writer, current)
		})
	}
}

func TestHostFreshTargetAndGenerationMustMatch(t *testing.T) {
	for _, field := range []string{"target", "generation"} {
		t.Run(field, func(t *testing.T) {
			_, writer, before := hostFixture(t, "mcp://*")
			proposal := hostProposal(before, "wrong-binding", "mcp://tools.*")
			reason := "TARGET_MISMATCH"
			if field == "target" {
				proposal.Target = "grant/other"
			} else {
				proposal.GenerationSHA256 = "sha256:" + strings.Repeat("0", 64)
				reason = "GENERATION_MISMATCH"
			}
			got := hostApply(t, writer, proposal)
			if got.Status != "CONFLICT" || got.Reason != reason || got.HasDecision || got.CommittedRevision != 0 {
				t.Fatalf("unbound proposal reached decision/write: %+v", got)
			}
			hostUnchanged(t, writer, before)
		})
	}
}

func TestHostBoundedLedgerRetainsDuplicatesAtCapacity(t *testing.T) {
	owner, writer, before := hostFixture(t, "mcp://*")
	first := hostProposal(before, "saved-real-operation", "mcp://tools.*")
	original := hostApply(t, writer, first)
	if original.Status != "APPLIED" || MaxOperations != 1024 {
		t.Fatalf("wrong capacity setup: %+v, limit %d", original, MaxOperations)
	}
	// Explicit private occupancy seam: preserve one real record and fill unused
	// slots without claiming 1022 further semantic executions.
	owner.store.mu.Lock()
	state := *owner.store.state
	operations := make(map[string]operationRecord, MaxOperations)
	for key, value := range state.operations {
		operations[key] = value
	}
	template := operations[first.OperationID]
	for i := 0; len(operations) < MaxOperations-1; i++ {
		id := fmt.Sprintf("test-reserved-%04d", i)
		record := template
		record.proposal.OperationID = id
		record.result.Proposal = record.proposal
		operations[id] = record
	}
	state.operations = operations
	owner.store.state = &state
	owner.store.mu.Unlock()
	boundary := hostProposal(hostSnapshot(t, writer), "last-free-slot", "mcp://tools.weather")
	last := hostApply(t, writer, boundary)
	if last.Status != "APPLIED" || !last.Recorded {
		t.Fatalf("last slot unusable: %+v", last)
	}
	full := hostSnapshot(t, writer)
	rejected := hostApply(t, writer, hostProposal(full, "over-capacity", full.Pattern))
	if rejected.Status != "REFUSED" || rejected.Reason != "LEDGER_CAPACITY" || rejected.Recorded || rejected.HasDecision || rejected.CommittedRevision != 0 {
		t.Fatalf("unbounded or evaluated capacity refusal: %+v", rejected)
	}
	duplicate := hostApply(t, writer, first)
	want := original
	want.Replay = true
	if duplicate != want {
		t.Fatalf("capacity lost prior exact receipt: %+v", duplicate)
	}
	resolved, err := writer.Resolve(first)
	if err != nil || resolved.Status != "KNOWN" || !resolved.HasResult || resolved.Result != want {
		t.Fatalf("capacity lost resolution: %+v, %v", resolved, err)
	}
	hostUnchanged(t, writer, full)
	owner.store.mu.Lock()
	count := len(owner.store.state.operations)
	owner.store.mu.Unlock()
	if count != MaxOperations {
		t.Fatalf("ledger size = %d", count)
	}
}

func TestHostResponseDropReconcilesSameOperationWithoutRetry(t *testing.T) {
	_, writer, before := hostFixture(t, "mcp://*")
	proposal := hostProposal(before, "response-dropped", "mcp://tools.*")
	// The caller deliberately discards the returned receipt after a real commit;
	// it resolves the same operation and never invokes Apply a second time.
	_, err := writer.Apply(proposal)
	if err != nil {
		t.Fatal(err)
	}
	resolution, err := writer.Resolve(proposal)
	if err != nil || resolution.Schema != "gooo/conditional-apply-host-resolution/v1" || resolution.Status != "KNOWN" || resolution.Reason != "OPERATION_RECORDED" || !resolution.HasResult || resolution.Result.Status != "APPLIED" || !resolution.Result.Replay || resolution.Result.Proposal != proposal || resolution.Result.CommittedRevision != before.Revision+1 {
		t.Fatalf("lost response treated as rollback: %+v, %v", resolution, err)
	}
	current := hostSnapshot(t, writer)
	if current.Pattern != proposal.Candidate || current.Revision != before.Revision+1 {
		t.Fatalf("reconciliation caused a second write: %+v", current)
	}
}

func TestHostUnknownEpochAndAbsentOperationPreserveUncertainty(t *testing.T) {
	_, writer, before := hostFixture(t, "mcp://*")
	absent := hostProposal(before, "not-yet-observed", "mcp://tools.*")
	resolution, err := writer.Resolve(absent)
	if err != nil || resolution.Status != "NOT_OBSERVED_AT_THIS_LOOKUP" || resolution.Reason != "OPERATION_NOT_RECORDED" || resolution.HasResult {
		t.Fatalf("absence claimed an outcome: %+v, %v", resolution, err)
	}
	wrong := absent
	wrong.Epoch[0] ^= 1
	got := hostApply(t, writer, wrong)
	if got.Status != "UNKNOWN_OUTCOME" || got.Reason != "DIFFERENT_EPOCH" || got.Recorded || got.HasDecision || got.CommittedRevision != 0 {
		t.Fatalf("different incarnation treated as safe refusal: %+v", got)
	}
	resolution, err = writer.Resolve(wrong)
	if err != nil || resolution.Status != "UNKNOWN_OUTCOME" || resolution.Reason != "DIFFERENT_EPOCH" || resolution.HasResult {
		t.Fatalf("lost incarnation was resolved: %+v, %v", resolution, err)
	}
	hostUnchanged(t, writer, before)
}

func TestHostReturnedValuesDoNotAliasStateOrSavedOutcome(t *testing.T) {
	_, writer, before := hostFixture(t, "mcp://*")
	proposal := hostProposal(before, "immutable-result", "mcp://tools.*")
	result := hostApply(t, writer, proposal)
	original := result
	originalProposal := proposal
	current := hostSnapshot(t, writer)
	snapshotCopy := current
	snapshotCopy.Pattern = "*"
	snapshotCopy.RequiredName = "tampered"
	proposal.Candidate = "*"
	result.Proposal.Candidate = "*"
	result.Observed.Pattern = "*"
	result.Decision.Status = "ELIGIBLE"
	result.Decision.SubsetRuleID = "UNKNOWN"
	result.CommittedRevision = 0
	hostUnchanged(t, writer, current)
	resolution, err := writer.Resolve(originalProposal)
	if err != nil {
		t.Fatal(err)
	}
	want := original
	want.Replay = true
	if resolution.Status != "KNOWN" || !resolution.HasResult || resolution.Result != want {
		t.Fatalf("returned value altered saved receipt: %+v", resolution)
	}
	resolution.Result.Proposal.Candidate = "*"
	resolution.Result.Decision.Reason = "tampered"
	again, err := writer.Resolve(originalProposal)
	if err != nil || again.Result != want {
		t.Fatalf("resolution value aliases saved receipt: %+v, %v", again, err)
	}
}

func TestHostRevisionExhaustionFailsClosed(t *testing.T) {
	owner, writer, _ := hostFixture(t, "mcp://*")
	// Explicit private boundary seam avoids inventing 2^64 successful writes.
	owner.store.mu.Lock()
	state := *owner.store.state
	state.snapshot.Revision = math.MaxUint64
	owner.store.state = &state
	owner.store.mu.Unlock()
	before := hostSnapshot(t, writer)
	got := hostApply(t, writer, hostProposal(before, "max-revision", "mcp://tools.*"))
	if got.Status != "REFUSED" || got.Reason != "REVISION_EXHAUSTED" || got.CommittedRevision != 0 {
		t.Fatalf("revision wrapped or published: %+v", got)
	}
	hostUnchanged(t, writer, before)
	if after, err := owner.Update(math.MaxUint64, Config{"mcp://tools.*", "mcp", "tools.weather", "automation"}); err == nil || after != (Snapshot{}) {
		t.Fatalf("owner update wrapped: %+v, %v", after, err)
	}
	hostUnchanged(t, writer, before)
}

func TestHostMalformedTypedEnvelopeErrorsBeforeWrite(t *testing.T) {
	changes := []struct {
		name string
		edit func(*Proposal)
	}{
		{"empty-operation", func(p *Proposal) { p.OperationID = "" }},
		{"oversized-operation", func(p *Proposal) { p.OperationID = strings.Repeat("x", MaxIdentityBytes+1) }},
		{"invalid-operation-utf8", func(p *Proposal) { p.OperationID = string([]byte{0xff}) }},
		{"empty-target", func(p *Proposal) { p.Target = "" }},
		{"oversized-target", func(p *Proposal) { p.Target = strings.Repeat("x", MaxIdentityBytes+1) }},
		{"zero-revision", func(p *Proposal) { p.ExpectedRevision = 0 }},
		{"empty-generation", func(p *Proposal) { p.GenerationSHA256 = "" }},
		{"oversized-generation", func(p *Proposal) { p.GenerationSHA256 = strings.Repeat("x", MaxGenerationBytes+1) }},
		{"invalid-candidate-utf8", func(p *Proposal) { p.Candidate = string([]byte{0xff}) }},
		{"oversized-transport-candidate", func(p *Proposal) { p.Candidate = strings.Repeat("x", MaxPolicyBytes+1) }},
	}
	for _, tc := range changes {
		t.Run(tc.name, func(t *testing.T) {
			owner, writer, before := hostFixture(t, "mcp://*")
			proposal := hostProposal(before, tc.name, "mcp://tools.*")
			tc.edit(&proposal)
			if got, err := writer.Apply(proposal); err == nil || got != (ApplyResult{}) {
				t.Fatalf("bad envelope returned nonzero result: %+v, %v", got, err)
			}
			if got, err := writer.Resolve(proposal); err == nil || got != (Resolution{}) {
				t.Fatalf("bad lookup envelope accepted: %+v, %v", got, err)
			}
			hostUnchanged(t, writer, before)
			owner.store.mu.Lock()
			count := len(owner.store.state.operations)
			owner.store.mu.Unlock()
			if count != 0 {
				t.Fatal("transport error published an operation record")
			}
		})
	}
	// 129 bytes is representable transport and must reach Gooo's narrower domain.
	_, writer, before := hostFixture(t, "mcp://*")
	result := hostApply(t, writer, hostProposal(before, "gooo-domain-129", "a://"+strings.Repeat("x", 125)))
	if result.Status != "REFUSED" || result.Reason != "INVALID_CANONICAL_INPUT" || !result.HasDecision || result.Decision.InvalidField != "after" {
		t.Fatalf("transport swallowed typed Gooo refusal: %+v", result)
	}
	hostUnchanged(t, writer, before)
}

func TestHostInjectedEvaluatorFaultsCannotPublish(t *testing.T) {
	// These are explicitly injected dependency faults, not claims that real
	// canonical input causes unknown or contradictory generated records.
	cases := []struct {
		name     string
		decision Decision
		err      error
	}{
		{"error", Decision{}, errors.New("injected evaluation failure")},
		{"unknown-status", Decision{Status: "FUTURE_UNKNOWN"}, nil},
		{"unknown-subset-rule", Decision{Status: "ELIGIBLE", Reason: "SUBSET_AND_REQUIRED_REQUEST_PRESERVED", RequiredObserved: true, RequiredBeforeMatch: true, RequiredAfterMatch: true, SubsetRuleID: "UNKNOWN"}, nil},
		{"contradictory-eligible", Decision{Status: "ELIGIBLE", Reason: "SUBSET_AND_REQUIRED_REQUEST_PRESERVED", RequiredObserved: true, RequiredBeforeMatch: true, RequiredAfterMatch: false, SubsetRuleID: "OLD_TYPED_ALL"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			owner, writer, before := hostFixture(t, "mcp://*")
			owner.store.eval = func(f Facts) (evaluationProof, error) {
				proof, err := evaluate(f)
				if err != nil {
					t.Fatal(err)
				}
				proof.Decision = tc.decision
				return proof, tc.err
			}
			result, err := writer.Apply(hostProposal(before, "injected-"+tc.name, "mcp://tools.*"))
			if err == nil || result != (ApplyResult{}) {
				t.Fatalf("injected boundary fault became an outcome: %+v, %v", result, err)
			}
			hostUnchanged(t, writer, before)
			owner.store.mu.Lock()
			count := len(owner.store.state.operations)
			owner.store.mu.Unlock()
			if count != 0 {
				t.Fatal("injected evaluator fault recorded or committed")
			}
		})
	}
}

func TestHostInjectedUnprovenRemainsRefused(t *testing.T) {
	// The current canonical analyzer has no known reachable UNPROVEN input. This
	// private dependency seam tests host consumption only, not actual Gooo reachability.
	owner, writer, before := hostFixture(t, "mcp://*")
	owner.store.eval = func(f Facts) (evaluationProof, error) {
		proof, err := evaluate(f)
		if err != nil {
			return evaluationProof{}, err
		}
		proof.Decision = Decision{Status: "REFUSED", Reason: "REFINEMENT_UNPROVEN", RequiredObserved: true, RequiredBeforeMatch: true, RequiredAfterMatch: true}
		return proof, nil
	}
	result := hostApply(t, writer, hostProposal(before, "injected-unproven", "mcp://tools.*"))
	if result.Status != "REFUSED" || result.Reason != "REFINEMENT_UNPROVEN" || !result.Recorded || result.CommittedRevision != 0 {
		t.Fatalf("unproven became eligible: %+v", result)
	}
	hostUnchanged(t, writer, before)
}

func TestHostSourceBindingFailureRefusesOwner(t *testing.T) {
	for _, tc := range []struct {
		name, generation string
		err              error
	}{
		{"source-binding-error", "", errors.New("injected source binding failure")},
		{"empty-generation", "", nil},
		{"truncated-generation", "sha256:abc", nil},
		{"non-hex-generation", "sha256:" + strings.Repeat("z", 64), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			owner, err := newInMemoryOwner("grant/tools", Config{"mcp://*", "mcp", "tools.weather", "automation"}, func() (string, error) {
				return tc.generation, tc.err
			})
			if owner != nil || !errors.Is(err, ErrInvalidGeneration) {
				t.Fatalf("invalid source binding created an owner: %v, %v", owner, err)
			}
		})
	}
}

func TestHostEvaluationGenerationAndFactsMustMatch(t *testing.T) {
	for _, field := range []string{"generation", "missing-generation", "before", "after", "required-type", "required-name"} {
		t.Run(field, func(t *testing.T) {
			owner, writer, before := hostFixture(t, "mcp://*")
			owner.store.eval = func(f Facts) (evaluationProof, error) {
				proof, err := evaluate(f)
				if err != nil {
					return evaluationProof{}, err
				}
				switch field {
				case "generation":
					proof.Bindings.GenerationSHA256 = "sha256:" + strings.Repeat("0", 64)
				case "missing-generation":
					proof.Bindings.GenerationSHA256 = ""
				case "before":
					proof.Facts.Before = "*"
				case "after":
					proof.Facts.After = "mcp://tools.weather"
				case "required-type":
					proof.Facts.RequiredType = "llm"
				case "required-name":
					proof.Facts.RequiredName = "other"
				}
				return proof, nil
			}
			result, err := writer.Apply(hostProposal(before, "misbound-"+field, "mcp://tools.*"))
			if !errors.Is(err, ErrInvalidGeneration) || result != (ApplyResult{}) {
				t.Fatalf("misbound eligible proof reached publication: %+v, %v", result, err)
			}
			hostUnchanged(t, writer, before)
			owner.store.mu.Lock()
			count := len(owner.store.state.operations)
			owner.store.mu.Unlock()
			if count != 0 {
				t.Fatal("misbound proof recorded an operation")
			}
		})
	}
}
