package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func workerTestSuites(count int) []suite {
	suites := make([]suite, count)
	for i := range suites {
		suites[i] = suite{Package: "example/worker", Activity: fmt.Sprintf("Suite%02d", i)}
	}
	return suites
}

func workerTestContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestNativeSuiteWorkersBoundCoverageAndOrder(t *testing.T) {
	for _, workers := range []int{1, 2, 3, maxNativeSuiteWorkers} {
		t.Run(fmt.Sprint(workers), func(t *testing.T) {
			ctx := workerTestContext(t)
			suites := workerTestSuites(59)
			calls := make([]atomic.Int32, len(suites))
			var active, peak atomic.Int32
			started := make(chan int, len(suites))
			release := make(chan struct{})
			finished := make(chan struct{})
			var results []nativeSuiteResult
			var err error
			go func() {
				results, err = runNativeSuites(ctx, suites, workers, func(ctx context.Context, index int, s suite) (nativeSuiteResult, error) {
					calls[index].Add(1)
					current := active.Add(1)
					defer active.Add(-1)
					for old := peak.Load(); current > old && !peak.CompareAndSwap(old, current); old = peak.Load() {
					}
					started <- index
					select {
					case <-release:
					case <-ctx.Done():
						return nativeSuiteResult{}, ctx.Err()
					}
					return nativeSuiteResult{Observation: observation{Package: s.Package, Activity: s.Activity, Cases: index + 1}}, nil
				})
				close(finished)
			}()
			for range workers {
				select {
				case <-started:
				case <-ctx.Done():
					t.Fatal("workers did not start", ctx.Err())
				}
			}
			select {
			case <-started:
				t.Fatal("started work above the worker bound")
			default:
			}
			close(release)
			select {
			case <-finished:
			case <-ctx.Done():
				t.Fatal("workers did not join", ctx.Err())
			}
			if err != nil || len(results) != len(suites) || active.Load() != 0 || peak.Load() != int32(workers) {
				t.Fatalf("coverage/bound/join failed: results=%d active=%d peak=%d error=%v", len(results), active.Load(), peak.Load(), err)
			}
			for index, result := range results {
				if calls[index].Load() != 1 || result.Observation.Activity != suites[index].Activity || result.Observation.Cases != index+1 {
					t.Fatal("suite skipped, repeated, or reordered", index)
				}
			}
		})
	}
}

func TestNativeSuiteWorkersRetainOrderAfterOutOfOrderCompletion(t *testing.T) {
	ctx := workerTestContext(t)
	first := make(chan struct{})
	second := make(chan struct{})
	finished := make(chan struct{})
	var results []nativeSuiteResult
	var err error
	go func() {
		results, err = runNativeSuites(ctx, workerTestSuites(2), 2, func(ctx context.Context, index int, s suite) (nativeSuiteResult, error) {
			if index == 0 {
				select {
				case <-first:
				case <-ctx.Done():
					return nativeSuiteResult{}, ctx.Err()
				}
			} else {
				close(second)
			}
			return nativeSuiteResult{Observation: observation{Activity: s.Activity}}, nil
		})
		close(finished)
	}()
	select {
	case <-second:
	case <-ctx.Done():
		t.Fatal("second suite did not finish")
	}
	close(first)
	select {
	case <-finished:
	case <-ctx.Done():
		t.Fatal("workers did not join")
	}
	if err != nil || results[0].Observation.Activity != "Suite00" || results[1].Observation.Activity != "Suite01" {
		t.Fatal("completion order changed result order", err)
	}
}

