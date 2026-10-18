package main

import (
	"context"
	"math"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// All observations below are synthetic callbacks. No compiler, command, native
// body, child process or new build cache is created by these tests.
func ciShardObservationFixture(t *testing.T) (*ciShardCommandObservations, ciShardPlan, ciShardPartial) {
	t.Helper()
	plan := ciShardTestPlan(t, 1)
	c, err := newCIShardCommandObservations(plan, 0)
	if err != nil {
		t.Fatal(err)
	}
	return c, plan, ciShardTestPartials(t, plan)[0]
}

func ciShardObservationSample() nativeCommandResources {
	return nativeCommandResourcesFromValues(runtime.GOOS, 1, 2, 3, 4, 5)
}

func fillCIShardObservationPrefix(c *ciShardCommandObservations, partial ciShardPartial) {
	for _, suite := range partial.Suites {
		for _, batch := range suite.Batches {
			for _, phase := range []string{"execute", "replay"} {
				c.observer(suite.OriginalIndex)(batch.Index, phase, ciShardObservationSample())
			}
		}
	}
}

func TestCIShardCommandObservationsParallelCoverageAndDetachedDocument(t *testing.T) {
	c, plan, partial := ciShardObservationFixture(t)
	var joined sync.WaitGroup
	for _, suite := range partial.Suites {
		joined.Go(func() {
			for _, batch := range suite.Batches {
				for _, phase := range []string{"execute", "replay"} {
					r := ciShardObservationSample()
					c.observer(suite.OriginalIndex)(batch.Index, phase, r)
					if r.MaxRSSBytes != nil {
						*r.MaxRSSBytes = 99999
					}
				}
			}
		})
	}
	joined.Wait()
	doc, err := c.finish(partial)
	if err != nil || !doc.Complete || !doc.Joined || doc.NativeAcceptance || doc.ProducerAuthenticated || doc.Schema != ciShardCommandObservationsSchema || doc.PlanSHA256 != plan.PlanSHA256 || len(doc.Suites) != len(partial.Suites) {
		t.Fatal(doc, err)
	}
	for _, suite := range doc.Suites {
		for _, batch := range suite.Batches {
			if !batch.VerifiedPrefix || !reflect.DeepEqual(batch.CommandResources.Execute, ciShardObservationSample()) || !reflect.DeepEqual(batch.CommandResources.Replay, ciShardObservationSample()) {
				t.Fatal("observation lost ownership or aliases callback pointers", batch)
			}
		}
	}
	for name := range doc.Sources {
		delete(doc.Sources, name)
	}
	if len(c.plan.Sources) == 0 || len(plan.Sources) == 0 {
		t.Fatal("document aliases collector/caller source map")
	}
	if _, err := c.finish(partial); err == nil {
		t.Fatal("second finish succeeded")
	}
}

func TestCIShardCommandObservationsActualUnavailableIsCoverageNotMissing(t *testing.T) {
	c, _, partial := ciShardObservationFixture(t)
	for _, suite := range partial.Suites {
		for _, batch := range suite.Batches {
			for _, phase := range []string{"execute", "replay"} {
				c.observer(suite.OriginalIndex)(batch.Index, phase, unavailableNativeCommandResources(runtime.GOOS, "synthetic unsupported wait accounting"))
			}
		}
	}
	if doc, err := c.finish(partial); err != nil || !doc.Complete {
		t.Fatal("actual unavailable callbacks lost", doc, err)
	}
	missing, _, partial := ciShardObservationFixture(t)
	doc, err := missing.finish(partial)
	if err == nil || doc.Complete || doc.Suites[0].Batches[0].CommandResources.Execute.Status != "NOT_OBSERVED" || doc.Suites[0].Batches[0].CommandResources.Execute.MaxRSSBytes != nil {
		t.Fatal("missing callback was fabricated", doc, err)
	}
}

func TestCIShardCommandObservationsRefuseForeignDuplicatePhaseAndLateCallbacks(t *testing.T) {
	for _, mutation := range []string{"original", "batch", "phase", "duplicate", "replay-first", "status", "scope", "goos"} {
		t.Run(mutation, func(t *testing.T) {
			c, plan, partial := ciShardObservationFixture(t)
			original, batch, phase := 0, 0, "execute"
			r := ciShardObservationSample()
			switch mutation {
			case "original":
				original = len(plan.Suites)
			case "batch":
				batch = int(plan.Suites[0].Batches)
			case "phase":
				phase = "inspect_execution"
			case "duplicate":
				c.observer(original)(batch, phase, r)
			case "replay-first":
				phase = "replay"
			case "status":
				r.Status = "NOT_OBSERVED"
			case "scope":
				r.Scope = "process-tree-peak"
			case "goos":
				r.GOOS = "foreign"
			}
			c.observer(original)(batch, phase, r)
			fillCIShardObservationPrefix(c, partial)
			if doc, err := c.finish(partial); err == nil || doc.Complete {
				t.Fatal("invalid observation succeeded", mutation, doc, err)
			}
		})
	}
	c, _, partial := ciShardObservationFixture(t)
	fillCIShardObservationPrefix(c, partial)
	if _, err := c.finish(partial); err != nil {
		t.Fatal(err)
	}
	c.observer(0)(0, "execute", ciShardObservationSample())
	if _, err := c.finish(partial); err == nil || !strings.Contains(err.Error(), "callback after") {
		t.Fatal("late callback hidden", err)
	}
}

func TestCIShardCommandObservationsRefuseUnjoinedOrForeignPartial(t *testing.T) {
	for _, mutation := range []string{"joined", "run", "plan", "source", "index", "roster", "original"} {
		t.Run(mutation, func(t *testing.T) {
			c, _, partial := ciShardObservationFixture(t)
			fillCIShardObservationPrefix(c, partial)
			switch mutation {
			case "joined":
				partial.Joined = false
			case "run":
				partial.Run.Attempt++
			case "plan":
				partial.PlanSHA256 = contentDigest([]byte("foreign"))
			case "source":
				partial.Sources = map[string]ciShardSource{}
			case "index":
				partial.Index++
			case "roster":
				partial.Suites = partial.Suites[:1]
			case "original":
				partial.Suites[0].OriginalIndex = -1
			}
			if doc, err := c.finish(partial); err == nil || doc.Complete {
				t.Fatal("foreign/unjoined partial succeeded", mutation, doc, err)
			}
		})
	}
}

func TestCIShardCommandObservationsPreserveFailedPrefixAndUnobservedTail(t *testing.T) {
	c, _, partial := ciShardObservationFixture(t)
	partial.State = "FAILED"
	partial.Suites[1].State, partial.Suites[1].Error = "FAILED", "second replay failed"
	partial.Suites[1].Batches = partial.Suites[1].Batches[:1]
	for i := 2; i < len(partial.Suites); i++ {
		partial.Suites[i].State = "NOT_STARTED"
		partial.Suites[i].Batches = []ciShardBatch{}
	}
	fillCIShardObservationPrefix(c, partial)
	c.observer(1)(1, "execute", ciShardObservationSample())
	doc, err := c.finish(partial)
	if err == nil || doc.Complete || !doc.Joined || !doc.Suites[1].Batches[0].VerifiedPrefix || doc.Suites[1].Batches[1].VerifiedPrefix || doc.Suites[1].Batches[1].CommandResources.Execute.Status == "NOT_OBSERVED" || doc.Suites[1].Batches[1].CommandResources.Replay.Status != "NOT_OBSERVED" || doc.Suites[2].Batches[0].CommandResources.Execute.Status != "NOT_OBSERVED" {
		t.Fatal("failure prefix/tail was discarded or inflated", doc, err)
	}
}

func TestCIShardCommandResourcesValidateExactUnitsAndUnavailableShape(t *testing.T) {
	for _, goos := range []string{"linux", "darwin"} {
		for _, mutation := range []string{"negative", "unit", "scale", "overflow", "null", "reason", "unknown"} {
			t.Run(goos+"/"+mutation, func(t *testing.T) {
				r := nativeCommandResourcesFromValues(goos, 0, 0, 0, 0, 1)
				switch mutation {
				case "negative":
					*r.UserCPUNS = -1
				case "unit":
					*r.MaxRSSUnit = "wrong"
				case "scale":
					*r.MaxRSSBytes++
				case "overflow":
					*r.MaxRSSRaw = math.MaxInt64
					*r.MaxRSSBytes = 0
				case "null":
					r.SystemCPUNS = nil
				case "reason":
					r.Reason = "not unavailable"
				case "unknown":
					r.Status = "UNKNOWN"
				}
				if err := validateCIShardCommandResources(r, goos); err == nil {
					t.Fatal("invalid accounting accepted", r)
				}
			})
		}
		r := unavailableNativeCommandResources(goos, "reason")
		if err := validateCIShardCommandResources(r, goos); err != nil {
			t.Fatal(err)
		}
		r.Reason = " "
		if err := validateCIShardCommandResources(r, goos); err == nil {
			t.Fatal("empty unavailable reason accepted")
		}
	}
}

func TestCIShardCommandObserverPreservesNilContextAndForwardsTimingOnce(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	var verifier nativeSuiteVerifier
	if verifier.commandContext(parent, nil, 0, "execute") != parent {
		t.Fatal("nil hook changed original context")
	}
	owner := &nativeTiming{}
	batch := &nativeBatchTiming{owner: owner}
	batch.phases[0].Name = "execute"
	calls := 0
	verifier.commandObserver = func(index int, phase string, r nativeCommandResources) {
		calls++
		if index != 4 || phase != "execute" || !reflect.DeepEqual(r, ciShardObservationSample()) {
			t.Error("foreign hook observation")
		}
	}
	ctx := verifier.commandContext(parent, batch, 4, "execute")
	callback := ctx.Value(nativeCommandResourcesContextKey{}).(func(nativeCommandResources))
	callback(ciShardObservationSample())
	if calls != 1 || !batch.commandObserved[0] || batch.commandObserved[1] || owner.errorCount != 0 || !reflect.DeepEqual(batch.commandResources.Execute, ciShardObservationSample()) {
		t.Fatal("timing/collector did not each receive exactly once", calls, batch, owner.errorCount)
	}
	cancel()
	if ctx.Err() != context.Canceled {
		t.Fatal("observer lost parent cancellation")
	}
}

func TestCIShardCommandObservationDocumentRejectsMissingDuplicateAndForgedClaims(t *testing.T) {
	c, plan, partial := ciShardObservationFixture(t)
	fillCIShardObservationPrefix(c, partial)
	doc, err := c.finish(partial)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateCIShardCommandObservationDocument(plan, partial, doc); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []string{"run", "source", "schema", "goos", "duplicate-suite", "missing-batch", "duplicate-batch", "prefix", "missing-execute", "native-acceptance", "producer-auth", "failure", "state"} {
		t.Run(mutation, func(t *testing.T) {
			d := ciShardTestClone(t, doc)
			switch mutation {
			case "run":
				d.Run.Attempt++
			case "source":
				d.Sources = map[string]ciShardSource{}
			case "schema":
				d.Schema = "legacy-timing"
			case "goos":
				d.GOOS = "foreign"
			case "duplicate-suite":
				d.Suites[1] = d.Suites[0]
			case "missing-batch":
				d.Suites[0].Batches = nil
			case "duplicate-batch":
				d.Suites[1].Batches[1] = d.Suites[1].Batches[0]
			case "prefix":
				d.Suites[0].Batches[0].VerifiedPrefix = false
			case "missing-execute":
				d.Suites[0].Batches[0].CommandResources.Execute = unobservedNativeCommandResources()
			case "native-acceptance":
				d.NativeAcceptance = true
			case "producer-auth":
				d.ProducerAuthenticated = true
			case "failure":
				d.Failure = "hidden failure"
			case "state":
				d.State = "COMPLETE"
			}
			if err := validateCIShardCommandObservationDocument(plan, partial, d); err == nil {
				t.Fatal("forged observation document accepted", mutation)
			}
		})
	}
}