func TestNativeSuiteWorkersReturnEarliestFailureAndJoinCancelledWork(t *testing.T) {
	ctx := workerTestContext(t)
	lowError, highError := errors.New("earlier suite failed"), errors.New("later suite failed")
	release := []chan struct{}{make(chan struct{}), make(chan struct{}), make(chan struct{})}
	started := make(chan int, 5)
	laterJoined := make(chan struct{})
	finished := make(chan struct{})
	var active atomic.Int32
	var results []nativeSuiteResult
	var err error
	go func() {
		results, err = runNativeSuites(ctx, workerTestSuites(5), 4, func(ctx context.Context, index int, s suite) (nativeSuiteResult, error) {
			active.Add(1)
			defer active.Add(-1)
			started <- index
			if index == 3 {
				<-ctx.Done()
				close(laterJoined)
				return nativeSuiteResult{}, ctx.Err()
			}
			if index > 3 {
				return nativeSuiteResult{}, errors.New("new suite started after failure")
			}
			select {
			case <-release[index]:
			case <-ctx.Done():
				return nativeSuiteResult{}, ctx.Err()
			}
			if index == 1 {
				return nativeSuiteResult{}, lowError
			}
			if index == 2 {
				return nativeSuiteResult{}, highError
			}
			return nativeSuiteResult{}, nil
		})
		close(finished)
	}()
	for range 4 {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("initial workers did not start")
		}
	}
	close(release[2])
	select {
	case <-laterJoined:
	case <-ctx.Done():
		t.Fatal("later suite was not cancelled")
	}
	close(release[0])
	close(release[1])
	select {
	case <-finished:
	case <-ctx.Done():
		t.Fatal("failure workers did not join")
	}
	if results != nil || !errors.Is(err, lowError) || active.Load() != 0 {
		t.Fatal("did not return the earliest failure after joining", err, active.Load())
	}
	select {
	case index := <-started:
		t.Fatal("started a new suite after failure", index)
	default:
	}
}

func TestNativeSuiteWorkersParentCancellationRetainsObservedSemanticFailure(t *testing.T) {
	outer := workerTestContext(t)
	ctx, cancel := context.WithCancel(outer)
	defer cancel()
	started := make(chan int, 3)
	releaseFailure := make(chan struct{})
	finished := make(chan error, 1)
	semantic := errors.New("current receipt activity identity differs")
	var active atomic.Int32
	go func() {
		results, err := runNativeSuitesOrdered(ctx, workerTestSuites(3), 2, []int{2, 0, 1},
			func(ctx context.Context, index int, s suite) (nativeSuiteResult, error) {
				active.Add(1)
				defer active.Add(-1)
				started <- index
				if index == 2 {
					select {
					case <-releaseFailure:
						return nativeSuiteResult{}, semantic
					case <-ctx.Done():
						return nativeSuiteResult{}, ctx.Err()
					}
				}
				<-ctx.Done()
				return nativeSuiteResult{}, ctx.Err()
			})
		if results != nil {
			finished <- fmt.Errorf("cancellation returned acceptance results")
			return
		}
		finished <- err
	}()
	initial := map[int]bool{}
	for range 2 {
		select {
		case index := <-started:
			initial[index] = true
		case <-outer.Done():
			t.Fatal("initial workers did not start", outer.Err())
		}
	}
	if !reflect.DeepEqual(initial, map[int]bool{2: true, 0: true}) {
		t.Fatal("priority changed", initial)
	}
	close(releaseFailure)
	// Index1 starts only after the coordinator has received index2's failure.
	select {
	case index := <-started:
		if index != 1 {
			t.Fatal("pending lower index changed", index)
		}
	case <-outer.Done():
		t.Fatal("pending lower original index did not start", outer.Err())
	}
	cancel()
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) || errors.Is(err, semantic) || active.Load() != 0 ||
			!strings.Contains(err.Error(), "previously observed suite 2 example/worker:Suite02: "+semantic.Error()) {
			t.Fatal("lost context outcome, observed diagnostic, or worker closure", err, active.Load())
		}
	case <-outer.Done():
		t.Fatal("canceled workers did not join", outer.Err())
	}
}

func TestNativeSuiteWorkersParentCancellationAndInvalidBounds(t *testing.T) {
	for _, workers := range []int{-1, 0, maxNativeSuiteWorkers + 1} {
		_, err := runNativeSuites(context.Background(), workerTestSuites(1), workers,
			func(context.Context, int, suite) (nativeSuiteResult, error) {
				t.Fatal("invalid worker setting executed work")
				return nativeSuiteResult{}, nil
			})
		if err == nil {
			t.Fatal("accepted invalid worker setting", workers)
		}
	}
	ctx, cancel := context.WithCancel(workerTestContext(t))
	started := make(chan struct{}, 2)
	finished := make(chan error, 1)
	var active atomic.Int32
	go func() {
		_, err := runNativeSuites(ctx, workerTestSuites(3), 2, func(ctx context.Context, _ int, _ suite) (nativeSuiteResult, error) {
			active.Add(1)
			defer active.Add(-1)
			started <- struct{}{}
			<-ctx.Done()
			return nativeSuiteResult{}, ctx.Err()
		})
		finished <- err
	}()
	for range 2 {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("workers did not start")
		}
	}
	cancel()
	if err := <-finished; !errors.Is(err, context.Canceled) || active.Load() != 0 {
		t.Fatal("parent cancellation returned before joining", err, active.Load())
	}
	select {
	case <-started:
		t.Fatal("started work after parent cancellation")
	default:
	}
}

func TestSuiteEvidenceDirectoryCollisionIsRejected(t *testing.T) {
	suites := []suite{{Package: "go/a-b", Activity: "Check"}, {Package: "go/a/b", Activity: "Check"}}
	if err := validateSuiteEvidenceDirectories(t.TempDir(), suites); err == nil {
		t.Fatal("accepted distinct suites with colliding output paths")
	}
}

func TestNativeSuiteDispatchOrderUsesCaseLengthsAndOriginalIndexTies(t *testing.T) {
	suites := workerTestSuites(5)
	for index, count := range []int{2, 5, 0, 5, 2} {
		suites[index].Cases = make([]testCase, count)
	}
	for _, test := range []struct {
		value string
		order nativeSuiteOrder
		want  []int
	}{
		{"", nativeSuiteOrderCatalog, []int{0, 1, 2, 3, 4}},
		{"catalog", nativeSuiteOrderCatalog, []int{0, 1, 2, 3, 4}},
		{"largest-corpus-first", nativeSuiteOrderLargestCorpusFirst, []int{1, 3, 0, 4, 2}},
	} {
		t.Run(test.value, func(t *testing.T) {
			order, err := parseNativeSuiteOrder(test.value)
			if err != nil || order != test.order {
				t.Fatal("parse order", order, err)
			}
			got, err := nativeSuiteDispatchOrder(suites, order)
			if err != nil || !reflect.DeepEqual(got, test.want) {
				t.Fatal("dispatch order", got, err)
			}
		})
	}
	for _, value := range []string{"largest", " catalog", "catalog ", "LARGEST-CORPUS-FIRST"} {
		if _, err := parseNativeSuiteOrder(value); err == nil {
			t.Fatal("unknown order accepted", value)
		}
		if _, err := nativeSuiteDispatchOrder(suites, nativeSuiteOrder(value)); err == nil {
			t.Fatal("unknown typed order accepted", value)
		}
	}
	for index, count := range []int{2, 5, 0, 5, 2} {
		if suites[index].Activity != fmt.Sprintf("Suite%02d", index) || len(suites[index].Cases) != count {
			t.Fatal("dispatch planning mutated the original suites", index)
		}
	}
}

func TestNativeSuiteWorkersDefaultCatalogAndExplicitOrderPreserveOriginalResults(t *testing.T) {
	for _, order := range [][]int{nil, {4, 2, 5, 0, 3, 1}} {
		ctx := workerTestContext(t)
		suites := workerTestSuites(6)
		called := []int{}
		verify := func(_ context.Context, index int, s suite) (nativeSuiteResult, error) {
			called = append(called, index)
			return nativeSuiteResult{Observation: observation{Activity: s.Activity, Cases: index + 1}}, nil
		}
		var results []nativeSuiteResult
		var err error
		want := order
		if order == nil {
			want = []int{0, 1, 2, 3, 4, 5}
			results, err = runNativeSuites(ctx, suites, 1, verify)
		} else {
			results, err = runNativeSuitesOrdered(ctx, suites, 1, order, verify)
		}
		if err != nil || !reflect.DeepEqual(called, want) || len(results) != len(suites) {
			t.Fatal("dispatch changed or coverage missing", called, err)
		}
		for index, result := range results {
			if result.Observation.Activity != suites[index].Activity || result.Observation.Cases != index+1 {
				t.Fatal("result no longer matches original suite index", index)
			}
		}
	}
}

func TestNativeSuiteWorkersRejectInvalidPermutationBeforeAnyCallback(t *testing.T) {
	for _, order := range [][]int{nil, {0, 1}, {0, 1, 2, 3}, {-1, 1, 2}, {0, 1, 3}, {0, 1, 1}} {
		calls := 0
		result, err := runNativeSuitesOrdered(context.Background(), workerTestSuites(3), 2, order,
			func(context.Context, int, suite) (nativeSuiteResult, error) {
				calls++
				return nativeSuiteResult{}, nil
			})
		if result != nil || err == nil || calls != 0 {
			t.Fatal("invalid permutation started work", order, calls, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	_, err := runNativeSuitesOrdered(ctx, workerTestSuites(3), 2, []int{2, 1, 0},
		func(context.Context, int, suite) (nativeSuiteResult, error) {
			calls++
			return nativeSuiteResult{}, nil
		})
	if !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatal("already canceled parent started work", calls, err)
	}
}

func TestNativeSuiteWorkersFreezePermutationBeforeCallbacks(t *testing.T) {
	order := []int{2, 1, 0}
	called := []int{}
	_, err := runNativeSuitesOrdered(workerTestContext(t), workerTestSuites(3), 1, order,
		func(_ context.Context, index int, _ suite) (nativeSuiteResult, error) {
			called = append(called, index)
			if index == 2 {
				order[1] = 2
			}
			return nativeSuiteResult{}, nil
		})
	if err != nil || !reflect.DeepEqual(called, []int{2, 1, 0}) {
		t.Fatal("caller order mutation changed already validated dispatch", called, err)
	}
}

func TestNativeSuiteWorkersOrderedBoundCoverageAndOriginalIndices(t *testing.T) {
	ctx := workerTestContext(t)
	suites := workerTestSuites(6)
	order := []int{4, 2, 5, 0, 3, 1}
	calls := make([]atomic.Int32, len(suites))
	var active, peak atomic.Int32
	started := make(chan int, len(suites))
	release := make(chan struct{})
	finished := make(chan struct{})
	var results []nativeSuiteResult
	var err error
	go func() {
		results, err = runNativeSuitesOrdered(ctx, suites, 3, order,
			func(ctx context.Context, index int, s suite) (nativeSuiteResult, error) {
				calls[index].Add(1)
				now := active.Add(1)
				defer active.Add(-1)
				for old := peak.Load(); now > old && !peak.CompareAndSwap(old, now); old = peak.Load() {
				}
				started <- index
				select {
				case <-release:
				case <-ctx.Done():
					return nativeSuiteResult{}, ctx.Err()
				}
				return nativeSuiteResult{Observation: observation{Activity: s.Activity, Cases: index + 1}}, nil
			})
		close(finished)
	}()
	initial := map[int]bool{}
	for range 3 {
		select {
		case index := <-started:
			initial[index] = true
		case <-ctx.Done():
			t.Fatal("ordered workers did not start", ctx.Err())
		}
	}
	if !reflect.DeepEqual(initial, map[int]bool{4: true, 2: true, 5: true}) {
		t.Fatal("initial priority indices changed", initial)
	}
	select {
	case index := <-started:
		t.Fatal("worker bound exceeded", index)
	default:
	}
	close(release)
	select {
	case <-finished:
	case <-ctx.Done():
		t.Fatal("ordered workers did not join", ctx.Err())
	}
	if err != nil || active.Load() != 0 || peak.Load() != 3 || len(results) != len(suites) {
		t.Fatal("ordered coverage/bound/join", err, active.Load(), peak.Load())
	}
	for index, result := range results {
		if calls[index].Load() != 1 || result.Observation.Activity != suites[index].Activity || result.Observation.Cases != index+1 {
			t.Fatal("original index lost, repeated or skipped", index)
		}
	}
}

func TestNativeSuiteWorkersOrderedFailureRunsUnstartedLowerIndicesAndJoinsHigher(t *testing.T) {
	ctx := workerTestContext(t)
	suites := workerTestSuites(7)
	order := []int{4, 6, 5, 3, 2, 1, 0}
	highError, middleError, lowError := errors.New("index4 failed first"), errors.New("index2 failed later"), errors.New("index1 failed last")
	calls := make([]atomic.Int32, len(suites))
	var active atomic.Int32
	started := make(chan int, len(suites))
	releaseHigh := make(chan struct{})
	higherCanceled := make(chan struct{})
	releaseCanceled := make(chan struct{})
	lowerPrefixFinished := make(chan struct{})
	finished := make(chan struct{})
	var results []nativeSuiteResult
	var err error
	go func() {
		results, err = runNativeSuitesOrdered(ctx, suites, 2, order,
			func(suiteCtx context.Context, index int, s suite) (nativeSuiteResult, error) {
				calls[index].Add(1)
				active.Add(1)
				defer active.Add(-1)
				started <- index
				switch index {
				case 4:
					select {
					case <-releaseHigh:
						return nativeSuiteResult{}, highError
					case <-suiteCtx.Done():
						return nativeSuiteResult{}, suiteCtx.Err()
					}
				case 6:
					<-suiteCtx.Done()
					close(higherCanceled)
					select {
					case <-releaseCanceled:
					case <-ctx.Done():
					}
					return nativeSuiteResult{}, suiteCtx.Err()
				case 2:
					return nativeSuiteResult{}, middleError
				case 1:
					return nativeSuiteResult{}, lowError
				case 0:
					close(lowerPrefixFinished)
				}
				return nativeSuiteResult{Observation: observation{Activity: s.Activity}}, nil
			})
		close(finished)
	}()
	initial := map[int]bool{}
	for range 2 {
		select {
		case index := <-started:
			initial[index] = true
		case <-ctx.Done():
			t.Fatal("initial priority workers did not start", ctx.Err())
		}
	}
	if !reflect.DeepEqual(initial, map[int]bool{4: true, 6: true}) {
		t.Fatal("unexpected initial workers", initial)
	}
	close(releaseHigh)
	for _, signal := range []<-chan struct{}{higherCanceled, lowerPrefixFinished} {
		select {
		case <-signal:
		case <-ctx.Done():
			t.Fatal("lower original indices did not complete after higher failure", ctx.Err())
		}
	}
	select {
	case <-finished:
		t.Fatal("returned before the canceled higher callback joined")
	default:
	}
	close(releaseCanceled)
	select {
	case <-finished:
	case <-ctx.Done():
		t.Fatal("canceled higher callback did not join", ctx.Err())
	}
	if results != nil || !errors.Is(err, lowError) || !strings.Contains(err.Error(), "suite 1 example/worker:Suite01:") || active.Load() != 0 {
		t.Fatal("original serial-earliest failure or join lost", err, active.Load())
	}
	for index := range suites {
		want := int32(1)
		if index == 5 {
			want = 0
		}
		if calls[index].Load() != want {
			t.Fatal("lower index skipped or higher pending work started", index, calls[index].Load())
		}
	}
}

func TestNativeSuiteWorkersOrderedParentCancellationSkipsPendingLowerAndJoins(t *testing.T) {
	guard := workerTestContext(t)
	ctx, cancel := context.WithCancel(guard)
	defer cancel()
	calls := make([]atomic.Int32, 5)
	var active atomic.Int32
	started := make(chan int, 5)
	canceled := make(chan struct{}, 2)
	releaseCanceled := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		_, err := runNativeSuitesOrdered(ctx, workerTestSuites(5), 2, []int{4, 3, 2, 1, 0},
			func(suiteCtx context.Context, index int, _ suite) (nativeSuiteResult, error) {
				calls[index].Add(1)
				active.Add(1)
				defer active.Add(-1)
				started <- index
				<-suiteCtx.Done()
				canceled <- struct{}{}
				select {
				case <-releaseCanceled:
				case <-guard.Done():
				}
				return nativeSuiteResult{}, suiteCtx.Err()
			})
		finished <- err
	}()
	initial := map[int]bool{}
	for range 2 {
		select {
		case index := <-started:
			initial[index] = true
		case <-guard.Done():
			t.Fatal("initial ordered callbacks did not start", guard.Err())
		}
	}
	if !reflect.DeepEqual(initial, map[int]bool{4: true, 3: true}) {
		t.Fatal("unexpected priority callbacks", initial)
	}
	cancel()
	for range 2 {
		select {
		case <-canceled:
		case <-guard.Done():
			t.Fatal("parent cancellation did not reach both callbacks", guard.Err())
		}
	}
	select {
	case err := <-finished:
		t.Fatal("parent cancellation returned before joining callbacks", err)
	default:
	}
	close(releaseCanceled)
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) || active.Load() != 0 {
			t.Fatal("parent context error/join changed", err, active.Load())
		}
	case <-guard.Done():
		t.Fatal("canceled callbacks did not join", guard.Err())
	}
	for index := 0; index < 3; index++ {
		if calls[index].Load() != 0 {
			t.Fatal("parent cancellation started pending lower index", index)
		}
	}
}

func TestNativeSuiteFailFastCancelsAllJoinsAndKeepsFirstFailure(t *testing.T) {
	guard := workerTestContext(t)
	ctx, cancel := context.WithCancel(guard)
	defer cancel()
	suites := workerTestSuites(7)
	order := []int{4, 0, 6, 1, 2, 3, 5}
	first, later := errors.New("first completed semantic failure"), errors.New("later lower-index failure")
	calls := make([]atomic.Int32, len(suites))
	var active atomic.Int32
	started := make(chan int, len(suites))
	releaseFailure := make(chan struct{})
	canceled := make(chan int, 2)
	releaseJoin := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		results, err := runNativeSuitesOrderedPolicy(ctx, suites, 3, order, true,
			func(suiteCtx context.Context, index int, _ suite) (nativeSuiteResult, error) {
				calls[index].Add(1)
				active.Add(1)
				defer active.Add(-1)
				started <- index
				if index == 4 {
					select {
					case <-releaseFailure:
						return nativeSuiteResult{}, first
					case <-suiteCtx.Done():
						return nativeSuiteResult{}, suiteCtx.Err()
					}
				}
				<-suiteCtx.Done()
				canceled <- index
				select {
				case <-releaseJoin:
				case <-guard.Done():
				}
				if index == 0 {
					return nativeSuiteResult{}, later
				}
				return nativeSuiteResult{}, suiteCtx.Err()
			})
		if results != nil {
			finished <- errors.New("failure returned acceptance results")
			return
		}
		finished <- err
	}()
	initial := map[int]bool{}
	for range 3 {
		select {
		case index := <-started:
			initial[index] = true
		case <-guard.Done():
			t.Fatal("initial workers did not start", guard.Err())
		}
	}
	if !reflect.DeepEqual(initial, map[int]bool{4: true, 0: true, 6: true}) {
		t.Fatal("initial dispatch changed", initial)
	}
	close(releaseFailure)
	stopped := map[int]bool{}
	for range 2 {
		select {
		case index := <-canceled:
			stopped[index] = true
		case <-guard.Done():
			t.Fatal("active lower and higher workers not canceled", guard.Err())
		}
	}
	if !reflect.DeepEqual(stopped, map[int]bool{0: true, 6: true}) {
		t.Fatal("fail-fast canceled only one index direction", stopped)
	}
	select {
	case err := <-finished:
		t.Fatal("returned before joining canceled callbacks", err)
	default:
	}
	// Both canceled notifications prove the coordinator selected the semantic
	// failure before this later parent cancellation.
	cancel()
	close(releaseJoin)
	select {
	case err := <-finished:
		if !errors.Is(err, first) || errors.Is(err, later) || errors.Is(err, context.Canceled) || active.Load() != 0 ||
			!strings.Contains(err.Error(), "suite 4 example/worker:Suite04:") {
			t.Fatal("first failure or full join changed", err, active.Load())
		}
	case <-guard.Done():
		t.Fatal("fail-fast callbacks did not join", guard.Err())
	}
	for index := range suites {
		want := int32(0)
		if initial[index] {
			want = 1
		}
		if calls[index].Load() != want {
			t.Fatal("pending suite ran or active repeated", index, calls[index].Load())
		}
	}
}

func TestNativeSuiteFailFastParentCancellationPrecedesLateSemanticFailure(t *testing.T) {
	guard := workerTestContext(t)
	ctx, cancel := context.WithCancel(guard)
	defer cancel()
	started := make(chan int, 2)
	release := make(chan struct{})
	finished := make(chan error, 1)
	semantic := errors.New("failure returned only after parent cancellation")
	var active atomic.Int32
	go func() {
		_, err := runNativeSuitesOrderedPolicy(ctx, workerTestSuites(4), 2, []int{3, 0, 2, 1}, true,
			func(suiteCtx context.Context, index int, _ suite) (nativeSuiteResult, error) {
				active.Add(1)
				defer active.Add(-1)
				started <- index
				if index == 3 {
					select {
					case <-release:
					case <-guard.Done():
					}
					return nativeSuiteResult{}, semantic
				}
				<-suiteCtx.Done()
				return nativeSuiteResult{}, suiteCtx.Err()
			})
		finished <- err
	}()
	for range 2 {
		select {
		case <-started:
		case <-guard.Done():
			t.Fatal("workers did not start", guard.Err())
		}
	}
	cancel()
	close(release)
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) || errors.Is(err, semantic) || active.Load() != 0 {
			t.Fatal("late semantic failure replaced parent outcome or join", err, active.Load())
		}
	case <-guard.Done():
		t.Fatal("canceled callbacks did not join", guard.Err())
	}
	select {
	case index := <-started:
		t.Fatal("pending work started after parent cancellation", index)
	default:
	}
}

func TestNativeSuiteFailFastSuccessPreservesAllOriginalIndices(t *testing.T) {
	const count = 82
	suites := workerTestSuites(count)
	order := make([]int, count)
	for index := range order {
		order[index] = count - 1 - index
	}
	calls := make([]atomic.Int32, count)
	results, err := runNativeSuitesOrderedPolicy(workerTestContext(t), suites, 3, order, true,
		func(_ context.Context, index int, s suite) (nativeSuiteResult, error) {
			calls[index].Add(1)
			return nativeSuiteResult{Observation: observation{Activity: s.Activity, Cases: index + 1}}, nil
		})
	if err != nil || len(results) != count {
		t.Fatal("successful run incomplete", len(results), err)
	}
	for index, result := range results {
		if calls[index].Load() != 1 || result.Observation.Activity != suites[index].Activity || result.Observation.Cases != index+1 {
			t.Fatal("success skipped, repeated or reordered original index", index)
		}
	}
}

func TestNativeSuiteTimeoutBoundsAndOptInSummary(t *testing.T) {
	for _, timeout := range []time.Duration{time.Second, defaultNativeSuiteTimeout, 60 * time.Minute} {
		if err := validateNativeSuiteTimeout(timeout); err != nil {
			t.Fatal("valid timeout rejected", timeout, err)
		}
	}
	for _, timeout := range []time.Duration{-time.Second, 0, time.Second - time.Nanosecond, 60*time.Minute + time.Nanosecond} {
		if err := validateNativeSuiteTimeout(timeout); err == nil {
			t.Fatal("invalid timeout accepted", timeout)
		}
	}
	original := map[string]any{"cases": 123, "scope": "unchanged"}
	addNativeExecutionSummary(original, false, false, defaultNativeSuiteTimeout)
	if !reflect.DeepEqual(original, map[string]any{"cases": 123, "scope": "unchanged"}) {
		t.Fatal("default summary changed", original)
	}
	for _, failFast := range []bool{false, true} {
		summary := map[string]any{"cases": 123}
		addNativeExecutionSummary(summary, true, failFast, 60*time.Minute)
		selection := "earliest-catalog-index"
		if failFast {
			selection = "first-completed-uncanceled"
		}
		want := map[string]any{"cases": 123, "native_fail_fast": failFast, "native_timeout_ns": int64(60 * time.Minute), "native_failure_selection": selection}
		if !reflect.DeepEqual(summary, want) {
			t.Fatal("explicit policy summary differs", summary, want)
		}
	}
}

func TestNativeExecutionCLIRefusesBeforeUpstreamOrOutput(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	target := filepath.Join(root, "uncreated-parent", "evidence")
	priorFlags, priorArgs := flag.CommandLine, os.Args
	t.Cleanup(func() { flag.CommandLine, os.Args = priorFlags, priorArgs })
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"--out", target, "--timeout=0"}, "--timeout must be between"},
		{[]string{"--out", target, "--timeout=-1s"}, "--timeout must be between"},
		{[]string{"--out", target, "--timeout=999ms"}, "--timeout must be between"},
		{[]string{"--out", target, "--timeout=60m1ns"}, "--timeout must be between"},
		{[]string{"--out", target, "--timeout=invalid"}, "invalid value"},
		{[]string{"--fail-fast"}, "require --out"},
		{[]string{"--timeout=1s"}, "require --out"},
		{[]string{"--browser-fixtures=browser.json", "--fail-fast=false"}, "cannot be used with --browser-fixtures"},
		{[]string{"--browser-fixtures=browser.json", "--timeout=30m"}, "cannot be used with --browser-fixtures"},
		{[]string{"--ci-shard-plan-out=plan.json", "--ci-shard-context=unused", "--ci-shard-count=2", "--fail-fast=false"}, "full execution flags cannot be combined"},
		{[]string{"--ci-shard-plan-out=plan.json", "--ci-shard-context=unused", "--ci-shard-count=2", "--timeout=30m"}, "full execution flags cannot be combined"},
		{[]string{"--ci-shard-execute-out=native", "--ci-shard-context=unused", "--ci-shard-plan=unused", "--ci-shard-index=0", "--fail-fast=false"}, "no full or metadata output flags"},
		{[]string{"--ci-shard-verify-out=verified", "--ci-shard-context=unused", "--ci-shard-plan=unused", "--ci-shard-partial=unused", "--ci-shard-evidence=unused", "--timeout=30m"}, "no full or metadata output flags"},
	} {
		flag.CommandLine = flag.NewFlagSet("native-execution-test", flag.ContinueOnError)
		flag.CommandLine.SetOutput(&bytes.Buffer{})
		os.Args = append([]string{"verify"}, test.args...)
		if err := run(); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatal("policy flag not refused before source/output work", test.args, err)
		}
		if _, err := os.Stat(filepath.Dir(target)); !os.IsNotExist(err) {
			t.Fatal("refusal created output parent", err)
		}
		for _, name := range []string{"browser.json", "plan.json", "native", "verified"} {
			if _, err := os.Stat(name); !os.IsNotExist(err) {
				t.Fatal("refusal created output", name, err)
			}
		}
	}
}
